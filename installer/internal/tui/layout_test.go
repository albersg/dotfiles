package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// TestLayoutForSpendsTheColumnsItHas pins the columns contract at the sizes the
// design was settled on: the 80x24 floor, the two-column threshold with the
// widths on either side of it, and the wide pane this feature exists for. Each
// row states the content width -- what the layout is a function of -- and the
// test derives the terminal width from it, so it reads the same path a render
// does instead of a second copy of the arithmetic.
func TestLayoutForSpendsTheColumnsItHas(t *testing.T) {
	cases := []struct {
		name        string
		inner       int
		twoColumn   bool
		composition int
		leading     int
		left        int
		right       int
		rowMeasure  int
	}{
		{"the 80x24 floor", 76, false, 76, 0, 76, 0, 76},
		{"78 columns, the floor as the design table states it", 78, false, 78, 0, 78, 0, 78},
		{"118 columns, one short of two", 118, false, 118, 0, 118, 0, 80},
		{"119 columns, still one short", 119, false, 119, 0, 119, 0, 80},
		{"120 columns, the two-column floor", 120, true, 120, 0, 56, 62, 56},
		{"121 columns, just into two", 121, true, 121, 0, 56, 63, 56},
		{"158 columns, the 160x50 terminal", 158, true, 158, 0, 71, 85, 71},
		{"160 columns, the composition cap", 160, true, 160, 0, 72, 86, 72},
		{"223 columns, the 227x62 terminal", 223, true, 160, 31, 72, 86, 72},
		{"225 columns", 225, true, 160, 32, 72, 86, 72},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			// The frame spends two columns of padding on each side of every
			// screen, so a terminal width is its content width plus four.
			m := Model{Width: c.inner + 4}
			if got := contentWidth(m); got != c.inner {
				t.Fatalf("the case is mis-stated: terminal %d gives content width %d, want %d",
					m.Width, got, c.inner)
			}

			l := layoutFor(m)
			if l.Inner != c.inner {
				t.Errorf("Inner = %d, want %d", l.Inner, c.inner)
			}
			if l.TwoColumn != c.twoColumn {
				t.Errorf("TwoColumn = %v, want %v", l.TwoColumn, c.twoColumn)
			}
			if l.Composition != c.composition {
				t.Errorf("Composition = %d, want %d", l.Composition, c.composition)
			}
			if l.Leading != c.leading {
				t.Errorf("Leading = %d, want %d", l.Leading, c.leading)
			}
			if l.Left != c.left {
				t.Errorf("Left = %d, want %d", l.Left, c.left)
			}
			if l.Right != c.right {
				t.Errorf("Right = %d, want %d", l.Right, c.right)
			}
			if l.RowMeasure != c.rowMeasure {
				t.Errorf("RowMeasure = %d, want %d", l.RowMeasure, c.rowMeasure)
			}
		})
	}
}

