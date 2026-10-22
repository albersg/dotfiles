package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/albersg/dotfiles/installer/internal/updates"
	"github.com/charmbracelet/lipgloss"
)

// TestUpdateCenterWideDrawsTheReferenceTableAndDetail pins the wide shape the
// reference draws: the grouped table with its header row and its own counts, the
// selected entry's card and the actions card. The detail is checked for the fact
// the table row cannot hold (the command the answer was read with) and for the
// absence of the two facts the row already states.
func TestUpdateCenterWideDrawsTheReferenceTableAndDetail(t *testing.T) {
	m := updateCenterFrameCase(t)
	m.Width, m.Height = 160, 50
	m.Animating = false

	view := m.View()
	plain := ansiEscape.ReplaceAllString(view, "")

	for _, want := range []string{"Package", "Current → New version", "Manager", "Scope", "Actions", "Read with"} {
		if !strings.Contains(plain, want) {
			t.Errorf("the wide update center does not draw %q:\n%s", want, plain)
		}
	}
	if !strings.Contains(plain, "updates)") {
		t.Errorf("the grouped table does not carry a group count:\n%s", plain)
	}
	if strings.Contains(plain, "Name        ") || strings.Contains(plain, "Manager     ") {
		t.Errorf("the detail repeats a fact the row already states:\n%s", plain)
	}
	if rows := renderedRowCount(view); rows > m.Height {
		t.Errorf("the wide update center draws %d rows at 160x50, want <= %d", rows, m.Height)
	}
	for _, line := range strings.Split(plain, "\n") {
		if w := lipgloss.Width(line); w > m.Width {
			t.Errorf("the wide update center draws a %d-column line, want <= %d: %q", w, m.Width, line)
		}
	}
}

// TestUpdateCenterFooterNamesTheRunOrTheCount pins the footer's middle line: a
// read in flight says which manager is being read, and a settled list says how
// many entries the managers reported. It is empty before the first read.
func TestUpdateCenterFooterNamesTheRunOrTheCount(t *testing.T) {
	m := updateCenterFrameCase(t)
	if got := m.updateCenterFooterStatus(); got == "" {
		t.Errorf("a checked update center has no footer status")
	}

	m.UpdateCenter.CheckedAt = time.Time{}
	if got := m.updateCenterFooterStatus(); got != "" {
		t.Errorf("an unread update center states %q, want no status", got)
	}
}

// updateCenterLongList frames the update center around a list longer than any
// frame the guards below measure, so the window is real: a list that fits states
// nothing about scrolling.
func updateCenterLongList(t *testing.T, count int) Model {
	t.Helper()

	m := updateCenterFrameCase(t)
	items := make([]updates.Item, 0, count)
	for i := 0; i < count; i++ {
		items = append(items, updates.Item{
			Manager:   updates.ManagerApt,
			Scope:     updates.ScopeSystem,
			Name:      fmt.Sprintf("package-%02d", i),
			Current:   "1.0.0-1",
			Available: "1.0.1-1",
		})
	}
	m.UpdateCenter.Results[0].Items = items
	m.UpdateCenter.Selected = updates.Selection{}
	m.Animating = false
	return m
}

