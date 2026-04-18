package tools

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/syntax"
)

type ToolRequest struct {
	Tool      string `json:"tool"`
	Path      string `json:"path,omitempty"`
	Content   string `json:"content,omitempty"`
	OldString string `json:"old_string,omitempty"`
	NewString string `json:"new_string,omitempty"`
	Agent     string `json:"agent,omitempty"` // for delegate_task
	Key       string `json:"key,omitempty"`   // for store_memory / retrieve_memory
}

type ToolResult struct {
	Success bool
	Output  string
	ExecCmd *exec.Cmd
	Cleanup func()
}

type Registry struct {
	FS     *FS
	Memory map[string]string // shared KV store for store_memory / retrieve_memory
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
