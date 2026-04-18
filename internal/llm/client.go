// Package llm provides the LLM client and agent orchestration layer.
package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	openai "github.com/sashabaranov/go-openai"

	agents "github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/prompt/agents"
)

// ── Agent definitions ────────────────────────────────────────────────────────

// AgentID identifies which agent is speaking.
type AgentID string

const (
	AgentZeus       AgentID = "Zeus"
	AgentAthena     AgentID = "Athena"
	AgentHephaestus AgentID = "Hephaestus"
	AgentApollo     AgentID = "Apollo"
	AgentHermes     AgentID = "Hermes"
	AgentAres       AgentID = "Ares"
)

// agentPrompts maps each agent to its system prompt.
var agentPrompts = map[AgentID]string{
	AgentZeus:       agents.ZeusPrompt,
	AgentAthena:     agents.AthenaPrompt,
	AgentHephaestus: agents.HephaestusPrompt,
	AgentApollo:     agents.ApolloPrompt,
	AgentHermes:     agents.HermesPrompt,
	AgentAres:       agents.AresPrompt,
}

// AgentEmoji returns a visual badge for the agent.
func AgentEmoji(id AgentID) string {
	switch id {
	case AgentZeus:
		return "⚡"
	case AgentAthena:
		return "🦉"
	case AgentHephaestus:
		return "🔨"
	case AgentApollo:
		return "☀️"
	case AgentHermes:
		return "🪽"
	case AgentAres:
		return "🛡️"
	default:
		return "🤖"
	}
}

// ── Messages ─────────────────────────────────────────────────────────────────

// ResponseMsg is the tea.Msg delivered when an LLM call completes.
type ResponseMsg struct {
	Text  string
	Err   error
	Agent AgentID
}

// DelegationMsg is sent when Zeus delegates to a sub-agent.
type DelegationMsg struct {
	Target  AgentID
	Task    string
	Context string // additional context from Zeus
}

// AgentStartMsg signals the UI that an agent has started working.
type AgentStartMsg struct {
	Agent AgentID
	Task  string
}

// AgentDoneMsg signals the UI that an agent finished.
type AgentDoneMsg struct {
	Agent AgentID
}

// ── Client ───────────────────────────────────────────────────────────────────

// NewClient creates a configured OpenAI-compatible client pointing at the LiteLLM proxy.
func NewClient(apiKey string) *openai.Client {
	cfg := openai.DefaultConfig(apiKey)
	cfg.BaseURL = "https://litellm-proxy-93ef.onrender.com/v1"
	return openai.NewClientWithConfig(cfg)
}

// ── Agent calls ──────────────────────────────────────────────────────────────

// callAgent sends a prompt to the LLM using a specific agent's system prompt.
func callAgent(client *openai.Client, agent AgentID, userPrompt string, extraContext string) (string, error) {
	systemPrompt, ok := agentPrompts[agent]
	if !ok {
		return "", fmt.Errorf("unknown agent: %s", agent)
	}

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
	}

	if extraContext != "" {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: "Context from Zeus:\n" + extraContext,
		})
	}

	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: userPrompt,
	})

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:    "google/gemma-4-31B-it",
			Messages: messages,
		},
	)
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}
	return resp.Choices[0].Message.Content, nil
}

// ── Orchestration ────────────────────────────────────────────────────────────

// AskWithOrchestration sends the user prompt to Zeus first, then delegates
// to sub-agents as Zeus directs. Returns tea.Cmds for async UI updates.
func AskWithOrchestration(client *openai.Client, userPrompt string) tea.Cmd {
	return func() tea.Msg {
		// Step 1: Ask Zeus to analyze and decide
		zeusPrompt := fmt.Sprintf(`Analyze this request and decide how to handle it.

If this is a SIMPLE request (quick answer, single file, small edit), handle it directly yourself using tool JSON lines or a plain text answer.

If this is a COMPLEX request needing specialized work, respond with a JSON delegation:
{"delegate": "<agent_name>", "task": "<specific instructions for that agent>", "context": "<relevant context>"}

Available agents: Athena (planning), Hephaestus (coding), Apollo (research), Hermes (docs/communication), Ares (testing)

User request: %s`, userPrompt)

		zeusReply, err := callAgent(client, AgentZeus, zeusPrompt, "")
		if err != nil {
			return ResponseMsg{Err: err, Agent: AgentZeus}
		}

		zeusReply = strings.TrimSpace(zeusReply)

		// Check if Zeus delegated
		delegation := parseDelegation(zeusReply)
		if delegation != nil {
			return DelegationMsg{
				Target:  delegation.Target,
				Task:    delegation.Task,
				Context: delegation.Context,
			}
		}

		// Zeus handled it directly
		return ResponseMsg{Text: zeusReply, Agent: AgentZeus}
	}
}

// AskAgent sends a task to a specific sub-agent and returns the result.
func AskAgent(client *openai.Client, agent AgentID, task string, zeusContext string) tea.Cmd {
	return func() tea.Msg {
		reply, err := callAgent(client, agent, task, zeusContext)
		if err != nil {
			return ResponseMsg{Err: err, Agent: agent}
		}
		return ResponseMsg{Text: strings.TrimSpace(reply), Agent: agent}
	}
}

// Ask is the legacy single-agent call (backward compatible — uses Hephaestus).
func Ask(client *openai.Client, userPrompt string) tea.Cmd {
	return AskWithOrchestration(client, userPrompt)
}

// ── Delegation parsing ───────────────────────────────────────────────────────

type delegationPayload struct {
	Target  AgentID `json:"-"`
	RawName string  `json:"delegate"`
	Task    string  `json:"task"`
	Context string  `json:"context"`
}

func parseDelegation(text string) *delegationPayload {
	// Try to find a delegation JSON in the response
	text = strings.TrimSpace(text)

	// Try full text as JSON
	if d := tryParseDelegation(text); d != nil {
		return d
	}

	// Try each line
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if d := tryParseDelegation(line); d != nil {
			return d
		}
	}

	return nil
}

func tryParseDelegation(s string) *delegationPayload {
	if !strings.Contains(s, "delegate") {
		return nil
	}

	var d delegationPayload
	if err := json.Unmarshal([]byte(s), &d); err != nil {
		return nil
	}
	if d.RawName == "" || d.Task == "" {
		return nil
	}

	d.Target = resolveAgentName(d.RawName)
	if d.Target == "" {
		return nil
	}

	return &d
}

func resolveAgentName(name string) AgentID {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "athena":
		return AgentAthena
	case "hephaestus":
		return AgentHephaestus
	case "apollo":
		return AgentApollo
	case "hermes":
		return AgentHermes
	case "ares":
		return AgentAres
	case "zeus":
		return AgentZeus
	default:
		return ""
	}
}
