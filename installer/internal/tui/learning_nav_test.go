package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestLearnHubEscAndBackReturnToTheMainMenu pins the escape from the Learn About
// Tools hub. Opening a tool page sets PrevScreen to the hub, so returning to the
// hub left PrevScreen pointing at the hub itself: both the advertised Esc and the
// manual Back row re-entered the hub and the screen could not be left.
func TestLearnHubEscAndBackReturnToTheMainMenu(t *testing.T) {
	m := Model{Screen: ScreenLearnTools, PrevScreen: ScreenLearnTools}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if got := updated.(Model); got.Screen != ScreenMainMenu {
		t.Errorf("Esc on the learn hub went to %v, want the main menu", got.Screen)
	}

	m = Model{Screen: ScreenLearnTools, PrevScreen: ScreenLearnTools}
	opts := m.GetCurrentOptions()
	if len(opts) == 0 {
		t.Fatal("the learn hub has no rows")
	}
	m.Cursor = len(opts) - 1
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got := updated.(Model); got.Screen != ScreenMainMenu {
		t.Errorf("the Back row went to %v, want the main menu", got.Screen)
	}
}
