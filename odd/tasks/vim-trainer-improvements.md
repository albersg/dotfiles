# ODD Feature Tasks: Vim Trainer improvements

## Goal
Fix the defects that let the Vim Trainer pass wrong answers, register wrong progress and lose
progress; then turn the dead timing data into a real countdown; then make progress visible and
removable from the UI; finally remove the dead surface the exploration found. Delivered in three
review-sized slices.

## Problem and rationale
Exploration of `installer/internal/tui/trainer/` (~6,500 lines, 7 modules, 156 lessons, 35 boss
steps) found five classes of problem, each confirmed by reading the code:

1. **Validation accepts garbage.** `ValidateAnswerDetailed` compares only the final cursor
   position (`validation.go:60-72`). When the optimal solution is an operator or a text object
   (`viw`, `dd`), the cursor does not move, so the "target" position is the starting position and
   any input the simulator does not understand also stays there. `q` or `x` therefore passes all 21
   Text Object lessons and all 24 Change & Repeat lessons, plus their bosses. The selection range
   is already computed by `SimulateMotionsWithSelection` and never consulted.
2. **Practice attempts are counted twice.** One correct submission calls both
   `RecordCorrectAnswer` (which increments `PracticeAttempts`/`PracticeCorrect`) and
   `RecordPracticeResult` (which increments them again), so the 10-attempt boss gate is reached in
   5 answers and the accuracy denominator is doubled.
3. **Four small correctness defects.** Space is swallowed by leader mode on the trainer menu and
   result screens, making their `case " "` arms unreachable; leaving a lesson with `Esc` does not
   save progress while the code comment claims it does, and the internal `case "esc"` arms are
   dead; `ctrl+a`/`ctrl+e`/`ctrl+w` insert the literal six-character string instead of a control
   character; and the boss path decrements lives outside `RecordIncorrectAnswer`, never records a
   failed attempt, and unconditionally claims the next module is unlocked, including after the
   final boss.
4. **The timing data is dead.** Every exercise carries `TimeoutSecs` except 30 of the 35 boss steps,
   every boss step declares a `TimeLimit`, and every boss but Change & Repeat declares a `BonusTime`,
   and nothing reads any of it. The UI always passes a fixed `10.0` seconds (`update.go:1654`), so the
   sub-2-second speed bonus in `CalculatePoints` can never fire and `TotalTime` is never accumulated.
5. **Dead surface.** Seven `getXPractice()` generators, `GetPracticeExercises`,
   `GetWeightedPracticeExercises`, `IsValidInput`, `GetAlternativeSolutions`,
   `SetPracticeExercise`, a redundant `max`, `CursorTarget` and `DimmedCodeStyle` have no
   production callers; the skip-simulation predicate is copy-pasted in three places that can
   silently diverge; `Bosses: %d/7` is hardcoded twice; `rand.Seed` is deprecated.

## Accepted decisions
- Scope chosen by the user: all four groups, delivered in three slices.
- Slices are separate branches and separate PRs, because the repository requires pull requests and
  the whole change is well over the reviewer budget for one PR:
  1. `fix/vim-trainer-correctness` — T1-T6, the defects. Correctness first, and independently
     reviewable without the features.
  2. `feat/vim-trainer-timing` — T7-T9, the countdown and the boss timers.
  3. `feat/vim-trainer-progress` — T10-T12, progress visibility and dead-code removal.
- New exercise content (undo/redo, insert entries, yank/put, `%`, marks, indent, visual block) is
  explicitly out of scope: it needs simulator work and is a new feature, not an improvement.
- Trainer goldens (`TestTrainerMenuGolden`, `TestTrainerLessonGolden`,
  `TestTrainerResultCorrectGolden`, `TestTrainerBossGolden`) are regenerated wherever a rendered
  label legitimately changes. They are deterministic since `isolateGoldenTest` landed, so a
  regeneration performed here is valid on CI.
