# Filter Bar Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a collapsible filter bar above the issue list that enables server-side JQL filtering by issue type, status, assignee, labels, date range, and free-text search.

**Architecture:** (1) Add `GetProjectStatusesAndTypes` API method, (2) create `MultiSelect` component (extends Dropdown pattern for multi-selection), (3) create `FilterBar` component that holds multi-selects, date pickers, and a search input, (4) integrate into `App` by replacing the old inline `/` filter with the new filter bar, (5) wire up `filterChangedMsg` to re-fetch issues with JQL built from active filters.

**Tech Stack:** Go, bubbletea, lipgloss. Jira REST API v3 `/rest/api/3/project/{key}/statuses` endpoint.

---

### Task 1: Add GetProjectStatusesAndTypes API method

**Files:**
- Modify: `internal/jira/types.go` (add response types)
- Modify: `internal/jira/client.go` (add method)

- [ ] **Step 1: Add Jira API response types**

In `internal/jira/types.go`, add after the `jiraIssueLinkType` struct (line 146):

```go
// jiraProjectStatusesResponse is one element of the array returned by
// GET /rest/api/3/project/{key}/statuses — one per issue type.
type jiraProjectStatusEntry struct {
	ID       string       `json:"id"`
	Name     string       `json:"name"` // issue type name
	Statuses []jiraStatus `json:"statuses"`
}
```

- [ ] **Step 2: Add the client method**

In `internal/jira/client.go`, add after `DeleteAttachment` (line 877):

```go
// GetProjectStatusesAndTypes fetches statuses grouped by issue type from
// GET /rest/api/3/project/{key}/statuses. Returns deduplicated statuses
// and issue types.
func (c *Client) GetProjectStatusesAndTypes(projectKey string) ([]models.Status, []models.IssueType, error) {
	path := "/rest/api/3/project/" + projectKey + "/statuses"
	data, err := c.get(path)
	if err != nil {
		return nil, nil, fmt.Errorf("fetching project statuses: %w", err)
	}

	entries, err := decodeJSON[[]jiraProjectStatusEntry](data)
	if err != nil {
		return nil, nil, fmt.Errorf("decoding project statuses: %w", err)
	}

	seenStatus := make(map[string]bool)
	var statuses []models.Status
	issueTypes := make([]models.IssueType, 0, len(entries))

	for _, entry := range entries {
		issueTypes = append(issueTypes, models.IssueType{
			ID:   entry.ID,
			Name: entry.Name,
		})
		for _, s := range entry.Statuses {
			if !seenStatus[s.ID] {
				seenStatus[s.ID] = true
				statuses = append(statuses, models.Status{
					ID:   s.ID,
					Name: s.Name,
				})
			}
		}
	}

	return statuses, issueTypes, nil
}
```

- [ ] **Step 3: Verify it compiles**

Run: `cd /Users/christopherdobbyn/work/dobbo-ca/jiratui && go build ./...`
Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add internal/jira/types.go internal/jira/client.go
git commit -m "feat(jira): add GetProjectStatusesAndTypes API method"
```

---

### Task 2: Create MultiSelect component

**Files:**
- Create: `internal/tui/multiselect.go`
- Create: `internal/tui/multiselect_test.go`

- [ ] **Step 1: Write tests for MultiSelect**

Create `internal/tui/multiselect_test.go`:

```go
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

	// No selection => "All"
	if ms.ValueText() != "All" {
		t.Errorf("empty selection: got %q, want %q", ms.ValueText(), "All")
	}

	// Select one
	ms.Toggle("1")
	if ms.ValueText() != "Bug" {
		t.Errorf("one selection: got %q, want %q", ms.ValueText(), "Bug")
	}

	// Select two
	ms.Toggle("2")
	if ms.ValueText() != "Bug, Task" {
		t.Errorf("two selections: got %q, want %q", ms.ValueText(), "Bug, Task")
	}

	// Deselect first
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

	// Check that selected item has checkmark
	found := false
	for _, line := range lines {
		if strings.Contains(line, "[✓]") && strings.Contains(line, "Bug") {
			found = true
		}
	}
	if !found {
		t.Error("overlay missing [✓] Bug")
	}

	// Check unselected item
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/christopherdobbyn/work/dobbo-ca/jiratui && go test ./internal/tui/ -run TestMultiSelect -v`
Expected: compilation errors (MultiSelect not defined)

- [ ] **Step 3: Implement MultiSelect**

Create `internal/tui/multiselect.go`:

```go
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// MultiSelect is a multi-selection dropdown component.
// It stays open on Enter (toggles items) and only closes on Esc.
type MultiSelect struct {
	label      string
	items      []DropdownItem
	selected   map[string]bool // set of selected IDs
	cursor     int
	open       bool
	scrollOff  int
	maxVisible int
	width      int
}

// NewMultiSelect creates a new multi-select dropdown.
func NewMultiSelect(label string, items []DropdownItem, width int) MultiSelect {
	return MultiSelect{
		label:      label,
		items:      items,
		selected:   make(map[string]bool),
		maxVisible: 8,
		width:      width,
	}
}

// Toggle adds or removes an item ID from the selection.
func (ms *MultiSelect) Toggle(id string) {
	if ms.selected[id] {
		delete(ms.selected, id)
	} else {
		ms.selected[id] = true
	}
}

// ToggleCursor toggles the item at the current cursor position.
func (ms *MultiSelect) ToggleCursor() {
	if ms.cursor >= 0 && ms.cursor < len(ms.items) {
		ms.Toggle(ms.items[ms.cursor].ID)
	}
}

// Clear removes all selections.
func (ms *MultiSelect) Clear() {
	ms.selected = make(map[string]bool)
}

// SetSelected replaces the selection set.
func (ms *MultiSelect) SetSelected(ids map[string]bool) {
	ms.selected = ids
}

// SelectedIDs returns the IDs of all selected items.
func (ms MultiSelect) SelectedIDs() []string {
	var ids []string
	// Maintain item order
	for _, item := range ms.items {
		if ms.selected[item.ID] {
			ids = append(ids, item.ID)
		}
	}
	return ids
}

// SelectedLabels returns the labels of all selected items in item order.
func (ms MultiSelect) SelectedLabels() []string {
	var labels []string
	for _, item := range ms.items {
		if ms.selected[item.ID] {
			labels = append(labels, item.Label)
		}
	}
	return labels
}

