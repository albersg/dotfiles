package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/albersg/dotfiles/installer/internal/system"
	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// MODEL TESTS - Comprehensive state management testing
// =============================================================================

func TestNewModelInitialization(t *testing.T) {
	m := NewModel()

	t.Run("should start at welcome screen", func(t *testing.T) {
		if m.Screen != ScreenWelcome {
			t.Errorf("Expected ScreenWelcome, got %v", m.Screen)
		}
	})

	t.Run("should have default dimensions", func(t *testing.T) {
		if m.Width != 80 || m.Height != 24 {
			t.Errorf("Expected 80x24, got %dx%d", m.Width, m.Height)
		}
	})

	t.Run("should have system info", func(t *testing.T) {
		if m.SystemInfo == nil {
			t.Error("SystemInfo should not be nil")
		}
	})

	t.Run("should have empty choices", func(t *testing.T) {
		if m.Choices.OS != "" || m.Choices.Terminal != "" || m.Choices.Shell != "" {
			t.Error("Choices should be empty initially")
		}
	})

	t.Run("should have keymap categories", func(t *testing.T) {
		if len(m.KeymapCategories) == 0 {
			t.Error("KeymapCategories should not be empty")
		}
	})

	t.Run("should have lazyvim topics", func(t *testing.T) {
		if len(m.LazyVimTopics) == 0 {
			t.Error("LazyVimTopics should not be empty")
		}
	})

	t.Run("should have empty backup data", func(t *testing.T) {
		if len(m.ExistingConfigs) != 0 || len(m.AvailableBackups) != 0 {
			t.Error("Backup data should be empty initially")
		}
	})

	t.Run("cursor should be at zero", func(t *testing.T) {
		if m.Cursor != 0 {
			t.Errorf("Cursor should be 0, got %d", m.Cursor)
		}
	})

	t.Run("should not be quitting", func(t *testing.T) {
		if m.Quitting {
			t.Error("Should not be quitting initially")
		}
	})
}

func TestGetCurrentOptionsForAllScreens(t *testing.T) {
	screens := []struct {
		name   Screen
		minLen int
	}{
		{ScreenMainMenu, 4},
		{ScreenOSSelect, 2},
		{ScreenTerminalSelect, 5},
		{ScreenFontSelect, 2},
		{ScreenShellSelect, 4},
		{ScreenWMSelect, 4},
		{ScreenNvimSelect, 5},
		// The setup screen lists one row per decision, its values on the same line:
		// seven always, and the backup row too when the machine holds configs.
		{ScreenSetup, 7},
		{ScreenBackupConfirm, 3},
		{ScreenRestoreConfirm, 3},
		{ScreenLearnTerminals, 5},
		{ScreenLearnShells, 4},
		{ScreenLearnWM, 3},
		{ScreenLearnNvim, 4},
		{ScreenLearnTools, 5},
		{ScreenLearnPrompts, 4},
		// The reference menu offers every tool it has a page for: the eight rows plus
		// the separator and Back.
		{ScreenKeymapsMenu, 10},
		{ScreenKeymapsKitty, 4},
		{ScreenKeymaps, 3},
		{ScreenLearnLazyVim, 3},
		// The review screen lists the run's plan grouped by area, so its count is
		// the plan's own: a negative minimum asks the loop to derive it from the
		// model rather than writing a number here that the catalogue could move.
		{ScreenInstallReview, -1},
	}

	for _, tc := range screens {
		t.Run(tc.name.String(), func(t *testing.T) {
			m := NewModel()
			m.Screen = tc.name
			m.Choices.OS = "mac" // For terminal options

			minLen := tc.minLen
			if minLen < 0 {
				minLen = len(planFor(m.planOptions()))
			}
			opts := m.GetCurrentOptions()
			if len(opts) < minLen {
				t.Errorf("Expected at least %d options, got %d: %v", minLen, len(opts), opts)
			}
		})
	}
}

func TestTerminalOptionsPerOS(t *testing.T) {
	t.Run("mac should have Kitty option", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenTerminalSelect
		m.Choices.OS = "mac"

		opts := m.GetCurrentOptions()
		hasKitty := false
		for _, opt := range opts {
			if strings.Contains(opt, "Kitty") {
				hasKitty = true
				break
			}
		}
		if !hasKitty {
			t.Error("Mac should have Kitty option")
		}
	})

	t.Run("linux should not have Kitty option", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenTerminalSelect
		m.Choices.OS = "linux"

		opts := m.GetCurrentOptions()
		hasKitty := false
		for _, opt := range opts {
			if strings.Contains(opt, "Kitty") {
				hasKitty = true
				break
			}
		}
		if hasKitty {
			t.Error("Linux should not have Kitty option")
		}
	})
}

func TestMainMenuOptionsWithBackups(t *testing.T) {
	t.Run("without backups should not show restore", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenMainMenu
		m.AvailableBackups = []system.BackupInfo{}

		opts := m.GetCurrentOptions()
		for _, opt := range opts {
			if strings.Contains(opt, "Restore") {
				t.Error("Should not show Restore option without backups")
			}
		}
	})

	t.Run("with backups should show restore", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenMainMenu
		m.AvailableBackups = []system.BackupInfo{
			{Path: "/test", Timestamp: time.Now(), Files: []string{"test"}},
		}

		opts := m.GetCurrentOptions()
		hasRestore := false
		for _, opt := range opts {
			if strings.Contains(opt, "Restore") {
				hasRestore = true
				break
			}
		}
		if !hasRestore {
			t.Error("Should show Restore option with backups")
		}
	})
}

func TestRestoreBackupOptions(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenRestoreBackup
	m.AvailableBackups = []system.BackupInfo{
		{Path: "/backup1", Timestamp: time.Now(), Files: []string{"a", "b"}},
		{Path: "/backup2", Timestamp: time.Now().Add(-time.Hour), Files: []string{"c"}},
	}

	opts := m.GetCurrentOptions()

	t.Run("should list all backups", func(t *testing.T) {
		// 2 backups + separator + back = 4
		if len(opts) != 4 {
			t.Errorf("Expected 4 options, got %d: %v", len(opts), opts)
		}
	})

	t.Run("should include item count in label", func(t *testing.T) {
		if !strings.Contains(opts[0], "2 items") {
			t.Errorf("First backup should show 2 items: %s", opts[0])
		}
		if !strings.Contains(opts[1], "1 items") {
			t.Errorf("Second backup should show 1 items: %s", opts[1])
		}
	})
}

