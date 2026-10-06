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

**A role is declared, or derived from the theme's own palette, or reported.** A
tool rarely reads the canonical roles: Starship's table is written in
Catppuccin's naming, fish has eighteen role names of its own, bat maps scopes to
roles, tmux paints its own style options and Neovim has highlight groups. A
definition that declares one of those tables (`[prompt]`, `[fish]`, `[bat]`,
`[nvim]`) is used as it is. A definition that does not has each role **derived
from its own canonical palette by the fixed mapping in "Derived roles" below** -
the same thing fish already did for Catppuccin Mocha - so a derived value is a
palette value the theme holds and nothing is chosen by eye. A role that is
neither declared nor derivable is **reported**: the theme leaves that tool on the
old palette and the row says which. A theme that misses the twenty-two canonical
roles or the `[syntax]` members the preview reads is `partial` and is **not**
offered as a switch at all.

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

## Derived roles: the fixed mapping, per tool

No tool reads the canonical roles, and not every definition carries the tool's own
table. Where a table is there (`[prompt]`, `[fish]`, `[bat]`, `[nvim]`) it is the
value. Where it is not, the value is **derived from the theme's own palette by
the mapping below**: a derived colour is always one the definition already holds,
which is what makes it the theme's colour rather than an invented one.
`TestGeneratedThemeBlocksInventNoColour` refuses any emitted colour the definition
does not hold, in a generated block or in a generated file, so the rule is
enforced rather than promised.

### fish - eighteen roles (`themeFishDerivation`)

| fish role | palette role | fish role | palette role |
|---|---|---|---|
| normal | text | comment | bright_black |
| command | green | selection | selection |
| keyword | magenta | search_match | selection |
| quote | yellow | operator | green |
| redirection | text | escape | magenta |
| end | cyan | autosuggestion | bright_black |
| error | red | pager_progress | bright_black |
| param | blue | pager_prefix | green |
| pager_completion | text | pager_description | bright_black |

`themes/dotfiles.toml` records the same mapping in its `[fish]` table and
`TestTheFishDerivationMatchesTheDotfilesTable` pins the equivalence, so the
derivation is the recorded mapping rather than a second opinion. Catppuccin Mocha,
Catppuccin Latte and Rosé Pine ship no `[fish]` table and take this one; Kanagawa
and Everforest ship their own, transcribed from the repository's fish files, and
take those.

### Starship's prompt roles - twenty-four (`themePromptDerivation`)

Starship's palette table is written in Catppuccin's role naming, and only five of
the roles below are terminal roles (text, red, green, yellow, blue): the rest
name surfaces, muted text and accents the canonical palette has no role for.

| prompt role | palette role | prompt role | palette role |
|---|---|---|---|
| text | text | subtext0 | bright_black |
| red | red | subtext1 | white |
| green | green | overlay0, overlay1, overlay2 | selection |
| yellow | yellow | surface0, surface1, surface2 | selection |
| blue | blue | rosewater | cursor |
| mauve | bright_blue | flamingo | bright_red |
| pink | magenta | maroon | red |
| teal | cyan | lavender | bright_magenta |
| peach | yellow | base, mantle, crust | base |

