package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/albersg/dotfiles/installer/internal/system"
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
// the file the next startup reads holds the same version and the outcome of each
// configuration path. It also pins the issue #239 honesty rule: the record is
// built from what the run did, not from the pre-run detection, so a config the
// run only found is recorded as found and not counted as touched.
func TestCompletingARunRecordsItAndWritesTheStateFile(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	t.Setenv("HOME", home)

	// A configuration that was on disk before this run. The run writes nothing,
	// so the record may only report it as found.
	found := filepath.Join(home, ".config", "nvim")
	if err := os.MkdirAll(found, 0o755); err != nil {
		t.Fatalf("test setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(found, "init.lua"), []byte("-- user\n"), 0o644); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	m := Model{
		Screen:          ScreenInstalling,
		ExistingConfigs: system.DetectExistingConfigs(),
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
	if len(rec.Files) != 0 {
		t.Errorf("the file counts %v as touched, but the run wrote nothing", rec.Files)
	}
	entry, ok := entryFor(*rec, "nvim")
	if !ok {
		t.Fatalf("the record does not name the detected nvim configuration")
	}
	if entry.Action != installActionFound {
		t.Errorf("nvim action = %q, want %q", entry.Action, installActionFound)
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

// --- Verification against the record ---------------------------------------

// verifyReportText joins the lines VerifyInstall returns, so a test asserts on
// what the user reads in one place.
func verifyReportText(report VerifyReport) string {
	return strings.Join(report.Lines, "\n")
}

// TestVerifyReportsAPathThatIsHereAsHereButUnverifiable is case one of the
// five: the record names a path that is still there. The report may say "here",
// and it must say what it cannot say -- the record names no digest, so it cannot
// claim the bytes are still the ones the run left.
func TestVerifyReportsAPathThatIsHereAsHereButUnverifiable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("HOME", home)

	present := filepath.Join(home, ".config", "nvim")
	if err := os.MkdirAll(present, 0o755); err != nil {
		t.Fatalf("test setup: %v", err)
	}
	if err := writeLastInstall(lastInstall{
		Timestamp: time.Date(2026, 9, 29, 14, 30, 0, 0, time.UTC),
		Version:   "v0.4.0",
		Entries:   []installEntry{{Key: "nvim", Path: present, Action: installActionReplaced}},
	}); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	report := VerifyInstall()
	if report.Unreadable {
		t.Fatalf("a readable record read as unreadable:\n%s", verifyReportText(report))
	}
	if !report.Consistent {
		t.Errorf("a path the record names and that is still here was reported as absent:\n%s", verifyReportText(report))
	}
	text := verifyReportText(report)
	if !strings.Contains(text, "here") || !strings.Contains(text, present) {
		t.Errorf("the report does not name the present path as here:\n%s", text)
	}
	if !strings.Contains(text, "cannot say whether this is still what the install left") {
		t.Errorf("the report claims more than the record holds: it must say the contents cannot be judged:\n%s", text)
	}
}

// TestVerifyReportsAPathThatIsGoneAsMissing is case two: the record names a
// path that is no longer there, and the report has to say missing rather than
// leave the reader to infer it.
func TestVerifyReportsAPathThatIsGoneAsMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("HOME", home)

	gone := filepath.Join(home, ".config", "fish")
	if err := writeLastInstall(lastInstall{
		Timestamp: time.Now(),
		Version:   "v0.4.0",
		Entries:   []installEntry{{Key: "fish", Path: gone, Action: installActionCreated}},
	}); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	report := VerifyInstall()
	if report.Consistent {
		t.Errorf("a path the record names that is gone was reported as consistent:\n%s", verifyReportText(report))
	}
	if report.Unreadable {
		t.Errorf("a readable record read as unreadable:\n%s", verifyReportText(report))
	}
	text := verifyReportText(report)
	if !strings.Contains(text, "missing") || !strings.Contains(text, gone) {
		t.Errorf("the report does not name the gone path as missing:\n%s", text)
	}
}

// TestVerifyReportsABackupThatIsGoneAsMissing is case three: the path the run
// replaced may still be here while the copy that would make a rollback possible
// is gone, and that is what verification must catch.
func TestVerifyReportsABackupThatIsGoneAsMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("HOME", home)

	present := filepath.Join(home, ".config", "kitty")
	if err := os.MkdirAll(present, 0o755); err != nil {
		t.Fatalf("test setup: %v", err)
	}
	goneBackup := filepath.Join(home, ".config", "backups", "kitty")
	if err := writeLastInstall(lastInstall{
		Timestamp: time.Now(),
		Version:   "v0.4.0",
		Entries: []installEntry{{
			Key:    "kitty",
			Path:   present,
			Action: installActionReplaced,
			Backup: goneBackup,
		}},
	}); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	report := VerifyInstall()
	if report.Consistent {
		t.Errorf("a backup the record names that is gone was reported as consistent:\n%s", verifyReportText(report))
	}
	text := verifyReportText(report)
	if !strings.Contains(text, "missing") || !strings.Contains(text, "backup of kitty") || !strings.Contains(text, goneBackup) {
		t.Errorf("the report does not name the missing backup:\n%s", text)
	}
}

// TestVerifyReportsARecordItCannotParse is case four: a record file that is
// there and cannot be read is exit 2, not "nothing was installed here",
// because those are different facts about the machine.
func TestVerifyReportsARecordItCannotParse(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	path := lastInstallPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("test setup: %v", err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	report := VerifyInstall()
	if !report.Unreadable {
		t.Errorf("a record that cannot be parsed was not reported as unreadable:\n%s", verifyReportText(report))
	}
	if report.Consistent {
		t.Errorf("a record that cannot be read was reported as consistent:\n%s", verifyReportText(report))
	}
	if !strings.Contains(verifyReportText(report), "Could not read the record") {
		t.Errorf("the report does not say the record could not be read:\n%s", verifyReportText(report))
	}
}

// TestVerifyReportsNoRecordAsNothingInstalled is case five: a machine with no
// record is not a failure. It is "nothing was installed here", and the report
// says so without an exit code that a script would read as a broken machine.
func TestVerifyReportsNoRecordAsNothingInstalled(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	report := VerifyInstall()
	if report.Unreadable {
		t.Errorf("a machine with no record was reported as unreadable:\n%s", verifyReportText(report))
	}
	if !report.Consistent {
		t.Errorf("a machine with no record was reported as divergent:\n%s", verifyReportText(report))
	}
	if !strings.Contains(verifyReportText(report), "nothing was installed here") {
		t.Errorf("the report does not say nothing was installed here:\n%s", verifyReportText(report))
	}
}

// TestVerifyReadsALegacyRecordAsFoundEntries pins that verification goes
// through the same upgrade the panel does: a record written before Entries
// existed names its paths as found, not as replacements the old build never
// established.
func TestVerifyReadsALegacyRecordAsFoundEntries(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("HOME", home)

	present := filepath.Join(home, ".config", "nvim")
	if err := os.MkdirAll(present, 0o755); err != nil {
		t.Fatalf("test setup: %v", err)
	}
	if err := writeLastInstall(lastInstall{
		Timestamp: time.Now(),
		Version:   "v0.3.0",
		Files:     []string{"nvim: " + present},
	}); err != nil {
		t.Fatalf("test setup: %v", err)
	}

	report := VerifyInstall()
	if !report.Consistent {
		t.Errorf("a legacy record whose only path is here was reported as divergent:\n%s", verifyReportText(report))
	}
	text := verifyReportText(report)
	if !strings.Contains(text, "found and left alone by the run") {
		t.Errorf("a legacy record's path was not read as found:\n%s", text)
	}
}
