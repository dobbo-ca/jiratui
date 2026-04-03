package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type pickerMode int

const (
	modeDays pickerMode = iota
	modeMonths
	modeYears
)

// DatePicker is a calendar-based date selector component.
type DatePicker struct {
	label      string
	value      *time.Time
	viewMonth  time.Time // the month currently displayed
	cursor     time.Time // the day the cursor is on
	open       bool
	mode       pickerMode
	viewYear   int // center year for year picker (first year of the 12-year page)
	width      int
	valueColor lipgloss.Color
}

// NewDatePicker creates a new date picker with the given label and initial value.
func NewDatePicker(label string, value *time.Time, width int) DatePicker {
	now := time.Now()
	viewMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
	cursor := now

	if value != nil {
		viewMonth = time.Date(value.Year(), value.Month(), 1, 0, 0, 0, 0, time.Local)
		cursor = *value
	}

	return DatePicker{
		label:      label,
		value:      value,
		viewMonth:  viewMonth,
		cursor:     cursor,
		viewYear:   viewMonth.Year() - viewMonth.Year()%12,
		width:      width,
		valueColor: colorText,
	}
}

func (dp *DatePicker) SetValueColor(c lipgloss.Color) {
	dp.valueColor = c
}

func (dp DatePicker) IsOpen() bool {
	return dp.open
}

func (dp DatePicker) Value() *time.Time {
	return dp.value
}

func (dp *DatePicker) OpenPicker() {
	dp.open = true
	dp.mode = modeDays
	if dp.value != nil {
		dp.viewMonth = time.Date(dp.value.Year(), dp.value.Month(), 1, 0, 0, 0, 0, time.Local)
		dp.cursor = *dp.value
	} else {
		now := time.Now()
		dp.viewMonth = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
		dp.cursor = now
	}
	dp.viewYear = dp.viewMonth.Year() - dp.viewMonth.Year()%12
}

func (dp *DatePicker) Close() {
	dp.open = false
	dp.mode = modeDays
}

func (dp DatePicker) Update(msg tea.Msg) (DatePicker, tea.Cmd) {
	if !dp.open {
		return dp, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if dp.mode == modeYears {
				dp.mode = modeMonths
			} else if dp.mode == modeMonths {
				dp.mode = modeDays
			} else {
				dp.Close()
			}
		case "enter":
			if dp.mode == modeDays {
				t := dp.cursor
				dp.value = &t
				dp.Close()
			}
		case "left", "h":
			if dp.mode == modeDays {
				dp.cursor = dp.cursor.AddDate(0, 0, -1)
				dp.syncViewMonth()
			}
		case "right", "l":
			if dp.mode == modeDays {
				dp.cursor = dp.cursor.AddDate(0, 0, 1)
				dp.syncViewMonth()
			}
		case "up", "k":
			if dp.mode == modeDays {
				dp.cursor = dp.cursor.AddDate(0, 0, -7)
				dp.syncViewMonth()
			}
		case "down", "j":
			if dp.mode == modeDays {
				dp.cursor = dp.cursor.AddDate(0, 0, 7)
				dp.syncViewMonth()
			}
		case "H": // previous month / previous year page
			if dp.mode == modeDays {
				dp.viewMonth = dp.viewMonth.AddDate(0, -1, 0)
				dp.cursor = dp.viewMonth
			} else if dp.mode == modeYears {
				dp.viewYear -= 12
			}
		case "L": // next month / next year page
			if dp.mode == modeDays {
				dp.viewMonth = dp.viewMonth.AddDate(0, 1, 0)
				dp.cursor = dp.viewMonth
			} else if dp.mode == modeYears {
				dp.viewYear += 12
			}
		case "t": // today
			now := time.Now()
			dp.cursor = now
			dp.viewMonth = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
			dp.mode = modeDays
		case "backspace", "delete":
			dp.value = nil
			dp.Close()
		}
	}
	return dp, nil
}

