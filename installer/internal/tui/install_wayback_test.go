package tui

import (
	"strings"
	"testing"
)

// T6, S2/S7: the way back is named where a person looks -- on the failure screen
// and on the review screen before a run -- and what no command undoes is named
// beside it.

// plainView is the frame as text, with the styling taken out: a phrase is asserted
// as a phrase, not as a run of escape codes.
func plainView(m Model) string {
	return ansiEscape.ReplaceAllString(m.View(), "")
}

// A failure screen that offers retry and quit and never mentions the backup leaves
// the reader with the one question the screen raised and no answer to it.
func TestTheFailureScreenNamesTheWayBack(t *testing.T) {
	m := NewModel()
	isolateGoldenTest(t, &m)
	m.Width = 100
	m.Height = 30
	m.Screen = ScreenError
	m.ErrorMsg = "Step 'Install zsh shell' failed"
	// A short, absolute path written out by hand, and not one built from t.TempDir(). This screen can
	// only name a path that fits its row, and a temp directory is long enough on a macOS runner for
	// the path to be cut with a trailing ellipsis -- which made this test pass on a machine with a
	// short temp directory and fail in CI (macos-latest, PR 354). The cut itself is deliberate and
	// is pinned, separately, by TestALongBackupPathIsCutButTheCommandIsNot.
	m.BackupDir = "/home/testuser/.dotfiles-backup-2026-10-16-120000"
	m.LogLines = []string{"a log line for context"}

	view := plainView(m)

	for _, want := range []string{m.BackupDir, "--revert", "Not undone"} {
		if !strings.Contains(view, want) {
			t.Errorf("the failure screen does not name %q:\n%s", want, view)
		}
	}
}

// A run that took no backup cannot be put back the same way, and the screen has to
// say which of the two machines the reader is on rather than offering a command
// that will report the file as unrevertible.
func TestTheFailureScreenSaysWhenTheRunTookNoBackup(t *testing.T) {
	m := NewModel()
	isolateGoldenTest(t, &m)
	m.Width = 100
	m.Height = 30
	m.Screen = ScreenError
	m.ErrorMsg = "Step 'Install zsh shell' failed"

	view := plainView(m)

	if !strings.Contains(view, "--revert") {
		t.Errorf("the failure screen does not name the way back:\n%s", view)
	}
	if !strings.Contains(view, "took no backup") {
		t.Errorf("the failure screen does not say that no backup was taken:\n%s", view)
	}
	if !strings.Contains(view, "Not undone") {
		t.Errorf("the failure screen does not name what no command undoes:\n%s", view)
	}
}

// The review screen is where a person can still decide not to start, so it is the
// screen that has to say what the run will keep and what it will not.
func TestTheReviewScreenNamesTheWayBackBeforeTheRun(t *testing.T) {
	m := NewModel()
	isolateGoldenTest(t, &m)
	m.Width = 100
	m.Height = 40
	m.Screen = ScreenInstallReview
	m.Choices.CreateBackup = true

	view := plainView(m)

	for _, want := range []string{".dotfiles-backup", "--revert", "Not undone"} {
		if !strings.Contains(view, want) {
			t.Errorf("the review screen does not name %q:\n%s", want, view)
		}
	}
}

// A path longer than its row cannot be named in full: the screen cuts it rather than spending the
// block on it. What it must never cut is the command, because that row is the reason the block is
// there, and the directory it cuts is named in full in the run's record. A stated limit in
// docs/tui-installer.md, not an accident.
func TestALongBackupPathIsCutButTheCommandIsNot(t *testing.T) {
	m := NewModel()
	isolateGoldenTest(t, &m)
	m.Width = 100
	m.Height = 30
	m.Screen = ScreenError
	m.ErrorMsg = "Step 'Install zsh shell' failed"
	long := "/Users/somebody-with-a-long-account-name/Documents/things/backups/.dotfiles-backup-2026-10-16-120000"
	m.BackupDir = long

	view := plainView(m)

	if !strings.Contains(view, "\u2026") {
		t.Errorf("a path longer than its row is not shown as cut:\n%s", view)
	}
	if strings.Contains(view, long) {
		t.Errorf("the screen claims to name a path longer than its row:\n%s", view)
	}
	if !strings.Contains(view, "--revert") {
		t.Errorf("the command was cut together with the path:\n%s", view)
	}
}

// And with backups off, the honest sentence is the one that says a revert will be
// able to remove and not to restore: that is the decision being made on this
// screen.
func TestTheReviewScreenSaysWhenBackupsAreOff(t *testing.T) {
	m := NewModel()
	isolateGoldenTest(t, &m)
	m.Width = 100
	m.Height = 40
	m.Screen = ScreenInstallReview
	m.Choices.CreateBackup = false

	view := plainView(m)

	if !strings.Contains(view, "Backups are off") {
		t.Errorf("the review screen does not say backups are off:\n%s", view)
	}
	if !strings.Contains(view, "--revert") {
		t.Errorf("the review screen does not name the way back:\n%s", view)
	}
}
