package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	openai "github.com/sashabaranov/go-openai"

	"github.com/NimbleMarkets/ntcharts/barchart"
	"github.com/NimbleMarkets/ntcharts/sparkline"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/appdir"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/audio"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/dbx"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/llm"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/mcp"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/qdrant"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/syntax"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/tools"
	apptty "github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/tty"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/ui"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/worker"
)

// ── Styles for dashboard ────────────────────────────────────────────────

var (
	mouseEscapePattern = regexp.MustCompile(`\x1b?\[<\d+;\d+;\d+[Mm]`)

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

const (
	viewportSyncInterval = 90 * time.Millisecond
	maxThinkDisplayBytes = 12000
)

func screenSize(w, h int) (int, int) {
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	return w, h
}

func contentWidth(totalWidth int, style lipgloss.Style) int {
	return max(1, totalWidth-style.GetHorizontalFrameSize())
}

func chatColumns(width int) (mainW, graphW int) {
	width, _ = screenSize(width, 0)
	mainW = width * 80 / 100
	graphW = width - mainW
	if mainW < 60 || graphW < 24 {
		return width, 0
	}
	return mainW, graphW
}

func clipLine(s string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(s, width, "")
}

func fitBlock(s string, width, height int) string {
	width, height = screenSize(width, height)
	lines := strings.Split(s, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	for i := range lines {
		lines[i] = clipLine(lines[i], width)
	}
	return strings.Join(lines, "\n")
}

// ── Dashboard list items ────────────────────────────────────────────────

type dashItem struct {
	kind      string // "project" or "chat" or "action"
	label     string
	desc      string
	id        int
	projectID int
	path      string // filesystem path for projects
}

// ── View modes ──────────────────────────────────────────────────────────

type ViewMode int

const (
	ViewDashboard ViewMode = iota
	ViewChat
	ViewTasks
	ViewMCP
	ViewSettings
	ViewAgentPrompt
)

// promptBridge wires the agent goroutine's ask_user calls back to the TUI.
// The agent sends an AgentPromptMsg on the react channel and blocks on ReplyCh.
type promptBridge struct {
	mu sync.RWMutex
	ch chan tea.Msg
}

func (b *promptBridge) setChannel(ch chan tea.Msg) {
	b.mu.Lock()
	b.ch = ch
	b.mu.Unlock()
}

func (b *promptBridge) ask(question, inputType string) string {
	b.mu.RLock()
	ch := b.ch
	b.mu.RUnlock()
	if ch == nil {
		return ""
	}
	replyCh := make(chan string, 1)
	select {
	case ch <- llm.AgentPromptMsg{Question: question, InputType: inputType, ReplyCh: replyCh}:
	case <-time.After(5 * time.Second):
		return "timeout: no UI available"
	}
	select {
	case reply := <-replyCh:
		return reply
	case <-time.After(5 * time.Minute):
		return "timeout: user did not respond"
	}
}

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

	activeProjectID   int
	activeProjectPath string
	activeChatID      int // current chat being persisted; 0 = none
	qClient           *qdrant.Client

	// ── MCP ──
	mcpManager *mcp.Manager
	mcpCursor  int // cursor in ViewMCP server list

	// ── Settings ──
	settingsCursor int    // cursor in settings view (theme row index)
	themeName      string // name of current theme key (saved to DB)
	providerIdx    int
	apiInput       textinput.Model
	baseURLInput   textinput.Model
	modelInput     textinput.Model
	grokInput      textinput.Model
	vercelInput    textinput.Model // Vercel token
	ollamaOK       bool
	ollamaChecked  bool
	modelList      []string
	modelListIdx   int
	settingsError  string
	isRecording    bool

	// ── Agent prompt (ask_user tool) ──
	bridge             *promptBridge
	agentPromptQ       string
	agentPromptT       string // "text" or "confirm"
	agentPromptReplyCh chan string
	agentPromptInput   textinput.Model

	// ── Usage stats (loaded when settings opens) ──
	usageLogs     []dbx.UsageEntry
	usageTotalIn  int
	usageTotalOut int

	// ── Multimodal ──
	pendingImages []string // image paths to attach to next prompt

	// ── Token tracking ──
	curInputTokens  int  // input tokens for the current/last request
	curOutputTokens int  // output tokens for current response (live)
	tokenStreaming  bool // true while a response is being streamed
	sessionIn       int  // cumulative session input tokens
	sessionOut      int  // cumulative session output tokens

	// ── Conversation memory ──
	chatLog        []string // plain-text rolling log for cross-prompt context
	lastUserPrompt string   // most recent user prompt

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
	thinkAgent  llm.AgentID
	thinkIdx    int
	thinkStr    string
	thinkDirty  bool

	// ── Verbose / quiet mode ──
	verboseMode       bool          // true = show all THOUGHT streaming; false = compact loading
	nonVerboseSpinner spinner.Model // a different spinner shown in quiet mode
	thinkLineCount    int           // how many think lines suppressed (shown in quiet status)
	viewportNeedsSync bool          // debounce flag for viewport string re-renders
	viewportLastSync  time.Time     // limits full viewport reflow while agents stream tokens

	// ── Render cache ──
	renderedCache []string
	renderedRaw   []string
	renderedW     int

	// ── Full view cache ──
	viewDirty bool   // set to true when anything visible changed
	viewCache string // cached output of renderChat()

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
	db          *dbx.DB
	err         error
	items       []dashItem
	apiKey      string
	modelName   string
	providerCfg dbx.ProviderConfigRow
	providerOK  bool
	providerErr error
	grokKey     string
	vercelToken string
}

type OllamaHealthMsg struct {
	Err error
}

type OllamaModelsMsg struct {
	Models []string
	Err    error
}

func initDB() tea.Msg {
	dataDir := appdir.Data()
	_ = os.MkdirAll(dataDir, 0755)
	dbPath := filepath.Join(dataDir, "data.db")

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
	if e := db.InitUsage(ctx); e != nil {
		return dbReadyMsg{err: e}
	}

	// Apply saved theme before first render.
	if themeName, ok, _ := db.GetSetting(ctx, "theme"); ok && themeName != "" {
		applyTheme(ui.GetTheme(themeName))
	}

	apiKey := os.Getenv("INFERX_API_KEY")
	if apiKey == "" {
		apiKey = "sk-FQO1aH7bCuogvr8cTeeVEA"
	}
	if dbAPI, ok, _ := db.GetSetting(ctx, "api_key"); ok && dbAPI != "" {
		apiKey = dbAPI
	}

	modelName := "google/gemma-4-31B-it"
	if dbModel, ok, _ := db.GetSetting(ctx, "llm_model"); ok && dbModel != "" {
		modelName = dbModel
	}

	providerCfg, providerOK, providerErr := db.LoadProviderConfig(ctx)
	if !providerOK || providerCfg.Provider == "" {
		providerCfg = dbx.ProviderConfigRow{
			Provider: "anthropic",
			APIKey:   apiKey,
			BaseURL:  "",
			Model:    modelName,
		}
	}

	grokKey := ""
	if dbGrok, ok, _ := db.GetSetting(ctx, "grok_key"); ok && dbGrok != "" {
		grokKey = dbGrok
	}

	vercelToken := os.Getenv("VERCEL_TOKEN")
	if dbVercel, ok, _ := db.GetSetting(ctx, "vercel_token"); ok && dbVercel != "" {
		vercelToken = dbVercel
		_ = os.Setenv("VERCEL_TOKEN", dbVercel)
	}

	items := buildDashItems(db)
	return dbReadyMsg{
		db:          db,
		items:       items,
		apiKey:      apiKey,
		modelName:   modelName,
		providerCfg: providerCfg,
		providerOK:  providerOK,
		providerErr: providerErr,
		grokKey:     grokKey,
		vercelToken: vercelToken,
	}
}

func providerIndex(provider string) int {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openai":
		return 1
	case "ollama":
		return 2
	default:
		return 0
	}
}

func providerName(idx int) string {
	switch idx {
	case 1:
		return "openai"
	case 2:
		return "ollama"
	default:
		return "anthropic"
	}
}

// ── Qdrant init message ─────────────────────────────────────────────────

type qdrantReadyMsg struct {
	err error
}

type indexMsg struct {
	err error
}

type mcpReadyMsg struct{}

type transcriptionMsg struct {
	text string
	err  error
}

type imagePickedMsg struct {
	path string
}

type chatHistoryMsg struct {
	chatID   int
	messages []dbx.Message
}

type usageLoadedMsg struct {
	logs     []dbx.UsageEntry
	totalIn  int
	totalOut int
}

// loadChatHistory fetches persisted messages for a chat from the DB.
func (m model) loadChatHistory(chatID int) tea.Cmd {
	if m.db == nil || chatID == 0 {
		return nil
	}
	db := m.db
	return func() tea.Msg {
		msgs, err := db.GetMessages(context.Background(), chatID)
		if err != nil || len(msgs) == 0 {
			return nil
		}
		return chatHistoryMsg{chatID: chatID, messages: msgs}
	}
}

// loadUsageData fetches token usage history from the DB for the settings view.
func (m model) loadUsageData() tea.Cmd {
	if m.db == nil {
		return nil
	}
	db := m.db
	return func() tea.Msg {
		logs, _ := db.GetRecentUsage(context.Background(), 20)
		// Reverse so oldest is first (for chart left→right).
		for i, j := 0, len(logs)-1; i < j; i, j = i+1, j-1 {
			logs[i], logs[j] = logs[j], logs[i]
		}
		totalIn, totalOut, _ := db.TotalUsage(context.Background())
		return usageLoadedMsg{logs: logs, totalIn: totalIn, totalOut: totalOut}
	}
}

// applyTheme updates the mutable brand-color vars and rebuilds derived styles.
func applyTheme(t ui.Theme) {
	brandPrimary = t.Primary
	brandSecondary = t.Secondary
	brandAccent = t.Accent
	brandSuccess = t.Success
	brandDanger = t.Danger
	brandDim = t.Dim

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
	taskBarDone = lipgloss.NewStyle().Foreground(brandSuccess)
	taskBarPending = lipgloss.NewStyle().Foreground(brandDim)
	taskBarFailed = lipgloss.NewStyle().Foreground(brandDanger).Bold(true)
}

// indexProject triggers background file indexing for the active project.
func (m model) indexProject() tea.Cmd {
	if m.qClient == nil || m.activeProjectID == 0 || m.activeProjectPath == "" {
		return nil
	}
	projectID := m.activeProjectID
	dirPath := m.activeProjectPath
	qc := m.qClient
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		err := qc.IndexDirectory(ctx, projectID, dirPath)
		return indexMsg{err: err}
	}
}

// buildChatContext returns the last N log entries as a context string for the LLM.
func (m model) buildChatContext() string {
	if len(m.chatLog) == 0 {
		return ""
	}
	start := 0
	if len(m.chatLog) > 20 {
		start = len(m.chatLog) - 20
	}
	return "Recent conversation:\n" + strings.Join(m.chatLog[start:], "\n")
}

// appendChatLog adds an entry to the rolling conversation log (capped at 60 entries).
func (m *model) appendChatLog(entry string) {
	m.chatLog = append(m.chatLog, entry)
	if len(m.chatLog) > 60 {
		m.chatLog = m.chatLog[len(m.chatLog)-60:]
	}
}

var continuationPhrases = []string{
	"continue", "keep going", "go on", "proceed", "next step", "go ahead",
	"more", "keep building", "carry on", "finish it", "finish up", "complete it",
	"next", "keep working", "resume", "what's next", "whats next",
}

