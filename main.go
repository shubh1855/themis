package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	openai "github.com/sashabaranov/go-openai"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/prompt"
	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/tools"
)

type responseMsg struct {
	text string
	err  error
}

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
	quit    bool

	pending *tools.ToolRequest
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

func looksLikeJSON(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")
}

func (m *model) pushOutput(text string) {
	m.history = append(m.history, text)
	content := strings.Join(m.history, "\n\n")
	m.viewport.SetContent(outputStyle.Render(content))
	m.viewport.GotoBottom()
}

func (m *model) resizeView() {
	if m.width == 0 || m.height == 0 {
		return
	}

	headerHeight := 2
	inputHeight := 5
	statusHeight := 1
	padding := 4

	suggestionsHeight := len(m.suggestions)

	m.viewport.Width = m.width - 4
	m.viewport.Height = m.height - headerHeight - inputHeight - statusHeight - padding - suggestionsHeight

	m.input.SetWidth(m.width - 6)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeView()

	case tea.MouseMsg:
		if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft {
			if len(m.suggestions) > 0 {
				statusLine := m.height - 1
				footerTop := statusLine - 5
				suggestionsTop := footerTop - len(m.suggestions)

				if msg.Y >= suggestionsTop && msg.Y < footerTop {
					m.selectedSug = msg.Y - suggestionsTop
					if m.selectedSug >= 0 && m.selectedSug < len(m.suggestions) {
						m.input.SetValue(m.suggestions[m.selectedSug])
						m.input.CursorEnd()
					}
				}
			}
		}

	case tea.KeyMsg:
		if m.pending != nil {
			switch msg.String() {
			case "y":
				res := m.registry.Execute(*m.pending)
				m.pushOutput("[tool] " + res.Output)
				m.pending = nil

			case "a":
				m.perms.Resolve(tools.AllowAlways)
				res := m.registry.Execute(*m.pending)
				m.pushOutput("[tool] " + res.Output)
				m.pending = nil

			case "n", "esc":
				m.pushOutput("[tool] permission denied")
				m.pending = nil
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

		if looksLikeJSON(text) {
			var req tools.ToolRequest

			if err := json.Unmarshal([]byte(text), &req); err != nil {
				m.pushOutput(text)
				return m, nil
			}

			if m.perms.NeedsPrompt() {
				m.pending = &req
				return m, nil
			}

			res := m.registry.Execute(req)
			m.pushOutput("[tool] " + res.Output)
			return m, nil
		}

		m.pushOutput("AI > " + text)
	}

	return m, nil
}

func (m model) View() string {
	if m.quit {
		return ""
	}

	header := titleStyle.Render("Themis")

	status := "Ready"
	if m.loading {
		status = "Thinking..."
	}

	if m.pending != nil {
		modal := warnStyle.Render("Permission Required") + "\n\n" +
			fmt.Sprintf("Tool: %s\nPath: %s\n\n[y] allow once   [a] always   [n] deny",
				m.pending.Tool,
				m.pending.Path,
			)

		return borderStyle.Render(header+"\n\n"+modal) + "\n"
	}

	body := borderStyle.Render(
		header + "\n\n" +
			m.viewport.View(),
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
		sugView = lipgloss.JoinVertical(lipgloss.Left, lines...)
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
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Println("error:", err)
	}
}
