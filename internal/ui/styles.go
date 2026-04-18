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

	// Review option styles
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
)
