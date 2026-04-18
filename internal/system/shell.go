// Package system provides shell execution and process management utilities.
package system

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/models"
)

const (
	// DefaultCmdTimeout is the default timeout for command execution.
	DefaultCmdTimeout = 60 * time.Second
	// MaxOutputSize limits captured output to 1MB.
	MaxOutputSize = 1 * 1024 * 1024
)

// ShellKind represents the type of shell to use.
type ShellKind string

const (
	ShellBash ShellKind = "bash"
	ShellZsh  ShellKind = "zsh"
	ShellFish ShellKind = "fish"
	ShellSh   ShellKind = "sh"
)

// DetectShell reads $SHELL and returns the shell kind and binary path.
func DetectShell() (ShellKind, string) {
	shellPath := os.Getenv("SHELL")
	if shellPath == "" {
		return ShellBash, "bash"
	}
	base := filepath.Base(shellPath)
	switch {
	case strings.Contains(base, "zsh"):
		return ShellZsh, shellPath
	case strings.Contains(base, "fish"):
		return ShellFish, shellPath
	case strings.Contains(base, "bash"):
		return ShellBash, shellPath
	default:
		return ShellSh, shellPath
	}
}

// RunCmd executes a command with a timeout and returns the result.
func RunCmd(ctx context.Context, command string, args []string, dir string, timeout time.Duration) (*models.ProcessResult, error) {
	if timeout == 0 {
		timeout = DefaultCmdTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = dir
	cmd.Env = os.Environ()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := &models.ProcessResult{
		Stdout: truncateOutput(stdout.String()),
		Stderr: truncateOutput(stderr.String()),
	}

	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return result, fmt.Errorf("system: command timed out after %v", timeout)
		}
		// Non-zero exit is not an error for us, just record the exit code
		if _, ok := err.(*exec.ExitError); ok {
			return result, nil
		}
		return result, fmt.Errorf("system: run %q: %w", command, err)
	}

	return result, nil
}

// RunShellCmd executes a command string through the user's shell.
func RunShellCmd(ctx context.Context, cmdStr, dir string, timeout time.Duration) (*models.ProcessResult, error) {
	_, shellBin := DetectShell()
	return RunCmd(ctx, shellBin, []string{"-c", cmdStr}, dir, timeout)
}

func truncateOutput(s string) string {
	if len(s) > MaxOutputSize {
		return s[:MaxOutputSize] + "\n... [output truncated]"
	}
	return s
}
