package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// T4/S4-S6 at the command line: the way back is a command a script can run, and
// its exit status tells "nothing to do" apart from "it went wrong".

// writeRecord writes a record the way an install writes one. The command line
// reads this file, so testing it against the file the installer really produces is
// the point: the shape of the record is the contract between the two.
func writeRecord(t *testing.T, stateHome string, record map[string]any) {
	t.Helper()

	dir := filepath.Join(stateHome, "dotfiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "last-install.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func digestOfFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// A machine with no record has nothing to revert, and that is an answer rather
// than a failure of the command: status 2, not 1.
func TestRevertReportsNothingToDoWhenThereIsNoRecord(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))

	if status := reportRevert(false); status != 2 {
		t.Errorf("reportRevert with no record = %d, want 2", status)
	}
}

// The command the owner asked for: the changes the last run made are put back,
// file by file, and the exit status says the machine is where it was.
func TestRevertPutsTheLastRunBack(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stateHome := filepath.Join(home, "state")
	t.Setenv("XDG_STATE_HOME", stateHome)

	zshrc := filepath.Join(home, ".zshrc")
	if err := os.WriteFile(zshrc, []byte("what the run left"), 0o644); err != nil {
		t.Fatal(err)
	}

	backupDir := filepath.Join(home, ".dotfiles-backup-2026-01-01-000000")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatal(err)
	}
	copyPath := filepath.Join(backupDir, "zsh")
	if err := os.WriteFile(copyPath, []byte("what it held before"), 0o644); err != nil {
		t.Fatal(err)
	}

	writeRecord(t, stateHome, map[string]any{
		"timestamp": "2026-01-01T00:00:00Z",
		"version":   "test",
		"status":    "complete",
		"files":     []string{"zsh: " + zshrc},
		"entries": []map[string]any{{
			"key": "zsh", "path": zshrc, "action": "replaced", "backup": copyPath,
		}},
		"changes": []map[string]any{{
			"key":    "zsh",
			"path":   zshrc,
			"action": "modified",
			"before": digestOfFile(t, copyPath),
			"after":  digestOfFile(t, zshrc),
			"backup": copyPath,
		}},
	})

	if status := reportRevert(false); status != 0 {
		t.Errorf("reportRevert = %d, want 0", status)
	}
	data, err := os.ReadFile(zshrc)
	if err != nil || string(data) != "what it held before" {
		t.Errorf("after the revert %s holds %q (%v), want what it held before the run", zshrc, string(data), err)
	}

	// Running it a second time is not a second change, and it is not a failure:
	// a command a person may run twice has to survive the second run.
	if status := reportRevert(false); status != 0 {
		t.Errorf("the second reportRevert = %d, want 0", status)
	}
	data, _ = os.ReadFile(zshrc)
	if string(data) != "what it held before" {
		t.Errorf("the second revert changed the file to %q", string(data))
	}
}

// A dry run is how a person reads the plan first, so it must leave the machine
// exactly as it found it.
func TestRevertDryRunLeavesTheMachineAlone(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stateHome := filepath.Join(home, "state")
	t.Setenv("XDG_STATE_HOME", stateHome)

	zshrc := filepath.Join(home, ".zshrc")
	if err := os.WriteFile(zshrc, []byte("what the run left"), 0o644); err != nil {
		t.Fatal(err)
	}

	backupDir := filepath.Join(home, ".dotfiles-backup-2026-01-01-000000")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatal(err)
	}
	copyPath := filepath.Join(backupDir, "zsh")
	if err := os.WriteFile(copyPath, []byte("what it held before"), 0o644); err != nil {
		t.Fatal(err)
	}

	writeRecord(t, stateHome, map[string]any{
		"timestamp": "2026-01-01T00:00:00Z",
		"version":   "test",
		"status":    "failed",
		"failedSteps": []string{
			"Install zsh shell",
		},
		"changes": []map[string]any{{
			"key":    "zsh",
			"path":   zshrc,
			"action": "modified",
			"before": digestOfFile(t, copyPath),
			"after":  digestOfFile(t, zshrc),
			"backup": copyPath,
		}},
	})

	if status := reportRevert(true); status != 0 {
		t.Errorf("a dry run reported %d, want 0", status)
	}
	data, _ := os.ReadFile(zshrc)
	if string(data) != "what the run left" {
		t.Errorf("a dry run changed the file to %q", string(data))
	}

	// And it says which run it would revert, including one that failed: a run that
	// stopped halfway is the run whose changes are already on the machine.
	if _, err := os.Stat(filepath.Join(stateHome, "dotfiles", "last-install.json")); err != nil {
		t.Fatalf("the record the dry run read is gone: %v", err)
	}
}

// A backup directory this program did not write is refused before anything is
// planned, and the status says the path was the problem rather than the entries.
func TestRestoreRefusesADirectoryThisProgramDidNotWrite(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	zshrc := filepath.Join(home, ".zshrc")
	if err := os.WriteFile(zshrc, []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}

	foreign := filepath.Join(home, ".dotfiles-backup-someone-elses")
	if err := os.MkdirAll(foreign, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(foreign, "notes.txt"), []byte("not configuration"), 0o644); err != nil {
		t.Fatal(err)
	}

	if status := reportRestore(foreign, false); status != 2 {
		t.Errorf("reportRestore of a foreign directory = %d, want 2", status)
	}
	data, _ := os.ReadFile(zshrc)
	if string(data) != "mine" {
		t.Errorf("the refused restore changed the configuration to %q", string(data))
	}
}

