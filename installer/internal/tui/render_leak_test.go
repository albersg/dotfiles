package tui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/tui/trainer"
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
			checkRenderedLineStyles(t, name, size.width, size.height, m.View())
			checked++
		}
	}

	for _, name := range trainerLeakScreenNames {
		for _, size := range sizes {
			m := trainerLeakFrameCase(t, name)
			m.Width, m.Height = size.width, size.height
			m.Animating = true
			m.PixelSprite = true
			checkRenderedLineStyles(t, name, size.width, size.height, m.View())
			checked++
		}
	}

	if checked == 0 {
		t.Fatal("no screens were checked, so this guard proves nothing")
	}
}

func checkRenderedLineStyles(t *testing.T, name string, width, height int, view string) {
	t.Helper()
	for i, line := range strings.Split(view, "\n") {
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
			name, width, height, i+1, last, line)
	}
}

var trainerLeakScreenNames = []string{
	"trainer-menu", "trainer-lesson", "trainer-practice", "trainer-boss",
	"trainer-result", "trainer-boss-result",
}

func trainerLeakFrameCase(t *testing.T, name string) Model {
	t.Helper()
	m := newTrainerFrameModel(t, trainer.ModuleHorizontal)
	switch name {
	case "trainer-menu":
		m.Screen = ScreenTrainerMenu
		m.TrainerModules = trainer.GetAllModules()
	case "trainer-lesson":
		m.Screen = ScreenTrainerLesson
	case "trainer-practice":
		m.Screen = ScreenTrainerPractice
	case "trainer-boss":
		m.Screen = ScreenTrainerBoss
		m.TrainerGameState.StartBoss(trainer.ModuleHorizontal)
	case "trainer-result":
		m.Screen = ScreenTrainerResult
	case "trainer-boss-result":
		m.TrainerGameState.StartBoss(trainer.ModuleHorizontal)
		m.Screen = ScreenTrainerBossResult
	default:
		t.Fatalf("unknown trainer leak screen %q", name)
	}
	return m
}