// isContinuation reports whether the prompt is asking to continue previous work.
func isContinuation(s string) bool {
	lower := strings.TrimRight(strings.ToLower(strings.TrimSpace(s)), ".!")
	for _, phrase := range continuationPhrases {
		if lower == phrase || strings.HasPrefix(lower, phrase+" ") {
			return true
		}
	}
	return false
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
			kind:      "project",
			label:     p.Name,
			desc:      p.Path + "  (" + p.UpdatedAt + ")",
			id:        p.ID,
			projectID: p.ID,
			path:      p.Path,
		})
	}

	// recent chats (without a project scope)
	chats, _ := db.RecentChats(context.Background())
	for _, c := range chats {
		items = append(items, dashItem{
			kind:      "chat",
			label:     c.Title,
			desc:      c.UpdatedAt,
			id:        c.ID,
			projectID: c.ProjectID,
		})
	}

	// permanent actions at the bottom
	items = append(items,
		dashItem{kind: "action", label: "＋  New Project", desc: "Create a new project workspace"},
		dashItem{kind: "action", label: "＋  New Chat", desc: "Start a standalone chat session"},
	)

	return items
}

type blinkMsg struct{}

func blinkTick() tea.Cmd {
	return tea.Tick(time.Millisecond*500, func(t time.Time) tea.Msg {
		return blinkMsg{}
	})
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

	apiIn := textinput.New()
	apiIn.Placeholder = "sk-..."
	apiIn.CharLimit = 200
	apiIn.Width = 40
	apiIn.EchoMode = textinput.EchoPassword

	baseURLIn := textinput.New()
	baseURLIn.Placeholder = "http://localhost:11434"
	baseURLIn.CharLimit = 200
	baseURLIn.Width = 40

	modIn := textinput.New()
	modIn.Placeholder = "google/gemma-4-31B-it"
	modIn.CharLimit = 100
	modIn.Width = 40

	grokIn := textinput.New()
	grokIn.Placeholder = "grok api key for whisper"
	grokIn.CharLimit = 200
	grokIn.Width = 40

	vercelIn := textinput.New()
	vercelIn.Placeholder = "paste Vercel token (vercel.com/account/tokens)"
	vercelIn.CharLimit = 200
	vercelIn.Width = 40
	vercelIn.EchoMode = textinput.EchoPassword
	if v := os.Getenv("VERCEL_TOKEN"); v != "" {
		vercelIn.SetValue(v)
	}

	promptIn := textinput.New()
	promptIn.Placeholder = "type your answer..."
	promptIn.CharLimit = 500
	promptIn.Width = 60

	bridge := &promptBridge{}

	apiKey := os.Getenv("INFERX_API_KEY")
	if apiKey == "" {
		apiKey = "sk-FQO1aH7bCuogvr8cTeeVEA"
	}
	llmClient := llm.NewClient(apiKey)
	llm.SetReactModel(llm.DefaultReactModel)
	llm.SetActiveClient(llmClient)
	mcpMgr := mcp.NewManager()

	return model{
		viewMode:   ViewDashboard,
		workers:    worker.NewPool(),
		qdrant:     qdrant.New(),
		qClient:    qdrant.NewClient("http://127.0.0.1:6333", llmClient),
		mcpManager: mcpMgr,

		dashItems: []dashItem{
			{kind: "action", label: "＋  New Project", desc: "Create a new project workspace"},
			{kind: "action", label: "＋  New Chat", desc: "Start a standalone chat session"},
			{kind: "action", label: "⚙  MCP Servers", desc: "Manage Model Context Protocol servers (ctrl+p)"},
		},
		dashCursor:       0,
		dashInput:        dashTA,
		apiInput:         apiIn,
		baseURLInput:     baseURLIn,
		modelInput:       modIn,
		grokInput:        grokIn,
		vercelInput:      vercelIn,
		agentPromptInput: promptIn,
		bridge:           bridge,
		client:           llmClient,
		registry:         tools.NewRegistry(fs),
		perms:            tools.NewPermissionManager(),
		executor:         tools.NewReactExecutor(wd, mcpMgr, bridge.ask),
		viewport:         vp,
		input:            ta,
		spinner:          sp,
		help:             help.New(),
		history:          []string{},
		selectedSug:      -1,
		thinkIdx:         -1,
		verboseMode:      false, // quiet by default keeps scrolling responsive; ctrl+v shows full thinking
		nonVerboseSpinner: func() spinner.Model {
			s := spinner.New()
			s.Spinner = spinner.Points
			s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Bold(true)
			return s
		}(),
		taskGraph: ui.NewTaskGraph(),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		m.spinner.Tick,
		m.nonVerboseSpinner.Tick,
		initDB,
		startQdrant(m.qdrant),
		m.startMCP(),
	)
}

func (m model) startMCP() tea.Cmd {
	mgr := m.mcpManager
	return func() tea.Msg {
		if err := mgr.LoadConfig(); err != nil {
			return mcpReadyMsg{} // non-fatal
		}
		mgr.StartEnabled(context.Background())
		return mcpReadyMsg{}
	}
}

// ── Helpers (existing) ──────────────────────────────────────────────────

func humanizeToolJSON(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") && strings.Contains(trimmed, "\"tool\"") {
			var req tools.ToolRequest
			if err := json.Unmarshal([]byte(trimmed), &req); err == nil && req.Tool != "" {
				lines[i] = humanizeReq(req)
			}
		}
	}
	return strings.Join(lines, "\n")
}

func humanizeAction(tool string, args map[string]interface{}) string {
	if strings.HasPrefix(tool, "mcp__filesystem__") {
		act := strings.TrimPrefix(tool, "mcp__filesystem__")
		switch act {
		case "write_file":
			return fmt.Sprintf("[W] **MCP Writing file:** `%v`", args["path"])
		case "read_file":
			return fmt.Sprintf("[R] **MCP Reading file:** `%v`", args["path"])
		case "list_directory":
			return fmt.Sprintf("[L] **MCP Listing directory:** `%v`", args["path"])
		case "search_files":
			return fmt.Sprintf("[?] **MCP Searching files:** `%v`", args["pattern"])
		case "get_file_info":
			return fmt.Sprintf("[i] **MCP File info:** `%v`", args["path"])
		}
	}
	switch tool {
	case "run_file", "run_cmd":
		var cmd string
		if c, ok := args["command"].(string); ok {
			cmd = c
		} else if c, ok := args["content"].(string); ok {
			cmd = c
		} else {
			cmd = fmt.Sprintf("%v", args)
		}
		return fmt.Sprintf("[>] **Executing terminal command:** \n```bash\n%s\n```", cmd)
	case "read_file":
		return fmt.Sprintf("[R] **Reading file:** `%v`", args["path"])
	case "write_file", "create_file":
		return fmt.Sprintf("[W] **Writing to file:** `%v`", args["path"])
	case "edit_file":
		return fmt.Sprintf("[W] **Editing file:** `%v`", args["path"])
	case "git_commit":
		return fmt.Sprintf("[*] **Git Commit:** `%v`", args["message"])
	case "git_push":
		return "[^] **Git Push**"
	case "git_status":
		return "[?] **Checking Git Status**"
	case "git_diff":
		return "[?] **Checking Git Diff**"
	case "task_plan":
		return "[#] **Constructing Task Plan**"
	case "complete_step":
		return fmt.Sprintf("[✓] **Completed step:** `%v`", args["step"])
	case "web_search":
		return fmt.Sprintf("[@] **Searching Web:** `%v`", args["query"])
	case "fetch_url":
		return fmt.Sprintf(" **Fetching URL:** `%v`", args["url"])
	case "browser_screenshot":
		return " **Capturing Browser Screenshot (Visual QA)**"
	case "browser_click":
		return fmt.Sprintf("️ **Clicking element:** `%v`", args["selector"])
	case "browser_type":
		return fmt.Sprintf("⌨️ **Typing into:** `%v`", args["selector"])
	case "browser_scroll":
		dir := args["direction"]
		if dir == nil {
			dir = "down"
		}
		return fmt.Sprintf(" **Scrolling:** `%v`", dir)
	case "browser_run_js":
		return "⚙️ **Running Javascript in Browser**"
	case "browser_view":
		return fmt.Sprintf("[@] **Opening Browser Window:** `%v`", args["url"])
	}
	return fmt.Sprintf("[>] **Using Tool:** %s", tool)
}

func humanizeReq(req tools.ToolRequest) string {
	switch req.Tool {
	case "run_file", "run_cmd":
		return fmt.Sprintf("[>] **Executing terminal command:** \n```bash\n%s\n```", req.Content)
	case "read_file":
		return fmt.Sprintf("[R] **Reading file:** `%s`", req.Path)
	case "write_file", "create_file":
		return fmt.Sprintf("[W] **Writing to file:** `%s`", req.Path)
	case "edit_file":
		return fmt.Sprintf("[W] **Editing file:** `%s`", req.Path)
	case "git_commit":
		return fmt.Sprintf("[*] **Git Commit:** `%s`", req.Message)
	case "git_push":
		return "[^] **Git Push**"
	case "git_status":
		return "[?] **Checking Git Status**"
	case "git_diff":
		return "[?] **Checking Git Diff**"
	default:
		return fmt.Sprintf("[>] **Using Tool:** %s", req.Tool)
	}
}

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

	// Dynamic word-wrap cache mechanism
	// Recalculating ANSI-wraps purely with lipgloss across 100kb+ of terminal history 10 times a second
	// is the direct root cause of UI frame latency when agents spin up.
	// By caching historical formatted blocks, we map the workload from O(N) to O(1) instantly.

	if m.renderedW != w {
		m.renderedCache = make([]string, 0, len(m.history))
		m.renderedRaw = make([]string, 0, len(m.history))
		m.renderedW = w
	}

	style := ui.OutputStyle.Copy().Width(w)
	out := make([]string, len(m.history))

	for i, raw := range m.history {
		if i < len(m.renderedCache) && i < len(m.renderedRaw) && m.renderedRaw[i] == raw {
			// Cache hit
			out[i] = m.renderedCache[i]
		} else {
			// Cache miss (newly generated, updating terminal block, or resized bounds)
			var formatted string
			if strings.HasPrefix(raw, "[PTY_BLOCK]") {
				content := raw[len("[PTY_BLOCK]"):]
				boxWidth := w - 4
				if boxWidth < 10 {
					boxWidth = 10
				}
				box := lipgloss.NewStyle().
					Border(lipgloss.RoundedBorder()).
					BorderForeground(lipgloss.Color("63")). // Blue/purple border
					Padding(0, 1).
					Width(boxWidth).
					Render(content)
				formatted = box
			} else {
				formatted = style.Render(raw)
			}

			if i < len(m.renderedCache) {
				m.renderedCache[i] = formatted
				m.renderedRaw[i] = raw
			} else {
				m.renderedCache = append(m.renderedCache, formatted)
				m.renderedRaw = append(m.renderedRaw, raw)
			}
			out[i] = formatted
		}
	}

	return strings.Join(out, "\n\n")
}

func (m *model) syncViewportNow(followBottom bool) {
	m.refreshThinkBlock()
	m.viewport.SetContent(m.renderContent())
	if followBottom {
		m.viewport.GotoBottom()
	}
	m.viewportNeedsSync = false
	m.viewportLastSync = time.Now()
	m.viewDirty = true
}

func (m *model) syncViewportIfNeeded(force bool) {
	if !m.viewportNeedsSync && !m.thinkDirty {
		return
	}
	if !force && !m.viewportLastSync.IsZero() && time.Since(m.viewportLastSync) < viewportSyncInterval {
		return
	}
	m.syncViewportNow(m.viewport.AtBottom())
}

func (m *model) pushOutput(text string) {
	followBottom := len(m.history) == 0 || m.viewport.AtBottom()
	m.history = append(m.history, text)
	m.syncViewportNow(followBottom)
}

func (m *model) pushAgentOutput(agent llm.AgentID, text string) {
	badge := ui.AgentStyle(string(agent)).Render(
		llm.AgentEmoji(agent) + " " + string(agent))
	m.pushOutput(badge + " › " + text)
}

func (m *model) updateViewport() {
	m.viewportNeedsSync = true
}

