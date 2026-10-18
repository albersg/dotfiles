package tui

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// T3, S10: the record must name every file the run changed, per file and with
// the digests that make the change checkable, and it must exist for a run that
// failed and for a run that never opened the interface.

// The record used to say only which configuration paths changed, and only in
// aggregate: a directory key was one line whether the run rewrote one file or
// deleted twenty. A reader could not tell what happened, and nothing could put it
// back.
func TestTheRecordNamesEveryFileTheRunChanged(t *testing.T) {
	before := map[string]string{
		"/home/u/.zshrc":                   "digest-zshrc-before",
		"/home/u/.config/nvim/init.lua":    "digest-init",
		"/home/u/.config/nvim/dropped.lua": "digest-dropped",
	}
	after := map[string]string{
		"/home/u/.zshrc":                 "digest-zshrc-after",
		"/home/u/.config/nvim/init.lua":  "digest-init",
		"/home/u/.config/nvim/added.lua": "digest-added",
	}

	changes := fileChanges(before, after)

	want := []fileChange{
		{Path: "/home/u/.config/nvim/added.lua", Action: fileActionCreated, After: "digest-added"},
		{Path: "/home/u/.config/nvim/dropped.lua", Action: fileActionDeleted, Before: "digest-dropped"},
		{Path: "/home/u/.zshrc", Action: fileActionModified, Before: "digest-zshrc-before", After: "digest-zshrc-after"},
	}
	if len(changes) != len(want) {
		t.Fatalf("the record names %d files, want %d: %+v", len(changes), len(want), changes)
	}
	for i, change := range changes {
		if change != want[i] {
			t.Errorf("change %d is %+v, want %+v", i, change, want[i])
		}
	}
}

// A run that changed nothing must not name anything: an empty record is how a
// reader learns the run left the machine as it found it.
func TestARunThatChangedNothingNamesNoFile(t *testing.T) {
	index := map[string]string{"/home/u/.zshrc": "same"}

	if changes := fileChanges(index, index); len(changes) != 0 {
		t.Errorf("a run that changed nothing named %d files: %+v", len(changes), changes)
	}
	if changes := fileChanges(nil, nil); len(changes) != 0 {
		t.Errorf("an index that was never taken named %d files: %+v", len(changes), changes)
	}
}

