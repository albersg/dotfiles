# Developer disk cleaner

A Utilities-section job that finds what development caches, downloads, build artifacts, dependency
trees, container stores and volumes are really holding, reports each one with its path and its
reason, measures what is reclaimable without counting the same bytes twice, and removes only what
the user has selected and previewed — through the tool's own command when one exists, and through a
guarded file removal when none does. The domain logic lives in `installer/internal/diskclean`; the
screen lives in the Utilities section.

## Specs

- **S1 — Detection is evidence-based.** Quote: *"Descubrir automáticamente herramientas/ecosistemas
  realmente presentes y sus cachés mediante mecanismos oficiales y rutas configuradas, no por
  supuestos arbitrarios."* An ecosystem is offered only when the tool is on `PATH`, or its
  environment variable names a store, or its configured store directory exists; every item carries
  the `Reason` that detection used. Ecosystems covered: *"JS/TS, Python, Rust, Go, ecosistemas JVM,
  gestores del sistema, contenedores"* plus the project artifact directories under directories the
  user adds voluntarily. An ecosystem that cannot be detected reliably is not offered.
- **S2 — Categories and risk.** Quote: *"Diferenciar cachés regenerables, descargas, artefactos de
  compilación, dependencias de proyectos, imágenes/contendedores y volúmenes persistentes.
  Clasificar riesgos y no presentar todo como basura segura."* And: *"Categorías delicadas
  (dependencias de proyectos, contenedores, volúmenes, datos persistentes) jamás preseleccionadas."*
  Default selection is empty; the "select the safe ones" key never selects a careful or delicate
  item.
- **S3 — Measured, grouped, honest units.** Quote: *"Mostrar espacio usado y espacio potencialmente
  recuperable con unidades legibles, agrupación por herramienta/proyecto, rutas u objetos de origen,
  motivo de detección y grado de certeza. Aclarar la diferencia entre tamaño lógico, espacio físico
  y estimación; evitar doble cómputo cuando se solapen rutas o almacenes compartidos."* One scan
  shares one device/inode set across every root, so a hardlink or an overlapping root is counted
  once; an item whose paths overlap another's is reported as overlapping rather than added again.
- **S4 — Bounded scope.** Quote: *"Exploración con alcance controlado; permitir añadir directorios de
  proyectos voluntariamente. No rastrear todos los discos, volúmenes montados o datos del host desde
  WSL por defecto."* Only known store paths and the explicitly added project directory are walked;
  the walk never crosses a filesystem boundary and never follows a symlink.
- **S5 — Selection, preview, confirmation.** Quote: *"Selección granular, vista previa/dry-run y
  confirmación explícita."* And: *"Dar explicaciones claras sobre qué podría necesitar volver a
  descargarse o reconstruirse."* Pressing clean shows the dry-run plan — the exact command or path
  per item, the bytes each is expected to free, the risk and the regeneration cost — and nothing
  runs until that plan is confirmed.
- **S6 — Official mechanism first.** Quote: *"Preferir el mecanismo de limpieza oficial de la
  herramienta correspondiente; nunca borrar directamente almacenes internos gestionados por Docker o
  gestores de paquetes cuando exista una API/comando de mantenimiento soportado."* Container stores
  are only ever reached through `docker`/`podman`, system package caches only through
  `apt-get`/`dnf`/`pacman`/`pkg`/`brew`.
- **S7 — Guarded direct removal.** Quote: *"Para borrado directo de archivos: verificar rutas y
  pertenencia a un ámbito aprobado justo antes de ejecutar, controlar symlinks, junctions, montajes,
  loops, hardlinks y carreras TOCTOU; bloquear raíces del sistema, `HOME`, repositorios, `.git`,
  archivos de configuración, credenciales y rutas ambiguas."* Every removal re-checks, after the
  user's confirmation and immediately before unlinking, that the path is inside its approved root,
  is not a symlink leaf, is not a mount point, has the same device identity it had when measured,
  and is not a blocked root.
- **S8 — Results after the run.** Quote: *"Actualizar resultados tras la ejecución y mostrar
  liberación real cuando pueda medirse, fallos parciales y tareas no realizadas. Si hay cuarentena o
  papelera, explicitar qué acciones son recuperables y cuáles no."* Freed bytes are re-measured the
  same way they were measured before, per item; partial failures and skipped entries are listed, not
  swallowed. There is no quarantine: the screen says which removals are irreversible and what
  regenerates them.
