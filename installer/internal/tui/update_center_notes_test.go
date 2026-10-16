package tui

import (
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/updates"
)

// ============================================================================
// WHAT THE MANAGER SAID BESIDE ITS ANSWER
// ============================================================================
//
// A manager's answer is not only a list. apt says which entries it is keeping
// back on its own dependency grounds, pacman says when it could not read its
// ignore list, Homebrew says when its metadata was not refreshed, flatpak says
// when it had to assume an installation. Those are caveats about the list being
// incomplete, stale or built on an assumption -- so a screen that drops them
// presents an answer as complete that the manager itself did not.
//
// These guards hold the two places those caveats can be read: the channel's own
// line, and the row of the entry the caveat is about.

// TestTheChannelsOwnCaveatsReachTheScreen pins a channel's notes: what the
// manager said about its own answer, in the manager's own words, on the screen.
func TestTheChannelsOwnCaveatsReachTheScreen(t *testing.T) {
	apt := updateCenterAvailableAdapter(updates.ManagerApt, updates.ScopeSystem)
	apt.reads[0].Notes = []string{
		"apt is holding these back on its own dependency grounds, so they are not offered here: libc6, perl-base",
	}
	m, _ := updateCenterTestModel(t, apt)
	m, cmd := updateCenterOpened(t, m)
	m, _ = runUpdateCenterReadCmd(t, m, cmd)

	body := strings.Join(m.updateCenterDescription(), " ")
	if !strings.Contains(body, "libc6") {
		t.Errorf("the manager's own caveat about its answer is not on the screen, so an incomplete list reads as a complete one:\n%s", body)
	}
}

// TestAnEntrysOwnNoteReachesItsRow pins the note that belongs to one entry: a
// version the manager does not report, a cask the vendor maintains itself. The
// row is where the reader decides about that entry, so the caveat belongs there
// rather than in a paragraph the entry is not named in.
func TestAnEntrysOwnNoteReachesItsRow(t *testing.T) {
	brew := updateCenterAvailableAdapter(updates.ManagerBrew, updates.ScopeUser)
	git := updateCenterFakeItem(brew, "git", "", "2.52.0")
	git.Note = "Homebrew did not report the installed version"
	brew.reads[0].Items = []updates.Item{git}
	m, _ := updateCenterTestModel(t, brew)
	m, cmd := updateCenterOpened(t, m)
	m, _ = runUpdateCenterReadCmd(t, m, cmd)

	for _, row := range m.GetCurrentOptions() {
		if strings.Contains(row, "git") && strings.Contains(row, "did not report") {
			return
		}
	}
	t.Errorf("the entry's own note is not in its row, so the row shows \"unknown\" without saying why:\n%s",
		strings.Join(m.GetCurrentOptions(), "\n"))
}

// TestTheUtilitiesPanelNeverCallsAFailedReadNothingToUpdate pins the panel's one
// line against the two facts it must not confuse. A channel that could not be
// asked is not a machine with nothing to update: saying so would retire the
// question with an answer nobody gave. The second half is the control: a channel
// that did answer "nothing to update" must still say it.
func TestTheUtilitiesPanelNeverCallsAFailedReadNothingToUpdate(t *testing.T) {
	offline := &tuiFakeAdapter{
		id:    updates.ManagerApt,
		label: "apt",
		scope: updates.ScopeSystem,
		avail: updates.Availability{Available: true},
		reads: []updates.Result{{
			Manager: updates.ManagerApt, Label: "apt", Scope: updates.ScopeSystem,
			Status:  updates.StatusOffline,
			Reason:  "could not reach its source",
			Command: "apt-get --simulate -o Debug::NoLocking=1 upgrade",
		}},
	}
	m, _ := updateCenterTestModel(t, offline)
	m, cmd := updateCenterOpened(t, m)
	m, _ = runUpdateCenterReadCmd(t, m, cmd)

	if got := updateCenterPanelValue(m.UpdateCenter); got == "nothing to update" {
		t.Errorf("the panel summarises an offline read as %q, which claims the machine is up to date", got)
	}
	if got := updateCenterPanelValue(m.UpdateCenter); got == "not checked yet" {
		t.Errorf("the panel summarises an answered-but-failed read as %q, which claims the question was never asked", got)
	}

	current := updateCenterAvailableAdapter(updates.ManagerBrew, updates.ScopeUser)
	fresh, _ := updateCenterTestModel(t, current)
	fresh, freshCmd := updateCenterOpened(t, fresh)
	fresh, _ = runUpdateCenterReadCmd(t, fresh, freshCmd)
	if got := updateCenterPanelValue(fresh.UpdateCenter); got != "nothing to update" {
		t.Errorf("a manager that answered \"nothing to update\" is summarised as %q, so the honest answer was lost", got)
	}
}
