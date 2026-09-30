package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/albersg/dotfiles/installer/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

var Version = "dev"

// CLI flags for non-interactive mode
type cliFlags struct {
	version        bool
	help           bool
	test           bool
	dryRun         bool
	nonInteractive bool
	terminal       string
	shell          string
	windowMgr      string
	nvim           bool
	font           bool
	backup         bool
	noAnim         bool
	noMouse        bool
	noSprite       bool
}

func parseFlags() *cliFlags {
	flags := &cliFlags{}

	flag.BoolVar(&flags.version, "version", false, "Show version information")
	flag.BoolVar(&flags.version, "v", false, "Show version information (shorthand)")
	flag.BoolVar(&flags.help, "help", false, "Show help message")
	flag.BoolVar(&flags.help, "h", false, "Show help message (shorthand)")
	flag.BoolVar(&flags.test, "test", false, "Run in test mode (uses temporary directory)")
	flag.BoolVar(&flags.test, "t", false, "Run in test mode (shorthand)")
	flag.BoolVar(&flags.dryRun, "dry-run", false, "Show what would be installed without doing it")
	flag.BoolVar(&flags.nonInteractive, "non-interactive", false, "Run without TUI, use CLI flags")
	flag.StringVar(&flags.terminal, "terminal", "", "Terminal: "+strings.Join(tui.SupportedTerminals(runtime.GOOS), ", "))
	flag.StringVar(&flags.shell, "shell", "", "Shell: fish, zsh, nushell")
	flag.StringVar(&flags.windowMgr, "wm", "", "Window manager: tmux, zellij, herdr, none")
	flag.BoolVar(&flags.nvim, "nvim", false, "Install Neovim configuration")
	flag.BoolVar(&flags.font, "font", false, "Install Nerd Font")
	flag.BoolVar(&flags.backup, "backup", true, "Backup existing configs (default: true)")
	flag.BoolVar(&flags.noAnim, "no-anim", false, "Disable animations (same as DOTFILES_ANIM=0)")
	flag.BoolVar(&flags.noMouse, "no-mouse", false, "Do not track the mouse pointer (same as DOTFILES_MOUSE=0)")
	flag.BoolVar(&flags.noSprite, "no-sprite", false, "Draw the creature as glyphs, not as the shaded sprite (same as DOTFILES_SPRITE=0)")

	flag.Parse()
	return flags
}

func main() {
	// Hand the linker-injected build version to the TUI before anything can
	// render or print, so the splash and the --version flag agree.
	tui.Version = Version

	flags := parseFlags()

	if flags.version {
		fmt.Println("dotfiles " + tui.VersionLabel())
		os.Exit(0)
	}

	if flags.help {
		printHelp()
		os.Exit(0)
	}

	if flags.test {
		setupTestMode()
	}

	if flags.dryRun {
		os.Setenv("DOTFILES_DRY_RUN", "1")
		fmt.Println("🧪 Dry-run mode: No actual installations will be performed")
	}

	// --no-anim and DOTFILES_ANIM=0 are the same switch: the flag sets the variable
	// the TUI reads when it builds the model, so the gate has one source of truth
	// and the flag cannot drift from the environment.
	if flags.noAnim {
		os.Setenv("DOTFILES_ANIM", "0")
	}

	// --no-mouse and DOTFILES_MOUSE=0 are the same switch, on the same terms: the
	// flag sets the variable the TUI reads when it builds the model. It is a switch
	// of its own rather than a side of --no-anim because the pointer costs the user
	// something -- the terminal gives its own selection up while an application is
	// reading the mouse -- so it has to be possible to want the animation without
	// it, and to get the pointer back without losing the creature.
	if flags.noMouse {
		os.Setenv("DOTFILES_MOUSE", "0")
	}

	// --no-sprite and DOTFILES_SPRITE=0 are the same switch on the same terms. It is
	// its own switch because the shaded sprite is the one drawing a terminal cannot
	// be asked to be good at: a run whose terminal claims true colour but renders
	// block glyphs badly keeps the glyph cat with this and loses nothing else.
	if flags.noSprite {
		os.Setenv("DOTFILES_SPRITE", "0")
	}

	// Non-interactive mode: run installation directly with provided flags
	if flags.nonInteractive {
		if err := runNonInteractive(flags); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Interactive TUI mode
	model := tui.NewModel()
	// The pointer is asked for only when the run can use it, and the decision is
	// the model's: NewModel already read the gates to set Hovering, so the program
	// option and the model cannot disagree about whether a pointer event means
	// anything. With hover off NO mouse option is passed at all, which is what gives
	// the terminal's own selection back -- a run that merely ignored the events
	// would still have cost the user the drag.
	options := []tea.ProgramOption{tea.WithAltScreen()}
	if model.Hovering {
		options = append(options, tea.WithMouseAllMotion())
	}
	p := tea.NewProgram(model, options...)
	tui.SetGlobalProgram(p)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running installer: %v\n", err)
		os.Exit(1)
	}
}

