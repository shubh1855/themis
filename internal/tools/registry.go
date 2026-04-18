package tools

import (
	"encoding/json"
	"errors"
)

type ToolRequest struct {
	Tool    string `json:"tool"`
	Path    string `json:"path,omitempty"`
	Content string `json:"content,omitempty"`
}

type ToolResult struct {
	Success bool   `json:"success"`
	Output  string `json:"output"`
}

type Registry struct {
	FS *FS
}

func NewRegistry(fs *FS) *Registry {
	return &Registry{FS: fs}
}

func (r *Registry) Execute(req ToolRequest) ToolResult {
	switch req.Tool {

	case "read_file":
		out, err := r.FS.ReadFile(req.Path)
		if err != nil {
			return fail(err)
		}
		return ok(out)

	case "write_file":
		err := r.FS.WriteFile(req.Path, req.Content)
		if err != nil {
			return fail(err)
		}
		return ok("file written")

	case "append_file":
		err := r.FS.AppendFile(req.Path, req.Content)
		if err != nil {
			return fail(err)
		}
		return ok("file appended")

	case "create_file":
		err := r.FS.CreateFile(req.Path, req.Content)
		if err != nil {
			return fail(err)
		}
		return ok("file created")

	case "mkdir":
		err := r.FS.Mkdir(req.Path)
		if err != nil {
			return fail(err)
		}
		return ok("directory created")

	case "run_file":
		out, err := r.FS.RunFile(req.Path, req.Content)
		if err != nil {
			if out != "" {
				return ToolResult{Success: false, Output: out + "\n" + err.Error()}
			}
			return fail(err)
		}
		return ok(out)

	default:
		return fail(errors.New("unknown tool"))
	}
}

func (r *Registry) ExecuteJSON(raw string) ToolResult {
	var req ToolRequest

	err := json.Unmarshal([]byte(raw), &req)
	if err != nil {
		return fail(err)
	}

	return r.Execute(req)
}

func ok(msg string) ToolResult {
	return ToolResult{
		Success: true,
		Output:  msg,
	}
}

func fail(err error) ToolResult {
	return ToolResult{
		Success: false,
		Output:  err.Error(),
	}
}
