# The dotfiles themes: one definition, every part

This directory is the single source of truth for the dotfiles' own theme — the palette the
repository ships across its terminals, its shell prompt, `bat`, `fish`, tmux and Herdr. Before this
directory existed the same palette was written by hand in **six** files (`alacritty.toml`,
`.wezterm.lua`, `dotfiles-kitty/kitty.conf`, `dotfiles-ghostty/config`, `starship.toml`,
`dotfiles-zsh/.zshrc` and its `.p10k.zsh` fallbacks), so the same colour was maintained by hand in
each of them and a drift between two of them was invisible until two terminals were put side by
side. A theme is a file here now.

## No colour is invented

A value comes from the repository, in this order:

1. **What the repository already ships wins.** The `dotfiles` palette is transcribed from the six
   hand-written blocks above. `catppuccin-mocha` is transcribed from the repository's own
   `dotfiles-ghostty/themes/catppuccin-mocha.conf` (the terminal mapping) and its
   `starship.toml` `[palettes.catppuccin_mocha]` table (the named palette).
2. **A role the repository does not fill stays empty.** It is never extrapolated by eye. If a
   theme misses any canonical role it is marked `partial` and is **not** offered as a switch; the
   menu reports it and names the tools it would leave on the old palette.

There is no network fetch. Every value in these files can be found in this repository.

## The canonical roles

A complete theme defines all twenty-two, in this order:

```
base, text, cursor, cursor_text, selection, selection_text,
black, red, green, yellow, blue, magenta, cyan, white,
bright_black, bright_red, bright_green, bright_yellow,
bright_blue, bright_magenta, bright_cyan, bright_white
```

`black`…`white` are the terminal's ANSI colours 0–7 and `bright_*` are 8–15; `base`/`text` are the
background/foreground, `cursor`/`cursor_text` the cursor pair, and `selection`/`selection_text` the
selection pair. The prompt's own roles are derived from these mechanically (its "muted" is
`bright_black`, its "surface" is `selection`) rather than stored twice.

### The syntax roles

`[syntax]` holds the installer's own code-display tints: a keyword colour and a string colour, each
with a light and a dark member, because the chrome is adaptive (it asks the terminal whether its
background is dark, the same detection the default palette uses).

- **dotfiles** — `keyword_light #7a3e9e`, `keyword_dark #c99ad6`, `string_light #8a6a00`,
  `string_dark #dfbd76`: transcribed from the two pairs `styles.go` already carried. `styles.go` now
  reads them and `TestTheDefaultStylesMatchTheDotfilesDefinition` pins **both** members, so these are
  theme roles and the guard no longer has to name an exception.
- **Catppuccin Mocha** — `keyword_dark #cba6f7`, `string_dark #a6e3a1`, both already in this
  repository's `starship.toml` `[palettes.catppuccin_mocha]` table. The choice follows **Catppuccin's
own convention**, which paints keywords *mauve* and strings *green*; neither value is invented.
- **Kanagawa, Everforest, Kagawa** — empty, like the rest of their canonical palette.

**A readability limit, stated rather than hidden.** Catppuccin Mocha ships no light member here: the
repository holds only Mocha, and the light flavour (Latte) is not in it, so a light value would be an
invented colour. The preview therefore uses the dark value on a light terminal as well, and Mocha's
mauve `#cba6f7` and green `#a6e3a1` are light colours drawn for a dark background, so they are
low-contrast on white. That is a property of the theme, not of the preview — the default chrome keeps
its own darker light members — and it is the honest alternative to fabricating a Latte value. The
preview guard logs it so it stays visible.

## The five themes

| Theme | File | State | Source of its values |
|---|---|---|---|
| dotfiles | `dotfiles.toml` | **complete** | the repository's six hand-written blocks, verbatim |
| Catppuccin Mocha | `catppuccin-mocha.toml` | **complete** | `dotfiles-ghostty/themes/catppuccin-mocha.conf` + `starship.toml` `[palettes.catppuccin_mocha]` |
| Kanagawa | `kanagawa.toml` | partial | `dotfiles-fish/fish/themes/Kanagawa.theme` (eight fish roles); nvim names the plugin's theme |
| Everforest | `everforest.toml` | partial | `dotfiles-fish/fish/themes/Everforest.theme` (eight fish roles) |
| Kagawa | `kagawa.toml` | partial | none of its own — see the defect below |

