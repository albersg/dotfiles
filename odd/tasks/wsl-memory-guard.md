# ODD Feature Tasks: The WSL memory guard and the policy that stops the distribution from dying

## Goal
Stop a WSL install from handing the VM a memory budget that lets one runaway workload take the whole
distribution down, and make the two safeguards that were built by hand outside the repository
(`~/.cache/dotfiles-wsl/`) into artifacts the installer owns: a corrected `memory`/`processors`
policy in `.wslconfig`, and a `wsl-memory-guard` unit the WSL step installs, verifies and reports.

## Problem and rationale
Measured on 2026-10-16 on this host (WSL2, 16016 MiB Windows host, VM limit `memory=7680MB`), two
global OOMs 15 minutes apart at 11:39:48 and 11:53:54 ended the **distribution**, not just a process:
`pi` held 5287 MB then 6509 MB of RSS, the kernel killed one task, and 90 s later systemd had still
not finished stopping `init.scope` (`DefaultTimeoutStopSec`), so it SIGKILLed every task in the scope.

The memory limit that produced that budget is this repository's own policy: `PlanWSLResources` derives
`memory = host/2` (`installer/internal/tui/wslconfig.go`), which on this host is exactly the
`memory=7680MB` that was in the file. The host lost 15 minutes of work twice, and nothing in the
installer knew the failure existed.

Two `init.scope` facts make the failure reachable, and both are measured rather than assumed:

- In WSL every terminal process lands in `init.scope` (there is no logind session around a `wsl.exe`
  relay), and that scope carried `MemoryMax=infinity`. The VM's whole memory was available to one
  scope with no internal limit, so the only backstop was the VM-wide OOM.
- Writing `memory.max` on the cgroup directly **does not last**: the next `systemctl daemon-reload`
  (which any package install runs) rewrites the attribute from the value systemd knows for the unit,
  and it knows "unlimited". Measured: `memory.max` came back as `max`. The surviving form is
  `systemctl set-property --runtime init.scope`, which stores per-property drop-ins under
  `/run/systemd/transient/init.scope.d/`, survives a reload, and recomputes at every boot because
  `/run` is empty when the VM starts.

Related front, linked rather than restated: [wsl-host-derived-config.md](wsl-host-derived-config.md)
introduced the template, the host detector and the proportional policy this change corrects.

## Specs
S1. The request, verbatim: "he tenido problemas de memoria y craasheo en mi wsl, he ejecutaod esto
    para soulcionar mi porblema bash ~/.cache/dotfiles-wsl/apply-wsl-memory-guard.sh. investigalo,
    revisa memoria engram, y en el dotfiles, mejora la parte que tanto en el installer como en la s
    utitlites se hacen cambios en la cpu y mem del wsl, y aplica estas salvaguardas y reomcendaciones"
S2. `memory` policy, decided with the user and revised by them the same day: "Memoria 3/4, CPU
    como hoy: 11". Written out: `target = max(min(host - host/4, host - 4 GiB), host/2)`, rounded
    down to 512 MB, omitted below 1 GiB. On this host that is `16016 MiB -> min(12012, 11920) =
    11920 -> 11776 MB`: the four GiB reserve binds before the ratio asks for 12012, and the 512 MB
    step falls to 23. The first version of this decision was "2/3 del host, reserva 4 GiB, suelo
    50%" (10240 MB here); L16 records the revision and what prompted it. The `host/2` floor is WSL's
    own proportional default, so the plan never gives the VM less than WSL would choose by itself,
    and on a small host the floor wins over the Windows reserve (a 7 GiB host plans 3.5 GiB, leaving
    Windows 3.5 GiB rather than the promised 4).
S3. `processors` policy, decided with the user and re-confirmed when S2 was revised: "Todos menos
    uno" — `max(1, host - 1)`, so Windows keeps one logical CPU (`host 12 CPUs -> processors=11`).
    Omitted when the host is unknown. The same three quarters applied to CPUs would be 9, so the
    revision left this rule alone on purpose rather than by omission (L16).
S4. The guard's home, decided with the user: "Paso WSL + fila en Utilities" — the Configure WSL step
    installs `/usr/local/lib/dotfiles-wsl/wsl-memory-guard.sh` and
    `/etc/systemd/system/wsl-memory-guard.service`, enables and starts the unit, and verifies that
    the cap survives a `daemon-reload`; the Utilities -> WSL resources screen reports the guard's
    state and offers a row that installs or repairs it.
S5. Verbatim from the handoff: "El guard debe pasar a ser un artefacto gestionado por el instalador,
    con la misma verificación que ya hace `apply-wsl-memory-guard.sh` (sobrevive a un
    `daemon-reload`)."
S6. Verbatim from the handoff: "La pantalla Utilities -> WSL resources debería mostrar el guard y
    avisar del modo de fallo medido, no solo los tres valores de `.wslconfig`."
S7. Unchanged contracts this change must not break:
    - the three managed keys merge in place and every other line of the user's `.wslconfig` survives
      byte for byte, with a `.bak-dotfiles-<stamp>` backup before the write;
    - the installation step, the interactive script and the Utilities utility all install
      byte-identical `.wslconfig` content (`TestWSLResourceUtilityAndTheInstallerAgreeByteForByte`);
    - `--dry-run` writes no file, no backup and no record, and runs no `systemctl`;
    - detection failure still omits `memory`, `processors` and `swap` and never fails the step.
