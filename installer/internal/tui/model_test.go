package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// The font presence probe runs the font step's own check, and on Linux that
// check shells out to fc-list. This package builds a model in hundreds of tests,
// and none of them should pay for that process: the stub answers "nothing
// measured", and for a host the step does not check that is also the true answer,
// so the screens keep their unmeasured sentence. Tests that need a machine with
// the font, or without one, replace the probe and restore it. The guard below
// fails if this init is removed.
func init() {
	fontPresenceProbe = func(info *system.SystemInfo) fontPresence {
		return fontPresence{}
	}
	fontPresenceStubbed = true
}

// fontPresenceStubbed records that the init above replaced the probe, so a guard
// can fail if it is ever removed.
var fontPresenceStubbed bool

func TestNewModel(t *testing.T) {
	t.Run("should create model with initial state", func(t *testing.T) {
		m := NewModel()

		if m.Screen != ScreenWelcome {
			t.Errorf("Expected initial screen to be ScreenWelcome, got %v", m.Screen)
		}

		if m.Width != 80 {
			t.Errorf("Expected initial width 80, got %d", m.Width)
		}

		if m.Height != 24 {
			t.Errorf("Expected initial height 24, got %d", m.Height)
		}

		if m.SystemInfo == nil {
			t.Error("SystemInfo should not be nil")
		}

		if m.Cursor != 0 {
			t.Errorf("Expected cursor at 0, got %d", m.Cursor)
		}

		if m.ShowDetails {
			t.Error("ShowDetails should be false initially")
		}

		if m.Quitting {
			t.Error("Quitting should be false initially")
		}
	})
}

func TestGetCurrentOptions(t *testing.T) {
	m := NewModel()

	t.Run("should return OS options for OSSelect screen", func(t *testing.T) {
		m.Screen = ScreenOSSelect
		opts := m.GetCurrentOptions()

		if len(opts) != 3 {
			t.Errorf("Expected 3 OS options (macOS, Linux, Termux), got %d", len(opts))
		}
		// Verify options contain expected OS names
		hasMac := strings.Contains(opts[0], "macOS")
		hasLinux := strings.Contains(opts[1], "Linux")
		hasTermux := strings.Contains(opts[2], "Termux")
		if !hasMac || !hasLinux || !hasTermux {
			t.Errorf("Unexpected OS options: %v", opts)
		}
	})

	t.Run("should return terminal options for mac", func(t *testing.T) {
		m.Screen = ScreenTerminalSelect
		m.Choices.OS = "mac"
		opts := m.GetCurrentOptions()

		// Should have: Alacritty, WezTerm, Kitty, Ghostty, None, separator, Learn
		if len(opts) != 7 {
			t.Errorf("Expected 7 terminal options for mac (including separator and learn), got %d", len(opts))
		}
		// Should include Kitty on mac
		hasKitty := false
		for _, opt := range opts {
			if opt == "Kitty" {
				hasKitty = true
				break
			}
		}
		if !hasKitty {
			t.Error("Mac should have Kitty option")
		}
	})

	t.Run("should return terminal options for linux without kitty", func(t *testing.T) {
		m.Screen = ScreenTerminalSelect
		m.Choices.OS = "linux"
		opts := m.GetCurrentOptions()

		// Should have: Alacritty, WezTerm, Ghostty, None, separator, Learn
		if len(opts) != 6 {
			t.Errorf("Expected 6 terminal options for linux (including separator and learn), got %d", len(opts))
		}
		// Should NOT include Kitty on linux
		for _, opt := range opts {
			if opt == "Kitty" {
				t.Error("Linux should not have Kitty option")
			}
		}
	})

	t.Run("should return shell options", func(t *testing.T) {
		m.Screen = ScreenShellSelect
		opts := m.GetCurrentOptions()

		// Should have: Fish, Zsh, Nushell, separator, Learn
		if len(opts) != 5 {
			t.Errorf("Expected 5 shell options (including separator and learn), got %d", len(opts))
		}
		expected := []string{"Fish", "Zsh", "Nushell"}
		for i, exp := range expected {
			if opts[i] != exp {
				t.Errorf("Expected %s at position %d, got %s", exp, i, opts[i])
			}
		}
	})

	t.Run("should return WM options", func(t *testing.T) {
		m.Screen = ScreenWMSelect
		opts := m.GetCurrentOptions()

		// Should have: Tmux, Zellij, Herdr, None, separator, Learn
		if len(opts) != 6 {
			t.Errorf("Expected 6 WM options (including separator and learn), got %d", len(opts))
		}
	})

	t.Run("should return empty for non-selection screens", func(t *testing.T) {
		m.Screen = ScreenInstalling
		opts := m.GetCurrentOptions()

		if len(opts) != 0 {
			t.Errorf("Expected 0 options for installing screen, got %d", len(opts))
		}
	})
}

