package llm

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	openai "github.com/sashabaranov/go-openai"

	"github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/prompt"
)

// ResponseMsg is the tea.Msg delivered when the LLM call completes.
type ResponseMsg struct {
	Text string
	Err  error
}

// NewClient creates a configured OpenAI-compatible client pointing at the LiteLLM proxy.
func NewClient(apiKey string) *openai.Client {
	cfg := openai.DefaultConfig(apiKey)
	cfg.BaseURL = "https://litellm-proxy-93ef.onrender.com/v1"
	return openai.NewClientWithConfig(cfg)
}

// Ask returns a tea.Cmd that calls the LLM and delivers a ResponseMsg.
func Ask(client *openai.Client, userPrompt string) tea.Cmd {
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
			return ResponseMsg{Err: err}
		}
		return ResponseMsg{Text: resp.Choices[0].Message.Content}
	}
}