S8. The `OOMPolicy` half cannot be asked for the way the cap is, and the guard must not assume
    either way. Measured on systemd 257: `set-property --runtime init.scope
    OOMPolicy=continue` is **refused**, because `OOMPolicy` is a unit property rather than a
    cgroup attribute and `set-property` takes only the resource-control ones. The guard
    therefore tries the two routes that are scoped to `init.scope` -- `set-property`, then a
    drop-in at `/etc/systemd/system/init.scope.d/10-oompolicy.conf` -- and **reads the
    effective policy back after each**, because which one works is a property of the systemd in
    front of it rather than something this repository can know.
S9. The guard may fix its own scope; it may not silently change the whole manager. The
    manager-wide `DefaultOOMPolicy=continue` in `/etc/systemd/system.conf` reaches every unit on
    the machine, and a service whose main process is OOM-killed would no longer be marked
    failed, so its `Restart=` would not fire. When neither scoped route works the guard reports
    that remedy **with its side effect**, names the explicit `OOMPolicy=stop` a service with a
    `Restart=` needs, and exits non-zero -- it never takes that trade itself.
S10. A guard that exits non-zero must never be reported as a bare "Job failed". Measured on the
    first run of `apply-wsl-memory-guard.sh`: `set -e` ended the script at the failed
    `systemctl restart`, so the guard's own line -- the one naming the missing half and its
    remedy -- was never printed, and the user saw only `Job for wsl-memory-guard.service failed
    because the control process exited with error code`. The restart status is captured, the
    guard's report is printed either way, and every check still runs before the verdict.

## Workflow configuration
- Route: ODD; parent orchestrates, bounded writers per write unit, independent verifier at the end.
- Test-first: on. `PlanWSLResources` is pure with a clear expected outcome, and the guard's
  verification is a deterministic read-back, so RED is meaningful in both. Exact runner:
  `cd installer && go test ./internal/...`; vet/fmt: `go vet ./internal/...`, `gofmt -l internal/`.
- Risk tier: high (installs root files and enables a systemd unit; changes the memory policy of every
  WSL host that runs the installer; item 1 data/irreversible, item 5 delivery/environment).
  Independent `gentle-ai-verify` is therefore mandatory after the change's own checks.

## Tasks

### T1 — The memory and processor policy, RED first
- Reopened once, when the user revised S2 (L16): the ratio became a quarter and `5d312a9` carried it
  through the policy, its table, the fixtures, the golden, the template comment and the docs. The
  processor half was re-confirmed, not changed.
- [x] `planMemoryMB`: `target = max(min(host - host/3, host - 4 GiB), host/2)` with the existing
      512 MB stepping and the existing omit-below-1-GiB rule. `wslMemoryReserveBytes` goes from
      2 GiB to 4 GiB; add the floor as its own named constant so the two rules are separable.
- [x] `planProcessors`: `max(1, LogicalCPUs - 1)`, omitted when the host is unknown.
- [x] RED first: the policy table gains this host (`16016 MiB -> memory=10240MB, processors=11`), a
      small host where the floor wins over the reserve, a host where the reserve wins over the ratio,
      the 1-CPU host, and the unknown host. Every existing case that the new policy moves is updated
      with the reason it moved.
- [x] Update the `.wslconfig.tmpl` comments: they state the old "half of the host" and "every logical
      CPU" policy, and a template comment that lies is the defect this task exists to remove.
- Check: the focused tests are green (`go test ./internal/tui/ -p 1 -run
  'TestPlanWSLResources|TestRenderWSLConfig|TestShippedWSLConfigTemplatePreservesFixedSettings|TestMergeWSLConfig|TestParseWSLConfigValues|TestWSLAdjustValue|TestUtilitiesPanel|TestTheRecommendationComesFromTheHost|TestWSLResource|TestWSLResourcesGolden|TestRepoAssetsExist'`),
  with `GOFLAGS=-p=2 GOMAXPROCS=2`. `gofmt` clean. **The full suite is deferred to T7 on purpose**: it
  is the command that froze this machine for six minutes at 12:26, so it runs with `-p 2` and after
  the guard is in place, not before.
