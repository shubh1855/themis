package scraper

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/httpx"
)

type Fetcher struct {
	client *httpx.Client
}

func NewFetcher(client *httpx.Client) *Fetcher {
	return &Fetcher{client: client}
}

func (f *Fetcher) FetchPage(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("scraper: request: %w", err)
	}

	for k, v := range httpx.CommonHeaders() {
		req.Header.Set(k, v)
	}
	req.Header.Set("User-Agent", httpx.DefaultUserAgents()[0])

	resp, err := f.client.Do(ctx, req)
	if err != nil {
		return "", fmt.Errorf("scraper: fetch %q: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("scraper: %q returned %d", url, resp.StatusCode)
	}

	limited := io.LimitReader(resp.Body, httpx.MaxBodySize)
	body, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("scraper: read: %w", err)
	}

	return string(body), nil
}

func (f *Fetcher) FetchJSON(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("scraper: json request: %w", err)
	}

	for k, v := range httpx.JSONHeaders() {
		req.Header.Set(k, v)
	}
	req.Header.Set("User-Agent", httpx.DefaultUserAgents()[0])

	resp, err := f.client.Do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("scraper: fetch json %q: %w", url, err)
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, httpx.MaxBodySize)
	return io.ReadAll(limited)
}
