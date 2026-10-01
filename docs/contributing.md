# Contributing to dotfiles

Guide for contributors and developers working on dotfiles.

## Table of Contents

- [Development Setup](#development-setup)
- [Project Structure](#project-structure)
- [AI Skills System](#ai-skills-system)
- [E2E Testing](#e2e-testing)
- [Continuous Integration](#continuous-integration)
- [Release Process](#release-process)

## Development Setup

### Requirements

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.21+ | Build the installer |
| Docker | Latest | Run E2E tests |
| Git | Latest | Version control |

### Build from Source

```bash
git clone https://github.com/albersg/dotfiles.git
cd dotfiles/installer
go build -o dotfiles ./cmd/dotfiles
./dotfiles
```

### Run Tests

```bash
cd installer
go test ./... -v
```

## Project Structure

```
dotfiles/
├── installer/                    # Go TUI installer
│   ├── cmd/dotfiles/  # Entry point
│   ├── internal/
│   │   ├── system/               # OS detection, command execution
│   │   └── tui/                  # Bubbletea screens, views, installer
│   └── e2e/                      # Docker-based E2E tests
├── skills/                       # Repository-specific AI skills
│   ├── setup.sh                  # Sync script for AI assistants
│   └── */SKILL.md                # Individual skills
├── dotfiles-nvim/                # Neovim configuration
├── dotfiles-fish/                # Fish shell config
├── dotfiles-zsh/                 # Zsh config
├── dotfiles-nushell/             # Nushell config
├── dotfiles-tmux/                # Tmux config
├── dotfiles-zellij/              # Zellij config
├── herdr/                        # Herdr config
├── docs/                         # Documentation
└── AGENTS.md                     # Single source of truth for AI skills
```

## AI Skills System

The repository uses a skills system to provide context to AI assistants (Claude, Gemini, Copilot, etc.).

### Single Source of Truth

`AGENTS.md` is the master file. All other AI instruction files are generated from it.

### Sync Skills

```bash
# Interactive menu
./skills/setup.sh

# Generate all formats (CLAUDE.md, GEMINI.md, etc.)
./skills/setup.sh --all

# Individual targets
./skills/setup.sh --claude      # CLAUDE.md
./skills/setup.sh --gemini      # GEMINI.md
./skills/setup.sh --copilot     # .github/copilot-instructions.md
./skills/setup.sh --codex       # CODEX.md
```

### Skill Types

| Type | Location | Purpose |
|------|----------|---------|
| Repository skills | `skills/` | For this codebase (bubbletea, trainer, etc.) |

### Creating a New Skill

1. Load the skill-creator skill: `mcp_skill("skill-creator")`
2. Create directory under `skills/`
3. Add `SKILL.md` following the template
4. Register in `AGENTS.md`
5. Run `./skills/setup.sh --all`

## E2E Testing

Docker-based tests verify the installer works across different environments.

### Quick Start

```bash
cd installer/e2e

# Interactive TUI menu
./docker-test.sh

# Run all E2E tests
./docker-test.sh e2e

# Run specific environment
./docker-test.sh e2e ubuntu
./docker-test.sh e2e debian
./docker-test.sh e2e fedora
./docker-test.sh e2e alpine
./docker-test.sh e2e termux

# Interactive shell for debugging
./docker-test.sh shell ubuntu
```

### Test Environments

Each image is built from `Dockerfile.<name>` and runs `e2e_test.sh`. What the script runs is decided
by two variables the Dockerfile sets: `RUN_FULL_E2E=1` turns on the real installation passes and the
verification steps, and `RUN_BACKUP_TESTS=1` turns on the backup suite. An image that sets neither
installs nothing and stops after the binary smoke tests and the dry-run planning tests.

| Environment | Shell | Package Manager | Tests |
|-------------|-------|-----------------|-------|
| Ubuntu | bash | apt + Homebrew | Full E2E + backup |
| Debian | dash | apt + Homebrew | Smoke + dry-run planning, installs nothing |
| Fedora | bash | dnf | Full E2E + backup |
| Arch | bash | pacman | Full E2E + backup |
| Alpine | ash | apk | Smoke + dry-run planning, installs nothing |
| Termux | sh | pkg (simulated) | Android/Termux suite, simulated pkg |

For scale, from run 36625550831: ubuntu 65 assertions, fedora 64, arch 65, debian 13, alpine 13,
termux 38. The three that install something differ only by platform, and the two that do not install
anything stop after the binary smoke tests and the dry-run planning matrix, which is why their jobs
finish in about a second of container time.

On a Debian-family host the installer routes its dependencies through Homebrew, and Ubuntu is one:
`internal/system/detect.go` classifies it as `OSDebian` from `/etc/debian_version`. That is why the
Ubuntu container installs Linuxbrew and 251 brew formulae across its four installation passes, and
why it takes about six minutes where Fedora and Arch, which use dnf and pacman, take about two.

GitHub Actions builds these images with Buildx and the GitHub Actions cache backend. Each image has
its own `e2e-<image>` cache scope; BuildKit matches layers from the base image, Dockerfile commands,
and their inputs. A cold cache still builds the complete image, and local runs keep using `docker
build` without requiring Buildx or Actions credentials. The workflow must set up Buildx before
calling `docker-test.sh` for the cache backend to work.

### Adding Tests

Edit `e2e_test.sh` or `e2e_test_termux.sh`:

```bash
test_my_feature() {
    log_test "My feature works"
    if [ -f "$HOME/.config/myfile" ]; then
        log_pass "Config file exists"
    else
        log_fail "Config file not found"
    fi
}
```

Tests must be POSIX-compliant (no bashisms).

## Continuous Integration

Two workflows run on a pull request and on every push to `main`: `ci.yml` (Go Validation, Shell
Validation, Branding Audit, Secret Scanning, Build on Linux and macOS) and `e2e-testing.yml` (Go
tests, the Linux installation matrix, Termux, macOS smoke). The pull request waits for whichever
finishes last, and that is always the Ubuntu job of the Linux matrix.

### Which images run where

The Linux matrix holds five images. A pull request runs four of them; a push to `main` and a
manual run execute all five.

| Image | Pull request | `main` | Why |
|-------|--------------|--------|-----|
| ubuntu | yes | yes | The only apt-plus-Homebrew installation, and the critical path |
| debian | no | yes | A second `OSDebian` run of the same code path; Ubuntu covers the family and installs for real |
| fedora | yes | yes | The only dnf execution |
| arch | yes | yes | The only pacman execution |
| alpine | yes | yes | The only apk detection, and the only non-bash shell left on a pull request |
| termux | yes | yes | The only Android/Termux detection (non-blocking) |

The rule the matrix follows: an image stays on a pull request unless every axis it covers is covered
by another image that runs there. Debian is the only one that fails that test. Removing any other
would leave a package manager or a shell with no execution on a pull request at all, which makes a
green matrix say less than it appears to.

Termux is `continue-on-error: true`: it reports, it does not block.

### Concurrency

Both workflows keep one run per ref and cancel a superseded run **on a pull request only**. On
`main` nothing is cancelled: a run there is the verdict on a commit that is already merged, so the
next run waits its turn rather than replacing a check that nobody will re-run.

The practical effect for a contributor: drafting a pull request with three pushes costs one matrix,
not three.

### Reading the timings

```bash
gh run list --limit 20 --json databaseId,workflowName,headBranch
gh run view <id> --json jobs \
  --jq '.jobs[] | "\(.conclusion) \(.name) \(( (.completedAt|fromdate) - (.startedAt|fromdate) ))s"'
```

## Release Process

### 1. Build Binaries

```bash
cd installer
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o dotfiles-installer-darwin-amd64 ./cmd/dotfiles
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o dotfiles-installer-darwin-arm64 ./cmd/dotfiles
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o dotfiles-installer-linux-amd64 ./cmd/dotfiles
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o dotfiles-installer-linux-arm64 ./cmd/dotfiles
```

### 2. Create Tag and Release

```bash
git tag v{VERSION}
git push origin v{VERSION}

gh release create v{VERSION} \
  installer/dotfiles-installer-darwin-amd64 \
  installer/dotfiles-installer-darwin-arm64 \
  installer/dotfiles-installer-linux-amd64 \
  installer/dotfiles-installer-linux-arm64 \
  --title "v{VERSION}" \
  --notes "## Changes
- Feature/fix description"
```

### 3. Update Homebrew Formula

```bash
# Get SHA256 for each binary
shasum -a 256 installer/dotfiles-installer-*

# Update homebrew-tap/Formula/dotfiles.rb with new version and hashes
# Commit to both repos
```

### Version Guidelines

| Change Type | Version Bump | Example |
|-------------|--------------|---------|
| New platform/major feature | Minor (x.Y.0) | v2.7.0 |
| Bug fixes, improvements | Patch (x.y.Z) | v2.6.2 |
| Breaking changes | Major (X.0.0) | v3.0.0 |

## Code Style

### Go

- Follow standard Go conventions
- Use `gofmt` for formatting
- Table-driven tests preferred
- Error wrapping with context

### Shell Scripts

- POSIX-compliant (no bashisms in tests)
- Use `shellcheck` for linting
- Quote all variables

### Documentation

- Use tables for structured data
- Include Table of Contents for long docs
- Code examples for every feature
