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

The CI pipeline includes a branding audit that fails if any residual "Gentleman" string is found outside of:

- `NOTICE` — attribution statement
- `docs/ATTRIBUTION.md` — attribution
- `docs/adr/` — references to upstream in ADRs
- `.downstream/version.json` — upstream metadata

Run manually:

```bash
rg Gentleman --count --glob '!NOTICE' --glob '!docs/adr/*' \
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

- Go module path (`github.com/Gentleman-Programming/Gentleman.Dots/installer`) — deferred rename
- `LICENSE` — upstream MIT license preserved
- `gentle-ai` references — separate project, use upstream URL
