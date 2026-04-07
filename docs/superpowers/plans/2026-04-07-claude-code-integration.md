# Claude Code Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Integrate Claude Code as a new tab in the detail pane, backed by tmux sessions, with CLI subcommands for Claude to interact with Jira tickets.

**Architecture:** When the user presses F1 on a ticket, jt gathers the ticket context (summary, description, comments, attachment listing) into a markdown file, creates a tmux session running Claude Code with that context, and renders the tmux output inside a new "Claude" tab (tab 5) in the detail pane. The detail pane expands when the Claude tab is active. F2 pops out to full-screen tmux attach. Three new `jt` subcommands (`attach`, `update-description`, `download-attachment`) let Claude Code interact with the ticket.

**Tech Stack:** Go 1.26, Bubble Tea, tmux, Claude Code CLI, cobra

---

## File Structure

| File | Responsibility |
|------|---------------|
| `internal/tui/claude.go` | Claude tab component: tmux capture polling, keystroke forwarding, render, long-press esc detection |
| `internal/tui/tmux.go` | tmux operations: create session, capture pane, send keys, resize, kill, check exists |
| `internal/tui/context.go` | Build markdown context file from issue model |
| `cmd/attach.go` | `jt attach <project> <key> <file>` subcommand |
| `cmd/update_description.go` | `jt update-description <project> <key> <file>` subcommand |
| `cmd/download_attachment.go` | `jt download-attachment <project> <key> <filename> [dest]` subcommand |
| `internal/tui/detail.go` | Add tabClaude constant, route to claude component, tab rendering |
| `internal/tui/app.go` | F1/F2/Shift+F1/Ctrl+F1 keybindings, layout resize for Claude tab, tea.Exec for F2 |
| `internal/tui/keys.go` | New key definitions |

---

### Task 1: tmux wrapper (`internal/tui/tmux.go`)

**Files:**
- Create: `internal/tui/tmux.go`
- Create: `internal/tui/tmux_test.go`

- [ ] **Step 1: Write tests for tmux helpers**

```go
// internal/tui/tmux_test.go
package tui

import (
	"os/exec"
	"testing"
)

func tmuxAvailable() bool {
	_, err := exec.LookPath("tmux")
	return err == nil
}

func TestSessionName(t *testing.T) {
	if got := claudeSessionName("TEC-123"); got != "jt-claude-TEC-123" {
		t.Errorf("got %q, want jt-claude-TEC-123", got)
	}
}

func TestTmuxSessionExists(t *testing.T) {
	if !tmuxAvailable() {
		t.Skip("tmux not available")
	}
	// Non-existent session should return false
	if tmuxSessionExists("jt-test-nonexistent-99999") {
		t.Error("expected false for nonexistent session")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/ -run TestSessionName -v`
Expected: FAIL — `claudeSessionName` undefined

- [ ] **Step 3: Implement tmux.go**

