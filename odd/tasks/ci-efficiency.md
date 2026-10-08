# Feature: pull-request feedback time, cut without losing a check

Status: in progress
Opened: 2026-09-29
Issue: #64
Owner: autonomous session, on a worktree of its own (`../dotfiles-wt-ci`, branch `chore/ci-efficiency`)
Orchestrator: the parent session merges; the writer never does.

## Why

A pull request waits eight to nine minutes for its checks, and one Docker job is almost all of it.
The suite that runs inside that job takes forty seconds locally, so the cost is the workflow, not
the tests. The complaint that opened this: two or three pushes on one branch, each of them paying
the whole matrix again while the previous run was already superseded.

## Scope

**Phase 1 is this branch: workflows and CI-facing scripts only.** No test file and no product Go, so
nothing here collides with the other fronts working in `installer/**`.

**Phase 2, after phase 1 is merged and the other fronts are quiet:** per-test timings inside the
suite, the fixed sleeps that should be condition waits, redundant guards inside `e2e_test.sh`, and
whether the largest package can be split so its tests run in parallel.

## In parallel (other worktrees)

| Front | Worktree | Files it owns |
| --- | --- | --- |
| Other installer fronts (trainer, panel, companion, ...) | `../dotfiles-wt-*` | `installer/**` |

This front owns `.github/workflows/**`, `docs/**` and this file. It touched nothing else.

## Measured baseline

Per-job durations, pull-request run 36625550831 (branch `fix/trainer-hint-coherence`):

| Job | Duration |
| --- | --- |
| Linux E2E (ubuntu) | **424 s** |
| Linux E2E (fedora) | 215 s |
| Linux E2E (arch) | 161 s |
| macOS smoke test | 83 s |
| Termux E2E (non-blocking) | 66 s |
| Go tests | 47 s |
| Go Validation | 47 s |
| Linux E2E (debian) | 36 s |
| Linux E2E (alpine) | 23 s |
| Build (macos-latest) | 35 s |
| Branding Audit | 18 s |
| Build (ubuntu-latest) | 17 s |
| Shell Validation | 15 s |
| Secret Scanning | 10 s |

The table in the issue reproduces exactly, job for job, from
`gh run view <id> --json jobs`.

Run wall clock on recent pull requests, which is what a reviewer actually waits for:

| Run | Branch | Wall clock |
| --- | --- | --- |
| 36625550831 | `fix/trainer-hint-coherence` | 481 s |
| 36624094222 | `feat/companion-v2` | 536 s |
| 36620997479 | `feat/companion-v2` (failed) | 560 s |
| 36598354604 | `feat/contextual-panel` | 526 s |

**Critical path:** the E2E workflow, and inside it the `ubuntu` Docker job. The CI workflow finishes
in 97 s and never gates anything the reviewer is waiting for. Every E2E matrix entry starts only
after `Go tests` ends, because `linux-e2e` declared `needs: go-tests`.

## Why ubuntu takes 424 s where alpine takes 23 s

Two timestamps inside one job log split the 424 s, and two set of log lines name each half.

`gh run view --job=109601822608 --log` gives, for run 36625550831:

| Phase | Start | End | Duration |
| --- | --- | --- | --- |
| Checkout | 20:20:34.5 | 20:20:37.5 | 3 s |
| Set up Go (toolcache + module cache) | 20:20:37.5 | 20:20:45.8 | 8 s |
| Build the Linux binary | 20:20:45.8 | 20:20:47.3 | 2 s |
| `docker build` the ubuntu image | 20:20:47.4 | 20:21:31.5 | **44 s** |
| `docker run` the E2E suite | 20:21:31.5 | 20:27:33.0 | **361 s** |

The same split for the other images:

| Image | `docker build` | `docker run` | Suite that ran |
| --- | --- | --- | --- |
| ubuntu | 44 s | **361 s** | full installation, 4 passes |
| fedora | 47 s | 148 s | full installation, 4 passes |
| arch | 22 s | 118 s | full installation, 4 passes |
| debian | 15 s | ~1 s | dry run only |
| alpine | 5 s | ~1 s | dry run only |

So the difference is **not** `apt update` and **not** the image build. It is what runs inside the
container:

* **alpine and debian install nothing.** Their Dockerfiles set neither `RUN_FULL_E2E` nor
  `RUN_BACKUP_TESTS`, so `e2e_test.sh` stops after three binary smoke tests and the four dry-run
  planning tests (13 assertions, 7 tests). That is the whole 23 s and 36 s.
* **ubuntu installs for real, and on Debian-family hosts the installer routes through Homebrew.**
  `installer/internal/tui/non_interactive.go` adds the `homebrew` step when
  `SystemInfo.OS` is `OSMac`, `OSDebian`, `OSLinux` or `OSWSL`, and `detect.go` classifies Ubuntu as
  `OSDebian` (`OSDebian // Debian-based (Debian, Ubuntu, etc.)`, decided by `/etc/debian_version`).
  The container log shows it happening: `[4/11] Install/Update Homebrew...`, then
  `==> Downloading and installing Homebrew...`, `==> Pouring portable-ruby-4.0.7...`, and then
  **one** bootstrap plus **251 Homebrew `Installing`/`Pouring`/`Fetching` operations** across the
  four installation passes. Fish alone drags in a whole C toolchain as bottles:
  `linux-headers@6.8, glibc, zlib-ng-compat, gmp, isl, mpfr, libmpc, lz4, xz, zstd, binutils, gcc,
  bzip2 and pcre2`.
* **fedora and arch do the same four installations through dnf and pacman**, and their logs contain
  **zero** `==>` Homebrew operations (the only "Homebrew" strings there are the advisory line
  "Install them with Homebrew (brew install) or from each tool's own upstream installer"). Hence
  148 s and 118 s instead of 361 s.

