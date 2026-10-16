package tui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// A key that is a symlink to a directory is a tree the run writes through, and the
// record now names what the run wrote inside it. When the only change is a file the
// run created, removing that file puts the key back exactly, so reporting the key as
// one this command cannot act on is a false alarm -- and says "could not be put back"
// about a run whose every change was put back.
func TestARevertOfASymlinkedKeyIsNotAFalseAlarmWhenOnlyCreatedFilesChanged(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))

	nvim := system.ConfigPaths()["nvim"]
	if nvim == "" {
		t.Skip("this machine has no nvim configuration path")
	}
	real := filepath.Join(home, "repo", "nvim")
	mustWrite(t, os.MkdirAll(real, 0o755))
	mustWrite(t, os.WriteFile(filepath.Join(real, "init.lua"), []byte("one"), 0o644))
	mustWrite(t, os.MkdirAll(filepath.Dir(nvim), 0o755))
	mustWrite(t, os.Symlink(real, nvim))

	m := NewModel()
	m.ExistingConfigs = []string{"nvim: " + nvim}
	m.ConfigFingerprints = configFingerprints()
	m.ConfigFileIndex = configFileIndex()

	mustWrite(t, os.WriteFile(filepath.Join(real, "new.lua"), []byte("new"), 0o644))
	m.recordRun(runStatusComplete, nil)

	report, err := RevertLastRun(RevertOptions{})
	if err != nil || !report.Complete() {
		t.Fatalf("the only change was a created file, and removing it restores the key; got %v (%+v)", err, report.Files)
	}
	if _, statErr := os.Lstat(filepath.Join(real, "new.lua")); !os.IsNotExist(statErr) {
		t.Fatalf("the created file the record names is still there: %v", statErr)
	}
}

// A link that points at nothing and that the run's copy of the key does not hold is a
// path the record cannot speak for: the run may have made it. A revert that calls
// itself complete while it is still there leaves a path the run wrote behind.
func TestARevertReportsADanglingLinkTheRunCreated(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))

	nvim := system.ConfigPaths()["nvim"]
	if nvim == "" {
		t.Skip("this machine has no nvim configuration path")
	}
	mustWrite(t, os.MkdirAll(nvim, 0o755))
	broken := filepath.Join(nvim, "broken")
	mustWrite(t, os.Symlink(filepath.Join(home, "nowhere"), broken))
	mustWrite(t, os.WriteFile(filepath.Join(nvim, "init.lua"), []byte("two"), 0o644))

	backup := filepath.Join(home, ".dotfiles-backup-1", "nvim")
	mustWrite(t, os.MkdirAll(backup, 0o755))
	mustWrite(t, os.WriteFile(filepath.Join(backup, "init.lua"), []byte("one"), 0o644))

	mustWrite(t, writeLastInstall(lastInstall{
		Timestamp: time.Now(),
		Files:     []string{"nvim: " + nvim},
		Entries:   []installEntry{{Key: "nvim", Path: nvim, Action: installActionReplaced, Backup: backup}},
		Changes: []fileChange{{
			Key: "nvim", Path: filepath.Join(nvim, "init.lua"), Action: fileActionModified,
			Before: fileDigest(filepath.Join(backup, "init.lua")),
			After:  fileDigest(filepath.Join(nvim, "init.lua")),
			Backup: filepath.Join(backup, "init.lua"),
		}},
	}))

	report, err := RevertLastRun(RevertOptions{})
	if err == nil && report.Complete() {
		if _, statErr := os.Lstat(broken); statErr == nil {
			t.Fatalf("the revert calls itself complete while the link the run made, %s, is still there", broken)
		}
	}
}

// The other side of that rule: a link that points at nothing and that the run's own
// copy holds, pointing at nothing, is a link the run found. Reporting it would turn
// every stale link a person already had into an alarm.
func TestADanglingLinkTheKeyHeldBeforeIsNotReported(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))

	nvim := system.ConfigPaths()["nvim"]
	if nvim == "" {
		t.Skip("this machine has no nvim configuration path")
	}
	mustWrite(t, os.MkdirAll(nvim, 0o755))
	stale := filepath.Join(nvim, "stale")
	mustWrite(t, os.Symlink(filepath.Join(home, "nowhere"), stale))
	mustWrite(t, os.WriteFile(filepath.Join(nvim, "init.lua"), []byte("two"), 0o644))

	backup := filepath.Join(home, ".dotfiles-backup-1", "nvim")
	mustWrite(t, os.MkdirAll(backup, 0o755))
	mustWrite(t, os.WriteFile(filepath.Join(backup, "init.lua"), []byte("one"), 0o644))
	mustWrite(t, os.Symlink(filepath.Join(home, "nowhere"), filepath.Join(backup, "stale")))

	mustWrite(t, writeLastInstall(lastInstall{
		Timestamp: time.Now(),
		Files:     []string{"nvim: " + nvim},
		Entries:   []installEntry{{Key: "nvim", Path: nvim, Action: installActionReplaced, Backup: backup}},
		Changes: []fileChange{{
			Key: "nvim", Path: filepath.Join(nvim, "init.lua"), Action: fileActionModified,
			Before: fileDigest(filepath.Join(backup, "init.lua")),
			After:  fileDigest(filepath.Join(nvim, "init.lua")),
			Backup: filepath.Join(backup, "init.lua"),
		}},
	}))

	report, err := RevertLastRun(RevertOptions{})
	if err != nil || !report.Complete() {
		t.Fatalf("the key held this dead link before the run and the copy says so; got %v (%+v)", err, report.Files)
	}
}
