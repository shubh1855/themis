package system_test

import (
	"context"
	"testing"
	"time"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/system"
)

func TestRunCmd_Success(t *testing.T) {
	result, err := system.RunCmd(context.Background(), "echo", []string{"hello"}, "", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if result.Stdout == "" {
		t.Error("expected output from echo")
	}
}

func TestRunCmd_Timeout(t *testing.T) {
	_, err := system.RunCmd(context.Background(), "sleep", []string{"10"}, "", 100*time.Millisecond)
	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestRunCmd_NonZeroExit(t *testing.T) {
	result, err := system.RunCmd(context.Background(), "false", nil, "", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode == 0 {
		t.Error("expected non-zero exit code from 'false'")
	}
}

func TestRunShellCmd(t *testing.T) {
	result, err := system.RunShellCmd(context.Background(), "echo 'shell test'", "", 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
}

func TestDetectShell(t *testing.T) {
	kind, path := system.DetectShell()
	if kind == "" {
		t.Error("expected shell kind")
	}
	if path == "" {
		t.Error("expected shell path")
	}
}
