package tui

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/christopherdobbyn/jiratui/internal/models"
	"github.com/muesli/ansi"
	"github.com/pkg/browser"
)

type detailTab int

const (
	tabDetails detailTab = iota
	tabComments
	tabSubtasks
	tabLinks
	tabAttachments
)

// Detail is the Bubble Tea model for the issue detail view.
type Detail struct {
	issue     models.Issue
	activeTab detailTab
	scrollY   int
	width     int
	height    int

	// Editable fields
	titleInput   textinput.Model
	descInput    textarea.Model
	descFocused  bool
	assigneeDrop Dropdown
	statusDrop   Dropdown
	priorityDrop Dropdown
	dueDatePick  DatePicker
}

// NewDetail creates a new detail model for the given issue.
func NewDetail(issue models.Issue, width, height int) Detail {
	ti := textinput.New()
	ti.SetValue(issue.Summary)
	ti.CharLimit = 255
	ti.Prompt = ""

	ta := textarea.New()
	ta.SetValue(issue.Description)
	ta.ShowLineNumbers = false
	ta.Prompt = ""
	ta.CharLimit = 0
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()

	// Assignee dropdown — searchable, with "Unassigned" pinned above search
	assigneeVal := "Unassigned"
	assigneeID := ""
	if issue.Assignee != nil {
		assigneeVal = issue.Assignee.DisplayName
		assigneeID = issue.Assignee.AccountID
	}
	assigneeDrop := NewDropdown("Assignee", nil, assigneeVal, assigneeID, 0)
	assigneeDrop.SetPinnedItems([]DropdownItem{{ID: "", Label: "Unassigned"}})
	assigneeDrop.StyleFunc = func(val string) string {
		if val == "Unassigned" {
			return lipgloss.NewStyle().Foreground(colorSubtle).Render(val)
		}
		return lipgloss.NewStyle().Foreground(colorSuccess).Render(val)
	}

	// Status dropdown — simple (no search)
	statusDrop := NewSimpleDropdown("Status", nil, issue.Status.Name, issue.Status.ID, 0)
	statusDrop.StyleFunc = func(val string) string {
		return StyledStatus(val)
	}

	// Priority dropdown — simple (no search)
	priorityDrop := NewSimpleDropdown("Priority", nil, issue.Priority.Name, issue.Priority.ID, 0)
	priorityDrop.StyleFunc = func(val string) string {
		return StyledPriority(val)
	}

	// Due date picker
	dueDatePick := NewDatePicker("Due Date", issue.DueDate, 0)
	dueDatePick.SetValueColor(dueDateColor(issue.DueDate))

	return Detail{
		issue:        issue,
		width:        width,
		height:       height,
		titleInput:   ti,
		descInput:    ta,
		assigneeDrop: assigneeDrop,
		statusDrop:   statusDrop,
		priorityDrop: priorityDrop,
		dueDatePick:  dueDatePick,
	}
}

// SetAssigneeOptions populates the assignee dropdown with users.
// "Unassigned" is pinned above the search; these are the searchable users below it.
func (d *Detail) SetAssigneeOptions(users []models.User) {
	items := make([]DropdownItem, len(users))
	for i, u := range users {
		items[i] = DropdownItem{ID: u.AccountID, Label: u.DisplayName}
	}
	d.assigneeDrop.SetItems(items)
}

// SetStatusOptions populates the status dropdown with transitions.
func (d *Detail) SetStatusOptions(transitions []models.Transition) {
	items := make([]DropdownItem, 0, len(transitions)+1)
	items = append(items, DropdownItem{ID: d.issue.Status.ID, Label: d.issue.Status.Name})
	for _, t := range transitions {
		if t.To.ID != d.issue.Status.ID {
			items = append(items, DropdownItem{ID: t.To.ID, Label: t.To.Name})
		}
	}
	d.statusDrop.SetItems(items)
}

// SetPriorityOptions populates the priority dropdown.
func (d *Detail) SetPriorityOptions(priorities []models.Priority) {
	items := make([]DropdownItem, len(priorities))
	for i, p := range priorities {
		items[i] = DropdownItem{ID: p.ID, Label: p.Name}
	}
	d.priorityDrop.SetItems(items)
}

// Editing returns true when any input field in the detail view is focused.
func (d Detail) Editing() bool {
	return d.titleInput.Focused() || d.descFocused ||
		d.assigneeDrop.IsOpen() || d.statusDrop.IsOpen() ||
		d.priorityDrop.IsOpen() || d.dueDatePick.IsOpen()
}

// tabLabels returns the display labels for each tab.
func (d Detail) tabLabels() []string {
	return []string{
		"Details",
		fmt.Sprintf("Comments(%d)", len(d.issue.Comments)),
		fmt.Sprintf("Subtasks(%d)", len(d.issue.Subtasks)),
		fmt.Sprintf("Links(%d)", len(d.issue.Links)),
		fmt.Sprintf("Attach(%d)", len(d.issue.Attachments)),
	}
}