// HasSelection returns true if any items are selected.
func (ms MultiSelect) HasSelection() bool {
	return len(ms.selected) > 0
}

// ValueText returns the display text: "All" or comma-separated selected labels.
func (ms MultiSelect) ValueText() string {
	labels := ms.SelectedLabels()
	if len(labels) == 0 {
		return "All"
	}
	return strings.Join(labels, ", ")
}

// ValueTextTruncated returns the display text truncated to fit a given width,
// with "+N" suffix if needed.
func (ms MultiSelect) ValueTextTruncated(maxW int) string {
	labels := ms.SelectedLabels()
	if len(labels) == 0 {
		return "All"
	}
	result := labels[0]
	for i := 1; i < len(labels); i++ {
		candidate := result + ", " + labels[i]
		remaining := len(labels) - i - 1
		suffix := ""
		if remaining > 0 {
			suffix = fmt.Sprintf(" +%d", remaining)
		}
		if len(candidate)+len(suffix) > maxW {
			return result + fmt.Sprintf(" +%d", len(labels)-i)
		}
		result = candidate
	}
	return result
}

func (ms MultiSelect) IsOpen() bool {
	return ms.open
}

func (ms *MultiSelect) Open() {
	ms.open = true
	ms.cursor = 0
	ms.scrollOff = 0
}

func (ms *MultiSelect) Close() {
	ms.open = false
}

func (ms *MultiSelect) SetItems(items []DropdownItem) {
	ms.items = items
	// Remove selections that are no longer in the item list
	valid := make(map[string]bool)
	for _, item := range items {
		valid[item.ID] = true
	}
	for id := range ms.selected {
		if !valid[id] {
			delete(ms.selected, id)
		}
	}
}

func (ms *MultiSelect) SetWidth(w int) {
	ms.width = w
}

func (ms MultiSelect) Update(msg tea.Msg) (MultiSelect, tea.Cmd) {
	if !ms.open {
		return ms, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			ms.Close()
			return ms, nil
		case "enter", " ":
			ms.ToggleCursor()
			return ms, nil
		case "down", "j":
			if ms.cursor < len(ms.items)-1 {
				ms.cursor++
				if ms.cursor >= ms.scrollOff+ms.maxVisible {
					ms.scrollOff = ms.cursor - ms.maxVisible + 1
				}
			}
			return ms, nil
		case "up", "k":
			if ms.cursor > 0 {
				ms.cursor--
				if ms.cursor < ms.scrollOff {
					ms.scrollOff = ms.cursor
				}
			}
			return ms, nil
		}
	}
	return ms, nil
}

// View renders the closed field box.
func (ms MultiSelect) View() string {
	width := ms.width
	if width < 8 {
		width = 8
	}
	innerW := width - 2
	valW := innerW - 2

	lbl := lipgloss.NewStyle().Foreground(colorAccent)
	bdr := lipgloss.NewStyle().Foreground(colorBorder)

	labelText := " " + ms.label + " ▾ "
	if ms.open {
		bdr = bdr.Foreground(colorAccent)
	}

	dashes := innerW - lipgloss.Width(labelText) - 1
	if dashes < 0 {
		dashes = 0
	}

	top := bdr.Render("╭─") + lbl.Render(labelText) + bdr.Render(strings.Repeat("─", dashes)+"╮")

	displayVal := ms.ValueTextTruncated(valW)
	valStyle := lipgloss.NewStyle().Foreground(colorText)
	if !ms.HasSelection() {
		valStyle = valStyle.Foreground(colorSubtle)
	}
	content := valStyle.Render(truncStr(displayVal, valW))
	visW := lipgloss.Width(content)
	pad := valW - visW
	if pad < 0 {
		pad = 0
	}
	mid := bdr.Render("│") + " " + content + strings.Repeat(" ", pad) + " " + bdr.Render("│")

	var bot string
	if ms.open {
		bot = bdr.Render("├" + strings.Repeat("─", innerW) + "┤")
	} else {
		bot = bdr.Render("╰" + strings.Repeat("─", innerW) + "╯")
	}

	return top + "\n" + mid + "\n" + bot
}

// RenderOverlay returns the dropdown overlay lines (below the field box).
func (ms MultiSelect) RenderOverlay() []string {
	if !ms.open {
		return nil
	}

	width := ms.width
	if width < 8 {
		width = 8
	}
	innerW := width - 2
	valW := innerW - 2

	bdr := lipgloss.NewStyle().Foreground(colorAccent)
	selectedStyle := lipgloss.NewStyle().Foreground(colorText).Background(colorSelection).Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(colorText)
	checkStyle := lipgloss.NewStyle().Foreground(colorSuccess)
	uncheckStyle := lipgloss.NewStyle().Foreground(colorSubtle)

	var lines []string

	if len(ms.items) == 0 {
		noMatch := lipgloss.NewStyle().Foreground(colorSubtle).Render("No items")
		pad := valW - lipgloss.Width(noMatch)
		if pad < 0 {
			pad = 0
		}
		lines = append(lines, bdr.Render("│")+" "+noMatch+strings.Repeat(" ", pad)+" "+bdr.Render("│"))
	} else {
		start := ms.scrollOff
		end := start + ms.maxVisible
		if end > len(ms.items) {
			end = len(ms.items)
		}

		for i := start; i < end; i++ {
			item := ms.items[i]
			var check string
			if ms.selected[item.ID] {
				check = checkStyle.Render("[✓]")
			} else {
				check = uncheckStyle.Render("[ ]")
			}

			labelMaxW := valW - 4 // "[✓] " prefix
			label := truncStr(item.Label, labelMaxW)

			style := normalStyle
			if i == ms.cursor {
				style = selectedStyle
			}

			content := check + " " + style.Render(label)
			visW := lipgloss.Width(content)
			pad := valW - visW
			if pad < 0 {
				pad = 0
			}
			lines = append(lines, bdr.Render("│")+" "+content+strings.Repeat(" ", pad)+" "+bdr.Render("│"))
		}

		if len(ms.items) > ms.maxVisible {
			indicator := lipgloss.NewStyle().Foreground(colorSubtle).Render(
				fmt.Sprintf(" %d-%d of %d", start+1, end, len(ms.items)),
			)
			pad := valW - lipgloss.Width(indicator)
			if pad < 0 {
				pad = 0
			}
			lines = append(lines, bdr.Render("│")+" "+indicator+strings.Repeat(" ", pad)+" "+bdr.Render("│"))
		}
	}

	lines = append(lines, bdr.Render("╰"+strings.Repeat("─", innerW)+"╯"))

	return lines
}

