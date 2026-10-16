# Feature: the WSL performance advisor — an environment the installer only offers inside WSL

Status: implementing
Opened: 2026-10-16
Owner: parent session in the `worktree/green-valley-ce3b` worktree
Orchestrator: one parent, two bounded writers for the engines (`system/wslmigrate.go`,
`system/wslbench.go`), everything else inline.

## Why

The installer already ships two WSL artefacts: the `wslconfig` step (`.wslconfig` in the Windows
profile plus `/etc/wsl.conf` inside the distribution) and the **WSL resources** utility, which edits
the same `.wslconfig` from the utilities section. Neither answers the question the user actually has
on a WSL host: *is the project I am working in sitting on the Windows drive, and what does that cost
me?* A repository under a host mount pays a metadata round trip for every `stat`, which is what Git,
`node_modules`, build caches and watchers do all day.

This front adds the missing utility: a **read-only diagnosis** of the environment, the mounts and the
configuration, an **optional bounded benchmark** that measures the claim instead of asserting it, and
an **optional assisted migration** that copies and verifies a project without ever deleting the
source.

The visibility condition is the hard requirement and it is specific to this module: the utility is
**invisible outside a real WSL instance**, and a Windows host that merely *has* WSL installed is not
enough.

## Deliverable

Real code, integrated in the existing TUI, with the repository's own guards: conditional navigation,
a screen in the utilities family, a panel entry, tests for the detection (positive and negative),
mount classification, findings and severities, the benchmark, and the migration engine.

## Specs

Quoted verbatim from the request; the numbering is this front's.

- **S1 — visibility.** "La utilidad SOLO debe aparecer y poder ejecutarse cuando el installer esté
  corriendo DENTRO de una instancia WSL real." / "Que Windows tenga WSL instalado no basta." /
  "En Windows nativo, Linux convencional, macOS y cualquier otro entorno, no mostrar ni habilitar la
  opción." / "Implementar una detección fiable del entorno actual; no depender de un simple nombre de
  variable fácilmente heredable. Incluir tests positivos y negativos."
- **S2 — mounts.** "Detectar montajes según el entorno sin hardcodear rutas como `/mnt/c`, que pueden
  estar configuradas de otra manera." / "Explicar el impacto potencial sobre Git, dependencias,
  builds y operaciones con muchos archivos, sin prometer mejoras numéricas no medidas."
- **S3 — scope of the analysis.** "Analizar el directorio actual y permitir evaluar carpetas de
  proyectos seleccionadas. Identificar información útil: tipo de sistema de archivos/montaje, rutas
  relevantes, ubicación probable de dependencias o builds, problemas de enlaces y patrones de I/O.
  No leer contenido privado innecesario."
- **S4 — configuration review.** "Revisar, cuando sea posible y con permisos adecuados, factores
  habituales: configuración y límites de recursos, memoria/swap, espacio disponible, DNS/red,
  interoperabilidad host-invitado, montajes y comportamiento de arranque de herramientas. Diferenciar
  recomendaciones documentadas de conclusiones derivadas de mediciones reales."
- **S5 — benchmark.** "Benchmark opcional, de alcance limitado y no destructivo, para comparar
  operaciones equivalentes cuando sea factible. Usar carpetas temporales propias, limpiar únicamente
  sus artefactos y señalar cuándo los resultados pueden estar contaminados por cachés, antivirus,
  carga del sistema o hardware."
- **S6 — findings.** "Cada hallazgo debe incluir severidad, evidencia, explicación comprensible,
  beneficio esperado cualitativo y acción recomendada. Mostrar claramente los cambios que dependen del
  host o requieren reiniciar WSL; no fingir que pueden aplicarse directamente desde Linux."
- **S7 — migration.** "Ofrecer, si es viable de manera segura, una migración asistida opcional de
  proyectos hacia el almacenamiento adecuado: selección de origen/destino, previsualización,
  estimación de espacio, exclusiones, verificación de permisos, tratamiento de repos Git/symlinks/
  archivos ocultos y confirmación. Copiar y verificar antes de recomendar eliminar el origen; no mover
  ni borrar proyectos silenciosamente. Si esta operación no puede garantizar integridad, ofrecer
  instrucciones y no automatizarla."