- Test-first applies: every defect fix starts from a failing test that reproduces it.
- Estimated size: slice 1 ~250 lines, slice 2 ~200, slice 3 ~250, including tests.

## Workflow configuration
- Route: ODD, one bounded writer per task, one independent verifier per slice.
- Exact runner: `cd installer && go test ./internal/tui/trainer/... ./internal/tui/ -run 'Trainer'`,
  plus `cd installer && go test ./...` before each slice closes.
- Delivery: branch, push, PR, wait for CI, then ask the user before merging.

## Slice 1 — correctness (`fix/vim-trainer-correctness`)

### T1 — Judge operator and text-object exercises by their selection
- [x] Reject an answer the simulator cannot fully parse, and accept an operator answer only when
      the resulting selection matches the optimal's selection. Keep plain motion exercises judged
      by final cursor position, and keep membership in `Solutions` as the fast path.
- [x] Reuse or replace the existing dead `IsValidInput` rather than inventing a second notion of
      "recognised input".
- [x] Tests: `textobjects_001` rejects `q`; accepts `viw`; the Change & Repeat `dd` lesson rejects
      `q`; `exercises_visual_test.go` stays green.
- Check: 406 trainer tests pass; `go test ./...` 1698 pass; no golden touched.
- Route: delegated writer, then independent read-only verification before the commit.
- Evidence: RED observed before the fix — `ValidateAnswer("q")` was `true` for `textobjects_001`,
  `changerepeat_015` and their boss steps. `IsValidInput` is deleted and `SimulationResult`
  carries `Recognized`, tracked through the parse loop; `ValidateAnswer` delegates to
  `ValidateAnswerDetailed` so the two cannot disagree; a nil-deref on a nil exercise was fixed.
  The verifier enumerated the corpus and confirmed no exercise became unpassable: all 156 lessons,
  35 boss steps and 156 practice exercises accept their own `Optimal`, and 566/566 predefined
  solutions still pass.
- Accepted and disclosed: answers that append a command the simulator does not model (`wq`, `wzz`,
  `zz`, `g_`) are now rejected where the position-only rule accepted them. That is the intended
  tightening; the simulator cannot verify those keys.
- Pre-existing, not introduced here: the simulator's `e`/`E` punctuation handling diverges from Vim
  and rejects `3e`/`eee` on `user.profile = 1`, and an out-of-range `CursorPos.Line` panics. Neither
  is reachable from the shipped corpus. The panic guard is owed in T12.
- Commit: `52bf0cf`.

### T2 — Count a practice attempt exactly once
- [x] One submission increments `PracticeAttempts` and `PracticeCorrect` exactly once, in one
      place, while per-exercise mastery still records.
- [x] Tests: one correct and one incorrect submission each move the counters by exactly one, and
      the mastery call still runs.
- Check: 406 trainer tests, 1701 module tests, `gofmt` and `go vet` clean.
- Route: delegated writer.
- Evidence: RED observed at the level the defect lives — the handler's two-call composition. One
  submission left `PracticeAttempts = 2` and `PracticeCorrect = 2`, and the mastery assertions
  passed even in RED, which proved the doubled module counters were the defect and mastery was not
  missing. `ModuleProgress.RecordPracticeResult` is now the single owner; `grep` confirms the only
  increments left are inside it. The generic recorder no longer does practice bookkeeping, and
  `LastPracticed` still moves because the new owner sets it (`practice.go:61`), checked by the
  parent. Commit `ede5003`.
- Found while fixing and carried into T6: `handleTrainerBossKeys` never calls the answer recorders
  at all, so boss steps bypass streak, score and attempt accounting.

### T3 — Let space reach the trainer menu and result screens
- [x] Space acts on the trainer menu, result and boss-result screens instead of arming leader mode,
      and still inserts nothing on the lesson and practice screens.
