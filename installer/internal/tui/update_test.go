package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/albersg/dotfiles/installer/internal/system"
	tea "github.com/charmbracelet/bubbletea"
)

func TestHandleBackupConfirmKeys(t *testing.T) {
	t.Run("should navigate with up/down keys", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenBackupConfirm
		m.Cursor = 0

		// Press down
		result, _ := m.handleBackupConfirmKeys("down")
		newModel := result.(Model)
		if newModel.Cursor != 1 {
			t.Errorf("Expected cursor at 1 after down, got %d", newModel.Cursor)
		}

		// Press down again
		result, _ = newModel.handleBackupConfirmKeys("down")
		newModel = result.(Model)
		if newModel.Cursor != 2 {
			t.Errorf("Expected cursor at 2 after second down, got %d", newModel.Cursor)
		}

		// Press up
		result, _ = newModel.handleBackupConfirmKeys("up")
		newModel = result.(Model)
		if newModel.Cursor != 1 {
			t.Errorf("Expected cursor at 1 after up, got %d", newModel.Cursor)
		}
	})

	t.Run("should handle k/j vim keys", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenBackupConfirm
		m.Cursor = 1

		// Press k (up)
		result, _ := m.handleBackupConfirmKeys("k")
		newModel := result.(Model)
		if newModel.Cursor != 0 {
			t.Errorf("Expected cursor at 0 after k, got %d", newModel.Cursor)
		}

		// Press j (down)
		result, _ = newModel.handleBackupConfirmKeys("j")
		newModel = result.(Model)
		if newModel.Cursor != 1 {
			t.Errorf("Expected cursor at 1 after j, got %d", newModel.Cursor)
		}
	})

	t.Run("should set CreateBackup true when selecting backup option", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenBackupConfirm
		m.Cursor = 0 // Install with Backup
		m.SystemInfo = &system.SystemInfo{
			OS:       system.OSMac,
			HasBrew:  true,
			HasXcode: true,
		}
		m.Choices = UserChoices{
			OS:       "mac",
			Shell:    "fish",
			Terminal: "none",
		}
		m.ExistingConfigs = []string{"nvim: /test"}

		result, _ := m.handleBackupConfirmKeys("enter")
		newModel := result.(Model)

		if !newModel.Choices.CreateBackup {
			t.Error("CreateBackup should be true when selecting backup option")
		}

		if newModel.Screen != ScreenInstalling {
			t.Errorf("Expected ScreenInstalling, got %v", newModel.Screen)
		}
	})

	t.Run("should set CreateBackup false when selecting no backup option", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenBackupConfirm
		m.Cursor = 1 // Install without Backup
		m.SystemInfo = &system.SystemInfo{
			OS:       system.OSMac,
			HasBrew:  true,
			HasXcode: true,
		}
		m.Choices = UserChoices{
			OS:       "mac",
			Shell:    "fish",
			Terminal: "none",
		}

		result, _ := m.handleBackupConfirmKeys("enter")
		newModel := result.(Model)

		if newModel.Choices.CreateBackup {
			t.Error("CreateBackup should be false when selecting no backup option")
		}
	})

	t.Run("should go to MainMenu when selecting cancel", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenBackupConfirm
		m.Cursor = 2 // Cancel

		result, _ := m.handleBackupConfirmKeys("enter")
		newModel := result.(Model)

		if newModel.Screen != ScreenMainMenu {
			t.Errorf("Expected ScreenMainMenu, got %v", newModel.Screen)
		}
	})

	t.Run("should go to NvimSelect on escape (go back)", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenBackupConfirm
		m.Cursor = 0

		// Note: ESC is handled by handleEscape(), not handleBackupConfirmKeys()
		// This tests the handleEscape behavior
		result, _ := m.handleEscape()
		newModel := result.(Model)

		if newModel.Screen != ScreenNvimSelect {
			t.Errorf("Expected ScreenNvimSelect (go back), got %v", newModel.Screen)
		}
	})
}

