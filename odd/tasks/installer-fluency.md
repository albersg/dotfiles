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
| F2 | One screen skeleton: header, sections, footer | S2, S3, S5 | | |
| F3 | Utilities: the system dependencies screen, grouped and paged | S4 | | |
| F4 | lazygit diff colours legible on all seven themes | S7, S8 | | |
| F5 | Screenshot harness: every screen, every theme, to inspect | S9 | | |
| F6 | Sweep the remaining screens against the skeleton | S2, S5, S6 | | |
| F7 | Docs and guards | S2, S7 | | |

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
