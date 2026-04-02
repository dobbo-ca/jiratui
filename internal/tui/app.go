package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
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
	stateDetail
	stateDetailLoading
)

// listPaneWidth returns the width of the list pane in split mode (~35% of terminal).
func (a App) listPaneWidth() int {
	w := a.width * 35 / 100
	if w < 40 {
		w = 40
	}
	if w > 60 {
		w = 60
	}
	return w
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

// App is the root Bubble Tea model.
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
		if a.state == stateDetail {
			// Update list for split view
			a.list.width = msg.Width
			a.list.height = msg.Height
			a.list.clampCursor()

			detailWidth := msg.Width - a.listPaneWidth() - 1
			adjusted := tea.WindowSizeMsg{Width: detailWidth, Height: msg.Height - 2}
			var cmd tea.Cmd
			a.detail, cmd = a.detail.Update(adjusted)
			return a, cmd
		}
		if a.state == stateDetailLoading {
			a.list.width = msg.Width
			a.list.height = msg.Height
			a.list.clampCursor()
			return a, nil
		}
		return a, nil

	case tea.KeyMsg:
		// Global quit — always works
		if msg.String() == "ctrl+c" {
			return a, tea.Quit
		}
		if a.state == stateDetail {
			if key.Matches(msg, detailKeys.Escape) {
				a.state = stateList
				return a, nil
			}
			var cmd tea.Cmd
			a.detail, cmd = a.detail.Update(msg)
			return a, cmd
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

	case openIssueMsg:
		a.state = stateDetailLoading
		return a, tea.Batch(a.spinner.Tick, fetchIssueDetail(a.client, msg.issueKey))

	case issueDetailMsg:
		contentHeight := a.height - 2
		detailWidth := a.width - a.listPaneWidth() - 1
		a.detail = NewDetail(msg.issue, detailWidth, contentHeight)
		a.state = stateDetail
		return a, nil

	case spinner.TickMsg:
		if a.state == stateLoading || a.state == stateDetailLoading {
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

	if a.state == stateDetail {
		// In split mode, route mouse events by X position
		if mouseMsg, ok := msg.(tea.MouseMsg); ok {
			listW := a.listPaneWidth()
			if mouseMsg.X < listW {
				// Mouse is in the list pane — adjust Y for status bar
				adjusted := mouseMsg
				adjusted.Y = mouseMsg.Y - 1 // account for status bar
				var cmd tea.Cmd
				a.list, cmd = a.list.Update(adjusted)
				return a, cmd
			}
			// Mouse is in the detail pane — adjust X for pane offset
			adjusted := mouseMsg
			adjusted.X = mouseMsg.X - listW - 1 // subtract list + border
			adjusted.Y = mouseMsg.Y - 1         // account for status bar
			var cmd tea.Cmd
			a.detail, cmd = a.detail.Update(adjusted)
			return a, cmd
		}
		var cmd tea.Cmd
		a.detail, cmd = a.detail.Update(msg)
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
	case stateDetail, stateDetailLoading:
		// Split: list on left, detail on right
		listW := a.listPaneWidth()
		detailW := a.width - listW - 1 // 1 for border
		contentH := a.height - 2       // status + help bars

		leftList := a.list.ViewWithWidth(listW, contentH)

		borderLines := make([]string, contentH)
		borderStyle := lipgloss.NewStyle().Foreground(colorBorder)
		for i := range borderLines {
			borderLines[i] = borderStyle.Render("│")
		}
		border := strings.Join(borderLines, "\n")

		var right string
		if a.state == stateDetailLoading {
			loadStyle := lipgloss.NewStyle().
				Width(detailW).
				Height(contentH).
				Foreground(colorText).
				Align(lipgloss.Center, lipgloss.Center)
			right = loadStyle.Render(a.spinner.View() + " Loading issue...")
		} else {
			right = a.detail.View()
		}

		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, leftList, border, right))
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
	switch {
	case a.state == stateList && a.list.filtering:
		help = "enter confirm · esc clear filter"
	case a.state == stateDetail:
		help = "esc back · 1-5 tabs · j/k scroll · o open in browser"
	default:
		help = "↑/k up · ↓/j down · enter open · / filter · o browser · r refresh · q quit"
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