- [x] Tests: a teatest assertion that space on the trainer menu performs the selection.
- Check: `-run Trainer` and the whole module pass; no golden moved, so rendering is unchanged.
- Route: delegated writer.
- Evidence: RED observed on the three screens (`space activated leader mode on the trainer menu`),
  with a guard test that already passed before the fix, proving space stayed ordinary input on the
  lesson, practice and boss screens. One `isTrainerScreen` predicate now feeds the exemption. It was
  deliberately not applied to `handleEscape`, which partitions the trainer screens into three
  different behaviours; T4 owns that path. Commit `007bcf9`.

### T4 — Save progress when a lesson is left with Esc, and drop the dead esc arms
- [x] Leaving lesson, practice or boss with `Esc` persists stats, and the internal `case "esc"`
      arms that can never run are removed with the comment that claims otherwise.
- [x] Tests: stats persisted after a simulated `Esc`; the boss abandon path behaves as documented.
- Check: 1712 module tests pass; the four trainer goldens did not move, so no rendering changed.
- Route: delegated writer.
- Evidence: RED observed as a missing stats file after leaving a lesson, a practice session and a
  boss with `Esc`, plus an empty abandon message. The escape path now saves for those three screens
  and reports the abandoned boss; the unreachable `case "esc"` arms and their tokens are deleted.
  Commit `46e7224`.

### T5 — Stop inserting the literal text `ctrl+a`
- [x] Keys that the simulator does not model are not accepted as input, so no literal `ctrl+`
      string can reach validation.
- [x] Tests: sending `ctrl+a` leaves the input empty.
- Check: 1731 module tests pass; `gofmt` and `go vet` clean.
- Route: delegated writer.
- Evidence: RED observed as `TrainerInput = "ctrl+a"` in the lesson and practice handlers. The two
  duplicate accepted sets are now one shared map of the four combinations the simulator parses, and
  anything else is ignored. The boss handler's copy listed only the modelled four, so its defect was
  the duplication rather than the literal insertion; that duplication is gone. Commit `2b111e8`.
- Known limitation, disclosed: the shared map mirrors the simulator rather than being derived from
  it, so a new control key in the simulator needs the map updated. The comment says so; T12 is the
  task that centralizes simulator-facing predicates.

### T6 — Boss bookkeeping and the final-boss message
- [ ] Life loss goes through the canonical recorder, failed attempts are recorded, the
      death-named field stops claiming victory, and the unlock message appears only when a next
      module exists.
- Tests: a lost boss decrements lives once through the canonical path and records an attempt; the
  final boss shows no unlock claim.
- Route: delegated writer.

## Slice 2 — timing (`feat/vim-trainer-timing`)

### T7 — Real elapsed time per answer
- [x] Replace the fixed `10.0` with the measured elapsed time for the current exercise, accumulate
      `TotalTime`, and set `LastPlayed`, so the sub-2-second speed bonus becomes reachable.
- [x] Tests: a fast answer earns the bonus and a slow one does not; `TotalTime` accumulates.
- Check: 426 trainer tests, 1762 module tests; `gofmt` and `go vet` clean; no golden moved.
- Route: delegated writer.
- Evidence: genuine RED — a 1.5 second answer scored 15 where the measurement earns 18, and the boss
  path recorded a flat 10 seconds. One presentation stamp and one injected clock, with `time.Now`
  as the production default, live in the game state; a wrong answer does not restart the clock, so
  the bonus rewards solving the exercise rather than answering twice. Commit `9e7726d`.

### T8 — Countdown and automatic hint
- [x] Show the remaining time for the current exercise and reveal the hint automatically when
      `TimeoutSecs` elapses, without blocking typing.
- [x] Tests: the hint appears once the deadline passes; the countdown renders.
- Check: 429 trainer tests, 1768 module tests; `TestTrainerLessonGolden` regenerated.
- Route: delegated writer.
- Evidence: RED observed three times, including the golden mismatch. Decision taken here and
  declared: `CalculatePoints` no longer requires the exercise to declare a timeout before awarding
  the speed multiplier. That gate was right while only some exercises were timed and became wrong
  once every answer was measured, because it denied the bonus to boss steps, which declare none.
  Commit `e6b491a`.
