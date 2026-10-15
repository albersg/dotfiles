# Changelog

All notable changes to the dotfiles downstream distribution will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Changed

- **The prompt symbol is a one-cell powerline triangle rather than the glyph that shipped before.** `starship.toml` drew `U+F15DE` (`nf-md-mustache`) in both the character block and the username format; both draw `U+E0B0` (`nf-pl-left_hard_divider`) now. A prompt symbol is not a matter of taste alone: it has to occupy exactly one cell, or every line the prompt sits on shifts by a fraction of one. Measured against the families this repository installs -- MesloLGS and IosevkaTerm, on the Windows side and on the WSL side -- `U+E0B0`'s advance is exactly **1.000 of a cell**, and in the same face it equals `U+F15DE`'s, so the prompt keeps the width it had and nothing below it moves. Every one of those faces draws it. That measurement is the verification rather than a test, because the fonts are put in place by the installer and are not shipped in this repository: a test here cannot read a `cmap` the repository does not hold.

### Fixed

- **Fish lists files the way zsh does, icons included.** The two shells are meant to be the same shell with the same tools, and they were not. `.zshrc` aliases `ls` to `eza --icons --group-directories-first` with `ll`, `la`, `lt` and `tree` beside it; `config.fish` aliased `ls` to `gls --color=auto` and added nothing. A fish user therefore got a listing with no icon anywhere, which cannot be told apart from a Nerd Font that was installed wrong -- and it was reported as that. `eza` was already on the fish `PATH` on every host this installer supports, so fish had the tool and never called it. The listing block now declares the same five aliases with the same flags, which a guard compares against `.zshrc` so that a flag changed in one shell and not the other fails a test instead of shipping one flag at a time, and it exports `EZA_ICON_SPACING` so an icon is not jammed against the name it labels. Every alias is declared inside a `command -q eza` guard, because an alias whose tool the host has not got fails on every call and the Homebrew step that puts `eza` in place is allowed to fail. Without `eza`, macOS keeps the `ls --color=auto` it already had and a host with Homebrew's coreutils keeps `gls --color=auto` -- itself guarded now, for the same reason: `gls` is a Homebrew-only name, so that alias failed on every call on a Debian host without Homebrew.
- **The pinned Nerd Fonts archive predates `dev-yaml`, so `.yaml` files were blank on Linux.** The installer downloads a pinned Nerd Fonts release for Linux, WSL and Termux, and it pinned **v3.3.0** while the macOS cask installs **v3.5.1** -- two releases of the same tree, when the comment beside the URL already said they were meant to be one. `eza` draws a `.yaml` file with Nerd Fonts' `dev-yaml` (`U+E8EB`), and that codepoint is **absent from v3.3.0's own `glyphnames.json`** and present in v3.5.1's, so every `.yaml` in a listing came out as a blank box on Linux, on WSL and on Termux while macOS, which had the later archive all along, drew it correctly. A machine that only ever runs the macOS route cannot see this at all. The pin moves to **v3.5.1**, and the two facts the font step rests on were re-measured at the new tag rather than assumed to carry over: the faces still declare `MesloLGS Nerd Font`, which is both the family `fc-list` is asked about and the one the post-extraction verification reads back, so the presence check and the verification keep working; and the archive's declared size moves from 108,281,715 to **116,387,026 bytes**. The version is a floor rather than a preference -- `U+F086F` and `U+F15DE` are present at both tags and `U+E8EB` only at the later one -- and that is written beside the URL now, because lowering it restores the blank box silently.

## [v0.7.10] — 2026-10-14

### Added

- **`dotfiles --verify` reports whether the machine is still as the last install left it.** Every run records what it did to each configuration path it touched -- the path, the action it took, and the backup it made when there was one -- and nothing read that record. The command reports one line per entry and exits **0** when the machine matches the record, **1** when something it names is gone, and **2** when the record could not be read. It names its own limit **on every line**: the record holds no digest, so it establishes that a path is there, what the run did to it, and whether the backup still exists -- and says it cannot tell whether a file's contents are still the ones the install left. The distinction between absent and present-but-unverifiable is the point of the report, so it is per line rather than in a footnote. A machine with **no record is not a failure**: it says that nothing was installed here, which is what a verification run before the first install should say. The manual gains a section saying the same.

## [v0.7.9] — 2026-10-13

### Fixed

- **The font screen says what each option does, and what it found on this machine.** The row that answered "No, I already have it" was a **reason** where an **action** belongs: it did not say whether choosing it skips the font, fails the step, or continues. The rows are verbs now -- install, or skip -- and the screen states the measurement the installer already makes before downloading anything, so the choice rests on what is on the machine rather than on what the user remembers. The measurement is the step's own check rather than a second one, it runs once when the screen is built rather than on every redraw, and on macOS, where the step installs through Homebrew's cask and checks nothing, the screen says nothing about presence instead of inventing an answer.


### Fixed

- **Installing asks for the password once, and the installer stays on screen.** Four complaints from using it on Linux -- a password prompt for every step, having to press enter to continue, and the installer leaving the screen for the terminal while it worked -- had one cause: it handed the terminal to a shell script whenever a step needed `sudo`, because `sudo` wants a terminal to ask its question on. Its own comment said the consequence, *"This suspends the TUI and gives full terminal control to the process"*, and that single decision produced all of them. The credential is requested once now, before the first step that needs it, and kept warm while the install runs; every step script runs `sudo -n`, so a step without the credential fails visibly instead of prompting somewhere the user cannot see. With the credential cached a script needs no terminal, so the scripts run through the executor that already captured their output line by line and put it in the activity log -- the machinery every other step already used. No step hands the terminal over any more; the one remaining handover is the credential request itself, which is where `sudo` reads the password from. And the pauses are gone: eight occurrences of a sentence this program printed itself, one of its own constants, and a guard that now refuses to let one back into a step that no longer needs it.

## [v0.7.8] — 2026-10-13

### Fixed

- **An install no longer discards the theme the machine has applied.** Every step that writes a
  configuration file writes the file this repository ships, and those carry the **default** theme's
  generated block. The picker was the only caller of `applyDotfilesTheme`, and the record of which
  theme is applied was read only to be displayed, so a run silently reverted an applied theme for every
  file it rewrote while the record went on naming it, and nothing reported it: the picker's own count
  stayed true of a machine that no longer matched it.

  Measured on the machine this was found on: the picker wrote Nocturne into six files at `15:31:19`,
  an install ran at `16:59:42`, and afterwards `~/.zshrc`, `~/.p10k.zsh` and
  `~/.config/herdr/config.toml` carried the default theme's block and a `16:59:42` timestamp, while
  `~/.config/starship.toml` and `~/.config/nushell/config.nu` still carried Nocturne's and their
  `15:31:19` one. `theme.json` went on naming Nocturne over six files. The reset was per step rather
  than uniform, and the prompt — which reads `PALETTE_*` from the first two — rendered the default
  palette on a machine whose own interface reported Nocturne as applied: measured in a real
  interactive shell, `POWERLEVEL9K_OS_ICON_FOREGROUND=#7fb4ca`, `POWERLEVEL9K_FOREGROUND=#f3f6f9` and
  `POWERLEVEL9K_PROMPT_CHAR_OK_VIINS_FOREGROUND=#b7cc85`, every one of them `themes/dotfiles.toml`'s
  value rather than Nocturne's `#8686cb`, `#c0c0c0` and `#86cb86`. The same configuration looked right
  on another machine because the theme there had been applied **after** the last install; the order was
  the whole difference.

  A new **Restore the Applied Theme** step runs after every step that writes a configuration file and
  before `Cleanup`, which removes the temporary checkout the theme definitions are read from. It
  re-renders each artifact's block from the definition the record names, through the picker's own gates
  — the artifact list, the path each artifact answers with, the ownership proof, the Neovim
  reachability rule and the same renderer — so a file the picker would refuse is refused here for the
  same reason. Unlike the picker it never adopts a file that carries no marker of this project, and it
  leaves the theme record and its undo snapshot exactly as the picker wrote them. No record, a record
  that cannot be read, and a record naming a theme the checkout does not define all leave every file
  alone and none of them fails the run: the last is a normal race between the installed binary and the
  branch it reads, and failing there would report an install as failed after it had already installed
  everything.

## [v0.7.7] — 2026-10-13

### Fixed

