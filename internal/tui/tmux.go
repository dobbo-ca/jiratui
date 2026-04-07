// internal/tui/tmux.go
package tui

import (
	"os/exec"
	"strconv"
	"strings"
)

func claudeSessionName(issueKey string) string {
	return "jt-claude-" + issueKey
}

func tmuxSessionExists(name string) bool {
	cmd := exec.Command("tmux", "has-session", "-t", name)
	return cmd.Run() == nil
}

func tmuxCreateSession(name string, width, height int) error {
	cmd := exec.Command("tmux", "new-session", "-d", "-s", name,
		"-x", strconv.Itoa(width), "-y", strconv.Itoa(height))
	return cmd.Run()
}

func tmuxSendKeys(name, keys string) error {
	cmd := exec.Command("tmux", "send-keys", "-t", name, keys, "Enter")
	return cmd.Run()
}

func tmuxSendRawKey(name, key string) error {
	cmd := exec.Command("tmux", "send-keys", "-t", name, key)
	return cmd.Run()
}

func tmuxCapture(name string) (string, error) {
	cmd := exec.Command("tmux", "capture-pane", "-t", name, "-p", "-e")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func tmuxResizeWindow(name string, width, height int) error {
	cmd := exec.Command("tmux", "resize-window", "-t", name,
		"-x", strconv.Itoa(width), "-y", strconv.Itoa(height))
	return cmd.Run()
}

func tmuxKillSession(name string) error {
	cmd := exec.Command("tmux", "kill-session", "-t", name)
	return cmd.Run()
}

func tmuxInstalled() bool {
	_, err := exec.LookPath("tmux")
	return err == nil
}

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