// HandleClick processes a mouse click on the overlay at the given line index.
// Returns true if an item was toggled.
func (ms *MultiSelect) HandleClick(overlayLine int) bool {
	if !ms.open || overlayLine < 0 {
		return false
	}

	start := ms.scrollOff
	end := start + ms.maxVisible
	if end > len(ms.items) {
		end = len(ms.items)
	}

	itemIdx := start + overlayLine
	if itemIdx >= start && itemIdx < end {
		ms.cursor = itemIdx
		ms.ToggleCursor()
		return true
	}

	return false
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/christopherdobbyn/work/dobbo-ca/jiratui && go test ./internal/tui/ -run TestMultiSelect -v`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tui/multiselect.go internal/tui/multiselect_test.go
git commit -m "feat(tui): add MultiSelect component with toggle checkmarks"
```

---

### Task 3: Create FilterBar component

**Files:**
- Create: `internal/tui/filterbar.go`
- Create: `internal/tui/filterbar_test.go`

- [ ] **Step 1: Write tests for FilterBar**

Create `internal/tui/filterbar_test.go`:

```go
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

	// Populate items so SelectedLabels works
	fb.statusDrop.SetItems([]DropdownItem{
		{ID: "10001", Label: "In Progress"},
		{ID: "10002", Label: "To Do"},
	})
	fb.statusDrop.Toggle("10001")

	jql := fb.BuildJQL("", "updated DESC")
	if !strings.Contains(jql, `status = "In Progress"`) && !strings.Contains(jql, `status IN ("In Progress")`) {
		t.Errorf("expected status filter, got: %s", jql)
	}
	// Should NOT have statusCategory != Done when explicit status filter is set
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/christopherdobbyn/work/dobbo-ca/jiratui && go test ./internal/tui/ -run TestFilterBar -v`
Expected: compilation errors (FilterBar not defined)

- [ ] **Step 3: Implement FilterBar**

Create `internal/tui/filterbar.go`:

```go
package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// filterChangedMsg is sent when any filter value changes and issues should be re-fetched.
type filterChangedMsg struct{}

// filterSearchTick is the debounce tick for the search field.
type filterSearchTick struct {
	seq int
}

// FilterBar is a collapsible filter bar at the top of the list pane.
type FilterBar struct {
	expanded     bool
	typeDrop     MultiSelect
	statusDrop   MultiSelect
	assigneeDrop MultiSelect
	labelsDrop   MultiSelect
	createdFrom  DatePicker
	createdUntil DatePicker
	search       textinput.Model
	searchSeq    int  // debounce counter
	searchActive bool // true when search input is focused
	width        int

	// Default assignee to restore on clear
	defaultAssigneeID   string
	defaultAssigneeName string
}

// NewFilterBar creates a new filter bar with default state.
func NewFilterBar(width int) FilterBar {
	ti := textinput.New()
	ti.Placeholder = "Filter by key or summary..."
	ti.CharLimit = 200
	ti.Prompt = ""

	fieldW := fieldWidth(width, 4)

	return FilterBar{
		typeDrop:     NewMultiSelect("Issue Type", nil, fieldW),
		statusDrop:   NewMultiSelect("Status", nil, fieldW),
		assigneeDrop: NewMultiSelect("Assignee", nil, fieldW),
		labelsDrop:   NewMultiSelect("Labels", nil, fieldW),
		createdFrom:  NewDatePicker("Created From", nil, 18),
		createdUntil: NewDatePicker("Created Until", nil, 18),
		search:       ti,
		width:        width,
	}
}

func fieldWidth(totalW, count int) int {
	w := (totalW - (count - 1)) / count // subtract gaps
	if w < 16 {
		w = 16
	}
	return w
}

// SetDefaultAssignee sets the initial assignee filter (current user).
func (fb *FilterBar) SetDefaultAssignee(accountID, displayName string) {
	fb.defaultAssigneeID = accountID
	fb.defaultAssigneeName = displayName
	fb.assigneeDrop.selected[accountID] = true
}

// SetWidth updates the filter bar width and recalculates field widths.
func (fb *FilterBar) SetWidth(w int) {
	fb.width = w
	fieldW := fieldWidth(w, 4)
	fb.typeDrop.SetWidth(fieldW)
	fb.statusDrop.SetWidth(fieldW)
	fb.assigneeDrop.SetWidth(fieldW)
	fb.labelsDrop.SetWidth(fieldW)
	// Search width is calculated dynamically in View
}

// SetStatusItems populates the status multi-select.
func (fb *FilterBar) SetStatusItems(items []DropdownItem) {
	fb.statusDrop.SetItems(items)
}

// SetTypeItems populates the issue type multi-select.
func (fb *FilterBar) SetTypeItems(items []DropdownItem) {
	fb.typeDrop.SetItems(items)
}

// SetAssigneeItems populates the assignee multi-select.
func (fb *FilterBar) SetAssigneeItems(items []DropdownItem) {
	fb.assigneeDrop.SetItems(items)
}

// SetLabelItems populates the labels multi-select.
func (fb *FilterBar) SetLabelItems(items []DropdownItem) {
	fb.labelsDrop.SetItems(items)
}

func (fb FilterBar) IsExpanded() bool {
	return fb.expanded
}

func (fb *FilterBar) Expand() {
	fb.expanded = true
}

func (fb *FilterBar) Collapse() {
	fb.expanded = false
	fb.closeAll()
}

func (fb *FilterBar) Toggle() {
	if fb.expanded {
		fb.Collapse()
	} else {
		fb.Expand()
	}
}

// HasActiveFilters returns true if any filter differs from defaults.
func (fb FilterBar) HasActiveFilters() bool {
	if fb.typeDrop.HasSelection() {
		return true
	}
	if fb.statusDrop.HasSelection() {
		return true
	}
	// Assignee: active if it differs from default (or if default is empty and something is selected)
	if fb.defaultAssigneeID != "" {
		// Active if selection differs from just the default
		ids := fb.assigneeDrop.SelectedIDs()
		if len(ids) != 1 || ids[0] != fb.defaultAssigneeID {
			if len(ids) > 0 {
				return true
			}
		}
	} else if fb.assigneeDrop.HasSelection() {
		return true
	}
	if fb.labelsDrop.HasSelection() {
		return true
	}
	if fb.createdFrom.Value() != nil {
		return true
	}
	if fb.createdUntil.Value() != nil {
		return true
	}
	if fb.search.Value() != "" {
		return true
	}
	return false
}

// ActiveDropdown returns true if any dropdown or picker is currently open.
func (fb FilterBar) ActiveDropdown() bool {
	return fb.typeDrop.IsOpen() || fb.statusDrop.IsOpen() ||
		fb.assigneeDrop.IsOpen() || fb.labelsDrop.IsOpen() ||
		fb.createdFrom.IsOpen() || fb.createdUntil.IsOpen() ||
		fb.searchActive
}

func (fb *FilterBar) closeAll() {
	fb.typeDrop.Close()
	fb.statusDrop.Close()
	fb.assigneeDrop.Close()
	fb.labelsDrop.Close()
	fb.createdFrom.Close()
	fb.createdUntil.Close()
	fb.searchActive = false
	fb.search.Blur()
}

// ClearAll resets all filters to defaults and returns a command to re-fetch.
func (fb *FilterBar) ClearAll() tea.Cmd {
	fb.typeDrop.Clear()
	fb.statusDrop.Clear()
	fb.assigneeDrop.Clear()
	fb.labelsDrop.Clear()
	fb.createdFrom = NewDatePicker("Created From", nil, 18)
	fb.createdUntil = NewDatePicker("Created Until", nil, 18)
	fb.search.SetValue("")
	fb.searchActive = false
	fb.search.Blur()
	// Do NOT restore default assignee — clear means "show everything"
	return func() tea.Msg { return filterChangedMsg{} }
}

// Height returns the number of terminal lines this component occupies.
func (fb FilterBar) Height() int {
	if !fb.expanded {
		return 1
	}
	return 8 // header(1) + row1 fields(3 lines) + row2 fields(3 lines) + separator(1)
}

// BuildJQL composes the JQL query string from all active filters.
func (fb FilterBar) BuildJQL(projectKey, orderBy string) string {
	var clauses []string

	if projectKey != "" {
		clauses = append(clauses, fmt.Sprintf(`project = "%s"`, projectKey))
	}

	// Issue type
	if fb.typeDrop.HasSelection() {
		names := fb.typeDrop.SelectedLabels()
		if len(names) == 1 {
			clauses = append(clauses, fmt.Sprintf(`issuetype = "%s"`, names[0]))
		} else {
			clauses = append(clauses, fmt.Sprintf(`issuetype IN (%s)`, quoteJoin(names)))
		}
	}

	// Status
	if fb.statusDrop.HasSelection() {
		names := fb.statusDrop.SelectedLabels()
		if len(names) == 1 {
			clauses = append(clauses, fmt.Sprintf(`status = "%s"`, names[0]))
		} else {
			clauses = append(clauses, fmt.Sprintf(`status IN (%s)`, quoteJoin(names)))
		}
	} else {
		// No explicit status filter: exclude Done to match default behavior
		clauses = append(clauses, "statusCategory != Done")
	}

	// Assignee
	if fb.assigneeDrop.HasSelection() {
		ids := fb.assigneeDrop.SelectedIDs()
		if len(ids) == 1 {
			clauses = append(clauses, fmt.Sprintf(`assignee = "%s"`, ids[0]))
		} else {
			clauses = append(clauses, fmt.Sprintf(`assignee IN (%s)`, quoteJoin(ids)))
		}
	}

	// Labels
	if fb.labelsDrop.HasSelection() {
		names := fb.labelsDrop.SelectedLabels()
		clauses = append(clauses, fmt.Sprintf(`labels IN (%s)`, quoteJoin(names)))
	}

	// Created from
	if fb.createdFrom.Value() != nil {
		clauses = append(clauses, fmt.Sprintf(`created >= "%s"`, fb.createdFrom.Value().Format("2006-01-02")))
	}

	// Created until
	if fb.createdUntil.Value() != nil {
		clauses = append(clauses, fmt.Sprintf(`created <= "%s"`, fb.createdUntil.Value().Format("2006-01-02")))
	}

	// Text search
	if fb.search.Value() != "" {
		escaped := strings.ReplaceAll(fb.search.Value(), `"`, `\"`)
		clauses = append(clauses, fmt.Sprintf(`text ~ "%s"`, escaped))
	}

	jql := strings.Join(clauses, " AND ")
	if orderBy != "" {
		jql += " ORDER BY " + orderBy
	}

	return jql
}

