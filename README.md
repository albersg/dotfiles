# dotfiles

📄 Read this in: **English** | [Español](README.es.md)

## Table of Contents

- [What is this?](#what-is-this)
- [Quick Start](#quick-start)
- [Supported Platforms](#supported-platforms)
- [Vim Mastery Trainer](#-vim-mastery-trainer)
- [Documentation](#documentation)
- [Tools Overview](#tools-overview)
- [Support](#support)

---

## Preview

### TUI Installer

<img width="1424" height="1536" alt="TUI Installer" src="https://github.com/user-attachments/assets/1db56d3b-a8c0-4885-82aa-c5ec04af4ac0" />

### Showcase

<img width="3840" height="2160" alt="Development Environment Showcase" src="https://github.com/user-attachments/assets/fff14c05-9676-4e04-b05e-dab5e3cf300a" />

---

## What is this?

A complete development environment configuration including:

- **Neovim** with LSP, autocompletion, and AI integration
- **Shells**: Fish, Zsh, Nushell
- **Terminal Multiplexers**: Tmux, Zellij, Herdr
- **Terminal Emulators**: Alacritty, WezTerm, Kitty, Ghostty
- **AI CLI Tools**: Claude Code and OpenCode CLI installers

## Quick Start

### Requirements

| Requirement | Details |
|-------------|---------|
| **Operating system** | macOS 10.15+; Linux (Ubuntu 20.04+, Debian, Fedora/RHEL, Arch); WSL2; or Termux |
| **Git and curl** | The installer clones this repository while it runs |
| **Internet** | To clone the repository and download packages |
| **Homebrew** | Installed automatically when missing, on macOS and Linux, except on Fedora and Arch, which keep their native package managers (`dnf` and `pacman`), and on Termux |

### Option 1: Homebrew (Recommended)

```bash
brew install albersg/tap/dotfiles
dotfiles
```

### Option 2: Direct Download

```bash
# macOS Apple Silicon
curl -fsSL https://github.com/albersg/dotfiles/releases/latest/download/dotfiles-darwin-arm64 -o dotfiles

# macOS Intel
curl -fsSL https://github.com/albersg/dotfiles/releases/latest/download/dotfiles-darwin-amd64 -o dotfiles

# Linux x86_64
curl -fsSL https://github.com/albersg/dotfiles/releases/latest/download/dotfiles-linux-amd64 -o dotfiles

# Linux ARM64 (Raspberry Pi, etc.)
curl -fsSL https://github.com/albersg/dotfiles/releases/latest/download/dotfiles-linux-arm64 -o dotfiles

# Then run
chmod +x dotfiles
./dotfiles
```

The downloaded file is not placed on your `PATH`, so it is run as `./dotfiles` from
wherever it was downloaded, and each release also publishes a `SHA256SUMS` asset to
check the download against.

### Option 3: Termux (Android)

Termux requires building locally: Android has no Homebrew and there is no published binary for it. The installer recognises Termux, installs its packages with `pkg` instead of a package manager that is not there, and writes the Nerd Font to `~/.termux/font.ttf` rather than a desktop font directory. Clone the repository, build the installer with Go, and run it from the checkout. Termux is the least exercised of the three platforms and has no step-by-step guide yet.

The TUI guides you through selecting your preferred tools and handles all the configuration automatically.

During multiplexer selection, choose **Tmux**, **Zellij**, **Herdr**, or **None**. Fish, Zsh, and Nushell are patched to auto-start the selected multiplexer on fresh interactive shells while avoiding nested sessions.

> **Tmux users:** After installation, open tmux and press `prefix + I` (capital I) to install plugins via TPM. This ensures the theme and all plugins load correctly.

> **Windows users:** You must set up WSL first. See the [Manual Installation Guide](docs/manual-installation.md#windows-wsl).

### What installing does

The installer is an interactive TUI. It detects your system and the tools you already
have, asks which shell, terminal, multiplexer and editor you want, then installs the
packages and writes the configuration.

Before replacing anything, it copies the configuration already in place into
`~/.dotfiles-backup-<timestamp>/`. Those backups are plain copies of your previous
files, so putting them back is a copy. See [Rollback Procedures](docs/ROLLBACK.md).

### Try it without changing anything

```bash
dotfiles --dry-run    # report what would be installed, change nothing
dotfiles -t           # run against a throwaway HOME, leaving yours untouched
```

### Non-interactive installation

```bash
dotfiles --non-interactive --shell=zsh --wm=herdr --nvim
```

`--shell` is required and takes `fish`, `zsh` or `nushell`. `--terminal` takes
`alacritty`, `wezterm`, `ghostty` or `none`, plus `kitty` on macOS only. `--wm`
takes `tmux`, `zellij`, `herdr` or `none`. `--nvim` and `--font` are opt-in.
`dotfiles --help` lists the rest, including `--backup=false` and the
`DOTFILES_VERBOSE=1` environment variable.

### After installing

Open a new shell. The installer writes configuration for the shell you chose but cannot
reload the shell you ran it from.

To update later, install the newer installer and run it again: `brew upgrade dotfiles`
on the Homebrew path, or download the current binary otherwise. A later run clones this
repository again, so it picks up the current configurations.

---

## Supported Platforms

| Platform | Architecture | Install Method | Package Manager |
|----------|--------------|----------------|-----------------|
| macOS | Apple Silicon (ARM64) | Homebrew, Direct Download | Homebrew |
| macOS | Intel (x86_64) | Homebrew, Direct Download | Homebrew |
| Linux (Ubuntu/Debian) | x86_64, ARM64 | Homebrew, Direct Download | Homebrew |
| Linux (Fedora/RHEL) | x86_64, ARM64 | Direct Download | dnf |
| Linux (Arch) | x86_64 | Direct Download | pacman |
| Windows | WSL | Direct Download (see docs) | Homebrew |
| Android | Termux (ARM64) | Build locally (see above) | pkg |

---

## 🎮 Vim Mastery Trainer

Learn Vim the fun way! The installer includes an interactive RPG-style trainer with:

| Module | Keys Covered |
|--------|--------------|
| 🏃 Horizontal Motions | `w, W, e, E, b, B, f, F, t, T, ;, ,, 0, $, ^` |
| 📐 Vertical Motions | `j, k, gg, G, {, }, H, M, L, ctrl+d/u/f/b` |
| 🎯 Text Objects | `viw, vaw, vi", va", vi{, diw, daw, ci", di{, yiw, yi"` |
| 🔁 Change & Repeat | `d, c, dd, D, C, x, *, #, n, N, gn, cgn, dgn, .` |
| 🔄 Substitution | `r, R, s, S, ~, gu, gU, J, :s, :%s, flags (g, c, i)` |
| 🔍 Regex & Vimgrep | `/, ?, n, N, *, #, \\v, :vimgrep, :copen, :cnext` |
| 🎪 Macros | `qa, q, @a, @@, :normal, :g/pattern/` |
| 📝 Editing & Undo | `i, a, I, A, o, O, <Esc>, u, Ctrl-r, dd, yy, p, P, >>, <<, x, D, %, marks` |
| 📋 Registers & Indentation | `yy, yiw, y$, yw, yj, p, P, "a-"z, "0, dd, x, D, >>, <<` |

Each module has 19–24 progressive lessons (with a minimum of 15 enforced), practice mode with intelligent exercise selection, boss fights, and XP tracking.

Launch it from the main menu: **Vim Mastery Trainer**

---

## Documentation

| Document | Description |
|----------|-------------|
| [TUI Installer Guide](docs/tui-installer.md) | Interactive installer features, navigation, backup/restore |
| [Manual Installation](docs/manual-installation.md) | Step-by-step manual setup for all platforms |
| [Rollback Procedures](docs/ROLLBACK.md) | Restore configurations from a backup and undo an installation |
| [Neovim Keymaps](docs/neovim-keymaps.md) | Complete reference of all keybindings |
| [AI Configuration](docs/ai-configuration.md) | Claude Code, OpenCode, Copilot, and other AI assistants |
| [Vim Trainer Spec](docs/vim-trainer-spec.md) | Technical specification for the Vim Mastery Trainer |
| [Docker Testing](docs/docker-testing.md) | E2E testing with Docker containers |
| [Contributing](docs/contributing.md) | Development setup, skills system, E2E tests, release process |

---

## Tools Overview

- **Terminal Emulators**: Ghostty, Kitty, WezTerm, Alacritty
- **Shells**: Nushell, Fish, Zsh (+ Powerlevel10k)
- **Multiplexers**: Tmux, Zellij, Herdr
- **Editor**: Neovim (LazyVim with LSP, completions, AI)
- **Prompt**: Starship

> See [Tools Reference](docs/tools.md) for detailed descriptions of each tool.

---

## Support

- **Issues**: [GitHub Issues](https://github.com/albersg/dotfiles/issues)

> This is a downstream distribution of [dotfiles](https://github.com/albersg/dotfiles). See [UPSTREAM.md](UPSTREAM.md) for upstream community links and attribution.

---

## License

MIT License - feel free to use, modify, and share.

**Happy coding!** 🧰

---

## Contributors

Thanks to everyone who has contributed to dotfiles!

[![Contributors](https://contrib.rocks/image?repo=albersg/dotfiles)](https://github.com/albersg/dotfiles/graphs/contributors)