The prompt's own "muted" is `bright_black` and its "surface" is `selection`,
which is the same reading the installer's preview uses for the roles it falls
back on; `TestThePromptDerivationAgreesWithThePreview` pins the agreement, so the
prompt and the interface cannot call two different colours "muted". `sky` and
`sapphire` are optional (Catppuccin's own extras): the two Catppuccin definitions
carry them and the generated table emits them, and a theme without them is not
reported as missing Starship.

### bat - nine scopes to roles (`themeBatRoles`)

| `.tmTheme` token | palette role | what it paints |
|---|---|---|
| muted | bright_black | comments, punctuation, invisible characters |
| yellow | yellow | strings |
| magenta | magenta | numbers, language constants, preprocessor directives |
| blue | blue | keywords, storage, types, tags, headings |
| cyan | cyan | operators, escapes, attribute names, class names |
| green | green | function names |
| red | red | variables, invalid syntax, deletions |
| text | text | everything else |
| selection | selection | the selection and line-highlight backgrounds |

`[bat] name` is the name the `.tmTheme` carries inside itself and `[bat] file` is
its file name. **bat selects a custom theme by the file name, not by the name
inside it** (measured below), so `BAT_THEME` is the file's stem, and the file
check in the generated block is what keeps a machine where the theme is not
installed from selecting a theme it does not have.

### tmux - every style option takes a palette role (`renderTmuxTheme`)

| tmux option | roles |
|---|---|
| status-style | text on base |
| status-left-style | base on blue, bold |
| status-right-style | bright_black on base |
| message-style, message-command-style | base on yellow |
| window-status-style | bright_black on base |
| window-status-current-style | base on blue, bold |
| window-status-activity-style | yellow on base |
| window-status-bell-style | red on base |
| window-status-last-style | green on base |
| pane-border-style | bright_black |
| pane-active-border-style | blue |
| copy-mode-match-style | text on selection |
| copy-mode-current-match-style | base on yellow |
| mode-style | text on selection |
| display-panes-colour | bright_black |
| display-panes-active-colour | blue |
| clock-mode-colour | blue |

### Neovim - highlight groups to palette roles (`themeNvimGroups`)

The generated colorscheme paints 96 groups. Which palette role each group's
foreground and background takes is the table in
`installer/internal/tui/installer.go`, and the generated file lists every
group with its value. Grouped by role, the mapping is this:

| palette role | what takes it |
|---|---|
| text | body text (Normal, NormalNC, NormalFloat, MsgArea, Identifier), the focused StatusLine and tab, Pmenu's text |
| base | the background of the chrome, floating borders, EndOfBuffer, the unfocused status line and tab line, the diff backgrounds |
| bright_black | Comment and SpecialComment (italic), line numbers, SignColumn, FoldColumn, NonText, SpecialKey, Whitespace, Ignore, PmenuThumb, DiagnosticHint |
| selection | CursorLine, CursorColumn, ColorColumn, Visual, Folded, MatchParen's background, Pmenu's background, the tab-line selection, QuickFixLine, PmenuSbar, the LspReference backgrounds |
| cursor / cursor_text | Cursor, lCursor, TermCursor |
| blue | Statement, Conditional, Repeat, Label, Keyword, Type, StorageClass, Structure, Typedef, Underlined, Directory, Title, FloatTitle, PmenuSel's background, DiffText |
| yellow | String, Character, Search, CursorLineNr, WarningMsg, Todo's background, DiffChange, Changed, DiagnosticWarn |
| magenta | Constant, Number, Boolean, Float, PreProc, Include, Define, Macro, PreCondit |
| cyan | Operator, Special, SpecialChar, Delimiter, MatchParen's foreground, DiagnosticInfo |
| green | Function, Tag, DiffAdd, IncSearch, CurSearch, MoreMsg, Question, Added, DiagnosticOk |
| red | Error, ErrorMsg, Exception, Debug, DiffDelete, Removed, DiagnosticError |
| the ANSI roles | the sixteen `g:terminal_color_N` values, in canonical order from black to bright_white |

The one derived non-colour is Neovim's own `background`: the colorscheme declares
`light` when the theme's base is lighter than mid grey and `dark` otherwise, which
is what makes Catppuccin Latte a light colorscheme and the other five dark ones.

## The six themes

| Theme | File | State | Source of its values |
|---|---|---|---|
| dotfiles | `dotfiles.toml` | **complete** | the repository's six hand-written blocks, verbatim |
| Catppuccin Mocha | `catppuccin-mocha.toml` | **complete** | `dotfiles-ghostty/themes/catppuccin-mocha.conf` + `starship.toml` `[palettes.catppuccin_mocha]` |
| Catppuccin Latte | `catppuccin-latte.toml` | **complete** | catppuccin/catppuccin, the published Latte palette and its terminal mapping |
| Kanagawa | `kanagawa.toml` | **complete** | rebelot/kanagawa.nvim, the "wave" palette and its terminal mapping; fish roles from the repository |
| Everforest | `everforest.toml` | **complete** | sainnhe/everforest, the dark-medium palette and its terminal mapping; fish roles from the repository |
| Rosé Pine | `rose-pine.toml` | **complete** | rose-pine/rose-pine, the published palette and its terminal mapping |

### Complete (offered): every theme paints every tool

| Theme | Leaves out |
|---|---|
| dotfiles | **nothing** |
| Catppuccin Mocha | **nothing** |
| Catppuccin Latte | **nothing** |
| Kanagawa | **nothing** |
| Everforest | **nothing** |
| Rosé Pine | **nothing** |

The twelve tools are Alacritty, Kitty, WezTerm, Ghostty, Starship, the zsh line
editor, the p10k prompt, Herdr, fish, bat, Neovim and tmux; they are `themeTools`
in `installer/internal/tui/installer.go`. Coverage is the generator's own answer -
fish asks whether it can derive the fish roles at all, bat whether it can render
the `.tmTheme`, tmux whether it can render its style block, Neovim whether the
name it selects resolves - so a theme can only leave a tool out for something it
is missing, and `TestEveryOfferedThemePaintsEveryTool` fails with the theme's own
name and the tool it stopped painting when one loses it.

Which roles are **declared** in the definition and which are **derived** by the
mappings above:

| Theme | Starship | fish | bat | tmux | Neovim |
|---|---|---|---|---|---|
| dotfiles | declared `[prompt]` | declared `[fish]` | declared `[bat]` | derived | generated colorscheme |
| Catppuccin Mocha | declared `[prompt]` | derived | declared `[bat]` | derived | the `catppuccin` plugin |
| Catppuccin Latte | declared `[prompt]` | derived | declared `[bat]` | derived | generated colorscheme |
| Kanagawa | derived | declared `[fish]` | declared `[bat]` | derived | the `kanagawa` plugin |
| Everforest | derived | declared `[fish]` | declared `[bat]` | derived | generated colorscheme |
| Rosé Pine | derived | derived | declared `[bat]` | derived | generated colorscheme |

Generated blocks: the four terminals, Herdr, Starship (both its `palette` line
and its `[palettes.<id>]` table), the zsh palette region including the `*_SGR`
twins and the `LS_COLORS`/`EZA_COLORS` tables, the p10k fallbacks, the `BAT_THEME`
selection, the fish palette block in `dotfiles-fish/fish/config.fish`, the tmux
style block in `dotfiles-tmux/tmux.conf`, the Neovim colorscheme line, the fish
theme files, the bat `.tmTheme` files and the Neovim colorschemes. Which of those
are files the repository ships and which are written by the installer is under
"The per-theme files" below.

**fish is switched, through the file this repository owns.** Its theme files are produced from the
definitions, but which one is active is **the user's own state**: fish keeps the colour variables a
`fish_config theme choose` writes in `fish_variables`, which this repository does not own. Writing
it would be editing the user's settings, which the `preserve-user-configs` rule forbids. The switch
therefore paints fish the way it paints every other tool: it rewrites a marked block in
`dotfiles-fish/fish/config.fish`, a file the repository does own, and records the exact bytes so the
change is reversible. The block sets the fish colour variables in the **global** scope, which fish
returns over a universal one, so the palette is seen even after a `fish_config theme choose` the user
ran earlier. `fish_variables` is never written.

### The per-theme files: shipped, or generated by the installer

There are three per-theme kinds of file, and each is generated from the same
definition:

- **The fish theme files** (`dotfiles-fish/fish/themes/*.theme`) are shipped for
  the themes the repository already had one for, and the whole set is pinned
  byte-for-byte by `TestGeneratedPerThemeFilesMatchTheirDefinition`. They are not
  how the switch paints fish: fish is painted through the marked block in
  `dotfiles-fish/fish/config.fish`, which every theme has, which is why
  Catppuccin Mocha paints fish while shipping no fish theme file.
- **The bat `.tmTheme` files** are two under version control, and one for every
  theme that names a bat theme is generated at install time
  (`generateBatThemesFromDefinitions`, called by the shell step) into bat's own
  themes directory, before the cache rebuild that makes it selectable. bat
  selects a theme by a name that directory has to hold, so a theme whose file is
  missing would leave bat on "Catppuccin Mocha" however the row read.
- **The Neovim colorschemes** (`dotfiles-nvim/nvim/colors/*.lua`) are generated
  for the themes whose plugin install ships no colorscheme for them: `dotfiles`,
  `catppuccin-latte`, `everforest` and `rose-pine`. Catppuccin Mocha and Kanagawa
  keep the names their plugins already provide. The definition's `[nvim] name` is
  the file's own name, which is what `:colorscheme` resolves and what
  `TestTheNeovimColorschemeNamesResolve` checks.

Every one of them is byte-for-byte what its definition produces, pinned by the
same guard: a hand edit fails rather than living on as a second palette. Neovim's
colorscheme declares `light` or `dark` from its own base, so Catppuccin Latte is a
light colorscheme rather than a dark one with light colours.

A complete theme is *offered*; a tool it cannot paint is *named*. For every theme
in the library that list is empty now, and the guard is what holds it there.

### Two tables, not one

The definition has a `[palette]` table (the terminal's 22 roles) and a `[prompt]`
table (Catppuccin's naming: `mauve`, `peach`, `subtext0`, `overlay0`, `maroon`,
`lavender`, `overlay2`, `overlay1`, `surface2`, ...). Two tables are more honest
than one: most of the prompt roles are not terminal roles at all, and a single
table would invite filling them from a nearest ANSI colour. A prompt role a
definition declares is used as it is; one it does not declare is derived from its
palette by the mapping above, never extrapolated by eye, and
`TestThePromptDerivationCoversEveryRequiredRole` refuses a derivation that would
leave one of the twenty-four empty.

### Findings recorded while generating

- **The zsh/p10k region is fully derivable.** Every `PALETTE_*` value, its
  `*_SGR` twin and every `LS_COLORS`/`EZA_COLORS` entry names one of the ten
  palette roles; no hand-written value outside the palette was found, so the
  region is generated whole rather than partly.
- **`bat` is generated and switched.** `dotfiles.tmTheme` and
  `catppuccin-mocha.tmTheme` are produced from the definitions, and the
  `BAT_THEME` selection in `.zshrc` is a generated region too. The syntax-to-role
  mapping is written down in the generator (`themeBatRoles`), because a scope name
  does not say which role it takes.
- **`bat` is generated for every theme at install time, not only shipped.** The
  completion rule needs the bat half to be real: bat selects a theme by a name
  `~/.config/bat/themes/` has to hold, and only two `.tmTheme` files are under
  version control, so a switch to one of the other four would have landed on
  "Unknown theme" and fallen back to Catppuccin Mocha. The shell step now writes
  a `.tmTheme` for **every** theme that names one from the same definition the
  switch reads (`generateBatThemesFromDefinitions`), before the cache rebuild, and
  `TestTheBatThemesTheSwitchOffersAreGeneratedAtInstallTime` drives that helper
  and checks the file, its name and the name inside it. The two committed files
  are still written by it, with the bytes the guard pins, so the installed set is
  the definitions' set rather than "the shipped files plus whatever was added
  last".
- **bat selects a custom theme by its file name, and the selection used to export
  the wrong one.** `[bat] name` is the `.tmTheme`'s own `<key>name</key>` and it is
  generated from the definition rather than left as a second literal, but it is
  **not** the selection key: bat registers a theme directory's `.tmTheme` under its
  file stem. Measured with bat 0.26.1 - a cache built from `dotfiles-bat/themes/`
  lists `catppuccin-mocha` beside its own bundled `Catppuccin Mocha`,
  `BAT_THEME=catppuccin-mocha` paints the repository's blue keyword (`#89b4fa`),
  and `BAT_THEME="Catppuccin Mocha"` paints the bundled theme's mauve
  (`#cba6f7`). The switch exported `[bat] name`, so five of the six themes named a
  theme bat had not registered: only `dotfiles` worked, because its file stem
  happens to equal its name. `BAT_THEME` is the file's stem now
  (`batSelectionName`), and `TestTheBatSelectionNamesTheFileBatRegisters` refuses a
  theme whose selection is not its file's name, or two themes whose files share
  one.
- **Neovim's colorscheme is generated for the themes whose plugin ships none.**
  `dotfiles-nvim/nvim/colors/{dotfiles,catppuccin-latte,everforest,rose-pine}.lua`
  are produced from the definitions by `renderNvimTheme`, and `[nvim] name` is
  each file's own name, so `:colorscheme` resolves to a file this repository
  ships and `TestTheNeovimColorschemeNamesResolve` checks that a name never
  resolves to nothing. Catppuccin Mocha keeps the `catppuccin` plugin's name and
  Kanagawa keeps `kanagawa`'s. **Latte is generated rather than pointed at the
  plugin's flavour**: the repository's Neovim configuration pins Catppuccin's
  `flavour = "mocha"`, so the plugin's own name would paint Mocha's palette under
  a Latte name, and the generated file is the one whose contents are checkable
  here. The four files parse and execute (measured with `luac -p` and by running
  each against a stubbed `vim`, 96 groups, 16 terminal colours each) and a real
  Neovim loads every one of them: `nvim --headless -u NONE --cmd 'set
  rtp+=dotfiles-nvim/nvim' -c 'colorscheme <id>'` reports the theme's own
  `g:colors_name`, background and `Normal`/`String`/`Comment` values, Catppuccin
  Latte's being light and the others dark.
- **Neovim's `background` is the only derived non-colour.** The generated
  colorscheme declares `light` when the theme's base is lighter than mid grey and
  `dark` otherwise (`themeBaseIsLight`), which is a rule about a value the
  definition holds. Catppuccin Latte is the one light theme,
  `TestTheGeneratedColorschemeDeclaresTheBackgroundItsBaseImplies` requires both
  answers to exist, and a constant would fail it.
- **The prompt roles are derived, and agree with the preview.** Starship's table
  is written in Catppuccin's naming and nineteen of its roles have no terminal
  equivalent, so a theme with no `[prompt]` table (Kanagawa, Everforest, Rosé
  Pine) derives each one from its palette by `themePromptDerivation`. The three
  roles the installer's own preview already fell back on (subtext0 ->
  bright_black, mauve -> bright_blue, peach -> yellow) read the same way,
  `TestThePromptDerivationAgreesWithThePreview` pins that, and
  `TestThePromptDerivationCoversEveryRequiredRole` refuses a derivation that
  would leave one of the twenty-four empty. The values each theme derives are
  logged by that guard, so the provenance can be audited from the test output.
- **There is no per-theme artifact table any more.** `themeToolArtifacts` used to
  record which tools each theme had a generator for, and a tool missing from it
  was reported as left out. Completion removed the need for the table: the
  definitions are that list now, and `themeCoverage` asks each tool's own
  generator (fish's role derivation, bat's renderer, tmux's renderer, Neovim's
  name resolution). A theme that loses what a tool needs is named by the guard
  with the theme and the tool, and a tool the menu names can no longer have
  nothing behind it: `TestEveryCoveredToolCanBeGeneratedForEveryOfferedTheme`
  walks the switch's own tool list and requires a rendered block for each.
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
- **Derived, from the theme's own palette:** every role a definition does not
  declare - fish's eighteen, Starship's twenty-four, bat's nine tokens, tmux's
  style options and Neovim's highlight groups - by the fixed mapping in "Derived
  roles" above. No value is invented: a derived value is one the definition
  already holds.

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

## Defects recorded here rather than hidden

- **~~`Kagawa.theme` is a copy of `Kanagawa.theme`.~~** Resolved by retiring
  Kagawa (above).
- **The `BAT_THEME` selection named the wrong key.** bat registers a custom
  `.tmTheme` under its **file name**, and the switch exported `[bat] name` (the
  `<key>name</key>` inside the file). Five of the six themes therefore selected a
  theme bat had not registered: `Catppuccin Mocha` and `Catppuccin Latte` silently
  selected bat's own bundled themes of those names, and the other three selected
  nothing this repository shipped. Found by measuring with bat 0.26.1 while
  closing the coverage matrix, fixed in `renderBatSelection`/`batSelectionName`,
  and pinned by `TestTheBatSelectionNamesTheFileBatRegisters`. No committed file
  moved: the default theme's name and its file name are the same string.
- **~~The `.tmTheme`'s own comment is not valid XML.~~** Resolved. The generated
  file's header comment told a reader to run `bat cache --build`, and an XML
  comment may not contain `--`: `xml.etree` refused both committed `.tmTheme`
  files on that line, and the four generated ones inherited it. bat 0.26.1
  accepted them (measured: the theme is listed and paints), so it was a strictness
  defect rather than a broken file. Fixed in `themeBatTemplate`: the comment names
  `bat cache` and its build flag without the `--` sequence, the two committed
  files were regenerated from the template with `-update-theme-artifacts`, and
  `TestGeneratedBatThemesAreValidXML` refuses a later edit that puts a `--` back
  inside a comment.
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
- `TestEveryOfferedThemePaintsEveryTool` — **the requirement as a guard**: for
  every theme the menu offers, `themeCoverage` leaves no tool out, and it fails
  with the theme's own name and the tool it stopped painting. It also refuses a
  row that still prints an exclusion for a theme that has none. Teeth: removing a
  `[bat]` table (or an `[nvim]` name) makes it fail naming that theme and that
  tool.
- `TestThePromptDerivationCoversEveryRequiredRole` — every prompt role Starship
  needs names a canonical palette role the definitions hold, no offered theme is
  left without a value, and the values each theme derives are logged for audit.
- `TestThePromptDerivationAgreesWithThePreview` — the three prompt roles the
  installer's preview falls back on read the same palette roles the Starship
  derivation does, so the prompt and the interface cannot disagree about "muted".
- `TestTheNeovimColorschemeNamesResolve` — every offered theme's `[nvim]` name is
  either a colorscheme the repository's plugin install provides or a generated
  file this repository ships under `dotfiles-nvim/nvim/colors/`, the file's own
  name is the selected name, and the file is inside the directory the Neovim step
  installs.
- `TestTheGeneratedColorschemeDeclaresTheBackgroundItsBaseImplies` — the derived
  `background` follows the base's luminance, and the guard requires at least one
  light and one dark answer, so a constant fails it.
- `TestTheBatThemesTheSwitchOffersAreGeneratedAtInstallTime` — the helper the
  shell step calls writes a `.tmTheme` for every theme that names one, with the
  name the switch exports as `BAT_THEME` inside it, and the guard requires some of
  them not to be under version control.
- `TestTheBatSelectionNamesTheFileBatRegisters` — `BAT_THEME` is the file's own
  name, which is the name bat registers a custom `.tmTheme` under, and no two
  themes share one.
- `TestGeneratedBatThemesAreValidXML` — every `.tmTheme` the generator emits (the
  two committed ones and the four the installer writes at run time) is handed to
  `encoding/xml`, so a comment bat tolerates but XML forbids (a body containing
  `--`) fails the guard rather than living on as a strictness defect.
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
- `TestGeneratedThemeBlocksInventNoColour` — every colour a generated **block or
  per-theme file** emits must be one the definition already holds, in its
  `[palette]`, `[fish]` or `[prompt]` table. A renderer that fills a role by eye
  fails here. Its colour token requires a whole word, so bat's `embedded` scope
  name ("bedded" is spelt with hex digits) is not read as a colour.
- `TestTheFishDerivationMatchesTheDotfilesTable` — the code mapping that derives a
  fish palette from the canonical roles must still produce the values
  `themes/dotfiles.toml` records in its `[fish]` table, so a theme without one
  (Catppuccin Mocha) gets the same palette by derivation.
- `TestEveryCoveredToolCanBeGeneratedForEveryOfferedTheme` — walks the switch's own
  tool list and requires a rendered block for every tool a theme covers, so a tool
  the menu names cannot have nothing behind it, and a definition missing a role a
  generator reads is caught before the row is pressed.
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
