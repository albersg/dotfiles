package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// ============================================================================
// THE EDITOR (NEOVIM) SELECT SCREEN'S CARD — the guards
// ============================================================================
//
// The editor question is the sixth wizard picker drawn in the reference grammar
// (nvim_select_view.go), through the OS, terminal, font, shell and multiplexer
// questions' card and rows. The guards below hold the four things that makes true:
// the card is the shared one, the rows are still the model's own answers in its own
// order with the option list's separator as the card's divider, the keys and the
// cursor are still the screen's, and a frame that cannot hold the card gets the
// compact list instead of a card with a fact cut off.

// nvimSelectPlain renders the editor question and returns the frame without
// escapes, so a guard reads the words a person reads.
func nvimSelectPlain(t *testing.T, model Model, width, height int) string {
	t.Helper()
	model.Width, model.Height = width, height
	model.Animating = false
	return ansiEscape.ReplaceAllString(model.View(), "")
}

// nvimSelectCard finds the card's borders around the chip the screen draws: the top
// border carrying the title, and the closing border under it. A screen without the
// card reports -1 for both, which is the fallback guards' expectation.
func nvimSelectCard(lines []string, title string) (top, bottom int) {
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

// nvimSelectBodyText reads the card's own words between its borders. It slices each
// row between the first pair of edges rather than trimming the whole row, because
// the two-column frame draws the plan panel beside the card: only the leftmost card
// is read here, which is the card the screen draws.
func nvimSelectBodyText(lines []string, top, bottom int) string {
	var b strings.Builder
	for i := top + 1; i < bottom && i < len(lines); i++ {
		line := lines[i]
		start := strings.Index(line, "│")
		if start < 0 {
			continue
		}
		rest := line[start+len("│"):]
		end := strings.Index(rest, "│")
		if end < 0 {
			continue
		}
		row := strings.Trim(strings.Join(strings.Fields(rest[:end]), " "), "─")
		if row == "" {
			continue
		}
		b.WriteString(row)
		b.WriteString(" ")
	}
	return b.String()
}

// TestNvimSelectDrawsTheReferenceGrammar is the shape guard: the screen's own title
// is the chip on the card's top border, the model's own description is drawn -- on
// that same row or, where the border has no room, inside the card -- the card closes
// under its rows, and every answer carries the grammar's icon, subtitle and chevron.
// It is measured at the floor and at the two-column frame, because the card is the
// shape at both.
func TestNvimSelectDrawsTheReferenceGrammar(t *testing.T) {
	for _, size := range []struct {
		name          string
		width, height int
	}{
		{"80x24, the floor", 80, 24},
		{"160x50, two columns", 160, 50},
	} {
		size := size
		t.Run(size.name, func(t *testing.T) {
			m := installerFrameCase(t, "nvim-select")
			plain := nvimSelectPlain(t, m, size.width, size.height)
			lines := strings.Split(plain, "\n")

			title := stripStepPrefix(m.GetScreenTitle())
			top, bottom := nvimSelectCard(lines, title)
			if top < 0 {
				t.Fatalf("the editor question does not carry %q as its card's chip at %s:\n%s", title, size.name, plain)
			}
			if bottom < top {
				t.Fatalf("the card at %s has no closing border:\n%s", size.name, plain)
			}
			// The description is the screen's own sentence, drawn on the chip's row or
			// kept inside the card when the border row cannot hold it -- which is what
			// happens in the two-column frame, whose left column is narrower than the
			// floor's card.
			caption := m.GetScreenDescription()
			if !strings.Contains(lines[top], caption) && !strings.Contains(nvimSelectBodyText(lines, top, bottom), caption) {
				t.Errorf("the card at %s does not carry the model's own description %q, on its border or inside it: %q\n%s",
					size.name, caption, lines[top], plain)
			}

			// The answers are the grammar's rows: the title, the subtitle under it, and
			// the chevron that says the row opens something. They are drawn inside the
			// card, so a row the border clipped cannot pass.
			for _, option := range m.GetCurrentOptions() {
				if strings.HasPrefix(option, menuSeparatorPrefix) {
					continue
				}
				label := plainOptionName(option)
				_, subtitle := nvimSelectRowParts(label)
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

			// The footer keeps the keys the screen really answers to, and the state the
			// frame carries on every screen.
			for _, want := range []string{"↑/k", "↓/j", "[Enter]", "[Esc]"} {
				if !strings.Contains(plain, want) {
					t.Errorf("the editor question at %s no longer names %q in its legend:\n%s", size.name, want, plain)
				}
			}
			if !strings.Contains(plain, statusDotGlyph+" "+statusReady) {
				t.Errorf("the editor question at %s does not carry the frame's status bar:\n%s", size.name, plain)
			}
		})
	}
}

// TestNvimSelectRowsAreTheModelsAnswersInOrder is the reachability guard: the card
// draws the answers in the order the key handler reads them, with no answer added,
// renamed or reordered, and the cursor's row is the one with the marker. The option
// list's separator is the card's divider -- a rule across the card, never a row the
// cursor can stand on -- so the three shortcut rows stay where the list puts them.
func TestNvimSelectRowsAreTheModelsAnswersInOrder(t *testing.T) {
	m := installerFrameCase(t, "nvim-select")
	options := m.GetCurrentOptions()
	if len(options) == 0 {
		t.Fatal("the editor question offers no answers to draw")
	}

	for cursor := 0; cursor < len(options); cursor++ {
		if strings.HasPrefix(options[cursor], menuSeparatorPrefix) {
			// The separator is the card's divider, and the key handler steps over it:
			// there is no cursor position on it to draw.
			continue
		}
		moved := m
		moved.Cursor = cursor
		plain := nvimSelectPlain(t, moved, 80, 24)
		lines := strings.Split(plain, "\n")

		title := stripStepPrefix(m.GetScreenTitle())
		top, _ := nvimSelectCard(lines, title)
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

		// The separator is the card's divider: a rule set into the card's borders, not
		// the option string laid across the body as a row.
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

// TestNvimSelectRowTableCoversEveryAnswer pins the copy the rows add: every answer
// the screen can offer -- the two halves of the question and the three shortcut rows
// -- has a one-cell glyph and a subtitle, and no subtitle is longer than the row can
// draw, so the card never ends a row with the cut marker.
func TestNvimSelectRowTableCoversEveryAnswer(t *testing.T) {
	m := Model{Screen: ScreenNvimSelect}
	labels := append([]string(nil), m.GetCurrentOptions()...)
	labels = append(labels, m.nvimOptions()...)
	labels = append(labels, nvimSelectLearnLabel, nvimSelectKeymapsLabel, nvimSelectGuideLabel)

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

		icon, subtitle := nvimSelectRowParts(label)
		if icon == "" {
			t.Errorf("the answer %q draws no glyph in the row's gutter", label)
		}
		if w := lipgloss.Width(icon); w != 1 {
			t.Errorf("the answer %q draws a %d-cell glyph %q, want one cell", label, w, icon)
		}
		if subtitle == "" {
			t.Errorf("the answer %q draws no subtitle under its title", label)
		}
		if w := lipgloss.Width(subtitle); w > nvimSelectSubtitleMeasure {
			t.Errorf("the answer %q draws a %d-column subtitle, over the %d its row can hold: %q",
				label, w, nvimSelectSubtitleMeasure, subtitle)
		}
	}
	for _, want := range []string{
		"Yes, install Neovim with config", "No, skip Neovim",
		nvimSelectLearnLabel, nvimSelectKeymapsLabel, nvimSelectGuideLabel,
	} {
		if !seen[want] {
			t.Fatalf("the guard did not check %q: the option lists no longer offer it", want)
		}
	}
}

// TestNvimSelectLeavesEnterUnnamedWithNoRowUnderTheCursor keeps the screen's own key
// rule: with noSelection no row is marked, and the legend must not advertise a press
// that would record nothing.
func TestNvimSelectLeavesEnterUnnamedWithNoRowUnderTheCursor(t *testing.T) {
	m := installerFrameCase(t, "nvim-select")
	m.Cursor = noSelection

	plain := nvimSelectPlain(t, m, 80, 24)
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

// TestNvimSelectKeepsItsShapeWhileTheCursorMoves pins the gate to the frame: the card
// is drawn or it is not, and the cursor's position never decides which. A shape that
// came and went as the cursor walked would move every row on the screen.
func TestNvimSelectKeepsItsShapeWhileTheCursorMoves(t *testing.T) {
	m := installerFrameCase(t, "nvim-select")
	title := stripStepPrefix(m.GetScreenTitle())

	var first string
	for cursor := 0; cursor < len(m.GetCurrentOptions()); cursor++ {
		moved := m
		moved.Cursor = cursor
		plain := nvimSelectPlain(t, moved, 80, 24)
		top, _ := nvimSelectCard(strings.Split(plain, "\n"), title)
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

// TestNvimSelectFallsBackToTheCompactListWhenTheCardCannotFit pins the floor: a frame
// too narrow or too short for the card draws the compact list instead -- the same
// answers, the page's title and description above them -- rather than a card that
// leaves a fact on the cutting-room floor.
func TestNvimSelectFallsBackToTheCompactListWhenTheCardCannotFit(t *testing.T) {
	for _, size := range []struct {
		name          string
		width, height int
	}{
		{"36x24, too narrow for a two-line row", 36, 24},
		{"80x12, too few rows for the card", 80, 12},
	} {
		size := size
		t.Run(size.name, func(t *testing.T) {
			m := installerFrameCase(t, "nvim-select")
			m.Cursor = 0

			plain := nvimSelectPlain(t, m, size.width, size.height)
			lines := strings.Split(plain, "\n")
			title := stripStepPrefix(m.GetScreenTitle())
			if top, _ := nvimSelectCard(lines, title); top >= 0 {
				t.Fatalf("the card is drawn at %s, where it cannot hold its rows:\n%s", size.name, plain)
			}
			// The compact list is the same screen: the title and the answer under the
			// cursor are still drawn. The 36-column frame cuts that answer's tail -- the
			// compact row is one line -- so the guard reads the part of it that says
			// which answer it is.
			if !strings.Contains(plain, title) {
				t.Errorf("the fallback at %s lost the page's title:\n%s", size.name, plain)
			}
			if !strings.Contains(plain, "Yes, install Neovim") {
				t.Errorf("the fallback at %s lost the answer under the cursor:\n%s", size.name, plain)
			}
		})
	}
}
