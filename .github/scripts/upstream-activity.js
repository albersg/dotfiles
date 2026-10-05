'use strict';

// Upstream activity tracker.
//
// Finds (or creates) one persistent tracker issue in the destination repository
// and appends one comment per newly observed upstream event: commits on the
// upstream default branch, and newly created upstream issues and pull requests.
//
// Design notes:
// - Baseline on first activation: the checkpoint records the current upstream
//   head SHA and run time, so historical upstream activity is never backfilled.
// - Commit detection is a persisted SHA/history comparison, not a timestamp
//   filter. GitHub `listCommits?since=<time>` filters by commit date, so a commit
//   that only becomes reachable now (a cherry-pick, an imported/backdated commit,
//   a merge of an old branch) would be silently skipped if its author/committer
//   date predates the cursor. Instead the tracker persists the last observed head
//   SHA and asks GitHub to compare that SHA against the current head; every commit
//   reachable from the new head but not the old one is reported regardless of its
//   dates.
// - History divergence is fail-closed. If the saved head is no longer comparable
//   (a force-push/rebase makes the comparison `diverged` or `behind`), the run
//   records nothing new for commits and leaves the commit cursor untouched rather
//   than guessing a delta.
// - Issue and pull request detection stays time-based: newly created items are
//   found through a persisted `issues_since` timestamp.
// - The checkpoint lives in a machine-readable block inside the tracker issue
//   body, so it needs no repository write access and stays out of tag/version
//   metadata (`.downstream/version.json` remains the release-sync source of truth).
// - Every comment carries a stable HTML-comment marker. Before posting, the
//   runner lists existing comments and skips markers it already sees, so a retry
//   after a partial failure cannot duplicate an event.
// - Cursors never advance past an event that was not recorded.
// - The upstream repository slug is never hardcoded here: the workflow passes it
//   through configuration (see `.github/workflows/upstream-watch.yml`).

const STATE_START = '<!-- upstream-activity-state';
const STATE_END = '-->';
const STATE_PATTERN = /<!-- upstream-activity-state\s*\n([\s\S]*?)\n-->/;
const MARKER_PATTERN = /<!-- upstream-activity:([a-z]+):([^\s>]+) -->/g;
const STATE_VERSION = 2;

const EVENT_LABEL = {
  commit: 'commit',
  issue: 'issue',
  pr: 'pull request',
};

function markerFor(type, id) {
  return `<!-- upstream-activity:${type}:${id} -->`;
}

function defaultNow() {
  return new Date().toISOString().replace(/\.\d{3}Z$/, 'Z');
}

function stateBlockFor(state) {
  return `${STATE_START}\n${JSON.stringify(state, null, 2)}\n${STATE_END}`;
}

function parseState(body) {
  const match = STATE_PATTERN.exec(body || '');
  if (!match) return { found: false, state: null, error: null };
  try {
    const state = JSON.parse(match[1]);
    if (!state || typeof state !== 'object' || typeof state.cursors !== 'object') {
      return { found: true, state: null, error: 'state block is missing a cursors object' };
    }
    return { found: true, state, error: null };
  } catch (error) {
    return { found: true, state: null, error: error.message };
  }
}

function withState(body, state) {
  const block = stateBlockFor(state);
  const source = body || '';
  if (STATE_PATTERN.test(source)) {
    return source.replace(STATE_PATTERN, block);
  }
  return `${source.replace(/\s*$/, '')}\n\n${block}\n`;
}

function createBaselineState(activation, branch) {
  return {
    version: STATE_VERSION,
    branch: branch || 'main',
    activated_at: activation,
    last_run_at: activation,
    cursors: {
      // Last observed upstream head. `null` means "not established yet": the next
      // successful run stores the current head without backfilling commits.
      commits_sha: null,
      issues_since: activation,
    },
  };
}

function renderTrackerBody(state, activation) {
  return [
    '# Upstream activity tracker',
    '',
    'This issue is maintained by the `Upstream Watch` workflow (`track-activity` job).',
    'Each new commit on the upstream default branch, and each newly created upstream',
    'issue or pull request, is appended below as one comment.',
    '',
    `Tracking started at ${activation} (UTC), from the upstream head recorded then.`,
    'Activity from before activation is **not backfilled**;',
    'delete this issue only if you intend to restart tracking from scratch.',
    '',
    `Current branch tracked: \`${state.branch}\``,
    '',
    'The checkpoint block at the bottom is rewritten on every run. Do not edit it by hand.',
    '',
    stateBlockFor(state),
  ].join('\n');
}

