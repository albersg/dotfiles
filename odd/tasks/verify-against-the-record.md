# Verify the machine against the record

## What the user asked for

> "El verify mételo."

A tool that writes into a person's home directory should be able to answer two questions: what
did it do, and is the machine still as it left it. Today it answers neither from a command
line, and only one half of it can be undone.

## What already exists, measured

**The record is complete and authoritative.** `installer/internal/tui/state.go`:

```go
// installEntry is one configuration path and what the run did to it. Backup names the
// copy saved before a replacement or existence check, when the backup step took one; it
// is empty when no copy was made. The record never infers a backup: an entry that names
// one names a file that is there.
type installEntry struct {
	Key    string        `json:"key"`
	Path   string        `json:"path"`
	Action installAction `json:"action"`
	Backup string        `json:"backup,omitempty"`
}
```

and its own comment on `lastInstall`:

> "Entries is **the authoritative list**: one record per configuration path, each naming its
> outcome. Files is the **legacy field** and keeps a reader that predates Entries working."

So the installer already writes, per run, what it did to each path it names, with the action and
any backup it took. `legacyEntries` upgrades an older record (one with only `Files`) on read.

**What is missing is the reader.** Nothing consults `Entries` to compare the machine against
it. The state panel shows the last install; no command checks it.

**And the other half of the tool can be undone while this one cannot**: `theme.json` records
the exact bytes each file held before a theme change, "which is what makes the change
reversible byte-for-byte" (`installer.go:295`), while `last-install.json` on the machine this
was measured on is 99 bytes with `files: null` — an old record, written before `Entries`
existed.

## The shape

**`--verify`** reads the record and reports, for each entry, whether the machine still matches
what the record says. Exit codes follow the convention `--check-update` already set: 0 when
everything matches, 1 when something does not, 2 when the record could not be read at all.

**What each entry lets `--verify` know, and what it does not:**

| the entry names | the honest check |
|---|---|
| `Path` | whether the file is **there** |
| `Action` | what the run did, so absence can be judged against it rather than reported flat |
| `Backup` | whether the backup it recorded **still exists**, which is what makes a rollback possible |
| — | **it does not name a hash**, so "the file changed since" **cannot be proven** |

That last row is the decision this task turns on, and it is the reason for writing the
document. Two answers:

1. **Report what the record supports**: presence, the action, and the backup. Say plainly that
   a file's contents cannot be judged from the record, rather than implying they were.
2. **Make the record support more**: add a digest of what the run wrote to `installEntry`, so a
   later verification can say "this file is not what the install left". That is a change to what
   every run writes, it costs a hash of every written file, and it cannot speak for records
   written before it — they would verify as "presence only".

**Prefer 1, and say why**, unless the case for 2 is concrete: a verification that reports what
it cannot know is worth more than one that appears to know more and cannot say which records it
is guessing about. Whichever is chosen, the output must distinguish **"this file is not here"**
from **"this file is here and the record cannot tell you whether it is the right one"**.

## Tasks

- [ ] **T10a — `--verify` reads the record and reports per entry.** Presence, the action, and
  the backup, with the three exit codes. A machine with no record is not a failure: it is
  "nothing was installed here", and it says so.
- [ ] **T10b — The report says what it cannot know.** No sentence claims a file matches the
  install when the record holds no digest. The distinction between absent and unverifiable is
  visible per line, not buried in a footer.
- [ ] **T10c — The guards.** A record naming a path that is there, one naming a path that is
  gone, one with a backup that is gone, a record that cannot be parsed, and no record at all:
  each asserted on the exit code and on the line the user reads.
- [ ] **T10d — The docs.** `--help` gains the flag in the same voice as `--check-update`, and
  the manual says what verification can and cannot tell.
