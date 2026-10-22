package tui

import (
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
	"github.com/charmbracelet/lipgloss"
)

// ============================================================================
// THE FONT SELECT SCREEN'S CARD — the guards
// ============================================================================
//
// The font question is the fourth wizard picker drawn in the reference grammar
// (font_select_view.go), through the OS, terminal and shell questions' card and
// rows. The guards below hold the four things that makes true: the card is the
// shared one, the rows are still the model's own answers in its own order, the keys
// and the cursor are still the screen's, and a frame that cannot hold the card gets
// the compact list instead of a card with a fact cut off.

// fontSelectPlain renders the font question and returns the frame without escapes,
// so a guard reads the words a person reads.
func fontSelectPlain(t *testing.T, model Model, width, height int) string {
	t.Helper()
	model.Width, model.Height = width, height
	model.Animating = false
	return ansiEscape.ReplaceAllString(model.View(), "")
}

// fontSelectCard finds the card's borders around the chip the screen draws: the top
// border carrying the title, and the closing border under it. A screen without the
// card reports -1 for both, which is the fallback guards' expectation.
func fontSelectCard(lines []string, title string) (top, bottom int) {
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

// fontSelectBodyText reads the card's own words between its borders. It slices each
// row between the first pair of edges rather than trimming the whole row, because
// the two-column frame draws the plan panel beside the card: the shared cardBodyText
// joins both cards' rows and a caption the card wrapped is then broken by the
// neighbour's facts. Only the leftmost card is read here, which is the card the
// screen draws.
func fontSelectBodyText(lines []string, top, bottom int) string {
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

// TestFontSelectDrawsTheReferenceGrammar is the shape guard: the screen's own title
// is the chip on the card's top border, the model's own description is drawn -- on
// that same row or, where the border has no room, inside the card -- the card closes
// under its rows, and every answer carries the grammar's icon, subtitle and chevron.
// It is measured at the floor and at the two-column frame, because the card is the
// shape at both.
func TestFontSelectDrawsTheReferenceGrammar(t *testing.T) {
	for _, size := range []struct {
		name          string
		width, height int
	}{
		{"80x24, the floor", 80, 24},
		{"160x50, two columns", 160, 50},
	} {
		size := size
		t.Run(size.name, func(t *testing.T) {
			m := installerFrameCase(t, "font-select")
			m.Choices.InstallFont = true
			m.Cursor = m.questionValueIndex(ScreenFontSelect)
			plain := fontSelectPlain(t, m, size.width, size.height)
			lines := strings.Split(plain, "\n")

			title := stripStepPrefix(m.GetScreenTitle())
			top, bottom := fontSelectCard(lines, title)
			if top < 0 {
				t.Fatalf("the font question does not carry %q as its card's chip at %s:\n%s", title, size.name, plain)
			}
			if bottom < top {
				t.Fatalf("the card at %s has no closing border:\n%s", size.name, plain)
			}
			// The description is the measurement of what this machine has, and it is
			// longer than the border row can hold beside the chip, so the card keeps
			// the words inside itself -- and either way the screen's own description is
			// drawn.
			caption := m.GetScreenDescription()
			if !strings.Contains(lines[top], strings.ReplaceAll(caption, "\n", " ")) &&
				!strings.Contains(fontSelectBodyText(lines, top, bottom), strings.ReplaceAll(caption, "\n", " ")) {
				t.Errorf("the card at %s does not carry the model's own description %q, on its border or inside it: %q\n%s",
					size.name, caption, lines[top], plain)
			}

			// The answers are the grammar's rows: the title, the subtitle under it,
			// and the chevron that says the row opens something. They are drawn
			// inside the card, so a row the border clipped cannot pass.
			for _, option := range m.GetCurrentOptions() {
				label := plainOptionName(option)
				_, subtitle := m.fontSelectRowParts(option)
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
					t.Errorf("the font question at %s no longer names %q in its legend:\n%s", size.name, want, plain)
				}
			}
			if !strings.Contains(plain, statusDotGlyph+" "+statusReady) {
				t.Errorf("the font question at %s does not carry the frame's status bar:\n%s", size.name, plain)
			}
		})
	}
}

// TestFontSelectRowsAreTheModelsAnswersInOrder is the reachability guard: the card
// draws the answers in the order the key handler reads them, with no answer added,
// renamed or reordered, and the cursor's row is the one with the marker. The font
// question has no separator in its option list, so the card has one group and no
// divider -- the two answers stay in the one group the list puts them in.
func TestFontSelectRowsAreTheModelsAnswersInOrder(t *testing.T) {
	m := installerFrameCase(t, "font-select")
	options := m.GetCurrentOptions()
	if len(options) != 2 {
		t.Fatalf("the font question offers %d answers, want the two the step asks: %v", len(options), options)
	}

	for cursor := 0; cursor < len(options); cursor++ {
		moved := m
		moved.Cursor = cursor
		plain := fontSelectPlain(t, moved, 80, 24)
		lines := strings.Split(plain, "\n")

		title := stripStepPrefix(m.GetScreenTitle())
		top, bottom := fontSelectCard(lines, title)
		if top < 0 {
			t.Fatalf("the card is not drawn with the cursor on %d:\n%s", cursor, plain)
		}

		at := -1
		for _, option := range options {
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

		// The option list carries no separator, so the card draws no divider: a group
		// rule here would be a section the question does not have.
		if rowAppearsBetween(lines, "├", top, bottom) {
			t.Errorf("the card draws a group divider the font question has no groups for (cursor %d):\n%s", cursor, plain)
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

// TestFontSelectRowTableCoversEveryAnswer pins the copy the rows add: every answer
// the screen can offer has a one-cell glyph and a subtitle, and no subtitle is longer
// than the row can draw, so the card never ends a row with the cut marker. The labels
// carry the family's own name, so the guard walks every host the step has a branch
// for rather than the one the test machine happens to be.
func TestFontSelectRowTableCoversEveryAnswer(t *testing.T) {
	hosts := []struct {
		name string
		m    Model
	}{
		{"a host nothing measured", Model{Screen: ScreenFontSelect}},
		{"a Linux host", Model{Screen: ScreenFontSelect, SystemInfo: &system.SystemInfo{OS: system.OSLinux}}},
		{"a Termux host", Model{Screen: ScreenFontSelect, SystemInfo: &system.SystemInfo{OS: system.OSLinux, IsTermux: true}}},
		{"a macOS host", Model{Screen: ScreenFontSelect, SystemInfo: &system.SystemInfo{OS: system.OSMac}}},
	}

	seen := map[string]bool{}
	for _, host := range hosts {
		host := host
		t.Run(host.name, func(t *testing.T) {
			options := host.m.GetCurrentOptions()
			if len(options) < 2 {
				t.Fatalf("the font question offers %d answers, want the two the step asks: %v", len(options), options)
			}
			for _, option := range options {
				label := plainOptionName(option)
				seen[label] = true

				icon, subtitle := host.m.fontSelectRowParts(option)
				if icon == "" {
					t.Errorf("the answer %q draws no glyph in the row's gutter", label)
				}
				if w := lipgloss.Width(icon); w != 1 {
					t.Errorf("the answer %q draws a %d-cell glyph %q, want one cell", label, w, icon)
				}
				if subtitle == "" {
					t.Errorf("the answer %q draws no subtitle under its title", label)
				}
				if w := lipgloss.Width(subtitle); w > fontSelectSubtitleMeasure {
					t.Errorf("the answer %q draws a %d-column subtitle, over the %d its row can hold: %q",
						label, w, fontSelectSubtitleMeasure, subtitle)
				}
			}
		})
	}

	for _, want := range []string{"Install Meslo Nerd Font", "Skip it"} {
		if !seen[want] {
			t.Fatalf("the guard did not check %q: the option lists no longer offer it", want)
		}
	}
}

// TestFontSelectLeavesEnterUnnamedWithNoRowUnderTheCursor keeps the screen's own key
// rule: with noSelection no row is marked, and the legend must not advertise a press
// that would record nothing.
func TestFontSelectLeavesEnterUnnamedWithNoRowUnderTheCursor(t *testing.T) {
	m := installerFrameCase(t, "font-select")
	m.Cursor = noSelection

	plain := fontSelectPlain(t, m, 80, 24)
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

// TestFontSelectKeepsItsShapeWhileTheCursorMoves pins the gate to the frame: the card
// is drawn or it is not, and the cursor's position never decides which. A shape that
// came and went as the cursor walked would move every row on the screen.
func TestFontSelectKeepsItsShapeWhileTheCursorMoves(t *testing.T) {
	m := installerFrameCase(t, "font-select")
	title := stripStepPrefix(m.GetScreenTitle())

	var first string
	for cursor := 0; cursor < len(m.GetCurrentOptions()); cursor++ {
		moved := m
		moved.Cursor = cursor
		plain := fontSelectPlain(t, moved, 80, 24)
		top, _ := fontSelectCard(strings.Split(plain, "\n"), title)
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

// TestFontSelectFallsBackToTheCompactListWhenTheCardCannotFit pins the floor: a frame
// too narrow or too short for the card draws the compact list instead -- the same
// answers, the page's title and description above them -- rather than a card that
// leaves a fact on the cutting-room floor.
func TestFontSelectFallsBackToTheCompactListWhenTheCardCannotFit(t *testing.T) {
	for _, size := range []struct {
		name          string
		width, height int
	}{
		{"36x24, too narrow for a two-line row", 36, 24},
		{"80x12, too few rows for the card", 80, 12},
	} {
		size := size
		t.Run(size.name, func(t *testing.T) {
			m := installerFrameCase(t, "font-select")
			m.Cursor = 0

			plain := fontSelectPlain(t, m, size.width, size.height)
			lines := strings.Split(plain, "\n")
			title := stripStepPrefix(m.GetScreenTitle())
			if top, _ := fontSelectCard(lines, title); top >= 0 {
				t.Fatalf("the card is drawn at %s, where it cannot hold its rows:\n%s", size.name, plain)
			}
			// The compact list is the same screen: the title and the answer under the
			// cursor are still drawn.
			if !strings.Contains(plain, title) {
				t.Errorf("the fallback at %s lost the page's title:\n%s", size.name, plain)
			}
			if !strings.Contains(plain, plainOptionName(m.GetCurrentOptions()[0])) {
				t.Errorf("the fallback at %s lost the answer under the cursor:\n%s", size.name, plain)
			}
		})
	}
}

// TestFontSelectKeepsTheNoteWhenTheBorderCannotHoldIt guards the one datum the card
// could drop: the font question's note is the font step's own measurement, and on a
// host it measured it is two sentences long -- longer than a card's border row can
// hold beside the chip. It moves inside the card instead of being given back,
// because the note is what states whether the family is already on the machine.
func TestFontSelectKeepsTheNoteWhenTheBorderCannotHoldIt(t *testing.T) {
	m := installerFrameCase(t, "font-select")
	m.FontPresence = fontPresence{Measured: true}
	description := m.GetScreenDescription()
	if !strings.Contains(description, "not installed here") {
		t.Fatalf("the measured host's description is not the note this guard reads: %q", description)
	}

	plain := fontSelectPlain(t, m, 80, 24)
	lines := strings.Split(plain, "\n")
	title := stripStepPrefix(m.GetScreenTitle())
	top, bottom := fontSelectCard(lines, title)
	if top < 0 {
		t.Fatalf("the card is not drawn:\n%s", plain)
	}
	// The note is wrapped inside the card, so its words are read from the card's own
	// body rather than from the frame's line breaks.
	bodyText := fontSelectBodyText(lines, top, bottom)
	for _, want := range []string{
		"Meslo Nerd Font is not installed here",
		"skipping it leaves the terminal without them",
	} {
		if !strings.Contains(bodyText, want) {
			t.Errorf("the card does not carry the note's %q in its body:\n%s\n(the model's description is %q)", want, plain, description)
		}
	}
}
