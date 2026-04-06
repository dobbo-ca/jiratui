package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type filterChangedMsg struct{}

type filterSearchTick struct {
	seq int
}

type FilterBar struct {
	expanded     bool
	typeDrop     MultiSelect
	statusDrop   MultiSelect
	assigneeDrop MultiSelect
	labelsDrop   MultiSelect
	createdFrom  DatePicker
	createdUntil DatePicker
	search       textinput.Model
	searchSeq    int
	searchActive bool
	width        int

	defaultAssigneeID   string
	defaultAssigneeName string
}

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
	w := (totalW - (count - 1)) / count
	if w < 16 {
		w = 16
	}
	return w
}

func (fb *FilterBar) SetDefaultAssignee(accountID, displayName string) {
	fb.defaultAssigneeID = accountID
	fb.defaultAssigneeName = displayName
	// Ensure the item exists so SelectedIDs() can find it.
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
	fieldW := fieldWidth(w, 4)
	fb.typeDrop.SetWidth(fieldW)
	fb.statusDrop.SetWidth(fieldW)
	fb.assigneeDrop.SetWidth(fieldW)
	fb.labelsDrop.SetWidth(fieldW)
}

func (fb *FilterBar) SetStatusItems(items []DropdownItem)   { fb.statusDrop.SetItems(items) }
func (fb *FilterBar) SetTypeItems(items []DropdownItem)     { fb.typeDrop.SetItems(items) }
func (fb *FilterBar) SetAssigneeItems(items []DropdownItem) { fb.assigneeDrop.SetItems(items) }
func (fb *FilterBar) SetLabelItems(items []DropdownItem)    { fb.labelsDrop.SetItems(items) }

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
	if fb.typeDrop.HasSelection() {
		return true
	}
	if fb.statusDrop.HasSelection() {
		return true
	}
	if fb.defaultAssigneeID != "" {
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
	return func() tea.Msg { return filterChangedMsg{} }
}

func (fb FilterBar) Height() int {
	if !fb.expanded {
		return 1
	}
	return 8
}

func (fb FilterBar) BuildJQL(projectKey, orderBy string) string {
	var clauses []string

	if projectKey != "" {
		clauses = append(clauses, fmt.Sprintf(`project = "%s"`, projectKey))
	}

	if fb.typeDrop.HasSelection() {
		names := fb.typeDrop.SelectedLabels()
		if len(names) == 1 {
			clauses = append(clauses, fmt.Sprintf(`issuetype = "%s"`, names[0]))
		} else {
			clauses = append(clauses, fmt.Sprintf(`issuetype IN (%s)`, quoteJoin(names)))
		}
	}

	if fb.statusDrop.HasSelection() {
		names := fb.statusDrop.SelectedLabels()
		if len(names) == 1 {
			clauses = append(clauses, fmt.Sprintf(`status = "%s"`, names[0]))
		} else {
			clauses = append(clauses, fmt.Sprintf(`status IN (%s)`, quoteJoin(names)))
		}
	} else {
		clauses = append(clauses, "statusCategory != Done")
	}

	if fb.assigneeDrop.HasSelection() {
		ids := fb.assigneeDrop.SelectedIDs()
		if len(ids) == 1 {
			clauses = append(clauses, fmt.Sprintf(`assignee = "%s"`, ids[0]))
		} else {
			clauses = append(clauses, fmt.Sprintf(`assignee IN (%s)`, quoteJoin(ids)))
		}
	}

	if fb.labelsDrop.HasSelection() {
		names := fb.labelsDrop.SelectedLabels()
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

func (fb FilterBar) Update(msg tea.Msg) (FilterBar, tea.Cmd) {
	if !fb.expanded {
		return fb, nil
	}

	if fb.typeDrop.IsOpen() {
		var cmd tea.Cmd
		fb.typeDrop, cmd = fb.typeDrop.Update(msg)
		if !fb.typeDrop.IsOpen() {
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

	if lipgloss.Width(summary) > fb.width && fb.width > 0 {
		summary = truncStr(summary, fb.width-4) + hint.Render(" (f)")
	}

	return summary
}

func (fb FilterBar) renderExpanded() string {
	var b strings.Builder

	style := lipgloss.NewStyle().Foreground(colorAccent)
	hint := lipgloss.NewStyle().Foreground(colorSubtle)
	b.WriteString(style.Render("▼ Filters") + " " + hint.Render("(esc to collapse)"))
	b.WriteString("\n")

	fieldW := fieldWidth(fb.width, 4)
	fb.typeDrop.SetWidth(fieldW)
	fb.statusDrop.SetWidth(fieldW)
	fb.assigneeDrop.SetWidth(fieldW)
	fb.labelsDrop.SetWidth(fieldW)

	b.WriteString(joinFieldsHorizontal(
		fb.typeDrop.View(),
		fb.statusDrop.View(),
		fb.assigneeDrop.View(),
		fb.labelsDrop.View(),
	))
	b.WriteString("\n")

	dateW := 20
	searchW := fb.width - dateW*2 - 2
	if searchW < 20 {
		searchW = 20
	}

	searchView := fb.renderSearchField(searchW)

	b.WriteString(joinFieldsHorizontal(
		fb.createdFrom.View(),
		fb.createdUntil.View(),
		searchView,
	))

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

func (fb FilterBar) OverlayLines() (lines []string, startLine, startCol int) {
	fieldW := fieldWidth(fb.width, 4)

	if fb.typeDrop.IsOpen() {
		return fb.typeDrop.RenderOverlay(), 4, 0
	}
	if fb.statusDrop.IsOpen() {
		return fb.statusDrop.RenderOverlay(), 4, fieldW + 1
	}
	if fb.assigneeDrop.IsOpen() {
		return fb.assigneeDrop.RenderOverlay(), 4, (fieldW + 1) * 2
	}
	if fb.labelsDrop.IsOpen() {
		return fb.labelsDrop.RenderOverlay(), 4, (fieldW + 1) * 3
	}
	if fb.createdFrom.IsOpen() {
		return fb.createdFrom.RenderOverlay(), 7, 0
	}
	if fb.createdUntil.IsOpen() {
		return fb.createdUntil.RenderOverlay(), 7, 21
	}

	return nil, 0, 0
}