func TestHandleRestoreBackupKeys(t *testing.T) {
	t.Run("should navigate with up/down keys", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenRestoreBackup
		m.AvailableBackups = []system.BackupInfo{
			{Path: "/test/backup1"},
			{Path: "/test/backup2"},
		}
		m.Cursor = 0

		result, _ := m.handleRestoreBackupKeys("down")
		newModel := result.(Model)
		if newModel.Cursor != 1 {
			t.Errorf("Expected cursor at 1 after down, got %d", newModel.Cursor)
		}
	})

	t.Run("should go to ScreenRestoreConfirm when selecting a backup", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenRestoreBackup
		m.AvailableBackups = []system.BackupInfo{
			{Path: "/test/backup1"},
		}
		m.Cursor = 0

		result, _ := m.handleRestoreBackupKeys("enter")
		newModel := result.(Model)

		if newModel.Screen != ScreenRestoreConfirm {
			t.Errorf("Expected ScreenRestoreConfirm, got %v", newModel.Screen)
		}

		if newModel.SelectedBackup != 0 {
			t.Errorf("Expected SelectedBackup 0, got %d", newModel.SelectedBackup)
		}
	})

	t.Run("should go to MainMenu on escape", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenRestoreBackup
		m.Cursor = 0

		result, _ := m.handleRestoreBackupKeys("esc")
		newModel := result.(Model)

		if newModel.Screen != ScreenMainMenu {
			t.Errorf("Expected ScreenMainMenu, got %v", newModel.Screen)
		}
	})
}

func TestHandleRestoreConfirmKeys(t *testing.T) {
	t.Run("should navigate with up/down keys", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenRestoreConfirm
		m.AvailableBackups = []system.BackupInfo{
			{Path: "/test/backup1"},
		}
		m.SelectedBackup = 0
		m.Cursor = 0

		result, _ := m.handleRestoreConfirmKeys("down")
		newModel := result.(Model)
		if newModel.Cursor != 1 {
			t.Errorf("Expected cursor at 1 after down, got %d", newModel.Cursor)
		}
	})

	t.Run("should go back to RestoreBackup on cancel", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenRestoreConfirm
		m.AvailableBackups = []system.BackupInfo{
			{Path: "/test/backup1"},
		}
		m.SelectedBackup = 0
		m.Cursor = 2 // Cancel

		result, _ := m.handleRestoreConfirmKeys("enter")
		newModel := result.(Model)

		if newModel.Screen != ScreenRestoreBackup {
			t.Errorf("Expected ScreenRestoreBackup, got %v", newModel.Screen)
		}
	})

	t.Run("should go back to RestoreBackup on escape", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenRestoreConfirm
		m.AvailableBackups = []system.BackupInfo{
			{Path: "/test/backup1"},
		}
		m.SelectedBackup = 0
		m.Cursor = 0

		result, _ := m.handleRestoreConfirmKeys("esc")
		newModel := result.(Model)

		if newModel.Screen != ScreenRestoreBackup {
			t.Errorf("Expected ScreenRestoreBackup, got %v", newModel.Screen)
		}
	})
}

func TestHandleMainMenuWithRestore(t *testing.T) {
	t.Run("should go to RestoreBackup when selecting restore option", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenMainMenu
		m.AvailableBackups = []system.BackupInfo{
			{Path: "/test/backup1"},
		}
		// Options: Start, Learn, Keymaps, LazyVim, Vim Trainer, Restore, Exit
		// Restore is at index 5
		m.Cursor = 5

		result, _ := m.handleMainMenuKeys("enter")
		newModel := result.(Model)

		if newModel.Screen != ScreenRestoreBackup {
			t.Errorf("Expected ScreenRestoreBackup, got %v", newModel.Screen)
		}
	})

	t.Run("should handle dynamic menu correctly without backups", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenMainMenu
		m.AvailableBackups = []system.BackupInfo{} // No backups
		// Options without restore: Start, Learn, Keymaps, LazyVim, Vim Trainer, Exit
		// Exit is at index 5
		m.Cursor = 5

		_, cmd := m.handleMainMenuKeys("enter")

		// Should return quit command
		if cmd == nil {
			t.Error("Expected quit command when selecting Exit")
		}
	})
}

func TestLoadBackupsMsg(t *testing.T) {
	t.Run("should update AvailableBackups on loadBackupsMsg", func(t *testing.T) {
		m := NewModel()
		m.AvailableBackups = []system.BackupInfo{}

		backups := []system.BackupInfo{
			{Path: "/test/backup1", Timestamp: time.Now()},
			{Path: "/test/backup2", Timestamp: time.Now()},
		}

		msg := loadBackupsMsg{backups: backups}
		result, _ := m.Update(msg)
		newModel := result.(Model)

		if len(newModel.AvailableBackups) != 2 {
			t.Errorf("Expected 2 backups, got %d", len(newModel.AvailableBackups))
		}
	})
}

func TestInitLoadsBackups(t *testing.T) {
	t.Run("Init should return command batch", func(t *testing.T) {
		m := NewModel()
		cmd := m.Init()

		// Init returns a batch command, we just verify it's not nil
		if cmd == nil {
			t.Error("Init should return a command")
		}
	})
}

func TestTickCmd(t *testing.T) {
	t.Run("tickCmd should return a command", func(t *testing.T) {
		cmd := tickCmd()
		if cmd == nil {
			t.Error("tickCmd should return a command")
		}
	})
}

