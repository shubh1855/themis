package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/files"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/mcp"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/models"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/scraper"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/system"
)

// PromptFn is called when an agent needs user input during a task.
// question is the text to show; inputType is "text" or "confirm".
// It blocks until the user responds and returns the user's answer.
type PromptFn func(question, inputType string) string

func NewReactExecutor(rootDir string, mcpMgr *mcp.Manager, promptFn PromptFn) func(string, map[string]interface{}) (string, error) {
	fm := files.NewManager(rootDir)
	deps := NewDependencies(rootDir)
	router := NewRouter(deps)
	memory := make(map[string]string) // session-scoped key/value store

	return func(tool string, args map[string]interface{}) (string, error) {
		switch tool {

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

		case "move_file":
			src := models.ArgString(args, "src")
			dst := models.ArgString(args, "dst")
			return "moved " + src + " → " + dst, fm.MoveFile(src, dst)

		case "copy_file":
			src := models.ArgString(args, "src")
			dst := models.ArgString(args, "dst")
			return "copied " + src + " → " + dst, fm.CopyFile(src, dst)

		case "tree":
			p := models.ArgString(args, "path")
			if p == "" {
				p = "."
			}
			out, err := fm.Tree(p, 4)
			if err != nil {
				return "", err
			}
			return out, nil

		case "glob_search":
			pattern := models.ArgString(args, "pattern")
			if pattern == "" {
				return "", fmt.Errorf("missing 'pattern'")
			}
			matches, err := fm.Glob(pattern)
			if err != nil {
				return "", err
			}
			return strings.Join(matches, "\n"), nil

		case "store_memory":
			key := models.ArgString(args, "key")
			if key == "" {
				return "", fmt.Errorf("store_memory: missing 'key'")
			}
			content := models.ArgString(args, "content")
			memory[key] = content
			return fmt.Sprintf("stored key %q (%d chars)", key, len(content)), nil

		case "retrieve_memory":
			key := models.ArgString(args, "key")
			if key == "" {
				return "", fmt.Errorf("retrieve_memory: missing 'key'")
			}
			val, exists := memory[key]
			if !exists {
				return fmt.Sprintf("(no value stored for key %q — memory is empty for this key)", key), nil
			}
			return val, nil

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

		
		case "browser_screenshot":
			path := "/tmp/agent_screenshot_" + time.Now().Format("150405") + ".png"
			if p := models.ArgString(args, "path"); p != "" {
				path = p
			}
			return scraper.BrowserScreenshot(path)

		case "browser_click":
			selector := models.ArgString(args, "selector")
			if selector == "" {
				return "", fmt.Errorf("missing 'selector'")
			}
			return scraper.BrowserClick(selector)

		case "browser_type":
			selector := models.ArgString(args, "selector")
			textToType := models.ArgString(args, "text")
			if selector == "" || textToType == "" {
				return "", fmt.Errorf("missing 'selector' or 'text'")
			}
			return scraper.BrowserType(selector, textToType)

		case "browser_scroll":
			direction := models.ArgString(args, "direction")
			amount := 500
			if a, ok := args["amount"].(float64); ok {
				amount = int(a)
			}
			if direction == "" {
				direction = "down"
			}
			return scraper.BrowserScroll(direction, amount)

		case "browser_highlight":
			selector := models.ArgString(args, "selector")
			if selector == "" {
				return "", fmt.Errorf("missing 'selector'")
			}
			return scraper.BrowserHighlight(selector)

		case "browser_hover":
			selector := models.ArgString(args, "selector")
			if selector == "" {
				return "", fmt.Errorf("missing 'selector'")
			}
			return scraper.BrowserHover(selector)

		case "browser_inspect":
			selector := models.ArgString(args, "selector")
			if selector == "" {
				return "", fmt.Errorf("missing 'selector'")
			}
			return scraper.BrowserInspect(selector)

		case "browser_close":
			return scraper.BrowserClose(), nil

		case "start_background":
			// Route to router for the actual process start.
			resp := ExecuteTool(context.Background(), models.ToolRequest{Tool: tool, Args: args}, router)
			if !resp.Success {
				return "", fmt.Errorf("%s", resp.Error)
			}
			result := ""
			if s, ok := resp.Data.(string); ok {
				result = s
			} else {
				b, _ := json.MarshalIndent(resp.Data, "", "  ")
				result = string(b)
			}
			// Auto-preview web dev servers in the rod browser.
			command := models.ArgString(args, "command")
			if port := detectWebDevPort(command); port != "" {
				url := "http://localhost:" + port
				result += "\n🌐 Detected web server on port " + port + " — opening in browser when ready..."
				go func() {
					if waitTCPPort("localhost", port, 45*time.Second) {
						_ = scraper.BrowserOpen(url)
					}
				}()
			}
			return result, nil

		case "npm_search", "pip_search", "cargo_search", "go_search":
			query := models.ArgString(args, "query")
			if query == "" {
				return "", fmt.Errorf("missing 'query'")
			}
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

		case "ask_user":
			question := models.ArgString(args, "question")
			if question == "" {
				return "", fmt.Errorf("ask_user: missing 'question'")
			}
			inputType := models.ArgString(args, "type")
			if inputType == "" {
				inputType = "text"
			}
			if promptFn != nil {
				return promptFn(question, inputType), nil
			}
			return "no prompt handler available", nil

		case "git_init":
			path := models.ArgString(args, "path")
			if path == "" {
				path = "."
			}
			cmd := fmt.Sprintf(`mkdir -p %q && git -C %q init && git -C %q add -A && git -C %q commit -m "chore: initial commit" --allow-empty`, path, path, path, path)
			result, err := system.RunShellCmd(context.Background(), cmd, rootDir, 30*time.Second)
			if err != nil {
				return "", err
			}
			out := result.Stdout
			if result.Stderr != "" {
				out += "\nSTDERR:\n" + result.Stderr
			}
			return out, nil

		case "github_create_repo":
			name := models.ArgString(args, "name")
			if name == "" {
				return "", fmt.Errorf("github_create_repo: missing 'name'")
			}
			private := false
			if v, ok := args["private"].(bool); ok {
				private = v
			}
			visibility := "--public"
			if private {
				visibility = "--private"
			}
			cmd := fmt.Sprintf("gh repo create %q %s --source=. --remote=origin --push", name, visibility)
			result, err := system.RunShellCmd(context.Background(), cmd, rootDir, 60*time.Second)
			if err != nil {
				return "", err
			}
			out := result.Stdout
			if result.Stderr != "" {
				out += "\nSTDERR:\n" + result.Stderr
			}
			return out, nil

		case "vercel_deploy":
			prod := false
			if v, ok := args["prod"].(bool); ok {
				prod = v
			}
			flags := "--yes"
			if prod {
				flags += " --prod"
			}
			result, err := system.RunShellCmd(context.Background(), "npx -y vercel@latest deploy "+flags, rootDir, 120*time.Second)
			if err != nil {
				return "", err
			}
			out := result.Stdout
			if result.Stderr != "" {
				out += "\nSTDERR:\n" + result.Stderr
			}
			return out, nil

		case "vercel_list":
			result, err := system.RunShellCmd(context.Background(), "npx -y vercel@latest list", rootDir, 30*time.Second)
			if err != nil {
				return "", err
			}
			out := result.Stdout
			if result.Stderr != "" {
				out += "\nSTDERR:\n" + result.Stderr
			}
			return out, nil

		case "vercel_logs":
			deployURL := models.ArgString(args, "url")
			if deployURL == "" {
				return "", fmt.Errorf("vercel_logs: missing 'url'")
			}
			result, err := system.RunShellCmd(context.Background(), "npx -y vercel@latest logs "+deployURL, rootDir, 30*time.Second)
			if err != nil {
				return "", err
			}
			out := result.Stdout
			if result.Stderr != "" {
				out += "\nSTDERR:\n" + result.Stderr
			}
			return out, nil

		case "delegate", "delegate_task":
			// This should be intercepted by the ReAct loop before reaching the executor.
			// If it reaches here, it means the agent name or task was not resolved properly.
			agent := models.ArgString(args, "agent")
			task := models.ArgString(args, "task")
			if task == "" {
				task = models.ArgString(args, "content")
			}
			if agent == "" {
				return "", fmt.Errorf("delegate: missing 'agent' field. Use: {\"tool\":\"delegate\",\"agent\":\"Hephaestus\",\"task\":\"...\"}")
			}
			return "", fmt.Errorf("delegate to %q failed: agent name not recognized or task missing. Valid agents: Athena, Hephaestus, Apollo, Hermes, Ares, Prometheus. You called with agent=%q task=%q", agent, agent, task)

		default:
			// Route MCP tools through the MCP manager.
			if mcp.IsMCPTool(tool) {
				if mcpMgr == nil {
					return "", fmt.Errorf("MCP manager not initialized")
				}
				return mcpMgr.CallTool(context.Background(), tool, args)
			}
			// Route unrecognized tools through the Router (handles git/github/registry/etc.)
			resp := ExecuteTool(context.Background(), models.ToolRequest{
				Tool: tool,
				Args: args,
			}, router)
			if !resp.Success {
				return "", fmt.Errorf("%s", resp.Error)
			}
			if s, ok := resp.Data.(string); ok {
				return s, nil
			}
			b, _ := json.MarshalIndent(resp.Data, "", "  ")
			return string(b), nil
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

// webDevKeywords are command fragments that indicate a web dev server.
var webDevKeywords = []string{
	"next dev", "next start",
	"vite", "vite dev", "vite preview",
	"npm run dev", "npm start", "npm run serve", "npm run preview",
	"yarn dev", "yarn start",
	"pnpm dev", "pnpm start",
	"bun dev", "bun run dev",
	"react-scripts start",
	"astro dev",
	"nuxt dev",
	"remix dev",
	"svelte-kit dev",
	"python -m http.server", "python3 -m http.server",
	"npx serve", "serve .",
}

// frameworkPorts maps framework keywords to their default dev ports.
var frameworkPorts = map[string]string{
	"next":          "3000",
	"react-scripts": "3000",
	"create-react":  "3000",
	"astro":         "4321",
	"nuxt":          "3000",
	"remix":         "3000",
	"svelte":        "5173",
	"vite":          "5173",
	"angular":       "4200",
	"vue":           "5173",
	"python":        "8000",
	"serve":         "3000",
}

var portFlagRe = regexp.MustCompile(`(?:--port|-p)\s+(\d+)`)

// detectWebDevPort returns the port a web dev command will listen on, or ""
// if the command doesn't look like a web dev server.
func detectWebDevPort(command string) string {
	lower := strings.ToLower(command)

	isWeb := false
	for _, kw := range webDevKeywords {
		if strings.Contains(lower, kw) {
			isWeb = true
			break
		}
	}
	if !isWeb {
		return ""
	}

	// Explicit --port / -p flag takes priority.
	if m := portFlagRe.FindStringSubmatch(command); len(m) == 2 {
		return m[1]
	}

	// Framework-specific defaults.
	for kw, port := range frameworkPorts {
		if strings.Contains(lower, kw) {
			return port
		}
	}

	// Generic fallback for known dev-server patterns.
	return "3000"
}

// waitTCPPort polls until host:port accepts a TCP connection or the timeout expires.
func waitTCPPort(host, port string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	addr := net.JoinHostPort(host, port)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			conn.Close()
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}
