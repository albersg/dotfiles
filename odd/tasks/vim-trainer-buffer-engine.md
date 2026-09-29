# ODD Feature Tasks: Vim Trainer buffer engine and the capabilities it unlocks

## Goal
Teach the trainer the capabilities it cannot verify today — undo/redo (`u`, `Ctrl-r`), insert
entries (`i a o O I A`), yank/put (`y p P`), match-pair (`%`), marks (`m`, backtick), indentation
(`>>`, `<<`) and visual block (`Ctrl-v`) — by giving the simulator a real buffer, modes, registers
and undo history, judged by the resulting state rather than by the cursor alone.

## Problem and rationale
The trainer has two judges, and neither can see a buffer change:

- **Exact match** (`ShouldSkipSimulation` → `IsInSolutions`, `validation.go:27-33,69-74,136`): the
  answer is correct if it equals one of the authored `Solutions` strings. Used by Substitution,
  Macros and Regex.
- **Simulator** (`SimulateMotionsWithSelection`, `simulator.go:51`): correct if the resulting cursor
  position matches, plus the selection range when the optimal produces one
  (`validation.go:81-99`).

`simulator.go` moves a cursor and computes selection ranges over **immutable** `code`. It has no
concept of buffer mutation, registers, undo history, insert mode, marks, `%`, indentation or visual
block; `u`, `p`, `%`, `m`, `>`, `<` all fall to the `default:` branch and are marked unrecognised.

So every capability in this feature is unverifiable today: its effect lives in the buffer, and the
buffer never changes. Worse, nine exercises already *promise* these capabilities in their prose
while the judge checks a different command:

| Exercise | Promises | Actually judged |
|---|---|---|
| `textobjects_020`, `textobjects_021` | `yiw`/`yi"` "then paste with p" | string match of the yank |
| `changerepeat_004`, `changerepeat_009` | "yy yanks line", "cc preserves indentation" | selection only |
| `substitution_007`, `substitution_008` | `S` preserves indentation | string match |
| `macros_011`-`macros_015`, `macros_boss_5` | registers `"ay`, `"ap`, `"+y`, `"+p`, `"0p` | string match |
| `macros_016`-`macros_019` | `:normal I…`, `:normal A…` | string match |
| `substitution_018`, `macros_018` | `%` (as a range) | string match |

That gap is the feature's motivation: a mission should not claim a result the trainer cannot check.

## Accepted decisions
- **The user chose the full-fidelity option (C)**: a real buffer with insert mode and registers, so
  a mission can claim a result and the trainer can verify it. Estimated 1500-2200 production lines
  across slices.
- **The buffer engine is OPT-IN per exercise, not a replacement of the current judge.** A new
  field on `Exercise` defaults to the existing motion/selection path, so the 156 shipped exercises
  and their tests keep running on the code path they were written against. The explorer's warning
  was explicit: mutation always-on breaks validation's position comparisons, the selection math and
  the render loops. Opt-in buys the fidelity without that blast radius.
- **The answer stays a flat keystroke string; the screen does not become a live editor.** The
  engine evaluates the string on submit and the result screen shows the resulting buffer, which is
  what makes the lesson teachable. A live editor would need a new input model and a new renderer,
  and it is not required to verify or to teach these commands.
- **Escape needs a token, because the answer is a string.** `Esc` is the trainer's global exit key,
  so an explicit, visible token stands for it in the answer; the key that inserts it and the token
  it renders are documented on the exercise screen. The module-level decision (which key) is taken
  in task E5.
- **Undo follows Vim's granularity**: one snapshot per normal-mode command, and one block per insert
  session closed by the escape token.
- **Registers exist as data**: unnamed, `0`, named `a`-`z`, and `+` aliased to the unnamed register
  since the trainer has no system clipboard. Linewise versus charwise is tracked, because `p` and
  `P` mean different things for each and that is exactly the lesson.
- Slices are separate branches and PRs: engine first, then the modules, then the honesty pass and
  visual block. Each engine task is sized for one bounded writer.

## Workflow configuration
- Route: ODD. One bounded writer per task, one independent read-only verifier per slice, and the
  parent commits each work unit.
- Exact runner: `cd installer && go test ./internal/tui/trainer/... -count=1` plus
  `cd installer && go test ./... -count=1` before a slice closes.
- Delivery: branch, push (after the commit, not before), PR with `Closes #N`, CI green, merge.
- The regression bar for every engine task: the 156 shipped exercises, their tests and the three
  trainer goldens must behave exactly as before.

## Slice 1 — engine (`feat/trainer-buffer-engine`)

### E1 — Engine core: buffer, cursor, history, normal-mode line mutations
- [x] New `editor.go` with a mutable buffer (rune-aware columns), cursor, undo/redo history and one
      additive entry point that takes the original code, the start position and the answer string.