// quoteJoin joins strings as quoted, comma-separated list for JQL IN clauses.
func quoteJoin(items []string) string {
	quoted := make([]string, len(items))
	for i, s := range items {
		quoted[i] = fmt.Sprintf(`"%s"`, s)
	}
	return strings.Join(quoted, ", ")
}

// Update handles messages when the filter bar is active.
func (fb FilterBar) Update(msg tea.Msg) (FilterBar, tea.Cmd) {
	if !fb.expanded {
		return fb, nil
	}

	// Forward to open dropdown/picker
	if fb.typeDrop.IsOpen() {
		var cmd tea.Cmd
		fb.typeDrop, cmd = fb.typeDrop.Update(msg)
		if !fb.typeDrop.IsOpen() {
			// Dropdown just closed — trigger filter
			return fb, func() tea.Msg { return filterChangedMsg{} }
		}
		return fb, cmd
	}
	if fb.statusDrop.IsOpen() {
		var cmd tea.Cmd
		fb.statusDrop, cmd = fb.statusDrop.Update(msg)
		if !fb.statusDrop.IsOpen() {
			return fb, func() tea.Msg { return filterChangedMsg{} }
		}
		return fb, cmd
	}
	if fb.assigneeDrop.IsOpen() {
		var cmd tea.Cmd
		fb.assigneeDrop, cmd = fb.assigneeDrop.Update(msg)
		if !fb.assigneeDrop.IsOpen() {
			return fb, func() tea.Msg { return filterChangedMsg{} }
		}
		return fb, cmd
	}
	if fb.labelsDrop.IsOpen() {
		var cmd tea.Cmd
		fb.labelsDrop, cmd = fb.labelsDrop.Update(msg)
		if !fb.labelsDrop.IsOpen() {
			return fb, func() tea.Msg { return filterChangedMsg{} }
		}
		return fb, cmd
	}
	if fb.createdFrom.IsOpen() {
		var cmd tea.Cmd
		fb.createdFrom, cmd = fb.createdFrom.Update(msg)
		if !fb.createdFrom.IsOpen() {
			return fb, func() tea.Msg { return filterChangedMsg{} }
		}
		return fb, cmd
	}
	if fb.createdUntil.IsOpen() {
		var cmd tea.Cmd
		fb.createdUntil, cmd = fb.createdUntil.Update(msg)
		if !fb.createdUntil.IsOpen() {
			return fb, func() tea.Msg { return filterChangedMsg{} }
		}
		return fb, cmd
	}

	// Search input focused
	if fb.searchActive {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc":
				fb.searchActive = false
				fb.search.Blur()
				return fb, nil
			case "enter":
				fb.searchActive = false
				fb.search.Blur()
				return fb, func() tea.Msg { return filterChangedMsg{} }
			default:
				var cmd tea.Cmd
				fb.search, cmd = fb.search.Update(msg)
				// Debounce search
				fb.searchSeq++
				seq := fb.searchSeq
				tickCmd := tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
					return filterSearchTick{seq: seq}
				})
				return fb, tea.Batch(cmd, tickCmd)
			}
		default:
			var cmd tea.Cmd
			fb.search, cmd = fb.search.Update(msg)
			return fb, cmd
		}
	}

	// Handle shortcut keys when no dropdown is open
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "t":
			fb.closeAll()
			fb.typeDrop.Open()
			return fb, nil
		case "s":
			fb.closeAll()
			fb.statusDrop.Open()
			return fb, nil
		case "a":
			fb.closeAll()
			fb.assigneeDrop.Open()
			return fb, nil
		case "l":
			fb.closeAll()
			fb.labelsDrop.Open()
			return fb, nil
		case "d":
			fb.closeAll()
			fb.createdFrom.OpenPicker()
			return fb, nil
		case "D":
			fb.closeAll()
			fb.createdUntil.OpenPicker()
			return fb, nil
		case "/":
			fb.closeAll()
			fb.searchActive = true
			fb.search.Focus()
			return fb, fb.search.Cursor.BlinkCmd()
		case "x":
			return fb, fb.ClearAll()
		case "esc":
			fb.Collapse()
			return fb, nil
		}

	case filterSearchTick:
		if msg.seq == fb.searchSeq {
			return fb, func() tea.Msg { return filterChangedMsg{} }
		}
		return fb, nil
	}

	return fb, nil
}

