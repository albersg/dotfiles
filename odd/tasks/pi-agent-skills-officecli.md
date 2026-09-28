# ODD Feature Tasks: Pi Agent Skills and OfficeCLI

## Goal
Integrate the user's corrected WSLg shell configuration into the tracked dotfiles source, and make the dotfiles installer provision the three requested Pi skills plus the OfficeCLI executable using pinned, checksum-verified upstream artifacts.

## Problem and rationale
The installer copies `dotfiles-zsh/.zshrc` over `~/.zshrc`, but the tracked template still contains a relative Wayland socket path that breaks WSLg clients. The three requested skills currently exist only in the user's untracked `~/.pi/agent/skills/` tree, and the installer has no Pi skill provisioning step. OfficeCLI's upstream installer is unpinned, so the executable must be installed through the repository's pinned per-architecture checksum pattern instead.

## Accepted decisions
- Use pinned upstream archives/commits and SHA-256 verification for Pi skills rather than vendoring the complete packages. This avoids adding Archify's full 8.6 MB / ~153,500-line development tree; setup will require network access.
- Provision skills only for Pi at `~/.pi/agent/skills/`; no Claude/OpenCode targets were requested.
- Install OfficeCLI from a versioned GitHub release with hard-coded verified SHA-256 values; do not execute `d.officecli.ai/install.sh`.
- Preserve the user's current skill versions where possible: Cloudflare security-audit-skill commit `c1c8a8c1471069fb0e188eeaff69b8e8db6564a8`, tt-a1i/archify commit `9e35d2b0b39b155553ba9fcfe0b4f2a5198dd993`, and iOfficeAI/OfficeCLI skill commit `ffa8a0afbe2e9686abd636368e3da38c50f22131`.
- Verified source pins from GitHub: the Cloudflare codeload ZIP SHA-256 is `18b53d57762ce312c8438e4252fdd72e33533fc50c7287d4d9eda2588f9c729d`; the Archify codeload ZIP SHA-256 is `2bb330db382f281247ad490c0a0291913b9f9c10cd357426ee4338e84334c3f5`; OfficeCLI's pinned `LICENSE` and `skills/officecli/SKILL.md` content hashes are `7e282402a5a6db33995fe638bb3fe79013f9884d8f7d15a42e481c1e86aadda1` and `c950d285ce60021712b4753fb2d9f592308d5622bab776229061dfecb1ce55d4`, respectively. The installed Archify version is a development commit newer than the latest published release, so pin the exact commit archive rather than downgrading to the v2.16.0 release.
- Current user platform is Linux x86_64. GitHub release v1.0.152 exposes these official asset SHA-256 digests:
  - `officecli-linux-x64`: `e54d3c1d248372365f0634aac56d6f1918bd04d6e71afc792ad50e075f56cfe9`
  - `officecli-linux-arm64`: `bc06deaa0ad931f5208717a40b94018dc44cdff0d8eefa842c4f4daf89fb35a8`
  - `officecli-linux-alpine-x64`: `390e246303bf43b4739e3195e9b171a660223b4c11218b65894d5fadf9a52755`
  - `officecli-linux-alpine-arm64`: `65c65e05100bac1376e23f6ca97086a046afdfcaf50f2bc215a8c139b3bc22ba`
  - `officecli-mac-x64`: `5071abef56c1d4a4d60e28ed12bc66183d8dc6a9783529c3f1a9cf6bdfe6c2dd`
  - `officecli-mac-arm64`: `e2ed6eba5cd46d6800139f2835097828b8ccd7c8c9b679463b50e45ba2f1dbf5`.
- Work on branch `feat/pi-agent-skills-officecli`. Do not commit, push, or create a PR unless the user explicitly asks.

## Workflow configuration
- Route: ODD; use one bounded implementation worker for each multi-file write unit.
- Strict TDD: off; no explicit project/session opt-in was found. Tests are still required. Exact focused Go runner: `cd installer && go test ./internal/...`.
- Delivery strategy: `ask-on-risk`. Estimate: approximately 450–600 authored diff lines across code, tests, and docs (excluding downloaded upstream archives). Chain strategy is deferred unless the user later authorizes commits/PR work; current repository policy forbids unrequested commits.