func (m *model) refreshThinkBlock() {
	if !m.thinkDirty || m.thinkIdx < 0 || m.thinkIdx >= len(m.history) || !m.verboseMode {
		return
	}
	m.history[m.thinkIdx] = m.renderLiveThinkBlock()
	m.thinkDirty = false
}

func (m *model) renderLiveThinkBlock() string {
	agent := m.thinkAgent
	if agent == "" {
		agent = m.activeAgent
	}
	badge := ui.AgentStyle(string(agent)).Render(llm.AgentEmoji(agent) + " " + string(agent))
	header := badge + " " + ui.ThinkStyle.Render("thinking...")
	body := m.thinkStr
	if strings.TrimSpace(body) == "" {
		return header
	}
	return header + "\n" + ui.ThinkStyle.Render(body)
}

func (m *model) resizeView() {
	if m.width == 0 || m.height == 0 {
		return
	}
	footerH := 5
	if m.review != nil {
		footerH = 4
	} else {
		if len(m.pendingImages) > 0 {
			footerH++
		}
		if m.isRecording {
			footerH++
		}
	}
	overhead := 4 + footerH + 2 + len(m.suggestions)
	h := m.height - overhead
	if h < 1 {
		h = 1
	}
	mainW, _ := chatColumns(m.width - 1)
	m.viewport.Width = contentWidth(mainW, ui.BorderStyle)
	m.viewport.Height = h
	m.input.SetWidth(max(20, contentWidth(m.width-1, ui.BorderStyle)-2))
	m.help.Width = m.width
}

func (m *model) startThinkBlock(agent llm.AgentID) {
	m.thinkLineCount = 0
	m.thinkStr = ""
	m.thinkDirty = false
	m.thinkAgent = agent
	if !m.verboseMode {
		// Quiet mode: push a single placeholder line we'll update in-place
		badge := ui.AgentStyle(string(agent)).Render(
			llm.AgentEmoji(agent) + " " + string(agent))
		m.history = append(m.history, badge+" "+ui.ThinkStyle.Render("processing…"))
		m.thinkIdx = len(m.history) - 1
		m.updateViewport()
		return
	}
	m.history = append(m.history, m.renderLiveThinkBlock())
	m.thinkIdx = len(m.history) - 1
	m.updateViewport()
}

func (m *model) appendToThink(chunk string) {
	if m.thinkIdx < 0 || m.thinkIdx >= len(m.history) {
		return
	}
	if !m.verboseMode {
		// Count suppressed lines; update placeholder with rolling line count
		m.thinkLineCount += strings.Count(chunk, "\n")
		return
	}
	clean := chunk
	clean = strings.ReplaceAll(clean, "THOUGHT:", "")
	clean = strings.ReplaceAll(clean, "THOUGHT :", "")
	clean = strings.ReplaceAll(clean, "ACTION:", "")
	clean = strings.ReplaceAll(clean, "ACTION :", "")

	m.thinkStr += clean
	m.thinkDirty = true
	m.updateViewport()
}