// View renders the filter bar.
func (fb FilterBar) View() string {
	if !fb.expanded {
		return fb.renderCollapsed()
	}
	return fb.renderExpanded()
}

func (fb FilterBar) renderCollapsed() string {
	style := lipgloss.NewStyle().Foreground(colorAccent)
	hint := lipgloss.NewStyle().Foreground(colorSubtle)

	if !fb.HasActiveFilters() {
		return style.Render("▶ Filters") + " " + hint.Render("(f to expand)")
	}

	var parts []string
	if fb.typeDrop.HasSelection() {
		parts = append(parts, "Type="+fb.typeDrop.ValueText())
	}
	if fb.statusDrop.HasSelection() {
		parts = append(parts, "Status="+fb.statusDrop.ValueText())
	}
	if fb.assigneeDrop.HasSelection() {
		ids := fb.assigneeDrop.SelectedIDs()
		// Only show if different from default
		if fb.defaultAssigneeID == "" || len(ids) != 1 || ids[0] != fb.defaultAssigneeID {
			parts = append(parts, "Assignee="+fb.assigneeDrop.ValueText())
		}
	}
	if fb.labelsDrop.HasSelection() {
		parts = append(parts, "Labels="+fb.labelsDrop.ValueText())
	}
	if fb.createdFrom.Value() != nil {
		parts = append(parts, "From="+fb.createdFrom.Value().Format("2006-01-02"))
	}
	if fb.createdUntil.Value() != nil {
		parts = append(parts, "Until="+fb.createdUntil.Value().Format("2006-01-02"))
	}
	if fb.search.Value() != "" {
		parts = append(parts, "Search="+fb.search.Value())
	}

	valStyle := lipgloss.NewStyle().Foreground(colorSuccess)
	sep := lipgloss.NewStyle().Foreground(colorSubtle).Render(" · ")

	summary := style.Render("▶ Filters: ")
	for i, part := range parts {
		if i > 0 {
			summary += sep
		}
		summary += valStyle.Render(part)
	}
	summary += " " + hint.Render("(f)")

	// Truncate to width
	if lipgloss.Width(summary) > fb.width {
		summary = truncStr(summary, fb.width-4) + hint.Render(" (f)")
	}

	return summary
}

func (fb FilterBar) renderExpanded() string {
	var b strings.Builder

	// Header
	style := lipgloss.NewStyle().Foreground(colorAccent)
	hint := lipgloss.NewStyle().Foreground(colorSubtle)
	b.WriteString(style.Render("▼ Filters") + " " + hint.Render("(esc to collapse)"))
	b.WriteString("\n")

	// Row 1: Issue Type, Status, Assignee, Labels
	fieldW := fieldWidth(fb.width, 4)
	fb.typeDrop.SetWidth(fieldW)
	fb.statusDrop.SetWidth(fieldW)
	fb.assigneeDrop.SetWidth(fieldW)
	fb.labelsDrop.SetWidth(fieldW)

	row1Fields := []string{
		fb.typeDrop.View(),
		fb.statusDrop.View(),
		fb.assigneeDrop.View(),
		fb.labelsDrop.View(),
	}
	b.WriteString(joinFieldsHorizontal(row1Fields))
	b.WriteString("\n")

	// Row 2: Created From, Created Until, Search
	dateW := 20
	searchW := fb.width - dateW*2 - 2 // 2 gaps
	if searchW < 20 {
		searchW = 20
	}

	// Build search field manually (same box style)
	searchView := fb.renderSearchField(searchW)

	row2Fields := []string{
		fb.createdFrom.View(),
		fb.createdUntil.View(),
		searchView,
	}
	b.WriteString(joinFieldsHorizontal(row2Fields))

	return b.String()
}

