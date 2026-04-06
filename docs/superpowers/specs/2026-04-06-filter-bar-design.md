# Filter Bar Design Spec

A collapsible filter bar at the top of the list pane that allows server-side JQL filtering of issues by type, status, assignee, labels, creation date range, and free-text search.

## Behavior

### Collapsed State (default)

The filter bar occupies a single line above the list separator:

```
▶ Filters (f to expand)
─────────────┼──────────────────────────
 Key          │ Summary
```

When any filter is active, the collapsed line shows a summary of active filter values:

```
▶ Filters: Status=In Progress,To Do · Assignee=Chris · From=2026-01-01 (f)
```

Values are truncated with `+N more` if they would overflow the terminal width.

### Expanded State

Press `f` to expand. The bar shows two rows of filter fields above the list:

```
▼ Filters (esc to collapse)
╭─ Issue Type ▾ ─╮ ╭─ Status ▾ ────╮ ╭─ Assignee ▾ ──╮ ╭─ Labels ▾ ────╮
│ All             │ │ All           │ │ Me            │ │ All           │
╰─────────────────╯ ╰───────────────╯ ╰───────────────╯ ╰───────────────╯
╭─ Created From ──╮ ╭─ Created Until ╮ ╭─ Search ──────────────────────────╮
│ —               │ │ —              │ │ Filter by key or summary...       │
╰─────────────────╯ ╰────────────────╯ ╰───────────────────────────────────╯
─────────────┼──────────────────────────
```

Each field uses the existing `╭─ label ▾ ─╮` dropdown box style. Field widths are calculated dynamically to fill the available list pane width:
- Row 1: Issue Type, Status, Assignee, Labels — equal widths, 4 fields across
- Row 2: Created From, Created Until (fixed ~18 chars each), Search (fills remaining width)

When a multi-select has active selections, the field value shows comma-separated names (e.g., `In Progress, To Do`), truncated with `+N` if they overflow the field width.

### Keyboard Shortcuts

All shortcuts are active only when the filter bar is expanded and no dropdown is open:

| Key | Action |
|-----|--------|
| `f` | Toggle filter bar expand/collapse (works from list view) |
| `t` | Open Issue Type multi-select |
| `s` | Open Status multi-select |
| `a` | Open Assignee multi-select |
| `l` | Open Labels multi-select |
| `d` | Open Created From date picker |
| `D` | Open Created Until date picker |
| `/` | Focus Search text input |
| `x` | Clear all filters and re-fetch |
| `esc` | Close open dropdown, or collapse bar if nothing open |

When a multi-select dropdown is open:
- `j`/`k`/`↑`/`↓` — navigate items
- `Enter`/`Space` — toggle `[✓]` on the highlighted item
- `Esc` — close dropdown (filter bar stays expanded)

When the Search input is focused:
- Typing updates the search text; re-fetch is debounced at 500ms
- `Enter` — immediately triggers search and blurs the input
- `Esc` — blurs the input (filter bar stays expanded)

### Interaction with Existing `/` Filter

The existing `/` key behavior changes:
- When the filter bar is **collapsed**, pressing `/` expands the filter bar and focuses the Search input
- When the filter bar is **expanded**, pressing `/` focuses the Search input
- The old inline filter row in the list is removed
- The Search field performs a server-side JQL `text ~ "query"` search
- The typed text is visible in the Search field in the filter bar

## Components

### MultiSelect (`internal/tui/multiselect.go`)

A new component extending the `Dropdown` pattern for multi-selection:

- **Fields**: `label`, `items []DropdownItem`, `selected map[string]bool` (set of selected IDs), `cursor int`, `open bool`, `scrollOff int`, `maxVisible int`, `width int`
- **Rendering**: Same `╭─ label ▾ ─╮` box style as `Dropdown`. When open, shows items with `[✓]`/`[ ]` prefixes. Selected items are bold/highlighted.
- **Value display**: When closed, shows "All" if nothing selected, or comma-separated labels of selected items, truncated with `+N` to fit field width.
- **Behavior**: Stays open on Enter (toggles selection). Only closes on Esc. Supports `j`/`k`/`↑`/`↓` navigation and mouse clicks.
- **Overlay**: Uses the same `RenderOverlay() []string` pattern as `Dropdown` for compositing in the App view.

### FilterBar (`internal/tui/filterbar.go`)

The main filter bar component:

```go
type FilterBar struct {
    expanded     bool
    typeDrop     MultiSelect
    statusDrop   MultiSelect
    assigneeDrop MultiSelect
    labelsDrop   MultiSelect
    createdFrom  DatePicker
    createdUntil DatePicker
    search       textinput.Model
    searchSeq    int         // debounce counter
    focusedField int         // which field is focused (-1 = none)
    width        int
    height       int         // calculated: 1 if collapsed, 8 if expanded (header + 2 field rows × 3 lines each + separator)
}
```

