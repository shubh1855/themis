package tools

import (
	"context"
	"fmt"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/auth"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/gitx"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/models"
)

func requireAuth() error {
	if !auth.IsLoggedIn() {
		return fmt.Errorf("not authenticated — call github_login first")
	}
	return nil
}

// HandleGitHubLogin starts the GitHub Device Flow and blocks until the user authorizes.
func HandleGitHubLogin(ctx Context) models.ToolResponse {
	instructions, _, err := auth.Login(context.Background())
	if err != nil {
		if instructions != "" {
			return models.ErrorResponsef("github_login: %v\n\nTo authenticate: %s", err, instructions)
		}
		return models.ErrorResponsef("github_login: %v", err)
	}
	return models.SuccessResponse(map[string]string{
		"status":       "authenticated",
		"message":      "GitHub authentication successful",
		"instructions": instructions,
	})
}

// HandleGitHubLogout removes stored GitHub credentials.
func HandleGitHubLogout(ctx Context) models.ToolResponse {
	if err := auth.Logout(); err != nil {
		return models.ErrorResponsef("github_logout: %v", err)
	}
	return models.SuccessResponse(map[string]string{"message": "Logged out of GitHub"})
}

// HandleGitHubStatus reports current GitHub authentication status.
func HandleGitHubStatus(ctx Context) models.ToolResponse {
	if !auth.IsLoggedIn() {
		return models.SuccessResponse(map[string]string{"status": "not authenticated"})
	}
	tok, err := auth.LoadToken()
	if err != nil {
		return models.ErrorResponsef("github_status: %v", err)
	}
	user := tok.Username
	if user == "" {
		user = "authenticated"
	}
	return models.SuccessResponse(map[string]string{
		"status":   "authenticated",
		"username": user,
		"scope":    tok.Scope,
	})
}

// HandleGitAdd stages files for commit.
func HandleGitAdd(ctx Context) models.ToolResponse {
	paths := models.ArgString(ctx.Req.Args, "paths")
	if paths == "" {
		paths = "-A"
	}
	repo := gitx.New(ctx.Deps.RootDir)
	out, err := repo.Add(context.Background(), paths)
	if err != nil {
		return models.ErrorResponsef("git_add: %v", err)
	}
	return models.SuccessResponse(models.GitResult{Output: out})
}

// HandleGitPush pushes a branch to a remote. Requires GitHub auth.
func HandleGitPush(ctx Context) models.ToolResponse {
	if err := requireAuth(); err != nil {
		return models.ErrorResponse(err.Error())
	}

	remote := models.ArgString(ctx.Req.Args, "remote")
	branch := models.ArgString(ctx.Req.Args, "branch")
	if remote == "" {
		remote = "origin"
	}
	if branch == "" {
		return models.ErrorResponse("git_push: missing 'branch' argument")
	}

	repo := gitx.New(ctx.Deps.RootDir)
	status, err := repo.Status(context.Background())
	if err != nil {
		return models.ErrorResponsef("git_push: status check: %v", err)
	}
	if status != "" {
		return models.ErrorResponse("git_push: working tree is dirty — commit or stash changes first")
	}

	out, err := repo.Push(context.Background(), remote, branch)
	if err != nil {
		return models.ErrorResponsef("git_push: %v", err)
	}
	return models.SuccessResponse(models.GitResult{Output: out})
}

// HandleGitCreatePR creates a GitHub pull request via gh CLI. Requires GitHub auth.
func HandleGitCreatePR(ctx Context) models.ToolResponse {
	if err := requireAuth(); err != nil {
		return models.ErrorResponse(err.Error())
	}

	title := models.ArgString(ctx.Req.Args, "title")
	body := models.ArgString(ctx.Req.Args, "body")
	base := models.ArgString(ctx.Req.Args, "base")
	head := models.ArgString(ctx.Req.Args, "head")

	if title == "" {
		return models.ErrorResponse("git_create_pr: missing 'title' argument")
	}
	if base == "" {
		base = "main"
	}
	if head == "" {
		return models.ErrorResponse("git_create_pr: missing 'head' argument")
	}

	repo := gitx.New(ctx.Deps.RootDir)

	diff, _ := repo.Diff(context.Background(), "--stat")
	if diff != "" && body != "" {
		body = body + "\n\n### Diff Summary\n```\n" + diff + "\n```"
	}

	out, err := repo.CreatePR(context.Background(), title, body, base, head)
	if err != nil {
		return models.ErrorResponsef("git_create_pr: %v", err)
	}
	return models.SuccessResponse(models.GitResult{Output: out})
}

// HandleGitCheckoutNewBranch creates and switches to a new branch.
func HandleGitCheckoutNewBranch(ctx Context) models.ToolResponse {
	branch := models.ArgString(ctx.Req.Args, "branch")
	if branch == "" {
		return models.ErrorResponse("git_checkout_new_branch: missing 'branch' argument")
	}

	repo := gitx.New(ctx.Deps.RootDir)
	out, err := repo.CheckoutNewBranch(context.Background(), branch)
	if err != nil {
		return models.ErrorResponsef("git_checkout_new_branch: %v", err)
	}
	return models.SuccessResponse(models.GitResult{Output: out})
}