func TestGetScreenTitleForAllScreens(t *testing.T) {
	allScreens := []Screen{
		ScreenWelcome, ScreenMainMenu, ScreenOSSelect, ScreenTerminalSelect,
		ScreenFontSelect, ScreenShellSelect, ScreenWMSelect, ScreenNvimSelect,
		ScreenSetup, ScreenInstallReview,
		ScreenBackupConfirm, ScreenRestoreBackup, ScreenRestoreConfirm,
		ScreenInstalling, ScreenComplete, ScreenError,
		ScreenLearnTerminals, ScreenLearnShells, ScreenLearnWM, ScreenLearnNvim,
		ScreenLearnTools, ScreenLearnFont, ScreenLearnPrompts,
		ScreenLearnTooling, ScreenLearnPlugins, ScreenLearnToolset,
		ScreenKeymaps, ScreenKeymapCategory, ScreenKeymapsMenu,
		ScreenKeymapsKitty, ScreenKeymapsKittyCat,
		ScreenKeymapsWezTerm, ScreenKeymapsAlacritty,
		ScreenKeymaps, ScreenKeymapCategory, ScreenLearnLazyVim, ScreenLazyVimTopic,
	}

	for _, screen := range allScreens {
		t.Run(screen.String(), func(t *testing.T) {
			m := NewModel()
			m.Screen = screen
			title := m.GetScreenTitle()
			if title == "" && screen != ScreenInstalling && screen != ScreenComplete && screen != ScreenError {
				// Some screens might have dynamic titles
				if screen == ScreenKeymapCategory || screen == ScreenLazyVimTopic {
					// These need selected category/topic
					return
				}
			}
		})
	}
}

func TestGetScreenDescription(t *testing.T) {
	t.Run("OSSelect shows detected OS", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenOSSelect
		desc := m.GetScreenDescription()
		if !strings.Contains(desc, "Detected") {
			t.Errorf("Expected 'Detected' in description, got: %s", desc)
		}
	})

	t.Run("ShellSelect shows current shell", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenShellSelect
		desc := m.GetScreenDescription()
		if !strings.Contains(desc, "shell") {
			t.Errorf("Expected 'shell' in description, got: %s", desc)
		}
	})
}

// =============================================================================
// SETUP INSTALL STEPS TESTS - Critical for installation
// =============================================================================

func TestSetupInstallStepsMinimal(t *testing.T) {
	m := NewModel()
	m.SystemInfo = &system.SystemInfo{HasBrew: true, HasXcode: true}
	m.Choices = UserChoices{
		OS:          "mac",
		Terminal:    "none",
		InstallFont: false,
		Shell:       "fish",
		WindowMgr:   "none",
		InstallNvim: false,
	}
	m.SetupInstallSteps()

	// setshell step runs interactively to change the default shell with chsh
	expectedSteps := []string{"clone", "shell", "toolset", "agentskills", "officecli", "theme", "setshell", "cleanup"}
	if len(m.Steps) != len(expectedSteps) {
		t.Errorf("Expected %d steps, got %d", len(expectedSteps), len(m.Steps))
		for _, s := range m.Steps {
			t.Logf("  Step: %s", s.ID)
		}
	}

	for i, expected := range expectedSteps {
		if i < len(m.Steps) && m.Steps[i].ID != expected {
			t.Errorf("Step %d: expected %s, got %s", i, expected, m.Steps[i].ID)
		}
	}
}

func TestSetupInstallStepsIncludesBackupWhenEnabled(t *testing.T) {
	m := NewModel()
	m.SystemInfo = &system.SystemInfo{HasBrew: true, HasXcode: true}
	m.Choices = UserChoices{
		OS:           "mac",
		Terminal:     "none",
		InstallFont:  false,
		Shell:        "fish",
		WindowMgr:    "none",
		InstallNvim:  false,
		CreateBackup: true,
	}
	m.ExistingConfigs = []string{"nvim: ~/.config/nvim"}
	m.SetupInstallSteps()

	if m.Steps[0].ID != "backup" {
		t.Errorf("First step should be backup, got %s", m.Steps[0].ID)
	}
}

func TestSetupInstallStepsNoBackupWhenNoConfigs(t *testing.T) {
	m := NewModel()
	m.SystemInfo = &system.SystemInfo{HasBrew: true, HasXcode: true}
	m.Choices = UserChoices{
		OS:           "mac",
		Terminal:     "none",
		InstallFont:  false,
		Shell:        "fish",
		WindowMgr:    "none",
		InstallNvim:  false,
		CreateBackup: true, // User wants backup but no existing configs
	}
	m.ExistingConfigs = []string{} // Empty!
	m.SetupInstallSteps()

	if m.Steps[0].ID == "backup" {
		t.Error("Should not include backup step when no existing configs")
	}
}

func TestSetupInstallStepsWithHomebrew(t *testing.T) {
	m := NewModel()
	m.SystemInfo = &system.SystemInfo{HasBrew: false, HasXcode: true}
	m.Choices = UserChoices{OS: "mac", Terminal: "none", Shell: "fish", WindowMgr: "none"}
	m.SetupInstallSteps()

	hasHomebrew := false
	for _, step := range m.Steps {
		if step.ID == "homebrew" {
			hasHomebrew = true
			break
		}
	}
	if !hasHomebrew {
		t.Error("Should include homebrew step when not installed")
	}
}

func TestSetupInstallStepsWithXcode(t *testing.T) {
	m := NewModel()
	m.SystemInfo = &system.SystemInfo{HasBrew: true, HasXcode: false}
	m.Choices = UserChoices{OS: "mac", Terminal: "none", Shell: "fish", WindowMgr: "none"}
	m.SetupInstallSteps()

	hasXcode := false
	for _, step := range m.Steps {
		if step.ID == "xcode" {
			hasXcode = true
			break
		}
	}
	if !hasXcode {
		t.Error("Should include xcode step when not installed on mac")
	}
}

func TestSetupInstallStepsLinuxDeps(t *testing.T) {
	m := NewModel()
	m.SystemInfo = &system.SystemInfo{HasBrew: true}
	m.Choices = UserChoices{OS: "linux", Terminal: "none", Shell: "fish", WindowMgr: "none"}
	m.SetupInstallSteps()

	hasDeps := false
	for _, step := range m.Steps {
		if step.ID == "deps" {
			hasDeps = true
			break
		}
	}
	if !hasDeps {
		t.Error("Should include deps step on linux")
	}
}

func TestSetupInstallStepsFullInstall(t *testing.T) {
	m := NewModel()
	m.SystemInfo = &system.SystemInfo{HasBrew: false, HasXcode: false}
	m.Choices = UserChoices{
		OS:           "mac",
		Terminal:     "ghostty",
		InstallFont:  true,
		Shell:        "fish",
		WindowMgr:    "tmux",
		InstallNvim:  true,
		CreateBackup: true,
	}
	m.ExistingConfigs = []string{"nvim"}
	m.SetupInstallSteps()

	// xcode runs before clone so git is available on a fresh macOS install.
	// setshell step runs interactively to change the default shell with chsh
	// theme runs after every step that writes a configuration file, and before
	// cleanup, which removes the checkout the theme definitions are read from
	expectedIDs := []string{"backup", "xcode", "clone", "homebrew", "terminal", "font", "shell", "wm", "nvim", "toolset", "agentskills", "officecli", "theme", "setshell", "cleanup"}
	if len(m.Steps) != len(expectedIDs) {
		t.Errorf("Expected %d steps, got %d", len(expectedIDs), len(m.Steps))
	}

	for i, expected := range expectedIDs {
		if i < len(m.Steps) && m.Steps[i].ID != expected {
			t.Errorf("Step %d: expected %s, got %s", i, expected, m.Steps[i].ID)
		}
	}
}