// TestUpdateCenterScrollsInsideItsCard is the guard for the scroll defect: the
// screen rebuilt its whole body for the window it drew, so the notes at the ends
// of the window grew the table's card, the frame drew past the rows it had, and
// the terminal scrolled -- the header left the screen and rows disappeared as the
// cursor walked down. The list scrolls inside its card now: the frame never draws
// more rows than its height, the rows above the window (the header, the stats
// strip, the table's heading) and the footer stay on the rows they started on,
// and only the window's own rows change.
func TestUpdateCenterScrollsInsideItsCard(t *testing.T) {
	base := updateCenterLongList(t, 40)

	// updateCenterAnchorRows is where the screen's fixed rows sit in the frame's
	// own lines: the stats strip's headline, the table's heading, the status rule
	// and the legend. They are found by their words rather than by an index this
	// guard would have to keep true, so a layout that grew a row above them is
	// measured rather than assumed.
	updateCenterAnchorRows := func(t *testing.T, size string, rows []string) map[string]int {
		t.Helper()
		anchors := map[string]int{}
		for _, want := range []string{"updates available", "Package", "● Ready"} {
			for i, row := range rows {
				if strings.Contains(row, want) {
					anchors[want] = i
					break
				}
			}
			if _, ok := anchors[want]; !ok {
				t.Fatalf("at %s the frame does not draw the %q row:\n%s", size, want, strings.Join(rows, "\n"))
			}
		}
		return anchors
	}

	for _, size := range measuredTerminalSizes {
		m := base
		m.Width, m.Height = size.width, size.height
		// The table is the wide shape's; the 80x24 floor keeps the section
		// renderer, whose own window is measured by its own guards. What this
		// guard measures at every size is that the frame stays inside its rows.
		l := layoutFor(m)
		wide := m.updateCenterWide(l, installerBodyRows(m.Height, m.footerReserveRows(l.Inner, m.updateCenterHints())))

		var firstFrame []string
		var firstAnchors map[string]int
		for cursor := range m.GetCurrentOptions() {
			m.Cursor = cursor
			view := m.View()
			assertScreenFitsTerminal(t, "update-center/"+size.name, size.width, size.height, view)

			if !wide {
				continue
			}
			rows := strings.Split(ansiEscape.ReplaceAllString(view, ""), "\n")
			anchors := updateCenterAnchorRows(t, size.name, rows)
			if cursor == 0 {
				firstFrame, firstAnchors = rows, anchors
				continue
			}
			if len(rows) != len(firstFrame) {
				t.Fatalf("at %s cursor %d draws %d rows and cursor 0 draws %d: the frame changes height while the list scrolls",
					size.name, cursor, len(rows), len(firstFrame))
			}
			for anchor, at := range anchors {
				if firstAnchors[anchor] != at {
					t.Errorf("at %s cursor %d puts the %q row on row %d, where cursor 0 put it on row %d: the rows above the window moved while the list scrolled",
						size.name, cursor, anchor, at, firstAnchors[anchor])
				}
			}
			// The footer is the status rule and the legend under it; neither moves,
			// and neither is allowed to change its words while the cursor walks.
			for i := len(rows) - 2; i < len(rows); i++ {
				if i >= 0 && rows[i] != firstFrame[i] {
					t.Errorf("at %s cursor %d changed the footer row %d from %q to %q: the footer must not move while the list scrolls",
						size.name, cursor, i, firstFrame[i], rows[i])
				}
			}
		}
	}
}

// TestUpdateCenterWindowNoteIsTruthful pins the note the window draws: a list with
// rows above the window says how many, a list with rows below says how many, and a
// window showing the first rows claims nothing above it. The notes are the screen's
// own count, so a note that appeared at the top of a list would be a lie about rows
// that are on screen.
func TestUpdateCenterWindowNoteIsTruthful(t *testing.T) {
	m := updateCenterLongList(t, 40)
	m.Width, m.Height = 160, 50
	m.Animating = false

	plain := ansiEscape.ReplaceAllString(m.View(), "")
	if strings.Contains(plain, "more above") {
		t.Errorf("the first rows of the list are on screen and the window still claims rows above it:\n%s", plain)
	}
	if !strings.Contains(plain, "more below") {
		t.Errorf("the window hides the rows below it and does not say so:\n%s", plain)
	}

	last := 0
	for i, row := range m.GetCurrentOptions() {
		if isUpdateCenterItemRow(row) {
			last = i
		}
	}
	m.Cursor = last
	plain = ansiEscape.ReplaceAllString(m.View(), "")
	if !strings.Contains(plain, "more above") {
		t.Errorf("the last row of the list is on screen and the window does not say what it hid above it:\n%s", plain)
	}
	if strings.Contains(plain, "more below") {
		t.Errorf("the last rows of the list are on screen and the window still claims rows below it:\n%s", plain)
	}
}

// updateCenterFrameRows is a rendered frame's lines with the escapes taken off,
// the way the guards below read them.
func updateCenterFrameRows(m Model) []string {
	return strings.Split(ansiEscape.ReplaceAllString(m.View(), ""), "\n")
}

