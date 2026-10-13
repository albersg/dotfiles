# The applied theme survives an install

## Goal

Make the installer keep the theme the machine has applied.

`applyDotfilesTheme` is called only from the picker, and every install writes the files this repository
ships — which carry the **default** theme's blocks. So an install silently discards an applied theme
for each file it rewrites, while the record that says which theme is applied keeps naming it. Nothing
reports it: the picker's own count stays true of a machine that no longer matches it.

This front re-applies the recorded theme at the end of a run, once, for every artifact whose file the
run rewrote.

## Evidence, measured on this machine (read-only)

The theme picker wrote Nocturne into six files at `2026-10-13T15:31:19`, recorded in
`~/.local/state/dotfiles/theme.json`. An install ran at `16:59:42`. Reading the files afterwards:

| file | the theme block's tag | mtime |
|------|----------------------|-------|
| `~/.config/starship.toml` | `dotfiles-theme-palette: nocturne` | 15:31:19 |
| `~/.config/nushell/config.nu` | `dotfiles-theme: nocturne` | 15:31:19 |
| `~/.zshrc` | `dotfiles-theme: dotfiles` | **16:59:42** |
| `~/.p10k.zsh` | `dotfiles-theme: dotfiles` | **16:59:42** |
| `~/.config/herdr/config.toml` | `dotfiles-theme: dotfiles` | **16:59:42** |

`~/.zshrc` contains no occurrence of `nocturne` at all. The record still says `nocturne` over six
files, and `theme.json`'s own timestamps say the record was not rewritten by the install.

**The reset is per step, not uniform.** `starship.toml` and `config.nu` kept the theme; the three above
lost it. So this is not "the install deletes everything the switch wrote" but "each step that writes a
shipped file decides for itself, and nothing puts the theme back".

**What the user sees.** The prompt reads `PALETTE_*` from `~/.zshrc` and `~/.p10k.zsh`. Measured in a
real interactive shell on that machine after the install:

```
PALETTE_BLUE=#7fb4ca        POWERLEVEL9K_OS_ICON_FOREGROUND=#7fb4ca
POWERLEVEL9K_FOREGROUND=#f3f6f9   POWERLEVEL9K_PROMPT_CHAR_OK_VIINS_FOREGROUND=#b7cc85
```

Those are `themes/dotfiles.toml`'s values, not Nocturne's (`#8686cb`, `#c0c0c0`, `#86cb86`). The
interface reported the theme as applied, the record named it, and the prompt was the default palette.

**Why the same configuration looked right on one machine and wrong on another:** there the theme had
been applied *after* the last install. The order is the whole difference.

### Where it happens

`stepInstallShell` (`installer/internal/tui/installer.go`) writes both files straight from the
checkout:

```go
system.ReplaceUserConfig(filepath.Join(repoDir, repoAssetZshrc), zshrcPath, ...)
system.CopyFile(filepath.Join(repoDir, repoAssetP10k), filepath.Join(homeDir, ".p10k.zsh"))
```

and `stepInstallWM` does the same for the Herdr configuration. Neither consults the record.

`readDotfilesThemeRecord()` has two readers — the Windows Terminal scheme
(`wslterminal.go`, v0.7.6) and the interface's own display (`update.go`) — and neither writes a theme
block. `applyDotfilesTheme` is called from exactly one place, `update.go`'s picker. So no step
re-applies the theme, and there is no step whose job is to.

## What is already right, and must stay right

- **The record is the source of truth.** `writeDotfilesThemeRecord` is written by the picker only, and
  a re-application must not touch it: applying a theme is a user action with a transaction and an undo
  record, and an install is not one.
- **The undo record.** `applyDotfilesTheme` records the previous bytes of every file it writes so the
  picker can take the change back. A re-application has no such business, and must leave `Files` alone.
- **A theme the repository no longer defines is not a reason to fail a run.** A checkout can predate a
  theme that is recorded, and the run has already done its work. The step reports it and leaves the
  files alone, which is the rule the Windows Terminal scheme follows for the same reason.
- **The step must not invent a theme-block write for a file it does not own.** `themeInstalledFileIsOurs`
  and `replaceThemeBlock` are the same two gates the picker uses, and a file that fails them is left
  exactly as it is.
- **The order.** The checkout the install works from is a temporary directory `stepCleanup` removes
  (`m.WorkDir`, cleared with `m.RepoDir`), so the re-application has to run before cleanup and after
  every step that writes a configuration file.

## Tasks

1. Guard it at the helper: a machine whose files carry the default theme while a theme is recorded must
   come out of the install naming the recorded theme.
2. Guard the fallbacks: no record leaves every file alone; a record naming a theme the checkout does not
   define leaves every file alone and does not fail the run.
3. Guard the wiring: the step exists, is scheduled by both planners, and its place in the sequence is
   after the steps that write configuration files and before cleanup.
4. Write the re-application, reusing the picker's gates and renderer rather than a second copy of them.
5. Update `CHANGELOG.md` and the documentation that describes what an install preserves.

## The proof

| what | value |
|------|-------|
| the helper guard, against the step with the work not yet written | fails, four times: `~/.zshrc` does not name nocturne, `BAT_THEME` is not nocturne, `~/.p10k.zsh` does not name nocturne, and the applied palette is missing |
| the fallback guards, against the same | pass (they pass before and after) |
| the ordering guard, against the same | passes (the stub is scheduled where it belongs) |
| every guard after the fix | pass |
| mutation: the step's body replaced by the stub again | the helper guard fails |
| mutation: the step scheduled before `shell` in `planFor` | the ordering guard fails |
| mutation: the step removed from `stepExecutors` | `TestEveryScheduledStepHasAnExecutor` fails |
| mutation: the record read ignored, the default theme used instead | the helper guard fails |

## What the change cost elsewhere, and how it was paid

A step is part of a list, so adding one moves a count and a sequence. Both were checked rather than
regenerated blindly.

- **Ten golden frames changed, and every changed line is the new count.** The diffs are `Plan 8 steps`
  to `Plan 9 steps`, `Steps 8` to `Steps 9`, `3/14` to `3/15`, `Step 3 of 14` to `Step 3 of 15`, the
  progress percentage, and the remaining-time estimate that follows from them. No layout change, no
  reflow, no text change beyond the count. `-update` was run only after reading the diffs, and the
  footprint is 9 files and 15 lines.
- **Six step-list assertions changed, because they state the plan.** `TestSetupInstallStepsMinimal`,
  `TestSetupInstallStepsFullInstall`, `TestInstallStepsSetup` (three configurations),
  `TestSetupInstallSteps`, `TestWillHappenPanelPreviewsThePlan` and
  `TestMainMenuPanelDescribesTheOptionUnderTheCursor`. Each expected list gained `theme` between
  `officecli` and `setshell`, and each expected count gained one.
- **`gofmt`, `go vet ./...` and `go build ./...` are clean**, and the full suite has the same four
  pre-existing Ghostty failures as clean `origin/main` and no others.

## Remaining

Nothing. The re-application is in the run, the four guards are in place, and the record is untouched.
