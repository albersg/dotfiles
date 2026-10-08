package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/albersg/dotfiles/installer/internal/system"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
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

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
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
			{Path: "/test/backup2"},
		}
		m.SelectedBackup = 1
		m.Cursor = 0

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		newModel := result.(Model)

		if newModel.Screen != ScreenRestoreBackup {
			t.Errorf("Expected ScreenRestoreBackup, got %v", newModel.Screen)
		}
		if newModel.Cursor != 1 {
			t.Errorf("Expected the cursor back on the selected backup 1, got %d", newModel.Cursor)
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
		// Options: Start, Learn, Keymaps, LazyVim, Vim Trainer, Restore, Utilities, Exit
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
		// Options without restore: Start, Learn, Keymaps, LazyVim, Vim Trainer, Utilities, Exit
		// Exit is at index 6
		m.Cursor = 6

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

// TestMainMenuOffersUtilitiesAsAVisibleRow pins the discoverability fix: the
// utilities section is a row on the main menu, not only the `u` shortcut, so a
// user who never reads the key list can still find it. The row sits immediately
// above Exit, and Exit stays last, because quitting lives at the bottom of the
// menu whether or not a section was added above it. Both backup states are
// checked because the restore row moves every index below it.
func TestMainMenuOffersUtilitiesAsAVisibleRow(t *testing.T) {
	for _, withBackups := range []bool{false, true} {
		name := "no backups"
		if withBackups {
			name = "with backups"
		}
		t.Run(name, func(t *testing.T) {
			m := NewModel()
			m.Screen = ScreenMainMenu
			if withBackups {
				m.AvailableBackups = []system.BackupInfo{testBackupInfo()}
			}

			options := m.GetCurrentOptions()
			utilities := -1
			for i, opt := range options {
				if strings.Contains(opt, "Utilities") {
					utilities = i
					break
				}
			}
			if utilities < 0 {
				t.Fatalf("the menu offers no Utilities row, so the section is reachable only by the u key: %v", options)
			}
			if last := options[len(options)-1]; !strings.Contains(last, "Exit") {
				t.Errorf("the menu's last row is %q, want Exit last", last)
			}
			if utilities != len(options)-2 {
				t.Errorf("Utilities is row %d of %d, want the row immediately above Exit", utilities, len(options))
			}

			m.Cursor = utilities
			next, _ := m.handleMainMenuKeys("enter")
			m = next.(Model)
			if m.Screen != ScreenUtilities {
				t.Errorf("selecting the Utilities row landed on %v, want ScreenUtilities", m.Screen)
			}
		})
	}
}

// TestMainMenuUtilitiesRowFitsTheMeasuredFloor pins the row budget the extra
// menu row has to live within. It prints what the main menu spends at the two
// sizes the fit guards measure first -- the 80x24 floor and the 60x20 the trainer
// documents as too small -- so the numbers in the review report can be
// re-derived instead of trusted, and it asserts the same frame the class guard
// asserts.
func TestMainMenuUtilitiesRowFitsTheMeasuredFloor(t *testing.T) {
	hints := []installerHint{hintUp, hintDown, hintSelect, hintQuit}
	for _, size := range []struct {
		name          string
		width, height int
	}{
		{"80x24", 80, 24},
		{"60x20", 60, 20},
	} {
		size := size
		t.Run(size.name, func(t *testing.T) {
			m := NewModel()
			m.Screen = ScreenMainMenu
			m.Width, m.Height = size.width, size.height

			// The frame's own hints, panel hint included: the main menu offers
			// panels, so [Tab] packs the footer to two rows at 60 columns and the
			// body budget is the frame's 14, not the 15 a shorter footer suggests.
			frameHints := m.panelHints(m.panelsFor(), hints)
			budget := installerBodyRows(m.Height, footerRowCount(contentWidth(m), frameHints))
			options := len(m.GetCurrentOptions())
			bodyRows := 3 + options // the title, the question and the blank under it
			if m.greeting() != "" {
				bodyRows++
			}
			view := m.View()
			t.Logf("%s: %d menu rows spend %d body rows of a %d-row budget (%d spare before the panel and creature); frame renders %d of %d rows",
				size.name, options, bodyRows, budget, budget-bodyRows, renderedRowCount(view), size.height)

			assertScreenFitsTerminal(t, "main-menu", size.width, size.height, view)
		})
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

// TestUtilitiesEscapeReturnsToTheMainMenu pins the way out the footer promises
// through the real dispatch: Esc travels the same path a keypress does, so a
// screen missing from handleEscape cannot pass by calling the handler directly.
func TestUtilitiesEscapeReturnsToTheMainMenu(t *testing.T) {
	m := utilitiesModel(t, true)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
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

// TestUtilitiesThemeRowsAreDerivedAndNameExclusions covers the utilities
// section's dotfiles-theme rows: one per complete definition, each naming the
// TestThemePickerRowsAreDerivedAndNameExclusions covers the theme picker's
// rows: one per complete definition, each naming the tools it cannot paint when
// it cannot paint some, and none for a partial theme. The list is derived from
// the definitions, never typed.
func TestThemePickerRowsAreDerivedAndNameExclusions(t *testing.T) {
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}

	m := NewModel()
	m.Screen = ScreenThemePicker
	m.DotfilesThemes = defs

	opts := m.GetCurrentOptions()
	var rows []string
	for _, option := range opts {
		if strings.HasPrefix(option, "Apply the ") {
			rows = append(rows, option)
		}
	}
	if want := offeredThemeIDs(defs); len(rows) != len(want) {
		t.Errorf("the section offers %d theme row(s) %v, the complete themes are %v", len(rows), rows, want)
	}

	for _, def := range defs {
		row := dotfilesThemeRow(def)
		if !def.Complete() {
			if slicesContains(opts, row) {
				t.Errorf("a partial theme %q is offered as a row: %q", def.ID, row)
			}
			continue
		}
		if !slicesContains(opts, row) {
			t.Errorf("the section has no row for the complete theme %q", def.ID)
			continue
		}
		_, uncovered := themeCoverage(def)
		for _, tool := range uncovered {
			if !strings.Contains(row, tool) {
				t.Errorf("the row for %q does not name %q, which it leaves out: %q", def.ID, tool, row)
			}
		}
		if len(uncovered) == 0 && strings.Contains(row, "(not ") {
			t.Errorf("theme %q covers every tool, but its row names exclusions: %q", def.ID, row)
		}
		if !strings.Contains(row, def.Name) {
			t.Errorf("the row for %q does not name the theme: %q", def.ID, row)
		}
	}
}

// TestUtilitiesSeesThemesWithoutACloneWhenRunFromTheRepo is the user's case: the
// installer is launched from inside the repository, before any clone, and
// opening Utilities must offer the way into the theme list, which must show the
// theme rows and their exclusion lists. The definitions live in the checkout the
// run is launched from; a clone created by this run is not a prerequisite for
// seeing them.
func TestUtilitiesSeesThemesWithoutACloneWhenRunFromTheRepo(t *testing.T) {
	root := repoRoot(t)
	t.Chdir(root)
	t.Setenv("DOTFILES_DIR", "")

	m := NewModel()
	m.Screen = ScreenUtilities
	m.RepoDir = "" // no clone this run
	m.DotfilesThemes = nil

	cmd := m.dotfilesThemesCmdIfNeeded()
	if cmd == nil {
		t.Fatal("opening Utilities from inside the repository issued no theme read, so no row can appear")
	}
	next, _ := m.Update(cmd())
	m = next.(Model)

	if !slices.Contains(m.GetCurrentOptions(), utilitiesThemeRow) {
		t.Fatalf("the utilities section shows no %q row without a clone: %v", utilitiesThemeRow, m.GetCurrentOptions())
	}

	// Step into the list the row opens and check its rows are what the user sees,
	// not only what the model holds.
	m.Screen = ScreenThemePicker
	opts := m.GetCurrentOptions()
	rows, naming := 0, 0
	for _, opt := range opts {
		if !strings.HasPrefix(opt, "Apply the ") {
			continue
		}
		rows++
		def, ok := m.dotfilesThemeForRow(opt)
		if !ok {
			t.Errorf("the theme row %q does not resolve to a definition", opt)
			continue
		}
		_, uncovered := themeCoverage(def)
		if len(uncovered) == 0 {
			if strings.Contains(opt, "(not ") {
				t.Errorf("the theme row %q names exclusions for a theme that covers every tool", opt)
			}
			continue
		}
		naming++
		for _, tool := range uncovered {
			if !strings.Contains(opt, tool) {
				t.Errorf("the theme row %q does not name %q, which it leaves out", opt, tool)
			}
		}
	}
	if rows == 0 {
		t.Fatalf("the picker shows no theme row without a clone: %v", opts)
	}

	// Render the picker and look for each row, and for the exclusion list of any
	// theme that has one, in the capture. Nothing here is pinned to a theme that
	// leaves a tool out: when every theme covers every tool - the requirement this
	// library is held to - this asserts the rows are drawn and that no stale
	// exclusion list is, and when a theme does leave a tool out the capture has to
	// show it. A guard that demanded an exclusion list would keep the fix from
	// being able to remove the last one.
	m.Width, m.Height = 200, 60
	plain := ansiEscape.ReplaceAllString(m.View(), "")
	for _, opt := range opts {
		if strings.HasPrefix(opt, "Apply the ") && !strings.Contains(plain, opt) {
			t.Errorf("the rendered picker does not show the theme row %q", opt)
		}
	}
	if naming > 0 && !strings.Contains(plain, "(not ") {
		t.Errorf("the rendered picker shows no exclusion list though %d theme row(s) name one", naming)
	}
	if naming == 0 && strings.Contains(plain, "(not ") {
		t.Error("the rendered picker shows an exclusion list though no theme row names one")
	}
}

// TestUtilitiesOffersTheThemeRowFromAnInstalledCopyOutsideTheRepo is the user's
// case at the screen: the program is launched from a working directory that is
// not a checkout, and no clone exists this run, but an earlier install left the
// definitions in the per-user data directory. Opening Utilities must offer the
// way into the theme list rather than hiding it.
func TestUtilitiesOffersTheThemeRowFromAnInstalledCopyOutsideTheRepo(t *testing.T) {
	root := repoRoot(t)
	dataHome := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataHome)
	t.Setenv("DOTFILES_DIR", "")
	t.Setenv("HOME", t.TempDir())

	// What the clone step does at install time: copy the definitions out of the
	// checkout before the temporary clone is removed.
	if _, _, _, err := installThemeDefinitions(root); err != nil {
		t.Fatalf("install the theme definitions: %v", err)
	}
	t.Chdir(t.TempDir())

	m := NewModel()
	m.Screen = ScreenUtilities
	m.RepoDir = "" // no clone this run
	m.DotfilesThemes = nil

	cmd := m.dotfilesThemesCmdIfNeeded()
	if cmd == nil {
		t.Fatal("opening Utilities outside the repository issued no theme read")
	}
	next, _ := m.Update(cmd())
	m = next.(Model)

	if !slices.Contains(m.GetCurrentOptions(), utilitiesThemeRow) {
		t.Fatalf("the utilities section shows no %q row from an installed copy outside the repository: %v",
			utilitiesThemeRow, m.GetCurrentOptions())
	}
	// The read says which copy it used, so the user can see where the list came
	// from rather than inferring it.
	if want := filepath.Join(dataHome, stateAppDir); m.DotfilesRepoDir != want {
		t.Errorf("the definitions were read from %q, want the installed copy %q", m.DotfilesRepoDir, want)
	}
}

// TestUtilitiesSaysTheThemeIsUnavailableWithoutARepository covers the honest
// state when no candidate holds themes/*.toml: no theme row, and the reason in
// the body.
func TestUtilitiesSaysTheThemeIsUnavailableWithoutARepository(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenUtilities
	m.DotfilesThemes = nil
	// The read has finished and failed: no candidate held themes/*.toml. The
	// failure is written directly because a runner standing inside the checkout
	// would find the repository's own themes/ and the read would succeed.
	m.DotfilesThemesErr = themeDefinitionsNotFoundMessage()
	m.RepoDir = ""

	for _, option := range m.GetCurrentOptions() {
		if strings.HasPrefix(option, "Apply the ") {
			t.Errorf("a theme row is offered before a read: %q", option)
		}
	}
	description := strings.Join(m.utilitiesDescription(), " ")
	if !strings.Contains(description, "not switchable") {
		t.Errorf("the section does not say the theme is unavailable: %q", description)
	}
	if !strings.Contains(description, "theme definitions") {
		t.Errorf("the section does not name the reason: %q", description)
	}

	// Entering the section reads once. The read no longer waits for the clone
	// step: the checkout may be the one the installer was launched from, so an
	// empty RepoDir is no reason to skip the read.
	if cmd := m.dotfilesThemesCmdIfNeeded(); cmd == nil {
		t.Error("entering the section issued no read")
	}
	m.DotfilesThemes = []themeDefinition{{ID: "already"}}
	if cmd := m.dotfilesThemesCmdIfNeeded(); cmd != nil {
		t.Error("entering the section read the definitions twice")
	}
}

// TestUtilitiesSaysItIsReadingTheThemeDefinitionsWhileTheReadRuns pins the
// honest half of the section's opening: the definitions are read when the
// section opens, and until that read answers the section must not claim the
// theme is not switchable. "Not switchable" is a fact only a finished read can
// establish, and saying it early is the lie the section used to draw for the
// instant between the keypress and the answer. The key is driven through the
// real dispatch, so the state this measures is the one a userpress produces.
func TestUtilitiesSaysItIsReadingTheThemeDefinitionsWhileTheReadRuns(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenMainMenu
	m.DotfilesThemes = nil
	m.DotfilesThemesErr = ""

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	m = next.(Model)
	if m.Screen != ScreenUtilities {
		t.Fatalf("u opened %v, want ScreenUtilities", m.Screen)
	}

	body := strings.Join(m.utilitiesDescription(), " ")
	if strings.Contains(body, "not switchable") {
		t.Errorf("the section calls the theme not switchable while it is still reading: %q", body)
	}
	if !strings.Contains(body, "Reading the dotfiles' theme definitions") {
		t.Errorf("the section does not say it is reading the definitions: %q", body)
	}
}

// TestALateBackupRowDoesNotMoveTheRowUnderTheCursor pins the no-move half of the
// rule for the main menu. The backup list is read off the update loop, and when
// it lands the Restore row is inserted above Utilities. The cursor is an index,
// so without holding it on the row it named, Enter on Utilities silently became
// Enter on a row that appeared a moment earlier. The press is driven through the
// real dispatch, so the assertion is about the key the user actually presses.
func TestALateBackupRowDoesNotMoveTheRowUnderTheCursor(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenMainMenu
	m.AvailableBackups = nil

	options := m.GetCurrentOptions()
	utilities := -1
	for i, option := range options {
		if strings.Contains(option, "Utilities") {
			utilities = i
			break
		}
	}
	if utilities < 0 {
		t.Fatal("the main menu holds no Utilities row")
	}
	m.Cursor = utilities

	next, _ := m.Update(loadBackupsMsg{backups: []system.BackupInfo{{Path: "/tmp/backup"}}})
	m = next.(Model)

	if got := m.GetCurrentOptions()[m.Cursor]; !strings.Contains(got, "Utilities") {
		t.Fatalf("a backup row that arrived late moved the cursor to %q, want the Utilities row", got)
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.Screen != ScreenUtilities {
		t.Errorf("enter after the backup row arrived opened %v, want ScreenUtilities", m.Screen)
	}

	// The row below the insertion point holds too: with the cursor on Exit, the
	// same late answer must not turn the quit key into a restore.
	exitModel := NewModel()
	exitModel.Screen = ScreenMainMenu
	exitModel.Cursor = len(exitModel.GetCurrentOptions()) - 1
	next, _ = exitModel.Update(loadBackupsMsg{backups: []system.BackupInfo{{Path: "/tmp/backup"}}})
	exitModel = next.(Model)
	if got := exitModel.GetCurrentOptions()[exitModel.Cursor]; !strings.Contains(got, "Exit") {
		t.Errorf("a backup row that arrived late moved the cursor to %q, want the Exit row", got)
	}
	if _, cmd := exitModel.Update(tea.KeyMsg{Type: tea.KeyEnter}); cmd == nil {
		t.Error("enter on Exit after the backup row arrived did not quit")
	}
}

// TestThemeRowNamesAToolTheThemeCannotPaint keeps the exclusion-row rendering
// covered after the coverage lists grew to their final shape: all seven themes in
// themes/ now paint all twelve tools, so no offered definition reaches
// dotfilesThemeRow with an uncovered tool and the exclusion branch ("(not ...)")
// stopped being exercised by real data. Without a guard on that branch it would
// go dark silently, so this test fabricates the missing state from a real
// definition whose id has no artifact row: themeCoverage then reports every
// tool, which is the state the row must name. A definition that cannot paint a
// tool must still have that tool named in the row the picker draws, so the
// honest half of a switch does not go dark just because the lists shrank.
func TestThemeRowNamesAToolTheThemeCannotPaint(t *testing.T) {
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	def, ok := themeByID(defs, "dotfiles")
	if !ok {
		t.Fatal("the dotfiles definition is missing")
	}
	// A synthetic id with no artifact row makes every tool unavailable while the
	// palette stays complete, so the row has an exclusion list to name.
	def.ID = "synthetic"
	if !def.Complete() {
		t.Fatal("the fixture is not complete, so the picker would not draw its row")
	}
	_, uncovered := themeCoverage(def)
	if len(uncovered) == 0 {
		t.Fatal("the fixture leaves out no tool, so this guard proves nothing")
	}
	row := dotfilesThemeRow(def)
	for _, tool := range uncovered {
		if !strings.Contains(row, tool) {
			t.Errorf("the row %q does not name %q, which the theme cannot paint", row, tool)
		}
	}
	if !strings.Contains(row, "(not ") {
		t.Errorf("the row %q does not open an exclusion list", row)
	}
}

// TestTheThemeRowMapsBackToItsDefinition covers the handler's lookup: the row
// labels are what the handler switches on, so a label that does not map back to
// a definition would be a row that does nothing.
func TestTheThemeRowMapsBackToItsDefinition(t *testing.T) {
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	m := Model{DotfilesThemes: defs}

	for _, id := range offeredThemeIDs(defs) {
		def, ok := themeByID(defs, id)
		if !ok {
			t.Fatalf("the offered theme %q has no definition", id)
		}
		got, ok := m.dotfilesThemeForRow(dotfilesThemeRow(def))
		if !ok || got.ID != def.ID {
			t.Errorf("the row for %q maps to %q, ok=%v", def.ID, got.ID, ok)
		}
	}
	if _, ok := m.dotfilesThemeForRow("Apply the nonexistent theme"); ok {
		t.Error("an unknown row mapped to a definition")
	}
}

// slicesContains is a local helper so this file needs no extra import.
func slicesContains(haystack []string, needle string) bool {
	for _, item := range haystack {
		if item == needle {
			return true
		}
	}
	return false
}

// TestTheThemePreviewShowsTheValuesTheApplyWouldWrite covers the live preview:
// while the cursor is on a theme row the section paints that theme's own values,
// and every value it paints is a value the apply block writes, so the preview
// cannot show a colour the switch would not.
func TestTheThemePreviewShowsTheValuesTheApplyWouldWrite(t *testing.T) {
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}

	m := NewModel()
	m.Screen = ScreenThemePicker
	m.DotfilesThemes = defs
	m.Width, m.Height = 120, 40

	idx := -1
	for i, option := range m.GetCurrentOptions() {
		if strings.HasPrefix(option, "Apply the ") {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatal("the picker offers no theme row, so the preview cannot be checked")
	}
	m.Cursor = idx

	def, ok := m.previewThemeDef()
	if !ok {
		t.Fatal("the cursor on a theme row selected no preview theme")
	}

	// The preview paints a real background per swatch, so the guard reads the
	// sequences back. lipgloss converts an 8-bit channel through 16 bits, which
	// can round a channel by one, so the comparison allows one step and no more:
	// a preview holding a different palette would be off by far more. Truecolour
	// is forced because a test renderer has no colour profile by default.
	previousProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previousProfile) })

	lines := m.themePreviewLines(120)
	if len(lines) == 0 {
		t.Fatal("the section rendered no preview for the theme under the cursor")
	}
	swatchRE := regexp.MustCompile(`\x1b\[48;2;(\d+);(\d+);(\d+)m`)
	matches := swatchRE.FindAllStringSubmatch(lines[0], -1)

	var roles []string
	for _, role := range themePreviewSwatchRoles {
		if def.Palette[role] != "" {
			roles = append(roles, role)
		}
	}
	if len(matches) != len(roles) {
		t.Fatalf("the preview paints %d swatch(es), the theme has %d roles to paint: %q", len(matches), len(roles), lines[0])
	}
	for i, role := range roles {
		want := def.Palette[role]
		r, _ := strconv.ParseInt(want[1:3], 16, 0)
		g, _ := strconv.ParseInt(want[3:5], 16, 0)
		b, _ := strconv.ParseInt(want[5:7], 16, 0)
		for channel, pair := range map[string][2]int{"r": {int(r), atoiOrZero(matches[i][1])}, "g": {int(g), atoiOrZero(matches[i][2])}, "b": {int(b), atoiOrZero(matches[i][3])}} {
			if diff := pair[0] - pair[1]; diff > 1 || diff < -1 {
				t.Errorf("the preview swatch for %s (%s) has %s %d, want %d", role, want, channel, pair[1], pair[0])
			}
		}
	}
	painted := len(matches)

	block, err := renderAlacrittyTheme(def)
	if err != nil {
		t.Fatalf("render the apply block: %v", err)
	}
	for _, role := range themePreviewSwatchRoles {
		hex := def.Palette[role]
		if hex == "" {
			continue
		}
		if !strings.Contains(block, hex) {
			t.Errorf("the apply would not write %s = %s, which the preview painted", role, hex)
		}
	}
	t.Logf("the preview paints %d roles of %q, and the apply writes every one of them", painted, def.ID)

	// With the cursor off the theme rows there is no preview: the section must not
	// paint a theme the cursor is not on.
	for i, option := range m.GetCurrentOptions() {
		if strings.HasPrefix(option, "Apply the ") {
			continue
		}
		m.Cursor = i
		if _, ok := m.previewThemeDef(); ok {
			t.Errorf("the row %q selected a preview theme", option)
		}
	}
}

