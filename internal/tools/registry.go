package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/auth"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/gitx"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/syntax"
)

// bgCtx is a convenience background context for synchronous tool calls.
var bgCtx = context.Background()

type ToolRequest struct {
	Tool      string `json:"tool"`
	Path      string `json:"path,omitempty"`
	Content   string `json:"content,omitempty"`
	OldString string `json:"old_string,omitempty"`
	NewString string `json:"new_string,omitempty"`
	Agent     string `json:"agent,omitempty"`    // for delegate_task
	Key       string `json:"key,omitempty"`
	// Git / GitHub tool fields
	Dir    string `json:"dir,omitempty"`
	Remote string `json:"remote,omitempty"`    // git_push
	Branch string `json:"branch,omitempty"`    // git_push, git_checkout_new_branch
	Target string `json:"target,omitempty"`    // git_checkout
	Name   string `json:"name,omitempty"`      // git_branch create
	Count  int    `json:"count,omitempty"`     // git_log
	URL    string `json:"url,omitempty"`       // git_clone
	Paths  string `json:"paths,omitempty"`     // git_add  (file glob)
	AddAll bool   `json:"add_all,omitempty"`  // git_commit
	Title  string `json:"title,omitempty"`     // git_create_pr
	Body   string `json:"body,omitempty"`      // git_create_pr
	Base   string `json:"base,omitempty"`      // git_create_pr
	Head   string `json:"head,omitempty"`      // git_create_pr
	Message string `json:"message,omitempty"` // git_commit (alias for Content)
}

type ToolResult struct {
	Success bool
	Output  string
	ExecCmd *exec.Cmd
	Cleanup func()
}

type Registry struct {
	FS     *FS
	Memory map[string]string
}

func NewRegistry(fs *FS) *Registry {
	return &Registry{FS: fs, Memory: make(map[string]string)}
}

func NeedsReview(tool string) bool {
	switch tool {
	case "create_file", "write_file", "append_file", "edit_file", "run_file":
		return true
	}
	return false
}

func (r *Registry) Preview(req ToolRequest) string {
	switch req.Tool {
	case "create_file":
		return syntax.DiffView("", req.Content, req.Path)
	case "write_file":
		old, _ := r.FS.ReadFile(req.Path)
		return syntax.DiffView(old, req.Content, req.Path)
	case "append_file":
		return syntax.DiffView("", req.Content, req.Path)
	case "edit_file":
		old, _ := r.FS.ReadFile(req.Path)
		if old == "" {
			return ""
		}
		newContent := strings.Replace(old, req.OldString, req.NewString, 1)
		return syntax.DiffView(old, newContent, req.Path)
	case "run_file":
		_, shell := DetectShell()
		return "  $ " + shell + " " + req.Path
	}
	return ""
}

