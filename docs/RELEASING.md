# Releasing

How a downstream release is cut, and what has to be true before it is. Everything
below is derived from reading `.github/workflows/release.yml` end to end. Each
claim names the job or the step it comes from, so a change to the workflow can be
checked against this page instead of against habit.

This is the only release document. The pre-flight list used to live in a second page,
`docs/release-checklist.md`, which was retired after its links were repointed here, the way
`docs/RELEASES.md` was retired before it. The two pages split the work - "before the tag" in one,
"after the tag" in the other - and the split is what let one half drift while the other was being
read. The list is now the section below, in reading order with the rest.

## Versioning

The project follows [Semantic Versioning](https://semver.org/):

- **Breaking changes**: MAJOR version bump.
- **New features (backward compatible)**: MINOR version bump.
- **Bug fixes**: PATCH version bump.

Downstream versions are independent of upstream tags; the first downstream release was `v0.1.0`. The
tag names the release and nothing in the working tree holds the version, so the policy decides the
number the tag carries and the workflow injects that tag into the binary (see the next section).

## The tag is the release

Nothing in the repository holds the release version.

- `installer/cmd/dotfiles/main.go` declares `var Version = "dev"` (with `Commit` and `Date` beside it)
  and hands all three to the TUI. The `build` job's `Build` step overwrites them at link time from the
  tag, `github.sha` and the build clock, so a build made by this workflow reports
  `dotfiles v0.5.1 (9f3c2a1, 2026-10-04)` and a build made by hand reports `dev build`
  (`installer/internal/tui/version.go`, `installer/internal/tui/update_check.go`).
- `.downstream/version.json`'s `downstream.version` is not read by any code. The only keys read
  anywhere are `upstream.latest_sha` and `upstream.latest_tag`, by `.github/workflows/upstream-watch.yml`.

So the tag names the release, and the numbers a user sees - the tag, the release title, the asset
names, `dotfiles --version` and the update check's comparison - all come from that one string.

## Pre-flight: what must be true before the tag exists

The tag starts `.github/workflows/release.yml`, which builds, checksums, verifies and drafts a
GitHub Release. Nothing in that workflow checks any of the items below, so this list is the only
thing standing between an unfinished `main` and a release named after it.

Do not tag until every box is ticked. A tag is cheap to create and expensive to move.

- [ ] **The tree is clean and the release commit is the one you think it is.**
      `git status --porcelain` prints nothing, and `git rev-parse HEAD` is the commit whose
      `CHANGELOG.md` section you are about to publish. Everything the workflow builds comes from that
      commit, so anything uncommitted or staged elsewhere is not in the release.
- [ ] **`make preflight` is green.** `bash scripts/preflight.sh` must end
      `PREFLIGHT PASSED - 7/7 steps, each one a CI job or part of one.` It runs `gofmt`, `go vet`, a
      build with the `--help` smoke test, the whole Go suite, `shellcheck`, the branding audit with
      `ci.yml`'s own exclusion globs, and `gitleaks` over the commits the branch adds. It cannot run
      the Docker E2E matrix, Termux, the E2E image-list job or the macOS toolchain, and it says so in
      its footer: a green preflight is not 14 of 14 checks.
- [ ] **`main` equals `origin/main`.** `git fetch origin && git rev-parse HEAD origin/main` prints the
      same commit twice. A local commit that has not been pushed is a release nobody else has, and the
      release job checks out the remote tag.
- [ ] **No open issue this release claims to close.** `gh issue list --state open` must not list any
      issue referenced by a `Closes #N` in the pull requests merged since the last tag. To list the
      claims: `git log --oneline v0.5.0..HEAD` for the range and `gh pr view <n> --json body` per pull
      request.
- [ ] **`CHANGELOG.md` matches the release notes.** The section for this version exists, its heading
      carries the release date, and it is the text that will replace the draft's generated notes -
      because the workflow's `generate_release_notes: true` writes GitHub's own list from merged pull
      requests and never reads `CHANGELOG.md`. If the two disagree, the published notes are the ones
      users will read. See "Publish the draft" below.
- [ ] **CI is green on the release commit.** `gh run list --branch main --limit 5` - the `CI` and
      `E2E Installer Tests` runs for the commit being tagged, not for an ancestor.
- [ ] **The three formatting and hygiene checks are clean**, which preflight covers but which are
      worth naming because they are the ones a reviewer asks about: `gofmt -l` inside `installer/`
      prints nothing, `go vet ./...` passes, `git diff --check` prints nothing.
- [ ] **No golden moved.** `git status --short installer/internal/tui/testdata/` prints nothing. A
      golden that changed without an `-update` run is a render change nobody reviewed.
- [ ] **The version metadata is bumped.** `.downstream/version.json`'s `downstream.version` is the new
      version. Nothing reads it, so this is a bookkeeping item.
- [ ] **The tag does not already exist.** `git tag -l vX.Y.Z` prints nothing and
      `gh release view vX.Y.Z` exits non-zero.
- [ ] **The Homebrew formula is identifiable.** `homebrew-tap/Formula/dotfiles.rb` and
      `albersg/homebrew-tap`'s `Formula/dotfiles.rb` are the same file before the release starts, so
      the version and four hashes can be written into both afterwards. The version one release behind
      is expected; a formula that differs from the in-repo copy in any other way is not.
- [ ] **The chain can be checked without publishing, if you want to.**
      `gh workflow run release.yml -f version=vX.Y.Z` builds the four assets, writes `SHA256SUMS` and
      `MANIFEST.txt`, and runs the whole verification, and creates no release. It is the same job the
      tag push runs, so a chain problem is found before the tag rather than after it. See
      "Verifying the chain without publishing".

## The quick path

```bash
VERSION=v0.6.0
PREVIOUS=v0.5.0

# 0. Pre-flight. Every box above must be ticked before a tag exists.

# 1. Write the version metadata on the commit that will be tagged.
#    CHANGELOG.md:  ## [v0.5.0]        ->  ## [v0.6.0] — <release date>
#    .downstream/version.json:  "version": "0.5.0"  ->  "0.6.0"
#    Neither is read by the workflow; the first one is the notes in step 5.

# 2. Tag it and push the tag. The tag is what starts the workflow.
git tag -a "$VERSION" -m "Release $VERSION"
git push origin "$VERSION"

# 3. Watch the run. Six jobs: build (one per target), checksums, verify, release,
#    verify-published.
gh run list --workflow release.yml --limit 1
gh run watch <run-id> --exit-status
#    The run is only green if the chain held: verify checks the artifacts this run
#    built, and verify-published re-downloads the release's own assets and checks
#    them again -- the step that used to be done by hand for v0.5.0.

# 4. Publish the draft. The workflow creates it, and still writes GitHub's own
#    generated notes; the body has to be replaced with the CHANGELOG section.
gh release view "$VERSION"
gh release edit "$VERSION" --notes-file /tmp/notes.md
gh release edit "$VERSION" --draft=false
#    or do both in the web UI: gh release view "$VERSION" --web

# 5. Update the tap, both copies, with one command. See "The Homebrew tap".

# 6. Verify the installed binary is the released one.
brew update
brew uninstall dotfiles 2>/dev/null
brew install albersg/tap/dotfiles
dotfiles --version            # dotfiles v0.6.0 (<commit>, <date>)
dotfiles --check-update       # dotfiles v0.6.0 (<commit>, <date>)
                              # Up to date with v0.6.0 (just now)
```

## What the tag starts

`release.yml`'s `on:` block takes a tag push (`v*`) and a manual run
(`workflow_dispatch`) with a `version` and a `publish` flag. `permissions: contents: write` is only
ever used by the release job.

Five jobs run in order, each on `ubuntu-latest`.

| Job | Needs | What it does |
|-----|-------|--------------|
| `build` | - | Four-way matrix: one build per `goos`/`goarch`, run with `working-directory: installer`. Resolves the version, builds, smokes the linux/amd64 asset, attests its provenance, uploads it |
| `checksums` | `build` | Downloads all four artifacts, writes `SHA256SUMS`, writes `MANIFEST.txt`, uploads both |
| `verify` | `build`, `checksums` | Assembles `dist/` and checks the chain: asset to `SHA256SUMS`, `SHA256SUMS` to `MANIFEST.txt` to the formula, and the binary's own `--version`. Verifies the attestations |
| `release` | `build`, `checksums`, `verify` | Collects the four binaries, `SHA256SUMS` and `MANIFEST.txt` into `dist/` and creates a **draft** GitHub Release. Runs on a tag push, or on a manual run with `publish` on |
| `verify-published` | `build`, `verify`, `release` | Downloads the assets **back from the release** and checks them against the published `SHA256SUMS` and `MANIFEST.txt`, then verifies their attestations |

## What it builds

The matrix is exactly four targets, each naming the asset it becomes:

| `goos` | `goarch` | Release asset |
|--------|----------|---------------|
| `darwin` | `arm64` | `dotfiles-darwin-arm64` |
| `darwin` | `amd64` | `dotfiles-darwin-amd64` |
| `linux` | `arm64` | `dotfiles-linux-arm64` |
| `linux` | `amd64` | `dotfiles-linux-amd64` |

The `Build` step is:

```bash
go build -trimpath \
  -ldflags="-s -w -X main.Version=${VERSION} -X main.Commit=${GITHUB_SHA} -X main.Date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -o "<asset>" ./cmd/dotfiles
```

with `GOOS`, `GOARCH` from the matrix and `CGO_ENABLED: 0`. So each asset is a statically linked
binary with symbols and the DWARF table stripped, and its identity is injected into `./cmd/dotfiles` -
the same three values `--version` reads. There is no Windows target, and the formula below has no
Windows branch, so the two agree by construction. `installer/internal/tui/update_check_test.go` holds
that agreement: the assets the installer can self-update from are derived from the same four names and
compared with the matrix, in both directions.

## What it uploads

The `release` job assembles `dist/` from the artifacts and hands it to
`softprops/action-gh-release@v2` with `draft: true` and `generate_release_notes: true`. So a run
produces, attached to a draft release for the pushed tag:

- the four binaries above, named exactly as the matrix names them;
- `SHA256SUMS`, from the `checksums` job;
- `MANIFEST.txt`, from the same job.

Two consequences worth stating, because they are not obvious from the outside:

- **The release is a draft.** Nothing is public until step 4 of the quick path un-drafts it, and
  `brew install` cannot work before then even though the assets exist.
- **`generate_release_notes: true` does not read `CHANGELOG.md`.** GitHub builds the body from the
  merged pull requests it can see for the tag. The hand-written section is the better body - it
  carries the direct-to-main commits and the grouping the repository uses - so replace the generated
  text rather than accepting it. Generating a changelog from commit subjects instead would replace a
  document that says *why* each change was made with a list of what changed; the curated section
  stays, and the generated list is the scaffold it replaces.

## The manifest, the four hashes, and who checks them

`MANIFEST.txt` is what makes a download verifiable by hand:

```
# dotfiles v0.6.0
# built from <40-character commit> on <RFC 3339 time>
# verify with: sha256sum -c SHA256SUMS

<64-character hash>  dotfiles-linux-amd64
...
```

The `checksums` job computes each hash from the artifact itself, so the manifest and `SHA256SUMS`
are two readings of the same bytes rather than one copied from the other. The `verify` job compares
them asset by asset, so a file that drifts between the two is caught before the draft exists.

`SHA256SUMS` is written by the `checksums` job from inside `artifacts/`, so:

- each line names its asset *with* the artifact directory as a prefix, so the file reads
  `hash  ./dotfiles-linux-amd64/dotfiles-linux-amd64`;
- the order is the filesystem's, not the matrix's. **Match each hash to its asset by name, never by
  line order** - which is what the `verify` job does, and what the installer's own update check does
  (`releaseSumsLine` in `installer/internal/tui/update_check.go`).

The `verify` job re-derives the chain from the bytes rather than from the run that produced them:

1. every asset hashes to the `SHA256SUMS` line whose own name it carries;
2. `MANIFEST.txt` and `SHA256SUMS` agree asset by asset, and the manifest names this version and this
   commit;
3. `dotfiles-linux-amd64 --version` prints that version and that commit - the one asset that can run
   on the runner, so a linker line that stopped injecting the identity is caught here;
4. when the formula already names this version, its four `sha256` values are the four published
   hashes. Between releases the formula names the previous version, which the step says out loud
   instead of either failing or passing in silence.

`verify-published` then does the same work against the assets **downloaded back from the release**,
which is the check that was done by hand for `v0.5.0`: hashing what was actually uploaded rather than
what was prepared. Both jobs verify the build attestations
(`actions/attest-build-provenance@v2` in the `build` job, `gh attestation verify` in the two verify
jobs), so a binary can be traced to the workflow and the commit that produced it, not only to the
checksum file beside it.

To read the hashes yourself:

```bash
gh release download "$VERSION" --pattern 'dotfiles-*' --pattern 'SHA256SUMS' --pattern 'MANIFEST.txt' --dir /tmp/assets
(cd /tmp/assets && sha256sum -c SHA256SUMS)      # sha256sum on Linux, shasum -a 256 on macOS
gh attestation verify /tmp/assets/dotfiles-linux-amd64 --repo albersg/dotfiles
```

## Verifying the chain without publishing

The `verify` job does not need a release to run. Start the workflow by hand:

```bash
gh workflow run release.yml -f version=v0.6.0
gh run watch "$(gh run list --workflow release.yml --limit 1 --json databaseId --jq '.[0].databaseId')" --exit-status
```

It builds the four assets, writes `SHA256SUMS` and `MANIFEST.txt`, runs the whole verification, and
creates no release, because `publish` defaults to off. A `-f publish=true` is what drafts one. So the
pre-flight item above is a real rehearsal of the release rather than a promise about it, and the same
job's `Verify the chain` step is the exact script - copy the step, point it at a `dist/` directory and
set `VERSION` and `GITHUB_SHA`, and it runs on a laptop.

The two static halves are guarded by `go test ./installer/internal/tui/`:
`TestTheWorkflowBuildsExactlyTheAssetsTheInstallerCanResolve` compares the matrix with the asset names
the self-update can resolve, and the release-prose guard derives the asset names from the workflow and
fails when a document names one the workflow never builds.

## The Homebrew tap

`brew install albersg/tap/dotfiles` reads `albersg/homebrew-tap`, a separate public repository, at
`Formula/dotfiles.rb`. The same file lives in this repository at `homebrew-tap/Formula/dotfiles.rb` as
the source of truth.

**No workflow touches the tap.** `release.yml` has `permissions: contents: write` scoped to this
repository's own release, and there is no token, no deploy key and no `repository_dispatch` for the
tap anywhere in `.github/`. That is the one thing this release process needs and the repository does
not have: cross-repository credentials. The step is manual - and it is one command, run in each
checkout, rather than four hashes copied by hand from a file whose order is not the matrix's:

```bash
VERSION=v0.6.0
gh release download "$VERSION" --pattern SHA256SUMS --dir /tmp/tap-update

python3 - "$VERSION" /tmp/tap-update/SHA256SUMS <<'PY'
import pathlib, re, sys

version, sums_path = sys.argv[1], sys.argv[2]
hashes = dict(re.findall(r'^([0-9a-f]{64})  \./([^/]+)/\2$', pathlib.Path(sums_path).read_text(), re.M))
if len(hashes) != 4:
    raise SystemExit(f'{sums_path} does not name four assets: {sorted(hashes)}')

for path in ('homebrew-tap/Formula/dotfiles.rb', 'Formula/dotfiles.rb'):
    formula = pathlib.Path(path)
    if not formula.exists():
        print(f'skip {path}: not here')
        continue
    text = formula.read_text()
    text = re.sub(r'^  version ".*"$', f'  version "{version.lstrip("v")}"', text, count=1, flags=re.M)
    for asset, digest in hashes.items():
        text = re.sub(
            r'(releases/download/v#\{version\}/' + asset + r'"\n      sha256 ")[0-9a-f]{64}',
            lambda m: m.group(1) + digest,
            text,
        )
    formula.write_text(text)
    print(f'wrote {path}')
PY

# The tap is updated only when both copies change together.
(cd albersg-homebrew-tap && git commit -am "dotfiles $VERSION" && git push)
git commit -am "dotfiles $VERSION: formula" && git push

# Confirm the formula carries what the release published, asset by asset.
for asset in dotfiles-darwin-arm64 dotfiles-darwin-amd64 dotfiles-linux-arm64 dotfiles-linux-amd64; do
  grep -q "$(grep -F "./$asset/$asset" /tmp/tap-update/SHA256SUMS | cut -d' ' -f1)" homebrew-tap/Formula/dotfiles.rb \
    || echo "MISSING: $asset"
done
```

The placeholders a brand-new formula would ship with (`PLACEHOLDER_*_SHA256`) are not valid hashes and
none are present in either copy today; a formula that still carries one installs nothing. The
`verify` job checks the formula for you whenever its version already matches the release, so a tap
commit made before the release can never go unnoticed.

## What the workflow does not do

| Not done | Consequence |
|----------|-------------|
| Updates the tap | The one command above, in two repositories: the credentials do not exist here |
| Publishes the release | The draft waits for step 4 |
| Reads `CHANGELOG.md` | The notes come from GitHub's generated list until they are replaced |
| Bumps `.downstream/version.json` | Whatever is committed at the tag is what the next sync sees |
| Checks that the tag is on `main` | Any commit can be tagged; the pre-flight list is what guards this |
| Signs or notarises the darwin binaries | No `codesign`/`notarytool` step exists; the formula installs the asset as published |
| Runs the macOS asset | Only linux/amd64 can execute on the runner, so its `--version` is the one that is checked |

## If a run fails

The tag exists and the workflow is idempotent enough to be re-run, so re-run the failed job rather
than moving the tag:

```bash
gh run list --workflow release.yml --limit 3
gh run rerun <run-id> --failed
```

A failed run can leave a draft release with a partial set of assets. Whatever the next attempt does
with an existing draft for the same tag, the thing to check before publishing is the resulting asset
list rather than the run's log:

```bash
gh release view "$VERSION" --json isDraft,assets --jq '{isDraft, assets: [.assets[] | .name]}'
```

**If `verify-published` fails, the release's bytes are not the bytes that were prepared.** Do not
un-draft it and do not "fix" the checksums: re-run the release job so the draft is built again from
the verified artifacts, and only publish once `verify-published` is green.

## After publishing

Not pre-flight, but the release is not finished without them.

- [ ] The draft is published, and its body is the `CHANGELOG.md` section for this version rather than
      GitHub's generated list.
- [ ] The tap command above has run in both repositories, and the formula's four hashes came from the
      `SHA256SUMS` lines for the matching asset names.
- [ ] `brew update && brew install albersg/tap/dotfiles && dotfiles --version` prints the new tag, and
      `dotfiles --check-update` says the install is up to date with it - which is the same check the
      installer runs on its own, so a release nobody can update to is noticed here.
- [ ] The `albersg/homebrew-tap` commit is pushed, because an unpushed commit there is a `brew install`
      that still serves the previous release, and nothing fails when it does.

## Related

- `.github/workflows/release.yml` - the source of every claim above.
- `CHANGELOG.md` - the curated notes, and the text the published release carries.
- `docs/contributing.md` - the process around a change, including what a release claims to close.
