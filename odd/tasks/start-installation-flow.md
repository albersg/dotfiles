# The start-installation flow: fewer screens, and every switch in view

The installer asks one question per screen -- operating system, terminal, font, shell,
window manager, Neovim, backup -- and then shows a plan listing units the user was never
asked about, while the plugin sets it provisions for five tools are not in the interface
at all. This front restructures that flow.

## Specs

**S1. Fewer screens.** Verbatim: *"Y tampcoo pedir todo en pantallas distintas creo que sea
buena idea, ya que son demasiados."* The decisions a run needs before it starts fit on one
screen: shell, terminal, multiplexer, editor, font, and the optional extras. A question that
fits beside another is not a screen of its own. The rich per-tool screens stay, as optional
detours reached from the row that decides them, and stop standing between the user and the
next decision.

**S2. Nothing reaches the plan that was never on screen.** Verbatim: *"Primero hace las
preguntas de zsh/fish, herdrh nvim..., y luego toda la lista de seleccionables, con cosas que
no han pedido antes..."* Every row of the plan is either a choice the flow asked for and
shows with the value taken, or a unit the machine or the run requires and says so, or an
extra that had its own switch on the setup screen. A unit that is none of the three is a
defect, not a default.

**S3. The plugins are a choice, visible and organic.** Verbatim: *"revisa tambien la parte de
osplugins si elegir instalarlos o no, de una forma mas clara visible y orgnica"* Every plugin
set the installer itself provisions is named in the interface, sits beside the tool it
belongs to, and can be turned off; turning one off is not a failure, it is reported like every
other thing the run chose not to do. A set the installer does not provision -- because the
tool bootstraps it from the configuration this installer ships -- is never given a switch that
would be a lie: the plan says where the set comes from instead. The shell's set is provisioned
and is not exempt from this rule (the user's decision, L13): it is a switch like the others,
and the configuration the run leaves behind has to stand on its own, which is what S4 asks of
it.

**S4. A configuration never names a plugin that is not there.** When a set is turned off, no
file this run writes may reference a plugin from it -- the rule the repository already
applies to the Neovim colorscheme and to the Tmux theme block placed after TPM.

**S5. Two vocabularies do not meet unannounced.** The setup screen speaks of choices, the
plan screen of units; where a unit is the consequence of a choice, the plan row says so
itself, so the user never has to translate between them.

**S6. The command line keeps parity.** Every switch the interface gains is reachable from the
command line, and the review screen and `--skip` keep planning through the one builder, so one
selection produces the same steps either way.

**S7. Nothing is lost by making it prettier.** The descriptions and learn content the
per-question screens carry stay reachable from the setup screen (the row opens the rich list,
`Esc` comes back), and every screen added or moved keeps its entry in each hand-maintained
guard list, with `measuredScreens` and the plan-name golden moved in the same commit.

## Tasks

The order is the dependency order: a set has to be switchable before the setup screen can offer
it, and the screen has to exist before the plan can show which choice each row came from.

- [ ] **T1. Every set the installer provisions becomes a switch** (S3, S4, S6), cheapest set
      first: each one is named in the interface, carried on the step of the tool it belongs
      to, honored when turned off, and paired with the configuration rule that keeps a
      skipped set from being named by a file this run writes.
      - **Herdr's set: committed as `99139b5`.** The registry, the set a step carries, the plan
        that shows it, the guard inside the step, and `--skip` naming both axes, with four
        guards that were red first.
      - **Tmux's set: done in this commit.** It is real installer work (`stepInstallWM` cloned
        TPM and ran `install_plugins`), and it is the set a configuration names, so leaving it
        out had to reach the file: `dotfiles-tmux/tmux.conf` now carries markers around the
        declarations and around the line that runs TPM, the patch strips exactly those, and the
        guard fails if a marker is missing, duplicated, or wide enough to take a key binding.
        The declarations and the key bindings that sat between them were grouped accordingly,
        and that regrouping is the only thing that moved in the file.
      - **The shell's set: not a switch, and the reason is measured.** It carries Oh My Zsh,
        the plugin packages and Powerlevel10k, and the `.zshrc` this installer installs is
        built on them: it declares the plugins (`plugins=(`), sources `$ZSH/oh-my-zsh.sh`
        (unguarded, so a machine without Oh My Zsh errors on every shell start) and sources
        `~/.p10k.zsh` for the prompt. Leaving the set out is therefore not a patch but a
        second, plain-zsh configuration this repository would have to ship and stand behind.
        The registry gained a third state for exactly this -- named, provisioned, not
        switchable, with the reason the row shows -- and
        `TestTheShellsSetIsNotASwitchBecauseTheConfigurationSourcesIt` reads the asset, so the
        reason cannot quietly stop being true. **The user answered yes (L13)**: the shell's set
        becomes a switch, its price is accepted, and the work is T7 rather than a reopening of
        this task.
