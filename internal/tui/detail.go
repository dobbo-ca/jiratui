package tui

import (
	"fmt"
	"sort"
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

type commentAction int

const (
	commentIdle commentAction = iota
	commentAdding
	commentEditing
	commentReplying
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
	parentDrop   Dropdown
	parentHover  bool // true when mouse is hovering over parent field
	transitions  []models.Transition // stored for status→transition ID lookup

	// Label editor
	labelsEditing bool
	labelInput    textinput.Model
	labelCursor   int // -1 = input focused, 0+ = existing label selected

	// Comment interaction state
	myAccountID     string
	commentInput    textarea.Model
	commentMode     commentAction
	commentEditID   string
	commentReplyTo  int
	commentCursor   int
	confirmDelete   bool
	confirmDeleteID string

	// Comment callbacks
	OnCommentAdd    func(issueKey, body string) tea.Cmd
	OnCommentEdit   func(issueKey, commentID, body string) tea.Cmd
	OnCommentDelete func(issueKey, commentID string) tea.Cmd

	// Change callbacks — each fires a tea.Cmd that performs the Jira API update
	OnLabelsChanged   func(issueKey string, labels []string) tea.Cmd
	OnSummaryChanged  func(issueKey, summary string) tea.Cmd
	OnAssigneeChanged func(issueKey, accountID string) tea.Cmd
	OnStatusChanged   func(issueKey, transitionID string) tea.Cmd
	OnPriorityChanged func(issueKey, priorityID string) tea.Cmd
	OnDueDateChanged  func(issueKey, dueDate string) tea.Cmd
	OnParentChanged   func(issueKey, parentKey string) tea.Cmd
}

// NewDetail creates a new detail model for the given issue.
func NewDetail(issue models.Issue, width, height int) Detail {
	ti := textinput.New()
	ti.SetValue(issue.Summary)
	ti.CharLimit = 255
	ti.Prompt = ""

	ta := textarea.New()
	ta.ShowLineNumbers = false
	ta.Prompt = ""
	ta.CharLimit = 0
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()
	// Set dimensions before SetValue so the textarea wraps content correctly.
	innerW := width - 1 - 2 - 2 // outer width - border - padding
	if innerW < 20 {
		innerW = 20
	}
	ta.SetWidth(innerW)
	ta.SetHeight(height / 2)
	ta.SetValue(issue.Description)

	// Assignee dropdown — searchable, with "Unassigned" pinned above search
	assigneeVal := "Unassigned"
	assigneeID := ""
	if issue.Assignee != nil {
		assigneeVal = issue.Assignee.DisplayName
		assigneeID = issue.Assignee.AccountID
	}
	assigneeDrop := NewDropdown("Assignee", nil, assigneeVal, assigneeID, 0)
	assigneeDrop.maxVisible = 10
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

	// Parent dropdown — searchable with min 3 chars
	parentVal := "—"
	parentID := ""
	if issue.Parent != nil {
		parentVal = issue.Parent.Key
		parentID = issue.Parent.Key
	}
	parentDrop := NewDropdown("Parent", nil, parentVal, parentID, 0)
	parentDrop.minSearchLen = 3
	parentDrop.maxVisible = 10

	li := textinput.New()
	li.Prompt = "+ "
	li.Placeholder = "Add label..."
	li.CharLimit = 100

	ci := textarea.New()
	ci.ShowLineNumbers = false
	ci.Prompt = ""
	ci.CharLimit = 0
	ci.Placeholder = "Add a comment..."
	ci.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ci.BlurredStyle.CursorLine = lipgloss.NewStyle()
	ciW := width - 1 - 2 - 2
	if ciW < 20 {
		ciW = 20
	}
	ci.SetWidth(ciW)
	ci.SetHeight(4)

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
		parentDrop:   parentDrop,
		labelInput:   li,
		labelCursor:  -1, // input focused by default
		commentInput: ci,
		commentCursor: -1,
		commentReplyTo: -1,
	}
}

// SetParentSearchFunc sets the async search callback for the parent dropdown.
func (d *Detail) SetParentSearchFunc(fn func(query string) tea.Cmd) {
	d.parentDrop.OnSearch = fn
}

// SetAssigneeOptions populates the assignee dropdown with users.
// "Unassigned" is pinned above the search; these are the searchable users below it.
// If myAccountID is non-empty, that user is moved to the top of the list.
func (d *Detail) SetAssigneeOptions(users []models.User, myAccountID string) {
	items := make([]DropdownItem, 0, len(users))
	var meItem *DropdownItem
	for _, u := range users {
		item := DropdownItem{ID: u.AccountID, Label: u.DisplayName}
		if u.AccountID == myAccountID {
			meItem = &item
		} else {
			items = append(items, item)
		}
	}
	// Put "me" at the top of the searchable list (after pinned "Unassigned")
	if meItem != nil {
		meItem.Label = meItem.Label + " (me)"
		items = append([]DropdownItem{*meItem}, items...)
	}
	d.assigneeDrop.SetItems(items)
}

// SetBoardAssignees populates the assignee dropdown with users who have
// issues on the board. "me" is moved to the top. These become the default
// items restored when the search box is cleared.
func (d *Detail) SetBoardAssignees(users []models.User, myAccountID string) {
	var meItem *DropdownItem
	var items []DropdownItem

	for _, u := range users {
		item := DropdownItem{ID: u.AccountID, Label: u.DisplayName}
		if u.AccountID == myAccountID {
			meItem = &item
		} else {
			items = append(items, item)
		}
	}
	if meItem != nil {
		meItem.Label = meItem.Label + " (me)"
		items = append([]DropdownItem{*meItem}, items...)
	}
	d.assigneeDrop.SetDefaultItems(items)
}

