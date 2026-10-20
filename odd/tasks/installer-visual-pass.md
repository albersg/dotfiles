# Feature: the installer's visual pass

Status: done (stacked on PR #387; the update-center column header and the vertical composition of short pages are recorded as remaining)
Opened: 2026-10-20
Owner: autonomous session, branch `feat/installer-visual-pass` (stacked on
`feat/installer-interface-cohesion`, PR #387)
Review tool: VHS (`/tmp/review`, `make screens` once #388 lands)

## Why

The previous front made the interface stable: regions no longer move when the
cursor, the selection or a notice changes. Looking at the result through VHS --
the first time the interface was judged from real pixels rather than text frames
-- that stability is not enough. The main menu and the welcome are designed; the
rest of the program is not.

## Specs

S1. **Every screen reviewed with VHS.** The user's words: "revisa toda la
interfaz, usando vhs". The captures are welcome, main menu, Vim hub, Learn hub,
learn terminals, Utilities, theme picker (dark and light previews) and the update
center (empty and settled). The screens that need host state or a run are read
from the code and share the grammar of the screens captured.

S2. **A much prettier interface.** The user's words: "mejorarla muchísimo, mucho
ma bonita". The main menu's language -- a title block, a bordered card for the
choices, a contextual panel on the right, the companion in the spare rows and a
footer that names only live keys -- becomes the language of every page.

S3. **One grammar across the installer.** The user's words: "que este
homogenizada por todo el installer". The header carries the section and the build
on every screen; every body opens with a title block; menus draw a card and a
contextual panel; lists carry one row-and-cursor grammar; the footer keeps the
one hint notation.

S4. **The inconsistencies VHS found are fixed.** The user's words: "revisa
inconsistencias". In the order they were seen:
  1. The build label (`dev build`) is on the main menu and on no other screen.
  2. The Utilities hub has no title block, no grouping and no contextual panel;
     its right column says `Notes` and repeats the section's caveats.
  3. The Utilities rows carry a `→` mark no other menu uses, and the main menu's
     rows do not.
  4. The hubs (Vim, Learn, keymaps, LazyVim) are bare lists: no card, no panel,
     no companion -- though the main menu gives the row that opens them a full
     panel.
  5. The Vim hub advertises `[Esc] back` and Escape does nothing: the global
     dispatch has no `ScreenVimHub` case, so the hub's own `case "esc"` is
     unreachable. Confirmed in code, not only in the capture.
  6. The theme picker never says which theme is applied, and its intro runs the
     full 200-column width (~180 characters a line).
  7. The theme picker's light preview draws muted prose at ~3.7:1 on the
     drift-tinted base (4.36:1 on the plain `#eff1f5`): nearly unreadable.
  8. The update center's table has no column header row, and the screen has no
     body title: the header says `Utilities`, then the table starts.

S5. **Nothing moves.** The inherited rule holds: no region of a screen may move
because the cursor, the selection or a notice changed.

S6. **Every hint is true.** The inherited key grammar holds: a key is named only
while it acts; Escape goes back; Enter opens or chooses.

## Tasks

| ID | Spec | Route | What it does | Commit |
| --- | --- | --- | --- | --- |
| T1 | S2 S3 S4.1 | inline | The page head: the section body title + the build label in every header vital, in the muted tone so a light preview cannot leave it white on white. | 4c3717f |
| T2 | S2 S3 S4.2 S4.3 | inline | Utilities: body title "The machine's own settings", the caveats in the body, and the `At a glance` status board in the right column. The arrow marks and the list grouping were left as they are: the arrow is the section's affordance that a row opens a screen, and removing it would be a separate, test-carrying change. | 4c3717f |
| T3 | S2 S3 S4.4 | inline | Hubs: `renderMenu` now draws the card + a contextual `About` panel; the Learn hub's two sections are named on the card's border and divider; the panel lists the destination's own content (trainer modules, keymaps per tool and category, terminals/shells/prompts, plugin sets, directives). | 4c3717f |
| T4 | S4.5 | inline | Vim hub Escape reaches the global dispatch and returns to the main menu. | 4c3717f |
| T5 | S2 S4.6 | inline | Theme picker: `The palette` panel (swatch + role + hex, applied theme named), the intro wrapped to the left column and rendered in the theme's ink, the list's actions following the list. | 4c3717f |
| T6 | S4.7 | inline | Light-preview contrast: `subtext1` for the muted role on a light base, the drift tint capped at the muted role's floor (4.5:1 where the plain base clears it, 3.0:1 for the deliberately dim themes), and the guard `TestPreviewMutedRoleStaysLegibleOnTheDrift` over every offered theme. The first cap required no loss at all and flattened the background on Nocturne; the floor keeps the motion. | 4c3717f, fix/drift-and-doctor-keys |
| T7 | S4.8 | inline | Update center: the body title was the empty default; it now returns "Updates". The table's column header row is a follow-up (see Remaining). | 4c3717f |
| T8 | S1 S5 S6 | inline | VHS captures at 200x55 (`90-vim-hub-card`, `91-learn-hub-card`, `92-utilities-status`, `93-theme-picker-panel`, `02-main-menu`) and at 80x24 (`94-vim-floor`, `95-learn-floor`); the non-golden suite, the goldens and the new guards. | 4c3717f |

## Log

L1 (user, verbatim): "revisa toda la interfaz, usando vhs, revisa la interfaz y
vamos a mejorarla muchisimo, mucho ma bonita, que este homogenizada por todo el
installer, revisa inconsistencias, etc..."

L2 (evidence, 200x55 real WSL, branch `feat/installer-interface-cohesion`):
welcome and main menu are designed; Vim hub / Learn hub are four-row lists over
~30 empty rows with the companion alone at the foot; Utilities is an 11-row flat
list with a `→` per row and a sparse `Notes` column; the theme picker previews
Catppuccin Latte with unreadable muted prose and a dark header chip; the update
center is a headerless table with a generic `Details` column. See `/tmp/review/`
(01-welcome, 02-main-menu, 20-vim-hub, 22-learn-hub, 25-utilities,
31-theme-picker, 70-update-center).

L3 (code, 2026-10-20): `handleEscape` (update.go:2095) switches on the screen and
has no `ScreenVimHub` case, so the global `esc` dispatch consumes the key before
`handleVimHubKeys` can; the footer promises a key that does nothing.

L4 (code, 2026-10-20): `renderMenu` (view.go:3781) is the shared hub renderer:
title, one muted line, a blank, then the windowed rows. It draws no card, no
panel, no companion, and `frameWithPanels` composes no panel because
`panelsFor` lists only welcome, main menu and the wizard. The hubs' bareness is
this one function, which is why T3 can change every hub at once.

L5 (code, 2026-10-20): the Utilities two-column pane is `panelBody("Notes",
paragraphs, ...)` in `renderThemeScreen` (view.go:1736): the same section caveats
the detail band would print, not context for the focused job.
`utilitiesPanelEntries` (panels.go:1094) already derives a status row per
utility for the main menu's section panel; the section screen can reuse it.

L6 (implementation, 4c3717f): the hubs now draw `panelBlockGrouped` and a
registered `hubPanel`; `panelsFor` lists every `renderMenu` screen so the card
and the panel cannot diverge; the utilities register `utilitiesStatusPanel` and
the theme picker `themePalettePanel`; `renderThemeScreen` defers the panel to
the frame when the registry has one, and the action tail follows the entries
while the band keeps its reservation. `frameVital` puts the build on every
header and `headerRowStyle` gives a plain vital the muted tone.
`inkParagraphs` gives unwrapped prose the theme's ink, which is what made the
theme picker's intro legible on a light preview (it was drawn in the terminal
default). `driftTintCap` samples the base-to-accent blend and stops before the
muted role's floor; the guard measures every theme and fails if the cap reaches
zero, because a zero cap is a background that no longer moves.

L7 (visual evidence, 2026-10-20): `/tmp/review/90-vim-hub-card.png` (card, About
panel with modules), `91-learn-hub-card.png` (named groups, page list),
`92-utilities-status.png` (body title, caveats, At a glance), 
`93-theme-picker-panel.png` (palette board, dark intro, actions beside the list),
`94-vim-floor.png` and `95-learn-floor.png` (80x24). The main menu and the welcome
keep their shape; the build label and the muted vital are the only changes there.

L8 (regression report, user, 2026-10-20): "me has quitado el fondo dinamico?"
The no-loss drift cap was too strict: Nocturne's muted role was 7.07:1 and the
first 0.0063 tint took it to 7.04:1, under the 7.05:1 no-loss threshold, so the
cap was zero and the gradient was flat. The cap now targets a floor (4.5:1 when
the plain base clears it, 3.0:1 for the dim themes), and the guard fails if the
cap is zero. The doctor's category rows also stopped repeating `[c]`: it acts on
the cursor, so it is named once in the footer.

## Remaining

- **The update center's column header row.** The table's columns (state, package,
  installed and available, manager) are readable from the values but not named.
  A header needs a fixed manager column so every row's name/version widths stop
  depending on the manager tag; that is a small table refactor with its own tests.
- **The vertical composition of short pages.** At 200x55 the hubs, the utilities
  and the theme picker still end in the top half; the companion and the panels
  give the lower area some weight, but the page does not fill it. A fixed-height
  block that is stable under the cursor (or a decoration sized to the frame) is
  the next visual question, not a defect of this pass.
- **The pet on work screens.** The companion draws where the body leaves spare
  rows; the section screens fill their frames, so they show none. That is the
  rule now (home and menu pages show it, work pages do not) and the doc records
  it, rather than a screen-by-screen accident.
