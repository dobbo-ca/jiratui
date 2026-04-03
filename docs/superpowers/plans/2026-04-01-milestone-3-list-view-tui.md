# Milestone 3: List View TUI — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a Bubble Tea TUI that displays Jira issues in an interactive table with keyboard/mouse navigation, quick filter, loading spinner, and status/help bars.

**Architecture:** The TUI layer lives in `internal/tui/` with an Elm-architecture Bubble Tea app. A root `App` model manages state transitions (loading → list), delegates to a `List` model for the issue table. Styles, key bindings, and the list view are separate files. The default CLI command (no args) launches the TUI; existing subcommands (`auth`, `issues`) remain.

**Tech Stack:** `charmbracelet/bubbletea` (TUI framework), `charmbracelet/lipgloss` (styling), `charmbracelet/bubbles` (table, spinner, textinput), `pkg/browser` (open URLs)

---

## File Structure

```
internal/tui/
├── theme.go       # Tokyo Night color constants + Lip Gloss style builders
├── keys.go        # Key binding definitions (help.KeyMap interface)
├── list.go        # List model: table, filter, row rendering, navigation
├── app.go         # Root model: state machine (loading → list), status bar, help bar
cmd/
└── root.go        # Modified: default command launches TUI
```

---

### Task 1: Add TUI Dependencies

**Files:**
- Modify: `go.mod`

- [ ] **Step 1: Install bubbletea, lipgloss, bubbles, and browser opener**

```bash
cd /Users/christopherdobbyn/work/dobbo-ca/jiratui
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/lipgloss@latest
go get github.com/charmbracelet/bubbles@latest
go get github.com/pkg/browser@latest
```

- [ ] **Step 2: Verify deps resolve**

Run: `go mod tidy && go build ./...`
Expected: clean build, no errors

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add bubbletea, lipgloss, bubbles, browser deps for TUI"
```

---

### Task 2: Tokyo Night Theme

**Files:**
- Create: `internal/tui/theme.go`
- Test: `internal/tui/theme_test.go`

- [ ] **Step 1: Write the test**

```go
package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestThemeColorsAreDefined(t *testing.T) {
	// Verify all theme colors are non-empty and parse as valid lipgloss colors
	colors := map[string]lipgloss.Color{
		"Background":  colorBackground,
		"HeaderBg":    colorHeaderBg,
		"Border":      colorBorder,
		"Text":        colorText,
		"Accent":      colorAccent,
		"Success":     colorSuccess,
		"Warning":     colorWarning,
		"Error":       colorError,
		"Info":        colorInfo,
		"Subtle":      colorSubtle,
		"Purple":      colorPurple,
	}

	for name, c := range colors {
		if string(c) == "" {
			t.Errorf("color %s is empty", name)
		}
	}
}

func TestPriorityStyle(t *testing.T) {
	tests := []struct {
		priority string
		wantIcon string
	}{
		{"Highest", "⏫"},
		{"High", "🔼"},
		{"Medium", "▶️"},
		{"Low", "🔽"},
		{"Lowest", "⏬"},
		{"Unknown", "•"},
	}
	for _, tt := range tests {
		t.Run(tt.priority, func(t *testing.T) {
			got := PriorityIcon(tt.priority)
			if got != tt.wantIcon {
				t.Errorf("PriorityIcon(%q) = %q, want %q", tt.priority, got, tt.wantIcon)
			}
		})
	}
}

