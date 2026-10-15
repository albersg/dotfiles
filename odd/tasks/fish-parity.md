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

## Remaining

Stages 2, 3 and 4 are open; each is its own change and its own review. Stage 2 is
the one that makes fish's listing the same *colours*, not only the same shape.
