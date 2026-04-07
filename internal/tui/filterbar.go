package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/christopherdobbyn/jiratui/internal/config"
)

type filterChangedMsg struct{}
type filterSaveMsg struct{}
type sortChangedMsg struct{}
type filterSaveFlashTickMsg struct{}
type filterDeleteViewMsg struct{ name string }

const DefaultViewName = "Default"

type filterSearchTick struct {
	seq int
}

var sinceOptions = []DropdownItem{
	{ID: "", Label: "Any time"},
	{ID: "-1d", Label: "Today"},
	{ID: "-3d", Label: "Last 3 days"},
	{ID: "-1w", Label: "Last week"},
	{ID: "-2w", Label: "Last 2 weeks"},
	{ID: "-4w", Label: "Last month"},
	{ID: "-12w", Label: "Last 3 months"},
	{ID: "-52w", Label: "Last year"},
}

var sortFields = []DropdownItem{
	{ID: "updated", Label: "Updated"},
	{ID: "created", Label: "Created"},
	{ID: "key", Label: "Key"},
	{ID: "summary", Label: "Summary"},
	{ID: "priority", Label: "Priority"},
	{ID: "status", Label: "Status"},
	{ID: "assignee", Label: "Assignee"},
}

type FilterBar struct {
	expanded     bool
	typeDrop     MultiSelect
	statusDrop   MultiSelect
	priorityDrop MultiSelect
	assigneeDrop MultiSelect
	labelsDrop   MultiSelect
	createdFrom  DatePicker
	createdUntil DatePicker
	search       textinput.Model
	searchSeq    int
	searchActive bool
	sinceDrop    Dropdown
	sortDrop     Dropdown
	sortAsc      bool
	claudeFilter bool // local toggle: show only tickets with Claude sessions
	width        int
	saveFlash    int // countdown frames for "Saved!" flash (0 = not flashing)

	// Action row state
	saveNaming      bool           // true when save name input is active
	saveInput       textinput.Model
	loadDrop        Dropdown       // dropdown of saved view names
	savedViews      map[string]config.SavedFilters // name → filters (for current project)
	activeView      string         // name of the currently loaded view
	confirmDelete   bool           // true when showing delete confirmation
	confirmDelName  string         // name of view to delete

	defaultAssigneeID   string
	defaultAssigneeName string
}

func NewFilterBar(width int) FilterBar {
	ti := textinput.New()
	ti.Placeholder = "Filter by key or summary..."
	ti.CharLimit = 200
	ti.Prompt = ""

	si := textinput.New()
	si.Placeholder = "View name..."
	si.CharLimit = 20
	si.Prompt = ""

	fieldW := fieldWidth(width, 5)

	statusDrop := NewMultiSelect("Status", nil, fieldW)
	statusDrop.overlayWidth = 30

	assigneeDrop := NewMultiSelect("Assignee", nil, fieldW)
	assigneeDrop.overlayWidth = 40

	labelsDrop := NewMultiSelect("Labels", nil, fieldW)
	labelsDrop.searchable = true
	labelsDrop.overlayWidth = 35

	defaultItems := []DropdownItem{{ID: DefaultViewName, Label: DefaultViewName}}
	loadDrop := NewSimpleDropdown("Load View", defaultItems, DefaultViewName, DefaultViewName, 25)
	loadDrop.overlayWidth = 40

	return FilterBar{
		typeDrop:     NewMultiSelect("Issue Type", nil, fieldW),
		statusDrop:   statusDrop,
		priorityDrop: NewMultiSelect("Priority", nil, fieldW),
		assigneeDrop: assigneeDrop,
		labelsDrop:   labelsDrop,
		createdFrom:  NewDatePicker("Created From", nil, 18),
		createdUntil: NewDatePicker("Created Until", nil, 18),
		sinceDrop:    NewSimpleDropdown("Since", sinceOptions, "Any time", "", 18),
		sortDrop:     NewSimpleDropdown("Sort By", sortFields, "Created", "created", 18),
		loadDrop:     loadDrop,
		search:       ti,
		saveInput:    si,
		sortAsc:      false,
		activeView:   DefaultViewName,
		savedViews:   make(map[string]config.SavedFilters),
		width:        width,
	}
}

func fieldWidth(totalW, count int) int {
	w := totalW / count
	if w < 16 {
		w = 16
	}
	return w
}

// fieldCount is the total number of fields in the expanded filter bar (excluding action row).
const fieldCount = 12

// Indices into the field list for click/overlay routing.
const (
	fieldType      = 0
	fieldStatus    = 1
	fieldPriority  = 2
	fieldAssignee  = 3
	fieldLabels    = 4
	fieldFrom      = 5
	fieldUntil     = 6
	fieldSearch    = 7
	fieldSince     = 8
	fieldSortBy    = 9
	fieldDirection = 10
	fieldClaude    = 11
)

// Action button indices in the action row.
const (
	actionSave  = 0
	actionLoad  = 1
	actionClear = 2
)

// minFieldW is the minimum width for a single field.
const minFieldW = 16

// fieldsPerRow returns how many fields fit in one row at the current width.
func (fb FilterBar) fieldsPerRow() int {
	n := fb.width / minFieldW
	if n < 1 {
		n = 1
	}
	if n > fieldCount {
		n = fieldCount
	}
	return n
}

// numRows returns the number of field rows needed.
func (fb FilterBar) numRows() int {
	fpr := fb.fieldsPerRow()
	return (fieldCount + fpr - 1) / fpr
}

// fieldRowWidth returns the per-field width for a row containing count fields.
func (fb FilterBar) fieldRowWidth(count int) int {
	return fieldWidth(fb.width, count)
}

// fieldPosition returns the row (0-based) and column (0-based) for a field index.
func (fb FilterBar) fieldPosition(idx int) (row, col int) {
	fpr := fb.fieldsPerRow()
	return idx / fpr, idx % fpr
}

