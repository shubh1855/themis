# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**Themis** is an interactive CLI coding assistant. It combines a Bubble Tea TUI with an LLM backend (Google Gemma-4-31B via LiteLLM proxy) and a permission-gated filesystem tool system.

## Commands

```bash
# Run the app
go run main.go

# Build
go build -o themis

# Tidy dependencies
go mod tidy

# Vet
go vet ./...
```

**Required env var:** `INFERX_API_KEY` — the API key for the LiteLLM proxy at `https://litellm-proxy-93ef.onrender.com/v1`.

## Architecture

### Data Flow

1. User types in the TUI textarea and submits
2. Message added to chat history; request sent to OpenAI-compatible API (`go-openai` client)
3. LLM response is either natural text or a JSON tool request
4. If JSON is detected, it's parsed as a `ToolRequest` and dispatched via the tool registry
5. Registry checks permissions before executing; prompts the user if no permission exists yet
6. Result is appended to the viewport

### Key Files

- **`main.go`** — Bubble Tea model, all TUI state, LLM API call, JSON detection logic, permission prompt UI. This is where most of the application logic lives.
- **`internal/tools/registry.go`** — Dispatches `ToolRequest` (name + parameters map) to the correct FS operation.
- **`internal/tools/fs.go`** — Safe filesystem wrapper; validates all paths stay within the working directory (prevents traversal).
- **`internal/tools/permissions.go`** — In-memory permission store with three states: deny, allow-once, allow-always. Keys are tool names.

### Permission Flow

When the registry receives a tool call, it checks `PermissionManager`:
- **No record / deny** → TUI enters permission-prompt mode; user presses `y` (allow once), `a` (allow always), or `n` (deny)
- **allow-once** → executes and clears the permission
- **allow-always** → executes immediately

### Tool Names

`create_file`, `write_file`, `read_file`, `append_file`, `mkdir` — all defined in `registry.go` and backed by `fs.go`.
