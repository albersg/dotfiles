# Unified themes across every dotfiles part, switchable and previewed live

The user's ask, in their words: *"review this whole theme area, so that every part (or as many as are capable)
has a theme, that the theme is **unified across all of them**, and that a tool whose theme can be changed but is
not included - Herdr, for example - **is included**, so every dotfiles part in the repository has a unified
theme; and from the dotfiles, be able to change it and **SEE the changes in real time**."*

## What the repository holds today (verified by reading, not assumed)

**One palette, written by hand in five places.** The background `#06080f` and its companions (`#f3f6f9`
foreground, `#e0c15a` cursor, `#263356` selection, `#7fb4ca` blue) appear inside a boxed `DOTFILES THEME` block
in each of:

| File | Mechanism |
|---|---|
| `alacritty.toml` | `[colors.*]` hex blocks |
| `.wezterm.lua` | Lua colour tables |
| `dotfiles-kitty/kitty.conf` | flat `name #hex` keys |
| `starship.toml` | prompt style colours |
| `dotfiles-zsh/.p10k.zsh` | `typeset -g` colour variables |

**Named theme files exist for three tools:**

- `dotfiles-fish/fish/themes/`: **Everforest**, **Kagawa**, **Kanagawa** (three files).
- `dotfiles-ghostty/themes/`: `catppuccin-mocha.conf`.
- `dotfiles-bat/themes/`: `dotfiles.tmTheme`, this repository's own.

**Tools that name a theme inside their own config:** `dotfiles-tmux/tmux.conf` (`@kanagawa-theme 'dragon'`),
`dotfiles-nvim/nvim/lua/plugins/colorscheme.lua` (`colorscheme = "kanagawa"`, with `wave`/`lotus` bound to the
background option), `dotfiles-herdr/config.toml` (`[theme]` plus `[theme.custom]`).

**The defect this exposes:** the same palette is maintained by hand in five files and partly duplicated in the
theme files above, so *the repository cannot switch its own theme today* - and any drift between those five
copies is invisible until someone looks at two terminals side by side. **That duplication is the thing to fix,
not the thing to work around.**

## Contracts this work must respect

- **One source of truth.** A theme is defined once; the per-tool blocks and files are **generated** from it. A
  sixth hand-written copy is the failure mode, and a guard must fail when a generated block drifts from its
  definition.
- **Only our files.** Switching edits files this repository ships - the `DOTFILES THEME` blocks, the theme-name
  lines, the theme files. It must never touch a file the user wrote: the `preserve-user-configs` rule applies (a
  marked file is ours; an unmarked one is preserved and named in the log).
- **Reversible, always.** It records what it replaced and can put it back - extending
  `$XDG_STATE_HOME/dotfiles/theme.json` rather than inventing a second record.
- **The list is derived, never typed.** Adding a theme must mean adding a definition; a test must derive the list
  from the repository and fail when the menu and the definitions disagree.
- **Honest degradation.** A tool that cannot take the theme says so; it is never offered as a button that does
  nothing.
