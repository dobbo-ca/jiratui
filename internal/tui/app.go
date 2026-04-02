package tui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/christopherdobbyn/jiratui/internal/jira"
	"github.com/christopherdobbyn/jiratui/internal/models"
	"github.com/muesli/ansi"
)

// truncateAnsi truncates a string with ANSI escape codes to a given visual width.
func truncateAnsi(s string, maxWidth int) string {
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	var (
		result strings.Builder
		vis    int
		i      int
	)
	for i < len(s) {
		// Skip ANSI escape sequences
		if s[i] == '\x1b' {
			j := i
			for j < len(s) && s[j] != 'm' {
				j++
			}
			if j < len(s) {
				j++ // include the 'm'
			}
			result.WriteString(s[i:j])
			i = j
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		w := ansi.PrintableRuneWidth(string(r))
		if vis+w > maxWidth {
			break
		}
		result.WriteRune(r)
		vis += w
		i += size
	}
	// Append reset to close any open ANSI sequences
	result.WriteString("\x1b[0m")
	return result.String()
}

type state int

const (
	stateLoading state = iota
	stateList
)

// defaultListWidth calculates the default list pane width (~35% of terminal).
func defaultListWidth(totalWidth int) int {
	w := totalWidth * 35 / 100
	if w < 30 {
		w = 30
	}
	if w > 80 {
		w = 80
	}
	return w
}

func (a App) detailPaneWidth() int {
	return a.width - a.listWidth - 1
}

// issuesMsg carries fetched issues into the model.
type issuesMsg struct {
	issues []models.Issue
}

// errMsg carries errors into the model.
type errMsg struct {
	err error
}

// issueDetailMsg carries a fetched issue detail into the model.
type issueDetailMsg struct {
	issue  models.Issue
	forKey string // which issue key this detail is for
}

func fetchIssueDetail(client *jira.Client, key string) tea.Cmd {
	return func() tea.Msg {
		issue, err := client.GetIssue(key)
		if err != nil {
			return errMsg{err: err}
		}
		return issueDetailMsg{issue: *issue, forKey: key}
	}
}

// App is the root Bubble Tea model.
type App struct {
	state         state
	list          List
	detail        *Detail
	detailLoading bool
	detailKey     string // issue key currently shown/loading in detail
	listWidth     int    // width of the list pane (draggable)
	dragging      bool   // true while dragging the border
	spinner       spinner.Model
	client        *jira.Client
	profileName   string
	err           error
	width         int
	height        int
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
		// Initialize or clamp list width
		if a.listWidth == 0 {
			a.listWidth = defaultListWidth(msg.Width)
		}
		if a.listWidth > msg.Width-30 {
			a.listWidth = msg.Width - 30
		}
		if a.listWidth < 20 {
			a.listWidth = 20
		}
		if a.state == stateList {
			a.list.width = msg.Width
			a.list.height = msg.Height
			a.list.clampCursor()
			if a.detail != nil {
				detailWidth := a.detailPaneWidth()
				adjusted := tea.WindowSizeMsg{Width: detailWidth, Height: msg.Height - 2}
				d := *a.detail
				d, _ = d.Update(adjusted)
				a.detail = &d
			}
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
				a.detail = nil
				a.detailKey = ""
				return a, tea.Batch(a.spinner.Tick, fetchIssues(a.client))
			}
			// Tab switching — forward 1-5 to detail if it exists
			if a.detail != nil && !a.list.filtering {
				if msg.String() >= "1" && msg.String() <= "5" {
					d := *a.detail
					d, _ = d.Update(msg)
					a.detail = &d
					return a, nil
				}
			}
		}

	case issuesMsg:
		a.list = NewList(msg.issues, a.width, a.height)
		a.state = stateList
		// Auto-fetch first issue detail
		if len(msg.issues) > 0 {
			a.detailLoading = true
			a.detailKey = msg.issues[0].Key
			return a, tea.Batch(a.spinner.Tick, fetchIssueDetail(a.client, msg.issues[0].Key))
		}
		return a, nil

	case cursorChangedMsg:
		// Only fetch if it's a different issue
		if msg.issueKey != a.detailKey {
			a.detailLoading = true
			a.detailKey = msg.issueKey
			return a, tea.Batch(a.spinner.Tick, fetchIssueDetail(a.client, msg.issueKey))
		}
		return a, nil

	case issueDetailMsg:
		// Only accept if this is still the issue we're waiting for
		if msg.forKey == a.detailKey {
			contentHeight := a.height - 2
			detailWidth := a.detailPaneWidth()
			d := NewDetail(msg.issue, detailWidth, contentHeight)
			a.detail = &d
			a.detailLoading = false
		}
		return a, nil

	case errMsg:
		a.err = msg.err
		a.state = stateList
		a.detailLoading = false
		return a, nil

	case spinner.TickMsg:
		if a.state == stateLoading || a.detailLoading {
			var cmd tea.Cmd
			a.spinner, cmd = a.spinner.Update(msg)
			return a, cmd
		}
		return a, nil
	}

	// Forward remaining messages to list
	if a.state == stateList {
		// Route mouse events by pane in split mode
		if mouseMsg, ok := msg.(tea.MouseMsg); ok {
			listW := a.listWidth

			// Handle border drag
			if mouseMsg.Action == tea.MouseActionPress && mouseMsg.Button == tea.MouseButtonLeft {
				if mouseMsg.X >= listW-1 && mouseMsg.X <= listW+1 {
					a.dragging = true
					return a, nil
				}
			}
			if mouseMsg.Action == tea.MouseActionRelease {
				a.dragging = false
				return a, nil
			}
			if a.dragging && mouseMsg.Action == tea.MouseActionMotion {
				newWidth := mouseMsg.X
				if newWidth < 20 {
					newWidth = 20
				}
				if newWidth > a.width-30 {
					newWidth = a.width - 30
				}
				a.listWidth = newWidth
				// Update detail dimensions
				if a.detail != nil {
					detailWidth := a.detailPaneWidth()
					d := *a.detail
					d.width = detailWidth
					a.detail = &d
				}
				return a, nil
			}

			if mouseMsg.X < listW {
				adjusted := mouseMsg
				adjusted.Y = mouseMsg.Y - 1
				var cmd tea.Cmd
				a.list, cmd = a.list.Update(adjusted)
				return a, cmd
			}
			// Mouse in detail pane
			if a.detail != nil {
				adjusted := mouseMsg
				adjusted.X = mouseMsg.X - listW - 1
				adjusted.Y = mouseMsg.Y - 1
				d := *a.detail
				d, _ = d.Update(adjusted)
				a.detail = &d
			}
			return a, nil
		}
		var cmd tea.Cmd
		a.list, cmd = a.list.Update(msg)
		return a, cmd
	}

	return a, nil
}

