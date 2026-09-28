# TUI Installer

The dotfiles TUI Installer is a modern, interactive terminal application built with Go and [Bubbletea](https://github.com/charmbracelet/bubbletea) that guides you through the complete setup of your development environment.

## Table of Contents

- [Features](#features)
- [Quick Start](#quick-start)
- [Screens & Navigation](#screens--navigation)
- [Command Line Interface](#command-line-interface)
- [Backup & Restore](#backup--restore)
- [Learn Mode](#learn-mode)
- [Requirements](#requirements)
- [Troubleshooting](#troubleshooting)
- [Development](#development)

## Features

- **Interactive Navigation**: Arrow keys or Vim-style `j/k` bindings
- **Adaptive Theme**: One palette that reads on a dark or a light terminal, and that
  degrades to 16 colours or no colour without losing information
- **Shared Help Notation**: Every screen writes its keys the same way, so a legend
  cannot drift from the keys the screen accepts
- **Smart Detection**: Automatically detects your OS, existing configs, and installed tools
- **Backup & Restore**: Safely backup existing configurations before installation
- **Educational Content**: Learn about each tool before choosing (terminals, shells, multiplexers)
- **Neovim Keymaps Reference**: Built-in keymap browser organized by category
- **LazyVim Guide**: Comprehensive guide to LazyVim concepts and usage
- **Vim Trainer**: RPG-style interactive Vim learning with exercises and progression
- **Progress Tracking**: Real-time installation progress with a frame-width progress bar,
  a percentage, and a per-step status rail, plus optional detailed logs
- **Non-Interactive Mode**: CI/CD friendly installation via CLI flags

## Quick Start

### Option 1: Homebrew (Recommended)

```bash
brew install albersg/tap/dotfiles
dotfiles
```

### Option 2: Download Pre-built Binary

| Platform | Command |
|----------|---------|
| macOS Apple Silicon | `curl -fsSL https://github.com/albersg/dotfiles/releases/latest/download/dotfiles-darwin-arm64 -o dotfiles && chmod +x dotfiles && ./dotfiles` |
| macOS Intel | `curl -fsSL https://github.com/albersg/dotfiles/releases/latest/download/dotfiles-darwin-amd64 -o dotfiles && chmod +x dotfiles && ./dotfiles` |
| Linux x86_64 | `curl -fsSL https://github.com/albersg/dotfiles/releases/latest/download/dotfiles-linux-amd64 -o dotfiles && chmod +x dotfiles && ./dotfiles` |
| Linux ARM64 | `curl -fsSL https://github.com/albersg/dotfiles/releases/latest/download/dotfiles-linux-arm64 -o dotfiles && chmod +x dotfiles && ./dotfiles` |

### Option 3: Build from Source

```bash
git clone https://github.com/albersg/dotfiles.git
cd dotfiles/installer
go build -o dotfiles ./cmd/dotfiles
./dotfiles
```

## Screens & Navigation

### Main Menu

From the main menu you can access:

- **Start Installation**: Begin the guided setup process
- **Learn About Tools**: Explore terminals, shells, and multiplexers
- **Neovim Keymaps**: Browse all configured keybindings
- **LazyVim Guide**: Learn LazyVim fundamentals
- **Vim Trainer**: Practice Vim motions with interactive exercises
- **Restore from Backup**: Restore previous configurations (if backups exist)
- **Exit**: Quit the installer

### Installation Flow

Every screen is built to fit an 80×24 terminal: it never runs past the frame on
either axis, and overlong text is cut with a visible marker or wrapped rather
than clipped silently. The installer first asks for the choices (operating
system, terminal emulator, font, shell, window manager, Neovim and backup), then
runs the steps that fit that machine; the set of steps depends on the platform
and the choices, so a run shows only the ones that apply.

| Step | What it does |
|------|--------------|
| Install Dependencies | Installs the base packages the system needs (needs sudo on Linux) |
| Install Xcode CLI | Installs the Apple developer command-line tools (macOS, when missing) |
| Install Homebrew | Installs Homebrew, the package manager |
| Clone Repository | Downloads your dotfiles repository |
| Install Terminal | Installs your chosen terminal emulator |
| Install Iosevka Nerd Font | Installs the Iosevka Nerd Font for icons |
| Install Shell | Installs your shell and its plugins |
| Install Multiplexer | Installs your terminal multiplexer |
| Install Neovim | Installs Neovim with your configuration |
| Install Toolset | Installs the command-line tools declared in the `Brewfile` (best effort; skipped on Termux and without Homebrew) |
| Install Pi Agent Skills | Installs the pinned AI agent skills |
| Install OfficeCLI | Installs the OfficeCLI document tool |
| Configure WSL | Derives the `.wslconfig` limits from the Windows host it runs on and installs the rendered file, plus `/etc/wsl.conf` in the distribution (WSL hosts only) |
| Set Default Shell | Sets your shell as the default |
| Cleanup | Removes the temporary files it created |

The installing screen is the longest thing a user watches, so it shows how far
the run has come: a progress bar sized to the frame with a percentage beside it,
and a rail of one row per step whose state is a glyph and a word (`✓` done,
`●` running, `○` pending, `✗` failed, `⊘` skipped). The running step's
description is shown under its row, and `d` opens a bounded log box with the most
recent output.

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `↑` / `k` | Move up |
| `↓` / `j` | Move down |
| `Enter` / `Space` | Select option |
| `Esc` | Go back |
| `q` | Quit (when not installing) |
| `d` | Toggle details (during installation) |
| `Ctrl+C` | Force quit |

## Command Line Interface

### Basic Flags

```bash
dotfiles [flags]
```

| Flag | Shorthand | Description |
|------|-----------|-------------|
| `--help` | `-h` | Show help message |
| `--version` | `-v` | Show version information |
| `--test` | `-t` | Run in test mode (uses temporary directory) |
| `--dry-run` | | Show what would be installed without doing it |
| `--non-interactive` | | Run without TUI, use CLI flags instead |

### Non-Interactive Mode

For CI/CD or scripted installations:

```bash
dotfiles --non-interactive --shell=<shell> [options]
```

| Flag | Values | Description |
|------|--------|-------------|
| `--shell` | `fish`, `zsh`, `nushell` | Shell to install (required) |
| `--terminal` | `alacritty`, `wezterm`, `kitty`, `ghostty`, `none` | Terminal emulator |
| `--wm` | `tmux`, `zellij`, `herdr`, `none` | Window manager |
| `--nvim` | | Install Neovim configuration |
| `--font` | | Install Nerd Font |
| `--backup` | `true`/`false` | Backup existing configs (default: true) |

### Examples

```bash
# Interactive TUI (default)
dotfiles

# Non-interactive with Fish + Herdr + Neovim
dotfiles --non-interactive --shell=fish --wm=herdr --nvim

# Test mode with Zsh + Tmux (no terminal, no nvim)
dotfiles --test --non-interactive --shell=zsh --wm=tmux

# Dry run to preview changes (installs nothing)
dotfiles --dry-run

# Verbose output (shows all command logs)
DOTFILES_VERBOSE=1 dotfiles --non-interactive --shell=fish --nvim
```

## Backup & Restore

### Automatic Backup Detection

The installer automatically detects existing configurations for:

| Tool | Paths |
|------|-------|
| Neovim | `~/.config/nvim` |
| Fish | `~/.config/fish` |
| Zsh | `~/.zshrc`, `~/.oh-my-zsh` |
| Nushell | `~/.config/nushell`, `~/Library/Application Support/nushell` |
| Tmux | `~/.tmux.conf`, `~/.tmux` |
| Zellij | `~/.config/zellij` |
| Herdr | `~/.config/herdr` |
| Alacritty | `~/.config/alacritty` |
| WezTerm | `~/.config/wezterm`, `~/.wezterm.lua` |
| Kitty | `~/.config/kitty` |
| Ghostty | `~/.config/ghostty` |
| Starship | `~/.config/starship.toml` |

### Backup Location

Backups are stored in your home directory with a timestamp:

```
~/.dotfiles-backup-YYYY-MM-DD-HHMMSS/
```

Directory-based configs such as `~/.oh-my-zsh` are backed up recursively, alongside single-file configs like `~/.zshrc`.

### Restoring a Backup

1. Select "Restore from Backup" from the main menu
2. Choose the backup you want to restore
3. Confirm the restoration
4. Your previous configurations will be restored

## Learn Mode

The installer includes educational content to help you understand each tool:

### Terminals

| Terminal | Description |
|----------|-------------|
| Ghostty | GPU-accelerated, native, fast |
| Kitty | Feature-rich, GPU-based |
| WezTerm | Lua-configurable, cross-platform |
| Alacritty | Minimal, Rust-based |

### Shells

| Shell | Description |
|-------|-------------|
| Nushell | Structured data, modern syntax |
| Fish | User-friendly, great defaults |
| Zsh | Highly customizable, POSIX-compatible |

### Multiplexers

| Multiplexer | Description |
|-------------|-------------|
| Tmux | Battle-tested, widely used |
| Zellij | Modern, WebAssembly plugins |
| Herdr | Agent-focused multiplexer for AI coding sessions |

### Neovim

- LazyVim configuration
- LSP setup
- AI assistants (OpenCode, Claude, Copilot, etc.)

## Requirements

| Requirement | Details |
|-------------|---------|
| **macOS** | 10.15+ |
| **Linux** | Ubuntu 20.04+, Debian, Fedora/RHEL, Arch |
| **Termux** | Android terminal emulator |
| **Homebrew** | Will be installed if missing (macOS/Linux, except Fedora) |
| **Git and curl** | Git for cloning the repository; curl for downloading packages and installers |
| **Internet** | For downloading packages |

## Troubleshooting

### Installation Fails

1. Press `d` during installation to view detailed logs
2. Ensure you have internet connectivity
3. Try running with `--test` flag first to verify detection
4. Check if Homebrew is properly installed: `brew --version`

### Backup Not Showing

Backups must be in your home directory with the format:

```
~/.dotfiles-backup-*
```

### Font Not Displaying Correctly

1. Ensure the terminal is using "Iosevka Term Nerd Font"
2. Restart your terminal after font installation
3. On macOS, you may need to manually select the font in terminal preferences

### Copy/Paste Not Working in Zellij (Linux)

If you can't copy text from the terminal when using Zellij on Linux:

1. Edit `~/.config/zellij/config.kdl`
2. Uncomment the appropriate line for your system:
   - **X11**: `copy_command "xclip -selection clipboard"` AND `copy_clipboard "primary"`
   - **Wayland**: `copy_command "wl-copy"`

See: [Zellij FAQ](https://zellij.dev/documentation/faq.html#copy--paste-isnt-working-how-can-i-fix-this)

### Fish/Zsh Fails in WSL with "missing or unsuitable terminal"

When using WezTerm on Windows with WSL, Fish or Zsh may fail with:
```
missing or unsuitable terminal: wezterm
```

**Status**: ✅ Fixed automatically in the default `.wezterm.lua` config (v2.7.7+).

If you're using an older config, update your `.wezterm.lua` to include the auto-detection:
```lua
if wezterm.target_triple:find("windows") then
  config.term = "xterm-256color"
else
  config.term = "wezterm"
end
```

## Development

The TUI installer is built with:

| Component | Description |
|-----------|-------------|
| **Go 1.25+** | Programming language |
| **Bubbletea** | Terminal UI framework |
| **Lipgloss** | Styling library |
| **Teatest** | Golden file testing |

### Running Tests

```bash
cd installer
go test ./... -v
```

### Updating Golden Files

```bash
cd installer
go test ./internal/tui/... -update
```

### Project Structure

```
installer/
├── cmd/
│   └── dotfiles/
│       └── main.go              # Entry point with CLI parsing
├── internal/
│   ├── system/
│   │   ├── detect.go            # OS/tool detection
│   │   └── exec.go              # Command execution, file ops, backups
│   └── tui/
│       ├── model.go             # App state, screens, choices
│       ├── update.go            # Event handlers
│       ├── view.go              # UI rendering
│       ├── installer.go         # Installation steps
│       ├── interactive.go       # TUI mode logic
│       ├── non_interactive.go   # CLI mode logic
│       ├── styles.go            # dotfiles theme colors
│       ├── tools_info.go        # Tool descriptions
│       ├── keymaps_*.go         # Keymap definitions
│       └── trainer/             # Vim Trainer RPG system
│           ├── types.go         # Exercise types, modules, progress
│           ├── exercises.go     # Module registry, lessons and bosses
│           ├── exercises_*.go   # The exercise corpus per module
│           ├── simulator.go     # Motion simulator (the motion judge)
│           ├── editor.go        # Mutable editing engine (the buffer judge)
│           ├── validation.go    # Chooses and runs the exercise's judge
│           ├── stats.go         # Progress tracking
│           ├── gamestate.go     # Lesson, practice and boss sessions
│           └── practice.go      # Weighted practice selection
└── go.mod
```
