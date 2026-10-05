# The pet on small screens, the panel that hides it, and a utilities section

Four asks from the user, in their own words:

1. *(context)* Use the dotfiles Herdr sessions if necessary — the three independent review sessions created for this work live at `/home/alberto/work/dotfiles-rev-{uso,docs,terminal}` and answer over intercom.
2. **"Que el gato se ve todavía ASCII si es muy pequeño la pantalla. Haz una versión más pequeña del pet 3D."** Below 32 rows of terminal the ladder falls back to the glyph cat; at 24 rows and under it is a 3-row ASCII face. The ask is a **small volumetric rung** so the creature is the 3D one at small sizes too, not a different animal.
3. **A new section, "utils"** — utilities as a menu entry, the user's example being switching the theme of the whole system.
4. **The main menu's right-hand "What will happen" panel is too big and the pet disappears.** Fix it by making the panel smaller, or by another means if a better one exists — the user's own words say "the best decision and practice, whatever you want".

## Standing constraints (from the repository's own contracts)

- **The rung is a pure function of the terminal and of whether the volumetric encoder is available** — never of spare rows, of the screen, of the selection or of the content (`odd/tasks/companion-constant-space.md`, PR #102). Any fix here moves thresholds or adds rungs; it must not reintroduce content dependence.
- The rung is now **bounded to a quarter of the terminal height** (`companionHeightShare = 4`, PR #148): volume-full at 48+, volume-small at 32–47, glyph below.
- **NO GOLDEN MAY MOVE** unless the movement is explained line by line before it is accepted; a moved framed golden is a finding, not a chore.
- **Termux and 80x24 are the floor**: no emoji-only meaning, no truecolor assumption, and the frame must fit (the guard `TestEveryScreenFitsEveryTerminalSize` measures 53 screens × 12 sizes).
- A guard that can see the class beats a test that can see the instance. A number in prose comes from a test that prints it.

## Tasks

| # | Task | Surface | Evidence required |
|---|---|---|---|
| 1 | A small volumetric rung: the 3D creature at the heights where only the glyph cat draws today | `companion.go` (encoder + ladder), `companion_test.go` | rendered rows per rung at each height; the glyph fallback's last height named; cost at that rung measured |
| 2 | The main-menu plan panel stops crowding the creature out | `panels.go`, `view.go`, `panels_test.go` | the measured rows of panel + body + rung at 80x24, 100x25, 120x34, 160x50, before and after |
| 3 | A utilities section reached from the menu, with at least one real utility (system theme) | `model.go` (screen + menu), `view.go`, `installer.go` (step), tests | the screen's frame fit at 12 sizes; the step's dry-run behaviour; what it changes on disk, named |

## Open questions the user must settle only if the answer changes what is built

- Task 3 changes files **outside the installer's own configuration** (a system theme). If the theme switch is to be offered, it must be reversible and must never destroy a value the user set — the same rule as `preserve-user-configs.md`. Which file(s) and which mechanism is the implementer's decision, but the reversibility is not optional.
