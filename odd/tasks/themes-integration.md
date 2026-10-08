# The theme library: complete palettes for the partials, and more themes (T2 + T4)

The user's ask, in their words: *"more themes: one purple-pink, one darker, one
lighter, etc."* and *"themes that do not exist in some places"*. Before this
change the repository offered two themes (`dotfiles`, `catppuccin-mocha`), held
three more as names with no palette (`everforest`, `kanagawa`, `kagawa`) and
would not offer them, and `[syntax]` was outside the completion rule, so a theme
could be offered without being previewable.

## The rule that governs every value

**No colour is invented.** A value is either what the repository already ships or
a transcription of a published palette, cited in `themes/README.md` with the
project, the URL and the roles it holds. A role the published palette does not
define stays empty. There is no network fetch; the transcriptions are committed
as definition files.

## What changed

| # | Task | State |
|---|---|---|
| T1 | Give the partial themes a palette | `everforest` and `kanagawa` transcribed from their published palettes and now complete; `kagawa` retired (it was never a theme) |
| T2 | The new themes the user asked for | `rose-pine` (purple-pink, and the darkest new base), `catppuccin-latte` (the light flavour); Rosé Pine Moon considered and left out |
| T3 | The completion rule | `[syntax]` is now part of `Complete()`: a theme is offered only when it can be applied **and** shown |
| T4 | Provenance, written | `themes/README.md` gains a per-theme origin section and a "what is transcription and what is the repository's" note; each definition carries `[theme] provenance` |
| T5 | Guards with teeth | `TestOnlyThemesThatCanBePreviewedAreOffered`, `TestNoInventedThemeRoleSlipsIn`; the stale Mocha-no-light guard and the partial-preview guard were updated rather than deleted |

## The themes now

| Theme | State | Source | Covers | Leaves out |
|---|---|---|---|---|
| dotfiles | complete | the repository's six hand-written blocks | terminals, Starship, zsh/p10k, Herdr, bat | fish, Neovim, tmux |
| Catppuccin Mocha | complete | ghostty conf + starship table | the nine above + Neovim | fish, tmux |
| Catppuccin Latte | complete | catppuccin/catppuccin (published Latte) | terminals, Starship, zsh/p10k, Herdr | fish, bat, Neovim, tmux |
| Kanagawa | complete | rebelot/kanagawa.nvim (wave) | terminals, zsh/p10k, Herdr, Neovim | Starship, fish, bat, tmux |
| Everforest | complete | sainnhe/everforest (dark medium) | terminals, zsh/p10k, Herdr | Starship, fish, bat, Neovim, tmux |
| Rosé Pine | complete | rose-pine/rose-pine | terminals, zsh/p10k, Herdr | Starship, fish, bat, Neovim, tmux |

## Kagawa, with the evidence

`themes/kagawa.toml` is removed. The repository's only Kagawa file,
`dotfiles-fish/fish/themes/Kagawa.theme`, was introduced upstream as a
byte-for-byte copy of `Kanagawa.theme`, with Kanagawa's own header. No published
"Kagawa" palette exists. A name without a theme is not offered, so the definition
is gone.

**Open item outside this change's edit surfaces:** the generated fish file
`dotfiles-fish/fish/themes/Kagawa.theme` still exists in the tree and should be
deleted in the same commit, or the repository keeps shipping a Kagawa file for a
theme that no longer exists. See the `risks` note in the handoff.

## The `[syntax]` rule

`Complete()` now requires the twenty-two canonical roles **and** `keyword_dark`
plus `string_dark`, the two `[syntax]` members the preview reads. The light
member stays optional (the preview falls back to the dark one), so a published
palette with no light flavour is not forced to invent one. Catppuccin is the
exception that proves the pairing: Mocha's light member is now Latte's mauve and
green, so the readability limit recorded in the old README is gone.

Teeth:

- `TestOnlyThemesThatCanBePreviewedAreOffered` builds a definition with every
  terminal role and no `[syntax]` and asserts it is not complete, is not offered,
  and that every offered theme previews. Dropping `[syntax]` from `Complete()`
  fails it.
- `TestNoInventedThemeRoleSlipsIn` asserts the parser refuses a palette role
  outside the canonical twenty-two and a `[syntax]` role outside the four the
  preview reads, and that every shipped definition cites a provenance.

## The frame constraint (why Rosé Pine Moon is not here)

