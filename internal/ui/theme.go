package ui

import "github.com/charmbracelet/lipgloss"

// Theme holds the color palette for a named visual theme.
type Theme struct {
	Name      string
	Primary   lipgloss.Color // accents, badges, titles
	Secondary lipgloss.Color // borders, subtext
	Accent    lipgloss.Color // highlights, warnings
	Success   lipgloss.Color // success states
	Danger    lipgloss.Color // errors, failures
	Dim       lipgloss.Color // dimmed / muted text
}

// ThemeOrder defines the display order in the settings view.
var ThemeOrder = []string{"default", "ocean", "hacker", "solarized"}

var Themes = map[string]Theme{
	"default": {
		Name:      "Default",
		Primary:   lipgloss.Color("205"), // hot pink
		Secondary: lipgloss.Color("141"), // lavender
		Accent:    lipgloss.Color("214"), // amber
		Success:   lipgloss.Color("40"),
		Danger:    lipgloss.Color("196"),
		Dim:       lipgloss.Color("241"),
	},
	"ocean": {
		Name:      "Ocean",
		Primary:   lipgloss.Color("39"),  // sky blue
		Secondary: lipgloss.Color("67"),  // steel blue
		Accent:    lipgloss.Color("44"),  // teal
		Success:   lipgloss.Color("35"),
		Danger:    lipgloss.Color("160"),
		Dim:       lipgloss.Color("240"),
	},
	"hacker": {
		Name:      "Hacker",
		Primary:   lipgloss.Color("40"),  // bright green
		Secondary: lipgloss.Color("28"),  // dark green
		Accent:    lipgloss.Color("154"), // lime
		Success:   lipgloss.Color("46"),
		Danger:    lipgloss.Color("196"),
		Dim:       lipgloss.Color("238"),
	},
	"solarized": {
		Name:      "Solarized",
		Primary:   lipgloss.Color("33"),  // blue
		Secondary: lipgloss.Color("136"), // yellow
		Accent:    lipgloss.Color("166"), // orange
		Success:   lipgloss.Color("64"),
		Danger:    lipgloss.Color("160"),
		Dim:       lipgloss.Color("240"),
	},
}

// GetTheme returns the named theme, falling back to default.
func GetTheme(name string) Theme {
	if t, ok := Themes[name]; ok {
		return t
	}
	return Themes["default"]
}