- [x] Normal-mode line mutations: `dd`, `yy`, `p`, `P`, `>>`, `<<`, plus `x` and `D` on the current
      line. `u` and `Ctrl-r` undo and redo with Vim's granularity.
- [x] `SimulateMotionsWithSelection` is untouched, so the existing corpus cannot regress.
- Check: 573 trainer tests, 1940 module tests; `gofmt` and `go vet` clean; no golden moved.
- Route: delegated writer.
- Evidence: RED observed as a build failure against the specification tests, then 117 subtests green.
  A regression-bar test pins the exact pre-change outputs of the motion simulator and the recognizer.
  Decisions taken where the brief left room: the buffer always holds at least one line, as Vim's
  does; the indent unit is two spaces; a snapshot is recorded only when the buffer changed; and the
  engine parses its own command set because the motion recognizer deliberately marks `x`, `p`, `>`,
  `<` and `u` unrecognised, keeping one exported notion (`Recognized`) rather than two. Commit `a9d42dd`.
- Checked and reported: no shipped exercise has non-ASCII in its `Code`, so the rune-versus-byte
  difference is not observable on today's corpus, but the new engine counts runes anyway.

### E2 — Motions inside the engine, delegated rather than reimplemented
- [x] The engine can move the cursor, so real answers such as `3Gdd`, `jdd` and `wD` parse.
- [x] Delegate to the existing motion parser instead of writing a second one: the per-command parse
      that already reports how much input it consumed moved out of the simulator's loop into one seam
      both callers use.
- [x] The motion semantics of the 156 shipped exercises must not change.
- Check: 933 trainer tests, 2300 module tests; `gofmt` and `go vet` clean; no golden moved.
- Route: delegated writer.
- Evidence: RED observed with 361 failing assertions, then green. The delegation is proved rather than
  asserted: for motion-only answers the engine's cursor must equal what the motion simulator returns
  for the same start, code and input, checked over three fixtures, three start positions and 36
  inputs. The `simulator.go` change is a mechanical extraction, confirmed by a normalised line diff
  whose only differences are `continue`/`break` becoming returns. Commit `d577709`.
- Why it existed: E1 shipped without motions, which made every answer that repositions the cursor
  unrecognised, and that is what the content needs most.
- Deliberately passed on: operator-plus-motion answers such as `$d$` and `yiw` need character-wise
  register content and stay unrecognised until E5, which owns registers.

### E3 — Judging by buffer, the opt-in field, input and the result preview
Ordered before the remaining engine features on purpose. This is the integration risk: the place where
the TUI, the judge and the engine must agree. Proving it now, with the mutations and motions that
already work, is cheaper than discovering a mismatch after insert, registers, `%` and marks are built
on top of it.
- [x] `Exercise` gains the opt-in field, and its zero value keeps the current judge, so the 156
      shipped exercises take the path they were written for.
- [x] The buffer path in `ValidateAnswerDetailed` compares the resulting buffer text, cursor and mode
      against the optimal's, and reports a useful mismatch message naming what differed.
- [x] `Ctrl-r` can be typed; it was ignored, which would have made a redo lesson unanswerable.
- [x] The result screen shows the resulting buffer, and only for buffer-verified exercises.
- Check: 959 trainer tests at the time, 2333 module tests; no golden moved; `gofmt` and `go vet` clean.
- Route: delegated writer, then a differential verification against real nvim.
- Evidence: RED observed as a build failure against the new API, then green. The equivalence pair is
  `2Gdd` against `jdd` on a three-line buffer: same buffer, same cursor, and `jdd` is deliberately
  absent from `Solutions`, so correctness comes from the result rather than from the keys. A rejected
  answer names what diverged from the compared results, and the message stays byte-identical for every
  exercise that does not opt in. A guard test asserts that no shipped exercise opts in, which turns
  the constraint into something the suite enforces. Commit `01dcc07`, plus two fidelity corrections the
  differential verification forced: `a6ec2a6` and the counted-linewise guard.
- Known gap, reported rather than hidden: the boss path does not store the validation, so a
  buffer-verified boss step would be judged correctly and show no buffer. The module slice decides.

## Fidelity notes, with nvim as the reference
The engine is verified differentially against `nvim --clean --headless` with
`set shiftwidth=2 expandtab tabstop=2 startofline`, one fresh process per case, and the buffer loaded
from a file. Both matter: the API joins the first undo block, and scripted input coalesces a whole
script into one block, so undo granularity cannot be settled any other way.
- Corrected after the differential run: indentation is a width in columns, so a tab advances to its
  tabstop and expandtab writes spaces, with truly empty lines skipped and whitespace-only lines
  shifted; undo of a linewise change lands on the first non-blank, as the mutation itself already
  did; `[count]D` is refused instead of parsed and ignored; and a counted linewise operator starting
  on the last line is a no-op for `dd`, `yy`, `>>` and `<<` alike. Indentation and landing cases went
  from seventeen of thirty-seven matching to all thirty-seven.
