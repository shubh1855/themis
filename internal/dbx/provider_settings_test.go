package dbx

import (
	"context"
	"testing"
)

func TestProviderSettings(t *testing.T) {
	ctx := context.Background()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := db.InitSettings(ctx); err != nil {
		t.Fatalf("InitSettings() error = %v", err)
	}

	if _, ok, err := db.LoadProviderConfig(ctx); err != nil || ok {
		t.Fatalf("LoadProviderConfig() empty ok = %v err = %v, want ok=false err=nil", ok, err)
	}

	first := ProviderConfigRow{
		Provider: "ollama",
		APIKey:   "",
		BaseURL:  "http://localhost:11434",
		Model:    "qwen2.5-coder:7b",
	}
	if err := db.SaveProviderConfig(ctx, first); err != nil {
		t.Fatalf("SaveProviderConfig() error = %v", err)
	}
	assertProviderConfig(t, db, first)

	second := ProviderConfigRow{
		Provider: "openai",
		APIKey:   "test-key",
		BaseURL:  "https://example.com/v1",
		Model:    "gpt-test",
	}
	if err := db.SaveProviderConfig(ctx, second); err != nil {
		t.Fatalf("SaveProviderConfig() overwrite error = %v", err)
	}
	assertProviderConfig(t, db, second)
}

func assertProviderConfig(t *testing.T, db *DB, want ProviderConfigRow) {
	t.Helper()
	got, ok, err := db.LoadProviderConfig(context.Background())
	if err != nil {
		t.Fatalf("LoadProviderConfig() error = %v", err)
	}
	if !ok {
		t.Fatalf("LoadProviderConfig() ok = false, want true")
	}
	if got != want {
		t.Fatalf("LoadProviderConfig() = %#v, want %#v", got, want)
	}
}
