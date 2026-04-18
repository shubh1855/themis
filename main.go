package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	openai "github.com/sashabaranov/go-openai"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/dbx"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/llm"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/qdrant"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/tools"
	apptty "github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/tty"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/ui"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/worker"
)

// ── Styles for dashboard ────────────────────────────────────────────────

var (
	// Gradient-esque brand colours
	brandPrimary   = lipgloss.Color("205") // hot pink
	brandSecondary = lipgloss.Color("141") // lavender
	brandAccent    = lipgloss.Color("214") // amber
	brandDim       = lipgloss.Color("241")
	brandSuccess   = lipgloss.Color("40")
	brandDanger    = lipgloss.Color("196")

	dashBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(brandSecondary).
			Padding(1, 2)

	dashTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(brandPrimary).
			MarginBottom(1)

	dashSubtitle = lipgloss.NewStyle().
			Foreground(brandSecondary).
			Italic(true)

	dashItemNormal = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			PaddingLeft(2)

	dashItemSelected = lipgloss.NewStyle().
				Foreground(brandPrimary).
				Bold(true).
				PaddingLeft(1)

	dashSectionTitle = lipgloss.NewStyle().
				Foreground(brandAccent).
				Bold(true).
				MarginTop(1)

	dashHint = lipgloss.NewStyle().
			Foreground(brandDim).
			Italic(true).
			MarginTop(1)

	taskTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("220"))

	taskBarDone = lipgloss.NewStyle().
			Foreground(brandSuccess)

	taskBarPending = lipgloss.NewStyle().
			Foreground(brandDim)

	taskBarFailed = lipgloss.NewStyle().
			Foreground(brandDanger).
			Bold(true)

	logoArt = `
  ████████╗██╗  ██╗███████╗███╗   ███╗██╗███████╗
  ╚══██╔══╝██║  ██║██╔════╝████╗ ████║██║██╔════╝
     ██║   ███████║█████╗  ██╔████╔██║██║███████╗
     ██║   ██╔══██║██╔══╝  ██║╚██╔╝██║██║╚════██║
     ██║   ██║  ██║███████╗██║ ╚═╝ ██║██║███████║
     ╚═╝   ╚═╝  ╚═╝╚══════╝╚═╝     ╚═╝╚═╝╚══════╝`
)

// ── Dashboard list items ────────────────────────────────────────────────

type dashItem struct {
	kind  string // "project" or "chat" or "action"
	label string
	desc  string
	id    int
}

// ── View modes ──────────────────────────────────────────────────────────

type ViewMode int

const (
	ViewDashboard ViewMode = iota
	ViewChat
	ViewTasks
)

// ── Review types (existing) ─────────────────────────────────────────────

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

// ── Model ───────────────────────────────────────────────────────────────

type model struct {
	// ── View routing ──
	viewMode ViewMode

	// ── Persistence ──
	db      *dbx.DB
	qdrant  *qdrant.Manager
	workers *worker.Pool

	// ── Dashboard state ──
	dashItems    []dashItem
	dashCursor   int
	dashInput    textarea.Model // for "new project" name entry
	dashCreating bool           // true when typing a new project name

	// ── Agent / Chat state (existing Themis logic) ──
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

	reactCh     <-chan tea.Msg
	activeAgent llm.AgentID
	thinkIdx    int

	taskGraph    *ui.TaskGraph
	activeTaskID string

	ptyMaster    *os.File
	ptyCmd       *exec.Cmd
	ptyCleanup   func()
	running      bool
	runOutputIdx int

	width, height int
	loading       bool
	quit          bool
}

// ── DB init message ─────────────────────────────────────────────────────

type dbReadyMsg struct {
	db    *dbx.DB
	err   error
	items []dashItem
}

func initDB() tea.Msg {
	home, _ := os.UserHomeDir()
	dbPath := filepath.Join(home, ".local", "share", "themis", "data.db")
	_ = os.MkdirAll(filepath.Dir(dbPath), 0755)

	db, err := dbx.Open(dbPath)
	if err != nil {
		return dbReadyMsg{err: err}
	}

	ctx := context.Background()
	if e := db.InitSettings(ctx); e != nil {
		return dbReadyMsg{err: e}
	}
	if e := db.InitProjects(ctx); e != nil {
		return dbReadyMsg{err: e}
	}

	items := buildDashItems(db)
	return dbReadyMsg{db: db, items: items}
}

// ── Qdrant init message ─────────────────────────────────────────────────

