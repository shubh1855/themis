package tools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/httpx"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/models"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/scraper"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/security"
)

// HandleWebSearch performs a web search via the search engine.
func HandleWebSearch(ctx Context) models.ToolResponse {
	query := models.ArgString(ctx.Req.Args, "query")
	if query == "" {
		return models.ErrorResponse("web_search: missing 'query' argument")
	}
	limit := models.ArgInt(ctx.Req.Args, "limit", 10)

	results, err := ctx.Deps.SearchEngine.Search(context.Background(), query, limit)
	if err != nil {
		return models.ErrorResponsef("web_search: %v", err)
	}
	return models.SuccessResponse(results)
}

// HandleFetchURL fetches the readable text content of a URL.
func HandleFetchURL(ctx Context) models.ToolResponse {
	rawURL := models.ArgString(ctx.Req.Args, "url")
	if rawURL == "" {
		return models.ErrorResponse("fetch_url: missing 'url' argument")
	}

	if _, err := security.ValidateURL(rawURL); err != nil {
		return models.ErrorResponsef("fetch_url: %v", err)
	}

	fetcher := scraper.NewFetcher(ctx.Deps.HTTP)
	html, err := fetcher.FetchPage(context.Background(), rawURL)
	if err != nil {
		return models.ErrorResponsef("fetch_url: %v", err)
	}

	text := scraper.ExtractMainText(html)
	meta := scraper.ExtractMetadata(html)

	return models.SuccessResponse(models.PageContent{
		URL:      rawURL,
		Title:    meta.Title,
		Text:     text,
		Links:    meta.Links,
		Headings: meta.Headings,
		Meta:     meta.Meta,
	})
}

// HandleFetchJSON fetches a URL and returns parsed JSON.
func HandleFetchJSON(ctx Context) models.ToolResponse {
	rawURL := models.ArgString(ctx.Req.Args, "url")
	if rawURL == "" {
		return models.ErrorResponse("fetch_json: missing 'url' argument")
	}

	if _, err := security.ValidateURL(rawURL); err != nil {
		return models.ErrorResponsef("fetch_json: %v", err)
	}

	fetcher := scraper.NewFetcher(ctx.Deps.HTTP)
	body, err := fetcher.FetchJSON(context.Background(), rawURL)
	if err != nil {
		return models.ErrorResponsef("fetch_json: %v", err)
	}

	var data interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return models.ErrorResponsef("fetch_json: invalid JSON: %v", err)
	}

	return models.SuccessResponse(data)
}

// HandleDownloadFile downloads a file from a URL and saves it locally.
func HandleDownloadFile(ctx Context) models.ToolResponse {
	rawURL := models.ArgString(ctx.Req.Args, "url")
	dest := models.ArgString(ctx.Req.Args, "path")
	if rawURL == "" {
		return models.ErrorResponse("download_file: missing 'url' argument")
	}

	if _, err := security.ValidateURL(rawURL); err != nil {
		return models.ErrorResponsef("download_file: %v", err)
	}

	if dest == "" {
		dest = filepath.Join(ctx.Deps.RootDir, security.SafeFilename(filepath.Base(rawURL)))
	}

	safeDest, err := security.SanitizePath(ctx.Deps.RootDir, dest)
	if err != nil {
		return models.ErrorResponsef("download_file: %v", err)
	}

	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return models.ErrorResponsef("download_file: %v", err)
	}

	resp, err := ctx.Deps.HTTP.Do(context.Background(), req)
	if err != nil {
		return models.ErrorResponsef("download_file: %v", err)
	}
	defer resp.Body.Close()

	if err := os.MkdirAll(filepath.Dir(safeDest), 0755); err != nil {
		return models.ErrorResponsef("download_file: mkdir: %v", err)
	}

	f, err := os.Create(safeDest)
	if err != nil {
		return models.ErrorResponsef("download_file: create: %v", err)
	}
	defer f.Close()

	written, err := io.Copy(f, io.LimitReader(resp.Body, httpx.MaxBodySize))
	if err != nil {
		return models.ErrorResponsef("download_file: write: %v", err)
	}

	return models.SuccessResponse(map[string]interface{}{
		"path":  safeDest,
		"bytes": written,
	})
}

// HandleScrapePage scrapes a page using CSS-like selectors.
func HandleScrapePage(ctx Context) models.ToolResponse {
	rawURL := models.ArgString(ctx.Req.Args, "url")
	if rawURL == "" {
		return models.ErrorResponse("scrape_page: missing 'url' argument")
	}

	if _, err := security.ValidateURL(rawURL); err != nil {
		return models.ErrorResponsef("scrape_page: %v", err)
	}

	fetcher := scraper.NewFetcher(ctx.Deps.HTTP)
	html, err := fetcher.FetchPage(context.Background(), rawURL)
	if err != nil {
		return models.ErrorResponsef("scrape_page: %v", err)
	}

	// Extract selectors if provided
	selectorsRaw := models.ArgString(ctx.Req.Args, "selectors")
	if selectorsRaw != "" {
		var selectors []string
		if uerr := json.Unmarshal([]byte(selectorsRaw), &selectors); uerr != nil {
			// Try comma-separated fallback
			for _, s := range strings.Split(selectorsRaw, ",") {
				s = strings.TrimSpace(s)
				if s != "" {
					selectors = append(selectors, s)
				}
			}
		}
		results := scraper.ExtractBySelector(html, selectors)
		return models.SuccessResponse(results)
	}

	// Default: full page extraction
	text := scraper.ExtractMainText(html)
	meta := scraper.ExtractMetadata(html)

	return models.SuccessResponse(map[string]interface{}{
		"title":    meta.Title,
		"text":     text,
		"links":    meta.Links,
		"headings": meta.Headings,
		"meta":     meta.Meta,
	})
}
