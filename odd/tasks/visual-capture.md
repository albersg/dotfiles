# Feature: visual capture of the installer's screens

Status: done
Opened: 2026-10-20
Owner: autonomous session, branch `feat/visual-capture`

## Why

The TUI's tests prove where every row lands, but they render text: no palette,
no weight, no idea of the space a screen leaves. The report that started this
front was about how the interface *looks* ("no se puede hacer mucho mejor, mucho
más bonito"), and the text frames could not answer it. VHS
(https://github.com/charmbracelet/vhs) drives a real terminal and writes PNG
stills, so the interface can be reviewed as it is drawn.

## What was proven

- VHS 0.12.1 runs here both ways: locally (`brew install vhs`, which brings
  `ttyd`; `ffmpeg` and Chromium were already present) and through
  `ghcr.io/charmbracelet/vhs` with Docker, nothing installed.
- Tapes capture the real binary at 200x55 with JetBrains Mono/Meslo, the dark
  palette and the real home: welcome, main menu, Utilities, Vim hub and the theme
  picker. The images are 2250x1358 (70-380 KB) and readable.
- The tape grammar's sharp edges: quoted `Output`/`Screenshot`/`Env`; `Enter` and
  `Escape` as bare commands (`Type "Enter"` types letters); `Set Columns`/`Rows`
  for the grid; a `Sleep` of ~700 ms before `Screenshot` or the capture can land
  mid-repaint (the previous screen's rows show behind the new one -- a capture
  artefact, not a defect).
- What the real captures showed, and the text frames could not:
  1. At 200x55 the main menu, Utilities and the Vim hub occupy the top ~15 rows
     and leave ~30 empty. The vertical composition is the next visual problem,
     not the colour or the frame.
  2. The theme picker's live preview of a light theme (Catppuccin Latte) draws
     the muted prose on the drift-tinted background at low contrast; the drift
     tint darkens the light base and costs the muted role ~0.6 of its ratio.
  3. The card and the panel start one row apart (panel tab row above the card's
     top border), and the companion is the minimal sprite at tall frames.
- The real-binary journey is host-dependent: the update row, backups and the
  managers change the rows, so a tape that presses a fixed number of downs is
  brittle. The stable keys (`s`, `u`, Enter, Escape) are the ones the harness
  uses.

## What was added

| Path | Purpose |
| --- | --- |
| `vhs/screens-wide.tape`, `vhs/screens-floor.tape` | Welcome, main menu and Utilities at 200x55 and 80x24 |
| `scripts/screens.sh` | Builds `installer/dotfiles`, runs the tapes locally or through Docker, writes `captures/` |
| `Makefile` `screens` | `make screens` |
| `.gitignore` `captures/` | The artifacts stay out of git |
| `vhs/README.md` | How to run it, the grammar, the portable font, the limits |

`make screens` is opt-in: it is not a CI check and preflight never runs it (VHS
needs Chromium, ffmpeg and a terminal at real sizes).

**Evidence.** `make screens` on this machine built `installer/dotfiles`, recorded
both tapes and wrote `captures/200x55/{01-welcome,02-main-menu,03-utilities}.png`
plus `session.gif` (and the same at 80x24); `git status` stayed clean because
`captures/` is ignored. `shellcheck --severity=warning scripts/screens.sh`
passes. The run used the real home, so the plan, the 17 overwrites and the newest
backup are this machine's facts. The captures on `main` show the interface before
the interface-cohesion branch; the harness is independent of it.

## Remaining

- Deterministic coverage of the stateful screens (update center with entries,
  disk cleaner with findings, doctor, complete/error): reuse the fixtures in
  `installer/internal/tui/screen_coverage_test.go` as truecolor ANSI frames and
  let VHS render them, or add a hidden capture mode to the binary. Until then the
  real-binary tapes cover the screens reachable with stable keys.
- The three visual findings above are follow-up UI work, not part of this front.
