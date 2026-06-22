package llm

import (
	"context"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

func TestOllamaHealth(t *testing.T) {
	if os.Getenv("OLLAMA_INTEGRATION") != "1" {
		t.Skip("set OLLAMA_INTEGRATION=1 to run Ollama integration tests")
	}

	baseURL := os.Getenv("OLLAMA_BASE_URL")
	if baseURL == "" {
		baseURL = defaultOllamaBaseURL
	}

	t.Run("daemon running", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if err := OllamaHealth(ctx, baseURL); err != nil {
			t.Fatalf("OllamaHealth(%q) error = %v, want nil", baseURL, err)
		}
	})

	t.Run("daemon not running", func(t *testing.T) {
		deadBaseURL := unusedLocalBaseURL(t)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err := OllamaHealth(ctx, deadBaseURL)
		if err == nil {
			t.Fatalf("OllamaHealth(%q) error = nil, want non-nil", deadBaseURL)
		}
		if !strings.Contains(err.Error(), "ollama health") {
			t.Fatalf("OllamaHealth(%q) error = %v, want meaningful ollama health error", deadBaseURL, err)
		}
	})
}

func unusedLocalBaseURL(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on local port: %v", err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("close local listener: %v", err)
	}
	return "http://" + addr
}

func TestBuildClient(t *testing.T) {
	tests := []struct {
		name       string
		cfg        ProviderConfig
		wantClient bool
		wantErr    bool
	}{
		{
			name:       "anthropic with key",
			cfg:        ProviderConfig{Provider: "anthropic", APIKey: "test-key"},
			wantClient: true,
		},
		{
			name:       "openai with key",
			cfg:        ProviderConfig{Provider: "openai", APIKey: "test-key"},
			wantClient: true,
		},
		{
			name:       "ollama with base url",
			cfg:        ProviderConfig{Provider: "ollama", BaseURL: "http://localhost:11434"},
			wantClient: true,
		},
		{
			name:    "unknown provider",
			cfg:     ProviderConfig{Provider: "unknown"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := BuildClient(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("BuildClient() error = %v, wantErr %v", err, tt.wantErr)
			}
			if (client != nil) != tt.wantClient {
				t.Fatalf("BuildClient() client nil = %v, wantClient %v", client == nil, tt.wantClient)
			}
		})
	}
}
