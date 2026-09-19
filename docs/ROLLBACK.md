# Rollback Procedures

## Installer Rollback

Before it changes anything, the installer copies each configuration it is about to
replace into `~/.dotfiles-backup-<timestamp>/`. Those backups are plain copies, not
archives, so restoring is a copy back.

### Restoring through the installer

Run `dotfiles`, choose `Restore from Backup` from the main menu, pick the backup and
confirm. See the [TUI Installer Guide](tui-installer.md#backup--restore) for what a
backup contains.

### Restoring without the installer

A backup is an ordinary directory, which is what matters when the installer cannot run.
Each entry inside it is named after the configuration it replaced, so list the backup
before copying anything back:

```bash
# What backups exist, newest last
ls -d ~/.dotfiles-backup-*

# A single file goes back where it came from: the entry "zsh" was ~/.zshrc
cp ~/.dotfiles-backup-2026-09-19-153000/zsh ~/.zshrc

# A directory keeps its shape, so it goes back to its original location:
# the entry "nvim" was ~/.config/nvim
cp -r ~/.dotfiles-backup-2026-09-19-153000/nvim ~/.config/
```

There is no restore flag and no restore subcommand, so this copy is the manual path
rather than a fallback for a broken option.

### Uninstalling

There is no uninstall subcommand. Undoing an installation has two halves, and the
removal step applies only to paths the backup does not contain: those are the files the
installer created from nothing, so there is nothing of yours to restore for them. The
backup is what tells the two apart, so list it first and keep it in view before running
anything:

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