- **S8 — no automatic host changes.** "No modificar archivos globales de configuración, registros del
  host, discos virtuales ni redes automáticamente. Las optimizaciones potencialmente arriesgadas deben
  ser guías o acciones plenamente confirmadas con respaldo."
- **S9 — robustness.** "Contemplar WSL con configuraciones y versiones distintas, automount
  personalizado, mountpoints con espacios, proyectos muy grandes, falta de permisos, discos llenos,
  symlinks entre sistemas y coexistencia con herramientas Windows. No inferir desde Linux datos del
  host que no puedas inspeccionar de forma fiable."
- **S10 — acceptance.** "Tests de detección dentro/fuera de WSL, visibilidad condicional en la
  navegación, sistemas de archivos, rutas y montajes personalizados, severidades, benchmarks y
  migración asistida cuando se implemente. El criterio principal es que sea invisible fuera de WSL y
  realmente útil dentro de WSL."
- **S11 — integration conditions.** "Antes de modificar nada, examina el repositorio y localiza su
  arquitectura, convenciones, comandos, utilidades compartidas, navegación, estados, tratamiento de
  errores y tests." / "utiliza la TUI, componentes, tema, navegación y patrones de interacción
  existentes." / "Mantén tu código encapsulado, reutiliza interfaces existentes, evita refactors
  globales o cambios innecesarios en ficheros compartidos y no alteres otras funcionalidades." /
  "Seguridad por defecto: nada de telemetría, envío de datos o funciones de IA; no interpolar entradas
  en comandos de shell." / "No toques la configuración real del desarrollador durante tests."

## Tasks

| ID | Spec | Route | What it delivers |
| --- | --- | --- | --- |
| T1 | S1, S9, S10 | inline | `installer/internal/system/wslruntime.go`: evidence-based detection of *running inside* a WSL distribution, with an injectable probe and a container veto — commit `5872121` |
| T2 | S2, S3, S9 | inline | `installer/internal/system/wslmount.go`: `/proc/self/mountinfo` parsing (octal escapes, spaces), longest-prefix mount resolution, host-mount vs Linux-native classification, disk space — commit `ae909ec` |
| T3 | S7, S8, S9 | worker | `installer/internal/system/wslmigrate.go`: migration plan (preview, space, exclusions, Git/symlinks/hidden files) and a copy-and-verify executor that never deletes — commit `f1da00e` |
| T4 | S5, S9 | worker | `installer/internal/system/wslbench.go`: bounded, non-destructive, cancellable benchmark in its own temporary directories with cleanup — commit `9848870` |
| T5 | S4, S6, S9 | inline | `installer/internal/tui/wsladvisor.go`: the facts a report is derived from, the findings with severity/evidence/explanation/benefit/action/scope/origin, and the project discovery — commit `9022e75`, with `system/meminfo.go` so the metrics sampler and this report share one `/proc/meminfo` reader |
| T6 | S1, S3, S6, S10, S11 | inline | TUI integration: the gated utilities row, `ScreenWSLAdvisor`, the panel entry, the keys, the render, and the guards (frames, visibility, goldens) |
| T7 | S11 | inline | `docs/tui-installer.md` and this file: what the utility is, what it refuses to do, and the detection rule |
| T8 | S10 | inline | `make preflight` and an honest report of coverage and limits |

## Design decisions

- **Detection is evidence, not a variable.** The rule is a small decision over named signals: the
  kernel release/version markers (a fact of the running kernel, not of the environment), the WSL init
  artefacts (`/proc/sys/fs/binfmt_misc/WSLInterop`, `/run/WSL`, `/usr/lib/wsl`), and the env vars last
  and only as corroboration. `WSL_DISTRO_NAME` alone is never enough, because a variable is inherited
  by any child process — including a container started from a WSL shell.
- **A container on the WSL kernel is refused**, with its own reason. Docker Desktop's WSL2 backend and
  a `docker run` inside a WSL distribution both report the WSL kernel while none of the distribution's
  own mounts, `wsl.conf` or native storage can be seen. The conservative direction is what S1 asks
  for: an answer that cannot be supported is not offered.
