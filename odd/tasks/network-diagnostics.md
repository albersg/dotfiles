# Feature: a network diagnostics centre in the Utilities section

Status: in progress
Opened: 2026-10-16
Front: the network diagnostics utility (agent 08). One task file, one writing thread.
Worktree: `../worktree-calm-forest-ae99`, branch `worktree/calm-forest-ae99`, from the `installer` Go module.

## Specs

S1. **Diagnostic, never offensive.** No mass port or network scans, no automatic changes to DNS,
firewall, proxy or certificates, no TLS bypass, no automatic installation of tools. Every check
reads; nothing is configured.

S2. **One check per category, and the stages are named.** Interfaces and addresses, route/gateway
when it is accessible, DNS resolution, TCP connectivity, orientative latency, HTTP(S) requests and
TLS validation. The report distinguishes the local network, DNS, the transport, the application and
the certificate. A blocked ping is never read as "no Internet": nothing here depends on ICMP, and
the latency that is reported is the TCP handshake time, said so in the report.

S3. **The user's target, and predefined destinations that are declared.** A domain, host, URL or
port entered by the user is diagnosed, with IPv4, IPv6, internationalized hostnames and URLs
validated before anything is dialled. Predefined destinations are used only when they are
appropriate and transparent: the screen names the host it contacts and why. Redirects, response
size, timeouts and attempt counts are all bounded.

S4. **DNS says what answered and how long it took.** The answer, the server/resolver when it can be
known, and approximate times. Empty answers, cache, multiple addresses, split DNS, corporate
policies and a different resolution on a host and a guest are reported as what they are — a
difference is never asserted to be an error without evidence.

S5. **TCP and HTTP(S) separate the stages.** "DNS answered but the port is closed", timeout,
refusal, TLS error, redirect, application-level HTTP failure and a successful response are told
apart, and each report carries the code, the stage and the duration. Authorization headers, cookies
and private response bodies are never read, kept or shown.

S6. **Proxy and VPN are observed, not judged.** Observable parameters (environment, the platform's
own proxy store, VPN-looking interfaces) are reported with users and passwords redacted. A proxy or
a VPN is not by itself a problem. Certificates are verified; verification is never skipped for
convenience.

S7. **Host and guest are separated.** In WSL the real scope is diagnosed: the guest's interfaces,
its generated resolver configuration, the virtual network and the networking mode, and why the
guest's answers can differ from the Windows host's. No fixed route, IP or topology is assumed.

S8. **Firewall, routes and traceroute only when the capability is real.** Routes come from the
platform's own table; traceroute runs only when the system already has the tool, only for a target
the user entered, and its report says what it needs and why a hop that answers nothing is not a
fault. Deprecated tools are not used where a native API exists.

S9. **Results are presented per step**: severity, evidence, possible causes, the next checks and the
manual actions. The verdict names the concrete stage that failed, or says there is not enough
evidence.

S10. **Export is optional and redacted.** A report can be reviewed as text and saved for support,
with no secrets, and with private (or every) IP address, the user name, the home directory and the
host name replaced when the user chooses to anonymize. The preview comes before the write.

S11. **Bounded, cancellable, low-concurrency.** Every check has a timeout and a size bound, the run
is cancellable from the interface, and concurrency is limited so a slow network is never saturated
and the TUI never blocks. Networks with no Internet, corporate DNS, IPv6 disabled, very slow links,
captive portals, ICMP-filtering firewalls, authenticated proxies, private certificate authorities
and intermittent resolution all produce a report rather than a hang.

S12. **Tests with doubles.** Simulated resolvers, sockets and HTTP servers; DNS, TCP and TLS
failures; invalid URLs; giant responses; timeouts; missing tools; WSL/host scope; credential
redaction; and no side effects. The developer's own configuration is never touched by a test. The
feature is finished when it locates the concrete stage of a failure, or declares uncertainty when
there is no evidence for an answer.

S13. **Built into what exists.** The installer's own architecture, conventions, shared utilities,
navigation, states, error handling and tests. Real implementation, not a design or a mockup.

