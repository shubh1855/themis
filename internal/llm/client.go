package llm

import (
	openai "github.com/sashabaranov/go-openai"

	agents "github.com/syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey/internal/prompt/agents"
)

// ── Agent definitions ────────────────────────────────────────────────────────

type AgentID string

const (
	AgentZeus       AgentID = "Zeus"
	AgentAthena     AgentID = "Athena"
	AgentHephaestus AgentID = "Hephaestus"
	AgentApollo     AgentID = "Apollo"
	AgentHermes     AgentID = "Hermes"
	AgentAres       AgentID = "Ares"
)

const reactModel = "google/gemma-4-31B-it"

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

// NewClient creates a configured OpenAI-compatible client.
func NewClient(apiKey string) *openai.Client {
	cfg := openai.DefaultConfig(apiKey)
	cfg.BaseURL = "https://litellm-proxy-93ef.onrender.com/v1"
	return openai.NewClientWithConfig(cfg)
}

func resolveAgentName(name string) AgentID {
	switch name {
	case "athena", "Athena":
		return AgentAthena
	case "hephaestus", "Hephaestus":
		return AgentHephaestus
	case "apollo", "Apollo":
		return AgentApollo
	case "hermes", "Hermes":
		return AgentHermes
	case "ares", "Ares":
		return AgentAres
	case "zeus", "Zeus":
		return AgentZeus
	default:
		return ""
	}
}