// TestLayoutForKeepsItsInvariantsAtEveryWidth is the guard around the table:
// the table pins the sizes that were decided, and this pins the arithmetic that
// has to hold at every size between them, including the ones nobody measured.
// The invariants are the ones the frame relies on to place a body, centre a
// composition and size a row: a column count smaller than the room, columns and
// gutter filling the composition exactly, symmetric margins, a row measure the
// terminal can hold, and a right column that exists whenever the layout says
// there is room to compose one.
func TestLayoutForKeepsItsInvariantsAtEveryWidth(t *testing.T) {
	const gutter = 2

	for width := 0; width <= 400; width++ {
		m := Model{Width: width}
		l := layoutFor(m)

		if l.Inner != contentWidth(m) {
			t.Fatalf("width %d: Inner = %d, want contentWidth %d", width, l.Inner, contentWidth(m))
		}
		if l.Composition != min(l.Inner, layoutMaxComposition) {
			t.Fatalf("width %d: Composition = %d, want min(%d, %d)", width, l.Composition, l.Inner, layoutMaxComposition)
		}
		if l.Composition > l.Inner {
			t.Fatalf("width %d: Composition %d does not fit in Inner %d", width, l.Composition, l.Inner)
		}
		if l.Leading != (l.Inner-l.Composition)/2 {
			t.Fatalf("width %d: Leading = %d, want %d", width, l.Leading, (l.Inner-l.Composition)/2)
		}
		if l.Inner-2*l.Leading-l.Composition > 1 {
			t.Fatalf("width %d: margins are not symmetric: Inner %d, Leading %d, Composition %d",
				width, l.Inner, l.Leading, l.Composition)
		}
		if l.TwoColumn != (l.Inner >= layoutTwoColumnWidth) {
			t.Fatalf("width %d: TwoColumn = %v at Inner %d", width, l.TwoColumn, l.Inner)
		}

		if l.TwoColumn {
			if l.Left+l.Right+gutter != l.Composition {
				t.Fatalf("width %d: Left %d + gutter %d + Right %d = %d, want Composition %d",
					width, l.Left, gutter, l.Right, l.Left+gutter+l.Right, l.Composition)
			}
			if l.Right < 1 {
				t.Fatalf("width %d: a two-column layout with no right column (%d)", width, l.Right)
			}
			if l.Left < layoutLeftMin || l.Left > layoutLeftMax {
				t.Fatalf("width %d: Left = %d, want between %d and %d", width, l.Left, layoutLeftMin, layoutLeftMax)
			}
			if l.RowMeasure != l.Left {
				t.Fatalf("width %d: RowMeasure = %d, want the left column %d", width, l.RowMeasure, l.Left)
			}
		} else {
			if l.Right != 0 {
				t.Fatalf("width %d: a one-column layout has a right column (%d)", width, l.Right)
			}
			if l.Left != l.Composition {
				t.Fatalf("width %d: Left = %d, want the whole composition %d", width, l.Left, l.Composition)
			}
			if l.RowMeasure != min(l.Inner, layoutRowMeasureMax) {
				t.Fatalf("width %d: RowMeasure = %d, want min(%d, %d)", width, l.RowMeasure, l.Inner, layoutRowMeasureMax)
			}
		}

		if l.RowMeasure > l.Inner || l.RowMeasure > layoutRowMeasureMax {
			t.Fatalf("width %d: RowMeasure = %d does not fit the room (%d) and the cap (%d)",
				width, l.RowMeasure, l.Inner, layoutRowMeasureMax)
		}
	}
}

// TestMenuRowsUseTheRowMeasure pins the other half of the row measure: the bar
// and the trailing meter of a row end at it, and the rule that groups the rows
// is drawn at the same width, so a menu reads as one object instead of a row bar
// under a rule that belongs to a wider frame. At the 80x24 floor the measure is
// the content width, which is why the rows there are byte-for-byte what they
// were and the existing goldens do not move.
func TestMenuRowsUseTheRowMeasure(t *testing.T) {
	for _, inner := range []int{76, 118, 223} {
		m := Model{Width: inner + 4}
		l := layoutFor(m)

		rows := m.menuRows([]string{"Install", m.menuSeparator(), "Quit"}, 0)
		if len(rows) != 3 {
			t.Fatalf("inner %d: menuRows returned %d rows, want 3", inner, len(rows))
		}
		for i, row := range rows {
			if got := lipgloss.Width(row); got != l.RowMeasure {
				t.Errorf("inner %d: menu row %d is %d columns, want the row measure %d", inner, i, got, l.RowMeasure)
			}
		}

		// A row that carries a trailing meter is the measure wide too, so the
		// meter sits at the measure's right edge instead of hanging past it.
		withMeter := m.rowBar("Lesson one", false, meterCellsPlain(0.5, 6))
		if got := lipgloss.Width(withMeter); got != l.RowMeasure {
			t.Errorf("inner %d: a row with a trailing meter is %d columns, want the row measure %d", inner, got, l.RowMeasure)
		}

		if inner == 76 && l.RowMeasure != contentWidth(m) {
			t.Errorf("the row measure at the 80x24 floor is %d, want the content width %d", l.RowMeasure, contentWidth(m))
		}
	}
}
