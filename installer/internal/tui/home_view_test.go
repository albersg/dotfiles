package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/golden"
)

// The home screen's guards. The wide home is the reference's two columns
// (home_view.go); these guards cover what a snapshot cannot: that Tab does the
// one thing the screen promises, and that the composition's margins, gutters and
// rhythm are one set of measures rather than each block's own.

// homeModel is the home screen at a size with every input a snapshot or a
// measurement would otherwise inherit from the running machine pinned.
func homeModel(t *testing.T, width, height int) Model {
	t.Helper()

	m := NewModel()
	isolateGoldenTest(t, &m)
	m.Width, m.Height = width, height
	m.Screen = ScreenWelcome
	m.PanelIndex = 0
	m.Metrics = pinnedMetrics(24)
	m.Animating = true
	m.AnimTick = 0
	return m
}

// homeView is the home screen with its escape sequences removed, so a guard can
// read the words and measure the geometry.
func homeView(m Model) string {
	return ansiEscape.ReplaceAllString(m.View(), "")
}

// homeLines is the home screen as rows, escape sequences removed.
func homeLines(m Model) []string {
	return strings.Split(homeView(m), "\n")
}

// homeBodyTop is the index of the composition's first row in homeLines: the
// frame's own top padding, the header's row and the rule under it.
func homeBodyTop() int {
	return viewPaddingRows + 2
}

// homeColumnSpan is one column of the composition, in columns of a rendered row.
type homeColumnSpan struct {
	from, to int
}

// homeColumnSpans reads the two columns' spans from the layout the screen is
// composed with, so the guard measures the columns the renderer really draws in
// rather than restating its arithmetic.
func homeColumnSpans(m Model) (left, right homeColumnSpan) {
	l := layoutFor(m)
	from := viewPaddingCols + l.Leading
	return homeColumnSpan{from, from + l.Left}, homeColumnSpan{from + l.Left + layoutGutter, from + l.Inner}
}

// cut is the line's own columns of the span, as runes: the frame's rows are one
// cell per rune.
func (s homeColumnSpan) cut(line string) string {
	runes := []rune(line)
	if s.from >= len(runes) {
		return ""
	}
	return string(runes[s.from:min(s.to, len(runes))])
}

// drawn is the body rows of the span that carry something, counted from the
// body's first row.
func (s homeColumnSpan) drawn(lines []string) []int {
	var out []int
	for i, line := range lines {
		if i < homeBodyTop() {
			continue
		}
		if strings.TrimSpace(s.cut(line)) != "" {
			out = append(out, i-homeBodyTop())
		}
	}
	return out
}

// gaps is the blank runs between the span's blocks, read over the rows the
// composition may use. A block's own rows are adjacent, so only a run of blank
// rows separates two of them; the run above the column's first block is its top
// margin and the run under its last block is the frame's tail, and neither is a
// gutter.
func (s homeColumnSpan) gaps(lines []string, rows int) []int {
	var out []int
	run := 0
	drawn := false
	for i := homeBodyTop(); i < min(homeBodyTop()+rows, len(lines)); i++ {
		if strings.TrimSpace(s.cut(lines[i])) == "" {
			run++
			continue
		}
		if drawn && run > 0 {
			out = append(out, run)
		}
		run = 0
		drawn = true
	}
	return out
}