- **The theme switch now writes the `bat` selection it always declared, and the prompt reload it names
  applies the theme.** Two silent gaps in the switch's reach, found by measuring one machine after
  Nocturne was applied and reported by neither the switch nor its screen.

  The switch declares fourteen artifacts and opens each iteration by asking where the artifact's file
  is, skipping it when the answer is empty — so an artifact it has no path for is declared, rendered
  and covered by tests while never being written, and a skip is not a failure. `bat` was that
  artifact: it shares `dotfiles-zsh/.zshrc` with the zsh artifact, further down the same file, and
  `themeInstalledPath` had no case for it. `BAT_THEME` therefore held the theme this repository ships
  for every theme on every machine, so `bat` — and `cat`, which the shipped `.zshrc` aliases to it —
  rendered the default palette while the rest of the machine rendered the applied one, with every
  generated `.tmTheme` present and nothing selecting any of them. The screen's own numbers showed it
  before any code was read: it reported `7 file(s) were themed and 2 file(s) were left exactly as
  they are` on a machine where fourteen artifacts minus the two files this repository does not own
  and the four absent tools leaves eight. Applying Nocturne to a copy of the machine's `.zshrc` left
  the bat block's tag and `BAT_THEME` on `dotfiles`, and counted one themed file where the same file
  holds two artifacts. A guard now fails unless every declared artifact resolves to a path.

  The second gap was in the instruction the screen prints. It names `p10k reload` for the prompt, and
  that re-sources `~/.p10k.zsh`; the generated palette block there **defaulted** each `PALETTE_*`
  (`${PALETTE_X:-"…"}`) while the shipped `.zshrc` sets the same names unconditionally and is
  sourced first. In a shell that was already open when the theme changed, `PALETTE_*` still held the
  previous theme, so the reloaded file kept exactly those values: measured in zsh with the previous
  theme's blue live in the shell, `POWERLEVEL9K_DIR_FOREGROUND` came out `#7fb4ca` where Nocturne's
  blue is `#8686cb`, and `POWERLEVEL9K_FOREGROUND` `#f3f6f9` where Nocturne's text is `#c0c0c0`. The
  one in-place action the installer offers for the prompt was the action that could not work. The
  block assigns now, which sets the file's values whether or not the names are defined — so the file
  still stands on its own without `.zshrc`, the property the fallbacks were written for, without the
  ability to lose to a value already in the environment. A guard asserts this against a real zsh: it
  sources the generated block with the previous theme's values set and fails unless the applied
  theme's values are what is left.

## [v0.7.6] — 2026-10-13

### Fixed

- **The Windows Terminal colour scheme is built from the theme the machine has applied, not from the default one.** `windowsTerminalThemeDefinition` loaded `dotfiles` unconditionally, so the scheme, the offer and the read-back all described the default palette no matter which theme the switch had applied. The machine this was measured on had `nocturne` applied -- recorded in the installer's own theme state, and written into `.zshrc`, `.p10k.zsh` and `starship.toml` -- while the `dotfiles` scheme in `settings.json` held `background #06080f`, `red #cb7c94`, `blue #7fb4ca`, to the digit the default theme's values. The palette now comes from the applied-theme record, with the default theme kept for a machine that has applied nothing, a record that cannot be read, and a record naming a theme the repository no longer defines -- the last of these falls back rather than failing, because an error in that function abandons the whole Windows side of the step, font face included. The scheme's name does not change: it is the one `dotfiles` scheme and a theme change repaints it.

## [v0.7.5] — 2026-10-13

### Fixed

- **The Windows per-user font registration no longer uninstalls the fonts it did not install, and the face it registers can now be resolved.** The registration program ran `New-Item -Path $key -Force` over `HKCU\Software\Microsoft\Windows NT\CurrentVersion\Fonts` before writing its four values, and on a key that already exists PowerShell's `New-Item -Force` recreates it and discards everything it held. That key is the machine's per-user font list, so every per-user font this installer did not put there lost its registration: the run this was measured on destroyed 36 of them, the whole CaskaydiaCove Nerd Font family, leaving the `.ttf` files in the store with nothing to resolve them by. The key is created only when `Test-Path` says it is missing. Separately, the value written was the file's base name, and Windows resolves a per-user face through the absolute path in the value -- a bare name is looked for in `%WINDIR%\Fonts`, where these faces are not. The family therefore never resolved, the verification that gates the face write refused correctly on every run, and Windows Terminal was never told the font, which is what a user sees as the installer doing nothing. The value is the absolute path now, built on the Windows side from `%LOCALAPPDATA%` with `Join-Path`.

## [v0.7.4] — 2026-10-13

### Fixed

- **Every terminal this installer configures now names a font that exists.** The installer wrote four shipped configurations naming `MesloLGS NF`, and no face in any artifact it fetches declares that family: the pinned Linux and Termux archive, the macOS cask and the Termux face all declare `MesloLGS Nerd Font`. On a fresh Linux or macOS install every terminal fell back to its default font -- silently, with no icons and the wrong letterforms. There is one family constant now, read from the face the archive ships, and the check, the verification, the Windows registration name, the four configurations, the interface label and both guards all derive from it.
- **The Linux font step no longer downloads 108 MB on every run, and no longer fails on a machine that has the font.** The presence check asked fontconfig for a family the archive does not declare, so it never resolved and the archive was refetched every time -- the defect #276 was opened to close, with a guard that could not fire. The verification after extraction asked the same wrong question, so the step reported an error on a machine where the font was installed correctly.
- **Termux installs the font the interface names.** It installed JetBrains Mono while the wizard row, the menu row, the requirement line and the summary all said Meslo, and the decision recorded in #265 is that the font is Meslo for every terminal. The URL it used had itself moved and was a 404, so that step would have failed the download outright.
- **The Windows account is asked about rather than guessed.** `defaultuser0` was not in the list of Windows system accounts, and it exists on machines; a single surviving directory was returned as the answer without being checked against anyone. The profile now comes from `%USERPROFILE%` through the Windows interop, gated on the directory existing, with the mount as a fallback that refuses rather than guesses.
- **The Windows Terminal write is reported from the file, not from the bytes that were sent.** A landing that came out partial could print a success over a configuration Windows Terminal would not read as sent. The write is read back, each setting is reported separately, and a partial landing is a shortfall that withholds the checkmark and names what did not land.


### Fixed

- **The dev channel serves the highest version, not the newest by date.** GitHub orders its releases list by creation date rather than by version, and the channel took the first pre-release in that list, so a pre-release cut after a higher one -- from a hotfix branch, or a re-cut of an older line -- was served over the newer version. It compares versions now, and it also skips a tag that is not a version at all, which it used to serve: a list holding a tag like `nightly` would have been offered as a release to install. Both were declared limits, written in the code, the README and this file, and all three said the channel took the publish order; they now say what the code does.


## [v0.7.3] — 2026-10-12

### Fixed

- **The pet moves within a second, and a key no longer stops its life.** Reported after v0.7.0 made it animate: it took a long time to start walking. Two causes, both measured. The first stroll waited two seconds of quiet before the wait even began, and a further zero to two seconds spread on top, so the first cell could take four seconds to appear. And a keypress reset the idle counter to zero, so a person reading the menu and pressing arrows or Enter held it at zero forever -- the pet never strolled for the user who reported it. The first stretch is half a second now and the gap between choices is zero to one, so the first cell lands around three quarters of a second.
- **The keypress reset was split rather than deleted.** What it was protecting -- the user's reading and navigation -- is not protected by it: the creature owns reserved rows, never reflows the layout and never touches the cursor, so its stroll cannot collide with either. What is kept is the sleep half: a key wakes a sleeping or yawning pet, restarts the sixty second sleep clock, and hops once so the wake is visible. The counter is clipped to the first tick past the roam threshold instead of cleared, so the sleep clock restarts while the pet keeps moving. One cancel survives because it is real: a resize that lands while a stroll is in progress. The initial size Bubble Tea sends at startup is excluded, or the first stroll would wait out the calm interval.

## [v0.7.2] — 2026-10-12

### Added

- **The installer installs the Herdr plugins.** It provisioned plugins for tmux, zsh, fish and Neovim and left Herdr's alone, and the two plugins this repository now declares -- `aigora.lantern` and `herdr.auto-title` -- are installed with `herdr plugin install <owner>/<repo> --yes` and verified by reading `herdr plugin list` back. No revision is pinned: the point is the latest, and the commit Herdr records is read and reported rather than chosen here. `--yes` is not optional, because without it the command opens a trust preview that nothing in a step can answer. A plugin that failed to install, or whose build failed, is a failed step rather than a success, and a plugin you added yourself is never touched.
- **The renderer offer reaches a screen.** Under WSL the installer could install the font, register it, verify it and write Windows Terminal's settings -- but only under `--non-interactive`. The interactive path, which is what a person using the TUI takes, printed the notice and never showed the offer. The installing screen now shows it, including the value that is there now when one is: a machine already configured by another tool has a font face set, and an offer that does not name it is how a user loses a setting they chose.

### Fixed

