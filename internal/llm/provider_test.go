package llm

import "testing"

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
