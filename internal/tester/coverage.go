package tester

import (
	"context"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/models"
)

// RunCoverage generates a test coverage report for the project.
func RunCoverage(ctx context.Context, dir string, args ...string) (*models.TestResult, error) {
	cmd := "go"
	cmdArgs := []string{"test", "-cover", "-coverprofile=coverage.out", "./..."}
	cmdArgs = append(cmdArgs, args...)

	if fileExists(dir, "package.json") {
		cmd = "npx"
		cmdArgs = []string{"jest", "--coverage"}
	} else if fileExists(dir, "Cargo.toml") {
		cmd = "cargo"
		cmdArgs = []string{"tarpaulin", "--out", "Stdout"}
	} else if fileExists(dir, "pyproject.toml") || fileExists(dir, "setup.py") {
		cmd = "python"
		cmdArgs = []string{"-m", "pytest", "--cov", "."}
	}

	return RunCommand(ctx, cmd, cmdArgs, dir, testTimeout)
}
