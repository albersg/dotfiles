# ADR 0002: Go Module Path — Deferred Rename

## Status

Accepted (2026-07-29)

## Context

The upstream Go module path is `github.com/Gentleman-Programming/Gentleman.Dots/installer`. For the downstream distribution with the binary renamed to `dotfiles` and the repository at `albersg/dotfiles`, the natural module path would be `github.com/albersg/dotfiles/installer`.

However, changing the module path would:

- Invalidate all 50+ Go files' import statements
- Break upstream merge compatibility (every import line becomes a conflict)
- Require updating `go.mod`, `go.sum`, and all internal references

## Decision

**Keep the upstream module path** (`github.com/Gentleman-Programming/Gentleman.Dots/installer`) **for now.** Defer the rename to a dedicated follow-up PR after the downstream distribution is stable.

## Rationale

- Minimizes merge conflicts with upstream — import lines stay identical
- The module path is internal to Go tooling; users interact with the binary, not the module
- Binary name is already `dotfiles` (changed in `cmd/dotfiles/main.go`)
- A dedicated rename PR can handle the full module path change, including updating CI, test expectations, and documentation in one focused change

## Consequences

- Go imports still reference `github.com/Gentleman-Programming/Gentleman.Dots/installer` — visually inconsistent but functionally neutral
- Any downstream contributors must understand this is deferred, not accidental
- The rename PR will touch most Go files and should be done in a single atomic commit

## When to Rename

After the downstream distribution has:

- A stable initial release (v0.1.0)
- CI passing on all platforms
- At least one successful upstream sync cycle

## References

- [Go Modules: Module Path](https://go.dev/ref/mod#module-path)
- ADR 0001: Downstream Bootstrap Strategy
