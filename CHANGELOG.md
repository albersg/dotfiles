# Changelog

All notable changes to the dotfiles downstream distribution will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [v0.2.3] — 2026-09-21

Two defects on the interactive path, and the assertions that stop the sequence they belonged to.
This is the release where **the TUI installs on WSL**: v0.2.2 fixed the step loop, and the run still
died one step later.

### Fixed

- **A step the interactive dispatch table did not know.** Every TUI run on WSL died at the WSL step
  before that step did anything: `failed to get script for wslconfig: unknown interactive step:
  wslconfig`. The step is flagged `Interactive` because `/etc/wsl.conf` needs sudo, but the table
  that flag routes into had no case for it, so neither `.wslconfig` nor `/etc/wsl.conf` was ever
  written. It had carried that flag since it was introduced, so the TUI had never been able to run
  it.
- **A step that reported itself done having installed nothing.** The interactive terminal step
  returned an empty script for WezTerm on Debian, Ubuntu and WSL, on the stated grounds that
  "Debian uses brew, not interactive", while the non-interactive step installs it through
  Homebrew. Both paths now render from one shared helper, so they agree by construction rather
  than by a comment.

### Added

- **Three assertions where there used to be prose.** A step marked interactive with nowhere to
  dispatch to, a step scheduled for the non-interactive run that the executor does not know, and a
  scheduler and an executor that must agree about the set of steps: all three are now checked in
  both directions. Three defects in a row on this path each revealed the next because those
  agreements were comments rather than tests. That is the part meant to stop the sequence.

### Changed

- The Go WezTerm path now aborts when Fedora's `dnf copr enable` fails instead of ignoring it, and
  the Debian tap runs through the Homebrew prefix with its error checked. The interactive path
  already used `set -e`, so this aligns the two.

## [v0.2.2] — 2026-09-21

A patch release for one defect, and it is urgent rather than routine: **v0.2.1's TUI could not
complete an installation at all.**

### Fixed

- **The TUI carries what each step records to the next one.** `runNextStep` had a value receiver,
  so the pointer it handed to the step pointed into a throwaway copy: the clone step recorded its
  working directory and checkout path there while `Update` returned its own model, which never
  received them. The clone logged `✓ Repository cloned successfully` and then every step after it
  failed with `the repository has not been cloned in this run`; cleanup lost the same fields, so
  the checkout was left behind as well.

  The reported cause was right about the mechanism and incomplete about the fix, which is worth
  recording: changing the receiver to a pointer **does not repair it**, because `Update` returns its
  model by value and that copy is taken before the asynchronous `tea.Cmd` runs. The recorded state
  now travels back through the step-completion message and is applied where the model is kept.

### Added

- **A test that drives the TUI's own step loop**, asserting that what one step records is visible
  to the next. This is the second defect in a row on the TUI path, after the dependency script
  fixed in v0.2.1, and both survived for the same reason: the E2E suite only runs
  `--non-interactive`, so nothing had ever exercised the TUI's step sequence. The new test failed
  with the exact symptom above before the fix, which is what makes it evidence rather than
  decoration.

### Notes

- If you installed v0.2.1 and the TUI stopped after the clone, this release is the fix. `brew
  upgrade dotfiles` is enough.

## [v0.2.1] — 2026-09-21

Four defects reported against v0.2.0, all of them on WSL, plus the coverage that was missing for the
Arch package path.

### Fixed

- **A failed step no longer abandons the run.** A failure writing the root-owned `/etc/wsl.conf`
  returned from the run immediately, so `set default shell` and `cleanup` never executed, the
  default shell was never attempted, and the temporary checkout was left behind in `/tmp`. The
  steps now run as a group that collects failures, finishes what it can, cleans up, and still
  exits non-zero naming what failed.
- **The Windows profile is resolved without the interop that just failed.** The fallback in
  `windowsUserProfile` read the Windows user name by running `cmd.exe` again, so both routes died
  together and `.wslconfig` was skipped on exactly the machines the fallback existed for. It now
  reads the mount and never runs interop, and on an ambiguous machine it lists the candidates and
  names `DOTFILES_WSL_WINDOWS_HOME` instead of writing into a stranger's profile.
- **The installed shell stopped printing an error on every prompt.** The `.zshrc` runs
  `fnm use default` on every shell, but nothing created that alias, so the configuration was
  installed in a state it could not satisfy: `error: Requested version default is not currently
  installed` before every prompt and, with Herdr, in every new pane. The prompt now requires the
  alias and discards fnm's stderr, and the installer creates the alias **only for an fnm it
  installed itself** — an fnm that was already there belongs to its user and is left alone.
