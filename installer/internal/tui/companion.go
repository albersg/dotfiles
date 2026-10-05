package tui

import (
	"image/color"
	"math"
	"os"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// ============================================================================
// THE COMPANION
// ============================================================================
//
// The installer's screens are wide and mostly empty below their content: at the
// 227x62 the layout was measured on, the main menu occupies 8 of 62 rows. The
// panels fill some of that room with facts; the companion is the one element in
// it that is not information. It is a small creature that walks the rows the body
// did not need, toward whatever the cursor points at, looks at it, sleeps when the
// user stops typing and reacts to what is on screen. It walks only when it has
// somewhere to go: a creature with nothing to do stands still, so a screen nobody
// is typing into settles into bytes the renderer never has to repaint.
//
// The art is drawn here, in the three tables below, and it is deliberately plain
// ASCII. Termux, a 16-colour terminal and a terminal with no font fallback all
// draw it, and the state has to read from the glyphs rather than from a colour:
// the eyes and the props say asleep before any tone does. Nothing was copied from
// a third-party mascot either -- the Go gopher is CC-BY, cowsay's cow is GPL-ish
// and nyancat's cat belongs to its author -- so this repository's attribution
// surface stays empty.
//
// There are three sets of art and one ladder that picks between them. The body of
// a screen is laid out first and the rows it did not need are what the summary and
// the creature share; companionHeight chooses the tallest sprite those rows can
// hold -- five rows, else three, else the one-row art the creature shipped with,
// else nothing -- so a small terminal keeps a smaller creature instead of losing
// it, and a screen whose body fills its frame still shows no creature at all. The
// panel summary is placed first and keeps its rows: when the spare rows cannot
// hold both, the facts win and the creature is the one that gives way.
//
// THE TRAINER'S FLOOR IS A STATED EXCEPTION. The trainer composes its own rows
// rather than going through frameWithPanels, and at or below trainerFloorHeight
// (the documented 80x24 floor) trainerCompanionRows reserves the legacy one-row
// mini whatever the ladder would have tried. That rung is not an accident: the
// trainer's one-column body at the floor fills the frame and leaves exactly ONE
// spare slot, so the three-row compact rung has nowhere to stand, and the 80x24
// trainer goldens are pinned to the one-row mini. Above the floor the trainer
// runs this same ladder unchanged. TestTrainerCompanionFloorIsADocumentedException
// pins that reading -- one row at the floor, a taller rung above it -- so the
// exception is a stated rule with a guard rather than a deviation from the
// documented ladder.
//
// The gaze is composed rather than drawn: every frame table holds one neutral
// frame per state, and companionGazeRows moves the pupil pair one column left or
// right and, where the art has a second eye row, up to it. Those two cells are the
// only characters the gaze touches, which is what keeps six or seven states and
// three gaze columns from becoming thirty hand-drawn frames.
//
// Nothing here reads the clock. The frame comes from the model's tick counter, the
// cell from CompanionPos and the gaze from CompanionGaze, so the same model and
// tick render the same bytes on every run and a snapshot can pin a frame instead
// of flaking on time. The render path is pure: companionSprite draws from the
// model and moves nothing, and advanceCompanion, called only from the frame tick
// (animTickMsg), is the one writer of everything below.
//
// Cost has two regimes and a test for each, rather than a promise in this comment.
// A still creature writes nothing between events; its breath, ear twitch, tail-tip
// flick and blink each have their own model-tick period. Walking repaints its owned
// rows as the cell moves. TestCompanionIdleEventsHaveIndependentPeriods pins the
// event clocks, TestCompanionTicksChangeOnlyItsOwnRows bounds row ownership, and
// TestCompanionCostHasTwoRegimes measures the terminal's actual line cost.
//
// With the animation gate off there is no companion anywhere: a frozen pet is not
// the point, and the gate already means "this run cannot animate".

// The creature's cells, its pace and its clocks. Each is a constant with a test
// beside it instead of a number buried in the arithmetic.
//
// Two kinds of number live here, and the name says which one it is. A DURATION
// is written in seconds (fooSeconds) and turned into frames through
// animTicksPerSecond, so moving the frame rate cannot silently change how long
// the creature sleeps or celebrates. A frame count is written in frames
// (fooTicks); it is derived from a duration wherever what it measures is a length
// of time, and written directly only where the thing counted really is frames.
const (
	// companionFullHeight, companionCompactHeight and companionMiniHeight are the
	// three heights the ladder can choose, in the order it tries them: the cat, the
	// cat's head, and the one-row creature the companion shipped with. They are the
	// steps of the fallback, not a preference.
	companionFullHeight    = 5
	companionCompactHeight = 3
	companionMiniHeight    = 1

	// companionFullWidth and companionCompactWidth are the columns each of the two
	// taller cells occupies: the widest row in its table, which is the row that
	// carries a prop. The mini cell is the one the shipped art already used.
	companionFullWidth    = 14
	companionCompactWidth = 14
	companionMiniWidth    = 7

	// companionCellWidth is the widest cell any height draws, and the one the walk
	// uses to bound the stage. Fixing the walk on the widest cell rather than on
	// the drawn one means no height can walk its sprite past the edge of the stage:
	// the narrower glyph cells and the smaller volumetric ones stop short of the right
	// edge instead of reaching it, which is invisible, where the full volumetric
	// sprite reaching past it would cross the frame's margin.
	companionCellWidth = companionVolumeFullWidth

	// companionStepCells is the walk's only speed: at most one cell per animation
	// frame, whatever the distance left. A step proportional to what remains is
	// what made the creature read as a jump -- a long move was thirty-five cells in
	// one frame and then a crawl for the last two -- so the step is capped instead
	// and a long walk is simply a walk, at a constant companionStepCells *
	// animTicksPerSecond = eight cells a second, with a known number of frames.
	companionStepCells = 1

	// companionBrakeCells is how far from its target the creature starts slowing
	// down, and companionBrakeFrames is the cadence it uses there: the last few
	// cells are crossed on every other frame, so it arrives in small steps instead
	// of stopping dead. Braking cannot be a fraction of a cell -- the grid is whole
	// cells -- so it is a frame count: one step every companionBrakeFrames.
	companionBrakeCells  = 3
	companionBrakeFrames = 2

	// companionFollowCells is how far the creature walks for one row of a menu.
	// Spreading the menu's whole index across the stage is what threw it thirty-five
	// cells for one arrow key on a wide terminal; walking a fixed three cells per
	// row makes one keypress a short stroll and a jump to the last row a walk of a
	// few seconds. It also keeps the creature near the menu it is following instead
	// of out in the middle of a 227-column stage on its own.
	companionFollowCells = 3

	// companionSleepSeconds is how long without a key puts it to sleep. It is a
	// duration: the stretch stays twenty seconds if the frame rate ever moves.
	companionSleepSeconds = 20

	// companionSleepTicks is that stretch in frames, the value the model's idle
	// counter is compared against. It is counted on the model, not in the
	// renderer, so a snapshot can pin the sleeping frame without waiting.
	companionSleepTicks = companionSleepSeconds * animTicksPerSecond

	// companionYawnSeconds is how long before sleeping the creature yawns: the last
	// stretch of the quiet run is drawn as the tired state instead of the idle one,
	// so falling asleep reads as a transition rather than a cut. It is a duration
	// for the same reason the sleep is.
	companionYawnSeconds = 2

	// companionYawnTicks is that stretch in frames.
	companionYawnTicks = companionYawnSeconds * animTicksPerSecond

	// companionBlinkSeconds is how often an awake, idle creature blinks. It is ten
	// and not five because every blink rewrites the creature's rows: a blink is a
	// decoration, and once it is frequent enough to notice it is noise in the middle
	// of whatever the reader is reading. The blink lasts one frame and is derived
	// from the tick counter rather than from a second clock, so a snapshot can pin
	// the blink without waiting for it.
	companionBlinkSeconds = 10

	// companionBlinkTicks is that interval in frames.
	companionBlinkTicks = companionBlinkSeconds * animTicksPerSecond

	// companionPleasedSeconds is how long a finished installation step is worth
	// celebrating: about a second, not forever.
	companionPleasedSeconds = 1

	// companionPleasedTicks is that celebration in frames.
	companionPleasedTicks = companionPleasedSeconds * animTicksPerSecond

	// companionHopTicks counts anticipation, lift and landing, one pinned frame each.
	companionHopTicks = 3

	// Idle motions are one-frame events. Their periods are durations converted by
	// the model's animation clock; frames between these boundaries are identical.
	companionBreathSeconds    = 4
	companionEarTwitchSeconds = 20
	companionTailFlickSeconds = 15
	companionBreathTicks      = companionBreathSeconds * animTicksPerSecond
	companionEarTwitchTicks   = companionEarTwitchSeconds * animTicksPerSecond
	companionTailFlickTicks   = companionTailFlickSeconds * animTicksPerSecond
)

// companionState is the state the creature is drawn in. The reactions come
// before the resting states, which is the precedence the design states: a
// reaction wins over the idle state, so a sleeping companion that must flinch
// flinches.
type companionState int

const (
	companionIdleState companionState = iota
	companionWalkingState
	companionBlinkingState
	companionAsleepState
	companionYawningState
	companionAlertState
	companionPleasedState
	companionFlinchingState
)

// companionStateNames names every state, in the order they are declared. A state
// added without a name fails TestCompanionStatesAreNamed rather than printing as
// a number in a failure message.
var companionStateNames = map[companionState]string{
	companionIdleState:      "idle",
	companionWalkingState:   "walking",
	companionBlinkingState:  "blinking",
	companionAsleepState:    "asleep",
	companionYawningState:   "yawning",
	companionAlertState:     "alert",
	companionPleasedState:   "pleased",
	companionFlinchingState: "flinching",
}

// ============================================================================
// THE GAZE
// ============================================================================

// companionGaze is where the creature is looking: horizontal direction (-1 left, 0 centre,
// +1 right) and vertical direction (-1 up, 0 level, +1 down). The volume uses the three
// vertical values to place its pupil along the eye's long axis; horizontal direction turns
// the head. The glyph art retains its older two-row up/level composer.
type companionGaze struct {
	X, Y int
}

// companionGazeDeadZone is how many columns either side of the creature's own
// cell still count as looking straight at the thing it wants. A pupil pair that
// followed every column of a fourteen-column body would flicker; two columns is
// the dead zone the design asks for and it is a constant with a test beside it so
// it cannot drift.
const companionGazeDeadZone = 2

// companionSelectionRow is the row the selection is on, counted from the
// creature's own row. A menu row is in the body and the creature walks the last
// row the body did not need, so the thing it looks at is always above it. It is a
// named constant rather than a computed row because the placement does not know
// the body's absolute rows; the pointer task passes its own row here instead and
// nothing else in the gaze changes.
const companionSelectionRow = -1

// companionGazeFor turns the cell the creature is looking at into a gaze. It is
// pure -- no model, no clock, no layout -- so every gaze can be pinned by a test.
// The volume maps above/level/below to three positions along its vertical slit; the
// horizontal value is expressed by the skull and ear turn.
func companionGazeFor(targetCol, targetRow, anchorCol int) companionGaze {
	gaze := companionGaze{}
	switch {
	case targetCol < anchorCol-companionGazeDeadZone:
		gaze.X = -1
	case targetCol > anchorCol+companionGazeDeadZone:
		gaze.X = 1
	}
	switch {
	case targetRow < 0:
		gaze.Y = -1
	case targetRow > 0:
		gaze.Y = 1
	}
	return gaze
}

// companionAsleepNow reports whether the quiet stretch has put the creature to
// sleep. The tick reads it to stop the creature walking, and the state machine
// reads it to draw the sleeping face; both read it here so the two cannot drift.
func (m Model) companionAsleepNow() bool {
	return m.CompanionIdle >= companionSleepTicks
}

// aimCompanion points the creature's gaze at whatever it should be looking at:
// the pointer when this run asked the terminal for one and has seen it, and the
// selection otherwise, which is what the gaze did before the pointer existed. It
// is called from the places that can change what the creature sees -- the model's
// own construction, a key that moved the selection or changed the screen, the tick
// that actually moved the creature, and a pointer event -- so the eyes settle when
// the thing they look at moves, never on a tick that changed nothing. The render
// path draws the gaze and never computes it, and a snapshot can pin one.
func (m *Model) aimCompanion() {
	stage := companionStageWidth(*m)
	if col, row, ok := m.companionPointerTarget(stage); ok {
		m.CompanionGaze = companionGazeFor(col, row, m.CompanionPos)
		return
	}
	target, ok := m.companionTarget(stage)
	if !ok {
		m.CompanionGaze = companionGaze{}
		return
	}
	m.CompanionGaze = companionGazeFor(target, companionSelectionRow, m.CompanionPos)
}

// ============================================================================
// THE POINTER
// ============================================================================
//
// The gaze follows the pointer when the run asked the terminal for mouse motion,
// and the selection when it did not. That fallback is the whole reason the pointer
// is a source of the gaze rather than a replacement for it: a terminal that
// refuses mouse reporting, a run with the gate off, and a Termux session where the
// pointer is a finger all keep the creature looking at what is selected instead of
// losing the gaze entirely.
//
// The terminal reports the pointer in its own cell coordinates and knows nothing
// about the frame, so two numbers from the layout turn one into the other: the
// frame pads viewPaddingCols columns on the left, which is where the stage starts,
// and the creature draws in the last rows of the frame, which is the band the
// vertical gaze is measured against. Neither is resolved against the body's
// absolute rows, for the reason companionSelectionRow gives -- the placement does
// not know them, and a gaze that needed them would have to be computed inside the
// renderer, which is exactly where the creature is not allowed to compute.

// companionGroundShare is the share of the frame's height the creature's ground
// band takes from the bottom, as a divisor: the band is the bottom third of the
// screen, and a pointer above it is above the creature. It is a share of the
// height rather than a row count because the frame's footer and the panel summary
// move the creature's exact rows and the placement does not know them (see
// companionSelectionRow); a two-thirds line is below the creature at every size,
// while a fixed row count is only right at the size it was measured at.
const companionGroundShare = 3

// companionGroundTop is the first row of that band: the row above it is "above the
// creature" and the row itself is level with it.
func companionGroundTop(height int) int {
	return height * (companionGroundShare - 1) / companionGroundShare
}

// companionPointerTarget is the cell the pointer is on, in the creature's own
// coordinates: the column of the stage it points at, clamped to the row the
// creature can walk, and its row measured against the band the creature draws in,
// where a negative row means above the creature. It reports false when this run has
// no live pointer -- the gate is off, or no event has arrived yet -- and the gaze
// then falls back to the selection.
func (m Model) companionPointerTarget(stage int) (col, row int, ok bool) {
	if !m.Hovering || !m.PointerSet {
		return 0, 0, false
	}
	col = m.PointerCol - viewPaddingCols
	col = min(max(col, 0), max(stage-companionCellWidth, 0))
	row = m.PointerRow - companionGroundTop(m.Height)
	return col, row, true
}

// handleCompanionMouse is the pointer's whole input path: it remembers the cell,
// wakes the creature, turns the gaze, and treats a left click as an event of its
// own -- the celebration a finished step earns plus the little jump.
//
// Waking on motion is oneko's rule, and there is nothing clever to detect about a
// parked mouse: a parked mouse sends no events at all, so the only pointer event
// that exists is the user moving the mouse and every one of them is the sudden
// movement that wakes the cat. A jittery hand therefore keeps it awake, which is
// the honest reading of "the user is at the mouse".
//
// A wheel is ignored: the installer runs in the alternate screen, where there is no
// scrollback for it to move, and the creature has nothing to say about it. The
// events that are not handled here are dropped exactly as they were before the
// pointer existed.
func (m Model) handleCompanionMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// tea.MouseMsg is a named copy of the event rather than an alias, so the
	// predicate that knows what a wheel is lives on the underlying type.
	if !m.Hovering || tea.MouseEvent(msg).IsWheel() {
		return m, nil
	}
	m.CompanionIdle = 0
	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
		m.CompanionPleased = companionPleasedTicks
		m.CompanionHop = companionHopTicks
	}
	m.PointerCol, m.PointerRow, m.PointerSet = msg.X, msg.Y, true
	m.aimCompanion()
	return m, nil
}

