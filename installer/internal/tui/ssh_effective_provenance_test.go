package tui

import (
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/sshcfg"
)

// TestSSHEffectiveRowsShowTheJumpHostsSource pins that every value the entry
// detail shows carries the file and line that decided it. The jump host is the
// value this was written for: its label and the key the effective configuration
// files it under do not spell the same thing, so a suffix derived from the label
// silently disappears for that one row -- and a source the interface chose not to
// show is worse than no source at all, because the other rows teach the user to
// expect it.
func TestSSHEffectiveRowsShowTheJumpHostsSource(t *testing.T) {
	eff := sshcfg.Effective{Alias: "web", ProxyJump: "bastion", Sources: map[string]sshcfg.Source{
		"proxyjump": {Key: "proxyjump", Value: "bastion", File: "/home/alice/.ssh/config", Line: 9},
	}}
	rows := strings.Join(sshEffectiveRows(nil, eff), "\n")
	if !strings.Contains(rows, "/home/alice/.ssh/config:9") {
		t.Fatalf("the jump host lost its source: %s", rows)
	}
}

// TestSSHEffectiveRowsShortenTheSourceAgainstTheHome covers the other half of the
// same row: a source is a path the user has to recognise. Printing the absolute
// path makes the row longer than it needs to be and hides that the file is the
// one under their own home, which is what the rest of these screens call ~.
func TestSSHEffectiveRowsShortenTheSourceAgainstTheHome(t *testing.T) {
	st := sshTestManagerState()
	eff := sshcfg.Effective{Alias: "web", User: "ops", Sources: map[string]sshcfg.Source{
		"user": {Key: "user", Value: "ops", File: "/home/testuser/.ssh/config", Line: 7},
	}}
	rows := strings.Join(sshEffectiveRows(st.Manager, eff), "\n")
	if !strings.Contains(rows, "(~/.ssh/config:7)") {
		t.Fatalf("the source was not shortened against the home it lives in: %s", rows)
	}
}