- [ ] **T2. The setup screen** (S1, S2, S7): rows for shell, terminal, multiplexer, editor,
      font and the extras, each cycling its value or taking a switch, with a plugin row
      indented under the tool it belongs to; all visible at once; `Enter` opens the row's rich
      list and `Esc` returns; the old question screens stop being steps of the flow and become
      that detour. Registered in every guard list.
- [x] **T3. The plan screen, grouped and labelled** (S2, S5): rows grouped by area in the
      order the run takes them, each row saying whether it comes from a choice, from this
      machine, or from an extra switch, with the value chosen where there is one, and a set
      row indented under its tool saying either its switch state or where the set comes from.
- [x] **T4. The command line and the plan agree** (S6): the sets and the switches the screen
      gains reach the flags and `--skip`, through the one builder.
- [ ] **T5. Docs and changelog** (S1-S7).
- [ ] **T6. The guards move with the screens** (S7): frame lists, counts, the plan-name
      golden, the teatest paths and the E2E expectations.
- [x] **T7. The shell's set becomes a switch too** (S3, S4), the user's decision at L13.

## Log

- **L1.** The user's request, verbatim: *"No me termina de convencer el flujo de start
  installation. Primero hace las preguntas de zsh/fish, herdrh nvim..., y luego toda la lista
  de seleccionables, con cosas que no han pedido antes... Y tampcoo pedir todo en pantallas
  distintas creo que sea buena idea, ya que son demasiados. Dale una vuelta, razona al maximo
  como se podria organizar mejor todo esto, revisa tambien la parte de osplugins si elegir
  instalarlos o no, de una forma mas clara visible y orgnica, y reestructuralo."*