- **S9 — WSL.** Quote: *"Dentro de WSL distinguir la liberación de espacio en Linux de la reducción
  del tamaño del archivo de disco virtual visto por Windows; no compactar ni manipular discos
  virtuales automáticamente."* On WSL the screen states that freeing Linux space does not shrink the
  virtual disk Windows sees, and no code path touches a `.vhdx`.
- **S10 — Robustness.** Quote: *"Escaneos grandes sin congelar la TUI; cancelación, permisos
  insuficientes, archivos bloqueados, ficheros cambiantes, rutas largas y Unicode, herramientas no
  instaladas y estimaciones no exactas. No hay limpieza automática en segundo plano."* Scan and clean
  run as Bubbletea commands with a context; `Esc` cancels; every unreadable path becomes a reported
  skip or error; nothing runs unless the user asked for it.
- **S11 — Tests.** Quote: *"Tests con carpetas temporales, detectores simulados, solapamientos,
  enlaces, límites de montajes, protecciones anti-borrado, previsualización y limpieza parcial."*
  New tests use `t.TempDir()`, injected probe functions, and never touch the developer's real
  configuration.
- **S12 — Integrated and non-destructive to coexist with.** Quote: *"Compatibilidad total con lo que
  soporte el proyecto… utiliza la TUI, componentes, tema, navegación y patrones de interacción
  existentes."* The screen is a Utilities job reached from its own row, uses the shared frame, menu
  and hint vocabulary, and touches no other front's files beyond the small registration points the
  section already has.
- **S13 — Security by default.** Quote: *"nada de telemetría, envío de datos o funciones de IA; no
  ejecutar comandos peligrosos por inferencia; no interpolar entradas en comandos de shell."*
  Commands are argument slices, never shell strings; paths are never interpolated into a shell.
- **S14 — Acceptance.** Quote: *"La funcionalidad se considera completa cuando muestra datos reales y
  solo elimina aquello que el usuario ha podido examinar y aprobar."*

## Decisions

- **D1 — A separate package.** The scan/measure/guard/clean domain lives in
  `installer/internal/diskclean` so it is testable without a terminal, and the TUI file is the
  adapter. `internal/diskclean` imports only the standard library; the screen passes
  `system.Detect()` facts in rather than the package detecting the platform itself twice.
- **D2 — No quarantine.** A moved-aside cache would either keep the space it was supposed to free
  or need its own restore lifecycle. Caches are regenerable by definition and the preview says the
  removal is not recoverable; a quarantine that lied about freeing space would be worse than none.
- **D3 — Sizing is a walk, docker is a report.** Filesystem stores are measured by a bounded,
  cancellable walk that counts logical size (sum of file bytes) and physical size (`Blocks*512`) and
  dedupes by `(device, inode)`. Container stores are never walked: their numbers come from
  `docker system df` / `podman system df`, which is the tool's own accounting.
- **D4 — Root-requiring commands degrade to a named refusal.** `apt-get clean`, `dnf clean all` and
  `pacman -Sc` run through `sudo -n`; when that is refused the item reports the exact command the
  user can run, and nothing else is attempted. No password prompt is ever raised from this screen.
- **D5 — Project roots are added by naming the directory this installer runs from.** No free-text
  path entry is introduced, so nothing typed can be interpolated anywhere; the added root is the
  process working directory, resolved and confirmed on screen, and the artifacts looked for inside
  it are a fixed list of directory names.

## Tasks

