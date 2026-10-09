# ODD work tracker

`odd/tasks/` is this repository's work tracker: one markdown file per front of work, written while the
front is open and kept afterwards as the record of what was decided and what was found.

## The convention: one file per front

A **front** is one thread of work with one owner and one reason to exist: a feature, a defect, an
audit, a design that has to be settled before code. A front that spans several layers is still one
file. A second front gets a second file, named for the front.

The numbers behind the rule are measured, not asserted: **11 of the last PRs touch `odd/tasks/`**, and
**4 conflicts in a single night happened only because two branches were editing the same task file**.
A task file is a working document with a single writing thread, so two fronts inside one file is a
conflict with no technical content — and a conflict resolution that merges two fronts is a diff nobody
can review.

1. **One front, one file, one owner.** Write in your own file; never open a file another front is
   editing.
2. **A fact that belongs elsewhere is linked, not copied.** One thing, one place: if a fact is
   already written down, point at it.
3. **Never merge existing task files to tidy the directory.** Consolidating `odd/tasks/` is its own
   front with its own review; doing it inside a feature change turns a reviewable diff into an
   unreadable one.
4. **The file carries evidence, not intentions.** The red that was observed, the commands that were
   run, the outcomes, and the items left open — for each thing the file claims.
5. **A file that is done says so.** `## Remaining` is what tells the next reader whether the front is
   finished, so "nothing" is written down rather than omitted.
6. **A guard needs teeth.** When a front adds a check, the file records what the check rejects as well
   as what it accepts — a guard nobody has seen fail proves nothing.

## Index

Open fronts and settled records, alphabetically. A file that is finished stays here: it is the
repository's memory of why something is the way it is.

| File | Front |
|------|-------|
| [ci-efficiency.md](tasks/ci-efficiency.md) | CI wall clock and what dominates it |
| [companion-constant-space.md](tasks/companion-constant-space.md) | The creature's placement ladder in constant space |
| [companion-realism.md](tasks/companion-realism.md) | The realism pass on the creature |
| [companion-v2.md](tasks/companion-v2.md) | The second generation of the creature (mandate) |
| [companion-v2-research.md](tasks/companion-v2-research.md) | The research behind it, superseding that decision only |
| [contextual-panel.md](tasks/contextual-panel.md) | The contextual panel |
| [install-update-release.md](tasks/install-update-release.md) | The installer's own release: checking it, updating from it, shipping it |
| [installer-companion.md](tasks/installer-companion.md) | The companion in the installer |
| [installer-ux.md](tasks/installer-ux.md) | Installer user experience |
| [installer-visual-identity.md](tasks/installer-visual-identity.md) | The installer's visual identity |
| [menu-easter-eggs.md](tasks/menu-easter-eggs.md) | The main menu's easter eggs |
| [night-close-out.md](tasks/night-close-out.md) | The night's closing handover |
| [pet-panel-utils.md](tasks/pet-panel-utils.md) | The pet panel and the utilities section |
| [pi-agent-skills-officecli.md](tasks/pi-agent-skills-officecli.md) | The pinned agent skills and OfficeCLI |
| [preserve-user-configs.md](tasks/preserve-user-configs.md) | Preserving user shell configurations |
| [themes-integration.md](tasks/themes-integration.md) | Theme integration |
| [trainer-hint-coherence.md](tasks/trainer-hint-coherence.md) | The trainer's hints |
| [tui-facelift.md](tasks/tui-facelift.md) | The TUI facelift |
| [unified-themes.md](tasks/unified-themes.md) | The unified theme library |
| [upstream-activity-tracker.md](tasks/upstream-activity-tracker.md) | Watching upstream activity |
| [upstream-triage.md](tasks/upstream-triage.md) | Triaging upstream |
| [vim-trainer-buffer-engine.md](tasks/vim-trainer-buffer-engine.md) | The trainer's buffer engine |
| [vim-trainer-improvements.md](tasks/vim-trainer-improvements.md) | Improvements to the trainer |
| [visual-dashboard.md](tasks/visual-dashboard.md) | The visual dashboard |
| [wsl-host-derived-config.md](tasks/wsl-host-derived-config.md) | WSL configuration derived from the host |

Add a file to this index in the same change that adds it: an index that lags is an index nobody
trusts, and the task file is the only place the front's reasoning exists.