// TestFontScreenOptionsNameTheirActions is the guard for the reported defect:
// the font step's question asks whether to install the family, so both rows have
// to answer with what they do. The old second row answered with a reason --
// "No, I already have it" -- so a person who did not have the font, or who was
// unsure, could not tell whether choosing it skipped the font, failed the step,
// or continued the install.
func TestFontScreenOptionsNameTheirActions(t *testing.T) {
	m := Model{Screen: ScreenFontSelect, Choices: UserChoices{OS: "linux"}}
	opts := m.GetCurrentOptions()

	want := []string{"Install " + m.installedFontLabel(), "Skip it"}
	if len(opts) != len(want) {
		t.Fatalf("font screen offers %d rows, want %d: %v", len(opts), len(want), opts)
	}
	for i := range want {
		if opts[i] != want[i] {
			t.Errorf("font row %d = %q, want %q", i, opts[i], want[i])
		}
	}
}

// TestFontPresenceProbeIsStubbedInATestBinary is the guard for the init above:
// the probe runs the font step's presence check, and on Linux that check shells
// out to fc-list. If the init goes, building a model in any test runs a process.
func TestFontPresenceProbeIsStubbedInATestBinary(t *testing.T) {
	if !fontPresenceStubbed {
		t.Fatal("the font presence probe is not stubbed in this test binary; building a model would run fc-list")
	}
}

// TestFontPresenceIsMeasuredOnceAndNeverWhileRendering is the guard for the rule
// the design turns on: the font screen's description and rows are computed on
// every render, so they must read the measurement rather than take one. The
// probe is counted, and drawing the screen repeatedly must not move the count.
func TestFontPresenceIsMeasuredOnceAndNeverWhileRendering(t *testing.T) {
	original := fontPresenceProbe
	t.Cleanup(func() { fontPresenceProbe = original })

	calls := 0
	fontPresenceProbe = func(info *system.SystemInfo) fontPresence {
		calls++
		return fontPresence{Measured: true, Installed: true}
	}

	m := NewModel()
	if calls != 1 {
		t.Fatalf("building the model called the probe %d times, want exactly 1", calls)
	}
	m.Screen = ScreenFontSelect
	m.Choices = UserChoices{OS: "linux"}
	m.Width, m.Height = 80, 24
	m.Animating = false

	for i := 0; i < 25; i++ {
		_ = m.GetScreenDescription()
		_ = m.GetCurrentOptions()
		_ = m.View()
	}

	if calls != 1 {
		t.Fatalf("rendering the font screen called the probe %d times, want still 1", calls)
	}
}

// TestFontScreenDescriptionStatesWhatTheCheckFound pins the second half of the
// fix: the screen says what the font step's own check found on this machine, so
// the choice is informed by a measurement rather than the user's memory. A
// platform the step does not check keeps the requirement sentence instead of
// claiming a state nothing measured.
func TestFontScreenDescriptionStatesWhatTheCheckFound(t *testing.T) {
	cases := []struct {
		name     string
		presence fontPresence
		want     string
	}{
		{"already installed", fontPresence{Measured: true, Installed: true}, "is already installed here"},
		{"not installed", fontPresence{Measured: true}, "is not installed here"},
		{"step does not check", fontPresence{}, "is required for icons and glyphs"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := Model{Screen: ScreenFontSelect, Choices: UserChoices{OS: "linux"}, FontPresence: tc.presence}
			got := m.GetScreenDescription()
			if !strings.Contains(got, tc.want) {
				t.Fatalf("font screen description = %q, want it to contain %q", got, tc.want)
			}
		})
	}
}

