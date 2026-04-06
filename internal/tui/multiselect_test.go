package tui

import (
	"strings"
	"testing"
)

func TestMultiSelectValueDisplay(t *testing.T) {
	items := []DropdownItem{
		{ID: "1", Label: "Bug"},
		{ID: "2", Label: "Task"},
		{ID: "3", Label: "Story"},
	}

	ms := NewMultiSelect("Type", items, 30)

	if ms.ValueText() != "All" {
		t.Errorf("empty selection: got %q, want %q", ms.ValueText(), "All")
	}

	ms.Toggle("1")
	if ms.ValueText() != "Bug" {
		t.Errorf("one selection: got %q, want %q", ms.ValueText(), "Bug")
	}

	ms.Toggle("2")
	if ms.ValueText() != "Bug, Task" {
		t.Errorf("two selections: got %q, want %q", ms.ValueText(), "Bug, Task")
	}

	ms.Toggle("1")
	if ms.ValueText() != "Task" {
		t.Errorf("after deselect: got %q, want %q", ms.ValueText(), "Task")
	}
}

func TestMultiSelectSelectedIDs(t *testing.T) {
	items := []DropdownItem{
		{ID: "1", Label: "Bug"},
		{ID: "2", Label: "Task"},
	}
	ms := NewMultiSelect("Type", items, 30)
	ms.Toggle("1")
	ms.Toggle("2")

	ids := ms.SelectedIDs()
	if len(ids) != 2 {
		t.Fatalf("expected 2 IDs, got %d", len(ids))
	}
}

func TestMultiSelectClear(t *testing.T) {
	items := []DropdownItem{
		{ID: "1", Label: "Bug"},
	}
	ms := NewMultiSelect("Type", items, 30)
	ms.Toggle("1")
	ms.Clear()

	if ms.ValueText() != "All" {
		t.Errorf("after clear: got %q, want %q", ms.ValueText(), "All")
	}
}

func TestMultiSelectOverlay(t *testing.T) {
	items := []DropdownItem{
		{ID: "1", Label: "Bug"},
		{ID: "2", Label: "Task"},
	}
	ms := NewMultiSelect("Type", items, 30)
	ms.Toggle("1")
	ms.Open()

	lines := ms.RenderOverlay()
	if len(lines) == 0 {
		t.Fatal("overlay should have lines when open")
	}

	found := false
	for _, line := range lines {
		if strings.Contains(line, "[✓]") && strings.Contains(line, "Bug") {
			found = true
		}
	}
	if !found {
		t.Error("overlay missing [✓] Bug")
	}

	foundEmpty := false
	for _, line := range lines {
		if strings.Contains(line, "[ ]") && strings.Contains(line, "Task") {
			foundEmpty = true
		}
	}
	if !foundEmpty {
		t.Error("overlay missing [ ] Task")
	}
}
