# The update center: real updates, from the interface

## Goal

The installer provisions tools with five package managers and a modern runtime manager, and
nothing in the interface ever says which of them has something newer. This front adds a
**Package & Tool Update Center** to the Utilities section: it detects which managers and channels
really exist on this machine, reads the updates each one honestly reports, shows every entry with
its manager, scope, current and available version, and applies a *selection* through the owning
manager's own command, with a plan shown first, progress, cancellation, per-item results and a
re-read afterwards. Manager identity is never collapsed: two installs of the same executable from
two managers stay two entries.

## Specs

- **S1 — real detection, real answers.** "Detectar qué gestores y canales de instalación están
  disponibles **realmente** en la máquina y cuáles usa el installer." and "Obtener actualizaciones
  disponibles de paquetes gestionados, herramientas de desarrollo, plugins/componentes del propio
  setup cuando proceda y versiones de runtimes administradas por gestores modernos."
- **S2 — every entry carries its origin.** "Mostrar origen/gestor, paquete, versión actual,
  disponible, alcance (usuario/sistema/proyecto), advertencias y si está fijado/pinneado."
- **S3 — the honest states, never invented severity.** "Diferenciar \"no hay actualizaciones\", \"no
  se pudo consultar\", \"gestor no disponible\", \"requiere permisos\" y \"fuente sin conexión\". No
  inventar vulnerabilidades ni etiquetar como actualización de seguridad sin metadatos fiables."
- **S4 — identity, never merged.** "Evitar entradas duplicadas cuando distintos gestores mencionen
  el mismo ejecutable; **no fusionar ni actualizar instalaciones distintas** solo porque compartan
  nombre."
- **S5 — selection is the user's, and nothing updates everything.** "Permitir filtrar, seleccionar,
  ver cambios/resumen cuando exista información verificable, actualizar un elemento, un conjunto
  seleccionado o una categoría. Nada de \"actualizar todo\" automáticamente al abrir la pantalla."
- **S6 — pins, locks and policies are respected.** "Respetar versiones fijadas, lockfiles,
  dependencias, gestores de versiones, políticas del sistema y conflictos conocidos. No alterar
  versiones de proyectos sin decisión expresa ni sustituir el gestor propietario del paquete."
- **S7 — the plan comes before the action.** "Mostrar plan/impacto antes de ejecutar: qué
  comando/API se usará, permisos requeridos, reinicios/reloads esperables y si hay posibilidad real
  de rollback. Confirmación explícita, progreso, logs legibles, posibilidad de cancelación segura y
  resultado individual."
- **S8 — the failures a package manager produces.** "Tratar bloqueos del gestor, repositorios
  caídos, redes lentas, prompts interactivos, elevación de permisos, necesidad de reinicio, fallos
  parciales y conflictos de dependencias. No ejecutar varios gestores en paralelo si compiten por
  locks o recursos críticos."
- **S9 — requested, executed, confirmed.** "Actualizar el inventario y volver a verificar versiones
  después de ejecutar. Distinguir actualización solicitada, ejecutada y efectivamente confirmada."
- **S10 — agnostic of AI.** "La función es agnóstica respecto a IA: no instalar, configurar ni
  sugerir stacks de IA como parte del setup."
- **S11 — security by default.** "no ejecutar comandos peligrosos por inferencia; no interpolar
  entradas en comandos de shell; proteger rutas, credenciales y secretos; manejar permisos,
  timeouts, procesos concurrentes y errores parciales. Toda acción que cambie o elimine algo
  requiere previsualización y consentimiento explícito".
- **S12 — every platform the project supports, and graceful degradation.** "Compatibilidad total
  con lo que soporte el proyecto: todos sus sistemas operativos, plataformas, entornos (incluido
  WSL cuando corresponda), shells y emuladores de terminal." and "ausencia de un gestor no es error
  global".
- **S13 — the layers stay apart.** "Separar detección/inventario, adaptadores de cada gestor,
  resolución de actualizaciones y UI."
