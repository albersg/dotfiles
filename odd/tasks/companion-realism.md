# Companion realism (slice 2)

The user's words: the creature is much better than the glyph one, but they want it **a little more
realistic** - the eyes, the movement - and they want it to occupy **constant space** (that part is slice 1,
`odd/tasks/companion-constant-space.md`). This document is the design for the realism pass, so it can be
built the moment slice 1 lands without redeciding anything.

Everything here assumes slice 1's contract: the creature's rung is fixed by the terminal, its block is
reserved on every screen, and it never resizes or moves because of what is on screen.

## What exists today (measured, not remembered)

- A 32x24 half-block raster drawn in twelve rows (24x16 in eight), from metaballs. Step 1 adds distinct
  skull, muzzle, neck, chest and haunch masses, triangular ears, four legs and a tapered three-segment tail.
- Lambert shading from the upper left, ordered dithering, a drop shadow and the existing five-step ramp;
  Step 1 does not change the light model.
- A four-pose walk, a bob, a lean, a tail counter-sway, a head and pupil turn; a blink every ten seconds.
- Pupils: three columns by two rows inside the head.
- After Step 1, `TestCompanionCostHasTwoRegimes` reports 0 bytes at rest and, at 227 columns, 19 moving
  frames over 2.38 s, 158228 bytes total and 8852 bytes in the widest changed lines (69.2 KB/s at 8 fps).
  `BenchmarkCompanionVolumeFrame` reports 1,557,471 ns/op, 304,295 B/op and 1040 allocs/op on Linux/amd64.

## The standard

Every item below is stated so a test can decide it, and none of it may break slice 1's block, the leak
guard, or the two cost regimes. The judge is still "does a person name it a cat at a glance", but the
assertions are structural, so a regression is a test failure rather than a matter of taste.

### 1. Anatomy - a silhouette someone can name

Parts, each with its own field contribution and its own tone band:

| Part | What it is | Notes |
| --- | --- | --- |
| Ears | two triangles above the skull, with a darker inner ear | hinged at the skull, not part of its ball |
| Skull | one rounded mass | the widest part of the head |
| Muzzle | a shorter mass in front and below the skull | carries the nose |
| Nose | the darkest one or two pixels | a landmark for the eye |
| Mouth | a two-pixel line under the nose | only at the largest rung |
| Eyes | a bright sclera with a dark pupil inside, plus a one-pixel highlight | see 3 |
| Neck | a narrowing between skull and chest | where ambient occlusion shows (see 2) |
| Chest and haunch | two masses, the haunch the larger | what makes a cat a quadruped rather than a blob |
| Legs | four, two pixels wide, with a darker paw pad | front and back pairs land in sequence |
| Tail | three pixels at the base tapering to one, with a curve and a tip | sways against the walk |
| Whiskers | three one-pixel lines a side | largest rung only; they vanish below twelve rows |

**Test**: structural assertions on the field and the raster - the two ear maxima sit above the skull's
maximum, the tail's pixel count per column decreases, the haunch's mass is the largest below the neck, each
leg reaches the ground line. Not a snapshot alone: a snapshot proves nothing moved, not that the parts exist.

### 2. Light and material

- Keep the Lambert term.
- Add a **rim light** on the silhouette's upper-left edge, so the creature separates from a dark terminal.
- Add **contact shading** (ambient occlusion) where parts meet: under the neck, under the body, between the
  legs. Two tones darker than the local surface.
- Keep the encoder's five-tone ramp, and keep it degrading readably: the encoder is not driven by a
  colour profile, so document and test its deliberate glyph fallback on 16-colour and colourless terminals.

**Test**: the tone histogram of a rendered frame has the four tones; the darkest tone's pixels fall inside
the documented contact regions; a rim tone appears on the upper-left boundary of the mask.

### 3. Eyes and gaze - the part the user named first

- Sclera of three by three pixels at the largest rung, two by two at the smaller ones, always with at least
  one pixel of sclera around the pupil, so the eye reads as an eye and not as a hole.