func (m *model) endThinkBlock() {
	if m.thinkIdx >= 0 && m.thinkIdx < len(m.history) {
		agent := m.thinkAgent
		if agent == "" {
			agent = m.activeAgent
		}
		badge := ui.AgentStyle(string(agent)).Render(llm.AgentEmoji(agent) + " " + string(agent))

		if !m.verboseMode {
			status := "processed"
			if m.thinkLineCount > 0 {
				status = fmt.Sprintf("processed %d streamed update(s)", m.thinkLineCount)
			}
			m.history[m.thinkIdx] = badge + " " + ui.ThinkStyle.Render(status)
			m.updateViewport()
		} else if m.thinkStr != "" {
			clean := strings.TrimSpace(m.thinkStr)
			if clean != "" {
				header := badge + " › " + ui.ThinkStyle.Render("thinking finished")
				m.history[m.thinkIdx] = header + "\n" + renderMarkdown(clean)
				m.updateViewport()
			}
		}
	}
	m.thinkIdx = -1
	m.thinkStr = ""
	m.thinkDirty = false
	m.thinkLineCount = 0
	m.thinkAgent = ""
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
	m.history = append(m.history, "[PTY_BLOCK]"+ui.WarnStyle.Render("► running")+"  (ctrl+d → EOF)\n")
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

		// Capture old content for diff preview BEFORE executing file tools
		var oldContent string
		switch req.Tool {
		case "write_file", "edit_file":
			oldContent, _ = m.registry.FS.ReadFile(req.Path)
		}

		m.pendingQueue = m.pendingQueue[1:]
		res := m.registry.Execute(req)
		if res.ExecCmd != nil {
			return m.startPTY(res.ExecCmd, res.Cleanup)
		}

		// Emit inline diff for file-write operations
		switch req.Tool {
		case "create_file":
			diffStr := syntax.DiffView(oldContent, req.Content, req.Path)
			header := lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true).Render("▪ "+req.Path) + "  " + lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("(create_file)")
			box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("2")).PaddingLeft(1).PaddingRight(1).Render(header + "\n" + diffStr)
			m.pushOutput(box)
		case "write_file":
			diffStr := syntax.DiffView(oldContent, req.Content, req.Path)
			header := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true).Render("▪ "+req.Path) + "  " + lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("(write_file)")
			box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("205")).PaddingLeft(1).PaddingRight(1).Render(header + "\n" + diffStr)
			m.pushOutput(box)
		case "edit_file":
			newContent, _ := m.registry.FS.ReadFile(req.Path)
			diffStr := syntax.DiffView(oldContent, newContent, req.Path)
			header := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true).Render("✎ "+req.Path) + "  " + lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("(edit_file)")
			box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("214")).PaddingLeft(1).PaddingRight(1).Render(header + "\n" + diffStr)
			m.pushOutput(box)
		case "delete_file":
			header := lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true).Render("⨯ "+req.Path) + "  " + lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("(delete_file)")
			box := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("1")).PaddingLeft(1).PaddingRight(1).Render(header)
			m.pushOutput(box)
		default:
			if res.Output != "" {
				header := lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Bold(true).Render("› " + req.Tool)
				box := lipgloss.NewStyle().
					Border(lipgloss.NormalBorder()).
					BorderForeground(lipgloss.Color("240")).
					Padding(0, 1).
					Render(header + "\n\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Render(res.Output))
				m.pushOutput(box)
			}
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

func moveIndex(current, delta, length int) int {
	if length <= 0 {
		return 0
	}
	current += delta
	if current < 0 {
		return 0
	}
	if current >= length {
		return length - 1
	}
	return current
}

// ── Update ──────────────────────────────────────────────────────────────

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {

	case dbReadyMsg:
		if msg.err != nil {
			return m, nil
		}
		m.db = msg.db
		m.dashItems = msg.items
		if msg.providerErr != nil {
			m.settingsError = msg.providerErr.Error()
		}
		// Load saved theme name so settings view cursor is correct.
		if m.db != nil {
			if name, ok, _ := m.db.GetSetting(context.Background(), "theme"); ok && name != "" {
				m.themeName = name
			} else {
				m.themeName = "default"
			}
		}
		if msg.apiKey != "" {
			m.apiInput.SetValue(msg.apiKey)
		}
		if msg.modelName != "" {
			m.modelInput.SetValue(msg.modelName)
		}
		if msg.providerCfg.Provider != "" {
			m.providerIdx = providerIndex(msg.providerCfg.Provider)
			if msg.providerCfg.APIKey != "" {
				m.apiInput.SetValue(msg.providerCfg.APIKey)
			}
			if msg.providerCfg.BaseURL != "" {
				m.baseURLInput.SetValue(msg.providerCfg.BaseURL)
			}
			if msg.providerCfg.Model != "" {
				m.modelInput.SetValue(msg.providerCfg.Model)
			}
			client, err := llm.BuildClient(llm.ProviderConfig{
				Provider: msg.providerCfg.Provider,
				APIKey:   msg.providerCfg.APIKey,
				BaseURL:  msg.providerCfg.BaseURL,
				Model:    msg.providerCfg.Model,
			})
			if err != nil {
				m.settingsError = err.Error()
			} else {
				m.client = client
				llm.SetActiveClient(client)
				llm.SetReactModel(msg.providerCfg.Model)
				m.qClient = qdrant.NewClient("http://127.0.0.1:6333", client)
			}
		}
		if msg.grokKey != "" {
			m.grokInput.SetValue(msg.grokKey)
		}
		if msg.vercelToken != "" {
			m.vercelInput.SetValue(msg.vercelToken)
		}
		return m, nil

	case usageLoadedMsg:
		m.usageLogs = msg.logs
		m.usageTotalIn = msg.totalIn
		m.usageTotalOut = msg.totalOut
		return m, nil

	case OllamaHealthMsg:
		m.ollamaChecked = true
		m.ollamaOK = msg.Err == nil
		if msg.Err != nil {
			m.settingsError = msg.Err.Error()
		} else if m.settingsError != "saved" {
			m.settingsError = ""
		}
		return m, nil

	case OllamaModelsMsg:
		if msg.Err != nil {
			m.settingsError = msg.Err.Error()
			return m, nil
		}
		m.modelList = msg.Models
		m.modelListIdx = moveIndex(m.modelListIdx, 0, len(m.modelList))
		if len(m.modelList) > 0 && m.modelInput.Value() == "" {
			m.modelInput.SetValue(m.modelList[m.modelListIdx])
		}
		m.settingsError = ""
		return m, nil

	case qdrantReadyMsg:
		if msg.err == nil {
			return m, m.indexProject()
		}
		return m, nil

	case indexMsg:
		// indexing completed in background — no UI action needed
		return m, nil

	case spinner.TickMsg:
		m.syncViewportIfNeeded(false)
		m.viewDirty = true // spinner frame changed

		if m.loading || m.running {
			m.spinner, cmd = m.spinner.Update(msg)
			m.nonVerboseSpinner, _ = m.nonVerboseSpinner.Update(msg)
			return m, tea.Batch(cmd, m.nonVerboseSpinner.Tick)
		}
		return m, m.nonVerboseSpinner.Tick

	case tea.WindowSizeMsg:
		followBottom := m.viewport.AtBottom()
		m.width = msg.Width
		m.height = msg.Height
		m.resizeView()
		m.syncViewportNow(followBottom)
		m.viewDirty = true

	case tea.MouseMsg:
		if msg.Action == tea.MouseActionMotion {
			return m, nil
		}

		switch msg.Button {
		case tea.MouseButtonWheelUp:
			switch m.viewMode {
			case ViewChat:
				m.viewport.LineUp(2)
				m.viewDirty = true
			case ViewDashboard:
				m.dashCursor = moveIndex(m.dashCursor, -1, len(m.dashItems))
			case ViewSettings:
				m.settingsCursor = moveIndex(m.settingsCursor, -1, settingsMaxCursor()+1)
			case ViewMCP:
				m.mcpCursor = moveIndex(m.mcpCursor, -1, len(m.mcpManager.Statuses()))
			}
			return m, nil
		case tea.MouseButtonWheelDown:
			switch m.viewMode {
			case ViewChat:
				m.viewport.LineDown(2)
				m.viewDirty = true
			case ViewDashboard:
				m.dashCursor = moveIndex(m.dashCursor, 1, len(m.dashItems))
			case ViewSettings:
				m.settingsCursor = moveIndex(m.settingsCursor, 1, settingsMaxCursor()+1)
			case ViewMCP:
				m.mcpCursor = moveIndex(m.mcpCursor, 1, len(m.mcpManager.Statuses()))
			}
			return m, nil
		case tea.MouseButtonLeft:
			if m.viewMode == ViewChat && msg.Action == tea.MouseActionPress && len(m.suggestions) > 0 {
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
			m.history[m.runOutputIdx] += ui.StatusStyle.Render("\n► done")
			m.updateViewport()
		}

		cmd := m.drainQueue()
		if cmd != nil {
			return m, cmd
		}

		// Queue is empty. If a sub-agent was executing this queue and its react loop is done, return to Zeus.
		if m.reactCh == nil && m.activeAgent != llm.AgentZeus && m.activeAgent != "" {
			m.activeAgent = llm.AgentZeus
			m.loading = true

			prompt := fmt.Sprintf("Sub-agent has finished its PTY tool execution. Analyze the outcome, call complete_step for your current task, and either delegate the next step or provide a final ANSWER to the user.")

			if m.taskGraph != nil && m.taskGraph.Root != nil {
				m.activeTaskID = m.taskGraph.Root.ID
			}

			ch, reactCmd := llm.StartReact(llm.GetActiveClient(), llm.AgentZeus, prompt, m.buildChatContext(), nil, m.mcpToolDescs(), m.executor)
			m.reactCh = ch
			m.bridge.setChannel(ch)
			return m, tea.Batch(reactCmd, m.spinner.Tick)
		}

		return m, nil

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
		actionStr := humanizeAction(msg.Tool, msg.Args)
		parsedMsg := badge + " " + renderMarkdown(actionStr)
		m.pushOutput(parsedMsg)
		if tid := m.activeTaskID; tid != "" {
			m.taskGraph.AddToolCall(tid, msg.Tool+": "+truncate(msg.Display, 40))
		}
		return m, llm.WaitReact(m.reactCh)

	case llm.TaskPlanMsg:
		steps := msg.Steps
		if len(steps) > 0 {
			parentID := m.activeTaskID
			if parentID == "" && m.taskGraph.Root != nil {
				parentID = m.taskGraph.Root.ID
			}
			m.taskGraph.AddPlannedSteps(parentID, steps)
			m.taskGraph.ActivateNextPending()
		}
		return m, llm.WaitReact(m.reactCh)

	case llm.TaskStepDoneMsg:
		m.taskGraph.CompleteStepByLabel(msg.Step)
		m.taskGraph.ActivateNextPending()
		return m, llm.WaitReact(m.reactCh)

	case llm.ToolResultMsg:
		rendered := renderToolResult(msg.Tool, msg.Args, msg.Result)
		m.pushOutput(rendered)
		m.appendChatLog("tool:" + msg.Tool + " → " + truncate(msg.Result, 150))
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
		ch, reactCmd := llm.StartReact(llm.GetActiveClient(), msg.Target, msg.Task, msg.Context, nil, m.mcpToolDescs(), m.executor)
		m.reactCh = ch
		m.bridge.setChannel(ch)
		return m, tea.Batch(reactCmd, m.spinner.Tick)

	case llm.ReactAnswerMsg:
		m.endThinkBlock()
		m.loading = false
		m.reactCh = nil

		if m.activeTaskID != "" {
			m.taskGraph.SetStatus(m.activeTaskID, ui.TaskDone)
		}

		text := strings.TrimSpace(msg.Text)
		if text != "" {
			m.appendChatLog(string(msg.Agent) + ": " + truncate(text, 150))
		}
		m.suggestions = nil
		m.selectedSug = -1

		if idx := strings.LastIndex(text, "SUGGESTIONS: "); idx != -1 {
			_ = json.Unmarshal([]byte(text[idx+len("SUGGESTIONS: "):]), &m.suggestions)
			text = strings.TrimSpace(text[:idx])
		}
		m.resizeView()

		if text != "" {
			humanizedText := humanizeToolJSON(text)
			m.pushAgentOutput(msg.Agent, renderMarkdown(humanizedText))
			// Persist assistant response and log token usage.
			if m.db != nil && m.activeChatID != 0 {
				_ = m.db.SaveMessage(context.Background(), m.activeChatID, "assistant", text)
			}
			if m.db != nil && (m.sessionIn > 0 || m.sessionOut > 0) {
				_ = m.db.LogUsage(context.Background(), string(msg.Agent), m.curInputTokens, m.curOutputTokens)
			}
		}

		if reqs := extractToolRequests(text); len(reqs) > 0 {
			m.pushAgentOutput(msg.Agent, fmt.Sprintf("executing %d tool(s)...", len(reqs)))
			m.pendingQueue = reqs
			cmd := m.drainQueue()
			// If executing a tool natively returns a tea.Cmd (like running PTY), dispatch it.
			// But for sub-agents, we must ensure Zeus kicks back in AFTER the PTY queue finishes.
			// m.drainQueue doesn't inherently loop back to LLM if it's not a tool call msg.
			// To fix this gracefully without breaking PTY loops, we process the queue,
			// and then immediately drop down to the Zeus orchestration handoff below.
			if cmd != nil {
				return m, cmd // Wait, if it's PTY, returning cmd means we pause.
				// We actually need a way to resume Zeus after PTY finishes (in apptty.DoneMsg).
			}
		}

		// ORCHESTRATION LOOP FIX:
		// If the worker was a sub-agent (not Zeus), control MUST return to Zeus to continue the master plan!
		if msg.Agent != llm.AgentZeus {
			m.activeAgent = llm.AgentZeus
			m.loading = true

			prompt := fmt.Sprintf("Sub-agent %s has finished its delegated execution. Analyze the outcome, call complete_step for your current task, and either delegate the next step or provide a final ANSWER to the user.", msg.Agent)

			if m.taskGraph != nil && m.taskGraph.Root != nil {
				m.activeTaskID = m.taskGraph.Root.ID
			}

			ch, reactCmd := llm.StartReact(llm.GetActiveClient(), llm.AgentZeus, prompt, m.buildChatContext(), nil, m.mcpToolDescs(), m.executor)
			m.reactCh = ch
			m.bridge.setChannel(ch)
			return m, tea.Batch(reactCmd, m.spinner.Tick)
		}

	case llm.AgentPromptMsg:
		m.agentPromptQ = msg.Question
		m.agentPromptT = msg.InputType
		m.agentPromptReplyCh = msg.ReplyCh
		m.agentPromptInput.Reset()
		m.agentPromptInput.SetValue("")
		m.agentPromptInput.Focus()
		m.viewMode = ViewAgentPrompt
		m.viewDirty = true
		return m, tea.Batch(llm.WaitReact(m.reactCh), textinput.Blink)

	case llm.ReactDoneMsg:
		m.endThinkBlock()
		m.loading = false
		m.reactCh = nil

	case llm.ReactStepLimitMsg:
		m.endThinkBlock()
		m.loading = false
		m.reactCh = nil
		if m.activeTaskID != "" {
			m.taskGraph.SetStatus(m.activeTaskID, ui.TaskFailed)
		}
		m.pushAgentOutput(msg.Agent, ui.WarnStyle.Render(
			fmt.Sprintf("⚠ Reached %d steps — asking Zeus to re-plan...", msg.StepsDone)))

		recoveryPrompt := fmt.Sprintf(
			"The agent %s hit the step limit (%d steps) while working on:\n\n%s\n\nLast known thought: %s\n\nReview what was accomplished and what still needs to be done. Re-delegate or continue the work.",
			msg.Agent, msg.StepsDone, msg.Prompt, msg.LastThought,
		)
		ctx := m.buildChatContext()
		childID := m.taskGraph.AddChild(m.activeTaskID, "Zeus", "re-planning after step limit")
		m.taskGraph.SetStatus(childID, ui.TaskRunning)
		m.activeTaskID = childID
		ch, reactCmd := llm.StartReact(llm.GetActiveClient(), llm.AgentZeus, recoveryPrompt, ctx, nil, m.mcpToolDescs(), m.executor)
		m.reactCh = ch
		m.bridge.setChannel(ch)
		m.loading = true
		return m, tea.Batch(reactCmd, m.spinner.Tick)

	case llm.ReactErrorMsg:
		m.endThinkBlock()
		m.loading = false
		m.reactCh = nil
		if m.activeTaskID != "" {
			m.taskGraph.SetStatus(m.activeTaskID, ui.TaskFailed)
		}
		errBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(brandDanger).
			Foreground(brandDanger).
			Bold(true).
			Padding(0, 2).
			Render("!  " + string(msg.Agent) + " Error\n\n" + msg.Err.Error())
		m.pushOutput(errBox)

	case chatHistoryMsg:
		if len(msg.messages) == 0 {
			return m, nil
		}
		themisStyle := lipgloss.NewStyle().Foreground(brandSecondary).Bold(true)
		for _, hm := range msg.messages {
			switch hm.Role {
			case "user":
				m.pushOutput("You > " + hm.Content)
			case "assistant":
				badge := themisStyle.Render("● Themis")
				m.pushOutput(badge + " ›\n" + renderMarkdown(hm.Content))
			}
		}
		m.pushOutput(lipgloss.NewStyle().Foreground(brandDim).Italic(true).Render("── history above, new messages below ──"))
		return m, nil

	case worker.ProgressMsg:
		// background worker updates, re-render tasks view
		return m, nil

	case mcpReadyMsg:
		// MCP servers started in background; no UI action needed.
		return m, nil

	case llm.TokenUpdateMsg:
		m.curInputTokens = msg.InputTokens
		m.curOutputTokens = msg.OutputTokens
		m.tokenStreaming = !msg.IsFinal
		if msg.IsFinal {
			m.sessionIn += msg.InputTokens
			m.sessionOut += msg.OutputTokens
		}
		// RE-SUBSCRIBE to the stream!
		return m, llm.WaitReact(m.reactCh)

	case blinkMsg:
		m.viewDirty = true
		if m.isRecording {
			return m, blinkTick()
		}
		return m, nil

	case transcriptionMsg:
		m.loading = false
		if msg.err != nil {
			m.pushOutput("STT Error: " + msg.err.Error())
		} else if msg.text != "" {
			cur := m.input.Value()
			if cur != "" && !strings.HasSuffix(cur, " ") {
				cur += " "
			}
			m.input.SetValue(cur + msg.text)
			m.input.CursorEnd()
		}
		return m, nil

	case imagePickedMsg:
		if msg.path != "" {
			m.pendingImages = append(m.pendingImages, msg.path)
			m.pushOutput(fmt.Sprintf("+ Image attached: %s", filepath.Base(msg.path)))
		}
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
		case "ctrl+p":
			m.viewMode = ViewMCP
			return m, nil
		case "ctrl+s":
			m.viewMode = ViewSettings
			// Set cursor to current theme index.
			for i, name := range ui.ThemeOrder {
				if name == m.themeName {
					m.settingsCursor = settingsThemeStart + i
				}
			}
			return m, m.loadUsageData()
		case "ctrl+r":
			if m.viewMode == ViewChat {
				if !m.isRecording {
					if err := audio.StartRecording("/tmp/voice.wav"); err == nil {
						m.isRecording = true
						return m, blinkTick()
					} else {
						m.pushOutput("Audio Error: " + err.Error())
					}
					return m, nil
				} else {
					audio.StopRecording()
					m.isRecording = false
					m.loading = true
					return m, func() tea.Msg {
						t, e := audio.Transcribe(context.Background(), m.grokInput.Value(), "/tmp/voice.wav")
						return transcriptionMsg{text: t, err: e}
					}
				}
			}
		case "ctrl+o":
			return m, pickImageFile()
		case "ctrl+v":
			m.verboseMode = !m.verboseMode
			if m.verboseMode {
				banner := lipgloss.NewStyle().
					Foreground(lipgloss.Color("40")).
					Bold(true).
					Render("● Verbose mode ON — full agent thinking shown")
				m.pushOutput(banner)
			} else {
				banner := lipgloss.NewStyle().
					Foreground(lipgloss.Color("99")).
					Bold(true).
					Render("◉ Quiet mode ON — compact loaders active  [ctrl+v to restore]")
				m.pushOutput(banner)
			}
			return m, nil
		}

		// ── MCP view ──
		if m.viewMode == ViewMCP {
			return m.updateMCPView(msg)
		}

		// ── Agent prompt view ──
		if m.viewMode == ViewAgentPrompt {
			return m.updateAgentPrompt(msg)
		}

		// ── Settings view ──
		if m.viewMode == ViewSettings {
			return m.updateProviderSettings(msg)
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

			// Expand continuation prompts into full task descriptions
			effectivePrompt := userPrompt
			if isContinuation(userPrompt) && m.lastUserPrompt != "" {
				effectivePrompt = "Continue the previous task: " + m.lastUserPrompt +
					"\n\nThe user said: " + userPrompt
			} else {
				m.lastUserPrompt = userPrompt
			}

			m.appendChatLog("User: " + userPrompt)
			m.pushOutput("You > " + userPrompt)
			m.input.SetValue("")
			m.loading = true

			// Persist message to DB — create chat on first message if needed.
			if m.db != nil {
				if m.activeChatID == 0 {
					title := userPrompt
					if len(title) > 60 {
						title = title[:57] + "…"
					}
					if id, err := m.db.CreateChat(context.Background(), m.activeProjectID, title); err == nil {
						m.activeChatID = int(id)
						m.dashItems = buildDashItems(m.db)
					}
				}
				if m.activeChatID != 0 {
					_ = m.db.SaveMessage(context.Background(), m.activeChatID, "user", userPrompt)
					_ = m.db.TouchChat(context.Background(), m.activeChatID)
				}
			}
			m.curInputTokens = 0
			m.curOutputTokens = 0
			m.tokenStreaming = false
			m.activeAgent = llm.AgentZeus
			m.suggestions = nil
			m.selectedSug = -1
			m.resizeView()

			m.taskGraph = ui.NewTaskGraph()
			rootID := m.taskGraph.AddRoot("Zeus", truncate(effectivePrompt, 50))
			m.activeTaskID = rootID

			chatCtx := m.buildChatContext()
			ep := effectivePrompt   // capture for closure
			imgs := m.pendingImages // capture images; clear from model
			m.pendingImages = nil

			// Build combined context: chat history + Qdrant vector search
			ch := make(chan tea.Msg, 32)
			m.reactCh = ch
			m.bridge.setChannel(ch)
			return m, tea.Batch(
				m.spinner.Tick,
				llm.WaitReact(ch),
				func() tea.Msg {
					var ctxParts []string
					// Skip chatLog when images are attached — previous image descriptions
					// in the log cause the LLM to describe the wrong image.
					if chatCtx != "" && len(imgs) == 0 {
						ctxParts = append(ctxParts, chatCtx)
					}
					if m.activeProjectID != 0 && m.qClient != nil {
						if vc, err := m.qClient.SearchContext(context.Background(), m.activeProjectID, ep); err == nil && vc != "" {
							ctxParts = append(ctxParts, "Relevant project files:\n"+vc)
						}
					}
					combinedCtx := strings.Join(ctxParts, "\n\n")
					go llm.RunReact(llm.GetActiveClient(), llm.AgentZeus, ep, combinedCtx, imgs, m.mcpToolDescs(), m.executor, ch)
					return nil
				},
			)
		}

		m.input, cmd = m.input.Update(msg)

		// Protective Regex Filter:
		// If terminal latency caused SGR mouse bytes to bypass bubbletea's
		// internal mouse parser and dump as string keys into our textarea,
		// we instantly vaporize them here so they never show up.
		val := m.input.Value()
		if strings.Contains(val, "[<") {
			if mouseEscapePattern.MatchString(val) {
				m.input.SetValue(mouseEscapePattern.ReplaceAllString(val, ""))
			}
		}

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
				id, _ := m.db.CreateProject(context.Background(), name, wd)
				m.dashItems = buildDashItems(m.db)
				m.dashCursor = 0
				if id > 0 {
					m.activeProjectID = int(id)
					m.activeProjectPath = wd
				}
			}
			m.dashCreating = false
			m.dashInput.SetValue("")
			return m, m.indexProject()
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
					chats, err := m.db.ListChats(context.Background(), item.id)
					if err == nil && len(chats) > 0 {
						// Resume most recent chat
						m.activeChatID = chats[0].ID
						_ = m.db.TouchChat(context.Background(), m.activeChatID)
					} else {
						// Only create a new one if no chats exist
						if id, err := m.db.CreateChat(context.Background(), item.id, "Session"); err == nil {
							m.activeChatID = int(id)
						}
					}
				}
				m.activeProjectID = item.projectID
				m.activeProjectPath = item.path
				m.viewMode = ViewChat
				m.input.Focus()
				m.pushOutput(dashSubtitle.Render("▪ Opened project: " + item.label))
				return m, tea.Batch(m.indexProject(), m.loadChatHistory(m.activeChatID))
			case "chat":
				if m.db != nil {
					_ = m.db.TouchChat(context.Background(), item.id)
				}
				m.activeChatID = item.id
				m.activeProjectID = item.projectID
				m.viewMode = ViewChat
				m.input.Focus()
				m.pushOutput(dashSubtitle.Render("· Resumed chat: " + item.label))
				return m, m.loadChatHistory(item.id)
			case "action":
				if strings.Contains(item.label, "New Project") {
					m.dashCreating = true
					m.dashInput.Focus()
				} else if strings.Contains(item.label, "New Chat") {
					m.activeChatID = 0
					if m.db != nil {
						if id, err := m.db.CreateChat(context.Background(), m.activeProjectID, "New Chat"); err == nil {
							m.activeChatID = int(id)
						}
					}
					m.viewMode = ViewChat
					m.input.Focus()
					m.pushOutput(dashSubtitle.Render("· New chat session started"))
				} else if strings.Contains(item.label, "MCP") {
					m.viewMode = ViewMCP
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
	case ViewMCP:
		return m.renderMCPView()
	case ViewSettings:
		return m.renderProviderSettings()
	case ViewAgentPrompt:
		return m.renderAgentPrompt()
	default:
		// Cache renderChat() output — it's extremely expensive due to
		// taskGraph.Render(), fitBlock(), and lipgloss.Join* running
		// on every single View() call (30-50x/sec during streaming).
		if !m.viewDirty && m.viewCache != "" {
			return m.viewCache
		}
		m.viewCache = m.renderChat()
		m.viewDirty = false
		return m.viewCache
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

	// Reset and render the actual items
	sb.Reset()
	sb.WriteString(logoStyle.Render(logoArt))
	sb.WriteString("\n")
	sb.WriteString("\n\n")
	sb.WriteString("  " + sep + "\n\n")

	maxItems := h - 18
	if maxItems < 5 {
		maxItems = 5
	}
	startIdx := 0
	endIdx := len(m.dashItems)

	if len(m.dashItems) > maxItems {
		startIdx = m.dashCursor - maxItems/2
		if startIdx < 0 {
			startIdx = 0
		}
		endIdx = startIdx + maxItems
		if endIdx > len(m.dashItems) {
			endIdx = len(m.dashItems)
			startIdx = endIdx - maxItems
			if startIdx < 0 {
				startIdx = 0
			}
		}
	}

	lastKind := ""
	for i := startIdx; i < endIdx; i++ {
		item := m.dashItems[i]
		// Section headers
		if item.kind != lastKind {
			switch item.kind {
			case "project":
				sb.WriteString("  " + dashSectionTitle.Render("[ PROJECTS ]") + "\n")
			case "chat":
				if lastKind != "" {
					sb.WriteString("\n")
				}
				sb.WriteString("  " + dashSectionTitle.Render("[ RECENT CHATS ]") + "\n")
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
	// MCP status badges
	mcpStatuses := m.mcpManager.Statuses()
	var mcpReady, mcpTotal int
	for _, s := range mcpStatuses {
		if s.Config.Enabled {
			mcpTotal++
			if s.Ready {
				mcpReady++
			}
		}
	}
	var mcpBadge string
	if mcpTotal == 0 {
		mcpBadge = lipgloss.NewStyle().Foreground(brandDim).Render("○ MCP")
	} else if mcpReady == mcpTotal {
		mcpBadge = lipgloss.NewStyle().Foreground(brandSuccess).Bold(true).
			Render(fmt.Sprintf("● MCP (%d/%d servers)", mcpReady, mcpTotal))
	} else {
		mcpBadge = lipgloss.NewStyle().Foreground(brandAccent).
			Render(fmt.Sprintf("◌ MCP (%d/%d ready)", mcpReady, mcpTotal))
	}
	sb.WriteString("  " + qdrantBadge + "  │  " + mcpBadge + "\n\n")

	// Status bar
	activeCount := 0
	m.workers.ForEach(func(_ string, _ *worker.Task) { activeCount++ })

	statusParts := []string{
		lipgloss.NewStyle().Foreground(brandDim).Render("↑↓/jk navigate"),
		lipgloss.NewStyle().Foreground(brandDim).Render("enter select"),
		lipgloss.NewStyle().Foreground(brandDim).Render("n new project"),
		lipgloss.NewStyle().Foreground(brandDim).Render("ctrl+t tasks"),
		lipgloss.NewStyle().Foreground(brandPrimary).Render("ctrl+p MCP servers"),
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
	screenW, screenH := screenSize(m.width, m.height)
	layoutW := max(1, screenW-1)

	status := "Ready"
	if m.loading {
		agentBadge := ""
		if m.activeAgent != "" {
			agentBadge = ui.AgentStyle(string(m.activeAgent)).Render(
				llm.AgentEmoji(m.activeAgent)+" "+string(m.activeAgent)) + " "
		}
		if m.verboseMode {
			status = m.spinner.View() + " " + agentBadge + "Thinking..."
		} else {
			// Quiet mode: show the cool spinner with a compact tag
			status = m.nonVerboseSpinner.View() + " " + agentBadge +
				lipgloss.NewStyle().Foreground(lipgloss.Color("99")).Render("working") +
				"  " + lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("[ctrl+v for verbose]")
		}
	} else if m.running {
		status = m.spinner.View() + " Running..."
	}

	mainW, graphW := chatColumns(layoutW)
	leftContentW := contentWidth(mainW, ui.BorderStyle)

	bodyContent := lipgloss.NewStyle().
		Width(leftContentW).
		MaxWidth(leftContentW).
		Height(m.viewport.Height).
		MaxHeight(m.viewport.Height).
		Render(m.viewport.View())
	leftPanel := ui.BorderStyle.Copy().Width(leftContentW).Render(
		ui.TitleStyle.Render("Themis") + "\n\n" + bodyContent)

	var topRow string
	if graphW > 0 {
		graphH := m.viewport.Height + 4
		rightPanel := m.taskGraph.Render(max(1, graphW-2), graphH)
		topRow = lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
	} else {
		topRow = leftPanel
	}

	var sugView string
	if len(m.suggestions) > 0 {
		lines := make([]string, len(m.suggestions))
		for i, s := range m.suggestions {
			if i == m.selectedSug {
				lines[i] = clipLine(ui.SelectedSuggestionStyle.Render("[*] "+s), layoutW)
			} else {
				lines[i] = clipLine(ui.SuggestionStyle.Render("[ ] "+s), layoutW)
			}
		}
		sugView = lipgloss.JoinVertical(lipgloss.Left, lines...)
	}

	var footer string
	if m.review != nil {
		footer = m.reviewFooter()
	} else {
		inputView := m.input.View()
		if m.isRecording {
			pulse := "●"
			if time.Now().UnixMilli()%1000 < 500 {
				pulse = "○"
			}
			recStr := lipgloss.NewStyle().Foreground(brandDanger).Bold(true).Render(pulse + " RECORDING AUDIO... [ |||||||||||||||||| ] (press ctrl+r to stop and transcribe)")
			inputView = recStr + "\n" + inputView
		}
		if len(m.pendingImages) > 0 {
			inputView += fmt.Sprintf("\n  [+] %d image(s) attached (ctrl+o to add more)", len(m.pendingImages))
		}
		footer = ui.BorderStyle.Copy().Width(contentWidth(layoutW, ui.BorderStyle)).Render(inputView)
	}

	tokenBar := clipLine(m.renderTokenBar(), layoutW)
	helpText := status + "  │  " + m.help.View(ui.Keys) + "  │  esc: dashboard  ctrl+t: tasks  ctrl+p: MCP  ctrl+o: image"
	helpBar := ui.StatusStyle.Render(clipLine(helpText, layoutW))

	parts := []string{topRow}
	if sugView != "" {
		parts = append(parts, sugView)
	}
	parts = append(parts, footer, tokenBar, helpBar)
	return fitBlock(lipgloss.JoinVertical(lipgloss.Left, parts...), layoutW, screenH)
}

func (m model) renderTokenBar() string {
	inStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))   // blue
	outStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("205")) // pink
	dimStyle := lipgloss.NewStyle().Foreground(brandDim)
	sessStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	inStr := fmtTokens(m.curInputTokens)
	outStr := fmtTokens(m.curOutputTokens)

	streamSuffix := ""
	if m.tokenStreaming {
		streamSuffix = "…"
	}

	var parts []string
	if m.curInputTokens > 0 || m.curOutputTokens > 0 {
		parts = append(parts,
			inStyle.Render("↑ "+inStr+" in"),
			outStyle.Render("↓ "+outStr+streamSuffix+" out"),
		)
	}
	if m.sessionIn > 0 {
		parts = append(parts,
			sessStyle.Render("session: "+fmtTokens(m.sessionIn+m.sessionOut)+" total"),
		)
	}

	if len(parts) == 0 {
		return dimStyle.Render("  tokens: —")
	}
	return dimStyle.Render("  ") + strings.Join(parts, dimStyle.Render("  │  "))
}

// mcpToolDescs builds a newline-delimited list of mcp__server__tool descriptions
// for all currently connected MCP servers, to be injected into the agent system prompt.
func (m model) mcpToolDescs() string {
	if m.mcpManager == nil {
		return ""
	}
	tools := m.mcpManager.AllTools()
	if len(tools) == 0 {
		return ""
	}
	// Keep descriptions compact — just name + one-line description, max 40 tools.
	var sb strings.Builder
	limit := len(tools)
	if limit > 40 {
		limit = 40
	}
	for _, t := range tools[:limit] {
		desc := t.Description
		if len(desc) > 80 {
			desc = desc[:77] + "…"
		}
		sb.WriteString(fmt.Sprintf(`{"tool":"%s"} — %s`+"\n", t.Name, desc))
	}
	return sb.String()
}

// renderToolResult formats a tool result for display, applying diff coloring
// or syntax highlighting when relevant.
func renderToolResult(tool string, args map[string]interface{}, result string) string {
	// Diff coloring: git_diff results or any result that looks like unified diff.
	if tool == "git_diff" || syntax.IsDiff(result) {
		colored := syntax.ColorDiff(result)
		return ui.ObservationStyle.Render("[#] "+tool) + "\n" + colored
	}

	// Syntax highlighting for file reads.
	if tool == "read_file" || tool == "create_file" || tool == "write_file" {
		path := ""
		if args != nil {
			if p, ok := args["path"].(string); ok {
				path = p
			}
		}
		if path != "" && looksLikeCode(path) {
			highlighted := syntax.Highlight(result, path)
			return ui.ObservationStyle.Render("[#] "+path) + "\n" + highlighted
		}
	}

	// MCP tool calls — show server name prominently.
	if strings.HasPrefix(tool, "mcp__") {
		parts := strings.SplitN(tool, "__", 3)
		header := "[*] MCP"
		if len(parts) == 3 {
			header = fmt.Sprintf("[*] MCP[%s] %s", parts[1], parts[2])
		}
		return ui.ObservationStyle.Render("[#] "+header) + "\n" +
			lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Render(truncate(result, 800))
	}

	return ui.ObservationStyle.Render("[#] " + truncate(result, 500))
}

func looksLikeCode(path string) bool {
	codeExts := []string{".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".rs", ".java",
		".c", ".cpp", ".h", ".cs", ".rb", ".sh", ".yaml", ".yml", ".json",
		".toml", ".html", ".css", ".scss", ".md", ".sql", ".tf", ".kt", ".swift"}
	lower := strings.ToLower(path)
	for _, ext := range codeExts {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

func fmtTokens(n int) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1000 {
		return fmt.Sprintf("%d,%03d", n/1000, n%1000)
	}
	return fmt.Sprintf("%d", n)
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
	width := contentWidth(m.width-1, ui.BorderStyle)
	return ui.BorderStyle.Copy().Width(width).Render(
		clipLine(strings.Join(opts, " "), width) + "\n" + clipLine(hint, width))
}

func renderMarkdown(content string) string {
	// Let glamour automatically detect width from TTY or default to 80.
	// Dark style ensures code blocks are syntax highlighted nicely.
	out, err := glamour.Render(content, "dark")
	if err == nil {
		return strings.TrimSpace(out)
	}
	// Fallback if Markdown parsing fails
	return ui.AnswerStyle.Render(content)
}

// ── Settings view ───────────────────────────────────────────────────────

const settingsThemeStart = 5

func settingsMaxCursor() int {
	return settingsThemeStart + len(ui.ThemeOrder) - 1
}

func ollamaHealthCmd(baseURL string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		return OllamaHealthMsg{Err: llm.OllamaHealth(ctx, baseURL)}
	}
}

func ollamaModelsCmd(baseURL string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		models, err := llm.ListOllamaModels(ctx, baseURL)
		return OllamaModelsMsg{Models: models, Err: err}
	}
}

