// Package scraper provides content extraction from web pages.
package scraper

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/httpx"
)

// Fetcher retrieves web page content using the hardened HTTP client.
type Fetcher struct {
	client *httpx.Client
}

// NewFetcher creates a page fetcher.
func NewFetcher(client *httpx.Client) *Fetcher {
	return &Fetcher{client: client}
}

// FetchPage retrieves the raw HTML of a URL.
func (f *Fetcher) FetchPage(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("scraper: request: %w", err)
	}

	for k, v := range httpx.CommonHeaders() {
		req.Header.Set(k, v)
	}

	resp, err := f.client.Do(ctx, req)
	if err != nil {
		return "", fmt.Errorf("scraper: fetch %q: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("scraper: %q returned %d", url, resp.StatusCode)
	}

	limited := io.LimitReader(resp.Body, httpx.MaxBodySize)
	body, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("scraper: read: %w", err)
	}

	return string(body), nil
}

// FetchJSON fetches a URL and returns the raw JSON body.
func (f *Fetcher) FetchJSON(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("scraper: json request: %w", err)
	}

	for k, v := range httpx.JSONHeaders() {
		req.Header.Set(k, v)
	}

	resp, err := f.client.Do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("scraper: fetch json %q: %w", url, err)
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, httpx.MaxBodySize)
	return io.ReadAll(limited)
}