| ID | Task | Specs | Route | Commit |
|----|------|-------|-------|--------|
| T1 | Types, sizes, measurement engine (walk, dedupe, overlap, mounts, cancel) | S1, S3, S4, S10, S11 | inline | `feat(diskclean): measure what each development store really holds` |
| T2 | Delete guard and blocked roots | S7, S11, S13 | inline | `feat(diskclean): a removal re-checks its scope before it unlinks` |
| T3 | Ecosystem detectors with real probes | S1, S2, S6 | inline | `feat(diskclean): the tools that are here, with the reason each was found` |
| T4 | Plan, dry-run and cleanup execution | S5, S6, S8, S9 | inline | `ff3d579` `feat(diskclean): a preview runs first, and the result is measured again`, plus `5c1e2cf` `fix(diskclean): a run that released nothing measurable says so, not zero` (found while wiring the screen: a zero delta was being printed as a measured release) |
| T5 | TUI state, commands and key handlers | S2, S5, S10 | inline | `004d9da` |
| T6 | Rendering, Utilities integration, frame and golden guards | S3, S9, S12 | inline | `004d9da` `feat(diskclean): the screen that shows what each store holds, and cleans what you mark` — T5 and T6 share this one work-unit commit on purpose: the frame, emoji and terminal-fit guards measure every declared screen, so a screen constant that nothing renders cannot be committed on its own without leaving the suite red |
| T7 | Docs and closure | S12, S14 | inline | `docs(diskclean): the development caches in the README and the installer guide` |
| T8 | The verification's S3/S5 defect: the screen must render the detection evidence, the covered paths or engine objects, the category and the reconstruction cost | S3, S5, S14 | inline | `29985dd` `fix(diskclean): the screen shows why each store was found and what removing it costs` — found by the independent verification (T7), not by this feature's own tests; see L9 |
| T9 | The re-verification's advisories: the identity re-check's test depended on the filesystem, and the findings were not grouped by the tool or the project the request names | S3, S7 | inline | `fix(diskclean): the identity check's test no longer depends on the filesystem`, `feat(diskclean): the findings are grouped by the tool or the project that owns them`, `test(diskclean): the verification's own S3/S5/S14 probe, made durable` — see L10 |

## Log

**L1 — the request, verbatim.**