- **L2.** The flow as measured in the tree: `model.go:24-32` carries the question screens in
  order -- `ScreenOSSelect`, `ScreenTerminalSelect`, `ScreenFontSelect`, `ScreenShellSelect`,
  `ScreenWMSelect`, `ScreenNvimSelect` -- then the run; `installerFrameScreenNames`
  (`teatest_test.go:1919`) names them `os-select`, `terminal-select`, `terminal-select-wsl`,
  `font-select`, `shell-select`, `wm-select`, `nvim-select`, `ghostty-warning`,
  `install-review`. `docs/tui-installer.md:718` states the order and the promise that follows
  it ("first asks for the choices ..., then shows every unit it is about to install and lets
  each one be turned off"). `UserChoices` (`model.go:137`) holds OS, Terminal, Shell,
  WindowMgr, InstallNvim, InstallFont and CreateBackup; the plan's seventeen units come from
  `installUnits` (`install_units.go`) and their executors from `stepExecutors`
  (`installer.go:4504`). The command line already carries `--terminal`, `--shell`, `--wm`,
  `--nvim`, `--font`, `--backup` and `--skip` (`cmd/dotfiles/main.go:61-82`), which is the
  parity S6 has to keep. The question screens share one renderer (`view.go:2470
  renderSelection`: title, wrapped description, option rows, and a wizard progress meter in the
  header), which is what makes them collapsible into a form without losing their words.

- **L3.** Two decisions were left with the user before any code: whether the setup collapses
  into one form screen with the per-tool lists as optional detours, or the per-question screens
  stay and only the plan screen is regrouped; and whether a plugin switch is one per set or one
  per plugin inside each set.

- **L4.** The user's answer, verbatim: *"me parece bien tu recomendacion"* -- both
  recommendations accepted: **one form screen** for the decisions, with the per-tool lists kept
  as optional detours, and **one switch per plugin set**, not one per plugin inside a set.

- **L5.** Measurement that corrects T1's shape, taken before any code: the documented claim
  (`docs/tui-installer.md:794`) is that "the installer provisions plugins for five tools ...
  Tmux (it clones TPM and runs `install_plugins`), Fish, Zsh and Neovim". In the tree that is
  three sets of installer work and two that are not: Tmux is real (`stepInstallWM` clones TPM
  at `installer.go:7490` and runs `install_plugins` at `:7576`), the shell is real
  (`stepInstallShell` calls `installOhMyZsh` at `:7169`), Herdr is real (`stepInstallWM` calls
  `installHerdrPlugins` at `:7713`), while **Fish** is bootstrapped by the configuration
  itself (`dotfiles-fish/fish/config.fish:6-8` sources fisher and installs it, and the fish
  set's manager is vendored at `dotfiles-fish/fish/functions/fisher.fish`) and **Neovim's**
  set is bootstrapped by lazy.nvim from the shipped configuration (`installer.go:7751` notes
  the lock file is rewritten on the machine; no `nvim --headless` exists anywhere in the
  package). Giving those two a switch would be a switch that lies, so S3 as amended asks for
  the truth in the row instead. Second decision taken from the same measurement: a set is
  **carried on its tool's step** rather than added to the catalogue as a unit of its own --
  a second scheduling path for work the tool's executor already does is the defect class the
  catalogue guard exists to prevent -- so the sets do not change the plan's step list and the
  name golden stays what it is.

- **L6.** The first work unit, Herdr's set, red first and then green. What changed:
  `install_plugin_sets.go` (the registry, `PluginSetState`, the set-to-step mapping, and
  `ParseSkipSelection`, which reads units and sets from the one `--skip` list);
  `InstallStep.PluginSet` and `UserChoices.DisabledPluginSets` with the offender reading of
  an empty list as "everything on"; `planFor` carrying the set on the `wm` step; the guard
  inside `installHerdrPlugins` that runs no `herdr` command, asks the machine nothing and
  says the set was left out; and `cmd/dotfiles/main.go` where the flag reaches the same
  selection. The four guards, all red before the code existed:
  `TestThePlanCarriesThePluginSetOfItsTool`,
  `TestTheHerdrSetNamesThePluginsItProvisions`, `TestTurningOffHerdrsSetRunsNothing`,
  `TestTheCommandLineReachesTheSameSwitch`. The decision recorded in L5 held and was
  measured: because a set is not a unit, `TestThePlanIsStillThePlanItWas` and
  `TestThePlanNamesWhatItInstalls` did not move -- the plan's identifiers and names are
  what they were.

- **L7.** The second work unit, Tmux's set, red first and then green: a registry entry
  (`pluginSetIDTmux`), the switch honoured by the three places in `stepInstallWM` that do the
  plugin work (the TPM clone, the seed copy, the `install_plugins` run), and the configuration
  rule the other two sets do not need. `dotfiles-tmux/tmux.conf` gained two marker pairs --
  `# >>> dotfiles-plugins >>>` / `# <<< dotfiles-plugins <<<` around the declarations and the
  options that belong to them, `# >>> dotfiles-tpm >>>` / `# <<< dotfiles-tpm <<<` around the
  `# TPM init` comment and the line that runs TPM -- and the declarations were grouped so the
  key bindings that sat between them (the `M-g` scratch popup and the `M-l` lazygit popup) are
  outside the block. That regrouping is the only thing that moved: every declaration still
  precedes the TPM run line, which is the property `TestTmuxThemeBlockLoadsAfterPlugins`
  reads. The stripper lives in `tmux_plugins.go` and refuses rather than guesses when a marker
  is missing or unterminated, because the alternative is a `tmux.conf` that asks tmux to run a
  checkout nobody installed, on every start. Guards, red first:
  `TestThePluginMarkersBoundTheTextTheInstallerTreatsAsTheSet` (exactly one of each marker, in
  the order the stripper reads them, every declaration inside, the TPM line inside, and no key
  binding inside either block),
  `TestTurningOffTmuxsSetLeavesNoTPMAndNoPluginInTheServer` (both halves on the machine: with
  the set off no checkout and no declaration, with it on the configuration this repository
  ships), and `TestThePlanCarriesThePluginSetOfItsTool` raised from one set to a table over
  every **switchable** set, so a set added later without a plan row fails there instead of
  arriving in silence.

- **L8.** The shell's set, and the decision this front took against its own first plan. The
  plan the user approved said one switch per set, naming the shell among them; the
  measurement says that would be a lie, or a much bigger change than a patch. Evidence, read
  from the asset the run installs: `dotfiles-zsh/.zshrc:367` declares `plugins=(`, `:370` is
  `source "$ZSH/oh-my-zsh.sh"` (unguarded, so a machine without Oh My Zsh gets an error on
  every shell start), `:39-40` is Powerlevel10k's instant prompt and `:850-851` sources
  `~/.p10k.zsh` for the prompt itself. The plugin packages are installed by the shell step
  from the platform's packages (`installer.go` around `:6685`). So the registry gained a third
  state: `Switchable` separate from `Provisioned`, and `Because` for every set that is named
  and cannot be left out (the shell's, whose configuration is built on it; fish's and
  Neovim's, which their tools bootstrap themselves). The plan now carries the set of every
  tool a run installs -- `planFor` calls `planPluginSet` on the `shell` and `nvim` steps as
  well as the multiplexer's -- with `Switchable` and the reason, so the row can say what the
  run installs and why it is not the user's to change. Guards, red first:
  `TestEveryRegisteredSetIsNamedOnThePlanOfARunThatInstallsItsTool` (over the whole registry)
  and `TestTheShellsSetIsNotASwitchBecauseTheConfigurationSourcesIt` (the asset still carries
  the three facts, and `--skip` refuses the set with the same sentence the row will show).
  The whole-plan golden caught the wiring gap this left: the plan's `shell` and `nvim` steps
  attached no set until `planFor` was fixed.

  **Left with the user**: whether the shell's set should become a switch after all, at the
  price of this repository shipping and maintaining a second, plain-zsh configuration. That is
  a product decision about what the project supports, not a patch, so the row explains the set
  in the meantime instead of pretending.

- **L9.** Preflight on this tree: exit 1, and the four Ghostty tests are the only
  failures (`TestStepInstallTerminalGhosttyReportsAFailedDownload`,
  `TestStepInstallTerminalGhosttyRunsTheDownloadItFetches`,
  `TestInteractiveGhosttyScriptFailsWhenTheDownloadFails`,
  `TestInteractiveGhosttyScriptDownloadsThenRuns`). That is the host baseline every
  earlier tree carried and the one the v0.9.0 release preflight recorded. The two
  step-runner lines in the same output (`❌ FAILED: Step 'Configure WSL' failed`,
  `❌ FAILED: zsh installation failed`) are progress printed by tests that assert the
  failure path -- `step_loop_test.go:133` returns that error on purpose and
  `non_interactive_test.go:27` asserts the failure names its step -- not failures:
  the whole run has exactly four `--- FAIL` markers.

- **L10.** T2's design, settled before the code. One screen, `ScreenSetup`, reached from the
  install action of the main menu, replaces `OSSelect -> TerminalSelect -> FontSelect ->
  ShellSelect -> WMSelect -> NvimSelect` as the flow's steps; those six screens stay as they
  are and become detours. Rows, in the order the decisions are taken:
  `Platform` (the detected system, changeable), `Shell`, then the shell's plugin-set row
  indented under it, `Terminal`, `Font`, `Multiplexer`, then its set row, `Editor (Neovim)`,
  then its set row, `Existing configurations` (the backup decision, which is what "the extras"
  came to: every other extra is a unit with its switch on the plan screen), and a last action
  row, `Continue to the plan`, which is where `ScreenInstallReview` is entered from. Keys:
  `↑↓` move, `←→` cycle the row's value in place, `Space` toggles the rows that are switches
  (font, editor, backup, and each set the registry calls switchable), `Enter` opens the row's
  rich list as the detour, `Esc` leaves for the main menu. A set a run cannot leave out has no
  `Space`: its row states the reason instead, which is the same sentence the plan will show.
  Detour contract: `Enter` records `PrevScreen = ScreenSetup` and enters the row's screen; a
  choice there is recorded and returns to `ScreenSetup` instead of the next question, `Esc`
  returns unchanged, the `Learn about ...` rows inside keep their own return path, and the
  Ghostty warning keeps its two routes, both leading back to the setup screen. The wizard meter
  counts the two mandatory screens (`1/2` on the setup screen, `2/2` on the plan) and a detour
  inherits the meter of the screen it returns to, so the vital sign stays honest. The platform's
  forced answers (WSL and Termux terminal, Termux fonts) stay in one place -- `enterSetup`,
  which is `enterShellSelect` promoted -- and the rows show them as values, not as questions
  that were never asked. Guard lists that move in this commit: `installerFrameScreenNames` and
  its fixtures, `screen_coverage_test.go`, the teatest flow paths, `measuredScreens`, the
  E2E -- which drives the installer through `--non-interactive --dry-run` flags and not through
  keys, so it moves only where it names a screen.

- **L11.** T2a delivered and closed. One `ScreenSetup` replaced the six question screens as steps of
  the flow; they keep their content and become detours that record an answer and return to it. The
  rows are `Platform`, `Shell`, `Terminal`, `Font`, `Multiplexer`, `Editor (Neovim)`, `Existing
  configurations` (offered only when there is something to back up) and `Continue to the plan`;
  `↑↓` move, `←→` cycle, `Space` toggles, `Enter` opens the row's screen, `Esc` leaves for the main
  menu with the answers forgotten. The row list is one table (`setup_rows.go`), whose rows derive
  their values and their answer-recording from the question machinery (`questionValueOptions`,
  `questionRecordedAnswer`, `questionValueIndex`, `applyQuestionAnswer`) instead of copying an option
  string or an answer twice. The guard lists moved in the same change: `installerFrameScreenNames`
  and its fixture, `measuredScreens` 62 -> 63, the companion count 896 -> 912, the anim and panel
  lists, and the traces the teatest flows walk. Six pre-existing guards that pinned the old chain
  were replaced by ones holding the new property, never relaxed: the two `...SkipsFont` /
  `...GoesToFont` tests became `...ReturnsToSetup` and now check the recorded value as well.

  Two things the unit's own worker left, and the parent decided. First, the plan did not paint the
  meter: `wizardProgress` answered `2/2` and the guard pinned that, but the screen's header showed
  nothing, so criterion 6 was half true -- modelled and not shown. The meter is painted there now,
  and the guard checks the rendered header rather than only the model. Second, the backup screen was
  an orphan: the decision is a row, the screen was reachable from nowhere, and its `Esc` still pointed
  at the Neovim question. The row now opens it as its detour, which restores the list of
  configurations the run would replace (S7) and removes the dangling route; its `Cancel` row leaves
  the screen the way `Esc` does instead of aborting a wizard nobody started, because the abort lives
  on the setup screen's own `Esc`. That decision cost six pre-existing guards in four files,
  rewritten to the new contract rather than relaxed.

  Honest record of the evidence. The worker wrote its seven new guards *after* the implementation, so
  they have teeth by mutation and not by test-first RED; it said so rather than presenting them as
  red first, and it verified every restore by sha256. The parent then added one guard of its own for
  the row that opens the list, because `TestEveryDetourRowReturnsToTheSetupScreen` skips rows without
  a screen: losing that row's screen would have left the list unreachable and nothing would have
  failed. Both parent guards were then shown to have teeth by mutation, each with the baseline re-run
  first: the plan stops being the meter's second step (`view.go`, `ScreenInstallReview` returns
  `0, 0`) -> `TestTheFlowHasExactlyTwoMandatorySteps` fails on `want 2/2`; the backup row loses its
  screen (`setup_rows.go`) -> `TestTheBackupRowOpensTheScreenThatListsTheConfigurations` fails on
  `want the screen that lists the configurations`. Both files were restored byte-identical afterwards
  (sha256 recorded in the run). The first attempt at that harness proved nothing -- one mutation
  broke the build, so no assertion ran, and a "baseline" ran with a mutation still applied -- and its
  numbers were discarded rather than reported.

  Still open: whether the shell's set becomes a switch at the price of a second zsh configuration
  (L8), and what to do with the backup screen in the long run -- it is a detour now, and deleting it
  would be a T6 cleanup that moves three pinned counts again.

- **L12.** T2b delivered: the plugin set of a tool's step is a row under that tool. The row is derived
  from the registry with the same chosen-tool test the plan uses (`pluginSetForRow`), so a row and a
  plan step cannot disagree about which set a tool carries, and the row carries the registry ID rather
  than a copy of anything: what the set holds, whether a run can leave it out and why stay the
  registry's answers. A switchable set's row is toggled by `Space`, `Enter` or the arrows and writes
  `Choices.DisabledPluginSets` through `togglePluginSet` -- the same list `--skip` names -- and its
  value says `[x] install` or `[ ] leave it out`. A set a run cannot leave out says `always installed`
  and `toggleSetupRow` refuses to change it, because a switch there would promise to leave out work
  this installer does anyway. Three guards were written first and observed red on the missing symbols,
  and they hold the positive direction, both negative ones (a tool nobody chose and a tool with no
  registered set contribute no row), the place directly under the tool, the key through to the plan
  and back off, and the absent key with the value that says why.

  Reconciliation with L10, which described the row as stating the reason: the row states the state
  instead, and the reason stays where a full sentence fits -- the plan's row for the set and `--skip`'s
  refusal, which already carry `Because`. A row is one line and the shell's reason is longer than the
  frame's value column, so truncating it on screen would have made the one sentence that matters
  unreadable exactly where it is read. The consequence is recorded rather than hidden: the row says
  what the run will do (`always installed`), and the plan says where it comes from.

- **L13.** Requirement change, from the user: **the shell's plugin set becomes a switch.** The
  question the front left open at T1 was put to him with its price, and his answer is the option
  that carries it: *"Sí: que sea interruptor"*, whose consequences he accepted in the same choice,
  verbatim: *"un segundo .zshrc sin plugins, versionado y mantenido; el instalador elige cuál escribe
  según el interruptor; reescribir TestTheShellsSetIsNotASwitchBecauseTheConfigurationSourcesIt; la
  fila deja de decir 'always installed' y pasa a [x]/[ ]"*. S3 is rewritten accordingly, T1's bullet
  closes its question and points here, and the work is T7. One measurement stands between the
  decision and its price, and the measurement removed the price: the shipped `.zshrc` is already
  written to survive without the set -- the third-party loaders at 372-404 and the prompt's two
  references at 39-41 and 851 all carry their own presence test -- so the only unguarded dependency
  is the Oh My Zsh load at 366-370, and one marker pair bounds the whole region 366-406. The second
  configuration the option carried is therefore never shipped, and the same question round settled
  one thing about publication: PR #339 (a draft another session opened from this branch) is left
  exactly as it is, while commits and pushes on this branch stay with this session and PR, tag and
  release remain the user's own decisions.

- **L14.** T7 delivered: the shell's set is a switch, and the configuration pays for it. What
  changed: `dotfiles-zsh/.zshrc` carries `# >>> dotfiles-plugins >>>` / `# <<< dotfiles-plugins <<<`
  around exactly what the set provides (+2 lines, 366-406); `zsh_plugins.go` holds the markers, the
  replacement line the user reads, a filter that drops from every platform package list -- and from
  the unavailable-name lists, so the log cannot name a package the run decided not to request --
  every package that exists only to provide a plugin of the set, and `leaveZshPluginsOut`, which
  refuses rather than guesses when a marker is missing or unterminated, the contract
  `leaveTmuxPluginsOut` already had; the registry's shell entry is `Switchable` with its reason gone;
  and the shell step gates all three parts -- no Oh My Zsh checkout or installer, no plugin package,
  and the strip of the `.zshrc` it just wrote -- each with its own log line.

  The guards, red first: the stripped asset's fences and the switch's own guards failed on the
  missing symbols before anything was implemented, and the golden failed on the shell step's
  `PluginSet` pointer, which is what pinned the old reason. The six guards now hold: the markers
  bound exactly the set's text (a duplicate or missing marker, a declaration or loader left outside,
  and a setting swallowed inside are each their own failure), the file the run writes parses under
  zsh's own `zsh -n` with the set on and off, the strip refuses and leaves a refused file
  byte-for-byte alone, the step installs nothing of the set (and still does with the set on), the
  setup screen's row is a switch that reaches the plan, and every loader left outside the markers
  carries its presence test. Two limits are recorded rather than hidden: that last fence scans
  source-shaped loaders plus a named table, so a loader written as `eval` or `autoload` would slip
  past it, and `zsh -n` proves the file parses, not that a shell start is error-free at runtime.

  Two decisions the worker took, both sound and both recorded because a reader will ask. The
  Powerlevel10k prompt stays outside the markers: its two references are guarded by the presence of
  the exact file each one reads, and one marker pair cannot span the instant prompt near the top and
  the source near the bottom without swallowing 365 lines of PATH, autostart and editor settings --
  the too-wide defect the guard exists to catch. And `~/.p10k.zsh` is still copied when the set is
  off, because the theme is what writes that file, not the set; the line that sources it is guarded
  by its presence, so the configuration still starts. The worker also corrected one of the parent's
  measurements rather than following it: the `.zshrc` is written at `installer.go:7071`, not in the
  6100s, which is where `installOhMyZsh` lives; the `withPlugins` pattern it followed is at 7500.

  Evidence: `go test ./... -count=1` ran the whole suite, whose only failures are the four
  host-condition Ghostty tests; `gofmt -l` and `go vet` are clean; `zsh -n dotfiles-zsh/.zshrc`
  exits 0. The guard that said to delete itself, `TestTheShellsSetIsNotASwitchBecauseTheConfigurationSourcesIt`,
  was replaced by `TestTheShellsSetIsASwitchAndTheConfigurationPaysForIt`, which is stronger in the
  direction that changed (the registry, the flag and the plan), and the asset facts it used to
  check moved beside the strip that depends on them. The setup screen's no-key guard moved to
  fish's set at the same strength.

L15 -- T3 delivered, and the correction turned up a second real defect.

  The plan screen now draws the plan the run will execute: `planFor(m.planOptions())`, the one builder
  the command line and the non-interactive path already read, grouped under `Preparation`, `The tools
  you chose`, `The extras` and `Finishing up` in the order the run meets them. `InstallStep` carries
  `Origin`, `OriginValue` and `Area`, set at the condition that schedules the step (all eighteen
  append sites), so the screen reads the builder's answer instead of keeping a second table of step
  IDs; a step that arrives without either fails `TestNoScheduledStepArrivesWithoutAnOriginOrAnArea`.
  A tool's plugin set is drawn indented under its step, and the cursor rests only on rows `Space` can
  switch (`TestTheCursorVisitsEverySwitchableRowAndNothingElse`). The plan's identifiers and names
  are untouched: `TestThePlanIsStillThePlanItWas` and `TestThePlanNamesWhatItInstalls` still pin the
  list the E2E asserts, and the golden was re-pinned to the two new fields rather than relaxed.

  The parent's verification of that delivery found a real defect and sent it back to the same worker:
  eight rows -- `deps`, `xcode`, `homebrew`, `toolset`, `agentskills`, `officecli`, `wslconfig` and
  `theme` -- had been given the sentence *the run needs it*, which is the fixed units' clause, while
  every one of those eight is a row the cursor can switch off. One phrase was carrying both "this run
  cannot leave it out" and "this run happens to schedule it", which is what S5 forbids. The correction
  separates the two facts and gives each its own carrier: provenance is now one of four honest
  sentences -- *your choice decided it* with the value the choice took, *an extra you turned on*,
  *this machine needs it*, *every run installs it* -- and none of the four claims the run cannot leave
  the step out, while fixity stays the catalogue's `Fixed` flag and the clause the three fixed rows
  (`clone`, `cleanup`, `setshell`) already carried. The invariant is a guard, red first:
  `TestARowClaimsTheRunNeedsItsUnitIfAndOnlyIfTheUnitIsFixed` over the builder's steps for every host
  and every choice, `TestTheRenderedRowClaimsTheRunNeedsItsUnitIfAndOnlyIfTheUnitIsFixed` over the
  rows the screen actually draws, and `TestEveryProvenanceSentenceIsOneOfFourAndNoTwoShareOne` holding
  the vocabulary distinct with no fifth sentence hiding in the zero value.

  The correction worker was cancelled mid-flight when the parent session shut down, so the parent
  finished the review itself rather than relaunching equivalent work. The tree it left is complete:
  no occurrence of the rejected `stepOriginRequired` remains, all eighteen append sites carry one of
  the four origins, the three guards above are present, and the focused run is green. The worker's
  own text for the invariant names the defect it was written for, which is the RED narrative rather
  than a claim about a run nobody saw.

  Reviewing it turned up a second real defect, this one created by the first fix and caught by the
  new guards. A set a run cannot leave out was drawing the registry's full reason on its row, and
  that sentence is longer than the frame: `TestThePlanScreenShowsThePlanByAreaInTheRunsOrder` rendered
  the screen at 100 columns and reported `the frame does not draw the row "  the LazyVim plugin set
  -- lazy.nvim installs it on first launch from the configuration this repository ships"`, because
  the frame draws a row instead of wrapping one. The row was therefore cut exactly where a user reads
  where the set came from. The width guard added with the invariant then reported a second row: the
  theme step's row reached 81 characters against the 76 the narrowest terminal the frame matrix
  measures leaves. Two changes, both in the direction of the smallest honest text: the set row now
  says `the tool installs it` -- nineteen characters available after the longest set name, and true
  for both sets, because the tool's own configuration is what bootstraps them, which is exactly why
  there is no switch -- and the fourth provenance became *every run installs it*, parallel to the
  other three and short enough for the longest row that carries it. Nothing was weakened to make room:
  the registry's reason is still required non-empty by its own guards, still what `--skip` refuses
  with, and still what `docs/tui-installer.md` explains; `TestAPluginSetRowSitsUnderTheStepThatCarriesIt`
  now asserts the drawn phrase and the row's width at 80 columns, and the plan guard asserts no row
  of the plan exceeds the width the narrowest measured terminal leaves.

  Limits recorded rather than hidden: the width checks are asserted at 80 columns, the narrowest size
  the frame matrix measures, so a terminal narrower than that still cuts rows; and the phrase on a
  set row is deliberately less informative than the registry's reason, which is why the reason keeps
  its other carriers.

  Evidence: the focused origin, plan-identity and plugin-set guards pass with `-count=1`; `gofmt -l`
  and `go vet` are clean; the work-unit commit is `dcef1d1`. The gate result needs its caveat
  stated plainly, and L16 states it: `scripts/preflight.sh` ran the whole gate and its only failures
  were the four host-condition Ghostty tests, but the tree it ran in also held another writer's
  in-flight work, so that run proves this changeset passes alongside theirs rather than alone. The
  isolated re-run is recorded at the end of L16.

L16 -- The tree was shared, so the gate result needed a caveat and a second run.

  Closing T3 turned up something the work itself did not show: `git status` listed another writer's
  in-flight work in this same worktree. A WSL memory guard, with its own front
  (`odd/tasks/wsl-memory-guard.md`, born from two global OOMs on this host that ended the distribution
  rather than a process), a `.wslconfig` template change, a new `wsl-memory-guard` unit and its
  service, and edits to `view.go`, `repoassets.go`, `wslconfig.go`, `wslconfig_test.go`,
  `wslconfig_interactive_test.go`, `wsl_test.go`, `util_screen_test.go`, `screen_coverage_test.go` and
  `testdata/TestWSLResourcesGolden.golden`. Their file timestamps run 12:26 to 12:51; the T3 gate ran
  12:22 to 12:26, and a `go test ./internal/tui/` of theirs was still running when T3 was committed.

  Nothing of theirs was touched or committed. The commit used `git add` with nine explicit paths, never
  `-A`, so `dcef1d1` holds exactly the plan-screen changeset, and `origin/feat/start-installation-flow`
  reads that commit. Their files are still uncommitted in the worktree, which is where a writer's work
  belongs until its author commits it.

  The gate result therefore carries a caveat that has to be read, not smoothed over:
  `scripts/preflight.sh` ran on a tree that also held their uncommitted work, so its verdict -- the four
  host-condition Ghostty tests and nothing else -- is a statement about the union of the two changesets.
  It is not evidence that this changeset alone passes. A commit whose own verification was collected on
  a tree containing somebody else's edits is exactly the kind of claim this front has refused to make
  elsewhere, so it is corrected rather than left standing.

  The correction is a linked worktree of this same clone at `dcef1d1` (`git worktree add --detach`),
  which holds this commit alone, with `scripts/preflight.sh` run there and its result is stated in L17.
  It was not run at the moment of the discovery on purpose: a second full suite would have competed with
  the writer's own `go test ./internal/tui/` for a host that has 1.5 GB free after losing its
  distribution twice today to memory pressure, and adding to that pressure to prove a cosmetic flow
  change would be the wrong trade.

  One durable lesson, recorded because this will recur while the branch is shared: a gate run in a
  worktree another writer is editing measures their work too, and the honest thing to do is to state
  that in the evidence rather than present the run as a property of one commit.

L17 -- T4 delivered: the command line can name the platform, and the non-interactive path stops overwriting it.

  `--os=mac|linux|termux` fills the one choice the command line could not express. Measured before
  implementing rather than assumed: `RunNonInteractive` assigned the detected platform over whatever the
  caller had chosen, so a chosen platform could not survive at all, and `planOptions()` reads the platform
  from `Choices.OS`, so the choice is exactly what the plan needs. The vocabulary is read from the setup
  screen's own first row -- `questionAnswerKey` turns each option's label into the answer key the interface
  records -- so the flag and the row cannot drift apart, and WSL is deliberately not one of the values: it
  is a fact about the machine (`SystemInfo.IsWSL`, which schedules the configuration step) and not a
  platform a person chooses. An unknown value is refused before anything is planned, with the accepted
  values named, which is the shape `--channel` already had; the non-interactive path's other value refusals
  exit 1 and this one sits with them. The platform appears in the non-interactive summary as the platform
  the run will actually use, so a run with no flag reports what the machine decided.

  The guards were written first, and their RED was a build failure on symbols that did not exist yet
  (`SupportedOperatingSystems`, `ValidateOperatingSystem`, `PlatformFor`, `platformFromFlags` and the `os`
  field), so their assertions were never seen failing. That is a weaker red than an assertion, and rather
  than present it as more than it is, the teeth were measured by mutation: making `ValidateOperatingSystem`
  accept everything failed five assertions in the interface package and three in the binary's, and making
  `PlatformFor` discard the chosen platform failed with the message naming the platform it decided instead.
  Both mutations were reverted and the guards are green.

  The tests that hold it: `TestThePlatformFlagOffersExactlyWhatTheScreenOffers` (both directions, so neither
  list can grow alone), `TestThePlatformValueIsRefusedWhenTheScreenCannotOfferIt` (the refusal, and that it
  names every accepted value, because a refusal that does not teach gets typed again),
  `TestTheChosenPlatformSurvivesTheNonInteractivePath`, `TestTheChosenPlatformReachesThePlan` (the Apple
  developer tools for mac through the one builder, and not for linux), and in the binary
  `TestTheOperatingSystemFlagIsDeclaredAndRefusesWhatItCannotOffer` and
  `TestTheHelpNamesEveryPlatformTheFlagAccepts`, beside the existing `TestHelpDocumentsEveryFlag`, which
  refuses to pass for a flag nobody documented and so had to be answered with the help text.

  Evidence: `gofmt -l` and `go vet` are clean and the focused guards of both packages are green. T3's owed
  isolated verification is also recorded here, because it is what L16 pointed at: `scripts/preflight.sh`
  at `b79fffc` in the isolated worktree -- the clean tree, with none of the other writer's work in it --
  failed only the four host-condition Ghostty tests (`TestStepInstallTerminalGhosttyReportsAFailedDownload`,
  `TestStepInstallTerminalGhosttyRunsTheDownloadItFetches`, `TestInteractiveGhosttyScriptFailsWhenTheDownloadFails`
  and `TestInteractiveGhosttyScriptDownloadsThenRuns`), so T3 alone passes by the host-condition convention.
  The gate for this entry, its work-unit commit and the push are recorded in the todo board and in the
  commit message; `docs/tui-installer.md` documents the flag where the other non-interactive flags are,
  and `CHANGELOG.md` carries the entry.