Compilations are not the cost, and neither are the tests. Downloads without a cache are: the
Homebrew bootstrap, the bottles, and the apt/dnf/pacman layer inside `docker build`, which is
rebuilt from an empty Docker daemon on every job.

## What changed

### 1. `concurrency` where it was missing, and not where it does harm

`ci.yml` had no `concurrency` at all, so every push on a branch ran the whole job set again. It now
has one group per ref, and `cancel-in-progress` only for pull requests.

`e2e-testing.yml` already had the group but cancelled unconditionally, and its group
(`refs/heads/main` for every push to main) meant a merge cancelled the previous merge's
verification. Run 36586936998 is the proof: `Linux E2E (ubuntu)` is `cancelled` at 15:09:32, five
seconds after the next push created run 36588029818 at 15:09:27. That commit's only full
apt-and-Homebrew installation was never verified by anything, and never re-ran. Both workflows now
cancel on pull requests only.

### 2. The `needs` gates that only serialised, and the checks they corrupted

`linux-e2e` and `termux-e2e` no longer wait for `go-tests`; `build` no longer waits for
`go-validate` and `branding-audit`. None of those jobs consumes another's output - the E2E jobs
build the binary from their own checkout, and the build job never read a validation artifact.

Two things were wrong with the gates:

* They cost the critical path. In run 36625550831, `Go tests` ran 20:19:41-20:20:28 while the whole
  E2E matrix sat idle and started at 20:20:31: **50 seconds of a 481-second wait** spent waiting for
  a 47-second job to finish before starting a 424-second one.
* They produced checks nobody could read. When an upstream job failed, GitHub skipped the matrix and
  listed it under its **unexpanded** name: run 36559187850 shows a `Branding Audit` failure beside a
  `skipped  Build (${{ matrix.os }})`, and a failing `Go tests` produced
  `skipped  Linux E2E (${{ matrix.image }})`. A skipped matrix is not a matrix; it is one line of
  noise where the reader expects results.

`build` also gained `fail-fast: false`, so one operating system failing no longer cancels the other
and turns a failed check into a cancelled one.

### 3. The E2E matrix: full where it matters, smaller where the coverage is already there

`linux-e2e` takes its image list from a small `E2E image list` job that resolves it in plain shell,
and the policy lives there:

* **Pull requests:** ubuntu, fedora, arch, alpine (debian excluded)
* **`main` and `workflow_dispatch`:** ubuntu, debian, fedora, arch, alpine

The list is resolved in a job rather than inside `strategy.matrix` on purpose. The shorter forms are
an expression in `exclude` or a whole-matrix expression, and the documented `exclude` examples are
static YAML lists - GitHub's own schema validation reports an expression there as a configuration
that "does not match in matrix combinations". A workflow file that fails to parse runs no checks at
all, which is a worse failure than the one this front is fixing, so the list is built where the
shell can be read and the syntax is not in question. The cost is one runner start, about seven
seconds, on a 424-second critical path.

Why debian and only debian: it is the second apt-family image, and the installer classifies both
Ubuntu and Debian as `OSDebian`, so ubuntu walks the *same* branch, the *same* `debianPackages`
list, and the *same* 240-combination dry-run matrix through the *same* binary - and then installs
for real, which debian never does. Debian's own 13 assertions are three binary smoke tests
(identical binary, also run on alpine), three option-planning tests (also run on ubuntu), and the
dry-run matrix (also run on ubuntu, same `OSDebian` path). Its one remaining difference is that
`e2e_test.sh` itself runs under `sh` (dash) instead of `bash`, and alpine covers that on pull
requests under busybox `ash`, a stricter POSIX shell.

Why nothing else was cut: fedora is the only dnf execution, arch the only pacman execution, alpine
the only apk detection and the only non-bash shell left on a pull request, and termux the only
Android/Termux detection. Each is the sole pull-request exercise of a path the installer can break.
Cutting any of them would be the false safety the issue warns about - a matrix that looks green over
a branch nobody ran.

### 4. Caches: the Go cache was already there, the Docker layers are not

The Go module and build caches are present and hitting in every job that sets up Go, via
`actions/setup-go@v5` with `cache-dependency-path: installer/go.sum`. The logs say
`Cache restored from key: setup-go-Linux-x64-ubuntu24-go-1.25.1-...` with `Cache Size: ~27 MB` in
all five E2E jobs, in `go-validate`, in both `build` jobs and in `macos-smoke`. Nothing was missing
there; the entry in the issue's list turned out to be already done, and this note records that
rather than adding a second cache on top of it.

What is genuinely uncached is the **Docker layer cache**. GitHub starts each job on a fresh daemon,
so `docker build` re-runs apt/dnf/pacman from scratch every time: 44 s (ubuntu), 47 s (fedora),
22 s (arch), 15 s (debian), 5 s (alpine) - about **133 s of runner time per run** (118 s on a pull
request, which no longer builds debian), of which 44 s is on the critical path. BuildKit can cache
those layers, but the flags belong in
`installer/e2e/docker-test.sh`, which is outside this front's edit surface. The change is written up
under "Handover" below.

Two smaller wastes were removed instead, both inside `ci.yml`: `Shell Validation` and `Branding
Audit` were running `sudo apt-get update` to install a package the runner image already has
(`shellcheck is already the newest version (0.9.0-1)`; the apt run fetched 12.5 MB of indexes in
6.4 s of a 15-second job). Both steps now install only when `command -v` does not find the tool.

## Coverage ledger

Nothing was removed from the suite. The in-container assertion count is unchanged; only the number
of distributions that run it on a pull request changed.

