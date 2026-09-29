# Companion v2 — research: a realistic, funny creature whose gaze follows the mouse

Status: proposal, for review. **No production code was written and nothing was committed.**
Branch: `feat/companion-v2`. Owner: the research session (this document is its only artifact).
Mandate: `odd/tasks/companion-v2.md`. Supersedes, for this decision only, the "no new Go
dependency" line at `odd/tasks/installer-companion.md:203-205`.

The three options are in §6, the recommendation in §7, the work breakdown in §8, and the things I
could not verify without running the program in §9. Every claim below is followed by where it came
from: a path with a line number in this repository, or a URL.

---

## 1. What was measured, and how

| # | Fact | Evidence |
|---|------|----------|
| 1 | The shipped companion is one row, 7 columns, 7 ASCII frames, and it is placed only in a spare row above the footer rule | `installer/internal/tui/companion.go` (403 lines), art table at `companion.go:100-112`, `placeCompanion` at `companion.go:386` |
| 2 | The frame tick is 8 fps, its own clock, gated by `DOTFILES_ANIM=0`, `--no-anim`, a non-TTY stdout or `TERM=dumb` | `installer/internal/tui/anim.go:39-110` |
| 3 | The tip rotates "every ten seconds" = 80 ticks; every duration in the companion is derived from `animTicksPerSecond` | `odd/tasks/installer-companion.md:145-149`, `companion.go:55-78` |
| 4 | The installer **already** enables mouse cell-motion (DECSET 1002) at startup | `installer/cmd/dotfiles/main.go:103` (`tea.WithMouseCellMotion()`) |
| 5 | Nothing in the installer handles `tea.MouseMsg`: clicks, wheel and drags are received and dropped today | no `MouseMsg` match anywhere under `installer/` except `main.go:103` and a keymap label |
| 6 | Bubble Tea's renderer is 60 fps (120 max) and diffs **line by line**, skipping unchanged lines and writing zero bytes for an identical view | `$GOMODCACHE/github.com/charmbracelet/bubbletea@v1.3.10/standard_renderer.go:18-19,65-75,161-167,212-222,283-289` |
| 7 | Bubble Tea turns the mouse modes off on exit (`?1002l ?1003l ?1006l`) | the trailing bytes of `installer/internal/tui/testdata/TestMainMenuGolden.golden` |
| 8 | The companion's row is the last spare body row, and its stage is the full inner width; with `H` rows and `F` footer rows it lands on absolute row `H-F-2`, and column 2 is its left edge | `view.go:165-171` (`rows = H-1-3-F`), `view.go:293-304`, `view.go:797` (`Padding(1,2,0,2)`), confirmed by the `(o.o)` on line 48 of `testdata/TestCompanionGoldenFramesTheCreatureAtTickZero.golden` (a 160×50 frame); the placement rule (the summary wins a single spare row, both fit with two) is asserted at `companion_test.go:559-680` |
| 9 | `examples/sprites` and `examples/splash` **do not exist** in bubbletea v1.3.10; `examples/mouse` and `examples/cellbuffer` do | `git/trees/v1.3.10` of `charmbracelet/bubbletea` (GitHub API) |
| 10 | `WithMouseAllMotion` is the only option that reports hover; bubbletea's own example uses it and reads `tea.MouseMsg` | `options.go:121-170`, `screen.go:55-90`, `examples/mouse/main.go` |

Nothing in this document changes the shipped behaviour; §7 is a proposal and §8 is a plan.

---

## 2. Lane 1 — how much detail a terminal really has

### 2.1 The four techniques

| Technique | Sub-cells per character | Tones per cell | Glyphs | Verdict for us |
|---|---|---|---|---|
| Half blocks (`▀▄█`) | 1 wide × 2 tall | 2 (foreground = top pixel, background = bottom pixel) | U+2580, U+2584, U+2588 | Usable **only** as a colour layer. Without colour the picture disappears (see §6.3) |
| Quadrants (`▖▝▙` …) | 2 × 2 | up to 4, needs 256-colour or truecolour | U+2596–U+259F | Same problem, more colours needed |
| Braille (`⠀`…`⣿`) | 2 wide × 4 tall | 1 (foreground only) | U+2800–U+28FF | High resolution, but stippled: the dots do not fill the cell, so solid shapes look like a screen door |
| Sextants (`🬀`…) | 2 × 3 | 3 | U+1FB00–U+1FB3B, Unicode 13.0 | Not safe: font and terminal coverage is the worst of the four |
| Plain ASCII (today) | 1 × 1 | 1 | any | Survives everything, including 16 colours, no colour, Termux and a font with no box-drawing glyphs |

