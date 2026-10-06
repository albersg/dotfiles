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

### Utilities

The installer also has a **Utilities** section for the small jobs that are not part of an
installation. Open it from the main menu's **Utilities** row, or press `u` as a shortcut.

**Switch the desktop's theme.** The first utility changes the system light/dark theme through
the desktop's own tool — `gsettings` on GNOME, `plasma-apply-colorscheme` on KDE Plasma,
`defaults` on macOS — and it only offers a desktop whose setting it can read back exactly as
it can write it. Before it changes anything it records the setting that was there in
`$XDG_STATE_HOME/dotfiles/theme.json` (or `~/.local/state/dotfiles/theme.json`), so
**Undo the last theme change** can put it back; a setting it cannot safely restore is left
alone and the reason is shown. A host with no desktop — a server, Termux, a bare terminal —
is told so rather than offered a switch that would fail. `--dry-run` skips the utility, like
every installation step. See the [TUI Installer Guide](docs/tui-installer.md#utilities) for
the exact files it touches.

**Switch the dotfiles' own theme.** The dotfiles ship one palette — the one the terminals, the
prompt, `bat`, `fish`, tmux and Herdr all read — and that palette is now defined once in [`themes/`](themes/)
rather than written by hand in each config. A theme is a file there, so adding one adds a row to the
theme list, which the Utilities section's **Change the dotfiles theme** row opens. The
switch applies a complete theme (a definition with every canonical role: today **dotfiles** and
**Catppuccin Mocha**) to the terminal, Starship, shell-prompt, Herdr, bat, fish, tmux and Neovim
configs this
repository owns, records the exact bytes it replaced in the same `theme.json`, and can put them back
with a dedicated undo row. Each theme row names the tools it leaves out (today Neovim for dotfiles;
Catppuccin Mocha leaves out nothing), so nothing is left on the old palette without being said. A
theme missing roles
is reported as partial and never offered. The definitions are read from a checkout on disk, never from
the binary: `$DOTFILES_DIR` first, then the clone this run makes, then the working directory and its
parents, then `~/dotfiles` and `~/.dotfiles`. Launching the installer from inside the checkout shows
the rows immediately; when none of those holds definitions the section says the switch is not
available here rather than drawing a row that fails. Move the
cursor onto a theme row and **the whole installer repaints itself in that theme's colours** — a live
preview built from the same definition the apply writes, labelled `Preview (nothing applied)` — and
leaving the row puts the default chrome back; nothing is written while the cursor moves. A file installed before those markers existed — the case on a machine set up by an older checkout — is brought
forward by the **Refresh outdated theme files** row, which names every file it would touch and where
an unowned one is preserved (into `~/.zshrc.d/` or `~/.config/fish/dotfiles.d/`, or beside itself as
`<path>.bak-dotfiles-<timestamp>`) before anything is written, records the previous bytes so Undo puts
each file back, and skips a file it cannot recognize while carrying on with the rest. It only rewrites files carrying the `dotfiles-managed-config:`
ownership marker, or files the refresh has adopted; a file you wrote is left exactly as it is. `--dry-run` skips it too. See
[`themes/README.md`](themes/README.md).

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

### Before you push

There are two local checks, and they answer different questions.

- **`make check`** is the inner loop. It runs `gofmt`, `go vet`, and the tests for the packages this branch changes, with Go's test cache left on. Run it after every edit: it is the check that tells you quickly whether what you just wrote still builds and still passes.
- **`make preflight`** is the full gate. It runs here, in CI's order, what CI would otherwise report after a full cycle - `gofmt`, `go vet`, `go build` and the `--help` smoke test, the whole Go test suite, `shellcheck`, the branding audit and a gitleaks scan of the commits your branch adds - and prints the CI job each step mirrors. Run it once, before pushing. CI still runs the full matrix on the push itself; `make preflight` is what keeps a CI cycle from being spent on a failure a local run would have caught.

Both live in [scripts/preflight.sh](scripts/preflight.sh). `make preflight` stops at the first failure with the failing command named and exits non-zero, and it names the jobs it cannot run locally (the Docker E2E matrix, Termux, and the macOS toolchain). It needs `go`, `git`, `ripgrep`, `shellcheck` and `gitleaks`; `brew bundle` installs the last three. `make check` needs only `go`, `gofmt` and `git`.

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