- The last of those was surprising, so it was reproduced before it was implemented, and the table
  that settles it carries twenty-eight nvim-referenced rows including the controls that must still
  clamp.
- Known and deliberately left alone: the motion simulator's `^` and `$` on an all-blank line and its
  `curswant` with tabs differ from nvim, but that path serves the 156 shipped exercises. Character
  wise register content arrives with E5. nvim gives a failed command its own undo block while the
  engine treats a no-op as no undo point, and existing assertions depend on the engine's choice.

### E4 — Insert mode and the escape token
- [ ] `i a o O I A` enter insert mode at the right position; typed text is inserted; the escape token
      leaves insert mode; the whole session is one undo block.
- [ ] The escape token in normal mode is a no-op rather than an error, and the key that types it is
      documented on the exercise screen next to the other trainer keys.
- Tests: every entry command, typing at line start and end, insert into an empty buffer, undo closing
      the whole session in one step, and the cursor position after each escape.
- Route: delegated writer.

### E5 — Registers and the yank/put pair
- [ ] `y` with a motion and `yy` fill the right register with the right linewise/charwise flag;
      unnamed, `0`, named `a`-`z` and `+` behave as Vim's do; `p`/`P` put by the flag, which is what
      makes `p` mean the right thing for a line and for a few characters.
- Tests: linewise put above and below, charwise put after and before the cursor, `"0p` after a delete
      changed the unnamed register, a named register surviving other yanks, and an operator-plus-motion
      yank such as `yiw` followed by `p`.
- Route: delegated writer.

### E6 — Match-pair and marks as real motions
- [ ] `%` jumps to the matching bracket, scanning the whole buffer. It is a motion, so the existing
      position comparator verifies it with no new judge.
- [ ] `m{a-z}` sets a mark, the backtick jumps to its exact position, the single quote to the first
      non-blank of its line, and marked positions follow buffer edits where Vim's do.
- Tests: nested brackets, unmatched bracket, a mark after a line was inserted above it, and both jump
      forms.
- Route: delegated writer.

## Slice 2 — the module (`feat/trainer-buffer-module`)

### M1 — Undo, redo and insert entries
- [x] A new module with lessons for `u`, `Ctrl-r` and the six insert entries, plus a boss. It shipped
      as "Editing & Undo" rather than `exercises_buffer.go`, with twenty-three lessons and the boss
      "The Historian".
- [x] Every mission states a result the judge can verify, and every lesson opts into the buffer judge.
- Check: the two invariant tests pass, and the shipped-corpus guard was rewritten to allow exactly
      this module and the one that followed.
- Evidence: `TestEditingModule_EveryOptimalValidates` and
      `TestEditingModule_PlausibleWrongAnswersAreRejected`. Registering the eighth module moved the
      counts the rest of the suite pins, including four assertions in the end-to-end test that this
      task's surfaces did not include and the parent applied. The wrong-answer test is the one worth
      reading: deleting without undoing makes the buffer differ, forgetting the session undo leaves
      the line behind, and forgetting a mark jump leaves the buffer identical and the cursor wrong,
      which is the case that proves the judge compares more than text. Commit `ca5fe26`.

### M2 — Yank, put and indentation
- [x] A second module, "Registers & Indentation", with twenty lessons and the boss "The Archivist":
      yanking a line and putting it below and above, yanking a word and putting it before and after
      the cursor, a named register, `"0p` after a delete moved the unnamed register, moving a line with
      `dd` then `p`, and shifting with counts.
- [x] Tests: both invariants of M1, and nine plausible wrong answers rejected with a mismatch that
      names what differed.
- Evidence: the lesson about register 0 is the reason the register model is worth teaching, and the
      wrong answers are the point of the judge: `"0p` where the unnamed register was asked for pastes
      the deleted line rather than the yanked one. Commit `45447ca`.
- Surface the task closed on the way: the engine had no range shift (`>j`, `Vj>`), which the module
      had to work around with counted shifts. That gap became E7.

### M3 — Registration, unlock order and the menu
- [x] Folded into M1 and M2 rather than done separately: each module registers itself the way the
      skill's decision tree prescribes, so a module arrives complete and testable in one task instead
      of depending on a later one to exist. `ModuleID`, the unlock order, `GetAllModules` with name,
      icon, description and boss name, the lesson and boss dispatch, `NextModule`, and every count
      assertion in the existing tests moved with each module, and the menu golden is regenerated and
      still seeded so the counts in it are proven rather than assumed.

