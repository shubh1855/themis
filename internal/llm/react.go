package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	openai "github.com/sashabaranov/go-openai"
)

// ── ReAct event types (all implement tea.Msg) ────────────────────────────────

// ThinkChunkMsg streams partial thinking text to the UI.
type ThinkChunkMsg struct {
	Agent AgentID
	Chunk string
}

// ToolCallMsg signals the agent is executing a tool.
type ToolCallMsg struct {
	Agent   AgentID
	Tool    string
	Args    map[string]interface{}
	Display string
}

// ToolResultMsg carries the result of a tool execution.
type ToolResultMsg struct {
	Agent  AgentID
	Tool   string
	Result string
}

// ReactAnswerMsg is the agent's final answer.
type ReactAnswerMsg struct {
	Agent AgentID
	Text  string
}

// ReactDelegateMsg signals Zeus is delegating to a sub-agent.
type ReactDelegateMsg struct {
	From    AgentID
	Target  AgentID
	Task    string
	Context string
}

// ReactDoneMsg signals the ReAct channel is closed.
type ReactDoneMsg struct{}

// ReactErrorMsg carries an error from the ReAct loop.
type ReactErrorMsg struct {
	Agent AgentID
	Err   error
}

// ToolExecutor runs a tool and returns text output.
type ToolExecutor func(tool string, args map[string]interface{}) (string, error)

const maxReactSteps = 8

// ── Agent tool access ────────────────────────────────────────────────────────

var agentTools = map[AgentID][]string{
	AgentZeus:       {"delegate", "read_file", "run_cmd", "web_search", "fetch_url", "list_dir"},
	AgentAthena:     {}, // pure planning
	AgentHephaestus: {"create_file", "write_file", "append_file", "read_file", "edit_file", "mkdir", "run_file", "run_cmd", "list_dir", "delete_file", "fetch_url"},
	AgentApollo:     {"web_search", "fetch_url", "run_cmd", "read_file", "npm_search", "pip_search", "cargo_search", "go_search"},
	AgentHermes:     {"create_file", "write_file", "append_file", "read_file", "edit_file", "mkdir", "run_cmd", "web_search", "fetch_url"},
	AgentAres:       {"read_file", "edit_file", "append_file", "create_file", "write_file", "run_file", "run_cmd", "web_search", "fetch_url", "list_dir"},
}

var toolDescs = map[string]string{
	"create_file":  `{"tool":"create_file","path":"<file>","content":"<text>"} — create new file`,
	"write_file":   `{"tool":"write_file","path":"<file>","content":"<text>"} — overwrite file`,
	"append_file":  `{"tool":"append_file","path":"<file>","content":"<text>"} — append to file`,
	"read_file":    `{"tool":"read_file","path":"<file>"} — read file contents`,
	"edit_file":    `{"tool":"edit_file","path":"<file>","old_string":"<old>","new_string":"<new>"} — find & replace in file`,
	"mkdir":        `{"tool":"mkdir","path":"<dir>"} — create directory`,
	"delete_file":  `{"tool":"delete_file","path":"<file>"} — delete a file`,
	"run_file":     `{"tool":"run_file","path":"<file>"} — run a script/program`,
	"run_cmd":      `{"tool":"run_cmd","command":"<shell command>"} — run terminal command`,
	"list_dir":     `{"tool":"list_dir","path":"<dir>"} — list directory contents`,
	"web_search":   `{"tool":"web_search","query":"<search query>"} — search the web`,
	"fetch_url":    `{"tool":"fetch_url","url":"<url>"} — fetch page content`,
	"npm_search":   `{"tool":"npm_search","query":"<pkg>"} — search npm registry`,
	"pip_search":   `{"tool":"pip_search","query":"<pkg>"} — search PyPI registry`,
	"cargo_search": `{"tool":"cargo_search","query":"<crate>"} — search crates.io`,
	"go_search":    `{"tool":"go_search","query":"<module>"} — search Go modules`,
	"delegate":     `{"tool":"delegate","agent":"<Athena|Hephaestus|Apollo|Hermes|Ares>","task":"<instructions>"} — delegate to sub-agent`,
}