func (fb FilterBar) renderSearchField(width int) string {
	if width < 8 {
		width = 8
	}
	innerW := width - 2
	valW := innerW - 2

	lbl := lipgloss.NewStyle().Foreground(colorAccent)
	bdr := lipgloss.NewStyle().Foreground(colorBorder)

	labelText := " Search "
	if fb.searchActive {
		bdr = bdr.Foreground(colorAccent)
		labelText = " Search (/) "
	}

	dashes := innerW - lipgloss.Width(labelText) - 1
	if dashes < 0 {
		dashes = 0
	}

	top := bdr.Render("╭─") + lbl.Render(labelText) + bdr.Render(strings.Repeat("─", dashes)+"╮")

	var content string
	if fb.searchActive {
		fb.search.Width = valW
		content = fb.search.View()
	} else if fb.search.Value() != "" {
		content = lipgloss.NewStyle().Foreground(colorText).Render(truncStr(fb.search.Value(), valW))
	} else {
		content = lipgloss.NewStyle().Foreground(colorSubtle).Render(truncStr("Filter by key or summary...", valW))
	}
	visW := lipgloss.Width(content)
	pad := valW - visW
	if pad < 0 {
		pad = 0
	}
	mid := bdr.Render("│") + " " + content + strings.Repeat(" ", pad) + " " + bdr.Render("│")
	bot := bdr.Render("╰" + strings.Repeat("─", innerW) + "╯")

	return top + "\n" + mid + "\n" + bot
}

// joinFieldsHorizontal joins multi-line field views side by side.
func joinFieldsHorizontal(fields []string) string {
	// Split each field into lines
	allLines := make([][]string, len(fields))
	maxLines := 0
	for i, f := range fields {
		allLines[i] = strings.Split(f, "\n")
		if len(allLines[i]) > maxLines {
			maxLines = len(allLines[i])
		}
	}

	var rows []string
	for lineIdx := 0; lineIdx < maxLines; lineIdx++ {
		var row string
		for fieldIdx, lines := range allLines {
			if fieldIdx > 0 {
				row += " "
			}
			if lineIdx < len(lines) {
				row += lines[lineIdx]
			}
		}
		rows = append(rows, row)
	}

	return strings.Join(rows, "\n")
}