// atoiOrZero parses a decimal channel, treating a malformed one as zero so the
// comparison reports a difference rather than panicking.
func atoiOrZero(value string) int {
	n, err := strconv.Atoi(value)
	if err != nil {
		return -1
	}
	return n
}

// TestThePreviewRepaintsTheWholeInterface covers piece D: with the cursor on a
// theme row the whole chrome takes that theme's colours, and the default chrome
// is back before View returns.
func TestThePreviewRepaintsTheWholeInterface(t *testing.T) {
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	catppuccin, ok := themeByID(defs, "catppuccin-mocha")
	if !ok {
		t.Fatal("the catppuccin-mocha definition is missing")
	}

	previousProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previousProfile) })

	// The SGR parameters lipgloss emits for the theme's blue, read from a probe
	// rather than computed from the hex: lipgloss converts an 8-bit channel through
	// 16 bits, which can shift a channel by one. The parameters are matched rather
	// than the whole sequence because a bold style prefixes them with "1;".
	probe := lipgloss.NewStyle().Foreground(lipgloss.Color(catppuccin.Palette["blue"])).Render("x")
	sequence := strings.TrimSuffix(strings.TrimPrefix(probe[:strings.Index(probe, "m")+1], "\x1b["), "m")

	m := NewModel()
	m.Screen = ScreenThemePicker
	m.DotfilesThemes = defs
	m.DotfilesThemeRecord = &dotfilesThemeRecord{Theme: "dotfiles"}
	m.Width, m.Height = 120, 40
	// With a record there is an undo row, so the cursor can sit on the way back
	// and no preview is active.
	m.Cursor = len(m.GetCurrentOptions()) - 1

	plain := m.View()
	if strings.Contains(plain, sequence) {
		t.Fatal("the default chrome already carries the theme's blue, so this guard proves nothing")
	}

	idx := -1
	for i, option := range m.GetCurrentOptions() {
		if strings.HasPrefix(option, "Apply the Catppuccin Mocha") {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatal("the picker offers no catppuccin row")
	}
	m.Cursor = idx
	previewed := m.View()

	if !strings.Contains(previewed, sequence) {
		t.Error("the previewed interface is not painted in the theme's blue")
	}
	if previewed == plain {
		t.Error("the preview did not repaint the interface")
	}

	// The chrome is restored before View returns: the next render off the theme
	// row is byte-for-byte the one from before the preview.
	if got := Primary.Dark; got != defaultUIColors().Primary.Dark {
		t.Errorf("the preview left Primary at %q, want %q", got, defaultUIColors().Primary.Dark)
	}
	m.Cursor = len(m.GetCurrentOptions()) - 1
	if m.View() != plain {
		t.Error("the preview changed the default chrome")
	}
}