func (fb *FilterBar) SetDefaultAssignee(accountID, displayName string) {
	fb.defaultAssigneeID = accountID
	fb.defaultAssigneeName = displayName
	found := false
	for _, item := range fb.assigneeDrop.items {
		if item.ID == accountID {
			found = true
			break
		}
	}
	if !found {
		fb.assigneeDrop.items = append(fb.assigneeDrop.items, DropdownItem{ID: accountID, Label: displayName})
	}
	fb.assigneeDrop.selected[accountID] = true
}

func (fb *FilterBar) SetWidth(w int) {
	fb.width = w
	// All fields get uniform width based on how many fit per row
	fpr := fb.fieldsPerRow()
	fieldW := fb.fieldRowWidth(fpr)
	fb.typeDrop.SetWidth(fieldW)
	fb.statusDrop.SetWidth(fieldW)
	fb.priorityDrop.SetWidth(fieldW)
	fb.assigneeDrop.SetWidth(fieldW)
	fb.labelsDrop.SetWidth(fieldW)
	fb.createdFrom.width = fieldW
	fb.createdUntil.width = fieldW
	fb.sinceDrop.width = fieldW
	fb.sortDrop.width = fieldW
}

func (fb FilterBar) sinceValue() string {
	if sel := fb.sinceDrop.SelectedItem(); sel != nil {
		return sel.ID
	}
	return ""
}

// Sort accessors
func (fb FilterBar) SortField() string {
	if sel := fb.sortDrop.SelectedItem(); sel != nil {
		return sel.ID
	}
	return "updated"
}
func (fb FilterBar) SortAsc() bool          { return fb.sortAsc }
func (fb *FilterBar) ToggleSortDirection()   { fb.sortAsc = !fb.sortAsc }
func (fb FilterBar) OrderByClause() string {
	field := fb.SortField()
	dir := "DESC"
	if fb.sortAsc {
		dir = "ASC"
	}
	return field + " " + dir
}
func (fb FilterBar) SortLabel() string {
	if sel := fb.sortDrop.SelectedItem(); sel != nil {
		return sel.Label
	}
	return "Updated"
}

func (fb *FilterBar) SetStatusItems(items []DropdownItem)   { fb.statusDrop.SetItemsKeepSelection(items) }
func (fb *FilterBar) SetTypeItems(items []DropdownItem)     { fb.typeDrop.SetItemsKeepSelection(items) }
func (fb *FilterBar) SetPriorityItems(items []DropdownItem) { fb.priorityDrop.SetItemsKeepSelection(items) }
func (fb *FilterBar) SetAssigneeItems(items []DropdownItem) {
	// Ensure "Unassigned" is always the first option
	hasUnassigned := false
	for _, item := range items {
		if item.ID == "" {
			hasUnassigned = true
			break
		}
	}
	if !hasUnassigned {
		items = append([]DropdownItem{{ID: "unassigned", Label: "Unassigned"}}, items...)
	}
	fb.assigneeDrop.SetItemsKeepSelection(items)
}
func (fb *FilterBar) SetLabelItems(items []DropdownItem)    { fb.labelsDrop.SetItemsKeepSelection(items) }

func (fb FilterBar) IsExpanded() bool { return fb.expanded }

func (fb *FilterBar) Expand()   { fb.expanded = true }
func (fb *FilterBar) Collapse() { fb.expanded = false; fb.closeAll() }
func (fb *FilterBar) Toggle() {
	if fb.expanded {
		fb.Collapse()
	} else {
		fb.Expand()
	}
}

