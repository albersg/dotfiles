# Releasing

How a downstream release is cut. Everything below is derived from reading
`.github/workflows/release.yml` end to end - each claim names the line it comes from, so a change to
the workflow can be checked against this page instead of against habit.

`docs/RELEASES.md` is the older procedure page and still holds the versioning policy. Where the two
disagree, this page is the one that matches the workflow. This page and `docs/release-checklist.md`
split the work: the checklist is what must be true *before* a tag exists, this page is the sequence
that follows.

## The tag is the release

Nothing in the repository holds the release version.

- `installer/cmd/dotfiles/main.go` declares `var Version = "dev"` and hands it to the TUI. The `build`
  job overwrites it at link time from the tag (`release.yml:49-54`), so a build made by this workflow
  reports the tag and a build made by hand reports `dev build`
  (`installer/internal/tui/version.go:20-27`).
- `.downstream/version.json`'s `downstream.version` is not read by any code. The only keys read
  anywhere are `upstream.latest_sha` and `upstream.latest_tag`, by
  `.github/workflows/upstream-watch.yml:35-36`.

So the tag names the release, and the four numbers a user sees - the tag, the release title, the
asset names and `dotfiles --version` - all come from that one string.

## The quick path

```bash
# 0. Pre-flight. Every line must hold before a tag exists.
#    See docs/release-checklist.md for the full list and what each one proves.
git status --porcelain                       # prints nothing
make preflight                               # ends "PREFLIGHT PASSED - 7/7 steps"
git fetch origin && git rev-parse HEAD origin/main   # both lines identical
gh issue list --state open                   # no issue this release claims to close

# 1. Set the date and the version metadata, on the commit that will be tagged.
#    CHANGELOG.md:  ## [v0.4.0]        ->  ## [v0.4.0] — <release date>
#    .downstream/version.json:  "version": "0.3.0"  ->  "0.4.0"
#    Neither is checked by the workflow; the notes in step 5 are written from the first one.

# 2. Tag it and push the tag. The tag is what starts the workflow.
git tag -a v0.4.0 -m "Release v0.4.0"
git push origin v0.4.0

# 3. Watch the run.
gh run list --workflow release.yml --limit 1
gh run watch <run-id> --exit-status

# 4. Read the four hashes. Each line is "<hash>  ./<asset>/<asset>".
gh run download <run-id> -n SHA256SUMS -D /tmp/sums && cat /tmp/sums/SHA256SUMS
#    Once the release is published, the same file is one of its assets:
gh release download v0.4.0 --pattern SHA256SUMS --dir /tmp && cat /tmp/SHA256SUMS

# 5. Publish the draft. The workflow creates it, and still writes GitHub's own
#    generated notes; the body has to be replaced with the CHANGELOG section.
gh release view v0.4.0
gh release edit v0.4.0 --notes-file /tmp/v0.4.0-notes.md
gh release edit v0.4.0 --draft=false
#    or do both in the web UI: gh release view v0.4.0 --web

# 6. Update the tap, in both copies. Nothing automates this step; see below.
#    homebrew-tap/Formula/dotfiles.rb                (this repository, source of truth)
#    albersg/homebrew-tap: Formula/dotfiles.rb       (what Homebrew reads)

# 7. Verify the installed binary is the released one.
brew update
brew uninstall dotfiles 2>/dev/null
brew install albersg/tap/dotfiles
dotfiles --version                           # dotfiles v0.4.0
```

## What the tag starts

`release.yml:3-6` - `on: push: tags: ['v*']`. Nothing else starts it: there is no `workflow_dispatch`,
no schedule and no branch trigger, and `permissions: contents: write` (`release.yml:8-9`) is only ever
used by the release job.

Three jobs run in order, each on `ubuntu-latest`.

| Job | Needs | What it does |
|-----|-------|--------------|
| `build` (`release.yml:12-60`) | - | Four-way matrix: one build per `goos`/`goarch`, run with `working-directory: installer` (`release.yml:30-32`) |
| `checksums` (`release.yml:62-84`) | `build` | Downloads all four artifacts, writes `SHA256SUMS`, uploads it as an artifact |
| `release` (`release.yml:86-117`) | `checksums` | Collects the four binaries and `SHA256SUMS` into `dist/` and creates a **draft** GitHub Release |

## What it builds

The matrix (`release.yml:15-29`) is exactly four targets, each naming the asset it becomes:

| `goos` | `goarch` | Release asset |
|--------|----------|---------------|
| `darwin` | `arm64` | `dotfiles-darwin-arm64` |
| `darwin` | `amd64` | `dotfiles-darwin-amd64` |
| `linux` | `arm64` | `dotfiles-linux-arm64` |
| `linux` | `amd64` | `dotfiles-linux-amd64` |

The build step (`release.yml:43-54`) is:

```bash
VERSION=${GITHUB_REF#refs/tags/}                    # the tag name, leading "v" included
go build -trimpath -ldflags="-s -w -X main.Version=${VERSION}" -o "<asset>" ./cmd/dotfiles
```

with `GOOS`, `GOARCH` from the matrix and `CGO_ENABLED: 0` (`release.yml:44-47`). So each asset is a
statically linked binary with symbols and the DWARF table stripped, and the version is injected into
`main.Version` in the `./cmd/dotfiles` package - the same variable `--version` reads. There is no
Windows target, and the formula below has no Windows branch, so the two agree by construction.

