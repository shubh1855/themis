# Agent Communication and Tool Execution Issues

After analyzing the latest multi-agent refactor in `main.go` and `internal/llm/client.go`, here are the core reasons why agents cannot properly communicate or use tools:

## 1. Tool Outputs Are Never Sent Back to the Agents
**The Issue:**
In `main.go` (`drainQueue` and `apptty.DoneMsg`), when a tool like `read_file` or `run_file` finishes executing, the output is pushed to the Bubbletea viewport using `m.pushOutput("[tool] " + res.Output)`. However, there is no corresponding `tea.Cmd` dispatched to send that output back to the LLM.

**The Impact:**
Agents can invoke tools, but they work blindly. If an agent calls `read_file`, it never receives the file's contents, so it cannot base its next decision on the code.

**The Fix:**
When the `pendingQueue` is fully drained (or after an interactive PTY session finishes), the UI should automatically bundle the tool outputs and dispatch a new `llm.AskAgent` or LLM API call with the role "tool" or "system" containing the results.

## 2. Athena's Plan only Executes the First Step
**The Issue:**
When Athena generates a structured JSON plan, it gets parsed by `llm.ParseAthenaPlan` and dispatched via `llm.DispatchPlanTasks`. 
If you look inside `DispatchPlanTasks()` in `internal/llm/client.go:228`, it builds an ordered list of tasks but explicitly only returns the first one:
```go
	first := ordered[0]
	return func() tea.Msg {
		return DelegationMsg{ ... Target: first.id ... }
	}
```

**The Impact:**
Only the first agent in Athena's sequence receives their instruction. Once that sub-agent finishes, there is no state machine in `main.go` tracking the remaining steps of the plan, so the rest of the plan is completely ignored. Agents do not communicate passing the baton.

**The Fix:**
Store the active `*llm.AthenaPlan` inside the `model` struct in `main.go`. When an `AgentDoneMsg` or `ResponseMsg` is received from a sub-agent, pop the next task from the plan and trigger the next `DelegationMsg`.

## 3. Disjointed JSON Parsing
**The Issue:**
You are parsing tools via `extractToolRequests` using regex/line-parsing, allocating Zeus delegations via `parseDelegation`, and allocating Athena plans via `ParseAthenaPlan`. 

**The Impact:**
Different agents have to use entirely different JSON formats to do things, which drastically increases prompt confusion and parsing failures. 

**The Fix:**
Standardize on the OpenAI native Tool Calling (Function Calling) API natively supported by `go-openai`.

---

## Recommended New Tools to Add to `registry.go`

To make a truly autonomous multi-agent ecosystem, you should add the following tools:

1. **`delegate_task`**: Allow *any* agent to invoke a JSON tool passing a task to another agent. (Takes `"agent": "Hephaestus"`, `"task": "..."`). This prevents the need for strict hardcoded routing.
2. **`store_memory` / `retrieve_memory`**: A simple key-value store tool (stored in memory or sqlite) so that Athena can store findings, and Hephaestus can retrieve them later without bloating the token context.
3. **`ask_user`**: A tool an agent can invoke when it hits an ambiguous requirement. It pauses execution, displays the prompt, and waits for human input.
