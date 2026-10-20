package tui

import (
	"fmt"
	"os"
	"regexp"
	"testing"
)

var scratchANSI = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]`)

func TestScratchScreens(t *testing.T) {
	if os.Getenv("SCRATCH_FRAMES") == "" {
		t.Skip("set SCRATCH_FRAMES=1")
	}
	for _, size := range [][2]int{{80, 24}, {160, 40}, {200, 55}} {
		for _, tc := range []struct {
			name  string
			build func(*testing.T) Model
		}{
			{"utilities", func(t *testing.T) Model { m := utilitiesFrameCase(t); m.Cursor = 4; return m }},
			{"updatecenter", func(t *testing.T) Model { return updateCenterFrameCase(t) }},
			{"diskclean", func(t *testing.T) Model { return diskCleanerFrameCase(t) }},
			{"doctor", func(t *testing.T) Model { return doctorFrameCase(t) }},
			{"theme", func(t *testing.T) Model { return themePickerFrameCase(t) }},
		} {
			m := tc.build(t)
			m.Width, m.Height = size[0], size[1]
			m.Animating = false
			os.WriteFile(fmt.Sprintf("/tmp/scratch-%s-%dx%d.txt", tc.name, size[0], size[1]), []byte(scratchANSI.ReplaceAllString(m.View(), "")), 0o644)
		}
	}
}
