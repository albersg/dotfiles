package tui

import "strings"

// ============================================================================
// THE COMPANION
// ============================================================================
//
// The installer's screens are wide and mostly empty below their content: at the
// 227x62 the layout was measured on, the main menu occupies 8 of 62 rows. The
// panels fill some of that room with facts; the companion is the one element in
// it that is not information. It is a small creature that strolls along the rows
// the body did not need, walks toward whatever the cursor points at, looks at it,
// sleeps when the user stops typing and reacts to what is on screen.
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
// Cost is bounded by a test rather than by this comment. The creature draws in the
// rows it owns, so a tick changes those rows and the renderer repaints those
// lines: TestCompanionTicksChangeOnlyItsOwnRows asserts exactly that over several
// screens and terminal sizes. The other half is asserted beside it -- a screen
// whose body fills its frame draws no companion, and the tick then changes nothing
// at all, so the view string is identical and the renderer skips the frame
// entirely.
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
	// the one-row art stops short of the right edge instead of reaching it, which
	// is invisible, where the five-row art reaching past it would cross the frame's
	// margin.
	companionCellWidth = companionFullWidth

	// companionStrollCells is how far it walks per frame when it is strolling.
	// One cell is what animTicksPerSecond frames a second deserves: eight cells a
	// second reads as a walk along a row that spans the terminal, where the two
	// cells a frame the slow clock needed would be a dash at this rate.
	companionStrollCells = 1

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

	// companionBlinkSeconds is how often an awake, idle creature blinks. The blink
	// lasts one frame, and it is derived from the tick counter rather than from a
	// second clock, so a snapshot can pin the blink without waiting for it.
	companionBlinkSeconds = 5

	// companionBlinkTicks is that interval in frames.
	companionBlinkTicks = companionBlinkSeconds * animTicksPerSecond

	// companionPleasedSeconds is how long a finished installation step is worth
	// celebrating: about a second, not forever.
	companionPleasedSeconds = 1

	// companionPleasedTicks is that celebration in frames.
	companionPleasedTicks = companionPleasedSeconds * animTicksPerSecond
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

// aimCompanion points the creature's gaze at whatever the cursor is on. It is
// called only from the two places that can change what the creature sees -- a key
// that moved the selection or changed the screen, and the tick that moved the
// creature -- so the render path draws a gaze it did not compute and a snapshot
// can pin one.
func (m *Model) aimCompanion() {
	target, ok := m.companionTarget(companionStageWidth(*m))
	if !ok {
		m.CompanionGaze = companionGaze{}
		return
	}
	m.CompanionGaze = companionGazeFor(target, companionSelectionRow, m.CompanionPos)
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
	case companionFullHeight:
		return companionFullWidth
	case companionCompactHeight:
		return companionCompactWidth
	case companionMiniHeight:
		return companionMiniWidth
	}
	return 0
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
	case m.CompanionIdle >= companionSleepTicks:
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

// companionTarget is the cell the cursor points at: the cursor's index spread
// across the stage, so the first row of a menu is the left edge and the last one
// the right. It reports false when there is nothing to point at -- a screen with
// no menu, like the installing screen or the trainer's exercises -- and the
// creature then simply strolls.
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
	return index * limit / (len(options) - 1), true
}

// armCompanionFollow points the companion at the row the cursor is on and turns
// its gaze with it. A key that moved the selection, or a key that opened another
// screen, is the event; the walking itself happens on the ticks that follow,
// which is why moving the cursor visibly moves the creature over the following
// ticks instead of teleporting it. A screen with nothing to point at leaves it
// strolling and looking straight ahead.
func (m *Model) armCompanionFollow() {
	if _, ok := m.companionTarget(companionStageWidth(*m)); ok {
		m.CompanionFollow = true
	}
	m.aimCompanion()
}

// advanceCompanion is the creature's whole clock: it ages the idle stretch, ages
// a celebration, takes one step and points the gaze at what it now sees. It is
// called from the frame tick and from nowhere else, so the render path never
// moves anything and the same model and tick always draw the same bytes.
func (m *Model) advanceCompanion() {
	m.CompanionIdle++
	if m.CompanionPleased > 0 {
		m.CompanionPleased--
	}

	// A sleeping companion is still. The frame says asleep, and a creature that
	// kept strolling while asleep would contradict its own face; it also means an
	// idle run settles into a view string that no longer changes, so the renderer
	// stops repainting anything at all. Its gaze is left where it fell asleep.
	if m.CompanionIdle >= companionSleepTicks {
		m.CompanionMoving = false
		return
	}

	m.stepCompanion()
	m.aimCompanion()
}

// stepCompanion takes one step: toward the row the cursor points at while the
// walk is armed, and a fixed stroll otherwise.
func (m *Model) stepCompanion() {
	stage := companionStageWidth(*m)
	limit := stage - companionCellWidth
	if limit < 1 {
		m.CompanionPos, m.CompanionMoving = 0, false
		return
	}
	pos := min(max(m.CompanionPos, 0), limit)

	if target, ok := m.companionTarget(stage); ok && m.CompanionFollow {
		if target == pos {
			// It has arrived at the row the cursor points at. It stands there for
			// this tick -- the idle frame, which is what "it resumes strolling"
			// looks like on the tick it stops walking -- and strolls on from the
			// next one.
			m.CompanionFollow = false
			m.CompanionPos, m.CompanionMoving = pos, false
			return
		}
		// Half the distance per frame. Because the step is a fraction and not a
		// length, arrival takes log2(distance) frames: the time is set by how far
		// away the row is. At animTicksPerSecond a one-row cursor move is four to
		// six frames (about half a second to three quarters), and the far edge of
		// the widest stage is eight frames, one second. The step is shortest where
		// the eye is, one cell away, so the arrival reads as a step rather than as a
		// jump.
		step := (companionDistance(pos, target) + 1) / 2
		if target > pos {
			pos = min(pos+step, target)
		} else {
			pos = max(pos-step, target)
		}
		m.CompanionPos, m.CompanionMoving = pos, true
		return
	}

	// Strolling: a fixed step in the direction it is already going, turning at the
	// edge instead of walking off the row. The direction is stored so the turn is
	// visible -- the creature reverses and walks back the way it came.
	m.CompanionFollow = false
	direction := m.CompanionDir
	if direction == 0 {
		direction = 1
	}
	next := pos + direction*companionStrollCells
	if next < 0 || next > limit {
		direction = -direction
		next = min(max(pos+direction*companionStrollCells, 0), limit)
		if next == pos {
			// The row is narrower than one step: walk its whole length instead of
			// standing still at one end, so a two-column stage still moves.
			next = limit - pos
		}
	}
	m.CompanionDir = direction
	m.CompanionPos, m.CompanionMoving = next, next != pos
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
	height := companionHeight(companionSpareRows(placed) - len(summary))
	sprite := m.companionSprite(stage, height)
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
