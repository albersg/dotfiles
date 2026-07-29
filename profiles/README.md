# Profiles

## Available Profiles

| Profile | File | Description |
|---------|------|-------------|
| Minimal | `minimal.yaml` | Shell + editor only. Fastest setup. |
| Default | `default.yaml` | Shell + editor + terminal + multiplexer. Recommended. |
| Full | `full.yaml` | Everything: all shells, terminals, multiplexers, trainer. |
| macOS | `macos.yaml` | macOS-specific overrides and tools. |
| Linux | `linux.yaml` | Linux-specific overrides and tools. |
| Termux | `termux.yaml` | Termux (Android) — limited tools, pkg-based. |
| CI | `ci.yaml` | CI/CD environment — minimal, headless, non-interactive. |

## Merge Order

Profiles are merged in order to allow progressive overrides:

```
base → platform → selected → machine → CLI override
```

1. **base**: Foundation (always applied)
2. **platform**: OS-specific (macos.yaml, linux.yaml, termux.yaml)
3. **selected**: User-chosen profile (minimal, default, or full)
4. **machine**: Machine-specific overrides from `~/.config/dotfiles/machine.yaml`
5. **CLI override**: Flags passed on the command line take final precedence

## Usage

```bash
# Use default profile
dotfiles install

# Select a profile
dotfiles install --profile full

# Select a profile with platform auto-detection
dotfiles install --profile default  # Platform is auto-detected
```

## Extension

See [docs/PROFILES.md](../docs/PROFILES.md) for creating custom profiles and extension points.