func (fb FilterBar) HasActiveFilters() bool {
	// Check raw selected maps (works even before items are loaded)
	if len(fb.typeDrop.selected) > 0 {
		return true
	}
	if len(fb.statusDrop.selected) > 0 {
		return true
	}
	if fb.defaultAssigneeID != "" {
		// Check if assignee differs from default
		if len(fb.assigneeDrop.selected) != 1 || !fb.assigneeDrop.selected[fb.defaultAssigneeID] {
			if len(fb.assigneeDrop.selected) > 0 {
				return true
			}
		}
	} else if len(fb.assigneeDrop.selected) > 0 {
		return true
	}
	if len(fb.priorityDrop.selected) > 0 {
		return true
	}
	if len(fb.labelsDrop.selected) > 0 {
		return true
	}
	if sel := fb.sinceDrop.SelectedItem(); sel != nil && sel.ID != "" {
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

func (fb FilterBar) ActiveDropdown() bool {
	return fb.typeDrop.IsOpen() || fb.statusDrop.IsOpen() ||
		fb.priorityDrop.IsOpen() || fb.assigneeDrop.IsOpen() ||
		fb.labelsDrop.IsOpen() || fb.createdFrom.IsOpen() ||
		fb.createdUntil.IsOpen() || fb.sinceDrop.IsOpen() ||
		fb.sortDrop.IsOpen() ||
		fb.loadDrop.IsOpen() || fb.searchActive || fb.saveNaming ||
		fb.confirmDelete
}

func (fb *FilterBar) closeAll() {
	fb.typeDrop.Close()
	fb.statusDrop.Close()
	fb.priorityDrop.Close()
	fb.assigneeDrop.Close()
	fb.labelsDrop.Close()
	fb.createdFrom.Close()
	fb.createdUntil.Close()
	fb.sinceDrop.Close()
	fb.sortDrop.Close()
	fb.loadDrop.Close()
	fb.searchActive = false
	fb.search.Blur()
	fb.saveNaming = false
	fb.saveInput.Blur()
}

// clearFiltersOnly resets all filter/sort state without emitting messages.
func (fb *FilterBar) clearFiltersOnly() {
	fb.typeDrop.Clear()
	fb.statusDrop.Clear()
	fb.priorityDrop.Clear()
	fb.assigneeDrop.Clear()
	fb.labelsDrop.Clear()
	fb.createdFrom = NewDatePicker("Created From", nil, fb.createdFrom.width)
	fb.createdUntil = NewDatePicker("Created Until", nil, fb.createdUntil.width)
	fb.search.SetValue("")
	fb.searchActive = false
	fb.search.Blur()
	// Reset since and sort to default
	fb.sinceDrop.value = "Any time"
	fb.sinceDrop.selected = 0
	fb.sortAsc = false
	for i, item := range sortFields {
		if item.ID == "created" {
			fb.sortDrop.value = item.Label
			fb.sortDrop.selected = i
			break
		}
	}
	fb.claudeFilter = false
	fb.activeView = ""
	fb.saveFlash = 0
}

func (fb *FilterBar) ClearAll() tea.Cmd {
	fb.clearFiltersOnly()
	return func() tea.Msg { return filterChangedMsg{} }
}

func (fb FilterBar) Height() int {
	if !fb.expanded {
		return strings.Count(fb.renderCollapsed(), "\n") + 1
	}
	return 1 + fb.numRows()*3 + 1 + 3 // header + 3 lines per field row + divider + 3 line action row
}


func (fb FilterBar) BuildJQL(projectKey, orderBy string) string {
	var clauses []string

	if projectKey != "" {
		clauses = append(clauses, fmt.Sprintf(`project = "%s"`, projectKey))
	}

	if names := fb.typeDrop.SelectedLabels(); len(names) > 0 {
		if len(names) == 1 {
			clauses = append(clauses, fmt.Sprintf(`issuetype = "%s"`, names[0]))
		} else {
			clauses = append(clauses, fmt.Sprintf(`issuetype IN (%s)`, quoteJoin(names)))
		}
	}
	// Note: type/status/priority use names in JQL, not IDs.
	// RawSelectedIDs contains numeric IDs which are invalid in JQL.
	// These filters apply once items load via SetItemsKeepSelection.

	if names := fb.statusDrop.SelectedLabels(); len(names) > 0 {
		if len(names) == 1 {
			clauses = append(clauses, fmt.Sprintf(`status = "%s"`, names[0]))
		} else {
			clauses = append(clauses, fmt.Sprintf(`status IN (%s)`, quoteJoin(names)))
		}
	}

	if names := fb.priorityDrop.SelectedLabels(); len(names) > 0 {
		if len(names) == 1 {
			clauses = append(clauses, fmt.Sprintf(`priority = "%s"`, names[0]))
		} else {
			clauses = append(clauses, fmt.Sprintf(`priority IN (%s)`, quoteJoin(names)))
		}
	}

	if ids := fb.assigneeDrop.SelectedIDs(); len(ids) > 0 {
		// Separate unassigned from real assignee IDs
		var realIDs []string
		hasUnassigned := false
		for _, id := range ids {
			if id == "unassigned" {
				hasUnassigned = true
			} else {
				realIDs = append(realIDs, id)
			}
		}
		if hasUnassigned && len(realIDs) == 0 {
			clauses = append(clauses, "assignee IS EMPTY")
		} else if hasUnassigned {
			clauses = append(clauses, fmt.Sprintf(`(assignee IN (%s) OR assignee IS EMPTY)`, quoteJoin(realIDs)))
		} else if len(realIDs) == 1 {
			clauses = append(clauses, fmt.Sprintf(`assignee = "%s"`, realIDs[0]))
		} else {
			clauses = append(clauses, fmt.Sprintf(`assignee IN (%s)`, quoteJoin(realIDs)))
		}
	} else if ids := fb.assigneeDrop.RawSelectedIDs(); len(ids) > 0 {
		// Same unassigned handling for raw IDs (items not loaded yet)
		var realIDs []string
		hasUnassigned := false
		for _, id := range ids {
			if id == "unassigned" {
				hasUnassigned = true
			} else {
				realIDs = append(realIDs, id)
			}
		}
		if hasUnassigned && len(realIDs) == 0 {
			clauses = append(clauses, "assignee IS EMPTY")
		} else if hasUnassigned {
			clauses = append(clauses, fmt.Sprintf(`(assignee IN (%s) OR assignee IS EMPTY)`, quoteJoin(realIDs)))
		} else if len(realIDs) == 1 {
			clauses = append(clauses, fmt.Sprintf(`assignee = "%s"`, realIDs[0]))
		} else {
			clauses = append(clauses, fmt.Sprintf(`assignee IN (%s)`, quoteJoin(realIDs)))
		}
	}

	if names := fb.labelsDrop.SelectedLabels(); len(names) > 0 {
		clauses = append(clauses, fmt.Sprintf(`labels IN (%s)`, quoteJoin(names)))
	} else if ids := fb.labelsDrop.RawSelectedIDs(); len(ids) > 0 {
		clauses = append(clauses, fmt.Sprintf(`labels IN (%s)`, quoteJoin(ids)))
	}

	if fb.createdFrom.Value() != nil {
		clauses = append(clauses, fmt.Sprintf(`created >= "%s"`, fb.createdFrom.Value().Format("2006-01-02")))
	}

	if fb.createdUntil.Value() != nil {
		clauses = append(clauses, fmt.Sprintf(`created <= "%s"`, fb.createdUntil.Value().Format("2006-01-02")))
	}

	// Since (relative time filter on updated date)
	if sel := fb.sinceDrop.SelectedItem(); sel != nil && sel.ID != "" {
		clauses = append(clauses, fmt.Sprintf(`updated >= "%s"`, sel.ID))
	}

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

func quoteJoin(items []string) string {
	quoted := make([]string, len(items))
	for i, s := range items {
		quoted[i] = fmt.Sprintf(`"%s"`, s)
	}
	return strings.Join(quoted, ", ")
}

// isToggleKey returns true if the key toggles a multiselect item.
func isToggleKey(msg tea.Msg) bool {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "enter", " ":
			return true
		}
	}
	return false
}

func (fb FilterBar) Update(msg tea.Msg) (FilterBar, tea.Cmd) {
	if !fb.expanded {
		return fb, nil
	}

	// Reset save flash on any user interaction (except the flash tick itself)
	if _, isFlashTick := msg.(filterSaveFlashTickMsg); !isFlashTick {
		if _, isKey := msg.(tea.KeyMsg); isKey {
			fb.saveFlash = 0
		}
	}

	if fb.typeDrop.IsOpen() {
		toggled := isToggleKey(msg)
		var cmd tea.Cmd
		fb.typeDrop, cmd = fb.typeDrop.Update(msg)
		if !fb.typeDrop.IsOpen() || toggled {
			return fb, func() tea.Msg { return filterChangedMsg{} }
		}
		return fb, cmd
	}
	if fb.statusDrop.IsOpen() {
		toggled := isToggleKey(msg)
		var cmd tea.Cmd
		fb.statusDrop, cmd = fb.statusDrop.Update(msg)
		if !fb.statusDrop.IsOpen() || toggled {
			return fb, func() tea.Msg { return filterChangedMsg{} }
		}
		return fb, cmd
	}
	if fb.priorityDrop.IsOpen() {
		toggled := isToggleKey(msg)
		var cmd tea.Cmd
		fb.priorityDrop, cmd = fb.priorityDrop.Update(msg)
		if !fb.priorityDrop.IsOpen() || toggled {
			return fb, func() tea.Msg { return filterChangedMsg{} }
		}
		return fb, cmd
	}
	if fb.assigneeDrop.IsOpen() {
		toggled := isToggleKey(msg)
		var cmd tea.Cmd
		fb.assigneeDrop, cmd = fb.assigneeDrop.Update(msg)
		if !fb.assigneeDrop.IsOpen() || toggled {
			return fb, func() tea.Msg { return filterChangedMsg{} }
		}
		return fb, cmd
	}
	if fb.labelsDrop.IsOpen() {
		toggled := isToggleKey(msg)
		var cmd tea.Cmd
		fb.labelsDrop, cmd = fb.labelsDrop.Update(msg)
		if !fb.labelsDrop.IsOpen() || toggled {
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
	if fb.sinceDrop.IsOpen() {
		prevValue := fb.sinceDrop.Value()
		var cmd tea.Cmd
		fb.sinceDrop, cmd = fb.sinceDrop.Update(msg)
		if !fb.sinceDrop.IsOpen() && fb.sinceDrop.Value() != prevValue {
			return fb, func() tea.Msg { return filterChangedMsg{} }
		}
		if !fb.sinceDrop.IsOpen() {
			return fb, nil
		}
		return fb, cmd
	}
	if fb.sortDrop.IsOpen() {
		prevValue := fb.sortDrop.Value()
		var cmd tea.Cmd
		fb.sortDrop, cmd = fb.sortDrop.Update(msg)
		if !fb.sortDrop.IsOpen() && fb.sortDrop.Value() != prevValue {
			return fb, func() tea.Msg { return sortChangedMsg{} }
		}
		if !fb.sortDrop.IsOpen() {
			return fb, nil
		}
		return fb, cmd
	}

	// Delete confirmation
	if fb.confirmDelete {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "y":
				name := fb.confirmDelName
				fb.confirmDelete = false
				fb.confirmDelName = ""
				delete(fb.savedViews, name)
				if fb.activeView == name {
					fb.activeView = ""
				}
				// Update load dropdown
				items := make([]DropdownItem, 0, len(fb.savedViews))
				for n := range fb.savedViews {
					items = append(items, DropdownItem{ID: n, Label: n})
				}
				fb.loadDrop.SetItems(items)
				return fb, func() tea.Msg { return filterDeleteViewMsg{name: name} }
			case "n", "esc":
				fb.confirmDelete = false
				fb.confirmDelName = ""
				return fb, nil
			}
		}
		return fb, nil
	}

	// Save name input
	if fb.saveNaming {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc":
				fb.saveNaming = false
				fb.saveInput.Blur()
				return fb, nil
			case "enter":
				name := strings.TrimSpace(fb.saveInput.Value())
				if name == "" {
					name = "Default"
				}
				fb.saveNaming = false
				fb.saveInput.Blur()
				fb.activeView = name
				fb.saveFlash = 1
				saveCmd := func() tea.Msg { return filterSaveMsg{} }
				flashCmd := tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
					return filterSaveFlashTickMsg{}
				})
				return fb, tea.Batch(saveCmd, flashCmd)
			default:
				var cmd tea.Cmd
				fb.saveInput, cmd = fb.saveInput.Update(msg)
				return fb, cmd
			}
		case tea.MouseMsg:
			// Any click closes the naming input
			if msg.Action == tea.MouseActionPress {
				fb.saveNaming = false
				fb.saveInput.Blur()
			}
			return fb, nil
		default:
			var cmd tea.Cmd
			fb.saveInput, cmd = fb.saveInput.Update(msg)
			return fb, cmd
		}
	}

	// Load dropdown
	if fb.loadDrop.IsOpen() {
		wasOpen := true
		var cmd tea.Cmd
		fb.loadDrop, cmd = fb.loadDrop.Update(msg)
		if wasOpen && !fb.loadDrop.IsOpen() {
			sel := fb.loadDrop.SelectedItem()
			if sel != nil {
				fb.clearFiltersOnly()
				if sf, ok := fb.savedViews[sel.ID]; ok {
					fb.RestoreFilters(sf)
				}
				fb.activeView = sel.ID
				return fb, func() tea.Msg { return filterChangedMsg{} }
			}
			return fb, nil
		}
		return fb, cmd
	}

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
				fb.searchSeq++
				seq := fb.searchSeq
				tickCmd := tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
					return filterSearchTick{seq: seq}
				})
				return fb, tea.Batch(cmd, tickCmd)
			}
		case filterSearchTick:
			if msg.seq == fb.searchSeq && len(fb.search.Value()) >= 3 {
				return fb, func() tea.Msg { return filterChangedMsg{} }
			}
			return fb, nil
		default:
			var cmd tea.Cmd
			fb.search, cmd = fb.search.Update(msg)
			return fb, cmd
		}
	}

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
		case "p":
			fb.closeAll()
			fb.priorityDrop.Open()
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
		case "o":
			fb.closeAll()
			return fb, fb.sortDrop.OpenDropdown()
		case "O":
			fb.ToggleSortDirection()
			return fb, func() tea.Msg { return sortChangedMsg{} }
		case "x":
			return fb, fb.ClearAll()
		case "esc":
			fb.Collapse()
			return fb, nil
		}

	case filterSaveFlashTickMsg:
		fb.saveFlash = 0
		return fb, nil

	case filterSearchTick:
		if msg.seq == fb.searchSeq {
			return fb, func() tea.Msg { return filterChangedMsg{} }
		}
		return fb, nil
	}

	return fb, nil
}

