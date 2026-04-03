# Milestone 4: Detail View Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a tabbed detail view for issues, opened via enter key from the list, with responsive split/full-screen layout.

**Architecture:** A new `Detail` model in `detail.go` manages 5 tabs (Details, Description, Comments, Subtasks, Links) with scrolling. The app state machine gains a `stateDetail` state. In narrow terminals (<120 cols), detail replaces the list full-screen; in wide terminals, list and detail render side by side. The `GetIssue` API call fetches full issue data on demand.

**Tech Stack:** Go, Bubble Tea, Lip Gloss (same as existing codebase)

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/tui/detail.go` | Create | Detail view model: tabs, metadata grid, description, comments, subtasks, links rendering, scrolling |
| `internal/tui/app.go` | Modify | Add `stateDetail` state, `detail` field, enter/esc transitions, issue fetch command, responsive layout in View() |
| `internal/tui/keys.go` | Modify | Add detail view key bindings (tab switching 1-5, scroll j/k, esc back, o open) |
| `internal/tui/list.go` | Modify | Add enter key handler that emits an `openIssueMsg`, adjust width for split layout |

---

## Task 1: Detail Model Skeleton + App Wiring

Wire up enter/esc navigation between list and detail views. Detail shows issue key and summary as proof of concept.

**Files:**
- Create: `internal/tui/detail.go`
- Modify: `internal/tui/app.go`
- Modify: `internal/tui/list.go`
- Modify: `internal/tui/keys.go`

- [ ] **Step 1: Add detail key bindings to keys.go**

Add a `DetailKeyMap` and `Enter` binding to the list keys:

```go
// Add to ListKeyMap struct:
Enter key.Binding

// Add to listKeys var:
Enter: key.NewBinding(
    key.WithKeys("enter"),
    key.WithHelp("enter", "open"),
),

// New struct:
type DetailKeyMap struct {
    Escape key.Binding
    Open   key.Binding
    Tab1   key.Binding
    Tab2   key.Binding
    Tab3   key.Binding
    Tab4   key.Binding
    Tab5   key.Binding
    Down   key.Binding
    Up     key.Binding
}

var detailKeys = DetailKeyMap{
    Escape: key.NewBinding(
        key.WithKeys("esc"),
        key.WithHelp("esc", "back"),
    ),
    Open: key.NewBinding(
        key.WithKeys("o"),
        key.WithHelp("o", "open in browser"),
    ),
    Tab1: key.NewBinding(key.WithKeys("1")),
    Tab2: key.NewBinding(key.WithKeys("2")),
    Tab3: key.NewBinding(key.WithKeys("3")),
    Tab4: key.NewBinding(key.WithKeys("4")),
    Tab5: key.NewBinding(key.WithKeys("5")),
    Down: key.NewBinding(
        key.WithKeys("down", "j"),
        key.WithHelp("j/↓", "scroll down"),
    ),
    Up: key.NewBinding(
        key.WithKeys("up", "k"),
        key.WithHelp("k/↑", "scroll up"),
    ),
}
```

- [ ] **Step 2: Create detail.go with skeleton Detail model**

```go
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

type detailTab int

const (
	tabDetails detailTab = iota
	tabDescription
	tabComments
	tabSubtasks
	tabLinks
)

// Detail is the Bubble Tea model for the issue detail view.
type Detail struct {
	issue     models.Issue
	activeTab detailTab
	scrollY   int
	width     int
	height    int
}

// NewDetail creates a new detail model for the given issue.
func NewDetail(issue models.Issue, width, height int) Detail {
	return Detail{
		issue:  issue,
		width:  width,
		height: height,
	}
}

