package tui

import (
	"strings"
	"testing"
)

// ============================================================================
// THE SECTION SCREENS' CARD GRAMMAR — the guards
// ============================================================================
//
// The section screens draw in the reference grammar on a frame that holds two
// columns of cards (section_wide.go). The guards below hold the four things that
// makes true: the cards are the shared ones, the rows are still the screen's own
// words and keys, the screen keeps its way back whatever the width, and the shape
// is a decision about the frame and never about the cursor.

// sectionWideCase is one section screen the grammar is measured on: a list that
// opens onto a finding, a list of checks, a report whose prose is the screen and
// the disk cleaner's long-form report.
type sectionWideCase struct {
	name      string
	build     func(*testing.T) Model
	listTitle string
}

func sectionWideCases(t *testing.T) []sectionWideCase {
	t.Helper()
	return []sectionWideCase{
		{"doctor", doctorFrameCase, "Development environment"},
		{"network-diagnostics", networkDiagnosticsFrameCase, "Network diagnostics"},
		{"disk-cleaner", diskCleanerFrameCase, "Development caches"},
		{"terminal-capabilities", terminalCapabilitiesFrameCase, "Terminal capabilities"},
		{"install-verify", installVerifyFrameCase, "The last install"},
		{"wsl-resources", wslResourcesFrameCase, "WSL resources"},
	}
}

// TestSectionWideDrawsTheGrammarsCards is the shape guard: on a frame that holds
// two columns, the screen's own title is a filled chip set into a card's top
// border with a rule under it, and the prose is in a card of its own; at the
// 80x24 floor the section renderer's bands are drawn and no card is.
func TestSectionWideDrawsTheGrammarsCards(t *testing.T) {
	for _, c := range sectionWideCases(t) {
		c := c
		t.Run(c.name, func(t *testing.T) {
			m := c.build(t)
			m.Width, m.Height = 160, 50
			m.Animating = false

			plain := ansiEscape.ReplaceAllString(m.View(), "")
			if !sectionCardDrawn(m, c.listTitle) {
				t.Errorf("%s does not carry its own title as the card's chip at 160x50:\n%s", c.name, plain)
			}
			if !strings.Contains(plain, "╰") {
				t.Errorf("%s draws no closing card border:\n%s", c.name, plain)
			}

			// The floor keeps the section renderer: no card is drawn there, so the
			// two shapes are told apart by the chrome and not by guesswork.
			floor := c.build(t)
			floor.Width, floor.Height = 80, 24
			floor.Animating = false
			if floorPlain := ansiEscape.ReplaceAllString(floor.View(), ""); sectionCardDrawn(floor, c.listTitle) {
				t.Errorf("%s draws the wide card at the 80x24 floor:\n%s", c.name, floorPlain)
			}
		})
	}
}

// sectionCardDrawn reports whether the screen drew the wide shape: its own title
// as the chip set into a card's top border. It is read from the frame rather than
// from the gate, so the guard measures what the user sees.
func sectionCardDrawn(m Model, title string) bool {
	plain := ansiEscape.ReplaceAllString(m.View(), "")
	return strings.Contains(plain, "╭─ "+title+" ") || strings.Contains(plain, "╭─"+title+" ")
}

// TestSectionWideKeepsEveryRowAndItsWayBack is the reachability guard: on the
// wide frame every row the screen offers is drawn whole or counted in the note
// that says how many the window hid, and the way back is on the screen whichever
// shape it took.
func TestSectionWideKeepsEveryRowAndItsWayBack(t *testing.T) {
	for _, c := range sectionWideCases(t) {
		c := c
		t.Run(c.name, func(t *testing.T) {
			m := c.build(t)
			m.Width, m.Height = 160, 50
			m.Animating = false

			options := m.GetCurrentOptions()
			if len(options) == 0 {
				t.Fatalf("%s offers no rows", c.name)
			}
			// The way back is a row of every section screen; the wide shapes must not
			// be the one place it disappears.
			foundBack := false
			for _, option := range options {
				if strings.HasPrefix(option, "← ") {
					foundBack = true
					break
				}
			}
			// The way back carries the arrow in its gutter, or the cursor's marker when
			// the cursor is on it -- the one row a report screen offers -- so the words
			// are what the guard reads.
			plain := ansiEscape.ReplaceAllString(m.View(), "")
			if !foundBack || (!strings.Contains(plain, "← Back") && !strings.Contains(plain, grammarMarker+" Back")) {
				t.Errorf("%s does not draw its way back on the wide frame", c.name)
			}

			// Every row the cursor can reach stays reachable: the row under the cursor
			// is the one carrying the marker, and the header names the window when the
			// list is cut.
			for cursor := 0; cursor < len(options); cursor++ {
				if strings.HasPrefix(options[cursor], menuSeparatorPrefix) {
					continue
				}
				moved := m
				moved.Cursor = cursor
				plain := ansiEscape.ReplaceAllString(moved.View(), "")
				if !strings.Contains(plain, grammarMarker) {
					t.Fatalf("%s draws no cursor marker with the cursor on row %d (%q)", c.name, cursor, options[cursor])
				}
				label := sectionWideComparableLabel(options[cursor])
				if !strings.Contains(plain, label) && !strings.Contains(plain, "more above") && !strings.Contains(plain, "more below") {
					t.Errorf("%s does not draw the row under the cursor (%q) and names no window:\n%s", c.name, label, plain)
				}
			}
		})
	}
}

