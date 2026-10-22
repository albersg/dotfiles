package tui

import (
	"strings"
	"testing"
)

// TestUpdateCenterCompositionHoldsStillWhileTheCursorWalks is the guard the four
// reports meet on: the screen is one composition, and walking the list may change
// the row that is filled and the rows the window shows, never where the cards sit.
// It is measured on the frame rather than from the model, so a card that grew, a
// panel that went blank or a detail that padded itself shows up as a moved row.
func TestUpdateCenterCompositionHoldsStillWhileTheCursorWalks(t *testing.T) {
	base := updateCenterLongList(t, 30)
	base.Width, base.Height = 160, 50
	base.Animating = false

	type shape struct {
		top, bottom int      // the first and last drawn row of the composition
		left, right [][2]int // the cards of each column, in the order drawn
	}

	measure := func(m Model) shape {
		t.Helper()
		rows := strings.Split(ansiEscape.ReplaceAllString(m.View(), ""), "\n")
		rule, footer := frameBodyEnds(rows)
		if rule < 0 || footer < 0 {
			t.Fatalf("the frame does not draw its own rule and footer:\n%s", strings.Join(rows, "\n"))
		}
		body := rows[rule+1 : footer]
		columns := cardColumns(body)
		if len(columns) != 2 {
			t.Fatalf("the wide update center draws %d card columns, want the table and the detail beside it:\n%s",
				len(columns), strings.Join(rows, "\n"))
		}
		left := cardSpansInColumn(body, columns[0])
		right := cardSpansInColumn(body, columns[1])
		if len(left) < 2 || len(right) < 2 {
			t.Fatalf("the update center draws %d cards at the table and %d at the detail, want two on each side:\n%s",
				len(left), len(right), strings.Join(rows, "\n"))
		}
		top, bottom := -1, -1
		for i, row := range body {
			if strings.TrimSpace(row) == "" {
				continue
			}
			if top < 0 {
				top = i
			}
			bottom = i
		}
		return shape{top: top, bottom: bottom, left: left, right: right}
	}

	want := measure(base)

	for cursor := range base.GetCurrentOptions() {
		moved := base
		moved.Cursor = cursor
		got := measure(moved)
		if got.top != want.top || got.bottom != want.bottom {
			t.Errorf("with the cursor on row %d the composition spans rows %d-%d, where row 0 put it on %d-%d: the composition moves as the cursor walks",
				cursor, got.top, got.bottom, want.top, want.bottom)
		}
		if got.left[0][0] != want.left[0][0] || got.left[len(got.left)-1][1] != want.left[len(want.left)-1][1] {
			t.Errorf("with the cursor on row %d the table's column spans rows %d-%d, where row 0 put it on %d-%d",
				cursor, got.left[0][0], got.left[len(got.left)-1][1], want.left[0][0], want.left[len(want.left)-1][1])
		}
		if got.right[0][0] != want.right[0][0] || got.right[len(got.right)-1][1] != want.right[len(want.right)-1][1] {
			t.Errorf("with the cursor on row %d the detail's column spans rows %d-%d, where row 0 put it on %d-%d: the panel the cursor is past is not the one the screen drew",
				cursor, got.right[0][0], got.right[len(got.right)-1][1], want.right[0][0], want.right[len(want.right)-1][1])
		}
	}
}
