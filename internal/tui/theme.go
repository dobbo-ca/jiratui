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
	colorOrange     = lipgloss.Color("#ff9e64")
	colorSelection  = lipgloss.Color("#292e42")
)

// statusColor returns the color for a Jira status name.
func statusColor(status string) lipgloss.Color {
	switch status {
	// Done category
	case "Done", "Closed", "Resolved", "Released", "Completed":
		return colorSuccess
	// In Progress category
	case "In Progress", "In Development", "Building", "Implementing":
		return colorWarning
	// Review / waiting category
	case "In Review", "In QA", "Code Review", "Awaiting Review", "Review",
		"QA", "Testing", "Validation", "Ready for Review":
		return colorPurple
	// Blocked / on hold
	case "Blocked", "On Hold", "Impediment", "Waiting", "Pending",
		"Awaiting Approval", "Waiting for Support", "Waiting for Customer":
		return colorError
	// To Do / backlog
	case "To Do", "Open", "New", "Backlog", "Selected for Development",
		"Ready", "Ready for Development", "Reopened":
		return colorSubtle
	// Deployed / shipped
	case "Deployed", "Shipped", "Live", "In Production":
		return colorInfo
	default:
		return colorText
	}
}

// StyledStatus returns a color-coded status string.
func StyledStatus(status string) string {
	return lipgloss.NewStyle().Foreground(statusColor(status)).Render(status)
}

// priorityColor returns the color for a Jira priority name.
func priorityColor(priority string) lipgloss.Color {
	switch priority {
	// Critical / Blocker
	case "Highest", "Blocker", "Critical", "P0", "Urgent":
		return colorError
	// High
	case "High", "Major", "P1":
		return colorOrange
	// Medium
	case "Medium", "Normal", "P2", "Default":
		return colorWarning
	// Low
	case "Low", "Minor", "P3":
		return colorSuccess
	// Lowest / Trivial
	case "Lowest", "Trivial", "P4", "P5":
		return colorAccent
	default:
		return colorSubtle
	}
}

// StyledPriority returns a color-coded priority string with a bullet indicator.
func StyledPriority(priority string) string {
	color := priorityColor(priority)
	return lipgloss.NewStyle().Foreground(color).Render("● " + priority)
}
