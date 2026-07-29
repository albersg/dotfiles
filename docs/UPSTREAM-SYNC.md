# Upstream Sync Procedure

## Automated Sync (CI)

The `.github/workflows/upstream-watch.yml` workflow handles automated sync:

1. **Schedule**: Runs every 6 hours + manual dispatch
2. **Detection**: Compares `upstream/main` with `.downstream/version.json`
3. **New tag found**: Creates `integration/upstream-vX.Y.Z` branch
4. **PR creation**: Opens PR with diff report (`--head integration/upstream-vX.Y.Z --base main`)
5. **Conflict handling**: Creates GitHub issue with conflict details (no force push)

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
