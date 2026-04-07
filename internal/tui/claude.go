// internal/tui/claude.go
package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// claudeTickMsg is sent periodically to refresh tmux capture output.
type claudeTickMsg struct{}

// claudeSessionEndedMsg is sent when the tmux session no longer exists.
type claudeSessionEndedMsg struct{ issueKey string }

// ClaudeTab manages an embedded Claude Code tmux session.
type ClaudeTab struct {
	issueKey    string
	sessionName string
	active      bool
	content     string
	width       int
	height      int
	err         string

	// Long-press esc detection
	escPressedAt time.Time
	escHeld      bool
}

func NewClaudeTab(issueKey string, width, height int) ClaudeTab {
	return ClaudeTab{
		issueKey:    issueKey,
		sessionName: claudeSessionName(issueKey),
		width:       width,
		height:      height,
	}
}

// Launch creates the tmux session and starts Claude Code with the context file.
func (ct *ClaudeTab) Launch(contextPath string) tea.Cmd {
	name := ct.sessionName
	w, h := ct.width, ct.height

	return func() tea.Msg {
		if !tmuxInstalled() {
			return claudeErrMsg{err: "tmux is not installed. Install with: brew install tmux"}
		}

		if !tmuxSessionExists(name) {
			if err := tmuxCreateSession(name, w, h); err != nil {
				return claudeErrMsg{err: "failed to create tmux session: " + err.Error()}
			}
			cmd := "claude --prompt-file " + contextPath
			if err := tmuxSendKeys(name, cmd); err != nil {
				return claudeErrMsg{err: "failed to launch claude: " + err.Error()}
			}
		}

		return claudeTickMsg{}
	}
}

type claudeErrMsg struct{ err string }

// Update handles messages for the Claude tab.
func (ct *ClaudeTab) Update(msg tea.Msg) tea.Cmd {
	switch msg.(type) {
	case claudeTickMsg:
		if !tmuxSessionExists(ct.sessionName) {
			return func() tea.Msg {
				return claudeSessionEndedMsg{issueKey: ct.issueKey}
			}
		}
		output, err := tmuxCapture(ct.sessionName)
		if err != nil {
			ct.content = "Error capturing tmux output: " + err.Error()
		} else {
			ct.content = output
		}
		return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
			return claudeTickMsg{}
		})
	case claudeErrMsg:
		ct.err = msg.(claudeErrMsg).err
		return nil
	}
	return nil
}

func (ct *ClaudeTab) SendKey(key string) {
	if ct.sessionName != "" && tmuxSessionExists(ct.sessionName) {
		tmuxSendRawKey(ct.sessionName, key)
	}
}

func (ct *ClaudeTab) Resize(width, height int) {
	ct.width = width
	ct.height = height
	if tmuxSessionExists(ct.sessionName) {
		tmuxResizeWindow(ct.sessionName, width, height)
	}
}

func (ct *ClaudeTab) Kill() {
	if tmuxSessionExists(ct.sessionName) {
		tmuxKillSession(ct.sessionName)
	}
}

func (ct *ClaudeTab) HasSession() bool {
	return tmuxSessionExists(ct.sessionName)
}

// View renders the Claude tab content.
func (ct ClaudeTab) View() string {
	if ct.err != "" {
		errStyle := lipgloss.NewStyle().Foreground(colorError)
		return errStyle.Render("Error: " + ct.err)
	}

	if ct.content == "" {
		subtleStyle := lipgloss.NewStyle().Foreground(colorSubtle)
		return subtleStyle.Render("Starting Claude Code session...")
	}

	lines := strings.Split(ct.content, "\n")
	if len(lines) > ct.height {
		lines = lines[len(lines)-ct.height:]
	}
	var result []string
	for _, line := range lines {
		if lipgloss.Width(line) > ct.width {
			line = truncateAnsi(line, ct.width)
		}
		result = append(result, line)
	}
	return strings.Join(result, "\n")
}

func (ct *ClaudeTab) CheckEscHeld() bool {
	if ct.escHeld && time.Since(ct.escPressedAt) >= 1500*time.Millisecond {
		ct.escHeld = false
		return true
	}
	return false
}

func (ct *ClaudeTab) StartEscHold() {
	ct.escPressedAt = time.Now()
	ct.escHeld = true
}

func (ct *ClaudeTab) CancelEscHold() {
	ct.escHeld = false
}
