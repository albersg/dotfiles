# The fish shell's parity with zsh

## Goal

Make fish the shell zsh already is: the same file listing, the same day-to-day
functions, and the same secrets and project hooks — each one installed by the
installer rather than typed by hand.

The gap is not taste, and it is not a plugin zsh has that fish lacks. It is that
`dotfiles-zsh/.zshrc` was built out and `dotfiles-fish/fish/config.fish` was not.

## Evidence, measured on this machine (read-only)

Machine: WSL2 Debian, Windows Terminal, herdr, dotfiles v0.7.10. `eza 0.23.5` is
installed at `/home/linuxbrew/.linuxbrew/bin/eza` and **is already on the fish
`PATH`** (`fish -c 'command -v eza'` prints that path), so fish has the tool and
never calls it.

| what | zsh | fish |
|------|-----|------|
| `ls` | `eza --icons --group-directories-first` | `gls --color=auto` |
| `ll`, `la`, `lt`, `tree` | all four defined | **none** |
| `EZA_ICON_SPACING` | `2` | **not set** |
| `LS_COLORS`, `EZA_COLORS` | from the generated theme block | **not set** |
| `cat` → `bat` for plain invocations | yes | **no** |
| `extract`, `compress`, `serve` | yes | **no** |
| sops/age secrets helpers | `env-edit`, `env-load`, `env-export` | **no** |
| venv activation on `cd` | `add-zsh-hook chpwd _dotfiles_activate_venv` | **no** |
| project-task announcement on `cd` | `add-zsh-hook chpwd _dotfiles_show_tasks` | **no** |

`fish -ic 'type ls'` on this machine prints the function the alias builds:
`gls --color=auto $argv`. That is the whole of fish's listing configuration.

### D1 — fish's `ls` prints no icons because it never calls `eza`

The user-visible half. `eza` is named by `macos.yaml`, `linux.yaml`, `Brewfile:45`
and the Arch and Fedora columns of `shellPlatformPackages`, and Debian gets it from
Homebrew, which this installer puts in place for Debian hosts
(`installer.go`, the `case "zsh"` comment). So `eza` is a tool every supported host
is expected to have, and fish simply does not use it.

### D2 — the fallback alias would break on a host without `gls`

`alias ls='gls --color=auto'` is the Linux branch and names `gls` unconditionally.
`gls` is GNU coreutils' prefixed binary and only Homebrew installs it under that
name; a Debian host whose only `ls` is the system one has no `gls`, and the alias
fails on every call. This is the same defect the zsh configuration already guards
against for `trip` and `btop` — an alias for a tool that may be absent is only
declared when the tool resolves.

### D3 — fish's listing has none of the palette zsh's listing has

`LS_COLORS` (which `eza` reads as its base layer), `EZA_COLORS` (metadata) and
`EZA_ICON_SPACING` are set for zsh inside and just below the generated theme block.
`renderFishConfig` emits `fish_color_*` and `fish_pager_color_*` only, so fish's
listing is `eza`'s own default sixteen colours, not the theme's. Stage 2, below.

### Where it happens

- `dotfiles-fish/fish/config.fish`, the `## alias` block — D1 and D2.
- `renderFishConfig` (`installer/internal/tui/installer.go:2894`) — D3. The zsh
  half is `renderZshTheme` (`:3446`) with `zshLsColorsBlock` (`:3513`) and
  `zshEzaColorsBlock` (`:3541`); those two tables are written as literal strings
  in the zsh renderer's own vocabulary, and fish cannot read them as they stand.

## What is already right, and must stay right

- **`eza` is installed by the toolset step, not by this front.** Nothing here adds
  a package: the front is that fish does not call a tool it already has.
- **The `dotfiles-theme: fish` block is generated and must stay generated.** The
  listing aliases belong outside it, next to the other hand-written aliases, or
  the theme switch's `AdoptStart`/`AdoptEnd` markers stop delimiting it.
- **A fish alias is not a zsh alias.** They are two different configurations that
  must agree on the *policy* — which tool lists files, with which flags — not on
  the text. The guard compares the flags, and nothing else.
- **`command -q eza` is the guard, not a version test.** The repository's rule for
  an alias whose tool may be missing is to ask whether it resolves.

## Tasks

Stage 1 — the listing (this change):
1. Give fish the five listing aliases, guarded on `eza`, with the flags zsh uses.
2. Keep a colour `ls` for a host where `eza` does not resolve, and guard `gls`.
3. Set `EZA_ICON_SPACING` for fish, so the icons have the space zsh gives them.
4. Guard the flags: a flag changed in one shell and not the other fails the guard.