// =============================================================================
// UPDATE/KEY HANDLER TESTS - Comprehensive keyboard interaction testing
// =============================================================================

func TestCtrlCAlwaysQuits(t *testing.T) {
	screens := []Screen{
		ScreenWelcome, ScreenMainMenu, ScreenOSSelect, ScreenTerminalSelect,
		ScreenInstallReview,
		ScreenInstalling, ScreenComplete, ScreenError, ScreenKeymapCategory,
	}

	for _, screen := range screens {
		t.Run(screen.String(), func(t *testing.T) {
			m := NewModel()
			m.Screen = screen

			result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
			newModel := result.(Model)

			if !newModel.Quitting {
				t.Error("Ctrl+C should set Quitting to true")
			}
			if cmd == nil {
				t.Error("Ctrl+C should return quit command")
			}
		})
	}
}

func TestQKeyBehavior(t *testing.T) {
	t.Run("space+q quits from main menu", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenMainMenu

		// First press space to enter leader mode
		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
		newModel := result.(Model)

		if !newModel.LeaderMode {
			t.Error("space should activate leader mode")
		}

		// Then press q to quit
		result, _ = newModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
		newModel = result.(Model)

		if !newModel.Quitting {
			t.Error("space+q should quit from main menu")
		}
	})

	t.Run("space+q does not quit during installation", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenInstalling

		// First press space to enter leader mode
		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
		newModel := result.(Model)

		// Then press q
		result, _ = newModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
		newModel = result.(Model)

		if newModel.Quitting {
			t.Error("space+q should not quit during installation")
		}
	})

	t.Run("q alone quits on the screens whose footer offers it", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenMainMenu

		// The main menu's footer names [q] quit (issue #375): leaving is its own key
		// there, and the Exit row is still chosen with Enter.
		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
		newModel := result.(Model)

		if !newModel.Quitting {
			t.Error("q alone should quit on the main menu, whose footer offers [q] quit")
		}
	})

	t.Run("q alone does not quit where the screen does not offer it", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenInstalling

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
		newModel := result.(Model)

		if newModel.Quitting {
			t.Error("q alone should not quit during installation, which offers no quit")
		}
	})

	t.Run("q goes back from keymap category (via ESC behavior)", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenKeymapCategory

		// Use ESC to go back (q no longer works for this)
		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
		newModel := result.(Model)

		if newModel.Screen != ScreenKeymaps {
			t.Errorf("ESC should go back to ScreenKeymaps, got %v", newModel.Screen)
		}
	})

	t.Run("ESC goes back from lazyvim topic", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenLazyVimTopic

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
		newModel := result.(Model)

		if newModel.Screen != ScreenLearnLazyVim {
			t.Errorf("ESC should go back to ScreenLearnLazyVim, got %v", newModel.Screen)
		}
	})
}

func TestEscapeKeyBehavior(t *testing.T) {
	testCases := []struct {
		from       Screen
		prevScreen Screen
		expected   Screen
	}{
		{ScreenKeymapCategory, ScreenMainMenu, ScreenKeymaps},
		{ScreenLazyVimTopic, ScreenMainMenu, ScreenLearnLazyVim},
		{ScreenLearnTerminals, ScreenTerminalSelect, ScreenTerminalSelect},
		{ScreenLearnShells, ScreenShellSelect, ScreenShellSelect},
		// ScreenKeymaps now goes to ScreenKeymapsMenu (intermediate menu), not MainMenu
		{ScreenKeymaps, ScreenMainMenu, ScreenKeymapsMenu},
		// ScreenKeymapsMenu uses PrevScreen to go back to MainMenu
		{ScreenKeymapsMenu, ScreenMainMenu, ScreenMainMenu},
		{ScreenLearnLazyVim, ScreenMainMenu, ScreenMainMenu},
		// The hub is the main menu's row into the wiki, so leaving it goes back
		// there rather than to the page it replaced.
		{ScreenLearnTools, ScreenMainMenu, ScreenMainMenu},
	}

	for _, tc := range testCases {
		t.Run(tc.from.String()+"_to_"+tc.expected.String(), func(t *testing.T) {
			m := NewModel()
			m.Screen = tc.from
			m.PrevScreen = tc.prevScreen

			result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
			newModel := result.(Model)

			if newModel.Screen != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, newModel.Screen)
			}
		})
	}

	t.Run("escape from main menu does nothing", func(t *testing.T) {
		// This subtest used to pin the defect: Esc on the main menu quit the whole
		// application, the most destructive action on the one global key the footer
		// did not announce (it announces quit on [Space q], and the documentation
		// says "Go back"). The main menu is the root, so Esc has nowhere back to go.
		// TestEscapeOnTheMainMenuDoesNotQuit holds the repair and the quit keys
		// that still work.
		m := NewModel()
		m.Screen = ScreenMainMenu

		result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		newModel := result.(Model)

		if newModel.Quitting {
			t.Error("Escape from main menu should not quit: the documentation says Esc goes back")
		}
		if cmd != nil {
			t.Error("Escape from main menu returned a command, want none")
		}
		if newModel.Screen != ScreenMainMenu {
			t.Errorf("Escape from main menu moved to %d, want the main menu", int(newModel.Screen))
		}
	})
}

func TestNavigationKeys(t *testing.T) {
	// Test up/down/j/k navigation
	keys := []struct {
		key   tea.KeyMsg
		delta int
		name  string
	}{
		{tea.KeyMsg{Type: tea.KeyDown}, 1, "down"},
		{tea.KeyMsg{Type: tea.KeyUp}, -1, "up"},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, 1, "j"},
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}, -1, "k"},
	}

	for _, k := range keys {
		t.Run(k.name, func(t *testing.T) {
			m := NewModel()
			m.Screen = ScreenMainMenu
			m.Cursor = mainMenuRowIndex(t, m, "Learn About Tools")

			result, _ := m.Update(k.key)
			newModel := result.(Model)

			// The menu is grouped, so one step lands on the next selectable row and
			// steps over the rule between the groups.
			options := m.GetCurrentOptions()
			expected := m.Cursor + k.delta
			for expected > 0 && expected < len(options) && strings.HasPrefix(options[expected], menuSeparatorPrefix) {
				expected += k.delta
			}
			if newModel.Cursor != expected {
				t.Errorf("Expected cursor %d, got %d", expected, newModel.Cursor)
			}
		})
	}
}