// statusOrder defines the canonical workflow ordering for statuses.
// Statuses not in this list sort to the end alphabetically.
var statusOrder = map[string]int{
	"Backlog":           0,
	"Schedule Next":     1,
	"To Do":             2,
	"In Progress":       3,
	"In Review":         4,
	"Resolution Review": 5,
	"Won't Do":          6,
	"Done":              7,
}

// SetStatusOptions populates the status dropdown with transitions.
func (d *Detail) SetStatusOptions(transitions []models.Transition) {
	d.transitions = transitions

	// Collect all statuses: current + transition targets (deduplicated)
	seen := make(map[string]bool)
	var items []DropdownItem

	current := DropdownItem{ID: d.issue.Status.ID, Label: d.issue.Status.Name}
	items = append(items, current)
	seen[current.ID] = true

	for _, t := range transitions {
		if !seen[t.To.ID] {
			seen[t.To.ID] = true
			items = append(items, DropdownItem{ID: t.To.ID, Label: t.To.Name})
		}
	}

	// Sort by workflow order
	sort.Slice(items, func(i, j int) bool {
		oi, oki := statusOrder[items[i].Label]
		oj, okj := statusOrder[items[j].Label]
		if oki && okj {
			return oi < oj
		}
		if oki {
			return true
		}
		if okj {
			return false
		}
		return items[i].Label < items[j].Label
	})

	d.statusDrop.SetItems(items)
}

