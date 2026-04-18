package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	openai "github.com/sashabaranov/go-openai"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/prompt"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/tools"
	apptty "github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/tty"
)

type responseMsg struct {
	text string
	err  error
}

// execDoneMsg fires after tea.ExecProcess returns so we can drain the rest of the queue.
type execDoneMsg struct{ err error }

type model struct {
	client   *openai.Client
	registry *tools.Registry
	perms    *tools.PermissionManager

	viewport viewport.Model
	input    textarea.Model

	history []string

	suggestions []string
	selectedSug int

	width  int
	height int

	loading bool
	running bool
	quit    bool

	pendingQueue []tools.ToolRequest

	// Agent state
	activeAgent llm.AgentID

	// PTY state
	ptyMaster    *os.File
	ptyCmd       *exec.Cmd
	ptyCleanup   func()
	runOutputIdx int
}

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205"))

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	outputStyle = lipgloss.NewStyle().
			PaddingLeft(1).
			PaddingRight(1)

	warnStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214")).
			Bold(true)

	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1)

	suggestionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")).
			PaddingLeft(2)

	selectedSuggestionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("205")).
				Bold(true).
				PaddingLeft(2)
)

func initialModel() model {
	apiKey := os.Getenv("INFERX_API_KEY")

	cfg := openai.DefaultConfig(apiKey)
	cfg.BaseURL = "https://litellm-proxy-93ef.onrender.com/v1"

	client := openai.NewClientWithConfig(cfg)

	wd, _ := os.Getwd()

	fs := tools.NewFS(wd)
	reg := tools.NewRegistry(fs)
	perms := tools.NewPermissionManager()

	vp := viewport.New(80, 20)
	vp.SetContent("")

	ta := textarea.New()
	ta.Placeholder = "Ask something..."
	ta.Focus()
	ta.CharLimit = 0
	ta.SetHeight(3)
	ta.ShowLineNumbers = false

	return model{
		client:      client,
		registry:    reg,
		perms:       perms,
		viewport:    vp,
		input:       ta,
		history:     []string{},
		selectedSug: -1,
	}
}

func askLLM(client *openai.Client, userPrompt string) tea.Cmd {
	return func() tea.Msg {
		resp, err := client.CreateChatCompletion(
			context.Background(),
			openai.ChatCompletionRequest{
				Model: "google/gemma-4-31B-it",
				Messages: []openai.ChatCompletionMessage{
					{
						Role:    openai.ChatMessageRoleSystem,
						Content: prompt.SystemPrompt,
					},
					{
						Role:    openai.ChatMessageRoleUser,
						Content: userPrompt,
					},
				},
			},
		)

		if err != nil {
			return responseMsg{err: err}
		}

		return responseMsg{text: resp.Choices[0].Message.Content}
	}
}

func (m model) Init() tea.Cmd {
	return textarea.Blink
}

// extractToolRequests parses all newline-delimited JSON tool calls from a response.
func extractToolRequests(text string) []tools.ToolRequest {
	var reqs []tools.ToolRequest
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "{") || !strings.HasSuffix(line, "}") {
			continue
		}
		var req tools.ToolRequest
		if err := json.Unmarshal([]byte(line), &req); err == nil && req.Tool != "" {
			reqs = append(reqs, req)
		}
	}
	return reqs
}

func (m *model) pushOutput(text string) {
	m.history = append(m.history, text)
	m.viewport.SetContent(ui.OutputStyle.Render(strings.Join(m.history, "\n\n")))
	m.viewport.GotoBottom()
}

// pushAgentOutput adds output prefixed with the agent's styled badge.
func (m *model) pushAgentOutput(agent llm.AgentID, text string) {
	emoji := llm.AgentEmoji(agent)
	badge := ui.AgentStyle(string(agent)).Render(emoji + " " + string(agent))
	m.pushOutput(badge + " › " + text)
}

func (m *model) updateViewport() {
	m.viewport.SetContent(ui.OutputStyle.Render(strings.Join(m.history, "\n\n")))
	m.viewport.GotoBottom()
}

func (m *model) resizeView() {
	if m.width == 0 || m.height == 0 {
		return
	}
	overhead := 11 + len(m.suggestions)
	h := m.height - overhead
	if h < 1 {
		h = 1
	}
	m.viewport.Width = m.width - 4
	m.viewport.Height = h
	m.input.SetWidth(m.width - 6)
}

func (m *model) startPTY(cmd *exec.Cmd, cleanup func()) tea.Cmd {
	master, readCmd, err := apptty.Start(cmd)
	if err != nil {
		m.pushOutput("[run] failed to start: " + err.Error())
		if cleanup != nil {
			cleanup()
		}
		return m.drainQueue()
	}
	m.ptyMaster = master
	m.ptyCmd = cmd
	m.ptyCleanup = cleanup
	m.running = true

	m.history = append(m.history, warnStyle.Render("▶ running")+"  (ctrl+d → EOF)\n")
	m.runOutputIdx = len(m.history) - 1
	m.viewport.SetContent(outputStyle.Render(strings.Join(m.history, "\n\n")))
	m.viewport.GotoBottom()

	return readCmd
}

