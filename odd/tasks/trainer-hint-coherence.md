# Feature: the trainer's hint must add something the description does not already say

Status: in progress
Opened: 2026-09-29
Issue: #59
Owner: autonomous session, on a worktree of its own (`../dotfiles-wt-trainer`, branch `fix/trainer-hint-coherence`)
Orchestrator: the parent session merges; the writer never does.

## Why

Reported by the user: "en la descripción del ejercicio pone que uses w, y luego en la hint, pone que use
w, es decir, pierde sentido la w". A hint that repeats the description costs a keypress and teaches
nothing, and the question is whether it is a few lessons or a rule nobody wrote down.

## Scope

1. Inventory the exercises whose hint repeats or spoils the description, with the classes.
2. Write the rule: description = the goal, hint = the how. Checked against the content that exists.
3. Apply it, changing copy: an exercise whose hint cannot add anything loses the hint.
4. A guard test for the class, in the spirit of the honesty pass that already guards this trainer.

## In parallel (other worktrees)

| Front | Worktree | Files it owns |
| --- | --- | --- |
| Contextual panel (#60) | `../dotfiles-wt-panel` | `installer/internal/tui/panels*.go`, `view.go`, `update.go` |
| Companion v2 (#61) | `../dotfiles-wt-mascot` | `installer/internal/tui/companion*.go`, `anim.go` |

This front owns `installer/internal/tui/trainer/**`. If it needs `view.go`, it says so: the panel front
is the one that owns that file this round.

## Evidence log

| Task | Commit | Checks observed |
| --- | --- | --- |
| (opened) | - | - |
