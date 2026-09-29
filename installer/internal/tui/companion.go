package tui

import "strings"

// ============================================================================
// THE COMPANION
// ============================================================================
//
// The installer's screens are wide and mostly empty below their content: at the
// 227x62 the layout was measured on, the main menu occupies 8 of 62 rows. The
// panels fill some of that room with facts; the companion is the one element in
// it that is not information. It is a five-to-seven character creature that strolls
// along one row of the frame, walks toward whatever the cursor points at, sleeps
// when the user stops typing and reacts to what is on screen.
//
// The art is drawn here, in companionFrames, and it is deliberately plain
// ASCII. Termux, a 16-colour terminal and a terminal with no font fallback all
// draw (o.o), and the state has to read from the glyphs rather than from a
// colour: the eyes and the z say asleep before any tone does. Nothing was copied
// from a third-party mascot either -- the Go gopher is CC-BY, cowsay's cow is
// GPL-ish and nyancat's cat belongs to its author -- so this repository's
// attribution surface stays empty.
//
// Nothing here reads the clock. The frame comes from the model's tick counter
// and the cell from CompanionPos, so the same model and tick render the same
// bytes on every run and a snapshot can pin a frame instead of flaking on time.
// The render path is pure: companionRow draws from the model and moves nothing,
// and advanceCompanion, called only from the frame tick (animTickMsg), is the one
// writer of everything below.
//
// Cost is bounded by a test rather than by this comment. The creature draws in
// one row, so a tick changes one row of the view and the renderer repaints that
// line: TestCompanionTicksChangeOnlyItsOwnRow asserts exactly that over several
// screens and terminal sizes. The other half is asserted beside it -- a screen
// whose body fills its frame draws no companion, and the tick then changes
// nothing at all, so the view string is identical and the renderer skips the
// frame entirely.
//
// With the animation gate off there is no companion anywhere: a frozen pet is
// not the point, and the gate already means "this run cannot animate".

// The creature's cell, its pace and its two clocks. Each is a constant with a
// test beside it instead of a number buried in the arithmetic.
//
// Two kinds of number live here, and the name says which one it is. A DURATION
// is written in seconds (fooSeconds) and turned into frames through
// animTicksPerSecond, so moving the frame rate cannot silently change how long
// the creature sleeps or celebrates. A frame count is written in frames
// (fooTicks); it is derived from a duration wherever what it measures is a length
// of time, and written directly only where the thing counted really is frames.
const (
	// companionCellWidth is the columns the sprite occupies wherever it stands:
	// the widest frame in the table. Fixing the cell rather than measuring each
	// frame means a one-cell step is always one column, so the creature never
	// stutters forward as its legs move.
	companionCellWidth = 7

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
	companionAsleepState
	companionAlertState
	companionPleasedState
	companionFlinchingState
)

// companionFrames is the whole art: one entry per frame, each one named by the
// state that draws it. It is a slice and not a map so that walking it -- here, in
// the drawing code and in a test -- has one order rather than whatever the
// runtime feels like, which is also what keeps the art reproducible.
var companionFrames = []struct {
	state companionState
	art   string
}{
	{companionIdleState, "(o.o)"},       // idle: awake and standing still
	{companionWalkingState, "(o.o)/"},   // walking: the leg kicked up
	{companionWalkingState, "(o.o)\\"},  // walking: the leg kicked down
	{companionAsleepState, "(-.-) z"},   // asleep: eyes shut, a z beside them
	{companionAlertState, "(O.O) !"},    // alert: eyes wide, an ! beside them
	{companionPleasedState, "\\(o.o)/"}, // pleased: both arms up
	{companionFlinchingState, "(>.<)"},  // flinch: eyes screwed shut
}