Stage 2 — the listing's colours (its own change):
5. Emit `LS_COLORS`, `EZA_COLORS` and the icon spacing from one mapping both
   renderers read, instead of a zsh-shaped literal fish cannot use.

Stage 3 — the functions: `cat` → `bat`, `extract`, `compress`, `serve`.

Stage 4 — the hooks: sops/age secrets, the venv activation on `cd`, and the
project-task announcement.

## The proof

Stage 1. Every line below was run; nothing here is intended to be run.

| what | value |
|------|-------|
| the three guards against the unmodified `config.fish` | **all three fail** |
| `TestFishListingAliasesMatchTheOnesZshDefines` | fails with `read no \`eza\` alias from ../../../dotfiles-fish/fish/config.fish: fish lists files without eza, so its \`ls\` prints no icons while zsh's does` |
| `TestFishGuardsItsListingAliasesOnEza` | fails with `declares eza aliases without asking whether eza resolves, so a host that has not got it gets an alias that fails on every call` |
| `TestFishKeepsAColourLsWithoutEza` | fails with ``aliases `ls` to `gls --color=auto` without asking whether gls resolves, so a host without Homebrew's coreutils gets an alias that fails on every call`` |
| the three guards after the change | pass |
| mutation: fish's `tree` `--level=3` changed to `--level=2` | fails, naming `tree` and both flag sets verbatim |
| mutation: `--git` dropped from fish's `ll` | fails, naming `ll` and both flag sets verbatim |
| mutation: the Linux `gls` fallback deleted | `TestFishKeepsAColourLsWithoutEza` fails, naming `gls --color=auto` |
| mutation: `if command -q eza` weakened to `if true` | `TestFishGuardsItsListingAliasesOnEza` fails |
| the configuration restored after every mutation | byte-identical to the version under test, checked with `diff -q` |
| `fish -n` on the rewritten `config.fish` | syntax OK |
| the listing block sourced in a clean `fish --no-config` | all five aliases resolve to the eza command zsh's are, and `EZA_ICON_SPACING` is `2` |
| `gofmt -l ./internal ./cmd` | no output |
| `go vet ./...` | exit 0 |
| `go build ./...` | ok |
| `go test ./internal/tui/ -count=1`, this branch | 4 failures |
| the same command on a detached worktree of `origin/main` (`bdbdaa4`) | the same 4 failures |
| the two failure sets compared | **identical**: this change adds none |

### A guard whose input is a file outside its package needs `-count=1`

This cost one wrong measurement and is worth writing down. The first mutation run reported
`0` for every mutation **including the baseline**, which reads as "the guard has no teeth".
It had teeth; the run was reading Go's test cache. Go caches **successful** test results
keyed by the package's sources and the test flags, and `dotfiles-fish/fish/config.fish` is
neither: the guard reads it at run time from outside `installer/internal/tui`. So every
mutation replayed the cached pass.

The tell is the other direction: the *first* run, against the unmodified file, was genuinely
fresh and genuinely failed, because Go does not cache failures. Any run of this guard that
has to be trusted — a mutation study, a pre-merge check — passes `-count=1`.

### The package suite already fails, and the failures are not this change's

`go test ./internal/tui/` reports **four failures on this branch and the same four on a clean
`origin/main`**, compared by name:

```
TestStepInstallTerminalGhosttyReportsAFailedDownload
TestStepInstallTerminalGhosttyRunsTheDownloadItFetches
TestInteractiveGhosttyScriptFailsWhenTheDownloadFails
TestInteractiveGhosttyScriptDownloadsThenRuns
```

They are the Ghostty failures `font-windows-truth.md` and `theme-survives-install.md`
already record. The comparison was run as two suites in parallel, one against this tree and
one against a `git worktree add --detach /tmp/dotfiles-baseline origin/main`, so the only
difference between the runs was this change; the worktree was removed afterwards.

Two traps are worth naming, because both produced a wrong reading first:

- **`go test` output is not a failure list.** The run prints the captured stdout of tests
  that *pass*, and two of those read like disasters — a step summary carrying
  `Configure WSL ... ❌ FAILED ... permission denied`, and another carrying
  `zsh installation failed`. Both are assertions about what a *simulated* failure
  produces, printed by tests that pass. Only lines beginning `--- FAIL` are failures, which
  is why the comparison counts those rather than reading the tail.
- **A one-line `tail` destroys the comparison.** The first run of this suite captured only
  its last 40 lines, so the failure list could not be reconstructed without running the
  suite again. Any run whose output will be compared later is captured whole.

## Stage 2 — the listing's colours

