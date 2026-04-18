package registry_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/cache"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/httpx"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/registry"
)

func testBase(t *testing.T) *registry.BaseClient {
	t.Helper()
	client := httpx.NewClient(httpx.WithSSRFAllowPrivate())
	memCache := cache.NewMemory(5*time.Minute, 1*time.Minute)
	t.Cleanup(memCache.Stop)
	return registry.NewBaseClient(client, memCache)
}

func TestNPMLookup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"name":        "express",
			"description": "Fast web framework",
			"license":     "MIT",
			"homepage":    "https://expressjs.com",
			"keywords":    []string{"web", "framework"},
			"dist-tags": map[string]string{
				"latest": "4.18.2",
			},
			"repository": map[string]string{
				"url": "https://github.com/expressjs/express",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := httpx.NewClient(httpx.WithSSRFAllowPrivate())
	body, err := client.GetBody(t.Context(), server.URL+"/express")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if body == "" {
		t.Error("expected non-empty body")
	}

	var pkg map[string]interface{}
	if err := json.Unmarshal([]byte(body), &pkg); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if pkg["name"] != "express" {
		t.Errorf("expected 'express', got %v", pkg["name"])
	}
}

func TestPyPILookup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"info": map[string]interface{}{
				"name":      "requests",
				"version":   "2.31.0",
				"summary":   "Python HTTP library",
				"home_page": "https://requests.readthedocs.io",
				"license":   "Apache 2.0",
				"keywords":  "http,requests",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := httpx.NewClient(httpx.WithSSRFAllowPrivate())
	body, err := client.GetBody(t.Context(), server.URL+"/requests/json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var pkg map[string]interface{}
	if err := json.Unmarshal([]byte(body), &pkg); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	info, ok := pkg["info"].(map[string]interface{})
	if !ok {
		t.Fatal("expected info object")
	}
	if info["name"] != "requests" {
		t.Errorf("expected 'requests', got %v", info["name"])
	}
}

func TestBaseClient_Creation(t *testing.T) {
	base := testBase(t)
	if base.HTTP == nil {
		t.Error("HTTP client not set")
	}
	if base.Cache == nil {
		t.Error("Cache not set")
	}
}