// sectionWideComparableLabel is the part of a row's own words that the card draws
// on one line: the mark is in the gutter and the words follow it, and a label the
// card wraps is read up to its first line's end.
func sectionWideComparableLabel(label string) string {
	_, title := sectionRowMark(label)
	if len(title) > 24 {
		return title[:24]
	}
	return title
}

// TestSectionWideRowMarksAreTheRowsOwn pins the only transformation the card makes
// to a row: the leading mark moves into the gutter, and the words the key handler
// matches are the words the card draws. A label is never renamed and never loses a
// line to the move.
func TestSectionWideRowMarksAreTheRowsOwn(t *testing.T) {
	for _, label := range []string{
		"✓ OK: PATH resolves",
		"✗ Error: ~/.ssh has mode 0755",
		"! Warning: PATH entry 4 of 9: ~/bin",
		"? Unknown: git resolves to /usr/bin/git",
		"⊘ Not applicable: pam on this host",
		"← Back",
		"[a] Run every check",
		"[x] The container volumes — 500.0 MiB",
		"Run only: …",
	} {
		mark, title := sectionRowMark(label)
		if mark != "" {
			if mark+" "+title != label {
				t.Errorf("splitting %q gave %q + %q, which does not read as the row", label, mark, title)
			}
			continue
		}
		if title != label {
			t.Errorf("the row %q was rewritten to %q", label, title)
		}
	}
}

// TestSectionWideRowMarkSplitsOnlyMarks holds the splitting to the marks a section
// row really carries: a key or a checkbox is the row's own first fact and stays in
// the title, where the key handler names it.
func TestSectionWideRowMarkSplitsOnlyMarks(t *testing.T) {
	for _, label := range []string{"[a] Run every check", "[ ] Marked", "Back", "Hello world"} {
		if mark, title := sectionRowMark(label); mark != "" || title != label {
			t.Errorf("%q was split into %q and %q, want the row whole", label, mark, title)
		}
	}
}

// TestSectionWideIsAFrameDecisionNotACursorDecision is the stability guard: the
// gate reads the width and the rows alone, so walking the list cannot change the
// shape -- and a shape that changed would move every row under the user.
func TestSectionWideIsAFrameDecisionNotACursorDecision(t *testing.T) {
	for _, c := range sectionWideCases(t) {
		c := c
		t.Run(c.name, func(t *testing.T) {
			for _, size := range [][2]int{{80, 24}, {120, 34}, {140, 44}, {160, 50}} {
				m := c.build(t)
				m.Width, m.Height = size[0], size[1]
				m.Animating = false
				want := sectionCardDrawn(m, c.listTitle)
				for cursor := 0; cursor < len(m.GetCurrentOptions()); cursor++ {
					moved := c.build(t)
					moved.Width, moved.Height = size[0], size[1]
					moved.Animating = false
					moved.Cursor = cursor
					if got := sectionCardDrawn(moved, c.listTitle); got != want {
						t.Fatalf("%s at %dx%d changes shape with the cursor on row %d: %v then %v",
							c.name, size[0], size[1], cursor, want, got)
					}
				}
			}
		})
	}
}

// TestSectionWideFitsEveryFrameItIsDrawnIn measures the shape the way the terminal
// does: no row wider than the terminal and no more rows than it has, at every
// measured size. The matrix guard covers the same screens from the model; this one
// pins the claim the shape makes -- a card never draws past the column it was
// measured in.
func TestSectionWideFitsEveryFrameItIsDrawnIn(t *testing.T) {
	for _, c := range sectionWideCases(t) {
		c := c
		t.Run(c.name, func(t *testing.T) {
			for _, size := range measuredTerminalSizes {
				m := c.build(t)
				m.Width, m.Height = size.width, size.height
				m.Animating = true
				assertScreenFitsTerminal(t, c.name, size.width, size.height, m.View())
			}
		})
	}
}

