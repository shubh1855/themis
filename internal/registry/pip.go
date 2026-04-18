package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/cache"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/models"
)

const pypiURL = "https://pypi.org/pypi"
const pypiSearchURL = "https://pypi.org/search"

type Pip struct {
	*BaseClient
}

func NewPip(base *BaseClient) *Pip {
	return &Pip{BaseClient: base}
}

func (p *Pip) Name() string { return "pip" }

func (p *Pip) Search(query string, limit int) (*models.PackageSearchResult, error) {
	if limit <= 0 {
		limit = 10
	}

	cacheKey := cache.SearchKey("pip", query, limit)
	if cached, ok := p.Cache.Get(cacheKey); ok {
		if result, ok := cached.(*models.PackageSearchResult); ok {
			return result, nil
		}
	}

	info, err := p.Lookup(query)
	if err != nil {
		return &models.PackageSearchResult{
			Query:    query,
			Registry: "pip",
			Packages: []models.PackageInfo{},
			Total:    0,
		}, nil
	}

	result := &models.PackageSearchResult{
		Query:    query,
		Registry: "pip",
		Packages: []models.PackageInfo{*info},
		Total:    1,
	}

	p.Cache.SetWithTTL(cacheKey, result, cache.RegistryTTL)
	return result, nil
}

func (p *Pip) Lookup(name string) (*models.PackageInfo, error) {
	cacheKey := cache.RegistryKey("pip", name)
	if cached, ok := p.Cache.Get(cacheKey); ok {
		if info, ok := cached.(*models.PackageInfo); ok {
			return info, nil
		}
	}

	pkgURL := fmt.Sprintf("%s/%s/json", pypiURL, url.PathEscape(name))
	body, err := p.HTTP.GetBody(context.Background(), pkgURL)
	if err != nil {
		return nil, fmt.Errorf("pip: lookup %q: %w", name, err)
	}

	var pkg pypiResponse
	if err := json.Unmarshal([]byte(body), &pkg); err != nil {
		return nil, fmt.Errorf("pip: parse %q: %w", name, err)
	}

	info := &models.PackageInfo{
		Name:        pkg.Info.Name,
		Version:     pkg.Info.Version,
		Description: pkg.Info.Summary,
		Homepage:    pkg.Info.HomePage,
		License:     pkg.Info.License,
		Keywords:    splitKeywords(pkg.Info.Keywords),
		Registry:    "pip",
	}

	if pkg.Info.ProjectURL != "" {
		info.Repository = pkg.Info.ProjectURL
	}

	p.Cache.SetWithTTL(cacheKey, info, cache.RegistryTTL)
	return info, nil
}

type pypiResponse struct {
	Info struct {
		Name       string `json:"name"`
		Version    string `json:"version"`
		Summary    string `json:"summary"`
		HomePage   string `json:"home_page"`
		License    string `json:"license"`
		Keywords   string `json:"keywords"`
		ProjectURL string `json:"project_url"`
	} `json:"info"`
}

func splitKeywords(s string) []string {
	if s == "" {
		return nil
	}
	var keywords []string
	for _, k := range []string{","} {
		_ = k
	}
	parts := make([]string, 0)
	for _, part := range split(s) {
		if part != "" {
			parts = append(parts, part)
		}
	}
	keywords = parts
	return keywords
}

func split(s string) []string {
	var result []string
	current := ""
	for _, r := range s {
		if r == ',' || r == ' ' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(r)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}
