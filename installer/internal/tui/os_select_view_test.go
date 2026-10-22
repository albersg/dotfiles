package tui

import (
	"strings"
	"testing"
)

// ============================================================================
// THE OS SELECT SCREEN'S CARD — the guards
// ============================================================================
//
// The OS question is the first wizard picker drawn in the reference grammar
// (os_select_view.go): the page IS the card, its title a chip set into the top
// border with the model's description as the caption beside it, and each answer a
// row of icon, title, subtitle and chevron with the cursor's row filled. The
// guards below hold the four things that makes true: the card is the shared one,
// the rows are still the model's own answers in its own order, the keys and the
// cursor are still the screen's, and a frame that cannot hold the card gets the
// compact list instead of a card with a fact cut off.

// osSelectPlain renders the OS question and returns the frame without escapes,
// so a guard reads the words a person reads.
func osSelectPlain(t *testing.T, width, height int) string {
	t.Helper()
	m := installerFrameCase(t, "os-select")
	m.Width, m.Height = width, height
	m.Animating = false
	return ansiEscape.ReplaceAllString(m.View(), "")
}

// TestOSSelectDrawsTheReferenceGrammar is the shape guard: the screen's own title
// is the chip on the card's top border, the model's own description is the caption
// on that same row, the card closes under its rows, and every answer carries the
// grammar's icon, subtitle and chevron. It is measured at the floor and at the
// two-column frame, because the card is the shape at both.
func TestOSSelectDrawsTheReferenceGrammar(t *testing.T) {
	for _, size := range []struct {
		name          string
		width, height int
	}{
		{"80x24, the floor", 80, 24},
		{"160x50, two columns", 160, 50},
	} {
		size := size
		t.Run(size.name, func(t *testing.T) {
			plain := osSelectPlain(t, size.width, size.height)
			lines := strings.Split(plain, "\n")

			title := stripStepPrefix((Model{Screen: ScreenOSSelect}).GetScreenTitle())
			top := -1
			for i, line := range lines {
				if strings.Contains(line, "╭─ "+title+" ") {
					top = i
					break
				}
			}
			if top < 0 {
				t.Fatalf("the OS question does not carry %q as its card's chip at %s:\n%s", title, size.name, plain)
			}
			// The caption rides the chip's row, as the reference draws it.
			if caption := "Detected: Linux"; !strings.Contains(lines[top], caption) {
				t.Errorf("the card's top border does not carry the model's own %q at %s: %q", caption, size.name, lines[top])
			}

			bottom := -1
			for i := top; i < len(lines); i++ {
				if strings.Contains(lines[i], "╰") {
					bottom = i
				}
			}
			if bottom < top {
				t.Fatalf("the card at %s has no closing border:\n%s", size.name, plain)
			}

			// The answers are the grammar's rows: the title, the subtitle under it,
			// and the chevron that says the row opens something. They are drawn
			// inside the card, so a row the border clipped cannot pass.
			m := installerFrameCase(t, "os-select")
			for _, option := range m.GetCurrentOptions() {
				_, subtitle := osSelectRowParts(plainOptionName(option))
				for _, want := range []string{plainOptionName(option), subtitle} {
					if !rowAppearsBetween(lines, want, top, bottom) {
						t.Errorf("the card at %s does not draw %q between its borders (rows %d-%d):\n%s",
							size.name, want, top, bottom, plain)
					}
				}
				if !rowAppearsBetween(lines, grammarChevron, top, bottom) {
					t.Errorf("the card at %s draws no chevron for %q:\n%s", size.name, option, plain)
				}
			}

			// The footer keeps the keys the screen really answers to.
			for _, want := range []string{"↑/k", "↓/j", "[Enter]", "[Esc]"} {
				if !strings.Contains(plain, want) {
					t.Errorf("the OS question at %s no longer names %q in its legend:\n%s", size.name, want, plain)
				}
			}
		})
	}
}

