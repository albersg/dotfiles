package tui

import (
	"image/color"
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
// At rest the creature moves nothing, so nothing it draws changes and the renderer
// writes no bytes at all: TestCompanionStaysPutWhenNothingHappens asserts the view
// is byte-identical across a run of ticks. Walking is the other regime: the sprite
// changes one column, every row of it moves with the move, and the renderer
// repaints every line those rows are on -- so the cost is the sprite's row count
// times the line width for as long as a stroll lasts, and zero afterwards.
// TestCompanionTicksChangeOnlyItsOwnRows bounds which rows a walking tick may
// touch, and TestCompanionCostHasTwoRegimes measures both regimes rather than
// pinning a byte count that the terminal's width decides.
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
	// the narrower glyph cells stop short of the right edge instead of reaching it,
	// which is invisible, where the sixteen-pixel sprite reaching past it would
	// cross the frame's margin.
	companionCellWidth = companionPixelWidth

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

	// companionHopTicks is how many frames the little jump a click earns lasts. It
	// is a frame count rather than a duration because what it counts really is
	// frames -- two consecutive frames of the same sprite drawn one row higher --
	// and at animTicksPerSecond a quarter of a second is as long as a jump should
	// take to read as a jump rather than as hovering.
	companionHopTicks = 2
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

// companionGaze is where the creature is looking: the horizontal pupil position
// (-1 left, 0 centre, +1 right) and the vertical one (-1 up, 0 level). Down is
// deliberately not drawn: the taller art has one interior row to spare for the
// eyes and a wrong-looking "down" would read worse than a missing one.
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
// the seam the pointer will feed: today the cell is the selection's, next task's
// is the pointer's, and this function is the only thing either of them has to
// satisfy. It is pure -- no model, no clock, no layout -- so every gaze can be
// pinned by a test.
func companionGazeFor(targetCol, targetRow, anchorCol int) companionGaze {
	gaze := companionGaze{}
	switch {
	case targetCol < anchorCol-companionGazeDeadZone:
		gaze.X = -1
	case targetCol > anchorCol+companionGazeDeadZone:
		gaze.X = 1
	}
	if targetRow < 0 {
		gaze.Y = -1
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
// no row of the sprite reaches past the cell it is placed in.
func companionSpriteWidth(height int) int {
	switch height {
	case companionPixelHeight:
		return companionPixelWidth
	case companionFullHeight:
		return companionFullWidth
	case companionCompactHeight:
		return companionCompactWidth
	case companionMiniHeight:
		return companionMiniWidth
	}
	return 0
}

// companionHeightNow is the ladder with the shaded sprite's step on top: the pixel
// sprite where the frame can hold it and the run may draw it, and the glyph ladder
// otherwise. The glyph ladder is untouched below it, which is what keeps the floor
// a floor: a terminal without true colour, a run with the sprite switched off and a
// frame with fewer than the sprite's rows all get the cat the glyph ladder picks.
func (m Model) companionHeightNow(spare int) int {
	if m.PixelSprite && spare >= companionPixelHeight {
		return companionPixelHeight
	}
	return companionHeight(spare)
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
// THE SHADED SPRITE
// ============================================================================
//
// The glyph cat is the floor: every terminal draws it. Above it there is one more
// step, drawn only where it can be drawn properly -- a terminal with true colour --
// and that step is the same cat as a shaded pixel sprite, sixteen pixels square,
// drawn as eight rows of half blocks with a foreground and a background colour per
// cell.
//
// Why a second sprite at all: the glyph art is a silhouette of punctuation, and at
// six or eight cells tall there is no room in it for a body, a cheek or a shaded
// ear. Half blocks give two pixels per cell vertically, so the same eight rows
// carry sixteen rows of drawing and the creature stops looking like a smiley and
// starts looking like a cat. The price is that it only reads in colour: with no
// colour every cell is a block glyph, so this sprite is drawn only when the
// terminal reports true colour and the glyph cat draws everywhere else. The ladder
// below still runs on the glyph heights, which is what makes the floor real.
//
// The art is a grid of tones rather than of glyphs, and the tone says which of the
// theme's colours a pixel takes. Six tones and the background: the outline is the
// palette's ink, the fur is two muted tones, the eyes -- the pupils, the lids and
// the expressions they draw -- take the one bright tone in the set, and the nose
// and mouth are the error rose. The one bright tone is spent on the eyes on
// purpose: the creature is decoration, and the reader's eye should find the small
// feature the gaze moves before it finds the body. The background tone is the
// theme's own background, so the pixels the cat does not cover blend with the
// terminal instead of drawing a box around it.
//
// The sprite is composed, not drawn per state, on the same terms as the glyph art:
// one block of face rows per state class, and the pupils are stamped into it. That
// is what lets one sprite carry eight states and six gazes without becoming
// fifty hand-drawn pictures.

// companionPixelWidth and companionPixelRows are the sprite's size in pixels: one
// pixel per column and two per half-block row, so the cell it occupies is
// companionPixelWidth columns wide and companionPixelHeight rows tall.
const (
	companionPixelWidth  = 16
	companionPixelRows   = 16
	companionPixelHeight = companionPixelRows / 2

	// companionInkReset retires the style a cell set. It is named rather than
	// written out at each site because missing one is not a cosmetic slip: a tone
	// left active paints every cell after it, which is how a sprite became a bar of
	// colour across the terminal.
	companionInkReset = "\x1b[0m"
)

// The tones. A pixel's tone picks its colour and nothing else: no state is carried
// by a tone that a colourless terminal would lose, because a colourless terminal
// does not draw this sprite at all.
const (
	companionPixelNone = ' ' // the theme's background: the pixels the cat does not cover
	companionPixelFurL = '.' // the light fur: the face between the eyes
	companionPixelFurM = 'o' // the mid fur: the body
	companionPixelInk  = '#' // the outline
	companionPixelEye  = '@' // the eyes: the pupils, the lids and the expressions they draw
	companionPixelRose = 'x' // the nose and the mouth
)

// The rows every state shares: the ears and brow above the face, and the nose,
// mouth and chin below it. Every row is exactly companionPixelWidth pixels wide,
// so the sprite is a rectangle and a one-cell step is always one column.
var (
	companionPixelTop = [6]string{
		`  o#        #o  `,
		`  o##      ##o  `,
		`  o###    ###o  `,
		` #o####  ####o# `,
		` #oooooooooooo# `,
		` #oooooooooooo# `,
	}
	companionPixelBottom = [6]string{
		` #oooooxxooooo# `,
		` #oooooxxooooo# `,
		` #oooox..xoooo# `,
		`  #oooooooooo#  `,
		`   #oooooooo#   `,
		`    ########    `,
	}
)

// companionPixelMouthOpen is the two rows the mouth takes when the creature yawns,
// in place of the two rows of companionPixelBottom that hold the closed mouth. An
// open mouth is the one expression the pixel sprite draws that the glyph art can
// only hint at, and it is worth the two rows: a yawn with a closed mouth is a cat
// with its eyes shut.
var companionPixelMouthOpen = [2]string{
	` #ooooxxxxoooo# `,
	` #ooooxxxxoooo# `,
}

// companionPixelEyes is one state's face: the four rows the eyes live in, and how
// wide the pupil pair is. A width of zero means the eyes are drawn into those rows
// and do not move -- a cat with its eyes shut has nothing to look with -- while a
// width of two or three is a pupil pair the gaze can stamp at three columns and two
// rows.
//
// The socket area is the ten pixels between the head's own sides, columns 3 to 12,
// and every pupil position the gaze can ask for lands inside it, which is why the
// pupils can never be drawn outside the face.
type companionPixelFace struct {
	rows   [4]string
	pupils int
}

var (
	companionPixelFaceOpen   = companionPixelFace{rows: [4]string{` #o..........o# `, ` #o..@@oo@@..o# `, ` #o..@@oo@@..o# `, ` #o..........o# `}, pupils: 2}
	companionPixelFaceWide   = companionPixelFace{rows: [4]string{` #o..........o# `, ` #o.@@@..@@@.o# `, ` #o.@@@..@@@.o# `, ` #o..........o# `}, pupils: 3}
	companionPixelFaceShut   = companionPixelFace{rows: [4]string{` #o..........o# `, ` #o.@@@..@@@.o# `, ` #o..........o# `, ` #o..........o# `}}
	companionPixelFaceHappy  = companionPixelFace{rows: [4]string{` #o..@....@..o# `, ` #o.@.@..@.@.o# `, ` #o..........o# `, ` #o..........o# `}}
	companionPixelFaceSquint = companionPixelFace{rows: [4]string{` #o.@......@.o# `, ` #o..@....@..o# `, ` #o.@......@.o# `, ` #o..........o# `}}
)

// companionPixelFaceFor is the face a state is drawn with. The shut face is shared
// by the three states whose eyes are closed -- a blink, a sleep and the eyes of a
// yawn -- because a closed eye is a closed eye, and the mouth is what tells those
// three apart.
func companionPixelFaceFor(state companionState) companionPixelFace {
	switch state {
	case companionBlinkingState, companionAsleepState, companionYawningState:
		return companionPixelFaceShut
	case companionAlertState:
		return companionPixelFaceWide
	case companionPleasedState:
		return companionPixelFaceHappy
	case companionFlinchingState:
		return companionPixelFaceSquint
	default:
		return companionPixelFaceOpen
	}
}

// companionPixelSocketLeft and companionPixelSocketRight are the columns the pupil
// pair is stamped at when the creature looks straight ahead. Looking to one side
// moves both pupils by one column, which is the whole of the horizontal gaze here.
const (
	companionPixelSocketLeft  = 5
	companionPixelSocketRight = 9
	companionPixelSocketRow   = 1
)

// companionPixelGrid is the sprite's whole pixel grid for one state and gaze: the
// shared rows, the state's face with the gaze stamped into it, and the mouth the
// state calls for. It is pure -- state in, pixels out, no model and no clock -- so
// a test can pin the exact grid a state draws.
func companionPixelGrid(state companionState, tick int, gaze companionGaze) [companionPixelRows]string {
	face := companionPixelFaceFor(state)
	face = companionPixelGaze(face, gaze)

	mouth := [2]string{companionPixelBottom[1], companionPixelBottom[2]}
	if state == companionYawningState {
		mouth = companionPixelMouthOpen
	}

	var grid [companionPixelRows]string
	copy(grid[0:6], companionPixelTop[:])
	copy(grid[6:10], face.rows[:])
	grid[10] = companionPixelBottom[0]
	grid[11], grid[12] = mouth[0], mouth[1]
	copy(grid[13:16], companionPixelBottom[3:])

	// The pixel sprite can move by half a cell, which the glyph art cannot: the walk
	// bobs one pixel row on alternate frames, and a sleeping cat sags the same way.
	// That is one row of drawing moved, not a row of the frame, so it costs nothing
	// and it reads as breathing rather than as a jump.
	shift := 0
	switch {
	case state == companionAsleepState:
		shift = 1
	case state == companionWalkingState && tick%2 == 1:
		shift = 1
	}
	if shift == 1 {
		blank := strings.Repeat(string(companionPixelNone), companionPixelWidth)
		copy(grid[1:], grid[:companionPixelRows-1])
		grid[0] = blank
	}
	return grid
}

// companionPixelGaze stamps the pupil pair at the position the gaze asks for: one
// column to either side of the middle and, for a creature looking up, one pixel row
// higher. A face whose pupils do not move is returned as it is, which is how a
// closed eye stays closed whatever the pointer does.
func companionPixelGaze(face companionPixelFace, gaze companionGaze) companionPixelFace {
	if face.pupils == 0 {
		return face
	}
	socket := [4]string{}
	for i, row := range face.rows {
		socket[i] = companionPixelClearSockets(row)
	}

	col := companionPixelSocketLeft + gaze.X
	row := companionPixelSocketRow
	if gaze.Y < 0 {
		row = 0
	}
	for _, pupil := range []int{col, companionPixelSocketRight + gaze.X} {
		for x := pupil; x < pupil+face.pupils; x++ {
			for y := row; y < row+2; y++ {
				socket[y] = companionPixelSetPixel(socket[y], x, companionPixelEye)
			}
		}
	}
	return companionPixelFace{rows: socket, pupils: face.pupils}
}

// companionPixelClearSockets returns a face row with its pupils taken out: every
// mark inside the socket area becomes light fur again, so a gaze that moved them
// leaves the row they came from clean instead of doubled. It clears the eye tone
// and the outline alike, because the two have both been the pupil mark at some
// point in the art's history and a stale one would be left behind. The head's
// outline and its two inner edges are outside the socket area and are left alone.
func companionPixelClearSockets(row string) string {
	pixels := []rune(row)
	for x := companionPixelSocketLeft - 2; x <= companionPixelSocketRight+3 && x < len(pixels); x++ {
		if pixels[x] == companionPixelInk || pixels[x] == companionPixelEye {
			pixels[x] = companionPixelFurL
		}
	}
	return string(pixels)
}

// companionPixelSetPixel writes one tone into one pixel of a row. It is a write
// and not an insert: the grid's width is the art's, and a gaze that asked for a
// pixel outside it would be a bug in the position arithmetic rather than something
// to draw past the edge of the face.
func companionPixelSetPixel(row string, x int, tone rune) string {
	pixels := []rune(row)
	if x < 0 || x >= len(pixels) {
		return row
	}
	pixels[x] = tone
	return string(pixels)
}

// companionInk is the colours the pixel sprite is drawn with, resolved once when
// the model is built. Resolving them asks the terminal about its background, which
// a render may not do, so the answer lives on the model the way the animation gate
// does.
//
// The palette is chosen so the creature is decoration and not information: the
// body and the face are muted tones, so the cat is not the loudest object on a
// screen whose words are, and the one bright tone in the set is spent on the eyes,
// which are the small feature the gaze moves and the one the reader's own eye
// should find first. The nose and mouth are the error rose, the only other accent.
//
// The outline is the palette's dark ink on a light terminal and its background on a
// dark one: the theme's text colour is near-white on a dark terminal, and a
// near-white outline around muted fur would be no outline at all. The background
// tone is the theme's own background, so the pixels the cat does not cover blend
// with the terminal.
type companionInk struct {
	none, furLight, furMid, ink, eye, rose color.RGBA
}

// companionInkFor resolves the palette against the terminal's background.
func companionInkFor(dark bool) companionInk {
	pick := func(c lipgloss.AdaptiveColor) string {
		if dark {
			return string(c.Dark)
		}
		return string(c.Light)
	}
	outline := Background
	if !dark {
		outline = Text
	}
	return companionInk{
		none:     parseHexColour(pick(Background)),
		furLight: parseHexColour(pick(Secondary)),
		furMid:   parseHexColour(pick(TextMuted)),
		ink:      parseHexColour(pick(outline)),
		eye:      parseHexColour(pick(Accent)),
		rose:     parseHexColour(pick(Error)),
	}
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

// companionPixelTone returns the colour a tone is drawn with.
func (ink companionInk) tone(tone rune) color.RGBA {
	switch tone {
	case companionPixelFurL:
		return ink.furLight
	case companionPixelFurM:
		return ink.furMid
	case companionPixelInk:
		return ink.ink
	case companionPixelEye:
		return ink.eye
	case companionPixelRose:
		return ink.rose
	default:
		return ink.none
	}
}

// companionPixelRow draws one row of the sprite: two pixel rows folded into one
// line of half blocks. A cell whose two pixels are the same tone is a solid block in
// that tone; a cell whose pixels differ is the upper half block with the upper
// pixel as its foreground and the lower one as its background, which is the trick
// the whole tier rests on. A cell that is background on both rows is a space with no
// escape sequence at all, so the empty half of the sprite costs a byte a cell.
//
// The style is written only when it changes from the cell before it, so a row of
// one tone -- most of the fur -- is one escape sequence and sixteen glyphs.
func (ink companionInk) row(top, bottom string) string {
	var out strings.Builder
	out.Grow(companionPixelWidth * 4)
	last := ""
	for x := 0; x < companionPixelWidth && x < len(top) && x < len(bottom); x++ {
		upper, lower := rune(top[x]), rune(bottom[x])
		switch {
		case upper == companionPixelNone && lower == companionPixelNone:
			// A transparent cell has to retire the style before writing its space. A
			// space is painted with whatever colour is still active, and a terminal's
			// escape state outlives the line: without this reset the last tone of a row
			// bleeds through every cell after it, to the end of the row and on through
			// the frame's own padding. It is not a subtle tint either - half the sprite's
			// cells set a background, so the leak is a solid bar of colour across the
			// terminal.
			if last != "" {
				out.WriteString(companionInkReset)
				last = ""
			}
			out.WriteByte(' ')
		case upper == lower:
			style := ink.style(ink.tone(upper), nil)
			if style != last {
				out.WriteString(style)
				last = style
			}
			out.WriteRune('\u2588')
		default:
			style := ink.style(ink.tone(upper), &lower)
			if style != last {
				out.WriteString(style)
				last = style
			}
			out.WriteRune('\u2580')
		}
	}
	if last != "" {
		out.WriteString(companionInkReset)
	}
	return out.String()
}

// style is the escape sequence one cell is drawn with: the tone as the foreground
// and, when the cell's two pixels differ, the lower tone as the background. A nil
// background means the cell is a solid block in one colour, which needs no second
// colour at all. It is built with strconv rather than with a format string because a
// sprite frame is a hundred and twenty-eight cells and the formatting was most of
// what the tier cost.
func (ink companionInk) style(fg color.RGBA, bg *rune) string {
	out := make([]byte, 0, 48)
	out = appendColour(out, fg, true)
	if bg != nil {
		out = appendColour(out, ink.tone(*bg), false)
	}
	return string(out)
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

// companionPixelSpriteRows is the whole pixel sprite, placed along the stage: eight
// rows of half blocks, each indented to the cell the creature is standing on, or
// nothing when this run does not draw the sprite at all.
func (m Model) companionPixelSpriteRows(stage int) []string {
	if !m.Animating || !m.PixelSprite || stage < companionPixelWidth {
		return nil
	}
	grid := companionPixelGrid(m.companionStateNow(), m.AnimTick, m.CompanionGaze)
	pos := min(max(m.CompanionPos, 0), stage-companionPixelWidth)
	indent := strings.Repeat(" ", pos)
	rows := make([]string, 0, companionPixelHeight)
	for i := 0; i < companionPixelRows; i += 2 {
		rows = append(rows, indent+m.ink.row(grid[i], grid[i+1]))
	}
	return rows
}

// pixelSpriteGate answers whether this run may draw the shaded sprite: the switch
// is not off and the terminal reports true colour. It reads the environment and the
// terminal once, when the model is built; the render path reads the model's answer
// and never the environment.
func pixelSpriteGate() bool {
	if os.Getenv(envSprite) == "0" {
		return false
	}
	return lipgloss.ColorProfile() == termenv.TrueColor
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
	width := companionSpriteWidth(height)
	if width == 0 || stage < width {
		return nil
	}
	if height == companionPixelHeight {
		return m.companionPixelSpriteRows(stage)
	}
	pos := min(max(m.CompanionPos, 0), stage-width)
	art := companionArtFor(m.companionStateNow(), height, m.AnimTick, m.CompanionGaze)
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

// trainerCompanionRow is the companion's row on a trainer screen. The trainer's
// exercise and boss screens spend every row they are given -- their code window
// takes whatever the chrome leaves -- so there is no spare row for the frame's
// placement to find. What they do have is one blank spacer row above the legend,
// which is the row nearest the footer and a row the body did not need; the
// companion takes that one row where the frame's screens take the last spare
// ones. One spare row is the ladder's last step, so the trainer draws the
// one-row art and keeps the screen exactly as tall as it was. It draws nothing
// when the gate is off, so the spacer stays blank and the screen renders exactly
// the bytes it did before the creature existed.
func (m Model) trainerCompanionRow(stage int) string {
	return m.companionRow(stage)
}

// placeCompanion puts the creature's sprite in the last rows the body did not
// need, below the panel summary, and returns the frame's rows unchanged when they
// cannot hold both. Nothing is dropped from a body to make room for a pet, and
// nothing is dropped from the summary either: the summary is the facts the panel
// was showing and the creature is decoration, so the summary's rows come off the
// spare rows first and the ladder picks the tallest sprite the rest can hold. When
// even the one-row art does not fit, the summary keeps its rows and the companion
// is not drawn. When both fit, the companion takes the rows nearest the footer and
// the summary sits above it, so neither displaces the other.
func (m Model) placeCompanion(placed, summary []string, stage int) []string {
	spare := companionSpareRows(placed) - len(summary)
	height := m.companionHeightNow(spare)
	sprite := m.companionSprite(stage, height)
	// A click's hop lifts the creature off the ground: the sprite gains a blank row
	// under it, which is what makes the lift visible on a grid of cells, and it
	// costs one spare row more than the sprite itself. Where the frame has no such
	// row the jump is simply not drawn -- the celebration still shows in the face,
	// which is the ladder's rule that the creature never takes a row a fact needs.
	if len(sprite) > 0 && m.CompanionHop > 0 && spare >= height+1 {
		sprite = append(sprite, "")
	}
	if len(sprite) == 0 || len(sprite) > len(placed) {
		return placeRotator(placed, summary)
	}
	out := placeRotator(placed[:len(placed)-len(sprite)], summary)
	return append(out, sprite...)
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