```go
// internal/tui/tmux.go
package tui

import (
	"os/exec"
	"strconv"
	"strings"
)

// claudeSessionName returns the tmux session name for a ticket.
func claudeSessionName(issueKey string) string {
	return "jt-claude-" + issueKey
}

// tmuxSessionExists checks if a tmux session with the given name exists.
func tmuxSessionExists(name string) bool {
	cmd := exec.Command("tmux", "has-session", "-t", name)
	return cmd.Run() == nil
}

// tmuxCreateSession creates a new detached tmux session with the given dimensions.
func tmuxCreateSession(name string, width, height int) error {
	cmd := exec.Command("tmux", "new-session", "-d", "-s", name,
		"-x", strconv.Itoa(width), "-y", strconv.Itoa(height))
	return cmd.Run()
}

// tmuxSendKeys sends a string of keys to a tmux session.
func tmuxSendKeys(name, keys string) error {
	cmd := exec.Command("tmux", "send-keys", "-t", name, keys, "Enter")
	return cmd.Run()
}

// tmuxSendRawKey sends a single raw key (no Enter appended) to a tmux session.
func tmuxSendRawKey(name, key string) error {
	cmd := exec.Command("tmux", "send-keys", "-t", name, key)
	return cmd.Run()
}

// tmuxCapture captures the current pane content with ANSI escapes.
func tmuxCapture(name string) (string, error) {
	cmd := exec.Command("tmux", "capture-pane", "-t", name, "-p", "-e")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// tmuxResizeWindow resizes a tmux session's window.
func tmuxResizeWindow(name string, width, height int) error {
	cmd := exec.Command("tmux", "resize-window", "-t", name,
		"-x", strconv.Itoa(width), "-y", strconv.Itoa(height))
	return cmd.Run()
}

// tmuxKillSession kills a tmux session.
func tmuxKillSession(name string) error {
	cmd := exec.Command("tmux", "kill-session", "-t", name)
	return cmd.Run()
}

// tmuxInstalled checks if tmux is available on PATH.
func tmuxInstalled() bool {
	_, err := exec.LookPath("tmux")
	return err == nil
}

// tmuxListClaudeSessions returns all active jt-claude-* session names.
func tmuxListClaudeSessions() []string {
	cmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name}")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var sessions []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.HasPrefix(line, "jt-claude-") {
			sessions = append(sessions, line)
		}
	}
	return sessions
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tui/ -run "TestSessionName|TestTmuxSession" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tui/tmux.go internal/tui/tmux_test.go
git commit -m "feat: add tmux session management helpers"
```

---

### Task 2: Context builder (`internal/tui/context.go`)

**Files:**
- Create: `internal/tui/context.go`
- Create: `internal/tui/context_test.go`

- [ ] **Step 1: Write test for context builder**

```go
// internal/tui/context_test.go
package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/christopherdobbyn/jiratui/internal/models"
)

func TestBuildContextMarkdown(t *testing.T) {
	issue := models.Issue{
		Key:         "TEC-123",
		Summary:     "Fix login bug",
		Description: "Users cannot log in after password reset.",
		Status:      models.Status{Name: "In Progress"},
		Priority:    models.Priority{Name: "High"},
		Type:        models.IssueType{Name: "Bug"},
		Assignee:    &models.User{DisplayName: "Chris"},
		ProjectKey:  "TEC",
		Comments: []models.Comment{
			{
				Author:  models.User{DisplayName: "Jane"},
				Body:    "I can reproduce this on staging.",
				Created: time.Date(2026, 4, 5, 10, 30, 0, 0, time.UTC),
			},
		},
		Attachments: []models.Attachment{
			{Filename: "screenshot.png", Size: 245000},
			{Filename: "logs.txt", Size: 1200},
		},
	}

	md := buildContextMarkdown(issue)

	checks := []string{
		"# Jira Ticket: TEC-123",
		"**Project:** TEC",
		"**Status:** In Progress",
		"**Priority:** High",
		"Fix login bug",
		"Users cannot log in after password reset.",
		"**Jane**",
		"I can reproduce this on staging.",
		"screenshot.png",
		"logs.txt",
		"jt download-attachment TEC TEC-123",
		"jt attach TEC TEC-123",
		"jt update-description TEC TEC-123",
	}
	for _, check := range checks {
		if !strings.Contains(md, check) {
			t.Errorf("context missing %q", check)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/ -run TestBuildContextMarkdown -v`
Expected: FAIL — `buildContextMarkdown` undefined

- [ ] **Step 3: Implement context.go**