**Methods**:
- `NewFilterBar(width int) FilterBar` — creates with defaults
- `Update(msg tea.Msg) (FilterBar, tea.Cmd)` — handles keys/mouse when bar is focused
- `View() string` — renders collapsed or expanded state
- `Height() int` — returns current height (1 collapsed, ~7 expanded)
- `BuildJQL(projectKey string) string` — composes active filters into a JQL string
- `IsExpanded() bool`
- `HasActiveFilters() bool`
- `ActiveDropdown() bool` — returns true if any dropdown/picker is open
- `SetStatusItems(items []DropdownItem)` — populate status options
- `SetTypeItems(items []DropdownItem)` — populate issue type options
- `SetAssigneeItems(items []DropdownItem)` — populate assignee options
- `SetLabelItems(items []DropdownItem)` — populate label options
- `SetDefaultAssignee(accountID, displayName string)` — set initial "Me" filter

### JQL Builder

`FilterBar.BuildJQL(projectKey string) string` composes a JQL query from active filters:

```
project = KEY
AND issuetype IN ("Bug", "Task")
AND status IN ("In Progress", "To Do")
AND assignee IN ("accountId1", "accountId2")
AND labels IN ("frontend", "backend")
AND created >= "2026-01-01"
AND created <= "2026-03-01"
AND text ~ "search query"
ORDER BY updated DESC
```

Rules:
- Omit any clause where the filter is "All" / empty / unset
- If no assignee filter is set, no `assignee` clause (shows all assignees)
- The default on first load: assignee is set to current user's account ID (preserving existing behavior)
- `statusCategory != Done` is NOT added by default — the status multi-select gives explicit control. However, if no status filter is active, include `statusCategory != Done` to match current behavior.
- Project filter comes from the existing footer project selector (not duplicated in filter bar)
- ORDER BY comes from the existing sort state

## Data Flow

### Fetching Filter Options

On app startup (alongside existing fetches):
1. `fetchStatuses(client, projectKey)` — new: calls `GET /rest/api/3/project/{key}/statuses` to get available statuses grouped by issue type, then deduplicates into a flat list
2. `fetchIssueTypes(client, projectKey)` — new: calls `GET /rest/api/3/project/{key}/statuses` (same endpoint — returns issue types with their statuses), extracts the issue type names
3. Board assignees — already fetched via `fetchBoardAssignees`
4. Labels — gathered from fetched issues (no dedicated API call needed; updated each time issues are re-fetched)

When the project changes, re-fetch statuses, issue types, and assignees for the new project.

### Filter Change Flow

1. User toggles a value in a multi-select, picks a date, or types in search
2. Multi-select/date picker: immediately emit `filterChangedMsg{}`
3. Search input: debounce 500ms, then emit `filterChangedMsg{}`
4. `App.Update` handles `filterChangedMsg`:
   - Calls `a.filterBar.BuildJQL(a.projectKey)` to get the new JQL
   - Calls `fetchIssuesWithJQL(client, jql, sort)` to re-fetch
   - Sets `stateLoading`, clears detail
5. `issuesMsg` arrives — list updates, labels are extracted for the Labels dropdown

### New Jira Client Methods

```go
// GetProjectStatusesAndTypes fetches statuses grouped by issue type from
// GET /rest/api/3/project/{key}/statuses. Returns both the deduplicated
// status list and the issue type list (since the endpoint provides both).
func (c *Client) GetProjectStatusesAndTypes(projectKey string) ([]models.Status, []models.IssueType, error)
```

### Modified Methods

- `SearchMyIssues` is no longer called directly from the filter path. Instead, `App` uses `SearchIssues(jql, maxResults, pageToken)` with the JQL built by `FilterBar.BuildJQL`.
- `SearchMyIssues` remains available for the initial load (before filter bar is populated).
- `fetchIssues` function in `app.go` is updated to accept a JQL string parameter when filters are active.

## Integration with App

### App Changes

- Add `filterBar FilterBar` field to `App` struct
- `NewApp` initializes `FilterBar` with default assignee set to current user
- `App.Init()` adds `fetchStatuses` and `fetchIssueTypes` to the batch
- `App.Update()`:
  - Routes `f` key to toggle filter bar
  - When filter bar is expanded, forwards key/mouse events to `FilterBar.Update` before the list
  - Handles `filterChangedMsg` by re-fetching with new JQL
  - Handles `statusesMsg`, `issueTypesMsg` to populate dropdowns
  - On `issuesMsg`, extracts unique labels and updates filter bar
- `App.View()`:
  - Renders filter bar above the list in the left pane
  - Adjusts list height: `contentH - filterBar.Height()`
  - Composites multi-select overlay (same pattern as existing dropdown overlays)

### List Changes

- Remove the old `filtering bool` / `filter textinput.Model` state and the inline filter rendering from `List`
- Remove the `filterIssues` client-side function
- `List.visibleRows()` no longer subtracts for the filter row
- The `/` key binding in `listKeys` is removed (handled by filter bar now)

### Help Bar Changes

- Update help text: replace `/ filter` with `f filters`
- When filter bar is expanded: show `t type · s status · a assignee · l labels · d dates · / search · x clear · esc close`