// drainQueue executes tools from the front of the queue until it's empty, a
// permission prompt is needed, or an interactive run_file is encountered.
// For interactive commands it returns a tea.ExecProcess cmd; otherwise nil.
func (m *model) drainQueue() tea.Cmd {
	for len(m.pendingQueue) > 0 {
		if m.perms.NeedsPrompt() {
			return nil // modal will show
		}
		req := m.pendingQueue[0]
		m.pendingQueue = m.pendingQueue[1:]

		res := m.registry.Execute(req)

		if res.ExecCmd != nil {
			return m.startPTY(res.ExecCmd, res.Cleanup)
		}

		m.pushOutput("[tool] " + res.Output)
	}
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeView()

	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.viewport.LineUp(3)
			return m, nil
		case tea.MouseButtonWheelDown:
			m.viewport.LineDown(3)
			return m, nil
		case tea.MouseButtonLeft:
			if (msg.Action == tea.MouseActionPress || msg.Action == tea.MouseActionRelease) &&
				len(m.suggestions) > 0 {
				sugTop := m.height - 1 - 3 - 1 - len(m.suggestions)
				if idx := msg.Y - sugTop; idx >= 0 && idx < len(m.suggestions) {
					m.selectedSug = idx
					m.input.SetValue(m.suggestions[idx])
					m.input.CursorEnd()
				}
			}
		}

	case apptty.OutputMsg:
		if m.runOutputIdx < len(m.history) {
			m.history[m.runOutputIdx] += string(msg)
			m.viewport.SetContent(outputStyle.Render(strings.Join(m.history, "\n\n")))
			m.viewport.GotoBottom()
		}
		return m, apptty.ReadOutput(m.ptyMaster)

	case apptty.DoneMsg:
		if m.ptyMaster != nil {
			m.ptyMaster.Close()
			m.ptyMaster = nil
		}
		if m.ptyCmd != nil {
			go m.ptyCmd.Wait()
			m.ptyCmd = nil
		}
		if m.ptyCleanup != nil {
			m.ptyCleanup()
			m.ptyCleanup = nil
		}
		m.running = false
		if m.runOutputIdx < len(m.history) {
			m.history[m.runOutputIdx] += statusStyle.Render("\n▶ done")
			m.viewport.SetContent(outputStyle.Render(strings.Join(m.history, "\n\n")))
			m.viewport.GotoBottom()
		}
		return m, m.drainQueue()

	// ── Agent delegation: Zeus delegates to a sub-agent ──────────────────
	case llm.DelegationMsg:
		m.activeAgent = msg.Target
		delegateText := fmt.Sprintf("delegating to %s %s: %s",
			llm.AgentEmoji(msg.Target), string(msg.Target), msg.Task)
		m.pushAgentOutput(llm.AgentZeus, ui.AgentDelegateStyle.Render(delegateText))
		return m, tea.Batch(
			llm.AskAgent(m.client, msg.Target, msg.Task, msg.Context),
			m.spinner.Tick,
		)

	// ── Agent response ───────────────────────────────────────────────────
	case llm.ResponseMsg:
		m.loading = false
		agent := msg.Agent
		if agent == "" {
			agent = llm.AgentZeus
		}
		m.activeAgent = agent

		if msg.Err != nil {
			m.pushAgentOutput(agent, "Error: "+msg.Err.Error())
			return m, nil
		}

		text := strings.TrimSpace(msg.Text)
		m.suggestions = nil
		m.selectedSug = -1

		if idx := strings.LastIndex(text, "SUGGESTIONS: "); idx != -1 {
			_ = json.Unmarshal([]byte(text[idx+len("SUGGESTIONS: "):]), &m.suggestions)
			text = strings.TrimSpace(text[:idx])
		}
		m.resizeView()

		if reqs := extractToolRequests(text); len(reqs) > 0 {
			// Show which agent is invoking tools
			m.pushAgentOutput(agent, fmt.Sprintf("executing %d tool(s)...", len(reqs)))
			m.pendingQueue = reqs
			return m, m.drainQueue()
		}

		if text != "" {
			m.pushAgentOutput(agent, text)
		}

	case tea.KeyMsg:

		if m.running && m.ptyMaster != nil {
			m.ptyMaster.Write(apptty.KeyToBytes(msg.String()))
			return m, nil
		}

		// permission modal
		if len(m.pendingQueue) > 0 {
			switch msg.String() {
			case "y":
				req := m.pendingQueue[0]
				m.pendingQueue = m.pendingQueue[1:]
				res := m.registry.Execute(req)
				if res.ExecCmd != nil {
					return m, m.startPTY(res.ExecCmd, res.Cleanup)
				}
				m.pushOutput("[tool] " + res.Output)
				return m, m.drainQueue()

			case "a":
				m.perms.Resolve(tools.AllowAlways)
				return m, m.drainQueue()

			case "n", "esc":
				m.pushOutput("[tool] permission denied")
				m.pendingQueue = nil
			}

			return m, nil
		}

		switch msg.String() {

		case "ctrl+c", "q":
			m.quit = true
			return m, tea.Quit

		case "tab":
			if len(m.suggestions) > 0 {
				m.selectedSug++
				if m.selectedSug >= len(m.suggestions) {
					m.selectedSug = 0
				}
				if m.selectedSug >= 0 && m.selectedSug < len(m.suggestions) {
					m.input.SetValue(m.suggestions[m.selectedSug])
					m.input.CursorEnd()
				}
			}
			return m, nil

		case "shift+tab":
			if len(m.suggestions) > 0 {
				if m.selectedSug == -1 {
					m.selectedSug = len(m.suggestions) - 1
				} else {
					m.selectedSug--
					if m.selectedSug < 0 {
						m.selectedSug = len(m.suggestions) - 1
					}
				}
				if m.selectedSug >= 0 && m.selectedSug < len(m.suggestions) {
					m.input.SetValue(m.suggestions[m.selectedSug])
					m.input.CursorEnd()
				}
			}
			return m, nil

		case "enter":
			if m.loading {
				return m, nil
			}

			userPrompt := strings.TrimSpace(m.input.Value())
			if userPrompt == "" {
				return m, nil
			}

			m.pushOutput("You > " + userPrompt)
			m.input.SetValue("")
			m.loading = true
			m.activeAgent = llm.AgentZeus
			m.suggestions = nil
			m.selectedSug = -1
			m.resizeView()

			return m, askLLM(m.client, userPrompt)
		}

		m.viewport, _ = m.viewport.Update(msg)
		m.input, cmd = m.input.Update(msg)
		return m, cmd

	case responseMsg:
		m.loading = false

		if msg.err != nil {
			m.pushOutput("Error: " + msg.err.Error())
			return m, nil
		}

		text := strings.TrimSpace(msg.text)

		m.suggestions = nil
		m.selectedSug = -1

		if idx := strings.LastIndex(text, "SUGGESTIONS: "); idx != -1 {
			sugJSON := text[idx+len("SUGGESTIONS: "):]
			text = strings.TrimSpace(text[:idx])
			if err := json.Unmarshal([]byte(sugJSON), &m.suggestions); err != nil {
				// if failed to parse, at least we stripped it or maybe we can log safely
			}
		}

		m.resizeView()

		reqs := extractToolRequests(text)
		if len(reqs) > 0 {
			m.pendingQueue = reqs
			return m, m.drainQueue()
		}

		m.pushOutput("AI > " + text)
	}

	return m, nil
}

