package tui

import (
	"strings"
	"testing"
)

// focusUnitRow puts the cursor on the plan row for a step, failing when the plan
// does not hold one. It is how a guard says "the row for this unit" without
// writing an index down: the row list is the plan's, and the plan's length moves
// with the machine and the choices.
func focusUnitRow(t *testing.T, m *Model, id string) {
	t.Helper()
	for i, row := range m.planReviewRows() {
		if row.Kind == planRowStep && row.StepID == id {
			m.UnitCursor = i
			return
		}
	}
	t.Fatalf("the plan has no row for %q", id)
}

// TestEveryCataloguedUnitHasALabelOnTheScreen holds the screen's own copy of the
// catalogue to the catalogue itself, in both directions. A unit without a label
// would read as its own ID on screen, and a label for a unit the catalogue dropped
// is a row that can never appear -- a name that rots while looking maintained.
func TestEveryCataloguedUnitHasALabelOnTheScreen(t *testing.T) {
	for _, unit := range installUnits {
		label, ok := installReviewLabels[unit.ID]
		if !ok || strings.TrimSpace(label) == "" {
			t.Errorf("the catalogue names %q and the review screen has no label for it, so its row would show the ID", unit.ID)
		}
	}

	known := map[string]bool{}
	for _, unit := range installUnits {
		known[unit.ID] = true
	}
	for id := range installReviewLabels {
		if !known[id] {
			t.Errorf("the review screen has a label for %q, but the catalogue does not name that unit", id)
		}
	}
}

// TestTheScreenListsEveryPlannedStepTurnedOn pins S2 with S4: the screen's rows
// are the run's plan, each step once, and with nothing touched every row is on,
// because the default selection is the empty disabled set.
func TestTheScreenListsEveryPlannedStepTurnedOn(t *testing.T) {
	m := planFixtures()[0].fixtureModel()
	m.Screen = ScreenInstallReview

	steps := planFor(m.planOptions())
	rows := planReviewStepRows(m.planReviewRows())
	if len(rows) != len(steps) {
		t.Fatalf("the screen draws %d step rows for a plan of %d steps", len(rows), len(steps))
	}
	for i, step := range steps {
		if rows[i].StepID != step.ID {
			t.Errorf("step row %d is %q, want %q", i, rows[i].StepID, step.ID)
		}
		if !strings.Contains(rows[i].Text, installReviewLabel(step.ID)) {
			t.Errorf("the row for %q does not name it: %q", step.ID, rows[i].Text)
		}
		if !strings.HasPrefix(rows[i].Text, "[x] ") {
			t.Errorf("the row for %q is not shown as installed with nothing touched: %q", step.ID, rows[i].Text)
		}
	}

	kept, off := m.installReviewCounts()
	if kept != len(steps) || off != 0 {
		t.Errorf("an untouched selection counts %d on and %d off, want %d and 0", kept, off, len(steps))
	}
}

// TestTheThreeUnitsTheRunNeedsCannotBeTurnedOff pins S5's second half. Both halves
// of it matter: the key must not change the selection, and the row has to say why,
// because a row that simply refuses to change reads as a broken key.
func TestTheThreeUnitsTheRunNeedsCannotBeTurnedOff(t *testing.T) {
	m := planFixtures()[0].fixtureModel()
	m.Screen = ScreenInstallReview

	checked := 0
	for _, step := range planFor(m.planOptions()) {
		if !installUnitIsFixed(step.ID) {
			continue
		}
		checked++
		focusUnitRow(t, m, step.ID)
		if m.toggleUnitAtCursor() {
			t.Errorf("toggling %q changed the selection, but the run needs it", step.ID)
		}
		if m.unitIsDisabled(step.ID) {
			t.Errorf("%q ended up turned off", step.ID)
		}

		row := installReviewStepRow(step)
		if !strings.Contains(row, "the run needs it") {
			t.Errorf("the row for %q does not say the run needs it: %q", step.ID, row)
		}
		if !strings.HasPrefix(row, "[x] ") {
			t.Errorf("the row for %q does not read as installed: %q", step.ID, row)
		}
	}
	if checked != 3 {
		t.Errorf("the full plan carries %d of the three units the run needs, want 3", checked)
	}
}