The Utilities section fits **six** complete theme rows at the 60×20 floor. A
seventh row overflows the frame guard
(`TestEveryScreenFitsEveryTerminalSize/utilities/60x20`) by one row. That guard lives
in `installer/internal/tui/screen_coverage_test.go`, which is not an edit surface
of this change. Rosé Pine Moon was therefore left out, on top of the honest
reason: its base (#232136) is *lighter* than Rosé Pine's (#191724), so it does not
answer the "más oscuro" the user asked for in the first place.

## Validation

- `cd installer && gofmt -l .` — clean
- `cd installer && go vet ./...` — clean
- `cd installer && go test ./...` — green (no golden moved)
- `make preflight` — green

---

# Themes: complete coverage, a picker, more themes, and a preview you can actually see

The user's ask, in their words:

1. *"¿Se podrían temas que no existen en algunos lados, incluirlos?"* - bring the theme to the tools that have
   none: **fish, tmux, Neovim** (and anything else the inventory finds), and give the **partial themes**
   (Everforest, Kagawa, Kanagawa) the palettes they are missing so they can be offered at all.
2. *"Que la utilidad sea cambiar de tema, y dentro de ahí, si le das, tener los diferentes temas y aplicar ahí el
   cambio"* - the utilities entry becomes **"Change theme"**, and inside it a **list of themes** where the change is
   applied. The current design puts the theme rows directly in the utilities menu; the user wants a level in
   between.
3. *"¿Se podría ver, de mejor manera, la aplicación en tiempo real? En Herdr, en la consola, en la utilidad, etc."*
   - the preview should be visible **where it happens**: in the terminal itself, in Herdr, not only inside the
   installer's own repaint.
4. *"¿Se podrían añadir más themes? Uno morado-rosa, uno más oscuro, uno más claro, etc."* - **more themes**:
   a purple-pink one, a darker one, a lighter one.
5. *"Que esté todo superintegrado."*

## What exists today, so the ask is built on facts

- **One definition per theme** in `themes/*.toml`: two complete (`dotfiles`, `catppuccin-mocha`) and three partial
  (`everforest`, `kagawa`, `kanagawa`) which are **not offered** because they have no canonical palette.
- **Generated from the definition**: the four terminal palettes, Herdr's `[theme.custom]`, Starship, the whole zsh
  region (`PALETTE_*`, `*_SGR`, `LS_COLORS`, `EZA_COLORS`), the p10k fallbacks, the Neovim colourscheme line, the
  fish theme files, the bat `.tmTheme` files, and `BAT_THEME`.
- **Exclusion lists**: `dotfiles` leaves out **fish, Neovim, tmux**; `catppuccin-mocha` leaves out **fish, tmux**.
- **The switch** is reachable from the utilities menu, reversible through `$XDG_STATE_HOME/dotfiles/theme.json`,
  refuses files without an ownership marker, and is gated on `--dry-run`. **The preview** repaints the installer's
  own interface from the definition with **zero writes**.
- Provenance rules that stand: **no colour is invented**; a value comes from the repository or is transcribed from
  the published palette **with its source recorded**; a role with no value stays empty and the tool is reported.

## Contracts this phase must keep

- **No invented colours.** New themes (ask 4) are **transcribed published palettes with attribution in
  `themes/README.md`**, exactly as Catppuccin's values were.
- **Only whole themes are offered.** A theme is offered when it is complete for what it must apply *and show* -
  which is why this phase should also settle whether `[syntax]` belongs in `Complete()` (it currently does not,
  and a complete theme without it would be offered but not previewable).
- **The apply stays reversible** and only touches files the repository owns (ownership marker or nothing).
- **`--dry-run` skips everything that writes.**
- **A live preview that writes to disk is a different animal from the current one**, and it is the only way to
  answer ask 3 honestly: seeing it "in the terminal and in Herdr" means the real tools reloading. It therefore
  needs a **guaranteed restore**: enter a preview session (record every file it is about to touch, apply on
  selection change, and restore **on exit, on quit, and on interrupt**), refuse when a file is not ours, and say on
  screen that the tools are being changed **temporarily**. If that guarantee cannot be made airtight for a given
  tool, that tool stays out of the live preview and is named.
- **NO GOLDEN MAY MOVE** without its changed rows pasted and explained.

## Tasks

| # | Task | Notes |
|---|---|---|
| T1 | **Bring the theme to the tools that have none**: fish (generate the active selection, reversible), tmux (a repository-owned generated colour block instead of depending on the kanagawa plugin), and Neovim where a colourscheme for that theme exists | shrinks the exclusion lists; tmux must not invent a theme |
| T2 | **Give the partial themes a palette**: Everforest, Kagawa, Kanagawa transcribed from their published palettes with attribution, so they can be offered | Kagawa is currently a copy of Kanagawa with a corrected header; decide its fate with evidence |
| T3 | **The picker**: utilities → *"Change theme"* → the list of themes (each naming its exclusions) → apply, with undo | replaces the current flat rows; the frame guards re-measured |
| T4 | **More themes**: a purple-pink one, a darker one, a lighter one, plus whatever the families already in the repository suggest (Catppuccin Latte for light, for instance) | transcribed with attribution; a lighter theme is what makes the light-variant roles matter |
| T5 | **A preview you can see where it happens**: the live-preview session described above, with the guaranteed restore, so the terminal and Herdr reload while the cursor moves | the riskiest task; its restore path is the deliverable, not its sparkle |
| T6 | **Guards**: coverage per tool, the picker's derived list, every new theme's completeness, the preview session's restore-on-every-exit-path, and no golden moved | a guard that can see the class |

## Order, and why

T4 first (a **lighter** theme is what exercises the light variants the partial themes and the syntax roles hint at),
then T1 and T2 together (coverage), then T3 (the picker, which is where the coverage becomes visible), then T5
(the live preview), with T6's guards written alongside each. T5 depends on T3 because a preview session needs a
place to be entered from and a list to move through.

## Progress

### T1 — the tools that had no theme (complete, branch `feat/theme-coverage`)

**The criterion is how many tools a theme stopped leaving out, not how many files were generated.**

| Theme | Left out before | Left out now |
|---|---|---|
| dotfiles | fish, Neovim, tmux | **Neovim** only |
| Catppuccin Mocha | fish, tmux | **nothing** |

Four theme/tool exclusions removed. The one that stays — Neovim under `dotfiles` — is honest: there is no
colorscheme in this repository for the dotfiles palette, and inventing one is forbidden.

- **fish is switched, reversibly, without writing the user's state.** The switch rewrites a marked block in
  `dotfiles-fish/fish/config.fish` (a file this repository owns) and records its exact bytes, so **Undo** restores
  it byte-for-byte. `fish_variables` — where `fish_config theme choose` keeps the user's own choice — is **never
  written**. The block sets the fish colour variables in the global scope, which fish returns over a universal one,
  so the palette is seen even after a `fish_config` choice. For a theme with no `[fish]` table (Catppuccin Mocha)
  the fish roles are derived from the canonical palette by the fixed mapping the dotfiles definition records; no
  colour is invented.
- **tmux gets its own style block, not a plugin's theme.** `dotfiles-tmux/tmux.conf` carries a marked block that
  paints tmux's own options (status bar, active/inactive windows, panes, copy mode, display panes, clock) from the
  definition. It is placed **after** `run '~/.tmux/plugins/tpm/tpm'`. Evidence: tmux runs `run-shell`
  synchronously — measured with tmux 3.3a, the server did not finish reading its config until the run-shell
  command returned — so TPM's plugin styles are already written when the block is read, and the block wins. The
  generated block parses cleanly (`tmux -f theme.conf new-session -d` exit 0) and every option applies.
- **Neovim** keeps its generated `colorscheme` line for the themes whose plugin ships one (Catppuccin Mocha →
  `catppuccin`); `dotfiles` names none, so Neovim is reported rather than pointed at a colorscheme that does not
  exist.

**The fish default changed, and the changed rows are pasted here.** The hand-written block in `config.fish` was a
third fish palette that disagreed with `themes/dotfiles.toml`'s `[fish]` table (which is the mechanical mapping
from the canonical palette). Generating it from the definition fixed the drift; every new value is a canonical
role:

```
-fish_color_command      $cyan   7AA89F   → +fish_color_command      b7cc85  (green)
-fish_color_end          $orange DEBA87   → +fish_color_end          7aa89f  (cyan)
-fish_color_param        $purple A3B5D6   → +fish_color_param        7fb4ca  (blue)
-fish_color_comment      8394A3           → +fish_color_comment      8a8fa3  (bright_black)
-fish_color_autosuggestion 8394A3         → +fish_color_autosuggestion 8a8fa3
-fish_pager_color_progress 8394A3         → +fish_pager_color_progress 8a8fa3
-fish_pager_color_prefix  $cyan 7AA89F    → +fish_pager_color_prefix  b7cc85  (green)
-fish_pager_color_description 8394A3      → +fish_pager_color_description 8a8fa3
```

`DEBA87`, `A3B5D6` and `8394A3` are not canonical roles of the palette (`DEBA87`/`A3B5D6` are prompt `peach`/
`mauve`; `8394A3` is in no table), which is exactly the hand-written drift the definition removes. No golden frame
moved; the 18 `.golden` snapshots are untouched.

