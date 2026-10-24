package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// ============================================================================
// THE COMPOSITION — the audit's guards
// ============================================================================
//
// The redesign places every screen on one composition: the printed body is the
// frame's inner width capped at layoutMaxComposition, centred between two
// symmetric margins, with the panel column and the creature placed against it.
// Reading the real renderers at the terminal the interface is judged on (200x55,
// the size the user's captures and the VHS tape show) found three ways the
// contract was not held there:
//
//   - a page that drew its own card at the composition width but never spent the
//     leading margin sat against the frame's left edge (the installing screen, the
//     keymap categories, the reference pages, the password screen, the trainer's
//     boss result) while every page beside it was centred;
//   - the section renderer padded its list band to the frame's rows, so a screen
//     that held no walkable list was glued under the header rule with the rest of
//     the band as one void above the status bar (the theme picker's refresh
//     review spent 32 of its 50 rows that way at 200x55);
//   - the plan panel's host note started in the label column, the one fact row on
//     the screen that broke the value column.
//
// The guards below read the rendered frame, not the directives: a screen that
// forgot the margin, filled its frame or moved a row is named here with the row
// it drew.

// auditMarginSizes are the terminals the composition's margin is measured at: the
// 200x55 the user's terminal and the VHS tape use, and the 227x62 the margin is
// widest at. 160x50 needs no margin guard -- the cap and the frame are the same
// width there, which is exactly why the defect was invisible until a wider
// terminal drew it -- but it is in the centring guard, with the 80x24 floor.
var (
	auditMarginSizes = [][2]int{{200, 55}, {227, 62}}
	auditCentreSizes = [][2]int{{80, 24}, {160, 50}, {200, 55}}
)

// TestEveryScreenSharesTheCompositionMargins is the gutter guard: on a terminal
// wider than the composition cap, no screen's body may draw outside the
// composition's columns. The creature is decoration and walks the frame's whole
// stage, so the frames are rendered with animation off -- the case its own block
// is reserved for, and the only case in which every row of the body is the
// screen's own.
func TestEveryScreenSharesTheCompositionMargins(t *testing.T) {
	for _, c := range terminalFitCases() {
		c := c
		for _, size := range auditMarginSizes {
			size := size
			t.Run(c.name+"/"+sizeName(size[0], size[1]), func(t *testing.T) {
				m := c.build(t)
				m.Width, m.Height = size[0], size[1]
				m.Animating = false

				l := layoutFor(m)
				if l.Leading <= 0 {
					t.Fatalf("%dx%d leaves no margin to measure: the guard only means something above the composition cap", size[0], size[1])
				}
				left, right := viewPaddingCols+l.Leading, viewPaddingCols+l.Leading+l.Composition

				for _, line := range frameBodyRows(m.View()) {
					if strings.TrimSpace(line) == "" {
						continue
					}
					// The ink, not the row's padding: the frame pads every row out to the
					// terminal, so a row's trailing blanks are the frame's and not the
					// page's.
					start := firstColumn(line)
					ink := strings.TrimRight(strings.TrimLeft(line, " "), " ")
					end := start + lipgloss.Width(ink)
					if start < left || end > right {
						t.Errorf("%s at %dx%d draws the row at columns %d-%d, want %d-%d inside the composition: the terminal is wider than the cap and this page is not spending the leading margin\n  %q",
							c.name, size[0], size[1], start, end, left, right, line)
					}
				}
			})
		}
	}
}

// TestNoScreenIsGluedToTheHeaderRule is the vertical guard: a body that does not
// fill its rows must not leave more blank rows under it than the frame reserves
// there. The frame centres what a screen draws, and the two reservations at the
// foot are the creature's block and the notice line; every other blank row below
// the content is a body that padded itself rather than handing its slack back.
//
// The budget is read from the model rather than guessed: companionBlockRows is
// the slot the frame keeps for the creature at this terminal, and
// sectionNoticeRows is the tallest notice band. The head of the composition may
// hold any number of blank rows; it is the foot that may not exceed it by more
// than what is reserved there.
func TestNoScreenIsGluedToTheHeaderRule(t *testing.T) {
	for _, c := range terminalFitCases() {
		c := c
		for _, size := range auditCentreSizes {
			size := size
			t.Run(c.name+"/"+sizeName(size[0], size[1]), func(t *testing.T) {
				m := c.build(t)
				m.Width, m.Height = size[0], size[1]
				m.Animating = false
				if trainerOwnsItsFrame(m) {
					// The trainer composes its own rows and pins the creature to the foot
					// of its frame (trainerCompanionRows), so its slack is above the
					// creature by design. Its own frame guards measure it.
					return
				}

				body := frameBodyRows(m.View())
				first, last := -1, -1
				for i, line := range body {
					if strings.TrimSpace(line) == "" {
						continue
					}
					if first < 0 {
						first = i
					}
					last = i
				}
				if first < 0 {
					t.Fatalf("%s at %dx%d drew no body at all", c.name, size[0], size[1])
				}

				topGap := first
				bottomGap := len(body) - 1 - last
				allowed := m.companionBlockRows(layoutFor(m).Inner) + sectionNoticeRows
				if bottomGap-topGap > allowed {
					t.Errorf("%s at %dx%d leaves %d blank rows under its block against %d above it, want at most %d more at the foot (the creature's block and the notice band): the body is filling the frame instead of taking the frame's centring\n%s",
						c.name, size[0], size[1], bottomGap, topGap, allowed, strings.Join(body, "\n"))
				}
			})
		}
	}
}