The two-colours-per-cell trick is exactly "foreground fills the upper half, background the lower
half" and is why half blocks give two vertical pixels per cell
(<https://docs.rs/textual-rs/latest/src/textual_rs/canvas.rs.html>: "By using the upper-half-block
character `▀` with foreground = top color and background = bottom color, we get two vertical pixels
per cell").

Braille's dot grid and bit order are unambiguous: dot *n* is bit *n−1*, so U+2801 = DOTS-1 (0x01),
U+2808 = DOTS-4 (0x08), U+2840 = DOTS-7 (0x40), U+2880 = DOTS-8 (0x80) — read directly off
<https://unicode.org/Public/18.0.0/charts/nameslist/2800/>. A braille cell is a 2×4 grid whose left
column is dots 1,2,3,7 and right column dots 4,5,6,8.

### 2.2 What degrades, and where

- **Sextants and legacy-computing glyphs are the fragile ones.** DejaVu Sans Mono has "gaps between
  them horizontally and vertically" for sextants and rectangular shades while Block Elements do not
  (<https://github.com/dejavu-fonts/dejavu-fonts/issues/383>); JetBrains Mono asked for them and the
  request was closed not-planned (<https://github.com/ryanoasis/nerd-fonts/issues/1674>); Zed's
  terminal rendered the whole U+1FB00–U+1FBFF block as tofu
  (<https://github.com/zed-industries/zed/issues/56342>).
- **But modern terminals rasterise block glyphs themselves**, ignoring the font: Alacritty's built-in
  font covers `U+2500..=U+259F`, `U+1FB00..=U+1FB3B` and `U+1FB82..=U+1FB8B`
  (<https://github.com/alacritty/alacritty/blob/4225cea2/alacritty/src/renderer/text/builtin_font.rs>);
  Ghostty has the same for Block Elements
  (<https://github.com/ghostty-org/ghostty/blob/65901966/src/font/sprite/draw/block.zig>); Windows
  Terminal added built-in sextants because the font glyph "doesn't cover the full cell height"
  (<https://github.com/microsoft/terminal/pull/19841>). So "it works on kitty/alacritty/Ghostty/Windows
  Terminal" and "it is tofu on someone else's" are both true.
- **Half-block misalignment is real even where it works**: Alacritty's built-in half blocks and
  quadrants are off by one pixel on odd cell heights
  (<https://github.com/alacritty/alacritty/issues/6201>).
- **Termux**: the terminal font's block/semigraphic glyphs "do not extend to the outer dimensions of
  one character" for some glyphs, and the fix offered is a different font
  (<https://github.com/termux/termux-app/issues/5076>). Termux does do truecolour with
  semicolon-separated SGR (<https://github.com/termux/termux-packages/issues/20655>).
- **16 colours**: one foreground and one background per cell is *enough* for half blocks (two tones),
  not enough for quadrants (four tones). Nothing here needs more than 16 but the shading gets coarse.
- **No colour at all** (`NO_COLOR`, `TERM=dumb`, a monochrome logger): every cell of a half-block
  sprite renders as `▀`, `▄` or `█` and the creature becomes a rectangle. This is the single strongest
  argument against making half blocks the base layer.

### 2.3 The rule the evidence gives us

The repository already states the rule this research confirms
(`installer/internal/tui/styles.go:11-23`, `docs/tui-installer.md:256-300`): *the glyphs and the
words carry the state; colour is decoration, and a 16-colour or no-colour terminal loses nothing.*
Half blocks and braille violate that. So the sprite's **shape** must be glyphs, and any colour/blocks
can only be a second layer that adds tone where the terminal can take it.

---

## 3. Lane 2 — the mouse in Bubble Tea v1.3.10

### 3.1 Two modes, and only one has hover

`tea.WithMouseCellMotion()` sends DECSET 1002: press, release, wheel and motion **while a button is
held** ("Cell motion mode enables mouse click, release, and wheel events. Mouse movement events are
also captured if a mouse button is pressed (i.e., drag events)", `bubbletea@v1.3.10/options.go:121-127`).
`tea.WithMouseAllMotion()` sends DECSET 1003: "motion events, which are delivered regardless of
whether a mouse button is pressed, effectively enabling support for hover interactions"
(`options.go:144-152`). Bubble Tea's own mouse example uses all-motion
(`examples/mouse/main.go`, `tea.WithMouseAllMotion()`), and terminfo.dev's page on the mode says 1003
"generates significantly more traffic than normal tracking (1000) or button-event tracking (1002), so
applications should only enable it when hover effects are needed"
(<https://terminfo.dev/modes/decset-1003-all-motion-mouse>). xterm calls it "Any-event tracking: the
same as button-event mode, except that all motion events are reported"
(<https://invisible-island.net/xterm/ctlseqs/ctlseqs.html>).

**Consequence: the gaze needs `WithMouseAllMotion()`, which is a `ProgramOption` and therefore a
startup decision** (`tea.go:664-674`), exactly like `--no-anim`/`DOTFILES_ANIM=0` is
(`installer/cmd/dotfiles/main.go:85-90`). The gate should mirror the existing one.

### 3.2 What it costs

- **Inbound**: one report per cell the pointer crosses while it moves — a fast sweep across a
  160-column frame is up to ~160 tiny SGR reports (each ~10–20 bytes). xterm's SGR encoding is the
  compact one, and Bubble Tea asks for it on top of 1003 (`tea.go:665-668`,
  `renderer.enableMouseSGRMode()`); the source itself notes that mode 1006 "is a no-op if the terminal
  doesn't support it" (`tea.go:429`).
- **In the program**: every motion event is one `Update` and one `View()` rebuild — the whole frame
  string, not just the sprite. The render budget is capped by the renderer's 60 fps ticker, and a
  frame whose lines did not change writes **zero bytes** (`standard_renderer.go:161-167`), with
  unchanged lines skipped individually (`:212-222`). So hover costs CPU proportional to *events ×
  frame build*, and bytes proportional to *changed lines × 60 fps at most*.
- **Mitigation, and the honest arithmetic**: keep a `lastGazeCell` on the model and ignore a motion
  event that lands on the same cell; then a hover only ever changes the sprite's own rows. With
  option 2 (5 rows × ≤160 columns) the worst case is ≈5 lines × 160 bytes = 800 bytes per changed
  frame; at 8 fps that is ≈6.4 KB/s, and it is zero when the creature is still or asleep. **These are
  arithmetic bounds, not measurements** — see §9.

### 3.3 What it does to text selection

Selection already needs the terminal's bypass modifier **today**, because `main.go:103` already
enables 1002 and the terminal cannot know whether the program uses the events. So the user's accepted
cost is mostly already being paid; what is new is that hover works. The bypass modifier is
terminal-specific:

| Terminal | Bypass for the terminal's own selection |
|---|---|
| xterm and most xterm-compatible terminals, kitty, VTE (GNOME Terminal), Alacritty, Konsole, Windows Terminal | hold **Shift** — kitty's maintainer: "You can simply press Shift (this is standard across many terminal emulators) to select with the mouse ignoring application mouse handling" (<https://github.com/kovidgoyal/kitty/pull/1184>); Windows Terminal documents Shift-extended selection (<https://github.com/MicrosoftDocs/terminal/blob/main/TerminalDocs/selection.md>) |
| iTerm2 | hold **Option** by default — "If selected, applications may choose to receive information about the mouse. This can be temporarily disabled by holding down Option" (<https://iterm2.com/documentation-preferences-profiles-terminal.html>) |

**Gating it off must restore normal selection, not merely stop the gaze.** Since mouse reporting is
opt-in in Bubble Tea, "off" means *not passing the option at all* — then no DECSET is sent, the
terminal never enters application-mouse mode, and plain drag-selects the text again. A switch that
only ignored the events would keep the selection cost for no benefit.

### 3.4 Where the terminal refuses, or a multiplexer eats it

- A terminal that refuses mouse reporting has no way to tell us: Bubble Tea writes `?1003h` and
  `?1006h` and reads events; nothing is sent back. The feature degrades silently to "no gaze" and
  costs nothing else. On exit Bubble Tea always turns the modes off — visible as
  `[?1002l[?1003l[?1006l` at the end of `testdata/TestMainMenuGolden.golden`.
- **tmux**: all-motion was removed in tmux 2.0 ("tmux doesn't support it any more, it was removed",
  <https://github.com/tmux/tmux/issues/55>) and restored later — tmux 3.3a/3.4 CHANGES: "The mouse
  'all event' mode (1003) is now supported" (<https://github.com/tmux/tmux/blob/3.4/CHANGES>). So
  inside tmux < 3.4 the gaze simply never moves (drags still arrive). Nested tmux had a separate
  scroll regression (<https://github.com/tmux/tmux/issues/2721>).
- **Termux has no hover at all**: a finger drag with mouse tracking on is translated into scroll-wheel
  events (<https://github.com/termux/termux-app/issues/1384>), and mouse tracking captures the swipe
  so it "blocks viewport scrollback" in normal screen mode
  (<https://github.com/termux/termux-app/issues/4302>). The installer runs in the alternate screen, so
  there is no scrollback to lose — but there is no gaze to gain either.
- zellij / herdr / screen: **not verified** for 1003 forwarding (see §9).

### 3.5 Mapping a mouse cell to the sprite

`tea.MouseMsg.X/Y` are 0-based terminal cells. The frame is padded by one row on top and two columns
on each side (`view.go:552-557,797`), and the companion's stage is the full inner width
(`companion.go:139-141`). So:

```
spriteRow (absolute, 0-based) = installerBodyRows(H, footerRows) + 2     // = H - footerRows - 2
spriteCol (absolute, 0-based) = 2 + CompanionPos
stageCol (0-based within the stage) = msg.X - 2
spriteBottomRow = spriteRow + spriteHeight - 1
```

The row is *derivable* — no render-time mutation is needed to know where the creature is — and the
golden at 160×50 confirms it (row index 47 holds the sprite, and the private formula gives
`rows = 50-1-3-1 = 45`, `45+2 = 47`). A test should pin this arithmetic beside the placement test, so
the mapping cannot drift when the frame changes.

### 3.6 One free extra: a click is already an event

Because clicks are dropped today (§1, fact 5), handling `MouseActionPress` costs nothing structurally
and buys a reaction that is genuinely funny: a click makes the creature jump (one row up for two
ticks) or, on the trainer's exercise screens, look at the pointer's column. It is optional and I would
put it in the last slice, not the first.

---

## 4. Lane 3 — smooth motion, and the clock it must not break

### 4.1 A faster *base* frame rate is the wrong answer

Motion in a character grid is quantised to whole columns. Raising 8 fps to 24 fps does not make a
sprite that moves one cell per frame smoother — it makes it three times faster or three times as
jumpy. It would also silently change how long everything else takes: sleep is `20 × animTicksPerSecond`
ticks, the celebration is `1 × animTicksPerSecond` ticks and the tip advances one item per 80 ticks
(`companion.go:55-78`, `odd/tasks/installer-companion.md:145-149`). It would also change the *tip's ten
seconds*, which is a documented user-visible promise. So the base tick stays 8 fps.

### 4.2 The burst tick, and why the tip invariant survives

Where a higher rate does buy something is *slow* movement: with a spring, the last two cells of an
approach are crossed with sub-cell-sized steps, and at 8 fps the crossing is timed to within 125 ms
(visible stutter); at 24 fps it is timed to within 42 ms. So the recommendation is a **second tick
that exists only while the creature is in motion**:

| Clock | Rate | Owns | Never touches |
|---|---|---|---|
| `animTickMsg` (today) | 8 fps, gated | art frame, idle/sleep counter, celebration counter, tip rotation, walk leg pair | — |
| `motionTickMsg` (new) | 24 fps, gated **and** armed only while `CompanionMoving` | `CompanionPos` and its velocity, the one-row walk bob | the idle counter, the tip, the art frame index |

The tip therefore still advances one item per 80 *anim* ticks = 10 s, and the existing test that pins
"a tick changes only the creature's rows" gains a sibling: "a motion tick changes only the creature's
rows and does not advance the tip". A motion tick blocked by the animation gate is never scheduled, so
a headless run still schedules nothing (`anim.go:60-110`).

### 4.3 `charmbracelet/harmonica` versus an easing function written here

Verified from the module itself and the GitHub API:

- `github.com/charmbracelet/harmonica` **v0.2.0**, MIT (`LICENSE`), 7 Go files / 23,331 bytes of Go,
  **zero requires in its own `go.mod`** ("module … go 1.16", nothing else), 1,616 stars, last push
  2026-08-13 — i.e. alive.
- It is a damped-spring integrator: `NewSpring(FPS(60), frequency, damping)` then
  `pos, velocity = spring.Update(pos, velocity, target)` (`harmonica/spring.go`, `README.md`). Pure
  and deterministic: the timestep is a constructor parameter and `Update` reads no clock.
- `spring.go` carries Ryan Juckett's zlib-style notice ("Altered source versions must be plainly
  marked as such… This notice may not be removed or altered"), so adopting it adds a second notice to
  a dependency surface this repository audits by hand (`THIRD_PARTY_NOTICES.md`,
  `docs/audits/LICENSE-AUDIT.md`).
- It is **not** currently a dependency of this module: `installer/go.mod` and `installer/go.sum` have
  no `bubbles` and no `harmonica`; `bubbles` v1.0.0 (present in the local module cache) requires
  `harmonica v0.2.0` **directly** in its first require block and only `bubbles/progress` imports it.

**Verdict: do not add it.** A critically-damped spring over an integer cell axis is about twenty
lines (`v += k*(target-pos); v *= damping; pos += v`, plus a clamp), it is easier to make
deterministic under a fixed tick than a float spring that assumes a 60 fps timestep, and it keeps
`go.mod`, `go.sum`, `THIRD_PARTY_NOTICES.md` and the licence audit untouched. The user opened the
door to dependencies that earn their place; this one does not clear that bar — the honest fallback if
the user wants *real* overshoot and anticipation is harmonica, and it is the only candidate in §5 that
would clear the licence and size bar.

### 4.4 Behaviour worth copying from `oneko`, and only the behaviour

`Neko`/`oneko` is the reference the shipped companion already claims
(`odd/tasks/installer-companion.md:49-53`). What the original does, from the sources: it chases the
pointer, **stops at a distance** from it (`-idle pixels`: "specifies the maximum number of pixels the
…", <https://github.com/radare/toys/blob/master/screencast/oneko/oneko.man>), scratches at window
borders, and "Leave the mouse idle for long enough and Neko will groom and go to sleep; move it
suddenly and Neko will awaken, to resume the chase!"
(<https://www.mobygames.com/game/28106/neko/>). Its state machine is `NEKO_STOP`, `NEKO_JARE` (wash),
`NEKO_KAKI` (scratch), `NEKO_AKUBI` (yawn), `NEKO_SLEEP`, `NEKO_AWAKE`
(<https://code.irenes.space/oneko/plain/src/oneko.c>), and the idle sequence is wash → scratch → yawn
→ sleep (<https://github.com/nucket/NekoAI/commit/1d49f6a3>). The art and the code are **not**
reusable: `tie/oneko` has no LICENSE file, and the original is a 1990s X11 bitmap program.

Three behaviours to take, in cost order: (1) a **dead zone** — walk toward the pointer's column but
stop two cells short, which is what makes it read as a pet rather than a label; (2) **wake on sudden
movement** — a large jump in the pointer or a click resets the idle counter, while a parked mouse
does not; (3) a **groom/yawn** transition before sleep, which is one extra art state for `Neko`'s
wash/yawn sequence and is the cheapest "funny" available.

---

## 5. Lane 4 — what to reuse, with a verdict each

Licences, versions, sizes and dates below were read from the GitHub API and from the raw files of
each repository on 2026-09-29; size is bytes of `.go` source from the repository tree.

| Candidate | Licence | Size / files | Library or app | Technique | Verdict |
|---|---|---|---|---|---|
| `charmbracelet/bubbles` → `spinner` | MIT | v1.0.0; spinner = 2 files, ~287 lines; repo 1.2 MB | library, but the whole module graph comes with it (its `go.mod` requires `harmonica`, `clipboard`, `lipgloss`, `bubbletea`) | fixed frame lists (`Line`, `Dot`, `MiniDot`, `Pulse`…) each with its own `FPS` and its own tick command | **Ignore.** It is a component with its *own* clock, which is exactly the thing `anim.go`'s one-gate design refuses; it would also pull a module graph into `go.mod`/`go.sum` and the notices, for ~20 lines of `frames[tick%len]` we already have |
| bubbletea `examples/mouse` | MIT (module) | 1 file | example | `WithMouseAllMotion` + `tea.MouseMsg` | **Copy the idea** (it is one line and it is the API we need) |
| bubbletea `examples/cellbuffer` | MIT | 1 file | example | `x/ansi` cell buffer for the view | **Ignore** — `x/cellbuf` is already an indirect dependency and the frame does its own composition |
| bubbletea `examples/sprites`, `examples/splash` | — | **do not exist in v1.3.10** | — | — | **Ignore** (they are not there; do not cite them as prior art) |
| `Nomadcxx/sysc-Go` | MIT | 52 files / 515,909 B; repo 362 MB | app + library | full-screen effects: matrix rain, fire, fireworks, aquarium; ASCII art text effects | **Ignore as a dependency.** It requires `bubbles`, `lipgloss/v2` (a beta) and `gonum` (a large numerical library) for effects that are not a walking pet, and a 362 MB repository. Worth a look one day for its palette/animation registry idea — not for the companion |
| `sergev/goquarium` | **GPL-3.0** | 12 files / 77,832 B | app | ASCII fish in termbox-go | **Ignore.** Copyleft: code from it cannot enter this repository |
| `ansoni/termination` | **no LICENSE file** | 4 files / 19,330 B | library | shape as `map[string][]string` + a colour mask, 1-cell movement callbacks | **Ignore.** No licence means no permission; also termbox-go + rtreego deps, its own loop and `os.Exit`. Its "movements are one-cell callbacks" is exactly the easing-free design we are trying to improve on |
| `Emmyme/virtual-pet` | MIT | 5 files / 10,027 B | app | `map[state][]string` of 3-row cats, stats bars | **Ignore as a dependency** (an app). Note it as independent confirmation that a 3-row cat is the shape people reach for: `pet/cat.go` holds `happy`, `unhappy`, `asleep` as 3-row strings |
| `jonaustin/vpet` | **no licence**, module path `vpet` (not importable) | 14 files / 201,186 B | app | frames as raw multi-line strings with emoji, 200 ms per frame | **Ignore.** Unlicensed, unimportable, emoji-dependent, and 104 KB of its own tests |
| `kirkegaard/terminal-pet` | MIT | 28 files / 89,691 B | SSH server app (wish + sqlite3 + CGO) | `Animation{Name, Frames []string, FPS}` with 2–4 frames per state, `FPS: 2` | **Copy the idea only**: a per-state `{frames, fps}` struct is a clean way to state "the walk is faster than the sleeping face" without touching the global tick |
| `oneko` / `Neko` | unlicensed (C); `adryd325/oneko.js` is MIT but JavaScript | — | X11 desktop pet | behaviour: chase, dead zone, groom/scratch/yawn, sleep, wake on sudden movement | **Copy the behaviour, never the art or code** (§4.4) |
| `charmbracelet/harmonica` | MIT (+ a zlib-style port notice) | 7 files / 23,331 B, zero deps | library | damped spring | **Not adopted, but the only dependency that would earn it** if the user wants real overshoot. See §4.3 |

`bubbles/spinner`, `goquarium`, `sysc-Go` and `harmonica` were already "considered and rejected" for
the v1 companion on weight and licence-surface grounds
(`odd/tasks/installer-companion.md:203-205`); the re-examination above does not overturn that, and now
says why per candidate.

---

## 6. Three design options

All three share the same plumbing: the mouse gate of §3.1, the two clocks of §4.2, the placement rules
of §3.5, and the rule of §2.3 (the glyphs carry the state). They differ in how far they go.

The art below is **drawn for this proposal** and validated only for geometry (every row is exactly the
cell's width; the gaze and walk frames differ only where they should). I have not seen any of it in a
terminal — see §9.

### 6.1 Option 1 — "the cat gets a face and a gaze" (measured, smallest)

A 3-row, 14-column cat. The head is 11 columns with a 2-column prop column. Nothing but ASCII, no
colour, no dependency, one clock (8 fps), and the gaze has three horizontal positions: the pupil pair
slides one column left or right inside the head.

```text
   idle, gaze centre      idle, gaze left        idle, gaze right
    /\_____/\              /\_____/\              /\_____/\
   (  o   o  )            ( o   o   )            (   o   o )
     >  ^  <                >  ^  <                >  ^  <
```

```text
   asleep                 alert                  pleased                flinch
    /\_____/\              /\_____/\              /\_____/\              /\_____/\
   (  -   -  )z           (  O   O  )!           (  ^   ^  )\o/         (  >   <  )!
     >  ^  <                >  ^  <                >  ^  <                >  ^  <
```

```text
   walking A              walking B
    /\_____/\              /\_____/\
   (  o   o  )            (  o   o  )
     >  ^   <                >  ^  <
```

- **Creature**: the current creature, with a real face; 3 rows × 14 columns including the prop column.
- **How the art is built**: one 3-row template per state, hand-written, exactly like today's table —
  `companionFrames` becomes `[]struct{state, art []string}`. The gaze is three eye rows.
- **How the gaze follows**: horizontal only (3 positions), derived from `msg.X − 2` against the
  sprite's own columns; vertical is ignored.
- **How it moves**: unchanged — one cell per tick, the halving approach, the edge turn — plus the paw
  swap already present, now visible.
- **Cost**: ≤3 lines per changed frame instead of 1; ≈2.4 KB/s at 8 fps worst case; one new
  `MouseMsg` case; no new clock, no new dependency.
- **What can go wrong**: it is a *bigger* version of the same creature — the user's complaint
  ("demasiado simple") is only partly answered. It also cannot look up or down, and at the 80×24
  floor it fits only where the body leaves about four spare rows; where it does not, the shipped
  one-row art is what remains, so the improvement is not visible on every screen (see §7.3).

### 6.2 Option 2 — "the cat with a body, a walk and a gaze in two axes" (recommended)

A 5-row, 16-column cat (one slack row for the walk bob, so the cell is 6 rows tall), still pure ASCII,
plus a **computed gaze composer**: one art set per state, with the pupil pair placed at one of three
interior positions and the eye row placed on one of two head rows. Colour is optional seasoning: the
body can be tinted with an existing `AdaptiveColor` (e.g. `Text`/`TextMuted`) when the terminal has
colour, and nothing about the state depends on it.

```text
   gaze centre, level                  gaze left, level                    gaze right, level
    /\______/\                          /\______/\                          /\______/\
   (          )                        (          )                        (          )
   (  o    o  )                        ( o    o   )                        (   o    o )
   (     ^    )                        (     ^    )                        (     ^    )
    >  <  >  <                          >  <  >  <                          >  <  >  <
```

```text
 gaze up (eyes on the upper row)     walking A (legs apart)              walking B (legs in)
    /\______/\                          /\______/\                          /\______/\
   (  o    o  )                        (          )                        (          )
   (          )                        (  o    o  )                        (  o    o  )
   (     ^    )                        (     ^    )                        (     ^    )
    >  <  >  <                          >  <  >  <                           > <  > <
       >  <  >  <                          >  <  >  <                           > <  > <
    >  <  >  <                          >  <  >  <                          >  <  >  <
```

```text
   asleep                              alert                               pleased
    /\______/\  z                       /\______/\  !                       /\______/\\o/
   (          )                        (          )                        (          )
   (  -    -  )                        (  O    O  )                        (  ^    ^  )
   (     ^    )                        (     ^    )                        (     ^    )
    >  <  >  <                          >  <  >  <                          >  <  >  <
```

- **Creature**: a cat, 5 rows × 16 columns (+1 slack row for the bob). Four paws instead of two give
  the walk something to move.
- **How the art is built**: `companionArtFor(state)` returns rows from a table; a small composer
  `companionGazeRows(rows, gazeX, gazeY)` (a) stamps the pupil pair into the eye row at interior
  column 1, 2 or 3 from the left, (b) for "up", swaps the two interior rows. The table stays
  hand-written and reviewable; only the eyes are computed, which is what keeps 6 states × 3 × 2 from
  becoming 36 hand-drawn frames.
- **How the gaze follows**: `gazeX = clamp(sign(pointerStageCol − spriteCentre), −1, 1)` and
  `gazeY = −1` when the pointer is above the sprite's rows, `0` when it is level with them (down is
  deliberately not drawn: with a 5-row head there is no honest room for a third eye row, and a
  wrong-looking "down" is worse than a missing one). A two-column dead zone around the centre stops
  the pupils flickering while the pointer is inside the sprite.
- **How it moves**: the motion tick of §4.2 (24 fps while moving, 8 fps art), a hand-written
  critically-damped spring on `CompanionPos`, a two-cell dead zone before the pointer's column
  (oneko's `-idle`), the paw swap on the art tick, and the one-row bob on the motion tick.
- **Cost**: ≤5 changed lines per art frame (≈6.4 KB/s at 8 fps worst case, zero when still or asleep),
  one extra 24 fps timer armed only while moving, one `MouseMsg` case, one new env/flag pair, no new
  dependency.
- **What can go wrong**: the 5-row sprite does not fit where fewer than 6 rows are spare (§7.3), so
  it needs the compact fallback or it is *less* visible than today's sprite on small terminals; the bob
  makes the sprite's top row move, so the placement must reserve the slack row or the ears collide
  with the panel summary; and hover traffic in a multiplexer without 1003 is a silent no-op, which is
  easy to mistake for a bug.

### 6.3 Option 3 — "the half-block portrait" (the ceiling, and why I would not ship it)

A 16×16 pixel-art cat head encoded as **8 rows × 16 columns** of half blocks (two pixels per row, one
foreground and one background colour per cell), extensible to 24×24 = 12 rows. The gaze moves the
pupil pixels *inside the bitmap*. This is the most "realistic" a terminal pet can get, and it is the
option that costs the most.

```text
   the bitmap (16x16 tones: '#'=outline, 'o'=fur, '.'=highlight, ' '=nothing)
     o#        #o
     o##      ##o
     o###    ###o
    #o####  ####o#
    #oooooooooooo#
    #oooooooooooo#
    #o....oo....o#
    #o..##oo##..o#      <- the pupils are the '##' blocks; gaze moves them
    #o..##oo##..o#         one pixel left or right inside the '.', by redrawing
    #o....oo....o#         this bitmap, not by substituting a glyph
    #ooooo##ooooo#
    #ooooo##ooooo#
    #oooo#..#oooo#
     #oooooooooo#
      #oooooooo#
       ########
```

```text
   what the terminal actually receives: 8 rows of half blocks, two colours per cell
     ██▄      ▄██
    ▄████▄  ▄████▄
    ██████████████
    ████▀▀██▀▀████
    ████▀▀██▀▀████
    ██████████████
    ▀▀███▀▀▀▀███▀▀
      ▀▀▀▀▀▀▀▀▀▀
```

The second block is the honest demonstration of the problem: without colour it is a rectangle. The
glyph grid above *is* the picture — the whole picture lives in the two colours of each cell.

- **Creature**: the same cat, but drawn as pixels with shading (outline, fur, highlight, eye socket).
- **How the art is built**: a hand-drawn bitmap in the Go source (as tone digits), plus a ~30-line
  encoder to half blocks. The gaze redraws the bitmap's pupil pixels; expressions redraw the eye
  region, not the whole bitmap, so one bitmap + a small eye patch per expression covers the states.
- **How it moves**: as option 2, plus *sub-cell* vertical motion for free (the bob can be one pixel,
  not one row) — the only place in this whole proposal where a higher frame rate buys real smoothness.
- **Cost**: 8–12 changed rows per frame (≈10–15 KB/s at 8 fps, ≈30–45 KB/s at 24 fps), a bitmap, an
  encoder, a colourProfile decision, and a terminal matrix to test.
- **What can go wrong**: everything in §2.2. On a no-colour terminal it is a rectangle; on a font with
  gap-toothed block glyphs it is a striped rectangle; on a terminal with an odd cell height the half
  blocks are one pixel off (Alacritty #6201); at 80×24 it never fits at all; and it makes the sprite's
  legibility depend on colour, which contradicts the rule the repository already lives by. **Verdict:
  do not ship it as the default.** If it is ever wanted, it should be an explicit opt-in mode
  (`DOTFILES_SPRITE=blocks`) that degrades to option 2, and the option-2 glyph layer must exist first
  so there is something to degrade to.

---

## 7. Recommendation

### 7.1 Option, dependencies, decisions

**Option 2**, with option 1's art kept as the compact fallback (it is the same creature smaller), and
option 3 documented but not built.

| Decision | Choice | Why |
|---|---|---|
| Creature | a cat, 16 columns × 5 rows (+1 slack row) | It is the reference the project already cites (`oneko`), it is what every terminal pet in §5 draws, and its states read from the glyphs |
| New Go dependency | **none** | §5: nothing in the list earns it; §4.3: the one that comes closest (harmonica) is ~20 lines of arithmetic to avoid |
| Mouse | `tea.WithMouseAllMotion()` when the mouse gate is on; **no mouse option at all** when it is off | §3.1 gives hover only on 1003; §3.3: only "off at the source" gives normal selection back |
| Mouse gate | `DOTFILES_MOUSE=1`/`0` + `--no-mouse`, resolved before `tea.NewProgram` exactly as `--no-anim`/`DOTFILES_ANIM` are (`main.go:50,84-90`) | One gate per capability, one source of truth, testable without a terminal |
| Gate default | on, **except** when `TERMUX_VERSION` is set (then off unless `DOTFILES_MOUSE=1`) | §3.4: Termux has no hover and turns a swipe into a wheel report, so the gate would buy nothing and cost a behaviour |
| Frame rate | art 8 fps unchanged; **new 24 fps motion tick armed only while moving** | §4.2: it buys the timing of the last cells without touching the tip's 80-tick promise |
| Sprite | per-state rows in a hand-written table + a composer for the pupils and the eye row | §6.2: 6 states × 3 gaze columns × 2 rows stays reviewable |
| Colour | optional tint from the existing `AdaptiveColor`s; **no state depends on it** | §2.3 and `styles.go:11-23` |
| Placement | the sprite takes the last `H` spare rows above the footer rule, the panel summary above it; `H = 5`, else `3` (the compact art of §6.1), else `1` (the shipped art, kept as the last fallback), else nothing | It keeps the existing promise that the creature never takes a row from a body and that the summary wins when there is only one row (`companion.go:377-403`, asserted at `companion_test.go:559-680`). Keeping the shipped one-row set as the last step costs no art and no renamed tests, and it means no screen loses the creature it has today |
| Determinism | gaze cell, pointer row, position, velocity and both tick counters live on the model; `View()` stays pure and reads no clock | It is how a snapshot pins a frame instead of flaking (`companion.go:22-30`) |

### 7.2 Determinism and tests

The property to protect is the one the current code has: *the same model and tick render the same
bytes*. The mouse adds one more input (never a clock), so tests can pin it.

New or changed tests, in the shape the repository already uses (table-driven, one assertion per
property, a named constant per number):

1. `TestCompanionArtIsRowsOfPrintableASCII` (today: one row, `companion_test.go:140`) — every art row
   is printable ASCII, all rows of a frame are the cell's width, and every state draws.
2. `TestCompanionGazeMovesPupilsAndNothingElse` — over the 3 × 2 gaze grid, the changed characters
   against the neutral frame are exactly the two pupil cells (and, for "up", the two eye rows).
3. `TestCompanionMouseCellMapsToTheSprite` — the §3.5 arithmetic, asserted against a rendered
   `View()` at several sizes, so the mapping cannot drift from `placeCompanion`.
4. `TestCompanionTicksChangeOnlyItsOwnRows` (today: one row, `companion_test.go:459`) — a tick changes
   only rows the sprite owns, over the same ten fixtures.
5. `TestMotionTicksChangeOnlyTheSpritesRowsAndDoNotAdvanceTheTip` — the new clock's half of it.
6. `TestCompanionMouseGateOffMeansNoMouseOption` — the startup decision, in the shape of the existing
   animation-gate tests.
7. `TestCompanionSleepsUnlessTheMouseJumps` — a parked pointer does not keep it awake; a large jump
   and a click wake it.
8. Golden frames for the full set at 160×50 and for the compact fallback at 100×24, pinned to a tick
   and a gaze cell so they cannot flake.

### 7.3 Where each height fits (the degradation ladder)

The sprite may only use rows the body did not need, and the panel summary is placed in those same
rows and wins ties (`view.go:293-304`, `companion.go:377-403`). From the fixtures the tests already
use (`companion_test.go:459-470`) and the arithmetic of §3.5:

| Frame | Spare rows in the body | What the ladder draws |
|---|---|---|
| 227×62 and 160×50 main menu | ~45 | the full 5-row sprite (6 rows with the summary) |
| 160×50 welcome / keymaps / installing | varies with the body | full where ≥6 rows are spare, compact where ≥4, the shipped one-row art where ≥2, nothing below that |
| 80×24 main menu (the size every screen is guaranteed to work at) | 5, of which the one-line summary takes 1 | **today: 1 row. After the change: the compact 3-row sprite** — the full 5-row one needs 6 |
| 80×24 backup confirm with a long list | 0 (the body fills the frame; asserted at `companion_test.go:559-680`) | nothing, before and after |

Two things about that table are worth the user's attention. First, the 80×24 floor keeps the compact
sprite, not the full one — the ladder is what makes the taller creature safe. Second, the current rule
that the facts win a single spare row ("when the spare rows cannot hold both, the summary keeps its
rows and the companion is not drawn") stays: I am not proposing to trade a fact's row for a smaller
creature.

### 7.4 What the user still has to decide

1. **The creature and its name.** I have drawn a cat because the project already cites `oneko` and it
   is the shape every terminal pet uses. Another animal is a redraw of §6.2 and costs nothing else.
2. **The gate default**, and in particular whether Termux should default to "no mouse" as §7.1
   recommends, or whether the switch should be manual everywhere.
3. **The 80×24 rule**: accept that the creature is invisible on the narrow main menu (my
   recommendation), or let it take the summary's row there (the facts then lose a row).
4. **Whether the click reaction (§3.6) is wanted**, because it is the only part of this that is pure
   fun and it is the easiest to leave out.

---

## 8. Work breakdown (the recommended option, in reviewable slices)

Each slice is one commit-sized, independently reviewable change with its tests, its doc update and no
other slice's files. I would keep each under ~400 changed lines, which is where this repository's
review guidance puts the limit.

1. **Slice 1 — the mouse gate and the gaze state (no art change).** `DOTFILES_MOUSE`/`--no-mouse` in
   `cmd/dotfiles/main.go` beside `--no-anim`; `WithMouseAllMotion` chosen from the gate; a `MouseMsg`
   case that stores the pointer's stage column and row on the model and ignores repeated cells, wheel
   and buttons; tests 6 and 3 (the mapping arithmetic); docs: one paragraph in "Turning animation
   off"'s neighbourhood. With this slice alone nothing visible changes.
2. **Slice 2 — multi-row placement and the compact fallback.** Generalise `placeCompanion` to `H`
   rows, add `companionHeight(m)` (5 / 3 / 1 / 0), keep the summary's precedence; tests 4 and the new
   fixtures; the goldens. Still no new art: the fallbacks are today's one-row set and, from slice 3,
   the compact set.
3. **Slice 3 — the art and the gaze composer.** The 3-row compact set of §6.1 and the 5-row set of
   §6.2, the pupil/eye-row composer, the new states (blink, yawn-before-sleep); tests 1 and 2 and the
   state table; docs: rewrite the art table in `docs/tui-installer.md`.
4. **Slice 4 — the motion tick.** `motionTickMsg` at 24 fps armed only while moving, the hand-written
   spring, the dead zone, the walk bob, wake-on-jump/click; tests 5 and 7; docs: the frame-rate
   paragraph, including why the tip is still 80 ticks.
5. **Slice 5 — the last polish.** The click reaction if wanted, the sleep→yawn transition sequence,
   and the `THIRD_PARTY_NOTICES.md`/`LICENSE-AUDIT.md` check (which, note, is stale today: both files
   list `bubbles`, `cobra`, `viper` and `yaml` that `installer/go.mod` does not contain — worth a
   separate fix, not this change's).

---

## 9. Honest limits, and what I could not verify

- **The art is unvalidated in a real terminal.** Every frame above is validated for width and for
  "the gaze frames differ only at the pupils"; none of it has been rendered. The 5-row cat may read as
  a helmet rather than a cat, and the walk may read as a jiggle. The cheapest way to settle it is to
  build slice 3 against a golden file and look at it once.
- **Cost numbers are arithmetic, not measurements.** ≈800 bytes per changed frame and ≈6.4 KB/s are
  derived from the renderer's per-line diff and the row/column counts; I did not run a terminal and
  count writes. What *is* measured is the renderer's behaviour (60 fps cap, zero bytes for an
  unchanged view, per-line skip) and its source lines.
- **The selection claim is per-terminal and incomplete.** Shift is documented for kitty and Windows
  Terminal and is the de-facto convention; Option on iTerm2 is documented by iTerm2. I did not verify
  the exact modifier for every terminal the installer installs for, nor for Termux's soft keyboard.
- **Multiplexer support is partly verified**: tmux ≥ 3.4 forwards 1003 (tmux CHANGES), older tmux does
  not (issue #55). **zellij, herdr and GNU screen were not verified** for 1003 forwarding. If the
  answer matters, it is one manual test per multiplexer.
- **Termux**: I verified that a finger drag becomes a wheel event when mouse tracking is on and that
  mouse tracking captures the swipe; I did **not** verify what the installer looks like there, nor
  whether `TERMUX_VERSION` is always set in the Termux environment, which the gate default of §7.1
  would rely on.
- **Font coverage**: the claims are about specific fonts and terminals (DejaVu, JetBrains Mono, Zed,
  Alacritty, Ghostty, Windows Terminal). "Most terminals" is not a verifiable statement and I have not
  made one. This is exactly why the recommendation keeps the sprite in ASCII.
- **The pupil in a 1-column eye is a compromise.** In options 1 and 2 the gaze moves a pair of
  single-cell pupils; a reader may see the pair shift as the *face* moving rather than the eyes. If
  that reads badly, the fix is either wider sockets (more columns) or option 3's pixels — both cost
  rows or colour.
- **`bubbles`/`harmonica` versions** were read from the local module cache (`bubbles@v1.0.0`,
  `bubbletea@v1.3.10`) and from the GitHub API; the GitHub API rate limit was exhausted partway
  through the session, so `Emmyme/virtual-pet`, `kirkegaard/terminal-pet` and `jonaustin/vpet` were
  read before it ran out and their licence/size facts come from that pass. Nothing in this document
  depends on a rate-limited reading after that point.

---

## 10. Sources

Local (this repository, branch `feat/companion-v2`):

- `installer/internal/tui/companion.go:22-30,55-78,100-112,137-140,272-340,352-403`
- `installer/internal/tui/anim.go:39-110`
- `installer/internal/tui/layout.go:11-95`
- `installer/internal/tui/view.go:165-171,214-275,281-319,555-566,710-798`
- `installer/internal/tui/companion_test.go:140-195,459-470,559-680`
- `installer/internal/tui/testdata/TestMainMenuGolden.golden`, `TestCompanionGoldenFramesTheCreatureAtTickZero.golden`
- `installer/cmd/dotfiles/main.go:30,50,84-107`
- `installer/go.mod`, `installer/go.sum`
- `docs/tui-installer.md:156-316`, `odd/tasks/installer-companion.md:49-57,145-152,203-207`, `odd/tasks/companion-v2.md`
- `styles.go:11-23`, `THIRD_PARTY_NOTICES.md`, `docs/audits/LICENSE-AUDIT.md`

Bubble Tea v1.3.10 and its neighbours (module cache):

- `bubbletea@v1.3.10/options.go:121-170`, `screen.go:55-90`, `tea.go:400-470,664-674`,
  `mouse.go:1-70`, `standard_renderer.go:18-19,65-75,161-167,212-222,283-289`
- `bubbletea@v1.3.10/examples/mouse/main.go`, `examples/cellbuffer/main.go`; the v1.3.10 example tree
  (no `sprites`, no `splash`)
- `bubbles@v1.0.0/spinner/spinner.go`, `bubbles@v1.0.0/go.mod:5-11`, `bubbles@v1.0.0/progress/progress.go:12,151,235`

External:

- xterm control sequences: <https://invisible-island.net/xterm/ctlseqs/ctlseqs.html>
- 1003 traffic and support: <https://terminfo.dev/modes/decset-1003-all-motion-mouse>,
  <https://terminfo.dev/input/button-event-mouse-1002>
- tmux 1003: <https://github.com/tmux/tmux/blob/3.4/CHANGES>,
  <https://github.com/tmux/tmux/issues/55>, <https://github.com/tmux/tmux/issues/2721>
- Termux: <https://github.com/termux/termux-app/issues/1384>,
  <https://github.com/termux/termux-app/issues/4302>,
  <https://github.com/termux/termux-app/issues/5076>,
  <https://github.com/termux/termux-packages/issues/20655>
- Half blocks and sub-cells: <https://docs.rs/textual-rs/latest/src/textual_rs/canvas.rs.html>,
  <https://deepwiki.com/ratatui/ratatui-image/4.4-halfblocks-protocol>,
  <https://docs.frankentui.com/extras/vfx-rasterizer>
- Braille: <https://unicode.org/Public/18.0.0/charts/nameslist/2800/>,
  <https://bbs.archlinux.org/viewtopic.php?id=284774> (the stippling failure mode)
- Sextants and legacy computing: <https://unicode.org/charts/nameslist/n_1FB00.html>,
  <https://github.com/microsoft/terminal/pull/19841>,
  <https://github.com/dejavu-fonts/dejavu-fonts/issues/383>,
  <https://github.com/ryanoasis/nerd-fonts/issues/1674>,
  <https://github.com/zed-industries/zed/issues/56342>
- Block glyph rasterisation and its bugs:
  <https://github.com/alacritty/alacritty/blob/4225cea2/alacritty/src/renderer/text/builtin_font.rs>,
  <https://github.com/alacritty/alacritty/issues/6201>,
  <https://github.com/ghostty-org/ghostty/blob/65901966/src/font/sprite/draw/block.zig>
- Text selection: <https://github.com/kovidgoyal/kitty/pull/1184>,
  <https://github.com/MicrosoftDocs/terminal/blob/main/TerminalDocs/selection.md>,
  <https://iterm2.com/documentation-preferences-profiles-terminal.html>
- oneko/Neko behaviour: <https://en.wikipedia.org/wiki/Neko_(software)>,
  <https://www.mobygames.com/game/28106/neko/>,
  <https://code.irenes.space/oneko/plain/src/oneko.c>,
  <https://github.com/nucket/NekoAI/commit/1d49f6a3>,
  <https://github.com/radare/toys/blob/master/screencast/oneko/oneko.man>
- Candidate Go projects (GitHub API, 2026-09-29): `charmbracelet/bubbles`, `charmbracelet/harmonica`,
  `Nomadcxx/sysc-Go`, `sergev/goquarium`, `ansoni/termination`, `Emmyme/virtual-pet`,
  `jonaustin/vpet`, `kirkegaard/terminal-pet`, `adryd325/oneko.js`, `tie/oneko`
