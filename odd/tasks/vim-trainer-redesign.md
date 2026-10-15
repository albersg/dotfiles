# Feature: the Vim trainer, honest about progress, calibrated about difficulty, and the pet turns

Status: in progress
Opened: 2026-10-16
Owner: autonomous session, worktree `../dotfiles-vim-trainer`, branch `feat/vim-trainer-redesign`
Orchestrator: this session; the user decides merges.

## Why

Requested by the user, verbatim:

> "Quiero que en dotfiles, rediseñes la parte del vim trainer, añadiendo mejoras, revisando toda la
> parte del avance, de la dificultad, de las pistas, de los desafios, etc..., que lo dejes todo
> mucho mejor que esta, buscando lugares donde posibles fallos hay o carencias y mejroarlo todo."

> "Una mejora friki (pero tu añade muchas otras mas), que la mascota, cuaqndo luches contra un boss
> de vim, se convierta en un boss (algo, se convierta en algo malo, siga siendo un perro/gato, pero
> con los ojos rojos, como un demonio, o casas asi, dale una vuetla)"

> "hay otro llm toqueteando. ten cuidado."

The second LLM is working in the primary worktree on the installer flow
(`odd/tasks/start-installation-flow.md`, `installer/internal/tui/install*.go`, `model.go`, `view.go`,
`update.go`, `panels.go`). This front therefore works in its own worktree and keeps its production
edits inside `installer/internal/tui/trainer/**`, with surgical, additive hooks in the shared TUI
files only where a screen has to show the new facts.

## Findings (evidence read, not inferred)

Read: `trainer/types.go`, `gamestate.go`, `practice.go`, `stats.go`, `validation.go`,
`tui/update.go` (trainer handlers), `tui/view.go` (trainer screens), `tui/panels.go` (trainer
panel), `tui/companion.go` (state machine, art tables, volume renderer). Corpus inventoried by a
temporary test over `GetLessons`/`GetBoss` for all 9 modules: 244 exercises (209 lessons, 35 boss
steps).

1. **Progression deadlock.** `IsPracticeComplete` is true once every lesson is mastered (3
   consecutive correct answers), and the menu then refuses practice with "🎉 Practice complete! All
   exercises mastered! Press [r] to reset." The boss gate is `PracticeAccuracy >= 0.80` over
   `PracticeAttempts >= 10` and the accuracy is LIFETIME. A player who mastered everything with a
   bad early run (entirely reachable: 3 right / 20 wrong per exercise still masters it) can neither
   practice nor fight, and the only exit is `[r]`, which erases practice progress.
2. **Browsing writes to the profile.** `GetExerciseStats` creates an entry, and read paths call it:
   `IsPracticeComplete` (the menu's `[p]` check) and `SelectRandomPracticeExercise` (every lesson,
   on entering practice). This is the same defect class the menu lookup already had fixed once.
3. **Dead boss time.** `GameState.TimeElapsed` is never advanced, so `BossBestTime` is always 0 and
   `RecordBossVictory` compares against a constant. The "best time" the profile stores is dead data.
4. **Two clocks, one of them wrong.** `RecordPracticeResult` stamps `LastAttempted`/`LastPracticed`
   with `time.Now()` while every other timing decision goes through the injected clock
   (`GameState.SetClock`), so practice timing is untestable and ignores the game clock.
5. **Global randomness.** `SelectRandomPracticeExercise` calls `rand.Intn`, so practice selection
   cannot be pinned by a test and has no seed.
6. **Per-exercise results are practice-only.** `RecordPracticeResult` is called only from the
   practice branch of the exercise handler, so lessons and boss steps never move
   `ExerciseStats` — mastery, weakness and the weighted pool ignore two thirds of the play.
7. **Mastery is forever, and the spec claims spaced repetition.** A mastered exercise leaves the
   pool permanently; `LastAttempted` is recorded and never read. `docs/vim-trainer-spec.md` sells
   "spaced repetition" that does not exist.