func (m model) providerConfigRow() dbx.ProviderConfigRow {
	provider := providerName(m.providerIdx)
	cfg := dbx.ProviderConfigRow{
		Provider: provider,
		APIKey:   m.apiInput.Value(),
		Model:    m.modelInput.Value(),
	}
	if provider == "ollama" {
		cfg.APIKey = ""
		cfg.BaseURL = m.baseURLInput.Value()
	}
	return cfg
}

func (m model) saveProviderConfig() (model, tea.Cmd) {
	cfg := m.providerConfigRow()
	client, err := llm.BuildClient(llm.ProviderConfig{
		Provider: cfg.Provider,
		APIKey:   cfg.APIKey,
		BaseURL:  cfg.BaseURL,
		Model:    cfg.Model,
	})
	if err != nil {
		m.settingsError = err.Error()
		return m, nil
	}
	if m.db != nil {
		if err := m.db.SaveProviderConfig(context.Background(), cfg); err != nil {
			m.settingsError = err.Error()
			return m, nil
		}
		_ = m.db.SetSetting(context.Background(), "api_key", cfg.APIKey)
		_ = m.db.SetSetting(context.Background(), "llm_model", cfg.Model)
	}
	m.client = client
	llm.SetActiveClient(client)
	llm.SetReactModel(cfg.Model)
	llm.CurrentAPIKey = cfg.APIKey
	m.qClient = qdrant.NewClient("http://127.0.0.1:6333", client)
	m.settingsError = "saved"
	if cfg.Provider == "ollama" {
		return m, ollamaHealthCmd(cfg.BaseURL)
	}
	return m, nil
}

