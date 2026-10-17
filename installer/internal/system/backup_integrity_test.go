package system

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// T1, S8: a backup must never claim to hold what it does not hold, one bad entry
// must never destroy the entries already copied, and only directories this
// program created may be offered to the user as restorable state.

// A symlink is part of the tree the user had, so it is part of the tree a backup
// holds. Following it would silently replace it with something else (a symlink to
// a directory used to come back as an empty directory) and restoring that backup
// would hand the user a different tree than the one they lost.
func TestBackupPreservesSymlinks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	nvim := ConfigPaths()["nvim"]
	real := filepath.Join(home, "real-plugin")
	if err := os.MkdirAll(real, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "plugin.lua"), []byte("return {}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(nvim, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, filepath.Join(nvim, "linked")); err != nil {
		t.Fatal(err)
	}

	dir, _, err := CreateBackup([]string{"nvim"})
	if err != nil {
		t.Fatalf("CreateBackup failed: %v", err)
	}

	info, err := os.Lstat(filepath.Join(dir, "nvim", "linked"))
	if err != nil {
		t.Fatalf("the backup has no entry for the symlink: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("the symlink came back as %v, so restoring this backup would not return the tree the user had", info.Mode())
	}
	target, err := os.Readlink(filepath.Join(dir, "nvim", "linked"))
	if err != nil {
		t.Fatal(err)
	}
	if target != real {
		t.Errorf("the symlink points at %q, want %q", target, real)
	}

	manifest, err := readBackupManifest(dir)
	if err != nil {
		t.Fatalf("the backup wrote no manifest: %v", err)
	}
	if kind := manifest.kindOf("linked"); kind != "symlink" {
		t.Errorf("the manifest calls the symlink %q, want \"symlink\"", kind)
	}
}

// One entry the run cannot copy must not take the entries it already copied with
// it. The step still fails, because a path the backup could not capture must not
// be overwritten, but the copies that succeeded are the user's only way back and
// they stay on disk.
func TestBackupIsNotDestroyedByAnEntryItCannotCopy(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	paths := ConfigPaths()
	if err := os.WriteFile(paths["zsh"], []byte("the user's real config"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(paths["tmux"], 0644); err != nil {
		t.Skipf("mkfifo is unavailable here: %v", err)
	}

	dir, _, err := CreateBackup([]string{"zsh", "tmux"})
	if err == nil {
		t.Fatal("CreateBackup succeeded although a path it was asked to protect cannot be captured")
	}

	if _, statErr := os.Stat(filepath.Join(dir, "zsh")); statErr != nil {
		t.Errorf("the copy of .zshrc was destroyed by an unrelated entry: %v", statErr)
	}
	if _, statErr := os.Stat(paths["zsh"]); statErr != nil {
		t.Errorf("the live .zshrc was touched: %v", statErr)
	}

	manifest, readErr := readBackupManifest(dir)
	if readErr != nil {
		t.Fatalf("the failed backup kept no account of itself: %v", readErr)
	}
	if len(manifest.Failed) != 1 {
		t.Fatalf("the manifest lists %d failures, want the one entry that could not be copied", len(manifest.Failed))
	}
	if manifest.Failed[0].Key != "tmux" {
		t.Errorf("the failure names key %q, want \"tmux\"", manifest.Failed[0].Key)
	}
}

// An entry that cannot be copied from inside a directory is skipped rather than
// fatal (a live socket beside real configuration is the case), and the backup
// says so in writing. Nothing used to persist this: the list was printed and
// forgotten, so a backup that was quietly incomplete looked complete.
func TestBackupRecordsWhatItCouldNotCopy(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	nvim := ConfigPaths()["nvim"]
	if err := os.MkdirAll(nvim, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nvim, "init.lua"), []byte("set number"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(nvim, "live.sock"), 0644); err != nil {
		t.Skipf("mkfifo is unavailable here: %v", err)
	}

	dir, skipped, err := CreateBackup([]string{"nvim"})
	if err != nil {
		t.Fatalf("a socket inside a directory must not fail the backup: %v", err)
	}
	if len(skipped) != 1 {
		t.Fatalf("CreateBackup reported %d skipped entries, want 1", len(skipped))
	}

	manifest, err := readBackupManifest(dir)
	if err != nil {
		t.Fatalf("the backup wrote no manifest: %v", err)
	}
	if len(manifest.Skipped) != 1 {
		t.Fatalf("the manifest records %d skipped entries, want the one the backup could not copy", len(manifest.Skipped))
	}
	if !filepath.IsAbs(manifest.Skipped[0].Path) {
		t.Errorf("the skipped entry records %q, want the absolute path that was left out", manifest.Skipped[0].Path)
	}
	if manifest.Skipped[0].Reason == "" {
		t.Error("the manifest does not say why the entry was left out")
	}
}

// A directory named like a backup is not a backup. The listing feeds the restore
// screen, which offers what it finds for restore and for deletion, so anything it
// accepts must be something this program wrote.
func TestListBackupsIgnoresDirectoriesItDidNotCreate(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	foreign := filepath.Join(home, ".dotfiles-backup-not-a-backup")
	if err := os.MkdirAll(foreign, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(foreign, "notes.txt"), []byte("mine"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(ConfigPaths()["zsh"], []byte("mine"), 0644); err != nil {
		t.Fatal(err)
	}
	real, _, err := CreateBackup([]string{"zsh"})
	if err != nil {
		t.Fatal(err)
	}

	backups := ListBackups()
	if len(backups) != 1 {
		listed := []string{}
		for _, b := range backups {
			listed = append(listed, filepath.Base(b.Path))
		}
		t.Fatalf("ListBackups offered %v, want only the backup this program wrote", listed)
	}
	if backups[0].Path != real {
		t.Errorf("ListBackups offered %q, want %q", backups[0].Path, real)
	}
	for _, name := range backups[0].Files {
		if name == backupManifestName {
			t.Errorf("the listing counts the backup's own manifest as content: %v", backups[0].Files)
		}
	}
}

// Two runs inside the same second must not share a directory: the second run
// merges into the first, and a failure in either then deletes the other's
// backup. The timestamp keeps its readable shape; a run that collides gets a
// directory of its own.
func TestTwoBackupsInTheSameSecondDoNotShareADirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := os.WriteFile(ConfigPaths()["zsh"], []byte("mine"), 0644); err != nil {
		t.Fatal(err)
	}

	first, _, err := CreateBackup([]string{"zsh"})
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := CreateBackup([]string{"zsh"})
	if err != nil {
		t.Fatal(err)
	}

	if first == second {
		t.Fatalf("both runs wrote %q, so the second can destroy the first", first)
	}
	for _, dir := range []string{first, second} {
		if _, statErr := os.Stat(filepath.Join(dir, "zsh")); statErr != nil {
			t.Errorf("the backup in %q does not hold its copy: %v", dir, statErr)
		}
	}
}
