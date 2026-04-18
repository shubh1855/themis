package registry

import (
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/cache"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/httpx"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/models"
)

type Client interface {
	Search(query string, limit int) (*models.PackageSearchResult, error)
	Lookup(name string) (*models.PackageInfo, error)
	Name() string
}

type BaseClient struct {
	HTTP  *httpx.Client
	Cache *cache.Memory
}

func NewBaseClient(http *httpx.Client, c *cache.Memory) *BaseClient {
	return &BaseClient{HTTP: http, Cache: c}
}
