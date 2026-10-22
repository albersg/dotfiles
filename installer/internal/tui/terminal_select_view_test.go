package tui

import (
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
	"github.com/charmbracelet/lipgloss"
)

// ============================================================================
// THE TERMINAL SELECT SCREEN'S CARD — the guards
// ============================================================================
//
// The terminal question is the second wizard picker drawn in the reference
// grammar (terminal_select_view.go), through the OS question's card and rows. The
// guards below hold the four things that makes true: the card is the shared one,
// the rows are still the model's own answers in its own order with the option
// list's separator as the card's divider, the keys and the cursor are still the
// screen's, and a frame that cannot hold the card gets the compact list instead of
// a card with a fact cut off.

// terminalSelectPlain renders the terminal question and returns the frame without
// escapes, so a guard reads the words a person reads.
func terminalSelectPlain(t *testing.T, model Model, width, height int) string {
	t.Helper()
	model.Width, model.Height = width, height
	model.Animating = false
	return ansiEscape.ReplaceAllString(model.View(), "")
}

// terminalSelectCard finds the card's borders around the chip the screen draws: the
// top border carrying the title, and the closing border under it. A screen without
// the card reports -1 for both, which is the fallback guards' expectation.
func terminalSelectCard(lines []string, title string) (top, bottom int) {
	top, bottom = -1, -1
	for i, line := range lines {
		if top < 0 && strings.Contains(line, "╭─ "+title+" ") {
			top = i
		}
		if top >= 0 && strings.Contains(line, "╰") {
			bottom = i
		}
	}
	return top, bottom
}

// cardBodyText reads what the card says between its borders: the edge glyph and the
// blank column each row carries are taken off and the rows are joined, so a sentence
// the card wrapped can be read as the words it holds. It is how a guard checks that a
// note moved inside the card still says everything, rather than checking the frame's
// line breaks.
func cardBodyText(lines []string, top, bottom int) string {
	var b strings.Builder
	for i := top + 1; i < bottom && i < len(lines); i++ {
		row := strings.Trim(strings.TrimSpace(lines[i]), "│├┤─")
		if row = strings.Join(strings.Fields(row), " "); row == "" {
			continue
		}
		b.WriteString(row)
		b.WriteString(" ")
	}
	return b.String()
}

// TestTerminalSelectDrawsTheReferenceGrammar is the shape guard: the screen's own
// title is the chip on the card's top border, the model's own description is the
// caption on that same row, the card closes under its rows, and every answer
// carries the grammar's icon, subtitle and chevron. It is measured at the floor and
// at the two-column frame, because the card is the shape at both.
func TestTerminalSelectDrawsTheReferenceGrammar(t *testing.T) {
	for _, size := range []struct {
		name          string
		width, height int
	}{
		{"80x24, the floor", 80, 24},
		{"160x50, two columns", 160, 50},
	} {
		size := size
		t.Run(size.name, func(t *testing.T) {
			m := installerFrameCase(t, "terminal-select")
			plain := terminalSelectPlain(t, m, size.width, size.height)
			lines := strings.Split(plain, "\n")

			title := stripStepPrefix(m.GetScreenTitle())
			top, bottom := terminalSelectCard(lines, title)
			if top < 0 {
				t.Fatalf("the terminal question does not carry %q as its card's chip at %s:\n%s", title, size.name, plain)
			}
			if bottom < top {
				t.Fatalf("the card at %s has no closing border:\n%s", size.name, plain)
			}
			// The caption rides the chip's row, as the reference draws it. The left
			// column of the two-column frame is narrower than the one-column body, so
			// on a frame whose border row has no room left the card keeps the words
			// inside itself instead -- and either way the screen's own description is
			// drawn.
			caption := "Select your preferred terminal emulator"
			if !strings.Contains(lines[top], caption) && !strings.Contains(cardBodyText(lines, top, bottom), caption) {
				t.Errorf("the card at %s does not carry the model's own %q, on its border or inside it: %q\n%s",
					size.name, caption, lines[top], plain)
			}

			// The answers are the grammar's rows: the title, the subtitle under it,
			// and the chevron that says the row opens something. They are drawn
			// inside the card, so a row the border clipped cannot pass.
			for _, option := range m.GetCurrentOptions() {
				if strings.HasPrefix(option, menuSeparatorPrefix) {
					continue
				}
				label := plainOptionName(option)
				_, subtitle := terminalSelectRowParts(label)
				for _, want := range []string{label, subtitle} {
					if !rowAppearsBetween(lines, want, top, bottom) {
						t.Errorf("the card at %s does not draw %q between its borders (rows %d-%d):\n%s",
							size.name, want, top, bottom, plain)
					}
				}
				if !rowAppearsBetween(lines, grammarChevron, top, bottom) {
					t.Errorf("the card at %s draws no chevron for %q:\n%s", size.name, label, plain)
				}
			}

			// The footer keeps the keys the screen really answers to, and the state
			// the frame carries on every screen.
			for _, want := range []string{"↑/k", "↓/j", "[Enter]", "[Esc]"} {
				if !strings.Contains(plain, want) {
					t.Errorf("the terminal question at %s no longer names %q in its legend:\n%s", size.name, want, plain)
				}
			}
			if !strings.Contains(plain, statusDotGlyph+" "+statusReady) {
				t.Errorf("the terminal question at %s does not carry the frame's status bar:\n%s", size.name, plain)
			}
		})
	}
}

