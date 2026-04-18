package search

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/httpx"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/models"
)

// DuckDuckGo implements the Provider interface using DuckDuckGo HTML search.
type DuckDuckGo struct {
	client *httpx.Client
}

// NewDuckDuckGo creates a new DuckDuckGo search provider.
func NewDuckDuckGo(client *httpx.Client) *DuckDuckGo {
	return &DuckDuckGo{client: client}
}

// Name returns the provider name.
func (d *DuckDuckGo) Name() string { return "duckduckgo" }

// Search queries DuckDuckGo's HTML interface and parses results.
func (d *DuckDuckGo) Search(ctx context.Context, query string, limit int) ([]models.SearchResult, error) {
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("duckduckgo: create request: %w", err)
	}

	for k, v := range httpx.CommonHeaders() {
		req.Header.Set(k, v)
	}

	resp, err := d.client.Do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("duckduckgo: fetch: %w", err)
	}
	defer resp.Body.Close()

	return ParseDuckDuckGoHTML(resp.Body, limit)
}

// ParseDuckDuckGoHTML parses DuckDuckGo HTML search results page.
func ParseDuckDuckGoHTML(body interface{ Read([]byte) (int, error) }, limit int) ([]models.SearchResult, error) {
	buf := make([]byte, httpx.MaxBodySize)
	n, _ := body.Read(buf)
	html := string(buf[:n])

	return parseDDGResults(html, limit), nil
}

func parseDDGResults(html string, limit int) []models.SearchResult {
	var results []models.SearchResult

	// Parse result blocks between <div class="result"> markers
	parts := strings.Split(html, "class=\"result__a\"")
	if len(parts) <= 1 {
		// Fallback: try alternate class names
		parts = strings.Split(html, "class=\"result-link\"")
	}

	for i := 1; i < len(parts) && len(results) < limit; i++ {
		part := parts[i]

		title := extractBetween(part, ">", "</a>")
		title = stripTags(title)

		href := extractBetween(part, "href=\"", "\"")
		if href == "" {
			continue
		}
		// DuckDuckGo sometimes uses redirect URLs
		if strings.Contains(href, "uddg=") {
			if u, err := url.Parse(href); err == nil {
				if actual := u.Query().Get("uddg"); actual != "" {
					href = actual
				}
			}
		}

		snippet := ""
		if snipIdx := strings.Index(part, "result__snippet"); snipIdx >= 0 {
			snippet = extractBetween(part[snipIdx:], ">", "</")
			snippet = stripTags(snippet)
		}

		if title != "" && href != "" {
			results = append(results, models.SearchResult{
				Title:   strings.TrimSpace(title),
				URL:     href,
				Snippet: strings.TrimSpace(snippet),
				Source:  "duckduckgo",
			})
		}
	}

	return results
}

func extractBetween(s, start, end string) string {
	si := strings.Index(s, start)
	if si < 0 {
		return ""
	}
	s = s[si+len(start):]
	ei := strings.Index(s, end)
	if ei < 0 {
		return s
	}
	return s[:ei]
}

func stripTags(s string) string {
	var result strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			result.WriteRune(r)
		}
	}
	return result.String()
}
