package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/cache"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/models"
)

const goProxyURL = "https://proxy.golang.org"
const goPkgSearchURL = "https://pkg.go.dev/search"

// GoLang implements the Client interface for the Go module proxy.
type GoLang struct {
	*BaseClient
}

// NewGoLang creates a Go module registry client.
func NewGoLang(base *BaseClient) *GoLang {
	return &GoLang{BaseClient: base}
}

// Name returns the registry identifier.
func (g *GoLang) Name() string { return "go" }

// Search finds Go packages matching the query.
// Uses the Go proxy's search or falls back to pkg.go.dev.
func (g *GoLang) Search(query string, limit int) (*models.PackageSearchResult, error) {
	if limit <= 0 {
		limit = 10
	}

	cacheKey := cache.SearchKey("go", query, limit)
	if cached, ok := g.Cache.Get(cacheKey); ok {
		if result, ok := cached.(*models.PackageSearchResult); ok {
			return result, nil
		}
	}

	// Go proxy doesn't have a search API, try direct lookup
	info, err := g.Lookup(query)
	if err != nil {
		return &models.PackageSearchResult{
			Query:    query,
			Registry: "go",
			Packages: []models.PackageInfo{},
			Total:    0,
		}, nil
	}

	result := &models.PackageSearchResult{
		Query:    query,
		Registry: "go",
		Packages: []models.PackageInfo{*info},
		Total:    1,
	}

	g.Cache.SetWithTTL(cacheKey, result, cache.RegistryTTL)
	return result, nil
}

// Lookup retrieves metadata for a specific Go module.
func (g *GoLang) Lookup(name string) (*models.PackageInfo, error) {
	cacheKey := cache.RegistryKey("go", name)
	if cached, ok := g.Cache.Get(cacheKey); ok {
		if info, ok := cached.(*models.PackageInfo); ok {
			return info, nil
		}
	}

	// Get latest version from proxy
	latestURL := fmt.Sprintf("%s/%s/@latest", goProxyURL, url.PathEscape(name))
	body, err := g.HTTP.GetBody(context.Background(), latestURL)
	if err != nil {
		return nil, fmt.Errorf("go: lookup %q: %w", name, err)
	}

	var resp goModuleResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil, fmt.Errorf("go: parse %q: %w", name, err)
	}

	info := &models.PackageInfo{
		Name:     name,
		Version:  resp.Version,
		Homepage: fmt.Sprintf("https://pkg.go.dev/%s", name),
		Registry: "go",
	}

	g.Cache.SetWithTTL(cacheKey, info, cache.RegistryTTL)
	return info, nil
}

type goModuleResponse struct {
	Version string `json:"Version"`
	Time    string `json:"Time"`
}
