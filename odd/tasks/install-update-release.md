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
| The automatic check runs on the **drawing gate** (`Animating`), and never in a test binary | The one existing gate that distinguishes an interactive run from a piped one, and it is what keeps a test from reaching the network and from writing into the machine's real state directory | A run with `DOTFILES_ANIM=0` does not refresh by itself; `--check-update` and the screen's own row do |
| The swap **reuses `verifyFileSHA256`** and the OfficeCLI order, and keeps the replaced binary as `<path>.previous` | One order, already proven in this repository; the previous release stays reachable by hand | A stale `.previous` file is left behind after an update (deliberate) |
| A binary **Homebrew owns is refused**, with `brew upgrade dotfiles` named | The same ownership question the theme switch asks of a config file (`themeOwnedByUs` / `themeInstallerFileIsOurs`), asked of a binary instead | Detection knows one package manager, by path prefix and `/Cellar/`; anything else it cannot see |
| `--check-update` and `--self-update` are **two entries to the same functions**, not two implementations | Two routes diverge -- WSL taught this repository that | Two flags, one `main.go` print path each |
| The workflow's chain check runs **twice**: `verify` on the artifacts, `verify-published` on the assets downloaded back | They are different facts. The first catches a broken chain; the second catches an upload that did not carry what was prepared, which is the failure that reached users silently before | One more job, one duplicate `sha256sum` loop |
| A **manual `workflow_dispatch`** was added, defaulting to `publish: false` | It is how the new verification step is tested without publishing anything: the same job, the same script, the artifacts from the run | The old workflow's "no manual trigger" property is gone; the run's `publish` input is what keeps it from being a release |
| The **changelog stays hand-written**; `MANIFEST.txt` and the attestation carry the verifiable half | `CHANGELOG.md` says why each change was made and groups it; a commit-subject list would replace that with what changed. The release notes stay generated and are replaced by the curated section | One generated body a human still replaces before publishing |
| `docs/release-checklist.md` was **deleted**, after the links were repointed at the one runbook | The order is the repository's own precedent (commit `477d8a9` retired `docs/RELEASES.md` the same way), so no link is ever broken and the guard never reads a missing file | One historical mention stays in `CHANGELOG.md`'s v0.5.0 entry, where the repository also kept the `docs/RELEASES.md` mention |
| The pre-flight list moved **into** `RELEASING.md` in reading order (before the tag, then after it) | The split was what let one half drift from `.github/workflows/release.yml` while the other was being read -- the same failure `docs/RELEASES.md` was retired for | One long page |

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

- **The utilities row appears when the first check lands, not before it.** On a machine that has never
  checked, the row is absent for the first second of the first run. Making it unconditional means
  moving `testdata/TestUtilitiesGolden.golden` (an added menu row changes that frame), which is one
  regenerated golden and a surface this front did not hold.
- **`CHANGELOG.md` has no entry for this work.** It is the pre-flight's own item ("the section for this
  version exists, its heading carries the release date"), and the file was not in this front's
  surfaces. Its v0.5.0 entry also still names `docs/release-checklist.md` in the past tense of that
  release; the repository kept the same kind of mention for `docs/RELEASES.md`, so it was left alone.
- **`packageManagerOwner` knows Homebrew by path only.** A Homebrew install whose symlink cannot be
  resolved, or a copy installed by a script and later claimed by a package manager, is not detected.
  The refusal is therefore narrower than the rule.
- **The published-asset verification has been rehearsed against a fixture, not against a real
  release.** `verify-published` has never run on GitHub; the next release is its first real run.
- **Only the linux/amd64 asset is executed.** The darwin and arm64 binaries are hashed and attested but
  never run by the workflow, because the runner cannot execute them.