### Complete (offered)

**dotfiles** covers **[Alacritty, Kitty, WezTerm, Ghostty, Starship, Herdr, the zsh line editor, the
p10k prompt, bat, fish, tmux]** and leaves out **[Neovim]** — it names no colorscheme of its own.
**Catppuccin Mocha** covers every tool whose colours this repository owns, the same eleven plus
**Neovim**, whose `catppuccin` colorscheme the plugin it already ships provides, and leaves out
nothing.

Generated blocks: the four terminals, Herdr, Starship (both its `palette` line and its
`[palettes.<id>]` table), the zsh palette region including the `*_SGR` twins and the
`LS_COLORS`/`EZA_COLORS` tables, the p10k fallbacks, the `BAT_THEME` selection, the four fish theme
files, the fish palette block in `dotfiles-fish/fish/config.fish`, the tmux style block in
`dotfiles-tmux/tmux.conf`, and the bat `.tmTheme` files. Neovim's colorscheme line is generated for
the themes whose plugin ships one.

**fish is switched, through the file this repository owns.** Its theme files are produced from the
definitions, but which one is active is **the user's own state**: fish keeps the colour variables a
`fish_config theme choose` writes in `fish_variables`, which this repository does not own. Writing
it would be editing the user's settings, which the `preserve-user-configs` rule forbids. The switch
therefore paints fish the way it paints every other tool: it rewrites a marked block in
`dotfiles-fish/fish/config.fish`, a file the repository does own, and records the exact bytes so the
change is reversible. The block sets the fish colour variables in the **global** scope, which fish
returns over a universal one, so the palette is seen even after a `fish_config theme choose` the user
ran earlier. `fish_variables` is never written.

- **dotfiles** — complete (22/22 canonical roles) and complete for the prompt too (all 24 required
  `[prompt]` roles).
- **Catppuccin Mocha** — complete on both tables. Its `[prompt]` table carries `sky` and `sapphire`
  because the repository holds their values; they are generated and not required, so a theme without
  them is not reported as missing Starship.

A complete theme is *offered*; the tools it cannot paint are *named*. The menu must draw that list.

### Two tables, not one

The definition has a `[palette]` table (the terminal's 22 roles) and a `[prompt]` table (Catppuccin's
naming: `mauve`, `peach`, `subtext0`, `overlay0`, `maroon`, `lavender`, `overlay2`, `overlay1`,
`surface2`, ...). Two tables are more honest than one: nine of the prompt roles are not terminal
roles at all, and a single table would invite filling them from a nearest ANSI colour. A prompt role
is filled only when the repository holds its value; a theme missing one is reported as not
switchable for the tool that needs it, never extrapolated.

### Findings recorded while generating

- **The zsh/p10k region is fully derivable.** Every `PALETTE_*` value, its `*_SGR` twin and every
  `LS_COLORS`/`EZA_COLORS` entry names one of the ten palette roles; no hand-written value outside
  the palette was found, so the region is generated whole rather than partly.
- **`Kagawa.theme` shares Kanagawa's palette.** Its generated file now carries its own `Kagawa`
  header (the hand-written one said `name: Kanagawa Fish shell theme`) and the two malformed hex
  values are gone, but the colours are still Kanagawa's: it was never a distinct theme.
- **`bat` is generated and switched.** `dotfiles.tmTheme` and `catppuccin-mocha.tmTheme` are produced
  from the definitions, and the `BAT_THEME` selection in `.zshrc` is a generated region too. The
  installer copies every shipped `.tmTheme` and rebuilds bat's cache, so choosing a theme cannot land
  on an "Unknown theme". The syntax-to-role mapping is written down in the generator
  (`themeBatRoles`), because a scope name does not say which role it takes.
- **A generated bat file's own name is a role too.** The `.tmTheme`'s `<key>name</key>` is what
  `BAT_THEME` must match, so it is generated from the definition (`[bat] name`) rather than left as a
  second literal: the first generated catppuccin file named itself "dotfiles" until this was fixed.
- **The deleted `[palettes.catppuccin_mocha]` table is recreatable.** Removing dead data is only
  legitimate while the generator can rebuild it: `TestTheRemovedStarshipPaletteIsRecreatable` renders
  `themes/catppuccin-mocha.toml`'s `[prompt]` table into
  `[palettes.catppuccin-mocha]` and pins all 26 values the deleted table held. If that guard ever
  fails, the deletion lost content and must be reverted.