- Route: parent inline (one pure function and its table; a delegated writer costs more than the unit).
- Evidence: `installer/internal/tui/wslconfig.go` and `wslconfig_test.go`. RED observed with 13
  policy cases plus the render failing against the old policy, then GREEN with the new one. The 16
  GiB / 8 CPU fixture now plans `10752 MB / 7 processors / 2560 MB swap`, and the dependents
  (`util_screen_test.go`, `screen_coverage_test.go`, `wslconfig_interactive_test.go`, `wsl_test.go`,
  `view.go`'s policy sentence and `testdata/TestWSLResourcesGolden.golden`) were updated to match.
  The golden diff was read rather than regenerated blind: it showed exactly the policy change and an
  `… and 7 more` in the frame budget, which is the paragraph growing by one line.

### T2 — The guard becomes a repository artifact
- [x] `dotfiles-wsl/wsl-memory-guard.sh` and `dotfiles-wsl/wsl-memory-guard.service`, taken from the
      working `~/.cache/dotfiles-wsl/` pair, with the evidence comments kept and the paths they name
      made true: the payload is read from the checkout and installed to
      `/usr/local/lib/dotfiles-wsl/`, so the script must not depend on being run beside itself.
- [x] Register both in `repoassets.go` (`repoAssetWSLGuardScript`, `repoAssetWSLGuardUnit`) and in the
      `repoAssets` list the existence test walks, so a rename cannot silently stop shipping them.
- [x] Keep `apply-wsl-memory-guard.sh` usable as the by-hand installer: it is what a user runs when
      they do not want to run the TUI, and the incident's own recovery path.
- [x] The guard tries both scoped routes for S8's `OOMPolicy` and verifies each by read-back; it never
      writes S9's manager-wide default.
- Check: `TestRepoAssetsExist` green for the two new paths; `bash -n` clean on both scripts; the guard
  exercised against a fabricated cgroup and a stub `systemctl` on four paths.
- Route: parent inline.
- Evidence: three defects were found and fixed inside this unit, each observed rather than suspected.
  (1) The backslash-plan wrote `8409M` straight into the cgroup, which takes only bytes or the literal
  `max`: it would have failed `EINVAL` on exactly the branch that exists for when systemd refuses.
  (2) `apply-wsl-memory-guard.sh` ran under `set -e` and died at the failed `systemctl restart`, so the
  guard's own line -- the one naming the missing half and the remedy -- was never printed; the user
  saw only "Job for wsl-memory-guard.service failed". That is S10. (3) The guard claimed a transient
  scope cannot take a drop-in from `/etc`; that was an assumption, and the measurement below proved it
  false. The stub harness' four cases: systemd accepts the property (exit 0, no file written), systemd
  refuses the cap (exit 1, "NOT durable"), drop-in honoured (exit 0, file written), drop-in ignored
  (exit 1, remedy printed).

### T3 — The WSL step installs and proves the guard
- [x] `installWSLMemoryGuard(stepID)` in the TUI: install both files through the existing
      backup/direct-write/`sudo` escalation, `daemon-reload`, `enable`, `restart`, then read
      `init.scope/memory.max` back, compare it with the expected bytes, run a second `daemon-reload`
      and require the value to be unchanged.
- [x] Degrade honestly and never fail the step: no `/run/systemd/system` (systemd is not the init, as
      `dotfiles-wsl/wsl.conf`'s `[boot] systemd=true` still requires), a refused `set-property`, or a
      cap a reload clears each produce their own line saying what was observed, and the step
      continues. The step's `--dry-run` path writes nothing and runs no `systemctl`.
- [x] Every host effect goes through one injected seam (the command runner, the cgroup reader, the
      unit-path lookups), in the style `hostResourceProbe` already uses, so no test needs systemd,
      root, or a real cgroup.
- Check: step tests for installed/absent systemd/refused property/cap-cleared-by-reload/dry-run;
  `go test ./internal/tui/ -p 1 -run WSL|Guard` green.
- Route: parent inline, same file as T1's neighbours but a different write unit.
- Evidence: delegated to a bounded writer, then reviewed and accepted by the parent.
  `installer/internal/tui/wslguard.go` carries `installWSLMemoryGuard` and `readWSLGuardState` plus one
  `wslGuardProbe` seam (`getenv`, `install`, `readFile`, `runSystemctl`, `systemdRunning`) with a real
  default, in the shape `hostResourceProbe` already uses; `stepInstallWSLConfig` calls it after the
  `wsl.conf` half and never fails on it. The mode trap was real: `applyArtifactContent` wrote `0644`,
  which is not an executable payload for systemd, so it now delegates to `applyArtifactContentMode`
  and the default stayed `0644` for every existing caller -- `TestApplyArtifactContentModeWritesTheModeItIsGiven`
  pins both. **A defect in my own earlier work was found here and fixed**: the payload was
  `-rw-r--r--` in the checkout, and `TestEveryProgramTheRepositoryShipsIsInstalledExecutable` derives
  the requirement from the file's own `#!`, so it went RED -- `chmod +x` on both `dotfiles-wsl`
  scripts, then GREEN. The worker also reported four `ghostty` failures in the same package; those
  are environmental and pre-existing, not caused by this change: `stubGhosttyCommands` builds
  `PATH` as the stub directory **plus `/usr/bin`**, where this machine has a real ghostty, so the
  step reports it already installed and the download stub is never reached.

### T4 — The interactive route installs the same guard
- [x] `getWSLConfigScript` gains the guard half, so the route the TUI actually takes (where
      `/etc/wsl.conf` needs `sudo`) does not silently skip the safeguard; the script reproduces the
      same install-and-verify sequence and the same honest failure lines.
- [x] Tests: the script contains the two install destinations and the read-back; a non-WSL host
      produces no script; the existing byte-identical `.wslconfig` assertion still holds.
- Check: `go test ./internal/tui/ -p 1 -run WSL` green.
- Route: parent inline.
- Evidence: `install_artifact` in the generated script takes a mode, so the same install half runs on
  the route a user actually takes and the two agree about `0755`/`0644`; the guard half is behind the
  same `--dry-run` gate as the rest of the script. `TestWSLConfigInteractiveScriptPassesShellcheck`
  (shellcheck 0.10.x present) and the byte-identical `.wslconfig` guard still pass, and the tests
  drive the script against a stub `systemctl`/`sudo` under `t.TempDir()`, so no test touches `/etc`,
  `/usr`, the real cgroup or systemd. Identical wording between the Go step and the script is **not**
  claimed: S7 requires the `.wslconfig` bytes to match, and they do.

### T5 — The Utilities screen shows the guard and the measured failure mode
- [x] Extend the utility's state with what the guard holds today: whether the unit file is present,
      whether the unit is active, and the live `init.scope` `MemoryHigh`/`MemoryMax` in bytes beside
      the VM's own total. A read that cannot answer says why, in the screen's own words.
- [x] A new row that installs or repairs the guard, running T3's function off the update loop, and a
      description paragraph that names the measured failure mode (how a scope with no limit let one
      workload end the distribution) and what the guard changes about it.
- [x] S6's warning: when the guard is not installed or its cap is not what it claims, the body says
      so in the same paragraph the three `.wslconfig` values are read in, so the failure mode is
      visible without opening the journal.
- Check: golden refresh plus the screen tests; `TestEveryScreenFitsEveryTerminalSize` green (the new
  paragraph and row are the frame-budget risk).
- Route: parent inline.
- Evidence: `wslResourceState.Guard`, filled by the same command the section already runs, and
  `wslGuardParagraphs` / `wslGuardFailureMode` which name each missing half in the terms it was
  measured in rather than printing one generic warning. `wslGuardRepairRow` sits in its own group
  after the write row, and the three value rows keep their indices because the arrow keys adjust the
  value under the cursor **by index** -- `TestWSLResourceRowsOfferTheGuardRepair` holds that, so a
  later row inserted before them cannot silently adjust the wrong value. The panel label
  (`missing` / `not running` / `unprotected` / `active`) puts the state on the section itself, before
  the row is opened. Seven tests added in `wslguard_screen_test.go`; the frame matrix, the panel and
  the screen-coverage guards are green, and the golden was regenerated and its diff **read**: it holds
  T1's policy change plus the guard paragraph and the repair row, and nothing else.

  The golden exposed a real contract conflict rather than only a snapshot change: inserting the guard
  paragraph before "how to apply the change" means a short frame now cuts the `wsl --shutdown`
  instruction, which the function's own comment promised never to cut. The comment was corrected to
  state the new order **and why**: the apply instruction has a second channel (`writeWSLResourceDraft`
  returns it in the write notice, exactly when it matters), and the guard's warning has none. A guard
  row's install also needed one seam, `wslGuardInstaller`/`wslGuardReader`, for the same reason
  `stepExecutor` is one: the row's test must observe the press without writing to `/etc`, and the two
  are separate so a test can prove the notice reports the **read** rather than the install's word.

### T6 — Documentation and changelog
- [x] `docs/tui-installer.md`: the WSL resources section (the policy table, the guard, the new row and
      the failure mode) and the installation-flow table's Configure WSL row.
- [x] `docs/manual-installation.md`: the same policy, and the by-hand guard recipe.
- [x] `CHANGELOG.md` entry in the existing Keep-a-Changelog style.
- Check: no stale "half of the host" or "every logical CPU" instruction survives anywhere in the repo.
- Route: parent inline.
- Evidence: the policy table in **both** documents was still the pre-T1 rule -- "half the host, capped so
  Windows keeps at least 2 GiB" and "every logical CPU the host reports" -- which is the defect class
  this front exists to remove, and it had been sitting in the two documents a user would read while
  the code said something else. Both tables now state the shipped arithmetic, with the measurement
  that changed it and the guard that holds it. `docs/tui-installer.md` gained **The WSL memory guard**
  (what each of the four limits is for, why `OOMPolicy` needs a drop-in, why the manager-wide default
  is refused on purpose, and the read-back verification); `docs/manual-installation.md` gained the two
  guard artifacts in its table, the by-hand `sudo bash dotfiles-wsl/apply-wsl-memory-guard.sh` recipe
  and the four commands that verify the result. The step table's Configure WSL row names the guard.
  Verified with the four documentation tests that exist and pass, plus `gofmt` and `go vet` clean.

### T7 — Independent verification and close
- [ ] `cd installer && go test ./...`, `go vet ./internal/...`, `gofmt -l internal/`.
- [ ] Independent `gentle-ai-verify` over the whole diff, per S2-S7, with the spec file named.
- [ ] Record every failed, skipped or unverified check here honestly.
- Route: delegated verification; parent spot-checks the evidence.
- Evidence: in progress. `gofmt -l internal/tui/` clean and `go vet ./internal/tui/` exit 0 -- after a
  formatting defect **of mine** was found and fixed: moving those strings out of a composite literal
  into `append` arguments changed how gofmt normalises the `+` that joins them (HEAD's `view.go` was
  gofmt-clean, so the regression was mine, and `-w` restored it). The full-module suite runs bounded
  to two processes (`-p 1`, `GOMAXPROCS=2`) in the background, **gated on the machine being idle**:
  two other sessions were running `go test ./internal/tui/` at full parallelism in their own worktrees
  (`/tmp/verify3.*`, `.herdr/worktrees/*`) while the scope sat at 8012 MiB of its 8409 MiB cap, and
  adding a third heavy run there is the load that produced the incident. Starting a wait-then-run job
  rather than reporting it is deliberate: the wait condition is inside the command, so its exit is
  the event. The independent verifier is launched once that run lands, never concurrently with it.

## Acceptance criteria
1. [x] `memory` follows S2 and this host plans exactly `11776MB`.
2. [x] `processors` follows S3 and this host plans `11`.
3. [x] The guard ships in the repository, the WSL step installs and verifies it, and a cap that does
       not survive a reload is reported rather than claimed.
4. [x] The Utilities screen shows the guard, the measured failure mode, and a row that repairs it.
5. [x] S7's four contracts still hold, each with a test that would fail if it stopped holding.
6. [x] Focused and full Go tests, `go vet` and `gofmt` pass, with skipped checks recorded. The
       module run landed with five failures: one is this front's (five environment hooks the branding
       guide did not name, fixed in the fifth commit) and four are the environmental Ghostty tests,
       recorded in L14 rather than fixed. `gofmt` is clean and `go vet ./...` exits 0 on the whole
       module; the independent verifier ran no Go test at all (L12).
7. [x] No push and no pull request without a separate explicit decision. Asked, and the answer was
       to merge `main`, push and open the PR (#343) -- L17.

## Log
- L1 (verbatim, 2026-10-16): "he tenido problemas de memoria y craasheo en mi wsl, he ejecutaod esto
  para soulcionar mi porblema bash ~/.cache/dotfiles-wsl/apply-wsl-memory-guard.sh. investigalo,
  revisa memoria engram, y en el dotfiles, mejora la parte que tanto en el installer como en la s
  utitlites se hacen cambios en la cpu y mem del wsl, y aplica estas salvaguardas y reomcendaciones"
- L2: three product decisions were put to the user and answered — memory "2/3 del host, reserva
  4 GiB, suelo 50%", processors "Todos menos uno", guard "Paso WSL + fila en Utilities". They are
  S2, S3 and S4 above.
- L3: Engram was searched as the request asked, with `engram search` over `wsl`, `wsl memory guard`,
  `wslconfig`, `init.scope`, `OOM killer distribution crash`, `7680` and `memory guard systemd`, in
  this project and with `--all`. **No memory of the 2026-10-16 incident exists**, which is why the
  handoff file is the only record; the search results and this gap are added to Engram as part of
  this front so the next session does not have to re-derive them.
- L4: the live guard was inspected rather than described: `wsl-memory-guard.service` is `active
  (exited)`, and `init.scope` carries `MemoryMax=8817475584` bytes (8409 MiB) on a VM whose
  `/proc/meminfo` reports 9945 MiB, which is `total - 1536 MiB`. The handoff's numbers are confirmed.
- L5 (the freeze is closed): the corrected guard was applied with the user's `sudo`. Measured after:
  `MemoryHigh=infinity`, `MemoryMax=8817475584`, `MemorySwapMax=536870912`, `memory.high = max`,
  `memory.oom.group = 0`. **The durable part is the evidence, not the claim**: systemd stores
  re-applied values as drop-ins, and all three are on disk in
  `/run/systemd/transient/init.scope.d/` (`50-MemoryHigh.conf` = `MemoryHigh=infinity`, and the other
  two), so every `daemon-reload` re-applies the numbers rather than clearing them. The user's
  `go test`-at-12-vCPU freeze can no longer repeat.
- L6 (the finding that only a real run could produce): the first application **failed on purpose**.
  `set-property --runtime init.scope OOMPolicy=continue` is **refused** by systemd 257 -- `OOMPolicy`
  is a unit property, not a resource-control one -- and the guard exits 1 rather than claiming a
  protection it does not have. So the unit is `failed` while the three memory protections are already
  in place, which is the honest state and not a broken one. S8, S9 and S10 were written from this.
- L7 (**measured, and it contradicts an assumption this repository had written down**): a drop-in
  scoped to the scope -- `/etc/systemd/system/init.scope.d/10-oompolicy.conf`, `[Scope]`
  `OOMPolicy=continue` -- **is honoured**, even though `set-property` refuses the same property.
  Confirmed by read-back: `systemctl show init.scope -p OOMPolicy --value` returns `continue`, the
  file is on disk for systemd to re-read on every reload, and the unit now reports
  `status=0/SUCCESS`. This is the mechanism T3 and T5 must rely on for the drop-in route, and it must
  not be re-litigated as impossible.
- L8 (closed): all four protections verified independently of the guard's own report --
  `OOMPolicy=continue`, `MemoryHigh=infinity` (`memory.high` = `max`), `MemoryMax=8817475584`,
  `MemorySwapMax=536870912`, `memory.oom.group=0` -- with the unit `enabled` and `active`, so it
  survives a reboot. The by-hand path and the installer path now share one artifact, and the
  `~/.cache/dotfiles-wsl/` copy is superseded.
- L9 (**the strongest evidence in this file, and it is not about the future**): the guard's own
  counters, read from the kernel with no sudo, show it working under real load. Since the VM booted
  at 12:50 -- which is when the guard was installed -- `init.scope` reports `oom 16`, `oom_kill 8`,
  `oom_group_kill 0`, and `high 0`, with `memory.current` at 8012 MiB against `memory.max` of
  8409 MiB. **Eight individual tasks were OOM-killed and the distribution is alive.** Under the old
  `OOMPolicy=stop` the first kill would have made systemd stop the scope and taken every process with
  it, which is exactly the 11:39:48 and 11:53:54 pair. `high 0` is the other half: the throttle that
  froze this VM for six minutes never fired, so that failure mode is closed as well. The load that
  produced the kills is other worktrees running `go test ./internal/tui/` at full parallelism, which
  is the same shape of load that caused the incident.
- L10 (a deviation, recorded rather than smoothed over): the T5 note above asks for the live
  `init.scope` values "beside the VM's own total", and the shipped screen reports what the guard
  holds -- `OOMPolicy`, `memory.high`, `memory.max`, `memory.swap.max`, whether the two files are
  installed and whether systemd is running the unit -- without printing the VM's total. The reason is
  that the payload derives the cap from `/proc/meminfo` and both the guard and the step prove it by
  reading the cgroup back; a screen that recomputed the total would be a second implementation of the
  arithmetic T1 exists to keep in one place. The numbers a user needs to check are all there; the
  input to them is not recomputed.
- L11: T6 also corrected documentation that had been stale since T1. The policy table in
  `docs/tui-installer.md` **and** `docs/manual-installation.md` still described the previous rule
  (half the host, a 2 GiB reserve, every logical CPU), so a user reading either document while the
  code did something else was reading a lie of exactly the class this front removes. Both tables now
  state the shipped arithmetic, and the CHANGELOG gained an entry for the policy change and one for
  the guard.
- L12: T7's independent verifier returned its verdict: **S1-S10 satisfied**. It recomputed the
  arithmetic itself from `MemTotal` (16016 MiB -> ratio 11195995478, room 12499025920, floor
  8396996608, target 11195995478, /512 MiB = 20 steps = 10240 MB; `max(1, 12-1)` = 11) and re-ran the
  payload against a fabricated cgroup and a stub systemctl (accept / refuse-cap / drop-in honoured /
  drop-in ignored, plus the boundaries). It also **falsified a claim this document makes**, and two
  smaller defects, which is what a verifier is for:
  - F1, real and change-caused: `README.md:296` still carried the pre-T1 policy -- "half the host's
    RAM ... (never so much that Windows keeps under 2 GiB), every logical CPU". L11 above claims the
    stale instruction survives "nowhere"; the main user README is exactly where it survived, and
    README is the first document a user reads.
  - F2: the comment on `installWSLMemoryGuard` said the only error it returns is a packaging one.
    That is false -- a `probe.install` the host refuses, such as a denied sudo write, returns one too.
  - F3: `shellcheck -s bash` on the payload reported SC2209 at `want_policy=continue`. A false
    positive, but the one script this distribution installs into `/etc` should be clean.
  It ran **no Go test and no `go vet`**: `init.scope` stood at 7919 MiB of its 8409 MiB cap, above
  the ~7800 MiB gate it set for itself, and it refused to be the load that reproduces the incident.
  Recorded as unrun, never as passed.
- L13: F1, F2 and F3 are fixed in the front's third commit. The README now states the shipped rule in
  the same words the TUI uses; the comment now says what the function actually returns; the payload
  quotes the value and `shellcheck` exits 0. The verifier's script-level probes are committed as
  `wsl_memory_guard_script_test.go`, together with the two boundaries it described but did not write,
  so the payload's half of S8, S9 and S10 are a test rather than an argument. Both run and pass.
- L14: T7's module-wide run landed, and it found a real defect of this front that neither the targeted
  runs nor the independent verifier could catch. Five failures in total:
  - `TestBrandingDocNamesEveryInstallerEnvironmentVariable`, this front's: the test reads the
    `DOTFILES_*` names out of the installer's own source and requires the branding guide to name
    each one. This front added five hooks -- `DOTFILES_WSL_GUARD_SCRIPT`, `DOTFILES_WSL_GUARD_UNIT`,
    `DOTFILES_WSL_GUARD_CGROUP`, `DOTFILES_WSL_GUARD_SYSTEMCTL` and `DOTFILES_WSL_SYSTEMD_DIR` -- and
    documented none of them, so a reader who followed the source to `docs/BRANDING.md` found no entry
    and could not tell a switch to set from a hook to ignore. Fixed in the fifth commit. The suite is
    the reason this was caught at all: the verifier could run no Go test (L12) and the targeted runs
    stopped at `./internal/tui/`, never entering `cmd/dotfiles` where the guard lives.
  - the four Ghostty tests (`TestStepInstallTerminalGhostty*`, `TestInteractiveGhostty*`), which are
    environmental and predate this front: this machine has a real `/usr/bin/ghostty` (27 MB, root,
    2026-04-01), so the step finds the terminal present and reports "Ghostty already installed" where
    the test expects a download. Recorded and not "fixed": nothing in this change touches that step,
    and the failure is a property of the machine, not of the change.
  Checks observed: the focused `-run` groups and the documentation group pass at
  `-p 1`/`GOMAXPROCS=2`; `gofmt -l installer/internal/` is empty; `go vet ./internal/tui/
  ./cmd/dotfiles/` exits 0, and so does `go vet ./...` on the whole module, in one second against a
  warm build cache. That suite run ended with the scope at 8211 MiB of its 8409 MiB cap, `oom_kill`
  still 10 and `high 0`: the guard held the distribution through the same shape of load that produced
  the incident.
- L15: The native review was attempted and the provider refused it twice, for two different reasons.
  Both are recorded, because the second is a fact about this front rather than about the transport:
  - The first START was rejected before any authority existed (`lenses-invalid`,
    `lineage_created: false`, `mutation_performed: false`): the contract caps the lens reason at 500
    bytes on one line and this front's was 941. That limit is written down in
    `gentle-pi/docs/readme-reference.md:809` and its negative cases are pinned in the package's own
    test, so it was a limit to read, not to discover.
  - The second START, with a 462-byte reason, was refused **terminally**:
    `lens_context_budget_exceeded` -- "The candidate's complete reviewer evidence exceeds the native
    context budget, so no review authority was created". `mutation_performed: false`,
    `retry_safe: true`, and the cause states that retrying this exact candidate cannot succeed. The
    provider's own continuation is to split the work into smaller candidates that each fit under the
    budget and to refresh the transition with `review status --next-transition` for the reduced scope.
  Measured before deciding: this front is 28 files, 3012 insertions and 157 deletions, and its single
  largest commit is 23 files and 2399 insertions. Because a candidate is the committed range from
  `base-ref` to `HEAD`, the only slices reachable without moving `HEAD` are prefixes, and the prefixes
  that fit under the budget exclude the commit that carries the risk.
  **Decision (the user's, asked and recorded): close the front here**, without rewriting the history
  or forcing a slice that would review documentation instead of the code that writes to `/etc`. The
  outside view this front did get was the independent verifier's, and it earned its place: it
  falsified the README claim at L11 and found the false comment and the shellcheck warning. No review
  authority exists or is left hanging: both refusals created no lineage, so there is nothing to
  abandon, quarantine or repair.
- L16: The fraction S2 states was revised by the user after this front had closed, so S2, S3 and
  criterion 1 were rewritten and T1 was reopened rather than a new front being opened. Their words,
  verbatim: "avanza con el installer, con la customizacion del wsl, estudia si podriamos mejor
  dejarlo como esta, pero que pueda usar 3/4 de los recursos de memoria y cpu, en vez de la mitad",
  answered as "Memoria 3/4, CPU como hoy: 11" after the study was put to them. The study's one
  non-obvious fact: `DetectHostResources` does not read the VM's `/proc/meminfo`, it asks Windows
  through `powershell.exe`, so the fraction is over the real 16016 MiB / 12-CPU machine and not over
  what the VM sees.
  - `5d312a9` applies it: `wslMemoryRatioDivisor` 3 -> 4, this host `10240MB -> 11776MB`, processors
    untouched at 11, and the small hosts the reserve decides unchanged (2 GiB -> 1024, 4 GiB -> 2048,
    8 GiB -> 4096).
  - The revision exposed a defect that is the reason it is written down: `view.go`'s own sentence --
    "Memory is two thirds of the host's RAM" -- was not carried with the arithmetic, and no test held
    the sentence and the divisor together, so the whole suite stayed green while the WSL resources
    screen told the reader a proportion the code no longer applied. Fixed, and
    `TestTheScreenNamesTheFractionThePolicyApplies` now ties the sentence's words to
    `wslMemoryRatioDivisor` so the next revision cannot leave the same lie behind. The same commit
    had also grown a second `### Changed` block in the CHANGELOG and an entry claiming the old
    fraction in the same unreleased version; both are fixed with it.
  - `3a4e138`, immediately before it, closes the TUI temporary-file leak the earlier measurement
    pointed at: the state directory, the rendered `.wslconfig` and every Alacritty build tree were
    left behind. Two of the three fixes had RED; the third has none because
    `internal/system.RunWithLogs` is a `func` and that path has no seam to fail through, which the
    commit states rather than inventing one.
- L17: Delivery was asked for and given separately from the work. The question "has hecho push y pr?"
  was answered with the measured state -- no upstream on the branch, `gh pr list --head` empty, tree
  clean -- and with the finding that mattered: `origin/main` was 34 commits ahead and a merge
  conflicted in `view.go`. The user chose **merge main, push, and open the PR to main** out of the
  routes offered.
  - `1936f46` is the merge, parents `5d312a9` and `7906f06`. One conflict, resolved by taking main's
    shorter Utilities row description -- PR #340 owns that section's body, and the two facts the
    shorter sentence drops are already in the WSL resources panel where the user acts on them -- and,
    in the same commit, four defects the integration exposed in this front's own work: the "two
    thirds" sentence in `view.go` that no test tied to the divisor, the duplicated `### Changed`
    block in the CHANGELOG together with an entry claiming the superseded fraction and its 10240,
    the one-word-short anchor `#wsl-resources`, and two files `gofmt` was unhappy with, one of them
    joined by the conflict resolution itself.
  - Verified after the merge, reading the whole log rather than its tail: `gofmt -l .` empty,
    `go vet ./...` exit 0, and `go test ./... -count=1 -p 1` (3m45s) green on every package except
    the four environmental Ghostty tests, whose cause is in their own output ("Ghostty already
    installed", because `/usr/bin/ghostty` is a real binary here).
  - The push was rejected once with a bare `(failed)` and no reason, which is recorded as what it was
    -- a transient remote refusal from which nothing landed -- and succeeded on the retry:
    `origin/feat/wsl-memory-guard` is `1936f460d110180ab3dd038df5ff3e6ff1b1dc58`.
  - **PR #343**, https://github.com/albersg/dotfiles/pull/343: 31 files, 3334 insertions, 218
    deletions, base `main`. It links no issue, and that is the target's policy rather than an
    omission: `CONTRIBUTING.md` asks for a conventional branch, tests, `go build` + `go vet`, docs
    and conventional commits, all of which hold, and this repository has no `status:*`/`type:*`
    labels for a Gentle-owned issue-first rule to attach to -- only the nine defaults. The PR body
    states it instead of the omission being papered over.
- L18: The high-risk tier put an independent verifier on the delivered change, and it earned its
  place twice over: it found two blockers no test in the suite could see, because what the suite held
  was the arithmetic and never the arithmetic against the words describing it.
  - **B1, the screen promised memory the plan does not keep.** The sentence in `view.go` stated both
    memory rules -- Windows keeps 4 GiB, and never below half the host -- without saying which wins
    where they disagree, which they do below 8 GiB, because the `host/2` floor is the larger of the
    two there: on a 6 GiB host the plan gives the VM 3072 MB and leaves Windows 3 GiB. The template's
    own comment had resolved that since T1 and the screen had not. Fixed in the sentence and in the
    two documentation tables carrying the same phrasing, and pinned by
    `TestTheScreenDoesNotPromiseWindowsMemoryThePlanDoesNotKeep`, which plans {4,6,7,8,16,32} GiB and
    requires the clause that decides when a host leaves Windows under 4 GiB -- and fails rather than
    passes if the set ever stops containing such a host. The test T6 added tied the sentence's words
    to the divisor; nothing tied the sentence to a plan, which is the gap this closed.
  - **B2, the recovery script printed the superseded fraction.** `dotfiles-wsl/apply-wsl-memory-
    guard.sh` told its reader to expect `memory=10240MB`, `~9.6Gi` and `~8380M`, which is what the
    two-thirds rule planned; under S2 the values are whatever `.wslconfig` sets and the guard's own
    output carries them, so the three lines say that now.
    `TestTheByHandScriptQuotesThePolicyItApplies` bans both those literals and any `expect <number>`,
    because an expectation with a number in it is an expectation that goes stale the next time the
    fraction moves. Why the earlier sweep missed the file is recorded rather than excused: the
    inventory grep was `\b10240\b` and this file writes `10240M`, where the word boundary never
    matches.
  - Both were red first, with the arithmetic in the failure message, and the golden was regenerated
    for the one line the longer sentence moved (`… and 13 more` -> `… and 14 more`) with the diff read
    rather than trusted. `-update` is the flag; `UPDATE_GOLDEN` is not a mechanism this library has,
    which a wasted attempt established.
  - The verifier also confirmed what a machine can be made to say without being broken: the guard's
    four limits and its four S8/S9/S10 paths against a fabricated cgroup and a stub `systemctl`, the
    boundaries (no controller, unreadable `MemTotal`, under 4096 MiB, exactly 4096 MiB), `shellcheck`
    and `bash -n`, and that the heredoc round-trips `.wslconfig` bytes including `$HOME`, backticks
    and a backslash. It could not run the by-hand script under real sudo and systemd, and reported
    that as unverified instead of passing it: true then of this front, and still true.
  - **The native review was refused terminally for the third time, and this time on the correctly
    chosen candidate.** `assess` over the PR slice (`origin/main..HEAD`) reports `risk: high` -- the
    `executable_mode` signal on the by-hand script -- with `reviewDue: true` and a plan requiring an
    independent verifier, so the route was followed rather than argued with. `inspect` offered
    `review.start` at `base-ref=7906f06`, the merge base with main, 31 paths, and zero lineages; the
    START was run with the provider's own tokens and four lenses and refused with
    `lens_context_budget_exceeded`, `mutation_performed: false`, `next_action: stop`. No lineage was
    created, so nothing has to be abandoned or quarantined. The provider's only continuation is to
    split the change into smaller reviewable candidates -- the route offered as three chained PRs and
    declined when the single PR was chosen -- so it went back to the user as the decision it is
    instead of being applied on this front's own authority.
