# Main-menu easter eggs

## What they do

- `vim` opens the Vim Trainer through the same `startTrainer` path as selecting the menu entry.
- `:q` quits the installer (as does `q`).
- `dd` briefly strikes and sweeps away the selected menu row, leaving a small puff in its reserved slot, then restores the original row automatically.

## Safety rules

The menu buffer contains only a prefix of `dd`, `:q`, or `vim`, and is bounded to three runes. A non-extending key clears it and is passed to normal menu handling in that same update; non-key events, including ticks and mouse events, clear it too. The gag stores its row and elapsed logical tick count on `Model`. Rendering reads only that state. Its 8-tick duration is named; animation-off holds one composed pose while the regular model tick still expires it. The row count and row slot stay fixed throughout.

## Tasks and evidence

- [x] Add bounded prefix recognition and share the trainer entry path. Evidence: `TestMainMenuEasterEggsFireOnlyAfterTheirLastCharacter` and `TestMainMenuEasterEggPrefixesDoNotSwallowOtherInput` drive `Model.Update`.
- [x] Add the deterministic selected-row gag and restoration. Evidence: `TestMainMenuDDEasterEggRestoresExactRowAndStaysWithinFrame` pins the struck content, row count, duration, and exact restored view; `TestMainMenuDDEasterEggIsDeterministicAndAnimationGateFreezesPose` pins repeatability and the gate-off pose.
- [x] Document the forms without fanfare and record the change. Evidence: `docs/tui-installer.md`, `CHANGELOG.md`.
- [x] Run frame guards, the complete installer suite, `go vet ./...`, and `gofmt -l`. Evidence: `TestInstallerScreensFitTheFrame` and `TestInstallerScreensFitWideTerminals` passed (145 assertions); `cd installer && go test ./... -count=1` passed (3,899); `cd installer && go vet ./...` passed; `gofmt -l` printed no files. `git diff --check` passed.
- [x] Break and restore two guarded behaviours. Evidence (verbatim):
  - Swallowing navigation after `d`:
    ```text
    Go test: 0 passed, 2 failed in 1 packages

    tui (0 passed, 2 failed)
      [FAIL] TestMainMenuEasterEggPrefixesDoNotSwallowOtherInput/d_then_j_navigates_immediately
         companion_test.go:126: navigation was consumed or gag queued: cursor=0 buffer="d" gag=false
      [FAIL] TestMainMenuEasterEggPrefixesDoNotSwallowOtherInput
    [full output: ~/.local/share/rtk/tee/1790939749_go_test.log]

    Command exited with code 1
    ```
  - Leaving the row struck:
    ```text
    Go test: 0 passed, 1 failed in 1 packages

    tui (0 passed, 1 failed)
      [FAIL] TestMainMenuDDEasterEggRestoresExactRowAndStaysWithinFrame
         companion_test.go:214: gag did not restore the original screen exactly at its duration
    [full output: ~/.local/share/rtk/tee/1790939768_go_test.log]

    Command exited with code 1
    ```
  Both deliberate mutations were restored, then focused and full tests passed.

## Left over

None known. Existing menu behavior remains that bare `q` passes through; the new `:q` form explicitly quits. No golden files moved: normal, non-gag rendering is unchanged, and the 80×24 gag is pinned by the behavioral test rather than a new snapshot.
