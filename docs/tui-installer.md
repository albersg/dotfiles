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
  shade and the body leaves twelve rows or eight, and the glyph cat at five, three or one everywhere else
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
| Main menu | **What will happen** | What the option under the cursor holds — the plan the run would execute, the configurations it would overwrite and the newest backup with when it was taken and how many files it carries for **Start Installation**; the terminal, shell and multiplexer counts the learn screens describe; the bindings each tool ships in the keymap reference; the topic count of the LazyVim guide; the curriculum of the Vim Trainer; every backup with its date and file count for **Restore from Backup**; one honest line for **Exit** |
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
numbers the steps, gives the `▸` marker its own column and right-aligns the numbers, so the step
names start on one column and the digits form a straight edge whether the plan has eight steps or
eighty, and it keeps the description of the step the run starts at — or, once a run is in progress,
the step it is on — because that is the next action. **The other steps' descriptions are
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

The rows of the frame the body did not need are not information. **A small creature walks the last
of them** — immediately above the footer rule — and only there: a screen whose body fills its frame
shows no companion at all, so the creature never costs a body a row, and with animation off there is
no companion anywhere, because a frozen pet is not the point.

It is drawn at five sizes, and a ladder picks between them so that a terminal which leaves fewer rows
gets a smaller creature rather than none. Where the terminal reports true colour: the **shaded volume**
at twelve rows where the frame can spare twelve, then the same volume at eight rows where it can spare
eight. Then, for a frame or a terminal that cannot shade, the glyph cat at **five rows**, its **head**
at three, the **one row** the creature shipped with, and nothing below that. The panel summary's rows
come off the spare rows first, because a fact beats a decoration: a narrow screen with one row to spare
shows the facts and no creature, one with two rows to spare shows both with the summary above the
creature, and one whose body fills its frame shows neither. At 80x24 — the floor every screen is
guaranteed to work at — the main menu leaves five spare rows of which the summary takes one, so the
three-row head is what draws there.

**The volume is rendered, not drawn.** It is an implicit surface made from separate masses: a rounded
skull and shorter muzzle, a narrowing neck, chest and larger haunch, four legs, two triangular ears,
and a three-segment tail. The ears are hinged above the skull rather than merged into its ball; their
inner triangles are darker. The face has a two-pixel dark nose, a two-pixel mouth and bright sclera
around each pupil. The four legs end at the ground line with darker paw pads, and the largest rung adds
three one-pixel whisker strokes on each side. `TestCompanionAnatomyIsStructural` checks that both ear
maxima clear the skull, the haunch has the largest mass below the neck, each leg reaches the ground,
and the rasterized tail's pixel count never increases from its base column toward its tip.

Normals come from the field's gradient and the existing upper-left light shades it; the result is
quantised to the theme's five-step ramp through a fixed ordered-dither matrix, and a drop shadow
grounds it on the row it stands on. Animation **moves the field primitives** — alternating legs, a
one-pixel body bob, a lean, a counter-swaying tail, and a head turn toward the gaze — which makes the
shape deform rather than jump between drawings. The field and raster assertions are in
`TestCompanionAnatomyIsStructural`; the saved frame is only a visual companion to those checks.

The glyph ladder under it needs no colour at all and stays exactly as it is: a half-block cell with no
colour draws as a plain block, and this shading means nothing there. That is what keeps it a floor
rather than a fallback — a terminal without true colour, a run with the sprite switched off
(`DOTFILES_SPRITE=0`) and a frame with fewer rows all get the cat the glyph ladder picks.

It costs what it looks like it costs, and only while it is moving. The renderer repaints a whole line
whenever any byte in it changed, so the cost of a tick is the lines it moved times the width of those
lines. The creature therefore has two regimes, measured by `TestCompanionCostHasTwoRegimes` at 227
columns and quoted here from that test's own output rather than from a number typed once:

- **At rest — nothing.** With no key and no pointer event the view string is byte-identical from tick
  to tick, so the renderer writes **no bytes at all**. Pacing on its own and settling its gaze a frame
  after the target were both removed for this: they made the creature repaint two of its rows on every
  frame over a screen nobody was touching.
- **While walking — twelve lines.** The volume's twelve rows all move with the one-cell step, so every
  one of them is repainted. At 227 columns `TestCompanionCostHasTwoRegimes` reports 19 moving frames
  over 2.38 s, 158228 bytes total and **8852 bytes** in the widest changed lines (about **69.2 KB/s**
  at eight frames a second); the same test reports zero bytes at rest. A wider terminal costs more per
  line and the same per cell walked.