func TestCursorBoundsMainMenu(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenMainMenu
	m.Cursor = 0

	// Try to go below 0
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	newModel := result.(Model)
	if newModel.Cursor != 0 {
		t.Error("Cursor should not go below 0")
	}

	// Go to last item
	opts := m.GetCurrentOptions()
	m.Cursor = len(opts) - 1

	// Try to go beyond last
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	newModel = result.(Model)
	if newModel.Cursor != len(opts)-1 {
		t.Error("Cursor should not exceed options length")
	}
}

func TestSeparatorSkipping(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenTerminalSelect
	m.Choices.OS = "mac"

	opts := m.GetCurrentOptions()

	// Find separator index
	separatorIdx := -1
	for i, opt := range opts {
		if strings.HasPrefix(opt, "───") {
			separatorIdx = i
			break
		}
	}

	if separatorIdx == -1 {
		t.Skip("No separator found in options")
	}

	// Position cursor just before separator
	m.Cursor = separatorIdx - 1

	// Move down - should skip separator
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	newModel := result.(Model)

	if newModel.Cursor == separatorIdx {
		t.Error("Cursor should skip separator when moving down")
	}
}

func TestEnterOnSeparatorDoesNothing(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenTerminalSelect
	m.Choices.OS = "mac"

	opts := m.GetCurrentOptions()

	// Find separator index
	for i, opt := range opts {
		if strings.HasPrefix(opt, "───") {
			m.Cursor = i
			break
		}
	}

	originalScreen := m.Screen
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	newModel := result.(Model)

	if newModel.Screen != originalScreen {
		t.Error("Enter on separator should not change screen")
	}
}

// =============================================================================
// SCREEN TRANSITION TESTS - Full flow testing
// =============================================================================

func TestWelcomeToMainMenu(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenWelcome

	// Starting the flow is a command on its own letter: Enter does nothing here.
	if result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter}); result.(Model).Screen == ScreenMainMenu {
		t.Error("Enter left the welcome screen; starting the flow runs on s")
	}

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	newModel := result.(Model)

	if newModel.Screen != ScreenMainMenu {
		t.Errorf("Expected ScreenMainMenu, got %v", newModel.Screen)
	}
}

func TestMainMenuToStartInstallation(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenMainMenu
	m.Cursor = 0 // Start Installation

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	newModel := result.(Model)

	if newModel.Screen != ScreenSetup {
		t.Errorf("Expected ScreenSetup, got %v", newModel.Screen)
	}
}

// TestMainMenuToLearnTools holds the main menu's own Learn row to the hub. The
// row used to open the terminal list directly, which left the shell, multiplexer
// and Neovim pages reachable only from inside an installation run.
func TestMainMenuToLearnTools(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenMainMenu
	m.Cursor = mainMenuRowIndex(t, m, "Learn About Tools")

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	newModel := result.(Model)

	if newModel.Screen != ScreenLearnTools {
		t.Errorf("Expected ScreenLearnTools, got %v", newModel.Screen)
	}
	if newModel.PrevScreen != ScreenMainMenu {
		t.Error("PrevScreen should be MainMenu")
	}

	// The terminal page is still one Enter away, and its way back is the hub, not
	// the main menu.
	opened, _ := newModel.Update(tea.KeyMsg{Type: tea.KeyEnter})
	page := opened.(Model)
	if page.Screen != ScreenLearnTerminals {
		t.Errorf("the hub's first row should open ScreenLearnTerminals, got %v", page.Screen)
	}
	if page.PrevScreen != ScreenLearnTools {
		t.Errorf("the terminal page should come back to the hub, PrevScreen = %v", page.PrevScreen)
	}
}

func TestMainMenuToKeymaps(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenMainMenu
	m.Cursor = mainMenuRowIndex(t, m, "Vim & Neovim")

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	hub := result.(Model)
	if hub.Screen != ScreenVimHub {
		t.Fatalf("the Vim row opened %v, want the Vim hub", hub.Screen)
	}
	hub.Cursor = 1 // Neovim Keymaps

	opened, _ := hub.Update(tea.KeyMsg{Type: tea.KeyEnter})
	newModel := opened.(Model)

	if newModel.Screen != ScreenKeymapsMenu {
		t.Errorf("Expected ScreenKeymapsMenu, got %v", newModel.Screen)
	}
	if newModel.PrevScreen != ScreenVimHub {
		t.Errorf("the keymaps reference comes back to the hub, PrevScreen = %v", newModel.PrevScreen)
	}
}

func TestMainMenuToLazyVim(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenMainMenu
	m.Cursor = mainMenuRowIndex(t, m, "Vim & Neovim")

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	hub := result.(Model)
	if hub.Screen != ScreenVimHub {
		t.Fatalf("the Vim row opened %v, want the Vim hub", hub.Screen)
	}
	hub.Cursor = 2 // LazyVim Guide

	opened, _ := hub.Update(tea.KeyMsg{Type: tea.KeyEnter})
	newModel := opened.(Model)

	if newModel.Screen != ScreenLearnLazyVim {
		t.Errorf("Expected ScreenLearnLazyVim, got %v", newModel.Screen)
	}
}

func TestMainMenuToRestore(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenMainMenu
	m.AvailableBackups = []system.BackupInfo{
		{Path: "/test", Timestamp: time.Now(), Files: []string{"test"}},
	}
	m.Cursor = mainMenuRowIndex(t, m, "Restore from Backup")

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	newModel := result.(Model)

	if newModel.Screen != ScreenRestoreBackup {
		t.Errorf("Expected ScreenRestoreBackup, got %v", newModel.Screen)
	}
}

func TestMainMenuExit(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenMainMenu

	opts := m.GetCurrentOptions()
	// Find Exit option
	for i, opt := range opts {
		if strings.Contains(opt, "Exit") {
			m.Cursor = i
			break
		}
	}

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	newModel := result.(Model)

	if !newModel.Quitting {
		t.Error("Exit option should quit")
	}
}

func TestOSSelectMac(t *testing.T) {
	m := NewModel()
	m.SystemInfo.IsWSL = false // Not testing WSL-specific flow
	m.Screen = ScreenOSSelect
	m.Cursor = 0 // macOS

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	newModel := result.(Model)

	if newModel.Choices.OS != "mac" {
		t.Errorf("Expected OS 'mac', got '%s'", newModel.Choices.OS)
	}
	// The platform question is a detour of the setup screen now: the choice is
	// recorded and the flow returns to the screen that opened it instead of
	// advancing to the terminal question.
	if newModel.Screen != ScreenSetup {
		t.Errorf("Expected ScreenSetup after the platform detour, got %v", newModel.Screen)
	}
}

func TestOSSelectLinux(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenOSSelect
	m.Cursor = 1 // Linux

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	newModel := result.(Model)

	if newModel.Choices.OS != "linux" {
		t.Errorf("Expected OS 'linux', got '%s'", newModel.Choices.OS)
	}
}

