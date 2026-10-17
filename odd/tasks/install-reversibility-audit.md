# Install reversibility audit — read-only

Base: `origin/main` = `8fbc87d`. Worktree: `/home/albeertoo/my-things/dotfiles-install-audit`
(detached, no branch, nothing published). Every claim below carries `path:line` evidence, and
the ones marked **reproduced** were executed, not read.

## Method and provenance

- Three read-only explorations, one per slice: the 17 install steps, the backup/restore
  apparatus, the record/verify surface and the visible ways to undo.
- Six scratch probes in `installer/internal/system/zz_audit_*_probe_test.go`, run against a
  temporary `$HOME`. Four are diagnostic; **two assert and currently FAIL**, which is what
  turns "the code reads like this" into "the machine behaves like this".
- No production file was modified. Nothing was pushed.

## How an installation works today

`planFor` (`installer/internal/tui/model.go:1352`) builds an ordered list from
`stepExecutors` (`installer/internal/tui/installer.go:4504-4522`) — 17 steps: `backup`,
`clone`, `homebrew`, `deps`, `xcode`, `terminal`, `font`, `shell`, `wm`, `nvim`,
`agentskills`, `officecli`, `wslconfig`, `theme`, `cleanup`, `setshell` — and the steps run
sequentially, writing in place. The `backup` step is scheduled only when
`CreateBackup && ConfigCount > 0` (`model.go:1356`), and it runs before any step writes a
config path (`installer.go:4529-4540`).

## The two backup mechanisms (they do not know about each other)

1. **The config backup**, `~/.dotfiles-backup-<timestamp>/`, written by `CreateBackup`
   (`installer/internal/system/exec.go:700`), read back by `RestoreBackup` (`:757`), driven
   only from the TUI restore screens (`installer/internal/tui/update.go:3311`). It copies
   exactly what `ConfigPaths()` names (`exec.go:600-641`): 22 keys.
2. **The WSL-side `.bak-dotfiles-<stamp>` files**, written by the `wslconfig` step before it
   modifies the Windows profile `.wslconfig`, the Windows Terminal `settings.json` and
   `/etc/wsl.conf` (`installer/internal/tui/wsl.go:113-142`, `wslterminal.go:99-129`).
   Nothing in the program restores them.

## What a run records

`~/.local/state/dotfiles/last-install.json` (`state.go:40-121`), written by
`recordLastInstall` → `writeLastInstall` (`update.go:228`, `state.go:132`). Entries carry
`{key, path, action, backup}` with `action ∈ found|backed-up|replaced|created`
(`state.go:60-102`). **No digest, no size, no per-path timestamp.**

- Written **only** from the `installCompleteMsg` branch (`update.go:757`).
- The failure path returns to `ScreenError` first (`update.go:728-735`), so **a run that
  fails halfway writes no record at all**.
- `--non-interactive` never calls it: a grep over `cmd/` and `non_interactive.go` finds no
  call site.
- The pre-run content fingerprints (`state.go:279-334`) die with the process; nothing
  persists them, so `--verify` can never re-derive them.

## The reversibility surface

Exists: TUI "Restore from Backup" → confirm; "Delete this backup"; the `theme` picker's own
single-value undo (`installer.go:2129`); `docs/ROLLBACK.md`, which documents a **manual**
procedure (`cp` the entries back, then `rm` the paths the backup does not contain).

Does not exist: any `restore`, `undo`, `uninstall` or `revert` **flag or subcommand** (the
full flag set is in `installer/cmd/dotfiles/main.go:57-91`; `parseFlags` is a plain
`flag.Parse()`); any record-driven rollback (`InspectInstall` is read-only by construction,
`state.go:493-496`); any removal of the paths an install created.

## Defects