// TestTerminalSelectRowsAreTheModelsAnswersInOrder is the reachability guard: the
// card draws the answers in the order the key handler reads them, with no answer
// added, renamed or reordered, and the cursor's row is the one with the marker. The
// option list's separator is the card's divider -- a rule across the card, never a
// row the cursor can stand on -- so the shortcut row stays where the list puts it.
func TestTerminalSelectRowsAreTheModelsAnswersInOrder(t *testing.T) {
	m := installerFrameCase(t, "terminal-select")
	options := m.GetCurrentOptions()
	if len(options) == 0 {
		t.Fatal("the terminal question offers no answers to draw")
	}

	for cursor := 0; cursor < len(options); cursor++ {
		if strings.HasPrefix(options[cursor], menuSeparatorPrefix) {
			// The separator is the card's divider, and the key handler steps over
			// it: there is no cursor position on it to draw.
			continue
		}
		moved := m
		moved.Cursor = cursor
		plain := terminalSelectPlain(t, moved, 80, 24)
		lines := strings.Split(plain, "\n")

		title := stripStepPrefix(m.GetScreenTitle())
		top, _ := terminalSelectCard(lines, title)
		if top < 0 {
			t.Fatalf("the card is not drawn with the cursor on %d:\n%s", cursor, plain)
		}

		at := -1
		for _, option := range options {
			if strings.HasPrefix(option, menuSeparatorPrefix) {
				continue
			}
			label := plainOptionName(option)
			pos := -1
			for i := top + 1; i < len(lines); i++ {
				if strings.Contains(lines[i], label) {
					pos = i
					break
				}
			}
			if pos < 0 {
				t.Fatalf("the card does not draw the answer %q (cursor %d):\n%s", label, cursor, plain)
			}
			if pos < at {
				t.Fatalf("the card reorders its answers: %q is drawn after the answer that follows it (cursor %d):\n%s",
					label, cursor, plain)
			}
			at = pos
		}

		// The separator is the card's divider: a rule set into the card's borders,
		// not the option string laid across the body as a row.
		if !rowAppearsBetween(lines, "├", top, len(lines)-1) {
			t.Errorf("the card draws no divider between its two groups (cursor %d):\n%s", cursor, plain)
		}
		for _, option := range options {
			if !strings.HasPrefix(option, menuSeparatorPrefix) {
				continue
			}
			for _, line := range lines {
				if strings.Contains(line, option) {
					t.Errorf("the card draws the option list's separator as a row of its own (cursor %d): %q", cursor, line)
				}
			}
		}

		// One marker, on the row the cursor is on: the grammar's selected row is a
		// filled bar carrying the cursor's mark, so the state is not the bar's colour
		// alone.
		if got := strings.Count(plain, grammarMarker); got != 1 {
			t.Fatalf("the card draws %d cursor markers with the cursor on %d, want exactly one:\n%s", got, cursor, plain)
		}
		markerRow := ""
		for _, line := range lines {
			if strings.Contains(line, grammarMarker) {
				markerRow = line
			}
		}
		if !strings.Contains(markerRow, plainOptionName(options[cursor])) {
			t.Errorf("the marker is not on the answer under the cursor (%q): %q", options[cursor], markerRow)
		}
	}
}