// TestThePreviewWritesNothing covers the first condition: the preview paints and
// records nothing.
func TestThePreviewWritesNothing(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	m := NewModel()
	m.Screen = ScreenThemePicker
	m.DotfilesThemes = defs
	m.Width, m.Height = 120, 40
	found := false
	for i, option := range m.GetCurrentOptions() {
		if strings.HasPrefix(option, "Apply the ") {
			m.Cursor = i
			found = true
			break
		}
	}
	if !found {
		t.Fatal("the picker offers no theme row, so the preview cannot be exercised")
	}
	m.View()

	if rec := readDotfilesThemeRecord(); rec != nil {
		t.Errorf("the preview wrote a record: %+v", rec)
	}
	entries, err := os.ReadDir(stateHome)
	if err != nil {
		t.Fatalf("read the state directory: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("the preview wrote into the state directory: %v", entries)
	}
}

// TestTheDefaultStylesMatchTheDotfilesDefinition pins styles.go's defaults
// against themes/dotfiles.toml, so the chrome is a consumer of the definition
// and not a second hand-written copy of the palette.
func TestTheDefaultStylesMatchTheDotfilesDefinition(t *testing.T) {
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	def, ok := themeByID(defs, "dotfiles")
	if !ok {
		t.Fatal("the dotfiles definition is missing")
	}

	colors := defaultUIColors()
	pairs := []struct {
		name string
		got  lipgloss.AdaptiveColor
		want string
	}{
		{"Background", colors.Background, def.Palette["base"]},
		{"Text", colors.Text, def.Palette["text"]},
		{"TextMuted", colors.TextMuted, def.Prompt["subtext0"]},
		{"Primary", colors.Primary, def.Palette["blue"]},
		{"Secondary", colors.Secondary, def.Prompt["mauve"]},
		{"Accent", colors.Accent, def.Palette["cursor"]},
		{"Error", colors.Error, def.Palette["red"]},
		{"Warning", colors.Warning, def.Prompt["peach"]},
		{"Success", colors.Success, def.Palette["green"]},
		{"Info", colors.Info, def.Palette["blue"]},
		{"BorderActive", colors.BorderActive, def.Palette["blue"]},
		{"SyntaxKeyword", colors.SyntaxKeyword, def.Syntax["keyword_dark"]},
		{"SyntaxString", colors.SyntaxString, def.Syntax["string_dark"]},
	}
	for _, pair := range pairs {
		if pair.want == "" {
			t.Errorf("themes/dotfiles.toml no longer holds the role behind %s", pair.name)
			continue
		}
		if !strings.EqualFold(pair.got.Dark, pair.want) {
			t.Errorf("the default %s is %s, themes/dotfiles.toml says %s: styles.go has its own copy",
				pair.name, pair.got.Dark, pair.want)
		}
	}
	// The syntax tints are adaptive, so their light members are pinned too: the
	// whole pair is a theme role now, not just the dark one.
	lightPairs := []struct {
		name string
		got  string
		want string
	}{
		{"SyntaxKeyword (light)", colors.SyntaxKeyword.Light, def.Syntax["keyword_light"]},
		{"SyntaxString (light)", colors.SyntaxString.Light, def.Syntax["string_light"]},
	}
	for _, pair := range lightPairs {
		if pair.want == "" {
			t.Errorf("themes/dotfiles.toml no longer holds %s", pair.name)
			continue
		}
		if !strings.EqualFold(pair.got, pair.want) {
			t.Errorf("the default %s is %s, themes/dotfiles.toml says %s: styles.go has its own copy",
				pair.name, pair.got, pair.want)
		}
	}
}

// TestThePreviewPaintsTheThemesSyntaxRoles covers piece E's proof: the preview
// reads the theme's [syntax] roles, not the nearest ANSI role it used to
// approximate them with.
func TestThePreviewPaintsTheThemesSyntaxRoles(t *testing.T) {
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}

	dotfiles, ok := themeByID(defs, "dotfiles")
	if !ok {
		t.Fatal("the dotfiles definition is missing")
	}
	colors, err := themePreviewColors(dotfiles)
	if err != nil {
		t.Fatalf("derive the preview colours: %v", err)
	}
	if !strings.EqualFold(colors.SyntaxKeyword.Dark, dotfiles.Syntax["keyword_dark"]) {
		t.Errorf("the preview paints keywords %s, the definition says %s", colors.SyntaxKeyword.Dark, dotfiles.Syntax["keyword_dark"])
	}
	if !strings.EqualFold(colors.SyntaxString.Dark, dotfiles.Syntax["string_dark"]) {
		t.Errorf("the preview paints strings %s, the definition says %s", colors.SyntaxString.Dark, dotfiles.Syntax["string_dark"])
	}
	if !strings.EqualFold(colors.SyntaxKeyword.Light, dotfiles.Syntax["keyword_light"]) {
		t.Errorf("the preview paints keywords %s on a light terminal, the definition says %s",
			colors.SyntaxKeyword.Light, dotfiles.Syntax["keyword_light"])
	}
	// The old approximation mapped keywords onto the prompt's mauve; the real
	// role is a different purple, so a preview that still approximated would
	// carry the mauve value here.
	if strings.EqualFold(colors.SyntaxKeyword.Dark, dotfiles.Prompt["mauve"]) {
		t.Errorf("the preview still paints the prompt's mauve (%s) for keywords instead of the definition's role",
			dotfiles.Prompt["mauve"])
	}

	// Catppuccin pairs Mocha with Latte: now that Latte is in the repository the
	// light member is Latte's value and the preview uses it on a light terminal,
	// so the old no-light-member readability limit is gone.
	catppuccin, ok := themeByID(defs, "catppuccin-mocha")
	if !ok {
		t.Fatal("the catppuccin-mocha definition is missing")
	}
	latte, ok := themeByID(defs, "catppuccin-latte")
	if !ok {
		t.Fatal("the catppuccin-latte definition is missing")
	}
	catColors, err := themePreviewColors(catppuccin)
	if err != nil {
		t.Fatalf("derive the catppuccin preview colours: %v", err)
	}
	if catppuccin.Syntax["keyword_light"] == "" {
		t.Error("catppuccin-mocha ships no light keyword, so the pairing with Latte was lost")
	}
	if !strings.EqualFold(catppuccin.Syntax["keyword_light"], latte.Prompt["mauve"]) {
		t.Errorf("catppuccin-mocha's light keyword is %q, Latte's mauve is %q: the flavour pairing was lost",
			catppuccin.Syntax["keyword_light"], latte.Prompt["mauve"])
	}
	if !strings.EqualFold(catppuccin.Syntax["string_light"], latte.Prompt["green"]) {
		t.Errorf("catppuccin-mocha's light string is %q, Latte's green is %q: the flavour pairing was lost",
			catppuccin.Syntax["string_light"], latte.Prompt["green"])
	}
	if !strings.EqualFold(catColors.SyntaxKeyword.Light, catppuccin.Syntax["keyword_light"]) {
		t.Errorf("the preview paints light keywords %s, the definition says %s",
			catColors.SyntaxKeyword.Light, catppuccin.Syntax["keyword_light"])
	}
	if strings.EqualFold(catColors.SyntaxKeyword.Light, catColors.SyntaxKeyword.Dark) {
		t.Errorf("the preview uses the dark keyword (%s) on a light terminal although a light member exists",
			catColors.SyntaxKeyword.Light)
	}
}

