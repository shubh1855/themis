package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/cache"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/models"
)

const npmRegistryURL = "https://registry.npmjs.org"
const npmSearchURL = "https://registry.npmjs.org/-/v1/search"

type NPM struct {
	*BaseClient
}

func NewNPM(base *BaseClient) *NPM {
	return &NPM{BaseClient: base}
}

func (n *NPM) Name() string { return "npm" }

func (n *NPM) Search(query string, limit int) (*models.PackageSearchResult, error) {
	if limit <= 0 {
		limit = 10
	}

	cacheKey := cache.SearchKey("npm", query, limit)
	if cached, ok := n.Cache.Get(cacheKey); ok {
		if result, ok := cached.(*models.PackageSearchResult); ok {
			return result, nil
		}
	}

	searchURL := fmt.Sprintf("%s?text=%s&size=%d", npmSearchURL, url.QueryEscape(query), limit)
	body, err := n.HTTP.GetBody(context.Background(), searchURL)
	if err != nil {
		return nil, fmt.Errorf("npm: search: %w", err)
	}

	var resp npmSearchResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil, fmt.Errorf("npm: parse search: %w", err)
	}

	result := &models.PackageSearchResult{
		Query:    query,
		Registry: "npm",
		Total:    resp.Total,
	}

	for _, obj := range resp.Objects {
		p := obj.Package
		result.Packages = append(result.Packages, models.PackageInfo{
			Name:        p.Name,
			Version:     p.Version,
			Description: p.Description,
			Homepage:    p.Links.Homepage,
			Repository:  p.Links.Repository,
			License:     p.License,
			Keywords:    p.Keywords,
			Registry:    "npm",
		})
	}

	n.Cache.SetWithTTL(cacheKey, result, cache.RegistryTTL)
	return result, nil
}

func (n *NPM) Lookup(name string) (*models.PackageInfo, error) {
	cacheKey := cache.RegistryKey("npm", name)
	if cached, ok := n.Cache.Get(cacheKey); ok {
		if info, ok := cached.(*models.PackageInfo); ok {
			return info, nil
		}
	}

	pkgURL := fmt.Sprintf("%s/%s", npmRegistryURL, url.PathEscape(name))
	body, err := n.HTTP.GetBody(context.Background(), pkgURL)
	if err != nil {
		return nil, fmt.Errorf("npm: lookup %q: %w", name, err)
	}

	var pkg npmPackageResponse
	if err := json.Unmarshal([]byte(body), &pkg); err != nil {
		return nil, fmt.Errorf("npm: parse %q: %w", name, err)
	}

	info := &models.PackageInfo{
		Name:        pkg.Name,
		Version:     pkg.DistTags.Latest,
		Description: pkg.Description,
		Homepage:    pkg.Homepage,
		License:     pkg.License,
		Keywords:    pkg.Keywords,
		Registry:    "npm",
	}

	if pkg.Repository.URL != "" {
		info.Repository = pkg.Repository.URL
	}

	n.Cache.SetWithTTL(cacheKey, info, cache.RegistryTTL)
	return info, nil
}

type npmSearchResponse struct {
	Objects []struct {
		Package struct {
			Name        string   `json:"name"`
			Version     string   `json:"version"`
			Description string   `json:"description"`
			Keywords    []string `json:"keywords"`
			License     string   `json:"license"`
			Links       struct {
				Homepage   string `json:"homepage"`
				Repository string `json:"repository"`
			} `json:"links"`
		} `json:"package"`
	} `json:"objects"`
	Total int `json:"total"`
}

type npmPackageResponse struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Homepage    string   `json:"homepage"`
	License     string   `json:"license"`
	Keywords    []string `json:"keywords"`
	DistTags    struct {
		Latest string `json:"latest"`
	} `json:"dist-tags"`
	Repository struct {
		URL string `json:"url"`
	} `json:"repository"`
}