func TestStatusStyle(t *testing.T) {
	// Status styling returns a rendered string — just verify it's not empty
	tests := []string{"To Do", "In Progress", "In Review", "Done", "Unknown"}
	for _, status := range tests {
		t.Run(status, func(t *testing.T) {
			got := StyledStatus(status)
			if got == "" {
				t.Errorf("StyledStatus(%q) returned empty string", status)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/christopherdobbyn/work/dobbo-ca/jiratui && go test ./internal/tui/ -run TestTheme -v`
Expected: FAIL — package doesn't exist yet

- [ ] **Step 3: Write the implementation**

```go
package tui

import "github.com/charmbracelet/lipgloss"

// Tokyo Night dark theme colors.
var (
	colorBackground = lipgloss.Color("#1a1b26")
	colorHeaderBg   = lipgloss.Color("#16161e")
	colorBorder     = lipgloss.Color("#292e42")
	colorText       = lipgloss.Color("#c9d1d9")
	colorAccent     = lipgloss.Color("#7aa2f7") // blue
	colorSuccess    = lipgloss.Color("#9ece6a") // green
	colorWarning    = lipgloss.Color("#e0af68") // amber
	colorError      = lipgloss.Color("#f7768e") // red
	colorInfo       = lipgloss.Color("#7dcfff") // cyan
	colorSubtle     = lipgloss.Color("#565f89") // gray
	colorPurple     = lipgloss.Color("#bb9af7")
)

// PriorityIcon returns an icon for the given Jira priority name.
func PriorityIcon(priority string) string {
	switch priority {
	case "Highest":
		return "⏫"
	case "High":
		return "🔼"
	case "Medium":
		return "▶️"
	case "Low":
		return "🔽"
	case "Lowest":
		return "⏬"
	default:
		return "•"
	}
}

// StyledStatus returns a color-coded status string.
func StyledStatus(status string) string {
	var color lipgloss.Color
	switch status {
	case "Done":
		color = colorSuccess
	case "In Progress":
		color = colorWarning
	case "In Review":
		color = colorPurple
	case "To Do":
		color = colorSubtle
	default:
		color = colorText
	}
	return lipgloss.NewStyle().Foreground(color).Render(status)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/christopherdobbyn/work/dobbo-ca/jiratui && go test ./internal/tui/ -run "TestTheme|TestPriority|TestStatus" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tui/theme.go internal/tui/theme_test.go
git commit -m "feat(tui): add Tokyo Night theme colors and priority/status styling"
```

---

### Task 3: Key Bindings

**Files:**
- Create: `internal/tui/keys.go`

- [ ] **Step 1: Write key bindings**

```go
package tui

import "github.com/charmbracelet/bubbles/key"

// ListKeyMap defines key bindings for the list view.
type ListKeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Filter  key.Binding
	Refresh key.Binding
	Open    key.Binding
	Quit    key.Binding
	Escape  key.Binding
}

var listKeys = ListKeyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Filter: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "filter"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh"),
	),
	Open: key.NewBinding(
		key.WithKeys("o"),
		key.WithHelp("o", "open in browser"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Escape: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "clear filter"),
	),
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/christopherdobbyn/work/dobbo-ca/jiratui && go build ./internal/tui/`
Expected: clean build

- [ ] **Step 3: Commit**

```bash
git add internal/tui/keys.go
git commit -m "feat(tui): add list view key bindings"
```

---

### Task 4: List Model — Table, Filter, Row Rendering

**Files:**
- Create: `internal/tui/list.go`
- Test: `internal/tui/list_test.go`

- [ ] **Step 1: Write tests for row rendering and filtering**

```go
package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/christopherdobbyn/jiratui/internal/models"
)

func TestIssueToRow(t *testing.T) {
	issue := models.Issue{
		Key:      "PROJ-123",
		Summary:  "Fix the login bug",
		Priority: models.Priority{Name: "High"},
		Status:   models.Status{Name: "In Progress"},
		Assignee: &models.User{DisplayName: "Alice"},
		Updated:  time.Now().Add(-2 * time.Hour),
	}

	row := issueToRow(issue)

	if row[colKey] != "PROJ-123" {
		t.Errorf("key = %q, want %q", row[colKey], "PROJ-123")
	}
	if row[colPriority] != "🔼" {
		t.Errorf("priority = %q, want %q", row[colPriority], "🔼")
	}
	if row[colAssignee] != "Alice" {
		t.Errorf("assignee = %q, want %q", row[colAssignee], "Alice")
	}
	if row[colSummary] != "Fix the login bug" {
		t.Errorf("summary = %q, want %q", row[colSummary], "Fix the login bug")
	}
}

func TestIssueToRowUnassigned(t *testing.T) {
	issue := models.Issue{
		Key:     "PROJ-456",
		Summary: "Unassigned task",
	}

	row := issueToRow(issue)

	if row[colAssignee] != "-" {
		t.Errorf("assignee = %q, want %q", row[colAssignee], "-")
	}
}

func TestFilterIssues(t *testing.T) {
	issues := []models.Issue{
		{Key: "PROJ-1", Summary: "Fix login bug", Status: models.Status{Name: "To Do"}},
		{Key: "PROJ-2", Summary: "Add dashboard", Status: models.Status{Name: "In Progress"}},
		{Key: "PROJ-3", Summary: "Update API docs", Status: models.Status{Name: "Done"}},
	}

	tests := []struct {
		query string
		want  int
	}{
		{"", 3},            // empty filter returns all
		{"login", 1},       // matches summary
		{"PROJ-2", 1},      // matches key
		{"proj", 3},        // case-insensitive key match
		{"dashboard", 1},   // matches summary
		{"nonexistent", 0}, // no match
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got := filterIssues(issues, tt.query)
			if len(got) != tt.want {
				t.Errorf("filterIssues(%q) returned %d issues, want %d", tt.query, len(got), tt.want)
			}
		})
	}
}

func TestRelativeTime(t *testing.T) {
	tests := []struct {
		dur  time.Duration
		want string
	}{
		{30 * time.Second, "just now"},
		{5 * time.Minute, "5m ago"},
		{2 * time.Hour, "2h ago"},
		{36 * time.Hour, "1d ago"},
		{14 * 24 * time.Hour, "2w ago"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := relativeTime(time.Now().Add(-tt.dur))
			if !strings.Contains(got, strings.TrimSuffix(tt.want, " ago")) && got != tt.want {
				t.Errorf("relativeTime(-%v) = %q, want %q", tt.dur, got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/christopherdobbyn/work/dobbo-ca/jiratui && go test ./internal/tui/ -run "TestIssueToRow|TestFilter|TestRelativeTime" -v`
Expected: FAIL — functions not defined

- [ ] **Step 3: Write the list model implementation**

```go
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
	table      table.Model
	filter     textinput.Model
	filtering  bool
	issues     []models.Issue
	filtered   []models.Issue
	width      int
	height     int
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/christopherdobbyn/work/dobbo-ca/jiratui && go test ./internal/tui/ -run "TestIssueToRow|TestFilter|TestRelativeTime" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tui/list.go internal/tui/list_test.go
git commit -m "feat(tui): add list model with table, filter, and row rendering"
```

---

### Task 5: Root App Model — State Machine, Status Bar, Help Bar

**Files:**
- Create: `internal/tui/app.go`

- [ ] **Step 1: Write the app model**

```go
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/christopherdobbyn/jiratui/internal/jira"
	"github.com/christopherdobbyn/jiratui/internal/models"
)

type state int

const (
	stateLoading state = iota
	stateList
)

// issuesMsg carries fetched issues into the model.
type issuesMsg struct {
	issues []models.Issue
}

// errMsg carries errors into the model.
type errMsg struct {
	err error
}

// App is the root Bubble Tea model.
type App struct {
	state       state
	list        List
	spinner     spinner.Model
	client      *jira.Client
	profileName string
	err         error
	width       int
	height      int
}

// NewApp creates the root TUI model.
func NewApp(client *jira.Client, profileName string) App {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(colorAccent)

	return App{
		state:       stateLoading,
		spinner:     s,
		client:      client,
		profileName: profileName,
	}
}

func fetchIssues(client *jira.Client) tea.Cmd {
	return func() tea.Msg {
		result, err := client.SearchMyIssues(50, "")
		if err != nil {
			return errMsg{err: err}
		}
		return issuesMsg{issues: result.Issues}
	}
}

// Init starts the spinner and fires the initial data fetch.
func (a App) Init() tea.Cmd {
	return tea.Batch(a.spinner.Tick, fetchIssues(a.client))
}

// Update handles all messages.
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		if a.state == stateList {
			var cmd tea.Cmd
			a.list, cmd = a.list.Update(msg)
			return a, cmd
		}
		return a, nil

	case tea.KeyMsg:
		// Global quit — always works
		if msg.String() == "ctrl+c" {
			return a, tea.Quit
		}
		if a.state == stateList {
			// Quit only when not filtering
			if msg.String() == "q" && !a.list.filtering {
				return a, tea.Quit
			}
			// Refresh
			if msg.String() == "r" && !a.list.filtering {
				a.state = stateLoading
				a.err = nil
				return a, tea.Batch(a.spinner.Tick, fetchIssues(a.client))
			}
		}

	case issuesMsg:
		a.list = NewList(msg.issues, a.width, a.height)
		a.state = stateList
		return a, nil

	case errMsg:
		a.err = msg.err
		a.state = stateList
		return a, nil

	case spinner.TickMsg:
		if a.state == stateLoading {
			var cmd tea.Cmd
			a.spinner, cmd = a.spinner.Update(msg)
			return a, cmd
		}
		return a, nil
	}

	if a.state == stateList {
		var cmd tea.Cmd
		a.list, cmd = a.list.Update(msg)
		return a, cmd
	}

	return a, nil
}

// View renders the full app.
func (a App) View() string {
	var b strings.Builder

	// Status bar
	b.WriteString(a.renderStatusBar())
	b.WriteString("\n")

	// Main content
	switch a.state {
	case stateLoading:
		loadingStyle := lipgloss.NewStyle().
			PaddingTop(a.height/2 - 2).
			PaddingLeft(a.width/2 - 12).
			Foreground(colorText)
		b.WriteString(loadingStyle.Render(a.spinner.View() + " Fetching issues..."))
	case stateList:
		if a.err != nil {
			errStyle := lipgloss.NewStyle().
				Foreground(colorError).
				PaddingLeft(2).
				PaddingTop(1)
			b.WriteString(errStyle.Render("Error: " + a.err.Error()))
		} else {
			b.WriteString(a.list.View())
		}
	}

	// Pad to push help bar to bottom
	contentLines := strings.Count(b.String(), "\n") + 1
	for i := contentLines; i < a.height-1; i++ {
		b.WriteString("\n")
	}

	// Help bar
	b.WriteString(a.renderHelpBar())

	return b.String()
}

func (a App) renderStatusBar() string {
	titleStyle := lipgloss.NewStyle().
		Background(colorHeaderBg).
		Foreground(colorText).
		Bold(true).
		PaddingLeft(1).
		PaddingRight(1)

	profileStyle := lipgloss.NewStyle().
		Background(colorHeaderBg).
		Foreground(colorSuccess).
		PaddingRight(1)

	title := titleStyle.Render("jiratui")
	profile := profileStyle.Render("● " + a.profileName)

	gap := a.width - lipgloss.Width(title) - lipgloss.Width(profile)
	if gap < 0 {
		gap = 0
	}
	spacer := lipgloss.NewStyle().
		Background(colorHeaderBg).
		Render(strings.Repeat(" ", gap))

	return title + spacer + profile
}

func (a App) renderHelpBar() string {
	helpStyle := lipgloss.NewStyle().
		Background(colorHeaderBg).
		Foreground(colorSubtle).
		PaddingLeft(1)

	var help string
	if a.state == stateList && a.list.filtering {
		help = "enter confirm · esc clear filter"
	} else {
		help = "↑/k up · ↓/j down · / filter · o open in browser · r refresh · q quit"
	}

	rendered := helpStyle.Render(help)
	gap := a.width - lipgloss.Width(rendered)
	if gap < 0 {
		gap = 0
	}
	pad := lipgloss.NewStyle().Background(colorHeaderBg).Render(strings.Repeat(" ", gap))

	return rendered + pad
}

// Run starts the Bubble Tea program.
func Run(client *jira.Client, profileName string) error {
	app := NewApp(client, profileName)
	p := tea.NewProgram(app,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	_, err := p.Run()
	if err != nil {
		return fmt.Errorf("running TUI: %w", err)
	}
	return nil
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/christopherdobbyn/work/dobbo-ca/jiratui && go build ./internal/tui/`
Expected: clean build

- [ ] **Step 3: Commit**

```bash
git add internal/tui/app.go
git commit -m "feat(tui): add root app model with loading spinner, status bar, and help bar"
```

---

### Task 6: Wire TUI to CLI as Default Command

**Files:**
- Modify: `cmd/root.go`

- [ ] **Step 1: Update root command to launch TUI when run with no subcommand**

In `cmd/root.go`, add `RunE` to the root command so that `jiratui` (no args) launches the TUI:

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/christopherdobbyn/jiratui/internal/config"
	"github.com/christopherdobbyn/jiratui/internal/jira"
	"github.com/christopherdobbyn/jiratui/internal/tui"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "jiratui",
	Short: "A terminal UI for Jira Cloud",
	Long:  "jiratui is a fast, lightweight terminal user interface for browsing and interacting with Jira Cloud.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip first-run check for auth commands (they handle it themselves)
		if cmd.Parent() != nil && cmd.Parent().Use == "auth" {
			return nil
		}
		// Also skip for the auth command group itself
		if cmd.Use == "auth" {
			return nil
		}

		cfgPath := config.DefaultPath()
		if !config.Exists(cfgPath) {
			fmt.Println("Welcome to jiratui!")
			fmt.Println()
			fmt.Println("No configuration found. Let's set up your first Jira Cloud profile.")
			fmt.Println()
			return runAuthAdd(cmd, nil)
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath := config.DefaultPath()
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		profile, err := cfg.ActiveProfileConfig()
		if err != nil {
			return err
		}

		client := jira.NewClient(profile.URL, profile.Email, profile.APIToken)
		return tui.Run(client, cfg.ActiveProfile)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

- [ ] **Step 2: Build and verify it launches**

Run: `cd /Users/christopherdobbyn/work/dobbo-ca/jiratui && go build -o jiratui .`
Expected: binary builds successfully

Manual test: run `./jiratui` — should show loading spinner, then a table of your Jira issues with:
- Status bar at top showing "jiratui" and your profile name in green
- Issue table with Key, P, Status, Assignee, Updated, Summary columns
- Help bar at bottom showing keybindings
- Press `q` to quit

- [ ] **Step 3: Verify existing subcommands still work**

Run: `./jiratui auth list`
Expected: shows your profiles as before

Run: `./jiratui issues`
Expected: shows text table of issues as before

- [ ] **Step 4: Commit**

```bash
git add cmd/root.go
git commit -m "feat: launch TUI as default command"
```

---

### Task 7: Manual Verification of All Features

This task is a manual QA pass — no code changes, just verification.

- [ ] **Step 1: Build and launch**

```bash
cd /Users/christopherdobbyn/work/dobbo-ca/jiratui && go build -o jiratui . && ./jiratui
```

- [ ] **Step 2: Verify loading spinner**

Expected: see a dot spinner with "Fetching issues..." centered on screen while data loads.

- [ ] **Step 3: Verify issue table**

Expected: table appears with real Jira issues. Columns: Key, P (priority icon), Status, Assignee, Updated (relative time), Summary. Selected row is highlighted.

- [ ] **Step 4: Verify keyboard navigation**

Test: press `j` / `k` / `↑` / `↓` to move selection up and down.
Expected: cursor moves, selected row highlight follows.

- [ ] **Step 5: Verify mouse support**

Test: click on a row, use scroll wheel.
Expected: clicking selects the row, scroll wheel scrolls the table.

- [ ] **Step 6: Verify quick filter**

Test: press `/`, type a partial issue key or summary word.
Expected: filter input appears, table filters in real-time. Press `esc` to clear.

- [ ] **Step 7: Verify open in browser**

Test: select an issue, press `o`.
Expected: issue opens in your default browser at the Jira URL.

- [ ] **Step 8: Verify refresh**

Test: press `r`.
Expected: spinner appears briefly, data reloads, table refreshes.

- [ ] **Step 9: Verify quit**

Test: press `q` (or `ctrl+c`).
Expected: TUI exits cleanly, terminal restored.

- [ ] **Step 10: Verify status bar**

Expected: top bar shows "jiratui" on left, "● profilename" in green on right, dark background.

- [ ] **Step 11: Verify help bar**

Expected: bottom bar shows context-sensitive keybindings. When filtering, shows "enter confirm · esc clear filter". Otherwise shows navigation/action keys.

- [ ] **Step 12: Run all tests**

```bash
cd /Users/christopherdobbyn/work/dobbo-ca/jiratui && go test ./... -v
```

Expected: all tests pass (config, jira client, and new TUI tests).
