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

// Update handles messages for the detail view.
func (d Detail) Update(msg tea.Msg) (Detail, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, detailKeys.Tab1):
			d.activeTab = tabDetails
			d.scrollY = 0
		case key.Matches(msg, detailKeys.Tab2):
			d.activeTab = tabDescription
			d.scrollY = 0
		case key.Matches(msg, detailKeys.Tab3):
			d.activeTab = tabComments
			d.scrollY = 0
		case key.Matches(msg, detailKeys.Tab4):
			d.activeTab = tabSubtasks
			d.scrollY = 0
		case key.Matches(msg, detailKeys.Tab5):
			d.activeTab = tabLinks
			d.scrollY = 0
		case key.Matches(msg, detailKeys.Tab6):
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
			// Tab bar click detection — y=0 is the tab bar
			if msg.Y <= 1 {
				// Calculate approximate tab positions based on rendered label widths
				// Labels: " 1:Details " (11), " 2:Description " (16), " 3:Comments(N) " (~16), " 4:Subtasks(N) " (~16), " 5:Links(N) " (~14)
				tabEnds := []int{11, 27, 43, 59, 73}
				for i, end := range tabEnds {
					start := 0
					if i > 0 {
						start = tabEnds[i-1]
					}
					if msg.X >= start && msg.X < end {
						d.activeTab = detailTab(i)
						d.scrollY = 0
						return d, nil
					}
				}
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

// View renders the detail view.
func (d Detail) View() string {
	var b strings.Builder

	b.WriteString(d.renderTabBar())
	b.WriteString("\n")

	// Tab content
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

	labels := []string{
		"Details",
		"Desc",
		fmt.Sprintf("Comments(%d)", len(d.issue.Comments)),
		fmt.Sprintf("Subtasks(%d)", len(d.issue.Subtasks)),
		fmt.Sprintf("Links(%d)", len(d.issue.Links)),
		fmt.Sprintf("Attach(%d)", len(d.issue.Attachments)),
	}

	var tabLine strings.Builder
	var divLine strings.Builder

	pad := " "
	sep := " "
	tabLine.WriteString(pad)
	divLine.WriteString(dividerStyle.Render("─"))

	for i, label := range labels {
		if i > 0 {
			tabLine.WriteString(sep)
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
	tabWidth := 1 // pad
	for i, label := range labels {
		if i > 0 {
			tabWidth++ // sep
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
	// Available height: total height minus tab labels(1) and divider(1)
	viewHeight := d.height - 2
	if viewHeight < 1 {
		viewHeight = 1
	}

	// Clamp scrollY (read-only — don't mutate d.scrollY on value receiver)
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

func (d Detail) renderDetailsTab() string {
	var b strings.Builder
	labelStyle := lipgloss.NewStyle().Foreground(colorSubtle).Width(12)
	valueStyle := lipgloss.NewStyle().Foreground(colorText)
	accentStyle := lipgloss.NewStyle().Foreground(colorAccent)
	boldStyle := lipgloss.NewStyle().Foreground(colorText).Bold(true)

	// Title
	b.WriteString("  " + boldStyle.Render(d.issue.Key+" "+d.issue.Summary) + "\n\n")

	// Status
	b.WriteString("  " + labelStyle.Render("Status") + StyledStatus(d.issue.Status.Name) + "\n")

	// Priority
	pColor := priorityColor(d.issue.Priority.Name)
	pDot := lipgloss.NewStyle().Foreground(pColor).Render("●")
	b.WriteString("  " + labelStyle.Render("Priority") + pDot + " " + valueStyle.Render(d.issue.Priority.Name) + "\n")

	// Type
	b.WriteString("  " + labelStyle.Render("Type") + valueStyle.Render(d.issue.Type.Name) + "\n")

	// Assignee
	if d.issue.Assignee != nil {
		assigneeStyle := lipgloss.NewStyle().Foreground(colorSuccess)
		b.WriteString("  " + labelStyle.Render("Assignee") + assigneeStyle.Render(d.issue.Assignee.DisplayName) + "\n")
	} else {
		unassigned := lipgloss.NewStyle().Foreground(colorSubtle).Render("Unassigned")
		b.WriteString("  " + labelStyle.Render("Assignee") + unassigned + "\n")
	}

	// Reporter
	if d.issue.Reporter != nil {
		b.WriteString("  " + labelStyle.Render("Reporter") + valueStyle.Render(d.issue.Reporter.DisplayName) + "\n")
	}

	// Sprint
	if d.issue.Sprint != "" {
		b.WriteString("  " + labelStyle.Render("Sprint") + valueStyle.Render(d.issue.Sprint) + "\n")
	}

	// Parent
	if d.issue.Parent != nil {
		parentText := accentStyle.Render(d.issue.Parent.Key) + " " + valueStyle.Render(d.issue.Parent.Summary)
		b.WriteString("  " + labelStyle.Render("Parent") + parentText + "\n")
	}

	// Labels
	if len(d.issue.Labels) > 0 {
		labelTags := make([]string, len(d.issue.Labels))
		tagStyle := lipgloss.NewStyle().Foreground(colorInfo)
		for i, l := range d.issue.Labels {
			labelTags[i] = tagStyle.Render("[" + l + "]")
		}
		b.WriteString("  " + labelStyle.Render("Labels") + strings.Join(labelTags, " ") + "\n")
	}

	// Created
	b.WriteString("  " + labelStyle.Render("Created") + valueStyle.Render(d.issue.Created.Format("2006-01-02 15:04")) + "\n")

	// Updated
	updatedText := relativeTime(d.issue.Updated) + " (" + d.issue.Updated.Format("2006-01-02 15:04") + ")"
	b.WriteString("  " + labelStyle.Render("Updated") + valueStyle.Render(updatedText) + "\n")

	// Due Date
	if d.issue.DueDate != nil {
		dueStr := d.issue.DueDate.Format("2006-01-02")
		now := time.Now()
		var dueStyle lipgloss.Style
		if d.issue.DueDate.Before(now) {
			dueStyle = lipgloss.NewStyle().Foreground(colorError)
		} else if d.issue.DueDate.Before(now.Add(7 * 24 * time.Hour)) {
			dueStyle = lipgloss.NewStyle().Foreground(colorWarning)
		} else {
			dueStyle = lipgloss.NewStyle().Foreground(colorText)
		}
		b.WriteString("  " + labelStyle.Render("Due Date") + dueStyle.Render(dueStr) + "\n")
	}

	return b.String()
}

func (d Detail) renderDescriptionTab() string {
	if d.issue.Description == "" {
		subtle := lipgloss.NewStyle().Foreground(colorSubtle)
		return "  " + subtle.Render("No description.")
	}
	contentWidth := d.width - 4
	if contentWidth < 20 {
		contentWidth = 20
	}
	return "  " + wordWrap(d.issue.Description, contentWidth)
}

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