## Tasks

### T1 — Sync corrected WSLg settings into the tracked shell template
- [x] Replace stale/duplicate relative Wayland handling in `dotfiles-zsh/.zshrc` with the proven absolute WSLg socket probe from `~/.zshrc`.
- [x] Add a regression assertion that the absolute socket is used and relative `wayland-0` assignments are absent.
- Check: focused regression test passed; `zsh -n`, `go vet`, and `gofmt` passed.
- Route: delegated direct; multi-file write trigger.
- Evidence: independent `gentle-ai-verify` confirmed the only failure in `go test ./internal/tui` is the pre-existing host-state `TestMainMenuGolden` mismatch (three existing `~/.dotfiles-backup-*` directories make the menu differ from the empty-home golden). No T1 source file affects backup detection or menu rendering; full package green remains unavailable on this host.

### T2 — Add pinned Pi skill provisioning to the installer
- [x] Add pinned-source metadata for `security-audit`, `archify`, and `officecli` skill packages; verify upstream refs, archive layout, and SHA-256 before locking constants.
- Verified pins: Cloudflare codeload ZIP `https://codeload.github.com/Cloudflare/security-audit-skill/zip/c1c8a8c1471069fb0e188eeaff69b8e8db6564a8` → `18b53d57762ce312c8438e4252fdd72e33533fc50c7287d4d9eda2588f9c729d`; Archify codeload ZIP `https://codeload.github.com/tt-a1i/archify/zip/9e35d2b0b39b155553ba9fcfe0b4f2a5198dd993` → `2bb330db382f281247ad490c0a0291913b9f9c10cd357426ee4338e84334c3f5`; OfficeCLI's GitHub API raw `LICENSE` and `skills/officecli/SKILL.md` hashes are listed above. Parent checked the pinned GitHub tree paths and directly downloaded/computed the codeload hashes; no upstream code was run.
- [x] Add an idempotent installer step that stages, verifies, and installs the skills under `~/.pi/agent/skills/` without silently discarding existing user data.
- [x] Wire the step into interactive and non-interactive schedules and the executor dispatch map; update all three exact-list tests under the user's explicit option-A scope extension.
- [x] Document the Pi-only skill installation and network/pinning behavior.
- Checks: 10 skill/archive-focused tests pass; both schedule/executor invariants pass; broader Go runs have the known `TestMainMenuGolden` host-state mismatch.
- Route: delegated direct; multi-file write trigger.
- Resolved by explicit user choice A: updated `installer/internal/tui/model_test.go`, `comprehensive_test.go`, and `integration_test.go` to include the new step ID and preserve interactive/non-interactive parity.
- Independent verifier: PASS; pins matched the task record, extraction/checksum/error/skip-existing paths passed, and schedule parity was confirmed. Live source archive download was performed by the parent before implementation; live installer invocation remains untested.

### T3 — Add and perform pinned local OfficeCLI binary installation
- [x] Add a pinned, per-architecture OfficeCLI release downloader that verifies SHA-256 before installing to `~/.local/bin/officecli`.
- [x] Keep unsupported platforms fail-closed and avoid any upstream shell installer.
- [x] Add focused tests for successful installation, checksum mismatch, and unsupported architecture.
- [x] Install the verified Linux x86_64 v1.0.152 executable locally and confirm `officecli --version`.
- Checks: 11 focused OfficeCLI tests pass; broader TUI/full-internal runs have only the known `TestMainMenuGolden` host-state mismatch.
- Route: delegated direct for code/tests; parent performs the explicitly authorized local install after independent verification.