// TestWelcomeTabOpensTheMenuAndNeverWalksPanels pins the one key the screen's own
// body and footer promise: "[Tab] menu". The welcome screen offers panels, so the
// frame's panel walk took Tab before the screen's own dispatch ever saw it, and
// the key drew nothing at all where the panel is not drawn -- the screen
// advertised a press that did nothing. The promise is the truth: Tab opens the
// main menu on the welcome screen at every size, and the panel index is left
// exactly where it was.
func TestWelcomeTabOpensTheMenuAndNeverWalksPanels(t *testing.T) {
	for _, size := range []struct{ width, height int }{{200, 55}, {160, 50}, {160, 40}, {80, 24}} {
		t.Run(sizeName(size.width, size.height), func(t *testing.T) {
			m := homeModel(t, size.width, size.height)
			if !strings.Contains(homeView(m), "[Tab] menu") {
				t.Fatalf("the home screen does not offer [Tab] menu:\n%s", homeView(m))
			}

			next, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
			got, ok := next.(Model)
			if !ok {
				t.Fatalf("Update returned %T, want Model", next)
			}
			if got.Screen != ScreenMainMenu {
				t.Errorf("Tab left the home screen on %v, want the main menu: the screen promises [Tab] menu", got.Screen)
			}
			if got.Cursor != 0 {
				t.Errorf("Tab left the main menu's cursor at %d, want 0", got.Cursor)
			}
			if got.PanelIndex != 0 {
				t.Errorf("Tab walked the panels to %d as well as opening the menu, want 0", got.PanelIndex)
			}
		})
	}
}

// TestWelcomeStartAndQuitStillDoWhatTheyDid pins the rest of the screen's keys:
// the fix for Tab moved nothing else. [s] opens the menu and [q] quits.
func TestWelcomeStartAndQuitStillDoWhatTheyDid(t *testing.T) {
	m := homeModel(t, 160, 50)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	if got := next.(Model); got.Screen != ScreenMainMenu {
		t.Errorf("s left the home screen on %v, want the main menu", got.Screen)
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("q returned no command on the home screen, want tea.Quit")
	}
}

// TestHomeWideCentresOneBlockBetweenTheHeaderAndThePet pins the first thing the
// screen's vertical placement promises now: the two columns are ONE block, both
// opening on the same row (homeTopMargin) and neither against the rule, and that
// block is centred in the rows between the header and the creature's reserved
// block. The composition used to be anchored to the rule with every row it did not
// need spent under the cards, which reads as a void under the screen rather than a
// composition.
func TestHomeWideCentresOneBlockBetweenTheHeaderAndThePet(t *testing.T) {
	for _, size := range []struct{ width, height int }{{200, 55}, {160, 50}, {160, 40}} {
		t.Run(sizeName(size.width, size.height), func(t *testing.T) {
			m := homeModel(t, size.width, size.height)
			if !m.homeWide(layoutFor(m)) {
				t.Fatalf("the home screen at %s is not the wide shape, so this guard measures nothing", sizeName(size.width, size.height))
			}
			lines := homeLines(m)
			left, right := homeColumnSpans(m)
			rows := homeCompositionRows(m)
			// Only the rows above the creature's reserved block are the
			// composition's: the pet is drawn in the tail the block did not take.
			block := lines[:min(homeBodyTop()+rows, len(lines))]

			leftRows, rightRows := left.drawn(block), right.drawn(block)
			if len(leftRows) == 0 || len(rightRows) == 0 {
				t.Fatalf("a column draws nothing at %s, so this guard measures nothing:\n%s", sizeName(size.width, size.height), homeView(m))
			}
			if leftRows[0] != rightRows[0] {
				t.Errorf("the columns start %d and %d rows into the body, want one shared top row", leftRows[0], rightRows[0])
			}

			top := min(leftRows[0], rightRows[0])
			bottom := max(leftRows[len(leftRows)-1], rightRows[len(rightRows)-1])
			above := top
			below := rows - 1 - bottom
			if above < homeTopMargin {
				t.Errorf("the composition starts %d rows under the rule, want at least its own top margin of %d", above, homeTopMargin)
			}
			if diff := above - below; diff > 1 || diff < -1 {
				t.Errorf("the block sits %d rows under the rule and %d rows above its tail, want it centred (a difference of at most one row):\n%s",
					above, below, homeView(m))
			}
		})
	}
}

