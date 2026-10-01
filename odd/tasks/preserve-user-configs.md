# Preserve user shell configurations

The installer addresses two destructive reports: issue #196 for Fish and issue #206 for Zsh. Replacing an existing unmarked shell startup file could discard user-written settings. The ownership decision is a standalone marker line (`# dotfiles-managed-config: fish` or `# dotfiles-managed-config: zsh`): marked files belong to dotfiles and may be replaced without another copy; unmarked files are preserved byte-for-byte as dated sourced drop-ins before replacement.

The drop-in locations must load after dotfiles' managed settings so the user's choices win. Zsh uses `~/.zshrc.d/*.zsh` at the end of `.zshrc`. Fish uses `~/.config/fish/dotfiles.d/dotfiles-user-config-*.fish`, sourced at the very end of `config.fish`; Fish's native `conf.d` is read before `config.fish`, which would let the managed file override the user's settings.

## Tasks and evidence

| Task | Outcome | Commit |
|---|---|---|
| 1. Couple preservation and replacement at a tested system seam for Fish and Zsh | Added `ReplaceUserConfig`; the behavior test covers unmarked preservation, exact bytes, returned location, replacement content, and marker-managed replacement without a drop-in. The test was run with the preservation call temporarily removed and failed on the missing preserved config. | 3905437 (tasks 2 and 3) / 6c1b254 (task 1) |
| 2. Make Fish user drop-ins load last and update user-facing documentation | Fish now saves into `dotfiles.d`, sources those files at the end of `config.fish`, and reports the right path. Manual-installation and TUI docs plus the changelog describe the new path and precedence. | 3905437 (tasks 2 and 3) / 6c1b254 (task 1) |
| 3. Guard Fish merge-copy retention | Installer-level regression test runs `stepInstallShell`, verifies preserved bytes survive the merge, and checks the shipped loader. Temporarily switching the installer to `CopyDirPruned` made the test fail because the drop-in disappeared. | 3905437 (tasks 2 and 3) / 6c1b254 (task 1) |
| 4. Record outcomes and remaining work | This document records the reports, marker/drop-in decision, precedence rationale, implementation outcomes, and validation evidence. | 3905437 (tasks 2 and 3) / 6c1b254 (task 1) |

## Validation

- `cd installer && go test ./... -count=1`: 3,861 tests passed across 4 packages.
- `cd installer && go vet ./...`: passed.
- `gofmt -l installer/internal/system/exec.go installer/internal/system/exec_test.go installer/internal/tui/installer.go installer/internal/tui/installer_test.go`: no files listed.
- The preservation-removal mutation failed at the unmanaged case (`unmanaged user config was not preserved`); restoring the call passed both table cases.
- The pruning-copy mutation failed because the preserved drop-in no longer existed; restoring `CopyDir` passed the installer regression test.

## Remaining

- Product work is implemented. The only remaining repository-process work is for the transaction controller to create the requested work-unit commits and record their identities; this implementation writer cannot commit, push, or rewrite history.
- Fish is not installed in the validation environment, so runtime parsing of `config.fish` was not verified here.
