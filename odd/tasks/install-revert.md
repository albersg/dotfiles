# install-revert — a run that can be undone

Branch `fix/install-reversibility`, base `origin/main` = `8fbc87d`, worktree
`/home/albeertoo/my-things/dotfiles-install-audit`. Findings and evidence live in
`odd/tasks/install-reversibility-audit.md` (14 defects, D1–D14, with the six probes that
reproduce them).

## Specs

- **S1.** The owner's words: *"no quiero que quede nada roto cuando isntale algo"*. An
  installation that stops halfway must leave a state that is either usable or
  recoverable, and it must never destroy the user's own data.
- **S2.** The owner's words: *"y en el caso de que ocurra, por lo que sea, volver al estado
  anterior"*. The way back must exist, be reachable, and be guided rather than a paragraph
  of prose: today the failure screen offers `r` retry or `Enter` quit and never names the
  backup (D9).
- **S3.** The owner's words: *"quiero revisar exhaustivamente todos los ficheros y todo lo
  que se cambia durante la installation"*. The exhaustive inventory is delivered in
  `odd/tasks/install-reversibility-audit.md`; it is the input for what follows.
- **S4.** The owner's words: *"y poder revertir todos los cmabios. Quiero abarcar todos lso
  ficheros yc cambios que se producen"*. Every change the run makes to files must be
  revertible, and the revert must be verified end to end.
- **S5.** The selected scope, quoted from the decision: *"N3 — N2 + revert real verificado
  (recomendado)"* — the manifest, `--restore`/`--revert` with `--dry-run`, idempotent, with a
  round-trip test that compares the tree byte by byte on a disposable `$HOME`.
- **S6.** The accepted description of that scope: *"Convierte «volver al estado anterior» en
  un comando. Es lo que responde a tu petición: todos los cambios de ficheros revertibles y
  comprobados de punta a punta."*
- **S7.** The boundary the scope choice draws, stated so nothing expects more: package
  installations, Xcode CLT, the default-shell change and the two unpinned `curl | bash`
  installers are **out of scope** for reversal on this branch (they are N4). They must be
  named as irreversible where the user looks, never silently omitted.
- **S8.** A backup must never claim to hold what it does not hold: a symlink is preserved as
  a symlink or reported, an entry that cannot be copied is recorded rather than dropped, and
  one bad entry never destroys the entries already copied (D3, D4, D5).
- **S9.** A restore must never lose data: it verifies before it removes, it never returns
  success after failing to restore, and it leaves a snapshot of what it overwrote (D1, D2,
  D13).
- **S10.** There is a record of every run that changed anything, including a run that failed
  and a `--non-interactive` run, and the record says per path whether it was created,
  modified or deleted (D8).
- **S11.** `--verify` reports whether a path still holds what the run left, not merely that it
  exists (D10), and it does not overclaim: what it cannot check is said in the output.
- **S12.** Every rule above is pinned by a test that fails before the change, including a
  round trip (install → revert → the tree is what it was) on a disposable `$HOME`.

## Tasks

| ID | Specs | What it delivers | Route | Commit |
|----|-------|------------------|-------|--------|
| T1 | S8 | Backup integrity: owned marker, no directory collision, symlinks preserved, skipped entries written into the backup, a bad entry cannot delete the good copies | inline | `aac0eb8`, `cd80338` |
| T2 | S9 | Safe restore: verify before removing, stage then swap, pre-restore snapshot, no silent skip, confirm text that matches the code | inline | `4d2db65` |
| T3 | S10 | Run manifest: per path action + digest + backup reference, written on failure and by `--non-interactive` | inline | |
| T4 | S4, S5, S6 | `--revert` and `--restore` from the manifest, with `--dry-run`, idempotent, plus the round-trip test | inline | |
| T5 | S11 | `--verify` compares content against the manifest digests | inline | |
| T6 | S2, S7 | The failure screen and the review screen name the way back and what cannot be undone | inline | |
| T7 | S3, S12 | Docs follow the new rules (`docs/ROLLBACK.md`, `docs/tui-installer.md`, `--help`), and the audit document is committed | inline | |