The mandate is stage 1's own task 5, quoted verbatim: *"Emit `LS_COLORS`,
`EZA_COLORS` and the icon spacing from one mapping both renderers read, instead of
a zsh-shaped literal fish cannot use."* The icon spacing is already done — stage 1
wrote `set -gx EZA_ICON_SPACING 2` at `config.fish:115`, outside the generated
region. What is missing is the two tables.

### What was measured before writing anything

- **fish reads what zsh writes; only the spelling has to change.** A probe run
  through `fish --no-config` on this machine, declaring two palette variables and
  the two tables with them:

  ```fish
  set -g PALETTE_BLUE_SGR "38;2;1;2;3"
  set -gx LS_COLORS "rs=0:\
  di=$PALETTE_BLUE_SGR:\
  ex=$PALETTE_RED_SGR"
  ```

  printed `rs=0:di=38;2;1;2;3:ex=38;2;4;5;6` as **one** line (a newline count of
  `0`), and `env` carried the same string. So `$VAR` expands inside fish's double
  quotes, a backslash at the end of a line continues the value without keeping the
  newline, and `set -gx` exports it for `eza` to read out of the environment — the
  same three things zsh's spelling buys.
- **Nothing guards either generated block.** `grep -l renderZshTheme renderFishConfig`
  over `installer/internal/tui/*_test.go` returns nothing: the stage-1 guard reads
  `config.fish` as a file and never renders it. A refactor of the zsh renderer
  therefore has no test to catch a change, which is why S6 is guarded here.
- **The sentinel path is the one that runs.** `replaceThemeBlock`
  (`installer.go:1342`) is tried before `adoptThemeBlock`, and `config.fish`
  carries both markers (`:121`, `:149`). The fish artifact's `AdoptStart`/`AdoptEnd`
  anchors (`set -l foreground F3F6F9 normal` / `set -g fish_pager_color_description $comment`,
  `installer.go:1293`) are the hand-written-to-generated bridge only, so stage 2
  leaves them alone.
- **The two blocks as they stand**, rendered from every theme in `themes/` before
  the change and kept for comparison: zsh 91 lines, 6567-6575 bytes; fish 27 lines,
  1533-1541 bytes. Both tables live in the zsh block, as literal strings
  (`zshLsColorsBlock`, `installer.go:3513`; `zshEzaColorsBlock`, `:3541`) whose every
  value is a `${PALETTE_*_SGR}` reference.

### Specs

S1. For every theme in `themes/`, the fish block's `LS_COLORS` and `EZA_COLORS` carry
    the same mapping the zsh block's do: the same keys, in the same order, each
    taking the same palette role.
S2. The mapping is written once. No key is stated in a shell's own renderer, so a key
    added to the mapping reaches both shells and neither can drift from the other.
S3. The fish tables read the theme's own values, computed the way zsh's are: the SGR
    sequence of the palette role the mapping names, so a switch moves both shells and
    neither falls back to a built-in sixteen.
S4. What fish is given is fish. The emitted block holds no `${VAR}` and no `export `,
    and declares every variable its tables read before reading it.
S5. The fish block stays the generated region: still inside the two sentinels, still
    replaced whole by a switch, and nothing outside it moves — the stage-1 aliases,
    the `EZA_ICON_SPACING` line and the hand-written functions are untouched.
S6. The zsh block is byte-identical before and after. This change moves *where* the
    palette is computed, not what zsh is given.

### Tasks

T1. Compute the palette once, for every renderer that reads it.
T2. Render the two listing tables from the single mapping, in each shell's spelling.
T3. Emit the palette and both tables in the fish block, above the fish roles.
T4. Guard it: the two shells agree on mapping and values, and fish is given fish.

### The proof (stage 2)

