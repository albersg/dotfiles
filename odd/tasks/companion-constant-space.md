# Companion constant-space contract

## Reported problem

The companion in the Vim Trainer still uses the old small pet, while the installer menu can show a much larger creature. Its rung is currently chosen from rows left by each screen's content, so the creature changes size as content changes.

## Contract

The desired rung is determined by terminal height and the model's pixel/glyph mode, never by screen, selection, or spare rows. Pixel mode selects 12 rows at height 48+, 8 rows at 32–47, 5 rows at 25–31, and the glyph table below 25; `companionHeightShare = 4` bounds the rung to a quarter of the height, and the 12- and 8-row thresholds replaced 34+/30–33 because the 12-row rung took 35% of a 34-row screen. Glyph mode selects 5 rows at height 25+ and 3 rows at 24 or below. The 5-row mini volume is the same height as the glyph cat's full rung, so it takes over exactly that rung, at the same 25-row threshold, and nowhere else: heights 20–24 keep the compact three-row glyph head even with the encoder on. A five-row volume does not fit the rows those frames leave, the smallest honest volume is five rows (four loses the pupil), and a visible ASCII cat is better than no pet. `companionSprite` disambiguates the shared five-row height by mode and the glyph fallback still draws at that height. Art is bottom-aligned in its reserved block.

At 80x24, framed screens keep their layout and their creature: 35 of the 53 measured screens draw one, exactly the number `main` drew before the mini volume, and `TestCompanionCoverageAcrossTerminalSizes` now asserts that floor as well as the 636-case total (461 with the encoder on). The trainer preserves its one-row mini at that floor; above the floor it reserves the selected rung or fails the frame guard. The trainer's code window absorbs the reservation down to its one-row minimum. The sprite's 32-column width must fit at the 80-column floor. No golden was regenerated for the constant-space change; a moved framed golden is a finding.

## Tasks and evidence

| Task | Outcome | Commit |
|------|---------|--------|
| Add the terminal-size/content-state guard and prove it rejects the spare-row rule | Complete: `TestCompanionBlockDependsOnlyOnTheTerminal` checks identical rung and placement across content states; `TestCompanionVolumeSpriteIsTheLadderTopSteps` now drives terminal height and mode | c59872a |
| Reserve fixed companion blocks in framed and trainer layouts while preserving floor output | Complete: trainer reserves the rung; framed layouts use leftover rows, preserve the 80x24 floor and refuse to overwrite content | 3b038c8 |
| Measure both companion cost regimes and verify frame, leak, and golden guards | Complete: focused guards pass; celebration face restored without regenerating its golden | this commit (documentation) |
| Update the TUI companion documentation from measured test output | Complete: height/mode ladder, trainer reservation, framed leftover rows, no shrinking, and measured output documented | this commit (documentation) |

## Measurements

`TestCompanionVolumeSpriteIsTheLadderTopSteps` prints the selected terminal rung: pixel mode at heights 48 and 47 selects 12 and 8 rows; height 32 also selects 8, height 31 and height 25 select the 5-row mini volume, and heights 24 and 20 keep the 3-row compact glyph head. Glyph mode at height 25 selects 5 rows, and height 24 selects the 3-row compact glyph rung. With pixel mode off at height 40, the rung is 5 rows. `TestCompanionRungIsNeverMoreThanAQuarterOfTheHeight` pins the share at each height (at 34 it is 23.5%, was 35.3%; at 30 it is 16.7%, was 26.7%). `TestCompanionCoverageAcrossTerminalSizes` measured 461 draw, 175 do not of the 636 screen × size cases, where the old boundaries left 213 without one, and 35 of 53 screens at 80x24. The mini volume is the same five rows as the glyph rung it replaces, so it does not change those numbers and the guard now asserts they cannot fall below them.

`TestCompanionCostHasTwoRegimes` at 227 columns reports: rest writes 0 bytes; walking uses 12 sprite rows and changes up to 12 lines per moving tick, 19 moving frames, 136,135 bytes total and 7,488 bytes in the widest changed lines (58.5 KB/s at 8 fps).

## Validation and remaining work

The final full suite passed: `cd installer && go test ./... -count=1` — 3,856 tests across four packages. `cd installer && go vet ./...` passed; `gofmt -l` on the changed Go files returned no paths; `git diff --check` passed. The focused frame, leak, cost, welcome-lockup, volume-ladder, and celebration-golden guards passed. The celebration golden matched as-is and was not regenerated.

The initial focused run reproduced the reported failures: both volume subtests at old spare-row cases observed zero half-block rows instead of eight, and `TestCompleteCelebrationGolden` mismatched because the celebration face was absent. A first implementation placement in `companionArtFor` also tripped `TestCompanionArtIsRowsOfPrintableASCII` by changing the compact art table; the rung-specific expression was correctly moved into `companionSprite`, preserving the table invariant while restoring the floor face. All three failures are resolved in the final suite.

No implementation work remains. The commits are `c59872a` (the rung comes from the terminal), `3b038c8` (the
tests follow the contract) and the documentation commit that carries this row; the parent verified the suite and
merged. Nothing about this front is left open.
