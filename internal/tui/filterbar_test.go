package tui

import (
	"strings"
	"testing"
)

func TestFilterBarBuildJQL_Defaults(t *testing.T) {
	fb := NewFilterBar(120)
	fb.SetDefaultAssignee("abc123", "Chris")

	jql := fb.BuildJQL("PROJ", "updated DESC")
	if !strings.Contains(jql, `assignee = "abc123"`) {
		t.Errorf("expected default assignee clause, got: %s", jql)
	}
	if !strings.Contains(jql, "statusCategory != Done") {
		t.Errorf("expected statusCategory != Done when no status filter, got: %s", jql)
	}
	if !strings.Contains(jql, `project = "PROJ"`) {
		t.Errorf("expected project clause, got: %s", jql)
	}
	if !strings.Contains(jql, "ORDER BY updated DESC") {
		t.Errorf("expected ORDER BY, got: %s", jql)
	}
}

func TestFilterBarBuildJQL_WithStatusFilter(t *testing.T) {
	fb := NewFilterBar(120)
	fb.statusDrop.Toggle("10001")

	fb.statusDrop.SetItems([]DropdownItem{
		{ID: "10001", Label: "In Progress"},
		{ID: "10002", Label: "To Do"},
	})
	fb.statusDrop.Toggle("10001")

	jql := fb.BuildJQL("", "updated DESC")
	if !strings.Contains(jql, `status = "In Progress"`) && !strings.Contains(jql, `status IN ("In Progress")`) {
		t.Errorf("expected status filter, got: %s", jql)
	}
	if strings.Contains(jql, "statusCategory") {
		t.Errorf("should not have statusCategory when status filter active, got: %s", jql)
	}
}

func TestFilterBarBuildJQL_NoProject(t *testing.T) {
	fb := NewFilterBar(120)
	jql := fb.BuildJQL("", "updated DESC")
	if strings.Contains(jql, "project") {
		t.Errorf("should not have project clause when empty, got: %s", jql)
	}
}

func TestFilterBarBuildJQL_WithSearch(t *testing.T) {
	fb := NewFilterBar(120)
	fb.search.SetValue("login bug")

	jql := fb.BuildJQL("", "updated DESC")
	if !strings.Contains(jql, `text ~ "login bug"`) {
		t.Errorf("expected text search clause, got: %s", jql)
	}
}

func TestFilterBarBuildJQL_WithLabels(t *testing.T) {
	fb := NewFilterBar(120)
	fb.labelsDrop.SetItems([]DropdownItem{
		{ID: "frontend", Label: "frontend"},
		{ID: "backend", Label: "backend"},
	})
	fb.labelsDrop.Toggle("frontend")
	fb.labelsDrop.Toggle("backend")

	jql := fb.BuildJQL("", "updated DESC")
	if !strings.Contains(jql, `labels IN ("frontend", "backend")`) {
		t.Errorf("expected labels clause, got: %s", jql)
	}
}

func TestFilterBarHeight(t *testing.T) {
	fb := NewFilterBar(120)

	if fb.Height() != 1 {
		t.Errorf("collapsed height: got %d, want 1", fb.Height())
	}

	fb.expanded = true
	h := fb.Height()
	if h < 7 {
		t.Errorf("expanded height: got %d, want >= 7", h)
	}
}

func TestFilterBarCollapsedSummary(t *testing.T) {
	fb := NewFilterBar(120)
	view := fb.View()
	if !strings.Contains(view, "Filters") {
		t.Errorf("collapsed view should contain 'Filters', got: %s", view)
	}
}

func TestFilterBarClearAll(t *testing.T) {
	fb := NewFilterBar(120)
	fb.SetDefaultAssignee("abc123", "Chris")
	fb.statusDrop.SetItems([]DropdownItem{{ID: "1", Label: "To Do"}})
	fb.statusDrop.Toggle("1")
	fb.search.SetValue("test")

	fb.ClearAll()

	if fb.statusDrop.HasSelection() {
		t.Error("status should be cleared")
	}
	if fb.search.Value() != "" {
		t.Error("search should be cleared")
	}
	if fb.HasActiveFilters() {
		t.Error("should have no active filters after clear")
	}
}
