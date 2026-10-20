# Installer fluency and theme legibility

## Specs

S1. "La parte de utilite development enviroment sigue sin ir, se queda pillado." The Utilities row that opens the environment doctor must render, scan and answer; no screen may hang.

S2. "Revisa toda la parte de la fluidez a traves de la interfaz, que este todo bien, homogenizado, con la misma interfaz y estilo." Every screen uses the same skeleton, the same header, the same section rhythm, the same footer and the same key vocabulary.

S3. "no una enumeracion de cosas y marcar cada una o no." A list of toggles is not the default shape: a screen shows what it is, what the state is, and what can be done, with the state legible at a glance.

S4. "en la parte de utilite de actualizar las deps del sistema, no me gsuta que sea una lista larga de cosas que no quepa en pantalla y sea dificil de leer." No screen may require scrolling through a long, ungrouped list; content that does not fit is grouped, summarised and paged, never a raw enumeration.

S5. "que todas las partes sean navehables de una manera visual y facil." Every part is reachable with the same two or three keys, and the screen says which.

S6. "Asegurate que toda slas partes del installer respeten esta estetica TOP ... que todas las partes esten accesibles, no hay ninguna parte que se quede pillada." The same visual grammar reaches every screen, including the ones the user rarely opens.

S7. "con lo nuevo de lazygit el theme, el diff se ve asi (no visible visuallmetne, colores demasiado staruasdos...)" The generated lazygit diff colours must be readable on every offered theme: added and removed lines clearly distinguishable from the background and from each other.

S8. "Solo lo he probado con el theme Nocturne, pero peudes revisar todos." The guarantee is measured for all seven definitions, not only Nocturne.

S9. "Puedes ir haciendo capturas de la interfaz o algo asi, para ver visualemtne lo que esta ocurriendo con cada theme" Evidence is visual: each screen and each theme is rendered and inspected, not assumed.

## Tasks

| ID | Task | Specs | Route | Commit |
|----|------|-------|-------|--------|
| F1 | Reproduce and fix the doctor hang | S1, S6 | inline | `pending` |
| F2 | One screen skeleton: header, sections, footer | S2, S3, S5 | inline | `pending` |
| F3 | Utilities: the system dependencies screen, pinned actions and paged entries | S4 | inline | `pending` |
| F4 | lazygit diff colours legible on all seven themes | S7, S8 | | |
| F5 | Screenshot harness: every screen, every theme, to inspect | S9 | | |
| F6 | Sweep the remaining screens against the skeleton | S2, S5, S6 | inline | `pending` |
| F7 | Docs and guards | S2, S7 | inline | `pending` |

## Log

L1 (the request, verbatim):

> La parte de utilite development enviroment sigue sin ir, se queda pillado. Luego, revisa toda la parte de la fluidez a traves de la interfaz, que este todo bien, homogenizado, con la misma interfaz y estilo, con un buen estilo, un estilo bonito, no una enumeracion de cosas y marcar cada una o no, etc... Tiene que ser muy bonito, y usar una interaze homogenizada por toda el installer de una forma muy bonita. Por ejemplo, en la parte de utilite de actualizar las deps del sistema, no me gsuta que sea una lista larga de cosas que no quepa en pantalla y sea dificil de leer.
>
> Quiero una interfaz bonita, que todas las partes sean navehables de una manera visual y facil, que sea homogenizada, que esta visualmetne bonita y facil de leer, etc...
>
> Asegurate que toda slas partes del installer respeten esta estetica TOP, visualmetne muy bonita, que todas las partes esten accesibles, no hay ninguna parte que se quede pillada, etc...
>
> Tambien por ejemplo, con lo nuevo de lazygit el theme, el diff se ve asi (no visible visuallmetne, colores demasiado staruasdos...), puedes corregir esto? Sol olo he probado con el theme Nocturne, pero peudes revisar todos. Puedes ir haciendo capturas de la interfaz o algo asi, para ver visualemtne lo que esta ocurriendo con cada theme y mejorarlo?
>
> Revisa todas las partes, usa intercom, subagentes, jobs, etc..., todo lo que dispones para paralelizarlo y hacerlo lo mas efectivo y preciso posible.

