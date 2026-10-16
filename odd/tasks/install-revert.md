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
| T3 | S10 | Run manifest: per path action + digest + backup reference, written on failure and by `--non-interactive` | inline | `0ff4ce6`, `637c916` |
| T4 | S4, S5, S6 | `--revert` and `--restore` from the manifest, with `--dry-run`, idempotent, plus the round-trip test | inline | `e201411`, `1b52dfe` |
| T5 | S11 | `--verify` compares content against the manifest digests | inline | `c8f6b72`, `1b52dfe` |
| T6 | S2, S7 | The failure screen and the review screen name the way back and what cannot be undone | inline | `6727d33` |
| T7 | S3, S12 | Docs follow the new rules (`docs/ROLLBACK.md`, `docs/tui-installer.md`, `--help`), and the audit document is committed | inline | `e06213d` |

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

- **L7 (T4, done).** `e201411`. "Back to the previous state" is a command: `dotfiles --revert`
  walks the record file by file, copies back what the run modified or removed, removes what
  it created, and compares every file against the digest the run recorded. It keeps what it
  replaces in a backup of its own, so a revert is undoable by the same command; it is
  idempotent, reporting an already-correct file as already there; and it names a file whose
  copy is gone as `unrevertible` instead of skipping it. `dotfiles --restore <dir>` does the
  same for one directory named by hand, and `--dry-run` on either prints the plan and touches
  nothing. The exit statuses separate "nothing to do" from "it went wrong": no record and a
  refused directory are 2, a file that could not be put back is 1. Evidence: five tests in
  `revert_test.go` (the round-trip runs one of each change -- modified, created, deleted --
  and compares the tree byte by byte against a snapshot of the tree before the run, then
  reverts twice to prove idempotency) and five in `revert_cli_test.go` for the command line.
  The irreversible effects are one list in `install_irreversible.go`, printed by the command,
  so the boundary N4 accepted is named where the user is looking rather than omitted.
- **L8 (T5, done).** `c8f6b72`. `--verify` compared presence and said so; a record that names
  a digest can answer a better question, so it now compares content: a file still holding what
  the run left is `here`, one that no longer does is `changed` with both digests, one that
  cannot be read is `unknown`, and a file the run removed is part of the state it left -- named
  while it stays gone and when something puts it back. The screen counts what is still what
  the run left instead of claiming the machine matches, and a record written before digests
  existed keeps the weaker presence claim in the same words. Evidence: six tests in
  `install_verify_content_test.go`; the verification golden was regenerated after its own
  header was found to be lying (it still claimed the record names no digest), and the
  verification-screen pins were corrected with it.
- **L9 (T6, done).** `6727d33`. The failure screen offered retry or quit and never mentioned
  that the run had already copied what it replaced; it now carries the way back -- the
  directory, the command, and what no command undoes -- above the log, because the log is
  context for the answer rather than the answer. The review screen names the same before the
  run can start, with the honest variant for a run that will take no backup. Both read the
  same sentences the command prints, so the screens and the command cannot drift. The review
  screen's block lives in the rows the list did not need, the rule that frame already follows
  for the panel summary and the creature: in the body it pushed the frame three rows past
  80x24, and the frame guard said so. Evidence: four tests in `install_wayback_test.go`, the
  error golden regenerated to pin the common case (a run that had taken the backup) with the
  no-backup variant pinned by a test instead of a second frame, and the whole package green
  with the four environment-only Ghostty tests skipped.
- **L10 (T7, done).** `e06213d`. `docs/ROLLBACK.md` said in print that there is no restore
  flag and no restore subcommand, and gave the manual copy as the only way back; it now
  documents the two things an install leaves (the backup with its `backup.json` manifest and
  the record with its per-file digests), `--revert` and `--restore` with their exit statuses,
  the three rules the restore is built on, `--verify`'s content check, and the boundary -- what
  no command undoes, named. The manual path stays for the machine where the installer cannot
  run, with the correction that matters: copy over the target instead of removing it first.
  `docs/tui-installer.md` repeated the verification screen's own claim that the record names
  no digest, which stopped being true in T5; the table now has the three answers a file can
  get and the prose says a path being here is not a path being unchanged.

- **L11 (verification, and what it found).** The independent pass ran over
  `8fbc87d..e06213d` and did not bless the branch: it returned "not safe to put in
  front of a maintainer yet", with a blocker and five smaller findings. This is the
  entry that matters most in this document, because the blocker was in the one place
  this branch's own tests could not see:

  - **Blocker (S10, S4 not met).** A failed *interactive* step writes no record. The
    interactive steps (`deps`, `homebrew`, `terminal`, `wslconfig`, `setshell`) report
    through `execFinishedMsg`, whose error branch went to the error screen and returned
    without `recordRun`; `recordRun` was only reached from the other failure path, from
    completion, and from `--non-interactive`. Every run whose changes are certainly on
    the machine -- the one that stopped halfway -- could therefore leave no record of
    what it had already changed, `--verify` answered against the previous record, and
    `--revert` would have reverted the wrong run. Fixed by `637c916`, with the RED
    reproduced first: `TestAFailedInteractiveStepStillWritesTheRecord` failed with "an
    interactive step failure left no record although the run had already rewritten
    …/.zshrc".
  - **False success, record with no digests (S4).** A legacy record names paths and says
    nothing about what changed in them; the revert walked its empty change list and
    reported that everything already held what it held before the run. Now each entry
    is `unrevertible` with the reason and the command that can put the whole entry back,
    so the report is incomplete and the command exits 1.
  - **False success, restore with a damaged manifest (S9).** The restore listed the
    files the directory holds, so a manifest entry whose copy is gone was not listed at
    all: "0 entr(ies) are back", exit 0. Now it is a `failed` entry with its reason and
    the exit status follows; an omission the run itself recorded is listed as `skipped`
    with that run's reason.
  - **Two smaller ones in the same place.** The revert's header counted this run's plan
    ("changed 0 file(s)" on a second run) and now counts the record's own files; and the
    content comparison of `--verify` is documented as a content comparison, because a
    path that is now a link to matching bytes reads as holding what the run left -- the
    record keeps digests and not shape. That one is a documented limit rather than a
    code change: the honest fix would need the record to carry the shape, which is a
    format change the branch has no reason to make for this.
  - Fixed in `1b52dfe` and `637c916`, both after RED tests of my own; `./internal/system/`,
    `./cmd/...` and the whole `internal/tui` package are green afterwards (the four
    Ghostty tests fail in this environment and fail identically at the base commit
    `8fbc87d`, which was checked rather than assumed).
  - **Met without reserve, per the same pass:** S1 (no path removes a destination before
    its replacement exists: FIFO and unreadable-source probes both left the destination
    byte-unchanged), S3, S5, S6, S7, S8, S9's swap semantics, S11's exit statuses.
  - **What the pass could not verify**, and no test here claims otherwise: a real
    install (no network, no system packages), the interface in a real terminal, and a
    race in `CreateBackup`. It also reported that this document changed under it while
    it ran, which was true: the log entries above were being written by the parent
    session during the pass. Its own copies of what it read are in its report.