| what | value |
|------|-------|
| the three guards against the unmodified renderer | **all three fail**: `the fish block declares no LS_COLORS at all, so LS_COLORS reaches the tool unset and the listing falls back to that tool's own colours: the fish block holds 1541 bytes and no listing table` |
| the same three after the change | pass, over all seven definitions in `themes/` |
| the two shells compared entry for entry | 67 `LS_COLORS` entries and 66 `EZA_COLORS` entries, identical under both shells |
| mutation: the fish table loses one entry | `TestBothShellsListFilesFromTheSameMapping` fails naming both sides: `zsh's LS_COLORS carries 67 entries and fish's carries 66`, and `entry 0 of LS_COLORS is "rs=0" under zsh and "di=38;2;30;102;245" under fish` |
| mutation: fish is handed zsh's `export` untranslated | the mapping guard fails (`declares no LS_COLORS at all ... the fish block holds 6120 bytes and no listing table`) and the shape guard fails with it |
| mutation: both halves paint a colour the theme never chose | **only** `TestFishListingTablesPaintTheThemesOwnValues` fails -- `di is painted "38;2;1;1;1", which is not a value in themes/catppuccin-latte.toml` -- while the mapping guard stays green, because both shells moved together |
| each mutation restored afterwards | byte-identical to the version under test, checked with `diff -q` |
| the zsh block, rendered for three themes before and after | **identical byte for byte**, all three (91 lines, 6567-6575 bytes) |
| `fish -n` on the rendered block | syntax OK |
| the block sourced in `fish --no-config` | `LS_COLORS` resolves to 67 entries, `EZA_COLORS` to 66, `di` is the theme's blue, and `env` carries `LS_COLORS`, so eza reads it |
| `dotfiles-fish/fish/config.fish`, regenerated | +68 lines and no deletions: the palette and the two tables, inside the same two sentinels, with `fish_pager_color_description` still the block's last line |
| `TestGeneratedThemeArtifactsMatchTheirDefinition` before regenerating | **fails**: `dotfiles-fish/fish/config.fish no longer matches themes/dotfiles.toml: regenerate it with go test ./internal/tui -run TestGeneratedThemeArtifactsMatchTheirDefinition -update-theme-artifacts` |
| the same guard after regenerating | passes; the other thirteen artifacts already matched byte for byte |
| `gofmt -l ./internal ./cmd` | no output |
| `go vet ./...` | exit 0 |
| the full suite, unfiltered, before regenerating | **5 failures**: the 4 Ghostty ones and the artifact guard above |
| the full suite, unfiltered, after regenerating | **the 4 Ghostty failures only**, the same four this file and `theme-survives-install.md` already record |

### Log

**L1.** The mandate is this file's task 5, quoted verbatim above. Nothing was added to
it: the icon spacing was already done, outside the generated region, by stage 1.

**L2.** fish's parser, measured rather than reasoned about (`fish --no-config`):

```
expandido -> [rs=0:di=38;2;1;2;3:ex=38;2;4;5;6]
lineas = 0
LS_COLORS=rs=0:di=38;2;1;2;3:ex=38;2;4;5;6
```

A bare `$VAR` expands inside a double-quoted value, the backslash at the end of a line
continues that value without keeping the newline (`lineas = 0`), and `set -gx` exports
the result.

**L3.** The space before the quote is load-bearing, and the naive translation is what
measured it:

```
set: A=uno:dos: invalid variable name. See `help language#shell-variable-and-function-names`
set: Did you mean `set A uno:dos`?
```

So `export NAME="` cannot become `set -gx NAME="`; fish refuses it. The translation
emits `set -gx NAME "` and the value as an argument of its own.

**L4.** **A wrong measurement, recorded because it was acted on before it was caught.**
A search for the renderers by name -- `grep -l renderZshTheme renderFishConfig` over
`installer/internal/tui/*_test.go` -- returns nothing, and I concluded that no test
guarded either generated block, so the shipped artifact could be left alone. The suite
said otherwise: `TestGeneratedThemeArtifactsMatchTheirDefinition`
(`install_paths_test.go:2665`) renders every artifact through the same table the
installer uses and compares it with the committed file, and it failed naming
`dotfiles-fish/fish/config.fish`. The search was for an identifier; the guard works
through the artifact's `Render` field. What was true is narrower: nothing calls the
renderers *by name*.

**L5.** The regression is regenerated, never hand-edited: the guard's own message
carries the command (`-update-theme-artifacts`) and it goes through
`themeArtifactBlock` + `replaceThemeBlock`, the path the installer takes.

**L6.** Design: the block declares the palette's SGR values and renders both tables from
the two constants zsh's block is rendered from, rather than inlining the sequences into
the tables. Inlining would have been fewer lines and no global variables, and it was
rejected because it hides what this stage is -- one mapping, read by both shells -- and
because it stops the two blocks from being read side by side.

**L7.** Order inside the block: the palette, then the tables, then fish's eighteen roles.
The roles stay last because `installer_test.go:72` uses `set -g fish_pager_color_description`
as the end of the shipped block when it checks that the drop-in loader comes after it.

**L8.** The values guard allows exactly one entry to name no palette colour: `rs=0` is
SGR's reset and is in the table on purpose. Any other entry without one fails.

**L9.** A mutation that does not compile measures nothing. The first form of the third
mutation -- forcing every SGR value to a constant -- left a local variable unused, so
the package failed to build and the guards never ran; it was rewritten so that both
halves receive the same value outside the theme, which is what makes only the values
guard fire.

## Remaining

Stages 3 and 4 are open; each is its own change and its own review. Stage 2 is written
and awaiting its merge: it is the one that makes fish's listing the same *colours*, not
only the same shape.
