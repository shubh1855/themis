package tester

import (
	"context"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/models"
)

// RunBenchmark runs a benchmark command in the given directory.
func RunBenchmark(ctx context.Context, dir string, cmd string, args []string) (*models.TestResult, error) {
	if cmd == "" {
		cmd = "go"
		args = []string{"test", "-bench=.", "-benchmem", "./..."}
	}
	return RunCommand(ctx, cmd, args, dir, testTimeout)
}