// TestTerminalSelectRowTableCoversEveryAnswer pins the copy the rows add: every
// answer the screen can offer -- including the two the macOS host adds and the
// shortcut row -- has a one-cell glyph and a subtitle, and no subtitle is longer
// than the row can draw, so the card never ends a row with the cut marker.
func TestTerminalSelectRowTableCoversEveryAnswer(t *testing.T) {
	labels := []string{}
	for _, choices := range []UserChoices{{}, {OS: "mac"}} {
		m := Model{Screen: ScreenTerminalSelect, Choices: choices}
		labels = append(labels, m.GetCurrentOptions()...)
		labels = append(labels, m.terminalOptions()...)
	}
	labels = append(labels, terminalSelectLearnLabel)

	seen := map[string]bool{}
	for _, option := range labels {
		if strings.HasPrefix(option, menuSeparatorPrefix) {
			continue
		}
		label := plainOptionName(option)
		if seen[label] {
			continue
		}
		seen[label] = true

		icon, subtitle := terminalSelectRowParts(label)
		if icon == "" {
			t.Errorf("the answer %q draws no glyph in the row's gutter", label)
		}
		if w := lipgloss.Width(icon); w != 1 {
			t.Errorf("the answer %q draws a %d-cell glyph %q, want one cell", label, w, icon)
		}
		if subtitle == "" {
			t.Errorf("the answer %q draws no subtitle under its title", label)
		}
		if w := lipgloss.Width(subtitle); w > terminalSelectSubtitleMeasure {
			t.Errorf("the answer %q draws a %d-column subtitle, over the %d its row can hold: %q",
				label, w, terminalSelectSubtitleMeasure, subtitle)
		}
	}
	for _, want := range []string{"Alacritty", "WezTerm", "Kitty", "Ghostty", "None", terminalSelectLearnLabel} {
		if !seen[want] {
			t.Fatalf("the guard did not check %q: the option lists no longer offer it", want)
		}
	}
}

// TestTerminalSelectLeavesEnterUnnamedWithNoRowUnderTheCursor keeps the screen's
// own key rule: with noSelection no row is marked, and the legend must not
// advertise a press that would record nothing.
func TestTerminalSelectLeavesEnterUnnamedWithNoRowUnderTheCursor(t *testing.T) {
	m := installerFrameCase(t, "terminal-select")
	m.Cursor = noSelection

	plain := terminalSelectPlain(t, m, 80, 24)
	if strings.Contains(plain, grammarMarker) {
		t.Errorf("the card marks a row with no cursor on any row:\n%s", plain)
	}
	if strings.Contains(plain, "[Enter]") {
		t.Errorf("the legend names Enter with no row under the cursor:\n%s", plain)
	}
	if !strings.Contains(plain, "[Esc]") {
		t.Errorf("the legend lost the way back with no row under the cursor:\n%s", plain)
	}
}

// TestTerminalSelectKeepsItsShapeWhileTheCursorMoves pins the gate to the frame: the
// card is drawn or it is not, and the cursor's position never decides which. A shape
// that came and went as the cursor walked would move every row on the screen.
func TestTerminalSelectKeepsItsShapeWhileTheCursorMoves(t *testing.T) {
	m := installerFrameCase(t, "terminal-select")
	title := stripStepPrefix(m.GetScreenTitle())

	var first string
	for cursor := 0; cursor < len(m.GetCurrentOptions()); cursor++ {
		moved := m
		moved.Cursor = cursor
		plain := terminalSelectPlain(t, moved, 80, 24)
		top, _ := terminalSelectCard(strings.Split(plain, "\n"), title)
		if top < 0 {
			t.Fatalf("the card is not drawn with the cursor on %d:\n%s", cursor, plain)
		}
		border := strings.Split(plain, "\n")[top]
		if first == "" {
			first = border
			continue
		}
		if border != first {
			t.Fatalf("the card's top border changed with the cursor on %d:\nwas: %q\nnow: %q", cursor, first, border)
		}
	}
}

