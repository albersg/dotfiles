# Changelog

All notable changes to the dotfiles downstream distribution will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Changed

- **The doctor walks the machine with a pool of checks instead of one check at a time, so a scan the owner reported as frozen now finishes in one pass of four rather than in the sum of twenty-seven.** The walk handed one check to Bubble Tea per answer, which is why the screen read as stuck rather than busy: the catalogue measures 27 checks and 2.3-3.1 s on the machine it was measured on, every check is bounded but the scan was their sum, and one slow tool held every remaining answer behind it -- the worst case, every check reaching its own bound, was 4 m 30 s. **The bound is what the pool buys, not the normal case.** The same walk measured 1.8-3.2 s over three runs, in eight rounds with at most four checks out at once, against 2.3-3.1 s over two runs on the sequential walk: one check -- `path.shadowed`, 1.25 s -- sets the floor for any ordering, so the tail is compressed and a typical walk barely moves, while the worst case becomes `ceil(checks/concurrency) x` the longest bound, about 70 s instead of 4 m 30 s, and Escape ends the run at the keystroke instead of at the next answer. `doctorConcurrency` is 4: `doctorStartRun` and every answer now start as many pending checks as the pool has room for (`doctorScheduleCmd`, returned as one `tea.Batch` so the runtime runs each check in its own goroutine), the run tracks `InFlight` and `Answered`, and `doctorFinishRun` closes it when every check in `run.Plan` has an answer. Nothing is reordered: the answers land in `run.Results` keyed by check ID and the report is still built from `run.Order`, which is the catalogue's order. The per-check bounds (`doctorVersionTimeout`, `doctorShellStartTimeout`, `doctorProbeTimeout`) did not move; the walk is bounded by `ceil(checks/concurrency) x` the longest bound.

  **Escape now cancels the checks in flight, and the notice counts answers rather than starts.** A command already handed to the runtime cannot be interrupted, so the old Escape waited for the next answer before the run could end -- and with several checks out that would have meant waiting for a bound of ten seconds. Escape now ends the run where it is, records everything that did not answer as `unknown` with the reason, and the answers still on their way are dropped by the guard in `update.go` (a run that has ended takes no more answers), so a cancelled run cannot rewrite the report it has already written. The notice reads **`Checked 0 of 27, 4 at once: <check>…`** -- how many have answered, out of how many, and how many are out at once -- and the closing line of a cancelled run states how far it got (`Cancelled after N of M`) instead of counting the plan. The scan's own caches (`lookups`, `versions`, `runs`) are the only mutable state a scan has and are now guarded by a mutex that is never held across a command, so two checks that ask the same question can both ask it and the cache is written by whichever finishes first. **A retry or a category run takes a new run ID.** Both replace the plan on the run that is already there, and the commands of the plan they replace may still be in flight; counted against the new plan, their answers could close it before its own checks had answered, so the run takes a new ID and the guard in `update.go` drops them. The answers the retry does not replace are kept, which is what makes a retry a retry rather than a new run. The update centre (`startUpdateCenterScan`) is deliberately untouched, and not because it is merely a separate change: its managers cannot run together at all. The code already says why -- apt, pacman, dnf and pkg keep locks over the same package state, and the screen's own `InFlight` guard exists so two managers are never started on either path, checking or applying. The doctor's checks read; the update centre's managers write, and #373's fourth decision reasons from a shared shape that differs exactly there.

  The guards, on a walk driven the way the runtime drives it -- one goroutine per command, a batch expanded into the commands it holds: `TestDoctorWalkKeepsSeveralChecksInFlight` runs four checks that each announce themselves and wait, and fails with `1 of 4 checks were in flight at once` on the sequential walk; `TestDoctorCancelledRunDropsTheAnswersStillInFlight` ends a run with Escape and then feeds the four answers that were already on their way, asserting the results, the closing line and the bytes of the report are untouched; `TestDoctorScanCachesSurviveConcurrentChecks` asks the scan from eight goroutines at once and reports a data race under `-race` when the caches are not guarded; `TestDoctorRetryDropsTheAnswersOfThePlanItReplaced` replaces the plan with a two-check retry while four answers are out and asserts that none of them is counted or written, and fails without the new ID with `answers to the plan that was replaced closed the plan that replaced it`. `TestDoctorScreenSaysHowManyChecksAreDoneOutOfHowMany` replaces `TestDoctorScreenRunsOneCheckAtATimeAndSaysWhich`, which pinned the one-check-per-message contract this change removes, and the test helper that walks a run to its end now expands a batch instead of reading one command per step.

## [v0.10.0] — 2026-10-18

### Added

