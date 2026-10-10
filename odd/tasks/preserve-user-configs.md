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

---

# Every replaced home file is protected, and copies keep their mode (#238, #241)

Two further destructive reports landed against the same copy routine the preservation seam calls.
`system.CopyFile` is the single writer behind `CopyDir`, `CopyDirReport`, `CopyDirPruned` and
`installConfigDir`, so a defect there reaches every configuration the installer installs.

## #238 - a replaced file that `ConfigPaths()` does not name is replaced silently

The backup step copies only what `ConfigPaths()` lists, and the overwrite count and the
last-install record are built from the same map, so a path the map omits is replaced with no copy
and with nothing on screen or on disk saying it happened. `~/.zshenv` was exactly that: the shell
step copied the repository's file over it (`installer.go:6133`) while the map named `gitconfig` - 
the sibling the comment eight lines above says was added to the map for precisely that reason -
and not `zshenv`.

| Task | Outcome |
|---|---|
| 1. Name every replaced home file in `ConfigPaths()` | Added `zshenv`, `gitconfig-personal`, `bashrc`, `bash-env-json`, `bash-env-nu` and `nushell_macos` (the macOS nushell directory the shell step writes on darwin), and pointed the existing `wezterm` key at `~/.config/wezterm`, the file the terminal step actually replaces: the old value `~/.wezterm.lua` is a file this installer never writes, so the backup copied a path nobody touched while the real one went unprotected. |
| 2. Guard the whole class, derived from the sources | `TestEveryHomePathTheInstallerWritesIsClassified` reads every non-test `.go` file in the package, resolves each `filepath.Join` rooted at `$HOME` from its string literals and string constants, and fails naming `file:line` and the path when the destination is neither under a `ConfigPaths()` entry nor in a documented table of installer-owned paths (data roots, caches, clone locations, the `.zshrc.d` drop-in directory). It also fails the other direction - a `ConfigPaths()` key no write targets is stale, which is how the `wezterm` mismatch was found. |
| 3. Make the appends checkable | The Homebrew step appended through a loop variable (`rcFile`), which no scanner can classify; the two destinations are now spelled as literals (`installer.go:4391`). |
| 4. Assert the user's symptom | `TestTheShellStepBacksUpEveryHomeFileItReplaces` seeds a temp `$HOME` with the user's own `.zshenv`, `.gitconfig-personal` and `.gitconfig`, runs the detection, the backup step and the shell step, and requires the user's exact bytes in the backup before the install replaces them. |
| 5. Document the list | `docs/tui-installer.md` backup-detection table now matches the map, which it did not (it listed `~/.tmux`, omitted `.zshenv`, `.gitconfig`, `.gitconfig-personal`, `.bashrc`, `.p10k.zsh`, `.config/bat` and the two bash-env helpers, and named both WezTerm locations as if the installer wrote both). |

Unprotected writes the guard found, with the fix applied to each. The line numbers are the ones in this
working tree; the guard printed the pre-fix numbers `6128`, `6133`, `6298`, `6303`, `6311`, `6884`,
`4947`, `4958` for the same sites (five lines difference, from task 3).

| Site | Path | Fix |
|---|---|---|
| `installer.go:6138` | `~/.zshenv` | new key (the reported defect) |
| `installer.go:6133` | `~/.gitconfig-personal` | new key - the user's identity, included by `.gitconfig` |
| `installer.go:6303` | `~/.config/bash-env-json` | new key |
| `installer.go:6308` | `~/.config/bash-env.nu` | new key |
| `installer.go:6889` (and the Homebrew append at `:4391`) | `~/.bashrc` | new key - appended to, still a file the user wrote |
| `installer.go:6316` | `~/Library/Application Support/nushell` | new key - the macOS nushell config, written by the same step that writes `~/.config/nushell` elsewhere |
| `installer.go:4952`, `installer.go:4963` | `~/.config/wezterm/wezterm.lua` | `wezterm` key corrected from `~/.wezterm.lua` |
| `installer.go:579` | `~/.local/share/dotfiles` | classified, not a key: the installer's own data root for the WSL template and theme definitions |

Nothing else was found: every other `$HOME` path in the package is either under a `ConfigPaths()`
entry or one of the classified installer-owned paths, and the guard fails on any new one.

## #241 - copied files lost the executable bit

`CopyFile` ended in `os.WriteFile(dst, input, 0644)`, so the mode was decided by the routine instead
of carried from the file, and every directory copy inherited it. `bash-env-json` and
`dotfiles-nvim/nvim/scripts/safe-update.sh` are shipped executable and are run as programs, so both
were installed un-runnable; the failure surfaced as `Permission denied` from nushell and as EACCES
from `:NzUpdatePlugins`. It stayed invisible because `os.WriteFile` leaves an existing file's mode
alone, so the machines where it was tested already had the bit from whenever the file was first
created.

