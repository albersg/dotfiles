package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/albersg/dotfiles/installer/internal/system"
	tea "github.com/charmbracelet/bubbletea"
)

const interactiveContinuePrompt = `if [ -t 0 ] && [ -r /dev/tty ]; then
    echo "Press Enter to continue..."
    if ! IFS= read -r dummy < /dev/tty; then
        echo "Continuing without confirmation prompt."
    fi
fi`

// needsExecProcessMsg signals that we need to run tea.ExecProcess
type needsExecProcessMsg struct {
	stepID string
	cmd    *exec.Cmd
}

// runInteractiveStep creates a tea.Cmd that runs an interactive step
// This suspends the TUI and gives full terminal control to the process
func runInteractiveStep(stepID string, m *Model) tea.Cmd {
	return func() tea.Msg {
		script, err := getInteractiveScript(stepID, m)
		if err != nil {
			return execFinishedMsg{stepID: stepID, err: fmt.Errorf("failed to get script for %s: %w", stepID, err)}
		}

		// If no script needed (e.g., already installed), just succeed
		if script == "" {
			return execFinishedMsg{stepID: stepID, err: nil}
		}

		cmd, err := createTempScriptCommand(script)
		if err != nil {
			return execFinishedMsg{stepID: stepID, err: fmt.Errorf("failed to create script for %s: %w", stepID, err)}
		}

		// Return message that tells Update to use tea.ExecProcess
		return needsExecProcessMsg{stepID: stepID, cmd: cmd}
	}
}

// interactiveScriptBuilders is the dispatch table for interactive steps. Each
// entry turns a step ID into the shell script tea.ExecProcess runs.
//
// It is a map rather than a switch so the flag-versus-dispatch invariant test
// can enumerate the cases instead of restating them: a step that
// SetupInstallSteps marks Interactive and a builder that this table does not
// know is the contradiction behind issue #28, and enumerating the table is what
// lets a test close it for every step rather than for the one that broke.
var interactiveScriptBuilders = map[string]func(*Model) (string, error){
	"homebrew":  getHomebrewScript,
	"deps":      getDepsScript,
	"terminal":  getTerminalScript,
	"setshell":  getSetShellScript,
	"wslconfig": getWSLConfigScript,
}

// getInteractiveScript returns the bash script for interactive steps only
// Interactive steps are those that NEED user input (sudo password, chsh, etc)
func getInteractiveScript(stepID string, m *Model) (string, error) {
	build, ok := interactiveScriptBuilders[stepID]
	if !ok {
		return "", fmt.Errorf("unknown interactive step: %s", stepID)
	}
	return build(m)
}

// getHomebrewScript returns script to install Homebrew (needs password on first install)
func getHomebrewScript(m *Model) (string, error) {
	if system.CommandExists("brew") {
		return "", nil // Already installed
	}

	brewPrefix := system.GetBrewPrefix()
	script := fmt.Sprintf(`#!/bin/bash
set -e
echo ""
echo "🍺 Installing Homebrew package manager..."
echo "   (You may be prompted for your password)"
echo ""
NONINTERACTIVE=1 /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

echo ""
echo "📝 Configuring shell to use Homebrew..."

# Add to shell configs
BREW_CONFIG='eval "$(%s/bin/brew shellenv)"'

for RC_FILE in "$HOME/.bashrc" "$HOME/.zshrc"; do
    if [ -f "$RC_FILE" ]; then
        if ! grep -q "brew shellenv" "$RC_FILE" 2>/dev/null; then
            echo "" >> "$RC_FILE"
            echo "$BREW_CONFIG" >> "$RC_FILE"
        fi
    fi
done

# Source it now
eval "$(%s/bin/brew shellenv)"

	echo ""
	echo "✅ Homebrew installed successfully!"
	echo ""
	%s
	`, brewPrefix, brewPrefix, interactiveContinuePrompt)

	return script, nil
}