// TestHomeWideLetsNothingMoveTheBlock pins the side of centring that a snapshot
// cannot: the block's row is a function of the terminal and of the screen's own
// data, never of a clock, a tick or the cursor. Centring is only safe while the
// block keeps its height, so the same model rendered again -- and the same model
// with its animation stepped -- must put every block on the same row.
func TestHomeWideLetsNothingMoveTheBlock(t *testing.T) {
	for _, size := range []struct{ width, height int }{{200, 55}, {160, 50}, {160, 40}} {
		t.Run(sizeName(size.width, size.height), func(t *testing.T) {
			m := homeModel(t, size.width, size.height)
			before := homeLines(m)

			next, _ := m.Update(animTickMsg{})
			stepped, ok := next.(Model)
			if !ok {
				t.Fatalf("Update returned %T, want Model", next)
			}
			after := homeLines(stepped)

			if len(before) != len(after) {
				t.Fatalf("the frame is %d rows and %d rows after a tick, want one height", len(before), len(after))
			}
			left, right := homeColumnSpans(m)
			for name, span := range map[string]homeColumnSpan{"left": left, "right": right} {
				first, then := span.drawn(before), span.drawn(after)
				if len(first) == 0 || len(then) == 0 {
					t.Fatalf("the %s column draws nothing, so this guard measures nothing", name)
				}
				if first[0] != then[0] {
					t.Errorf("the %s column's block starts on row %d and on row %d after one tick, want it standing still",
						name, first[0], then[0])
				}
			}
		})
	}
}

// TestHomeWideSeparatesEveryBlockByOneGutter pins the second measure: between any
// two blocks of either column there is exactly homeBlockGap blank row, so the
// screen has one rhythm instead of a gap per block. The columns' own gutter is
// layoutGutter, which composeColumns spends for every two-column screen; this
// guard reads it back off the rendering.
func TestHomeWideSeparatesEveryBlockByOneGutter(t *testing.T) {
	for _, size := range []struct{ width, height int }{{200, 55}, {160, 50}, {160, 40}} {
		t.Run(sizeName(size.width, size.height), func(t *testing.T) {
			m := homeModel(t, size.width, size.height)
			lines := homeLines(m)
			left, right := homeColumnSpans(m)

			rows := homeCompositionRows(m)
			for at, column := range []homeColumnSpan{left, right} {
				gaps := column.gaps(lines, rows)
				if len(gaps) == 0 {
					t.Fatalf("column %d draws no gap between two blocks, so this guard measures nothing:\n%s", at, homeView(m))
				}
				for _, gap := range gaps {
					if gap != homeBlockGap {
						t.Errorf("column %d has a %d-row gap between two blocks, want %d: %v",
							at, gap, homeBlockGap, gaps)
					}
				}
			}

			// The two columns are layoutGutter columns apart on every row.
			if got := right.from - left.to; got != layoutGutter {
				t.Errorf("the columns are %d columns apart, want the layout's gutter %d", got, layoutGutter)
			}
		})
	}
}

// TestHomeWideChipsSitOnTheTextsLeftEdge pins the third measure: the machine
// chips' border starts in the column where the tagline, the greeting and the call
// to action start. The screen used to centre each line of text over a flush-left
// band of chips, so the left column had two left edges and neither looked meant.
func TestHomeWideChipsSitOnTheTextsLeftEdge(t *testing.T) {
	for _, size := range []struct{ width, height int }{{200, 55}, {160, 50}, {160, 40}} {
		t.Run(sizeName(size.width, size.height), func(t *testing.T) {
			m := homeModel(t, size.width, size.height)
			lines := homeLines(m)

			chipRow, ok := rowHolding(lines, "╭", "OS")
			if !ok {
				t.Fatalf("the home screen draws no chip row:\n%s", homeView(m))
			}
			chips, ok := leftEdge(lines[chipRow])
			if !ok {
				t.Fatalf("the chip row starts nowhere: %q", lines[chipRow])
			}
			// The call to action is left out on purpose: it is a filled block that
			// carries its own padding, so its mark sits one column inside the fill
			// while every word on the screen starts on the edge.
			for _, marker := range []string{
				homeTagline,
				homeTaglineWarm,
				"Good morning, testuser!",
				"Running on",
				homeExploreHint,
			} {
				row, ok := rowHolding(lines, marker)
				if !ok {
					t.Fatalf("the home screen does not draw %q:\n%s", marker, homeView(m))
				}
				if at, _ := leftEdge(lines[row]); at != chips {
					t.Errorf("%q starts at column %d, want the chips' column %d", marker, at, chips)
				}
			}
		})
	}
}

