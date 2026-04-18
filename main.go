package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	openai "github.com/sashabaranov/go-openai"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/llm"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/tools"
	apptty "github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/tty"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/ui"
)

// ── review types ─────────────────────────────────────────────────────────────

type reviewOpt int

const (
	optAccept reviewOpt = iota
	optReject
	optAcceptAll
)

type toolReview struct {
	req      tools.ToolRequest
	selected reviewOpt
}

var reviewLabels = []string{"  Accept  ", "  Reject  ", "  Accept All  "}
var reviewStyles = []lipgloss.Style{
	ui.ReviewAcceptStyle, ui.ReviewRejectStyle, ui.ReviewNeutralStyle,
}

// ── model ────────────────────────────────────────────────────────────────────

type model struct {
	client   *openai.Client
	registry *tools.Registry
	perms    *tools.PermissionManager
	executor func(string, map[string]interface{}) (string, error)

	viewport viewport.Model
	input    textarea.Model
	spinner  spinner.Model
	help     help.Model

	history     []string
	suggestions []string
	selectedSug int

	pendingQueue []tools.ToolRequest
	review       *toolReview

	// ReAct state
	reactCh     <-chan tea.Msg
	activeAgent llm.AgentID
	thinkIdx    int // index in history for live thinking block (-1 = none)

	// Task graph
	taskGraph      *ui.TaskGraph
	activeTaskID   string // currently running task node ID

	// PTY state
	ptyMaster    *os.File
	ptyCmd       *exec.Cmd
	ptyCleanup   func()
	running      bool
	runOutputIdx int

	width, height int
	loading       bool
	quit          bool
}

func initialModel() model {
	wd, _ := os.Getwd()
	fs := tools.NewFS(wd)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = ui.SpinnerStyle

	vp := viewport.New(80, 20)
	ta := textarea.New()
	ta.Placeholder = "Ask something..."
	ta.Focus()
	ta.CharLimit = 0
	ta.SetHeight(3)
	ta.ShowLineNumbers = false

	return model{
		client:      llm.NewClient(os.Getenv("INFERX_API_KEY")),
		registry:    tools.NewRegistry(fs),
		perms:       tools.NewPermissionManager(),
		executor:    tools.NewReactExecutor(wd),
		viewport:    vp,
		input:       ta,
		spinner:     sp,
		help:        help.New(),
		history:     []string{},
		selectedSug: -1,
		thinkIdx:    -1,
		taskGraph:   ui.NewTaskGraph(),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.spinner.Tick)
}

// ── helpers ───────────────────────────────────────────────────────────────────

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

func (m *model) renderContent() string {
	w := m.viewport.Width
	if w <= 0 {
		w = 80
	}
	return ui.OutputStyle.Copy().Width(w).Render(
		strings.Join(m.history, "\n\n"))
}

func (m *model) pushOutput(text string) {
	m.history = append(m.history, text)
	m.viewport.SetContent(m.renderContent())
	m.viewport.GotoBottom()
}

func (m *model) pushAgentOutput(agent llm.AgentID, text string) {
	badge := ui.AgentStyle(string(agent)).Render(
		llm.AgentEmoji(agent) + " " + string(agent))
	m.pushOutput(badge + " › " + text)
}

func (m *model) updateViewport() {
	m.viewport.SetContent(m.renderContent())
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
	// 80% width for main panel, 20% for task graph
	mainW := m.width * 80 / 100
	if mainW < 40 {
		mainW = m.width - 4 // fallback if terminal too narrow
	}
	m.viewport.Width = mainW - 4
	m.viewport.Height = h
	m.input.SetWidth(m.width - 6)
	m.help.Width = m.width
}

// ── ReAct helpers ─────────────────────────────────────────────────────────────

func (m *model) startThinkBlock(agent llm.AgentID) {
	badge := ui.AgentStyle(string(agent)).Render(
		llm.AgentEmoji(agent) + " " + string(agent))
	header := badge + " " + ui.ThinkStyle.Render("thinking...")
	m.history = append(m.history, header+"\n")
	m.thinkIdx = len(m.history) - 1
	m.updateViewport()
}

func (m *model) appendToThink(chunk string) {
	if m.thinkIdx >= 0 && m.thinkIdx < len(m.history) {
		// Strip ReAct markers for display
		clean := chunk
		clean = strings.ReplaceAll(clean, "THOUGHT:", "")
		clean = strings.ReplaceAll(clean, "THOUGHT :", "")
		clean = strings.ReplaceAll(clean, "ACTION:", "")
		clean = strings.ReplaceAll(clean, "ACTION :", "")
		m.history[m.thinkIdx] += ui.ThinkStyle.Render(clean)
		m.updateViewport()
	}
}