// getDepsScript returns the interactive script for the dependency step. It is
// the TUI counterpart of stepInstallDeps and shares its decisions rather than
// re-deriving them: depsForInstall picks and filters the package names,
// planPlatformInstall picks the manager and the command, and this function only
// wraps that in the shell script tea.ExecProcess runs. Nothing about which
// packages or which manager is decided here.
func getDepsScript(m *Model) (string, error) {
	deps, debianGaps := depsForInstall(m)
	plan := planPlatformInstall(m, deps)

	var script strings.Builder
	script.WriteString("#!/bin/sh\nset -e\necho \"\"\n")
	script.WriteString("echo \"🔄 Installing base dependencies...\"\n")
	script.WriteString("echo \"   (You may be prompted for your password)\"\n")
	script.WriteString("echo \"\"\n")

	// The notice and the index refresh come from the plan, exactly as they do in
	// stepInstallDeps: they belong to the apt route, so a script that installs
	// through Homebrew neither names a Debian gap nor refreshes an index it will
	// not use.
	if plan.Manager == "apt-get" {
		for _, line := range debianUnavailableMessage(debianGaps) {
			fmt.Fprintf(&script, "echo %q\n", line)
		}
	}
	if plan.Update != "" {
		fmt.Fprintf(&script, "sudo %s\n", plan.Update)
	}

	switch plan.Manager {
	case "pkg":
		fmt.Fprintf(&script, "pkg install -y %s\n", plan.Packages)
	case "pacman", "dnf", "apt-get":
		fmt.Fprintf(&script, "%s\n", plan.sudoCommand())
	case "brew":
		// Run Homebrew through its prefix, exactly as runBrewWithLogs does, so the
		// script does not depend on brew being on the PATH this shell was started
		// with.
		fmt.Fprintf(&script, "%q install %s\n", filepath.Join(system.GetBrewPrefix(), "bin", "brew"), plan.Packages)
	default:
		return "", fmt.Errorf("no package manager available for this platform")
	}

	script.WriteString("echo \"\"\necho \"✅ Dependencies installed successfully!\"\necho \"\"\n")
	script.WriteString("echo \"Press Enter to continue...\"\nread -r _\n")
	return script.String(), nil
}

// getTerminalScript returns script to install terminal on Linux (needs sudo)
func getTerminalScript(m *Model) (string, error) {
	terminal := m.Choices.Terminal
	homeDir := os.Getenv("HOME")

	var installCmd string
	var configCmd string

	switch terminal {
	case "alacritty":
		if system.CommandExists("alacritty") {
			installCmd = `echo "✓ Alacritty already installed"`
		} else if m.SystemInfo.OS == system.OSArch {
			installCmd = `sudo pacman -S --noconfirm alacritty`
		} else if m.SystemInfo.OS == system.OSFedora {
			installCmd = `sudo dnf install -y alacritty`
		} else {
			// Debian/Ubuntu: compile from source (PPAs are unreliable)
			installCmd = `echo "📦 Installing build dependencies..."
sudo apt-get install -y cmake pkg-config libfreetype6-dev libfontconfig1-dev libxcb-xfixes0-dev libxkbcommon-dev python3 gzip scdoc git curl

# Install Rust if not present
if ! command -v cargo &> /dev/null && [ ! -f "$HOME/.cargo/bin/cargo" ]; then
    echo "🦀 Installing Rust/Cargo toolchain..."
    curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y
    source "$HOME/.cargo/env"
fi

# Make sure cargo is in PATH
export PATH="$HOME/.cargo/bin:$PATH"

echo "📥 Cloning Alacritty repository..."
ALACRITTY_DIR=$(mktemp -d)/alacritty
git clone https://github.com/alacritty/alacritty.git "$ALACRITTY_DIR"

echo "🔨 Building Alacritty (this may take 5-10 minutes)..."
cd "$ALACRITTY_DIR"
cargo build --release

echo "📦 Installing Alacritty binary..."
sudo cp target/release/alacritty /usr/local/bin/alacritty
sudo cp extra/linux/Alacritty.desktop /usr/share/applications/ 2>/dev/null || true

echo "🧹 Cleaning up..."
rm -rf "$ALACRITTY_DIR"
cd -

echo "✓ Alacritty built and installed from source"`
		}
		configCmd = fmt.Sprintf(`mkdir -p "%s/.config/alacritty"
cp "dotfiles/alacritty.toml" "%s/.config/alacritty/alacritty.toml"`, homeDir, homeDir)

	case "wezterm":
		if system.CommandExists("wezterm") {
			installCmd = `echo "✓ WezTerm already installed"`
		} else {
			// The install lines come from the same table stepInstallTerminal
			// executes, so the interactive path cannot be an empty script while
			// the non-interactive path installs. On Debian/Ubuntu and WSL that
			// table is the Homebrew tap and formula.
			installCmd = strings.Join(weztermInstallCommands(m.SystemInfo), "\n")
		}
		configCmd = fmt.Sprintf(`mkdir -p "%s/.config/wezterm"
cp "dotfiles/.wezterm.lua" "%s/.config/wezterm/wezterm.lua"`, homeDir, homeDir)

	case "ghostty":
		if system.CommandExists("ghostty") {
			installCmd = `echo "✓ Ghostty already installed"`
		} else if m.SystemInfo.OS == system.OSArch {
			installCmd = `sudo pacman -S --noconfirm ghostty`
		} else if m.SystemInfo.OS == system.OSFedora {
			installCmd = `sudo dnf copr enable -y pgdev/ghostty
sudo dnf install -y ghostty`
		} else {
			// Debian uses install script
			installCmd = `curl -fsSL https://raw.githubusercontent.com/mkasberg/ghostty-ubuntu/HEAD/install.sh | bash`
		}
		configCmd = fmt.Sprintf(`mkdir -p "%s/.config/ghostty"
cp -r dotfiles/dotfiles-ghostty/* "%s/.config/ghostty/"`, homeDir, homeDir)

	default:
		return "", nil
	}

	script := fmt.Sprintf(`#!/bin/sh
set -e
echo ""
echo "🖥️  Installing %s..."
echo "   (You may be prompted for your password)"
echo ""
%s
echo ""
echo "📝 Copying %s configuration..."
%s
echo ""
echo "✅ %s configured!"
echo ""
echo "Press Enter to continue..."
read -r _
`, terminal, installCmd, terminal, configCmd, terminal)

	return script, nil
}