- **The Linux and WSL runs raise IPv4 above IPv6 in `/etc/gai.conf` before they do anything that needs the network, and a file the step cannot safely own is left exactly as it was.** The address-selection policy glibc reads decides which family `getaddrinfo` answers with, so a host that advertises IPv6 without carrying it turns every download into a wait for a route that never arrives -- and the run's own `git clone` is the first thing to pay for it. The step appends one line, `precedence ::ffff:0:0/96 100`, inside a block it marks with `# >>> dotfiles: prefer IPv4 (gai) >>>` and `# <<< dotfiles: prefer IPv4 (gai) <<<`, so a second run replaces its own block in place instead of stacking another one; the destination is `/etc/gai.conf` unless `DOTFILES_GAI_CONF_PATH` names another file. It is scheduled on Linux and WSL only, and before `deps`, because the clone is where a broken route is felt first (issue #361).

  A precedence the owner set **outside** the block governs the file: the step writes nothing and names the line it found, because a policy a person wrote is not the installer's to override. A file that already prefers IPv4 is left alone. A value inside the block is restored to the value the block declares, and the log names the value it replaced, because the block is the installer's own text and the log is the only place the previous value survives. Four malformed shapes produce no write at all and one line saying why -- a start marker without its end, an end without its start, a duplicated block, and markers in the wrong order -- since text bounded by markers that are themselves wrong cannot be trusted to be the installer's. A write that fails warns and continues rather than failing the run: the preference is a convenience, and the file belongs to `libc-bin` on Debian and Ubuntu, whose upgrades may replace it, which the review line beside the step says. The step names the owning package it read from `dpkg`, or says it could not read it instead of guessing.

  Fifteen guards hold it, and five of them exist because an independent pass over the candidate reproduced four defects on the tree before they were fixed: `TestGAIStepDoesNotDeleteContentAfterAnUnterminatedBlock` (the first version deleted the user's own lines after an unclosed marker), `TestGAIStepLeavesAForeignPrecedenceOutsideTheMarkedBlock` (it wrote over a precedence the owner had set outside its block), `TestGAIStepLeavesADuplicatedBlockAlone` (a file holding its block twice was half-managed), `TestGAIStepRestoresAValueInsideTheBlockAndNamesIt` (a restored value was silent) and `TestGAIStepWarnsAndContinuesWhenTheWriteFails` (a failed write stopped the whole installation). They sit beside the ten that pin the scheduling, the append to a factory file, the in-place replacement, the already-IPv4 file, the foreign precedence, the review line naming `libc-bin` and the upgrade risk, the missing and the failing `dpkg` lookup, the package name read from `dpkg`, and the shared writer. The container proof runs both halves -- `test_gai_prefers_ipv4` against a factory file, `test_gai_leaves_foreign_precedence` beside it. [TUI Installer](docs/tui-installer.md#the-ipv4-preference) documents the step, its boundary and the four shapes it refuses, [Tools Reference](docs/tools.md) lists it beside the other units, and `DOTFILES_GAI_CONF_PATH` is in [Branding](docs/BRANDING.md).

- **The installer now diagnoses the development environment it just ran on, and repairs only what it can prove.** A run that finishes leaves a machine whose faults surface later and elsewhere -- a command that is not found in a new terminal, a shell that has been slow to start for a month, a tool that cannot read the file it wrote -- and the only way to ask about them was to read the configuration by hand. The Utilities section gains **Diagnose the development environment**: a read-only scan over six categories (PATH and tool resolution, the shells and their configuration, the managed tools and the commands a run itself needs, the dotfiles this installer wrote, git and ssh, and the environment: home, the XDG directories, the temporary directory, room on disk and open-file limits, the locale, terminfo, the installer's own variables and a platform's own faults), whose every finding states **the evidence it was read from**, the impact it has and what to do about it -- a tool is asked for its version, a shell is asked for its own syntax check, a mode is read rather than assumed. Each check is one bounded command, so progress is real and names the check it is running, `Esc` cancels a run in flight, and the checks that never ran are recorded as `unknown` with the reason instead of disappearing; `unknown` is deliberately the zero value of the five states (`ok`, `warning`, `error`, `not applicable`, `unknown`) and outranks `ok` when a scan produced both, so a machine nobody finished looking at can never be summarised as a clean one. The states are printed as words and plain glyphs as well as colour, an undetectable answer is `unknown` with the manual command that replaces it, and a platform-specific fault is `not applicable` with the reason on every other platform. A finished run writes a **redacted report** beside the installer's own record and can export it; a scan writes nothing, and no secret is read in the first place -- the credential helper is reported by kind, `.git-credentials` by presence and mode, `ssh-add -l` is counted rather than listed, and every written line passes the redactor. Only two repairs are offered -- **creating a directory** the environment itself names, and **setting the mode** of `~/.ssh` to `0700` or a key to `0600` -- and each is a preview of the effect, the risk, the permission needed and what is kept, a re-read that refuses a state that moved, one change, a verification that reads the result back, and a record of the previous state with the exact command that puts it back; everything else the scan finds is advice, and the doctor never edits a startup file, installs a package or deletes a file.

- **The installer can show what the machine's own package managers have to update, and move only what the user selects through the manager that owns it.** The Utilities section had a row for every small job except the one a machine with eight package managers actually needs: what is out of date, who owns it, and what moving it would cost. The new **Check the installed tools for updates** row opens a package update center that reads what Homebrew, apt, pkg (Termux), pacman, dnf, snap, Flatpak and rustup each report, one manager at a time, and plans a command only from an explicit selection. An entry's identity is its manager, its scope and its name, so a `git` from Homebrew and a `git` from apt are two rows with two owners and are never merged into one; a pinned entry (`brew pin`, an apt hold, `pacman`'s `IgnorePkg`, `dnf versionlock`, a held snap) is shown with the manager's own words about the hold and refused when it is selected. Four answers stay four answers: updates, current, cannot be asked here (with the reason), and could-not-be-read (with the manager's own words -- an unreachable mirror, a refused permission, a lock another process holds, a conflict). "Nothing to update" is never printed for a manager that was never asked, and a version a manager did not print is left empty instead of guessed.

  Nothing runs on its own. Opening the screen only reads; there is no "update everything" row anywhere, and the row that opens the plan carries the number of entries selected. The confirmation names every command exactly as it will run, the entries it moves, whether it needs the administrator, what stays pending afterwards (a service that keeps the old version until it is restarted, an application that must be reopened) and what going back would cost in the manager's own terms. Elevation is `sudo -n --`, never an interactive prompt: a machine whose administrator program would ask for a password is refused with the command named rather than left waiting on a prompt the frame cannot show. Check and apply both run one manager at a time, because apt, pacman, dnf and pkg keep locks over the same package state; a running command's output is shown as it arrives and can be stopped with the stop row or `c`, and Escape does not leave the screen while a command is running. When a command finishes, the manager that ran it is read again before anything is concluded, so each entry is reported as requested, executed or confirmed -- three different facts -- with failed, cancelled and unreadable outcomes reported as what they are. After the run the whole inventory is read again, so the list that follows is the machine's answer and not the plan's expectation. Every command is a list of arguments handed to the manager directly, never a line of shell, so a package name can never become a second command and there is no `curl | sh` on this path; credentials in a manager's output are redacted before the text reaches the screen.

  The engine is its own package (`installer/internal/updates`), layered so that detection, one adapter per manager, the resolution of a selection into commands, and the screen are separable, and the runner is a seam an adapter is tested through instead of being run. The guards, red first: `TestSelectionTogglesAndKeepsIdentity` and `TestAFreshSelectionsFirstToggleIsNotLost` (the second is the regression for a real defect the screen's own guard found -- `Selection.Toggle` took the selection by value, so the first press on a screen whose selection was still its zero value created the map on a copy and threw it away), `TestChannelsReportWhatCanBeAskedAndWhyNot`, `TestReadNeverMergesTwoManagersInstallations`, `TestPinnedEntriesAreRefusedByEveryAdapter`, `TestApplyReReadsBeforeItConfirms`, the parser tables in `adapters_test.go` (apt's simulation reports only `Inst` lines that name an earlier version and lists what was kept back separately, pacman's arrow and `[ignored]`, dnf's exit status 100 and its `Obsoleting` section, snap's per-snapshot versions, Flatpak's join by application and branch, rustup's update lines, Homebrew's pinned and multi-version entries), and seventeen screen guards in `update_center_test.go` -- the door and the first manager, one manager at a time, two managers' same-named installs staying two rows, a held entry refused in the manager's words, no command running until the plan is confirmed, the running frame refusing to be left behind, confirmed only when the manager says so followed by the re-read, a stopped run reporting what it never reached, the bounded log, the three honest states, the absent "update all", the filter, navigation skipping the rules, and the panel's line. The screen is also in the twelve-size frame matrix and the emoji guard (`updateCenterFrameCase`), the utilities golden was regenerated for the new row, and `TestEveryScreenConstantNamesItself` guards the diagnostic label after the update center first appeared as `Unknown` in a failing guard's message. [TUI Installer](docs/tui-installer.md#the-package-update-center) documents the row, the eight managers with the exact command each is read and moved with, and the rules above.

  An independent verification pass read the whole change against its own specifications and found three blocking defects and three should-fix gaps, all now fixed with a guard each, red first. A reading pass could be re-entered -- the re-check key, leaving the screen and coming back mid-pass, and confirming a plan while a manager's read was still outstanding could each put two managers on the machine at once, which is the lock this screen exists never to take -- so a pass that is already out is refused everywhere and the apply row is neither drawn nor honoured until every manager has answered. The managers' own caveats about their answers were computed and never drawn, which presented an incomplete list as a complete one: apt's kept-back list, pacman's unreadable ignore list, Homebrew's unrefreshed metadata and Flatpak's assumed installation now follow their manager's line as `note:` paragraphs, an entry's own note rides its row, and a hold list apt or pkg could not read says so instead of leaving a held entry looking movable. The Utilities panel called a failed read "nothing to update", which retired the question with an answer nobody gave, and now reports what could not be read beside what the others answered. An entry whose command a stop never started was reported as "left out" -- a claim that no command could carry it -- instead of as requested and never run, which is now a real per-entry outcome (`StepRequested`), so requested, executed, confirmed and left out stay four different facts. A filtered list can be taken in one press (`Select the N shown)` / `Drop the N shown)`), which never reaches past the filter or past a hold. Two documentation rows named commands the code does not run (`snap refresh --hold` for `snap held`, and Flatpak's installed-version read with the wrong column set) and now name the real ones, and the section's new paragraph moved to the end of the Utilities body so that the terminal capability line it had pushed behind the frame's counter is on screen again. The new guards: `TestTheRecheckKeyStartsNoSecondPassWhileAReadIsOutstanding`, `TestLeavingTheScreenAndReturningStartsNoSecondPass`, `TestApplyIsRefusedWhileAReadIsStillOutstanding`, `TestTheChannelsOwnCaveatsReachTheScreen`, `TestAnEntrysOwnNoteReachesItsRow`, `TestTheUtilitiesPanelNeverCallsAFailedReadNothingToUpdate`, `TestAnUnreadableHoldListIsSaidOutLoud`, `TestAnUnreadableHoldListDoesNotClaimThereAreNoHolds`, `TestTheSelectVisibleRowTakesTheCategoryTheFilterShows` and `TestARequestedEntryThatNeverRanIsReportedAsRequested`.

- **The shell's startup is measured as a plan of named starts, with an instrument per shell and a history the user keeps.** The Utilities section's **Measure the shell's startup** row used to answer one question with one shape: `$SHELL -i -c exit`, five runs, a median and a range, and an attribution only where zsh had `zprof`. It now measures the start the user names -- the interactive start a terminal gives you, the login start, or a clean start that reads no configuration at all -- with the number of runs (five by default, up to twenty), an optional warm-up run that is thrown away, and an optional clean-configuration comparison, and the line above the rule draws the exact command a press would run so the plan and the measurement cannot disagree. The statistics follow the runs: the mean beside the median, the spread with a steady/variable/erratic verdict from three runs, the 95th percentile and a trimmed median that drops the samples outside 1.5 times the interquartile range from eight, each reported only when the runs behind it exist -- and each named as missing, with the count it would need, when they do not, rather than printed as a zero. Attribution is per shell and honest about itself: zsh's own `zprof`, bash 5.0+ traced with `PS4` + `EPOCHREALTIME`, fish 3.6+ through its own `--profile-startup`, and for every other shell or an instrument that reports no table, the screen says only the total is measurable and names the reason. The paragraph that introduces the table is written for the instrument that ran -- a trace is not introduced as a profile, and a share the instrument did not print is not printed by the screen -- and a table larger than the size the utility reads is refused with its size named rather than parsed. `Esc` now stops the runs that are left while keeping the samples that finished, counted as neither a timeout nor a failure. With the clean comparison on, the difference between the two medians is reported as the shell's own time versus what the configuration adds, refused when the two sides are not comparable (another shell, another version, another start, a stopped side, too few runs) and never presented as a saving when the difference is negative. `[h]` keeps the last fifty measurements in `shell-audit-history.json` under the state directory -- off until the user turns it on, numbers and already-redacted names only, written through a temporary neighbour and a rename -- and turns them into a trend against the stored medians, refused across a start, a version or a stopped run. `[r]` opens recommendations labelled **Measured**, **Observed** or **Confirmed** by the kind of claim each one is, every one citing its evidence and none naming a cause the utility did not measure. Nothing is written to the user's configuration: the wrappers and tables the profiled start needs live in a temporary directory that is removed again, the real startup files are sourced from where the shell already looked for them, a clean start is given an empty directory of the utility's own, captured output is discarded and error text is bounded. The terminal is context now rather than a gate -- a run with no terminal attached is measured and says which terminal it ran under -- and the refusals that remain are no `$SHELL` and a login shell this machine does not have.

  The guards came red first, one work unit at a time: `shellaudit_stats_test.go` (the median, the hinges, Tukey fences, the floors below which a statistic is named as unreported rather than zeroed), `shellaudit_modes_test.go` (each shell's start, the clean directory each one honours, the version gate for bash's trace and fish's startup profile, the refusal to offer a start the shell does not have), `shellaudit_engine_test.go` (a warm-up that is made and thrown away, one event per start, a plan the cancel does not shrink, the bound on the whole plan, a start the shell does not offer named as such), `shellaudit_profile_test.go` (each instrument's table parsed heaviest-first, a secret the profile printed staying out of the drawn rows, bash's deltas read as deltas, the trace of a login start refused with the reason, every write inside the run's own directory and removed), `shellaudit_baseline_test.go` (the subtraction and its share, a negative difference called not separable, and six refusals), `shellaudit_history_test.go` (the stored fields pinned as an exact JSON key set, newest-first and bounded, the trend's floor and its refusals, evidence required in every recommendation and causal prose asserted absent, a history that cannot be read reported rather than overwritten) and `shellaudit_ui_test.go` (the plan keys changing one setting each, the plan reaching the measurement by the arguments the probe was really called with, every start reported as it lands, the cancel pressed while a read is in flight, the history written only when the switch is on, the recommendations leading the text, and the screen measured with a measurement in flight at the fit guard's twelve terminals). Two guards outside those files were stale rather than deleted and now pin the new truth: the utilities section offers the row wherever a login shell resolved, and a run with no terminal is measured rather than refused. [TUI Installer](docs/tui-installer.md#the-shells-startup) and the [README](README.md) describe the plan, the instruments, the comparison and the stored history.

- **The installer can say what a WSL distribution is doing wrong, and changes nothing while saying it.** A project read from the Windows drive pays for every file it opens, and nothing in the interface could say so: the utilities section was written for the machine as a whole. It now carries a **Diagnose this WSL environment** row whose screen reads the mount table, `/proc/meminfo`, `/etc/wsl.conf`, `/etc/resolv.conf`, the same `.wslconfig` the resources screen reads and the project directories beside the working directory, and states each finding with its severity, the evidence it was read from, the consequence in plain words, the scope the fix would need and whether the claim is **measured** on this machine or **documented** behaviour. Findings are ordered by severity, and a reading that failed is a finding of its own -- an unreadable mount table, a missing `/proc/meminfo` and an unread resolver are reported as gaps rather than omitted, because an empty section and a healthy machine look identical on screen. Opening the screen changes nothing, installs nothing and dials nothing.

  **The row is offered only inside a real WSL instance, and the gate is not an environment variable.** `WSL_DISTRO_NAME` is inherited by any child process, so a Windows host that merely has WSL installed -- or a build agent running under a Windows shell -- would satisfy it while having none of the filesystem the advice is about. Detection reads the evidence the instance leaves behind: the kernel release and version, `WSLInterop` in the binfmt registry, `/run/WSL` and `/usr/lib/wsl`, with more than one signal required where the kernel does not name WSL outright, and a container on the WSL kernel refused with its own reason, because Docker Desktop's backend runs on that kernel while its filesystem is not an instance's. Storage is classified from the filesystem type each mount exposes, never from a path: `[automount] root` can move the Windows drives anywhere, automount can be off, a drive can arrive later with `wsl --mount` and a mount point can contain a space, so a path resolves to the longest mount boundary above it -- `/mnt/cd` is never answered by the verdict for `/mnt/c` -- and an unknown filesystem type is reported as unknown rather than assumed to be fine.

  **Two rows act, one level in, and each exists only where its engine can finish; neither deletes anything.** The measurement writes scratch directories of its own inside each target, bounds itself at 64 files of 64 KiB per pass, 32 MiB and 4096 files per target and 30 seconds per run, removes what it wrote even after a failure or the time bound, reports an operation it could not run as a note instead of a rate of zero, and carries the caveats beside the numbers -- the order the targets were measured in and the warmer cache the second one meets -- with the comparison stated as relative, because a faster copy here proves nothing about a build, a test suite or a Git operation. The assisted copy previews the source, the destination, what would be copied, what is left behind and the room behind the destination, then creates the destination only when it is absent, refuses one that already holds anything, opens every file with `O_EXCL`, and opens no source path for writing at all: the by-hand removal of the source and the `cp`/`rsync` equivalent stay quoted data the installer never executes, and a copy stopped with `Esc` leaves the partial destination in place with what it placed already stated, because deleting it would destroy the evidence of what went wrong. Afterwards the copy is verified rather than trusted -- the path set, each entry's type, size, mode, a symlink's target and the bytes up to 8 MiB per file, with the entries covered by size and mode alone counted -- and a difference is reported, never repaired.

  The guards: `TestDetectWSLRuntimeInsideFixtures`, `TestDetectWSLRuntimeOutsideFixtures`, `TestDetectWSLRuntimeNeverContradictsSystemInfo` and `TestWSLAdvisorCollectOnThisHost` hold the gate in both directions, the last of them on the machine that runs the suite; `TestParseMountTableReadsPointsTypesAndSources`, `TestMountTableResolveUsesTheLongestBoundary`, `TestParseMountTableRejectsAMalformedLineByNameAndNumber` and `TestReadMountTableFromMissingFileReportsTheReason` hold the reading; `TestClassifyStorageKinds`, `TestClassifyStorageSaysWhenItCannotTell` and `TestWSLFindingsNameTheHostMountWhereverItIs` hold that a path is never the answer; `TestWSLFindingsAreOrderedAndComplete` and `TestWSLFindingsStorageCases` hold what a finding says; `TestPlanMigrationBlocksANonEmptyDestinationAndCreatesNothing`, `TestRunMigrationNeverOverwritesAnExistingDestinationEntry`, `TestRunMigrationStopsOnAReadErrorAndKeepsTheSourceAndThePartialCopy`, `TestRunMigrationInstructionsAreQuotedAndNotExecuted` and `TestVerifyMigrationNamesEveryDifference` hold the copy; `TestRunBenchmarkLeavesTheTargetExactlyAsItWas`, `TestRunBenchmarkStopsAtTheDeadlineAndCleansUp`, `TestRunBenchmarkNeverEmitsAZeroThroughputRow`, `TestRunBenchmarkReportsTheReductionWhenItCaps` and `TestRunBenchmarkContaminationStatesTheRequiredCaveats` hold the measurement; `TestWSLModuleRunsNoCommandAndAssumesNoHostPath` fails if any file of the module names a shell, a command runner or an assumed host path; and `TestEveryScreenFitsEveryTerminalSize` with `TestWSLAdvisorGolden` holds the frames at every measured size and at the 80x24 floor. [TUI Installer](docs/tui-installer.md#the-wsl-performance-advisor) documents the gate, the report and the two rows.

- **The WSL distribution has a memory guard now, and the installer proves it instead of claiming it.** Under WSL every process in the distribution -- the shell, the editor, systemd itself and its journal -- lives in one cgroup, `init.scope`, and nothing bounded it. Two global out-of-memory kills on 2026-10-16 (11:39:48 and 11:53:54) took the whole distribution down because `init.scope` carried systemd's default `OOMPolicy=stop`: one OOM-killed task makes systemd stop the scope, and every process in the distribution is inside that scope. An earlier six-minute freeze (12:26) was the same problem's other half, because a numeric `memory.high` is a throttle: a spike is stalled rather than killed. The repository now ships `dotfiles-wsl/wsl-memory-guard.sh` with `dotfiles-wsl/wsl-memory-guard.service`, and the Configure WSL step installs both -- `memory.high=infinity` so there is no throttle band, `memory.max` at the VM's own total minus 1536 MiB, `memory.swap.max=1280M`, `memory.oom.group=0`, and `OOMPolicy=continue`, which is the half that decides whether the distribution survives at all. That property is a **unit** property, which systemd 257 refuses through `set-property`, so the guard writes a drop-in scoped to the scope and reads the effective policy back rather than assuming either route worked; it deliberately refuses to write the manager-wide `DefaultOOMPolicy`, which reaches every unit on the machine and would stop an OOM-killed service from ever firing its `Restart=`, and reports that remedy with its cost when the scoped route does not take. The payload installs `0755` because systemd has to execute it, the step reads `OOMPolicy` and `memory.max` back and then requires the cap to survive a second `daemon-reload`, and no degradation fails the step, because the `.wslconfig` half has already succeeded. The Utilities section carries the state beside the WSL row (`missing`, `not running`, `unprotected` or `active`), the WSL resource screen names the failure each missing half produces in the terms it was measured in, and **Repair the WSL memory guard** installs and re-reads it without a full run. `TestInstallWSLMemoryGuardInstallsAndVerifies` and its five degradation cases hold the step, `TestEveryProgramTheRepositoryShipsIsInstalledExecutable` holds the mode, and `TestWSLResourceScreenNamesWhatAMissingGuardCosts` holds the screen. [TUI Installer](docs/tui-installer.md#the-wsl-memory-guard) and [Manual Installation](docs/manual-installation.md#8-apply-the-wsl-configuration) describe it.

- **The installer can say which stage of a connection stopped, and finds out without changing anything.** Every support thread starts with a host that cannot be reached, and the interface had no way to answer it: the utilities section offered the theme switches, the WSL resources and three read-only reports, so a user whose connection failed left the installer to find out why. The section now offers a **Diagnose the network** row that opens a read-only centre. It describes this machine's own network first -- the interfaces and their addresses, the default route, the configured resolvers, the proxy the environment declares -- and then separates a connection into the stages that fail differently -- local, resolver, transport, certificate and application -- because a name that does not resolve, a port that refuses, a socket a firewall keeps silent and a certificate this machine does not trust are four problems with four owners, and a screen that reports "no internet" for all of them is worse than no screen at all. One row per check carries its status (`ok`, `noted`, `failed`, `unknown` or `skipped`), and the cursor on a row is what shows that step: what it measured, how long it took, the cause a failure most plausibly has, the next check to make and the command to run by hand.

  **Nothing is changed, and nothing is skipped.** No resolver, route, firewall, proxy, VPN or certificate setting is written, no verification can be disabled (`Probe.HTTPClient` has no parameter for it), no capability is probed by changing one, and no tool is installed; the one check that starts a program -- the optional `tracepath`/`traceroute` row -- runs only where the machine already has one, only for a destination the user typed, and never for the overview. Every check is bounded by construction, a bound left at zero takes the default instead of removing the limit, the checks run one at a time so a slow link is diagnosed rather than saturated, and `Esc` stops a run while keeping what finished: the report still lands, with the steps that ran listed and the rest named as skipped rather than missing. The overview contacts exactly one host -- the release API this installer already talks to for its own update check -- and the screen names it and says why; everything else is a destination the user types: a host, an IPv4 or IPv6 address with its zone, a `host:port`, an internationalized name, or a complete `http`/`https` URL. A destination is validated before anything is dialled, the URL's credentials are dropped before any request, and a request's query string is never part of the report. The latency it prints is the TCP handshake's, never a ping's, because a network that drops ICMP is diagnosed exactly like one that answers it. Inside WSL the report says which side it is describing, and a machine missing a capability degrades one check at a time with the reason on that row.

  **The report is available only after it has been read.** The preview row shows the file line for line and the save row exists only from that preview, so what is written cannot differ from what was reviewed; every line of the text passes through the same redactor rather than being trusted; and the file is written 0600 under the installer's own state directory, one per run. The anonymised form replaces this machine's addresses, user name, home directory and machine name -- and says, in its row, in its preview paragraph and in the file's own `anonymized:` header, that the host being diagnosed is kept, because that host is what a support reader needs and it is not this machine's identity.

  The guards: `TestScreenNetworkDiagnosticsNamesTheStageAndTheEvidence` holds the rows and what a row shows, `TestUtilitiesSectionOpensTheDiagnosisWithoutDialling` holds that opening the screen dials nothing, `TestRunFollowsTheStageOrderAndStreamsEveryCheck` holds the order and the streaming, `TestRunReportsAStoppedRunWithoutLosingWhatFinished` and `TestVerdictSaysWhenAStoppedRunLeftLaterStagesUnchecked` hold the cancellation, `TestNetworkScreenWritesOnlyFromThePreviewAndOnlyWhatItShows` and `TestNetworkScreenReportsWhenThereIsNowhereToSave` hold the write, `TestTextIsRedactedByConstruction` and `TestRunNeverPutsAQueryStringOrACredentialInTheReport` hold the redaction, `TestCheckInterfacesDoesNotCountABridgeAsAWayOut` holds that a bridge is not a way off the machine while a tunnel is, `TestNotRunChecksAreNamedLikeTheChecksThemselves` holds that a step the run skipped is named the way the run names it, and `TestNetworkDiagnosticsGolden` pins the 80x24 frame with `TestNetworkDiagnosticsFrameFitIsMeasuredAtEverySize` at every size the guards measure. Two defects an independent verification found are fixed with them: the certificate check assigned its title after three early returns, so a TLS step the run skipped and one it ran read as two different checks, and the anonymised report now states what it keeps. [TUI Installer](docs/tui-installer.md#the-network-diagnosis) documents the screen, the stages and both forms of the report.

- **The shell's plugin set is a switch too, and the `.zshrc` a run writes with it off stands on its own.** The sets this installer provisions became switches one at a time, and the shell's was the one left out on purpose: the `.zshrc` the run installs declared `plugins=(`, sourced `$ZSH/oh-my-zsh.sh` unguarded and sourced `~/.p10k.zsh` for the prompt, so leaving the set out looked like a second, plain-zsh configuration this repository would have to ship and maintain. The measurement that changed it is the markers: `dotfiles-zsh/.zshrc` now carries `# >>> dotfiles-plugins >>>` / `# <<< dotfiles-plugins <<<` around exactly what the set provides -- the plugin declaration, the Oh My Zsh source and the third-party loaders -- and a run that leaves the set out (`--skip=plugins-shell`, or the row under Shell on the setup screen) runs no Oh My Zsh installer, requests no package that exists only to provide a plugin of the set, and replaces that block with a line saying why, so the configuration it leaves behind starts on a machine without Oh My Zsh, Powerlevel10k or the plugin packages. The strip refuses rather than guesses when a marker is missing or unterminated, the same contract `leaveTmuxPluginsOut` already had.

  The guards, red first: `TestTheZshPluginMarkersBoundTheTextTheInstallerTreatsAsTheSet` fails if the block grows wide enough to swallow a setting that is not the set's or if a declaration or loader survives outside it, `TestTurningOffTheShellsSetLeavesNoPluginInTheConfiguration` holds both halves on the file the run writes -- including that zsh itself parses it (`zsh -n`, skipped where zsh is absent) -- `TestTheZshStripRefusesWhenAMarkerIsMissingOrUnterminated` holds the refusal and that a refused file is left byte-for-byte alone, `TestTheZshStepInstallsNothingOfTheSetWhenItIsOff` holds the executor, `TestTheSetupScreenCanTurnTheShellsSetOff` follows the row through to the plan, and `TestEveryPluginTheZshConfigurationNamesOutsideTheMarkersCarriesAPresenceTest` holds the boundary the markers do not cover -- the Powerlevel10k prompt, carapace and fzf-tab loaders, each already guarded by a presence test. The registry's third state loses the shell's entry: the set is `Switchable`, the reason it could not be left out is gone, and `TestTheShellsSetIsASwitchAndTheConfigurationPaysForIt` replaces `TestTheShellsSetIsNotASwitchBecauseTheConfigurationSourcesIt`, whose failure message asked for exactly this. [Tools Reference](docs/tools.md#plugins) and [TUI Installer](docs/tui-installer.md#installation-flow) describe the switch and the configuration it leaves behind.

- **The plugin set a step provisions can be left out of a run, and a configuration a run writes never names a plugin it did not install.** The installer provisions plugin sets for five tools and the interface never named one of them: a set was work inside its tool's step, so a user who did not want it had no way to say so and nothing to read. The sets this installer installs itself are now carried by the step of the tool that installs them -- Herdr's two declared plugins, and Tmux's set, which its own configuration declares -- so a row can name what the step would install and whether the run turned it off, and `--skip=plugins-herdr` or `--skip=plugins-tmux` leaves it out. A set that was turned off is neither a failure nor a silent no-op: the step runs no `herdr` command at all, asks the machine nothing, installs nothing and updates nothing, and its log says the set was left out, the way a skipped unit is reported. `--skip` names both axes because to a user they are one question -- what should this run not do -- and a name in the sets' own namespace that no set answers to is refused as a set rather than reported as an unknown unit, so the error sends the user to the list that has it.   The Tmux set is the one a configuration names, so leaving it out has to reach the file: the installed `tmux.conf` declares every plugin through TPM and runs TPM near the end, and a machine without that checkout would be asked to run a file that is not there on every start. The configuration now carries markers around exactly that text -- the declarations, and separately the line that runs TPM -- and a run that left the set out strips both and leaves one line saying why. The markers are what makes the strip exact rather than a guess: too wide would take a key binding out of the user's configuration, too narrow would leave a plugin declared on a machine that has none, and `TestThePluginMarkersBoundTheTextTheInstallerTreatsAsTheSet` fails on either. The declarations, the options that belong to them and the key bindings that sat between them are grouped accordingly in `dotfiles-tmux/tmux.conf`, which is the only thing that moved in that file.

  The set is on the setup screen, under the tool that installs it. A set a run installs was
  work inside its tool's step and nothing else, so the screen that asks which tools to install
  said nothing about the plugins they would bring. The row list now carries the registry's set
  for the chosen tool as a row indented under it -- the same registry and the same chosen-tool
  test the plan reads, so a row and a plan step cannot disagree about which set a tool carries
  -- with the switch the flag carries for the sets a run can leave out (`Space`, `Enter` or the
  arrows write the same list `--skip` names) and `always installed` with no key at all for the
  three a run cannot leave out. `TestTheSetupScreenNamesThePluginSetOfTheToolItInstalls` holds
  the row and its place directly under its tool, including both negative directions -- a tool
  nobody chose contributes no row, and a tool this installer provisions nothing for gets none
  -- `TestTheSetupScreenCanTurnAProvisionedSetOff` follows the key through to the plan the
  executor reads and back on again, and `TestTheSetupScreenGivesNoKeyToASetTheRunCannotLeaveOut`
  holds the absent key and the value that says why.

  The row list is a window now, and it is drawn as one because the plugin rows took the screen
  past the terminal: `TestEveryScreenFitsEveryTerminalSize` reported the setup screen drawing **21
  rows on the 60x20 case** of the twelve sizes it measures, one more than the terminal has. The fix
  is the plan's own arithmetic, renamed to what it is -- `listWindowSize`, `windowRowsFor` and
  `listWindowMin` now serve both lists -- and the rows outside the window are counted above and
  below rather than dropped, which `TestTheSetupWindowShowsTheRowTheCursorIsOnAndCountsWhatItHides`
  holds for the first and last row of the list.

  Five guards hold the switch: `TestThePlanCarriesThePluginSetOfItsTool` (every switchable set is carried by the step of its tool, on by default and off when told, with the step still planned either way), `TestTheHerdrSetNamesThePluginsItProvisions` (the interface's label is read from the table the step installs from, so a renamed plugin moves the label), `TestTurningOffHerdrsSetRunsNothing` and `TestTurningOffTmuxsSetLeavesNoTPMAndNoPluginInTheServer` (no command, no registry read, no checkout on disk, and a configuration that declares nothing), and `TestTheCommandLineReachesTheSameSwitch` (`--skip=officecli,plugins-herdr` reads as one unit and one set, and a name nothing recognises and a unit the run needs both still refuse). `TestEveryRegisteredSetIsNamedOnThePlanOfARunThatInstallsItsTool` closes the other direction for the whole registry: every set is named on the plan of a run that installs its tool, switchable or not, because a set nobody can see is the defect this work exists to remove.

  The three sets that are named and are **not** switches are the other half of that, and the difference is measured rather than a preference. The shell's set -- Oh My Zsh, the plugin packages and the Powerlevel10k prompt -- is installed by this installer and cannot be left out, because the zsh configuration this repository ships is built on it: the `.zshrc` the run installs declares the plugins, sources `$ZSH/oh-my-zsh.sh` and sources `~/.p10k.zsh` for the prompt, so a switch there would mean shipping and maintaining a second, plain-zsh configuration rather than patching a file. Fish's set and Neovim's are not the installer's to leave out at all: the fish configuration bootstraps Fisher on first start, and lazy.nvim installs the Neovim set on first launch. `TestTheShellsSetIsNotASwitchBecauseTheConfigurationSourcesIt` reads the shipped `.zshrc` for those three facts and fails the moment the configuration stops depending on the set, so the reason the row shows cannot quietly become false -- and `--skip` refuses the set with the same sentence.

- **The non-interactive path can say which platform it is installing for, and no longer overwrites the answer.** `dotfiles --os=mac|linux|termux` fills the one choice the command line could not express: the non-interactive path detected the platform and assigned it over whatever the caller had chosen, so a run could not be *told* it was installing for another platform at all. The flag's values are read from the setup screen's first row rather than written out a second time -- `questionAnswerKey` turns each option's label into the answer key the interface records, and WSL is deliberately not one of them, because WSL is a fact about the machine (`SystemInfo.IsWSL`, which schedules the configuration step) and not something a person chooses -- so a platform the screen gains cannot leave the flag behind and the flag cannot offer a platform no row can record. A value outside that vocabulary is refused before anything is planned, with the accepted values named, instead of being quietly read as Linux. The choice then reaches the plan through the one builder: `planFor` reads the chosen platform, so `--os mac` schedules the Apple developer tools and `--os linux` does not, which is what makes the flag mean anything at all.

  The guards, written before the code and red on a build that did not have the symbols yet: `TestThePlatformFlagOffersExactlyWhatTheScreenOffers` compares the flag's vocabulary with the screen's own answer keys in both directions, `TestThePlatformValueIsRefusedWhenTheScreenCannotOfferIt` holds the refusal and that it names every accepted value, `TestTheChosenPlatformSurvivesTheNonInteractivePath` holds that a chosen platform is the one the path decides and that only an absent one falls back to the machine, `TestTheChosenPlatformReachesThePlan` holds the plan half through the one builder, and in the binary `TestTheOperatingSystemFlagIsDeclaredAndRefusesWhatItCannotOffer` and `TestTheHelpNamesEveryPlatformTheFlagAccepts` hold the flag and the help. Their teeth were measured by mutation rather than assumed: making the refusal accept everything failed five assertions in one package and three in the other, and making the path discard the chosen platform failed with the message naming the platform it decided instead. `dotfiles --help` and [TUI Installer](docs/tui-installer.md#non-interactive-mode) document the flag and its values.

### Changed

- **The profile system is gone from the documentation, because nothing ever read it.** `docs/PROFILES.md` and the eight files under `profiles/` -- a README and seven YAMLs naming `default`, `minimal`, `full`, `linux`, `macos`, `ci` and `termux` -- described named sets of the installer's units, and no code, workflow or script read any of them: what a run does is decided by the flags and the setup screen. The profiles had been a proposal carried as documentation for long enough to read as a feature, which is the defect, and 229 lines leave the repository. The only reference to them inside it was `docs/PROFILES.md` pointing at `profiles/README.md`, checked before deleting, and the fact that nothing read them had already been written down as a finding in `odd/tasks/installer-customization.md`. The choice, and the two alternatives it was chosen over, are recorded in issue #333, and the files stay in the history, so restoring them is a revert rather than a rewrite.

- **The WSL VM takes three quarters of the host's RAM instead of two thirds.** The shipped policy planned `memory` as `max(min(host - host/3, host - 4 GiB), host/2)`; the ratio is now a quarter, `max(min(host - host/4, host - 4 GiB), host/2)`. On the 16,016 MiB host the 2026-10-16 OOMs ran on, the old fraction planned `memory=10240MB` and the new one plans `memory=11776MB`: the ratio asks for 12,012 MiB, the 4 GiB Windows reserve leaves 11,920, and the lower of the two falls to 23 whole steps of 512. Windows keeps about 4.1 GiB instead of 5.6. Raising this number is what the memory guard made safe rather than reckless: before it, reaching `memory.max` meant systemd stopped `init.scope` and took every process in the distribution with it, and now the scope caps itself, `OOMPolicy=continue` absorbs the kill and `memory.high` is infinite, so a runaway workload dies alone. The extra 1,536 MB is also the headroom the measured failure needed: with `/tmp` on tmpfs, `shmem` counted 4.5 GiB against an 8,409 MiB cap. Hosts small enough for the reserve to decide are unchanged -- 2 GiB still plans 1,024 MB, 4 GiB 2,048 and 8 GiB 4,096 -- while 32 GiB plans 24,576 and 64 GiB 49,152. The processor rule is deliberately unchanged: a quarter of a 12-CPU host would be 9 rather than the 11 the all-but-one rule plans, so the same fraction would have taken parallelism away on the axis where the 12:26 freeze was measured. `TestPlanWSLResources` pins every case and `TestRenderWSLConfigForAKnownHost` the exact bytes; [TUI Installer](docs/tui-installer.md#the-wsl-resources) and [Manual Installation](docs/manual-installation.md#8-apply-the-wsl-configuration) were carrying the old fraction.

- **The guard's swap allowance inside the scope went from 512 MiB to 1280 MiB, because a bound that tight turned reclaimable pages into kills.** Measured on 2026-10-16 with five agents running: the scope's `memory.max` is 8409 MiB and 4524 MiB of it was shmem -- tmpfs pages, because `/tmp` is a tmpfs in WSL -- while the agents' own heaps came to 2151 MiB between them. shmem is reclaimable, but only by swapping, and 512 MiB of allowance against a VM holding 1536 MiB left the kernel nowhere to put the pages it could not drop, so it killed the process holding the most: five `pi` agents over one afternoon. 1280 of the VM's 1536 leaves the kernel 256 MiB and gives pressure somewhere to go instead of taking a process with it. `TestWSLMemoryGuardScriptAppliesTheFourLimits` pins the value and the guard's own comment carries the measurement. It is the second line of defence: the first is not leaving 4.5 GiB of a memory-backed `/tmp` behind, which `TMPDIR` outside this repository decides.

- **The WSL limits the installer derives from the host follow the host instead of a fixed fraction of it, and the template's own comments changed with the rule.** The old policy gave the VM half the host's RAM with the reserve pinned at 2 GiB and handed it every logical CPU, which on a 16,016 MiB / 12-CPU machine planned `memory=7680MB, processors=12` -- the size at which the 2026-10-16 kills happened -- and left Windows competing with the VM over all twelve. `memory` is now derived from the host's RAM, never leaving Windows below 4 GiB and never below half the host, which is WSL's own proportional default and so never gives less than WSL would; `processors` is every logical CPU the host reports except one, so Windows keeps one; and swap stays a quarter of the planned memory. The same host plans `memory=11776MB, processors=11, swap=2560MB` under the fraction the entry above states. `TestPlanWSLResources` pins this host, both boundaries, the single-CPU host and the unknown host, and `.wslconfig.tmpl`'s comments state the new arithmetic, because a template comment that describes the previous policy is the defect this removes. [TUI Installer](docs/tui-installer.md#the-wsl-resources) and [Manual Installation](docs/manual-installation.md#deriving-the-limits-yourself) document it.

- **The plan screen shows the run's own plan, grouped by area and labelled with where each row came from.** The screen between the last choice and the run listed the catalogue rather than the plan: it could not say why a row was there, and a unit the machine would never schedule still took a row while the plugin sets the run provisions were absent from it. It now draws the plan `planFor` will execute -- the same builder the command line reads -- in the order the run takes it, grouped under the areas the run meets (`Preparation`, `The tools you chose`, `The extras` and `Finishing up`), with each row saying where it came from in one of four honest sentences: *your choice decided it* and the value the choice took (`Install kitty — your choice decided it: kitty`), *an extra you turned on* for a switch that is on, *this machine needs it* for the dependency step, the Apple developer tools, Homebrew and the WSL configuration, and *every run installs it* for the toolset, the agent skills, OfficeCLI and the theme. A step's plugin set is drawn indented directly under its step, saying either the switch's state (`[x]`/`[ ]`) or, for a set a run cannot leave out, where the set comes from, in a phrase short enough for the narrowest terminal the frame matrix measures -- the frame draws a row instead of wrapping it, and the registry's full reason for the two sets that carry one is longer than that row -- which `TestAPluginSetRowSitsUnderTheStepThatCarriesIt` holds together with the registry's own guard that the reason stays non-empty; the cursor refusing to rest on those rows, and the row the frame cuts in half being reported as a row the frame never drew, is how the too-long reason was found. Fixity is a separate fact with its own carrier, the catalogue's `Fixed` flag: the three units the run cannot leave out -- `clone`, `cleanup` and `setshell` -- read *the run needs it*, so that phrase never describes a row the cursor can switch off. The origin and the area live on `InstallStep`, set at the condition that schedules the step, so the screen reads the builder's answer instead of keeping a second table of step IDs and a step that arrives without either fails `TestNoScheduledStepArrivesWithoutAnOriginOrAnArea`. `TestARowClaimsTheRunNeedsItsUnitIfAndOnlyIfTheUnitIsFixed` and its rendered twin `TestTheRenderedRowClaimsTheRunNeedsItsUnitIfAndOnlyIfTheUnitIsFixed` hold that a row claims the run needs a unit if and only if that unit is fixed -- both were red first, on the eight switchable rows that said *the run needs it* -- and `TestEveryProvenanceSentenceIsOneOfFourAndNoTwoShareOne` holds the four provenances distinct. A tool's plugin set is drawn indented directly under its step, saying either the switch's state (`[x]`/`[ ]`) or, for a set a run cannot leave out, where the set comes from, which `TestAPluginSetRowSitsUnderTheStepThatCarriesIt` holds. The cursor rests only on rows `Space` can switch: it steps over the area headings, the plugin-set rows and the three fixed units, and walks to every switchable unit in the plan, which `TestTheCursorVisitsEverySwitchableRowAndNothingElse` holds. The window, the counting line and the meter are unchanged and stay held by their existing guards, and the plan's identifiers and names are untouched -- `TestThePlanIsStillThePlanItWas` and `TestThePlanNamesWhatItInstalls` pin the same list and the same names the E2E asserts. [TUI Installer](docs/tui-installer.md#installation-flow) describes the grouped plan and its rows.

- **The installer's questions are one setup screen now, and the six rich screens it used to chain are detours that return to it.** The flow was one question per screen -- platform, then terminal, then font, then shell, then multiplexer, then editor, then the backup decision -- so what a run was going to do to the machine was spread across six screens a user could only read one at a time, and every screen's `Enter` led to the next question rather than back to a place where the whole set could be seen at once. The choices are a single **setup screen**: one row per decision, each row carrying the value the choices hold on the same line, and a last row that opens the plan. `↑`/`↓` move the cursor, `←`/`→` cycle the focused row's value in place, `Space` turns the font, editor and backup switches on or off, `Enter` opens the focused row's own screen as a detour, and `Esc` leaves for the main menu with the answers forgotten. The six screens themselves are unchanged -- their lists, their `Learn about ...` rows, the font's sample and the Ghostty compatibility warning all keep their behaviour -- but a choice records the answer and returns to the setup screen instead of advancing to a next question, and `Esc` returns without changing anything. The setup screen is the only entry to those screens, so the question-to-question chain is gone rather than reordered: `TestTheFlowHasNoQuestionToQuestionTransition` walks all six and `TestEveryDetourRowReturnsToTheSetupScreen` holds the detour contract for every row the screen offers, whether it opens a screen of its own or is a switch turned in place.

  The platform's forced answers are no longer questions that are never put. A WSL or Termux host has no terminal emulator question, and Termux has no font question; the old flow skipped those screens, and the answer is instead applied when the setup screen is entered and shown as the row's own value, so the row states what the run will do rather than resting on a value nobody was asked for. The derivation lives in one function the setup screen and the run's own planning both read, which `TestTheForcedPlatformAnswersComeFromOnePlace` holds.

  The wizard's counter counts the two screens the flow cannot skip: the setup screen shows **`1/2`** where it showed the old six-step count, the plan is the second and last, and a detour inherits the setup screen's half -- so the meter never claims a step the flow does not have, which `TestTheFlowHasExactlyTwoMandatorySteps` pins. Its rows are still measured at all twelve terminals the frame matrix measures, the setup screen draws its own rows rather than a side panel because each row already says what its choice does, and the count of screens the frame guards render is written out so that adding a screen stays a deliberate act. [TUI Installer](docs/tui-installer.md#installation-flow) describes the setup screen and the detours it opens.

- **The Utilities section can check this machine against the last install.** A completed run writes a record -- one entry per configuration path, naming what the run did to that path and where the copy it took went -- and the only reader of it was the command line's `--verify`, so a person who had just run the installer had to leave the interface to ask whether the machine was still what the run left. The section's new **Check the machine against the last install** row opens a read-only screen that walks that record once and compares every entry with the machine: the paths the run wrote and the backups it took that are no longer here each get their own row -- `missing  zsh  ~/.zshrc  replaced by the run` for a path, `missing  backup of zsh  ~/.zshrc.bak` for a copy -- and what is still here is counted rather than listed, because a normal run's record is longer than any frame. The screen says where the full list is (`dotfiles --verify` prints every entry, including the ones that are here) instead of printing a truncated version of it.

  **It reports the limit of the record rather than hiding it.** `installEntry` names no digest, so a path that is present is reported as present and never as unchanged: the check establishes that the file is there, not that it is still what the install left; the body says so in the same paragraph as the check it just ran. Four states are answered and none is guessed: no record means no run has completed here and there is nothing to check yet, an unreadable record says why, a record naming no path says there is nothing to be compared (neither a pass nor a failure), and a record with entries reports what is gone.

  **One walk, two presenters.** The walk was lifted out of `VerifyInstall` into `InspectInstall`, which returns the facts; the command line's report and this screen are two presenters of them. Two walks of one record could disagree about the same file, and only one of them would be right. Nothing runs at startup and nothing blocks a keystroke: the check runs as a command when the screen is opened, and the panel's own row reads the model -- `not checked yet` before the first answer -- so a press never waits on a machine whose disks are not answering. Nine tests hold it, and all nine were red against the tree before the utility existed in the plainest way a guard can be: `install_verify_test.go` does not compile without the screen it names. `TestTheSectionOffersTheInstallVerification` (the row and the screen), `TestChoosingTheVerificationRowRunsTheCheckOffTheUpdateLoop` (the command), `TestTheVerificationReadsTheVerdictRatherThanTheDiskAgain` (a keypress reads nothing), `TestTheVerificationScreenIsReadOnly` (the only key is the way back and the filesystem is untouched), `TestTheVerificationPanelValueComesFromTheModel` (the panel's four states), the report's three bodies pinned one per state (`TestTheVerificationScreenShowsWhatIsGoneAndSaysWhatIsStillHere`, `TestTheVerificationScreenSaysWhenNothingIsGone`, `TestTheVerificationScreenSaysWhatItCannotCheck`), and `TestInstallVerifyGolden` at 80x24. `TestVerifyInstallContractIsPinned` holds the other half of the change: the record shapes the command line's report was already read against, so extracting `InspectInstall` out of `VerifyInstall` cannot drift the exported contract silently. The screen is registered in the frame matrix, so it is measured against all twelve terminal sizes rather than the one it was written on; that registration is what `TestEveryScreenFitsEveryTerminalSize`'s pinned count caught when the case was first added.

  The section's own sentences were rewritten to fit five jobs in one paragraph each, and `docs/tui-installer.md` gained a section for the check beside the terminal capability report. `dotfiles --verify` is unchanged: the same walk, the same lines, the same exit status.

### Fixed

- **The clone step handed no timeout to a runner that takes one, so a transfer that stopped moving hung instead of failing.** `stepCloneRepo` called `system.RunWithLogs` with `nil` options although the runner already bounds a command with `context.WithTimeout` and `CommandContext`: a `git clone` that stalls -- a dropped connection, a proxy that answers and then stops -- waited for the job's own twenty-minute ceiling and reported the whole matrix as a timeout, which sends the search after a defect that is not there. `cloneTimeout` is five minutes now and the step passes it, the way the theme commands are bounded. `TestStepCloneRepoIsBoundedWhenTheTransferStalls` puts a `git` on `PATH` that sleeps for thirty seconds: red against the tree before the fix, where the twenty-second watchdog fired, and green in about two (issue #359).

- **Three tests asked the machine a question and reported its answer as the code's behaviour.** They are one defect in three packages, and each had failed on a pull request whose diff could not reach it, which is what makes a flaky test expensive rather than merely annoying: it is read as evidence about the change under it. `TestTrainerMenuGolden` pinned the whole `teatest` stream, so the golden carried the number of frames the emulator happened to emit instead of the menu's own frame; it now waits for the same text, quits, and pins `m.View()` through `requireGoldenCapture` -- and the first attempt at that fix read `tm.FinalModel` and wrote a 23-byte golden holding nothing but the terminal title, which would have passed forever and was discarded (issue #358). `TestDiscoverAndStopAProcessTheTestOwns` leaned on `/proc` being readable and on the port being free, neither of which it owns; it now asserts the stop's own claim -- that it never blames the child it stopped -- and holds whether the socket is gone or still bound (issue #357). `TestCleanRunsTheOwnerCommandAndMeasuresTheRelease` demanded that the free space of the filesystem grow around a clean run, which another process writing at that moment hides; it now follows the rule the same file already states in its hardlink test, where a measured delta is checked for being non-negative and an unmeasured one has to carry a reason and no bytes (issue #369). What does not depend on the machine is untouched in all three: the owner's command still runs, the store is still emptied, and the item still reports a walking measurement of the bytes it released.

- **The assertion my own port fix added proved something the kernel does not promise, and CI proved it by disagreeing.** The test compared the inode the listening socket had before the stop with the one the port carried afterwards, and failed when the two numbers differed. Inode numbers are reused, so the run that broke it had a fresh socket, from another process, wearing exactly the number the stopped child had held: the assertion reported a live child where there was none. The comparison is gone and the reason is written where it stood, so it is not added back, because a test cannot assert a process is gone through a number the kernel recycles (issue #367).

- **The terminal capability report could lose its own answer to the interface reading the same terminal, and that answer could arrive as keystrokes.** The reply to a capability query is written into the terminal's input queue -- the queue a keystroke goes into, and the queue the interface reads -- so with the interface reading the terminal and the probe opening `/dev/tty` for itself, the answer went to whichever read first. Measured on one pty with a terminal that answers all three queries (`scripts/measure-terminal-ownership.sh`): the probe alone received the whole 41-byte reply in 5 of 5 runs, and with the interface reading **its own open of the terminal** -- the arrangement bubbletea creates whenever stdin is not a terminal -- it received **nothing in 1 of 20**, and in that run the terminal's answer reached the interface as **seven input messages**, `\x1bP` read as alt-`P`, an XTGETTCAP body read as typed text and the DECRQM half swallowed by the parser. The same run fell back to `COLORTERM` and answered `unknown` about a terminal that answers all three. Which second reader that is matters, and the harness's own correction is what named it: with the pty on stdin -- the arrangement the installer commonly runs in -- twenty runs gave the probe the whole reply in **20 of 20**, so the race is real without being equally likely in both arrangements, and neither sample is a rate. The terminal now has **one reader**: `tui.TerminalInput` wraps `os.Stdin` and the program is given it, and while a query is outstanding the reply leaves the stream there instead of reaching the interface. The reader is handed over only **when stdin is the terminal the run is attached to**, which is the whole subtlety of this change: bubbletea opens the controlling terminal by itself when stdin is not it, and handing it any input option at all takes that choice away -- a program given a pipe reads the pipe, and no key ever reaches it. `programInput` makes that decision with bubbletea's own predicate, on the same descriptor: `charmbracelet/x/term`'s `IsTerminal` is the function bubbletea itself calls before it decides whether to read stdin or open `/dev/tty`, so the interface reads exactly the stream it would have read and only the probe's answer leaves it. Two other tests look right and are not, which is why the code says so: this package's own `isTerminal` is `ModeCharDevice` and `/dev/null` satisfies it, and comparing devices with `os.SameFile` against an opened `/dev/tty` matches **nothing ever** -- that descriptor reports the devtmpfs node (dev 5, inode 10) and not the terminal it stands for, so the reader was never handed over and this change was **inert** until the guard's own skip message printed both devices. `TestAPipedStdinIsLeftToBubbletea`, `TestACharacterDeviceThatIsNotATerminalIsLeftToBubbletea` and `TestTheTerminalIsReadThroughTheOneReader` hold the gate, the last one observable only where the tests have a terminal on stdin (run the compiled test under `script(1)`) and printing what stdin is when it skips instead of skipping silently. The wrapper keeps the stream's descriptor, so bubbletea still makes the terminal raw (`term.MakeRaw` runs on the input's `Fd()`, and a wrapper without one would be a program that never enters raw mode at all), and outside the probe's window it hands every byte on untouched -- the property that matters most, because it sits on the path every keystroke takes, and `TestTheInterfaceLosesNothingWhenTheProbeIsNotAsking` is the guard that holds it. `TestTheInterfaceReadsThroughTheReaderAndTheProbeGetsTheAnswer` is the wiring guard, on a pipe rather than a pty: it fails loudly when the program cannot start at all, which is the failure mode that otherwise looks exactly like an answer nobody could read. The filter itself is held by `TestAnAnswerIsTakenOutOfTheInputStream`, `TestTypingIsForwardedUntouched`, `TestTypingBeforeAnAnswerStillReachesTheInterface`, `TestAnAnswerSplitAfterItsHeaderIsDivertedWhole`, `TestATerminatorSplitAcrossReadsEndsTheAnswer` (the string terminator split between its two bytes), `TestTheAnswerIsDivertedWholeWhenItArrivesInPieces`, `TestTwoAnswersInOneReadAreBothDiverted`, `TestTypingOnBothSidesOfAnAnswerIsForwarded`, `TestAnErrorFromTheTerminalIsHandedOn`, `TestAnErrorThatArrivesWithTheAnswerIsStillHandedOn`, `TestAWindowLeavesNoStateBehind` and `TestTheWindowIsOpenBeforeTheQueryIsWritten` -- the window opens before the query is written, because a fast terminal answers while the query is still on its way out. `TestASplitInsideAnAnswerHeaderLetsTheFragmentThrough` records the one case the filter cannot decide: a read boundary inside an answer's own header leaves a fragment indistinguishable from input, so it is forwarded and that answer is lost -- bounded and one-sided, and written to fail loudly if a later change makes it wait for the rest of the header instead. The probe's own half is unchanged where it still applies: a program started without this reader writes **no query at all**, which is the arrangement the next sentence is about, and the three-state report, the sources it names and the `unknown` with a reason are untouched. **The other arrangement is closed too, by not asking.** When stdin is **not** the terminal the gate hands nothing over, so bubbletea reads the controlling terminal for itself; the probe now writes **no query at all** there, because a query would put its answer in a queue two readers share -- the arrangement the 1-in-20 above was measured in. The rows only a query can answer say why they are unknown, the colour depth still comes from the environment (which is not a query), and the probe's only remaining open of the controlling terminal is write-only: `TestTheProbeNeverReadsTheTerminalForItself` reads the probe's own source to keep it that way, because a host with no terminal cannot tell "did not ask" from "asked and nothing came back" by behaviour alone. The read machinery the probe used for queries of its own went with the path that used it, and the no-hang guard now runs through the reader.
- **The Utilities section kept its cursor by index while every other menu had been taught to keep it by label, so a background read moved the selection under the user.** The section's rows come from three reads that are armed when it opens -- the dotfiles theme definitions, the theme record, the WSL resources -- and each one inserts its row at the TOP of the list, above the rows the user could already see. The main menu, the restore list and the update row all had the same defect and all had the same fix (`selectedOption`/`holdCursorOn`, applied when the last-install record and the update answer arrive), and this section was the one place the rule had not been applied: the index was kept, so the row it named after the answer was a different job, and the next Enter opened a screen nobody aimed at. All three reads now keep the label and re-find it. `TestTheSectionHoldsItsCursorWhenABackgroundReadInsertsARow` measures the two reads that insert a row on an unlocked model -- the theme definitions, which are read on every host, and the WSL resources -- and is red against the tree before the fix for both.
- **A failed desktop theme switch was erased before it could be read.** The failure of a switch is written into the section's notice, and its own comment says a failure stays on the screen -- but stepping into the section cleared the notice first, and a real failure is the slow case: the switch is bounded by `themeCommandTimeout` (ten seconds), so the sequence that produced an unreadable failure was the ordinary one of pressing a row, stepping away before it answered and coming back to find out what happened. The way in no longer clears the notice, so the outcome of a switch that failed while the user was elsewhere survives the trip back; leaving the section still clears it, which is where the user has already read it. `TestAFailedThemeSwitchIsStillReadableWhenTheUserComesBack` walks exactly that sequence and is red against the tree before the fix.
- **The section denied the theme utility its own row offered.** The **Change the dotfiles theme** row is offered when there are definitions OR when there is a record to put back, and the record is the half that survives a machine where the definitions cannot be read -- but the body's default branch printed `The dotfiles' own theme is not switchable here.` in that state too, while the panel read the same state as an undo being available. The sentence is now worded from the row's own condition: what cannot be read is the list, so no theme can be applied, and the change the record names can still be undone. `TestTheSectionDoesNotDenyTheThemeItStillOffers` is red against the tree before the fix, and `TestUtilitiesSaysTheThemeIsUnavailableWithoutARepository` holds the state it was written for -- no record at all -- where the denial is the truth.
- **The Utilities row had no summary at all on a host with no desktop theme switch.** The one-line summary the narrow layout gives every row was blank for this row when no switch was detected, so the row that opens six utilities described itself as nothing while the rows beside it said what they do. It now names what the section holds on such a host, and still names the detected switch when there is one -- the half `TestTheUtilitiesSummaryNamesTheSectionWithoutADesktopSwitch` asserts beside it, because that is the behaviour the fix touches. The guard is red against the tree before the fix.
- **A comment claimed a theme change repaints the Windows Terminal colour scheme.** `windowsTerminalThemeDefinition` said the scheme is repainted by a theme change because Windows Terminal resolves it by name, which is only half the rule: the name does not change with the theme, which is the point, and the colours inside the scheme are written when the WSL terminal settings step runs. The Linux-side switch rewrites the files it owns and cannot reach `settings.json`, so the Windows side follows a later theme change the next time that step runs -- which `docs/tui-installer.md` already said, and now the code says it too, in the same terms and with the pointer to the front that owns the other half.
## [v0.9.0] — 2026-10-15

### Added

- **The installer shows every unit it is about to install, and each one can be turned off.** The wizard's last question led straight to the run, so the answer to "what is it going to do to this machine" was a step list that would only be complete once it was too late to disagree with it. A review screen now stands between the two: every unit of the run, in the order the run takes it, each one on or off, with the three the run cannot do without marked and refusing to change -- `clone`, the repository the run works from; `cleanup`, the state it created; and `setshell`, the default shell the user chose. `Space` turns the unit under the cursor off or back on, `Enter` starts the run and `Esc` returns to the question the screen was opened from, which is the backup question when there were configurations to ask about and the last question otherwise. The list is longer than the frame, so it is drawn as a window sized by the terminal -- the frame guard measured this screen at **28 rows on a 24-row terminal**, and the first fix, a fixed nine-row window, still overflowed the **60x20** case of the twelve sizes the matrix measures -- and the rows that do not fit are counted above and below rather than dropped. Nothing about the wizard's own questions changed: an untouched selection is exactly the plan the answers alone produced, which is what makes turning a unit off free of consequence rather than a second way of choosing.

  A unit that is turned off stays in the plan, is never executed, is not reported as a failure, and is named as skipped in the run's own reporting. That is what `StatusSkipped` was declared for and had never once been assigned: the plan panel already declined to count it as pending, the step list and the runner's list already gave it the finished marker, and the status line already had a label for it, so the state only had to be set and both runners taught to walk past it -- before the status becomes running, and before any credential is asked for on its behalf. The three fixed units are protected in the plan rather than only on the screen, so the rule holds for a selection the interface could not have produced.

  The command line can express the same selection with `--skip`, comma-separated, and it reaches the plan through the same builder, so the same units produce the same steps on either route. The run refuses a unit it needs and a name nothing installs, naming what was refused and listing what can be skipped, because an accepted token that quietly does nothing is worse than an error: the user believes they asked for it. `DOTFILES_SKIP_DEPS` and `DOTFILES_SKIP_TOOLSET` keep their effect exactly, and neither of them moves the plan: the guards pin the same selection, the same plan and the environment's own behaviour, and both routes are held together by one builder and one parity guard.

### Fixed

- **The password screen says what to type and what Enter does.** It was readable only to someone who already knew: an empty field drew the word `waiting`, which reads as the installer waiting for something rather than as the place to type, and the footer's only key was `[Enter] select` -- while this screen's Enter is not a selection at all, it is an attempt, so "select" said the opposite of what the key does. The body now names the action in one line (`Type your administrator password and press Enter.`), an empty field reads `type it here`, and the screen owns its own key pair: `[Enter] validate` and `[Esc] ask in the terminal`. Nothing else about the exchange changed -- the field, the count, the refusal line and the escape route are exactly as they were -- and the guard that holds it reads the rendered screen rather than the code that draws it, so a screen that says the words without showing them fails.
- **The runbook's Homebrew tap command could not update a hash, and said nothing about it.** The command that rewrites `Formula/dotfiles.rb` read the release's `SHA256SUMS` into a dictionary keyed by digest: `re.findall` yields the digest first and the asset name second, and the comprehension keyed them the other way round, so every substitution pattern was built from a digest where an asset name belongs and matched nothing. `re.sub` reports no error for a non-match, so the command's only effect was the version line -- a formula carrying the new version beside the previous release's four hashes, which is the one shape it must never have, because it installs the previous release's bytes under the new number. Measured on v0.8.0: zero matches per asset and no hash changed. The pairs are read the right way round now, and both substitutions count their replacements and raise instead of passing quietly when one does not happen. The verification at the end of the step is what caught it, and it is unchanged.

## [v0.8.0] — 2026-10-15

### Added

- **The installer's background drifts: a gradient drawn from the theme, behind every screen.** The interface was painted on whatever the terminal's own background is, so every screen sat on a flat field and the only colour in the frame was the text's. Each screen now paints a vertical gradient built from the theme's own `Background` and a lightly accent-tinted variant of it -- **20%** toward the accent, small on purpose, because the text has to stay exactly as legible on the ramp as it was on the flat field -- and the band scrolls **half a row per frame tick**, four rows a second, so the whole screen takes several seconds to cross and no single tick reads as a jump.

  It is painted in **one** place. `View()` already assembled every screen into a single string and ended in one tail; that tail now ends in one call, so the forty-odd cases of its switch, the utilities section and the trainer are all painted by the same function, a screen added later is painted without doing anything, and no screen can opt out -- the guard asserts it by parsing the package's own syntax tree rather than by reading text, and counts one definition so that a second paint point fails rather than passing as a copy. The drift is a pure function of the frame counter the frame tick already advances: the render reads no clock and no environment, the same rule the greeting follows and for the same reason, since a frame that moves while nothing else did cannot be pinned by a snapshot. The painter only ever inserts and removes escape sequences, so the visible text, its rows, their order and every row's width are exactly what they were -- and after every reset a row already carries, the background is re-armed, or a styled segment would punch a hole in the gradient.

  Three properties are held by tests rather than described. With the drift off the render does not move with the frame counter, and the package's own **227 `View()` comparisons across 20 test files** keep passing untouched -- 193 seconds of the whole package, the same four pre-existing Ghostty failures and nothing else -- which is what makes a drift-off run today's bytes rather than an approximation of them. With everything identical except the drift, the ANSI-stripped renders are identical row for row, so the background can never reflow, clip or shorten a screen. And every one of the **732** screen-by-size cases in the shared matrix differs with the drift on, the **60x20** floor included, so no screen is left on the flat field. Seven guards hold those rules, five of them red against the tree before the feature existed, and eight mutations prove their teeth: a second paint point (**S6 alone**), the field renamed (the name is the contract five rules need), a painter that yields when the frame is tight (**S1 alone**, naming the 5 of 732 screens it skipped -- the trainer's result screens, which are shorter than the threshold it invented), a frame tick that stops re-arming (**S2 alone**, on its second assertion), a phase that reads something outside the model (**S3 alone**), the early return removed (S1, S4, S7 **and the repository's own `TestWelcomeScreenGolden`**, which is the fleet half of the same rule), a space appended to each row (**S5 alone**), and a painter that asks the sprite's question. Each was restored byte-identical -- four hashes re-checked after every one -- and the guards were green on the tree that remained.

  The switch that stops it is the one that already existed. `--no-anim` / `DOTFILES_ANIM=0` stops the background along with the creature; a terminal that cannot show 24-bit colour never gets it at all, which is the same question the sprite asks before it draws, because a ramp that is approximated or dropped is worse than the theme's own background; and the answer is read once, when the model is built, never while drawing.

### Changed

- **The prompt symbol is a one-cell powerline triangle rather than the glyph that shipped before.** `starship.toml` drew `U+F15DE` (`nf-md-mustache`) in both the character block and the username format; both draw `U+E0B0` (`nf-pl-left_hard_divider`) now. A prompt symbol is not a matter of taste alone: it has to occupy exactly one cell, or every line the prompt sits on shifts by a fraction of one. Measured against the families this repository installs -- MesloLGS and IosevkaTerm, on the Windows side and on the WSL side -- `U+E0B0`'s advance is exactly **1.000 of a cell**, and in the same face it equals `U+F15DE`'s, so the prompt keeps the width it had and nothing below it moves. Every one of those faces draws it. That measurement is the verification rather than a test, because the fonts are put in place by the installer and are not shipped in this repository: a test here cannot read a `cmap` the repository does not hold.

- **The administrator password is asked inside the installer, on a screen of its own.** The run used to leave its own interface to ask: `sudoPrimer` handed the terminal to `sudo -v`, sudo wrote its prompt on the normal screen, and the interface came back afterwards. That handover is not a defect -- it is the only way `sudo -v` can run its own prompt -- but it is a visible hop out of the screen the user is watching, and the password should be asked on that screen. `sudo -S` reads the password from standard input and needs no terminal at all, which is what makes an in-screen prompt possible. The run asks once, before the first step that needs root, and only when it has to: `sudo -n -v` is tried first, so a machine where sudo is configured not to want a password -- common in containers -- is never asked for one. Three properties are pinned by tests that put a `sudo` of their own on `PATH`, so they measure where the password actually travels rather than what the code intends: it reaches sudo on its **standard input** and never in the child's arguments, where any account on the host could read it out of the process table; it is cleared as soon as sudo has read it, which is why the field is a `[]byte` of fixed capacity rather than a string -- `Update` returns the model by value, so the copies made on every keystroke share one backing array and clearing it once clears all of them, while a Go string is immutable and could not be cleared at all; and the screen draws how much has been typed and never a character of it. A refused password stays on the screen to be retyped, and **Esc** asks in the terminal instead: a system whose PAM wants a terminal -- a two-factor or fingerprint module, or a legacy `Defaults requiretty` -- cannot be authenticated through standard input, and that path staying reachable is the difference between a limitation and a dead end.

- **The Windows Terminal offer is a table of what changes, not a paragraph.** The offer shown mid-install in the WSL step is the screen that stands between the run and a write to `settings.json`, and it was seven sentences: where the file is, what a write would set, what happens to the previous one, the value the file already holds, and a closing line naming the keys that answer. That closing line restated the installing screen's own hint -- `[y/n] configure Windows Terminal or leave it unchanged` -- so it spent a row saying nothing new, and rows are what this panel does not have. It is drawn from the rows the installing body has left after its live meters, which at **60x20**, the tightest terminal the frame guards measure, is **eight content rows** while the paragraph needed more: at that size it cut off at `The previous file is copied beside it as…`, so the rows that fell off the bottom first were the ones naming **the value a write would replace**, which is the fact this screen exists to state. The body is a table now: the file, then one row per setting with the value the file holds today opposed to the value a write would set (`font.face  CascadiaMonoNF > MesloLGS Nerd Font`), the backup, and what happens to the rest of the file. A setting the file does not carry says `(unset)` instead of saying nothing, which it can state as a fact because an available offer proves the file was read and parsed. No body row is a sentence any more, and that is a rule the tests hold rather than a description: a paragraph can come back one clause at a time, so the guard rejects a row that ends in a full stop. The panel had **no test and no frame guard for its width at any size**, which is how it could truncate unmeasured; it is now rendered at every measured terminal, where every fact the choice turns on has to be readable in it and not merely narrower than the edge. The shared description is also what the `--non-interactive` notice prints, so the notice stopped printing sentences with it, and the label helper the old joined sentence needed was removed rather than left as the one thing still reading the old shape. The row budget is the whole budget, and CI found that out: the `macOS smoke test` job caught the table's last row falling out of the panel, because the path a test is handed there is **197 characters** where Linux's is 149 -- one wrapped row further -- so the backup and the promise about the rest of the file share a row now, and a guard measures the panel against the longest path that has to fit rather than against whatever the host happens to hand a test.

### Fixed

- **Fish has the two hooks that run on every directory change, and the three commands that keep secrets encrypted.** Stage 3 gave fish zsh's four shell commands; the hooks and the secrets were still zsh's alone, so a fish user entering a project got no virtual environment, no task list, and no way to read the repository's `secrets.env` from the shell. They are ported with the same policy: the environment search is bounded to the current directory and, **only inside a repository**, its ancestors up to the root, so a stray `.venv` in `/tmp` or in `$HOME` cannot capture every project underneath it; only an environment this hook activated is deactivated, tracked under the same `_DOTFILES_VENV`, so one the user activated by hand is never touched; moving around inside the project neither re-activates nor re-announces anything; the task hook reads `justfile`, `Makefile`, `package.json` and `Taskfile.yml` the way zsh does -- including the awk pass that lists `make` targets without executing the file -- and stays silent in `$HOME` and with `DOTFILES_NO_TASKS=1`; and `env-edit`, `env-load` and `env-export` keep the same defaults, the same readiness check, and the same refusal to write a plaintext destination git does not ignore.

  Fish needs one thing zsh does not, and the guard holds that difference instead of assuming the two are equal in everything: an environment is activated through `bin/activate.fish`, because fish cannot source the POSIX `bin/activate` at all, and an environment that has only got that one is left alone rather than half-activated. The primitives were measured, not assumed: a handler declared `--on-variable PWD` does not fire at startup, fires on every directory change, sees the **new** directory, and keeps what it sets with `set -gx` -- `chpwd` under another name. The hooks were run in a real fish: activation from an ancestor inside the repository, no re-activation while moving inside it, the environment left on the way out, the stray environment in a sibling directory **not** capturing the subdirectory, and an environment activated by hand surviving a directory change. The secrets were run against a real age key and a real encrypted file: `env-export` decrypts and prints `export ` lines, `env-load` refuses a destination git does not ignore and then writes the plaintext, a corrupt file fails **without leaving the destination behind** -- which matters because the redirect creates it first, so the removal is the difference between an empty file and no file, measured rather than assumed: `sops` exits **100** on a file it cannot read and the empty destination is already there -- and every refusal prints zsh's own sentence, the missing key included, with the command that creates it. Seven mutations hold the guard's teeth, each firing the guard written for it and nothing else: a hook that stops being a hook, a shorter candidate list, the POSIX activation script, the git refusal removed, a changed message, the readiness check removed from one command, and a task source dropped.
- **Fish can unpack and pack archives and serve a directory; it could not before.** `.zshrc` declares four commands `config.fish` never had: `cat`, which prints through `bat` and sends anything starting with a dash to the real `cat` so the flags keep working; `extract`, which maps thirteen patterns onto eleven tools; `compress`, which writes five archive formats and four single-file ones and refuses a single-file target given several paths or a directory; and `serve`, which validates a port as a whole number in 1-65535 and runs `python3 -m http.server` bound to `127.0.0.1`. Under fish the first archive typed at it answered `fish: Unknown command: extract`, so the two shells were the same shell only as far as the listing and the prompt, and everything had to be unpacked by hand. The four are ported with the same tool per format, the same flags and the same messages, and the guard compares the two shells rather than reading either one: for every `case` pattern in a shell it requires the other shell's branch for that same pattern to name the same tool and carry the same flags, so a format handled by `tar -xzf` under zsh and `tar -xf` under fish fails a test -- and so does the harder mistake, a branch moved onto another format's command (`.tar.bz2` running `.xz`'s `-xJf`), which a textual comparison of the two tables would have accepted because both tables would still have been present and well formed. The messages are compared as sets after shell variable names are normalised, so a mistake reads the same whichever shell is running.

  Fish needs a different spelling and nothing more, and that was measured rather than assumed: `case` takes several glob patterns where zsh separates them with `|`, a `|` inside a pattern is a literal, `$argv[2..-1]` is the rest of the arguments, `command -q` is the guard zsh spells `command -v`, and a command built in a variable runs like a typed one -- so zsh's own shape survives the translation: build the command in the table, ask once whether the tool resolves, run it once. The behaviour was run, not read: the four functions were executed in a real fish against real archives in a scratch directory, twenty checks, including the tool guard itself, which was exercised by taking `tar` off `PATH` and answering `extract: needs tar, which is not installed` instead of a bare command-not-found; a `.gz` written by `compress` passes `gzip -t`, and `extract` unpacks it back. Six mutations hold the guard's teeth -- a deleted function, a changed flag, a changed message, the tool check removed, the mismatched branch above, and a dropped `bat` flag -- and each fails the guard written for it and nothing else, then was restored byte-identical. The port is an insertion and only an insertion: **213 lines added to `config.fish` and no line removed**, one hunk, placed after the theme block's closing sentinel and before the drop-in loader, so the generated block, stage 1's aliases and the loader are exactly where they were.
- **Fish lists files the way zsh does, icons included.** The two shells are meant to be the same shell with the same tools, and they were not. `.zshrc` aliases `ls` to `eza --icons --group-directories-first` with `ll`, `la`, `lt` and `tree` beside it; `config.fish` aliased `ls` to `gls --color=auto` and added nothing. A fish user therefore got a listing with no icon anywhere, which cannot be told apart from a Nerd Font that was installed wrong -- and it was reported as that. `eza` was already on the fish `PATH` on every host this installer supports, so fish had the tool and never called it. The listing block now declares the same five aliases with the same flags, which a guard compares against `.zshrc` so that a flag changed in one shell and not the other fails a test instead of shipping one flag at a time, and it exports `EZA_ICON_SPACING` so an icon is not jammed against the name it labels. Every alias is declared inside a `command -q eza` guard, because an alias whose tool the host has not got fails on every call and the Homebrew step that puts `eza` in place is allowed to fail. Without `eza`, macOS keeps the `ls --color=auto` it already had and a host with Homebrew's coreutils keeps `gls --color=auto` -- itself guarded now, for the same reason: `gls` is a Homebrew-only name, so that alias failed on every call on a Debian host without Homebrew.
- **Fish's listing is painted by the theme now, not by eza's own defaults.** The aliases above gave fish the shape of zsh's listing and left the colour behind: `LS_COLORS` -- which `eza` reads as its base layer -- and `EZA_COLORS`, which paints its metadata, were exported in `.zshrc` and nowhere else, so a file zsh drew blue was drawn in whatever colour eza ships with, and the two listings could be told apart at a glance. The two tables are structure rather than palette data: which extension or key takes which role is settled once, and every value in them is one of the `PALETTE_*_SGR` variables, so fish's block renders them from the same mapping the zsh block reads instead of restating it. What the shells disagree about is only how a variable is spoken -- zsh writes a braced reference and exports with `export`, fish writes a bare one and exports with `set -gx` -- and a guard renders both blocks and compares the two tables entry for entry, **67 entries of `LS_COLORS` and 66 of `EZA_COLORS`**, so a key added for one shell cannot leave the other behind. What fish is handed is fish, which was measured rather than assumed: a bare reference expands inside a double-quoted value, a backslash at the end of a line continues that value without keeping the newline, and `set -gx` exports the result, so eza reads the table out of the environment exactly as it does under zsh. The space before the value is not cosmetic -- fish's `set` refuses a joined `NAME=value` with *invalid variable name*, which is what a literal translation of zsh's `export` line would have shipped. The zsh block is untouched, measured the same way: it was rendered for three themes before and after the change and compared byte for byte, because the palette both shells read is now computed once rather than in each renderer. The shipped `config.fish` is regenerated through the artifact updater rather than edited, and that is how this change was caught while it was being made: `TestGeneratedThemeArtifactsMatchTheirDefinition` failed naming the file, so a fresh install cannot start on a block the renderer no longer produces.
- **The pinned Nerd Fonts archive predates `dev-yaml`, so `.yaml` files were blank on Linux.** The installer downloads a pinned Nerd Fonts release for Linux, WSL and Termux, and it pinned **v3.3.0** while the macOS cask installs **v3.5.1** -- two releases of the same tree, when the comment beside the URL already said they were meant to be one. `eza` draws a `.yaml` file with Nerd Fonts' `dev-yaml` (`U+E8EB`), and that codepoint is **absent from v3.3.0's own `glyphnames.json`** and present in v3.5.1's, so every `.yaml` in a listing came out as a blank box on Linux, on WSL and on Termux while macOS, which had the later archive all along, drew it correctly. A machine that only ever runs the macOS route cannot see this at all. The pin moves to **v3.5.1**, and the two facts the font step rests on were re-measured at the new tag rather than assumed to carry over: the faces still declare `MesloLGS Nerd Font`, which is both the family `fc-list` is asked about and the one the post-extraction verification reads back, so the presence check and the verification keep working; and the archive's declared size moves from 108,281,715 to **116,387,026 bytes**. The version is a floor rather than a preference -- `U+F086F` and `U+F15DE` are present at both tags and `U+E8EB` only at the later one -- and that is written beside the URL now, because lowering it restores the blank box silently.

## [v0.7.11] — 2026-10-14

### Changed

- **The Herdr plugin step installs its declared set every run and updates it where the CLI can.** `herdr plugin update` exists from Herdr **0.9.4**, and the step reads the machine's own version before it runs, so three cases still **succeed** because they are not failures: a version that cannot be read, an answer carrying no version, and a version older than 0.9.4 -- the last naming the version it found and saying the plugins were installed but not updated, rather than failing a run on a machine that is merely old. Only an update that is **attempted** and fails fails the step.
- **The version this installer falls back to when Herdr is absent moves from v0.9.1 to v0.9.5**, with both Linux checksums taken from the release assets and verified by downloading and hashing them. The old pin predated `herdr plugin update` entirely, so that download could never update a plugin.
- **Four documents claimed Herdr has no plugin update command.** That was true for 0.9.3 and false from 0.9.4, so it misled a reader: `docs/tui-installer.md`, `README.md`, `README.es.md`, `docs/tools.md` and the installer's own comment now say what the CLI actually does.

## [v0.7.10] — 2026-10-14

### Added

- **`dotfiles --verify` reports whether the machine is still as the last install left it.** Every run records what it did to each configuration path it touched -- the path, the action it took, and the backup it made when there was one -- and nothing read that record. The command reports one line per entry and exits **0** when the machine matches the record, **1** when something it names is gone, and **2** when the record could not be read. It names its own limit **on every line**: the record holds no digest, so it establishes that a path is there, what the run did to it, and whether the backup still exists -- and says it cannot tell whether a file's contents are still the ones the install left. The distinction between absent and present-but-unverifiable is the point of the report, so it is per line rather than in a footnote. A machine with **no record is not a failure**: it says that nothing was installed here, which is what a verification run before the first install should say. The manual gains a section saying the same.

## [v0.7.9] — 2026-10-13

### Fixed

- **The font screen says what each option does, and what it found on this machine.** The row that answered "No, I already have it" was a **reason** where an **action** belongs: it did not say whether choosing it skips the font, fails the step, or continues. The rows are verbs now -- install, or skip -- and the screen states the measurement the installer already makes before downloading anything, so the choice rests on what is on the machine rather than on what the user remembers. The measurement is the step's own check rather than a second one, it runs once when the screen is built rather than on every redraw, and on macOS, where the step installs through Homebrew's cask and checks nothing, the screen says nothing about presence instead of inventing an answer.


### Fixed

- **Installing asks for the password once, and the installer stays on screen.** Four complaints from using it on Linux -- a password prompt for every step, having to press enter to continue, and the installer leaving the screen for the terminal while it worked -- had one cause: it handed the terminal to a shell script whenever a step needed `sudo`, because `sudo` wants a terminal to ask its question on. Its own comment said the consequence, *"This suspends the TUI and gives full terminal control to the process"*, and that single decision produced all of them. The credential is requested once now, before the first step that needs it, and kept warm while the install runs; every step script runs `sudo -n`, so a step without the credential fails visibly instead of prompting somewhere the user cannot see. With the credential cached a script needs no terminal, so the scripts run through the executor that already captured their output line by line and put it in the activity log -- the machinery every other step already used. No step hands the terminal over any more; the one remaining handover is the credential request itself, which is where `sudo` reads the password from. And the pauses are gone: eight occurrences of a sentence this program printed itself, one of its own constants, and a guard that now refuses to let one back into a step that no longer needs it.

## [v0.7.8] — 2026-10-13

### Fixed

- **An install no longer discards the theme the machine has applied.** Every step that writes a
  configuration file writes the file this repository ships, and those carry the **default** theme's
  generated block. The picker was the only caller of `applyDotfilesTheme`, and the record of which
  theme is applied was read only to be displayed, so a run silently reverted an applied theme for every
  file it rewrote while the record went on naming it, and nothing reported it: the picker's own count
  stayed true of a machine that no longer matched it.

  Measured on the machine this was found on: the picker wrote Nocturne into six files at `15:31:19`,
  an install ran at `16:59:42`, and afterwards `~/.zshrc`, `~/.p10k.zsh` and
  `~/.config/herdr/config.toml` carried the default theme's block and a `16:59:42` timestamp, while
  `~/.config/starship.toml` and `~/.config/nushell/config.nu` still carried Nocturne's and their
  `15:31:19` one. `theme.json` went on naming Nocturne over six files. The reset was per step rather
  than uniform, and the prompt — which reads `PALETTE_*` from the first two — rendered the default
  palette on a machine whose own interface reported Nocturne as applied: measured in a real
  interactive shell, `POWERLEVEL9K_OS_ICON_FOREGROUND=#7fb4ca`, `POWERLEVEL9K_FOREGROUND=#f3f6f9` and
  `POWERLEVEL9K_PROMPT_CHAR_OK_VIINS_FOREGROUND=#b7cc85`, every one of them `themes/dotfiles.toml`'s
  value rather than Nocturne's `#8686cb`, `#c0c0c0` and `#86cb86`. The same configuration looked right
  on another machine because the theme there had been applied **after** the last install; the order was
  the whole difference.

  A new **Restore the Applied Theme** step runs after every step that writes a configuration file and
  before `Cleanup`, which removes the temporary checkout the theme definitions are read from. It
  re-renders each artifact's block from the definition the record names, through the picker's own gates
  — the artifact list, the path each artifact answers with, the ownership proof, the Neovim
  reachability rule and the same renderer — so a file the picker would refuse is refused here for the
  same reason. Unlike the picker it never adopts a file that carries no marker of this project, and it
  leaves the theme record and its undo snapshot exactly as the picker wrote them. No record, a record
  that cannot be read, and a record naming a theme the checkout does not define all leave every file
  alone and none of them fails the run: the last is a normal race between the installed binary and the
  branch it reads, and failing there would report an install as failed after it had already installed
  everything.

## [v0.7.7] — 2026-10-13

### Fixed

- **The theme switch now writes the `bat` selection it always declared, and the prompt reload it names
  applies the theme.** Two silent gaps in the switch's reach, found by measuring one machine after
  Nocturne was applied and reported by neither the switch nor its screen.

  The switch declares fourteen artifacts and opens each iteration by asking where the artifact's file
  is, skipping it when the answer is empty — so an artifact it has no path for is declared, rendered
  and covered by tests while never being written, and a skip is not a failure. `bat` was that
  artifact: it shares `dotfiles-zsh/.zshrc` with the zsh artifact, further down the same file, and
  `themeInstalledPath` had no case for it. `BAT_THEME` therefore held the theme this repository ships
  for every theme on every machine, so `bat` — and `cat`, which the shipped `.zshrc` aliases to it —
  rendered the default palette while the rest of the machine rendered the applied one, with every
  generated `.tmTheme` present and nothing selecting any of them. The screen's own numbers showed it
  before any code was read: it reported `7 file(s) were themed and 2 file(s) were left exactly as
  they are` on a machine where fourteen artifacts minus the two files this repository does not own
  and the four absent tools leaves eight. Applying Nocturne to a copy of the machine's `.zshrc` left
  the bat block's tag and `BAT_THEME` on `dotfiles`, and counted one themed file where the same file
  holds two artifacts. A guard now fails unless every declared artifact resolves to a path.

  The second gap was in the instruction the screen prints. It names `p10k reload` for the prompt, and
  that re-sources `~/.p10k.zsh`; the generated palette block there **defaulted** each `PALETTE_*`
  (`${PALETTE_X:-"…"}`) while the shipped `.zshrc` sets the same names unconditionally and is
  sourced first. In a shell that was already open when the theme changed, `PALETTE_*` still held the
  previous theme, so the reloaded file kept exactly those values: measured in zsh with the previous
  theme's blue live in the shell, `POWERLEVEL9K_DIR_FOREGROUND` came out `#7fb4ca` where Nocturne's
  blue is `#8686cb`, and `POWERLEVEL9K_FOREGROUND` `#f3f6f9` where Nocturne's text is `#c0c0c0`. The
  one in-place action the installer offers for the prompt was the action that could not work. The
  block assigns now, which sets the file's values whether or not the names are defined — so the file
  still stands on its own without `.zshrc`, the property the fallbacks were written for, without the
  ability to lose to a value already in the environment. A guard asserts this against a real zsh: it
  sources the generated block with the previous theme's values set and fails unless the applied
  theme's values are what is left.

## [v0.7.6] — 2026-10-13

### Fixed

- **The Windows Terminal colour scheme is built from the theme the machine has applied, not from the default one.** `windowsTerminalThemeDefinition` loaded `dotfiles` unconditionally, so the scheme, the offer and the read-back all described the default palette no matter which theme the switch had applied. The machine this was measured on had `nocturne` applied -- recorded in the installer's own theme state, and written into `.zshrc`, `.p10k.zsh` and `starship.toml` -- while the `dotfiles` scheme in `settings.json` held `background #06080f`, `red #cb7c94`, `blue #7fb4ca`, to the digit the default theme's values. The palette now comes from the applied-theme record, with the default theme kept for a machine that has applied nothing, a record that cannot be read, and a record naming a theme the repository no longer defines -- the last of these falls back rather than failing, because an error in that function abandons the whole Windows side of the step, font face included. The scheme's name does not change: it is the one `dotfiles` scheme and a theme change repaints it.

## [v0.7.5] — 2026-10-13

### Fixed

- **The Windows per-user font registration no longer uninstalls the fonts it did not install, and the face it registers can now be resolved.** The registration program ran `New-Item -Path $key -Force` over `HKCU\Software\Microsoft\Windows NT\CurrentVersion\Fonts` before writing its four values, and on a key that already exists PowerShell's `New-Item -Force` recreates it and discards everything it held. That key is the machine's per-user font list, so every per-user font this installer did not put there lost its registration: the run this was measured on destroyed 36 of them, the whole CaskaydiaCove Nerd Font family, leaving the `.ttf` files in the store with nothing to resolve them by. The key is created only when `Test-Path` says it is missing. Separately, the value written was the file's base name, and Windows resolves a per-user face through the absolute path in the value -- a bare name is looked for in `%WINDIR%\Fonts`, where these faces are not. The family therefore never resolved, the verification that gates the face write refused correctly on every run, and Windows Terminal was never told the font, which is what a user sees as the installer doing nothing. The value is the absolute path now, built on the Windows side from `%LOCALAPPDATA%` with `Join-Path`.

## [v0.7.4] — 2026-10-13

### Fixed

- **Every terminal this installer configures now names a font that exists.** The installer wrote four shipped configurations naming `MesloLGS NF`, and no face in any artifact it fetches declares that family: the pinned Linux and Termux archive, the macOS cask and the Termux face all declare `MesloLGS Nerd Font`. On a fresh Linux or macOS install every terminal fell back to its default font -- silently, with no icons and the wrong letterforms. There is one family constant now, read from the face the archive ships, and the check, the verification, the Windows registration name, the four configurations, the interface label and both guards all derive from it.
- **The Linux font step no longer downloads 108 MB on every run, and no longer fails on a machine that has the font.** The presence check asked fontconfig for a family the archive does not declare, so it never resolved and the archive was refetched every time -- the defect #276 was opened to close, with a guard that could not fire. The verification after extraction asked the same wrong question, so the step reported an error on a machine where the font was installed correctly.
- **Termux installs the font the interface names.** It installed JetBrains Mono while the wizard row, the menu row, the requirement line and the summary all said Meslo, and the decision recorded in #265 is that the font is Meslo for every terminal. The URL it used had itself moved and was a 404, so that step would have failed the download outright.
- **The Windows account is asked about rather than guessed.** `defaultuser0` was not in the list of Windows system accounts, and it exists on machines; a single surviving directory was returned as the answer without being checked against anyone. The profile now comes from `%USERPROFILE%` through the Windows interop, gated on the directory existing, with the mount as a fallback that refuses rather than guesses.
- **The Windows Terminal write is reported from the file, not from the bytes that were sent.** A landing that came out partial could print a success over a configuration Windows Terminal would not read as sent. The write is read back, each setting is reported separately, and a partial landing is a shortfall that withholds the checkmark and names what did not land.


### Fixed

- **The dev channel serves the highest version, not the newest by date.** GitHub orders its releases list by creation date rather than by version, and the channel took the first pre-release in that list, so a pre-release cut after a higher one -- from a hotfix branch, or a re-cut of an older line -- was served over the newer version. It compares versions now, and it also skips a tag that is not a version at all, which it used to serve: a list holding a tag like `nightly` would have been offered as a release to install. Both were declared limits, written in the code, the README and this file, and all three said the channel took the publish order; they now say what the code does.


## [v0.7.3] — 2026-10-12

### Fixed

- **The pet moves within a second, and a key no longer stops its life.** Reported after v0.7.0 made it animate: it took a long time to start walking. Two causes, both measured. The first stroll waited two seconds of quiet before the wait even began, and a further zero to two seconds spread on top, so the first cell could take four seconds to appear. And a keypress reset the idle counter to zero, so a person reading the menu and pressing arrows or Enter held it at zero forever -- the pet never strolled for the user who reported it. The first stretch is half a second now and the gap between choices is zero to one, so the first cell lands around three quarters of a second.
- **The keypress reset was split rather than deleted.** What it was protecting -- the user's reading and navigation -- is not protected by it: the creature owns reserved rows, never reflows the layout and never touches the cursor, so its stroll cannot collide with either. What is kept is the sleep half: a key wakes a sleeping or yawning pet, restarts the sixty second sleep clock, and hops once so the wake is visible. The counter is clipped to the first tick past the roam threshold instead of cleared, so the sleep clock restarts while the pet keeps moving. One cancel survives because it is real: a resize that lands while a stroll is in progress. The initial size Bubble Tea sends at startup is excluded, or the first stroll would wait out the calm interval.

## [v0.7.2] — 2026-10-12

### Added

- **The installer installs the Herdr plugins.** It provisioned plugins for tmux, zsh, fish and Neovim and left Herdr's alone, and the two plugins this repository now declares -- `aigora.lantern` and `herdr.auto-title` -- are installed with `herdr plugin install <owner>/<repo> --yes` and verified by reading `herdr plugin list` back. No revision is pinned: the point is the latest, and the commit Herdr records is read and reported rather than chosen here. `--yes` is not optional, because without it the command opens a trust preview that nothing in a step can answer. A plugin that failed to install, or whose build failed, is a failed step rather than a success, and a plugin you added yourself is never touched.
- **The renderer offer reaches a screen.** Under WSL the installer could install the font, register it, verify it and write Windows Terminal's settings -- but only under `--non-interactive`. The interactive path, which is what a person using the TUI takes, printed the notice and never showed the offer. The installing screen now shows it, including the value that is there now when one is: a machine already configured by another tool has a font face set, and an offer that does not name it is how a user loses a setting they chose.

### Fixed

- **One unowned file no longer leaves every other file on the old palette.** The theme switch returned at the first artifact whose ownership it could not prove, after putting back everything it had already written, and alacritty is the first entry: on a machine carrying a legacy `alacritty.toml` the run stopped before kitty, ghostty, herdr, starship, zsh, p10k, nvim, bat, fish, tmux and nushell were reached, including the files that are marked and were perfectly themeable. The run now collects every file it cannot prove it owns in one pass, themes what it can, and reports a partial result naming each skipped file and its reason. A skipped file's bytes are untouched and the undo record holds only the files written. The refusal also stopped naming an action the interface does not offer.
- **The update row is never silent about what it knows.** A machine reported it exactly: on the stable channel `--check-update` exits 1 and the main menu shows nothing, because the row was added only when the update was installable from here. A newer release the installer could not install -- a binary Homebrew owns, or a target it could not resolve -- left no row at all, and a hidden row reads as "nothing new". The row now appears whenever the check has something to report and names the reason in the words the update screen already uses; pressing it opens that screen rather than attempting a swap. The write path is untouched: a binary a package manager owns is still never replaced, and the row reports rather than tries.
- **The font step asks whether the family is already installed, and verifies what landed.** It installed the font once per run, every run: Linux always downloaded the pinned 108 MB archive to overwrite identical bytes, and then reported "Font installed" whether a family registered or not. A broken extraction, a fontconfig cache that did not pick the directory up, or a user font directory the terminal cannot read was reported as success. The step now asks before it downloads and verifies after it installs, and Termux gets the same treatment.
- **The sound the managed config turns on gets its dependency.** The Herdr configuration this repository installs enables `[ui.sound]`, and Herdr ships no audio decoder: it shells out to an external mp3 player and fails silently when none resolves. The step now resolves a player before it reports success, provisions `mpg123` when none is present, and names the gap and withholds the success line when even that cannot be provided.

### Changed

- **The documentation and the code cannot disagree about the Herdr plugins.** A guard derives from source whether the installer invokes the plugin CLI and requires each document to carry the matching stance, so a provisioning step added without the documentation being updated fails it. It covers the Spanish README too: the other language drifting is a defect this repository has already had once.

## [v0.7.1] — 2026-10-11

### Fixed

- **The pet moves at a speed a person can see.** The report was exact: it did not look like it was moving on its own. The idle held one frame for two seconds, the creature did not start walking until six to fourteen seconds of quiet, and a play bout came every twenty-two, so ten seconds of watching showed one stroll. The idle now steps every quarter second, a stroll starts after two seconds, and a play bout comes every ten. Printed at successive ticks, the idle draws four distinct images in twelve ticks, the walk alternates its legs, and the play bout puts a ball at the creature's side and bats at it; before this, the same twelve ticks drew one image twice over.
- **The screens carry a frame.** The main menu was a plain list with a rule above it, and the theme reached the content and stopped at the border. Borders, titles, the selected row and the step indicators now take the palette the user chose, so the whole surface reads as one design. The truecolor path is captured separately for the first time: until now every golden was taken on the adaptive chrome, so the themed frames a user with a truecolor terminal actually sees had no golden at all.
- **Under WSL the installer configures the renderer instead of only naming it.** Issue #236 added the notice and stopped there: the code that would act on it was called only from a test, so nothing put a font where Windows could read it and nothing could write the settings Windows Terminal renders with. The step now copies the faces where Windows resolves them, registers the family per-user, verifies it in the font store, and only then writes the font face and the colour scheme into the reachable `settings.json`. A face named without that verification produces a silent fallback, which is the defect the notice was about.
- **The font is Meslo, and the family is resolved before it is named.** The installer installed IosevkaTerm and named it everywhere, and three of the four shipped terminal configurations named the Nerd Fonts v2 spelling of a family the v3 archive does not provide, so those terminals fell back silently. The font is now Meslo, whose naming does not follow IosevkaTerm's: the archive was read rather than assumed. The Windows face is resolved at install time from the font store, so a machine that carries its own patched family gets that one, and the public family is installed and registered only when nothing resolves.
- **The Homebrew formula parses again, so the tap installs.** It had not parsed since the 0.6.4 tap commit: the four `end` tokens closing the on_arm/on_intel pairs had been glued to the end of the `sha256` lines, and the script that rewrote those lines deleted them. `brew install` and `brew upgrade` failed before resolving anything for three releases. The formula is restored and two guards hold it: one asks Ruby to compile the file, which is what brew does before it resolves anything, and one rejects the shape that caused the defect, because the next edit that rewrites such a line will delete its terminator again.

### Changed

- **A guard that fails on correct input is worse than no guard.** The first attempt at the formula guard counted blocks with a regular expression and failed on the correct file, because `elsif` needs no `end` of its own. It asks the interpreter now. The same review found that one of the font guards could pass without reading any file at all, which is the defect it existed to prevent.

## [v0.7.0] — 2026-10-11

### Added

- **The companion plays, hops, and notices what the installer is doing.** It already wandered on its own; it barely moved while it did. Every state now has a real sequence instead of one glyph that shifts position -- a breath that moves the head a column and holds it, a three-step cheer, a play bout with paws batting and a tail swishing, and a hop with a take-off and a landing. Five states are earned by the installer rather than by a timer: watching a step that is running, charmed by a theme just chosen, curious about a published release this build does not have, mid-hop, and playing a game it starts on its own after a quiet stretch. None of them claims a step finished: the only path to the pleased face is the model's celebration counter, armed when a step really completes. The cadences are written down in the code, because a motion that changed every tick would repaint the creature's rows eight times a second over a screen nobody is touching.
- **The theme reaches the frame.** The chrome around the content was close to plain: the theme reached the content and stopped at the border. Borders, titles, the selected row and the step indicators now take the palette the user chose. And the truecolor path is captured separately for the first time -- until now every golden was taken on the adaptive chrome, so the themed frames a user with a truecolor terminal actually sees had no golden at all; three screens have one now.
- **The Neovim configuration syncs the clipboard over SSH.** Copy and paste worked in WSL and never over SSH, because nothing bridged to OSC 52. `vim.opt.clipboard` now lives in `options.lua`, which installs an OSC 52 clipboard when `SSH_CONNECTION` is set. A guard loads the file in headless Neovim with and without `SSH_CONNECTION` and asserts both directions; when nvim is missing it exits non-zero with a skip message rather than passing without checking anything.

### Fixed

- **Pre-releases are ordered correctly, and an unknown answer says why.** Version parsing dropped everything after `-`, so `v0.7.0-rc9` and `v0.7.0-rc10` parsed identically: a build on rc9 was told it was up to date with rc10, and a final `v0.7.0` was invisible over its own `v0.7.0-rcN`. Parsing is semver-aware now -- `rc9 < rc10`, a pre-release is behind its final, `dev.1 < dev.2 < dev.10`, build metadata ignored -- keeping the two rules that were already right: a source build is behind every release, and an unparseable candidate is never newer.
- **A tag that cannot be parsed no longer reads as "up to date".** It is unknown, with a reason, and `--self-update` refuses rather than installing from a tag it cannot parse. Failures are classified instead of stored verbatim: no network, rate limit, nothing published, or another status. A dev answer now names the pre-release it found, when it was published, and that it is a pre-release -- a dev answer that reads like a stable one is how a user installs something they did not choose.
- **The upstream mirror stays current, and says so when it cannot.** The tracker already compared a persisted HEAD and deduplicated by commit, so commits past the last tag were never invisible; what was wrong is that our mirror branch sat at a July commit while upstream's `main` moved two commits ahead of its last tag, and nothing said so. The mirror is advanced from the workflow now, and a failed fetch or push is reported as a comment instead of staying in a log.

### Changed

- **Declared limits are stated where they belong.** The dev channel does not verify that a pre-release came from `main`, and it takes the publish order rather than the highest version, so an older pre-release published later would be served. Both are written in the docs and in the code, and the README no longer claims ancestry the code does not check.

## [v0.6.5] — 2026-10-11

### Fixed

- **The install record says what the run did, not what it found.** It was built from the list of configs the run found on disk, so a config merely detected was recorded exactly like one the run overwrote, and a reader could not tell what had changed. Each entry now carries an action -- found, backed-up, replaced or created -- and an entry claiming a replacement carries the backup that proves it. An earlier version's record still loads (#239).
- **Under WSL, the installer no longer reports success where nothing renders.** The Nerd Font went to `~/.local/share/fonts` and the palette into the Linux terminals' configurations, but the program that draws the session is Windows Terminal, on the Windows side, which reads neither. The step now states where both landed and that Windows Terminal is the renderer before reporting anything as done, and then offers -- behind one explicit choice -- to write the font face and the colour scheme into the `settings.json` reachable from the mount. The offer keeps its promises in code: it takes a backup before writing and aborts if the backup fails, changes only the keys it owns while preserving comments and every other key, is idempotent, and refuses what it cannot edit safely instead of rewriting it. Outside WSL it touches nothing on the Windows side (#236).

### Changed

- **The release names the commit a tag points at, not the tag object.** `GITHUB_SHA` is the commit for a lightweight tag and the tag object for an annotated one, so a binary stamped from it could name a tag's id and be impossible to match to the commit the release built. The commit is now resolved once, in the job that has a checkout and produces the binaries, and read by every other job from an output it already needed; peeling falls back to the raw value when the tag object is absent, because naming a tag is worse than naming the commit but inventing one is worse still (#246).

## [v0.6.4] — 2026-10-10

### Fixed

- **nushell takes the theme now, and the coverage table can say so.** The shell's own
  `dotfiles-nushell/config.nu` shipped eighteen hand-written Kanagawa colours that appear in no
  `themes/*.toml` - the `$dark_theme` record the configuration selects with `color_config`, the
  `$env.LS_COLORS` table it paints `ls` output from, and two hexes in the `explore` block - and nushell
  was not one of the tools `themeTools` named. Choosing any theme but Kanagawa therefore repainted
  every other tool while `nu` kept the old palette, and the coverage row, which is read from the same
  list, said "nothing" was left out and was silent about the one shell it never painted (#240).
  Nushell is the thirteenth tool now and both of its colour regions are generated from the theme's own
  palette by the mapping recorded in `themes/README.md`, so no colour is invented; the two `explore`
  hexes take nushell's own ANSI names, which the terminal paints from whichever theme is applied. The
  switch rewrites only the bytes between the region's two anchors - a `config.nu` that carries the
  user's own PATH lines keeps them byte for byte - and records the original so **Undo** puts the file
  back exactly. `TestEveryOfferedThemePaintsEveryTool` now covers the shell (all seven themes paint it),
  `TestNushellIsPaintedByTheThemeSwitch` decodes every `LS_COLORS` triple and refuses any of the
  eighteen values, and `TestTheThemeRowCountsNushell` holds the row to the same truth it tells.

## [v0.6.3] — 2026-10-09

This is the release where the installer learns to apply the themes it offers. The library grows to seven
complete themes, every one of them painting all twelve tools the switch names, and applying one now reloads the
tools that show it and says what is left instead of reporting "applied" and stopping. The installer also learns
which release is published and installs it over itself, the companion stops following the cursor and wanders on
its own, and the ten interface defects an inventory found are fixed in one pass, together with the class that had
hidden three of them behind unreachable code.

### Added

- **Seven complete themes, every one painting every tool the switch names.** `dotfiles`, `Catppuccin Mocha`,
  `Catppuccin Latte`, `Rosé Pine`, `Everforest`, `Kanagawa` and `Nocturne` each paint the twelve tools - the four
  terminals (Alacritty, Kitty, WezTerm, Ghostty), Starship, the zsh line editor, the p10k prompt, Herdr, fish,
  bat, Neovim and tmux - and the picker's "leaves out" column reads `nada` for all of them (#167, #171, #183).
  Each theme has one definition in `themes/*.toml` and every artifact is generated from it, so a theme is offered
  only when it can be applied **and** shown (#159, #160). No colour is invented: a value is either what the
  repository already ships or a transcription of a published palette, and the roles a tool derives are named per
  tool in `themes/README.md` (#165, #171). `TestEveryOfferedThemePaintsEveryTool` reads the same coverage the menu
  reads and fails by naming the theme and the tool it left out; the failure that named `nocturne` and `Neovim` is
  the guard's own teeth (#171, #190).
- **A seventh theme, measured rather than transcribed.** `nocturne` follows a screenshot of the user's own
  terminal: the hues the screenshot carries were measured from its pixels, the ones it does not were derived from
  the measured set by one documented mapping, and `[syntax]` is dark-only because the reference has no light
  version. Its generated Neovim colorscheme and bat theme come from the repository's generator, and the definition
  carries its provenance (#190).
- **Applying a theme now reloads the tools that show it, and says what is left.** Writing the file is not
  applying the theme. The switch runs only the safe, idempotent reload commands - `bat cache --build` with
  `BAT_CONFIG_DIR`, `tmux source-file` only after `tmux list-sessions` succeeds, `kitty @ load-config` only when
  `KITTY_LISTEN_ON` is set, and `herdr server reload-config` only when `HERDR_ENV=1` - each bounded by a
  five-second timeout, and the result screen gives one row per tool: `✓` needs nothing more, `→` names the exact
  action a person still has to take (#194, #200). Herdr's reload is the command behind its own key, measured on
  herdr 0.9.3 (about 40 ms, exit 0, and `server_not_running` with exit 1 against a socket with no server), because
  a signal is not a key and the alternative was signalling the process the installer runs inside (#200).
- **A theme picker one level in.** Utilities offers `Change the dotfiles theme`, which opens the list of complete
  themes, each naming the tools it leaves out, followed by the undo row and the way back; the live preview moved
  with the list and is reached only from it (#164). The list windows around the cursor through the repository's
  existing `listWindow`, and the header names the visible range, so the frame - not the palette - no longer caps
  how many themes can be offered (#167).
- **The installer knows which release is published, and installs it over itself.** `--check-update` reads the
  latest release (exit 0 current, 1 newer, 2 unknown) and `--self-update` replaces the binary through the order
  the repository already used for OfficeCLI: stage beside the destination, verify the asset against the release's
  `SHA256SUMS`, make it executable, `rename`, keeping the replaced binary as `<path>.previous`. A failed check is
  recorded as unknown, never as current, and a binary Homebrew owns is refused with `brew upgrade dotfiles` named
  (#216). The main menu offers the update as a row once a newer release is installable, and the automatic check
  now runs on every real open rather than only behind the drawing gate; the row arrives after the first frame, so
  the cursor is held by the label it named (#222). The release workflow verifies its own chain twice - `verify` on
  the artifacts it built and `verify-published` on the assets downloaded back - and a manual `workflow_dispatch`
  (defaulting to `publish: false`) exercises the same job without publishing (#216).
- **Three more utilities, and the definitions they need outside a checkout.** WSL resources adjusts memory,
  processors and swap from the menu with the host's numbers and the recommendation on screen, sharing the
  installer's own calculation and writer rather than growing a second one (#172); the shell startup audit starts
  the login shell named by `$SHELL` the way a terminal does and reports the median of five bounded starts with
  zsh's own `zprof` attribution, where a start that does not finish is killed, counted and named as a timeout
  (#179); the terminal capability report answers what the terminal can do, with `unknown` as its own state
  carrying a reason and a manual check (#181). The clone step copies the theme definitions and the WSL template
  into `$XDG_DATA_HOME/dotfiles` (`~/.local/share/dotfiles`), which the resolvers read last, so the utilities are
  offered from any working directory and not only from inside a checkout (#192, #196).
- **Two degrees of adopting a file an older version installed.** A whole-file file is adopted as before, by the
  ownership marker or byte-identity to the shipped file; a file that carries our anchors but has drifted outside
  them is now adopted by region, rewriting only the bytes between the anchors, leaving every byte outside them
  untouched and recording the original so Undo restores it byte for byte. When neither degree holds, the refusal
  names the file, every probe that was made and the way forward (#173). Files that predate the marker can also be
  refreshed in place and reversibly: the picker names exactly which files it will touch and where each user-owned
  file will be preserved before it writes anything, and the cursor starts on Cancel (#169).
- **Goldens for the five utility screens that had none, and a guard that a golden renders and never installs.**
  The theme picker in three states, Utilities, WSL resources, the shell startup audit and the terminal capability
  report now have frozen frames, which is how the layout defects below were found by looking rather than by
  measuring (#212). The same change writes the rule as a guard: a golden renders a screen and never installs, by
  replacing the TUI's step executor with a spy on the screen where Enter starts a run. On a runner without
  privilege the old capture stopped at `/etc/wsl.conf`; anywhere with privilege it would have installed (#212).
- **The companion wanders on its own.** Its position and gaze are no longer governed by the cursor: after eight
  seconds without input it chooses between pausing, a short stroll, a look around and a small hop, waiting six to
  fourteen seconds between decisions, moving at most one cell every three ticks and returning to calm on input
  (#218). The random source is a private, injectable xorshift so the goldens stay repeatable, and the measured
  cost is about 6% more bytes over the same frames (#218).

### Fixed

- **The keys that lied are real, and the tests can now see a dead one.** Escape did nothing in Utilities, jumped
  past the picker in the theme refresh review, and in the restore confirmation jumped two levels to the main menu,
  all while the footer announced `[Esc] back`. They survived because `handleEscape` owns Escape, so the per-screen
  `esc` branches were unreachable code, and the tests pressed the key by calling those handlers directly. Five
  tests now drive `Update` with a real `tea.KeyMsg`, the path a keypress takes (#202).
- **Emoji are gone from the installer, and the rule is enforced where it was not.** The repository banned emoji
  because a terminal without an emoji font draws a box, and enforced it for the trainer alone while the
  installer's menus, titles and bodies kept theirs. Options and titles are words now, and where a glyph carried
  meaning it was replaced rather than deleted. The guard reads the `Screen` constants out of the source, renders
  the frames the shared terminal matrix already built, and fails by name - measured over 637 frames (#208).
- Escape on the main menu quit the application while the documentation called it "go back". It is a no-op on the
  root now, and the key, the documentation and the footer say the same thing; Backspace means exactly what Escape
  means, in one route through the real dispatch, with the trainer's answer line the written exception (#208).
- **Choosing a theme changes the screen once, not twice.** A press returned the switch command with the model
  untouched, so the frame after the press was the frame before it, and the result replaced the panel about a
  second later. The activity slot is now filled from the press and the result lands in the same slot, and the
  window around the list is a function of the frame alone, so the row under the cursor, the frame's height and
  the report's rows do not move between the press and the result (#204).
- **The interface stops stating facts it has not established, and stops moving the row under the cursor.**
  Utilities said "not switchable here" from an empty model while the definitions were still being read (it says
  it is reading now, and the main-menu panel says the theme has not been checked yet); writing the `.wslconfig`
  no longer replaces the table with "reading" while it re-reads, it keeps the table and marks the refresh; and a
  release row that arrives late is re-found by the label the cursor named, so a late `Restore from Backup` cannot
  steal an Enter aimed at Utilities (#206).
- **Neovim can always resolve the colorscheme the installer writes.** The switch wrote a name without leaving the
  generated file on the machine, so `:colorscheme nocturne` failed with `E185` because `~/.config/nvim/colors/`
  did not exist. It now installs the generated colorscheme before writing the line and names a plugin's
  colorscheme only when the machine's Neovim tree has one; otherwise it leaves the line out and says why (#210).
  The default theme was the same defect one level down - the committed block asked for the `kanagawa` plugin and
  the LazyVim statusline named it by hand - so the default is the only colorscheme this repository generates and
  installs, and the statusline leaves the theme to LazyVim (#214).
- **`BAT_THEME` named the wrong key, and five of six themes were affected.** `[bat] name` is the `.tmTheme`'s own
  `<key>name</key>`, but bat registers a custom theme under its **file** name, so only `dotfiles` reached the
  repository's file while the others painted bat's bundled theme or nothing at all. `batSelectionName` now exports
  the file's stem, measured with bat 0.26.1 against a cache built in a temporary `XDG_CACHE_HOME` (#171).
- The generated bat theme is well-formed XML: its header comment carried `bat cache --build`, and `--` cannot
  appear inside an XML comment, so both committed `.tmTheme` files were rejected by `xml.etree` even though bat
  rendered them. The guard parses every generated theme through `encoding/xml` and fails when it checked nothing
  (#175).
- The theme refresh review names the managed file the switch cannot bring forward instead of skipping it
  silently (#188).

- **A copied program keeps its executable bit, and every home file the shell step replaces is backed up.** The copy routine decided the mode itself, so `bash-env-json` -- executed by the nushell configuration the same install writes -- and `safe-update.sh` -- run as `argv[0]` by a Neovim command -- were installed un-runnable, and the failure surfaced as `EACCES` far from its cause. The mode now comes from the file, and an existing destination is `chmod`'ed explicitly, because `os.WriteFile` applies its mode only when it creates the file: leaving it alone would repair new installs and never the broken one (#241). `~/.zshenv` was replaced without being in `ConfigPaths()`, so the backup step and the record never saw it and a user's own file was destroyed with no copy and no mention; it is classified now, behind a guard that fails naming any home path the installer writes that the map does not classify (#238).

### Changed

- **Themes whose palettes were missing got them, and Kagawa was retired.** `everforest` and `kanagawa` are
  transcribed from their published palettes, `catppuccin-latte` is the light flavour, `rose-pine` the purple-pink
  one (and the darkest new base), and `kagawa` is removed because its only file was a byte-for-byte copy of
  Kanagawa and no published palette exists behind the name (#167).
- **The release procedure is one document, and the guard on it got stronger.** `docs/release-checklist.md` is
  deleted after its links were repointed at `docs/RELEASING.md`, and the guard now requires the runbook to name
  every job `release.yml` declares and rejects any release document that cites a workflow line number (#216).
- **The test loop's cost is now recorded, and the claim behind it was wrong.** `make check` runs `gofmt`, `go vet`
  and the changed packages with Go's cache on, so a rerun of `internal/tui` returns `(cached)`; what costs is host
  contention, measured at 10m20s of wall time against 72 seconds of CPU. `make preflight` adds the whole suite
  with `-count=1` by design, and CI is the complete verification (#166, #198).
- **The inspector screens got a hierarchy instead of a wall of prose.** Each terminal capability is now two
  units, the answer first and its source after; the shell audit shows the `zprof` attribution before the start
  list and stops printing the median twice; the WSL values come before the formula that derives them; and the
  theme picker's header says `Utilities`, like its siblings (#220).

### Removed

- `themes/kagawa.toml`, and the fish theme file that copied Kanagawa under that name (#167).
- `docs/release-checklist.md`, after its links were repointed at the one runbook (#216).
- The Utilities entry for the update, which is now a main-menu row; the update screen it opened is compiled but
  unreachable from the interface, and the Homebrew refusal no longer surfaces in it, though it still stops
  `--self-update` (#222).

### Notes

- **Declared limits, kept rather than hidden.** At 80x24 the theme picker's report cannot be read in full (the
  list keeps the rows and the report gets what is left) and the preview strip moves four rows when the result
  lands; both are the trade the picker was built with (#212). The companion's long pause is an approximation,
  because there is no "the user is reading" signal in Bubbletea (#218). Homebrew ownership is detected by path
  prefix and `/Cellar/`, so a Homebrew install whose symlink cannot be resolved is not detected (#216).
  `verify-published` has never run against a real release - the next release is its first real run - and only the
  linux/amd64 asset is executed by the workflow: the darwin and arm64 binaries are hashed and attested but never
  run (#216).

## [v0.5.0] — 2026-10-05

The release where the installer offers something beyond installing, and where the creature it draws survives
the screens it is drawn on. A utilities section joins the menu, and the small-terminal ladder gains a
volumetric rung so the cat is the 3D one on a laptop terminal as well as on a wide pane.

### Added

- **A utilities section**, reached from a visible `🧰 Utilities` row or the `u` key. Its first utility switches
  the desktop's light/dark theme through the desktop's own tool (GNOME `gsettings`, KDE
  `plasma-apply-colorscheme`, macOS `defaults`), never by editing that store itself. It records the value it
  replaced so the change can be undone, refuses to change a setting it could not restore, offers nothing on a
  host without a desktop, and is skipped by `--dry-run` like every other step (#153, #154).
- **A volumetric rung for small terminals.** The shaded creature now draws at 25-31 rows, where only the ASCII
  cat drew before; heights 20-24 keep the compact glyph deliberately, because a four-row volume loses the eye
  and a visible cat beats no creature at all (#152).
- The Utilities screen joins the frame and coverage guards, which now measure 648 screen x size cases and 39 of
  39 declared screens (#153).

### Fixed

- **The main menu's plan panel no longer crowds the creature out.** "What will happen" now names how many steps
  the plan has and the step the run is on, instead of spending a row per step, and the frame reserves the
  creature's block on the screens that draw the panel. At 100x25 the menu draws five rows where it drew none,
  and at 60x20 three where it drew none (#151).
- **The companion's rung is bounded to a quarter of the terminal height.** At 34 rows it took 35% of the screen;
  it now takes 23.5%, and 461 of 636 measured cases draw a creature against 423 before (#148).
- The wizard's plan panel no longer previews a plan when nothing has been selected (#148).
- A failed trainer progress save is now reported where the player lands, not on a screen they have left (#148).

### Changed

- **The release procedure has one home**: `docs/RELEASING.md`, derived from the workflow itself, plus
  `docs/release-checklist.md`, with a guard that derives the artifact names from `release.yml` and rejects the
  shapes the old documents used. `docs/RELEASES.md` was removed after its links were repointed (#149, #150).
- The local preflight added in this cycle runs the pull-request checks in about 35 seconds where CI needs about
  400 (#145).

## [v0.4.0] — 2026-10-04

This is the release where **the installer stops reporting successes it did not have, and fits the
terminal it is actually drawn into**. The wizard preselects the platform it detected, and on a host
whose platform it does not know it preselects nothing rather than macOS (#135, #143). Four places
that logged success whatever happened - a Ghostty download that failed, a tmux plugin install that
failed, a backup that failed to delete and ten trainer saves that failed - now report the failure
(#137), and `--dry-run` no longer runs `sudo`, Homebrew and `chsh` for real (#134). The frame guard
that had been missing measured 636 screen-and-size cases and caught the four screens that still
overflowed or clipped their frame (#139), and the screens that wrapped their text to a width they
never got now wrap to the column they are drawn in (#133). The cycle's verification work ships with
it: the last 47 sleeps in the suite became waits for the facts each test was about to assume
(#116, #118, #122), the restore tests stopped depending on the runner's `HOME` (#131), a pull request
stopped waiting eight to nine minutes for checks that were never gating anything (#82), and
`make preflight` runs the checks that can run locally in 34.8 s against roughly 400 s in CI (#145).

This cycle finishes the Vim trainer's buffer engine and its two new modules, and gives the installer
an interface that fits the terminal it claims and reads on a light one. The WSL configuration is also
now derived from the host that receives it, which removes the last artifact that carried one
machine's limits into another machine's VM. The installer's companion now stays still between
periodic idle events rather than repainting its own rows on every frame.

### Fixed

- **The companion's render cost and reserved block now have executable ceilings.** `BenchmarkCompanionVolumeFrame` fails above 10,000,000 ns/op, 450,000 B/op or 1,400 allocs/op, with headroom chosen from measured Linux/amd64 runs; `TestCompanionRenderedPosesStayInsideTheirReservedBlock` checks every walk pose, idle event, hop phase, gaze extreme and blink across both sprite modes and every rung. Raster-buffer optimization and performance guarantees for slower terminals remain out of scope.
- **The shaded companion's eyes now hold a sclera around a moving pupil, blink with a lid, and track gaze without dragging the body.** The full rung uses a 3×4 sclera and 1×2 slit at three long-axis positions. The slit keeps sclera columns on both sides at every position and has a full ring at centre; at either extreme it touches the corresponding lid. The smaller rung uses a 2×2 sclera and 1×1 pupil. `TestCompanionVolumeEyePupilAdjacencyByRung`, `TestCompanionEyeGazeIsolationAndBlinkPinsTheFaceContract` and `TestCompanionVolumeGazeTurnsTheHeadAndThePupils` pin the pupil margins, highlight, eyelid, head/ear motion and named-region bounds. The terminal-height ladder and reserved block are unchanged.
- **The shaded companion has a directional rim and contact shadows.** Lambert shading and ordered
  dithering remain, while a rim term now catches only the upper-left silhouette and two-step ambient
  occlusion darkens the neck, body and leg contact regions. `TestCompanionLightTerms` pins the encoder's
  five distinct tones and both spatial lighting terms. The encoder remains true-colour-only; 16-colour
  and colourless terminals keep the glyph companion.
- **The shaded companion has a cat's anatomy.** The volume now separates the skull, muzzle, neck, chest,
  haunch, triangular ears, four legs and tapered tail; the raster adds inner ears, paw pads, a dark nose,
  a two-pixel mouth and full-rung whiskers. Structural field and raster assertions guard the anatomy without
  changing the companion ladder or its reserved block.
- **Fish and Zsh startup customizations survive installer updates.** The shipped shell configs carry ownership markers, and the installer now preserves an existing unmarked `config.fish` or `.zshrc` as a dated, sourced drop-in (`~/.config/fish/dotfiles.d/` or `~/.zshrc.d/`) before replacing it. Both shells source personal drop-ins last so the user's settings override managed defaults; Fish uses `dotfiles.d` because native `conf.d` loads before `config.fish`. The install log names the preserved file; managed configs update without creating another copy. New personal Fish and Zsh startup changes belong in those extension directories.
- **The companion is the same size everywhere, and it no longer changes size as you move around it.** Its
  rung was chosen from the rows a screen happened to leave over, so the Vim Trainer kept a smaller creature
  than the menu while the menu's creature grew and shrank with its own content. The rung now comes from the
  terminal height alone, the trainer reserves it so it shows the same animal as the menu, and a screen that
  cannot hold it draws no creature rather than a smaller one.

- **The creature's colours stay inside the creature.** The shaded sprite set a tone per cell and never
  retired it, so the colour it left active painted every cell after it: a space is drawn with whatever
  colour is still set, half of the sprite's cells set a background, and a terminal's escape state outlives
  the line. The visible result was a solid bar of amber and one of light blue across the bottom of the
  screen, with the creature's head sitting inside a painted band. The pinned snapshot for the sprite
  contained not a single reset, so nothing had retired a style in the whole creature - and no test could
  see it, because a snapshot compares the escapes it is given and the check that verified the ink
  stripped them. There is now a guard that can see the class instead of the instance: **no rendered line
  may end with a style still active**, over every screen at five sizes, and it fails when the leak is
  reintroduced.
- **The companion is quiet until you move something.** It used to stroll along its row on its own and
  to halve the remaining distance to every target, so a screen nobody was touching repainted two of
  its rows on every frame — including the row the eyes settle on, a frame after the target moves —
  and the creature flickered. It now walks only while it has somewhere to go: moving the cursor points
  it at a row and it walks there at a fixed one cell a frame, slowing to a step every other frame over
  the last three cells, and stops when it arrives. One menu row is three cells of walking and no more,
  so one arrow key is a short stroll rather than a dash across a 227-column stage, and the gaze is
  settled when the target changes rather than on the next tick. Between idle events the view string
  is byte-identical, so the renderer writes nothing; while walking it repaints only the creature's own
  lines. The creature's body is also drawn in the theme's muted tones now, with
  the one bright tone spent on the eyes, so the decoration is no longer the loudest thing on a screen
  whose words should be.
- **The shaded companion walks, breathes, twitches, flicks and hops on model ticks.** Its four one-frame
  poses land front-left, back-right, front-right, back-left; the head leads by one pixel, the body rises
  at passing and the tail follows by two poses. Breath, ear twitch, tail-tip flick and blink recur at 4,
  20, 15 and 10 seconds, with no redraw between events. Clicks get anticipation, one terminal row of travel and
  landing squash inside the fixed block. `TestCompanionWalkPoseSequencePinsContactsAndLag`,
  `TestCompanionIdleEventsHaveIndependentPeriods`, `TestCompanionClickHopsAndCelebrates` and
  `TestCompanionCostHasTwoRegimes` pin the behavior and the zero-byte-between-events statement.
- **The installer's frames no longer tear while they animate.** bubbletea builds each frame in one
  write, but the terminal paints those bytes as they arrive, so a redraw that rewrites several lines
  was visible half-updated. The installer now brackets every frame in the terminal's
  synchronized-output mode (DECSET 2026, `\x1b[?2026h` / `\x1b[?2026l`), which makes the terminal
  hold the frame and repaint it once, installed through bubbletea's own `tea.WithOutput` and with no
  renderer of its own. A terminal that does not implement the mode ignores the sequences, so nothing
  is detected and nothing is at risk; the sequences are emitted only when stdout is a terminal, so a
  piped or redirected run keeps its bytes, and `DOTFILES_SYNC=0` turns the mode off for a multiplexer
  that mishandles it.
- **Four places that logged success whatever happened now report the failure.** The Ghostty step ran
  `curl … | /bin/bash`, which expands a failed download to the empty string, runs nothing and exits
  zero - the exact shape the Homebrew step's own comment had already named - and then logged "Ghostty
  configured"; both Ghostty paths now download to a file, check `curl` and run that file. The tmux
  plugin install discarded its result and logged "✓ Tmux configured" unconditionally, and now
  withholds the success line on failure (it still returns success and warns, because a pre-existing
  test outside that change asserts the step does not fail on a missing plugin seed - a decision,
  stated rather than hidden). A backup that failed to delete advanced as if it had been removed, and
  now reaches `ScreenError` like the restore branch beside it. Ten trainer save sites ignored the
  error while the profile reset beside them checked it, so a read-only `HOME` lost the player's
  progress without a word; all ten go through one `saveTrainerStats` helper that reports on the
  trainer's message line, with the warning ordered to win. Each site is guarded by its own test, and
  the Ghostty and tmux tests are `HOME`-sandboxed because a red run of them wrote
  `~/.config/ghostty/config` on the development machine. (#137)
- **`--dry-run` no longer performs the interactive steps for real.** The guard lived inside
  `executeStep`, and the TUI routes the interactive steps around it, so a flag documented as
  installing nothing ran `sudo` package installs, a real Homebrew install and a real `chsh` for the
  `deps`, `homebrew`, `terminal`, `wslconfig` and `setshell` steps. The interactive path now returns
  the same skipped result `executeStep` returns, with a `DRY RUN: skipping step "…"` log line, so the
  walkthrough still runs and changes nothing. The old test called `executeStep` directly and the E2E
  dry-run matrix is `--non-interactive`, which is why neither saw it; the new test drives the
  interactive dispatch. (#134)
- **The wizard preselects the platform it detected, and never falls through to macOS.** The
  preselection compared the detected platform with the generic `system.OSLinux` constant and used an
  `else` for everything else, so Debian, Ubuntu, Arch, Fedora, Termux and WSL - each with its own
  constant - all landed on the macOS row, under a title that said `Detected: Debian/Ubuntu (WSL)`.
  `osOptionIndex` now maps every platform explicitly with no `default`, and a class-level guard
  derives the platform constants from the `system` package so adding one forces a decision instead of
  a default. (#135) A second round found the same default one level up: the caller assigned the
  cursor to option 0 - macOS - when the mapping reported an unknown platform. An unrecognised
  platform and an unrecognised shell now open with nothing highlighted, say so (`Platform not detected
  - select your operating system`) and refuse `Enter` until the reader chooses, because a placeholder
  that reads as a selection is the shape this defect was born from. (#143)
- **The terminal step copies its configuration from the checkout instead of a `cwd`-relative
  `dotfiles/`.** There is no `dotfiles/` directory in this repository and the clone step checks out
  into a private temporary directory, so the copy source resolved only by accident, if at all, and
  the interactive script repeated the same hard-coded path under `set -e`. Both paths now resolve
  each asset through `m.repoDir()`. (#132)
- **The selection screens wrap to the width they are drawn at.** Their descriptions were wrapped to
  the whole terminal while the two-column composer truncates every left-hand line to its column, so at
  124x24 a 97-cell note was wrapped at 120 columns and cut at 56: the tail of every long line was
  silently discarded, in the screens whose own comment claimed the clipping had been fixed. The guard
  knew one of the two ways a cut appears - a suffix marker - and the panel's own wrap marker made a
  mid-line cut invisible, so it now walks each line's display columns and fails when the marker falls
  inside the body's own range. (#133)
- **Every screen is now measured against every terminal it ships to.** The frame guards iterated a
  hand-written list of states, so four defects shared one cause and nothing was checking it:
  `TestEveryScreenFitsEveryTerminalSize` renders 53 screens at 12 sizes - 636 cases, and the test pins
  that count - and asserts each frame's rows and its widest visible line against the terminal, counting
  a wide rune as two cells. It caught the welcome screen dropping its bottom rule and help footer off a
  44-row terminal (lipgloss `Style.Width` word-wraps, so centring doubled the wordmark once the left
  column narrowed), the trainer header letting its title be cut to the frame, the LazyVim topic
  reserving one footer row where the footer packs two, and the `keymaps` description widening its block
  until the terminal clipped it. The trainer's one-row companion rung at the 80x24 floor stays a
  documented exception, now stated where the ladder is stated and pinned by its own test. (#139)
- **The installer's titles say what happened.** Routing three screens through the one place titles come
  from was right, and the values they inherited were vaguer than the text they replaced: a failed
  install said `Error` where it had said `Installation Failed`, and the installing screen said
  `Installing...` where it had said `Installing dotfiles`. They now read `Installation failed`,
  `Installing dotfiles` and `Installation complete`. (#44)
- **The Vim trainer stops accepting wrong answers, miscounting practice and losing progress.** An
  answer the simulator cannot fully parse is rejected, and an operator or text-object answer must
  match the selection it produces, through one notion of fully consumed input; the practice counters
  have a single owner, so one submission is counted once; the space key is exempted by one predicate
  over the screens where it selects and stays ordinary input on the exercise screens; leaving a
  lesson, practice session or boss with `esc` persists stats and reports an abandoned boss instead of
  dropping them; control keys the simulator cannot parse are ignored rather than inserted as the
  literal text `ctrl+a`; and a boss answer is recorded through the canonical recorders, so a spent
  life and the attempt count have one owner and one window cannot cost two lives. (#34)
- **The trainer's hints say something the exercise's description does not, and the hint line is
  guarded.** A hint revealed with `Tab` used to repeat the mission — the mission read "Move to the
  start of 'userName' using w (word)" and the hint read "w moves to the start of the next word" — so
  asking for it cost a keypress and taught nothing, which is the defect a player reported. Every hint
  now adds the mechanism its mission leaves out: the count, flag or range the command takes, the part
  of it the mission does not name, how it compares with the command it is easiest to confuse it with,
  or what follows from it. One hundred and twenty-one hint lines were rewritten across the nine
  modules and the change-and-repeat boss fight, and no judging, solution set or lesson count changed.
  `TestShippedHintsAddWhatTheirMissionDoesNot` sweeps every shipped hint for one of those additions,
  and `hintEchoRewrites` pins the exact echoes that were withdrawn so a later edit cannot restore
  them. The hint line itself is now guarded: `trainerHintLabel` is the one place that builds
  "💡 Hint: …", and it returns nothing when the exercise carries no hint, so a hint that was
  legitimately dropped no longer renders a bare label — and the five content tests that required every
  lesson to have a hint were reshaped, because a hint is optional under the mission/hint rule and the
  guard over a hint that *is* present is the one that matters. The hint copy the Change & Repeat boss
  steps already carried was unreachable — the boss screen had no key that could show it — so the boss
  screen now reveals its step's hint on `Tab` on the same terms, and its legend advertises the key
  without costing a row.

### Changed

- **The trainer's lesson screen goes two columns on a wide terminal.** From the same 124-terminal-
  column floor the framed screens use, the code window moves to the left column and the mission, the
  answer line and the feedback move to the right, through the same `layoutFor` columns and the same
  `composeColumns` composition the installer's own screens use — no second idea of "wide". The code
  window keeps every row the stacked right column no longer occupies: at 160×50 the lesson's window
  budget is 44 rows against 11 at the 80×24 floor, and a fourteen-line exercise that scrolls below
  the floor shows all fourteen in full. The mission, the answer and the feedback are wrapped to their
  own column and lose no line; the menu and the boss screens keep their one-column bodies; and at
  exactly 80×24 the lesson screen is byte-for-byte what it was.
- **The companion became a creature.** The installer's ASCII cat was one row of seven characters; it is
  now a **shaded volume** where the terminal can shade, and a ladder picks the size from the rows the body
  did not need: the volume at twelve rows where it can spare twelve, the same volume at eight where it can
  spare eight, and then the glyph cat at five, its head at three, the one-row art the creature shipped
  with, and nothing below that. The panel summary's rows come off the spare rows first, because a fact
  still beats a decoration, and no screen loses the creature it had.

  The volume is **rendered, not drawn**: an implicit surface — a handful of metaballs for the head, the
  muzzle, the body, the four legs and the tail, cut at a threshold — shaded by a light from the upper left
  with normals taken from the field's gradient, quantised to the installer's own tones through a fixed
  ordered-dither matrix so the gradient reads smooth instead of banded, and grounded by a drop shadow.
  Animation **moves the metaballs** rather than swapping drawings, which is what makes the shape deform:
  a walk cycle with alternating legs and a one-pixel bob, a lean, a tail that counter-sways, and a head
  and pupils that turn toward what the creature is looking at. The glyph ladder under it needs no colour
  at all and is untouched — a half-block cell without colour draws as a plain block — so a terminal
  without true colour, a run with `DOTFILES_SPRITE=0` / `--no-sprite`, and a frame with fewer rows all
  still get a cat. The pupils sit at one of three columns and the eye row on one of two, and a composer
  moves them, so the tables stay one frame per state instead of thirty drawn frames; it blinks every ten
  seconds and yawns through the last two before it sleeps.

  **Its eyes follow the pointer.** They followed the selection; the pointer is what they look at when
  there is one, and the selection remains the fallback for a run or a terminal without one. Mouse motion
  is asked for only when the run can use it and it is a switch of its own — `--no-mouse` /
  `DOTFILES_MOUSE=0` — because mouse reporting costs the user the terminal's own drag-to-select: with the
  pointer off **no mouse mode is requested at all**, so ordinary selection works again, where a run that
  merely ignored the events would still have taken the drag. Termux defaults to no pointer, because a
  finger is not a hover and Termux turns a drag into a wheel report, and `DOTFILES_MOUSE=1` overrides
  that. The turn happens on the mouse message rather than on the next frame, the pupils rest inside a
  two-column dead zone so they cannot flicker, a pointer that has not moved a cell changes no byte, and
  every pointer event wakes a sleeping creature. A click earns the celebration a finished step gets plus a
  hop of one row, which costs a spare row and is skipped where there is none rather than take a fact's row.

  Its price is **measured, not estimated**, and it has two regimes (`TestCompanionCostHasTwoRegimes`): at
  rest, with no key and no pointer event, the view is byte-identical from tick to tick and the renderer
  writes **nothing**; while it walks, the volume's twelve rows all move, so twelve whole lines are
  repainted — 7488 bytes in the widest frame, about 58 KB/s at eight frames a second for the two and a
  half seconds a walk lasts. Rendering the volume is the expensive part of that:
  `BenchmarkCompanionVolumeFrame` measures about 1.6 ms a frame. `charmbracelet/x/mosaic` was evaluated for
  this and rejected on two measured reasons — it forces `x/ansi` ≥0.11.7, which breaks the `x/cellbuf`
  Bubble Tea pins and drags eight modules through the render path of every screen, and its
  luminance-threshold colour model collapses the shading boundaries the volume is made of — so the
  hand-written renderer is a decision, and the reasoning is written down beside it. The frame, the cell
  and the gaze all come from the model, so a snapshot can pin a frame instead of flaking on a clock.
- **The installer has one palette that reads on a light or a dark terminal.** The theme was near-white
  text on whatever background the terminal happened to have, so on a light terminal the body text was
  effectively invisible. Every colour is now adaptive: it asks the terminal for its background and
  picks a light or dark variant, so a light terminal gets dark ink and darker accents while a dark
  terminal keeps the existing values. Only the colours change between the two; words and glyphs still
  carry every state, so a 16-colour or no-colour terminal loses no information.
- **Every screen writes its key hints the same way.** Navigation, the action and the way back now use
  one bracketed notation and one separator everywhere, including the trainer, whose legends were
  hand-typed and could drift from the keys they promised.
- **The installer now uses the terminal it is given instead of assuming the 80 columns it was
  designed at.** The body is capped at 160 columns and centred in the rest, the bar behind a selected
  row is a measure of at most 80 columns instead of a slab of colour across the whole terminal, and
  from 124 terminal columns the welcome screen and the main menu carry a second column: the machine
  the run is about to change, and the plan it will execute with the configs it will overwrite and the
  newest backup. Below that floor the panel is dropped and the screen renders exactly as it did, so
  nothing is lost. A body shorter than its frame no longer floats down the screen either: the blank
  rows above it are capped at six, where a nine-row menu in a 62-row terminal sat 26 rows below the
  header that named it. The plan panel numbers its steps with the marker in its own column and the
  numbers right-aligned, so the names line up. The panel follows the selection instead of always
  describing the installation: on `Restore from Backup` it lists the backups with their dates and
  file counts, on `Keymaps Reference` it counts the bindings each tool ships, on the trainer it
  counts the curriculum, and on `Exit` it says in one line that nothing changes. The panel's name
  stays put so `Tab` does not wander — the option is named in the panel's first row instead — and
  the wizard's own questions preview the plan the highlighted OS or shell would lead to while still
  marking the step the run is on. The newest-backup row keeps its age ("9 days ago") without the
  render path reading a clock: the reference time is the model's own, so a snapshot still pins the
  bytes while a live run reads like a person would write it.
- **Menu and status copy was rewritten to say what to do.** Detected-platform labels no longer repeat
  the line above them, the Alacritty build warning no longer sits inside the terminal's name where it
  read as part of it, the welcome screen's environment note is one sentence instead of three facts
  joined by pipes, and empty or blocked states name the next step instead of only the problem.
- **The Macros module's global-command lessons claim only what the judge can check.** The lessons that
  described the result of `:g/...` and `:v/...` commands named an effect the engine does not run; they
  now teach the keystrokes and say so, and a guard test catches that class of unkept promise in
  future content.
- **`.wslconfig` is rendered for the host it is installed on instead of shipped as one host's
  numbers.** The file was checked in holding what a single Windows machine had settled on — 6 GB of
  memory and 4 GB of swap against a 15.6 GB host, with 8 processors on a 12-thread one — and the
  manual recipe told the reader to copy it and adjust it by hand. The checkout now ships
  `dotfiles-wsl/.wslconfig.tmpl`, and the installer reads the Windows host through WSL interop and
  renders it, so the limits that reach a machine are computed from that machine: memory is half its
  physical memory, swap is a quarter of that memory, and processors are every logical CPU it reports.
  Both memory values are rounded down to 512 MB, Windows is always left at least 2 GiB, and a memory
  plan below 1 GiB is not written at all.
- **A host that cannot be read omits the limits rather than guessing them.** When interop is disabled
  or unavailable, `memory`, `processors` and `swap` are left out of the rendered file and WSL applies
  its own proportional defaults, which Windows computes from the real host. That is a supported
  configuration and not a broken one, and it is now the instruction the manual recipe leads with.
- **The install step and the interactive script render the same bytes.** Both routes go through one
  shared render helper, so the non-interactive step and the shell script a guided run executes can no
  longer disagree about what `.wslconfig` contains.
- **The suite waits for the facts it was about to assume instead of sleeping past them.** The last 47
  sleeps - 13 became waits for output to arrive, 22 waits for the screen the test opens, and 12 moves
  that needed no wait at all - are gone, and the trainer's own file, which held 36 of them, now waits
  on conditions through `teatest.WaitFor`. A first attempt replaced the library's 14 existing waiters
  with a home-made reader that read the stream differently and timed out over output the library would
  have matched; restoring them, with the shared helper delegating to the library, is kept in the branch
  history as the reason the fix is short. (#116, #118, #122)
- **The slowest guard renders its screens in parallel, and counts what it covers.** The constant-space
  guard rendered about eight hundred screens one after another on a single idle core. The models are
  still built sequentially, because the helpers call `t.Setenv` and that is legal only on the test's own
  goroutine; the renders - the expensive part, and pure per model - go through a bounded pool over an
  indexed slice so each result keeps the identity its failure message needs, and the assertions run in
  order over what came back. The test now asserts its own counts, 848 rendered screens and 848
  comparisons, so a change that quietly covers less fails there instead of saving time. (#120)
- **The restore tests waited on the runner's `HOME`, not on a race.** Seven attempts blamed timing: the
  startup backup scan replaces the model's seeded backups with a real scan of `$HOME` and has no guard,
  so on a runner whose home holds no `.dotfiles-backup-*` directories the list emptied, `Enter` stopped
  leaving the screen and the confirm screen became unreachable - while on a machine with three real
  backups the tests passed for the wrong reason. Each restore test now points `$HOME` at a `t.TempDir()`
  holding exactly the backups it declares. (#131)
- **A pull request no longer waits eight to nine minutes for checks that were not gating anything.**
  The E2E workflow cancelled unconditionally on a group shared by every push to `main`, so the next
  merge killed the previous commit's matrix: a merged commit whose only full installation was cancelled
  five seconds in, and never re-ran. Both workflows now cancel on pull requests only, because waiting
  behind the previous run costs minutes while abandoning a merged commit's check costs the check. The
  gates that only serialised - `linux-e2e` waiting on `go-tests`, `build` on `go-validate` and
  `branding-audit`, none of which consumes another's output - were removed for the same reason. (#82)
- **The E2E image cache was implemented, measured and deliberately not shipped.** Three runs of the same
  ubuntu job put the image build at 32.5 s of a roughly 400 s job, and the warm cache build took 42.2 s
  against the 32.5 s with no cache at all, so the cache was not being reused; even a hit is bounded by
  under ten per cent of the job, while the cold run paid 193 s to write it. The cache is out, with the
  figures and the untested `mode=max` hypothesis in `odd/tasks/ci-efficiency.md`. (#84)
- **The E2E shell check fails when the shell is missing.** `test_shell_functional` asked whether fish and
  zsh worked and had no branch for their absence: it recorded no verdict and the suite counted it as a
  pass. Both absences are failures now, and the coverage ledger names the new guard. (#116)
- **Two guards for classes the previous checks could not see.** The frame guards iterated a hand-written
  list of states, so a screen added to `model.go` was measured by nothing and the omission stayed
  invisible because the guards stayed green either way; the list now derives from the screens the source
  declares, and the count is asserted. (#94) And the installer's promise that nothing it means is
  carried by colour alone - Termux is supported, a 16-colour profile is supported, a piped run has no
  colour at all - is now checked by rendering five pairs of models with every escape sequence removed
  and requiring each pair to neither draw the same characters nor differ only in trailing padding. (#98)

### Added

- **The installer shows the machine it is changing, live.** A sampler reads the host about once a
  second — CPU busy as the delta between two readings, memory used and total, the load average, the
  free space on the target and the process count — as a command, never while rendering, and the
  readings live in a ring on the model. It reads `/proc` on Linux and WSL and `kern.cp_time`,
  `hw.memsize`, `vm_stat`, `vm.loadavg` and `ps` on macOS, and it degrades row by row: Termux and an
  unreadable host report nothing rather than a zero dressed as a measurement, and the panel says so
  in one line. The numbers are drawn by hand, in block sparklines and braille, so a 16-colour or
  no-colour terminal loses nothing — the shape is the information. They appear in a new **This
  machine, now** panel on the welcome screen and, most of all, on the installing screen, where the
  machine's pulse sits beside the progress bar and the run's own progress is charted over time; the
  pulse is drawn only from the rows the rail and the log do not need, so it never displaces them.
  The sampling is gated by the same switch as the animation: with it off no sample is taken and the
  panel says the sampling is off rather than freezing a chart and calling it live. The cost is
  bounded by a test: a reading repaints only the rows a live widget owns, and a screen that shows no
  reading renders the same bytes after one lands.
- **The progress bar has a travelling highlight, and a finished run celebrates.** During a long step
  one lighter cell (`▒`) walks the filled part of the bar on the frame tick, so the wait reads as
  alive; it is a glyph difference, not a colour one, and it only draws while a run is in flight. When
  the run finishes, a two-second burst of particles rises in the rows the body did not need and the
  companion is pleased — both model state advanced by the frame tick, both absent with animation off.
- **The installer has a companion.** A small ASCII creature walks the row immediately above the
  footer, follows the selection you move the cursor to, sleeps after twenty quiet seconds and wakes
  on the first key, and reacts to what is on screen: alert on the choices that throw something away,
  pleased for about a second when an installation step finishes, flinching while an error is showing and
  on the trainer's result screens when the last answer was wrong. The art is drawn in this repository
  in plain ASCII — no emoji and no third-party mascot — so Termux and a 16-colour terminal keep
  working and the state reads from the glyphs rather than from a colour. It draws only in a row the
  body did not need, so a screen whose body fills its frame shows no companion and loses nothing, and
  with animation off there is none at all (`DOTFILES_ANIM=0`, `--no-anim`, a piped stdout or
  `TERM=dumb`).
- **The Vim trainer can judge an answer by the buffer it leaves, not only by where it moves the
  cursor.** A mutable editing engine runs the answer and compares the buffer text, the cursor and
  the mode it produces against the optimal's result, so undo, put, register and indentation
  exercises are marked by their actual effect. When a rejected answer differs, the result screen
  shows the expected buffer next to the produced one. Commands the engine does not run yet are
  taught as keystroke exercises and say so, instead of promising a result nothing checks.
- **Two trainer modules: Editing & Undo and Registers & Indentation.** Editing & Undo covers
  insertion, escape, undo/redo and the line edits around them; Registers & Indentation covers the
  named registers, the yank-only register, put before and after the cursor, and the shift commands.
  Each has its own lessons, practice pool and boss, and both extend the unlock chain after Macros.
- **A progress bar and a per-step status rail on the installing screen.** The screen shows a bar
  sized to the terminal with a percentage beside it, and a rail of one row per step whose state is a
  glyph and a word, so the run's progress and each step's state are readable without colour.
- **`DOTFILES_WSL_HOST_CPUS` and `DOTFILES_WSL_HOST_MEMORY_MB`**, for a machine whose interop is
  unavailable: setting both to the host's logical CPU count and its memory in MiB replaces the
  Windows-side query. Both must be present and valid, so a half-configured override falls back to
  detection instead of planning a limit from one guessed number.
- **Three more panels, a `Tab` key and a narrow-terminal summary.** Every panel answers a question:
  **Your trainer** answers what you have gained (the lessons and mastery of each started module, the
  overall accuracy, the best streak, and the next boss with the practice gate it needs), **Did you
  know?** answers what you can learn right now (one shortcut at a time), and **Last install** answers
  when this machine was last installed. A screen that offers more than one panel starts its column
  with a tab row naming them and cycles them with `Tab`, which the footer advertises only where the
  key does something. Below the two-column floor the active panel collapses to a one-line summary
  placed in rows the body did not need, so no summary ever takes a row from a body.
- **The not-yet-known panels say nothing rather than zero.** A panel with no run behind it leaves the
  row out: with no trainer record the **Your trainer** panel says so in one line instead of drawing
  zeros that read as progress, and the **Last install** panel is offered only when a record exists,
  so no section ever says "never".
- **Tips come from content the repository already ships.** The **Did you know?** pool is built in a
  declared order with no randomness from the keymap reference data and the trainer's own lessons, so
  two runs on one machine show the same sequence; it advances one tip per ten seconds, and with
  animation off it stays on the first tip.
- **The installer records when it last ran.** A completed run writes a small record — the finish
  time, the build and the configuration paths it replaced — to
  `$XDG_STATE_HOME/dotfiles/last-install.json` (falling back to
  `~/.local/state/dotfiles/last-install.json`). The write is best effort: a state directory the
  machine will not let us write never turns an install that finished into one that failed.
- **The installing screen says how long the run has taken and what is left.** Under the progress bar
  it names the step the run is on, and it states the elapsed time with an estimate of the remainder
  scaled from this run's own clock and progress (`Elapsed 3m 05s · ~1m 42s left`). The estimate is
  stated only when it has a basis: with nothing complete it says `estimating…`, and with no run
  behind it it says `Estimating the time remaining…` rather than a number. The start timestamp and
  the latest tick live on the model, so the render never reads the clock and a snapshot cannot flake
  on the hour. The run keeps no file tally, so the screen shows no file counter.
- **The log box follows the terminal instead of a fixed three lines.** The installing screen's log
  and the error screen's log panel — which cut at five lines for no stated reason — now show the
  freshest lines that fit the frame and, when earlier lines do not, name how many were left out in
  one dim row (`… 9 earlier lines`). At 80x24 the installing box shows more than twice what it did;
  the step rail keeps a three-row floor, so the box can never push it or the footer off screen.
- **The welcome screen and the main menu greet by the time of day.** One added dim line
  (`Good morning`, `Good afternoon`, `Good evening`) greets the reader without replacing any existing
  copy. The greeting is a pure function of the time the model was created with, never of the clock
  read while drawing, so a snapshot pins it instead of flaking at the hour.
- **The main menu answers to `vim`, `:q` and `dd`.** `vim` opens the Vim Trainer, `:q` quits, and `dd`
  briefly removes the selected row as Vim would. (#114)
- **The Vim trainer scores real time, shows a countdown and times the boss fights.** The game state
  stamps when an exercise is presented and exposes the elapsed seconds through an injected clock, so
  the scorer no longer receives a hardcoded `10.0`; the screens show the time remaining before the hint
  and reveal it when the deadline passes, and expiry is a hint rather than a failure, because the
  exercise stays open and answerable. Boss steps run their own clock: an unanswered step costs exactly
  one life through the same recorder a wrong answer uses, shows the solution and re-arms, a won step
  grants the boss's bonus time to the step that follows, and a spent life is the single owner of the
  window re-arm, so one deadline cannot cost two. (#36)
- **The Vim trainer reports its progress and can clear it on request.** Every module entry shows the
  lessons completed against the total, its mastered count and its boss state, and the selected entry
  adds its commands and its most-often-missed exercises, with mastery moved to one predicate shared
  with weighted practice selection. `shift+R` arms a whole-profile reset and a second press clears it,
  while any other key cancels and `esc` leaves everything alone; the lowercase `r` still resets one
  module. (#38)
- **The installer provisions the Pi agent skills and the OfficeCLI binary.** `security-audit`,
  `archify` and `officecli` are installed under `~/.pi/agent/skills/`, each from an immutable commit
  and each checked against a hard-coded SHA-256 before anything is staged, so a mismatch or a failed
  transfer fails the step without touching the destination; extraction refuses archive entries that
  would escape the staging directory, and a destination that already exists is reported and left
  untouched, so a locally modified or newer skill survives an installer run. The pinned files come
  from the raw host rather than the rate-limited contents API, which settles a `403` that had turned
  the ubuntu E2E job red (#56). OfficeCLI is installed from one versioned release asset per platform,
  verified the same way, instead of the project's unpinned shell installer (commit `37e71c1`).
- **The installer's welcome emblem is a faceted gem, and it is the same mark in the Neovim
  configuration.** The emblem and the compact lockup a short terminal gets are redrawn as a cut stone
  in the installer's welcome screen and in the Neovim dashboard, so the two no longer disagree about
  what the mark is (commit `3f4961f`).
- **`make preflight` runs the checks that can run locally, in CI's order.** Seven steps - `gofmt`,
  `go vet`, a build with the `--help` smoke test, the whole Go suite, `shellcheck`, the branding audit
  with `ci.yml`'s own exclusion globs, and `gitleaks` over the commits the branch adds - each naming the
  CI job it mirrors, measured at 34.8 s locally against roughly 400 s in CI. A missing tool stops the
  run rather than skipping a check, the script builds into a temporary directory so a run leaves the
  checkout clean, and the footer names what it cannot run: the Docker E2E matrix, Termux, the E2E
  image-list job and the macOS toolchain. A green preflight is not 14 of 14, and it says so. (#145)

### Removed

- **The deprecated Neovim Gemini CLI integration is gone.** `jonroosevelt/gemini-cli.nvim` is removed
  and replaced by `mceazy2700/antigravity-cli.nvim`, which ships disabled like every other AI plugin,
  because the configuration enables one assistant at a time and the `<leader>a` key is shared by all of
  them. The plugin it replaced called `setup()` at load time on top of the config function the file
  declared, so it ran its own setup twice. (#79)

## [v0.3.0] — 2026-09-22

This is the release where **the installer stops ignoring the `Brewfile`**. The repository has
declared the machine's toolset in that file since it was adopted, and no step ever read it, so a
fresh install produced a machine carrying only what the shipped configuration needs at shell start.
Installing the dotfiles and then finding `btop` absent was the visible symptom: the `Brewfile` names
62 installable entries and the installer provisioned none of them.

### Added

- **A step that provisions the toolset the `Brewfile` declares.** It reads the file from the
  checkout the clone step already creates and hands a filtered copy to `brew bundle`, so the
  `Brewfile` stays the single source of truth: a tool added to it is provisioned without a change to
  the installer, and 62 package names are not duplicated into Go. `brew bundle` supports every
  directive the file uses.
- **`btop` in the zsh shell step**, with a guarded `alias top='btop'` in the zsh configuration. The
  alias is defined only where `btop` is present, so it stays inert on a host with neither Homebrew
  nor `btop`, which is the shape `trippy` already used.

### Changed

- **The `vscode` and `winget` sections are deliberately excluded** from what the step installs.
  Installing Windows desktop applications — Chrome, Office, Teams, a JDK — as a side effect of a
  dotfiles install is out of proportion, and the `vscode` extensions need the `code` CLI, which is
  absent on the servers this also runs on. The step logs how many lines it dropped from each section
  instead of hiding the omission, and both sections stay in the `Brewfile` for anyone who applies it
  by hand on a workstation.
- **`brew bundle` runs with `HOMEBREW_BUNDLE_NO_UPGRADE=1`.** `brew bundle install` upgrades by
  default, which an installer must not do: the step installs what is missing and leaves the rest.

### Notes

- **The step is best-effort.** An entry that cannot be installed, and a `Brewfile` that is missing
  or unreadable, are logged and the run continues. The one exception is a missing checkout, which
  reports a failure: a run with no checkout has already failed at the clone step, and a silent skip
  there would hide it.
- Termux and any host without a usable Homebrew skip the step, and `DOTFILES_SKIP_TOOLSET=1` skips
  it explicitly. The E2E script sets it, because `brew bundle` would otherwise install roughly sixty
  formulae in every container and blow the job runtime.

## [v0.2.3] — 2026-09-21

Two defects on the interactive path, and the assertions that stop the sequence they belonged to.
This is the release where **the TUI installs on WSL**: v0.2.2 fixed the step loop, and the run still
died one step later.

### Fixed

- **A step the interactive dispatch table did not know.** Every TUI run on WSL died at the WSL step
  before that step did anything: `failed to get script for wslconfig: unknown interactive step:
  wslconfig`. The step is flagged `Interactive` because `/etc/wsl.conf` needs sudo, but the table
  that flag routes into had no case for it, so neither `.wslconfig` nor `/etc/wsl.conf` was ever
  written. It had carried that flag since it was introduced, so the TUI had never been able to run
  it.
- **A step that reported itself done having installed nothing.** The interactive terminal step
  returned an empty script for WezTerm on Debian, Ubuntu and WSL, on the stated grounds that
  "Debian uses brew, not interactive", while the non-interactive step installs it through
  Homebrew. Both paths now render from one shared helper, so they agree by construction rather
  than by a comment.

### Added

- **Three assertions where there used to be prose.** A step marked interactive with nowhere to
  dispatch to, a step scheduled for the non-interactive run that the executor does not know, and a
  scheduler and an executor that must agree about the set of steps: all three are now checked in
  both directions. Three defects in a row on this path each revealed the next because those
  agreements were comments rather than tests. That is the part meant to stop the sequence.

### Changed

- The Go WezTerm path now aborts when Fedora's `dnf copr enable` fails instead of ignoring it, and
  the Debian tap runs through the Homebrew prefix with its error checked. The interactive path
  already used `set -e`, so this aligns the two.

## [v0.2.2] — 2026-09-21

A patch release for one defect, and it is urgent rather than routine: **v0.2.1's TUI could not
complete an installation at all.**

### Fixed

- **The TUI carries what each step records to the next one.** `runNextStep` had a value receiver,
  so the pointer it handed to the step pointed into a throwaway copy: the clone step recorded its
  working directory and checkout path there while `Update` returned its own model, which never
  received them. The clone logged `✓ Repository cloned successfully` and then every step after it
  failed with `the repository has not been cloned in this run`; cleanup lost the same fields, so
  the checkout was left behind as well.

  The reported cause was right about the mechanism and incomplete about the fix, which is worth
  recording: changing the receiver to a pointer **does not repair it**, because `Update` returns its
  model by value and that copy is taken before the asynchronous `tea.Cmd` runs. The recorded state
  now travels back through the step-completion message and is applied where the model is kept.

### Added

- **A test that drives the TUI's own step loop**, asserting that what one step records is visible
  to the next. This is the second defect in a row on the TUI path, after the dependency script
  fixed in v0.2.1, and both survived for the same reason: the E2E suite only runs
  `--non-interactive`, so nothing had ever exercised the TUI's step sequence. The new test failed
  with the exact symptom above before the fix, which is what makes it evidence rather than
  decoration.

### Notes

- If you installed v0.2.1 and the TUI stopped after the clone, this release is the fix. `brew
  upgrade dotfiles` is enough.

## [v0.2.1] — 2026-09-21

Four defects reported against v0.2.0, all of them on WSL, plus the coverage that was missing for the
Arch package path.

### Fixed

- **A failed step no longer abandons the run.** A failure writing the root-owned `/etc/wsl.conf`
  returned from the run immediately, so `set default shell` and `cleanup` never executed, the
  default shell was never attempted, and the temporary checkout was left behind in `/tmp`. The
  steps now run as a group that collects failures, finishes what it can, cleans up, and still
  exits non-zero naming what failed.
- **The Windows profile is resolved without the interop that just failed.** The fallback in
  `windowsUserProfile` read the Windows user name by running `cmd.exe` again, so both routes died
  together and `.wslconfig` was skipped on exactly the machines the fallback existed for. It now
  reads the mount and never runs interop, and on an ambiguous machine it lists the candidates and
  names `DOTFILES_WSL_WINDOWS_HOME` instead of writing into a stranger's profile.
- **The installed shell stopped printing an error on every prompt.** The `.zshrc` runs
  `fnm use default` on every shell, but nothing created that alias, so the configuration was
  installed in a state it could not satisfy: `error: Requested version default is not currently
  installed` before every prompt and, with Herdr, in every new pane. The prompt now requires the
  alias and discards fnm's stderr, and the installer creates the alias **only for an fnm it
  installed itself** — an fnm that was already there belongs to its user and is left alone.
- **The TUI's dependency step now shares the tested path's decisions.** This is a correction to
  the previous release's claim as much as a code fix. `getDepsScript` was a second implementation
  of the dependency step: a raw `sudo apt-get` script that never consulted the Homebrew preference
  and never went through the availability filter. On WSL Debian it asked `apt` for `wslu`, which
  Debian does not carry, and `apt` aborts the whole transaction on one unknown name, so the TUI
  stopped at step 2. v0.2.0's entry says that class of failure was eliminated; it was eliminated on
  the path that had been tested, and this was the other one. Both paths now build from the same
  package set and the same dispatch, and what differs between them is only where the plan is sent —
  executed, or rendered into the script the TUI runs so `sudo` can prompt on a TTY.

### Changed

- The TUI dependency step no longer runs `pacman -Syu` on Arch or `dnf check-update` on Fedora: the
  tested non-interactive path never did, and the two now agree.

### Added

- **The Arch package path is covered by CI.** `docker-test.sh` had five images and none of them was
  Arch, so that path had been verified by hand exactly once. The new image uses a real `archlinux`
  base and a real `pacman` — no simulated package manager, because an image that fakes its package
  manager is how an E2E job stops proving anything.

### Notes

- `wslu` is now dropped for Debian and Ubuntu alike, because the installer cannot tell them apart;
  the run says so and points at Homebrew or the tool's own installer.

## [v0.2.0] — 2026-09-20

The installer now installs on the platforms it claims to support, and stops reporting success for
work it did not do.

### Added

- **Herdr in the Keymaps Reference.** Herdr was offered by the multiplexer selector, accepted by the
  CLI, documented on the Learn screen and installed — but it was the one tool with no reference
  entry. Every binding comes from `herdr --default-config` on herdr 0.9.1, cross-checked against the
  versioned keyboard page; the two entries the config does not print are marked as such. Because
  Herdr documents itself as mouse-native, the reference leads with a pointer category rather than
  presenting keyboard bindings alone.
- **The E2E suite exercises the terminal axis, the font option and `--wm=herdr`**, and walks the
  whole option space under `--dry-run`. Two of the five option axes had no automated coverage at
  all, which is why defects in them were invisible to CI.

### Changed

- **WSL keeps the distribution underneath it.** It was modelled as an operating system rather than
  a hosting environment, so detection stopped before identifying the distribution and every
  dispatch that reads the OS was blind on WSL. Package installation works there now, and the E2E
  suite can finally validate the installer on a WSL2 host.

### Fixed

- **The installer installs on WSL without Homebrew**, instead of failing every shell and
  window-manager install with `no package manager available for this platform`.
- **Package names the distribution does not carry no longer abort the whole transaction.** It was
  enough for one unknown name to take down `apt`, `pacman` or `dnf` and with it the shells that did
  exist — `zsh` and `fish` failed to install because `kubectx` or `starship` were in the same
  command. Every name in the Debian, Arch and Fedora columns was checked against a real
  distribution, and the ones that are absent are filtered and reported rather than silently
  skipped.
- **A selected shell or window manager the distribution cannot install fails with a message
  naming the routes**, instead of reporting success for a component that was never installed.
- **`--terminal=kitty` off macOS is refused** with the values that platform does support. It used
  to install nothing and log `Kitty already installed` on a machine where Kitty was not installed.
- **Installing a configuration now prunes what the repository no longer ships.** Directories were
  merged and never pruned, so a file a newer version dropped stayed on disk forever, and when it
  had been replaced by a differently named one both loaded at once.
- **The font step no longer leaves its 347 MB archive** in `~/.local/share/fonts`, a directory
  `fontconfig` scans. The manual installation guide taught the same leftover and no longer does.
- **The Homebrew step no longer reports an installation that did not happen.** The download ran
  inside a command substitution, so a failed download expanded to an empty string, the shell it was
  handed ran nothing and still exited 0, and the run went on to log `✓ Homebrew installed
  successfully`. Its own log had already printed `Warning: Homebrew binary not found`.
- **A failed image build fails the E2E harness** instead of being counted as a product test
  failure. `set -e` could not catch it: the failing call sat inside an `||` list, which disables
  `set -e` for the whole call.
- **The E2E asserts the window manager it asks for.** A missing one produced no verdict at all and
  the run was counted as a pass, which is how the Fedora job stayed green while Fedora had no route
  to install the multiplexer it was asked for.
- **`docs/ROLLBACK.md` documented rollback commands that do not exist**, including `dotfiles
  restore --latest` and a recipe built on a tag that was never created. It now documents the path
  that works.
- **The manual installation guide's font instructions** no longer download an archive into the
  font directory, and the TUI installer guide's download commands use the asset names that are
  actually published.

### Documentation

- The READMEs state what installing requires and what it does, including that everything replaced is
  copied to `~/.dotfiles-backup-<timestamp>/` first, and how to try it without consequences.
- The release procedure documents the Homebrew step that actually happens: it is manual, the formula
  lives in two places, and the tap must be public for `brew` to clone it.
- A `.gitleaks.toml` records eleven verified false positives from vendored oh-my-zsh examples and the
  Vim trainer's exercise text, so a full-history secret scan is clean.

## [v0.1.0] — 2026-09-19

First release of the downstream distribution.

### Added

- **Theme coherence**: one palette, declared once in `dotfiles-zsh/.zshrc` and shared by `LS_COLORS`,
  `EZA_COLORS`, `fzf`, `bat`, `zsh-autosuggestions`, `zsh-syntax-highlighting`, the powerlevel10k
  prompt and Herdr. The terminal emulators already defined it; nothing inside the terminal used it.
- A `bat` syntax theme generated from that palette (`dotfiles-bat/themes/dotfiles.tmTheme`), because
  no published theme can match a custom palette.
- An original emblem for the installer splash and the Neovim dashboard, replacing the upstream
  project's mark, with a compact variant for short terminals and a test that guards its geometry.
- `fzf-tab` and `zsh-completions`, declared in the `Brewfile` and in the installer's package list.
- A clock and the active Node version in the shell prompt. The Node version is read from the symlink
  `fnm` already put on `PATH` (0.02 ms per prompt); powerlevel10k's own segment shells out to
  `node --version` (6.07 ms).
- The build version on the welcome screen, so a bug report carries it without needing a flag.

### Changed

- The welcome screen is height-aware: the full lockup needs 34 rows, and anything shorter gets the
  emblem without the wordmark instead of being clipped from the top.
- `cat` is now a shell function that keeps syntax highlighting for plain invocations and hands any
  invocation carrying a flag to the real binary.
- A stale `~/.oh-my-zsh` is left untouched, and `PATH` no longer gains one entry per nested shell
  from `fnm`.

### Fixed

- `grep` is no longer aliased to ripgrep. In ripgrep `-r` means *replace*, so `grep -r pattern .` ran
  a replace command; `-E`, `--include` and `-A/-B` differ too.
- `bat` no longer renders with a theme whose colours the terminal does not define.
- `--version` printed `dotfiles vv0.1.0` for a release build, because release tags already carry the
  `v` prefix and the format added another. An uninjected build now reports `dev build`.
- The installer no longer aborts when its checkout predates an asset it wants to copy. It clones the
  repository at install time, so a newer binary can meet an older checkout; the case is now skipped
  with a log line, as `.gitconfig-personal` already did.
- `FZF_ALT_COMMAND` was never read by fzf. The variable is `FZF_ALT_C_COMMAND`, so Alt+C listed files
  where only directories were expected.
- The installer's Docker test runner no longer prints the upstream project's name as block art.

### Bootstrap

- Downstream distribution from [Gentleman.Dots v2.12.2](https://github.com/Gentleman-Programming/Gentleman.Dots/releases/tag/v2.12.2)
- Full rebranding: binary → `dotfiles`, env vars → `DOTFILES_*`, package dirs → `dotfiles-*`
- Upstream tracking via `upstream-main` mirror branch
- `.downstream/version.json` for sync tracking
- Documentation: UPSTREAM.md, DOWNSTREAM.md, 3 ADRs, legal notices, operational docs
- CI/CD pipeline: Go validation, shellcheck, branding audit, upstream-watch, release workflow
- Homebrew tap: `albersg/tap/dotfiles`
- Profile system: YAML-based profiles (minimal, default, full, platform-specific)
- Threat matrix RED tests for git automation safety
- License audit: docs/audits/LICENSE-AUDIT.md

### Preserved

- Original upstream LICENSE (MIT)
- OpenCode personal config (`stow/opencode/`)
- Go module path (`github.com/Gentleman-Programming/Gentleman.Dots/installer`) — deferred rename per ADR 0002
