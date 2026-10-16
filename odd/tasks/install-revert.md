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

- **L12 (the verification caught a regression the fix itself introduced).** This is the
  entry to read before touching the revert again. The second pass (which had to be
  launched twice: the first launch reported a task id that never appeared in the task
  list, so nothing had been checked) closed four of the five findings and re-opened the
  second one as a blocker -- not in the original work, but in `1b52dfe`, the fix for it:

  - `1b52dfe` recognised a legacy record by its *shape*: entries and no per-file changes.
    That is also the shape a modern completed run writes when it changed nothing (entries
    with actions `found`/`backed-up`, no changes, empty files), so the machine's most
    ordinary state -- a second run that found everything already in place -- became
    "everything is unrevertible, exit 1". The pass reported it with the probe output:
    `the revert could not put back 1 of 1 files: …/.zshrc: the record predates per-file
    digests`. My own RED reproduced it before I touched the code.
  - The way back the legacy message printed was `--restore=<BackupDir>/key`, a *file*:
    `--restore` takes the directory, so the printed command exited 2. The reason now names
    the directory, which is the copy's parent by the writer's own convention for a file
    key and a directory key alike.
  - A record's digests and its copies have to describe the same moment. A retry after a
    failure starts a new run, but the baseline the record compared against was the failed
    attempt's, so the retry recorded a "before" it never saw: the pass logged
    `before=0270da4…` and my test failed naming *the same digest*, which is what made it
    clear the two were the same defect. `beginInstallFromReview` now clears both
    baselines, and `captureRunBaseline` (extracted from `executeStep`, which still fills
    only what is empty) takes the new run's own.
  - Two smaller answers: a restore whose manifest names an entry the directory no longer
    holds said "failed for 1 of 0 entries", and a `--dry-run` of such a backup exited 0,
    so a script could not tell a backup that can go back from one that cannot. The account
    counts what the manifest says should be there, and the dry run of a plan with a failed
    entry exits 1.
  - Two limits are now stated instead of left to be found: an install stopped with
    `ctrl+c` leaves no record (nothing failed; the backup it had already taken before its
    first write is still the way back), and the baseline a record keeps belongs to its own
    run.
  - **The lesson, which is the reason this entry exists.** A fix for a false *success* was
    checked against records that were broken and not against records that were fine: the
    legacy shape was inferred, and the shape it was inferred from is one the current writer
    also produces. What a reader needs is not a shape but a fact -- the record now carries
    an explicit marker for "read from the older shape" (unserialized, set where the fold
    happens, in both readers). Any future change to what the revert believes about a
    record has to answer that question first: which *state* is this record describing, and
    what evidence says so.
  - Fixed in `603a90a`, after RED tests of my own for each behaviour; the whole `tui`
    package is green again with the four environment-only Ghostty tests skipped.

