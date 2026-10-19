package tui

import (
	"strings"
	"testing"
)

// TestEveryUtilitiesRowOpensAScreen walks every row the Utilities section
// offers and presses Enter on it. A row that is drawn but has no branch in the
// section's selection handler is a dead row: the user presses Enter, nothing
// happens, and nothing distinguishes that from a key that was never delivered.
//
// #372 was exactly that. utilitiesWSLAdvisorRow was appended by the section and
// rendered with its panel line, but the selection switch — twelve branches, one
// per other row — had no case for it, so Enter on it did nothing. No test caught
// it because every test that pressed a Utilities row pressed one whose branch
// already existed; each of those tests is correct and none of them is a walk.
//
// This guard is deliberately written against the section's own row list rather
// than a hand-kept copy: a row added later without a branch fails here, and a
// branch added for a row the section no longer offers costs nothing.
func TestEveryUtilitiesRowOpensAScreen(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenUtilities

	rows := m.GetCurrentOptions()
	if len(rows) == 0 {
		t.Fatal("the Utilities section offers no rows, so this guard would pass by looking at nothing")
	}

	for index, row := range rows {
		// The divider a section uses to group its choices is offered in this list
		// and is not a choice: it has no branch, and it must not have one. The
		// views detect it by its dashes, which is the same test the section's own
		// handler uses to ignore it.
		if strings.HasPrefix(row, menuSeparatorPrefix) {
			continue
		}

		m.Cursor = index
		next, _ := m.handleUtilitiesKeys("enter")
		landed := next.(Model)
		if landed.Screen == ScreenUtilities {
			t.Errorf("the row %q has no branch in the section's handler: pressing Enter left the section on its own screen", row)
		}
	}
}
