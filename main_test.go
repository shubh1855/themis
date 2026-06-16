package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/dbx"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/llm"
)

func TestInitDBFreshAnthropicFallback(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("ANTHROPIC_API_KEY", "test-anthropic-key")

	msg, ok := initDB().(dbReadyMsg)
	if !ok {
		t.Fatalf("initDB() returned %T, want dbReadyMsg", msg)
	}
	t.Cleanup(func() {
		if msg.db != nil {
			_ = msg.db.Close()
		}
	})

	if msg.err != nil {
		t.Fatalf("initDB() error = %v", msg.err)
	}
	if msg.providerCfg.Provider != "anthropic" {
		t.Fatalf("provider = %q, want anthropic", msg.providerCfg.Provider)
	}
	if msg.providerCfg.APIKey != "test-anthropic-key" {
		t.Fatalf("api key = %q, want env ANTHROPIC_API_KEY", msg.providerCfg.APIKey)
	}
	if llm.GetActiveClient() == nil {
		t.Fatal("active LLM client is nil")
	}
	if llm.GetReactModel() != msg.providerCfg.Model {
		t.Fatalf("react model = %q, want provider model %q", llm.GetReactModel(), msg.providerCfg.Model)
	}
}

func TestInitDBSavedOllamaConfig(t *testing.T) {
	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("ANTHROPIC_API_KEY", "")

	wantCfg := dbx.ProviderConfigRow{
		Provider: "ollama",
		BaseURL:  "http://localhost:11434",
		Model:    "qwen2.5-coder:7b",
	}
	writeProviderConfigForTest(t, dataHome, wantCfg)

	msg, ok := initDB().(dbReadyMsg)
	if !ok {
		t.Fatalf("initDB() returned %T, want dbReadyMsg", msg)
	}
	t.Cleanup(func() {
		if msg.db != nil {
			_ = msg.db.Close()
		}
	})

	if msg.err != nil {
		t.Fatalf("initDB() error = %v", msg.err)
	}
	if msg.providerCfg.Provider != wantCfg.Provider {
		t.Fatalf("provider = %q, want %q", msg.providerCfg.Provider, wantCfg.Provider)
	}
	if msg.providerCfg.BaseURL != wantCfg.BaseURL {
		t.Fatalf("base url = %q, want %q", msg.providerCfg.BaseURL, wantCfg.BaseURL)
	}
	if msg.providerCfg.Model != wantCfg.Model {
		t.Fatalf("model = %q, want %q", msg.providerCfg.Model, wantCfg.Model)
	}
	if llm.GetActiveClient() == nil {
		t.Fatal("active LLM client is nil")
	}
	if llm.GetReactModel() != wantCfg.Model {
		t.Fatalf("react model = %q, want %q", llm.GetReactModel(), wantCfg.Model)
	}
}

func TestInitDBUnknownProviderFatal(t *testing.T) {
	if os.Getenv("THEMIS_TEST_UNKNOWN_PROVIDER_FATAL") == "1" {
		dataHome := os.Getenv("THEMIS_TEST_XDG_DATA_HOME")
		if dataHome == "" {
			panic("THEMIS_TEST_XDG_DATA_HOME is required")
		}
		writeProviderConfigForTest(t, dataHome, dbx.ProviderConfigRow{Provider: "unknown", Model: "bad-model"})
		_ = initDB()
		os.Exit(0)
	}

	dataHome := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run", "^TestInitDBUnknownProviderFatal$")
	cmd.Env = append(os.Environ(),
		"THEMIS_TEST_UNKNOWN_PROVIDER_FATAL=1",
		"THEMIS_TEST_XDG_DATA_HOME="+dataHome,
		"XDG_DATA_HOME="+dataHome,
	)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("subprocess exited nil error, want fatal exit; output:\n%s", out)
	}
	if !strings.Contains(string(out), "unknown") {
		t.Fatalf("fatal output = %q, want provider name unknown", out)
	}
}

func writeProviderConfigForTest(t *testing.T, dataHome string, cfg dbx.ProviderConfigRow) {
	t.Helper()

	dataDir := filepath.Join(dataHome, "themis")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("create data dir: %v", err)
	}
	db, err := dbx.Open(filepath.Join(dataDir, "data.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := db.InitSettings(ctx); err != nil {
		t.Fatalf("init settings: %v", err)
	}
	if err := db.SaveProviderConfig(ctx, cfg); err != nil {
		t.Fatalf("save provider config: %v", err)
	}
}

func TestOllamaModelsMsgSelectsAvailableModel(t *testing.T) {
	m := model{}
	m.modelInput.SetValue("google/gemma-4-31B-it")

	updated, _ := m.Update(OllamaModelsMsg{Models: []string{"qwen2.5-coder:7b", "llama3.2:latest"}})
	got := updated.(model)

	if got.modelInput.Value() != "qwen2.5-coder:7b" {
		t.Fatalf("modelInput = %q, want first available Ollama model", got.modelInput.Value())
	}
	if got.modelListIdx != 0 {
		t.Fatalf("modelListIdx = %d, want 0", got.modelListIdx)
	}
}

func TestOllamaModelsMsgPreservesSelectedModel(t *testing.T) {
	m := model{}
	m.modelInput.SetValue("llama3.2:latest")

	updated, _ := m.Update(OllamaModelsMsg{Models: []string{"qwen2.5-coder:7b", "llama3.2:latest"}})
	got := updated.(model)

	if got.modelInput.Value() != "llama3.2:latest" {
		t.Fatalf("modelInput = %q, want existing selected Ollama model", got.modelInput.Value())
	}
	if got.modelListIdx != 1 {
		t.Fatalf("modelListIdx = %d, want 1", got.modelListIdx)
	}
}

func TestOllamaHealthErrorClearsStaleModels(t *testing.T) {
	m := model{modelList: []string{"qwen2.5-coder:7b"}, modelListIdx: 0}

	updated, _ := m.Update(OllamaHealthMsg{Err: errTestOllamaUnreachable{}})
	got := updated.(model)

	if len(got.modelList) != 0 {
		t.Fatalf("modelList length = %d, want 0", len(got.modelList))
	}
	if got.modelListIdx != 0 {
		t.Fatalf("modelListIdx = %d, want 0", got.modelListIdx)
	}
	if got.settingsError == "" {
		t.Fatal("settingsError is empty, want health error message")
	}
}

type errTestOllamaUnreachable struct{}

func (errTestOllamaUnreachable) Error() string { return "ollama health: unreachable" }