> **Misión:** encontrar espacio ocupado por residuos y cachés de desarrollo y permitir su limpieza **selectiva, explicable y segura**.
>
> ## Alcance funcional
>
> - Descubrir automáticamente herramientas/ecosistemas realmente presentes y sus cachés mediante mecanismos oficiales y rutas configuradas, no por supuestos arbitrarios. Contemplar los entornos de paquetes y builds usados hoy en desarrollo: JS/TS, Python, Rust, Go, ecosistemas JVM, gestores del sistema, contenedores y otros soportados por el proyecto. No hace falta soportar un ecosistema que no pueda detectarse fiablemente.
> - Diferenciar cachés regenerables, descargas, artefactos de compilación, dependencias de proyectos, imágenes/contendedores y **volúmenes persistentes**. Clasificar riesgos y no presentar todo como basura segura.
> - Mostrar espacio usado y **espacio potencialmente recuperable** con unidades legibles, agrupación por herramienta/proyecto, rutas u objetos de origen, motivo de detección y grado de certeza. Aclarar la diferencia entre tamaño lógico, espacio físico y estimación; evitar doble cómputo cuando se solapen rutas o almacenes compartidos.
> - Exploración con alcance controlado; permitir añadir directorios de proyectos voluntariamente. No rastrear todos los discos, volúmenes montados o datos del host desde WSL por defecto.
> - Selección granular, vista previa/dry-run y confirmación explícita. Categorías delicadas (dependencias de proyectos, contenedores, volúmenes, datos persistentes) jamás preseleccionadas. Dar explicaciones claras sobre qué podría necesitar volver a descargarse o reconstruirse.
> - Preferir el mecanismo de limpieza oficial de la herramienta correspondiente; **nunca borrar directamente almacenes internos gestionados por Docker o gestores de paquetes** cuando exista una API/comando de mantenimiento soportado.
> - Para borrado directo de archivos: verificar rutas y pertenencia a un ámbito aprobado justo antes de ejecutar, controlar symlinks, junctions, montajes, loops, hardlinks y carreras TOCTOU; bloquear raíces del sistema, `HOME`, repositorios, `.git`, archivos de configuración, credenciales y rutas ambiguas.
> - Actualizar resultados tras la ejecución y mostrar liberación real cuando pueda medirse, fallos parciales y tareas no realizadas. Si hay cuarentena o papelera, explicitar qué acciones son recuperables y cuáles no.
> - Dentro de WSL distinguir la liberación de espacio en Linux de la reducción del tamaño del archivo de disco virtual visto por Windows; no compactar ni manipular discos virtuales automáticamente.
>
> ## Robustez y aceptación
>
> Escaneos grandes sin congelar la TUI; cancelación, permisos insuficientes, archivos bloqueados, ficheros cambiantes, rutas largas y Unicode, herramientas no instaladas y estimaciones no exactas. No hay limpieza automática en segundo plano.
>
> Tests con carpetas temporales, detectores simulados, solapamientos, enlaces, límites de montajes, protecciones anti-borrado, previsualización y limpieza parcial. La funcionalidad se considera completa cuando muestra datos reales y solo elimina aquello que el usuario ha podido examinar y aprobar.
>
> ## Contexto y condiciones obligatorias
>
> Estás trabajando en un fork de los dotfiles de [el proyecto de origen] que ya incluye un installer/configurador TUI con navegación, estilos y temas dinámicos, mascota, utilidades e integración con el entorno de desarrollo. Tu cometido es **implementar esta funcionalidad realmente**, no entregar un diseño, pseudocódigo o una maqueta sin lógica.
>
> Nota: la marca del proyecto de origen va redactada en la cita anterior, porque el *branding audit*
> del repositorio (`.github/workflows/ci.yml`) la rechaza fuera de las rutas de atribución y `odd/` no
> es una de ellas. El resto de la cita es literal, y `odd/tasks/` conserva así la petición sin
> introducir la marca en el árbol que audita CI.
>
> - **Antes de modificar nada**, examina el repositorio y localiza su arquitectura, convenciones, comandos, utilidades compartidas, navegación, estados, tratamiento de errores y tests. Adapta tu solución a lo que existe; no presupongas un lenguaje ni un framework.
> - **Compatibilidad total con lo que soporte el proyecto:** todos sus sistemas operativos, plataformas, entornos (incluido WSL cuando corresponda), shells y emuladores de terminal. No acoples la implementación a una shell concreta ni confundas el entorno del host con el de WSL. Detecta capacidades reales y ofrece degradación elegante cuando alguna API/función no exista. Una plataforma sin capacidad específica no debe provocar un fallo global.
> - **Experiencia integrada:** utiliza la TUI, componentes, tema, navegación y patrones de interacción existentes. Prioriza navegación por teclado, terminales con capacidades gráficas distintas, mensajes claros, progreso y cancelación cuando proceda. Mantén la UI responsiva.
> - **Tecnología actual y mantenida:** cuando necesites herramientas o bibliotecas, investiga las alternativas vigentes, consolidadas, seguras y apropiadas para el ecosistema del proyecto. Prefiere APIs oficiales y mecanismos nativos bien soportados. No introduzcas herramientas obsoletas/deprecadas, dependencias innecesarias ni reinventes soluciones maduras sin motivo. Comprueba versiones y compatibilidad; no impongas un binario externo como dependencia obligatoria si existe una opción más fiable.
> - **Seguridad por defecto:** nada de telemetría, envío de datos o funciones de IA; no ejecutar comandos peligrosos por inferencia; no interpolar entradas en comandos de shell; proteger rutas, credenciales y secretos; manejar permisos, timeouts, procesos concurrentes y errores parciales. Toda acción que cambie o elimine algo requiere previsualización y consentimiento explícito, salvo que la aplicación ya tenga un flujo inequívoco equivalente. Emplea copias de seguridad y reversibilidad donde sea viable; nunca afirmes que una operación es reversible si no lo es.
> - **Coexistencia con otros agentes:** otros agentes implementan otras utilities simultáneamente. Mantén tu código encapsulado, reutiliza interfaces existentes, evita refactors globales o cambios innecesarios en ficheros compartidos y no alteres otras funcionalidades. Si existe funcionalidad similar, intégrate con ella en lugar de duplicarla.
> - **Calidad:** código mantenible, tipado cuando corresponda, datos/estados reales y no simulados, errores recuperables, tests automatizados con dobles de prueba para acciones destructivas, cobertura de casos límite y validaciones del repositorio. No toques la configuración real del desarrollador durante tests.
>
> **Entrega esperada:** cambios funcionales en el repositorio, integrados y probados. Al terminar, informa brevemente de archivos modificados, decisiones importantes, comprobaciones ejecutadas, cobertura real y limitaciones detectadas. No te quedes en un plan: implementa.

**L2 — exploration, before any write.** Read `installer/` architecture: Bubbletea model in
`internal/tui` (one screen constant per screen, rows derived in `GetCurrentOptions`, handlers in
`update.go`, view in `view.go`), utilities precedent in `shellaudit.go` (probe struct with function
fields, resolution at `NewModel`, measurement as a command), `internal/system.Detect()` for platform
and WSL facts, `Makefile`/`scripts/preflight.sh` for the gate, `odd/README.md` for the tracker
convention. Inherited constraints found: `installer/go.mod` and `go.sum` are upstream copies and are
not edited; the utilities section's option list is `GetCurrentOptions`, its panel list is
`utilitiesPanelEntries`, and every screen is measured by `TestEveryScreenIsCoveredByAFrameGuard`.