S14. **Compatible with everything the project supports.** Every operating system and environment the
installer supports, including WSL and Termux, every shell and every terminal it supports. Nothing is
coupled to one shell, and the host's environment is never confused with the guest's. Capabilities are
detected; a platform missing one degrades honestly instead of failing globally.

S15. **Integrated experience.** The existing TUI, its components, theme and navigation: keyboard
first, clear messages, progress and cancellation where they apply, and a responsive interface.

S16. **Current, maintained technology.** Official APIs and native mechanisms; no deprecated tools,
no unnecessary dependencies, no reinvention of mature solutions, and no external binary as a
mandatory dependency. A capability that needs a binary is offered only where that binary exists.

S17. **Security and coexistence.** No telemetry, no data sent anywhere, no AI features, no dangerous
command inferred, no user input interpolated into a shell command. Paths, credentials and secrets are
protected. Other agents are implementing other utilities at the same time: this front stays in its
own files, reuses existing interfaces, avoids global refactors, changes no other utility, and
integrates rather than duplicating anything similar that already exists. Tests use doubles for every
action that writes.

## Tasks

| # | Task | Specs | Route | Commit |
|---|------|-------|-------|--------|
| T1 | Freeze the interface: `internal/netdiag` report, target validation, IDN, redaction, probe seam | S1, S3, S5, S6, S10, S16, S17 | parent, inline | 1 |
| T2 | The environment checks: interfaces, routes, resolver configuration, proxy/VPN, WSL scope | S2, S4, S6, S7, S8, S14 | `gentle-ai-worker` | 2 |
| T3 | The protocol checks: DNS, TCP, latency, HTTP(S), TLS, interception, bounded traceroute | S2, S4, S5, S8, S11 | `gentle-ai-worker` | 3 |
| T4 | The runner: stage order, streaming, cancellation, bounds, report text | S9, S11, S12 | parent, inline | 4 |
| T5 | The screen and the wiring: utilities row, panel entry, input, progressive results, export | S3, S9, S10, S13, S15 | parent, inline | 5 |
| T6 | Guards: netdiag unit tests, TUI wiring tests, frame-guard case, docs | S12, S13, S15 | parent, inline | 6 |
| T7 | Verification: independent verifier over the whole diff, then closing report | S1–S17 | `gentle-ai-verify` | - |

Frozen interface, so T2 and T3 can be written in parallel and cannot drift.
Everything these functions need already exists in `report.go`, `redact.go`, `target.go` and
`probe.go`; they add one file each and change nothing else.

```go
// internal/netdiag/env.go (T2)
func checkInterfaces(p Probe, s Session) Check   // id env.interfaces, StageLocal
func checkRoute(p Probe, s Session) Check        // id env.route,      StageLocal
func checkResolver(p Probe, s Session) Check     // id env.resolver,   StageResolver
func checkProxy(p Probe, s Session) Check        // id env.proxy,      StageProxy

// internal/netdiag/checks.go (T3)
func checkDNS(p Probe, s Session) (Check, Facts) // id dns.resolve,    StageResolver
func checkTCP(p Probe, s Session) Check          // id tcp.connect,    StageTransport
func checkHTTP(p Probe, s Session) Check         // id http.get,       StageApplication
func checkTLS(p Probe, s Session) Check          // id tls.handshake,  StageCertificate
func checkTrace(p Probe, s Session) Check        // id trace.route,    StageTransport
func checkInterception(p Probe, s Session) Check // id http.interception, StageApplication
```

The contract both halves share:

- Each function runs exactly one check and calls no other `check*` function: the runner (T4) decides
  the order, the skips and the interception cross-check. It is why the signatures never need to
  change.
