package search

import (
	"context"
	"fmt"
	"io"
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
		return nil, fmt.Errorf("duckduckgo: %w", err)
	}

	for k, v := range httpx.CommonHeaders() {
		req.Header.Set(k, v)
	}
	req.Header.Set("User-Agent", httpx.DefaultUserAgents()[0])

	resp, err := d.client.Do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("duckduckgo: fetch: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, httpx.MaxBodySize))
	if err != nil {
		return nil, fmt.Errorf("duckduckgo: read: %w", err)
	}

	return parseDDGResults(string(body), limit), nil
}

// ParseDuckDuckGoHTML parses DuckDuckGo HTML search results from a reader.
func ParseDuckDuckGoHTML(body io.Reader, limit int) ([]models.SearchResult, error) {
	data, err := io.ReadAll(io.LimitReader(body, httpx.MaxBodySize))
	if err != nil {
		return nil, err
	}
	return parseDDGResults(string(data), limit), nil
}

func parseDDGResults(html string, limit int) []models.SearchResult {
	var results []models.SearchResult

	// Strategy 1: Split on result__a class (standard DDG HTML layout)
	parts := strings.Split(html, "result__a")
	if len(parts) <= 1 {
		// Strategy 2: Try result-link
		parts = strings.Split(html, "result-link")
	}
	if len(parts) <= 1 {
		// Strategy 3: generic href extraction
		return extractHrefResults(html, limit)
	}

	for i := 1; i < len(parts) && len(results) < limit; i++ {
		part := parts[i]

		// Extract href - can appear before or after the class marker
		href := extractHref(part)
		if href == "" {
			// Try looking backward in the previous part for the href
			if i > 0 {
				prevTail := parts[i-1]
				if len(prevTail) > 200 {
					prevTail = prevTail[len(prevTail)-200:]
				}
				href = extractLastHref(prevTail)
			}
		}

		if href == "" {
			continue
		}

		// Decode DDG redirect URLs
		href = decodeDDGURL(href)

		// Skip non-http results
		if !strings.HasPrefix(href, "http") {
			continue
		}

		// Extract title (text between > and </a>)
		title := extractBetween(part, ">", "</a>")
		if title == "" {
			title = extractBetween(part, ">", "</")
		}
		title = stripTags(title)

		// Extract snippet
		snippet := ""
		if snipIdx := strings.Index(part, "result__snippet"); snipIdx >= 0 {
			snippet = extractBetween(part[snipIdx:], ">", "</")
			snippet = stripTags(snippet)
		}

		if title != "" {
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

// extractHref finds the first href="..." in a string.
func extractHref(s string) string {
	idx := strings.Index(s, "href=\"")
	if idx < 0 {
		return ""
	}
	s = s[idx+6:]
	end := strings.Index(s, "\"")
	if end < 0 {
		return ""
	}
	return s[:end]
}

// extractLastHref finds the last href="..." in a string.
func extractLastHref(s string) string {
	idx := strings.LastIndex(s, "href=\"")
	if idx < 0 {
		return ""
	}
	s = s[idx+6:]
	end := strings.Index(s, "\"")
	if end < 0 {
		return ""
	}
	return s[:end]
}

// decodeDDGURL handles DuckDuckGo's redirect wrapper.
func decodeDDGURL(href string) string {
	if strings.Contains(href, "uddg=") {
		if u, err := url.Parse(href); err == nil {
			if actual := u.Query().Get("uddg"); actual != "" {
				return actual
			}
		}
	}
	// Also handle //duckduckgo.com/l/?... redirects
	if strings.Contains(href, "duckduckgo.com/l/") {
		if u, err := url.Parse(href); err == nil {
			if actual := u.Query().Get("uddg"); actual != "" {
				return actual
			}
		}
	}
	return href
}

// extractHrefResults is a fallback that extracts all links from the page.
func extractHrefResults(html string, limit int) []models.SearchResult {
	var results []models.SearchResult
	seen := make(map[string]bool)

	offset := 0
	for len(results) < limit {
		idx := strings.Index(html[offset:], "href=\"http")
		if idx < 0 {
			break
		}
		pos := offset + idx + 6
		end := strings.Index(html[pos:], "\"")
		if end < 0 {
			break
		}
		href := html[pos : pos+end]
		offset = pos + end

		href = decodeDDGURL(href)

		// Skip DDG's own URLs
		if strings.Contains(href, "duckduckgo.com") {
			continue
		}
		if seen[href] {
			continue
		}
		seen[href] = true

		results = append(results, models.SearchResult{
			Title:  extractDomainTitle(href),
			URL:    href,
			Source: "duckduckgo",
		})
	}
	return results
}

func extractDomainTitle(rawURL string) string {
	if u, err := url.Parse(rawURL); err == nil {
		return u.Host
	}
	return rawURL
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
