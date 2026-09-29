# Feature: The installer uses the terminal it is given, and gets a companion

Status: in progress (S1 and S2 done)
Opened: 2026-09-29
Owner: autonomous session (design approved by the user in a brainstorm)

## Why

Measured on the user's terminal, from a screenshot of the shipped build:

| Measurement | Value |
| --- | --- |
| Terminal pane | ~227 columns x ~62 rows |
| Rows the main menu occupies | 8 of 62 = **13%** |
| Columns its text occupies | ~40 of 227 = **18%** |
| Selected row bar width | **227 columns** of bar behind ~20 characters |
| Selected row contrast | 9:1 (near-black ink on the light brand bar) - fine |
| Rule under the header | drawn, double the background luminance - fine |

So the screen is not ugly; it is empty, and one element is actively odd at that
size: the full-width row bar becomes a slab of colour across the whole terminal.
The design was tuned at 80x24 (the frame floor) and never asked what to do with
more room than it needs. The void predates this feature; centring the body made
it symmetric and therefore more visible.

The fix is not decoration. It is (a) a layout that responds to the room it has,
(b) real information in that room, and (c) one small living thing for warmth,
which the user explicitly asked for ("un muñequito de terminal que se vaya
moviendo, sin consumir muchos recursos").

## Design decisions (settled with the user)

- **Two columns on wide terminals.** `inner >= 120` columns: left = the screen's
  own body, right = a panel. Below that, today's single column, and the panel
  collapses to one rotating line above the footer. Threshold and widths are
  implementation constants with a test, not magic numbers in a renderer.
- **The row bar gets a measure.** The bar spans `min(inner, 80)` columns, not the
  whole terminal, and a row's metadata sits at the right edge of that measure.
  A 227-column slab behind 20 characters is not a design.
- **Panels carry data that already exists.** Nothing is invented for display:
  `system.Detect()` (`SystemInfo`, including the WSL host CPU/RAM/swap the
  `.wslconfig` feature already computes), `InstallStep` (the wizard's real plan),
  `system.ConfigPaths()`, `system.BackupInfo{Path, Timestamp, Files}`, the
  trainer's `UserStats`/`GameState`/`ModuleInfo`, and the keymap data the
  reference screens already ship. A tip is content the repository already wrote.
- **Every panel answers a question**: where am I (machine), what will this do
  (plan), what have I gained (trainer), what can I learn now (tip), when did I
  last run this (state). A panel that answers nothing does not ship.
- **The companion is ours: no new dependency.** Three glyphs of box drawing, no
  emoji, so Termux and a 16-colour terminal keep working and the branding and
  licence surface stays empty. Behaviour modelled on `oneko` (a pet that follows
  the cursor); art drawn here, never copied (the Go gopher is CC-BY, cowsay is
  GPL-ish, nyancat's cat belongs to its author).
- **Cost is a design constraint with evidence**: the renderer in bubbletea
  v1.3.10 returns early when the view string is unchanged and repaints only the
  changed lines, so a one-row sprite at 8 fps is ~150 bytes/s and 8 wakeups/s.
  Animate only where the companion is visible; no always-on ticker.
- **Determinism**: the animation frame comes from a tick counter in the model,
  and the tick is injectable, so snapshots pin a frame instead of flaking.
  `--no-anim` and `DOTFILES_ANIM=0` turn it off; non-TTY and `TERM=dumb` turn it
  off by themselves.
- **Nothing depends on colour alone**, the 80x24 floor keeps working, and both
  frame guards keep passing at every size the layout accepts.

## Slices

Each slice is its own issue, branch and PR: the whole feature is far past what a
reviewer should read at once, and every slice leaves the installer better on its
own.

### S1 - Responsive frame, capped row measure, and the first two panels

Tasks:
1. `layoutFor(m)` helper: available columns, two-column threshold, left/right
   widths, and the row measure; unit-tested at the boundaries (119/120/121,
   80x24, 160x50, 227x62).
2. The row bar measures `min(inner, 80)`; a row's trailing metadata sits at the
   right edge of the measure. At 80x24 nothing moves.
3. Two-column composition for the main menu: left = the menu rows, right = the
   active panel, with a shared way to place a panel under the same frame.
4. Panel **"Your machine"**: OS and name, architecture, WSL version, Termux or
   macOS, shell, package manager available, Xcode CLT, `$HOME`; and for WSL, the
   host's CPU/RAM/swap. Metadata as labels and values, dim labels.
5. Panel **"What will happen"**: the wizard's own steps with their descriptions,
   the current step marked, and the count of config paths that will be
   overwritten plus the newest backup and its age.
6. Guard tests at 160x50 and 227x62: no line exceeds the frame, the body fits,
   the right column never collides with the left, and the collapse threshold is
   respected. The 80x24 goldens must not move.
7. Docs and CHANGELOG.

### S2 - The remaining panels, `Tab`, and the narrow-terminal rotator

Tasks:
1. Panel **"Your trainer"**: per-module lessons and mastery, accuracy, best
   streak, the next boss and what it needs.
2. Panel **"Did you know?"**: a tip rotating every ~10s, sourced from the keymap
   data and the trainer's own exercises. No new copy to maintain.
3. Panel **"Last install"**: date, version and files touched, read from a small
   state file written when an install completes.
4. `Tab` cycles panels; the active panel's name is part of the frame, so it is
   visible where you are.
5. Narrow terminals (<120 columns): one rotating line above the footer, no
   truncation of meaning.
6. Guards and goldens for both modes; docs and CHANGELOG.

### S3 - The companion

Tasks:
1. Frame table and animator: 3-4 frames, 8 fps, one row, box-drawing only,
   injectable tick, `--no-anim` / `DOTFILES_ANIM=0` / non-TTY / `TERM=dumb` off.
2. Behaviours: walk, sleep after idle and wake on input, follow the cursor.
3. Reactions: alert on a destructive choice, celebrate a finished step, flinch on
   error; and in the trainer, react to right and wrong answers and carry a health
   bar in a boss.
4. Tests: the animator is deterministic and its cost is bounded (one row of
   change per tick); goldens pin frame 0.

### S4 - Remates

Tasks:
1. Installing screen: use the rows it has (the error panel currently cuts at 5 log
   lines), progress with an ETA and a file counter.
2. Trainer in two columns on wide terminals: code left, mission and explanation
   right.
3. Easter eggs in the menu only: `dd` sweeps the selected row away with a puff of
   dust, `:q` quits, `vim` opens the trainer. Navigation must not change.
4. Greeting by time of day on the welcome and the menu.

## Evidence log

| Task | Commit | Checks observed |
| --- | --- | --- |
| (tracker opened) | b5e8d41 | issue #47 |
| S1-1..3 layout, row measure, composition | b25f950 | 3324 pass; 10/10 snapshots untouched; both frame guards green; new guard at 160x50 and 227x62 |
| S1-4..5 panels, plan extraction, startup scan, `Arch` | 95e3b92 | 3369 pass; error snapshot moved whitespace-only (verified by diffing the non-blank lines, not asserted); wide snapshot added at 160x50 |
| S1-7 docs and CHANGELOG | d037943 | - |
| S1 fixes: branding audit, welcome truncation | PR #49, PR #50 | both green before merge (the first one taught that lesson) |
| S2 plumbing: registry, `Tab`, tab row, rotator, tick | aa6a4f7 | 3459 pass; no snapshot moved; `Tab` was already bound in the trainer but is not stolen, pinned by a guard |
| S2 content: three panels, tips, state file, docs | 02d0d19 | 3473 pass; three snapshots moved, every moved line additive (verified by diffing the non-blank lines) |

### S2 notes

- **`Tab` was already taken** by the trainer's hint reveal. The cycle fires only on a
  screen offering more than one panel and trainer screens offer none, so nothing was
  stolen - but the next person to give a trainer screen a panel must handle this.
- **The full four-panel tab row is exactly 62 columns at the 124-column floor**: zero
  slack. A fifth panel or a longer title fails a test rather than wrapping badly.
- **The tip pool is 621 tips** from the keymap reference data and the trainer's
  lessons, in declared order, no randomness, one every ten seconds on the animation
  tick. 61 trainer lessons whose first sentence does not fit the narrowest panel are
  left out, so no shipped tip is ever cut.
- **`trainer.LoadStats()` conflates a missing file with a corrupt one** (both `nil`),
  and a file that records nothing parses to zeros. The panel treats all three as "no
  run" and says so in one line. A corrupt profile is therefore silently ignored: worth
  a follow-up in the trainer package, not here.
- **The last-install record** lives at `$XDG_STATE_HOME/dotfiles/last-install.json`
  (`~/.local/state` as fallback), is written best-effort when a run completes, and is
  read on the startup path. Tests point the state directory at a temporary one.

### S1 notes

- **The two-column floor is 124 terminal columns, not 120.** The constant holds the
  *content* width and the frame spends two columns of padding on each side. Any
  future threshold change has to move the constant and the test together.
- **The plan needed a pure builder.** It did not exist before the main menu's panel
  asked for it: `SetupInstallSteps` ran at the editor step, so the panel would have
  been empty on the screen where it is shown. `planFor(planOptions)` is now the one
  source and `SetupInstallSteps` fills the struct.
- **Two facts moved to startup** so the panel is true rather than empty: the
  existing-config scan (the non-interactive path already scanned there) and
  `SystemInfo.Arch`. The startup scan fills only an empty field, so the wizard's own
  scan stays the authority and a seeded model is not overwritten.
- **A panel shows only what the installer measured.** No row for an unmeasured fact,
  ever - not `unknown` and not a guess. A panel with more rows than the frame has
  says how many it could not show.
- **Descriptions are read where the step runs.** The plan panel numbers the steps and
  keeps the description of the step the run starts at (or is on); the rest are on the
  installing screen. This is a deliberate placement, not lost information.
- Open question for S2: `model.go`'s `menuSeparator()` still builds a full-width dash
  string while the renderer draws at the row measure. Detection is by prefix so
  nothing breaks, but the data should agree with the render.

## Out of scope

- Full-screen effects and any new Go dependency (`goquarium`, `sysc-Go`,
  `bubbles/spinner`, `harmonica` were all considered and rejected: weight for
  something we can do in 90 lines, and a licence surface we do not want).
- Animating on screens where the companion is not visible.
- A network call for "what's new": the version line stays local.