func (m *model) endThinkBlock() {
	m.thinkIdx = -1
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

// ── queue / PTY (legacy) ─────────────────────────────────────────────────────

func (m *model) startPTY(cmd *exec.Cmd, cleanup func()) tea.Cmd {
	master, readCmd, err := apptty.Start(cmd)
	if err != nil {
		m.pushOutput("[run] failed: " + err.Error())
		if cleanup != nil {
			cleanup()
		}
		return m.drainQueue()
	}
	m.ptyMaster = master
	m.ptyCmd = cmd
	m.ptyCleanup = cleanup
	m.running = true
	m.history = append(m.history, ui.WarnStyle.Render("▶ running")+"  (ctrl+d → EOF)\n")
	m.runOutputIdx = len(m.history) - 1
	m.updateViewport()
	return readCmd
}

func (m *model) drainQueue() tea.Cmd {
	for len(m.pendingQueue) > 0 {
		req := m.pendingQueue[0]
		if tools.NeedsReview(req.Tool) && !m.perms.IsGloballyAllowed() {
			label := ui.ToolLabelStyle.Render("  "+req.Tool) + "  " + ui.StatusStyle.Render(req.Path)
			preview := m.registry.Preview(req)
			m.pushOutput(label + "\n" + preview)
			m.review = &toolReview{req: req, selected: optAccept}
			return nil
		}
		m.pendingQueue = m.pendingQueue[1:]
		res := m.registry.Execute(req)
		if res.ExecCmd != nil {
			return m.startPTY(res.ExecCmd, res.Cleanup)
		}
		if res.Output != "" {
			m.pushOutput("[tool] " + res.Output)
		}
	}
	return nil
}

func (m *model) confirmReview(opt reviewOpt) tea.Cmd {
	req := m.review.req
	m.review = nil
	m.pendingQueue = m.pendingQueue[1:]
	switch opt {
	case optReject:
		m.pushOutput(ui.ReviewRejectStyle.Render("✗ rejected: " + req.Tool + " " + req.Path))
		return m.drainQueue()
	case optAcceptAll:
		m.perms.Resolve(tools.AllowAlways)
		fallthrough
	case optAccept:
		res := m.registry.Execute(req)
		if res.ExecCmd != nil {
			return m.startPTY(res.ExecCmd, res.Cleanup)
		}
		m.pushOutput(ui.ReviewAcceptStyle.Render("✓ " + res.Output))
		return m.drainQueue()
	}

	return nil
}

// ── Update ────────────────────────────────────────────────────────────────────

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

	// ── PTY events ───────────────────────────────────────────────────────
	case apptty.OutputMsg:
		if m.runOutputIdx < len(m.history) {
			m.history[m.runOutputIdx] += string(msg)
			m.updateViewport()
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
			m.history[m.runOutputIdx] += ui.StatusStyle.Render("\n▶ done")
			m.updateViewport()
		}
		return m, m.drainQueue()

	// ── ReAct: live thinking stream ──────────────────────────────────────
	case llm.ThinkChunkMsg:
		if m.thinkIdx < 0 {
			m.startThinkBlock(msg.Agent)
		}
		m.activeAgent = msg.Agent
		m.appendToThink(msg.Chunk)
		return m, llm.WaitReact(m.reactCh)

	// ── ReAct: agent calls a tool ────────────────────────────────────────
	case llm.ToolCallMsg:
		m.endThinkBlock()
		badge := ui.AgentStyle(string(msg.Agent)).Render(llm.AgentEmoji(msg.Agent))
		m.pushOutput(badge + " " + ui.ToolExecStyle.Render("🔧 "+msg.Tool) +
			"  " + ui.StatusStyle.Render(truncate(msg.Display, 80)))
		// Track tool call in task graph
		if tid := m.activeTaskID; tid != "" {
			m.taskGraph.AddToolCall(tid, msg.Tool+": "+truncate(msg.Display, 40))
		}
		return m, llm.WaitReact(m.reactCh)

	// ── ReAct: tool result ───────────────────────────────────────────────
	case llm.ToolResultMsg:
		m.pushOutput(ui.ObservationStyle.Render("📋 " + truncate(msg.Result, 500)))
		// Start a new think block for the next step
		m.startThinkBlock(msg.Agent)
		return m, llm.WaitReact(m.reactCh)

	// ── ReAct: agent delegates ───────────────────────────────────────────
	case llm.ReactDelegateMsg:
		m.endThinkBlock()
		m.pushAgentOutput(msg.From,
			ui.AgentDelegateStyle.Render(fmt.Sprintf("→ delegating to %s %s",
				llm.AgentEmoji(msg.Target), string(msg.Target))))
		m.pushOutput(ui.ThinkStyle.Render("  task: " + msg.Task))

		// Mark parent task done, add child task
		if m.activeTaskID != "" {
			m.taskGraph.SetStatus(m.activeTaskID, ui.TaskDone)
		}
		parentID := m.activeTaskID
		if parentID == "" && m.taskGraph.Root != nil {
			parentID = m.taskGraph.Root.ID
		}
		childID := m.taskGraph.AddChild(parentID, string(msg.Target), truncate(msg.Task, 50))
		m.taskGraph.SetStatus(childID, ui.TaskRunning)
		m.activeTaskID = childID

		// Start sub-agent ReAct loop
		m.activeAgent = msg.Target
		ch, reactCmd := llm.StartReact(m.client, msg.Target, msg.Task, msg.Context, m.executor)
		m.reactCh = ch
		return m, tea.Batch(reactCmd, m.spinner.Tick)

	// ── ReAct: final answer ──────────────────────────────────────────────
	case llm.ReactAnswerMsg:
		m.endThinkBlock()
		m.loading = false
		m.reactCh = nil

		// Mark task as done
		if m.activeTaskID != "" {
			m.taskGraph.SetStatus(m.activeTaskID, ui.TaskDone)
		}

		text := strings.TrimSpace(msg.Text)
		m.suggestions = nil
		m.selectedSug = -1

		if idx := strings.LastIndex(text, "SUGGESTIONS: "); idx != -1 {
			_ = json.Unmarshal([]byte(text[idx+len("SUGGESTIONS: "):]), &m.suggestions)
			text = strings.TrimSpace(text[:idx])
		}
		m.resizeView()

		// Check for tool calls in the answer (backward compat)
		if reqs := extractToolRequests(text); len(reqs) > 0 {
			m.pushAgentOutput(msg.Agent, fmt.Sprintf("executing %d tool(s)...", len(reqs)))
			m.pendingQueue = reqs
			return m, m.drainQueue()
		}
		if text != "" {
			m.pushAgentOutput(msg.Agent, ui.AnswerStyle.Render(text))
		}

	// ── ReAct: done ──────────────────────────────────────────────────────
	case llm.ReactDoneMsg:
		m.endThinkBlock()
		m.loading = false
		m.reactCh = nil

	case llm.ReactErrorMsg:
		m.endThinkBlock()
		m.loading = false
		m.reactCh = nil
		if m.activeTaskID != "" {
			m.taskGraph.SetStatus(m.activeTaskID, ui.TaskFailed)
		}
		m.pushAgentOutput(msg.Agent, "Error: "+msg.Err.Error())

	// ── Key events ───────────────────────────────────────────────────────
	case tea.KeyMsg:
		if m.running && m.ptyMaster != nil {
			m.ptyMaster.Write(apptty.KeyToBytes(msg.String()))
			return m, nil
		}

		if m.review != nil {
			switch msg.String() {
			case "left", "h", "shift+tab":
				if m.review.selected > 0 {
					m.review.selected--
				}
			case "right", "l", "tab":
				if int(m.review.selected) < len(reviewLabels)-1 {
					m.review.selected++
				}
			case "enter":
				return m, m.confirmReview(m.review.selected)
			case "y":
				return m, m.confirmReview(optAccept)
			case "n", "esc":
				return m, m.confirmReview(optReject)
			case "a":
				return m, m.confirmReview(optAcceptAll)
			}
			return m, nil
		}

		if key.Matches(msg, ui.Keys.Quit) {
			m.quit = true
			return m, tea.Quit
		}

		switch msg.String() {
		case "pgup", "ctrl+b":
			m.viewport.HalfViewUp()
			return m, nil
		case "pgdown", "ctrl+f":
			m.viewport.HalfViewDown()
			return m, nil
		}

		switch msg.String() {
		case "up":
			if len(m.suggestions) > 0 {
				if m.selectedSug <= 0 {
					m.selectedSug = len(m.suggestions) - 1
				} else {
					m.selectedSug--
				}
				m.input.SetValue(m.suggestions[m.selectedSug])
				m.input.CursorEnd()
				return m, nil
			}
			m.viewport.LineUp(1)
			return m, nil
		case "down":
			if len(m.suggestions) > 0 {
				m.selectedSug = (m.selectedSug + 1) % len(m.suggestions)
				m.input.SetValue(m.suggestions[m.selectedSug])
				m.input.CursorEnd()
				return m, nil
			}
			m.viewport.LineDown(1)
			return m, nil
		case "tab":
			if len(m.suggestions) > 0 {
				m.selectedSug = (m.selectedSug + 1) % len(m.suggestions)
				m.input.SetValue(m.suggestions[m.selectedSug])
				m.input.CursorEnd()
				return m, nil
			}
		case "shift+tab":
			if len(m.suggestions) > 0 {
				if m.selectedSug <= 0 {
					m.selectedSug = len(m.suggestions) - 1
				} else {
					m.selectedSug--
				}
				m.input.SetValue(m.suggestions[m.selectedSug])
				m.input.CursorEnd()
				return m, nil
			}
		}

		// ── Submit ────────────────────────────────────────────────────────
		if key.Matches(msg, ui.Keys.Submit) {
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

			// Reset task graph for new request
			m.taskGraph = ui.NewTaskGraph()
			rootID := m.taskGraph.AddRoot("Zeus", truncate(userPrompt, 50))
			m.activeTaskID = rootID

			// Start ReAct loop with Zeus
			ch, reactCmd := llm.StartReact(m.client, llm.AgentZeus, userPrompt, "", m.executor)
			m.reactCh = ch
			return m, tea.Batch(reactCmd, m.spinner.Tick)
		}

		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	return m, nil
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m model) View() string {
	if m.quit {
		return ""
	}

	status := "Ready"
	if m.loading {
		agentBadge := ""
		if m.activeAgent != "" {
			agentBadge = ui.AgentStyle(string(m.activeAgent)).Render(
				llm.AgentEmoji(m.activeAgent)+" "+string(m.activeAgent)) + " "
		}
		status = m.spinner.View() + " " + agentBadge + "Thinking..."
	} else if m.running {
		status = m.spinner.View() + " Running..."
	}

	// ── Calculate panel widths ──────────────────────────────────────────
	mainW := m.width * 80 / 100
	graphW := m.width - mainW
	if mainW < 40 {
		mainW = m.width
		graphW = 0
	}

	// ── Left panel: chat viewport ───────────────────────────────────────
	bodyContent := lipgloss.NewStyle().
		Height(m.viewport.Height).
		MaxHeight(m.viewport.Height).
		Render(m.viewport.View())
	leftPanel := ui.BorderStyle.Copy().Width(mainW - 2).Render(
		ui.TitleStyle.Render("Themis") + "\n\n" + bodyContent)

	// ── Right panel: task graph ─────────────────────────────────────────
	var topRow string
	if graphW > 8 {
		graphH := m.viewport.Height + 4 // match left panel height
		rightPanel := m.taskGraph.Render(graphW, graphH)
		topRow = lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
	} else {
		topRow = leftPanel
	}

	// ── Suggestions ─────────────────────────────────────────────────────
	var sugView string
	if len(m.suggestions) > 0 {
		lines := make([]string, len(m.suggestions))
		for i, s := range m.suggestions {
			if i == m.selectedSug {
				lines[i] = ui.SelectedSuggestionStyle.Render("[*] " + s)
			} else {
				lines[i] = ui.SuggestionStyle.Render("[ ] " + s)
			}
		}
		sugView = lipgloss.JoinVertical(lipgloss.Left, lines...)
	}

	// ── Footer ──────────────────────────────────────────────────────────
	var footer string
	if m.review != nil {
		footer = m.reviewFooter()
	} else {
		footer = ui.BorderStyle.Render(m.input.View())
	}

	helpBar := ui.StatusStyle.Render(status + "   " + m.help.View(ui.Keys))

	parts := []string{topRow}
	if sugView != "" {
		parts = append(parts, sugView)
	}
	parts = append(parts, footer, helpBar)
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m model) reviewFooter() string {
	var opts []string
	for i, label := range reviewLabels {
		s := reviewStyles[i].Render(label)
		if reviewOpt(i) == m.review.selected {
			switch reviewOpt(i) {
			case optAccept:
				s = ui.ReviewSelectedBg.Copy().Foreground(lipgloss.Color("2")).Render("❯" + label)
			case optReject:
				s = ui.ReviewSelectedBg.Copy().Foreground(lipgloss.Color("1")).Render("❯" + label)
			case optAcceptAll:
				s = ui.ReviewSelectedBg.Copy().Foreground(lipgloss.Color("33")).Render("❯" + label)
			}
		}
		opts = append(opts, s)
	}
	hint := ui.ReviewHintStyle.Render("  ←→ navigate   enter confirm   y/n/a shortcut")
	return ui.BorderStyle.Render(strings.Join(opts, " ") + "\n" + hint)
}

func main() {
	p := tea.NewProgram(
		initialModel(),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Println("error:", err)
	}
}
