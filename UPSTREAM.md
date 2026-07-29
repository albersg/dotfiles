# Upstream: Gentleman.Dots

## Tracking

This repository is a downstream distribution of [Gentleman.Dots](https://github.com/Gentleman-Programming/Gentleman.Dots) by [Gentleman Programming](https://github.com/Gentleman-Programming).

- **Upstream remote**: `https://github.com/Gentleman-Programming/Gentleman.Dots.git`
- **Mirror branch**: `upstream-main` — exact copy of upstream's `main` branch
- **Base version**: v2.12.2 (tag `v2.12.2`, commit `02584500`)

## Sync Procedure

See [docs/UPSTREAM-SYNC.md](docs/UPSTREAM-SYNC.md) for the full automated sync workflow.

In short:

1. The `upstream-watch` CI workflow checks for new tags every 6 hours
2. New tags create an `integration/upstream-vX.Y.Z` branch
3. A PR is opened with a diff report
4. Conflicts generate a GitHub issue instead of silently failing

For manual sync:

```bash
git fetch upstream
git checkout upstream-main
git merge upstream/main
# Resolve conflicts, then:
git push origin upstream-main
```

## What Never to Modify

These files are direct copies from upstream and should **never** be modified downstream:

| Path | Reason |
|------|--------|
| `installer/go.mod` | Go module path kept as `github.com/Gentleman-Programming/Gentleman.Dots/installer` for merge compatibility |
| `installer/go.sum` | Auto-generated, must match upstream |
| `installer/internal/tui/trainer/` | Trainer exercises and game logic — modification would break upstream patches |
| `skills/` (AI skills) | Skill files track upstream structure for merge compatibility |
| `homebrew-tap/` (originals) | Upstream formula kept as reference |

## Upstream Links

- **Repository**: [Gentleman-Programming/Gentleman.Dots](https://github.com/Gentleman-Programming/Gentleman.Dots)
- **Issues**: [Upstream Issues](https://github.com/Gentleman-Programming/Gentleman.Dots/issues)
- **Community**: [Gentleman Programming Discord](https://discord.gg/gentleman-programming)

## Attribution

Gentleman.Dots is created and maintained by Gentleman Programming. This downstream distribution is unofficial and not endorsed by the upstream maintainers.
