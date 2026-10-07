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
- **Utilities**: The small jobs that are not part of an installation, starting with a
  reversible system light/dark theme switch and the reversible dotfiles-theme switch, in its own
  main-menu row
- **Progress Tracking**: Real-time installation progress with a frame-width progress bar whose
  filled part carries a travelling highlight during long steps, a percentage, the step the run is on
  with its name, the elapsed time and an estimate of what is left measured from the run's own clock,
  a chart of the run's own progress over time, a per-step status rail, and optional detailed logs
  sized to the rows the frame leaves
- **The Machine's Pulse**: A sampler reads the host about once a second — CPU and memory sparklines,
  the load average, the free space on the target and the process count — and draws them in block
  glyphs on the welcome screen's **This machine, now** panel and beside the installing screen's
  progress bar, where the run's progress is charted over time; a host that cannot be read shows no
  row rather than a lie, and with animation off the panel says the sampling is off instead of
  freezing a chart
- **A Companion**: A creature walks the rows above the footer — a shaded volume where the terminal can
  shade and the body leaves twelve, eight or five rows, and the glyph cat at five, three or one where it cannot
  — follows the mouse pointer with its gaze where the terminal reports one, blinks, yawns before it sleeps
  when you stop typing, reacts to failures and to destructive choices, and celebrates with a two-second
  burst of particles when the run finishes
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
- **Utilities**: The small jobs that are not part of an installation: a reversible
  system light/dark theme switch, the reversible dotfiles-theme switch, whose list of themes is
  one level in behind the **Change the dotfiles theme** row, the WSL resources, one level in
  behind the **Adjust the WSL resources** row on a WSL host, the shell's startup, one level in
  behind the **Measure the shell's startup** row -- the utility that changes nothing -- and the
  read-only terminal capability report behind the **Report the terminal capabilities** row
- **Restore from Backup**: Restore previous configurations (if backups exist)
- **Exit**: Quit the installer

On the main menu, the **Utilities** row opens the Utilities section and `u` opens it too as a
shortcut, `vim` opens the Vim Trainer, `:q` quits, and `dd` briefly sweeps away the selected row
before it returns.

### Utilities

The **Utilities** section holds the small jobs that are not part of an installation. It is reached
from the main menu by its own **Utilities** row, just above **Exit**, and `u` is kept as a shortcut
for anyone who learned it. The row is the discoverable route: a section reachable only by an
undocumented key is a section most users never find.

It holds three jobs: the reversible **system theme switch**, the reversible **dotfiles-theme
switch** (whose theme list is one level in), the **WSL resources** (one level in, and only on a WSL
host), and the read-only **terminal capability report**. The main menu's panel derives one row per
job from the same model state the section's own options are built from -- `utilitiesPanelEntries`
is the only list -- so a utility added to the section shows up in the panel or is visibly missing
from that one function.

The first utility is **the system theme switch**. It changes the desktop's light/dark theme through
the desktop's own tool, and it offers only the desktops whose setting it can read back exactly as it
can write it, because a switch that cannot be put back would destroy a setting the user chose:

| Desktop | Detected by | The switch runs | The store it changes |
|---------|-------------|-----------------|----------------------|
| GNOME | `XDG_CURRENT_DESKTOP`/`DESKTOP_SESSION` naming GNOME, with `gsettings` on `PATH` | `gsettings get`/`set org.gnome.desktop.interface color-scheme` | the GNOME color-scheme key in the dconf database (`~/.config/dconf/user`) |
| KDE Plasma | the desktop naming KDE, with both `plasma-apply-colorscheme` and `kreadconfig6` on `PATH` | `kreadconfig6` to read, `plasma-apply-colorscheme BreezeDark`/`BreezeLight` to switch | the Plasma colour scheme in `~/.config/kdeglobals` |
| macOS | `defaults` on `PATH` | `defaults read`/`write`/`delete -g AppleInterfaceStyle` | the global appearance preference in `~/Library/Preferences/.GlobalPreferences.plist` |

**Nothing is destroyed and every change is reversible.** Before it changes anything the utility reads
the setting that is there and writes it to the installer's own state file,
`$XDG_STATE_HOME/dotfiles/theme.json` — or `~/.local/state/dotfiles/theme.json` when `XDG_STATE_HOME`
is unset. That file is the only file the installer writes for this; the desktop's own store belongs
to the desktop's tool and is never edited directly. The **Undo the last theme change** row puts the
recorded value back, and it records the value it is itself replacing, so undo can always be run
again. A setting the tool reports in a form the restore command cannot safely carry (a value with a
quote, a `$` or a backtick in it) is left exactly as it is and the reason is shown on the screen,
because an unrecoverable change is worse than no change.

**A host with no desktop is told so.** On a server, in Termux, in a bare terminal, or on a desktop
whose tool is missing, the section offers no switch row at all and says in its own body why. It never
shows a row that would fail when it is pressed. Termux is refused outright: it has no desktop theme
to switch.

**`--dry-run` skips it.** The switch and the undo are gated on the same flag as the installation
steps, from the same place, so a documented no-op run runs no `gsettings`, no
`plasma-apply-colorscheme` and no `defaults`, and writes no record.

### The dotfiles theme switch

The other utility changes the **dotfiles' own theme** — the palette this repository ships across its
terminals, its prompt, `bat`, `fish`, tmux and Herdr — not the desktop's light/dark mode. The Utilities section
offers it through a single **Change the dotfiles theme** row, which opens the theme list; the themes
live there so the section reads as a list of jobs rather than a list of themes. That palette used to be
written by hand in six files, so the same colour was maintained in each of them and a drift between
two was invisible. It is defined **once** now, one file per theme under [`themes/`](../themes/), and
the terminal blocks are generated from that definition; a generated block that stops matching its
definition fails a guard rather than being found on screen.

A theme is a definition file, so adding a theme adds a row and the list is never typed by hand. A
**complete** theme defines every canonical role **and** the `[syntax]` members the preview reads, so
it can be applied *and* shown, and is offered; a **partial** one is reported with its reason and is
never offered, so a switch can never apply half a theme and call it unified. The library ships six
complete themes — dotfiles, Catppuccin Mocha, Catppuccin Latte, Kanagawa, Everforest and Rosé Pine —
transcribed from the repository's own blocks or from the published palettes named in
[`themes/README.md`](../themes/README.md). Kagawa was retired: its only file was a byte-for-byte copy
of Kanagawa's and no published Kagawa palette exists, so it is no longer a theme.

**Every offered theme paints every tool the switch names**, and that is a guard rather than a
promise: the twelve tools are the four terminals, Starship, the zsh line editor, the p10k prompt,
Herdr, fish, bat, Neovim and tmux, and `TestEveryOfferedThemePaintsEveryTool` fails naming the theme
and the tool the moment a definition loses what a tool needs. A row still names the tools a theme
cannot paint — a row reads `Apply the <name> theme (not <tools>)` where there is something to name —
but for all six themes that list is empty. Where a tool reads roles the canonical palette does not
carry — Starship's prompt roles, fish's eighteen, bat's scopes, tmux's style options, Neovim's
highlight groups — the values are **derived from the theme's own palette by the fixed mapping
written down in [`themes/README.md`](../themes/README.md)**, so a derived colour is a colour the
theme already holds and nothing is chosen by eye. A role that is neither declared nor derivable is
reported rather than filled.

