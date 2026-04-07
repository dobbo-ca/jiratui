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

// isEscapeSequenceFragment returns true if the key message looks like a raw
// escape sequence fragment that leaked through as a KeyMsg. This catches mouse
// reports (e.g. "[65;133;27M"), CSI fragments, and other non-text sequences.
func isEscapeSequenceFragment(msg tea.KeyMsg) bool {
	if msg.Type != tea.KeyRunes {
		return false
	}
	runes := msg.Runes
	if len(runes) == 0 {
		return false
	}
	// Multi-rune messages starting with '[' or '<' are escape sequence fragments.
	// A real user typing '[' produces a single-rune KeyMsg.
	if len(runes) > 1 && (runes[0] == '[' || runes[0] == '<') {
		return true
	}
	// Check for semicolons (CSI parameter separators — never in normal text input)
	for _, r := range runes {
		if r == ';' {
			return true
		}
	}
	// Multi-rune messages that are all digits + uppercase letters are likely
	// escape sequence fragments (e.g. "27M", "65")
	if len(runes) > 1 {
		allDigitsOrControl := true
		for _, r := range runes {
			if !((r >= '0' && r <= '9') || r == 'M' || r == 'm') {
				allDigitsOrControl = false
				break
			}
		}
		if allDigitsOrControl {
			return true
		}
	}
	return false
}

type detailTab int

