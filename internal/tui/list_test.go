package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/christopherdobbyn/jiratui/internal/models"
)

func TestRenderRowContainsIssueData(t *testing.T) {
	issue := models.Issue{
		Key:      "PROJ-123",
		Summary:  "Fix the login bug",
		Priority: models.Priority{Name: "High"},
		Status:   models.Status{Name: "In Progress"},
		Assignee: &models.User{DisplayName: "Alice"},
		Updated:  time.Now().Add(-2 * time.Hour),
	}

	l := NewList([]models.Issue{issue}, 120, 40)
	row := l.renderRow(issue, false)

	for _, want := range []string{"PROJ-123", "In Progress", "Alice", "Fix the login bug"} {
		if !strings.Contains(row, want) {
			t.Errorf("row missing %q, got: %q", want, row)
		}
	}
	// Should contain the priority circle
	if !strings.Contains(row, "●") {
		t.Errorf("row missing priority circle, got: %q", row)
	}
}

func TestRenderRowUnassigned(t *testing.T) {
	issue := models.Issue{
		Key:     "PROJ-456",
		Summary: "Unassigned task",
	}

	l := NewList([]models.Issue{issue}, 120, 40)
	row := l.renderRow(issue, false)

	if !strings.Contains(row, "-") {
		t.Errorf("unassigned row missing dash, got: %q", row)
	}
}

func TestRowColor(t *testing.T) {
	now := time.Now()

	noDue := models.Issue{Key: "A-1"}
	if got := rowColor(noDue); got != colorText {
		t.Errorf("no due date: got %v, want colorText", got)
	}

	pastDue := now.Add(-24 * time.Hour)
	overdue := models.Issue{Key: "A-2", DueDate: &pastDue}
	if got := rowColor(overdue); got != colorError {
		t.Errorf("overdue: got %v, want colorError", got)
	}

	soonDue := now.Add(3 * 24 * time.Hour)
	soon := models.Issue{Key: "A-3", DueDate: &soonDue}
	if got := rowColor(soon); got != colorWarning {
		t.Errorf("due soon: got %v, want colorWarning", got)
	}

	farDue := now.Add(14 * 24 * time.Hour)
	far := models.Issue{Key: "A-4", DueDate: &farDue}
	if got := rowColor(far); got != colorText {
		t.Errorf("far due: got %v, want colorText", got)
	}
}

func TestPriorityColor(t *testing.T) {
	tests := []struct {
		priority string
		want     string
	}{
		{"Highest", string(colorError)},
		{"High", "#ff9e64"},
		{"Medium", string(colorWarning)},
		{"Low", string(colorSuccess)},
		{"Lowest", string(colorAccent)},
		{"Unknown", string(colorSubtle)},
	}
	for _, tt := range tests {
		t.Run(tt.priority, func(t *testing.T) {
			got := string(priorityColor(tt.priority))
			if got != tt.want {
				t.Errorf("priorityColor(%q) = %q, want %q", tt.priority, got, tt.want)
			}
		})
	}
}

func TestRelativeTime(t *testing.T) {
	tests := []struct {
		dur  time.Duration
		want string
	}{
		{30 * time.Second, "just now"},
		{5 * time.Minute, "5m ago"},
		{2 * time.Hour, "2h ago"},
		{36 * time.Hour, "1d ago"},
		{14 * 24 * time.Hour, "2w ago"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := relativeTime(time.Now().Add(-tt.dur))
			if !strings.Contains(got, strings.TrimSuffix(tt.want, " ago")) && got != tt.want {
				t.Errorf("relativeTime(-%v) = %q, want %q", tt.dur, got, tt.want)
			}
		})
	}
}