- A dark pupil of one by two that moves **inside** the sclera: three columns by two rows of positions, which
  is what the current dead zone already assumes.
- A one-pixel highlight, fixed relative to the eye, not to the gaze.
- A blink closes the eyelid - a line of the face's tone across the eye - rather than deleting the eye.
- The ears tilt one pixel toward the pointer, and at the extreme gaze positions the skull itself shifts one
  pixel. Both stay inside the block.

**Test**: over the whole gaze grid, the pixels that change against the neutral frame are exactly the pupil's
and the eyelid's, each pupil position keeps sclera on all sides, and the blink changes no other pixel.

### 4. Motion with weight

- **Walk**: a four-pose cycle (contact, down, passing, up), the body one pixel higher at passing, the head
  leading by one pixel, the paws landing in the order front-left, back-right, front-right, back-left, and the
  tail's sway lagging two frames behind the body.
- **Idle**: a breath (the chest one pixel, over about four seconds), an ear twitch roughly every twenty
  seconds, a tail-tip flick roughly every fifteen, the blink every ten.
- **Hop**: on a click, one frame of crouch, then one row of travel, then one frame of landing squash.
- The at-rest claim has to be restated precisely, because a breath is a repaint: **a still creature writes
  nothing between events**, and the breath is an event with its own period. Whatever the code ends up
  meaning by "at rest", the cost test must say it in those terms - a claim of zero bytes that a breath
  contradicts is exactly the kind of number this repository stopped tolerating.