- `s.Endpoint()` is the destination (the user's target, else the predefined one) and `ok == false`
  means there is none: a check that needs one returns `StatusSkipped` with the reason.
- `s.Context()` is the run's context and `s.WithTimeout(bound)` derives one check's bound from it, so
  a stopped run stops dialling and every check is bounded twice (its own stage and the run).
- The bound accessors on `Bounds` (`dns()`, `dnsServer()`, `tcp()`, `tls()`, `http()`, `trace()`,
  `attempts()`, `redirects()`, `bodyBytes()`) already substitute their default for
  a zero field: a check must use them, never the raw field, so no run can be unbounded. There is no
  concurrency bound to read: one check runs at a time, by construction, which is the strongest form
  of that bound.
- Results are built with `newCheck(id, stage, title)`, `(*Check).addEvidence(...)` and
  `(*Check).conclude(status, cause, next, manual)`; lines go through `RedactProxy`,
  `RedactCredentials` and `SafeURL` before they reach evidence, and a response body is never
  evidence — a length, a type or a boolean is.
- Nothing is written, no verification is ever skipped, no host other than the endpoint is contacted,
  and no program is started except through `Probe.ExecOutput` after `Probe.LookPath` confirmed it
  exists.

## Log

### L1 — the request, verbatim

# Agente 08 — Network Diagnostics

**Misión:** ofrecer un centro de diagnóstico de red orientado a desarrollo local y acceso a
servicios, útil para identificar problemas de DNS, conectividad, puertos, proxy y TLS.

## Alcance funcional

- Vista general de conectividad con comprobaciones por categoría: interfaces y direcciones,
  ruta/gateway cuando sea accesible, resolución DNS, conectividad TCP, latencia orientativa,
  solicitudes HTTP(S) y validación TLS. Distinguir red local, DNS, transporte, aplicación y
  certificado; no interpretar un ping bloqueado como ausencia de Internet.
- Diagnóstico de dominio/host/URL/puerto **introducido explícitamente por el usuario** y destinos
  predefinidos solo cuando sean apropiados y transparentes. Validar entradas, IPv4/IPv6, hostname
  internacionalizado y URL; limitar redirecciones, tamaño de respuesta, tiempos y número de intentos.
- DNS: mostrar respuesta, servidor/resolvedor cuando pueda conocerse y tiempos aproximados; gestionar
  respuestas vacías, caché, múltiples IP, split DNS, políticas corporativas y resoluciones distintas
  entre host e invitado sin afirmar errores donde no hay certeza.
- TCP/HTTP(S): distinguir DNS correcto pero puerto cerrado, timeout, rechazo, error TLS, redirección,
  fallo HTTP de aplicación y respuesta exitosa. Reportar códigos, etapas y duración con contexto; no
  exponer cabeceras de autorización, cookies ni cuerpos de respuesta privados.
- Proxy/VPN: detectar parámetros observables y posibles interferencias, redactando usuarios y
  contraseñas. No suponer que la presencia de VPN/proxy es un problema. Comprobar certificados sin
  ignorar verificaciones por comodidad.
- En entornos separados host/invitado, diagnosticar el alcance real y explicar diferencias de
  resolución, interfaces y modo de red; no asumir rutas, IP ni topología fijas.
- Mostrar información de firewall/rutas/traceroute solo cuando exista capacidad fiable, con etiquetas
  claras sobre permisos y limitaciones; no depender de herramientas deprecadas cuando haya APIs
  modernas.
- Presentar resultados por pasos, severidad, evidencia, posibles causas, siguientes comprobaciones y
  acciones manuales. Historial/Exportación opcional de informes redactados para soporte, sin secretos
  ni IP privadas si el usuario elige anonimizar.
- La herramienta debe ser **diagnóstica, no un escáner ofensivo**: sin escaneos masivos de puertos o
  redes, sin cambios automáticos de DNS, firewall, proxy o certificados, sin bypass de TLS ni
  instalación automática de herramientas.

## Robustez y aceptación

Trabajar en redes sin Internet, DNS corporativo, IPv6 deshabilitado, conexiones muy lentas, captive
portals, firewalls que filtran ICMP, proxies autenticados, servicios con certificados propios y
resoluciones intermitentes. Las comprobaciones deben tener timeout, cancelación y concurrencia
limitada para no bloquear la TUI ni saturar una red.

Tests con resolutores, sockets y HTTP simulados; fallos DNS/TCP/TLS; URLs inválidas; respuestas
gigantes; timeout; ausencia de herramientas; alcance WSL/host; redacción de credenciales y ausencia
de efectos secundarios. La funcionalidad está acabada cuando permite localizar la **etapa concreta
del fallo**, o declarar incertidumbre cuando no hay evidencia suficiente.

---

## Contexto y condiciones obligatorias

Estás trabajando en un fork de los dotfiles de su proyecto upstream que ya incluye un
installer/configurador TUI con navegación, estilos y temas dinámicos, mascota, utilidades e
integración con el entorno de desarrollo. Tu cometido es **implementar esta funcionalidad realmente**,
no entregar un diseño, pseudocódigo o una maqueta sin lógica.

- **Antes de modificar nada**, examina el repositorio y localiza su arquitectura, convenciones,
  comandos, utilidades compartidas, navegación, estados, tratamiento de errores y tests. Adapta tu
  solución a lo que existe; no presupongas un lenguaje ni un framework.
- **Compatibilidad total con lo que soporte el proyecto:** todos sus sistemas operativos,
  plataformas, entornos (incluido WSL cuando corresponda), shells y emuladores de terminal. No
  acoples la implementación a una shell concreta ni confundas el entorno del host con el de WSL.
  Detecta capacidades reales y ofrece degradación elegante cuando alguna API/función no exista. Una
  plataforma sin capacidad específica no debe provocar un fallo global.
- **Experiencia integrada:** utiliza la TUI, componentes, tema, navegación y patrones de interacción
  existentes. Prioriza navegación por teclado, terminales con capacidades gráficas distintas, mensajes
  claros, progreso y cancelación cuando proceda. Mantén la UI responsiva.
- **Tecnología actual y mantenida:** cuando necesites herramientas o bibliotecas, investiga las
  alternativas vigentes, consolidadas, seguras y apropiadas para el ecosistema del proyecto. Prefiere
  APIs oficiales y mecanismos nativos bien soportados. No introduzcas herramientas
  obsoletas/deprecadas, dependencias innecesarias ni reinventes soluciones maduras sin motivo.
  Comprueba versiones y compatibilidad; no impongas un binario externo como dependencia obligatoria si
  existe una opción más fiable.
- **Seguridad por defecto:** nada de telemetría, envío de datos o funciones de IA; no ejecutar
  comandos peligrosos por inferencia; no interpolar entradas en comandos de shell; proteger rutas,
  credenciales y secretos; manejar permisos, timeouts, procesos concurrentes y errores parciales. Toda
  acción que cambie o elimine algo requiere previsualización y consentimiento explícito, salvo que la
  aplicación ya tenga un flujo inequívoco equivalente. Emplea copias de seguridad y reversibilidad
  donde sea viable; nunca afirmes que una operación es reversible si no lo es.
- **Coexistencia con otros agentes:** otros agentes implementan otras utilities simultáneamente.
  Mantén tu código encapsulado, reutiliza interfaces existentes, evita refactors globales o cambios
  innecesarios en ficheros compartidos y no alteres otras funcionalidades. Si existe funcionalidad
  similar, intégrate con ella en lugar de duplicarla.
- **Calidad:** código mantenible, tipado cuando corresponda, datos/estados reales y no simulados,
  errores recuperables, tests automatizados con dobles de prueba para acciones destructivas, cobertura
  de casos límite y validaciones del repositorio. No toques la configuración real del desarrollador
  durante tests.

**Entrega esperada:** cambios funcionales en el repositorio, integrados y probados. Al terminar,
informa brevemente de archivos modificados, decisiones importantes, comprobaciones ejecutadas,
cobertura real y limitaciones detectadas. No te quedes en un plan: implementa.

### L2 — reconnaissance, decided inline
The installer is one Go module (`installer/`, `github.com/albersg/dotfiles/installer`) with a
Bubbletea TUI in `internal/tui`; `docs/tui-installer.md` and `odd/tasks/*.md` document each screen.
The utilities section is a list of jobs (`ScreenUtilities`), each job one level in, with its own
state resolved on the startup path and its work done by a `tea.Cmd`. The conventions copied here:
every effect behind a probe struct of function fields (`shellAuditProbe`), the render path performs
no I/O, a screen's rows are the data, `renderThemeScreen` for the frame budget, one row constant per
row (`utilitiesShellAuditRow`), one panel entry per row (`utilitiesPanelEntries`), and a frame-guard
case for every screen (`screen_coverage_test.go` derives the screens from `model.go`).

Two facts fixed the design: (1) the installer already contacts `api.github.com` for its own update
check, so that host is the predefined destination the overview uses, named on screen, and no other
external host is contacted; (2) `golang.org/x/text` is already in `installer/go.mod` as an indirect
dependency, so the internationalized-hostname mapping uses `norm.NFC` from it and an in-package
punycode encoder, instead of adding `golang.org/x/net/idna` and rewriting the module graph the other
agents are working in. `go.mod` and `go.sum` are therefore untouched: the import resolves against a
module that was already required, and the direct/indirect comment in `go.mod` is left alone on
purpose while other fronts are editing this module (`github.com/charmbracelet/x/exp/golden` is in the
same state today, so `go mod tidy` already had drift before this front).

### L3 — T1, the frozen interface

`installer/internal/netdiag/{doc,report,redact,target,idna,probe}.go` and their four test files.
Checks observed: `go test ./internal/netdiag/` green; `go vet` clean; `gofmt -l` empty. The encoder is
pinned against the punycode vectors (`bücher`→`xn--bcher-kva`, `日本語.jp`→`xn--wgv71a119e.jp`,
`пример.рф`→`xn--e1afmkfd.xn--p1ai`) and every internationalized label is decoded again before it is
used, so a wrong domain cannot be resolved silently. `go.mod`/`go.sum` show no diff.

### L3 — evidence log

| Task | Commit | Checks observed |
|------|--------|-----------------|
| T1 | `0ccd0f9` | `go test ./internal/netdiag/` green, `go vet` clean, `gofmt -l` empty; punycode vectors pinned; redaction and anonymization pinned by test; `go mod tidy -diff` shows no change |
| T2 | `2b17d4d` | `env.go` + `env_test.go`; gofmt/vet/`go test ./internal/netdiag/` green. The interface summary counts a container bridge as virtual and does not count it as a way off the machine (a test pins it); the proxy check reports credentials masked and never fails on a proxy |
| T3 | `7a59c93` | `checks.go` + `checks_test.go`; gofmt/vet/`go test ./internal/netdiag/` green. A bare `host:22` target is refused a request rather than dialled (a regression would have reached a real host); an untrusted certificate is refused against a real TLS handshake; a 404 is a note and not a failure |
| T4 | `92b9160` | `run.go` + `run_test.go`; stage order, streaming, the skip of the stages a failed transport never reached, the stopped run and the run's own bound are each pinned by test; a skipped check is named exactly like the check that runs |
| T5 | (this commit) | `tui/netdiag.go` and the additive wiring; the screen's own tests pin the refusal before anything is dialled, one run at a time, Escape keeping what finished, the preview as the consent for the write, the mode of the saved file and the anonymized content. Four defects were found by those tests and fixed: the utilities list never offered the row, `handleEscape` had no case for the screen (so Escape fell through to an unrelated state), the save row was offered without the preview, and `1 interfaces` |
| T6 | (this commit) | every screen guard green: the frame case is registered in the measured matrix (`measuredScreens` 63 → 64), `TestEveryScreenFitsEveryTerminalSize` renders the new screen at all twelve terminals, and `TestNetworkDiagnosticsGolden` pins the 80x24 frame |

### L4 — the anonymized report keeps the diagnosed destination (the user's decision)

The independent verification asked whether the anonymized form should also replace the host being
diagnosed, since S10 names only this machine's identity. The user decided: the destination stays, and
the copy has to say so plainly. S10 is unchanged -- it always named this machine's addresses and
names -- and the row label, the preview paragraph and the docs now state that the diagnosed host is
kept, with a test pinning it.

### L5 — the front was rebased onto main, and the section's seventh job moved three things

`main` advanced 36 commits while this front was being verified -- the `utilities-refresh-v2` and
`start-installation-flow-next` fronts merged as #340 and #341 -- and it touched all seven shared files
this front edits, so the branch could not be merged or CI-verified as it stood. `docs/GIT-BRANCHING.md`
asks a feature branch that has fallen behind to be rebased before merging, and the seven commits were
replayed onto `origin/main` (`7906f06`), becoming `a67b61e`, `a8258ff`, `20e74d8`, `218304d`, `289454c`,
`9613ccd` and `8996e71`. Every conflicted hunk in the screen's commit had the same shape -- main added
its own utility, this front added this one -- so both sides were kept at every switch, and the commit's
message carries the resolution it needed.

Three things the merge moved, each with the guard that proves it:

- The section holds **seven** jobs now, so the job-count sentence says seven and names the network
diagnosis; `TestTheUtilitiesDocNamesEveryJobTheSectionOffers` reads that sentence against the rows and
was moved with it, because a sentence that states a count it cannot derive is exactly what that guard
exists for.
- The measured screen matrix went from main's 75 to **76**, measured by the guard rather than added up
from either side, and `TestUtilitiesGolden` was regenerated on the merged tree: three insertions and
three deletions, reviewed line by line (the theme-switch sentence in one row, the network line, and the
new panel row).
- Main's `TestUtilitiesBodyCarriesEveryJobAtTheFloor` forbids the 80x24 section from cutting any job,
and the seventh one pushed the body past its ten-row budget ("… and 3 more"). The network line is one
row now and the theme-switch line lost its second one, so the body carries all seven jobs with a row to
spare. This guard did not exist when the screen was written: it is the one thing the rebase could not
have known, and the failure was the guard doing its job.

Two fixes closed the front after the rebase: the package doc and the sentence the screen prints no
longer claim a concurrency bound the runner never had (`1599cb3`), and `CHANGELOG.md` records the screen
for a reader who never opens this file.

Also recorded here: **L1's first sentence no longer names the upstream project.** That token was the only
occurrence of the upstream brand anywhere in the tree outside the attribution files, so the CI's
`branding-audit` job failed on this front's own task record; the sentence says "su proyecto upstream" now
and the rest of the record is verbatim.

### L6 — the second time main moved, the branch merged it instead of rebasing again

The pull request was opened on `a596d1d`, green on both workflows, and the merge came back "the base
branch policy prohibits the merge": `main` requires an approving review, and GitHub does not let a
session approve the pull request it opened. While that decision was being taken, another session merged
its own front, #343 (`feat/wsl-memory-guard`), so `main` moved from `7906f06` to `ee6a4b4` and this pull
request turned `DIRTY` -- GitHub refused the merge a second time, now because the branch had fallen
behind a file both fronts add to.

L5 records what happened the first time `main` moved: a rebase. This time the branch merges `origin/main`
in instead, because that keeps the history this branch had already published reachable -- and because
#343's own branch met the same situation the same way while it waited. One conflict, in `CHANGELOG.md`
under `[Unreleased] -> Added`, where both fronts add a bullet; the resolution keeps both, #343's first
because that is how `main` reads, and the work-unit hashes L5 named stay reachable.

The merged tree is byte-identical to the tree the second rebase had produced, which is how the two routes
were checked against each other before one of them was published.

## Remaining

T1–T7, T9, T10 and T14–T16 are complete and committed; T8 was dropped by the user's decision to deliver
without a native review, and that lineage stays open and unburned. The branch is published as
`feat/network-diagnostics` and PR #342 carries it to `main`. The four `Ghostty` test failures that red
`make preflight` belong to another front -- they fail identically at `c1e4a64`, before this work, and the
CI's `Go tests` job passes over the whole module, which is where no such binary exists.
