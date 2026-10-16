# Port and process manager

## Goal

A utility inside the installer's Utilities section that answers "what is holding this port, and may I
free it?" with real data from the machine: the bound sockets it can actually see, the process identity
behind each one where the kernel allows it, and one deliberate, revalidated stop of a single process.
It is the sixth utility of the section, shaped like the others: a read-only-capable screen one level in
from `ScreenUtilities`, a command off the update loop, and honest degradation when a platform, a
permission or a missing tool hides part of the answer.

## Specs

- **S1 — discovery is real, typed and honest about the transport.** The source brief: "Listar
  conexiones/puertos **en escucha o enlazados** según la visibilidad real del sistema: TCP y, donde la
  API lo permita, UDP; IPv4 e IPv6; dirección local, interfaz, protocolo y estado. **No equiparar
  escucha TCP y socket UDP**." TCP `LISTEN` and bound UDP (`UNCONN`) are different states and are shown
  as such; no row is invented for a family the reader could not read.
- **S2 — the owner is resolved when it can be, and the gap is declared.** "Resolver el proceso
  propietario cuando sea posible: PID, nombre, propietario, ejecutable, argumentos redactados y hora de
  inicio. **Un PID desconocido no equivale a puerto libre.** Mostrar limitaciones de permisos sin
  inventar información."
- **S3 — finding, filtering, sorting and refresh are usable.** "Búsqueda rápida por puerto, proceso o
  protocolo; filtros por ámbito, ordenación, detalles, actualización manual y refresco periódico
  opcional que **no provoque saltos de selección ni carga excesiva**."
- **S4 — conflicts are recognized and explained.** "Reconocer conflictos comunes de desarrollo,
  incluidas diferencias entre **loopback y todas las interfaces**, puertos compartidos y ocupación por
  múltiples procesos. Informar si la causa puede encontrarse **fuera del ámbito inspeccionable**."
- **S5 — host and guest scopes are never mixed.** "En entornos con separación host/invitado,
  **identificar claramente el ámbito** al que pertenece cada resultado; nunca mezclar PIDs ni asumir
  que un proceso del host es controlable desde el invitado." In WSL the screen says "WSL guest", and
  states plainly that the Windows host's own ports and processes are not visible from here and cannot
  be stopped from here.
- **S6 — one stop, previsualized, confirmed and revalidated.** "Acción para detener un proceso: mostrar
  identidad, propietario, puertos afectados y consecuencias; **exigir confirmación; revalidar PID y,
  cuando sea posible, tiempo de creación/identidad** para evitar reutilización; proteger el proceso del
  installer y procesos críticos. **Detención suave primero y forzada solamente como acción adicional
  inequívoca.**"
- **S7 — the outcome is verified, never assumed.** "Tras cualquier intento de detenerlo, comprobar si
  sigue vivo y si el puerto sigue ocupado; reportar éxito, falta de permisos, proceso ya finalizado o
  fallo, **sin falsos positivos**."
- **S8 — copying and diagnostics carry no secrets.** "Operaciones de copiar detalles o diagnóstico
  reproducible **sin incluir secretos en argumentos/entorno**."
- **S9 — the prohibitions are code, not advice.** "**No matar por nombre de proceso, no matar todos los
  procesos de un puerto en bloque, no elevar privilegios silenciosamente y no admitir terminación del
  host desde WSL** salvo soporte explícito y selección consciente del ámbito."
- **S10 — the edge cases have answers.** "Manejar procesos efímeros, **PID reciclados**, colisiones de
  direcciones, puertos no inspeccionables y herramientas del sistema no disponibles."
- **S11 — the tests cover the brief.** "Tests de descubrimiento, permisos, IPv4/IPv6, cambios entre
  refrescos, protección de procesos, revalidación antes del kill y modo WSL. Si procede, pruebas
  integradas con **servidores temporales propiedad del propio test**, sin terminar procesos reales del
  usuario."
- **S12 — platform support is the project's, with degradation.** "Compatibilidad total con lo que
  soporte el proyecto... Detecta capacidades reales y ofrece **degradación elegante**... Una plataforma
  sin capacidad específica **no debe provocar un fallo global**." macOS, Linux, Termux and WSL use the
  reader each allows; a platform with no reader says so and the rest of the installer is unaffected.
