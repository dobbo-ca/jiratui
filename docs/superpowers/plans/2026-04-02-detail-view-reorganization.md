# Detail View Reorganization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reorganize detail view layout (Title full-width, 4-col row 2, new row ordering) and add searchable parent dropdown with debounced API search.

**Architecture:** Extend existing Dropdown with async search support (minSearchLen, debounce via tea.Tick, OnSearch callback). Reorganize renderDetailsTabWithOverlays and handleDetailClick for new row layout. Add parent search API integration in app.go.

**Tech Stack:** Go, Bubbletea, Lipgloss, Jira REST API

---

### Task 1: Extend Dropdown with async search support

**Files:**
- Modify: `internal/tui/dropdown.go`

- [ ] **Step 1: Add new fields to Dropdown struct**

Add after the `StyleFunc` field:

```go
minSearchLen int            // minimum chars before search triggers (0 = filter immediately)
searchSeq    int            // debounce counter — incremented on each keystroke
OnSearch     func(query string) tea.Cmd // async search callback; when set, replaces local filtering
```

- [ ] **Step 2: Add debounce tick message type**

Add after the `DropdownItem` type:

```go
// dropdownSearchTick carries the sequence number at the time the tick was scheduled.
type dropdownSearchTick struct {
	seq   int
	label string // identifies which dropdown this tick belongs to
}
```

- [ ] **Step 3: Update the Update method to support debounced async search**

In the searchable key forwarding block (after `d.search, cmd = d.search.Update(msg)`), replace `d.applyFilter()` with logic that checks for OnSearch:

```go
if d.searchable {
	var cmd tea.Cmd
	d.search, cmd = d.search.Update(msg)
	if d.OnSearch != nil {
		query := d.search.Value()
		if len(query) < d.minSearchLen {
			d.filtered = nil
			d.rebuildAllDisplay()
			d.cursor = 0
			d.scrollOff = 0
			return d, cmd
		}
		d.searchSeq++
		seq := d.searchSeq
		label := d.label
		tickCmd := tea.Tick(300*time.Millisecond, func(t time.Time) tea.Msg {
			return dropdownSearchTick{seq: seq, label: label}
		})
		return d, tea.Batch(cmd, tickCmd)
	}
	d.applyFilter()
	return d, cmd
}
```

- [ ] **Step 4: Add HandleSearchTick method**

```go
// HandleSearchTick processes a debounce tick. Returns a search command if the tick is still current.
func (d *Dropdown) HandleSearchTick(tick dropdownSearchTick) tea.Cmd {
	if tick.seq != d.searchSeq || !d.open {
		return nil // stale tick or dropdown closed
	}
	if d.OnSearch != nil {
		return d.OnSearch(d.search.Value())
	}
	return nil
}
```

- [ ] **Step 5: Update RenderOverlay to show placeholder when below minSearchLen**

In the `len(d.filtered) == 0` block, change the "No matches" text to be context-aware:

```go
if len(d.filtered) == 0 {
	msg := "No matches"
	if d.OnSearch != nil && len(d.search.Value()) < d.minSearchLen {
		msg = fmt.Sprintf("Type %d+ chars to search...", d.minSearchLen)
	}
	noMatch := lipgloss.NewStyle().Foreground(colorSubtle).Render(msg)
	// ... rest unchanged
}
```

- [ ] **Step 6: Add time import**

Add `"time"` to imports.

- [ ] **Step 7: Build and verify**

Run: `go build ./...`

- [ ] **Step 8: Commit**

```bash
git add internal/tui/dropdown.go
git commit -m "feat(dropdown): add async search with debounce support"
```

---

### Task 2: Add parentDrop to Detail struct and constructor

**Files:**
- Modify: `internal/tui/detail.go`

- [ ] **Step 1: Add parentDrop field to Detail struct**

Add after `dueDatePick`:

```go
parentDrop   Dropdown
parentHover  bool // true when mouse is hovering over parent field
```

- [ ] **Step 2: Initialize parentDrop in NewDetail**

After the dueDatePick initialization:

```go
parentVal := "—"
parentID := ""
if issue.Parent != nil {
	parentVal = issue.Parent.Key
	parentID = issue.Parent.Key
}
parentDrop := NewDropdown("Parent", nil, parentVal, parentID, 0)
parentDrop.minSearchLen = 3
```

Add `parentDrop: parentDrop` to the return struct.

- [ ] **Step 3: Update Editing(), blurAll(), anyOverlayOpen()**

Add `d.parentDrop.IsOpen()` to all three methods.

- [ ] **Step 4: Add parentDrop key forwarding in Update**

Add before the assigneeDrop forwarding block:

```go
if d.parentDrop.IsOpen() {
	var cmd tea.Cmd
	d.parentDrop, cmd = d.parentDrop.Update(msg)
	return d, cmd
}
```

- [ ] **Step 5: Add SetParentSearchFunc method**

```go
func (d *Detail) SetParentSearchFunc(fn func(query string) tea.Cmd) {
	d.parentDrop.OnSearch = fn
}
```

- [ ] **Step 6: Build and verify**

Run: `go build ./...`

- [ ] **Step 7: Commit**

```bash
git add internal/tui/detail.go
git commit -m "feat(detail): add parentDrop field with async search support"
```

---

### Task 3: Reorganize renderDetailsTabWithOverlays layout

**Files:**
- Modify: `internal/tui/detail.go`

- [ ] **Step 1: Rewrite renderDetailsTabWithOverlays**

Replace the entire method body with the new layout:

**Row 1:** Title (full width) — uses `d.renderInputField("Title", &d.titleInput, w)`

**Row 2:** Parent + Key + Type + Due Date (4 columns, `col4W = w / 4`)
- Parent uses `d.parentDrop.View()`
- Key uses `renderField("Key", d.issue.Key, col4W, colorAccent)`
- Type uses `renderField("Type", d.issue.Type.Name, col4W, colorText)`
- Due Date uses `d.dueDatePick.View()`
- Set `d.parentDrop.width = col4W` and `d.dueDatePick.width = col4W`
- Collect parent overlay at `{y: lineY-3, x: 0}` and datepicker overlay at `{y: lineY, x: 3*col4W}`

**Row 3:** Assignee + Reporter + Status (3 columns, `col3W = w / 3`)
- Reporter uses `renderField("Reporter", reporter, col3W, colorText)`
- Set `d.assigneeDrop.width = col3W`, `d.statusDrop.width = col3W`
- Collect assignee overlay at `{y: lineY-3, x: 0}` and status overlay at `{y: lineY-3, x: 2*col3W}`

**Row 4:** Updated + Created + Priority (3 columns)
- Updated and Created are read-only `renderField`
- Set `d.priorityDrop.width = col3W`
- Collect priority overlay at `{y: lineY-3, x: 2*col3W}`

**Row 5:** Labels (full width, always shown)
- Display labels as `[label1] [label2]` or `—`

**Row 6:** Description (full width)

Remove the old Sprint row entirely.

- [ ] **Step 2: Update descMaxScroll**

Change `usedLines` calculation: 5 rows × 3 lines = 15 (labels row always present, no conditional sprint).

- [ ] **Step 3: Build and verify**

Run: `go build ./...`

- [ ] **Step 4: Commit**

```bash
git add internal/tui/detail.go
git commit -m "feat(detail): reorganize layout to new row structure"
```

---

### Task 4: Update handleDetailClick for new layout

**Files:**
- Modify: `internal/tui/detail.go`

- [ ] **Step 1: Rewrite handleDetailClick**

Update the click zones per the spec:

```go
func (d *Detail) handleDetailClick(x, y int) tea.Cmd {
	w := d.width - 1
	col4W := w / 4
	col3W := w / 3

	if d.Editing() {
		if d.anyOverlayOpen() {
			// Handle overlay clicks for each open dropdown/picker
			row2FieldY := 2 + 3  // after tab bar (2) + row 1 (3)
			row3FieldY := row2FieldY + 3
			row4FieldY := row3FieldY + 3

			if d.parentDrop.IsOpen() {
				overlay := d.parentDrop.RenderStandaloneOverlay()
				if overlay != nil && y >= row2FieldY && y < row2FieldY+len(overlay) && x < col4W {
					clickLine := y - row2FieldY - 3
					if clickLine >= 0 && d.parentDrop.HandleClick(clickLine) {
						return nil
					}
				}
			}
			if d.assigneeDrop.IsOpen() {
				overlay := d.assigneeDrop.RenderStandaloneOverlay()
				if overlay != nil && y >= row3FieldY && y < row3FieldY+len(overlay) && x < col3W {
					clickLine := y - row3FieldY - 3
					if clickLine >= 0 && d.assigneeDrop.HandleClick(clickLine) {
						return nil
					}
				}
			}
			if d.statusDrop.IsOpen() {
				overlay := d.statusDrop.RenderStandaloneOverlay()
				if overlay != nil && y >= row3FieldY && y < row3FieldY+len(overlay) && x >= 2*col3W {
					clickLine := y - row3FieldY - 3
					if clickLine >= 0 && d.statusDrop.HandleClick(clickLine) {
						return nil
					}
				}
			}
			if d.priorityDrop.IsOpen() {
				overlay := d.priorityDrop.RenderStandaloneOverlay()
				if overlay != nil && y >= row4FieldY && y < row4FieldY+len(overlay) && x >= 2*col3W {
					clickLine := y - row4FieldY - 3
					if clickLine >= 0 && d.priorityDrop.HandleClick(clickLine) {
						return nil
					}
				}
			}
			if d.dueDatePick.IsOpen() {
				overlay := d.dueDatePick.RenderOverlay()
				if overlay != nil {
					overlayStartY := row2FieldY + 3
					if y >= overlayStartY && y < overlayStartY+len(overlay) &&
						x >= 3*col4W {
						overlayLine := y - overlayStartY
						localX := x - 3*col4W
						if d.dueDatePick.HandleClick(overlayLine, localX, col4W) {
							return nil
						}
					}
				}
			}
		}
		d.blurAll()
		return nil
	}

	// Row 1: y=2..4 — Title (full width)
	if y >= 2 && y <= 4 {
		d.blurAll()
		d.titleInput.Focus()
		return d.titleInput.Cursor.BlinkCmd()
	}

	// Row 2: y=5..7 — Parent + Key + Type + Due Date (4 columns)
	if y >= 5 && y <= 7 {
		if x < col4W {
			d.blurAll()
			return d.parentDrop.OpenDropdown()
		} else if x >= 3*col4W {
			d.blurAll()
			d.dueDatePick.OpenPicker()
			return nil
		}
		d.blurAll()
		return nil
	}

	// Row 3: y=8..10 — Assignee + Reporter + Status (3 columns)
	if y >= 8 && y <= 10 {
		if x < col3W {
			d.blurAll()
			return d.assigneeDrop.OpenDropdown()
		} else if x >= 2*col3W {
			d.blurAll()
			return d.statusDrop.OpenDropdown()
		}
		d.blurAll()
		return nil
	}

	// Row 4: y=11..13 — Updated + Created + Priority (3 columns)
	if y >= 11 && y <= 13 {
		if x >= 2*col3W {
			d.blurAll()
			return d.priorityDrop.OpenDropdown()
		}
		d.blurAll()
		return nil
	}

	// Row 5: y=14..16 — Labels (read-only)
	if y >= 14 && y <= 16 {
		d.blurAll()
		return nil
	}

	// Row 6+: y=17+ — Description
	if y >= 17 {
		d.blurAll()
		d.descFocused = true
		d.descInput.Focus()
		return d.descInput.Cursor.BlinkCmd()
	}

	d.blurAll()
	return nil
}
```

- [ ] **Step 2: Remove keyFieldWidth method** (no longer needed — Key is just a regular field now)

- [ ] **Step 3: Build and verify**

Run: `go build ./...`

- [ ] **Step 4: Commit**

```bash
git add internal/tui/detail.go
git commit -m "feat(detail): update click handling for new layout"
```

---

### Task 5: Add parent hover tooltip

**Files:**
- Modify: `internal/tui/detail.go`

- [ ] **Step 1: Track mouse motion for parent hover**

In the `tea.MouseMsg` handler in `Update`, add motion tracking:

```go
if msg.Action == tea.MouseActionMotion {
	// Check if hovering over parent field (row 2, col 1)
	col4W := (d.width - 1) / 4
	d.parentHover = msg.Y >= 5 && msg.Y <= 7 && msg.X < col4W && d.issue.Parent != nil
	return d, nil
}
```

- [ ] **Step 2: Render tooltip in renderDetailsTabWithOverlays**

After rendering row 2, if `d.parentHover && d.issue.Parent != nil && !d.parentDrop.IsOpen()`:

```go
if d.parentHover && d.issue.Parent != nil && !d.parentDrop.IsOpen() {
	tooltip := d.issue.Parent.Key + ": " + d.issue.Parent.Summary
	tooltipStyle := lipgloss.NewStyle().Foreground(colorText).Background(colorSelection)
	tooltipLine := tooltipStyle.Render(truncStr(tooltip, w-2))
	overlays = append(overlays, overlayInfo{lines: []string{tooltipLine}, y: lineY, x: 0})
}
```

- [ ] **Step 3: Build and verify**

Run: `go build ./...`

- [ ] **Step 4: Commit**

```bash
git add internal/tui/detail.go
git commit -m "feat(detail): add parent hover tooltip"
```

---

### Task 6: Add parent search API integration

**Files:**
- Modify: `internal/tui/app.go`

- [ ] **Step 1: Add parentSearchResultsMsg type**

```go
type parentSearchResultsMsg struct {
	items  []DropdownItem
	forKey string // which issue detail this is for
}
```

- [ ] **Step 2: Add searchParentIssues command**

```go
func searchParentIssues(client *jira.Client, query, forKey string) tea.Cmd {
	return func() tea.Msg {
		jql := fmt.Sprintf(`summary ~ "%s" ORDER BY updated DESC`, strings.ReplaceAll(query, `"`, `\"`))
		result, err := client.SearchIssues(jql, 10, "")
		if err != nil {
			return nil
		}
		items := make([]DropdownItem, len(result.Issues))
		for i, issue := range result.Issues {
			items[i] = DropdownItem{
				ID:    issue.Key,
				Label: issue.Key + " - " + issue.Summary,
			}
		}
		return parentSearchResultsMsg{items: items, forKey: forKey}
	}
}
```

- [ ] **Step 3: Wire up OnSearch in issueDetailMsg handler**

After creating the Detail, set the parent search callback:

```go
d.SetParentSearchFunc(func(query string) tea.Cmd {
	return searchParentIssues(a.client, query, msg.issue.Key)
})
```

- [ ] **Step 4: Handle parentSearchResultsMsg**

```go
case parentSearchResultsMsg:
	if a.detail != nil && msg.forKey == a.detailKey {
		d := *a.detail
		d.parentDrop.SetItems(msg.items)
		a.detail = &d
	}
	return a, nil
```

- [ ] **Step 5: Handle dropdownSearchTick in app Update**

```go
case dropdownSearchTick:
	if a.detail != nil {
		d := *a.detail
		cmd := d.parentDrop.HandleSearchTick(msg)
		a.detail = &d
		return a, cmd
	}
	return a, nil
```

- [ ] **Step 6: Add DropdownItem import if needed**

The `DropdownItem` type is in the `tui` package, same as app.go — no import needed.

- [ ] **Step 7: Build and verify**

Run: `go build ./...`

- [ ] **Step 8: Commit**

```bash
git add internal/tui/app.go
git commit -m "feat(app): add parent issue search with debounced API calls"
```

---

### Task 7: Integration test and cleanup

- [ ] **Step 1: Run full test suite**

Run: `go test ./...`

- [ ] **Step 2: Build the binary and manual smoke test**

Run: `go build -o jiratui .`

Verify:
- Title spans full width
- Row 2 shows Parent, Key, Type, Due Date in 4 columns
- Row 3 shows Assignee, Reporter, Status in 3 columns
- Row 4 shows Updated, Created, Priority in 3 columns
- Labels row always visible
- Clicking Parent opens search dropdown
- Typing 3+ chars in parent search triggers API search
- Hovering over parent shows tooltip

- [ ] **Step 3: Commit any fixes**

- [ ] **Step 4: Push to GitHub**

```bash
git push
```
