package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// The bodies of this file and of record_routes_test.go are the eighth pass's probes,
// verbatim except for their names and the helper they call: the pass ran them at the
// pre-fix baseline and at HEAD, so a verdict already covers these exact assertions.
// The probes that measured a pre-existing advisory (a fifo or socket with no copy, a
// no-copy link reaching outside every named path) were deliberately not kept: a test
// asserting one would pin a known false alarm as the contract.

// A link the key already held, with no copy of the key at all, must not make a revert that
// put every change back fail. The same regular file reached behind the link and directly is
// judged the same way, because a missing copy is not evidence that the run wrote anything.
func TestANoCopyLinkToARegularFileIsNotAFalseAlarm(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	nvim := system.ConfigPaths()["nvim"]
	if nvim == "" {
		t.Skip("no nvim path")
	}
	if err := os.MkdirAll(nvim, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nvim, "init.lua"), []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nvim, "target"), []byte("untouched"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(nvim, "target"), filepath.Join(nvim, "lnk")); err != nil {
		t.Fatal(err)
	}
	m := NewModel()
	m.ExistingConfigs = []string{"nvim: " + nvim}
	m.ConfigFingerprints = configFingerprints()
	m.ConfigFileIndex = configFileIndex()
	if err := os.WriteFile(filepath.Join(nvim, "new.lua"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.recordRun(runStatusComplete, nil)

	report, err := RevertLastRun(RevertOptions{})
	if err != nil || !report.Complete() {
		t.Fatalf("routes disagreed / false alarm: %v (%+v)", err, report.Files)
	}
	if _, statErr := os.Lstat(filepath.Join(nvim, "new.lua")); !os.IsNotExist(statErr) {
		t.Fatalf("created file not removed: %v", statErr)
	}
	if _, statErr := os.Lstat(filepath.Join(nvim, "target")); statErr != nil {
		t.Fatalf("target removed: %v", statErr)
	}
}

// The pass-6 scenario, which must never become a silent success: a key that is a symlink to
// a directory, no backup, and a file inside it was modified rather than created. Nothing can
// put that file back, and the command has to say so.
func TestAModifiedFileInsideALinkedKeyWithNoBackupIsNotASilentSuccess(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	nvim := system.ConfigPaths()["nvim"]
	if nvim == "" {
		t.Skip("no nvim path")
	}
	real := filepath.Join(home, "repo", "nvim")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "init.lua"), []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(nvim), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, nvim); err != nil {
		t.Fatal(err)
	}
	m := NewModel()
	m.ExistingConfigs = []string{"nvim: " + nvim}
	m.ConfigFingerprints = configFingerprints()
	m.ConfigFileIndex = configFileIndex()
	if err := os.WriteFile(filepath.Join(real, "init.lua"), []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.recordRun(runStatusComplete, nil)
	report, err := RevertLastRun(RevertOptions{})
	if err == nil || report.Complete() {
		t.Fatalf("modified file inside a linked key, no backup, reported success: %v %+v", err, report.Files)
	}
	t.Logf("modified linked key: %v", err)
}
