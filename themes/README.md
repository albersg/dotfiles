# The dotfiles themes: one definition, every part

This directory is the single source of truth for the palettes the repository can
paint across its terminals, its shell prompt, `bat`, fish, tmux, Herdr and
Neovim. Before this directory existed the same palette was written by hand in
**six** files (`alacritty.toml`, `.wezterm.lua`, `dotfiles-kitty/kitty.conf`,
`dotfiles-ghostty/config`, `starship.toml`, `dotfiles-zsh/.zshrc` and its
`.p10k.zsh` fallbacks), so the same colour was maintained by hand in each of them
and a drift between two of them was invisible until two terminals were put side
by side. A theme is a file here now.

## No colour is invented

A value comes from one of two places, and both are cited:

1. **What the repository already ships.** The `dotfiles` palette is transcribed
   from the six hand-written blocks above. `catppuccin-mocha` is transcribed from
   the repository's own `dotfiles-ghostty/themes/catppuccin-mocha.conf` and its
   `starship.toml` `[palettes.catppuccin_mocha]` table.
2. **A published palette, transcribed and cited.** `catppuccin-latte`,
   `kanagawa`, `everforest` and `rose-pine` are transcriptions of the published
   palettes named in each file's header and in the per-theme notes below. Every
   value is a role the published palette itself names; the source is named with
   its project, its URL and the palette roles it holds.

**A role the published palette does not define stays empty.** It is never
extrapolated by eye. A theme that misses the twenty-two canonical roles or the
`[syntax]` members the preview reads is `partial` and is **not** offered as a
switch; the menu reports it and names the tools it would leave on the old
palette.

There is no network fetch. The transcriptions are committed here, one file per
theme, and the README of each records where its values come from.

## The canonical roles

A complete theme defines all twenty-two, in this order:

```
base, text, cursor, cursor_text, selection, selection_text,
black, red, green, yellow, blue, magenta, cyan, white,
bright_black, bright_red, bright_green, bright_yellow,
bright_blue, bright_magenta, bright_cyan, bright_white
```

`black`…`white` are the terminal's ANSI colours 0–7 and `bright_*` are 8–15;
`base`/`text` are the background/foreground, `cursor`/`cursor_text` the cursor
pair, and `selection`/`selection_text` the selection pair. The prompt's own roles
are derived from these mechanically (its "muted" is `bright_black`, its "surface"
is `selection`) rather than stored twice.

### The syntax roles and the completion rule

`[syntax]` holds the installer's own code-display tints: a keyword colour and a
string colour, each with a light and a dark member, because the chrome is
adaptive (it asks the terminal whether its background is dark, the same detection
the default palette uses).

**`[syntax]` is part of completeness.** A theme is offered only when it can be
applied **and** shown: the twenty-two canonical roles plus the two `[syntax]`
members the preview reads (`keyword_dark`, `string_dark`). A definition with
every terminal role but no syntax is reported as partial and is not offered,
because a row whose selection cannot repaint the interface would be half a
feature. The light member stays optional: a theme whose published palette ships
no light flavour leaves it empty and the preview falls back to the dark value,
rather than having a light value invented for it.

- **dotfiles** — `keyword_light #7a3e9e`, `keyword_dark #c99ad6`,
  `string_light #8a6a00`, `string_dark #dfbd76`: transcribed from the two pairs
  `styles.go` already carried. `styles.go` now reads them and
  `TestTheDefaultStylesMatchTheDotfilesDefinition` pins **both** members, so these
  are theme roles and the guard no longer has to name an exception.
- **Catppuccin Mocha and Catppuccin Latte** — the two flavours pair up, which is
  exactly the flavour pairing Catppuccin defines: `keyword_light #8839ef`
  (Latte mauve), `keyword_dark #cba6f7` (Mocha mauve), `string_light #40a02b`
  (Latte green), `string_dark #a6e3a1` (Mocha green). The choice follows
  Catppuccin's own convention, which paints keywords *mauve* and strings *green*;
  all four values are named palette colours of the two published flavours.
  Mocha used to carry no light member because Latte was not in the repository;
  the light member is Latte's value now, so the old readability limit is gone.