- **`--dry-run` skips it** (the gate PR #134 established).
- **NO GOLDEN MAY MOVE** without its changed rows pasted and explained.

## Tasks

| # | Task | Notes |
|---|---|---|
| 1 | **The inventory**: every themeable surface in the repository - tool, mechanism, what it takes today, whether a second theme exists, and what would have to be generated | read-only; the matrix comes before any code |
| 2 | **One definition, generated blocks**: the palette defined once (theme names include the ones the repository already ships: **dotfiles, Everforest, Kagawa, Kanagawa, Catppuccin Mocha**), with the five hand-written copies and the theme files generated from it | this is what makes a switch possible at all |
| 3 | **Every part that can take a theme gets one** - including **Herdr**, and any other part the inventory finds that names a theme or takes colours | the user's explicit addition |
| 4 | **The switch in the installer**, from the utilities section: apply a theme across every part, reversibly | extends the utilities section (#153/#154) |
| 5 | **Live preview**: while the cursor moves over a theme, the installer shows **that theme's colours in the interface itself** - the preview is the real palette, not a description of it, and it changes as the selection changes without applying anything | the user's explicit ask: *"see the changes in real time"* |
| 6 | **Guards**: the list is derived from the definitions; a generated block that drifts from its definition fails; the switch is reversible; `--dry-run` skips it; the preview renders the same values the apply would write | a guard that can see the class |

## Open question, stated because it changes task 5's design

A live preview can mean two different things, and they cost differently: **(a)** the installer restyles **its own
interface** in the theme's colours as the cursor moves (self-contained, no filesystem writes, honest about being
a preview), or **(b)** the installer **applies each theme to disk** as the cursor moves so the terminal really
changes, and reverts on exit (a real preview, but it writes files on every keypress and can be interrupted
mid-way). **(a) is the safe default and is what task 5 builds** unless the user says otherwise; **(b)** is only
acceptable with a guaranteed restore path, and is recorded here as the alternative it was chosen over.

---

## Findings from the implementation (recorded here as defects found)

These three were found while building the definitions. They are recorded as defects even though
fixing two of them falls outside this change.

1. **`dotfiles-zsh/.zshrc` is a SIXTH hand-written copy of the palette.** The task counted five
   files; the prompt's `PALETTE_*` values and their `*_SGR` twins, plus the derived `LS_COLORS` and
   `EZA_COLORS`, live in `.zshrc`, and `.p10k.zsh` carries the same ten as fallbacks. The palette is
   therefore written by hand in six files, and `.zshrc` also duplicates each value in a second
   (SGR) form. Not generated yet.
2. **`dotfiles-ghostty/themes/catppuccin-mocha.conf` is an unreferenced asset.** `dotfiles-ghostty/config`
   paints the hand-written `DOTFILES THEME` block and never selects the file, so the shipped Catppuccin
   theme has been dead since it landed. It is now the provenance of the `catppuccin-mocha` definition,
   so it has a purpose even before the switch can select it.
3. **`dotfiles-bat/themes/dotfiles.tmTheme` is the only bat theme, and `.zshrc`'s fallback names a
   theme that does not exist here.** `BAT_THEME` falls back to `"Catppuccin Mocha"`, for which the
   repository ships no `.tmTheme`. Generated bat themes are not implemented yet.
4. **`Kagawa.theme` is a copy of `Kanagawa.theme`** (same values, its own header reads
   `# name: Kanagawa Fish shell theme`), and both carry the invalid hex
   `fish_color_selection --background=2D4FG67` and `fish_pager_color_description 727269`. Kagawa has
   no distinct palette. Generation to fix these is not implemented yet.

## Progress

| # | Task | State |
|---|---|---|
| 1 | The inventory | done (in the report; the tool table is reproduced in `themes/README.md`) |
| 2 | One definition + generated blocks | **definitions done** (`themes/*.toml`, `themes/README.md`); **generation done for the four terminals** (Alacritty, Kitty, WezTerm, Ghostty) with a byte-for-byte guard and `-update-theme-artifacts`; generation for Starship, the zsh/p10k prompt, Herdr, fish, bat, Neovim and tmux is **not done** |
| 3 | Every part that can take a theme gets one - including Herdr | **not done** (Herdr is declared in the definitions and reported as not switchable yet) |
| 4 | The switch in the utilities section | **switch mechanism done and tested** (apply/undo, byte-for-byte record in the same `theme.json`, ownership-marker refusal, restore-on-failure, `--dry-run` gate); the utilities **screen row is not wired** |
| 5 | Live preview | **not done** |
| 6 | Guards | **done for what exists**: list derived from definitions, partial never offered, tools-left-out reported, shipped blocks match definitions, generated blocks byte-for-byte, generator uses the definition, missing role refused, switch reversible, dry-run gate, unowned file refused |

## Update: Herdr and fish generated (A, part 2)

- **Herdr** is generated and switched in place: `[theme.custom]` (`panel_bg`, `selection_bg`) and the
  `[ui] accent` are rendered from `themes/<id>.toml`, with ownership and generator markers. `[theme]
  name = "vesper"` is **kept**: `[theme.custom]` is an override layer, and the tokens the palette owns
  are the ones generated; the remaining Vesper tokens are not palette values and are left alone.
- **fish** theme files are generated from `[fish]` tables in the definitions: `dotfiles.theme`,
  `Everforest.theme`, `Kanagawa.theme` and `Kagawa.theme` (repository capitalisation kept).
  `Kagawa.theme` now carries its own `Kagawa` header and the invalid hex `2D4FG67` and the wrong
  `727269` are gone; its palette is still Kanagawa's, recorded as such.
- Exclusion list shrank by one for both offered themes: they now cover
  **[Alacritty, Kitty, WezTerm, Ghostty, Herdr]** and leave out [Starship, zsh, p10k, fish, bat,
  Neovim, tmux].
- **New finding (reported, not invented):** the Starship `[palettes.dotfiles]` table holds nine values
  the canonical palette does not contain (`mauve #A3B5D6`, `peach #DEBA87`, `subtext0 #5C6170`,
  `overlay0 #232A40`, `maroon #C4746E`, `lavender #B99BF2`, `overlay2 #313342`, `overlay1 #191E28`,
  `surface2 #27345C`). Generating it needs the definition extended with a prompt-role table.
- **Still to do:** Starship, the zsh/p10k block (derived `*_SGR` + `LS_COLORS`/`EZA_COLORS`), bat
  `.tmTheme` files, the Neovim/tmux name lines, then B (the utilities row) and C (live preview).

## Update: Starship, zsh/p10k and Neovim generated (A, part 3)

- **`[prompt]` table added** to `themes/*.toml`. The nine prompt-only roles (mauve, peach, subtext0,
  overlay0, maroon, lavender, overlay2, overlay1, surface2) are declared from the values the repository
  already holds in `starship.toml`; a role the repository lacks is left empty and the tool is reported,
  never extrapolated. Two tables (ANSI and prompt) are deliberately more honest than one.
- **Starship** generated: the `palette = "<id>"` line and the `[palettes.<id>]` table, both marked.
  The unused hand-written `[palettes.catppuccin_mocha]` table was replaced by the generated one.
- **zsh** generated whole: `PALETTE_*`, the `*_SGR` twins, `PALETTE_ESC`, and the `LS_COLORS` and
  `EZA_COLORS` tables. Every entry names a palette role, so the region is fully derivable; no
  hand-written value outside the palette was found. (Finding: the region is byte-reproducible.)
- **p10k** generated: the ten `${VAR:-default}` fallbacks.
- **Neovim** generated where the plugin ships a colorscheme: Catppuccin names `catppuccin`, Kanagawa
  names `kanagawa`. Dotfiles names none, so Neovim is honestly left out for dotfiles.
- Exclusion list shrank again:
  **dotfiles** covers [Alacritty Kitty WezTerm Ghostty Starship zsh p10k Herdr], leaves out
  [fish bat Neovim tmux];
  **catppuccin-mocha** covers the same eight plus Neovim, leaves out [fish bat tmux].
- **Still to do in A:** bat (`.tmTheme`, derivable: only palette colours, ~330-line template) and tmux
  (only the kanagawa plugin names a theme; no theme invented for dotfiles/Catppuccin). Then B (the
  utilities row + the "unavailable until the repository is cloned" state + re-measured frame guards)
  and C (live preview).

## Update: bat generated, tmux confirmed reported (A complete)

- **bat** is generated: `dotfiles.tmTheme` (byte-identical to the hand-written one except a trailing
  newline) and the new `catppuccin-mocha.tmTheme`, both from the definitions. The syntax-to-role
  mapping is written down in the generator (`themeBatRoles`). No colour outside the palette was found.
  The theme *selection* (`BAT_THEME` in `.zshrc`) is not an edit surface of this change, so bat stays
  **reported, not claimed**.
- **tmux** is confirmed reported: the only tmux theme is the `tmux-kanagawa` plugin's name, and
  Kanagawa is partial, so nothing was generated and no theme was invented.
- **Deletion sanctioned with a condition:** the unused `[palettes.catppuccin_mocha]` table was removed
  from `starship.toml`, and `TestTheRemovedStarshipPaletteIsRecreatable` pins that the generator
  rebuilds it from `themes/catppuccin-mocha.toml` with all 26 of its values.
- **A is complete.** Final exclusion lists:
  dotfiles covers [Alacritty Kitty WezTerm Ghostty Starship zsh p10k Herdr], leaves out
  [fish bat Neovim tmux];
  catppuccin-mocha covers the same eight plus Neovim, leaves out [fish bat tmux].
  (Started at eight excluded; now four for dotfiles and three for catppuccin.)
- **Not started: B** (the utilities row, the "unavailable until the repository is cloned" state, the
  exclusion list drawn on the screen, the re-measured frame guards) **and C** (the live preview).

## Update: bat made switchable, and B (the utilities row) opened

- **bat is now genuinely switchable.** `BAT_THEME` in `.zshrc` is a generated region (a named block),
  the `.tmTheme`'s own `<key>name</key>` is generated too (it was hard-coded `dotfiles`, so the first
  generated catppuccin file named itself dotfiles), and the install step copies every shipped
  `.tmTheme` and rebuilds bat's cache. bat left the exclusion list.
