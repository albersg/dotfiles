package tui

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// Bodies are the eighth pass's probes, verbatim except for their names and the helper
// below: the pass ran them at the pre-fix baseline and at HEAD. Together they pin both
// directions of the dry-run rule: no route may overwrite the record of a real run, and no
// route may stop a real run from recording what it did.

// seedLastRealRun writes a record of a real run and returns its path and bytes, so a test
// can prove the bytes are untouched.
func seedLastRealRun(t *testing.T, home string) (string, []byte) {
	t.Helper()
	zsh := system.ConfigPaths()["zsh"]
	if zsh == "" {
		t.Skip("no zsh path")
	}
	backup := filepath.Join(home, ".dotfiles-backup-1", "zsh")
	if err := os.MkdirAll(backup, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backup, "zsh"), []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(zsh, []byte("after"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeLastInstall(lastInstall{
		Timestamp: time.Now(), Status: runStatusComplete, Files: []string{"zsh"},
		Entries: []installEntry{{Key: "zsh", Path: zsh, Action: installActionReplaced, Backup: filepath.Join(backup, "zsh")}},
		Changes: []fileChange{{Key: "zsh", Path: zsh, Action: fileActionModified, Before: fileDigest(filepath.Join(backup, "zsh")), After: fileDigest(zsh), Backup: filepath.Join(backup, "zsh")}},
	}); err != nil {
		t.Fatal(err)
	}
	rp := filepath.Join(home, "state", "dotfiles", "last-install.json")
	b, err := os.ReadFile(rp)
	if err != nil {
		t.Fatal(err)
	}
	return rp, b
}

// Every route that can finish a run is guarded: a dry run through any of them leaves the
// record of the last real run byte for byte as it was. The interface reaches the record by
// routes of its own, which is how one of these came to be missed.
func TestADryRunThroughEachRecordRouteLeavesTheRecord(t *testing.T) {
	routes := []struct {
		name string
		run  func(t *testing.T, m *Model)
	}{
		{"noninteractive-success", func(t *testing.T, m *Model) { _ = finishNonInteractiveRun(m, nil) }},
		{"noninteractive-failure", func(t *testing.T, m *Model) { _ = finishNonInteractiveRun(m, []error{errors.New("boom")}) }},
		{"interface-complete", func(t *testing.T, m *Model) { _, _ = m.Update(installCompleteMsg{totalTime: 0}) }},
		{"interface-step-failure", func(t *testing.T, m *Model) {
			m.Steps = []InstallStep{{ID: "zsh", Name: "Zsh"}}
			_, _ = m.Update(stepCompleteMsg{stepID: "zsh", err: errors.New("boom")})
		}},
		{"interface-exec-failure", func(t *testing.T, m *Model) {
			m.Steps = []InstallStep{{ID: "zsh", Name: "Zsh"}}
			_, _ = m.Update(execFinishedMsg{stepID: "zsh", err: errors.New("boom")})
		}},
	}
	for _, r := range routes {
		t.Run(r.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
			t.Setenv("DOTFILES_DRY_RUN", "1")
			rp, before := seedLastRealRun(t, home)
			m := NewModel()
			r.run(t, &m)
			after, err := os.ReadFile(rp)
			if err != nil {
				t.Fatalf("record gone: %v", err)
			}
			if string(after) != string(before) {
				t.Fatalf("dry run route %s replaced the record", r.name)
			}
		})
	}
}

// The other direction of that rule: the guard must not stop a real run from recording what
// it did, including the failed steps a reader needs to know about.
func TestARealRunStillRecordsWhatItDid(t *testing.T) {
	t.Run("noninteractive-success", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
		t.Setenv("DOTFILES_DRY_RUN", "0")
		m := NewModel()
		if err := finishNonInteractiveRun(&m, nil); err != nil {
			t.Fatal(err)
		}
		rec := readLastInstall()
		if rec == nil || rec.Status != runStatusComplete || len(rec.FailedSteps) != 0 {
			t.Fatalf("real noninteractive success record wrong: %+v", rec)
		}
	})
	t.Run("noninteractive-failure", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
		t.Setenv("DOTFILES_DRY_RUN", "0")
		m := NewModel()
		_ = finishNonInteractiveRun(&m, []error{errors.New("boom")})
		rec := readLastInstall()
		if rec == nil || rec.Status != runStatusFailed || len(rec.FailedSteps) != 1 {
			t.Fatalf("real noninteractive failure record wrong: %+v", rec)
		}
	})
	t.Run("interface-complete", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
		t.Setenv("DOTFILES_DRY_RUN", "0")
		m := NewModel()
		_, _ = m.Update(installCompleteMsg{totalTime: 0})
		rec := readLastInstall()
		if rec == nil || rec.Status != runStatusComplete {
			t.Fatalf("real interface completion record wrong: %+v", rec)
		}
	})
	t.Run("interface-step-failure", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
		t.Setenv("DOTFILES_DRY_RUN", "0")
		m := NewModel()
		m.Steps = []InstallStep{{ID: "zsh", Name: "Zsh"}}
		_, _ = m.Update(stepCompleteMsg{stepID: "zsh", err: errors.New("boom")})
		rec := readLastInstall()
		if rec == nil || rec.Status != runStatusFailed || len(rec.FailedSteps) != 1 || rec.FailedSteps[0] != "Zsh" {
			t.Fatalf("real interface failure record wrong: %+v", rec)
		}
	})
}

// A run stopped with ctrl+c leaves the record of the previous run alone, whether or not a
// dry run is in progress: the command that stopped it is the one that knows.
func TestAStoppedRunLeavesThePreviousRecordAlone(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	t.Setenv("DOTFILES_DRY_RUN", "0")
	rp, before := seedLastRealRun(t, home)
	m := NewModel()
	m.Screen = ScreenInstalling
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	after, err := os.ReadFile(rp)
	if err != nil {
		t.Fatalf("record gone: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("a stopped run rewrote the record")
	}
}