The switch writes the four terminals, Starship, the zsh/p10k prompt, Herdr, the `BAT_THEME`
selection, fish's `config.fish` palette block, tmux's style block and Neovim's colorscheme selection.
fish is switched through the file this repository owns, never through the user's `fish_config` state
in `fish_variables`: the block sets fish's colour variables in the global scope, which fish returns
over a universal one, and the switch records and restores the exact bytes like every other block.
tmux gets its own generated style block after the TPM run line instead of depending on the
`tmux-kanagawa` plugin, whose Kanagawa palette is partial. Neovim is selected by name: `catppuccin`
and `kanagawa` are the two colorschemes the repository's own plugin install provides, and the other
four themes ship a colorscheme generated from their definition under `dotfiles-nvim/nvim/colors/` —
pointing Latte at the Catppuccin plugin's own name would paint Mocha's flavour, because the plugin's
`flavour` is pinned to `mocha` in this configuration. bat selects a theme by a name its own themes
directory has to hold, so the shell step generates a `.tmTheme` for **every** theme that names one
before it rebuilds bat's cache — and the name the switch exports is the file's own name, because that
is what bat registers a custom theme under (measured against bat 0.26.1; the `<key>name</key>` inside
the file is not the selection key, and a theme that exported it selected a bundled theme of the same
name in five of the six cases).

**The list scrolls, so the frame is not the limit on how many themes there can be.** The picker
draws as many rows as the frame leaves and windows the rest around the cursor, so a theme added to
`themes/` cannot push the list off the bottom of a short terminal. Moving the cursor to a theme the
first screen cannot show scrolls it into view, and the header names the slice that is on screen
(`Showing 3-14 of 20`) so a long list does not look like a list that ends where the screen does. The
undo row and the way back are reached the same way, and the row under the cursor is always drawn:
when the list is long only the description gives up its rows, never a theme, the undo or the way back.

**It only edits files dotfiles own.** Each generated block carries a `dotfiles-managed-config:`
marker, and the switch refuses a file without one, leaving it exactly as the user wrote it — the same
rule `preserve-user-configs` applies to the shell startup files.

**A file with no marker is adopted when its content or its region proves it is ours.** An
installation older than the marker leaves managed files without it, and refusing them made the
switch unusable on a machine that already had dotfiles installed. Before refusing, the switch asks
the *content* — never the path — whether the file is ours, and it recognises two degrees:

- **The whole file.** It carries a generated block marker (`>>> dotfiles-theme...`), or, with the
  ownership-marker lines removed, it is byte-identical to the file the repository ships. Adoption
  writes the marker line and nothing else; the bytes from before adoption go into the same
  `theme.json`, and **Undo** takes the marker back out.
- **Only a region.** The file does not match what the repository ships — it has drifted, or it holds
  the user's own keys — but it still carries the anchors the generator knows (the zsh `PALETTE_*`/SGR
  region, the p10k fallback block, the Starship palette line and table, Herdr's `[theme.custom]`,
  fish's colours, the bat/tmux/Neovim blocks). The switch rewrites **only the bytes between those
  anchors** and leaves every other byte of the file exactly as it was, then records the original so
  **Undo** restores it byte-for-byte. No preserve copy is written for the shell startup files on
  purpose: a `~/.zshrc.d` or `~/.config/fish/dotfiles.d` copy is sourced *after* the managed file,
  so it would put the old palette back at the next shell start. The refresh below is the path that
  preserves a whole file before it replaces it.

A file that proves neither degree is still refused, unchanged — and the refusal **names the file,
every proof that was attempted** (the ownership marker, the generated block, the region anchors, the
byte-for-byte match) and **what the user can do**: reinstall so dotfiles writes its marked files, or
leave that file out of the theme change.

**A marked file with nothing to rewrite is named, not skipped in silence.** A managed file can carry
the `dotfiles-managed-config:` marker yet hold no `>>> dotfiles-theme...` block and none of the region
anchors the generator knows - a generated block that was removed by hand, for instance. (A marked
file that still carries its region anchors is not this case: the switch adopts that region, above.)
The file is ours, so it is not refused, but there is nothing to replace and no region to rewrite.
Rather than quietly leaving it on the old palette, the switch **names the file and says what fixes
it**: a reinstall rewrites the block, because that is what writes generated blocks into installed
files. One line per affected file goes to the log and the result that heads them counts the files, so
a switch that updated the rest is never reported as a whole one. A file whose content has drifted past
both proofs above is a different case (no marker at all): it is refused, and the run stops rather than
writing around it. The marker-but-nothing-to-rewrite case **applies the files it can and reports the
ones it cannot**, because the file is already ours — refusing the whole switch would put a machine
that already has dotfiles back where adoption was introduced to rescue it, and leaving the rest
unchanged would be a worse outcome than a reported, reversible partial switch.

**It is reversible.** Before writing, the switch records the exact bytes each file held in the same
`$XDG_STATE_HOME/dotfiles/theme.json` the desktop switch uses (the two halves coexist in that one
file), so **Undo the last dotfiles theme change** restores each file byte-for-byte. A definition
missing a role is refused rather than written half-empty, and if a later file cannot be written the
ones already changed are put back. **`--dry-run` skips it**, exactly as it skips the desktop switch
and the installation steps.

**Where the definitions are read from.** The definitions live in the checkout, never in the binary,
and the installer reads the first of these directories that holds `themes/*.toml`:

1. `$DOTFILES_DIR`, when it is set — the way to point the installer at a checkout it was not
   launched from.
2. the clone this run made, when there is one.
3. the working directory and each of its parents, nearest first — launching the installer from inside
   the checkout is the normal case.
4. `~/dotfiles`, then `~/.dotfiles`.

When none of them holds `themes/*.toml`, the section says the dotfiles theme is not switchable here
and names the search, instead of drawing rows that would fail.

**The preview writes nothing, and it says so.** While the cursor is on a theme row the **whole
interface is repainted** in that theme's colours — the header, the rules, every row and marker, the
footers and the frame — because every style is built from one palette and the preview rebuilds that
palette from the definition. A label beside the swatches reads `Preview (nothing applied) — <name>`,
so a repainted installer cannot be mistaken for one whose theme has changed. Leaving the theme row
puts the default chrome back: with no preview active the interface is byte-for-byte what it always
was. The palette comes from `themes/*.toml` — the chrome holds no second copy of it — and a theme
with no canonical palette is refused rather than previewed with invented colours.

**Refreshing files installed before the markers.** A machine installed by an older checkout holds
files with neither the `dotfiles-managed-config:` marker nor a generated block. Adoption (above)
accepts one whose whole content matches the repository, and the region proof accepts one that still
carries the anchors the generator knows; a file that carries neither is refused. The picker
therefore offers **Refresh outdated theme files** for the case the switch leaves out — a file whose
content is not a recognizable region — and because the refresh is the path that preserves a file
that may be the user's before it replaces it. Choosing it first **detects** the installed
theme files that are not in the generated form (no marker, no block, or a block that no longer
matches its definition) and opens a **review** that names every file it would touch, why it is out of
date, and where each file that may be the user's will be preserved first. **Nothing is written until
the review is confirmed**, and Cancel leaves every file exactly as it is. A file that cannot be
refreshed (unreadable, not a regular file, or a content that is not a recognizable dotfiles theme
block) is named in the review and skipped, and the rest of the refresh carries on; one bad file never
aborts the batch.

