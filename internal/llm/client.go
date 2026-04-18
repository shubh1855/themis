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
		return ""
	case AgentAthena:
		return ""
	case AgentHephaestus:
		return ""
	case AgentApollo:
		return ""
	case AgentHermes:
		return ""
	case AgentAres:
		return ""
	default:
		return ""
	}
}

// ── Messages ─────────────────────────────────────────────────────────────────

// ResponseMsg is the tea.Msg delivered when an LLM call completes.
type ResponseMsg struct {
	Text    string
	Err     error
	Agent   AgentID
	History []openai.ChatCompletionMessage
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
func AskAgent(client *openai.Client, agent AgentID, task string, zeusContext string, history []openai.ChatCompletionMessage) tea.Cmd {
	return func() tea.Msg {
		if len(history) == 0 {
			systemPrompt, ok := agentPrompts[agent]
			if !ok {
				return ResponseMsg{Err: fmt.Errorf("unknown agent"), Agent: agent}
			}
			history = append(history, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleSystem, Content: systemPrompt})
			if zeusContext != "" {
				history = append(history, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleSystem, Content: "Context from Zeus:\n" + zeusContext})
			}
		}

		history = append(history, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: task})

		resp, err := client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
			Model:    "google/gemma-4-31B-it",
			Messages: history,
		})
		if err != nil {
			return ResponseMsg{Err: err, Agent: agent}
		}
		reply := resp.Choices[0].Message.Content
		history = append(history, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: reply})

		return ResponseMsg{Text: strings.TrimSpace(reply), Agent: agent, History: history}
	}
}

// Ask is the legacy single-agent call (backward compatible — uses Hephaestus).
func Ask(client *openai.Client, userPrompt string) tea.Cmd {
	return AskWithOrchestration(client, userPrompt)
}

// ── Athena plan dispatch ─────────────────────────────────────────────────────

// AthenaPlan is the structured JSON plan Athena returns.
type AthenaPlan struct {
	Goal     string       `json:"goal"`
	Tasks    []AthenaTask `json:"tasks"`
	Sequence []string     `json:"sequence"`
}

// AthenaTask is one task in Athena's plan.
type AthenaTask struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Owner     string   `json:"owner"`
	Type      string   `json:"type"`
	Targets   []string `json:"targets"`
	DependsOn []string `json:"depends_on"`
}

// ParseAthenaPlan parses Athena's JSON output into a structured plan.
// Returns nil if the text is not a valid plan (no tasks field).
func ParseAthenaPlan(text string) *AthenaPlan {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start == -1 || end == -1 || end < start {
		return nil
	}
	cleanJSON := text[start : end+1]

	var plan AthenaPlan
	if err := json.Unmarshal([]byte(cleanJSON), &plan); err != nil {
		return nil
	}
	if len(plan.Tasks) == 0 {
		return nil
	}
	return &plan
}

// DispatchPlanTasks returns ALL agent delegations from Athena's plan in sequence order,
// skipping Zeus and Athena (orchestration roles). Tasks for the same agent are batched.
func DispatchPlanTasks(plan *AthenaPlan) []DelegationMsg {
	type batch struct {
		id    AgentID
		tasks []string
	}
	var ordered []batch
	seen := map[AgentID]int{}

	for _, seqID := range plan.Sequence {
		for _, t := range plan.Tasks {
			if t.ID != seqID {
				continue
			}
			agentID := resolveAgentName(t.Owner)
			if agentID == "" || agentID == AgentZeus || agentID == AgentAthena {
				continue
			}
			entry := fmt.Sprintf("[%s] %s", t.ID, t.Title)
			if len(t.Targets) > 0 {
				entry += fmt.Sprintf(" (files: %s)", strings.Join(t.Targets, ", "))
			}
			if idx, exists := seen[agentID]; exists {
				ordered[idx].tasks = append(ordered[idx].tasks, entry)
			} else {
				seen[agentID] = len(ordered)
				ordered = append(ordered, batch{id: agentID, tasks: []string{entry}})
			}
		}
	}

	var delegations []DelegationMsg
	for _, b := range ordered {
		delegations = append(delegations, DelegationMsg{
			Target:  b.id,
			Task:    strings.Join(b.tasks, "\n"),
			Context: "Plan goal: " + plan.Goal,
		})
	}
	return delegations
}

// AskAgentWithResults sends tool execution results back to an agent so it can
// continue its task with the actual output from the tools it invoked.
func AskAgentWithResults(client *openai.Client, agent AgentID, results []string, history []openai.ChatCompletionMessage) tea.Cmd {
	return func() tea.Msg {
		prompt := "Tool execution results from your previous requests:\n\n" +
			strings.Join(results, "\n") +
			"\n\nBased on these results, continue your task (using more tools if needed) or provide a final answer."

		history = append(history, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: prompt})

		resp, err := client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
			Model:    "google/gemma-4-31B-it",
			Messages: history,
		})
		if err != nil {
			return ResponseMsg{Err: err, Agent: agent}
		}
		reply := resp.Choices[0].Message.Content
		history = append(history, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: reply})

		return ResponseMsg{Text: strings.TrimSpace(reply), Agent: agent, History: history}
	}
}

// ResolveAgentName exports the private resolveAgentName for use in main.go's drainQueue.
func ResolveAgentName(name string) AgentID {
	return resolveAgentName(name)
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

	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start == -1 || end == -1 || end < start {
		return nil
	}
	cleanJSON := s[start : end+1]

	var d delegationPayload
	if err := json.Unmarshal([]byte(cleanJSON), &d); err != nil {
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
