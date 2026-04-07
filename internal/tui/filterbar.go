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

type filterSearchTick struct {
	seq int
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
	sortDrop     Dropdown
	sortAsc      bool
	width        int

	defaultAssigneeID   string
	defaultAssigneeName string
}

func NewFilterBar(width int) FilterBar {
	ti := textinput.New()
	ti.Placeholder = "Filter by key or summary..."
	ti.CharLimit = 200
	ti.Prompt = ""

	fieldW := fieldWidth(width, 5)

	return FilterBar{
		typeDrop:     NewMultiSelect("Issue Type", nil, fieldW),
		statusDrop:   NewMultiSelect("Status", nil, fieldW),
		priorityDrop: NewMultiSelect("Priority", nil, fieldW),
		assigneeDrop: NewMultiSelect("Assignee", nil, fieldW),
		labelsDrop:   NewMultiSelect("Labels", nil, fieldW),
		createdFrom:  NewDatePicker("Created From", nil, 18),
		createdUntil: NewDatePicker("Created Until", nil, 18),
		sortDrop:     NewSimpleDropdown("Sort By", sortFields, "Updated", "updated", 18),
		search:       ti,
		width:        width,
	}
}

func fieldWidth(totalW, count int) int {
	w := (totalW - (count - 1)) / count
	if w < 16 {
		w = 16
	}
	return w
}

// fieldCount is the total number of fields in the expanded filter bar.
const fieldCount = 11

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
	fieldSortBy    = 8
	fieldDirection = 9
	fieldSave      = 10
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
	fb.sortDrop.width = fieldW
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
func (fb *FilterBar) SetAssigneeItems(items []DropdownItem) { fb.assigneeDrop.SetItemsKeepSelection(items) }
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
	if len(fb.typeDrop.SelectedIDs()) > 0 {
		return true
	}
	if len(fb.statusDrop.SelectedIDs()) > 0 {
		return true
	}
	if fb.defaultAssigneeID != "" {
		ids := fb.assigneeDrop.SelectedIDs()
		if len(ids) != 1 || ids[0] != fb.defaultAssigneeID {
			if len(ids) > 0 {
				return true
			}
		}
	} else if len(fb.assigneeDrop.SelectedIDs()) > 0 {
		return true
	}
	if len(fb.priorityDrop.SelectedIDs()) > 0 {
		return true
	}
	if len(fb.labelsDrop.SelectedIDs()) > 0 {
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
		fb.createdUntil.IsOpen() || fb.sortDrop.IsOpen() ||
		fb.searchActive
}

func (fb *FilterBar) closeAll() {
	fb.typeDrop.Close()
	fb.statusDrop.Close()
	fb.priorityDrop.Close()
	fb.assigneeDrop.Close()
	fb.labelsDrop.Close()
	fb.createdFrom.Close()
	fb.createdUntil.Close()
	fb.sortDrop.Close()
	fb.searchActive = false
	fb.search.Blur()
}

func (fb *FilterBar) ClearAll() tea.Cmd {
	fb.typeDrop.Clear()
	fb.statusDrop.Clear()
	fb.priorityDrop.Clear()
	fb.assigneeDrop.Clear()
	fb.labelsDrop.Clear()
	fb.createdFrom = NewDatePicker("Created From", nil, 18)
	fb.createdUntil = NewDatePicker("Created Until", nil, 18)
	fb.search.SetValue("")
	fb.searchActive = false
	fb.search.Blur()
	return func() tea.Msg { return filterChangedMsg{} }
}

