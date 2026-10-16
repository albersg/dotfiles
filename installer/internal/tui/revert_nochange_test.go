package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// A completed run that changed no configuration file writes entries (found, or
// backed up when the backup step took a copy) and no per-file changes. It is not a
// legacy record: the machine already holds what that run left, so a revert has
// nothing to put back. Reporting every path unrevertible and exiting 1 would be a
// false alarm on the most ordinary state a machine can be in -- a second run that
// found everything already in place.
func TestANoChangeRecordIsAlreadyInPlaceNotUnrevertible(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))

	zsh := system.ConfigPaths()["zsh"]
	if err := os.WriteFile(zsh, []byte("unchanged"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := NewModel()
	m.ExistingConfigs = []string{"zsh: " + zsh}
	m.ConfigFingerprints = configFingerprints()
	m.ConfigFileIndex = configFileIndex()
	m.recordRun(runStatusComplete, nil)

	record := readLastInstall()
	if record == nil || len(record.Changes) != 0 || len(record.Entries) == 0 || len(record.Files) != 0 {
		t.Fatalf("this test needs a no-change record with entries and no files, got %+v", record)
	}

	report, err := RevertLastRun(RevertOptions{})
	if err != nil {
		t.Fatalf("a run that changed nothing is already reverted; the revert answered %v", err)
	}
	if report.Unrevertible != 0 || report.Failed != 0 {
		t.Errorf("the no-change record reported unrevertible=%d failed=%d", report.Unrevertible, report.Failed)
	}
	if len(report.Files) != 0 {
		t.Errorf("the no-change record named %d file(s) for the revert to act on", len(report.Files))
	}
	if !report.Complete() {
		t.Error("the revert of a run that changed nothing did not call itself complete")
	}
	if data, _ := os.ReadFile(zsh); string(data) != "unchanged" {
		t.Errorf("the revert touched a file the run did not change: %q", string(data))
	}
}

// The baseline a record compares against belongs to its own run. A person who
// fails and retries starts a new run, and the record of that run has to name the
// state it found: the two digests it keeps are what the revert copies back and
// checks against, so a baseline borrowed from the failed attempt would make the
// record describe a machine that no longer exists.
func TestARetryRecordsWhatItsOwnRunFound(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))

	zsh := system.ConfigPaths()["zsh"]
	if err := os.WriteFile(zsh, []byte("v0"), 0o644); err != nil {
		t.Fatal(err)
	}

	// The first run starts on v0 and stops after writing v1.
	m := NewModel()
	m.ConfigFingerprints = configFingerprints()
	m.ConfigFileIndex = configFileIndex()

	if err := os.WriteFile(zsh, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	v1 := fileDigest(zsh)
	m.recordRun(runStatusFailed, []string{"setshell"})

	first := readLastInstall()
	if first == nil || len(first.Changes) != 1 {
		t.Fatalf("the failed run's record names %d changes; it wrote one file", len(first.Changes))
	}

	// The retry: the error screen's `r` goes back to the welcome screen, and the
	// same model starts a new run whose first step takes its own baseline.
	next, _ := m.beginInstallFromReview()
	retry := next.(Model)
	// The retry takes its own copy of what it is about to replace.
	backupDir := filepath.Join(home, ".dotfiles-backup-2026-10-16-120000")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "zsh"), []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	retry.BackupDir = backupDir
	retry.captureRunBaseline()

	if err := os.WriteFile(zsh, []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	v2 := fileDigest(zsh)
	retry.recordRun(runStatusComplete, nil)

	second := readLastInstall()
	if second == nil || len(second.Changes) != 1 {
		t.Fatalf("the retried run's record names %d changes; it wrote one file", len(second.Changes))
	}
	if second.Changes[0].Before != v1 {
		t.Errorf("the retried run's record says the file held %s before it; that run found %s, the state the failed attempt left",
			shortDigest(second.Changes[0].Before), shortDigest(v1))
	}
	if second.Changes[0].After != v2 {
		t.Errorf("the retried run's record says the file holds %s after it; that run left %s",
			shortDigest(second.Changes[0].After), shortDigest(v2))
	}
	// The digests and the copy have to describe the same moment: a revert puts the
	// copy back and checks it against the digest, so a pair that disagrees makes
	// the revert report a failure on a machine where nothing is wrong.
	if second.Changes[0].Backup == "" {
		t.Fatal("the retried run's record names no copy, so there is nothing for a revert to put back")
	}
	if data, err := os.ReadFile(second.Changes[0].Backup); err != nil || string(data) != "v1" {
		t.Errorf("the copy the record names holds %q (%v); the run it describes found v1", string(data), err)
	}
}
