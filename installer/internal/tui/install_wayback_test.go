package tui

import (
	"path/filepath"
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
	m.BackupDir = filepath.Join(t.TempDir(), ".dotfiles-backup-2026-10-16-120000")
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
