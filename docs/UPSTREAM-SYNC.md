# Upstream Sync Procedure

## Automated Sync (CI)

The `.github/workflows/upstream-watch.yml` workflow handles automated sync:

1. **Schedule**: Runs every 6 hours + manual dispatch
2. **Detection**: Compares `upstream/main` with `.downstream/version.json`
3. **New tag found**: Creates `integration/upstream-vX.Y.Z` branch
4. **PR creation**: Opens PR with diff report (`--head integration/upstream-vX.Y.Z --base main`)
5. **Conflict handling**: Creates GitHub issue with conflict details (no force push)

Tag detection, the `upstream-main` mirror push, integration branches, conflict issues
and integration PRs are unchanged. The activity tracker below is a separate job and
does not gate or depend on that flow.

## Upstream Activity Tracker (issue feed)

The `track-activity` job in the same workflow keeps one persistent issue in this
repository (`Upstream activity tracker`) and appends one comment per newly observed
upstream event:

| Event | Source | Comment marker |
|-------|--------|----------------|
| New commit reachable on upstream `main` | `GET /repos/{upstream}/branches/main` + `GET /repos/{upstream}/compare/{sha}...main` | `commit:<sha>` |
| Newly created upstream issue | `GET /repos/{upstream}/issues?state=all` (no `pull_request` field) | `issue:<number>` |
| Newly created upstream pull request | same endpoint, entries carrying `pull_request` | `pr:<number>` |

Commits use a persisted **head-SHA comparison** rather than a date filter. GitHub's
`listCommits?since=<time>` endpoint filters by commit date, so a commit that only
becomes reachable now — a cherry-pick, an imported/backdated commit, a merge of an
old branch — would be missed when its author or committer date predates the cursor.
The job stores the last observed head SHA and asks GitHub to compare it with the
current head; every commit reachable from the new head but not the saved one is
reported regardless of its dates. When the upstream head has not moved, the
comparison request is skipped entirely.

Issues and pull requests share one numbering space upstream, so the markers always
carry the event kind; a shared `issues_since` timestamp cursor is enough because a
single endpoint returns both.

### Cadence and activation

- The job runs on the same triggers as release detection: every 6 hours
  (`0 */6 * * *`) and on manual `workflow_dispatch`.
- **Baseline, no backfill**: on the first run the job creates the tracker issue,
  records the current upstream head SHA, and sets the issue/PR timestamp cursor to
  that run's start time. Commits reachable before activation and issues/PRs created
  before activation are never reported.
- The stored head SHA is advanced only after every commit found by a comparison has
  been posted (or confirmed already recorded), so a backdated commit that becomes
  reachable later is reported on the run that observes it.
- To deliberately restart tracking, delete the tracker issue. The next run creates a
  fresh tracker with a new baseline starting at that point.

### Idempotency, ordering and failure behaviour

- The checkpoint lives in a machine-readable HTML-comment block at the bottom of the
  tracker issue body (`<!-- upstream-activity-state ... -->`). It is not coupled to
  `.downstream/version.json`, which stays the release-sync source of truth.
- Every comment ends with a marker such as `<!-- upstream-activity:commit:<sha> -->`.
  Before posting, the job lists the existing comments and skips markers it already
  sees, so repeating a run cannot duplicate an event.
- Runs are serialised by a job-level `concurrency` group
  (`upstream-activity-tracker`, `cancel-in-progress: false`), so a scheduled and a
  manual run cannot race on the tracker issue.
- The issue/PR cursor only moves forward to timestamps of events that were
  successfully recorded (or that were already recorded earlier), and the commit head
  SHA only advances once its whole comparison batch is recorded. A crash between a
  comment and a checkpoint save replays that event on the next run and is
  deduplicated by the marker.
- The checkpoint is saved after every recorded event, so a partially completed run
  resumes instead of restarting.
- Both upstream feeds are paginated explicitly; a page limit being reached, or a
  comparison returning fewer commits than it reports, raises a clear error rather
  than silently truncating results.
- If the saved head is no longer comparable — an upstream force-push/rebase makes
  the comparison `diverged` or `behind` — commit detection fails closed: no commit
  is posted and the commit cursor is not advanced. Issues/PRs are still processed and
  the job exits with a partial-fetch error. To resume commit tracking, delete and
  recreate the tracker issue to establish a new baseline from the rewritten head.
- Fetch and comment failures fail the job loudly. If one event stream fails, events
  from the other stream are still recorded and their cursor advances, but the job
  still exits with a partial-fetch error so the failure is visible. Errors are never
  swallowed.
