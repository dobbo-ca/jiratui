package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/christopherdobbyn/jiratui/internal/models"
	"github.com/pkg/browser"
)

// openIssueMsg is sent when the user selects an issue to open in detail view.
type openIssueMsg struct {
	issueKey string
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
	filter    textinput.Model
	filtering bool
	issues    []models.Issue
	filtered  []models.Issue
	cursor    int
	offset    int
	width     int
	height    int
}

// NewList creates a new list model with the given issues.
func NewList(issues []models.Issue, width, height int) List {
	ti := textinput.New()
	ti.Placeholder = "Filter by key or summary..."
	ti.CharLimit = 100

	return List{
		issues:   issues,
		filtered: issues,
		filter:   ti,
		width:    width,
		height:   height,
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
	// Reserve: status bar(1) + header(1) + separator(1) + help bar(1) + padding(1)
	h := l.height - 5
	if l.filtering {
		h--
	}
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
func priorityColor(priority string) lipgloss.Color {
	switch priority {
	case "Highest":
		return colorError
	case "High":
		return lipgloss.Color("#ff9e64")
	case "Medium":
		return colorWarning
	case "Low":
		return colorSuccess
	case "Lowest":
		return colorAccent
	default:
		return colorSubtle
	}
}

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

func filterIssues(issues []models.Issue, query string) []models.Issue {
	if query == "" {
		return issues
	}
	q := strings.ToLower(query)
	var result []models.Issue
	for _, issue := range issues {
		if strings.Contains(strings.ToLower(issue.Key), q) ||
			strings.Contains(strings.ToLower(issue.Summary), q) {
			result = append(result, issue)
		}
	}
	return result
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
		if l.filtering {
			switch {
			case key.Matches(msg, listKeys.Escape):
				l.filtering = false
				l.filter.SetValue("")
				l.filter.Blur()
				l.filtered = l.issues
				l.cursor = 0
				l.offset = 0
				return l, nil
			case msg.Type == tea.KeyEnter:
				l.filtering = false
				l.filter.Blur()
				return l, nil
			default:
				var cmd tea.Cmd
				l.filter, cmd = l.filter.Update(msg)
				l.filtered = filterIssues(l.issues, l.filter.Value())
				l.cursor = 0
				l.offset = 0
				return l, cmd
			}
		}

		switch {
		case key.Matches(msg, listKeys.Filter):
			l.filtering = true
			l.filter.Focus()
			return l, l.filter.Cursor.BlinkCmd()
		case key.Matches(msg, listKeys.Open):
			if l.cursor < len(l.filtered) {
				_ = browser.OpenURL(l.filtered[l.cursor].BrowseURL)
			}
			return l, nil
		case key.Matches(msg, listKeys.Enter):
			if l.cursor < len(l.filtered) {
				return l, func() tea.Msg {
					return openIssueMsg{issueKey: l.filtered[l.cursor].Key}
				}
			}
			return l, nil
		case key.Matches(msg, listKeys.Down):
			if l.cursor < len(l.filtered)-1 {
				l.cursor++
				l.clampCursor()
			}
			return l, nil
		case key.Matches(msg, listKeys.Up):
			if l.cursor > 0 {
				l.cursor--
				l.clampCursor()
			}
			return l, nil
		}

	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelDown:
			if l.cursor < len(l.filtered)-1 {
				l.cursor++
				l.clampCursor()
			}
			return l, nil
		case tea.MouseButtonWheelUp:
			if l.cursor > 0 {
				l.cursor--
				l.clampCursor()
			}
			return l, nil
		case tea.MouseButtonLeft:
			// Header takes 2 lines, filter takes 1 if active
			headerOffset := 2
			if l.filtering {
				headerOffset++
			}
			clickedRow := msg.Y - headerOffset + l.offset
			if clickedRow >= 0 && clickedRow < len(l.filtered) {
				if l.cursor == clickedRow {
					return l, func() tea.Msg {
						return openIssueMsg{issueKey: l.filtered[l.cursor].Key}
					}
				}
				l.cursor = clickedRow
				l.clampCursor()
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

	if l.filtering {
		filterStyle := lipgloss.NewStyle().
			Foreground(colorAccent).
			PaddingLeft(1)
		b.WriteString(filterStyle.Render("/ "))
		b.WriteString(l.filter.View())
		b.WriteString("\n")
	}

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

	// Simplified header for narrow pane
	headerStyle := lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	keyW := 12
	statusW := 12
	summaryW := width - keyW - statusW - 3
	if summaryW < 10 {
		summaryW = 10
	}
	header := fmt.Sprintf(" %s %s %s",
		padRight("Key", keyW),
		padRight("Status", statusW),
		padRight("Summary", summaryW),
	)
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(colorBorder).Render(strings.Repeat("─", width)))
	b.WriteString("\n")

	// Rows
	vis := height - 2 // header + separator
	if vis < 1 {
		vis = 1
	}
	end := l.offset + vis
	if end > len(l.filtered) {
		end = len(l.filtered)
	}

	for i := l.offset; i < end; i++ {
		issue := l.filtered[i]
		row := fmt.Sprintf(" %s %s %s",
			padRight(truncStr(issue.Key, keyW), keyW),
			padRight(truncStr(issue.Status.Name, statusW), statusW),
			padRight(truncStr(issue.Summary, summaryW), summaryW),
		)

		if i == l.cursor {
			b.WriteString(lipgloss.NewStyle().Foreground(colorText).Background(colorSelection).Render(row))
		} else {
			b.WriteString(lipgloss.NewStyle().Foreground(colorText).Render(row))
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
	l.filtered = filterIssues(issues, l.filter.Value())
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