- **Kanagawa** — `keyword_dark #d27e99` (sakuraPink), `string_dark #c0a36e`
  (boatYellow2): the fish file's keyword and quote roles, the only Kanagawa
  syntax values this repository holds. No light member: this repository ships no
  Kanagawa light flavour.
- **Everforest** — `keyword_dark #d699b6` (purple), `string_dark #dbbc7f`
  (yellow): the fish file's keyword and quote roles. No light member.
- **Rosé Pine** — `keyword_dark #c4a7e7` (iris), `string_dark #f6c177` (gold):
  Rosé Pine's own syntax convention, both named palette colours. No light member.

## The six themes

| Theme | File | State | Source of its values |
|---|---|---|---|
| dotfiles | `dotfiles.toml` | **complete** | the repository's six hand-written blocks, verbatim |
| Catppuccin Mocha | `catppuccin-mocha.toml` | **complete** | `dotfiles-ghostty/themes/catppuccin-mocha.conf` + `starship.toml` `[palettes.catppuccin_mocha]` |
| Catppuccin Latte | `catppuccin-latte.toml` | **complete** | catppuccin/catppuccin, the published Latte palette and its terminal mapping |
| Kanagawa | `kanagawa.toml` | **complete** | rebelot/kanagawa.nvim, the "wave" palette and its terminal mapping; fish roles from the repository |
| Everforest | `everforest.toml` | **complete** | sainnhe/everforest, the dark-medium palette and its terminal mapping; fish roles from the repository |
| Rosé Pine | `rose-pine.toml` | **complete** | rose-pine/rose-pine, the published palette and its terminal mapping |

### Complete (offered)

- **dotfiles** covers **[Alacritty, Kitty, WezTerm, Ghostty, Starship, Herdr, the
  zsh line editor, the p10k prompt, bat, fish, tmux]** and leaves out **[Neovim]**:
  it names no colorscheme of its own.
- **Catppuccin Mocha** covers every tool whose colours this repository owns, the
  same eleven plus **Neovim**, whose `catppuccin` colorscheme the plugin it
  already ships provides, and leaves out **nothing**.
- **Catppuccin Latte** covers **[Alacritty, Kitty, WezTerm, Ghostty, Starship,
  Herdr, the zsh line editor, the p10k prompt]** and leaves out **[fish, bat,
  Neovim, tmux]**: Latte ships no `[bat]` file and no Neovim colorscheme of its
  own, and the repository's Neovim install hard-codes Mocha's flavour, so Neovim
  is named rather than pointed at a name that would not switch it.
- **Kanagawa** covers **[Alacritty, Kitty, WezTerm, Ghostty, Herdr, the zsh line
  editor, the p10k prompt, Neovim, fish]** and leaves out **[Starship, bat,
  tmux]**: it holds no `[prompt]` table and no `[bat]` name.
- **Everforest** covers **[Alacritty, Kitty, WezTerm, Ghostty, Herdr, the zsh line
  editor, the p10k prompt, fish]** and leaves out **[Starship, bat, Neovim,
  tmux]**: it holds no `[prompt]` table and no `[bat]` name.
- **Rosé Pine** covers **[Alacritty, Kitty, WezTerm, Ghostty, Herdr, the zsh line
  editor, the p10k prompt]** and leaves out **[Starship, fish, bat, Neovim,
  tmux]**: it holds no `[prompt]`, `[fish]` or `[bat]` table and no Neovim name.

Generated blocks: the four terminals, Herdr, Starship (both its `palette` line
and its `[palettes.<id>]` table), the zsh palette region including the `*_SGR`
twins and the `LS_COLORS`/`EZA_COLORS` tables, the p10k fallbacks, the `BAT_THEME`
selection, the fish theme files, the fish palette block in
`dotfiles-fish/fish/config.fish`, the tmux style block in
`dotfiles-tmux/tmux.conf`, and the bat `.tmTheme` files. Neovim's colorscheme
line is generated for the themes whose plugin ships one.

