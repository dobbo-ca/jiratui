package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/christopherdobbyn/jiratui/internal/models"
	"github.com/pkg/browser"
)

// cursorChangedMsg is sent when the selected issue changes in the list.
type cursorChangedMsg struct {
	issueKey string
}

// sortClickMsg is sent when a column header is clicked to sort.
type sortClickMsg struct {
	column string // "key" or "summary"
}

func (l List) emitCursorChanged() tea.Cmd {
	if l.cursor < len(l.filtered) {
		key := l.filtered[l.cursor].Key
		return func() tea.Msg {
			return cursorChangedMsg{issueKey: key}
		}
	}
	return nil
}

// Column widths.
const (
	colWPriority = 2
	colWKey      = 12
	colWStatus   = 14
	colWAssignee = 16
	colWUpdated  = 10
	colGap       = 1
	fixedCols    = colWPriority + colWKey + colWStatus + colWAssignee + colWUpdated + 5*colGap
)

// List is the Bubble Tea model for the issue list view.
type List struct {
	issues      []models.Issue
	filtered    []models.Issue
	cursor      int
	offset      int
	width       int
	height      int
	keyColWidth int    // adjustable key column width for split view
	draggingCol bool   // true while dragging column border
	sortCol     string // "key", "summary", or "" (for default/updated)
	sortAsc     bool   // sort direction
}

// maxKeyWidth returns the visual width of the longest issue key.
func maxKeyWidth(issues []models.Issue) int {
	w := 3 // minimum: "Key"
	for _, issue := range issues {
		if len(issue.Key) > w {
			w = len(issue.Key)
		}
	}
	return w
}

// NewList creates a new list model with the given issues.
func NewList(issues []models.Issue, width, height int) List {
	return List{
		issues:      issues,
		filtered:    issues,
		width:       width,
		height:      height,
		keyColWidth: maxKeyWidth(issues) + 2,
	}
}

func (l *List) summaryWidth() int {
	w := l.width - fixedCols
	if w < 20 {
		w = 20
	}
	return w
}

func (l *List) visibleRows() int {
	h := l.height - 5
	if h < 1 {
		h = 1
	}
	return h
}

func (l *List) clampCursor() {
	if len(l.filtered) == 0 {
		l.cursor = 0
		l.offset = 0
		return
	}
	if l.cursor >= len(l.filtered) {
		l.cursor = len(l.filtered) - 1
	}
	if l.cursor < 0 {
		l.cursor = 0
	}
	vis := l.visibleRows()
	if l.cursor < l.offset {
		l.offset = l.cursor
	}
	if l.cursor >= l.offset+vis {
		l.offset = l.cursor - vis + 1
	}
	if l.offset < 0 {
		l.offset = 0
	}
}

// priorityColor returns the color for a priority level.
func rowColor(issue models.Issue) lipgloss.Color {
	if issue.DueDate == nil {
		return colorText
	}
	now := time.Now()
	if issue.DueDate.Before(now) {
		return colorError // red — overdue
	}
	if issue.DueDate.Before(now.Add(7 * 24 * time.Hour)) {
		return colorWarning // yellow — due within a week
	}
	return colorText
}

func truncStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 1 {
		return s[:max]
	}
	return s[:max-1] + "…"
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

func (l List) renderHeader() string {
	style := lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	header := fmt.Sprintf("%s %s %s %s %s %s",
		padRight("P", colWPriority),
		padRight("Key", colWKey),
		padRight("Status", colWStatus),
		padRight("Assignee", colWAssignee),
		padRight("Updated", colWUpdated),
		padRight("Summary", l.summaryWidth()),
	)
	return style.Render(header)
}

func (l List) renderSeparator() string {
	style := lipgloss.NewStyle().Foreground(colorBorder)
	return style.Render(strings.Repeat("─", l.width))
}

