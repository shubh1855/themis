package search

import (
	"context"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/models"
)

// Provider defines the interface for search providers.
type Provider interface {
	// Name returns the provider identifier.
	Name() string
	// Search returns results for the given query.
	Search(ctx context.Context, query string, limit int) ([]models.SearchResult, error)
}