// TestAPartialThemeCannotBePreviewed covers the honest edge: a definition with no
// canonical palette cannot paint the interface, so the preview refuses it rather
// than inventing colours. Every shipped theme is complete now, so the refusal is
// also exercised with a built fixture, which keeps the guard's teeth instead of
// letting it pass because the repository happens to hold no partial theme.
func TestAPartialThemeCannotBePreviewed(t *testing.T) {
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	for _, def := range defs {
		if def.Complete() {
			continue
		}
		if _, err := themePreviewColors(def); err == nil {
			t.Errorf("theme %q has no canonical palette yet it previewed", def.ID)
		}
	}

	partial := themeDefinition{
		ID:      "partial-fixture",
		Palette: map[string]string{"base": "#000000", "text": "#ffffff"},
		Prompt:  map[string]string{},
		Syntax:  map[string]string{},
	}
	if _, err := themePreviewColors(partial); err == nil {
		t.Error("a definition with two canonical roles previewed, so the refusal is not enforced")
	}
}

// ---------------------------------------------------------------------------
// The intermediate "Change the dotfiles theme" level
// ---------------------------------------------------------------------------

// changeThemeRow is the utilities row that opens the theme picker. The list of
// themes lives one level in, so the section is a list of jobs rather than a list
// of themes.
const changeThemeRow = "Change the dotfiles theme"

