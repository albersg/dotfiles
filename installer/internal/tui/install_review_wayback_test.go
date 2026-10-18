package tui

import (
	"strings"
	"testing"
)

// The way back needs rows. At 80x24 the review screen shows the marker and the first row of
// the block and does not show the command, which is the limit docs/tui-installer.md names:
// this test fails if the limit changes without the document changing with it.
//
// The eighth pass measured the rendering at both sizes and handed over the rendering; the
// assertions are this file's, because a probe that only logs cannot fail.
func TestTheReviewWayBackAt80x24ShowsTheMarkerButNotTheCommand(t *testing.T) {
	at := func(t *testing.T, w, h int) string {
		t.Helper()
		m := NewModel()
		isolateGoldenTest(t, &m)
		m.Width = w
		m.Height = h
		m.Screen = ScreenInstallReview
		m.Choices.CreateBackup = true
		return plainView(m)
	}

	narrow := at(t, 80, 24)
	if !strings.Contains(narrow, "Way back") {
		t.Fatalf("the review screen at 80x24 does not even name the way back:\n%s", narrow)
	}
	if strings.Contains(narrow, "--revert") {
		t.Fatalf("the review screen at 80x24 now names --revert, so the limit has changed and docs/tui-installer.md has to change with it:\n%s", narrow)
	}

	wide := at(t, 100, 40)
	if !strings.Contains(wide, "--revert") || !strings.Contains(wide, ".dotfiles-backup") {
		t.Fatalf("the review screen at 100x40 no longer names the command and the directory:\n%s", wide)
	}
}
