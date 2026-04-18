package ui

import "github.com/charmbracelet/lipgloss"

var (
	TitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	StatusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	OutputStyle = lipgloss.NewStyle().PaddingLeft(1).PaddingRight(1)
	WarnStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)

	BorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1)

	ModalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("214")).
			Padding(1, 4)

	SuggestionStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("243")).
			PaddingLeft(2)

	SelectedSuggestionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("205")).
				Bold(true).
				PaddingLeft(2)

	SpinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	ReviewAcceptStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("2")).
				Bold(true).
				PaddingLeft(1).PaddingRight(2)

	ReviewRejectStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("1")).
				Bold(true).
				PaddingLeft(1).PaddingRight(2)

	ReviewNeutralStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("33")).
				Bold(true).
				PaddingLeft(1).PaddingRight(2)

	ReviewDimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			PaddingLeft(1).PaddingRight(2)

	ReviewSelectedBg = lipgloss.NewStyle().
				Reverse(true).
				PaddingLeft(1).PaddingRight(1)

	ReviewHintStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Italic(true)

	ToolLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true)

	AgentZeusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("226")).Bold(true)

	AgentAthenaStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("75")).Bold(true)

	AgentHephaestusStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("208")).Bold(true)

	AgentApolloStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("220")).Bold(true)

	AgentHermesStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("158")).Bold(true)

	AgentAresStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).Bold(true)

	AgentDelegateStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("141")).
				Italic(true)

	AgentBadgeStyle = lipgloss.NewStyle().
			Bold(true).
			PaddingLeft(1).PaddingRight(1)
)

func AgentStyle(name string) lipgloss.Style {
	switch name {
	case "Zeus":
		return AgentZeusStyle
	case "Athena":
		return AgentAthenaStyle
	case "Hephaestus":
		return AgentHephaestusStyle
	case "Apollo":
		return AgentApolloStyle
	case "Hermes":
		return AgentHermesStyle
	case "Ares":
		return AgentAresStyle
	default:
		return StatusStyle
	}
}
