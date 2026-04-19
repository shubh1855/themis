# 🏛️ Themis
**An Advanced Agentic Multi-Agent TUI Assistant**

Themis is a cutting-edge interactive command-line application that seamlessly merges an expressive Terminal User Interface (TUI) with a powerful LLM backend (Google Gemma-4-31B via LiteLLM). Operating on a true multi-agent ReAct architecture, Themis functions as an autonomous pair programmer—capable of deep codebase search, running processes, building features, resolving errors, and managing your terminal environment directly.

---

## 🛠️ High-Level Architecture
The application runs as a cohesive monolith written in Go, relying extensively on [Bubble Tea](https://github.com/charmbracelet/bubbletea) for a stateful TUI, with multiple asynchronous subsystems operating in tandem:

1. **TUI & State Manager** (`main.go`, `internal/ui`): Manages the view modes (Dashboard, Chat, Settings, MCP), orchestrates interactions, streams tokens dynamically, and renders Markdown content natively.
2. **ReAct Execution Engine** (`internal/llm/react.go`, `internal/tools/executor.go`): The brain of Themis. Parses inputs, drives agent prompt loops to think/act/observe, routes tool execution, and builds context chains. 
3. **Agent Federation** (`internal/prompt/agents/`): Distinct specialized prompts corresponding to different domain expert personas (Zeus, Athena, Hephaestus, etc.).
4. **Tool Sandbox & Registry** (`internal/tools/`): Maps LLM JSON tool requests to actual native Go functions. Regulated by a strict permission gate (`permissions.go`).
5. **RAG / Vector Subsystem** (`internal/qdrant/`): Plugs into Qdrant to auto-index the workspace directory, embedding file chunks to feed the AI via the `search_files` tool.
6. **Data & Persistence** (`internal/dbx/`): Tracks user projects, conversations, dashboard configuration, and token usage locally in an SQLite datastore.
7. **Model Context Protocol (MCP)** (`internal/mcp/`): Allows bridging external tools and services directly to Themis agents via the MCP RPC standard.

---

## 🗂️ Codebase Breakdown

The project follows idiomatic Go patterns, breaking heavy logic down into specific packages nested within the `internal/` boundary.

### Core Foundation
- **`main.go`**: The entry point. Handles Bubble Tea lifecycle (Init, Update, View), integrates the LLM bridging, and ties components together.
- **`go.mod` & `go.sum`**: Includes standard deps like `go-openai`, `bubbletea`, `go-rod`, `chroma`, `modernc.org/sqlite`, and `pty`.
- **`CLAUDE.md`**: Outlines tool constraints, file flow, and commands for adjacent AIs contributing to the project.

### Internal Modules (`internal/`)
- `appdir/`: Ensures persistent user data directories are consistently provisioned across OS platforms.
- `audio/`: Logic for handling whisper transcription or audio interactions.
- `auth/`: Cross-system authentication integrations.
- `cache/`: Volatile system memory to reduce repeat LLM/API calls.
- `dbx/`: SQLite wrapper containing logic for `projects.go`, `settings.go`, `usage.go`, and `migrate.go` ensuring persistent user states.
- `files/`: General file manipulation and validation helpers complementing the `tools` sandbox.
- `gitx/`: Source control abstractions (staging, diffing, branching).
- `httpx/`: Handles any incoming/outgoing generic web request structures.
- `llm/`: Contains the fundamental AI loop constraints. `client.go` acts as a facade to interact with models, while `react.go` embodies the core Thought/Action/Observation event loop that drives the multi-agent system.
- `logger/`: Structured application logging handling output distinct from the TUI.
- `mcp/`: Contains `client.go`, `manager.go`, and `types.go` that bootstrap MCP servers and allow the LLM to route actions to external plugins dynamically.
- `models/`: Contains centralized struct definitions used globally.
- `prompt/`: Holds routing and initialization logic for the system prompt construction. Its major subsidiary is `agents/`.
  - **`prompt/agents/`**: Specifically houses the meticulously crafted prompts for the system's specialized roles:
    - *01_zeus.go* (System Orchestrator)
    - *02_athena.go* (Architect & Researcher)
    - *03_hephaestus.go* (Lead Engineer & Coder)
    - *04_apollo.go* (Debugger & Validator)
    - *05_hermes.go* (Communicator & Documenter)
    - *06_ares.go* (Operations & Security)
    - *07_prometheus.go* (Visionary / Unbound Intelligence)
- `qdrant/`: Manages the local vector DB connection (`qdrant.go`, `client.go`), embedding document chunks for efficient, token-saving RAG inside large workspaces.
- `registry/`: General service locator to avoid circular dependencies between massive internal domains.
- `scraper/`: Web traversal leveraging `go-rod` (headless browser controller) to scrape documentation/answers.
- `search/`: Fallback or specialized AST/text string search utilities.
- `security/`: Encapsulates path validation and token sanitization logic.
- `syntax/`: Grammar and AST breakdown algorithms for parsing code buffers.
- `system/`: Broad OS utilities.
- `tester/`: In-built sandbox runner used by agents to run TDD evaluations automatically.
- `tools/`: The action framework accessed by LLM context.
  - Features tools for database mutation (`db_tools.go`), files (`file_tools.go`, `fs.go`), Git operations (`git_tools.go`, `github_tools.go`), bash execution (`process_tools.go`), browser reading (`web_tools.go`) and LLM reflection (`test_tools.go`). Handled actively via `registry.go` and `executor.go`. Includes permission management to protect end-users.
- `tty/`: Contains `tty_unix.go`, `tty_windows.go`, and `tty.go`. Enables pseudo-terminal creation allowing the AI to run real OS commands interactively instead of merely writing files.
- `ui/`: Encapsulates reusable UI elements like colors (`theme.go`), mappings (`keys.go`), styles (`styles.go`), and the reactive `taskgraph.go`.
- `worker/`: A goroutine pool for delegating background indexing and parallel RAG embeddings without freezing the TUI event loop.

## ⚙️ How It Works

1. **Initialization:** Starting Themis builds the SQLite DB, boots Qdrant indices, tests MCP bridges, and opens the main dashboard view.
2. **Context Creation:** The user sends natural language queries via the Terminal textarea.
3. **Agent ReAct Loop:**
   - The Active Agent answers with JSON containing either natural `message` or a `tool` request.
   - If a tool is requested (`write_file`, `run_cmd`, `github_pr`), the `executor.go` validates it via the Registry.
   - The tool executes natively on the file system or standard OS terminal, and the result is fed back into the agent as an observation.
4. **Resolution:** The AI iterates through thoughts and actions until the requested task is verified dynamically via testing tools or human approval.

## 🚀 Getting Started

```bash
# Clone the repository
git clone <repository_url>

# Inside the root folder, tidy your Go environment
go mod tidy

# Execute the local builder
go build -o themis main.go

# Start the multi-agent TUI
./themis
```
*Note: Ensure you configure the API key inside Settings within the TUI or define `INFERX_API_KEY` in your environment variables.*
