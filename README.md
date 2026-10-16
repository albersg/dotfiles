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

> **Herdr users:** The installer installs Herdr's plugin set itself — `aigora.lantern` (`aigorahub/herdr-lantern`) and `herdr.auto-title` (`kryptamine/herdr-auto-title`) — with `herdr plugin install <owner>/<repo> --yes`, then updates them with `herdr plugin update --yes` when your Herdr can (0.9.4 and later), and verifies each with `herdr plugin list`, reporting the revision Herdr recorded. `herdr.auto-title` builds from Go at install time. On Herdr 0.9.1–0.9.3, whose plugin surface has no update command, the step names the version it found and says the plugins were installed but not updated — upgrading Herdr and re-running is what updates them. It never removes a plugin you added yourself. See [Tools Reference](docs/tools.md#plugins) for the commands.

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

To update later, ask the installer itself:

```bash
dotfiles --check-update    # dotfiles v0.5.0 (9f3c2a1, 2026-10-04)
                           # Up to date with v0.5.1 (2h ago)
dotfiles --self-update     # installs the published release over this binary
```

`--check-update` exits `0` when your build is the published one, `1` when a newer release
exists, and `2` when it could not reach GitHub at all — three different facts, so a script
never reads a network failure as a green light. `--self-update` (and the main menu's
**Update this installer** button, offered when a newer release is published) downloads the
release asset, checks it against the
`SHA256SUMS` that release publishes, keeps the binary it replaced as
`<binary>.previous`, and only then moves the new one into place with a single rename.

The check follows one of two channels. The default, **stable**, is the latest published
release. **dev** is the highest-versioned pre-release, which is the artifact the release workflow
builds from the tag it was started on; the installer cannot build from source, so a
pre-release is what a build of `main` is served as -- the installer takes the
highest-versioned pre-release GitHub offers and does not verify that it came from `main`. Choose a channel with
`--channel stable|dev` (it applies to `--check-update`, `--self-update` and the TUI) or by
pressing the main menu's **Update channel** row, which appears once a check has named one.
The choice is remembered in the same `update-check.json` the answers live in, and every
answer — including a failed check — records which channel it came from. A `dev` run with
no network says the check could not be made; it never serves the stable release under a
`dev` name.

One case it refuses, on purpose: if a package manager installed this binary, the file is
not the installer's to replace. It says so and names the command that does own the update —
`brew upgrade dotfiles` on the Homebrew path. Replacing a file another program manages is
how a package manager and an installer start disagreeing about what is installed.

A later run still clones this repository again, so it picks up the current configurations
whatever the binary's version.

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

### Updating the installer

