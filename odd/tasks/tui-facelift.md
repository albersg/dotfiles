# ODD Feature Tasks: Installer facelift

## Goal
Review every screen the installer renders and make it fit the terminal it claims, read clearly on any
background, and say what it means, without dropping anything a user needs.

## Problem and rationale
The ask was "review the whole installer and give it a facelift", which is open-ended, so it started
with a measured audit of every screen rather than with taste. The audit found defects:

- **Nine installer screens and the whole trainer exercise/boss family did not fit 80x24.** The trainer
  rendered about 22 plus one row per code line, and the corpus has a lesson with 7 code lines and boss
  steps with 14, which is 35 rows in a 24 row terminal. A restore confirmation reached 39 rows and the
  error screen 127 columns.
- **The test that should have caught it could not**, because it asserted that two labels appear and
  both sit at the top.
- **Long lines were clipped silently**, code and explanation alike, mid-word.
- **The installer was unreadable on a light terminal**: near-white text with no background set.
- **Help notation was written six ways across twelve screens**, with some screens advertising keys
  their handlers do not accept and others hiding keys that work.
- **Two pieces of state were carried by emoji alone**, so a terminal without an emoji font lost the
  meaning of a lock and of the remaining lives.
- **Copy read like an internal ticket**, and several branches named a fact and no next step.

## Accepted decisions
- **Fitting is not enough; nothing may be dropped to fit.** The audit's frame guards can only prove
  that a screen is small enough, so the final verification compared every touched screen against
  `main` by hand and listed what disappeared. That is what caught the unreachable scroll tail.
- **Two guards, one per family**: every lesson and boss step of the trainer, and every installer screen
  state, both rendered at the frame floor with an assertion on height, width and the start cursor.
- **Adaptive colours rather than custom detection**: every dark value kept, light variants added, and
  no screen has to know which background it is on.
- **State is never carried by colour or by an emoji alone**: a glyph plus a word, everywhere.
- **One help notation, defined once**, including the leader-mode banner, which takes over the help line
  instead of being appended to every screen.
- Sequence: trainer first (the only defect with content actually lost), then the rest, then contrast,
  copy and documentation, because string edits re-pad widths and are cheapest to fold into snapshots
  that already moved.

## Outcome
- Commits on `feat/trainer-facelift`, merged as pull request `#42` with every CI check green.
- The guards went from failing 170 of 244 trainer screens and 9 of 43 installer screens to passing all
  of them, at a worst case that fell from 39 rows and 127 columns to 24 rows and 80 columns.
- Ten snapshots moved, every one explained by a rendering change rather than regenerated around a bug.
- The final verification found one blocking regression the guards could not see, because it only
  appeared once a list was scrolled: the shared window centred on the cursor while the handlers clamped
  a top offset, which made the last two to seven entries of the longest keymap categories unreachable.
  Fixed, with three tests that render the last entry rather than the first.

## Left alone, with the reason
- The splash art and its measured height threshold: guarded by geometry tests and a deliberate
  decision, so redrawing it is a rewrite with no structural return.
- The `✓ ● ○` step glyphs and the `▸ ` selection marker: they are why the interface survives a terminal
  without colour.
- The option-separator sentinel, which six key handlers test for by prefix.
- The non-interactive output and the Docker end-to-end suite, which are not user-visible in the TUI.

## Not verified
- The light-background palette is proven by contrast computation, not rendered on a real light
  terminal; no test exercises it.
- The guards pin the 80x24 floor; narrower widths rely on the existing truncation.
- The guards assert fit, not content, which is exactly why the hand comparison against `main` existed.