type qdrantReadyMsg struct {
	err error
}

func startQdrant(mgr *qdrant.Manager) tea.Cmd {
	return func() tea.Msg {
		err := mgr.EnsureRunning()
		return qdrantReadyMsg{err: err}
	}
}

func buildDashItems(db *dbx.DB) []dashItem {
	var items []dashItem

	// existing projects
	projects, _ := db.ListProjects(context.Background())
	for _, p := range projects {
		items = append(items, dashItem{
			kind:  "project",
			label: p.Name,
			desc:  p.Path + "  (" + p.UpdatedAt + ")",
			id:    p.ID,
		})
	}

	// recent chats (without a project scope)
	chats, _ := db.RecentChats(context.Background())
	for _, c := range chats {
		items = append(items, dashItem{
			kind:  "chat",
			label: c.Title,
			desc:  c.UpdatedAt,
			id:    c.ID,
		})
	}

	// permanent actions at the bottom
	items = append(items,
		dashItem{kind: "action", label: "＋  New Project", desc: "Create a new project workspace"},
		dashItem{kind: "action", label: "＋  New Chat", desc: "Start a standalone chat session"},
	)

	return items
}

// ── initialModel ────────────────────────────────────────────────────────

func initialModel() model {
	wd, _ := os.Getwd()
	fs := tools.NewFS(wd)

	sp := spinner.New()
	sp.Spinner = spinner.MiniDot
	sp.Style = ui.SpinnerStyle

	vp := viewport.New(80, 20)
	ta := textarea.New()
	ta.Placeholder = "Ask Themis anything..."
	ta.Focus()
	ta.CharLimit = 0
	ta.SetHeight(3)
	ta.ShowLineNumbers = false

	dashTA := textarea.New()
	dashTA.Placeholder = "Project name..."
	dashTA.CharLimit = 100
	dashTA.SetHeight(1)
	dashTA.ShowLineNumbers = false

	return model{
		viewMode: ViewDashboard,
		workers:  worker.NewPool(),
		qdrant:   qdrant.New(),

		dashItems: []dashItem{
			{kind: "action", label: "＋  New Project", desc: "Create a new project workspace"},
			{kind: "action", label: "＋  New Chat", desc: "Start a standalone chat session"},
		},
		dashCursor: 0,
		dashInput:  dashTA,

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
	return tea.Batch(
		textarea.Blink,
		m.spinner.Tick,
		initDB,
		startQdrant(m.qdrant),
	)
}

// ── Helpers (existing) ──────────────────────────────────────────────────

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
	mainW := m.width * 80 / 100
	if mainW < 40 {
		mainW = m.width - 4
	}
	m.viewport.Width = mainW - 4
	m.viewport.Height = h
	m.input.SetWidth(m.width - 6)
	m.help.Width = m.width
}

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