// TestUtilitiesOffersAChangeThemeRowInsteadOfTheThemes pins the new level: the
// utilities section offers one row that opens the theme list, and it does not
// list the themes itself any more.
func TestUtilitiesOffersAChangeThemeRowInsteadOfTheThemes(t *testing.T) {
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}

	m := utilitiesModel(t, true)
	m.DotfilesThemes = defs
	m.DotfilesThemeRecord = &dotfilesThemeRecord{Theme: "dotfiles"}

	var changeRows, applyRows int
	for _, opt := range m.GetCurrentOptions() {
		if opt == changeThemeRow {
			changeRows++
		}
		if strings.HasPrefix(opt, "Apply the ") {
			applyRows++
		}
	}
	if changeRows != 1 {
		t.Errorf("the utilities section shows %d %q row(s), want exactly 1: %v", changeRows, changeThemeRow, m.GetCurrentOptions())
	}
	if applyRows != 0 {
		t.Errorf("the utilities section still lists %d theme row(s); the list belongs one level in: %v", applyRows, m.GetCurrentOptions())
	}
}

// TestThemePickerListsTheDerivedThemesAndUndo pins what the new level holds: the
// complete themes derived from the definitions, the undo row when a change is
// recorded, and the way back. A partial theme is never offered.
func TestThemePickerListsTheDerivedThemesAndUndo(t *testing.T) {
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}

	m := NewModel()
	m.Screen = ScreenThemePicker
	m.DotfilesThemes = defs
	m.DotfilesThemeRecord = &dotfilesThemeRecord{Theme: "dotfiles"}

	got := m.GetCurrentOptions()
	want := append([]string{}, m.dotfilesThemeOptions()...)
	want = append(want, m.menuSeparator(), themeRefreshRow, dotfilesThemeUndoRow, m.menuSeparator(), "← Back")
	if !slices.Equal(got, want) {
		t.Errorf("the picker lists %v, want the derived themes, the refresh, the undo and the way back %v", got, want)
	}

	if len(m.dotfilesThemeOptions()) != len(offeredThemeIDs(defs)) {
		t.Errorf("the picker derives %d theme row(s), the complete themes are %v",
			len(m.dotfilesThemeOptions()), offeredThemeIDs(defs))
	}
	for _, def := range defs {
		if !def.Complete() && slices.Contains(got, dotfilesThemeRow(def)) {
			t.Errorf("the picker offers the partial theme %q", def.ID)
		}
	}
}

