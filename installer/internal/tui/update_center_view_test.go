package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

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