// getSetShellScript returns script to set the default shell (needs chsh password)
func getSetShellScript(m *Model) (string, error) {
	shell := m.Choices.Shell
	var shellCmd string

	switch shell {
	case "fish":
		shellCmd = "fish"
	case "zsh":
		shellCmd = "zsh"
	case "nushell":
		shellCmd = "nu"
	default:
		return "", fmt.Errorf("unknown shell: %s", shell)
	}

	// Termux: no chsh, we modify ~/.bashrc to start the shell
	if m.SystemInfo.IsTermux {
		return getSetShellScriptTermux(shellCmd)
	}

	brewPrefix := system.GetBrewPrefix()

	script := fmt.Sprintf(`#!/bin/sh
set -e

# Add brew to PATH for this script
export PATH="%s/bin:$PATH"

SHELL_PATH=$(which %s 2>/dev/null)

if [ -z "$SHELL_PATH" ]; then
    echo "❌ Shell '%s' not found in PATH"
    echo ""
    echo "Press Enter to continue..."
    read dummy
    exit 1
fi

echo ""
echo "🐚 Setting $SHELL_PATH as your default shell..."
echo ""

# Check if shell is already in /etc/shells
if ! grep -q "^$SHELL_PATH$" /etc/shells 2>/dev/null; then
    echo "📝 Adding $SHELL_PATH to /etc/shells (requires sudo)..."
    echo "$SHELL_PATH" | sudo tee -a /etc/shells > /dev/null
fi

# Change shell
echo ""
echo "🔐 Changing default shell..."
echo "   (You may need to enter your password)"
echo ""
if chsh -s "$SHELL_PATH" 2>/dev/null; then
    echo ""
    echo "✅ Default shell changed to $SHELL_PATH"
elif sudo usermod -s "$SHELL_PATH" "$(whoami)" 2>/dev/null; then
    echo ""
    echo "✅ Default shell changed to $SHELL_PATH (via usermod)"
else
    echo ""
    echo "⚠️  Could not change default shell automatically."
    echo "   Run manually: chsh -s $SHELL_PATH"
fi

echo "   Please log out and log back in for changes to take effect."
echo ""
echo "Press Enter to continue..."
read dummy
`, brewPrefix, shellCmd, shellCmd)

	return script, nil
}

