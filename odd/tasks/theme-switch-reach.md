# The theme switch's reach

## Goal

Make two promises the theme switch makes about itself true.

The switch declares a set of artifacts it owns and rewrites, and it tells the user, on the installing
screen, what to run so an already-running tool picks up the new palette. One declared artifact is
never written, and one of the actions it names cannot work. Both failures are silent: neither the
switch nor its screen says anything went wrong.

This is one front because it is one question asked in two places — what the switch actually reaches —
and because the two answers were found by the same measurement on the same screen.

## Evidence, measured on this machine (read-only)

Machine: WSL2, Windows user `alber`, herdr 0.9.3, dotfiles v0.7.6. Theme applied: `nocturne`,
recorded in the installer's own theme state at `2026-10-13T15:31:19+02:00` over 6 files
(`~/.zshrc`, `~/.p10k.zsh`, `~/.config/starship.toml`, `~/.config/herdr/config.toml`,
`~/.config/nushell/config.nu`, `~/.config/nvim/lua/plugins/colorscheme.lua`).

### The screen's own numbers

The theme picker's result paragraph, after applying Nocturne:

| what | what it says |
|------|--------------|
| the sentence | `Partial: 7 file(s) were themed and 2 file(s) were left exactly as they are` |
| the two left alone | `~/.config/alacritty/alacritty.toml`, `~/.config/ghostty/config` — not owned by dotfiles |
| the count claim | `The Nocturne theme is applied to 7 file(s).` |

`themeActiveArtifacts` declares **14** artifacts. Two are refused for a reason the screen states
(the two files are not owned), and four name files that do not exist on this machine (`kitty`,
`wezterm`, `fish`, `tmux`, which `applyDotfilesTheme` skips when the file is absent). That leaves
**8** artifacts that should have been themed. The screen reports **7**.

### D1 — the `bat` artifact resolves to no path, so the switch skips it in silence

`applyDotfilesTheme` (`installer/internal/tui/installer.go`) starts every iteration with

```go
path := themeInstalledPath(art, homeDir)
if path == "" {
    continue
}
```

and `themeInstalledPath` is a `switch art.Tool` with cases for `alacritty`, `kitty`, `wezterm`,
`ghostty`, `herdr`, `starship`, `zsh`, `p10k`, `nvim`, `fish`, `tmux` and `nushell`. There is **no
`bat` case**, and the fall-through returns `""`.

The `bat` artifact is declared (`installer.go`, `themeActiveArtifacts`):

```go
{Tool: "bat", Path: "dotfiles-zsh/.zshrc", Comment: "#", Block: "bat", NotBoxed: true,
 AdoptStart: "# --- bat ", AdoptEnd: "# --- zsh-autosuggestions", AdoptEndKeep: true,
 Render: renderBatSelection},
```

Measured over all 14 artifacts, with a throwaway test that printed `themeInstalledPath` for each:

```
UNREACHABLE tool="bat" block="bat" path="dotfiles-zsh/.zshrc"
```

It is the only one. Every other artifact resolves.

Reproduced by applying `nocturne` to a copy of this machine's `~/.zshrc` in a temporary HOME:

| what | before | after |
|------|--------|-------|
| the zsh block's tag | `>>> dotfiles-theme: nocturne >>>` | still `nocturne` — the zsh artifact ran |
| the bat block's tag | `>>> dotfiles-theme-bat: dotfiles >>>` | **unchanged** |
| `BAT_THEME` | `"dotfiles"` | **unchanged** |
| `themed` | — | **1**, where the two artifacts both reaching the same file would make it 2 |
| the skip list | — | empty, so no `Partial` sentence is emitted |

**Effect.** `BAT_THEME` is the theme the repository ships, for every theme, on every machine. `bat`
renders the default palette while the rest of the machine renders the applied one, and `cat` is
aliased to `bat` in the shipped `.zshrc`, so the file viewer is the surface a user meets first. The
generated `.tmTheme` files are all present and correct — `~/.config/bat/themes/` holds `dotfiles`,
`nocturne`, `kanagawa`, `everforest`, `rose-pine` and both Catppuccin variants — and nothing ever
selects any of them.

The screen's line for the tool is true about the cache and false about the selection: "bat — bat's
theme cache was rebuilt, so the new theme exists; open a new shell (or `exec zsh`) so `BAT_THEME` is
read." The cache is rebuilt; the value it would read never changes.

**Why nothing caught it.** `install_paths_test.go` iterates `themeActiveArtifacts` for several
purposes — the generated `file.PerTheme` tokens, the committed values in the repository's own files,
the per-tool colour tables — but no test asserts that each declared artifact resolves to a path. The
renderer is exercised directly by tests that never go through `applyDotfilesTheme`, so a renderer
that works and an artifact that is unreachable look the same from the test's point of view.

### D2 — the reload the installer names cannot apply a theme change

The picker's result paragraph tells the user, per tool, what to run. For the prompt:

> **p10k** — a prompt that is already drawn keeps the old palette: run `p10k reload` in that shell, or
> open a new shell.

`p10k reload` re-sources `POWERLEVEL9K_CONFIG_FILE`, which is `~/.p10k.zsh`. The generated palette
block in that file defaults each value instead of setting it:

