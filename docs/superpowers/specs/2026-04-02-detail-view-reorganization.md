# Detail View Reorganization + Parent Search

## Layout

```
Row 1: Title                                          (full width, editable)
Row 2: Parent ▾  | Key        | Type      | Due Date  (4 columns)
Row 3: Assignee ▾| Reporter   | Status ▾              (3 columns)
Row 4: Updated   | Created    | Priority ▾             (3 columns)
Row 5: Labels                                          (full width, always shown)
Row 6+: Description                                    (full width, editable)
```

## Field behavior

| Field       | Editable | Click action              |
|-------------|----------|---------------------------|
| Title       | Yes      | Focus text input          |
| Parent      | Yes      | Open search dropdown      |
| Key         | No       | —                         |
| Type        | No       | —                         |
| Due Date    | Yes      | Open date picker          |
| Assignee    | Yes      | Open dropdown             |
| Reporter    | No       | —                         |
| Status      | Yes      | Open dropdown             |
| Updated     | No       | —                         |
| Created     | No       | —                         |
| Priority    | Yes      | Open dropdown             |
| Labels      | No       | — (future: editable)      |
| Description | Yes      | Focus textarea            |

## Parent field

**Display**: Shows parent key only (e.g. `TEC-1234`) or `—` if none. Rendered as a dropdown field (with `▾`).

**Hover tooltip**: When mouse hovers over the Parent field value, a tooltip line appears showing `KEY: Summary text`. Uses `tea.MouseMotion` to track hover state. Tooltip disappears when mouse leaves.

**Search dropdown**: Opens on click. Empty list initially with placeholder "Type 3+ chars to search...". After 3+ characters typed, fires a debounced Jira search.

**Debounce (300ms)**: Each keystroke increments a `searchSeq` counter. A `tea.Tick(300ms)` is sent carrying the current seq. When the tick fires, if seq matches (no newer keystrokes), the search command fires. If the user typed again, the tick is stale and ignored.

**API**: `SearchIssues` with JQL `summary ~ "query" ORDER BY updated DESC`, maxResults=10, no project filter. Results populate the dropdown as `KEY - Summary` items.

**Selection**: Choosing an item updates `issue.Parent` and fires a Jira API call to set the parent.

## Dropdown changes

Add to existing `Dropdown` struct:
- `minSearchLen int` — minimum characters before filtering/searching (0 = current behavior)
- `searchSeq int` — debounce counter
- `OnSearch func(query string) tea.Cmd` — callback for async search; when set, `applyFilter` defers to this instead of local filtering
- When `OnSearch` is set and query < minSearchLen, show placeholder text "Type N+ chars to search..." instead of "No matches"

## Click handling (handleDetailClick)

Row boundaries (each row = 3 lines: top border, value, bottom border):
- y=2..4 → Row 1 (Title): full width click focuses title input
- y=5..7 → Row 2 (Parent, Key, Type, Due Date): 4 columns, `col4W = w / 4`
  - x < col4W → Parent dropdown
  - x < 2*col4W → Key (no action)
  - x < 3*col4W → Type (no action)
  - x >= 3*col4W → Due Date picker
- y=8..10 → Row 3 (Assignee, Reporter, Status): 3 columns, `col3W = w / 3`
  - x < col3W → Assignee dropdown
  - x < 2*col3W → Reporter (no action)
  - x >= 2*col3W → Status dropdown
- y=11..13 → Row 4 (Updated, Created, Priority): 3 columns
  - x < col3W → Updated (no action)
  - x < 2*col3W → Created (no action)
  - x >= 2*col3W → Priority dropdown
- y=14..16 → Row 5 (Labels): read-only, no click action
- y=17+ → Row 6 (Description): click focuses textarea

## Overlay positioning

- Parent dropdown overlay: y starts at row 2 top (y=5), x=0, width=col4W
- Assignee dropdown overlay: y starts at row 3 top (y=8), x=0, width=col3W
- Status dropdown overlay: y starts at row 3 top (y=8), x=2*col3W, width=col3W
- Priority dropdown overlay: y starts at row 4 top (y=11), x=2*col3W, width=col3W
- Due Date picker overlay: y starts at row 2 bottom+1 (y=8), x=3*col4W, width=col4W

## Sprint row

Dropped entirely.

## Labels row

Full width, always shown. Displays labels as `[label1] [label2]` or `—` if none. Read-only for now.

## Files affected

- `internal/tui/detail.go` — layout, click handling, overlay positioning, new parentDrop field
- `internal/tui/dropdown.go` — add minSearchLen, searchSeq, OnSearch, debounce tick msg
- `internal/tui/app.go` — new searchParentIssues command, parentSearchResultsMsg handler
- `internal/jira/issues.go` — may need a lightweight search variant (existing SearchIssues works)
