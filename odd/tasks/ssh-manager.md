# SSH Manager

A developer-grade SSH manager inside the installer's Utilities section: the hosts
the client would really use, the keys that exist, what the agent holds, what is
misconfigured, and a connection test that runs only when asked. Nothing outside
the tool's own managed block is rewritten, no private key is ever read by this
program, and every write is backed up, validated and atomic.

## Specs

S1. Hosts. The screen shows the configured hosts with alias, hostname, user,
port, associated identity and the significant options. It respects the
**effective** configuration and the client's precedence; reading the lines of a
single file is not enough. Quote: "Respetar la configuración **efectiva** y la
precedencia del cliente SSH, sin asumir que basta con leer líneas de un único
archivo."

S2. Managed entries. The tool creates, edits, duplicates and deletes entries it
manages itself. Foreign content, comments, order, `Include`, patterns, wildcards,
`Match` and unknown options are preserved. Quote: "Conservar el contenido ajeno,
comentarios, orden, `Include`, patrones, wildcards, `Match` y opciones
desconocidas. No reescribir ni normalizar agresivamente toda la configuración".

S3. Read-only when unsafe. An entry that cannot be edited safely is shown as
read-only with the reason. Quote: "cuando no sea seguro editar una entrada
compleja, mostrarla como solo lectura y explicar el motivo."

S4. Keys. Public keys and known pairs are listed without ever showing private key
content; type, fingerprint and path are identified. Quote: "Listar claves
públicas y pares conocidos sin mostrar nunca el contenido de claves privadas;
identificar tipos, huellas/fingerprints y rutas."

S5. Generation. New keys are generated with the algorithms and practices OpenSSH
currently recommends, with compatibility when it is needed. The passphrase is
offered safely and is never logged or stored in plain text. Quote: "Generar
nuevas claves usando algoritmos y prácticas actualmente recomendadas por
OpenSSH, contemplando compatibilidad cuando sea necesaria. Ofrecer passphrase de
forma segura sin registrarla o guardarla en texto plano."

S6. Agent. The keys available in the SSH agent are managed safely: state,
loaded identities, add/remove when there is support. Quote: "Evitar prometer una
integración idéntica entre agentes/servicios con capacidades diferentes."

S7. Copying. Public keys are shown and can be copied; a private key is never
copied by default nor sent to any server. Quote: "Mostrar y permitir copiar
claves **públicas**; nunca copiar una clave privada por defecto ni enviarla a
ningún servidor."

S8. Diagnosis. Insecure permissions, missing files, duplicate/conflicting
aliases, wrong identity paths and configuration syntax are diagnosed through
reliable mechanisms. Quote: "Diagnosticar permisos inseguros, archivos
faltantes, duplicación/conflictos de alias, rutas de identidad incorrectas y
sintaxis de configuración mediante mecanismos fiables."

S9. Connection test. A connection is tested only at the user's request, with a
timeout, a comprehensible result and appropriate controls for connections that
could execute actions or trigger authentication. New host keys are never accepted
automatically, host verification is never disabled and security warnings are
never hidden. Quote: "No aceptar automáticamente nuevas host keys, no deshabilitar
la verificación del host ni ocultar advertencias de seguridad."

S10. Modern configurations. Jump hosts, per-identity/project keys, aliases and
included configurations are supported without destructive simplification.
Quote: "Apoyar configuraciones modernas habituales (jump hosts, claves por
identidad/proyecto, aliases y configuraciones incluidas) sin simplificarlas de
forma destructiva."

S11. Writes. Changes are atomic, with backup, validation and correct permissions;
secrets never appear in logs, reports, command lines or tests; an existing key is
never silently overwritten. Quote: "Cambios atomizados con backup, validación y
permisos correctos; evitar exposición de secretos en logs, informes, command
lines y tests. Nunca sobrescribir claves existentes silenciosamente."

S12. Platforms. Host and WSL configurations, processes and credentials are never
mixed automatically. Quote: "Distinguir configuraciones de host y WSL: no mezclar
sus archivos, procesos ni credenciales de manera automática."

S13. TUI. The manager is integrated in the existing TUI: list, details, add/edit
wizard, key and agent actions, connection test and clear feedback, with keyboard
navigation and cancellation, inside the shared frame at 80x24.

