# Themis
**Advanced Agentic Multi-Agent Assistant**

[![Go Report Card](https://goreportcard.com/badge/github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey)](https://goreportcard.com/report/github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey)
[![Go Reference](https://pkg.go.dev/badge/github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey.svg)](https://pkg.go.dev/github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

<video src="images/demo.mp4" controls="controls" muted="muted" width="100%"></video>

Themis is a highly autonomous command-line application that merges a Terminal User Interface (TUI) with an advanced multi-agent Language Model backend. Operating on a ReAct architecture, Themis functions as an autonomous pair programmer—capable of performing deep codebase searches, executing processes, implementing features, resolving errors, and managing the terminal environment directly.

---

## Architecture Overview
The application is a resilient Go monolith relying extensively on Bubble Tea for a stateful TUI, utilizing multiple asynchronous subsystems:

1. **TUI & State Manager** (`main.go`, `internal/ui`): Manages the view modes (Dashboard, Chat, Settings, MCP), orchestrates interactions, streams tokens dynamically, and natively renders Markdown content.
2. **ReAct Execution Engine** (`internal/llm/react.go`, `internal/tools/executor.go`): Parses inputs, drives agent loops to iterate upon thoughts, actions, and observations, routes tool execution, and builds context logic.
3. **Agent Federation** (`internal/prompt/agents/`): Contains distinct, specialized prompts corresponding to different domain expert roles (e.g., system orchestration, architecture, engineering, validation).
4. **Tool Sandbox & Registry** (`internal/tools/`): Maps LLM tool requests to executable Go functions, securely regulated by a strict permission boundary.
5. **RAG / Vector Subsystem** (`internal/qdrant/`): Integrates with Qdrant to incrementally index the workspace directory, embedding file chunks to supply context during searches.
6. **Data & Persistence** (`internal/dbx/`): Tracks user projects, conversations, dashboard configuration, and token usage via a local SQLite datastore.
7. **Model Context Protocol (MCP)** (`internal/mcp/`): Bridges external tools and services directly to Themis agents via standard RPCs.

---

## Codebase Breakdown

The project follows strict Go best practices, abstracting complex logic into focused internal packages.

### Core Foundation
- **`main.go`**: The entry point. Handles the lifecycle, integrates the LLM bridging, and ties dependencies together.
- **`CLAUDE.md`**: Outlines tool constraints, file flows, and commands for LLMs contributing to the repository.

### Internal Modules (`internal/`)
- `appdir/`: Ensures persistent user data directories are correctly provisioned across platforms.
- `audio/`: Handles background recording and whisper transcription.
- `auth/`: Manages cross-system authentication mechanisms.
- `cache/`: Implements volatile memory to optimize redundant LLM inference calls.
- `dbx/`: SQLite wrapper handling persistent states for projects, settings, and usage.
- `files/`: Contains file manipulation and validation routines interfacing with the execution sandbox.
- `gitx/`: Abstracts source control integration (staging, diffing, branching).
- `httpx/`: Handles structured generic web requests.
- `llm/`: Constrains the core AI processing limits. Contains facade logic for interacting with models and the primary ReAct processing loop.
- `mcp/`: Contains implementations that bootstrap MCP servers and allow the LLM to access external plugin routes dynamically.
- `prompt/agents/`: Holds the meticulously crafted prompts for specialized roles (System Orchestrator, Architect, Engineer, Validator, Documenter, Operations, etc.).
- `qdrant/`: Manages local scalable vector databases, handling embeddings natively.
- `scraper/`: Web traversal logic leveraging headless browser control modules for active automated quality assurance.
- `tty/`: Contains modules that construct raw pseudo-terminals allowing the interface to natively bridge operating system terminal interactions safely.

## Execution Flow

1. **Initialization:** Themis establishes the SQLite relational stores, initiates Qdrant indices, tests MCP bridges, and opens the main TUI.
2. **Context Creation:** Users send standard natural language instructions via the interface.
3. **Agent ReAct Loop:**
   - The Active Agent returns iterative responses utilizing natural messaging or invoking standard system tools.
   - Any invoked tools are processed securely by the execution layer natively within the operating environment.
   - Outputs are returned as observations until task requirements are programmatically fulfilled.
4. **Resolution:** Artificial operators verify task completions utilizing deployment verification procedures before finalizing outputs.

## Installation

### Precompiled Binary Installation (Recommended)

To streamline installations across endpoints securely, you can run the following automated installation scripts. These scripts dynamically resolve the appropriate artifacts tailored to your processor architecture from the private repository.

**Linux / macOS:**
```bash
# Requires an environment variable or argument holding a standard Github Personal Access Token
curl -sL https://raw.githubusercontent.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/main/scripts/install.sh | bash -s -- YOUR_GITHUB_TOKEN
```

*(Alternatively, if an infrastructure administrator has deployed the Cloudflare proxy infrastructure stored in `scripts/proxy-worker.js`, point the cURL endpoint to the provisioned public edge server URL to bypass client-side token requirements entirely.)*

**Windows (PowerShell):**
```powershell
# Download and execute the Windows resolution script natively
iwr https://raw.githubusercontent.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/main/scripts/install.ps1 -useb | iex
```
*(The scripts natively acquire the binaries, extract them, and append the executable directly to your environment variables.)*

### Source Compilation

If you possess read privileges to the repository and prefer to compile the monolith natively:
```bash
git clone <repository_url>
cd <repository_directory>
go mod tidy
go build -o themis main.go
./themis
```
*(Ensure valid API keys are configured globally within the TUI settings immediately post-initialization.)*
