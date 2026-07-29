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
rg Gentleman --count installer/  # Should match expectations

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

After publishing, the release triggers a PR to `albersg/homebrew-tap` with:
- Updated formula with new binary URLs and SHA256 hashes
- Version bump

Verify the formula and merge the PR.

## Release Checklist

- [ ] All CI checks pass on main
- [ ] Branding audit passes
- [ ] `go build ./cmd/dotfiles` succeeds on at least one platform locally
- [ ] CHANGELOG.md updated with release notes
- [ ] `.downstream/version.json` version bumped
- [ ] Tag created and pushed
- [ ] GitHub Release draft reviewed and published
- [ ] Homebrew formula updated