// TestHomeWideFitsItsFrameAndKeepsThePet pins the frame's own budget at the two
// sizes the composition is read at: the screen is exactly as tall as the
// terminal, no row runs past its width, nothing is cut, and the creature keeps
// the rows it is reserved at the bottom.
func TestHomeWideFitsItsFrameAndKeepsThePet(t *testing.T) {
	for _, size := range []struct{ width, height int }{{200, 55}, {160, 50}, {160, 40}, {160, 35}, {80, 24}} {
		t.Run(sizeName(size.width, size.height), func(t *testing.T) {
			m := homeModel(t, size.width, size.height)
			view := homeView(m)
			if rows := renderedRowCount(view); rows != size.height {
				t.Errorf("the home screen renders %d rows at %s, want exactly %d:\n%s", rows, sizeName(size.width, size.height), size.height, view)
			}
			for _, line := range strings.Split(view, "\n") {
				if w := lipgloss.Width(line); w > size.width {
					t.Errorf("a home row is %d columns wide, want <= %d: %q", w, size.width, line)
				}
				if strings.Contains(line, cutMarker) {
					t.Errorf("a home row is cut with %q: %q", cutMarker, line)
				}
			}
			for _, want := range []string{"● Ready", "[s] start", "[q] quit", "[Tab] menu"} {
				if !strings.Contains(view, want) {
					t.Errorf("the home screen at %s does not draw %q:\n%s", sizeName(size.width, size.height), want, view)
				}
			}
			if m.homeWide(layoutFor(m)) {
				// The wide home reserves the creature's block before it measures either
				// column, so the pet is the frame's tail on every frame that holds it.
				if !strings.Contains(view, petArtRow) {
					t.Errorf("the wide home at %s lost the creature's block:\n%s", sizeName(size.width, size.height), view)
				}
			}
		})
	}
}

// TestHomeWideGolden pins the wide home at 200x55: the composition at the
// terminal size the reference's own screen is read at, where the composition cap
// leaves symmetric margins on both sides.
func TestHomeWideGolden(t *testing.T) {
	skipIfTermux(t)
	m := homeModel(t, 200, 55)
	golden.RequireEqual(t, []byte(m.View()))
}

// TestHomeStandardGolden pins the wide home at 160x50: the same composition with
// no margin left over, which is the size the rest of the frames are read at.
func TestHomeStandardGolden(t *testing.T) {
	skipIfTermux(t)
	m := homeModel(t, 160, 50)
	golden.RequireEqual(t, []byte(m.View()))
}

// rowHolding is the first row holding every marker.
func rowHolding(lines []string, markers ...string) (int, bool) {
	for i, line := range lines {
		held := true
		for _, marker := range markers {
			if !strings.Contains(line, marker) {
				held = false
				break
			}
		}
		if held {
			return i, true
		}
	}
	return 0, false
}

// leftEdge is the column a row's first drawn character sits in.
func leftEdge(line string) (int, bool) {
	for at, r := range []rune(line) {
		if r != ' ' {
			return at, true
		}
	}
	return 0, false
}

// petArtRow is the first row of the compact creature's art: the mark that says the
// pet's reserved block was drawn rather than spent by a column.
const petArtRow = "/\\______/\\"

// homeCompositionRows is the rows the composition may use on this frame: the
// body's rows less the creature's reserved block.
func homeCompositionRows(m Model) int {
	budget := installerBodyRows(m.Height, m.footerReserveRows(layoutFor(m).Inner, m.welcomeHints()))
	rows := budget - m.companionBlockRows(layoutFor(m).Inner)
	if rows < 1 {
		rows = budget
	}
	return rows
}

// sizeName is the width x height label a size-parameterised guard runs under.
func sizeName(width, height int) string {
	return itoa(width) + "x" + itoa(height)
}