- **L13 (the released build's record, and the class behind all three passes).** The third pass
  closed four of its six points and re-opened the blocker, on a shape neither of my fixes had
  covered: the record written by the *base* build (`origin/main`, the released installer) has
  `Entries` with actions and copies plus `Files`, and **no `Changes` field at all** -- it
  predates it. `1b52dfe` caught that shape by accident (it keyed on "entries and no changes");
  `603a90a` replaced that with a marker for the *older* shape (`Files` with no `Entries`), and
  therefore missed it. The result was the same false success both earlier passes had found, on
  the record every installed machine actually holds: `--revert` said the run changed nothing,
  exit 0, with every file the run had changed left in place.
  - The fix is the one the previous two were avoiding: the decision is **read from what the
    record says**, not from the shape it has. No per-file changes and the record names paths
    the run wrote (`Files` non-empty, or an entry whose action is `replaced`/`created`) -> this
    command cannot act on it file by file: every named path is `unrevertible`, with the backup
    directory taken from the entry's own `Backup` and a `--restore=<that directory>` that works.
    No per-file changes and no named path -> the record says the run changed nothing. An entry
    the backup step only copied or found is not a path the run wrote: that step copies what a
    run *may* write.
  - The interim `legacy` marker is gone, and with it L12's claim that the record carries one:
    the marker was itself a shape test, which is the mistake it had been introduced to fix.
  - The test that closes the class is a table of every record shape this repository has ever
    written -- current with changes, current without, current without but with copies, released
    with `replaced`, released with `created`, released without changes, the oldest `Files`-only
    shape, and an empty record -- each with the answer it earns, and each unrevertible case
    checked against the directory it names really being one `--restore` accepts.
    `TestRevertOnAReleasedBuildRecordSaysItCannotActOnIt` pins the same shape through the
    command line.
  - Two limits that were only prose now have pins: an install stopped with `ctrl+c` leaves no
    record (and a run that takes a backup takes it first, on each of the three systems), and the
    copy a retried run records holds the state that run found.
  - One of my own assertions was wrong and the test said so: I had pinned "the first step is
    backup", and the first step of a run that takes no backup is the checkout step. The
    invariant is the conditional one.
  - `docs/ROLLBACK.md` and `--help` name the released-build case, because it is the one every
    machine that already ran an installer will meet.

- **L14 (the fourth pass: the class, not the shape, and the reproduction with the real
  writer).** The pass closed points 1, 3, 4 and 5 and re-opened the blocker with the strongest
  evidence of the four passes: it did not fabricate a record, it drove the run's own writer.
  A record makes its claim in two halves -- the entries (and the `Files` list) say which paths
  the run wrote; the per-file `Changes` say what changed inside them -- and only the second
  half carries a before-state. The index behind it steps over anything that is not a regular
  file (`configFileIndex`'s `!entry.Type().IsRegular()`), while the fingerprint an entry is
  decided by walks the whole tree. A configuration directory holding a symlink is the ordinary
  case: the run retargets the link, the entry says the key was written, no change names the
  link -- and the revert, reading only the changes, reported the machine back with the link
  still pointing where the run left it. The same defect reached it through records that name
  one path in `Changes` and another as written (exit 0, file untouched).
  - The fix cross-checks both halves: a path the record says the run wrote is accounted for by
    a change naming exactly it, by the changes under it when it is gone, or -- for a directory
    -- by changes inside it plus nothing inside of a kind the index steps over that no change
    names. Everything else is reported, with the exact path and, when the record names one, the
    backup directory that puts the whole entry back.
  - The run's own copy is evidence: a link inside a written key that still matches the copy the
    run took is a link the run did not touch. Without that, the fix would have turned every
    configuration directory holding a link into a false alarm, and a rule that fixes a false
    success by inventing a false failure is not a fix. Both directions are pinned by
    `TestALinkTheRunLeftAloneDoesNotStopTheRevertFromBeingComplete`.
  - `TestARevertDoesNotCallItselfCompleteWithALinkTheRecordDoesNotName` is the pass's own
    scenario through the real writer, and `TestAVerticalRecordWithPartialEvidenceIsNotReported
    AsReverted` is the partial shape through the command line. Both were RED first.
  - Two of the new tests were wrong before they were right, and their failures said so: the
    first passed for the wrong reason (with no copy, the file's own failure masked the link),
    and the second asserted on `RevertReport.Complete()` when the contract that decides the
    exit status is the error. They now assert that both agree.
  - The pass's minor point is corrected: `docs/ROLLBACK.md` and `--help` promised a backup
    directory in a case (`created`, no copy) where the record has none; both now say that the
    report names a directory only when the record names one.
  - **Named follow-up, not omitted**: this rule *reports* what the per-file record cannot name;
    it does not yet *act* on a non-regular entry. Extending the index to symlinks and to link
    targets would let the revert put one back instead of reporting it -- and would close the
    advisory this audit recorded earlier (a link to matching bytes reading as "still holds"),
    because a link would be compared by its target rather than read through. That is the next
    step for this rule, deliberately not taken in this branch: it changes the meaning of the
    digests already stored in records written by earlier builds.

- **L15 (the hole I found myself, after committing the fix).** Re-reading the coverage rule
  against the exit it produces rather than against the tests it satisfies: the rule answered
  "the run did not touch this link" from the link's target text matching the run's copy, which
  is half the question. What a link *reaches* can be a tree the record never walked -- a
  checkout, a shared directory, a cache -- and a write on the other side of it is invisible to
  both halves of the record: the per-file index steps over non-regular entries and the walk
  does not descend through links. Reporting a machine back on that strength is the same false
  success in a quieter voice.
  - The rule now asks two questions about a link inside a written key: whether the target it
    holds differs from the target the run's copy holds (a link the run moved, which the record
    does not list), and where that target resolves to (a path outside every path the record
    says the run wrote is somewhere this run may have written with nothing on the record's
    side). A link into the covered tree keeps the ordinary answer, because what it reaches is
    recorded file by file -- so a configuration directory of links to its own tree stays quiet.
  - **My own new tests caught a defect in the message**: it named the key, the copy's target and
    the machine's target, and left the reader without the path inside the key. A report that
    does not name the path it cannot account for is not actionable, and now every one of these
    answers names it.
  - Full gate after the change: `go test ./... -count=1 -skip Ghostty` green (system, tui,
    trainer). Commits `9c8a89e` (cross-check), `565ccb8` (reach).
  - The fifth independent pass is the gate for the handoff: it reviews the coverage rule and the
    reach rule together, and is asked for false negatives (reporting a machine it can put back)
    as hard as for false positives -- five passes have each found one more defect in the fix
    before, and a rule that fixes a false success by inventing a false alarm is not a fix.

- **L16 (the fifth pass: five findings, and the writer among them).** The pass attacked both
  directions and closed points 4 and 5; points 1, 2, 3 and 6 came back open. The largest finding
  is not in the rule at all: a configuration key whose path is a **symlink to a directory** --
  `~/.config/nvim -> ~/repo/nvim`, which is what a dotfiles checkout looks like -- made the
  run's own writer record `entries=[{nvim found}]` and `changes=[]`, because `configFileIndex`
  stopped at the link and `fingerprintPath` walked the link itself. `dotfiles --revert` then
  exited 0 with every file the run wrote through that key still holding what it left. A silent
  false success on an ordinary layout, predating the reviewed delta, and not fixable inside the
  rule: the record never named the path.
  - The writer now reads through a linked key, naming the paths a person sees (`namedPath`), so
    what the run wrote inside it is recorded. The backup keeps the link and not what it reaches,
    so a revert of such a key reports that it holds no copy of the file -- the truth, with the
    tree the link points at as the way back. That is the audit's N4 boundary, now surfaced per
    path instead of hidden.
  - **A false alarm of my own making** (`565ccb8`): a link into a key the record says the run
    did not write was reported, although the record's own word is that the run left that key
    alone. The reach is now covered by *every* entry the record names, not only the written
    ones.
  - A change naming the entry's own directory short-circuited the walk; a directory is not a
    file a change carries a before-state for, so the shortcut now applies only to non-directories.
  - A path the record says the run created and that is not there is where the machine already
    was: accounted for, not reported.
  - **"Inside a path the record wrote" was a claim, not a check.** For a tree the run wrote,
    every file inside the reach must now hold what the run's copy of it holds or be named by a
    change. The copy is the before-state; the record's silence about a file that disagrees with
    it is evidence of nothing.
  - A sixth, found by my own tests while fixing: undoing a path the record says the run created
    read an unreadable path (a directory) as an absent one, so a directory declared itself
    already gone. It is reported now: putting a directory back means removing a tree.
  - `revert_probes_test.go` holds probes for all six, RED first; the subcase that must NOT alarm
    (a file inside a reached tree matching its copy) is pinned beside the one that must.
  - Accepted and documented rather than fixed: a named pipe or socket inside a written key is
    reported, because the copy of a key cannot hold that kind and the record cannot say whether
    the run made it or found it.
  - Full gate after the six: green (system, tui, trainer).

- **L17 (the sixth pass: my own fix, caught turning a false success into a false alarm).** The
  pass closed points 3, 5 and 6 and five of the six fixes, and reopened the first: closing the
  writer defect (L16) made the writer mark a key that is a link as written and name the files
  inside it, and `firstUnaccountedFor` then decided whether that key was a tree with `os.Lstat`,
  which reports a link to a directory as "not a directory". Every key of that shape was reported
  as one this command cannot act on -- with the claim that it could not be put back -- even when
  the revert had put every change back: with `~/.config/nvim -> ~/repo/nvim` and a run that only
  created a file, `--revert` removed the file and still exited 1.
  - Fixed: the tree decision is `os.Stat` (it follows), and the walk inside a key follows the
    key's own link and names every path as a person sees it (`namedPath`), instead of walking an
    empty tree that starts at a link.
  - A second advisory of the same family, also mine: a link the run's copy of the key does not
    hold is one the run may have made, and is reported -- the per-file record never lists a link,
    so the copy is the only thing that can say one was already there. A dead link the key held
    before, with a copy holding the same dead target, stays quiet; both directions are pinned in
    `revert_symlinked_key_test.go`.
  - A third: `--non-interactive --dry-run` wrote its empty record over the one a real run left, so
    the next `--revert` reported that the run changed nothing while the machine held every file
    that run wrote -- this branch's own false success, arriving through the flag that promises to
    change nothing. A dry run records nothing now, held against a real record byte for byte.
  - A stale comment in `cmd/dotfiles/main.go` said the verify report never claims contents match
    because entries carry no digest; that describes the behaviour before T5, where `--verify`
    compares the digests the record does carry and says so per line. Corrected.
  - **Known limit, recorded rather than fixed**: the way-back block does not fit the review screen
    at 80x24 (the failure screen does show it), so at that size the screen where a person can
    still decide not to start says nothing about the way back. It is not merge-caused -- the same
    probe fails at `2c50ca1` -- and it is a layout question, not a correctness one. The seventh
    pass judges whether it must be closed before the handoff.
  - Full gate after the four fixes: green (system, tui, trainer).

- **L18 (the seventh pass: a rule that could not tell the difference, and the interface's own route).**
  The pass closed claims 1, 3 and 4 and found the round before's rule inventing the alarm this branch
  keeps producing: with no backup, a missing copy reported *every* link inside a written key, so a
  dead link the key already held failed a revert whose only change -- a file the run created -- had
  been put back correctly (`could not put back 1 of 2 files ... .config/nvim/stale, which the run's
  copy of the key does not hold`, exit 1; the same probe at the baseline was quiet and right).
  - Causality checked here, not taken on faith: `os.Symlink` appears only in
    `internal/system/backup.go` (`:217`, `:264`, the copy code), and `copyDirTree` walks with
    `filepath.Walk` + `os.Stat` and creates nothing. This installer never makes a link, so "the run
    may have made it" was unreachable and the rule could only produce alarms.
  - The rule now: **a missing copy is not evidence that the run wrote a path.** A link that reaches
    nothing is left alone; one that lands inside a path the record says the run did not write is left
    alone; one that lands outside every path the record names is reported; one that lands inside a
    written key is reported only when a copy exists and a file behind it neither matches that copy nor
    is named by a change. With no copy of the key, the files behind a link are judged as the same
    files are when the key is walked directly, by the changes the record carries -- the report must not
    depend on which route the reader took to a path.
  - A copy pointing somewhere else is still reported, and that direction is now pinned by a case a
    person can produce. The probe it replaces asserted "not complete and the link is present", which
    passed on any error, and its scenario was not producible by the writer at all.
  - The pass also found that the dry-run fix guarded only the scripted route: the interface reaches
    `recordRun` by three routes of its own, so a `--dry-run` through the interface still overwrote the
    last real record (919 bytes to 123). The rule now lives at the one place a run records,
    `recordRun`, and the scripted epilogue's copy of it was removed as duplication. Pinned by a test
    that finishes a dry run through the interface's own completion message and holds the last real
    record against it byte for byte.
  - The 80x24 review screen is **published as a named limit**, on the pass's recommendation and mine:
    the marker renders, the actionable lines do not, and the same sentence is on the failure screen
    and in `--revert`'s own error. It is presentation, not a command path; `docs/tui-installer.md`
    names the width instead of leaving it to be discovered.
  - Gate after the fixes: green (system, tui 208.8s, trainer).

- **L19 (the eighth pass: safe to publish, and what it left named).** The pass closed F1, F2 and F3
  with RED at `04f0d1b` and GREEN at HEAD, showed the two routes to the same file agreeing, showed all
  five dry-run routes leaving the record of a real run byte for byte while real runs still record what
  they did (including the failed steps), and answered the product question: the 80x24 review screen is
  publishable as a named limit, because it is presentation and the same way back is on the failure
  screen and in `--revert`'s own error.
  - It handed over its probes. Five became durable tests with their bodies verbatim (names and helper
    changed): the no-copy link judged the same by both routes, the modified file inside a linked key
    with no backup that must not be a silent success, the five dry-run routes, the real runs that must
    still record, and the stopped run that leaves the previous record alone. The 80x24 probe only
    logged, so its assertions are new: a probe that cannot fail is not a pin.
  - The probes that measured a pre-existing advisory were deliberately **not** kept: a test asserting
    one would pin a known false alarm as the contract. They are advisories instead, all measured at
    `04f0d1b` and at HEAD, so none is caused by this delta:
    - **A1** a fifo or socket inside a written key with no copy still reports and exits 1 for a run
      whose only change was a created file. It is the same shape F1 had, for another kind, and it is
      **left for a decision rather than fixed here**: the discriminator is not the one this round used
      (`copied == ""` conflates "the run took no backup" with "the copy ran and did not take that
      path", which are different facts), and the case is rare. The rule the code holds today is the
      one the docs now describe.
    - **A2** a no-copy link reaching outside every named path is still reported, which is rule (ii) as
      stated, and there the alarm guards a real invisibility: the run may have written through that
      link where the record names nothing.
    - **A3** with a copy holding a regular file where the machine holds a link, the dead and
      inside-a-written-tree cases are silent: that is the intended leniency of F1, now pinned in both
      directions.
  - Two document sentences the pass showed to be inexact are corrected: the 80x24 block shows the
    marker and its first row and does not show the command, and the link rules are a bullet of their
    own, with the no-copy exception named instead of leaving the previous bullet's sentences about a
    key that is itself a link inside them.
  - No behavioural code changed after the verdict. What changed is documentation and test files the
    pass itself wrote and ran at HEAD, and the full gate ran green over the final tree.