func (l List) renderRow(issue models.Issue, selected bool) string {
	assignee := "-"
	if issue.Assignee != nil {
		assignee = issue.Assignee.DisplayName
	}

	sw := l.summaryWidth()
	text := fmt.Sprintf(" %s %s %s %s %s",
		padRight(truncStr(issue.Key, colWKey), colWKey),
		padRight(truncStr(issue.Status.Name, colWStatus), colWStatus),
		padRight(truncStr(assignee, colWAssignee), colWAssignee),
		padRight(truncStr(relativeTime(issue.Updated), colWUpdated), colWUpdated),
		padRight(truncStr(issue.Summary, sw), sw),
	)

	urgency := rowColor(issue)
	pColor := priorityColor(issue.Priority.Name)

	if selected {
		prio := lipgloss.NewStyle().Foreground(pColor).Background(colorSelection).Render(padRight("●", colWPriority))
		rest := lipgloss.NewStyle().Foreground(urgency).Background(colorSelection).Render(text)
		return prio + rest
	}

	prio := lipgloss.NewStyle().Foreground(pColor).Render(padRight("●", colWPriority))
	rest := lipgloss.NewStyle().Foreground(urgency).Render(text)
	return prio + rest
}

func relativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return fmt.Sprintf("%dw ago", int(d.Hours()/(24*7)))
	}
}

// Init satisfies tea.Model.
func (l List) Init() tea.Cmd {
	return nil
}

// Update handles messages for the list view.
func (l List) Update(msg tea.Msg) (List, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, listKeys.Open):
			if l.cursor < len(l.filtered) {
				_ = browser.OpenURL(l.filtered[l.cursor].BrowseURL)
			}
			return l, nil
		case key.Matches(msg, listKeys.Down):
			prev := l.cursor
			if l.cursor < len(l.filtered)-1 {
				l.cursor++
				l.clampCursor()
			}
			if l.cursor != prev {
				return l, l.emitCursorChanged()
			}
			return l, nil
		case key.Matches(msg, listKeys.Up):
			prev := l.cursor
			if l.cursor > 0 {
				l.cursor--
				l.clampCursor()
			}
			if l.cursor != prev {
				return l, l.emitCursorChanged()
			}
			return l, nil
		case key.Matches(msg, listKeys.PageDown):
			prev := l.cursor
			vis := l.visibleRows()
			l.cursor += vis
			if l.cursor >= len(l.filtered) {
				l.cursor = len(l.filtered) - 1
			}
			l.clampCursor()
			if l.cursor != prev {
				return l, l.emitCursorChanged()
			}
			return l, nil
		case key.Matches(msg, listKeys.PageUp):
			prev := l.cursor
			vis := l.visibleRows()
			l.cursor -= vis
			if l.cursor < 0 {
				l.cursor = 0
			}
			l.clampCursor()
			if l.cursor != prev {
				return l, l.emitCursorChanged()
			}
			return l, nil
		}

	case tea.MouseMsg:
		// Handle column drag motion
		if l.draggingCol {
			if msg.Action == tea.MouseActionMotion {
				newW := msg.X // X is relative to the list pane
				if newW < 8 {
					newW = 8
				}
				if newW > 30 {
					newW = 30
				}
				l.keyColWidth = newW
				return l, nil
			}
			if msg.Action == tea.MouseActionRelease {
				l.draggingCol = false
				return l, nil
			}
		}

		switch msg.Button {
		case tea.MouseButtonWheelDown:
			maxOffset := len(l.filtered) - l.visibleRows()
			if maxOffset < 0 {
				maxOffset = 0
			}
			if l.offset < maxOffset {
				l.offset++
			}
			return l, nil
		case tea.MouseButtonWheelUp:
			if l.offset > 0 {
				l.offset--
			}
			return l, nil
		case tea.MouseButtonLeft:
			colBorderX := l.keyColWidth + 1 // 1 for leading space

			// Column border drag: click near the key/summary boundary on header row
			if msg.Action == tea.MouseActionPress && msg.Y <= 1 && msg.X >= colBorderX-1 && msg.X <= colBorderX+1 {
				l.draggingCol = true
				return l, nil
			}

			// Header sort click (row 0, not near column border)
			if msg.Y == 0 && msg.Action == tea.MouseActionPress {
				if msg.X < colBorderX-1 {
					return l, func() tea.Msg { return sortClickMsg{column: "key"} }
				}
				return l, func() tea.Msg { return sortClickMsg{column: "summary"} }
			}

			headerOffset := 2
			clickedRow := msg.Y - headerOffset + l.offset
			if clickedRow >= 0 && clickedRow < len(l.filtered) {
				prev := l.cursor
				l.cursor = clickedRow
				l.clampCursor()
				if l.cursor != prev {
					return l, l.emitCursorChanged()
				}
			}
			return l, nil
		case tea.MouseButtonNone:
			if msg.Action == tea.MouseActionRelease {
				l.draggingCol = false
			}
			return l, nil
		}

	case tea.WindowSizeMsg:
		l.width = msg.Width
		l.height = msg.Height
		l.clampCursor()
		return l, nil
	}

	return l, nil
}