| What changed | Covered before by | Covered after by |
| --- | --- | --- |
| `Linux E2E (debian)` not run on pull requests (13 assertions, 7 tests) | The debian job | The same three binary smoke tests on ubuntu/fedora/arch/alpine; the same three planning tests and the same 240-combination dry-run matrix on **ubuntu**, which the installer classifies as `OSDebian` and which runs the identical code path; `sh`-hosted script execution on alpine's `ash`. **Runs unchanged on `main`.** |
| `Build (${{ matrix.os }})` phantom skipped check | Never ran, by construction | Gone: `build` no longer depends on failing jobs, so its matrix always expands and both platforms report |
| `Linux E2E (${{ matrix.image }})` phantom skipped check | Never ran, by construction | Gone: the matrix no longer waits behind `go-tests` |
| `Shell Validation` apt install | `apt-get update` + `install -y shellcheck` | `command -v shellcheck` guard; the install still runs when the image lacks it |
| `Branding Audit` apt install | `apt-get update` + `install -y ripgrep` | `command -v rg` guard; the install still runs when the image lacks it |
| E2E shell-functional check silently skipped missing fish/zsh | Command-existence guards had no `else`, so a missing shell generated no verdict while the full-install test could still pass | Missing fish or zsh now calls `log_fail`; the same E2E suite verifies shell execution when installed and reports absence otherwise |
| Cancelled `main` runs | `cancel-in-progress: true` on `refs/heads/main` | Main runs are no longer cancelled; the next run waits |

Check count per run: **14 before** (6 in `ci.yml`: Go Validation, Shell Validation, Branding Audit,
Secret Scanning, Build ×2; 8 in `e2e-testing.yml`: Go tests, Linux E2E ×5, Termux E2E, macOS smoke).
**14 on a pull request after**, and the same fourteen jobs: debian's E2E check is replaced one for one
by the `E2E image list` check that decides the matrix. **15 on `main`**, which is the fourteen plus
that resolver. Assertions inside the containers: 258 on a pull-request run before, 245 after; the 13
that difference accounts for are debian's, listed above with where each one runs instead.

## Before and after

Job durations are measured from the API. Run wall clocks after the change are **projections from
the same run**, not measurements: they are the baseline wall clock minus the measured gate delay,
because this worktree cannot push. The first real pull request on this branch is what turns them
into measurements.

