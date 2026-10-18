package system

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// T2, S9: a restore must never lose data.

// The case that lost it: the destination was removed and only then was the
// backup entry read, so an entry that could not be restored left the user with
// neither their config nor the copy. The destination must survive untouched
// whenever the entry that would replace it cannot be put in place.
func TestRestoreKeepsTheCurrentConfigWhenTheEntryCannotBeRestored(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	current := ConfigPaths()["zsh"]
	if err := os.WriteFile(current, []byte("the user's current config"), 0644); err != nil {
		t.Fatal(err)
	}

	dir, _, err := CreateBackup([]string{"zsh"})
	if err != nil {
		t.Fatal(err)
	}

	// The entry stops being restorable: whatever the reason, the restore cannot
	// rely on the source to replace what it is about to remove, so it must not
	// remove it. A named pipe is the deterministic case, and the one the backup
	// itself already refuses to copy.
	if err := os.RemoveAll(filepath.Join(dir, "zsh")); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(dir, "zsh"), 0644); err != nil {
		t.Skipf("mkfifo is unavailable here: %v", err)
	}

	report, restoreErr := RestoreBackupReport(dir)

	data, readErr := os.ReadFile(current)
	if readErr != nil {
		t.Fatalf("the restore destroyed the current configuration and put nothing in its place: %v", readErr)
	}
	if string(data) != "the user's current config" {
		t.Errorf("the current configuration now holds %q", string(data))
	}
	if restoreErr == nil && report.Failed == 0 {
		t.Error("the restore reported neither an error nor a failed entry")
	}
}

// A restore is itself a change, so it must leave the state it replaces somewhere
// the user can get back to: the copy taken before the install is the way back to
// before the install, and this is the way back to before the restore.
func TestRestoreSnapshotsWhatItOverwrites(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	current := ConfigPaths()["zsh"]
	if err := os.WriteFile(current, []byte("the config as it was before the install"), 0644); err != nil {
		t.Fatal(err)
	}
	earlier, _, err := CreateBackup([]string{"zsh"})
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(current, []byte("what the install left"), 0644); err != nil {
		t.Fatal(err)
	}

	report, restoreErr := RestoreBackupReport(earlier)
	if restoreErr != nil {
		t.Fatalf("restore failed: %v", restoreErr)
	}

	data, err := os.ReadFile(current)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "the config as it was before the install" {
		t.Errorf("the restored configuration holds %q", string(data))
	}

	if report.SnapshotDir == "" {
		t.Fatal("the restore overwrote a configuration without keeping the state it replaced")
	}
	snapshot, err := os.ReadFile(filepath.Join(report.SnapshotDir, "zsh"))
	if err != nil {
		t.Fatalf("the snapshot does not hold what was overwritten: %v", err)
	}
	if string(snapshot) != "what the install left" {
		t.Errorf("the snapshot holds %q, want what the restore replaced", string(snapshot))
	}
}

// The restore screen lists what the listing returns, and the listing now accepts
// only directories this program wrote. A restore asked for one of those
// directories by hand must refuse rather than delete a configuration on the
// strength of a stranger's file.
func TestRestoreRefusesADirectoryItDidNotWrite(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	current := ConfigPaths()["zsh"]
	if err := os.WriteFile(current, []byte("mine"), 0644); err != nil {
		t.Fatal(err)
	}

	foreign := filepath.Join(home, ".dotfiles-backup-someone-elses")
	if err := os.MkdirAll(foreign, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(foreign, "notes.txt"), []byte("not configuration"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := RestoreBackupReport(foreign); err == nil {
		t.Error("a directory this program did not write was accepted for restore")
	}

	data, err := os.ReadFile(current)
	if err != nil || string(data) != "mine" {
		t.Errorf("the configuration was touched by a refused restore: %q, %v", string(data), err)
	}
}

// Backups written before manifests existed are still on machines, and they are
// still the way back for whoever made them. A directory holding nothing but
// configuration keys is one of those, and it must keep working.
func TestRestoreAcceptsABackupWrittenBeforeManifestsExisted(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	current := ConfigPaths()["zsh"]
	if err := os.WriteFile(current, []byte("what the install left"), 0644); err != nil {
		t.Fatal(err)
	}

	legacy := filepath.Join(home, ".dotfiles-backup-2020-01-01-000000")
	if err := os.MkdirAll(legacy, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "zsh"), []byte("the config from before"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := RestoreBackupReport(legacy); err != nil {
		t.Fatalf("a backup written before manifests existed was refused: %v", err)
	}
	data, err := os.ReadFile(current)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "the config from before" {
		t.Errorf("the restored configuration holds %q", string(data))
	}
}

// An entry the restore cannot place is reported, not skipped in silence: the
// difference between "your configuration is back" and "two of the five files are
// back" is the whole value of the report.
func TestRestoreReportsEntriesItCouldNotPlace(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	current := ConfigPaths()["zsh"]
	if err := os.WriteFile(current, []byte("what the install left"), 0644); err != nil {
		t.Fatal(err)
	}

	dir, _, err := CreateBackup([]string{"zsh"})
	if err != nil {
		t.Fatal(err)
	}
	// An entry that names no configuration path: nothing can be put back for it.
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("the backup's own note"), 0644); err != nil {
		t.Fatal(err)
	}

	report, restoreErr := RestoreBackupReport(dir)
	if restoreErr != nil {
		t.Fatalf("an entry nothing can be restored for must not fail the restore: %v", restoreErr)
	}

	if report.Restored != 1 {
		t.Errorf("the report counts %d restored entries, want 1", report.Restored)
	}
	if report.Skipped != 1 {
		t.Fatalf("the report counts %d skipped entries, want the one that names no configuration path", report.Skipped)
	}
	skipped := ""
	for _, entry := range report.Entries {
		if entry.Status == "skipped" {
			skipped = entry.Key
			if entry.Reason == "" {
				t.Error("the report does not say why the entry was skipped")
			}
		}
	}
	if skipped != "notes.txt" {
		t.Errorf("the skipped entry is reported as %q, want \"notes.txt\"", skipped)
	}
}

// The second way an entry can be unrestorable, chosen so that a root test runner
// cannot quietly turn it into a passing test: a file the process cannot read.
func TestRestoreKeepsTheCurrentConfigWhenTheEntryCannotBeRead(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: file permissions do not withhold anything")
	}

	home := t.TempDir()
	t.Setenv("HOME", home)

	current := ConfigPaths()["zsh"]
	if err := os.WriteFile(current, []byte("the user's current config"), 0644); err != nil {
		t.Fatal(err)
	}

	dir, _, err := CreateBackup([]string{"zsh"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "zsh"), 0o000); err != nil {
		t.Fatal(err)
	}

	report, restoreErr := RestoreBackupReport(dir)

	data, readErr := os.ReadFile(current)
	if readErr != nil {
		t.Fatalf("the restore destroyed the current configuration and put nothing in its place: %v", readErr)
	}
	if string(data) != "the user's current config" {
		t.Errorf("the current configuration now holds %q", string(data))
	}
	if restoreErr == nil && report.Failed == 0 {
		t.Error("the restore reported neither an error nor a failed entry")
	}
}
