# Rollback Procedures

## Installer Rollback

An install leaves two things behind so that the machine can be put back:

- **`~/.dotfiles-backup-<timestamp>/`** — a copy of every configuration the run was about
  to replace, taken before anything was written. Each entry is named after the
  configuration it replaces (`zsh` is `~/.zshrc`, `nvim` is `~/.config/nvim`), and the
  directory carries a `backup.json` manifest that names every entry it took, every entry
  it could not copy and why, and the entries it deliberately left out. A symlink is
  preserved as a symlink; an entry that cannot be copied is recorded rather than
  dropped; and one bad entry no longer costs the copies already made.
- **`~/.local/state/dotfiles/last-install.json`** — the record of the run (or
  `$XDG_STATE_HOME/dotfiles/last-install.json` when that variable is set). It names what
  the run did to each configuration path and, file by file, the digest each file held
  before the run, the digest it held after, and the copy in the backup that puts it
  back. It is written by a run that failed as well as by one that finished, and by
  `--non-interactive` as well as by the interface, because a run that stopped halfway is
  exactly the run whose changes are already on the machine.

### Putting the last install back

```bash
dotfiles --revert --dry-run   # read the plan: which files, and what would happen to each
dotfiles --revert             # do it
```

`--revert` reads the record and undoes it one file at a time: a file the run modified or
removed is copied back out of the backup, and a file the run created is removed. Every
file it puts back is then compared against the digest the run recorded before it wrote
anything, so the command reports a measurement rather than an attempt.

- **It is idempotent.** A file already in the state the revert restores is reported as
  already there, and running the command twice does not change the machine twice.
- **It keeps what it replaces.** The configuration it is about to write is copied into a
  backup of its own first, so a revert can be undone by the same command. The report
  names that directory.
- **It says what it cannot do.** A file the backup holds no copy of is reported as
  `unrevertible`, never skipped, and a file that changed again since the run is reverted
  and named. A record written before the installer recorded files one by one -- what every
  machine that ran a released version holds -- does not say what changed inside each path,
  so this command cannot undo it file by file: it reports each path as one it cannot act
  on, rather than reporting success on a machine whose files it never restored. The same
  check covers a path it *does* name as written but cannot vouch for file by file, which
  is what a configuration directory holding a symlink looks like: the per-file record
  lists regular files, so the link is reported unless the copy the run took of it matches
  where it points now. When the record names a backup of that path, the report names the
  directory that puts the whole entry back; when it does not, it says so.
- **Exit status:** `0` when the machine is back where the run found it (or already was),
  `1` when a file could not be put back, `2` when there is no record to revert. A run that
  changed no file is `0` and says so.
- **The record describes the run it belongs to.** A retry after a failure starts a new run,
  whose record names the state that run found and whose backup holds that same state, so a
  record's two digests and its copies agree with each other. Undoing further back -- past a
  retry -- is what `--restore` is for, and every backup directory a machine ever took is
  still there.
- **A run that was stopped is not a run that failed.** An install interrupted with `ctrl+c`
  leaves no record, because nothing failed: the command that stopped it is the one that
  knows. What it wrote before stopping is still undone from the other direction -- the backup
  taken before its first write is on disk, `ls -d ~/.dotfiles-backup-*` names it, and
  `dotfiles --restore=<that directory>` puts it back.

### Putting one backup directory back

```bash
dotfiles --restore ~/.dotfiles-backup-2026-10-16-120000 --dry-run
dotfiles --restore ~/.dotfiles-backup-2026-10-16-120000
```

The same operation is offered as **Restore from Backup** in the main menu. Three rules
make it safe to run on a machine whose configuration you still care about:

- the replacement is copied beside the destination and only a complete copy moves into
  place, so a destination is never absent because a copy failed halfway, and a swap that
  cannot finish restores what was there;
- the state it replaces is copied into a backup of its own first, so the restore is
  itself undoable;
- a directory this program did not write is refused rather than trusted.

The report accounts for itself entry by entry: `restored`; `skipped` with the reason, for a
key no configuration path names or one the run that took the backup recorded it could not
copy; and `failed` with the reason, for an entry the manifest names whose copy is gone or a
replacement that could not be put in place. A `failed` entry is never reported as a machine
that is back: the command names it and exits. **Exit status:** `0` when every entry went
back, `1` when an entry could not be placed, `2` when the directory is not a backup this
program wrote or could not be read. `--dry-run` prints the plan and changes nothing; it
exits `0` when that plan would put everything back and `1` when it names an entry the backup
does not hold, so a script can ask the question before it asks the restore.

### Checking what a machine is

```bash
dotfiles --verify
```

`--verify` prints every path, backup and file the record names, with the action the run
took. For every file the run changed it compares the content against the digest the run
recorded, so a file is reported as `here` (still holding what the run left), `changed` (no
longer holding it, with the two digests), `unknown` (the file could not be read) or
`missing`. Where the record carries no digest — one written before it kept them, or a path
the run did not write — the report says the path is here and unverifiable rather than
claiming it matches. **Exit status:** `0` when the machine is what that run left, `1` when
a path or backup is gone or a file no longer holds what the run left, `2` when a record is
present and could not be read. A machine with no record exits `0`.