// Update handles messages for the detail view.
func (d Detail) Update(msg tea.Msg) (Detail, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, detailKeys.Open):
			_ = browser.OpenURL(d.issue.BrowseURL)
			return d, nil
		case key.Matches(msg, detailKeys.Tab1):
			d.activeTab = tabDetails
			d.scrollY = 0
			return d, nil
		case key.Matches(msg, detailKeys.Tab2):
			d.activeTab = tabDescription
			d.scrollY = 0
			return d, nil
		case key.Matches(msg, detailKeys.Tab3):
			d.activeTab = tabComments
			d.scrollY = 0
			return d, nil
		case key.Matches(msg, detailKeys.Tab4):
			d.activeTab = tabSubtasks
			d.scrollY = 0
			return d, nil
		case key.Matches(msg, detailKeys.Tab5):
			d.activeTab = tabLinks
			d.scrollY = 0
			return d, nil
		case key.Matches(msg, detailKeys.Down):
			d.scrollY++
			return d, nil
		case key.Matches(msg, detailKeys.Up):
			if d.scrollY > 0 {
				d.scrollY--
			}
			return d, nil
		}
	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelDown:
			d.scrollY++
			return d, nil
		case tea.MouseButtonWheelUp:
			if d.scrollY > 0 {
				d.scrollY--
			}
			return d, nil
		}
	case tea.WindowSizeMsg:
		d.width = msg.Width
		d.height = msg.Height
		return d, nil
	}
	return d, nil
}

// contentHeight returns available lines for tab content (below tab bar).
func (d Detail) contentHeight() int {
	// Reserve: tab bar(1) + separator(1)
	h := d.height - 2
	if h < 1 {
		h = 1
	}
	return h
}

// View renders the detail view.
func (d Detail) View() string {
	var b strings.Builder

	b.WriteString(d.renderTabBar())
	b.WriteString("\n")

	sep := lipgloss.NewStyle().Foreground(colorBorder).Render(strings.Repeat("─", d.width))
	b.WriteString(sep)
	b.WriteString("\n")

	var content string
	switch d.activeTab {
	case tabDetails:
		content = d.renderDetailsTab()
	case tabDescription:
		content = d.renderDescriptionTab()
	case tabComments:
		content = d.renderCommentsTab()
	case tabSubtasks:
		content = d.renderSubtasksTab()
	case tabLinks:
		content = d.renderLinksTab()
	}

	b.WriteString(d.applyScroll(content))

	return b.String()
}