func TestTerminalSelectNoneReturnsToSetup(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenTerminalSelect
	m.Choices.OS = "mac"

	opts := m.GetCurrentOptions()
	for i, opt := range opts {
		if strings.Contains(strings.ToLower(opt), "none") {
			m.Cursor = i
			break
		}
	}

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	newModel := result.(Model)

	if newModel.Choices.Terminal != "none" {
		t.Errorf("Expected terminal 'none', got %q", newModel.Choices.Terminal)
	}
	if newModel.Screen != ScreenSetup {
		t.Errorf("A detour returns to the setup screen, got %v", newModel.Screen)
	}
}

func TestTerminalSelectWithTerminalReturnsToSetup(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenTerminalSelect
	m.Choices.OS = "mac"
	m.Cursor = 0 // Alacritty

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	newModel := result.(Model)

	if newModel.Screen != ScreenSetup {
		t.Errorf("A detour returns to the setup screen, got %v", newModel.Screen)
	}
	if newModel.Choices.Terminal != "alacritty" {
		t.Errorf("Expected terminal 'alacritty', got '%s'", newModel.Choices.Terminal)
	}
}

func TestFontSelect(t *testing.T) {
	t.Run("yes installs font", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenFontSelect
		m.Cursor = 0 // Yes

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		newModel := result.(Model)

		if !newModel.Choices.InstallFont {
			t.Error("Cursor 0 should set InstallFont true")
		}
		if newModel.Screen != ScreenSetup {
			t.Errorf("A detour returns to the setup screen, got %v", newModel.Screen)
		}
	})

	t.Run("no skips font", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenFontSelect
		m.Cursor = 1 // No

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		newModel := result.(Model)

		if newModel.Choices.InstallFont {
			t.Error("Cursor 1 should set InstallFont false")
		}
	})
}

func TestShellSelect(t *testing.T) {
	shells := []string{"fish", "zsh", "nushell"}

	for i, shell := range shells {
		t.Run(shell, func(t *testing.T) {
			m := NewModel()
			m.Screen = ScreenShellSelect
			m.Cursor = i

			result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			newModel := result.(Model)

			if newModel.Choices.Shell != shell {
				t.Errorf("Expected shell '%s', got '%s'", shell, newModel.Choices.Shell)
			}
			if newModel.Screen != ScreenSetup {
				t.Errorf("A detour returns to the setup screen, got %v", newModel.Screen)
			}
		})
	}
}

func TestWMSelect(t *testing.T) {
	wms := []string{"tmux", "zellij", "herdr", "none"}

	for i, wm := range wms {
		t.Run(wm, func(t *testing.T) {
			m := NewModel()
			m.Screen = ScreenWMSelect
			m.Cursor = i

			result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			newModel := result.(Model)

			if newModel.Choices.WindowMgr != wm {
				t.Errorf("Expected WM '%s', got '%s'", wm, newModel.Choices.WindowMgr)
			}
			if newModel.Screen != ScreenSetup {
				t.Errorf("A detour returns to the setup screen, got %v", newModel.Screen)
			}
		})
	}
}

func TestNvimSelectWithExistingConfigs(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenNvimSelect
	m.Cursor = 0 // Yes, install Neovim
	// Mock DetectExistingConfigs - we'll set ExistingConfigs after selection
	// In real flow, this happens in handleSelection

	// Simulate the flow
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	newModel := result.(Model)

	// The handler calls DetectExistingConfigs and sets the screen accordingly
	// Without mocking system calls, we just verify InstallNvim is set
	if !newModel.Choices.InstallNvim {
		t.Error("Should set InstallNvim true when cursor=0")
	}
}

// =============================================================================
// BACKUP FLOW TESTS
// =============================================================================

func TestBackupConfirmWithBackup(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenBackupConfirm
	m.Cursor = 0 // Install with Backup
	m.ExistingConfigs = []string{"nvim"}

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	newModel := result.(Model)

	if !newModel.Choices.CreateBackup {
		t.Error("Should set CreateBackup true")
	}
	// The decision is recorded, and then the setup screen the answer belongs to --
	// the review is its action row now, not this screen's Enter. The screen is a
	// detour: it lists the configurations the run would replace and records the
	// answer, which is what this guard holds.
	if newModel.Screen != ScreenSetup {
		t.Errorf("Expected the setup screen after the backup answer, got %v", newModel.Screen)
	}
	if cmd != nil {
		t.Error("Enter on the backup question started something before the plan")
	}
}

func TestBackupConfirmWithoutBackup(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenBackupConfirm
	m.Cursor = 1 // Install without Backup
	m.ExistingConfigs = []string{"nvim"}

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	newModel := result.(Model)

	if newModel.Choices.CreateBackup {
		t.Error("Should set CreateBackup false")
	}
	// Same contract as the backup answer above: the decision is recorded and the
	// setup screen comes back, with the review reached from its action row.
	if newModel.Screen != ScreenSetup {
		t.Errorf("Expected the setup screen after the backup answer, got %v", newModel.Screen)
	}
	if cmd != nil {
		t.Error("Enter on the backup question started something before the plan")
	}
}

func TestBackupConfirmCancel(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenBackupConfirm
	m.Cursor = 2 // Cancel

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	newModel := result.(Model)

	// The screen is a detour of the setup screen's backup row now, so its third row
	// is the way out of the screen rather than an abort: back to the setup screen
	// with nothing changed, which is its Escape too. Aborting the wizard is the
	// setup screen's own Escape, and it is the screen that offers it, not a row
	// buried in a list of configurations.
	if newModel.Screen != ScreenSetup {
		t.Errorf("Cancel should return to the setup screen, got %v", newModel.Screen)
	}
	if newModel.Choices.CreateBackup {
		t.Error("Cancel recorded a backup decision")
	}
}

func TestBackupConfirmEscape(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenBackupConfirm

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	newModel := result.(Model)

	// Every detour's Escape goes back where it came from. It used to return to the
	// Neovim question, which was the backup question's predecessor in the chain the
	// setup screen replaced.
	if newModel.Screen != ScreenSetup {
		t.Errorf("Escape on the backup detour should return to the setup screen, got %v", newModel.Screen)
	}
}

func TestRestoreBackupSelectBackup(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenRestoreBackup
	m.AvailableBackups = []system.BackupInfo{
		{Path: "/backup1", Timestamp: time.Now(), Files: []string{"a"}},
		{Path: "/backup2", Timestamp: time.Now(), Files: []string{"b"}},
	}
	m.Cursor = 1 // Second backup

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	newModel := result.(Model)

	if newModel.Screen != ScreenRestoreConfirm {
		t.Errorf("Expected ScreenRestoreConfirm, got %v", newModel.Screen)
	}
	if newModel.SelectedBackup != 1 {
		t.Errorf("Expected SelectedBackup 1, got %d", newModel.SelectedBackup)
	}
}

func TestRestoreBackupBack(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenRestoreBackup
	m.AvailableBackups = []system.BackupInfo{
		{Path: "/backup1", Timestamp: time.Now(), Files: []string{"a"}},
	}

	opts := m.GetCurrentOptions()
	for i, opt := range opts {
		if strings.Contains(opt, "Back") {
			m.Cursor = i
			break
		}
	}

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	newModel := result.(Model)

	if newModel.Screen != ScreenMainMenu {
		t.Errorf("Back should return to MainMenu, got %v", newModel.Screen)
	}
}

