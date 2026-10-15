# Installer customization: the user chooses what gets installed

The installer schedules eleven of its seventeen units without asking anyone. This
front gives every unit a switch the user owns, keeps today's plan byte-identical
when nobody touches the selection, and makes a unit that was turned off report
itself as skipped rather than quietly absent.

Feature front for the installer's unit selection. Specs first, then tasks; the log
is last and holds the user's own words.

## Specs

- **S1.** The user asked, verbatim: "Metele en el installer, mas customizacion de las cosas que quiere instalar el usuario o no".
- **S2.** Chosen shape (the user's words: "Las dos cosas"): the wizard's own questions stay exactly as they are, and a screen after the last question reviews and adjusts every unit before the run starts. No question is added and none is removed.
- **S3.** Chosen scope (the user's words: "Todo menos lo imprescindible"): every unit can be turned off except the three the run itself needs -- `clone` (the repository the run works from), `cleanup` (the state it created) and `setshell` (leaving the machine's default shell as chosen). Every other unit of the executor table is a switch: `backup, homebrew, deps, xcode, terminal, font, shell, wm, nvim, toolset, agentskills, officecli, wslconfig, theme`.
- **S4.** With the selection untouched, the plan is today's plan: the same units in the same order, built from the same `UserChoices`, and a guard compares it against what the code plans today.
- **S5.** The screen shows every unit with its state, marks the three of S3 as always installed, and cannot be made to turn one of them off. It is reachable in both directions: back to the wizard's questions and forward to the run.
- **S6.** A unit turned off is not executed, is not reported as a failure, and appears as skipped in the run's own reporting -- which is what `StatusSkipped` exists for and has never once been assigned to.
- **S7.** The two existing environment variables keep their effect exactly: `DOTFILES_SKIP_DEPS` and `DOTFILES_SKIP_TOOLSET` still skip those units, and their behaviour is unchanged when the selection is untouched.
- **S8.** `--non-interactive` gets a flag that expresses the same selection, and the plan it builds is the plan the same selection produces in the interface, because both paths read one shared builder instead of the two that can already drift apart.
- **S9.** The new screen satisfies the screen family's guards: a title, a description, options for every unit, a panel, a render case, a key-dispatch case, an escape-map entry, and the frame guard at every measured terminal size.

## Tasks

- [x] **T1** (S1, S3) Shared unit table: one place names the seventeen units, their order, and which three are fixed. Route: inline. Commit: `refactor(tui): one table names the installer's units`.
- [x] **T2** (S4, S8) One builder: the interactive and non-interactive planners become callers of a single function, with the defaults unchanged. Route: inline. Commit: `refactor(tui): one builder turns choices into steps`.
- [x] **T3** (S5, S9) The screen: enum, title, description, options, panel, render, key dispatch, escape map. Route: worker. Commit: `feat(tui): a screen that shows every unit and its switch`.
- [x] **T4** (S6) The selection reaches the plan, and a unit turned off reports itself as skipped. Route: worker. Commit: `feat(tui): a unit turned off says so instead of vanishing`.
- **T5** (S7, S8) The command line and the environment: the flag, and parity guards that hold both routes to the same plan. Route: worker. Commit: `feat(tui): the command line says what the screen can say`.
- **T6** (S2, S5) Documentation and changelog: `docs/tui-installer.md`, the `--help` text and `CHANGELOG.md`. Route: inline. Commit: `docs(tui): the installer's choices are described where its screens are`.

## Log

- **L1.** The user's request, verbatim: "Metele en el installer, mas customizacion de las cosas que quiere instalar el usuario o no".
- **L2.** The user's two design answers, verbatim: "Las dos cosas" (how the choice is made) and "Todo menos lo imprescindible" (what may be switched off).
- **L3.** Reconnaissance, with `path:line` evidence: the executor's seventeen units `installer/internal/tui/installer.go:4504-4520`; the interactive plan `model.go:1216-1406`; its duplicated non-interactive twin `non_interactive.go:97-171`, with the duplication acknowledged at `executor_dispatch_test.go:9-10`; the wizard's six questions `model.go:704-783`; the only two skip switches, `DOTFILES_SKIP_DEPS` `installer.go:4780-4790` and `DOTFILES_SKIP_TOOLSET` `installer.go:4798-4806`; `StatusSkipped` declared at `model.go:127` and rendered at `view.go:2932,2993,3167` with nothing ever assigning it; the screen enum and its append rule `model.go:18-109` and `model.go:57-65`.
- **L4.** Reconnaissance finding, out of scope for this front and reported to the user: `profiles/*.yaml` and `docs/PROFILES.md` document a profile system with a merge order (`base->platform->selected->machine->CLI`) and nothing reads them -- no Go code, no script. The `base` profile the document names is not in `profiles/`.
- **L5.** The guards this front has to keep green: `TestEveryScheduledStepHasAnExecutor` and `TestEveryExecutorEntryIsAScheduledStep` (`executor_dispatch_test.go:50,63`), `TestEveryInteractiveStepIsDispatchable` and `TestEveryInteractiveDispatchCaseIsAFlaggedStep` (`interactive_dispatch_test.go:109,140`), `TestEveryScreenIsCoveredByAFrameGuard` (`screen_coverage_test.go:32`), `TestGetCurrentOptionsForAllScreens` (`comprehensive_test.go:75`), `TestGetScreenTitleForAllScreens` (`comprehensive_test.go:211`), `TestGetScreenDescription` (`comprehensive_test.go:237`), `TestDryRunSkipsEveryStep` (`install_paths_test.go:943`), `TestRunInstallStepsContinuesPastAFailure` (`non_interactive_test.go:12`).
- **L6.** Measured while T2 was designed, before a line of it was written: the two planners do not
  agree today, and what separates them is behaviour rather than wording. `planFor` (`model.go:1216`)
  asks the distribution for the dependency step -- `deps` on Linux and Termux, `xcode` on macOS and
  only when the tools are missing, the two mutually exclusive -- skips `homebrew` when
  `opts.HasBrew`, and needs `ConfigCount > 0` before it schedules `backup`. `buildStepsForChoices`
  (`non_interactive.go:98`) always schedules `deps`, schedules `xcode` on every macOS host, schedules
  `homebrew` on macOS/Debian/Linux/WSL without ever reading `HasBrew`, and schedules `backup` from
  `Choices.CreateBackup` alone. The step names differ too, so the same unit is announced with
  different words on each path. The behaviour the documentation describes (`docs/tui-installer.md`)
  is `planFor`'s: dependencies are a Linux/Termux step and macOS gets the Apple developer tools.
  S8's direction therefore follows from S4 rather than from taste -- the interface's plan is the
  reference, so the non-interactive run adopts it -- and the price is visible and bounded: on macOS
  that run stops installing `baseDependencies().Brew`, `curl file git wget unzip fontconfig`
  (`installer.go:4813`), of which the `Brewfile` carries only `curl` and `git` (measured, this day).
  The interface installs none of those four on macOS today, so this converges a non-interactive run
  onto what every interactive user already gets; whether they belong in the `Brewfile` instead is a
  separate decision and is not part of this front. The churn this costs in the repository's own
  guards is small and was measured too: two lines pin the old names, `non_interactive_test.go:16`
  (`Set shell as default`) and `step_loop_test.go:118` (`Clone dotfiles repository`).

- **L7.** T1 shipped as `58e08f2` ("refactor(tui): one table names the installer's units"); T2 is
  the commit that carries this line. T1's evidence is a mutation set rather than a red test, and it
  has to be: a table that nothing reads yet has no behaviour to make red. Four mutations were applied
  and each failed the guard that names its rule -- cleanup unmarked, agentskills dropped, a unit
  added to catalogue and executor that no configuration can schedule (caught by the pre-existing
  `TestEveryExecutorEntryIsAScheduledStep`, which is the transitive coverage the new guards lean on),
  and backup named twice -- with both files restored byte-identical: sha256 over the pair
  `c7fa0552728d45a8`, before and after.
- **L8.** T2 did not rewrite `planFor`, which the task table allowed it to. The catalogue is made the
  authority on the order by a guard instead: every plan has to be a subsequence of it, and the union
  of every configuration has to plan every unit it names. That is the same force the executor
  agreement gets from enumerating both schedulers, and it is a diff of fourteen lines rather than
  seventy-four -- the risky version of this task was the elegant one.
- **L9.** "Today's plan" is a measurement, not a memory. A probe printed the plan, field by field,
  for six configurations before the builder changed, and the guard is written against that output, so
  a unit that appears, disappears or moves fails rather than being noticed in the field. The whole
  module then ran twice, before and after, with the same four pre-existing Ghostty failures and no
  others.
- **L10.** That green is also the finding, and it is not a comfortable one: nothing in the suite
  pinned the divergence the two planners had been carrying. On macOS the non-interactive path
  scheduled `deps` -- a Linux step -- and `xcode` regardless of the tools being present; on a Termux
  host it scheduled `homebrew`; with nothing to back up it scheduled `backup`. Fifteen tests name
  `buildStepsForChoices`, and not one of them said which units it should produce. The convergence is
  therefore covered by the four guards written here and by nothing that already existed, which is
  worth knowing before trusting the next green.
- **L11.** T3's red came before its code and named the rule: with the screen constant declared and no
  render case, `TestEveryScreenIsCoveredByAFrameGuard` failed with "no frame guard measures 1 of 46
  screens: [ScreenInstallReview]". Registering the screen in the lists that measure it -- the
  frame-state list, the frame case, the options and title lists, the panel map, the `Ctrl+C` list and
  the two pinned enumerations -- is part of the task rather than cleanup after it: a screen the guards
  do not measure is a screen nothing holds.
- **L12.** The frame guard then measured the screen's real size and it was wrong: 28 rows on a 24-row
  terminal. The cascade is worth recording because the first reading of it was wrong too. The footer's
  Enter and Esc hints were reported as "not shown", which reads as a footer that drops hints; it was
  the body pushing the footer off the bottom, and the footer at `footerMaxRows = 2` only truncates
  *its own* rows. One cause, two symptoms, and the guard that found it named the screen.
- **L13.** A fixed window was the next wrong answer. Nine rows fit 80x24 and still overflowed the
  twenty-row terminal the matrix measures -- there are twelve sizes, not four. The window is now
  computed rather than chosen: `installerBodyRows` says what the body has, the title, the description,
  the count and the two "more above"/"more below" rows are subtracted, and the floor is three rows.
  Measured with a temporary probe before the numbers were used: 23 rows at 80x24 (window 11), 19 at
  60x20 (window 5, footer on two rows), 29 at 100x30 with all seventeen units and no "more" line. The
  probe was deleted and checked absent from the index, the `zz_probe_test.go` hygiene step this front
  already learned once.
- **L14.** One guard failed against a screen that was correct, and the guard was the defect: it built
  `Model{Screen: ScreenInstallReview}` with no terminal size, so the footer wrapped past
  `footerMaxRows` and lost its last hints. A guard that renders must name the terminal it renders at,
  for the same reason the password screen's guard folds the frame before comparing: otherwise the
  harness decides the verdict.
- **L15.** Three tests pinned the old wiring -- the neovim question and the backup question's two
  answers all started the run outright. They now assert that the review opens, that the question it
  came from is remembered, and that nothing has started yet; and the golden-capture guard's own teeth
  walk both presses to reach the executor, so the teeth stay on the real seam instead of being
  loosened. Found while fixing them, not fixed here: `comprehensive_test.go`'s test-only
  `Screen.String()` table stops at `RestoreConfirm`, so every screen added since prints as "Unknown"
  in failures -- which is how this one read before its index was understood.
- **L16.** T4's red came before its code and spoke in behaviour rather than in build errors: "toolset is
  planned as 0, want skipped", "the executor ran \"toolset\", which the user turned off", "the run stopped
  on the unit that was turned off: it is on step 1", "returned tui.stepCompleteMsg, want the
  completion". `planOptions.DisabledUnits` and its copy were added first for exactly that reason: a
  guard that cannot compile fails for the wrong cause, and the cause is the only thing a red is worth
  reading for.
- **L17.** The state was the easy half and it is worth saying why. `StatusSkipped` had existed since
  the enum was written and nothing had ever assigned it, but three views already read it correctly:
  the plan panel does not count it as pending (`panels.go:1485`), the step list and the runner's list
  give it the finished marker (`view.go:2943`, `:3178`), and the status line has a label of its own
  (`view.go:3004`). T4 had to set it and teach both runners to walk past it -- nothing had to learn to
  display it.
- **L18.** Two choices are load-bearing. The three units the run needs are protected *in the plan*
  rather than only in the screen, which turns S3 from a promise the screen keeps into a property of the
  plan -- and the interactive walk recurses instead of falling through, so a unit turned off at the end
  of the plan finishes the run rather than leaving it waiting on a step that will never execute. The
  teeth are the evidence: four mutations, each biting its own guard with its own message -- dropping
  the fixed-unit clause, dropping the marking, and neutering each runner's skip -- with the three files
  restored byte-identical (`model.go` `f008647c739465e2`, `update.go` `0be36a9243225327`,
  `non_interactive.go` `855d8ccfbfcdb6ef`). Two of these guards assert an absence and pass trivially
  without a mutation; they are worth something because M1 and M2 bit.
