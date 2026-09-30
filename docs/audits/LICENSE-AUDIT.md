# License Audit

## Overview

This document provides a comprehensive audit of all licenses applicable to the dotfiles downstream distribution.

## Upstream License

| Project | License | File |
|---------|---------|------|
| Gentleman.Dots | MIT | [LICENSE](/LICENSE) |

## Go Module Dependencies

The installer binary links against the following Go modules. This list is derived from `installer/go.mod` and `installer/go.sum`.

### Direct Dependencies

Four modules are imported by the installer's own code; the rest of the graph is transitive. The
versions are the ones `installer/go.mod` pins.

| Module | Version | License | Type |
|--------|---------|---------|------|
| github.com/charmbracelet/bubbletea | v1.3.10 | MIT | TUI framework and runtime |
| github.com/charmbracelet/lipgloss | v1.1.0 | MIT | Terminal styling and colour profiles |
| github.com/charmbracelet/x/exp/teatest | v0.0.0-20251215102626-e0db08df7383 | MIT | End-to-end TUI testing (test-only) |
| github.com/muesli/termenv | v0.16.0 | MIT | Terminal capability and colour detection |

Every direct dependency is MIT. The transitive set is permissive as well - MIT, with
`golang.org/x/sys` and `golang.org/x/text` under BSD-3-Clause - and no copyleft (GPL, AGPL, LGPL)
dependency is linked into the installer in any form.

### Corrected 2026-09-29

The table above used to list `charmbracelet/bubbles` and `gopkg.in/yaml.v3`, neither of which the
installer depends on, and it presented two transitive modules as direct ones while omitting
`muesli/termenv`, which became a direct import. It now matches `installer/go.mod`.

## Shell Configurations

### Oh My Zsh

- **License**: MIT
- **Path**: `dotfiles-zsh/.oh-my-zsh/`
- **Note**: Bundled as configuration, not compiled into the binary

### Powerlevel10k

- **License**: MIT
- **Path**: Referenced in `.p10k.zsh`
- **Note**: Installed separately by Oh My Zsh

## Neovim Plugins

The Neovim configuration references plugins that are installed by LazyVim. These plugins have their own licenses. The dotfiles distribution does not bundle these plugins — they are installed at runtime by the user's Neovim.

## Fonts

- **Iosevka Term Nerd Font**: SIL Open Font License 1.1
- Installed via the system package manager, not bundled in this repository

## Compliance Notes

- All licenses are permissive (MIT, Apache 2.0, SIL OFL)
- No GPL, AGPL, or other copyleft licenses are present
- The MIT license allows modification and redistribution with attribution preserved
- Upstream attribution is maintained in NOTICE and docs/ATTRIBUTION.md

## Audit Date

2026-07-29

## Auditor

Automated audit as part of downstream bootstrap. Manual verification recommended before first release.