S14. Tests. Parsing and precedence, `Include`/`Match`, special names and paths,
conflicts, backups, restricted-permission files, mocked key generation, an
unavailable agent, timeout, host-key error and cancellation are covered; the
developer's real configuration is never touched by a test.

## Tasks

One line per task: what it delivered, where it lives, and the evidence that it works. Every package
test runs against a temporary home and an injected runner: no test opens a real key, reads the
developer's own configuration or talks to an agent.

T1 — Parsing core (`installer/internal/sshcfg/config.go`, `config_test.go`): files, `Include` inlined
where it appeared, `Host`/`Match` stanzas in file order, per-entry read-only reasons, `ssh -G --`
resolution with this package's own reading as the named fallback and its unevaluated `Match`
criteria listed. Evidence: 15 tests, plus the client's own answers checked by hand for inline
comments, a repeated `Host` block (both apply; per key the first assignment wins) and the
`IdentityFile` accumulation rule. S1-S4, S12.
T2 — Managed-block writes (`write.go`, `write_test.go`): plan/create/update/duplicate/delete confined
to the marked block, a preview with `+`/`-` lines, a never-overwritten `.bak-dotfiles-<stamp>`
backup, temp file + fsync + rename, validation by the client's own parser (a refused file is never
installed), `0600` for new files and for a group/other-writable one, and a refusal when the file
moved since the preview. Evidence: 13 tests, including byte-for-byte assertions on everything
outside the changed lines. S5-S8.
T3 — Keys, known hosts, generation (`keys.go`, `keys_test.go`): the inventory reads the public half
only (proven by a test whose private key is mode `0000` and still described), permissions warnings,
`Encrypted` said only when proven, `known_hosts` parsed with hashed entries left hashed and
`known_hosts.old` never read, `ssh-keygen -F` for lookups, generation with the passphrase on the
pipe. S9-S10.
T4 — Agent (`agent.go`, `agent_test.go`): state from `ssh-add -l` plus the socket, exit 1 read as
running-and-empty, per-kind notes (gpg-agent, 1Password, KeePassXC), add/remove/clear with the
passphrase proven never to reach an argument or the environment. S11.
T5 — Diagnostics (`diagnose.go`, `diagnose_test.go`): permissions, syntax via the client, duplicate
aliases, missing identities, host-key checking disabled, missing `known_hosts`, the WSL note and the
Match caveat. S13.
T6 — Connection test (`connect.go`, `connect_test.go`): the exact option list asserted as a whole,
the outcome table, timeout and cancellation, bounded output, and nothing run without a client. S14.
T7 — TUI (`sshmanager.go`, `sshmanager_test.go`, wiring in `model.go`, `update.go`, `view.go`,
`panels.go`, guards in `teatest_test.go` and `screen_coverage_test.go`): the utilities row, seven
screens, the plan confirmation, the generation form, the agent actions and the test screen, measured
by the same 80x24 floor guard as every other installer screen. S1-S14.
T8 — Documentation: `README.md`, `docs/tui-installer.md` (the utilities bullet and *The SSH
configuration*), the regenerated `TestUtilitiesGolden`, and this file.

## Log

L1 (user request, verbatim):