// HandleFieldClick handles a mouse click on the expanded filter bar.
// x, y are coordinates relative to the filter bar top-left.
func (fb *FilterBar) HandleFieldClick(x, y int) tea.Cmd {
	if !fb.expanded {
		return nil
	}
	fb.saveFlash = 0

	// y=0 is the header line — click toggles collapse
	if y == 0 {
		fb.Collapse()
		return nil
	}

	// Check if click is on the action row (after divider, last 3 lines)
	actionRowY := 1 + fb.numRows()*3 + 1 // after header + fields + divider
	if y >= actionRowY {
		return fb.HandleActionClick(x)
	}

	// Determine which row was clicked (each row is 3 lines tall)
	row := (y - 1) / 3
	fpr := fb.fieldsPerRow()
	fieldW := fb.fieldRowWidth(fpr)
	col := x / fieldW

	// Calculate the field index
	idx := row*fpr + col
	if idx >= fieldCount {
		return nil
	}

	return fb.activateField(idx)
}

// activateField opens/activates the field at the given index.
func (fb *FilterBar) activateField(idx int) tea.Cmd {
	fb.closeAll()
	switch idx {
	case fieldType:
		fb.typeDrop.Open()
	case fieldStatus:
		fb.statusDrop.Open()
	case fieldPriority:
		fb.priorityDrop.Open()
	case fieldAssignee:
		fb.assigneeDrop.Open()
	case fieldLabels:
		fb.labelsDrop.Open()
	case fieldFrom:
		fb.createdFrom.OpenPicker()
	case fieldUntil:
		fb.createdUntil.OpenPicker()
	case fieldSearch:
		fb.searchActive = true
		fb.search.Focus()
		return fb.search.Cursor.BlinkCmd()
	case fieldSince:
		return fb.sinceDrop.OpenDropdown()
	case fieldSortBy:
		return fb.sortDrop.OpenDropdown()
	case fieldDirection:
		fb.ToggleSortDirection()
		return func() tea.Msg { return sortChangedMsg{} }
	case fieldClaude:
		fb.claudeFilter = !fb.claudeFilter
		return func() tea.Msg { return filterChangedMsg{} }
	}
	return nil
}

