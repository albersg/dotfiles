# The administrator password, asked inside the interface

## Goal

The installer currently leaves its own screen to ask for the administrator password:
`sudoPrimer` hands the terminal to `sudo -v`, sudo writes its prompt on the normal screen, and the
interface comes back afterwards. That back-and-forth is the mechanism as the code documents it, and
it is not a defect — but it is not what the user wants. This front makes the whole exchange happen
inside the installer: the prompt, the masked input, and the outcome.

## Specs

- **S1 — the terminal is never handed over for the password.** "que no veulva aa terinal, para
  perdite la password, y luego vuelva al installer" — no `tea.ExecProcess` on the credential path,
  no leaving the alternate screen, no visible hop out and back.
- **S2 — all of it is inside the installer.** "me gustaria que fuese todo dentro del sitnaller" —
  the prompt, the masked input and any error are drawn by the interface, on its own screen.
- **S3 — only if it can be done without breaking the install.** "si esp osible" — the ordinary path
  must work without a terminal. Where authenticating without one genuinely cannot work, the run must
  say so and still offer a way forward rather than dead-ending.
- **S4 — the password is not kept.** Not in the model, not in the command line, not in a log line,
  and not in a Go `string` that cannot be cleared. It exists as bytes, for as long as sudo takes to
  read them, and then it is overwritten.

## Why S4 is a design constraint and not an afterthought

`Update` returns the model **by value** — every keystroke produces a copy. A password held as a
`string` would be copied an unknown number of times, and Go strings are immutable, so none of those
copies could be cleared. The password is therefore held in a `[]byte` of fixed capacity: the slice
header is copied with the model, but the backing array is shared, so overwriting it once clears
every copy. Capacity is fixed so an append can never allocate a second array behind our back.

## Tasks

- [ ] **T1 — The prompt is a screen of its own (S1, S2).** A masked field that takes runes,
  backspace, enter and escape. It shows the number of characters typed and never a character
  itself. Route: inline, the interface has no text-input component to reuse.
- [ ] **T2 — sudo reads the password without a terminal (S1, S3).** `sudo -S -v` with the password
  on the child's standard input, run as a `tea.Cmd` while the interface keeps the screen. No
  `ExecProcess`. Route: inline, test-first against the existing `sudoPrimer` seam.
- [ ] **T3 — the bytes are cleared (S4).** A fixed-capacity `[]byte`, overwritten immediately after
  sudo has been handed the reader, and never placed in the model as a string, in the log, or in
  `exec.Command`'s arguments. Route: inline, with a guard that asserts both the capacity rule and
  the clearing.
- [ ] **T4 — the way out stays reachable (S3).** A wrong password leaves the screen up so it can be
  retyped, and the escape key falls back to the terminal handover for a system whose PAM needs a
  TTY. The automatic re-ask still never happens: only the user starts another attempt.
- [ ] **T5 — the at-most-once promise is preserved and re-pinned.** The three existing tests that
  pin "the credential is requested exactly once" must still describe the run's behaviour after the
  change; where they no longer do, they change because the behaviour did, not to go green.
- [ ] **T6 — the documentation says which one happens.** `docs/tui-installer.md` and the CHANGELOG
  describe the in-interface prompt and name the fallback, so the next person does not read the
  handover comment and think it is still the normal path.

## Log

- **L1** (verbatim, the user's request): "osea me guastaria, que, cuadno sinstallas con el
  installer, que no veulva aa la terinal, para perdite la password, y luego vuelva al installer, me
  gustaria que fuese todo dentro del sitnaller, si esp osible"
- **L2**: this supersedes the disposition recorded in issue #318. There the reported failure
  ("the installer does not come back") did not reproduce and the step-aside was judged to be the
  mechanism rather than a defect. The user has since said plainly that the step-aside itself is
  what they do not want, which turns it from a defect question into a product decision, and this
  front is that decision.
- **L3**: the cost, stated plainly and accepted. An in-interface prompt means the password exists
  in the installer's memory, and `sudo -S` cannot authenticate on a system whose PAM requires a
  terminal. T4 keeps the handover reachable for exactly that case instead of leaving a dead end.