- **The TUI's dependency step now shares the tested path's decisions.** This is a correction to
  the previous release's claim as much as a code fix. `getDepsScript` was a second implementation
  of the dependency step: a raw `sudo apt-get` script that never consulted the Homebrew preference
  and never went through the availability filter. On WSL Debian it asked `apt` for `wslu`, which
  Debian does not carry, and `apt` aborts the whole transaction on one unknown name, so the TUI
  stopped at step 2. v0.2.0's entry says that class of failure was eliminated; it was eliminated on
  the path that had been tested, and this was the other one. Both paths now build from the same
  package set and the same dispatch, and what differs between them is only where the plan is sent —
  executed, or rendered into the script the TUI runs so `sudo` can prompt on a TTY.

### Changed

- The TUI dependency step no longer runs `pacman -Syu` on Arch or `dnf check-update` on Fedora: the
  tested non-interactive path never did, and the two now agree.

### Added

- **The Arch package path is covered by CI.** `docker-test.sh` had five images and none of them was
  Arch, so that path had been verified by hand exactly once. The new image uses a real `archlinux`
  base and a real `pacman` — no simulated package manager, because an image that fakes its package
  manager is how an E2E job stops proving anything.

### Notes

- `wslu` is now dropped for Debian and Ubuntu alike, because the installer cannot tell them apart;
  the run says so and points at Homebrew or the tool's own installer.

## [v0.2.0] — 2026-09-20

The installer now installs on the platforms it claims to support, and stops reporting success for
work it did not do.

### Added

- **Herdr in the Keymaps Reference.** Herdr was offered by the multiplexer selector, accepted by the
  CLI, documented on the Learn screen and installed — but it was the one tool with no reference
  entry. Every binding comes from `herdr --default-config` on herdr 0.9.1, cross-checked against the
  versioned keyboard page; the two entries the config does not print are marked as such. Because
  Herdr documents itself as mouse-native, the reference leads with a pointer category rather than
  presenting keyboard bindings alone.
- **The E2E suite exercises the terminal axis, the font option and `--wm=herdr`**, and walks the
  whole option space under `--dry-run`. Two of the five option axes had no automated coverage at
  all, which is why defects in them were invisible to CI.

### Changed

- **WSL keeps the distribution underneath it.** It was modelled as an operating system rather than
  a hosting environment, so detection stopped before identifying the distribution and every
  dispatch that reads the OS was blind on WSL. Package installation works there now, and the E2E
  suite can finally validate the installer on a WSL2 host.

### Fixed

- **The installer installs on WSL without Homebrew**, instead of failing every shell and
  window-manager install with `no package manager available for this platform`.
- **Package names the distribution does not carry no longer abort the whole transaction.** It was
  enough for one unknown name to take down `apt`, `pacman` or `dnf` and with it the shells that did
  exist — `zsh` and `fish` failed to install because `kubectx` or `starship` were in the same
  command. Every name in the Debian, Arch and Fedora columns was checked against a real
  distribution, and the ones that are absent are filtered and reported rather than silently
  skipped.
- **A selected shell or window manager the distribution cannot install fails with a message
  naming the routes**, instead of reporting success for a component that was never installed.
- **`--terminal=kitty` off macOS is refused** with the values that platform does support. It used
  to install nothing and log `Kitty already installed` on a machine where Kitty was not installed.
- **Installing a configuration now prunes what the repository no longer ships.** Directories were
  merged and never pruned, so a file a newer version dropped stayed on disk forever, and when it
  had been replaced by a differently named one both loaded at once.
- **The font step no longer leaves its 347 MB archive** in `~/.local/share/fonts`, a directory
  `fontconfig` scans. The manual installation guide taught the same leftover and no longer does.
- **The Homebrew step no longer reports an installation that did not happen.** The download ran
  inside a command substitution, so a failed download expanded to an empty string, the shell it was
  handed ran nothing and still exited 0, and the run went on to log `✓ Homebrew installed
  successfully`. Its own log had already printed `Warning: Homebrew binary not found`.
- **A failed image build fails the E2E harness** instead of being counted as a product test
  failure. `set -e` could not catch it: the failing call sat inside an `||` list, which disables
  `set -e` for the whole call.
- **The E2E asserts the window manager it asks for.** A missing one produced no verdict at all and
  the run was counted as a pass, which is how the Fedora job stayed green while Fedora had no route
  to install the multiplexer it was asked for.
- **`docs/ROLLBACK.md` documented rollback commands that do not exist**, including `dotfiles
  restore --latest` and a recipe built on a tag that was never created. It now documents the path
  that works.
- **The manual installation guide's font instructions** no longer download an archive into the
  font directory, and the TUI installer guide's download commands use the asset names that are
  actually published.

### Documentation

- The READMEs state what installing requires and what it does, including that everything replaced is
  copied to `~/.dotfiles-backup-<timestamp>/` first, and how to try it without consequences.
- The release procedure documents the Homebrew step that actually happens: it is manual, the formula
  lives in two places, and the tap must be public for `brew` to clone it.
- A `.gitleaks.toml` records eleven verified false positives from vendored oh-my-zsh examples and the
  Vim trainer's exercise text, so a full-history secret scan is clean.

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
