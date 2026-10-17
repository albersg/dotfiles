# Terminal ownership: who reads the terminal when the capability screen is open

The install check's front reported one thing it did not fix: the capability probe opens `/dev/tty` and
queries the terminal for its capabilities while bubbletea is holding a read on the same terminal, so
two readers share one input queue. Whether that loses the answer, steals keystrokes or nothing at all
was never measured. This front measures it first, and only then decides what to change.

Work happens in a dedicated worktree (`dotfiles-probe-ownership`, branch `fix/terminal-probe-ownership`)
because other sessions write to the same clone.

## Specs

- **S1.** The owner asked which option was best for the open defect and then authorized the one
  proposed, verbatim: "cual es la mejor?" followed by "dale". The authorized proposal, verbatim:
  "abrirle un front propio, y que su primera tarea sea reproducirlo, no arreglarlo" — a front of its
  own whose first task is to reproduce the interference rather than to fix it.
- **S2.** No change to terminal I/O is made before a measurement exists. The defect's premise is
  unproven: the first front's audit could not confirm it by execution, and its own record says so.
- **S3.** The measurement must answer three questions separately, because each implies a different
  fix: does the probe's query reply reach the probe while bubbletea reads the same terminal; if it
  does not, which reader consumed it; and can raw terminal reply bytes reach bubbletea's input and be
  interpreted as keys. Each answer is reported with the command that produced it.
- **S4.** The measurement is made with the same reader bubbletea uses in production
  (`installer/cmd/dotfiles/main.go:207-210` installs no `tea.WithInput`, so bubbletea reads `stdin`)
  and the same access the probe uses (`terminal_probe.go:253-260`), under one responder, so that the
  only difference between the measured cases is which readers are attached.
- **S5.** The mechanism is chosen by the owner with the measurement in front, from the three options
  this front records with their costs. A fix that reverses a documented decision -- that the probe
  runs when the screen is opened and never at startup, and that the installer does not hand the
  terminal over -- says so explicitly instead of doing it quietly.
- **S6.** Whatever is decided, `detectTerminalCapabilities` keeps its three states and the screen
  keeps saying `unknown` with the reason it could not measure, because that honesty is what makes a
  lost reply a limitation rather than a wrong answer.

## Tasks

- [x] **T1** (S2) This document: the premise, the code each half of it rests on, the measurement spec
  and the three candidate mechanisms with their costs. Route: inline. Commit: this file.
- [x] **T2** (S3, S4) The measurement: does the reply reach the probe, which reader consumed it, and
  can reply bytes be read as keys. Route: inline; the harness is a diagnostic and its output is the
  deliverable. Commit: the recorded output in the Log.
- [x] **T3** (S1, S5) The owner's mechanism decision, with the measurement in front of them. Decision:
  **mechanism 2**, the one reader, for the reason the measurement made plain (a lost answer is a
  limitation the screen can disclose; the answer arriving as keystrokes is not).
- [x] **T4** (S5, S6) The change, its guards, and the doc updates. Route: inline, test-first. Commits:
  `56f1494` (the instrument), `83c092e` (the change), and this file.
- [ ] **T5** (S4, S5) The configuration the gate deliberately leaves alone: when stdin is not the
  terminal, bubbletea opens the controlling terminal itself and the probe opens it too, so the arrangement
  the one-in-twenty was measured in still has two readers. Closing it means opening the controlling
  terminal in `cmd/dotfiles`, handing that stream to the same one reader, closing it when the program
  ends, and measuring it the same way (the diagnostic under `script(1)` with stdin away from the pty).
  Open, with its evidence in "What the corrected instrument read".

## What is known, with the code it rests on

**The probe's I/O half.** `terminalReply()` (`installer/internal/tui/terminal_probe.go:253-260`) opens
`controllingTerminal` (`/dev/tty`, `:57`), returns `""` if it cannot, and otherwise calls
`queryTerminal(f, f, terminalCapabilityQuery, terminalQueryTimeout)` -- **the same descriptor as both
the write side and the read side**, with the whole exchange bounded by `terminalQueryTimeout`
(`250ms`, `:62`).

