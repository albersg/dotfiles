package tui

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// A step that runs the captured script reports through execFinishedMsg rather than
// through a finished step carrying a status. By the time such a step fails, earlier
// steps have already rewritten configuration files, so this is one of the runs whose
// changes are on the machine: it has to leave a record like every other exit path.
func TestAFailedInteractiveStepStillWritesTheRecord(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))

	current := system.ConfigPaths()["zsh"]
	if err := os.WriteFile(current, []byte("before the run"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := NewModel()
	m.ConfigFingerprints = configFingerprints()
	m.ConfigFileIndex = configFileIndex()
	m.Steps = []InstallStep{{ID: "setshell", Name: "Set Default Shell", Status: StatusRunning, Interactive: true}}
	m.Screen = ScreenInstalling

	if err := os.WriteFile(current, []byte("what the run already wrote"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.Update(execFinishedMsg{stepID: "setshell", err: errors.New("chsh failed")})

	record := readLastInstall()
	if record == nil {
		t.Fatalf("an interactive step failure left no record although the run had already rewritten %s", current)
	}
	if record.Status != runStatusFailed {
		t.Errorf("the record says the run was %q; that run failed at the shell step", record.Status)
	}
	if len(record.Changes) != 1 || record.Changes[0].Path != current {
		t.Fatalf("the record names %d changes; the run had rewritten %s", len(record.Changes), current)
	}
	if record.Changes[0].Before == record.Changes[0].After {
		t.Errorf("the record names the same digest before and after the run: %q", record.Changes[0].After)
	}
}
