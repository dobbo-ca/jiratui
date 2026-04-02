package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/christopherdobbyn/jiratui/internal/models"
)

func TestIssueToRow(t *testing.T) {
	issue := models.Issue{
		Key:      "PROJ-123",
		Summary:  "Fix the login bug",
		Priority: models.Priority{Name: "High"},
		Status:   models.Status{Name: "In Progress"},
		Assignee: &models.User{DisplayName: "Alice"},
		Updated:  time.Now().Add(-2 * time.Hour),
	}

	row := issueToRow(issue)

	if row[colKey] != "PROJ-123" {
		t.Errorf("key = %q, want %q", row[colKey], "PROJ-123")
	}
	if row[colPriority] != "🔼" {
		t.Errorf("priority = %q, want %q", row[colPriority], "🔼")
	}
	if row[colAssignee] != "Alice" {
		t.Errorf("assignee = %q, want %q", row[colAssignee], "Alice")
	}
	if row[colSummary] != "Fix the login bug" {
		t.Errorf("summary = %q, want %q", row[colSummary], "Fix the login bug")
	}
}

func TestIssueToRowUnassigned(t *testing.T) {
	issue := models.Issue{
		Key:     "PROJ-456",
		Summary: "Unassigned task",
	}

	row := issueToRow(issue)

	if row[colAssignee] != "-" {
		t.Errorf("assignee = %q, want %q", row[colAssignee], "-")
	}
}

func TestFilterIssues(t *testing.T) {
	issues := []models.Issue{
		{Key: "PROJ-1", Summary: "Fix login bug", Status: models.Status{Name: "To Do"}},
		{Key: "PROJ-2", Summary: "Add dashboard", Status: models.Status{Name: "In Progress"}},
		{Key: "PROJ-3", Summary: "Update API docs", Status: models.Status{Name: "Done"}},
	}

	tests := []struct {
		query string
		want  int
	}{
		{"", 3},
		{"login", 1},
		{"PROJ-2", 1},
		{"proj", 3},
		{"dashboard", 1},
		{"nonexistent", 0},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got := filterIssues(issues, tt.query)
			if len(got) != tt.want {
				t.Errorf("filterIssues(%q) returned %d issues, want %d", tt.query, len(got), tt.want)
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
