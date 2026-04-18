package tools

import (
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/cache"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/httpx"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/models"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/search"
)

// ToolHandler is a function that handles a specific tool invocation.
type ToolHandler func(ctx Context) models.ToolResponse

// Context carries all dependencies and the incoming request for a tool handler.
type Context struct {
	Req     models.ToolRequest
	Deps    *Dependencies
}

// Dependencies holds all shared service instances injected into tool handlers.
type Dependencies struct {
	HTTP         *httpx.Client
	Cache        *cache.Memory
	SearchEngine *search.Engine
	RootDir      string
}

// NewDependencies creates the full dependency graph for tool handlers.
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
