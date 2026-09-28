# ODD Feature Tasks: Host-derived WSL configuration

## Goal
Make the machine-dependent parts of the WSL configuration (`memory`, `processors`, `swap` in
`.wslconfig`) derive from the resources of the Windows host the installer runs on, instead of
shipping the frozen values of one development machine (6 GB / 8 processors / 4 GB swap, commented
as "Host: 15.6 GB / 10 cores"). The artifact becomes a template rendered at install time.

## Problem and rationale
`dotfiles-wsl/.wslconfig` currently carries hard-coded limits that were tuned for a single host and
are copied verbatim by `stepInstallWSLConfig` through `applyArtifact`. Any other machine receives
someone else's memory and CPU budget: a 4 GB laptop gets a 6 GB VM limit, and a 32 GB workstation
keeps 6 GB. `docs/manual-installation.md` already documents the fallout ("carries machine-specific
limits; adjust them to your host before applying it somewhere else").

Measured on the WSL2 host used for this work:

- `MemTotal: 6071484 kB` (~5.79 GiB) matches the applied `memory=6GB`, not the 15.6 GB host.
  Memory visible from inside the distribution is bounded by the file being generated, so the host
  total is **not derivable from `/proc/meminfo`**.
- `nproc` reports 12 while the file declares `processors=8`, so the CPU count seen from inside is
  not a faithful reading of the key either.
- `powershell.exe` and `cmd.exe` are reachable under `/mnt/c`, so the host can be queried through
  interop.

## Accepted decisions
- **Mechanism: render inside the installer.** `dotfiles-wsl/.wslconfig` becomes
  `dotfiles-wsl/.wslconfig.tmpl`; the WSL step detects the host, applies the policy, renders, and
  writes the result. No standalone tuning script was requested.
- **Policy: WSL proportional defaults.** `memory = 50%` of host RAM, `processors =` all logical
  CPUs, `swap = 25%` of the VM memory.
- **Detection: Windows interop first.** Host capacities are read by executing a single non
  interactive `powershell.exe` query. `detect.go` has no CPU/RAM fields today and gains a dedicated
  host-resource type.
- **Fallback: omit, never guess.** When detection is unavailable or fails, the template omits
  `memory`, `processors`, and `swap` entirely. WSL then applies its own proportional defaults, which
  are computed from the real host by Windows and are strictly better than a hard-coded guess.
  Detection failure degrades the feature; it never fails the step.
- **No user-facing policy knobs.** Only test/override environment variables in the style of the
  existing `DOTFILES_WSL_*` variables.
- Estimated size: ~350-450 authored diff lines across code, tests, docs, and the template.

## Workflow configuration
- Route: ODD; one bounded writer per write unit, parent orchestrates.
- Test-first: on. `PlanWSLResources` is a pure function with a clear expected outcome, so RED is
  meaningful. Exact runner: `cd installer && go test ./internal/...`.
- Delivery strategy: `ask-on-risk`; the commit policy is a blocking parent decision (see T5).

## Tasks

### T1 — Host-resource policy as a pure function
- [x] Add a pure `PlanWSLResources(host HostResources) WSLResources` covering memory, processors,
      and swap, with no environment or filesystem access.
- [x] Policy: `target = host/2`; clamp to `host - 2 GiB` when the reserve is smaller; round down to
      512 MiB; omit `memory` below 1 GiB. `processors = host logical CPUs`, omitted when unknown.
      `swap` = 25% of `memory`, rounded down to 512 MiB, omitted when it rounds to zero.
- [x] Table-driven tests: 4 GB, 6 GB, 15.6 GB, 32 GB, and 64 GB hosts; unknown-host case; the
      Windows-reserve clamp; the sub-1 GiB floor.
- Check: 12 policy cases pass (`go test ./internal/tui/ -run WSLResources`); `gofmt`/`go vet` clean.
- Route: delegated writer; multi-file write trigger.
- Evidence: `installer/internal/tui/wslconfig.go` and `wslconfig_test.go`. Independent
  `gentle-ai-verify` re-derived all eight required cases by hand and agreed with them, then found
  the sub-1 GiB floor reachable by no case; the 2.5 GiB case that now exercises it was confirmed by
  mutating the check and observing only that case fail. Commit `5b933c2`.

### T2 — Windows host detection through interop
- [x] Add host detection that runs one non-interactive `powershell.exe` query for total physical
      memory and logical processor count, with a short timeout and lenient parsing.
- [x] Inject the command runner, the PATH lookup, and the clock through one probe seam, following
      the parameter-injection style of `classifyLinux`. `system/exec.go` calls `exec` directly and
      has no reusable convention, so a new local seam was the correct choice over inheriting one
      that tests could not use without spawning a process; expose `DOTFILES_WSL_HOST_MEMORY_MB` and
      `DOTFILES_WSL_HOST_CPUS` overrides in the established `DOTFILES_WSL_*` style.
- [x] Return an empty result (not an error path that fails the step) when the binary is missing,
      times out, or prints something unparseable.
- [x] Tests: parsed success, missing binary, timeout/garbage output, override precedence.
- Check: 26 detection results pass; no test spawns a real Windows binary.
- Route: delegated writer; same write unit as T1.
- Evidence: `installer/internal/system/host.go` and `host_test.go`. The production query was run by
  hand against this host and returns `cpus=12` / `memmb=16016` with CRLF, matching the 15.6 GiB /
  12-thread host. Independent verification found `WaitDelay` unset, so the deadline bounded the kill
  but not the pipe wait; it is now 1 s and asserted to stay below the timeout. Commit `5b933c2`.

### T3 — Render the template in the WSL step
- [x] Convert `dotfiles-wsl/.wslconfig` into `dotfiles-wsl/.wslconfig.tmpl` keeping every fixed key
      (`networkingMode`, `dnsTunneling`, `autoMemoryReclaim`, `sparseVhd`) and its explaining
      comments; only the three machine-dependent values become template actions.
- [x] Render with `text/template` in `stepInstallWSLConfig` and write the rendered bytes through the
      existing backup/`sudo` escalation path, without changing that path's behavior.
- [x] Log the detected host, the derived values, and whether they came from detection or from the
      omitted-keys fallback, so a user can audit the result.
- [x] Tests: rendered output for a known host; fallback render with the three keys absent; fixed keys
      preserved; no template delimiters leak into the output.
- Check: `go test ./internal/tui/ -run WSL` → 45 passed; full `go test ./internal/tui/` → 1076 passed.
- Route: delegated writer; second write unit, together with T4.
- Evidence: `RenderWSLConfig` in `wslconfig.go`; `applyArtifact` now reads and delegates to
  `applyArtifactContent`, so rendered bytes and checkout files share the backup/direct-write/sudo
  path without duplicating it. Real-host render through the production path: 16016 MiB and 12
  logical CPUs produce `memory=7680MB`, `processors=12`, `swap=1536MB`, and the zero host omits all
  three keys while the fixed keys survive. Commit `cafd8de`.

### T4 — Update asset registration and the interactive path
- [x] Update `repoassets.go` to the template asset and keep its existence validation green.
- [x] Point the interactive shell-script path (`getWSLConfigScript`) at the same rendered artifact so
      both routes install identical content, and keep its message about an unavailable Windows
      profile.
- [x] Tests: asset list/validation test, interactive script test, and a cross-route test proving the
      step and the script install byte-identical content for the same overridden host.
- Check: focused TUI tests pass; the `interactive_dispatch_test.go` invariants hold.
- Route: delegated writer; second write unit, together with T3.
- Evidence: the script renders through the same `renderedRepoWSLConfig` helper and receives a temp
  file it removes, so the two routes cannot disagree by construction. Independent verification
  exercised the script end to end and confirmed the temp source is gone afterwards. Commit
  `cafd8de`.

### T5 — Documentation, changelog, and delivery decision
- [x] Update `docs/manual-installation.md` (the manual `cp` recipe and the "adjust them to your host"
      paragraph) and `docs/tui-installer.md` to describe host-derived rendering.
- [x] Add a `CHANGELOG.md` entry following the existing Keep-a-Changelog format.
- [x] Resolve the commit policy: the user authorised sealing the previous feature and doing
      work-unit commits per task on a new branch. That feature is committed as `37e71c1` on
      `feat/pi-agent-skills-officecli`; this feature works on `feat/wsl-host-derived-config`.
      Push and PR still need a separate explicit decision.
- Check: docs contain no stale verbatim-copy instruction; changelog entry present.
- Route: parent decision, then delegated docs writer.
- Evidence: the manual recipe now renders the template with the three host-derived keys omitted,
  which is the same supported state an unreadable host produces, and a subsection gives the policy
  plus the installer's own host query for anyone who wants the values written explicitly. A
  repository-wide rescan found no remaining reference to the deleted path outside this tracker.
  Commit `801e647`.

### T6 — Verification and reconciliation
- [x] Run `cd installer && go test ./...`, `go vet ./internal/...`, and `gofmt -l internal/`.
- [x] Render the template against this real host and compare the three derived values with the host's
      actual memory and CPU count; do not write into `C:\Users` without explicit authorization.
- [x] Record every failed, skipped, or unverified check honestly.
- Route: delegated verification; parent spot-checks evidence.
- Evidence: a final independent verifier ran the whole module (4 packages, 0 skips, all green),
  re-derived this host's values from a live query and confirmed the documented claims against the
  code. It returned PASS on all six acceptance criteria and raised one medium finding, the missing
  step-level test for the fallback, which was fixed in `5b76051`; the template comment now states the
  two clamps its sentence depended on.
- Not verified, disclosed rather than implied: the Docker E2E suite was not run, no install was
  executed against the real Windows profile or `/etc/wsl.conf`, `gofmt`/`go vet` covered `internal/`
  and not `cmd/` (which has no test files and no changes on this branch), and remote branch state was
  not checked without network access.

## Acceptance criteria
1. [x] No machine-specific WSL limit is hard-coded for other hosts in the rendered artifact.
2. [x] The three derived values follow the WSL proportional policy for the detected host.
3. [x] Detection failure produces WSL's own defaults via omitted keys and never fails the step.
4. [x] Interactive and non-interactive routes install identical content.
5. [x] Focused Go tests, `go vet`, and `gofmt` pass, with skipped checks recorded.
6. [x] No commit, push, or PR without explicit user authorization.

## Delivery and open items
- Commits on `feat/wsl-host-derived-config`, in order: `5b933c2` (policy and detection), `cafd8de`
  (template and render, both routes), `801e647` (docs and changelog), `5b76051` (fallback test and
  comment precision). Nothing was pushed and no pull request was opened, by design.
- Accepted without change, disclosed in the verifier's findings: the interactive route leaves its
  rendered temp file behind if the script dies before its `rm -f`; the step now reads and renders the
  template before the Windows-profile lookup, so a broken checkout fails earlier and also blocks the
  `wsl.conf` half, where the old code would have skipped only the Windows half.
- Open: the real Windows profile still holds the previous `memory=6GB` file, because no install has
  been run against it. Applying it is a decision for the user, and the step backs the old file up
  before overwriting it.
- Open: the interactive `sed` recipe in the manual docs is a rendering aid, not the installer's
  renderer; a future change to the template's directive names would need it updated.

## Progress
- 2026-09-21: Feature opened after the user authorized the change and chose the installer-render
  mechanism and the proportional policy. Exploration recorded the `/proc` and `nproc` measurements
  above. Tracker written before the first source write.
- 2026-09-21: Commit policy decided with the user. The previous feature was sealed as `37e71c1` on
  its own branch, and `feat/wsl-host-derived-config` was created from it with a clean tree.
- 2026-09-21: T1 and T2 implemented by one bounded writer, verified independently, and committed as
  `5b933c2`. Two medium findings from verification were fixed inside the unit before the commit:
  an unbounded pipe wait after the kill (`WaitDelay`), and a sub-1 GiB policy branch no test could
  reach. Accepted without change: the `int` conversion assumes a 64-bit build, which the code
  documents and WSL's 64-bit Windows requirement backs; the BOM trim only covers the first line and
  degrades to omitting the keys, which is the safe direction.
- 2026-09-21: T3 and T4 implemented by a second bounded writer and committed as `cafd8de`. Independent
  verification passed with no blocker and confirmed the rendered values by hand. Accepted without
  change, disclosed rather than hidden: the step now reads the template before the Windows-profile
  lookup, so a broken checkout fails earlier and also blocks the `wsl.conf` half, with no test pinning
  the old order; the interactive temp file leaks if the script dies before its `rm -f`; the docs that
  still describe the old file belong to T5.
- 2026-09-21: T5 and T6 closed. Final independent verification ran the whole module green with no
  skips, re-derived this host's plan from a live host query, and returned PASS on all six acceptance
  criteria with one medium finding: no step-level test covered the unreadable-host fallback. That gap
  is closed in `5b76051`, verified by mutating the step to fail on a zero host and watching only the
  new test fail. Feature complete on the branch; nothing pushed.