The digests the record keeps are content digests, taken by reading the file. A path that is
now a symlink to content that matches is therefore reported as holding what the run left:
the record says what the bytes are, not what shape the path had, and the report does not
claim the second from the first.

### Restoring without the installer

A backup is an ordinary directory, which is what matters when the installer cannot run or
when the machine is not the one that was installed. List the backup before copying
anything back — and read its `backup.json` first when it has one, because that is where
the entries it could not copy are named:

```bash
# What backups exist, newest last
ls -d ~/.dotfiles-backup-*

# What this one holds, and what it could not copy
cat ~/.dotfiles-backup-2026-09-19-153000/backup.json

# A single file goes back where it came from: the entry "zsh" was ~/.zshrc
cp ~/.dotfiles-backup-2026-09-19-153000/zsh ~/.zshrc

# A directory keeps its shape, so it goes back to its original location:
# the entry "nvim" was ~/.config/nvim
cp -r ~/.dotfiles-backup-2026-09-19-153000/nvim ~/.config/
```

A backup written before the manifest existed has no `backup.json`; it is still a valid
backup, and its entries are still named after the configurations they replaced. When the
entry's target is still there, copy the entry over it rather than removing it first:
losing your only copy of a file to a copy that fails is the failure this whole path
exists to avoid.

### What no command undoes

The reversal above covers the configuration files this program writes. It does not cover,
and no command here reverses:

- packages installed by the platform package manager (`apt`, `brew`, `pacman`, `dnf`,
  `pkg`);
- the Xcode command line tools;
- the default shell recorded for your account (`/etc/shells` and the login shell);
- anything installed by a script fetched from the internet (Claude Code, OpenCode);
- files written outside the configuration paths the backup covers: fonts, caches,
  `~/.local/bin`, `~/.pi/agent/skills`, `~/.cargo`, `~/.rustup`, `~/.tmux`, the repository
  files a package manager adds for itself (`/etc/yum.repos.d`, `/etc/apt/sources.list.d`),
  and, on WSL, the Windows-side `.wslconfig`, its editor settings and its font
  registration.

Those are named on the failure screen and printed by `dotfiles --revert`, so the boundary
is visible at the moment it matters instead of being discovered later.

### Uninstalling

There is no uninstall subcommand. Undoing an installation has two halves. The first is
`dotfiles --revert`, which puts back every file the record names and removes the files the
run created. The second is what a revert deliberately leaves alone: the packages, the
shell and the files outside the backed-up configuration paths listed above, which are
removed by whatever installed them (`brew uninstall dotfiles` on the Homebrew path, the
platform package manager for the rest).

If the record is gone, the backup is what tells your files from the ones the run created,
so list it first and keep it in view:

```bash
# What the most recent backup holds, one entry per configuration it replaced
ls ~/.dotfiles-backup-2026-09-19-153000/

# Put every entry back where it came from, as in the section above
cp ~/.dotfiles-backup-2026-09-19-153000/zsh ~/.zshrc
cp -r ~/.dotfiles-backup-2026-09-19-153000/nvim ~/.config/
```

A path present in the backup is the user's own file and must not be deleted after being
restored. The entries are named after the configuration each one replaced, so match an
entry to its path the way the copy above does: the entry `zsh` is `~/.zshrc`, the entry
`gitconfig` is `~/.gitconfig`, and so on. Delete only the paths the backup does not
contain, because only those never existed before the installer ran:

```bash
# Configuration files the installer writes
rm ~/.zshrc ~/.p10k.zsh ~/.tmux.conf ~/.wezterm.lua ~/.gitconfig \
   ~/.config/starship.toml  # etc.

# Configuration directories the installer writes
rm -rf ~/.config/nvim ~/.config/fish ~/.config/nushell ~/.config/zellij \
       ~/.config/herdr ~/.config/alacritty ~/.config/kitty \
       ~/.config/ghostty ~/.config/bat ~/.oh-my-zsh  # etc.
```

The installer itself is removed by whatever installed it: `brew uninstall dotfiles` on
the Homebrew path, or deleting the downloaded file otherwise.

## Repository Rollback

### Rollback a Release

```bash
# Check out the released version
git checkout v0.1.0

# Build the installer from that version
cd installer && go build -o dotfiles ./cmd/dotfiles
```

## Upstream Sync Rollback

If an upstream sync introduces issues:

```bash
# Revert the merge commit
git revert -m 1 <merge-commit-hash>

# Or reset if the merge hasn't been merged to main
git reset --hard HEAD~1
```

## Backup Location

`GetBackupDir()` derives the backup path from `$HOME` with no per-platform branch, so the
location is the same everywhere, Termux and WSL included:

```
$HOME/.dotfiles-backup-<timestamp>/
```

Two backups taken inside the same second keep the first one: the second gets a `-2`
suffix rather than sharing the directory, so a failed run cannot take the copies the first
one made with it.
