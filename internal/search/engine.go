package search

import (
	"context"
	"fmt"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/cache"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/models"
)

type Engine struct {
	providers []Provider
	cache     *cache.Memory
}

func NewEngine(providers []Provider, c *cache.Memory) *Engine {
	return &Engine{
		providers: providers,
		cache:     c,
	}
}

func (e *Engine) Search(ctx context.Context, query string, limit int) (*models.SearchResponse, error) {
	if query == "" {
		return nil, fmt.Errorf("search: empty query")
	}
	if limit <= 0 {
		limit = 10
	}

	cacheKey := cache.SearchKey("meta", query, limit)
	if cached, ok := e.cache.Get(cacheKey); ok {
		if resp, ok := cached.(*models.SearchResponse); ok {
			return resp, nil
		}
	}

	var allResults []models.SearchResult
	for _, p := range e.providers {
		results, err := p.Search(ctx, query, limit)
		if err != nil {
			continue
		}
		allResults = append(allResults, results...)
	}

	if len(allResults) == 0 {
		return &models.SearchResponse{
			Query:   query,
			Results: []models.SearchResult{},
			Total:   0,
		}, nil
	}

	deduped := Deduplicate(allResults)
	ranked := Rank(deduped)
	normalized := NormalizeResults(ranked)

	if len(normalized) > limit {
		normalized = normalized[:limit]
	}

	for i := range normalized {
		normalized[i].Rank = i + 1
	}

	resp := &models.SearchResponse{
		Query:   query,
		Results: normalized,
		Total:   len(normalized),
	}

	e.cache.SetWithTTL(cacheKey, resp, cache.SearchTTL)

	return resp, nil
}