```go
// internal/tui/context.go
package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/christopherdobbyn/jiratui/internal/models"
)

// buildContextMarkdown creates a markdown string with the ticket context
// and jt CLI instructions for Claude Code.
func buildContextMarkdown(issue models.Issue) string {
	var b strings.Builder

	// Header
	b.WriteString(fmt.Sprintf("# Jira Ticket: %s\n", issue.Key))
	assignee := "Unassigned"
	if issue.Assignee != nil {
		assignee = issue.Assignee.DisplayName
	}
	b.WriteString(fmt.Sprintf("**Project:** %s | **Status:** %s | **Priority:** %s\n",
		issue.ProjectKey, issue.Status.Name, issue.Priority.Name))
	b.WriteString(fmt.Sprintf("**Type:** %s | **Assignee:** %s\n\n",
		issue.Type.Name, assignee))

	// Summary
	b.WriteString("## Summary\n")
	b.WriteString(issue.Summary + "\n\n")

	// Description
	if issue.Description != "" {
		b.WriteString("## Description\n")
		b.WriteString(issue.Description + "\n\n")
	}

	// Comments
	if len(issue.Comments) > 0 {
		b.WriteString("## Comments\n")
		for _, c := range issue.Comments {
			ts := c.Created.Format("2006-01-02 15:04")
			b.WriteString(fmt.Sprintf("**%s** (%s):\n", c.Author.DisplayName, ts))
			b.WriteString(fmt.Sprintf("> %s\n\n", c.Body))
		}
	}

	// Attachments
	if len(issue.Attachments) > 0 {
		b.WriteString("## Attachments\n")
		for _, a := range issue.Attachments {
			b.WriteString(fmt.Sprintf("- %s (%s)\n", a.Filename, formatBytes(a.Size)))
		}
		b.WriteString("\n")
	}

	// CLI instructions
	b.WriteString("## Available Commands\n")
	b.WriteString("You have access to the `jt` CLI for interacting with this ticket:\n\n")
	b.WriteString(fmt.Sprintf("- Download an attachment:\n  `jt download-attachment %s %s \"filename\" /tmp/`\n\n",
		issue.ProjectKey, issue.Key))
	b.WriteString(fmt.Sprintf("- Attach a file to this ticket:\n  `jt attach %s %s path/to/file`\n\n",
		issue.ProjectKey, issue.Key))
	b.WriteString(fmt.Sprintf("- Update the ticket description:\n  `jt update-description %s %s path/to/file.md`\n",
		issue.ProjectKey, issue.Key))

	return b.String()
}

// writeContextFile writes the context markdown to a temp file and returns the path.
func writeContextFile(issue models.Issue) (string, error) {
	dir := filepath.Join(os.TempDir(), "jt-claude", issue.Key)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("creating context dir: %w", err)
	}
	path := filepath.Join(dir, "context.md")
	md := buildContextMarkdown(issue)
	if err := os.WriteFile(path, []byte(md), 0o600); err != nil {
		return "", fmt.Errorf("writing context file: %w", err)
	}
	return path, nil
}

func formatBytes(size int) string {
	switch {
	case size >= 1_000_000:
		return fmt.Sprintf("%.1f MB", float64(size)/1_000_000)
	case size >= 1_000:
		return fmt.Sprintf("%.0f KB", float64(size)/1_000)
	default:
		return fmt.Sprintf("%d B", size)
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tui/ -run TestBuildContextMarkdown -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tui/context.go internal/tui/context_test.go
git commit -m "feat: add ticket context builder for Claude Code sessions"
```

---

### Task 3: CLI subcommands (`cmd/attach.go`, `cmd/update_description.go`, `cmd/download_attachment.go`)

**Files:**
- Create: `cmd/attach.go`
- Create: `cmd/update_description.go`
- Create: `cmd/download_attachment.go`

- [ ] **Step 1: Implement `jt attach` subcommand**

```go
// cmd/attach.go
package cmd

import (
	"fmt"

	"github.com/christopherdobbyn/jiratui/internal/config"
	"github.com/christopherdobbyn/jiratui/internal/jira"
	"github.com/spf13/cobra"
)

var attachCmd = &cobra.Command{
	Use:   "attach <project-key> <issue-key> <file-path>",
	Short: "Upload a file as an attachment to a Jira ticket",
	Args:  cobra.ExactArgs(3),
	RunE:  runAttach,
}

func init() {
	rootCmd.AddCommand(attachCmd)
}

func runAttach(cmd *cobra.Command, args []string) error {
	issueKey := args[1]
	filePath := args[2]

	client, err := newClientFromConfig()
	if err != nil {
		return err
	}

	fmt.Printf("Uploading %s to %s...\n", filePath, issueKey)
	if err := client.UploadAttachment(issueKey, filePath); err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}
	fmt.Println("Done.")
	return nil
}

// newClientFromConfig loads config and creates a Jira client.
func newClientFromConfig() (*jira.Client, error) {
	cfgPath := config.DefaultPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	profile, err := cfg.ActiveProfileConfig()
	if err != nil {
		return nil, err
	}
	return jira.NewClient(profile.URL, profile.Email, profile.APIToken), nil
}
```

- [ ] **Step 2: Implement `jt update-description` subcommand**

```go
// cmd/update_description.go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var updateDescCmd = &cobra.Command{
	Use:   "update-description <project-key> <issue-key> <file-path>",
	Short: "Update a ticket's description from a markdown file",
	Args:  cobra.ExactArgs(3),
	RunE:  runUpdateDescription,
}