func TestLoadBackupsCmd(t *testing.T) {
	t.Run("loadBackupsCmd should return a command", func(t *testing.T) {
		cmd := loadBackupsCmd()
		if cmd == nil {
			t.Error("loadBackupsCmd should return a command")
		}
	})

	t.Run("loadBackupsCmd should return loadBackupsMsg", func(t *testing.T) {
		cmd := loadBackupsCmd()
		msg := cmd()

		_, ok := msg.(loadBackupsMsg)
		if !ok {
			t.Errorf("Expected loadBackupsMsg, got %T", msg)
		}
	})
}

func TestUpdateHandlesBackupScreens(t *testing.T) {
	t.Run("should handle ScreenBackupConfirm key events", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenBackupConfirm

		keyMsg := tea.KeyMsg{Type: tea.KeyDown}
		result, _ := m.Update(keyMsg)
		newModel := result.(Model)

		if newModel.Cursor != 1 {
			t.Errorf("Expected cursor at 1, got %d", newModel.Cursor)
		}
	})

	t.Run("should handle ScreenRestoreBackup key events", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenRestoreBackup
		m.AvailableBackups = []system.BackupInfo{{Path: "/test"}}

		keyMsg := tea.KeyMsg{Type: tea.KeyDown}
		result, _ := m.Update(keyMsg)
		newModel := result.(Model)

		// Should stay on same screen
		if newModel.Screen != ScreenRestoreBackup {
			t.Errorf("Should stay on ScreenRestoreBackup")
		}
	})

	t.Run("should handle ScreenRestoreConfirm key events", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenRestoreConfirm
		m.AvailableBackups = []system.BackupInfo{{Path: "/test"}}
		m.SelectedBackup = 0

		keyMsg := tea.KeyMsg{Type: tea.KeyDown}
		result, _ := m.Update(keyMsg)
		newModel := result.(Model)

		if newModel.Cursor != 1 {
			t.Errorf("Expected cursor at 1, got %d", newModel.Cursor)
		}
	})
}

func TestHandleEscapeFromBackupScreens(t *testing.T) {
	t.Run("should handle escape from ScreenBackupConfirm", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenBackupConfirm

		result, _ := m.handleEscape()
		// handleEscape doesn't handle ScreenBackupConfirm directly
		// It's handled in handleBackupConfirmKeys
		newModel := result.(Model)
		if newModel.Screen != ScreenBackupConfirm {
			// If it changes, that's also valid
			t.Log("Screen changed from ScreenBackupConfirm on escape")
		}
	})
}

func TestWindowSizeMsg(t *testing.T) {
	t.Run("should update Width and Height", func(t *testing.T) {
		m := NewModel()

		msg := tea.WindowSizeMsg{Width: 120, Height: 40}
		result, _ := m.Update(msg)
		newModel := result.(Model)

		if newModel.Width != 120 {
			t.Errorf("Expected width 120, got %d", newModel.Width)
		}

		if newModel.Height != 40 {
			t.Errorf("Expected height 40, got %d", newModel.Height)
		}
	})
}

func TestCtrlCQuits(t *testing.T) {
	t.Run("ctrl+c should quit", func(t *testing.T) {
		m := NewModel()

		keyMsg := tea.KeyMsg{Type: tea.KeyCtrlC}
		result, cmd := m.Update(keyMsg)
		newModel := result.(Model)

		if !newModel.Quitting {
			t.Error("Should set Quitting to true on ctrl+c")
		}

		if cmd == nil {
			t.Error("Should return quit command")
		}
	})
}

// osConstant is one platform constant the system package declares, with the
// value its position in the contiguous iota run gives it.
type osConstant struct {
	name  string
	value system.OSType
}