8. **Difficulty is decoration.** `Exercise.Level` is written on all 244 exercises and read nowhere;
   `Points` are authored ad hoc (10 to 50) with no rule tying them to difficulty; one exercise
   (`substitution_022`) declares `Level: 11` outside the documented 1-10 range and nothing notices.
9. **The boss clock is arbitrary.** Step limits are authored 5, 5, 5, 5, 5, 6, 6, 6, 8, 10, 25, 30,
   45, 60 seconds for steps of the same shape: the horizontal boss gives 5 seconds to read a mission
   and type `fe`, while the regex boss gives 45 for a comparable answer. `BonusTime` is 0 for the
   Change & Repeat fight (its wins buy nothing) and 120 for regex (larger than a whole step).
10. **The boss hint key is dead.** The boss handler honours `Tab`, but all 35 boss steps carry no
    `Hint`, so the key does nothing on every shipped fight. `TimeoutSecs` is also unset on all 35
    steps and on 5 lessons, so the automatic hint reveal cannot fire there either.
11. **The combo is invisible.** `ComboMultiplier` is raised only by `NextExercise` in boss mode and
    multiplies every score, so the same correct answer is worth 1x in a lesson and up to 4x in a
    fight, with nothing on screen saying so.
12. **The profile can be lost silently.** `SaveStats` writes with `os.WriteFile` (truncate then
    write: a crash mid-write leaves a truncated file) and `LoadStats` returns nil on any parse error,
    which the UI reads as "new player" and the next save overwrites the damaged file. The JSON has no
    schema version, so no future format change can migrate.
13. **The pet does not react to the trainer at all.** `companionStateNow` knows about installing,
    the theme picker and a newer release, but nothing about a boss fight, and there is no possessed
    or monstrous form among the 13 states.

## Specs

**S1. Looking at progress never changes it.** Verbatim: *"buscando lugares donde posibles fallos
hay o carencias"*. Every read path — the menu, the practice pool, the completion checks, the panel —
reads the recorded per-exercise stats without creating an entry, so opening a screen or starting
practice cannot write to the player's profile. A guard test asserts that a read of a module with no
recorded stats leaves the module's `ExerciseStats` untouched.

**S2. Practice never deadlocks.** A module whose exercises are all mastered still offers practice
while its boss gate is unmet: the session selects from the mastered exercises instead of refusing,
and the menu says why. The word "complete" is reserved for the state in which nothing is left to do.

**S3. One clock, one source of randomness, both injectable.** Practice recording stamps its times
through the same clock the game state already injects, and practice selection draws from an
injectable source, so a test can pin both without sleeping or flaking.

**S4. Progress is recorded in every mode.** A lesson answer and a boss answer move the same
per-exercise counters a practice answer does, so mastery, weakness and the weighted pool describe
all the play, not one third of it.

**S5. Mastered is a state, not a grave.** A mastered exercise returns to the practice pool once a
review interval has passed since its last attempt, and mastery is re-earned: the interval is a named
constant and a test pins both the due and the not-yet-due case.

**S6. The boss gate looks at the recent run.** The accuracy the gate reads is measured over the last
N practice answers (a named window), not over the whole history, so one bad first run cannot tax a
player forever. The requirement line on the menu and the panel says which accuracy it means.

**S7. Difficulty is a contract, not a comment.** `Level` is 1..10 on every shipped exercise, is
enforced by a corpus guard test, drives the points a correct answer is worth, and steers the
practice mix towards the harder exercises once the easier ones are mastered. The one out-of-range
exercise is corrected.

**S8. The boss clock is calibrated.** Every boss step's `TimeLimit` is at least the time its own
shape needs — the answer's length plus the mission's reading time — with one documented function as
the rule and a corpus guard test that every shipped step satisfies it. `BonusTime` is inside a
documented range on every fight, including the one that declared none.

**S9. The combo is real everywhere and visible.** Consecutive correct answers raise the combo in
lessons, practice and boss fights alike, a wrong answer or an expired window resets it, and the
screens show the multiplier the score is already multiplied by.