// Update handles messages for the detail view.
func (d Detail) Update(msg tea.Msg) (Detail, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// When a dropdown is open, forward keys to it
		if d.assigneeDrop.IsOpen() {
			var cmd tea.Cmd
			d.assigneeDrop, cmd = d.assigneeDrop.Update(msg)
			return d, cmd
		}
		if d.statusDrop.IsOpen() {
			var cmd tea.Cmd
			d.statusDrop, cmd = d.statusDrop.Update(msg)
			return d, cmd
		}
		if d.priorityDrop.IsOpen() {
			var cmd tea.Cmd
			d.priorityDrop, cmd = d.priorityDrop.Update(msg)
			return d, cmd
		}
		if d.dueDatePick.IsOpen() {
			var cmd tea.Cmd
			d.dueDatePick, cmd = d.dueDatePick.Update(msg)
			return d, cmd
		}

		// When title input is focused
		if d.titleInput.Focused() {
			switch msg.String() {
			case "esc", "enter":
				d.titleInput.Blur()
				return d, nil
			}
			if msg.String() == "ctrl+j" || msg.String() == "ctrl+m" {
				return d, nil
			}
			var cmd tea.Cmd
			d.titleInput, cmd = d.titleInput.Update(msg)
			return d, cmd
		}

		// When description is focused
		if d.descFocused {
			if msg.String() == "esc" {
				d.descFocused = false
				d.descInput.Blur()
				return d, nil
			}
			var cmd tea.Cmd
			d.descInput, cmd = d.descInput.Update(msg)
			return d, cmd
		}

		switch {
		case key.Matches(msg, detailKeys.Tab1):
			d.activeTab = tabDetails
			d.scrollY = 0
		case key.Matches(msg, detailKeys.Tab2):
			d.activeTab = tabComments
			d.scrollY = 0
		case key.Matches(msg, detailKeys.Tab3):
			d.activeTab = tabSubtasks
			d.scrollY = 0
		case key.Matches(msg, detailKeys.Tab4):
			d.activeTab = tabLinks
			d.scrollY = 0
		case key.Matches(msg, detailKeys.Tab5):
			d.activeTab = tabAttachments
			d.scrollY = 0
		case key.Matches(msg, detailKeys.Down):
			if d.activeTab == tabDetails {
				if d.scrollY < d.descMaxScroll() {
					d.scrollY++
				}
			} else {
				d.scrollY++
			}
		case key.Matches(msg, detailKeys.Up):
			if d.scrollY > 0 {
				d.scrollY--
			}
		case key.Matches(msg, detailKeys.Open):
			_ = browser.OpenURL(d.issue.BrowseURL)
		}
		return d, nil

	case tea.MouseMsg:
		// Only handle press events, not release
		if msg.Action != tea.MouseActionPress {
			// Still forward non-press to open components for blink etc.
			if msg.Button == tea.MouseButtonWheelDown {
				if d.activeTab == tabDetails {
					if d.scrollY < d.descMaxScroll() {
						d.scrollY++
					}
				} else {
					d.scrollY++
				}
				return d, nil
			}
			if msg.Button == tea.MouseButtonWheelUp {
				if d.scrollY > 0 {
					d.scrollY--
				}
				return d, nil
			}
			return d, nil
		}

		switch msg.Button {
		case tea.MouseButtonLeft:
			if msg.Y <= 1 {
				d.handleTabClick(msg.X)
			} else if d.activeTab == tabDetails {
				return d, d.handleDetailClick(msg.X, msg.Y)
			}
			return d, nil
		}

	case tea.WindowSizeMsg:
		d.width = msg.Width
		d.height = msg.Height
		if d.activeTab == tabDetails {
			max := d.descMaxScroll()
			if d.scrollY > max {
				d.scrollY = max
			}
		}
		return d, nil
	}

	// Forward blink messages to focused components
	if d.assigneeDrop.IsOpen() {
		var cmd tea.Cmd
		d.assigneeDrop, cmd = d.assigneeDrop.Update(msg)
		return d, cmd
	}
	if d.statusDrop.IsOpen() {
		var cmd tea.Cmd
		d.statusDrop, cmd = d.statusDrop.Update(msg)
		return d, cmd
	}
	if d.priorityDrop.IsOpen() {
		var cmd tea.Cmd
		d.priorityDrop, cmd = d.priorityDrop.Update(msg)
		return d, cmd
	}
	if d.titleInput.Focused() {
		var cmd tea.Cmd
		d.titleInput, cmd = d.titleInput.Update(msg)
		return d, cmd
	}
	if d.descFocused {
		var cmd tea.Cmd
		d.descInput, cmd = d.descInput.Update(msg)
		return d, cmd
	}

	return d, nil
}

// handleTabClick switches tab based on X click position.
func (d *Detail) handleTabClick(x int) {
	labels := d.tabLabels()
	pos := 1
	for i, label := range labels {
		if i > 0 {
			pos++
		}
		numLabel := fmt.Sprintf("%d:%s", i+1, label)
		w := len(numLabel)
		if x >= pos && x < pos+w {
			d.activeTab = detailTab(i)
			d.scrollY = 0
			return
		}
		pos += w
	}
}