- **S13 — integration is with the existing interface.** "utiliza la TUI, componentes, tema, navegación
  y patrones de interacción existentes... Prioriza navegación por teclado... Mantén la UI responsiva."
- **S14 — nothing dangerous happens by inference.** "no interpolar entradas en comandos de shell;
  proteger rutas, credenciales y secretos... Toda acción que cambie o elimine algo requiere
  previsualización y consentimiento explícito."

## Decisions

- **D1 — the discovery and stop logic lives in a new package `installer/internal/portscan`, not inside the
  TUI.** The TUI holds state and draws; the package holds the readers, the parsers, the protection
  rules, the revalidation and the stop. The boundary is an injectable `Probe`, the same shape the shell
  audit and terminal probes already use, so the destructive path is testable with doubles and the
  tests never signal a real foreign process.
- **D2 — no external binary is required.** On Linux and Termux `/proc/net/*` is read directly and
  ownership is resolved through `/proc/<pid>/fd`; `ss` and `netstat` are only fallbacks for a host
  without a readable `/proc`. On macOS `lsof -F` is the primary reader and `netstat -anv` the fallback,
  and a host with neither reports why instead of failing. No shell is ever involved: every command is
  an argv executed directly.
- **D3 — one stop, one PID.** The screen never kills by name and never iterates a port's processes;
  the selected listener's process is the only candidate, and its other ports are listed as affected
  before the confirmation.
- **D4 — the forced signal is a second, separate action.** `SignalTerminate` is the default the
  confirmation offers; `SignalKill` only appears on the confirm screen once a terminate attempt left
  the process running, as its own row with its own wording.
- **D5 — protection and scope are checked before the signal and re-checked inside `Stop`.** PID 0 and
  1, the installer's own PID and its ancestor chain, and a small list of OS init/session daemons are
  refused with the reason. A WSL guest never claims control of a host process, because none is ever
  listed: the scope label and the limit line say so.
- **D6 — identity revalidation is token-based.** Linux uses the boot id plus `/proc/<pid>/stat` field 22
  (start time in clock ticks); macOS uses the raw `ps -o lstart=` string. When a platform supplies no
  token, the screen says identity cannot be revalidated and only liveness is checked, rather than
  pretending.
- **D7 — the copy action writes OSC 52 and reports what it did, without a clipboard helper.** The
  diagnostic text is built from redacted data only. When the terminal does not answer the capability
  query, the screen says the terminal did not take it and leaves the diagnostic on screen to copy by
  hand.
- **D8 — the periodic refresh preserves the selection by identity key** (`protocol`, address, port,
  inode and, when known, PID) and reports what changed as a count on the screen; it never moves the
  cursor and pauses on the confirmation screen and while a filter is being typed.

## Interface (frozen before T3/T4)

