# Architecture Proposal: Multi-Agent Scaling

This document proposes a scalable architecture to transition the CLI from a single prompt loop to a robust multi-agent ecosystem.

## 1. Current Architecture Constraints
- The current implementation in `main.go` heavily intertwines the Bubbletea UI model with agentic logic.
- A single LLM instance is used for all tasks, limiting the possibility of having specialized agents (e.g., Code Reviewer, SysAdmin, Planner).
- PTY output blocks the main execution flow and tool permission queue.

## 2. Proposed Multi-Agent Architecture

### **The Coordinator (Router) Pattern**
We should introduce a Coordinator agent that intercepts the user's prompt and delegates to specific Sub-Agents depending on the task:
- **Search Agent:** Specialized in `grep_search` and `find_by_name`. Only returns context.
- **Coder Agent:** Specialized in `replace_file_content` and code synthesis.
- **Shell Agent:** Specialized in executing PTY sessions.

### **Component Decoupling**
`main.go` -> `ui/app.go`
`model.Update` handles pure UI. An event bus decoupled from Bubbles can be used to pass messages asynchronously.

```go
type Agent interface {
    Process(ctx context.Context, input string) (Response, error)
    Tools() []tools.ToolDef
}
```

### **Parallel execution**
Enhance the `tools.Registry` to allow concurrent execution of non-mutating tools (view_file, grep_search) to speed up response generation.

## 3. Persistent Memory
- Implement a SQLite-backed context mechanism.
- KIs (Knowledge Items): Before agents start, they query the DB for context on similar tasks.