### T4 — Verify integrated behavior and reconcile task evidence
- [x] Run focused Go verification and skill package checks; verify all three skills are discoverable from `~/.pi/agent/skills/` and `officecli` resolves from `~/.local/bin`.
- [x] Inspect final diff and record any skipped/blocked checks without claiming success.
- [x] Update this file and the visible todo projection. Engram mirror remains pending for the reason recorded in Progress.
- Route: delegated verification when commands execute; parent spot-checks reported evidence.
- Unverified by design: a live installer run against a clean `HOME`, runtime Pi skill loading, and non-Linux asset delivery. All three require unapproved network or home-state mutation.

## Acceptance criteria
1. [x] Tracked `.zshrc` preserves the working absolute WSLg Wayland socket configuration and has regression coverage.
2. [x] Installer provisions all three pinned Pi skills to `~/.pi/agent/skills/` using verified sources and does not execute an unpinned upstream installer.
3. [x] Installer supports a pinned OfficeCLI binary release with checksum verification; this Linux x86_64 host has the verified CLI installed locally.
4. [x] Relevant focused tests pass, with every unavailable or skipped check recorded honestly.
5. [x] No commit, push, or PR was made without explicit user authorization.

## Progress
- Exploration complete: working tree was clean on `main`; the user-local WSLg block differs from the tracked template; the current skills are under `~/.pi/agent/skills/`; no Pi skill installer exists.
- User approved pinned upstream downloads instead of full package vendoring.
- Branch created: `feat/pi-agent-skills-officecli`.
- Engram mirror is pending: memory writes are rejected because this Pi session is bound to project `/tmp`; a separate `mem_session_start` for dotfiles did not change the runtime binding. Keep this repository file as the durable record and retry synchronization from a Pi session rooted in this repository.
- T1 is complete and independently verified: the focused WSLg test passed; zsh syntax, `go vet`, and `gofmt` checks passed. The only broader TUI failure is the confirmed host-state `TestMainMenuGolden` mismatch described under T1.
- Native `assess` returned `unassessable` because the authorized `odd/` task file is untracked; its fail-closed plan required an independent verifier, which completed and confirmed T1's focused correctness. Parent spot-check remains required before final delivery.
- User approved pinned downloads; immutable source refs and SHA-256 values are verified and recorded above. The OfficeCLI v1.0.152 Linux x64 binary hash is the GitHub asset digest recorded above.
- T2 user selected A, explicitly authorizing updates to `model_test.go`, `comprehensive_test.go`, and `integration_test.go` to keep the interactive schedule tests in parity.
- T2 implementation and independent verification are complete. The `agentskills` step is wired into both schedules and the executor; focused tests pass; the package/full-internal runs have only the known `TestMainMenuGolden` host-state failure. The verifier found no new defects.
- Parent verified the current local OfficeCLI skill's `LICENSE` and `SKILL.md` hashes match the pinned upstream contents exactly; all three skill `SKILL.md` files are present. The OfficeCLI executable is not installed yet, and `~/.local/bin` exists.
- T3 implementation returned with a separate pinned OfficeCLI step, per-platform asset/hash mapping, atomic install, tests, schedule updates and docs. Eleven focused tests pass; broader Go runs have only the known `TestMainMenuGolden` failure. No real binary download/install was performed by the writer.
- T3 independent verification is complete and passed: release pin matrix, mapping, Termux behavior, checksum-before-install, atomic staging, permissions, preservation, docs, and schedule parity all match; 13 focused tests passed. The broader TUI/full-internal runs retain only the known host-state `TestMainMenuGolden` failure.
- Parent installed the official v1.0.152 Linux x64 release asset after SHA-256 verification. `/home/alberto/.local/bin/officecli` hashes to `e54d3c1d248372365f0634aac56d6f1918bd04d6e71afc792ad50e075f56cfe9`, mode `0755`, and `officecli --version` reports `1.0.152`; `command -v officecli` resolves to that path. No upstream shell installer was executed.
- Engram mirror remains pending because this Pi host session is bound to `/tmp`; local task file is the durable progress record.
- Next step: T4 final integrated verification, local skill discovery/hash check, diff inspection, and reconciliation.

