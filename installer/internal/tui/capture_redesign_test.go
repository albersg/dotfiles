package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// redesignedPages are the pages of the installer-redesign pass that the mockup does
// not draw: the wizard's questions, the update center, the doctor and its fix, the
// learn section's menus, tables and tool pages, the backup screens, the completion
// screen and the screens that end in a dead end. They are named here because a
// review of the pass reads them side by side, and the frame guards cover the rest of
// the program from the model rather than from a list.
var redesignedPages = []struct {
	name  string
	build func(*testing.T) Model
}{
	{"os-select", func(t *testing.T) Model { return installerFrameCase(t, "os-select") }},
	{"terminal-select", func(t *testing.T) Model { return installerFrameCase(t, "terminal-select") }},
	{"terminal-select-wsl", func(t *testing.T) Model { return installerFrameCase(t, "terminal-select-wsl") }},
	{"shell-select", func(t *testing.T) Model { return installerFrameCase(t, "shell-select") }},
	{"font-select", func(t *testing.T) Model { return installerFrameCase(t, "font-select") }},
	{"wm-select", func(t *testing.T) Model { return installerFrameCase(t, "wm-select") }},
	{"nvim-select", func(t *testing.T) Model { return installerFrameCase(t, "nvim-select") }},
	{"ghostty-warning", func(t *testing.T) Model { return installerFrameCase(t, "ghostty-warning") }},
	{"update-center", updateCenterFrameCase},
	{"doctor", doctorFrameCase},
	{"doctor-fix", doctorFixFrameCase},
	{"learn-tools", func(t *testing.T) Model { return installerFrameCase(t, "learn-tools") }},
	{"learn-terminals", func(t *testing.T) Model { return installerFrameCase(t, "learn-terminals") }},
	{"learn-terminals-info", func(t *testing.T) Model { return installerFrameCase(t, "learn-terminals-info") }},
	{"learn-tooling", func(t *testing.T) Model { return installerFrameCase(t, "learn-tooling") }},
	{"learn-plugins", func(t *testing.T) Model { return installerFrameCase(t, "learn-plugins") }},
	{"learn-toolset", func(t *testing.T) Model { return installerFrameCase(t, "learn-toolset") }},
	{"learn-font", func(t *testing.T) Model { return installerFrameCase(t, "learn-font") }},
	{"backup-confirm", func(t *testing.T) Model { return installerFrameCase(t, "backup-confirm") }},
	{"restore-backup", func(t *testing.T) Model { return installerFrameCase(t, "restore-backup") }},
	{"restore-confirm", func(t *testing.T) Model { return installerFrameCase(t, "restore-confirm") }},
	{"complete", func(t *testing.T) Model { return installerFrameCase(t, "complete") }},
	{"error", func(t *testing.T) Model { return installerFrameCase(t, "error") }},
	{"dead-end-tool", func(t *testing.T) Model {
		m := installerFrameCase(t, "learn-terminals")
		m.ViewingTool = "no-such-terminal"
		return m
	}},
	{"dead-end-restore", func(t *testing.T) Model {
		m := installerFrameCase(t, "restore-confirm")
		m.AvailableBackups = nil
		m.SelectedBackup = 0
		return m
	}},
}

// TestCaptureEveryRedesignedPage writes the frames of the pages this pass changed,
// at the 80x24 floor and at the wide frame a capture is reviewed in, so the layout
// of a slice can be read instead of assumed. It is opt-in and writes nothing unless
// DOTFILES_CAPTURE_DIR is set, exactly like TestCaptureEveryScreen: a test that
// writes files into the machine is not a check.
func TestCaptureEveryRedesignedPage(t *testing.T) {
	out := os.Getenv("DOTFILES_CAPTURE_DIR")
	if out == "" {
		t.Skip("set DOTFILES_CAPTURE_DIR to write the frames")
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}

	for _, page := range redesignedPages {
		page := page
		for _, size := range [][2]int{{80, 24}, {160, 50}} {
			m := page.build(t)
			m.Width, m.Height = size[0], size[1]
			m.Animating = false
			name := filepath.Join(out, fmt.Sprintf("%s-%dx%d.txt", page.name, size[0], size[1]))
			if err := os.WriteFile(name, []byte(m.View()), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}
