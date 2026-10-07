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
- **Every utility has an installation route and a menu route, and the two share one implementation; a guard proves they agree.** A utility that also runs during an installation (today the WSL resources: `dotfiles-wsl/.wslconfig.tmpl` is rendered at install time *and* editable from the menu) must not grow a second calculation or a second writer. One function computes the recommended values, one function builds the content, one function writes it, and `TestWSLResourceUtilityAndTheInstallerAgreeByteForByte` mounts the file by both routes from the same host and the same pre-existing file and fails unless the bytes are identical. This repository already paid for the alternative once — the palette hand-written in six places, and the sixth copy that fell behind — so the rule is inherited by every later utility: the terminal probe, the startup audit, the doctor.
- **The main menu's Utilities panel derives its facts from the same model state the section does, one entry per utility, and the count is `len(entries)`.** `utilitiesPanelEntries` (`panels.go`) is the only list: it reads `ThemeSwitchFound`, `themeUndoAvailable`, `DotfilesThemeRecord`, `dotfilesThemeOptions` and `WSLState` — the fields the section's own options are built from — so the panel and the section cannot disagree, and a utility added to the section shows up in the panel or is visibly missing from that one function. `TestUtilitiesPanelFactsAreDerivedFromTheModelState` turns each field on and requires the panel to change, so a fact typed into the panel cannot survive.
- A guard that can see the class beats a test that can see the instance. A number in prose comes from a test that prints it.

## Tasks

| # | Task | Surface | Evidence required |
|---|---|---|---|
| 1 | A small volumetric rung: the 3D creature at the heights where only the glyph cat draws today | `companion.go` (encoder + ladder), `companion_test.go` | rendered rows per rung at each height; the glyph fallback's last height named; cost at that rung measured |
| 2 | The main-menu plan panel stops crowding the creature out | `panels.go`, `view.go`, `panels_test.go` | the measured rows of panel + body + rung at 80x24, 100x25, 120x34, 160x50, before and after |
| 3 | A utilities section reached from the menu, with at least one real utility (system theme) | `model.go` (screen + menu), `view.go`, `installer.go` (step), tests | the screen's frame fit at 12 sizes; the step's dry-run behaviour; what it changes on disk, named |
| 4 | The WSL resources as a menu utility, sharing the install step's calculation and writer | `wslconfig.go` (plan + merge + write), `wsl.go` (step + utility I/O), `model.go`/`update.go`/`view.go` (screen + row), `panels.go` (the panel's derived facts), `util_screen_test.go`, `wslconfig_test.go`, `screen_coverage_test.go` | the row offered on WSL and absent elsewhere with the reason stated; the host numbers and the recommendation on screen; the two routes byte-identical for one host and one pre-existing file, with the guard's teeth shown; the user's own keys preserved and the previous file backed up; the dry-run skip; the frame measured at 12 sizes; the Utilities panel naming the row from the derived entry list |
| 5 | The terminal capability report: a read-only utility that says what the terminal can do and what it implies, with `unknown` as a first-class answer | `terminal_probe.go` (probe + three-state answers + bounded query), `model.go`/`update.go`/`view.go`/`panels.go` (screen + row + derived panel entry), `terminal_probe_test.go`, `util_screen_test.go`, `screen_coverage_test.go`, `docs/tui-installer.md`, `README.md` | the three states including `unknown` with a reason and a manual check, with the teeth shown; the non-responding-terminal guard proving the bounded read; the source named for every answer; the probe armed only when the screen is opened (never at startup); the frame measured at 12 sizes; the row derived from `utilitiesPanelEntries` |

## Open questions the user must settle only if the answer changes what is built

- Task 3 changes files **outside the installer's own configuration** (a system theme). If the theme switch is to be offered, it must be reversible and must never destroy a value the user set — the same rule as `preserve-user-configs.md`. Which file(s) and which mechanism is the implementer's decision, but the reversibility is not optional.

## Task 5 notes: the terminal capability report

- **The report is read-only and offered everywhere.** Unlike the WSL row (offered only where there
  is a `.wslconfig`), the capability report is always offered: there is always a terminal to describe,
  and where there is none the screen says so. There is therefore no silent absence to explain. The
  screen writes nothing and runs nothing, so `--dry-run` has nothing to skip.
- **`unknown` is the zero state.** A capability the probe did not determine is reported as `unknown`
  with the exact reason and, where a person can settle it, a manual check. The guard
  `TestUnknownIsItsOwnStateNotAnInventedNo` fails by name if the unknown case is folded into
  "unsupported".
- **The bounded read is the whole safety property.** `queryTerminal` writes the DECRQM and XTGETTCAP
  queries and reads under a read deadline (falling back to a timer for a stream that cannot take
  one). `TestTerminalQueryDoesNotHangWhenTheTerminalNeverAnswers` measures a silent terminal and
  fails if the call outlives the timeout. The probe is a `tea.Cmd` armed when the screen is opened,
  so a run that never visits it writes no query byte.
- **Residual risk, named rather than hidden.** On a host whose stdin is the controlling terminal,
  the live query reads the same terminal bubbletea is reading its input from, so a keystroke in the
  query window could be consumed. The read is bounded and short, and it happens only on the screen
  the user deliberately opened; a stream that cannot take a deadline is abandoned on the timeout
  rather than cancelled (a reader cannot be cancelled), which is the price of the bound. A future
  change could route the query through a dedicated input reader if the race is ever observed.