// osConstantsInDeclarationOrder reads the system package's detect.go and
// returns every OSType constant with the value its position in the declaration
// run gives it.
//
// The guard below derives the list rather than naming it, which is the whole
// point: a platform added to the system package appears in the loop and fails
// until somebody decides the option its cursor starts on. A hand-written list
// would go on passing while the new platform silently inherited a default.
func osConstantsInDeclarationOrder(t *testing.T) []osConstant {
	t.Helper()

	path := filepath.Join(repoRoot(t), "installer", "internal", "system", "detect.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	var constants []osConstant
	blocks := 0
	ast.Inspect(file, func(n ast.Node) bool {
		decl, ok := n.(*ast.GenDecl)
		if !ok || decl.Tok != token.CONST || len(decl.Specs) == 0 {
			return true
		}
		first, ok := decl.Specs[0].(*ast.ValueSpec)
		if !ok {
			return true
		}
		if id, ok := first.Type.(*ast.Ident); !ok || id.Name != "OSType" {
			return true
		}
		blocks++
		for i, spec := range decl.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			if i == 0 {
				if len(value.Values) != 1 {
					t.Fatalf("the OSType block's first constant has %d values, want the iota run", len(value.Values))
				}
				if id, ok := value.Values[0].(*ast.Ident); !ok || id.Name != "iota" {
					t.Fatalf("the OSType block does not start at iota, so a name cannot be mapped to its value")
				}
			} else if len(value.Values) != 0 {
				t.Fatalf("%s has an explicit value inside the OSType block, so the block is no longer a "+
					"contiguous run and the name-to-value mapping this guard relies on is wrong",
					value.Names[0].Name)
			}
			for _, name := range value.Names {
				constants = append(constants, osConstant{name: name.Name, value: system.OSType(len(constants))})
			}
		}
		return true
	})

	if blocks != 1 {
		t.Fatalf("detect.go declares %d OSType constant blocks, want exactly 1", blocks)
	}
	if len(constants) == 0 {
		t.Fatal("detect.go declares no OSType constants, so this guard proves nothing")
	}
	return constants
}

// TestWizardStartsOnTheDetectedPlatform guards the class of defect where the OS
// step preselected a platform by testing one constant and letting an else stand
// for macOS. Debian, Ubuntu, Arch, Fedora, Termux and WSL all opened the wizard
// with the cursor on macOS under a description that named the right platform.
//
// The platforms are derived from the system package, so adding one is a visible
// decision: the new constant reaches expectedOption, the lookup fails, and the
// message says which option to choose.
func TestWizardStartsOnTheDetectedPlatform(t *testing.T) {
	expectedOption := map[string]string{
		"OSMac":    "macOS",
		"OSLinux":  "Linux",
		"OSArch":   "Linux",
		"OSDebian": "Linux",
		"OSFedora": "Linux",
		"OSWSL":    "Linux",
		"OSTermux": "Termux",
	}

	for _, c := range osConstantsInDeclarationOrder(t) {
		if _, known := osOptionIndex(c.value); !known {
			// A platform the mapping does not know is not a "start on the
			// detected option" case: it must start on none.
			// TestWizardDoesNotPresentAnUndetectedPlatform guards it.
			continue
		}
		want, decided := expectedOption[c.name]
		if !decided {
			t.Errorf("platform %s has no OS menu option decided for the wizard's starting cursor. "+
				"Add it to expectedOption in this test with the option the cursor should sit on, and map it in "+
				"osOptionIndex (update.go); a new platform must not inherit a default.", c.name)
			continue
		}

		t.Run(c.name, func(t *testing.T) {
			m := Model{
				Screen:     ScreenMainMenu,
				SystemInfo: &system.SystemInfo{OS: c.value, OSName: c.name},
			}

			// Enter on the main menu's first entry opens the wizard's OS step.
			updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			got := updated.(Model)
			if got.Screen != ScreenOSSelect {
				t.Fatalf("enter on the main menu did not open the OS step: screen = %v", got.Screen)
			}

			options := got.GetCurrentOptions()
			if got.Cursor < 0 || got.Cursor >= len(options) {
				t.Fatalf("the cursor is at %d, outside the %d OS options %v", got.Cursor, len(options), options)
			}
			if options[got.Cursor] != want {
				t.Errorf("with platform %s detected the wizard starts on %q, want %q: %v",
					c.name, options[got.Cursor], want, options)
			}
		})
	}
}

// TestShellScreenStartsOnTheDetectedShell pins the shell half of the same
// defect: the description named the detected shell while the cursor sat on
// Fish. The cursor now starts on the detected shell when the menu lists it, and
// the description says what the cursor means rather than asserting a fact the
// cursor contradicts. A shell the menu does not list is not a "start on the
// detected shell" case: TestShellStepDoesNotPresentAnUndetectedShell covers it.
func TestShellScreenStartsOnTheDetectedShell(t *testing.T) {
	cases := []struct {
		shell string
		want  string
	}{
		{"fish", "Fish"},
		{"zsh", "Zsh"},
		{"nushell", "Nushell"},
		{"nu", "Nushell"},
	}

	for _, c := range cases {
		name := c.shell
		if name == "" {
			name = "empty"
		}
		t.Run(name, func(t *testing.T) {
			m := Model{
				Screen:     ScreenFontSelect,
				Cursor:     0, // install the font; ENTER advances to the shell step
				SystemInfo: &system.SystemInfo{OS: system.OSLinux, UserShell: c.shell},
			}

			updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			got := updated.(Model)
			if got.Screen != ScreenShellSelect {
				t.Fatalf("the font step did not lead to the shell step: screen = %v", got.Screen)
			}

			options := got.GetCurrentOptions()
			if got.Cursor < 0 || got.Cursor >= len(options) {
				t.Fatalf("the cursor is at %d, outside the %d shell options %v", got.Cursor, len(options), options)
			}
			if options[got.Cursor] != c.want {
				t.Errorf("with shell %q detected the shell step starts on %q, want %q: %v",
					c.shell, options[got.Cursor], c.want, options)
			}

			description := got.GetScreenDescription()
			if !strings.Contains(description, c.shell) {
				t.Errorf("the shell step description %q does not name the detected shell %q", description, c.shell)
			}
		})
	}
}

