package tester

import (
	"context"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/models"
)

// RunLinter runs a linter or format check in the given directory.
func RunLinter(ctx context.Context, dir string, linter string, args ...string) (*models.TestResult, error) {
	if linter == "" {
		linter = detectLinter(dir)
	}
	return RunCommand(ctx, linter, args, dir, testTimeout)
}

func detectLinter(dir string) string {
	if fileExists(dir, "go.mod") {
		return "golangci-lint"
	}
	if fileExists(dir, "package.json") {
		return "npx"
	}
	if fileExists(dir, "Cargo.toml") {
		return "cargo"
	}
	if fileExists(dir, "pyproject.toml") || fileExists(dir, "setup.py") {
		return "ruff"
	}
	return "echo"
}
