# TUI Installer

The dotfiles TUI Installer is a modern, interactive terminal application built with Go and [Bubbletea](https://github.com/charmbracelet/bubbletea) that guides you through the complete setup of your development environment.

## Table of Contents

- [Features](#features)
- [Quick Start](#quick-start)
- [Screens & Navigation](#screens--navigation)
- [Layout](#layout)
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
  a percentage, the step the run is on with its name, the elapsed time and an estimate of what is
  left measured from the run's own clock, a per-step status rail, and optional detailed logs sized
  to the rows the frame leaves
- **A Companion**: A small ASCII creature walks the row above the footer, follows the selection you
  move the cursor to, sleeps when you stop typing, and reacts to failures and to destructive choices
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

The welcome screen and the main menu greet you by the time of day (`Good morning`, `Good afternoon`,
`Good evening`) in one added dim line, so no existing copy is replaced. The greeting is a pure
function of the time the model was created with, never of the clock read while drawing, so a
snapshot pins it instead of flaking on the hour.

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
a rail of one row per step whose state is a glyph and a word (`✓` done,
`●` running, `○` pending, `✗` failed, `⊘` skipped), and, under the bar, the step
the run is on with its name (`Step 6 of 9 · Install Iosevka Nerd Font`) and the
run's clock — how long it has taken and how much is left. The running step's
description is shown under its row, and `d` opens a log box.

**The estimate is the run's own, and it is stated only when it can be.** It scales
the time this run has already spent by the work still to do, so it is derived from
the run's own start timestamp and progress — never from a constant per step and
never from a guess about the machine. With nothing complete there is no rate to
scale and the row says `estimating…`; with no run behind it at all it says
`Estimating the time remaining…` rather than a number. The start timestamp and the
latest tick live on the model, so the renderer never reads the clock: the same
model always renders the same bytes, which is what lets a snapshot pin the screen
and stops the estimate from changing when nothing else did.

**The log box uses the rows the frame leaves.** Its height follows the terminal
instead of a fixed three lines: it shows the freshest lines that fit — the rail
keeps a three-row floor, so turning details on never pushes it or the footer off
the screen — and when earlier lines do not fit it says how many it could not show
in one dim row (`… 9 earlier lines`) instead of dropping them silently. The error
screen's log panel is sized the same way; it used to cut at five lines whatever
the frame was.

**The installer shows only the counts it keeps.** The step counter and the step's
name exist on the run and are shown. The run does not count the files it installs
— no step reports a file tally — so the screen has no file counter and does not
invent one.

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `↑` / `k` | Move up |
| `↓` / `j` | Move down |
| `Tab` | Cycle the panels of a screen that offers more than one |
| `Enter` / `Space` | Select option |
| `Esc` | Go back |
| `q` | Quit (when not installing) |
| `d` | Toggle details (during installation) |
| `Ctrl+C` | Force quit |

## Layout

Every non-trainer screen is drawn inside the same frame — a header, a rule, the body, a rule and
the footer — and that frame spends whatever terminal it is given instead of assuming the 80 columns
it was designed at. Each choice below has a reason and a test behind it.

**The composition is capped at 160 columns and centred.** A full-width row of prose is harder to
read on a 227-column terminal than a 160-column one, because the eye loses the start of the line on
the way back. The body is therefore never wider than 160 columns, and when the terminal is wider
the surplus becomes a margin on each side: the composition sits centred rather than pinned left,
because one void on the right is not a layout.

**Two columns start at 124 terminal columns.** From that width the screen puts the body on the left
and a panel on the right. The threshold is 124 and not 120 because the number the renderer checks is
the *content* width: the frame spends two columns of padding on each side of every screen, so 120
columns of content — the width at which a readable body and a panel fit side by side — is 124
columns of terminal. Below the floor the panel is dropped, not squeezed: the screen is exactly what
it was before the panels existed, and nothing else on it moves.

**A row is a measure, not the terminal.** The bar behind a selected row runs
`min(content width, 80)` columns in a one-column screen, and the left column's width when there are
two, so a row stays a row instead of becoming a 227-column slab of colour behind twenty characters.
A row's trailing meter sits at the right edge of that same measure.

**A short body sits under its own header.** A body shorter than the rows the frame leaves is centred
in them, but the blank rows above it are capped at **six**. Centring alone is right at the 80×24
floor and wrong at scale: on a 227×62 terminal a nine-row menu was centred 24 rows down and sat 26
rows below the header that named it, reading as content that had fallen to the bottom of the screen.
At 80×24 the shift is five rows, below the cap, so nothing moves there.

### Panels

Where the frame has room for a second column, a screen offers one, and each panel answers the
question its own screen asks:

| Screen | Panel | The question it answers |
|--------|-------|-------------------------|
| Welcome | **Your machine** | Where am I — the machine this run is about to change: its OS, WSL host and version, architecture, shell, package manager, Xcode command-line tools and `$HOME` |
| Welcome | **Did you know?** | What can I learn right now — one shortcut at a time, rotating every ten seconds |
| Main menu | **What will happen** | What is about to happen — the plan the run would execute, the configurations it will overwrite, and the newest backup with when it was taken and how old it is |
| Main menu | **Your trainer** | What have I gained — the lessons and mastery of every module you have started, your overall accuracy, your best streak, and the next boss with what it needs |
| Main menu | **Did you know?** | What can I learn right now — one shortcut at a time, rotating every ten seconds |
| Main menu | **Last install** | When did I last run this — when the previous run finished, from which build, and which configuration paths it replaced |

The welcome screen asks where you are, so its panel is the machine; the main menu asks what it is
about to do, so its panel is the plan and the state that run will read. The plan is the wizard's own:
before the first question the panel builds it with the same pure builder from the detected host, and
names the host it is describing (`on Linux (detected)`) because the operating-system question has not
been asked yet.

A panel is a glance, not a document. The main menu's panel numbers the steps, gives the `▸` marker
its own column and right-aligns the numbers, so the step names start on one column and the digits
form a straight edge whether the plan has eight steps or eighty, and it keeps the description of the
step the run starts at — or, once a run is in progress, the step it is on — because that is the next
action. **The other steps' descriptions are deliberately left out**: they are read on the installing
screen, beside the step that is running, which is where a description is read rather than skimmed.

The panels also leave out every fact the installer has not measured. A value the model does not hold
produces no row at all — never `unknown`, `none` or a guessed default — because a panel padded with
invented facts is worse than a short one. Nothing is clipped silently either: a value wider than the
column wraps under itself, and a panel with more rows than the frame leaves says how many it could
not show.

**The plan is on the wizard's choice screens, and the trainer's panel reads the trainer's own file.**
The wizard's own questions (operating system, terminal, font, shell, multiplexer, Neovim, the Ghostty
warning) each draw a plan panel beside the choice, for the same reason the main menu does. The
**Your trainer** panel reads the stats the trainer already persists (`~/.config/dotfiles-trainer/stats.json`)
on the startup path, through the same accessors the trainer menu uses, so the panel and the menu
cannot disagree, and a module you have never opened is left out rather than shown as `0/5`. When
there is no record of any run — no file at all, or a file saved before anything was played — the
panel says so in one line instead of drawing the zeros as progress.

**`Tab` cycles the panels, and the tab row names them.** A screen that offers more than one panel
starts its right column with a row naming them all, the active one in the brand tone and the others
dim; `Tab` moves to the next and wraps at the end. The footer advertises `[Tab] panel` only on those
screens, because on a screen with one panel the key does nothing (the trainer's exercise screens
still read it as "hint"). A screen the frame drops the column from below the two-column floor shows
a **one-line summary** of the active panel above the footer rule instead. That summary names the
panel short (`Machine`, `Plan`, `Trainer`, `Tip`, `Last install`) because it has one row to spend,
and it carries the same headline the panel would. It is placed only in rows the body did not need: a
screen whose body fills its frame shows no summary, and no summary ever takes a row from a body.

**A tip rotates every ten seconds.** The **Did you know?** panel shows one item at a time: the keys
as key tokens, what they do, and the source in the dim tone above them. The pool is built from
content the repository already ships, in a declared order with no randomness — the keymap reference
data (Neovim, then Tmux, Zellij, Ghostty and Herdr, each in its own declared order) followed by the
trainer's own lessons in module order, with the lessons whose mission does not fit the tip's two
rows left out so a tip is never cut — so two runs on the same machine show the same sequence. The
panel advances one tip per ten seconds; with animation off it stays on the first tip.

**The installer records when it last ran.** When a run completes, the installer writes a small
record, best effort, to
`$XDG_STATE_HOME/dotfiles/last-install.json` — or `~/.local/state/dotfiles/last-install.json` when
`XDG_STATE_HOME` is unset. The record holds when the run finished, the build it ran (`VersionLabel`)
and the configuration paths that run backed up and replaced. It is written once, when the run
finishes, and read once on the startup path; a run whose state directory cannot be written still
completes, because the record is a convenience and not a step. On the next run the **Last install**
panel shows that record; when there is no file the panel is not offered at all, so the tab row never
names a panel that would have to say "never".

### The companion

One row of the frame is not information. **A small creature walks the row immediately above the
footer rule** — the last row the body did not need — and only there: a screen whose body fills its
frame shows no companion at all, so the creature never costs a body a row, and with animation off
there is no companion anywhere, because a frozen pet is not the point.

The art is drawn in this repository, in `installer/internal/tui/companion.go`, and it is plain ASCII:
a 16-colour terminal, a terminal without an emoji font and Termux all draw it.

| State | Frame | When |
|-------|-------|------|
| Idle | `(o.o)` | Awake and standing still: the first frame, and the tick it arrives at the row you pointed at |
| Walking | `(o.o)/` `(o.o)\` | The frames it moves: the leg alternates, which is what reads as motion |
| Asleep | `(-.-) z` | Twenty seconds with no key pressed |
| Alert | `(O.O) !` | The selection throws something away |
| Pleased | `\(o.o)/` | About a second after an installation step finishes, or a right answer on a trainer result screen |
| Flinch | `(>.<)` | A failure is on screen, or a wrong answer on a trainer result screen |

The state reads from the glyphs — the eyes, the `z` and the `!` — and not from the tone, so a
16-colour or no-colour terminal loses nothing. Nothing was copied from a third-party mascot either:
the Go gopher is CC-BY, cowsay's cow is GPL-ish and nyancat's cat belongs to its author, and this
repository's attribution surface stays empty.

**It walks, follows the selection, sleeps and reacts.** It strolls one cell per animation frame —
eight frames a second, the frame tick the animation gate owns — and turns at the edge of its row.
Moving the cursor points it at the new row and it walks there over the frames that follow — not in
the frame you pressed the key in — and then resumes strolling. The walk halves the remaining
distance each frame, so a one-row cursor move arrives in four to six frames (about half a second to
three quarters) and the far edge of the widest stage in eight frames, one second. Twenty seconds
without a key put it to sleep and the first key wakes it. It is alert on the screens whose purpose is to restore or overwrite — the backup list,
the restore confirm, and the screen that installs over the configs it just listed — and on the menu
rows that name a destructive action (`Restore`, `Delete`, install *without* backup), it is pleased
for a few ticks after an installation step completes, and it flinches while an error is on screen. On
the trainer's result screens it reacts to the verdict in the header: pleased on `✓ Correct`,
flinching on `✗ Incorrect`. The reaction wins over the resting state, so a sleeping companion that
must flinch flinches.

**The frame and the cell come from the model, never from the clock.** The frame tick advances the
counter and takes one step; the renderer only draws. The same model and tick therefore produce the
same bytes on every run, which is what lets a snapshot pin a frame instead of flaking on the clock.
The cost is bounded by a test rather than by a promise: the creature draws in one row, so a tick
changes exactly that row and the renderer repaints one line. A screen with no spare row draws no
companion, and on those the tick changes nothing at all: the view string is identical, so the
renderer skips the frame entirely.

### Turning animation off

The tip rotation and the companion are driven by one gate. Animation is off when any of these is
true, and with it off no frame tick is scheduled, the screen stays on the first tip and there is no
companion at all:

- `DOTFILES_ANIM=0` is set;
- `--no-anim` is passed (the flag sets `DOTFILES_ANIM=0` before the model is built);
- stdout is not a terminal (a redirected or piped run), so the installer does not stream escape
  sequences into a file; or
- `TERM=dumb`, because that terminal cannot address a cursor well enough to animate on.

The gate is decided once, when the model is built; the render path itself never reads the
environment, so a render stays pure.

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
| `--no-anim` | | Disable animation; the same as `DOTFILES_ANIM=0` |
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
