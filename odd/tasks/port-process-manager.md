# Port and process manager

## Goal

A utility inside the installer's Utilities section that answers "what is holding this port, and may I
free it?" with real data from the machine: the bound sockets it can actually see, the process identity
behind each one where the kernel allows it, and one deliberate, revalidated stop of a single process.
It sits with the section's other utilities and is shaped like them: a read-only-capable screen one level in
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
  sockets a machine has bound and the processes behind them` (`33b01a8`).
- **T2 — the safety path: redaction, protection, revalidation and the verified stop.** S2, S6-S9, S14.
  Route: inline (parent). Files: `installer/internal/portscan/{safety.go,stop.go}` plus the darwin
  reader. Commit: `feat(portscan): stop one process only after revalidating its identity` (`954ffb6`).
- **T3 — `portscan` tests, doubles and the own-child integration test.** S1-S11. Route: inline (parent).
  Files: `installer/internal/portscan/*_test.go`. Commit: `test(portscan): pin the readers, the
  protection rules and the revalidated stop` (`5de1f6f`).
- **T4 — the TUI state and wiring: screen constants, model, options, utilities row, panel, dispatch and
  keys.** S3, S5, S9, S12, S13, S14. Route: worker `gentle-ai-worker`, independent of T1-T3 file sets,
  frozen interface above. Files: `installer/internal/tui/{model.go,update.go,view.go,panels.go}` and
  `installer/internal/tui/portmanager.go`. Commit: `feat(installer): the Utilities section finds what
  holds a port` (`7ededd7`), which carries T5 and T6 with it on purpose: a screen's code, its tests, its
  frame guards and its goldens are one reviewable unit, because a screen that landed without its guard
  would leave `TestEveryScreenIsCoveredByAFrameGuard` red and a guard landed without its screen measures
  nothing.
- **T5 — the port screens render: list, search, filters, details, refresh and the stop confirmation.**
  S3, S4, S6, S7, S8, S13. Route: worker `gentle-ai-worker`, same unit as T4. Commit included in T4
  (`7ededd7`).
- **T6 — TUI tests and frame guards for the new screens.** S3, S5-S8, S11, S13. Route: worker
  `gentle-ai-worker` then inspected by the parent. Files: `installer/internal/tui/portmanager_test.go`
  and the guard lists in `teatest_test.go` / `screen_coverage_test.go`. Commit: rolled into T4's
  (`7ededd7`) for the reason recorded there, plus `test(installer): pin what the port copy action puts on
  the clipboard` (`a83d2d4`) for the one behaviour the worker's tests left open -- what the copy action
  hands to the terminal, and the two gates in front of it -- written by the parent because the independent
  review of this section came back unavailable.
- **T7 — docs and help text.** S1-S10. Route: inline (parent). Files: `docs/tui-installer.md`,
  `README.md` and `README.es.md` (the utility's own paragraph in each language). Commit: `docs: record
  the port and process utility` (this commit).
- **T8 — verification: `make check`, the full suite, and an independent verifier over the safety path.**
  S6, S7, S9, S11. Route: `gentle-ai-verify` + parent. Commit: none of its own; its findings are fixed
  in `d86b55f` (`fix(portscan): mask every credential and never call an unread port free`), `e0079e7`
  (`fix(portscan): mask the pass flag, a joined password and an escaped quote`) and `535e738`
  (`fix(portscan): mask the user%password form the SMB clients take`), recorded in L5 and L6, and
  re-verified by two further bounded verifiers, whose third round found six more credential forms and the
  over-masks fixed in `d757bb1` (L7). The independent review of the TUI section was launched three times:
  the first pair never started (the worker's cleanup failure quarantined capacity), the relaunched
  reviewer failed after sixty-two turns without a report, and that is recorded here as an **unavailable
  verifier rather than retried**, because a third launch is ceremony and not evidence. What stands in its
  place is named: the ten invariants are covered by the tests listed in `portmanager_test.go`, the two
  goldens, the frame guards the section was measured by, and the parent's read of the diff against the
  frozen `portscan` interface.

## Log

- **L1 (the user's request, quoted word for word except where a word is marked as elided. The upstream
  project's own name is elided as `[the upstream project]`: this repository's branding audit fails on it
  outside its exclusion list, and no other document under `odd/tasks/` spells it out. Nothing else in the
  quote is changed):**

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

Estás trabajando en un fork de los dotfiles de [the upstream project] que ya incluye un
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
- **L5 — the independent verification of the safety path found two real failures, both fixed.** A
  read-only verifier reviewed the package against S1-S14 with its own probes in a scratch copy and
  reported: **B1**, `RedactArgs` left credentials that any real process can carry unredacted --
  `curl -u alice:s3cret`, `--user=alice:s3cret`, `mysql -ps3cret`, `--pw=s3cret`, and a JSON body on
  the command line (`{"password":"hunter2"}`) -- which S8 forbids, because those arguments are shown
  on the screen and copied into the diagnostic; and **B2**, the post-stop verification reported "The
  ports it held are free" when the fresh reading had failed on every reader, which is a false
  positive about occupancy that S7 forbids and an empty list reading as "nothing is bound" that S12
  forbids. It also raised three limitations: a blank limitation string reached `Snapshot.Limits` from
  the `ss` fallback with an empty stderr; `StopRefusal` skipped the scope check when a hand-built
  target carried no scope at all; and a denied `/proc/<pid>/fd` link was reported as a possible exit
  rather than as the permission it is. All five are fixed in one work unit
  (`safety_probe_test.go` carries one regression test per finding), and four decisions are recorded
  here rather than left in the code: a **user:password pair is masked whole** while a **bare account
  name is not masked**, because the account identifies which service the process is and is not the
  secret; a **credential flag's inline value is masked only when it is a pair**, so `--user=alice`
  stays readable; an **unknown scope fails closed** in `StopRefusal`, since a signal across a
  host/guest boundary decided by nobody's reading is the mistake the scope rule exists to prevent;
  and the post-stop sentence is now **per transport**, so a reading that answered for TCP and failed
  on UDP says exactly that instead of calling either one free. The macOS identity token keeps the
  one-second granularity `ps lstart` reports (D6): a PID reused inside that second is still not
  distinguishable, and the screen says which token it compared.
- **L6 — the second verification round, and what its three leaks changed.** A bounded re-verification of
  `d86b55f` confirmed both blockers closed and the fail-closed scope honest, then found three more
  credential forms that still survived `RedactArgs`, all of them argv a real process can carry: the word
  `pass` was not a secret name, so `openssl -pass pass:s3cret`, `--pass=`, `--pass` and `DB_PASS=`
  leaked; a short password flag was only masked when its value was joined, so `sshpass -p s3cret`
  leaked; and the embedded scan stopped at an escaped quote, so a JSON body that escapes its quotes
  (`{"password":"a\"s3cret"}`) left the rest of the secret in the clear. It also found the reverse
  cost, over-masking: `-print`, `-path` and `-p8080:80` were eaten by the joined-short rule. All five
  are fixed in `e0079e7`, and `blindTo` now compares **protocols rather than transports**, because the
  `/proc/net` files are one table each: a reading that lost `tcp6` cannot call an IPv6 port free on the
  strength of its IPv4 sibling, while `lsof`, `netstat` and `ss` answer for a whole transport in one
  call. Four decisions are recorded with the fix: `pass` and `sessionid` join the secret names, the
  latter because a session identifier is a bearer credential by name; a **flag whose own name is a
  secret masks its value unconditionally**, since a password that begins with a dash is still a
  password; `-p` standing alone masks the value apart from it **unless the value reads as a port or an
  address**, which is what keeps `ssh -p 2222` and `docker -p 8080:80` readable; and a joined short
  value is a password only when it carries a character a flag name would not, because `-print` and
  `-path` begin with the same letter. Two boundaries are deliberate and stay: a joined `-p` value that
  is a bare word with no digit, upper-case letter or punctuation (`-psecret`) is read as the flag name
  it most often is, and an all-digit `-p 123456` is read as the port it most often is. Both are the
  price of the diagnostics that stay legible, and both are stated rather than hidden.
- **L7 — the third verification round, and the rules it rewrote.** A bounded re-verification of `23b272d`
  ran the redaction rules against the argv real tools take rather than against the shapes the rules were
  written for, and six of those forms survived: curl's `--proxy-user alice:s3cret`; `redis-cli -a
  s3cret`; `mysql -psecret db`, which both showed the password and masked the database name after it,
  because the suffix rule read `-psecret` as a flag whose own name ends in a secret word; `-p123456`
  outright and `-p 123456` because an all-digit value was read as a port; a personal access token carried
  as `https://s3cret@host/repo.git`, whose userinfo has no colon; and `sshpass -p -s3cret host`, whose
  value begins with a dash and was skipped as a flag. The same round found the reverse cost in
  `--bypass=true`, `--compass north`, `--overpass x`, `--pass-through x` and `--token-file /etc/token`,
  and three coverage gaps: `sessionid=` had no test, the decision that `--user=alice` stays readable was
  unpinned, and the empty-scope refusal was asserted by its neighbour's wording. All of it is fixed in
  `d757bb1`, with `credential_probe_test.go` carrying one regression test per finding. The rules that
  changed, and why: the short secret words (`pass`, `pw`, `pwd`) count **only at the end of a name**,
  because `bypass`, `compass`, `overpass` and `--pass-through` contain them and name no credential; a
  segment after a **negation** is not a secret, because `--no-auth` takes none; a secret word followed by
  the name of a **container** (`file`, `path`, `dir`, `env`, `id`, ...) is not one either, because
  `--token-file` takes a path; the standalone password letters are `-p` for mysql and `-a` for
  redis-cli, and their value apart is masked unless it **can be a port** -- a number inside the port
  range, an address, a mapping or a path -- which is what keeps `ssh -p 2222`, `docker -p 8080:80` and
  `ls -a /tmp` readable while `-p 123456` and `redis-cli -a s3cret` are masked; a joined short value is a
  flag name only when it is a bare word that carries no secret (`-print`, `-path`, `-pfoo`), so
  `-psecret` is masked and the argument after it is left alone; a URL userinfo with no colon is masked
  when it carries a character a name would not (`https://ghp_Abc123@host`, `https://s3cret@host`) and
  kept when it is a plain account name, and a `user:password@host` with no scheme is masked too; and a
  value after a standalone short flag is masked even when it begins with a dash, because `-t` carries
  nothing a flag name would not and `-s3cret` does. This supersedes the two boundaries L6 called
  deliberate: `-psecret` is a password, and an all-digit `-p` value is a port only inside the port
  range. Two over-masks remain and are stated: a plain word or a bare relative filename after a
  standalone `-p` or `-a` (`grep -a foo file`, `tar -p notes.txt`) is masked, and a compound name whose
  container word is not on the list (`--api-key-file`) keeps the secret reading. Both are the safe side
  of the same trade: a value the diagnostic hides costs fidelity, a value it shows costs a credential.
  The same round's review of this repository's own copy action is `a83d2d4`.

- **L8 — the delivery, as a chain of three pull requests.** The user authorized the remote work and,
  when one pull request was measured against the review budget, chose a **chain of three stacked pull
  requests to `main`** over a single oversized one. The three slices are the package, the screen and the
  documents, each separately green:

  | Slice | Branch | Commits | Changed lines |
  |---|---|---|---|
  | the package | `feat/port-process-manager` | `33b01a8`, `954ffb6`, `5de1f6f`, `d86b55f`, `e0079e7`, `535e738`, `d757bb1`, `11a610e`, `9abd43c` | 20 files, 6,338 insertions |
  | the screen | `feat/port-process-manager-tui` | `7ededd7`, `a83d2d4`, and a merge of the package branch in `59585e3` and again in `561d0d1` | 13 files, 2,157 insertions, 22 deletions |
  | the documents | `feat/port-process-manager-docs` | `83b1029`, `23b272d`, `5c1e956`, `52efeb5`, `de32fd1`, `51f1484`, and a merge of the screen branch in `f9f47ad` and again in `c7efc0b` | 5 files, 659 insertions, 9 deletions |

  The chain is built on `a24ced6`, the `main` the work was rebased onto, because `main` moved while the
  feature was in flight: two screens landed in the same list, so `ScreenPortManager` and
  `ScreenPortStop` had to leave the areas block for the end of the screens block or the section's own
  rows would have read `Finishing up`; the Utilities body gained a tenth job's line at 80x24, where a
  job's line is pushed past the cut unless the newest one yields; and the guard's measured screen count
  became 93. Each was resolved by `main`'s own rule rather than by loosening a guard -- the newest line
  yields in the body, and the count that is measured is not the count that is summed -- and slice B also
  folded in the guards, the golden and the screen names its own screens changed, so the slice is green on
  its own rather than green only after the next one.

  The commits above are the delivered identities. The hashes the earlier entries name belong to the
  pre-rebase branch and are superseded by these; each pair is the same commit by subject, verified
  before the rewrite. `main`'s documents changed the same regions the port utility's own documents
  touch, so slice C keeps `main`'s sentences and adds the port utility to them: the section now holds
  **ten** jobs, the count `TestTheUtilitiesDocNamesEveryJobTheSectionOffers` reads went from `nine` to
  `ten` with the new job in its list, and the panel table keeps `main`'s newest row with the port reading
  appended. One claim did not survive that merge and was dropped rather than renumbered: the utility is
  no longer described as "the sixth", because the row that holds it is not the sixth of the section's
  rows and the ordinal had become a false statement. The three pull requests are **#351** (the package,
  against `main`), **#352** (the screen, based on #351's branch) and **#353** (the documents, based on
  #352's branch): each base is the previous branch so that every pull request's diff is its own slice,
  and each one is retargeted as the one before it merges.

- **L9 — the macOS runner found the defect every host-only check had hidden.** The first CI run of #351
  failed exactly one job: `macOS smoke test`, which runs `go test ./...` on a macOS runner. The package
  did not compile there at all -- `undefined: splitEndpoint`, `normalizeHostForFamily`, `socketState`,
  `discoverLinux`, `parseProcStat`, `readProcProcess`, `readBootFacts` -- while the same command was
  green on Linux, and it had been green in every earlier round of verification, because every one of
  them ran on this host, and this host is Linux. Two things had hidden it: `discover_mac.go` carries no
  build constraint, since `_mac.go` is not a GOOS suffix, so the macOS reader compiled everywhere while
  the helpers it used lived in the Linux-only `discover_linux.go`; and `Discover`'s deliberate *runtime*
  switch on `p.GOOS()` named `discoverLinux` in a case body, so the reference itself only compiled where
  that symbol existed. The untagged `stop.go` and the `!windows` probe called `/proc` fact readers the
  same way, and `parsers_test.go` was untagged and named a Linux-only helper, which is the same defect
  inside the test tree. `go build ./cmd/dotfiles` passed on macOS throughout because nothing in this
  slice imports the package yet -- the TUI does, in slice B -- so only the compile of the tests could
  see it.

  The fix is `11a610e`, and it puts the OS boundary back where the design already had it, in the Probe:
  the readers are portable Go over a Probe, so the `/proc` reader moved to the untagged
  `discover_proc.go` and the shared parsers to `parse.go`, which leaves the runtime switch compiling on
  every platform and keeps the macOS reader reachable from a Linux test through a fake probe. What is
  genuinely per-platform is the process facts: `probe_linux.go` reads `/proc`, `probe_darwin.go` asks
  `ps -o lstart=`, and `probe_windows.go` answers honestly that it cannot revalidate. On macOS
  `readBootFacts` is the zero value on purpose, because there is no boot id to compare and the identity
  token is the raw `ps` string -- that is D6, not a gap -- and the unreachable `parseProcStat` parity
  stub says so in its own words. The fixtures split with the readers: the `/proc`, `ss` and `netstat`
  tables became `//go:build linux`, the `lsof` and `ps` fixtures stayed untagged so the macOS runner
  executes them, and one new test pins the macOS identity as the raw `ps lstart` string while running on
  Linux. `TestThePackageTypeChecksForEveryPlatformItClaims` vets the package for linux, darwin and
  windows in child processes and names the platform that broke, so a host-only build cannot hide the
  next one of these. The plan in T1 named `discover_darwin.go` for the macOS reader; a `_darwin.go`
  suffix is what must *not* be used there, because it would hide the reader from the Linux-side tests
  that cover it, so the file kept its untagged name and this entry records the deviation.

  Verification of the fix, run by the parent on this worktree: `GOOS=linux|darwin|windows go build` and
  `go vet` for the package are clean, the new guard passes for the three platforms, the package suite is
  green, the TUI consumer's port tests are green, the moved test bodies are byte-identical under
  comparison (twelve of them, none lost, two added), and the exported API has nothing added or removed.
  Real macOS execution remains CI's: the darwin probe's `Parent`, `Alive` and `Signal` and the real
  `lsof` run only type-check here. The second CI run of #351 is what confirms the fix, and it was still
  running when this entry was written. Slices B and C carry the fix through merge commits (`59585e3`,
  `f9f47ad`) rather than a history rewrite, because the harness blocked a force-push of those branches
  and the user chose the merge: each pull request's diff is still its own slice, because each base is
  the previous branch.

  The second macOS run confirmed the build fix and found the next layer of the same blindness: fourteen
  flow tests failed, not because they compiled wrongly but because they were written in one platform's
  dialect. On darwin the identity reader asks `ps`, the fixtures stated each process only in the `/proc`
  files, and every stop therefore read an identity it could not revalidate and refused before reaching the
  decision the test pins -- the refusal was right behaviour and the fixture was one-sided. `9abd43c`
  makes the fake derive the two `ps` runs from the same `/proc` state: the account from `status`, the
  executable from `exe` or `comm`, the start wording built from `/proc/stat`'s `btime` plus the process's
  start ticks, and the argv from `cmdline`, with `ps` on the fake's PATH because the reader asks
  `LookPath` first, and no identity line at all for a pid whose start fact is absent, which is what keeps
  an unreadable identity testable on both platforms. A test now states each fact once, and the mutations
  a test already made between two readings -- recycled start ticks, a vanished start file, a port that
  moved -- reach the darwin dialect with no second fixture: `stop_test.go` did not change at all for this.
  Two new untagged tests drive the very fixture the flow tests build through `readDarwinProcess`, which is
  untagged and therefore runs on Linux, and pin the same facts the `/proc` reader yields from it; a typo
  in the bridge would otherwise arrive as a platform failure on the runner instead of as a fixture bug in
  this worktree. No assertion was weakened, no test gained a build constraint, and no production file
  changed. The cross-dialect start time is compared to the second rather than to the instant, because
  `/proc` counts start ticks in 10 ms while `ps lstart` is one-second granular (D6). The third run of both
  workflows is green, the macOS smoke job included, and that job is the only place those fourteen tests
  run against the darwin reader. What stays outside this worktree's reach is unchanged: the darwin
  probe's `Parent`, `Alive` and `Signal` and the real `lsof` run only type-check here.