// TestFontSummaryLineNamesWhatTheRunFound pins the completion row's optional
// provenance: when the pre-run check measured this machine, the row says whether
// the font was already there or was installed by this run. A host the step did
// not check -- and a run that chose the Termux branch on a host the check did not
// measure -- keeps the label alone.
func TestFontSummaryLineNamesWhatTheRunFound(t *testing.T) {
	cases := []struct {
		name     string
		choices  UserChoices
		presence fontPresence
		want     string
	}{
		{"already installed", UserChoices{OS: "linux", InstallFont: true}, fontPresence{Measured: true, Installed: true}, "Font: Meslo Nerd Font (already installed)"},
		{"installed now", UserChoices{OS: "linux", InstallFont: true}, fontPresence{Measured: true}, "Font: Meslo Nerd Font (installed now)"},
		{"nothing measured", UserChoices{OS: "linux", InstallFont: true}, fontPresence{}, "Font: Meslo Nerd Font"},
		{"termux chosen on another host", UserChoices{OS: "termux", InstallFont: true}, fontPresence{Measured: true, Installed: true}, "Font: Meslo Nerd Font"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := Model{Screen: ScreenComplete, Choices: tc.choices, FontPresence: tc.presence, SystemInfo: &system.SystemInfo{OS: system.OSLinux}}
			if got := m.fontSummaryLine(); got != tc.want {
				t.Fatalf("font summary = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestFontPresenceForHostUsesTheStepsOwnCheck pins the dispatch the probe makes:
// the Termux branch reads the slot file the step itself reads, and macOS -- whose
// step installs through the cask and never reads the font store -- answers "not
// measured" rather than claiming the family is absent. The Linux branch is the
// step's own fc-list call, covered where the step is.
func TestFontPresenceForHostUsesTheStepsOwnCheck(t *testing.T) {
	home := t.TempDir()
	termux := &system.SystemInfo{IsTermux: true, HomeDir: home}

	if got := fontPresenceForHost(termux); !got.Measured || got.Installed {
		t.Fatalf("empty Termux slot = %+v, want measured and not installed", got)
	}

	slot := filepath.Join(home, termuxFontDirName, termuxFontFileName)
	if err := os.MkdirAll(filepath.Dir(slot), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(slot, []byte("font-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := fontPresenceForHost(termux); !got.Measured || !got.Installed {
		t.Fatalf("filled Termux slot = %+v, want measured and installed", got)
	}

	if got := fontPresenceForHost(&system.SystemInfo{OS: system.OSMac, HomeDir: home}); got.Measured {
		t.Fatalf("macOS presence = %+v, want not measured: the step never checks the font store", got)
	}
	if got := fontPresenceForHost(nil); got.Measured {
		t.Fatalf("nil host presence = %+v, want not measured", got)
	}
}

func TestGetScreenTitle(t *testing.T) {
	m := NewModel()

	tests := []struct {
		screen   Screen
		expected string
	}{
		{ScreenWelcome, "Welcome to dotfiles Installer"},
		{ScreenOSSelect, "Step 1: Select Your Operating System"},
		{ScreenTerminalSelect, "Step 2: Choose Terminal Emulator"},
		{ScreenInstallReview, "Choose What Gets Installed"},
		{ScreenShellSelect, "Step 4: Choose Your Shell"},
		{ScreenComplete, "Installation complete"},
		{ScreenError, "Installation failed"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			m.Screen = tt.screen
			title := m.GetScreenTitle()
			if title != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, title)
			}
		})
	}
}

func TestSetupInstallSteps(t *testing.T) {
	t.Run("should setup minimal steps", func(t *testing.T) {
		m := NewModel()
		m.SystemInfo = &system.SystemInfo{
			OS:       system.OSMac,
			HasBrew:  true,
			HasXcode: true,
		}
		m.Choices = UserChoices{
			OS:          "mac",
			Terminal:    "none",
			InstallFont: false,
			Shell:       "fish",
			WindowMgr:   "none",
			InstallNvim: false,
		}

		m.SetupInstallSteps()

		// Should have: clone, shell, cleanup
		if len(m.Steps) < 3 {
			t.Errorf("Expected at least 3 steps, got %d", len(m.Steps))
		}

		// First step should be clone
		if m.Steps[0].ID != "clone" {
			t.Errorf("First step should be 'clone', got '%s'", m.Steps[0].ID)
		}

		// Last step should be cleanup
		lastStep := m.Steps[len(m.Steps)-1]
		if lastStep.ID != "cleanup" {
			t.Errorf("Last step should be 'cleanup', got '%s'", lastStep.ID)
		}
	})

	t.Run("should include homebrew step when not installed", func(t *testing.T) {
		m := NewModel()
		m.SystemInfo = &system.SystemInfo{
			OS:       system.OSMac,
			HasBrew:  false, // Not installed
			HasXcode: true,
		}
		m.Choices = UserChoices{
			OS:          "mac",
			Terminal:    "none",
			Shell:       "fish",
			WindowMgr:   "none",
			InstallNvim: false,
		}

		m.SetupInstallSteps()

		hasBrewStep := false
		for _, step := range m.Steps {
			if step.ID == "homebrew" {
				hasBrewStep = true
				break
			}
		}

		if !hasBrewStep {
			t.Error("Should include homebrew step when brew is not installed")
		}
	})

	t.Run("should include terminal step when selected", func(t *testing.T) {
		m := NewModel()
		m.SystemInfo = &system.SystemInfo{
			OS:       system.OSMac,
			HasBrew:  true,
			HasXcode: true,
		}
		m.Choices = UserChoices{
			OS:          "mac",
			Terminal:    "ghostty",
			Shell:       "fish",
			WindowMgr:   "none",
			InstallNvim: false,
		}

		m.SetupInstallSteps()

		hasTerminalStep := false
		for _, step := range m.Steps {
			if step.ID == "terminal" {
				hasTerminalStep = true
				break
			}
		}

		if !hasTerminalStep {
			t.Error("Should include terminal step when terminal is selected")
		}
	})

	t.Run("should include nvim step when selected", func(t *testing.T) {
		m := NewModel()
		m.SystemInfo = &system.SystemInfo{
			OS:       system.OSMac,
			HasBrew:  true,
			HasXcode: true,
		}
		m.Choices = UserChoices{
			OS:          "mac",
			Terminal:    "none",
			Shell:       "fish",
			WindowMgr:   "none",
			InstallNvim: true,
		}

		m.SetupInstallSteps()

		hasNvimStep := false
		for _, step := range m.Steps {
			if step.ID == "nvim" {
				hasNvimStep = true
				break
			}
		}

		if !hasNvimStep {
			t.Error("Should include nvim step when nvim is selected")
		}
	})

	t.Run("should include all steps for full installation", func(t *testing.T) {
		m := NewModel()
		m.SystemInfo = &system.SystemInfo{
			OS:       system.OSLinux,
			HasBrew:  false,
			HasXcode: false,
		}
		m.Choices = UserChoices{
			OS:          "linux",
			Terminal:    "ghostty",
			InstallFont: true,
			Shell:       "zsh",
			WindowMgr:   "tmux",
			InstallNvim: true,
		}

		m.SetupInstallSteps()

		expectedSteps := []string{"deps", "clone", "homebrew", "terminal", "font", "shell", "wm", "nvim", "toolset", "agentskills", "officecli", "theme", "setshell", "cleanup"}

		if len(m.Steps) != len(expectedSteps) {
			t.Errorf("Expected %d steps, got %d", len(expectedSteps), len(m.Steps))
		}

		for i, expected := range expectedSteps {
			if i < len(m.Steps) && m.Steps[i].ID != expected {
				t.Errorf("Step %d: expected '%s', got '%s'", i, expected, m.Steps[i].ID)
			}
		}
	})
}

func TestStepStatus(t *testing.T) {
	t.Run("status constants should have correct values", func(t *testing.T) {
		if StatusPending != 0 {
			t.Error("StatusPending should be 0")
		}
		if StatusRunning != 1 {
			t.Error("StatusRunning should be 1")
		}
		if StatusDone != 2 {
			t.Error("StatusDone should be 2")
		}
		if StatusFailed != 3 {
			t.Error("StatusFailed should be 3")
		}
		if StatusSkipped != 4 {
			t.Error("StatusSkipped should be 4")
		}
	})
}

func TestScreen(t *testing.T) {
	t.Run("screen constants should exist", func(t *testing.T) {
		// Just verify the screens exist and can be used
		screens := []Screen{
			ScreenWelcome,
			ScreenMainMenu,
			ScreenOSSelect,
			ScreenTerminalSelect,
			ScreenFontSelect,
			ScreenShellSelect,
			ScreenWMSelect,
			ScreenNvimSelect,
			ScreenInstalling,
			ScreenComplete,
			ScreenError,
			ScreenLearnTerminals,
			ScreenLearnShells,
			ScreenLearnWM,
			ScreenLearnNvim,
			ScreenKeymaps,
			ScreenKeymapCategory,
			ScreenLearnLazyVim,
			ScreenLazyVimTopic,
			ScreenBackupConfirm,
			ScreenRestoreBackup,
			ScreenRestoreConfirm,
		}

		// Verify we have all expected screens (including 3 new backup screens)
		if len(screens) != 22 {
			t.Errorf("Expected 22 screens, got %d", len(screens))
		}
	})
}

func TestBackupScreenOptions(t *testing.T) {
	t.Run("ScreenBackupConfirm should return correct options", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenBackupConfirm

		opts := m.GetCurrentOptions()

		if len(opts) != 3 {
			t.Errorf("Expected 3 options for BackupConfirm, got %d", len(opts))
		}

		// Check options contain expected text
		expectedOptions := []string{"Install with Backup", "Install without Backup", "Cancel"}
		for i, expected := range expectedOptions {
			found := false
			for _, opt := range opts {
				if containsString(opt, expected) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected option %d to contain '%s'", i, expected)
			}
		}
	})

	t.Run("ScreenRestoreBackup should return options based on available backups", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenRestoreBackup
		m.AvailableBackups = []system.BackupInfo{
			{Path: "/test/backup1", Files: []string{"file1", "file2"}},
			{Path: "/test/backup2", Files: []string{"file3"}},
		}

		opts := m.GetCurrentOptions()

		// Should have: 2 backups + separator + Back = 4 options
		if len(opts) != 4 {
			t.Errorf("Expected 4 options for RestoreBackup with 2 backups, got %d", len(opts))
		}

		// Last option should be Back
		if opts[len(opts)-1] != "← Back" {
			t.Errorf("Last option should be '← Back', got '%s'", opts[len(opts)-1])
		}
	})

	t.Run("ScreenRestoreConfirm should return correct options", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenRestoreConfirm

		opts := m.GetCurrentOptions()

		if len(opts) != 3 {
			t.Errorf("Expected 3 options for RestoreConfirm, got %d", len(opts))
		}
	})
}

func TestBackupScreenTitles(t *testing.T) {
	m := NewModel()

	tests := []struct {
		screen   Screen
		expected string
	}{
		{ScreenBackupConfirm, "Existing Configs Detected"},
		{ScreenRestoreBackup, "Restore from Backup"},
		{ScreenRestoreConfirm, "Confirm Restore"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			m.Screen = tt.screen
			title := m.GetScreenTitle()
			if !containsString(title, tt.expected) {
				t.Errorf("Expected title to contain '%s', got '%s'", tt.expected, title)
			}
		})
	}
}