function commitTimestamp(item) {
  return item?.commit?.committer?.date || item?.commit?.author?.date || null;
}

function selectNewCommits(commits) {
  // The caller (a head-SHA comparison) has already restricted this list to commits
  // reachable from the new head but not the saved head, so no date filtering is
  // applied here; that is what keeps backdated commits visible. A malformed entry
  // throws instead of being dropped, so the commit cursor is never advanced past
  // a commit that was not reported.
  return commits
    .map((item) => {
      if (!item || typeof item.sha !== 'string' || item.sha === '') {
        throw new Error('comparison returned a commit without a SHA');
      }
      const timestamp = commitTimestamp(item);
      const title = String(item.commit?.message || '').split('\n')[0].trim() || item.sha;
      return {
        type: 'commit',
        key: `commit:${item.sha}`,
        id: item.sha,
        sha: item.sha,
        title,
        author: item.commit?.author?.name || item.author?.login || 'unknown',
        timestamp: timestamp || '',
        url: item.html_url || '',
        marker: markerFor('commit', item.sha),
      };
    })
    .sort((a, b) => a.timestamp.localeCompare(b.timestamp) || a.sha.localeCompare(b.sha));
}

function selectNewIssues(items, cursor) {
  return items
    .filter((item) => item && Number.isInteger(item.number) && typeof item.created_at === 'string')
    .filter((item) => !cursor || item.created_at >= cursor)
    .map((item) => {
      const isPullRequest = Boolean(item.pull_request);
      const type = isPullRequest ? 'pr' : 'issue';
      return {
        type,
        key: `${type}:${item.number}`,
        id: String(item.number),
        number: item.number,
        title: item.title || `#${item.number}`,
        author: item.user?.login || 'unknown',
        timestamp: item.created_at,
        url: item.html_url || '',
        marker: markerFor(type, item.number),
      };
    })
    .sort((a, b) => a.timestamp.localeCompare(b.timestamp) || a.key.localeCompare(b.key));
}

function formatComment(event) {
  const lines = [`#### New upstream ${EVENT_LABEL[event.type]}: ${event.title}`];
  if (event.type === 'commit') {
    lines.push(`- SHA: \`${event.sha}\``);
  } else {
    lines.push(`- Number: #${event.number}`);
  }
  lines.push(`- Author: ${event.author}`);
  lines.push(`- Timestamp: ${event.timestamp}`);
  lines.push(`- Link: ${event.url}`);
  lines.push('');
  lines.push(event.marker);
  return lines.join('\n');
}

function collectMarkers(comments) {
  const markers = new Set();
  for (const comment of comments) {
    const body = comment?.body || '';
    MARKER_PATTERN.lastIndex = 0;
    let match = MARKER_PATTERN.exec(body);
    while (match) {
      markers.add(`<!-- upstream-activity:${match[1]}:${match[2]} -->`);
      match = MARKER_PATTERN.exec(body);
    }
  }
  return markers;
}

async function paginate(fetchPage, { pageSize, maxPages }) {
  const items = [];
  for (let page = 1; page <= maxPages; page += 1) {
    const batch = await fetchPage(page);
    if (!Array.isArray(batch)) {
      throw new Error(`Page ${page} did not return an array`);
    }
    items.push(...batch);
    if (batch.length < pageSize) return items;
  }
  throw new Error(`Pagination limit reached after ${maxPages} pages of ${pageSize}; refusing to silently truncate`);
}

