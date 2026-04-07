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

func NewMultiSelect(label string, items []DropdownItem, width int) MultiSelect {
	return MultiSelect{
		label:      label,
		items:      items,
		selected:   make(map[string]bool),
		maxVisible: 8,
		width:      width,
	}
}

func (ms *MultiSelect) Toggle(id string) {
	if ms.selected[id] {
		delete(ms.selected, id)
	} else {
		ms.selected[id] = true
	}
}

func (ms *MultiSelect) ToggleCursor() {
	if ms.cursor >= 0 && ms.cursor < len(ms.items) {
		ms.Toggle(ms.items[ms.cursor].ID)
	}
}

func (ms *MultiSelect) Clear() {
	ms.selected = make(map[string]bool)
}

func (ms *MultiSelect) SetSelected(ids map[string]bool) {
	ms.selected = ids
}

func (ms MultiSelect) SelectedIDs() []string {
	var ids []string
	for _, item := range ms.items {
		if ms.selected[item.ID] {
			ids = append(ids, item.ID)
		}
	}
	return ids
}

func (ms MultiSelect) SelectedLabels() []string {
	var labels []string
	for _, item := range ms.items {
		if ms.selected[item.ID] {
			labels = append(labels, item.Label)
		}
	}
	return labels
}

func (ms MultiSelect) HasSelection() bool {
	return len(ms.selected) > 0
}

func (ms MultiSelect) ValueText() string {
	labels := ms.SelectedLabels()
	if len(labels) == 0 {
		return "All"
	}
	return strings.Join(labels, ", ")
}

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
	ms.selected = make(map[string]bool)
}

// SetItemsKeepSelection updates the item list but preserves any existing
// selections whose IDs still appear in the new list.
func (ms *MultiSelect) SetItemsKeepSelection(items []DropdownItem) {
	newIDs := make(map[string]bool, len(items))
	for _, item := range items {
		newIDs[item.ID] = true
	}
	// Remove selections for IDs that no longer exist
	for id := range ms.selected {
		if !newIDs[id] {
			delete(ms.selected, id)
		}
	}
	ms.items = items
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

			labelMaxW := valW - 4
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