func init() {
	rootCmd.AddCommand(updateDescCmd)
}

func runUpdateDescription(cmd *cobra.Command, args []string) error {
	issueKey := args[1]
	filePath := args[2]

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	client, err := newClientFromConfig()
	if err != nil {
		return err
	}

	fmt.Printf("Updating description of %s...\n", issueKey)
	if err := client.UpdateDescription(issueKey, string(data)); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}
	fmt.Println("Done.")
	return nil
}
```

- [ ] **Step 3: Implement `jt download-attachment` subcommand**

```go
// cmd/download_attachment.go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/christopherdobbyn/jiratui/internal/config"
	"github.com/christopherdobbyn/jiratui/internal/jira"
	"github.com/spf13/cobra"
)

var downloadAttachCmd = &cobra.Command{
	Use:   "download-attachment <project-key> <issue-key> <filename> [dest-dir]",
	Short: "Download a ticket attachment to a local path",
	Args:  cobra.RangeArgs(3, 4),
	RunE:  runDownloadAttachment,
}

func init() {
	rootCmd.AddCommand(downloadAttachCmd)
}

func runDownloadAttachment(cmd *cobra.Command, args []string) error {
	issueKey := args[1]
	filename := args[2]
	destDir := "."
	if len(args) > 3 {
		destDir = args[3]
	}

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

	// Fetch issue to find attachment URL
	issue, err := client.GetIssue(issueKey)
	if err != nil {
		return fmt.Errorf("fetching issue: %w", err)
	}

	var attachURL string
	for _, a := range issue.Attachments {
		if a.Filename == filename {
			attachURL = a.URL
			break
		}
	}
	if attachURL == "" {
		return fmt.Errorf("attachment %q not found on %s", filename, issueKey)
	}

	fmt.Printf("Downloading %s from %s...\n", filename, issueKey)
	data, err := client.DownloadURL(attachURL)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	destPath := filepath.Join(destDir, filename)
	if err := os.WriteFile(destPath, data, 0o644); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}
	fmt.Printf("Saved to %s\n", destPath)
	return nil
}
```

- [ ] **Step 4: Build and verify subcommands register**

Run: `go build -o jt . && ./jt attach --help && ./jt update-description --help && ./jt download-attachment --help`
Expected: Each prints usage help without errors

- [ ] **Step 5: Commit**

```bash
git add cmd/attach.go cmd/update_description.go cmd/download_attachment.go
git commit -m "feat: add jt attach, update-description, download-attachment subcommands"
```

---

### Task 4: Claude tab component (`internal/tui/claude.go`)

**Files:**
- Create: `internal/tui/claude.go`

- [ ] **Step 1: Define the Claude tab component with message types**

```go
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
	issueKey   string // ticket key (e.g., "TEC-123")
	sessionName string // tmux session name
	active     bool   // whether this tab is currently displayed
	content    string // last captured tmux pane output
	width      int
	height     int
	err        string // error message if launch failed

	// Long-press esc detection
	escPressedAt time.Time
	escHeld      bool
}

// NewClaudeTab creates a new Claude tab for the given issue.
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

		// Create session if it doesn't exist
		if !tmuxSessionExists(name) {
			if err := tmuxCreateSession(name, w, h); err != nil {
				return claudeErrMsg{err: "failed to create tmux session: " + err.Error()}
			}
			// Launch claude with the context file
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
		// Schedule next tick
		return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
			return claudeTickMsg{}
		})
	case claudeErrMsg:
		ct.err = msg.(claudeErrMsg).err
		return nil
	}
	return nil
}