- **The global `IsWSL` is left alone.** It is what the install step and the WSL resources utility
  already read, and this front does not widen or narrow their behaviour (S11). The advisor has its own
  stricter answer.
- **No path is hardcoded.** Drives are recognised from the mount table's filesystem type
  (`drvfs`, `9p`, `virtiofs`, `cifs`, `ntfs3`) and the distro's native storage from its own root
  mount, so `[automount] root = /windir` and a mount point with spaces work the same way.
- **One writer, and it never deletes.** The advisor's screens write nothing at all. The only writer is
  the migration executor, which copies, verifies and reports; removing the source stays a human act.

## Log

- **L1 (user, verbatim, abridged to the module):** "Agente 06 — WSL Performance Advisor (solo dentro de
  WSL) … Miśion: crear un asesor de rendimiento específico para el entorno WSL, que detecte cuellos de
  botella frecuentes y recomiende acciones verificables y seguras." Full request: see Specs above;
  every `S#` quote is taken from it.
- **L2 (repo evidence, this session):** the utilities section is `utilitiesPanelEntries` (panels.go)
  plus the `ScreenUtilities` option list (model.go) and `utilitiesDescription` (view.go); a new screen
  must be registered in `GetCurrentOptions`, `GetScreenTitle`, the `handleKey` dispatch, the render
  dispatch, and a frame-guard case (`screensTheInstallerStatesNeverReach`), or
  `TestEveryScreenIsCoveredByAFrameGuard` fails by name.
- **L3 (repo evidence):** `system.classifyLinux` already sets `IsWSL` from the kernel string *or*
  `WSL_DISTRO_NAME`; that is exactly the env-var dependence S1 forbids for this module, so the
  advisor's detection is a new, separate function and the existing field keeps its meaning.
- **L4 (delegation, this session):** T3 and T4 were handed to two bounded background writers with the
  exact API, the behavioural rules and `## Allowed edit surfaces` limited to their own two files each.
  The first pair of tasks was cancelled by an earlier session shutdown having written nothing; both
  were relaunched. Both returned with `gofmt`, `go vet` and `go test ./internal/system/` green, and
  the parent re-ran the package suite (6.6 s, ok) before committing.
- **L5 (decision, parent review of T3):** the worker added one unexported field (`verifyLimit`) to
  `MigrationPlan` because the frozen export surface had no way to carry `MigrationOptions.VerifyLimit`
  into `RunMigration`. The parent accepted it: the exported names and signatures are unchanged, a
  hand-built plan still falls back to `DefaultMigrationVerifyLimit`, and the alternative was a second
  exported field on a report struct.
- **L6 (decision, parent, T4):** `DefaultBenchLimits()` was added to `system/wslbench.go` so the
  benchmark screen can state the file count, the file size, the time bound and the byte ceiling it is
  about to use. The numbers are owned by the engine; the screen must not hardcode a copy that could
  drift from what the run really does.
- **L7 (defect found in review, T6):** the first wiring gated the utilities row on the strict
  detection alone, and the golden frames caught it immediately: this development machine *is* a WSL
  instance, so the pinned host the goldens use (`goldenSystemInfo`, `IsWSL: false`) drew a WSL row that
  the fixture does not have. The gate was changed to require the installer's own host answer as well
  (`WSLRuntime.Inside && SystemInfo.IsWSL`) and to be evaluated where it is used instead of cached in
  the screen's state, so a pinned host stays in charge of what is offered and no stale copy of the
  condition can outlive it. Both directions are narrower than before, never wider.
- **L8 (decision, parent, T6):** the two acting rows exist only where their engine can finish, and the
  body states what is missing when they do not: a row that fails when it is pressed is not offered in
  this installer, and the copy's preview is the confirmation, so no action here is reachable without
  the facts it depends on printed first.
- **L9 (checks, this session):** `go test ./internal/system/` ok (6.6 s) after both engines landed;
  the TUI suite and `make preflight` are recorded in the closing report.
