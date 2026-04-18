// Package search implements a meta search engine with pluggable providers.
package search

import (
	"context"
	"fmt"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/cache"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/models"
)

// Engine coordinates search across providers with caching and deduplication.
type Engine struct {
	providers []Provider
	cache     *cache.Memory
}

// NewEngine creates a search engine with the given providers and cache.
func NewEngine(providers []Provider, c *cache.Memory) *Engine {
	return &Engine{
		providers: providers,
		cache:     c,
	}
}

// Search queries all providers and returns merged, deduplicated, ranked results.
func (e *Engine) Search(ctx context.Context, query string, limit int) (*models.SearchResponse, error) {
	if query == "" {
		return nil, fmt.Errorf("search: empty query")
	}
	if limit <= 0 {
		limit = 10
	}

	// Check cache
	cacheKey := cache.SearchKey("meta", query, limit)
	if cached, ok := e.cache.Get(cacheKey); ok {
		if resp, ok := cached.(*models.SearchResponse); ok {
			return resp, nil
		}
	}

	// Query all providers
	var allResults []models.SearchResult
	for _, p := range e.providers {
		results, err := p.Search(ctx, query, limit)
		if err != nil {
			// Log but don't fail; try other providers
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

	// Dedupe, rank, normalize
	deduped := Deduplicate(allResults)
	ranked := Rank(deduped)
	normalized := NormalizeResults(ranked)

	// Trim to limit
	if len(normalized) > limit {
		normalized = normalized[:limit]
	}

	// Assign final ranks
	for i := range normalized {
		normalized[i].Rank = i + 1
	}

	resp := &models.SearchResponse{
		Query:   query,
		Results: normalized,
		Total:   len(normalized),
	}

	// Cache
	e.cache.SetWithTTL(cacheKey, resp, cache.SearchTTL)

	return resp, nil
}
