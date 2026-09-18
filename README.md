# dotfiles

📄 Read this in: **English** | [Español](README.es.md)

## Table of Contents

- [What is this?](#what-is-this)
- [Quick Start](#quick-start)
- [Supported Platforms](#supported-platforms)
- [Vim Mastery Trainer](#-vim-mastery-trainer)
- [Documentation](#documentation)
- [Tools Overview](#tools-overview)
- [Bleeding Edge](#bleeding-edge)
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

### Option 3: Termux (Android)

Termux requires building locally. See the [Termux Installation Guide](docs/manual-installation.md#termux) for full instructions.

The TUI guides you through selecting your preferred tools and handles all the configuration automatically.

During multiplexer selection, choose **Tmux**, **Zellij**, **Herdr**, or **None**. Fish, Zsh, and Nushell are patched to auto-start the selected multiplexer on fresh interactive shells while avoiding nested sessions.

> **Tmux users:** After installation, open tmux and press `prefix + I` (capital I) to install plugins via TPM. This ensures the theme and all plugins load correctly.

> **Windows users:** You must set up WSL first. See the [Manual Installation Guide](docs/manual-installation.md#windows-wsl).

---

## Supported Platforms

| Platform | Architecture | Install Method | Package Manager |
|----------|--------------|----------------|-----------------|
| macOS | Apple Silicon (ARM64) | Homebrew, Direct Download | Homebrew |
| macOS | Intel (x86_64) | Homebrew, Direct Download | Homebrew |
| Linux (Ubuntu/Debian) | x86_64, ARM64 | Homebrew, Direct Download | Homebrew |
| Linux (Fedora/RHEL) | x86_64, ARM64 | Direct Download | dnf |
| Linux (Arch) | x86_64 | Homebrew, Direct Download | Homebrew |
| Windows | WSL | Direct Download (see docs) | Homebrew |
| Android | Termux (ARM64) | Build locally (see above) | pkg |

---

## 🎮 Vim Mastery Trainer

Learn Vim the fun way! The installer includes an interactive RPG-style trainer with:

| Module | Keys Covered |
|--------|--------------|
| 🔤 Horizontal Movement | `w`, `e`, `b`, `f`, `t`, `0`, `$`, `^` |
| ↕️ Vertical Movement | `j`, `k`, `G`, `gg`, `{`, `}` |
| 📦 Text Objects | `iw`, `aw`, `i"`, `a(`, `it`, `at` |
| ✂️ Change & Repeat | `d`, `c`, `dd`, `cc`, `D`, `C`, `x` |
| 🔄 Substitution | `r`, `R`, `s`, `S`, `~`, `gu`, `gU`, `J` |
| 🎬 Macros & Registers | `qa`, `@a`, `@@`, `"ay`, `"+p` |
| 🔍 Regex/Search | `/`, `?`, `n`, `N`, `*`, `#`, `\v` |

Each module includes 15 progressive lessons, practice mode with intelligent exercise selection, boss fights, and XP tracking.

Launch it from the main menu: **Vim Mastery Trainer**

---

## Documentation

| Document | Description |
|----------|-------------|
| [TUI Installer Guide](docs/tui-installer.md) | Interactive installer features, navigation, backup/restore |
| [Manual Installation](docs/manual-installation.md) | Step-by-step manual setup for all platforms |
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

## Bleeding Edge

Want the latest experimental features from my daily workflow (macOS only)?

Check out the [`nix-migration` branch](https://https://github.com/albersg/dotfiles/tree/nix-migration).

This branch contains cutting-edge configurations that eventually make their way to `main` once stable.

---

## Support

- **Issues**: [GitHub Issues](https://github.com/albersg/dotfiles/issues)

> This is a downstream distribution of [dotfiles](https://github.com/albersg/dotfiles). See [UPSTREAM.md](UPSTREAM.md) for upstream community links and attribution.

---

## License

MIT License - feel free to use, modify, and share.

**Happy coding!** 🎩

---

## Contributors

Thanks to everyone who has contributed to dotfiles!

[![Contributors](https://contrib.rocks/image?repo=albersg/dotfiles)](https://github.com/albersg/dotfiles/graphs/contributors)
