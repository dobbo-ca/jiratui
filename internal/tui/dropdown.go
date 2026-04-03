package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// DropdownItem represents a selectable option in the dropdown.
type DropdownItem struct {
	ID    string
	Label string
}

// Dropdown is a select/dropdown component with optional search filtering.
type Dropdown struct {
	label       string
	items       []DropdownItem // selectable items (below search if searchable)
	pinnedItems []DropdownItem // items pinned above the search bar
	filtered    []DropdownItem
	allDisplay  []DropdownItem // pinnedItems + filtered, used for cursor navigation
	selected    int            // index in allDisplay
	cursor      int            // visual cursor in allDisplay
	value       string
	valueColor  lipgloss.Color
	open        bool
	searchable  bool // whether to show search input
	search      textinput.Model
	width       int
	maxVisible  int
	scrollOff   int // scroll offset for long lists
	styledValue string
	StyleFunc   func(value string) string // dynamically style the displayed value
}

// NewDropdown creates a new dropdown with the given label and items.
func NewDropdown(label string, items []DropdownItem, currentValue, currentID string, width int) Dropdown {
	s := textinput.New()
	s.Prompt = "🔍 "
	s.Placeholder = "Search..."
	s.CharLimit = 100

	d := Dropdown{
		label:      label,
		items:      items,
		filtered:   items,
		value:      currentValue,
		valueColor: colorText,
		searchable: true,
		search:     s,
		width:      width,
		maxVisible: 6,
	}

	d.rebuildAllDisplay()

	// Find selected index
	for i, item := range d.allDisplay {
		if item.ID == currentID {
			d.selected = i
			break
		}
	}

	return d
}

// NewSimpleDropdown creates a dropdown without a search bar (for static lists like status/priority).
func NewSimpleDropdown(label string, items []DropdownItem, currentValue, currentID string, width int) Dropdown {
	d := Dropdown{
		label:      label,
		items:      items,
		filtered:   items,
		value:      currentValue,
		valueColor: colorText,
		searchable: false,
		width:      width,
		maxVisible: 8,
	}

	d.rebuildAllDisplay()

	for i, item := range d.allDisplay {
		if item.ID == currentID {
			d.selected = i
			break
		}
	}

	return d
}

// SetPinnedItems sets items that appear above the search bar (e.g. "Unassigned").
func (d *Dropdown) SetPinnedItems(items []DropdownItem) {
	d.pinnedItems = items
	d.rebuildAllDisplay()
}

// SetStyledValue sets a pre-styled display value (for status colors, etc.).
func (d *Dropdown) SetStyledValue(styled string) {
	d.styledValue = styled
}

// SetValueColor sets the color used to render the value when not using styled value.
func (d *Dropdown) SetValueColor(c lipgloss.Color) {
	d.valueColor = c
}

func (d Dropdown) IsOpen() bool {
	return d.open
}

func (d Dropdown) Value() string {
	return d.value
}

func (d Dropdown) SelectedItem() *DropdownItem {
	if d.selected >= 0 && d.selected < len(d.allDisplay) {
		return &d.allDisplay[d.selected]
	}
	return nil
}

func (d *Dropdown) Toggle() tea.Cmd {
	if d.open {
		return d.Close()
	}
	return d.OpenDropdown()
}

func (d *Dropdown) OpenDropdown() tea.Cmd {
	d.open = true
	d.search.SetValue("")
	d.filtered = d.items
	d.rebuildAllDisplay()
	// Position cursor on the currently selected item
	d.cursor = d.selected
	if d.cursor < 0 || d.cursor >= len(d.allDisplay) {
		d.cursor = 0
	}
	// Scroll to make cursor visible
	if d.cursor >= d.maxVisible {
		d.scrollOff = d.cursor - d.maxVisible/2
	} else {
		d.scrollOff = 0
	}
	if d.searchable {
		d.search.Focus()
		return d.search.Cursor.BlinkCmd()
	}
	return nil
}

func (d *Dropdown) Close() tea.Cmd {
	d.open = false
	d.search.Blur()
	return nil
}

func (d *Dropdown) SetItems(items []DropdownItem) {
	d.items = items
	d.applyFilter()
}

// rebuildAllDisplay combines pinned items and filtered items into the navigable list.
func (d *Dropdown) rebuildAllDisplay() {
	d.allDisplay = make([]DropdownItem, 0, len(d.pinnedItems)+len(d.filtered))
	d.allDisplay = append(d.allDisplay, d.pinnedItems...)
	d.allDisplay = append(d.allDisplay, d.filtered...)
}

