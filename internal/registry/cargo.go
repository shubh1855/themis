package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/cache"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/models"
)

const cratesSearchURL = "https://crates.io/api/v1/crates"

type Cargo struct {
	*BaseClient
}

func NewCargo(base *BaseClient) *Cargo {
	return &Cargo{BaseClient: base}
}

func (c *Cargo) Name() string { return "cargo" }

func (c *Cargo) Search(query string, limit int) (*models.PackageSearchResult, error) {
	if limit <= 0 {
		limit = 10
	}

	cacheKey := cache.SearchKey("cargo", query, limit)
	if cached, ok := c.Cache.Get(cacheKey); ok {
		if result, ok := cached.(*models.PackageSearchResult); ok {
			return result, nil
		}
	}

	searchURL := fmt.Sprintf("%s?q=%s&per_page=%d", cratesSearchURL, url.QueryEscape(query), limit)
	body, err := c.HTTP.GetBody(context.Background(), searchURL)
	if err != nil {
		return nil, fmt.Errorf("cargo: search: %w", err)
	}

	var resp cratesSearchResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil, fmt.Errorf("cargo: parse search: %w", err)
	}

	result := &models.PackageSearchResult{
		Query:    query,
		Registry: "cargo",
		Total:    resp.Meta.Total,
	}

	for _, crate := range resp.Crates {
		result.Packages = append(result.Packages, models.PackageInfo{
			Name:        crate.Name,
			Version:     crate.MaxVersion,
			Description: crate.Description,
			Homepage:    crate.Homepage,
			Repository:  crate.Repository,
			Downloads:   crate.Downloads,
			Registry:    "cargo",
		})
	}

	c.Cache.SetWithTTL(cacheKey, result, cache.RegistryTTL)
	return result, nil
}

func (c *Cargo) Lookup(name string) (*models.PackageInfo, error) {
	cacheKey := cache.RegistryKey("cargo", name)
	if cached, ok := c.Cache.Get(cacheKey); ok {
		if info, ok := cached.(*models.PackageInfo); ok {
			return info, nil
		}
	}

	pkgURL := fmt.Sprintf("%s/%s", cratesSearchURL, url.PathEscape(name))
	body, err := c.HTTP.GetBody(context.Background(), pkgURL)
	if err != nil {
		return nil, fmt.Errorf("cargo: lookup %q: %w", name, err)
	}

	var resp crateDetailResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil, fmt.Errorf("cargo: parse %q: %w", name, err)
	}

	info := &models.PackageInfo{
		Name:        resp.Crate.Name,
		Version:     resp.Crate.MaxVersion,
		Description: resp.Crate.Description,
		Homepage:    resp.Crate.Homepage,
		Repository:  resp.Crate.Repository,
		Downloads:   resp.Crate.Downloads,
		Registry:    "cargo",
	}

	c.Cache.SetWithTTL(cacheKey, info, cache.RegistryTTL)
	return info, nil
}

type cratesSearchResponse struct {
	Crates []crateInfo `json:"crates"`
	Meta   struct {
		Total int `json:"total"`
	} `json:"meta"`
}

type crateDetailResponse struct {
	Crate crateInfo `json:"crate"`
}

type crateInfo struct {
	Name        string `json:"name"`
	MaxVersion  string `json:"max_version"`
	Description string `json:"description"`
	Homepage    string `json:"homepage"`
	Repository  string `json:"repository"`
	Downloads   int64  `json:"downloads"`
}
