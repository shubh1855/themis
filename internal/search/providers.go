package search

import (
	"context"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/models"
)

type Provider interface {
	Name() string
	Search(ctx context.Context, query string, limit int) ([]models.SearchResult, error)
}
