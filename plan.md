# Syn3rgy CLI Architecture & Implementation Plan

## 1. Overview
The goal is to build a highly concurrent, standalone CLI/TUI application in Go that allows managing Projects, Chats, and Tasks. The application must support running and monitoring parallel background tasks via goroutines while handling interactive operations concurrently. Crucially, the app must compile down to a single binary with no external daemon setups (like a separate Qdrant server).

## 2. Tech Stack Setup
* **UI Framework:** [Bubble Tea](https://github.com/charmbracelet/bubbletea) (by Charm) for the interactive terminal user interface, `lipgloss` for styling, and `bubblemacs` / `bubbles` for components.
* **Concurrency:** Native Go Contexts, Goroutines, and Channels, integrated with Bubble Tea's `tea.Cmd` message loop.
* **Relational Storage:** `SQLite` (using pure-Go `modernc.org/sqlite` to avoid CGO cross-compilation headaches, or `mattn/go-sqlite3`).
* **Vector Storage (Addressing the Qdrant constraint):**
  * **The Problem:** Qdrant is written in Rust and operates as a separate server. Shipping it requires either Docker or packaging the Qdrant binary separately, which violates the "standalone single package" requirement.
  * **The Solution:** We strictly need Qdrant's API, the CLI could transparently download the Qdrant executable on first run and manage the process via Go's `os/exec` in the background—but pure embedded is cleaner (cross platform).

---

## 3. Application Design & UI Flow

The CLI will operate as a Terminal UI (TUI) with a router that switches between "Views" or "Pages".

### A. The Dashboard (First Page)
* **Purpose:** The entry point.
* **Components:**
  * **Recent Activity List:** Shows the latest modified Projects and Chats to select from.
  * **Global Status Bar:** Shows active background tasks running globally.
  * **Keybindings:** `Enter` to open a project/chat, `n` to create new, `q` to quit, `t` to open Task Manager.

### B. Workspace (Project / Chat View)
* **Splitscreen / Tabs:**
  * **Main Area:** Active chat session or project management (creating sub-tasks, triggering actions).
  * **Right Sidebar (Collapsible):** Live view of Goroutines/Tasks specific to this project (e.g., "Embedding codebase [█████░░] 75%").
* **Switching:** You can hop out of a Chat back to the Workspace, while the model is still streaming its response in the background or a shell command is running. Everything is asynchronous.

### C. Task Manager / Parallel Stuff View
* A dedicated view showing the worker pool. You can see what goroutines are doing (e.g., "Syncing repo", "Generating Embeddings for Chat X"). You can cancel (`ctrl+c` on a selected task to cancel its `context.Context`).

---

## 4. Concurrency Architecture (Goroutines)

To allow the user to switch between "doing tasks" and "letting the AI/system do parallel stuff":

1. **Job Queue / Worker Pool:** We create an `internal/workers` package.
2. **Task Structure:**
   ```go
   type Task struct {
       ID       string
       Type     string // "LLM_INFERENCE", "FILE_INDEX", etc.
       Status   string
       Progress float64
       Cancel   context.CancelFunc
   }
   ```
3. **Dispatch & Reporting (The Magic):**
   * When a user triggers an action (e.g., "Index this codebase"), the UI layer sends a message to the Job Queue.
   * The Job Queue spawns a `goroutine` via a waitgroup or an interface manager.
   * As the goroutine works, it periodically sends `ProgressMsg` to the Bubble Tea application via `tea.Program.Send(ProgressMsg{})`.
   * The Bubble Tea `Update()` function catches these messages and rerenders the UI, animating the progress bar natively while the user continues typing to a chat.

---

## 5. Storage Architecture

We split the logic into `internal/storage` with repository patterns:

```go
type Storage struct {
    sqlite *sql.DB
    vector VectorDB // Interfaces out the specific choice
}
```

### Relational Schema (SQLite)
* **Projects:** `id`, `name`, `path`, `created_at`, `updated_at`
* **Chats:** `id`, `project_id`, `title`, `created_at`
* **Messages:** `id`, `chat_id`, `role`, `content`, `timestamp`
* **Tasks:** (Optional, if persistent task history is needed)

### Vector Schema (Embedded Vector DB)
* **Collections:** One per Project.
* **Documents:** Code chunks, markdown chunks, or chat summaries accompanied by floating-point array embeddings.
* Automatically hydrated in the background by a goroutine when a project is initialized.

---

## 6. Directory Structure

```text
├── cmd/
│   └── syn3rgy/
│       └── main.go           # CLI Entrypoint, triggers Bubble Tea
├── internal/
│   ├── config/               # Handles loading config file
│   ├── ui/                   # Bubble Tea components
│   │   ├── dashboard/        # First page UI (Recent projects/chats)
│   │   ├── chat/             # Chat UI
│   │   ├── tasks/            # Background tasks UI
│   │   └── router/           # Handles switching between views
│   ├── worker/               # Goroutine dispatcher and contexts
│   └── storage/              # Database implementations
│       ├── sqlite/           # Relational schemas
│       └── vector/           # chromem-go or sqlite-vec abstractions
├── pkg/
│   └── llm/                  # Packages for external inference / embedding models
├── go.mod
└── go.sum
```

## 7. Next Steps for Implementation
1. **Bootstrap the UI:** Start by writing the `main.go` and integrating Bubble Tea with a simple Router that shows a placeholder Dashboard and handles `Ctrl+C` cleanly.
2. **Implement SQLite Setup:** Create the `storage/sqlite` migrations that run via `go:embed` SQL schemas on app startup. Ensure a seamless zero-setup first launch.
3. **Spike Vector Storage:** Validate an embedded vector database approach. Create a dummy project, insert 5 chunks with embeddings, and query them.
4. **Implement Global Worker Pool:** Write the goroutine manager using a map and Contexts, passing an update channel back to the UI.
5. **Connect the App:** Build the first complete flow -> Open App -> See Dashboard -> Create Project -> Triggers Background Task (seen in Tasks view).