**The interpretation half is separated on purpose.** `detectTerminalCapabilities(getenv, reply)`
(`:286-297`) takes the reply as a string and performs no I/O, which is why the three-state rules are
testable without a terminal. The consequence is that the I/O half has **no test at all**: a grep for
`terminalReply` finds its definition and one caller (`:271`, inside `loadTerminalCapabilityCmd`) and
nothing else. The part that can be wrong on a real terminal is the part nothing measures.

**When it runs.** The probe is armed as a `tea.Cmd` from the screen that needs it
(`terminalCapabilityCmdIfNeeded`, `:275-284`), never at startup, and it runs once.

**Who else reads the terminal.** `installer/cmd/dotfiles/main.go:203-215` builds the program's options
with `tea.WithOutput(outputWriter(os.Stdout))` and starts `tea.NewProgram(model, options...)` with no
`tea.WithInput`, so bubbletea reads `os.Stdin` -- the same terminal device the probe opens through
`/dev/tty`. Both read the terminal's input queue: bubbletea continuously, from its own goroutine, and
the probe for up to 250ms after it writes a query. Nothing arbitrates between them.

**What the terminal sends.** A capability query is answered by the terminal writing an escape sequence
into the *same* input queue a keystroke goes into. So the reply is not on a private channel: it is
input, and whoever reads the queue first consumes it. That is the whole premise, and it is why the
measurement has to look at both consequences -- a lost answer and a stray input event -- and not only
at the first.

## The measurement (T2)

The controllable difference is which readers are attached, under a terminal that answers queries. Three
cases, run in the same terminal emulator on the same host:

| Case | Attached readers | Question it answers |
|------|------------------|---------------------|
| A | the probe alone, raw terminal | does this terminal answer the probe at all (the baseline) |
| B | the probe + bubbletea reading `stdin` | does the answer still reach the probe when the installer's reader is attached |
| C | bubbletea reading `stdin`, probe run concurrently | does a reply byte sequence arrive at bubbletea as input, and what does it become |
| D | the probe alone, canonical terminal | can the probe read a reply on the terminal as it is before the interface starts |

A and B together are the measurement: if A answers and B does not, or answers shorter, the response is
being consumed by the installer's own reader, which is the defect. C says whether the loss also
corrupts input, which decides how severe it is. The terminal on the far side has to answer these three
queries -- DECRQM for mode 2026 and two XTGETTCAP asks -- so the harness supplies one that does;
`tmux` would be a responder in principle but is not installed on the host this was measured on.

## What the measurement found (T2)

The responder is `scripts/measure-terminal-ownership-stub.py`: it answers the way a terminal that
supports all three would (`\x1b[?2026;2$y`, then `\x1bP1+r524742=31\x1b\\` and `\x1bP1+r4D73=31\x1b\\`).
`script(1)` supplies the pty and the controlling terminal, the terminal is held in the raw mode the
interface sets in production, and `installer/internal/tui/probe_ownership_measure_test.go` makes each
case's reading. The whole measurement is one command:

```
scripts/measure-terminal-ownership.sh alone|shared [runs]
```

**Case A, the baseline: the probe on a terminal nobody else is reading.** In 5 of 5 runs the probe
received all 41 bytes and `detectTerminalCapabilities` read them as `colour=supported
clipboard=supported sync=supported ... (the terminal's reply to ...)`. The harness is therefore sound,
this terminal answers all three queries, and the bytes the probe receives are the bytes it can use.

**Case B, the installer's arrangement: the probe with the interface reading `os.Stdin`.**

| runs | the probe received | the interface received |
|------|--------------------|------------------------|
| 19 of 20 | 41 bytes, all three `supported` | 0 messages |
| 1 of 20 | **0 bytes** | **7 messages** |

Twenty runs, and not one of them in between: the reply goes to one reader or to the other, never split,
and the losing reader receives nothing at all. **Corrected in T4:** this sample's arrangement is the
interface reading **its own open of the terminal**, not `stdin` -- the harness's second defect (recorded
in the section below) was also a defect of the measurement. The numbers stand for that arrangement; they
are not the rate of the arrangement the installer commonly runs in. In the run the interface won, one 41-byte reply arrived
as seven messages:

