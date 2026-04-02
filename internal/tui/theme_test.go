package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestThemeColorsAreDefined(t *testing.T) {
	colors := map[string]lipgloss.Color{
		"Background": colorBackground,
		"HeaderBg":   colorHeaderBg,
		"Border":     colorBorder,
		"Text":       colorText,
		"Accent":     colorAccent,
		"Success":    colorSuccess,
		"Warning":    colorWarning,
		"Error":      colorError,
		"Info":       colorInfo,
		"Subtle":     colorSubtle,
		"Purple":     colorPurple,
	}

	for name, c := range colors {
		if string(c) == "" {
			t.Errorf("color %s is empty", name)
		}
	}
}

func TestPriorityStyle(t *testing.T) {
	tests := []struct {
		priority string
		wantIcon string
	}{
		{"Highest", "⏫"},
		{"High", "🔼"},
		{"Medium", "▶️"},
		{"Low", "🔽"},
		{"Lowest", "⏬"},
		{"Unknown", "•"},
	}
	for _, tt := range tests {
		t.Run(tt.priority, func(t *testing.T) {
			got := PriorityIcon(tt.priority)
			if got != tt.wantIcon {
				t.Errorf("PriorityIcon(%q) = %q, want %q", tt.priority, got, tt.wantIcon)
			}
		})
	}
}

func TestStatusStyle(t *testing.T) {
	tests := []string{"To Do", "In Progress", "In Review", "Done", "Unknown"}
	for _, status := range tests {
		t.Run(status, func(t *testing.T) {
			got := StyledStatus(status)
			if got == "" {
				t.Errorf("StyledStatus(%q) returned empty string", status)
			}
		})
	}
}
