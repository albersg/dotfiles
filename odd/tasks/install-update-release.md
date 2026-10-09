# The installer's own release: checking it, updating from it, and shipping it

One front, one file. This document covers four things that are the same front: the installer knows
which release is published and can install it over itself; the release workflow proves the chain it
publishes instead of proving it by hand; the release process is one document instead of two; and the
tracker's own convention is written down here because this is the file that was open.

The inventory this started from, all of it re-checked in the tree:

- **No version check existed.** `api.github.com` appeared in the installer only in a test that
  forbids it elsewhere. Nothing read the latest release.
- **The workflow did not verify what it published.** `build` → `checksums` (`SHA256SUMS`) →
  `release` (a draft). The chain was checked by hand for `v0.5.0`, and by nobody for the rest.
- **No provenance.** No `attest-build-provenance`, no `id-token`, no signature anywhere.
- **The verification mechanism already existed.** `verifyFileSHA256`
  (`installer/internal/tui/agent_skills.go`) with the order the OfficeCLI step uses: stage beside the
  destination → verify → make executable → `rename`. It was applied to OfficeCLI and not to the
  installer itself.
- **`--version` carried no commit and no date.** `Version = "dev"` injected with `-X main.Version`,
  nothing else.
- **Two release documents.** `docs/RELEASING.md` (233 lines) and `docs/release-checklist.md` (71).

## Decisions, and what they cost

| Decision | Why | Cost, accepted |
|---|---|---|
| The check is a **file read on the startup path** and a **command** for the request | The interface must never wait on the network; the state directory is already the place for state that survives a run (`theme.json`, `last-install.json`) | A first-ever run has no cached answer until its own check lands (about a second), so the utilities row appears when it does -- see the open item below |
| A failed check is **recorded as unknown, never as current** | "You are up to date" is a claim about GitHub. `Summary()` cannot say it without a tag, and `--check-update` exits 2 for unknown, 1 for newer, 0 for current | Three exit codes to document; a script that ignores them treats unknown as current |
| The automatic check runs on **every real open**, gated only by `updateAutoCheckAllowed()` and the record's own lifetime, and never in a test binary | The user asked for the check when dotfiles opens; the command is off the update loop, so it cannot delay the first frame, and a failed check is recorded rather than shown. The test-only gate keeps a guard from reaching the network or the machine's real state directory | A run with `DOTFILES_ANIM=0` now refreshes like any other; the utility is a **button on the main menu**, and it is silent when the check cannot answer |
| The update row is a **main-menu button**, offered when a newer release is published and the file is the installer's own, and the cursor is **held by label** when it arrives late | The user asked for it in the main menu, not buried in Utilities; the check answers after the first frame, so an index-based cursor would hand the next Enter to the inserted row (the A5 lesson) | The main-menu goldens gained the row; `TestUtilitiesGolden` did not move, because its state never had an answer to draw |
| The swap **reuses `verifyFileSHA256`** and the OfficeCLI order, and keeps the replaced binary as `<path>.previous` | One order, already proven in this repository; the previous release stays reachable by hand | A stale `.previous` file is left behind after an update (deliberate) |
| A binary **Homebrew owns is refused**, with `brew upgrade dotfiles` named | The same ownership question the theme switch asks of a config file (`themeOwnedByUs` / `themeInstallerFileIsOurs`), asked of a binary instead | Detection knows one package manager, by path prefix and `/Cellar/`; anything else it cannot see |
| `--check-update` and `--self-update` are **two entries to the same functions**, not two implementations | Two routes diverge -- WSL taught this repository that | Two flags, one `main.go` print path each |
| The workflow's chain check runs **twice**: `verify` on the artifacts, `verify-published` on the assets downloaded back | They are different facts. The first catches a broken chain; the second catches an upload that did not carry what was prepared, which is the failure that reached users silently before | One more job, one duplicate `sha256sum` loop |
| A **manual `workflow_dispatch`** was added, defaulting to `publish: false` | It is how the new verification step is tested without publishing anything: the same job, the same script, the artifacts from the run | The old workflow's "no manual trigger" property is gone; the run's `publish` input is what keeps it from being a release |
| The **changelog stays hand-written**; `MANIFEST.txt` and the attestation carry the verifiable half | `CHANGELOG.md` says why each change was made and groups it; a commit-subject list would replace that with what changed. The release notes stay generated and are replaced by the curated section | One generated body a human still replaces before publishing |
| `docs/release-checklist.md` was **deleted**, after the links were repointed at the one runbook | The order is the repository's own precedent (commit `477d8a9` retired `docs/RELEASES.md` the same way), so no link is ever broken and the guard never reads a missing file | One historical mention stays in `CHANGELOG.md`'s v0.5.0 entry, where the repository also kept the `docs/RELEASES.md` mention |
| The pre-flight list moved **into** `RELEASING.md` in reading order (before the tag, then after it) | The split was what let one half drift from `.github/workflows/release.yml` while the other was being read -- the same failure `docs/RELEASES.md` was retired for | One long page |

## The two channels: stable and dev