// View renders the full app.
func (a App) View() string {
	// Guard: don't render until we know terminal dimensions
	if a.width == 0 || a.height == 0 {
		return ""
	}

	contentH := a.height - 1 // just help bar at bottom (no title bar)
	if contentH < 1 {
		contentH = 1
	}

	// Main content — always split layout
	var content string
	if a.err != nil {
		errStyle := lipgloss.NewStyle().
			Foreground(colorError).
			PaddingLeft(2).
			PaddingTop(1)
		content = errStyle.Render("Error: " + a.err.Error())
	} else {
		listW := a.listWidth
		detailW := a.detailPaneWidth()

		// Left pane: list (or loading placeholder)
		var left string
		if a.state == stateLoading {
			loadStyle := lipgloss.NewStyle().
				Width(listW).
				Height(contentH).
				Foreground(colorText).
				Align(lipgloss.Center, lipgloss.Center)
			left = loadStyle.Render(a.spinner.View() + " Loading...")
		} else {
			left = a.list.ViewWithWidth(listW, contentH)
		}

		// Border
		borderLines := make([]string, contentH)
		borderStyle := lipgloss.NewStyle().Foreground(colorBorder)
		for i := range borderLines {
			borderLines[i] = borderStyle.Render("│")
		}
		border := strings.Join(borderLines, "\n")

		// Right pane: detail
		var right string
		if a.state == stateLoading || a.detailLoading {
			loadStyle := lipgloss.NewStyle().
				Width(detailW).
				Height(contentH).
				Foreground(colorText).
				Align(lipgloss.Center, lipgloss.Center)
			msg := " Loading..."
			if a.detailLoading {
				msg = a.spinner.View() + " Loading issue..."
			}
			right = loadStyle.Render(msg)
		} else if a.detail != nil {
			right = a.detail.View()
		} else {
			emptyStyle := lipgloss.NewStyle().
				Width(detailW).
				Height(contentH).
				Foreground(colorSubtle).
				Align(lipgloss.Center, lipgloss.Center)
			right = emptyStyle.Render("No issues found")
		}

		content = lipgloss.JoinHorizontal(lipgloss.Top, left, border, right)
	}

	// Build all lines, hard-cap to exact dimensions
	contentLines := strings.Split(content, "\n")
	if len(contentLines) > contentH {
		contentLines = contentLines[:contentH]
	}
	for len(contentLines) < contentH {
		contentLines = append(contentLines, "")
	}

	allLines := make([]string, 0, a.height)
	allLines = append(allLines, contentLines...)
	allLines = append(allLines, a.renderHelpBar())

	// Truncate every line to terminal width and cap total lines
	if len(allLines) > a.height {
		allLines = allLines[:a.height]
	}
	for i, line := range allLines {
		if lipgloss.Width(line) > a.width {
			allLines[i] = truncateAnsi(line, a.width)
		}
	}

	return strings.Join(allLines, "\n")
}


func (a App) renderHelpBar() string {
	bgStyle := lipgloss.NewStyle().Background(colorHeaderBg)
	helpStyle := bgStyle.Foreground(colorSubtle).PaddingLeft(1)
	profileStyle := bgStyle.Foreground(colorSuccess).PaddingRight(1)

	var help string
	if a.state == stateList && a.list.filtering {
		help = "enter confirm · esc clear"
	} else {
		help = "/ filter · o browser · r refresh · q quit · ? help"
	}

	left := helpStyle.Render(help)
	right := profileStyle.Render("● " + a.profileName)

	gap := a.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}
	spacer := bgStyle.Render(strings.Repeat(" ", gap))

	return left + spacer + right
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