func (m model) updateProviderSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg.String() {
	case "esc", "ctrl+s":
		m.viewMode = ViewDashboard
		m.apiInput.Blur()
		m.baseURLInput.Blur()
		m.modelInput.Blur()
		m.grokInput.Blur()
		m.vercelInput.Blur()
		return m, nil
	case "ctrl+w":
		return m.saveProviderConfig()
	case "ctrl+r":
		if m.providerIdx == 2 {
			return m, tea.Batch(ollamaHealthCmd(m.baseURLInput.Value()), ollamaModelsCmd(m.baseURLInput.Value()))
		}
	case "ctrl+n":
		if m.providerIdx == 2 && len(m.modelList) > 0 {
			m.modelListIdx = moveIndex(m.modelListIdx, 1, len(m.modelList))
			m.modelInput.SetValue(m.modelList[m.modelListIdx])
			return m, nil
		}
	case "ctrl+b":
		if m.providerIdx == 2 && len(m.modelList) > 0 {
			m.modelListIdx = moveIndex(m.modelListIdx, -1, len(m.modelList))
			m.modelInput.SetValue(m.modelList[m.modelListIdx])
			return m, nil
		}
	case "up", "shift+tab":
		m.settingsCursor = moveIndex(m.settingsCursor, -1, settingsMaxCursor()+1)
	case "down", "tab":
		m.settingsCursor = moveIndex(m.settingsCursor, 1, settingsMaxCursor()+1)
	case "left":
		if m.settingsCursor == 0 {
			m.providerIdx = moveIndex(m.providerIdx, -1, 3)
			m.settingsError = ""
			if m.providerIdx == 2 {
				cmds = append(cmds, ollamaHealthCmd(m.baseURLInput.Value()))
			}
		}
	case "right", " ":
		if m.settingsCursor == 0 {
			m.providerIdx = (m.providerIdx + 1) % 3
			m.settingsError = ""
			if m.providerIdx == 2 {
				cmds = append(cmds, ollamaHealthCmd(m.baseURLInput.Value()))
			}
		}
	case "enter":
		if m.settingsCursor == 0 {
			m.providerIdx = (m.providerIdx + 1) % 3
			m.settingsError = ""
			if m.providerIdx == 2 {
				cmds = append(cmds, ollamaHealthCmd(m.baseURLInput.Value()))
			}
		} else if m.settingsCursor == 1 && m.providerIdx == 2 {
			cmds = append(cmds, ollamaHealthCmd(m.baseURLInput.Value()))
		} else if m.settingsCursor >= settingsThemeStart {
			themeIdx := m.settingsCursor - settingsThemeStart
			if themeIdx < len(ui.ThemeOrder) {
				name := ui.ThemeOrder[themeIdx]
				m.themeName = name
				applyTheme(ui.GetTheme(name))
				if m.db != nil {
					_ = m.db.SetSetting(context.Background(), "theme", name)
				}
			}
		}
	}

	m.apiInput.Blur()
	m.baseURLInput.Blur()
	m.modelInput.Blur()
	m.grokInput.Blur()
	m.vercelInput.Blur()

	switch m.settingsCursor {
	case 1:
		if m.providerIdx == 2 {
			m.baseURLInput.Focus()
			m.baseURLInput, cmd = m.baseURLInput.Update(msg)
		} else {
			m.apiInput.Focus()
			m.apiInput, cmd = m.apiInput.Update(msg)
		}
		cmds = append(cmds, cmd)
	case 2:
		m.modelInput.Focus()
		m.modelInput, cmd = m.modelInput.Update(msg)
		cmds = append(cmds, cmd)
	case 3:
		m.grokInput.Focus()
		m.grokInput, cmd = m.grokInput.Update(msg)
		cmds = append(cmds, cmd)
		if m.db != nil {
			_ = m.db.SetSetting(context.Background(), "grok_key", m.grokInput.Value())
		}
	case 4:
		m.vercelInput.Focus()
		m.vercelInput, cmd = m.vercelInput.Update(msg)
		cmds = append(cmds, cmd)
		val := m.vercelInput.Value()
		if m.db != nil {
			_ = m.db.SetSetting(context.Background(), "vercel_token", val)
		}
		_ = os.Setenv("VERCEL_TOKEN", val)
	default:
		if msg.String() == "k" {
			m.settingsCursor = moveIndex(m.settingsCursor, -1, settingsMaxCursor()+1)
		} else if msg.String() == "j" {
			m.settingsCursor = moveIndex(m.settingsCursor, 1, settingsMaxCursor()+1)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m model) renderProviderSettings() string {
	w := m.width - 4
	if w < 40 {
		w = 40
	}
	t := ui.GetTheme(m.themeName)
	if m.themeName == "" {
		t = ui.GetTheme("default")
	}
	accent := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	sectionStyle := lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(t.Dim)
	activeStyle := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	errStyle := lipgloss.NewStyle().Foreground(t.Danger).Bold(true)
	okStyle := lipgloss.NewStyle().Foreground(t.Success).Bold(true)
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Secondary).
		Padding(1, 2).
		Width(w)

	cursor := func(row int) string {
		if m.settingsCursor == row {
			return activeStyle.Render("> ")
		}
		return "  "
	}
	providers := []string{"Anthropic", "OpenAI", "Ollama"}
	var tabs []string
	for i, p := range providers {
		label := " " + p + " "
		if i == m.providerIdx {
			tabs = append(tabs, activeStyle.Copy().Underline(true).Render(label))
		} else {
			tabs = append(tabs, dimStyle.Render(label))
		}
	}

	var sb strings.Builder
	sb.WriteString(accent.Render("Settings") + "\n\n")
	sb.WriteString(sectionStyle.Render("-- LLM Provider") + "\n\n")
	sb.WriteString(cursor(0) + strings.Join(tabs, "  ") + "\n")
	if m.providerIdx == 2 {
		sb.WriteString(fmt.Sprintf("%s%-10s %s\n", cursor(1), "Base URL:", m.baseURLInput.View()))
		if m.ollamaChecked {
			if m.ollamaOK {
				sb.WriteString("  " + okStyle.Render("connected") + "\n")
			} else {
				sb.WriteString("  " + errStyle.Render("unreachable - run: ollama serve") + "\n")
			}
		}
	} else {
		sb.WriteString(fmt.Sprintf("%s%-10s %s\n", cursor(1), "API Key:", m.apiInput.View()))
	}
	sb.WriteString(fmt.Sprintf("%s%-10s %s\n", cursor(2), "Model:", m.modelInput.View()))
	if m.providerIdx == 2 {
		if len(m.modelList) == 0 {
			sb.WriteString(dimStyle.Render("  ctrl+r refresh models") + "\n")
		} else {
			sb.WriteString(dimStyle.Render("  ctrl+n/ctrl+b select model") + "\n")
			limit := len(m.modelList)
			if limit > 6 {
				limit = 6
			}
			for i := 0; i < limit; i++ {
				prefix := "    "
				name := dimStyle.Render(m.modelList[i])
				if i == m.modelListIdx {
					prefix = activeStyle.Render("  > ")
					name = activeStyle.Render(m.modelList[i])
				}
				sb.WriteString(prefix + name + "\n")
			}
		}
	} else {
		sb.WriteString(fmt.Sprintf("  %-10s %s\n", "Proxy:", dimStyle.Render("https://litellm-proxy-93ef.onrender.com/v1")))
	}
	if m.settingsError != "" {
		style := errStyle
		if m.settingsError == "saved" {
			style = okStyle
		}
		sb.WriteString("  " + style.Render(m.settingsError) + "\n")
	}
	sb.WriteString(dimStyle.Render("  ctrl+w save provider") + "\n\n")

	sb.WriteString(sectionStyle.Render("-- Whisper STT") + "\n\n")
	sb.WriteString(fmt.Sprintf("%s%-10s %s\n\n", cursor(3), "Grok API:", m.grokInput.View()))

	sb.WriteString(sectionStyle.Render("-- Vercel Deployment") + "\n\n")
	vercelStatus := ""
	if m.vercelInput.Value() != "" {
		vercelStatus = " " + okStyle.Render("set")
	}
	sb.WriteString(fmt.Sprintf("%s%-10s %s%s\n\n", cursor(4), "Token:", m.vercelInput.View(), vercelStatus))

	sb.WriteString(sectionStyle.Render("-- Theme") + "\n\n")
	for i, name := range ui.ThemeOrder {
		th := ui.Themes[name]
		row := settingsThemeStart + i
		label := th.Name
		dot := lipgloss.NewStyle().Foreground(th.Primary).Render("*")
		isCurrent := name == m.themeName || (m.themeName == "" && name == "default")
		if row == m.settingsCursor {
			label = activeStyle.Render(label)
		} else {
			label = dimStyle.Render(label)
		}
		tag := ""
		if isCurrent {
			tag = " " + okStyle.Render("active")
		}
		sb.WriteString(fmt.Sprintf("%s%s %s%s\n", cursor(row), dot, label, tag))
	}
	sb.WriteString("\n")
	sb.WriteString(dimStyle.Render("up/down navigate - enter apply theme - ctrl+s close") + "\n")

	return borderStyle.Render(sb.String())
}