## Verification evidence
### T1
- `cd installer && go test ./internal/tui -run TestShippedZshrcUsesAbsoluteWSLgWaylandSocket -count=1`: PASS.
- `cd installer && go test ./internal/tui`: 1029 passed, 1 failed (`TestMainMenuGolden`); independently traced to the verifier host's three existing backup directories and the golden's empty-home assumption.
- `zsh -n dotfiles-zsh/.zshrc`: PASS.
- `go vet ./internal/tui/`: PASS.
- `gofmt -l internal/tui/install_paths_test.go`: no output (formatted).
- Independent verifier's source review: PASS for T1; it did not run a clean-HEAD baseline because that would require an unauthorized worktree mutation.

### T2
- `cd installer && go test ./internal/tui -run 'AgentSkill|SafeArchivePath' -v`: 10/10 PASS; independent verifier's focused test set including schedule invariants: 12/12 PASS.
- `cd installer && go test ./internal/tui`: 1039 passed, 1 known `TestMainMenuGolden` host-state failure.
- `cd installer && go test ./internal/...`: 1619 passed, 1 same known failure.
- `go vet ./internal/tui/`: PASS; `gofmt -l` over all changed Go files: no output.
- Independent verifier: PASS for pin strings, safe extraction, checksums, cleanup, skip-existing, and schedule parity. Parent verified the upstream tree paths and codeload SHA-256 values before implementation. The installer has not yet been invoked against a clean HOME.

### T3
- `cd installer && go test ./internal/tui -run 'OfficeCLI|Alpine' -count=1`: 11/11 PASS; independent verifier's focused set including schedule invariants: 13/13 PASS.
- `cd installer && go test ./internal/tui`: 1050 passed, 1 known `TestMainMenuGolden` host-state failure.
- `cd installer && go test ./internal/...`: 1630 passed, 1 same known failure.
- `go vet ./internal/tui/`: PASS; `gofmt -l` across changed Go files: no output.
- Independent verifier: PASS for release digest matrix, mapping, Termux skip, checksum ordering, atomic staging, preservation, permissions, docs, and schedule parity.
- Local install completed after verification: the pinned asset was downloaded from the versioned release URL, its SHA-256 matched, and it was published atomically to `~/.local/bin/officecli` with mode 0755. `officecli --version` reports `1.0.152`. No upstream shell installer was executed.

### T4 (complete)
- Parent spot-check: `cd installer && go test ./internal/tui -run 'AgentSkill|SafeArchivePath|OfficeCLI|Alpine|WSLg' -count=1` → 22 passed.
- Independent integrated verification: `go test ./internal/tui -run '<focused set + schedule invariants>'` → 24 passed; `go test ./internal/...` → 1630 passed, 1 known `TestMainMenuGolden` failure; `zsh -n dotfiles-zsh/.zshrc` → exit 0; `go vet ./internal/...` → clean; working tree identical before/after the test runs.
- Local install confirmation: `/home/alberto/.local/bin/officecli` mode 0755, SHA-256 `e54d3c1d248372365f0634aac56d6f1918bd04d6e71afc792ad50e075f56cfe9`, `officecli --version` → `1.0.152`.
- Local skill confirmation: `SKILL.md` present for `security-audit`, `archify`, and `officecli` with matching frontmatter names; OfficeCLI skill `LICENSE`/`SKILL.md` hashes equal the pinned upstream contents.
- Acceptance criteria AC1–AC5 confirmed by the independent verifier; no pin mismatch, missing executor, schedule asymmetry, unexpected failure, or repository mutation found.
- Known documentation gap reported, not fixed: `docs/ai-configuration.md` lists the three pinned skill commits but not their package SHA-256 values, which live in the code, tests, and this task file.
- Unverified by design: live installer run against a clean `HOME`, runtime Pi skill loading, and non-Linux assets.

## Next step
Feature work is complete and verified. Await user direction on whether to (a) add the skill package hashes to `docs/ai-configuration.md`, (b) commit the candidate as reviewable work units, or (c) leave the branch as-is. No commit, push, or PR has been made.