// OverlayLines returns any open dropdown/picker overlay lines with their
// position info (startLine relative to filter bar top, startCol).
func (fb FilterBar) OverlayLines() (lines []string, startLine, startCol int) {
	fieldW := fieldWidth(fb.width, 4)

	if fb.typeDrop.IsOpen() {
		return fb.typeDrop.RenderOverlay(), 4, 0 // below row1 field (header + 3 field lines)
	}
	if fb.statusDrop.IsOpen() {
		return fb.statusDrop.RenderOverlay(), 4, fieldW + 1
	}
	if fb.assigneeDrop.IsOpen() {
		return fb.assigneeDrop.RenderOverlay(), 4, (fieldW+1)*2
	}
	if fb.labelsDrop.IsOpen() {
		return fb.labelsDrop.RenderOverlay(), 4, (fieldW+1)*3
	}
	if fb.createdFrom.IsOpen() {
		return fb.createdFrom.RenderOverlay(), 7, 0 // below row2 field
	}
	if fb.createdUntil.IsOpen() {
		return fb.createdUntil.RenderOverlay(), 7, 21 // after date field + gap
	}

	return nil, 0, 0
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/christopherdobbyn/work/dobbo-ca/jiratui && go test ./internal/tui/ -run TestFilterBar -v`
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tui/filterbar.go internal/tui/filterbar_test.go
git commit -m "feat(tui): add FilterBar component with JQL builder"
```

---

### Task 4: Remove old inline filter from List

**Files:**
- Modify: `internal/tui/list.go`
- Modify: `internal/tui/keys.go`
- Modify: `internal/tui/list_test.go`

- [ ] **Step 1: Update list_test.go — remove TestFilterIssues**

The `filterIssues` function is being removed. Delete the `TestFilterIssues` function (lines 49–76) from `internal/tui/list_test.go`.

- [ ] **Step 2: Remove filter state from List struct**

In `internal/tui/list.go`, remove the `filter` and `filtering` fields from the `List` struct (lines 49-50):

Remove:
```go
	filter      textinput.Model
	filtering   bool
```

- [ ] **Step 3: Remove filter from NewList**

In `internal/tui/list.go`, update `NewList` (lines 75-88) to remove the text input initialization:

```go
func NewList(issues []models.Issue, width, height int) List {
	return List{
		issues:      issues,
		filtered:    issues,
		width:       width,
		height:      height,
		keyColWidth: maxKeyWidth(issues) + 2,
	}
}
```

- [ ] **Step 4: Remove filter rendering and key handling from List**

In `internal/tui/list.go`, update `visibleRows` to remove the `filtering` check:

```go
func (l *List) visibleRows() int {
	h := l.height - 5
	if h < 1 {
		h = 1
	}
	return h
}
```

In the `Update` method, remove the entire `if l.filtering` block (lines 253-275) and the `case key.Matches(msg, listKeys.Filter):` block (lines 278-281).

In the `View` method, remove the `if l.filtering` block (lines 421-428).

In the `ViewWithWidth` method, remove the filter-related `headerOffset` adjustment — change:
```go
		headerOffset := 2
		if l.filtering {
			headerOffset++
		}
```
to:
```go
		headerOffset := 2
```

- [ ] **Step 5: Remove filterIssues function**

Delete the `filterIssues` function (lines 213-226) from `internal/tui/list.go`.

- [ ] **Step 6: Update SetIssues to not use filterIssues**

In `internal/tui/list.go`, update `SetIssues`:

```go
func (l *List) SetIssues(issues []models.Issue) {
	l.issues = issues
	l.filtered = issues
	l.cursor = 0
	l.offset = 0
}
```

- [ ] **Step 7: Remove the Escape key handler for filter clearing**

In the list `Update` key handler, remove the `key.Matches(msg, listKeys.Escape)` case that was inside the `l.filtering` block (already removed in step 4).

- [ ] **Step 8: Remove Filter key binding from keys.go**

In `internal/tui/keys.go`, remove the `Filter` field from `ListKeyMap` (line 11) and its binding from `listKeys` (lines 35-38):

Remove from struct:
```go
	Filter   key.Binding
```

Remove from var:
```go
	Filter: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "filter"),
	),
```

- [ ] **Step 9: Remove unused textinput import from list.go**

Remove `"github.com/charmbracelet/bubbles/textinput"` from the import block in `list.go`. Also remove the `key` import if no longer used (check: it's still used for `listKeys.Escape` etc., so keep it).

- [ ] **Step 10: Verify it compiles and tests pass**

Run: `cd /Users/christopherdobbyn/work/dobbo-ca/jiratui && go build ./... && go test ./internal/tui/ -v`
Expected: builds clean, all tests pass

- [ ] **Step 11: Commit**

```bash
git add internal/tui/list.go internal/tui/keys.go internal/tui/list_test.go
git commit -m "refactor(tui): remove old inline filter from List component"
```

---

### Task 5: Integrate FilterBar into App

**Files:**
- Modify: `internal/tui/app.go`

This is the largest task. It wires up the filter bar, new message types, fetch commands, and view compositing.

- [ ] **Step 1: Add new message types and fetch commands**

In `internal/tui/app.go`, add after the `linkTypesMsg` type (around line 330):

```go
// statusesAndTypesMsg carries fetched statuses and issue types for a project.
type statusesAndTypesMsg struct {
	statuses   []models.Status
	issueTypes []models.IssueType
}

func fetchStatusesAndTypes(client *jira.Client, projectKey string) tea.Cmd {
	return func() tea.Msg {
		if projectKey == "" {
			return nil
		}
		statuses, issueTypes, err := client.GetProjectStatusesAndTypes(projectKey)
		if err != nil {
			logDebug("GetProjectStatusesAndTypes(%s) error: %v", projectKey, err)
			return nil
		}
		return statusesAndTypesMsg{statuses: statuses, issueTypes: issueTypes}
	}
}

func fetchIssuesWithJQL(client *jira.Client, jql string) tea.Cmd {
	return func() tea.Msg {
		result, err := client.SearchIssues(jql, 50, "")
		if err != nil {
			return errMsg{err: err}
		}
		return issuesMsg{issues: result.Issues}
	}
}
```

- [ ] **Step 2: Add filterBar field to App struct**

In `internal/tui/app.go`, add to the `App` struct (after `profileDrop` around line 555):

```go
	filterBar     FilterBar
```

- [ ] **Step 3: Initialize FilterBar in NewApp**

In `NewApp`, before the `return App{` block, add:

```go
	filterBar := NewFilterBar(0) // width set on first WindowSizeMsg
	if myAccountID != "" {
		filterBar.SetDefaultAssignee(myAccountID, "")
	}
```

And include `filterBar: filterBar,` in the returned App struct.

- [ ] **Step 4: Update App.Init to fetch statuses/types**

In `App.Init()`, update the `tea.Batch` to include the new fetch:

```go
func (a App) Init() tea.Cmd {
	return tea.Batch(
		a.spinner.Tick,
		fetchIssues(a.client, a.sort, a.projectKey),
		fetchProjects(a.client),
		fetchStatusesAndTypes(a.client, a.projectKey),
	)
}
```

- [ ] **Step 5: Handle filter bar in WindowSizeMsg**

In the `tea.WindowSizeMsg` handler, after setting `a.listWidth`, add:

```go
		a.filterBar.SetWidth(a.listWidth)
```

- [ ] **Step 6: Handle filter bar keyboard routing in Update**

In `App.Update`, in the `tea.KeyMsg` case, add filter bar routing after the profile dropdown check and before the detail editing check. The key routing logic:

```go
		// Filter bar: toggle with f, forward / to expand+search
		if a.state == stateList {
			// f toggles filter bar
			if msg.String() == "f" && !a.filterBar.ActiveDropdown() {
				a.filterBar.Toggle()
				return a, nil
			}
			// / expands filter bar and focuses search
			if msg.String() == "/" {
				if !a.filterBar.IsExpanded() {
					a.filterBar.Expand()
				}
				// Simulate / key to focus search
				var cmd tea.Cmd
				a.filterBar, cmd = a.filterBar.Update(msg)
				return a, cmd
			}
		}

		// When filter bar is expanded and has focus, forward keys to it
		if a.filterBar.IsExpanded() && (a.filterBar.ActiveDropdown() || isFilterBarKey(msg.String())) {
			var cmd tea.Cmd
			a.filterBar, cmd = a.filterBar.Update(msg)
			return a, cmd
		}
```

Add a helper function:

```go
// isFilterBarKey returns true if the key is a filter bar shortcut.
func isFilterBarKey(key string) bool {
	switch key {
	case "t", "s", "a", "l", "d", "D", "x":
		return true
	}
	return false
}
```

- [ ] **Step 7: Handle filterChangedMsg**

In `App.Update`, add a case for `filterChangedMsg`:

```go
	case filterChangedMsg:
		jql := a.filterBar.BuildJQL(a.projectKey, a.sort.orderByClause())
		a.state = stateLoading
		a.detail = nil
		a.detailKey = ""
		return a, tea.Batch(a.spinner.Tick, fetchIssuesWithJQL(a.client, jql))
```

- [ ] **Step 8: Handle filterSearchTick**

In `App.Update`, add a case for `filterSearchTick`:

```go
	case filterSearchTick:
		var cmd tea.Cmd
		a.filterBar, cmd = a.filterBar.Update(msg)
		return a, cmd
```

- [ ] **Step 9: Handle statusesAndTypesMsg**

In `App.Update`, add a case:

```go
	case statusesAndTypesMsg:
		statusItems := make([]DropdownItem, len(msg.statuses))
		for i, s := range msg.statuses {
			statusItems[i] = DropdownItem{ID: s.ID, Label: s.Name}
		}
		a.filterBar.SetStatusItems(statusItems)

		typeItems := make([]DropdownItem, len(msg.issueTypes))
		for i, it := range msg.issueTypes {
			typeItems[i] = DropdownItem{ID: it.ID, Label: it.Name}
		}
		a.filterBar.SetTypeItems(typeItems)
		return a, nil
```

- [ ] **Step 10: Extract labels from issuesMsg and populate assignees**

In the `issuesMsg` handler, after setting `a.state = stateList`, add:

```go
		// Extract unique labels from fetched issues for the filter bar
		labelSet := make(map[string]bool)
		for _, issue := range msg.issues {
			for _, l := range issue.Labels {
				labelSet[l] = true
			}
		}
		if len(labelSet) > 0 {
			labelItems := make([]DropdownItem, 0, len(labelSet))
			for l := range labelSet {
				labelItems = append(labelItems, DropdownItem{ID: l, Label: l})
			}
			a.filterBar.SetLabelItems(labelItems)
		}
```

- [ ] **Step 11: Populate filter bar assignees from boardAssigneesMsg**

In the `boardAssigneesMsg` handler, add after the existing detail logic:

```go
		// Also populate filter bar assignees
		assigneeItems := make([]DropdownItem, len(msg.users))
		for i, u := range msg.users {
			label := u.DisplayName
			if u.AccountID == a.myAccountID {
				label += " (me)"
			}
			assigneeItems[i] = DropdownItem{ID: u.AccountID, Label: label}
		}
		a.filterBar.SetAssigneeItems(assigneeItems)
```

- [ ] **Step 12: Re-fetch statuses/types on project change**

In the project change handlers (both keyboard and mouse), add `fetchStatusesAndTypes` to the batch. Find both places where `fetchIssues(a.client, a.sort, a.projectKey)` is called after a project change and add:

```go
fetchStatusesAndTypes(a.client, a.projectKey),
```

Also clear filter bar selections when project changes:
```go
a.filterBar.ClearAll()
```

- [ ] **Step 13: Update fetchIssues to use filter bar JQL when available**

Update the initial `fetchIssues` call in `App.Init()` — this is fine as-is since the filter bar's `BuildJQL` handles defaults. But update the refresh (`r` key) to use filter bar:

```go
		if msg.String() == "r" && !a.list.filtering {
			a.state = stateLoading
			a.err = nil
			a.detail = nil
			a.detailKey = ""
			jql := a.filterBar.BuildJQL(a.projectKey, a.sort.orderByClause())
			return a, tea.Batch(a.spinner.Tick, fetchIssuesWithJQL(a.client, jql))
		}
```

Also update the sort click handler to use filter bar JQL:

```go
	case sortClickMsg:
		// ... existing sort toggle logic ...
		a.state = stateLoading
		a.detail = nil
		a.detailKey = ""
		jql := a.filterBar.BuildJQL(a.projectKey, a.sort.orderByClause())
		return a, tea.Batch(a.spinner.Tick, fetchIssuesWithJQL(a.client, jql))
```

- [ ] **Step 14: Update App.View to render filter bar**

In `App.View()`, in the list rendering section, render the filter bar above the list. Replace the left pane rendering block:

```go
		var left string
		if a.state == stateLoading {
			loadStyle := lipgloss.NewStyle().
				Width(listW).
				Height(contentH).
				Foreground(colorText).
				Align(lipgloss.Center, lipgloss.Center)
			left = loadStyle.Render(a.spinner.View() + " Loading...")
		} else {
			filterView := a.filterBar.View()
			filterH := a.filterBar.Height()
			listH := contentH - filterH
			if listH < 3 {
				listH = 3
			}
			listView := a.list.ViewWithWidth(listW, listH)
			left = filterView + "\n" + listView
		}
```

- [ ] **Step 15: Composite filter bar overlays**

In `App.View()`, after building `contentLines`, add overlay compositing for filter bar dropdowns (before the existing project/profile dropdown overlay):

```go
		// Composite filter bar dropdown overlays
		if a.filterBar.IsExpanded() {
			overlayLines, startLine, startCol := a.filterBar.OverlayLines()
			if overlayLines != nil {
				for i, oLine := range overlayLines {
					idx := startLine + i
					if idx >= 0 && idx < len(contentLines) {
						existing := contentLines[idx]
						existVis := lipgloss.Width(existing)
						if existVis > startCol {
							existing = truncateAnsi(existing, startCol)
						} else {
							existing += strings.Repeat(" ", startCol-existVis)
						}
						contentLines[idx] = existing + oLine
					}
				}
			}
		}
```

- [ ] **Step 16: Update help bar text**

In `renderHelpBar()`, update the help text. Change:

```go
		help = "/ filter · p project · o browser · r refresh · q quit · ? help"
```

to:

```go
		help = "f filters · p project · o browser · r refresh · q quit · ? help"
```

Add a new condition for when the filter bar is expanded (before the existing conditions):

```go
	if a.filterBar.IsExpanded() && a.filterBar.ActiveDropdown() {
		help = "↑↓ navigate · enter/space toggle · esc close"
	} else if a.filterBar.IsExpanded() {
		help = "t type · s status · a assignee · l labels · d dates · / search · x clear · esc close"
	} else if a.projectDrop.IsOpen() || a.profileDrop.IsOpen() {
```

- [ ] **Step 17: Remove `a.list.filtering` references**

Search `app.go` for `a.list.filtering` — there are references in the help bar and key routing. Remove all references to `a.list.filtering` since the `filtering` field no longer exists. Specifically:

- In the key routing for `q`: change `!a.list.filtering` to just check state
- In the help bar key routing for the project dropdown `p`: remove `!a.list.filtering`
- In the help bar condition: remove the `a.state == stateList && a.list.filtering` branch
- In help bar click zone check: remove `!a.list.filtering`

- [ ] **Step 18: Verify it compiles**

Run: `cd /Users/christopherdobbyn/work/dobbo-ca/jiratui && go build ./...`
Expected: no errors

- [ ] **Step 19: Commit**

```bash
git add internal/tui/app.go
git commit -m "feat(tui): integrate FilterBar into App with JQL-based filtering"
```

---

### Task 6: Update help screen and final polish

**Files:**
- Modify: `internal/tui/app.go` (help screen)

- [ ] **Step 1: Update the help screen**

In `renderHelpScreen()`, add a "Filters" section after the "Actions" section:

```go
	section("Filters")
	entry("f", "Toggle filter bar")
	entry("t / s / a / l", "Type / Status / Assignee / Labels")
	entry("d / D", "Created from / until")
	entry("/", "Search")
	entry("x", "Clear all filters")
```

Remove the existing `entry("/", "Filter issues")` from the Actions section.

- [ ] **Step 2: Run full test suite**

Run: `cd /Users/christopherdobbyn/work/dobbo-ca/jiratui && go test ./... -v`
Expected: all tests pass

- [ ] **Step 3: Build and smoke test**

Run: `cd /Users/christopherdobbyn/work/dobbo-ca/jiratui && go build -o jiratui .`
Expected: binary builds successfully

- [ ] **Step 4: Commit**

```bash
git add internal/tui/app.go
git commit -m "feat(tui): update help screen with filter bar shortcuts"
```