func TestRestoreConfirmCancel(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenRestoreConfirm
	m.AvailableBackups = []system.BackupInfo{
		{Path: "/backup1", Timestamp: time.Now(), Files: []string{"a"}},
	}
	m.SelectedBackup = 0
	m.Cursor = 2 // Cancel

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	newModel := result.(Model)

	if newModel.Screen != ScreenRestoreBackup {
		t.Errorf("Cancel should return to RestoreBackup, got %v", newModel.Screen)
	}
}

// =============================================================================
// LEARN SCREENS TESTS
// =============================================================================

func TestLearnTerminalsSelectTool(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenLearnTerminals
	m.Cursor = 0 // Alacritty

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	newModel := result.(Model)

	if newModel.ViewingTool != "alacritty" {
		t.Errorf("Expected ViewingTool 'alacritty', got '%s'", newModel.ViewingTool)
	}
}

func TestLearnTerminalsBack(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenLearnTerminals
	m.PrevScreen = ScreenTerminalSelect

	opts := m.GetCurrentOptions()
	for i, opt := range opts {
		if strings.Contains(opt, "Back") {
			m.Cursor = i
			break
		}
	}

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	newModel := result.(Model)

	if newModel.Screen != ScreenTerminalSelect {
		t.Errorf("Back should return to PrevScreen, got %v", newModel.Screen)
	}
	if newModel.ViewingTool != "" {
		t.Error("ViewingTool should be cleared")
	}
}

func TestLearnNvimViewFeatures(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenLearnNvim
	m.Cursor = 0 // View Features

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	newModel := result.(Model)

	if newModel.ViewingTool != "features" {
		t.Errorf("Expected ViewingTool 'features', got '%s'", newModel.ViewingTool)
	}
}

func TestLearnNvimViewKeymaps(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenLearnNvim
	m.Cursor = 1 // View Keymaps

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	newModel := result.(Model)

	if newModel.Screen != ScreenKeymaps {
		t.Errorf("Expected ScreenKeymaps, got %v", newModel.Screen)
	}
	if newModel.PrevScreen != ScreenLearnNvim {
		t.Error("PrevScreen should be LearnNvim")
	}
}

func TestLearnNvimLazyVimGuide(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenLearnNvim
	m.Cursor = 2 // LazyVim Guide

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	newModel := result.(Model)

	if newModel.Screen != ScreenLearnLazyVim {
		t.Errorf("Expected ScreenLearnLazyVim, got %v", newModel.Screen)
	}
}

// =============================================================================
// KEYMAPS TESTS
// =============================================================================

func TestKeymapsSelectCategory(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenKeymaps
	m.Cursor = 0 // First category

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	newModel := result.(Model)

	if newModel.Screen != ScreenKeymapCategory {
		t.Errorf("Expected ScreenKeymapCategory, got %v", newModel.Screen)
	}
	if newModel.SelectedCategory != 0 {
		t.Errorf("Expected SelectedCategory 0, got %d", newModel.SelectedCategory)
	}
	if newModel.KeymapScroll != 0 {
		t.Error("KeymapScroll should be reset to 0")
	}
}

func TestKeymapCategoryScroll(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenKeymapCategory
	// Use a category with many keymaps (Search Commands has 27)
	m.SelectedCategory = 5
	m.KeymapScroll = 5

	t.Run("scroll up", func(t *testing.T) {
		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
		newModel := result.(Model)
		if newModel.KeymapScroll != 4 {
			t.Errorf("Expected scroll 4, got %d", newModel.KeymapScroll)
		}
	})

	t.Run("scroll down", func(t *testing.T) {
		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		newModel := result.(Model)
		if newModel.KeymapScroll != 6 {
			t.Errorf("Expected scroll 6, got %d", newModel.KeymapScroll)
		}
	})
}

func TestKeymapCategoryScrollBounds(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenKeymapCategory
	m.SelectedCategory = 0 // Harpoon has only 7 keymaps
	m.KeymapScroll = 0

	// Try to scroll up from 0
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	newModel := result.(Model)
	if newModel.KeymapScroll != 0 {
		t.Error("Should not scroll below 0")
	}

	// Category with 7 items, showing 10 at a time = maxScroll 0
	// So down should also not change
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	newModel = result.(Model)
	if newModel.KeymapScroll != 0 {
		t.Error("Should not scroll when items <= view size")
	}
}

// =============================================================================
// LAZYVIM TOPIC TESTS
// =============================================================================

func TestLazyVimMenuSelectTopic(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenLearnLazyVim
	m.Cursor = 0

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	newModel := result.(Model)

	if newModel.Screen != ScreenLazyVimTopic {
		t.Errorf("Expected ScreenLazyVimTopic, got %v", newModel.Screen)
	}
	if newModel.SelectedLazyVimTopic != 0 {
		t.Errorf("Expected SelectedLazyVimTopic 0, got %d", newModel.SelectedLazyVimTopic)
	}
}

func TestLazyVimTopicScroll(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenLazyVimTopic
	m.SelectedLazyVimTopic = 0
	m.LazyVimScroll = 5

	t.Run("scroll up", func(t *testing.T) {
		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
		newModel := result.(Model)
		if newModel.LazyVimScroll != 4 {
			t.Errorf("Expected scroll 4, got %d", newModel.LazyVimScroll)
		}
	})

	t.Run("scroll down", func(t *testing.T) {
		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		newModel := result.(Model)
		if newModel.LazyVimScroll != 6 {
			t.Errorf("Expected scroll 6, got %d", newModel.LazyVimScroll)
		}
	})

	t.Run("page up", func(t *testing.T) {
		m.LazyVimScroll = 15
		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
		newModel := result.(Model)
		if newModel.LazyVimScroll != 5 {
			t.Errorf("Expected scroll 5, got %d", newModel.LazyVimScroll)
		}
	})

	t.Run("page down", func(t *testing.T) {
		m.LazyVimScroll = 0
		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
		newModel := result.(Model)
		if newModel.LazyVimScroll < 10 {
			t.Errorf("Expected scroll >= 10, got %d", newModel.LazyVimScroll)
		}
	})
}

func TestLazyVimTopicBack(t *testing.T) {
	// Keys that should go back from LazyVim topic
	// Note: space now activates leader mode, so it's not a "back" key
	backKeys := []tea.KeyMsg{
		{Type: tea.KeyEnter},
		{Type: tea.KeyEsc},
	}

	for _, key := range backKeys {
		t.Run(key.String(), func(t *testing.T) {
			m := NewModel()
			m.Screen = ScreenLazyVimTopic
			m.SelectedLazyVimTopic = 0
			m.LazyVimScroll = 10

			result, _ := m.Update(key)
			newModel := result.(Model)

			if newModel.Screen != ScreenLearnLazyVim {
				t.Errorf("Expected ScreenLearnLazyVim, got %v", newModel.Screen)
			}
			if newModel.LazyVimScroll != 0 {
				t.Error("LazyVimScroll should be reset")
			}
		})
	}
}

