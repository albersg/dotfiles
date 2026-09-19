# Release Procedure

## Versioning

This project follows [Semantic Versioning](https://semver.org/).

- **Breaking changes**: MAJOR version bump
- **New features (backward compatible)**: MINOR version bump
- **Bug fixes**: PATCH version bump

Downstream versions are independent of upstream tags. The first release is v0.1.0.

## Creating a Release

### 1. Prepare

```bash
# Ensure main is up to date and clean
git checkout main
git pull origin main

# Verify everything passes
cd installer && go build ./cmd/dotfiles && go vet ./... && go test ./...
rg dotfiles --count installer/  # Should match expectations

# Update version
# Edit .downstream/version.json: bump downstream.version
```

### 2. Tag

```bash
git tag -a v0.1.0 -m "Release v0.1.0"
git push origin v0.1.0
```

### 3. Automated Release

The `.github/workflows/release.yml` workflow triggers on tag push and:

1. Builds binaries for all platforms:
   - `darwin-arm64`
   - `darwin-amd64`
   - `linux-arm64`
   - `linux-amd64`
2. Generates SHA256 checksums
3. Creates a **draft** GitHub Release with:
   - All binary artifacts
   - SHA256SUMS file
   - Auto-generated changelog

### 4. Publish

1. Review the draft release on GitHub
2. Edit release notes (add highlights, breaking changes, upgrade instructions)
3. Publish the release

### 5. Homebrew Update

Nothing automates this step. No workflow touches the tap, so the formula is updated
by hand, and it exists in two places that have to stay in step:

- `homebrew-tap/Formula/dotfiles.rb` in this repository, which is the source of truth
  and the copy the branding audit and code review can see.
- `albersg/homebrew-tap`, the repository Homebrew actually reads, which must be public
  for `brew install albersg/tap/dotfiles` to work without credentials.

Download the checksums the release generated:

```bash
gh release download v<version> --pattern SHA256SUMS --dir /tmp
cat /tmp/SHA256SUMS
```

The file lists each asset with the path it had inside the build job, so the four
hashes have to be matched by asset name, not by line order. Write the version and
the four hashes into `homebrew-tap/Formula/dotfiles.rb`, copy the file into the tap
repository, and commit both. The placeholders that ship with a new formula
(`PLACEHOLDER_*_SHA256`) are not valid hashes, so a formula that still carries one
installs nothing.

Verify end to end, which also confirms the hashes match the published assets:

```bash
brew uninstall dotfiles 2>/dev/null
brew install albersg/tap/dotfiles
dotfiles --version
```

The formula's own `test` block runs `dotfiles --help`, so a `brew install` that
succeeds has already exercised the binary.

## Release Checklist

- [ ] All CI checks pass on main
- [ ] Branding audit passes
- [ ] `go build ./cmd/dotfiles` succeeds on at least one platform locally
- [ ] CHANGELOG.md updated with release notes
- [ ] `.downstream/version.json` version bumped
- [ ] Tag created and pushed
- [ ] GitHub Release draft reviewed and published
- [ ] Homebrew formula updated
