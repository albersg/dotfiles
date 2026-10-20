# ODD Feature Tasks: Installer interface cohesion

One interface on every screen: stable regions that never move while the cursor moves, the same layout
grammar everywhere, a main menu that reads as designed, and the Vim entries grouped under one row.

## Specs

**S1 — Stable regions.**
> en utilieites, por ejemplo, en el cache (pero aplicable en todas las partes), cada vez uqe ovy cambiando de opcion, como el tamaño del texto cambia, la interfaz se va cambiando moviendo    │
> de un lado a otro, no me gusta, quiero que la interfaz sea esatble, no se este subienod / bajando en funcin de la opcion que tengo seleccionada.

Regional heights are decided by the frame and a per-screen constant, never by the selected row,
the selected row's detail, a notice's length, or an action that appears with a selection. The title,
the first list row, the action tail (`← Back` included) and the notice keep their row across every
cursor position, selection state and notice.

**S2 — One layout grammar.**
> siento que no sigue todo la misma interfaz el mismo estilo. Además, no veo la interfaz general del installer mejor, ni mejorada,

Every content screen starts its body on the first row under the header rule (the welcome lockup keeps
its centred composition), and shares one stack of bands: title, detail, list, notice. A screen whose
list is the subject shows the focused row's detail in the right-hand column when the frame has room
for two columns, and in the detail band when it does not.

**S3 — The updates screen.**
> la interfaz de las updates esta mejor ahora, pero sigue un poco sin mas, se podria majejorar mas. como esta ahora: /tmp/pi-clipboard-c2e7d12e-c9a5-48d9-a217-3e5349a64e42.png

The update center reads as a table: one aligned row per entry (mark, name, installed → available,
manager · scope), a fixed action bar, and the focused entry's complete facts in the detail pane or
band. A version that does not fit is cut with the shared marker, and the focused entry's full values
are always readable.

**S4 — The Vim entries are grouped.**
> Tabmien, la parte de vim, voe que esta separada en varias opciones del menu principal, se podria agrupar

The main menu offers one `Vim & Neovim` row; it opens a hub holding the Vim Trainer, the Neovim
keymaps reference, the LazyVim guide and the Neovim page. The three separate rows are gone.

**S5 — The main menu.**
> el mun principal: /tmp/pi-clipboard-97bcf688-6a6b-4a21-ab28-7877e6dd050c.png, no se puede mejorar, no es mejorable? no se puede hacer mucho mejor, mucho mas bonito

The main menu is a masthead (greeting, question, version and channel in the header's vital) over
three titled cards — Install, Learn, Session — beside the context panel, with the companion where it
already lives.

**S6 — No regression.**
Key grammar holds (a footer names a key only while its row is offered; Enter opens, letters act);
no fact is invented; every screen fits the terminal at the documented floor and at 12 measured
sizes; trainer snapshots do not move.

## Tasks

| ID | Spec | Route | State | Commit |
|----|------|-------|-------|--------|
| T1 | S1, S2 | inline | done | 96cb6ce |
| T2 | S2, S3 | inline | done | recorded in T3 |
| T3 | S3 | inline | done | 3e10e64 |
| T4 | S4, S5 | inline | done | d821755 |
| T5 | S1–S6 | inline | done | this commit |

- **T1 — Stable bands.** Add the band contract (`section_bands.go`), rewrite `renderThemeScreen` to
  draw fixed bands, reserve footer rows from the screen's full hint set, anchor the action tail to
  the bottom of the list band, move the notice to the bottom of the body, and top-anchor every
  installer body except the welcome lockup. Guard: band heights are a pure function of screen and
  frame; rendering two cursor/selection states keeps the title, first entry row, action rows and
  notice on the same rows.
- **T2 — Detail column.** Register the section detail panel for the screens whose list is the
  subject (utilities, update center, disk cleaner, theme picker, doctor, port manager) and draw the
  same paragraphs in the detail band below the two-column floor.
- **T3 — Updates table.** Align the entry rows, add the focused entry's facts (full versions,
  manager, scope, note, pin reason, read command) and shorten the standing prose.
- **T4 — Main menu and Vim hub.** Grouped main-menu cards with the version/channel vital; new
  `ScreenVimHub`; the three Vim rows collapse into one; navigation skips group separators.
- **T5 — Checks and docs.** Regenerate goldens deliberately, update `docs/tui-installer.md` and the
  README's menu description, run `make preflight`, and record the evidence here.

## Log

**L1 — the request (verbatim).**
> en utilieites, por ejemplo, en el cache (pero aplicable en todas las partes), cada vez uqe ovy cambiando de opcion, como el tamaño del texto cambia, la interfaz se va cambiando moviendo    │
> de un lado a otro, no me gusta, quiero que la interfaz sea esatble, no se este subienod / bajando en funcin de la opcion que tengo seleccionada.                                             │
>                                                                                                                                                                                             │
> la interfaz de las updates esta mejor ahora, pero sigue un poco sin mas, se podria majejorar mas. como esta ahora: /tmp/pi-clipboard-c2e7d12e-c9a5-48d9-a217-3e5349a64e42.png                │
>                                                                                                                                                                                             │
> siento que no sigue todo la misma interfaz el mismo estilo. Además, no veo la interfaz general del installer mejor, ni mejorada,                                                             │
>                                                                                                                                                                                             │
> Tabmien, la parte de vim, voe que esta separada en varias opciones del menu principal, se podria agrupar                                                                                     │
>                                                                                                                                                                                             │
>                                                                                                                                                                                             │
> el mun principal: /tmp/pi-clipboard-97bcf688-6a6b-4a21-ab28-7877e6dd050c.png, no se puede mejorar, no es mejorable? no se puede hacer mucho mejor, mucho mas bonito