- **fish stays out on purpose**: its active theme is the user's `fish_config` state in
  `fish_variables`, which this repository does not own. Recorded in `themes/README.md` as a decision,
  not an omission.
- **tmux stays out**: no repository-owned theme (only the plugin name, and Kanagawa is partial).
- **B opened**: the utilities section now draws one row per complete theme, each naming the tools it
  leaves out (`Apply the dotfiles theme (not fish, Neovim, tmux)`), a **Undo the last dotfiles theme
  change** row when a record exists, and the honest state *"The dotfiles' own theme is not switchable
  here: the repository has not been cloned yet."* before a checkout. The definitions are read from
  `m.RepoDir` when the section is first opened.
- Final exclusion lists: dotfiles leaves out [fish, Neovim, tmux]; catppuccin-mocha leaves out
  [fish, tmux]. (Started at eight.)
- **C (the live preview) is not started.**

## Update: C (the live preview) done

- While the cursor is on a theme row, the utilities section paints that theme's **real palette**: a
  background swatch per role and its own title in the theme's accent, built from **the same definition
  the apply writes**. Zero writes while the cursor moves; nothing is applied.
- `TestTheThemePreviewShowsTheValuesTheApplyWouldWrite` reads the painted SGR sequences back and checks
  each swatch against the definition's value (allowing lipgloss's one-step 8-bit rounding), then checks
  every painted value is one the apply block writes. `TestEveryScreenFitsEveryTerminalSize` and the
  utilities frame guards measure the preview as a row.