## Slice 3 — honesty and the last capability (`feat/trainer-buffer-honesty`)

### H1 — The nine exercises that promise what they cannot check
- [x] Seventeen exercises in total were made honest, not nine: eight are backed by the buffer judge
      with answers that finish the job the prose describes, and nine are reworded because what they
      promise is an ex command the engine does not model. A second pass found five more in the macros
      module and reworded those too.
- [x] The judge order changed so this is possible at all: the buffer check now runs before the
      skip-simulation bypass, which is what lets an exercise in any module be verified by its result.
- [x] A guard test keeps the class from returning, and its blind spots are written down rather than
      implied: prose that names no mechanism, a single-line `:s` claim without a result shape, and
      promises written in a language other than English.
- Evidence: commits `cc4620c` (amended to `03e0a7d`), and the second pass in the facelift close-out.
      The verifier that reviewed the second pass is the one that found the remaining five, which is
      why the task exists twice.

### H2 — Visual block
- [x] `V` for linewise and `Ctrl-v` for blockwise selection, with `d`/`x`, `c`, `y`, `>` and `<` on
      lines and `d`/`x`, `y`, `I` and `A` on blocks, where `I` and `A` insert the text typed afterwards
      on every line of the block. `Ctrl-v` became typeable and the legend documents it beside the
      escape token.
- [x] The engine returns the selection it made, so a screen can draw it, and the two modes are
      covered by forty-five subtests against the reference.
- Evidence: commit `db968bf`. The rules worth knowing came from nvim rather than reasoning: `I` skips
      a line shorter than the block's left edge while `A` pads every short line with spaces, and a
      block that runs past a short line extends instead of failing. Blockwise `$`, blockwise shifts
      and counted visual operators are refused explicitly rather than approximated, because emulating
      them needs a cursor model this engine does not have.

## Acceptance criteria
1. [ ] A mission in a buffer-verified exercise can claim a result, and a wrong answer is rejected
   with the difference reported.
2. [ ] The 156 shipped exercises, their tests and the three trainer goldens are unchanged in
   behaviour.
3. [ ] Every capability in the goal is either verifiable or explicitly reported as out of reach.
4. [ ] No mission promises a result its judge cannot check.
5. [ ] `go test ./...`, `gofmt` and `go vet` clean before each slice is pushed.
6. [ ] One PR per slice, CI green, and the user decides each merge.

## Progress
- 2026-09-28: Feature opened. A read-only cost analysis compared exact-match modules, a line-level
  buffer model and the full engine; the user chose the full engine knowing the estimate. The opt-in
  decision and the flat-answer decision are recorded above because both bound the blast radius.
  Branch `feat/trainer-buffer-engine` created from `main` at `0c3f2ac`.
- 2026-09-28: E1 committed as `a9d42dd`, E2 as `d577709`. The E1 report added a task the plan was
  missing, because the engine had no motions and could not parse the answers the content is made of;
  E2 delegates motion parsing to the existing simulator and proves the delegation with an equality
  invariant instead of asserting it.
- 2026-09-28: Slice reordered. The judge wiring moved ahead of insert mode, registers and `%`/marks,
  because it is the integration risk where the TUI, the judge and the engine must agree, and proving
  it with the mutations and motions that already work is cheaper than discovering a mismatch after
  three more engine features sit on top of it. The escape token moved to the insert task that needs it.

- 2026-09-28: E3 committed as `01dcc07`, corrected twice by the differential verification as
  `a6ec2a6` and the counted-linewise guard. The verification also proved the motion seam is
  behaviour-identical to `main` by fuzzing 60,504 cases, and confirmed the opt-in gate, the bypass and
  the scoring are textually unchanged.

## Delivery
- **Merged to `main`** through two pull requests: `#40` for the engine, the judge, the two modules and
  the honesty pass, and `#42` for the installer facelift that followed. Both had every CI check green
  before the merge, and the local installer binary was rebuilt from the merged `main`.
- Every task above is committed on its own, with the tests that pin it, and each slice had either a
  differential verification against `nvim` or an independent read-only review. Those verifications
  found real defects in the work itself, not in the plan: four Vim-fidelity divergences, a boss hint
  that named the wrong step, a window that could cost two lives, a menu render that wrote to the
  player's profile, and a scroll window that hid the tail of the longest lists. All were fixed before
  landing.
- Left deliberately undone and recorded in the pull requests rather than hidden: ex commands cannot be
  verified, cross-line `d%`/`y%` and blockwise `$` are refused, the exercise screen's live preview
  still uses the motion simulator, and a negative cursor column is unguarded while the out-of-range
  line and column are.