Rendering the volume is the expensive part of that: `BenchmarkCompanionVolumeFrame` reports
**1,557,471 ns/op, 304,295 B/op and 1040 allocs/op** on the measured Linux/amd64 host. Those are
benchmark outputs, not portable limits; the test and benchmark are the sources for remeasurement.

**Why the encoder is ours and not a library.** `github.com/charmbracelet/x/mosaic` was evaluated for
this step and deliberately rejected, for two reasons that were measured rather than guessed. It
requires `x/ansi` at 0.11.7 or later, and with that version the `x/cellbuf` that Bubble Tea v1.3.10
pins does not compile — so adopting it means moving `x/ansi`, `x/cellbuf`, `colorprofile`, `x/term`,
`x/sys`, `x/text`, `go-colorful` and `go-runewidth` inside the **render path of every screen**, for a
sprite. And its colour choice is keyed on a luminance threshold: the block glyph it picks depends on
which pixel counts as "set", and a cell whose two pixels fall on the same side of the threshold is
collapsed to their average colour, which is exactly the shading boundary a four-tone sprite exists to
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
bright tone, so the decoration stays quieter than the words around it. The face below is the centre
gaze; the composer moves the pupil pair one column to either side and, when the pointer is above the
creature, one pixel row up.

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

**It looks where it is going.** The pupils sit at one of three columns across the head and the eye
row is one of two rows, and those cells are the only characters a gaze frame changes: the tables hold
one neutral frame per state and a composer moves the eyes, which is what keeps seven states and three
gaze columns from becoming thirty hand-drawn frames. On the five-row cat "up" moves the eyes to the
upper of the two interior rows; the three-row head has room for one eye row, so its gaze is
horizontal; and the one-row face has no interior column to move a pupil into at all, which is why it
is kept exactly as it shipped. Down is not drawn: there is no honest third eye row, and a
wrong-looking "down" would read worse than a missing one. The pupils rest while the thing it wants is
within a two-column dead zone of its own cell, so they cannot flicker.

**Its eyes follow the mouse.** With the pointer live (which is the default; see below) the pupils turn
one column to the side the pointer is on and look up when it is above the creature, and that is what
`companionGazeFor` receives: the pointer's column clamped to the stage the creature walks, and its row
measured against the bottom third of the frame — the band the creature draws in — so a pointer up the
screen makes it look up and one across the floor makes it look sideways. The turn happens on the
mouse message itself, not on the next tick, so the eyes arrive with the hand rather than a frame
behind it. The pupils rest while the pointer is within a two-column dead zone of the creature's own
cell, so they cannot flicker, and a pointer that lands on the cell it was already on changes nothing
at all — the renderer sees the same bytes and skips the frame.

Where there is no live pointer the creature looks at the selection, which is what it did before the
pointer existed: the row the cursor is on is a body row above its own, so a menu screen makes it look
up and, while the selection is off to one side, that way too, and a screen with nothing to point at
leaves it looking straight ahead. That fallback is why a terminal that refuses mouse reporting, a run
with the pointer switched off and a Termux session all keep a gaze rather than losing it. The gaze is
a cell on the model, like the position and the frame, so a snapshot pins it either way.

**A pointer event also wakes it, and a click makes it jump.** There is nothing clever to detect about
a parked mouse — a parked mouse sends no events at all — so the only pointer event that exists is the
user moving the mouse, and every one of them is the sudden movement that wakes the cat, which is
oneko's rule as much as the sleeping face is. A left click is an event of its own: the creature earns
the same celebration as a finished installation step and hops one row off the ground, drawn as one
blank row under it. The hop costs one spare row more than the sprite itself, so on a frame that has
none the creature stays on the ground and the celebration shows in its face instead — no decoration
takes a row a fact needs. A wheel is ignored: the installer runs in the alternate screen, where there
is no scrollback for it to move.

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
instead of flaking on the clock. The cost is bounded by tests rather than by a promise: at rest the
tick changes nothing at all, so the renderer writes nothing, and while walking it changes only the rows
the creature owns, so those are the lines the renderer repaints. A screen with no spare row draws no
companion, and on those the tick changes nothing either.

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