// =============================================================================
// ERROR SCREEN TESTS
// =============================================================================

func TestErrorScreenRetry(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenError
	m.ErrorMsg = "Test error"

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	newModel := result.(Model)

	if newModel.Screen != ScreenWelcome {
		t.Errorf("r should reset to Welcome, got %v", newModel.Screen)
	}
	if newModel.ErrorMsg != "" {
		t.Error("ErrorMsg should be cleared")
	}
}

func TestErrorScreenQuit(t *testing.T) {
	// Quitting is a command: the error screen retries on r and quits on q, and
	// Enter opens nothing here.
	m := NewModel()
	m.Screen = ScreenError
	if result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter}); result.(Model).Quitting {
		t.Error("Enter quit the error screen; quitting runs on q")
	}

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	newModel := result.(Model)

	if !newModel.Quitting {
		t.Error("q on the error screen should quit")
	}
}

// =============================================================================
// COMPLETE SCREEN TESTS
// =============================================================================

func TestCompleteScreenQuit(t *testing.T) {
	// Quitting is a command and runs on its own letter: Enter opens and Space
	// toggles a switch, and this screen has nothing to open and no switch.
	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyEnter},
		{Type: tea.KeyRunes, Runes: []rune{' '}},
	} {
		m := NewModel()
		m.Screen = ScreenComplete
		if result, _ := m.Update(key); result.(Model).Quitting {
			t.Errorf("%s quit the completion screen; quitting runs on q", key.String())
		}
	}

	m := NewModel()
	m.Screen = ScreenComplete
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if !result.(Model).Quitting {
		t.Error("q should quit from the complete screen")
	}
}

// =============================================================================
// INSTALLING SCREEN TESTS
// =============================================================================

func TestInstallingToggleDetails(t *testing.T) {
	// Space is not the details key: it toggles a switch, and this screen has none.
	m := NewModel()
	m.Screen = ScreenInstalling
	if result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}}); result.(Model).ShowDetails {
		t.Error("Space toggled the details box")
	}

	// The box opens and closes on its own letter, in one press.
	m = NewModel()
	m.Screen = ScreenInstalling
	m.ShowDetails = false
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if !result.(Model).ShowDetails {
		t.Error("d should toggle ShowDetails to true")
	}

	result, _ = result.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if result.(Model).ShowDetails {
		t.Error("a second d should toggle ShowDetails back to false")
	}
}

func TestInstallingCannotToggleDetailsOnOtherScreens(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenMainMenu
	m.ShowDetails = false

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	newModel := result.(Model)

	if newModel.ShowDetails {
		t.Error("d should not toggle details outside installing screen")
	}
}

// =============================================================================
// MESSAGE HANDLER TESTS
// =============================================================================

func TestWindowSizeMessage(t *testing.T) {
	m := NewModel()

	result, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	newModel := result.(Model)

	if newModel.Width != 120 || newModel.Height != 40 {
		t.Errorf("Expected 120x40, got %dx%d", newModel.Width, newModel.Height)
	}
}

func TestLoadBackupsMessage(t *testing.T) {
	m := NewModel()

	backups := []system.BackupInfo{
		{Path: "/test1", Timestamp: time.Now(), Files: []string{"a"}},
		{Path: "/test2", Timestamp: time.Now(), Files: []string{"b"}},
	}

	result, _ := m.Update(loadBackupsMsg{backups: backups})
	newModel := result.(Model)

	if len(newModel.AvailableBackups) != 2 {
		t.Errorf("Expected 2 backups, got %d", len(newModel.AvailableBackups))
	}
}

func TestStepProgressMessage(t *testing.T) {
	m := NewModel()
	m.Steps = []InstallStep{
		{ID: "test", Status: StatusRunning, Progress: 0},
	}

	result, _ := m.Update(stepProgressMsg{stepID: "test", progress: 0.5, log: "Test log"})
	newModel := result.(Model)

	if newModel.Steps[0].Progress != 0.5 {
		t.Errorf("Expected progress 0.5, got %f", newModel.Steps[0].Progress)
	}
	if len(newModel.LogLines) != 1 || newModel.LogLines[0] != "Test log" {
		t.Error("Log line should be added")
	}
}

func TestStepProgressMessageLogLimit(t *testing.T) {
	m := NewModel()
	m.Steps = []InstallStep{{ID: "test"}}

	// Add 25 log lines
	for i := 0; i < 25; i++ {
		result, _ := m.Update(stepProgressMsg{stepID: "test", log: "line"})
		m = result.(Model)
	}

	if len(m.LogLines) > 20 {
		t.Errorf("Log lines should be capped at 20, got %d", len(m.LogLines))
	}
}

func TestStepCompleteMessageSuccess(t *testing.T) {
	m := NewModel()
	m.Steps = []InstallStep{
		{ID: "step1", Status: StatusRunning},
		{ID: "step2", Status: StatusPending},
	}
	m.CurrentStep = 0

	result, cmd := m.Update(stepCompleteMsg{stepID: "step1", err: nil})
	newModel := result.(Model)

	if newModel.Steps[0].Status != StatusDone {
		t.Error("Step should be marked as Done")
	}
	if newModel.Steps[0].Progress != 1.0 {
		t.Error("Progress should be 1.0")
	}
	if newModel.CurrentStep != 1 {
		t.Errorf("CurrentStep should be 1, got %d", newModel.CurrentStep)
	}
	if cmd == nil {
		t.Error("Should return command for next step")
	}
}

func TestStepCompleteMessageFailure(t *testing.T) {
	m := NewModel()
	m.Steps = []InstallStep{
		{ID: "step1", Name: "Test Step", Status: StatusRunning},
	}
	m.CurrentStep = 0

	testErr := fmt.Errorf("test error")
	result, _ := m.Update(stepCompleteMsg{stepID: "step1", err: testErr})
	newModel := result.(Model)

	if newModel.Steps[0].Status != StatusFailed {
		t.Error("Step should be marked as Failed")
	}
	if newModel.Screen != ScreenError {
		t.Errorf("Should go to Error screen, got %v", newModel.Screen)
	}
	expectedMsg := "Step 'Test Step' failed:\ntest error"
	if newModel.ErrorMsg != expectedMsg {
		t.Errorf("ErrorMsg should be '%s', got '%s'", expectedMsg, newModel.ErrorMsg)
	}
}

func TestInstallCompleteMessage(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenInstalling

	result, _ := m.Update(installCompleteMsg{totalTime: 123.45})
	newModel := result.(Model)

	if newModel.Screen != ScreenComplete {
		t.Errorf("Expected ScreenComplete, got %v", newModel.Screen)
	}
	if newModel.TotalTime != 123.45 {
		t.Errorf("Expected TotalTime 123.45, got %f", newModel.TotalTime)
	}
}