func runNonInteractive(flags *cliFlags) error {
	// Validate required flags
	if flags.shell == "" {
		return fmt.Errorf("--shell is required (fish, zsh, nushell)")
	}

	// Normalize inputs
	terminal := strings.ToLower(flags.terminal)
	shell := strings.ToLower(flags.shell)
	wm := strings.ToLower(flags.windowMgr)

	// Validate values
	validShells := map[string]bool{"fish": true, "zsh": true, "nushell": true}
	validWMs := map[string]bool{"tmux": true, "zellij": true, "herdr": true, "none": true, "": true}

	if terminal == "" {
		terminal = "none"
	}

	// The terminal axis is the one that depends on the platform: kitty is only
	// installable on macOS, so asking for it here is refused before anything is
	// planned, with the values this tool does support named in the error.
	if err := tui.ValidateTerminal(terminal, runtime.GOOS); err != nil {
		return err
	}
	if !validShells[shell] {
		return fmt.Errorf("invalid shell: %s (valid: fish, zsh, nushell)", shell)
	}
	if !validWMs[wm] {
		return fmt.Errorf("invalid window manager: %s (valid: tmux, zellij, herdr, none)", wm)
	}

	// Default an empty window manager to "none"
	if wm == "" {
		wm = "none"
	}

	// Create choices
	choices := tui.UserChoices{
		Terminal:     terminal,
		Shell:        shell,
		WindowMgr:    wm,
		InstallNvim:  flags.nvim,
		InstallFont:  flags.font,
		CreateBackup: flags.backup,
	}

	fmt.Println("🚀 dotfiles Non-Interactive Installer")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("  Terminal:    %s\n", choices.Terminal)
	fmt.Printf("  Shell:       %s\n", choices.Shell)
	fmt.Printf("  Window Mgr:  %s\n", choices.WindowMgr)
	fmt.Printf("  Neovim:      %v\n", choices.InstallNvim)
	fmt.Printf("  Font:        %v\n", choices.InstallFont)
	fmt.Printf("  Backup:      %v\n", choices.CreateBackup)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	// Run the installation
	return tui.RunNonInteractive(choices)
}

func setupTestMode() {
	// Create a temporary test directory
	testDir := filepath.Join(os.TempDir(), "dotfiles-test")
	testHome := filepath.Join(testDir, "home")
	testConfig := filepath.Join(testHome, ".config")

	// Create directories
	os.MkdirAll(testConfig, 0755)

	// Override HOME to use test directory
	os.Setenv("HOME", testHome)
	os.Setenv("DOTFILES_TEST_MODE", "1")

	fmt.Printf("🧪 Test mode enabled!\n")
	fmt.Printf("   Test HOME: %s\n", testHome)
	fmt.Printf("   Your real configs are SAFE.\n")
	fmt.Printf("   Press Enter to continue...\n")

	// Wait for user to acknowledge
	fmt.Scanln()
}

func printHelp() {
	fmt.Println(`dotfiles - TUI installer for dotfiles terminal environment

Usage:
  dotfiles [flags]

Interactive Mode (default):
  Just run 'dotfiles' to start the TUI installer.

Non-Interactive Mode:
  dotfiles --non-interactive --shell=<shell> [options]

Flags:
  -h, --help           Show this help message
  -v, --version        Show version information
  -t, --test           Run in test mode (uses temporary directory)
  --dry-run            Show what would be installed without doing it
  --non-interactive    Run without TUI, use CLI flags instead
  --no-anim            Disable animations (same as DOTFILES_ANIM=0); animation
                       is also off when stdout is not a terminal or TERM=dumb

Non-Interactive Options:
  --shell=<shell>      Shell to install (required): fish, zsh, nushell
  --terminal=<term>    Terminal: alacritty, wezterm, ghostty, none
                       (kitty is available on macOS only)
  --wm=<wm>            Window manager: tmux, zellij, herdr, none
  --nvim               Install Neovim configuration
  --font               Install Nerd Font
  --backup=false       Disable config backup (default: true)

Examples:
  # Interactive TUI
  dotfiles

  # Non-interactive with Fish + Herdr + Neovim
  dotfiles --non-interactive --shell=fish --wm=herdr --nvim

  # Test mode with Zsh + Tmux (no terminal, no nvim)
  dotfiles --test --non-interactive --shell=zsh --wm=tmux

  # Verbose output (shows all command logs)
  DOTFILES_VERBOSE=1 dotfiles --non-interactive --shell=fish --nvim

Navigation (TUI mode):
  ↑/k, ↓/j        Navigate up/down
  Enter/Space     Select option
  Esc             Go back
  q               Quit
  d               Toggle details (during installation)

For more info: https://github.com/albersg/dotfiles`)
}
