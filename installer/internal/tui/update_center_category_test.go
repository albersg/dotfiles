package tui

import (
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/updates"
)

// ============================================================================
// A CATEGORY AT A TIME, AND WHAT A STOP LEFT BEHIND
// ============================================================================
//
// Selecting is per entry, which is what a list is for, but taking a manager's
// worth of updates one press at a time is work the list itself can save: what a
// filter has narrowed the screen to is a category, and one row takes it. The
// category is still only a selection -- the plan is shown and confirmed before
// anything runs -- and it never reaches past the filter or past a hold.
//
// The second guard here is about the one state a stopped sequence leaves: an entry
// the run was asked to move and never reached is a request, not a manager's
// refusal, and the report must not fold the two together.

// TestTheSelectVisibleRowTakesTheCategoryTheFilterShows pins the group row: its
// count is the entries that can actually be selected, one press takes them, the
// next puts them back, and a hold is never taken.
func TestTheSelectVisibleRowTakesTheCategoryTheFilterShows(t *testing.T) {
	brew := updateCenterAvailableAdapter(updates.ManagerBrew, updates.ScopeUser)
	apt := updateCenterAvailableAdapter(updates.ManagerApt, updates.ScopeSystem)
	git := updateCenterFakeItem(brew, "git", "2.51.0", "2.52.0")
	jq := updateCenterFakeItem(brew, "jq", "1.7.1", "1.8.0")
	held := updateCenterFakeItem(apt, "tar", "1.35-1", "1.35-2")
	held.Pinned, held.PinReason = true, "apt-mark holds tar"
	brew.reads[0].Items = []updates.Item{git, jq}
	apt.reads[0].Items = []updates.Item{held}

	m, _ := updateCenterTestModel(t, brew, apt)
	m, cmd := updateCenterOpened(t, m)
	m = updateCenterReadEveryManager(t, m, cmd)

	// The row counts what it would take: the two entries that can be selected,
	// not the three the screen is showing.
	groupRow := ""
	for _, row := range m.GetCurrentOptions() {
		if strings.HasPrefix(row, updateCenterSelectVisibleRowPrefix) {
			groupRow = row
		}
	}
	want := updateCenterSelectVisibleRowPrefix + "2 shown)"
	if groupRow != want {
		t.Fatalf("the category row is %q, want %q: its count is what can be selected, holds included in the list they are shown in", groupRow, want)
	}

	m, _ = pressUpdateCenterRow(t, m, groupRow)
	if got := m.UpdateCenter.Selected.Count(); got != 2 {
		t.Fatalf("the category row selected %d entries, want the 2 selectable ones it counted", got)
	}
	if !m.UpdateCenter.Selected.Has(git) || !m.UpdateCenter.Selected.Has(jq) {
		t.Error("the category row did not select both entries of the manager it was showing")
	}
	if m.UpdateCenter.Selected.Has(held) {
		t.Error("the category row selected a held entry, which this screen refuses one entry at a time")
	}

	// The same row now reads as the way back, and puts the category down. The check
	// is updateCenterIndexOfRow: hasRowWithName answers about entry rows, which is a different
	// question from whether an action row is on screen.
	dropRow := updateCenterDropVisibleRowPrefix + "2 shown)"
	if updateCenterIndexOfRow(m.GetCurrentOptions(), dropRow) < 0 {
		t.Fatalf("the category row does not say it would put the category back: %v", m.GetCurrentOptions())
	}
	m, _ = pressUpdateCenterRow(t, m, dropRow)
	if got := m.UpdateCenter.Selected.Count(); got != 0 {
		t.Fatalf("%d entries are still selected after the category was put back, want none", got)
	}

	// Narrowed to the manager whose every entry is held, there is nothing the row
	// could take, so the row is not offered at all.
	m.UpdateCenter.Filter = updates.ManagerApt
	m.Cursor = 0
	for _, row := range m.GetCurrentOptions() {
		if strings.HasPrefix(row, updateCenterSelectVisibleRowPrefix) {
			t.Errorf("the screen offers %q for a manager whose only entry is held, so the row could only lie", row)
		}
	}
}

// TestARequestedEntryThatNeverRanIsReportedAsRequested pins the distinction a
// stopped run has to keep: the entry whose command ran is unknown, and the entry
// whose command was never started is a request the run did not reach -- not an
// entry no command could carry.
func TestARequestedEntryThatNeverRanIsReportedAsRequested(t *testing.T) {
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

	next, _ := m.Update(updateCenterApplyDoneMsg{index: 0, result: updates.ApplyResult{
		Plan:   m.UpdateCenter.Plans[0],
		Status: updates.StatusCancelled,
		Outcomes: []updates.Outcome{
			{Item: first, Step: updates.StepUnknown, Detail: "the run was stopped before the manager answered"},
		},
	}})
	m = next.(Model)

	steps := map[string]updates.OutcomeStep{}
	for _, outcome := range m.updateCenterItemOutcomes() {
		steps[outcome.Item.Name] = outcome.Step
	}
	if steps["tar"] != updates.StepUnknown {
		t.Errorf("the entry the stopped command carried is reported as %q, want %q", steps["tar"], updates.StepUnknown)
	}
	if steps["libc6"] != updates.StepRequested {
		t.Errorf("the entry whose command was never started is reported as %q, want %q: a stop is not a manager refusing",
			steps["libc6"], updates.StepRequested)
	}

	notice := m.UpdateCenter.Notice
	if !strings.Contains(notice, "never run") {
		t.Errorf("the report does not say what was requested and never run: %q", notice)
	}
	if strings.Contains(notice, "left out") {
		t.Errorf("the report calls an entry whose command was never started an entry left out: %q", notice)
	}
}