| # | Severity | Defect | Evidence |
|---|---|---|---|
| D1 | **critical — data loss** | `RestoreBackup` removes the destination *before* checking the source; an unstattable source is skipped and the function still returns `nil` | `exec.go:775` `os.RemoveAll(dstPath)` precedes `exec.go:778-781` `os.Stat(srcPath)` → `continue`; **reproduced: the user's only copy was deleted and the call reported success** |
| D2 | **critical — not a revert** | A restore never removes what the install created, because it iterates the backup directory, not the destination tree | `exec.go:761`; **reproduced: a file the install created survives the restore** |
| D3 | high — silent corruption | A symlink inside a backed-up directory is not preserved and not reported: a symlink to a directory becomes an empty directory, and `skipped` stays empty | `exec.go:504`, `exec.go:510-513`; **reproduced (failing probe)** |
| D4 | high — destructive abort | One entry at a top-level key that is not a regular file aborts the whole backup **and the deferred cleanup deletes the copies already made** | `exec.go:396-398`, `exec.go:706-711`; **reproduced (failing probe): the user's `.zshrc` copy was destroyed by an unrelated FIFO** |
| D5 | high — integrity | `skipped` is returned but only ever logged (`installer.go:4571-4576`); nothing writes it into the backup directory, its file list, or the record, so an incomplete backup looks complete. A restore of a directory whose socket was skipped wipes the whole destination and recreates nothing | `exec.go:510-513`, `exec.go:775`; no test |
| D6 | high — identity | `ListBackups` identifies a backup by the name prefix alone: no marker or manifest is ever written, so a foreign directory in `$HOME` is listed, offered as restorable **and offered for deletion**. `Timestamp` is the directory's mtime, not the name, so the "newest backup" row can be wrong | `exec.go:660-694`, `exec.go:686`, `update.go:3331`, `panels.go:1514`; **reproduced** |
| D7 | high — collision | The backup directory is named to the second; two runs inside one second share it, and a failure in the second run deletes the first run's backup | `exec.go:653-656`, `exec.go:583`, `exec.go:709`; **reproduced** |
| D8 | **critical for "it broke"** | A failed run leaves **no record** of what it changed, and `--verify` on such a machine reports "nothing was installed here" with exit 0 | `update.go:728-757`; verified by reading the only call site |
| D9 | high — recoverability UX | The failure screen never mentions the backup or the way back: the user gets "Step 'X' failed", `r` to retry or `Enter` to quit | `view.go:534-536`, `update.go:1471-1481`; `rg BackupDir view.go` finds nothing |
| D10 | medium — honest limit | `--verify` proves presence, never content; the code says so itself | `state.go:590-595`, `state.go:563-566` |
| D11 | medium — untracked rewrites | The `theme` step rewrites twelve configuration files in place, best-effort per file, and keeps **neither a record nor an undo** (the picker does write `theme.json`) | `installer.go:7942-8010` vs `installer.go:331` |
| D12 | medium — reachability | Restore is TUI-only, and the documented manual path (`docs/ROLLBACK.md`) is prose the user must execute by hand; it covers configuration only | `cmd/dotfiles/main.go:57-91`, `docs/ROLLBACK.md:9-67` |
| D13 | low | The confirm screen says "Restoring will overwrite your current configs!" while the code deletes the whole destination tree | `view.go:3664` vs `exec.go:775` |
| D14 | low | An entry whose key is absent from `ConfigPaths()` is silently ignored, and a key that was remapped between versions writes old content to the new path (the `wezterm` rename is the precedent) | `exec.go:762-768`, `exec.go:617-621` |

## Reproduce it

```
cd /home/albeertoo/my-things/dotfiles-install-audit/installer
go test ./internal/system/ -run TestAuditProbe -count=1 -v
```

`TestAuditProbe_InnerSymlinkToDirectorySurvivesAsEmpty` and
`TestAuditProbe_NonRegularEntryDestroysTheWholeBackup` fail today (two of the six probes; the
other four log the failure mode without failing, having been written as diagnostics first).

## Perimeter the backup does not cover