const (
	tabDetails detailTab = iota
	tabComments
	tabAssociations
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
	descInput          textarea.Model
	descFocused        bool
	descEscSeqPending  bool // true after receiving a lone '[' that may be an escape sequence start
	descViewScrollY    int  // independent viewport scroll offset for description editor
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

	// Link interaction state
	linkAdding        bool
	linkTypeDrop      Dropdown
	linkTargetInput   textinput.Model
	linkTypeSelected  string
	linkDirection     string
	linkTypes         []models.LinkType

	// Link callbacks
	OnLinkCreate      func(issueKey, targetKey, linkTypeName, direction string) tea.Cmd
	OnLinkDelete      func(linkID string) tea.Cmd

	// Link delete confirmation
	confirmLinkDelete   bool
	confirmLinkDeleteID string

	// Navigation callback — navigates to a different issue in the detail pane
	OnNavigate func(issueKey string) tea.Cmd

	// Attachment callbacks
	OnAttachmentOpen   func(attachment models.Attachment) tea.Cmd
	OnAttachmentUpload func(issueKey, filePath string) tea.Cmd
	OnAttachmentDelete func(issueKey, attachmentID string) tea.Cmd
	downloadingAttach  string // filename currently downloading, "" if none
	uploadingAttach    bool   // true while upload is in progress
	confirmAttachDel   bool
	confirmAttachDelID string

	// Change callbacks — each fires a tea.Cmd that performs the Jira API update
	OnLabelsChanged       func(issueKey string, labels []string) tea.Cmd
	OnSummaryChanged      func(issueKey, summary string) tea.Cmd
	OnDescriptionChanged  func(issueKey, description string) tea.Cmd
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

	linkTypeDrop := NewSimpleDropdown("Link Type", nil, "Select type...", "", 0)
	linkTarget := textinput.New()
	linkTarget.Placeholder = "Issue key (e.g. TEC-123)"
	linkTarget.CharLimit = 30
	linkTarget.Prompt = ""

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
		linkTypeDrop:    linkTypeDrop,
		linkTargetInput: linkTarget,
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

// SetLinkTypes populates the link type dropdown with available link types.
func (d *Detail) SetLinkTypes(types []models.LinkType) {
	d.linkTypes = types
	var items []DropdownItem
	for _, lt := range types {
		items = append(items, DropdownItem{ID: lt.Name + "|outward", Label: lt.Outward})
		if lt.Inward != lt.Outward {
			items = append(items, DropdownItem{ID: lt.Name + "|inward", Label: lt.Inward})
		}
	}
	d.linkTypeDrop.SetItems(items)
}

// Editing returns true when any input field in the detail view is focused.
func (d Detail) Editing() bool {
	return d.titleInput.Focused() || d.descFocused ||
		d.assigneeDrop.IsOpen() || d.statusDrop.IsOpen() ||
		d.priorityDrop.IsOpen() || d.dueDatePick.IsOpen() ||
		d.parentDrop.IsOpen() || d.labelsEditing ||
		d.commentMode != commentIdle || d.confirmDelete ||
		d.linkAdding || d.confirmLinkDelete ||
		d.confirmAttachDel
}

// tabLabels returns the display labels for each tab.
func (d Detail) tabLabels() []string {
	assocCount := len(d.issue.Links) + len(d.issue.Subtasks)
	return []string{
		"Details",
		fmt.Sprintf("Comments(%d)", len(d.issue.Comments)),
		fmt.Sprintf("Associations(%d)", assocCount),
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
				d.descEscSeqPending = false
				d.descInput.Blur()
				return d, d.checkDescriptionChanged()
			}
			if msg.String() == "ctrl+s" {
				d.descFocused = false
				d.descEscSeqPending = false
				d.descInput.Blur()
				return d, d.checkDescriptionChanged()
			}
			// Page up/down: move cursor by one page of visual lines, viewport follows
			if msg.Type == tea.KeyPgDown {
				visH := d.descVisibleLines()
				for i := 0; i < visH; i++ {
					d.descInput.CursorDown()
				}
				// Scroll viewport to follow cursor
				curVis := d.descCursorVisualLine()
				if curVis >= d.descViewScrollY+visH {
					d.descViewScrollY = curVis - visH + 1
				}
				max := d.descMaxEditScroll()
				if d.descViewScrollY > max {
					d.descViewScrollY = max
				}
				return d, nil
			}
			if msg.Type == tea.KeyPgUp {
				visH := d.descVisibleLines()
				for i := 0; i < visH; i++ {
					d.descInput.CursorUp()
				}
				// Scroll viewport to follow cursor
				curVis := d.descCursorVisualLine()
				if curVis < d.descViewScrollY {
					d.descViewScrollY = curVis
				}
				return d, nil
			}
			if isEscapeSequenceFragment(msg) {
				d.descEscSeqPending = false
				return d, nil
			}
			// A lone '[' may be the start of a split escape sequence.
			// Hold it — if the next message is an escape fragment, discard both.
			if msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == '[' {
				if d.descEscSeqPending {
					// Two consecutive '[' — first was real, insert it
					d.descInput, _ = d.descInput.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}})
				}
				d.descEscSeqPending = true
				return d, nil
			}
			// If we had a pending '[' and this is normal input, insert the '[' first
			if d.descEscSeqPending {
				d.descEscSeqPending = false
				d.descInput, _ = d.descInput.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}})
			}
			var cmd tea.Cmd
			d.descInput, cmd = d.descInput.Update(msg)
			// Auto-scroll viewport to keep cursor visible (using visual/wrapped lines)
			curVisLine := d.descCursorVisualLine()
			visH := d.descVisibleLines()
			if curVisLine < d.descViewScrollY {
				d.descViewScrollY = curVisLine
			}
			if curVisLine >= d.descViewScrollY+visH {
				d.descViewScrollY = curVisLine - visH + 1
			}
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
			if isEscapeSequenceFragment(msg) {
				return d, nil
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

		// Link add flow
		if d.activeTab == tabAssociations && d.linkAdding {
			if msg.String() == "esc" {
				d.cancelLinkAdd()
				return d, nil
			}
			if d.linkTypeDrop.IsOpen() {
				var cmd tea.Cmd
				d.linkTypeDrop, cmd = d.linkTypeDrop.Update(msg)
				if !d.linkTypeDrop.IsOpen() {
					sel := d.linkTypeDrop.SelectedItem()
					if sel != nil {
						parts := strings.SplitN(sel.ID, "|", 2)
						d.linkTypeSelected = parts[0]
						d.linkDirection = sel.Label
						d.linkTargetInput.Focus()
						return d, d.linkTargetInput.Cursor.BlinkCmd()
					} else {
						d.cancelLinkAdd()
					}
				}
				return d, cmd
			}
			if d.linkTargetInput.Focused() {
				if msg.Type == tea.KeyEnter {
					return d, d.submitLink()
				}
				var cmd tea.Cmd
				d.linkTargetInput, cmd = d.linkTargetInput.Update(msg)
				return d, cmd
			}
			return d, nil
		}

		// Link delete confirmation
		if d.confirmLinkDelete {
			switch msg.String() {
			case "y":
				linkID := d.confirmLinkDeleteID
				d.confirmLinkDelete = false
				d.confirmLinkDeleteID = ""
				if d.OnLinkDelete != nil {
					return d, d.OnLinkDelete(linkID)
				}
				return d, nil
			case "n", "esc":
				d.confirmLinkDelete = false
				d.confirmLinkDeleteID = ""
				return d, nil
			}
			return d, nil
		}

		// Attachment delete confirmation
		if d.confirmAttachDel {
			switch msg.String() {
			case "y":
				attID := d.confirmAttachDelID
				d.confirmAttachDel = false
				d.confirmAttachDelID = ""
				if d.OnAttachmentDelete != nil {
					return d, d.OnAttachmentDelete(d.issue.Key, attID)
				}
				return d, nil
			case "n", "esc":
				d.confirmAttachDel = false
				d.confirmAttachDelID = ""
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
			d.activeTab = tabAssociations
			d.scrollY = 0
		case key.Matches(msg, detailKeys.Tab4):
			d.activeTab = tabAttachments
			d.scrollY = 0
		case key.Matches(msg, detailKeys.Down):
			maxScroll := d.tabMaxScroll()
			if d.scrollY < maxScroll {
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

		// When description editor is focused, scroll the viewport independently of cursor
		if d.descFocused {
			if msg.Button == tea.MouseButtonWheelDown {
				d.descViewScrollY += 3
				max := d.descMaxEditScroll()
				if d.descViewScrollY > max {
					d.descViewScrollY = max
				}
				return d, nil
			}
			if msg.Button == tea.MouseButtonWheelUp {
				d.descViewScrollY -= 3
				if d.descViewScrollY < 0 {
					d.descViewScrollY = 0
				}
				return d, nil
			}
		}
		// When comment editor is focused, consume wheel events
		if d.commentMode != commentIdle {
			if msg.Button == tea.MouseButtonWheelDown || msg.Button == tea.MouseButtonWheelUp {
				return d, nil
			}
		}

		// Handle wheel events for read-only scrolling
		if msg.Button == tea.MouseButtonWheelDown {
			maxScroll := d.tabMaxScroll()
			if d.scrollY < maxScroll {
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
			} else if d.activeTab == tabAssociations {
				return d, d.handleAssociationClick(msg.X, msg.Y-2)
			} else if d.activeTab == tabAttachments {
				return d, d.handleAttachmentClick(msg.X, msg.Y-2)
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
			// Re-wrap and clamp edit scroll on resize
			if d.descFocused {
				// Update textarea width to match new size
				innerW := d.width - 1 - 2 - 2
				if innerW < 20 {
					innerW = 20
				}
				d.descInput.SetWidth(innerW)
				// Clamp scroll and re-sync cursor visibility
				max := d.descMaxEditScroll()
				if d.descViewScrollY > max {
					d.descViewScrollY = max
				}
				curVis := d.descCursorVisualLine()
				visH := d.descVisibleLines()
				if curVis < d.descViewScrollY {
					d.descViewScrollY = curVis
				}
				if curVis >= d.descViewScrollY+visH {
					d.descViewScrollY = curVis - visH + 1
				}
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
	if d.linkAdding {
		if d.linkTypeDrop.IsOpen() {
			var cmd tea.Cmd
			d.linkTypeDrop, cmd = d.linkTypeDrop.Update(msg)
			return d, cmd
		}
		if d.linkTargetInput.Focused() {
			var cmd tea.Cmd
			d.linkTargetInput, cmd = d.linkTargetInput.Update(msg)
			return d, cmd
		}
	}
	if d.descFocused {
		// Only forward safe message types to the textarea (cursor blink, etc.)
		// Block mouse events and any stray key messages that bypassed the KeyMsg handler
		switch m := msg.(type) {
		case tea.MouseMsg:
			return d, nil
		case tea.KeyMsg:
			// KeyMsg shouldn't reach here (handled above), but filter just in case
			if isEscapeSequenceFragment(m) {
				return d, nil
			}
		}
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

// blurAllAndSave unfocuses all fields and saves any pending changes.
func (d *Detail) blurAllAndSave() tea.Cmd {
	var cmds []tea.Cmd
	if d.descFocused {
		cmds = append(cmds, d.checkDescriptionChanged())
	}
	if d.titleInput.Focused() {
		cmds = append(cmds, d.checkSummaryChanged())
	}
	d.blurAll()
	return tea.Batch(cmds...)
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
	d.cancelLinkAdd()
	d.confirmLinkDelete = false
	d.confirmLinkDeleteID = ""
	d.confirmAttachDel = false
	d.confirmAttachDelID = ""
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

func (d *Detail) startLinkAdd() tea.Cmd {
	d.blurAll()
	d.linkAdding = true
	d.linkTypeSelected = ""
	d.linkDirection = ""
	d.linkTargetInput.SetValue("")
	return d.linkTypeDrop.OpenDropdown()
}

func (d *Detail) cancelLinkAdd() {
	d.linkAdding = false
	d.linkTypeDrop.Close()
	d.linkTargetInput.Blur()
	d.linkTargetInput.SetValue("")
	d.linkTypeSelected = ""
	d.linkDirection = ""
}

func (d *Detail) submitLink() tea.Cmd {
	target := strings.TrimSpace(d.linkTargetInput.Value())
	if target == "" || d.linkTypeSelected == "" {
		d.cancelLinkAdd()
		return nil
	}
	issueKey := d.issue.Key
	typeName := d.linkTypeSelected
	direction := d.linkDirection
	d.cancelLinkAdd()
	if d.OnLinkCreate != nil {
		return d.OnLinkCreate(issueKey, target, typeName, direction)
	}
	return nil
}

func (d *Detail) startLinkDelete(linkID string) {
	d.confirmLinkDelete = true
	d.confirmLinkDeleteID = linkID
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

func (d *Detail) handleAssociationClick(x, y int) tea.Cmd {
	adjustedY := y + d.scrollY

	// "Add link..." prompt (first line when not adding)
	if !d.linkAdding && adjustedY == 0 {
		return d.startLinkAdd()
	}

	// Walk through the rendered layout to find delete buttons
	lineY := 2 // skip add prompt + separator
	if d.linkAdding {
		lineY = 4 // input takes more space
	}

	if len(d.issue.Links) > 0 {
		// Reconstruct group order to match rendering
		groups := make(map[string][]models.IssueLink)
		var groupOrder []string
		for _, link := range d.issue.Links {
			if _, exists := groups[link.Type]; !exists {
				groupOrder = append(groupOrder, link.Type)
			}
			groups[link.Type] = append(groups[link.Type], link)
		}

		for gi, groupName := range groupOrder {
			lineY++ // group header
			for _, link := range groups[groupName] {
				if adjustedY == lineY {
					// Delete button at right edge
					if x >= d.width-10 {
						d.startLinkDelete(link.ID)
						return nil
					}
					// Click on issue key area — navigate to that issue
					var issueKey string
					if link.OutwardIssue != nil {
						issueKey = link.OutwardIssue.Key
					} else if link.InwardIssue != nil {
						issueKey = link.InwardIssue.Key
					}
					if issueKey != "" && d.OnNavigate != nil {
						return d.OnNavigate(issueKey)
					}
				}
				lineY++
			}
			if gi < len(groupOrder)-1 || len(d.issue.Subtasks) > 0 {
				lineY++ // separator
			}
		}
	}

	// Subtasks section
	if len(d.issue.Subtasks) > 0 {
		lineY++ // "Subtasks" header
		for _, st := range d.issue.Subtasks {
			if adjustedY == lineY && d.OnNavigate != nil {
				return d.OnNavigate(st.Key)
			}
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

		// Click within the description area while editing — reposition cursor
		if d.descFocused && y >= row6Y {
			visualLine := (y - row6Y - 1) + d.descViewScrollY
			if visualLine < 0 {
				visualLine = 0
			}
			clickCol := x - 2
			if clickCol < 0 {
				clickCol = 0
			}
			d.descSetCursorToVisualLine(visualLine, clickCol)
			return nil
		}

		return d.blurAllAndSave()
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
		// Account for the read-only scroll offset that was active before editing
		savedScrollY := d.scrollY
		d.blurAll()
		d.descFocused = true
		d.descViewScrollY = 0
		d.descInput.Focus()
		// Map visual click position to logical line + column
		// Add the read-only scroll offset since the content was scrolled
		visualLine := (y - row6Y - 1) + savedScrollY
		if visualLine < 0 {
			visualLine = 0
		}
		clickCol := x - 2
		if clickCol < 0 {
			clickCol = 0
		}
		d.descSetCursorToVisualLine(visualLine, clickCol)
		// Set edit scroll to match where the user was viewing
		d.descViewScrollY = savedScrollY
		max := d.descMaxEditScroll()
		if d.descViewScrollY > max {
			d.descViewScrollY = max
		}
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

// descVisibleLines returns how many lines of the description editor are visible.
func (d *Detail) descVisibleLines() int {
	// detail height minus fixed rows (tab bar=2, title=3, row2=3, row3=3, row4=3, labels=3, border=2)
	h := d.height - 19
	if h < 3 {
		h = 3
	}
	return h
}

// descTextWidth returns the inner width available for description text.
func (d *Detail) descTextWidth() int {
	w := d.width - 1 - 2 - 2 // outer - border - padding
	if w < 10 {
		w = 10
	}
	return w
}

// descWrappedHeight returns the number of visual lines a logical line wraps to,
// matching the textarea's internal wrap algorithm (rune-based at textarea width).
func (d *Detail) descWrappedHeight(line string) int {
	w := d.descTextWidth()
	if w <= 0 {
		return 1
	}
	runeLen := utf8.RuneCountInString(line)
	if runeLen == 0 {
		return 1
	}
	wrapped := (runeLen + w - 1) / w
	return wrapped
}

// descLogicalToVisual returns the visual (wrapped) line number for a given
// logical line. It counts how many visual lines all prior logical lines take.
func (d *Detail) descLogicalToVisual(logicalLine int) int {
	val := d.descInput.Value()
	lines := strings.Split(val, "\n")
	visual := 0
	for i := 0; i < logicalLine && i < len(lines); i++ {
		visual += d.descWrappedHeight(lines[i])
	}
	return visual
}

// descVisualToLogical converts a visual (wrapped) line number to a logical
// line number and the sub-line offset within that logical line.
func (d *Detail) descVisualToLogical(visualLine int) (logicalLine, subLine int) {
	val := d.descInput.Value()
	lines := strings.Split(val, "\n")
	visual := 0
	for i, line := range lines {
		h := d.descWrappedHeight(line)
		if visual+h > visualLine {
			return i, visualLine - visual
		}
		visual += h
	}
	if len(lines) > 0 {
		return len(lines) - 1, 0
	}
	return 0, 0
}

// descTotalVisualLines returns the total number of visual (wrapped) lines.
func (d *Detail) descTotalVisualLines() int {
	val := d.descInput.Value()
	lines := strings.Split(val, "\n")
	total := 0
	for _, line := range lines {
		total += d.descWrappedHeight(line)
	}
	return total
}

// descMaxEditScroll returns the max scroll offset for the description editor.
func (d *Detail) descMaxEditScroll() int {
	max := d.descTotalVisualLines() - d.descVisibleLines()
	if max < 0 {
		return 0
	}
	return max
}

// descCursorVisualLine returns the visual (wrapped) line the cursor is currently on.
func (d *Detail) descCursorVisualLine() int {
	logLine := d.descInput.Line()
	vis := d.descLogicalToVisual(logLine)
	// Add sub-line offset from LineInfo
	li := d.descInput.LineInfo()
	vis += li.RowOffset
	return vis
}

// descSetCursorToVisualLine moves the textarea cursor to the given visual
// (wrapped) line by navigating from the top. Each CursorDown moves one
// visual line (including wrapped sub-lines).
func (d *Detail) descSetCursorToVisualLine(visualLine, col int) {
	// Go to the very top first
	for d.descInput.Line() > 0 || d.descInput.LineInfo().RowOffset > 0 {
		d.descInput.CursorUp()
	}
	d.descInput.CursorStart()
	// Navigate down by visual lines
	for i := 0; i < visualLine; i++ {
		d.descInput.CursorDown()
	}
	if col < 0 {
		col = 0
	}
	// SetCursor sets column within the current logical line's sub-line
	d.descInput.SetCursor(d.descInput.LineInfo().StartColumn + col)
}

func (d *Detail) checkDescriptionChanged() tea.Cmd {
	newVal := d.descInput.Value()
	if newVal != d.issue.Description && d.OnDescriptionChanged != nil {
		d.issue.Description = newVal
		d.markUpdated()
		return d.OnDescriptionChanged(d.issue.Key, newVal)
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

// tabMaxScroll returns the max scroll for the current tab.
func (d Detail) tabMaxScroll() int {
	switch d.activeTab {
	case tabDetails:
		return d.descMaxScroll()
	case tabComments:
		// Estimate: each comment ~6 lines + header
		lines := len(d.issue.Comments) * 8
		maxScroll := lines - d.height + 4
		if maxScroll < 0 {
			return 0
		}
		return maxScroll
	case tabAssociations:
		lines := len(d.issue.Links)*2 + len(d.issue.Subtasks)*2 + 4
		maxScroll := lines - d.height + 4
		if maxScroll < 0 {
			return 0
		}
		return maxScroll
	case tabAttachments:
		lines := len(d.issue.Attachments)*2 + 2
		maxScroll := lines - d.height + 4
		if maxScroll < 0 {
			return 0
		}
		return maxScroll
	}
	return 0
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
		case tabAssociations:
			baseContent = d.renderAssociationsTab()
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
		// Set height very large so textarea renders ALL lines without internal scrolling.
		// We apply our own viewport slice via descViewScrollY.
		ta.SetHeight(500)
		ta.FocusedStyle.Base = lipgloss.NewStyle()
		taView := ta.View()

		taLines := strings.Split(taView, "\n")

		// Use our own computed total visual lines for precise clamping (0 buffer)
		totalVisual := d.descTotalVisualLines()
		if totalVisual < 1 {
			totalVisual = 1
		}

		// Apply independent viewport scroll, clamped to actual content
		maxScroll := totalVisual - contentLines
		if maxScroll < 0 {
			maxScroll = 0
		}
		scrollOff := d.descViewScrollY
		if scrollOff > maxScroll {
			scrollOff = maxScroll
		}
		if scrollOff < 0 {
			scrollOff = 0
		}

		var midLines []string
		for i := scrollOff; i < scrollOff+contentLines && i < len(taLines); i++ {
			line := taLines[i]
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

func (d Detail) renderAssociationsTab() string {
	var b strings.Builder
	contentWidth := d.width - 6
	if contentWidth < 20 {
		contentWidth = 20
	}

	typeStyle := lipgloss.NewStyle().Foreground(colorPurple).Bold(true)
	accentStyle := lipgloss.NewStyle().Foreground(colorAccent)
	textStyle := lipgloss.NewStyle().Foreground(colorText)
	subtleStyle := lipgloss.NewStyle().Foreground(colorSubtle)
	dotStyle := lipgloss.NewStyle().Foreground(colorBorder)
	deleteStyle := lipgloss.NewStyle().Foreground(colorSubtle)

	// "Add link..." prompt or active input
	if d.linkAdding {
		if d.linkTypeDrop.IsOpen() {
			b.WriteString("  " + lipgloss.NewStyle().Foreground(colorAccent).Render("Select link type:") + "\n")
		} else if d.linkTargetInput.Focused() {
			b.WriteString("  " + lipgloss.NewStyle().Foreground(colorAccent).Render(d.linkDirection) + " ")
			b.WriteString(d.linkTargetInput.View() + "\n")
			b.WriteString("  " + subtleStyle.Render("Enter to submit · Esc to cancel") + "\n")
		}
		b.WriteString("\n")
	} else {
		addStyle := lipgloss.NewStyle().Foreground(colorSubtle).Italic(true)
		b.WriteString("  " + addStyle.Render("Add link...") + "\n")
		b.WriteString("  " + dotStyle.Render(strings.Repeat("─", contentWidth)) + "\n")
	}

	// --- Links grouped by type ---
	if len(d.issue.Links) > 0 {
		type groupedLink struct {
			issueKey string
			summary  string
			status   string
			linkID   string
		}
		groups := make(map[string][]groupedLink)
		var groupOrder []string
		for _, link := range d.issue.Links {
			var gl groupedLink
			gl.linkID = link.ID
			if link.OutwardIssue != nil {
				gl.issueKey = link.OutwardIssue.Key
				gl.summary = link.OutwardIssue.Summary
				gl.status = link.OutwardIssue.Status.Name
			} else if link.InwardIssue != nil {
				gl.issueKey = link.InwardIssue.Key
				gl.summary = link.InwardIssue.Summary
				gl.status = link.InwardIssue.Status.Name
			}
			if _, exists := groups[link.Type]; !exists {
				groupOrder = append(groupOrder, link.Type)
			}
			groups[link.Type] = append(groups[link.Type], gl)
		}

		for gi, groupName := range groupOrder {
			b.WriteString("  " + typeStyle.Render(groupName) + "\n")
			for _, gl := range groups[groupName] {
				sumW := contentWidth - len(gl.issueKey) - 18
				if sumW < 10 {
					sumW = 10
				}
				line := "    " + accentStyle.Render(gl.issueKey) + " " + textStyle.Render(truncStr(gl.summary, sumW)) + " " + StyledStatus(gl.status)
				// Delete button
				if d.confirmLinkDelete && d.confirmLinkDeleteID == gl.linkID {
					line = "    " + lipgloss.NewStyle().Foreground(colorError).Render("Delete this link? ") +
						lipgloss.NewStyle().Foreground(colorText).Bold(true).Render("[y]") +
						lipgloss.NewStyle().Foreground(colorSubtle).Render("es / ") +
						lipgloss.NewStyle().Foreground(colorText).Bold(true).Render("[n]") +
						lipgloss.NewStyle().Foreground(colorSubtle).Render("o")
				} else {
					line += "  " + deleteStyle.Render("✗")
				}
				b.WriteString(line + "\n")
			}
			if gi < len(groupOrder)-1 || len(d.issue.Subtasks) > 0 {
				b.WriteString("  " + dotStyle.Render("·  ·  ·") + "\n")
			}
		}
	}

	// --- Subtasks ---
	if len(d.issue.Subtasks) > 0 {
		b.WriteString("  " + typeStyle.Render("Subtasks") + "\n")
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
			b.WriteString("    " + indicator + " " + accentStyle.Render(st.Key) + " " + textStyle.Render(st.Summary) + " " + StyledStatus(st.Status.Name) + "\n")
		}
	}

	if len(d.issue.Links) == 0 && len(d.issue.Subtasks) == 0 {
		return "  " + subtleStyle.Render("No associations.")
	}

	return b.String()
}

func (d Detail) renderAttachmentsTab() string {
	var b strings.Builder
	contentWidth := d.width - 6
	if contentWidth < 20 {
		contentWidth = 20
	}
	dotStyle := lipgloss.NewStyle().Foreground(colorBorder)

	// Upload prompt
	if d.uploadingAttach {
		b.WriteString("  " + lipgloss.NewStyle().Foreground(colorWarning).Render("⏳ Uploading...") + "\n")
	} else {
		addStyle := lipgloss.NewStyle().Foreground(colorSubtle).Italic(true)
		b.WriteString("  " + addStyle.Render("Upload file...") + "\n")
	}
	b.WriteString("  " + dotStyle.Render(strings.Repeat("─", contentWidth)) + "\n")

	if len(d.issue.Attachments) == 0 {
		subtle := lipgloss.NewStyle().Foreground(colorSubtle)
		b.WriteString("  " + subtle.Render("No attachments."))
		return b.String()
	}

	nameStyle := lipgloss.NewStyle().Foreground(colorAccent)
	sizeStyle := lipgloss.NewStyle().Foreground(colorSubtle)
	typeStyle := lipgloss.NewStyle().Foreground(colorInfo)

	deleteStyle := lipgloss.NewStyle().Foreground(colorSubtle)

	for _, att := range d.issue.Attachments {
		if d.confirmAttachDel && d.confirmAttachDelID == att.ID {
			line := "  " + nameStyle.Render(att.Filename) + "  " +
				lipgloss.NewStyle().Foreground(colorError).Render("Delete? ") +
				lipgloss.NewStyle().Foreground(colorText).Bold(true).Render("[y]") +
				lipgloss.NewStyle().Foreground(colorSubtle).Render("es / ") +
				lipgloss.NewStyle().Foreground(colorText).Bold(true).Render("[n]") +
				lipgloss.NewStyle().Foreground(colorSubtle).Render("o")
			b.WriteString(line + "\n")
			continue
		}
		sizeStr := formatFileSize(att.Size)
		line := "  " + nameStyle.Render(att.Filename) + "  " + sizeStyle.Render(sizeStr) + "  " + typeStyle.Render(att.MimeType)
		if d.downloadingAttach == att.Filename {
			line += "  " + lipgloss.NewStyle().Foreground(colorWarning).Render("⏳ Downloading...")
		}
		line += "  " + deleteStyle.Render("✗")
		b.WriteString(line + "\n")
	}
	return b.String()
}

func isImageMIME(mime string) bool {
	return strings.HasPrefix(mime, "image/")
}

func (d *Detail) handleAttachmentClick(x, y int) tea.Cmd {
	adjustedY := y + d.scrollY

	// "Upload file..." prompt is line 0
	if adjustedY == 0 && !d.uploadingAttach {
		d.uploadingAttach = true
		if d.OnAttachmentUpload != nil {
			return d.OnAttachmentUpload(d.issue.Key, "")
		}
		return nil
	}

	// Attachments start at line 2 (upload prompt + separator)
	idx := adjustedY - 2
	if idx < 0 || idx >= len(d.issue.Attachments) {
		return nil
	}
	att := d.issue.Attachments[idx]

	// Compute where the ✗ starts: measure the line content before it
	sizeStr := formatFileSize(att.Size)
	linePrefix := "  " + att.Filename + "  " + sizeStr + "  " + att.MimeType + "  "
	deleteX := len(linePrefix)

	// Delete click (✗ at end of line)
	if x >= deleteX {
		d.confirmAttachDel = true
		d.confirmAttachDelID = att.ID
		return nil
	}

	// Open click (image files only)
	if !isImageMIME(att.MimeType) {
		return nil
	}
	if d.downloadingAttach != "" {
		return nil
	}
	if d.OnAttachmentOpen != nil {
		d.downloadingAttach = att.Filename
		return d.OnAttachmentOpen(att)
	}
	return nil
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