- Teeth: a preview painted with a constant colour instead of the definition's fails the guard with
  "the preview swatch for base (#1e1e2e) has r 0, want 30".
- **A, B and C are all implemented.** Remaining known gaps: fish is generated but not switched (user
  state by decision), tmux has no repository-owned theme, and the desktop light/dark switch is
  unchanged.

## Update: D - the preview repaints the whole interface

- **styles.go is now built from one palette.** Every colour name and every style is assigned by
  `applyUIColors(uiColors)`, and the package vars are that build's output. The default calls it with
  `defaultUIColors()`; the preview calls it with the highlighted theme's colours
  (`theme_preview.go`), so the chrome and the preview cannot drift and there is no second copy of a
  palette in Go.
- **The whole interface repaints.** `View()` applies the preview for the duration of one render and
  restores the previous palette before it returns, so a screen with no preview active is byte-for-byte
  the default chrome and leaving the theme row puts the default back.
- **It says so.** The preview row reads `Preview (nothing applied) — <name>` followed by a swatch per
  role, so a repainted installer cannot be mistaken for one already changed.
- **Nothing is written.** The preview only reassigns in-memory styles.
- **The defaults are pinned to the definition.** `TestTheDefaultStylesMatchTheDotfilesDefinition`
  checks `styles.go`'s Dark entries against `themes/dotfiles.toml` (base/text/blue/cursor/red/green
  plus the prompt's subtext0, mauve and peach). Two installer-only tints remain and are named in the
  guard: `SyntaxKeyword #C99AD6`, `SyntaxString #DFBD76` have no role in `themes/*.toml`, so the
  preview maps them onto the theme's magenta and peach instead; the README records them as
  installer-only.
