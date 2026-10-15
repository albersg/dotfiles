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
- **T3** (S5, S9) The screen: enum, title, description, options, panel, render, key dispatch, escape map. Route: worker. Commit: `feat(tui): a screen that shows every unit and its switch`.
- **T4** (S6) The selection reaches the plan, and a unit turned off reports itself as skipped. Route: worker. Commit: `feat(tui): a unit turned off says so instead of vanishing`.
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