```
tea.unknownCSISequenceMsg                                     <- the \x1b[?2026;2$y half, swallowed
KeyMsg type=runes runes=['P'] alt=true paste=false            <- \x1bP, read as alt-P
KeyMsg type=runes runes=['1' '+' 'r' '5' '2' '4' '7' '4' '2' '=' '3' '1'] alt=false   <- the XTGETTCAP body, read as typed text
KeyMsg type=runes runes=['\\'] alt=true paste=false           <- the string terminator
... the second XTGETTCAP reply, the same way
```

The consequence is visible in the run the probe lost: `colour` fell back to `COLORTERM=truecolor` and
`clipboard` and `sync` answered `unknown (not determined)`. That is one screen answering `unknown` about
a terminal that supports all three, decided by which goroutine read first -- and, in the same run, the
terminal's answer entering the interface as input.

**Case D: the probe alone on a canonical terminal.** The same instrument with the terminal as it is
before the interface starts -- `scripts/measure-terminal-ownership.sh alone 3 canonical` -- returned
**0 bytes in 3 of 3 runs**, where the same case on a raw terminal returned all 41 bytes in 5 of 5. The
reason is not a race but a rule: `queryTerminal` writes to and reads from the descriptor the probe
opened (`terminal_probe.go:191-215`), and neither it nor `terminalReply` touches termios, so the probe
can read a partial reply only because the interface has already set raw mode. In canonical mode a read
returns at a newline and a capability reply contains none, so the reply reaches nobody until the
deadline passes. Moving the probe ahead of the interface would therefore not move the question to a
quieter terminal; it would move it to a terminal that cannot answer at all, unless the probe also learns
to set raw mode itself -- in a component that today touches no termios.

## The three mechanisms, with their costs (T3)

1. **The probe keeps its own reader and its own deadline, and the query is written only when nothing
   else could be reading.** Costs least and touches no other component; it cannot prevent a race it
   does not control, so if the measurement shows loss, this option's honest form is to say so in the
   screen, which is what it already does for an unanswered query.
2. **The probe reads through bubbletea's own input reader**, with the reply filtered out of the key
   stream. Correct about ownership and the only option that removes the race rather than reporting it;
   it is invasive -- it couples the probe to the interface's input loop, and it has to be right about
   the reply's framing, or it swallows a keystroke.
3. **The query runs before the interface starts and is cached on the model.** The simplest to reason
   about, and it **reverses a documented decision**: the probe deliberately runs when the screen is
   opened and never at startup, so that a run which never opens that screen writes not a single query
   byte. Choosing it means changing that rule and the document that states it, on purpose.

## What was decided and built (T3, T4)

**The owner chose mechanism 2**: the interface reads the terminal through one reader, and the probe's
answer leaves that stream. It is the only one of the three that removes both halves of the defect -- a
lost answer is a limitation the screen can disclose, but the terminal's answer arriving as keystrokes is
a hazard no screen can disclose.

**The reader** (`installer/internal/tui/terminal_input.go`) is the interface's input and the probe's
answer at once. `TerminalInput` wraps the stream and registers it; `Read` hands every byte on untouched
except while a query is outstanding, when an answer that starts with a header no keystroke can produce is
taken out of the stream and given to the probe. It never holds a byte back to decide later, so no
keystroke is delayed; a read boundary that falls inside an answer's own header is the one case it cannot
decide, and that fragment is handed on (recorded by its own guard). Two consequences are deliberate and
bounded, and both are in the code: that fragment is forwarded rather than held back, because holding a
byte is how a keystroke gets delayed, and an answer that arrives after the deadline is input again,
because by then nothing is waiting for it -- the position the interface would be in if the probe never
read the terminal at all.

