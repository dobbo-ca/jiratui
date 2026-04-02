# jiratui — Go TUI for Jira Cloud

## Overview

A terminal user interface for browsing and interacting with Jira Cloud, built in Go. Ships as a single static binary with zero runtime dependencies — replacing the Python-based jiratui which pulls in 38+ dependencies (Pillow, Cairo, X11 libs, image codecs) just to show tickets in a terminal.

## Goals

- Single binary, zero dependencies (`go install` or `brew install`)
- Fast startup, responsive UI
- Keyboard-first with full mouse support
- Incremental build: each milestone is a usable, testable tool
- Jira Cloud only (v3 REST API)

## Architecture

Three layers:

1. **API layer** (`internal/jira/`) — Thin Jira Cloud REST v3 client. Handles auth (email + API token), pagination (50 items per page, load-more on scroll), rate limiting, and error mapping.

2. **Domain layer** (`internal/models/`) — Clean Go structs for Issue, Comment, User, etc. Transforms Jira's verbose JSON responses into focused types the TUI consumes.

3. **TUI layer** (`internal/tui/`) — Bubble Tea app using the Elm architecture (Model → Update → View). Composable views with Lip Gloss styling.

### Data Flow

- Startup: fetch assigned issues, cache in memory
- Detail view: fetch full issue data on demand (comments, subtasks, links), cache for the session
- Mutations (status change, add comment): fire API call with inline spinner, optimistically update cache on success
- No disk cache — fresh data every launch
- Refresh with `r` key

## Config & Auth

Config file: `~/.config/jiratui/config.yaml`

```yaml
active_profile: work

profiles:
  work:
    url: https://company.atlassian.net
    email: chris@company.com
    api_token: xxxxxxxxxxx
  personal:
    url: https://personal.atlassian.net
    email: chris@personal.com
    api_token: xxxxxxxxxxx
```

- Multiple profiles with named identifiers
- `active_profile` determines which is used on launch
- First run with no config: interactive auth flow prompts for URL, email, API token, writes config
- Add profiles via `jiratui auth add` CLI command or from within the TUI
- Profile switching visible in TUI status bar, clickable to switch

## UI Layout

### Responsive Layout

- **Wide terminal (~120+ cols):** Split view — issue list on left, tabbed detail pane on right
- **Narrow terminal:** Full-screen swap — list takes full screen, enter opens detail as full-screen replacement, esc goes back

### Status Bar (top)

```
☰ jiratui                                    ● work           ? help
```

Shows app name, active profile (clickable to switch), help shortcut.

### Quick Filters (top, below status bar)

Always-visible filter dropdowns:
- Project
- Status
- Assignee(s)
- Sort By

### Issue List

Columns: Key, Priority (color-coded icon), Status (color-coded), Assignee, Updated, Summary.

- Selected row highlighted with left border accent
- Click or j/k + enter to open
- `/` for instant text/ticket ID filter (inline, filters as you type)

### Advanced Filters

Triggered by key combo (e.g. `ctrl+f`), opens a modal/overlay panel with:
- Work Item Key
- Created From / Created To (date range)
- JQL Query (free text)
- Label

### Detail View (tabbed)

Tabs: **Details** | **Description** | **Comments** (count) | **Subtasks** (count) | **Links** (count)

Switch tabs with number keys `1-5` or click.

#### Details Tab

Metadata grid showing:
- Title (large, bold)
- Status (dropdown, clickable to transition)
- Priority (color-coded)
- Type
- Assignee (dropdown, clickable to reassign)
- Reporter
- Sprint
- Parent (clickable link to parent issue)
- Labels (styled tags)
- Created / Updated / Due Date (due date highlighted red if overdue)

#### Description Tab

Rendered issue description, scrollable.

#### Comments Tab

List of comments with author, relative timestamp, and body. Expandable/collapsible. Ability to add new comments.

#### Subtasks Tab

List of subtasks with status indicators (✓ done, ◦ in progress/todo). Clickable to navigate to subtask.

#### Links Tab

Linked issues with relationship type ("blocks", "is blocked by", "relates to"). Clickable to navigate.

### Bottom Bar

Context-sensitive keyboard shortcuts:

List view:
```
enter open · j/k navigate · / filter · o open in browser · q quit
```

Detail view:
```
esc back · 1-5 tabs · j/k scroll · o open in browser · c comment
```

### Interaction Model

- **Keyboard-first:** j/k navigation, enter to select, esc to go back, vim-style
- **Full mouse support:** click rows, click tabs, click dropdowns, scroll wheel
- **Dropdowns:** Status and Assignee fields are clickable, show a dropdown overlay for selection

## Theme

Tokyo Night-inspired dark theme:
- Background: `#1a1b26`
- Header/footer: `#16161e`
- Borders: `#292e42`
- Text: `#c9d1d9`
- Accent/links: `#7aa2f7` (blue)
- Success/names: `#9ece6a` (green)
- Warning/in-progress: `#e0af68` (amber)
- Error/critical: `#f7768e` (red)
- Info/tags: `#7dcfff` (cyan)
- Subtle: `#565f89` (gray)
- Review/special: `#bb9af7` (purple)

## Project Structure

```
jiratui/
├── main.go                  # Entry point, CLI commands (auth, ui)
├── internal/
│   ├── config/              # Config file parsing, profile management
│   ├── jira/                # REST API client, request/response types
│   ├── models/              # Domain types (Issue, Comment, User, etc.)
│   └── tui/
│       ├── app.go           # Root Bubble Tea model, routing
│       ├── keys.go          # Key bindings
│       ├── styles.go        # Lip Gloss styles/theme
│       ├── components/      # Reusable widgets (filter bar, tabs, spinner)
│       └── views/           # List view, detail view, auth flow
├── go.mod
└── go.sum
```

