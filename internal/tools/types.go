package tools

import (
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/cache"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/httpx"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/models"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/search"
)

type ToolHandler func(ctx Context) models.ToolResponse

type Context struct {
	Req     models.ToolRequest
	Deps    *Dependencies
}

type Dependencies struct {
	HTTP         *httpx.Client
	Cache        *cache.Memory
	SearchEngine *search.Engine
	RootDir      string
}

func NewDependencies(rootDir string) *Dependencies {
	httpClient := httpx.NewClient()
	memCache := cache.NewMemory(cache.DefaultTTL, cache.CleanupInterval)

	ddg := search.NewDuckDuckGo(httpClient)
	searchEngine := search.NewEngine([]search.Provider{ddg}, memCache)

	return &Dependencies{
		HTTP:         httpClient,
		Cache:        memCache,
		SearchEngine: searchEngine,
		RootDir:      rootDir,
	}
}
