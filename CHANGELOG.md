# Changelog

All notable changes to the dotfiles downstream distribution will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

This cycle finishes the Vim trainer's buffer engine and its two new modules, and gives the installer
an interface that fits the terminal it claims and reads on a light one. The WSL configuration is also
now derived from the host that receives it, which removes the last artifact that carried one
machine's limits into another machine's VM.

### Changed

- **The companion has a body, a face and a gaze.** The installer's ASCII creature was one row of
  seven characters; it is now a cat drawn at three heights, and a ladder picks between them from the
  rows the body did not need — five rows where it leaves six, three where it leaves four, the one-row
  art it shipped with where it leaves two, and nothing below that — so a smaller terminal gets a
  smaller creature instead of none, and no screen loses the creature it had. The panel summary's rows
  come off the spare rows first, because a fact still beats a decoration. Its pupils sit at one of
  three columns and its eye row on one of two, and those cells are the only characters a gaze frame
  changes, so the frame tables stay hand-written and reviewable instead of becoming thirty drawn
  frames; it blinks every five seconds and yawns through the last two seconds before it sleeps. All
  of it stays plain ASCII — no emoji, no block glyphs — so Termux and a 16-colour or no-colour
  terminal keep working, and the frame, the cell and the gaze all come from the model, so a snapshot
  can still pin a frame instead of flaking on the clock.
- **The installer has one palette that reads on a light or a dark terminal.** The theme was near-white
  text on whatever background the terminal happened to have, so on a light terminal the body text was
  effectively invisible. Every colour is now adaptive: it asks the terminal for its background and
  picks a light or dark variant, so a light terminal gets dark ink and darker accents while a dark
  terminal keeps the existing values. Only the colours change between the two; words and glyphs still
  carry every state, so a 16-colour or no-colour terminal loses no information.
- **Every screen writes its key hints the same way.** Navigation, the action and the way back now use
  one bracketed notation and one separator everywhere, including the trainer, whose legends were
  hand-typed and could drift from the keys they promised.
- **The installer now uses the terminal it is given instead of assuming the 80 columns it was
  designed at.** The body is capped at 160 columns and centred in the rest, the bar behind a selected
  row is a measure of at most 80 columns instead of a slab of colour across the whole terminal, and
  from 124 terminal columns the welcome screen and the main menu carry a second column: the machine
  the run is about to change, and the plan it will execute with the configs it will overwrite and the
  newest backup. Below that floor the panel is dropped and the screen renders exactly as it did, so
  nothing is lost. A body shorter than its frame no longer floats down the screen either: the blank
  rows above it are capped at six, where a nine-row menu in a 62-row terminal sat 26 rows below the
  header that named it. The plan panel numbers its steps with the marker in its own column and the
  numbers right-aligned, so the names line up.
- **Menu and status copy was rewritten to say what to do.** Detected-platform labels no longer repeat
  the line above them, the Alacritty build warning no longer sits inside the terminal's name where it
  read as part of it, the welcome screen's environment note is one sentence instead of three facts
  joined by pipes, and empty or blocked states name the next step instead of only the problem.
- **The Macros module's global-command lessons claim only what the judge can check.** The lessons that
  described the result of `:g/...` and `:v/...` commands named an effect the engine does not run; they
  now teach the keystrokes and say so, and a guard test catches that class of unkept promise in
  future content.
- **`.wslconfig` is rendered for the host it is installed on instead of shipped as one host's
  numbers.** The file was checked in holding what a single Windows machine had settled on — 6 GB of
  memory and 4 GB of swap against a 15.6 GB host, with 8 processors on a 12-thread one — and the
  manual recipe told the reader to copy it and adjust it by hand. The checkout now ships
  `dotfiles-wsl/.wslconfig.tmpl`, and the installer reads the Windows host through WSL interop and
  renders it, so the limits that reach a machine are computed from that machine: memory is half its
  physical memory, swap is a quarter of that memory, and processors are every logical CPU it reports.
  Both memory values are rounded down to 512 MB, Windows is always left at least 2 GiB, and a memory
  plan below 1 GiB is not written at all.