**Test**: the pose sequence is asserted frame by frame (the ground contact of each paw, the body's row, the
tail's lag); the idle's events are asserted by their periods, with the clock pinned as the repository's
snapshot harness does.

### 5. Budget, block and leaks

- Keep a ceiling on the frame's cost, not only a measurement: the benchmark exists
  (`BenchmarkCompanionVolumeFrame`), so give it a bound the way the other benchmarks in this repository have
  one, and state the number in the docs next to the test that prints it.
- A bounding-box guard: no pixel of the creature is drawn outside its reserved block, in any pose, at any
  rung. Slice 1 reserves the block; this proves it is respected.
- The leak guard (`TestNoRenderedLineLeavesAColourActive`) keeps covering the creature's screens.
- No new third-party dependencies: the encoder stays ours for the reasons the docs already record.

## Order of work

1. Anatomy first: the parts and the structural assertions, on the current light model. A cat that is a
   recognisable cat, even flat-shaded, is the whole point.
2. Then the light: rim and contact shading, with the histogram assertions.
3. Then the eyes and the gaze, which is the item the user named.
4. Then the motion, and the restatement of the at-rest claim.
5. Then the budget ceiling and the bounding-box guard, and the documentation pass that ties every figure to
   the test that prints it.

## Step 1 outcome: anatomy

Implemented in `installer/internal/tui/companion.go`. The volume now has a rounded skull and shorter
muzzle, a separate narrowing neck, chest and larger haunch, two triangular ears with darker inner-ear
raster pixels, four ground-reaching legs with dark paw pads, and a three-segment curved/tapered tail.
The face has a one-to-two-pixel dark nose, a two-pixel mouth on the full rung, and three one-pixel whisker
strokes on each side only on the twelve-row rung. Step 3 below replaces the original sclera/pupil renderer
with the rung-specific raster contract.

`TestCompanionAnatomyIsStructural` pins the ear maxima above the skull, haunch mass against chest/body/
neck, each leg against the ground line, inner-ear and paw-pad raster pixels, nose and mouth landmarks,
full-rung whiskers, and a non-increasing tail pixel count from the base column to the tip. Deletion experiments were run and
restored:

- With the second ear removed, the test failed verbatim: `companion_test.go:2227: ear 1 maximum 0.000 is not above skull maximum 1.205`.
- With the haunch removed, the test failed verbatim: `companion_test.go:2233: haunch 0.0000 is not largest (chest 0.0840 body 0.1092 neck 0.0375)`.

The sole moved snapshot is `installer/internal/tui/testdata/TestCompanionGoldenPinsThePixelSpriteAndItsGaze.golden`:
its pixel silhouette and its ANSI foreground/background runs changed with the new field masses and facial
landmarks. The glyph snapshots did not move. The ANSI snapshot was read after regeneration; each sprite
line ends with a reset, and `TestNoRenderedLineLeavesAColourActive` remains the executable escape-state
guard.

Measured after the anatomy change: rest is 0 bytes; walking at 227 columns changes 12 lines, over 19
moving frames/2.38 s, with 158228 bytes total, a maximum 8852-byte changed-line cost and 69.2 KB/s at
8 fps (`TestCompanionCostHasTwoRegimes`). `BenchmarkCompanionVolumeFrame`: 1,557,471 ns/op,
304,295 B/op, 1040 allocs/op. These replace the earlier baseline figures; the tests/benchmark above are
the sources for remeasurement. The ladder, rung heights and block remain unchanged.

## Step 2 outcome: light and material

Implemented in `installer/internal/tui/companion.go`. The existing Lambert diffuse term and fixed 2×2
Bayer dithering remain. The rim term now contributes only on upper-left-facing boundary normals. Ambient
occlusion darkens by exactly two ramp steps in named normalized-coordinate regions under the neck, under
the body and between the legs. `TestCompanionLightTerms` checks the rendered five-step histogram
(`[70 98 82 75 112]` in its measured full idle frame), darkest pixels within the documented contact
regions, an upper-left rim contribution and no lower-right rim contribution. It verifies the lighting
terms against no-rim/no-contact controls, so deleting either term fails multiple assertions.

The encoder cannot accept a colour profile: `pixelSpriteAllowed` selects the volume tier only for
`termenv.TrueColor`. `TestCompanionVolumeIsRefusedWithoutTrueColour` rejects ANSI/16-colour, ANSI256 and
ASCII profiles, and `TestCompanionVolumeSpriteIsTheLadderTopSteps` pins the unchanged glyph fallback.
There is no claim of five-tone shading on those profiles; Termux and colourless terminals keep the
existing cat glyphs without loss of meaning.

Measured by `TestCompanionCostHasTwoRegimes` at 227 columns: 0 bytes at rest; while walking, 12 sprite
rows, up to 12 changed lines, 19 moving frames/2.38 s, 168177 bytes total, a widest changed-line cost
of 9479 bytes, and 74.1 KB/s at 8 fps. `BenchmarkCompanionVolumeFrame` on Linux/amd64 reports
1,457,641 ns/op, 357,321 B/op and 1079 allocs/op. The latter measurements are observations, not portable
limits. `TestCompanionGoldenPinsThePixelSpriteAndItsGaze` is regenerated because AO and the directional
rim change the sprite's shaded pixels and ANSI foreground/background runs; glyph snapshots do not move.

The sentence above requesting a four-tone ramp was written before the encoder existed and is corrected:
the encoder prints a five-tone ramp. Reducing it would remove shading and break the dither tuning.

## Step 3 outcome: eyes and gaze

Implemented in `installer/internal/tui/companion.go` without changing the ladder, thresholds, head size,
light model or reserved block. Each full-rung eye is 3×4 with a vertical 1×2 slit pupil; the small rung
retains a 2×2 eye and 1×1 pupil. **At full size, the slit has a sclera column on both sides at all three
long-axis positions. The central position has a full ring; at the up/down extremes the slit touches the
corresponding lid edge.** At the small rung, the 1×1 pupil keeps horizontal and vertical sclera
adjacency. The pupil never touches the face outline. Vertical gaze selects pupil rows 0–1, 1–2 or 2–3;
horizontal gaze is the one-pixel skull shift and ear tilt, not a sideways pupil movement.

`TestCompanionVolumeEyePupilAdjacencyByRung` asserts the 3×4 size, both sclera side columns at all three full-rung positions, the full ring at centre, and the expected lid contact at the extremes.
`TestCompanionEyeGazeIsolationAndBlinkPinsTheFaceContract` asserts central-gaze eye-box isolation,
the stationary one-pixel highlight and a blink that changes only the face-tone eyelid row while retaining
sclera. `TestCompanionVolumeGazeTurnsTheHeadAndThePupils` checks extreme skull/ear motion and that every
changed pixel is confined to the named EYE BOX or HEAD OUTLINE BAND. The raster grid remains exactly the
rung's reserved dimensions. The 10-second blink period is unchanged.

Two fault experiments were run and restored. Moving the highlight with vertical gaze failed verbatim:
`companion_test.go:2770: eye 0 highlight moved with gaze; fixed upper-left highlight not preserved`. Disabling
the out-of-region clamp failed verbatim: `companion_test.go:2851: extreme gaze changed (20,0) outside EYE BOX and HEAD OUTLINE BAND: ramp4 -> none`.

The only moved golden is `installer/internal/tui/testdata/TestCompanionGoldenPinsThePixelSpriteAndItsGaze.golden`.
Its pixel shape changed around the face from the new sclera, pupils, fixed glints, lids and extreme gaze;
its ANSI foreground/background escapes changed with those tone locations. I inspected the shape and escapes:
the sprite lines end in resets, and `TestNoRenderedLineLeavesAColourActive` remains the escape guard. All
glyph snapshots stayed unchanged.

Measured by `TestCompanionCostHasTwoRegimes` at 227 columns: **0 bytes at rest**; walking changes 12 sprite
rows, up to 12 lines, 19 moving frames/2.38 s, **166735 bytes** total, **9235 bytes** widest changed lines,
**72.1 KB/s** at 8 fps. Three `BenchmarkCompanionVolumeFrame` runs on Linux/amd64 measured
**666939–723914 ns/op, 360033–360035 B/op and 1088 allocs/op**. These are test/benchmark observations, not portable limits. The full
suite passed 3877 tests; both frame guards, the block, anatomy, light, leak, welcome lockup and cost guards
were also run directly and passed.

## Step 4 outcome: motion with weight

Implemented in `installer/internal/tui/companion.go`, with geometry/tick assertions in
`installer/internal/tui/companion_test.go`. The walk is contact, down, passing and up, one model tick
per pose (four frames, 0.50 seconds at 8 fps, printed by `TestCompanionWalkPoseSequencePinsContactsAndLag`).
That test asserts the paw touchdown order front-left, back-right, front-right, back-left, the body's
one-pixel rise at passing, the head's one-pixel lead one pose earlier, and the tail's two-pose delay.
The old two-ticks-per-pose arithmetic would have made this cycle eight frames / one second; it is
corrected because the plan calls for a four-pose cycle and the tick arithmetic now gives each named
pose exactly one frame.

Idle motion is a set of independent one-frame events on `AnimTick`: breath 32 frames / 4 seconds,
ear twitch 160 / 20 seconds, tail-tip flick 120 / 15 seconds, blink 80 / 10 seconds. The periods come
from duration constants times the model's 8 fps (`TestCompanionIdleEventsHaveIndependentPeriods`
and `TestCompanionCostHasTwoRegimes` print/assert them). Raster assertions prove each event changes
pixels and every tick between event boundaries leaves geometry and raster unchanged. The hop countdown
is three frames: anticipation crouch, one terminal row of travel (two raster pixels), landing squash,
then settled; the fixed raster and
rendered block do not grow, and `TestCompanionClickHopsAndCelebrates` proves no non-companion row moves.

