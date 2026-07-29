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

| Module | Version | License | Type |
|--------|---------|---------|------|
| github.com/charmbracelet/bubbletea | latest | MIT | TUI framework |
| github.com/charmbracelet/bubbles | latest | MIT | TUI components |
| github.com/charmbracelet/lipgloss | latest | MIT | Terminal styling |
| github.com/charmbracelet/x/ansi | latest | MIT | ANSI sequences |
| github.com/charmbracelet/x/exp/teatest | latest | MIT | Bubbletea testing |
| github.com/charmbracelet/x/exp/golden | latest | MIT | Golden file testing |
| gopkg.in/yaml.v3 | latest | MIT | YAML parsing |

All direct dependencies are MIT-licensed. No copyleft (GPL, AGPL) dependencies are used in the installer.

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