// The record the released build writes -- what every installed machine therefore
// holds -- has entries with actions and copies, files for the paths the run wrote,
// and no per-file changes at all. Answering "the run changed no configuration file"
// to it is the false success this branch has now produced twice.
func TestRevertOnAReleasedBuildRecordSaysItCannotActOnIt(t *testing.T) {
	home := t.TempDir()
	stateHome := filepath.Join(home, "state")
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", stateHome)

	zshrc := filepath.Join(home, ".zshrc")
	if err := os.WriteFile(zshrc, []byte("after-run"), 0o644); err != nil {
		t.Fatal(err)
	}
	backupDir := filepath.Join(home, ".dotfiles-backup-2026-01-01-000000")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatal(err)
	}
	copyPath := filepath.Join(backupDir, "zsh")
	if err := os.WriteFile(copyPath, []byte("before-run"), 0o644); err != nil {
		t.Fatal(err)
	}

	writeRecord(t, stateHome, map[string]any{
		"timestamp": "2026-01-01T00:00:00Z",
		"version":   "released",
		"status":    "complete",
		"files":     []string{"zsh: " + zshrc},
		"entries": []map[string]any{{
			"key": "zsh", "path": zshrc, "action": "replaced", "backup": copyPath,
		}},
	})

	if status := reportRevert(false); status != 1 {
		t.Errorf("reportRevert on a record the released build wrote = %d, want 1", status)
	}
	data, err := os.ReadFile(zshrc)
	if err != nil || string(data) != "after-run" {
		t.Errorf("the revert touched a file it could not act on: %q (%v)", string(data), err)
	}
}

// The machine's most ordinary state after a second run: a completed run that
// changed nothing. It has entries and no per-file changes, which is also the shape
// a record written before this branch carried -- and the answer is not "everything
// is unrevertible, exit 1", it is that there is nothing to put back.
func TestRevertOnAMachineWhereNothingChangedSaysSo(t *testing.T) {
	home := t.TempDir()
	stateHome := filepath.Join(home, "state")
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", stateHome)

	zshrc := filepath.Join(home, ".zshrc")
	if err := os.WriteFile(zshrc, []byte("unchanged"), 0o644); err != nil {
		t.Fatal(err)
	}

	writeRecord(t, stateHome, map[string]any{
		"timestamp": "2026-01-01T00:00:00Z",
		"version":   "test",
		"status":    "complete",
		"files":     []string{},
		"entries":   []map[string]any{{"key": "zsh", "path": zshrc, "action": "found"}},
		"changes":   []map[string]any{},
	})

	if status := reportRevert(false); status != 0 {
		t.Errorf("reportRevert on a machine where the run changed nothing = %d, want 0", status)
	}
	data, err := os.ReadFile(zshrc)
	if err != nil || string(data) != "unchanged" {
		t.Errorf("the revert touched a file the run did not change: %q (%v)", string(data), err)
	}
}

// A dry run is how a person -- or a script -- asks "can this backup go back
// before I ask it to". A plan that names an entry the backup does not hold is not
// a yes.
func TestRestoreDryRunSaysABackupCannotComplete(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	zshrc := filepath.Join(home, ".zshrc")
	if err := os.WriteFile(zshrc, []byte("after"), 0o644); err != nil {
		t.Fatal(err)
	}

	backupDir := filepath.Join(home, ".dotfiles-backup-2026-01-01-000000")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "zsh"), []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The manifest names two entries; the directory holds one.
	gitconfig := filepath.Join(home, ".gitconfig")
	manifest := `{"schemaVersion":1,"createdAt":"2026-01-01T00:00:00Z","entries":[` +
		`{"key":"zsh","rel":".","path":"` + zshrc + `","kind":"file"},` +
		`{"key":"gitconfig","rel":".","path":"` + gitconfig + `","kind":"file"}]}`
	if err := os.WriteFile(filepath.Join(backupDir, "backup.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	if status := reportRestore(backupDir, true); status != 1 {
		t.Errorf("dry run of a backup whose manifest names an entry it does not hold = %d, want 1", status)
	}
	data, _ := os.ReadFile(zshrc)
	if string(data) != "after" {
		t.Errorf("the dry run changed %s to %q", zshrc, string(data))
	}
}

// The other way back, for a machine whose record is gone: a backup directory
// named by hand goes back where its entries came from.
func TestRestorePutsBackupDirectoryBack(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	zshrc := filepath.Join(home, ".zshrc")
	if err := os.WriteFile(zshrc, []byte("what the run left"), 0o644); err != nil {
		t.Fatal(err)
	}

	backupDir := filepath.Join(home, ".dotfiles-backup-2026-01-01-000000")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "zsh"), []byte("what it held before"), 0o644); err != nil {
		t.Fatal(err)
	}

	if status := reportRestore(backupDir, false); status != 0 {
		t.Errorf("reportRestore = %d, want 0", status)
	}
	data, err := os.ReadFile(zshrc)
	if err != nil || string(data) != "what it held before" {
		t.Errorf("after the restore %s holds %q (%v)", zshrc, string(data), err)
	}
}