| Item | Before | After |
| --- | --- | --- |
| Pull-request wall clock, run 36625550831 (projected) | 481 s | ~438 s |
| Same, for the three other recent pull requests (projected) | 526-560 s | ~483-517 s |
| E2E matrix start | 50 s after the run starts | as soon as the image list resolves, ~7 s |
| First push on a branch | 481-560 s | 481-560 s (unchanged) |
| Second and third push on the same branch | another 481-560 s each | ~0 s: the superseded run is cancelled |
| Checks per pull-request run | 14 | 14 (debian's E2E check is replaced by the image-list job) |
| Checks on `main` | 14, but cancellable mid-flight | 15, none cancellable |
| `Shell Validation` / `Branding Audit` | 15 s / 18 s | ~9 s / ~11 s |
| Uncached Docker layer time | 133 s per run, 44 s of it on the critical path | 118 s per run on a pull request, still 44 s on the critical path (see Handover) |

The single biggest win is not in the table: it is that a branch with three pushes used to sum three
full matrices - roughly 25 minutes of waiting - and now pays one.

## Self-critique

Honest leftovers, in the order a reviewer should look at them.

1. **`Go tests` duplicates `Go Validation`.** Both run `go test ./... -skip Golden`, on the same
   runner, with the same toolchain; `Go Validation`'s step even adds `-count=1`, so it cannot be
   more cached than this one. It was kept because it is the only place a test failure is reported
   without a gofmt failure in front of it (a gofmt failure stops `Go Validation` before its test
   step) and because at 47 s it is not on the critical path, which is 424 s. Merging them is a
   defensible next step; it is called out here rather than done quietly in a pull request that
   claims to have lost no check.
2. **`Termux E2E` cannot fail a pull request.** `continue-on-error: true` means a Termux regression
   reports red but does not block, and its image simulates `pkg` (the commit that added the Arch
   image says as much: "an image that fakes its package manager is how an E2E job stops proving
   anything, and the termux image already shows what that looks like"). It is a signal, not a guard.
   Its name says "(non-blocking)", so it is not lying, but a reader should know that the only
   Android path in CI is the one that cannot say no. Left as it was: making it blocking is a
   behaviour change on every pull request and belongs to the owner of that call.
3. **Three of the five images never install anything.** debian, alpine and termux are detection and
   dry-run checks. That is worth knowing when reading a green matrix: only ubuntu, fedora and arch
   prove an installation happens.
4. **`ubuntu` duration is noisy.** 424 s, 468 s, 486 s, 510 s, 624 s across recent runs with
   identical test content. Most of that variance is remote downloads from Homebrew and ghcr.io, not
   anything this repository controls.
5. **The image list costs one extra runner start.** `E2E image list` is a five-line shell job that
   `linux-e2e` waits for: about seven seconds on a 424-second critical path, under two percent. It
   replaced an expression inside `strategy.matrix` (`exclude: ${{ ... }}`), which is shorter but
   whose support is not documented for `exclude` and which the schema reports as invalid there.
   Seven seconds is cheaper than a workflow file that might not parse, because a file that does not
   parse runs no checks at all. If the extra job is ever removed, it should be replaced with the
   *whole-matrix* expression form (`matrix: ${{ fromJSON(...) }}`), which the docs do show, not with
   `exclude`.
6. **Main runs now queue instead of replacing each other.** Two merges in quick succession mean the
   second matrix waits for the first. Deliberate: the alternative is an unverified merge.
7. **`release.yml` runs no E2E at all** despite the issue's "releases with the full matrix". It
   builds and publishes four binaries on a tag. Adding an install matrix to a release is a
   behaviour change with its own risk, and this phase did not take it.
8. **`e2e_test.sh` disagrees with itself about the backup suite.** The script says
   `# Backup tests (can run in basic mode)` and then gates them on
   `RUN_BACKUP_TESTS = 1 || RUN_FULL_E2E = 1`. The debian and alpine Dockerfiles set neither, so the
   basic images never run a backup test - debian's seven tests are three smoke tests and four
   planning tests, with no backup assertion among them. Either the comment is wrong or those two
   Dockerfiles are missing a variable, and it is not this front's file to decide. Worth knowing
   because it is the difference between "basic images cover the backup path cheaply" and "nothing
   but the three full images has ever run a backup test".
9. **`upstream-watch.yml` shows a permanent `skipped  Create integration PR`** whenever upstream has
   no new tag (run 36565443368), which is the normal state. Unlike `Build (${{ matrix.os }})` it is
   not a placeholder: the name is honest and the job runs when there is something to do. Left alone.

## The Docker layer cache: measured, and deliberately not shipped

The handover below asked for a cache because GitHub starts every job on a fresh Docker daemon and the
images are rebuilt from nothing. It was implemented with BuildKit's Actions cache (scoped per image, taken
only in Actions so a local `docker build` is untouched) and then **measured**, which is the only way this
question has an answer. Three runs of the same job:

| Run | `Run Docker E2E` (ubuntu) | Image build phase | Suite |
| --- | --- | --- | --- |
| Baseline, no cache (run 36830572115) | 403 s | **32.5 s** | 368 s |
| Cold: the cache being written (run 36832989552) | 596 s | - | - |
| Warm: the same run re-run, cache readable | 451 s | **42.2 s** | 406 s |

The warm build is **slower** than the build with no cache at all, so the cache is not being reused. The
likely reason is in the export mode: `mode=min` exports only the layers of the final stage, and for these
Dockerfiles the work worth caching sits in earlier layers, so there is nearly nothing to import. `mode=max`
would export the intermediate layers and is the untested hypothesis.

Even if it worked, the prize is bounded by the image build: **32.5 s of a ~400 s job, about 7%**, while the
cold run paid **+193 s** to write a cache that did not read back in the same job, and the suite's own
run-to-run variance (±40 s, from Homebrew's network) is larger than the prize itself.

So it was reverted rather than kept: a cache that does not pay for itself is worse than none, because it
hides its own cost behind a number nobody checks. Anyone who wants to try again should start from the table
above, change `mode=min` to `mode=max` first, and require a warm build under ten seconds before believing
it.

## Handover - Docker layer cache

Implemented in this worktree; no workflow was run and hosted cache-hit behavior is not verified.
The workflow invokes `./installer/e2e/docker-test.sh`, so Buildx flags belong in that script rather
than changing the workflow's build invocation. Both E2E jobs now set up Buildx first.

In Actions, each image uses `docker buildx build --cache-from type=gha,scope=e2e-${name}` and
`--cache-to type=gha,mode=min,scope=e2e-${name}`, followed by `--load`; local runs retain their
original `docker build` path. The per-image scope is a namespace, not a content key. BuildKit derives
layer matches from the base-image digest, Dockerfile instructions, and inputs. Reuse is likely across
PR commits because expensive package-install `RUN` layers precede changing binary/script `COPY`
layers; changing an earlier input correctly invalidates downstream layers. PRs can read the default
branch cache. `mode=min` is sufficient because the expensive layers are part of the final image and
avoids exporting intermediate-only layers.

Before: the workflow ran `./installer/e2e/docker-test.sh e2e ${{ matrix.image }}` and the script ran
`docker build $platform_flag -f "$dockerfile" -t "dotfiles-test-${name}${tag_suffix}" .`. After:
the workflow invocation is unchanged, while Actions uses `docker buildx build $platform_flag
--cache-from type=gha,scope=e2e-${name} --cache-to type=gha,mode=min,scope=e2e-${name} --load
-f "$dockerfile" -t "dotfiles-test-${name}${tag_suffix}" .`.

Measured image builds total **118 s on a PR** (ubuntu 44 + fedora 47 + arch 22 + alpine 5; debian
is excluded), and **133 s on main** (+ debian 15). Ubuntu's 44 s is on the critical path. The first
cache-writing run still pays 118/133 s of cold build time, plus unmeasured cache export overhead. A
warm run can avoid up to 118/133 s of image build time, including up to 44 s on the PR critical path;
net savings equal that avoided build time minus cache import/export and Buildx setup time. Those
transfer costs and cache sizes are unknown until CI runs, so these are projections, not measured
savings. The scopes isolate each BuildKit image cache, but they do not guarantee storage isolation from the
shared Actions cache quota or the existing Go cache entries. The first run on this PR writes its
cache; a second run on the same PR is the proof of a hit. A worktree cannot verify a hosted cache hit,
and the orchestrator will run both and report.

## Evidence log

| Task | Run / job | Observed |
| --- | --- | --- |
| baseline | 36625550831 | 14 jobs, per-job durations reproduced; run wall clock 481 s |
| gate cost | 36625550831 | `Go tests` 20:19:41-20:20:28; E2E matrix starts 20:20:31 |
| ubuntu split | job 109601822608 | `docker build` 44 s (20:20:47.4-20:21:31.5); `docker run` 361 s (20:21:31.5-20:27:33.0) |
| Homebrew cause | job 109601822608 | 1 `Installing Homebrew package manager`, 251 `==>` operations, and fish pulling gcc, glibc, binutils, isl, mpfr and libmpc as bottles |
| no Homebrew elsewhere | jobs 109601822659, 109601822693 | fedora/arch logs contain 0 `==>` operations |
| alpine/debian suite | jobs 109601822584, 109601822662 | 13 assertions, 7 tests, no installation |
| phantom check | 36559187850 | `failure Branding Audit` + `skipped Build (${{ matrix.os }})` |
| main run cancelled | 36586936998 | `cancelled Linux E2E (ubuntu)` at 15:09:32; next main push created 36588029818 at 15:09:27 |
| Go cache | job 109601822608 | `Cache restored from key: setup-go-Linux-x64-ubuntu24-go-1.25.1-...`, `Cache Size: ~27 MB` |
| apt waste | jobs 109601476939, 109601477212 | `shellcheck is already the newest version (0.9.0-1)`; 12.5 MB fetched in 6.4 s |
| upstream pipefail family | `installer/e2e/docker-test.sh:271` | the `cmd \| tee` defect is already fixed here, with the reason in a comment; no remaining pipeline in the E2E scripts decides a pass or a fail |
| upstream `rm -rf` family | `installer/e2e/*.sh` | every `rm -rf` is anchored to `$HOME`, `$PREFIX` or a `mktemp -d`; none is a bare relative path |

## Phase 2, second pass: trainer waits converted; remaining waits measured

The workflow half is done (PR #116: the shell check that could not fail now fails, and there are no fixed waits
left in the workflows or the e2e scripts). This section records the measured baseline, the earlier guard
rendering pass, and the trainer-wait follow-up. The trainer follow-up restored the existing `teatest.WaitFor`
conditions, converted the 36 sleeps in `trainer_e2e_test.go`, and left the separate 47 waits in
`teatest_test.go` as the last remaining unit of this work.

MEASURED BASELINE, this host, `go test ./internal/tui/... -count=1`:

- **54.3 s wall clock**, 3,651 tests in 2 packages.
- The waits that bet on time: **47** in `teatest_test.go`, **36** in `trainer_e2e_test.go`, and **1** in
  `companion_test.go` that is the subject under test rather than a wait (the animation's elapsed time).
- The slowest tests: `TestCompanionBlockDependsOnlyOnTheTerminal` **14.5 s**, `TestStepCloneRepository`
  **8.7 s**, `TestStepInstallShellCreatesTheAliasOnlyForANewFnm` **1.7 s**.

THE FINDING WORTH KEEPING: the slowest test in the package is not a `teatest` golden at all, it is the
constant-space guard this repository added last night, which renders roughly eight hundred screens (four
terminal sizes x two sprite modes x forty-seven framed states x two content states, plus the trainer's six x
two). That is real work rather than a wait, so it is not a sleep to convert but a candidate to **parallelise**
or to sample - and any change to it must not shrink what it covers, because its coverage is the reason the
creature's size can no longer drift with a screen's content.

The trainer conversion preserved every test and golden. All 36 sleeps were synchronization waits, not behavior
under test: startup sleeps became first-frame waits; sleeps following key input became waits for rendered
output. **Kept sleeps: 0**, because none measured elapsed behavior, animation, countdown, or timestamp
resolution. The existing 14 `teatest.WaitFor` call sites keep their original predicates and 50 ms / 2 s
interval and timeout. The shared trainer helper now delegates stream reading to `teatest.WaitFor` and captures
the matched bytes only for golden output; the previous custom polling reader was removed.

Before/after measurements from this host, using wall-clock timing (`TIMEFORMAT='WALL_SECONDS=%3R'; time ...`):

| Measurement | Before this pass (working-in-progress state) | After this pass |
| --- | ---: | ---: |
| Trainer command `cd installer && go test ./internal/tui -run '^TestTrainer' -count=1` | 4.120 s; 393 passed, 1 failed (the replaced waiter timed out) | 4.923 s, 3.039 s, 3.132 s; 394 passed each run (mean 3.698 s) |
| Package command `cd installer && go test ./... -count=1` | 32.146 s; 3,898 passed, 1 failed (same timeout) | 38.003 s; 3,899 passed in 4 packages |

The before figures are the real failed WIP runs, not a clean pre-conversion baseline; the suite wall clocks are
therefore evidence of this host's runs, not a controlled claim that the conversion alone saved time. The three
trainer runs all passed, and the full package suite passed without changing goldens.

The failure proof was produced by temporarily changing the `TestTrainerMenuGolden` predicate to
`FACT_THAT_NEVER_ARRIVES`, then restoring it. The observed failure was:

```text
trainer_e2e_test.go:41: WaitFor: condition not met after 2s. Last output:
```

The last output was the terminal initialization bytes only. No production file changed and there is no changelog
entry: user-visible behavior did not change. **Remaining work: convert the 47 waits in `teatest_test.go`; that
file is the last remaining unit of this work.**

### Guard rendering pass: parallel, coverage held

The guard was parallelised without making its fixture helpers concurrent. It builds all 848 models
sequentially (the fixture helpers call `t.Setenv`), then uses a bounded `runtime.NumCPU()` worker
pool over indexed cases to run only `Model.View()` concurrently. The indexed results are asserted
sequentially, preserving each screen/state identity and the original row, size, mode, message and
position checks. There is no `t.Parallel()`.

Coverage is unchanged: 47 framed screens + 6 trainer screens, each at 4 sizes × 2 sprite modes × 2
content states = **848 rendered screens and 848 comparisons**, before and after. The guard now counts
completed renders atomically, counts sequential comparisons, asserts both are 848, and logs the
counts.

Measured on this host (wall-clock, `TIMEFORMAT='WALL_SECONDS=%3R'; time ...`):

| Measurement | Before | After |
| --- | ---: | ---: |
| Guard command `cd installer && go test ./internal/tui -run '^TestCompanionBlockDependsOnlyOnTheTerminal$' -count=1 -v` | 7.911 s | 7.261 s, 5.825 s, 5.755 s (three runs; mean 6.280 s) |
| Package command `cd installer && go test ./internal/tui/... -count=1` | 33.779 s | 32.715 s |
| Guard test execution (Go test JSON `Elapsed`) | prior recorded measurement: 14.5 s | 5.99 s (`go test -json ./internal/tui -run '^TestCompanionBlockDependsOnlyOnTheTerminal$' -count=1`) |

The direct pre-change wall-clock run in this session was faster than the historical 14.5 s reported
above, so the test-execution figures are not a controlled apples-to-apples comparison. The same
shell wall-clock command measured the guard at 7.911 s before and a 6.280 s mean after; the package
also measured 1.064 s faster. These are local measurements, not a claim about other hosts.
The required three focused runs all passed. Full suite: `cd installer && go test ./... -count=1`
passed 3,899 tests in 4 packages (30.960 s wall-clock). `go vet ./...` passed, and `gofmt -l installer/internal/tui/companion_test.go` returned no paths.

### Phase 2, third pass: teatest waits classified by shape

This closes phase 2's test half. The 47 fixed sleeps in `installer/internal/tui/teatest_test.go`
were classified individually instead of replacing them with one wait rule:

| Shape | Sleep sites | Replacement | Why |
| --- | ---: | --- | --- |
| Golden / test quits immediately | **13** | Wait for two successive output chunks using only `len(output) > 0`; send keys, `WaitFinished`, then compare the captured stream plus the remaining output | The first output may be terminal initialization bytes, not a drawn frame. Waiting for the next output event lets the frame arrive without depending on screen text. The waiter consumes its stream, so golden output is retained explicitly rather than assumed to remain in `FinalOutput`. |
| Screen opens | **22** | Wait for that screen's own distinguishing text; existing screen waits now follow the key directly where they already proved the transition | The stream is consumed in sequence. Each next predicate is evaluated against the output after the preceding wait, and no wait uses text from a previous screen as its condition. |
| Pure arrow/j/k navigation | **12** | **No wait** | Cursor movement has no reliable new text: the marker moves and unchanged labels are not a new observable. The next screen-opening wait checks the consequential navigation. |
| **Total** | **47** | **47 removed** | No fixed sleep remains in `teatest_test.go`. |

The necessary second length-only observation is evidence-driven. A first converted run that waited for
any output once failed both golden output comparisons and
`TestMainMenuWithRestoreOption/main_menu_renders_without_restore_when_no_backups`: the bytes observed
were only terminal initialization, and the program could be stopped before drawing its first frame.
That attempt exposed the old assumption that 100 ms always covered startup; it was not papered over
with a longer timeout. The final helper waits for the next length-positive output event and captures
both consumed chunks. All six teatest goldens still compare against their unchanged snapshots.

The required deliberately broken-wait proof changed `TestMainMenuGolden` to a predicate that always
returns false. Verbatim observed failure:

```text
teatest_test.go:166: WaitFor: condition not met after 2s. Last output:
\x1b[?25l\x1b[?2004h\x1b]2;dotfiles Installer\x07
```

The predicate was restored immediately. No tests were deleted, skipped, shortened, or given weaker
assertions. No production file changed, and no changelog entry was added because user-visible behavior
did not change.

Measured on this host, after conversion:

| Command | Wall clock | Result |
| --- | ---: | --- |
| `cd installer && go test ./internal/tui/ -count=1` (run 1) | 31.231 s | 2,019 passed |
| same (run 2) | 29.508 s | 2,019 passed |
| same (run 3) | 29.479 s | 2,019 passed |
| `cd installer && go test ./... -count=1` | 27.964 s | 3,899 passed in 4 packages |

For before/after context, the prior phase's real full-suite run on this host was 30.960 s for
`cd installer && go test ./... -count=1` (3,899 passed). The prior package measurement in the log was
33.779 s for `cd installer && go test ./internal/tui/... -count=1`; it includes the package subtree,
so it is not an exact pre-change measurement of the requested `./internal/tui/` command. No exact
pre-change wall-clock run for that one-package command was recorded, so the package before/after
comparison is unavailable rather than inferred. These are host measurements, not a controlled
performance claim.

`cd installer && go vet ./...` passed. `gofmt -l installer/internal/tui/teatest_test.go installer/internal/tui/wait_test.go`
returned no paths. Phase 2's test half is **complete**: trainer and teatest sleeps have been removed
by their appropriate shapes, the three required package reruns and full suite pass, and the goldens
remain unchanged.

### Restore-screen CI follow-up

CI exposed that `tm.Output()` is a cell-diff stream, not a complete frame: styled text may be split
across escape sequences, and earlier waits consume bytes. The restore navigation and confirmation
option tests now finish the program and assert on the captured output plus `FinalOutput`, rather than
searching for screen text in the diff stream. Local runs validate the new assertions, but only CI can
establish whether the former diff-stream failure reproduces there.

The first length-positive output event is not necessarily the first rendered frame: it can contain only
terminal-initialization bytes, so restore tests must wait for the next positive event before sending any
key (including Ctrl-C). The assertions over the finished output are still right; stopping on a bare quit
can beat the first render on a slow runner and leave that output empty. Wait for the frame, synchronize
on the transition repaint as needed, then finish and assert on the accumulated finished transcript.

### The restore failures were the test's HOME, not a race

The three failures are now reproduced locally and deterministically, and they were never a timing
flake. The prescribed commands (`go test ./internal/tui -run TestRestore -count=50`, then the same
under `GOMAXPROCS=1`, `taskset -c 0` and eight spinning `yes` processes) passed 50/50 on this host,
because this host's `$HOME` holds three real `.dotfiles-backup-*` directories. Pointing `$HOME` at an
empty directory - the CI condition - turned all three red in every one of five iterations:
`can_select_backup_and_go_to_confirm` stopped on `WaitFor: condition not met after 2s`, while
`shows_restore,_delete,_cancel_options` and `escape_returns_to_backup_list` failed their finished-output
assertions. The mechanism is `Init`'s asynchronous `loadBackupsMsg`, which runs
`m.AvailableBackups = msg.backups` unconditionally and so replaces the backups a test seeded into its
model with `system.ListBackups()`'s scan of the real `$HOME`. On a developer machine that scan finds
real backups, the seeded fixture was silently replaced by a non-empty list, and the tests passed for
the wrong reason; on a CI runner the scan finds none, the list goes empty, `Enter` no longer leaves
`ScreenRestoreBackup`, and the confirm screen is never reached. The fix keeps the finished-transcript
assertion (option a) and adds the missing fixture in the test file: `seedBackupHome` gives each restore
test a `t.TempDir()` `HOME` holding exactly the backups it declares, so the scan and the seed agree on
every machine and the asynchronous replacement is harmless. With an empty outer `HOME` the three tests
now pass 50/50, the same 50/50 under `GOMAXPROCS=1`/`taskset -c 0` with eight `yes` processes, and the
exact CI command (`go test ./... -skip Golden -count=1`) passes in all four packages; `gofmt -l`
returns no paths and `go vet ./...` is clean. No golden moved, no production file changed, and the other
restore tests are untouched. What remains CI's to decide is only whether this was the whole difference
between the runner and this host: if the branch still fails, inspect the runner's `$HOME` - what the
startup scan finds there is what decides these tests - and whether the outer environment differs in
some other way the restore tests still inherit.

## Phase 2, fourth pass: the test-loop render matrix, measured, and a two-speed check

`internal/tui` had gone from roughly thirty seconds to five minutes. The cause was not a wait and not
the product code: three matrix guards each built the same 55 screen models at the same 12 terminals
and rendered them, and two of those guards rendered the same frames twice. The build is the expensive
half - every fixture points `HOME` and `XDG_STATE_HOME` at fresh temporary directories - and
`-count=1`, which the briefs asked for on every run, disables the Go test cache that would otherwise
skip the packages nothing touched.

### Before and after, measured on this host

`go test ./internal/tui/ -count=1`. The host is shared with other worktrees, so wall clock is noisy;
CPU time is the figure this change is responsible for.

| Measurement | Before | After |
| --- | ---: | ---: |
| Package wall clock | 295.4 s | 216.8 s |
| Package CPU (user+sys) | 100.5 s | 62.1 s |
| `TestCompanionBlockDependsOnlyOnTheTerminal` | 54.4 s | 54.4 s (untouched; see below) |
| `TestCompanionCoverageAcrossTerminalSizes` | 44.3 s | reads the shared pass |
| `TestEveryScreenFitsEveryTerminalSize` | 42.4 s | reads the shared pass |
| `TestNoRenderedLineLeavesAColourActive` | 18.0 s | ~4 s (each case built once) |

Renders and builds, counted from the guards themselves:

| Quantity | Before | After |
| --- | ---: | ---: |
| Matrix renders (fit + companion coverage) | 660 + 660 = 1320 | 660 (one pass, read by both) |
| Colour-leak renders | 265 | 265 |
| **Total matrix renders** | **1585** | **925** |
| **Matrix model builds** | **1585** | **108** |

### What was fused

`terminalFrames` (`screen_coverage_test.go`) builds each of the 55 cases once and renders it once per
measured terminal with the companion animating; the fit guard and the companion-coverage guard both
read those frames. `View()` has a value receiver, so resizing one built model per terminal cannot
leave state behind, and the pass is cached for the test binary, so the second guard reuses the first
one's bytes. The pass is the strictest frame (companion on), so the fit guard now fails on a screen
that overflows only with the creature - it cannot miss one the old static render would have passed.
`TestNoRenderedLineLeavesAColourActive` builds each of its 53 cases once and resizes it per terminal
instead of rebuilding per size.

No assertion was removed or weakened. The two coverage floors (`mainCompanionCoverage80x24`,
`mainCompanionCoverageTotal`) still hold (the pass measured 36/55 at 80x24 and 480/660 overall), the
fit guard still pins 55 screens, and the colour-leak guard's `checked == 0` check was replaced by a
hard pin that fails if any of the 53 screens drops out of its 5-size sweep.

### What was deliberately not optimised, and why

- **`TestCompanionBlockDependsOnlyOnTheTerminal` (54 s).** It lives in `companion_test.go`, outside
  this front's edit surface. A previous pass parallelised only its `View()` calls; its 848 model
  builds are still sequential, and each one pays the fixture helper's two `t.TempDir()` calls.
  Removing that means caching or cloning models across calls, which risks the isolation every other
  caller of `installerFrameCase` relies on. Left for the owner of that file, with the measurement
  above.
- **Sampling the matrices.** A fixed sample would trade counted coverage for time. The guards exist
  to cover every screen at every terminal, and the fusion already removed the repetition, so no
  sample was taken. The existing asserted floors remain the only sampled claims.
- **`TestAllUserSelectionPaths` (22 s).** Not a matrix render, and not this front's.

### A two-speed local loop

`make preflight` was the only local command and it ran the whole suite uncached. The inner loop now
has its own command:

| Command | Runs | When |
| --- | --- | --- |
| `make check` | `gofmt`, `go vet`, and the tests for the packages this branch changes, with Go's test cache on | after every edit |
| `make preflight` | the full gate, unchanged: 7 steps, whole suite with `-count=1` | once, before pushing |

`scripts/preflight.sh --check` derives the changed packages from `git diff` against the merge-base
with `main` (committed, staged, unstaged and untracked `.go` files under `installer/`). The full path
is untouched and keeps `-count=1`: that is the reproduce-the-whole-suite-once command. `-count=1`
was removed from the inner loop, where disabling the cache was the work the loop exists to avoid.
README's "Before you push" and `docs/tui-installer.md`'s "Running Tests" state the two commands and
when each runs.

### Teeth, pasted from real runs

`make check` fails on a dirty `gofmt`:

```text
Files not gofmt'd:
internal/tui/screen_coverage_test.go
CHECK FAILED: gofmt check: run 'gofmt -w .' inside installer/
```

on a broken `go vet`:

```text
internal/tui/screen_coverage_test.go:192:25: fmt.Printf format %d has arg "not a number" of wrong type string
CHECK FAILED: go vet ./...
```

and on a broken test:

```text
--- FAIL: TestEveryScreenFitsEveryTerminalSize (5.48s)
    screen_coverage_test.go:208: the guard rendered 660 screen x size cases, want 672
FAIL
FAIL	github.com/albersg/dotfiles/installer/internal/tui	268.437s
CHECK FAILED: go test ./internal/tui
make: *** [Makefile:10: check] Error 1
```

Measured with the teeth restored: `make check` on a change to `internal/tui` ends
`ok ... 235.967s`; `make preflight` ends `PREFLIGHT PASSED - 7/7 steps`, with the suite step reporting
`ok .../internal/tui 243.576s`. Both are dominated by the 54 s guard this front cannot touch, and both
are wall-clock figures from a host shared with other worktrees, so they are evidence of this host's
runs rather than a controlled claim about an idle one.

## Phase 2, fifth pass: the isolation never invalidated the cache, measured; the loop stays as it is

The fourth pass removed `-count=1` from the inner loop. A follow-up reading went further: that the
fixtures' fresh `t.TempDir()` `HOME`/`XDG_STATE_HOME` directories made the suite uncacheable, so the
cache never hit and the inner loop could not be fast. The tools say otherwise, and the measurement
agrees. It is written down here so the same change is not proposed again.

### The mechanism: a `t.TempDir()` is invisible to the cache key

Go's test cache records the environment variables and files a test consults and re-hashes their
values on the next run (`computeTestInputsID`). Three facts in Go 1.27.1 decide this:

- `src/os/env.go:102` - `os.Getenv` logs only the *name*: `testlog.Getenv(key)` is called and the
  value is returned from `syscall.Getenv` without entering the log.
- `src/cmd/go/internal/test/test.go:2018` - `computeTestInputsID` re-hashes each logged name with
  `hashGetenv(name)`, which is an `os.Getenv` in the **`go` command's** process. `t.Setenv` changes
  only the test binary's environment, so the temporary path never reaches the parent `go`, and the
  value hashed is the same from run to run.
- `src/cmd/go/internal/test/test.go:2048-2066` - `stat` and `open` entries are dropped unless the
  path is inside the package's module root (`search.InDir(name, a.Package.Root) == ""`). Every
  `t.TempDir()` lives under `/tmp`, outside the module, so nothing it contains is rechecked.

So the suspected invalidation does not happen. The lines that were blamed, with their
`origin/main` positions:

- `installer/internal/tui/install_paths_test.go:765,799,829,852,872,894,2330,2421,2460,2537,2582,2626,2664,2724,2785,2819,2864,3138,3201,3256,3296,3336`
  - `t.Setenv("XDG_STATE_HOME", t.TempDir())`
- `installer/internal/tui/update_test.go:1063,1294,1946,1996` - `t.Setenv("XDG_STATE_HOME"/"HOME", t.TempDir())`
- `installer/internal/tui/util_screen_test.go:867,925,969,1044` - `t.Setenv("XDG_STATE_HOME", t.TempDir())`
- `installer/internal/tui/teatest_test.go:108,109` - `t.Setenv("HOME"/"XDG_STATE_HOME", t.TempDir())`
- `installer/internal/tui/companion_test.go:84`, `homebrew_step_test.go:58`, `fnm_alias_test.go:84,98`
  - `t.Setenv("HOME", t.TempDir())`
- `installer/internal/tui/arch_packages_test.go:162-170`, `debian_packages_test.go:128`,
  `fedora_packages_test.go:172-179` - `home := t.TempDir(); t.Setenv("HOME", home)`

### Measured on this host, cache left on

| Command | First run | Run again |
| --- | ---: | ---: |
| `go test ./internal/tui` | `ok ... 227.128s` | `ok ... (cached)` |
| `go test ./internal/system` | `ok ... 7.461s` | `(cached)` 0.78 s, then 0.83 s |
| `go test ./internal/tui -run '^TestMainMenuEasterEggsFireOnlyAfterTheirLastCharacter$'` | `ok ... 0.208s` | `(cached)` 1.13 s |
| `go test -c -o /dev/null ./internal/tui` (build only) | - | 1.08 s (binary already built) |
| `go vet ./...` | - | 0.25 s (cached) |
| `go vet ./internal/tui` | - | 0.19 s (cached) |

No test changed, and the `TestMain`/shared-stable-temp-directory idea retired with this: it would
have shared state between tests and bought no cache hit.

### The real cost, and why the loop was not shortened

- **The tests are the cost, not vet.** `scripts/preflight.sh:136` runs `go test` for each changed
  package, and an edit anywhere under `internal/tui` makes that the whole package: about 227 s
  uncached on this host. `scripts/preflight.sh:124` runs `go vet ./...` over the whole repository,
  but it is cached and measured at 0.25 s, so scoping it to the changed packages would save nothing
  and would drop vet coverage of dependents. It was left alone.
- **No honest shortening exists at this surface.** The fused render matrix is already the minimum the
  guards assert, and the 54 s `TestCompanionBlockDependsOnlyOnTheTerminal` lives in
  `companion_test.go`, outside this surface. Cutting either trades coverage for time.
- **The variable that actually moved the wall clock was the host.** On the same host a cache *hit*
  was measured at 10 m 20 s wall with about 72 s of CPU (`user 15.6 s + sys 56.6 s`): the process was
  being starved, not failing to find the cache. Named at the time: a `git-remote-https` at ~200 %
  CPU for hours and a Gradle daemon at ~280 %. `make check` with the cache on is therefore honest
  only as "cache on, changed packages": on a real `internal/tui` edit it still runs the whole
  package, and the wall clock depends on what else the machine is doing.

### What changed

Nothing that affects test behavior: no fixture lost its isolation, no golden moved, `make check`
and `make preflight` are unchanged. The refutation is recorded so the shared-temp-directory idea is
not re-proposed.