func (m model) updateSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	if msg.String() == "esc" || msg.String() == "ctrl+s" {
		m.viewMode = ViewDashboard
		m.apiInput.Blur()
		m.modelInput.Blur()
		m.grokInput.Blur()
		return m, nil
	}

	switch msg.String() {
	case "up", "shift+tab":
		if m.settingsCursor > 0 {
			m.settingsCursor--
		}
	case "down", "tab":
		if m.settingsCursor < len(ui.ThemeOrder)+3 {
			m.settingsCursor++
		}
	case "enter", " ":
		if m.settingsCursor >= 4 {
			themeIdx := m.settingsCursor - 4
			if themeIdx < len(ui.ThemeOrder) {
				name := ui.ThemeOrder[themeIdx]
				m.themeName = name
				applyTheme(ui.GetTheme(name))
				if m.db != nil {
					_ = m.db.SetSetting(context.Background(), "theme", name)
				}
			}
		}
	}

	if m.settingsCursor == 0 {
		m.apiInput.Focus()
		m.modelInput.Blur()
		m.grokInput.Blur()
		m.vercelInput.Blur()
		m.apiInput, cmd = m.apiInput.Update(msg)
		cmds = append(cmds, cmd)
		if m.db != nil {
			_ = m.db.SetSetting(context.Background(), "api_key", m.apiInput.Value())
		}
		llm.CurrentAPIKey = m.apiInput.Value()
		m.client = llm.NewClient(m.apiInput.Value())
	} else if m.settingsCursor == 1 {
		m.modelInput.Focus()
		m.apiInput.Blur()
		m.grokInput.Blur()
		m.vercelInput.Blur()
		m.modelInput, cmd = m.modelInput.Update(msg)
		cmds = append(cmds, cmd)
		if m.db != nil {
			_ = m.db.SetSetting(context.Background(), "llm_model", m.modelInput.Value())
		}
		llm.CurrentModel = m.modelInput.Value()
	} else if m.settingsCursor == 2 {
		m.grokInput.Focus()
		m.apiInput.Blur()
		m.modelInput.Blur()
		m.vercelInput.Blur()
		m.grokInput, cmd = m.grokInput.Update(msg)
		cmds = append(cmds, cmd)
		if m.db != nil {
			_ = m.db.SetSetting(context.Background(), "grok_key", m.grokInput.Value())
		}
	} else if m.settingsCursor == 3 {
		m.vercelInput.Focus()
		m.apiInput.Blur()
		m.modelInput.Blur()
		m.grokInput.Blur()
		m.vercelInput, cmd = m.vercelInput.Update(msg)
		cmds = append(cmds, cmd)
		val := m.vercelInput.Value()
		if m.db != nil {
			_ = m.db.SetSetting(context.Background(), "vercel_token", val)
		}
		_ = os.Setenv("VERCEL_TOKEN", val)
	} else {
		m.apiInput.Blur()
		m.modelInput.Blur()
		m.grokInput.Blur()
		m.vercelInput.Blur()
		// map "j" and "k" to navigate if not focused on text inputs
		if msg.String() == "k" && m.settingsCursor > 0 {
			m.settingsCursor--
		} else if msg.String() == "j" && m.settingsCursor < len(ui.ThemeOrder)+3 {
			m.settingsCursor++
		}
	}

	return m, tea.Batch(cmds...)
}

func (m model) renderSettings() string {
	w := m.width - 4
	if w < 40 {
		w = 40
	}

	t := ui.GetTheme(m.themeName)
	if m.themeName == "" {
		t = ui.GetTheme("default")
	}
	accent := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	sectionStyle := lipgloss.NewStyle().Foreground(t.Accent).Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(t.Dim)
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Secondary).
		Padding(1, 2).
		Width(w)

	var sb strings.Builder

	// ── Header ──────────────────────────────────────────────────────────
	sb.WriteString(accent.Render("⚙  Settings") + "\n\n")

	// ── API Configuration ────────────────────────────────────────────────
	sb.WriteString(sectionStyle.Render("── API Configuration") + "\n\n")

	apiCursor := "  "
	if m.settingsCursor == 0 {
		apiCursor = lipgloss.NewStyle().Foreground(t.Primary).Bold(true).Render("▸ ")
	}
	sb.WriteString(fmt.Sprintf("%s%-10s %s\n", apiCursor, "API Key:", m.apiInput.View()))

	modCursor := "  "
	if m.settingsCursor == 1 {
		modCursor = lipgloss.NewStyle().Foreground(t.Primary).Bold(true).Render("▸ ")
	}
	sb.WriteString(fmt.Sprintf("%s%-10s %s\n", modCursor, "Model:", m.modelInput.View()))

	sb.WriteString(fmt.Sprintf("  %-10s %s\n\n", "Proxy:", dimStyle.Render("https://litellm-proxy-93ef.onrender.com/v1")))

	// ── Grok / STT ───────────────────────────────────────────────────────
	sb.WriteString(sectionStyle.Render("── Whisper STT") + "\n\n")
	grokCursor := "  "
	if m.settingsCursor == 2 {
		grokCursor = lipgloss.NewStyle().Foreground(t.Primary).Bold(true).Render("▸ ")
	}
	sb.WriteString(fmt.Sprintf("%s%-10s %s\n\n", grokCursor, "Grok API:", m.grokInput.View()))

	// ── Vercel ───────────────────────────────────────────────────────────
	sb.WriteString(sectionStyle.Render("── Vercel Deployment") + "\n\n")
	vercelCursor := "  "
	if m.settingsCursor == 3 {
		vercelCursor = lipgloss.NewStyle().Foreground(t.Primary).Bold(true).Render("▸ ")
	}
	vercelStatus := ""
	if m.vercelInput.Value() != "" {
		vercelStatus = " " + lipgloss.NewStyle().Foreground(t.Success).Render("✓ set")
	}
	sb.WriteString(fmt.Sprintf("%s%-10s %s%s\n", vercelCursor, "Token:", m.vercelInput.View(), vercelStatus))
	sb.WriteString(fmt.Sprintf("  %-10s %s\n\n", "",
		dimStyle.Render("Get token at vercel.com/account/tokens  ·  CLI installs via npx (no global install needed)")))

	// ── Theme ────────────────────────────────────────────────────────────
	sb.WriteString(sectionStyle.Render("── Theme") + "\n\n")
	for i, name := range ui.ThemeOrder {
		th := ui.Themes[name]
		cursor := "  "
		label := th.Name
		dot := lipgloss.NewStyle().Foreground(th.Primary).Render("●")
		isCurrent := name == m.themeName || (m.themeName == "" && name == "default")
		if i+4 == m.settingsCursor {
			cursor = lipgloss.NewStyle().Foreground(t.Primary).Bold(true).Render("▸ ")
			label = lipgloss.NewStyle().Foreground(t.Primary).Bold(true).Render(label)
		} else {
			label = dimStyle.Render(label)
		}
		tag := ""
		if isCurrent {
			tag = lipgloss.NewStyle().Foreground(t.Success).Render("  ✓ active")
		}
		sb.WriteString(fmt.Sprintf("%s%s %s%s\n", cursor, dot, label, tag))
	}
	sb.WriteString("\n")
	sb.WriteString(dimStyle.Render("  ↑↓ navigate · enter: apply theme") + "\n\n")

	// ── Token Usage (current session) ─────────────────────────────────
	sb.WriteString(sectionStyle.Render("── Session Tokens") + "\n\n")
	if m.sessionIn == 0 && m.sessionOut == 0 {
		sb.WriteString(dimStyle.Render("  No activity this session yet.") + "\n\n")
	} else {
		chartW := w - 8
		if chartW > 60 {
			chartW = 60
		}
		if chartW < 10 {
			chartW = 10
		}
		// Bar chart: in vs out for current session.
		bc := barchart.New(chartW, 8,
			barchart.WithNoAutoBarWidth(),
			barchart.WithBarWidth(chartW/2-2),
			barchart.WithBarGap(2),
			barchart.WithStyles(
				lipgloss.NewStyle().Foreground(t.Dim),
				lipgloss.NewStyle().Foreground(t.Dim),
			),
		)
		bc.Push(barchart.BarData{
			Label: "Input",
			Values: []barchart.BarValue{{
				Name:  "in",
				Value: float64(m.sessionIn),
				Style: lipgloss.NewStyle().Foreground(t.Secondary),
			}},
		})
		bc.Push(barchart.BarData{
			Label: "Output",
			Values: []barchart.BarValue{{
				Name:  "out",
				Value: float64(m.sessionOut),
				Style: lipgloss.NewStyle().Foreground(t.Primary),
			}},
		})
		bc.Draw()
		sb.WriteString("  " + strings.ReplaceAll(bc.View(), "\n", "\n  ") + "\n")
		sb.WriteString(fmt.Sprintf("  %s  in: %s   out: %s\n\n",
			dimStyle.Render("session"),
			lipgloss.NewStyle().Foreground(t.Secondary).Render(fmtTokens(m.sessionIn)),
			lipgloss.NewStyle().Foreground(t.Primary).Render(fmtTokens(m.sessionOut)),
		))
	}

	// ── Historical usage sparklines ──────────────────────────────────────
	if len(m.usageLogs) > 0 {
		sb.WriteString(sectionStyle.Render("── Usage History (last 20 sessions)") + "\n\n")
		sparkW := w - 8
		if sparkW > 60 {
			sparkW = 60
		}
		if sparkW < 8 {
			sparkW = 8
		}

		spIn := sparkline.New(sparkW, 4,
			sparkline.WithStyle(lipgloss.NewStyle().Foreground(t.Secondary)),
		)
		spOut := sparkline.New(sparkW, 4,
			sparkline.WithStyle(lipgloss.NewStyle().Foreground(t.Primary)),
		)
		for _, e := range m.usageLogs {
			spIn.Push(float64(e.InputTokens))
			spOut.Push(float64(e.OutputTokens))
		}
		spIn.DrawBraille()
		spOut.DrawBraille()
		sb.WriteString("  " + lipgloss.NewStyle().Foreground(t.Secondary).Render("▸ Input tokens") + "\n")
		sb.WriteString("  " + strings.ReplaceAll(spIn.View(), "\n", "\n  ") + "\n")
		sb.WriteString("  " + lipgloss.NewStyle().Foreground(t.Primary).Render("▸ Output tokens") + "\n")
		sb.WriteString("  " + strings.ReplaceAll(spOut.View(), "\n", "\n  ") + "\n\n")
	}

	// ── Lifetime Stats ───────────────────────────────────────────────────
	sb.WriteString(sectionStyle.Render("── Lifetime Stats") + "\n\n")
	if m.db == nil || (m.usageTotalIn == 0 && m.usageTotalOut == 0) {
		sb.WriteString(dimStyle.Render("  No usage recorded yet.") + "\n\n")
	} else {
		sb.WriteString(fmt.Sprintf("  Sessions logged:  %s\n",
			accent.Render(fmt.Sprintf("%d", len(m.usageLogs)))))
		sb.WriteString(fmt.Sprintf("  Total input:      %s\n",
			lipgloss.NewStyle().Foreground(t.Secondary).Bold(true).Render(fmtTokens(m.usageTotalIn))))
		sb.WriteString(fmt.Sprintf("  Total output:     %s\n\n",
			lipgloss.NewStyle().Foreground(t.Primary).Bold(true).Render(fmtTokens(m.usageTotalOut))))
	}

	sb.WriteString(dimStyle.Render("  esc: back to dashboard"))

	return borderStyle.Render(sb.String())
}