// frameBodyEnds is where the frame's own chrome is: the row of the rule under the
// header and the row of the footer's status line. The body is the rows between
// them, so a guard can measure what the composition does with the room.
func frameBodyEnds(rows []string) (rule, footer int) {
	rule, footer = -1, -1
	for i, row := range rows {
		if rule < 0 && strings.Contains(row, "━━━") {
			rule = i
		}
		if strings.Contains(row, "● Ready") {
			footer = i
		}
	}
	return rule, footer
}

// blankRun is how many blank rows run from `from` in `step`'s direction before the
// first drawn row, stopping at `limit`.
func blankRun(rows []string, from, limit, step int) int {
	count := 0
	for i := from; i != limit; i += step {
		if strings.TrimSpace(rows[i]) != "" {
			return count
		}
		count++
	}
	return count
}

// TestUpdateCenterListCardHugsItsWindow is the guard for the stretched table: the
// card was given every row the stats strip left, so a machine reporting seventy
// packages drew a table from the strip to the footer -- the rows were the frame's,
// not the list's, and the composition had no room left to sit centred. The window
// is a reading measure now, the card is exactly as tall as the rows it draws, and
// the frame's spare rows stay above and below the composition instead of inside
// the card.
func TestUpdateCenterListCardHugsItsWindow(t *testing.T) {
	base := updateCenterLongList(t, 72)

	// The table's card in the left column: the stats strip, a blank row and then
	// the card, so the card's rows are what the column has left over.
	tableCardRows := func(t *testing.T, height int) (card, room int, m Model) {
		t.Helper()
		m = base
		m.Width, m.Height = 160, height
		m.Animating = false
		l := layoutFor(m)
		budget := installerBodyRows(m.Height, m.footerReserveRows(l.Inner, m.updateCenterHints()))
		stats := len(m.updateCenterStatsStrip(l.Left))
		column := m.updateCenterTableColumn(l.Left, budget)
		return len(column) - stats - 1, budget - stats - 1, m
	}

	short, shortRoom, _ := tableCardRows(t, 50)
	tall, tallRoom, tallModel := tableCardRows(t, 80)

	if short != tall {
		t.Errorf("the table's card is %d rows at 160x50 and %d at 160x80: it grows with the frame's rows instead of hugging the window it draws", short, tall)
	}
	for _, tc := range []struct {
		size string
		card int
		room int
	}{{"160x50", short, shortRoom}, {"160x80", tall, tallRoom}} {
		if tc.card > updateTableWindowRows+cardChromeRows+2+2 {
			t.Errorf("at %s the card draws %d rows, want no more than the reading's %d plus its chrome and notes",
				tc.size, tc.card, updateTableWindowRows)
		}
		if tc.card >= tc.room {
			t.Errorf("at %s the card takes the column's whole room (%d of %d rows): the frame's spare rows went into the card",
				tc.size, tc.card, tc.room)
		}
	}

	// The composition is centred: the blank rows above and below it inside the
	// body are the same, give or take the odd row the split cannot halve.
	rows := updateCenterFrameRows(tallModel)
	rule, footer := frameBodyEnds(rows)
	if rule < 0 || footer < 0 {
		t.Fatalf("the frame does not draw its own rule and footer:\n%s", strings.Join(rows, "\n"))
	}
	above := blankRun(rows, rule+1, footer, 1)
	below := blankRun(rows, footer-1, rule, -1)
	if above == 0 {
		t.Errorf("the composition starts on the row under the header's rule, so it is not centred:\n%s", strings.Join(rows, "\n"))
	}
	if diff := above - below; diff < -1 || diff > 1 {
		t.Errorf("the composition keeps %d blank rows above it and %d below: it is not centred between the header and the footer", above, below)
	}

	// And the card is the rows it draws, with nothing padded inside it: with the
	// cursor in the middle of the list both notes carry a count and the scrollbar
	// fills the column, so every row inside the card states something.
	middle := base
	middle.Width, middle.Height = 160, 80
	middle.Animating = false
	middle.Cursor = len(middle.GetCurrentOptions()) / 2
	rows = updateCenterFrameRows(middle)
	rule, footer = frameBodyEnds(rows)
	body := rows[rule+1 : footer]
	column := firstCardColumn(body)
	if column < 0 {
		t.Fatalf("the body draws no card at all:\n%s", strings.Join(rows, "\n"))
	}
	spans := cardSpansInColumn(body, column)
	if len(spans) < 2 {
		t.Fatalf("the left column draws %d cards, want the stats strip and the table:\n%s", len(spans), strings.Join(rows, "\n"))
	}
	last := spans[len(spans)-1]
	card := body[last[0] : last[1]+1]
	for i, row := range card[1 : len(card)-1] {
		if cardInnerBlank(row) {
			t.Errorf("the table's card has a blank row %d inside it, so it is padded rather than hugging its window:\n%s",
				i+1, strings.Join(card, "\n"))
		}
	}
}