`~/.local/share/dotfiles/**` (themes, WSL template) · `~/.local/share/fonts/**` ·
`~/.cache/{starship,carapace,atuin,fontconfig,bat}` · `~/.local/share/atuin` ·
`~/.local/share/fnm/**` · `~/.cargo`, `~/.rustup` · `~/.termux/font.ttf` ·
`~/.local/bin/{officecli,herdr,win32yank.exe}` · `~/.pi/agent/skills/**` · `~/.tmux/**`
(only `~/.tmux.conf` is a key) · `~/.zshrc.d/**` · `~/.config/obsidian/**` ·
`~/.local/state/dotfiles/*.json` · `/etc/shells`, `/etc/wsl.conf` ·
`$PREFIX/etc/shells` (Termux) · `/usr/local/bin/alacritty`,
`/usr/share/applications/Alacritty.desktop` · `/etc/yum.repos.d` COPR files ·
Windows-side `.wslconfig`, `settings.json`, `AppData/.../Fonts/*`, HKCU font keys.

The installer's own guard enumerates 22 of these with a reason each
(`installer/internal/tui/install_paths_test.go:95-118`): the code knows it does not back them
up. `TestEveryHomePathTheInstallerWritesIsClassified` passes today, and it is a regex over
`filepath.Join(home|homeDir|os.Getenv("HOME"), …)` (`install_paths_test.go:88`), so a path
assembled any other way escapes it.

## Effects with no undo at all

Xcode CLT; every package manager install (apt/pacman/dnf/brew/`pkg`, including
`brew bundle install` with a 60-line Brewfile); the Homebrew installer itself (unpinned
upstream script); `dnf copr enable` repository files; rustup; the default-shell change
(`/etc/shells` append plus `usermod -s`/`chsh -s`); `/etc/wsl.conf` and the Windows-side
writes including HKCU font registration; the two unpinned `curl | bash` installers for
Claude Code and OpenCode; `fnm install --lts`; TPM's plugin checkout; `herdr plugin install`.

## Strengths worth keeping

The plan/executor agreement invariant tied to a map so it can be enumerated
(`installer.go:4504-4510`); the home-path classification guard (`install_paths_test.go`);
`officecli` and `agentskills` install through a staging directory and a rename
(`officecli.go:171-245`, `agent_skills.go:157-259`); the `wslconfig` step backs up before it
writes; unowned `.zshrc`/`fish` content is preserved under a timestamped name
(`system/exec.go:353-361`); `--dry-run` and `--skip` exist; `--verify` and `docs/ROLLBACK.md`
state their limits instead of overclaiming.

## Proposed scope (for the owner to choose)

- **N1 — report only.** This document. No code.
- **N2 — repair the reversibility core.** Make the backup trustworthy and the restore safe:
  verify before deleting, restore into a staging tree then swap, pre-restore snapshot of what
  is being overwritten, persist the skipped list and a manifest, mark backups as owned,
  collision-proof directory naming, honest confirm text; plus write the record on failure
  (D8) and point the failure screen at the backup (D9). RED tests for D1-D7, D13.
- **N3 — N2 plus a real revert.** Record a complete manifest of the run (created / modified /
  deleted, per-path digest, backup reference), and expose `--restore`/`--revert` with
  `--dry-run`, idempotent, with a round-trip test (install → revert → the tree compared
  byte-by-byte on a disposable `$HOME`), and upgrade `--verify` to compare content. This is
  what makes "volver al estado anterior" a command instead of a paragraph.
- **N4 — N3 plus the system perimeter.** Record and reverse what is reversible (brew/apt
  removals, the configured shell, `/etc/shells` lines, COPR files) and report explicitly
  what cannot be reversed (Xcode CLT, delegated `curl | bash` installers).

## Coordination constraint

`install*.go`, `update.go`, `view.go`, `model.go` and `panels.go` are exactly the files the
`feat/start-installation-flow` session has open in the main worktree. Any code work in N2-N4
must be isolated in a worktree and rebased after that branch lands, or sequenced behind it.
Publication is centralized in the coordinating session.