**Why it must keep its descriptor.** bubbletea puts the terminal into raw mode on the `Fd()` of the input
it is given (`tea`'s `initInput`), so a wrapper without one would be a program that never enters raw mode
at all. The type asserts that shape at compile time.

**Handing it over is a decision, not a default, and the decision has to be bubbletea's own.** Given no
input option, bubbletea reads stdin when stdin is a terminal and opens the controlling terminal when it
is not (`tty_unix.go:17`, `tea.go:604-617`); passing any option takes that choice away, and a program
given a pipe reads the pipe and no key ever reaches it. So `cmd/dotfiles` hands the reader over exactly
when bubbletea would have chosen stdin: the predicate is bubbletea's own, `charmbracelet/x/term`'s
`IsTerminal`, applied to the same descriptor the wrapper reports. The stream is therefore identical by
construction, and the only thing that changes is that the probe's answer leaves it.

**Two tests that look right and are not, kept because both were written first.** This repository's own
`isTerminal` (`sync_output.go`) is `ModeCharDevice`, and `/dev/null` satisfies it: a run started with it
as its input would have been handed a stream that never carries a key. And comparing devices with
`os.SameFile` against an opened `/dev/tty` matches **nothing, ever** -- that descriptor reports the
devtmpfs node (dev 5, inode 10), not the terminal it stands for -- so the reader was never handed over and
the change was **inert** until the guard's skip message printed both devices (`stdin` as `Rdev=34839`, that
is 136:23, the pty; `/dev/tty` as `Dev=5 Ino=10`). Both wrong versions are described in the code, for the
same reason this document exists.

**The instrument was wrong first, and that is recorded here rather than quietly fixed.** The harness ran
the measurement through `go test`, which hands the test binary a stdin that is not the terminal it was
started from; with the reader wrapping that stdin, the answer reached nobody, and the reading came back
`0 bytes / 0 messages` in 20 runs of 20 while the program's own error said `<nil>`. It was the *shape* of
the result -- not a leak, not a loss, but nothing at all -- that pointed at the instrument. Two things
fixed it, and both are in the branch: the runner builds the test binary once and runs it under
`script(1)` itself, which is what puts the pty on stdin, and the reading now names the stream it measured
(`case B: the program's stdin is /dev/pts/23`) and reports the program's own error, so the same failure
cannot look like a defect in the reader again. What the broken instrument found in passing was a real
defect in the change -- the unconditional `tea.WithInput` -- which is now the gate described above. The
lesson is the one this repository keeps re-learning: a measurement that does not state the arrangement it
measured is not evidence.

## What the corrected instrument read (T4)