**Guards added / updated** (`installer/internal/tui/install_paths_test.go`, `update_test.go`):

- `TestEveryThemeReportsTheToolsItWouldLeaveOut` now pins the exact exclusion lists per offered theme (dotfiles →
  `[Neovim]`, Catppuccin Mocha → none), so shrinking or growing one is a visible decision.
- `TestGeneratedThemeBlocksInventNoColour` — every colour a generated block emits must be in the definition's
  `[palette]`, `[fish]` or `[prompt]` table; it fails if the fish or tmux block is filled by eye.
- `TestTheFishDerivationMatchesTheDotfilesTable` — the derivation that gives Catppuccin Mocha a fish palette
  must still match the `[fish]` table `themes/dotfiles.toml` records.
- `TestTmuxThemeBlockLoadsAfterPlugins` — the tmux block must sit after the TPM `run` line.
- `TestDotfilesThemeSwitchIsReversible` / `TestDotfilesThemeSwitchSkipsOnDryRun` now install and exercise the fish
  and tmux files too, so the byte-for-byte restore and the `--dry-run` gate cover them.
- `themeSourceBlocks` now pins the fish and tmux shipped blocks to the definition's palette, and the per-theme and
  per-artifact byte-for-byte guards cover both new blocks.

**Validation:** `go test ./... -count=1` in `installer/` (4792 passing), `gofmt -l`, `go vet`, `git diff --check`,
and `make preflight`.

### T3 — the picker (complete, branch `feat/theme-picker`)

The utilities row opens a level, and the list lives inside.

Follow-up to [`unified-themes.md`](unified-themes.md) piece B/C. The user's ask, in their words:
*"que la utilidad sea cambiar de tema, y dentro de ahí, si le das, tener los diferentes temas y
aplicar ahí el cambio"* — the Utilities section listed the theme rows directly; the themes now live
one level in.

#### What changed

- **Utilities offers one row, `Change the dotfiles theme`.** The theme rows and the undo row are no
  longer listed there. The row appears when there is at least one complete theme or a recorded
  change to undo, the same condition the old block used.
- **A new screen, `ScreenThemePicker`, holds the list.** It is opened by that row. Its rows are the
  complete themes, derived from the definitions (`dotfilesThemeOptions()`), each naming the tools it
  leaves out (`Apply the <name> theme (not <tools>)`); then `Undo the last dotfiles theme change`
  when a record exists; then `← Back`. The list order is the definitions' order.
- **The live preview moved with the list.** `previewThemeDef()` reads `GetCurrentOptions()` of the
  current screen, so moving the cursor over a theme row on the picker repaints the whole interface
  exactly as it did on Utilities. Nothing was reimplemented.
- **Apply and undo call the existing switch.** `handleThemePickerKeys` returns
  `applyDotfilesThemeCmd(def)` / `undoDotfilesThemeCmd(record)` — the same commands the utilities
  handler used; the dry-run gate is inside `applyDotfilesTheme`/`undoDotfilesTheme`, unchanged.
- **Esc steps back to Utilities**, not to the main menu, so the new level is a level.

#### The list is derived, never typed

`ScreenThemePicker`'s options are `m.dotfilesThemeOptions()`, which iterates `m.DotfilesThemes` and
keeps `def.Complete()`; `offeredThemeIDs()` is the same rule. No theme name, count or order is written
in the picker.

#### Guards

- **Screen coverage.** `ScreenThemePicker` is measured by `screensTheInstallerStatesNeverReach` and
  `terminalFitCases` (the pinned count went 54 → 55), and its own case is
  `themePickerFrameCase` — definitions read from the repository, a record to undo, and the cursor on a
  theme row so the preview row is measured. `TestEveryScreenIsCoveredByAFrameGuard` failed with
  `[ScreenThemePicker]` before the case was added (the RED for the guard).
- **Frame fit.** The picker is rendered at all twelve sizes. Its measured frame is logged by
  `TestThemePickerFrameFitIsMeasuredAtEverySize`, which also asserts the data survives: at every size,
  including 60x20, each theme row, the undo row and the way back are on screen.

#### Fit at the small sizes (measured)

`renderThemeScreen` sizes the description to `bodyRows - (title + blank + menu + preview + notice)`; the
menu is never windowed, so when the frame is short the description is what gives way and the rows win.
Nothing cedes at either floor:

| Terminal | Content width | Rendered rows | Rendered columns | Menu rows intact |
|----------|---------------|---------------|------------------|------------------|
| 80x24    | 76            | 24 of 24      | 80 of 80         | yes              |
| 60x20    | 56            | 20 of 20      | 60 of 60         | yes              |

(All twelve: 80x24, 90x28, 100x25, 100x30, 120x24, 120x34, 140x44, 160x50, 200x60, 227x62, 80x30,
60x20 — each renders exactly its terminal.)

#### Goldens

**No golden file moved.** `git diff --stat installer/internal/tui/testdata/` is empty and the 20
golden tests pass. The main-menu row is still `🧰 Utilities`, so `TestMainMenuGolden`,
`TestMainMenuWideGolden` and the four companion/creature goldens are byte-identical — the creature's
rows are intact. What moved is the pinned behaviour of the utilities *section*, which is tested in
`update_test.go` rather than snapshotted: the theme rows left the section and the preview tests moved
to `ScreenThemePicker` (`TestThemePickerRowsAreDerivedAndNameExclusions`,
`TestTheThemePreviewShowsTheValuesTheApplyWouldWrite`, `TestThePreviewRepaintsTheWholeInterface`,
`TestThePreviewWritesNothing`). `TestUtilitiesSeesThemesWithoutACloneWhenRunFromTheRepo` now checks
the `Change the dotfiles theme` row on Utilities and then the rows on the picker.

#### Not changed

The desktop light/dark switch, its rows and its undo; the theme definitions, generators and ownership
markers; `applyDotfilesTheme`/`undoDotfilesTheme`; the main-menu row and every other screen; no new
dependency.

#### Follow-up: PR #164's macOS golden failure, diagnosed and fixed

The `macOS smoke test` went red on `TestCompanionGoldenPinsThePixelSpriteAndItsGaze` with a `--- golden`
diff. It was **not** the theme-picker work and **not** the empty-frame race fixed in `e3ed85c`. It is a
race in the golden infrastructure itself, and it is recorded here because the next person who sees a
`--- golden` on macOS at 3am should find this before questioning their own change.

##### Root cause

The OSC terminal-title sequence (`\x1b]2;<title>\x07`) is written when bubbletea handles the
`setWindowTitleMsg` produced by `tea.SetWindowTitle` in `Init`; the first frame is buffered and flushed
by the renderer's own ticker. The two are independent, so under load the title can appear:

1. before the frame (what the pinned golden records),
2. after the frame (the observed failure), or
3. not at all before the capture quits (the `SetWindowTitle` message lost to Ctrl-C).

