package tui

import (
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/tui/trainer"
)

// TestMeaningSurvivesWithoutColour guards the promise that nothing the installer
// means is carried by colour alone. Termux is a supported terminal, a 16-colour
// profile is a supported profile, and a piped run has no colour at all: in each of
// them a state a user has to recognise - where the selection is, how far the
// installation has got - has to survive as characters.
//
// The property is deliberately about the characters and not about a particular
// glyph: the marker that says "here" may be redrawn, and the guard should keep
// passing while the two states stay tellable apart. It fails in the two ways this
// actually breaks:
//
//   - the two states render the same characters, so colour is the only
//     difference and a colourless terminal shows nothing;
//   - the two states differ only in trailing padding, which is the same failure
//     wearing a disguise: a bar whose colour is its only content is a colour.
func TestMeaningSurvivesWithoutColour(t *testing.T) {
	cases := []struct {
		name string
		a, b Model
	}{
		{
			"the main menu, with the first option selected against the second",
			installerMenuAt(t, ScreenMainMenu, 0),
			installerMenuAt(t, ScreenMainMenu, 1),
		},
		{
			"the operating system list, first entry against second",
			installerMenuAt(t, ScreenOSSelect, 0),
			installerMenuAt(t, ScreenOSSelect, 1),
		},
		{
			"the shell list, first entry against second",
			installerMenuAt(t, ScreenShellSelect, 0),
			installerMenuAt(t, ScreenShellSelect, 1),
		},
		{
			"the trainer's module list, first module against second",
			trainerMenuAt(t, 0),
			trainerMenuAt(t, 1),
		},
		{
			"the installation meter, empty against complete",
			installingWithProgress(t, 0),
			installingWithProgress(t, 1),
		},
	}

	if len(cases) == 0 {
		t.Fatal("no states to compare, so this guard proves nothing")
	}

	for _, c := range cases {
		plainA, plainB := colourlessView(c.a), colourlessView(c.b)
		if plainA == plainB {
			t.Errorf("%s draws the same characters: a terminal that cannot show colour cannot tell them apart",
				c.name)
			continue
		}
		if trimTrailingLines(plainA) == trimTrailingLines(plainB) {
			t.Errorf("%s differs only in trailing padding, so the difference is the width of a coloured bar "+
				"and nothing a colourless terminal can read", c.name)
		}
	}
}

// installerMenuAt builds a list screen at the supported floor with one entry
// selected.
func installerMenuAt(t *testing.T, screen Screen, cursor int) Model {
	t.Helper()
	m := installerFrameModel(t, screen)
	m.Width, m.Height = trainerFrameWidth, trainerFrameHeight
	m.Cursor = cursor
	return m
}

// trainerMenuAt builds the trainer's module list with one module selected.
func trainerMenuAt(t *testing.T, cursor int) Model {
	t.Helper()
	m := installerFrameModel(t, ScreenTrainerMenu)
	m.Width, m.Height = trainerFrameWidth, trainerFrameHeight
	m.TrainerModules = trainer.GetAllModules()
	m.TrainerCursor = cursor
	return m
}

// installingWithProgress builds the installation screen with one running step at
// the given fraction, which is what the meter under it draws.
func installingWithProgress(t *testing.T, progress float64) Model {
	t.Helper()
	m := installerFrameModel(t, ScreenInstalling)
	m.Width, m.Height = trainerFrameWidth, trainerFrameHeight
	m.Steps = []InstallStep{
		{ID: "deps", Name: "Install Dependencies", Status: StatusRunning, Progress: progress},
	}
	return m
}

// colourlessView is the view with every escape sequence removed, which is what a
// terminal without colour, or a redirected run, ends up showing.
func colourlessView(m Model) string {
	return ansiEscape.ReplaceAllString(m.View(), "")
}

// trimTrailingLines strips the padding at the end of each line, so a difference
// that exists only there cannot pass for content.
func trimTrailingLines(view string) string {
	lines := strings.Split(view, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	return strings.Join(lines, "\n")
}
