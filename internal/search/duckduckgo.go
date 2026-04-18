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

type DuckDuckGo struct {
	client *httpx.Client
}

func NewDuckDuckGo(client *httpx.Client) *DuckDuckGo {
	return &DuckDuckGo{client: client}
}

func (d *DuckDuckGo) Name() string { return "duckduckgo" }

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

func ParseDuckDuckGoHTML(body io.Reader, limit int) ([]models.SearchResult, error) {
	data, err := io.ReadAll(io.LimitReader(body, httpx.MaxBodySize))
	if err != nil {
		return nil, err
	}
	return parseDDGResults(string(data), limit), nil
}

func parseDDGResults(html string, limit int) []models.SearchResult {
	var results []models.SearchResult

	parts := strings.Split(html, "result__a")
	if len(parts) <= 1 {
		parts = strings.Split(html, "result-link")
	}
	if len(parts) <= 1 {
		return extractHrefResults(html, limit)
	}

	for i := 1; i < len(parts) && len(results) < limit; i++ {
		part := parts[i]

		href := extractHref(part)
		if href == "" {
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

		href = decodeDDGURL(href)

		if !strings.HasPrefix(href, "http") {
			continue
		}

		title := extractBetween(part, ">", "</a>")
		if title == "" {
			title = extractBetween(part, ">", "</")
		}
		title = stripTags(title)

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

func decodeDDGURL(href string) string {
	if strings.Contains(href, "uddg=") {
		if u, err := url.Parse(href); err == nil {
			if actual := u.Query().Get("uddg"); actual != "" {
				return actual
			}
		}
	}
	if strings.Contains(href, "duckduckgo.com/l/") {
		if u, err := url.Parse(href); err == nil {
			if actual := u.Query().Get("uddg"); actual != "" {
				return actual
			}
		}
	}
	return href
}

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