func (fb FilterBar) View() string {
	if !fb.expanded {
		return fb.renderCollapsed()
	}
	return fb.renderExpanded()
}

func (fb FilterBar) renderCollapsed() string {
	style := lipgloss.NewStyle().Foreground(colorAccent)
	hint := lipgloss.NewStyle().Foreground(colorSubtle)
	valStyle := lipgloss.NewStyle().Foreground(colorSuccess)
	sep := lipgloss.NewStyle().Foreground(colorSubtle).Render(" · ")
	sortStyle := lipgloss.NewStyle().Foreground(colorInfo)

	// Build filter parts
	var parts []string
	if fb.typeDrop.HasSelection() {
		parts = append(parts, "Type="+fb.typeDrop.ValueText())
	}
	if fb.statusDrop.HasSelection() {
		parts = append(parts, "Status="+fb.statusDrop.ValueText())
	}
	if fb.priorityDrop.HasSelection() {
		parts = append(parts, "Priority="+fb.priorityDrop.ValueText())
	}
	if fb.assigneeDrop.HasSelection() {
		ids := fb.assigneeDrop.SelectedIDs()
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

	if sel := fb.sinceDrop.SelectedItem(); sel != nil && sel.ID != "" {
		parts = append(parts, "Since="+sel.Label)
	}

	// Build sort info
	dirIcon := "↓"
	if fb.sortAsc {
		dirIcon = "↑"
	}
	sortInfo := sortStyle.Render("Sort:" + fb.SortLabel() + " " + dirIcon)

	// Assemble all segments for wrapping
	var segments []string
	for _, part := range parts {
		segments = append(segments, valStyle.Render(part))
	}
	segments = append(segments, sortInfo)

	// Build with wrapping
	prefix := style.Render("▶ Filters: ")
	suffix := " " + hint.Render("(f)")
	maxW := fb.width
	if maxW <= 0 {
		maxW = 200
	}

	var lines []string
	line := prefix
	for i, seg := range segments {
		addition := seg
		if i > 0 {
			addition = sep + seg
		}
		candidate := line + addition
		if lipgloss.Width(candidate+suffix) > maxW && lipgloss.Width(line) > lipgloss.Width(prefix) {
			// Wrap to next line
			lines = append(lines, line)
			line = "  " + seg // indent continuation
		} else {
			line = line + addition
		}
	}
	line += suffix
	lines = append(lines, line)

	return strings.Join(lines, "\n")
}

func (fb FilterBar) renderExpanded() string {
	var b strings.Builder

	style := lipgloss.NewStyle().Foreground(colorAccent)
	hint := lipgloss.NewStyle().Foreground(colorSubtle)
	b.WriteString(style.Render("▼ Filters") + " " + hint.Render("(esc to collapse)"))

	fpr := fb.fieldsPerRow()
	fieldW := fb.fieldRowWidth(fpr)

	// Set widths on all components
	fb.typeDrop.SetWidth(fieldW)
	fb.statusDrop.SetWidth(fieldW)
	fb.priorityDrop.SetWidth(fieldW)
	fb.assigneeDrop.SetWidth(fieldW)
	fb.labelsDrop.SetWidth(fieldW)
	fb.createdFrom.width = fieldW
	fb.createdUntil.width = fieldW
	fb.sortDrop.width = fieldW

	// Render all fields in order (no save button — it's in the action row)
	allFields := []string{
		fb.typeDrop.View(),
		fb.statusDrop.View(),
		fb.priorityDrop.View(),
		fb.assigneeDrop.View(),
		fb.labelsDrop.View(),
		fb.createdFrom.View(),
		fb.createdUntil.View(),
		fb.renderSearchField(fieldW),
		fb.sinceDrop.View(),
		fb.sortDrop.View(),
		fb.renderDirToggle(fieldW),
		fb.renderClaudeToggle(fieldW),
	}

	// Split into rows of fpr fields each
	for i := 0; i < len(allFields); i += fpr {
		end := i + fpr
		if end > len(allFields) {
			end = len(allFields)
		}
		b.WriteString("\n")
		b.WriteString(joinFieldsHorizontal(allFields[i:end]...))
	}

	// Action row + Divider
	b.WriteString("\n")
	b.WriteString(fb.renderActionRow())
	divStyle := lipgloss.NewStyle().Foreground(colorBorder)
	b.WriteString("\n")
	b.WriteString(divStyle.Render(strings.Repeat("─", fb.width)))

	return b.String()
}

func (fb FilterBar) renderDirToggle(width int) string {
	innerW := width - 2
	valW := innerW - 2

	lbl := lipgloss.NewStyle().Foreground(colorInfo)
	bdr := lipgloss.NewStyle().Foreground(colorBorder)

	labelText := " Direction "
	dashes := innerW - lipgloss.Width(labelText) - 1
	if dashes < 0 {
		dashes = 0
	}

	top := bdr.Render("╭─") + lbl.Render(labelText) + bdr.Render(strings.Repeat("─", dashes)+"╮")

	var icon string
	if fb.sortAsc {
		icon = lipgloss.NewStyle().Foreground(colorInfo).Render("↑ ASC")
	} else {
		icon = lipgloss.NewStyle().Foreground(colorInfo).Render("↓ DESC")
	}
	pad := valW - lipgloss.Width(icon)
	if pad < 0 {
		pad = 0
	}
	mid := bdr.Render("│") + " " + icon + strings.Repeat(" ", pad) + " " + bdr.Render("│")
	bot := bdr.Render("╰" + strings.Repeat("─", innerW) + "╯")

	return top + "\n" + mid + "\n" + bot
}

func (fb FilterBar) renderClaudeToggle(width int) string {
	innerW := width - 2
	valW := innerW - 2

	lbl := lipgloss.NewStyle().Foreground(colorPurple)
	bdr := lipgloss.NewStyle().Foreground(colorBorder)

	labelText := " Claude "
	dashes := innerW - lipgloss.Width(labelText) - 1
	if dashes < 0 {
		dashes = 0
	}

	top := bdr.Render("╭─") + lbl.Render(labelText) + bdr.Render(strings.Repeat("─", dashes)+"╮")

	var icon string
	if fb.claudeFilter {
		icon = lipgloss.NewStyle().Foreground(colorPurple).Render("● Active")
	} else {
		icon = lipgloss.NewStyle().Foreground(colorSubtle).Render("○ All")
	}
	pad := valW - lipgloss.Width(icon)
	if pad < 0 {
		pad = 0
	}
	mid := bdr.Render("│") + " " + icon + strings.Repeat(" ", pad) + " " + bdr.Render("│")
	bot := bdr.Render("╰" + strings.Repeat("─", innerW) + "╯")

	return top + "\n" + mid + "\n" + bot
}

// ClaudeFilter returns whether the Claude filter is active.
func (fb FilterBar) ClaudeFilter() bool { return fb.claudeFilter }

// SetSavedViews updates the available saved view names for the load dropdown.
// The Default view always appears first.
func (fb *FilterBar) SetSavedViews(views map[string]config.SavedFilters, activeView string) {
	fb.savedViews = views
	fb.activeView = activeView
	fb.rebuildLoadDropItems()
}

// rebuildLoadDropItems rebuilds the load dropdown items, ensuring Default is always first.
func (fb *FilterBar) rebuildLoadDropItems() {
	items := []DropdownItem{{ID: DefaultViewName, Label: DefaultViewName}}
	for name := range fb.savedViews {
		if name != DefaultViewName {
			items = append(items, DropdownItem{ID: name, Label: name})
		}
	}
	fb.loadDrop.SetItems(items)
}

// actionButtonWidth returns the width for action buttons.
// When the Add input is active, returns a wider width for the text field.
func (fb FilterBar) actionButtonWidth() int {
	if fb.saveNaming {
		return 25
	}
	return 6 // border(1) + pad(1) + content(2) + pad(1) + border(1)
}

// actionLoadWidth returns the static width for the Load dropdown.
func (fb FilterBar) actionLoadWidth() int {
	return 40
}

func (fb FilterBar) renderActionRow() string {
	// Delete confirmation takes over the row
	if fb.confirmDelete {
		warnStyle := lipgloss.NewStyle().Foreground(colorError).Bold(true)
		hintStyle := lipgloss.NewStyle().Foreground(colorSubtle)
		return " " + warnStyle.Render("Delete \""+fb.confirmDelName+"\"? ") +
			hintStyle.Render("(y)es / (n)o")
	}

	loadW := fb.actionLoadWidth()

	// Load dropdown / new view name input
	fb.loadDrop.width = loadW
	var loadField string
	if fb.saveNaming {
		fb.saveInput.Width = loadW - 6 // account for borders + label padding
		loadField = fb.renderActionField("View Name", fb.saveInput.View(), loadW, colorAccent)
	} else {
		loadField = fb.loadDrop.View()
	}

	saveBg := lipgloss.Color("#1a2e1a") // dark green for flash fill

	// Save button (saves current view without prompting)
	var saveField string
	if fb.saveFlash > 0 {
		saveField = fb.renderActionButton("✓", colorSuccess, colorSuccess, true, saveBg)
	} else {
		saveField = fb.renderActionButton("💾", colorText, colorSuccess, false, saveBg)
	}

	// Add button
	addField := fb.renderActionButton("＋", colorText, colorAccent, false, colorBackground)

	// Delete button (disabled for Default view, moved to far right)
	var deleteField string
	if fb.activeView != "" && fb.activeView != DefaultViewName {
		deleteField = fb.renderActionButton("🗑", colorText, colorError, false, colorBackground)
	} else {
		deleteField = fb.renderActionButton("🗑", colorSubtle, colorSubtle, false, colorBackground)
	}

	return joinFieldsHorizontal(loadField, addField, saveField, deleteField)
}

// renderActionButton renders a bordered icon button.
// If filled is true, the interior gets the bg color.
func (fb FilterBar) renderActionButton(icon string, fg, borderColor lipgloss.Color, filled bool, bg lipgloss.Color) string {
	innerW := 4
	valW := innerW - 2

	bdr := lipgloss.NewStyle().Foreground(borderColor)

	top := bdr.Render("╭" + strings.Repeat("─", innerW) + "╮")

	rendered := lipgloss.NewStyle().Foreground(fg).Render(icon)
	iconW := lipgloss.Width(rendered)
	padLeft := (valW - iconW) / 2
	padRight := valW - iconW - padLeft
	if padRight < 0 {
		padRight = 0
	}

	var mid string
	if filled {
		bgStyle := lipgloss.NewStyle().Background(bg)
		renderedBg := lipgloss.NewStyle().Foreground(fg).Background(bg).Render(icon)
		mid = bdr.Render("│") +
			bgStyle.Render(" "+strings.Repeat(" ", padLeft)) +
			renderedBg +
			bgStyle.Render(strings.Repeat(" ", padRight)+" ") +
			bdr.Render("│")
	} else {
		mid = bdr.Render("│") + " " + strings.Repeat(" ", padLeft) + rendered + strings.Repeat(" ", padRight) + " " + bdr.Render("│")
	}

	bot := bdr.Render("╰" + strings.Repeat("─", innerW) + "╯")
	return top + "\n" + mid + "\n" + bot
}

// renderActionField renders a bordered button-like field (used for save input).
func (fb FilterBar) renderActionField(label, content string, width int, labelColor lipgloss.Color) string {
	innerW := width - 2
	valW := innerW - 2

	lbl := lipgloss.NewStyle().Foreground(labelColor)
	bdr := lipgloss.NewStyle().Foreground(colorBorder)

	top := bdr.Render("╭" + strings.Repeat("─", innerW) + "╮")

	var mid string
	if content != "" {
		visW := lipgloss.Width(content)
		pad := valW - visW
		if pad < 0 {
			pad = 0
		}
		mid = bdr.Render("│") + " " + content + strings.Repeat(" ", pad) + " " + bdr.Render("│")
	} else {
		rendered := lbl.Render(label)
		labelW := lipgloss.Width(rendered)
		padLeft := (valW - labelW) / 2
		if padLeft < 0 {
			padLeft = 0
		}
		padRight := valW - labelW - padLeft
		if padRight < 0 {
			padRight = 0
		}
		mid = bdr.Render("│") + " " + strings.Repeat(" ", padLeft) + rendered + strings.Repeat(" ", padRight) + " " + bdr.Render("│")
	}

	bot := bdr.Render("╰" + strings.Repeat("─", innerW) + "╯")
	return top + "\n" + mid + "\n" + bot
}

// HandleActionClick handles a click on the action row.
// x is the click X position, returns a tea.Cmd.
func (fb *FilterBar) HandleActionClick(x int) tea.Cmd {
	loadW := fb.actionLoadWidth()
	btnW := fb.actionButtonWidth()

	if x < loadW {
		// Load (always has at least the Default entry)
		fb.closeAll()
		return fb.loadDrop.OpenDropdown()
	}
	idx := (x - loadW) / btnW
	switch idx {
	case 0: // Add (prompt for new view name)
		fb.closeAll()
		fb.saveNaming = true
		fb.saveInput.SetValue("")
		fb.saveInput.Focus()
		return fb.saveInput.Cursor.BlinkCmd()
	case 1: // Save (immediate save to current view)
		if fb.activeView != "" {
			fb.closeAll()
			fb.saveFlash = 1
			saveCmd := func() tea.Msg { return filterSaveMsg{} }
			flashCmd := tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
				return filterSaveFlashTickMsg{}
			})
			return tea.Batch(saveCmd, flashCmd)
		}
		return nil
	case 2: // Delete
		if fb.activeView != "" && fb.activeView != DefaultViewName {
			fb.confirmDelete = true
			fb.confirmDelName = fb.activeView
		}
		return nil
	}
	return nil
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

