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

// TestTheRecheckKeyDismissesAStaleConfirmation pins what a re-read does to the
// screen that asks for confirmation: the plan was built from the answers being
// replaced, so it is dismissed rather than left on screen where Enter would apply
// a command chosen from a list that no longer exists.
func TestTheRecheckKeyDismissesAStaleConfirmation(t *testing.T) {
	apt := updateCenterAvailableAdapter(updates.ManagerApt, updates.ScopeSystem)
	tar := updateCenterFakeItem(apt, "tar", "1.35-1", "1.35-2")
	apt.reads[0].Items = []updates.Item{tar}
	m, _ := updateCenterTestModel(t, apt)
	m, cmd := updateCenterOpened(t, m)
	m = updateCenterReadEveryManager(t, m, cmd)

	m, _ = pressUpdateCenterKey(t, m, updateCenterKeySpace())
	m, _ = pressUpdateCenterRow(t, m, updateCenterApplyRow(1))
	if !m.UpdateCenter.Confirming {
		t.Fatal("the confirmation did not open, so the guard proves nothing about a stale plan")
	}

	m, read := pressUpdateCenterKey(t, m, updateCenterKeyRune('r'))
	if read == nil {
		t.Fatal("the re-check started no read")
	}
	if m.UpdateCenter.Confirming || len(m.UpdateCenter.Plans) != 0 {
		t.Error("the plan built from the answers being re-read is still on screen and confirmable")
	}
}

// TestAStoppedRunsReportDoesNotLeakIntoTheNextRun pins the boundary of a run's
// report. What a stopped sequence never reached belongs to that run: a later run,
// whose plan may have moved the same entry, must not inherit it and report an entry
// as requested and never run a second time.
func TestAStoppedRunsReportDoesNotLeakIntoTheNextRun(t *testing.T) {
	apt := updateCenterAvailableAdapter(updates.ManagerApt, updates.ScopeSystem)
	first := updateCenterFakeItem(apt, "tar", "1.35-1", "1.35-2")
	never := updateCenterFakeItem(apt, "libc6", "2.41-1", "2.41-2")
	m, _ := updateCenterRunningModel(t, apt, first)
	m.UpdateCenter.Plans = append(m.UpdateCenter.Plans, updates.Plan{
		Manager: updates.ManagerApt,
		Scope:   updates.ScopeSystem,
		Command: updates.Command{Argv: []string{"apt-get", "install", "-y", "libc6"}, Elevate: true},
		Items:   []updates.Item{never},
	})

	next, cmd := m.Update(updateCenterApplyDoneMsg{index: 0, result: updates.ApplyResult{
		Plan:   m.UpdateCenter.Plans[0],
		Status: updates.StatusCancelled,
		Outcomes: []updates.Outcome{
			{Item: first, Step: updates.StepUnknown, Detail: "the run was stopped before the manager answered"},
		},
	}})
	m = next.(Model)
	if len(m.UpdateCenter.NotReached) == 0 {
		t.Fatal("the stopped run recorded nothing it never reached, so the guard proves nothing")
	}
	// The run's own report survives the re-read it starts: it is the answer to the
	// question the user asked by running it.
	m = updateCenterReadEveryManager(t, m, cmd)
	if len(m.UpdateCenter.NotReached) == 0 {
		t.Fatal("the re-read that follows a run erased the run's own report")
	}

	m.UpdateCenter.Confirming = true
	m.UpdateCenter.Plans = []updates.Plan{{
		Manager: updates.ManagerApt,
		Scope:   updates.ScopeSystem,
		Command: updates.Command{Argv: []string{"apt-get", "install", "-y", "libc6"}, Elevate: true},
		Items:   []updates.Item{never},
	}}
	if apply := m.startUpdateCenterApply(); apply == nil {
		t.Fatal("the second run did not start, so the guard proves nothing")
	}
	if len(m.UpdateCenter.NotReached) != 0 {
		t.Errorf("the second run inherited the previous run's unreached entries: %+v", m.UpdateCenter.NotReached)
	}

	// The second run's own plan carries the same entry, so it is legitimately
	// "requested and not started yet" while the run is out. What must not survive is
	// the first run's stop being reported as this run's: the notice is what the user
	// reads, and it must count the entry once, as this run's outcome.
	next, _ = m.Update(updateCenterApplyDoneMsg{index: 0, result: updates.ApplyResult{
		Plan:   m.UpdateCenter.Plans[0],
		Status: updates.StatusCurrent,
		Outcomes: []updates.Outcome{
			{Item: never, Step: updates.StepConfirmed, Detail: "the manager no longer reports this as out of date"},
		},
	}})
	m = next.(Model)
	if notice := m.UpdateCenter.Notice; strings.Contains(notice, "never run") {
		t.Errorf("the second run's report still counts the previous run's stop as its own: %q", notice)
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
