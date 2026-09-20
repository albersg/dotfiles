package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestHerdrKeymapsDataIsPresent verifies the Herdr reference has real content
// and keeps the mouse-first surface that Herdr's own documentation leads with.
func TestHerdrKeymapsDataIsPresent(t *testing.T) {
	categories := GetHerdrKeymaps()

	if len(categories) == 0 {
		t.Fatal("GetHerdrKeymaps returned no categories")
	}

	if categories[0].Name != "Mouse & Pointer (Primary)" {
		t.Errorf("first Herdr category should present the mouse-first surface, got %q", categories[0].Name)
	}

	for _, cat := range categories {
		if cat.Name == "" {
			t.Error("Herdr category with empty name")
		}
		if cat.Description == "" {
			t.Errorf("Herdr category %q has no description", cat.Name)
		}
		if len(cat.Keymaps) == 0 {
			t.Errorf("Herdr category %q has no keymaps", cat.Name)
		}
		for _, km := range cat.Keymaps {
			if km.Keys == "" {
				t.Errorf("Herdr category %q has a keymap with empty keys", cat.Name)
			}
			if km.Description == "" {
				t.Errorf("Herdr keymap %q in %q has no description", km.Keys, cat.Name)
			}
		}
	}
}

// TestHerdrAppearsInKeymapsMenu verifies the Keymaps Reference offers Herdr.
func TestHerdrAppearsInKeymapsMenu(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenKeymapsMenu

	found := false
	for _, opt := range m.GetCurrentOptions() {
		if opt == "Herdr" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Herdr missing from the keymaps menu options: %v", m.GetCurrentOptions())
	}
}

// TestHerdrKeymapsNavigation verifies entering and leaving the Herdr screen the
// same way the other tools do.
func TestHerdrKeymapsNavigation(t *testing.T) {
	t.Run("menu selection opens Herdr keymaps", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenKeymapsMenu
		m.Cursor = 3 // Herdr, after Neovim, Tmux and Zellij

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = result.(Model)

		if m.Screen != ScreenKeymapsHerdr {
			t.Fatalf("Expected ScreenKeymapsHerdr, got %v", m.Screen)
		}
	})

	t.Run("selecting a category opens the Herdr category", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenKeymapsHerdr
		m.Cursor = 0

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = result.(Model)

		if m.Screen != ScreenKeymapsHerdrCat {
			t.Fatalf("Expected ScreenKeymapsHerdrCat, got %v", m.Screen)
		}
	})

	t.Run("esc from a category returns to the Herdr menu", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenKeymapsHerdrCat
		m.HerdrSelectedCategory = 0

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
		m = result.(Model)

		if m.Screen != ScreenKeymapsHerdr {
			t.Fatalf("Expected ScreenKeymapsHerdr, got %v", m.Screen)
		}
	})

	t.Run("esc from Herdr keymaps returns to the tool menu", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenKeymapsHerdr
		m.PrevScreen = ScreenMainMenu

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
		m = result.(Model)

		if m.Screen != ScreenKeymapsMenu {
			t.Fatalf("Expected ScreenKeymapsMenu, got %v", m.Screen)
		}
	})

	t.Run("back entry returns to the tool menu", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenKeymapsHerdr
		// "← Back" is the last option, after the categories and the separator.
		m.Cursor = len(m.GetCurrentOptions()) - 1

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = result.(Model)

		if m.Screen != ScreenKeymapsMenu {
			t.Fatalf("Expected ScreenKeymapsMenu, got %v", m.Screen)
		}
	})
}

// TestHerdrKeymapsRender verifies both Herdr screens render without a terminal.
func TestHerdrKeymapsRender(t *testing.T) {
	m := NewModel()
	m.Width = 80
	m.Height = 24

	m.Screen = ScreenKeymapsHerdr
	menu := m.View()
	if !strings.Contains(menu, "Herdr") {
		t.Errorf("Herdr keymaps menu should mention Herdr, got:\n%s", menu)
	}
	if !strings.Contains(menu, "mouse-first") {
		t.Errorf("Herdr keymaps menu should state the mouse-first emphasis, got:\n%s", menu)
	}
	if !strings.Contains(menu, GetHerdrKeymaps()[0].Name) {
		t.Errorf("Herdr keymaps menu should list the mouse category, got:\n%s", menu)
	}

	m.Screen = ScreenKeymapsHerdrCat
	m.HerdrSelectedCategory = 0
	cat := m.View()
	if !strings.Contains(cat, "Click") {
		t.Errorf("Herdr mouse category should render its bindings, got:\n%s", cat)
	}
}