// View renders the list.
func (l List) View() string {
	var b strings.Builder

	b.WriteString(l.renderHeader())
	b.WriteString("\n")
	b.WriteString(l.renderSeparator())
	b.WriteString("\n")

	vis := l.visibleRows()
	end := l.offset + vis
	if end > len(l.filtered) {
		end = len(l.filtered)
	}

	for i := l.offset; i < end; i++ {
		b.WriteString(l.renderRow(l.filtered[i], i == l.cursor))
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// ViewWithWidth renders the list at a specific width and height (for split layout).
func (l List) ViewWithWidth(width, height int) string {
	var b strings.Builder

	keyW := l.keyColWidth
	if keyW < 8 {
		keyW = 8
	}
	if keyW > width-10 {
		keyW = width - 10
	}
	summaryW := width - keyW - 3 // leading space + divider + space
	if summaryW < 10 {
		summaryW = 10
	}

	divStyle := lipgloss.NewStyle().Foreground(colorBorder)
	div := divStyle.Render("│")

	// Header with sort indicators
	headerStyle := lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	keyLabel := "Key"
	sumLabel := "Summary"
	if l.sortCol == "key" {
		if l.sortAsc {
			keyLabel = "Key ▲"
		} else {
			keyLabel = "Key ▼"
		}
	}
	if l.sortCol == "summary" {
		if l.sortAsc {
			sumLabel = "Summary ▲"
		} else {
			sumLabel = "Summary ▼"
		}
	}
	keyHeader := headerStyle.Render(" " + padRight(keyLabel, keyW))
	sumHeader := headerStyle.Render(" " + padRight(sumLabel, summaryW))
	b.WriteString(keyHeader + div + sumHeader)
	b.WriteString("\n")

	// Separator with ┼ at the column boundary
	sepLeft := strings.Repeat("─", keyW+1)
	sepRight := strings.Repeat("─", width-keyW-2)
	b.WriteString(divStyle.Render(sepLeft + "┼" + sepRight))
	b.WriteString("\n")

	// Rows
	vis := height - 2
	if vis < 1 {
		vis = 1
	}
	end := l.offset + vis
	if end > len(l.filtered) {
		end = len(l.filtered)
	}

	for i := l.offset; i < end; i++ {
		issue := l.filtered[i]
		keyText := " " + padRight(truncStr(issue.Key, keyW), keyW)
		sumText := " " + padRight(truncStr(issue.Summary, summaryW), summaryW)

		if i == l.cursor {
			sel := lipgloss.NewStyle().Foreground(colorText).Background(colorSelection)
			b.WriteString(sel.Render(keyText) + div + sel.Render(sumText))
		} else {
			urgency := rowColor(issue)
			rowStyle := lipgloss.NewStyle().Foreground(urgency)
			b.WriteString(rowStyle.Render(keyText) + div + rowStyle.Render(sumText))
		}
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// SetIssues replaces the issue data and rebuilds.
func (l *List) SetIssues(issues []models.Issue) {
	l.issues = issues
	l.filtered = issues
	l.cursor = 0
	l.offset = 0
}

// SelectedIssue returns the currently selected issue, if any.
func (l List) SelectedIssue() *models.Issue {
	if l.cursor >= 0 && l.cursor < len(l.filtered) {
		return &l.filtered[l.cursor]
	}
	return nil
}
