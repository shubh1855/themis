package ui

import (
	"fmt"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
)


type TaskStatus int

const (
	TaskPending TaskStatus = iota
	TaskRunning
	TaskDone
	TaskFailed
)

func (s TaskStatus) Icon() string {
	switch s {
	case TaskPending:
		return "○"
	case TaskRunning:
		return "◉"
	case TaskDone:
		return "✓"
	case TaskFailed:
		return "✗"
	default:
		return "?"
	}
}

func (s TaskStatus) Style() lipgloss.Style {
	switch s {
	case TaskPending:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	case TaskRunning:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	case TaskDone:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("40"))
	case TaskFailed:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	default:
		return lipgloss.NewStyle()
	}
}


type TaskNode struct {
	ID       string
	Agent    string
	Label    string
	Status   TaskStatus
	Children []*TaskNode
	ToolCalls []string
}


type TaskGraph struct {
	mu    sync.Mutex
	Root  *TaskNode
	nodes map[string]*TaskNode
	seq   int
}

func NewTaskGraph() *TaskGraph {
	return &TaskGraph{nodes: make(map[string]*TaskNode)}
}

func (g *TaskGraph) nextID() string {
	g.seq++
	return fmt.Sprintf("T%d", g.seq)
}

func (g *TaskGraph) AddRoot(agent, label string) string {
	g.mu.Lock()
	defer g.mu.Unlock()
	id := g.nextID()
	g.Root = &TaskNode{ID: id, Agent: agent, Label: label, Status: TaskRunning}
	g.nodes[id] = g.Root
	return id
}

func (g *TaskGraph) AddChild(parentID, agent, label string) string {
	g.mu.Lock()
	defer g.mu.Unlock()
	id := g.nextID()
	node := &TaskNode{ID: id, Agent: agent, Label: label, Status: TaskPending}
	g.nodes[id] = node
	if p, ok := g.nodes[parentID]; ok {
		p.Children = append(p.Children, node)
	} else if g.Root != nil {
		g.Root.Children = append(g.Root.Children, node)
	}
	return id
}

func (g *TaskGraph) SetStatus(id string, status TaskStatus) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if n, ok := g.nodes[id]; ok {
		n.Status = status
	}
}

func (g *TaskGraph) AddToolCall(id, summary string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if n, ok := g.nodes[id]; ok {
		n.ToolCalls = append(n.ToolCalls, summary)
	}
}

func (g *TaskGraph) FindByAgent(agent string) string {
	g.mu.Lock()
	defer g.mu.Unlock()
	var latest string
	for id, n := range g.nodes {
		if n.Agent == agent && n.Status == TaskRunning {
			latest = id
		}
	}
	return latest
}

func (g *TaskGraph) Stats() (int, int, int, int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	total, done, failed, running := 0, 0, 0, 0
	for _, n := range g.nodes {
		total++
		switch n.Status {
		case TaskDone:
			done++
		case TaskFailed:
			failed++
		case TaskRunning:
			running++
		}
	}
	return total, done, failed, running
}


var (
	panelBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1)

	panelTitle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("141")).
			Bold(true)

	connectorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	toolCallStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")).
			Italic(true)
)

func (g *TaskGraph) Render(width, height int) string {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.Root == nil {
		return panelBorder.Copy().Width(width - 2).Height(height - 2).Render(
			panelTitle.Render("📋 Tasks") + "\n\n" +
				lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("No active tasks"))
	}

	var sb strings.Builder
	sb.WriteString(panelTitle.Render("📋 Task Graph") + "\n")

	total, done, failed, running := 0, 0, 0, 0
	for _, n := range g.nodes {
		total++
		switch n.Status {
		case TaskDone:
			done++
		case TaskFailed:
			failed++
		case TaskRunning:
			running++
		}
	}
	statsLine := fmt.Sprintf("%d/%d done", done, total)
	if running > 0 {
		statsLine += fmt.Sprintf(" · %d active", running)
	}
	if failed > 0 {
		statsLine += fmt.Sprintf(" · %d failed", failed)
	}
	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(statsLine) + "\n")
	sb.WriteString(connectorStyle.Render("─────────") + "\n")

	g.renderNode(&sb, g.Root, "", true, width-4, height-5)

	content := sb.String()

	return panelBorder.Copy().Width(width - 2).Height(height - 2).Render(content)
}

func (g *TaskGraph) renderNode(sb *strings.Builder, node *TaskNode, prefix string, isLast bool, maxW, maxLines int) {
	if maxLines <= 0 {
		return
	}

	var connector string
	if prefix == "" {
		connector = ""
	} else if isLast {
		connector = prefix + "└─"
	} else {
		connector = prefix + "├─"
	}

	agentEmoji := map[string]string{
		"Zeus": "⚡", "Athena": "🦉", "Hephaestus": "🔨",
		"Apollo": "☀️", "Hermes": "🪽", "Ares": "⚔️",
	}
	emoji := agentEmoji[node.Agent]
	if emoji == "" {
		emoji = "●"
	}

	icon := node.Status.Icon()
	stl := node.Status.Style()

	label := node.Label
	usable := maxW - len(prefix) - 6
	if usable < 10 {
		usable = 10
	}
	if len(label) > usable {
		label = label[:usable-1] + "…"
	}

	line := connectorStyle.Render(connector) + stl.Render(icon+" ") + emoji + " " + stl.Render(label)
	sb.WriteString(line + "\n")
	remaining := maxLines - 1

	calls := node.ToolCalls
	if len(calls) > 2 {
		calls = calls[len(calls)-2:]
	}
	childPrefix := prefix
	if prefix != "" {
		if isLast {
			childPrefix = prefix + "   "
		} else {
			childPrefix = prefix + "│  "
		}
	} else {
		childPrefix = "  "
	}

	for _, tc := range calls {
		if remaining <= 0 {
			break
		}
		tcLabel := tc
		if len(tcLabel) > usable-2 {
			tcLabel = tcLabel[:usable-3] + "…"
		}
		sb.WriteString(connectorStyle.Render(childPrefix+"  ") + toolCallStyle.Render("⚙ "+tcLabel) + "\n")
		remaining--
	}

	for i, child := range node.Children {
		if remaining <= 0 {
			sb.WriteString(connectorStyle.Render(childPrefix) + "  …\n")
			break
		}
		g.renderNode(sb, child, childPrefix, i == len(node.Children)-1, maxW, remaining)
		remaining--
	}
}