**The preserve-user-configs rule is not negotiated.** Before an unowned file is replaced it is copied
to the same place the install steps use — `~/.zshrc.d/` for `.zshrc`, `~/.config/fish/dotfiles.d/` for
`config.fish` — and every other file is copied beside itself as `<path>.bak-dotfiles-<timestamp>`.
The result names the exact path each file was preserved at. The previous bytes are recorded in the
same `theme.json` the switch writes, so **Undo the last dotfiles theme change** puts every refreshed
file back byte-for-byte. The refresh is gated on `--dry-run` like every other writer, and a dry run
writes no file, no preserve copy and no record.

The welcome screen and the main menu greet you by the time of day (`Good morning`, `Good afternoon`,
`Good evening`) in one added dim line, so no existing copy is replaced. The greeting is a pure
function of the time the model was created with, never of the clock read while drawing, so a
snapshot pins it instead of flaking on the hour.

### The WSL resources

The third utility is **the WSL resources**: the memory, processors and swap the WSL 2 VM may use,
seen and changed from the installer. It is offered **only on a WSL host**, behind the
**Adjust the WSL resources** row; on Linux, macOS and Termux the section offers no such row and its
body says why, because `.wslconfig` is a Windows file that only WSL reads.

The installation already writes this file: the WSL step renders
[`dotfiles-wsl/.wslconfig.tmpl`](../dotfiles-wsl/.wslconfig.tmpl) for the Windows host the installer
is running on, so a freshly installed machine comes out with values that fit it. The utility is the
**second way into the same file**, not a second calculation: both routes call `PlanWSLResources` for
the values, `wslConfigContent` for the content and `writeWSLConfig` for the write, and the guard
`TestWSLResourceUtilityAndTheInstallerAgreeByteForByte` fails if either route grows its own copy.

**The recommended values come from the host.** The same detector the step uses reads the Windows
host's total RAM and logical processor count through interop, and the screen prints those numbers
beside the recommendation so it can be checked against the machine:

| Key | Recommended from |
|-----|------------------|
| `memory` | half the host's RAM, rounded down to 512 MB, capped so Windows keeps at least 2 GiB, omitted below 1 GiB |
| `processors` | every logical CPU the host reports |
| `swap` | a quarter of the planned memory, rounded down to 512 MB |

The rows start on what the file holds today where it sets a key, and on the recommendation where it
does not. **←/→** (`h`/`l`, `-`/`+`) move the value under the cursor by one step — 512 MB for memory
and swap, one CPU for processors — and **`r`** puts every row back on the recommendation. A value
shown as **not set** is left exactly as the file has it: the utility updates the keys the template
manages and never deletes one, so a limit it cannot compute is never thrown away.

**Only the managed keys are touched.** The merge updates the keys the shipped template renders, in
place, under their own section. Everything else in the user's `.wslconfig` — networking settings,
experimental flags, keys of their own, comments, the order of their lines — is kept byte for byte,
and the previous file is copied beside itself as `<path>.bak-dotfiles-<timestamp>` before the write.
The three routes into the file (the non-interactive step, the interactive script and the utility)
all merge this way, so a machine installed today already has its own keys preserved.

**The change is honest about when it applies.** WSL reads `.wslconfig` when the VM starts, so the
screen says that nothing takes effect until `wsl --shutdown` is run on Windows and the terminal is
reopened — and it does not run it, because that would close the session the user is working in.
**`--dry-run` skips the write**, leaving no file, no backup and no record, exactly as it skips every
installation step.

### The shell's startup

The fourth utility is **the shell's startup**, behind the **Measure the shell's startup** row. It is
the one utility that **changes nothing at all**: no startup file is opened for writing, no plugin is
disabled, and no recommended edit is applied. It answers a question the user lives with every day and
cannot see — why the terminal takes a second to open — and leaves the decision to them.

**It measures the way a terminal starts the shell.** The shell named by `$SHELL` is started as
`zsh -i -c exit` (or the same shape for the login shell this machine has), **five times**, and the
screen reports the **median** with the **range** beside it, the count it covers, and every completed
start in the order it ran. Five is odd on purpose, so the median is a run that really happened rather
than the average of two, and it is enough that one cold start does not decide the answer. The number
is never a single run and never an average, and the screen says so: the method travels with the
number, so it can be reproduced by hand.

**Every start is bounded by a ten-second timeout, and a timeout is reported as a timeout.** A
startup can hang — a plugin that waits on the network, a prompt reading a filesystem that is not
answering — and the utility must not hang with it. A start that does not finish inside the bound is
killed, counted, and named on the screen as a timeout; it is **left out of the median**, and the
reason is that a hung start has no duration to report. Writing one down as `0.0 s` would be inventing
a number, which is the one thing this utility cannot afford to do.

**It attributes with zsh's own profiler, or says it cannot.** On zsh, one further start is taken with
`zmodload zsh/zprof` loaded before the startup runs, and the functions zprof reports are listed
heaviest first — in zprof's own order, by the time each function spent on itself — with the total,
the self time, the call count and zprof's own percentage. The utility does not diagnose by guessing:
when `zprof` is not available, when the shell is not zsh, or when the profiled start reports no
table, the screen says **only the total is measurable** and gives the reason. No function is ever
named that zprof did not name.

**It is read-only, and it says so.** The profiled start writes two wrapper files and zprof's table
into a directory of the utility's own under the system temporary directory, and removes it again.
The user's own startup files are **sourced from where `ZDOTDIR` already pointed** — never copied,
moved or rewritten — and the run's own environment is what tells the wrapper where they are. The
screen states plainly that nothing is changed and that the fix belongs in the user's own
configuration.

**The profiled start is the one that can differ, and the screen says so rather than guessing.** For
that single run `ZDOTDIR` points at the wrapper's directory, so a startup file that itself refers to
`$ZDOTDIR` resolves it to the wrapper for that run — the one way the profiled start can behave
differently from the five measured ones, which run with the environment exactly as it is. If that
makes the startup fail, the screen reports that zprof named no table instead of a function, which is
the honest answer: the total is still measured, and no culprit is invented.

**Where it cannot measure, it says why.** With no login shell named (`$SHELL` empty), a login shell
this machine does not have, or a run with no terminal attached — a server, a container, a redirected
run — the section offers no row and its own body names the reason. Without a terminal the interactive
start cannot be reproduced faithfully: job control and the plugins that read the terminal behave
differently, and the number would describe a start the user never gets.

### The terminal capabilities

The report answers one question -- **what can the terminal this installer is running in do, and what
does that mean for the themes and the interface** -- and it is the one utility that writes nothing
and changes nothing. It is reached from the section's **Report the terminal capabilities** row and is
read-only: its only key is the way back.

It reports four capabilities, each with the answer, **where the answer came from**, and its
consequence:

| Capability | The answer | Where it comes from | What it implies |
|------------|-----------|---------------------|-----------------|
| Colour depth | truecolor (24-bit), 256, 16 or none | the terminal's reply to the `RGB` capability query, else `COLORTERM`, the terminal's own `TERM_PROGRAM` name, or `TERM` | no truecolor means **the themes will look approximate** |
| Clipboard (OSC 52) | supported, not supported or unknown | the terminal's reply to the `Ms` capability query | without it, copying **does not reach the clipboard over SSH** |
| Synchronized output (mode 2026) | supported, not supported or unknown | the terminal's reply to a DECRQM query for mode 2026 | without it, **a large redraw may flicker or tear** |
| Nerd Font glyphs | supported, not supported or unknown | there is no terminal query for it | icons may draw as boxes or question marks if the font is missing |

