package tester

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/models"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/system"
)

const testTimeout = 120 * time.Second

func RunTests(ctx context.Context, dir string, args ...string) (*models.TestResult, error) {
	cmd, cmdArgs := detectTestCommand(dir)
	cmdArgs = append(cmdArgs, args...)

	result, err := system.RunCmd(ctx, cmd, cmdArgs, dir, testTimeout)
	if err != nil {
		return &models.TestResult{
			Passed: false,
			Output: fmt.Sprintf("Error: %v\n%s%s", err, result.Stdout, result.Stderr),
		}, nil
	}

	return &models.TestResult{
		Passed:  result.ExitCode == 0,
		Output:  result.Stdout + result.Stderr,
		Summary: summarizeTests(result.Stdout),
	}, nil
}

func RunCommand(ctx context.Context, cmd string, args []string, dir string, timeout time.Duration) (*models.TestResult, error) {
	if timeout == 0 {
		timeout = testTimeout
	}
	result, err := system.RunCmd(ctx, cmd, args, dir, timeout)
	if err != nil {
		return &models.TestResult{
			Passed: false,
			Output: fmt.Sprintf("Error: %v\n%s%s", err, result.Stdout, result.Stderr),
		}, nil
	}

	return &models.TestResult{
		Passed: result.ExitCode == 0,
		Output: result.Stdout + result.Stderr,
	}, nil
}

func detectTestCommand(dir string) (string, []string) {
	checks := []struct {
		file string
		cmd  string
		args []string
	}{
		{"go.mod", "go", []string{"test", "./..."}},
		{"Cargo.toml", "cargo", []string{"test"}},
		{"package.json", "npm", []string{"test"}},
		{"pytest.ini", "pytest", nil},
		{"setup.py", "python", []string{"-m", "pytest"}},
		{"pyproject.toml", "python", []string{"-m", "pytest"}},
		{"requirements.txt", "python", []string{"-m", "pytest"}},
	}

	for _, c := range checks {
		if fileExists(dir, c.file) {
			return c.cmd, c.args
		}
	}

	return "echo", []string{"no test framework detected"}
}

func fileExists(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}

func summarizeTests(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "pass") || strings.Contains(lower, "fail") || strings.Contains(lower, "ok") {
			return strings.TrimSpace(line)
		}
	}
	if len(lines) > 0 {
		return strings.TrimSpace(lines[len(lines)-1])
	}
	return ""
}