func TestMainMenuWithBackups(t *testing.T) {
	t.Run("should include restore option when backups exist", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenMainMenu
		m.AvailableBackups = []system.BackupInfo{
			{Path: "/test/backup"},
		}

		opts := m.GetCurrentOptions()

		hasRestoreOption := false
		for _, opt := range opts {
			if containsString(opt, "Restore from Backup") {
				hasRestoreOption = true
				break
			}
		}

		if !hasRestoreOption {
			t.Error("Main menu should include 'Restore from Backup' when backups exist")
		}
	})

	t.Run("should not include restore option when no backups", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenMainMenu
		m.AvailableBackups = []system.BackupInfo{}

		opts := m.GetCurrentOptions()

		for _, opt := range opts {
			if containsString(opt, "Restore from Backup") {
				t.Error("Main menu should not include 'Restore from Backup' when no backups exist")
			}
		}
	})
}

func TestSetupInstallStepsWithBackup(t *testing.T) {
	t.Run("should include backup step when CreateBackup is true", func(t *testing.T) {
		m := NewModel()
		m.SystemInfo = &system.SystemInfo{
			OS:       system.OSMac,
			HasBrew:  true,
			HasXcode: true,
		}
		m.Choices = UserChoices{
			OS:           "mac",
			Terminal:     "none",
			Shell:        "fish",
			WindowMgr:    "none",
			InstallNvim:  false,
			CreateBackup: true,
		}
		m.ExistingConfigs = []string{"nvim: /home/user/.config/nvim"}

		m.SetupInstallSteps()

		// First step should be backup
		if m.Steps[0].ID != "backup" {
			t.Errorf("First step should be 'backup' when CreateBackup is true, got '%s'", m.Steps[0].ID)
		}
	})

	t.Run("should not include backup step when CreateBackup is false", func(t *testing.T) {
		m := NewModel()
		m.SystemInfo = &system.SystemInfo{
			OS:       system.OSMac,
			HasBrew:  true,
			HasXcode: true,
		}
		m.Choices = UserChoices{
			OS:           "mac",
			Terminal:     "none",
			Shell:        "fish",
			WindowMgr:    "none",
			InstallNvim:  false,
			CreateBackup: false,
		}
		m.ExistingConfigs = []string{"nvim: /home/user/.config/nvim"}

		m.SetupInstallSteps()

		// First step should NOT be backup
		if m.Steps[0].ID == "backup" {
			t.Error("First step should not be 'backup' when CreateBackup is false")
		}
	})

	t.Run("should not include backup step when no existing configs", func(t *testing.T) {
		m := NewModel()
		m.SystemInfo = &system.SystemInfo{
			OS:       system.OSMac,
			HasBrew:  true,
			HasXcode: true,
		}
		m.Choices = UserChoices{
			OS:           "mac",
			Terminal:     "none",
			Shell:        "fish",
			WindowMgr:    "none",
			InstallNvim:  false,
			CreateBackup: true, // Even if true
		}
		m.ExistingConfigs = []string{} // Empty

		m.SetupInstallSteps()

		// First step should be clone, not backup
		if m.Steps[0].ID == "backup" {
			t.Error("Should not include backup step when ExistingConfigs is empty")
		}
	})
}

func TestUserChoicesBackupField(t *testing.T) {
	t.Run("UserChoices should have CreateBackup field", func(t *testing.T) {
		choices := UserChoices{
			OS:           "mac",
			Terminal:     "ghostty",
			InstallFont:  true,
			Shell:        "fish",
			WindowMgr:    "tmux",
			InstallNvim:  true,
			CreateBackup: true,
		}

		if !choices.CreateBackup {
			t.Error("CreateBackup should be true")
		}

		choices.CreateBackup = false
		if choices.CreateBackup {
			t.Error("CreateBackup should be false")
		}
	})
}

func TestModelBackupFields(t *testing.T) {
	t.Run("Model should have backup-related fields initialized", func(t *testing.T) {
		m := NewModel()

		if m.ExistingConfigs == nil {
			t.Error("ExistingConfigs should not be nil")
		}

		if m.AvailableBackups == nil {
			t.Error("AvailableBackups should not be nil")
		}

		if m.SelectedBackup != 0 {
			t.Errorf("SelectedBackup should be 0, got %d", m.SelectedBackup)
		}

		if m.BackupDir != "" {
			t.Errorf("BackupDir should be empty, got '%s'", m.BackupDir)
		}
	})
}

// Helper function
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
