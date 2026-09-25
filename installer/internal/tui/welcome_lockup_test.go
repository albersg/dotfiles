package tui

import (
	"regexp"
	"strings"
	"testing"
)

var ansiEscape = regexp.MustCompile("\x1b\\[[0-9;]*[a-zA-Z]")

// welcomeView renders the welcome screen at the given terminal height with the
// escape sequences removed, so tests can measure the rendered geometry.
func welcomeView(height int) string {
	m := NewModel()
	m.SystemInfo = goldenSystemInfo()
	m.Width = 120
	m.Height = height
	m.Screen = ScreenWelcome
	return ansiEscape.ReplaceAllString(m.View(), "")
}

// TestWelcomeLockupFitsWhenFullEmblemIsChosen pins welcomeFullLockupHeight to
// the real height of the full lockup. The constant selects the version with the
// wordmark, and CenterBoth clips the top of anything taller than the frame, so
// a constant that is too small silently crops the emblem. Reading the constant
// cannot catch that after the art changes height, which is exactly what
// happened when the emblem dropped from 15 rows to 13.
func TestWelcomeLockupFitsWhenFullEmblemIsChosen(t *testing.T) {
	t.Run("at the constant the full lockup fits and keeps its wordmark", func(t *testing.T) {
		out := welcomeView(welcomeFullLockupHeight)
		if !strings.Contains(out, "╗") {
			t.Error("the welcome screen dropped the ASCII wordmark at welcomeFullLockupHeight: the full lockup is taller than the constant says")
		}
		last := lastContentRow(out)
		if last > welcomeFullLockupHeight-1 {
			t.Errorf("the lockup ends on row %d but the frame is %d rows tall: the top of the emblem is clipped", last, welcomeFullLockupHeight)
		}
	})

	t.Run("one row less falls back to the compact emblem", func(t *testing.T) {
		out := welcomeView(welcomeFullLockupHeight - 1)
		if strings.Contains(out, "╗") {
			t.Error("the wordmark still renders one row below the constant: welcomeFullLockupHeight is larger than needed")
		}
	})
}

// lastContentRow returns the index of the last row that has visible content.
func lastContentRow(out string) int {
	last := -1
	for i, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) != "" {
			last = i
		}
	}
	return last
}