// blurAll unfocuses all editable fields.
func (d *Detail) blurAll() {
	d.titleInput.Blur()
	d.descFocused = false
	d.descInput.Blur()
	d.assigneeDrop.Close()
	d.statusDrop.Close()
	d.priorityDrop.Close()
	d.dueDatePick.Close()
}

// anyOverlayOpen returns true if any dropdown/picker overlay is showing.
func (d Detail) anyOverlayOpen() bool {
	return d.assigneeDrop.IsOpen() || d.statusDrop.IsOpen() ||
		d.priorityDrop.IsOpen() || d.dueDatePick.IsOpen()
}

// handleDetailClick handles mouse clicks on detail fields.
func (d *Detail) handleDetailClick(x, y int) tea.Cmd {
	w := d.width - 1
	col3W := w / 3

	// If anything is being edited, the first click just closes/blurs it.
	// Overlay clicks on the open overlay's items are the exception.
	if d.Editing() {
		if d.anyOverlayOpen() {
			// Standalone overlays start at the field's row position
			// Row 2 fields start at y = 2 (tab bar) + 3 (row1) = 5
			// Row 3 fields start at y = 5 + 3 = 8
			row2FieldY := 2 + 3
			row3FieldY := row2FieldY + 3

			if d.assigneeDrop.IsOpen() {
				overlay := d.assigneeDrop.RenderStandaloneOverlay()
				if overlay != nil && y >= row2FieldY && y < row2FieldY+len(overlay) && x < col3W {
					clickLine := y - row2FieldY - 3 // subtract field header
					if clickLine >= 0 && d.assigneeDrop.HandleClick(clickLine) {
						return nil
					}
				}
			}
			if d.statusDrop.IsOpen() {
				overlay := d.statusDrop.RenderStandaloneOverlay()
				if overlay != nil && y >= row2FieldY && y < row2FieldY+len(overlay) && x >= col3W && x < 2*col3W {
					clickLine := y - row2FieldY - 3
					if clickLine >= 0 && d.statusDrop.HandleClick(clickLine) {
						return nil
					}
				}
			}
			if d.priorityDrop.IsOpen() {
				overlay := d.priorityDrop.RenderStandaloneOverlay()
				if overlay != nil && y >= row3FieldY && y < row3FieldY+len(overlay) && x < col3W {
					clickLine := y - row3FieldY - 3
					if clickLine >= 0 && d.priorityDrop.HandleClick(clickLine) {
						return nil
					}
				}
			}
			if d.dueDatePick.IsOpen() {
				overlay := d.dueDatePick.RenderOverlay()
				if overlay != nil {
					// Date picker overlay starts after the field (3 lines for field header)
					overlayStartY := row3FieldY + 3
					if y >= overlayStartY && y < overlayStartY+len(overlay) &&
						x >= col3W && x < 2*col3W {
						overlayLine := y - overlayStartY
						localX := x - col3W
						if d.dueDatePick.HandleClick(overlayLine, localX, col3W) {
							return nil
						}
					}
				}
			}
		}

		// Click wasn't on an overlay item — just close/blur everything
		d.blurAll()
		return nil
	}

	// Row 1: y=2..4 — Key + Title
	if y >= 2 && y <= 4 {
		keyW := d.keyFieldWidth()
		if x >= keyW {
			d.blurAll()
			d.titleInput.Focus()
			return d.titleInput.Cursor.BlinkCmd()
		}
		d.blurAll()
		return nil
	}

	// Row 2: y=5..7 — Assignee + Status + Type
	if y >= 5 && y <= 7 {
		if x < col3W {
			d.blurAll()
			return d.assigneeDrop.OpenDropdown()
		} else if x < 2*col3W {
			d.blurAll()
			return d.statusDrop.OpenDropdown()
		}
		d.blurAll()
		return nil
	}

	// Row 3: y=8..10 — Priority + Due Date + Reporter
	if y >= 8 && y <= 10 {
		if x < col3W {
			d.blurAll()
			return d.priorityDrop.OpenDropdown()
		} else if x < 2*col3W {
			d.blurAll()
			d.dueDatePick.OpenPicker()
			return nil
		}
		d.blurAll()
		return nil
	}

	// Calculate description row start
	descRowStart := 4*3 + 2 // 4 rows * 3 lines + tab bar (2 lines)
	hasSprint := d.issue.Sprint != "" || len(d.issue.Labels) > 0
	if hasSprint {
		descRowStart += 3
	}

	if y >= descRowStart {
		d.blurAll()
		d.descFocused = true
		d.descInput.Focus()
		return d.descInput.Cursor.BlinkCmd()
	}

	d.blurAll()
	return nil
}

// keyFieldWidth returns the width for the Key field, sized to fit the key value.
func (d Detail) keyFieldWidth() int {
	valueW := len(d.issue.Key) + 4
	labelW := 9
	w := valueW
	if w < labelW {
		w = labelW
	}
	return w
}