**Three states, never two, and never an invented "no".** A capability is `supported`,
`not supported`, or **`unknown`**. `unknown` is a first-class answer: the screen says why the probe
could not determine it and, where a person can settle it, how to check by hand (copy text and paste
it elsewhere to test OSC 52, watch a large redraw to test mode 2026, look at the glyphs to test the
font). The probe never turns silence into a "no": a terminal that does not answer is unknown, not
unsupported. Whether a Nerd Font is installed cannot be asked of the terminal at all, so that answer
is always unknown with a manual check.

**Nothing blocks and nothing runs at startup.** The probe runs only when the screen is opened,
off the update loop, never when the model is built or when the installer starts. The terminal query
is bounded by a timeout and the read is taken under a read deadline, so a terminal that never answers
produces an honest empty reply within the timeout rather than a frozen interface; a host with no
controlling terminal (a container, a service, a run with no TTY) gets the empty reply too and the
report then says `unknown` with that reason. The screen never writes, so a run that never opens it
writes not a single query byte.

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
description is shown under its row, and `d` opens a log box. Under the clock, once
the sampler has a reading, one row shows the machine's pulse (cpu and memory
sparklines, load and free disk) and the next charts the run's own progress over
time; the block is drawn only from the rows the rail and the log do not need, so
it never displaces them.

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

**The trainer's lesson screen uses the same two columns, without a panel.** From the same 124-column
floor the trainer's own `layoutFor` columns put the code window on the left and the mission, the
answer line and the feedback on the right, through the same `composeColumns` the framed screens use.
The code window then keeps every row the stacked right column no longer needs, so a wide terminal
shows more code instead of a narrow window under a full-width body. The boss and the menu keep their
one-column bodies; below the floor the lesson screen is byte-for-byte what it was.

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
| Welcome | **This machine, now** | How is it doing right now — the CPU and memory sparklines and the load, disk free and process count, sampled about once a second; with animation off it says the sampling is off, and on a host that reports nothing it says that |
| Welcome | **Did you know?** | What can I learn right now — one shortcut at a time, rotating every ten seconds |
| Main menu | **What will happen** | What the option under the cursor holds — the plan the run would execute, the configurations it would overwrite and the newest backup with when it was taken and how many files it carries for **Start Installation**; the terminal, shell and multiplexer counts the learn screens describe; the bindings each tool ships in the keymap reference; the topic count of the LazyVim guide; the curriculum of the Vim Trainer; every backup with its date and file count for **Restore from Backup**; one row per utility for **Utilities** — the switch this host offers and whether the undo is available, the theme list, the WSL recommendation, and the terminal capability report — or one honest line when there is no desktop; one honest line for **Exit** |
| Main menu | **Your trainer** | What have I gained — the lessons and mastery of every module you have started, your overall accuracy, your best streak, and the next boss with what it needs |
| Main menu | **Did you know?** | What can I learn right now — one shortcut at a time, rotating every ten seconds |
| Main menu | **Last install** | When did I last run this — when the previous run finished, from which build, and which configuration paths it replaced |

The welcome screen asks where you are, so its panels are the machine — first what it is, then what
it is doing right now — and then a tip; the main menu asks what is about to happen, so its panel
follows the option under the cursor and names that option in its first
row. **The panel's name does not move with the cursor** — the tab row would wander, and `Tab` would
become a moving target on the very key that walks the panels — so the selection is named inside the
panel instead. The plan is the wizard's own: before the first question the panel builds it with the
same pure builder from the detected host, and names the host it is describing
(`on Linux (detected)`) because the operating-system question has not been asked yet.

A panel is a glance, not a document. On the **Start Installation** selection the main menu's panel
names how many steps the plan has and the single step the run starts at — or, once a run is in
progress, the step it is on — with the `▸` marker in its own column and the number right-aligned so
the step names start on one column, because the next action is the only one the reader acts on now.
The plan's other steps are not listed here at all: listing them cost the creature its rows on
screens where both cannot fit, and the rung never shrinks to fit. **The other steps' descriptions are
deliberately left out**: they are read on the installing screen, beside the step that is running,
which is where a description is read rather than skimmed.

The panels also leave out every fact the installer has not measured. A value the model does not hold
produces no row at all — never `unknown`, `none` or a guessed default — because a panel padded with
invented facts is worse than a short one. Nothing is clipped silently either: a value wider than the
column wraps under itself, and a panel with more rows than the frame leaves says how many it could
not show.

**The plan is on the wizard's choice screens, and the trainer's panel reads the trainer's own file.**
The wizard's own questions (operating system, terminal, font, shell, multiplexer, Neovim, the Ghostty
warning) each draw the plan panel beside the choice, name the highlighted choice in its first row
and show the plan that choice would lead to — so the operating-system question previews the
highlighted OS and the shell question previews the highlighted shell — while still marking the step
the plan is on. The **Your trainer** panel reads the stats the trainer already persists
(`~/.config/dotfiles-trainer/stats.json`)
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

**The installer records the theme it changed too.** The Utilities section writes the setting it
replaced to `$XDG_STATE_HOME/dotfiles/theme.json` — or `~/.local/state/dotfiles/theme.json` — so the
**Undo the last theme change** row can put it back. Unlike the last-install record this write is not
best effort: the record is what makes the change undoable, so a switch whose record cannot be saved
is reported rather than left looking reversible when it is not. It is read once on the startup path
and written only when a switch or an undo succeeds.

**The installer records when it last ran.** When a run completes, the installer writes a small
record, best effort, to
`$XDG_STATE_HOME/dotfiles/last-install.json` — or `~/.local/state/dotfiles/last-install.json` when
`XDG_STATE_HOME` is unset. The record holds when the run finished, the build it ran (`VersionLabel`)
and the configuration paths that run backed up and replaced. It is written once, when the run
finishes, and read once on the startup path; a run whose state directory cannot be written still
completes, because the record is a convenience and not a step. On the next run the **Last install**
panel shows that record; when there is no file the panel is not offered at all, so the tab row never
names a panel that would have to say "never".

### The machine, live

While a run takes minutes the screen should show the machine doing the work, so a sampler reads the
host **about once a second** and the readings live in a ring on the model. The sampler is a command,
never a renderer: it touches `/proc` on Linux and WSL, `kern.cp_time`, `hw.memsize`, `vm_stat`,
`vm.loadavg` and `ps` on macOS, and it asks the filesystem behind `$HOME` for its free bytes. What
it reads is CPU busy (the busy share of the difference between two readings), memory used and total,
the load average, the free space on the target and the process count.

**A reading degrades row by row, and a host that cannot be read reports nothing.** Termux, an
unknown platform and an unreadable file all come back with no rows rather than with zeros dressed up
as a measurement, and the panel then says so in one line instead of drawing an empty chart. The ring
is on the model, so the renderer reads no clock and no file — a snapshot pins a series, which is what
lets an "alive" panel be tested.

