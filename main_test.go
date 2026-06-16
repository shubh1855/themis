package main

import "testing"

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
