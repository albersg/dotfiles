# Change theme: the utilities row opens a level, and the list lives inside

Follow-up to [`unified-themes.md`](unified-themes.md) piece B/C. The user's ask, in their words:
*"que la utilidad sea cambiar de tema, y dentro de ahí, si le das, tener los diferentes temas y
aplicar ahí el cambio"* — the Utilities section listed the theme rows directly; the themes now live
one level in.

## What changed

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

## The list is derived, never typed

`ScreenThemePicker`'s options are `m.dotfilesThemeOptions()`, which iterates `m.DotfilesThemes` and
keeps `def.Complete()`; `offeredThemeIDs()` is the same rule. No theme name, count or order is written
in the picker.

## Guards

- **Screen coverage.** `ScreenThemePicker` is measured by `screensTheInstallerStatesNeverReach` and
  `terminalFitCases` (the pinned count went 54 → 55), and its own case is
  `themePickerFrameCase` — definitions read from the repository, a record to undo, and the cursor on a
  theme row so the preview row is measured. `TestEveryScreenIsCoveredByAFrameGuard` failed with
  `[ScreenThemePicker]` before the case was added (the RED for the guard).
- **Frame fit.** The picker is rendered at all twelve sizes. Its measured frame is logged by
  `TestThemePickerFrameFitIsMeasuredAtEverySize`, which also asserts the data survives: at every size,
  including 60x20, each theme row, the undo row and the way back are on screen.

## Fit at the small sizes (measured)

`renderThemeScreen` sizes the description to `bodyRows - (title + blank + menu + preview + notice)`; the
menu is never windowed, so when the frame is short the description is what gives way and the rows win.
Nothing cedes at either floor:

| Terminal | Content width | Rendered rows | Rendered columns | Menu rows intact |
|----------|---------------|---------------|------------------|------------------|
| 80x24    | 76            | 24 of 24      | 80 of 80         | yes              |
| 60x20    | 56            | 20 of 20      | 60 of 60         | yes              |

(All twelve: 80x24, 90x28, 100x25, 100x30, 120x24, 120x34, 140x44, 160x50, 200x60, 227x62, 80x30,
60x20 — each renders exactly its terminal.)

## Goldens

**No golden file moved.** `git diff --stat installer/internal/tui/testdata/` is empty and the 20
golden tests pass. The main-menu row is still `🧰 Utilities`, so `TestMainMenuGolden`,
`TestMainMenuWideGolden` and the four companion/creature goldens are byte-identical — the creature's
rows are intact. What moved is the pinned behaviour of the utilities *section*, which is tested in
`update_test.go` rather than snapshotted: the theme rows left the section and the preview tests moved
to `ScreenThemePicker` (`TestThemePickerRowsAreDerivedAndNameExclusions`,
`TestTheThemePreviewShowsTheValuesTheApplyWouldWrite`, `TestThePreviewRepaintsTheWholeInterface`,
`TestThePreviewWritesNothing`). `TestUtilitiesSeesThemesWithoutACloneWhenRunFromTheRepo` now checks
the `Change the dotfiles theme` row on Utilities and then the rows on the picker.

## Not changed

The desktop light/dark switch, its rows and its undo; the theme definitions, generators and ownership
markers; `applyDotfilesTheme`/`undoDotfilesTheme`; the main-menu row and every other screen; no new
dependency.

## Follow-up: PR #164's macOS golden failure, diagnosed and fixed

The `macOS smoke test` went red on `TestCompanionGoldenPinsThePixelSpriteAndItsGaze` with a `--- golden`
diff. It was **not** the theme-picker work and **not** the empty-frame race fixed in `e3ed85c`. It is a
race in the golden infrastructure itself, and it is recorded here because the next person who sees a
`--- golden` on macOS at 3am should find this before questioning their own change.

### Root cause

The OSC terminal-title sequence (`\x1b]2;<title>\x07`) is written when bubbletea handles the
`setWindowTitleMsg` produced by `tea.SetWindowTitle` in `Init`; the first frame is buffered and flushed
by the renderer's own ticker. The two are independent, so under load the title can appear:

1. before the frame (what the pinned golden records),
2. after the frame (the observed failure), or
3. not at all before the capture quits (the `SetWindowTitle` message lost to Ctrl-C).

All three are the same screen. The diff in case 2 is exactly two hunks, both about the title's
position; every frame byte is identical.

### Why the previous fix did not cover it

The golden was **already a frame-waiting golden**: it used an inline `teatest.WaitFor(... "Main Menu")`
whose predicate is byte-for-byte `waitForGoldenFrame`'s. The commit that added the helper said so:
*"The companion goldens already did the equivalent."* So the list of six refactored goldens was
correctly scoped to the ones still on `waitForAnyOutput`; there was no weaker golden left out. Routing
this capture through `goldenTranscript`/`waitForGoldenFrame` was tested and **still flaked**, because
the wait guarantees the frame is present, not that the title arrives before it.

### Evidence

- Real branch (`d65cc89`), 12× `yes >/dev/null`, pixel golden: FAIL 3/40, 3/160, 4/20.
- Base `9093262` (extracted read-only with `git archive`, no branch change): FAIL 3/40, same diff.
- Base + `goldenTranscript`: FAIL 3/40 — the existing remedy does not fix it.
- Base + title normalization: 60/60 pass.
- The six fixed goldens (`Animating=false`) never flaked under the same load; the pixel golden's larger
  first frame is what makes it lose the race most often.

### Fix

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
