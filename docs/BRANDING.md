# Branding Guide

## Overview

This documentation covers the branding conventions for the dotfiles downstream distribution.

## Brand Elements

### Name

- **Project**: `dotfiles` (lowercase, one word)
- **Repository**: `albersg/dotfiles`
- **Binary**: `dotfiles`

### Environment Variables

All installer environment variables use the `DOTFILES_` prefix. This section is the
inventory of every such name the installer reads or writes, grouped by who sets it.
`dotfiles --help` documents the subset that also has a flag, and it is not the whole
set: the installer reads overrides no flag reaches, and it writes names of its own.
The tables below carry them all, and `installer/cmd/dotfiles/help_test.go` derives the
names from the installer's own source and fails when one is missing here.

#### Switches a user sets

| Variable | Purpose |
|----------|---------|
| `DOTFILES_DRY_RUN` | Simulate installation without changes; `--dry-run` sets it |
| `DOTFILES_VERBOSE` | `=1` prints every command the installer runs |
| `DOTFILES_ANIM` | `=0` stops the animation and the tip rotation; the same as `--no-anim` |
| `DOTFILES_MOUSE` | `=0` stops reading the pointer; `=1` asks for it in Termux, where it is off by default |
| `DOTFILES_SPRITE` | `=0` draws the glyph cat instead of the shaded sprite; the same as `--no-sprite` |
| `DOTFILES_SYNC` | `=0` stops bracketing each frame in synchronized output |
| `DOTFILES_SKIP_DEPS` | `=1` skips the dependency step; the step's own failure message names this variable |
| `DOTFILES_SKIP_TOOLSET` | `=1` skips the toolset step, the ~60 Brewfile entries a container or CI run does not need |

#### Overrides for non-standard layouts

These exist for hosts the defaults do not describe. `docs/manual-installation.md`
tells a WSL user without interop to set the two host overrides.

| Variable | Purpose |
|----------|---------|
| `DOTFILES_DIR` | Points the installer at a repository checkout other than the one it was launched from; a new interface introduced with the dotfiles theme switch, so Utilities can read `themes/*.toml` before any clone |
| `DOTFILES_WSL_HOST_CPUS` | Sets the host's logical CPU count and skips the interop query; both host overrides must be present and valid |
| `DOTFILES_WSL_HOST_MEMORY_MB` | Sets the host's memory in MiB on the same terms as `DOTFILES_WSL_HOST_CPUS` |
| `DOTFILES_WSL_WINDOWS_HOME` | Points the Windows profile lookup at a Windows drive mounted somewhere other than `/mnt/c` |
| `DOTFILES_WSL_USERS_DIR` | Points the Windows users mount the profile lookup scans away from the default `/mnt/c/Users` |
| `DOTFILES_WSL_CONF_PATH` | Points the `wsl.conf` destination away from `/etc/wsl.conf` for a distribution that keeps the file elsewhere |

#### Names the installer reads or writes that are not user switches

| Variable | Purpose |
|----------|---------|
| `DOTFILES_TEST_MODE` | The binary exports it as `1` for `--test`; nothing reads it back, so setting it from outside has no effect |
| `DOTFILES_SHELL_STARTED` | The installer writes it into your shell rc; it stops a second shell from auto-starting the installer |
| `DOTFILES_REPO_REF` | Test harness hook: selects the revision the container end-to-end suite clones |
| `DOTFILES_BINFMT_DIR` | Test hook: points the interop check at a fixture directory instead of `/proc/sys/fs/binfmt_misc` |
| `DOTFILES_ALPINE_RELEASE` | Test hook: points the Alpine/musl probe at a fixture instead of `/etc/alpine-release` |

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
