package tools

import (
	"context"
	"strings"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/models"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/tester"
)

func HandleRunTests(ctx Context) models.ToolResponse {
	dir := models.ArgString(ctx.Req.Args, "dir")
	if dir == "" {
		dir = ctx.Deps.RootDir
	}

	argsStr := models.ArgString(ctx.Req.Args, "args")
	var args []string
	if argsStr != "" {
		args = strings.Fields(argsStr)
	}

	result, err := tester.RunTests(context.Background(), dir, args...)
	if err != nil {
		return models.ErrorResponsef("run_tests: %v", err)
	}
	return models.SuccessResponse(result)
}

func HandleRunLinter(ctx Context) models.ToolResponse {
	dir := models.ArgString(ctx.Req.Args, "dir")
	if dir == "" {
		dir = ctx.Deps.RootDir
	}

	linter := models.ArgString(ctx.Req.Args, "linter")
	argsStr := models.ArgString(ctx.Req.Args, "args")
	var args []string
	if argsStr != "" {
		args = strings.Fields(argsStr)
	}

	result, err := tester.RunLinter(context.Background(), dir, linter, args...)
	if err != nil {
		return models.ErrorResponsef("run_linter: %v", err)
	}
	return models.SuccessResponse(result)
}

func HandleCoverageReport(ctx Context) models.ToolResponse {
	dir := models.ArgString(ctx.Req.Args, "dir")
	if dir == "" {
		dir = ctx.Deps.RootDir
	}

	result, err := tester.RunCoverage(context.Background(), dir)
	if err != nil {
		return models.ErrorResponsef("coverage_report: %v", err)
	}
	return models.SuccessResponse(result)
}

func HandleBenchmarkCmd(ctx Context) models.ToolResponse {
	dir := models.ArgString(ctx.Req.Args, "dir")
	if dir == "" {
		dir = ctx.Deps.RootDir
	}

	cmd := models.ArgString(ctx.Req.Args, "command")
	argsStr := models.ArgString(ctx.Req.Args, "args")
	var args []string
	if argsStr != "" {
		args = strings.Fields(argsStr)
	}

	result, err := tester.RunBenchmark(context.Background(), dir, cmd, args)
	if err != nil {
		return models.ErrorResponsef("benchmark_cmd: %v", err)
	}
	return models.SuccessResponse(result)
}