func (dp *DatePicker) syncViewMonth() {
	dp.viewMonth = time.Date(dp.cursor.Year(), dp.cursor.Month(), 1, 0, 0, 0, 0, time.Local)
}

// calWidth returns the effective overlay width, clamped to min 24.
func (dp DatePicker) calWidth() int {
	w := dp.width
	if w < 24 {
		w = 24
	}
	return w
}

// View renders the closed state of the date picker field.
func (dp DatePicker) View() string {
	width := dp.width
	if width < 8 {
		width = 8
	}

	innerW := width - 2
	valW := innerW - 2

	lbl := lipgloss.NewStyle().Foreground(colorAccent)
	bdr := lipgloss.NewStyle().Foreground(colorBorder)

	var labelText string
	if dp.open {
		labelText = " " + dp.label + " 📅 "
		bdr = bdr.Foreground(colorAccent)
	} else {
		labelText = " " + dp.label + " "
	}

	dashes := innerW - lipgloss.Width(labelText) - 1
	if dashes < 0 {
		dashes = 0
	}

	top := bdr.Render("╭─") + lbl.Render(labelText) + bdr.Render(strings.Repeat("─", dashes)+"╮")

	displayVal := "—"
	if dp.value != nil {
		displayVal = dp.value.Format("2006-01-02")
	}
	content := lipgloss.NewStyle().Foreground(dp.valueColor).Render(truncStr(displayVal, valW))
	visW := lipgloss.Width(content)
	pad := valW - visW
	if pad < 0 {
		pad = 0
	}
	mid := bdr.Render("│") + " " + content + strings.Repeat(" ", pad) + " " + bdr.Render("│")

	// When open, use a connecting border instead of closing the box
	var bot string
	if dp.open {
		bot = bdr.Render("├" + strings.Repeat("─", innerW) + "┤")
	} else {
		bot = bdr.Render("╰" + strings.Repeat("─", innerW) + "╯")
	}

	return top + "\n" + mid + "\n" + bot
}

// HandleClick processes a mouse click on the overlay at the given line and
// overlay-local x position (0-based from left edge of overlay).
// overlayWidth is the actual rendered width of the overlay (col3W from the caller).
func (dp *DatePicker) HandleClick(overlayLine, localX, overlayWidth int) bool {
	if !dp.open {
		return false
	}

	// Use the caller-provided width since dp.width may be stale
	// (it's set in a value-receiver render method).
	dp.width = overlayWidth

	switch dp.mode {
	case modeYears:
		return dp.handleYearClick(overlayLine, localX)
	case modeMonths:
		return dp.handleMonthClick(overlayLine, localX)
	default:
		return dp.handleDayClick(overlayLine, localX)
	}
}

