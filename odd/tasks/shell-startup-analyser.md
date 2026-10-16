# The terminal performance analyzer: the shell startup, measured honestly

## Goal

The repository already has a terminal performance analyzer: the **Measure the shell's startup** row in
the Utilities section, `installer/internal/tui/shellaudit.go`. This front evolves it -- it does not
create a second feature.

## Specs

S1. Measure the real startup time of the shell with the **mode named explicitly**: interactive,
non-interactive, login, and with and without the user's own configuration, each one only where it can
be reproduced correctly. Running a one-off command is never presented as if it measured the opening of
an interactive terminal.

S2. Support every shell the project supports through **capability detection**. Each shell has its own
startup semantics and its own profiling mechanism; the shell's own mechanism is used rather than one
common method forced onto all of them. The shell that is measured **need not be the shell running the
installer**.

S3. **Monotonic timing, several samples, optional warm-up and reasonable bounds.** Present median,
mean, minimum and maximum, variability, percentiles when there are enough samples, valid and failed
runs, and the context of the measurement.

S4. Where the shell allows it, **profile the stages, scripts, functions or plugins responsible** for
the latency, showing cost and evidence. Where a shell offers no reliable introspection, report only the
global times and explain the limitation -- **never invent a per-plugin percentage**.

S5. **Compare with a clean baseline** where it is feasible, without modifying the user's permanent
configuration.

S6. Keep an **optional local history**, with before/after comparisons, trend, regressions, improvements
and actionable recommendations. A cause is named only where the data supports it.

S7. UI: start a benchmark **by shell and mode**, see progress, cancel, review the breakdown and open
the recommendations, integrated with the themes and the interaction behaviour that already exist.

S8. Current, contextual and **non-destructive** optimization advice. Any automatic change to a startup
file must be reviewable, confirmed and backed up, and permanent instrumentation is never injected. This
front applies none: the analyzer only reads.

S9. No hang from an interactive hook, a stuck command, a banner, a prompt or a program that needs a
TTY. Bounded runs, and a PTY only where it is indispensable. Process duration and the cost of the
graphical terminal are distinguished rather than conflated.

S10. No secret from a profile, a variable or captured output is displayed or persisted.

S11. Tests cover statistics, anomalous timings, startup modes, an absent shell, failures, timeouts,
cancellation, history and backward compatibility.

S12. The outcome is a **demonstrable improvement of the analyzer that exists**, with reproducible
benchmarks and honest metrics -- not a parallel module.

## What exists before this front (read-only audit, on this machine)

| What | Where | State before |
|------|-------|--------------|
| Measurement engine | `installer/internal/tui/shellaudit.go` | one mode: `$SHELL -i -c exit`, five runs, median/range, per-run timeout |
| Introspection | same file, `profileShellAudit` | zsh + `zprof` only; every other shell gets a reason, no number |
| Baseline | -- | none |
| Statistics | `shellAuditSummary` | median, fastest, slowest; no mean, no spread, no percentile |
| Warm-up | -- | none; the first (cold) start is a sample like any other |
| History | -- | none persisted |
| Progress / cancel | `update.go`, `handleShellAuditKeys` | one flag `Measuring`; no progress, no cancel |
| UI reach | `ScreenShellAudit` | one measure row, the result rows, back |

Measured capability of the shells on this machine (read-only probes):

| Shell | version reported here | clean-start mechanism | introspection |
|-------|-----------------------|------------------------|---------------|
| zsh 5.9.2 | `zsh --version` | `ZDOTDIR=<empty>` + `-d` | `zmodload zsh/zprof` (table) |
| bash 5.3.20 | `bash --version` | `--noprofile --norc` | `PS4` + `EPOCHREALTIME` (`set -x`), bash >= 5 |
| fish 4.9.3 | `fish --version` | `--no-config` | `fish --profile=<file>` (table) |
| nushell 0.116.1 | `nu --version` | `--no-config-file` | none: total only |
| sh / dash | `--version` (exit 2 on dash) | `ENV=` empty | none: total only |

## Tasks

| ID | Spec | Route | Work |
|----|------|-------|------|
| T1 | S3 | inline | Statistics layer: mean, sample standard deviation, percentiles with a sample floor, Tukey outliers, a summary that refuses a percentile it cannot support |
| T2 | S1, S2 | inline | Capability matrix: shell kinds, the four startup modes, each shell's own clean-start mechanism and its own profiler |
| T3 | S1, S3, S7, S9 | inline | Plan and engine: warm-up, progress events, cancellation, bounded runs, per-mode results |
| T4 | S4, S10 | inline | Profilers: zprof kept, bash `xtrace` parsed as a stream, fish `--profile` parsed and redacted; nothing raw is kept |
| T5 | S5 | inline | Clean baseline measured in the same pass, without touching the user's configuration, and the delta |
| T6 | S6 | inline | History under the XDG state dir, trend, regression and improvement, and advice that cites its evidence |
| T7 | S7, S8 | inline | UI: a plan the user cycles, progress with the run it is on, cancel, the history and advice view, frame guards |
| T8 | S12 | inline | Documentation and the utility's own prose brought in line with the new measurement |
| T9 | S11, S12 | verifier | `make preflight` once, then an independent read-only verification of the change |

