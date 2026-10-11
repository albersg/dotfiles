package tui

import (
	"testing"

	"github.com/charmbracelet/x/exp/golden"
)

// The installer's own chrome is adaptive, and the layout goldens beside this
// file are captured with no colour profile -- they pin where every row lands and
// nothing about the palette. That leaves the palette the frame wears unpinned:
// the header tab, its brand rule, the selected row's bar and the footer's keys
// could all fall back to plain text and every other golden would still pass.
//
// These two snapshots force 24-bit colour and pin the shipped chrome, so a
// change that drops the theme from the frame is caught the way a row change is.
// The model is isolated exactly as the layout goldens are (HOME, XDG_STATE_HOME,
// the greeting time and the detected platform are all pinned), so the bytes
// depend on the palette and the code and on nothing the machine happens to hold.

// TestMainMenuTruecolorGolden pins the frame's new visual language at the 80x24
// floor: the filled header tab, the brand rule under it, the selected row's bar
// with its accent marker and the footer's accent keys.
func TestMainMenuTruecolorGolden(t *testing.T) {
	skipIfTermux(t)
	forceTrueColor(t)

	m := NewModel()
	isolateGoldenTest(t, &m)
	pinAvailableUpdate(&m)
	m.Width, m.Height = 80, 24
	m.Screen = ScreenMainMenu
	m.Animating = false

	golden.RequireEqual(t, []byte(m.View()))
}

// TestInstallingTruecolorGolden pins the screen the run is watched through: the
// brand progress bar with its bold percentage, the current step in the brand
// tone, and the rail's state glyphs and meters.
func TestInstallingTruecolorGolden(t *testing.T) {
	skipIfTermux(t)
	forceTrueColor(t)

	m := installerFrameCase(t, "installing-live")
	m.Width, m.Height = 80, 24
	m.Animating = false
	m.AnimTick = 0

	golden.RequireEqual(t, []byte(m.View()))
}

// TestCompleteTruecolorGolden pins the one frame the installer makes
// celebratory: the completion crown in the success tone, the check before the
// summary label, and the primary action the footer names.
func TestCompleteTruecolorGolden(t *testing.T) {
	skipIfTermux(t)
	forceTrueColor(t)

	m := installerFrameCase(t, "complete")
	m.Width, m.Height = 80, 24
	m.Animating = false

	golden.RequireEqual(t, []byte(m.View()))
}
