package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/files"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/models"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/scraper"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/system"
)

// NewReactExecutor builds a tool executor for the ReAct loop.
// It executes tools synchronously and returns text results.
func NewReactExecutor(rootDir string) func(string, map[string]interface{}) (string, error) {
	fm := files.NewManager(rootDir)
	deps := NewDependencies(rootDir)

	return func(tool string, args map[string]interface{}) (string, error) {
		switch tool {

		// ── File tools ──────────────────────────────────────────────
		case "create_file":
			p := models.ArgString(args, "path")
			c := models.ArgString(args, "content")
			if p == "" {
				return "", fmt.Errorf("missing 'path'")
			}
			return "created " + p, fm.CreateFile(p, c)

		case "write_file":
			p := models.ArgString(args, "path")
			c := models.ArgString(args, "content")
			if p == "" {
				return "", fmt.Errorf("missing 'path'")
			}
			return "wrote " + p, fm.WriteFile(p, c)

		case "append_file":
			p := models.ArgString(args, "path")
			c := models.ArgString(args, "content")
			return "appended to " + p, fm.AppendFile(p, c)

		case "read_file":
			p := models.ArgString(args, "path")
			if p == "" {
				return "", fmt.Errorf("missing 'path'")
			}
			return fm.ReadFile(p)

		case "edit_file":
			p := models.ArgString(args, "path")
			old := models.ArgString(args, "old_string")
			new_ := models.ArgString(args, "new_string")
			if p == "" || old == "" {
				return "", fmt.Errorf("missing 'path' or 'old_string'")
			}
			return "edited " + p, fm.EditFile(p, old, new_)

		case "mkdir":
			p := models.ArgString(args, "path")
			return "created dir " + p, fm.Mkdir(p)

		case "delete_file":
			p := models.ArgString(args, "path")
			return "deleted " + p, fm.DeleteFile(p)

		case "list_dir":
			p := models.ArgString(args, "path")
			if p == "" {
				p = "."
			}
			entries, err := fm.ListDir(p)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%v", entries), nil

		// ── Terminal ────────────────────────────────────────────────
		case "run_cmd", "terminal":
			command := models.ArgString(args, "command")
			if command == "" {
				return "", fmt.Errorf("missing 'command'")
			}
			result, err := system.RunShellCmd(context.Background(), command, rootDir, 30*time.Second)
			if err != nil {
				return "", err
			}
			out := result.Stdout
			if result.Stderr != "" {
				out += "\nSTDERR:\n" + result.Stderr
			}
			if result.ExitCode != 0 {
				out += fmt.Sprintf("\n(exit code %d)", result.ExitCode)
			}
			return out, nil

		case "run_file":
			p := models.ArgString(args, "path")
			if p == "" {
				return "", fmt.Errorf("missing 'path'")
			}
			cmd := buildFileCmd(p)
			if cmd == "" {
				return "", fmt.Errorf("unsupported file type: %s", p)
			}
			result, err := system.RunShellCmd(context.Background(), cmd, rootDir, 60*time.Second)
			if err != nil {
				return "", err
			}
			return result.Stdout + result.Stderr, nil

		// ── Web tools ───────────────────────────────────────────────
		case "web_search":
			query := models.ArgString(args, "query")
			if query == "" {
				return "", fmt.Errorf("missing 'query'")
			}
			results, err := deps.SearchEngine.Search(context.Background(), query, 5)
			if err != nil {
				return "", err
			}
			b, _ := json.MarshalIndent(results, "", "  ")
			return string(b), nil

		case "fetch_url":
			url := models.ArgString(args, "url")
			if url == "" {
				return "", fmt.Errorf("missing 'url'")
			}
			fetcher := scraper.NewFetcher(deps.HTTP)
			html, err := fetcher.FetchPage(context.Background(), url)
			if err != nil {
				return "", err
			}
			text := scraper.ExtractMainText(html)
			if len(text) > 3000 {
				text = text[:3000] + "\n...(truncated)"
			}
			return text, nil

		case "browser_view":
			url := models.ArgString(args, "url")
			if url == "" {
				return "", fmt.Errorf("missing 'url'")
			}
			return scraper.BrowserView(url)

		case "browser_run_js":
			script := models.ArgString(args, "script")
			if script == "" {
				return "", fmt.Errorf("missing 'script'")
			}
			return scraper.BrowserRunJS(script)

		case "browser_close":
			return scraper.BrowserClose(), nil

		// ── Registry tools ──────────────────────────────────────────
		case "npm_search", "pip_search", "cargo_search", "go_search":
			query := models.ArgString(args, "query")
			if query == "" {
				return "", fmt.Errorf("missing 'query'")
			}
			// Use the v2 router for registry tools
			router := NewRouter(deps)
			resp := ExecuteTool(context.Background(), models.ToolRequest{
				Tool: tool,
				Args: args,
			}, router)
			if !resp.Success {
				return "", fmt.Errorf("%s", resp.Error)
			}
			b, _ := json.MarshalIndent(resp.Data, "", "  ")
			return string(b), nil

		default:
			return "", fmt.Errorf("unknown tool: %s", tool)
		}
	}
}

func buildFileCmd(path string) string {
	p := strings.ToLower(path)
	switch {
	case strings.HasSuffix(p, ".py"):
		return "python3 " + path
	case strings.HasSuffix(p, ".js"):
		return "node " + path
	case strings.HasSuffix(p, ".ts"):
		return "npx ts-node " + path
	case strings.HasSuffix(p, ".go"):
		return "go run " + path
	case strings.HasSuffix(p, ".sh"), strings.HasSuffix(p, ".bash"):
		return "bash " + path
	case strings.HasSuffix(p, ".rb"):
		return "ruby " + path
	default:
		return ""
	}
}
