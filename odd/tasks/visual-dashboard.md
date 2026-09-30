# Feature: the machine it is changing, live

Status: done (recovered and landed by the orchestrator after the worker's session ended)
Opened: 2026-09-29
Issue: #65
Worktree: `../dotfiles-wt-visual`, branch `feat/visual-dashboard`

## Why

The installer's screens had become dense and informative, and the machine doing the work was invisible:
a run takes minutes and the screen showed a progress bar and nothing about the host. The user asked for
"una mejora tremenda visualmente" and for the kind of dev-flavoured thing that looks good ("visualizar
recursos con gráficos, cosas visuales dinámicas").

## What shipped

- **A sampler that is not a renderer.** CPU busy, memory used and total, load average, free space on the
  target and process count, read about once a second from `/proc` on Linux and WSL and from the `sysctl`
  family on macOS, behind a probe interface so the formats and the failure paths are testable without a
  `/proc`. Readings live in a ring on the model; the derived values are computed in `Update`, never while
  drawing.
- **Sparklines and charts drawn by hand** with block glyphs and braille, so 16 colours and no colour lose
  nothing.
- **Where the graphs live**: a live panel in the rotation ("this machine, right now") and the installing
  screen, where the machine's pulse sits beside the run's progress and the run's own progress is charted
  over time. The pulse draws only from rows the rail and the log do not need.
- **A travelling highlight on the progress bar** during a long step, and a **celebration** when a run
  finishes: a burst of particles, a glyph difference rather than a colour one, gated with the animation.
- **The gate is honest**: with the animation off, the panel says the sampling is off instead of freezing a
  chart and calling it live.

## The rules it had to respect, and does

- 80x24 floor and both frame guards green at every size; the tests assert that a reading repaints only the
  rows a live widget owns, and that a screen showing no live widget changes no byte.
- Nothing depends on colour alone; no emoji-only meaning; Termux keeps working.
- Deterministic: samples, frames and positions come from the model, so a snapshot pins them.

## Notes from landing it

The worker's session ended before it reported, so the orchestrator reviewed and landed its tree. Two
things were fixed on the way in:

1. `TestWelcomeScreenGolden` was **flaky** - it started a program and captured everything it wrote after a
   fixed 100 ms sleep, and this screen now shows the live panel, whose metrics arrive from a command: the
   capture raced that read and pinned a different number of frames depending on scheduling. It renders a
   pinned model now, like its newer siblings, and is deterministic over twenty runs. Its snapshot changed
   by one line, the terminal-title sequence that only exists in program output.
2. No tracker document existed; this one was written when the work landed.