- **`tmux` has a repository-owned theme now.** Its only theme used to be the `tmux-kanagawa`
  plugin's name, and Kanagawa is partial (0 of 22 roles), so nothing could be generated from it. tmux
  accepts colours of its own, though, so the switch paints tmux's own style options — status bar,
  active and inactive windows, panes, copy mode, display panes and the clock — from the definition,
  in a marked block in `dotfiles-tmux/tmux.conf`. No second palette is invented: every colour is a
  role the definition already holds. The block is placed **after** the
  `run '~/.tmux/plugins/tpm/tpm'` line, because measurement shows tmux runs `run-shell`
  synchronously: the server does not finish reading `tmux.conf` until the command returns, so the
  plugins TPM sources have already written their styles by the time the block is read, and the block
  wins. When the kanagawa plugin is installed, its window colours are therefore overridden by this
  block rather than by disabling the plugin.

### Partial (reported, never offered)

- **Kanagawa** — `dotfiles-fish/fish/themes/Kanagawa.theme` holds only fish's own role names
  (`fish_color_normal`, `fish_color_command`, …), and nvim names the plugin's theme. The
  terminal roles (`base`, `cursor`, `selection`, the 16 ANSI colours) do not exist in this
  repository, so they are left empty. A Kanagawa switch would leave nine of the twelve tools on the
  old palette, which is why it is reported instead of offered.
- **Everforest** — one fish file, nothing else.
- **Kagawa** — nothing of its own.

## Two defects recorded here rather than hidden

- **`Kagawa.theme` is a copy of `Kanagawa.theme`.** The values are identical and its own header line
  reads `# name: Kanagawa Fish shell theme`. Kagawa therefore has no distinct palette in this
  repository.
- **Both fish files carry malformed colours.** `Kanagawa.theme` (and its copy `Kagawa.theme`) write
  `fish_color_selection --background=2D4FG67` (`G` is not a hex digit) and
  `fish_pager_color_description 727269`, where `727169` is meant. Nothing in this change rewrites
  those files; the point is that a hand-written theme file drifts, and these are the drift.

## What enforces this

The guards live in `installer/internal/tui/install_paths_test.go`:

- `TestThemeListIsDerivedFromDefinitions` — the menu's list is read from `themes/*.toml`; adding a
  file adds a row, and the two lists cannot disagree.
- `TestOnlyCompleteThemesAreOffered` — a theme missing a canonical role is partial and is never
  offered.
- `TestEveryThemeReportsTheToolsItWouldLeaveOut` — every theme names the tools it cannot paint.
- `TestShippedThemeBlocksMatchTheirDefinition` — every value a definition claims must still be
  present in the shipped block it came from.
- `TestGeneratedThemeArtifactsMatchTheirDefinition` — the four terminal files must be byte-for-byte
  what the generator produces from `themes/dotfiles.toml`. Run it with `-update-theme-artifacts` to
  regenerate; running it without that flag is what fails when somebody edits a generated block by
  hand.
- `TestThemeGeneratorUsesTheDefinition` and `TestThemeGeneratorRefusesAMissingRole` — the block a
  tool gets is its own theme's colours, and a definition missing a role is refused rather than
  emitted half-empty.
- `TestGeneratedThemeBlocksInventNoColour` — every colour a generated block emits must be one the
  definition already holds, in its `[palette]`, `[fish]` or `[prompt]` table. A renderer that fills a
  role by eye fails here.
- `TestTheFishDerivationMatchesTheDotfilesTable` — the code mapping that derives a fish palette from
  the canonical roles must still produce the values `themes/dotfiles.toml` records in its `[fish]`
  table, so a theme without one (Catppuccin Mocha) gets the same palette by derivation.
- `TestTmuxThemeBlockLoadsAfterPlugins` — the tmux block must sit after the TPM `run` line, which is
  what makes it win over the kanagawa plugin.

The same parser the guards use (`loadThemeDefinitions`) is what the installer reads at run time, so
there is one definition and one reader.

Every generated file carries `# dotfiles-managed-config: <tool>` — the ownership marker the
`preserve-user-configs` rule reads — and a `>>> dotfiles-theme: <id> >>>` … `<<< dotfiles-theme <<<`
pair around the generated block, so a switch can find and replace exactly its own region.