// TestThemePickerIsOpenedByTheChangeThemeRow covers the step in: choosing the
// utilities row lands on the picker with its own cursor reset.
func TestThemePickerIsOpenedByTheChangeThemeRow(t *testing.T) {
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}

	m := utilitiesModel(t, true)
	m.DotfilesThemes = defs

	idx := -1
	for i, opt := range m.GetCurrentOptions() {
		if opt == changeThemeRow {
			idx = i
		}
	}
	if idx < 0 {
		t.Fatalf("the utilities section offers no %q row: %v", changeThemeRow, m.GetCurrentOptions())
	}
	m.Cursor = idx

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.Screen != ScreenThemePicker {
		t.Fatalf("choosing %q landed on %v, want ScreenThemePicker", changeThemeRow, m.Screen)
	}
	if m.Cursor != 0 {
		t.Errorf("the picker opened at cursor %d, want 0", m.Cursor)
	}
}

// TestThemePickerEscapeReturnsToTheUtilities pins the way out of the new level.
func TestThemePickerEscapeReturnsToTheUtilities(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenThemePicker
	m.Cursor = 1
	m.ThemeNotice = "something"

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if m.Screen != ScreenUtilities {
		t.Errorf("esc left the picker on %v, want ScreenUtilities", m.Screen)
	}
	if m.Cursor != 0 {
		t.Errorf("cursor after leaving the picker = %d, want 0", m.Cursor)
	}
	if m.ThemeNotice != "" {
		t.Errorf("the notice survived leaving the picker: %q", m.ThemeNotice)
	}
}

