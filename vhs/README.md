# Screen captures with VHS

`vhs/` holds the tapes that drive the built installer through a real terminal and
write PNG stills to `captures/`. They exist so a change can be judged by what the
interface looks like -- palette, weight, spacing -- and not only by the text
frames the tests pin.

## Run it

```sh
make screens
```

`scripts/screens.sh` builds `installer/dotfiles`, creates
`captures/<columns>x<rows>/`, and runs every tape from the repository root. The
captures are ignored by git; nothing here runs in CI or preflight.

Requirements: `vhs`, `ttyd` and `ffmpeg` on `PATH` (`brew install vhs` brings the
last two), or Docker:

```sh
scripts/screens.sh --docker
```

The Docker route mounts this repository at `/vhs` and uses the
`ghcr.io/charmbracelet/vhs` image, so nothing is installed locally. It builds a
Linux binary for the daemon's architecture first, because a macOS `go build`
would be a binary the container cannot start. The container writes the captures
as root.

Run it on the machine whose terminal you want to see. The tapes use the real
home, so the welcome, the plan and the overwrites are that machine's facts; the
Docker route shows a container's facts instead.

## What is captured

| Tape | Size | Screens |
| --- | --- | --- |
| `screens-wide.tape` | 200x55 | welcome, main menu, Utilities |
| `screens-floor.tape` | 80x24 | the same, at the documented floor |

The journey uses only keys that are stable on every host: `s` from the welcome
opens the menu and `u` opens the Utilities section. Deeper screens need host
state (managers, backups, a scan) and are not captured by a real-binary tape
yet; the deterministic frame fixtures in
`installer/internal/tui/screen_coverage_test.go` are the source for those.

## The tape grammar that matters

- `Output`, `Screenshot` and `Env` take a **quoted** value:
  `Screenshot "captures/200x55/01-welcome.png"`.
- `Enter` and `Escape` are **commands**, not text: write `Enter`, not
  `Type "Enter"`.
- `Set Columns`/`Set Rows` set the terminal grid; `Set FontSize`, `Set
  FontFamily`, `Set Padding` and `Set Theme` set the pixels around it. Use
  `Set Columns`/`Set Width` -- never both.
- `Wait+Screen /regex/` waits for the screen to contain the text; leave a
  `Sleep 700ms` before `Screenshot`, or the capture can land mid-repaint and show
  the previous screen's rows behind the new one. That is a capture artefact, not
  a rendering defect.
- `Set FontFamily "DejaVu Sans Mono"` is the portable default: the VHS container
  ships it and Linux hosts already have it. The installer draws no icon font
  glyphs, so a Nerd Font is not needed.

## Adding a tape

Copy one of the existing tapes and point it at a different journey. Keep the
screens reachable with stable keys; anything that depends on this machine's
managers, backups or scans belongs to the deterministic fixtures, not here.