// handleDayClick handles clicks on the day-grid overlay.
func (dp *DatePicker) handleDayClick(overlayLine, localX int) bool {
	width := dp.calWidth()
	innerW := width - 2
	contentW := innerW - 2 // inside "│ " and " │"

	// Line 0: month/year header
	if overlayLine == 0 {
		// Compute positions matching the rendering exactly.
		// Layout: │ <padL> ◀ MonthYear ▶ <padR> │
		monthYear := dp.viewMonth.Format("January 2006")
		headerContent := "◀ " + monthYear + " ▶"
		headerVisW := lipgloss.Width(headerContent)
		padL := (contentW - headerVisW) / 2
		if padL < 0 {
			padL = 0
		}

		// Left zone: everything up to and including the space after ◀
		leftEnd := 2 + padL + 2
		// Right zone: starts at the space before ▶ and extends to the right edge
		rightStart := 2 + padL + headerVisW - 2

		if localX < leftEnd {
			dp.viewMonth = dp.viewMonth.AddDate(0, -1, 0)
			dp.cursor = dp.viewMonth
			return true
		}
		if localX >= rightStart {
			dp.viewMonth = dp.viewMonth.AddDate(0, 1, 0)
			dp.cursor = dp.viewMonth
			return true
		}
		// Clicked on month/year text → open year picker
		dp.viewYear = dp.viewMonth.Year() - dp.viewMonth.Year()%12
		dp.mode = modeYears
		return true
	}

	// Line 1: day-of-week headers — ignore
	if overlayLine == 1 {
		return true
	}

	// Lines 2+: calendar weeks
	startDOW := int(dp.viewMonth.Weekday())
	daysInMon := daysIn(dp.viewMonth.Month(), dp.viewMonth.Year())

	weekIdx := overlayLine - 2
	if weekIdx < 0 || weekIdx >= 6 {
		return true
	}

	// Calendar "Su Mo Tu We Th Fr Sa" = 20 visual cols, centered in content area
	calVisW := 20
	calPadL := (contentW - calVisW) / 2
	if calPadL < 0 {
		calPadL = 0
	}

	// Calendar content starts at localX = 2 (border+space) + calPadL
	relX := localX - 2 - calPadL
	if relX < 0 || relX >= calVisW {
		return true // clicked on padding
	}

	dow := relX / 3
	if dow > 6 {
		dow = 6
	}

	dayNum := weekIdx*7 + dow - startDOW + 1
	if dayNum >= 1 && dayNum <= daysInMon {
		selected := time.Date(dp.viewMonth.Year(), dp.viewMonth.Month(), dayNum, 0, 0, 0, 0, time.Local)
		dp.cursor = selected
		dp.value = &selected
		dp.Close()
		return true
	}

	return true // empty cell, consume
}

// handleYearClick handles clicks on the year-picker overlay.
func (dp *DatePicker) handleYearClick(overlayLine, localX int) bool {
	width := dp.calWidth()
	innerW := width - 2
	contentW := innerW - 2

	// Line 0: header with arrows "◀  2020 – 2031  ▶"
	if overlayLine == 0 {
		rangeStr := fmt.Sprintf("%d – %d", dp.viewYear, dp.viewYear+11)
		headerContent := "◀ " + rangeStr + " ▶"
		headerVisW := lipgloss.Width(headerContent)
		padL := (contentW - headerVisW) / 2
		if padL < 0 {
			padL = 0
		}

		leftEnd := 2 + padL + 2
		rightStart := 2 + padL + headerVisW - 2

		if localX < leftEnd {
			dp.viewYear -= 12
			return true
		}
		if localX >= rightStart {
			dp.viewYear += 12
			return true
		}
		return true // consume
	}

	// Lines 1-3: year grid (3 rows × 4 cols)
	rowIdx := overlayLine - 1
	if rowIdx < 0 || rowIdx >= 3 {
		return true
	}

	// Each year cell: "2020" (4 chars), separated by 2 spaces
	// Row: "2020  2021  2022  2023" = 4+2+4+2+4+2+4 = 22 chars
	gridVisW := 22
	gridPadL := (contentW - gridVisW) / 2
	if gridPadL < 0 {
		gridPadL = 0
	}

	relX := localX - 2 - gridPadL
	if relX < 0 || relX >= gridVisW {
		return true
	}

	// Each cell is 6 chars wide (4 digits + 2 spaces), last cell is 4 chars
	col := relX / 6
	if col > 3 {
		col = 3
	}

	yearIdx := rowIdx*4 + col
	if yearIdx >= 0 && yearIdx < 12 {
		selectedYear := dp.viewYear + yearIdx
		dp.viewMonth = time.Date(selectedYear, dp.viewMonth.Month(), 1, 0, 0, 0, 0, time.Local)
		dp.mode = modeMonths
		return true
	}

	return true
}