**fish is switched, through the file this repository owns.** Its theme files are produced from the
definitions, but which one is active is **the user's own state**: fish keeps the colour variables a
`fish_config theme choose` writes in `fish_variables`, which this repository does not own. Writing
it would be editing the user's settings, which the `preserve-user-configs` rule forbids. The switch
therefore paints fish the way it paints every other tool: it rewrites a marked block in
`dotfiles-fish/fish/config.fish`, a file the repository does own, and records the exact bytes so the
change is reversible. The block sets the fish colour variables in the **global** scope, which fish
returns over a universal one, so the palette is seen even after a `fish_config theme choose` the user
ran earlier. `fish_variables` is never written.

**The per-theme files that exist are the ones the repository already shipped.** A
new theme gets a generated block in the active files (the terminals, the prompt,
Herdr and bat's selection) when it is applied, and those are produced by the
generators rather than written by hand. A new theme gets a fish theme file or a
`.tmTheme` file only when the repository also ships that file for it; none of the
new themes does, so they name fish and bat as left out instead of claiming a file
that does not exist.

A complete theme is *offered*; the tools it cannot paint are *named*. The menu
must draw that list.

### Two tables, not one

The definition has a `[palette]` table (the terminal's 22 roles) and a `[prompt]`
table (Catppuccin's naming: `mauve`, `peach`, `subtext0`, `overlay0`, `maroon`,
`lavender`, `overlay2`, `overlay1`, `surface2`, ...). Two tables are more honest
than one: nine of the prompt roles are not terminal roles at all, and a single
table would invite filling them from a nearest ANSI colour. A prompt role is
filled only when the palette holds its value; a theme missing one is reported as
not switchable for the tool that needs it, never extrapolated.

### Findings recorded while generating

- **The zsh/p10k region is fully derivable.** Every `PALETTE_*` value, its
  `*_SGR` twin and every `LS_COLORS`/`EZA_COLORS` entry names one of the ten
  palette roles; no hand-written value outside the palette was found, so the
  region is generated whole rather than partly.
- **`bat` is generated and switched.** `dotfiles.tmTheme` and
  `catppuccin-mocha.tmTheme` are produced from the definitions, and the
  `BAT_THEME` selection in `.zshrc` is a generated region too. The installer
  copies every shipped `.tmTheme` and rebuilds bat's cache, so choosing a theme
  cannot land on an "Unknown theme". The syntax-to-role mapping is written down
  in the generator (`themeBatRoles`), because a scope name does not say which
  role it takes.
- **A generated bat file's own name is a role too.** The `.tmTheme`'s
  `<key>name</key>` is what `BAT_THEME` must match, so it is generated from the
  definition (`[bat] name`) rather than left as a second literal.
- **The deleted `[palettes.catppuccin_mocha]` table is recreatable.** Removing
  dead data is only legitimate while the generator can rebuild it:
  `TestTheRemovedStarshipPaletteIsRecreatable` renders
  `themes/catppuccin-mocha.toml`'s `[prompt]` table into
  `[palettes.catppuccin-mocha]` and pins all 26 values the deleted table held. If that guard ever
  fails, the deletion lost content and must be reverted.
- **`tmux` has a repository-owned theme now.** Its only theme used to be the `tmux-kanagawa`
  plugin's name, and Kanagawa was partial (0 of 22 roles), so nothing could be generated from it. tmux
  accepts colours of its own, though, so the switch paints tmux's own style options — status bar,
  active and inactive windows, panes, copy mode, display panes and the clock — from the definition,
  in a marked block in `dotfiles-tmux/tmux.conf`. No second palette is invented: every colour is a
  role the definition already holds. The block is placed **after** the
  `run '~/.tmux/plugins/tpm/tpm'` line, because measurement shows tmux runs `run-shell`
  synchronously: the server does not finish reading `tmux.conf` until the command returns, so the
  plugins TPM sources have already written their styles by the time the block is read, and the block
  wins. When the kanagawa plugin is installed, its window colours are therefore overridden by this
  block rather than by disabling the plugin.

## The origin of every transcribed palette

Each published palette below is a transcription, committed as a definition file.
Nothing is fetched at run time.

- **Catppuccin Latte** — catppuccin/catppuccin
  (<https://github.com/catppuccin/catppuccin>). The definition carries the
  published Latte roles (rosewater, flamingo, pink, mauve, red, maroon, peach,
  yellow, green, teal, sky, sapphire, blue, lavender, text, subtext0/1,
  overlay0/1/2, surface0/1/2, base, mantle, crust) and the project's own Latte
  terminal mapping (cursor rosewater, selection surface0, normal black subtext1,
  normal white surface2). Its `[prompt]` table is the Catppuccin role naming
  Starship uses, with Latte's values. Keywords take mauve and strings take green,
  Catppuccin's own syntax convention.
- **Kanagawa** — rebelot/kanagawa.nvim
  (<https://github.com/rebelot/kanagawa.nvim>). The definition carries the
  published "wave" palette: base sumiInk3, text fujiWhite, cursor oldWhite,
  selection waveBlue2, and the terminal colours 0–15
  (sumiInk0, autumnRed, autumnGreen, autumnYellow, crystalBlue, oniViolet,
  waveAqua1, oldWhite, fujiGray, samuraiRed, springGreen, carpYellow, springBlue,
  springViolet1, waveAqua2, fujiWhite). The fish roles are the repository's own;
  the plugin's other variant ("dragon", which the tmux plugin names) is left out
  so the terminal palette, the fish file and the nvim plugin agree on "wave".
- **Everforest** — sainnhe/everforest
  (<https://github.com/sainnhe/everforest>). The definition carries the published
  dark-medium palette: base bg0, text fg, cursor fg, selection bg3, and the
  terminal colours 0–15 (bg3, red, green, yellow, blue, purple, aqua, fg, grey0
  and the bright repeats). The fish roles are the repository's own; the hard and
  soft backgrounds are left out, so the definition is the medium dark variant the
  fish file already used.
- **Rosé Pine** — rose-pine/rose-pine (<https://rosepinetheme.com>,
  <https://github.com/rose-pine/rose-pine>). The definition carries the published
  palette (base, surface, overlay, muted, subtle, text, love, gold, rose, pine,
  foam, iris, highlight low/medium/high) and the project's terminal port (normal
  black overlay, cyan rose, selection highlight-medium). Keywords take iris and
  strings take gold, Rosé Pine's own syntax convention. **Rosé Pine Moon was
  considered and is not included**: its base (#232136) is lighter than Rosé
  Pine's (#191724), so it is not the "más oscuro" the user asked for, and the
  theme picker fits six complete themes at the 60×20 floor — a seventh
  overflows the frame guard.

### What is transcription and what is the repository's

- **Transcription from a published palette:** every `[palette]` role of
  `catppuccin-latte`, `kanagawa`, `everforest` and `rose-pine`, and the `[prompt]`
  table of `catppuccin-latte`. The value, its role name and the source are named
  in the file's header.
- **The repository's own:** the `dotfiles` palette (six hand-written blocks), the
  `catppuccin-mocha` palette (the ghostty conf and the starship table), all fish
  role tables, and the Neovim colorscheme names the repository's plugin install
  already provides.
- **Both:** the `[syntax]` tables that take a fish file's keyword/quote roles
  (kanagawa, everforest), and the Catppuccin pairing that takes one member from
  each of the two flavours.

## Kagawa was retired: it was never a theme

`themes/kagawa.toml` has been **removed**. The evidence:

- The repository's only Kagawa file, `dotfiles-fish/fish/themes/Kagawa.theme`,
  was introduced upstream as a byte-for-byte copy of `Kanagawa.theme` — the same
  values and the same `# name: Kanagawa Fish shell theme` header (verified
  against the upstream bootstrap commit).
- No published "Kagawa" palette exists to transcribe; the name is a copy of
  "Kanagawa", not a theme with its own colours.

Per the rule that a name without a theme is not offered, the definition is gone
and the theme list no longer holds it. The generated fish file
`dotfiles-fish/fish/themes/Kagawa.theme` is deleted in the same change, so the
repository no longer ships a Kagawa file for a theme that no longer exists.

## Two defects recorded here rather than hidden

- **~~`Kagawa.theme` is a copy of `Kanagawa.theme`.~~** Resolved by retiring
  Kagawa (above).
- **The hand-written fish files carried malformed colours.**
  `Kanagawa.theme` (and its copy `Kagawa.theme`) wrote
  `fish_color_selection --background=2D4FG67` (`G` is not a hex digit) and
  `fish_pager_color_description 727269`, where `727169` is meant. The generated
  `Kanagawa.theme` carries the corrected values now, because it is produced from
  `themes/kanagawa.toml` rather than hand-edited.

## What enforces this

The guards live in `installer/internal/tui/install_paths_test.go` and
`installer/internal/tui/update_test.go`:

- `TestThemeListIsDerivedFromDefinitions` — the menu's list is read from
  `themes/*.toml`; adding a file adds a row, and the two lists cannot disagree.
- `TestOnlyCompleteThemesAreOffered` — a theme missing a canonical role is
  partial and is never offered.
- `TestOnlyThemesThatCanBePreviewedAreOffered` — **new**: a definition with every
  terminal role but no `[syntax]` is not complete and is not offered, and every
  offered theme must preview. Teeth: dropping `[syntax]` from `Complete()`
  offers a theme the preview refuses.
- `TestNoInventedThemeRoleSlipsIn` — **new**: a palette role outside the
  canonical twenty-two and a `[syntax]` role outside the four the preview reads
  are both refused at load time, and every definition must cite a provenance.
- `TestEveryThemeReportsTheToolsItWouldLeaveOut` — every theme names the tools it
  cannot paint.
- `TestShippedThemeBlocksMatchTheirDefinition` — every value a definition claims
  must still be present in the shipped block it came from.
- `TestGeneratedThemeArtifactsMatchTheirDefinition` and
  `TestGeneratedPerThemeFilesMatchTheirDefinition` — the shipped generated files
  must be byte-for-byte what the generators produce. Run with
  `-update-theme-artifacts` to regenerate; running without that flag is what
  fails when somebody edits a generated block by hand.
- `TestThemeGeneratorUsesTheDefinition` and
  `TestThemeGeneratorRefusesAMissingRole` — the block a tool gets is its own
  theme's colours, and a definition missing a role is refused rather than emitted
  half-empty.
- `TestGeneratedThemeBlocksInventNoColour` — every colour a generated block emits
  must be one the definition already holds, in its `[palette]`, `[fish]` or
  `[prompt]` table. A renderer that fills a role by eye fails here.
- `TestTheFishDerivationMatchesTheDotfilesTable` — the code mapping that derives a
  fish palette from the canonical roles must still produce the values
  `themes/dotfiles.toml` records in its `[fish]` table, so a theme without one
  (Catppuccin Mocha) gets the same palette by derivation.
- `TestTmuxThemeBlockLoadsAfterPlugins` — the tmux block must sit after the TPM
  `run` line, which is what makes it win over the kanagawa plugin.
- `TestThePreviewPaintsTheThemesSyntaxRoles` — the preview reads the theme's
  `[syntax]` roles, and the Catppuccin light member is Latte's mauve (the flavour
  pairing), used on a light terminal rather than the dark value.
- `TestAPartialThemeCannotBePreviewed` — a definition with no canonical palette is
  refused by the preview; every shipped theme is complete now, so the guard also
  builds a partial fixture to keep its teeth.

The same parser the guards use (`loadThemeDefinitions`) is what the installer
reads at run time, so there is one definition and one reader.

Every generated file carries `# dotfiles-managed-config: <tool>` — the ownership marker the
`preserve-user-configs` rule reads — and a `>>> dotfiles-theme: <id> >>>` … `<<< dotfiles-theme <<<`
pair around the generated block, so a switch can find and replace exactly its own region.