- **S14 — no remote scripts, no secrets in logs.** "No ejecutar scripts remotos arbitrarios ni usar
  `curl | sh` como método de actualización predeterminado. Redactar credenciales de registros
  privados."
- **S15 — the tests the front names.** "Tests para versiones, pins, ámbitos, resultados vacíos,
  distintas instalaciones homónimas, fallos de red, locks, operaciones parciales, permisos y
  confirmación."
- **S16 — functional is the bar.** "La entrega es funcional cuando el usuario puede **comprobar
  actualizaciones reales y aplicar selectivamente las soportadas** desde la TUI, con trazabilidad y
  sin romper su configuración."

## Tasks

- [x] **T1 — the domain, its states and its seam (S1, S2, S3, S4, S9, S13).**
  `installer/internal/updates`: `Item` (identity = manager + scope + name), `Status` (updates,
  current, unavailable, permission, offline, failed), `Scope`, `Result`, `Plan`, `Outcome`,
  `Command`/`Runner` seam. Route: inline, it is the contract everything else reads.
- [x] **T2 — one adapter per manager (S1, S3, S6, S8, S12, S14).**
  Homebrew, apt, pacman, dnf, Termux's pkg, snap, flatpak and rustup: availability probe, the
  manager's own read command, a pure parser, the update argv built from the selected names only,
  pins (`brew outdated --json=v2`'s `pinned`, `apt-mark showhold`, pacman's `IgnorePkg`, dnf's
  versionlock, snap's `held`, rustup's toolchain pin) and the honest offline/permission/lock
  classification. Route: inline, each adapter is a pure parser plus an argv builder.
- [x] **T3 — the orchestration (S5, S7, S8, S9, S11, S14).** One manager at a time, never in
  parallel; `CheckAll` reports per-manager results as they land; `Apply` runs the built argv with
  no shell in between, streams lines, honours cancellation, and re-reads the manager afterwards to
  say requested / executed / confirmed per item. Route: inline.
- [x] **T4 — the screen (S2, S5, S7, S12, S16).** `ScreenUpdateCenter`, one level in from Utilities:
  inventory rows with `[ ]`/`[x]`, filters, select-visible/clear, plan preview, an explicit
  confirmation state that names the exact commands, elevation, reloads and rollback, a running
  state with the tail of the log and a cancel row, and per-item results. Route: inline, following
  the section's existing screens.
- [x] **T5 — the tests (S15).** Pure parser tables, pin and scope cases, empty results, homonymous
  installs, offline/permission/lock classification, partial failure, cancellation, and the screen's
  keys and confirmation gate. Route: inline.
- [x] **T6 — the documentation (S13, S14, S16).** `docs/tui-installer.md` gains the section,
  `CHANGELOG.md` the entry, and the Utilities body names the row. Route: inline.

## Log

- **L1 (the request, verbatim).** "**Misión:** centralizar la detección, revisión y ejecución
  controlada de actualizaciones de las herramientas de desarrollo y componentes instalados,
  respetando los gestores originales. — Detectar qué gestores y canales de instalación están
  disponibles **realmente** en la máquina y cuáles usa el installer... Obtener actualizaciones...
  Diferenciar \"no hay actualizaciones\", \"no se pudo consultar\", \"gestor no disponible\",
  \"requiere permisos\" y \"fuente sin conexión\"... Evitar entradas duplicadas... Permitir filtrar,
  seleccionar... Nada de \"actualizar todo\" automáticamente... Respetar versiones fijadas,
  lockfiles... Mostrar plan/impacto antes de ejecutar... Tratar bloqueos del gestor... Actualizar el
  inventario y volver a verificar versiones después de ejecutar. Distinguir actualización
  solicitada, ejecutada y efectivamente confirmada... Tests para versiones, pins, ámbitos,
  resultados vacíos, distintas instalaciones homónimas, fallos de red, locks, operaciones parciales,
  permisos y confirmación." The full brief is the front's mission statement; the Specs above quote
  the operative lines from it.