High risk by the orchestrator's list: item 1 (stored data, deletion, format) and item 3
(the CLI contract gains commands). Independent read-only verification is required at the end;
one `gentle-ai-verify` pass over the whole diff.

Work is isolated in this worktree because the `feat/start-installation-flow` session has
`install*.go`, `update.go`, `view.go`, `model.go` and `panels.go` open in the main worktree;
this branch rebases after that one lands. Publication is centralized in the coordinating
session.

## Log

- **L1 (owner, verbatim).** "quiero tambien que te encargues de,  ani vel de instalacion,
  quiero revisar toda la parte de backup etc... no quiero que quede nada roto cuando isntale
  algo, y en el caso de que ocurra, por lo que sea, volver al estado anterior. quiero revisar
  exhaustivamente todos los ficheros y todo lo que se cambia durante la installation, y poder
  revertir todos los cmabios. Quiero abarcar todos lso ficheros yc cambios que se producen"
- **L2 (owner decision).** Scope **N3** (report N2 core + real verified revert) over N1, N2
  and N4; and **start now, isolated in a worktree** rather than waiting for
  `feat/start-installation-flow`. Consequences accepted: system effects (packages, Xcode CLT,
  unpinned installers) are recorded and named as irreversible, not reversed (S7).
- **L3 (findings, evidence).** The read-only audit ran three explorations and six probes on
  the pinned base. Six defects are reproduced by execution, two of the probes with
  assertions that fail today: a restore that deletes the user's only copy and returns
  success; a restore that leaves behind what the install created; a symlink that becomes an
  empty directory in the backup without being reported; a single non-regular entry that
  destroys the whole backup including the copies already made. Details and the other eight
  defects in the audit document.
- **L4 (T1, done).** `cd80338`. A backup now carries its own manifest, so it can say what it
  holds and what it does not: `Entries` names every entry written, `Skipped` names every
  entry it could not write with the reason, and `Failed` keeps the copies already made
  instead of deleting the whole directory. A symlink stays a symlink, including a dangling
  one. Two directories can no longer collide inside one second. `ListBackups` accepts a
  directory this program wrote and no longer offers a stranger's directory as restorable and
  deletable. Evidence: five tests in `backup_integrity_test.go` that failed to compile before
  the change; two stale pins rewritten, one of which passed by accident because `Stat`
  followed the link the test meant to check.
- **L5 (T2, done).** `4d2db65`. The restore verifies before it removes: the replacement is
  copied beside the destination and only a complete copy moves into position, so the
  destination is never absent because a copy failed halfway, and a swap that cannot finish
  puts the user's own file back. The state it replaces is copied into a backup of its own
  first, so a restore is itself undoable, and a directory this program did not write is
  refused. The restore now accounts for itself per entry, which is what separates "your
  configuration is back" from "two of the five files are back". Evidence: six tests in
  `restore_safety_test.go`, two of them covering the two ways an entry cannot be put back
  (a named pipe, and a file the process cannot read). One of my own tests was wrong on the
  first run and the failure taught the rule: a dangling symlink *is* restorable, because the
  backup preserves it as a symlink, so it cannot serve as the unrestorable case. The old pin
  that hand-made an empty directory inside the developer's real `$HOME` now pins what happens
  for a backup this program wrote and for one it did not.
- **L6 (T3, done).** The record names every file the run changed, one line per file, with the
  digest it held before the run and the digest it held after, the configuration key it
  belongs to, and the copy in the backup that puts it back. It is written by a run that
  failed as well as by one that finished, and by the scripted route as well as by the
  interface: before this, a run that stopped at a failed step left no record at all, which is
  precisely the run whose changes are already on the machine. Evidence: five tests in
  `install_manifest_test.go`, and the existing record tests keep passing with the new fields.