> # Agente 07 — SSH Manager
>
> **Misión:** implementar un administrador SSH de calidad para desarrolladores, centrado en hosts, claves, agente, configuración y diagnóstico de conectividad, sin comprometer la seguridad.
>
> ## Alcance funcional
>
> - Vista de hosts SSH configurados, alias, hostname, usuario, puerto, identidad asociada y opciones significativas. Respetar la configuración **efectiva** y la precedencia del cliente SSH, sin asumir que basta con leer líneas de un único archivo.
> - Crear, editar, duplicar y eliminar entradas **administradas por la herramienta**. Conservar el contenido ajeno, comentarios, orden, `Include`, patrones, wildcards, `Match` y opciones desconocidas. No reescribir ni normalizar agresivamente toda la configuración; cuando no sea seguro editar una entrada compleja, mostrarla como solo lectura y explicar el motivo.
> - Listar claves públicas y pares conocidos sin mostrar nunca el contenido de claves privadas; identificar tipos, huellas/fingerprints y rutas. Generar nuevas claves usando algoritmos y prácticas actualmente recomendadas por OpenSSH, contemplando compatibilidad cuando sea necesaria. Ofrecer passphrase de forma segura sin registrarla o guardarla en texto plano.
> - Gestionar de forma segura claves disponibles en el agente SSH: ver estado, identidades cargadas, solicitar añadir/eliminar cuando haya soporte. Evitar prometer una integración idéntica entre agentes/servicios con capacidades diferentes.
> - Mostrar y permitir copiar claves **públicas**; nunca copiar una clave privada por defecto ni enviarla a ningún servidor. Las claves privadas existentes no deben leerse, procesarse ni almacenarse fuera de su mecanismo legítimo.
> - Diagnosticar permisos inseguros, archivos faltantes, duplicación/conflictos de alias, rutas de identidad incorrectas y sintaxis de configuración mediante mecanismos fiables.
> - Probar una conexión **solo a petición del usuario**, con timeout, resultado comprensible y controles apropiados para conexiones que podrían ejecutar acciones o disparar autenticación. No aceptar automáticamente nuevas host keys, no deshabilitar la verificación del host ni ocultar advertencias de seguridad.
> - Apoyar configuraciones modernas habituales (jump hosts, claves por identidad/proyecto, aliases y configuraciones incluidas) sin simplificarlas de forma destructiva.
> - Integración TUI: listado, detalles, asistente de alta/edición, acciones sobre claves y agente, test de conexión y feedback claro.
>
> ## Seguridad, compatibilidad y tests
>
> Reutilizar preferentemente el cliente y herramientas OpenSSH soportadas, mediante procesos con argumentos seguros o APIs mantenidas. No introducir stacks heredados o librerías sin mantenimiento. Distinguir configuraciones de host y WSL: no mezclar sus archivos, procesos ni credenciales de manera automática. Cambios atomizados con backup, validación y permisos correctos; evitar exposición de secretos en logs, informes, command lines y tests. Nunca sobrescribir claves existentes silenciosamente.
>
> Probar parsing/precedencias, `Include`/`Match`, nombres y rutas especiales, conflictos, backups, archivos con permisos restringidos, key generation mockeada, agent no disponible, timeout, error de host key y cancelación. Terminado cuando se puedan gestionar hosts propios y claves públicas de forma segura sin corromper configuraciones previas.

L2 (what happened between the first dispatch and delivery):

> The three worker tasks that were to write T1-T6 were lost when the parent session was restarted;
> `config.go`, `keys.go`, `diagnose.go` and `connect.go` landed, while `write.go`, `agent.go` and
> every test file did not. The parent finished the package itself: `write.go` and `agent.go`, then
> `config_test.go`, `keys_test.go`, `connect_test.go`, `write_test.go`, `agent_test.go` and
> `helpers_test.go` against the landed code.

L3 (defects the checks found, and the decision each one forced):