### Dependencies

- `charmbracelet/bubbletea` — TUI framework
- `charmbracelet/lipgloss` — terminal styling
- `charmbracelet/bubbles` — pre-built components (table, text input, spinner, viewport)
- `gopkg.in/yaml.v3` — YAML config parsing
- Standard library for HTTP, JSON

No other external dependencies. Single binary output.

## Implementation Milestones

Each milestone is independently testable. Build, verify, get feedback, then continue.

### Milestone 1: Config & Auth

- Config file reading/writing (`~/.config/jiratui/config.yaml`)
- Profile struct with URL, email, API token
- `jiratui auth add` command: interactive prompt for URL, email, token
- First-run detection: if no config exists, run auth flow automatically
- `jiratui auth list` to show profiles
- `jiratui auth switch <name>` to change active profile
- **Test:** run `jiratui auth add`, enter credentials, verify config file written. Run `jiratui auth list`, see profiles.

### Milestone 2: API Client

- Jira Cloud REST v3 HTTP client
- Auth: email + API token (Basic auth header)
- Fetch issues assigned to current user (`assignee = currentUser()`)
- Fetch single issue with full detail (comments, subtasks, links)
- Pagination support (50 per page)
- Error handling: auth failures, network errors, rate limits
- Domain model mapping: Jira JSON → Go structs
- **Test:** run a CLI command (e.g. `jiratui issues`) that prints your real Jira issues as formatted text to stdout. Verify data is correct against Jira web UI.

### Milestone 3: List View

- Bubble Tea app with issue table
- Columns: Key, Priority, Status, Assignee, Updated, Summary
- j/k and arrow key navigation, mouse click to select
- Scroll wheel support
- `/` quick filter (filters list as you type by text or ticket ID)
- Loading spinner on startup
- `r` to refresh
- `q` to quit
- Status bar with profile name
- Bottom bar with keybindings
- **Test:** launch `jiratui`, see your real issues in a table, navigate with keyboard and mouse, filter with `/`, verify data matches Jira.

### Milestone 4: Detail View

- Responsive layout: split view (wide) vs full-screen swap (narrow)
- Tabbed detail pane: Details, Description, Comments, Subtasks, Links
- Details tab: metadata grid with all fields (title, status, priority, type, assignee, reporter, sprint, parent, labels, dates)
- Description tab: scrollable rendered description
- Comments tab: list with author, timestamp, body
- Subtasks tab: list with status indicators, clickable
- Links tab: linked issues with relationship type, clickable
- Tab switching via number keys and mouse click
- `esc` to go back to list
- `o` to open in browser
- **Test:** click into a ticket, verify all fields are correct, switch tabs, check comments/subtasks/links match Jira. Resize terminal to test responsive layout.

### Milestone 5: Quick Filters & Advanced Filters

- Top bar filter dropdowns: Project, Status, Assignee(s), Sort By
- Mouse-clickable dropdowns with overlay selection
- `ctrl+f` advanced filter panel: Work Item Key, Created From/To, JQL Query, Label
- Filters applied to API query (server-side filtering where possible)
- **Test:** filter by project, by status, by assignee. Use JQL query. Verify results match Jira web UI filters.

### Milestone 6: Mutations

- Status transitions: click status dropdown in detail view, select new status, fires API call
- Add comment: `c` key opens text input, submit posts comment
- Inline loading spinners during mutations
- Optimistic cache update on success
- Error display on failure
- **Test:** transition a ticket status, verify in Jira web UI. Add a comment, verify it appears in Jira.

## Future Milestones (for later brainstorming)

These are deferred to future sessions. Each should go through its own brainstorm → design → plan cycle.

### Kanban Board View

A board-style view showing columns for each status (To Do, In Progress, In Review, Done, etc.). Cards are issue summaries that can be dragged (or keyboard-moved) between columns to transition status. Key design questions to explore:
- How to handle boards with many statuses (horizontal scrolling vs collapsing)
- Card density — how much info per card
- Which board to show (sprint board vs kanban board, project-specific)
- How column mappings work (Jira board config vs custom)

### Search & Browse

JQL-powered search across all projects, not just assigned issues. Full-text search across descriptions and comments. Browse any project's backlog. Key questions:
- Saved searches / recent searches
- How search results differ from "my issues" list (different columns? different default sort?)
- Project browser vs flat search

### Dev Info Integration (PRs, Commits, Branches)

Show related development information on the detail view — linked PRs, commits, branches from GitHub/Bitbucket/GitLab. Requires Jira's development information API which uses OAuth scopes beyond basic API token auth. Key questions:
- OAuth flow vs app-level token
- Which SCM providers to support
- How to display PR status (open, merged, declined) and review state
- Whether to show commit history or just PR links

### Create & Edit Issues

Full issue creation form within the TUI — project, type, summary, description, assignee, priority, labels, parent. Editing existing issue fields beyond status. Key questions:
- Field validation and required fields per issue type
- Rich text editing for descriptions (markdown? plain text?)
- Template support for common issue types

### Notifications & Activity Feed

Real-time or polled notifications for mentions, status changes, comments on watched issues. Activity feed showing recent changes across your projects. Key questions:
- Polling interval vs webhook support
- Which events to surface
- Desktop notification integration
