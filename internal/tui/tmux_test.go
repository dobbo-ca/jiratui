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
	if tmuxSessionExists("jt-test-nonexistent-99999") {
		t.Error("expected false for nonexistent session")
	}
}