The second harness defect did not only misreport the change; it also misreported **what had been
measured**. `go test` hands the test binary a stdin that is not the terminal the suite was started from,
so the `shared` case was not measuring the arrangement `cmd/dotfiles` runs in: bubbletea, finding stdin
not a terminal, opened `/dev/tty` itself (`tea`'s `openInputTTY`), and the two readers were two separate
descriptors on the same terminal. That is a real configuration -- a run whose stdin is redirected is
exactly that -- but it is not the common one, and it had been reported as if it were.

With the pty on stdin -- the arrangement the installer actually runs in -- and with the runner naming the
stream it measured:

| arrangement | the probe received the reply | the interface received |
|-------------|------------------------------|------------------------|
| `shared`: bubbletea reads `stdin`, the probe reads `/dev/tty` | 20 of 20 | 0 messages, 20 of 20 |
| `diverted`: bubbletea reads through the one reader | 20 of 20 | 0 messages, 20 of 20 |

The honest reading is therefore narrower than the first sample and wider than this table:

- **The race is real and was observed.** In the arrangement where the interface's reader is its own open
  of the terminal, one run in twenty lost the reply to the interface, and in that run the terminal's
  answer arrived as input. That evidence stands, for that arrangement (L4, L6).
- **In the arrangement stdin is the terminal, this sample lost nothing.** Twenty runs is a sample, not a
  proof: the same queue and the same two readers are there, and bubbletea's own reader is an
  epoll-then-read loop rather than a blocked read, which is a plausible reason the probe won every time.
  This instrument did not test that hypothesis, and the document does not claim it.
- **The change leaves one reader where it applies**, which is why the measurement can say the probe
  received the reply in 20 of 20 and cannot say "fewer losses": in the arrangement it replaced, this
  sample had none to lose.

**What the change does not cover, written down because it is measured and not because it is small.**
`programInput` leaves stdin alone when stdin is *not* the terminal, so in that configuration bubbletea
opens `/dev/tty` and the probe opens `/dev/tty`, and the arrangement the one-in-twenty came from is the
one the gate preserves. Closing it is the same two lines of policy bubbletea applies, with the one reader
in between: open the controlling terminal in `cmd/dotfiles`, hand that stream to `TerminalInput`, close it
when the program ends, and measure it the same way -- the diagnostic under `script(1)` with stdin
redirected away from the pty. It is not done here, and the reason is recorded rather than implied: the
stand-as-it-is gate is the conservative half of the owner's decision, chosen so that a measured rare loss
is not traded for an unmeasured broken run, and the other half needs its own measured round (T5).

## What the gate found (T4)

The front gate is this repository's own (`make check`, `make preflight`), and it is the reason this front
did not ship a broken change. Two of its findings are recorded here because they are this front's
evidence, not an anecdote.

**A guard I had just written was failing, and my own "green" check had never run it.** The gate failed on
`TestTheAnswerIsDivertedWholeWhenItArrivesInPieces` -- a test added in this very task -- while the earlier
local check had reported everything green. Both were true: that check selected its tests with a `-run`
pattern, and the pattern did not match this test's name, so the test was never executed and could not
fail. The test was also wrong in itself: it cut the answer at byte 12, which is the escape introducing the
*next* answer's header rather than the end of the mode answer (byte 11), so the reader forwarded a lone
escape -- correctly, since it never holds a byte back -- and the test measured the reader's documented
limit instead of the continuation it meant to measure. It is now built from the three answers' own lengths
instead of offsets, and every guard in the file is run **by name**, with the count of `--- PASS` lines as
the evidence rather than the exit code alone. **A `-run` pattern that does not match the test you mean is
not evidence that it passed** (L7).

**The change was inert, and only a skip message found it.** `make check` passed the cmd guards: the pipe
guard and the `/dev/null` guard both passed, and the positive guard *skipped*, which looked like a host
without a terminal. It was not. The skip printed the two devices it had compared, and they can never
match: stdin reported the pty (`Rdev=34839`, 136:23) while an opened `/dev/tty` reported the devtmpfs node
(`Dev=5 Ino=10`), because that descriptor stands for the terminal without being it. So
`isControllingTerminal` answered false in production too, `programInput` returned nil, and the one reader
was **never installed** -- the fix was a no-op behind a passing suite. The predicate is now bubbletea's
own (the same `x/term IsTerminal` bubbletea calls to make this decision), and the positive guard passes
under `script(1)`, where the tests do have a terminal on stdin. **A guard that skips silently can hide the
absence of the thing it guards**; printing the evidence of *why* it skipped is what turned this one
around (L7).

**The final reading of both gates on this branch.** `make check`: `gofmt` PASS, `go vet` PASS,
`go test ./cmd/dotfiles ./internal/tui` failing only on the four Ghostty tests. `make preflight`: the same
four and nothing else, with the same causing line (`Ghostty already installed`, from the `/usr/bin/ghostty`
on this host). Four failures, the same four names, and the identical result when the same two gates were
run on the pristine `8fbc87d` before this front began: this branch adds no failure to either gate.

## Log

- **L1.** The defect is not this front's discovery; it comes from the audit of the Utilities section and
  is recorded there with the same evidence: `terminal_probe.go:66-72` and `:275-282`,
  `cmd/dotfiles/main.go:207-209`, reported as **unconfirmed by execution** and therefore not fixed. The
  Utilities front's own log says it was "reported with its evidence as an open item" for exactly one
  reason -- closing it "means choosing between a probe under its own read deadline and a program started
  with `tea.WithInput`", which is a design decision that front did not own.
- **L2.** The owner asked which of the three options -- leave it documented, fix it in the merged
  front, or open a front of its own -- was best. The recommendation was the third, and it was accepted:
  a front whose first task is the reproduction, because **the premise is unproven**, and because the
  fix is a change of terminal ownership that touches three documented decisions of the repository (the
  probe never runs at startup; the installer does not hand the terminal over, which the password screen
  paid for; the probe must be able to answer `unknown` honestly). Changing shared terminal I/O on an
  unproven premise is the kind of unrequested break the repository's own risk rules warn about.
- **L3.** The evidence re-read in this branch, on the merged tree: `installer/cmd/dotfiles/main.go:203-215`
  installs `tea.WithOutput` and no `tea.WithInput`; `terminalReply()` opens `/dev/tty` and passes the
  same descriptor to both sides of `queryTerminal`; the probe is armed only from the capability screen;
  and the I/O half has no test while the interpretation half has its own guards. The premise therefore
  rests on four facts that are all visible in the code, and on one that only a terminal can answer: what
  actually arrives, and where.
- **L4.** The measurement (T2), with the command it came from:
  `scripts/measure-terminal-ownership.sh alone|shared [runs] [raw|canonical]`, which puts
  `installer/internal/tui/probe_ownership_measure_test.go` under a pty from `script(1)`, holds the
  terminal in the mode asked for, and runs `scripts/measure-terminal-ownership-stub.py` on the far side
  as a terminal that answers all three queries. **Case A** (probe alone, raw): 41 bytes in 5 of 5 runs,
  read as `colour=supported clipboard=supported sync=supported`. **Cases B and C** (probe with the
  interface reading `os.Stdin`, as `cmd/dotfiles` starts it): 20 runs, the probe received the whole
  reply in 19 and **0 bytes in 1**, and in that one run the interface received the reply as **7 input
  messages** -- `\x1bP` as alt-P, an XTGETTCAP body as typed text, the DECRQM half swallowed by the
  parser. Not one run split the reply between the readers. **Case D** (probe alone, canonical): 0 bytes
  in 3 of 3, against 41 bytes in 5 of 5 on a raw terminal, which is the raw-mode dependency measured
  rather than argued.
- **L5.** Two corrections of this front's own instrument, kept because both are the kind of error that
  would have produced a confident wrong answer. First, the measurement section's first draft named
  `tmux` as the terminal on the far side; `tmux` is not installed on the host this was measured on, so
  the responder is the stub script, and only the stub's answers are ever reported as the measurement's
  own. Second, an intermediate scripted aggregation of the first sample reported "1 message" in every
  run including the three that received none, because its pattern counted the report's own summary line
  ("the interface received N messages") as if it were a message; the per-run literal lines settled it,
  and they agree with the first reading. An aggregate that also matches the sentence it is counting is
  not evidence.
- **L6.** The corrected instrument's reading (T4), and the correction it forces on this document's own
  earlier numbers. The T2 sample put the interface on **its own open of the terminal**, not on `stdin`,
  because `go test` does not hand the test binary the pty the suite was started from -- the harness's
  second defect was also a defect of the measurement. Its numbers stand for that arrangement (one run in
  twenty lost the reply, and that run's answer arrived as input) and are not the rate of the common one.
  With the pty on stdin and the stream named in every reading, `scripts/measure-terminal-ownership.sh
  shared 20` gave the probe the whole reply in **20 of 20** runs and the interface 0 messages, and
  `diverted 20` the same, 20 of 20. Two conclusions are kept: the defect this front fixes is reachable in
  the configuration where the interface's reader is its own open of the terminal (which the gate
  preserves, T5), and "1 in 20" is not the common arrangement's rate. **A sample that does not name the
  arrangement it sampled cannot be quoted as that arrangement's rate** -- the same lesson as L5, one level
  up.
- **L7.** Two lessons the gate produced, kept because the front would otherwise have shipped them. First:
  the local "green" check before the gate selected its tests with a `-run` pattern that did not match
  `TestTheAnswerIsDivertedWholeWhenItArrivesInPieces`, so a test with a wrong boundary (byte 12, the escape
  before the next header, instead of byte 11, the end of the mode answer) ran for the first time inside
  `make check` and failed there. It is L5's mistake one level up: a list that does not name what it covers
  is not evidence that it covered it. The file's guards are now run by name, and the count of `--- PASS`
  lines is the evidence, not the exit code alone. Second: the gate's *passing* cmd guards included a
  **skip**, and the skip was hiding an inert change -- `os.SameFile` against an opened `/dev/tty` can never
  match it (stdin reports the pty, `Rdev=34839` = 136:23; `/dev/tty` reports the devtmpfs node, `Dev=5
  Ino=10`), so the reader was never handed over in production while the suite stayed green. What exposed it
  was the skip message naming both devices, which is why the guards here print *what* they compared instead
  of only that they could not. Predicate corrected to bubbletea's own (`x/term IsTerminal`); the positive
  guard passes under `script(1)`. Both lessons have the shape of this front's earlier ones: **what makes a
  check worthless is not its failure but its silence.**
