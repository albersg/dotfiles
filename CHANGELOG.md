# Changelog

All notable changes to the dotfiles downstream distribution will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [v0.1.0] — 2026-09-19

First release of the downstream distribution.

### Added

- **Theme coherence**: one palette, declared once in `dotfiles-zsh/.zshrc` and shared by `LS_COLORS`,
  `EZA_COLORS`, `fzf`, `bat`, `zsh-autosuggestions`, `zsh-syntax-highlighting`, the powerlevel10k
  prompt and Herdr. The terminal emulators already defined it; nothing inside the terminal used it.
- A `bat` syntax theme generated from that palette (`dotfiles-bat/themes/dotfiles.tmTheme`), because
  no published theme can match a custom palette.
- An original emblem for the installer splash and the Neovim dashboard, replacing the upstream
  project's mark, with a compact variant for short terminals and a test that guards its geometry.
- `fzf-tab` and `zsh-completions`, declared in the `Brewfile` and in the installer's package list.
- A clock and the active Node version in the shell prompt. The Node version is read from the symlink
  `fnm` already put on `PATH` (0.02 ms per prompt); powerlevel10k's own segment shells out to
  `node --version` (6.07 ms).
- The build version on the welcome screen, so a bug report carries it without needing a flag.

### Changed

- The welcome screen is height-aware: the full lockup needs 34 rows, and anything shorter gets the
  emblem without the wordmark instead of being clipped from the top.
- `cat` is now a shell function that keeps syntax highlighting for plain invocations and hands any
  invocation carrying a flag to the real binary.
- A stale `~/.oh-my-zsh` is left untouched, and `PATH` no longer gains one entry per nested shell
  from `fnm`.

### Fixed

- `grep` is no longer aliased to ripgrep. In ripgrep `-r` means *replace*, so `grep -r pattern .` ran
  a replace command; `-E`, `--include` and `-A/-B` differ too.
- `bat` no longer renders with a theme whose colours the terminal does not define.
- `--version` printed `dotfiles vv0.1.0` for a release build, because release tags already carry the
  `v` prefix and the format added another. An uninjected build now reports `dev build`.
- The installer no longer aborts when its checkout predates an asset it wants to copy. It clones the
  repository at install time, so a newer binary can meet an older checkout; the case is now skipped
  with a log line, as `.gitconfig-personal` already did.
- `FZF_ALT_COMMAND` was never read by fzf. The variable is `FZF_ALT_C_COMMAND`, so Alt+C listed files
  where only directories were expected.
- The installer's Docker test runner no longer prints the upstream project's name as block art.

### Bootstrap

- Downstream distribution from [Gentleman.Dots v2.12.2](https://github.com/Gentleman-Programming/Gentleman.Dots/releases/tag/v2.12.2)
- Full rebranding: binary → `dotfiles`, env vars → `DOTFILES_*`, package dirs → `dotfiles-*`
- Upstream tracking via `upstream-main` mirror branch
- `.downstream/version.json` for sync tracking
- Documentation: UPSTREAM.md, DOWNSTREAM.md, 3 ADRs, legal notices, operational docs
- CI/CD pipeline: Go validation, shellcheck, branding audit, upstream-watch, release workflow
- Homebrew tap: `albersg/tap/dotfiles`
- Profile system: YAML-based profiles (minimal, default, full, platform-specific)
- Threat matrix RED tests for git automation safety
- License audit: docs/audits/LICENSE-AUDIT.md

### Preserved

- Original upstream LICENSE (MIT)
- OpenCode personal config (`stow/opencode/`)
- Go module path (`github.com/Gentleman-Programming/Gentleman.Dots/installer`) — deferred rename per ADR 0002