// ============================================================================
// THE ART
// ============================================================================

// companionEyes locates a frame's pupils. row is the row that carries them and
// upper is the row they move to when the creature looks up, or -1 where the art
// has no eye row (the one-row art, whose face is three cells wide and has no
// column to move a pupil into) or only one (the compact art, whose head has room
// for a single eye row).
type companionEyes struct {
	row   int
	upper int
}

var (
	companionFullEyes    = companionEyes{row: 2, upper: 1}
	companionCompactEyes = companionEyes{row: 1, upper: -1}
	companionNoEyes      = companionEyes{row: -1, upper: -1}
)

// companionFrame is one frame of the art at one height: the state it draws, the
// rows it draws, and where its eyes are so the composer can move them. It is
// hand-written and reviewable on purpose -- only the eyes are computed.
type companionFrame struct {
	state companionState
	rows  []string
	eyes  companionEyes
}

// The rows the five-row cat is drawn from. They are named because several states
// share a row and because a frame reads better as one line per row below than as
// a wall of quoted text. Every row is exactly companionFullWidth columns wide, so
// the creature never jitters sideways as the eye reads it.
const (
	companionFullEars        = ` /\______/\   `
	companionFullEarsAsleep  = ` /\______/\  z`
	companionFullEarsAlert   = ` /\______/\  !`
	companionFullEarsPleased = ` /\______/\\o/`
	companionFullEyeEmpty    = `(          )  `
	companionFullEyeIdle     = `(  o    o  )  `
	companionFullEyeShut     = `(  -    -  )  `
	companionFullEyeAlert    = `(  O    O  )  `
	companionFullEyePleased  = `(  ^    ^  )  `
	companionFullEyeFlinch   = `(  >    <  )  `
	companionFullMouth       = `(     ^    )  `
	companionFullMouthYawn   = `(     o    )  `
	companionFullPaws        = ` >  <  >  <   `
	companionFullPawsStep    = `  >  <  >  <  `
	companionFullPawsIn      = `  > <  > <    `
)

// The rows the three-row head is drawn from, on the same terms.
const (
	companionCompactEars       = ` /\_____/\    `
	companionCompactEyeIdle    = `(  o   o  )   `
	companionCompactEyeShut    = `(  -   -  )   `
	companionCompactEyeAsleep  = `(  -   -  )z  `
	companionCompactEyeAlert   = `(  O   O  )!  `
	companionCompactEyePleased = `(  ^   ^  )\o/`
	companionCompactEyeFlinch  = `(  >   <  )!  `
	companionCompactMouth      = `  >  ^  <     `
	companionCompactMouthYawn  = `  >  o  <     `
	companionCompactMouthStep  = `  >  ^   <    `
	companionCompactMouthIn    = `  >  ^ <      `
)

// companionFullFrames is the whole five-row art: the cat with a body, one frame
// per state and two for the walk. The walk's two frames differ in the paws row
// (legs apart, legs in) and the idle frame is a third stance, so no two states
// draw the same frame and the alternation is visible without colour.
var companionFullFrames = []companionFrame{
	{companionIdleState, []string{companionFullEars, companionFullEyeEmpty, companionFullEyeIdle, companionFullMouth, companionFullPaws}, companionFullEyes},
	{companionWalkingState, []string{companionFullEars, companionFullEyeEmpty, companionFullEyeIdle, companionFullMouth, companionFullPawsStep}, companionFullEyes},
	{companionWalkingState, []string{companionFullEars, companionFullEyeEmpty, companionFullEyeIdle, companionFullMouth, companionFullPawsIn}, companionFullEyes},
	{companionBlinkingState, []string{companionFullEars, companionFullEyeEmpty, companionFullEyeShut, companionFullMouth, companionFullPaws}, companionFullEyes},
	{companionAsleepState, []string{companionFullEarsAsleep, companionFullEyeEmpty, companionFullEyeShut, companionFullMouth, companionFullPaws}, companionFullEyes},
	{companionYawningState, []string{companionFullEars, companionFullEyeEmpty, companionFullEyeShut, companionFullMouthYawn, companionFullPaws}, companionFullEyes},
	{companionAlertState, []string{companionFullEarsAlert, companionFullEyeEmpty, companionFullEyeAlert, companionFullMouth, companionFullPaws}, companionFullEyes},
	{companionPleasedState, []string{companionFullEarsPleased, companionFullEyeEmpty, companionFullEyePleased, companionFullMouth, companionFullPaws}, companionFullEyes},
	{companionFlinchingState, []string{companionFullEarsAlert, companionFullEyeEmpty, companionFullEyeFlinch, companionFullMouth, companionFullPaws}, companionFullEyes},
}

// companionCompactFrames is the whole three-row art: the cat's head, with the
// same states and the same walk. Its head has room for one eye row, so the gaze
// is horizontal only here.
var companionCompactFrames = []companionFrame{
	{companionIdleState, []string{companionCompactEars, companionCompactEyeIdle, companionCompactMouth}, companionCompactEyes},
	{companionWalkingState, []string{companionCompactEars, companionCompactEyeIdle, companionCompactMouthStep}, companionCompactEyes},
	{companionWalkingState, []string{companionCompactEars, companionCompactEyeIdle, companionCompactMouthIn}, companionCompactEyes},
	{companionBlinkingState, []string{companionCompactEars, companionCompactEyeShut, companionCompactMouth}, companionCompactEyes},
	{companionAsleepState, []string{companionCompactEars, companionCompactEyeAsleep, companionCompactMouth}, companionCompactEyes},
	{companionYawningState, []string{companionCompactEars, companionCompactEyeShut, companionCompactMouthYawn}, companionCompactEyes},
	{companionAlertState, []string{companionCompactEars, companionCompactEyeAlert, companionCompactMouth}, companionCompactEyes},
	{companionPleasedState, []string{companionCompactEars, companionCompactEyePleased, companionCompactMouth}, companionCompactEyes},
	{companionFlinchingState, []string{companionCompactEars, companionCompactEyeFlinch, companionCompactMouth}, companionCompactEyes},
}

