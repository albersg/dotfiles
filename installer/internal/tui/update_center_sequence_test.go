package tui

import (
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/updates"
)

// ============================================================================
// ONE MANAGER AT A TIME, ON EVERY PATH
// ============================================================================
//
// The screen's central promise is that two managers never run together: apt,
// pacman, dnf and pkg keep locks over the same package state, so a second read
// while the first one is still out is a lock contention the user never asked for.
//
// These guards drive the three paths that could reach that state -- the re-check
// key, leaving the screen and coming back mid-pass, and confirming a plan while a
// read is still outstanding -- because a promise held on one path and not the
// others is not a promise. The guards after the first one are the paths where the
// key handler and the row handler agree by accident rather than by design.

// TestTheRecheckKeyStartsNoSecondPassWhileAReadIsOutstanding presses `r` in the
// one state where a second pass would put two managers on the machine at once:
// the first manager has answered and the second one's read is still outstanding.
func TestTheRecheckKeyStartsNoSecondPassWhileAReadIsOutstanding(t *testing.T) {
	brew := updateCenterAvailableAdapter(updates.ManagerBrew, updates.ScopeUser)
	apt := updateCenterAvailableAdapter(updates.ManagerApt, updates.ScopeSystem)
	m, _ := updateCenterTestModel(t, brew, apt)

	m, first := updateCenterOpened(t, m)
	// The first manager answers, which is what makes the screen ask the second
	// one: that read is now outstanding.
	m, readingApt := runUpdateCenterReadCmd(t, m, first)
	if readingApt == nil {
		t.Fatal("the pass did not ask the second manager, so no read was outstanding to re-enter")
	}
	if !m.UpdateCenter.InFlight {
		t.Fatal("the screen called the pass over while a manager's read was still outstanding")
	}

	m, again := pressUpdateCenterKey(t, m, updateCenterKeyRune('r'))
	if again != nil {
		t.Error("`r` dispatched a second reading pass while a manager's read was still outstanding: " +
			"two managers would run together over the same package state")
	}
	if !m.UpdateCenter.InFlight {
		t.Errorf("`r` declared the pass over while its read was still outstanding: notice=%q", m.UpdateCenter.Notice)
	}
}

// TestLeavingTheScreenAndReturningStartsNoSecondPass leaves mid-pass with Esc and
// comes back through the section's own row. The read the first pass started is
// still out, so re-entering must resume the pass that is running rather than
// start another one beside it.
func TestLeavingTheScreenAndReturningStartsNoSecondPass(t *testing.T) {
	brew := updateCenterAvailableAdapter(updates.ManagerBrew, updates.ScopeUser)
	apt := updateCenterAvailableAdapter(updates.ManagerApt, updates.ScopeSystem)
	m, _ := updateCenterTestModel(t, brew, apt)

	m, first := updateCenterOpened(t, m)
	m, readingApt := runUpdateCenterReadCmd(t, m, first)
	if readingApt == nil {
		t.Fatal("the pass did not ask the second manager, so no read was outstanding to re-enter")
	}

	m, _ = pressUpdateCenterKey(t, m, updateCenterKeyEsc())
	if m.Screen != ScreenUtilities {
		t.Fatalf("Esc left the screen on %v, want the section it was opened from", m.Screen)
	}

	m, again := pressUpdateCenterRow(t, m, utilitiesUpdatesRow)
	if again != nil {
		t.Error("returning to the screen started a second reading pass while the first one's read was outstanding")
	}
	if !m.UpdateCenter.InFlight {
		t.Errorf("returning to the screen declared the unfinished pass over: notice=%q", m.UpdateCenter.Notice)
	}
}

// TestApplyIsRefusedWhileAReadIsStillOutstanding holds the other half of the same
// promise: a plan is built from the list the managers gave, and confirming one
// while a manager is still being read is a command built from half an answer.
// The row is not on the screen in that state, and the handler refuses it anyway,
// because a row the screen declines to draw is not a guarantee.
func TestApplyIsRefusedWhileAReadIsStillOutstanding(t *testing.T) {
	brew := updateCenterAvailableAdapter(updates.ManagerBrew, updates.ScopeUser)
	apt := updateCenterAvailableAdapter(updates.ManagerApt, updates.ScopeSystem)
	brewItem := updateCenterFakeItem(brew, "git", "2.51.0", "2.52.0")
	brew.reads[0].Items = []updates.Item{brewItem}
	m, _ := updateCenterTestModel(t, brew, apt)

	m, first := updateCenterOpened(t, m)
	m, readingApt := runUpdateCenterReadCmd(t, m, first)
	if readingApt == nil {
		t.Fatal("the pass did not ask the second manager, so no read was outstanding")
	}

	// Selecting the entry that has already been read, while apt's read is out.
	m, _ = pressUpdateCenterKey(t, m, updateCenterKeySpace())
	if m.UpdateCenter.Selected.Count() == 0 {
		t.Fatal("the guard selected nothing, so it proves nothing about applying")
	}
	for _, row := range m.GetCurrentOptions() {
		if strings.HasPrefix(row, updateCenterApplyRowPrefix) {
			t.Errorf("the screen offers %q while a manager is still being read: a plan must not be confirmed from half an answer", row)
		}
	}

	// The absent row is not the only gate. The state machine refuses the same
	// thing, so a plan reached by some other path cannot start its commands either
	// -- which is the half of this rule a guard on the drawn rows cannot prove.
	m.UpdateCenter.Plans = []updates.Plan{{
		Manager: updates.ManagerBrew,
		Scope:   updates.ScopeUser,
		Command: updates.Command{Argv: []string{"brew", "upgrade", "git"}},
		Items:   []updates.Item{brewItem},
	}}
	m.UpdateCenter.Confirming = true
	apply := m.startUpdateCenterApply()
	if apply != nil || m.UpdateCenter.Applying {
		t.Error("the state machine started a plan while a manager's read was still outstanding")
	}
}
