package tui

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// StepError provides context about which step failed and why
type StepError struct {
	StepID      string
	StepName    string
	Description string
	Cause       error
}

func (e *StepError) Error() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Step '%s' failed\n", e.StepName))
	sb.WriteString(fmt.Sprintf("Description: %s\n", e.Description))
	if e.Cause != nil {
		sb.WriteString(fmt.Sprintf("\nDetails:\n%v", e.Cause))
	}
	return sb.String()
}

func (e *StepError) Unwrap() error {
	return e.Cause
}

// wrapStepError creates a detailed error for a step failure
func wrapStepError(stepID, stepName, description string, cause error) error {
	return &StepError{
		StepID:      stepID,
		StepName:    stepName,
		Description: description,
		Cause:       cause,
	}
}

// dryRun reports whether this run was started with --dry-run. The flag used to
// be advertised in --help but never read, so a "dry run" performed a real
// installation.
func dryRun() bool {
	switch os.Getenv("DOTFILES_DRY_RUN") {
	case "", "0", "false":
		return false
	default:
		return true
	}
}

// executeStep runs the actual installation for a step
func executeStep(stepID string, m *Model) error {
	if dryRun() {
		SendLog(stepID, fmt.Sprintf("DRY RUN: skipping step %q", stepID))
		return nil
	}

	switch stepID {
	case "backup":
		return stepBackupConfigs(m)
	case "clone":
		return stepCloneRepo(m)
	case "homebrew":
		return stepInstallHomebrew(m)
	case "deps":
		return stepInstallDeps(m)
	case "xcode":
		return stepInstallXcode(m)
	case "terminal":
		return stepInstallTerminal(m)
	case "font":
		return stepInstallFont(m)
	case "shell":
		return stepInstallShell(m)
	case "wm":
		return stepInstallWM(m)
	case "nvim":
		return stepInstallNvim(m)
	case "wslconfig":
		return stepInstallWSLConfig(m)
	case "cleanup":
		return stepCleanup(m)
	case "setshell":
		return stepSetDefaultShell(m)
	default:
		return fmt.Errorf("unknown step: %s", stepID)
	}
}

func stepBackupConfigs(m *Model) error {
	stepID := "backup"
	if len(m.ExistingConfigs) == 0 {
		SendLog(stepID, "No existing configs to backup")
		return nil
	}

	SendLog(stepID, fmt.Sprintf("Backing up %d existing configs...", len(m.ExistingConfigs)))

	// Extract just the config keys from the ExistingConfigs slice
	configKeys := make([]string, len(m.ExistingConfigs))
	for i, config := range m.ExistingConfigs {
		configKeys[i] = config
		SendLog(stepID, fmt.Sprintf("  → %s", config))
	}

	backupDir, skipped, err := system.CreateBackup(configKeys)
	if err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	m.BackupDir = backupDir
	// Runtime state such as a live Unix socket cannot be copied. Say so instead
	// of leaving the user with a backup that is quietly incomplete.
	if len(skipped) > 0 {
		SendLog(stepID, fmt.Sprintf("Skipped %d entry(ies) that are not regular files:", len(skipped)))
		for _, path := range skipped {
			SendLog(stepID, fmt.Sprintf("  ⤫ %s", path))
		}
	}
	SendLog(stepID, fmt.Sprintf("✓ Backup created at: %s", backupDir))
	return nil
}

// dotfilesRepoURL is the repository the installer clones at run time.
const dotfilesRepoURL = "https://github.com/albersg/dotfiles.git"

// envDotfilesRepoRef selects the revision to clone, defaulting to the
// repository's default branch.
//
// It exists because the container end-to-end tests build a binary from the
// revision under test and then let it clone the default branch, so the installer
// ran against a repository that did not contain the very files it was written to
// deploy. That mismatch failed those tests twice for two different reasons
// before this override existed.
const envDotfilesRepoRef = "DOTFILES_REPO_REF"

func stepCloneRepo(m *Model) error {
	stepID := "clone"

	// Clone into a directory owned by this run. The previous implementation used
	// the cwd-relative name "dotfiles", so running the installer from a directory
	// that already contained an unrelated "dotfiles" folder removed it with
	// `rm -rf dotfiles` before cloning.
	workDir, err := os.MkdirTemp("", "dotfiles-install-")
	if err != nil {
		return wrapStepError("clone", "Clone Repository",
			"Failed to create a temporary installation directory",
			err)
	}
	repoDir := filepath.Join(workDir, "dotfiles")

	SendLog(stepID, fmt.Sprintf("Cloning repository into %s...", repoDir))
	// --branch accepts a branch or a tag, and an empty value keeps the default
	// branch, so an ordinary installation is unaffected.
	branch := ""
	if ref := os.Getenv(envDotfilesRepoRef); ref != "" {
		branch = fmt.Sprintf(" --branch %q", ref)
		SendLog(stepID, fmt.Sprintf("Using revision %s", ref))
	}
	result := system.RunWithLogs(fmt.Sprintf("git clone --progress%s %s %q", branch, dotfilesRepoURL, repoDir), nil, func(line string) {
		SendLog(stepID, line)
	})
	if result.Error != nil {
		os.RemoveAll(workDir)
		return wrapStepError("clone", "Clone Repository",
			"Failed to clone the repository. Check your internet connection and git installation.",
			result.Error)
	}

	// A clone can exit zero and still leave nothing usable behind (interrupted
	// transfer, missing git). Verify the checkout before any step reads from it.
	if !system.DirExists(filepath.Join(repoDir, ".git")) {
		os.RemoveAll(workDir)
		return wrapStepError("clone", "Clone Repository",
			"Repository was cloned but is not a git checkout",
			fmt.Errorf("%s does not contain a .git directory", repoDir))
	}

	m.WorkDir = workDir
	m.RepoDir = repoDir
	SendLog(stepID, "✓ Repository cloned successfully")
	return nil
}

// repoDir returns the checkout created by the clone step. Steps fail loudly
// instead of silently falling back to a guessed, cwd-relative path.
func (m *Model) repoDir() (string, error) {
	if m.RepoDir == "" {
		return "", fmt.Errorf("the repository has not been cloned in this run")
	}
	return m.RepoDir, nil
}

// homebrewInstallerURL is the upstream install script the step downloads. It is
// fetched to a file before it is run instead of through `bash -c "$(curl ...)"`:
// see stepInstallHomebrew.
const homebrewInstallerURL = "https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh"

func stepInstallHomebrew(m *Model) error {
	stepID := "homebrew"

	// Termux doesn't use Homebrew - it uses pkg
	if m.SystemInfo.IsTermux {
		SendLog(stepID, "Skipping Homebrew (Termux uses pkg package manager)")
		return nil
	}

	if system.BrewInstalled() {
		SendLog(stepID, "Homebrew already installed, skipping...")
		m.SystemInfo.HasBrew = true
		return nil
	}

	SendLog(stepID, "Installing Homebrew package manager...")

	// The installer is downloaded and then run as two commands. The old
	// `/bin/bash -c "$(curl -fsSL ...)"` ran curl inside a command substitution,
	// where a failed download expands to the empty string: the shell it was
	// handed ran nothing and still exited 0. A 403 from
	// raw.githubusercontent.com therefore reached the caller as a successful
	// installation. Downloading to a file keeps curl's own exit status, which is
	// the failure this step has to see. The file lives in a temporary directory
	// this step owns and is removed on every path out.
	installer, err := os.CreateTemp("", "homebrew-install-*.sh")
	if err != nil {
		return wrapStepError("homebrew", "Install Homebrew",
			"Failed to create a temporary file for the Homebrew install script",
			err)
	}
	installerPath := installer.Name()
	if err := installer.Close(); err != nil {
		return wrapStepError("homebrew", "Install Homebrew",
			"Failed to create a temporary file for the Homebrew install script",
			err)
	}
	defer func() { _ = os.Remove(installerPath) }()

	logLine := func(line string) { SendLog(stepID, line) }

	if result := system.RunWithLogs(
		fmt.Sprintf("curl -fsSL -o %q %s", installerPath, homebrewInstallerURL), nil, logLine); result.Error != nil {
		return wrapStepError("homebrew", "Install Homebrew",
			"Failed to download the Homebrew install script. Check your internet connection and whether raw.githubusercontent.com is reachable; a proxy or firewall can return an HTTP error there.",
			result.Error)
	}

	if result := system.RunWithLogs(fmt.Sprintf("/bin/bash %q", installerPath), nil, logLine); result.Error != nil {
		return wrapStepError("homebrew", "Install Homebrew",
			"Failed to run the Homebrew install script",
			result.Error)
	}

	// Tell the rest of the run that Homebrew exists. SystemInfo is detected once
	// at startup, and the install script only exports brew into its own child
	// shell, so without this refresh every later step keeps using the native
	// package manager and ignores the Homebrew it just installed.
	//
	// The refresh is also the step's success test. The script can exit 0 having
	// installed nothing, and that is exactly what the empty-download idiom above
	// produced, so a missing brew is reported as the failure it is instead of
	// being logged as a warning under a "✓ installed successfully" line. The
	// message names the download because that is the cause this guard exists to
	// catch.
	if !system.BrewInstalled() {
		return wrapStepError("homebrew", "Install Homebrew",
			"Homebrew is still missing after the install script ran. The download or the script itself most likely failed; check your internet connection and whether raw.githubusercontent.com is reachable, then run the installer again.",
			fmt.Errorf("brew not found at %s after the install script completed", system.GetBrewPrefix()))
	}
	m.SystemInfo.HasBrew = true
	SendLog(stepID, fmt.Sprintf("✓ Homebrew installed at %s", system.GetBrewPrefix()))

	// Add to PATH
	homeDir := os.Getenv("HOME")
	brewPrefix := system.GetBrewPrefix()

	shellConfig := fmt.Sprintf(`eval "$(%s/bin/brew shellenv)"`, brewPrefix)

	SendLog(stepID, "Configuring shell to use Homebrew...")
	// Add to common shell configs
	for _, rcFile := range []string{".bashrc", ".zshrc"} {
		rcPath := filepath.Join(homeDir, rcFile)
		if f, err := os.OpenFile(rcPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			f.WriteString("\n" + shellConfig + "\n")
			f.Close()
		}
	}

	// Source it now
	system.Run(shellConfig, nil)

	SendLog(stepID, "✓ Homebrew installed successfully")
	return nil
}

// envSkipDeps lets a run proceed without the dependency step. It exists for
// environments where sudo cannot prompt for a password (containers, CI, the
// non-interactive installer) and the base packages are already present or are
// installed another way. The failure below names it, so a run that cannot
// install dependencies has a documented way forward instead of a raw sudo
// error.
const envSkipDeps = "DOTFILES_SKIP_DEPS"

// skipDeps reports whether the dependency step was explicitly skipped.
func skipDeps() bool {
	switch os.Getenv(envSkipDeps) {
	case "", "0", "false":
		return false
	default:
		return true
	}
}

// baseDependencies names the tools every later step relies on, per package
// manager. Homebrew is listed separately because it is preferred whenever it is
// present, matching installPlatformPackages.
func baseDependencies() platformPackages {
	return platformPackages{
		Brew:   "curl file git wget unzip fontconfig",
		Arch:   "base-devel curl file git wget unzip fontconfig",
		Fedora: "@development-tools curl file git wget unzip fontconfig",
		Debian: "build-essential curl file git unzip fontconfig procps",
	}
}

// usesApt mirrors the Debian branch of installPlatformPackages: apt runs only
// for a Debian-like host without Homebrew.
func usesApt(m *Model) bool {
	return (m.SystemInfo.OS == system.OSDebian || m.SystemInfo.OS == system.OSLinux) && !m.SystemInfo.HasBrew
}

// usesPacman mirrors the Arch branch of installPlatformPackages: pacman runs on
// an Arch host whether or not Homebrew is present, so the Arch package lists are
// the ones the install reaches and the ones the Arch filter has to protect.
func usesPacman(m *Model) bool {
	return m.SystemInfo.OS == system.OSArch
}

// usesDnf mirrors the Fedora branch of installPlatformPackages: dnf runs only
// for a Fedora host without Homebrew, because Homebrew takes over the install
// whenever it is present, exactly as it does for Debian. The Fedora package
// lists are the ones the dnf route reaches and the ones the Fedora filter
// protects; with Homebrew present the unfiltered Brew list is reached instead.
func usesDnf(m *Model) bool {
	return m.SystemInfo.OS == system.OSFedora && !m.SystemInfo.HasBrew
}

