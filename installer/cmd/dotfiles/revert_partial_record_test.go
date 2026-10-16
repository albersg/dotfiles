package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// A record can carry per-file changes for one path while naming another as written
// -- by hand, by a build whose index saw less, or by a key whose change the file
// index steps over. Reading the changes and ignoring the rest is how a machine gets
// reported as back with a file the run wrote still holding what the run left in it.
func TestAVerticalRecordWithPartialEvidenceIsNotReportedAsReverted(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))

	zshrc := system.ConfigPaths()["zsh"]
	if err := os.WriteFile(zshrc, []byte("after-zsh"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitconfig := system.ConfigPaths()["gitconfig"]
	if err := os.WriteFile(gitconfig, []byte("after-git"), 0o644); err != nil {
		t.Fatal(err)
	}

	stateDir := filepath.Join(home, "state", "dotfiles")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	record := map[string]any{
		"timestamp": "2026-01-01T00:00:00Z",
		"version":   "0.1.0",
		"status":    "complete",
		"files":     []string{"zsh"},
		"entries": []map[string]string{
			{"key": "zsh", "path": zshrc, "action": "created"},
		},
		"changes": []map[string]string{
			{"key": "gitconfig", "path": gitconfig, "action": "created"},
		},
	}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "last-install.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	if code := reportRevert(false); code != 1 {
		t.Errorf("reportRevert = %d, want 1: the record names a path the run wrote that its per-file changes do not account for", code)
	}
	if data, _ := os.ReadFile(zshrc); string(data) != "after-zsh" {
		t.Errorf("the revert wrote to a path the record gives it no before-state for: %q", string(data))
	}
}
