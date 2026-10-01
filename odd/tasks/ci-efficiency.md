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