async function runActivityTracker(config) {
  const {
    api,
    trackerTitle,
    now = defaultNow,
    branch,
    pageSize = 100,
    maxPages = 20,
    log = () => {},
  } = config;

  if (!api) throw new Error('runActivityTracker requires an api gateway');
  if (!trackerTitle) throw new Error('runActivityTracker requires a trackerTitle');

  const activation = now();

  // Serialized by the workflow `concurrency` group; this lookup keeps repeated
  // runs attached to the single tracker issue.
  const destinationIssues = await paginate(
    (page) => api.listTrackerIssues({ page, perPage: pageSize }),
    { pageSize, maxPages },
  );
  let tracker = destinationIssues.find((issue) => issue.title === trackerTitle) || null;
  let trackerBody;
  let state;

  if (tracker) {
    const parsed = parseState(tracker.body);
    if (parsed.found && !parsed.state) {
      throw new Error(
        `Tracker issue #${tracker.number} state block could not be parsed (${parsed.error}); `
        + 'refusing to continue so that historical activity is not backfilled',
      );
    }
    trackerBody = tracker.body || '';
    state = parsed.state || createBaselineState(activation, branch);
  } else {
    state = createBaselineState(activation, branch);
    tracker = await api.createTrackerIssue({ title: trackerTitle, body: renderTrackerBody(state, activation) });
    trackerBody = tracker.body;
    log(`Created activity tracker issue #${tracker.number} with baseline ${activation}`);
  }

  const comments = await paginate(
    (page) => api.listTrackerComments(tracker.number, { page, perPage: pageSize }),
    { pageSize, maxPages },
  );
  const recordedMarkers = collectMarkers(comments);

  const fetchErrors = [];
  const fetched = { commits: false, issues: false };
  let newCommits = [];
  let nextCommitsHead = null;
  let upstreamIssues = [];

  // Commit stream: compare the saved head against the current upstream head.
  // `commits_sha` is the safety cursor; it only moves once every commit in this
  // batch has been posted (or confirmed already recorded).
  try {
    const upstreamBranch = state.branch || branch || 'main';
    const savedHead = state.cursors.commits_sha || null;
    const headSha = await api.getUpstreamHead({ branch: upstreamBranch });
    if (!headSha || typeof headSha !== 'string') {
      throw new Error('could not resolve the current upstream head SHA');
    }
    nextCommitsHead = headSha;

    if (!savedHead) {
      // First activation: store the current head as the baseline, post nothing.
      newCommits = [];
    } else if (headSha === savedHead) {
      // Fast path: nothing moved, so no comparison request is needed.
      newCommits = [];
    } else {
      const firstPage = await api.compareUpstreamCommits({
        base: savedHead,
        head: upstreamBranch,
        page: 1,
        perPage: pageSize,
      });
      const status = firstPage?.status;
      if (status === 'ahead') {
        const expectedTotal = Number.isInteger(firstPage.total_commits)
          ? firstPage.total_commits
          : null;
        const commits = await paginate(
          async (page) => {
            if (page === 1) return Array.isArray(firstPage.commits) ? firstPage.commits : [];
            const data = await api.compareUpstreamCommits({
              base: savedHead,
              head: upstreamBranch,
              page,
              perPage: pageSize,
            });
            return data.commits;
          },
          { pageSize, maxPages },
        );
        if (expectedTotal !== null && commits.length < expectedTotal) {
          throw new Error(
            `comparison reported ${expectedTotal} commit(s) but only ${commits.length} were returned; `
            + 'refusing to silently truncate',
          );
        }
        newCommits = selectNewCommits(commits);
      } else if (status === 'identical') {
        newCommits = [];
      } else if (status === 'diverged' || status === 'behind') {
        throw new Error(
          `upstream history ${status}: saved head ${savedHead} is no longer comparable to ${headSha}. `
          + 'The commit cursor was not advanced; reset the tracker issue to re-baseline',
        );
      } else {
        throw new Error(`unexpected comparison status ${JSON.stringify(status)}`);
      }
    }
    fetched.commits = true;
  } catch (error) {
    fetchErrors.push(`commits: ${error.message}`);
    nextCommitsHead = null;
  }

  try {
    upstreamIssues = await paginate(
      (page) => api.listUpstreamIssues({ since: state.cursors.issues_since, page, perPage: pageSize }),
      { pageSize, maxPages },
    );
    fetched.issues = true;
  } catch (error) {
    fetchErrors.push(`issues: ${error.message}`);
  }

  const pending = [
    ...newCommits,
    ...(fetched.issues ? selectNewIssues(upstreamIssues, state.cursors.issues_since) : []),
  ].sort((a, b) => a.timestamp.localeCompare(b.timestamp) || a.key.localeCompare(b.key));

  const cursors = {
    commits_sha: state.cursors.commits_sha || null,
    issues_since: state.cursors.issues_since,
  };
  const posted = [];
  let outstandingCommits = fetched.commits ? newCommits.length : 0;

  function noteCommitProgress() {
    if (outstandingCommits <= 0) return;
    outstandingCommits -= 1;
    if (outstandingCommits === 0 && nextCommitsHead) {
      cursors.commits_sha = nextCommitsHead;
    }
  }

  if (fetched.commits && outstandingCommits === 0 && nextCommitsHead) {
    // Head is unchanged or this is the first run: advance the SHA cursor now so a
    // baseline (or no-op) run still checkpoints the observed head.
    cursors.commits_sha = nextCommitsHead;
  }

  async function persist(nextState) {
    trackerBody = withState(trackerBody, { ...nextState, last_run_at: activation });
    await api.updateTrackerIssue(tracker.number, { body: trackerBody });
  }

  for (const event of pending) {
    if (recordedMarkers.has(event.marker)) {
      advanceCursor(cursors, event);
      if (event.type === 'commit') noteCommitProgress();
      continue;
    }
    try {
      await api.createTrackerComment(tracker.number, formatComment(event));
    } catch (error) {
      try {
        await persist({ ...state, cursors: { ...cursors } });
      } catch (persistError) {
        throw new Error(`Failed to record upstream ${event.key}: ${error.message} (checkpoint save also failed: ${persistError.message})`);
      }
      throw new Error(`Failed to record upstream ${event.key}: ${error.message}`);
    }
    recordedMarkers.add(event.marker);
    posted.push(event);
    advanceCursor(cursors, event);
    if (event.type === 'commit') noteCommitProgress();
    // Persist after every event so a crash mid-run never replays recorded work.
    await persist({ ...state, cursors: { ...cursors } });
  }

  if (fetchErrors.length > 0) {
    // A fully recorded commit batch is checkpointed even when the other stream
    // failed, because its markers make any replay idempotent.
    await persist({ ...state, cursors: { ...cursors } });
    throw new Error(`Upstream activity tracker aborted with partial fetches: ${fetchErrors.join('; ')}`);
  }

  await persist({ ...state, cursors: { ...cursors } });

  return {
    trackerIssueNumber: tracker.number,
    baseline: activation,
    posted,
    cursors,
  };
}