func reactSuffix(agent AgentID) string {
	tools := agentTools[agent]
	if len(tools) == 0 {
		return "\n\nYou have no tools. Respond with THOUGHT then ANSWER only.\n\nTHOUGHT: <reasoning>\nANSWER: <your JSON output>"
	}
	var sb strings.Builder
	sb.WriteString("\n\n--- ReAct Protocol ---\nYou MUST follow this exact format:\n\n")
	sb.WriteString("THOUGHT: <your reasoning>\nACTION: <one JSON tool call>\n")
	sb.WriteString("Then WAIT for the system to provide OBSERVATION.\n")
	sb.WriteString("Repeat THOUGHT→ACTION→OBSERVATION as needed.\n")
	sb.WriteString("When done: THOUGHT: <final reasoning>\\nANSWER: <complete response>\n\n")
	sb.WriteString("RULES:\n- One ACTION per turn, then wait\n- ACTION must be valid JSON on one line\n- If no tool needed, skip ACTION and go straight to ANSWER\n\n")
	sb.WriteString("YOUR AVAILABLE TOOLS:\n")
	for _, t := range tools {
		if d, ok := toolDescs[t]; ok {
			sb.WriteString(d + "\n")
		}
	}
	return sb.String()
}

// ── RunReact — the core loop ─────────────────────────────────────────────────

// RunReact runs a ReAct loop in the calling goroutine, sending events to ch.
func RunReact(client *openai.Client, agent AgentID, userPrompt string, extraCtx string, executor ToolExecutor, ch chan<- tea.Msg) {
	defer close(ch)

	sysPrompt := agentPrompts[agent] + reactSuffix(agent)
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: sysPrompt},
	}
	if extraCtx != "" {
		messages = append(messages, openai.ChatCompletionMessage{
			Role: openai.ChatMessageRoleSystem, Content: "Context:\n" + extraCtx,
		})
	}
	messages = append(messages, openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleUser, Content: userPrompt,
	})

	for step := 0; step < maxReactSteps; step++ {
		full, err := streamCall(client, messages, agent, ch)
		if err != nil {
			ch <- ReactErrorMsg{Agent: agent, Err: err}
			return
		}

		thought, action, answer, deleg := parseReact(full)
		_ = thought // already streamed

		if deleg != nil {
			ch <- ReactDelegateMsg{From: agent, Target: deleg.target, Task: deleg.task, Context: deleg.ctx}
			return
		}
		if answer != "" {
			ch <- ReactAnswerMsg{Agent: agent, Text: answer}
			return
		}
		if action != nil {
			ch <- ToolCallMsg{Agent: agent, Tool: action.tool, Args: action.args, Display: action.raw}

			result, execErr := executor(action.tool, action.args)
			if execErr != nil {
				result = "ERROR: " + execErr.Error()
			}
			if len(result) > 4000 {
				result = result[:4000] + "\n...(truncated)"
			}

			ch <- ToolResultMsg{Agent: agent, Tool: action.tool, Result: result}

			messages = append(messages,
				openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: full},
				openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: "OBSERVATION: " + result},
			)
			continue
		}

		// No markers found — treat entire response as answer
		ch <- ReactAnswerMsg{Agent: agent, Text: full}
		return
	}

	ch <- ReactErrorMsg{Agent: agent, Err: fmt.Errorf("reached %d ReAct steps without answer", maxReactSteps)}
}

// ── streaming ────────────────────────────────────────────────────────────────

func streamCall(client *openai.Client, messages []openai.ChatCompletionMessage, agent AgentID, ch chan<- tea.Msg) (string, error) {
	stream, err := client.CreateChatCompletionStream(
		context.Background(),
		openai.ChatCompletionRequest{Model: reactModel, Messages: messages},
	)
	if err != nil {
		// Fallback to non-streaming
		return nonStreamCall(client, messages, agent, ch)
	}
	defer stream.Close()

	var full strings.Builder
	var buf strings.Builder

	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if full.Len() > 0 {
				break // use what we have
			}
			return "", fmt.Errorf("stream: %w", err)
		}
		if len(resp.Choices) == 0 {
			continue
		}

		chunk := resp.Choices[0].Delta.Content
		full.WriteString(chunk)
		buf.WriteString(chunk)

		if buf.Len() > 40 || strings.ContainsAny(chunk, "\n") {
			ch <- ThinkChunkMsg{Agent: agent, Chunk: buf.String()}
			buf.Reset()
		}
	}

	if buf.Len() > 0 {
		ch <- ThinkChunkMsg{Agent: agent, Chunk: buf.String()}
	}
	return full.String(), nil
}

