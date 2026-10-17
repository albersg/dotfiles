package tui

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// A run stopped with ctrl+c failed at nothing: the person stopped it. No record is
// written, which is what the documentation now promises -- and the promise that
// goes with it is that the way back does not need one, because the backup step runs
// before any step can write a configuration file.
func TestAnAbortedRunLeavesNoRecord(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))

	m := NewModel()
	m.Screen = ScreenInstalling
	m.ConfigFingerprints = configFingerprints()
	m.ConfigFileIndex = configFileIndex()
	m.Steps = []InstallStep{{ID: "homebrew", Name: "Install Homebrew", Status: StatusRunning}}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	after := updated.(Model)
	if !after.Quitting {
		t.Error("ctrl+c with a step running did not stop the run")
	}

	if _, err := os.Stat(lastInstallPath()); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the abort left a record at %s; nothing failed, so nothing is recorded", lastInstallPath())
	}
}

// The other half of that promise: when a run takes a backup, it is the first thing
// the run does, so a machine whose install was stopped still has the state it
// started from. The steps that follow it -- a checkout, packages, the
// configuration -- all come after the copy exists.
func TestARunThatTakesABackupTakesItFirst(t *testing.T) {
	for _, osName := range []string{"linux", "mac", "wsl"} {
		steps := planFor(planOptions{CreateBackup: true, ConfigCount: 2, OS: osName})
		if len(steps) == 0 {
			t.Fatalf("a run on %s plans no steps", osName)
		}
		if steps[0].ID != "backup" {
			t.Errorf("the first step of a run on %s is %q; the copy of what it may replace has to be taken before anything else",
				osName, steps[0].ID)
		}
	}
}