// TestTurningAUnitOffAndOnAgainLeavesTheSelectionAsItWas is the round trip: the key
// works on an ordinary unit, the row follows it, and the count with it.
func TestTurningAUnitOffAndOnAgainLeavesTheSelectionAsItWas(t *testing.T) {
	m := planFixtures()[0].fixtureModel()
	m.Screen = ScreenInstallReview
	steps := planFor(m.planOptions())

	focusUnitRow(t, m, "toolset")

	if !m.toggleUnitAtCursor() {
		t.Fatalf("toggling toolset reported no change")
	}
	if !m.unitIsDisabled("toolset") {
		t.Fatalf("toolset is not turned off after the toggle")
	}
	if row := m.planReviewRows()[m.UnitCursor]; !strings.HasPrefix(row.Text, "[ ] ") {
		t.Errorf("the row does not follow the switch: %q", row.Text)
	}
	kept, off := m.installReviewCounts()
	if off != 1 || kept != len(steps)-1 {
		t.Errorf("counts are %d on and %d off, want %d and 1", kept, off, len(steps)-1)
	}

	if !m.toggleUnitAtCursor() {
		t.Fatalf("toggling toolset back reported no change")
	}
	if m.unitIsDisabled("toolset") {
		t.Errorf("toolset is still turned off after the second toggle")
	}
	if len(m.DisabledUnits) != 0 {
		t.Errorf("the round trip left %v behind", m.DisabledUnits)
	}
}

// TestTurningAUnitOffDoesNotWriteThroughTheSliceItWasGiven is the hazard this
// model shape carries: Update returns the model by value, so the disabled set it
// was built with may be shared with the previous copy. Both directions build a new
// slice; if either edited in place, the earlier model would silently change too,
// which is the kind of defect that shows up as a selection that cannot be undone.
func TestTurningAUnitOffDoesNotWriteThroughTheSliceItWasGiven(t *testing.T) {
	shared := []string{"toolset"}
	m := planFixtures()[0].fixtureModel()
	m.Screen = ScreenInstallReview
	m.DisabledUnits = shared
	focusUnitRow(t, m, "officecli")

	m.toggleUnitAtCursor()
	if len(shared) != 1 || shared[0] != "toolset" {
		t.Fatalf("turning a unit off wrote through the slice it was given: %v", shared)
	}

	back := planFixtures()[0].fixtureModel()
	back.Screen = ScreenInstallReview
	back.DisabledUnits = shared
	focusUnitRow(t, back, "toolset")
	back.toggleUnitAtCursor()
	if len(shared) != 1 || shared[0] != "toolset" {
		t.Fatalf("turning a unit back on wrote through the slice it was given: %v", shared)
	}
}

// TestTheStartKeyStartsTheRunFromTheScreen is what keeps the screen from being
// decorative: the start key records the plan from the choices and starts the run,
// so the units the user just reviewed are the units the plan holds. Enter, which
// opens and chooses everywhere else, must not start it: a row here is a switch.
func TestTheStartKeyStartsTheRunFromTheScreen(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenInstallReview
	m.Choices = UserChoices{OS: "linux", Shell: "zsh", Terminal: "none", WindowMgr: "none"}

	if idle, cmd := m.handleInstallReviewKeys("enter"); cmd != nil || idle.(Model).Screen != ScreenInstallReview {
		t.Error("Enter started the run; the screen's decision carries its own letter")
	}

	next, cmd := m.handleInstallReviewKeys("s")
	started, ok := next.(Model)
	if !ok {
		t.Fatalf("the start key returned %T, want a Model", next)
	}
	if started.Screen != ScreenInstalling {
		t.Errorf("the start key left the screen on %v, want the installing screen", started.Screen)
	}
	if len(started.Steps) == 0 {
		t.Errorf("the run started with no plan")
	}
	if cmd == nil {
		t.Errorf("the start key did not start the run: no command came back")
	}
}

// TestEscapeGoesBackToTheQuestionTheReviewCameFrom is S5's other direction. The
// answer is recorded when the screen opens, so it is the question the user came
// from rather than a fixed one -- the backup question when there were configs to
// ask about, the last question otherwise.
func TestEscapeGoesBackToTheQuestionTheReviewCameFrom(t *testing.T) {
	for _, from := range []Screen{ScreenNvimSelect, ScreenBackupConfirm} {
		m := NewModel()
		m.Screen = from
		m.openInstallReview()

		if m.Screen != ScreenInstallReview {
			t.Fatalf("opening the review from %v left the screen on %v", from, m.Screen)
		}
		if m.PrevScreen != from {
			t.Errorf("the review opened from %v remembers %v", from, m.PrevScreen)
		}

		back, _ := m.handleInstallReviewKeys("esc")
		if got := back.(Model).Screen; got != from {
			t.Errorf("Escape from the review opened on %v landed on %v", from, got)
		}
	}
}

// TestTheBodySaysWhatTheKeysDo is the same rule the password screen carries: the
// screen has to answer what to press rather than assuming the legend is read. The
// sentence is asked for through the description, which is also what the frame
// guard renders at every size, so the two cannot disagree.
//
// The terminal is named here too, and for a second reason: the footer packs its hints
// into at most two rows within the width it has, and a model with no width wraps them
// past that and silently loses the last two -- which is exactly how this guard first
// failed, on a screen whose footer was correct.
func TestTheBodySaysWhatTheKeysDo(t *testing.T) {
	m := Model{Screen: ScreenInstallReview, Width: 80, Height: 24}
	description := m.GetScreenDescription()

	for _, want := range []string{"Space turns one off or back on"} {
		if !strings.Contains(description, want) {
			t.Errorf("the screen's description does not say %q: %q", want, description)
		}
	}
	rendered := m.renderInstallReview()
	for _, hint := range []installerHint{hintToggle, hintStartInstall, hintBack} {
		if !strings.Contains(rendered, hint.keys) {
			t.Errorf("the screen does not show the %s key", hint.keys)
		}
	}
}