// ── Update ──────────────────────────────────────────────────────────────

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {

	case dbReadyMsg:
		if msg.err != nil {
			// silently continue without db
			return m, nil
		}
		m.db = msg.db
		m.dashItems = msg.items
		return m, nil

	case qdrantReadyMsg:
		// Qdrant startup completed (success or failure) — just re-render
		return m, nil

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
		if m.viewMode == ViewChat {
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
		}

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

	case llm.ThinkChunkMsg:
		if m.thinkIdx < 0 {
			m.startThinkBlock(msg.Agent)
		}
		m.activeAgent = msg.Agent
		m.appendToThink(msg.Chunk)
		return m, llm.WaitReact(m.reactCh)

	case llm.ToolCallMsg:
		m.endThinkBlock()
		badge := ui.AgentStyle(string(msg.Agent)).Render(llm.AgentEmoji(msg.Agent))
		m.pushOutput(badge + " " + ui.ToolExecStyle.Render("🔧 "+msg.Tool) +
			"  " + ui.StatusStyle.Render(truncate(msg.Display, 80)))
		if tid := m.activeTaskID; tid != "" {
			m.taskGraph.AddToolCall(tid, msg.Tool+": "+truncate(msg.Display, 40))
		}
		return m, llm.WaitReact(m.reactCh)

	case llm.ToolResultMsg:
		m.pushOutput(ui.ObservationStyle.Render("📋 " + truncate(msg.Result, 500)))
		m.startThinkBlock(msg.Agent)
		return m, llm.WaitReact(m.reactCh)

	case llm.ReactDelegateMsg:
		m.endThinkBlock()
		m.pushAgentOutput(msg.From,
			ui.AgentDelegateStyle.Render(fmt.Sprintf("→ delegating to %s %s",
				llm.AgentEmoji(msg.Target), string(msg.Target))))
		m.pushOutput(ui.ThinkStyle.Render("  task: " + msg.Task))

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

		m.activeAgent = msg.Target
		ch, reactCmd := llm.StartReact(m.client, msg.Target, msg.Task, msg.Context, m.executor)
		m.reactCh = ch
		return m, tea.Batch(reactCmd, m.spinner.Tick)

	case llm.ReactAnswerMsg:
		m.endThinkBlock()
		m.loading = false
		m.reactCh = nil

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

		if reqs := extractToolRequests(text); len(reqs) > 0 {
			m.pushAgentOutput(msg.Agent, fmt.Sprintf("executing %d tool(s)...", len(reqs)))
			m.pendingQueue = reqs
			return m, m.drainQueue()
		}
		if text != "" {
			m.pushAgentOutput(msg.Agent, ui.AnswerStyle.Render(text))
		}

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

	case worker.ProgressMsg:
		// background worker updates, re-render tasks view
		return m, nil

	case tea.KeyMsg:
		// ── Global quit ──
		if msg.String() == "ctrl+c" {
			m.quit = true
			return m, tea.Quit
		}

		// ── View switching ──
		switch msg.String() {
		case "ctrl+t":
			m.viewMode = ViewTasks
			return m, nil
		}

		// ── Dashboard mode ──
		if m.viewMode == ViewDashboard {
			return m.updateDashboard(msg)
		}

		// ── Tasks mode ──
		if m.viewMode == ViewTasks {
			if msg.String() == "esc" {
				m.viewMode = ViewDashboard
				return m, nil
			}
			return m, nil
		}

		// ── Chat mode below ──
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
			case "n":
				return m, m.confirmReview(optReject)
			case "a":
				return m, m.confirmReview(optAcceptAll)
			}
			return m, nil
		}

		if msg.String() == "esc" {
			m.viewMode = ViewDashboard
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

			m.taskGraph = ui.NewTaskGraph()
			rootID := m.taskGraph.AddRoot("Zeus", truncate(userPrompt, 50))
			m.activeTaskID = rootID

			ch, reactCmd := llm.StartReact(m.client, llm.AgentZeus, userPrompt, "", m.executor)
			m.reactCh = ch
			return m, tea.Batch(reactCmd, m.spinner.Tick)
		}

		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	return m, nil
}

// ── Dashboard Update ────────────────────────────────────────────────────

func (m model) updateDashboard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If creating a new project name
	if m.dashCreating {
		switch msg.String() {
		case "esc":
			m.dashCreating = false
			m.dashInput.SetValue("")
			return m, nil
		case "enter":
			name := strings.TrimSpace(m.dashInput.Value())
			if name != "" && m.db != nil {
				wd, _ := os.Getwd()
				_, _ = m.db.CreateProject(context.Background(), name, wd)
				m.dashItems = buildDashItems(m.db)
				m.dashCursor = 0
			}
			m.dashCreating = false
			m.dashInput.SetValue("")
			return m, nil
		default:
			var cmd tea.Cmd
			m.dashInput, cmd = m.dashInput.Update(msg)
			return m, cmd
		}
	}

	switch msg.String() {
	case "up", "k":
		if m.dashCursor > 0 {
			m.dashCursor--
		}
	case "down", "j":
		if m.dashCursor < len(m.dashItems)-1 {
			m.dashCursor++
		}
	case "enter":
		if m.dashCursor < len(m.dashItems) {
			item := m.dashItems[m.dashCursor]
			switch item.kind {
			case "project":
				// open project → switch to chat
				if m.db != nil {
					_ = m.db.TouchProject(context.Background(), item.id)
				}
				m.viewMode = ViewChat
				m.input.Focus()
				m.pushOutput(dashSubtitle.Render("📂 Opened project: " + item.label))
			case "chat":
				if m.db != nil {
					_ = m.db.TouchChat(context.Background(), item.id)
				}
				m.viewMode = ViewChat
				m.input.Focus()
				m.pushOutput(dashSubtitle.Render("💬 Resumed chat: " + item.label))
			case "action":
				if strings.Contains(item.label, "New Project") {
					m.dashCreating = true
					m.dashInput.Focus()
				} else if strings.Contains(item.label, "New Chat") {
					m.viewMode = ViewChat
					m.input.Focus()
					m.pushOutput(dashSubtitle.Render("💬 New chat session started"))
				}
			}
		}
	case "n":
		m.dashCreating = true
		m.dashInput.Focus()
	}

	return m, nil
}

