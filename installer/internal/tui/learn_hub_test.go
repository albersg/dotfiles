package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// The Learn About Tools hub exists for one reason: before it, the main menu's
// own row opened the terminal list directly, and the shell, multiplexer and
// Neovim pages could only be reached from inside the installation wizard. A
// reader who had not started a run could not read them at all, while
// docs/tui-installer.md promised "terminals, shells, and multiplexers".
//
// These guards hold the hub to that job: its rows and its menu are one table, a
// row opens the page it names and Escape comes back, and every Learn screen the
// model declares is reachable from it.

// TestLearnHubRowsAndOptionsAgree keeps the row table and the menu from drifting:
// a page added to the table must appear in the menu, and the menu may offer
// nothing the table does not know how to open.
func TestLearnHubRowsAndOptionsAgree(t *testing.T) {
	rows := learnToolRows()
	if len(rows) == 0 {
		t.Fatal("the hub declares no row, so this guard proves nothing")
	}

	m := NewModel()
	m.Screen = ScreenLearnTools
	options := m.GetCurrentOptions()
	if len(options) == 0 {
		t.Fatal("the hub offers no option, so the menu cannot be walked")
	}

	for _, row := range rows {
		found := false
		for _, option := range options {
			if option == row.Label {
				found = true
			}
		}
		if !found {
			t.Errorf("the hub table carries the row %q and the menu does not offer it", row.Label)
		}
	}

	for _, option := range options {
		if strings.Contains(option, "Back") || strings.HasPrefix(option, m.menuSeparator()) {
			continue
		}
		if _, ok := learnToolRowByLabel(option); !ok {
			t.Errorf("the hub offers %q and no row in the table opens it", option)
		}
	}
}

// TestEveryLearnHubRowOpensItsPageAndComesBack is the behavioral half: Enter on
// a row lands on the screen the row names, and Escape returns to the hub rather
// than dumping the reader on the main menu.
//
// The row is found by its label in the menu the hub draws, not by its index in
// the table. The menu carries a rule where the hub's two sections meet, so a
// row's index in the table and its position in the menu are two different
// numbers from the first row of the second section on. The guard walked the
// table by index until that section existed, and then the index led to the row
// under the one it meant to press.
func TestEveryLearnHubRowOpensItsPageAndComesBack(t *testing.T) {
	for _, row := range learnToolRows() {
		m := NewModel()
		m.Screen = ScreenLearnTools
		m.PrevScreen = ScreenMainMenu

		index := -1
		for i, option := range m.GetCurrentOptions() {
			if option == row.Label {
				index = i
			}
		}
		if index < 0 {
			t.Errorf("the hub table carries the row %q and the menu does not offer it", row.Label)
			continue
		}
		m.Cursor = index

		opened, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		page := opened.(Model)
		if page.Screen != row.Screen {
			t.Errorf("the row %q opens %v, want %v", row.Label, page.Screen, row.Screen)
			continue
		}
		if page.PrevScreen != ScreenLearnTools {
			t.Errorf("the row %q left PrevScreen at %v, so Escape cannot come back to the hub",
				row.Label, page.PrevScreen)
		}

		back, _ := page.Update(tea.KeyMsg{Type: tea.KeyEscape})
		if got := back.(Model).Screen; got != ScreenLearnTools {
			t.Errorf("Escape from the page the row %q opened goes to %v, want the hub", row.Label, got)
		}
	}
}

// TestEscapeFromTheHubReturnsToTheMainMenu pins the way out of the hub itself,
// which is the row the reader came in from.
func TestEscapeFromTheHubReturnsToTheMainMenu(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenLearnTools
	m.PrevScreen = ScreenMainMenu

	back, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if got := back.(Model).Screen; got != ScreenMainMenu {
		t.Errorf("Escape from the hub goes to %v, want the main menu", got)
	}
}

// TestTheLearnHubReachesEveryLearnScreen derives the Learn screens from model.go
// and walks the hub to each of them, so a page added later that no row opens
// fails by name instead of shipping unreachable.
func TestTheLearnHubReachesEveryLearnScreen(t *testing.T) {
	declared := screenConstantsInDeclarationOrder(t)
	if len(declared) == 0 {
		t.Fatal("model.go declares no Screen constant, so this guard proves nothing")
	}

	reachable := screensReachedFromTheLearnHub(t)

	var missing []string
	for screen, name := range declared {
		if !strings.HasPrefix(name, "ScreenLearn") || Screen(screen) == ScreenLearnTools {
			continue
		}
		if !reachable[Screen(screen)] {
			missing = append(missing, name)
		}
	}
	if len(missing) == 0 {
		return
	}
	t.Errorf("pressing Enter through the Learn About Tools hub reaches no page for %v\n"+
		"A Learn screen the hub does not open is a screen the main menu cannot show either: "+
		"add its row to learnToolRows().", missing)
}

// screensReachedFromTheLearnHub walks the hub by pressing Enter on every row of
// every screen it reaches, and returns the screens it can land on.
func screensReachedFromTheLearnHub(t *testing.T) map[Screen]bool {
	t.Helper()

	reachable := map[Screen]bool{ScreenLearnTools: true}
	queue := []Model{}

	start := NewModel()
	start.Screen = ScreenLearnTools
	start.PrevScreen = ScreenMainMenu
	queue = append(queue, start)

	// The subgraph is small; the bound only keeps a cycle from hanging the guard.
	for steps := 0; len(queue) > 0 && steps < 200; steps++ {
		current := queue[0]
		queue = queue[1:]

		options := current.GetCurrentOptions()
		for i, option := range options {
			if strings.Contains(option, "Back") || strings.HasPrefix(option, current.menuSeparator()) {
				continue
			}
			step := current
			step.Cursor = i
			opened, _ := step.Update(tea.KeyMsg{Type: tea.KeyEnter})
			page := opened.(Model)
			if reachable[page.Screen] {
				continue
			}
			reachable[page.Screen] = true
			queue = append(queue, page)
		}
	}
	return reachable
}