// handleMonthClick handles clicks on the month-picker overlay.
func (dp *DatePicker) handleMonthClick(overlayLine, localX int) bool {
	width := dp.calWidth()
	innerW := width - 2
	contentW := innerW - 2

	// Line 0: year header (just the year, clickable to go back to year picker)
	if overlayLine == 0 {
		dp.viewYear = dp.viewMonth.Year() - dp.viewMonth.Year()%12
		dp.mode = modeYears
		return true
	}

	// Lines 1-3: month grid (3 rows × 4 cols)
	rowIdx := overlayLine - 1
	if rowIdx < 0 || rowIdx >= 3 {
		return true
	}

	// Each month cell: "Jan" (3 chars), separated by 2 spaces
	// Row: "Jan  Feb  Mar  Apr" = 3+2+3+2+3+2+3 = 18 chars
	gridVisW := 18
	gridPadL := (contentW - gridVisW) / 2
	if gridPadL < 0 {
		gridPadL = 0
	}

	relX := localX - 2 - gridPadL
	if relX < 0 || relX >= gridVisW {
		return true
	}

	// Each cell is 5 chars wide (3 chars + 2 spaces), last is 3
	col := relX / 5
	if col > 3 {
		col = 3
	}

	monthIdx := rowIdx*4 + col // 0-based month index
	if monthIdx >= 0 && monthIdx < 12 {
		month := time.Month(monthIdx + 1)
		dp.viewMonth = time.Date(dp.viewMonth.Year(), month, 1, 0, 0, 0, 0, time.Local)
		dp.cursor = dp.viewMonth
		dp.mode = modeDays
		return true
	}

	return true
}

// RenderOverlay returns the calendar overlay lines. Returns nil if closed.
func (dp DatePicker) RenderOverlay() []string {
	if !dp.open {
		return nil
	}

	switch dp.mode {
	case modeYears:
		return dp.renderYearOverlay()
	case modeMonths:
		return dp.renderMonthOverlay()
	default:
		return dp.renderDayOverlay()
	}
}

// renderDayOverlay renders the day-grid calendar overlay.
func (dp DatePicker) renderDayOverlay() []string {
	width := dp.calWidth()
	innerW := width - 2

	bdr := lipgloss.NewStyle().Foreground(colorAccent)
	headerStyle := lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	dayHeaderStyle := lipgloss.NewStyle().Foreground(colorSubtle)
	normalDay := lipgloss.NewStyle().Foreground(colorText)
	todayStyle := lipgloss.NewStyle().Foreground(colorInfo)
	cursorStyle := lipgloss.NewStyle().Foreground(colorText).Background(colorSelection).Bold(true)
	otherMonth := lipgloss.NewStyle().Foreground(colorSubtle)

	var lines []string

	// Month/year header
	monthYear := dp.viewMonth.Format("January 2006")
	headerContent := "◀ " + headerStyle.Render(monthYear) + " ▶"
	lines = append(lines, dp.centeredLine(bdr, headerContent, innerW))

	// Day-of-week headers
	dayHeaders := dayHeaderStyle.Render("Su Mo Tu We Th Fr Sa")
	lines = append(lines, dp.centeredLine(bdr, dayHeaders, innerW))

	// Calendar grid
	today := time.Now()
	startDOW := int(dp.viewMonth.Weekday()) // 0=Sun
	daysInMonth := daysIn(dp.viewMonth.Month(), dp.viewMonth.Year())

	day := 1 - startDOW
	for week := 0; week < 6; week++ {
		if day > daysInMonth {
			break
		}
		var cells []string
		for dow := 0; dow < 7; dow++ {
			if day < 1 || day > daysInMonth {
				cells = append(cells, otherMonth.Render("  "))
			} else {
				dayStr := fmt.Sprintf("%2d", day)
				current := time.Date(dp.viewMonth.Year(), dp.viewMonth.Month(), day, 0, 0, 0, 0, time.Local)
				if sameDay(current, dp.cursor) {
					cells = append(cells, cursorStyle.Render(dayStr))
				} else if sameDay(current, today) {
					cells = append(cells, todayStyle.Render(dayStr))
				} else {
					cells = append(cells, normalDay.Render(dayStr))
				}
			}
			day++
		}

		weekLine := strings.Join(cells, " ")
		lines = append(lines, dp.centeredLine(bdr, weekLine, innerW))
	}

	lines = append(lines, bdr.Render("╰"+strings.Repeat("─", innerW)+"╯"))

	return lines
}