// =============================================================================
// INIT TESTS
// =============================================================================

func TestInitReturnsCommands(t *testing.T) {
	m := NewModel()
	cmd := m.Init()

	if cmd == nil {
		t.Error("Init should return commands")
	}
}

// =============================================================================
// SCREEN STRING REPRESENTATION (for debugging)
// =============================================================================

// screenNames names every Screen constant, for the failure messages of the tests
// in this package. It is a map rather than a slice indexed by the constant's
// value: a slice made a name a matter of counting, so a screen inserted above
// another one was renamed after its neighbour and the newest ones printed as
// "Unknown" in exactly the messages that were supposed to name them.
// TestEveryDeclaredScreenHasAName fails by name when a constant is added without
// an entry here.
var screenNames = map[Screen]string{
	ScreenWelcome:              "Welcome",
	ScreenVimHub:               "VimHub",
	ScreenMainMenu:             "MainMenu",
	ScreenOSSelect:             "OSSelect",
	ScreenTerminalSelect:       "TerminalSelect",
	ScreenFontSelect:           "FontSelect",
	ScreenShellSelect:          "ShellSelect",
	ScreenWMSelect:             "WMSelect",
	ScreenNvimSelect:           "NvimSelect",
	ScreenInstalling:           "Installing",
	ScreenComplete:             "Complete",
	ScreenError:                "Error",
	ScreenLearnTerminals:       "LearnTerminals",
	ScreenLearnShells:          "LearnShells",
	ScreenLearnWM:              "LearnWM",
	ScreenLearnNvim:            "LearnNvim",
	ScreenKeymaps:              "Keymaps",
	ScreenKeymapCategory:       "KeymapCategory",
	ScreenKeymapsMenu:          "KeymapsMenu",
	ScreenKeymapsTmux:          "KeymapsTmux",
	ScreenKeymapsTmuxCat:       "KeymapsTmuxCat",
	ScreenKeymapsZellij:        "KeymapsZellij",
	ScreenKeymapsZellijCat:     "KeymapsZellijCat",
	ScreenKeymapsGhostty:       "KeymapsGhostty",
	ScreenKeymapsGhosttyCat:    "KeymapsGhosttyCat",
	ScreenLearnLazyVim:         "LearnLazyVim",
	ScreenLazyVimTopic:         "LazyVimTopic",
	ScreenBackupConfirm:        "BackupConfirm",
	ScreenRestoreBackup:        "RestoreBackup",
	ScreenRestoreConfirm:       "RestoreConfirm",
	ScreenGhosttyWarning:       "GhosttyWarning",
	ScreenTrainerMenu:          "TrainerMenu",
	ScreenTrainerLesson:        "TrainerLesson",
	ScreenTrainerPractice:      "TrainerPractice",
	ScreenTrainerBoss:          "TrainerBoss",
	ScreenTrainerResult:        "TrainerResult",
	ScreenTrainerBossResult:    "TrainerBossResult",
	ScreenKeymapsHerdr:         "KeymapsHerdr",
	ScreenKeymapsHerdrCat:      "KeymapsHerdrCat",
	ScreenKeymapsKitty:         "KeymapsKitty",
	ScreenKeymapsKittyCat:      "KeymapsKittyCat",
	ScreenKeymapsWezTerm:       "KeymapsWezTerm",
	ScreenKeymapsAlacritty:     "KeymapsAlacritty",
	ScreenUtilities:            "Utilities",
	ScreenThemePicker:          "ThemePicker",
	ScreenWSLResources:         "WSLResources",
	ScreenTerminalCapabilities: "TerminalCapabilities",
	ScreenShellAudit:           "ShellAudit",
	ScreenUpdate:               "Update",
	ScreenAdminPassword:        "AdminPassword",
	ScreenInstallReview:        "InstallReview",
	ScreenLearnTools:           "LearnTools",
	ScreenLearnFont:            "LearnFont",
	ScreenLearnPrompts:         "LearnPrompts",
	ScreenLearnTooling:         "LearnTooling",
	ScreenLearnPlugins:         "LearnPlugins",
	ScreenLearnToolset:         "LearnToolset",
	ScreenInstallVerify:        "InstallVerify",
	// ScreenSetup is the last screen main's merge declared: it went after the screens
	// above, so its name belongs here in the same order, and the guard that reads this
	// map is what said so rather than a guess about where the line should go.
	ScreenSetup: "Setup",
	// The update center is declared after ScreenSetup and before the network
	// diagnosis: the section's eighth destination, named here in the order model.go
	// declares it.
	ScreenUpdateCenter: "UpdateCenter",
	// The network diagnosis is declared after ScreenSetup and is the last screen the
	// enum holds: the same guard named it, and its name belongs in the same order.
	ScreenNetworkDiagnostics: "NetworkDiagnostics",
	// The WSL performance advisor and its two assistants are declared after the
	// network diagnosis: the guard named them too, so they follow it in the same
	// order -- the report and the reading state it shows before the facts land, the
	// measurement screen, and the assisted copy.
	ScreenWSLAdvisor:   "WSLAdvisor",
	ScreenWSLBenchmark: "WSLBenchmark",
	ScreenWSLMigration: "WSLMigration",
	// The SSH manager's seven screens follow the advisor's in the enum, so their names
	// follow its here. The guard in screen_coverage_test.go walks every Screen value in model.go
	// and fails until each one has an entry, which is what asked for these.
	ScreenSSHManager:     "SSHManager",
	ScreenSSHHost:        "SSHHost",
	ScreenSSHForm:        "SSHForm",
	ScreenSSHKeys:        "SSHKeys",
	ScreenSSHAgent:       "SSHAgent",
	ScreenSSHDiagnostics: "SSHDiagnostics",
	ScreenSSHTest:        "SSHTest",
	// The disk cleaner is the screen this front added, and model.go declares it after
	// every screen above -- the guard in screen_coverage_test.go named it as
	// Screen(58) printing as "Unknown", which is the failure this entry answers.
	ScreenDiskCleaner: "DiskCleaner",
	// The port and process manager and its stop confirmation, in an alignment group
	// of their own: their names are longer than every key above, and gofmt aligns a
	// block as one, so sharing it would re-space the whole map.
	ScreenPortManager: "PortManager",
	ScreenPortStop:    "PortStop",

	// The doctor and its confirmation arrived with the other front's merge -- the
	// guard named them as Screen(71) and Screen(72) printing as "Unknown" on the
	// front that added them, which is the failure these entries answer. They keep a
	// group of their own for the same reason as the pair above.
	ScreenDoctor:    "Doctor",
	ScreenDoctorFix: "DoctorFix",
}

func (s Screen) String() string {
	if name, ok := screenNames[s]; ok {
		return name
	}
	return "Unknown"
}

// Helper to avoid import error
var _ = fmt.Sprintf