// TestThemePickerAppliesTheThemeUnderTheCursor covers the switch the picker
// calls: enter on a theme row returns the same apply command the utilities
// section used, and the result stays on the picker as a notice. The dry-run gate
// is set so the guard exercises the wiring without writing to the user's files.
func TestThemePickerAppliesTheThemeUnderTheCursor(t *testing.T) {
	t.Setenv("DOTFILES_DRY_RUN", "1")
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}

	m := NewModel()
	m.Screen = ScreenThemePicker
	m.DotfilesThemes = defs

	idx := -1
	for i, opt := range m.GetCurrentOptions() {
		if strings.HasPrefix(opt, "Apply the ") {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatalf("the picker offers no theme row: %v", m.GetCurrentOptions())
	}
	m.Cursor = idx

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("choosing a theme returned no command, so the row is not wired to the switch")
	}
	// The press answers at once, in the row the result will use.
	if !m.themeActivityPending() {
		t.Fatal("the press left no pending line, so the wait before the switch is silent")
	}
	next, _ = m.Update(cmd())
	m = next.(Model)
	if m.Screen != ScreenThemePicker {
		t.Errorf("applying a theme left the picker on %v", m.Screen)
	}
	if m.ThemeActivity == nil || m.ThemeActivity.Pending != "" {
		t.Fatalf("the switch did not land in the pending slot: %+v", m.ThemeActivity)
	}
	if got := strings.Join(m.ThemeActivity.Result, "\n"); !strings.Contains(got, "DRY RUN") {
		t.Errorf("the result after applying = %q, want the dry-run result of the switch", got)
	}
	if m.ThemeNotice != "" {
		t.Errorf("the outcome also filled the notice, so it has a second home: %q", m.ThemeNotice)
	}
}

// themePickerRowIndex finds a row in the picker's current list, so a guard drives
// the screen through the rows a user sees rather than through an index it typed.
func themePickerRowIndex(t *testing.T, m Model, row string) int {
	t.Helper()
	for i, option := range m.GetCurrentOptions() {
		if option == row {
			return i
		}
	}
	t.Fatalf("the picker offers no %q row: %v", row, m.GetCurrentOptions())
	return -1
}

// TestUndoLeavesTheCursorOnASelectableRow is the undo's half of the one-change
// rule: an undo removes the undo row it was pressed on, so the index that row
// had falls on a separator. The cursor must be settled onto a row the frame can
// still draw, on the same list, rather than left on a rule with no marker -- the
// focus is where the user can see it and where the next arrow key explains
// itself. The switch itself is not a navigation here either.
func TestUndoLeavesTheCursorOnASelectableRow(t *testing.T) {
	t.Setenv("DOTFILES_DRY_RUN", "0")

	m := NewModel()
	isolateGoldenTest(t, &m)
	home := os.Getenv("HOME")
	installThemeFiles(t, home, "alacritty")
	t.Setenv("PATH", t.TempDir())

	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	def, ok := themeByID(defs, defaultThemeID)
	if !ok {
		t.Fatalf("%s is not defined", defaultThemeID)
	}

	m.Screen = ScreenThemePicker
	m.DotfilesThemes = defs

	// Apply once through the row, so a record exists and the undo row appears.
	m.Cursor = themePickerRowIndex(t, m, dotfilesThemeRow(def))
	model, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(Model)
	model, _ = m.Update(cmd())
	m = model.(Model)
	if m.DotfilesThemeRecord == nil {
		t.Fatal("the apply recorded nothing, so there is no undo row to press")
	}

	// Press the undo row.
	m.Cursor = themePickerRowIndex(t, m, dotfilesThemeUndoRow)
	model, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(Model)
	if cmd == nil {
		t.Fatal("pressing the undo row returned no command")
	}
	if !m.themeActivityPending() {
		t.Fatal("the undo press left no pending line")
	}
	model, _ = m.Update(cmd())
	m = model.(Model)

	options := m.GetCurrentOptions()
	if m.Cursor < 0 || m.Cursor >= len(options) {
		t.Fatalf("the cursor %d is outside the list of %d rows", m.Cursor, len(options))
	}
	if strings.HasPrefix(options[m.Cursor], menuSeparatorPrefix) {
		t.Errorf("after an undo the cursor is on a separator, which the frame draws without a marker: %q", options[m.Cursor])
	}
	if m.Screen != ScreenThemePicker {
		t.Errorf("the undo left the picker on %v", m.Screen)
	}
	if markerRow(pickerFrameLines(m), "▸") < 0 {
		t.Errorf("the frame after an undo draws no cursor marker:\n%s", strings.Join(pickerFrameLines(m), "\n"))
	}
	if m.ThemeRefreshReview {
		t.Error("the undo opened the refresh review, which is the panel the user was looking at")
	}
}