The readings are drawn by hand, in block glyphs (`▁▂▃▄▅▆▇█`) for the sparklines and braille for a
denser chart, so a 16-colour terminal and a terminal with no colour at all lose nothing: the shape is
the information and colour only decorates. They appear in two places. The welcome screen's **This
machine, now** panel shows the CPU and memory sparklines and the load, disk and process facts; and
the installing screen — the screen a person stares at while waiting — shows the machine's pulse on
one row beside the progress bar, with the run's own progress charted over time on the next. The pulse
is a fact and earns its rows only from the budget the rail and the log do not need: a frame that
cannot hold it drops it rather than the rail, and a model that has not been sampled draws the
installing screen exactly as it was.

```text
  cpu ▃▅▆▄▆▇▅▃▅▆ 42%  mem ▃▃▅▅▆▆▇▇ 61%  load 1.20  disk 314.2 GiB free
  run ▁▂▃▃▄▄▅▅▆▆▇▇█
```

**The sampling is gated by the same switch as the animation.** With the gate off no sample is
scheduled at all, and the panel says the sampling is off rather than showing the last reading as if
it were current; a stale chart presented as live is the lie the gate exists to prevent. The cost is
bounded the same way the companion's is: a reading only repaints the rows a live widget owns, and a
screen that shows no reading — the main menu — is byte-for-byte the same after one lands. The
interval is named once, so the sampler's cadence and the cost the test measures cannot drift.

**The bar has a traveller, and a finished run celebrates.** During a long step the filled part of the
progress bar carries one lighter cell (`▒`) that walks it on the frame tick; the highlight is a glyph
difference, so a colourless terminal still sees it move, and it only draws while a run is actually in
flight. When the run finishes, a two-second burst of particles rises in the rows the body did not
need and the companion is pleased — both are model state advanced by the frame tick, so a snapshot
pins the frame they are on, and both are absent with the animation gate off.

### The companion

The companion's size is **a function of terminal height and sprite mode only**. Spare rows and the
selected item never choose a smaller creature. `TestCompanionVolumeSpriteIsTheLadderTopSteps` prints
and asserts the rung at each boundary: pixel mode selects the shaded volume at **12 rows at height
48+**, **8 rows at height 32–47**, and **5 rows at height 25–31**; glyph mode selects **5 rows at
height 25+** and **3 rows at height 24 or below**. `companionHeightShare = 4` bounds the rung to a
quarter of the terminal height, which is why the volume rungs start at 48 and 32: the 12-row rung
used to start at 34, where it took 35% of the screen. These are rung sizes, not screen reservations.

The 5-row mini volume is the smallest grid that survives the anatomy assertions below
(`TestCompanionAnatomyHoldsAtEveryRung`), and it is the same five rows as the glyph cat's full rung,
so it **takes over exactly that rung**, at the same 25-row threshold, and nowhere else. Heights
**20–24 keep the compact three-row glyph head even with the encoder on**: a five-row volume does not
fit the rows those frames leave, a four-row volume loses the eye (the smallest honest volume is five
rows), and dropping the creature is worse than an ASCII one. That is a decision, not an oversight —
a visible glyph cat is the floor, and `companionSprite` disambiguates the shared five-row height by
mode.

At the **80×24 floor** the framed screens keep the creature they had: 35 of the 53 measured screens
draw a creature, exactly the number `main` drew before the mini volume existed. `TestCompanionCoverageAcrossTerminalSizes`
now asserts that floor and the 636-case total (**461 of 636** with the encoder on) and fails if either
falls below the pre-mini baseline. The screens that show no creature at 80×24 are the ones whose body
leaves fewer rows than the compact head needs, which is what the follow-up panel work targets.

The trainer reserves its rung by reducing the code window's available rows; framed installer screens
do not reserve body rows. They draw the creature in existing rows their body did not need. With the
encoder off the framed goldens are unchanged; with it on, a screen whose body leaves fewer rows than
the rung draws no creature rather than shrinking. The sprite is bottom-anchored in those rows. If an
unusual frame cannot fit it without covering content, no creature is drawn: it never shrinks to fit,
and a fact always beats decoration.
At the 80x24 floor, the compact rung's celebration expression remains the shipped one-row face so the
end-of-run burst and the face both fit; `TestCompleteCelebrationGolden` pins that output without a
snapshot update.

**The volume is rendered, not drawn.** It is an implicit surface made from separate masses: a rounded
skull and shorter muzzle, a narrowing neck, chest and larger haunch, four legs, two triangular ears,
and a three-segment tail. The ears are hinged above the skull rather than merged into its ball; their
inner triangles are darker. The face has a two-pixel dark nose, a two-pixel mouth and bright sclera
around each pupil. The four legs end at the ground line with darker paw pads, and the largest rung adds
three one-pixel whisker strokes on each side. `TestCompanionAnatomyIsStructural` checks that both ear
maxima clear the skull, the haunch has the largest mass below the neck, each leg reaches the ground,
and the rasterized tail's pixel count never increases from its base column toward its tip.

Normals come from the field's gradient. Lambert diffuse light comes from the upper left, and a
silhouette rim term is limited to upper-left-facing boundary normals; fixed 2×2 ordered dithering
quantises the result to the encoder's **five distinct ramp tones**. `TestCompanionLightTerms` asserts
that all five occur in a rendered frame, checks the rim's upper-left boundary contribution and its
absence on the lower-right boundary, and verifies two-step ambient-occlusion darkening in the
normalized contact regions under the neck, under the body and between the legs. A separate drop
shadow grounds it on the row it stands on.

The volume encoder is not colour-profile adaptive: the model gate admits the shaded tier only for
true-colour terminals. `TestCompanionVolumeIsRefusedWithoutTrueColour` checks that 16-colour,
256-colour and colourless profiles refuse it, and `TestCompanionVolumeSpriteIsTheLadderTopSteps`
checks the unchanged glyph fallback. Termux, 16-colour and colourless terminals therefore lose no
companion or meaning; they do not receive a falsely promised five-tone conversion.

Animation **moves the field primitives** through four one-tick poses — contact, down, passing and up.
`TestCompanionWalkPoseSequencePinsContactsAndLag` prints the four-frame (**0.50-second at 8 fps**)
cycle and asserts each paw's landing frame in the order
front-left, back-right, front-right, back-left, the body's one-pixel rise at passing, the head's
one-pixel lead and the tail's two-pose lag. The geometry is checked against the raster; the art stays
inside the terminal-sized block. The anatomy guard remains `TestCompanionAnatomyIsStructural`.

An idle creature emits one-frame events on the model tick: the chest breathes one pixel every **32
frames / 4 seconds**, an ear twitches every **160 frames / 20 seconds**, the tail tip flicks every
**120 frames / 15 seconds**, and the existing blink recurs every **80 frames / 10 seconds**. These
figures are asserted by `TestCompanionIdleEventsHaveIndependentPeriods`. Every frame between events
is unchanged; drawing reads no clock.

The glyph ladder under it needs no colour at all and stays exactly as it is: a half-block cell with no
colour draws as a plain block, and this shading means nothing there. That is what keeps it a floor
rather than a fallback — a terminal without true colour, a run with the sprite switched off
(`DOTFILES_SPRITE=0`) and a frame with fewer rows all get the cat the glyph ladder picks.

It costs what it looks like it costs. The renderer repaints a whole line whenever any byte in it
changes. `TestCompanionCostHasTwoRegimes` prints the measured figures at **227 columns and height 62**:

- **Between events:** the still creature changed **0 bytes across 24 pinned ticks**. It is not accurate
to claim zero bytes for all idle time: each periodic event repaints the sprite's owned rows.
- **Walking:** the volume owns **12 sprite rows** and changes up to **12 lines per moving tick**. The
test reports **19 moving frames out of 19 (2.38 seconds)**, **166586 bytes total**, and **9138 bytes**
in the widest changed lines (**71.4 KB/s at 8 fps**). The test prints the four periods alongside this
measurement; wider terminals cost more per line.

Rendering the volume is the expensive part of that: `BenchmarkCompanionVolumeFrame` on Linux/amd64
measured **2,179,233–4,199,595 ns/op, 359,912–359,928 B/op and 1087 allocs/op** across three
Linux/amd64 runs. The mini rung is the one the 80×24 floor draws; `BenchmarkCompanionVolumeMiniFrame`
measures it against the same ceilings and reported **701,671–724,544 ns/op, 112,483–112,496 B/op and
545 allocs/op** across three Linux/amd64 runs. The benchmark now enforces ceilings of **10,000,000 ns/op, 450,000 B/op and
1,400 allocs/op**. The time ceiling is over twice the slowest of those shared-host measurements to
leave room for host scheduling and Go-version variance; the byte and allocation ceilings leave about
25% headroom over the stable measured allocation footprint. A fivefold allocation regression exceeds
the byte and allocation ceilings. These limits are a regression guard, not a promise of latency on
other hosts; rerun `go test ./internal/tui -run '^$' -bench '^BenchmarkCompanionVolume(Frame|MiniFrame)$' -benchmem`
to remeasure. This measurement also does not include a raster-buffer optimization: roughly **293 KB
per frame** was measured and declared as future optimization work, not implemented here.

`TestCompanionRenderedPosesStayInsideTheirReservedBlock` checks the rendered rows themselves for
both sprite modes: the 12-row, 8-row and 5-row volume rungs, the 5-row and 3-row glyph rungs, and the
one-row trainer floor. It covers all four walk poses, breath/ear/tail idle events, all three hop
phases, left/up and right/down gaze extremes, and blink. Every pose must own exactly the terminal's
rung rows in the same rendered position (above the frame rule on framed screens), with no additional
art row outside that block. `TestCompanionBlockDependsOnlyOnTheTerminal` continues to assert the
block's terminal-sized position across screens and content states. The cost bound and bounding-box
guard do not change the creature, rung or reserved block. The benchmark does not guarantee cost on a
terminal slower than the measured Linux/amd64 host.

**Why the encoder is ours and not a library.** `github.com/charmbracelet/x/mosaic` was evaluated for
this step and deliberately rejected, for two reasons that were measured rather than guessed. It
requires `x/ansi` at 0.11.7 or later, and with that version the `x/cellbuf` that Bubble Tea v1.3.10
pins does not compile — so adopting it means moving `x/ansi`, `x/cellbuf`, `colorprofile`, `x/term`,
`x/sys`, `x/text`, `go-colorful` and `go-runewidth` inside the **render path of every screen**, for a
sprite. And its colour choice is keyed on a luminance threshold: the block glyph it picks depends on
which pixel counts as "set", and a cell whose two pixels fall on the same side of the threshold is
collapsed to their average colour, which is exactly the shading boundary a five-tone sprite exists to
draw (and it softens every edge on a light terminal). The fifty lines here give exact colours per
cell, cost nothing at install time and add no module, so this is a decision and not an oversight: if
someone wants to reintroduce the library, the two costs above are what they have to answer.

The art is drawn in this repository, in `installer/internal/tui/companion.go`, and it is plain ASCII:
a 16-colour terminal, a terminal without an emoji font and Termux all draw it. Every row of a frame is
the same width, so the creature never jitters sideways, and the cell is fixed per height, so a
one-cell step is always one column.

```text
   the whole cat (5 rows)        the head (3 rows)          the shipped art (1 row)
    /\______/\                    /\_____/\
   (          )                  (  o   o  )                (o.o)
   (  o    o  )                    >  ^  <
   (     ^    )
    >  <  >  <
```

The shaded sprite is the same cat as pixels, and it is drawn from a grid of tones rather than of
glyphs: `.` is the light fur, `o` the mid fur, `#` the outline, `@` the eyes — the pupils, the lids and
the expressions — `x` the nose and the mouth, and a blank the terminal's own background, so the pixels
the cat does not cover blend with it. The fur takes the palette's muted tones and the eyes take its one
bright tone, so the decoration stays quieter than the words around it. Each full-rung volume eye is
3×4, with a 1×2 vertical slit pupil; the small rung retains its 2×2 eye and 1×1 pupil.
`TestCompanionVolumeEyePupilAdjacencyByRung` pins the full-rung contract: the slit has a sclera column
on both sides at all three long-axis positions. The central position has a full ring, while the up/down
extremes touch the corresponding lid edge. The pupil never touches the face outline. The one-pixel
highlight is fixed at the eye's upper-left relative position, independent of gaze.

```text
    o#        #o
    o##      ##o
    o###    ###o
   #o####  ####o#
   #oooooooooooo#
   #oooooooooooo#
   #o..........o#
   #o..##oo##..o#
   #o..##oo##..o#
   #o..........o#
   #oooooxxooooo#
   #oooooxxooooo#
   #oooox..xoooo#
    #oooooooooo#
     #oooooooo#
      ########
```

**The state reads from the glyphs**, not from the tone — the eyes, the `z` and the `!` — so a
16-colour or no-colour terminal loses nothing. The faces below are the five-row cat's; the head
carries the same eyes and props, and the one-row art the same state as the frame it replaced.

| State | What it looks like | When |
|-------|--------------------|------|
| Idle | `(  o    o  )` | Awake and standing still: the first frame, and the tick it arrives at the row you pointed at |
| Walking | the paws alternate between legs apart and legs in | The frames it moves: the legs change, which is what reads as motion |
| Blinking | `(  -    -  )` for one frame, every ten seconds | It is awake and idle. The blink is the tick counter read at a modulus, not a second clock |
| Yawning | `(  -    -  )` with the mouth open, over the last two seconds before it sleeps | The quiet run is nearly over, so falling asleep reads as a transition rather than a cut |
| Asleep | `(  -    -  )` and a `z` beside the ears | Twenty seconds with no key pressed |
| Alert | `(  O    O  )` and a `!` | The selection throws something away |
| Pleased | `(  ^    ^  )` and `\o/` above the head | About a second after an installation step finishes, or a right answer on a trainer result screen |
| Flinch | `(  >    <  )` and a `!` | A failure is on screen, or a wrong answer on a trainer result screen |

**The glyph cat looks where it is going.** Its pupils sit at three columns across the head and the eye
row is one of two rows; the five-row cat can look up and the three-row head has one horizontal eye row.
The one-row face remains unchanged. Down is not drawn. The two-column dead zone keeps the gaze from
flickering. These are the glyph-art rules; the volume has its own raster contract below.