**L3 — the measurement engine (T1).** `internal/diskclean` was written with the standard library
only: `Category`, `Risk`, `Mechanism`, `Item`, `Probe`, `Measure`. The scan dedupes by `(device,
inode)` across all roots of one run, never follows a symlink, and stops at a device boundary of the
root it was given. Red observed first: the package's tests were written against these contracts
before the implementation compiled.

**L4 — the guard (T2).** `Guard.Check` verifies at call time: absolute, clean, inside an approved
root, not one of the blocked roots, not a symlink leaf, not a mount point, same device identity as
when measured. `Remove` re-runs `Check` immediately before unlinking and refuses a directory that
contains a mount point instead of deleting around it.

**L5 — detectors (T3).** Registry of detectors: npm, pnpm, yarn, bun, pip, uv, poetry, conda, cargo,
go, gradle, maven, coursier, apt, dnf, pacman, pkg (Termux), brew, docker, podman, plus project
artifact directories inside an explicitly added root. Every item names the reason it was found, and
container items are reachable only through their tool's own command.

**L6 — execution (T4).** `Plan` is the dry-run; `Clean` runs one item at a time with a context,
re-measures the paths afterwards and reports the real delta, partial failures and skipped entries.
Tool-reported items (docker/podman) are re-queried and compared. Commands are argument slices; shell
strings do not exist in the package.

**L7 — the screen (T5, T6).** `ScreenDiskCleaner` appended to the screen constants; the Utilities
section gained its row (`Clean up development caches`), its panel entry (`Caches`) and its paragraph.
Scan and clean are Bubbletea commands with a context; `Esc` cancels a scan or a cleanup in flight and
leaves a preview without running it. Every checkbox starts empty, `a` marks only the items whose risk
is safe, `n` clears the selection, and a finding nothing can be done about is not selectable — the
screen names the reason instead. The confirm row exists only in the preview. `screen_coverage_test.go`
learned the new screen in four places (`measuredScreens`, `terminalFitCases`,
`screensTheInstallerStatesNeverReach`, `diskCleanerFrameCase`), a golden frame pins the screen
(`TestDiskCleanerGolden`), and the Utilities golden and panel tests were updated for the new row.
The working path also swallowed a real defect once: the screen's escape key and space key are owned
globally by the leader key and the back handler, so the first version of the screen never saw them —
the tests caught it, and both are now taken by the disk cleaner before the globals.

**L8 — verification.**

Own checks, run inline as each task landed:

- `gofmt -l internal/diskclean/ internal/tui/` → nothing; `go build ./...` → clean; `go vet
  ./internal/diskclean/ ./internal/tui/` → clean.
- `go test ./internal/diskclean/` → `ok` (measurement, mounts, dedupe and overlap, guard, TOCTOU,
  detectors and tool parsers, plan, removal walker, execution and release measurement).
- `go test -run 'TestDiskCleaner' ./internal/tui/` → `ok`, 16 screen tests: scan-is-a-command,
  one-scan-at-a-time, nothing-preselected, bulk key leaves delicate items, preview-before-clean,
  escape-leaves-the-preview, confirmation-runs-the-plan, escape-stops-the-run, escape-stops-a-scan,
  add/forget project root, the confirm row counts the work, inert checkbox, section offers the row,
  what the screen will not do, the preview renders every command and refusal, the results render every
  outcome and the filesystem number.