func (d Detail) renderTabBar() string {
	tabs := []struct {
		name  string
		count int
	}{
		{"Details", -1},
		{"Description", -1},
		{"Comments", len(d.issue.Comments)},
		{"Subtasks", len(d.issue.Subtasks)},
		{"Links", len(d.issue.Links)},
	}

	active := lipgloss.NewStyle().
		Foreground(colorAccent).
		Bold(true).
		PaddingLeft(1).
		PaddingRight(1)

	inactive := lipgloss.NewStyle().
		Foreground(colorSubtle).
		PaddingLeft(1).
		PaddingRight(1)

	var parts []string
	for i, tab := range tabs {
		label := fmt.Sprintf("%d:%s", i+1, tab.name)
		if tab.count >= 0 {
			label = fmt.Sprintf("%d:%s(%d)", i+1, tab.name, tab.count)
		}
		if detailTab(i) == d.activeTab {
			parts = append(parts, active.Render(label))
		} else {
			parts = append(parts, inactive.Render(label))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func (d Detail) applyScroll(content string) string {
	lines := strings.Split(content, "\n")
	if d.scrollY >= len(lines) {
		d.scrollY = len(lines) - 1
	}
	if d.scrollY < 0 {
		d.scrollY = 0
	}
	visible := d.contentHeight()
	start := d.scrollY
	end := start + visible
	if end > len(lines) {
		end = len(lines)
	}
	if start >= end {
		return ""
	}
	return strings.Join(lines[start:end], "\n")
}

func (d Detail) renderDetailsTab() string {
	var b strings.Builder
	pad := lipgloss.NewStyle().PaddingLeft(2)
	labelStyle := lipgloss.NewStyle().Foreground(colorSubtle).Width(14)
	valueStyle := lipgloss.NewStyle().Foreground(colorText)
	titleStyle := lipgloss.NewStyle().Foreground(colorText).Bold(true).PaddingLeft(2).PaddingBottom(1)

	// Title
	b.WriteString(titleStyle.Render(d.issue.Key + "  " + d.issue.Summary))
	b.WriteString("\n\n")

	row := func(label, value string) {
		b.WriteString(pad.Render(labelStyle.Render(label) + valueStyle.Render(value)))
		b.WriteString("\n")
	}

	row("Status", StyledStatus(d.issue.Status.Name))
	row("Priority", lipgloss.NewStyle().Foreground(priorityColor(d.issue.Priority.Name)).Render("● "+d.issue.Priority.Name))
	row("Type", d.issue.Type.Name)

	if d.issue.Assignee != nil {
		row("Assignee", lipgloss.NewStyle().Foreground(colorSuccess).Render(d.issue.Assignee.DisplayName))
	} else {
		row("Assignee", lipgloss.NewStyle().Foreground(colorSubtle).Render("Unassigned"))
	}

	if d.issue.Reporter != nil {
		row("Reporter", d.issue.Reporter.DisplayName)
	}

	if d.issue.Sprint != "" {
		row("Sprint", d.issue.Sprint)
	}

	if d.issue.Parent != nil {
		row("Parent", lipgloss.NewStyle().Foreground(colorAccent).Render(d.issue.Parent.Key+" "+d.issue.Parent.Summary))
	}

	if len(d.issue.Labels) > 0 {
		tagStyle := lipgloss.NewStyle().Foreground(colorInfo)
		tags := make([]string, len(d.issue.Labels))
		for i, l := range d.issue.Labels {
			tags[i] = tagStyle.Render("[" + l + "]")
		}
		row("Labels", strings.Join(tags, " "))
	}

	row("Created", d.issue.Created.Format("2006-01-02 15:04"))
	row("Updated", relativeTime(d.issue.Updated)+" ("+d.issue.Updated.Format("2006-01-02 15:04")+")")

	if d.issue.DueDate != nil {
		dueDateStr := d.issue.DueDate.Format("2006-01-02")
		if d.issue.DueDate.Before(time.Now()) {
			dueDateStr = lipgloss.NewStyle().Foreground(colorError).Render(dueDateStr + " OVERDUE")
		} else if d.issue.DueDate.Before(time.Now().Add(7 * 24 * time.Hour)) {
			dueDateStr = lipgloss.NewStyle().Foreground(colorWarning).Render(dueDateStr)
		}
		row("Due Date", dueDateStr)
	}

	return b.String()
}

func (d Detail) renderDescriptionTab() string {
	pad := lipgloss.NewStyle().PaddingLeft(2).PaddingTop(1).Foreground(colorText)
	if d.issue.Description == "" {
		return pad.Foreground(colorSubtle).Render("No description.")
	}
	// Word wrap to available width
	wrapped := wordWrap(d.issue.Description, d.width-4)
	return pad.Render(wrapped)
}

func (d Detail) renderCommentsTab() string {
	if len(d.issue.Comments) == 0 {
		return lipgloss.NewStyle().PaddingLeft(2).PaddingTop(1).Foreground(colorSubtle).Render("No comments.")
	}

	var b strings.Builder
	authorStyle := lipgloss.NewStyle().Foreground(colorSuccess).Bold(true)
	timeStyle := lipgloss.NewStyle().Foreground(colorSubtle)
	bodyStyle := lipgloss.NewStyle().Foreground(colorText).PaddingLeft(4)

	for i, c := range d.issue.Comments {
		b.WriteString("  ")
		b.WriteString(authorStyle.Render(c.Author.DisplayName))
		b.WriteString("  ")
		b.WriteString(timeStyle.Render(relativeTime(c.Created)))
		b.WriteString("\n")
		wrapped := wordWrap(c.Body, d.width-6)
		b.WriteString(bodyStyle.Render(wrapped))
		if i < len(d.issue.Comments)-1 {
			b.WriteString("\n\n")
			b.WriteString(lipgloss.NewStyle().Foreground(colorBorder).PaddingLeft(2).Render(strings.Repeat("·", d.width-4)))
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (d Detail) renderSubtasksTab() string {
	if len(d.issue.Subtasks) == 0 {
		return lipgloss.NewStyle().PaddingLeft(2).PaddingTop(1).Foreground(colorSubtle).Render("No subtasks.")
	}

	var b strings.Builder
	for _, st := range d.issue.Subtasks {
		indicator := "◦"
		indColor := colorSubtle
		if st.Status.Name == "Done" {
			indicator = "✓"
			indColor = colorSuccess
		} else if st.Status.Name == "In Progress" {
			indicator = "●"
			indColor = colorWarning
		}

		b.WriteString("  ")
		b.WriteString(lipgloss.NewStyle().Foreground(indColor).Render(indicator))
		b.WriteString(" ")
		b.WriteString(lipgloss.NewStyle().Foreground(colorAccent).Render(st.Key))
		b.WriteString("  ")
		b.WriteString(lipgloss.NewStyle().Foreground(colorText).Render(st.Summary))
		b.WriteString("  ")
		b.WriteString(StyledStatus(st.Status.Name))
		b.WriteString("\n")
	}

	return b.String()
}

func (d Detail) renderLinksTab() string {
	if len(d.issue.Links) == 0 {
		return lipgloss.NewStyle().PaddingLeft(2).PaddingTop(1).Foreground(colorSubtle).Render("No linked issues.")
	}

	var b strings.Builder
	relStyle := lipgloss.NewStyle().Foreground(colorPurple)
	keyStyle := lipgloss.NewStyle().Foreground(colorAccent)
	textStyle := lipgloss.NewStyle().Foreground(colorText)

	for _, link := range d.issue.Links {
		var linkedIssue *models.IssueSummary
		if link.OutwardIssue != nil {
			linkedIssue = link.OutwardIssue
		} else if link.InwardIssue != nil {
			linkedIssue = link.InwardIssue
		}
		if linkedIssue == nil {
			continue
		}

		b.WriteString("  ")
		b.WriteString(relStyle.Render(padRight(link.Type, 20)))
		b.WriteString(" ")
		b.WriteString(keyStyle.Render(linkedIssue.Key))
		b.WriteString("  ")
		b.WriteString(textStyle.Render(linkedIssue.Summary))
		b.WriteString("  ")
		b.WriteString(StyledStatus(linkedIssue.Status.Name))
		b.WriteString("\n")
	}

	return b.String()
}

// wordWrap wraps text at the given width, breaking on spaces.
func wordWrap(s string, width int) string {
	if width <= 0 {
		return s
	}
	var result strings.Builder
	for _, paragraph := range strings.Split(s, "\n") {
		if result.Len() > 0 {
			result.WriteString("\n")
		}
		words := strings.Fields(paragraph)
		if len(words) == 0 {
			continue
		}
		lineLen := 0
		for i, w := range words {
			wLen := len(w)
			if i > 0 && lineLen+1+wLen > width {
				result.WriteString("\n")
				lineLen = 0
			} else if i > 0 {
				result.WriteString(" ")
				lineLen++
			}
			result.WriteString(w)
			lineLen += wLen
		}
	}
	return result.String()
}
```

- [ ] **Step 3: Add enter key to list.go to emit openIssueMsg**

In `list.go`, add a handler for Enter that returns an `openIssueMsg`:

```go
// Add message type at top of file (below imports):
type openIssueMsg struct {
	issueKey string
}
```

In the `Update` method, in the non-filtering key handling section (after the `key.Matches(msg, listKeys.Open)` case), add:

```go
case key.Matches(msg, listKeys.Enter):
    if l.cursor < len(l.filtered) {
        return l, func() tea.Msg {
            return openIssueMsg{issueKey: l.filtered[l.cursor].Key}
        }
    }
    return l, nil
```

Also handle double-click on a row in the MouseButtonLeft handler — when the clicked row is already the cursor, emit the open message:

```go
case tea.MouseButtonLeft:
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
```

- [ ] **Step 4: Wire app.go state machine for detail view**

Add to the `state` constants:

```go
const (
    stateLoading state = iota
    stateList
    stateDetail
    stateDetailLoading
)
```

Add fields to the `App` struct:

```go
type App struct {
    state       state
    list        List
    detail      Detail
    spinner     spinner.Model
    client      *jira.Client
    profileName string
    err         error
    width       int
    height      int
}
```

Add new message type and fetch command:

```go
type issueDetailMsg struct {
    issue models.Issue
}

func fetchIssueDetail(client *jira.Client, key string) tea.Cmd {
    return func() tea.Msg {
        issue, err := client.GetIssue(key)
        if err != nil {
            return errMsg{err: err}
        }
        return issueDetailMsg{issue: *issue}
    }
}
```

Update the `Update` method to handle new states and messages:

1. In the `tea.KeyMsg` handler, add detail state handling before the list state block:

```go
if a.state == stateDetail {
    if key.Matches(msg, detailKeys.Escape) {
        a.state = stateList
        return a, nil
    }
    var cmd tea.Cmd
    a.detail, cmd = a.detail.Update(msg)
    return a, cmd
}
```

2. Add handler for `openIssueMsg`:

```go
case openIssueMsg:
    a.state = stateDetailLoading
    return a, tea.Batch(a.spinner.Tick, fetchIssueDetail(a.client, msg.issueKey))
```

3. Add handler for `issueDetailMsg`:

```go
case issueDetailMsg:
    contentHeight := a.height - 2 // status bar + help bar
    a.detail = NewDetail(msg.issue, a.width, contentHeight)
    a.state = stateDetail
    return a, nil
```

4. In the `tea.WindowSizeMsg` handler, add detail state:

```go
if a.state == stateDetail {
    var cmd tea.Cmd
    a.detail, cmd = a.detail.Update(msg)
    return a, cmd
}
```

5. Allow spinner to tick in `stateDetailLoading`:

```go
case spinner.TickMsg:
    if a.state == stateLoading || a.state == stateDetailLoading {
        var cmd tea.Cmd
        a.spinner, cmd = a.spinner.Update(msg)
        return a, cmd
    }
    return a, nil
```

6. Forward mouse messages to detail in the fallthrough at the end of Update:

```go
if a.state == stateDetail {
    var cmd tea.Cmd
    a.detail, cmd = a.detail.Update(msg)
    return a, cmd
}
```

Update the `View` method to render detail state:

```go
case stateDetailLoading:
    loadingStyle := lipgloss.NewStyle().
        PaddingTop(a.height/2 - 2).
        PaddingLeft(a.width/2 - 12).
        Foreground(colorText)
    b.WriteString(loadingStyle.Render(a.spinner.View() + " Loading issue..."))
case stateDetail:
    b.WriteString(a.detail.View())
```

Update `renderHelpBar` to show detail keybindings:

```go
func (a App) renderHelpBar() string {
    // ... existing style setup ...

    var help string
    switch {
    case a.state == stateList && a.list.filtering:
        help = "enter confirm · esc clear filter"
    case a.state == stateDetail:
        help = "esc back · 1-5 tabs · j/k scroll · o open in browser"
    default:
        help = "↑/k up · ↓/j down · enter open · / filter · o browser · r refresh · q quit"
    }

    // ... rest unchanged ...
}
```

- [ ] **Step 5: Build and verify**

Run: `cd /Users/christopherdobbyn/work/dobbo-ca/jiratui && go build -o jiratui .`
Expected: compiles cleanly.

Test manually: Launch `./jiratui`, navigate to an issue, press enter. Verify:
- Loading spinner shows "Loading issue..."
- Detail view appears with metadata grid
- All 5 tabs render (Details, Description, Comments, Subtasks, Links)
- Press 1-5 to switch tabs
- j/k scrolls content
- esc returns to list
- o opens issue in browser

- [ ] **Step 6: Commit**

```bash
git add internal/tui/detail.go internal/tui/app.go internal/tui/list.go internal/tui/keys.go
git commit -m "feat(tui): add detail view with 5 tabs and enter/esc navigation"
```

---

## Task 2: Responsive Split Layout

Add side-by-side layout when terminal is wide (120+ cols).

**Files:**
- Modify: `internal/tui/app.go`
- Modify: `internal/tui/list.go`
- Modify: `internal/tui/detail.go`

- [ ] **Step 1: Add split layout constant and helper to app.go**

```go
const splitThreshold = 120
const listPaneWidth = 50

func (a App) isWide() bool {
    return a.width >= splitThreshold
}
```

- [ ] **Step 2: Update App.View() for split rendering**

Replace the `stateDetail` case in View() and update list rendering for split mode:

```go
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
case stateDetail, stateDetailLoading:
    if a.isWide() {
        // Split: list on left, detail on right
        listW := listPaneWidth
        detailW := a.width - listW - 1 // 1 for border
        contentH := a.height - 2       // status + help bars

        leftList := a.list.ViewWithWidth(listW, contentH)
        border := lipgloss.NewStyle().Foreground(colorBorder).
            Render(strings.Repeat("│\n", contentH))

        var right string
        if a.state == stateDetailLoading {
            loadStyle := lipgloss.NewStyle().
                Width(detailW).
                Foreground(colorText).
                PaddingTop(contentH/2 - 1).
                PaddingLeft(detailW/2 - 10)
            right = loadStyle.Render(a.spinner.View() + " Loading issue...")
        } else {
            right = a.detail.View()
        }

        b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, leftList, border, right))
    } else {
        // Narrow: full-screen swap
        if a.state == stateDetailLoading {
            loadingStyle := lipgloss.NewStyle().
                PaddingTop(a.height/2 - 2).
                PaddingLeft(a.width/2 - 12).
                Foreground(colorText)
            b.WriteString(loadingStyle.Render(a.spinner.View() + " Loading issue..."))
        } else {
            b.WriteString(a.detail.View())
        }
    }
```

- [ ] **Step 3: Add ViewWithWidth method to list.go**

A compact list renderer for the split view left pane:

```go
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
		assignee := ""
		_ = assignee

		row := fmt.Sprintf(" %s %s %s",
			padRight(truncStr(issue.Key, keyW), keyW),
			padRight(truncStr(issue.Status.Name, statusW), statusW),
			padRight(truncStr(issue.Summary, summaryW), summaryW),
		)

		if i == l.cursor {
			bg := lipgloss.Color("#292e42")
			b.WriteString(lipgloss.NewStyle().Foreground(colorText).Background(bg).Render(row))
		} else {
			b.WriteString(lipgloss.NewStyle().Foreground(colorText).Render(row))
		}
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}
```

- [ ] **Step 4: Update detail width in split mode**

In `app.go`, when creating the detail model or handling window resize, set the correct width for split mode:

In the `issueDetailMsg` handler:
```go
case issueDetailMsg:
    contentHeight := a.height - 2
    detailWidth := a.width
    if a.isWide() {
        detailWidth = a.width - listPaneWidth - 1
    }
    a.detail = NewDetail(msg.issue, detailWidth, contentHeight)
    a.state = stateDetail
    return a, nil
```

In the `tea.WindowSizeMsg` handler, update detail dimensions when in split mode:
```go
case tea.WindowSizeMsg:
    a.width = msg.Width
    a.height = msg.Height
    if a.state == stateList {
        var cmd tea.Cmd
        a.list, cmd = a.list.Update(msg)
        return a, cmd
    }
    if a.state == stateDetail || a.state == stateDetailLoading {
        // Update list dimensions too (for split view)
        a.list.width = msg.Width
        a.list.height = msg.Height
        a.list.clampCursor()
        if a.state == stateDetail {
            detailWidth := msg.Width
            if a.isWide() {
                detailWidth = msg.Width - listPaneWidth - 1
            }
            d := a.detail
            d.width = detailWidth
            d.height = msg.Height - 2
            a.detail = d
        }
        return a, nil
    }
    return a, nil
```

- [ ] **Step 5: Build and verify**

Run: `cd /Users/christopherdobbyn/work/dobbo-ca/jiratui && go build -o jiratui .`
Expected: compiles cleanly.

Test manually:
1. In a wide terminal (120+ cols): open an issue — list should appear on left, detail on right
2. In a narrow terminal (<120 cols): open an issue — detail should replace list full-screen
3. Resize terminal while in detail view — layout should adapt

- [ ] **Step 6: Commit**

```bash
git add internal/tui/app.go internal/tui/list.go internal/tui/detail.go
git commit -m "feat(tui): responsive split layout for detail view (wide/narrow)"
```

---

## Task 3: Polish and Edge Cases

Handle tab click, enter key in list during filtering, and ensure the q key only quits from the list.

**Files:**
- Modify: `internal/tui/detail.go`
- Modify: `internal/tui/app.go`
- Modify: `internal/tui/list.go`

- [ ] **Step 1: Add mouse click tab switching in detail.go**

In the `Detail.Update` method, add tab click handling in the `tea.MouseMsg` case:

```go
case tea.MouseButtonLeft:
    // Tab bar is at y=0 relative to detail pane
    if msg.Y == 0 {
        // Approximate click zones based on rendered tab positions
        x := msg.X
        tabWidths := []int{12, 16, 16, 14, 12} // approximate widths of each tab label
        pos := 0
        for i, w := range tabWidths {
            if x >= pos && x < pos+w {
                d.activeTab = detailTab(i)
                d.scrollY = 0
                return d, nil
            }
            pos += w
        }
    }
    return d, nil
```

- [ ] **Step 2: Ensure q doesn't quit from detail view**

In `app.go` Update, modify the `q` key handler to only work in list state:

```go
if msg.String() == "q" && a.state == stateList && !a.list.filtering {
    return a, tea.Quit
}
```

This should already be the case since the condition checks `a.state == stateList`, but verify it does.

- [ ] **Step 3: Handle enter during filtering in list.go**

When filtering is active and user presses Enter, it should confirm the filter (already works). But when not filtering, Enter should open the issue. The existing code in Step 3 of Task 1 handles this — verify the Enter handler is placed in the non-filtering branch.

- [ ] **Step 4: Clamp scroll bounds in detail.go**

Update the `Down` key handler to prevent scrolling past content:

```go
case key.Matches(msg, detailKeys.Down):
    d.scrollY++
    // Will be clamped in applyScroll
    return d, nil
```

The `applyScroll` method already clamps — this is sufficient.

- [ ] **Step 5: Build and verify**

Run: `cd /Users/christopherdobbyn/work/dobbo-ca/jiratui && go build -o jiratui .`

Test manually:
1. Click on tab labels to switch tabs
2. Press q in detail view — should NOT quit
3. Press esc in detail view — returns to list
4. Scroll down past content — should stop at bottom
5. Open an issue with real comments/subtasks/links — verify all render correctly

- [ ] **Step 6: Commit**

```bash
git add internal/tui/detail.go internal/tui/app.go internal/tui/list.go
git commit -m "feat(tui): detail view polish — tab clicks, scroll clamping, key fixes"
```