**L2 — measured movement (before).** `TestScratchMovement`, update-center fixture, one selection
toggled off: 80x24 `libc6` 10 → 13, `← Back` 17 → 18, and the footer grew from 1 to 2 rows;
200x55 `libc6` 11 → 10, `← Back` 18 → 15. The two causes are exactly the two the contract removes:
the body is centred in whatever height it happens to have, and the pinned block's height follows
the selection.

**L3 — T1 landed.** `section_bands.go` holds the contract: `sectionBandsFor` reads
the screen, the frame, the wrapped detail count and the menu length, and nothing
else; `pinnedTailLen` pins the action block; `footerReserveRows` reserves the
hints a selection can add; `noticeBand` reserves the result line; and every
section body is padded to `Body` rows, so the frame's centring never moves it.
Goldens regenerated deliberately (41 golden tests); the welcome lockup, the
trainer and the installing screen did not move.

Measured after, with the update-center fixture and the cursor walked over every
row plus the selection toggled: `libc6` and `← Back` now sit on the same rows in
every state at 80x24, 120x36 and 200x55 (before: `libc6` 10 → 13 at 80x24 and
11 → 10 at 200x55, `← Back` 17 → 18).

Two defects were found and fixed by the new guards rather than by the product
report: (a) the reserve probe mutated the live disk-cleaner selection map from
the render path, which the drift guard caught as a one-line change of the card's
cut count (`TestBackgroundDriftS5`); (b) the picker's bands read the cursor
through `previewThemeDef`, so the preview row is now reserved on every picker
state (`TestSectionBandsReadNeitherCursorNorSelectionNorNotice`).

**L4 — T2 landed.** On a frame with room for two columns (`layoutFor` >= 120
content columns) the screens whose detail is context for a list -- the utilities
hub, the update center, the disk cleaner, the doctor and its confirmation -- draw
their prose in a `Details` column (a `Notes` column on the hub, `The change` on
the confirmation) and give the whole body to the list. Below the floor the same
prose is the band above the list, unchanged. The report screens keep the
full-width body because their prose is the report.

The pane is assembled through `panelBody` and `composeColumns`, the same pair the
main menu's panel uses, so the chip, the rule and the cut marker are shared. The
guard now checks the band contract in both modes: `sectionBandsFor(..., true)`
must be cursor-, selection- and notice-independent too. Goldens and all 1738
frame-fit assertions are green unchanged.

**L5 — T3 landed.** The update center's entries are a table: `[x] name` in a
name column, `installed → available` in a version column, then the owner, each
cut inside its own column with the shared marker, so a long Debian version can
no longer push the owner off the row. The row is built to the list's measure
minus the two cursor columns, and the resolver that maps a row back to an entry
uses the same measure, so the drawn rows and the pressable rows are one list.

The prose now opens with the entry under the cursor: Name, Manager, Installed,
Available, Held, Note and the command the manager's answer was read with, in
full. The standing explanation shrank to two lines. The header vital carries the
selection (`Showing 1-2 of 2 · 1 selected`).

Two consequences found by the suite, both fixed at the right level: (a) the
disk-cleaner tests read a phrase from the whole frame, and with the detail in the
right column the phrase was interleaved with the list -- the helper now reads the
detail column on a two-column frame and a second helper reads the whole frame for
header and footer assertions; (b) a path is one token, so the card now prints
`On disk:` and the paths on the next line instead of breaking a directory name
mid-token when the column is narrower than the path, and its guard compares the
path with the spaces removed, because where the wrap lands is the column's
business.

**L6 — T4 landed.** The main menu is a masthead (greeting and question) over one
card with named dividers -- the first group under the card's own `dotfiles`
title, then `Learn` and `Session` -- and the header's vital carries
`VersionLabel()` (`dev build` on a source build). The options are three groups in
`GetCurrentOptions`, separated by the same rules every grouped menu uses, and the
cursor steps over them.

The three Vim rows are one: `Vim & Neovim` opens a new `ScreenVimHub` whose rows
are the trainer, the Neovim keymaps, the LazyVim guide and the Neovim page, with
the way back under one rule. The hub answers the context panel too: Modules,
Lessons, Bosses, Keymaps and Topics, the counts the three subjects hold.

Keys: `v` was tried as a hub shortcut and removed -- the main menu's `vim`
easter-egg form owns every `v` prefix, so the key never reached the handler. The
end-to-end tests isolate `HOME` (so the runner's own backups cannot insert the
Restore row) and walk the real rows instead.

Guards and tests moved with the contract: the panel test resolves its rows by
label instead of index, the companion's destructive-row table derives the Restore
row from the option list, the DD gag is drawn through the same `menuRowAt` the
grouped card uses (a gag only one renderer honoured would vanish from the screen
it belongs to), and the main-menu, companion and truecolor goldens were
regenerated deliberately. `go test ./internal/tui -skip Golden` is green.

**L7 — T5 landed.** The docs say what the menu is now: three groups under one
card with the build in the header, one `Vim & Neovim` row opening a hub, and the
keymaps reference one level in from it. The panel table names the hub's facts.
`make preflight` passed 7/7 (gofmt, vet, build + `--help`, `go test ./...`,
shellcheck, branding, gitleaks), so the full suite including every golden is
green.

**Remaining.** Nothing open. The branch is `feat/installer-interface-cohesion`;
the merge is the maintainer's decision. Two things this front deliberately did
not do: the `v` shortcut for the hub (unreachable behind the `vim` easter-egg
prefix) and a repo-wide sweep of the remaining section screens that keep a
full-width body (the report screens, the SSH screens) -- their bands are stable,
but they have no detail column because their prose is the screen.
