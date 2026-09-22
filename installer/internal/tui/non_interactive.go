package tui

import (
	"errors"
	"fmt"
	"runtime"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// RunNonInteractive executes the installation without TUI
func RunNonInteractive(choices UserChoices) error {
	// Enable non-interactive mode for logging
	SetNonInteractiveMode(true)

	// Detect system info
	sysInfo := system.Detect()

	// Determine OS choice based on system
	osChoice := "linux"
	if runtime.GOOS == "darwin" {
		osChoice = "mac"
	}
	choices.OS = osChoice

	// Create a minimal model for the installation functions
	model := &Model{
		SystemInfo: sysInfo,
		Choices:    choices,
		LogLines:   []string{},
	}

	// Detect existing configs for backup functionality
	if choices.CreateBackup {
		model.ExistingConfigs = system.DetectExistingConfigs()
	}

	// Define steps to run based on choices
	steps := buildStepsForChoices(model)

	fmt.Printf("📋 Running %d installation steps...\n\n", len(steps))

	failures := runInstallSteps(steps, model, executeStep)

	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	switch {
	case len(failures) > 0:
		// The run finished what it could and cleaned up after itself, but it is
		// not a success: a step that failed is reported and the exit status is
		// non-zero so a script or CI run cannot mistake it for one.
		fmt.Printf("⚠️  Installation finished with %d failed step(s):\n", len(failures))
		for _, failure := range failures {
			fmt.Printf("   - %v\n", failure)
		}
	case dryRun():
		fmt.Println("🧪 Dry run complete: nothing was installed.")
	default:
		fmt.Println("✅ Installation complete!")
	}
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	if len(failures) > 0 {
		return errors.Join(failures...)
	}
	return nil
}

// runInstallSteps executes the planned steps in order. A failed step is recorded
// and reported, but it does not stop the run: every later step that can still do
// its work is attempted, the default-shell step is never skipped because an
// earlier root-only step failed, and cleanup always gets its turn so a temporary
// checkout is not left behind. One error is returned per failed step, in order,
// and the caller reports them together when the run ends.
func runInstallSteps(steps []InstallStep, model *Model, execute func(stepID string, m *Model) error) []error {
	var failures []error

	for i, step := range steps {
		fmt.Printf("[%d/%d] %s...\n", i+1, len(steps), step.Name)

		err := execute(step.ID, model)
		if err != nil {
			fmt.Printf("    ❌ FAILED: %v\n", err)
			failures = append(failures, fmt.Errorf("step '%s' failed: %w", step.Name, err))
			continue
		}
		if dryRun() {
			fmt.Printf("    • skipped (dry run)\n")
			continue
		}
		fmt.Printf("    ✓ Done\n")
	}

	return failures
}

// buildStepsForChoices creates the list of steps based on user choices
func buildStepsForChoices(m *Model) []InstallStep {
	var steps []InstallStep

	// Always backup first if enabled
	if m.Choices.CreateBackup {
		steps = append(steps, InstallStep{ID: "backup", Name: "Backup existing configs"})
	}

	// Dependencies first (must run before clone so git is available on fresh Linux installs)
	steps = append(steps, InstallStep{ID: "deps", Name: "Install dependencies"})

	// Clone repo (after deps so git is available)
	steps = append(steps, InstallStep{ID: "clone", Name: "Clone dotfiles repository"})

	// Homebrew (for Mac and Debian/Ubuntu Linux - NOT Fedora/Arch which use native package managers)
	if m.SystemInfo.OS == system.OSMac || m.SystemInfo.OS == system.OSDebian || m.SystemInfo.OS == system.OSLinux || m.SystemInfo.OS == system.OSWSL {
		steps = append(steps, InstallStep{ID: "homebrew", Name: "Install/Update Homebrew"})
	}

	// Xcode (Mac only)
	if m.SystemInfo.OS == system.OSMac {
		steps = append(steps, InstallStep{ID: "xcode", Name: "Install Xcode CLI tools"})
	}

	// Terminal
	if m.Choices.Terminal != "none" {
		steps = append(steps, InstallStep{ID: "terminal", Name: fmt.Sprintf("Install %s terminal", m.Choices.Terminal)})
	}

	// Font
	if m.Choices.InstallFont {
		steps = append(steps, InstallStep{ID: "font", Name: "Install Nerd Font"})
	}

	// Shell
	steps = append(steps, InstallStep{ID: "shell", Name: fmt.Sprintf("Install %s shell", m.Choices.Shell)})

	// Window Manager
	if m.Choices.WindowMgr != "none" {
		steps = append(steps, InstallStep{ID: "wm", Name: fmt.Sprintf("Install %s", m.Choices.WindowMgr)})
	}

	// Neovim
	if m.Choices.InstallNvim {
		steps = append(steps, InstallStep{ID: "nvim", Name: "Install Neovim configuration"})
	}

	// Toolset (after the shell step, whose fnm provides the Node runtime the
	// Brewfile's npm entries need; best-effort, so it never fails the run)
	steps = append(steps, InstallStep{ID: "toolset", Name: "Install toolset"})

	// WSL configuration (Windows host + in-distribution settings)
	if m.SystemInfo.IsWSL {
		steps = append(steps, InstallStep{ID: "wslconfig", Name: "Configure WSL"})
	}

	// Set shell as default
	steps = append(steps, InstallStep{ID: "setshell", Name: "Set shell as default"})

	// Cleanup
	steps = append(steps, InstallStep{ID: "cleanup", Name: "Cleanup"})

	return steps
}
