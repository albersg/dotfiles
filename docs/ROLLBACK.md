# Rollback Procedures

## Installer Rollback

The dotfiles installer automatically backs up existing configurations before overwriting them.

### Restore from Backup

```bash
# List available backups
dotfiles restore --list

# Restore a specific backup
dotfiles restore --backup 2026-07-29T12-00-00

# Restore the most recent backup
dotfiles restore --latest
```

Backups are stored in `~/.dotfiles-backup/` by default.

## Repository Rollback

### Revert to Pre-Downstream State

The original `albersg/dotfiles` (pre-migration) is tagged as `archive/pre-downstream`:

```bash
git checkout archive/pre-downstream
# Or to overwrite main entirely:
git checkout main
git reset --hard archive/pre-downstream
git push --force origin main
```

### Rollback a Release

```bash
# Checkout the previous version
git checkout v0.0.1  # Adjust tag as needed

# Build from that version
cd installer && go build ./cmd/dotfiles
```

### Uninstall

```bash
# Remove all symlinks (manual)
rm ~/.zshrc ~/.config/nvim ~/.config/kitty  # etc.

# Or use the installer's uninstall mode
dotfiles uninstall
```

## Upstream Sync Rollback

If an upstream sync introduces issues:

```bash
# Revert the merge commit
git revert -m 1 <merge-commit-hash>

# Or reset if the merge hasn't been merged to main
git reset --hard HEAD~1
```

## OpenCode Config Recovery

The personal OpenCode configuration was preserved from the original repository. If it was lost during migration:

1. The original config is in `stow/opencode/` in the `archive/pre-downstream` tag
2. Restore it:
   ```bash
   git checkout archive/pre-downstream -- stow/opencode/
   ```

## Manual Backup Location

The installer stores backups at:

| OS | Backup Path |
|----|-------------|
| macOS | `~/.dotfiles-backup/` |
| Linux | `~/.dotfiles-backup/` |
| WSL2 | `~/.dotfiles-backup/` |
| Termux | `~/storage/shared/dotfiles-backup/` |
