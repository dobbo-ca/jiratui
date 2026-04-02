package tui

import "github.com/charmbracelet/lipgloss"

// Tokyo Night dark theme colors.
var (
	colorBackground = lipgloss.Color("#1a1b26")
	colorHeaderBg   = lipgloss.Color("#16161e")
	colorBorder     = lipgloss.Color("#292e42")
	colorText       = lipgloss.Color("#c9d1d9")
	colorAccent     = lipgloss.Color("#7aa2f7") // blue
	colorSuccess    = lipgloss.Color("#9ece6a") // green
	colorWarning    = lipgloss.Color("#e0af68") // amber
	colorError      = lipgloss.Color("#f7768e") // red
	colorInfo       = lipgloss.Color("#7dcfff") // cyan
	colorSubtle     = lipgloss.Color("#565f89") // gray
	colorPurple     = lipgloss.Color("#bb9af7")
)

// StyledStatus returns a color-coded status string.
func StyledStatus(status string) string {
	var color lipgloss.Color
	switch status {
	case "Done":
		color = colorSuccess
	case "In Progress":
		color = colorWarning
	case "In Review":
		color = colorPurple
	case "To Do":
		color = colorSubtle
	default:
		color = colorText
	}
	return lipgloss.NewStyle().Foreground(color).Render(status)
}