// The record of the last run was written in exactly one place: the message that
// says the run finished. A run that stopped at a failed step left no record at
// all, so the next person to look saw the previous run, or nothing, and had no
// way to learn which files this one had already rewritten before it stopped.
func TestARunThatFailedStillLeavesARecord(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))

	current := system.ConfigPaths()["zsh"]
	if err := os.WriteFile(current, []byte("before the run"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := &Model{ConfigFileIndex: configFileIndex()}
	if len(m.ConfigFileIndex) == 0 {
		t.Fatal("the before-run file index is empty, so there is nothing to compare")
	}

	if err := os.WriteFile(current, []byte("what the failed run left behind"), 0o644); err != nil {
		t.Fatal(err)
	}

	m.recordRun(runStatusFailed, []string{"font"})

	rec := readLastInstall()
	if rec == nil {
		t.Fatal("a run that failed left no record at all")
	}
	if rec.Status != runStatusFailed {
		t.Errorf("the record says the run was %q, want %q", rec.Status, runStatusFailed)
	}
	if len(rec.FailedSteps) != 1 || rec.FailedSteps[0] != "font" {
		t.Errorf("the record names the failed steps as %v, want [font]", rec.FailedSteps)
	}

	change, ok := findFileChange(rec.Changes, current)
	if !ok {
		t.Fatalf("the record does not name %s, the file the run had already rewritten: %+v", current, rec.Changes)
	}
	if change.Action != fileActionModified {
		t.Errorf("the file is recorded as %q, want %q", change.Action, fileActionModified)
	}
	if change.Before == "" || change.After == "" || change.Before == change.After {
		t.Errorf("the record's digests do not describe a change: before=%q after=%q", change.Before, change.After)
	}
}

// The digests say what changed; the copy says how to undo it. A file the backup
// holds must name the copy, because that path is what a revert reads, and a file
// it does not hold must name nothing rather than a path that is not there.
func TestTheRecordPointsAtTheCopyThatPutsAFileBack(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))

	current := system.ConfigPaths()["zsh"]
	if err := os.WriteFile(current, []byte("before the run"), 0o644); err != nil {
		t.Fatal(err)
	}

	backupDir, _, err := system.CreateBackup([]string{"zsh"})
	if err != nil {
		t.Fatal(err)
	}

	m := &Model{BackupDir: backupDir, ConfigFileIndex: configFileIndex()}
	if err := os.WriteFile(current, []byte("after the run"), 0o644); err != nil {
		t.Fatal(err)
	}

	m.recordRun(runStatusComplete, nil)
	rec := readLastInstall()
	if rec == nil {
		t.Fatal("the run left no record")
	}

	change, ok := findFileChange(rec.Changes, current)
	if !ok {
		t.Fatalf("the record does not name %s: %+v", current, rec.Changes)
	}
	want := filepath.Join(backupDir, "zsh")
	if change.Backup != want {
		t.Errorf("the record points at %q for the copy that puts the file back, want %q", change.Backup, want)
	}
	if _, err := os.Stat(change.Backup); err != nil {
		t.Errorf("the copy the record points at is not there: %v", err)
	}

	// A file the backup does not hold must not claim one.
	if change, ok := findFileChange(rec.Changes, filepath.Join(home, "never-existed")); ok {
		t.Errorf("the record names a file the run never touched: %+v", change)
	}
}

// The interface's record is written by the message that ends the run, and the
// non-interactive route has no such message: it printed its summary and exited,
// which is how a scripted install came to leave no record of itself at all.
func TestANonInteractiveRunLeavesARecordToo(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))

	current := system.ConfigPaths()["zsh"]
	if err := os.WriteFile(current, []byte("before the scripted run"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := &Model{ConfigFileIndex: configFileIndex()}
	if err := os.WriteFile(current, []byte("what the scripted run left"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := finishNonInteractiveRun(m, []error{errors.New("step 'Font' failed")}); err == nil {
		t.Error("a run with a failed step reported success")
	}

	rec := readLastInstall()
	if rec == nil {
		t.Fatal("a scripted run left no record")
	}
	if rec.Status != runStatusFailed {
		t.Errorf("the record says the run was %q, want %q", rec.Status, runStatusFailed)
	}
	if len(rec.FailedSteps) != 1 {
		t.Errorf("the record names the failed steps as %v, want one entry", rec.FailedSteps)
	}
	if _, ok := findFileChange(rec.Changes, current); !ok {
		t.Errorf("the record does not name %s: %+v", current, rec.Changes)
	}

	// And a scripted run that worked records itself as one.
	second := t.TempDir()
	t.Setenv("HOME", second)
	t.Setenv("XDG_STATE_HOME", filepath.Join(second, "state"))
	quiet := &Model{}
	if err := finishNonInteractiveRun(quiet, nil); err != nil {
		t.Fatalf("a run with no failed step reported one: %v", err)
	}
	rec = readLastInstall()
	if rec == nil {
		t.Fatal("a scripted run that worked left no record")
	}
	if rec.Status != runStatusComplete {
		t.Errorf("the record says the run was %q, want %q", rec.Status, runStatusComplete)
	}
	if len(rec.FailedSteps) != 0 {
		t.Errorf("a run with no failed step names %v", rec.FailedSteps)
	}
}

func findFileChange(changes []fileChange, path string) (fileChange, bool) {
	for _, change := range changes {
		if change.Path == path {
			return change, true
		}
	}
	return fileChange{}, false
}
