# The per-user font registration Windows has to resolve

## Goal

`installAndVerifyWindowsFont` copies four Meslo faces into the Windows per-user font store
and registers them under `HKCU\Software\Microsoft\Windows NT\CurrentVersion\Fonts`, and the
face is written to Windows Terminal only when the store then resolves the family. That
gate is right and stays. This front is about its two premises being wrong: the registration
step **destroys every registration already in the key**, and the value it writes **cannot be
resolved**, so the gate refuses forever and the user is left with neither the font nor the
registrations they had.

This is a separate front from [font-windows-truth.md](font-windows-truth.md): that file is
an open front owned by another machine (`C:\Users\t749597`), and this defect was measured on
a different one.

## Evidence, measured on this machine (read-only)

Reproduced with:

```
DOTFILES_SKIP_DEPS=1 DOTFILES_SKIP_TOOLSET=1 DOTFILES_VERBOSE=1 \
  dotfiles --non-interactive --shell=zsh --wm=herdr --nvim --font
```

| what | what it says |
|---|---|
| the step's own line | `Windows Terminal's font face was not written: "MesloLGS Nerd Font" is not present in the Windows font store after registering it, so the face was not written` |
| `HKCU\...\Fonts` before the run | **36 entries**, including the whole CaskaydiaCove Nerd Font family |
| `HKCU\...\Fonts` after the run | **4 entries**, all of them the installer's own Meslo ones |
| the value the installer writes | `MesloLGS Nerd Font Regular (TrueType)` => `MesloLGSNerdFont-Regular.ttf` |
| the shape the destroyed entries had | `CaskaydiaCove NF Bold (TrueType)` => `C:\Users\alber\AppData\Local\Microsoft\Windows\Fonts\CaskaydiaCoveNerdFont-Bold.ttf` |
| `InstalledFontCollection` after the run | resolves **neither** `MesloLGS Nerd Font` **nor** `CaskaydiaCove NF` |
| the per-user font store | **40** `.ttf` files, all of them still on disk; only 4 are registered |
| `profiles.defaults` | `{}` — the face was never written, which is the correct refusal |

The mechanism was proven against a scratch key rather than inferred:

```powershell
New-Item -Path $k -Force | Out-Null            # key already held Keep A and Keep B
New-ItemProperty -Path $k -Name 'X (TrueType)' -Value 'x.ttf' -PropertyType String -Force
```

```
--- before ---          --- after ---
Keep A (TrueType)       X (TrueType)
Keep B (TrueType)
```

## The defects

### D1 — `New-Item -Path $key -Force` wipes the key's values

`windowsFontRegisterScript` opens with

```powershell
$key = 'HKCU:\Software\Microsoft\Windows NT\CurrentVersion\Fonts'
New-Item -Path $key -Force | Out-Null
```

On a key that already exists, PowerShell's `New-Item -Force` **deletes and recreates it**,
discarding every value it held. The intent was plainly to make sure the key exists; the
effect is that the installer silently uninstalls every per-user font on the machine that was
not installed by Windows itself. Measured cost on this machine: 36 registrations, the entire
CaskaydiaCove Nerd Font family, whose `.ttf` files are still in the store and now resolve for
nothing.

This is the defect that makes the step dangerous rather than merely useless, and it is
independent of D2: it fires on every run that reaches the font path, whether or not the face
is eventually written.

### D2 — a bare filename is not a resoluble registration for a per-user font

The script writes the file's base name as the value data:

```powershell
New-ItemProperty -Path $key -Name 'MesloLGS Nerd Font Regular (TrueType)' \
                 -Value 'MesloLGSNerdFont-Regular.ttf' -PropertyType String -Force
