# Features Roadmap

To surpass existing CLI AI tools like Claude Code and Gemini CLI, the tool must move beyond basic prompt-and-response and offer deeper local integrations and proactive assistance.

## Phase 1: Context & Intelligence
 DONE - **Intelligent RAG (Retrieval-Augmented Generation):** Auto-index the active git repository. When the user asks a question, automatically retrieve relevant files silently without needing to consume tokens listing directories.
- **Language Server Protocol (LSP) Integration:** Use `gopls` or `ts-server` directly to navigate to definition, find references, and surface compile errors to the AI before presenting code to the user.
- **Token Optimization:** Track context lengths dynamically. Automatically summarize older history or dump it to create persistent summaries for long-running sessions.(tick-tokens)

## Phase 2: Autonomous Workflows
- **Background Agents:** Allow the CLI to dispatch a task to the background (e.g., `themis --background "refactor the API module"`).
- **Test-Driven Auto-Correction:** When modifying a file, automatically run the testing suite associated with it in a background sandbox, continuing to fix it autonomously until the tests pass.

## Phase 3: External Integrations
- **Web Browsing:** Integrate a headless browser tool (Playwright/Puppeteer) so the CLI can read API documentation, scrape answers, and diagnose production issues dynamically.
- **GitHub/GitLab Integrations:** Auto-generate PRs with properly formatted descriptions and inline code-reviews based on git diffs directly from the terminal.