> - `write.go`: appending the managed block to a file that already ended with a newline produced two
>   blank lines. Fixed with one insertion helper; the unit test asserts single-blank-line spacing.
> - `write.go`: a stanza's line range originally ran to the next `Host`/`Match` line, so deleting an
>   entry removed a comment the user had written after it. The range now ends at the entry's last
>   directive.
> - `write.go`: the client *refusing* the new content used to be recorded and then written anyway.
>   It now blocks the write: a file the client will not parse is worse than a refused change.
> - `write.go`: a `0644` configuration was silently chmodded to `0600` on any write. Only
>   group/other *write* bits are now tightened.
> - `agent.go`/`connect.go`/`write.go`: a tool's own message was embedded in user-facing sentences
>   unbounded, so a host answering with one long line set the screen's width. All three now quote a
>   bounded first line.
> - `sshmanager.go`: the run row was offered with no client to run, the generation form never offered
>   its replace switch, the test screen stated its "nothing is sent" rule only while the target field
>   was empty, and an entry's public key could not be matched because the comparison rebuilt an
>   absolute path under `$HOME` (which the installer's path guard correctly refuses to classify).
>   All four are fixed; the comparison now reduces both sides to their form relative to the home the
>   paths were resolved for.
> - `TestUtilitiesGolden`: the new row pushed another row's description line behind the section's
>   "… and n more" marker at 80x24. The SSH paragraph was cut to a single line, which is the shape
>   the section's budget was built for; the marker is the section's own visible trade-off.
> - Inline comments keep an entry read-only (`the value carries a comment`): rewriting such a line
>   would mean deciding what happens to the comment, and refusing is the honest answer.

L4 (repository gates on this machine, and what they do and do not prove):

> - Two pinned counts moved by design, and both say so where they are pinned:
>   `measuredScreens` 63 -> 73 (screen_coverage_test.go) and the companion guard's
>   rendered-screen count 912 -> 1072 (companion_test.go). Both are deliberate: 10 new case names
>   x 2 states x 2 sprite modes x 4 terminals = 160, which is the exact delta the guard reported.
> - The four Ghostty download tests fail on this machine and are **not** caused by this change:
>   `internal/tui/installer.go:5386` gates the download on `system.CommandExists("ghostty")`, which
>   is `exec.LookPath` (`internal/system/detect.go:295`), the test's stub PATH is
>   `<stub>:/usr/bin:/bin`, and this machine has `/usr/bin/ghostty` (`command -v ghostty`). The step
>   therefore takes its "Ghostty already installed" branch and the assertions about a download never
>   run. On a machine (or CI runner) without that binary they exercise the path they were written
>   for. Nothing in this change touches the terminal step, `system.CommandExists` or PATH handling.
> - Consequence: `make check` and `make preflight` report the `internal/tui` package as failed on
>   this machine for that reason. Every check this change is responsible for passes:
>   `gofmt -l`, `go vet`, `go build ./...`, `go test ./internal/sshcfg/` (66 tests) and the SSH
>   manager's own TUI tests, the shared frame guards and the utilities golden.

L5 (the native review could not run on this candidate, and what was decided):

> - The whole change was offered as one candidate and refused as over budget:
> `lens_context_budget_exceeded` - "the candidate's complete reviewer evidence exceeds the native
> context budget" (11,636 inserted lines). That attempt created no review authority, so nothing had
> to be abandoned or repaired.
> - The ecosystem's review budget is 400 changed lines (the same figure the chained-PR guard uses),
> and a candidate is always base -> HEAD, so the branch was rebuilt as a chained sequence of twelve
> slices over the same final tree. The pre-split chain is kept locally as `backup/ssh-manager-3unit`;
> every file outside this log entry is byte-identical to it, and every slice below is a commit the
> review can name as its own candidate:
> - `c84cc84` feat(installer): the ssh manager gets its contract and the platform facts it needs (535 lines)
> - `c904f71` feat(installer): the client's own configuration is read with its provenance (1303 lines)
> - `a9cb4ac` feat(installer): writes stay inside one marked block of the ssh configuration (1365 lines)
> - `514de52` feat(installer): a key pair is described from its public half (1314 lines)
> - `278479b` feat(installer): the agent is described by what it really is (865 lines)
> - `482a471` feat(installer): the connection test asks the client and classifies its answer (516 lines)
> - `ff0ef5e` feat(installer): the diagnosis names what is wrong and what to do (1316 lines)
> - `dd3bcbd` feat(installer): the ssh manager's screens and the commands behind them (2866 lines)
> - `09b4608` test(installer): the ssh manager's screens are pinned by their own tests (917 lines)
> - `ac0d7b4` feat(installer): the utilities section gets the ssh manager's row (239 lines)
> - `47a471b` test(installer): the ssh screens are measured by the shared frame guards (87 lines)
> - (this commit) docs(installer): the ssh manager is documented (313 lines)
> - Reviewing the first slice (535 lines, tier high, four lenses required) reached the capture step
> and was refused typed: `reviewer-config-invalid`, "no model is configured for review-risk". The
> reviewer completion reads `review-risk`, `review-resilience`, `review-readability` and
> `review-reliability` (and later `review-refuter` and `review-validator`) from the agent model
> routing config, and it never falls back to another provider or an ambient default, so no lens run
> could start. Nothing was mutated, and that transaction stays intact: the capture can be retried
> without a new start once those roles have models assigned (from `/gentle:models`; global agents
> write `~/.pi/agent/subagents.json`).
> - Decision (user, this session, verbatim): "Seguir sin revisión nativa de los cortes". Verification
> therefore rests on the feature's own checks and on an independent read-only verifier run over the
> range, and delivery follows ordinary repository policy. The review switch stays on.
