# Security Policy

## Reporting Vulnerabilities

If you discover a security vulnerability, please **do not** open a public issue.

Instead, report it privately:

1. **Email**: Create a security advisory via GitHub Security tab
2. **Response time**: You can expect an initial response within 72 hours
3. **Disclosure**: We follow coordinated disclosure — fixes are released before public disclosure

## Supported Versions

| Version | Supported |
|---------|-----------|
| Latest release | ✅ |
| Older releases | ❌ |

## Security Model

### What dotfiles does

- Installs development tools via system package managers (Homebrew, apt, pacman, etc.)
- Creates symlinks from the repository to your home directory
- Installs fonts and terminal configurations
- Backs up existing configurations before overwriting

### What dotfiles does NOT do

- Run with elevated privileges (no `sudo` required for normal operation)
- Install kernel modules or system services
- Modify system-wide configurations outside the user's home directory
- Transmit data over the network (except cloning the repository and installing packages)

### Sensitive Data

- **No secrets are stored in this repository**
- Personal profiles (machine-specific configs) should be in a private repository
- Secrets should be managed via 1Password CLI, Bitwarden, sops, or environment variables

## Dependency Security

- Go dependencies are pinned in `installer/go.sum`
- CI includes secret scanning on every push
- Shell scripts are validated with `shellcheck`
- Dependencies are audited periodically

## Reporting Upstream Vulnerabilities

For vulnerabilities in dotfiles (upstream), report to:
https://github.com/dotfiles-Programming/dotfiles/security
