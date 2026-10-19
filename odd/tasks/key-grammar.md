# key-grammar — one key grammar for the installer

Issue #375, decided with the owner on 2026-10-19. Every list in the installer says what its keys do, and a key means the same thing on all of them: Enter opens or chooses; Space toggles a switch; every command has its own letter. The work lands as chained work units, one commit per unit.

## Specs

- S1 (owner, verbatim): **"Solo abrir; el resto, tecla propia"** — Enter opens or selects; turning off, starting and validating carry their own named key.
- S2 (owner, verbatim): **"Todas las pantallas, ahora"** — the pass covers every screen now, not only Utilities, and as chained work units rather than one change.
- S3 (issue #375, verbatim): "One meaning for Enter across lists, and a named key for the action that is not "open": Space where a row is a switch, a letter where a row is a command."
- S4 (issue #375, verbatim): "Move keys that cannot collide with the letters an action uses."
- S5 (issue #375, verbatim): "The Utilities section's own list drawn like the lists inside it, since it is the first thing a user sees of the whole area."
- S6 (issue #375, verbatim): "Whether the section's rows are switches, screens or commands, said on the row rather than in a footer that changes shape per screen."

### The grammar, fixed

1. **Enter opens or chooses.** It enters the row's screen, opens the row's detail, or chooses the highlighted option of a picker. It never toggles, starts, applies, writes, retries, validates or confirms.
2. **Space toggles** the highlighted row when that row is a switch. A screen with no switches does not offer it.
3. **A command runs on its own letter**, said on the row and in the footer. No command letter is a move key (`j`, `k`, and `h`/`l` where the screen moves horizontally), and no two commands on one screen share a letter.
4. **Esc goes back**; where a run is in progress Esc stops it, and the screen says which of the two it is doing.
5. **`↑`/`↓` and `k`/`j` move** the cursor; `←`/`→` and `h`/`l` move a value or a choice horizontally.
6. **A text field is not a list**: its letters are its text and Enter commits the field. The password screen's `[Enter] validate` is this rule, not an exception to it.

### What a unit must leave behind

- The footer names exactly the keys the handler accepts, and nothing else.
- Enter is never offered for a toggle, a start, an apply, a write, a retry, a validate or a confirm.
- Space is never offered for anything but a switch.
- A row that acts in place says so on the row: a switch shows its state, a command shows its letter.

## Tasks

| # | Unit | Screens and files | Specs | Commit |
|---|---|---|---|---|
| U1 | The vocabulary, in one place | `view.go` (hints), the guard that pins the constants | S1, S3, S4 | `314b68b` |
| U2 | Utilities drawn like the lists inside it | Utilities section (`model.go`, `update.go`, `panels.go`), the switcher rows | S5, S6 | `d6f8480` |
| U3 | The first list and the two form screens | install review, admin password, install verify | S1, S3 | `ed288f6` |
| U4 | The two machine readers | disk cleaner, network diagnosis | S1, S3 | `17f29ed` |
| U5 | The two managers | port manager, update centre | S1, S3, S4 | `6eb8ccb` |
| U6 | The doctor | doctor, doctor fix | S1, S3 | `64ccb23` |
| U7 | The SSH screens | manager, host, form, keys, agent, diagnostics, test | S1, S3 | |
| U8 | The WSL screens | resources, advisor, benchmark, migration | S1, S3 | `0f27bc6`, `efc91e4` |
| U9 | The remaining lists | shell audit, theme refresh, selection, restore confirm, setup wizard | S1, S3 | `f754934` |
| U10 | The docs | `docs/tui-installer.md`, and any README that names a key | S1–S6 | |
| U11 | The gaps the units could not reach | Escape stops the benchmark and the update centre's apply; the port detail and the setup switch name their keys | S1, S3, S4 | `6c7ef66`, `ed406d4` |

Out of scope by design: the Vim Trainer's own screens keep their game keys — `docs/vim-trainer-spec.md` is their spec, and its keys are the lesson.

## Log

- L1 (owner, verbatim): "puedes comprobar las issues que hay en el repo albersg/dotfiles, y revisarlas y solucionarlas¿?"
- L2 (owner, verbatim): "REvisa las issues del repo, y ademas, revisa este aviso de nvim lualine: There are some issues with your config. Run :Luali"
- L3 (owner, the two decisions, verbatim): "Solo abrir; el resto, tecla propia" and "Todas las pantallas, ahora".
- L4 (evidence): the hints are declared in `installer/internal/tui/view.go:79-105`; there are 56 key handlers; Enter today means open, select, start, see-the-fix, validate, apply and confirm depending on the screen, and Space is a second Enter on the network diagnosis, the doctor, the update centre and the port manager while being absent from others.
- L5 (decision): a text field commits with Enter because its letters are its content; the alternative would make a typed letter ambiguous with a command.
- L6 (decision): the trainer is excluded — its keys are the game, and its spec already fixes them.
- L7 (decision, U2): a row says what it is on the row itself — `→ label` opens a screen, `[key] label` runs where it stands, `[ ]`/`[x]` is a switch, `← Back` goes back. The end-of-list guard presses the key the row declares, so the mark and the handler cannot drift.
- L8 (decision, U3): the review's run is `[s] start` because starting is not opening; the password field keeps `[Enter] validate` because its letters are its text (rule 6), and no key can be named for typing.
- L9 (decision, U8): the WSL resources screen writes on `[w]` and repairs the guard on `[g]`; the value rows end with `[←/→]` because the arrows, not Enter, adjust them.
- L10 (decision, U11): Escape stops every run it can reach — the doctor's checks, the shell audit's starts, the update centre's apply and the WSL benchmark — and the footer says which of the two Esc is doing. The port manager's detail and the setup screen's switches name the key that runs them ([Enter] and [Space]), so the footer and the handler cannot drift.
- L11 (tradeoff, U11): the WSL advisor's prose at 80×24 is cut once its rows are drawn. The environment and the selected finding's card do not both fit at the frame floor, and a prose floor would window the findings list to a row or two without fitting the card either; the cut is the honest choice, and the frame guards measure it.