The installer is a release of its own. When **dotfiles** opens it checks in the background
which release the chosen channel offers, remembers the answer in
`$XDG_STATE_HOME/dotfiles/update-check.json` (or `~/.local/state/dotfiles/`), and offers the
main menu's **Update this installer** button only when a newer release exists and the file
is the installer's own to replace. Pressing it downloads the release asset, checks it against
the `SHA256SUMS` that release publishes, keeps the binary it replaced as
`<binary>.previous`, and moves the new one into place with a single rename; the result is
reported on the main menu, where the button was, and the new binary runs after a restart. A
check that could not reach GitHub is recorded as unknown and never shown as an error or as
up to date. The refusal described in [After installing](#after-installing) is the whole
answer when a package manager owns the file, which is why no button is offered there.

Beside it, the **Update channel** row switches between **stable** (the latest published
release, the default) and **dev** (the highest-versioned pre-release the workflow built from its tag).
The row appears once the record names a channel — every check writes one, success or failure
— and pressing it writes the new choice to the same record and asks for a fresh answer from
that channel. `--channel stable|dev` is the scriptable form, and `--check-update` and
`--self-update` use the channel they were given.

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

**Switch the dotfiles' own theme.** The dotfiles ship a palette library — seven complete themes that
cover every tool the switch paints — and each theme's palette is defined once in
[`themes/`](themes/) rather than written by hand in each config. A theme is a file there, so adding
one adds a row to the theme list, which the Utilities section's **Change the dotfiles theme** row
opens. The switch applies a complete theme (a definition with every canonical role and the `[syntax]`
members the preview reads: **dotfiles**, **Catppuccin Mocha**, **Catppuccin Latte**, **Kanagawa**,
**Everforest**, **Rosé Pine** and **Nocturne**) to the configs this repository owns for all thirteen tools, records
the exact bytes it replaced in the same `theme.json`, and can put them back with a dedicated undo
row. Writing a file is not the same as the running tool showing it, so after the switch it brings the
theme forward where it safely can — rebuilding bat's theme cache, sourcing tmux's config into a
server that is already running, and asking a Herdr the installer is running inside to reload its config
over that session's own socket — and it lists every tool it painted with what, if anything, the user
still has to reload; a tool that is only reachable from a live session is named, never signalled. bat selects its theme by the custom theme file's name, so the switch exports that name rather
than the one written inside the file. Neovim is named by a colorscheme, and that name resolves on the machine or the line is not written: a generated colorscheme is installed into `~/.config/nvim/colors/<name>.lua` — where Neovim reads it on its runtimepath — before the switch writes the line, and a plugin's colorscheme is named only when the plugin is installed, so the switch never starts Neovim with `E185: Cannot find color scheme`. The default is chosen by the same rule: the committed line names the **dotfiles** palette, whose colorscheme file the Neovim step copies with the rest of the config, so a fresh install always has it, where a default naming a plugin's colorscheme only resolves on a machine that installed that plugin. The statusline follows the same rule from the other side: its theme is derived from whatever colorscheme is active (LazyVim sets lualine's `auto`) and never hard-coded, so it cannot report a theme it could not find. Every one of the seven paints every tool the switch names —
Alacritty, Kitty, WezTerm, Ghostty, Starship, the zsh line editor, the p10k prompt, Herdr, fish,
bat, Neovim, tmux and nushell — and a guard keeps it that way: a theme that could not paint one would name it
on its row rather than leaving it on the old palette silently, and a theme missing roles, or the
syntax the preview needs to show it, is reported as partial and never offered. Where a tool reads roles the palette does not
carry (Starship's prompt roles, fish's eighteen, bat's scopes, tmux's style options, Neovim's
highlight groups, nushell's own record and `LS_COLORS` table) the values are derived from the theme's own palette by the fixed mapping recorded
in [`themes/README.md`](themes/README.md), so a derived colour is one the theme already holds. The
definitions are read from a checkout on disk, never from
the binary: `$DOTFILES_DIR` first, then the clone this run makes, then the working directory and its
parents, then `~/dotfiles` and `~/.dotfiles`, and last the per-user copy the installer leaves under
`~/.local/share/dotfiles/` (or `$XDG_DATA_HOME/dotfiles/`). That copy is made at install time out of
the temporary clone, because the clone is removed when the run finishes and a user who launches the
program from their home directory is not inside a checkout; it is refreshed on each install, only
files the repository ships are written, a definition that already matches is left as it is, and the
clone step says how many it wrote. Resolution reads the first candidate that holds `themes/*.toml`
and never merges two, so a stale copy cannot mix two versions into one list. Launching the installer
from inside the checkout shows the rows immediately; when none of those holds definitions the section
says the switch is not available here, names the search and the copy directory, rather than drawing a
row that fails. Move the
cursor onto a theme row and **the whole installer repaints itself in that theme's colours** — a live
preview built from the same definition the apply writes, labelled `Preview (nothing applied)` — and
leaving the row puts the chrome the run wears back; nothing is written while the cursor moves.
**Pressing Enter applies the theme and the interface keeps those colours**, and a run that is
reopened paints its first frame with the theme the record in `theme.json` names, so the applied theme
is remembered; **Undo puts the interface back to the default chrome**. The applied palette is only
painted where the terminal reports truecolour — a 256-, sixteen- or no-colour terminal keeps the
installer's own adaptive chrome — and the labels on the filled blocks stay readable against the
theme's own palette. It only rewrites files carrying the `dotfiles-managed-config:`
ownership marker; a file you wrote is left exactly as it is. A managed file that predates the marker
is adopted first: when its whole content proves it is ours (it is what the repository ships, or it
carries a generated block marker) only that marker line is added, and when only a region is ours —
the file has drifted but still carries the anchors the generator knows — **only the bytes between
those anchors are rewritten and the rest of the file is left untouched**; either way Undo puts the
original back byte-for-byte, and a file that proves neither is left exactly as it is: the switch
keeps going, themes every file it can prove it owns, and reports the run as **partial**, naming
every file it left alone, every proof it tried and the way forward rather than stopping at the first
one. A managed file that carries the ownership marker but has neither the generated block nor a
region to rewrite is named the same way, not skipped in silence: the switch applies the files it
can, names each one it left alone, and names what refreshes it, rather than reporting a change it
did not make. A partial run is never reported as a whole one. The **Refresh outdated theme files** row brings an old file
forward as a named, preserved change — it names every file it would touch and where an unowned one is
preserved (into `~/.zshrc.d/` or `~/.config/fish/dotfiles.d/`, or beside itself as
`<path>.bak-dotfiles-<timestamp>`) before anything is written — records the previous bytes so Undo
puts each file back, and skips a file it cannot recognize while carrying on with the rest.
`--dry-run` skips it too. See
[`themes/README.md`](themes/README.md).

**Adjust the WSL resources.** On a WSL host, **Adjust the WSL resources** opens the memory,
processors and swap the WSL 2 VM may use. The screen prints the Windows host's real RAM and logical
processor count, recommends values from them — three quarters of the host's RAM rounded down to 512 MB
(never so much that Windows keeps less than 4 GiB, and never below half the host), every logical CPU
except one, and a quarter of that memory for swap — and lets you move each value with **←/→** or put
everything back on the recommendation with **`r`**. It
is the same values and the same writer the installation step uses, not a second calculation, and it
is offered only where there is a `.wslconfig` to edit: elsewhere the section says why. The template
it renders is read from a checkout when one is present — `$DOTFILES_DIR`, the clone, the working
directory and its parents, `~/dotfiles` or `~/.dotfiles` — and otherwise from the copy the installer
leaves under `~/.local/share/dotfiles/` (or `$XDG_DATA_HOME/dotfiles/`), so the row is offered from
any working directory, not only from inside a checkout. A write only
touches the keys [`dotfiles-wsl/.wslconfig.tmpl`](dotfiles-wsl/.wslconfig.tmpl) manages — the rest of
your file, its comments and your own keys are kept exactly as they are — and the previous file is
copied beside itself as `.wslconfig.bak-dotfiles-<timestamp>` first. WSL reads the file when the VM
starts, so the screen says to run `wsl --shutdown` on Windows to apply the change and deliberately
does not run it; `--dry-run` skips the write.

**Measure the shell's startup.** The fourth utility **changes nothing**. It starts
the login shell named by `$SHELL` the way a terminal does — `zsh -i -c exit`, or the same shape for
your shell — and the **start is yours to name**: the interactive start a terminal gives you, the
login start, or a clean start that reads no configuration at all, with `[m]`/`[n]`/`[w]`/`[b]` for
the start, the number of runs (five by default, at most twenty), a warm-up run to throw away, and a
clean comparison. The line above the rule draws the exact command a press would run, so the plan and
the measurement cannot disagree. It reports the **median** with the **mean** and the **range** and
every completed start, so the number is a measurement and not a single run: the method travels with it and it can be
reproduced by hand. The **spread**, the **95th percentile** and a **trimmed median** join it only once
there are enough runs behind them, and the screen names what is missing, with the runs it would need,
instead of printing a zero.
Every start is bounded by a **ten-second timeout**, because a startup can hang on a plugin that waits
for the network and the utility must not hang with it — a start that does not finish is reported as a
timeout and left out of the median, never written down as `0.0 s` — and **`Esc` stops the runs that
are left while keeping the samples that finished**. On zsh a further start is taken under `zmodload
zsh/zprof`; on bash 5.0+ it is traced with `PS4` + `EPOCHREALTIME`; on fish 3.6+ the shell's own
`--profile-startup` is used. The functions the instrument names are listed heaviest first, and when
the shell has no instrument here, its version is too old, or the instrument reports no table, the
screen says **only the total is measurable** and gives the reason rather than inventing a culprit. With
`[b]` on, the same shell is measured again with no configuration read, and the difference between the
two medians is reported as the shell and the machine's own time versus what your configuration adds —
refused rather than guessed when the two sides are not comparable. `[h]` keeps the last fifty
measurements in `shell-audit-history.json` under the state directory, which is off until you turn it
on, and `[r]` opens the recommendations, each labelled **Measured**, **Observed** or **Confirmed**
according to what kind of claim it is. Nothing is written to your configuration: no startup file is
touched and no plugin is disabled — the tables and wrappers the profiled start needs live in a
temporary directory that is removed again, your own files are sourced from where they already were, a
clean start reads an empty directory of the utility's own, and nothing a startup printed is displayed
or kept. Where it cannot measure — no `$SHELL`, or a login shell this machine does not have — the
section says so and why; a run with **no terminal attached** is measured and says which terminal it
ran under, because a pipe, a container or a CI run is exactly where a slow startup is worth knowing.

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

**Manage the SSH configuration.** This utility reads the SSH setup the client would really
use and keeps the one part of it this tool owns in order. The hosts list is **the client's own
answer**, never this package's idea of it: for each alias the screen asks `ssh -G --` and reports
the effective `HostName`, `User`, `Port`, `ProxyJump` and every `IdentityFile`, naming the file and
line each value came from and saying plainly when a `Match` criterion (an `exec`, a `canonical`, a
`final`) was not evaluated rather than guessing which block won. An entry that cannot be rewritten
safely — a wildcard, a negation, a line with an `Include` or a `Match` under it, a value carrying a
comment, a file this tool may not write — is shown **read-only with the reason**, and the rows that
would edit it are not offered. A new entry is written inside a block this tool marks in
`~/.ssh/config` (`# >>> dotfiles ssh-manager >>>`), and an entry your own file already declares is
edited **where it is**; either way every line the change does not name is preserved
**byte-for-byte**, including your comments, blank lines and unknown options, and the plan of a change
is previewed with its
`+`/`-` lines and the backup it will take before anything is written. A write copies the file to
`.bak-dotfiles-<stamp>` first, writes a temporary file, has **the client's own parser** validate it
with `ssh -G -F`, and replaces the file atomically — refusing outright when the file changed since
the preview. The keys screen describes a pair from its **public half** (the private key is only
`stat`ed, never opened), reports its mode and warns when it is too open, and generates a key with
the passphrase travelling on a **pipe** rather than an argument list. The agent screen names which
agent this environment actually advertises and what it does differently — `gpg-agent` ignores
`ssh-add -t`, 1Password and KeePassXC serve the keys they own — instead of pretending every agent
behaves like OpenSSH's. A connection is tested **only when you press the row that tests it**, with
`BatchMode`, `StrictHostKeyChecking=yes`, `UpdateHostKeys=no` and no password prompt: a test can
never accept a new host key or write to `known_hosts`. See the
[TUI Installer Guide](docs/tui-installer.md#the-ssh-configuration).

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