// renderYearOverlay renders the year-picker overlay.
func (dp DatePicker) renderYearOverlay() []string {
	width := dp.calWidth()
	innerW := width - 2

	bdr := lipgloss.NewStyle().Foreground(colorAccent)
	headerStyle := lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(colorText)
	currentStyle := lipgloss.NewStyle().Foreground(colorText).Background(colorSelection).Bold(true)

	var lines []string

	// Header: ◀  2020 – 2031  ▶
	rangeStr := fmt.Sprintf("%d – %d", dp.viewYear, dp.viewYear+11)
	headerContent := "◀ " + headerStyle.Render(rangeStr) + " ▶"
	lines = append(lines, dp.centeredLine(bdr, headerContent, innerW))

	// 3 rows × 4 cols of years
	currentYear := dp.viewMonth.Year()
	for row := 0; row < 3; row++ {
		var cells []string
		for col := 0; col < 4; col++ {
			yr := dp.viewYear + row*4 + col
			yrStr := fmt.Sprintf("%4d", yr)
			if yr == currentYear {
				cells = append(cells, currentStyle.Render(yrStr))
			} else {
				cells = append(cells, normalStyle.Render(yrStr))
			}
		}
		rowLine := strings.Join(cells, "  ")
		lines = append(lines, dp.centeredLine(bdr, rowLine, innerW))
	}

	lines = append(lines, bdr.Render("╰"+strings.Repeat("─", innerW)+"╯"))

	return lines
}

// renderMonthOverlay renders the month-picker overlay.
func (dp DatePicker) renderMonthOverlay() []string {
	width := dp.calWidth()
	innerW := width - 2

	bdr := lipgloss.NewStyle().Foreground(colorAccent)
	headerStyle := lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(colorText)
	currentStyle := lipgloss.NewStyle().Foreground(colorText).Background(colorSelection).Bold(true)

	var lines []string

	// Header: just the year (clickable to go back to year picker)
	yearStr := fmt.Sprintf("%d", dp.viewMonth.Year())
	headerContent := headerStyle.Render(yearStr)
	lines = append(lines, dp.centeredLine(bdr, headerContent, innerW))

	// 3 rows × 4 cols of months
	monthNames := []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	currentMonth := dp.viewMonth.Month()
	for row := 0; row < 3; row++ {
		var cells []string
		for col := 0; col < 4; col++ {
			idx := row*4 + col
			m := time.Month(idx + 1)
			if m == currentMonth {
				cells = append(cells, currentStyle.Render(monthNames[idx]))
			} else {
				cells = append(cells, normalStyle.Render(monthNames[idx]))
			}
		}
		rowLine := strings.Join(cells, "  ")
		lines = append(lines, dp.centeredLine(bdr, rowLine, innerW))
	}

	lines = append(lines, bdr.Render("╰"+strings.Repeat("─", innerW)+"╯"))

	return lines
}

// centeredLine renders content centered inside │ ... │ borders with given innerW.
func (dp DatePicker) centeredLine(bdr lipgloss.Style, content string, innerW int) string {
	visW := lipgloss.Width(content)
	contentW := innerW - 2 // space after │ and before │
	padL := (contentW - visW) / 2
	if padL < 0 {
		padL = 0
	}
	padR := contentW - visW - padL
	if padR < 0 {
		padR = 0
	}
	return bdr.Render("│") + " " + strings.Repeat(" ", padL) + content + strings.Repeat(" ", padR) + " " + bdr.Render("│")
}

func daysIn(m time.Month, year int) int {
	return time.Date(year, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func sameDay(a, b time.Time) bool {
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}
