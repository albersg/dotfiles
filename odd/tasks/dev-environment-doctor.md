# The development-environment doctor: one utility that diagnoses this machine and fixes only what is deterministic

One front, one utility. It is the next entry in the Utilities section (the section's own contract:
`utilitiesPanelEntries` is the only list, the frame is measured at twelve sizes, Termux and 80x24 are
the floor, a report is read-only until a confirmation says otherwise). What it adds that the earlier
utilities do not have is **classification**: every answer states `ok`, a warning, an error, `n/a` or
`unknown` with evidence, impact and a suggestion — and a check that did not run is never `ok`.

The inventory this started from, re-checked in the tree:

- **No doctor existed.** `grep -rn "doctor" internal/` finds the word only in the Utilities panel
  comment and in `docs/tui-installer.md`; no check, no classification, no report.
- **The read half already exists in pieces.** `system.Detect()` (OS/WSL/Termux/arch),
  `system.DetectExistingConfigs()` (pre-existing user config), `readLastInstallRecord()` /
  `VerifyInstall()` (`state.go`: presence of the paths the last run touched),
  `system.GetShell()`, `hostResourceProbe` (bounded command execution with `WaitDelay`).
- **Utilities section screens are the template.** `terminal_probe.go` (a read-only report whose
  answers carry source and reason), `shellaudit.go` (bounded command runs, a timeout is a result,
  armed only when the screen opens), `util_screen_test.go` + `screen_coverage_test.go` (panel
  derivation and frame guards that fail by name).
- **`--verify` is not the doctor.** It compares the record with the machine and prints lines with an
  exit code that scripts read. The doctor classifies, carries evidence, and runs many checks; the
  dotfiles check reads the same record and must not re-implement its reader.
- **No managed-tool registry existed.** What the installer really manages is in `installUnits`
  (step catalogue) and the config writers; the doctor's tool table is derived from those, not typed
  beside them.

## Specs

S1 — running
> Ejecutar diagnósticos completos o por categoría, con progreso, reintentos, cancelación, filtros, resultados persistibles cuando tenga sentido y exportación opcional de informe redactado (sin secretos).

S2 — PATH
> Validar `PATH` y resolución efectiva de ejecutables: entradas duplicadas, rutas inexistentes, colisiones entre versiones, ejecutables ocultos por precedencia y discrepancias entre configuración declarada y entorno efectivo. Considerar semántica de rutas y permisos de cada plataforma.

S3 — shells
> Revisar configuración de shells instaladas: archivos esperados, sintaxis cuando haya verificador fiable, dependencias ausentes, referencias rotas, aliases/variables conflictivas cuando puedan detectarse sin interpretar arbitrariamente scripts. Distinguir shell actual, de inicio y configuraciones de otras shells.

S4 — managed tools
> Verificar la instalación y versión de herramientas **gestionadas realmente por el installer** y sus dependencias. Distinguir componente opcional no instalado de componente requerido roto.

S5 — dotfiles integrity
> Comprobar integridad de dotfiles gestionados: enlaces simbólicos, destinos, archivos ausentes, conflictos con configuraciones preexistentes y permisos problemáticos. No asumir propiedad sobre archivos ajenos.

S6 — git and ssh
> Revisar los elementos básicos de Git, identidad/configuración, credenciales sin leerlas, agent SSH y permisos de claves sin duplicar la interfaz completa del SSH Manager.

S7 — environment and platform
> Identificar errores habituales de entorno: directorios que no existen, variables incoherentes, capacidad insuficiente para ciertas operaciones, dependencias del installer incumplidas o incompatibilidades verificables. Incluir comprobaciones especializadas únicamente cuando el entorno realmente lo permita.

S8 — what a result is
> Cada resultado debe indicar estado (`ok`, advertencia, error, no aplicable, desconocido), evidencia verificable, impacto y solución sugerida. Un chequeo no ejecutado nunca debe figurar como correcto.

S9 — Fix
> Ofrecer **Fix** solo para acciones deterministas y relativamente seguras: mostrar diff/efecto, riesgo, permisos requeridos y copia de seguridad; confirmar; volver a comprobar el estado previo; aplicar la corrección mínima; verificar el resultado. Para problemas ambiguos, limitarse a instrucciones manuales.

S10 — not falsely categorical
> Evitar diagnósticos falsamente categóricos, como considerar defectuosa una identidad Git local diferente de la global por el mero hecho de ser distinta.