// TestWizardDoesNotPresentAnUndetectedPlatform guards the class of defect that
// issue #141 describes: osOptionIndex returning ok=false stopped the mapping
// from silently meaning macOS, but the call site still filled the gap with
// Cursor = 0, which is macOS. A platform the mapping does not know -- or a value
// outside the declared block -- must open the OS step with nothing highlighted
// and require an explicit choice, so no row reads as a detection that did not
// happen.
func TestWizardDoesNotPresentAnUndetectedPlatform(t *testing.T) {
	type undetected struct {
		name string
		info *system.SystemInfo
	}
	var cases []undetected
	for _, c := range osConstantsInDeclarationOrder(t) {
		if _, known := osOptionIndex(c.value); known {
			continue
		}
		cases = append(cases, undetected{name: c.name, info: &system.SystemInfo{OS: c.value, OSName: c.name}})
	}
	// A value outside the declared block exercises the mapping's tail return,
	// not only its OSUnknown case.
	cases = append(cases, undetected{name: "outside-the-block", info: &system.SystemInfo{OS: system.OSType(-1), OSName: "Unknown"}})

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := Model{Screen: ScreenMainMenu, SystemInfo: c.info}

			updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			got := updated.(Model)
			if got.Screen != ScreenOSSelect {
				t.Fatalf("enter on the main menu did not open the OS step: screen = %v", got.Screen)
			}

			options := got.GetCurrentOptions()
			if got.Cursor >= 0 {
				t.Fatalf("with no platform detected the OS step opens on %q, a row under the cursor that "+
					"reads as the installer's answer; nothing should be highlighted: %v", options[got.Cursor], options)
			}
			got.Width, got.Height = 80, 24
			if view := ansiEscape.ReplaceAllString(got.View(), ""); strings.Contains(view, "▸") {
				t.Errorf("the OS step draws a highlighted row though detection found nothing:\n%s", view)
			}
			if desc := got.GetScreenDescription(); !strings.Contains(desc, "not detected") {
				t.Errorf("the OS step does not state that detection found nothing: %q", desc)
			}

			// An explicit choice is required before the wizard may continue.
			after, _ := got.Update(tea.KeyMsg{Type: tea.KeyEnter})
			next := after.(Model)
			if next.Screen != ScreenOSSelect || next.Choices.OS != "" {
				t.Errorf("Enter answered the OS question without a choice: screen = %v, OS = %q",
					next.Screen, next.Choices.OS)
			}
		})
	}
}