// ---------------------------------------------------------------------------
// Refreshing the managed files that predate the markers
// ---------------------------------------------------------------------------

// pickerWithRefreshFixture parks the picker on the refresh row with one old
// managed file in a temporary home, so the detection and confirmation flow can
// be driven the way a user drives it.
func pickerWithRefreshFixture(t *testing.T) (Model, string, string) {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("DOTFILES_DRY_RUN", "0")

	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	def, ok := themeByID(defs, defaultThemeID)
	if !ok {
		t.Fatalf("%s is not defined", defaultThemeID)
	}

	art := artifactByName(t, "alacritty")
	dst := themeInstalledPath(art, home)
	old := legacyThemeFile(t, art, def)
	writeThemeFileAt(t, dst, old)

	m := NewModel()
	m.Screen = ScreenThemePicker
	m.DotfilesThemes = defs
	for i, option := range m.GetCurrentOptions() {
		if option == themeRefreshRow {
			m.Cursor = i
			break
		}
	}
	return m, dst, old
}

// TestThemePickerOffersTheRefreshRow covers the way in: the list the switch
// lives in also offers the refresh, so a machine that predates the markers has
// a route to the feature without reinstalling.
func TestThemePickerOffersTheRefreshRow(t *testing.T) {
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}

	m := NewModel()
	m.Screen = ScreenThemePicker
	m.DotfilesThemes = defs
	if !slices.Contains(m.GetCurrentOptions(), themeRefreshRow) {
		t.Errorf("the picker offers no %q row: %v", themeRefreshRow, m.GetCurrentOptions())
	}
}

// TestThemeRefreshNamesTheFilesBeforeWriting covers the promise that nothing is
// refreshed by surprise: detection opens a review that names the files and the
// preserved destinations, and the file is untouched until the confirmation.
func TestThemeRefreshNamesTheFilesBeforeWriting(t *testing.T) {
	m, dst, old := pickerWithRefreshFixture(t)

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("choosing the refresh row returned no detection command")
	}
	next, _ = m.Update(cmd())
	m = next.(Model)

	if !m.ThemeRefreshReview || m.ThemeRefreshDone {
		t.Fatalf("detection left the review in review=%v done=%v, want the confirmation", m.ThemeRefreshReview, m.ThemeRefreshDone)
	}
	if got, _ := os.ReadFile(dst); string(got) != old {
		t.Error("detection wrote the file before the user confirmed")
	}
	plain := ansiEscape.ReplaceAllString(strings.Join(m.themePickerDescription(), "\n"), "")
	if !strings.Contains(plain, dst) {
		t.Errorf("the review does not name the file it would touch (%s):\n%s", dst, plain)
	}
	if !strings.Contains(plain, ".bak-dotfiles-") {
		t.Errorf("the review does not say where the unowned file is preserved:\n%s", plain)
	}

	// Confirming writes the file.
	m.Cursor = 0
	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("confirming the refresh returned no command")
	}
	next, _ = m.Update(cmd())
	m = next.(Model)
	if !m.ThemeRefreshDone {
		t.Error("the review did not move to its result after the refresh")
	}
	if got, _ := os.ReadFile(dst); !strings.Contains(string(got), themeOwnershipMarker) {
		t.Error("confirming did not refresh the file")
	}
}

// TestThemeRefreshCancelLeavesTheFilesAlone covers the safe default: Cancel
// leaves the review and writes nothing.
func TestThemeRefreshCancelLeavesTheFilesAlone(t *testing.T) {
	m, dst, old := pickerWithRefreshFixture(t)

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	if !m.ThemeRefreshReview {
		t.Fatal("detection did not open the review, so there is nothing to cancel")
	}

	m.Cursor = 1 // Cancel is the second row and the safe default.
	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if cmd != nil {
		t.Error("Cancel returned a command, so it is not a no-op")
	}
	if m.ThemeRefreshReview {
		t.Error("Cancel stayed in the review")
	}
	if got, _ := os.ReadFile(dst); string(got) != old {
		t.Error("Cancel wrote the file")
	}
}

// TestThemeRefreshEscapeCancelsTheReview covers the review's own Esc through the
// real dispatch: the footer says "back" and the review's handler cancels it, so
// Esc leaves the review on the picker list rather than stepping past the picker
// to the utilities section.
func TestThemeRefreshEscapeCancelsTheReview(t *testing.T) {
	m, dst, old := pickerWithRefreshFixture(t)

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	if !m.ThemeRefreshReview {
		t.Fatal("detection did not open the review, so there is nothing to cancel")
	}

	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if cmd != nil {
		t.Error("Esc in the review returned a command, want none")
	}
	if m.ThemeRefreshReview {
		t.Error("Esc stayed in the review")
	}
	if m.Screen != ScreenThemePicker {
		t.Errorf("Esc left the picker for %v, want the picker list", m.Screen)
	}
	if got, _ := os.ReadFile(dst); string(got) != old {
		t.Error("Esc in the review wrote the file")
	}
}