- Guards and teeth: the repaint, the restore, the no-write rule and the defaults are all covered
  (`TestThePreviewRepaintsTheWholeInterface`, `TestThePreviewWritesNothing`,
  `TestTheDefaultStylesMatchTheDotfilesDefinition`, `TestAPartialThemeCannotBePreviewed`). Teeth:
  disabling the preview selection fails the repaint guard, removing the restore leaves `Primary` at
  the theme's blue, and changing a `styles.go` default fails the pin.
- **No golden moved**; the full suite and `make preflight` are green.
## Update: E - the syntax tints are theme roles

- `themes/*.toml` gained a `[syntax]` table with four keys: `keyword_light`, `keyword_dark`,
  `string_light`, `string_dark`.
- **dotfiles**: the two pairs `styles.go` already carried, transcribed
  (`keyword #7a3e9e/#c99ad6`, `string #8a6a00/#dfbd76`). **Catppuccin Mocha**:
  `keyword_dark #cba6f7`, `string_dark #a6e3a1`, both already in the repository's
  `[palettes.catppuccin_mocha]`, chosen by Catppuccin's own convention (keywords mauve, strings
  green). **Kanagawa, Everforest, Kagawa**: empty, like the rest of their palette.
- `styles.go`'s defaults are pinned by `TestTheDefaultStylesMatchTheDotfilesDefinition`, which now
  covers **both members** of both roles; the guard no longer logs an exception, because there is none.
- `theme_preview.go` reads `[syntax]` instead of approximating with the prompt's mauve and peach. The
  approximation is gone; `TestThePreviewPaintsTheThemesSyntaxRoles` fails if it comes back.
- **A readability limit is recorded:** Catppuccin Mocha has no light member here (the repository ships
  only Mocha, not Latte), so the preview uses the dark value on a light terminal too, where Mocha's
  mauve and green are low-contrast. A property of the theme, stated in `themes/README.md`.
- **No block generator needed a change:** no generated file (terminals, Starship, zsh/p10k, Herdr,
  fish, bat, Neovim) carries the installer's syntax tints; they are chrome-only.
- Teeth: changing `keyword_dark` in `themes/dotfiles.toml` fails the defaults guard naming `styles.go`;
  putting the mauve approximation back fails the preview guard.

## Update: the definitions are found without a clone (visibility fix)

- **Defect.** B read the definitions from `m.RepoDir`, the checkout the **clone step** creates, so
  before an install `m.RepoDir` was empty and `dotfilesThemesCmdIfNeeded` returned no read at all
  (`if m.DotfilesThemes != nil || m.RepoDir == ""`). A user who launches the installer from inside
  their checkout — the normal case — saw no theme row until they had installed. A feature that cannot
  be seen does not exist.
- **Fix.** The definitions are resolved, in order, from the first directory that holds
  `themes/*.toml`: 1. `$DOTFILES_DIR` (a new interface: the user names a checkout), 2. the clone this
  run made, 3. the working directory and its parents (nearest first), 4. `~/dotfiles`, `~/.dotfiles`.
  When none holds definitions the section keeps the honest "not switchable here" message and names
  the search. The definitions remain repository files; nothing is packaged into the binary.
- **Guards.** `TestThemeResolutionTriesDotfilesDirFirst`,
  `TestThemeResolutionUsesTheCloneBeforeTheWorkdir`,
  `TestThemeResolutionUsesTheWorkdirAndItsParents`,
  `TestThemeResolutionFallsBackToHomeDotfiles`,
  `TestThemeResolutionFallsBackToHiddenHomeDotfiles`,
  `TestThemeResolutionReportsWhenNothingIsFound`, and the user's case
  `TestUtilitiesSeesThemesWithoutACloneWhenRunFromTheRepo`, which renders the section and asserts the
  rows and their exclusion lists are visible. Teeth: restoring the `RepoDir == ""` guard makes the
  user-case test fail with "opening Utilities from inside the repository issued no theme read".
