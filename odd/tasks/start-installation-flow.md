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
would be a lie: the plan says where the set comes from instead.

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
        reason cannot quietly stop being true. **This is a question back to the user**:
        whether the shell's set should become a switch at that price.
- [ ] **T2. The setup screen** (S1, S2, S7): rows for shell, terminal, multiplexer, editor,
      font and the extras, each cycling its value or taking a switch, with a plugin row
      indented under the tool it belongs to; all visible at once; `Enter` opens the row's rich
      list and `Esc` returns; the old question screens stop being steps of the flow and become
      that detour. Registered in every guard list.
- [ ] **T3. The plan screen, grouped and labelled** (S2, S5): rows grouped by area in the
      order the run takes them, each row saying whether it comes from a choice, from this
      machine, or from an extra switch, with the value chosen where there is one, and a set
      row indented under its tool saying either its switch state or where the set comes from.
- [ ] **T4. The command line and the plan agree** (S6): the sets and the switches the screen
      gains reach the flags and `--skip`, through the one builder.
- [ ] **T5. Docs and changelog** (S1-S7).
- [ ] **T6. The guards move with the screens** (S7): frame lists, counts, the plan-name
      golden, the teatest paths and the E2E expectations.

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