// companionMiniFrames is the one-row art the creature shipped with, kept exactly
// as it was. It is the ladder's last step: where the body leaves one spare row
// this is what the screen draws, so no screen loses the creature it has today.
// Its face is three cells wide, which is why it carries no eyes: there is no
// interior column to move a pupil into, and the walk reads from the leg instead.
var companionMiniFrames = []companionFrame{
	{companionIdleState, []string{`(o.o)`}, companionNoEyes},
	{companionWalkingState, []string{`(o.o)/`}, companionNoEyes},
	{companionWalkingState, []string{`(o.o)\`}, companionNoEyes},
	{companionBlinkingState, []string{`(-.-)`}, companionNoEyes},
	{companionAsleepState, []string{`(-.-) z`}, companionNoEyes},
	{companionYawningState, []string{`(-.-) v`}, companionNoEyes},
	{companionAlertState, []string{`(O.O) !`}, companionNoEyes},
	{companionPleasedState, []string{`\(o.o)/`}, companionNoEyes},
	{companionFlinchingState, []string{`(>.<)`}, companionNoEyes},
}

// companionFramesAt is the art at one height, in the ladder's own order. A height
// the ladder cannot choose -- anything but the three steps -- draws the one-row
// art rather than nothing, so a missing table is visible instead of silent.
func companionFramesAt(height int) []companionFrame {
	switch height {
	case companionFullHeight:
		return companionFullFrames
	case companionCompactHeight:
		return companionCompactFrames
	default:
		return companionMiniFrames
	}
}

// companionSpriteWidth is the columns the sprite's cell takes at one height: the
// widest row in that height's table, so a one-cell step is always one column and
// no row of the sprite reaches past the cell it is placed in. The volumetric
// heights answer with their own pixel width, which is one column per pixel.
func companionSpriteWidth(height int) int {
	if size, ok := companionVolumeSizeFor(height); ok {
		return size.width
	}
	switch height {
	case companionFullHeight:
		return companionFullWidth
	case companionCompactHeight:
		return companionCompactWidth
	case companionMiniHeight:
		return companionMiniWidth
	}
	return 0
}

// companionGlyphFullMinHeight is the terminal height at which the glyph ladder
// picks its full five-row cat. The mini volume is also five rows, so it takes over
// exactly that rung and shares this threshold rather than the quarter-bound: the
// mini replaces the five-row glyph cat from 25 to 31, and below 25 the frame has
// room only for the three-row compact head. A five-row volume cannot replace the
// compact head at 20-24 because the smallest volume whose eye survives the realism
// assertions is five rows (four loses the pupil), and forcing it there would drop
// the creature on the screens that only had room for three. A visible glyph cat is
// better than no pet, so 20-24 keeps the compact glyph even with the encoder on.
const companionGlyphFullMinHeight = 25

// companionHeightShare is the share of the terminal a rung may take, as a
// divisor: the sprite is decoration in the rows the body did not need, so a rung
// is only drawn where it fits without taking more than a quarter of the
// terminal. The thresholds are the height at which each rung's own row count
// reaches that share -- the twelve-row volume at 48 rows and the eight-row one at
// 32 -- so those boundaries come from the bound rather than from a taste for round
// numbers, and a taller sprite cannot crowd out the screen it decorates. The
// glyph cat's five rows already fit the bound at the height its own step names, so
// its threshold is unchanged; the miniature volume shares that same threshold
// (companionGlyphFullMinHeight) because it is the same five rows.
const companionHeightShare = 4

// companionHeightNow is the ladder with the volumetric sprite's three steps on
// top: the full sprite where the frame can hold it and the run may draw it, else
// the smaller one on the same terms, else the mini one where the glyph cat's full
// rung would have been, else the glyph ladder. The glyph ladder is untouched below
// them, which is what keeps the floor a floor: a terminal without true colour, a
// run with the sprite switched off and the heights where the frame has room only
// for the compact head all get the cat the glyph ladder picks, and every step
// degrades into the next rather than disappearing.
func (m Model) companionHeightNow() int {
	if m.PixelSprite {
		switch {
		case m.Height >= companionVolumeFullHeight*companionHeightShare:
			return companionVolumeFullHeight
		case m.Height >= companionVolumeSmallHeight*companionHeightShare:
			return companionVolumeSmallHeight
		case m.Height >= companionGlyphFullMinHeight:
			return companionVolumeMiniHeight
		}
	}
	if m.Height >= companionGlyphFullMinHeight {
		return companionFullHeight
	}
	return companionCompactHeight
}

// companionHeight is the ladder: the tallest sprite the rows the body did not
// need can hold, or zero when they cannot hold even the one-row frame. The
// summary's rows are taken off before it is asked, so the summary always keeps
// them -- a fact beats a decoration -- and the creature takes what is left.
func companionHeight(spare int) int {
	switch {
	case spare >= companionFullHeight:
		return companionFullHeight
	case spare >= companionCompactHeight:
		return companionCompactHeight
	case spare >= companionMiniHeight:
		return companionMiniHeight
	default:
		return 0
	}
}

// companionArtFor is the frame a state draws at a tick and height, with the gaze
// stamped in. A state with one frame always draws it; the walk has two and
// alternates them on the tick's parity, so the legs move on every tick it is
// walking. A state with no frame at all -- one a future edit adds without art --
// draws the first frame of that height rather than an empty row, so a missing
// frame is visible in a snapshot instead of silently blank.
func companionArtFor(state companionState, height, tick int, gaze companionGaze) []string {
	frames := companionFramesAt(height)
	var set []companionFrame
	for _, f := range frames {
		if f.state == state {
			set = append(set, f)
		}
	}
	if len(set) == 0 {
		set = frames[:1]
	}
	phase := tick % len(set)
	if phase < 0 {
		phase += len(set)
	}
	return companionGazeRows(set[phase].rows, set[phase].eyes, gaze)
}

// companionEyeRow is one row's eyes: the two pupil glyphs and the columns they
// sit at between the head's parentheses.
type companionEyeRow struct {
	left, right       rune
	leftCol, rightCol int
}

// companionEyesOf reads a row's eyes. A row without exactly two marks between its
// parentheses carries no eyes -- the one-row art's face is the case that matters
// -- and the composer then leaves the row alone.
func companionEyesOf(row string) (companionEyeRow, bool) {
	open := strings.IndexByte(row, '(')
	close := strings.LastIndexByte(row, ')')
	if open < 0 || close <= open {
		return companionEyeRow{}, false
	}
	var marks []int
	var glyphs []rune
	for i, r := range []rune(row[open+1 : close]) {
		if r != ' ' {
			marks = append(marks, i)
			glyphs = append(glyphs, r)
		}
	}
	if len(marks) != 2 {
		return companionEyeRow{}, false
	}
	return companionEyeRow{left: glyphs[0], right: glyphs[1], leftCol: marks[0], rightCol: marks[1]}, true
}

// companionBlankEyes returns an eye row with its pupils taken out, so a gaze that
// moves them to the other eye row leaves the row they came from empty instead of
// doubled. Every other character, the row's padding included, is the table's.
func companionBlankEyes(row string) string {
	open := strings.IndexByte(row, '(')
	close := strings.LastIndexByte(row, ')')
	if open < 0 || close <= open {
		return row
	}
	body := []rune(row[open+1 : close])
	for i := range body {
		body[i] = ' '
	}
	return row[:open+1] + string(body) + row[close:]
}

// companionEyesAt returns an eye row with the pupil pair one gaze column to the
// side. The only characters it writes are the two pupil cells; a gaze that would
// push a pupil past the head's own parentheses leaves the row untouched, so the
// eyes can never be drawn outside the face.
func companionEyesAt(row string, eye companionEyeRow, gazeX int) string {
	open := strings.IndexByte(row, '(')
	close := strings.LastIndexByte(row, ')')
	if open < 0 || close <= open {
		return row
	}
	body := []rune(row[open+1 : close])
	for i := range body {
		body[i] = ' '
	}
	for _, pupil := range [2]struct {
		glyph rune
		col   int
	}{{eye.left, eye.leftCol + gazeX}, {eye.right, eye.rightCol + gazeX}} {
		if pupil.col < 0 || pupil.col >= len(body) {
			return row
		}
		body[pupil.col] = pupil.glyph
	}
	return row[:open+1] + string(body) + row[close:]
}

// companionGazeRows is the gaze composer, and the only place the gaze reaches the
// art. It moves the pupil pair one column to the side and, where the table has a
// second eye row, moves the eyes up to it. The pupil cells are the only
// characters that change between gaze frames, which is what makes the eyes read
// as eyes rather than as a face that slides.
func companionGazeRows(rows []string, eyes companionEyes, gaze companionGaze) []string {
	out := append([]string(nil), rows...)
	if eyes.row < 0 || eyes.row >= len(out) {
		return out
	}
	eye, ok := companionEyesOf(out[eyes.row])
	if !ok {
		return out
	}
	row := eyes.row
	if eyes.upper >= 0 && eyes.upper < len(out) && gaze.Y < 0 {
		row = eyes.upper
	}
	if row != eyes.row {
		out[eyes.row] = companionBlankEyes(out[eyes.row])
	}
	out[row] = companionEyesAt(out[row], eye, gaze.X)
	return out
}

// ============================================================================
// THE VOLUMETRIC SPRITE
// ============================================================================
//
// The glyph cat is the floor: every terminal draws it. Above it there are two more
// steps, drawn only where they can be drawn properly -- a terminal with true colour
// -- and those two steps are the same creature shaded as a volume rather than drawn
// as a silhouette.
//
// Why a volume and not a bigger drawing. The cat that shipped as pixels was a
// hand-written grid of tones: sixteen pixels square of flat colour behind a hard
// outline, which is to say two or three flat regions and an edge, and that is why it
// read as a sticker rather than as a body. The creature is drawn here from an
// implicit surface instead: a handful of ellipsoids and capsules -- a body, a head, a
// muzzle, two ears, four legs and a tail -- summed into one field whose threshold is
// the creature's outline. Nothing is hand-placed but the primitives themselves, so the
// silhouette is round where the eye expects a body to be round, the limbs melt into
// the torso instead of meeting it at a seam, and -- the point of the exercise -- every
// part of the shape is a parameter, which is what lets the gait, the head turn and the
// reactions move a model instead of swapping between drawings.
//
// The shading is what makes it read as a solid rather than as a flat cut-out. The
// field says where the surface is; the field's own gradient says which way the surface
// faces. h = sqrt(1 - T/F) is the surface's height above the outline -- zero on the
// outline, growing towards the middle of a primitive, and for a lone primitive it is
// exactly the cap of a sphere -- so the normal is the vertical to that height field
// and nothing has to be guessed. A light from the upper left then gives the body a lit
// top and a dark underside, an ambient term keeps the shaded side visible as fur, and
// a rim term brightens the silhouette so it separates from the terminal's background
// even where the fur is at its lightest.
//
// A gradient that is continuous in the model still arrives as bands once it is
// quantised, so the ramp is dithered: a fixed 2x2 Bayer matrix decides which of the
// two neighbouring tones each pixel takes. Nothing about it is random and nothing of
// it is time: the matrix is indexed by the pixel's own place in the grid, so a frame
// is a pure function of the model's state and the same model renders the same bytes
// twice. That is also why the dither is in the sprite's own pixel coordinates rather
// than in cells: the two pixels of one cell are one pixel apart, and a dither that was
// constant across a cell could not smooth anything.
//
// Three more parts of the drawing are what make it a creature rather than a shaded
// blob. A drop shadow -- a flat, stippled ellipse on the ground, outside the volume --
// is what puts it in space: in flat media a contact shadow is the whole of depth. The
// eyes are a light disc, a slit pupil and one bright pixel of glint, drawn over the
// volume at the head's own place, and the palette's loudest tone is spent on that
// single glint because the reader's eye should find the one small thing the gaze moves
// rather than the body around it. The nose and the mouth are the two marks on the
// muzzle that make the head a face.
//
// Three sizes, and the ladder tries them in order: the full sprite at
// companionVolumeFullHeight rows, else the small one at companionVolumeSmallHeight,
// else the mini one at companionVolumeMiniHeight, else the glyph cat's five, three
// and one, else nothing. Each smaller one is not a crop of the larger and not a
// redrawing of it: it is the same field sampled on a smaller grid, so the steps
// differ in resolution and not in shape, and a frame that cannot spare the full
// sprite's rows loses size rather than the creature. All three need true colour,
// because the shading is the drawing: on a sixteen-colour terminal those cells are
// blocks in whatever the terminal maps five near-neighbours to, so the gate refuses
// the whole tier and the glyph cat draws instead -- a sixteen-colour terminal and a
// colourless one lose the volume, not the companion.

// companionVolumeFullHeight, companionVolumeSmallHeight and companionVolumeMiniHeight
// are the three heights the ladder can pick above the glyph cat, in the order it tries
// them, and the pixel grid each is drawn on. A pixel is one column and half a row -- the
// half-block glyph the encoder draws with carries two pixels vertically, which is what
// makes a pixel square on a terminal whose cells are twice as tall as they are wide -- so
// the rows here are the pixels and the height is half of them. The mini rung exists so
// the small screens that already showed the five-row glyph cat show the volume instead;
// it is the same five rows as that rung, so it takes over exactly where the glyph cat's
// full rung was chosen (companionGlyphFullMinHeight), and below that the frame has room
// only for the three-row compact head. Five is the smallest grid whose eye and tail
// survive the realism pass's structural assertions
// (TestCompanionAnatomyHoldsAtEveryRung), and it is also the row count the glyph cat's
// own full rung uses, so the two tables share a height and companionSprite disambiguates
// them by mode: the volume draws only where the encoder is on, and the glyph cat
// otherwise.
const (
	companionVolumeFullHeight  = 12
	companionVolumeFullWidth   = 32
	companionVolumeFullRows    = 24
	companionVolumeSmallHeight = 8
	companionVolumeSmallWidth  = 24
	companionVolumeSmallRows   = 16
	companionVolumeMiniHeight  = 5
	companionVolumeMiniWidth   = 14
	companionVolumeMiniRows    = 10
)

// companionVolumeSize is one rung of that ladder: the pixel grid one frame is sampled
// on, and the radius of the eye's light disc in pixels. The eye's radius is the one
// part of the drawing that is not derived from the scale, because the smaller rungs'
// scale puts the world's eye below a pixel and a disc with no pixels left is not an
// eye: the floor is what keeps the gaze readable at every size.
type companionVolumeSize struct {
	width, rows       int
	eyeRadius         float64
	eyeWidth, eyeRows int
}

// companionVolumeSizeFor is the rung a height draws on, and whether that height is one
// of the rungs at all. The glyph heights are not: they are drawn from the tables above.
func companionVolumeSizeFor(height int) (companionVolumeSize, bool) {
	switch height {
	case companionVolumeFullHeight:
		return companionVolumeSize{width: companionVolumeFullWidth, rows: companionVolumeFullRows, eyeRadius: 1.5, eyeWidth: 3, eyeRows: 4}, true
	case companionVolumeSmallHeight:
		return companionVolumeSize{width: companionVolumeSmallWidth, rows: companionVolumeSmallRows, eyeRadius: 1.1, eyeWidth: 2, eyeRows: 2}, true
	case companionVolumeMiniHeight:
		return companionVolumeSize{width: companionVolumeMiniWidth, rows: companionVolumeMiniRows, eyeRadius: 1.0, eyeWidth: 2, eyeRows: 2}, true
	}
	return companionVolumeSize{}, false
}

// ============================================================================
// THE CREATURE'S BODY
// ============================================================================
//
// The world the creature is modelled in is its own: y = 0 is the ground, y grows up,
// and one unit is about the width of the body. The box below is what the frame's cell
// shows, and it is the whole of the camera: both rungs look at the same box from the
// same distance, so the small sprite is the full one seen smaller. The box takes the
// room the shadow needs under the paws and the ear tips' own height above them, and the
// scale is the tighter of the two axes, so no part of a leaning, striding or startled
// creature can leave the cell it is drawn in.
const (
	companionWorldHalfWidth = 1.0
	companionWorldTop       = 1.45
	companionWorldBottom    = -0.18
)

// companionVolumeScale is pixels per world unit: the tighter of the cell's two axes.
func companionVolumeScale(size companionVolumeSize) float64 {
	byWidth := float64(size.width) / (2 * companionWorldHalfWidth)
	byHeight := float64(size.rows) / (companionWorldTop - companionWorldBottom)
	return math.Min(byWidth, byHeight)
}

// companionWorldX and companionWorldY are the cell's camera: the world point at the
// middle of one pixel, given the pixel's own column and row. The row counts downwards
// and the world upwards, and half a pixel is added to each so a pixel is sampled at its
// centre rather than at its corner -- which is also why the two are the exact inverses
// of companionPixelOf.
func companionWorldX(px int, size companionVolumeSize, scale float64) float64 {
	return (float64(px) + 0.5 - float64(size.width)/2) / scale
}

func companionWorldY(py int, size companionVolumeSize, scale float64) float64 {
	return companionWorldTop - (float64(py)+0.5)/scale
}

// companionPixelOf is the inverse: the pixel a world point falls on.
func companionPixelOf(x, y float64, size companionVolumeSize, scale float64) [2]int {
	return [2]int{
		int(math.Round(x*scale + float64(size.width)/2 - 0.5)),
		int(math.Round((companionWorldTop-y)*scale - 0.5)),
	}
}

// companionSurface is the field's threshold: the outline is where the primitives' sum
// crosses it. It is deliberately above one. Each primitive is written with the
// half-axes the creature's own shape wants, and at a threshold of one a lone
// primitive's surface would sit exactly on them; a little above one shaves every
// primitive the same few per cent and leaves the fusion between two neighbours to do the
// growing, which is the whole reason the shape is a field and not a list of ellipses.
const companionSurface = 1.15

// companionBlob is one ellipsoid and companionCapsule one thick segment: the two
// primitives the creature is made of. Both answer with the same kind of number -- the
// inverse square of the distance to their own surface, in their own units -- so they can
// be summed, and the sum's threshold is the creature.
type companionBlob struct {
	x, y float64 // the centre
	a, b float64 // the half-axes
	c, s float64 // the rotation, as its cosine and sine: resolved when the pose is built
}

type companionCapsule struct {
	x0, y0 float64 // one end, the end that stays on the body
	x1, y1 float64 // the other, the end the animation moves
	r      float64 // the segment's thickness
}

type companionTriangle struct {
	points [3][2]float64
	r      float64
}

func (tr companionTriangle) contains(x, y float64) bool {
	positive, negative := false, false
	for i, a := range tr.points {
		b := tr.points[(i+1)%len(tr.points)]
		cross := (b[0]-a[0])*(y-a[1]) - (b[1]-a[1])*(x-a[0])
		positive = positive || cross > 0
		negative = negative || cross < 0
	}
	return !(positive && negative)
}

func (tr companionTriangle) field(x, y float64) float64 {
	minDistance2 := math.Inf(1)
	for i, a := range tr.points {
		b := tr.points[(i+1)%len(tr.points)]
		dx, dy := b[0]-a[0], b[1]-a[1]
		length2 := dx*dx + dy*dy
		t := 0.0
		if length2 > 0 {
			t = min(1, max(0, ((x-a[0])*dx+(y-a[1])*dy)/length2))
		}
		ex, ey := x-a[0]-t*dx, y-a[1]-t*dy
		minDistance2 = math.Min(minDistance2, ex*ex+ey*ey)
	}
	if tr.contains(x, y) {
		return 1 / 1e-6
	}
	return 1 / math.Max(minDistance2/(tr.r*tr.r), 1e-6)
}

// companionBlobAt is an ellipsoid written the way the art reads: a centre, the two
// half-axes and a rotation in radians, taken anticlockwise.
func companionBlobAt(x, y, a, b, rot float64) companionBlob {
	return companionBlob{x: x, y: y, a: a, b: b, c: math.Cos(rot), s: math.Sin(rot)}
}

// field is one primitive's contribution at a point. It is one on the primitive's own
// surface, more inside it and less outside, so the threshold is the outline.
func (bl companionBlob) field(x, y float64) float64 {
	dx, dy := x-bl.x, y-bl.y
	if bl.s != 0 {
		dx, dy = dx*bl.c+dy*bl.s, -dx*bl.s+dy*bl.c
	}
	u, v := dx/bl.a, dy/bl.b
	return 1 / math.Max(u*u+v*v, 1e-6)
}

// field is the capsule's: the distance to its own segment, measured in its thickness.
func (cp companionCapsule) field(x, y float64) float64 {
	dx, dy := cp.x1-cp.x0, cp.y1-cp.y0
	t := 0.0
	if length := dx*dx + dy*dy; length > 0 {
		t = min(1, max(0, ((x-cp.x0)*dx+(y-cp.y0)*dy)/length))
	}
	ex, ey := x-cp.x0-t*dx, y-cp.y0-t*dy
	return 1 / math.Max((ex*ex+ey*ey)/(cp.r*cp.r), 1e-6)
}

// companionPose is the creature's primitives at one instant: the whole of its shape for
// one frame. It is a value built from the state, the tick, the gaze, the rung's size
// and the tremble of a startle, and it carries no model and no clock of its own -- the
// render path can therefore build one, draw it and drop it.
type companionPose struct {
	body, chest, haunch, neck, head, muzzle companionBlob
	ears                                    [2]companionTriangle
	legs                                    [4]companionCapsule
	tail                                    [3]companionCapsule
	// shadow is flat on the ground and does not move with anything the creature does:
	// it is what the creature stands on, so it is drawn from the ground and not from
	// the body.
	shadow companionBlob
}

// field is the creature's own field: the sum of every primitive, whose threshold is the
// outline. The shadow is not part of it -- the shadow is not the creature, it is what
// the creature casts.
func (p companionPose) field(x, y float64) float64 {
	sum := p.body.field(x, y) + p.chest.field(x, y) + p.haunch.field(x, y) + p.neck.field(x, y) + p.head.field(x, y) + p.muzzle.field(x, y)
	for _, ear := range p.ears {
		sum += ear.field(x, y)
	}
	for _, leg := range p.legs {
		sum += leg.field(x, y)
	}
	for _, part := range p.tail {
		sum += part.field(x, y)
	}
	return sum
}

// height is how far the surface stands above the outline at one point: the height field
// the shading reads its normals from. sqrt(1 - T/F) is zero where the field is at its
// threshold, rises towards the middle of a primitive and is exactly the cap of a sphere
// for a lone one, which is what makes its gradient a surface normal rather than a
// guess. It is bounded in [0, 1) instead of growing with the field, so the gradient
// stays finite at a primitive's centre.
func (p companionPose) height(x, y float64) float64 {
	field := p.field(x, y)
	if field <= companionSurface {
		return 0
	}
	return math.Sqrt(1 - companionSurface/field)
}

// heightField samples that height on the sprite's own grid, one pixel wider on every
// side. The border is there because a pixel's normal is read from its neighbours and the
// pixels on the edge of the cell need neighbours too; the field is defined everywhere,
// so the border is sampled exactly like any other pixel.
func (p companionPose) heightField(size companionVolumeSize, scale float64) [][]float64 {
	heights := make([][]float64, size.rows+2)
	for gy := range heights {
		heights[gy] = make([]float64, size.width+2)
		for gx := range heights[gy] {
			heights[gy][gx] = p.height(
				companionWorldX(gx-1, size, scale),
				companionWorldY(gy-1, size, scale),
			)
		}
	}
	return heights
}

// The animation's amplitudes and cadence. The geometry below is the art; these
// are the motion values a reader can change without redrawing the creature.
const (
	// companionStrideReach is how far a swinging paw reaches in front of its own middle
	// and companionStrideLift how far off the ground it comes while it swings. At the
	// full sprite they are about a pixel and most of another, which is as far as a paw
	// can move and still read as a paw rather than as a leg that teleports.
	companionStrideReach = 0.075
	companionStrideLift  = 0.055

	// companionBodyBobPx is the body's rise at the passing pose, in raster pixels.
	companionBodyBobPx = 1

	// companionTailSway scales the tail's signal, sampled two gait poses behind the body.
	companionTailSway = 0.9

	// companionHeadTurnX and companionHeadTurnY are how far the head turns towards what
	// the creature is looking at -- a pixel at the full size, and a little less upwards
	// -- and companionMuzzleSwing is how much further the muzzle swings than the head
	// does. A turn that moved the head without moving the muzzle further would read as a
	// head sliding sideways rather than as a face turning.
	companionHeadTurnX   = 0.07
	companionHeadTurnY   = 0.025
	companionMuzzleSwing = 1.8

	// companionLeanTurn is how far the torso tilts with the gait. Small, and worth it: a
	// body that stays exactly rigid while its legs stride reads as a cut-out being
	// carried along.
	companionLeanTurn = 0.10

	// companionSagPx is how far a sleeping or startled creature's body sinks, and the
	// companion of the walk's bob: both move the body while the paws stay on the ground.
	companionSagPx = 1

	// companionShiverPx and companionShiverTicks are the startle: one pixel either way
	// on alternate frames, for the first few frames after a key. It is bounded on
	// purpose, and that bound is why a startle costs three repaints rather than one
	// repaint per frame for as long as an error stays on screen: the creature shivers,
	// and then it holds the pose.
	companionShiverPx    = 1
	companionShiverTicks = 3

	// companionGaitTicks is one frame per named pose: a four-frame, half-second
	// cycle at the model's eight ticks per second.
	companionGaitTicks = 1
)

// companionGaitReach and companionGaitLift are the four values sampled by each paw's
// local pose. Each paw's phase offset determines its touchdown frame; the sequence is
// staggered rather than paired into a diagonal trot.
var (
	companionGaitReach = [4]float64{0, 1, -1, 0}
	companionGaitLift  = [4]float64{0, 0.8, 1, 0.35}

	// The body rises at the passing pose only; its one-frame lead is applied to the head.
	companionGaitBob = [4]float64{0, 0, 1, 0}

	// Geometry order is front-left, front-right, back-left, back-right. The offsets
	// make touchdown occur front-left, back-right, front-right, back-left.
	companionLegPhase = [4]int{0, 2, 1, 3}
)

// companionGaitPhase is the walk's pose, read from the tick and from nothing else:
// companionGaitTicks frames a pose, four poses, then round again. It is the tick's own
// arithmetic and not a clock's, so the same model draws the same stride and a snapshot
// can pin one.
func companionGaitPhase(tick int) int {
	phase := (tick / companionGaitTicks) % 4
	if phase < 0 {
		phase += 4
	}
	return phase
}

// companionEventAt emits a one-frame event at each independent period boundary.
func companionEventAt(tick, period int) bool {
	return period > 0 && tick%period == period-1
}

func boolFloat(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

// companionPoseFor builds the creature's primitives for one frame: the whole of its
// shape, in the world's own units. Everything above draws from this and nothing here
// reads the model, the terminal or the clock.
//
// The creature faces right -- the direction it walks -- and it is built the way the eye
// reads a cat: a long low body, a head a little over a third of the body's own length
// with two ears and a muzzle, four legs under the body and a tail out of the back. The
// head is the part the animation moves most, because the head is where a reader looks.
// A gaze turns that head, not just its pupils: because the outline is a threshold over
// the summed field, the turn may move nearby silhouette and shading pixels too. That is
// a real consequence of this volume model, not a redraw defect; tests bound all such
// changes to the creature's world-space cell and separately require the pupils to
// move in the looked-at direction.
func companionPoseFor(state companionState, tick int, gaze companionGaze, shiver int, size companionVolumeSize, hop ...int) companionPose {
	scale := companionVolumeScale(size)
	unit := func(pixels float64) float64 { return pixels / scale }
	gait := companionGaitPhase(tick)
	hopPhase := 0
	if len(hop) > 0 {
		hopPhase = hop[0]
	}

	// What the body itself does this frame: the walk lifts it over the passing poses, sleep
	// and a startle fold it down, and a yawn stretches it up. The paws stay on the ground
	// through all of them, which is what makes a bob read as a body moving over standing legs
	// rather than as the whole creature hopping. The head keeps its own height while the body
	// folds: that is what a cat crouching does, and it is also what keeps the ears -- which
	// are the top of the creature -- inside the cell's own top row in every state, so no
	// frame is a row shorter than the ladder asked for.
	lift, headLift := 0.0, 0.0
	switch state {
	case companionWalkingState:
		lift = unit(companionBodyBobPx * companionGaitBob[gait])
		headLift = unit(companionBodyBobPx * companionGaitBob[(gait+1)%4])
	case companionAsleepState, companionFlinchingState:
		lift = -unit(companionSagPx)
	case companionYawningState:
		lift = unit(1)
		headLift = lift
	}
	// Hop poses deform within the rung's fixed raster: anticipation crouches,
	// flight lifts one pixel and landing compresses the body without adding rows.
	if hopPhase == companionHopTicks {
		lift -= unit(1)
		headLift -= unit(1)
	} else if hopPhase == companionHopTicks-1 {
		// Two square raster pixels make one terminal row of travel; clipping at
		// the fixed grid edge is preferable to borrowing another block row.
		lift += unit(2)
		headLift += unit(2)
	}

	// The tremble of a startle: the whole creature, moved one pixel, the way a shiver
	// moves a whole animal. The pose it settles into is the crouch above, so a creature
	// that has stopped trembling is still a creature that was startled.
	tremble := unit(float64(shiver))

	// The extreme horizontal gaze shifts the skull exactly one pixel. The central
	// dead-zone columns leave its geometry untouched, so only the eye overlay changes.
	headShift := 0.0
	if gaze.X < 0 {
		headShift = -unit(1)
	} else if gaze.X > 0 {
		headShift = unit(1)
	}
	earTilt := headShift
	headX := 0.42 + headShift + tremble
	headY := 0.93 + headLift
	muzzleX := headX + 0.28
	muzzleY := headY - 0.12

	// The ears and the tail are the two parts that carry a state's mood: a startled
	// creature sweeps its ears back over its head and tucks its tail, an alert one pricks
	// both up, and a sleeping one lets both go slack. The ears are swept rather than
	// lowered on purpose -- an ear that dropped would take the top row of the sprite with
	// it -- so even the lowest ear still reaches the cell's own top.
	earBack, earLift, tailTip := 0.0, 0.0, 0.0
	idlePose := state == companionIdleState || state == companionBlinkingState
	breath := idlePose && companionEventAt(tick, companionBreathTicks)
	if idlePose && companionEventAt(tick, companionEarTwitchTicks) {
		earLift += unit(1)
	}
	if idlePose && companionEventAt(tick, companionTailFlickTicks) {
		tailTip += unit(1)
	}
	switch state {
	case companionAlertState:
		earLift, tailTip = 0.045, 0.12
	case companionFlinchingState:
		earBack, earLift, tailTip = -0.10, -0.02, -0.16
	case companionAsleepState:
		earBack, tailTip = -0.03, -0.18
	case companionPleasedState:
		earLift, tailTip = 0.035, 0.14
	}

	// The walk's own four poses, one per paw. Every other state plants all four paws
	// where they stand, which is what makes the idle pose a stance.
	var reach, swing [4]float64
	if state == companionWalkingState {
		for i := range reach {
			foot := (gait + companionLegPhase[i]) % 4
			reach[i] = companionStrideReach * companionGaitReach[foot]
			swing[i] = companionStrideLift * companionGaitLift[foot]
		}
	}

	// The tail's signal trails the body by two gait poses; the tip travels farther
	// than the middle so the tail bends as it counterbalances the stride.
	sway := 0.0
	if state == companionWalkingState {
		sway = -companionTailSway * companionGaitReach[(gait+2)%4]
	}

	// The torso tilts with the gait.
	lean := 0.0
	if state == companionWalkingState {
		lean = companionLeanTurn * companionGaitReach[gait]
	}

	// The two ends of the tail, so the capsule above and the one below meet at the same
	// point: a tail whose two parts disagreed would have a gap in it.
	tailJointX := -0.78 + tremble + sway*0.35
	tailJointY := 0.86 + lift + tailTip*0.3
	tailTipX := -0.96 + tremble + sway
	tailTipY := 1.16 + lift + tailTip

	bodyDepth := 1.0
	if hopPhase == 1 {
		bodyDepth = 0.78
	}
	return companionPose{
		body:   companionBlobAt(-0.18+tremble, 0.56+lift, 0.42, 0.26*bodyDepth, lean),
		chest:  companionBlobAt(0.22+tremble, 0.57+lift+boolFloat(breath)*unit(1), 0.30, 0.28*bodyDepth, lean),
		haunch: companionBlobAt(-0.53+tremble, 0.57+lift, 0.43, 0.34, lean),
		neck:   companionBlobAt(0.38+tremble, 0.73+lift, 0.15, 0.25, lean),
		head:   companionBlobAt(headX, headY, 0.29, 0.275, 0),
		muzzle: companionBlobAt(muzzleX, muzzleY, 0.17, 0.105, 0),
		ears: [2]companionTriangle{
			{points: [3][2]float64{{headX - 0.22 + earBack, headY + 0.17}, {headX - 0.13 + earBack + earTilt, headY + 0.50 + earLift}, {headX + 0.01 + earBack, headY + 0.17}}, r: 0.025},
			{points: [3][2]float64{{headX + 0.02 + earBack, headY + 0.17}, {headX + 0.20 + earBack + earTilt, headY + 0.50 + earLift}, {headX + 0.26 + earBack, headY + 0.17}}, r: 0.025},
		},
		legs: [4]companionCapsule{
			{x0: 0.16 + tremble, y0: 0.40 + lift, x1: 0.19 + tremble + reach[0], y1: 0.06 + swing[0], r: 0.072},
			{x0: 0.02 + tremble, y0: 0.38 + lift, x1: -0.01 + tremble + reach[1], y1: 0.06 + swing[1], r: 0.072},
			{x0: -0.46 + tremble, y0: 0.40 + lift, x1: -0.49 + tremble + reach[2], y1: 0.06 + swing[2], r: 0.072},
			{x0: -0.58 + tremble, y0: 0.38 + lift, x1: -0.61 + tremble + reach[3], y1: 0.06 + swing[3], r: 0.072},
		},
		tail: [3]companionCapsule{
			{x0: -0.62 + tremble, y0: 0.60 + lift, x1: tailJointX, y1: tailJointY, r: 0.090},
			{x0: tailJointX, y0: tailJointY, x1: tailTipX + 0.04, y1: tailTipY - 0.08, r: 0.045},
			{x0: tailTipX + 0.04, y0: tailTipY - 0.08, x1: tailTipX, y1: tailTipY, r: 0.020},
		},
		// The shadow is flat on the ground and does not move with the creature: it is what
		// the creature stands on, and it is drawn from the ground so that a hop or a bob
		// leaves it where it is.
		shadow: companionBlobAt(0, -0.07, 0.62, 0.105, 0),
	}
}

// ============================================================================
// THE LIGHT AND THE RAMP
// ============================================================================

// The light model's four numbers, named because they are the difference between a
// shaded body and a flat one. Ambient is what the unlit side is still worth, diffuse is
// what the lamp adds where the surface faces it, rim is how much brighter the silhouette
// is than the surface behind it, and relief is how much of a normal the height field's
// gradient is worth: the gradient of a gentle dome is a shallow slope, and relief is
// what turns it into a body rather than a saucer.
const (
	companionAmbient = 0.34
	companionDiffuse = 0.72
	companionRim     = 0.45
	companionRelief  = 1.5
)

// companionRampSteps is how many tones the shading quantises into. Five is what the
// palette can support honestly: the two ends are the darkest fur and the lit one, and
// the three between them are the ramp the dither picks from. Fewer steps band visibly;
// more would need tones the theme does not have.
const companionRampSteps = 5

// Contact regions are expressed in the creature's normalized world coordinates. They
// describe the creases under the neck and body and the narrow gap between the legs; the
// test checks dark pixels against these regions rather than choosing pixels by appearance.
func companionContactRegion(x, y float64) bool {
	underNeck := x > 0.20 && x < 0.48 && y > 0.37 && y < 0.54
	underBody := x > -0.44 && x < 0.18 && y > 0.25 && y < 0.40
	betweenLegs := x > -0.42 && x < 0.10 && y > 0.12 && y < 0.31
	return underNeck || underBody || betweenLegs
}

// companionEyeBox is the named rectangular raster region owned by an eye, measured from
// the same rung-sized dimensions used by companionPaintEye.
func companionEyeBox(centre [2]int, size companionVolumeSize, x, y int) bool {
	halfW, halfH := size.eyeWidth/2, size.eyeRows/2
	return x >= centre[0]-halfW && x < centre[0]+size.eyeWidth-halfW &&
		y >= centre[1]-halfH && y < centre[1]+size.eyeRows-halfH
}

// companionHeadOutlineBand is the named silhouette-edge region that extreme gaze may
// alter. It includes pixels close to the skull field's threshold and either ear's edge;
// the interior of the head, torso, limbs, tail and shadow are deliberately excluded.
func companionHeadOutlineBand(pose companionPose, size companionVolumeSize, scale float64, px, py int) bool {
	x, y := companionWorldX(px, size, scale), companionWorldY(py, size, scale)
	near := func(field float64) bool { return math.Abs(field-companionSurface) <= 0.55 }
	if near(pose.head.field(x, y)) {
		return true
	}
	for _, ear := range pose.ears {
		if near(ear.field(x, y)) {
			return true
		}
	}
	return false
}

// companionLight is where the light is: up, to the left and in front of the creature,
// normalised once so the Lambert term is a dot product and nothing else. The direction
// is what puts the bright side of the body at its upper left and the dark side under its
// chin and along its right -- the way a reader expects a photograph to be lit.
var companionLight = companionUnit([3]float64{-0.50, 0.62, 0.60})

// companionUnit normalises a three-vector, which the light is read far too often per
// frame to do on every call.
func companionUnit(v [3]float64) [3]float64 {
	n := math.Sqrt(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])
	return [3]float64{v[0] / n, v[1] / n, v[2] / n}
}

// companionBayer is the ordered-dither matrix, as the thresholds a pixel's fraction of
// the way to the next tone is compared against. It is the standard 2x2 arrangement --
// the classic (0 2 / 3 1)/4 -- moved by half a step so that a fraction of zero stays on
// the lower tone. Nothing about it is random and nothing is time: it is indexed by the
// pixel's own place in the grid, which is what makes the dither part of the drawing
// rather than noise over it.
var companionBayer = [2][2]float64{{0.125, 0.625}, {0.875, 0.375}}

// companionShadowLevel is where on the ramp the ground's shadow sits: half the pixels take
// the darkest tone and half the one above it, which is what a soft shadow looks like on a
// grid of whole cells -- the two average to something darker than the background and lighter
// than the darkest fur. A flat tone would read as a bar under the creature and a solid one
// as a hole in the screen, and the level is half rather than a quarter because a quarter
// leaves the odd pixel rows solid, which stripes the shadow from the inside.
const companionShadowLevel = 0.5

// companionDitherTone quantises a place on the ramp to one of its tones, using the
// pixel's own cell of the Bayer matrix to choose between the two tones the place falls
// between. A place of zero or less is left on the lowest tone rather than dithered, so a
// pixel meant to be the darkest fur is not stippled with anything.
func companionDitherTone(level float64, px, py int) companionTone {
	if level <= 0 {
		return companionRampTone(0)
	}
	if level >= companionRampSteps-1 {
		return companionRampTone(companionRampSteps - 1)
	}
	low := int(level)
	if level-float64(low) > companionBayer[py%2][px%2] {
		low++
	}
	return companionRampTone(low)
}

// companionRampTone is the tone one step of the ramp draws as. It is a cast with its
// bounds held here because the ramp's tones are the ramp's own indices, which is what
// lets the dither choose between them with arithmetic instead of with a table.
func companionRampTone(step int) companionTone {
	return companionTone(min(max(step, 0), companionRampSteps-1))
}

// companionShadeTone is the light model at one pixel: the surface's normal read off the
// height field's four neighbours, Lambert with companionLight, the ambient term, and the
// rim term that brightens the silhouette. The normal is the vertical to the height field
// -- (-dh/dx, -dh/dy, 1) -- and the height field's row runs downwards while the world's
// y runs up, which is the one sign in here worth a comment.
func companionShadeTone(heights [][]float64, px, py int, scale float64) companionTone {
	return companionShadeToneWithTerms(heights, px, py, scale, companionRim, true)
}

func companionShadeToneWithTerms(heights [][]float64, px, py int, scale, rimStrength float64, contact bool) companionTone {
	gx := (heights[py+1][px+2] - heights[py+1][px]) * scale / 2
	gy := (heights[py+2][px+1] - heights[py][px+1]) * scale / 2
	nx, ny, nz := -gx*companionRelief, gy*companionRelief, 1.0
	if inv := 1 / math.Sqrt(nx*nx+ny*ny+nz*nz); inv > 0 {
		nx, ny, nz = nx*inv, ny*inv, nz*inv
	}
	lambert := math.Max(0, nx*companionLight[0]+ny*companionLight[1]+nz*companionLight[2])
	// The rim is directional: only the creature's upper-left-facing edge catches it.
	rim := 0.0
	if nx < -0.18 && ny > 0.12 {
		rim = (1 - nz) * (1 - nz) * rimStrength
	}
	// The light is a luminance in [0, 1] and the ramp is a count of tones, so the one is
	// scaled into the other here rather than inside the dither, which has to stay the same
	// function for the shadow's own place on the ramp.
	luminance := min(1, max(0, companionAmbient+companionDiffuse*lambert+rim))
	tone := companionDitherTone(luminance*float64(companionRampSteps-1), px, py)
	x, y := companionWorldX(px, companionVolumeSize{width: len(heights[0]) - 2, rows: len(heights) - 2}, scale),
		companionWorldY(py, companionVolumeSize{width: len(heights[0]) - 2, rows: len(heights) - 2}, scale)
	if contact && companionContactRegion(x, y) {
		tone = companionRampTone(int(tone) - 2)
	}
	return tone
}

// companionTone is one pixel of the sprite before it is a colour: a step of the
// shading's ramp, or one of the marks the art draws on top of the volume. It is a tone
// and not a colour because the ramp and the marks are the drawing and the palette is only
// how the terminal is told about it: a test can pin the exact tones a frame is made of
// without a terminal, and the ink below answers with the colours.
type companionTone int

const (
	// companionToneNone is a pixel the creature does not cover: the terminal's own
	// background, drawn as a space with no escape sequence at all.
	companionToneNone companionTone = -1

	// The ramp, darkest first: the shading quantises into these, and a test that counts
	// how many of them a frame uses is a test of whether the gradient is really there.
	companionToneRamp0 companionTone = 0
	companionToneRamp1 companionTone = 1
	companionToneRamp2 companionTone = 2
	companionToneRamp3 companionTone = 3
	companionToneRamp4 companionTone = 4

	// The marks. They sit above the ramp's own numbers so that "is this tone part of the
	// shading" is a comparison rather than a lookup.
	companionToneEye   companionTone = 5 // the light disc of an eye
	companionTonePupil companionTone = 6 // a pupil, and the lid or the mouth line that is the same dark
	companionToneGlint companionTone = 7 // the one bright pixel in the whole sprite
	companionToneNose  companionTone = 8 // the rose nose
)

// companionToneNames names every tone, in the order they are declared, so a tone added
// without a name fails a test rather than printing as a number in a failure message.
var companionToneNames = map[companionTone]string{
	companionToneNone:  "none",
	companionToneRamp0: "ramp0",
	companionToneRamp1: "ramp1",
	companionToneRamp2: "ramp2",
	companionToneRamp3: "ramp3",
	companionToneRamp4: "ramp4",
	companionToneEye:   "eye",
	companionTonePupil: "pupil",
	companionToneGlint: "glint",
	companionToneNose:  "nose",
}

// companionTones is every tone the art draws, the empty one included, which is the list
// a test walks rather than a hand-kept copy of it.
func companionTones() []companionTone {
	return []companionTone{
		companionToneNone,
		companionToneRamp0, companionToneRamp1, companionToneRamp2, companionToneRamp3, companionToneRamp4,
		companionToneEye, companionTonePupil, companionToneGlint, companionToneNose,
	}
}

// companionEyeMark is what an eye is doing this frame, which is the whole of the face's
// expression: the volume supplies the head and the light supplies the body, and a state
// is read off its eyes and its pose before it is read off its nose.
type companionEyeMark int

const (
	companionEyeOpen companionEyeMark = iota
	companionEyeWide
	companionEyeShut
	companionEyeArc
	companionEyeSquint
	companionEyeHalfShut
)

// companionEyeMarkFor is the state's own eye mark. The shut eye is shared by a blink and
// a sleep -- a closed eye is a closed eye, and the sag of the body is what tells those
// two apart -- while a yawn keeps the top half of its own open, which is what stops a
// yawn from reading as a blink.
func companionEyeMarkFor(state companionState) companionEyeMark {
	switch state {
	case companionBlinkingState, companionAsleepState:
		return companionEyeShut
	case companionYawningState:
		return companionEyeHalfShut
	case companionAlertState:
		return companionEyeWide
	case companionPleasedState:
		return companionEyeArc
	case companionFlinchingState:
		return companionEyeSquint
	default:
		return companionEyeOpen
	}
}

// companionPupilStartRow maps vertical gaze to the pupil's long axis. The full-rung 3x4
// sclera places the 1x2 slit at each of its three possible row offsets: the centre has a full
// ring, while each extreme touches one eyelid edge. The small rung keeps its one-pixel pupil
// inside its actual two-row box.
func companionPupilStartRow(size companionVolumeSize, gaze companionGaze) int {
	if size.eyeRows >= 4 {
		switch {
		case gaze.Y < 0:
			return 0
		case gaze.Y > 0:
			return 2
		default:
			return 1
		}
	}
	if gaze.Y > 0 {
		return 1
	}
	return 0
}

// companionEyeAt is one pixel of one eye. The sclera stays a rectangular, rung-sized
// field and the pupil is composed over it; a closed eye overlays a face-tone lid instead
// of deleting the eye. The pupil never touches the face outline and is always inside the
// sclera. At the full rung, the vertical slit has sclera on both sides across its own length;
// at the small 2x2 rung, the 1x1 pupil keeps horizontal and vertical sclera adjacency. The
// full-rung 3x4 box keeps a sclera column on both sides at every pupil position; only its
// central position has room for a full ring. The rung, head and block never grow to fit an eye.
func companionEyeAt(dx, dy int, size companionVolumeSize, mark companionEyeMark, gaze companionGaze) companionTone {
	halfW, halfH := size.eyeWidth/2, size.eyeRows/2
	if dx < -halfW || dx >= size.eyeWidth-halfW || dy < -halfH || dy >= size.eyeRows-halfH {
		return companionToneNone
	}
	if mark == companionEyeShut {
		if dy == 0 {
			return companionToneRamp2
		}
		return companionToneEye
	}
	if mark == companionEyeArc && dy == -halfH {
		return companionToneRamp2
	}
	if mark == companionEyeSquint && dy == -halfH {
		return companionToneRamp2
	}
	if mark == companionEyeHalfShut && dy == 0 {
		return companionToneRamp2
	}
	pupilX := 0 // Horizontal gaze is carried by the head, leaving a full sclera ring.
	pupilHeight := 1
	if size.eyeRows >= 4 {
		pupilHeight = 2
	}
	pupilY := companionPupilStartRow(size, gaze) - halfH
	if dx == pupilX && dy >= pupilY && dy < pupilY+pupilHeight {
		return companionTonePupil
	}
	return companionToneEye
}

// companionEyeCentres is where the pair of eyes sits: one pixel row above the head's own
// centre and either side of it by a share of the head's own radius, so a head drawn larger
// carries its eyes further apart -- with the eye's own radius as a floor, because the small
// rung's share of its head is one pixel and two eyes one pixel either side of the middle
// would share the middle. Both centres are whole pixels, which is what lets a pupil sit on
// one.
func companionEyeCentres(head companionBlob, size companionVolumeSize, scale float64) [2][2]int {
	centre := companionPixelOf(head.x, head.y, size, scale)
	offset := max(int(math.Ceil(size.eyeRadius)), int(math.Round(head.a*scale*0.48)))
	return [2][2]int{
		{centre[0] - offset, centre[1]},
		{centre[0] + offset, centre[1]},
	}
}

// companionPaintTone writes one tone into the grid, inside the sprite and inside the
// creature: a mark is a mark on a body, and one that asked for a pixel the body does not
// cover is a bug in the placement rather than something to draw in mid-air.
func companionPaintTone(grid [][]companionTone, inside func(int, int) bool, x, y int, tone companionTone) {
	if !inside(x, y) {
		return
	}
	grid[y][x] = tone
}

// companionPaintEye draws the rung-sized sclera, pupil/lid and a fixed upper-left
// highlight. The glint's position is relative to the eye centre, never to gaze.
func companionPaintEye(grid [][]companionTone, centre [2]int, size companionVolumeSize, mark companionEyeMark, gaze companionGaze) {
	halfW, halfH := size.eyeWidth/2, size.eyeRows/2
	for dy := -halfH; dy < size.eyeRows-halfH; dy++ {
		for dx := -halfW; dx < size.eyeWidth-halfW; dx++ {
			x, y := centre[0]+dx, centre[1]+dy
			if y < 0 || y >= len(grid) || x < 0 || x >= len(grid[y]) {
				continue
			}
			grid[y][x] = companionEyeAt(dx, dy, size, mark, gaze)
		}
	}
	if mark != companionEyeArc {
		pupilY := companionPupilStartRow(size, gaze) - halfH
		pupilHeight := 1
		if size.eyeRows >= 4 {
			pupilHeight = 2
		}
		for dy := pupilY; dy < pupilY+pupilHeight; dy++ {
			if mark == companionEyeShut && dy == 0 {
				continue // The eyelid line closes over this pupil pixel.
			}
			x, y := centre[0], centre[1]+dy
			if y >= 0 && y < len(grid) && x >= 0 && x < len(grid[y]) {
				grid[y][x] = companionTonePupil
			}
		}
	}
	if mark != companionEyeArc {
		glintX, glintY := centre[0]-halfW, centre[1]-halfH
		if glintY >= 0 && glintY < len(grid) && glintX >= 0 && glintX < len(grid[glintY]) {
			grid[glintY][glintX] = companionToneGlint
		}
	}
}

// companionPaintMuzzle draws the two marks that make the head a face: the nose, in the
// palette's rose, and the mouth under it. Both are sized off the eye's own radius
// because they are the same problem -- how much detail a mark is worth at this rung -- and
// a mouth as wide as the full rung's on the small one would be the whole muzzle. A yawn
// opens the mouth by a row, which is the one expression the volume draws that the glyph
// art can only hint at.
func companionPaintMuzzle(grid [][]companionTone, inside func(int, int) bool, muzzle companionBlob, size companionVolumeSize, scale float64, state companionState) {
	width := 1
	if size.width == companionVolumeFullWidth {
		width = 2
	}
	nose := companionPixelOf(muzzle.x+0.045, muzzle.y+0.03, size, scale)
	for dx := 0; dx < width; dx++ {
		companionPaintTone(grid, inside, nose[0]+dx, nose[1], companionToneNose)
	}

	if size.width != companionVolumeFullWidth {
		return
	}
	mouth := companionPixelOf(muzzle.x+0.03, muzzle.y-0.045, size, scale)
	open := state == companionYawningState
	for dx := 0; dx < 2; dx++ {
		dy := 0
		if !open && dx == 0 {
			// A closed mouth is a shallow curve rather than a line: the middle of it is a
			// pixel lower than its ends, which is what makes it read as a mouth.
			dy = 1
		}
		companionPaintTone(grid, inside, mouth[0]+dx, mouth[1]+dy, companionTonePupil)
		if open {
			companionPaintTone(grid, inside, mouth[0]+dx, mouth[1]+1, companionTonePupil)
		}
	}
}

// companionWhiskerPixel places one whisker landmark. If its nominal pixel overlaps the
// full-rung eye box, it steps outward along its own whisker until the sclera ring stays
// intact; all three strokes per side remain visible.
func companionWhiskerPixel(muzzle companionBlob, offset, side float64, step int, size companionVolumeSize, scale float64, eyes [2][2]int) [2]int {
	point := companionPixelOf(muzzle.x+side*(0.17+float64(step)*0.065), muzzle.y+offset, size, scale)
	for moved := 0; moved < size.width; moved++ {
		insideEye := false
		for _, centre := range eyes {
			insideEye = insideEye || companionEyeBox(centre, size, point[0], point[1])
		}
		if !insideEye {
			break
		}
		point[0] += int(side)
	}
	return point
}

// companionPaintAnatomyDetails draws the small landmarks that do not belong to the
// smooth field: inner ears, paw pads and the largest rung's whiskers.
func companionPaintAnatomyDetails(grid [][]companionTone, pose companionPose, size companionVolumeSize, scale float64) {
	for _, ear := range pose.ears {
		var centre [2]float64
		for _, point := range ear.points {
			centre[0] += point[0] / 3
			centre[1] += point[1] / 3
		}
		inner := ear
		for i, point := range ear.points {
			inner.points[i][0] = centre[0] + (point[0]-centre[0])*0.42
			inner.points[i][1] = centre[1] + (point[1]-centre[1])*0.42
		}
		for py := range grid {
			for px := range grid[py] {
				if inner.contains(companionWorldX(px, size, scale), companionWorldY(py, size, scale)) && grid[py][px] != companionToneNone {
					grid[py][px] = companionTonePupil
				}
			}
		}
	}

	for _, leg := range pose.legs {
		paw := companionPixelOf(leg.x1, leg.y1, size, scale)
		for dx := -1; dx <= 1; dx++ {
			x, y := paw[0]+dx, paw[1]
			if y >= 0 && y < len(grid) && x >= 0 && x < len(grid[y]) && grid[y][x] != companionToneNone {
				grid[y][x] = companionToneRamp0
			}
		}
	}

	if size.width != companionVolumeFullWidth {
		return
	}
	eyes := companionEyeCentres(pose.head, size, scale)
	for _, offset := range []float64{-0.09, 0, 0.09} {
		for _, side := range []float64{-1, 1} {
			for step := 0; step < 3; step++ {
				point := companionWhiskerPixel(pose.muzzle, offset, side, step, size, scale, eyes)
				if point[1] >= 0 && point[1] < len(grid) && point[0] >= 0 && point[0] < len(grid[point[1]]) {
					grid[point[1]][point[0]] = companionTonePupil
				}
			}
		}
	}
}

// companionVolumePixel is one pixel of the drawing outside the face: the volume's own
// shading where the field covers it, the ground's shadow where it does not. The shadow is
// asked for last because it is what the creature stands on: a pixel the creature covers
// is fur, whatever the ground under it is doing.
func companionVolumePixel(pose companionPose, heights [][]float64, size companionVolumeSize, scale float64, px, py int) companionTone {
	if heights[py+1][px+1] > 0 {
		return companionShadeTone(heights, px, py, scale)
	}
	if pose.shadow.field(companionWorldX(px, size, scale), companionWorldY(py, size, scale)) >= 1 {
		return companionDitherTone(companionShadowLevel, px, py)
	}
	return companionToneNone
}

// companionVolumeTones draws one frame of the volumetric sprite: a grid of tones, one per
// pixel, that the half-block encoder turns into the rows of the sprite. It is pure --
// state, tick, gaze, tremble and rung in, tones out, with no model, no clock and no
// terminal -- so a test can pin the exact frame a tick draws, and calling it twice returns
// the same grid.
func companionVolumeTones(state companionState, tick int, gaze companionGaze, shiver int, size companionVolumeSize, hop ...int) [][]companionTone {
	pose := companionPoseFor(state, tick, gaze, shiver, size, hop...)
	scale := companionVolumeScale(size)
	heights := pose.heightField(size, scale)
	inside := func(px, py int) bool {
		return px >= 0 && py >= 0 && px < size.width && py < size.rows && heights[py+1][px+1] > 0
	}

	grid := make([][]companionTone, size.rows)
	for py := range grid {
		grid[py] = make([]companionTone, size.width)
		for px := range grid[py] {
			grid[py][px] = companionVolumePixel(pose, heights, size, scale, px, py)
		}
	}

	companionPaintAnatomyDetails(grid, pose, size, scale)
	for _, centre := range companionEyeCentres(pose.head, size, scale) {
		companionPaintEye(grid, centre, size, companionEyeMarkFor(state), gaze)
	}
	companionPaintMuzzle(grid, inside, pose.muzzle, size, scale, state)

	// Extreme horizontal gaze may move only the named eye boxes and the skull/ear
	// silhouette band. Keep the rest of the shaded volume byte-for-byte at its neutral
	// geometry so a head turn cannot drag the torso, limbs, tail or ground shadow.
	if gaze.X != 0 {
		baselineGaze := companionGaze{Y: gaze.Y}
		baseline := companionVolumeTones(state, tick, baselineGaze, shiver, size, hop...)
		baselinePose := companionPoseFor(state, tick, baselineGaze, shiver, size, hop...)
		shiftedEyes := companionEyeCentres(pose.head, size, scale)
		baselineEyes := companionEyeCentres(baselinePose.head, size, scale)
		for py := range grid {
			for px := range grid[py] {
				if grid[py][px] == baseline[py][px] {
					continue
				}
				inEye := false
				for _, centre := range append(baselineEyes[:], shiftedEyes[:]...) {
					inEye = inEye || companionEyeBox(centre, size, px, py)
				}
				if !inEye && !companionHeadOutlineBand(baselinePose, size, scale, px, py) && !companionHeadOutlineBand(pose, size, scale, px, py) {
					grid[py][px] = baseline[py][px]
				}
			}
		}
	}
	return grid
}

// companionShiverPx is how far the creature is trembling on this frame: nothing unless it
// is flinching, and then one pixel either way on alternate frames, for the first few
// frames after a key. The bound is the point of it. A tremble that ran for as long as an
// error stayed on screen would repaint the creature's rows on every frame over a screen
// nobody was touching, which is exactly the regime this repository measured and fixed;
// a tremble that stops after three frames leaves the creature crouched, which is the part
// of a startle a reader can still see at this size.
func (m Model) companionShiverPx() int {
	if m.companionStateNow() != companionFlinchingState || m.CompanionIdle >= companionShiverTicks {
		return 0
	}
	return companionShiverPx - 2*companionShiverPx*(m.AnimTick%2)
}

// companionInk is the palette the volume is drawn with, resolved once when the model is
// built -- resolving it asks the terminal about its background, which a render may not do,
// so the answer lives on the model the way the animation gate does.
//
// It is a ramp and four marks rather than a list of colours. The ramp is the shading: five
// tones from the deepest fold to the lit fur, blended from the theme's own entries because
// a theme written for text does not carry five fur tones. The marks are the only pixels
// that are not the volume: the eye's light disc, the pupil -- and the lid and the mouth
// line, which are the same dark -- the single pixel of glint, and the rose nose.
//
// The loudest tone in the palette is spent on the glint alone, which is one pixel of the
// whole sprite: the creature is decoration, and the reader's eye should find the small
// thing the gaze moves rather than the body around it. The nose and the mouth are the only
// other accents. The dark end of the ramp is the palette's own ink, which is its text
// colour on a light terminal and its background on a dark one -- a near-white fold on a
// dark terminal would read as no shading at all.
type companionInk struct {
	ramp  [companionRampSteps]color.RGBA
	eye   color.RGBA
	pupil color.RGBA
	glint color.RGBA
	nose  color.RGBA
}

// companionInkFor resolves that palette against the terminal's background. The two
// ramps run in the same direction -- darkest fold first -- but they are built from
// different places in the palette, because every entry in the light column is dark: on a
// light terminal the lit side of the creature is a pale grey rather than a bright one, so
// that it stays visible against a white background, and the folds are the palette's own
// ink.
func companionInkFor(dark bool) companionInk {
	pick := func(c lipgloss.AdaptiveColor) string {
		if dark {
			return string(c.Dark)
		}
		return string(c.Light)
	}
	background := parseHexColour(pick(Background))
	muted := parseHexColour(pick(TextMuted))
	fur := parseHexColour(pick(Secondary))
	text := parseHexColour(pick(Text))

	ramp := [companionRampSteps]color.RGBA{
		blendColour(text, muted, 0.5),
		muted,
		blendColour(muted, background, 0.45),
		blendColour(muted, background, 0.72),
		blendColour(muted, background, 0.9),
	}
	pupil := text
	if dark {
		ramp = [companionRampSteps]color.RGBA{
			blendColour(background, muted, 0.45),
			muted,
			blendColour(muted, fur, 0.55),
			fur,
			blendColour(fur, text, 0.55),
		}
		// On a dark terminal the pupil is the terminal's own background, exactly as the
		// shipped pixel cat's outline was: the palette's near-white would put two bright
		// holes in a dark face.
		pupil = background
	}
	return companionInk{
		ramp:  ramp,
		eye:   ramp[companionRampSteps-1],
		pupil: pupil,
		glint: parseHexColour(pick(Accent)),
		nose:  pupil,
	}
}

// blendColour mixes two palette entries, which is where the ramp's intermediate steps come
// from: they are places the theme does not name, and rounding them to the nearest theme
// colour would flatten the two ends of the shading into nothing.
func blendColour(from, to color.RGBA, t float64) color.RGBA {
	mix := func(a, b uint8) uint8 {
		return uint8(math.Round(float64(a) + (float64(b)-float64(a))*t))
	}
	return color.RGBA{R: mix(from.R, to.R), G: mix(from.G, to.G), B: mix(from.B, to.B), A: 0xff}
}

// parseHexColour reads the #rrggbb a theme colour is written as. The palette is
// the repository's own and every entry is that shape, so a value that is not is a
// programming error and falls back to black rather than panicking mid-render.
func parseHexColour(hex string) color.RGBA {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return color.RGBA{A: 0xff}
	}
	value, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return color.RGBA{A: 0xff}
	}
	return color.RGBA{R: uint8(value >> 16), G: uint8(value >> 8 & 0xff), B: uint8(value & 0xff), A: 0xff}
}

// colour returns the colour a tone is drawn with. The empty tone answers with the ramp's
// darkest step: the encoder draws a space for it and never asks, so the answer only has to
// be a colour and not the right one.
func (ink companionInk) colour(tone companionTone) color.RGBA {
	switch {
	case tone >= companionToneRamp0 && tone <= companionRampTone(companionRampSteps-1):
		return ink.ramp[tone]
	case tone == companionToneEye:
		return ink.eye
	case tone == companionTonePupil:
		return ink.pupil
	case tone == companionToneGlint:
		return ink.glint
	case tone == companionToneNose:
		return ink.nose
	}
	return ink.ramp[0]
}

// pen is one escape sequence for one tone, as a foreground or as a background.
func (ink companionInk) pen(tone companionTone, foreground bool) string {
	return string(appendColour(make([]byte, 0, 24), ink.colour(tone), foreground))
}

// cellStyle is the escape sequence and the glyph one cell of the sprite is drawn with: a
// space with no escape at all for a cell the creature does not cover, a solid block for one
// tone, and a half block with the two colours for two. A cell whose upper pixel is empty
// takes the lower half block rather than an upper one over a background colour, which is
// one escape sequence fewer on the sprite's edges.
func (ink companionInk) cellStyle(upper, lower companionTone) (string, rune) {
	switch {
	case upper == companionToneNone && lower == companionToneNone:
		return "", ' '
	case upper == companionToneNone:
		return ink.pen(lower, true), '\u2584'
	case lower == companionToneNone:
		return ink.pen(upper, true), '\u2580'
	case upper == lower:
		return ink.pen(upper, true), '\u2588'
	default:
		return ink.pen(upper, true) + ink.pen(lower, false), '\u2580'
	}
}

// row draws one row of the volume: two rows of tones folded into one line of half blocks.
// The style is written only where it changes from the cell before it, so a stretch of one
// tone -- most of the fur, and every solid run the dither makes -- is one escape sequence
// and its glyphs, and the empty cells of a sprite's edges cost one byte each.
func (ink companionInk) row(top, bottom []companionTone) string {
	var out strings.Builder
	out.Grow(len(top) * 8)
	last := ""
	for x := 0; x < len(top) && x < len(bottom); x++ {
		style, glyph := ink.cellStyle(top[x], bottom[x])
		if style == "" {
			if last != "" {
				out.WriteString("\x1b[0m")
				last = ""
			}
			out.WriteByte(' ')
			continue
		}
		if style != last {
			out.WriteString(style)
			last = style
		}
		out.WriteRune(glyph)
	}
	if last != "" {
		out.WriteString("\x1b[0m")
	}
	return out.String()
}

// appendColour appends one true-colour escape sequence: 38;2 for a foreground, 48;2
// for a background.
func appendColour(out []byte, c color.RGBA, foreground bool) []byte {
	if foreground {
		out = append(out, "\x1b[38;2;"...)
	} else {
		out = append(out, "\x1b[48;2;"...)
	}
	out = strconv.AppendUint(out, uint64(c.R), 10)
	out = append(out, ';')
	out = strconv.AppendUint(out, uint64(c.G), 10)
	out = append(out, ';')
	out = strconv.AppendUint(out, uint64(c.B), 10)
	return append(out, 'm')
}

// companionVolumeRows is the whole volumetric sprite, placed along the stage: the rung's
// rows of half blocks, each indented to the cell the creature is standing on, or nothing
// when this run does not draw the volume at all.
func (m Model) companionVolumeRows(stage int, size companionVolumeSize) []string {
	if !m.Animating || !m.PixelSprite || stage < size.width {
		return nil
	}
	grid := companionVolumeTones(m.companionStateNow(), m.AnimTick, m.CompanionGaze, m.companionShiverPx(), size, m.CompanionHop)
	pos := min(max(m.CompanionPos, 0), stage-size.width)
	indent := strings.Repeat(" ", pos)
	rows := make([]string, 0, size.rows/2)
	for py := 0; py+1 < size.rows; py += 2 {
		rows = append(rows, indent+m.ink.row(grid[py], grid[py+1]))
	}
	return rows
}

// pixelSpriteGate answers whether this run may draw the volumetric sprite: the switch is
// not off and the terminal reports true colour. It reads the environment and the terminal
// once, when the model is built; the render path reads the model's answer and never the
// environment.
func pixelSpriteGate() bool {
	return pixelSpriteAllowed(termenv.ColorProfile(), os.Getenv(envSprite))
}

// pixelSpriteAllowed is that decision as a function of its two inputs, which is what makes
// the floor testable without a terminal: true colour draws the volume, and a terminal that
// reports sixteen colours, eight or none draws the glyph cat instead -- the shading is the
// drawing, so a tier that cannot show it is not attempted. DOTFILES_SPRITE=0 is the same
// refusal for a terminal that reports true colour and renders block glyphs badly.
func pixelSpriteAllowed(profile termenv.Profile, spriteEnv string) bool {
	if spriteEnv == "0" {
		return false
	}
	return profile == termenv.TrueColor
}

// ============================================================================
// THE STATE THE MODEL IS IN
// ============================================================================

// companionStageWidth is the row the creature walks along: the frame's inner
// width, which is what the rules above the footer and under the header span.
func companionStageWidth(m Model) int {
	return layoutFor(m).Inner
}

// companionStateNow is the state the creature is drawn in, read from the model
// and nothing else. The order of the cases is the reactions' precedence, and the
// blink and the yawn sit below the resting states they qualify: a walking or
// sleeping creature has no business blinking, and a reaction outranks both.
func (m Model) companionStateNow() companionState {
	switch {
	case m.companionFlinching():
		return companionFlinchingState
	case m.companionAlerting():
		return companionAlertState
	case m.companionPleased():
		return companionPleasedState
	case m.companionAsleepNow():
		return companionAsleepState
	case m.companionYawning():
		return companionYawningState
	case m.CompanionMoving:
		return companionWalkingState
	case m.companionBlinking():
		return companionBlinkingState
	default:
		return companionIdleState
	}
}

// companionBlinking reports whether this tick is a blink. It is the tick counter
// read at a modulus, not a second clock, so the blink is as pinnable as any other
// frame and it never fires while the creature is walking or reacting.
func (m Model) companionBlinking() bool {
	return m.AnimTick%companionBlinkTicks == companionBlinkTicks-1
}

// companionYawning reports whether the quiet run has reached the stretch before
// sleep. It is the last companionYawnTicks of the idle counter, so the creature
// yawns once and then sleeps, and waking it resets the stretch and the yawn with
// it.
func (m Model) companionYawning() bool {
	return m.CompanionIdle >= companionSleepTicks-companionYawnTicks
}

// companionFlinching reports whether the run is showing a failure: the error
// screen itself, an error the model is carrying, or a trainer result screen that
// says the last answer was wrong.
func (m Model) companionFlinching() bool {
	if m.Screen == ScreenError || m.ErrorMsg != "" {
		return true
	}
	correct, showing := m.companionVerdict()
	return showing && !correct
}

// companionPleased reports whether the creature is celebrating: a step that has
// just finished is worth a few ticks, and a trainer result screen that says the
// answer was right is worth one for as long as it stays on screen.
func (m Model) companionPleased() bool {
	if m.CompanionPleased > 0 {
		return true
	}
	correct, showing := m.companionVerdict()
	return showing && correct
}

// companionVerdict is the trainer's verdict on the last answer, and whether the
// screen on screen is showing one. Only the two result screens show a verdict:
// the exercise screens carry TrainerLastCorrect too, but it is false on a model
// that has answered nothing, so reading it there would flinch at a wrong answer
// nobody gave.
func (m Model) companionVerdict() (correct, showing bool) {
	switch m.Screen {
	case ScreenTrainerResult, ScreenTrainerBossResult:
		return m.TrainerLastCorrect, true
	}
	return false, false
}

// companionAlerting reports whether the user is pointing at something that
// overwrites or discards their files. Two screens are alert on every row -- the
// one that chooses a backup to restore and the one that confirms the restore --
// and the screen that confirms the installation overwrites the configs it just
// listed. Everywhere else the row under the cursor decides, so the main menu is
// alert on "Restore from Backup" and quiet on the entries that only read.
func (m Model) companionAlerting() bool {
	switch m.Screen {
	case ScreenBackupConfirm, ScreenRestoreBackup, ScreenRestoreConfirm:
		return true
	}
	return companionDestructiveRow(m.companionSelectedRow())
}

// companionDestructiveWords are the words a menu row uses when it throws
// something away. The list is deliberately short, and a test walks every screen's
// options against it: a row that adds a fourth way to lose data is caught by that
// test rather than by someone remembering this list.
var companionDestructiveWords = []string{"restore", "delete", "erase", "without backup"}

// companionDestructiveRow reports whether one menu row names a destructive
// action. It is case-insensitive because the rows carry their own capitalisation
// and the labels are written for the reader rather than for this check.
func companionDestructiveRow(row string) bool {
	lowered := strings.ToLower(row)
	for _, word := range companionDestructiveWords {
		if strings.Contains(lowered, word) {
			return true
		}
	}
	return false
}

// companionSelectedRow is the option the cursor is on, or the empty string when
// the screen offers no options at all. It reads the same list the screen draws,
// so the creature reacts to the row the user can actually see selected.
func (m Model) companionSelectedRow() string {
	options := m.GetCurrentOptions()
	if m.Cursor < 0 || m.Cursor >= len(options) {
		return ""
	}
	return options[m.Cursor]
}

// companionTarget is the cell the cursor points at: the cursor's index times
// companionFollowCells, capped at the right edge of the stage. Spreading the whole
// index across the stage is what threw the creature tens of cells for one arrow
// key on a wide terminal; a fixed three cells a row makes one keypress a short
// stroll and keeps the creature near the menu it follows. It reports false when
// there is nothing to point at -- a screen with no menu, like the installing
// screen or the trainer's exercises -- and the creature then simply stands still.
func (m Model) companionTarget(stage int) (int, bool) {
	options := m.GetCurrentOptions()
	if len(options) < 2 {
		return 0, false
	}
	limit := stage - companionCellWidth
	if limit < 1 {
		return 0, false
	}
	index := min(max(m.Cursor, 0), len(options)-1)
	return min(index*companionFollowCells, limit), true
}

// armCompanionFollow points the companion at the row the cursor is on and turns
// its gaze with it. A key that moved the selection, or a key that opened another
// screen, is the event; the walking itself happens on the ticks that follow,
// which is why moving the cursor visibly moves the creature over the following
// ticks instead of teleporting it. A screen with nothing to point at leaves it
// standing still and looking straight ahead.
func (m *Model) armCompanionFollow() {
	if _, ok := m.companionTarget(companionStageWidth(*m)); ok {
		m.CompanionFollow = true
	}
	m.aimCompanion()
}

// advanceCompanion is the creature's whole clock: it ages the idle stretch, ages
// a celebration, takes one step and, when that step moved it, points the gaze at
// what it now sees. It is called from the frame tick and from nowhere else, so the
// render path never moves anything and the same model and tick always draw the
// same bytes.
func (m *Model) advanceCompanion() {
	m.CompanionIdle++
	if m.CompanionPleased > 0 {
		m.CompanionPleased--
	}
	// The hop is aged here rather than in the renderer for the same reason the
	// celebration is: it is a frame count the tick owns, so a snapshot can pin the
	// jump it is in the middle of. It ages before the sleep return below, because a
	// click that lands on a sleeping creature must still land.
	if m.CompanionHop > 0 {
		m.CompanionHop--
	}

	// A sleeping companion is still. The frame says asleep, and a creature that
	// kept walking while asleep would contradict its own face; it also means an
	// idle run settles into a view string that no longer changes, so the renderer
	// stops repainting anything at all. Its gaze is left where it fell asleep, and
	// only the pointer -- which wakes it -- turns it again.
	if m.companionAsleepNow() {
		m.CompanionMoving = false
		return
	}

	// The gaze is aimed only when the tick actually moved the creature. The cell it
	// stands on is half of what the gaze is computed from, so a step can turn the
	// eyes; a tick that moved nothing must change nothing, or the pupils would
	// settle a frame after the target did and a still screen would flicker. The
	// other events that change what it sees -- the model's construction and a key
	// that moved the cursor or changed the screen -- aim the gaze themselves, and so
	// does a pointer event.
	before := m.CompanionPos
	m.stepCompanion()
	if m.CompanionPos != before {
		m.aimCompanion()
	}
}

// stepCompanion takes one step toward the row the cursor points at, and does
// nothing at all when there is nothing to walk toward. Standing still is the
// default: the creature walks only while the walk is armed, and a creature with
// nothing to do does nothing.
func (m *Model) stepCompanion() {
	stage := companionStageWidth(*m)
	limit := stage - companionCellWidth
	if limit < 1 {
		m.CompanionPos, m.CompanionMoving = 0, false
		return
	}
	pos := min(max(m.CompanionPos, 0), limit)

	// Standing still is the default and the whole of the quiet: the creature walks
	// only to reach the row the cursor points at, and a creature with nothing to do
	// does nothing. That is what makes a screen nobody is typing into settle into a
	// view string that never changes, so the renderer writes no bytes at all while
	// the user reads -- which is the difference between a pet and a flicker.
	target, ok := m.companionTarget(stage)
	if !ok || !m.CompanionFollow {
		m.CompanionFollow, m.CompanionMoving = false, false
		return
	}
	distance := companionDistance(pos, target)
	if distance == 0 {
		m.CompanionFollow, m.CompanionMoving = false, false
		return
	}

	// Braking: the last few cells are crossed on every other frame, so the arrival
	// is a step and not a stop. A step is never more than companionStepCells, so a
	// long walk takes as many frames as it has cells and can be timed exactly.
	if distance <= companionBrakeCells && m.AnimTick%companionBrakeFrames == 1 {
		m.CompanionMoving = false
		return
	}
	step := min(distance, companionStepCells)
	if target > pos {
		pos += step
	} else {
		pos -= step
	}
	m.CompanionPos, m.CompanionMoving = pos, true
}

// companionDistance is how far apart two cells are, always positive.
func companionDistance(a, b int) int {
	if a > b {
		return a - b
	}
	return b - a
}

// ============================================================================
// DRAWING AND PLACEMENT
// ============================================================================

// companionSprite is the creature's whole sprite: the rows it draws at the height
// the ladder chose for this screen, each placed along the stage, or nothing when
// there is no creature to draw -- animation off, or a stage too narrow to hold
// the cell. It reads the model and draws and does nothing else: no clock, no I/O
// and no mutation, so a render stays pure and a snapshot can pin a frame and a
// gaze.
func (m Model) companionSprite(stage, height int) []string {
	if !m.Animating {
		return nil
	}
	// A height the volume and the glyph tables share -- the mini volume is the same
	// row count as the glyph cat's full rung -- is disambiguated by the mode here:
	// the volume only draws where the encoder is on, and the glyph art otherwise.
	if m.PixelSprite {
		if size, ok := companionVolumeSizeFor(height); ok {
			return m.companionVolumeRows(stage, size)
		}
	}
	width := companionSpriteWidth(height)
	if width == 0 || stage < width {
		return nil
	}
	pos := min(max(m.CompanionPos, 0), stage-width)
	state := m.companionStateNow()
	artHeight := height
	// At the compact rung a celebration keeps the shipped one-row face. The
	// three-row pleased head has no room beside the end-of-run burst at the 80x24
	// floor; this is the expression for that rung, not a fallback selected from
	// the frame's spare rows. Other states still draw the full selected rung.
	if state == companionPleasedState && height == companionCompactHeight {
		artHeight = companionMiniHeight
	}
	art := companionArtFor(state, artHeight, m.AnimTick, m.CompanionGaze)
	rows := make([]string, len(art))
	for i, row := range art {
		rows[i] = strings.Repeat(" ", pos) + CompanionStyle.Render(row)
	}
	return rows
}

// companionRow is the creature's single row at the one-row height: the ladder's
// last step, and the row the trainer's screens place. It is the art the companion
// shipped with, unchanged, so a screen with one spare row draws exactly what it
// drew before the taller sets existed.
func (m Model) companionRow(stage int) string {
	rows := m.companionSprite(stage, companionMiniHeight)
	if len(rows) == 0 {
		return ""
	}
	return rows[0]
}

// trainerCompanionRows places the same ladder-selected sprite as the installer's
// frame in the rows the trainer's content did not need. The trainer owns its row
// composition, so it supplies the rows already spent and the legend's height;
// the ladder and renderer remain shared. At the documented floor only the legacy
// one-row slot is available, preserving the 80x24 layout exactly.
//
// That forced mini is the ladder's one stated exception, described where the
// ladder is stated (the package comment above): the trainer's floor body leaves
// exactly one spare row, so the compact rung cannot be drawn there, and the
// 80x24 goldens are pinned to the mini. Above trainerFloorHeight this function
// does not touch the height and the shared ladder chooses the rung.
func (m Model) trainerCompanionRows(stage, precedingRows, footerRows int) []string {
	height := m.companionHeightNow()
	if m.Height <= trainerFloorHeight {
		height = companionMiniHeight
	}
	padding := 0
	if m.Height > trainerFloorHeight {
		padding = m.Height - viewPaddingRows - footerRows - height - precedingRows
		if padding < 0 {
			padding = 0
		}
	}
	rows := m.companionSprite(stage, height)
	block := make([]string, padding+height)
	if len(rows) == 0 {
		return block
	}
	copy(block[len(block)-len(rows):], rows)
	return block
}

// placeCompanion puts the creature's sprite in the last rows the body did not
// need, below the panel summary, and returns the frame's rows unchanged when they
// cannot hold both. Nothing is dropped from a body to make room for a pet, and
// nothing is dropped from the summary either: the summary is the facts the panel
// was showing and the creature is decoration, so the summary's rows come off the
// spare rows first and the sprite the terminal's rung selected is drawn only if
// the rows it and the summary need are blank. When even that does not fit, the
// summary keeps its rows and the companion is not drawn. When both fit, the
// companion takes the rows nearest the footer and the summary sits above it, so
// neither displaces the other. The frame reserves the block before placing the
// body (see frameWithPanels), so this refusal is the last resort for a screen
// whose body genuinely fills the frame, not the ordinary case.
func (m Model) placeCompanion(placed, summary []string, stage int) []string {
	height := m.companionHeightNow()
	// Nothing to draw means nothing to reserve: the summary keeps the row the
	// creature would have taken, which is what a run without a terminal, or with
	// the creature off, has always rendered.
	sprite := m.companionSprite(stage, height)
	if len(sprite) == 0 || len(sprite) > height {
		return placeRotator(placed, summary)
	}
	// Hop anticipation, flight and landing are raster poses inside this reserved
	// block; they never borrow a row from the frame or move facts.
	// The block is what the sprite actually draws, not the rung that chose it: a
	// state whose art is one row tall - a celebration face - needs one row, and
	// demanding the rung's rows instead would silently drop it from a frame that
	// has room for exactly what it draws.
	blockStart, ok := companionBlockStart(placed, summary, len(sprite))
	if !ok {
		return placeRotator(placed, summary)
	}
	out := append([]string(nil), placed...)
	for i, line := range summary {
		out[blockStart-len(summary)+i] = line
	}
	copy(out[blockStart:], sprite)
	return out
}

// companionBlockStart is the row the creature's sprite starts at when the block
// and the summary above it are blank rows of placed, or false when they are not.
// The block is the sprite's own rows -- not the rung's -- so a one-row
// celebration face is not dropped from a frame that has room for exactly what it
// draws, and the check is shared with frameWithPanels so the reservation and the
// placement cannot disagree about where the block is.
func companionBlockStart(placed, summary []string, blockHeight int) (int, bool) {
	if blockHeight <= 0 || blockHeight > len(placed) {
		return 0, false
	}
	blockStart := len(placed) - blockHeight
	for i := range summary {
		row := blockStart - len(summary) + i
		if row < 0 || placed[row] != "" {
			return 0, false
		}
	}
	// The block has to be rows the body did not need. The rung is chosen by the
	// terminal rather than by the space this screen happens to leave, so a block
	// that lands on content is possible - and a fact still beats a decoration: the
	// creature is not drawn at all rather than painted over a row the screen is
	// stating something in.
	for row := blockStart; row < len(placed); row++ {
		if placed[row] != "" {
			return 0, false
		}
	}
	return blockStart, true
}

// companionSpareRows counts the blank rows at the bottom of a frame: the rows the
// body did not need, which the summary and the companion share between them.
func companionSpareRows(placed []string) int {
	spare := 0
	for spare < len(placed) && placed[len(placed)-1-spare] == "" {
		spare++
	}
	return spare
}
