# Claude Code Integration — Design Spec

**Date:** 2026-04-07

## Overview

Integrate Claude Code into jt as a new tab (tab 5: Claude) in the detail pane. When activated for a ticket, a tmux session is created running Claude Code with the ticket's context pre-loaded. The user interacts with Claude Code directly within the detail pane, with the option to pop out to full-screen tmux for more space.

## User Experience

### Activation
- **F1** while viewing a ticket creates (or reattaches to) a Claude Code tmux session for that ticket
- Switches to the Claude tab automatically
- Detail pane expands (list narrows/collapses) to give Claude Code more room

### Interaction
- All keystrokes are forwarded to the tmux session when the Claude tab is active
- Claude Code runs normally — the user types, Claude responds
- Claude Code can use `jt` CLI subcommands to interact with the ticket (download attachments, upload plans, update description)

### Navigation
- **Esc held >1.5s** or **Shift+F1** — exit Claude tab, return to previous detail tab, detail pane resizes back to normal
- **F2** — pop out to full-screen tmux attach (TUI suspends, `Ctrl-B d` or tmux detach returns to TUI)
- **Ctrl+F1** — kill the Claude session for the current ticket
- Clicking other tabs (1-4) exits Claude input mode and shows that tab normally

### Multiple Sessions
- Each ticket gets its own tmux session named `jt-claude-{ticket-key}`
- Sessions persist in the background when you navigate away
- Switching to a different ticket and pressing F1 creates/attaches a separate session
- The Claude tab shows the session for whichever ticket is currently selected in the list
- A visual indicator (e.g., dot on tab) shows which tickets have active Claude sessions

## Architecture

### Components

**1. Claude Tab (detail pane tab 5)**
- Renders tmux pane output captured via `tmux capture-pane -t {session} -p -e` (with ANSI escape codes)
- Forwards keystrokes via `tmux send-keys -t {session} {key}`
- Polls tmux output on a ticker (e.g., every 100ms) for smooth updates
- Handles resize by sending `tmux resize-window` when the detail pane dimensions change

**2. Session Manager**
- Creates tmux sessions: `tmux new-session -d -s jt-claude-{key} -x {width} -y {height}`
- Tracks active sessions per ticket key
- Launches Claude Code inside the session with the context prompt file
- Detects when Claude Code exits (session ends)
- Cleanup on TUI exit: optionally kill all `jt-claude-*` sessions

**3. Context Builder**
- Gathers ticket data from the already-loaded issue model (no extra API calls needed)
- Writes a markdown context file to `/tmp/jt-claude/{ticket-key}/context.md`
- Includes: key, project, summary, description, status, priority, type, assignee, comments, attachment listing
- Includes instructions for available `jt` CLI subcommands

**4. jt CLI Subcommands**
New subcommands under `jt` that Claude Code (or the user) can invoke from within the tmux session:

- `jt attach <project-key> <ticket-key> <file-path>` — upload a file as an attachment to the ticket
- `jt update-description <project-key> <ticket-key> <file-path>` — replace the ticket description with the contents of a markdown file
- `jt download-attachment <project-key> <ticket-key> <filename> [dest-dir]` — download a ticket attachment to a local path (defaults to current directory)

These subcommands reuse the existing config/auth system — they load `~/.config/jt/config.json`, find the matching profile, and use the Jira API client.

### Data Flow

```
F1 pressed on TEC-2787
        │
        ▼
┌─ Context Builder ──────────────────────────┐
│ 1. Read issue from app.detail.issue        │
│ 2. Format as markdown                      │
│ 3. Write to /tmp/jt-claude/TEC-2787/       │
│    context.md                              │
└────────────────────┬───────────────────────┘
                     │
                     ▼
┌─ Session Manager ──────────────────────────┐
│ 1. tmux new-session -d                     │
│    -s jt-claude-TEC-2787                   │
│    -x {pane-width} -y {pane-height}        │
│ 2. tmux send-keys                          │
│    "claude --prompt-file .../context.md"   │
│    Enter                                   │
└────────────────────┬───────────────────────┘
                     │
                     ▼
┌─ Claude Tab (render loop) ─────────────────┐
│ Every 100ms:                               │
│ 1. tmux capture-pane -t ... -p -e          │
│ 2. Render ANSI output in detail viewport   │
│                                            │
│ On keystroke:                              │
│ 1. tmux send-keys -t ... {key}             │
│                                            │
│ On resize:                                 │
│ 1. tmux resize-window -t ... -x W -y H    │
└────────────────────────────────────────────┘
```

