package search

import (
	"strings"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/models"
)

// Deduplicate removes duplicate results by URL.
func Deduplicate(results []models.SearchResult) []models.SearchResult {
	seen := make(map[string]bool)
	var unique []models.SearchResult

	for _, r := range results {
		normalized := normalizeURL(r.URL)
		if seen[normalized] {
			continue
		}
		seen[normalized] = true
		unique = append(unique, r)
	}

	return unique
}

// NormalizeResults cleans up titles and snippets across all results.
func NormalizeResults(results []models.SearchResult) []models.SearchResult {
	for i := range results {
		results[i].Title = normalizeText(results[i].Title)
		results[i].Snippet = normalizeText(results[i].Snippet)
		results[i].URL = strings.TrimSpace(results[i].URL)
	}
	return results
}

func normalizeURL(u string) string {
	u = strings.TrimSpace(u)
	u = strings.TrimRight(u, "/")
	u = strings.ToLower(u)
	// Remove trailing fragments
	if idx := strings.Index(u, "#"); idx >= 0 {
		u = u[:idx]
	}
	return u
}

func normalizeText(s string) string {
	s = strings.TrimSpace(s)
	// Collapse multiple spaces
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	// Remove control characters
	var clean strings.Builder
	for _, r := range s {
		if r >= 32 || r == '\n' || r == '\t' {
			clean.WriteRune(r)
		}
	}
	return clean.String()
}