- **L2 (decision).** The front lives in its own package (`internal/updates`) rather than inside
  `internal/tui`, because the mission asks for the four layers to stay apart and because every
  adapter is then testable without a terminal. The TUI keeps only the screen.
- **L3 (decision).** Commands are argv slices handed to `exec.CommandContext`, never shell strings:
  a package name that is a user's file name can never become a shell token, and there is no
  `curl | sh` anywhere on this path.
- **L4 (decision).** Elevation is `sudo -n` (never a prompt): a system manager that needs a password
  reports `permission` with the exact command to run, instead of hanging the interface on a prompt
  it cannot show.
- **L5 (evidence).** `go build ./...` and `go test ./...` green in `installer/`; the new package's
  tests plus the screen's guards are listed in the commit bodies.
- **L6 (evidence, T1–T3 + T5a).** The engine landed as `c0ea7ad`
  ("feat(installer): the machine's own managers answer what has an update") — 17 files, +5087,
  including this document. `go build ./...` and `go vet ./internal/updates/` clean;
  `go test ./internal/updates/` green at 72.2% coverage, with the three test files named above.
  Defects the suite found and the fix they forced: `unsupported()` had to report
  `Available:false`; apt's parser read the `[amd64]` architecture tag as a version and skipped
  `Inst` lines that name a package being installed; `rpm -q` had to skip "package X is not
  installed"; `FirstLine` had to bound runes rather than bytes; plan ordering had to be fixed by
  kind and scope so a formula precedes a cask and a user plan precedes a system plan.
- **L7 (evidence, T4).** The screen is `installer/internal/tui/update_center.go`, wired through
  `model.go` (`ScreenUpdateCenter`, `GetCurrentOptions`, the section's row), `view.go` (render
  dispatch, screen title, the panel's own body anchor, the section's one-line description),
  `update.go` (key dispatch, the `handleEscape` route, the four messages, the space-key exemption)
  and `panels.go` (the `Updates` entry). `TestUtilitiesGolden` was regenerated for the new row (the
  section's body is bounded by the frame, so the sentence is one line and the frame says
  `… and 3 more` rather than clipping); the screen joined the twelve-size frame matrix as
  `updateCenterFrameCase`, and `measuredScreens` moved 63 → 64 with the reason commented beside it.
- **L8 (evidence, T5b + two real defects).** Seventeen guards in
  `installer/internal/tui/update_center_test.go`, driving the real dispatch with a fake adapter
  per manager and a runner that records instead of running — no test in the file can start a
  package manager. They found two defects that are now fixed with their own regression guards:
  `Selection.Toggle` took the selection **by value**, so the first press on a screen whose
  selection was still its zero value created the map on a copy and threw it away (the row looked
  selected and the count stayed zero); and a line arriving from a command that had already been
  reported on was appended to the log of whatever ran next, because the handler kept lines before
  checking that a run was open. The suite also found a wording defect in the screen's own notice
  ("One one update is available") which is now a single `countedNoun`.
- **L9 (evidence, a defect the screen exposed).** `Screen.String()` — the diagnostic label used in
  failing guards' messages — was a slice indexed by the constant's value, so it was both
  incomplete (the last 26 screens printed `Unknown`, which is how this screen first appeared) and
  misaligned for every screen after the first two. It is a `map[Screen]string` naming all 48
  constants now, and `TestEveryScreenConstantNamesItself` fails by name for a constant added
  without one.
- **L10 (evidence, T6).** `docs/tui-installer.md` gained **The package update center** — the row,
  the eight managers with the exact command each is read with and moved with, the identity rule,
  the four honest answers, the confirmation and elevation rules, the one-manager-at-a-time rule,
  the three result facts and the re-read, and the argv/redaction rules — plus the Utilities body,
  the main menu's panel row and the golden list. `README.md` gained the fifth utility, and
  `CHANGELOG.md` the entry with every guard named.
