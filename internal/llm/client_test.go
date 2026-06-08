package llm

import "testing"

func TestReactModelAccessors(t *testing.T) {
	original := GetReactModel()
	t.Cleanup(func() { SetReactModel(original) })

	SetReactModel("test-model")
	if got := GetReactModel(); got != "test-model" {
		t.Fatalf("GetReactModel() = %q, want %q", got, "test-model")
	}

	SetReactModel("")
	if got := GetReactModel(); got != "test-model" {
		t.Fatalf("GetReactModel() after empty set = %q, want %q", got, "test-model")
	}
}