// OverlayLines returns any open dropdown/picker overlay lines with their
// position info. startLine is relative to filterBar top (absolute screen Y).
func (fb FilterBar) OverlayLines() (lines []string, startLine, startCol int) {
	fpr := fb.fieldsPerRow()
	fieldW := fb.fieldRowWidth(fpr)

	// Helper: given a field index and its MultiSelect, return the startLine and startCol.
	// If the overlay has a connector (wider than field), start 1 line earlier to overlap ├──┤.
	pos := func(idx int, ms *MultiSelect) (int, int) {
		row, col := fb.fieldPosition(idx)
		sl := 1 + (row+1)*3
		if ms != nil && ms.overlayWidth > 0 && ms.overlayWidth > ms.width {
			sl-- // connector line overlaps the field's bottom border
		}
		sc := col * fieldW
		return sl, sc
	}

	if fb.typeDrop.IsOpen() {
		sl, sc := pos(fieldType, &fb.typeDrop)
		return fb.typeDrop.RenderOverlay(), sl, sc
	}
	if fb.statusDrop.IsOpen() {
		sl, sc := pos(fieldStatus, &fb.statusDrop)
		return fb.statusDrop.RenderOverlay(), sl, sc
	}
	if fb.priorityDrop.IsOpen() {
		sl, sc := pos(fieldPriority, &fb.priorityDrop)
		return fb.priorityDrop.RenderOverlay(), sl, sc
	}
	if fb.assigneeDrop.IsOpen() {
		sl, sc := pos(fieldAssignee, &fb.assigneeDrop)
		return fb.assigneeDrop.RenderOverlay(), sl, sc
	}
	if fb.labelsDrop.IsOpen() {
		sl, sc := pos(fieldLabels, &fb.labelsDrop)
		return fb.labelsDrop.RenderOverlay(), sl, sc
	}
	if fb.createdFrom.IsOpen() {
		row, col := fb.fieldPosition(fieldFrom)
		sl := 1 + (row+1)*3
		if fb.createdFrom.calWidth() > fb.createdFrom.width {
			sl--
		}
		return fb.createdFrom.RenderOverlay(), sl, col * fieldW
	}
	if fb.createdUntil.IsOpen() {
		row, col := fb.fieldPosition(fieldUntil)
		sl := 1 + (row+1)*3
		if fb.createdUntil.calWidth() > fb.createdUntil.width {
			sl--
		}
		return fb.createdUntil.RenderOverlay(), sl, col * fieldW
	}
	if fb.sinceDrop.IsOpen() {
		sl, sc := pos(fieldSince, nil)
		return fb.sinceDrop.RenderOverlay(), sl, sc
	}
	if fb.sortDrop.IsOpen() {
		sl, sc := pos(fieldSortBy, nil)
		return fb.sortDrop.RenderOverlay(), sl, sc
	}
	// Load dropdown overlay appears directly below the Load field, overlapping the divider
	if fb.loadDrop.IsOpen() {
		actionRowY := 1 + fb.numRows()*3 + 1
		return fb.loadDrop.RenderOverlay(), actionRowY + 2, 0
	}

	return nil, 0, 0
}