```

A bare name is resolved against `%WINDIR%\Fonts`, which is where the system-wide store lives.
These faces are in `%LOCALAPPDATA%\Microsoft\Windows\Fonts`, so the value points at nothing.
Windows names a per-user font with the **absolute path** — which is exactly the shape the
destroyed Caskaydia entries had — and `System.Drawing.Text.InstalledFontCollection`, the list
Windows Terminal resolves against, only sees a family whose entry can be followed.

So the family never resolves, `installAndVerifyWindowsFont` returns its refusal, and the face
is correctly withheld. The gate is doing its job; D1 and D2 are why it can never pass.

### D3 — collateral, same step

- The same run overwrote the hand-tuned `C:\Users\alber\.wslconfig` with the host-derived
  values (`memory` 6GB -> 7680MB, `processors` 8 -> 12, `swap` 4GB -> 1536MB), losing the
  comments too. A backup exists, so this is recoverable, but the step replaces a deliberate
  local choice without saying that it is doing so.
- The `.meslo-*` scratch directory is left inside `~/.local/share/fonts`, so `fc-cache`
  reports `skipping, looped directory detected` on every run.

## What is already right, and must stay right

- **Verification gates the write.** The family is read back from the store the terminal reads,
  and a family that does not resolve withholds the face instead of writing one that would
  fall back silently. D2 was found *because* this gate fired.
- **A refusal is reported, not swallowed.** The step says which face was not written and why.
- **The registration is per-user.** No administrator is needed and nothing outside the current
  user's hive is touched — once D1 is fixed.

## Tasks

- [x] **T1 — Restore the destroyed registrations.** Rebuilt `HKCU\...\Fonts` from the 40
  `.ttf` files in the per-user store: each value named `<name ID 4> (TrueType)` — read from the
  font's own `name` table, which is how the destroyed Caskaydia entries were named — and pointed
  at the absolute path. 40 files, 40 readable names, 0 collisions, 40 registrations. Before: 4.
  After: `InstalledFontCollection` resolves `MesloLGS Nerd Font` and the whole CaskaydiaCove
  family again (`NF`, `NFM`, `NFP`).
- [x] **T2 — Restore the user's `.wslconfig`.** The first backup holding the authored values was
  `.wslconfig.bak-dotfiles-20261010-143849`; every later one already held the derived ones, so the
  replacement began on **2026-10-11 17:06**, not on the run that found this. Read back:
  `memory=6GB processors=8 swap=4GB`, CRLF line endings preserved.
- [x] **T3 — The failing test.** `TestWindowsFontRegistrationCannotUninstallTheFontsItDoesNotOwn`
  pins both halves. Against the untouched code it reported **10 distinct failures**: 1 for the
  unguarded key creation, 1 for the store not being resolved from `%LOCALAPPDATA%`, 4 for the face
  not being joined onto the store, and 4 for the bare file name written as the value.
- [x] **T4 — Fix the script.** `windowsFontRegisterScript` now creates the key only when
  `Test-Path` says it is missing, resolves `$store` from `$env:LOCALAPPDATA` and writes
  `-Value (Join-Path $store '<face>.ttf')`. The refusal is untouched: a family the store does not
  resolve still withholds the face.
- [x] **T5 — The local proof.** The whole font path was run against this machine's real Windows
  after the fix and the registrations were read back before and after.

## The proof

Both fixes were validated against the real machine, not only in the unit test. A scratch key
holding two values was put through the proposed sequence: both survived and the new value landed
with the absolute path
(`MesloLGS Nerd Font Regular (TrueType)` => `C:\Users\alber\AppData\Local\Microsoft\Windows\Fonts\MesloLGSNerdFont-Regular.ttf`).
Each half of the test was then mutated to confirm it has teeth: removing the `Test-Path` guard
fails it (1 failure), and restoring the bare file name fails it (8 failures). Both mutants were
reverted and the test is green on the fix.

The end-to-end run of the fixed binary over this machine's Windows reported:

```
✓ Windows Terminal now reads from ...\LocalState\settings.json: the font face
  "MesloLGS Nerd Font" and the colour scheme "dotfiles"; the previous file is at
  ...settings.json.bak-dotfiles-20261013-135559.
```

Read back afterwards:

| what | value |
|---|---|
| `HKCU\...\Fonts` | **40** entries, unchanged by the run — nothing was destroyed |
| sample restored entry | `CaskaydiaCove NF Bold (TrueType)` => `C:\Users\alber\AppData\Local\Microsoft\Windows\Fonts\CaskaydiaCoveNerdFont-Bold.ttf` |
| `profiles.defaults` | `{"font":{"face":"MesloLGS Nerd Font"},"colorScheme":"dotfiles"}` |
| `InstalledFontCollection` | resolves `MesloLGS Nerd Font` |
| `schemes` | 1, `dotfiles` |
| the rest of `settings.json` | 6 profiles, 4 keybindings, `defaultProfile` and `copyOnSelect` all preserved |

The step still reports the `/etc/wsl.conf` install as failed, because `sudo` needs a terminal and
the run had none. That is the step's other half, unrelated to any of this, and it is not a
regression.

## Remaining

- **D3 is not fixed here.** The step still replaces a hand-tuned `.wslconfig` with the host-derived
  values, comments included. It is recoverable (a backup is taken) and it is a different decision
  from this one, so it is left as its own front rather than folded into a registration fix.
- **The `.meslo-*` scratch directory is still left inside `~/.local/share/fonts`,** so `fc-cache`
  reports `skipping, looped directory detected` once per run. Cosmetic, and also a different front.
- **Four Ghostty tests fail on `main` without this change**
  (`TestStepInstallTerminalGhosttyReportsAFailedDownload`, `TestStepInstallTerminalGhosttyRunsTheDownloadItFetches`,
  `TestInteractiveGhosttyScriptFailsWhenTheDownloadFails`, `TestInteractiveGhosttyScriptDownloadsThenRuns`).
  They were confirmed failing on a clean `main` worktree, and the failing set is identical before and
  after this change.
