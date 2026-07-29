# Git Branching Strategy

## Branches

| Branch | Purpose | Protected |
|--------|---------|-----------|
| `main` | Downstream development — all PRs target this | Yes |
| `upstream-main` | Exact mirror of upstream's `main` — never merge directly | Force-push OK |

## Branch Naming

Feature branches follow conventional naming:

```
feat/<description>      # New features
fix/<description>       # Bug fixes
docs/<description>      # Documentation
chore/<description>     # Maintenance, CI, tooling
refactor/<description>  # Code restructuring
```

## Workflow

### Feature Development

```bash
git checkout main
git checkout -b feat/my-feature
# Make changes, commit
git push -u origin feat/my-feature
# Open PR targeting main
```

### Upstream Sync

```bash
git fetch upstream
# A new tag triggers:
#   - checkout upstream-main
#   - merge upstream/main
#   - create integration/upstream-vX.Y.Z branch
#   - open PR
```

### Release

```bash
git checkout main
git tag v0.1.0
git push --tags
# CI builds binaries for all platforms and creates a GitHub Release
```

## Protected Branches

- `main`: Requires PR review. No force push. CI must pass.
- `upstream-main`: Mirror only. Force push allowed for sync updates.

## Merging

- **Always merge, never squash** PRs to `main` — preserves commit history for upstream diffing
- Rebase feature branches before merging if they have fallen behind `main`
- Never rebase `upstream-main` — it must track upstream exactly
