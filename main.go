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
		client:   client,
		registry: reg,
		perms:    perms,
		viewport: vp,
		input:    ta,
		history:  []string{},
	}
}

func askLLM(client *openai.Client, prompt string) tea.Cmd {
	return func() tea.Msg {
		resp, err := client.CreateChatCompletion(
			context.Background(),
			openai.ChatCompletionRequest{
				Model: "google/gemma-4-31B-it",
				Messages: []openai.ChatCompletionMessage{
					{
						Role: openai.ChatMessageRoleSystem,
						Content: `
You are a coding CLI assistant.

If user asks to create/edit/read/run files, ONLY return JSON:

{"tool":"create_file","path":"main.go","content":"package main"}
{"tool":"write_file","path":"x.txt","content":"hello"}
{"tool":"append_file","path":"log.txt","content":"line"}
{"tool":"read_file","path":"main.go"}
{"tool":"mkdir","path":"internal/api"}
{"tool":"run_file","path":"main.py"}
{"tool":"run_file","path":"main.py","content":"arg1 arg2"}

Supported run_file extensions: .py .js .ts .sh .rb .go .c .cpp .java .json(package.json)
For C/C++ the file is compiled then executed automatically.

No markdown. No explanation.
Otherwise answer normally.
`,
					},
					{
						Role:    openai.ChatMessageRoleUser,
						Content: prompt,
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

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		headerHeight := 2
		inputHeight := 5
		statusHeight := 1
		padding := 4

		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - headerHeight - inputHeight - statusHeight - padding

		m.input.SetWidth(msg.Width - 6)

	case tea.KeyMsg:

		// permission modal
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

		case "enter":
			if m.loading {
				return m, nil
			}

			prompt := strings.TrimSpace(m.input.Value())
			if prompt == "" {
				return m, nil
			}

			m.pushOutput("You > " + prompt)
			m.input.SetValue("")
			m.loading = true

			return m, askLLM(m.client, prompt)
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

	footer := borderStyle.Render(m.input.View())

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
	)

	if _, err := p.Run(); err != nil {
		fmt.Println("error:", err)
	}
}