// firstCardColumn is the column the body's cards start at: the leading margin the
// frame gives the composition plus the frame's own padding.
func firstCardColumn(rows []string) int {
	for _, row := range rows {
		if at := strings.IndexRune(row, '╭'); at >= 0 {
			return len([]rune(row[:at]))
		}
	}
	return -1
}

// cardSpansInColumn returns the top and bottom row of every card drawn in one
// column of a composed body, in the order they are drawn.
func cardSpansInColumn(rows []string, column int) [][2]int {
	var spans [][2]int
	top := -1
	for i, row := range rows {
		runes := []rune(row)
		if column >= len(runes) {
			continue
		}
		switch runes[column] {
		case '╭':
			top = i
		case '╰':
			if top >= 0 {
				spans = append(spans, [2]int{top, i})
				top = -1
			}
		}
	}
	return spans
}

// cardInnerBlank reports whether a card row draws nothing between its own edges.
func cardInnerBlank(row string) bool {
	inner := strings.TrimSpace(row)
	inner = strings.Trim(inner, "│")
	inner = strings.TrimSpace(inner)
	inner = strings.Trim(inner, "─")
	return strings.TrimSpace(inner) == ""
}

// TestUpdateCenterCursorVisitsOnlyPackageRows is the guard for the cursor that
// rested on rows the detail cannot describe: an action row, the way back, a group
// rule. Space and the action letters act where they stand, so the cursor's only
// job is to say which package is being looked at, and it moves between packages.
func TestUpdateCenterCursorVisitsOnlyPackageRows(t *testing.T) {
	base := updateCenterFrameCase(t)
	base.Width, base.Height = 160, 50
	base.Animating = false

	next := func(m Model, key tea.KeyMsg) Model {
		updated, _ := m.Update(key)
		return updated.(Model)
	}
	down := func(m Model) Model { return next(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}) }
	up := func(m Model) Model { return next(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")}) }

	m := base
	m.Cursor = 0
	seen := map[string]bool{}
	for steps := 0; steps < len(m.GetCurrentOptions())+2; steps++ {
		options := m.GetCurrentOptions()
		if m.Cursor < 0 || m.Cursor >= len(options) {
			t.Fatalf("the cursor is %d with %d rows on the screen", m.Cursor, len(options))
		}
		row := options[m.Cursor]
		if !isUpdateCenterItemRow(row) {
			t.Fatalf("walking down lands the cursor on %q, which is not a package", row)
		}
		seen[row] = true
		if plain := ansiEscape.ReplaceAllString(m.View(), ""); !strings.Contains(plain, "Read with") {
			t.Errorf("with the cursor on %q the detail card is gone:\n%s", row, plain)
		}
		m = down(m)
	}
	if got, want := len(seen), len(m.updateCenterItems()); got != want {
		t.Errorf("walking down reaches %d of the %d packages", got, want)
	}
	if row := m.GetCurrentOptions()[m.Cursor]; !isUpdateCenterItemRow(row) {
		t.Errorf("walking past the last package parks the cursor on %q", row)
	}

	last := m.GetCurrentOptions()[m.Cursor]
	for steps := 0; steps < len(m.GetCurrentOptions())+2; steps++ {
		m = up(m)
	}
	if row := m.GetCurrentOptions()[m.Cursor]; row != base.GetCurrentOptions()[0] {
		t.Errorf("walking up from %q stops on %q, want the first package %q", last, row, base.GetCurrentOptions()[0])
	}
}

// TestUpdateCenterDetailFallsBackToTheNearestPackage is the guard for the second
// half of the same defect: a cursor the screen puts on a row that is not a package
// -- a filter that shortened the list, a selection that added an action row above
// the way back -- used to blank the whole detail column and take the composition
// with it. The card now speaks for the nearest package instead.
func TestUpdateCenterDetailFallsBackToTheNearestPackage(t *testing.T) {
	base := updateCenterLongList(t, 12)
	base.Width, base.Height = 160, 50
	base.Animating = false

	options := base.GetCurrentOptions()
	lastItem, lastItemAt := "", -1
	actionAt := -1
	for i, row := range options {
		if isUpdateCenterItemRow(row) {
			lastItem, lastItemAt = row, i
		}
		if actionAt < 0 && strings.HasPrefix(row, updateCenterCheckRow) {
			actionAt = i
		}
	}
	if lastItemAt < 0 || actionAt < 0 {
		t.Fatalf("the fixture offers no package or no action row: %v", options)
	}

	m := base
	m.Cursor = actionAt
	item, ok := m.updateCenterDetailItem()
	if !ok {
		t.Fatalf("the cursor on the action row %q has no entry to describe", options[actionAt])
	}
	if got := updateCenterItemRow(item, false, m.updateCenterRowMeasure()); got != lastItem {
		t.Errorf("the detail falls back to %q, want the nearest package %q", got, lastItem)
	}
	plain := ansiEscape.ReplaceAllString(m.View(), "")
	if !strings.Contains(plain, "Read with") {
		t.Errorf("the detail card is gone with the cursor on an action row:\n%s", plain)
	}
	if !strings.Contains(plain, "package-11") {
		t.Errorf("the detail card does not name the nearest package:\n%s", plain)
	}

	// The row under the cursor is the package the card describes: the list keeps
	// its filled row, so the panel and the bar cannot disagree.
	if !strings.Contains(plain, "package-11        1.0.0-1 → 1.0.1-1") {
		t.Errorf("the nearest package is not drawn as a row of its own:\n%s", plain)
	}
}

// TestUpdateScrollbarFollowsTheWindowProportionally is the guard for the crooked
// scrollbar: the thumb was placed around the middle of the visible window wherever
// the window sat in the list, so it said a window at the top was half way down.
// The track stands for the whole list, so the slice's share of the list is its
// share of the track: the thumb begins at the slice's first row and is as long as
// the slice.
func TestUpdateScrollbarFollowsTheWindowProportionally(t *testing.T) {
	const total, window = 100, 10

	filledCells := func(start int) []int {
		var filled []int
		end := start + window
		for index := start; index < end; index++ {
			if strings.Contains(updateScrollbarCell(index, start, end, total), "█") {
				filled = append(filled, index-start)
			}
		}
		return filled
	}

	for _, tc := range []struct {
		name  string
		start int
		want  []int
	}{
		// A tenth of the list is a tenth of the track: one cell, at the top when
		// the window is at the top and at the foot when it is at the end.
		{"first rows", 0, []int{0}},
		{"middle", 45, []int{4}},
		{"last rows", total - window, []int{window - 1}},
	} {
		got := filledCells(tc.start)
		if len(got) != len(tc.want) {
			t.Fatalf("%s: the window at row %d fills %v, want %v", tc.name, tc.start, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("%s: the window at row %d fills %v, want %v", tc.name, tc.start, got, tc.want)
			}
		}
	}

	// A window that is half the list fills half the track, and the whole list
	// fills all of it: the thumb's length is the slice's share.
	half := 0
	for index := 0; index < 10; index++ {
		if strings.Contains(updateScrollbarCell(index, 0, 10, 20), "█") {
			half++
		}
	}
	if half != 5 {
		t.Errorf("a window of half the list fills %d of 10 cells, want 5", half)
	}
	whole := 0
	for index := 0; index < 4; index++ {
		if strings.Contains(updateScrollbarCell(index, 0, 4, 4), "█") {
			whole++
		}
	}
	if whole != 4 {
		t.Errorf("a window holding the whole list fills %d of 4 cells, want 4", whole)
	}

	// And the thumb travels down as the window walks the list, one place per slice.
	previous := -1
	for start := 0; start <= total-window; start += 7 {
		cells := filledCells(start)
		if len(cells) == 0 {
			t.Fatalf("the window at row %d fills no cell at all", start)
		}
		if cells[0] < previous {
			t.Fatalf("the thumb moves up from %d to %d as the window walks down", previous, cells[0])
		}
		previous = cells[0]
	}
}