func (fb FilterBar) Height() int {
	if !fb.expanded {
		return strings.Count(fb.renderCollapsed(), "\n") + 1
	}
	return 1 + fb.numRows()*3 // header + 3 lines per field row
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

	if names := fb.statusDrop.SelectedLabels(); len(names) > 0 {
		if len(names) == 1 {
			clauses = append(clauses, fmt.Sprintf(`status = "%s"`, names[0]))
		} else {
			clauses = append(clauses, fmt.Sprintf(`status IN (%s)`, quoteJoin(names)))
		}
	} else {
		clauses = append(clauses, "statusCategory != Done")
	}

	if names := fb.priorityDrop.SelectedLabels(); len(names) > 0 {
		if len(names) == 1 {
			clauses = append(clauses, fmt.Sprintf(`priority = "%s"`, names[0]))
		} else {
			clauses = append(clauses, fmt.Sprintf(`priority IN (%s)`, quoteJoin(names)))
		}
	}

	if ids := fb.assigneeDrop.SelectedIDs(); len(ids) > 0 {
		if len(ids) == 1 {
			clauses = append(clauses, fmt.Sprintf(`assignee = "%s"`, ids[0]))
		} else {
			clauses = append(clauses, fmt.Sprintf(`assignee IN (%s)`, quoteJoin(ids)))
		}
	}

	if names := fb.labelsDrop.SelectedLabels(); len(names) > 0 {
		clauses = append(clauses, fmt.Sprintf(`labels IN (%s)`, quoteJoin(names)))
	}

	if fb.createdFrom.Value() != nil {
		clauses = append(clauses, fmt.Sprintf(`created >= "%s"`, fb.createdFrom.Value().Format("2006-01-02")))
	}

	if fb.createdUntil.Value() != nil {
		clauses = append(clauses, fmt.Sprintf(`created <= "%s"`, fb.createdUntil.Value().Format("2006-01-02")))
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
		case "S":
			return fb, func() tea.Msg { return filterSaveMsg{} }
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

// HandleFieldClick handles a mouse click on the expanded filter bar.
// x, y are coordinates relative to the filter bar top-left.
func (fb *FilterBar) HandleFieldClick(x, y int) tea.Cmd {
	if !fb.expanded {
		return nil
	}

	// y=0 is the header line — click toggles collapse
	if y == 0 {
		fb.Collapse()
		return nil
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
	case fieldSortBy:
		return fb.sortDrop.OpenDropdown()
	case fieldDirection:
		fb.ToggleSortDirection()
		return func() tea.Msg { return sortChangedMsg{} }
	case fieldSave:
		return func() tea.Msg { return filterSaveMsg{} }
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

	// Render all fields in order
	allFields := []string{
		fb.typeDrop.View(),
		fb.statusDrop.View(),
		fb.priorityDrop.View(),
		fb.assigneeDrop.View(),
		fb.labelsDrop.View(),
		fb.createdFrom.View(),
		fb.createdUntil.View(),
		fb.renderSearchField(fieldW),
		fb.sortDrop.View(),
		fb.renderDirToggle(fieldW),
		fb.renderSaveButton(fieldW),
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

func (fb FilterBar) renderSaveButton(width int) string {
	innerW := width - 2
	valW := innerW - 2

	lbl := lipgloss.NewStyle().Foreground(colorSuccess)
	bdr := lipgloss.NewStyle().Foreground(colorBorder)

	labelText := " Save "
	dashes := innerW - lipgloss.Width(labelText) - 1
	if dashes < 0 {
		dashes = 0
	}

	top := bdr.Render("╭─") + lbl.Render(labelText) + bdr.Render(strings.Repeat("─", dashes)+"╮")

	icon := lipgloss.NewStyle().Foreground(colorSuccess).Render("S")
	pad := valW - lipgloss.Width(icon)
	if pad < 0 {
		pad = 0
	}
	mid := bdr.Render("│") + " " + icon + strings.Repeat(" ", pad) + " " + bdr.Render("│")
	bot := bdr.Render("╰" + strings.Repeat("─", innerW) + "╯")

	return top + "\n" + mid + "\n" + bot
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

	// Helper: given a field index, return the startLine and startCol
	pos := func(idx int) (int, int) {
		row, col := fb.fieldPosition(idx)
		// startLine = header(1) + row * 3 lines + 3 (below the field box)
		sl := 1 + (row+1)*3
		sc := col * fieldW
		return sl, sc
	}

	if fb.typeDrop.IsOpen() {
		sl, sc := pos(fieldType)
		return fb.typeDrop.RenderOverlay(), sl, sc
	}
	if fb.statusDrop.IsOpen() {
		sl, sc := pos(fieldStatus)
		return fb.statusDrop.RenderOverlay(), sl, sc
	}
	if fb.priorityDrop.IsOpen() {
		sl, sc := pos(fieldPriority)
		return fb.priorityDrop.RenderOverlay(), sl, sc
	}
	if fb.assigneeDrop.IsOpen() {
		sl, sc := pos(fieldAssignee)
		return fb.assigneeDrop.RenderOverlay(), sl, sc
	}
	if fb.labelsDrop.IsOpen() {
		sl, sc := pos(fieldLabels)
		return fb.labelsDrop.RenderOverlay(), sl, sc
	}
	if fb.createdFrom.IsOpen() {
		sl, sc := pos(fieldFrom)
		return fb.createdFrom.RenderOverlay(), sl, sc
	}
	if fb.createdUntil.IsOpen() {
		sl, sc := pos(fieldUntil)
		return fb.createdUntil.RenderOverlay(), sl, sc
	}
	if fb.sortDrop.IsOpen() {
		sl, sc := pos(fieldSortBy)
		return fb.sortDrop.RenderOverlay(), sl, sc
	}

	return nil, 0, 0
}

// GetSavedFilters returns the current filter state for persistence.
func (fb FilterBar) GetSavedFilters() config.SavedFilters {
	sf := config.SavedFilters{
		TypeIDs:     fb.typeDrop.SelectedIDs(),
		StatusIDs:   fb.statusDrop.SelectedIDs(),
		PriorityIDs: fb.priorityDrop.SelectedIDs(),
		AssigneeIDs: fb.assigneeDrop.SelectedIDs(),
		LabelIDs:    fb.labelsDrop.SelectedIDs(),
		SearchText:  fb.search.Value(),
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
	for _, id := range sf.TypeIDs {
		fb.typeDrop.selected[id] = true
	}
	for _, id := range sf.StatusIDs {
		fb.statusDrop.selected[id] = true
	}
	for _, id := range sf.PriorityIDs {
		fb.priorityDrop.selected[id] = true
	}
	for _, id := range sf.AssigneeIDs {
		fb.assigneeDrop.selected[id] = true
	}
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
}
