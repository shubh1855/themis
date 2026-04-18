package tools

import (
	"context"
	"fmt"
	"sync"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/models"
)

// Router dispatches tool requests to registered handlers.
type Router struct {
	mu       sync.RWMutex
	handlers map[string]ToolHandler
	deps     *Dependencies
}

// NewRouter creates a tool router with all built-in tools registered.
func NewRouter(deps *Dependencies) *Router {
	r := &Router{
		handlers: make(map[string]ToolHandler),
		deps:     deps,
	}
	r.registerBuiltins()
	return r
}

// Register adds a tool handler. Overwrites existing handlers with the same name.
func (r *Router) Register(name string, handler ToolHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[name] = handler
}

// Tools returns a list of all registered tool names.
func (r *Router) Tools() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.handlers))
	for name := range r.handlers {
		names = append(names, name)
	}
	return names
}

// ExecuteTool dispatches a tool request to the appropriate handler.
func ExecuteTool(ctx context.Context, req models.ToolRequest, router *Router) models.ToolResponse {
	_ = ctx // reserved for future context propagation

	router.mu.RLock()
	handler, ok := router.handlers[req.Tool]
	router.mu.RUnlock()

	if !ok {
		return models.ErrorResponse(fmt.Sprintf("unknown tool: %q", req.Tool))
	}

	toolCtx := Context{
		Req:  req,
		Deps: router.deps,
	}

	defer func() {
		if r := recover(); r != nil {
			// Don't let panicking tools crash the agent
		}
	}()

	return handler(toolCtx)
}

// registerBuiltins registers all built-in tool handlers.
func (r *Router) registerBuiltins() {
	// Web tools
	r.Register("web_search", HandleWebSearch)
	r.Register("fetch_url", HandleFetchURL)
	r.Register("fetch_json", HandleFetchJSON)
	r.Register("download_file", HandleDownloadFile)
	r.Register("scrape_page", HandleScrapePage)

	// Registry tools
	r.Register("npm_search", HandleNPMSearch)
	r.Register("npm_lookup", HandleNPMLookup)
	r.Register("pip_search", HandlePipSearch)
	r.Register("pip_lookup", HandlePipLookup)
	r.Register("cargo_search", HandleCargoSearch)
	r.Register("crate_lookup", HandleCrateLookup)
	r.Register("go_search", HandleGoSearch)
	r.Register("go_lookup", HandleGoLookup)

	// File tools
	r.Register("create_file", HandleCreateFile)
	r.Register("write_file", HandleWriteFile)
	r.Register("append_file", HandleAppendFile)
	r.Register("read_file", HandleReadFile)
	r.Register("edit_file", HandleEditFile)
	r.Register("mkdir", HandleMkdir)
	r.Register("delete_file", HandleDeleteFile)
	r.Register("move_file", HandleMoveFile)
	r.Register("copy_file", HandleCopyFile)
	r.Register("list_dir", HandleListDir)
	r.Register("tree", HandleTree)
	r.Register("glob_search", HandleGlob)

	// Process tools
	r.Register("run_cmd", HandleRunCmd)
	r.Register("run_file", HandleRunFile)
	r.Register("start_background", HandleStartBackground)
	r.Register("stop_background", HandleStopBackground)
	r.Register("logs_process", HandleLogsProcess)
	r.Register("wait_port", HandleWaitPort)

	// Git tools
	r.Register("git_status", HandleGitStatus)
	r.Register("git_diff", HandleGitDiff)
	r.Register("git_log", HandleGitLog)
	r.Register("git_branch", HandleGitBranch)
	r.Register("git_checkout", HandleGitCheckout)
	r.Register("git_commit", HandleGitCommit)
	r.Register("git_clone", HandleGitClone)

	// Test tools
	r.Register("run_tests", HandleRunTests)
	r.Register("run_linter", HandleRunLinter)
	r.Register("coverage_report", HandleCoverageReport)
	r.Register("benchmark_cmd", HandleBenchmarkCmd)

	// DB tools
	r.Register("sql_query", HandleSQLQuery)
	r.Register("db_tables", HandleDBTables)
	r.Register("db_schema", HandleDBSchema)
	r.Register("db_migrate", HandleDBMigrate)
}