All three are the same screen. The diff in case 2 is exactly two hunks, both about the title's
position; every frame byte is identical.

##### Why the previous fix did not cover it

The golden was **already a frame-waiting golden**: it used an inline `teatest.WaitFor(... "Main Menu")`
whose predicate is byte-for-byte `waitForGoldenFrame`'s. The commit that added the helper said so:
*"The companion goldens already did the equivalent."* So the list of six refactored goldens was
correctly scoped to the ones still on `waitForAnyOutput`; there was no weaker golden left out. Routing
this capture through `goldenTranscript`/`waitForGoldenFrame` was tested and **still flaked**, because
the wait guarantees the frame is present, not that the title arrives before it.

##### Evidence

- Real branch (`d65cc89`), 12× `yes >/dev/null`, pixel golden: FAIL 3/40, 3/160, 4/20.
- Base `9093262` (extracted read-only with `git archive`, no branch change): FAIL 3/40, same diff.
- Base + `goldenTranscript`: FAIL 3/40 — the existing remedy does not fix it.
- Base + title normalization: 60/60 pass.
- The six fixed goldens (`Animating=false`) never flaked under the same load; the pixel golden's larger
  first frame is what makes it lose the race most often.

##### Fix

`requireGoldenCapture` (teatest_test.go) is now the single comparison every teatest golden goes through.
It strips the OSC title from both sides and restores the golden's own title at its pinned offset, so the
title's arrival time is ignored while the frame bytes are still compared one by one. A live title whose
text differs still fails; a missing one is restored from the golden. It reuses `readGoldenBytes` and
`normalizeGoldenBytes` and still compares through `teatest.RequireEqualOutput`, so `-update` keeps
working. All ten teatest-driven goldens (the four companion + the six fixed) go through it, so the
latent race is covered in every one of them. **No golden was regenerated.**

Teeth: `TestGoldenCaptureIgnoresTheTerminalTitlePosition` stages start → frame → title-last and requires
the match, then a title that never arrived, then a negative control where a frame row differs (must
fail) and a different title text (must fail).

Rate under load (12× `yes`, `-count=60` on the pixel golden): **before ~3–4/40, after 60/60.**

### T3b — the list scales: the frame is no longer the theme limit (complete)

**The defect, measured.** The picker drew every row it had. The guard that failed first was not the
row count, though: with the repository's six themes the row budget still fits, and what failed was
`TestThemePickerFrameFitIsMeasuredAtEverySize` reading a long exclusion row (`Apply the Catppuccin
Latte theme (not fish, bat, Neovim, tmux)`) as **dropped** because the row measure had cut it. The row
count is the deeper limit the same guard exposed: once the list grows past the frame, the frame -- not
the palette -- caps how many themes can be offered, which the synthetic 24-theme guard below measures
as 36 rows drawn with 16 clipped at 60x20.

#### The mechanism, and why this one

The picker reuses the mechanism the repository already had for long lists: **`listWindow`**, the
cursor-centred window the keymap menus, the restore list and the installing rail already use. The body
budget is split so the menu -- the data -- takes its share first and the description takes only what is
left; when the list is longer than that share it is windowed around `m.Cursor`, so the highlighted row
is always inside the window by construction. No stored scroll offset was added, and no new key was
bound: the cursor already walks the list, and `listWindow` derives the window from it, so the view and
the keys cannot disagree. The visible slice is named in the frame's header with the existing
`scrollVital` (`Showing 3-14 of 20`), the same indicator the keymap and restore lists use, so a long
list does not look like a list that ends where the screen does. Reusing the existing window kept the
change to `renderThemeScreen` and its guard; the handlers, the options and the preview did not move.

#### Reachable rows, measured (24 complete themes: 28 menu entries, 26 data rows)

| Terminal | First screen | Frame before | Frame after | Data rows reachable |
|----------|--------------|--------------|-------------|---------------------|
| 60x20    | 12 themes    | 36 rows (16 clipped) | 20 rows exactly | all 26, by cursor |
| 80x24    | 16 themes    | 36 rows (12 clipped) | 24 rows exactly | all 26, by cursor |

Before the fix the cursor could still move through every row, but the frame drew 36 rows and the
terminal took the excess away without a marker, so the rows below the fold were not actually visible.
After the fix every theme, the undo row and the way back are reached by moving the cursor, and the
selected row is always drawn.

#### Guards

`TestThemePickerScrollsToEveryRowAtTheSmallTerminals` (screen_coverage_test.go) builds 24 complete
definitions (more than either floor holds) and walks the whole list with the real key handler at 60x20
and 80x24. At every step it asserts: the frame renders **exactly** its terminal's rows and columns; the
row under the cursor is the one carrying the `▸` marker; the preview row is drawn for every theme row,
including the ones off the first screen; and at step 0 the list is genuinely windowed (fewer theme rows
on screen than built) and the header carries the visible-range count. After the walk it asserts every
non-separator option was under the cursor. The synthetic definitions carry the canonical palette *and*
the `[syntax]` members, so they pass the merged branch's `Complete()` -- without the syntax they would
be partial and not offered, and the guard would pass vacuously.

`TestThemePickerFrameFitIsMeasuredAtEverySize` changed its **property** rather than its numbers: it no
longer demands that every row fit the frame at once, but that **every row is reachable by moving the
cursor and the frame still fits exactly**. That is a change of property, not a lowered guard: a row
that cannot be reached, or a frame that grows to hold one, still fails.

**Teeth (RED).** With the windowing disabled the guard fails at both sizes: `renders 36 rows, want
exactly 20/24` and `the first screen shows 24 theme rows, want fewer than the 24 built`, and the header
carries no visible-range count. With the window restored it passes.

#### Goldens

**No golden moved.** `git diff --stat installer/internal/tui/testdata/` is empty; all golden tests
pass, including `TestMainMenuGolden`, `TestMainMenuWideGolden` and the four companion/creature goldens.
With the repository's six themes the list still fits every measured terminal, so `renderThemeScreen`
renders byte-identical bytes there and the companion-coverage baselines are unchanged.

#### Not changed

The theme definitions, generators and ownership markers; the apply/undo commands; the preview; the
handlers and the option derivation (`dotfilesThemeOptions()` still offers only complete themes); the
main-menu row and every other screen; no new dependency.

### T7 — the exclusion column says "nada" for all six (complete, branch `feat/theme-full-coverage`)

The user's requirement, in their words: *"en 'deja fuera', para todos los temas debería salir 'nada',
es decir, que todo admita esos temas"* — the **six themes in the twelve tools, with no gap in the
matrix**, and the requirement fixed by a guard rather than by a promise.

#### The final matrix

| Theme | Alacritty | Kitty | WezTerm | Ghostty | Starship | zsh | p10k | Herdr | fish | bat | Neovim | tmux | Deja fuera |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| dotfiles | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | **nada** |
| Catppuccin Mocha | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | **nada** |
| Catppuccin Latte | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | **nada** |
| Kanagawa | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | **nada** |
| Everforest | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | **nada** |
| Rosé Pine | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | **nada** |