- An unreadable checkpoint block aborts the run instead of treating the tracker as
  new (which would backfill history).

### Permissions and safety

- The job requests `contents: read` and `issues: write` only, enough to read this
  repository and write the tracker issue. It inherits no `contents: write`.
- All upstream calls are read-only `GET` requests; the tracker never mutates the
  upstream repository and never writes files.
- The upstream repository slug is configured once as an environment variable in
  `.github/workflows/upstream-watch.yml` (kept out of the helper script and this
  document's other paths by the branding audit exclusions).

### Troubleshooting

| Symptom | Cause | Action |
|---------|-------|--------|
| Job fails with `state block could not be parsed` | The checkpoint block in the tracker issue was edited or truncated | Restore valid JSON inside the block, or delete the issue to restart tracking from now |
| Job fails with `Pagination limit reached` | More results than `maxPages * pageSize` in one window | Raise `pageSize`/`maxPages` defaults in `.github/scripts/upstream-activity.js` |
| Job fails with `partial fetches: commits: ...` | Upstream head resolution or comparison failed (rate limit, transient API error) | Rerun the job; nothing was recorded twice and the cursor did not advance past the failed stream |
| Job fails with `upstream history diverged` or `upstream history behind` | Upstream rewrote history (force-push/rebase), so the saved head is no longer comparable | Delete the tracker issue to re-baseline commit tracking from the new head; the issue/PR cursor restarts from that point without backfill |
| Event appears twice in the tracker | A comment was posted without its marker, or a marker was edited | Restore or delete the duplicate comment; markers are what deduplication reads |
| No activity reported after activation | Expected before the first new upstream event arrives | Verify the tracker issue exists and its checkpoint holds the current head SHA and a recent issue cursor; no history is backfilled by design |

Run the tracker's deterministic checks locally (no network, no GitHub token):

```bash
node --test .github/scripts/upstream-activity.test.js
```

The tests cover baseline activation, issue/PR/commit recording, backdated commits,
divergence fail-closed, retry deduplication, pagination and its limits, partial fetch
failure, comment failure with resume, corrupt-checkpoint abort, and the unchanged
release gate.

## Manual Sync

When automated sync fails or you need to sync immediately:

```bash
# 1. Fetch latest upstream
git fetch upstream

# 2. Update mirror
git checkout upstream-main
git merge upstream/main --ff-only  # Must be fast-forward
git push origin upstream-main

# 3. Create integration branch
git checkout main
git checkout -b integration/upstream-v2.13.0

# 4. Merge upstream changes
git merge upstream-main --no-ff -m "merge: sync upstream v2.13.0"

# 5. Resolve conflicts
# - Upstream changes take priority for functional code
# - Re-apply branding overlay on resolved files
# - Verify: cd installer && go build ./cmd/dotfiles && go vet ./...

# 6. Push and open PR
git push -u origin integration/upstream-v2.13.0
gh pr create --head integration/upstream-v2.13.0 --base main \
  --title "merge: sync upstream v2.13.0" \
  --body "Automated sync from upstream tag v2.13.0"
```

## Rebranding Re-Application

After merging upstream changes, re-apply the branding overlay:

```bash
# Re-apply branding to changed Go files
# (This is automated in CI — manual only for conflict resolution)
find installer/ -name '*.go' -newer .downstream/version.json -exec sed -i \
  -e 's/DOTFILES_DRY_RUN/DOTFILES_DRY_RUN/g' \
  -e 's/DOTFILES_VERBOSE/DOTFILES_VERBOSE/g' \
  ... {} +

# Verify
cd installer && go build ./cmd/dotfiles && go vet ./...
rg dotfiles --count installer/  # Should be 0 (or imports only)
```

## Conflict Resolution Guidelines

1. **Functional code changes**: Accept upstream version first, then rebrand
2. **New files**: Apply branding rules to new files before committing
3. **Deleted files**: Remove from downstream, update internal references
4. **Config changes**: Merge carefully — keep downstream personalizations
5. **When stuck**: Create an issue with the conflict output, don't force-resolve

## Verification Checklist

- [ ] `git diff upstream-main` shows expected changes (branding only)
- [ ] `cd installer && go build ./cmd/dotfiles` succeeds
- [ ] `cd installer && go vet ./...` passes
- [ ] `rg dotfiles --count installer/` matches expectations
- [ ] Branding audit in CI passes
- [ ] `.downstream/version.json` updated with new sha and tag