**S10. A fight's time is real.** A boss fight measures its own duration and records the best one, and
the result screen shows both the fight's time and the module's best.

**S11. A boss step has a hint, and it does not spoil the answer.** `Tab` on a boss step reveals the
step's authored hint when it has one and the answer's first keystrokes when it does not, so the key
stops being dead on all 35 shipped steps without printing the whole solution.

**S12. The pet turns.** Verbatim: *"la mascota, cuaqndo luches contra un boss de vim, se convierta
en un boss ... siga siendo un perro/gato, pero con los ojos rojos, como un demonio"*. While a boss
fight is on screen the creature takes a possessed form — horns, fangs, red eyes — in both the glyph
art and the volumetric sprite, it escalates as the fight turns against the player (the last life is
the deepest form), and it is its ordinary self again the moment the fight ends. The form is a state
in the companion's own machine, so every rung of the sprite ladder has an art frame for it and the
existing frame guard still refuses an overflowing one.

**S13. The profile is written atomically and never lost in silence.** A save goes through a temporary
file and a rename, so a crash cannot truncate the profile; the JSON carries a schema version so a
future format can migrate; and a profile that cannot be parsed is preserved beside the file instead
of being overwritten by the next save. A test pins each of the three.

## Tasks

- [x] **T1. Progression core** (S1-S4): read-only lookups, the review fallback that removes the
      deadlock, clock and RNG seams, per-exercise recording in all three modes. Tests first.
- [x] **T2. The pet turns** (S12): possessed companion state, art for the three glyph rungs and the
      volume sprite, escalation with the boss's remaining lives, restored after the fight.
- [x] **T3. Spaced repetition and the recent gate** (S5, S6): review interval, due count on the menu,
      windowed accuracy for the boss gate, requirement copy.
- [x] **T4. Calibrated difficulty** (S7, S8): the Level contract and its guard, points from
      difficulty, the practice mix, the boss clock rule and the corrected step limits.
- [x] **T5. The visible combo** (S9): combo in all modes, shown on the exercise and boss headers.
- [x] **T6. Fight time and boss hints** (S10, S11): measured fight duration, best time on the result
      screen, the boss nudge behind `Tab`.
- [ ] **T7. Profile hardening** (S13): atomic save, schema version, corrupt profile preserved.
- [ ] **T8. Docs** (S2, S5, S6, S7, S8, S12): the trainer spec and the trainer skill describe the
      rules this front changed, so the repository does not document behaviour it no longer has.

## Workflow configuration

- Route: ODD. The parent implements inline, one work unit per task, test-first (RED observed before
  the fix), with one independent read-only verification at the end of the front.
- Exact runner: `cd installer && go test ./internal/tui/trainer/... -count=1`, then
  `cd installer && go test ./... -count=1`, plus `gofmt -l` and `go vet ./...`.
- Delivery: commits on `feat/vim-trainer-redesign` in this worktree, one work unit each. Push, PR and
  merge stay the user's decision.

## Log

- **L1 (user, verbatim).** "hay otro llm toqueteando. ten cuidado. Quiero que en dotfiles, rediseñes
  la parte del vim trainer, añadiendo mejoras, revisando toda la parte del avance, de la dificultad,
  de las pistas, de los desafios, etc..., que lo dejes todo mucho mejor que esta, buscando lugares
  donde posibles fallos hay o carencias y mejroarlo todo. Una mejora friki (pero tu añade muchas
  otras mas), que la mascota, cuaqndo luches contra un boss de vim, se convierta en un boss (algo,
  se convierta en algo malo, siga siendo un perro/gato, pero con los ojos rojos, como un demonio, o
  casas asi, dale una vuetla)"
