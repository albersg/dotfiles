package system

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The manifest is what says a backup took a copy of something. When the copy is no
// longer in the directory, the restore has to name that entry: a backup that holds
// three of the four things it says it holds is not a machine that is back, and
// "0 entries are back" with nothing else said is the same silence either way.
func TestARestoreNamesAManifestEntryWhoseCopyIsGone(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	zsh := ConfigPaths()["zsh"]
	if err := os.WriteFile(zsh, []byte("before the run"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ConfigPaths()["gitconfig"], []byte("before the run too"), 0o644); err != nil {
		t.Fatal(err)
	}

	backupDir, _, err := CreateBackup([]string{"zsh", "gitconfig"})
	if err != nil {
		t.Fatalf("taking the backup this test damages failed: %v", err)
	}

	// The run replaces both, and then the copy of one of them is lost: a partial
	// copy, a deleted file, a directory synced halfway.
	if err := os.WriteFile(zsh, []byte("after"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest, err := readBackupManifest(backupDir)
	if err != nil {
		t.Fatalf("the backup this test just wrote has no readable manifest: %v", err)
	}
	var named []string
	for _, entry := range manifest.Entries {
		if entry.Key == "gitconfig" {
			named = append(named, entry.Key)
		}
	}
	if len(named) != 1 {
		t.Fatalf("the manifest names %d entries for the gitconfig key; the backup took one", len(named))
	}
	damaged := named[0]
	if err := os.Remove(filepath.Join(backupDir, damaged)); err != nil {
		t.Fatal(err)
	}

	report, err := RestoreBackupReport(backupDir)
	if err == nil {
		t.Fatal("a restore that could not put back an entry the manifest names reported success")
	}
	// The account has to count the entries it was supposed to place -- the two the
	// manifest names, not the one the directory still holds -- or it tells a reader
	// that one of nothing went wrong.
	if !strings.Contains(err.Error(), "failed for 1 of 2 entries") {
		t.Errorf("the failure account does not count the entry it could not place: %v", err)
	}

	var namedFailure bool
	for _, entry := range report.Entries {
		if entry.Key != damaged {
			continue
		}
		namedFailure = entry.Status == "failed" && strings.Contains(entry.Reason, "no copy")
	}
	if !namedFailure {
		t.Fatalf("the restore did not name %s as an entry whose copy is gone: %+v", damaged, report.Entries)
	}
	if report.Failed != 1 {
		t.Errorf("the restore counted %d failed entries; %s could not be put back and nothing else failed",
			report.Failed, damaged)
	}

	// A plan has to promise what the restore can do: the same gap, before anything
	// moves.
	plan, err := RestorePlan(backupDir)
	if err != nil {
		t.Fatal(err)
	}
	var plannedFailure bool
	for _, entry := range plan.Entries {
		if entry.Key == damaged && entry.Status == "failed" {
			plannedFailure = true
		}
	}
	if !plannedFailure {
		t.Errorf("the plan promised a restore that the backup cannot complete: %+v", plan.Entries)
	}
}