// componentPresentAfterInstall reports whether a component is on the machine
// once an install attempt has finished. The install result is authoritative for
// failure: a route that returned an error did not install it. On success the
// component is looked up on PATH and in the Homebrew prefix, because Homebrew
// writes its binaries into its own prefix and this process's PATH does not
// necessarily contain it moments after a successful install. A PATH-only lookup
// would report a component that was installed moments earlier as missing.
func componentPresentAfterInstall(result *system.ExecResult, command string) bool {
	if result != nil && result.Error != nil {
		return false
	}
	if system.CommandExists(command) {
		return true
	}
	prefix := os.Getenv("HOMEBREW_PREFIX")
	if prefix == "" {
		prefix = system.GetBrewPrefix()
	}
	info, err := os.Stat(filepath.Join(prefix, "bin", command))
	return err == nil && !info.IsDir()
}

// dependencyManualCommands returns the root-only commands the user can run by
// hand when sudo cannot prompt. It mirrors the dispatch of the dependency
// install so the guidance cannot drift from what would actually run.
func dependencyManualCommands(m *Model, deps platformPackages) string {
	switch {
	case m.SystemInfo.OS == system.OSArch:
		return "sudo pacman -S --needed --noconfirm " + deps.Arch
	case usesDnf(m):
		return "sudo dnf install -y " + deps.Fedora
	case usesApt(m):
		return "sudo apt-get update\nsudo apt-get install -y " + deps.Debian
	default:
		return "brew install " + deps.Brew
	}
}