```
typeset -g PALETTE_BASE=${PALETTE_BASE:-"#151316"}
typeset -g PALETTE_BLUE=${PALETTE_BLUE:-"#8686cb"}
```

The shipped `.zshrc` sets the same names **unconditionally** and is sourced first:

```
typeset -g PALETTE_BLUE="#8686cb"       PALETTE_BLUE_SGR="38;2;134;134;203"
```

and `.zshrc` sources `~/.p10k.zsh` last (line 851 of the shipped file). So in a shell that was
already open when the theme changed, `PALETTE_*` still holds the **previous** theme's values, and the
`:-` in the reloaded file preserves exactly those. Measured in zsh, with the previous theme's blue
(`#7fb4ca`) live in the shell, which is what a shell opened before the change holds:

| shell | `POWERLEVEL9K_DIR_FOREGROUND` | `POWERLEVEL9K_FOREGROUND` |
|-------|-------------------------------|---------------------------|
| open before the change, then `p10k reload` | `#7fb4ca` — the previous theme | `#f3f6f9` — the previous theme |
| a new shell | `#8686cb` — nocturne's blue | `#c0c0c0` — nocturne's text |

**Effect.** The one in-place action the installer offers for the prompt is the action that cannot
work. A user who follows the instruction exactly keeps the previous prompt palette until they open a
new shell, and nothing tells them the instruction failed.

**Why the fallbacks are there, and why they do not earn their cost.** The block's own comment says
they "keep this file working when it is sourced on its own, without `.zshrc`". An assignment sets the
file's values whether or not the name is already defined, so unconditional lines keep that property
too: the file defines its own palette either way. What the conditional form adds is the ability to
lose to a value that is already in the environment — which for a generated block that names the
applied theme is the one thing it must never do.

## What is already right, and must stay right

- **The p10k palette block's placement and reach.** The switch rewrites only the palette region
  (`AdoptStart: "  # ── Palette"`, `AdoptEnd: "typeset -g PALETTE_CYAN="`), and every element
  assignment below it is a static `$PALETTE_*` reference, so one region repaints the whole prompt.
  Nothing about that changes here.
- **The refusal to invent a colour.** `renderP10kTheme` and `renderBatSelection` both fail a
  definition that carries no value for what they must emit. The fixes feed them the same definition,
  never a new rule.
- **The bat block's file check and its Catppuccin fallback**, which keep a machine whose `.tmTheme`
  cache is not built yet from turning every `bat` call into `Unknown theme`.
- **The empty `POWERLEVEL9K_BACKGROUND`**, which is deliberate: it lets the terminal's own background
  show through instead of painting a second black behind the prompt.
- **The per-tool instructions themselves.** They are generated from one list
  (`windowsTerminalSettings`'s sibling for the theme picker), so the screen and the writer cannot name
  different things. D2 is a wrong action name, not a duplicated list.

## Tasks

1. Guard D1 with teeth: a test that fails unless every artifact in `themeActiveArtifacts` resolves to
   a path.
2. Guard the bat block: applying a theme that is not the default must leave that theme's name in the
   `BAT_THEME` line and in the block's tag.
3. Make the `bat` artifact reachable, so `themeInstalledPath` answers for it the same way it answers
   for the `zsh` artifact that writes the same file.
4. Guard D2 against a real shell: the generated palette block, sourced into zsh where the previous
   theme's values are already set, must leave the theme's own values.
5. Emit the p10k palette as assignments, so re-sourcing the file applies the theme.
6. Update `CHANGELOG.md` and the documentation that states what the switch reaches.

## The proof

| what | value |
|------|-------|
| the reachability guard, against the untouched code | fails, naming `bat` |
| the bat-block guard, against the untouched code | fails |
| the reload guard, against the untouched code | fails, showing the previous theme's value |
| all three after the fixes | pass |
| mutation: the `bat` case removed again | the reachability guard fails |
| mutation: the p10k palette guarded again | the reload guard fails |

## Commits

| commit | what |
|--------|------|
| `eb73876` | `fix(theme)`: the reachability case, the unconditional palette, and the three guards |
| the `docs(theme)` commit on this branch | this document, the index row, `CHANGELOG.md` and `docs/tui-installer.md` |

## Remaining

Nothing. Both defects are fixed in the same change, each with the guard that was missing.

Deliberately outside this front, and each its own concern:

- The theme switch still does not repaint the Windows Terminal scheme. Measured again here: after
  Nocturne was applied, `state/dotfiles/theme.json` recorded six rewritten files and the Windows side
  was not among them, while the `dotfiles` scheme in `settings.json` did hold Nocturne's palette —
  because v0.7.6 made the scheme read the applied-theme record each time the step runs, not because
  the switch repainted it. Closing that half means the picker offering a Windows-side write after a
  theme change, under the documented "only after an explicit yes" rule, with its own test.
- `themes/nocturne.toml` says its own palette was measured from a screenshot and that its
  red/green/cyan/blue/magenta were derived at saturation 40% and lightness 66%, and it closes the
  discussion by saying whether it looks the same is the user's call. That is a decision about the
  theme, not a defect in the switch, and it is not touched here.