- Frame and rendering guards → `ok`: `TestEveryScreenIsCoveredByAFrameGuard`, `TestNoInstallerScreenDrawsAnEmojiBox`,
  `TestEveryScreenFitsEveryTerminalSize` (with `measuredScreens` raised 63→64 and the new case added to
  `terminalFitCases`, which is what the guard's own pinned count demands), `TestDiskCleanerGolden`
  (new) and `TestUtilitiesGolden` (regenerated for the new row and paragraph).
- `make preflight` → **fails at step 4/7** (`cd installer && go test ./... -count=1`), exit 2, and the
  only failures are `TestStepInstallTerminalGhosttyReportsAFailedDownload`,
  `TestStepInstallTerminalGhosttyRunsTheDownloadItFetches`,
  `TestInteractiveGhosttyScriptFailsWhenTheDownloadFails` and
  `TestInteractiveGhosttyScriptDownloadsThenRuns` — the Ghostty install-script download path, which
  this feature does not touch. All four were reproduced in a detached worktree at the base commit
  `c1e4a64` before this feature's first commit, so they are pre-existing and not a regression: the
  same four, the same assertions, the same output. Every other package in the gate is `ok`, this
  feature's `internal/diskclean` among them.

Independent verification (`gentle-ai-verify`, read-only, over `c1e4a64..HEAD`) returned one verdict
per spec: S1 PASS, S2 PASS, **S3 FAIL**, S4 PASS, **S5 PARTIAL**, S6–S13 PASS, S14 PARTIAL. S3 and
S5 are the same defect — the screen showed a size and a risk and none of the evidence fields beside
them — and its fix, with the checks that followed it, is L9. S14 was partial for that same open
defect and was re-run closed by the fix.

Native review (receipt-driven development) disposition: `gentle_review` inspect offered
`review.start` for the frozen candidate. A START carrying the 4R lens selection was rejected as
`native-start-input-invalid` / `lenses-invalid` — the native provider on this machine is
`gentle-ai 3.7.0`, whose `review start` knows `--focus`, not the lens selection — and it created no
lineage. A START with the ordinary input returned `lens_context_budget_exceeded`: the candidate
frozen for this workspace is the accumulated branch diff, which exceeds the reviewer's context
budget, and the provider created **no review authority** (`mutation_outcome: none`, `next_action:
stop`) and states that retrying that exact candidate cannot succeed. Its continuation is to reduce
scope to smaller candidates; for that path one slice worktree was created at `1f6799d`, whose own
inspect reported `rdd_disabled`. `gentle-ai review mode status` then answered in both worktrees:
`receipt-driven development: off (decided by clone_local)` — global unset, clone-local off — so the
review switch is off for this clone by its own record, the candidate is left unreviewed by that
decision, and the slice worktree was removed and pruned (`git worktree remove --force` +
`git worktree prune`). Nothing has to be abandoned or repaired: no lineage, no authority, no
mutation was left behind.

**L9 — the one real defect the verification found, and its fix (T8).**

The verifier read the screen against S3 ("mostrar espacio usado y espacio potencialmente recuperable
... rutas u objetos de origen, motivo de detección y grado de certeza") and S5, and found that the
rows rendered a title, a size and a risk while `Item.Reason`, `Item.Detail`, `Item.Paths`,
`Item.VolatileObjects` and `Category.Label()` were never drawn anywhere: the evidence existed in the
package and no screen showed it. Its own probe was red on the list and on the preview. Everything
else it checked passed: the honest numbers (S2), the selection rules (S4), the mechanisms and the
engine-owned stores (S6), the guard and its re-checks (S7), the plan and the release measurement
(S8), the key handling and the frames (S9, S10), the scope (S11, S13) and the docs (S12). It made no
writes to the workspace and worked from a scratch copy with its own `HOME`, `TMPDIR` and `GOCACHE`.

The fix (`29985dd`): the finding under the cursor carries a card, in the findings list and inside the
preview — its category and risk, what it holds and which half of that number is the disk's, where
the number came from (walked, or the owner's own accounting), the evidence that found it, the paths
or the engine objects it covers, and the reconstruction cost from `Detail`. In the preview the card
opens with the exact command, the exact path or the exact refusal the confirmation would use. The
preview also lists the items it is about as rows, so the cursor can walk them; space toggles nothing
there and the confirmation counts only the runnable ones. The card sits above the measured summary,
the introduction and the standing notes, so the smallest measured frame cuts the orientation text
and never the evidence; the golden pins that and shows each finding's reason and paths. A store that
is gone, unreadable, cut short by the walk's ceiling or sharing bytes with another finding says so in
the same card.

Checks run after the fix:

- `gofmt -l` → nothing, `go build ./...` → clean, `go vet ./internal/diskclean/ ./internal/tui/` → clean.
- `go test ./internal/diskclean/` → `ok`, including the new `TestCleanNeverUnlinksAnEngineOrManagerStore`:
  an engine's or a manager's store is never unlinked file by file, even when it is selected and planned.
- `go test -run 'TestDiskCleaner' ./internal/tui/` → `ok`: the seventeen tests the re-verification counted, including
  `TestDiskCleanerShowsWhyEachFindingWasFoundAndWhereItIs`,
  `TestDiskCleanerCardSaysWhyAnUnmeasuredFindingIsNotAZero`, and
  `TestDiskCleanerPreviewRendersEachCommandAndEachRefusal` rewritten to walk every preview row and
  check each one's card. (A3: this said "18 screen tests"; the verifier observed 17 `RUN`s, and 18 is
  what the count became once T9's grouping test landed.)
- Full `go test ./internal/tui/` → only the four pre-existing Ghostty failures, identical to the base
  commit `c1e4a64`.
- `TestDiskCleanerGolden` regenerated; re-read at 80x24 it carries the card — category, both sizes and
  their source, the reason, the path. `TestUtilitiesGolden` did not change.
- `make preflight` re-run after the fix: the same failure set as before it, nothing added. Step 4/7
  (`go test ./... -count=1`) still fails only on the four Ghostty tests
  (`TestStepInstallTerminalGhosttyReportsAFailedDownload`,
  `TestStepInstallTerminalGhosttyRunsTheDownloadItFetches`,
  `TestInteractiveGhosttyScriptFailsWhenTheDownloadFails`,
  `TestInteractiveGhosttyScriptDownloadsThenRuns`), which were reproduced at the base commit
  `c1e4a64`, plus the two end-to-end smoke steps that fail for the environment rather than for the
  code: `Configure WSL` (`permission denied` writing `/etc/wsl.conf`) and `zsh installation failed`.
  All six are present, identically, in this feature's pre-fix run of the same gate. Every other
  package is `ok`, this feature's `internal/diskclean` among them.

**L10 — the re-verification, its verdicts, and what its advisories changed (T9).**

The independent verification was re-run (read-only, its own scratch copy, `HOME`, `TMPDIR` and
`GOCACHE` inside it, `GOPROXY=off`) over the same range. Verdicts: **S3 PASS, S5 PASS, S14 PASS**,
S1–S13 checked again with no regression, and **no blocker caused by the fix**. It reproduced the old
defect as RED at `52be18a` and GREEN at `29985dd` on the rendered string -- not on the model -- and
it checked the evidence at the 80x24 floor, which is the frame the specs care about.

Its advisories, and what each one changed:

- **A1 — the identity test depended on the filesystem.** `TestVerifyRefusesAPathReplacedAfterThePreview`
  recreated the directory and expected a new inode, but a filesystem that recycles inode numbers
  (ext4 in its default layout) hands the recreated directory the same `(device, inode)` pair, so the
  guard verified it and the test failed there. Reproduced in this session with
  `TMPDIR=/var/tmp/...`: `--- FAIL: TestVerifyRefusesAPathReplacedAfterThePreview`. Fixed by making
  the identity injectable -- `Probe.Identity`, used by the guard, the walk's dedupe and the removal's
  own identity -- so the test states the identity it is testing instead of hoping the filesystem
  produces it. The residual risk is now stated instead of hidden behind a filesystem-dependent test:
  `TestVerifyCannotTellASwapTheFilesystemHides` pins it, `Guard.Verify` documents it, and it is a
  bullet under `## Remaining`.
- **A2 — no visible grouping by tool or project**, which the request asks for verbatim. The findings
  are grouped now: a heading row per tool and, for the artifacts of a project directory the user
  added, a heading row per project -- the project's own directory name, not the word "project" --
  with the headings ordered by name and, inside a group, the findings largest first. Two projects
  whose last directory has the same name stay in two groups, because the group is keyed by the whole
  project path. The preview groups its items the same way. A heading is inert: the cursor can rest on
  it and no key does anything through it.
- **A3 — L9's test count was wrong** (17 `TestDiskCleaner` tests, not 18). Corrected in L9.
- **A4 — the preview's headline promised more than the rows carried**: a removal row carries the
  count of paths, not the path. The rows now carry the exact path when there is one, and the headline
  says what is true.
- **A5 — the doc commit the verifier saw land mid-run** (`99a4db0`) is the L9 entry written by this
  session; the verifier made no writes and reported the change instead of fixing it. Nothing to do.

One consequence of the grouping that the durable probe caught immediately, and that is fixed here:
heading rows cost the description its rows, and at 80x24 the cut landed on the very card the S3
verdict rests on. The development-caches screen now declares a prose floor (`proseFloor`: the card's
six rows, plus the preview's headline in the preview), and the frame honours it by windowing the list
instead of handing it every row it asks for. A screen that declares no floor -- every other screen --
is budgeted exactly as before.

The verifier's own probe is part of the suite now: `internal/tui/diskclean_evidence_probe_test.go`,
kept as it was written, unique markers and all, with its provenance in the header. It is the test
that is red at `52be18a`.

Checks after T9:

- `gofmt -l` clean; `go build ./...` clean; `go vet ./internal/diskclean/ ./internal/tui/` clean.
- `go test -count=1 ./internal/diskclean/` → `ok` on tmpfs (`/tmp`) **and** with `TMPDIR` on ext4
  (`/var/tmp`), which is the filesystem where the old test failed.
- `go test -count=1 -run 'TestVerifyS3|TestVerifyS5|TestDiskCleaner' ./internal/tui/` → `ok`.
- Frame, emoji and terminal-fit guards → `ok`; `TestDiskCleanerGolden` regenerated at 80x24: the
  grouped list with the card of the finding under the cursor on screen.
- Full `go test ./internal/tui/` → only the four pre-existing Ghostty failures, the same at base
  `c1e4a64`.
- `make preflight` re-run after T9: the same failure groups as before the fix, plus one the
  **environment** caused. Step 4/7 (`go test ./... -count=1`) fails on the four pre-existing Ghostty
  tests, on the two end-to-end smoke steps that fail for the environment (`Configure WSL`:
  `permission denied` writing `/etc/wsl.conf`; `zsh installation failed`), and on
  `TestInstalledAssetsKeepTheRepositorysMode`, whose Neovim subtest failed with `Step 'Install
  Neovim' failed` because the tmpfs that holds `TMPDIR` was at 100% -- 35 MB free of 4.9 GB, with the
  scratch copies of several verification runs resident. That diagnosis was reproduced: the subtest
  fails while the filesystem is full and passes once this session's own scratch copy (323 MB) was
  removed, and it installs Neovim assets without ever touching the disk cleaner. Every other package
  in the gate, `internal/diskclean` among them, is `ok`. The gate was run once more with the space
  free, and it then fails on exactly the same set as before this feature: the four Ghostty tests and
  the two environmental smoke steps, and nothing else.

## Remaining

- **The identity re-check is `(device, inode)`.** It is the cheapest pair that exists, and a
  filesystem that recycles inode numbers can hand a directory recreated at the approved path the very
  same pair, which makes that one swap invisible to the check. The window is between the preview and
  the removal, only a process able to write inside the approved scope can race there, and every other
  check is still re-established at the unlink: the approved scope, the blocklist, the kind of object,
  no symbolic-link leaf and no mount point. `TestVerifyCannotTellASwapTheFilesystemHides` pins the
  limit so it cannot be forgotten, and `Guard.Verify` states it where the check lives.
- **Termux `pkg` has no cache command.** Termux's `pkg`/`apt` cache is reached through
  `apt-get clean` when that exists; where it does not, the finding is reported with the reason and
  cannot be selected. No direct removal of the package manager's cache is attempted on Termux.
- **Platforms without a free-space query or a device/inode pair.** `freeSpaceOn` is implemented for
  Unix (`statfs`); elsewhere the result says the release could not be measured and falls back to the
  per-item numbers instead of printing a zero. Likewise, the walk's dedupe by `(device, inode)` and
  its mount-point refusals only apply where the platform reports those fields; on a platform that
  does not, a shared file may be counted twice and the walk will not claim to have found a mount it
  cannot prove. This is the honest failure mode, and it is documented in `stat_other.go`.
- **Physical size is `Blocks*512` on Unix and equals the logical size elsewhere**, so the physical
  column is only as good as the platform's own accounting.
- **A bounded walk can report a lower bound.** A store larger than the scan's ceilings (file count,
  time, or depth) is reported with `Truncated` set and the screen says the number is the part that
  was read.
- **Container volumes stay delicate and unselected.** The Docker and Podman volume rows are found
  and can be cleaned through `volume prune`, but they are never preselected by the bulk key: their
  contents are data the engine owns and may not be reproducible. A volume in use is never touched.
- **Ecosystems that cannot be detected reliably are omitted on purpose** — no guessing is done for a
  tool whose store layout could not be established from its own documentation or its own command.
- **No quarantine, by decision (D2).** The preview says the removal is irreversible; there is no
  trash or moved-aside store to restore from.
- **The scan is never automatic.** Nothing scans, measures or removes unless the user opened the
  screen and pressed a row; a run in flight stops when it is cancelled, and no work survives the
  screen.