// companionArtFor is the frame a state draws at a tick. A state with one frame
// always draws it; the walk has two and alternates them on the tick's parity, so
// the leg moves on every tick it is walking. A state with no frame at all -- one
// a future edit adds without art -- draws the idle frame rather than an empty
// row, so a missing frame is visible in a snapshot instead of silently blank.
func companionArtFor(state companionState, tick int) string {
	var frames []string
	for _, f := range companionFrames {
		if f.state == state {
			frames = append(frames, f.art)
		}
	}
	if len(frames) == 0 {
		return companionFrames[0].art
	}
	phase := tick % len(frames)
	if phase < 0 {
		phase += len(frames)
	}
	return frames[phase]
}

// companionStageWidth is the row the creature walks along: the frame's inner
// width, which is what the rules above the footer and under the header span.
func companionStageWidth(m Model) int {
	return layoutFor(m).Inner
}

// companionStateNow is the state the creature is drawn in, read from the model
// and nothing else. The order of the cases is the reactions' precedence.
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
	case m.CompanionMoving:
		return companionWalkingState
	default:
		return companionIdleState
	}
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

// armCompanionFollow points the companion at the row the cursor is on. A key that
// moved the selection, or a key that opened another screen, is the event; the
// walking itself happens on the ticks that follow, which is why moving the cursor
// visibly moves the creature over the following ticks instead of teleporting it.
// A screen with nothing to point at leaves it strolling.
func (m *Model) armCompanionFollow() {
	if _, ok := m.companionTarget(companionStageWidth(*m)); ok {
		m.CompanionFollow = true
	}
}

// advanceCompanion is the creature's whole clock: it ages the idle stretch, ages
// a celebration and takes one step. It is called from the frame tick and from
// nowhere else, so the render path never moves anything and the same model and
// tick always draw the same bytes.
func (m *Model) advanceCompanion() {
	m.CompanionIdle++
	if m.CompanionPleased > 0 {
		m.CompanionPleased--
	}

	// A sleeping companion is still. The frame says asleep, and a creature that
	// kept strolling while asleep would contradict its own face; it also means an
	// idle run settles into a view string that no longer changes, so the renderer
	// stops repainting anything at all.
	if m.CompanionIdle >= companionSleepTicks {
		m.CompanionMoving = false
		return
	}

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

// companionRow is the companion's row: its cell placed along the stage, or the
// empty string when there is no companion to draw -- animation off, or a stage
// too narrow to hold the creature. It reads the model and draws and does nothing
// else: no clock, no I/O and no mutation, so a render stays pure and a snapshot
// can pin a frame.
func (m Model) companionRow(stage int) string {
	if !m.Animating || stage < companionCellWidth {
		return ""
	}
	pos := min(max(m.CompanionPos, 0), stage-companionCellWidth)
	return strings.Repeat(" ", pos) + CompanionStyle.Render(companionArtFor(m.companionStateNow(), m.AnimTick))
}

// trainerCompanionRow is the companion's row on a trainer screen. The trainer's
// exercise and boss screens spend every row they are given -- their code window
// takes whatever the chrome leaves -- so there is no spare row for the frame's
// placement to find. What they do have is one blank spacer row above the legend,
// which is the row nearest the footer and a row the body did not need; the
// companion takes that row where the frame's screens take the last spare one. It
// draws nothing when the gate is off, so the spacer stays blank and the screen
// renders exactly the bytes it did before the creature existed.
func (m Model) trainerCompanionRow(stage int) string {
	return m.companionRow(stage)
}

// placeCompanion puts the companion's row on the last row the body did not need,
// which is the row immediately above the footer rule, and returns the frame's rows
// unchanged when that row is not spare or the summary needs it. Nothing is dropped
// from a body to make room for a pet, and nothing is dropped from the summary
// either: the summary is the facts the panel was showing and the creature is
// decoration, so when the spare rows cannot hold both, the summary keeps its rows
// and the companion is not drawn. When both fit, the companion takes the row
// nearest the footer and the summary sits above it, so neither displaces the
// other.
func placeCompanion(placed []string, row string, summary []string) []string {
	if row == "" || len(placed) == 0 || placed[len(placed)-1] != "" ||
		companionSpareRows(placed) < len(summary)+1 {
		return placeRotator(placed, summary)
	}
	out := placeRotator(placed[:len(placed)-1], summary)
	return append(out, row)
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