| Task | Outcome |
|---|---|
| 1. Carry the mode from the file | `CopyFile` writes with the source's permission bits and then chmods the destination explicitly. The chmod is unconditional, not a creation-time mode: the file that needs repairing is the one already installed, and `os.WriteFile`'s mode argument only applies when it creates the file. The contract is "the destination matches the source", which has to hold for the existing file too. Only `Mode().Perm()` is carried - a source's setuid or setgid bits are not part of a copy and must not be installed into a home directory. |
| 2. Check the inconsistency the report named | The installer does chmod what it downloads (`installer.go:5717` after the Herdr archive, `installer.go:6618` for Herdr's config, `officecli.go:216`, `update_check.go:891`, `interactive.go:616`), so the missing bit was a difference between two routes inside one program, not a deliberate policy. |
| 3. Guard the whole class, derived from the repository | `TestEveryProgramTheRepositoryShipsIsInstalledExecutable` walks the installer's declared assets (`repoAssets`, itself guarded by `TestRepoAssetsExist`), treats any file whose first two bytes are `#!` as a program, and requires it to be executable in the repository and executable after a copy. No hand-written asset list: an asset added later inherits the check. |
| 4. Add the missing mode assertion | `TestInstalledAssetsKeepTheRepositorysMode` runs the nushell shell step and the nvim step against a temp `$HOME` and compares the installed `~/.config/bash-env-json`, `~/.config/bash-env.nu` and `~/.config/nvim/scripts/safe-update.sh` against the repository's own modes. Before this, a grep of the installer's tests found no assertion on the mode of an installed asset. |

## Validation

- `make check`: gofmt, `go vet ./...` and `go test ./internal/system ./internal/tui -count=1` all passed (253s for the TUI package on a loaded machine), no golden moved.
- Focused: `go test ./internal/system -run 'TestCopyFileCarriesTheSourceMode|TestCopyDirCarriesTheModeOfTheFilesInside' -count=1` and `go test ./internal/tui -run 'TestEveryHomePathTheInstallerWritesIsClassified|TestTheHomePathGuardFailsOnAnUnprotectedWrite|TestTheShellStepBacksUpEveryHomeFileItReplaces|TestEveryProgramTheRepositoryShipsIsInstalledExecutable|TestInstalledAssetsKeepTheRepositorysMode' -count=1`.
- Red, #238: the guard failed on `installer.go:6128`, `:6133`, `:6298`, `:6303`, `:6311`, `:6884`, `:4947`, `:4958`, `:579` and on the unresolvable `:4388`, plus `ConfigPaths() names wezterm at ~/.wezterm.lua, but no write in the installer targets it`; the behavioral test failed with `the user's ~/.zshenv was replaced with no copy in the backup`.
- Red, #241: `the copy is -rw-r--r--, want the source's -rwxr-xr-x: the routine decided the mode instead of the file`, and the guard named `bash-env-json` and `dotfiles-nvim/nvim/scripts/safe-update.sh` as installed without the bit.
- Teeth, #238: `TestTheHomePathGuardFailsOnAnUnprotectedWrite` feeds the guard a synthetic join and pins that an unprotected path is named, that a path under a protected directory is not, that a sibling sharing a name (`~/.zshrc.bak` against `~/.zshrc`) is named, and that an allowlisted directory does not cover anything under it.
- Teeth, #241: `chmod -x dotfiles-nvim/nvim/scripts/safe-update.sh` made the guard fail with `safe-update.sh starts with #! but is -rw-r--r-- in the repository`; the mode was restored to 755 and `git status` for that file is clean.
- No test writes to the real `$HOME`: every new test points `HOME`/`XDG_STATE_HOME` at a `t.TempDir()`.

## Remaining

- **#239 is blocked on a decision, not on work.** The record's `Files` is written from
  `m.ExistingConfigs`, the pre-run detection, so a run that skips a step still claims that step's
  files. Recording what the run *actually replaced* needs per-run state the model does not have: the
  replacements would have to be accumulated through `stepRecordedState` into a new `Model` field,
  and `installer/internal/tui/model.go` is outside this task's allowed edit surfaces; the honest
  change also makes `TestCompletingARunRecordsItAndWritesTheStateFile` in
  `installer/internal/tui/state_test.go` wrong, because it pins today's behaviour (`Files` ==
  `ExistingConfigs` for a run with no steps), and that file is outside the surfaces too. The smaller
  alternative - keeping the data and relabelling the panel row from "Touched" to what it actually
  is - needs `installer/internal/tui/panels_test.go`, which is also outside them. A guess in either
  direction would either be a lie in new words or a change no test in the tree can check.
- The repository tracks `alacritty.toml` with the executable bit (`git ls-files -s` reports
  `100755`). With the mode now carried from the file, an installed `~/.config/alacritty/alacritty.toml`
  keeps that bit. It is harmless - the file is still read as configuration - but it is repository
  hygiene that this task's surfaces do not cover (`git update-index --chmod=-x alacritty.toml` would).