// TestThePlanPanelsNoteRidesTheValueColumn guards the one fact row that used to
// break the rhythm: the host note under the selection ("on Linux (detected)").
// Every fact row on the panel puts its value at the label measure, and a note with
// no label of its own is the selection's continuation, so it belongs in the same
// column -- not in the label column, where it read as a broken row and its wrap
// moved a second time.
//
// It measures the panel's own rows, which are what the frame draws in the column,
// so the guard reads the rhythm rather than the frame's offset.
func TestThePlanPanelsNoteRidesTheValueColumn(t *testing.T) {
	cases := []struct {
		name  string
		build func(*testing.T) Model
		facts func(Model, layout) []string
	}{
		{"main-menu", func(t *testing.T) Model { return installerFrameCase(t, "main-menu") },
			func(m Model, l layout) []string { return m.mainMenuPanelFacts(l) }},
		{"main-menu-restore", func(t *testing.T) Model { return installerFrameCase(t, "main-menu-restore") },
			func(m Model, l layout) []string { return m.mainMenuPanelFacts(l) }},
	}
	for _, name := range []string{"os-select", "shell-select", "nvim-select"} {
		name := name
		cases = append(cases, struct {
			name  string
			build func(*testing.T) Model
			facts func(Model, layout) []string
		}{name, func(t *testing.T) Model { return installerFrameCase(t, name) },
			func(m Model, l layout) []string { return m.contextPanelFacts(l) }})
	}

	for _, c := range cases {
		c := c
		for _, size := range auditCentreSizes {
			size := size
			t.Run(c.name+"/"+sizeName(size[0], size[1]), func(t *testing.T) {
				m := c.build(t)
				m.Width, m.Height = size[0], size[1]
				rows := c.facts(m, layoutFor(m))

				selected, note := -1, -1
				for i, row := range rows {
					plain := ansiEscape.ReplaceAllString(row, "")
					if selected < 0 && strings.HasPrefix(strings.TrimLeft(plain, " "), "Selected") {
						selected = i
						continue
					}
					if selected >= 0 && strings.Contains(plain, "(detected)") {
						note = i
						break
					}
				}
				if selected < 0 || note < 0 {
					// A host detection the fixture does not carry draws no note, and the
					// narrow summary has no fact rows at all: the row this guard is about
					// is not on the screen.
					return
				}

				valueCol := secondColumn(ansiEscape.ReplaceAllString(rows[selected], ""))
				noteCol := firstColumn(ansiEscape.ReplaceAllString(rows[note], ""))
				if noteCol != valueCol {
					t.Errorf("%s at %dx%d starts its host note at column %d, want the value column %d: %q",
						c.name, size[0], size[1], noteCol, valueCol, ansiEscape.ReplaceAllString(rows[note], ""))
				}
			})
		}
	}
}

// TestTrainerBossResultBoxesShareTheCardsEdge guards the fight's result: the stat
// boxes and the summary card under them are one composition, so they start on the
// same column. The boxes used to be drawn before the card was centred, which put
// them against the frame's left edge on a terminal wider than the composition
// cap.
func TestTrainerBossResultBoxesShareTheCardsEdge(t *testing.T) {
	for _, size := range auditMarginSizes {
		m := trainerLeakFrameCase(t, "trainer-boss-result")
		m.Width, m.Height = size[0], size[1]
		m.Animating = false

		box, summary := -1, -1
		for _, line := range frameBodyRows(m.View()) {
			if box < 0 && strings.Contains(line, "╭") {
				box = firstColumn(line)
				continue
			}
			if box >= 0 && strings.Contains(line, "Summary") {
				summary = firstColumn(line)
				break
			}
		}
		if box < 0 || summary < 0 {
			t.Fatalf("trainer-boss-result at %dx%d drew no stat row and summary card to compare", size[0], size[1])
		}
		if box != summary {
			t.Errorf("trainer-boss-result at %dx%d starts its stat boxes at column %d and its summary card at column %d, want one edge",
				size[0], size[1], box, summary)
		}
	}
}

// trainerOwnsItsFrame reports whether the screen draws the trainer's own frame
// rather than the installer's.
func trainerOwnsItsFrame(m Model) bool {
	switch m.Screen {
	case ScreenTrainerMenu, ScreenTrainerLesson, ScreenTrainerPractice,
		ScreenTrainerBoss, ScreenTrainerResult, ScreenTrainerBossResult:
		return true
	}
	return false
}

// frameBodyRows is the rows between the frame's header rule and its status bar:
// the rows a screen composes. The status bar is the last row that carries the
// state dot, so a body row with a dot of its own (a running step's glyph) is not
// mistaken for the chrome.
func frameBodyRows(view string) []string {
	lines := strings.Split(ansiEscape.ReplaceAllString(view, ""), "\n")
	status := -1
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.Contains(lines[i], "● ") {
			status = i
			break
		}
	}
	if status < 0 || status < 4 {
		return nil
	}
	return lines[3:status]
}

// firstColumn is the cell the row's ink starts at. Columns are counted in cells,
// not bytes: a Nerd Font glyph is one cell and three bytes, and a rule is one cell
// per box character, so a byte offset would call a correct row a hundred columns
// too wide.
func firstColumn(line string) int {
	runes := []rune(line)
	at := 0
	for at < len(runes) && runes[at] == ' ' {
		at++
	}
	return at
}

// secondColumn is the cell the row's second word starts at: the value beside a
// fact row's label.
func secondColumn(line string) int {
	runes := []rune(line)
	at := firstColumn(line)
	for at < len(runes) && runes[at] != ' ' && runes[at] != '\t' {
		at++
	}
	for at < len(runes) && (runes[at] == ' ' || runes[at] == '\t') {
		at++
	}
	return at
}