// ── Agent Prompt view ───────────────────────────────────────────────────

func (m model) updateAgentPrompt(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		if m.agentPromptReplyCh != nil {
			select {
			case m.agentPromptReplyCh <- "":
			default:
			}
		}
		m.viewMode = ViewChat
		m.viewDirty = true
		return m, nil
	case "enter":
		var reply string
		if m.agentPromptT == "confirm" {
			v := strings.ToLower(strings.TrimSpace(m.agentPromptInput.Value()))
			if v == "" || v == "y" || v == "yes" {
				reply = "yes"
			} else {
				reply = "no"
			}
		} else {
			reply = m.agentPromptInput.Value()
		}
		if m.agentPromptReplyCh != nil {
			select {
			case m.agentPromptReplyCh <- reply:
			default:
			}
		}
		m.viewMode = ViewChat
		m.viewDirty = true
		m.agentPromptInput.Reset()
		return m, nil
	case "y", "Y":
		if m.agentPromptT == "confirm" {
			if m.agentPromptReplyCh != nil {
				select {
				case m.agentPromptReplyCh <- "yes":
				default:
				}
			}
			m.viewMode = ViewChat
			m.viewDirty = true
			m.agentPromptInput.Reset()
			return m, nil
		}
	case "n", "N":
		if m.agentPromptT == "confirm" {
			if m.agentPromptReplyCh != nil {
				select {
				case m.agentPromptReplyCh <- "no":
				default:
				}
			}
			m.viewMode = ViewChat
			m.viewDirty = true
			m.agentPromptInput.Reset()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.agentPromptInput, cmd = m.agentPromptInput.Update(msg)
	return m, cmd
}

func (m model) renderAgentPrompt() string {
	t := ui.GetTheme(m.themeName)
	if m.themeName == "" {
		t = ui.GetTheme("default")
	}

	boxW := m.width - 8
	if boxW < 40 {
		boxW = 40
	}
	if boxW > 90 {
		boxW = 90
	}

	var inner strings.Builder

	agentBadge := ui.AgentStyle(string(m.activeAgent)).Render(llm.AgentEmoji(m.activeAgent) + "  " + string(m.activeAgent) + " is asking")
	inner.WriteString(agentBadge + "\n\n")

	questionStyle := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
	inner.WriteString(questionStyle.Render(m.agentPromptQ) + "\n\n")

	dimStyle := lipgloss.NewStyle().Foreground(t.Dim)

	if m.agentPromptT == "confirm" {
		yStyle := lipgloss.NewStyle().
			Background(t.Success).
			Foreground(lipgloss.Color("0")).
			Bold(true).
			Padding(0, 3)
		nStyle := lipgloss.NewStyle().
			Background(t.Danger).
			Foreground(lipgloss.Color("0")).
			Bold(true).
			Padding(0, 3)
		inner.WriteString(yStyle.Render("Y  Yes") + "   " + nStyle.Render("N  No") + "\n\n")
		inner.WriteString(dimStyle.Render("Press Y or N  ·  Esc to cancel"))
	} else {
		inner.WriteString(m.agentPromptInput.View() + "\n\n")
		inner.WriteString(dimStyle.Render("Enter to confirm  ·  Esc to cancel"))
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Primary).
		Padding(1, 3).
		Width(boxW).
		Render(inner.String())

	topPad := (m.height - lipgloss.Height(box)) / 3
	if topPad < 0 {
		topPad = 0
	}

	return strings.Repeat("\n", topPad) +
		lipgloss.NewStyle().Width(m.width).Align(lipgloss.Center).Render(box)
}

// ── MCP view ────────────────────────────────────────────────────────────

func (m model) updateMCPView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	statuses := m.mcpManager.Statuses()
	switch msg.String() {
	case "esc", "ctrl+p":
		m.viewMode = ViewDashboard
	case "up", "k":
		if m.mcpCursor > 0 {
			m.mcpCursor--
		}
	case "down", "j":
		if m.mcpCursor < len(statuses)-1 {
			m.mcpCursor++
		}
	case "enter", " ":
		if m.mcpCursor < len(statuses) {
			s := statuses[m.mcpCursor]
			ctx := context.Background()
			go m.mcpManager.ToggleServer(ctx, s.Config.Name, !s.Config.Enabled) //nolint:errcheck
		}
	case "e":
		// Open config file in $EDITOR or default editor.
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "nano"
		}
		cfgPath := m.mcpManager.ConfigPath()
		c := exec.Command(editor, cfgPath)
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		_ = c.Run()
	}
	return m, nil
}

// renderMCPView renders the MCP server dashboard.
func (m model) renderMCPView() string {
	statuses := m.mcpManager.Statuses()
	w := m.width
	if w <= 0 {
		w = 80
	}

	var sb strings.Builder
	sb.WriteString(dashTitle.Render("⚙  MCP Servers") + "\n")
	sb.WriteString(dashSubtitle.Render("esc back  ↑↓ navigate  enter enable/disable  e edit config JSON") + "\n\n")

	sep := lipgloss.NewStyle().Foreground(brandDim).Render(strings.Repeat("─", min(w-12, 60)))
	sb.WriteString("  " + sep + "\n")

	for i, s := range statuses {
		var statusStr string
		if s.Config.Enabled && s.Ready {
			toolCount := fmt.Sprintf("%d tools", len(s.Tools))
			statusStr = lipgloss.NewStyle().Foreground(brandSuccess).Bold(true).Render("● ready") +
				lipgloss.NewStyle().Foreground(brandDim).Render("  "+toolCount)
		} else if s.Config.Enabled {
			statusStr = lipgloss.NewStyle().Foreground(brandAccent).Render("◌ connecting...")
		} else {
			statusStr = lipgloss.NewStyle().Foreground(brandDim).Render("○ disabled")
		}

		name := fmt.Sprintf("%-26s", s.Config.Name)
		if i == m.mcpCursor {
			sb.WriteString(dashItemSelected.Render("▶ "+name) + statusStr + "\n")
		} else {
			sb.WriteString(dashItemNormal.Render("  "+name) + statusStr + "\n")
		}
	}

	sb.WriteString("\n  " + sep + "\n")
	sb.WriteString("\n" + dashHint.Render("Config file: "+m.mcpManager.ConfigPath()))
	sb.WriteString("\n" + dashHint.Render("Add a server: edit the JSON file directly with 'e', then restart Themis"))

	return lipgloss.NewStyle().Padding(1, 2).Render(sb.String())
}

// ── Image picker ─────────────────────────────────────────────────────────

// pickImageFile spawns an OS file-manager / dialog to pick an image.
func pickImageFile() tea.Cmd {
	return func() tea.Msg {
		path := openImageDialog()
		return imagePickedMsg{path: path}
	}
}

// openImageDialog tries common GUI file pickers in order, falling back to a
// simple terminal prompt if none are available.
func openImageDialog() string {
	if runtime.GOOS == "windows" {
		// Use PowerShell Windows Forms file dialog.
		script := `Add-Type -AssemblyName System.Windows.Forms; ` +
			`$f = New-Object System.Windows.Forms.OpenFileDialog; ` +
			`$f.Filter = 'Images|*.png;*.jpg;*.jpeg;*.gif;*.webp'; ` +
			`if ($f.ShowDialog() -eq 'OK') { $f.FileName }`
		out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script).Output()
		if err == nil {
			return strings.TrimSpace(string(out))
		}
		return ""
	}
	// Linux / macOS: zenity (GNOME), kdialog (KDE), yad, osascript (macOS)
	pickers := [][]string{
		{"zenity", "--file-selection", "--title=Select Image",
			"--file-filter=Images | *.png *.jpg *.jpeg *.gif *.webp"},
		{"kdialog", "--getopenfilename", ".", "*.png *.jpg *.jpeg *.gif *.webp"},
		{"yad", "--file-selection", "--title=Select Image"},
	}
	if runtime.GOOS == "darwin" {
		pickers = append([][]string{
			{"osascript", "-e", `choose file with prompt "Select Image" of type {"public.image"}`},
		}, pickers...)
	}
	for _, args := range pickers {
		out, err := exec.Command(args[0], args[1:]...).Output()
		if err == nil {
			return strings.TrimSpace(string(out))
		}
	}
	return ""
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