**The volume's eyes follow the mouse.** With the pointer live (which is the default; see below), the
pupil travels over three positions along the eye's vertical long axis (up, level, down); horizontal
gaze shifts the skull one pixel and tilts the ears one pixel toward the pointer. `companionGazeFor` receives the pointer's
column clamped to the stage and its row measured against the bottom third of the frame. The turn
happens on the mouse message itself, not on the next tick. `TestCompanionEyeGazeIsolationAndBlinkPinsTheFaceContract`
requires central gaze changes to stay in the eye boxes and pins the fixed highlight and blink; the
`TestCompanionVolumeGazeTurnsTheHeadAndThePupils` permits changes only in the named EYE BOX or HEAD
OUTLINE BAND. The test pins the sclera side columns at all three vertical pupil positions and the full
ring at the central position, where the 3×4 box has room above and below the slit. At the two extremes
the slit reaches the lid edge. Horizontal direction is carried by the head, not by sliding the slit
sideways. The pupils rest while the pointer is within a two-column dead zone of the creature's own
cell, so they cannot flicker, and a pointer that lands on the cell it was already on changes nothing
at all — the renderer sees the same bytes and skips the frame.

Where there is no live pointer the creature looks at the selection, which is what it did before the
pointer existed: the row the cursor is on is a body row above its own, so a menu screen makes it look
up and, while the selection is off to one side, that way too, and a screen with nothing to point at
leaves it looking straight ahead. That fallback is why a terminal that refuses mouse reporting, a run
with the pointer switched off and a Termux session all keep a gaze rather than losing it. The gaze is
a cell on the model, like the position and the frame, so a snapshot pins it either way.

**A pointer event also wakes it, and a click earns a hop.** There is nothing clever to detect about
a parked mouse — a parked mouse sends no events at all — so the only pointer event that exists is the
user moving the mouse, and every one of them is the sudden movement that wakes the cat, which is
oneko's rule as much as the sleeping face is. A left click is an event of its own: the creature earns
the same celebration as a finished installation step, then shows anticipation, one terminal row of travel (two raster pixels) and a
landing squash in three successive frames. All three poses deform the raster inside the terminal-sized
block; `TestCompanionClickHopsAndCelebrates` and `TestCompanionHopHasThreeRasterPhases` pin the order,
fixed block and unchanged non-companion rows. A fact row is never borrowed or moved. A wheel is
ignored: the installer runs in the alternate screen, where there is no scrollback for it to move.

**It walks, follows the selection, sleeps and reacts.** It takes at most one cell per animation
frame — eight frames a second, the frame tick the animation gate owns — and only when it has somewhere
to go. Moving the cursor points it at the new row and it walks there over the frames that follow, not
in the frame you pressed the key in, slowing to a step every other frame over the last three cells so
the arrival reads as a step rather than as a stop; when it arrives it stops, because a creature with
nothing to do does nothing. One row of a menu is three cells of walking and no more, so one arrow key
is a short stroll rather than a dash across a stage that can be 200 columns wide, and a walk from the
first menu row to the last is about two and a half seconds. Twenty seconds without a key put it to
sleep and the first key wakes it. It is alert on the screens whose purpose is to restore or overwrite — the backup list,
the restore confirm, and the screen that installs over the configs it just listed — and on the menu
rows that name a destructive action (`Restore`, `Delete`, install *without* backup), it is pleased
for a few ticks after an installation step completes, and it flinches while an error is on screen. On
the trainer's result screens it reacts to the verdict in the header: pleased on `✓ Correct`,
flinching on `✗ Incorrect`. The reaction wins over the resting state, so a sleeping companion that
must flinch flinches.

Nothing was copied from a third-party mascot: the Go gopher is CC-BY, cowsay's cow is GPL-ish and
nyancat's cat belongs to its author, so this repository's attribution surface stays empty.

**The frame, the cell and the gaze come from the model, never from the clock.** The frame tick
advances the counter and takes one step; the renderer only draws. The same model and tick therefore
produce the same bytes on every run, which is what lets a snapshot pin a frame, a cell and a gaze
instead of flaking on the clock. A still creature writes nothing between its periodic events; the
breath, ear twitch, tail-tip flick and blink are model-tick events with periods asserted by
`TestCompanionIdleEventsHaveIndependentPeriods`. `TestCompanionCostHasTwoRegimes` measures the
zero-byte interval between events and the walking line cost; `TestCompanionTicksChangeOnlyItsOwnRows`
proves animation changes no row outside the widget. A screen with no reserved block draws no companion
and its tick changes no content row.

### Turning animation off

Three switches decide what the creature is drawn as, and they are read once, when the model is built;
the render path reads the model's answers and never the environment. They are independent on purpose:
the animation is the creature moving at all, the pointer is the mouse being read, and the sprite is
the drawing being shaded.

| Switch | What it turns off | What is drawn instead |
|--------|-------------------|-----------------------|
| `DOTFILES_ANIM=0`, `--no-anim` | the frame tick and the tip rotation | no creature anywhere and the first tip. A frozen pet is not the point |
| `DOTFILES_MOUSE=0`, `--no-mouse` | reading the mouse; no mouse mode is requested at all | the glyph or pixel cat looking at the selection, and the terminal's own drag-to-select |
| `DOTFILES_SPRITE=0`, `--no-sprite` | the shaded pixel sprite | the glyph cat at whichever of its three heights the rows allow, with the pointer and the animation untouched |

The first two also switch off further down: with the animation off there is nothing to move, so the
pointer is off too, and with the sprite off true colour is never asked about. Turning the pointer off
is the one that has a price attached, not a benefit: it gives the terminal's own drag-to-select back.

The tip rotation, the companion and the host sampler are driven by one gate. Animation is off when
any of these is true, and with it off no frame tick and no sampling tick are scheduled, the screen
stays on the first tip, there is no companion at all and the live panel says the sampling is off:

- `DOTFILES_ANIM=0` is set;
- `--no-anim` is passed (the flag sets `DOTFILES_ANIM=0` before the model is built);
- stdout is not a terminal (a redirected or piped run), so the installer does not stream escape
  sequences into a file; or
- `TERM=dumb`, because that terminal cannot address a cursor well enough to animate on.

The gate is decided once, when the model is built; the render path itself never reads the
environment, so a render stays pure.

### The mouse pointer, and what it costs

The creature's gaze is the one thing in the installer that reads the mouse, and it has **a switch of
its own** rather than a side of the animation gate, because it costs the user something: a terminal
that is reporting the mouse gives its own text selection up to the application, so on the installer's
screens a normal drag no longer selects text — hold the terminal's bypass key (Shift on xterm-family
terminals, kitty, VTE, Windows Terminal and Alacritty; Option on iTerm2 by default) to select with the
mouse anyway. The pointer is off when any of these is true:

- `DOTFILES_MOUSE=0` is set, or `--no-mouse` is passed (the flag sets the variable before the model is
  built, exactly as `--no-anim` does);
- stdout is not a terminal; or
- the session is Termux, where the pointer is a finger: Termux turns a drag into a wheel report, so the
  gaze would cost the user the swipe and give nothing back. `DOTFILES_MOUSE=1` overrides that default
  for a Termux session with a real mouse attached.

With the pointer off **no mouse mode is asked for at all**, so the terminal never enters application
mouse reporting and its ordinary selection works again; a run that merely ignored the events would
still have cost the user the drag. With the animation gate off there is nothing to move, so the
pointer is off too. The two switches are read once, when the model is built, and the model's own field
decides the program's mouse option, so the option and the model cannot disagree.

The shaded sprite has a switch of its own too, `DOTFILES_SPRITE=0` / `--no-sprite`, and it turns off
only the pixel sprite: a run whose terminal claims true colour but renders block glyphs badly keeps
the glyph cat instead, with the pointer and the animation untouched.

### Synchronized output: the frame, painted at once

