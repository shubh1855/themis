package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	openai "github.com/sashabaranov/go-openai"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/llm"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/tools"
	apptty "github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/tty"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/ui"
)

// execDoneMsg fires after tea.ExecProcess returns so we can drain the rest of the queue.
type execDoneMsg struct{ err error }

type model struct {
	client   *openai.Client
	registry *tools.Registry
	perms    *tools.PermissionManager

	viewport viewport.Model
	input    textarea.Model
	spinner  spinner.Model

	history []string

	suggestions []string
	selectedSug int

	width  int
	height int

	loading bool
	running bool
	quit    bool

	pendingQueue []tools.ToolRequest

	// Agent orchestration
	activeAgent        llm.AgentID
	agentHistory       []openai.ChatCompletionMessage
	pendingDelegations []llm.DelegationMsg // remaining steps from Athena's plan
	toolCallAgent      llm.AgentID         // which agent invoked the current tool queue
	pendingToolResults []string            // accumulated tool outputs to return to agent

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

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = ui.SpinnerStyle

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
		spinner:     sp,
		history:     []string{},
		selectedSug: -1,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.spinner.Tick)
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

// drainQueue executes tools from the front of the queue until it's empty,
// a permission prompt is needed, or an interactive run_file is encountered.
func (m *model) drainQueue() tea.Cmd {
	for len(m.pendingQueue) > 0 {
		req := m.pendingQueue[0]

		// delegate_task: any agent can hand off work to another agent.
		if req.Tool == "delegate_task" {
			m.pendingQueue = m.pendingQueue[1:]
			agentID := llm.ResolveAgentName(req.Agent)
			if agentID == "" {
				m.pushOutput("[delegate_task] unknown agent: " + req.Agent)
				continue
			}
			next := llm.DelegationMsg{Target: agentID, Task: req.Content, Context: ""}
			return func() tea.Msg { return next }
		}

		// Only gate destructive/side-effecting tools; read_file and mkdir run freely.
		if tools.NeedsReview(req.Tool) && m.perms.NeedsPrompt() {
			return nil // permission modal will show for this tool
		}
		m.pendingQueue = m.pendingQueue[1:]

		res := m.registry.Execute(req)

		if res.ExecCmd != nil {
			return m.startPTY(res.ExecCmd, res.Cleanup)
		}

		label := req.Tool
		if req.Path != "" {
			label += " " + req.Path
		}
		if req.Key != "" {
			label += " " + req.Key
		}
		m.pendingToolResults = append(m.pendingToolResults,
			fmt.Sprintf("[%s] %s", label, res.Output))
		m.pushOutput("[tool] " + res.Output)
	}

	// Queue fully drained — send accumulated tool outputs back to the invoking agent.
	if len(m.pendingToolResults) > 0 && m.toolCallAgent != "" {
		results := m.pendingToolResults
		agent := m.toolCallAgent
		history := m.agentHistory
		m.pendingToolResults = nil
		m.loading = true
		return llm.AskAgentWithResults(m.client, agent, results, history)
	}

	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {

	case spinner.TickMsg:
		if m.loading || m.running {
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

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
		m.loading = true
		m.activeAgent = msg.Target
		m.agentHistory = nil
		delegateText := fmt.Sprintf("delegating to %s %s: %s",
			llm.AgentEmoji(msg.Target), string(msg.Target), msg.Task)
		m.pushAgentOutput(llm.AgentZeus, ui.AgentDelegateStyle.Render(delegateText))
		return m, tea.Batch(
			llm.AskAgent(m.client, msg.Target, msg.Task, msg.Context, nil),
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
		m.agentHistory = msg.History

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
			m.pushAgentOutput(agent, fmt.Sprintf("executing %d tool(s)...", len(reqs)))
			m.toolCallAgent = agent // track so results can be sent back
			m.pendingQueue = reqs
			return m, m.drainQueue()
		}

		// If Athena returned a structured plan, queue ALL delegations — not just the first.
		if agent == llm.AgentAthena {
			if plan := llm.ParseAthenaPlan(text); plan != nil {
				if text != "" {
					m.pushAgentOutput(agent, text)
				}
				delegations := llm.DispatchPlanTasks(plan)
				if len(delegations) > 0 {
					m.pendingDelegations = delegations[1:]
					m.loading = true
					first := delegations[0]
					return m, func() tea.Msg { return first }
				}
				return m, nil
			}
		}

		// Agent is done (no more tool calls) — show its response.
		m.toolCallAgent = ""
		if text != "" {
			m.pushAgentOutput(agent, text)
		}

		// Advance to the next step in the active plan if any remain.
		if len(m.pendingDelegations) > 0 {
			next := m.pendingDelegations[0]
			m.pendingDelegations = m.pendingDelegations[1:]
			m.loading = true
			m.agentHistory = nil
			return m, func() tea.Msg { return next }
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
				if m.toolCallAgent != "" {
					label := req.Tool
					if req.Path != "" {
						label += " " + req.Path
					}
					m.pendingToolResults = append(m.pendingToolResults,
						fmt.Sprintf("[%s] %s", label, res.Output))
				}
				return m, m.drainQueue()

			case "a":
				m.perms.Resolve(tools.AllowAlways)
				return m, m.drainQueue()

			case "n", "esc":
				m.pushOutput("[tool] permission denied")
				m.pendingQueue = nil
				m.pendingDelegations = nil
				m.pendingToolResults = nil
				m.toolCallAgent = ""
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
			m.agentHistory = nil
			m.suggestions = nil
			m.selectedSug = -1
			m.resizeView()

			return m, llm.AskWithOrchestration(m.client, userPrompt)
		}

		m.viewport, _ = m.viewport.Update(msg)
		m.input, cmd = m.input.Update(msg)
		return m, cmd

	}

	return m, nil
}

func (m model) View() string {
	if m.quit {
		return ""
	}

	header := titleStyle.Render("Themis")

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