### Context Prompt Template

```markdown
# Jira Ticket: {key}
**Project:** {project} | **Status:** {status} | **Priority:** {priority}
**Type:** {type} | **Assignee:** {assignee}

## Summary
{summary}

## Description
{description}

## Comments
**{author}** ({date}):
> {comment body}

## Attachments
- {filename} ({size})
- {filename} ({size})

## Available Commands
You have access to the `jt` CLI for interacting with this ticket:

- Download an attachment:
  `jt download-attachment {project} {key} "filename" /tmp/`

- Attach a file to this ticket:
  `jt attach {project} {key} path/to/file`

- Update the ticket description:
  `jt update-description {project} {key} path/to/file.md`
```

### Layout Behavior

**Normal mode (tabs 1-4 active):**
```
[ List (dynamic) | Detail (dynamic) ]
```
Standard split with the existing resizable divider.

**Claude tab active:**
```
[ List (narrowed) |     Claude tab (expanded)     ]
```
List narrows to a minimum width (enough to show keys + truncated summaries). Detail pane expands to fill remaining space. The Claude tab content fills the detail viewport.

**Full-screen mode (F2):**
TUI shells out via Bubble Tea's `tea.Exec(exec.Command("tmux", "attach", "-t", session), ...)`. The TUI suspends while tmux is attached. Detaching (`Ctrl-B d`) exits the tmux attach command and the TUI resumes automatically.

### Long-press Esc Detection

Track esc key-down timing in the TUI's Update loop:
- On esc keypress: record timestamp, start a 1.5s timer
- If another key is pressed before timer fires: cancel timer, forward the esc to tmux as normal
- If timer fires with no intervening key: trigger Claude tab exit

This ensures single esc taps work normally in Claude Code (interrupt, cancel) while a held esc exits the embedded view.

### Session Lifecycle

1. **Creation:** F1 pressed, no existing session → create tmux session, write context, launch Claude
2. **Reattach:** F1 pressed, session exists → switch to Claude tab, resume capture loop
3. **Background:** User switches to another tab or ticket → capture loop pauses, session continues in tmux
4. **Kill:** Ctrl+F1 → `tmux kill-session -t jt-claude-{key}`, remove from tracked sessions
5. **Natural exit:** Claude Code exits → session ends, tab shows "Session ended" with option to restart
6. **TUI exit:** On quit, prompt to kill active Claude sessions or leave them running

### Error Handling

- **tmux not installed:** Show error message on F1, suggest `brew install tmux`
- **claude not installed:** Show error in the tmux session (it will fail to launch, user sees the error)
- **Session creation fails:** Display error in Claude tab area
- **Capture fails (session died):** Detect and show "Session ended" state

## New Files

| File | Purpose |
|------|---------|
| `internal/tui/claude.go` | Claude tab component: render loop, keystroke forwarding, session state |
| `internal/tui/tmux.go` | tmux session management: create, capture, send-keys, resize, kill |
| `internal/tui/context.go` | Context builder: issue → markdown prompt file |
| `cmd/attach.go` | `jt attach` subcommand |
| `cmd/update_description.go` | `jt update-description` subcommand |
| `cmd/download_attachment.go` | `jt download-attachment` subcommand |

## Modified Files

| File | Changes |
|------|---------|
| `internal/tui/app.go` | F1/F2/Shift+F1/Ctrl+F1 keybindings, layout resize logic for Claude tab, session tracking |
| `internal/tui/detail.go` | Add tab 5 (Claude), route rendering/input to claude component when active |
| `internal/tui/keys.go` | New key definitions for F1, F2, Shift+F1, Ctrl+F1 |
| `cmd/root.go` | Register new subcommands |

## Out of Scope

- MCP server / bidirectional API between Claude and jt (push-only for now)
- Auto-detecting when Claude's plan is "done" (user-driven)
- Streaming Claude API directly (we use the Claude Code CLI binary)
- Custom Claude Code configuration (uses whatever the user has configured)