Two defect experiments were run and restored. Swapping the paw phase order failed verbatim:

```text
companion_test.go:3574: pose 2 paw 1 ground contact=false, want true in landing order [0 3 1 2]
companion_test.go:3574: pose 3 paw 1 ground contact=true, want false in landing order [0 3 1 2]
companion_test.go:3574: pose 2 paw 2 ground contact=true, want false in landing order [0 3 1 2]
companion_test.go:3574: pose 3 paw 2 ground contact=false, want true in landing order [0 3 1 2]
--- FAIL: TestCompanionWalkPoseSequencePinsContactsAndLag (0.00s)
```

Making tail sway in phase failed verbatim:

```text
companion_test.go:3589: tail sway at pose 0 is 0.000, want body sway delayed two poses (0.900)
companion_test.go:3589: tail sway at pose 1 is -0.900, want body sway delayed two poses (-0.000)
companion_test.go:3589: tail sway at pose 2 is 0.900, want body sway delayed two poses (-0.000)
companion_test.go:3589: tail sway at pose 3 is 0.000, want body sway delayed two poses (-0.900)
--- FAIL: TestCompanionWalkPoseSequencePinsContactsAndLag (0.00s)
```

No companion snapshots moved: the glyph captures pin idle frames at ticks 0, 3 and 6, all before an
event boundary; the pixel capture also pins an idle tick. I inspected all four current goldens in both
shape and escapes: the three ASCII shapes remain unchanged, and the pixel raster's shape and ANSI
runs match the current idle/gaze model with a reset at the end of its sprite lines. No golden was
regenerated because no captured frame changed. `TestNoRenderedLineLeavesAColourActive` remains the
executable escape-state guard.

