package tui

import (
	"testing"
)

func TestFilterBarMouseClickHandling(t *testing.T) {
	items := []DropdownItem{
		{ID: "1", Label: "Bug"},
		{ID: "2", Label: "Task"},
	}

	fb := NewFilterBar(100)
	fb.Expand()
	fb.typeDrop.Open()
	fb.typeDrop.SetItems(items)

	overlay := fb.typeDrop.RenderOverlay()
	if len(overlay) == 0 {
		t.Fatal("overlay should have lines when dropdown is open")
	}

	if !fb.typeDrop.HandleClick(0) {
		t.Error("HandleClick should return true for a valid item click")
	}

	selected := fb.typeDrop.SelectedIDs()
	if len(selected) != 1 || selected[0] != "1" {
		t.Error("clicking should toggle the item selection")
	}
}

func TestFilterBarHandleFieldClick(t *testing.T) {
	fb := NewFilterBar(100)
	fb.SetWidth(100)
	fb.Expand()

	// Click on the header (y=0) should collapse
	fb.HandleFieldClick(10, 0)
	if fb.IsExpanded() {
		t.Error("clicking header should collapse the filter bar")
	}

	// Re-expand and click on fields
	fb.Expand()
	fpr := fb.fieldsPerRow()
	fieldW := fb.fieldRowWidth(fpr)

	// Type (index 0, row 0, col 0)
	fb.HandleFieldClick(5, 2)
	if !fb.typeDrop.IsOpen() {
		t.Error("clicking on Type field should open it")
	}

	// Status (index 1)
	fb.closeAll()
	fb.HandleFieldClick(fieldW+2, 2)
	if !fb.statusDrop.IsOpen() {
		t.Error("clicking on Status field should open it")
	}

	// Priority (index 2)
	fb.closeAll()
	fb.HandleFieldClick(fieldW*2+2, 2)
	if !fb.priorityDrop.IsOpen() {
		t.Error("clicking on Priority field should open it")
	}
}

func TestFilterBarHandleFieldClickRow2(t *testing.T) {
	fb := NewFilterBar(100)
	fb.SetWidth(100)
	fb.Expand()

	fpr := fb.fieldsPerRow()
	fieldW := fb.fieldRowWidth(fpr)

	// Fields in the second row depend on fieldsPerRow
	// With 100 width and minFieldW=16: fpr = 100/16 = 6
	// Row 0 (y=1-3): fields 0-5 (Type..Labels)
	// Row 1 (y=4-6): fields 6-10 (Until..Save)

	// Created Until is at fieldUntil=6, row 1, col 0
	row1Y := 4 + 1 // middle of second field row
	fb.HandleFieldClick(2, row1Y)
	expectedIdx := fpr // first field in row 1
	if expectedIdx == fieldFrom {
		if !fb.createdFrom.IsOpen() {
			t.Error("clicking first field of row 1 should open Created From")
		}
	} else if expectedIdx == fieldUntil {
		if !fb.createdUntil.IsOpen() {
			t.Error("clicking first field of row 1 should open Created Until")
		}
	}

	// Sort By
	fb.closeAll()
	sortRow, sortCol := fb.fieldPosition(fieldSortBy)
	sortY := 1 + sortRow*3 + 1
	sortX := sortCol*fieldW + 2
	fb.HandleFieldClick(sortX, sortY)
	if !fb.sortDrop.IsOpen() {
		t.Errorf("clicking on Sort By field (row=%d col=%d) should open it", sortRow, sortCol)
	}
}

func TestFilterBarDynamicRows(t *testing.T) {
	// Wide terminal: fewer rows
	fb := NewFilterBar(200)
	fb.SetWidth(200)
	fb.Expand()
	wideRows := fb.numRows()
	wideHeight := fb.Height()

	// Narrow terminal: more rows
	fb2 := NewFilterBar(80)
	fb2.SetWidth(80)
	fb2.Expand()
	narrowRows := fb2.numRows()
	narrowHeight := fb2.Height()

	if narrowRows <= wideRows {
		t.Errorf("narrow terminal (%d rows) should have more rows than wide (%d rows)", narrowRows, wideRows)
	}
	if narrowHeight <= wideHeight {
		t.Errorf("narrow height (%d) should be taller than wide (%d)", narrowHeight, wideHeight)
	}

	// Height should be 1 + numRows * 3
	if wideHeight != 1+wideRows*3 {
		t.Errorf("wide height should be %d, got %d", 1+wideRows*3, wideHeight)
	}
}

func TestFilterBarOverlayPositions(t *testing.T) {
	fb := NewFilterBar(120)
	fb.SetWidth(120)
	fb.Expand()
	fb.typeDrop.SetItems([]DropdownItem{{ID: "1", Label: "Bug"}})
	fb.typeDrop.Open()

	lines, startLine, startCol := fb.OverlayLines()
	if lines == nil {
		t.Fatal("overlay lines should not be nil when dropdown is open")
	}
	// Type is at index 0, row 0 → startLine should be 1 + 3 = 4
	if startLine != 4 {
		t.Errorf("typeDrop overlay startLine should be 4, got %d", startLine)
	}
	if startCol != 0 {
		t.Errorf("typeDrop overlay startCol should be 0, got %d", startCol)
	}

	// Sort overlay
	fb.closeAll()
	fb.sortDrop.OpenDropdown()
	_, sortStartLine, sortStartCol := fb.OverlayLines()
	sortRow, sortCol := fb.fieldPosition(fieldSortBy)
	expectedLine := 1 + (sortRow+1)*3
	expectedCol := sortCol * fb.fieldRowWidth(fb.fieldsPerRow())
	if sortStartLine != expectedLine {
		t.Errorf("sortDrop overlay startLine should be %d, got %d", expectedLine, sortStartLine)
	}
	if sortStartCol != expectedCol {
		t.Errorf("sortDrop overlay startCol should be %d, got %d", expectedCol, sortStartCol)
	}
}

func TestFilterBarPriorityJQL(t *testing.T) {
	fb := NewFilterBar(100)
	fb.priorityDrop.SetItems([]DropdownItem{
		{ID: "1", Label: "High"},
		{ID: "2", Label: "Medium"},
	})
	fb.priorityDrop.Toggle("1")

	jql := fb.BuildJQL("TEST", "")
	if !contains(jql, `priority = "High"`) {
		t.Errorf("JQL should contain priority clause, got: %s", jql)
	}
}

func TestFilterBarHeaderSeparation(t *testing.T) {
	fb := NewFilterBar(100)

	collapsed := fb.renderCollapsed()
	if !contains(collapsed, "Filters") {
		t.Error("collapsed view should contain 'Filters'")
	}

	fb.Expand()
	expanded := fb.renderExpanded()
	if !contains(expanded, "Filters") {
		t.Error("expanded view should contain 'Filters'")
	}
}

func contains(s, substr string) bool {
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