// SendKey forwards a key to the tmux session.
func (ct *ClaudeTab) SendKey(key string) {
	if ct.sessionName != "" && tmuxSessionExists(ct.sessionName) {
		tmuxSendRawKey(ct.sessionName, key)
	}
}

// Resize updates dimensions and resizes the tmux window.
func (ct *ClaudeTab) Resize(width, height int) {
	ct.width = width
	ct.height = height
	if tmuxSessionExists(ct.sessionName) {
		tmuxResizeWindow(ct.sessionName, width, height)
	}
}

// Kill terminates the tmux session.
func (ct *ClaudeTab) Kill() {
	if tmuxSessionExists(ct.sessionName) {
		tmuxKillSession(ct.sessionName)
	}
}

// HasSession returns true if a tmux session exists for this tab.
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

	// Trim to height and width
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

// CheckEscHeld checks if esc has been held long enough to exit.
// Returns true if the user should exit the Claude tab.
func (ct *ClaudeTab) CheckEscHeld() bool {
	if ct.escHeld && time.Since(ct.escPressedAt) >= 1500*time.Millisecond {
		ct.escHeld = false
		return true
	}
	return false
}

// StartEscHold records that esc was pressed.
func (ct *ClaudeTab) StartEscHold() {
	ct.escPressedAt = time.Now()
	ct.escHeld = true
}

// CancelEscHold cancels the esc hold (another key was pressed).
func (ct *ClaudeTab) CancelEscHold() {
	ct.escHeld = false
}
```

- [ ] **Step 2: Build to verify compilation**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add internal/tui/claude.go
git commit -m "feat: add Claude tab component with tmux integration"
```

---

### Task 5: Add tab 5 to detail view (`internal/tui/detail.go`, `internal/tui/keys.go`)

**Files:**
- Modify: `internal/tui/detail.go`
- Modify: `internal/tui/keys.go`

- [ ] **Step 1: Add Tab5 key binding to keys.go**

In `internal/tui/keys.go`, add `Tab5` to `DetailKeyMap`:

```go
// Add to DetailKeyMap struct (after Tab4):
Tab5 key.Binding

// Add to detailKeys initialization (after Tab4):
Tab5: key.NewBinding(key.WithKeys("5")),
```

- [ ] **Step 2: Add tabClaude constant to detail.go**

In `internal/tui/detail.go`, add the new tab constant after `tabAttachments`:

```go
const (
	tabDetails detailTab = iota
	tabComments
	tabAssociations
	tabAttachments
	tabClaude
)
```

- [ ] **Step 3: Add ClaudeTab field to Detail struct**

In `internal/tui/detail.go`, add to the Detail struct:

```go
// Claude Code integration
claudeTab *ClaudeTab
```

- [ ] **Step 4: Update tabLabels to include Claude tab**

Modify the `tabLabels()` method to conditionally show the Claude tab:

```go
func (d Detail) tabLabels() []string {
	assocCount := len(d.issue.Links) + len(d.issue.Subtasks)
	labels := []string{
		"Details",
		fmt.Sprintf("Comments(%d)", len(d.issue.Comments)),
		fmt.Sprintf("Associations(%d)", assocCount),
		fmt.Sprintf("Attach(%d)", len(d.issue.Attachments)),
	}
	if d.claudeTab != nil {
		indicator := "Claude"
		if d.claudeTab.HasSession() {
			indicator = "Claude●"
		}
		labels = append(labels, indicator)
	}
	return labels
}
```

- [ ] **Step 5: Add tab 5 keyboard switching in Detail.Update()**

In the `tea.KeyMsg` section of `Detail.Update()`, after the `case key.Matches(msg, detailKeys.Tab4)` block, add:

```go
case key.Matches(msg, detailKeys.Tab5):
	if d.claudeTab != nil {
		d.activeTab = tabClaude
		d.scrollY = 0
	}
```