The creature is not the only thing that moves. bubbletea builds each frame in a buffer and hands it to
the program's output in one write, but the terminal does not wait for that write to finish before
painting: it draws the bytes as they arrive, so a frame large enough to reach it in more than one read
is visible half-updated — the tearing the animation shows on a redraw that rewrites several lines. The
terminal has a frame buffer of its own for exactly this: the synchronized-output mode (DECSET 2026, the
`\x1b[?2026h` / `\x1b[?2026l` pair) tells the terminal to hold everything between the two sequences
and show it in one repaint. Windows Terminal has supported it since 1.23.20211, and a terminal that
does not implement the mode ignores the two sequences, so no feature detection is needed and the worst
case is a build without this writer.

The installer emits the pair around each frame through bubbletea's own `tea.WithOutput` extension
point, not from `View()`: the frame's bytes belong to the program's writer, and putting escapes in the
view would break the snapshots and the renderer's line diffing. Two rules keep it out of the way:

- **Only when stdout is a terminal.** A pipe or a file has nobody painting frames, and the sequences
  in it would be noise, so a redirected run keeps its bytes byte-for-byte. The same character-device
  check the animation gate uses decides it.
- **`DOTFILES_SYNC=0` turns it off**, in the same spirit as the animation and pointer gates: a
  multiplexer that mishandles the mode has to be escapable without a rebuild. There is no flag for it,
  because the environment variable is the escape hatch.

The wrapper keeps the stream's file descriptor rather than only the stream, so bubbletea can still see
the terminal underneath it — that is where the window size comes from, and on Windows it is also what
enables virtual terminal processing — and a zero-length write emits nothing rather than an empty pair
of sequences.

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
| `--no-mouse` | | Do not track the mouse pointer; the same as `DOTFILES_MOUSE=0` |
| `--no-sprite` | | Draw the creature as glyphs, not as the shaded sprite; the same as `DOTFILES_SPRITE=0` |
| `--non-interactive` | | Run without TUI, use CLI flags instead |

### Non-Interactive Mode

For CI/CD or scripted installations:

```bash
dotfiles --non-interactive --shell=<shell> [options]
```

| Flag | Values | Description |
|------|--------|-------------|
| `--shell` | `fish`, `zsh`, `nushell` | Shell to install (required) |
| `--terminal` | `alacritty`, `wezterm`, `ghostty`, `none`, `kitty` (macOS only) | Terminal emulator |
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

Directory-based configs such as `~/.oh-my-zsh` are backed up recursively, alongside single-file configs like `~/.zshrc`. When installing Fish or Zsh, an existing shell startup file without the shipped ownership marker is also preserved as a dated, sourced drop-in: Fish at `~/.config/fish/dotfiles.d/dotfiles-user-config-*.fish`, Zsh at `~/.zshrc.d/dotfiles-user-config-*.zsh`. The install log names the exact saved path. The shipped `config.fish` sources Fish drop-ins last so personal settings override managed defaults; `.zshrc` likewise sources `.zshrc.d` files last. Fish uses `dotfiles.d` rather than native `conf.d`, because Fish loads `conf.d` before `config.fish`. Files already marked as dotfiles-managed are replaced silently without accumulating drop-ins. For future personal changes, edit or add files in those drop-in directories rather than the managed startup file.

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
| **Homebrew** | Will be installed if missing (macOS/Linux, except Fedora and Arch, which keep `dnf` and `pacman`) |
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

Two speeds, both wrapping `scripts/preflight.sh`:

| Command | What it runs | When |
|---------|--------------|------|
| `make check` | `gofmt`, `go vet`, and the tests for the packages this branch changes, with Go's test cache on | After every edit (the inner loop) |
| `make preflight` | The full local gate: format, vet, build, the whole Go suite uncached, `shellcheck`, the branding audit and a gitleaks scan, each labelled with the CI job it mirrors | Once, before pushing |

`make check` derives the changed packages from `git diff` against the merge-base
with `main` (committed, staged, unstaged and untracked `.go` files under
`installer/`), so it runs the tests that touch the change and skips the rest. It
leaves Go's test cache on: a package that did not change is not re-run. The full
suite runs in CI on every push, which is the matrix of record; `make preflight` is
that same suite once, before the push, so a CI cycle is not spent on a failure a
local run would have caught.

**Do not add `-count=1` to the inner loop.** It disables Go's test cache, which is
exactly the work `make check` exists to avoid. Keep it for one thing only:
reproducing a failure, where a cached result would hide the run you are trying to
watch.

```bash
cd installer
go test ./... -v                                # all packages, cache on
go test ./internal/tui -run TestX -count=1 -v   # reproduce one failure, uncached
```

The terminal matrix guards share one render pass. `terminalFrames`
(`screen_coverage_test.go`) builds each screen case once and renders it once per
measured terminal; the fit guard and the companion-coverage guard both read those
frames rather than building and drawing the same 55 screens at the same 12
terminals again. The pass renders with the companion animating, which is the
strictest frame, so a screen that overflows only with the creature on fails the
fit guard instead of escaping it.

### Updating Golden Files

```bash
cd installer
go test ./internal/tui/... -update
```

### The terminal-title race in the framed goldens (a known pre-existing flake)

If a macOS run fails a `teatest` golden (`--- golden`) **only sometimes**, and the
diff is about the OSC window-title sequence `\x1b]2;<title>\x07` and not about a
frame row, it is this: the title is written by `tea.SetWindowTitle` from
`Init`, so bubbletea emits it as soon as it handles the message, while the first
frame is buffered and flushed by the renderer's own ticker. Under load the title
can land **after** the frame, or not at all before the capture quits. All three
orderings are the same screen.

It is a race in the golden *infrastructure*, not in the change under test. It was
reproduced on `origin/main` at `9093262` — before the themed-picker work — with
the same diff and the same rate, and routing the capture through the then-current
`waitForGoldenFrame` did **not** fix it, because the capture already waited for the
frame; the title's *arrival time* was the only variable.

Every `teatest` golden now compares through `requireGoldenCapture`, which strips
the title from both sides and restores the golden's own at its pinned offset, so
the title's position is ignored while the frame bytes are still compared one by
one, a changed row still fails, and a live title whose text differs still fails.
A title that never arrived is restored from the golden. `TestGoldenCaptureIgnoresTheTerminalTitlePosition`
holds both halves. `-update` still works: the comparison goes through
`teatest.RequireEqualOutput`.

Reproduction: 12 concurrent `yes > /dev/null`, then
`go test ./internal/tui -run TestCompanionGoldenPinsThePixelSpriteAndItsGaze -count=60`.
Before the fix it failed ~3–4 times per 40 loaded runs; after it, **60/60 pass**.

### Project Structure

```
installer/
├── cmd/
│   └── dotfiles/
│       └── main.go              # Entry point with CLI parsing
├── internal/
│   ├── system/
│   │   ├── detect.go            # OS/tool detection
│   │   ├── metrics.go           # The host sampler: /proc, sysctl, vm_stat, Statfs
│   │   └── exec.go              # Command execution, file ops, backups
│   └── tui/
│       ├── model.go             # App state, screens, choices
│       ├── update.go            # Event handlers
│       ├── view.go              # UI rendering
│       ├── metrics.go           # The sample ring, the gated tick, the live panel
│       ├── sparkline.go         # Hand-drawn block sparklines
│       ├── chart.go             # Hand-drawn braille charts
│       ├── panels.go            # The panel registry and its facts
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