// sudoPasswordUnavailable reports whether a failed command failed because sudo
// could not obtain a password. The non-interactive installer and container runs
// have no TTY to prompt on, so sudo refuses instead of running the command.
func sudoPasswordUnavailable(result *system.ExecResult) bool {
	if result == nil || result.Error == nil {
		return false
	}
	message := strings.ToLower(result.Stderr)
	if message == "" {
		message = strings.ToLower(result.Error.Error())
	}
	for _, marker := range []string{
		"a password is required",
		"no tty present",
		"a terminal is required",
		"askpass",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

// dependencyInstallError turns a failed dependency command into a visible
// failure. When sudo could not prompt for a password it names the commands to
// run and how to skip the step, instead of surfacing the raw sudo error. Every
// other failure keeps the step visible as a failure.
func dependencyInstallError(m *Model, deps platformPackages, result *system.ExecResult) error {
	if sudoPasswordUnavailable(result) {
		return wrapStepError("deps", "Install Dependencies",
			fmt.Sprintf("Installing dependencies needs root, and sudo could not ask for a password in this run.\n"+
				"Run these commands in a terminal, then run the installer again:\n\n%s\n\n"+
				"To skip installing dependencies instead, set %s=1 and run the installer again.",
				dependencyManualCommands(m, deps), envSkipDeps),
			nil)
	}
	return wrapStepError("deps", "Install Dependencies",
		"Failed to install base dependencies",
		result.Error)
}

func stepInstallDeps(m *Model) error {
	stepID := "deps"

	if skipDeps() {
		SendLog(stepID, fmt.Sprintf("Skipping dependency installation (%s is set)", envSkipDeps))
		return nil
	}

	// Termux: use pkg (no sudo needed)
	// Check both SystemInfo and Choices.OS for redundancy
	isTermux := m.SystemInfo.IsTermux || m.Choices.OS == "termux"
	if isTermux {
		SendLog(stepID, "Updating Termux packages...")
		result := system.RunPkgWithLogs("update", nil, func(line string) {
			SendLog(stepID, line)
		})
		if result.Error != nil {
			return wrapStepError("deps", "Install Dependencies",
				"Failed to update Termux packages",
				result.Error)
		}
		result = system.RunPkgWithLogs("upgrade -y", nil, func(line string) {
			SendLog(stepID, line)
		})
		if result.Error != nil {
			// Upgrade failures are not critical
			SendLog(stepID, "Warning: package upgrade had issues, continuing...")
		}
		SendLog(stepID, "Installing base dependencies...")
		result = system.RunPkgInstall("git curl", nil, func(line string) {
			SendLog(stepID, line)
		})
		if result.Error != nil {
			return wrapStepError("deps", "Install Dependencies",
				"Failed to install base dependencies on Termux",
				result.Error)
		}
		return nil
	}

	// Everything below uses the same package-manager preference as the shell
	// installs: Homebrew when it is present, the distribution's native manager
	// otherwise. The distribution is read from OS, which detection now fills in
	// on WSL too, so Fedora-on-WSL runs dnf and Arch-on-WSL runs pacman.
	deps := baseDependencies()
	if m.SystemInfo.IsWSL {
		// wslu provides wslview/wslpath integration on Debian-like WSL.
		deps.Debian += " wslu"
	}

	// apt needs its index refreshed before it can install. Only apt does, and
	// only when Homebrew is not taking over the install, exactly as
	// installPlatformPackages decides.
	if usesApt(m) {
		result := runSudoWithLogs("apt-get update", nil, func(line string) {
			SendLog(stepID, line)
		})
		if result.Error != nil {
			return dependencyInstallError(m, deps, result)
		}
	}

	result := installPlatformPackages(m, stepID, deps, func(line string) {
		SendLog(stepID, line)
	})
	if result.Error != nil {
		return dependencyInstallError(m, deps, result)
	}

	if m.SystemInfo.IsWSL && !m.SystemInfo.HasBrew {
		SendLog(stepID, "✓ WSL utilities (wslu) installed for clipboard/browser integration")
	}
	return nil
}

func stepInstallXcode(m *Model) error {
	result := system.Run("xcode-select --install", nil)
	if result.Error != nil {
		// xcode-select returns error if already installed, which is fine
		if result.ExitCode == 1 && strings.Contains(result.Stderr, "already installed") {
			return nil
		}
		return wrapStepError("xcode", "Install Xcode CLI",
			"Failed to install Xcode Command Line Tools. You may need to install them manually from the App Store.",
			result.Error)
	}
	return nil
}

// supportedTerminals lists, in the order the help text shows them, the
// --terminal values this installer will honour on a platform. Only one value
// varies: kitty has a Homebrew cask on macOS and no installation route at all
// on the platforms this installer otherwise supports.
//
// This is the single source of truth for that rule. The --terminal flag
// description, the CLI validator and stepInstallTerminal all read it, so the
// value the tool refuses and the values it names as supported cannot drift
// apart. "none" is listed because the CLI accepts it, not because it is a
// terminal that gets installed.
func supportedTerminals(onMac bool) []string {
	if onMac {
		return []string{"alacritty", "wezterm", "kitty", "ghostty", "none"}
	}
	return []string{"alacritty", "wezterm", "ghostty", "none"}
}

// SupportedTerminals returns the --terminal values this installer honours on
// the platform named by goos (a runtime.GOOS value).
func SupportedTerminals(goos string) []string {
	return supportedTerminals(goos == "darwin")
}

// terminalSupported reports whether --terminal=<terminal> is a value this
// installer will install on the given platform.
func terminalSupported(onMac bool, terminal string) bool {
	for _, supported := range supportedTerminals(onMac) {
		if terminal == supported {
			return true
		}
	}
	return false
}

// unsupportedTerminalError is the refusal for a --terminal value this installer
// will not honour on a platform. It names the values it does support there, so
// the caller is told what to ask for instead of only what was rejected.
func unsupportedTerminalError(onMac bool, terminal string) error {
	return fmt.Errorf("invalid terminal: %s (valid: %s)", terminal, strings.Join(supportedTerminals(onMac), ", "))
}

// ValidateTerminal reports whether --terminal=<terminal> is honoured on the
// platform named by goos. The CLI refuses an unsupported value here, before
// anything is planned or touched, so an input this tool will not honour never
// reaches an installation step.
func ValidateTerminal(terminal, goos string) error {
	onMac := goos == "darwin"
	if terminalSupported(onMac, terminal) {
		return nil
	}
	return unsupportedTerminalError(onMac, terminal)
}

// kittyAction is what the kitty branch of stepInstallTerminal does once the
// platform and the result of the presence check are known.
type kittyAction int

const (
	// kittyRefuse means this platform has no route this installer will take.
	kittyRefuse kittyAction = iota
	// kittyInstall means the binary was verified absent and macOS is honoured.
	kittyInstall
	// kittyPresent means the binary was verified present.
	kittyPresent
)

// kittyActionFor decides what the kitty branch does for a platform and for the
// result of the presence check. The only way to reach the "already installed"
// report is present == true, which is what it used to get wrong: the old branch
// fell through to that report on Linux, for a binary it had never looked for.
func kittyActionFor(onMac, present bool) kittyAction {
	if !terminalSupported(onMac, "kitty") {
		return kittyRefuse
	}
	if present {
		return kittyPresent
	}
	return kittyInstall
}

func stepInstallTerminal(m *Model) error {
	terminal := m.Choices.Terminal
	homeDir := os.Getenv("HOME")
	repoDir := "dotfiles"
	stepID := "terminal"

	switch terminal {
	case "alacritty":
		if !system.CommandExists("alacritty") {
			SendLog(stepID, "Installing Alacritty...")
			var result *system.ExecResult
			if m.SystemInfo.OS == system.OSArch {
				result = system.RunSudoWithLogs("pacman -S --noconfirm alacritty", nil, func(line string) {
					SendLog(stepID, line)
				})
			} else if m.SystemInfo.OS == system.OSMac {
				result = system.RunBrewWithLogs("install --cask alacritty", nil, func(line string) {
					SendLog(stepID, line)
				})
			} else if m.SystemInfo.OS == system.OSFedora {
				// Fedora: install from dnf
				result = system.RunSudoWithLogs("dnf install -y alacritty", nil, func(line string) {
					SendLog(stepID, line)
				})
			} else if m.SystemInfo.OS == system.OSDebian || m.SystemInfo.OS == system.OSLinux {
				// Debian/Ubuntu: compile from source (PPAs are unreliable)
				SendLog(stepID, "Building Alacritty from source...")
				SendLog(stepID, "Installing build dependencies...")
				result = system.RunSudoWithLogs("apt-get install -y cmake pkg-config libfreetype6-dev libfontconfig1-dev libxcb-xfixes0-dev libxkbcommon-dev python3 gzip scdoc git curl", nil, func(line string) {
					SendLog(stepID, line)
				})
				if result.Error != nil {
					return wrapStepError("terminal", "Install Alacritty",
						"Failed to install build dependencies",
						result.Error)
				}
				// Install Rust/Cargo only for this build
				cargoPath := filepath.Join(homeDir, ".cargo/bin/cargo")
				if !system.CommandExists("cargo") && !system.CommandExists(cargoPath) {
					SendLog(stepID, "Installing Rust/Cargo toolchain...")
					result = system.RunWithLogs("curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y", nil, func(line string) {
						SendLog(stepID, line)
					})
					if result.Error != nil {
						return wrapStepError("terminal", "Install Alacritty",
							"Failed to install Rust",
							result.Error)
					}
					cargoPath = filepath.Join(homeDir, ".cargo/bin/cargo")
				}
				// Clone and build Alacritty
				SendLog(stepID, "Cloning Alacritty repository...")
				alacrittyDir := filepath.Join(os.TempDir(), "alacritty-build")
				os.RemoveAll(alacrittyDir)
				result = system.RunWithLogs(fmt.Sprintf("git clone https://github.com/alacritty/alacritty.git %s", alacrittyDir), nil, func(line string) {
					SendLog(stepID, line)
				})
				if result.Error != nil {
					return wrapStepError("terminal", "Install Alacritty",
						"Failed to clone Alacritty repository",
						result.Error)
				}
				SendLog(stepID, "Building Alacritty (this may take 5-10 minutes)...")
				if !system.CommandExists("cargo") {
					cargoPath = filepath.Join(homeDir, ".cargo/bin/cargo")
				} else {
					cargoPath = "cargo"
				}
				result = system.RunWithLogs(fmt.Sprintf("%s build --release --manifest-path %s/Cargo.toml", cargoPath, alacrittyDir), nil, func(line string) {
					SendLog(stepID, line)
				})
				if result.Error != nil {
					return wrapStepError("terminal", "Install Alacritty",
						"Failed to build Alacritty",
						result.Error)
				}
				SendLog(stepID, "Installing Alacritty binary...")
				result = system.RunSudoWithLogs(fmt.Sprintf("cp %s/target/release/alacritty /usr/local/bin/alacritty", alacrittyDir), nil, func(line string) {
					SendLog(stepID, line)
				})
				if result.Error != nil {
					return wrapStepError("terminal", "Install Alacritty",
						"Failed to install Alacritty binary",
						result.Error)
				}
				system.RunSudoWithLogs(fmt.Sprintf("cp %s/extra/linux/Alacritty.desktop /usr/share/applications/", alacrittyDir), nil, func(line string) {
					SendLog(stepID, line)
				})
				os.RemoveAll(alacrittyDir)
				SendLog(stepID, "✓ Alacritty built and installed from source")
			} else {
				return wrapStepError("terminal", "Install Alacritty",
					"Unsupported operating system for Alacritty installation",
					fmt.Errorf("OS type: %v", m.SystemInfo.OS))
			}
			if result.Error != nil {
				return wrapStepError("terminal", "Install Alacritty",
					"Failed to install Alacritty terminal emulator",
					result.Error)
			}
		} else {
			SendLog(stepID, "Alacritty already installed")
		}
		SendLog(stepID, "Copying Alacritty configuration...")
		if err := system.EnsureDir(filepath.Join(homeDir, ".config/alacritty")); err != nil {
			return wrapStepError("terminal", "Install Alacritty",
				"Failed to create Alacritty config directory",
				err)
		}
		if err := system.CopyFile(filepath.Join(repoDir, repoAssetAlacritty), filepath.Join(homeDir, ".config/alacritty/alacritty.toml")); err != nil {
			return wrapStepError("terminal", "Install Alacritty",
				"Failed to copy Alacritty configuration",
				err)
		}
		SendLog(stepID, "✓ Alacritty configured")

	case "wezterm":
		if !system.CommandExists("wezterm") {
			SendLog(stepID, "Installing WezTerm...")
			var result *system.ExecResult
			if m.SystemInfo.OS == system.OSArch {
				result = system.RunSudoWithLogs("pacman -S --noconfirm wezterm", nil, func(line string) {
					SendLog(stepID, line)
				})
			} else if m.SystemInfo.OS == system.OSFedora {
				// Fedora: enable COPR and install
				system.RunSudo("dnf copr enable -y wezfurlong/wezterm-nightly", nil)
				result = system.RunSudoWithLogs("dnf install -y wezterm", nil, func(line string) {
					SendLog(stepID, line)
				})
			} else if m.SystemInfo.OS == system.OSMac {
				result = system.RunBrewWithLogs("install --cask wezterm", nil, func(line string) {
					SendLog(stepID, line)
				})
			} else {
				system.Run("brew tap wez/wezterm-linuxbrew", nil)
				result = system.RunBrewWithLogs("install wezterm", nil, func(line string) {
					SendLog(stepID, line)
				})
			}
			if result.Error != nil {
				return wrapStepError("terminal", "Install WezTerm",
					"Failed to install WezTerm terminal emulator",
					result.Error)
			}
		} else {
			SendLog(stepID, "WezTerm already installed")
		}
		SendLog(stepID, "Copying WezTerm configuration...")
		if err := system.EnsureDir(filepath.Join(homeDir, ".config/wezterm")); err != nil {
			return wrapStepError("terminal", "Install WezTerm",
				"Failed to create WezTerm config directory",
				err)
		}
		if err := system.CopyFile(filepath.Join(repoDir, repoAssetWezterm), filepath.Join(homeDir, ".config/wezterm/wezterm.lua")); err != nil {
			return wrapStepError("terminal", "Install WezTerm",
				"Failed to copy WezTerm configuration",
				err)
		}
		SendLog(stepID, "✓ WezTerm configured")

	case "kitty":
		onMac := m.SystemInfo.OS == system.OSMac
		// Only macOS has a kitty route this installer takes, and the "already
		// installed" report is only reachable once CommandExists verified the
		// binary. The old branch installed only on macOS and ran its else branch
		// everywhere else, so on Linux it reported "Kitty already installed"
		// about a binary it had never checked for.
		switch kittyActionFor(onMac, system.CommandExists("kitty")) {
		case kittyRefuse:
			return wrapStepError("terminal", "Install Kitty",
				"Kitty is only installable on macOS",
				unsupportedTerminalError(onMac, terminal))
		case kittyInstall:
			SendLog(stepID, "Installing Kitty...")
			result := system.RunBrewWithLogs("install --cask kitty", nil, func(line string) {
				SendLog(stepID, line)
			})
			if result.Error != nil {
				return wrapStepError("terminal", "Install Kitty",
					"Failed to install Kitty terminal emulator",
					result.Error)
			}
		case kittyPresent:
			SendLog(stepID, "Kitty already installed")
		}
		SendLog(stepID, "Copying Kitty configuration...")
		if err := system.EnsureDir(filepath.Join(homeDir, ".config/kitty")); err != nil {
			return wrapStepError("terminal", "Install Kitty",
				"Failed to create Kitty config directory",
				err)
		}
		if err := system.CopyDir(filepath.Join(repoDir, repoAssetKitty), filepath.Join(homeDir, ".config", "kitty")); err != nil {
			return wrapStepError("terminal", "Install Kitty",
				"Failed to copy Kitty configuration",
				err)
		}
		SendLog(stepID, "✓ Kitty configured")

	case "ghostty":
		if !system.CommandExists("ghostty") {
			SendLog(stepID, "Installing Ghostty...")
			var result *system.ExecResult
			if m.SystemInfo.OS == system.OSArch {
				result = system.RunSudoWithLogs("pacman -S --noconfirm ghostty", nil, func(line string) {
					SendLog(stepID, line)
				})
			} else if m.SystemInfo.OS == system.OSFedora {
				// Fedora: enable COPR and install
				system.RunSudo("dnf copr enable -y pgdev/ghostty", nil)
				result = system.RunSudoWithLogs("dnf install -y ghostty", nil, func(line string) {
					SendLog(stepID, line)
				})
			} else if m.SystemInfo.OS == system.OSMac {
				result = system.RunBrewWithLogs("install --cask ghostty", nil, func(line string) {
					SendLog(stepID, line)
				})
			} else {
				result = system.RunWithLogs(`/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/mkasberg/ghostty-ubuntu/HEAD/install.sh)"`, nil, func(line string) {
					SendLog(stepID, line)
				})
			}
			if result.Error != nil {
				return wrapStepError("terminal", "Install Ghostty",
					"Failed to install Ghostty terminal emulator",
					result.Error)
			}
		} else {
			SendLog(stepID, "Ghostty already installed")
		}
		SendLog(stepID, "Copying Ghostty configuration...")
		if err := system.EnsureDir(filepath.Join(homeDir, ".config/ghostty")); err != nil {
			return wrapStepError("terminal", "Install Ghostty",
				"Failed to create Ghostty config directory",
				err)
		}
		if err := system.CopyDir(filepath.Join(repoDir, repoAssetGhostty), filepath.Join(homeDir, ".config", "ghostty")); err != nil {
			return wrapStepError("terminal", "Install Ghostty",
				"Failed to copy Ghostty configuration",
				err)
		}
		SendLog(stepID, "✓ Ghostty configured")
	}

	return nil
}

// The Linux font step downloads the Nerd Fonts release archive and extracts it.
// The release URL is pinned, and the archive is fetched into a scratch directory
// the step owns instead of into the font directory: see stepInstallFont.
const (
	iosevkaTermArchiveURL  = "https://github.com/ryanoasis/nerd-fonts/releases/download/v3.3.0/IosevkaTerm.zip"
	iosevkaTermArchiveName = "IosevkaTerm.zip"
	fontScratchDirPrefix   = ".iosevka-term-"
)

func stepInstallFont(m *Model) error {
	homeDir := os.Getenv("HOME")
	stepID := "font"

	// Termux: fonts work differently - copy to ~/.termux/font.ttf
	isTermux := m.SystemInfo.IsTermux || m.Choices.OS == "termux"
	if isTermux {
		SendLog(stepID, "Downloading JetBrainsMono Nerd Font for Termux...")
		termuxDir := filepath.Join(homeDir, ".termux")
		if err := system.EnsureDir(termuxDir); err != nil {
			return wrapStepError("font", "Install Nerd Font",
				"Failed to create .termux directory",
				err)
		}

		// Download a single TTF file for Termux
		result := system.RunWithLogs(fmt.Sprintf("curl -fsSL -o %s/font.ttf https://github.com/ryanoasis/nerd-fonts/raw/HEAD/patched-fonts/JetBrainsMono/Ligatures/Regular/JetBrainsMonoNerdFont-Regular.ttf", termuxDir), nil, func(line string) {
			SendLog(stepID, line)
		})
		if result.Error != nil {
			return wrapStepError("font", "Install Nerd Font",
				"Failed to download font. Check your internet connection.",
				result.Error)
		}

		SendLog(stepID, "Reloading Termux settings...")
		system.Run("termux-reload-settings", nil)
		SendLog(stepID, "✓ Font installed - restart Termux to apply")
		return nil
	}

	if m.SystemInfo.OS == system.OSMac {
		SendLog(stepID, "Installing Iosevka Term Nerd Font...")
		result := system.RunBrewWithLogs("install --cask font-iosevka-term-nerd-font", nil, func(line string) {
			SendLog(stepID, line)
		})
		if result.Error != nil {
			return wrapStepError("font", "Install Iosevka Nerd Font",
				"Failed to install font via Homebrew. Try installing manually from https://www.nerdfonts.com/",
				result.Error)
		}
		SendLog(stepID, "✓ Font installed")
		return nil
	}

	// Linux
	fontDir := filepath.Join(homeDir, ".local/share/fonts")
	SendLog(stepID, "Creating fonts directory...")
	if err := system.EnsureDir(fontDir); err != nil {
		return wrapStepError("font", "Install Iosevka Nerd Font",
			"Failed to create fonts directory",
			err)
	}

	// The archive used to be downloaded into fontDir itself and never removed, so
	// a 347 MB zip stayed in a directory fontconfig scans. It is fetched into a
	// scratch directory this step owns instead: inside fontDir, so the download
	// lands on the same filesystem rather than in a memory-backed /tmp, but never
	// the font directory itself, and removing that directory covers the failure
	// paths too. Doing this also keeps the cleanup away from every other file in
	// ~/.local/share/fonts, which belongs to the user.
	scratchDir, err := os.MkdirTemp(fontDir, fontScratchDirPrefix)
	if err != nil {
		return wrapStepError("font", "Install Iosevka Nerd Font",
			"Failed to create a temporary directory for the font archive",
			err)
	}
	defer func() {
		// A cleanup that fails is reported but never turns the step's own outcome
		// into a different one: the fonts are installed either way.
		if err := os.RemoveAll(scratchDir); err != nil {
			SendLog(stepID, fmt.Sprintf("Warning: could not remove the downloaded font archive: %v", err))
		}
	}()

	archivePath := filepath.Join(scratchDir, iosevkaTermArchiveName)

	SendLog(stepID, "Downloading Iosevka Term Nerd Font...")
	result := system.RunWithLogs(fmt.Sprintf("curl -fsSL -o %s %s", archivePath, iosevkaTermArchiveURL), nil, func(line string) {
		SendLog(stepID, line)
	})
	if result.Error != nil {
		return wrapStepError("font", "Install Iosevka Nerd Font",
			"Failed to download font. Check your internet connection.",
			result.Error)
	}

	SendLog(stepID, "Extracting font archive...")
	result = system.RunWithLogs(fmt.Sprintf("unzip -o %s -d %s/", archivePath, fontDir), nil, func(line string) {
		SendLog(stepID, line)
	})
	if result.Error != nil {
		return wrapStepError("font", "Install Iosevka Nerd Font",
			"Failed to extract font archive",
			result.Error)
	}

	SendLog(stepID, "Updating font cache...")
	system.RunWithLogs("fc-cache -fv", nil, func(line string) {
		SendLog(stepID, line)
	})
	SendLog(stepID, "✓ Font installed")
	return nil
}

type platformPackages struct {
	Termux string
	Brew   string
	Arch   string
	Fedora string
	Debian string

	// archUnavailable names the packages the Arch list had to drop because the
	// official repositories do not carry them. It travels beside the filtered
	// list so the caller can report the gap for the package manager that
	// actually runs. The Debian gap is returned separately by the constructors,
	// which predate this field.
	archUnavailable []string

	// fedoraUnavailable names the packages the Fedora list had to drop because
	// the distribution's own repositories do not carry them. It travels beside
	// the filtered list so the caller can report the gap for dnf, the manager
	// that actually runs on a Fedora host. It exists for the same reason as
	// archUnavailable beside the constructors' separate Debian return.
	fedoraUnavailable []string
}

var (
	runPkgInstallWithLogs = system.RunPkgInstall
	runSudoWithLogs       = system.RunSudoWithLogs
	runBrewWithLogs       = system.RunBrewWithLogs
	runOhMyZshInstaller   = system.RunWithLogs
)

// debianUnavailable names the packages this installer requests on other
// platforms but that Debian and Ubuntu do not carry in their own repositories.
//
// apt aborts the whole transaction when a single requested name is unknown, so
// a name listed in a Debian package set but missing from the distribution takes
// every package beside it down as well. The names below were verified against
// ubuntu:22.04, the image the installer's own E2E target runs, with
// `apt-cache policy`; a name that was not there is left out of every Debian
// list and reported to the user instead. apt is only reached when Homebrew is
// absent, so the Homebrew path already covers these tools where it works.
//
// The omission is version-specific: kubectx and tree-sitter-cli appear in
// Ubuntu 24.04 (and tree-sitter-cli again in 25.04) but not in 24.10, and
// starship only appears in 25.04. The list has to hold what the oldest
// supported release carries, because apt fails as a unit.
var debianUnavailable = map[string]bool{
	"starship":        true,
	"kubectx":         true,
	"nushell":         true,
	"zellij":          true,
	"lazygit":         true,
	"tree-sitter-cli": true,
}

// debianPackages joins wanted into a package list apt can install, dropping the
// names Debian/Ubuntu does not carry. It returns the dropped names separately so
// the caller can tell the user where to get them instead.
func debianPackages(wanted ...string) (installable string, unavailable []string) {
	kept := make([]string, 0, len(wanted))
	for _, name := range wanted {
		if debianUnavailable[name] {
			unavailable = append(unavailable, name)
			continue
		}
		kept = append(kept, name)
	}
	return strings.Join(kept, " "), unavailable
}

// logDebianUnavailable tells the user which requested tools the distribution
// does not provide, so a skipped tool is never mistaken for an installed one.
func logDebianUnavailable(stepID string, unavailable []string) {
	if len(unavailable) == 0 {
		return
	}
	SendLog(stepID, fmt.Sprintf(
		"Not available in the Debian/Ubuntu repositories, so not installed: %s.",
		strings.Join(unavailable, ", ")))
	SendLog(stepID, "Install them with Homebrew (brew install) or from each tool's own upstream installer.")
}

// archUnavailable names the packages this installer requests on other platforms
// but that Arch does not carry in its official repositories.
//
// pacman aborts the whole transaction when a single requested name is unknown,
// exactly as apt does, so a name listed in an Arch package set but missing from
// the distribution takes every package beside it down as well. Every name the
// Arch columns request was verified against archlinux:latest with `pacman -Sy`
// followed by `pacman -Si`; the names below were the only ones not found. A
// name that was not found is left out of every Arch list and reported to the
// user instead. pacman is reached whenever the host is Arch, with or without
// Homebrew, so the Homebrew fallback does not cover these tools by itself.
//
// Both names are companion packages the configuration reads at shell start, not
// components the user selects: carapace provides the completions fish, zsh and
// nushell source, and zsh-theme-powerlevel10k is the prompt theme .zshrc
// sources. Both are available from Homebrew as carapace and powerlevel10k and
// from the AUR, so the report names a route that exists.
var archUnavailable = map[string]bool{
	"carapace":                true,
	"zsh-theme-powerlevel10k": true,
}

// archPackages joins wanted into a package list pacman can install, dropping
// the names Arch does not carry. It returns the dropped names separately so the
// caller can tell the user where to get them instead.
func archPackages(wanted ...string) (installable string, unavailable []string) {
	kept := make([]string, 0, len(wanted))
	for _, name := range wanted {
		if archUnavailable[name] {
			unavailable = append(unavailable, name)
			continue
		}
		kept = append(kept, name)
	}
	return strings.Join(kept, " "), unavailable
}

// logArchUnavailable tells the user which requested tools the Arch repositories
// do not provide, so a skipped tool is never mistaken for an installed one.
func logArchUnavailable(stepID string, unavailable []string) {
	if len(unavailable) == 0 {
		return
	}
	SendLog(stepID, fmt.Sprintf(
		"Not available in the Arch repositories, so not installed: %s.",
		strings.Join(unavailable, ", ")))
	SendLog(stepID, "Install them with Homebrew (brew install) or from each tool's own upstream installer.")
}

// fedoraUnavailable names the packages this installer requests on other
// platforms but that Fedora does not carry in its own repositories.
//
// dnf aborts the whole transaction when a single requested name is unknown,
// exactly as apt and pacman do, so a name listed in a Fedora package set but
// missing from the distribution takes every package beside it down as well and
// leaves the shell or window manager that did exist uninstalled. Every name the
// Fedora columns request was checked against fedora:latest, the image the E2E
// target builds from, with `dnf install --assumeno`. That check resolves
// virtual provides and groups the same way the real install command does, so
// wget (provided by wget2-wget) and npm (provided by nodejs-npm) count as
// present instead of being dropped by a bare name comparison. The names below
// are the only ones that did not resolve; a name that did not resolve is left
// out of every Fedora list and reported to the user instead.
//
// The check is version-specific, and this note is the Fedora counterpart of the
// Debian one: nushell is absent from Fedora 40 but present in Fedora 44, so it
// is deliberately not listed here. Holding the names the target image actually
// reaches keeps a release that does not carry one failing dnf loudly, exactly
// as it would for apt or pacman.
//
// zellij is a component the user can select, and it is the one name below the
// step must fail on. carapace (the completions fish, zsh and nushell source),
// starship (the prompt those shells start) and lazygit (the Neovim git UI) are
// companions the configuration reads when present, so they are a logged notice.
// All four are available from Homebrew and from each tool's upstream installer.
var fedoraUnavailable = map[string]bool{
	"carapace": true,
	"starship": true,
	"zellij":   true,
	"lazygit":  true,
}

// fedoraPackages joins wanted into a package list dnf can install, dropping the
// names Fedora does not carry. It returns the dropped names separately so the
// caller can tell the user where to get them instead.
func fedoraPackages(wanted ...string) (installable string, unavailable []string) {
	kept := make([]string, 0, len(wanted))
	for _, name := range wanted {
		if fedoraUnavailable[name] {
			unavailable = append(unavailable, name)
			continue
		}
		kept = append(kept, name)
	}
	return strings.Join(kept, " "), unavailable
}

// logFedoraUnavailable tells the user which requested tools the Fedora
// repositories do not provide, so a skipped tool is never mistaken for an
// installed one.
func logFedoraUnavailable(stepID string, unavailable []string) {
	if len(unavailable) == 0 {
		return
	}
	SendLog(stepID, fmt.Sprintf(
		"Not available in the Fedora repositories, so not installed: %s.",
		strings.Join(unavailable, ", ")))
	SendLog(stepID, "Install them with Homebrew (brew install) or from each tool's own upstream installer.")
}

// logFedoraStillMissing reports the Fedora-filtered packages that are still
// absent once an install attempt has finished. The notice has to describe the
// gap that remains, not the names the filter dropped before anything ran: a
// component the available route installed, or one the host already had, is not
// missing and must not be named. The filtered list is only consulted when the
// dnf route is the one that ran, so a Fedora host whose Homebrew installed the
// components is never told they are unavailable.
func logFedoraStillMissing(stepID string, result *system.ExecResult, unavailable []string) {
	if len(unavailable) == 0 {
		return
	}
	missing := make([]string, 0, len(unavailable))
	for _, name := range unavailable {
		if !componentPresentAfterInstall(result, name) {
			missing = append(missing, name)
		}
	}
	logFedoraUnavailable(stepID, missing)
}

// ohMyZshInstallerURL is the official Oh My Zsh installer, pinned to the exact
// revision this repository ships and the local machine runs. An unpinned master
// URL would execute whatever upstream publishes next, which the previous
// repository-committed snapshot never did.
const (
	ohMyZshInstallerURL = "https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/0ee67f042872d1dfab74270c31867771ca35aef4/tools/install.sh"
	ohMyZshInstallerRef = "0ee67f042872d1dfab74270c31867771ca35aef4"
)

// ohMyZshEntrypoint is the file .zshrc sources. Its presence is what tells a
// complete installation apart from a directory an interrupted run left behind.
const ohMyZshEntrypoint = "oh-my-zsh.sh"

// shouldInstallOhMyZsh reports whether the installation is missing or incomplete.
//
// Checking the directory alone was not enough: the installer creates it early,
// so a download or clone that fails afterwards leaves a directory that every
// later run would report as installed and would never repair. PathExists follows
// symlinks on purpose, so a symlinked ~/.oh-my-zsh counts as present.
func shouldInstallOhMyZsh(dir string) bool {
	return !system.PathExists(filepath.Join(dir, ohMyZshEntrypoint))
}

// installOhMyZsh downloads the official installer and runs it against dir.
//
// It is deliberately two commands instead of the usual
// `sh -c "$(curl -fsSL ...)"`. That idiom needs an outer shell to expand the
// substitution; Termux does not use one because Go's fork/exec through a shell
// misbehaves on Android, so the substitution reached `sh -c` literally, which
// then tried to execute the first word of the downloaded script as a command:
//
//	sh: #!/bin/sh: not found
//
// Two plain commands also give a clear error for the download and for the run.
func installOhMyZsh(dir, stepID string) error {
	installer, err := os.CreateTemp("", "oh-my-zsh-install-*.sh")
	if err != nil {
		return fmt.Errorf("could not create a temporary file for the installer: %w", err)
	}
	installerPath := installer.Name()
	if err := installer.Close(); err != nil {
		return err
	}
	defer func() { _ = os.Remove(installerPath) }()

	logLine := func(line string) { SendLog(stepID, line) }

	if result := runOhMyZshInstaller(
		fmt.Sprintf("curl -fsSL -o %q %q", installerPath, ohMyZshInstallerURL), nil, logLine); result.Error != nil {
		return fmt.Errorf("could not download the Oh My Zsh installer: %w", result.Error)
	}

	// RUNZSH keeps the installer from starting a shell, CHSH from changing the
	// login shell and KEEP_ZSHRC from overwriting the .zshrc copied above.
	if result := runOhMyZshInstaller(
		fmt.Sprintf("env ZSH=%q RUNZSH=no CHSH=no KEEP_ZSHRC=yes sh %q", dir, installerPath), nil, logLine); result.Error != nil {
		removePartialOhMyZsh(dir)
		return fmt.Errorf("could not run the Oh My Zsh installer: %w", result.Error)
	}

	if !system.PathExists(filepath.Join(dir, ohMyZshEntrypoint)) {
		removePartialOhMyZsh(dir)
		return fmt.Errorf("the installer completed but %s does not exist", filepath.Join(dir, ohMyZshEntrypoint))
	}
	return nil
}

// removePartialOhMyZsh deletes a directory an interrupted install left behind.
// Without it the guard would read that directory as a finished installation and
// no later run could ever repair the shell.
func removePartialOhMyZsh(dir string) {
	if system.PathExists(dir) {
		_ = os.RemoveAll(dir)
	}
}

func installPlatformPackages(m *Model, stepID string, packages platformPackages, onLog func(string)) *system.ExecResult {
	switch {
	case m.SystemInfo.IsTermux:
		return runPkgInstallWithLogs(packages.Termux, nil, onLog)
	case m.SystemInfo.OS == system.OSArch && packages.Arch != "":
		return runNativeWithBrewFallback("pacman -S --needed --noconfirm "+packages.Arch, packages.Brew, m.SystemInfo.HasBrew, onLog)
	// dnf aborts the whole transaction on a single unknown name, exactly as apt
	// and pacman do, so the Fedora lists are filtered by fedoraPackages before
	// they reach this command and the names Fedora does not carry never arrive.
	//
	// dnf's own answer to that abort, --skip-unavailable, is deliberately not
	// used. With the filter in place it no longer buys the install its coverage,
	// and keeping it would let a name the filter does not know about be dropped
	// without a word, which is the defect this change removes. A name that is
	// genuinely absent now fails the command loudly instead of disappearing.
	//
	// dnf runs only when Homebrew is absent. With Homebrew present the switch
	// falls through to the default branch and the unfiltered Brew list installs
	// everything, so a component the Fedora repositories do not carry is still
	// installed instead of being left to a step that would fail.
	case usesDnf(m) && packages.Fedora != "":
		return runNativeWithBrewFallback("dnf install -y "+packages.Fedora, packages.Brew, m.SystemInfo.HasBrew, onLog)
	case (m.SystemInfo.OS == system.OSDebian || m.SystemInfo.OS == system.OSLinux) && !m.SystemInfo.HasBrew && packages.Debian != "":
		return runNativeWithBrewFallback("apt-get install -y "+packages.Debian, packages.Brew, m.SystemInfo.HasBrew, onLog)
	default:
		if m.SystemInfo.HasBrew && packages.Brew != "" {
			return runBrewWithLogs("install "+packages.Brew, nil, onLog)
		}
		return &system.ExecResult{
			Error: fmt.Errorf("no package manager available for this platform"),
		}
	}
}

func runNativeWithBrewFallback(nativeCommand string, brewPackages string, hasBrew bool, onLog func(string)) *system.ExecResult {
	result := runSudoWithLogs(nativeCommand, nil, onLog)
	if result.Error == nil || !hasBrew || brewPackages == "" {
		return result
	}

	return runBrewWithLogs("install "+brewPackages, nil, onLog)
}

// Herdr release used by the fallback download. Keep the repository and the tag
// in sync with the asset checksums below.
const (
	herdrRepo    = "herdrdev/herdr"
	herdrVersion = "v0.9.1"
)

func installHerdrBinary(m *Model, stepID string) error {
	if system.CommandExists("herdr") {
		SendLog(stepID, "Herdr already installed")
		return nil
	}
	if m.SystemInfo.IsTermux {
		return fmt.Errorf("herdr is not available through the Termux package installer")
	}
	if m.SystemInfo.OS == system.OSMac || m.SystemInfo.HasBrew {
		result := system.RunBrewWithLogs("install herdr", nil, func(line string) {
			SendLog(stepID, line)
		})
		return result.Error
	}

	assetArch := ""
	expectedSHA256 := ""
	switch runtime.GOARCH {
	case "amd64":
		assetArch = "x86_64"
		expectedSHA256 = "2a02fed16beb651ef006e1d43f048f652ca4dc58ad053cd2d44450563d5c54b7"
	case "arm64":
		assetArch = "aarch64"
		expectedSHA256 = "f4ccf4de745f2cb9a39a983e9ba3703dad50ec2a58dea83026ceab721bbd8d9e"
	default:
		return fmt.Errorf("unsupported Herdr architecture: %s", runtime.GOARCH)
	}

	homeDir := os.Getenv("HOME")
	binDir := filepath.Join(homeDir, ".local", "bin")
	if err := system.EnsureDir(binDir); err != nil {
		return err
	}

	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/herdr-linux-%s", herdrRepo, herdrVersion, assetArch)
	dest := filepath.Join(binDir, "herdr")
	SendLog(stepID, "Downloading Herdr release binary...")
	result := system.RunWithLogs(fmt.Sprintf("curl -fsSL %q -o %q", url, dest), nil, func(line string) {
		SendLog(stepID, line)
	})
	if result.Error != nil {
		return result.Error
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		return err
	}
	actualSHA256 := sha256.Sum256(data)
	if hex.EncodeToString(actualSHA256[:]) != expectedSHA256 {
		os.Remove(dest)
		return fmt.Errorf("Herdr checksum mismatch for %s", url)
	}

	return os.Chmod(dest, 0755)
}

// shellPlatformPackages returns the packages each package manager needs for the
// requested shell and the tools its configuration reads at start.
//
// The Debian entry is built with debianPackages, the Arch entry with
// archPackages and the Fedora entry with fedoraPackages, so a name the
// distribution does not carry never reaches apt, pacman or dnf and cannot abort
// the whole transaction. The second return value names the omitted Debian
// packages, and the archUnavailable and fedoraUnavailable fields name the
// omitted Arch and Fedora ones, so the caller can report them for the manager
// that actually runs.
func shellPlatformPackages(shell string) (platformPackages, []string) {
	switch shell {
	case "fish":
		debian, unavailable := debianPackages("fish", "zoxide", "starship")
		arch, archGaps := archPackages("fish", "carapace", "zoxide", "atuin", "starship")
		fedora, fedoraGaps := fedoraPackages("fish", "carapace", "zoxide", "atuin", "starship")
		return platformPackages{
			Termux:            "fish starship zoxide",
			Brew:              "fish carapace zoxide atuin starship",
			Arch:              arch,
			Fedora:            fedora,
			Debian:            debian,
			archUnavailable:   archGaps,
			fedoraUnavailable: fedoraGaps,
		}, unavailable
	case "zsh":
		// The zsh configuration depends on these at shell start: eza/bat/rg/fd/fzf
		// drive the aliases and the fzf integration, delta drives .gitconfig,
		// fnm owns Node, direnv hooks directory environments, and jq/gh/xh/trip
		// back the documented helper aliases. kubectx is only in the Homebrew and
		// Arch lists: the Debian/Ubuntu repositories do not carry it, and apt
		// cannot skip it the way dnf can. zsh-completions and fzf-tab are
		// Homebrew-only names, and fnm, eza, delta and xh are not in the Debian
		// repositories either.
		debian, unavailable := debianPackages(
			"zsh", "zoxide", "zsh-autosuggestions", "zsh-syntax-highlighting",
			"kubectx", "direnv", "jq", "gh", "bat", "fd-find", "ripgrep", "fzf",
		)
		arch, archGaps := archPackages(
			"zsh", "carapace", "zoxide", "atuin", "zsh-autosuggestions",
			"zsh-syntax-highlighting", "zsh-autocomplete",
			"zsh-theme-powerlevel10k", "kubectx", "eza", "bat", "fd",
			"ripgrep", "fzf", "direnv", "jq", "github-cli", "git-delta",
		)
		fedora, fedoraGaps := fedoraPackages(
			"zsh", "carapace", "zoxide", "atuin", "zsh-autosuggestions",
			"zsh-syntax-highlighting", "starship", "eza", "bat", "fd-find",
			"ripgrep", "fzf", "direnv", "jq", "gh", "git-delta",
		)
		return platformPackages{
			Termux: "zsh starship zoxide",
			Brew:   "zsh carapace zoxide atuin zsh-autosuggestions zsh-syntax-highlighting zsh-completions fzf-tab zsh-autocomplete powerlevel10k kubectx eza bat fd ripgrep fzf fnm direnv jq gh git-delta xh trippy",
			Arch:   arch,
			Fedora: fedora,
			// Debian stable does not package starship, fnm, eza, delta or xh; those
			// come from Homebrew, which this installer puts in place for Debian hosts.
			Debian:            debian,
			archUnavailable:   archGaps,
			fedoraUnavailable: fedoraGaps,
		}, unavailable
	case "nushell":
		debian, unavailable := debianPackages("nushell", "zoxide", "jq", "bash", "starship")
		arch, archGaps := archPackages("nushell", "carapace", "zoxide", "atuin", "jq", "bash", "starship")
		fedora, fedoraGaps := fedoraPackages("nushell", "carapace", "zoxide", "atuin", "jq", "bash", "starship")
		return platformPackages{
			Termux:            "nushell starship zoxide jq",
			Brew:              "nushell carapace zoxide atuin jq bash starship",
			Arch:              arch,
			Fedora:            fedora,
			Debian:            debian,
			archUnavailable:   archGaps,
			fedoraUnavailable: fedoraGaps,
		}, unavailable
	}
	return platformPackages{}, nil
}

// shellPackageName maps the installer's shell choice to the package that
// provides the shell binary itself, rather than a plugin beside it.
func shellPackageName(shell string) string {
	if shell == "nushell" {
		return "nushell"
	}
	return shell
}

// shellCommandName maps the installer's shell choice to the command a user
// runs, which is what a presence check has to look for. The nushell package
// provides the nu binary.
func shellCommandName(shell string) string {
	if shell == "nushell" {
		return "nu"
	}
	return shell
}

// fedoraMissingSelectedShell is the selected-component rule for the shell,
// applied after the install instead of before it. A shell the Fedora filter
// dropped is still missing only when it is absent once dnf and the Homebrew
// fallback have both been attempted; dnf can report success while the filtered
// shell was never in its command, so the install result alone does not settle
// it and the shell itself has to be looked for. A nil return means the step may
// continue.
func fedoraMissingSelectedShell(m *Model, stepID, shell string, result *system.ExecResult) error {
	if !usesDnf(m) || !fedoraUnavailable[shellPackageName(shell)] {
		return nil
	}
	if componentPresentAfterInstall(result, shellCommandName(shell)) {
		return nil
	}
	return wrapStepError(stepID, "Install Shell",
		fmt.Sprintf("%s is not available in this distribution's own package repositories, so it was not installed. "+
			"Install it with Homebrew (brew install %s) or from its upstream installer, then run the installer again.",
			shell, shellPackageName(shell)),
		nil)
}

func stepInstallShell(m *Model) error {
	homeDir := os.Getenv("HOME")
	shell := m.Choices.Shell
	stepID := "shell"

	repoDir, err := m.repoDir()
	if err != nil {
		return wrapStepError(stepID, "Install Shell",
			"Failed to locate the cloned repository",
			err)
	}

	packages, unavailable := shellPlatformPackages(shell)

	// Common dependencies
	SendLog(stepID, "Creating required directories...")
	system.EnsureDir(filepath.Join(homeDir, ".config"))
	system.EnsureDir(filepath.Join(homeDir, ".cache/starship"))
	system.EnsureDir(filepath.Join(homeDir, ".cache/carapace"))
	system.EnsureDir(filepath.Join(homeDir, ".local/share/atuin"))

	// On a Debian-like host without Homebrew, apt is the only route and it cannot
	// install the names its repositories do not carry. Say which ones are being
	// skipped so a missing tool is never mistaken for an installed one, and fail
	// outright when the shell the user explicitly selected is the missing one:
	// a companion package can be a logged note, the requested shell cannot.
	if usesApt(m) {
		logDebianUnavailable(stepID, unavailable)
		if debianUnavailable[shellPackageName(shell)] {
			return wrapStepError(stepID, "Install Shell",
				fmt.Sprintf("%s is not available in this distribution's own package repositories, so it was not installed. "+
					"Install it with Homebrew (brew install %s) or from its upstream installer, then run the installer again.",
					shell, shellPackageName(shell)),
				nil)
		}
	}

	// Arch reaches pacman whether or not Homebrew is present, and every shell
	// this installer can select is in the official repositories, so this is the
	// same companion-versus-selected rule applied rather than implied: the
	// companions below are a logged note, and the guard only fires if a future
	// edit makes a selectable shell one of the missing names.
	if usesPacman(m) {
		logArchUnavailable(stepID, packages.archUnavailable)
		if archUnavailable[shellPackageName(shell)] {
			return wrapStepError(stepID, "Install Shell",
				fmt.Sprintf("%s is not available in this distribution's own package repositories, so it was not installed. "+
					"Install it with Homebrew (brew install %s) or from its upstream installer, then run the installer again.",
					shell, shellPackageName(shell)),
				nil)
		}
	}

	// Fedora reaches dnf only when Homebrew is absent, and nushell is absent
	// from Fedora 40 while Fedora 44 carries it, so the filter holds the names
	// the target image actually lacks. Every shell this installer can select is
	// present there, which makes this the same companion-versus-selected rule the
	// Debian and Arch branches apply: the companions are a logged note. The
	// selected-component guard is not applied before the install: dnf can succeed
	// while a shell the filter dropped was never in its command, so the guard is
	// applied after the install below, once the route that ran has finished. The
	// companion notice is logged after that same attempt, so it names only what
	// is still missing rather than what the filter dropped.
	switch shell {
	case "fish":
		SendLog(stepID, "Installing Fish shell and plugins...")
		result := installPlatformPackages(m, stepID, packages, func(line string) {
			SendLog(stepID, line)
		})
		if usesDnf(m) {
			logFedoraStillMissing(stepID, result, packages.fedoraUnavailable)
		}
		if err := fedoraMissingSelectedShell(m, stepID, shell, result); err != nil {
			return err
		}
		if result.Error != nil {
			return wrapStepError("shell", "Install Fish",
				"Failed to install Fish shell and dependencies",
				result.Error)
		}
		SendLog(stepID, "Copying Fish configuration...")
		if err := system.CopyFile(filepath.Join(repoDir, repoAssetStarship), filepath.Join(homeDir, ".config/starship.toml")); err != nil {
			return wrapStepError("shell", "Install Fish",
				"Failed to copy starship configuration",
				err)
		}
		if err := system.CopyDir(filepath.Join(repoDir, repoAssetFish), filepath.Join(homeDir, ".config", "fish")); err != nil {
			return wrapStepError("shell", "Install Fish",
				"Failed to copy Fish configuration",
				err)
		}
		// Patch config.fish based on WM choice
		SendLog(stepID, "Configuring shell for window manager...")
		if err := system.PatchFishForWM(filepath.Join(homeDir, ".config/fish/config.fish"), m.Choices.WindowMgr, m.Choices.InstallNvim); err != nil {
			return wrapStepError("shell", "Install Fish",
				"Failed to configure config.fish for window manager",
				err)
		}
		// Remove tmux.fish function if not using tmux
		if m.Choices.WindowMgr != "tmux" {
			os.Remove(filepath.Join(homeDir, ".config/fish/functions/tmux.fish"))
		}
		// Termux: Add fish to $PREFIX/etc/shells so tmux doesn't complain
		if m.SystemInfo.IsTermux {
			SendLog(stepID, "Adding fish to Termux shells...")
			prefix := os.Getenv("PREFIX")
			if prefix == "" {
				prefix = "/data/data/com.termux/files/usr"
			}
			shellsFile := filepath.Join(prefix, "etc", "shells")
			system.EnsureDir(filepath.Join(prefix, "etc"))
			f, err := os.OpenFile(shellsFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err == nil {
				f.WriteString(filepath.Join(prefix, "bin", "fish") + "\n")
				f.Close()
			}
		}
		SendLog(stepID, "✓ Fish shell configured")

	case "zsh":
		SendLog(stepID, "Installing Zsh and plugins...")
		result := installPlatformPackages(m, stepID, packages, func(line string) {
			SendLog(stepID, line)
		})
		if usesDnf(m) {
			logFedoraStillMissing(stepID, result, packages.fedoraUnavailable)
		}
		if err := fedoraMissingSelectedShell(m, stepID, shell, result); err != nil {
			return err
		}
		if result.Error != nil {
			return wrapStepError("shell", "Install Zsh",
				"Failed to install Zsh and plugins",
				result.Error)
		}
		SendLog(stepID, "Copying Zsh configuration...")
		// Git's configuration is installed here rather than in a step of its own
		// because it belongs with the shell: delta and fzf, both installed by this
		// step, are what .gitconfig actually calls, and .gitconfig is a loose home
		// dotfile in the same family as .zshenv below. It is listed in ConfigPaths
		// so the backup step protects the existing file before this overwrites it.
		SendLog(stepID, "Copying Git configuration...")
		if err := system.CopyFile(filepath.Join(repoDir, repoAssetGitconfig), filepath.Join(homeDir, ".gitconfig")); err != nil {
			return wrapStepError("shell", "Install Zsh",
				"Failed to copy .gitconfig",
				err)
		}
		// The personal identity is optional by design: a machine may simply not
		// want one, and .gitconfig's includeIf does nothing when the file is
		// absent. It is also the file most likely to be missing from an older
		// checkout, because the installer clones the repository's default branch
		// while the binary can come from a branch that already ships this file.
		// TestRepoAssetsExist is what guarantees the repository contains it.
		personalSrc := filepath.Join(repoDir, repoAssetGitconfigPersonal)
		if _, err := os.Stat(personalSrc); err != nil {
			SendLog(stepID, "Skipping .gitconfig-personal: not present in this checkout")
		} else if err := system.CopyFile(personalSrc, filepath.Join(homeDir, ".gitconfig-personal")); err != nil {
			return wrapStepError("shell", "Install Zsh",
				"Failed to copy .gitconfig-personal",
				err)
		}
		if err := system.CopyFile(filepath.Join(repoDir, repoAssetZshEnv), filepath.Join(homeDir, ".zshenv")); err != nil {
			return wrapStepError("shell", "Install Zsh",
				"Failed to copy .zshenv configuration",
				err)
		}
		if err := system.CopyFile(filepath.Join(repoDir, repoAssetZshrc), filepath.Join(homeDir, ".zshrc")); err != nil {
			return wrapStepError("shell", "Install Zsh",
				"Failed to copy .zshrc configuration",
				err)
		}
		// Patch .zshrc based on WM choice
		SendLog(stepID, "Configuring shell for window manager...")
		if err := system.PatchZshForWM(filepath.Join(homeDir, ".zshrc"), m.Choices.WindowMgr, m.Choices.InstallNvim); err != nil {
			return wrapStepError("shell", "Install Zsh",
				"Failed to configure .zshrc for window manager",
				err)
		}
		if err := system.CopyFile(filepath.Join(repoDir, repoAssetP10k), filepath.Join(homeDir, ".p10k.zsh")); err != nil {
			return wrapStepError("shell", "Install Zsh",
				"Failed to copy Powerlevel10k configuration",
				err)
		}
		// The theme ships in the repository rather than being fetched at install
		// time because it has to match the palette the terminal emulators declare,
		// and no published theme does: the closest one only looked like it. It is
		// copied before the cache rebuild below, which is what makes bat find it.
		// It is also the newest asset here, so an older checkout may not ship it
		// yet: the installer clones the repository's default branch while the
		// binary can come from a branch that already carries this file, and a
		// missing theme must not abort the whole shell step. TestRepoAssetsExist is
		// what guarantees the repository contains it.
		batThemeSrc := filepath.Join(repoDir, repoAssetBatTheme)
		if _, err := os.Stat(batThemeSrc); err != nil {
			SendLog(stepID, "Skipping the bat theme: this checkout predates it")
		} else {
			batConfigDir := filepath.Join(homeDir, ".config", "bat")
			batThemesDir := filepath.Join(batConfigDir, "themes")
			if err := os.MkdirAll(batThemesDir, 0o755); err != nil {
				return wrapStepError("shell", "Install Zsh",
					"Failed to create the bat themes directory",
					err)
			}
			if err := system.CopyFile(batThemeSrc, filepath.Join(batThemesDir, "dotfiles.tmTheme")); err != nil {
				return wrapStepError("shell", "Install Zsh",
					"Failed to copy the bat theme",
					err)
			}
			// bat reads a theme from its cache, not from the themes directory, so the
			// cache has to be rebuilt for the copy above to have any effect. The rebuild
			// is best-effort on purpose: bat comes from the platform package lists, but
			// not on every platform, and a missing or failing bat must not fail the
			// whole shell step. BAT_CONFIG_DIR is set explicitly so the build reads the
			// directory the theme was just written into instead of resolving whatever
			// XDG_CONFIG_HOME the ambient environment happens to carry.
			if !system.CommandExists("bat") {
				SendLog(stepID, "bat not found in PATH, skipping the theme cache rebuild")
			} else if result := system.RunWithLogs("bat cache --build", &system.ExecOptions{
				Env: []string{"BAT_CONFIG_DIR=" + batConfigDir},
			}, func(line string) {
				SendLog(stepID, line)
			}); result.Error != nil {
				SendLog(stepID, fmt.Sprintf("Warning: could not rebuild the bat cache: %v", result.Error))
			} else {
				SendLog(stepID, "✓ bat theme installed")
			}
		}
		// Oh My Zsh manages its own checkout. Writing a vendored copy over an
		// existing clone dirties its tracked files, and `omz update` then fails on
		// the autostash pop, so the official installer runs only when nothing is
		// installed yet and an existing installation is never written into.
		ohMyZshDir := filepath.Join(homeDir, ".oh-my-zsh")
		if shouldInstallOhMyZsh(ohMyZshDir) {
			SendLog(stepID, "Installing Oh My Zsh...")
			if err := installOhMyZsh(ohMyZshDir, stepID); err != nil {
				return wrapStepError("shell", "Install Zsh",
					"Failed to install Oh My Zsh",
					err)
			}
		} else {
			SendLog(stepID, "Oh My Zsh already installed, leaving it untouched")
		}
		// Termux: Add zsh to $PREFIX/etc/shells so tmux doesn't complain
		if m.SystemInfo.IsTermux {
			SendLog(stepID, "Adding zsh to Termux shells...")
			prefix := os.Getenv("PREFIX")
			if prefix == "" {
				prefix = "/data/data/com.termux/files/usr"
			}
			shellsFile := filepath.Join(prefix, "etc", "shells")
			system.EnsureDir(filepath.Join(prefix, "etc"))
			f, err := os.OpenFile(shellsFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err == nil {
				f.WriteString(filepath.Join(prefix, "bin", "zsh") + "\n")
				f.Close()
			}
		}
		SendLog(stepID, "✓ Zsh configured with Powerlevel10k")

	case "nushell":
		SendLog(stepID, "Installing Nushell and dependencies...")
		result := installPlatformPackages(m, stepID, packages, func(line string) {
			SendLog(stepID, line)
		})
		if usesDnf(m) {
			logFedoraStillMissing(stepID, result, packages.fedoraUnavailable)
		}
		if err := fedoraMissingSelectedShell(m, stepID, shell, result); err != nil {
			return err
		}
		if result.Error != nil {
			return wrapStepError("shell", "Install Nushell",
				"Failed to install Nushell and dependencies",
				result.Error)
		}
		SendLog(stepID, "Copying Nushell configuration...")
		if err := system.CopyFile(filepath.Join(repoDir, repoAssetStarship), filepath.Join(homeDir, ".config/starship.toml")); err != nil {
			return wrapStepError("shell", "Install Nushell",
				"Failed to copy starship configuration",
				err)
		}
		if err := system.CopyFile(filepath.Join(repoDir, repoAssetBashEnvJSON), filepath.Join(homeDir, ".config/bash-env-json")); err != nil {
			return wrapStepError("shell", "Install Nushell",
				"Failed to copy bash-env-json",
				err)
		}
		if err := system.CopyFile(filepath.Join(repoDir, repoAssetBashEnvNu), filepath.Join(homeDir, ".config/bash-env.nu")); err != nil {
			return wrapStepError("shell", "Install Nushell",
				"Failed to copy bash-env.nu",
				err)
		}

		var nuDir string
		if runtime.GOOS == "darwin" {
			nuDir = filepath.Join(homeDir, "Library/Application Support/nushell")
		} else {
			nuDir = filepath.Join(homeDir, ".config/nushell")
		}
		if err := system.EnsureDir(nuDir); err != nil {
			return wrapStepError("shell", "Install Nushell",
				"Failed to create Nushell config directory",
				err)
		}
		if err := system.CopyDir(filepath.Join(repoDir, repoAssetNushell), nuDir); err != nil {
			return wrapStepError("shell", "Install Nushell",
				"Failed to copy Nushell configuration",
				err)
		}
		// Patch config.nu based on WM choice
		SendLog(stepID, "Configuring shell for window manager...")
		if err := system.PatchNushellForWM(filepath.Join(nuDir, "config.nu"), m.Choices.WindowMgr); err != nil {
			return wrapStepError("shell", "Install Nushell",
				"Failed to configure config.nu for window manager",
				err)
		}
		// Termux: Add nu to $PREFIX/etc/shells so tmux doesn't complain
		if m.SystemInfo.IsTermux {
			SendLog(stepID, "Adding nushell to Termux shells...")
			prefix := os.Getenv("PREFIX")
			if prefix == "" {
				prefix = "/data/data/com.termux/files/usr"
			}
			shellsFile := filepath.Join(prefix, "etc", "shells")
			system.EnsureDir(filepath.Join(prefix, "etc"))
			f, err := os.OpenFile(shellsFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err == nil {
				f.WriteString(filepath.Join(prefix, "bin", "nu") + "\n")
				f.Close()
			}
		}
		SendLog(stepID, "✓ Nushell configured")
	}

	return nil
}

// zellijPlatformPackages returns the packages for the Zellij window manager.
// Debian/Ubuntu does not carry zellij at all and Fedora does not either, so
// those entries are empty and the caller reports the omission instead of
// handing apt or dnf a name it would reject and fail the whole transaction on.
// Arch carries zellij, so its entry is unchanged, but it still goes through
// archPackages so a future edit cannot slip an unknown name past the filter.
func zellijPlatformPackages() (platformPackages, []string) {
	debian, unavailable := debianPackages("zellij")
	arch, archGaps := archPackages("zellij")
	fedora, fedoraGaps := fedoraPackages("zellij")
	return platformPackages{
		Termux:            "zellij",
		Brew:              "zellij",
		Arch:              arch,
		Fedora:            fedora,
		Debian:            debian,
		archUnavailable:   archGaps,
		fedoraUnavailable: fedoraGaps,
	}, unavailable
}

func stepInstallWM(m *Model) error {
	homeDir := os.Getenv("HOME")
	wm := m.Choices.WindowMgr
	stepID := "wm"

	repoDir, err := m.repoDir()
	if err != nil {
		return wrapStepError(stepID, "Install Window Manager",
			"Failed to locate the cloned repository",
			err)
	}

	switch wm {
	case "tmux":
		if !system.CommandExists("tmux") {
			SendLog(stepID, "Installing Tmux...")
			result := installPlatformPackages(m, stepID, platformPackages{
				Termux: "tmux",
				Brew:   "tmux",
				Arch:   "tmux",
				Fedora: "tmux",
				Debian: "tmux",
			}, func(line string) {
				SendLog(stepID, line)
			})
			if result.Error != nil {
				return wrapStepError("wm", "Install Tmux",
					"Failed to install Tmux",
					result.Error)
			}
		} else {
			SendLog(stepID, "Tmux already installed")
		}

		// TPM
		tpmDir := filepath.Join(homeDir, ".tmux/plugins/tpm")
		if _, err := os.Stat(tpmDir); os.IsNotExist(err) {
			SendLog(stepID, "Cloning TPM (Tmux Plugin Manager)...")
			result := system.RunWithLogs(fmt.Sprintf("git clone https://github.com/tmux-plugins/tpm %s", tpmDir), nil, func(line string) {
				SendLog(stepID, line)
			})
			if result.Error != nil {
				return wrapStepError("wm", "Install Tmux",
					"Failed to clone TPM (Tmux Plugin Manager)",
					result.Error)
			}
		}

		SendLog(stepID, "Copying Tmux configuration...")
		if err := system.EnsureDir(filepath.Join(homeDir, ".tmux")); err != nil {
			return wrapStepError("wm", "Install Tmux",
				"Failed to create .tmux directory",
				err)
		}
		// The plugin seed is optional: tmux.conf declares every plugin through TPM
		// and the install_plugins run below downloads them. Copy only when the
		// repository actually ships a seed, instead of failing the whole step.
		pluginsSrc := filepath.Join(repoDir, repoAssetTmuxPlugins)
		if system.DirExists(pluginsSrc) {
			if err := system.CopyDir(pluginsSrc, filepath.Join(homeDir, ".tmux", "plugins")); err != nil {
				return wrapStepError("wm", "Install Tmux",
					"Failed to copy Tmux plugins",
					err)
			}
		} else {
			SendLog(stepID, "No seeded Tmux plugins in the repository; TPM will install them")
		}
		if err := system.CopyFile(filepath.Join(repoDir, repoAssetTmuxConf), filepath.Join(homeDir, ".tmux.conf")); err != nil {
			return wrapStepError("wm", "Install Tmux",
				"Failed to copy tmux.conf",
				err)
		}

		// Configure tmux to use the user's chosen shell
		SendLog(stepID, "Configuring tmux default shell...")
		tmuxConfPath := filepath.Join(homeDir, ".tmux.conf")
		shellName := ""
		switch m.Choices.Shell {
		case "fish":
			shellName = "fish"
		case "zsh":
			shellName = "zsh"
		case "nushell":
			shellName = "nu"
		}
		if shellName != "" {
			// Find the full path to the shell
			shellFullPath := ""
			if m.SystemInfo.IsTermux {
				// In Termux, construct the path directly (which command has issues)
				prefix := os.Getenv("PREFIX")
				if prefix == "" {
					prefix = "/data/data/com.termux/files/usr"
				}
				shellFullPath = filepath.Join(prefix, "bin", shellName)
			} else {
				result := system.Run(fmt.Sprintf("which %s", shellName), nil)
				if result.Error == nil && result.Output != "" {
					shellFullPath = strings.TrimSpace(result.Output)
				}
			}
			if shellFullPath == "" {
				shellFullPath = shellName // Fallback
			}

			// Replace placeholder in tmux.conf with actual shell config
			content, err := os.ReadFile(tmuxConfPath)
			if err == nil {
				shellConfig := fmt.Sprintf("set -g default-command \"%s\"\nset -g default-shell \"%s\"", shellFullPath, shellFullPath)
				newContent := strings.Replace(string(content), "# DOTFILES_DEFAULT_SHELL", shellConfig, 1)
				os.WriteFile(tmuxConfPath, []byte(newContent), 0644)
			}
		}

		// Install plugins
		SendLog(stepID, "Installing Tmux plugins...")
		system.RunWithLogs(filepath.Join(homeDir, ".tmux/plugins/tpm/bin/install_plugins"), nil, func(line string) {
			SendLog(stepID, line)
		})
		SendLog(stepID, "✓ Tmux configured")

	case "zellij":
		if !system.CommandExists("zellij") {
			SendLog(stepID, "Installing Zellij...")
			packages, unavailable := zellijPlatformPackages()
			// Zellij is the window manager the user explicitly asked for, not a
			// companion package, so a distribution that cannot install it has to
			// fail the step rather than report a success that never happened.
			if usesApt(m) && len(unavailable) > 0 {
				logDebianUnavailable(stepID, unavailable)
				return wrapStepError(stepID, "Install Zellij",
					"Zellij is not available in this distribution's own package repositories, so it was not installed. "+
						"Install it with Homebrew (brew install zellij) or from https://zellij.dev, then run the installer again.",
					nil)
			}
			// Arch carries zellij, so this guard is inert today; it is the same
			// selected-component rule the Debian branch applies, kept so a future
			// edit cannot make the step report a success it never achieved.
			if usesPacman(m) && len(packages.archUnavailable) > 0 {
				logArchUnavailable(stepID, packages.archUnavailable)
				return wrapStepError(stepID, "Install Zellij",
					"Zellij is not available in this distribution's own package repositories, so it was not installed. "+
						"Install it with Homebrew (brew install zellij) or from https://zellij.dev, then run the installer again.",
					nil)
			}
			// Fedora does not carry zellij. The selected-component rule is applied
			// after the install rather than before it: zellij is only reported
			// missing once every available route has been attempted, so a Fedora
			// host whose Homebrew can provide it is not told it cannot be
			// installed. dnf is not asked for it (the Fedora list is empty), so
			// installPlatformPackages goes to its default branch and Homebrew is the
			// only route; the presence check below confirms the result, looking in
			// the Homebrew prefix as well as PATH.
			result := installPlatformPackages(m, stepID, packages, func(line string) {
				SendLog(stepID, line)
			})
			if usesDnf(m) && len(packages.fedoraUnavailable) > 0 && !componentPresentAfterInstall(result, "zellij") {
				logFedoraUnavailable(stepID, packages.fedoraUnavailable)
				return wrapStepError(stepID, "Install Zellij",
					"Zellij is not available in this distribution's own package repositories, so it was not installed. "+
						"Install it with Homebrew (brew install zellij) or from https://zellij.dev, then run the installer again.",
					nil)
			}
			if result.Error != nil {
				return wrapStepError("wm", "Install Zellij",
					"Failed to install Zellij",
					result.Error)
			}
		} else {
			SendLog(stepID, "Zellij already installed")
		}

		SendLog(stepID, "Copying Zellij configuration...")
		zellijDir := filepath.Join(homeDir, ".config/zellij")
		if err := system.EnsureDir(zellijDir); err != nil {
			return wrapStepError("wm", "Install Zellij",
				"Failed to create Zellij config directory",
				err)
		}
		if err := system.CopyDir(filepath.Join(repoDir, repoAssetZellij), zellijDir); err != nil {
			return wrapStepError("wm", "Install Zellij",
				"Failed to copy Zellij configuration",
				err)
		}

		// Configure zellij to use the user's chosen shell
		SendLog(stepID, "Configuring zellij default shell...")
		zellijConfPath := filepath.Join(zellijDir, "config.kdl")
		shellPath := ""
		switch m.Choices.Shell {
		case "fish":
			shellPath = "fish"
		case "zsh":
			shellPath = "zsh"
		case "nushell":
			shellPath = "nu"
		}
		if shellPath != "" {
			// Append default_shell config to zellij config.kdl
			f, err := os.OpenFile(zellijConfPath, os.O_APPEND|os.O_WRONLY, 0644)
			if err == nil {
				f.WriteString(fmt.Sprintf("\n// Default shell (configured by dotfiles)\ndefault_shell \"%s\"\n", shellPath))
				f.Close()
			}
		}
		SendLog(stepID, "✓ Zellij configured")

	case "herdr":
		if !system.CommandExists("herdr") {
			SendLog(stepID, "Installing Herdr...")
			if err := installHerdrBinary(m, stepID); err != nil {
				return wrapStepError("wm", "Install Herdr",
					"Failed to install Herdr",
					err)
			}
		} else {
			SendLog(stepID, "Herdr already installed")
		}

		SendLog(stepID, "Copying Herdr configuration...")
		herdrDir := filepath.Join(homeDir, ".config", "herdr")
		if err := system.EnsureDir(herdrDir); err != nil {
			return wrapStepError("wm", "Install Herdr",
				"Failed to create Herdr config directory",
				err)
		}
		if err := system.CopyFile(filepath.Join(repoDir, repoAssetHerdrConfig), filepath.Join(herdrDir, "config.toml")); err != nil {
			return wrapStepError("wm", "Install Herdr",
				"Failed to copy Herdr configuration",
				err)
		}
		if err := os.Chmod(filepath.Join(herdrDir, "config.toml"), 0644); err != nil {
			return wrapStepError("wm", "Install Herdr",
				"Failed to make Herdr configuration writable",
				err)
		}
		SendLog(stepID, "✓ Herdr configured")
	}

	return nil
}

// clipboardProvidersFor reports which packages Neovim needs for
// clipboard=unnamedplus on this platform, and whether it needs any at all.
//
// macOS needs none, because Neovim uses pbcopy and pbpaste there. Termux has
// neither package and uses the termux-api clipboard instead. Everything else
// gets both: the installer cannot know whether the session offers Wayland or
// X11, and under WSL either may be the one that works.
func clipboardProvidersFor(info *system.SystemInfo) (packages string, needed bool) {
	if info == nil || info.OS == system.OSMac || info.IsTermux {
		return "", false
	}
	return "xclip wl-clipboard", true
}

// clipboardPlatformPackages names the providers for every package manager that
// needs them. All four use the same two package names, so they are filled from
// one string: leaving a field empty would silently skip that platform.
func clipboardPlatformPackages(providers string) platformPackages {
	return platformPackages{
		Brew:   providers,
		Arch:   providers,
		Fedora: providers,
		Debian: providers,
	}
}

// nvimUserOwnedEntries are destination-relative paths inside ~/.config/nvim
// that belong to the user and to lazy.nvim rather than to the repository.
// lazy-lock.json is rewritten on the machine whenever plugins are updated, so
// its content legitimately differs from the repository copy and the installer
// must neither overwrite nor delete it when it is already present.
var nvimUserOwnedEntries = []string{"lazy-lock.json"}

// installConfigDir copies a configuration directory into place and reports every
// path the repository no longer ships that had to be removed, so a pruned file
// is visible in the install log instead of disappearing silently. keep names
// destination-relative paths that are user-owned runtime state.
func installConfigDir(stepID, src, dst string, keep ...string) error {
	removed, err := system.CopyDirPruned(src, dst, keep...)
	if err != nil {
		return err
	}
	if len(removed) == 0 {
		return nil
	}
	SendLog(stepID, fmt.Sprintf("Removed %d path(s) the repository no longer ships:", len(removed)))
	for _, path := range removed {
		SendLog(stepID, fmt.Sprintf("  ✗ %s", path))
	}
	return nil
}

// nvimPlatformPackages returns the packages the Neovim step installs. The
// Debian entry drops lazygit and tree-sitter-cli, which its repositories do not
// carry and apt cannot skip, and the Fedora entry drops lazygit, which Fedora
// does not carry for the same reason. The Arch entry goes through archPackages
// for the same reason even though every name it asks for exists there today.
func nvimPlatformPackages() (platformPackages, []string) {
	debian, unavailable := debianPackages(
		"neovim", "git", "gcc", "fzf", "fd-find", "ripgrep", "coreutils",
		"bat", "curl", "lazygit", "tree-sitter-cli",
	)
	arch, archGaps := archPackages(
		"neovim", "git", "gcc", "fzf", "fd", "ripgrep", "coreutils",
		"bat", "curl", "lazygit", "tree-sitter",
	)
	fedora, fedoraGaps := fedoraPackages(
		"neovim", "git", "gcc", "fzf", "fd-find", "ripgrep", "coreutils",
		"bat", "curl", "lazygit", "tree-sitter-cli",
	)
	return platformPackages{
		Termux:            "neovim git clang fzf fd ripgrep bat curl lazygit",
		Brew:              "nvim git gcc fzf fd ripgrep coreutils bat curl lazygit tree-sitter",
		Arch:              arch,
		Fedora:            fedora,
		Debian:            debian,
		archUnavailable:   archGaps,
		fedoraUnavailable: fedoraGaps,
	}, unavailable
}

func stepInstallNvim(m *Model) error {
	homeDir := os.Getenv("HOME")
	stepID := "nvim"

	repoDir, err := m.repoDir()
	if err != nil {
		return wrapStepError(stepID, "Install Neovim",
			"Failed to locate the cloned repository",
			err)
	}

	// Obsidian path
	SendLog(stepID, "Creating Obsidian directories...")
	obsidianDir := filepath.Join(homeDir, ".config/obsidian")
	system.EnsureDir(obsidianDir)
	system.EnsureDir(filepath.Join(obsidianDir, "templates"))

	// Check Node.js
	if !system.CommandExists("node") {
		SendLog(stepID, "Installing Node.js...")
		result := installPlatformPackages(m, stepID, platformPackages{
			Termux: "nodejs",
			Brew:   "node",
			Arch:   "nodejs npm",
			Fedora: "nodejs npm",
			Debian: "nodejs npm",
		}, func(line string) {
			SendLog(stepID, line)
		})
		if result.Error != nil {
			return wrapStepError("nvim", "Install Neovim",
				"Failed to install Node.js (required for LSP servers)",
				result.Error)
		}
	} else {
		SendLog(stepID, "Node.js already installed")
	}

	// Install dependencies
	SendLog(stepID, "Installing Neovim and dependencies...")
	// Termux package names differ from desktop Linux package managers.
	packages, unavailable := nvimPlatformPackages()
	if usesApt(m) {
		logDebianUnavailable(stepID, unavailable)
	}
	result := installPlatformPackages(m, stepID, packages, func(line string) {
		SendLog(stepID, line)
	})
	if usesDnf(m) {
		logFedoraStillMissing(stepID, result, packages.fedoraUnavailable)
	}
	if result.Error != nil {
		return wrapStepError("nvim", "Install Neovim",
			"Failed to install Neovim and dependencies",
			result.Error)
	}

	// Neovim runs with clipboard=unnamedplus, so it needs a provider from the
	// system. Without one every yank stays inside Neovim, never reaches the
	// desktop clipboard, and the editor reports nothing at all.
	if providers, needed := clipboardProvidersFor(m.SystemInfo); needed {
		SendLog(stepID, "Installing clipboard providers for Neovim...")
		clipboard := installPlatformPackages(m, stepID, clipboardPlatformPackages(providers), func(line string) {
			SendLog(stepID, line)
		})
		if clipboard.Error != nil {
			// Deliberately not fatal. Neovim itself is installed and usable, only
			// the clipboard integration is missing, and a distribution that names
			// the packages differently should not abort a finished installation.
			SendLog(stepID, "Could not install xclip and wl-clipboard, so yanks will stay inside Neovim: "+clipboard.Error.Error())
		} else {
			SendLog(stepID, "✓ Clipboard providers installed")
		}
	}

	// Copy config
	SendLog(stepID, "Copying Neovim configuration...")
	nvimDir := filepath.Join(homeDir, ".config/nvim")
	if err := system.EnsureDir(nvimDir); err != nil {
		return wrapStepError("nvim", "Install Neovim",
			"Failed to create Neovim config directory",
			err)
	}
	// Copy nvim config directory. The destination must end up matching the
	// checkout: a file the repository dropped used to survive forever and keep
	// being loaded beside the file that replaced it (issue #13).
	srcNvim := filepath.Join(repoDir, repoAssetNvim)
	if err := installConfigDir(stepID, srcNvim, nvimDir, nvimUserOwnedEntries...); err != nil {
		return wrapStepError("nvim", "Install Neovim",
			"Failed to copy Neovim configuration",
			err)
	}

	// Install Claude Code CLI (optional, don't fail on error)
	// Skip on Termux - Claude Code doesn't support Android
	if !m.SystemInfo.IsTermux {
		SendLog(stepID, "Installing Claude Code CLI (optional)...")
		system.RunWithLogs(`curl -fsSL https://claude.ai/install.sh | bash`, nil, func(line string) {
			SendLog(stepID, line)
		})
	} else {
		SendLog(stepID, "Skipping Claude Code (not supported on Termux)")
	}

	// Install OpenCode CLI (optional, don't fail on error)
	// Skip on Termux - OpenCode doesn't support Android
	if !m.SystemInfo.IsTermux {
		SendLog(stepID, "Installing OpenCode CLI (optional)...")
		system.RunWithLogs(`curl -fsSL https://opencode.ai/install | bash`, nil, func(line string) {
			SendLog(stepID, line)
		})
	} else {
		SendLog(stepID, "Skipping OpenCode (not supported on Termux)")
	}

	SendLog(stepID, "✓ Neovim configured with dotfiles setup")
	return nil
}

func stepCleanup(m *Model) error {
	stepID := "cleanup"
	if m.WorkDir == "" {
		SendLog(stepID, "Nothing to clean up")
		return nil
	}

	// Only remove the directory this run created, and only when the checkout is
	// actually inside it. Never derive the target from the current directory.
	if !strings.HasPrefix(m.RepoDir, m.WorkDir+string(os.PathSeparator)) {
		SendLog(stepID, fmt.Sprintf("Warning: refusing to remove %s", m.WorkDir))
		return nil
	}

	SendLog(stepID, fmt.Sprintf("Removing temporary checkout at %s...", m.WorkDir))
	if err := os.RemoveAll(m.WorkDir); err != nil {
		// Non-critical error, just log it
		SendLog(stepID, "Warning: Could not remove temporary directory")
		return nil
	}
	m.WorkDir = ""
	m.RepoDir = ""
	SendLog(stepID, "✓ Cleanup complete")
	return nil
}

// stepSetDefaultShell sets the selected shell as the user's default shell
// In non-interactive mode, this handles Termux specially (via .bashrc)
// and attempts to set the shell on other systems if possible
func stepSetDefaultShell(m *Model) error {
	stepID := "setshell"
	shell := m.Choices.Shell
	homeDir := os.Getenv("HOME")

	var shellCmd string
	switch shell {
	case "fish":
		shellCmd = "fish"
	case "zsh":
		shellCmd = "zsh"
	case "nushell":
		shellCmd = "nu"
	default:
		SendLog(stepID, fmt.Sprintf("Unknown shell: %s, skipping", shell))
		return nil
	}

	// Termux: no chsh available, modify .bashrc to auto-start shell
	if m.SystemInfo.IsTermux {
		SendLog(stepID, "Configuring shell auto-start for Termux...")

		// Find the shell path
		shellPath := system.Run(fmt.Sprintf("which %s", shellCmd), nil)
		if shellPath.Error != nil || strings.TrimSpace(shellPath.Output) == "" {
			SendLog(stepID, fmt.Sprintf("Shell '%s' not found in PATH, skipping", shellCmd))
			return nil
		}
		shellPathStr := strings.TrimSpace(shellPath.Output)

		// Read existing .bashrc
		bashrcPath := filepath.Join(homeDir, ".bashrc")
		existingContent := ""
		if data, err := os.ReadFile(bashrcPath); err == nil {
			existingContent = string(data)
		}

		// Check if already configured
		if strings.Contains(existingContent, "# dotfiles shell auto-start") {
			SendLog(stepID, "Shell auto-start already configured in ~/.bashrc")
			return nil
		}

		// Append auto-start configuration
		autoStartConfig := fmt.Sprintf(`
# dotfiles shell auto-start
if [ -x "%s" ] && [ -z "$DOTFILES_SHELL_STARTED" ]; then
    export DOTFILES_SHELL_STARTED=1
    exec %s
fi
`, shellPathStr, shellPathStr)

		f, err := os.OpenFile(bashrcPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return wrapStepError("setshell", "Set Default Shell",
				"Failed to open ~/.bashrc for writing",
				err)
		}
		defer f.Close()

		if _, err := f.WriteString(autoStartConfig); err != nil {
			return wrapStepError("setshell", "Set Default Shell",
				"Failed to write shell auto-start to ~/.bashrc",
				err)
		}

		SendLog(stepID, fmt.Sprintf("✓ Configured %s to auto-start in ~/.bashrc", shell))
		SendLog(stepID, "Close and reopen Termux for changes to take effect")
		return nil
	}

	// Non-Termux: Try to set shell using sudo usermod (works if NOPASSWD configured)
	// Find the shell path first
	shellPath := system.Run(fmt.Sprintf("which %s", shellCmd), nil)
	if shellPath.Error != nil || strings.TrimSpace(shellPath.Output) == "" {
		SendLog(stepID, fmt.Sprintf("Shell '%s' not found in PATH, skipping", shellCmd))
		return nil
	}
	shellPathStr := strings.TrimSpace(shellPath.Output)

	// Get current username
	currentUser := os.Getenv("USER")
	if currentUser == "" {
		currentUser = os.Getenv("LOGNAME")
	}
	if currentUser == "" {
		// Fallback to whoami command (useful in Docker containers)
		whoamiResult := system.Run("whoami", nil)
		if whoamiResult.Error == nil {
			currentUser = strings.TrimSpace(whoamiResult.Output)
		}
	}
	if currentUser == "" {
		SendLog(stepID, "Could not determine current user, skipping shell change")
		return nil
	}

	// First, ensure shell is in /etc/shells
	SendLog(stepID, fmt.Sprintf("Adding %s to /etc/shells if needed...", shellPathStr))
	checkShells := system.Run(fmt.Sprintf("grep -q '^%s$' /etc/shells", shellPathStr), nil)
	if checkShells.Error != nil {
		// Shell not in /etc/shells, try to add it
		addResult := system.RunSudo(fmt.Sprintf("sh -c 'echo \"%s\" >> /etc/shells'", shellPathStr), nil)
		if addResult.Error != nil {
			SendLog(stepID, fmt.Sprintf("Could not add %s to /etc/shells (may need manual setup)", shellPathStr))
		}
	}

	// Try sudo usermod first (more reliable than chsh in scripts)
	SendLog(stepID, fmt.Sprintf("Setting %s as default shell for %s...", shell, currentUser))
	result := system.RunSudo(fmt.Sprintf("usermod -s %s %s", shellPathStr, currentUser), nil)
	if result.Error != nil {
		// usermod failed, try chsh as fallback
		SendLog(stepID, "usermod failed, trying chsh...")
		result = system.RunSudo(fmt.Sprintf("chsh -s %s %s", shellPathStr, currentUser), nil)
		if result.Error != nil {
			// Both failed - not critical, just inform user
			SendLog(stepID, fmt.Sprintf("Could not set default shell automatically"))
			SendLog(stepID, fmt.Sprintf("Run manually: chsh -s %s", shellPathStr))
			return nil
		}
	}

	SendLog(stepID, fmt.Sprintf("✓ Default shell set to %s", shell))
	SendLog(stepID, "Log out and log back in for changes to take effect")
	return nil
}