## Log

L1 (the request, verbatim):

> **Misión:** localizar el analizador/benchmark de terminal **que ya existe** en el repositorio y
> evolucionarlo, sin crear una segunda funcionalidad ni romper la actual.
>
> ## Alcance funcional
>
> - Auditar primero la implementación existente: mediciones, interfaz, persistencia, scripts y
>   limitaciones. Conservar flujos y datos compatibles, corregir mediciones poco fiables y ampliar solo
>   donde aporte valor.
> - Medir **tiempo real de inicio de shell**, diferenciando explícitamente los modos relevantes
>   (interactiva, no interactiva, login, con y sin configuración) cuando se puedan reproducir
>   correctamente. No presentar la ejecución de un comando puntual como si midiera la apertura de una
>   terminal interactiva.
> - Soportar todas las shells y terminales compatibles con el proyecto mediante detección de
>   capacidades. Cada shell tiene semántica diferente de inicio y perfilado: usar sus mecanismos
>   soportados en lugar de forzar un método común incorrecto. No exigir que la shell benchmarkeada sea
>   la que ejecuta el installer.
> - Usar temporización monotónica, varias muestras, calentamiento opcional y límites razonables;
>   presentar mediana, media, mínimo/máximo, variabilidad, percentiles si hay muestras suficientes,
>   muestra válida/fallida y contexto de prueba. Evitar conclusiones engañosas causadas por outliers o
>   por muestras insuficientes.
> - Cuando sea posible, **perfilar etapas, scripts, funciones o plugins** responsables de la latencia,
>   mostrando coste y evidencia. Si una shell no ofrece introspección fiable, mostrar solo tiempos
>   globales y explicar la limitación; no inventar porcentajes por plugin.
> - Comparar resultados con baseline limpio donde sea factible sin modificar la configuración permanente
>   del usuario. Medir versiones/configuración suficientes para que las comparaciones sean
>   interpretables.
> - Mantener historial local optativo, comparativas antes/después, tendencia, regresiones, mejoras y
>   recomendaciones accionables. Atribuir causas solo cuando haya datos; separar "correlación" de
>   "causa confirmada".
> - UI: iniciar benchmark por shell/modo, ver progreso, cancelar, revisar desglose y abrir
>   recomendaciones; integración plena con temas y comportamiento actual.
> - Incluir consejos de optimización actuales, contextualizados y **no destructivos**. Cualquier cambio
>   automático en archivos de inicio debe ser revisable, confirmado y respaldado con backup; nunca
>   inyectar instrumentación permanente.
>
> ## Robustez y aceptación
>
> Evitar bloqueos por hooks interactivos, comandos colgados, banners, prompts, programas que requieren
> TTY, ejecución en PTY cuando sea imprescindible y diferencia entre duración de proceso y coste del
> terminal gráfico. No filtrar secretos de perfiles, variables ni salidas capturadas.
>
> Tests de estadísticos, tiempos anómalos, modos de inicio, shell ausente, fallos, timeouts,
> cancelación, historial y retrocompatibilidad. El resultado esperado es **una mejora comprobable del
> analizador actual**, con benchmarks reproducibles y métricas honestas, no un módulo paralelo.

L2. Audit result: the analyzer exists and is `shellaudit.go` plus `ScreenShellAudit`; the table above is
what it does and does not do. The decision that follows is to evolve that code rather than add a
parallel screen: `shellAuditState` keeps its fields, so the existing tests and the frame guards keep
their meaning, and the new work lands in new files beside it.

L3. Design decision on the fourth mode. "With and without configuration" is measured against a **clean
directory of the utility's own** (`ZDOTDIR` for zsh, `XDG_CONFIG_HOME` for fish, flags for bash and
nushell, an empty `ENV` for `sh`), never by moving, copying or rewriting anything of the user's. The
user's files are only ever read.

L4. Design decision on the PTY. The measurement is `-i -c exit` under our own stdin, with a deadline,
`WaitDelay` and no terminal attached; none of the supported shells needs a PTY for that, so no PTY is
allocated and no new dependency is added. Where no terminal is attached at all, the measurement is
still made and the *context* says so, instead of the screen refusing to measure -- the run in a pipe is
the case the old refusal turned into a missing number.

