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
}

// NewDetail creates a new detail model for the given issue.
func NewDetail(issue models.Issue, width, height int) Detail {
	return Detail{
		issue:  issue,
		width:  width,
		height: height,
	}
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
			d.scrollY++
		case key.Matches(msg, detailKeys.Up):
			if d.scrollY > 0 {
				d.scrollY--
			}
		case key.Matches(msg, detailKeys.Open):
			_ = browser.OpenURL(d.issue.BrowseURL)
		}
		return d, nil

	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonLeft:
			if msg.Y <= 1 {
				d.handleTabClick(msg.X)
			}
			return d, nil
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

// handleTabClick switches tab based on X click position.
func (d *Detail) handleTabClick(x int) {
	labels := d.tabLabels()
	pos := 1 // initial padding
	for i, label := range labels {
		if i > 0 {
			pos++ // separator space
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

// View renders the detail view.
func (d Detail) View() string {
	var b strings.Builder

	b.WriteString(d.renderTabBar())
	b.WriteString("\n")

	var content string
	switch d.activeTab {
	case tabDetails:
		content = d.renderDetailsTab()
	case tabComments:
		content = d.renderCommentsTab()
	case tabSubtasks:
		content = d.renderSubtasksTab()
	case tabLinks:
		content = d.renderLinksTab()
	case tabAttachments:
		content = d.renderAttachmentsTab()
	}

	b.WriteString(d.applyScroll(content))

	return b.String()
}

func (d Detail) renderTabBar() string {
	activeStyle := lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	inactiveStyle := lipgloss.NewStyle().Foreground(colorSubtle)
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
			tabLine.WriteString(inactiveStyle.Render(numLabel))
			divLine.WriteString(dividerStyle.Render(strings.Repeat("─", w)))
		}
	}

	// Fill remaining width with regular divider
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
	viewHeight := d.height - 2 // tab labels + divider
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

// renderField renders a bordered form field: ╭─ Label ───╮ │ value │ ╰───────────╯
func renderField(label, value string, width int, valueColor lipgloss.Color) string {
	if width < 8 {
		width = 8
	}
	bdr := lipgloss.NewStyle().Foreground(colorBorder)
	lbl := lipgloss.NewStyle().Foreground(colorSubtle)

	innerW := width - 2 // left + right border chars
	labelText := " " + label + " "
	dashes := innerW - len(labelText) - 1 // -1 for initial ─
	if dashes < 0 {
		dashes = 0
	}
	top := bdr.Render("╭─") + lbl.Render(labelText) + bdr.Render(strings.Repeat("─", dashes)+"╮")

	valW := innerW - 2 // padding inside
	displayVal := truncStr(value, valW)
	pad := valW - len(displayVal)
	if pad < 0 {
		pad = 0
	}
	mid := bdr.Render("│") + " " + lipgloss.NewStyle().Foreground(valueColor).Render(displayVal) + strings.Repeat(" ", pad) + " " + bdr.Render("│")

	bot := bdr.Render("╰" + strings.Repeat("─", innerW) + "╯")

	return top + "\n" + mid + "\n" + bot
}

// renderFieldStyled renders a bordered field with a pre-styled value string.
func renderFieldStyled(label, styledValue string, width int) string {
	if width < 8 {
		width = 8
	}
	bdr := lipgloss.NewStyle().Foreground(colorBorder)
	lbl := lipgloss.NewStyle().Foreground(colorSubtle)

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

// renderFieldMultiline renders a bordered field with word-wrapped content.
// minContentLines sets the minimum number of content lines (0 = fit to content).
func renderFieldMultiline(label, text string, width, minContentLines int, valueColor lipgloss.Color) string {
	if width < 8 {
		width = 8
	}
	bdr := lipgloss.NewStyle().Foreground(colorBorder)
	lbl := lipgloss.NewStyle().Foreground(colorSubtle)

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
		pad := valW - len(line)
		if pad < 0 {
			pad = 0
		}
		midLines = append(midLines,
			bdr.Render("│")+" "+lipgloss.NewStyle().Foreground(valueColor).Render(line)+strings.Repeat(" ", pad)+" "+bdr.Render("│"))
	}

	// Pad to minimum height
	emptyLine := bdr.Render("│") + " " + strings.Repeat(" ", valW) + " " + bdr.Render("│")
	for len(midLines) < minContentLines {
		midLines = append(midLines, emptyLine)
	}

	bot := bdr.Render("╰" + strings.Repeat("─", innerW) + "╯")

	return top + "\n" + strings.Join(midLines, "\n") + "\n" + bot
}

// joinFieldsHorizontal places rendered fields side by side with a space between.
func joinFieldsHorizontal(fields ...string) string {
	return lipgloss.JoinHorizontal(lipgloss.Top, fields...)
}

// ── Details tab ───────────────────────────────────────────────

func (d Detail) renderDetailsTab() string {
	var b strings.Builder
	w := d.width - 1 // right margin to prevent overflow
	gap := 0

	// Row 1: Key + Title
	keyW := 20
	titleW := w - keyW - gap
	if titleW < 20 {
		titleW = 20
	}
	row1 := joinFieldsHorizontal(
		renderField("Key", d.issue.Key, keyW, colorAccent),
		renderField("Title", d.issue.Summary, titleW, colorText),
	)
	b.WriteString(row1 + "\n")

	// Row 2: Assignee + Status + Type
	col3W := (w - 2*gap) / 3
	col3Rem := w - 3*col3W - 2*gap

	assignee := "Unassigned"
	assigneeColor := colorSubtle
	if d.issue.Assignee != nil {
		assignee = d.issue.Assignee.DisplayName
		assigneeColor = colorSuccess
	}
	row2 := joinFieldsHorizontal(
		renderField("Assignee", assignee, col3W, assigneeColor),
		renderFieldStyled("Status", StyledStatus(d.issue.Status.Name), col3W),
		renderField("Type", d.issue.Type.Name, col3W+col3Rem, colorText),
	)
	b.WriteString(row2 + "\n")

	// Row 3: Priority + Due Date + Reporter
	pColor := priorityColor(d.issue.Priority.Name)
	pValue := lipgloss.NewStyle().Foreground(pColor).Render("● " + d.issue.Priority.Name)

	dueStr := "—"
	dueColor := colorSubtle
	if d.issue.DueDate != nil {
		dueStr = d.issue.DueDate.Format("2006-01-02")
		now := time.Now()
		if d.issue.DueDate.Before(now) {
			dueColor = colorError
		} else if d.issue.DueDate.Before(now.Add(7 * 24 * time.Hour)) {
			dueColor = colorWarning
		} else {
			dueColor = colorText
		}
	}

	reporter := "—"
	if d.issue.Reporter != nil {
		reporter = d.issue.Reporter.DisplayName
	}

	row3 := joinFieldsHorizontal(
		renderFieldStyled("Priority", pValue, col3W),
		renderField("Due Date", dueStr, col3W, dueColor),
		renderField("Reporter", reporter, col3W+col3Rem, colorText),
	)
	b.WriteString(row3 + "\n")

	// Row 4: Parent + Created + Updated
	parentStr := "—"
	parentColor := colorSubtle
	if d.issue.Parent != nil {
		parentStr = d.issue.Parent.Key + " " + d.issue.Parent.Summary
		parentColor = colorAccent
	}

	updatedStr := relativeTime(d.issue.Updated)

	row4 := joinFieldsHorizontal(
		renderField("Parent", parentStr, col3W, parentColor),
		renderField("Created", d.issue.Created.Format("2006-01-02 15:04"), col3W, colorText),
		renderField("Updated", updatedStr, col3W+col3Rem, colorText),
	)
	b.WriteString(row4 + "\n")

	// Row 5: Sprint + Labels
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
	}

	// Row 6: Description — fills remaining vertical space
	// Calculate lines used so far: 4 rows × 3 lines each + 4 newlines between rows = 16 lines
	usedLines := 4 * 3 // 4 form rows, 3 lines each (top/mid/bot)
	if d.issue.Sprint != "" || len(d.issue.Labels) > 0 {
		usedLines += 3 // sprint/labels row
	}
	// Available height for description: total content height minus used lines minus 2 (top+bot border)
	availH := d.height - 2 - usedLines - 2
	if availH < 3 {
		availH = 3
	}

	descText := d.issue.Description
	if descText == "" {
		descText = "No description."
	}
	b.WriteString(renderFieldMultiline("Description", descText, w, availH, colorText))
	b.WriteString("\n")

	return b.String()
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
