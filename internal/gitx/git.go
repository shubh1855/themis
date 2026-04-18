// Package gitx provides safe wrappers around git CLI operations.
package gitx

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/system"
)

const gitTimeout = 30 * time.Second

// Git wraps git operations for a specific repository directory.
type Git struct {
	Dir string
}

// New creates a Git wrapper for the given directory.
func New(dir string) *Git {
	return &Git{Dir: dir}
}

func (g *Git) run(ctx context.Context, args ...string) (string, error) {
	result, err := system.RunCmd(ctx, "git", args, g.Dir, gitTimeout)
	if err != nil {
		return "", fmt.Errorf("gitx: %w", err)
	}
	if result.ExitCode != 0 {
		return "", fmt.Errorf("gitx: git %s: %s", args[0], strings.TrimSpace(result.Stderr))
	}
	return strings.TrimSpace(result.Stdout), nil
}

// Status returns the output of git status --porcelain.
func (g *Git) Status(ctx context.Context) (string, error) {
	return g.run(ctx, "status", "--porcelain")
}

// Diff returns the output of git diff.
func (g *Git) Diff(ctx context.Context, args ...string) (string, error) {
	cmdArgs := append([]string{"diff"}, args...)
	return g.run(ctx, cmdArgs...)
}

// Log returns recent git log entries.
func (g *Git) Log(ctx context.Context, n int) (string, error) {
	return g.run(ctx, "log", fmt.Sprintf("-n%d", n), "--oneline")
}

// Branch lists branches or creates a new branch.
func (g *Git) Branch(ctx context.Context, args ...string) (string, error) {
	cmdArgs := append([]string{"branch"}, args...)
	return g.run(ctx, cmdArgs...)
}

// Checkout switches branches or restores files.
func (g *Git) Checkout(ctx context.Context, target string, args ...string) (string, error) {
	cmdArgs := append([]string{"checkout", target}, args...)
	return g.run(ctx, cmdArgs...)
}

// Commit creates a commit with the given message.
func (g *Git) Commit(ctx context.Context, message string, args ...string) (string, error) {
	cmdArgs := append([]string{"commit", "-m", message}, args...)
	return g.run(ctx, cmdArgs...)
}

// Clone clones a repository to the given directory.
func Clone(ctx context.Context, url, dir string) (string, error) {
	result, err := system.RunCmd(ctx, "git", []string{"clone", url, dir}, "", 120*time.Second)
	if err != nil {
		return "", fmt.Errorf("gitx: clone: %w", err)
	}
	if result.ExitCode != 0 {
		return "", fmt.Errorf("gitx: clone: %s", strings.TrimSpace(result.Stderr))
	}
	return strings.TrimSpace(result.Stdout), nil
}

// Add stages files for commit.
func (g *Git) Add(ctx context.Context, paths ...string) (string, error) {
	cmdArgs := append([]string{"add"}, paths...)
	return g.run(ctx, cmdArgs...)
}