// descMaxScroll returns the maximum scroll offset for the description field.
func (d Detail) descMaxScroll() int {
	if d.descFocused {
		return 0
	}
	w := d.width - 1
	if w < 8 {
		w = 8
	}
	innerW := w - 2
	valW := innerW - 2

	descText := d.issue.Description
	if descText == "" {
		descText = "No description."
	}
	wrapped := wordWrap(descText, valW)
	totalLines := len(strings.Split(wrapped, "\n"))

	usedLines := 4 * 3
	if d.issue.Sprint != "" || len(d.issue.Labels) > 0 {
		usedLines += 3
	}
	availH := d.height - 2 - usedLines - 2
	if availH < 3 {
		availH = 3
	}

	maxScroll := totalLines - availH
	if maxScroll < 0 {
		maxScroll = 0
	}
	return maxScroll
}

// overlayInfo describes content to composite on top of rendered output.
type overlayInfo struct {
	lines []string // overlay lines to render
	y     int      // starting line index (relative to detail content, after tab bar)
	x     int      // x offset (0 = left aligned)
}

// View renders the detail view with proper overlay compositing.
func (d Detail) View() string {
	var b strings.Builder

	b.WriteString(d.renderTabBar())
	b.WriteString("\n")

	var baseContent string
	var overlays []overlayInfo

	switch d.activeTab {
	case tabDetails:
		baseContent, overlays = d.renderDetailsTabWithOverlays()
	default:
		switch d.activeTab {
		case tabComments:
			baseContent = d.renderCommentsTab()
		case tabSubtasks:
			baseContent = d.renderSubtasksTab()
		case tabLinks:
			baseContent = d.renderLinksTab()
		case tabAttachments:
			baseContent = d.renderAttachmentsTab()
		}
		baseContent = d.applyScroll(baseContent)
	}

	// Split base content into lines and apply overlays on top
	lines := strings.Split(baseContent, "\n")

	// Ensure enough lines exist for all overlays
	for _, ov := range overlays {
		needed := ov.y + len(ov.lines)
		for len(lines) < needed {
			lines = append(lines, "")
		}
	}

	for _, ov := range overlays {
		for i, oLine := range ov.lines {
			idx := ov.y + i
			if idx >= 0 && idx < len(lines) {
				existing := lines[idx]
				oLineW := lipgloss.Width(oLine)
				existW := lipgloss.Width(existing)

				var result string

				// Left portion: preserve content before the overlay's x offset
				if ov.x > 0 {
					if existW > ov.x {
						result = truncateAnsi(existing, ov.x)
					} else {
						result = existing + strings.Repeat(" ", ov.x-existW)
					}
				}

				// Overlay content
				result += oLine

				// Right portion: preserve content after the overlay ends
				rightStart := ov.x + oLineW
				if existW > rightStart {
					// Reset ANSI state before appending the right portion,
					// so the overlay's styling doesn't bleed into it
					result += "\x1b[0m" + extractAfter(existing, rightStart)
				}

				lines[idx] = result
			}
		}
	}

	b.WriteString(strings.Join(lines, "\n"))
	return b.String()
}

func (d Detail) renderTabBar() string {
	tabStyle := lipgloss.NewStyle().Foreground(colorAccent)
	activeStyle := tabStyle.Bold(true)
	dividerStyle := lipgloss.NewStyle().Foreground(colorBorder)
	activeDiv := lipgloss.NewStyle().Foreground(colorAccent)

	labels := d.tabLabels()

	var tabLine strings.Builder
	var divLine strings.Builder

	tabLine.WriteString(" ")
	divLine.WriteString(dividerStyle.Render("─"))

	for i, label := range labels {
		if i > 0 {
			tabLine.WriteString(" ")
			divLine.WriteString(dividerStyle.Render("─"))
		}
		numLabel := fmt.Sprintf("%d:%s", i+1, label)
		w := len(numLabel)
		if detailTab(i) == d.activeTab {
			tabLine.WriteString(activeStyle.Render(numLabel))
			divLine.WriteString(activeDiv.Render(strings.Repeat("━", w)))
		} else {
			tabLine.WriteString(tabStyle.Render(numLabel))
			divLine.WriteString(dividerStyle.Render(strings.Repeat("─", w)))
		}
	}

	tabWidth := 1
	for i, label := range labels {
		if i > 0 {
			tabWidth++
		}
		tabWidth += len(fmt.Sprintf("%d:%s", i+1, label))
	}
	remaining := d.width - tabWidth
	if remaining > 0 {
		divLine.WriteString(dividerStyle.Render(strings.Repeat("─", remaining)))
	}

	return tabLine.String() + "\n" + divLine.String()
}

func (d Detail) applyScroll(content string) string {
	lines := strings.Split(content, "\n")
	viewHeight := d.height - 2
	if viewHeight < 1 {
		viewHeight = 1
	}

	maxScroll := len(lines) - viewHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	scrollY := d.scrollY
	if scrollY > maxScroll {
		scrollY = maxScroll
	}

	start := scrollY
	end := start + viewHeight
	if end > len(lines) {
		end = len(lines)
	}

	return strings.Join(lines[start:end], "\n")
}

// ── Form field rendering ──────────────────────────────────────

