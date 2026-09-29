# Feature: the panel describes the option you have selected, not only the plan

Status: in progress
Opened: 2026-09-29
Issue: #60
Owner: autonomous session, on a worktree of its own (`../dotfiles-wt-panel`, branch `feat/contextual-panel`)
Orchestrator: the parent session merges; the writer never does.

## Why

The main menu's panel explains the installation whatever the cursor is on, including on
`Restore from Backup` and `Keymaps Reference`. The user asked for it to follow the selection.

## Scope

1. The main menu's panel describes the option under the cursor, with real state where it exists: the
   plan and the configs it will overwrite, the backups with their dates, what the trainer holds, what
   the reference and guide screens contain, an honest line for quitting.
2. The **panel name stays stable** so `Tab` navigation does not wander; the selected option is named
   inside the panel.
3. The wizard's choice screens do the same for the choice being made, and still mark the current step.
4. The trainer, tips and last-install panels stay reachable with `Tab`.

## Non-negotiable rules from the earlier slices

- A panel shows only what the installer has measured: no row for a fact it does not hold, never
  `unknown`, never a guess.
- It never truncates a line: it wraps, or it says how many rows it could not show.
- The render path stays pure, and a tick changes only the companion's row and nothing else.

## In parallel (other worktrees)

| Front | Worktree | Files it owns |
| --- | --- | --- |
| Trainer hint coherence (#59) | `../dotfiles-wt-trainer` | `installer/internal/tui/trainer/**` |
| Companion v2 (#61) | `../dotfiles-wt-mascot` | `installer/internal/tui/companion*.go`, `anim.go` |

This front owns `panels*.go`, `view.go`, `model.go` and `update.go` this round.

## Evidence log

| Task | Commit | Checks observed |
| --- | --- | --- |
| (opened) | - | - |
