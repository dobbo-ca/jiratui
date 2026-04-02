package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/christopherdobbyn/jiratui/internal/models"
	"github.com/pkg/browser"
)

// Column indices for issueToRow output.
const (
	colKey      = 0
	colPriority = 1
	colStatus   = 2
	colAssignee = 3
	colUpdated  = 4
	colSummary  = 5
)

// List is the Bubble Tea model for the issue list view.
type List struct {
	table     table.Model
	filter    textinput.Model
	filtering bool
	issues    []models.Issue
	filtered  []models.Issue
	width     int
	height    int
}

// NewList creates a new list model with the given issues.
func NewList(issues []models.Issue, width, height int) List {
	ti := textinput.New()
	ti.Placeholder = "Filter by key or summary..."
	ti.CharLimit = 100

	l := List{
		issues:   issues,
		filtered: issues,
		filter:   ti,
		width:    width,
		height:   height,
	}
	l.table = l.buildTable()
	return l
}

func (l *List) buildTable() table.Model {
	columns := []table.Column{
		{Title: "Key", Width: 12},
		{Title: "P", Width: 3},
		{Title: "Status", Width: 14},
		{Title: "Assignee", Width: 16},
		{Title: "Updated", Width: 10},
		{Title: "Summary", Width: l.summaryWidth()},
	}

	rows := make([]table.Row, len(l.filtered))
	for i, issue := range l.filtered {
		rows[i] = issueToRow(issue)
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(l.tableHeight()),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorBorder).
		BorderBottom(true).
		Bold(true).
		Foreground(colorAccent)
	s.Selected = s.Selected.
		Foreground(colorText).
		Background(lipgloss.Color("#292e42")).
		Bold(false)
	t.SetStyles(s)

	return t
}

func (l *List) summaryWidth() int {
	// Key(12) + P(3) + Status(14) + Assignee(16) + Updated(10) + gaps(5*3=15) = 70
	w := l.width - 70
	if w < 20 {
		w = 20
	}
	return w
}

func (l *List) tableHeight() int {
	// Reserve: status bar(1) + filter(1 if active) + help bar(1) + table header(2) + padding(1)
	h := l.height - 5
	if l.filtering {
		h--
	}
	if h < 3 {
		h = 3
	}
	return h
}

func issueToRow(issue models.Issue) table.Row {
	assignee := "-"
	if issue.Assignee != nil {
		assignee = issue.Assignee.DisplayName
	}
	return table.Row{
		issue.Key,
		PriorityIcon(issue.Priority.Name),
		issue.Status.Name,
		assignee,
		relativeTime(issue.Updated),
		issue.Summary,
	}
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
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if l.filtering {
			switch {
			case key.Matches(msg, listKeys.Escape):
				l.filtering = false
				l.filter.SetValue("")
				l.filter.Blur()
				l.filtered = l.issues
				l.table = l.buildTable()
				return l, nil
			case msg.Type == tea.KeyEnter:
				l.filtering = false
				l.filter.Blur()
				return l, nil
			default:
				var cmd tea.Cmd
				l.filter, cmd = l.filter.Update(msg)
				cmds = append(cmds, cmd)
				l.filtered = filterIssues(l.issues, l.filter.Value())
				l.table = l.buildTable()
				return l, tea.Batch(cmds...)
			}
		}

		switch {
		case key.Matches(msg, listKeys.Filter):
			l.filtering = true
			l.filter.Focus()
			l.table = l.buildTable()
			return l, l.filter.Cursor.BlinkCmd()
		case key.Matches(msg, listKeys.Open):
			if idx := l.table.Cursor(); idx < len(l.filtered) {
				_ = browser.OpenURL(l.filtered[idx].BrowseURL)
			}
			return l, nil
		case key.Matches(msg, listKeys.Up), key.Matches(msg, listKeys.Down):
			var cmd tea.Cmd
			l.table, cmd = l.table.Update(msg)
			return l, cmd
		}

	case tea.WindowSizeMsg:
		l.width = msg.Width
		l.height = msg.Height
		l.table = l.buildTable()
		return l, nil
	}

	var cmd tea.Cmd
	l.table, cmd = l.table.Update(msg)
	cmds = append(cmds, cmd)
	return l, tea.Batch(cmds...)
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

	b.WriteString(l.table.View())

	return b.String()
}

// SetIssues replaces the issue data and rebuilds the table.
func (l *List) SetIssues(issues []models.Issue) {
	l.issues = issues
	l.filtered = filterIssues(issues, l.filter.Value())
	l.table = l.buildTable()
}

// SelectedIssue returns the currently selected issue, if any.
func (l List) SelectedIssue() *models.Issue {
	idx := l.table.Cursor()
	if idx >= 0 && idx < len(l.filtered) {
		return &l.filtered[idx]
	}
	return nil
}