## Open questions left by task 4

- **The interactive route's merge lives in the content builder, not the writer.** There is no Go writer on that route at all — the generated script copies the bytes `mergedRepoWSLConfig` returns straight over `%USERPROFILE%\\.wslconfig` — so the merge cannot be moved to a write site. The function is named for what it does (`mergedRepoWSLConfig`, not `renderedRepoWSLConfig`) precisely so the next reader does not treat it as a pure render. Residual risk: between the render and the shell's copy there is a window in which a concurrent edit of the file would be lost; the non-interactive step and the utility do not have that window because they read and write the same file in one pass.
- **The three keys the utility adjusts are memory, processors and swap.** The rest of the template (`networkingMode`, `dnsTunneling`, `localhostForwarding`, `autoMemoryReclaim`, `sparseVhd`) is managed by a write but is not editable on screen: changing it from a menu would change how WSL networks and reclaims memory, which is a different decision than sizing the VM, and it was not asked for.
- **Removing a key is not offered.** A key the plan omits is left as the file has it, so the utility cannot delete a limit; adding removal is only safe if it can distinguish "the host is unknown" from "the user means to unset this", which is a separate design.

## The shell startup audit, recorded as a later utility

The audit is the next entry in the section and the **first utility that changes nothing**: it starts
the login shell named by `$SHELL` the way a terminal does, reports the median of those starts, and
attributes the number with zsh's own `zmodload zsh/zprof`. It is not one of the four asks above; it is
recorded here because the constraints above were written to be inherited by it, and because its own
rules are the same shape.

- **Every start is bounded, and a timeout is a result of its own.** A startup can hang on a plugin
  that waits on the network, and the utility must not hang with it. A start that does not finish is
  killed, counted and named as a timeout, and left out of the median: a hung start has no duration to
  report, so writing one down as `0.0 s` would be inventing a number. The guard that hangs without
  the bound is `TestShellAuditDoesNotHangWhenTheShellDoes`; the bound itself is pinned by
  `TestShellAuditBoundsEveryStart`.
- **The number travels with its method.** Five starts -- odd, so the median is a run that really
  happened -- of the exact command (`zsh -i -c exit`), with the median, the range, the count it
  covers and every completed start on screen, so it can be reproduced by hand. Five is also the
  bound on the wait: with every start hitting the timeout the whole measurement is bounded by five
  times it.
- **Attribution is zprof's, or it is nothing.** With no `zprof`, with a shell that is not zsh, or
  with a profiled start that reports no table, the screen says only the total is measurable and gives
  the reason. No function is named that zprof did not name, and the table is read once -- zprof
  repeats each row in a per-function detail block, and `TestShellAuditReadsZprofsSummaryTableOnce`
  is what keeps the second copy off the screen.
- **Read-only.** The two wrappers and zprof's table live in a directory of the utility's own under
  the system temporary directory and are removed again; the user's startup files are sourced from
  where `ZDOTDIR` already pointed, so nothing of theirs is copied, moved or rewritten. Nothing is
  disabled and no suggested edit is applied.
- **One list, one entry, one frame guard.** The section's row and the main-menu panel both come from
  `utilitiesPanelEntries`, and the new screen is measured at the twelve sizes by the same guard the
  other utilities screens are (`shellAuditCaseName`).
- **Where it cannot measure, it says why.** No `$SHELL`, a login shell this machine does not have, or
  a run with no terminal attached: each is named in the section's own body rather than left as a
  hole, and the row is not offered.

## Follow-up: the WSL resources from any working directory

The user's case — *"Si no estoy en la ruta del dotfiles, no me salen las utilities"* — also reached
the WSL resources: the utility resolved `dotfiles-wsl/.wslconfig.tmpl` only from a checkout
(`$DOTFILES_DIR`, the clone, the working directory and its parents, `~/dotfiles`, `~/.dotfiles`), and
the clone an install makes is removed when the run finishes. The clone step now copies the template
into the per-user data directory — the same root as the theme definitions, `$XDG_DATA_HOME/dotfiles`
or `~/.local/share/dotfiles`, keeping the `dotfiles-wsl/` path — and the resolver reads it last, so a
checkout always wins and the row is offered from anywhere. Only the one shipped file is written, an
identical copy is left as it is, nothing else under the data directory is touched and nothing is
deleted; when no candidate holds the template the section names the search including the copy
directory. The user's case is reproduced by
`TestWSLResourcesAreOfferedFromTheInstalledCopyWithoutACheckout`, whose teeth are the new candidate:
removing it fails the guard by name. `TestInstallWSLTemplateCopiesOnlyMissingOrDifferent` pins the
copy rules and `TestWSLTemplateResolutionReportsWhereItLooked` pins the message.

The test that exercised the clone step now pins `HOME`, `XDG_DATA_HOME` and `XDG_STATE_HOME` to
`t.TempDir()`, because the clone step is the step that installs runtime assets into the data
directory: a stray write must land in the test's own tree, never in the runner's real
`~/.local/share` or `~/.config`.