// TestTerminalSelectFallsBackToTheCompactListWhenTheCardCannotFit pins the floor: a
// frame too narrow or too short for the card draws the compact list instead -- the
// same answers, the page's title and description above them -- rather than a card
// that leaves a fact on the cutting-room floor.
func TestTerminalSelectFallsBackToTheCompactListWhenTheCardCannotFit(t *testing.T) {
	for _, size := range []struct {
		name          string
		width, height int
	}{
		{"36x24, too narrow for a two-line row", 36, 24},
		{"80x12, too few rows for the card", 80, 12},
	} {
		size := size
		t.Run(size.name, func(t *testing.T) {
			m := installerFrameCase(t, "terminal-select")
			m.Cursor = 0

			plain := terminalSelectPlain(t, m, size.width, size.height)
			lines := strings.Split(plain, "\n")
			title := stripStepPrefix(m.GetScreenTitle())
			if top, _ := terminalSelectCard(lines, title); top >= 0 {
				t.Fatalf("the card is drawn at %s, where it cannot hold its rows:\n%s", size.name, plain)
			}
			// The compact list is the same screen: the title and the answer under the
			// cursor are still drawn.
			if !strings.Contains(plain, title) {
				t.Errorf("the fallback at %s lost the page's title:\n%s", size.name, plain)
			}
			if !strings.Contains(plain, "Alacritty") {
				t.Errorf("the fallback at %s lost the answer under the cursor:\n%s", size.name, plain)
			}
		})
	}
}

// TestTerminalSelectKeepsTheNoteWhenTheBorderCannotHoldIt guards the one datum the
// card could drop: the terminal question's note is two sentences on a WSL host and
// on a host where Alacritty builds from source, and it is longer than a card's
// border row can hold beside the chip. It moves inside the card instead of being
// given back, because the note is what tells the reader the step will be skipped or
// will cost minutes.
func TestTerminalSelectKeepsTheNoteWhenTheBorderCannotHoldIt(t *testing.T) {
	cases := []struct {
		name    string
		build   func(*testing.T) Model
		phrases []string
	}{
		{
			name:  "a WSL host",
			build: func(t *testing.T) Model { return installerFrameCase(t, "terminal-select-wsl") },
			phrases: []string{
				"should be installed on Windows",
				"skip terminal setup",
			},
		},
		{
			name: "Alacritty builds from source",
			build: func(t *testing.T) Model {
				m := installerFrameCase(t, "terminal-select")
				m.Choices = UserChoices{OS: "linux"}
				m.SystemInfo = &system.SystemInfo{OS: system.OSDebian, OSName: "Debian/Ubuntu", Arch: "x86_64"}
				return m
			},
			phrases: []string{
				"Select your preferred terminal emulator",
				"builds from source on this system",
				"5–10 minutes",
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			m := c.build(t)
			description := m.GetScreenDescription()
			plain := terminalSelectPlain(t, m, 80, 24)
			lines := strings.Split(plain, "\n")
			title := stripStepPrefix(m.GetScreenTitle())
			top, bottom := terminalSelectCard(lines, title)
			if top < 0 {
				t.Fatalf("the card is not drawn on %s:\n%s", c.name, plain)
			}
			// The note is wrapped inside the card, so its sentences are read from the
			// card's own body rather than from the frame's line breaks.
			bodyText := cardBodyText(lines, top, bottom)
			for _, want := range c.phrases {
				if !strings.Contains(bodyText, want) {
					t.Errorf("the card on %s does not carry the note's %q in its body:\n%s\n(the model's description is %q)", c.name, want, plain, description)
				}
			}
		})
	}
}