- Accepted with disclosure: the countdown rides the installer's existing 100 ms animation tick
  instead of a dedicated timer. That tick re-arms unconditionally today and never reads the wall
  clock, so the countdown works, but trainer liveness is now coupled to it; a future conditional tick
  would need the countdown re-armed locally.

### T9 — Boss step time limits and bonus time
- [x] Enforce `BossStep.TimeLimit` on the injected clock: a step left unanswered costs exactly ONE
      life through the canonical recorder, shows the solution, keeps the user on the step, and
      restarts the window, so the repeating tick cannot drain the lives.
- [x] Apply `BonusTime` and document the rule on the field itself.
- [x] Tests: one life per window across repeated ticks, the step kept with a fresh window, the bonus
      reaching the next step's effective limit, the last life lost to the clock ending the fight, and
      an answer inside the limit losing nothing.
- Check: 433 trainer tests, 1779 module tests; `TestTrainerBossGolden` regenerated for the countdown.
- Route: delegated writer.
- Evidence: six RED failures, the decisive one being `BossLives = 3 after 20 ticks past one
  deadline`, which is exactly the failure mode a repeating tick invites. Commit `34bebe6`.
- Decision taken on the user's behalf and cheap to overrule (one assignment plus its test): a won
  step grants `BonusTime` to the step that FOLLOWS it, per win and never accumulated, and a lost
  step grants nothing.
- Correction to this tracker's own planning claim: the shipped per-step limits are 5 to 60 seconds,
  not the 30 to 60 written when the task was planned, and 30 of the 35 boss steps declare no
  `TimeoutSecs` while the five Change & Repeat steps do (45/60/45/30/30). The Change & Repeat boss is
  also the only one declaring no `BonusTime`, so its wins buy nothing, while the others declare 30 to
  120 seconds — larger than a whole step limit, which is why the non-accumulating reading was chosen.
  A fight-wide pool remains a defensible alternative and the user can ask for it.
- Defect found by the slice verification and fixed in a corrective commit: a wrong boss answer did
  not re-arm the step window, so one window could cost two lives — the wrong answer, then the expiry
  of the same window. Any lost life now re-arms the window, in one place.

## Slice 3 — progress and cleanup (`feat/vim-trainer-progress`)

### T10 — Show real progress on the trainer menu
- [x] Show mastered-per-total per module and surface the exercises answered wrong most often.
- [x] Tests: the menu renders the counts from stats; the golden is regenerated.
- Check: 439 trainer tests, 1802 module tests; `TestTrainerMenuGolden` regenerated with a seeded
  profile so the snapshot proves the numbers render instead of matching an empty one.
- Route: delegated writer.
- Evidence: RED observed for the golden, the per-module counts and the weakest-list order. Mastery is
  now read through one predicate shared with the weighted practice selection, so the number on
  screen cannot drift from the rule that drives practice. Commit `c8c5b95`.
- Fixed on the way, and worth remembering: looking a module's stats up used to CREATE empty exercise
  records, so merely opening the menu wrote to the player's profile. The lookup is read-only now.
- Declared UX trade: the module descriptions moved to the selected entry's detail line so the screen
  still fits 80x24, and the weakest list is per module because the recorded stats are per module.

### T11 — Reset progress from the UI
- [x] A reachable reset that clears all trainer progress behind a confirmation, replacing the
      current unreachable `ResetStats`.
- [x] Tests: confirming clears, cancelling does not; the golden is regenerated.
- Check: 1802 module tests; the menu golden regenerated because the help text changed.
- Route: delegated writer.
- Evidence: RED observed for the armed state, the confirmed clear (memory and disk), the missing
  profile and the help line. `shift+R` arms and a second `shift+R` clears; any other key cancels and
  escape leaves everything alone; the lowercase `r` still resets one module. Commit `a1ba07f`.
- Fixed on the way: the old help line was 89 columns and the layout sized the whole frame to it, so
  the menu had been rendering wider than the 80 column terminal it documents.