// transitionIDForStatus finds the transition ID that moves to the given target status ID.
func (d Detail) transitionIDForStatus(targetStatusID string) string {
	for _, t := range d.transitions {
		if t.To.ID == targetStatusID {
			return t.ID
		}
	}
	return ""
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
		d.priorityDrop.IsOpen() || d.dueDatePick.IsOpen() ||
		d.parentDrop.IsOpen() || d.labelsEditing ||
		d.commentMode != commentIdle || d.confirmDelete
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
		// Label editor
		if d.labelsEditing {
			switch msg.String() {
			case "esc":
				d.labelsEditing = false
				d.labelInput.Blur()
				return d, nil
			case "enter":
				newLabel := strings.TrimSpace(d.labelInput.Value())
				if newLabel != "" {
					exists := false
					for _, l := range d.issue.Labels {
						if strings.EqualFold(l, newLabel) {
							exists = true
							break
						}
					}
					if !exists {
						d.issue.Labels = append(d.issue.Labels, newLabel)
						if d.OnLabelsChanged != nil {
							d.labelInput.SetValue("")
							return d, d.OnLabelsChanged(d.issue.Key, d.issue.Labels)
						}
					}
					d.labelInput.SetValue("")
				}
				return d, nil
			}
			var cmd tea.Cmd
			d.labelInput, cmd = d.labelInput.Update(msg)
			return d, cmd
		}

		// When a dropdown is open, forward keys to it
		if d.parentDrop.IsOpen() {
			var cmd tea.Cmd
			d.parentDrop, cmd = d.parentDrop.Update(msg)
			if !d.parentDrop.IsOpen() {
				if updateCmd := d.checkParentChanged(); updateCmd != nil {
					cmd = tea.Batch(cmd, updateCmd)
				}
			}
			return d, cmd
		}
		if d.assigneeDrop.IsOpen() {
			var cmd tea.Cmd
			d.assigneeDrop, cmd = d.assigneeDrop.Update(msg)
			if !d.assigneeDrop.IsOpen() {
				if updateCmd := d.checkAssigneeChanged(); updateCmd != nil {
					cmd = tea.Batch(cmd, updateCmd)
				}
			}
			return d, cmd
		}
		if d.statusDrop.IsOpen() {
			var cmd tea.Cmd
			d.statusDrop, cmd = d.statusDrop.Update(msg)
			if !d.statusDrop.IsOpen() {
				if updateCmd := d.checkStatusChanged(); updateCmd != nil {
					cmd = tea.Batch(cmd, updateCmd)
				}
			}
			return d, cmd
		}
		if d.priorityDrop.IsOpen() {
			var cmd tea.Cmd
			d.priorityDrop, cmd = d.priorityDrop.Update(msg)
			if !d.priorityDrop.IsOpen() {
				if updateCmd := d.checkPriorityChanged(); updateCmd != nil {
					cmd = tea.Batch(cmd, updateCmd)
				}
			}
			return d, cmd
		}
		if d.dueDatePick.IsOpen() {
			var cmd tea.Cmd
			d.dueDatePick, cmd = d.dueDatePick.Update(msg)
			if !d.dueDatePick.IsOpen() {
				if updateCmd := d.checkDueDateChanged(); updateCmd != nil {
					cmd = tea.Batch(cmd, updateCmd)
				}
			}
			return d, cmd
		}

		// When title input is focused
		if d.titleInput.Focused() {
			switch msg.String() {
			case "esc", "enter":
				d.titleInput.Blur()
				return d, d.checkSummaryChanged()
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

		// When comment input is active
		if d.activeTab == tabComments && d.commentMode != commentIdle {
			if msg.String() == "esc" {
				d.blurComment()
				return d, nil
			}
			if msg.String() == "ctrl+s" {
				return d, d.submitComment()
			}
			var cmd tea.Cmd
			d.commentInput, cmd = d.commentInput.Update(msg)
			return d, cmd
		}

		// When delete confirmation is showing
		if d.confirmDelete {
			switch msg.String() {
			case "y":
				commentID := d.confirmDeleteID
				d.confirmDelete = false
				d.confirmDeleteID = ""
				if d.OnCommentDelete != nil {
					return d, d.OnCommentDelete(d.issue.Key, commentID)
				}
				return d, nil
			case "n", "esc":
				d.confirmDelete = false
				d.confirmDeleteID = ""
				return d, nil
			}
			return d, nil
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
		// Track hover for parent tooltip
		if msg.Action == tea.MouseActionMotion && d.activeTab == tabDetails {
			col4W := (d.width - 1) / 4
			d.parentHover = msg.Y >= 5 && msg.Y <= 7 && msg.X < col4W && d.issue.Parent != nil
			return d, nil
		}

		// Handle wheel events before press/non-press split so they always work.
		// When description is focused, we handle scrolling ourselves rather than
		// letting the textarea receive wheel events (which corrupts its viewport).
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

		// Only handle press events, not release
		if msg.Action != tea.MouseActionPress {
			return d, nil
		}

		switch msg.Button {
		case tea.MouseButtonLeft:
			if msg.Y <= 1 {
				d.handleTabClick(msg.X)
			} else if d.activeTab == tabDetails {
				return d, d.handleDetailClick(msg.X, msg.Y)
			} else if d.activeTab == tabComments {
				return d, d.handleCommentClick(msg.X, msg.Y-2)
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
	if d.commentMode != commentIdle {
		var cmd tea.Cmd
		d.commentInput, cmd = d.commentInput.Update(msg)
		return d, cmd
	}
	if d.descFocused {
		var cmd tea.Cmd
		d.descInput, cmd = d.descInput.Update(msg)
		return d, cmd
	}
	if d.labelsEditing {
		var cmd tea.Cmd
		d.labelInput, cmd = d.labelInput.Update(msg)
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
	d.parentDrop.Close()
	d.labelsEditing = false
	d.labelInput.Blur()
	d.blurComment()
}

// blurComment resets all comment interaction state.
func (d *Detail) blurComment() {
	d.commentMode = commentIdle
	d.commentInput.Blur()
	d.commentInput.SetValue("")
	d.commentEditID = ""
	d.commentReplyTo = -1
	d.confirmDelete = false
	d.confirmDeleteID = ""
}

func (d *Detail) startCommentAdd() tea.Cmd {
	d.blurAll()
	d.commentMode = commentAdding
	d.commentInput.SetValue("")
	d.commentInput.Placeholder = "Add a comment..."
	d.commentInput.Focus()
	return d.commentInput.Cursor.BlinkCmd()
}

func (d *Detail) startCommentEdit(commentIdx int) tea.Cmd {
	if commentIdx < 0 || commentIdx >= len(d.issue.Comments) {
		return nil
	}
	c := d.issue.Comments[commentIdx]
	d.blurAll()
	d.commentMode = commentEditing
	d.commentEditID = c.ID
	d.commentInput.Placeholder = ""
	d.commentInput.SetValue(c.Body)
	d.commentInput.Focus()
	return d.commentInput.Cursor.BlinkCmd()
}

func (d *Detail) startCommentReply(commentIdx int) tea.Cmd {
	if commentIdx < 0 || commentIdx >= len(d.issue.Comments) {
		return nil
	}
	c := d.issue.Comments[commentIdx]
	d.blurAll()
	d.commentMode = commentReplying
	d.commentReplyTo = commentIdx
	lines := strings.Split(c.Body, "\n")
	var quoted strings.Builder
	quoted.WriteString("> **" + c.Author.DisplayName + "** wrote:\n")
	for _, line := range lines {
		quoted.WriteString("> " + line + "\n")
	}
	quoted.WriteString("\n")
	d.commentInput.Placeholder = ""
	d.commentInput.SetValue(quoted.String())
	d.commentInput.Focus()
	d.commentInput.CursorEnd()
	return d.commentInput.Cursor.BlinkCmd()
}

func (d *Detail) startCommentDelete(commentIdx int) {
	if commentIdx < 0 || commentIdx >= len(d.issue.Comments) {
		return
	}
	c := d.issue.Comments[commentIdx]
	d.confirmDelete = true
	d.confirmDeleteID = c.ID
	d.commentCursor = commentIdx
}

func (d *Detail) submitComment() tea.Cmd {
	body := strings.TrimSpace(d.commentInput.Value())
	if body == "" {
		d.blurComment()
		return nil
	}
	issueKey := d.issue.Key
	mode := d.commentMode
	editID := d.commentEditID
	d.blurComment()

	switch mode {
	case commentAdding, commentReplying:
		if d.OnCommentAdd != nil {
			return d.OnCommentAdd(issueKey, body)
		}
	case commentEditing:
		if d.OnCommentEdit != nil {
			return d.OnCommentEdit(issueKey, editID, body)
		}
	}
	return nil
}

// anyOverlayOpen returns true if any dropdown/picker overlay is showing.
func (d Detail) anyOverlayOpen() bool {
	return d.assigneeDrop.IsOpen() || d.statusDrop.IsOpen() ||
		d.priorityDrop.IsOpen() || d.dueDatePick.IsOpen() ||
		d.parentDrop.IsOpen()
}

// handleCommentClick handles mouse clicks on the comments tab.
func (d *Detail) handleCommentClick(x, y int) tea.Cmd {
	adjustedY := y + d.scrollY
	contentWidth := d.width - 6
	if contentWidth < 20 {
		contentWidth = 20
	}

	lineY := 0

	// "Add a comment..." prompt area
	if d.commentMode == commentIdle {
		if adjustedY == lineY {
			return d.startCommentAdd()
		}
		lineY += 2 // prompt + separator
	} else {
		lineY += 7 // input area height
	}

	actionStyle := lipgloss.NewStyle().Foreground(colorSubtle)

	for ci, c := range d.issue.Comments {
		isMine := c.Author.AccountID == d.myAccountID

		// Header line
		lineY++

		// Body lines
		rendered, err := glamour.Render(c.Body, "dark")
		if err != nil {
			rendered = c.Body
		}
		rendered = strings.TrimRight(rendered, "\n")
		bodyLines := len(strings.Split(rendered, "\n"))
		lineY += bodyLines

		// Action/confirm line
		if d.confirmDelete && d.confirmDeleteID == c.ID {
			lineY++ // confirm line
		} else if adjustedY == lineY {
			// Check which action button was clicked
			xPos := 2
			replyW := lipgloss.Width(actionStyle.Render("↩ Reply"))
			if x >= xPos && x < xPos+replyW {
				return d.startCommentReply(ci)
			}
			xPos += replyW + 2

			if isMine {
				editW := lipgloss.Width(actionStyle.Render("✎ Edit"))
				if x >= xPos && x < xPos+editW {
					return d.startCommentEdit(ci)
				}
				xPos += editW + 2

				deleteW := lipgloss.Width(actionStyle.Render("✗ Delete"))
				if x >= xPos && x < xPos+deleteW {
					d.startCommentDelete(ci)
					return nil
				}
			}
			lineY++
		} else {
			lineY++ // action line
		}

		// Separator
		if ci < len(d.issue.Comments)-1 {
			lineY++
		}
	}

	return nil
}

// handleDetailClick handles mouse clicks on detail fields.
func (d *Detail) handleDetailClick(x, y int) tea.Cmd {
	w := d.width - 1
	col4W := w / 4
	col3W := w / 3

	// Row positions (each row = 3 lines, tab bar = 2 lines)
	row1Y := 2         // Title
	row2Y := row1Y + 3 // Parent, Key, Type, Due Date
	row3Y := row2Y + 3 // Assignee, Reporter, Status
	row4Y := row3Y + 3 // Updated, Created, Priority
	row5Y := row4Y + 3 // Labels
	row5H := 3
	if d.labelsEditing {
		row5H = d.labelEditorHeight()
	}
	row6Y := row5Y + row5H // Description

	// If anything is being edited, handle overlay clicks or close.
	if d.Editing() {
		if d.anyOverlayOpen() {
			if d.parentDrop.IsOpen() {
				overlay := d.parentDrop.RenderStandaloneOverlay()
				if overlay != nil && y >= row2Y && y < row2Y+len(overlay) && x < col4W {
					clickLine := y - row2Y - 3
					if clickLine >= 0 && d.parentDrop.HandleClick(clickLine) {
						if !d.parentDrop.IsOpen() {
							return d.checkParentChanged()
						}
						return nil
					}
				}
			}
			if d.dueDatePick.IsOpen() {
				overlay := d.dueDatePick.RenderOverlay()
				if overlay != nil {
					overlayW := d.dueDatePick.calWidth()
					lastCol4W := w - 3*col4W
					dpX := w - overlayW
					if dpX < 0 {
						dpX = 0
					}
					hasConnector := overlayW > lastCol4W
					overlayStartY := row2Y + 3
					if hasConnector {
						overlayStartY = row2Y + 2
					}
					totalLines := len(overlay)
					if hasConnector {
						totalLines++
					}
					if y >= overlayStartY && y < overlayStartY+totalLines &&
						x >= dpX && x < dpX+overlayW {
						overlayLine := y - overlayStartY
						if hasConnector {
							if overlayLine == 0 {
								return nil
							}
							overlayLine--
						}
						localX := x - dpX
						if d.dueDatePick.HandleClick(overlayLine, localX, overlayW) {
							if !d.dueDatePick.IsOpen() {
								return d.checkDueDateChanged()
							}
							return nil
						}
					}
				}
			}
			if d.assigneeDrop.IsOpen() {
				overlay := d.assigneeDrop.RenderStandaloneOverlay()
				if overlay != nil && y >= row3Y && y < row3Y+len(overlay) && x < col3W {
					clickLine := y - row3Y - 3
					if clickLine >= 0 && d.assigneeDrop.HandleClick(clickLine) {
						if !d.assigneeDrop.IsOpen() {
							return d.checkAssigneeChanged()
						}
						return nil
					}
				}
			}
			if d.statusDrop.IsOpen() {
				overlay := d.statusDrop.RenderStandaloneOverlay()
				statusX := 2 * col3W
				if overlay != nil && y >= row3Y && y < row3Y+len(overlay) && x >= statusX {
					clickLine := y - row3Y - 3
					if clickLine >= 0 && d.statusDrop.HandleClick(clickLine) {
						if !d.statusDrop.IsOpen() {
							return d.checkStatusChanged()
						}
						return nil
					}
				}
			}
			if d.priorityDrop.IsOpen() {
				overlay := d.priorityDrop.RenderStandaloneOverlay()
				prioX := 2 * col3W
				if overlay != nil && y >= row4Y && y < row4Y+len(overlay) && x >= prioX {
					clickLine := y - row4Y - 3
					if clickLine >= 0 && d.priorityDrop.HandleClick(clickLine) {
						if !d.priorityDrop.IsOpen() {
							return d.checkPriorityChanged()
						}
						return nil
					}
				}
			}
		}

		// Handle clicks on label editor area before blurring
		if d.labelsEditing && y >= row5Y && y < row6Y {
			return d.handleLabelClick(x, y, row5Y, w)
		}

		d.blurAll()
		return nil
	}

	// Row 1: Title (full width)
	if y >= row1Y && y < row2Y {
		d.blurAll()
		d.titleInput.Focus()
		return d.titleInput.Cursor.BlinkCmd()
	}

	// Row 2: Parent + Key + Type + Due Date (4 columns)
	if y >= row2Y && y < row3Y {
		if x < col4W {
			d.blurAll()
			return d.parentDrop.OpenDropdown()
		} else if x >= 3*col4W {
			d.blurAll()
			d.dueDatePick.OpenPicker()
			return nil
		}
		d.blurAll()
		return nil
	}

	// Row 3: Assignee + Reporter + Status (3 columns)
	if y >= row3Y && y < row4Y {
		if x < col3W {
			d.blurAll()
			return d.assigneeDrop.OpenDropdown()
		} else if x >= 2*col3W {
			d.blurAll()
			return d.statusDrop.OpenDropdown()
		}
		d.blurAll()
		return nil
	}

	// Row 4: Updated + Created + Priority (3 columns)
	if y >= row4Y && y < row5Y {
		if x >= 2*col3W {
			d.blurAll()
			return d.priorityDrop.OpenDropdown()
		}
		d.blurAll()
		return nil
	}

	// Row 5: Labels
	if y >= row5Y && y < row6Y {
		d.blurAll()
		d.labelsEditing = true
		d.labelCursor = -1
		d.labelInput.SetValue("")
		d.labelInput.Focus()
		return d.labelInput.Cursor.BlinkCmd()
	}

	// Row 6+: Description
	if y >= row6Y {
		d.blurAll()
		d.descFocused = true
		d.descInput.Focus()
		return d.descInput.Cursor.BlinkCmd()
	}

	d.blurAll()
	return nil
}

// handleLabelClick handles clicks within the label editor area.
func (d *Detail) handleLabelClick(x, y, row5Y, w int) tea.Cmd {
	if len(d.issue.Labels) == 0 {
		return nil
	}
	valW := w - 4 // inside "│ " and " │"

	// Build the same wrapped layout as renderLabelEditor to find which label was clicked.
	// Each chip is "[name]×" with spaces between, wrapping to next line when full.
	chipLine := 0  // which wrapped line (0-based)
	chipCol := 0   // current column within the content area

	// The first label content line is at row5Y+1. Additional wrapped lines follow.
	clickLine := y - (row5Y + 1)
	if clickLine < 0 {
		return nil
	}

	for i, l := range d.issue.Labels {
		chipW := len("["+l+"]") + 1 // [name] + ×
		totalW := chipW
		if i < len(d.issue.Labels)-1 {
			totalW++ // space separator
		}

		// Wrap to next line if this chip doesn't fit
		if chipCol > 0 && chipCol+chipW > valW {
			chipLine++
			chipCol = 0
		}

		if chipLine == clickLine {
			// Check if click x hits the × for this chip
			// × is at column 2 (border+space) + chipCol + len("[name]")
			removeX := 2 + chipCol + len("["+l+"]")
			if x >= removeX && x <= removeX+1 {
				d.issue.Labels = append(d.issue.Labels[:i], d.issue.Labels[i+1:]...)
				if d.OnLabelsChanged != nil {
					return d.OnLabelsChanged(d.issue.Key, d.issue.Labels)
				}
				return nil
			}
		}

		chipCol += totalW
	}
	return nil
}

// descMaxScroll returns the maximum scroll offset for the description field.
// markUpdated sets the Updated timestamp to now for immediate UI feedback.
func (d *Detail) markUpdated() {
	d.issue.Updated = time.Now()
}

func (d *Detail) checkSummaryChanged() tea.Cmd {
	newVal := d.titleInput.Value()
	if newVal != d.issue.Summary && d.OnSummaryChanged != nil {
		d.issue.Summary = newVal
		d.markUpdated()
		return d.OnSummaryChanged(d.issue.Key, newVal)
	}
	return nil
}

func (d *Detail) checkAssigneeChanged() tea.Cmd {
	sel := d.assigneeDrop.SelectedItem()
	if sel == nil {
		return nil
	}
	oldID := ""
	if d.issue.Assignee != nil {
		oldID = d.issue.Assignee.AccountID
	}
	if sel.ID != oldID && d.OnAssigneeChanged != nil {
		if sel.ID == "" {
			d.issue.Assignee = nil
		} else {
			d.issue.Assignee = &models.User{AccountID: sel.ID, DisplayName: sel.Label}
		}
		d.markUpdated()
		return d.OnAssigneeChanged(d.issue.Key, sel.ID)
	}
	return nil
}

func (d *Detail) checkStatusChanged() tea.Cmd {
	sel := d.statusDrop.SelectedItem()
	if sel == nil || sel.ID == d.issue.Status.ID {
		return nil
	}
	if d.OnStatusChanged != nil {
		transitionID := d.transitionIDForStatus(sel.ID)
		if transitionID == "" {
			return nil // no valid transition found
		}
		d.issue.Status = models.Status{ID: sel.ID, Name: sel.Label}
		d.markUpdated()
		return d.OnStatusChanged(d.issue.Key, transitionID)
	}
	return nil
}

func (d *Detail) checkPriorityChanged() tea.Cmd {
	sel := d.priorityDrop.SelectedItem()
	if sel == nil || sel.ID == d.issue.Priority.ID {
		return nil
	}
	if d.OnPriorityChanged != nil {
		d.issue.Priority = models.Priority{ID: sel.ID, Name: sel.Label}
		d.markUpdated()
		return d.OnPriorityChanged(d.issue.Key, sel.ID)
	}
	return nil
}

func (d *Detail) checkDueDateChanged() tea.Cmd {
	newVal := d.dueDatePick.Value()
	oldVal := d.issue.DueDate

	// Check if actually changed
	if newVal == nil && oldVal == nil {
		return nil
	}
	if newVal != nil && oldVal != nil && sameDay(*newVal, *oldVal) {
		return nil
	}
	if d.OnDueDateChanged != nil {
		d.issue.DueDate = newVal
		d.markUpdated()
		dateStr := ""
		if newVal != nil {
			dateStr = newVal.Format("2006-01-02")
		}
		return d.OnDueDateChanged(d.issue.Key, dateStr)
	}
	return nil
}

func (d *Detail) checkParentChanged() tea.Cmd {
	sel := d.parentDrop.SelectedItem()
	if sel == nil {
		return nil
	}
	oldKey := ""
	if d.issue.Parent != nil {
		oldKey = d.issue.Parent.Key
	}
	if sel.ID != oldKey && d.OnParentChanged != nil {
		if sel.ID == "" {
			d.issue.Parent = nil
			d.parentDrop.value = "—"
		} else {
			// Label is "KEY - Summary", extract key and summary
			parts := strings.SplitN(sel.Label, " - ", 2)
			summary := ""
			if len(parts) > 1 {
				summary = parts[1]
			}
			d.issue.Parent = &models.IssueSummary{Key: sel.ID, Summary: summary}
			d.parentDrop.value = sel.ID
		}
		d.markUpdated()
		return d.OnParentChanged(d.issue.Key, sel.ID)
	}
	return nil
}

func (d Detail) descMaxScroll() int {
	if d.descFocused {
		return 0
	}

	descText := d.descInput.Value()
	if descText == "" {
		descText = "No description."
	}

	// Count lines from the glamour-rendered output to match what's actually displayed.
	var totalLines int
	rendered, err := glamour.Render(descText, "dark")
	if err == nil {
		rendered = strings.TrimRight(rendered, "\n")
		totalLines = len(strings.Split(rendered, "\n"))
	} else {
		w := d.width - 1
		if w < 8 {
			w = 8
		}
		valW := w - 2 - 2
		wrapped := wordWrap(descText, valW)
		totalLines = len(strings.Split(wrapped, "\n"))
	}

	labelH := 3
	if d.labelsEditing {
		labelH = d.labelEditorHeight()
	}
	usedLines := 4*3 + labelH
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

// renderLabelEditor renders the labels field in editing mode.
// Shows existing labels as styled chips (selected one highlighted for removal),
// and a text input at the bottom for adding new labels.
func (d Detail) renderLabelEditor(width int) string {
	if width < 8 {
		width = 8
	}
	innerW := width - 2
	valW := innerW - 2

	bdr := lipgloss.NewStyle().Foreground(colorAccent)
	lbl := lipgloss.NewStyle().Foreground(colorAccent)
	labelTag := lipgloss.NewStyle().Foreground(colorInfo)
	removeTag := lipgloss.NewStyle().Foreground(colorError)

	// Top border
	labelText := " Labels ✎ "
	dashes := innerW - lipgloss.Width(labelText) - 1
	if dashes < 0 {
		dashes = 0
	}
	top := bdr.Render("╭─") + lbl.Render(labelText) + bdr.Render(strings.Repeat("─", dashes)+"╮")

	// Build wrapped label lines
	var labelLines []string
	if len(d.issue.Labels) == 0 {
		labelLines = append(labelLines, lipgloss.NewStyle().Foreground(colorSubtle).Render("No labels"))
	} else {
		var currentLine strings.Builder
		currentCol := 0
		for i, l := range d.issue.Labels {
			chip := labelTag.Render("["+l+"]") + removeTag.Render("×")
			chipVisW := lipgloss.Width(chip)

			// Wrap if this chip doesn't fit on the current line
			if currentCol > 0 && currentCol+1+chipVisW > valW {
				labelLines = append(labelLines, currentLine.String())
				currentLine.Reset()
				currentCol = 0
			}

			if currentCol > 0 {
				currentLine.WriteString(" ")
				currentCol++
			}
			currentLine.WriteString(chip)
			currentCol += chipVisW
			_ = i
		}
		if currentCol > 0 {
			labelLines = append(labelLines, currentLine.String())
		}
	}

	// Render each label line with borders
	var midLines []string
	for _, ll := range labelLines {
		visW := lipgloss.Width(ll)
		pad := valW - visW
		if pad < 0 {
			pad = 0
		}
		midLines = append(midLines, bdr.Render("│")+" "+ll+strings.Repeat(" ", pad)+" "+bdr.Render("│"))
	}
	mid := strings.Join(midLines, "\n")

	// Separator
	sep := bdr.Render("├" + strings.Repeat("─", innerW) + "┤")

	// Text input
	d.labelInput.Width = valW - lipgloss.Width(d.labelInput.Prompt)
	inputContent := d.labelInput.View()
	inputVisW := lipgloss.Width(inputContent)
	inputPad := valW - inputVisW
	if inputPad < 0 {
		inputPad = 0
	}
	inputLine := bdr.Render("│") + " " + inputContent + strings.Repeat(" ", inputPad) + " " + bdr.Render("│")

	// Bottom border
	bot := bdr.Render("╰" + strings.Repeat("─", innerW) + "╯")

	return top + "\n" + mid + "\n" + sep + "\n" + inputLine + "\n" + bot
}

// labelEditorHeight returns the number of lines the label editor will render.
func (d Detail) labelEditorHeight() int {
	w := d.width - 1
	if w < 8 {
		w = 8
	}
	valW := w - 4

	// Count wrapped lines
	wrappedLines := 1
	if len(d.issue.Labels) > 0 {
		col := 0
		for i, l := range d.issue.Labels {
			chipW := len("["+l+"]") + 1 // [name] + ×
			if i > 0 {
				chipW++ // space before chip
			}
			if col > 0 && col+chipW > valW {
				wrappedLines++
				col = len("["+l+"]") + 1
			} else {
				col += chipW
			}
		}
	}

	return 1 + wrappedLines + 1 + 1 + 1 // top + label lines + separator + input + bottom
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
	lineY := 0

	// Row 1: Title (full width, 3 lines)
	d.titleInput.Width = w - 6
	row1 := d.renderInputField("Title", &d.titleInput, w)
	b.WriteString(row1 + "\n")
	lineY += 3

	// Row 2: Parent + Key + Type + Due Date (4 columns, 3 lines)
	col4W := w / 4
	lastCol4W := w - 3*col4W // last column absorbs remainder for right-edge alignment

	d.parentDrop.width = col4W
	d.parentDrop.overlayWidth = w // overlay spans full detail pane width
	d.dueDatePick.width = lastCol4W

	row2 := joinFieldsHorizontal(
		d.parentDrop.View(),
		renderField("Key", d.issue.Key, col4W, colorAccent),
		renderField("Type", d.issue.Type.Name, col4W, colorText),
		d.dueDatePick.View(),
	)
	b.WriteString(row2 + "\n")
	lineY += 3

	// Collect row2 overlays
	if overlay := d.parentDrop.RenderStandaloneOverlay(); overlay != nil {
		overlays = append(overlays, overlayInfo{lines: overlay, y: lineY - 3, x: 0})
	}
	if overlay := d.dueDatePick.RenderOverlay(); overlay != nil {
		overlayW := d.dueDatePick.calWidth()
		fieldLeft := 3 * col4W
		fieldRight := w // last field extends to right edge
		dpX := fieldRight - overlayW
		if dpX < 0 {
			dpX = 0
		}

		// If overlay is wider than the field, prepend a connector line
		// that bridges from the overlay's left edge to the field's borders
		if overlayW > lastCol4W {
			bdr := lipgloss.NewStyle().Foreground(colorAccent)
			connFieldLeft := fieldLeft - dpX // field's left border relative to overlay
			connW := overlayW - 2            // inside the corners
			var conn strings.Builder
			conn.WriteString(bdr.Render("╭"))
			for i := 0; i < connW; i++ {
				if i == connFieldLeft-1 {
					conn.WriteString(bdr.Render("┴"))
				} else {
					conn.WriteString(bdr.Render("─"))
				}
			}
			conn.WriteString(bdr.Render("┤"))
			overlay = append([]string{conn.String()}, overlay...)
			// Position one line higher so connector overlays the field's ├──┤
			overlays = append(overlays, overlayInfo{lines: overlay, y: lineY - 1, x: dpX})
		} else {
			overlays = append(overlays, overlayInfo{lines: overlay, y: lineY, x: dpX})
		}
	}
	// Parent hover tooltip
	if d.parentHover && d.issue.Parent != nil && !d.parentDrop.IsOpen() {
		tooltip := d.issue.Parent.Key + ": " + d.issue.Parent.Summary
		tooltipStyle := lipgloss.NewStyle().Foreground(colorText).Background(colorSelection)
		tooltipLine := tooltipStyle.Render(truncStr(tooltip, w-2))
		overlays = append(overlays, overlayInfo{lines: []string{tooltipLine}, y: lineY, x: 0})
	}

	// Row 3: Assignee + Reporter + Status (3 columns, 3 lines)
	col3W := w / 3
	lastCol3W := w - 2*col3W // last column absorbs remainder

	d.assigneeDrop.width = col3W
	d.statusDrop.width = lastCol3W

	reporter := "—"
	if d.issue.Reporter != nil {
		reporter = d.issue.Reporter.DisplayName
	}

	row3 := joinFieldsHorizontal(
		d.assigneeDrop.View(),
		renderField("Reporter", reporter, col3W, colorText),
		d.statusDrop.View(),
	)
	b.WriteString(row3 + "\n")
	lineY += 3

	// Collect row3 overlays
	if overlay := d.assigneeDrop.RenderStandaloneOverlay(); overlay != nil {
		overlays = append(overlays, overlayInfo{lines: overlay, y: lineY - 3, x: 0})
	}
	if overlay := d.statusDrop.RenderStandaloneOverlay(); overlay != nil {
		overlays = append(overlays, overlayInfo{lines: overlay, y: lineY - 3, x: 2 * col3W})
	}

	// Row 4: Updated + Created + Priority (3 columns, 3 lines)
	d.priorityDrop.width = lastCol3W

	row4 := joinFieldsHorizontal(
		renderField("Updated", d.issue.Updated.Format("2006-01-02 15:04"), col3W, colorText),
		renderField("Created", d.issue.Created.Format("2006-01-02 15:04"), col3W, colorText),
		d.priorityDrop.View(),
	)
	b.WriteString(row4 + "\n")
	lineY += 3

	// Collect row4 overlays
	if overlay := d.priorityDrop.RenderStandaloneOverlay(); overlay != nil {
		overlays = append(overlays, overlayInfo{lines: overlay, y: lineY - 3, x: 2 * col3W})
	}

	// Row 5: Labels (full width, always shown)
	if d.labelsEditing {
		b.WriteString(d.renderLabelEditor(w) + "\n")
	} else {
		labelsVal := "—"
		labelsColor := colorSubtle
		if len(d.issue.Labels) > 0 {
			tags := make([]string, len(d.issue.Labels))
			for i, l := range d.issue.Labels {
				tags[i] = "[" + l + "]"
			}
			labelsVal = strings.Join(tags, " ")
			labelsColor = colorInfo
		}
		b.WriteString(renderField("Labels", labelsVal, w, labelsColor) + "\n")
	}
	labelH := 3
	if d.labelsEditing {
		labelH = d.labelEditorHeight()
	}
	lineY += labelH

	// Row 6: Description
	usedLines := 4*3 + labelH // 4 rows × 3 lines + labels
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
	var b strings.Builder
	contentWidth := d.width - 6
	if contentWidth < 20 {
		contentWidth = 20
	}

	// Comment input area at the top
	if d.commentMode == commentAdding || d.commentMode == commentReplying {
		label := "Add a comment"
		if d.commentMode == commentReplying && d.commentReplyTo >= 0 && d.commentReplyTo < len(d.issue.Comments) {
			label = "Reply to " + d.issue.Comments[d.commentReplyTo].Author.DisplayName
		}
		b.WriteString(d.renderCommentInput(label, contentWidth+4))
		b.WriteString("\n")
	} else if d.commentMode == commentEditing {
		b.WriteString(d.renderCommentInput("Edit comment", contentWidth+4))
		b.WriteString("\n")
	} else {
		addStyle := lipgloss.NewStyle().Foreground(colorSubtle).Italic(true)
		addLine := "  " + addStyle.Render("Add a comment...")
		b.WriteString(addLine + "\n")
		b.WriteString("  " + lipgloss.NewStyle().Foreground(colorBorder).Render(strings.Repeat("─", contentWidth)) + "\n")
	}

	if len(d.issue.Comments) == 0 {
		subtle := lipgloss.NewStyle().Foreground(colorSubtle)
		b.WriteString("  " + subtle.Render("No comments yet."))
		return b.String()
	}

	authorStyle := lipgloss.NewStyle().Foreground(colorSuccess).Bold(true)
	myAuthorStyle := lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	timeStyle := lipgloss.NewStyle().Foreground(colorSubtle)
	actionStyle := lipgloss.NewStyle().Foreground(colorSubtle)
	dotStyle := lipgloss.NewStyle().Foreground(colorBorder)

	for ci, c := range d.issue.Comments {
		isMine := c.Author.AccountID == d.myAccountID

		aStyle := authorStyle
		if isMine {
			aStyle = myAuthorStyle
		}
		header := "  " + aStyle.Render(c.Author.DisplayName) + "  " + timeStyle.Render(relativeTime(c.Created))
		if !c.Updated.Equal(c.Created) {
			header += "  " + timeStyle.Render("(edited)")
		}
		b.WriteString(header + "\n")

		// Render body with glamour markdown
		rendered, err := glamour.Render(c.Body, "dark")
		if err != nil {
			rendered = c.Body
		}
		rendered = strings.TrimRight(rendered, "\n")
		for _, line := range strings.Split(rendered, "\n") {
			visW := lipgloss.Width(line)
			if visW > contentWidth {
				line = truncateAnsi(line, contentWidth)
			}
			b.WriteString("  " + line + "\n")
		}

		// Delete confirmation
		if d.confirmDelete && d.confirmDeleteID == c.ID {
			confirmLine := "  " + lipgloss.NewStyle().Foreground(colorError).Render("Delete this comment? ") +
				lipgloss.NewStyle().Foreground(colorText).Bold(true).Render("[y]") +
				lipgloss.NewStyle().Foreground(colorSubtle).Render("es / ") +
				lipgloss.NewStyle().Foreground(colorText).Bold(true).Render("[n]") +
				lipgloss.NewStyle().Foreground(colorSubtle).Render("o")
			b.WriteString(confirmLine + "\n")
		} else {
			// Action buttons
			var actions []string

			replyRendered := actionStyle.Render("↩ Reply")
			actions = append(actions, replyRendered)

			if isMine {
				editRendered := actionStyle.Render("✎ Edit")
				actions = append(actions, editRendered)

				deleteRendered := actionStyle.Render("✗ Delete")
				actions = append(actions, deleteRendered)
			}
			b.WriteString("  " + strings.Join(actions, "  ") + "\n")
		}

		if ci < len(d.issue.Comments)-1 {
			b.WriteString("  " + dotStyle.Render("·  ·  ·") + "\n")
		}
	}

	return b.String()
}

func (d Detail) renderCommentInput(label string, width int) string {
	if width < 8 {
		width = 8
	}
	innerW := width - 2
	valW := innerW - 2

	bdr := lipgloss.NewStyle().Foreground(colorAccent)
	lbl := lipgloss.NewStyle().Foreground(colorAccent)
	labelText := " " + label + " ✎ "
	dashes := innerW - lipgloss.Width(labelText) - 1
	if dashes < 0 {
		dashes = 0
	}

	top := bdr.Render("╭─") + lbl.Render(labelText) + bdr.Render(strings.Repeat("─", dashes)+"╮")

	ta := d.commentInput
	ta.SetWidth(valW)
	ta.SetHeight(4)
	ta.FocusedStyle.Base = lipgloss.NewStyle()
	taView := ta.View()

	taLines := strings.Split(taView, "\n")
	var midLines []string
	for i, line := range taLines {
		if i >= 4 {
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
	for len(midLines) < 4 {
		midLines = append(midLines, emptyLine)
	}

	hint := lipgloss.NewStyle().Foreground(colorSubtle).Render("  Ctrl+S to submit · Esc to cancel")
	bot := bdr.Render("╰" + strings.Repeat("─", innerW) + "╯")

	return top + "\n" + strings.Join(midLines, "\n") + "\n" + bot + "\n" + hint
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