func (d Dropdown) Update(msg tea.Msg) (Dropdown, tea.Cmd) {
	if !d.open {
		return d, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			d.Close()
			return d, nil
		case "enter":
			if len(d.allDisplay) > 0 && d.cursor < len(d.allDisplay) {
				item := d.allDisplay[d.cursor]
				d.value = item.Label
				d.selected = d.cursor
			}
			d.Close()
			return d, nil
		case "down", "ctrl+n", "j":
			if d.cursor < len(d.allDisplay)-1 {
				d.cursor++
				if d.cursor >= d.scrollOff+d.maxVisible {
					d.scrollOff = d.cursor - d.maxVisible + 1
				}
			}
			return d, nil
		case "up", "ctrl+p", "k":
			if d.cursor > 0 {
				d.cursor--
				if d.cursor < d.scrollOff {
					d.scrollOff = d.cursor
				}
			}
			return d, nil
		}

		if d.searchable {
			// Forward to search input
			var cmd tea.Cmd
			d.search, cmd = d.search.Update(msg)
			d.applyFilter()
			return d, cmd
		}
		return d, nil
	}

	// Forward non-key messages (blink, etc.)
	if d.searchable {
		var cmd tea.Cmd
		d.search, cmd = d.search.Update(msg)
		return d, cmd
	}
	return d, nil
}

func (d *Dropdown) applyFilter() {
	query := strings.ToLower(d.search.Value())
	if query == "" {
		d.filtered = d.items
	} else {
		d.filtered = nil
		for _, item := range d.items {
			if strings.Contains(strings.ToLower(item.Label), query) {
				d.filtered = append(d.filtered, item)
			}
		}
	}
	d.rebuildAllDisplay()
	d.cursor = 0
	d.scrollOff = 0
}

func (d Dropdown) View() string {
	return d.renderClosed()
}

func (d Dropdown) renderClosed() string {
	width := d.width
	if width < 8 {
		width = 8
	}

	innerW := width - 2
	valW := innerW - 2

	lbl := lipgloss.NewStyle().Foreground(colorAccent)
	bdr := lipgloss.NewStyle().Foreground(colorBorder)

	labelText := " " + d.label + " ▾ "
	if d.open {
		bdr = bdr.Foreground(colorAccent)
	}

	dashes := innerW - lipgloss.Width(labelText) - 1
	if dashes < 0 {
		dashes = 0
	}

	top := bdr.Render("╭─") + lbl.Render(labelText) + bdr.Render(strings.Repeat("─", dashes)+"╮")

	var content string
	if d.StyleFunc != nil {
		content = d.StyleFunc(d.value)
	} else if d.styledValue != "" {
		content = d.styledValue
	} else {
		content = lipgloss.NewStyle().Foreground(d.valueColor).Render(truncStr(d.value, valW))
	}
	visW := lipgloss.Width(content)
	pad := valW - visW
	if pad < 0 {
		pad = 0
	}
	mid := bdr.Render("│") + " " + content + strings.Repeat(" ", pad) + " " + bdr.Render("│")

	// When open, use connecting border instead of closing the box
	var bot string
	if d.open {
		bot = bdr.Render("├" + strings.Repeat("─", innerW) + "┤")
	} else {
		bot = bdr.Render("╰" + strings.Repeat("─", innerW) + "╯")
	}

	return top + "\n" + mid + "\n" + bot
}

// HandleClick processes a mouse click on the overlay at the given line index
// (0-based, relative to the overlay). Returns true if an item was selected.
func (d *Dropdown) HandleClick(overlayLine int) bool {
	if !d.open {
		return false
	}

	// Reconstruct the same layout as RenderOverlay to map line -> item
	pinnedCount := len(d.pinnedItems)
	line := 0

	// Pinned items
	for i := range d.pinnedItems {
		if overlayLine == line {
			d.value = d.pinnedItems[i].Label
			d.selected = i
			d.Close()
			return true
		}
		line++
	}

	// Search separator (if pinned items exist and searchable)
	if d.searchable && pinnedCount > 0 {
		line++ // separator ├──┤
	}

	// Search bar
	if d.searchable {
		line++ // search input line
	}

	// Separator before items
	if pinnedCount > 0 || d.searchable {
		line++ // separator ├──┤
	}

	// Filtered items - calculate visible range (same logic as RenderOverlay)
	filteredStart := 0
	filteredEnd := len(d.filtered)
	if filteredEnd > d.maxVisible {
		filteredCursor := d.cursor - pinnedCount
		if filteredCursor < 0 {
			filteredStart = 0
		} else {
			filteredStart = filteredCursor - d.maxVisible/2
			if filteredStart < 0 {
				filteredStart = 0
			}
		}
		filteredEnd = filteredStart + d.maxVisible
		if filteredEnd > len(d.filtered) {
			filteredEnd = len(d.filtered)
			filteredStart = filteredEnd - d.maxVisible
			if filteredStart < 0 {
				filteredStart = 0
			}
		}
	}

	for i := filteredStart; i < filteredEnd; i++ {
		if overlayLine == line {
			d.value = d.filtered[i].Label
			d.selected = pinnedCount + i
			d.Close()
			return true
		}
		line++
	}

	return false
}

