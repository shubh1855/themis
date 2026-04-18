package tools_test

import (
	"context"
	"testing"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/models"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/tools"
)

func testRouter(t *testing.T) *tools.Router {
	t.Helper()
	deps := tools.NewDependencies(t.TempDir())
	return tools.NewRouter(deps)
}

func TestRouterDispatch_KnownTool(t *testing.T) {
	router := testRouter(t)

	// read_file on a non-existent file should return an error response
	resp := tools.ExecuteTool(context.Background(), models.ToolRequest{
		Tool: "read_file",
		Args: map[string]interface{}{"path": "nonexistent.txt"},
	}, router)

	if resp.Success {
		t.Error("expected failure for non-existent file, got success")
	}
	if resp.Error == "" {
		t.Error("expected error message, got empty")
	}
}

func TestRouterDispatch_UnknownTool(t *testing.T) {
	router := testRouter(t)

	resp := tools.ExecuteTool(context.Background(), models.ToolRequest{
		Tool: "totally_fake_tool",
		Args: map[string]interface{}{},
	}, router)

	if resp.Success {
		t.Error("expected failure for unknown tool")
	}
	if resp.Error == "" {
		t.Error("expected error message for unknown tool")
	}
}

func TestRouterDispatch_AllToolsRegistered(t *testing.T) {
	router := testRouter(t)

	expectedTools := []string{
		"web_search", "fetch_url", "fetch_json", "download_file", "scrape_page",
		"npm_search", "npm_lookup", "pip_search", "pip_lookup",
		"cargo_search", "crate_lookup", "go_search", "go_lookup",
		"create_file", "write_file", "append_file", "read_file", "edit_file",
		"mkdir", "delete_file", "move_file", "copy_file", "list_dir", "tree", "glob_search",
		"run_cmd", "run_file", "start_background", "stop_background", "logs_process", "wait_port",
		"git_status", "git_diff", "git_log", "git_branch", "git_checkout", "git_commit", "git_clone",
		"run_tests", "run_linter", "coverage_report", "benchmark_cmd",
		"sql_query", "db_tables", "db_schema", "db_migrate",
	}

	registered := router.Tools()
	registeredMap := make(map[string]bool)
	for _, name := range registered {
		registeredMap[name] = true
	}

	for _, expected := range expectedTools {
		if !registeredMap[expected] {
			t.Errorf("tool %q not registered", expected)
		}
	}
}

func TestRouterDispatch_WriteAndReadFile(t *testing.T) {
	router := testRouter(t)

	// Write
	resp := tools.ExecuteTool(context.Background(), models.ToolRequest{
		Tool: "write_file",
		Args: map[string]interface{}{
			"path":    "test.txt",
			"content": "hello world",
		},
	}, router)
	if !resp.Success {
		t.Fatalf("write_file failed: %s", resp.Error)
	}

	// Read
	resp = tools.ExecuteTool(context.Background(), models.ToolRequest{
		Tool: "read_file",
		Args: map[string]interface{}{"path": "test.txt"},
	}, router)
	if !resp.Success {
		t.Fatalf("read_file failed: %s", resp.Error)
	}
	if resp.Data != "hello world" {
		t.Errorf("expected 'hello world', got %v", resp.Data)
	}
}

func TestRouterDispatch_MissingArgs(t *testing.T) {
	router := testRouter(t)

	tests := []struct {
		name string
		tool string
		args map[string]interface{}
	}{
		{"web_search no query", "web_search", map[string]interface{}{}},
		{"fetch_url no url", "fetch_url", map[string]interface{}{}},
		{"edit_file no path", "edit_file", map[string]interface{}{}},
		{"git_checkout no target", "git_checkout", map[string]interface{}{}},
		{"npm_search no query", "npm_search", map[string]interface{}{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := tools.ExecuteTool(context.Background(), models.ToolRequest{
				Tool: tt.tool,
				Args: tt.args,
			}, router)
			if resp.Success {
				t.Error("expected failure for missing args")
			}
		})
	}
}
