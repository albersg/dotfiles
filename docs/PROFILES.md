# Profiles

## Overview

The profile system allows users to select different configuration presets. Profiles control which tools are installed and which configuration files are applied.

## Profile Definitions

Profiles are defined as YAML files in the `profiles/` directory:

| Profile | Description |
|---------|-------------|
| `minimal.yaml` | Shell + editor only. Fastest setup. |
| `default.yaml` | Shell + editor + terminal + multiplexer. Recommended. |
| `full.yaml` | Everything: all shells, terminals, multiplexers, trainer. |
| `macos.yaml` | macOS-specific overrides and tools. |
| `linux.yaml` | Linux-specific overrides and tools. |
| `termux.yaml` | Termux (Android) — limited tools, pkg-based. |
| `ci.yaml` | CI/CD environment — minimal, headless, non-interactive. |

## Merge Order

Profiles are merged in a specific order to allow progressive overrides:

```
base → platform → selected → machine → CLI override
```

1. **base**: Foundation profile (always applied)
2. **platform**: OS-specific overrides (macos.yaml, linux.yaml, termux.yaml)
3. **selected**: User-chosen profile (minimal, default, or full)
4. **machine**: Machine-specific overrides (from `~/.config/dotfiles/machine.yaml`)
5. **CLI override**: Flags passed on the command line take final precedence

## Profile Format

```yaml
name: default
extends: [base]
tools:
  shells: [zsh]
  terminals: [kitty]
  multiplexers: [tmux]
  editor: nvim
  fonts: true
  prompt: starship
```

## Extension Points

### Personal Profiles

Set `DOTFILES_PROFILE_REPO` to a private Git repository for machine-specific configurations:

```bash
DOTFILES_PROFILE_REPO=albersg/dotfiles-profile dotfiles install
```

The personal profile repo can contain:

- `conf.d/` — post-install hooks (shell scripts)
- `personal.zsh` — sourced in `.zshrc`
- `personal.lua` — sourced in Neovim config
- `machine.yaml` — machine-specific profile overrides

### Custom Tools

Add custom tools via the `tools` key in profiles. Tools must be installable via the system package manager.

## Creating a Custom Profile

1. Create a YAML file in `profiles/` or `~/.config/dotfiles/profiles/`
2. Define the profile following the format above
3. Reference it: `dotfiles install --profile my-profile`

## CI Profile

The `ci.yaml` profile is used in CI/CD pipelines:

- Non-interactive mode
- Minimal tool installation
- No font installation
- No multiplexer auto-start