- The order is documented in `docs/tui-installer.md`, beside the dotfiles theme switch.
- **The documentation guard caught it the same day.** `TestBrandingDocNamesEveryInstallerEnvironmentVariable`
  (`installer/cmd/dotfiles/help_test.go`, from #140) derives every `DOTFILES_*`
  literal from the installer source and failed on `DOTFILES_DIR` as soon as it was introduced, before
  anyone had to remember to document it. The fix is one row in `docs/BRANDING.md` under *Overrides for
  non-standard layouts*: the variable is an override the user sets, not an installer switch. That the
  guard stopped the work is the feature working, not friction.

## Update: the macOS golden flake, diagnosed and fixed

- PR #161's macOS smoke test failed on `TestMainMenuGolden` with a transcript that held only the
  terminal's initialisation and teardown sequences. It was not a content change: the golden file was
  byte-identical, the failing `got` contained no frame at all, and the main-menu render never touches a
  theme definition.
- **The golden did not depend on the working directory.** `NewModel`, `Init` and the main-menu render do
  not call the theme resolver; it is reached only when Utilities is entered (update.go). The snapshot is
  the same wherever the checkout lives, which is why every other golden and every other machine passed.
- **The cause was a test race.** `waitForAnyOutput` quits after the first two length-positive output
  events; the terminal's initialisation is one of them and the first frame can arrive in several, so on a
  loaded macOS runner Ctrl+C won before the frame was drawn.
- **Fix:** `waitForGoldenFrame` waits for the frame's own marker before quitting, and the six
  `teatest`-driven golden tests use it through `goldenTranscript`. `TestGoldenFrameWaitsForTheScreen`
  reproduces the race with a staged reader (init, partial repaint, frame): it fails deterministically
  while the wait accepts "any output" and passes when it waits for the frame. No golden file changed.

## Update: the refresh that makes a pre-marker machine usable (F)

- **The defect.** On a machine installed by an older checkout the managed files carry
  neither the `dotfiles-managed-config:` marker nor a generated block, and some have
  drifted. `applyDotfilesTheme` refuses every one of them (it cannot tell an old dotfiles
  file from one the user wrote), so the theme switch is unusable there and "reinstall" is
  not an answer.
- **The fix.** The theme picker now offers **Refresh outdated theme files**. It detects the
  installed theme files that are not in the generated form (no marker, no block, or a block
  that no longer matches its definition) and opens a **review** naming every file it would
  touch, why, and where each file that may be the user's will be preserved first. Nothing is
  written before the review is confirmed; Cancel leaves everything.
- **Preserve-user-configs is honoured.** An unowned file is copied first to the install
  steps' own places (`~/.zshrc.d/`, `~/.config/fish/dotfiles.d/`) or to
  `<path>.bak-dotfiles-<timestamp>` beside itself, and the result names the exact path. The
  previous bytes go into the same `theme.json` the switch writes, so **Undo the last
  dotfiles theme change** restores every file byte-for-byte.
- **Honest degradation.** A file that cannot be refreshed (unreadable, not regular, or a
  content that is not a recognizable block) is named and skipped; the rest of the refresh
  carries on. `--dry-run` writes no file, no preserve copy and no record.
- **Tests (one per case).** `TestThemeRefreshBringsAnOldManagedFileUpToDate`,
  `TestThemeRefreshPreservesUserContentAndSaysWhere`, `TestThemeRefreshIsReversible`,
  `TestThemeRefreshSkipsOnDryRun`, `TestThemeRefreshSkipsAnUnrefreshableFileAndContinues`,
  and the picker wiring in `TestThemePickerOffersTheRefreshRow`,
  `TestThemeRefreshNamesTheFilesBeforeWriting`, `TestThemeRefreshCancelLeavesTheFilesAlone`.
  The refresh review is measured by the terminal matrix as its own screen case
  (`measuredScreens` 55 -> 56). Teeth: making `refreshThemeFiles` a no-op fails the old-file
  and the reversible guards again.
- **No golden moved.** The only assertion rows that changed are the theme picker's own
  option list (`TestThemePickerListsTheDerivedThemesAndUndo`), which now holds the refresh
  row between the themes and the undo row.