func renderField(label, value string, width int, valueColor lipgloss.Color) string {
	if width < 8 {
		width = 8
	}
	bdr := lipgloss.NewStyle().Foreground(colorBorder)
	lbl := lipgloss.NewStyle().Foreground(colorAccent)

	innerW := width - 2
	labelText := " " + label + " "
	dashes := innerW - len(labelText) - 1
	if dashes < 0 {
		dashes = 0
	}
	top := bdr.Render("╭─") + lbl.Render(labelText) + bdr.Render(strings.Repeat("─", dashes)+"╮")

	valW := innerW - 2
	displayVal := truncStr(value, valW)
	visW := lipgloss.Width(displayVal)
	pad := valW - visW
	if pad < 0 {
		pad = 0
	}
	mid := bdr.Render("│") + " " + lipgloss.NewStyle().Foreground(valueColor).Render(displayVal) + strings.Repeat(" ", pad) + " " + bdr.Render("│")

	bot := bdr.Render("╰" + strings.Repeat("─", innerW) + "╯")

	return top + "\n" + mid + "\n" + bot
}

func renderFieldStyled(label, styledValue string, width int) string {
	if width < 8 {
		width = 8
	}
	bdr := lipgloss.NewStyle().Foreground(colorBorder)
	lbl := lipgloss.NewStyle().Foreground(colorAccent)

	innerW := width - 2
	labelText := " " + label + " "
	dashes := innerW - len(labelText) - 1
	if dashes < 0 {
		dashes = 0
	}
	top := bdr.Render("╭─") + lbl.Render(labelText) + bdr.Render(strings.Repeat("─", dashes)+"╮")

	valW := innerW - 2
	visualWidth := lipgloss.Width(styledValue)
	pad := valW - visualWidth
	if pad < 0 {
		pad = 0
	}
	mid := bdr.Render("│") + " " + styledValue + strings.Repeat(" ", pad) + " " + bdr.Render("│")

	bot := bdr.Render("╰" + strings.Repeat("─", innerW) + "╯")

	return top + "\n" + mid + "\n" + bot
}

func renderFieldMultiline(label, text string, width, minContentLines, scrollY int, valueColor lipgloss.Color) string {
	if width < 8 {
		width = 8
	}
	bdr := lipgloss.NewStyle().Foreground(colorBorder)
	lbl := lipgloss.NewStyle().Foreground(colorAccent)

	innerW := width - 2
	labelText := " " + label + " "
	dashes := innerW - len(labelText) - 1
	if dashes < 0 {
		dashes = 0
	}
	top := bdr.Render("╭─") + lbl.Render(labelText) + bdr.Render(strings.Repeat("─", dashes)+"╮")

	valW := innerW - 2
	wrapped := wordWrap(text, valW)
	var midLines []string
	for _, line := range strings.Split(wrapped, "\n") {
		pad := valW - lipgloss.Width(line)
		if pad < 0 {
			pad = 0
		}
		midLines = append(midLines,
			bdr.Render("│")+" "+lipgloss.NewStyle().Foreground(valueColor).Render(line)+strings.Repeat(" ", pad)+" "+bdr.Render("│"))
	}

	emptyLine := bdr.Render("│") + " " + strings.Repeat(" ", valW) + " " + bdr.Render("│")

	if scrollY > 0 && scrollY < len(midLines) {
		midLines = midLines[scrollY:]
	} else if scrollY >= len(midLines) && len(midLines) > 0 {
		midLines = midLines[len(midLines)-1:]
	}

	if minContentLines > 0 && len(midLines) > minContentLines {
		midLines = midLines[:minContentLines]
	}

	for len(midLines) < minContentLines {
		midLines = append(midLines, emptyLine)
	}

	bot := bdr.Render("╰" + strings.Repeat("─", innerW) + "╯")

	return top + "\n" + strings.Join(midLines, "\n") + "\n" + bot
}

// renderMarkdownField renders a bordered field with glamour-rendered markdown content.
func renderMarkdownField(label, text string, width, minContentLines, scrollY int) string {
	if width < 8 {
		width = 8
	}
	bdr := lipgloss.NewStyle().Foreground(colorBorder)
	lbl := lipgloss.NewStyle().Foreground(colorAccent)

	innerW := width - 2
	labelText := " " + label + " "
	dashes := innerW - len(labelText) - 1
	if dashes < 0 {
		dashes = 0
	}
	top := bdr.Render("╭─") + lbl.Render(labelText) + bdr.Render(strings.Repeat("─", dashes)+"╮")

	valW := innerW - 2

	// Render markdown with glamour
	rendered, err := glamour.Render(text, "dark")
	if err != nil {
		// Fallback to plain text
		return renderFieldMultiline(label, text, width, minContentLines, scrollY, colorText)
	}

	// Strip trailing newlines and split into lines
	rendered = strings.TrimRight(rendered, "\n")
	var midLines []string
	for _, line := range strings.Split(rendered, "\n") {
		visW := lipgloss.Width(line)
		if visW > valW {
			line = truncateAnsi(line, valW)
			visW = lipgloss.Width(line)
		}
		pad := valW - visW
		if pad < 0 {
			pad = 0
		}
		midLines = append(midLines, bdr.Render("│")+" "+line+strings.Repeat(" ", pad)+" "+bdr.Render("│"))
	}

	emptyLine := bdr.Render("│") + " " + strings.Repeat(" ", valW) + " " + bdr.Render("│")

	if scrollY > 0 && scrollY < len(midLines) {
		midLines = midLines[scrollY:]
	} else if scrollY >= len(midLines) && len(midLines) > 0 {
		midLines = midLines[len(midLines)-1:]
	}

	if minContentLines > 0 && len(midLines) > minContentLines {
		midLines = midLines[:minContentLines]
	}

	for len(midLines) < minContentLines {
		midLines = append(midLines, emptyLine)
	}

	bot := bdr.Render("╰" + strings.Repeat("─", innerW) + "╯")

	return top + "\n" + strings.Join(midLines, "\n") + "\n" + bot
}

