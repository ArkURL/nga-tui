package ui

import "github.com/charmbracelet/lipgloss"

// 全局共享样式。
var (
	accentColor   = lipgloss.Color("#ff8c00") // NGA 橙
	dimColor      = lipgloss.Color("#6b6b6b")
	errorColor    = lipgloss.Color("#ff5555")
	okColor       = lipgloss.Color("#55ff55")
	selectedColor = lipgloss.Color("#2b2b2b")

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(accentColor).
			Background(lipgloss.Color("#3a3a3a")).
			Padding(0, 1)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#ffffff"))

	groupHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(dimColor)

	categoryStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(accentColor)

	accentStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(accentColor)

	selectedStyle = lipgloss.NewStyle().
			Background(selectedColor).
			Foreground(accentColor).
			Bold(true)

	dimStyle = lipgloss.NewStyle().Foreground(dimColor)

	errorStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)

	okStyle = lipgloss.NewStyle().
		Foreground(okColor)

	statusBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#1a1a1a")).
			Foreground(lipgloss.Color("#aaaaaa")).
			Padding(0, 1)

	helpStyle = lipgloss.NewStyle().
			Foreground(dimColor)

	footerStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#3a3a3a")).
			Foreground(lipgloss.Color("#dddddd")).
			Padding(0, 1)

	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(dimColor)
)