```go
package portscan

type Protocol string   // "tcp", "tcp6", "udp", "udp6"
type State string      // "LISTEN", "UNCONN"
type Signal int        // SignalTerminate, SignalKill

type Process struct {
    PID          int
    Name         string
    User         string
    Exe          string
    Args         []string  // already redacted
    StartedAt    time.Time // zero when the platform gave no parseable start time
    StartedLabel string    // the platform's own wording when it has one
    Scope        string    // "this machine" or "WSL guest"
    // unexported identity token; compared through SameIdentity
}

type Listener struct {
    Protocol Protocol
    Address  string // numeric, brackets stripped: "127.0.0.1", "::", "0.0.0.0"
    Port     int
    State    State
    Inode    string
    Owner    *Process
    OwnerErr string
}

type Source struct { Name string; Rows int; Err string }
type Snapshot struct {
    At        time.Time
    Scope     string
    Listeners []Listener
    Sources   []Source
    Limits    []string
}

type Probe interface {
    Now() time.Time
    GOOS() string
    IsWSL() bool
    SelfPID() int
    Parent(pid int) (int, bool)
    ReadFile(name string) ([]byte, error)
    ReadDir(name string) ([]os.DirEntry, error)
    Readlink(name string) (string, error)
    LookPath(name string) (string, error)
    Run(ctx context.Context, name string, args []string, env []string) (CommandResult, error)
    Alive(pid int) (bool, error)
    Signal(pid int, sig Signal) error
    LookupUser(uid int) (string, error)
}

type StopTarget struct { Process Process; Sockets []Listener }
type StopOptions struct { Signal Signal; Grace, Poll time.Duration }
type Outcome string
type StopResult struct {
    Outcome        Outcome
    Signal         Signal
    Message        string
    Alive          bool
    PortsStillHeld []Listener
    Verified       bool
}

func Discover(ctx context.Context, p Probe) Snapshot
func RedactArgs(args []string) []string
func SameIdentity(a, b Process) bool
func StopRefusal(p Probe, target StopTarget) string   // pre-signal protection/scope refusal, "" when allowed
func Stop(ctx context.Context, p Probe, target StopTarget, opts StopOptions) StopResult
func DefaultProbe() Probe

// CommandResult is what one executed argv produced; both streams are read
// because a tool that warns on stderr while answering on stdout is reporting a
// limitation the screen has to be able to show.
type CommandResult struct { Stdout, Stderr []byte }
var ErrNoSuchProcess, ErrPermissionDenied error
func (s Signal) Forced() bool
func (s Signal) String() string   // "SIGTERM" / "SIGKILL"
func (l Listener) Endpoint() string
func (l Listener) Key() string
func (l Listener) Wildcard() bool
func (l Listener) Loopback() bool
func (s Snapshot) ListenersFor(pid int) []Listener
const ScopeSelf, ScopeWSLGuest = "this machine", "WSL guest"
```

## Tasks

- **T1 — the `portscan` package: types, probe leaves and the Linux/Termux reader.** S1, S2, S10, S12.
  Route: inline (parent). Files: `installer/internal/portscan/{portscan.go,probe_unix.go,probe_windows.go,
  discover_linux.go,discover_darwin.go,discover_unsupported.go}`. Commit: `feat(portscan): read the
  sockets a machine has bound and the processes behind them`.
- **T2 — the safety path: redaction, protection, revalidation and the verified stop.** S2, S6-S9, S14.
  Route: inline (parent). Files: `installer/internal/portscan/{safety.go,stop.go}` plus the darwin
  reader. Commit: `feat(portscan): stop one process only after revalidating its identity`.
- **T3 — `portscan` tests, doubles and the own-child integration test.** S1-S11. Route: inline (parent).
  Files: `installer/internal/portscan/*_test.go`. Commit: `test(portscan): pin the readers, the
  protection rules and the revalidated stop`.
- **T4 — the TUI state and wiring: screen constants, model, options, utilities row, panel, dispatch and
  keys.** S3, S5, S9, S12, S13, S14. Route: worker `gentle-ai-worker`, independent of T1-T3 file sets,
  frozen interface above. Files: `installer/internal/tui/{model.go,update.go,view.go,panels.go}` and
  `installer/internal/tui/portmanager.go`. Commit: `feat(installer): the Utilities section finds what
  holds a port`.
- **T5 — the port screens render: list, search, filters, details, refresh and the stop confirmation.**
  S3, S4, S6, S7, S8, S13. Route: worker `gentle-ai-worker`, same unit as T4. Commit included in T4.
- **T6 — TUI tests and frame guards for the new screens.** S3, S5-S8, S11, S13. Route: worker
  `gentle-ai-worker` then inspected by the parent. Files: `installer/internal/tui/portmanager_test.go`
  and the guard lists in `teatest_test.go` / `screen_coverage_test.go`. Commit: `test(installer): pin
  the port screen's filtering, refresh and stop confirmation`.
- **T7 — docs and help text.** S1-S10. Route: inline (parent). Files: `docs/tui-installer.md`,
  `README.md` (feature bullet). Commit: `docs: record the port and process utility`.
- **T8 — verification: `make check`, the full suite, and an independent verifier over the safety path.**
  S6, S7, S9, S11. Route: `gentle-ai-verify` + parent. Commit: none.