#### What closed each gap, and whether the value is direct or derived

No colour was invented. A value is **direct** when the definition declares it (`[prompt]`, `[fish]`,
`[bat]`, `[nvim]`) or the repository's plugin install provides it, and **derived** when it comes from
the theme's own canonical palette by the fixed mapping recorded in
[`themes/README.md`](../../themes/README.md#derived-roles-the-fixed-mapping-per-tool) — a derived
colour is a colour the theme already holds.

| Theme | Tool that was left out | What closed it | Direct or derived |
|---|---|---|---|
| dotfiles | Neovim | `[nvim] name = "dotfiles"` + a colorscheme generated into `dotfiles-nvim/nvim/colors/dotfiles.lua` | the group→role mapping is derived; every colour is a declared palette role |
| Catppuccin Latte | fish | the fish role derivation (`themeFishRolesFor`) | derived (`[fish]` mapping) |
| Catppuccin Latte | bat | `[bat] name`/`[bat] file` + the install-time `.tmTheme` generation | direct (name) + derived (bat scope→role) |
| Catppuccin Latte | Neovim | `[nvim] name = "catppuccin-latte"` + generated `colors/catppuccin-latte.lua` | derived group→role mapping |
| Catppuccin Latte | tmux | the tmux style block (`renderTmuxTheme`) | derived (option→role) |
| Kanagawa | Starship | `themePromptDerivation` | derived (prompt role→palette role) |
| Kanagawa | bat | `[bat]` + install-time generation | direct + derived |
| Kanagawa | tmux | the tmux style block | derived |
| Everforest | Starship | `themePromptDerivation` | derived |
| Everforest | bat | `[bat]` + install-time generation | direct + derived |
| Everforest | Neovim | `[nvim] name = "everforest"` + generated `colors/everforest.lua` | derived |
| Everforest | tmux | the tmux style block | derived |
| Rosé Pine | Starship | `themePromptDerivation` | derived |
| Rosé Pine | fish | the fish role derivation | derived |
| Rosé Pine | bat | `[bat]` + install-time generation | direct + derived |
| Rosé Pine | Neovim | `[nvim] name = "rose-pine"` + generated `colors/rose-pine.lua` | derived |
| Rosé Pine | tmux | the tmux style block | derived |

The two `[prompt]` tables that stay explicit are the Catppuccin pair (transcribed) and dotfiles'
(transcribed from `starship.toml`); Kanagawa, Everforest and Rosé Pine have none and take the
derivation, exactly as Catppuccin Mocha already did for fish. Deciding **against** writing four
hand-copied `[fish]`/`[prompt]` tables was deliberate: Catppuccin Mocha — the theme the user named as
the reference — has no `[fish]` table at all, and one mapping plus
`TestTheFishDerivationMatchesTheDotfilesTable` cannot drift the way four copies can.

#### Changes

- **`themes/*.toml`**: `[nvim]` for dotfiles, Catppuccin Latte, Everforest and Rosé Pine; `[bat]` for
  Catppuccin Latte, Kanagawa, Everforest and Rosé Pine. Every value already existed; nothing new is
  transcribed.
- **`installer/internal/tui/installer.go`**: `themePromptDerivation` + `themePromptRolesFor` (Starship
  derives a table it does not have); `renderNvimTheme` + `themeNvimGroups` + `themeNvimTerminalRoles` +
  `themeBaseIsLight` + `themeNvimAvailable` (the generated colorscheme); `generateBatThemesFromDefinitions`
  called by the shell step (bat's `.tmTheme` for every named theme, written before the cache rebuild);
  `themeThemeFiles` gained a `Tool` and the four Neovim entries; **`themeToolArtifacts` is gone** — the
  definitions are the per-theme table now, and `themeCoverage` asks each tool's own generator (fish's
  role derivation, bat's renderer, tmux's renderer, Neovim's name resolution) so a tool the menu names
  cannot have nothing behind it. `batSelectionName` was added in the same file: the bat `BAT_THEME`
  selection now exports the `.tmTheme`'s **file** name, which is the name bat registers a custom theme
  under (see the defect below).
- **`themes/README.md`**: the "Derived roles" section with every mapping (fish's eighteen, Starship's
  twenty-four, bat's nine, tmux's options, Neovim's groups), the matrix above with "nada" in all six
  rows, and a declared-versus-derived table per theme and tool.
- **`README.md` / `docs/tui-installer.md`**: the copy that said a row names its exclusions now says
  what is true — every offered theme paints every tool, the mechanism is still there for a theme that
  could not, and the derived mappings are where the values come from.
- **No golden moved.** `git diff --stat installer/internal/tui/testdata/` is empty.

#### The guard, its RED and its teeth

`TestEveryOfferedThemePaintsEveryTool` replaced `TestEveryThemeReportsTheToolsItWouldLeaveOut` (which
pinned the exclusion lists by hand). It reads the same `themeCoverage` the menu reads, so it cannot
pass while a row still prints an exclusion, and it names the theme and the tool:

```
install_paths_test.go:1151: theme "catppuccin-latte" leaves out fish, bat, Neovim, tmux: every offered theme must paint every tool (Alacritty, Kitty, WezTerm, Ghostty, Starship, the zsh line editor, the p10k prompt, Herdr, fish, bat, Neovim, tmux)
install_paths_test.go:1151: theme "dotfiles" leaves out Neovim: ...
install_paths_test.go:1151: theme "everforest" leaves out Starship, bat, Neovim, tmux: ...
install_paths_test.go:1151: theme "kanagawa" leaves out Starship, bat, tmux: ...
install_paths_test.go:1151: theme "rose-pine" leaves out Starship, fish, bat, Neovim, tmux: ...
```

That failure is the RED, run before any implementation. Teeth, measured by removing a table and
restoring it:

```
# [bat] removed from themes/rose-pine.toml
install_paths_test.go:1153: theme "rose-pine" leaves out bat: every offered theme must paint every tool (...)
install_paths_test.go:1160: rose-pine: covers 11 of 12 tools; leaves out [bat]
install_paths_test.go:1173: the row for "rose-pine" still names an exclusion though it leaves out no tool: "Apply the Rosé Pine theme (not bat)"

# [nvim] removed from themes/everforest.toml
install_paths_test.go:1160: everforest: covers 11 of 12 tools; leaves out [Neovim]
--- FAIL: TestEveryOfferedThemePaintsEveryTool
--- FAIL: TestTheNeovimColorschemeNamesResolve
```

New guards, all in `install_paths_test.go`:

`TestThePromptDerivationCoversEveryRequiredRole` (every prompt role has a palette parent; the derived
values are logged for audit), `TestThePromptDerivationAgreesWithThePreview` (the prompt and the
preview cannot call two different colours "muted"), `TestTheNeovimColorschemeNamesResolve` (a name
resolves to a plugin colorscheme or a shipped generated file whose own name matches),
`TestTheGeneratedColorschemeDeclaresTheBackgroundItsBaseImplies` (the derived `light`/`dark` follows
the base, and both answers must exist), `TestTheBatThemesTheSwitchOffersAreGeneratedAtInstallTime`
(the helper the shell step calls writes every named theme, name inside included),
`TestEveryCoveredToolCanBeGeneratedForEveryOfferedTheme` (rewritten to walk the switch's tool list),
`TestTheBatSelectionNamesTheFileBatRegisters` (the bat selection is the file's name and no two themes
share one — its teeth, run by temporarily restoring the old `[bat] name` export: `theme
"catppuccin-mocha" exports BAT_THEME="Catppuccin Mocha", but bat registers catppuccin-mocha.tmTheme as
"catppuccin-mocha"`, and the same for four more themes),
and `TestGeneratedThemeBlocksInventNoColour` (extended to the per-theme files; its colour token now
requires a whole word, because bat's `embedded` scope name contains "bedded", which is spelled with
hex digits and was read as an invented colour).

#### A pre-existing defect found while measuring, and fixed

`BAT_THEME` named the wrong key, and five of the six themes were affected. `[bat] name` is the
`.tmTheme`'s own `<key>name</key>`; bat registers a custom `.tmTheme` under its **file** name.
Measured with bat 0.26.1, in a cache built from `dotfiles-bat/themes/` into a temporary
`XDG_CACHE_HOME` (nothing under the repository or the user's config was written):

```
$ XDG_CACHE_HOME=$T/cache BAT_CONFIG_DIR=$PWD/dotfiles-bat bat cache --build
$ XDG_CACHE_HOME=$T/cache bat --list-themes | grep -i 'catppuccin\|dotfiles'
Catppuccin Frappe
Catppuccin Latte        <- bat's own bundled themes
Catppuccin Macchiato
Catppuccin Mocha
catppuccin-mocha        <- the repository's file, by its file name
dotfiles
$ printf 'if true; then echo hi; fi\n' | bat --language=bash --color=always --theme=catppuccin-mocha | cat -v | head -1
^[[38;2;137;180;250mif   <- #89b4fa, the repository's blue: our file
$ ... --theme="Catppuccin Mocha" ...
^[[38;2;203;166;247mif   <- #cba6f7, bat's bundled mauve: bat's own theme
```

Before the fix, `renderBatSelection` exported `[bat] name`, so only `dotfiles` reached the
repository's file (its name and its file name are the same string); `Catppuccin Mocha` painted bat's
bundled theme and the other four painted nothing this repository ships. `batSelectionName` now
exports the file's stem, `TestTheBatSelectionNamesTheFileBatRegisters` pins it, and **no committed
file moved**: the default theme's name and its file name coincide, so the `.zshrc` block the
byte-for-byte guard holds is unchanged (`13 shipped theme artifacts match their definitions
byte-for-byte`).

#### Validation

- `make check` — PASS (gofmt, `go vet ./...`, `go test ./internal/tui`, 250 s).
- `go test ./... -count=1 -timeout 30m` in `installer/` — **4826 passing**, 231 s.
- `luac -p dotfiles-nvim/nvim/colors/*.lua` — all four parse; running each against a stubbed `vim`
  sets 96 highlight groups, 16 terminal colours and the right `background`/`colors_name`; and a real
  Neovim loads each one (`nvim --headless -u NONE --cmd 'set rtp+=dotfiles-nvim/nvim' -c 'colorscheme
  <id>'`), reporting the theme's own `colors_name`, `background` and
  `Normal`/`String`/`Comment` values — Latte light, the other three dark.
- bat 0.26.1 against a cache built from `dotfiles-bat/themes/` in a temporary `XDG_CACHE_HOME`:
  `--theme=catppuccin-mocha` paints the repository's blue, `--theme="Catppuccin Mocha"` paints bat's
  bundled mauve — the measurement behind `batSelectionName` and its guard.
- `git diff --check` — clean. No new dependency; no commit, no push.

#### Not done, and why

Nothing is left uncovered, so there is no gap to report. One judgement call is recorded rather than
hidden: **Catppuccin Latte's Neovim colorscheme is generated instead of pointing at the Catppuccin
plugin's own `catppuccin-latte` name**, because this repository's Neovim configuration pins
`flavour = "mocha"` and the generated file is the one whose bytes can be checked here.

---

# T8 - Nocturne: a theme measured from the user's own terminal

The user showed a screenshot of their terminal (Gentle Shell running, 1701×947) and asked whether any
theme followed that look. None did - the closest, Rosé Pine, is a brighter plum with lavender text
and a stronger contrast - so a theme was built and implemented across every tool the switch paints.

No colour is invented. What the screenshot shows was **measured** from its pixels; the hues the
screenshot does not carry were **derived** from the measured set by one documented mapping.

| role | value | provenance |
|---|---|---|
| `base` | `#151316` | measured: the main background |
| `text` | `#c0c0c0` | measured: the body text (10.2:1 on base) |
| `cursor` | `#e0c070` | measured: the warm accent |
| `cursor_text` | `#151316` | measured: the base under the cursor |
| `selection` | `#202030` | measured: the slate block surface |
| `selection_text` | `#c0c0c0` | measured: the body text |
| `black` | `#100a0f` | measured: the side-panel background |
| `red` | `#cb8686` | derived: hue 0°, saturation 40%, lightness 66% |
| `green` | `#86cb86` | derived: hue 120°, same |
| `yellow` | `#e0c070` | measured: the warm accent |
| `blue` | `#8686cb` | derived: hue 240°, same |
| `magenta` | `#a08090` | measured: the mauve accent |
| `cyan` | `#86cbcb` | derived: hue 180°, same |
| `white` | `#c0c0c0` | measured: the body text |
| `bright_black` | `#a0a0a0` | measured: the dim text |
| `bright_red` | `#daa9a9` | derived: the red hue, lightness +10 |
| `bright_green` | `#a9daa9` | derived: the green hue, +10 |
| `bright_yellow` | `#e9d29a` | derived: the measured gold, +10 |
| `bright_blue` | `#a9a9da` | derived: the blue hue, +10 |
| `bright_magenta` | `#b69daa` | derived: the measured mauve, +10 |
| `bright_cyan` | `#a9dada` | derived: the cyan hue, +10 |
| `bright_white` | `#f0f0f0` | measured: the bright text |

The derivation's two numbers are themselves measured: 40% is the midpoint of the two measured
accents' saturation (mauve 14% and gold 64% average to 39%, rounded), and 66% is the measured warm
accent's own lightness. `[syntax]` is dark only (`keyword_dark #a08090`, `string_dark #e0c070`) -
the reference has no light version, so none is invented.

## The registration the task could not be done without

Nocturne's first RED proved the theme was complete except for Neovim: the guard failed naming
`nocturne` and `Neovim` because `themeNvimAvailable` reads `themeThemeFiles` by design
(`installer/internal/tui/installer.go:2584`) with no disk fallback, and that list is where a shipped
colorscheme is registered. `installer/internal/tui/installer.go` was outside the first set of edit
surfaces, so the work stopped there rather than guessing; the surfaces were widened and the
registration was added:

```go
{Tool: "bat", Theme: "nocturne", Path: "dotfiles-bat/themes/nocturne.tmTheme", Render: renderBatTheme},
{Tool: "nvim", Theme: "nocturne", Path: "dotfiles-nvim/nvim/colors/nocturne.lua", Render: renderNvimTheme},
```

## The generated files, with the repo's own mechanism

Both committed files were written by the generator, never by hand:

```
$ cd installer && go test ./internal/tui \
    -run TestGeneratedPerThemeFilesMatchTheirDefinition -update-theme-artifacts -count=1 -v
```

It produced `dotfiles-nvim/nvim/colors/nocturne.lua` and `dotfiles-bat/themes/nocturne.tmTheme`, and
`TestGeneratedPerThemeFilesMatchTheirDefinition` now pins **11** per-theme files byte-for-byte (was
9). `TestGeneratedThemeBlocksInventNoColour` checks 2182 colour tokens across 12 tools, 870 of them
in per-theme files, and no emitted colour is one the definition does not hold.

## The red (teeth) and the green

The **RED**, captured before the registration (the same failure returns if the `[nvim]` entry is
removed, which is the guard's teeth):

```
# themes/nocturne.toml present, but nocturne not in themeThemeFiles
install_paths_test.go:1154: theme "nocturne" leaves out Neovim: every offered theme must paint every tool (Alacritty, Kitty, WezTerm, Ghostty, Starship, the zsh line editor, the p10k prompt, Herdr, fish, bat, Neovim, tmux)
install_paths_test.go:1158: theme "nocturne" covers 11 of the 12 tools the switch names: [Alacritty Kitty WezTerm Ghostty Starship the zsh line editor the p10k prompt Herdr fish bat tmux]
install_paths_test.go:1174: the row for "nocturne" still names an exclusion though it leaves out no tool: "Apply the Nocturne theme (not Neovim)"
install_paths_test.go:1393: theme "nocturne" names the Neovim colorscheme "nocturne", which neither the repository's plugin install nor a generated file provides
--- FAIL: TestEveryOfferedThemePaintsEveryTool
--- FAIL: TestTheNeovimColorschemeNamesResolve
```

The **GREEN**, with the registration and the generated files in place:

```
install_paths_test.go:1161: nocturne: covers 12 of 12 tools; leaves out []
--- PASS: TestEveryOfferedThemePaintsEveryTool (0.00s)
install_paths_test.go:1422: 5 offered theme(s) select a generated colorscheme, 2 select a plugin's
--- PASS: TestTheNeovimColorschemeNamesResolve (0.00s)
```

It is now the seventh complete theme and **doce de doce, sin exclusiones**: every offered theme
paints every tool, and `TestEveryCoveredToolCanBeGeneratedForEveryOfferedTheme` renders 91
covered-tool blocks across the 7 offered themes.

## The colorscheme, loaded for real

```
$ luac -p dotfiles-nvim/nvim/colors/nocturne.lua
luac: parsed OK
$ nvim --headless -u NONE --cmd 'set rtp+=dotfiles-nvim/nvim' -c 'colorscheme nocturne' -c 'lua ...' -c 'qa!'
colors_name=nocturne
background=dark
termguicolors=true
Normal={ bg = 1381142, fg = 12632256 }   -- #151316 on #c0c0c0
String={ fg = 14729328 }                 -- #e0c070 (the measured gold)
Comment={ fg = 10526880, italic = true }-- #a0a0a0 (the measured dim text)
Keyword={ fg = 8816331 }                -- #8686cb (the derived blue)
Function={ fg = 8833926 }               -- #86cb86 (the derived green)
Error={ bg = 1381142, fg = 13338246 }    -- #cb8686 (the derived red)
terminal_color_0=#100a0f                 -- measured side-panel background
terminal_color_15=#f0f0f0                -- measured bright text
```

Every value is the one the definition holds; nothing was chosen by eye.

## Docs

`themes/README.md` gained the third value source ("measured from a reference, or derived from that
measurement"), a "Nocturne: measured, not transcribed" note with the derivation table, the measured
contrast, the dark-only `[syntax]` and the limitation (a reading of a rendered screenshot, not a
published palette), plus the theme in the seven-theme, coverage and declared/derived tables. Its
stale claim that "a seventh would overflow the 60×20 theme picker" was corrected: the picker windows
a longer list now, and Nocturne is that seventh theme with the frame guard green. `README.md` and
`docs/tui-installer.md` now say seven complete themes and name Nocturne.

## Validation

- `make check` - PASS (gofmt, `go vet ./...`, `go test ./internal/tui`, 13m23s).
- `go test ./... -count=1 -timeout 30m` in `installer/` - **5020 passing, 0 failing, 1 skipped**,
  4m17s (cmd/dotfiles 10, internal/system 240, internal/tui 3138 (+1 skip), trainer 1632).
- `gofmt -l installer/` - clean; `go vet` - clean; `git diff --check` - clean.
- **No golden moved.** The theme picker gains a row but draws it inside its existing window, so no
  selector golden changed (`TestThemePickerFrameFitIsMeasuredAtEverySize` and
  `TestThemePickerScrollsToEveryRowAtTheSmallTerminals` are green); a full `git status` shows only the
  edited files and the three new files.
- No commit, no push.

## Follow-up: the definitions outside the checkout

The user's report — *"Si no estoy en la ruta del dotfiles, no me salen las utilities"* — is the hole
this task left: the definitions were read only from a checkout (`$DOTFILES_DIR`, the clone, the
working directory and its parents, `~/dotfiles`, `~/.dotfiles`), the clone this run makes lives in a
temporary directory the cleanup step removes, and a user who launches the program from their home
directory is inside none of them. The theme switch was therefore offered only from inside a checkout.

The clone step now copies `themes/*.toml` into `~/.local/share/dotfiles/themes/` (or
`$XDG_DATA_HOME/dotfiles/themes/`) as the last resolution candidate. Only the files the repository
ships are written, an identical definition is left as it is, nothing is deleted, and the step says
where it wrote and how many were already current. Resolution still reads one directory and never
merges two. The user's case is reproduced by
`TestThemeResolutionFallsBackToTheInstalledDefinitions` and
`TestUtilitiesOffersTheThemeRowFromAnInstalledCopyOutsideTheRepo`, whose teeth are the new candidate:
removing it fails both by name.

---

# T9 - Applying the theme is not writing the file: bring the change forward per tool

The user, running the real installer: *"Para aplicar los temas, cuando le doy, por ejemplo, herdr, no
se actualiza, hay que hacer un reload config. ¿Se podría? Con el resto de partes de los dotfiles pasa
similar o igual? ¿Se podría arreglar en todos los lados?"* They are right, and it is the gap almost
no theme system closes: **writing the file is not applying the theme.** The switch wrote all twelve
artefacts and told no tool that anything had changed, so a tool that was already running kept the old
palette and the screen said "applied".

## The twelve, with the evidence for each

| Tool | Reloads itself? | Can we reload it safely? | What is left for the user | Evidence |
|---|---|---|---|---|
| Alacritty | Yes — watches its config and reloads live | Nothing to run | Nothing | Alacritty's live config reload is on by default; `alacritty.toml` does not turn it off (checked: no `live_config_reload` line). Tool documentation. |
| WezTerm | Yes — watches its config and reloads live | Nothing to run | Nothing | WezTerm reloads the config on file change. `.wezterm.lua` sets no `automatically_reload_config = false`. Tool documentation. |
| Starship | Yes — reads `starship.toml` at each prompt | Nothing to run | Nothing | Starship is invoked per prompt and re-reads its config; there is no daemon. Tool documentation. |
| Kitty | No — a running window reloads on a signal or the remote-control command | Yes, when we are inside Kitty: `kitty @ load-config` | `Ctrl+Shift+F5`, or `kitty @ load-config` | `dotfiles-kitty/kitty.conf:56-57`: `allow_remote_control yes`, `listen_on unix:/tmp/kitty`. The command runs only when `KITTY_LISTEN_ON` is set, so it targets the session the installer is in. |
| Ghostty | No — a running window reloads on a key or a signal | Not without a signal; documented, not run | `Ctrl+Shift+,` (macOS) or `SIGUSR2` | Ghostty reload keybinding/signal. Tool documentation — **not verified in this environment.** |
| Herdr | No — a running session reloads its config on a key | **No.** Signalling or scripting a live multiplexer can end the session | `Ctrl+b Shift+r`, or restart | `installer/internal/tui/keymaps_herdr.go:35`: `Ctrl+b Shift+r` → "Reload config". **A live Herdr is never touched.** |
| zsh line editor | No — `.zshrc` runs at shell start | No — it is the user's interactive shell | `exec zsh`, or a new shell | `.zshrc` is sourced at start; an already-open shell does not re-read it. |
| p10k prompt | No — the prompt is drawn from the loaded config | No — it runs inside the user's shell | `p10k reload`, or a new shell | `dotfiles-zsh/.p10k.zsh:26` ("type `source ~/.p10k.zsh`") and `:1851` (`p10k reload`). |
| fish | No — `config.fish` runs at shell start | No — it is the user's interactive shell | `exec fish`, or a new shell | `dotfiles-fish/fish/config.fish` is sourced at start. |
| bat | No — the theme must be **in bat's cache**, and `BAT_THEME` is read by a shell | Yes: `bat cache --build` (idempotent) | Open a new shell so `BAT_THEME` is re-read | bat reads a theme from its cache, not the themes directory: the install step already rebuilds it (`installer.go:5750`), and a stale cache is the defect recorded in `themes/README.md:592`. |
| Neovim | No — a running editor keeps its colorscheme | No — only `:colorscheme` in that session reaches it | `:colorscheme <name>` | `dotfiles-nvim/nvim/lua/plugins/colorscheme.lua` is read at startup. |
| tmux | Yes, for a server that is already running | Yes: `tmux source-file <config>`, only after `tmux list-sessions` succeeds | `tmux source-file ~/.tmux.conf` when no server is running | tmux re-reads its config on `source-file`; it does not watch the file. |

## What changed

- `installer/internal/tui/installer.go`: `themeReloadTool` (the per-tool line), `reloadThemeTools`
  (one line per installed tool, in the artifact table's order) and `reloadThemeToolFor` (the per-tool
  knowledge). The reload runs only the safe, idempotent commands: `bat cache --build` with
  `BAT_CONFIG_DIR`, `tmux source-file` after `tmux list-sessions` succeeded, and `kitty @ load-config`
  when `KITTY_LISTEN_ON` is set. Every command runs through one runner with a five-second timeout.
- `installer/internal/tui/update.go`: the apply and undo commands collect the list off the update
  loop and log every line; the result handler puts it on the picker in the existing result view, so
  each tool gets its own row and the list has the whole body.
- The screen after a switch (and after an undo) is one line per tool: `✓` needs nothing more, `→`
  names the exact action. "Applied" no longer stands alone.

## What was deliberately not done, and why

- **No signal to a terminal.** Ghostty reloads on `SIGUSR2` and Kitty on `SIGUSR1`, but sending
  signals to the process the installer is running inside is exactly the class of action the rule
  forbids, and getting the wrong pid is unrecoverable. Kitty's remote-control command is used only
  when the installer knows which socket it is on; Ghostty is documented, not signalled.
- **No touch of a live Herdr.** It is a running multiplexer; a reload is a keypress in that session.
  It is named, never signalled or scripted.
- **No `tmux source-file` without a server.** It would start a server. The command runs only after
  `tmux list-sessions` succeeds.
- **No restart, no `kill`, no `wsl --shutdown`-style action, no plugin manager run.** Anything that
  ends a session is a user action, named on the screen.

## Red first, and the teeth

- **RED:** `TestApplyingAThemeListsWhatWasReloadedAndWhatIsLeft` (in `util_screen_test.go`) drives a
  real switch into a temporary home and asserts the picker names Alacritty, Neovim and tmux and says
  what Neovim (`:colorscheme`) and tmux (`source-file`) still need. Observed before the change:
  `after a switch the picker is in review=false done=false, want the per-tool reload list`. It uses
  only pre-existing symbols, so it failed on behaviour, not on a missing name.
- **GREEN:** the same guard passes; the 93 theme/utilities guards are green.
- **Teeth:** `TestReloadThemeToolsRunsOnlyTheSafeCommands` observes the exact commands and refuses any
  containing `kill`, `pkill`, `shutdown`, `reboot`, `poweroff`, `restart`, `reset`, `SIGUSR`,
  `SIGTERM`, `SIGKILL` or `wsl --`; it also pins `BAT_CONFIG_DIR` and the manual lines.
  `TestReloadThemeToolsDoesNotStartATmuxServer` proves no `source-file` is sent without a server.
  `TestThemeReloadRunnerTimesOut` proves a `sleep 30` is killed and returns an error, so a hung tmux
  or bat cannot hang the screen.

## Validation

- `make check` - PASS (see the handoff for the exact commands and results).
- `go test ./... -count=1 -timeout 30m` in `installer/` - see the handoff.
- No commit, no push.