function advanceCursor(cursors, event) {
  // Only issue/PR events still use a timestamp cursor; commit progress is tracked
  // by the head SHA in `cursors.commits_sha`.
  if (event.type === 'commit') return;
  if (!cursors.issues_since || event.timestamp > cursors.issues_since) {
    cursors.issues_since = event.timestamp;
  }
}

// Adapter for `actions/github-script` (`github.rest`). Keeping the SDK shape out
// of the tracking logic is what makes the behaviour testable without network.
function createOctokitGateway(client, config) {
  const rest = client.rest;
  const [upstreamOwner, upstreamRepo] = String(config.upstream).split('/');

  return {
    async listTrackerIssues({ page, perPage }) {
      const response = await rest.issues.listForRepo({
        owner: config.owner,
        repo: config.repo,
        state: 'all',
        per_page: perPage,
        page,
      });
      return response.data;
    },
    async createTrackerIssue({ title, body }) {
      const response = await rest.issues.create({
        owner: config.owner,
        repo: config.repo,
        title,
        body,
      });
      return response.data;
    },
    async updateTrackerIssue(number, { body }) {
      const response = await rest.issues.update({
        owner: config.owner,
        repo: config.repo,
        issue_number: number,
        body,
      });
      return response.data;
    },
    async listTrackerComments(number, { page, perPage }) {
      const response = await rest.issues.listComments({
        owner: config.owner,
        repo: config.repo,
        issue_number: number,
        per_page: perPage,
        page,
      });
      return response.data;
    },
    async createTrackerComment(number, body) {
      const response = await rest.issues.createComment({
        owner: config.owner,
        repo: config.repo,
        issue_number: number,
        body,
      });
      return response.data;
    },
    async getUpstreamHead({ branch }) {
      const response = await rest.repos.getBranch({
        owner: upstreamOwner,
        repo: upstreamRepo,
        branch,
      });
      return response.data.commit.sha;
    },
    async compareUpstreamCommits({ base, head, page, perPage }) {
      const response = await rest.repos.compareCommits({
        owner: upstreamOwner,
        repo: upstreamRepo,
        base,
        head,
        per_page: perPage,
        page,
      });
      return response.data;
    },
    async listUpstreamIssues({ since, page, perPage }) {
      const response = await rest.issues.listForRepo({
        owner: upstreamOwner,
        repo: upstreamRepo,
        state: 'all',
        sort: 'created',
        direction: 'desc',
        since,
        per_page: perPage,
        page,
      });
      return response.data;
    },
  };
}

module.exports = {
  createBaselineState,
  createOctokitGateway,
  formatComment,
  markerFor,
  parseState,
  renderTrackerBody,
  runActivityTracker,
  selectNewCommits,
  selectNewIssues,
  withState,
};