- **One unowned file no longer leaves every other file on the old palette.** The theme switch returned at the first artifact whose ownership it could not prove, after putting back everything it had already written, and alacritty is the first entry: on a machine carrying a legacy `alacritty.toml` the run stopped before kitty, ghostty, herdr, starship, zsh, p10k, nvim, bat, fish, tmux and nushell were reached, including the files that are marked and were perfectly themeable. The run now collects every file it cannot prove it owns in one pass, themes what it can, and reports a partial result naming each skipped file and its reason. A skipped file's bytes are untouched and the undo record holds only the files written. The refusal also stopped naming an action the interface does not offer.
- **The update row is never silent about what it knows.** A machine reported it exactly: on the stable channel `--check-update` exits 1 and the main menu shows nothing, because the row was added only when the update was installable from here. A newer release the installer could not install -- a binary Homebrew owns, or a target it could not resolve -- left no row at all, and a hidden row reads as "nothing new". The row now appears whenever the check has something to report and names the reason in the words the update screen already uses; pressing it opens that screen rather than attempting a swap. The write path is untouched: a binary a package manager owns is still never replaced, and the row reports rather than tries.
- **The font step asks whether the family is already installed, and verifies what landed.** It installed the font once per run, every run: Linux always downloaded the pinned 108 MB archive to overwrite identical bytes, and then reported "Font installed" whether a family registered or not. A broken extraction, a fontconfig cache that did not pick the directory up, or a user font directory the terminal cannot read was reported as success. The step now asks before it downloads and verifies after it installs, and Termux gets the same treatment.
- **The sound the managed config turns on gets its dependency.** The Herdr configuration this repository installs enables `[ui.sound]`, and Herdr ships no audio decoder: it shells out to an external mp3 player and fails silently when none resolves. The step now resolves a player before it reports success, provisions `mpg123` when none is present, and names the gap and withholds the success line when even that cannot be provided.

### Changed

- **The documentation and the code cannot disagree about the Herdr plugins.** A guard derives from source whether the installer invokes the plugin CLI and requires each document to carry the matching stance, so a provisioning step added without the documentation being updated fails it. It covers the Spanish README too: the other language drifting is a defect this repository has already had once.

## [v0.7.1] — 2026-10-11

### Fixed

- **The pet moves at a speed a person can see.** The report was exact: it did not look like it was moving on its own. The idle held one frame for two seconds, the creature did not start walking until six to fourteen seconds of quiet, and a play bout came every twenty-two, so ten seconds of watching showed one stroll. The idle now steps every quarter second, a stroll starts after two seconds, and a play bout comes every ten. Printed at successive ticks, the idle draws four distinct images in twelve ticks, the walk alternates its legs, and the play bout puts a ball at the creature's side and bats at it; before this, the same twelve ticks drew one image twice over.
- **The screens carry a frame.** The main menu was a plain list with a rule above it, and the theme reached the content and stopped at the border. Borders, titles, the selected row and the step indicators now take the palette the user chose, so the whole surface reads as one design. The truecolor path is captured separately for the first time: until now every golden was taken on the adaptive chrome, so the themed frames a user with a truecolor terminal actually sees had no golden at all.
- **Under WSL the installer configures the renderer instead of only naming it.** Issue #236 added the notice and stopped there: the code that would act on it was called only from a test, so nothing put a font where Windows could read it and nothing could write the settings Windows Terminal renders with. The step now copies the faces where Windows resolves them, registers the family per-user, verifies it in the font store, and only then writes the font face and the colour scheme into the reachable `settings.json`. A face named without that verification produces a silent fallback, which is the defect the notice was about.
- **The font is Meslo, and the family is resolved before it is named.** The installer installed IosevkaTerm and named it everywhere, and three of the four shipped terminal configurations named the Nerd Fonts v2 spelling of a family the v3 archive does not provide, so those terminals fell back silently. The font is now Meslo, whose naming does not follow IosevkaTerm's: the archive was read rather than assumed. The Windows face is resolved at install time from the font store, so a machine that carries its own patched family gets that one, and the public family is installed and registered only when nothing resolves.
- **The Homebrew formula parses again, so the tap installs.** It had not parsed since the 0.6.4 tap commit: the four `end` tokens closing the on_arm/on_intel pairs had been glued to the end of the `sha256` lines, and the script that rewrote those lines deleted them. `brew install` and `brew upgrade` failed before resolving anything for three releases. The formula is restored and two guards hold it: one asks Ruby to compile the file, which is what brew does before it resolves anything, and one rejects the shape that caused the defect, because the next edit that rewrites such a line will delete its terminator again.

### Changed

- **A guard that fails on correct input is worse than no guard.** The first attempt at the formula guard counted blocks with a regular expression and failed on the correct file, because `elsif` needs no `end` of its own. It asks the interpreter now. The same review found that one of the font guards could pass without reading any file at all, which is the defect it existed to prevent.

## [v0.7.0] — 2026-10-11

### Added

- **The companion plays, hops, and notices what the installer is doing.** It already wandered on its own; it barely moved while it did. Every state now has a real sequence instead of one glyph that shifts position -- a breath that moves the head a column and holds it, a three-step cheer, a play bout with paws batting and a tail swishing, and a hop with a take-off and a landing. Five states are earned by the installer rather than by a timer: watching a step that is running, charmed by a theme just chosen, curious about a published release this build does not have, mid-hop, and playing a game it starts on its own after a quiet stretch. None of them claims a step finished: the only path to the pleased face is the model's celebration counter, armed when a step really completes. The cadences are written down in the code, because a motion that changed every tick would repaint the creature's rows eight times a second over a screen nobody is touching.
- **The theme reaches the frame.** The chrome around the content was close to plain: the theme reached the content and stopped at the border. Borders, titles, the selected row and the step indicators now take the palette the user chose. And the truecolor path is captured separately for the first time -- until now every golden was taken on the adaptive chrome, so the themed frames a user with a truecolor terminal actually sees had no golden at all; three screens have one now.
- **The Neovim configuration syncs the clipboard over SSH.** Copy and paste worked in WSL and never over SSH, because nothing bridged to OSC 52. `vim.opt.clipboard` now lives in `options.lua`, which installs an OSC 52 clipboard when `SSH_CONNECTION` is set. A guard loads the file in headless Neovim with and without `SSH_CONNECTION` and asserts both directions; when nvim is missing it exits non-zero with a skip message rather than passing without checking anything.

### Fixed

- **Pre-releases are ordered correctly, and an unknown answer says why.** Version parsing dropped everything after `-`, so `v0.7.0-rc9` and `v0.7.0-rc10` parsed identically: a build on rc9 was told it was up to date with rc10, and a final `v0.7.0` was invisible over its own `v0.7.0-rcN`. Parsing is semver-aware now -- `rc9 < rc10`, a pre-release is behind its final, `dev.1 < dev.2 < dev.10`, build metadata ignored -- keeping the two rules that were already right: a source build is behind every release, and an unparseable candidate is never newer.
- **A tag that cannot be parsed no longer reads as "up to date".** It is unknown, with a reason, and `--self-update` refuses rather than installing from a tag it cannot parse. Failures are classified instead of stored verbatim: no network, rate limit, nothing published, or another status. A dev answer now names the pre-release it found, when it was published, and that it is a pre-release -- a dev answer that reads like a stable one is how a user installs something they did not choose.
- **The upstream mirror stays current, and says so when it cannot.** The tracker already compared a persisted HEAD and deduplicated by commit, so commits past the last tag were never invisible; what was wrong is that our mirror branch sat at a July commit while upstream's `main` moved two commits ahead of its last tag, and nothing said so. The mirror is advanced from the workflow now, and a failed fetch or push is reported as a comment instead of staying in a log.

### Changed

- **Declared limits are stated where they belong.** The dev channel does not verify that a pre-release came from `main`, and it takes the publish order rather than the highest version, so an older pre-release published later would be served. Both are written in the docs and in the code, and the README no longer claims ancestry the code does not check.

## [v0.6.5] — 2026-10-11

### Fixed

- **The install record says what the run did, not what it found.** It was built from the list of configs the run found on disk, so a config merely detected was recorded exactly like one the run overwrote, and a reader could not tell what had changed. Each entry now carries an action -- found, backed-up, replaced or created -- and an entry claiming a replacement carries the backup that proves it. An earlier version's record still loads (#239).
- **Under WSL, the installer no longer reports success where nothing renders.** The Nerd Font went to `~/.local/share/fonts` and the palette into the Linux terminals' configurations, but the program that draws the session is Windows Terminal, on the Windows side, which reads neither. The step now states where both landed and that Windows Terminal is the renderer before reporting anything as done, and then offers -- behind one explicit choice -- to write the font face and the colour scheme into the `settings.json` reachable from the mount. The offer keeps its promises in code: it takes a backup before writing and aborts if the backup fails, changes only the keys it owns while preserving comments and every other key, is idempotent, and refuses what it cannot edit safely instead of rewriting it. Outside WSL it touches nothing on the Windows side (#236).

### Changed

- **The release names the commit a tag points at, not the tag object.** `GITHUB_SHA` is the commit for a lightweight tag and the tag object for an annotated one, so a binary stamped from it could name a tag's id and be impossible to match to the commit the release built. The commit is now resolved once, in the job that has a checkout and produces the binaries, and read by every other job from an output it already needed; peeling falls back to the raw value when the tag object is absent, because naming a tag is worse than naming the commit but inventing one is worse still (#246).