- **A host that cannot be read omits the limits rather than guessing them.** When interop is disabled
  or unavailable, `memory`, `processors` and `swap` are left out of the rendered file and WSL applies
  its own proportional defaults, which Windows computes from the real host. That is a supported
  configuration and not a broken one, and it is now the instruction the manual recipe leads with.
- **The install step and the interactive script render the same bytes.** Both routes go through one
  shared render helper, so the non-interactive step and the shell script a guided run executes can no
  longer disagree about what `.wslconfig` contains.

### Added

- **The installer has a companion.** A small ASCII creature walks the row immediately above the
  footer, follows the selection you move the cursor to, sleeps after twenty quiet seconds and wakes
  on the first key, and reacts to what is on screen: alert on the choices that throw something away,
  pleased for about a second when an installation step finishes, flinching while an error is showing and
  on the trainer's result screens when the last answer was wrong. The art is drawn in this repository
  in plain ASCII — no emoji and no third-party mascot — so Termux and a 16-colour terminal keep
  working and the state reads from the glyphs rather than from a colour. It draws only in a row the
  body did not need, so a screen whose body fills its frame shows no companion and loses nothing, and
  with animation off there is none at all (`DOTFILES_ANIM=0`, `--no-anim`, a piped stdout or
  `TERM=dumb`).
- **The Vim trainer can judge an answer by the buffer it leaves, not only by where it moves the
  cursor.** A mutable editing engine runs the answer and compares the buffer text, the cursor and
  the mode it produces against the optimal's result, so undo, put, register and indentation
  exercises are marked by their actual effect. When a rejected answer differs, the result screen
  shows the expected buffer next to the produced one. Commands the engine does not run yet are
  taught as keystroke exercises and say so, instead of promising a result nothing checks.
- **Two trainer modules: Editing & Undo and Registers & Indentation.** Editing & Undo covers
  insertion, escape, undo/redo and the line edits around them; Registers & Indentation covers the
  named registers, the yank-only register, put before and after the cursor, and the shift commands.
  Each has its own lessons, practice pool and boss, and both extend the unlock chain after Macros.
- **A progress bar and a per-step status rail on the installing screen.** The screen shows a bar
  sized to the terminal with a percentage beside it, and a rail of one row per step whose state is a
  glyph and a word, so the run's progress and each step's state are readable without colour.
- **`DOTFILES_WSL_HOST_CPUS` and `DOTFILES_WSL_HOST_MEMORY_MB`**, for a machine whose interop is
  unavailable: setting both to the host's logical CPU count and its memory in MiB replaces the
  Windows-side query. Both must be present and valid, so a half-configured override falls back to
  detection instead of planning a limit from one guessed number.
- **Three more panels, a `Tab` key and a narrow-terminal summary.** Every panel answers a question:
  **Your trainer** answers what you have gained (the lessons and mastery of each started module, the
  overall accuracy, the best streak, and the next boss with the practice gate it needs), **Did you
  know?** answers what you can learn right now (one shortcut at a time), and **Last install** answers
  when this machine was last installed. A screen that offers more than one panel starts its column
  with a tab row naming them and cycles them with `Tab`, which the footer advertises only where the
  key does something. Below the two-column floor the active panel collapses to a one-line summary
  placed in rows the body did not need, so no summary ever takes a row from a body.
- **The not-yet-known panels say nothing rather than zero.** A panel with no run behind it leaves the
  row out: with no trainer record the **Your trainer** panel says so in one line instead of drawing
  zeros that read as progress, and the **Last install** panel is offered only when a record exists,
  so no section ever says "never".
- **Tips come from content the repository already ships.** The **Did you know?** pool is built in a
  declared order with no randomness from the keymap reference data and the trainer's own lessons, so
  two runs on one machine show the same sequence; it advances one tip per ten seconds, and with
  animation off it stays on the first tip.
