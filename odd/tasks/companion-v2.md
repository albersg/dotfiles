# Feature: the companion v2 — realistic, funny, and looking at the mouse

Status: research
Opened: 2026-09-29
Owner: research session in a visible Herdr pane (`../dotfiles-wt-mascot`, branch `feat/companion-v2`)
Orchestrator: the parent session reviews the proposal and turns it into work.

## Why

The user's verdict on the shipped companion (one row, seven ASCII frames, walks a cell per frame,
follows the cursor, sleeps, reacts) is that it is too simple: they want something much better -
realistic, funny, with the gaze following the mouse.

## Decisions already settled with the user

- **The gaze follows the real mouse**, accepting that normal text selection then needs Shift+drag on
  those screens, and that the feature must be gated independently of the animation gate.
- **External Go dependencies are allowed if they earn their place** (the user's words: "puedes usar
  dependencias externas si es necesario y merece la pena"). Each one judged on licence, size,
  maintenance, and what it saves. The default stays no new dependency.
- **Stay in Go.** No other language, and no terminal feature that cannot be relied on.

## Deliverable

A proposal (not production code, not committed): three design options with candidate art drawn as
text, a recommendation, the technical decisions, a work breakdown, and the honest limits.

## In parallel (other worktrees)

| Front | Worktree | Files it owns |
| --- | --- | --- |
| Trainer hint coherence (#59) | `../dotfiles-wt-trainer` | `installer/internal/tui/trainer/**` |
| Contextual panel (#60) | `../dotfiles-wt-panel` | `installer/internal/tui/panels*.go`, `view.go`, `update.go` |

The research session owns nothing yet: it reads and writes only its proposal document.

## Evidence log

| Task | Commit | Checks observed |
| --- | --- | --- |
| (opened) | - | - |