## Log

- **L1 (the user's request, verbatim):**

```
# Agente 02 — Port & Process Manager

**Misión:** implementar una herramienta interactiva para localizar qué está usando un puerto, entender
el proceso implicado y liberarlo de manera controlada cuando sea seguro.

## Alcance funcional

- Listar conexiones/puertos **en escucha o enlazados** según la visibilidad real del sistema: TCP y,
  donde la API lo permita, UDP; IPv4 e IPv6; dirección local, interfaz, protocolo y estado. No equiparar
  escucha TCP y socket UDP.
- Resolver el proceso propietario cuando sea posible: PID, nombre, propietario, ejecutable, argumentos
  redactados y hora de inicio. Un PID desconocido no equivale a puerto libre. Mostrar limitaciones de
  permisos sin inventar información.
- Búsqueda rápida por puerto, proceso o protocolo; filtros por ámbito, ordenación, detalles,
  actualización manual y refresco periódico opcional que no provoque saltos de selección ni carga
  excesiva.
- Reconocer conflictos comunes de desarrollo, incluidas diferencias entre loopback y todas las
  interfaces, puertos compartidos y ocupación por múltiples procesos. Informar si la causa puede
  encontrarse fuera del ámbito inspeccionable.
- En entornos con separación host/invitado, **identificar claramente el ámbito** al que pertenece cada
  resultado; nunca mezclar PIDs ni asumir que un proceso del host es controlable desde el invitado.
- Acción para detener un proceso: mostrar identidad, propietario, puertos afectados y consecuencias;
  exigir confirmación; revalidar PID y, cuando sea posible, tiempo de creación/identidad para evitar
  reutilización; proteger el proceso del installer y procesos críticos. Detención suave primero y
  forzada solamente como acción adicional inequívoca.
- Tras cualquier intento de detenerlo, comprobar si sigue vivo y si el puerto sigue ocupado; reportar
  éxito, falta de permisos, proceso ya finalizado o fallo, sin falsos positivos.
- Operaciones de copiar detalles o diagnóstico reproducible sin incluir secretos en argumentos/entorno.

## Seguridad y casos límite

No matar por nombre de proceso, no matar todos los procesos de un puerto en bloque, no elevar
privilegios silenciosamente y no admitir terminación del host desde WSL salvo soporte explícito y
selección consciente del ámbito. Manejar procesos efímeros, PID reciclados, colisiones de direcciones,
puertos no inspeccionables y herramientas del sistema no disponibles.

Tests de descubrimiento, permisos, IPv4/IPv6, cambios entre refrescos, protección de procesos,
revalidación antes del kill y modo WSL. Si procede, pruebas integradas con servidores temporales
propiedad del propio test, sin terminar procesos reales del usuario. El resultado debe ser una utilidad
funcional y segura, no una tabla decorativa.

---

## Contexto y condiciones obligatorias

Estás trabajando en un fork de los dotfiles de Gentleman Programming que ya incluye un
installer/configurador TUI con navegación, estilos y temas dinámicos, mascota, utilidades e integración
con el entorno de desarrollo. Tu cometido es **implementar esta funcionalidad realmente**, no entregar
un diseño, pseudocódigo o una maqueta sin lógica.

- **Antes de modificar nada**, examina el repositorio y localiza su arquitectura, convenciones,
  comandos, utilidades compartidas, navegación, estados, tratamiento de errores y tests. Adapta tu
  solución a lo que existe; no presupongas un lenguaje ni un framework.
- **Compatibilidad total con lo que soporte el proyecto:** todos sus sistemas operativos, plataformas,
  entornos (incluido WSL cuando corresponda), shells y emuladores de terminal. No acoples la
  implementación a una shell concreta ni confundas el entorno del host con el de WSL. Detecta
  capacidades reales y ofrece degradación elegante cuando alguna API/función no exista. Una plataforma
  sin capacidad específica no debe provocar un fallo global.
- **Experiencia integrada:** utiliza la TUI, componentes, tema, navegación y patrones de interacción
  existentes. Prioriza navegación por teclado, terminales con capacidades gráficas distintas, mensajes
  claros, progreso y cancelación cuando proceda. Mantén la UI responsiva.
- **Tecnología actual y mantenida:** cuando necesites herramientas o bibliotecas, investiga las
  alternativas vigentes, consolidadas, seguras y apropiadas para el ecosistema del proyecto. Prefiere
  APIs oficiales y mecanismos nativos bien soportados. No introduzcas herramientas obsoletas/deprecadas,
  dependencias innecesarias ni reinventes soluciones maduras sin motivo. Comprueba versiones y
  compatibilidad; no impongas un binario externo como dependencia obligatoria si existe una opción más
  fiable.
- **Seguridad por defecto:** nada de telemetría, envío de datos o funciones de IA; no ejecutar comandos
  peligrosos por inferencia; no interpolar entradas en comandos de shell; proteger rutas, credenciales y
  secretos; manejar permisos, timeouts, procesos concurrentes y errores parciales. Toda acción que
  cambie o elimine algo requiere previsualización y consentimiento explícito, salvo que la aplicación ya
  tenga un flujo inequívoco equivalente. Emplea copias de seguridad y reversibilidad donde sea viable;
  nunca afirmes que una operación es reversible si no lo es.
- **Coexistencia con otros agentes:** otros agentes implementan otras utilities simultáneamente.
  Mantén tu código encapsulado, reutiliza interfaces existentes, evita refactors globales o cambios
  innecesarios en ficheros compartidos y no alteres otras funcionalidades. Si existe funcionalidad
  similar, intégrate con ella en lugar de duplicarla.
- **Calidad:** código mantenible, tipado cuando corresponda, datos/estados reales y no simulados, errores
  recuperables, tests automatizados con dobles de prueba para acciones destructivas, cobertura de casos
  límite y validaciones del repositorio. No toques la configuración real del desarrollador durante
  tests.

**Entrega esperada:** cambios funcionales en el repositorio, integrados y probados. Al terminar, informa
brevemente de archivos modificados, decisiones importantes, comprobaciones ejecutadas, cobertura real y
limitaciones detectadas. No te quedes en un plan: implementa.
```

- **L2 — repository conventions located before the first write.** `installer/internal/tui` holds the
  Bubbletea model in `model.go`, key handling in `update.go`, drawing in `view.go` and the side panel
  in `panels.go`; the Utilities section (`ScreenUtilities`) opens sub-screens one level in
  (`ScreenWSLResources`, `ScreenTerminalCapabilities`, `ScreenShellAudit`, `ScreenUpdate`,
  `ScreenThemePicker`). New screens are measured by the frame guards through
  `installerFrameScreenNames` or `screensTheInstallerStatesNeverReach`, and renders never read the
  clock, the environment or a file. Platform facts live in `installer/internal/system`.
- **L3 — the reader facts, checked on this host.** `/proc/net/tcp`, `tcp6`, `udp` and `udp6` are
  readable here; `/proc/<pid>/fd` symlinks name `socket:[inode]`; `/proc/<pid>/stat` carries the start
  ticks and `/proc/stat` the boot time; `ss` is available as a fallback. macOS keeps `lsof -F` and
  `netstat -anv` as its two readers. WSL exposes only guest sockets inside the distro, so the Windows
  host is reported as outside the inspectable scope rather than mixed in.
- **L4 — interface amendment before the TUI work started.** Implementing T1/T2 changed three things in
  the frozen block above, all of them additive and recorded here before T4 was handed off:
  `LookupUser(uid) (string, error)` was added to `Probe` so the account name is injectable rather than
  read with `os/user` inside the package; `Run` returns a `CommandResult` with both streams instead of
  a bare `[]byte`, because a tool that warns on stderr while answering on stdout is reporting a
  limitation the screen must be able to show; and `ErrNoSuchProcess` / `ErrPermissionDenied` name the
  two signal failures so the outcome is classified by the package instead of by a platform error
  string. `Signal.String()`, `Signal.Forced()`, `Listener.Endpoint/Key/Wildcard/Loopback` and
  `Snapshot.ListenersFor` are the small helpers the TUI reads.