Measured by `TestCompanionCostHasTwoRegimes` at 227×62: still-between-events **0 changed bytes across
24 pinned ticks**; walking **12 sprite rows**, up to **12 changed lines**, **19 moving frames of 19 / 2.38 s**,
**166586 bytes**, **9138 widest-line bytes**, **71.4 KB/s at 8 fps**. `BenchmarkCompanionVolumeFrame`
measured **2,179,233–4,199,595 ns/op, 359,912–359,928 B/op, 1087 allocs/op** over three
Linux/amd64 runs; the observed time varied in the shared environment.
The cost wording is deliberately no longer "zero bytes at rest" across all idle time: each named event
repaints the sprite.

## Step 5 outcome: budget, block and leaks

Implemented in `installer/internal/tui/companion_test.go`, with the budget and block contract documented
in `docs/tui-installer.md`. `BenchmarkCompanionVolumeFrame` now enforces 10,000,000 ns/op,
450,000 B/op and 1,400 allocs/op ceilings. The time ceiling leaves more than 2× headroom over the
slowest recent measured run (4,199,595 ns/op); byte and allocation ceilings leave about 25% over the
stable ~360 KB / 1087–1088 allocs measured footprint. The bounds catch a fivefold allocation
regression while allowing for shared-host timing and Go-version variance. They are not a cost guarantee
for terminals slower than the measured host.

`TestCompanionRenderedPosesStayInsideTheirReservedBlock` asserts on rendered rows—not only raster
fields—in both sprite modes: volume full (12 rendered rows), volume small (8), glyph full (5), glyph
compact (3), and the trainer's one-row floor. For each rung it covers the four walk ticks, breath, ear
twitch, tail flick, all three hop phases, the left/up and right/down gaze extremes, and blink. Each
state must retain exactly the baseline block rows and framed blocks must remain immediately above the
frame rule. The existing `TestCompanionBlockDependsOnlyOnTheTerminal`, both frame guards,
`TestCompanionCostHasTwoRegimes`, anatomy/light/eye/gaze/walk/idle/hop guards and
`TestNoRenderedLineLeavesAColourActive` remain intact. No creature geometry, rung, block, lighting or
motion changed.

The five plan steps are complete; implementation commits are recorded here. Steps 1–4 are the existing
commits, and step 5's commit ID is intentionally pending the parent transaction controller, which owns
the commit:

| Step | Commit |
|------|--------|
| 1. Anatomy | `9cf2bcf` |
| 2. Light and material | `217fe65` |
| 3. Eyes and gaze | `fc3ecf5` |
| 4. Motion with weight | `848e7de` |
| 5. Budget, block and leaks | pending parent commit |

**Not done:** raster-buffer optimization. Approximately 293 KB per frame was measured and declared as
future optimization work, not implemented in this realism pass. The benchmark does not guarantee cost
on a terminal slower than the measured Linux/amd64 host.
