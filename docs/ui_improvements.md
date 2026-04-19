# UI/UX Improvements

The current Bubbletea-based CLI has a great foundation, but can be elevated to feel significantly more premium and functional.

## 1. Rich Diff Previews
Before replacing file contents, show a unified diff viewer inline so the user doesn't just blindly accept a file replacement.
- Use `github.com/sergi/go-diff` to calculate additions and deletions.
- Render them using Bubbletea viewports with red/green styling.

## 2. Dynamic Layout & Tree Views
- Add a collapsible **Sidebar** using a Bubbletea pane (`lipgloss.JoinHorizontal`). The sidebar could display the project's file tree (`charmbracelet/bubbles/list`), highlighting which files are currently loaded into context.
- Show an active "Memory" or "Tokens" usage meter on the bottom status bar, so the user visually understands when the prompt is getting too bloated.

## 3. Better Inline Modals
Currently, permission modals replace the viewport content or render awkwardly above it.
- Implement an overlapping floating window for `Tools` requiring permissions using `charmbracelet/lipgloss` overlay functions, creating a true dialog box feel.
- Include a "Diff" button inside the modal to expand exactly what the tool changes.

## 4. Enhanced PTY Experience
- When running long tasks (`go test` or `npm install`), gracefully capture standard out into an expandable accordion block rather than spamming the global chat history.
- Put a glowing or dot-matrix spinner specifically next to the command being run.

