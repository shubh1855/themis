// Package registry provides clients for package ecosystem registries.
package registry

import (
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/cache"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/httpx"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/models"
)

// Client defines the interface every registry client must implement.
type Client interface {
	// Search finds packages matching a query.
	Search(query string, limit int) (*models.PackageSearchResult, error)
	// Lookup retrieves metadata for a specific package.
	Lookup(name string) (*models.PackageInfo, error)
	// Name returns the registry identifier.
	Name() string
}

// BaseClient contains shared dependencies for all registry clients.
type BaseClient struct {
	HTTP  *httpx.Client
	Cache *cache.Memory
}

// NewBaseClient creates the shared base for registry clients.
func NewBaseClient(http *httpx.Client, c *cache.Memory) *BaseClient {
	return &BaseClient{HTTP: http, Cache: c}
}