- **L2 (user decision).** Working root is `/home/albeertoo/my-things/dotfiles` (outside this
  session's project root, which is a backup directory with no sources) and the work is isolated in
  the git worktree `../dotfiles-vim-trainer` on a branch of its own, because another LLM is editing
  the installer flow in the primary worktree.
- **L3 (parent, baseline).** `go test ./internal/tui/trainer/...` green at `4ba4332` before any
  edit. Corpus inventory run with a temporary test that was deleted before the first commit.
- **T1 (S1-S4).** RED observed in two steps: the new read-side accessor
  (`ExerciseStatsOrNil`) failed the build, then, with only that accessor added, seven genuine
  failures: `selecting a practice exercise created 19 profile entries`, `the completion check
  created 1 profile entries`, `a lesson answer recorded nothing for its own exercise`, `a boss
  answer recorded nothing`, `one practice answer moved PracticeAttempts to 0`, and the wrong-answer
  counterpart. Implemented: `ExerciseStatsOrNil` plus a nil-safe `GetPracticeWeight`; the read paths
  (`SelectRandomPracticeExercise`, `IsPracticeComplete`, `UserStats.IsPracticeComplete`) read without
  creating; `MasteredReviewWeight = 1` keeps mastered exercises in the pool so it can never empty;
  `UserStats.PracticeComplete` (all mastered AND boss unlocked) is the rule that ends a session, and
  the menu now starts a review run instead of refusing; `RecordExerciseResult` /
  `RecordPracticeAttempt` /
  `RecordPracticeResult(id, correct, now)` split the per-exercise record from the practice-only
  counters and take the caller's clock; `GameState.SetRandomSource` + `GameState.Now()` are the
  session seams, and `GameState.recordAnswer` is the single owner of every answer counter, so the
  screen no longer records anything.
  Three pins were rewritten because they encoded the defects: `TestGetPracticeWeight_MasteredIsZero`,
  `TestSelectRandomPracticeExercise_NilWhenAllMastered` and
  `TestGameState_NextPracticeExercise_ReturnsFalseWhenAllMastered` (now the two-sided rule), plus
  `TestGameState_UpdatePracticeStats` and `TestGameState_PracticeAccuracyCalculation`, which pinned
  the old two-call composition the UI used to run, and the practice subtest of
  `TestTrainerSessionCompleteReportsFailedSave`, which now has to satisfy the boss gate for the
  session to be complete at all.
  Pre-existing and NOT introduced here: four Ghostty installer tests fail at `4ba4332` as well
  (`TestStepInstallTerminalGhosttyReportsAFailedDownload`,
  `TestStepInstallTerminalGhosttyRunsTheDownloadItFetches`,
  `TestInteractiveGhosttyScriptFailsWhenTheDownloadFails`,
  `TestInteractiveGhosttyScriptDownloadsThenRuns`), confirmed by running them in a detached worktree
  at `4ba4332`.

- **T3 (S5, S6).** RED observed as a build failure naming every new rule
  (`MasteryReviewInterval`, `IsReviewDue`, `GetPracticeWeightAt`), then implemented:
  `MasteryReviewInterval = 12h` and `ExerciseStats.IsReviewDue(now)` read the per-exercise
  `LastAttempted` stamp the trainer had recorded since it was written and never used;
  `GetPracticeWeightAt(now)` weighs a due review at `MasteredReviewDueWeight = 12` and a resting
  one at `MasteredReviewWeight = 1`, so mastery is a resting state rather than a grave;
  `ModuleProgress.RecentResults` is the last `BossGateWindow = 20` practice answers and
  `RecentAccuracy` is what `IsBossReady` reads (a profile written before the window existed falls
  back to the lifetime accuracy, so old profiles keep the gate they had);
  `UserStats.PracticeComplete(module, now)` now also waits for due reviews, because a promised
  review that has come due is work, not a finished module. Visible: the menu detail line carries
  "N mastered reviews due", the `[p]` handler announces a review run, and the boss requirement line
  reports the recent window instead of the lifetime average.
  Fixtures corrected, not rules: `recordPractice` answers at the real clock (a session's own
  answers are never due), the mastered-weight pin and the session-complete pin stamp
  `LastAttempted` "now", and the panel pin now asserts the new requirement copy.

- **T4 (S7, S8).** RED observed as a build failure on the new names, then, once the rules
  existed, as three corpus guards failing on real content: `substitution_022` declared level 11;
  30 of the 35 boss steps gave the player less time than their own mission and answer need (the
  worst was `editing_boss_2`, 33s of work in a 10s window); `changerepeat_boss` granted no bonus
  time at all and `regex_boss` granted two minutes. Implemented: `difficulty.go` holds the Level
  contract (`Min/MaxExerciseLevel`, `ClampExerciseLevel`, `PointsForLevel` 10..55,
  `ExercisePoints` with the level as a floor so a hand-tuned reward survives and a hard exercise
  can never pay less than an easy one), the frontier rule (`MasteredLevelFloor`, `frontierBonus`,
  `PracticeFrontierWeight = 10`, `PracticeBeyondFrontierWeight = 4`) and `CalculatePoints` now
  pays `ExercisePoints`; `bossclock.go` holds the clock rule (`BossStepBaseSeconds = 4` + mission
  read at 20 chars/s + 1s per answer character, capped at 90s) and the bonus-time range (15..60s).
  Content corrected to the rules, not the rules to the content: the level-11 exercise is now 10,
  30 step limits were raised to the recommendation, `changerepeat_boss` gained `BonusTime: 30` and
  `regex_boss` came down to 45. The boss golden changed by exactly the one line that reads the
  clock (5s -> 7s on `horizontal_boss_1`), regenerated with `-update` and inspected.
  Deferred to T6 by design: the guard that every boss step carries a hint (35 steps do not), which
  is S11's content work rather than S7/S8's.

- **T5 (S9).** RED observed as a build failure on `MaxComboMultiplier`. Implemented: the combo
  is raised by `RecordCorrectAnswer` -- the single owner of a correct answer -- instead of by the
  boss branch of `NextExercise`, so lessons and practice are multiplied too, and it is capped by
  the named `MaxComboMultiplier = 4`; `RecordIncorrectAnswer` already reset it, which is why an
  expired boss step resets it as well. Visible: `trainerComboText` is the one owner of the wording
  and of when the fact is worth showing (nothing at multiplier 1), and both the exercise header
  and the boss header carry it. The boss chip sits before the countdown because the header drops
  what does not fit from its tail, and the seconds left on a step are worth more than a run of
  wins. Two pins that encoded the old ownership were rewritten
  (`TestGameState_NextExercise_IncreasesCombo`, `TestGameState_NextExercise_ComboMaxAt4`), and the
  new `TestTrainerComboSurvivesTheDocumentedFloor` holds the chip inside 80 columns.

- **T6 (S10, S11).** RED observed as a build failure on `BossFightElapsed`, then as the corpus
  hint guard failing on all 35 steps. Implemented for S10: the fight has its own clock
  (`bossFightStartedAt`/`bossFightEndedAt`, frozen by `finishBossFight` when the last life goes or
  the boss falls), `BossFightElapsed` is what `RecordBossVictory` now writes into the dead
  `TimeElapsed` field, `BossBestTime` is finally recorded, and the result screen reports "Fight
  time: Ns · Best: Ns". For S11: `trainer.BossStepNudge` is the one owner of what `Tab` reveals --
  the step's own hint, or the first keystrokes of its answer, never the whole answer and nothing at
  all for a one-keystroke answer -- and all 35 steps now ship an authored hint.
  Content lesson worth keeping: the repo's own `TestShippedHintsAddWhatTheirMissionDoesNot` guard
  rejected 17 of the first drafts for repeating the mission, so those were rewritten to add a
  mechanism (a count, a sibling command, or the consequence of the command) rather than a synonym.
  The new `TestCorpusHintsDoNotQuoteTheAnswer` holds the other end. One pin that encoded the old
  contract was rewritten: `TestGameState_RecordBossVictory` assigned `TimeElapsed` by hand and now
  moves the fight's clock instead.
