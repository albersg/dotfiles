package tui

import (
	"strings"
	"testing"
)

// unitIndex is where a unit sits in the catalogue, which is also the row the
// screen draws it on and the value the cursor holds.
func unitIndex(id string) int {
	for i, unit := range installUnits {
		if unit.ID == id {
			return i
		}
	}
	return -1
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

// TestTheScreenListsEveryUnitTurnedOn pins S5's first half and S4 with it: every
// catalogued unit has a row, and with nothing touched every row is on, because the
// default selection is the empty disabled set.
func TestTheScreenListsEveryUnitTurnedOn(t *testing.T) {
	m := Model{Screen: ScreenInstallReview}
	rows := m.GetCurrentOptions()

	if len(rows) != len(installUnits) {
		t.Fatalf("the screen draws %d rows for %d catalogued units", len(rows), len(installUnits))
	}
	for i, unit := range installUnits {
		if !strings.Contains(rows[i], installReviewLabel(unit.ID)) {
			t.Errorf("row %d does not name %q: %q", i, unit.ID, rows[i])
		}
		if !strings.HasPrefix(rows[i], "[x] ") {
			t.Errorf("row %d is not shown as installed with nothing touched: %q", i, rows[i])
		}
	}

	kept, off := m.installReviewCounts()
	if kept != len(installUnits) || off != 0 {
		t.Errorf("an untouched selection counts %d on and %d off, want %d and 0", kept, off, len(installUnits))
	}
}

// TestTheThreeUnitsTheRunNeedsCannotBeTurnedOff pins S5's second half. Both halves
// of it matter: the key must not change the selection, and the row has to say why,
// because a row that simply refuses to change reads as a broken key.
func TestTheThreeUnitsTheRunNeedsCannotBeTurnedOff(t *testing.T) {
	for _, unit := range installUnits {
		if !unit.Fixed {
			continue
		}
		m := Model{Screen: ScreenInstallReview, UnitCursor: unitIndex(unit.ID)}
		if m.toggleUnitAtCursor() {
			t.Errorf("toggling %q changed the selection, but the run needs it", unit.ID)
		}
		if m.unitIsDisabled(unit.ID) {
			t.Errorf("%q ended up turned off", unit.ID)
		}

		row := m.installReviewRow(unit)
		if !strings.Contains(row, "the run needs it") {
			t.Errorf("the row for %q does not say the run needs it: %q", unit.ID, row)
		}
		if !strings.HasPrefix(row, "[x] ") {
			t.Errorf("the row for %q does not read as installed: %q", unit.ID, row)
		}
	}
}

// TestTurningAUnitOffAndOnAgainLeavesTheSelectionAsItWas is the round trip: the key
// works on an ordinary unit, the row follows it, and the count with it.
func TestTurningAUnitOffAndOnAgainLeavesTheSelectionAsItWas(t *testing.T) {
	m := Model{Screen: ScreenInstallReview, UnitCursor: unitIndex("toolset")}

	if !m.toggleUnitAtCursor() {
		t.Fatalf("toggling toolset reported no change")
	}
	if !m.unitIsDisabled("toolset") {
		t.Fatalf("toolset is not turned off after the toggle")
	}
	if row := m.installReviewRow(installUnits[m.UnitCursor]); !strings.HasPrefix(row, "[ ] ") {
		t.Errorf("the row does not follow the switch: %q", row)
	}
	kept, off := m.installReviewCounts()
	if off != 1 || kept != len(installUnits)-1 {
		t.Errorf("counts are %d on and %d off, want %d and 1", kept, off, len(installUnits)-1)
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
	m := Model{Screen: ScreenInstallReview, UnitCursor: unitIndex("nvim"), DisabledUnits: shared}

	m.toggleUnitAtCursor()
	if len(shared) != 1 || shared[0] != "toolset" {
		t.Fatalf("turning a unit off wrote through the slice it was given: %v", shared)
	}

	back := Model{Screen: ScreenInstallReview, UnitCursor: unitIndex("toolset"), DisabledUnits: shared}
	back.toggleUnitAtCursor()
	if len(shared) != 1 || shared[0] != "toolset" {
		t.Fatalf("turning a unit back on wrote through the slice it was given: %v", shared)
	}
}

// TestTheCursorWalksTheListAndWraps keeps the screen navigable at both ends rather
// than stuck on the first row, which is what a clamp looks like from the keyboard.
func TestTheCursorWalksTheListAndWraps(t *testing.T) {
	m := Model{Screen: ScreenInstallReview}
	m.moveUnitCursor(-1)
	if m.UnitCursor != len(installUnits)-1 {
		t.Errorf("moving up from the first row landed on %d, want the last row", m.UnitCursor)
	}
	m.moveUnitCursor(1)
	if m.UnitCursor != 0 {
		t.Errorf("moving down from the last row landed on %d, want the first row", m.UnitCursor)
	}
	m.moveUnitCursor(1)
	if m.UnitCursor != 1 {
		t.Errorf("moving down landed on %d, want 1", m.UnitCursor)
	}
}

// TestEnterStartsTheRunFromTheScreen is what keeps the screen from being
// decorative: Enter records the plan from the choices and starts the run, so the
// units the user just reviewed are the units the plan holds.
func TestEnterStartsTheRunFromTheScreen(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenInstallReview
	m.Choices = UserChoices{OS: "linux", Shell: "zsh", Terminal: "none", WindowMgr: "none"}

	next, cmd := m.handleInstallReviewKeys("enter")
	started, ok := next.(Model)
	if !ok {
		t.Fatalf("Enter returned %T, want a Model", next)
	}
	if started.Screen != ScreenInstalling {
		t.Errorf("Enter left the screen on %v, want the installing screen", started.Screen)
	}
	if len(started.Steps) == 0 {
		t.Errorf("the run started with no plan")
	}
	if cmd == nil {
		t.Errorf("Enter did not start the run: no command came back")
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
	for _, hint := range []installerHint{hintToggle, hintStart, hintBack} {
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
	total := len(installUnits)
	if total <= window {
		t.Fatalf("the catalogue holds %d units and the guard's window shows %d, so this guard would test nothing", total, window)
	}

	for cursor := 0; cursor < total; cursor++ {
		start, end := reviewWindowRows(total, cursor, window)
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

	if start, _ := reviewWindowRows(total, 0, window); start != 0 {
		t.Errorf("on the first row the window starts at %d, want 0", start)
	}
	if _, end := reviewWindowRows(total, total-1, window); end != total {
		t.Errorf("on the last row the window ends at %d, want %d", end, total)
	}
}

// TestTheWindowIsSizedByTheTerminalAndNotByANumber is the reason the screen takes a
// fixed number of rows from nowhere: a terminal that leaves less room has to show a
// shorter list, and the one that leaves more has to show all of it. The 60x20 case of
// the matrix is the size that made a fixed number wrong.
func TestTheWindowIsSizedByTheTerminalAndNotByANumber(t *testing.T) {
	total := len(installUnits)

	short := installReviewWindowSize(14, 8, total)
	tall := installReviewWindowSize(25, 8, total)

	if short >= tall {
		t.Errorf("a body of 14 rows gives a window of %d and one of 25 gives %d, so the window is not following the terminal", short, tall)
	}
	if tall != total {
		t.Errorf("a body of 25 rows shows %d of %d units, want all of them", tall, total)
	}
	if short < 1 {
		t.Errorf("a body of 14 rows with 8 rows of chrome gives a window of %d rows", short)
	}

	if got := installReviewWindowSize(1, 8, total); got != installReviewWindowMin {
		t.Errorf("a body too short for any list gives a window of %d, want the floor of %d", got, installReviewWindowMin)
	}
	if got := installReviewWindowSize(100, 8, total); got != total {
		t.Errorf("a tall terminal gives a window of %d, want the catalogue's %d", got, total)
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
	last := len(installUnits) - 1
	m := Model{Screen: ScreenInstallReview, Width: 80, Height: 24, UnitCursor: last}
	rendered := m.renderInstallReview()

	if !strings.Contains(rendered, installReviewLabel(installUnits[last].ID)) {
		t.Errorf("the unit the cursor is on is not drawn")
	}
	if !strings.Contains(rendered, "more above") {
		t.Errorf("the screen does not say that rows are hidden above the window:\n%s", rendered)
	}
	if strings.Contains(rendered, installReviewLabel(installUnits[0].ID)) {
		t.Errorf("the first unit is still drawn while the cursor is %d rows below it:\n%s", last, rendered)
	}
}