- [ ] **Step 6: Route Claude tab rendering in Detail.View()**

In the `View()` method, add a case for `tabClaude` in the tab content rendering switch:

```go
case tabClaude:
	if d.claudeTab != nil {
		content = d.claudeTab.View()
	}
```

- [ ] **Step 7: Update handleTabClick for 5 tabs**

In `handleTabClick`, the existing logic calculates position based on label widths. Since `tabLabels()` now returns 5 items when Claude is active, clicks on the 5th tab will automatically work with the existing loop logic. No change needed.

- [ ] **Step 8: Build to verify compilation**

Run: `go build ./...`
Expected: No errors

- [ ] **Step 9: Commit**

```bash
git add internal/tui/detail.go internal/tui/keys.go
git commit -m "feat: add Claude tab (tab 5) to detail view"
```

---

### Task 6: Integrate into app — keybindings and layout (`internal/tui/app.go`)

**Files:**
- Modify: `internal/tui/app.go`

- [ ] **Step 1: Add Claude session tracking to App struct**

Add to the `App` struct:

```go
// Claude Code sessions
claudeSessions map[string]*ClaudeTab // issueKey → tab
claudeListWidth int                  // narrowed list width when Claude active
```

Initialize in `NewApp`:

```go
claudeSessions: make(map[string]*ClaudeTab),
```

- [ ] **Step 2: Add F1 handler — launch/attach Claude session**

In the `tea.KeyMsg` case of `Update()`, after the help overlay check and before the state-specific handling, add:

```go
// F1: Launch/attach Claude Code session
if msg.String() == "f1" && a.detail != nil && a.state == stateList {
	issue := a.detail.issue
	ct, exists := a.claudeSessions[issue.Key]
	if !exists {
		// Create new session
		ct = &ClaudeTab{}
		*ct = NewClaudeTab(issue.Key, a.detailPaneWidth()-2, a.contentHeight()-2)
		a.claudeSessions[issue.Key] = ct

		// Write context and launch
		contextPath, err := writeContextFile(issue)
		if err != nil {
			ct.err = "Failed to write context: " + err.Error()
		} else {
			a.detail.claudeTab = ct
			a.detail.activeTab = tabClaude
			return a, ct.Launch(contextPath)
		}
	} else {
		// Reattach existing session
		a.detail.claudeTab = ct
		a.detail.activeTab = tabClaude
		return a, tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
			return claudeTickMsg{}
		})
	}
	return a, nil
}
```

- [ ] **Step 3: Add Shift+F1 handler — exit Claude tab**

After the F1 handler:

```go
// Shift+F1: Exit Claude tab
if msg.String() == "shift+f1" && a.detail != nil && a.detail.activeTab == tabClaude {
	a.detail.activeTab = tabDetails
	return a, nil
}
```

- [ ] **Step 4: Add F2 handler — full-screen tmux attach**

```go
// F2: Pop out to full-screen tmux
if msg.String() == "f2" && a.detail != nil && a.detail.activeTab == tabClaude {
	if ct := a.detail.claudeTab; ct != nil && ct.HasSession() {
		c := exec.Command("tmux", "attach", "-t", ct.sessionName)
		return a, tea.ExecProcess(c, func(err error) tea.Msg {
			return tea.WindowSizeMsg{Width: a.width, Height: a.height}
		})
	}
	return a, nil
}
```

Add `"os/exec"` to imports.

- [ ] **Step 5: Add Ctrl+F1 handler — kill Claude session**

```go
// Ctrl+F1: Kill Claude session
if msg.String() == "ctrl+f1" && a.detail != nil {
	issue := a.detail.issue
	if ct, ok := a.claudeSessions[issue.Key]; ok {
		ct.Kill()
		delete(a.claudeSessions, issue.Key)
		a.detail.claudeTab = nil
		if a.detail.activeTab == tabClaude {
			a.detail.activeTab = tabDetails
		}
	}
	return a, nil
}
```

- [ ] **Step 6: Forward Claude tick messages**

In the `Update()` method's message type switch, add:

```go
case claudeTickMsg:
	if a.detail != nil && a.detail.claudeTab != nil && a.detail.activeTab == tabClaude {
		cmd := a.detail.claudeTab.Update(msg)
		return a, cmd
	}
	return a, nil

case claudeSessionEndedMsg:
	if a.detail != nil && a.detail.claudeTab != nil {
		a.detail.claudeTab.err = "Claude Code session ended."
	}
	return a, nil

case claudeErrMsg:
	if a.detail != nil && a.detail.claudeTab != nil {
		a.detail.claudeTab.Update(msg)
	}
	return a, nil
```

- [ ] **Step 7: Forward keystrokes to tmux when Claude tab is active**

In the `tea.KeyMsg` handler, add this BEFORE the `a.detail.Editing()` check (around line 931):

```go
// When Claude tab is active, forward keys to tmux
if a.detail != nil && a.detail.activeTab == tabClaude && a.detail.claudeTab != nil {
	ct := a.detail.claudeTab

	// Long-press esc detection
	if msg.String() == "esc" {
		ct.StartEscHold()
		ct.SendKey("Escape")
		return a, tea.Tick(1500*time.Millisecond, func(t time.Time) tea.Msg {
			return escHoldCheckMsg{}
		})
	}

	// Any other key cancels esc hold and forwards to tmux
	ct.CancelEscHold()
	ct.SendKey(msg.String())
	return a, nil
}
```

Add the esc hold check message type near the top of the file:

```go
type escHoldCheckMsg struct{}
```

And handle it in the Update switch:

```go
case escHoldCheckMsg:
	if a.detail != nil && a.detail.claudeTab != nil {
		if a.detail.claudeTab.CheckEscHeld() {
			a.detail.activeTab = tabDetails
		}
	}
	return a, nil
```

- [ ] **Step 8: Adjust layout when Claude tab is active**

Modify the `detailPaneWidth()` or the View rendering to expand the detail pane when Claude tab is active. In the View method where the list/detail split is rendered, add:

```go
// In the View() method, when calculating listWidth for rendering:
listW := a.listWidth
if a.detail != nil && a.detail.activeTab == tabClaude {
	// Narrow list to minimum when Claude tab is active
	listW = 20
}
```

Use this `listW` instead of `a.listWidth` for rendering the split layout.

- [ ] **Step 9: Update Claude tab when switching issues**

When the selected issue changes (detail loads for a new key), check if there's an existing Claude session:

In the section where `a.detail` is set for a new issue, add:

```go
// Attach existing Claude session if any
if ct, ok := a.claudeSessions[issue.Key]; ok {
	a.detail.claudeTab = ct
}
```

- [ ] **Step 10: Build and test**

Run: `go build -o jt . && go test ./... -v`
Expected: Build succeeds, all tests pass

- [ ] **Step 11: Commit**

```bash
git add internal/tui/app.go
git commit -m "feat: integrate Claude Code sessions with F1/F2/Shift-F1/Ctrl-F1 keybindings"
```

---

### Task 7: Help text and cleanup

**Files:**
- Modify: `internal/tui/app.go` (help screen rendering)

- [ ] **Step 1: Add Claude keybindings to help screen**

Find the `renderHelpScreen()` function and add Claude Code keybindings:

```
F1          Launch/attach Claude Code for current ticket
Shift+F1    Exit Claude tab
F2          Full-screen Claude (tmux attach)
Ctrl+F1     Kill Claude session
```

- [ ] **Step 2: Add .gitignore entry for superpowers directory**

In `.gitignore`, add:

```
.superpowers/
```

- [ ] **Step 3: Build final binary and manual test**

Run: `go build -o jt . && ./jt --version`
Expected: Prints version info

Manual test steps:
1. Open jt, navigate to a ticket
2. Press F1 — Claude tab should appear with session starting
3. Type in the Claude tab — keystrokes should reach Claude Code
4. Press F2 — should pop out to full-screen tmux
5. Ctrl-B d to detach — should return to jt
6. Shift+F1 — should exit Claude tab back to Details
7. Ctrl+F1 — should kill the session

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "feat: add help text for Claude Code integration"
```