S11 — robustness and secrets
> La exploración ha de ser esencialmente de solo lectura; gestionar binarios ausentes, configuraciones personalizadas, permisos denegados, symlinks rotos, rutas Unicode/con espacios, resultados parciales, comandos que cuelgan y cambios durante el escaneo. No exponer tokens, contenido sensible ni variables secretas en pantalla o informes.

S12 — tests and acceptance
> Tests para detección, clasificación, `PATH`, shell/configs, permisos, enlaces, fallos de comando y ciclo completo de corrección segura. La utilidad está terminada cuando ofrece diagnósticos útiles y verificables desde la TUI, sin bloquearla ni modificar configuraciones sin autorización.

S13 — integration (from the task's own conditions)
> **Compatibilidad total con lo que soporte el proyecto** […] **Experiencia integrada:** utiliza la TUI, componentes, tema, navegación y patrones de interacción existentes […] **Seguridad por defecto:** […] no interpolar entradas en comandos de shell […] Toda acción que cambie o elimine algo requiere previsualización y consentimiento explícito […] **Coexistencia con otros agentes:** Mantén tu código encapsulado, reutiliza interfaces existentes, evita refactors globales o cambios innecesarios en ficheros compartidos.

## Decisions, and what they cost

| Decision | Why | Cost, accepted |
|---|---|---|
| One package (`internal/tui`) and five files: `doctor.go` (domain, host, runner, report), `doctor_path.go`, `doctor_shell.go`, `doctor_tools.go`, `doctor_dotfiles.go`, `doctor_git.go` + `doctor_env.go`, `doctor_fix.go`, `doctor_screen.go` | The other utilities live here (`shellaudit.go`, `terminal_probe.go`, `wslconfig.go`); a separate package would need exported seams for a screen nobody else calls | Six logic files instead of one; each has its own test file |
| Every host read goes through `doctorHost` (env, fs, path lookup, bounded command run, capacity, clock) | S11/S12: a test must exercise unicode paths, denied permissions, dangling symlinks, a hanging command and denied reads **without touching the developer's real configuration** | One interface (about twenty methods) plus a real implementation and one fake in the tests |
| Checks are a **registry** (`doctorChecks()`), each declaring ID, category, title and required/optional, and a run function returning findings | S1 (per category, retry), S4 (required vs optional), S8 (a check that did not run keeps `unknown`) | One more list; the guard `TestDoctorCheckCatalogueIsWellFormed` keeps IDs unique and every category populated |
| Statuses are typed: `ok`, `warning`, `error`, `notApplicable`, `unknown`; `unknown` is the zero value | S8: a check that did not run must not read as correct, so the zero value is the honest one | Every switch over status must handle five cases; `String()` is the only renderer |
| Every finding carries **evidence, impact and suggestion**, and the screen refuses to draw a finding without evidence | S8: "evidencia verificable, impacto y solución sugerida" — a finding with no evidence is an opinion | Verbose finding rows; the detail of the cursor row is what the body shows |
| Commands are run through one bounded helper (`doctorRun`) with context, timeout and `WaitDelay`, mirroring `shellaudit.go` | S11: a command that hangs must not hang the interface; a timeout is a result, never a duration | A timed-out check reports `unknown` with the reason, not a verdict |
| Only **two** fixes are offered: create a missing directory the machine's own configuration names, and set the safe mode on `~/.ssh` (0700) or a private key (0600) | S9: "acciones deterministas y relativamente seguras" — both are single, minimal, reversible-by-record changes with a verifiable result | Every other finding is manual instructions, named as such on screen |
| A fix shows its effect, risk, required permissions and backup, re-reads the state before applying, applies the minimal change and re-runs its own check to verify | S9 verbatim, and the confirmation screen is where "confirmar" lives | A fix that fails leaves a record of the previous state in the state directory |
| The applied fix is recorded in `<state>/dotfiles/doctor-fixes.json` with the previous state and the exact revert command | S9's backup and "nunca afirmes que una operación es reversible si no lo es": a recorded mode is what makes `chmod` reversible | The record is a new file in the state directory; nothing reads it to decide anything |
| The report is redacted by name (`*TOKEN*`, `*SECRET*`, `*KEY*`, `*PASSWORD*`, `*CREDENTIAL*`, `*AUTH*`, `*API*`) and by value shape, and no check reads a credential store | S11: no secret on screen or in the report | A variable with a secret in its name is reported as present but never printed |
| The report is written to `<state>/dotfiles/doctor-report.txt` on every finished run, and the **same** renderer writes the exported file when the row is pressed | S1's "persistible" and "exportación opcional": one renderer, so the file on screen and the file on disk cannot diverge | Two writes for two purposes; the export names its path in the notice |
| The screen is one level in from Utilities, like every other utility, and the panel entry comes from `utilitiesPanelEntries` | The section's own contract (`pet-panel-utils.md`), and `TestUtilitiesPanelFactsAreDerivedFromTheModelState` fails otherwise | The panel gains one entry; the test's five distinct panel states become six |
| The run is a `tea.Cmd` sequence: one message per finished check, so progress is real and `Esc` cancels through the context | S1 (progress, cancellation) and the harness rule that the update loop never blocks | The model holds a run id so a cancelled run's late messages are ignored |
| WSL, Termux and macOS-specific checks are gated on `system.Detect()` (or the host's own `GOOS`) and report `notApplicable` with the reason elsewhere | S7: "comprobaciones especializadas únicamente cuando el entorno realmente lo permita" and the ban on confusing host with WSL | Three platform paths; each is a finding the fake host can produce |
| The tool table is derived from `installUnits` where the unit names a real binary, plus the installer's own dependencies | S4: "gestionadas realmente por el installer" — a typed list beside the catalogue is the second list this repository already paid for | Some units (backup, cleanup, theme) name no binary and are left out deliberately |

## Tasks

| # | Task | Specs | Route | Commit |
|---|---|---|---|---|
| T1 | Domain, host seam, registry, runner (progress/cancel/retry), statuses, report + redaction + persistence | S1, S8, S11 | inline | `feat(installer): a doctor that classifies, and every answer carries its evidence` |
| T2 | PATH and executable resolution checks | S2 | inline | `feat(installer): the doctor reads PATH the way the shell resolves it` |
| T3 | Shell configuration checks (files, syntax, references, conflicts, three shells) | S3 | inline | `feat(installer): the doctor reads the shells' configuration without running it` |
| T4 | Managed tools and installer dependencies | S4, S7 | inline | `feat(installer): the doctor names the tools the installer manages` |
| T5 | Managed dotfiles integrity (record, links, permissions, foreign files) | S5 | inline | `feat(installer): the doctor checks the dotfiles this installer owns` |
| T6 | Git and SSH basics (identity, helper, agent, key modes) | S6, S10 | inline | `feat(installer): the doctor reads git and ssh without reading a secret` |
| T7 | Environment and platform checks (HOME, XDG, temp, capacity, locale, TERM, WSL) | S7 | inline | `feat(installer): the doctor names the environment faults that break a run` |
| T8 | The two deterministic fixes, with preview, backup, re-check and verification | S9, S12 | inline | `feat(installer): the doctor fixes only what it can prove and verify` |
| T9 | The screen, the panel entry, the frame guards and the docs | S1, S13 | inline | `feat(installer): the doctor's screen, and the section that opens it` |
| T10 | Full suite, `go vet`, frame guards at twelve sizes, real-machine spot check | S12, S13 | inline | (no commit: verification only) |

## Log

L1 (the user's request, verbatim): the full brief of "Agente 01 — Dev Environment Doctor": build a
development-environment diagnosis centre that detects real problems, explains their cause and allows
safe repairs when they are unambiguous; functional scope as quoted in S1–S12 above; robustness and
acceptance as quoted; the mandatory conditions (explore the repository first, total compatibility
with the supported platforms and shells including WSL, integrated TUI experience, current and
maintained technology, security by default, coexistence with other agents, quality with real data
and tests that never touch the developer's real configuration); and the delivery expectation:
functional, integrated and tested changes in the repository, followed by a short report of modified
files, important decisions, checks run, real coverage and limitations detected.

L2 (evidence, this session): `grep -rn "doctor" installer/internal` finds no implementation; the
Utilities section already holds four jobs and its contract (one list, twelve sizes, Termux floor) is
in `odd/tasks/pet-panel-utils.md`; `state.go` already reads the last-install record and `--verify`
already compares it with the machine, so the doctor's dotfiles check reads that record instead of a
second reader; `shellaudit.go` already establishes the bounded-command pattern
(`shellAuditTimeout` + `shellAuditWaitDelay`) that every doctor command copies; the repo has no build
tags and CI builds on ubuntu + macos, so platform differences are handled at run time and disk
capacity is read through `df` (a missing `df` is `unknown`, never a verdict).

L3 (implementation, this session): T1–T8 landed as one batch of files in `installer/internal/tui`
(`doctor.go`, `doctor_path.go`, `doctor_shell.go`, `doctor_tools.go`, `doctor_dotfiles.go`,
`doctor_git.go`, `doctor_env.go`, `doctor_fix.go`) plus the one seam outside it,
`internal/system/exec.go`'s `ConfigPathsFor(home)` (the existing `ConfigPaths()` now delegates to it,
so the backup step, the overwrite count, the record and the doctor cannot disagree about which files
this installer writes). One design decision was made while writing and is worth stating: rather than
teaching each check to be testable, every effect goes through the `doctorHost` seam
(`Getenv`/`Environ`/`LookPath`/`Lstat`/`Stat`/`ReadDir`/`ReadFile`/`Readlink`/`UserHomeDir`/`GOOS`/
`Run`/`MkdirAll`/`Chmod`/`WriteFile`), so the tests run the whole catalogue against a machine of
their own — a dangling link, a unicode path, a directory nobody may read, a command that never
answers — instead of against the developer's machine, and `doctorScan` holds only caches and the
platform detection.

L4 (defects the tests found, and the fixes): the guards were written after the code and found five
real faults, each fixed in the source rather than relaxed in the test. (1) `worseDoctorStatus`
aggregated by constant value, so `ok` outranked `unknown` and a scan nobody finished could be
summarised as clean: it now aggregates by an explicit `rank()` where `unknown` sits above `ok` and
`counts.worst()` uses the same order. (2) `path.shadowed` iterated the raw tool catalogue instead of
`doctorToolsForPlatform`, so the two `brew` entries (one gated to macOS, one to the other hosts)
produced two identical rows on a machine where only one applies. (3) `shell.syntax` reported a
checker that timed out or could not be started as a *broken file*: it now separates "the shell ran and
refused the file" (an error) from "the checker could not be asked" (unknown, with the manual command),
through the exit status rather than through the message. (4) the locale check read `LC_CTYPE` only to
decide whether anything was set: a category variable that overrides `LANG` with a locale that has no
UTF-8 in it is now its own warning, because that is what misprints accented paths. (5) the redactor
knew a URL's credentials, a secret-named assignment and a PEM body but no token shape: it now also
redacts an authorization header and the shapes of the tokens the services this installer touches issue
(GitHub, an OpenAI-style key, Slack, an AWS key id). A sixth fault was found by the screen tests: the
**Diagnose the development environment** row was drawn by the section but had no case in
`handleUtilitiesKeys`, so pressing it did nothing — the row, the frame and the panel were all there
and the key was not.

L5 (evidence, this session): `go build ./...` and `go vet ./internal/tui/` are clean. The doctor's own
suite is green: `TestDoctorCheckCatalogueIsWellFormed` (27 checks over 6 categories),
`TestEveryFindingCarriesEvidenceImpactAndAdvice`, `TestUnknownIsItsOwnStateNotAnInventedOk`,
`TestEveryDeterministicFixIsPreviewedAppliedAndVerified`, `TestFixRefusesWhatItCannotProve`,
`TestADryRunChangesNothing`, `TestTheReportNeverCarriesASecret`,
`TestTheRendererRedactsEveryFieldItWrites`, `TestRedactionCoversTheShapesAValueCanTake`,
`TestCountsAndSummaryAgreeWithTheFindings`, `TestAHealthyMachineRaisesNoAlarm` (a machine with nothing
wrong with it produces no warning and no error at all), the per-category guards (`TestPath*`,
`TestShell*`, `TestTools*`, `TestDotfiles*`, `TestGit*`, `TestEnv*`) and the screen guards
(`TestDoctorScreen*`, `TestDoctorFixScreen*`, `TestDoctorRenderingTouchesNothing`,
`TestDoctorScreenStatesAreWordsNotOnlyColours`, `TestUtilitiesPanelNamesTheDoctorFromTheModelState`,
`TestTheUtilitiesSectionOpensTheDoctor`). Two frames are pinned as goldens at 80x24
(`TestDoctorGolden`, `TestDoctorFixGolden`) and both screens are in the frame-fit guards at twelve
terminal sizes.

L6 (real-machine spot check, this session): the whole catalogue was run once against the machine this
session runs on — through the real host (`newDoctorScan(nil, system.Detect(), now)`), with no fake —
and produced a report in **1.27 s** over 27 checks: 49 `ok`, 25 warnings, 6 not applicable, no
error, no panic and no hang. The findings were all real: eleven `PATH` entries that do not resolve
(`~/.pi/agent/bin`, `~/.nix-profile/bin`, two `/mnt/c/...` entries), two duplicates, and the
shadowed `git`/`curl`/`file` copies where the Linuxbrew build (git 2.56.0, curl 8.23.0, file 5.48)
runs while `/usr/bin` holds 2.47.3, 8.14.1 and 5.46 — the difference the check exists to name. The
platform was detected as Debian/Ubuntu x86_64 on WSL2, which is why the WSL-specific checks ran and
reported rather than reading as not applicable.

L7 (seventh round of evidence, this session): the fourth full-suite run replaced the third, and the
third had found two different things. First, `TestEveryHomePathTheInstallerWritesIsClassified` — the
repository's guard over every `$HOME` destination the installer joins — reported seven joins in the
doctor: the XDG fallback helper joined a variable, the environment check joined it again, the git and
ssh check named `~/.ssh`, and the shell literal resolver expanded three spellings of the home prefix
out of a file the user wrote. The change is not a rename of the variable, which would dodge the scan:
the four fallbacks are now string literals at their own joins (the guard classifies what it can read);
`.local/state` and `.cache` are classified in `installerOwnedHomePaths` as XDG roots that hold state
and caches rather than configuration; `~/.ssh` is classified in a new and deliberately separate table,
`installerReadOnlyHomePaths`, because the doctor reads the directory and can set a mode and never
writes a file into it, which is a different statement from the owned table's; and the one resolution
helper that expands a path the user's own shell configuration names is classified in
`installerReadOnlyHomeExpansions`, by function, because there is no literal to classify. Both new
tables are checked for staleness — an entry no join uses is reported — and the guard's own teeth test
gained four cases pinning that a read-only path is not reported, that a path under it still is, that
an unresolvable join outside the classified function still is, and that an unused classification is
reported. Second, four Ghostty tests (`TestStepInstallTerminalGhosttyReportsAFailedDownload`,
`TestStepInstallTerminalGhosttyRunsTheDownloadItFetches`,
`TestInteractiveGhosttyScriptFailsWhenTheDownloadFails`,
`TestInteractiveGhosttyScriptDownloadsThenRuns`) failed on download behaviour this feature does not
touch. They were run in a scratch worktree of the same clone checked out at the base commit
`c1e4a64` and failed there identically, so they are pre-existing and not caused by this change; the
scratch worktree was removed afterwards. The suite therefore cannot be all-green on this tree for
reasons outside this feature, and that is reported rather than hidden.

L8 (commits and the closing gate, this session): the work is committed in the dependency order the
tree needs, one work unit per commit, on `worktree/green-cloud-c18b`:

| Commit | Unit |
|---|---|
| `adc502c` | `refactor(installer): one seam resolves the config paths for any home` — `ConfigPathsFor(home)`, with `ConfigPaths()` delegating and its 22 keys unchanged |
| `eb8379c` | `feat(installer): a doctor that classifies, and every answer carries its evidence` — the domain, the registry, the six check files, the two fixes and their guards |
| `af83e62` | `feat(installer): the doctor's screen, and the section that opens it` — the screen, the panel entry, the wiring, the frame guards, both new goldens and the Utilities golden that moves with the section |
| `e09c5c4` | `test(installer): the home-path guard classifies the paths the doctor reads` — the four literal XDG joins, the two new classification tables and the cases that pin them |
| `55b8d53` | `docs(installer): the doctor is documented where the utilities are` — `docs/tui-installer.md`, `README.md`, `CHANGELOG.md` |

The gate that closed T10 is the fourth full-suite run over exactly this tree: `go test ./... -count=1`
exits 1, and the only failures are the four Ghostty tests proven pre-existing in L7 —
`cmd/dotfiles`, `internal/release`, `internal/system`, `internal/tui/trainer` pass, and `internal/tui`
fails on those four alone. `go vet ./...` is clean, `gofmt -l` reports nothing, and the doctor's own
suite, the frame guards at twelve sizes, the two doctor goldens, the Utilities golden and the
home-path guard are green. The commits add no content the run above did not cover: the tree the suite
measured is the tree they record.
