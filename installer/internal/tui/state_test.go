package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestLastInstallRoundTripsUnderATemporaryStateDir covers both halves of the
// state file against a directory the test owns: the writer puts the record where
// the documented path says, and the reader gets the same record back. Nothing
// here touches the user's real state directory.
func TestLastInstallRoundTripsUnderATemporaryStateDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	want := lastInstall{
		Timestamp: time.Date(2026, 9, 29, 14, 30, 0, 0, time.UTC),
		Version:   "v0.4.0",
		Files:     []string{"nvim: /home/testuser/.config/nvim", "fish: /home/testuser/.config/fish"},
	}
	if err := writeLastInstall(want); err != nil {
		t.Fatalf("writeLastInstall: %v", err)
	}

	if got, wantPath := lastInstallPath(), filepath.Join(dir, "dotfiles", "last-install.json"); got != wantPath {
		t.Errorf("lastInstallPath = %q, want %q", got, wantPath)
	}
	if _, err := os.Stat(lastInstallPath()); err != nil {
		t.Errorf("the record was not written at %s: %v", lastInstallPath(), err)
	}

	got := readLastInstall()
	if got == nil {
		t.Fatalf("readLastInstall returned nil after a successful write")
	}
	if !got.Timestamp.Equal(want.Timestamp) {
		t.Errorf("Timestamp = %v, want %v", got.Timestamp, want.Timestamp)
	}
	if got.Version != want.Version {
		t.Errorf("Version = %q, want %q", got.Version, want.Version)
	}
	if strings.Join(got.Files, "|") != strings.Join(want.Files, "|") {
		t.Errorf("Files = %v, want %v", got.Files, want.Files)
	}
}

// TestLastInstallPathFallsBackToTheHomeStateDir pins the documented fallback:
// with no XDG_STATE_HOME the record lives under ~/.local/state, which is the
// path the manual page writes down.
func TestLastInstallPathFallsBackToTheHomeStateDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", home)

	want := filepath.Join(home, ".local", "state", "dotfiles", "last-install.json")
	if got := lastInstallPath(); got != want {
		t.Errorf("lastInstallPath = %q, want %q", got, want)
	}
}

// TestLastInstallReadsNoRecordForMissingCorruptOrIncompleteFile pins the reader's
// honesty rule: none of these is a run, so each reads as no record rather than as
// a partial one the panel would then draw.
func TestLastInstallReadsNoRecordForMissingCorruptOrIncompleteFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	if got := readLastInstall(); got != nil {
		t.Errorf("a missing file read as %+v, want no record", got)
	}

	path := lastInstallPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("test setup: %v", err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatalf("test setup: %v", err)
	}
	if got := readLastInstall(); got != nil {
		t.Errorf("a corrupt file read as %+v, want no record", got)
	}

	if err := os.WriteFile(path, []byte(`{"version":"v0.4.0","files":["nvim"]}`), 0o644); err != nil {
		t.Fatalf("test setup: %v", err)
	}
	if got := readLastInstall(); got != nil {
		t.Errorf("a record with no timestamp read as %+v, want no record", got)
	}
}

// TestCompletingARunRecordsItAndWritesTheStateFile pins the write the run's end
// triggers: the model carries the record so the panel updates immediately, and
// the file the next startup reads holds the same version and touched paths.
func TestCompletingARunRecordsItAndWritesTheStateFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	m := Model{
		Screen:          ScreenInstalling,
		ExistingConfigs: []string{"nvim: /home/testuser/.config/nvim"},
	}
	next, _ := m.update(installCompleteMsg{totalTime: 12.5})
	got, ok := next.(Model)
	if !ok {
		t.Fatalf("the completion handler returned %T, want Model", next)
	}

	if got.Screen != ScreenComplete {
		t.Errorf("Screen = %v after the run completed, want ScreenComplete", got.Screen)
	}
	if got.LastInstall == nil {
		t.Fatalf("the completed run left no record on the model")
	}
	if got.LastInstall.Version == "" {
		t.Errorf("the record carries no version")
	}
	if got.LastInstall.Timestamp.IsZero() {
		t.Errorf("the record carries no timestamp")
	}

	rec := readLastInstall()
	if rec == nil {
		t.Fatalf("the completed run wrote no file at %s", lastInstallPath())
	}
	if rec.Version != got.LastInstall.Version {
		t.Errorf("the file's version = %q, the model's = %q", rec.Version, got.LastInstall.Version)
	}
	if strings.Join(rec.Files, "|") != "nvim: /home/testuser/.config/nvim" {
		t.Errorf("the file's files = %v, want the run's existing configs", rec.Files)
	}
}

// TestAStateDirectoryThatCannotBeWrittenDoesNotFailTheRun pins the best-effort
// contract: a state directory the machine will not let us create returns an
// error from the writer, and the run still completes with its record on the
// model. An install that already succeeded must not be reported as failed by its
// own bookkeeping.
func TestAStateDirectoryThatCannotBeWrittenDoesNotFailTheRun(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("test setup: %v", err)
	}
	// stateDir() will be blocker/state/dotfiles, and blocker is a file, so MkdirAll
	// cannot create it.
	t.Setenv("XDG_STATE_HOME", filepath.Join(blocker, "state"))

	if err := writeLastInstall(lastInstall{Timestamp: time.Now(), Version: "v0.4.0"}); err == nil {
		t.Errorf("writeLastInstall reported success into a directory it cannot create")
	}

	m := Model{Screen: ScreenInstalling}
	next, _ := m.update(installCompleteMsg{totalTime: 1})
	got := next.(Model)
	if got.Screen != ScreenComplete {
		t.Errorf("Screen = %v after an unwritable state directory, want ScreenComplete", got.Screen)
	}
	if got.LastInstall == nil {
		t.Errorf("the run did not record itself on the model")
	}
	if readLastInstall() != nil {
		t.Errorf("a record was read back from a directory nothing could be written to")
	}
}