// RenderStandaloneOverlay returns the full dropdown including field header + overlay.
// Use this when the field itself is not rendered inline (e.g. project selector).
func (d Dropdown) RenderStandaloneOverlay() []string {
	if !d.open {
		return nil
	}

	// Render the field header lines, then append the overlay
	fieldLines := strings.Split(d.renderClosed(), "\n")
	overlayLines := d.RenderOverlay()
	return append(fieldLines, overlayLines...)
}

// RenderOverlay returns the dropdown menu lines to be overlaid below the field.
func (d Dropdown) RenderOverlay() []string {
	if !d.open {
		return nil
	}

	width := d.width
	if width < 8 {
		width = 8
	}
	innerW := width - 2
	valW := innerW - 2

	bdr := lipgloss.NewStyle().Foreground(colorAccent)
	selectedStyle := lipgloss.NewStyle().Foreground(colorText).Background(colorSelection).Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(colorText)

	var lines []string

	// Pinned items (above search) — field's bottom border already provides ├──┤
	pinnedCount := len(d.pinnedItems)
	if pinnedCount > 0 {
		for i, item := range d.pinnedItems {
			style := normalStyle
			if i == d.cursor {
				style = selectedStyle
			}
			label := truncStr(item.Label, valW)
			lVisW := lipgloss.Width(label)
			pad := valW - lVisW
			if pad < 0 {
				pad = 0
			}
			lines = append(lines, bdr.Render("│")+" "+style.Render(label+strings.Repeat(" ", pad))+" "+bdr.Render("│"))
		}
	}

	// Search bar (only for searchable dropdowns)
	if d.searchable {
		if pinnedCount > 0 {
			// Separator between pinned items and search
			lines = append(lines, bdr.Render("├")+bdr.Render(strings.Repeat("─", innerW))+bdr.Render("┤"))
		}
		d.search.Width = valW - lipgloss.Width(d.search.Prompt)
		searchContent := d.search.View()
		searchVisW := lipgloss.Width(searchContent)
		searchPad := valW - searchVisW
		if searchPad < 0 {
			searchPad = 0
		}
		lines = append(lines, bdr.Render("│")+" "+searchContent+strings.Repeat(" ", searchPad)+" "+bdr.Render("│"))
	}

	// Separator before items list (if there's anything above)
	if pinnedCount > 0 || d.searchable {
		lines = append(lines, bdr.Render("├")+bdr.Render(strings.Repeat("─", innerW))+bdr.Render("┤"))
	}

	// Filtered items
	if len(d.filtered) == 0 {
		noMatch := lipgloss.NewStyle().Foreground(colorSubtle).Render("No matches")
		pad := valW - lipgloss.Width(noMatch)
		if pad < 0 {
			pad = 0
		}
		lines = append(lines, bdr.Render("│")+" "+noMatch+strings.Repeat(" ", pad)+" "+bdr.Render("│"))
	} else {
		// Calculate visible range within filtered items
		// The cursor in allDisplay: 0..pinnedCount-1 = pinned, pinnedCount.. = filtered
		filteredStart := 0
		filteredEnd := len(d.filtered)
		if filteredEnd > d.maxVisible {
			// Determine scroll based on cursor position within filtered items
			filteredCursor := d.cursor - pinnedCount
			if filteredCursor < 0 {
				filteredStart = 0
			} else {
				filteredStart = filteredCursor - d.maxVisible/2
				if filteredStart < 0 {
					filteredStart = 0
				}
			}
			filteredEnd = filteredStart + d.maxVisible
			if filteredEnd > len(d.filtered) {
				filteredEnd = len(d.filtered)
				filteredStart = filteredEnd - d.maxVisible
				if filteredStart < 0 {
					filteredStart = 0
				}
			}
		}

		for i := filteredStart; i < filteredEnd; i++ {
			item := d.filtered[i]
			displayIdx := pinnedCount + i // index in allDisplay
			style := normalStyle
			if displayIdx == d.cursor {
				style = selectedStyle
			}
			label := truncStr(item.Label, valW)
			lVisW := lipgloss.Width(label)
			pad := valW - lVisW
			if pad < 0 {
				pad = 0
			}
			lines = append(lines, bdr.Render("│")+" "+style.Render(label+strings.Repeat(" ", pad))+" "+bdr.Render("│"))
		}

		// Scroll indicator
		if len(d.filtered) > d.maxVisible {
			indicator := lipgloss.NewStyle().Foreground(colorSubtle).Render(
				fmt.Sprintf(" %d-%d of %d", filteredStart+1, filteredEnd, len(d.filtered)),
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
