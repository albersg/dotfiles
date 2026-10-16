package tui

import (
	"os"
	"path/filepath"
	"strings"
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

// The other direction, and a case a person can produce: the run's copy of the key holds a
// link at this path and the machine points it somewhere else, so the machine is not where
// the run found it. This report has to survive making a dead link quiet, and it has to name
// the path and what the copy holds.
func TestALinkTheRunDidNotLeaveWhereItsCopyHasItIsReported(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))

	nvim := system.ConfigPaths()["nvim"]
	if nvim == "" {
		t.Skip("this machine has no nvim configuration path")
	}
	mustWrite(t, os.MkdirAll(nvim, 0o755))
	link := filepath.Join(nvim, "current")
	mustWrite(t, os.Symlink(filepath.Join(home, "one"), link))
	mustWrite(t, os.WriteFile(filepath.Join(nvim, "init.lua"), []byte("two"), 0o644))

	backup := filepath.Join(home, ".dotfiles-backup-1", "nvim")
	mustWrite(t, os.MkdirAll(backup, 0o755))
	mustWrite(t, os.WriteFile(filepath.Join(backup, "init.lua"), []byte("one"), 0o644))
	mustWrite(t, os.Symlink(filepath.Join(home, "one"), filepath.Join(backup, "current")))

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

	// The machine's link now points somewhere else, and no change names it.
	mustWrite(t, os.Remove(link))
	mustWrite(t, os.Symlink(filepath.Join(home, "two"), link))

	report, err := RevertLastRun(RevertOptions{})
	if err == nil || report.Complete() {
		t.Fatalf("the machine's link is not the one the copy holds, yet the revert answered %v (%+v)", err, report.Files)
	}
	if !strings.Contains(err.Error(), link) || !strings.Contains(err.Error(), "which the run's copy points at") {
		t.Fatalf("the report does not name the path and what the copy holds: %v", err)
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

// A dead link the key already held, with no backup taken, was where the run found it:
// removing the file the run created restores the key, so reporting the link as one nothing
// can account for is a false alarm. The record has no copy of the link, and this installer
// never creates one -- a missing copy is not evidence that the run wrote it -- so a
// legitimate revert must not fail.
func TestAPreExistingDeadLinkInsideAKeyWithoutABackupIsNotAFalseAlarm(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))

	nvim := system.ConfigPaths()["nvim"]
	if nvim == "" {
		t.Skip("this machine has no nvim configuration path")
	}
	mustWrite(t, os.MkdirAll(nvim, 0o755))
	mustWrite(t, os.WriteFile(filepath.Join(nvim, "init.lua"), []byte("one"), 0o644))
	mustWrite(t, os.Symlink(filepath.Join(home, "nowhere"), filepath.Join(nvim, "stale")))

	m := NewModel()
	m.ExistingConfigs = []string{"nvim: " + nvim}
	m.ConfigFingerprints = configFingerprints()
	m.ConfigFileIndex = configFileIndex()

	mustWrite(t, os.WriteFile(filepath.Join(nvim, "new.lua"), []byte("new"), 0o644))
	m.recordRun(runStatusComplete, nil)

	report, err := RevertLastRun(RevertOptions{})
	if err != nil || !report.Complete() {
		t.Fatalf("the run created one file and the key already held its dead link; removing it restores the key, but the revert answered %v (%+v)", err, report.Files)
	}
	if _, statErr := os.Lstat(filepath.Join(nvim, "new.lua")); !os.IsNotExist(statErr) {
		t.Fatalf("the created file the record names is still there: %v", statErr)
	}
}
