# ODD Feature Tasks: Installer visual identity

## Goal
Give the installer a deliberate look instead of a correct-but-plain one: a persistent frame that tells
you where you are and how far along you are, rows and blocks that read as objects, keys that look like
keys, and a first screen that looks designed. This is the "real aesthetic jump" the user asked for
after the two measurement-driven slices that only made things fit and read.

## What "plain" means today, concretely
- Every screen invents its own header, and three of them had hard-coded titles until the previous slice.
- A selected row is a marker plus text: `▸ 🚀 Start Installation`, with a different indent than the
  unselected rows and no visual weight.
- Content is loose indentation: a label in one style, the value indented under it, and blank lines used
  as the only separator.
- The footer is one muted italic sentence, so the keys inside it are indistinguishable from the words
  around them.
- Progress is a flat inline rail (`✓ ● ○ →`) that also wraps.
- The splash emblem is one flat colour, and the status line is a sentence.

## Accepted design decisions (parent-owned, since the user delegated taste)
**Palette stays.** The adaptive palette from the previous slice is good and already solves light and
dark. This feature changes how the palette is USED, not what it is.

**One meaning per colour, and no colour alone.**
- `Brand` — chrome: the header title, section chips, the filled part of step progress.
- `Accent` — the single thing to act on right now: the selected row's marker, the countdown chip, the
  key tokens in the footer, the current step.
- `Paper` on `BrandSoft` — the selected row's bar.
- `Ink` — body. `InkDim` — metadata, help verbs, line numbers, secondary.
- `Rule` — every separator and gutter.
- `Success`, `Warn`, `Danger` — state only, never decoration.
Every state keeps a glyph or a word, so a 16-colour or colourless terminal loses nothing.

**A persistent frame.** Header, body, footer, on every screen:
- Header: the app name in `Brand`, the current section, and on the right either the step counter with a
  small bar (`▓▓▓▓▓░░░░░ 5/8`) or the screen's own vital sign (the trainer puts the mode, the score and
  the streak there; the boss puts the lives and the countdown). One row.
- A `Rule` line under the header and above the footer, each spanning the inner width.
- Footer: the help line with key tokens in `Accent` and verbs in `InkDim`, in the canonical order the
  previous slice established. One or two rows, never more.

**Rows are objects.** The selected row is a full-width bar: `BrandSoft` background, `Paper` bold text,
and the `▸` marker in `Accent`. Unselected rows are plain `Ink` at the same indent, so state comes from
weight and the bar rather than from indentation. A row may carry a trailing meter (`▓▓▓░░ 3/20`) in dim
for anything with progress.

**Blocks carry a gutter.** A label chip (`Mission`, `Code`, `Summary`, `Contents`, `Tools`) in `Brand`
bold, and the content indented behind a `Rule`-coloured `│` gutter, so a block reads as one object
instead of a label followed by floating text. No boxes: a box costs rows and the frame already groups.
Blocks are separated by exactly one blank line, and no screen ends with a blank line.

**The splash looks designed.** Keep the emblem glyphs and give them a vertical ramp through the palette
so they are not one flat colour, keep the wordmark, move the version and the detected environment into
one dim line under it, and keep the full-emblem/compact threshold and every geometry test exactly as
they are: those are measured decisions.

**Density.** Move per-screen metadata into the header instead of stacking label rows; one blank line
between blocks; no trailing blanks. The goal is that the same information occupies fewer rows, which the
code windows and the fit guards then spend on content.

## Constraints that are not negotiable
- The 80x24 floor, and both frame guards must keep passing: they assert height, width, and that the
  trainer's start cursor row is visible. Adding chrome means updating each screen's reserved rows in
  those guards, which is expected and must be reported.
- Termux and Docker: no dependence on emoji fonts for meaning, no truecolor assumption.
- Every golden will move. That is expected. What is not acceptable is regenerating a snapshot around a
  bug, so each moved golden must be explained.
- The non-interactive output, the trainer's engine and judge, and the install logic are out of scope.

## Slices
### V1 — chrome, components and the installer screens
- [ ] Shared components: header bar with optional step counter and bar, rules, footer with key tokens,
      panel block with a gutter, row bar, trailing meter, and the section chip.
- [ ] Apply them to every non-trainer screen: welcome, main menu, every selection screen, installing,
      complete, error, learn and tool info, keymaps and their tables, LazyVim, backup, restore.
- [ ] Splash ramp and the dim status line.
- [ ] Density pass across those screens.
- [ ] Update the installer frame guard's reservations and regenerate the goldens.

### V2 — the trainer screens
- [ ] Trainer menu: the row bar, per-module meters, the status word plus glyph, the detail panel behind
      a gutter.
- [ ] Exercise and boss: header carrying mode, position, score and streak (and for the boss, lives and
      the countdown as a chip); Mission, Code and Answer as panelled blocks; the code window keeping
      exactly as much room as the chrome leaves.
- [ ] Result and boss result: panels for the summary, the explanation and the buffer preview, with the
      score in the header rather than a stacked line.
- [ ] Update the trainer frame guard's reservations, which assert exactly 24 rows when the code
      overflows, and regenerate the trainer goldens.

### V3 — close out
- [ ] Documentation for the new look, and the changelog entry.
- [ ] Final verification, then merge.

## Acceptance criteria
1. [ ] Every screen has the header, the rules and the footer, and the header carries that screen's
   vital sign rather than a decoration.
2. [ ] A selected row reads as a bar at the same indent as its neighbours, and nothing is distinguished
   by colour alone.
3. [ ] Blocks read as objects behind a gutter, with one blank line between them and none at the end.
4. [ ] The same information occupies fewer rows than before on the screens with the most content.
5. [ ] Both frame guards pass at 80x24, and every moved golden is explained.
6. [ ] `go test ./...`, `gofmt` and `go vet` clean before the push.

## Progress
- 2026-09-29: Feature opened. The user asked for a real aesthetic jump after the two measurement-driven
  slices; the audit's advice of restraint was written for that earlier scope, and this scope is the
  deliberate opposite, so the design decisions above are recorded before any code moves.
