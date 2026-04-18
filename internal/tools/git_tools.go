package tools

import (
	"context"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/gitx"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/models"
)

func gitRepo(ctx Context) *gitx.Git {
	dir := models.ArgString(ctx.Req.Args, "dir")
	if dir == "" {
		dir = ctx.Deps.RootDir
	}
	return gitx.New(dir)
}

func HandleGitStatus(ctx Context) models.ToolResponse {
	out, err := gitRepo(ctx).Status(context.Background())
	if err != nil {
		return models.ErrorResponsef("git_status: %v", err)
	}
	return models.SuccessResponse(models.GitResult{Output: out})
}

func HandleGitDiff(ctx Context) models.ToolResponse {
	out, err := gitRepo(ctx).Diff(context.Background())
	if err != nil {
		return models.ErrorResponsef("git_diff: %v", err)
	}
	return models.SuccessResponse(models.GitResult{Output: out})
}

func HandleGitLog(ctx Context) models.ToolResponse {
	n := models.ArgInt(ctx.Req.Args, "count", 10)
	out, err := gitRepo(ctx).Log(context.Background(), n)
	if err != nil {
		return models.ErrorResponsef("git_log: %v", err)
	}
	return models.SuccessResponse(models.GitResult{Output: out})
}

func HandleGitBranch(ctx Context) models.ToolResponse {
	name := models.ArgString(ctx.Req.Args, "name")
	var args []string
	if name != "" {
		args = append(args, name)
	}

	out, err := gitRepo(ctx).Branch(context.Background(), args...)
	if err != nil {
		return models.ErrorResponsef("git_branch: %v", err)
	}
	return models.SuccessResponse(models.GitResult{Output: out})
}

func HandleGitCheckout(ctx Context) models.ToolResponse {
	target := models.ArgString(ctx.Req.Args, "target")
	if target == "" {
		return models.ErrorResponse("git_checkout: missing 'target' argument")
	}

	out, err := gitRepo(ctx).Checkout(context.Background(), target)
	if err != nil {
		return models.ErrorResponsef("git_checkout: %v", err)
	}
	return models.SuccessResponse(models.GitResult{Output: out})
}

func HandleGitCommit(ctx Context) models.ToolResponse {
	message := models.ArgString(ctx.Req.Args, "message")
	if message == "" {
		return models.ErrorResponse("git_commit: missing 'message' argument")
	}

	if models.ArgBool(ctx.Req.Args, "add_all") {
		if _, err := gitRepo(ctx).Add(context.Background(), "-A"); err != nil {
			return models.ErrorResponsef("git_commit: add: %v", err)
		}
	}

	out, err := gitRepo(ctx).Commit(context.Background(), message)
	if err != nil {
		return models.ErrorResponsef("git_commit: %v", err)
	}
	return models.SuccessResponse(models.GitResult{Output: out})
}

func HandleGitClone(ctx Context) models.ToolResponse {
	url := models.ArgString(ctx.Req.Args, "url")
	dir := models.ArgString(ctx.Req.Args, "dir")
	if url == "" {
		return models.ErrorResponse("git_clone: missing 'url' argument")
	}
	if dir == "" {
		dir = ctx.Deps.RootDir
	}

	out, err := gitx.Clone(context.Background(), url, dir)
	if err != nil {
		return models.ErrorResponsef("git_clone: %v", err)
	}
	return models.SuccessResponse(models.GitResult{Output: out})
}
