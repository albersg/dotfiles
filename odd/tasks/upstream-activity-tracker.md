# Upstream Activity Tracker

## Objective
Notify this repository about every newly opened issue and pull request, and every new commit on upstream `main`, by appending one entry per event to a single persistent GitHub issue.

## Problem and rationale
The existing `upstream-watch` workflow detects new upstream tags and opens integration PRs, but it does not report ordinary upstream activity. Keep tag-based integration unchanged and add a separate activity feed so commits/issues/PRs are not treated as release syncs.

## Scope and constraints
- Upstream: the configured public source repository, branch `main`.
- Destination: this repository, one persistent activity-tracker issue; each unseen issue, PR, and commit is recorded as an issue comment.
- Preserve current tag/release detection, integration branch, conflict issue, and integration PR behavior.
- Begin tracking from activation: establish a baseline without backfilling historical issues, PRs, or commits.
- Prevent overlapping scheduled/manual runs from duplicating notifications; checkpoint progress and make partial retries idempotent.
- Use existing GitHub Actions permissions only; never expose credentials or mutate the upstream repository.

## Accepted design rationale
- A single tracking issue is the selected destination, rather than one issue per event or a periodic PR.
- Activity notices are separate from release/tag-based code synchronization.
- Initial execution establishes a current baseline rather than flooding the tracker with past activity.
- Commit detection uses a stored head SHA comparison, not a date filter, so backdated commits newly reachable on `main` are included.

## Tasks

### UAT-1 — Add idempotent upstream activity feed
- [x] Add scheduled/manual tracking to the existing upstream-watch workflow without changing release-sync gates.
- [x] Find or create one destination tracker issue, initialize its baseline on first activation, then append notices for new upstream-main commits, issues, and PRs.
- [x] Persist checkpoint state, deduplicate retries, serialize runs, and use a commit-SHA comparison cursor that captures newly reachable backdated commits.
- [x] Validate API pagination, event formatting, failure behavior, and permissions; fail closed without advancing past unrecorded events.

### UAT-2 — Add focused regression checks and documentation
- [x] Add deterministic focused checks for watcher contract, including initial baseline, new events, backdated commits, retries, pagination, divergence, and release-gate preservation.
- [x] Document the activity issue, activation baseline/no-backfill behavior, polling cadence, and troubleshooting in upstream synchronization docs.

## Acceptance criteria
- Every newly reachable commit on upstream `main` (including commits with older author/committer dates), newly created issue, and newly created PR is visible in the same issue in this repository.
- Each event is recorded at most once, including after a retry; no historical backfill occurs on activation.
- Concurrent runs cannot create duplicate tracker issues or post duplicate events.
- Existing release/tag-driven integration behavior remains unchanged.
- Documentation and focused checks cover the user-visible behavior.

## Verification plan
- RED: run the focused deterministic workflow/helper checks before implementation and observe failures for the intended new behavior.
- GREEN: rerun after implementation and exercise baseline, events, retry deduplication, and unchanged release-sync behavior.
- Run applicable workflow syntax/readback checks through the verifier; report any unavailable external GitHub execution as unverified (do not claim live delivery was tested).

## Progress and evidence
- Authorized by user: implement the selected single persistent tracking issue approach.
- Route: delegated direct implementation; multi-file writer and independent verifier used.
- Actual authored change from delegated implementation: approximately 1,278 lines across workflow, helper, deterministic tests, and documentation. This exceeds the default ~400-line delivery threshold. User selected `feature-branch-chain`; no push, PR, or merge authorized.
- Current branch: `feat/upstream-activity-tracker` (branched from clean detached `11ce61e`).
- Design: one serialized workflow job creates/finds a tracker issue, appends marker-deduplicated comments, stores issue/PR timestamp and commit-head SHA cursors in the issue body, and establishes no-backfill baselines. Release-sync job graph/gate remains unchanged.
- Commit cursor correction: initial implementation used commit dates and could miss a newly reachable backdated commit. It now compares saved and current branch-head SHAs and fails closed on divergent/behind histories. Issue/PR discovery remains time-based.
- TDD: old suite baseline 19/19; backdated-commit test RED was observed (19 pass, 1 fail); after SHA comparison, 27/27 tests pass. Independent verifier reran all 27 tests, `node --check` for helper/tests, and tracked/untracked whitespace checks; no blocking findings.
- Not verified: live GitHub API or Actions runtime, concurrency behavior on GitHub, actionlint, and a YAML parser. Focused tests are documented but not wired into CI.
- Audit adjustment: this task file avoids the exact upstream brand token because the existing branding audit scans `odd/tasks/*.md`.
- No native review, commit, push, PR, or merge yet. RDD is on; earlier ASSESS failed closed as unassessable because candidate files were untracked. Run inspect with exact intended-untracked selection before committing/reviewing the work unit.

## Next step
Complete native preflight for the exact intended-untracked set, close the verified work unit on the selected feature branch, record its commit evidence, and stop before push/PR/merge unless separately authorized.
