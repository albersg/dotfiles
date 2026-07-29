# Downstream: dotfiles

## Overview

This is a downstream distribution of [Gentleman.Dots v2.12.2](https://github.com/Gentleman-Programming/Gentleman.Dots) maintained by [albersg](https://github.com/albersg).

## Differences from Upstream

### Branding

| Aspect | Upstream | Downstream |
|--------|----------|------------|
| Project name | Gentleman.Dots | dotfiles |
| Binary name | `gentleman-dots` / `gentleman.dots` | `dotfiles` |
| Environment vars | `GENTLEMAN_DRY_RUN`, `GENTLEMAN_VERBOSE`, etc. | `DOTFILES_DRY_RUN`, `DOTFILES_VERBOSE`, etc. |
| Clone URL | `github.com/Gentleman-Programming/Gentleman.Dots.git` | `github.com/albersg/dotfiles.git` |
| Homebrew tap | `Gentleman-Programming/tap` | `albersg/tap` |
| Package dirs | `GentlemanZsh`, `GentlemanFish`, etc. | `dotfiles-zsh`, `dotfiles-fish`, etc. |
| Go module path | `github.com/Gentleman-Programming/Gentleman.Dots/installer` | Same (deferred rename per ADR 0002) |

### Repository Structure

- **Upstream tracking**: `upstream-main` branch mirrors upstream/main exactly
- **Metadata**: `.downstream/version.json` records sync state
- **Documentation**: All downstream docs in `docs/` with ADRs for key decisions

### Tools

Identical to upstream. No tools added, removed, or substituted.

## Pending Upstream Contributions

None yet. Future downstream changes that would benefit upstream will be submitted as PRs to Gentleman.Dots.

## Rebase Strategy

When rebranding changes conflict with upstream merges:

1. Apply upstream changes first (keep upstream semantics)
2. Re-apply branding overlay on top
3. Manual review of all `Gentleman` → `dotfiles` replacements
4. Verify `go build ./...` and `go vet ./...` pass

## Versioning

- Downstream versions are independent of upstream tags
- First downstream release will be v0.1.0
- `.downstream/version.json` tracks the mapping between versions
