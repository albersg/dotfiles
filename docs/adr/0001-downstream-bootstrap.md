# ADR 0001: Downstream Bootstrap Strategy

## Status

Accepted (2026-07-29)

## Context

The `albersg/dotfiles` repository was a minimal GNU Stow repo (~60 files, ~872 LOC). The goal is to transform it into a maintainable downstream distribution of Gentleman.Dots v2.12.2 with its own identity, cross-platform TUI installer, CI/CD pipeline, and Homebrew tap.

Options considered:

1. **Incremental migration**: Add Gentleman.Dots incrementally while keeping existing stow structure
2. **Full replacement**: Replace the entire repo with Gentleman.Dots v2.12.2, then rebrand
3. **Monorepo approach**: Keep both in separate directories with a shared build system

## Decision

**Full replacement with systematic rebranding.**

The entire repository is replaced with Gentleman.Dots v2.12.2 as the new base. All branding (binary name, environment variables, URLs, TUI strings, package directories) is changed from "Gentleman" to "dotfiles" via scripted find-and-replace plus manual review.

## Rationale

- Gentlemen.Dots v2.12.2 provides a mature TUI installer, cross-platform support, CI/CD, and package structure — all out of the box
- Incremental migration would require rebuilding each component, duplicating years of upstream work
- Full replacement preserves upstream's exact structure, making future syncs and merge conflict resolution tractable
- Scripted rebranding (50+ Go files) ensures consistency; manual review catches edge cases

## Consequences

- **Positive**: Immediate access to all upstream features (TUI installer, trainer, E2E tests, CI)
- **Positive**: Clean separation via `upstream-main` branch makes `git diff upstream-main` trivial
- **Positive**: Go module path kept as `github.com/Gentleman-Programming/Gentleman.Dots/installer` minimizes merge conflicts
- **Negative**: ~1,200 files in the initial commit; large initial review surface
- **Negative**: Rebranding must be re-applied on every upstream merge
- **Negative**: Personal OpenCode config (`stow/opencode/`) must be preserved separately

## Rollback

Original repo tagged `archive/pre-downstream`. Restore: force-push tag to main, remove upstream remote.