// GetSavedFilters returns the current filter state for persistence.
func (fb FilterBar) GetSavedFilters() config.SavedFilters {
	// Use SelectedIDs when items are loaded, fall back to RawSelectedIDs
	typeIDs := fb.typeDrop.SelectedIDs()
	if len(typeIDs) == 0 {
		typeIDs = fb.typeDrop.RawSelectedIDs()
	}
	statusIDs := fb.statusDrop.SelectedIDs()
	if len(statusIDs) == 0 {
		statusIDs = fb.statusDrop.RawSelectedIDs()
	}
	priorityIDs := fb.priorityDrop.SelectedIDs()
	if len(priorityIDs) == 0 {
		priorityIDs = fb.priorityDrop.RawSelectedIDs()
	}
	assigneeIDs := fb.assigneeDrop.SelectedIDs()
	if len(assigneeIDs) == 0 {
		assigneeIDs = fb.assigneeDrop.RawSelectedIDs()
	}
	labelIDs := fb.labelsDrop.SelectedIDs()
	if len(labelIDs) == 0 {
		labelIDs = fb.labelsDrop.RawSelectedIDs()
	}
	sf := config.SavedFilters{
		TypeIDs:     typeIDs,
		StatusIDs:   statusIDs,
		PriorityIDs: priorityIDs,
		AssigneeIDs: assigneeIDs,
		LabelIDs:    labelIDs,
		SearchText:  fb.search.Value(),
		Since:       fb.sinceValue(),
		SortField:   fb.SortField(),
		SortAsc:     fb.sortAsc,
	}
	if fb.createdFrom.Value() != nil {
		sf.CreatedFrom = fb.createdFrom.Value().Format("2006-01-02")
	}
	if fb.createdUntil.Value() != nil {
		sf.CreatedUntil = fb.createdUntil.Value().Format("2006-01-02")
	}
	return sf
}