## [v0.6.4] — 2026-10-10

### Fixed

- **nushell takes the theme now, and the coverage table can say so.** The shell's own
  `dotfiles-nushell/config.nu` shipped eighteen hand-written Kanagawa colours that appear in no
  `themes/*.toml` - the `$dark_theme` record the configuration selects with `color_config`, the
  `$env.LS_COLORS` table it paints `ls` output from, and two hexes in the `explore` block - and nushell
  was not one of the tools `themeTools` named. Choosing any theme but Kanagawa therefore repainted
  every other tool while `nu` kept the old palette, and the coverage row, which is read from the same
  list, said "nothing" was left out and was silent about the one shell it never painted (#240).
  Nushell is the thirteenth tool now and both of its colour regions are generated from the theme's own
  palette by the mapping recorded in `themes/README.md`, so no colour is invented; the two `explore`
  hexes take nushell's own ANSI names, which the terminal paints from whichever theme is applied. The
  switch rewrites only the bytes between the region's two anchors - a `config.nu` that carries the
  user's own PATH lines keeps them byte for byte - and records the original so **Undo** puts the file
  back exactly. `TestEveryOfferedThemePaintsEveryTool` now covers the shell (all seven themes paint it),
  `TestNushellIsPaintedByTheThemeSwitch` decodes every `LS_COLORS` triple and refuses any of the
  eighteen values, and `TestTheThemeRowCountsNushell` holds the row to the same truth it tells.

## [v0.6.3] — 2026-10-09

This is the release where the installer learns to apply the themes it offers. The library grows to seven
complete themes, every one of them painting all twelve tools the switch names, and applying one now reloads the
tools that show it and says what is left instead of reporting "applied" and stopping. The installer also learns
which release is published and installs it over itself, the companion stops following the cursor and wanders on
its own, and the ten interface defects an inventory found are fixed in one pass, together with the class that had
hidden three of them behind unreachable code.

### Added

- **Seven complete themes, every one painting every tool the switch names.** `dotfiles`, `Catppuccin Mocha`,
  `Catppuccin Latte`, `Rosé Pine`, `Everforest`, `Kanagawa` and `Nocturne` each paint the twelve tools - the four
  terminals (Alacritty, Kitty, WezTerm, Ghostty), Starship, the zsh line editor, the p10k prompt, Herdr, fish,
  bat, Neovim and tmux - and the picker's "leaves out" column reads `nada` for all of them (#167, #171, #183).
  Each theme has one definition in `themes/*.toml` and every artifact is generated from it, so a theme is offered
  only when it can be applied **and** shown (#159, #160). No colour is invented: a value is either what the
  repository already ships or a transcription of a published palette, and the roles a tool derives are named per
  tool in `themes/README.md` (#165, #171). `TestEveryOfferedThemePaintsEveryTool` reads the same coverage the menu
  reads and fails by naming the theme and the tool it left out; the failure that named `nocturne` and `Neovim` is
  the guard's own teeth (#171, #190).
- **A seventh theme, measured rather than transcribed.** `nocturne` follows a screenshot of the user's own
  terminal: the hues the screenshot carries were measured from its pixels, the ones it does not were derived from
  the measured set by one documented mapping, and `[syntax]` is dark-only because the reference has no light
  version. Its generated Neovim colorscheme and bat theme come from the repository's generator, and the definition
  carries its provenance (#190).
- **Applying a theme now reloads the tools that show it, and says what is left.** Writing the file is not
  applying the theme. The switch runs only the safe, idempotent reload commands - `bat cache --build` with
  `BAT_CONFIG_DIR`, `tmux source-file` only after `tmux list-sessions` succeeds, `kitty @ load-config` only when
  `KITTY_LISTEN_ON` is set, and `herdr server reload-config` only when `HERDR_ENV=1` - each bounded by a
  five-second timeout, and the result screen gives one row per tool: `✓` needs nothing more, `→` names the exact
  action a person still has to take (#194, #200). Herdr's reload is the command behind its own key, measured on
  herdr 0.9.3 (about 40 ms, exit 0, and `server_not_running` with exit 1 against a socket with no server), because
  a signal is not a key and the alternative was signalling the process the installer runs inside (#200).
- **A theme picker one level in.** Utilities offers `Change the dotfiles theme`, which opens the list of complete
  themes, each naming the tools it leaves out, followed by the undo row and the way back; the live preview moved
  with the list and is reached only from it (#164). The list windows around the cursor through the repository's
  existing `listWindow`, and the header names the visible range, so the frame - not the palette - no longer caps
  how many themes can be offered (#167).
- **The installer knows which release is published, and installs it over itself.** `--check-update` reads the
  latest release (exit 0 current, 1 newer, 2 unknown) and `--self-update` replaces the binary through the order
  the repository already used for OfficeCLI: stage beside the destination, verify the asset against the release's
  `SHA256SUMS`, make it executable, `rename`, keeping the replaced binary as `<path>.previous`. A failed check is
  recorded as unknown, never as current, and a binary Homebrew owns is refused with `brew upgrade dotfiles` named
  (#216). The main menu offers the update as a row once a newer release is installable, and the automatic check
  now runs on every real open rather than only behind the drawing gate; the row arrives after the first frame, so
  the cursor is held by the label it named (#222). The release workflow verifies its own chain twice - `verify` on
  the artifacts it built and `verify-published` on the assets downloaded back - and a manual `workflow_dispatch`
  (defaulting to `publish: false`) exercises the same job without publishing (#216).
- **Three more utilities, and the definitions they need outside a checkout.** WSL resources adjusts memory,
  processors and swap from the menu with the host's numbers and the recommendation on screen, sharing the
  installer's own calculation and writer rather than growing a second one (#172); the shell startup audit starts
  the login shell named by `$SHELL` the way a terminal does and reports the median of five bounded starts with
  zsh's own `zprof` attribution, where a start that does not finish is killed, counted and named as a timeout
  (#179); the terminal capability report answers what the terminal can do, with `unknown` as its own state
  carrying a reason and a manual check (#181). The clone step copies the theme definitions and the WSL template
  into `$XDG_DATA_HOME/dotfiles` (`~/.local/share/dotfiles`), which the resolvers read last, so the utilities are
  offered from any working directory and not only from inside a checkout (#192, #196).
- **Two degrees of adopting a file an older version installed.** A whole-file file is adopted as before, by the
  ownership marker or byte-identity to the shipped file; a file that carries our anchors but has drifted outside
  them is now adopted by region, rewriting only the bytes between the anchors, leaving every byte outside them
  untouched and recording the original so Undo restores it byte for byte. When neither degree holds, the refusal
  names the file, every probe that was made and the way forward (#173). Files that predate the marker can also be
  refreshed in place and reversibly: the picker names exactly which files it will touch and where each user-owned
  file will be preserved before it writes anything, and the cursor starts on Cancel (#169).
- **Goldens for the five utility screens that had none, and a guard that a golden renders and never installs.**
  The theme picker in three states, Utilities, WSL resources, the shell startup audit and the terminal capability
  report now have frozen frames, which is how the layout defects below were found by looking rather than by
  measuring (#212). The same change writes the rule as a guard: a golden renders a screen and never installs, by
  replacing the TUI's step executor with a spy on the screen where Enter starts a run. On a runner without
  privilege the old capture stopped at `/etc/wsl.conf`; anywhere with privilege it would have installed (#212).
- **The companion wanders on its own.** Its position and gaze are no longer governed by the cursor: after eight
  seconds without input it chooses between pausing, a short stroll, a look around and a small hop, waiting six to
  fourteen seconds between decisions, moving at most one cell every three ticks and returning to calm on input
  (#218). The random source is a private, injectable xorshift so the goldens stay repeatable, and the measured
  cost is about 6% more bytes over the same frames (#218).

### Fixed

- **The keys that lied are real, and the tests can now see a dead one.** Escape did nothing in Utilities, jumped
  past the picker in the theme refresh review, and in the restore confirmation jumped two levels to the main menu,
  all while the footer announced `[Esc] back`. They survived because `handleEscape` owns Escape, so the per-screen
  `esc` branches were unreachable code, and the tests pressed the key by calling those handlers directly. Five
  tests now drive `Update` with a real `tea.KeyMsg`, the path a keypress takes (#202).
- **Emoji are gone from the installer, and the rule is enforced where it was not.** The repository banned emoji
  because a terminal without an emoji font draws a box, and enforced it for the trainer alone while the
  installer's menus, titles and bodies kept theirs. Options and titles are words now, and where a glyph carried
  meaning it was replaced rather than deleted. The guard reads the `Screen` constants out of the source, renders
  the frames the shared terminal matrix already built, and fails by name - measured over 637 frames (#208).
- Escape on the main menu quit the application while the documentation called it "go back". It is a no-op on the
  root now, and the key, the documentation and the footer say the same thing; Backspace means exactly what Escape
  means, in one route through the real dispatch, with the trainer's answer line the written exception (#208).
- **Choosing a theme changes the screen once, not twice.** A press returned the switch command with the model
  untouched, so the frame after the press was the frame before it, and the result replaced the panel about a
  second later. The activity slot is now filled from the press and the result lands in the same slot, and the
  window around the list is a function of the frame alone, so the row under the cursor, the frame's height and
  the report's rows do not move between the press and the result (#204).
- **The interface stops stating facts it has not established, and stops moving the row under the cursor.**
  Utilities said "not switchable here" from an empty model while the definitions were still being read (it says
  it is reading now, and the main-menu panel says the theme has not been checked yet); writing the `.wslconfig`
  no longer replaces the table with "reading" while it re-reads, it keeps the table and marks the refresh; and a
  release row that arrives late is re-found by the label the cursor named, so a late `Restore from Backup` cannot
  steal an Enter aimed at Utilities (#206).
- **Neovim can always resolve the colorscheme the installer writes.** The switch wrote a name without leaving the
  generated file on the machine, so `:colorscheme nocturne` failed with `E185` because `~/.config/nvim/colors/`
  did not exist. It now installs the generated colorscheme before writing the line and names a plugin's
  colorscheme only when the machine's Neovim tree has one; otherwise it leaves the line out and says why (#210).
  The default theme was the same defect one level down - the committed block asked for the `kanagawa` plugin and
  the LazyVim statusline named it by hand - so the default is the only colorscheme this repository generates and
  installs, and the statusline leaves the theme to LazyVim (#214).
- **`BAT_THEME` named the wrong key, and five of six themes were affected.** `[bat] name` is the `.tmTheme`'s own
  `<key>name</key>`, but bat registers a custom theme under its **file** name, so only `dotfiles` reached the
  repository's file while the others painted bat's bundled theme or nothing at all. `batSelectionName` now exports
  the file's stem, measured with bat 0.26.1 against a cache built in a temporary `XDG_CACHE_HOME` (#171).
- The generated bat theme is well-formed XML: its header comment carried `bat cache --build`, and `--` cannot
  appear inside an XML comment, so both committed `.tmTheme` files were rejected by `xml.etree` even though bat
  rendered them. The guard parses every generated theme through `encoding/xml` and fails when it checked nothing
  (#175).
- The theme refresh review names the managed file the switch cannot bring forward instead of skipping it
  silently (#188).

- **A copied program keeps its executable bit, and every home file the shell step replaces is backed up.** The copy routine decided the mode itself, so `bash-env-json` -- executed by the nushell configuration the same install writes -- and `safe-update.sh` -- run as `argv[0]` by a Neovim command -- were installed un-runnable, and the failure surfaced as `EACCES` far from its cause. The mode now comes from the file, and an existing destination is `chmod`'ed explicitly, because `os.WriteFile` applies its mode only when it creates the file: leaving it alone would repair new installs and never the broken one (#241). `~/.zshenv` was replaced without being in `ConfigPaths()`, so the backup step and the record never saw it and a user's own file was destroyed with no copy and no mention; it is classified now, behind a guard that fails naming any home path the installer writes that the map does not classify (#238).

### Changed

- **Themes whose palettes were missing got them, and Kagawa was retired.** `everforest` and `kanagawa` are
  transcribed from their published palettes, `catppuccin-latte` is the light flavour, `rose-pine` the purple-pink
  one (and the darkest new base), and `kagawa` is removed because its only file was a byte-for-byte copy of
  Kanagawa and no published palette exists behind the name (#167).
- **The release procedure is one document, and the guard on it got stronger.** `docs/release-checklist.md` is
  deleted after its links were repointed at `docs/RELEASING.md`, and the guard now requires the runbook to name
  every job `release.yml` declares and rejects any release document that cites a workflow line number (#216).
- **The test loop's cost is now recorded, and the claim behind it was wrong.** `make check` runs `gofmt`, `go vet`
  and the changed packages with Go's cache on, so a rerun of `internal/tui` returns `(cached)`; what costs is host
  contention, measured at 10m20s of wall time against 72 seconds of CPU. `make preflight` adds the whole suite
  with `-count=1` by design, and CI is the complete verification (#166, #198).
- **The inspector screens got a hierarchy instead of a wall of prose.** Each terminal capability is now two
  units, the answer first and its source after; the shell audit shows the `zprof` attribution before the start
  list and stops printing the median twice; the WSL values come before the formula that derives them; and the
  theme picker's header says `Utilities`, like its siblings (#220).

### Removed

- `themes/kagawa.toml`, and the fish theme file that copied Kanagawa under that name (#167).
- `docs/release-checklist.md`, after its links were repointed at the one runbook (#216).
- The Utilities entry for the update, which is now a main-menu row; the update screen it opened is compiled but
  unreachable from the interface, and the Homebrew refusal no longer surfaces in it, though it still stops
  `--self-update` (#222).

### Notes

- **Declared limits, kept rather than hidden.** At 80x24 the theme picker's report cannot be read in full (the
  list keeps the rows and the report gets what is left) and the preview strip moves four rows when the result
  lands; both are the trade the picker was built with (#212). The companion's long pause is an approximation,
  because there is no "the user is reading" signal in Bubbletea (#218). Homebrew ownership is detected by path
  prefix and `/Cellar/`, so a Homebrew install whose symlink cannot be resolved is not detected (#216).
  `verify-published` has never run against a real release - the next release is its first real run - and only the
  linux/amd64 asset is executed by the workflow: the darwin and arm64 binaries are hashed and attested but never
  run (#216).

## [v0.5.0] — 2026-10-05

The release where the installer offers something beyond installing, and where the creature it draws survives
the screens it is drawn on. A utilities section joins the menu, and the small-terminal ladder gains a
volumetric rung so the cat is the 3D one on a laptop terminal as well as on a wide pane.

### Added

- **A utilities section**, reached from a visible `🧰 Utilities` row or the `u` key. Its first utility switches
  the desktop's light/dark theme through the desktop's own tool (GNOME `gsettings`, KDE
  `plasma-apply-colorscheme`, macOS `defaults`), never by editing that store itself. It records the value it
  replaced so the change can be undone, refuses to change a setting it could not restore, offers nothing on a
  host without a desktop, and is skipped by `--dry-run` like every other step (#153, #154).
- **A volumetric rung for small terminals.** The shaded creature now draws at 25-31 rows, where only the ASCII
  cat drew before; heights 20-24 keep the compact glyph deliberately, because a four-row volume loses the eye
  and a visible cat beats no creature at all (#152).
- The Utilities screen joins the frame and coverage guards, which now measure 648 screen x size cases and 39 of
  39 declared screens (#153).

### Fixed

- **The main menu's plan panel no longer crowds the creature out.** "What will happen" now names how many steps
  the plan has and the step the run is on, instead of spending a row per step, and the frame reserves the
  creature's block on the screens that draw the panel. At 100x25 the menu draws five rows where it drew none,
  and at 60x20 three where it drew none (#151).
- **The companion's rung is bounded to a quarter of the terminal height.** At 34 rows it took 35% of the screen;
  it now takes 23.5%, and 461 of 636 measured cases draw a creature against 423 before (#148).
- The wizard's plan panel no longer previews a plan when nothing has been selected (#148).
- A failed trainer progress save is now reported where the player lands, not on a screen they have left (#148).

### Changed

- **The release procedure has one home**: `docs/RELEASING.md`, derived from the workflow itself, plus
  `docs/release-checklist.md`, with a guard that derives the artifact names from `release.yml` and rejects the
  shapes the old documents used. `docs/RELEASES.md` was removed after its links were repointed (#149, #150).
- The local preflight added in this cycle runs the pull-request checks in about 35 seconds where CI needs about
  400 (#145).

## [v0.4.0] — 2026-10-04

This is the release where **the installer stops reporting successes it did not have, and fits the
terminal it is actually drawn into**. The wizard preselects the platform it detected, and on a host
whose platform it does not know it preselects nothing rather than macOS (#135, #143). Four places
that logged success whatever happened - a Ghostty download that failed, a tmux plugin install that
failed, a backup that failed to delete and ten trainer saves that failed - now report the failure
(#137), and `--dry-run` no longer runs `sudo`, Homebrew and `chsh` for real (#134). The frame guard
that had been missing measured 636 screen-and-size cases and caught the four screens that still
overflowed or clipped their frame (#139), and the screens that wrapped their text to a width they
never got now wrap to the column they are drawn in (#133). The cycle's verification work ships with
it: the last 47 sleeps in the suite became waits for the facts each test was about to assume
(#116, #118, #122), the restore tests stopped depending on the runner's `HOME` (#131), a pull request
stopped waiting eight to nine minutes for checks that were never gating anything (#82), and
`make preflight` runs the checks that can run locally in 34.8 s against roughly 400 s in CI (#145).

This cycle finishes the Vim trainer's buffer engine and its two new modules, and gives the installer
an interface that fits the terminal it claims and reads on a light one. The WSL configuration is also
now derived from the host that receives it, which removes the last artifact that carried one
machine's limits into another machine's VM. The installer's companion now stays still between
periodic idle events rather than repainting its own rows on every frame.

### Fixed

- **The companion's render cost and reserved block now have executable ceilings.** `BenchmarkCompanionVolumeFrame` fails above 10,000,000 ns/op, 450,000 B/op or 1,400 allocs/op, with headroom chosen from measured Linux/amd64 runs; `TestCompanionRenderedPosesStayInsideTheirReservedBlock` checks every walk pose, idle event, hop phase, gaze extreme and blink across both sprite modes and every rung. Raster-buffer optimization and performance guarantees for slower terminals remain out of scope.
- **The shaded companion's eyes now hold a sclera around a moving pupil, blink with a lid, and track gaze without dragging the body.** The full rung uses a 3×4 sclera and 1×2 slit at three long-axis positions. The slit keeps sclera columns on both sides at every position and has a full ring at centre; at either extreme it touches the corresponding lid. The smaller rung uses a 2×2 sclera and 1×1 pupil. `TestCompanionVolumeEyePupilAdjacencyByRung`, `TestCompanionEyeGazeIsolationAndBlinkPinsTheFaceContract` and `TestCompanionVolumeGazeTurnsTheHeadAndThePupils` pin the pupil margins, highlight, eyelid, head/ear motion and named-region bounds. The terminal-height ladder and reserved block are unchanged.
- **The shaded companion has a directional rim and contact shadows.** Lambert shading and ordered
  dithering remain, while a rim term now catches only the upper-left silhouette and two-step ambient
  occlusion darkens the neck, body and leg contact regions. `TestCompanionLightTerms` pins the encoder's
  five distinct tones and both spatial lighting terms. The encoder remains true-colour-only; 16-colour
  and colourless terminals keep the glyph companion.
- **The shaded companion has a cat's anatomy.** The volume now separates the skull, muzzle, neck, chest,
  haunch, triangular ears, four legs and tapered tail; the raster adds inner ears, paw pads, a dark nose,
  a two-pixel mouth and full-rung whiskers. Structural field and raster assertions guard the anatomy without
  changing the companion ladder or its reserved block.
- **Fish and Zsh startup customizations survive installer updates.** The shipped shell configs carry ownership markers, and the installer now preserves an existing unmarked `config.fish` or `.zshrc` as a dated, sourced drop-in (`~/.config/fish/dotfiles.d/` or `~/.zshrc.d/`) before replacing it. Both shells source personal drop-ins last so the user's settings override managed defaults; Fish uses `dotfiles.d` because native `conf.d` loads before `config.fish`. The install log names the preserved file; managed configs update without creating another copy. New personal Fish and Zsh startup changes belong in those extension directories.
- **The companion is the same size everywhere, and it no longer changes size as you move around it.** Its
  rung was chosen from the rows a screen happened to leave over, so the Vim Trainer kept a smaller creature
  than the menu while the menu's creature grew and shrank with its own content. The rung now comes from the
  terminal height alone, the trainer reserves it so it shows the same animal as the menu, and a screen that
  cannot hold it draws no creature rather than a smaller one.

- **The creature's colours stay inside the creature.** The shaded sprite set a tone per cell and never
  retired it, so the colour it left active painted every cell after it: a space is drawn with whatever
  colour is still set, half of the sprite's cells set a background, and a terminal's escape state outlives
  the line. The visible result was a solid bar of amber and one of light blue across the bottom of the
  screen, with the creature's head sitting inside a painted band. The pinned snapshot for the sprite
  contained not a single reset, so nothing had retired a style in the whole creature - and no test could
  see it, because a snapshot compares the escapes it is given and the check that verified the ink
  stripped them. There is now a guard that can see the class instead of the instance: **no rendered line
  may end with a style still active**, over every screen at five sizes, and it fails when the leak is
  reintroduced.
- **The companion is quiet until you move something.** It used to stroll along its row on its own and
  to halve the remaining distance to every target, so a screen nobody was touching repainted two of
  its rows on every frame — including the row the eyes settle on, a frame after the target moves —
  and the creature flickered. It now walks only while it has somewhere to go: moving the cursor points
  it at a row and it walks there at a fixed one cell a frame, slowing to a step every other frame over
  the last three cells, and stops when it arrives. One menu row is three cells of walking and no more,
  so one arrow key is a short stroll rather than a dash across a 227-column stage, and the gaze is
  settled when the target changes rather than on the next tick. Between idle events the view string
  is byte-identical, so the renderer writes nothing; while walking it repaints only the creature's own
  lines. The creature's body is also drawn in the theme's muted tones now, with
  the one bright tone spent on the eyes, so the decoration is no longer the loudest thing on a screen
  whose words should be.
- **The shaded companion walks, breathes, twitches, flicks and hops on model ticks.** Its four one-frame
  poses land front-left, back-right, front-right, back-left; the head leads by one pixel, the body rises
  at passing and the tail follows by two poses. Breath, ear twitch, tail-tip flick and blink recur at 4,
  20, 15 and 10 seconds, with no redraw between events. Clicks get anticipation, one terminal row of travel and
  landing squash inside the fixed block. `TestCompanionWalkPoseSequencePinsContactsAndLag`,
  `TestCompanionIdleEventsHaveIndependentPeriods`, `TestCompanionClickHopsAndCelebrates` and
  `TestCompanionCostHasTwoRegimes` pin the behavior and the zero-byte-between-events statement.
- **The installer's frames no longer tear while they animate.** bubbletea builds each frame in one
  write, but the terminal paints those bytes as they arrive, so a redraw that rewrites several lines
  was visible half-updated. The installer now brackets every frame in the terminal's
  synchronized-output mode (DECSET 2026, `\x1b[?2026h` / `\x1b[?2026l`), which makes the terminal
  hold the frame and repaint it once, installed through bubbletea's own `tea.WithOutput` and with no
  renderer of its own. A terminal that does not implement the mode ignores the sequences, so nothing
  is detected and nothing is at risk; the sequences are emitted only when stdout is a terminal, so a
  piped or redirected run keeps its bytes, and `DOTFILES_SYNC=0` turns the mode off for a multiplexer
  that mishandles it.
- **Four places that logged success whatever happened now report the failure.** The Ghostty step ran
  `curl … | /bin/bash`, which expands a failed download to the empty string, runs nothing and exits
  zero - the exact shape the Homebrew step's own comment had already named - and then logged "Ghostty
  configured"; both Ghostty paths now download to a file, check `curl` and run that file. The tmux
  plugin install discarded its result and logged "✓ Tmux configured" unconditionally, and now
  withholds the success line on failure (it still returns success and warns, because a pre-existing
  test outside that change asserts the step does not fail on a missing plugin seed - a decision,
  stated rather than hidden). A backup that failed to delete advanced as if it had been removed, and
  now reaches `ScreenError` like the restore branch beside it. Ten trainer save sites ignored the
  error while the profile reset beside them checked it, so a read-only `HOME` lost the player's
  progress without a word; all ten go through one `saveTrainerStats` helper that reports on the
  trainer's message line, with the warning ordered to win. Each site is guarded by its own test, and
  the Ghostty and tmux tests are `HOME`-sandboxed because a red run of them wrote
  `~/.config/ghostty/config` on the development machine. (#137)
- **`--dry-run` no longer performs the interactive steps for real.** The guard lived inside
  `executeStep`, and the TUI routes the interactive steps around it, so a flag documented as
  installing nothing ran `sudo` package installs, a real Homebrew install and a real `chsh` for the
  `deps`, `homebrew`, `terminal`, `wslconfig` and `setshell` steps. The interactive path now returns
  the same skipped result `executeStep` returns, with a `DRY RUN: skipping step "…"` log line, so the
  walkthrough still runs and changes nothing. The old test called `executeStep` directly and the E2E
  dry-run matrix is `--non-interactive`, which is why neither saw it; the new test drives the
  interactive dispatch. (#134)
- **The wizard preselects the platform it detected, and never falls through to macOS.** The
  preselection compared the detected platform with the generic `system.OSLinux` constant and used an
  `else` for everything else, so Debian, Ubuntu, Arch, Fedora, Termux and WSL - each with its own
  constant - all landed on the macOS row, under a title that said `Detected: Debian/Ubuntu (WSL)`.
  `osOptionIndex` now maps every platform explicitly with no `default`, and a class-level guard
  derives the platform constants from the `system` package so adding one forces a decision instead of
  a default. (#135) A second round found the same default one level up: the caller assigned the
  cursor to option 0 - macOS - when the mapping reported an unknown platform. An unrecognised
  platform and an unrecognised shell now open with nothing highlighted, say so (`Platform not detected
  - select your operating system`) and refuse `Enter` until the reader chooses, because a placeholder
  that reads as a selection is the shape this defect was born from. (#143)
- **The terminal step copies its configuration from the checkout instead of a `cwd`-relative
  `dotfiles/`.** There is no `dotfiles/` directory in this repository and the clone step checks out
  into a private temporary directory, so the copy source resolved only by accident, if at all, and
  the interactive script repeated the same hard-coded path under `set -e`. Both paths now resolve
  each asset through `m.repoDir()`. (#132)
- **The selection screens wrap to the width they are drawn at.** Their descriptions were wrapped to
  the whole terminal while the two-column composer truncates every left-hand line to its column, so at
  124x24 a 97-cell note was wrapped at 120 columns and cut at 56: the tail of every long line was
  silently discarded, in the screens whose own comment claimed the clipping had been fixed. The guard
  knew one of the two ways a cut appears - a suffix marker - and the panel's own wrap marker made a
  mid-line cut invisible, so it now walks each line's display columns and fails when the marker falls
  inside the body's own range. (#133)
- **Every screen is now measured against every terminal it ships to.** The frame guards iterated a
  hand-written list of states, so four defects shared one cause and nothing was checking it:
  `TestEveryScreenFitsEveryTerminalSize` renders 53 screens at 12 sizes - 636 cases, and the test pins
  that count - and asserts each frame's rows and its widest visible line against the terminal, counting
  a wide rune as two cells. It caught the welcome screen dropping its bottom rule and help footer off a
  44-row terminal (lipgloss `Style.Width` word-wraps, so centring doubled the wordmark once the left
  column narrowed), the trainer header letting its title be cut to the frame, the LazyVim topic
  reserving one footer row where the footer packs two, and the `keymaps` description widening its block
  until the terminal clipped it. The trainer's one-row companion rung at the 80x24 floor stays a
  documented exception, now stated where the ladder is stated and pinned by its own test. (#139)
- **The installer's titles say what happened.** Routing three screens through the one place titles come
  from was right, and the values they inherited were vaguer than the text they replaced: a failed
  install said `Error` where it had said `Installation Failed`, and the installing screen said
  `Installing...` where it had said `Installing dotfiles`. They now read `Installation failed`,
  `Installing dotfiles` and `Installation complete`. (#44)
- **The Vim trainer stops accepting wrong answers, miscounting practice and losing progress.** An
  answer the simulator cannot fully parse is rejected, and an operator or text-object answer must
  match the selection it produces, through one notion of fully consumed input; the practice counters
  have a single owner, so one submission is counted once; the space key is exempted by one predicate
  over the screens where it selects and stays ordinary input on the exercise screens; leaving a
  lesson, practice session or boss with `esc` persists stats and reports an abandoned boss instead of
  dropping them; control keys the simulator cannot parse are ignored rather than inserted as the
  literal text `ctrl+a`; and a boss answer is recorded through the canonical recorders, so a spent
  life and the attempt count have one owner and one window cannot cost two lives. (#34)
- **The trainer's hints say something the exercise's description does not, and the hint line is
  guarded.** A hint revealed with `Tab` used to repeat the mission — the mission read "Move to the
  start of 'userName' using w (word)" and the hint read "w moves to the start of the next word" — so
  asking for it cost a keypress and taught nothing, which is the defect a player reported. Every hint
  now adds the mechanism its mission leaves out: the count, flag or range the command takes, the part
  of it the mission does not name, how it compares with the command it is easiest to confuse it with,
  or what follows from it. One hundred and twenty-one hint lines were rewritten across the nine
  modules and the change-and-repeat boss fight, and no judging, solution set or lesson count changed.
  `TestShippedHintsAddWhatTheirMissionDoesNot` sweeps every shipped hint for one of those additions,
  and `hintEchoRewrites` pins the exact echoes that were withdrawn so a later edit cannot restore
  them. The hint line itself is now guarded: `trainerHintLabel` is the one place that builds
  "💡 Hint: …", and it returns nothing when the exercise carries no hint, so a hint that was
  legitimately dropped no longer renders a bare label — and the five content tests that required every
  lesson to have a hint were reshaped, because a hint is optional under the mission/hint rule and the
  guard over a hint that *is* present is the one that matters. The hint copy the Change & Repeat boss
  steps already carried was unreachable — the boss screen had no key that could show it — so the boss
  screen now reveals its step's hint on `Tab` on the same terms, and its legend advertises the key
  without costing a row.

### Changed

- **The trainer's lesson screen goes two columns on a wide terminal.** From the same 124-terminal-
  column floor the framed screens use, the code window moves to the left column and the mission, the
  answer line and the feedback move to the right, through the same `layoutFor` columns and the same
  `composeColumns` composition the installer's own screens use — no second idea of "wide". The code
  window keeps every row the stacked right column no longer occupies: at 160×50 the lesson's window
  budget is 44 rows against 11 at the 80×24 floor, and a fourteen-line exercise that scrolls below
  the floor shows all fourteen in full. The mission, the answer and the feedback are wrapped to their
  own column and lose no line; the menu and the boss screens keep their one-column bodies; and at
  exactly 80×24 the lesson screen is byte-for-byte what it was.
- **The companion became a creature.** The installer's ASCII cat was one row of seven characters; it is
  now a **shaded volume** where the terminal can shade, and a ladder picks the size from the rows the body
  did not need: the volume at twelve rows where it can spare twelve, the same volume at eight where it can
  spare eight, and then the glyph cat at five, its head at three, the one-row art the creature shipped
  with, and nothing below that. The panel summary's rows come off the spare rows first, because a fact
  still beats a decoration, and no screen loses the creature it had.

  The volume is **rendered, not drawn**: an implicit surface — a handful of metaballs for the head, the
  muzzle, the body, the four legs and the tail, cut at a threshold — shaded by a light from the upper left
  with normals taken from the field's gradient, quantised to the installer's own tones through a fixed
  ordered-dither matrix so the gradient reads smooth instead of banded, and grounded by a drop shadow.
  Animation **moves the metaballs** rather than swapping drawings, which is what makes the shape deform:
  a walk cycle with alternating legs and a one-pixel bob, a lean, a tail that counter-sways, and a head
  and pupils that turn toward what the creature is looking at. The glyph ladder under it needs no colour
  at all and is untouched — a half-block cell without colour draws as a plain block — so a terminal
  without true colour, a run with `DOTFILES_SPRITE=0` / `--no-sprite`, and a frame with fewer rows all
  still get a cat. The pupils sit at one of three columns and the eye row on one of two, and a composer
  moves them, so the tables stay one frame per state instead of thirty drawn frames; it blinks every ten
  seconds and yawns through the last two before it sleeps.

  **Its eyes follow the pointer.** They followed the selection; the pointer is what they look at when
  there is one, and the selection remains the fallback for a run or a terminal without one. Mouse motion
  is asked for only when the run can use it and it is a switch of its own — `--no-mouse` /
  `DOTFILES_MOUSE=0` — because mouse reporting costs the user the terminal's own drag-to-select: with the
  pointer off **no mouse mode is requested at all**, so ordinary selection works again, where a run that
  merely ignored the events would still have taken the drag. Termux defaults to no pointer, because a
  finger is not a hover and Termux turns a drag into a wheel report, and `DOTFILES_MOUSE=1` overrides
  that. The turn happens on the mouse message rather than on the next frame, the pupils rest inside a
  two-column dead zone so they cannot flicker, a pointer that has not moved a cell changes no byte, and
  every pointer event wakes a sleeping creature. A click earns the celebration a finished step gets plus a
  hop of one row, which costs a spare row and is skipped where there is none rather than take a fact's row.

  Its price is **measured, not estimated**, and it has two regimes (`TestCompanionCostHasTwoRegimes`): at
  rest, with no key and no pointer event, the view is byte-identical from tick to tick and the renderer
  writes **nothing**; while it walks, the volume's twelve rows all move, so twelve whole lines are
  repainted — 7488 bytes in the widest frame, about 58 KB/s at eight frames a second for the two and a
  half seconds a walk lasts. Rendering the volume is the expensive part of that:
  `BenchmarkCompanionVolumeFrame` measures about 1.6 ms a frame. `charmbracelet/x/mosaic` was evaluated for
  this and rejected on two measured reasons — it forces `x/ansi` ≥0.11.7, which breaks the `x/cellbuf`
  Bubble Tea pins and drags eight modules through the render path of every screen, and its
  luminance-threshold colour model collapses the shading boundaries the volume is made of — so the
  hand-written renderer is a decision, and the reasoning is written down beside it. The frame, the cell
  and the gaze all come from the model, so a snapshot can pin a frame instead of flaking on a clock.
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
  numbers right-aligned, so the names line up. The panel follows the selection instead of always
  describing the installation: on `Restore from Backup` it lists the backups with their dates and
  file counts, on `Keymaps Reference` it counts the bindings each tool ships, on the trainer it
  counts the curriculum, and on `Exit` it says in one line that nothing changes. The panel's name
  stays put so `Tab` does not wander — the option is named in the panel's first row instead — and
  the wizard's own questions preview the plan the highlighted OS or shell would lead to while still
  marking the step the run is on. The newest-backup row keeps its age ("9 days ago") without the
  render path reading a clock: the reference time is the model's own, so a snapshot still pins the
  bytes while a live run reads like a person would write it.
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
- **The suite waits for the facts it was about to assume instead of sleeping past them.** The last 47
  sleeps - 13 became waits for output to arrive, 22 waits for the screen the test opens, and 12 moves
  that needed no wait at all - are gone, and the trainer's own file, which held 36 of them, now waits
  on conditions through `teatest.WaitFor`. A first attempt replaced the library's 14 existing waiters
  with a home-made reader that read the stream differently and timed out over output the library would
  have matched; restoring them, with the shared helper delegating to the library, is kept in the branch
  history as the reason the fix is short. (#116, #118, #122)
- **The slowest guard renders its screens in parallel, and counts what it covers.** The constant-space
  guard rendered about eight hundred screens one after another on a single idle core. The models are
  still built sequentially, because the helpers call `t.Setenv` and that is legal only on the test's own
  goroutine; the renders - the expensive part, and pure per model - go through a bounded pool over an
  indexed slice so each result keeps the identity its failure message needs, and the assertions run in
  order over what came back. The test now asserts its own counts, 848 rendered screens and 848
  comparisons, so a change that quietly covers less fails there instead of saving time. (#120)
- **The restore tests waited on the runner's `HOME`, not on a race.** Seven attempts blamed timing: the
  startup backup scan replaces the model's seeded backups with a real scan of `$HOME` and has no guard,
  so on a runner whose home holds no `.dotfiles-backup-*` directories the list emptied, `Enter` stopped
  leaving the screen and the confirm screen became unreachable - while on a machine with three real
  backups the tests passed for the wrong reason. Each restore test now points `$HOME` at a `t.TempDir()`
  holding exactly the backups it declares. (#131)
- **A pull request no longer waits eight to nine minutes for checks that were not gating anything.**
  The E2E workflow cancelled unconditionally on a group shared by every push to `main`, so the next
  merge killed the previous commit's matrix: a merged commit whose only full installation was cancelled
  five seconds in, and never re-ran. Both workflows now cancel on pull requests only, because waiting
  behind the previous run costs minutes while abandoning a merged commit's check costs the check. The
  gates that only serialised - `linux-e2e` waiting on `go-tests`, `build` on `go-validate` and
  `branding-audit`, none of which consumes another's output - were removed for the same reason. (#82)
- **The E2E image cache was implemented, measured and deliberately not shipped.** Three runs of the same
  ubuntu job put the image build at 32.5 s of a roughly 400 s job, and the warm cache build took 42.2 s
  against the 32.5 s with no cache at all, so the cache was not being reused; even a hit is bounded by
  under ten per cent of the job, while the cold run paid 193 s to write it. The cache is out, with the
  figures and the untested `mode=max` hypothesis in `odd/tasks/ci-efficiency.md`. (#84)
- **The E2E shell check fails when the shell is missing.** `test_shell_functional` asked whether fish and
  zsh worked and had no branch for their absence: it recorded no verdict and the suite counted it as a
  pass. Both absences are failures now, and the coverage ledger names the new guard. (#116)
- **Two guards for classes the previous checks could not see.** The frame guards iterated a hand-written
  list of states, so a screen added to `model.go` was measured by nothing and the omission stayed
  invisible because the guards stayed green either way; the list now derives from the screens the source
  declares, and the count is asserted. (#94) And the installer's promise that nothing it means is
  carried by colour alone - Termux is supported, a 16-colour profile is supported, a piped run has no
  colour at all - is now checked by rendering five pairs of models with every escape sequence removed
  and requiring each pair to neither draw the same characters nor differ only in trailing padding. (#98)

### Added

- **The installer shows the machine it is changing, live.** A sampler reads the host about once a
  second — CPU busy as the delta between two readings, memory used and total, the load average, the
  free space on the target and the process count — as a command, never while rendering, and the
  readings live in a ring on the model. It reads `/proc` on Linux and WSL and `kern.cp_time`,
  `hw.memsize`, `vm_stat`, `vm.loadavg` and `ps` on macOS, and it degrades row by row: Termux and an
  unreadable host report nothing rather than a zero dressed as a measurement, and the panel says so
  in one line. The numbers are drawn by hand, in block sparklines and braille, so a 16-colour or
  no-colour terminal loses nothing — the shape is the information. They appear in a new **This
  machine, now** panel on the welcome screen and, most of all, on the installing screen, where the
  machine's pulse sits beside the progress bar and the run's own progress is charted over time; the
  pulse is drawn only from the rows the rail and the log do not need, so it never displaces them.
  The sampling is gated by the same switch as the animation: with it off no sample is taken and the
  panel says the sampling is off rather than freezing a chart and calling it live. The cost is
  bounded by a test: a reading repaints only the rows a live widget owns, and a screen that shows no
  reading renders the same bytes after one lands.
- **The progress bar has a travelling highlight, and a finished run celebrates.** During a long step
  one lighter cell (`▒`) walks the filled part of the bar on the frame tick, so the wait reads as
  alive; it is a glyph difference, not a colour one, and it only draws while a run is in flight. When
  the run finishes, a two-second burst of particles rises in the rows the body did not need and the
  companion is pleased — both model state advanced by the frame tick, both absent with animation off.
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
- **The main menu answers to `vim`, `:q` and `dd`.** `vim` opens the Vim Trainer, `:q` quits, and `dd`
  briefly removes the selected row as Vim would. (#114)
- **The Vim trainer scores real time, shows a countdown and times the boss fights.** The game state
  stamps when an exercise is presented and exposes the elapsed seconds through an injected clock, so
  the scorer no longer receives a hardcoded `10.0`; the screens show the time remaining before the hint
  and reveal it when the deadline passes, and expiry is a hint rather than a failure, because the
  exercise stays open and answerable. Boss steps run their own clock: an unanswered step costs exactly
  one life through the same recorder a wrong answer uses, shows the solution and re-arms, a won step
  grants the boss's bonus time to the step that follows, and a spent life is the single owner of the
  window re-arm, so one deadline cannot cost two. (#36)
- **The Vim trainer reports its progress and can clear it on request.** Every module entry shows the
  lessons completed against the total, its mastered count and its boss state, and the selected entry
  adds its commands and its most-often-missed exercises, with mastery moved to one predicate shared
  with weighted practice selection. `shift+R` arms a whole-profile reset and a second press clears it,
  while any other key cancels and `esc` leaves everything alone; the lowercase `r` still resets one
  module. (#38)
- **The installer provisions the Pi agent skills and the OfficeCLI binary.** `security-audit`,
  `archify` and `officecli` are installed under `~/.pi/agent/skills/`, each from an immutable commit
  and each checked against a hard-coded SHA-256 before anything is staged, so a mismatch or a failed
  transfer fails the step without touching the destination; extraction refuses archive entries that
  would escape the staging directory, and a destination that already exists is reported and left
  untouched, so a locally modified or newer skill survives an installer run. The pinned files come
  from the raw host rather than the rate-limited contents API, which settles a `403` that had turned
  the ubuntu E2E job red (#56). OfficeCLI is installed from one versioned release asset per platform,
  verified the same way, instead of the project's unpinned shell installer (commit `37e71c1`).
- **The installer's welcome emblem is a faceted gem, and it is the same mark in the Neovim
  configuration.** The emblem and the compact lockup a short terminal gets are redrawn as a cut stone
  in the installer's welcome screen and in the Neovim dashboard, so the two no longer disagree about
  what the mark is (commit `3f4961f`).
- **`make preflight` runs the checks that can run locally, in CI's order.** Seven steps - `gofmt`,
  `go vet`, a build with the `--help` smoke test, the whole Go suite, `shellcheck`, the branding audit
  with `ci.yml`'s own exclusion globs, and `gitleaks` over the commits the branch adds - each naming the
  CI job it mirrors, measured at 34.8 s locally against roughly 400 s in CI. A missing tool stops the
  run rather than skipping a check, the script builds into a temporary directory so a run leaves the
  checkout clean, and the footer names what it cannot run: the Docker E2E matrix, Termux, the E2E
  image-list job and the macOS toolchain. A green preflight is not 14 of 14, and it says so. (#145)

### Removed

- **The deprecated Neovim Gemini CLI integration is gone.** `jonroosevelt/gemini-cli.nvim` is removed
  and replaced by `mceazy2700/antigravity-cli.nvim`, which ships disabled like every other AI plugin,
  because the configuration enables one assistant at a time and the `<leader>a` key is shared by all of
  them. The plugin it replaced called `setup()` at load time on top of the config function the file
  declared, so it ran its own setup twice. (#79)

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