func nonStreamCall(client *openai.Client, messages []openai.ChatCompletionMessage, agent AgentID, ch chan<- tea.Msg) (string, error) {
	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{Model: reactModel, Messages: messages},
	)
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no choices")
	}
	text := resp.Choices[0].Message.Content
	ch <- ThinkChunkMsg{Agent: agent, Chunk: text}
	return text, nil
}

// ── parsing ──────────────────────────────────────────────────────────────────

type parsedAction struct {
	tool string
	args map[string]interface{}
	raw  string
}

type parsedDelegate struct {
	target AgentID
	task   string
	ctx    string
}

func parseReact(text string) (thought string, action *parsedAction, answer string, deleg *parsedDelegate) {
	lines := strings.Split(text, "\n")

	var thoughtBuf, answerBuf strings.Builder
	inAnswer := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "ANSWER:") || strings.HasPrefix(trimmed, "ANSWER :") {
			inAnswer = true
			rest := strings.TrimPrefix(trimmed, "ANSWER:")
			rest = strings.TrimPrefix(rest, "ANSWER :")
			answerBuf.WriteString(strings.TrimSpace(rest) + "\n")
			continue
		}
		if inAnswer {
			answerBuf.WriteString(line + "\n")
			continue
		}

		if strings.HasPrefix(trimmed, "THOUGHT:") || strings.HasPrefix(trimmed, "THOUGHT :") {
			rest := strings.TrimPrefix(trimmed, "THOUGHT:")
			rest = strings.TrimPrefix(rest, "THOUGHT :")
			thoughtBuf.WriteString(strings.TrimSpace(rest) + "\n")
			continue
		}

		if strings.HasPrefix(trimmed, "ACTION:") || strings.HasPrefix(trimmed, "ACTION :") {
			rest := strings.TrimPrefix(trimmed, "ACTION:")
			rest = strings.TrimPrefix(rest, "ACTION :")
			rest = strings.TrimSpace(rest)
			act := parseToolJSON(rest)
			if act != nil {
				// Check for delegation
				if act.tool == "delegate" {
					agentName, _ := act.args["agent"].(string)
					task, _ := act.args["task"].(string)
					ctx, _ := act.args["context"].(string)
					target := resolveAgentName(agentName)
					if target != "" && task != "" {
						deleg = &parsedDelegate{target: target, task: task, ctx: ctx}
						return
					}
				}
				action = act
			}
			continue
		}

		// Non-prefixed lines go to thought
		if trimmed != "" {
			thoughtBuf.WriteString(trimmed + "\n")
		}
	}

	thought = strings.TrimSpace(thoughtBuf.String())
	answer = strings.TrimSpace(answerBuf.String())
	return
}

func parseToolJSON(s string) *parsedAction {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "{") {
		return nil
	}
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return nil
	}
	tool, _ := raw["tool"].(string)
	if tool == "" {
		return nil
	}
	delete(raw, "tool")
	return &parsedAction{tool: tool, args: raw, raw: s}
}

// ── Public helpers ───────────────────────────────────────────────────────────

// StartReact creates a channel, starts the loop, and returns the first Cmd.
func StartReact(client *openai.Client, agent AgentID, prompt string, ctx string, executor ToolExecutor) (chan tea.Msg, tea.Cmd) {
	ch := make(chan tea.Msg, 32)
	go RunReact(client, agent, prompt, ctx, executor, ch)
	return ch, WaitReact(ch)
}

// WaitReact returns a Cmd that reads the next event from a ReAct channel.
func WaitReact(ch <-chan tea.Msg) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return ReactDoneMsg{}
		}
		return msg
	}
}