func (r *Registry) Execute(req ToolRequest) ToolResult {
	switch req.Tool {

	case "read_file":
		out, err := r.FS.ReadFile(req.Path)
		if err != nil {
			return fail(err)
		}
		return ok(syntax.Highlight(out, req.Path))

	case "write_file":
		if err := r.FS.WriteFile(req.Path, req.Content); err != nil {
			return fail(err)
		}
		return ok("wrote " + req.Path)

	case "append_file":
		if err := r.FS.AppendFile(req.Path, req.Content); err != nil {
			return fail(err)
		}
		return ok("appended → " + req.Path)

	case "create_file":
		if err := r.FS.CreateFile(req.Path, req.Content); err != nil {
			return fail(err)
		}
		return ok("created " + req.Path)

	case "edit_file":
		if err := r.FS.EditFile(req.Path, req.OldString, req.NewString); err != nil {
			return fail(err)
		}
		return ok("edited " + req.Path)

	case "mkdir":
		if err := r.FS.Mkdir(req.Path); err != nil {
			return fail(err)
		}
		return ok("created dir " + req.Path)

	case "store_memory":
		if req.Key == "" {
			return fail(errors.New("store_memory requires a key"))
		}
		r.Memory[req.Key] = req.Content
		return ok(fmt.Sprintf("stored key %q (%d chars)", req.Key, len(req.Content)))

	case "retrieve_memory":
		if req.Key == "" {
			return fail(errors.New("retrieve_memory requires a key"))
		}
		val, exists := r.Memory[req.Key]
		if !exists {
			return fail(fmt.Errorf("key not found: %q", req.Key))
		}
		return ok(val)

	case "run_file":
		cmd, cleanup, err := r.FS.BuildRunCmd(req.Path, req.Content)
		if err != nil {
			return fail(err)
		}
		return ToolResult{Success: true, ExecCmd: cmd, Cleanup: cleanup}

	// ── Git tools ────────────────────────────────────────────────────────────
	case "git_status":
		out, err := gitx.New(r.FS.Root).Status(bgCtx)
		if err != nil {
			return fail(err)
		}
		if out == "" {
			out = "working tree clean"
		}
		return ok(out)

	case "git_diff":
		out, err := gitx.New(r.FS.Root).Diff(bgCtx)
		if err != nil {
			return fail(err)
		}
		if out == "" {
			out = "no changes"
		}
		return ok(out)

	case "git_log":
		n := req.Count
		if n <= 0 {
			n = 10
		}
		out, err := gitx.New(r.FS.Root).Log(bgCtx, n)
		if err != nil {
			return fail(err)
		}
		return ok(out)

	case "git_branch":
		var args []string
		if req.Name != "" {
			args = append(args, req.Name)
		}
		out, err := gitx.New(r.FS.Root).Branch(bgCtx, args...)
		if err != nil {
			return fail(err)
		}
		return ok(out)

	case "git_checkout":
		if req.Target == "" {
			return fail(fmt.Errorf("git_checkout: missing 'target' field"))
		}
		out, err := gitx.New(r.FS.Root).Checkout(bgCtx, req.Target)
		if err != nil {
			return fail(err)
		}
		return ok(out)

	case "git_checkout_new_branch":
		if req.Branch == "" {
			return fail(fmt.Errorf("git_checkout_new_branch: missing 'branch' field"))
		}
		out, err := gitx.New(r.FS.Root).CheckoutNewBranch(bgCtx, req.Branch)
		if err != nil {
			return fail(err)
		}
		if out == "" {
			out = fmt.Sprintf("switched to new branch '%s'", req.Branch)
		}
		return ok(out)

	case "git_add":
		paths := req.Paths
		if paths == "" {
			paths = "-A"
		}
		out, err := gitx.New(r.FS.Root).Add(bgCtx, paths)
		if err != nil {
			return fail(err)
		}
		if out == "" {
			out = "staged files successfully"
		}
		return ok(out)

	case "git_commit":
		msg := req.Message
		if msg == "" {
			msg = req.Content
		}
		if msg == "" {
			return fail(fmt.Errorf("git_commit: missing 'message' field"))
		}
		g := gitx.New(r.FS.Root)
		if req.AddAll {
			if _, err := g.Add(bgCtx, "-A"); err != nil {
				return fail(fmt.Errorf("git_commit add: %w", err))
			}
		}
		out, err := g.Commit(bgCtx, msg)
		if err != nil {
			return fail(err)
		}
		return ok(out)

	case "git_clone":
		if req.URL == "" {
			return fail(fmt.Errorf("git_clone: missing 'url' field"))
		}
		dir := req.Dir
		if dir == "" {
			dir = r.FS.Root
		}
		out, err := gitx.Clone(bgCtx, req.URL, dir)
		if err != nil {
			return fail(err)
		}
		return ok(out)

	case "git_push":
		if !auth.IsLoggedIn() {
			return fail(fmt.Errorf("🔐 Not authenticated. Run github_login first"))
		}
		g := gitx.New(r.FS.Root)
		status, _ := g.Status(bgCtx)
		if status != "" {
			return fail(fmt.Errorf("git_push: dirty working tree — commit changes first:\n%s", status))
		}
		remote := req.Remote
		if remote == "" {
			remote = "origin"
		}
		out, err := g.Push(bgCtx, remote, req.Branch)
		if err != nil {
			return fail(err)
		}
		if out == "" {
			out = "pushed successfully"
		}
		return ok(out)

	case "git_create_pr":
		if !auth.IsLoggedIn() {
			return fail(fmt.Errorf("🔐 Not authenticated. Run github_login first"))
		}
		if req.Title == "" {
			return fail(fmt.Errorf("git_create_pr: missing 'title' field"))
		}
		body := req.Body
		if body == "" {
			body = req.Title
		}
		g := gitx.New(r.FS.Root)
		diff, _ := g.Diff(bgCtx, "--stat")
		if diff != "" {
			body += "\n\n### Diff Summary\n```\n" + diff + "\n```"
		}
		base := req.Base
		if base == "" {
			base = "main"
		}
		out, err := g.CreatePR(bgCtx, req.Title, body, base, req.Head)
		if err != nil {
			return fail(err)
		}
		return ok("PR created: " + out)

	// ── GitHub auth tools ─────────────────────────────────────────────────────
	case "github_status":
		if !auth.IsLoggedIn() {
			return ok("❌ Not authenticated. Run github_login to sign in via GitHub OAuth.")
		}
		tok, _ := auth.LoadToken()
		user := tok.Username
		if user == "" {
			user = "authenticated user"
		}
		return ok(fmt.Sprintf("✅ Logged in as %s", user))

	case "github_login":
		if auth.IsLoggedIn() {
			tok, _ := auth.LoadToken()
			if tok.Username != "" {
				return ok(fmt.Sprintf("Already logged in as %s", tok.Username))
			}
			return ok("Already authenticated with GitHub")
		}
		// Run the Device Flow — this blocks until user completes auth in browser
		instructions, _, err := auth.Login(bgCtx)
		if err != nil {
			if instructions != "" {
				return fail(fmt.Errorf("%v (instructions: %s)", err, instructions))
			}
			return fail(err)
		}
		tok, _ := auth.LoadToken()
		user := tok.Username
		if user == "" {
			user = "GitHub user"
		}
		return ok(fmt.Sprintf("✅ Logged in as %s — %s", user, instructions))

	case "github_logout":
		if err := auth.Logout(); err != nil {
			return fail(err)
		}
		return ok("🔓 Logged out of GitHub")

	default:
		return fail(errors.New("unknown tool: " + req.Tool))
	}
}

func (r *Registry) ExecuteJSON(raw string) ToolResult {
	var req ToolRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		return fail(err)
	}
	return r.Execute(req)
}

func ok(msg string) ToolResult  { return ToolResult{Success: true, Output: msg} }
func fail(err error) ToolResult { return ToolResult{Success: false, Output: err.Error()} }