L2 (evidence, F1). The 27 checks of `doctorChecks()` all answer on this machine in 3.5 s, so the scan was never the hang. Reproduced in a real terminal: the installer runs under `tmux`, `Utilities` → `Diagnose the development environment` draws its rows, and `[a]`, `[c]`, `[r]`, `[t]`, `[w]` and `Esc` all did nothing — the screen could not even be left. Cause: `handleDoctorKeys` and `handleDoctorFixKeys` existed, and the two dispatch switches in `update.go` (keys and Escape) had no case for `ScreenDoctor` or `ScreenDoctorFix`, so every key reached nothing. Fixed by naming both screens in both switches. Guards: `TestTheDoctorScreenAnswersTheKeysItAdvertises` drives the dispatcher (fails on the old code, passes now) and `TestEveryScreenKeyHandlerIsDispatched` walks the package's own AST and fails when a `handle…Keys` handler has no `case Screen…` naming it, so the next screen with a handler and no dispatch cannot ship silently.

L3 (evidence, F4). The screenshot is delta's, not lazygit's: it carries two line-number columns, which is delta's unified gutter, and lazygit 0.66.2 exposes no `diffAdded`/`diffRemoved` key (the default config it prints has none, and the binary carries no such string). The colours came from our generated `[delta]` block: `plus-style = "normal #b7cc85"` puts the palette's green in delta's **background** slot, which was measured as `\x1b[48;2;183;204;133m` on an added line. The block now marks the text (`38;2`), and picks the defined role with the better contrast on the theme's base: bright green and bright red for dotfiles, kanagawa and nocturne; the same value where the palette's two variants are identical. `TestTheDeltaDiffMarksTheTextAndNeverFillsTheLine` holds the rule for all seven definitions and fails if a style regains a background token or picks the weaker variant.

L4 (evidence, F3). The update center put its actions at the end of the same list the packages are in, so the first screenful of a real machine (66 entries across brew and apt) pushed `[s]`, `[r]` and `[f]` past the bottom while the footer still advertised them. Measured in the running TUI under tmux, not only in a golden: after the split the block stays pinned and the header reports `Showing 1-26`. The shared renderer now pins a menu's tail -- the way back everywhere, the action block on the update center -- and windows only the entries, so the same fix reaches every screen with trailing actions. The section also repeated its own name as the body's first line; the body no longer draws a title the header already carries.

The utilities body is still the block of one sentence per row it was; replacing it with the reasons a row is absent is the remaining piece of F2, and the floor guard (`TestUtilitiesBodyCarriesEveryJobAtTheFloor`) is the test that describes the current rule and must move with it.

L5 (evidence, F2). The utilities section's body was one sentence per row -- seven lines at the floor for seven jobs -- and the first line repeated the screen's own name, which the header already carries. The body is now the reasons a job is missing: the desktop switch this host has none of, the theme list being read, the WSL screens this host cannot open, and one line naming the switch when there is one. A row that is on screen names itself and says what it opens, so the prose no longer narrates it. Verified by capture: three reason lines instead of seven, no duplicated title, and the list keeps its rows.

L6 (evidence, F6). All 75 screens were rendered at 120x36 by `TestCaptureEveryScreen`: none panics, none overflows its frame, and after the header rule no screen repeats its own title in the body. The screens whose body is long are the reports -- development caches 18 lines, Learn tooling 16, the WezTerm table 14, network diagnostics 11, the shell's startup 9 -- and their prose is the content being read, windowed and paginated, not decoration under a list: they stay as they are, and a later pass that shortens them is a content change, not a layout one. The captures of the trainer lesson, practice and boss screens are empty states (no exercise loaded), not defects.

L7 (guards, F7). `TestTheUpdateCenterKeepsItsActionsOnScreenAtTheFloor` renders the real screen at 80x24 with forty packages and reads the frame back; with the tail unpinned it fails naming `[s]` and `[r]`, and with the tail pinned it passes. `TestEveryScreenKeyHandlerIsDispatched` walks the package's AST for `handle…Keys` and fails when a screen has no dispatch case. `docs/tui-installer.md` now states the skeleton: the header from the frame, the body for what is missing, the pinned tail, the windowed entries and the footer from the hints.
