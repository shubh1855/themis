package search

import (
	"sort"
	"strings"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/models"
)

func Rank(results []models.SearchResult) []models.SearchResult {
	sort.SliceStable(results, func(i, j int) bool {
		return score(results[i]) > score(results[j])
	})
	return results
}

func score(r models.SearchResult) int {
	s := 0
	if r.Title != "" {
		s += 3
	}
	if r.URL != "" {
		s += 2
	}
	if r.Snippet != "" {
		s += 2
	}
	if len(r.Snippet) > 50 {
		s += 1
	}
	if strings.HasPrefix(r.URL, "https://") {
		s += 1
	}
	return s
}