// ── View ────────────────────────────────────────────────────────────────

func (m model) View() string {
	if m.quit {
		return ""
	}

	switch m.viewMode {
	case ViewDashboard:
		return m.renderDashboard()
	case ViewTasks:
		return m.renderTasks()
	default:
		return m.renderChat()
	}
}

// ── Dashboard View ──────────────────────────────────────────────────────

func (m model) renderDashboard() string {
	w := m.width
	if w <= 0 {
		w = 80
	}
	h := m.height
	if h <= 0 {
		h = 24
	}

	var sb strings.Builder

	// Logo with gradient
	logoStyle := lipgloss.NewStyle().
		Foreground(brandPrimary).
		Bold(true)
	sb.WriteString(logoStyle.Render(logoArt))

	sb.WriteString("\n\n")

	// Separator
	sep := lipgloss.NewStyle().Foreground(brandDim).Render(strings.Repeat("─", min(w-8, 70)))
	sb.WriteString("  " + sep + "\n\n")

	// Items list
	hasProjects := false
	hasChats := false
	for _, item := range m.dashItems {
		if item.kind == "project" && !hasProjects {
			sb.WriteString("  " + dashSectionTitle.Render("📂 PROJECTS") + "\n")
			hasProjects = true
		}
		if item.kind == "chat" && !hasChats {
			sb.WriteString("\n  " + dashSectionTitle.Render("💬 RECENT CHATS") + "\n")
			hasChats = true
		}
		if item.kind == "action" && (hasProjects || hasChats) {
			sb.WriteString("\n  " + sep + "\n")
			hasProjects = false
			hasChats = false
		}
	}

	// Reset and render the actual items
	sb.Reset()
	sb.WriteString(logoStyle.Render(logoArt))
	sb.WriteString("\n")
	sb.WriteString(dashSubtitle.Render("  Multi-Agent AI Coding Assistant  ⚡ Zeus 🦉 Athena 🔨 Hephaestus ☀️ Apollo 🪽 Hermes ⚔️ Ares"))
	sb.WriteString("\n\n")
	sb.WriteString("  " + sep + "\n\n")

	lastKind := ""
	for i, item := range m.dashItems {
		// Section headers
		if item.kind != lastKind {
			switch item.kind {
			case "project":
				sb.WriteString("  " + dashSectionTitle.Render("📂 PROJECTS") + "\n")
			case "chat":
				if lastKind != "" {
					sb.WriteString("\n")
				}
				sb.WriteString("  " + dashSectionTitle.Render("💬 RECENT CHATS") + "\n")
			case "action":
				sb.WriteString("\n  " + sep + "\n")
			}
			lastKind = item.kind
		}

		cursor := "  "
		style := dashItemNormal
		if i == m.dashCursor {
			cursor = "❯ "
			style = dashItemSelected
		}

		label := style.Render(item.label)
		desc := lipgloss.NewStyle().Foreground(brandDim).Render("  " + item.desc)
		sb.WriteString(cursor + label + desc + "\n")
	}

	// Creating a new project?
	if m.dashCreating {
		sb.WriteString("\n")
		inputBox := dashBorder.Copy().
			BorderForeground(brandAccent).
			Width(min(w-10, 60)).
			Render(
				dashSectionTitle.Render("Enter project name:") + "\n" +
					m.dashInput.View(),
			)
		sb.WriteString("  " + inputBox + "\n")
	}

	// Qdrant status indicator
	var qdrantBadge string
	switch m.qdrant.GetStatus() {
	case qdrant.StatusReady:
		qdrantBadge = lipgloss.NewStyle().Foreground(brandSuccess).Bold(true).Render("● Qdrant")
	case qdrant.StatusDownloading:
		qdrantBadge = lipgloss.NewStyle().Foreground(brandAccent).Render("◌ Qdrant downloading...")
	case qdrant.StatusStarting:
		qdrantBadge = lipgloss.NewStyle().Foreground(brandAccent).Render("◌ Qdrant starting...")
	case qdrant.StatusFailed:
		errStr := ""
		if e := m.qdrant.LastError(); e != nil {
			errStr = " (" + truncate(e.Error(), 40) + ")"
		}
		qdrantBadge = lipgloss.NewStyle().Foreground(brandDanger).Bold(true).Render("✗ Qdrant" + errStr)
	default:
		qdrantBadge = lipgloss.NewStyle().Foreground(brandDim).Render("○ Qdrant")
	}
	sb.WriteString("  " + qdrantBadge + "\n\n")

	// Status bar
	activeCount := 0
	m.workers.ForEach(func(_ string, _ *worker.Task) { activeCount++ })

	statusParts := []string{
		lipgloss.NewStyle().Foreground(brandDim).Render("↑↓/jk navigate"),
		lipgloss.NewStyle().Foreground(brandDim).Render("enter select"),
		lipgloss.NewStyle().Foreground(brandDim).Render("n new project"),
		lipgloss.NewStyle().Foreground(brandDim).Render("ctrl+t tasks"),
		lipgloss.NewStyle().Foreground(brandDim).Render("ctrl+c quit"),
	}
	if activeCount > 0 {
		statusParts = append(statusParts,
			lipgloss.NewStyle().Foreground(brandAccent).Bold(true).Render(
				fmt.Sprintf("⚙ %d active task(s)", activeCount)))
	}
	sb.WriteString("  " + strings.Join(statusParts, "  │  "))

	return lipgloss.NewStyle().Padding(1, 2).Render(sb.String())
}

