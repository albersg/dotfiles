# The Windows Terminal scheme follows the applied theme

## Goal

Make the Windows Terminal colour scheme carry the palette of the theme the machine has
**applied**, instead of the palette of the default theme, so the Windows side of a WSL install is
painted from the same colours as the shell, the prompt and every other tool the theme switch reaches.

This front exists because the code states that invariant and does not hold it. The comment on
`windowsTerminalThemeDefinition` reads "The scheme is the dotfiles theme, the one the installer
applies to every other tool, so the renderer paints the same palette as the shell", and the function
below it loads `defaultThemeID` unconditionally, whatever theme is applied.

## Evidence, measured on this machine (read-only)

Machine: WSL2, Windows user `alber`, Windows Terminal 1.24.12741.0. The applied theme is
`nocturne`, recorded in the installer's own state file.

| what | what it says |
|------|--------------|
| `state/dotfiles/theme.json` | `"dotfiles": { "theme": "nocturne", "applied_at": "2026-10-13T14:03:10.609160833+02:00", "files": 6 entries }` |
| `~/.zshrc` | holds `>>> dotfiles-theme: nocturne >>>` |
| `~/.p10k.zsh` | holds `>>> dotfiles-theme: nocturne >>>` |
| `~/.config/starship.toml` | holds `>>> dotfiles-theme-palette: nocturne >>>` |
| `settings.json` scheme `dotfiles` | `background #06080f`, `foreground #f3f6f9`, `red #cb7c94`, `green #b7cc85`, `blue #7fb4ca`, `cursorColor #e0c15a`, `selectionBackground #263356` |
| `themes/dotfiles.toml` | `base #06080f`, `red #cb7c94`, `green #b7cc85`, `blue #7fb4ca` |
| `themes/nocturne.toml` | `base #151316`, `red #cb8686`, `blue #8686cb` |
| `profiles.defaults` | `{"font":{"face":"MesloLGS Nerd Font"},"colorScheme":"dotfiles"}` |

The eight colours in the scheme are the default theme's, to the digit, while the machine has
`nocturne` applied. The two palettes are far enough apart that this is not a rounding question: the
background is `#06080f` against `#151316`.

## The defect

`windowsTerminalThemeDefinition` (`installer/internal/tui/wslterminal.go`) resolves the palette the
scheme is built from:

```go
def, ok := themeByID(defs, defaultThemeID)
```

`defaultThemeID` is the string `"dotfiles"`. The function never reads the record of the applied
theme, so the scheme is the default theme's palette on every machine, including one whose theme
switch has applied something else. The three consumers of that palette — the scheme written into
`settings.json`, the offer shown on the installing screen, and the value the read-back verifies —
therefore all describe a theme the Linux side is not using.

The name is not the defect and must not change: `windowsTerminalSchemeName` is the constant
`"dotfiles"`, and the scheme is one object that a theme change repaints rather than one object per
theme. Windows Terminal resolves the scheme by name from `profiles.defaults.colorScheme`, so a second
scheme per theme would leave every profile pointing at the old one.

## What is deliberately not in this front

**The theme switch does not repaint the scheme.** `themeActiveArtifacts`
(`installer/internal/tui/installer.go`) lists the files the switch rewrites — alacritty, kitty,
wezterm, ghostty, herdr, starship's two blocks, the zsh palette, p10k, nvim's colorscheme, the bat
selection, fish, tmux and nushell — and it holds no Windows Terminal entry, because every entry in it
is a marked text block in a Linux-side file and `settings.json` is a JSON document on the Windows
side, reachable only through the mount.

Closing that half means the picker has to offer a Windows-side write after a theme change, under the
same rule `docs/tui-installer.md` already states for this step: the font face and the colour scheme
are written only after an explicit yes. That is a new offer flow with its own test, not a line in an
existing one, so it is its own front. Until then, a Windows repaint happens when the WSL config step
runs again, which is what this front fixes to read the record at all.

The alternative — writing `settings.json` silently from `applyDotfilesTheme` — is refused here. That
function's whole shape is a transaction with `previous`/`written` maps and a `restoreWritten` rollback
over paths it owns, and a Windows-side write bolted into it would either sit outside the rollback or
have to join a transaction whose undo record the state file stores per Linux path.

## What is already right, and must stay right

- **The verification gate.** `windowsTerminalLandedValue` reads the file back and withholds the face
  and the scheme when the value does not land. It is what turned "the scheme is wrong" into a
  reported line rather than a silent mismatch, and it is not touched here.
- **The refusal to invent a colour.** `windowsTerminalSchemeFor` refuses a definition with a missing
  role rather than emitting a partial scheme. The fix feeds it a different definition, never a
  different rule.
- **`themes/dotfiles.toml` as the fresh-install palette.** A machine with no record has applied
  nothing, so the default theme is the correct palette for it and stays the fallback.

## Tasks

1. Write the regression guard: a machine whose record names a theme other than the default must get
   that theme's palette in the scheme. Failing first, against the untouched function.
2. Make `windowsTerminalThemeDefinition` read the applied theme from the record, keeping the default
   theme as the fallback for a machine with no record and for a record naming a theme this repository
   no longer defines.
3. Correct the function's comment so it describes what the code does.
4. Update `CHANGELOG.md` and `docs/tui-installer.md`.

## The proof

| what | value |
|------|-------|
| the guard, against the untouched function | **fails** — 21 lines: the resolution (`the scheme's palette came from "dotfiles", want the applied "nocturne"`) and all 20 roles of the scheme |
| the guard, after the fix | passes |
| the fallback test, against the untouched function | passes — and must: the old function always answered the default, so both subtests are what the new code has to keep true |
| mutation: the resolution ignores the record again | the guard fails (1 line) |
| mutation: the record's theme is used without checking the repository defines it | the fallback test fails on the stale-record subtest |
| both mutations reverted | the guard is green again |
| `gofmt -l`, `go vet ./...`, `go build ./...` | clean |
| the full suite, with and without the fix | the identical 4 failures, all pre-existing Ghostty ones, measured on a clean `origin/main` (v0.7.5) as well |
| `make preflight` | stages 1–3 (`gofmt`, `go vet`, `go build` + `--help`) PASS; stage 4 is those same 4 Ghostty failures, so stages 5–7 (shellcheck, branding audit, gitleaks) do not run. Nothing here changes a `.sh` file, and the branding audit's own exclusions cover `CHANGELOG.md` |

## Remaining

The theme switch does not repaint the Windows Terminal scheme; see "What is deliberately not in this
front". A user who changes theme has to re-run the WSL config step for the Windows side to follow.