// TestTheWindowKeepsTheCursorInsideIt walks every cursor position, because the list
// is longer than the window and the one thing a window must never do is scroll the
// cursor off its own screen.
//
// The size here is the guard's own rather than the screen's: what is being measured
// is the arithmetic of a small window over a long list, and taking the size from the
// screen would let a screen that stopped sizing its window hide the defect.
func TestTheWindowKeepsTheCursorInsideIt(t *testing.T) {
	const window = 7
	m := planFixtures()[0].fixtureModel()
	total := len(m.planReviewRows())
	if total <= window {
		t.Fatalf("the plan draws %d rows and the guard's window shows %d, so this guard would test nothing", total, window)
	}

	for cursor := 0; cursor < total; cursor++ {
		start, end := windowRowsFor(total, cursor, window)
		if start < 0 || end > total || end < start {
			t.Fatalf("cursor %d gives the window [%d,%d) out of %d", cursor, start, end, total)
		}
		if end-start > window {
			t.Fatalf("cursor %d draws %d rows, more than the window's %d", cursor, end-start, window)
		}
		if cursor < start || cursor >= end {
			t.Errorf("cursor %d is outside the window [%d,%d) it draws", cursor, start, end)
		}
	}

	if start, _ := windowRowsFor(total, 0, window); start != 0 {
		t.Errorf("on the first row the window starts at %d, want 0", start)
	}
	if _, end := windowRowsFor(total, total-1, window); end != total {
		t.Errorf("on the last row the window ends at %d, want %d", end, total)
	}
}

// TestTheWindowIsSizedByTheTerminalAndNotByANumber is the reason the screen takes a
// fixed number of rows from nowhere: a terminal that leaves less room has to show a
// shorter list, and the one that leaves more has to show all of it. The 60x20 case of
// the matrix is the size that made a fixed number wrong.
func TestTheWindowIsSizedByTheTerminalAndNotByANumber(t *testing.T) {
	m := planFixtures()[0].fixtureModel()
	total := len(m.planReviewRows())

	short := listWindowSize(14, 8, total)
	tall := listWindowSize(40, 8, total)

	if short >= tall {
		t.Errorf("a body of 14 rows gives a window of %d and one of 40 gives %d, so the window is not following the terminal", short, tall)
	}
	if tall != total {
		t.Errorf("a body of 40 rows shows %d of %d rows, want all of them", tall, total)
	}
	if short < 1 {
		t.Errorf("a body of 14 rows with 8 rows of chrome gives a window of %d rows", short)
	}
	if got := listWindowSize(1, 8, total); got != listWindowMin {
		t.Errorf("a body too short for any list gives a window of %d, want the floor of %d", got, listWindowMin)
	}
	if got := listWindowSize(100, 8, total); got != total {
		t.Errorf("a tall terminal gives a window of %d, want the plan's %d rows", got, total)
	}
}

// TestTheScreenFollowsTheCursorAndReportsWhatItHides is the same rule through the
// rendered screen rather than the arithmetic: the row the cursor is on is drawn, the
// rows that were left behind are not, and their number is on screen.
//
// The terminal is named rather than left to the model's zero value: a screen sized by
// its terminal draws nothing useful at 0x0, and a guard that renders nothing would
// pass while the screen was broken.
func TestTheScreenFollowsTheCursorAndReportsWhatItHides(t *testing.T) {
	m := planFixtures()[0].fixtureModel()
	m.Screen = ScreenInstallReview
	// Wide enough for the rows whole, and still single-column, so the check is of
	// the plan's own text rather than a cut of it.
	m.Width, m.Height = 100, 24

	rows := m.planReviewRows()
	last := -1
	for i, row := range rows {
		if row.Switchable {
			last = i
		}
	}
	if last < 0 {
		t.Fatal("the plan holds no switchable row, so this guard tests nothing")
	}
	m.UnitCursor = last
	rendered := m.renderInstallReview()

	if !strings.Contains(rendered, rows[last].Text) {
		t.Errorf("the row the cursor is on is not drawn")
	}
	if !strings.Contains(rendered, "more above") {
		t.Errorf("the screen does not say that rows are hidden above the window:\n%s", rendered)
	}
	first := firstSwitchablePlanRow(rows)
	if strings.Contains(rendered, rows[first].Text) {
		t.Errorf("the first switchable row is still drawn while the cursor is %d rows below it:\n%s", last, rendered)
	}
}