// renderInputField renders a bordered form field containing a textinput.Model.
func (d Detail) renderInputField(label string, input *textinput.Model, width int) string {
	if width < 8 {
		width = 8
	}

	innerW := width - 2
	valW := innerW - 2
	focused := input.Focused()

	var labelText string
	if focused {
		labelText = " " + label + " ✎ "
	} else {
		labelText = " " + label + " "
	}

	lbl := lipgloss.NewStyle().Foreground(colorAccent)

	dashes := innerW - lipgloss.Width(labelText) - 1
	if dashes < 0 {
		dashes = 0
	}

	// When focused, highlight the entire field border with accent color
	borderColor := colorBorder
	if focused {
		borderColor = colorAccent
	}
	bdr := lipgloss.NewStyle().Foreground(borderColor)

	top := bdr.Render("╭─") + lbl.Render(labelText) + bdr.Render(strings.Repeat("─", dashes)+"╮")

	var content string
	if focused {
		input.TextStyle = lipgloss.NewStyle().Foreground(colorText)
		input.Cursor.Style = lipgloss.NewStyle().Foreground(colorAccent)
		content = input.View()
	} else {
		content = lipgloss.NewStyle().Foreground(colorText).Render(truncStr(input.Value(), valW))
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

// renderTextareaField renders a bordered field with a textarea or markdown-rendered content.
func (d Detail) renderTextareaField(label string, ta *textarea.Model, width, contentLines int) string {
	if width < 8 {
		width = 8
	}

	innerW := width - 2
	valW := innerW - 2
	focused := d.descFocused

	if focused {
		var labelText string
		labelText = " " + label + " ✎ "

		lbl := lipgloss.NewStyle().Foreground(colorAccent)

		dashes := innerW - lipgloss.Width(labelText) - 1
		if dashes < 0 {
			dashes = 0
		}

		bdr := lipgloss.NewStyle().Foreground(colorAccent)

		top := bdr.Render("╭─") + lbl.Render(labelText) + bdr.Render(strings.Repeat("─", dashes)+"╮")

		ta.SetWidth(valW)
		ta.SetHeight(contentLines)
		ta.FocusedStyle.Base = lipgloss.NewStyle()
		taView := ta.View()

		taLines := strings.Split(taView, "\n")
		var midLines []string
		for i, line := range taLines {
			if i >= contentLines {
				break
			}
			visW := lipgloss.Width(line)
			pad := valW - visW
			if pad < 0 {
				pad = 0
			}
			midLines = append(midLines, bdr.Render("│")+" "+line+strings.Repeat(" ", pad)+" "+bdr.Render("│"))
		}
		emptyLine := bdr.Render("│") + " " + strings.Repeat(" ", valW) + " " + bdr.Render("│")
		for len(midLines) < contentLines {
			midLines = append(midLines, emptyLine)
		}

		bot := bdr.Render("╰" + strings.Repeat("─", innerW) + "╯")
		return top + "\n" + strings.Join(midLines, "\n") + "\n" + bot
	}

	// Not focused — render with markdown formatting
	text := ta.Value()
	if text == "" {
		text = "No description."
		return renderFieldMultiline(label, text, width, contentLines, d.scrollY, colorSubtle)
	}
	return renderMarkdownField(label, text, width, contentLines, d.scrollY)
}

// extractAfter returns the portion of an ANSI string starting at the given visual column,
// preserving any ANSI styling that was active at that column.
func extractAfter(s string, startCol int) string {
	var (
		vis       int
		i         int
		lastAnsi  string // track the last ANSI sequence seen before startCol
	)
	for i < len(s) {
		// Track ANSI escape sequences
		if s[i] == '\x1b' {
			j := i
			for j < len(s) && s[j] != 'm' {
				j++
			}
			if j < len(s) {
				j++ // include 'm'
			}
			if vis >= startCol {
				// We're past the start point, include this escape
				return lastAnsi + s[i:]
			}
			lastAnsi = s[i:j] // remember this sequence
			i = j
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		w := ansi.PrintableRuneWidth(string(r))
		vis += w
		i += size
		if vis >= startCol {
			return lastAnsi + s[i:]
		}
	}
	return ""
}

func joinFieldsHorizontal(fields ...string) string {
	return lipgloss.JoinHorizontal(lipgloss.Top, fields...)
}

// ── Details tab ───────────────────────────────────────────────

func (d Detail) renderDetailsTabWithOverlays() (string, []overlayInfo) {
	var b strings.Builder
	var overlays []overlayInfo
	w := d.width - 1
	gap := 0
	lineY := 0 // track current line position

	// Row 1: Key + Title (3 lines)
	keyW := d.keyFieldWidth()
	titleW := w - keyW - gap
	if titleW < 20 {
		titleW = 20
	}
	d.titleInput.Width = titleW - 6
	row1 := joinFieldsHorizontal(
		renderField("Key", d.issue.Key, keyW, colorAccent),
		d.renderInputField("Title", &d.titleInput, titleW),
	)
	b.WriteString(row1 + "\n")
	lineY += 3

	// Row 2: Assignee + Status + Type (3 lines)
	col3W := (w - 2*gap) / 3
	col3Rem := w - 3*col3W - 2*gap

	d.assigneeDrop.width = col3W
	d.statusDrop.width = col3W

	row2 := joinFieldsHorizontal(
		d.assigneeDrop.View(),
		d.statusDrop.View(),
		renderField("Type", d.issue.Type.Name, col3W+col3Rem, colorText),
	)
	b.WriteString(row2 + "\n")
	lineY += 3

	// Collect row2 overlays — use standalone (includes field header) for full overlay
	if overlay := d.assigneeDrop.RenderStandaloneOverlay(); overlay != nil {
		// Position so the field header aligns with row2's field position
		overlays = append(overlays, overlayInfo{lines: overlay, y: lineY - 3, x: 0})
	}
	if overlay := d.statusDrop.RenderStandaloneOverlay(); overlay != nil {
		overlays = append(overlays, overlayInfo{lines: overlay, y: lineY - 3, x: col3W})
	}

	// Row 3: Priority + Due Date + Reporter (3 lines)
	d.priorityDrop.width = col3W
	d.dueDatePick.width = col3W

	reporter := "—"
	if d.issue.Reporter != nil {
		reporter = d.issue.Reporter.DisplayName
	}

	row3 := joinFieldsHorizontal(
		d.priorityDrop.View(),
		d.dueDatePick.View(),
		renderField("Reporter", reporter, col3W+col3Rem, colorText),
	)
	b.WriteString(row3 + "\n")
	lineY += 3

	// Collect row3 overlays — use standalone for priority, regular for datepicker
	if overlay := d.priorityDrop.RenderStandaloneOverlay(); overlay != nil {
		overlays = append(overlays, overlayInfo{lines: overlay, y: lineY - 3, x: 0})
	}
	if overlay := d.dueDatePick.RenderOverlay(); overlay != nil {
		// Date picker doesn't have RenderStandaloneOverlay — field already renders with ├──┤
		overlays = append(overlays, overlayInfo{lines: overlay, y: lineY, x: col3W})
	}

	// Row 4: Parent + Created + Updated (3 lines)
	parentStr := "—"
	parentColor := colorSubtle
	if d.issue.Parent != nil {
		parentStr = d.issue.Parent.Key + " " + d.issue.Parent.Summary
		parentColor = colorAccent
	}

	updatedStr := d.issue.Updated.Format("2006-01-02 15:04")

	row4 := joinFieldsHorizontal(
		renderField("Parent", parentStr, col3W, parentColor),
		renderField("Created", d.issue.Created.Format("2006-01-02 15:04"), col3W, colorText),
		renderField("Updated", updatedStr, col3W+col3Rem, colorText),
	)
	b.WriteString(row4 + "\n")
	lineY += 3

	// Row 5: Sprint + Labels (optional, 3 lines)
	if d.issue.Sprint != "" || len(d.issue.Labels) > 0 {
		sprintW := col3W
		labelsW := w - sprintW - gap

		sprintVal := "—"
		if d.issue.Sprint != "" {
			sprintVal = d.issue.Sprint
		}

		labelsVal := "—"
		if len(d.issue.Labels) > 0 {
			tags := make([]string, len(d.issue.Labels))
			for i, l := range d.issue.Labels {
				tags[i] = "[" + l + "]"
			}
			labelsVal = strings.Join(tags, " ")
		}

		row5 := joinFieldsHorizontal(
			renderField("Sprint", sprintVal, sprintW, colorText),
			renderField("Labels", labelsVal, labelsW, colorInfo),
		)
		b.WriteString(row5 + "\n")
		lineY += 3
	}

	// Row 6: Description
	usedLines := 4 * 3
	if d.issue.Sprint != "" || len(d.issue.Labels) > 0 {
		usedLines += 3
	}
	availH := d.height - 2 - usedLines - 2
	if availH < 3 {
		availH = 3
	}

	b.WriteString(d.renderTextareaField("Description", &d.descInput, w, availH))
	b.WriteString("\n")

	return b.String(), overlays
}

// ── Helper ────────────────────────────────────────────────────

func dueDateColor(dueDate *time.Time) lipgloss.Color {
	if dueDate == nil {
		return colorSubtle
	}
	now := time.Now()
	if dueDate.Before(now) {
		return colorError
	} else if dueDate.Before(now.Add(7 * 24 * time.Hour)) {
		return colorWarning
	}
	return colorText
}

// ── Other tabs ────────────────────────────────────────────────

func (d Detail) renderCommentsTab() string {
	if len(d.issue.Comments) == 0 {
		subtle := lipgloss.NewStyle().Foreground(colorSubtle)
		return "  " + subtle.Render("No comments.")
	}

	authorStyle := lipgloss.NewStyle().Foreground(colorSuccess).Bold(true)
	timeStyle := lipgloss.NewStyle().Foreground(colorSubtle)
	dotStyle := lipgloss.NewStyle().Foreground(colorBorder)
	contentWidth := d.width - 4
	if contentWidth < 20 {
		contentWidth = 20
	}

	var parts []string
	for _, c := range d.issue.Comments {
		var cb strings.Builder
		cb.WriteString("  " + authorStyle.Render(c.Author.DisplayName))
		cb.WriteString("  " + timeStyle.Render(relativeTime(c.Created)))
		cb.WriteString("\n")
		cb.WriteString("  " + wordWrap(c.Body, contentWidth))
		parts = append(parts, cb.String())
	}

	sep := "\n  " + dotStyle.Render("·  ·  ·") + "\n"
	return strings.Join(parts, sep)
}

func (d Detail) renderSubtasksTab() string {
	if len(d.issue.Subtasks) == 0 {
		subtle := lipgloss.NewStyle().Foreground(colorSubtle)
		return "  " + subtle.Render("No subtasks.")
	}

	accentStyle := lipgloss.NewStyle().Foreground(colorAccent)
	textStyle := lipgloss.NewStyle().Foreground(colorText)

	var b strings.Builder
	for _, st := range d.issue.Subtasks {
		var indicator string
		switch st.Status.Name {
		case "Done":
			indicator = lipgloss.NewStyle().Foreground(colorSuccess).Render("✓")
		case "In Progress":
			indicator = lipgloss.NewStyle().Foreground(colorWarning).Render("●")
		default:
			indicator = lipgloss.NewStyle().Foreground(colorSubtle).Render("◦")
		}
		b.WriteString("  " + indicator + " " + accentStyle.Render(st.Key) + " " + textStyle.Render(st.Summary) + " " + StyledStatus(st.Status.Name) + "\n")
	}
	return b.String()
}

func (d Detail) renderLinksTab() string {
	if len(d.issue.Links) == 0 {
		subtle := lipgloss.NewStyle().Foreground(colorSubtle)
		return "  " + subtle.Render("No linked issues.")
	}

	typeStyle := lipgloss.NewStyle().Foreground(colorPurple)
	accentStyle := lipgloss.NewStyle().Foreground(colorAccent)
	textStyle := lipgloss.NewStyle().Foreground(colorText)

	var b strings.Builder
	for _, link := range d.issue.Links {
		relType := padRight(link.Type, 20)
		var issueKey, summary, status string
		if link.OutwardIssue != nil {
			issueKey = link.OutwardIssue.Key
			summary = link.OutwardIssue.Summary
			status = link.OutwardIssue.Status.Name
		} else if link.InwardIssue != nil {
			issueKey = link.InwardIssue.Key
			summary = link.InwardIssue.Summary
			status = link.InwardIssue.Status.Name
		}
		b.WriteString("  " + typeStyle.Render(relType) + " " + accentStyle.Render(issueKey) + " " + textStyle.Render(summary) + " " + StyledStatus(status) + "\n")
	}
	return b.String()
}

func (d Detail) renderAttachmentsTab() string {
	if len(d.issue.Attachments) == 0 {
		subtle := lipgloss.NewStyle().Foreground(colorSubtle)
		return "  " + subtle.Render("No attachments.")
	}

	nameStyle := lipgloss.NewStyle().Foreground(colorAccent)
	sizeStyle := lipgloss.NewStyle().Foreground(colorSubtle)
	typeStyle := lipgloss.NewStyle().Foreground(colorInfo)

	var b strings.Builder
	for _, att := range d.issue.Attachments {
		sizeStr := formatFileSize(att.Size)
		b.WriteString("  " + nameStyle.Render(att.Filename) + "  " + sizeStyle.Render(sizeStr) + "  " + typeStyle.Render(att.MimeType) + "\n")
	}
	return b.String()
}

func formatFileSize(bytes int) string {
	switch {
	case bytes >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	case bytes >= 1024:
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func wordWrap(s string, width int) string {
	if width <= 0 {
		return s
	}
	var result strings.Builder
	for _, paragraph := range strings.Split(s, "\n") {
		if paragraph == "" {
			result.WriteString("\n")
			continue
		}
		words := strings.Fields(paragraph)
		if len(words) == 0 {
			result.WriteString("\n")
			continue
		}
		lineLen := 0
		for i, w := range words {
			wLen := len(w)
			if i == 0 {
				result.WriteString(w)
				lineLen = wLen
			} else if lineLen+1+wLen > width {
				result.WriteString("\n" + w)
				lineLen = wLen
			} else {
				result.WriteString(" " + w)
				lineLen += 1 + wLen
			}
		}
		result.WriteString("\n")
	}
	return strings.TrimRight(result.String(), "\n")
}
