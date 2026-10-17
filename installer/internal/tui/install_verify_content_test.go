package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// T5, S11: --verify reports whether a path still holds what the run left, not
// merely that it exists, and it does not overclaim about what it cannot check.

// writeInstallRecord writes a record the way a run writes one, so the verification
// is tested against the file an install really produces.
func writeInstallRecord(t *testing.T, rec lastInstall) {
	t.Helper()

	if err := writeLastInstall(rec); err != nil {
		t.Fatalf("writing the record: %v", err)
	}
}

func recordWithZshrc(t *testing.T, zshrc string, change fileChange) {
	t.Helper()

	writeInstallRecord(t, lastInstall{
		Timestamp: time.Now(),
		Version:   "test",
		Status:    runStatusComplete,
		Entries:   []installEntry{{Key: "zsh", Path: zshrc, Action: installActionReplaced}},
		Changes:   []fileChange{change},
	})
}

// A record that names a digest turns "the file is here" into a fact: it holds what
// the run left, or it does not. The second case is what --verify could never say.
func TestVerifySaysWhetherAFileStillHoldsWhatTheRunLeft(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))

	zshrc := system.ConfigPaths()["zsh"]
	if err := os.WriteFile(zshrc, []byte("what the run left"), 0o644); err != nil {
		t.Fatal(err)
	}
	left := fileDigest(zshrc)
	recordWithZshrc(t, zshrc, fileChange{
		Key: "zsh", Path: zshrc, Action: fileActionModified, Before: "what-it-held-before", After: left,
	})

	report := VerifyInstall()
	joined := strings.Join(report.Lines, "\n")
	if !report.Consistent {
		t.Errorf("the machine is reported as not what the install left, while the file still holds it:\n%s", joined)
	}
	if !strings.Contains(joined, "still holds what the run left") {
		t.Errorf("the report does not say the file still holds what the run left:\n%s", joined)
	}

	// The person edits it afterwards. That is not the install's doing, and the
	// report has to say the file no longer holds what the run left rather than
	// reporting it as here and leaving the reader to guess.
	if err := os.WriteFile(zshrc, []byte("the person's own edit"), 0o644); err != nil {
		t.Fatal(err)
	}

	edited := VerifyInstall()
	joined = strings.Join(edited.Lines, "\n")
	if edited.Consistent {
		t.Errorf("the report calls the machine what the install left, after the file changed:\n%s", joined)
	}
	if !strings.Contains(joined, "no longer holds") {
		t.Errorf("the report does not say the file no longer holds what the run left:\n%s", joined)
	}
	if strings.Contains(joined, "missing") {
		t.Errorf("the report calls a changed file missing:\n%s", joined)
	}
}

// The path-level record could never see inside a directory: one key, one line,
// whatever happened under it. The per-file record can, and that is the case the
// report had no answer for.
func TestVerifyReportsAFileInsideADirectoryThatChanged(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))

	nvim := system.ConfigPaths()["nvim"]
	if err := os.MkdirAll(nvim, 0o755); err != nil {
		t.Fatal(err)
	}
	init := filepath.Join(nvim, "init.lua")
	if err := os.WriteFile(init, []byte("what the run left"), 0o644); err != nil {
		t.Fatal(err)
	}
	left := fileDigest(init)

	writeInstallRecord(t, lastInstall{
		Timestamp: time.Now(),
		Version:   "test",
		Status:    runStatusComplete,
		Entries:   []installEntry{{Key: "nvim", Path: nvim, Action: installActionReplaced}},
		Changes:   []fileChange{{Key: "nvim", Path: init, Action: fileActionModified, After: left}},
	})

	if report := VerifyInstall(); !report.Consistent {
		t.Errorf("a file that holds what the run left is reported as changed:\n%s", strings.Join(report.Lines, "\n"))
	}

	if err := os.WriteFile(init, []byte("something else entirely"), 0o644); err != nil {
		t.Fatal(err)
	}

	report := VerifyInstall()
	joined := strings.Join(report.Lines, "\n")
	if report.Consistent {
		t.Errorf("a file under a directory key changed and the report did not notice:\n%s", joined)
	}
	if !strings.Contains(joined, init) {
		t.Errorf("the report does not name the file that changed:\n%s", joined)
	}
}