The check has always read the latest published release. A user asked for a choice between
"stable" and "dev", and the base already knew everything except how to choose: it reads a
release, compares versions numerically, verifies the asset against the release's own
`SHA256SUMS`, replaces atomically, keeps the old binary, and refuses a Homebrew-owned file. The
channel decides nothing about any of those rules; it decides which tag is asked about.

**dev is the newest pre-release, not a build from source.** The installer carries no compiler
and only installs an asset it can verify against that release's `SHA256SUMS`; the release
workflow builds its four assets from a tag (`on: push: tags: v*`), so the only artifact a dev
channel could serve is a pre-release. A pre-release is therefore the natural dev artifact: the
same four assets, the same sums file, the same verified swap, tagged on `main`. When no
pre-release exists, dev says the check could not answer rather than serving the stable release
under a dev name.

| Decision | Why | Cost, accepted |
|---|---|---|
| **stable** stays the default and keeps `/releases/latest`; **dev** reads the `/releases` list and takes the newest non-draft pre-release | GitHub's latest endpoint cannot return a pre-release by definition, so dev has to read the list; stable is byte-for-byte unchanged | One more response shape to test, and a dev answer needs a pre-release to have been published |
| The choice is remembered in `update-check.json`, in the record's `channel` field | One thing, one place: the state file already holds the answer, so the channel travels with it, and a channel-only record (no answer yet) is readable so a switch before the check lands is not forgotten | `readUpdateRecord` now accepts a record whose only fact is its channel |
| The record names the channel **every** answer came from, including a failure | An answer without its source is an opinion; the repository already applies this with `unknown` and with "where each answer came from" | `Summary` prefixes `dev channel: ` and the screen draws a `Channel` row; stable stays unnamed because it is the default |
| Three entries: `--channel stable|dev`, the main menu's **Update channel** row, and the remembered record | The flag is scriptable, the row is discoverable, the record is what survives the run | `--check-update` and `--self-update` read the chosen channel; the TUI resolves the flag over the record, and a cached answer for another channel is dropped |
| The channel row is offered **once the record names a channel** | Every check writes one, success or failure, so in practice the row is there with the first answer; a run that has never checked shows nothing about a choice it has not made | A fresh run's first second has no row, exactly as the update row already behaves, and the cursor is held by label when it arrives |

## Tracker convention

The convention lives in [`odd/README.md`](../README.md), which is where a contributor looks for it: one
file per front, one owner, a fact that belongs elsewhere is linked rather than copied, existing files
are never merged to tidy the directory, and a file carries evidence rather than intentions. This file
is one front, which is why it exists at all.

The numbers behind it are measured: **11 of the last PRs touch `odd/tasks/`**, and **4 conflicts in
one night happened only because two branches were editing the same task file**.

## Evidence

RED first, against the behaviour, with the stub deliberately written the unsafe way round (no
checksum, no ownership question, failure recorded as current, a hash matched by line number, the
automatic check armed everywhere):

```
--- FAIL: TestRefreshUpdateRecordRecordsAFailureInsteadOfAtag (0.00s)
    Latest = "dev" after a failed check, want empty: an unreachable GitHub is not a version
    a failed check left no reason behind, so the interface cannot tell the user why it does not know
--- FAIL: TestReleaseSumsLineFindsTheAssetByName/dotfiles-linux-amd64 (0.00s)
    hash = "819776e084df...", want "5861314d7fcc...": the hash has to be matched by asset name, never by line number
--- FAIL: TestApplySelfUpdateRefusesABinaryHomebrewOwns (0.00s)
    refusal = "GET https://github.com/albersg/dotfiles/releases/download/v0.5.1/dotfiles-linux-amd64: Not Found",
      want it to name the command that does own this binary: `brew upgrade dotfiles`
    the refusal still made 1 requests: the ownership question is answered from the path, before anything is downloaded
--- FAIL: TestApplySelfUpdateRefusesATamperedAsset (0.00s)
    a binary that does not match the release's own checksum was installed
--- FAIL: TestAutoUpdateCheckIsNeverArmedInATestBinary (0.00s)
    updateAutoCheckAllowed() is true inside a test binary: a test would reach the network and write to
      the machine's real state directory
```

16 guards failed on the stub and pass on the implementation. The one GREEN failure that followed was
my own assertion comparing "up to date" against "Up to date"; the implementation was right and the
test was fixed, which is recorded here rather than papered over.

The channel work was written the same way, with a stub whose dev channel read the stable endpoint,
whose record omitted its channel, whose state never named it, and whose menu offered no row:

```
--- FAIL: TestDevChannelReadsTheNewestPreReleaseNotTheStableRelease
    dev tag = "v1.0.0", want the newest pre-release v1.2.0-dev.1: a dev channel that answers with
      the stable release is the stable channel
    dev requests = [.../releases/latest], want exactly [.../releases?per_page=30]
--- FAIL: TestDevChannelSkipsDraftsAndReleasedEntries
    latestReleaseTagForChannel(dev): GET .../releases/latest: Not Found
--- FAIL: TestTheRecordNamesTheChannelTheAnswerCameFrom
    record channel = "", want dev
    record latest = "", want the dev tag
    stored channel = "", want dev: an answer without its source is an opinion
--- FAIL: TestTheStateNamesTheDevChannel
    state.Channel = "", want dev
--- FAIL: TestSwitchingToDevDropsTheStableAnswerAndRemembersTheChoice
    the stable answer "v1.0.0" is still on the state: a stable answer is not a dev answer
--- FAIL: TestDevChannelWithoutANetworkSaysUnknownNotCurrent
    Summary() = "Unknown -- ...": want the dev channel named even when the check failed
    state.Channel = "", want dev even on failure
--- FAIL: TestCheckForUpdateUsesTheChosenChannel
    summary = "v1.0.0 is published; ...", want the dev tag in it
    the dev check read .../releases/latest: the dev channel must read the release list
--- FAIL: TestMainMenuOffersTheChannelRowAndSwitchesIt
    the main menu does not offer the channel row: [Start Installation ...]
```

GREEN: the same guards pass with the implementation, and the whole `internal/tui` and
`cmd/dotfiles` packages (3294 tests, goldens included) pass without a golden moving -- every
golden pins an `UpdateCheck` whose channel is empty, and the row is offered only when the state
names a channel.

The workflow's chain check was run locally against a fixture, using the step's **extracted** shell
rather than a copy of it:

- the positive case: `ok` for all four assets, the manifest agreeing with `SHA256SUMS`, the binary
  printing `dotfiles v0.0.0-chainproof (7cd4c3b, 2026-10-04)` and the formula carrying all four
  published hashes → exit 0;
- **the formula one release behind** (what a tag push really sees) → a notice, exit 0;
- **a stale formula hash** → `::error::the formula does not carry the hash this release publishes for
  dotfiles-linux-arm64 (3f9461...)`, exit 1;
- **a manifest hash that disagrees with `SHA256SUMS`** → `::error::dotfiles-linux-arm64 is missing
  from SHA256SUMS or from MANIFEST.txt`, exit 1;
- **a manifest that does not name the commit** → `::error::MANIFEST.txt does not name commit
  7cd4c3b22a63fbee210705e4dc01fb854811fbd7`, exit 1;
- **a build whose version is not the one being verified** → exit 1.

`make check` and the focused packages are recorded in the handoff for this front.

The retirement of `docs/release-checklist.md` shrank the release documents from three to two, so the
same change strengthened the guard that reads them rather than leaving it with less to check:
`docs/RELEASING.md` must now name **every job** `release.yml` declares (derived from the workflow's
`jobs:` block: `[build checksums verify release verify-published]`), no release document may cite a
workflow **line number** (the shape that made the retired pages stale), and the runbook must still
carry both halves it merged - the pre-flight items and the after-the-tag names. Both new guards were
probed for teeth: converting the runbook's `- [ ]` items to `- [x]` and appending a
`release.yml:49-54` citation failed them by name (`carries 0 pre-flight items`, `cites
"release.yml:4"`), and reverting the probe restored the file byte-identical (`cmp` verified).

## Remaining

- ~~The utilities row appears when the first check lands, not before it.~~ **Done:** the row is a
  main-menu button, offered only when a newer release is actually installable, and the two main-menu
  goldens were regenerated with it. The check now runs on every real open, so the answer arrives on
  its own; the button's late arrival is held by its label so the cursor does not move with it.
- ~~**`CHANGELOG.md` has no entry for this work.**~~ **Done:** the section is written at the top of
  `CHANGELOG.md` under `## [Unreleased]`, because no one can know the release date before the tag and the
  runbook is the thing that renames it (the quick path in `docs/RELEASING.md` uses `v0.6.0` as the next
  version). Writing `## [v0.6.0] — <date>` now would invent the half of the heading the tag decides. The
  v0.5.0 entry still names `docs/release-checklist.md` in that release's past tense; it is left alone as the
  same kind of historical mention the repository kept for `docs/RELEASES.md`.
- **`packageManagerOwner` knows Homebrew by path only.** A Homebrew install whose symlink cannot be
  resolved, or a copy installed by a script and later claimed by a package manager, is not detected.
  The refusal is therefore narrower than the rule.
- **The dev channel orders pre-releases by their numeric version.** `parseVersion` drops the
  `-rc1` suffix by design, so `v0.6.0-rc2` is not newer than `v0.6.0-rc1` and a further
  pre-release of the same version would read as up to date. Comparing the commit the build was
  made from would fix it; it is not this front.
- **dev assumes a pre-release is cut from `main`.** The channel reads the newest pre-release and
  does not check that its tag points at the head of `main`. A pre-release cut from another branch
  would be served, and a commit on `main` with no pre-release is invisible to it.
- **The published-asset verification has been rehearsed against a fixture, not against a real
  release.** `verify-published` has never run on GitHub; the next release is its first real run.
- **Only the linux/amd64 asset is executed.** The darwin and arm64 binaries are hashed and attested but
  never run by the workflow, because the runner cannot execute them.