// TestSectionWideScreensAreTheOnesWithoutTheirOwnWideView names the registry: a
// screen that composes its own wide view is drawn by that view and never by this
// one, so the two shapes cannot disagree about a screen.
func TestSectionWideScreensAreTheOnesWithoutTheirOwnWideView(t *testing.T) {
	for _, screen := range []Screen{
		ScreenUtilities, ScreenThemePicker, ScreenShellAudit, ScreenUpdateCenter,
	} {
		m := Model{Screen: screen}
		if m.sectionWideScreen() {
			t.Errorf("the grey area: %v draws its own wide view and is also in the section's wide registry", screen)
		}
	}
	// And the screens this slice converts are in it.
	for _, screen := range []Screen{
		ScreenDoctor, ScreenDoctorFix, ScreenWSLResources, ScreenWSLAdvisor,
		ScreenWSLBenchmark, ScreenWSLMigration, ScreenTerminalCapabilities,
		ScreenInstallVerify, ScreenNetworkDiagnostics, ScreenDiskCleaner,
		ScreenSSHManager, ScreenSSHHost, ScreenSSHForm, ScreenSSHKeys,
		ScreenSSHAgent, ScreenSSHDiagnostics, ScreenSSHTest,
	} {
		m := Model{Screen: screen}
		if !m.sectionWideScreen() {
			t.Errorf("%v does not draw in the section's wide grammar", screen)
		}
	}
}

// cardColumns is every column a card's top border starts at in a composed body,
// smallest first: the left column's cards and then the right column's.
func cardColumns(rows []string) []int {
	seen := map[int]bool{}
	var columns []int
	for _, row := range rows {
		for at, r := range []rune(row) {
			if r != '╭' || seen[at] {
				continue
			}
			seen[at] = true
			columns = append(columns, at)
		}
	}
	sortInts(columns)
	return columns
}

// sortInts orders a short slice of columns; the guard's lists are one or two long.
func sortInts(values []int) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}

// cardTrailingBlanks is how many blank rows the card keeps under its last drawn
// row: none when the card is exactly the reading it holds.
func cardTrailingBlanks(card []string) int {
	count := 0
	for i := len(card) - 2; i > 0; i-- {
		if !cardInnerBlank(card[i]) {
			return count
		}
		count++
	}
	return count
}

// TestSectionWideDetailCardHoldsItsTallestReading is the guard for the pad: the
// Details card used to be as tall as the column the frame gave it, so a screen
// whose detail is a short reading left a card running from the header's rule to
// the footer. The card is the tallest reading the cursor can reach now -- which is
// why it is one height at every cursor position -- and a short reading therefore
// leaves a short card with the composition centred under it.
func TestSectionWideDetailCardHoldsItsTallestReading(t *testing.T) {
	short := map[string]bool{"doctor": true, "network-diagnostics": true}

	for _, c := range sectionWideCases(t) {
		c := c
		t.Run(c.name, func(t *testing.T) {
			m := c.build(t)
			m.Width, m.Height = 160, 50
			m.Animating = false

			rows := strings.Split(ansiEscape.ReplaceAllString(m.View(), ""), "\n")
			rule, footer := frameBodyEnds(rows)
			if rule < 0 || footer < 0 {
				t.Fatalf("%s does not draw the frame's own rule and footer:\n%s", c.name, strings.Join(rows, "\n"))
			}
			body := rows[rule+1 : footer]
			columns := cardColumns(body)
			if len(columns) < 2 {
				// A report draws one card across the composition and has no detail
				// column beside the list, so this card is not the one measured here.
				return
			}
			right := columns[len(columns)-1]

			var want [2]int
			filledSomewhere := false
			for cursor := 0; cursor < len(m.GetCurrentOptions()); cursor++ {
				moved := c.build(t)
				moved.Width, moved.Height = 160, 50
				moved.Animating = false
				moved.Cursor = cursor
				movedRows := strings.Split(ansiEscape.ReplaceAllString(moved.View(), ""), "\n")
				movedRule, movedFooter := frameBodyEnds(movedRows)
				if movedRule < 0 || movedFooter < 0 {
					t.Fatalf("%s does not draw the frame's own rule and footer with the cursor on row %d", c.name, cursor)
				}
				spans := cardSpansInColumn(movedRows[movedRule+1:movedFooter], right)
				if len(spans) != 1 {
					t.Fatalf("%s draws %d cards in its detail column with the cursor on row %d:\n%s",
						c.name, len(spans), cursor, strings.Join(movedRows, "\n"))
				}
				if cursor == 0 {
					want = spans[0]
				} else if spans[0] != want {
					t.Errorf("%s puts its detail card on rows %d-%d with the cursor on row %d, where row 0 put it on rows %d-%d: the composition moves as the cursor walks",
						c.name, spans[0][0], spans[0][1], cursor, want[0], want[1])
				}
				card := movedRows[movedRule+1 : movedFooter][spans[0][0] : spans[0][1]+1]
				if cardTrailingBlanks(card) == 0 {
					filledSomewhere = true
				}
			}
			if !filledSomewhere {
				t.Errorf("%s never draws its detail card without blank rows under the last fact: the card is padded, not measured from its reading", c.name)
			}
			if short[c.name] && want[1]-want[0]+1 >= len(body) {
				t.Errorf("%s draws a %d-row detail card in a %d-row body: a short reading still fills the column",
					c.name, want[1]-want[0]+1, len(body))
			}
		})
	}
}
