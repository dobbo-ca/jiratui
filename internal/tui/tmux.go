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

// bubbteaToTmuxKey maps Bubble Tea key names to tmux key names.
var bubbleTeaToTmuxKey = map[string]string{
	"backspace": "BSpace",
	"enter":     "Enter",
	"tab":       "Tab",
	"up":        "Up",
	"down":      "Down",
	"left":      "Left",
	"right":     "Right",
	"home":      "Home",
	"end":       "End",
	"pgup":      "PageUp",
	"pgdown":    "PageDown",
	"delete":    "DC",
	"esc":       "Escape",
	"space":     "Space",
	"f1":        "F1",
	"f2":        "F2",
	"f3":        "F3",
	"f4":        "F4",
	"f5":        "F5",
	"f6":        "F6",
	"f7":        "F7",
	"f8":        "F8",
	"f9":        "F9",
	"f10":       "F10",
	"f11":       "F11",
	"f12":       "F12",
	"ctrl+a":    "C-a",
	"ctrl+b":    "C-b",
	"ctrl+c":    "C-c",
	"ctrl+d":    "C-d",
	"ctrl+e":    "C-e",
	"ctrl+f":    "C-f",
	"ctrl+g":    "C-g",
	"ctrl+h":    "C-h",
	"ctrl+k":    "C-k",
	"ctrl+l":    "C-l",
	"ctrl+n":    "C-n",
	"ctrl+p":    "C-p",
	"ctrl+r":    "C-r",
	"ctrl+s":    "C-s",
	"ctrl+u":    "C-u",
	"ctrl+w":    "C-w",
	"ctrl+z":    "C-z",
}

func tmuxSendRawKey(name, key string) error {
	// Check if it's a special key that needs translation
	if tmuxKey, ok := bubbleTeaToTmuxKey[key]; ok {
		cmd := exec.Command("tmux", "send-keys", "-t", name, tmuxKey)
		return cmd.Run()
	}
	// For regular characters, use -l (literal) to prevent tmux interpretation
	cmd := exec.Command("tmux", "send-keys", "-t", name, "-l", key)
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