// getSetShellScriptTermux returns script to set default shell in Termux
// Termux doesn't have chsh, so we add shell launch to ~/.bashrc
func getSetShellScriptTermux(shellCmd string) (string, error) {
	script := fmt.Sprintf(`#!/data/data/com.termux/files/usr/bin/sh
set -e

SHELL_PATH=$(which %s 2>/dev/null)

if [ -z "$SHELL_PATH" ]; then
    echo "❌ Shell '%s' not found in PATH"
    echo ""
    echo "Press Enter to continue..."
    read dummy
    exit 1
fi

echo ""
echo "🐚 Setting $SHELL_PATH as your default shell in Termux..."
echo ""

# Termux doesn't have chsh, so we add to ~/.bashrc
BASHRC="$HOME/.bashrc"

# Check if already configured
if grep -q "# dotfiles shell auto-start" "$BASHRC" 2>/dev/null; then
    echo "Shell auto-start already configured in ~/.bashrc"
else
    echo "" >> "$BASHRC"
    echo "# dotfiles shell auto-start" >> "$BASHRC"
    echo "if [ -x \"$SHELL_PATH\" ] && [ -z \"\$DOTFILES_SHELL_STARTED\" ]; then" >> "$BASHRC"
    echo "    export DOTFILES_SHELL_STARTED=1" >> "$BASHRC"
    echo "    exec $SHELL_PATH" >> "$BASHRC"
    echo "fi" >> "$BASHRC"
    echo "✅ Added shell auto-start to ~/.bashrc"
fi

echo ""
echo "✅ Default shell set to $SHELL_PATH"
echo "   Close and reopen Termux for changes to take effect."
echo ""
echo "Press Enter to continue..."
read dummy
`, shellCmd, shellCmd)

	return script, nil
}

