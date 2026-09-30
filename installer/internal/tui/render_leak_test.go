package tui

import (
	"regexp"
	"strings"
	"testing"
)

// sgrEscape matches a Select Graphic Rendition sequence: the escape that sets a
// colour, a weight or an underline.
var sgrEscape = regexp.MustCompile("\x1b\\[([0-9;]*)m")

// TestNoRenderedLineLeavesAColourActive is the guard for a defect no snapshot could
// see: the companion's shaded sprite wrote a space over a colour it had set and never
// retired, so the tone - including a background, because half the sprite's cells set
// one - ran through the rest of the row and, since a terminal's escape state outlives
// the line, through the rows after it. On a real terminal the result was a solid bar
// of colour across the screen; in a snapshot the bytes matched, because a snapshot
// compares the escapes it is given and the file had been generated from the same
// mistake. The previous check could not see it either: stripping the escapes - which
// is how the shape of the ink was verified - removes exactly the evidence.
//
// So this asserts the property that actually matters to a terminal: at the end of
// every rendered line, no style is left active. A line may carry any escapes it
// likes; the last one has to be a reset, or there are none.
func TestNoRenderedLineLeavesAColourActive(t *testing.T) {
	sizes := []struct{ width, height int }{
		{80, 24}, {100, 30}, {124, 24}, {160, 50}, {227, 62},
	}

	checked := 0
	for _, name := range installerFrameScreenNames {
		for _, size := range sizes {
			m := installerFrameCase(t, name)
			m.Width, m.Height = size.width, size.height
			m.Animating = true
			m.PixelSprite = true

			for i, line := range strings.Split(m.View(), "\n") {
				matches := sgrEscape.FindAllStringSubmatch(line, -1)
				if len(matches) == 0 {
					continue
				}
				last := matches[len(matches)-1][1]
				if last == "" || last == "0" {
					continue
				}
				t.Errorf("%s at %dx%d line %d ends with a style still active (%q): the colour will bleed "+
					"through every cell after it and through the lines below.\nline: %q",
					name, size.width, size.height, i+1, last, line)
			}
			checked++
		}
	}

	if checked == 0 {
		t.Fatal("no screens were checked, so this guard proves nothing")
	}
}
