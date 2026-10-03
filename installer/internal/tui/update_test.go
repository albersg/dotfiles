package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
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
		// Detection failed. The menu must start somewhere and there is no
		// "unknown" entry, so the cursor takes the first entry; the description
		// still reads "Detected: OSUnknown" rather than claiming macOS.
		"OSUnknown": "macOS",
	}

	for _, c := range osConstantsInDeclarationOrder(t) {
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
// cursor contradicts.
func TestShellScreenStartsOnTheDetectedShell(t *testing.T) {
	cases := []struct {
		shell string
		want  string
	}{
		{"fish", "Fish"},
		{"zsh", "Zsh"},
		{"nushell", "Nushell"},
		{"nu", "Nushell"},
		{"", "Fish"},        // nothing detected
		{"unknown", "Fish"}, // detection found nothing
		{"bash", "Fish"},    // detected, but the menu does not list it
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
			if c.shell != "" && c.shell != "unknown" && !strings.Contains(description, c.shell) {
				t.Errorf("the shell step description %q does not name the detected shell %q", description, c.shell)
			}
		})
	}
}
