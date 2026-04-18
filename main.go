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
	apptty "github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/tty"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/tools"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/ui"
)

// ── review types ─────────────────────────────────────────────────────────────

type reviewOpt int

const (
	optAccept    reviewOpt = iota // y — execute once
	optReject                     // n — skip
	optAcceptAll                  // a — always allow
)

type toolReview struct {
	req      tools.ToolRequest
	selected reviewOpt
}

var reviewLabels = []string{"  Accept  ", "  Reject  ", "  Accept All  "}
var reviewStyles = []lipgloss.Style{
	ui.ReviewAcceptStyle,
	ui.ReviewRejectStyle,
	ui.ReviewNeutralStyle,
}

// ── model ────────────────────────────────────────────────────────────────────

type model struct {
	client   *openai.Client
	registry *tools.Registry
	perms    *tools.PermissionManager

	viewport viewport.Model
	input    textarea.Model
	spinner  spinner.Model
	help     help.Model

	history     []string
	suggestions []string
	selectedSug int

	pendingQueue []tools.ToolRequest
	review       *toolReview // non-nil → footer shows review options

	// PTY state
	ptyMaster    *os.File
	ptyCmd       *exec.Cmd
	ptyCleanup   func()
	running      bool
	runOutputIdx int

	width  int
	height int

	loading bool
	quit    bool
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
		viewport:    vp,
		input:       ta,
		spinner:     sp,
		help:        help.New(),
		history:     []string{},
		selectedSug: -1,
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

func (m *model) pushOutput(text string) {
	m.history = append(m.history, text)
	m.viewport.SetContent(ui.OutputStyle.Render(strings.Join(m.history, "\n\n")))
	m.viewport.GotoBottom()
}

func (m *model) updateViewport() {
	m.viewport.SetContent(ui.OutputStyle.Render(strings.Join(m.history, "\n\n")))
	m.viewport.GotoBottom()
}

func (m *model) resizeView() {
	if m.width == 0 || m.height == 0 {
		return
	}
	// rows consumed: border top/bot(2) + title+gap(2) + footer border(3) + input(3) + status(1) + suggestions
	overhead := 11 + len(m.suggestions)
	h := m.height - overhead
	if h < 1 {
		h = 1
	}
	m.viewport.Width = m.width - 4
	m.viewport.Height = h
	m.input.SetWidth(m.width - 6)
	m.help.Width = m.width
}

// ── queue / PTY ───────────────────────────────────────────────────────────────

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

	m.history = append(m.history, ui.WarnStyle.Render("▶ running")+"  (ctrl+d → EOF)\n")
	m.runOutputIdx = len(m.history) - 1
	m.updateViewport()

	return readCmd
}

// drainQueue runs the next tool in the queue.
// If the tool needs review it pauses, pushes the diff, and sets m.review.
func (m *model) drainQueue() tea.Cmd {
	for len(m.pendingQueue) > 0 {
		req := m.pendingQueue[0]

		if tools.NeedsReview(req.Tool) && !m.perms.IsGloballyAllowed() {
			// Push the diff/preview inline into chat.
			label := ui.ToolLabelStyle.Render("  "+req.Tool) + "  " + ui.StatusStyle.Render(req.Path)
			preview := m.registry.Preview(req)
			m.pushOutput(label + "\n" + preview)
			m.review = &toolReview{req: req, selected: optAccept}
			return nil
		}

		// Auto-execute (globally allowed or no review needed).
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

// confirmReview executes or skips based on the user's choice.
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

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeView()

	case tea.MouseMsg:
		if msg.Button == tea.MouseButtonLeft &&
			(msg.Action == tea.MouseActionPress || msg.Action == tea.MouseActionRelease) &&
			len(m.suggestions) > 0 {
			sugTop := m.height - 1 - 3 - 1 - len(m.suggestions)
			if idx := msg.Y - sugTop; idx >= 0 && idx < len(m.suggestions) {
				m.selectedSug = idx
				m.input.SetValue(m.suggestions[idx])
				m.input.CursorEnd()
			}
		}

	case spinner.TickMsg:
		if m.loading || m.running {
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
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

	case llm.ResponseMsg:
		m.loading = false
		if msg.Err != nil {
			m.pushOutput("Error: " + msg.Err.Error())
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
			m.pendingQueue = reqs
			return m, m.drainQueue()
		}

		if text != "" {
			m.pushOutput("AI > " + text)
		}

	case tea.KeyMsg:

		// ── PTY passthrough ───────────────────────────────────────────────────
		if m.running && m.ptyMaster != nil {
			m.ptyMaster.Write(apptty.KeyToBytes(msg.String()))
			return m, nil
		}

		// ── Inline review ─────────────────────────────────────────────────────
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

		// ── Global quit ───────────────────────────────────────────────────────
		if key.Matches(msg, ui.Keys.Quit) {
			m.quit = true
			return m, tea.Quit
		}

		// ── Scroll ────────────────────────────────────────────────────────────
		switch msg.String() {
		case "pgup", "ctrl+b":
			m.viewport.HalfViewUp()
			return m, nil
		case "pgdown", "ctrl+f":
			m.viewport.HalfViewDown()
			return m, nil
		}

		// ── Suggestions: up/down cycle; fall through to scroll when none ──────
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

		// ── Submit ────────────────────────────────────────────────────────────
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
			m.suggestions = nil
			m.selectedSug = -1
			m.resizeView()
			return m, tea.Batch(llm.Ask(m.client, userPrompt), m.spinner.Tick)
		}

		// forward remaining keys to textarea
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

	// Status bar text
	status := "Ready"
	if m.loading {
		status = m.spinner.View() + " Thinking..."
	} else if m.running {
		status = m.spinner.View() + " Running..."
	}

	// Viewport
	bodyContent := lipgloss.NewStyle().
		Height(m.viewport.Height).
		MaxHeight(m.viewport.Height).
		Render(m.viewport.View())

	body := ui.BorderStyle.Render(ui.TitleStyle.Render("Themis") + "\n\n" + bodyContent)

	// Suggestions
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

	// Footer: review options OR normal input
	var footer string
	if m.review != nil {
		footer = m.reviewFooter()
	} else {
		footer = ui.BorderStyle.Render(m.input.View())
	}

	helpBar := ui.StatusStyle.Render(status + "   " + m.help.View(ui.Keys))

	parts := []string{body}
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
			s = ui.ReviewSelectedBg.Render(label)
			// tint the selected one
			switch reviewOpt(i) {
			case optAccept:
				s = ui.ReviewSelectedBg.Copy().
					Foreground(lipgloss.Color("2")).Render("❯" + label)
			case optReject:
				s = ui.ReviewSelectedBg.Copy().
					Foreground(lipgloss.Color("1")).Render("❯" + label)
			case optAcceptAll:
				s = ui.ReviewSelectedBg.Copy().
					Foreground(lipgloss.Color("33")).Render("❯" + label)
			}
		}
		opts = append(opts, s)
	}
	hint := ui.ReviewHintStyle.Render("  ←→ navigate   enter confirm   y/n/a shortcut")
	line := strings.Join(opts, " ")
	return ui.BorderStyle.Render(line + "\n" + hint)
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