// ── Tasks View ──────────────────────────────────────────────────────────

func (m model) renderTasks() string {
	w := m.width
	if w <= 0 {
		w = 80
	}

	var sb strings.Builder

	title := taskTitleStyle.Render("⚙  Themis Task Manager")
	sb.WriteString(title + "\n")
	sb.WriteString(lipgloss.NewStyle().Foreground(brandDim).Render(strings.Repeat("─", min(w-8, 60))) + "\n\n")

	count := 0
	m.workers.ForEach(func(id string, t *worker.Task) {
		// Progress bar
		barWidth := 20
		filled := int(t.Progress * float64(barWidth))
		if filled > barWidth {
			filled = barWidth
		}
		bar := taskBarDone.Render(strings.Repeat("█", filled)) +
			taskBarPending.Render(strings.Repeat("░", barWidth-filled))

		pct := fmt.Sprintf("%3d%%", int(t.Progress*100))
		status := lipgloss.NewStyle().Foreground(brandSecondary).Render(t.Status)

		sb.WriteString(fmt.Sprintf("  %s  %s  %s  %s\n", bar, pct, status, id))
		count++
	})

	if count == 0 {
		emptyStyle := lipgloss.NewStyle().Foreground(brandDim).Italic(true).PaddingLeft(2)
		sb.WriteString(emptyStyle.Render("No active background tasks") + "\n")
		sb.WriteString(emptyStyle.Render("Tasks appear here when agents run file indexing, embeddings, etc.") + "\n")
	}

	sb.WriteString("\n")
	sb.WriteString(dashHint.Render("  [esc] back to dashboard  │  [ctrl+c] quit"))

	return lipgloss.NewStyle().Padding(2, 3).Render(sb.String())
}

// ── Chat View (existing Themis UI) ──────────────────────────────────────

func (m model) renderChat() string {
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

	mainW := m.width * 80 / 100
	graphW := m.width - mainW
	if mainW < 40 {
		mainW = m.width
		graphW = 0
	}

	bodyContent := lipgloss.NewStyle().
		Height(m.viewport.Height).
		MaxHeight(m.viewport.Height).
		Render(m.viewport.View())
	leftPanel := ui.BorderStyle.Copy().Width(mainW - 2).Render(
		ui.TitleStyle.Render("Themis") + "\n\n" + bodyContent)

	var topRow string
	if graphW > 8 {
		graphH := m.viewport.Height + 4
		rightPanel := m.taskGraph.Render(graphW, graphH)
		topRow = lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
	} else {
		topRow = leftPanel
	}

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

	var footer string
	if m.review != nil {
		footer = m.reviewFooter()
	} else {
		footer = ui.BorderStyle.Render(m.input.View())
	}

	helpBar := ui.StatusStyle.Render(status + "  │  " + m.help.View(ui.Keys) + "  │  esc: dashboard  ctrl+t: tasks")

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

// ── Main ────────────────────────────────────────────────────────────────

func main() {
	m := initialModel()
	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if _, err := p.Run(); err != nil {
		fmt.Println("error:", err)
	}
	// Clean up Qdrant daemon on exit
	m.qdrant.Stop()
}