// getWSLConfigScript returns the interactive script for the WSL step. It is the
// TUI counterpart of stepInstallWSLConfig and shares that step's decisions: the
// checkout, the resolved Windows profile, the wsl.conf destination and the
// repository asset names all come from the same helpers the step uses, so the
// two cannot disagree about what to install or where.
//
// The script only exists because /etc/wsl.conf is root-owned and sudo has to be
// able to prompt, which needs the terminal the TUI suspends with
// tea.ExecProcess. It reproduces what the step installs, and the win32yank
// bridge keeps the same pinned URL, checksum and interop-health decision.
func getWSLConfigScript(m *Model) (string, error) {
	if !m.SystemInfo.IsWSL {
		return "", nil
	}

	repoDir, err := m.repoDir()
	if err != nil {
		return "", err
	}

	confDst := os.Getenv(envWSLConfPath)
	if confDst == "" {
		confDst = defaultWSLConfPath
	}

	var script strings.Builder
	script.WriteString("#!/bin/sh\n")
	script.WriteString("set -e\n\n")
	fmt.Fprintf(&script, "WSL_CONFIG_SRC=%s\n", shellSingleQuote(filepath.Join(repoDir, repoAssetWSLConfig)))
	fmt.Fprintf(&script, "WSL_CONF_SRC=%s\n", shellSingleQuote(filepath.Join(repoDir, repoAssetWSLConf)))
	fmt.Fprintf(&script, "WSL_CONF_DST=%s\n", shellSingleQuote(confDst))

	profileDir, profileErr := windowsUserProfile()
	if profileErr != nil {
		// The lookup can legitimately fail (interop disabled, unusual mount
		// layout) and that must not stop the in-distribution half, exactly as in
		// stepInstallWSLConfig.
		script.WriteString("WINDOWS_PROFILE=''\n")
		fmt.Fprintf(&script, "echo %s\n", shellSingleQuote(fmt.Sprintf(
			"Skipping .wslconfig: %v. Set %s to override the lookup.", profileErr, envWSLWindowsHome)))
	} else {
		fmt.Fprintf(&script, "WINDOWS_PROFILE=%s\n", shellSingleQuote(profileDir))
	}

	script.WriteString(`
STAMP=$(date +%Y%m%d-%H%M%S)

# install_artifact mirrors applyArtifact: back up an existing destination, write
# directly when this user may, and escalate to sudo only when the plain write is
# refused, so an unprivileged temporary destination never prompts for a password.
install_artifact() {
	src=$1
	dst=$2
	if [ ! -f "$src" ]; then
		echo "reading $src: no such file" >&2
		return 1
	fi
	if [ -e "$dst" ]; then
		backup="$dst.bak-dotfiles-$STAMP"
		if cp -a "$dst" "$backup" 2>/dev/null; then
			echo "Previous $(basename "$dst") backed up to $backup"
		elif sudo cp -a "$dst" "$backup" 2>/dev/null; then
			echo "Previous $(basename "$dst") backed up to $backup"
		else
			echo "Warning: could not back up $dst" >&2
		fi
	fi
	mkdir -p "$(dirname "$dst")" 2>/dev/null || true
	if cp "$src" "$dst" 2>/dev/null && chmod 0644 "$dst" 2>/dev/null; then
		return 0
	fi
	tmp=$(mktemp)
	cp "$src" "$tmp"
	chmod 0644 "$tmp"
	sudo install -m 0644 "$tmp" "$dst"
	rm -f "$tmp"
}

if [ -n "$WINDOWS_PROFILE" ]; then
	install_artifact "$WSL_CONFIG_SRC" "$WINDOWS_PROFILE/.wslconfig"
	echo ".wslconfig installed at $WINDOWS_PROFILE/.wslconfig"
else
	echo "Skipping .wslconfig: no Windows profile was found"
fi

install_artifact "$WSL_CONF_SRC" "$WSL_CONF_DST"
echo "wsl.conf installed at $WSL_CONF_DST"
`)

	// The clipboard bridge is deliberately not fatal, for the same reason the Go
	// step does not fail on it: the editor works without it.
	interopOK := "0"
	if peInteropHealthy() {
		interopOK = "1"
	}
	fmt.Fprintf(&script, "\nPE_INTEROP_OK=%s\n", interopOK)
	script.WriteString(`
if [ "$PE_INTEROP_OK" = "0" ]; then
	echo "Skipping win32yank: this distribution cannot execute Windows binaries stored on the Linux filesystem, so the bridge would install into a path that cannot run. Neovim will keep using wl-clipboard."
elif command -v win32yank.exe >/dev/null 2>&1; then
	echo "win32yank already installed"
else
	mkdir -p "$HOME/.local/bin" || true
	WY_ARCHIVE=$(mktemp)
	if curl -fsSL `)
	fmt.Fprintf(&script, "%s", shellSingleQuote(win32yankArchive))
	script.WriteString(` -o "$WY_ARCHIVE"; then
		WY_ACTUAL=$(sha256sum "$WY_ARCHIVE" 2>/dev/null | awk '{print $1}')
		if [ "$WY_ACTUAL" != `)
	fmt.Fprintf(&script, "%s", shellSingleQuote(win32yankSHA256))
	script.WriteString(` ]; then
			echo "win32yank checksum mismatch" >&2
		else
			WY_DIR=$(mktemp -d)
			if unzip -o -q "$WY_ARCHIVE" -d "$WY_DIR" && [ -f "$WY_DIR/win32yank.exe" ] && mv "$WY_DIR/win32yank.exe" "$HOME/.local/bin/win32yank.exe" && chmod 0755 "$HOME/.local/bin/win32yank.exe"; then
				echo "Windows clipboard bridge ready"
			else
				echo "Could not install win32yank, so Neovim will not reach the Windows clipboard" >&2
			fi
			rm -rf "$WY_DIR"
		fi
	else
		echo "Could not install win32yank, so Neovim will not reach the Windows clipboard" >&2
	fi
	rm -f "$WY_ARCHIVE"
fi

echo "Run wsl --shutdown on Windows and reopen the terminal to apply the changes"
`)

	return script.String(), nil
}

// shellSingleQuote renders s as a single-quoted POSIX shell literal, so a path
// with spaces or shell metacharacters cannot escape the assignment it is
// embedded in.
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// createTempScriptCommand creates a temporary bash script and returns a command to execute it
func createTempScriptCommand(script string) (*exec.Cmd, error) {
	// Create temp file
	tmpFile, err := os.CreateTemp("", "dotfiles-install-*.sh")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp script: %w", err)
	}

	// Write script
	if _, err := tmpFile.WriteString(script); err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("failed to write script: %w", err)
	}
	tmpFile.Close()

	// Make executable
	if err := os.Chmod(tmpFile.Name(), 0755); err != nil {
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("failed to make script executable: %w", err)
	}

	// Return command - use available shell (bash, sh, or zsh)
	shellPath := system.GetShell()
	cmd := exec.Command(shellPath, tmpFile.Name())
	cmd.Env = os.Environ()

	return cmd, nil
}