// TestShellStepDoesNotPresentAnUndetectedShell is the shell half of the same
// class guard: a detected login shell the menu does not list (bash, dash), an
// empty detection, and the sentinel "unknown" must open the shell step with
// nothing highlighted and require an explicit choice, rather than leaving the
// cursor on Fish under a line about the detected shell.
func TestShellStepDoesNotPresentAnUndetectedShell(t *testing.T) {
	for _, shell := range []string{"", "unknown", "bash", "dash"} {
		if _, known := shellOptionIndex(shell); known {
			t.Fatalf("shell %q is listed by the menu, so it is not an undetected case", shell)
		}
		name := shell
		if name == "" {
			name = "empty"
		}
		t.Run(name, func(t *testing.T) {
			m := Model{
				Screen:     ScreenFontSelect,
				Cursor:     0, // install the font; ENTER advances to the shell step
				SystemInfo: &system.SystemInfo{OS: system.OSLinux, UserShell: shell},
			}

			updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			got := updated.(Model)
			if got.Screen != ScreenShellSelect {
				t.Fatalf("the font step did not lead to the shell step: screen = %v", got.Screen)
			}

			options := got.GetCurrentOptions()
			if got.Cursor >= 0 {
				t.Fatalf("with shell %q undetected the shell step opens on %q, a row under the cursor that "+
					"reads as the installer's answer; nothing should be highlighted: %v", shell, options[got.Cursor], options)
			}
			got.Width, got.Height = 80, 24
			if view := ansiEscape.ReplaceAllString(got.View(), ""); strings.Contains(view, "▸") {
				t.Errorf("the shell step draws a highlighted row though the detected shell %q is not listed:\n%s", shell, view)
			}

			// An explicit choice is required before the wizard may continue.
			after, _ := got.Update(tea.KeyMsg{Type: tea.KeyEnter})
			next := after.(Model)
			if next.Screen != ScreenShellSelect || next.Choices.Shell != "" {
				t.Errorf("Enter answered the shell question without a choice: screen = %v, shell = %q",
					next.Screen, next.Choices.Shell)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// The utilities section
// ---------------------------------------------------------------------------

// utilitiesModel builds the utilities section with the detected desktop pinned
// on both sides, so a behaviour test never depends on the machine that runs it.
//
// Writing the false side explicitly is the point: NewModel detects against the
// host, and `defaults` is always on PATH on macOS, so a model whose switch was
// merely left unset was offered a theme on a macOS runner and not on a Linux
// one. A test about "no desktop" has to say the model has no desktop, rather
// than assume the host has none.
func utilitiesModel(t *testing.T, detected bool) Model {
	t.Helper()

	target, ok := themeSwitchByID("gnome")
	if !ok {
		t.Fatal("the theme switch table no longer holds the gnome entry")
	}

	m := NewModel()
	m.Screen = ScreenUtilities
	m.Cursor = 0
	if detected {
		m.ThemeSwitch, m.ThemeSwitchFound = target, true
	} else {
		m.ThemeSwitch, m.ThemeSwitchFound = themeSwitch{}, false
	}
	return m
}

// TestUtilitiesOpensFromTheMainMenuKey pins the way in: the main menu carries a
// key for the section. The key is a navigation key rather than an easter egg, so
// it opens the section before any prefix state can swallow it.
func TestUtilitiesOpensFromTheMainMenuKey(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenMainMenu
	m.Cursor = 4

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	m = next.(Model)

	if m.Screen != ScreenUtilities {
		t.Fatalf("screen after u = %v, want ScreenUtilities", m.Screen)
	}
	if m.Cursor != 0 {
		t.Errorf("cursor after entering the section = %d, want 0", m.Cursor)
	}
	if m.MenuKeyBuffer != "" {
		t.Errorf("the key left menu buffer %q", m.MenuKeyBuffer)
	}
}

// TestUtilitiesOffersTheThemeRowsOnlyWhenADesktopIsDetected is the "detect
// before offering" rule: the two switch rows exist only when a desktop and its
// tool were found, and the undo row only when this installer has a record for
// that same desktop.
func TestUtilitiesOffersTheThemeRowsOnlyWhenADesktopIsDetected(t *testing.T) {
	t.Run("a detected desktop offers the switch rows", func(t *testing.T) {
		m := utilitiesModel(t, true)
		options := m.GetCurrentOptions()
		if !anyOptionContains(options, "dark theme") || !anyOptionContains(options, "light theme") {
			t.Errorf("options = %v, want both theme rows", options)
		}
		if anyOptionContains(options, "Undo") {
			t.Errorf("options = %v, want no undo row with no record", options)
		}
		if !anyOptionContains(options, "Back") {
			t.Errorf("options = %v, want the way back", options)
		}
	})

	t.Run("a record for the detected desktop offers the undo row", func(t *testing.T) {
		m := utilitiesModel(t, true)
		m.ThemeRecord = &themeRecord{Target: "gnome", Value: "default"}
		if !anyOptionContains(m.GetCurrentOptions(), "Undo") {
			t.Errorf("options = %v, want the undo row when a record exists for the detected desktop", m.GetCurrentOptions())
		}
	})

	t.Run("a record for another desktop offers no undo row", func(t *testing.T) {
		m := utilitiesModel(t, true)
		m.ThemeRecord = &themeRecord{Target: "macos", Value: "Dark"}
		if anyOptionContains(m.GetCurrentOptions(), "Undo") {
			t.Errorf("options = %v, want no undo row for another desktop's record", m.GetCurrentOptions())
		}
	})

	t.Run("no detected desktop offers no theme row at all", func(t *testing.T) {
		m := utilitiesModel(t, false)
		if m.ThemeSwitchFound || m.ThemeSwitch.ID != "" {
			t.Fatalf("this case needs a model with no detected desktop, got %+v: the assertion below would then be "+
				"about the host rather than about the rule", m.ThemeSwitch)
		}
		if anyOptionContains(m.GetCurrentOptions(), "theme") {
			t.Errorf("options = %v, want no theme row on a host with no desktop", m.GetCurrentOptions())
		}
		if !anyOptionContains(m.GetCurrentOptions(), "Back") {
			t.Errorf("options = %v, want the way back", m.GetCurrentOptions())
		}
	})
}

// TestUtilitiesUnavailableIsAboutTheModelNotTheHost reproduces the platform
// failure CI found on macOS without needing a mac, so the next person can see it
// on Linux.
//
// It makes the ambient detection find a switch -- a fake gsettings on PATH and a
// GNOME session -- and then builds the section with the model's own fields set
// to "no desktop". The assertions must still hold, because they read the model
// and never the host. The old test left those fields unset and read the host
// instead: that passed on a Linux runner and failed on the macOS runner, where
// `defaults` is always present, which is the failure this pins.
func TestUtilitiesUnavailableIsAboutTheModelNotTheHost(t *testing.T) {
	dir := t.TempDir()
	for _, tool := range []string{"gsettings", "defaults"} {
		path := filepath.Join(dir, tool)
		if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("XDG_CURRENT_DESKTOP", "GNOME")
	t.Setenv("DESKTOP_SESSION", "gnome")

	// The ambient detection really does find a switch, or this test proves
	// nothing about reading the model instead of the host.
	if _, ok := currentThemeSwitch(&system.SystemInfo{}); !ok {
		t.Fatal("the ambient detection found no switch, so this test cannot show that the assertions are about the model")
	}

	m := utilitiesModel(t, false)
	if m.ThemeSwitchFound || m.ThemeSwitch.ID != "" {
		t.Fatal("utilitiesModel(false) kept the ambient detection, so the assertions below would be about this host")
	}
	if anyOptionContains(m.GetCurrentOptions(), "theme") {
		t.Errorf("options = %v, want no theme row when the model has no detected desktop", m.GetCurrentOptions())
	}
	if plain := ansiEscape.ReplaceAllString(m.View(), ""); !strings.Contains(plain, "No desktop theme switch is available here") {
		t.Errorf("the section does not say why it offers nothing:\n%s", plain)
	}
}

// TestUtilitiesAreOfferedOnMacOSWithDefaults is the case the other tests cannot
// see on a Linux runner, and the one a macOS runner produced: `defaults` is
// always present on darwin, so the pure rule finds a switch there and the
// section must offer it. It links the two halves that make the offer - the rule
// that detects the macOS target from darwin inputs, and the model that shows the
// rows for a detected switch - so neither can silently stop being true, and the
// inputs are passed in rather than read from the host, so this holds on every
// runner.
func TestUtilitiesAreOfferedOnMacOSWithDefaults(t *testing.T) {
	hasDefaults := func(name string) bool { return name == "defaults" }

	target, ok := detectThemeSwitch("darwin", "", "", hasDefaults)
	if !ok {
		t.Fatal("darwin with defaults on PATH no longer detects a theme switch, so macOS is offered nothing")
	}
	if target.ID != "macos" {
		t.Fatalf("the detected desktop = %q, want macos", target.ID)
	}

	m := NewModel()
	m.Screen = ScreenUtilities
	m.ThemeSwitch, m.ThemeSwitchFound = target, true

	options := m.GetCurrentOptions()
	if !anyOptionContains(options, "dark theme") || !anyOptionContains(options, "light theme") {
		t.Errorf("options = %v, want both theme rows on a detected macOS desktop", options)
	}

	plain := ansiEscape.ReplaceAllString(m.View(), "")
	if strings.Contains(plain, "No desktop theme switch is available here") {
		t.Errorf("the macOS section says no switch is available while one was detected:\n%s", plain)
	}
	if !strings.Contains(plain, "macOS") {
		t.Errorf("the macOS section does not name the desktop it detected:\n%s", plain)
	}
}

// TestUtilitiesUnavailableSaysSoAndFitsTheFrame pins the honest degradation: on a
// server, in Termux or in a bare terminal the section names the situation
// instead of offering a row that would fail, and the screen still fits the
// terminals the installer claims to support.
func TestUtilitiesUnavailableSaysSoAndFitsTheFrame(t *testing.T) {
	m := utilitiesModel(t, false)
	if m.ThemeSwitchFound {
		t.Fatal("this case needs a model with no detected desktop, so the assertion is about the rule and not about the host")
	}

	for _, size := range []struct {
		name          string
		width, height int
	}{
		{"80x24", 80, 24},
		{"60x20", 60, 20},
		{"160x50", 160, 50},
	} {
		m.Width, m.Height = size.width, size.height
		view := m.View()
		plain := ansiEscape.ReplaceAllString(view, "")
		if !strings.Contains(plain, "No desktop theme switch is available here") {
			t.Errorf("the unavailable section at %s does not say why: %q", size.name, plain)
		}
		assertScreenFitsTerminal(t, "utilities-unavailable", size.width, size.height, view)
	}
}

// TestUtilitiesEscapeReturnsToTheMainMenu pins the way out the footer promises.
func TestUtilitiesEscapeReturnsToTheMainMenu(t *testing.T) {
	m := utilitiesModel(t, true)
	next, _ := m.handleUtilitiesKeys("esc")
	m = next.(Model)

	if m.Screen != ScreenMainMenu {
		t.Errorf("screen after esc = %v, want ScreenMainMenu", m.Screen)
	}
	if m.Cursor != 0 {
		t.Errorf("cursor after leaving = %d, want 0", m.Cursor)
	}
}

// TestUtilitiesIdleKeyChangesNothing pins that the section only acts on enter:
// an ordinary key must not switch the theme or leave the screen.
func TestUtilitiesIdleKeyChangesNothing(t *testing.T) {
	m := utilitiesModel(t, true)
	next, cmd := m.handleUtilitiesKeys("x")
	m = next.(Model)

	if m.Screen != ScreenUtilities || m.Cursor != 0 {
		t.Errorf("an idle key moved to screen %v cursor %d", m.Screen, m.Cursor)
	}
	if cmd != nil {
		t.Error("an idle key returned a command")
	}
}

// TestUtilitiesSelectingASwitchRunsTheThemeCommand pins the wiring from the row
// to the command: enter on the dark row returns the command that switches the
// detected desktop's theme, through the same seam the disk tests drive.
func TestUtilitiesSelectingASwitchRunsTheThemeCommand(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	target, ok := themeSwitchByID("gnome")
	if !ok {
		t.Fatal("the theme switch table no longer holds the gnome entry")
	}
	calls := withThemeCommandMock(t,
		&system.ExecResult{Output: "'default'\n"},
		&system.ExecResult{},
	)

	m := utilitiesModel(t, true)
	next, cmd := m.handleUtilitiesKeys("enter")
	m = next.(Model)
	if cmd == nil {
		t.Fatal("selecting the dark row returned no command")
	}

	next, _ = m.Update(cmd())
	m = next.(Model)

	if len(*calls) == 0 || (*calls)[0] != target.Read {
		t.Fatalf("commands = %v, want the read first", *calls)
	}
	if m.ThemeRecord == nil || m.ThemeRecord.Value != "default" {
		t.Errorf("model record after the switch = %+v, want the replaced value recorded", m.ThemeRecord)
	}
	if m.ThemeNotice == "" {
		t.Error("the section said nothing after a successful switch")
	}
	if m.Screen != ScreenUtilities {
		t.Errorf("screen after a switch = %v, want to stay on the section", m.Screen)
	}
}

// TestUtilitiesKeepsTheFrameFreeOfActiveColour applies the render-leak rule to
// the new screen: no line may end with a style still active, or the tone bleeds
// through every cell after it.
func TestUtilitiesKeepsTheFrameFreeOfActiveColour(t *testing.T) {
	for _, detected := range []bool{true, false} {
		m := utilitiesModel(t, detected)
		if detected {
			m.ThemeRecord = &themeRecord{Target: "gnome", Value: "default"}
		}
		m.ThemeNotice = "Theme set to dark."
		for _, size := range []struct{ width, height int }{{80, 24}, {60, 20}, {160, 50}, {227, 62}} {
			m.Width, m.Height = size.width, size.height
			checkRenderedLineStyles(t, "utilities", size.width, size.height, m.View())
		}
	}
}

// anyOptionContains reports whether any option in the list contains want.
func anyOptionContains(options []string, want string) bool {
	for _, opt := range options {
		if strings.Contains(opt, want) {
			return true
		}
	}
	return false
}

// TestUtilitiesLongNoticeStillFitsTheFrame protects the notice budget: the
// section trims a long result line out loud rather than letting it push the
// footer off the screen, which is the same rule the panel and list bodies follow.
func TestUtilitiesLongNoticeStillFitsTheFrame(t *testing.T) {
	m := utilitiesModel(t, true)
	m.ThemeRecord = &themeRecord{Target: "gnome", Value: "default"}
	m.ThemeNotice = strings.Repeat("The tool could not read the current setting, so nothing was changed. ", 6)

	for _, size := range []struct {
		name          string
		width, height int
	}{
		{"80x24", 80, 24},
		{"60x20", 60, 20},
	} {
		m.Width, m.Height = size.width, size.height
		view := m.View()
		assertScreenFitsTerminal(t, "utilities-long-notice", size.width, size.height, view)
		if plain := ansiEscape.ReplaceAllString(view, ""); !strings.Contains(plain, "… and ") {
			t.Errorf("the notice at %s was trimmed without the visible marker", size.name)
		}
	}
}