- **The installer records when it last ran.** A completed run writes a small record — the finish
  time, the build and the configuration paths it replaced — to
  `$XDG_STATE_HOME/dotfiles/last-install.json` (falling back to
  `~/.local/state/dotfiles/last-install.json`). The write is best effort: a state directory the
  machine will not let us write never turns an install that finished into one that failed.
- **The installing screen says how long the run has taken and what is left.** Under the progress bar
  it names the step the run is on, and it states the elapsed time with an estimate of the remainder
  scaled from this run's own clock and progress (`Elapsed 3m 05s · ~1m 42s left`). The estimate is
  stated only when it has a basis: with nothing complete it says `estimating…`, and with no run
  behind it it says `Estimating the time remaining…` rather than a number. The start timestamp and
  the latest tick live on the model, so the render never reads the clock and a snapshot cannot flake
  on the hour. The run keeps no file tally, so the screen shows no file counter.
- **The log box follows the terminal instead of a fixed three lines.** The installing screen's log
  and the error screen's log panel — which cut at five lines for no stated reason — now show the
  freshest lines that fit the frame and, when earlier lines do not, name how many were left out in
  one dim row (`… 9 earlier lines`). At 80x24 the installing box shows more than twice what it did;
  the step rail keeps a three-row floor, so the box can never push it or the footer off screen.
- **The welcome screen and the main menu greet by the time of day.** One added dim line
  (`Good morning`, `Good afternoon`, `Good evening`) greets the reader without replacing any existing
  copy. The greeting is a pure function of the time the model was created with, never of the clock
  read while drawing, so a snapshot pins it instead of flaking at the hour.

## [v0.3.0] — 2026-09-22

This is the release where **the installer stops ignoring the `Brewfile`**. The repository has
declared the machine's toolset in that file since it was adopted, and no step ever read it, so a
fresh install produced a machine carrying only what the shipped configuration needs at shell start.
Installing the dotfiles and then finding `btop` absent was the visible symptom: the `Brewfile` names
62 installable entries and the installer provisioned none of them.

### Added

- **A step that provisions the toolset the `Brewfile` declares.** It reads the file from the
  checkout the clone step already creates and hands a filtered copy to `brew bundle`, so the
  `Brewfile` stays the single source of truth: a tool added to it is provisioned without a change to
  the installer, and 62 package names are not duplicated into Go. `brew bundle` supports every
  directive the file uses.
- **`btop` in the zsh shell step**, with a guarded `alias top='btop'` in the zsh configuration. The
  alias is defined only where `btop` is present, so it stays inert on a host with neither Homebrew
  nor `btop`, which is the shape `trippy` already used.

### Changed

- **The `vscode` and `winget` sections are deliberately excluded** from what the step installs.
  Installing Windows desktop applications — Chrome, Office, Teams, a JDK — as a side effect of a
  dotfiles install is out of proportion, and the `vscode` extensions need the `code` CLI, which is
  absent on the servers this also runs on. The step logs how many lines it dropped from each section
  instead of hiding the omission, and both sections stay in the `Brewfile` for anyone who applies it
  by hand on a workstation.
- **`brew bundle` runs with `HOMEBREW_BUNDLE_NO_UPGRADE=1`.** `brew bundle install` upgrades by
  default, which an installer must not do: the step installs what is missing and leaves the rest.

### Notes

- **The step is best-effort.** An entry that cannot be installed, and a `Brewfile` that is missing
  or unreadable, are logged and the run continues. The one exception is a missing checkout, which
  reports a failure: a run with no checkout has already failed at the clone step, and a silent skip
  there would hide it.
- Termux and any host without a usable Homebrew skip the step, and `DOTFILES_SKIP_TOOLSET=1` skips
  it explicitly. The E2E script sets it, because `brew bundle` would otherwise install roughly sixty
  formulae in every container and blow the job runtime.

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
