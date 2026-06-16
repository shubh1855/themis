package llm

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

const defaultOllamaBaseURL = "http://localhost:11434"
const defaultAnthropicBaseURL = "https://litellm-proxy-93ef.onrender.com/v1"
const ollamaHealthTimeout = 2 * time.Second

type ProviderConfig struct {
	Provider string `json:"provider"`
	APIKey   string `json:"api_key"`
	BaseURL  string `json:"base_url"`
	Model    string `json:"model"`
}

func BuildClient(cfg ProviderConfig) (*openai.Client, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
	case "ollama":
		base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
		if base == "" {
			base = defaultOllamaBaseURL
		}
		ocfg := openai.DefaultConfig("ollama")
		ocfg.BaseURL = base + "/v1"
		return openai.NewClientWithConfig(ocfg), nil
	case "openai":
		if strings.TrimSpace(cfg.APIKey) == "" {
			return nil, fmt.Errorf("openai api key is required")
		}
		ocfg := openai.DefaultConfig(cfg.APIKey)
		if base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"); base != "" {
			ocfg.BaseURL = base
		}
		return openai.NewClientWithConfig(ocfg), nil
	case "anthropic":
		if strings.TrimSpace(cfg.APIKey) == "" {
			return nil, fmt.Errorf("anthropic api key is required")
		}
		ocfg := openai.DefaultConfig(cfg.APIKey)
		if base := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"); base != "" {
			ocfg.BaseURL = base
		} else {
			ocfg.BaseURL = defaultAnthropicBaseURL
		}
		return openai.NewClientWithConfig(ocfg), nil
	default:
		return nil, fmt.Errorf("unknown provider %q", cfg.Provider)
	}
}

func OllamaHealth(ctx context.Context, baseURL string) error {
	client := newOllamaClientWithHTTPClient(baseURL, &http.Client{Timeout: ollamaHealthTimeout})
	if _, err := client.ListModels(ctx); err != nil {
		return fmt.Errorf("ollama health: %w", err)
	}
	return nil
}

func ListOllamaModels(ctx context.Context, baseURL string) ([]string, error) {
	client := newOllamaClient(baseURL)
	list, err := client.ListModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("ollama list models: %w", err)
	}
	models := make([]string, 0, len(list.Models))
	for _, model := range list.Models {
		if model.ID != "" {
			models = append(models, model.ID)
		}
	}
	return models, nil
}

func newOllamaClient(baseURL string) *openai.Client {
	return newOllamaClientWithHTTPClient(baseURL, nil)
}

func newOllamaClientWithHTTPClient(baseURL string, httpClient openai.HTTPDoer) *openai.Client {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		base = defaultOllamaBaseURL
	}
	cfg := openai.DefaultConfig("ollama")
	cfg.BaseURL = base + "/v1"
	if httpClient != nil {
		cfg.HTTPClient = httpClient
	}
	return openai.NewClientWithConfig(cfg)
}