## What it uploads

The `release` job assembles `dist/` from the artifacts (`release.yml:101-108`) and hands it to
`softprops/action-gh-release@v2` with `draft: true` and `generate_release_notes: true`
(`release.yml:110-117`). So a run produces, attached to a draft release for the pushed tag:

- the four binaries above, named exactly as the matrix names them;
- `SHA256SUMS`, copied from the `SHA256SUMS` artifact (`release.yml:107`).

Two consequences worth stating, because they are not obvious from the outside:

- **The release is a draft.** Nothing is public until step 5 of the quick path un-drafts it, and
  `brew install` cannot work before then even though the assets exist.
- **`generate_release_notes: true` does not read `CHANGELOG.md`.** GitHub builds the body from the
  merged pull requests it can see for the tag. The hand-written section is the better body - it
  carries the direct-to-main commits and the grouping the repository uses - so replace the generated
  text rather than accepting it.

## The four hashes

The `checksums` job hashes the file *inside* each downloaded artifact, from inside the `artifacts/`
directory (`release.yml:72-79`):

```bash
cd artifacts
find . -type f -name 'dotfiles-*' | while read -r f; do
  sha256sum "$f" >> ../SHA256SUMS
done
```

Two things follow, and both were checked against the published `v0.3.0` assets:

- each line names its asset *with* the artifact directory as a prefix, so the file reads
  `hash  ./dotfiles-linux-amd64/dotfiles-linux-amd64`;
- the order is the filesystem's, not the matrix's. The `v0.3.0` file lists linux-amd64, darwin-amd64,
  linux-arm64, darwin-arm64. **Match each hash to its asset by name, never by line order.**

The hash is of the bytes that are released: the `release` job copies each artifact into `dist/` with
`cp` (`release.yml:104-106`), and the same bytes are attached to the release. That was verified for two
`v0.3.0` assets by fetching them and hashing them, and both matched both the `SHA256SUMS` line and the
formula entry.

To read the four hashes before publishing, take the artifact from the run
(`gh run download <run-id> -n SHA256SUMS -D /tmp/sums`). To confirm them after publishing, hash the
published assets themselves - `shasum -a 256` on macOS, `sha256sum` on Linux:

```bash
gh release download v0.4.0 --pattern 'dotfiles-*' --dir /tmp/assets
(cd /tmp/assets && shasum -a 256 dotfiles-*)
```

## The Homebrew tap

`brew install albersg/tap/dotfiles` reads `albersg/homebrew-tap`, a separate public repository, at
`Formula/dotfiles.rb`. The same file lives in this repository at `homebrew-tap/Formula/dotfiles.rb` as
the source of truth; at the v0.3.0 release the two copies are byte-identical, and the recipe below
assumes they still are.

**No workflow touches the tap.** `release.yml` has `permissions: contents: write` scoped to this
repository's own release, and there is no token, no deploy key and no `repository_dispatch` for the
tap anywhere in `.github/`. That is the one thing this release process needs and the repository does
not have: cross-repository credentials. The tap step is manual, and it is the step most likely to be
forgotten, because nothing fails when it is skipped - the next `brew install` just serves the previous
release.

For each release, two things change in `Formula/dotfiles.rb`:

1. `version "0.3.0"` becomes the new version **without** the leading `v`. The four `url` lines
   interpolate `v#{version}`, so they do not change.
2. The four `sha256` lines, each from the `SHA256SUMS` line for the asset whose name matches it.

Nothing else differs between releases: `desc`, `homepage`, `license`, the four `on_macos`/`on_linux`
blocks, the `install` method and the `test` block all carry over untouched, and the file's `test`
block runs `dotfiles --help`, so a `brew install` that succeeds has already exercised the binary.

The placeholders a brand-new formula would ship with (`PLACEHOLDER_*_SHA256`) are not valid hashes and
none are present in either copy today; a formula that still carries one installs nothing.

## What the workflow does not do

| Not done | Consequence |
|----------|-------------|
| Updates the tap | Step 6, by hand, in two repositories |
| Publishes the release | The draft waits for step 5 |
| Reads `CHANGELOG.md` | The notes come from GitHub's generated list until they are replaced |
| Bumps `.downstream/version.json` | Whatever is committed at the tag is what the next sync sees |
| Checks that the tag is on `main` | Any commit can be tagged; the pre-flight list is what guards this |
| Signs or notarises the darwin binaries | No `codesign`/`notarytool` step exists; the formula installs the asset as published |
| Verifies the hashes against the published assets | Step 4 reads them from the artifact; hashing the assets is a separate, stronger check |
| Has a manual trigger | A re-run uses the existing run: `gh run rerun <run-id> --failed`. Do not delete and re-push the tag |

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
gh release view v0.4.0 --json isDraft,assets --jq '{isDraft, assets: [.assets[] | .name]}'
```

## Related

- `docs/release-checklist.md` - what must be true before the tag exists.
- `docs/RELEASES.md` - versioning policy and the older procedure text.
- `.github/workflows/release.yml` - the source of every claim above.