- Accepted with disclosure: the armed state rides on `TrainerMessage` holding a known prompt rather
  than a dedicated model field, because `model.go` was outside that task's surfaces. Every writer of
  that message was checked and a stray write can only cancel the prompt, never complete it, but
  promoting it to a boolean field on the model is the clean follow-up.

### T12 — Remove the dead surface and centralize the duplicated predicate
- [x] Delete the dead symbols listed in the rationale, including the seven `getXPractice()`
      generators and their tests, and confirm practice still selects from a real corpus.
- [x] One exported `ShouldSkipSimulation(exercise)` replaces the three copied skip-simulation
      predicates, and the boss count derives from `len(GetAllModules())`.
- [x] Tests: the suite passes after the deletions; a test pins the shared predicate against the three
      former call sites' behavior.
- Check: 443 trainer tests, 1806 module tests; no golden moved, so nothing a player sees changed.
- Route: delegated writer.
- Evidence: the corpus is unchanged at 156 lessons because practice selection has always drawn from
  the lessons; net `-358` lines. The simulator's blind code-line indexing is fixed with a real RED,
  `panic: index out of range [2] with length 2` from `dd` with a cursor line past the end, and the
  start line is now clamped once at entry. `CursorTarget` was deleted rather than kept with a test
  because nothing read it. Commit `f53118d`.
- Kept deliberately: `SetPracticeExercise` is used only by tests, but it is the seam those tests use
  to put the game in a state, so deleting it would push them into poking fields directly. A negative
  cursor COLUMN is still unguarded; that is a separate malformed-input class and was left alone.

## Acceptance criteria
1. [x] Typing an unrecognised key sequence cannot pass an operator or text-object exercise.
2. [x] One practice submission moves the attempt counters by exactly one.
3. [x] Space, `Esc` and the modelled control keys behave as the UI claims on every trainer screen.
4. [x] The most recently edited answer contributes its real elapsed time to the score.
5. [x] Every exercise's `TimeoutSecs` and every boss step's `TimeLimit` are enforced or removed.
6. [x] Progress can be inspected and reset from the UI.
7. [x] No production-dead symbol removed or kept remains unexplained.
8. [x] `go test ./...` green before each slice is pushed, with every skipped or unverified check
   recorded honestly.
9. [x] One PR per slice, CI green, and no merge without the user's decision. The user authorised the
   merges after the first PR, and each merge used `--admin` because `main` requires a review that a
   single-account repository cannot self-provide.

## Progress
- 2026-09-28: Feature opened after a read-only exploration of the trainer reported defects D1-D12
  and twelve ranked improvement candidates. The user selected the full scope. Parent confirmed the
  two severe defects by reading `validation.go:19-118`, `gamestate.go:145-195` and
  `update.go:1640-1700` directly. Slice 1 branch created from `main` at `c690cd2`.
- 2026-09-28: **Slice 1 complete and merged.** Seven commits, all of T1-T6 plus one regression the
  slice verification caught in T6's own commit. Delivered as issue #33 and PR #34 with the user's
  decision to keep it one PR; CI green (14 checks) after one re-run of `Linux E2E (ubuntu)`, which
  failed on an unrelated, pre-existing fragility: the Pi skills step downloads from
  `api.github.com` unauthenticated and took a 403 rate-limit answer. Merged as `9c7a550`.
- 2026-09-28: Slice 2 opened on `feat/vim-trainer-timing` from the merged `main`.

## Delivery notes
- Slice 1: 1057 insertions / 209 deletions, 12 files; ~211 lines of production change and ~846 of
  tests, spread over seven commits that merge cleanly into `main`.
- The flaky step worth fixing separately: `agent_skills`/`officecli` fetch pinned files through
  `api.github.com/repos/.../contents/...`, which is rate-limited for anonymous callers and is shared
  per runner IP. `raw.githubusercontent.com` serves the same pinned content without that limit.
