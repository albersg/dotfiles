# The WSL offer as a change table

The Windows Terminal offer the interactive WSL step puts mid-install is read as a
table of what changes, not as prose. Every fact the current sentences carry is
kept; the sentence form is what goes.

## Specs

S1. The offer's body is a change table, not a paragraph. The user approved this
layout verbatim:

```
 Configure Windows Terminal?

   settings.json   found

   font.face       Cascadia Mono  >  MesloLGS NF
   colorScheme     One Half Dark  >  dotfiles

   keeps           every other key and comment
   backup          .bak-dotfiles-<stamp> first
   also            the Nerd Font is registered
                   under HKCU

   [y] write   [n] leave it alone
```

S2. The offer still names the `settings.json` it would change. `describeWindowsTerminalOffer`
and the rendered frame are both held to it.

S3. The value each setting holds today is named on the same row as the value this
installer would write, opposed by the transition column: `old  >  new`. When the
file names no value for a key the row says `(unset)`, which is accurate exactly
because an available offer proves the file was read and parsed.

S4. The backup and the promise about the rest of the file are still stated.

S5. The registration of the font under HKCU is still stated. It is the one
effect of the choice that does not live in `settings.json`.

S6. The trailing sentence `Press y to write these settings, or n to leave Windows
Terminal exactly as it is.` is dropped. The installing screen's own hint already
reads `[y/n] configure Windows Terminal or leave it unchanged`, so the sentence
spent a row to repeat the footer.

S7. No body row is a full sentence: every row is a label and a value. This is the
requirement the user actually stated ("mucho texto", "el formato"), and it is what
S1's layout encodes.

S8. The panel fits the terminal it is drawn on. At 60 columns -- the tightest size
the frame guards use -- no rendered row is wider than the frame, and the settings
path is still present in the frame. No test covered this panel's width before this
front; the guard is added here because the change is what makes it worth having.

S9. Both paths that describe the offer keep describing it the same way.
`describeWindowsTerminalOffer` is shared by the interactive offer and the
non-interactive notice, so the notice is not left carrying the old sentences.

## Tasks

| ID | Specs | Route | Commit |
| -- | ----- | ----- | ------ |
| T1 | S8 | The panel's width guard, plus the table's row expectations, written first and observed RED | the commit that carries this front |
| T2 | S1-S7, S9 | `describeWindowsTerminalOffer` and `wslTerminalOfferLines` rewritten as the table; the label helper went with the sentence it existed for | the commit that carries this front |
| T3 | S1-S8 | Full unfiltered suite; `docs/tui-installer.md` and `CHANGELOG.md` | the commit that carries this front |

## Log

L1 (user, verbatim): "y tambien, el texto y la forma que sle el aviso de que si se
quiere instalar la font, no me gusta ni me convence, el fomrato, mucho texto...,
podemos mejorar esto?"

L2 (user, verbatim, correcting where the text is): "yo me refiero a un texot que
sale en el start installation a medias de la installation"

L3 (user): chose the diff-style change table from three layouts offered.

L4 (finding, not a requirement): the panel that draws this offer had no test and
no frame guard for its width. The offer is the screen that decides whether a
user's settings.json is edited, so a row clipped past the frame edge hides which
file is at stake. S8 exists because of this, not because the user asked. The line
the offer draws today is 7 sentences deep and its longest is 200 characters.

L5 (finding): the offer is only *available* after the settings file was read and
parsed (`readWindowsTerminalOffer` returns unavailable on either failure), which is
what makes S3's `(unset)` honest rather than a guess.

L6 (measured, before the table existed): the panel's content width is `contentWidth
- panelFrame - panelPad*2`, so **52 columns at 60x20** and 72 at 80x24. Its content
rows are `bodyRows - len(body) - panelFrame`, which at 60x20 is **8 rows** and at
80x24 **13**. The settings path in the guard's own temporary profile is 149
characters, which is **3 wrapped rows** at 52 columns. So the table has room for the
path plus **five fact rows** at the tightest measured terminal, and that is the row
count the implementation holds.

L7 (measured, RED before GREEN): with the paragraph still in place, the new guard
failed at 60x20, 80x24, 90x28, 100x25, 100x30, 120x24, 120x34, 140x44, 160x50 and
80x30 -- at 80x24 and below because the sentences naming the value a write replaces
had already fallen out of the panel. After the rewrite the same guard passes at all
twelve measured terminals. The first version of the guard's own assertion was wrong,
not the code: `settings.json` can sit either side of a wrap (`.../sett` and
`ings.json`), so the assertion had to tolerate the newline and padding the panel
itself inserts.

L8 (deliberate deviation from the approved layout, L3): the approved preview opened
with a `settings.json  found` row. It is not in the implementation. The path is read
whole rather than clipped -- a decision the renderer already documents -- and its last
wrapped row already ends in `settings.json`, so the row restated the file name the
row above it carries, at the price of one of the five rows the panel has at 60x20.
L3's layout is otherwise followed as approved, including the transition column and
the `backup`, `keeps` and `also` rows.

L9 (deviation from the approved layout, L3): the approved preview ended with
`[y] write   [n] leave it alone`. That row is not in the implementation either: the
installing screen's hint already reads `[y/n] configure Windows Terminal or leave it
unchanged`, and S6 is exactly the rule that a row restating the hint is the row that
should go. That hint stays asserted by
`TestInteractiveWSLStepShowsTheOfferOnTheInstallingScreen`, so the deletion cannot take
the answer with it.

L10 (measured, found by CI and not by any local run): on macOS the path a test is
handed lives under `/var/folders/...` and carries the test's own name, so it is **197
characters** where the Linux one is 149 -- four wrapped rows at 52 columns instead of
three. The table's eight content rows are the whole budget, so the fifth fact row fell
off the bottom of the panel and `TestWindowsTerminalOfferFitsTheFrameItIsDrawnOn/60x20`
failed in the `macOS smoke test` job while every local run passed. S4's two promises are
one row now -- `.bak-dotfiles-<stamp>, rest kept` -- because a backup taken and nothing
else lost is a single promise about the rest of the file, and that is the third
deliberate deviation from L3's layout. The alternative considered and rejected: freeing
a row by not drawing the installing screen's `Estimating the time remaining…` row while
a question blocks the run. It reads well, but it reaches into the composition every
screen shares to buy one row, where the merge stays inside the two functions this front
already owns.

L11 (the capacity guard's teeth, demonstrated rather than argued): with the fifth row
put back, `TestWindowsTerminalOfferFitsThePanelAtItsTightestBudget` fails with "the
offer's rows take 9 of the panel's 8 content rows at 60x20, so its last row is cut off
with nothing saying which fact went". The frame guard does **not** fail on Linux in the
same state -- with a three-row path the fifth row still fits -- which is the whole reason
the capacity guard exists: the frame guard was measuring the platform's temporary
directory as much as the panel.

## Remaining

Nothing. The front is closed: T1-T3 landed and the suite is clean apart from the four
Ghostty failures that predate this work. The three deliberate deviations from the
approved layout (L8, L9, L10) are stated above rather than left to be found in the diff.
