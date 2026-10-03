# Branding Guide

## Overview

This documentation covers the branding conventions for the dotfiles downstream distribution.

## Brand Elements

### Name

- **Project**: `dotfiles` (lowercase, one word)
- **Repository**: `albersg/dotfiles`
- **Binary**: `dotfiles`

### Environment Variables

All installer environment variables use the `DOTFILES_` prefix. The table lists the
variables a user sets from outside the program. Internal wiring uses the same prefix
without appearing here — for example the `DOTFILES_WSL_HOST_CPUS` and
`DOTFILES_WSL_HOST_MEMORY_MB` overrides read by `installer/internal/system/host.go` —
and `dotfiles --help` is the source of truth for the user-facing set.

| Variable | Purpose |
|----------|---------|
| `DOTFILES_DRY_RUN` | Simulate installation without changes |
| `DOTFILES_TEST_MODE` | Run in test mode (temp directory) |
| `DOTFILES_VERBOSE` | Enable verbose logging |
| `DOTFILES_ANIM` | `=0` stops the animation and the tip rotation; the same as `--no-anim` |
| `DOTFILES_MOUSE` | `=0` stops reading the pointer; `=1` asks for it in Termux, where it is off by default |
| `DOTFILES_SPRITE` | `=0` draws the glyph cat instead of the shaded sprite; the same as `--no-sprite` |
| `DOTFILES_SYNC` | `=0` stops bracketing each frame in synchronized output |
| `DOTFILES_SHELL_STARTED` | Internal: prevents nested shell auto-start |

### Package Directories

All configuration packages use the `dotfiles-` prefix:

```
dotfiles-zsh/
dotfiles-fish/
dotfiles-nushell/
dotfiles-nvim/
dotfiles-kitty/
dotfiles-ghostty/
dotfiles-tmux/
dotfiles-zellij/
dotfiles-herdr/
```

## Branding Audit

The CI pipeline includes a branding audit that fails if the upstream
distribution's name — the token `gentleman`, in any case — appears outside of:

- `NOTICE` — attribution statement
- `LICENSE` — upstream MIT license preserved
- `docs/ATTRIBUTION.md` — attribution
- `docs/adr/` — references to upstream in ADRs
- `docs/audits/` — audit records
- `THIRD_PARTY_NOTICES.md` — third-party notices
- `CHANGELOG.md`, `DOWNSTREAM.md`, `UPSTREAM.md`, `docs/UPSTREAM-SYNC.md`,
  `docs/BRANDING.md` — traceability records
- `.downstream/version.json` — upstream metadata (a hidden path, so ripgrep
  skips it by default)
- `Brewfile` — installed tools published by a third party
- `go.sum` — dependency checksums
- `.github/workflows/upstream-watch.yml` — the watcher that names upstream
- `dotfiles-nvim/nvim/spell/` — the `en_US` dictionary

The `Brewfile` exclusion is a dependency, not an attribution. One CLI tool on
this machine is published by the upstream project and is installed from its Go
module path, so the file has to name that module verbatim; there is no other way
to declare it. No tap and no formula of that project is declared any more.
Everything else in this repository names no upstream project.

Run the same audit manually from the repository root:

```bash
rg -i 'gentleman' \
  --glob '!NOTICE' --glob '!LICENSE' --glob '!docs/adr/*' \
  --glob '!docs/ATTRIBUTION.md' --glob '!docs/audits/*' \
  --glob '!THIRD_PARTY_NOTICES.md' --glob '!CHANGELOG.md' \
  --glob '!DOWNSTREAM.md' --glob '!UPSTREAM.md' --glob '!docs/UPSTREAM-SYNC.md' \
  --glob '!docs/BRANDING.md' --glob '!go.sum' --glob '!Brewfile' \
  --glob '!dotfiles-nvim/nvim/spell/*' --glob '!.github/workflows/upstream-watch.yml' \
  --glob '!.gitignore' .
# Prints nothing on a clean tree. Any path it names is one the audit would fail on.
```

This searches for the upstream token, not for this distribution's own name: every
file here says `dotfiles`, so a search for that can only ever return matches.

## When Branding Changes

If you need to change the branding further:

1. Update this guide
2. Apply changes consistently across all files
3. Run the branding audit
4. Update CI allowlist if needed

## Don't Change

- Go module path (`github.com/albersg/dotfiles/installer`)
- `LICENSE` — upstream MIT license preserved