L5. Design decision on secrets. Attribution output is parsed and **redacted before it is stored**: the
zsh table keeps zprof's own names, the bash trace is parsed from the stream and only `file:line` plus
elapsed deltas are kept, and the fish table keeps only the first word of a command (plus the path after
`source`). No raw capture is written to the state file or drawn on screen.

L6. T3 in place: `shellaudit_engine.go` holds the plan (`shellAuditPlan`: mode, samples, warm-up,
baseline), the clamping (`shellAuditMaxSamples = 20`, `shellAuditMaxWarmup = 3`), the session
(buffered event channel, own context and cancel), the per-start events with their part and place, the
outcomes (finished, timed out, failed, cancelled) and `shellAuditRun` as the single record which the
flat state is a projection of. A timeout is never folded into a sample, a cancelled start is neither a
timeout nor a failure, and the last event carries the clamped plan that really ran, so a cancel cannot
shrink the plan a user chose. The version probe is now what the capability matrix decides the
instrument with, and the run reads the version once.

L7. T4 in place: `shellaudit_profile.go` reads the shell's own instrument where the shell has one --
zprof's table for zsh (wrapper directory with the user's own startup files stepped through, in all
four modes), bash's own `EPOCHREALTIME` trace read from the live stream (the elapsed time of a traced
command is the distance to the next stamp, attributed to the file it came from and the function it ran
in), and fish's `--profile-startup` table. Redaction is applied before anything is kept: a command is
reduced to its first word, a `source` keeps its path, and the bash trace is parsed line by line and
never stored. Bash's login start is not traced at all -- a login bash reads its profile files instead
of the rc file a trace can be pointed at -- and says so. Every profiled start is an extra start, never
one of the samples. Fish rows carry no share: every row's own sum already contains the rows under it,
so a denominator cannot be recovered from the table, and a percentage nobody can re-derive is exactly
the invented per-plugin number this front refuses to print.

L8. T3 and T4 landed under one work-unit commit: the engine dispatches into the instruments, so a tree
with the engine and without the profilers does not build, and a commit that does not build is not a
review unit. The two fronts' guards are in `shellaudit_engine_test.go` and `shellaudit_profile_test.go`.

L9. T5 in place: `shellaudit_baseline.go` holds `shellAuditComparison` and `compareShellAuditBaseline`,
the subtraction of the two medians with both numbers carried beside the difference so a reader can redo
the arithmetic. The floor is the spread's own (`shellAuditComparisonFloor = shellAuditSpreadFloor`), the
refusals are the substance of the file (another shell, another version, another mode, a cancelled side,
a side that could not be made, fewer runs than the floor), and the verdict is kept apart from the
arithmetic: the same number means "this is where your time goes" above a tenth of the start and "this is
the shell's own" below it. A negative difference is reported as the two starts not being separable, not
as a saving. Two real bugs were caught by these guards and fixed: the baseline's start time was read from
the nil pointer that was about to hold it (a panic on the first measured baseline), and the state had no
inverse projection, so the history and the comparison could not have read the record the screen draws.
The inverse (`shellAuditState.shellAuditRun`) now lives beside `withShellAuditResult` with a round-trip
guard over every field.

L10. T6 in place: `shellaudit_history.go` holds the stored measurement (`shellAuditHistoryEntry`: stamp,
shell, family, version, the start's key, counts, median/fastest/slowest, the clean comparison when one
was made, and at most the heaviest three profile rows, redacted names only), the sequence
(newest first, bounded at 50, one measurement that lands twice is one measurement), the trend
(`shellAuditTrend`: the median of the stored medians against this one, refused across another start,
another version and a cancelled measurement, quiet under ten percent) and the advice, whose three kinds
are the discipline of the section: `measured` (a fact about the measurement: the spread, the runs that
did not finish, the limit of what could be profiled), `observed` (what the instrument named, which is
not the same as why) and `confirmed` (the only attribution the utility can make, and only through the
clean comparison). The file lives beside the install record in the XDG state directory, is read once on
the startup path and never in `View`, is written through a temporary neighbour and a rename so a
half-written history is not a state the next run can read, and is written only when the user turned
history on: with the switch off the measurement is measured and stored nowhere. `shellAuditReport` is the
read model the screen will draw, so the record, the comparison, the trend and the advice are one read of
one state rather than four. The measurement also records the bound each start was given
(`shellAuditRun.Timeout`), because a start killed at it has no duration and the number on screen has to
say which of the two it is.

L11. T6's guards are `shellaudit_history_test.go`. Two things they pinned that are worth naming: the
stored fields are compared as the exact set of JSON keys, so a field added to the entry is a failing
guard rather than a number that quietly leaves the machine's memory; and the advice is checked for
causal language ("because", "is caused by", "due to"), because the profile can say where the time went
and cannot say why it went there.

## Remaining

T7..T9, with their evidence recorded here as each one lands.