// A file the run removed is part of the state the run left: it belongs in the
// report both while it stays gone and when something puts it back.
func TestVerifySaysWhenAFileTheRunRemovedCameBack(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))

	nvim := system.ConfigPaths()["nvim"]
	if err := os.MkdirAll(nvim, 0o755); err != nil {
		t.Fatal(err)
	}
	dropped := filepath.Join(nvim, "dropped.lua")

	writeInstallRecord(t, lastInstall{
		Timestamp: time.Now(),
		Version:   "test",
		Status:    runStatusComplete,
		Entries:   []installEntry{{Key: "nvim", Path: nvim, Action: installActionReplaced}},
		Changes:   []fileChange{{Key: "nvim", Path: dropped, Action: fileActionDeleted, Before: "what-it-held-before"}},
	})

	if report := VerifyInstall(); !report.Consistent {
		t.Errorf("a file the run removed and that is still gone is reported as a difference:\n%s", strings.Join(report.Lines, "\n"))
	}

	if err := os.WriteFile(dropped, []byte("back again"), 0o644); err != nil {
		t.Fatal(err)
	}

	report := VerifyInstall()
	joined := strings.Join(report.Lines, "\n")
	if report.Consistent {
		t.Errorf("a file the run removed came back and the report did not notice:\n%s", joined)
	}
	if !strings.Contains(joined, dropped) {
		t.Errorf("the report does not name the file that came back:\n%s", joined)
	}
}

// A file the run created that is no longer here is a difference in the other
// direction, and it is the one a person is most likely to care about.
func TestVerifySaysWhenAFileTheRunCreatedIsGone(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))

	nvim := system.ConfigPaths()["nvim"]
	if err := os.MkdirAll(nvim, 0o755); err != nil {
		t.Fatal(err)
	}
	added := filepath.Join(nvim, "added.lua")

	writeInstallRecord(t, lastInstall{
		Timestamp: time.Now(),
		Version:   "test",
		Status:    runStatusComplete,
		Entries:   []installEntry{{Key: "nvim", Path: nvim, Action: installActionReplaced}},
		Changes:   []fileChange{{Key: "nvim", Path: added, Action: fileActionCreated, After: "what-the-run-wrote"}},
	})

	report := VerifyInstall()
	if report.Consistent {
		t.Errorf("a file the run created is gone and the report did not notice:\n%s", strings.Join(report.Lines, "\n"))
	}
	if verdict := InspectInstall(); verdict.Missing() == 0 {
		t.Errorf("the file the run created is gone and nothing is counted as missing:\n%s", strings.Join(report.Lines, "\n"))
	}
}

// The limit has to survive: a record written before digests existed cannot say
// whether a file is still what the install left, and the report must say that
// rather than turning presence into a stronger claim.
func TestVerifyStillSaysWhatItCannotCheck(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))

	zshrc := system.ConfigPaths()["zsh"]
	if err := os.WriteFile(zshrc, []byte("anything at all"), 0o644); err != nil {
		t.Fatal(err)
	}

	writeInstallRecord(t, lastInstall{
		Timestamp: time.Now(),
		Version:   "old",
		Files:     []string{"zsh: " + zshrc},
	})

	report := VerifyInstall()
	joined := strings.Join(report.Lines, "\n")
	if !report.Consistent {
		t.Errorf("a record that names no digest cannot make the machine inconsistent:\n%s", joined)
	}
	if !strings.Contains(joined, "the record names no digest") {
		t.Errorf("the report does not say which part of it cannot be checked:\n%s", joined)
	}
}

// A file that is here and cannot be read is a third answer, and the honest one:
// the report may not call it changed, and may not call it fine either.
func TestVerifySaysWhenAFileCouldNotBeRead(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: file permissions do not withhold anything")
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))

	zshrc := system.ConfigPaths()["zsh"]
	if err := os.WriteFile(zshrc, []byte("what the run left"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(zshrc, 0o000); err != nil {
		t.Fatal(err)
	}

	recordWithZshrc(t, zshrc, fileChange{
		Key: "zsh", Path: zshrc, Action: fileActionModified, Before: "before", After: "what-the-run-left",
	})

	report := VerifyInstall()
	joined := strings.Join(report.Lines, "\n")
	if report.Consistent {
		t.Errorf("a file that could not be read is reported as holding what the run left:\n%s", joined)
	}
	if !strings.Contains(joined, "could not be read") {
		t.Errorf("the report does not say the file could not be read:\n%s", joined)
	}
	if strings.Contains(joined, "no longer holds") {
		t.Errorf("the report says a file it could not read no longer holds what the run left:\n%s", joined)
	}
}