func (m model) View() string {
	if m.quit {
		return ""
	}

	// Status bar text with active agent indicator
	status := "Ready"
	if m.loading {
		agentBadge := ""
		if m.activeAgent != "" {
			emoji := llm.AgentEmoji(m.activeAgent)
			agentBadge = ui.AgentStyle(string(m.activeAgent)).Render(
				emoji+" "+string(m.activeAgent)) + " "
		}
		status = m.spinner.View() + " " + agentBadge + "Thinking..."
	} else if m.running {
		status = "Running..."
	}

	var bodyContent string
	if len(m.pendingQueue) > 0 {
		front := m.pendingQueue[0]
		remaining := ""
		if len(m.pendingQueue) > 1 {
			remaining = fmt.Sprintf("  (+%d more queued)", len(m.pendingQueue)-1)
		}
		modal := warnStyle.Render("Permission Required") + "\n\n" +
			fmt.Sprintf("Tool: %s\nPath: %s%s\n\n[y] allow once   [a] always   [n] deny",
				front.Tool,
				front.Path,
				remaining,
			)

		modalBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("214")).
			Padding(1, 4).
			Render(modal)

		bodyContent = lipgloss.Place(
			m.viewport.Width, m.viewport.Height,
			lipgloss.Center, lipgloss.Center,
			modalBox,
		)
	} else {
		bodyContent = lipgloss.NewStyle().
			Height(m.viewport.Height).
			MaxHeight(m.viewport.Height).
			Render(m.viewport.View())
	}

	body := borderStyle.Render(
		header + "\n\n" +
			bodyContent,
	)

	var sugView string
	if len(m.suggestions) > 0 {
		var lines []string
		for i, s := range m.suggestions {
			prefix := "[ ] "
			if i == m.selectedSug {
				prefix = "[*] "
				lines = append(lines, selectedSuggestionStyle.Render(prefix+s))
			} else {
				lines = append(lines, suggestionStyle.Render(prefix+s))
			}
		}

		sugView = lipgloss.NewStyle().
			Height(len(m.suggestions)).
			MaxHeight(len(m.suggestions)).
			Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	}

	footer := borderStyle.Render(m.input.View())

	if sugView != "" {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			body,
			sugView,
			footer,
			statusStyle.Render(status+"   (q to quit)"),
		)
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		body,
		footer,
		statusStyle.Render(status+"   (q to quit)"),
	)
}

func main() {
	p := tea.NewProgram(
		initialModel(),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(), // Or tea.WithMouseAllMotion() but cell motion captures mouse clicks cleanly
	)

	if _, err := p.Run(); err != nil {
		fmt.Println("error:", err)
	}
}