// TestOSSelectRowsAreTheModelsAnswersInOrder is the reachability guard: the card
// draws exactly m.GetCurrentOptions(), in the order the key handler reads them,
// with no answer added, renamed or reordered, and the cursor's row is the one with
// the marker. A row the screen stopped drawing would be an answer Enter could
// still record from a row nobody can see.
func TestOSSelectRowsAreTheModelsAnswersInOrder(t *testing.T) {
	m := installerFrameCase(t, "os-select")
	m.Width, m.Height = 80, 24
	m.Animating = false

	options := m.GetCurrentOptions()
	if len(options) == 0 {
		t.Fatal("the OS question offers no answers to draw")
	}

	for cursor := 0; cursor < len(options); cursor++ {
		moved := m
		moved.Cursor = cursor
		plain := ansiEscape.ReplaceAllString(moved.View(), "")

		// The answers are read from the card's own rows, not from the whole frame: the
		// caption on the top border also names the detected platform, so a search over
		// every line would find "Linux" in "Detected: Linux" before the first row.
		lines := strings.Split(plain, "\n")
		top := 0
		for i, line := range lines {
			if strings.Contains(line, "╭─ "+stripStepPrefix(m.GetScreenTitle())+" ") {
				top = i
				break
			}
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

		// One marker, on the row the cursor is on: the grammar's selected row is a
		// filled bar carrying the cursor's mark, so the state is not the bar's colour
		// alone.
		if got := strings.Count(plain, grammarMarker); got != 1 {
			t.Fatalf("the card draws %d cursor markers with the cursor on %d, want exactly one:\n%s", got, cursor, plain)
		}
		markerRow := ""
		for _, line := range strings.Split(plain, "\n") {
			if strings.Contains(line, grammarMarker) {
				markerRow = line
			}
		}
		if !strings.Contains(markerRow, plainOptionName(options[cursor])) {
			t.Errorf("the marker is not on the answer under the cursor (%q): %q", options[cursor], markerRow)
		}
	}
}

// TestOSSelectLeavesEnterUnnamedWithNoRowUnderTheCursor keeps the screen's own key
// rule: with noSelection no row is marked, and the legend must not advertise a
// press that would record nothing. It is read from the rendered frame, so the card
// cannot rename a key the handler still answers to.
func TestOSSelectLeavesEnterUnnamedWithNoRowUnderTheCursor(t *testing.T) {
	m := installerFrameCase(t, "os-select")
	m.Width, m.Height = 80, 24
	m.Animating = false
	m.Cursor = noSelection

	plain := ansiEscape.ReplaceAllString(m.View(), "")
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

// TestOSSelectFallsBackToTheCompactListWhenTheCardCannotFit pins the floor: a
// frame too narrow or too short for the card draws the compact list instead --
// the same answers, the page's title and description above them -- rather than a
// card that leaves a fact on the cutting-room floor.
func TestOSSelectFallsBackToTheCompactListWhenTheCardCannotFit(t *testing.T) {
	for _, size := range []struct {
		name          string
		width, height int
	}{
		{"36x24, too narrow for a two-line row", 36, 24},
		{"80x12, too few rows for the card", 80, 12},
	} {
		size := size
		t.Run(size.name, func(t *testing.T) {
			m := installerFrameCase(t, "os-select")
			m.Width, m.Height = size.width, size.height
			m.Animating = false
			m.Cursor = osOptionLinux

			plain := ansiEscape.ReplaceAllString(m.View(), "")
			if strings.Contains(plain, "╭─ "+stripStepPrefix(m.GetScreenTitle())+" ") {
				t.Fatalf("the card is drawn at %s, where it cannot hold its rows:\n%s", size.name, plain)
			}
			// The compact list is the same screen: the title, the description and the
			// answer under the cursor are still drawn.
			if !strings.Contains(plain, stripStepPrefix(m.GetScreenTitle())) {
				t.Errorf("the fallback at %s lost the page's title:\n%s", size.name, plain)
			}
			if !strings.Contains(plain, "Linux") {
				t.Errorf("the fallback at %s lost the answer under the cursor:\n%s", size.name, plain)
			}
		})
	}
}

// TestOSSelectKeepsTheDescriptionWhenTheBorderCannotHoldIt guards the one datum
// the card could drop: on a host where detection found nothing, the description is
// the sentence that says so, and it is longer than a 74-column card's border row
// can hold beside the chip. It moves inside the card instead of being given back.
func TestOSSelectKeepsTheDescriptionWhenTheBorderCannotHoldIt(t *testing.T) {
	m := installerFrameCase(t, "os-select")
	m.Width, m.Height = 80, 24
	m.Animating = false
	m.SystemInfo = nil

	description := m.GetScreenDescription()
	if !strings.Contains(description, "not detected") {
		t.Fatalf("this guard wants the undetected host's sentence, got %q", description)
	}

	plain := ansiEscape.ReplaceAllString(m.View(), "")
	if !strings.Contains(plain, description) {
		t.Errorf("the card dropped the screen's own %q:\n%s", description, plain)
	}
	if !strings.Contains(plain, "╭─ "+stripStepPrefix(m.GetScreenTitle())+" ") {
		t.Errorf("the undetected host does not draw the card:\n%s", plain)
	}
}
