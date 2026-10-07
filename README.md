# dotfiles

> A dotfiles manager and TUI installer for a complete terminal development
> environment. It detects your system, asks which shell, terminal, multiplexer and
> editor you want, then installs the packages and writes the configuration for you.

📄 Read this in: **English** | [Español](README.es.md)

## Table of Contents

- [What is this?](#what-is-this)
- [Quick Start](#quick-start)
- [Supported Platforms](#supported-platforms)
- [Installer Features](#installer-features)
- [Vim Mastery Trainer](#-vim-mastery-trainer)
- [Documentation](#documentation)
- [Tools Overview](#tools-overview)
- [Support](#support)
- [License](#license)
- [Contributors](#contributors)

---

## What is this?

A complete development environment configuration including:

- **Neovim** with LSP, autocompletion, and AI integration
- **Shells**: Fish, Zsh, Nushell
- **Terminal Multiplexers**: Tmux, Zellij, Herdr
- **Terminal Emulators**: Alacritty, WezTerm, Kitty, Ghostty
- **AI CLI Tools**: Claude Code and OpenCode CLI installers

---

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

The display is adjustable too. Animation, mouse and sprite are each both a flag
and an environment variable, so a script can pin them: `--no-anim`
(`DOTFILES_ANIM=0`) stops the motion, `--no-mouse` (`DOTFILES_MOUSE=0`) gives
selection and scrolling back to the terminal, and `--no-sprite`
(`DOTFILES_SPRITE=0`) draws the ASCII cat instead of the shaded creature.
Synchronized output has no flag and is turned off with `DOTFILES_SYNC=0` only.
Animation turns itself off when the output is not a terminal or `TERM=dumb`;
mouse reporting and synchronized output are separate gates, both off on a
non-terminal, with mouse reporting also off on Termux (`DOTFILES_MOUSE=1`
overrides that default).

Run `dotfiles --help` for the complete list, including `--dry-run`, `--backup=false`
and the `DOTFILES_VERBOSE=1` environment variable.

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

## Installer Features

Alongside the installation itself, the TUI ships two features you can come back to:
a **Utilities** section for small, reversible jobs and an interactive **Vim Mastery
Trainer**. The [TUI Installer Guide](docs/tui-installer.md) covers both in full.

### Utilities

The **Utilities** section holds the small jobs that are not part of an installation.
Open it from the main menu's **Utilities** row, or press `u` as a shortcut.

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

**Switch the dotfiles' own theme.** The dotfiles ship a palette library — six complete themes that
cover every tool the switch paints — and each theme's palette is defined once in
[`themes/`](themes/) rather than written by hand in each config. A theme is a file there, so adding
one adds a row to the theme list, which the Utilities section's **Change the dotfiles theme** row
opens. The switch applies a complete theme (a definition with every canonical role and the `[syntax]`
members the preview reads: **dotfiles**, **Catppuccin Mocha**, **Catppuccin Latte**, **Kanagawa**,
**Everforest** and **Rosé Pine**) to the configs this repository owns for all twelve tools, records
the exact bytes it replaced in the same `theme.json`, and can put them back with a dedicated undo
row. bat selects its theme by the custom theme file's name, so the switch exports that name rather
than the one written inside the file. Every one of the six paints every tool the switch names —
Alacritty, Kitty, WezTerm, Ghostty, Starship, the zsh line editor, the p10k prompt, Herdr, fish,
bat, Neovim and tmux — and a guard keeps it that way: a theme that could not paint one would name it
on its row rather than leaving it on the old palette silently, and a theme missing roles, or the
syntax the preview needs to show it, is reported as partial and never offered. Where a tool reads roles the palette does not
carry (Starship's prompt roles, fish's eighteen, bat's scopes, tmux's style options, Neovim's
highlight groups) the values are derived from the theme's own palette by the fixed mapping recorded
in [`themes/README.md`](themes/README.md), so a derived colour is one the theme already holds. The
definitions are read from a checkout on disk, never from
the binary: `$DOTFILES_DIR` first, then the clone this run makes, then the working directory and its
parents, then `~/dotfiles` and `~/.dotfiles`. Launching the installer from inside the checkout shows
the rows immediately; when none of those holds definitions the section says the switch is not
available here rather than drawing a row that fails. Move the
cursor onto a theme row and **the whole installer repaints itself in that theme's colours** — a live
preview built from the same definition the apply writes, labelled `Preview (nothing applied)` — and
leaving the row puts the default chrome back; nothing is written while the cursor moves. It only rewrites files carrying the `dotfiles-managed-config:`
ownership marker; a file you wrote is left exactly as it is. A managed file that predates the marker
is adopted first: when its whole content proves it is ours (it is what the repository ships, or it
carries a generated block marker) only that marker line is added, and when only a region is ours —
the file has drifted but still carries the anchors the generator knows — **only the bytes between
those anchors are rewritten and the rest of the file is left untouched**; either way Undo puts the
original back byte-for-byte, and a file that proves neither is refused with a message that names the
proofs it tried and the way forward. A managed file that carries the ownership marker but has neither
the generated block nor a region to rewrite is named, not skipped in silence: the switch applies the
files it can, names each one it left alone, and says that reinstalling the dotfiles refreshes it,
rather than reporting a change it did not make. The **Refresh outdated theme files** row brings an old file
forward as a named, preserved change — it names every file it would touch and where an unowned one is
preserved (into `~/.zshrc.d/` or `~/.config/fish/dotfiles.d/`, or beside itself as
`<path>.bak-dotfiles-<timestamp>`) before anything is written — records the previous bytes so Undo
puts each file back, and skips a file it cannot recognize while carrying on with the rest.
`--dry-run` skips it too. See
[`themes/README.md`](themes/README.md).

**Adjust the WSL resources.** On a WSL host, **Adjust the WSL resources** opens the memory,
processors and swap the WSL 2 VM may use. The screen prints the Windows host's real RAM and logical
processor count, recommends values from them — half the host's RAM rounded down to 512 MB (never so
much that Windows keeps under 2 GiB), every logical CPU, and a quarter of that memory for swap — and
lets you move each value with **←/→** or put everything back on the recommendation with **`r`**. It
is the same values and the same writer the installation step uses, not a second calculation, and it
is offered only where there is a `.wslconfig` to edit: elsewhere the section says why. A write only
touches the keys [`dotfiles-wsl/.wslconfig.tmpl`](dotfiles-wsl/.wslconfig.tmpl) manages — the rest of
your file, its comments and your own keys are kept exactly as they are — and the previous file is
copied beside itself as `.wslconfig.bak-dotfiles-<timestamp>` first. WSL reads the file when the VM
starts, so the screen says to run `wsl --shutdown` on Windows to apply the change and deliberately
does not run it; `--dry-run` skips the write.

**Measure the shell's startup.** The fourth utility is the one that **changes nothing**. It starts
the login shell named by `$SHELL` the way a terminal does — `zsh -i -c exit`, or the same shape for
your shell — **five times**, and reports the **median** with the **range** and every completed start,
so the number is a measurement and not a single run: the method travels with it and it can be
reproduced by hand. Every start is bounded by a **ten-second timeout**, because a startup can hang on
a plugin that waits for the network and the utility must not hang with it — a start that does not
finish is reported as a timeout and left out of the median, never written down as `0.0 s`. On zsh one
further start is taken under `zmodload zsh/zprof`, and the functions zprof blames are named heaviest
first with the total, the self time and the call count; when `zprof` is unavailable, when the shell is
not zsh, or when the profiled start reports no table, the screen says **only the total is
measurable** and gives the reason rather than inventing a culprit. Nothing is written: no startup
file is touched and no plugin is disabled — the wrappers the profiled start needs live in a
temporary directory that is removed again, and your own files are sourced from where they already
were. Where it cannot measure — no `$SHELL`, no terminal — the section says so and why.

**Report the terminal capabilities.** The read-only report describes **the terminal this run is
in** and what each answer means for the theme and the interface: the colour depth (**truecolor**,
**256**, **16** or **none**), whether **OSC 52** works (what makes a copy reach your clipboard over
SSH), whether **synchronized output** (DECSET mode 2026) is supported (what stops a redraw tearing),
and whether a **Nerd Font** is installed. Every answer names **where it came from** — the exact
environment variable (`COLORTERM`, `TERM`, `TERM_PROGRAM`), the terminal's own reply to a bounded
query, or **not determined** — and an answer the probe could not determine is reported as **unknown**
with the reason and, where a person can settle it, a way to check by hand, never as an invented "no".
If the colour depth is not truecolor the report says the themes will look approximate; if
synchronized output is not supported it says a large redraw may flicker or tear. The probe runs only
when you open the screen (never at startup), writes nothing, and its terminal query is bounded by a
timeout so a terminal that never answers cannot freeze the interface. See the
[TUI Installer Guide](docs/tui-installer.md#the-terminal-capabilities).

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
| [TUI Installer Guide](docs/tui-installer.md) | Interactive installer features, utilities, themes, navigation, backup/restore |
| [Manual Installation](docs/manual-installation.md) | Step-by-step manual setup for all platforms |
| [Rollback Procedures](docs/ROLLBACK.md) | Restore configurations from a backup and undo an installation |
| [Neovim Keymaps](docs/neovim-keymaps.md) | Complete reference of all keybindings |
| [AI Configuration](docs/ai-configuration.md) | Claude Code, OpenCode, Copilot, and other AI assistants |
| [Vim Trainer Spec](docs/vim-trainer-spec.md) | Technical specification for the Vim Mastery Trainer |
| [Tools Reference](docs/tools.md) | Descriptions of every tool the installer configures |
| [Docker Testing](docs/docker-testing.md) | E2E testing with Docker containers |
| [Contributing](docs/contributing.md) | Development setup, skills system, E2E tests, release process |

### Before you push

There are two local checks, and they answer different questions.

- **`make check`** is the inner loop. It runs `gofmt`, `go vet`, and the tests for the packages this branch changes, with Go's test cache left on. Run it after every edit: it is the check that tells you quickly whether what you just wrote still builds and still passes.
- **`make preflight`** is the full gate. It runs here, in CI's order, what CI would otherwise report after a full cycle - `gofmt`, `go vet`, `go build` and the `--help` smoke test, the whole Go test suite, `shellcheck`, the branding audit and a gitleaks scan of the commits your branch adds - and prints the CI job each step mirrors. Run it once, before pushing. CI still runs the full matrix on the push itself; `make preflight` is what keeps a CI cycle from being spent on a failure a local run would have caught.

Both live in [scripts/preflight.sh](scripts/preflight.sh). `make preflight` stops at the first failure with the failing command named and exits non-zero, and it names the jobs it cannot run locally (the Docker E2E matrix, Termux, and the macOS toolchain). It needs `go`, `gofmt`, `git`, `ripgrep`, `shellcheck` and `gitleaks`; `brew bundle` installs the last three. `make check` needs only `go`, `gofmt` and `git`.

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

---

## License

MIT License - feel free to use, modify, and share.

**Happy coding!** 🧰

---

## Contributors

Thanks to everyone who has contributed to dotfiles!

[![Contributors](https://contrib.rocks/image?repo=albersg/dotfiles)](https://github.com/albersg/dotfiles/graphs/contributors)
