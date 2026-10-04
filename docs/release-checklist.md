# Release checklist

What must be true **before** `git tag vX.Y.Z` exists. The tag starts
`.github/workflows/release.yml`, which builds, checksums and drafts a GitHub Release; nothing in that
workflow checks any of the items below, so this page is the only thing standing between an unfinished
`main` and a release named after it. The sequence that follows the tag is in `docs/RELEASING.md`.

Do not tag until every box is ticked. A tag is cheap to create and expensive to move: the release
workflow has no manual trigger, and moving a tag means re-pushing it.

## The five that matter

- [ ] **The tree is clean and the release commit is the one you think it is.**
      `git status --porcelain` prints nothing, and `git rev-parse HEAD` is the commit whose
      `CHANGELOG.md` section you are about to publish. Everything the workflow builds comes from that
      commit, so anything uncommitted or staged elsewhere is not in the release.
- [ ] **`make preflight` is green.** `bash scripts/preflight.sh` must end
      `PREFLIGHT PASSED - 7/7 steps, each one a CI job or part of one.` It runs `gofmt`, `go vet`, a
      build with the `--help` smoke test, the whole Go suite, `shellcheck`, the branding audit with
      `ci.yml`'s own exclusion globs, and `gitleaks` over the commits the branch adds - in about 35 s,
      against roughly 400 s in CI. It cannot run the Docker E2E matrix (ubuntu, fedora, arch, alpine;
      debian on main), Termux, the E2E image-list job or the macOS toolchain, and it says so in its
      footer: a green preflight is not 14 of 14 checks. If a check needs CI, let CI run it on `main`.
- [ ] **`main` equals `origin/main`.** `git fetch origin && git rev-parse HEAD origin/main` prints the
      same commit twice. A local commit that has not been pushed is a release nobody else has, and the
      release job checks out the remote tag.
- [ ] **No open issue this release claims to close.** `gh issue list --state open` must not list any
      issue referenced by a `Closes #N` in the pull requests merged since the last tag. A release that
      names an issue it did not close is a promise the notes cannot keep. To list the claims:
      `git log --oneline v0.3.0..HEAD` for the range and `gh pr view <n> --json body` per pull request.
- [ ] **`CHANGELOG.md` matches the release notes.** The section for this version exists, its heading
      carries the release date, and it is the text that will replace the draft's generated notes -
      because the workflow's `generate_release_notes: true` writes GitHub's own list from merged pull
      requests and never reads `CHANGELOG.md`. If the two disagree, the published notes are the ones
      users will read. See the "Publish the draft" step in `docs/RELEASING.md`.

## The rest of the pre-flight

- [ ] **CI is green on the release commit.**
      `gh run list --branch main --limit 5` - the `CI` and `E2E Installer Tests` runs for the commit
      being tagged, not for an ancestor.
- [ ] **The three formatting and hygiene checks are clean**, which preflight covers but which are worth
      naming because they are the ones a reviewer asks about:
      `gofmt -l` inside `installer/` prints nothing, `go vet ./...` passes, `git diff --check` prints
      nothing.
- [ ] **No golden moved.** `git status --short installer/internal/tui/testdata/` prints nothing. A
      golden that changed without a `-update` run is a render change nobody reviewed.
- [ ] **The version metadata is bumped.** `.downstream/version.json`'s `downstream.version` is the new
      version. Nothing reads it - only `upstream.latest_sha` and `upstream.latest_tag` are read, by
      `upstream-watch.yml` - so this is a bookkeeping item, and it is still the item the repository's
      older checklist asks for.
- [ ] **The tag does not already exist.** `git tag -l vX.Y.Z` prints nothing and
      `gh release view vX.Y.Z` exits non-zero. A tag that exists makes the release workflow run, fail
      or overwrite, depending on which job replays.
- [ ] **The Homebrew formula is identifiable.** `homebrew-tap/Formula/dotfiles.rb` and
      `albersg/homebrew-tap`'s `Formula/dotfiles.rb` are the same file before the release starts, so
      the version and four hashes can be written into both afterwards. The version one release behind
      is expected; a formula that differs from the in-repo copy in any other way is not. See
      `docs/RELEASING.md` for the exact lines that change.

## After publishing

Not pre-flight, but the release is not finished without them.

- [ ] The draft is published, and its body is the `CHANGELOG.md` section for this version rather than
      GitHub's generated list.
- [ ] The four hashes in the formula came from the `SHA256SUMS` line for the matching asset name, not
      from the line's position - the order is the filesystem's.
- [ ] `brew update && brew install albersg/tap/dotfiles && dotfiles --version` prints the new tag.
- [ ] The `albersg/homebrew-tap` commit is pushed, because an unpushed commit there is a `brew install`
      that still serves the previous release, and nothing fails when it does.
