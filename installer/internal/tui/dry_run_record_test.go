package tui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// A dry run does nothing, so it has nothing to record. Writing its empty record over
// the one a real run left destroys the evidence the revert and the content check read:
// the next --revert would report that the run changed nothing while the machine still
// holds every file that run wrote -- the same false success this branch exists to
// remove, arriving through the flag that promises to change nothing.
func TestADryRunDoesNotReplaceTheRecordOfTheLastRealRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))

	zsh := system.ConfigPaths()["zsh"]
	if zsh == "" {
		t.Skip("this machine has no zsh configuration path")
	}
	backup := filepath.Join(home, ".dotfiles-backup-1", "zsh")
	mustWrite(t, os.MkdirAll(backup, 0o755))
	mustWrite(t, os.WriteFile(filepath.Join(backup, "zsh"), []byte("before"), 0o644))
	mustWrite(t, os.WriteFile(zsh, []byte("after"), 0o644))
	mustWrite(t, writeLastInstall(lastInstall{
		Timestamp: time.Now(),
		Status:    runStatusComplete,
		Files:     []string{"zsh"},
		Entries:   []installEntry{{Key: "zsh", Path: zsh, Action: installActionReplaced, Backup: filepath.Join(backup, "zsh")}},
		Changes:   []fileChange{{Key: "zsh", Path: zsh, Action: fileActionModified, Before: fileDigest(filepath.Join(backup, "zsh")), After: fileDigest(zsh), Backup: filepath.Join(backup, "zsh")}},
	}))

	recordPath := filepath.Join(home, "state", "dotfiles", "last-install.json")
	before, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatal(err)
	}

	m := NewModel()
	t.Setenv("DOTFILES_DRY_RUN", "1")
	if err := finishNonInteractiveRun(&m, nil); err != nil {
		t.Fatalf("a dry run that did nothing answered %v", err)
	}

	after, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatalf("a dry run removed the record of the last real run: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("a dry run replaced the record of the last real run:\n before: %s\n after:  %s", before, after)
	}
}

// The interface reaches its record through routes of its own. Finishing a dry run through
// one of them must not overwrite the record of the last real run either.
func TestADryRunThroughTheInterfaceDoesNotReplaceTheRecordOfTheLastRealRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))

	zsh := system.ConfigPaths()["zsh"]
	if zsh == "" {
		t.Skip("this machine has no zsh configuration path")
	}
	backup := filepath.Join(home, ".dotfiles-backup-1", "zsh")
	mustWrite(t, os.MkdirAll(backup, 0o755))
	mustWrite(t, os.WriteFile(filepath.Join(backup, "zsh"), []byte("before"), 0o644))
	mustWrite(t, os.WriteFile(zsh, []byte("after"), 0o644))
	mustWrite(t, writeLastInstall(lastInstall{
		Timestamp: time.Now(), Status: runStatusComplete, Files: []string{"zsh"},
		Entries: []installEntry{{Key: "zsh", Path: zsh, Action: installActionReplaced, Backup: filepath.Join(backup, "zsh")}},
		Changes: []fileChange{{Key: "zsh", Path: zsh, Action: fileActionModified, Before: fileDigest(filepath.Join(backup, "zsh")), After: fileDigest(zsh), Backup: filepath.Join(backup, "zsh")}},
	}))
	recordPath := filepath.Join(home, "state", "dotfiles", "last-install.json")
	before, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatal(err)
	}

	m := NewModel()
	t.Setenv("DOTFILES_DRY_RUN", "1")
	_, _ = m.Update(installCompleteMsg{totalTime: 0})

	after, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatalf("a dry run removed the record of the last real run: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("a dry run through the interface replaced the record of the last real run:\n before: %s\n after:  %s", before, after)
	}
}
