package tui

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// T4, S4/S5/S6/S12: the way back is a command, and it is verified end to end.

// configTree is the content of the given configuration paths, one entry per
// file. Comparing two of these is what "the tree is what it was" means at the
// byte level, and it is the comparison the round trip is decided by.
func configTree(t *testing.T, roots []string) map[string]string {
	t.Helper()

	tree := map[string]string{}
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				if os.IsNotExist(walkErr) {
					return nil
				}
				return walkErr
			}
			if !entry.Type().IsRegular() {
				return nil
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			sum := sha256.Sum256(data)
			tree[path] = hex.EncodeToString(sum[:])
			return nil
		})
		if err != nil {
			t.Fatalf("reading the tree under %s: %v", root, err)
		}
	}
	return tree
}

func compareTrees(t *testing.T, got, want map[string]string) {
	t.Helper()

	for path, digest := range want {
		other, ok := got[path]
		if !ok {
			t.Errorf("%s was there before the run and is not back", path)
			continue
		}
		if other != digest {
			t.Errorf("%s came back with different content", path)
		}
	}
	for path := range got {
		if _, ok := want[path]; !ok {
			t.Errorf("%s was not there before the run and is still there", path)
		}
	}
}

// runOneOfEachChange sets up a machine, runs a simulated install that modifies,
// creates and deletes a file, and records the run the way the installer does.
// It returns the configuration roots, the tree as it was before the run, and the
// backup directory the run took.
func runOneOfEachChange(t *testing.T) ([]string, map[string]string, string) {
	t.Helper()

	paths := system.ConfigPaths()
	roots := []string{paths["zsh"], paths["nvim"]}
	keys := []string{"zsh", "nvim"}

	if err := os.WriteFile(paths["zsh"], []byte("zshrc before the run"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(paths["nvim"], 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(paths["nvim"], "init.lua"), []byte("init before the run"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(paths["nvim"], "dropped.lua"), []byte("dropped before the run"), 0o644); err != nil {
		t.Fatal(err)
	}

	before := configTree(t, roots)

	m := &Model{ConfigFileIndex: configFileIndex()}
	backupDir, _, err := system.CreateBackup(keys)
	if err != nil {
		t.Fatal(err)
	}
	m.BackupDir = backupDir

	if err := os.WriteFile(paths["zsh"], []byte("zshrc after the run"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(paths["nvim"], "init.lua"), []byte("init after the run"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(paths["nvim"], "added.lua"), []byte("added by the run"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(paths["nvim"], "dropped.lua")); err != nil {
		t.Fatal(err)
	}

	m.recordRun(runStatusComplete, nil)

	return roots, before, backupDir
}

// The whole point of the manifest: every change the run made is undone, and the
// result is checked against the bytes the machine had before the run rather than
// against the manifest's own opinion of them.
func TestRevertPutsTheTreeBackByteForByte(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))

	roots, before, backupDir := runOneOfEachChange(t)

	report, err := RevertLastRun(RevertOptions{})
	if err != nil {
		t.Fatalf("the revert failed: %v", err)
	}
	if report.Failed != 0 || report.Unrevertible != 0 {
		t.Fatalf("the revert did not put everything back: %+v", report)
	}
	if report.Reverted != 4 {
		t.Errorf("the revert reports %d files put back, want 4 (two modified, one created, one deleted)", report.Reverted)
	}
	if report.Record == nil || report.Record.Timestamp.IsZero() {
		t.Error("the report does not name the run it reverted")
	}

	compareTrees(t, configTree(t, roots), before)

	// A revert is a change like any other: what it replaced has to be somewhere.
	if report.SnapshotDir == "" {
		t.Fatal("the revert replaced the machine's state without keeping what it replaced")
	}
	data, err := os.ReadFile(filepath.Join(report.SnapshotDir, "zsh"))
	if err != nil {
		t.Fatalf("the snapshot does not hold what the revert replaced: %v", err)
	}
	if string(data) != "zshrc after the run" {
		t.Errorf("the snapshot holds %q, want the state the revert replaced", string(data))
	}
	if _, err := os.Stat(backupDir); err != nil {
		t.Errorf("the revert disturbed the backup it read: %v", err)
	}

	// Running it again is not a second change: the machine is already where the
	// revert puts it, and a command a person may run twice must say so instead of
	// failing or rewriting.
	second, err := RevertLastRun(RevertOptions{})
	if err != nil {
		t.Fatalf("a second revert reported an error: %v", err)
	}
	if second.Reverted != 0 {
		t.Errorf("the second revert changed %d files, want none", second.Reverted)
	}
	if second.AlreadyDone != 4 {
		t.Errorf("the second revert reports %d files already in place, want 4", second.AlreadyDone)
	}
	compareTrees(t, configTree(t, roots), before)
}

// A dry run is how a person reads the plan before letting it happen, so it must
// name every file it would touch and touch none of them.
func TestRevertDryRunNamesThePlanAndTouchesNothing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))

	roots, _, _ := runOneOfEachChange(t)
	after := configTree(t, roots)

	report, err := RevertLastRun(RevertOptions{DryRun: true})
	if err != nil {
		t.Fatalf("the dry run failed: %v", err)
	}
	if report.Reverted != 0 {
		t.Errorf("a dry run put %d files back", report.Reverted)
	}
	if report.Planned != 4 {
		t.Errorf("the dry run plans %d files, want 4", report.Planned)
	}
	if len(report.Files) != 4 {
		t.Errorf("the dry run names %d files, want 4: %+v", len(report.Files), report.Files)
	}
	for _, file := range report.Files {
		if file.Status != RevertWouldRevert {
			t.Errorf("%s is reported as %q in a dry run, want %q", file.Path, file.Status, RevertWouldRevert)
		}
	}
	if report.SnapshotDir != "" {
		t.Errorf("a dry run made a snapshot at %s", report.SnapshotDir)
	}

	compareTrees(t, configTree(t, roots), after)
}

// The record can name a file the backup never copied -- a path the backup step
// could not read, or a record written before the backup step ran. Saying so is
// the whole value: a revert that reported success while leaving the file changed
// would be the same defect in a new place.
func TestRevertSaysWhatItCannotPutBack(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))

	paths := system.ConfigPaths()
	if err := os.WriteFile(paths["zsh"], []byte("before the run"), 0o644); err != nil {
		t.Fatal(err)
	}

	// No backup directory: the run recorded what it changed but kept no copy.
	m := &Model{ConfigFileIndex: configFileIndex()}
	if err := os.WriteFile(paths["zsh"], []byte("after the run"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.recordRun(runStatusComplete, nil)

	report, err := RevertLastRun(RevertOptions{})
	if err == nil {
		t.Error("a revert that could not put a file back reported success")
	}
	if report.Unrevertible != 1 {
		t.Fatalf("the revert reports %d files it cannot put back, want 1: %+v", report.Unrevertible, report.Files)
	}

	var named bool
	for _, file := range report.Files {
		if file.Path == paths["zsh"] {
			named = true
			if file.Status != RevertUnrevertible {
				t.Errorf("%s is reported as %q, want %q", file.Path, file.Status, RevertUnrevertible)
			}
			if file.Reason == "" {
				t.Errorf("%s is reported as unrevertible with no reason", file.Path)
			}
		}
	}
	if !named {
		t.Errorf("the revert does not name the file it could not put back: %+v", report.Files)
	}

	// Nothing may be removed on the way: the file it could not restore is still
	// the file the run left, not a hole.
	data, readErr := os.ReadFile(paths["zsh"])
	if readErr != nil || string(data) != "after the run" {
		t.Errorf("the file it could not put back now holds %q (%v)", string(data), readErr)
	}
}

// A machine with no record has nothing to revert, and saying that is not a
// failure of the command: it is the answer.
func TestRevertWithNoRecordSaysThereIsNothingToRevert(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))

	report, err := RevertLastRun(RevertOptions{})
	if err == nil {
		t.Fatal("a revert with no record reported success")
	}
	if !errors.Is(err, errNoRecord) {
		t.Errorf("the error is %v, want the one that says there is no record", err)
	}
	if report.Record != nil {
		t.Error("the report names a record that does not exist")
	}
}

// The scope decision names what stays irreversible instead of quietly omitting
// it, and the revert is where a person looks for that answer.
func TestTheRevertNamesWhatNoCommandCanUndo(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))

	// A real run, because what cannot be undone is what that run did: a machine
	// with no record has nothing to say about it.
	runOneOfEachChange(t)

	report, err := RevertLastRun(RevertOptions{DryRun: true})
	if err != nil {
		t.Fatalf("the dry run failed: %v", err)
	}

	joined := strings.Join(report.Irreversible, " ")
	if len(report.Irreversible) == 0 {
		t.Fatal("the revert says nothing about what it cannot undo")
	}
	for _, want := range []string{"package", "shell"} {
		if !strings.Contains(joined, want) {
			t.Errorf("what the revert cannot undo does not mention %q: %q", want, joined)
		}
	}
	for _, item := range report.Irreversible {
		if strings.TrimSpace(item) == "" {
			t.Error("an item in the list of what cannot be undone is empty")
		}
	}
}
