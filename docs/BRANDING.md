# Branding Guide

## Overview

This documentation covers the branding conventions for the dotfiles downstream distribution.

## Brand Elements

### Name

- **Project**: `dotfiles` (lowercase, one word)
- **Repository**: `albersg/dotfiles`
- **Binary**: `dotfiles`

### Environment Variables

All installer environment variables use the `DOTFILES_` prefix:

| Variable | Purpose |
|----------|---------|
| `DOTFILES_DRY_RUN` | Simulate installation without changes |
| `DOTFILES_TEST_MODE` | Run in test mode (temp directory) |
| `DOTFILES_VERBOSE` | Enable verbose logging |
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

The CI pipeline includes a branding audit that fails if the upstream distribution's name appears outside of:

- `NOTICE` — attribution statement
- `docs/ATTRIBUTION.md` — attribution
- `docs/adr/` — references to upstream in ADRs
- `.downstream/version.json` — upstream metadata
- `Brewfile` — installed tools published by a third party

The `Brewfile` exclusion is a dependency, not an attribution. One CLI tool on
this machine is published by the upstream project and is installed from its Go
module path, so the file has to name that module verbatim; there is no other way
to declare it. No tap and no formula of that project is declared any more.
Everything else in this repository names no upstream project.

Run manually:

```bash
rg dotfiles --count --glob '!NOTICE' --glob '!docs/adr/*' \
  --glob '!docs/ATTRIBUTION.md' --glob '!.downstream/*'
# Should return 0 matches
```

## When Branding Changes

If you need to change the branding further:

1. Update this guide
2. Apply changes consistently across all files
3. Run the branding audit
4. Update CI allowlist if needed

## Don't Change

- Go module path (`github.com/albersg/dotfiles/installer`)
- `LICENSE` — upstream MIT license preserved