// RestoreFilters applies a saved filter state to the dropdowns.
// Items must already be populated for ID-based selections to take effect.
func (fb *FilterBar) RestoreFilters(sf config.SavedFilters) {
	// Clear and restore each field. We clear first so that defaults
	// (like "me" as assignee) don't persist when the saved view
	// doesn't include them.
	fb.typeDrop.selected = make(map[string]bool)
	for _, id := range sf.TypeIDs {
		fb.typeDrop.selected[id] = true
	}
	fb.statusDrop.selected = make(map[string]bool)
	for _, id := range sf.StatusIDs {
		fb.statusDrop.selected[id] = true
	}
	fb.priorityDrop.selected = make(map[string]bool)
	for _, id := range sf.PriorityIDs {
		fb.priorityDrop.selected[id] = true
	}
	fb.assigneeDrop.selected = make(map[string]bool)
	for _, id := range sf.AssigneeIDs {
		fb.assigneeDrop.selected[id] = true
	}
	fb.labelsDrop.selected = make(map[string]bool)
	for _, id := range sf.LabelIDs {
		fb.labelsDrop.selected[id] = true
	}
	if sf.CreatedFrom != "" {
		if t, err := time.Parse("2006-01-02", sf.CreatedFrom); err == nil {
			fb.createdFrom = NewDatePicker("Created From", &t, fb.createdFrom.width)
		}
	}
	if sf.CreatedUntil != "" {
		if t, err := time.Parse("2006-01-02", sf.CreatedUntil); err == nil {
			fb.createdUntil = NewDatePicker("Created Until", &t, fb.createdUntil.width)
		}
	}
	if sf.SearchText != "" {
		fb.search.SetValue(sf.SearchText)
	}
	if sf.Since != "" {
		for i, item := range sinceOptions {
			if item.ID == sf.Since {
				fb.sinceDrop.value = item.Label
				fb.sinceDrop.selected = i
				break
			}
		}
	}
	if sf.SortField != "" {
		fb.sortAsc = sf.SortAsc
		for i, item := range sortFields {
			if item.ID == sf.SortField {
				fb.sortDrop.value = item.Label
				fb.sortDrop.selected = i
				break
			}
		}
	}
}
