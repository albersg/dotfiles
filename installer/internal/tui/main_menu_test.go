package tui

import "testing"

// TestMainMenuTwoLineCardYieldsToItsNotices pins the correction the review
// required: the two-line card's height check used to ignore the trainer/update
// notices above it, so at the first height the card fit a notice could take the
// rows the footer needed. The notices now count against the same budget.
func TestMainMenuTwoLineCardYieldsToItsNotices(t *testing.T) {
	m := Model{Screen: ScreenMainMenu, Width: 200, Height: 55}
	l := layoutFor(m)
	spare := m.companionHeightNow()
	if !l.TwoColumn {
		spare += len(m.rotatorLines(m.panelsFor(), l.Inner))
	}
	budget := m.mainMenuRowsNeeded(true) + spare
	if !m.mainMenuTwoLine(budget) {
		t.Fatal("the two-line card does not fit the budget measured for it")
	}
	m.TrainerMessage = "the trainer could not save your progress"
	if m.mainMenuTwoLine(budget) {
		t.Error("a notice still lets the two-line card take the rows it needs")
	}
}
