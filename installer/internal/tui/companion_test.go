package tui

import (
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/albersg/dotfiles/installer/internal/system"
	"github.com/albersg/dotfiles/installer/internal/tui/trainer"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/termenv"
)

// TestMain pins the animation gate off for the whole package. The gate reads
// stdout, so without this the screens a test renders would depend on whether the
// test binary was started from a terminal -- and a golden snapshot would then
// carry the companion on one host and not on another. Tests that are about the
// animation itself turn the gate on directly through the model's own field,
// which is what the field is for.
func TestMain(m *testing.M) {
	os.Setenv(envAnim, "0")
	os.Exit(m.Run())
}

// --- helpers ---------------------------------------------------------------

// companionTick drives one frame tick through Update, the way the program does,
// and returns the model it produced.
func companionTick(t *testing.T, m Model) Model {
	t.Helper()
	next, _ := m.Update(animTickMsg{})
	got, ok := next.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", next)
	}
	return got
}

// companionTicks drives n frame ticks.
func companionTicks(t *testing.T, m Model, n int) Model {
	t.Helper()
	for i := 0; i < n; i++ {
		m = companionTick(t, m)
	}
	return m
}

// companionKey drives one printable key through Update.
func companionKey(t *testing.T, m Model, key string) Model {
	t.Helper()
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	got, ok := next.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", next)
	}
	return got
}

// companionRowHasArt reports whether a rendered row carries one of the art's
// rows, stripped of styling: the art is what the eye reads, so the art is what
// a test looks for. It asks the composer, not only the tables: a frame at any
// height and any gaze is a row the sprite may be drawing.
func companionRowHasArt(row string) bool {
	plain := ansiEscape.ReplaceAllString(row, "")
	for _, art := range companionArtRowSet {
		if strings.Contains(plain, art) {
			return true
		}
	}
	return companionPixelRowShape(plain)
}

// companionPixelRowShape reports whether a stripped row is a row of the volumetric sprite:
// nothing but the half blocks it draws with and the spaces its empty cells leave, which is a
// shape no other row in the frame has. "Any block glyph" would not do -- the welcome
// screen's wordmark is full blocks and the panels draw their meters with shading glyphs --
// and the sprite's own rows are the only ones made exclusively of these, so the two cannot
// be confused.
func companionPixelRowShape(plain string) bool {
	if !strings.ContainsAny(plain, "\u2580\u2584\u2588") {
		return false
	}
	for _, r := range plain {
		if r != ' ' && r != '\u2580' && r != '\u2584' && r != '\u2588' {
			return false
		}
	}
	return true
}

// companionArtRowSet is every row the art can draw, at every height and every
// gaze, computed once.
var companionArtRowSet = companionEveryArtRow()

// companionEveryArtRow walks the tables and the composer to list every row the
// sprite can be drawn as.
func companionEveryArtRow() []string {
	var rows []string
	for _, height := range companionHeights() {
		for _, state := range companionStates() {
			for _, gaze := range []companionGaze{{}, {X: -1}, {X: 1}, {Y: -1}, {X: -1, Y: -1}, {X: 1, Y: -1}} {
				rows = append(rows, companionArtFor(state, height, 0, gaze)...)
			}
		}
	}
	return rows
}

// companionHeights is the ladder's three steps, in the order it tries them.
func companionHeights() []int {
	return []int{companionFullHeight, companionCompactHeight, companionMiniHeight}
}

// companionStates is every state the art draws, in the order they are declared.
func companionStates() []companionState {
	return []companionState{
		companionIdleState, companionWalkingState, companionBlinkingState,
		companionAsleepState, companionYawningState, companionAlertState,
		companionPleasedState, companionFlinchingState,
	}
}

// companionFrameDiff lists the cells two frames disagree on, as "row:column".
// The gaze tests use it to say exactly which characters a gaze moved, rather
// than trusting a whole-row comparison to notice a second change.
func companionFrameDiff(before, after []string) []string {
	var diff []string
	for i := 0; i < len(before) && i < len(after); i++ {
		beforeRunes, afterRunes := []rune(before[i]), []rune(after[i])
		for c := 0; c < len(beforeRunes) && c < len(afterRunes); c++ {
			if beforeRunes[c] != afterRunes[c] {
				diff = append(diff, fmt.Sprintf("%d:%d", i, c))
			}
		}
	}
	return diff
}

// findCompanionRow finds the companion's row in a rendered view. It is found
// rather than computed, so a test pins where the row actually landed instead of
// repeating the placement arithmetic it is checking.
func findCompanionRow(view string) (int, string, bool) {
	for i, row := range strings.Split(view, "\n") {
		if companionRowHasArt(row) {
			return i, row, true
		}
	}
	return -1, "", false
}

// companionRenderedRow is findCompanionRow for the tests that require a row: a
// screen with no companion there is a failure, not an alternative.
func companionRenderedRow(t *testing.T, view string) (int, string) {
	t.Helper()
	index, row, ok := findCompanionRow(view)
	if !ok {
		t.Fatalf("no companion row in:\n%s", view)
	}
	return index, row
}

// plainRow is a rendered row with its styling removed.
func plainRow(row string) string {
	return ansiEscape.ReplaceAllString(row, "")
}

// isRuleRow reports whether a rendered row is a frame rule and nothing else.
func isRuleRow(row string) bool {
	plain := strings.TrimSpace(plainRow(row))
	if plain == "" {
		return false
	}
	for _, r := range plain {
		if r != '─' {
			return false
		}
	}
	return true
}

// companionScreenFixture is one screen to render at one size. The framed flag
// says whether the screen draws inside the installer's frame, whose body ends
// with a rule above the footer; the trainer's exercise screens compose their own
// rows, so their companion row is the blank spacer above the legend instead.
type companionScreenFixture struct {
	name          string
	screen        Screen
	width, height int
	framed        bool
	// backups gives the model the backup the restore screens need: without one
	// they render their own dead end instead of the frame, which would test
	// nothing.
	backups bool
}

// --- the art ---------------------------------------------------------------

// TestCompanionArtIsRowsOfPrintableASCII pins the drawing rules the art was
// chosen for, at every height the ladder can pick. Every art row is printable
// ASCII, so Termux, a 16-colour terminal and a terminal whose font lacks the
// glyphs all draw the creature; every row of a frame is the same width and no
// row is wider than the cell it is placed in, so a one-cell step is always one
// column and the creature never jitters sideways as the eye reads it; the walk
// has two frames that differ, because a walk with one frame is a slide; and every
// state draws at every height.
//
// The two taller sets are exactly the cell's width on every row, so their
// sprites are rectangles. The one-row art is left as it shipped -- its face is
// five columns and padding it to the seven-column cell would change bytes a
// snapshot already pins -- so for it the guard is "no wider than the cell".
func TestCompanionArtIsRowsOfPrintableASCII(t *testing.T) {
	// The walk bounds itself on the widest cell, which is the full volumetric sprite's: no
	// height may draw wider than that, and every glyph height is narrower, so a sprite can
	// never walk past the edge of the stage it is drawn on.
	if companionCellWidth != companionVolumeFullWidth {
		t.Errorf("the walk bounds itself on %d columns but the widest cell is %d",
			companionCellWidth, companionVolumeFullWidth)
	}
	for _, height := range companionHeights() {
		if cell := companionSpriteWidth(height); cell > companionCellWidth {
			t.Errorf("height %d draws a %d-column cell, wider than the %d the walk allows",
				height, cell, companionCellWidth)
		}
	}
	for _, height := range companionHeights() {
		states := map[companionState][]string{}
		cell := companionSpriteWidth(height)

		for _, frame := range companionFramesAt(height) {
			if len(frame.rows) != height {
				t.Errorf("height %d draws state %s in %d rows, want %d",
					height, companionStateNames[frame.state], len(frame.rows), height)
			}
			rowWidth := -1
			for _, row := range frame.rows {
				if row == "" {
					t.Errorf("height %d draws state %s with an empty row", height, companionStateNames[frame.state])
					continue
				}
				for _, r := range row {
					if r > 0x7E || r < 0x20 {
						t.Errorf("row %q of state %s carries %q, which is not printable ASCII",
							row, companionStateNames[frame.state], r)
					}
				}
				if strings.ContainsAny(row, "\n\t") {
					t.Errorf("row %q of state %s is not one row", row, companionStateNames[frame.state])
				}
				if rowWidth < 0 {
					rowWidth = len(row)
				}
				if len(row) != rowWidth {
					t.Errorf("state %s at height %d has rows of %d and %d columns, want one width",
						companionStateNames[frame.state], height, rowWidth, len(row))
				}
			}
			if rowWidth > cell {
				t.Errorf("a frame at height %d is %d columns wide, want at most the cell's %d",
					height, rowWidth, cell)
			}
			if height > companionMiniHeight && rowWidth != cell {
				t.Errorf("a frame at height %d is %d columns, want the whole cell's %d",
					height, rowWidth, cell)
			}
			states[frame.state] = append(states[frame.state], strings.Join(frame.rows, "\n"))
		}

		for _, state := range companionStates() {
			if len(states[state]) == 0 {
				t.Errorf("state %s draws nothing at height %d", companionStateNames[state], height)
			}
			if len(states[state]) == 0 {
				continue
			}
			want := states[state][0]
			if got := strings.Join(companionArtFor(state, height, 0, companionGaze{}), "\n"); got != want {
				t.Errorf("state %s at height %d draws\n%s\nat tick 0, want\n%s",
					companionStateNames[state], height, got, want)
			}
		}

		walk := states[companionWalkingState]
		if len(walk) < 2 {
			t.Fatalf("the walk at height %d has %d frames, want at least 2 to alternate", height, len(walk))
		}
		// The walk cycles through its frames on the tick's parity, so the art
		// changes on every tick it walks and returns to the first frame after a full
		// cycle.
		for tick := 0; tick < 2*len(walk); tick++ {
			got := strings.Join(companionArtFor(companionWalkingState, height, tick, companionGaze{}), "\n")
			if got != walk[tick%len(walk)] {
				t.Errorf("the walk at height %d draws\n%s\nat tick %d, want\n%s", height, got, tick, walk[tick%len(walk)])
			}
		}
	}
}

// --- the gaze ---------------------------------------------------------------

// companionEyesFor is the idle frame's eye description at one height.
func companionEyesFor(t *testing.T, height int) companionEyes {
	t.Helper()
	for _, frame := range companionFramesAt(height) {
		if frame.state == companionIdleState {
			return frame.eyes
		}
	}
	t.Fatalf("height %d draws no idle frame", height)
	return companionEyes{}
}

// TestCompanionGazeMovesPupilsAndNothingElse pins the composer's contract. The
// gaze rewrites the eye row and touches nothing else: every other row of the
// frame is byte-identical, the eye row's only marks are still the two pupils, and
// their glyphs are the ones the table draws. The "up" gaze swaps the two
// interior rows rather than drawing a third eye, and the compact art -- whose
// head has room for one eye row -- ignores it entirely.
func TestCompanionGazeMovesPupilsAndNothingElse(t *testing.T) {
	for _, height := range []int{companionCompactHeight, companionFullHeight} {
		height := height
		t.Run(fmt.Sprintf("height %d", height), func(t *testing.T) {
			eyes := companionEyesFor(t, height)
			neutral := companionArtFor(companionIdleState, height, 0, companionGaze{})
			base, ok := companionEyesOf(neutral[eyes.row])
			if !ok {
				t.Fatalf("the idle frame at height %d has no eye row to move", height)
			}

			for _, gazeX := range []int{-1, 0, 1} {
				got := companionArtFor(companionIdleState, height, 0, companionGaze{X: gazeX})
				for row := range neutral {
					if row == eyes.row || row == eyes.upper {
						continue
					}
					if neutral[row] != got[row] {
						t.Errorf("gaze %d changed row %d, which is not an eye row", gazeX, row)
					}
				}
				moved, ok := companionEyesOf(got[eyes.row])
				if !ok {
					t.Fatalf("gaze %d left the eye row without its two pupils: %q", gazeX, got[eyes.row])
				}
				if moved.left != base.left || moved.right != base.right {
					t.Errorf("gaze %d changed the pupils' glyphs from %q %q to %q %q",
						gazeX, base.left, base.right, moved.left, moved.right)
				}
				if moved.leftCol != base.leftCol+gazeX || moved.rightCol != base.rightCol+gazeX {
					t.Errorf("gaze %d moved the pupils to columns %d and %d, want %d and %d",
						gazeX, moved.leftCol, moved.rightCol, base.leftCol+gazeX, base.rightCol+gazeX)
				}
			}

			up := companionArtFor(companionIdleState, height, 0, companionGaze{Y: -1})
			if eyes.upper < 0 {
				if strings.Join(up, "\n") != strings.Join(neutral, "\n") {
					t.Errorf("height %d has one eye row but looked up anyway:\n%s", height, strings.Join(up, "\n"))
				}
				return
			}
			if up[eyes.upper] != neutral[eyes.row] || up[eyes.row] != neutral[eyes.upper] {
				t.Errorf("the up gaze did not swap the two eye rows:\n%s", strings.Join(up, "\n"))
			}
		})
	}
}

// TestCompanionGazeForTurnsACellAndARowIntoAGaze pins the seam the pointer will
// feed: inside the dead zone it looks straight ahead, beyond it it looks the way
// the target is, and a target on a row above the creature's own makes it look up.
// Down is never returned: the art has no honest third eye row.
func TestCompanionGazeForTurnsACellAndARowIntoAGaze(t *testing.T) {
	const anchor = 40

	tests := []struct {
		name     string
		col, row int
		want     companionGaze
	}{
		{"the cell it stands on is straight ahead", anchor, 0, companionGaze{}},
		{"inside the dead zone is straight ahead", anchor + companionGazeDeadZone, 0, companionGaze{}},
		{"past the dead zone is to the right", anchor + companionGazeDeadZone + 1, 0, companionGaze{X: 1}},
		{"before the dead zone is to the left", anchor - companionGazeDeadZone - 1, 0, companionGaze{X: -1}},
		{"a row above the creature is up", anchor, -1, companionGaze{Y: -1}},
		{"a row below is down", anchor, 1, companionGaze{Y: 1}},
		{"up and right combine", anchor + companionGazeDeadZone + 1, -1, companionGaze{X: 1, Y: -1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := companionGazeFor(tt.col, tt.row, anchor); got != tt.want {
				t.Errorf("companionGazeFor(%d, %d, %d) = %+v, want %+v", tt.col, tt.row, anchor, got, tt.want)
			}
		})
	}
}

// TestCompanionLooksAtTheSelectionItWalksToward pins the gaze the model derives
// today, before the pointer exists: it looks at the row the cursor is on -- a body
// row above its own -- so a menu screen makes it look up and, while the selection
// is off to one side, that way too. When it arrives under the thing it was looking
// at, or when the screen has nothing to point at, it looks straight ahead.
func TestCompanionLooksAtTheSelectionItWalksToward(t *testing.T) {
	build := func(t *testing.T) Model {
		t.Helper()
		m := NewModel()
		isolateGoldenTest(t, &m)
		m.Screen = ScreenMainMenu
		m.Width, m.Height = 160, 50
		m.Animating = true
		return m
	}

	m := build(t)
	options := m.GetCurrentOptions()
	if len(options) < 3 {
		t.Fatalf("the main menu offers %d options, want enough to look between", len(options))
	}

	// The first row is the left edge, which is under the creature's own cell: it
	// looks at it straight on, and up, because a menu row is above the creature.
	m.Cursor = 0
	m.CompanionPos = 0
	m.armCompanionFollow()
	if m.CompanionGaze != (companionGazeFor(0, companionSelectionRow, 0)) {
		t.Errorf("the first row points the gaze at %+v", m.CompanionGaze)
	}
	if m.CompanionGaze.Y != -1 {
		t.Errorf("the creature does not look up at the row the cursor is on: %+v", m.CompanionGaze)
	}

	// The last row is the right edge, far to the right of a creature at the left.
	m.Cursor = len(options) - 1
	m.CompanionPos = 0
	m.armCompanionFollow()
	if !m.CompanionFollow {
		t.Fatalf("moving the cursor did not point the companion at the new row")
	}
	if m.CompanionGaze.X != 1 {
		t.Errorf("a selection at the right edge leaves the gaze at %+v, want it to the right", m.CompanionGaze)
	}
	target, ok := m.companionTarget(companionStageWidth(m))
	if !ok {
		t.Fatalf("the last option has no target cell")
	}

	// Walking to it takes the gaze with it: once the creature stands on the cell it
	// was looking at, the pupils are straight ahead again.
	for i := 0; i < 40 && m.CompanionFollow; i++ {
		m = companionTick(t, m)
	}
	if m.CompanionFollow {
		t.Fatalf("the companion never reached cell %d", target)
	}
	if m.CompanionPos != target {
		t.Fatalf("the companion stands at cell %d, want %d", m.CompanionPos, target)
	}
	if m.CompanionGaze.X != 0 {
		t.Errorf("the creature arrived under its target and still looks aside: %+v", m.CompanionGaze)
	}

	// A screen with nothing to point at looks straight ahead and never arms a walk.
	quiet := build(t)
	quiet.Screen = ScreenInstalling
	quiet.Cursor = 4
	quiet.CompanionPos = 30
	quiet.armCompanionFollow()
	if quiet.CompanionFollow {
		t.Errorf("a screen with no menu armed the walk")
	}
	if quiet.CompanionGaze != (companionGaze{}) {
		t.Errorf("a screen with no menu points the gaze at %+v, want it straight ahead", quiet.CompanionGaze)
	}
}

// TestCompanionHeightFollowsTheLadder pins the fallback: the tallest sprite the
// spare rows can hold, in the order five, three, one, and nothing at all when even
// one row is not left. The rows handed to it are the body's spare rows after the
// panel summary has taken its own -- which is how the summary always keeps its
// rows.
func TestCompanionHeightFollowsTheLadder(t *testing.T) {
	tests := []struct {
		spare, want int
	}{
		{-5, 0},
		{0, 0},
		{1, companionMiniHeight},
		{2, companionMiniHeight},
		{3, companionCompactHeight},
		{4, companionCompactHeight},
		{5, companionFullHeight},
		{6, companionFullHeight},
		{40, companionFullHeight},
	}
	for _, tt := range tests {
		if got := companionHeight(tt.spare); got != tt.want {
			t.Errorf("companionHeight(%d) = %d, want %d", tt.spare, got, tt.want)
		}
	}

	// The three steps are ordered tallest first and every one of them has art and a
	// cell, so the ladder cannot pick a height nothing can be drawn at.
	previous := 0
	for _, height := range companionHeights() {
		if previous > 0 && height >= previous {
			t.Errorf("the ladder's steps are not tallest-first: %d after %d", height, previous)
		}
		previous = height
		if companionSpriteWidth(height) == 0 {
			t.Errorf("height %d has no cell", height)
		}
		if len(companionFramesAt(height)) == 0 {
			t.Errorf("height %d has no art", height)
		}
	}
}

// --- the state the model is in ---------------------------------------------

// TestCompanionStateFollowsTheModelAndItsPrecedence pins the whole state machine
// in one table: what the model holds, and the frame that follows from it. The
// order of the rows is the precedence the design states -- a reaction wins over
// the resting states, so a sleeping companion that must flinch flinches.
func TestCompanionStateFollowsTheModelAndItsPrecedence(t *testing.T) {
	sleeping := companionSleepTicks + 5
	errorMsg := "Step 'clone' failed"

	tests := []struct {
		name  string
		model Model
		want  companionState
	}{
		{
			name:  "awake and standing still is idle",
			model: Model{Animating: true, CompanionIdle: 3},
			want:  companionIdleState,
		},
		{
			name:  "a tick that moved it is walking",
			model: Model{Animating: true, CompanionIdle: 3, CompanionMoving: true},
			want:  companionWalkingState,
		},
		{
			name:  "twenty quiet seconds put it to sleep",
			model: Model{Animating: true, CompanionIdle: companionSleepTicks},
			want:  companionAsleepState,
		},
		{
			name:  "a sleeping companion that must flinch flinches",
			model: Model{Animating: true, CompanionIdle: sleeping, ErrorMsg: errorMsg},
			want:  companionFlinchingState,
		},
		{
			name:  "the error screen flinches",
			model: Model{Animating: true, CompanionIdle: sleeping, Screen: ScreenError},
			want:  companionFlinchingState,
		},
		{
			name:  "a restore screen is alert even while it sleeps",
			model: Model{Animating: true, CompanionIdle: sleeping, Screen: ScreenRestoreConfirm},
			want:  companionAlertState,
		},
		{
			name:  "the screen that overwrites configs is alert",
			model: Model{Animating: true, CompanionIdle: 2, Screen: ScreenBackupConfirm},
			want:  companionAlertState,
		},
		{
			name:  "a finished step is pleased while it sleeps",
			model: Model{Animating: true, CompanionIdle: sleeping, CompanionPleased: 2},
			want:  companionPleasedState,
		},
		{
			name:  "flinch wins over alert",
			model: Model{Animating: true, Screen: ScreenRestoreConfirm, ErrorMsg: errorMsg},
			want:  companionFlinchingState,
		},
		{
			name:  "alert wins over pleased",
			model: Model{Animating: true, Screen: ScreenRestoreConfirm, CompanionPleased: 3},
			want:  companionAlertState,
		},
		{
			name:  "a right answer on the result screen is pleased",
			model: Model{Animating: true, Screen: ScreenTrainerResult, TrainerLastCorrect: true},
			want:  companionPleasedState,
		},
		{
			name:  "a wrong answer on the result screen is a flinch",
			model: Model{Animating: true, Screen: ScreenTrainerResult, TrainerLastCorrect: false},
			want:  companionFlinchingState,
		},
		{
			name:  "a boss defeat is a flinch too",
			model: Model{Animating: true, Screen: ScreenTrainerBossResult, TrainerLastCorrect: false},
			want:  companionFlinchingState,
		},
		{
			name:  "an exercise screen with no answer yet does not flinch",
			model: Model{Animating: true, Screen: ScreenTrainerLesson},
			want:  companionIdleState,
		},
		{
			name:  "the last quiet stretch before sleep is a yawn",
			model: Model{Animating: true, CompanionIdle: companionSleepTicks - 1},
			want:  companionYawningState,
		},
		{
			name:  "an awake creature blinks on the blink tick",
			model: Model{Animating: true, AnimTick: companionBlinkTicks - 1},
			want:  companionBlinkingState,
		},
		{
			name:  "a blink never interrupts the walk",
			model: Model{Animating: true, AnimTick: companionBlinkTicks - 1, CompanionMoving: true},
			want:  companionWalkingState,
		},
		{
			name:  "a yawn never interrupts a flinch",
			model: Model{Animating: true, CompanionIdle: companionSleepTicks - 1, ErrorMsg: errorMsg},
			want:  companionFlinchingState,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.model.companionStateNow(); got != tt.want {
				t.Errorf("state = %s, want %s",
					companionStateNames[got], companionStateNames[tt.want])
			}
			// The state has to reach the pixels: the rendered row carries the art
			// of the state the model is in, and no colour is needed to read it.
			row := tt.model.companionRow(80)
			if row == "" {
				t.Fatalf("the model draws no companion row at all")
			}
			want := companionArtFor(tt.want, companionMiniHeight, tt.model.AnimTick, tt.model.CompanionGaze)
			if got := strings.TrimLeft(plainRow(row), " "); got != want[0] {
				t.Errorf("the row draws %q, want %q", got, want[0])
			}
		})
	}
}

// TestCompanionStatesAreDistinctWithoutColour pins that nothing about the state
// depends on the tone: at every height, the glyph run of each state's frame is
// different from every other state's -- the walk's two frames included -- so a
// terminal with no colour loses no information.
func TestCompanionStatesAreDistinctWithoutColour(t *testing.T) {
	for _, height := range companionHeights() {
		plain := map[string]companionState{}
		for _, frame := range companionFramesAt(height) {
			// Two states may share a frame only if they are the same state: the walk's
			// two frames are the exception, and they differ from each other.
			art := strings.Join(frame.rows, "\n")
			if other, ok := plain[art]; ok && other != frame.state {
				t.Errorf("at height %d states %s and %s draw the same frame:\n%s",
					height, companionStateNames[other], companionStateNames[frame.state], art)
			}
			plain[art] = frame.state
		}
	}

	// The one-row face still carries five different glyph runs: the eyes, the z
	// and the ! are the state.
	distinct := map[string]companionState{
		"(o.o)":    companionIdleState,
		"(-.-) z":  companionAsleepState,
		"(O.O) !":  companionAlertState,
		"\\(o.o)/": companionPleasedState,
		"(>.<)":    companionFlinchingState,
	}
	for art, state := range distinct {
		got := companionArtFor(state, companionMiniHeight, 0, companionGaze{})
		if got[0] != art {
			t.Errorf("state %s draws %q, want %q", companionStateNames[state], got[0], art)
		}
	}
}

// TestCompanionStatesAreNamed pins that every state the machine can return has a
// name: a state added without one prints as a number in a failure message, which
// is exactly when the name is needed.
func TestCompanionStatesAreNamed(t *testing.T) {
	for _, state := range companionStates() {
		if companionStateNames[state] == "" {
			t.Errorf("state %d has no name", state)
		}
	}
}

// --- reactions -------------------------------------------------------------

// TestCompanionReactsToAFinishedStepAndToAFailedOne pins the two beats the
// installer itself produces: a step that completes is worth a few ticks of
// celebration and then subsides, and a step that fails flinches instead -- and
// never celebrates, which is the difference between the two return paths in the
// step message handler.
func TestCompanionReactsToAFinishedStepAndToAFailedOne(t *testing.T) {
	step := InstallStep{ID: "clone", Name: "Clone the repository"}

	finished := Model{
		Animating: true,
		Screen:    ScreenInstalling,
		Steps:     []InstallStep{step},
	}
	next, _ := finished.Update(stepCompleteMsg{stepID: step.ID})
	done, ok := next.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", next)
	}
	if done.Screen == ScreenError {
		t.Fatalf("a step with no error went to the error screen")
	}
	if done.CompanionPleased != companionPleasedTicks {
		t.Errorf("a finished step set CompanionPleased to %d, want %d",
			done.CompanionPleased, companionPleasedTicks)
	}
	if state := done.companionStateNow(); state != companionPleasedState {
		t.Errorf("State = %d after a finished step, want the pleased one", state)
	}

	// The celebration is a few ticks, not a mode: it settles by itself, and what it
	// settles to is the resting state and not motion. This screen has no menu, so the
	// creature has nothing to walk toward, and the ticks after the celebration leave
	// it standing exactly where it is.
	settled := companionTicks(t, done, companionPleasedTicks)
	if settled.CompanionPleased != 0 {
		t.Errorf("CompanionPleased = %d after %d ticks, want 0",
			settled.CompanionPleased, companionPleasedTicks)
	}
	if state := settled.companionStateNow(); state != companionIdleState {
		t.Errorf("State = %d after the celebration settles, want the idle one", state)
	}
	if settled.CompanionMoving {
		t.Errorf("the companion walked on its own after its celebration")
	}

	failed := Model{
		Animating: true,
		Screen:    ScreenInstalling,
		Steps:     []InstallStep{step},
	}
	next, _ = failed.Update(stepCompleteMsg{stepID: step.ID, err: errors.New("exit status 1")})
	broken, ok := next.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", next)
	}
	if broken.Screen != ScreenError {
		t.Fatalf("a failed step left the screen on %v, want the error screen", broken.Screen)
	}
	if broken.CompanionPleased != 0 {
		t.Errorf("a failed step celebrated: CompanionPleased = %d", broken.CompanionPleased)
	}
	if state := broken.companionStateNow(); state != companionFlinchingState {
		t.Errorf("State = %d after a failed step, want the flinching one", state)
	}
}

// TestCompanionSleepsAfterAQuietStretchAndWakesOnAKey pins the idle clock: the
// stretch is counted on the model, twenty seconds without a key put the creature
// to sleep, and the first key brings it back. It also pins what sleeping buys: two
// consecutive ticks with a sleeping companion render the same bytes, so the
// renderer has nothing to repaint while nothing is happening.
func TestCompanionSleepsAfterAQuietStretchAndWakesOnAKey(t *testing.T) {
	m := Model{Animating: true, Screen: ScreenComplete, Width: 100, Height: 24}

	awake := companionTicks(t, m, companionSleepTicks-1)
	if state := awake.companionStateNow(); state == companionAsleepState {
		t.Fatalf("the companion slept after %d ticks, want %d", companionSleepTicks-1, companionSleepTicks)
	}

	asleep := companionTick(t, awake)
	if state := asleep.companionStateNow(); state != companionAsleepState {
		t.Fatalf("the companion did not sleep after %d ticks", companionSleepTicks)
	}
	if settled := companionTick(t, asleep); settled.View() != asleep.View() {
		t.Errorf("a tick with a sleeping companion changed the view:\nbefore:\n%s\nafter:\n%s",
			asleep.View(), settled.View())
	}
	if asleep.CompanionMoving {
		t.Errorf("a sleeping companion is still walking")
	}

	woken := companionKey(t, asleep, "x")
	if woken.CompanionIdle != 0 {
		t.Errorf("CompanionIdle = %d after a key, want 0", woken.CompanionIdle)
	}
	if state := woken.companionStateNow(); state == companionAsleepState {
		t.Errorf("the companion is still asleep after a key")
	}
	// Waking restores the resting state, not motion: this screen has no menu, so the
	// tick after a key leaves the creature standing where it woke and changes nothing.
	// The walk is the wake's other half, and it is pinned on a menu screen by
	// TestCompanionWalksTowardWhatTheCursorPointsAt.
	walked := companionTick(t, woken)
	if walked.CompanionMoving {
		t.Errorf("the companion walked on its own after being woken")
	}
	if walked.View() != woken.View() {
		t.Errorf("the tick after a key changed a screen with nothing to walk toward:\nbefore:\n%s\nafter:\n%s",
			woken.View(), walked.View())
	}
}

// --- the gate --------------------------------------------------------------

// TestCompanionDrawsNothingWhenAnimationIsOff pins the gate's reach: with
// animation off there is no companion anywhere and the ticks do not move one,
// because the counter they drive stays at zero. A frozen pet is not the point.
func TestCompanionDrawsNothingWhenAnimationIsOff(t *testing.T) {
	m := Model{Animating: false, Screen: ScreenComplete, Width: 100, Height: 24}

	first := m.View()
	for tick := 1; tick <= 5; tick++ {
		m = companionTick(t, m)
		if m.AnimTick != 0 {
			t.Fatalf("AnimTick = %d with animation off, want 0", m.AnimTick)
		}
		if got := m.View(); got != first {
			t.Errorf("tick %d changed the view with animation off:\n%s", tick, got)
		}
	}
	if _, row, ok := findCompanionRow(first); ok {
		t.Errorf("an animation-off screen drew a companion: %q", plainRow(row))
	}
	if got := m.companionRow(80); got != "" {
		t.Errorf("companionRow = %q with animation off, want the empty string", got)
	}
}

// --- cost -----------------------------------------------------------------

// TestCompanionTicksChangeOnlyItsOwnRows is the cost bound, and it is two-sided.
// At rest the creature has nothing to do, so a tick changes no row at all -- the
// stronger half, and the one the reported flicker came from. While it is armed and
// walking the sprite moves one column, so every row it owns changes and no other
// row may: the frame is not re-laid out, no body row is touched and the row count
// does not move. Both halves are asserted over several screens and terminal sizes,
// on ticks that are not a tip boundary, because a tip rotating is the one other
// thing the slow clock shows.
func TestCompanionTicksChangeOnlyItsOwnRows(t *testing.T) {
	fixtures := []companionScreenFixture{
		{name: "main menu 80x24", screen: ScreenMainMenu, width: 80, height: 24, framed: true},
		{name: "main menu 100x24", screen: ScreenMainMenu, width: 100, height: 24, framed: true},
		{name: "main menu 160x50", screen: ScreenMainMenu, width: 160, height: 50, framed: true},
		{name: "main menu 227x62", screen: ScreenMainMenu, width: 227, height: 62, framed: true},
		{name: "welcome 160x50", screen: ScreenWelcome, width: 160, height: 50, framed: true},
		{name: "os select 80x24", screen: ScreenOSSelect, width: 80, height: 24, framed: true},
		{name: "restore confirm 100x30", screen: ScreenRestoreConfirm, width: 100, height: 30, framed: true, backups: true},
		{name: "error 100x30", screen: ScreenError, width: 100, height: 30, framed: true},
		{name: "installing 160x50", screen: ScreenInstalling, width: 160, height: 50, framed: true},
		{name: "keymaps 160x50", screen: ScreenKeymaps, width: 160, height: 50, framed: true},
	}

	// A tick pair inside one tip: AnimTick 3 to 4 keeps the tip panel on tip 0.
	const firstTick = 3

	for _, f := range fixtures {
		f := f
		t.Run(f.name, func(t *testing.T) {
			build := func() Model {
				m := NewModel()
				isolateGoldenTest(t, &m)
				m.Screen = f.screen
				m.Width, m.Height = f.width, f.height
				m.Animating, m.AnimTick = true, firstTick
				if f.backups {
					m.AvailableBackups = []system.BackupInfo{testBackupInfo()}
					m.SelectedBackup = 0
				}
				return m
			}

			// At rest a tick changes nothing at all: no body row, no sprite row, no
			// byte. The creature is not strolling and its gaze is not settling a frame
			// late, so the view is the same string a tick later.
			atRest := build()
			before := atRest.View()
			after := companionTick(t, atRest).View()
			if before != after {
				t.Fatalf("a tick with nothing to do changed the view:\nbefore:\n%s\nafter:\n%s", before, after)
			}

			// Walking is the other half: the sprite moves one column, so every row of it
			// changes and no other row may. The walk is armed by hand here because a key
			// that moved the cursor is the only thing that arms it, and that path is
			// pinned by TestCompanionWalksTowardWhatTheCursorPointsAt.
			walking := build()
			options := walking.GetCurrentOptions()
			if len(options) < 2 {
				return
			}
			walking.Cursor = len(options) - 1
			walking.armCompanionFollow()

			before = walking.View()
			ticked := companionTick(t, walking)
			after = ticked.View()
			beforeRows := strings.Split(before, "\n")
			afterRows := strings.Split(after, "\n")
			if len(beforeRows) != len(afterRows) {
				t.Fatalf("a walking tick changed the row count from %d to %d",
					len(beforeRows), len(afterRows))
			}

			var changed []int
			for i := range beforeRows {
				if beforeRows[i] != afterRows[i] {
					changed = append(changed, i)
				}
			}
			owned := companionOwnedRows(afterRows)
			if !ticked.CompanionMoving {
				t.Fatalf("the armed walk did not move the companion on its tick")
			}
			if len(owned) == 0 {
				t.Fatalf("the sprite owns no row at %dx%d, so the walking bound would be vacuous", f.width, f.height)
			}
			if len(changed) != len(owned) {
				t.Fatalf("a walking tick changed %d rows, want the sprite's %d:\nbefore:\n%s\nafter:\n%s",
					len(changed), len(owned), before, after)
			}
			for i, row := range changed {
				if row != owned[i] {
					t.Fatalf("a walking tick changed row %d, which the sprite does not own: %q", row, plainRow(afterRows[row]))
				}
			}
			for _, row := range owned {
				if !companionRowHasArt(afterRows[row]) {
					t.Errorf("the sprite owns row %d but it carries no art: %q", row, plainRow(afterRows[row]))
				}
			}
			if len(owned) > companionFullHeight {
				t.Errorf("the sprite owns %d rows, want at most the tallest cell's %d", len(owned), companionFullHeight)
			}
			if f.framed {
				next := owned[len(owned)-1] + 1
				switch {
				case next >= len(afterRows):
					t.Errorf("the sprite's lowest row is the frame's last row, so no footer rule follows it")
				case !isRuleRow(afterRows[next]):
					t.Errorf("the sprite is not the block immediately above the footer rule; the row under it is %q",
						plainRow(afterRows[next]))
				}
			}
		})
	}

	t.Run("a screen with no spare row changes nothing at all", func(t *testing.T) {
		// The backup screen sizes its config list to the rows the frame leaves, so
		// its body fills the frame and there is no row the body did not need: the
		// tick may run and the view is the same string, which is what makes the
		// renderer skip the frame entirely while nothing on screen can change.
		m := NewModel()
		isolateGoldenTest(t, &m)
		m.Screen = ScreenBackupConfirm
		m.Width, m.Height = 80, 24
		m.Animating, m.AnimTick = true, firstTick
		for i := 0; i < 40; i++ {
			m.ExistingConfigs = append(m.ExistingConfigs, "~/.config/example/one")
		}

		before := m.View()
		after := companionTick(t, m).View()
		if before != after {
			t.Errorf("a tick changed a screen that has no spare row:\nbefore:\n%s\nafter:\n%s",
				before, after)
		}
		if _, row, ok := findCompanionRow(before); ok {
			t.Errorf("a full-body screen drew a companion: %q", plainRow(row))
		}
	})
}

// companionOwnedRows is the rows of a rendered view that carry the sprite, in
// order. The cost test compares a tick's changed rows against these, so it asks
// "did anything but the sprite move?" of the bytes rather than of a count that
// assumes a height.
func companionOwnedRows(rows []string) []int {
	var owned []int
	for i, row := range rows {
		if companionRowHasArt(row) {
			owned = append(owned, i)
		}
	}
	return owned
}

// companionChangedRows lists the rows two rendered views disagree on and the bytes
// those rows hold. The bytes are the whole line, not the difference between the
// lines: the renderer writes a whole line when any byte in it changed, so that is
// what a changed row costs the terminal.
func companionChangedRows(before, after string) (rows []int, bytes int) {
	beforeRows, afterRows := strings.Split(before, "\n"), strings.Split(after, "\n")
	for i := 0; i < len(beforeRows) && i < len(afterRows); i++ {
		if beforeRows[i] != afterRows[i] {
			rows = append(rows, i)
			bytes += len(afterRows[i])
		}
	}
	return rows, bytes
}

// companionArtRun is how many consecutive rows immediately above a rule carry art:
// the sprite's height as it was actually drawn on that screen and size, read off
// the render so a guard can bound the creature by the art the ladder chose rather
// than by a number that hard-codes one of its steps.
func companionArtRun(rows []string, rule int) int {
	run := 0
	for i := rule - 1; i >= 0 && companionRowHasArt(rows[i]); i-- {
		run++
	}
	return run
}

// --- placement ------------------------------------------------------------

// TestCompanionTakesTheSpareRowAboveTheFooterRule pins where the creature lives:
// the last row the body did not need, immediately above the rule that opens the
// footer, spanning the frame. A screen whose body fills its frame keeps every
// row it draws and shows no companion at all, and a narrow screen that shows a
// panel summary keeps the summary -- above the companion, which takes the row
// nearest the footer.
func TestCompanionTakesTheSpareRowAboveTheFooterRule(t *testing.T) {
	t.Run("a short body leaves the companion its own row", func(t *testing.T) {
		m := NewModel()
		isolateGoldenTest(t, &m)
		m.Screen = ScreenMainMenu
		m.Width, m.Height = 160, 50
		m.Animating = true

		view := m.View()
		rows := strings.Split(view, "\n")
		_, row := companionRenderedRow(t, view)
		// The sprite is the last spare block, so its lowest row -- not its first --
		// is the one immediately above the footer rule.
		owned := companionOwnedRows(rows)
		if len(owned) == 0 {
			t.Fatalf("the wide main menu drew no companion")
		}
		lowest := owned[len(owned)-1]
		if lowest+1 >= len(rows) || !isRuleRow(rows[lowest+1]) {
			t.Fatalf("the companion's lowest row is not immediately above the footer rule")
		}
		// Where the body leaves the whole five rows the ladder's first step wants,
		// the frame draws the cat rather than the one-row creature it used to.
		if len(owned) != companionFullHeight {
			t.Errorf("the sprite owns %d rows at 160x50, want the full set's %d", len(owned), companionFullHeight)
		}
		if !strings.Contains(plainRow(row), companionFullEars) {
			t.Errorf("the frame's first companion row is not the ears of the idle cat: %q", plainRow(row))
		}
	})

	t.Run("the 80x24 floor keeps the compact sprite", func(t *testing.T) {
		// The floor terminal is the size every screen is guaranteed to work at, and
		// there the body leaves five spare rows of which the summary takes one. The
		// full cat needs six, so the ladder steps down to the three-row head: this
		// is the screen that used to draw one row and now draws three.
		m := NewModel()
		isolateGoldenTest(t, &m)
		m.Screen = ScreenMainMenu
		m.Width, m.Height = 80, 24
		m.Animating = true

		view := m.View()
		rows := strings.Split(view, "\n")
		owned := companionOwnedRows(rows)
		if len(owned) != companionCompactHeight {
			t.Fatalf("the sprite owns %d rows at 80x24, want the compact set's %d", len(owned), companionCompactHeight)
		}
		for _, i := range owned {
			if !companionRowHasArt(rows[i]) {
				t.Errorf("row %d of the compact sprite carries no art: %q", i, plainRow(rows[i]))
			}
		}
		// The sprite replaces blank rows rather than adding any: the frame still fills
		// the terminal exactly, so its last row is still the legend.
		if rendered := renderedRowCount(view); rendered != m.Height {
			t.Errorf("the frame renders %d rows with the compact sprite, want the terminal's %d", rendered, m.Height)
		}
	})

	t.Run("a full body keeps every row it draws", func(t *testing.T) {
		// The backup screen sizes its config list to the rows the frame leaves, so
		// its body fills the frame exactly and there is nothing spare.
		m := NewModel()
		isolateGoldenTest(t, &m)
		m.Screen = ScreenBackupConfirm
		m.Width, m.Height = 80, 24
		m.Animating = true
		for i := 0; i < 40; i++ {
			m.ExistingConfigs = append(m.ExistingConfigs, "~/.config/example/"+strings.Repeat("x", 3))
		}

		withCompanion := m.View()
		m.Animating = false
		without := m.View()

		if withCompanion != without {
			t.Errorf("the companion displaced a body row:\nwith:\n%s\nwithout:\n%s",
				withCompanion, without)
		}
		if _, row, ok := findCompanionRow(withCompanion); ok {
			t.Errorf("a full-body screen drew a companion: %q", plainRow(row))
		}
	})

	t.Run("a narrow screen keeps its summary above the companion", func(t *testing.T) {
		m := NewModel()
		isolateGoldenTest(t, &m)
		m.Screen = ScreenMainMenu
		m.Width, m.Height = 100, 24
		m.Animating = true

		view := m.View()
		rows := strings.Split(view, "\n")
		companion, row := companionRenderedRow(t, view)

		summary := -1
		for i, line := range rows[:companion] {
			plain := plainRow(line)
			if strings.HasPrefix(strings.TrimLeft(plain, " "), "Plan") ||
				strings.HasPrefix(strings.TrimLeft(plain, " "), "What will happen") {
				summary = i
			}
		}
		if summary < 0 {
			t.Fatalf("the narrow screen lost its panel summary:\n%s", view)
		}
		if summary >= companion {
			t.Errorf("the summary at row %d is not above the companion at row %d", summary, companion)
		}
		if !companionRowHasArt(row) {
			t.Errorf("the companion row carries no art: %q", plainRow(row))
		}
	})

	t.Run("the facts keep the last spare row", func(t *testing.T) {
		// A narrow screen shows the panel's summary above the creature, and the
		// creature is the one that gives way: the summary is the fact the panel was
		// answering with and the creature is decoration. What "gives way" means
		// changed with the constant-space contract - the creature used to shrink to
		// whatever was left over, so the same screen drew a different animal as its
		// content changed; now it draws the rung the terminal calls for, or nothing
		// at all, and never a smaller creature than the one every other screen shows.
		panels := []panel{
			testPanel(panelMachine, "Machine", "Linux · x86_64", "OS  Linux"),
			testPanel(panelPlan, "Plan", "8 steps", "Steps  8"),
		}
		hints := []installerHint{hintUp, hintDown, hintSelect, hintQuit}

		rung := Model{Width: 100, Height: 24}.companionHeightNow()
		if rung < 1 {
			t.Fatal("this terminal height has no creature at all, so the test proves nothing")
		}

		// `spare` is the rows the body leaves blank. At the rung itself there is room
		// for the creature but no row left for the summary above it: the summary wins,
		// because a fact beats a decoration. With room to spare the creature draws the
		// SAME rung - not a bigger one, and not a smaller one.
		for _, spare := range []int{rung, rung + 4} {
			m := Model{Width: 100, Height: 24, Animating: true}
			l := layoutFor(m)
			rows := installerBodyRows(m.Height, footerRowCount(l.Inner, m.panelHints(panels, hints)))
			// placeBody centres a short body, so a body of rows-2*spare leaves
			// exactly `spare` blank rows at the bottom: the rows the decorations
			// share.
			body := make([]string, rows-2*spare)
			for i := range body {
				body[i] = fmt.Sprintf("body row %d", i+1)
			}

			view := ansiEscape.ReplaceAllString(m.frameWithPanels("Main Menu", "", body, hints, panels), "")
			_, _, companion := findCompanionRow(view)
			summary := strings.Contains(view, "Linux · x86_64")

			if !summary {
				t.Errorf("with %d spare row(s) the panel summary is gone:\n%s", spare, view)
			}
			want := 0
			if spare > rung {
				want = rung
			}
			if got := len(companionOwnedRows(strings.Split(view, "\n"))); got != want {
				t.Errorf("with %d spare row(s) the sprite owns %d rows, want %d: the creature draws the "+
					"terminal's rung or nothing, never an animal of a different size", spare, got, want)
			}
			if companion != (want > 0) {
				t.Errorf("with %d spare row(s) the companion is %v while it owns %d rows", spare, companion, want)
			}
			if rows := renderedRowCount(view); rows != 24-viewPaddingRows {
				t.Errorf("the frame renders %d rows with %d spare row(s), want %d",
					rows, spare, 24-viewPaddingRows)
			}
		}
	})
}

// TestCompanionTicksOnlyWithinTheStage pins the row budget: wherever the creature
// is, its cell stays inside the stage, so no frame guard can see it stick out.
//
// It used to walk the row to both edges and turn, so it was also the test of the
// idle stroll. There is no idle stroll now: the still half of what it asserted
// lives in the first block here -- two hundred ticks with no input leave the cell
// exactly where it was -- and, for the bytes, in
// TestCompanionStaysPutWhenNothingHappens. What is left is the walk the creature
// still does, and the bound is now checked over every target the menu can ask for
// rather than over a stroll: not a lost assertion but a wider one.
func TestCompanionTicksOnlyWithinTheStage(t *testing.T) {
	m := NewModel()
	isolateGoldenTest(t, &m)
	m.Screen = ScreenMainMenu
	m.Width, m.Height = 100, 24
	m.Animating = true

	stage := companionStageWidth(m)
	limit := stage - companionCellWidth

	// With no input at all the creature stands where it is, for far longer than the
	// twenty quiet seconds that put it to sleep: none of the two hundred ticks may
	// move it.
	for tick := 0; tick < 200; tick++ {
		m = companionTick(t, m)
		if m.CompanionPos != 0 {
			t.Fatalf("tick %d moved a creature with no input to cell %d, want 0", tick, m.CompanionPos)
		}
	}

	// Walking to every row the menu offers stays inside the stage: the cell never
	// leaves 0..limit, it lands on the target, and no row of the sprite at any
	// height is wider than the stage it is drawn on.
	options := m.GetCurrentOptions()
	if len(options) < 2 {
		t.Fatalf("the main menu offers %d options, want enough to walk between", len(options))
	}
	for target := 0; target < len(options); target++ {
		m.Cursor = target
		// The quiet stretch above put it to sleep, and a sleeping creature does not
		// walk; waking it is what a key does, and this block is about the awake walk.
		m.CompanionIdle = 0
		m.armCompanionFollow()
		for tick := 0; tick < 200 && m.CompanionFollow; tick++ {
			m = companionTick(t, m)
			if m.CompanionPos < 0 || m.CompanionPos > limit {
				t.Fatalf("walking to option %d left the companion at cell %d, outside 0..%d",
					target, m.CompanionPos, limit)
			}
			if row := m.companionRow(stage); len([]rune(plainRow(row))) > stage {
				t.Fatalf("walking to option %d drew a %d-column row on a %d-column stage",
					target, len([]rune(plainRow(row))), stage)
			}
			for _, height := range companionHeights() {
				for _, row := range m.companionSprite(stage, height) {
					if w := len([]rune(plainRow(row))); w > stage {
						t.Fatalf("walking to option %d drew a %d-column row at height %d on a %d-column stage",
							target, w, height, stage)
					}
				}
			}
		}
		if m.CompanionFollow {
			t.Fatalf("the companion never reached the row for option %d", target)
		}
		want, ok := m.companionTarget(stage)
		if !ok {
			t.Fatalf("option %d has no target cell", target)
		}
		if m.CompanionPos != want {
			t.Errorf("option %d left the companion at cell %d, want %d", target, m.CompanionPos, want)
		}
	}
}

// TestCompanionStepIsCappedAtOneCellPerFrame is the walk's speed bound: however
// far the creature has to go, one frame moves it at most companionStepCells -- one
// cell -- and never a fraction of the distance that remains. The proportional step
// is what made a long move read as a jump: from the far edge it arrived in one
// frame and then crawled the last cell. The walk is now a walk, at
// animTicksPerSecond cells a second, with one deliberate exception inside the last
// companionBrakeCells, where a step comes every other frame. It logs three
// consecutive frames' rows so the movement can be read instead of trusted.
func TestCompanionStepIsCappedAtOneCellPerFrame(t *testing.T) {
	if companionStepCells != 1 {
		t.Fatalf("the walk is %d cells a frame, want exactly one", companionStepCells)
	}

	m := NewModel()
	isolateGoldenTest(t, &m)
	m.Screen = ScreenMainMenu
	m.Width, m.Height = 160, 50
	m.Animating = true
	stage := companionStageWidth(m)

	options := m.GetCurrentOptions()
	m.Cursor = len(options) - 1
	m.armCompanionFollow()
	target, ok := m.companionTarget(stage)
	if !ok {
		t.Fatalf("the last option has no target cell")
	}
	if distance := companionDistance(m.CompanionPos, target); distance <= companionBrakeCells {
		t.Fatalf("the last option is only %d cells away, too close to read a long walk", distance)
	}

	first := companionTick(t, m)
	second := companionTick(t, first)
	if first.CompanionPos != m.CompanionPos+companionStepCells {
		t.Fatalf("one frame moved the companion from cell %d to %d, want %d cells",
			m.CompanionPos, first.CompanionPos, companionStepCells)
	}
	if second.CompanionPos != first.CompanionPos+companionStepCells {
		t.Fatalf("the next frame moved the companion from cell %d to %d, want %d cells",
			first.CompanionPos, second.CompanionPos, companionStepCells)
	}
	for _, f := range []Model{m, first, second} {
		t.Logf("frame %d, cell %d: %q", f.AnimTick, f.CompanionPos, plainRow(f.companionRow(stage)))
	}

	// The bound holds for the whole walk, not only for the first two frames: no
	// frame may move more than one cell, and no frame outside the braking window may
	// stand still.
	previous := second.CompanionPos
	frames := 2
	m = second
	for tick := 0; tick < 200 && m.CompanionFollow; tick++ {
		next := companionTick(t, m)
		step := companionDistance(previous, next.CompanionPos)
		if step > companionStepCells {
			t.Fatalf("tick %d moved the companion %d cells, want at most %d", tick, step, companionStepCells)
		}
		if step == 0 && companionDistance(previous, target) > companionBrakeCells {
			t.Fatalf("tick %d stood still at cell %d, %d cells from the target and outside the braking window",
				tick, previous, companionDistance(previous, target))
		}
		if step > 0 {
			frames++
		}
		previous = next.CompanionPos
		m = next
	}
	if m.CompanionPos != target {
		t.Fatalf("the companion stands at cell %d, want %d", m.CompanionPos, target)
	}
	t.Logf("a %d-cell walk took %d moving frames, at most %d cell each",
		target, frames, companionStepCells)
}

// --- following the selection ----------------------------------------------

// TestCompanionWalksTowardWhatTheCursorPointsAt is the behaviour the user asked
// for by name: moving the cursor moves the creature, over the ticks that follow
// rather than in the same frame, and the walk ends where the creature can see the
// row it was pointed at. A key that moves nothing leaves it standing still.
func TestCompanionWalksTowardWhatTheCursorPointsAt(t *testing.T) {
	build := func(t *testing.T) Model {
		t.Helper()
		m := NewModel()
		isolateGoldenTest(t, &m)
		m.Screen = ScreenMainMenu
		m.Width, m.Height = 160, 50
		m.Animating = true
		return m
	}

	m := build(t)
	options := m.GetCurrentOptions()
	if len(options) < 3 {
		t.Fatalf("the main menu offers %d options, want enough to move between", len(options))
	}
	stage := companionStageWidth(m)

	// A key that changes nothing must not send it anywhere: it stands still.
	quiet := companionKey(t, m, "x")
	if quiet.CompanionFollow {
		t.Errorf("a key that changed no selection armed the walk")
	}

	// Move the cursor to the last row and walk it there.
	last := len(options) - 1
	moved := m
	for i := 0; i < last; i++ {
		moved = companionKey(t, moved, "j")
	}
	if moved.Cursor != last {
		t.Fatalf("the cursor is on option %d, want %d", moved.Cursor, last)
	}
	if !moved.CompanionFollow {
		t.Fatalf("moving the cursor did not point the companion at the new row")
	}
	target, ok := moved.companionTarget(stage)
	if !ok {
		t.Fatalf("the last option has no target cell")
	}
	if want := min(last*companionFollowCells, stage-companionCellWidth); target != want {
		t.Errorf("the last option points at cell %d, want %d", target, want)
	}

	previousDistance := companionDistance(moved.CompanionPos, target)
	arrivedAfter := -1
	for tick := 1; tick <= 40; tick++ {
		moved = companionTick(t, moved)
		distance := companionDistance(moved.CompanionPos, target)
		// The walk never moves away from the row the cursor points at. It may stand
		// still on a frame inside the braking window -- the last companionBrakeCells
		// are crossed every other frame -- which is why the bound is "not further"
		// rather than "strictly closer".
		if distance > previousDistance {
			t.Fatalf("tick %d moved the companion away from the row the cursor points at: %d -> %d",
				tick, previousDistance, distance)
		}
		previousDistance = distance
		if distance == 0 {
			arrivedAfter = tick
			break
		}
	}
	if arrivedAfter < 0 {
		t.Fatalf("the companion never reached cell %d from %d", target, m.CompanionPos)
	}
	if moved.CompanionPos != target {
		t.Fatalf("the companion stands at cell %d, want %d", moved.CompanionPos, target)
	}

	// The tick after the one that lands is the tick it stands still on: the walk is
	// over and the frame is the idle one.
	stood := companionTick(t, moved)
	if stood.CompanionFollow {
		t.Errorf("the companion is still walking toward a row it has reached")
	}
	if stood.CompanionMoving {
		t.Errorf("the companion moved on the tick it should have stood still on")
	}
	if state := stood.companionStateNow(); state != companionIdleState {
		t.Errorf("the tick it arrives on draws state %d, want the idle one", state)
	}
	// There is no stroll to resume: the tick after the arrival is a second still one
	// and moves nothing.
	if still := companionTick(t, stood); still.CompanionMoving {
		t.Errorf("the companion walked on its own after arriving")
	}
	moved = stood

	// Walking it back is the same behaviour in the other direction.
	back := moved
	for i := 0; i < last; i++ {
		back = companionKey(t, back, "k")
	}
	if back.Cursor != 0 {
		t.Fatalf("the cursor is on option %d, want 0", back.Cursor)
	}
	before := back.CompanionPos
	for tick := 0; tick < 40 && back.CompanionPos != 0; tick++ {
		back = companionTick(t, back)
	}
	if back.CompanionPos != 0 {
		t.Errorf("the companion walked from cell %d to %d, want 0", before, back.CompanionPos)
	}
}

// TestCompanionFollowIsArmedOnlyWhereThereIsSomethingToPointAt pins the other
// half of the walk: a screen with no menu -- the installing screen, the trainer's
// exercises -- has no cursor to point with, so the creature stays where it is
// instead of walking to the corner for nothing.
func TestCompanionFollowIsArmedOnlyWhereThereIsSomethingToPointAt(t *testing.T) {
	m := NewModel()
	isolateGoldenTest(t, &m)
	m.Screen = ScreenInstalling
	m.Width, m.Height = 160, 50
	m.Animating = true
	m.Cursor = 3

	if _, ok := m.companionTarget(companionStageWidth(m)); ok {
		t.Errorf("the installing screen reports a target for a cursor it does not have")
	}
	m2 := companionKey(t, m, "x")
	if m2.CompanionFollow {
		t.Errorf("a screen with no menu armed the walk")
	}

	trainerModel := newTrainerFrameModel(t, trainer.ModuleHorizontal)
	trainerModel.Animating = true
	if _, ok := trainerModel.companionTarget(companionStageWidth(trainerModel)); ok {
		t.Errorf("the trainer's exercise screen reports a target for a cursor it does not have")
	}
}

// --- the trainer ----------------------------------------------------------

// TestCompanionReactsToTheTrainersVerdict pins the trainer's own reaction on the
// screen that shows the verdict, and pins that the row it draws in costs the
// trainer frame nothing: the creature takes the blank spacer above the legend, so
// the screen is exactly as tall as it was and the bottom row still lands inside
// the 80x24 frame.
func TestCompanionReactsToTheTrainersVerdict(t *testing.T) {
	correct := newTrainerFrameModel(t, trainer.ModuleHorizontal)
	correct.Animating = true
	correct.Screen = ScreenTrainerResult
	correct.TrainerLastCorrect = true

	wrong := newTrainerFrameModel(t, trainer.ModuleHorizontal)
	wrong.Animating = true
	wrong.Screen = ScreenTrainerResult
	wrong.TrainerLastCorrect = false

	if got := correct.companionStateNow(); got != companionPleasedState {
		t.Errorf("a correct result draws state %d, want the pleased one", got)
	}
	if got := wrong.companionStateNow(); got != companionFlinchingState {
		t.Errorf("a wrong result draws state %d, want the flinching one", got)
	}
	if correct.View() == wrong.View() {
		t.Errorf("the two result screens render identically, so the reaction is not visible")
	}

	exercise := newTrainerFrameModel(t, trainer.ModuleHorizontal)
	exercise.Animating = true
	withCreature := exercise.View()

	exercise.Animating = false
	without := exercise.View()

	if rows := renderedRowCount(withCreature); rows != renderedRowCount(without) {
		t.Errorf("the companion changed the exercise screen's height from %d to %d",
			renderedRowCount(without), renderedRowCount(withCreature))
	}
	if rows := renderedRowCount(withCreature); rows > trainerFrameHeight {
		t.Errorf("the exercise screen renders %d rows, past the %d-row frame it claims",
			rows, trainerFrameHeight)
	}
	before := strings.Split(without, "\n")
	after := strings.Split(withCreature, "\n")
	if len(before) != len(after) {
		t.Fatalf("the companion changed the row count from %d to %d", len(before), len(after))
	}
	changed := 0
	for i := range before {
		if before[i] != after[i] {
			changed++
			if !companionRowHasArt(after[i]) {
				t.Errorf("the companion changed a row that is not its own: %q", plainRow(after[i]))
			}
		}
	}
	if changed != 1 {
		t.Errorf("the companion changed %d rows of the exercise screen, want exactly 1", changed)
	}
}

// --- the destructive row --------------------------------------------------

// TestCompanionIsAlertOnTheRowsThatThrowSomethingAway pins the alert rule: the
// two screens whose purpose is to restore or overwrite are alert on every row,
// and everywhere else the row under the cursor decides. The second half is a
// table over every screen that offers options, so a row added later that discards
// something is caught here rather than by someone remembering a word list.
func TestCompanionIsAlertOnTheRowsThatThrowSomethingAway(t *testing.T) {
	t.Run("the word list", func(t *testing.T) {
		tests := []struct {
			row  string
			want bool
		}{
			{"🔄 Restore from Backup", true},
			{"🗑️ Delete this backup", true},
			{"⚠️ Install without Backup", true},
			{"✅ Install with Backup (recommended)", false},
			{"✅ Yes, restore this backup", true},
			{"🚀 Start Installation", false},
			{"❌ Exit", false},
			{"← Back", false},
			{"", false},
		}
		for _, tt := range tests {
			if got := companionDestructiveRow(tt.row); got != tt.want {
				t.Errorf("companionDestructiveRow(%q) = %v, want %v", tt.row, got, tt.want)
			}
		}
	})

	t.Run("the rows every screen offers", func(t *testing.T) {
		build := func(t *testing.T) Model {
			t.Helper()
			m := NewModel()
			isolateGoldenTest(t, &m)
			m.AvailableBackups = []system.BackupInfo{testBackupInfo()}
			return m
		}

		// The destructive rows the installer offers today, by screen and cursor.
		// A row added later that discards something fails here instead of shipping
		// quietly.
		want := map[Screen]map[int]bool{
			ScreenMainMenu:       {5: true}, // "Restore from Backup"
			ScreenBackupConfirm:  {1: true}, // "Install without Backup"
			ScreenRestoreBackup:  {},
			ScreenRestoreConfirm: {0: true, 1: true}, // restore this backup, delete this backup
			ScreenOSSelect:       {},
			ScreenFontSelect:     {},
			ScreenShellSelect:    {},
			ScreenWMSelect:       {},
			ScreenNvimSelect:     {},
			ScreenGhosttyWarning: {},
			ScreenKeymapsMenu:    {},
			ScreenLearnTerminals: {},
			ScreenLearnLazyVim:   {},
		}

		for screen, rows := range want {
			m := build(t)
			m.Screen = screen
			options := m.GetCurrentOptions()
			if len(options) == 0 {
				t.Fatalf("screen %v offers no options, so the table cannot be checked", screen)
			}
			for cursor := range options {
				m.Cursor = cursor
				wantRow := rows[cursor]
				if got := companionDestructiveRow(m.companionSelectedRow()); got != wantRow {
					t.Errorf("screen %v row %d (%q) is destructive=%v, want %v",
						screen, cursor, options[cursor], got, wantRow)
				}
				// The two screens whose purpose is to restore are alert on every
				// row, including the rows that do not name a destructive verb.
				wantAlert := wantRow || alertOnEveryRow(screen)
				if got := m.companionAlerting(); got != wantAlert {
					t.Errorf("screen %v alerts=%v on row %d, want %v", screen, got, cursor, wantAlert)
				}
			}
		}
	})

	t.Run("the restore screens are alert on every row", func(t *testing.T) {
		for _, screen := range []Screen{ScreenBackupConfirm, ScreenRestoreBackup, ScreenRestoreConfirm} {
			if !alertOnEveryRow(screen) {
				t.Fatalf("screen %v is not one of the restore screens", screen)
			}
			m := NewModel()
			isolateGoldenTest(t, &m)
			m.Screen = screen
			m.AvailableBackups = []system.BackupInfo{testBackupInfo()}
			for cursor := 0; cursor < 3; cursor++ {
				m.Cursor = cursor
				if !m.companionAlerting() {
					t.Errorf("screen %v is quiet on row %d", screen, cursor)
				}
			}
		}
	})
}

// alertOnEveryRow reports whether a screen is one whose whole purpose is to
// restore or overwrite, so the companion is alert whatever the cursor is on.
func alertOnEveryRow(screen Screen) bool {
	switch screen {
	case ScreenBackupConfirm, ScreenRestoreBackup, ScreenRestoreConfirm:
		return true
	}
	return false
}

// --- determinism ----------------------------------------------------------

// TestCompanionRendersTheSameBytesAndMovesNothing pins the two properties a
// snapshot depends on: the same model and tick render the same bytes every time,
// and rendering does not move the creature or the clock it reads. Nothing here
// reads the clock, a random source or a map, so two runs of the same program
// produce the same frames.
func TestCompanionRendersTheSameBytesAndMovesNothing(t *testing.T) {
	state := func(m Model) string {
		return fmt.Sprintf("%d/%v/%v/%d/%d/%d/%d/%d",
			m.CompanionPos, m.CompanionFollow, m.CompanionMoving,
			m.CompanionIdle, m.CompanionPleased, m.AnimTick, m.CompanionGaze.X, m.CompanionGaze.Y)
	}

	build := func(t *testing.T) Model {
		t.Helper()
		m := NewModel()
		isolateGoldenTest(t, &m)
		m.Screen = ScreenMainMenu
		m.Width, m.Height = 160, 50
		m.Animating, m.AnimTick = true, 3
		return companionKey(t, companionTicks(t, m, 2), "j")
	}

	first := build(t)
	second := build(t)
	if first.View() != second.View() {
		t.Errorf("two identical models rendered different bytes")
	}

	before := state(first)
	rendered := first.View()
	for i := 0; i < 5; i++ {
		if got := first.View(); got != rendered {
			t.Fatalf("render %d differs from the one before it", i)
		}
	}
	if after := state(first); after != before {
		t.Errorf("rendering moved the companion from %s to %s", before, after)
	}

	// The tick, not the render, is what moves it, and it moves one step a tick.
	moved := companionTick(t, first)
	if state(moved) == before {
		t.Errorf("a tick did not move the companion")
	}
}

// TestCompanionTicksAreNotReadFromTheClock is the determinism rule stated as a
// test: two models built the same way, advanced the same number of ticks, render
// the same bytes -- and their frames come from the counter alone, so the same
// counter always draws the same art.
func TestCompanionTicksAreNotReadFromTheClock(t *testing.T) {
	first := NewModel()
	isolateGoldenTest(t, &first)
	second := NewModel()
	isolateGoldenTest(t, &second)

	for _, m := range []*Model{&first, &second} {
		m.Screen = ScreenMainMenu
		m.Width, m.Height = 160, 50
		m.Animating = true
	}
	first = companionTicks(t, first, 6)
	second = companionTicks(t, second, 6)

	if first.View() != second.View() {
		t.Errorf("two models advanced by six ticks rendered different bytes")
	}
	// The sixth tick's frame is on screen, at whatever height the ladder picked for
	// this size, and it is the frame the counter names rather than the one before.
	rows := strings.Split(first.View(), "\n")
	owned := companionOwnedRows(rows)
	if len(owned) == 0 {
		t.Fatalf("the sixth tick drew no companion at all")
	}
	art := companionArtFor(first.companionStateNow(), len(owned), 6, first.CompanionGaze)
	if len(art) != len(owned) {
		t.Fatalf("the frame at height %d has %d rows but the view draws %d", len(owned), len(art), len(owned))
	}
	for i, row := range owned {
		if got := strings.TrimSpace(plainRow(rows[row])); got != strings.TrimSpace(art[i]) {
			t.Errorf("row %d of the sixth tick's frame draws %q, want %q", i, got, art[i])
		}
	}

	// Time passing without a tick changes nothing: the clock is the counter.
	time.Sleep(20 * time.Millisecond)
	if got := first.View(); got != second.View() {
		t.Errorf("the view changed with no tick, so it read the clock")
	}
}

// testBackupInfo is one backup row for the models that offer the restore path.
func testBackupInfo() system.BackupInfo {
	return system.BackupInfo{
		Path:      "/home/testuser/.dotfiles-backup-2026-01-01",
		Timestamp: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
		Files:     []string{"/home/testuser/.zshrc"},
	}
}

// ============================================================================
// THE POINTER
// ============================================================================
//
// The gaze follows the pointer when the run asked the terminal for mouse motion
// and falls back to the selection when it did not. These tests pin both halves:
// the gate that decides which one is in force, the gaze the pointer produces, the
// rows a pointer event is allowed to change, the wake-and-click reactions, and
// what happens to all of it when the gate is off.

// companionMouse drives one mouse message through Update, the way the program
// does, and returns the model it produced.
func companionMouse(t *testing.T, m Model, msg tea.MouseMsg) Model {
	t.Helper()
	next, _ := m.Update(msg)
	got, ok := next.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", next)
	}
	return got
}

// companionMotion is the pointer moving to one cell.
func companionMotion(x, y int) tea.MouseMsg {
	return tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionMotion, Button: tea.MouseButtonNone}
}

// companionClick is the left button going down on one cell.
func companionClick(x, y int) tea.MouseMsg {
	return tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
}

// pointerModel is the model every pointer test starts from: a screen with room
// for the full sprite and a run that asked the terminal for the pointer.
func pointerModel(t *testing.T, width, height int) Model {
	t.Helper()
	m := NewModel()
	isolateGoldenTest(t, &m)
	m.Screen = ScreenMainMenu
	m.Width, m.Height = width, height
	m.Animating, m.Hovering = true, true
	return m
}

// TestCompanionHoverGateIsItsOwnSwitch pins the pointer's gate: it is not the
// animation gate, and it turns off for the three things that make a pointer
// useless -- the switch, a stdout that is not a terminal, and a Termux session,
// where the pointer is a finger. The forced value overrides the Termux default
// because a Termux session with a real mouse attached is a real case.
func TestCompanionHoverGateIsItsOwnSwitch(t *testing.T) {
	terminal, err := os.Open(os.DevNull) // a character device, which is what the gate asks for
	if err != nil {
		t.Fatalf("opening %s: %v", os.DevNull, err)
	}
	defer terminal.Close()

	regular, err := os.CreateTemp(t.TempDir(), "not-a-terminal")
	if err != nil {
		t.Fatalf("creating a regular file: %v", err)
	}
	defer regular.Close()

	tests := []struct {
		name   string
		mouse  string
		termux string
		file   *os.File
		want   bool
	}{
		{"a terminal with nothing set hovers", "", "", terminal, true},
		{"DOTFILES_MOUSE=0 turns the pointer off", "0", "", terminal, false},
		{"DOTFILES_MOUSE=1 keeps it on", "1", "", terminal, true},
		{"a Termux session defaults the pointer off", "", "0.118.0", terminal, false},
		{"DOTFILES_MOUSE=1 overrides the Termux default", "1", "0.118.0", terminal, true},
		{"a stdout that is not a terminal cannot hover", "", "", regular, false},
		{"no stdout at all cannot hover", "", "", nil, false},
		{"an unrelated value is not a switch", "yes", "", terminal, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(envMouse, tt.mouse)
			t.Setenv(envTermux, tt.termux)
			if got := hoverGate(tt.file); got != tt.want {
				t.Errorf("hoverGate = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestCompanionAsksForNoPointerWithoutACreature pins the coupling between the two
// gates. The package's TestMain pins the animation gate off, so the model built
// here is the frozen case: with DOTFILES_MOUSE=1 there is still nothing to look
// with, and a run that asked the terminal for the pointer anyway would have cost
// the user the terminal's own selection for nothing.
func TestCompanionAsksForNoPointerWithoutACreature(t *testing.T) {
	t.Setenv(envMouse, envMouseForce)

	m := NewModel()
	if m.Animating {
		t.Fatalf("the package's animation gate is on, so this test cannot tell the two halves apart")
	}
	if m.Hovering {
		t.Errorf("Hovering = true with the animation gate off, want false")
	}

	t.Setenv(envMouse, "0")
	if off := NewModel(); off.Hovering {
		t.Errorf("Hovering = true with DOTFILES_MOUSE=0, want false")
	}
}

// TestCompanionGazeFollowsThePointer pins what the pointer does to the eyes: the
// pupil pair turns to the side the pointer is on and looks up when the pointer is
// above the creature's band, both of them one column outside the dead zone, and
// the turn happens on the message rather than on the next tick -- which is what
// makes the eyes arrive with the mouse instead of a frame later.
func TestCompanionGazeFollowsThePointer(t *testing.T) {
	const width, height = 160, 50
	m := pointerModel(t, width, height)
	m.CompanionPos = 60
	anchor := m.CompanionPos
	level := companionGroundTop(height)

	tests := []struct {
		name string
		x, y int
		want companionGaze
	}{
		{"left of the creature", viewPaddingCols + anchor - companionGazeDeadZone - 1, height - 2, companionGaze{X: -1, Y: 1}},
		{"just inside the dead zone", viewPaddingCols + anchor - companionGazeDeadZone, height - 2, companionGaze{Y: 1}},
		{"straight at the creature", viewPaddingCols + anchor, height - 2, companionGaze{Y: 1}},
		{"right of the creature", viewPaddingCols + anchor + companionGazeDeadZone + 1, height - 2, companionGaze{X: 1, Y: 1}},
		{"above the creature's band", viewPaddingCols + anchor, level - 1, companionGaze{Y: -1}},
		{"level with the creature's band", viewPaddingCols + anchor, level, companionGaze{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := companionMouse(t, m, companionMotion(tt.x, tt.y))
			if got.CompanionGaze != tt.want {
				t.Errorf("gaze = %+v, want %+v", got.CompanionGaze, tt.want)
			}
			if !got.PointerSet {
				t.Errorf("the pointer was not remembered")
			}
			if got.AnimTick != m.AnimTick {
				t.Errorf("the gaze turned on a tick rather than on the message")
			}
		})
	}

	// The column is clamped to the row the creature can walk: a pointer past the
	// right edge of the stage points at the last cell it could stand on, which is
	// where oneko's cat stops too.
	edge := companionMouse(t, m, companionMotion(width, height-2))
	if edge.PointerCol != width {
		t.Errorf("the pointer was not remembered as it arrived: %d", edge.PointerCol)
	}
	if got := edge.CompanionGaze; got != (companionGaze{X: 1, Y: 1}) {
		t.Errorf("a pointer past the stage turned the gaze %+v, want it right", got)
	}
	if edge.CompanionPos != m.CompanionPos {
		t.Errorf("the pointer moved the creature: cell %d, want %d", edge.CompanionPos, m.CompanionPos)
	}
}

// TestCompanionPointerChangesOnlyItsOwnRows pins the cost of the pointer the way
// the tick's own test pins the tick's: a pointer event may change the rows the
// creature draws in and nothing else, and a pointer that lands on the cell it was
// already on changes nothing at all -- which is what keeps a hover from streaming
// escape sequences into a terminal whose frame did not change.
func TestCompanionPointerChangesOnlyItsOwnRows(t *testing.T) {
	fixtures := []companionScreenFixture{
		{name: "main menu 160x50", screen: ScreenMainMenu, width: 160, height: 50, framed: true},
		{name: "main menu 227x62", screen: ScreenMainMenu, width: 227, height: 62, framed: true},
		{name: "welcome 160x50", screen: ScreenWelcome, width: 160, height: 50, framed: true},
		{name: "os select 100x24", screen: ScreenOSSelect, width: 100, height: 24, framed: true},
	}

	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			m := NewModel()
			isolateGoldenTest(t, &m)
			m.Screen = f.screen
			m.Width, m.Height = f.width, f.height
			m.Animating, m.Hovering, m.AnimTick = true, true, 3

			pointer := func(x int) tea.MouseMsg { return companionMotion(x, m.Height-2) }
			far := viewPaddingCols + m.CompanionPos + 20

			before := m.View()
			moved := companionMouse(t, m, pointer(far))
			after := moved.View()

			beforeRows := strings.Split(before, "\n")
			afterRows := strings.Split(after, "\n")
			if len(beforeRows) != len(afterRows) {
				t.Fatalf("a pointer event changed the row count from %d to %d",
					len(beforeRows), len(afterRows))
			}
			var changed []int
			for i := range beforeRows {
				if beforeRows[i] != afterRows[i] {
					changed = append(changed, i)
				}
			}
			if len(changed) == 0 {
				t.Fatalf("a pointer event that turned the gaze changed no row at all")
			}
			owned := map[int]bool{}
			for _, row := range companionOwnedRows(afterRows) {
				owned[row] = true
			}
			for _, row := range changed {
				if !owned[row] {
					t.Errorf("a pointer event changed row %d, which carries no art: %q",
						row, plainRow(afterRows[row]))
				}
			}

			// The same cell twice is not a change: the creature is already looking
			// there, so the view is byte for byte what it was.
			again := companionMouse(t, moved, pointer(far)).View()
			if again != after {
				t.Errorf("a pointer event on the cell it was already on changed the view")
			}
		})
	}
}

// TestCompanionPointerWakesItAndAParkedMouseDoesNot pins the two halves of the
// sleep rule. A pointer event is the user moving the mouse -- a parked mouse sends
// no events at all, so there is nothing else a pointer event can mean -- and it
// wakes the creature and turns its gaze. With no events the quiet stretch still
// runs out, which is what the same screen proves a tick before.
func TestCompanionPointerWakesItAndAParkedMouseDoesNot(t *testing.T) {
	m := pointerModel(t, 160, 50)
	m = companionTicks(t, m, companionSleepTicks)

	if got := m.companionStateNow(); got != companionAsleepState {
		t.Fatalf("with no events for %d ticks the creature draws state %s, want asleep",
			companionSleepTicks, companionStateNames[got])
	}

	woke := companionMouse(t, m, companionMotion(viewPaddingCols+m.CompanionPos+20, m.Height-2))
	if woke.CompanionIdle != 0 {
		t.Errorf("the quiet stretch is %d frames after a pointer event, want 0", woke.CompanionIdle)
	}
	if got := woke.companionStateNow(); got == companionAsleepState {
		t.Errorf("the creature is still asleep after the pointer moved")
	}
	if got := woke.CompanionGaze; got != (companionGaze{X: 1, Y: 1}) {
		t.Errorf("a waking pointer turned the gaze %+v, want it right", got)
	}
}

// TestCompanionClickHopsAndCelebrates pins the click reaction: the same
// celebration a finished step earns, plus a jump that is drawn as one blank row
// under the creature. The jump is the placement's business, so it is pinned where
// it is decided: the creature's first row moves one row up while the hop lasts and
// returns when the tick has aged it.
func TestCompanionClickHopsAndCelebrates(t *testing.T) {
	m := pointerModel(t, 160, 50)
	grounded, _, ok := findCompanionRow(m.View())
	if !ok {
		t.Fatalf("the roomy screen drew no companion:\n%s", m.View())
	}

	clicked := companionMouse(t, m, companionClick(viewPaddingCols+m.CompanionPos+6, m.Height-2))
	if clicked.CompanionPleased != companionPleasedTicks {
		t.Errorf("the click set the celebration to %d frames, want %d",
			clicked.CompanionPleased, companionPleasedTicks)
	}
	if clicked.CompanionHop != companionHopTicks {
		t.Errorf("the click set the hop to %d frames, want %d",
			clicked.CompanionHop, companionHopTicks)
	}

	lifted, _, ok := findCompanionRow(clicked.View())
	if !ok {
		t.Fatalf("the click left the screen with no companion:\n%s", clicked.View())
	}
	if lifted != grounded-1 {
		t.Errorf("the hop drew the creature's first row at %d, want one row above %d",
			lifted, grounded)
	}

	landed := companionTicks(t, clicked, companionHopTicks)
	if landed.CompanionHop != 0 {
		t.Errorf("after %d ticks the hop has %d frames left, want 0",
			companionHopTicks, landed.CompanionHop)
	}
	if back, _, ok := findCompanionRow(landed.View()); !ok || back != grounded {
		t.Errorf("after the hop the creature's first row is %d (found %v), want %d",
			back, ok, grounded)
	}
}

// TestCompanionHopNeedsItsOwnRow pins the row budget of the jump: the sprite gains
// a blank row under it, so the hop costs one spare row more than the sprite itself.
// Where the frame has no such row the creature stays on the ground and the
// celebration shows in its face, which is the rule that no decoration takes a row a
// fact needs.
//
// The last two rows of the table used to expect a smaller creature - the compact
// sprite - because the rung was read from the spare rows. A rung chosen by the
// terminal cannot do that: when a frame has less room than the rung, no creature is
// drawn at all, because a creature that changes size with its screen is the defect
// this contract exists to remove.
func TestCompanionHopNeedsItsOwnRow(t *testing.T) {
	body := make([]string, 10)
	for i := range body {
		body[i] = fmt.Sprintf("body row %d", i+1)
	}

	rung := (Model{Width: 160, Height: 50}).companionHeightNow()
	if rung < 1 {
		t.Fatal("this terminal height has no creature at all, so the test proves nothing")
	}

	tests := []struct {
		name   string
		spare  int
		drawn  int // how many rows carry art: the sprite's own rows, hop or no hop
		lifted bool
	}{
		{"room for the sprite and the hop", rung + 1, rung, true},
		{"room for the sprite only", rung, rung, false},
		{"not enough room for the rung", rung - 1, 0, false},
		{"far less room than the rung needs", 2, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{Width: 160, Height: 50, Animating: true, Hovering: true, CompanionHop: companionHopTicks}
			placed := append(append([]string(nil), body...), make([]string, tt.spare)...)

			got := m.placeCompanion(placed, nil, companionStageWidth(m))
			if len(got) != len(placed) {
				t.Fatalf("placement drew %d rows on a %d-row frame", len(got), len(placed))
			}
			drawn := 0
			for _, row := range got {
				if companionRowHasArt(row) {
					drawn++
				}
			}
			if drawn != tt.drawn {
				t.Errorf("%d rows carry art, want the sprite's %d", drawn, tt.drawn)
			}
			if tt.drawn == 0 {
				// No creature: there is no hop to check and no row it may touch.
				return
			}
			if last := companionRowHasArt(got[len(got)-1]); last == tt.lifted {
				if tt.lifted {
					t.Errorf("the hop drew art on the frame's last row, so the creature did not lift")
				} else {
					t.Errorf("the creature did not lift and its last row carries no art")
				}
			}
		})
	}
}

// TestCompanionIgnoresWhatItCannotUse pins the two events that are not the
// creature's: a wheel, which the alternate screen has no scrollback for, and any
// pointer event at all on a run whose gate is off. Neither may move the creature,
// wake it, or change a single byte of the view -- a gate that only stopped the
// gaze would still have cost the user the terminal's own selection.
func TestCompanionIgnoresWhatItCannotUse(t *testing.T) {
	t.Run("the pointer is inert when the gate is off", func(t *testing.T) {
		m := pointerModel(t, 160, 50)
		m.Hovering = false

		got := companionMouse(t, m, companionClick(viewPaddingCols+m.CompanionPos, m.Height-2))
		if got.PointerSet {
			t.Errorf("the pointer was remembered with the gate off")
		}
		if got.CompanionHop != 0 || got.CompanionPleased != m.CompanionPleased {
			t.Errorf("a click moved the creature with the gate off")
		}
		if got.CompanionIdle != m.CompanionIdle {
			t.Errorf("a click woke the creature with the gate off")
		}
		if got.View() != m.View() {
			t.Errorf("a click changed the view with the gate off")
		}
	})

	t.Run("a wheel is not an event", func(t *testing.T) {
		m := pointerModel(t, 160, 50)

		got := companionMouse(t, m, tea.MouseMsg{
			X: viewPaddingCols + m.CompanionPos, Y: m.Height - 2,
			Action: tea.MouseActionPress, Button: tea.MouseButtonWheelUp,
		})
		if got.PointerSet {
			t.Errorf("a wheel was remembered as a pointer position")
		}
		if got.CompanionIdle != m.CompanionIdle || got.CompanionGaze != m.CompanionGaze {
			t.Errorf("a wheel woke the creature or turned its gaze")
		}
		if got.View() != m.View() {
			t.Errorf("a wheel changed the view")
		}
	})
}

// ============================================================================
// THE VOLUMETRIC SPRITE
// ============================================================================
//
// The sprite above the glyph ladder is a volume: a field for its shape, a light for its
// shading and a fixed dither for the ramp between them. These tests pin the rungs it is
// drawn at, the shading and the dither that are the drawing, the gaze and the gait that
// move it, the layout the eye reads it by, its determinism and its cost; the benchmarks
// after them measure what a live run pays for it.

// companionGazes is every gaze the composer can be asked for.
func companionGazes() []companionGaze {
	return []companionGaze{{}, {X: -1}, {X: 1}, {Y: -1}, {Y: 1}, {X: -1, Y: -1}, {X: 1, Y: -1}, {X: -1, Y: 1}, {X: 1, Y: 1}}
}

// companionVolumeRungs is the volume's two sizes, in the ladder's own order: the full
// sprite first and the small one after it.
func companionVolumeRungs() []companionVolumeSize {
	rungs := make([]companionVolumeSize, 0, 2)
	for _, height := range []int{companionVolumeFullHeight, companionVolumeSmallHeight} {
		if size, ok := companionVolumeSizeFor(height); ok {
			rungs = append(rungs, size)
		}
	}
	return rungs
}

// companionVolumeText renders one grid as the text a reader can look at: the ramp as
// characters from the darkest fold to the lit fur, and the marks as their own letters. It
// exists because a frame of this sprite is colours and a test's failure message is not,
// and the tests that say something about the drawing use it to say what they saw.
func companionVolumeText(grid [][]companionTone) string {
	glyphs := map[companionTone]rune{
		companionToneNone:  ' ',
		companionToneRamp0: '.',
		companionToneRamp1: ':',
		companionToneRamp2: '-',
		companionToneRamp3: '=',
		companionToneRamp4: '*',
		companionToneEye:   'o',
		companionTonePupil: '@',
		companionToneGlint: '+',
		companionToneNose:  '^',
	}
	var out strings.Builder
	for _, row := range grid {
		for _, tone := range row {
			glyph, ok := glyphs[tone]
			if !ok {
				glyph = '?'
			}
			out.WriteRune(glyph)
		}
		out.WriteByte('\n')
	}
	return out.String()
}

// companionVolumeGrid is one frame of the volume at one rung, with the tremble of a
// startle left out: the tests below are about the shape, the light and the movement, and
// none of them is about the shiver, which has its own test.
func companionVolumeGrid(state companionState, tick int, gaze companionGaze, size companionVolumeSize) [][]companionTone {
	return companionVolumeTones(state, tick, gaze, 0, size)
}

// TestCompanionAnatomyIsStructural checks the volume and its raster, not just a saved frame.
func TestCompanionAnatomyIsStructural(t *testing.T) {
	size, _ := companionVolumeSizeFor(companionVolumeFullHeight)
	pose := companionPoseFor(companionIdleState, 0, companionGaze{}, 0, size)
	skullTop := pose.head.y + pose.head.b
	for i, ear := range pose.ears {
		top := math.Max(ear.points[0][1], math.Max(ear.points[1][1], ear.points[2][1]))
		if top <= skullTop {
			t.Errorf("ear %d maximum %.3f is not above skull maximum %.3f", i, top, skullTop)
		}
	}

	area := func(blob companionBlob) float64 { return blob.a * blob.b }
	if area(pose.haunch) <= math.Max(area(pose.chest), area(pose.body)) || area(pose.haunch) <= area(pose.neck) {
		t.Errorf("haunch %.4f is not largest (chest %.4f body %.4f neck %.4f)",
			area(pose.haunch), area(pose.chest), area(pose.body), area(pose.neck))
	}
	for i, leg := range pose.legs {
		if leg.y1-leg.r > 0.01 {
			t.Errorf("leg %d stops at y=%.3f and does not reach ground line y=0", i, leg.y1-leg.r)
		}
	}

	grid := companionVolumeGrid(companionIdleState, 0, companionGaze{}, size)
	eyeCentres := companionEyeCentres(pose.head, size, companionVolumeScale(size))
	for i, ear := range pose.ears {
		var centre [2]float64
		for _, point := range ear.points {
			centre[0] += point[0] / 3
			centre[1] += point[1] / 3
		}
		inner := ear
		for j, point := range ear.points {
			inner.points[j][0] = centre[0] + (point[0]-centre[0])*0.42
			inner.points[j][1] = centre[1] + (point[1]-centre[1])*0.42
		}
		innerPixels := 0
		for py := range grid {
			for px := range grid[py] {
				worldX, worldY := companionWorldX(px, size, companionVolumeScale(size)), companionWorldY(py, size, companionVolumeScale(size))
				insideEye := false
				for _, eye := range eyeCentres {
					insideEye = insideEye || companionEyeBox(eye, size, px, py)
				}
				if inner.contains(worldX, worldY) && !insideEye && grid[py][px] == companionTonePupil {
					innerPixels++
				}
			}
		}
		if innerPixels == 0 {
			t.Errorf("ear %d has no darker inner-ear raster pixels outside EYE BOX", i)
		}
	}

	for i, leg := range pose.legs {
		paw := companionPixelOf(leg.x1, leg.y1, size, companionVolumeScale(size))
		pad := false
		for dx := -1; dx <= 1; dx++ {
			x, y := paw[0]+dx, paw[1]
			if y >= 0 && y < len(grid) && x >= 0 && x < len(grid[y]) && grid[y][x] == companionToneRamp0 {
				pad = true
			}
		}
		if !pad {
			t.Errorf("leg %d has no dark paw-pad pixel", i)
		}
	}

	nose := companionPixelOf(pose.muzzle.x+0.045, pose.muzzle.y+0.03, size, companionVolumeScale(size))
	for dx := 0; dx < 2; dx++ {
		if grid[nose[1]][nose[0]+dx] != companionToneNose {
			t.Errorf("nose pixel %d is not the dark nose tone", dx)
		}
	}
	mouth := companionPixelOf(pose.muzzle.x+0.03, pose.muzzle.y-0.045, size, companionVolumeScale(size))
	if grid[mouth[1]+1][mouth[0]] != companionTonePupil || grid[mouth[1]][mouth[0]+1] != companionTonePupil {
		t.Errorf("the largest rung does not draw the two-pixel mouth under its nose")
	}
	for _, offset := range []float64{-0.09, 0, 0.09} {
		for _, side := range []float64{-1, 1} {
			for step := 0; step < 3; step++ {
				point := companionWhiskerPixel(pose.muzzle, offset, side, step, size, companionVolumeScale(size), eyeCentres)
				if grid[point[1]][point[0]] != companionTonePupil {
					t.Errorf("whisker at (%d, %d) is missing", point[0], point[1])
				}
			}
		}
	}

	// Rasterize the tail primitives alone so the torso cannot hide a taper at the root.
	scale := companionVolumeScale(size)
	counts := make([]int, size.width)
	for py := 0; py < size.rows; py++ {
		for px := 0; px < size.width; px++ {
			x, y := companionWorldX(px, size, scale), companionWorldY(py, size, scale)
			for _, segment := range pose.tail {
				if segment.field(x, y) >= companionSurface {
					counts[px]++
					break
				}
			}
		}
	}
	base, tip := companionPixelOf(-0.70, 0.60, size, scale)[0], companionPixelOf(-0.96, 1.16, size, scale)[0]
	if base < tip {
		base, tip = tip, base
	}
	previous := counts[base]
	for x := base - 1; x >= tip; x-- {
		if counts[x] > previous {
			t.Errorf("tail pixel count grows from base to tip at column %d: %d after %d", x, counts[x], previous)
		}
		previous = counts[x]
	}
}

// companionVolumeGridTones counts the tones one grid uses, which is how the tests below
// ask whether the shading is really there.
func companionVolumeGridTones(grid [][]companionTone) map[companionTone]int {
	counts := map[companionTone]int{}
	for _, row := range grid {
		for _, tone := range row {
			counts[tone]++
		}
	}
	return counts
}

// TestCompanionVolumeIsATonedRectangle pins the volume's own rules at every state, every
// gaze and both rungs: the grid is the rung's own rectangle, every tone in it has a name,
// and a frame is really shaded -- four of the ramp's five tones and one of the face's
// marks, which is what a frame of this tier is and what a frame that had gone flat or
// faceless would fail to be.
func TestCompanionVolumeIsATonedRectangle(t *testing.T) {
	for _, size := range companionVolumeRungs() {
		for _, state := range companionStates() {
			for _, gaze := range companionGazes() {
				grid := companionVolumeGrid(state, 0, gaze, size)
				if len(grid) != size.rows {
					t.Fatalf("state %s draws %d pixel rows, want %d", companionStateNames[state], len(grid), size.rows)
				}
				for i, row := range grid {
					if len(row) != size.width {
						t.Fatalf("state %s draws row %d as %d pixels, want %d",
							companionStateNames[state], i, len(row), size.width)
					}
					for x, tone := range row {
						if _, ok := companionToneNames[tone]; !ok {
							t.Errorf("state %s draws tone %d at row %d column %d, which has no name",
								companionStateNames[state], tone, i, x)
						}
					}
				}

				counts := companionVolumeGridTones(grid)
				steps := 0
				for step := 0; step < companionRampSteps; step++ {
					if counts[companionRampTone(step)] > 0 {
						steps++
					}
				}
				if steps < companionRampSteps-1 {
					t.Errorf("state %s at %d rows uses %d of the %d ramp tones, so it is not shaded:\n%s",
						companionStateNames[state], size.rows, steps, companionRampSteps, companionVolumeText(grid))
				}
				if counts[companionToneEye]+counts[companionTonePupil]+counts[companionToneGlint] == 0 {
					t.Errorf("state %s at %d rows draws no eye at all:\n%s",
						companionStateNames[state], size.rows, companionVolumeText(grid))
				}
			}
		}
	}
}

// TestCompanionVolumeShadingRunsFromTheLight puts the light model's one claim to the test
// it is worth: the side the light comes from is brighter than the side it does not. The
// pixels of the volume are split by where they sit against the light's own diagonal, and
// the mean tone of the lit side has to beat the mean tone of the shaded one -- which a
// flat fill, an inside-out normal or a light on the wrong side all fail.
func TestCompanionVolumeShadingRunsFromTheLight(t *testing.T) {
	size, ok := companionVolumeSizeFor(companionVolumeFullHeight)
	if !ok {
		t.Fatal("the full height is not a rung of the volume")
	}
	grid := companionVolumeGrid(companionIdleState, 0, companionGaze{}, size)

	lit, litCount, shaded, shadedCount := 0, 0, 0, 0
	for py, row := range grid {
		for px, tone := range row {
			if tone < companionToneRamp0 || tone > companionRampTone(companionRampSteps-1) {
				continue
			}
			// The light is up and to the left, so the diagonal that separates the two sides
			// is the grid's own: a pixel is lit when its column plus its row is small.
			if px+py < (size.width+size.rows)/2 {
				lit += int(tone)
				litCount++
				continue
			}
			shaded += int(tone)
			shadedCount++
		}
	}
	if litCount == 0 || shadedCount == 0 {
		t.Fatalf("the volume has nothing on one side of the light:\n%s", companionVolumeText(grid))
	}
	litMean := float64(lit) / float64(litCount)
	shadedMean := float64(shaded) / float64(shadedCount)
	if litMean <= shadedMean {
		t.Errorf("the lit side of the creature means %.2f on the ramp and the shaded side %.2f, so the light is not coming from the upper left:\n%s",
			litMean, shadedMean, companionVolumeText(grid))
	}
}

// TestCompanionVolumeFillsEveryPixelRow pins the row the sprite does not use: none. Every
// pixel row of every frame carries something at both rungs. The placement, the cost test
// and the sprite's own row count all read the sprite's rows off the render, so a frame
// whose top or bottom row was empty would be a frame one row shorter than the ladder
// thinks it asked for -- and the ear tips and the shadow are exactly the two things that
// could fall short of the cell's own edges.
func TestCompanionVolumeFillsEveryPixelRow(t *testing.T) {
	for _, size := range companionVolumeRungs() {
		for _, state := range companionStates() {
			for _, gaze := range companionGazes() {
				grid := companionVolumeGrid(state, 0, gaze, size)
				for py, row := range grid {
					empty := true
					for _, tone := range row {
						if tone != companionToneNone {
							empty = false
							break
						}
					}
					if empty {
						t.Errorf("state %s at %d rows leaves pixel row %d empty:\n%s",
							companionStateNames[state], size.rows, py, companionVolumeText(grid))
					}
				}
			}
		}
	}
}

// companionVolumeEyeBox is where one eye sits and how far the art can draw from it: the
// two eye centres of a pose, and the reach every eye mark fits inside. The tests below ask
// the implementation where the head is rather than repeating the head's own arithmetic,
// which is what keeps them about the drawing rather than about the numbers behind it.
func companionVolumeEyeBox(t *testing.T, state companionState, size companionVolumeSize) [2][2]int {
	t.Helper()
	scale := companionVolumeScale(size)
	pose := companionPoseFor(state, 0, companionGaze{}, 0, size)
	return companionEyeCentres(pose.head, size, scale)
}

// companionVolumeCountAround counts the tones of one kind inside the box an eye can draw
// in, which is how the tests tell a disc from a pupil from the one pixel of glint.
func companionVolumeCountAround(grid [][]companionTone, centre [2]int, tone companionTone, reach int) int {
	count := 0
	for dy := -reach; dy <= reach; dy++ {
		for dx := -reach; dx <= reach; dx++ {
			x, y := centre[0]+dx, centre[1]+dy
			if y < 0 || y >= len(grid) || x < 0 || x >= len(grid[y]) {
				continue
			}
			if grid[y][x] == tone {
				count++
			}
		}
	}
	return count
}

// TestCompanionVolumeEyeIsADiscAPupilAndAGlint pins the rung-sized sclera, pupil and
// highlight. A shut eye keeps its sclera and draws a face-tone lid across it; the highlight
// is absent only while shut. The glint's colour is the palette's loudest tone.
func TestCompanionVolumeEyeIsADiscAPupilAndAGlint(t *testing.T) {
	for _, size := range companionVolumeRungs() {
		for _, state := range companionStates() {
			grid := companionVolumeGrid(state, 0, companionGaze{}, size)
			mark := companionEyeMarkFor(state)
			shut := mark == companionEyeShut || mark == companionEyeArc || mark == companionEyeHalfShut
			for _, centre := range companionVolumeEyeBox(t, state, size) {
				disc, pupils, lids, glints := 0, 0, 0, 0
				for y := 0; y < size.rows; y++ {
					for x := 0; x < size.width; x++ {
						if !companionEyeBox(centre, size, x, y) {
							continue
						}
						switch grid[y][x] {
						case companionToneEye:
							disc++
						case companionTonePupil:
							pupils++
						case companionToneRamp2:
							lids++
						case companionToneGlint:
							glints++
						}
					}
				}

				if shut {
					wantGlints := 1
					if mark == companionEyeArc {
						wantGlints = 0
					}
					if (disc == 0 && size.eyeRows >= 4) || glints != wantGlints {
						t.Errorf("state %s at %d rows drew %d sclera pixels and %d glints with its eyes shut, want %d glints:\n%s",
							companionStateNames[state], size.rows, disc, glints, wantGlints, companionVolumeText(grid))
					}
					if lids == 0 {
						t.Errorf("state %s at %d rows shut its eyes with no face-tone eyelid at all:\n%s",
							companionStateNames[state], size.rows, companionVolumeText(grid))
					}
					continue
				}
				if disc == 0 {
					t.Errorf("state %s at %d rows has no light disc in one of its eyes:\n%s",
						companionStateNames[state], size.rows, companionVolumeText(grid))
				}
				if pupils == 0 {
					t.Errorf("state %s at %d rows has no pupil in one of its eyes:\n%s",
						companionStateNames[state], size.rows, companionVolumeText(grid))
				}
				if glints != 1 {
					t.Errorf("state %s at %d rows drew %d glints in one eye, want one:\n%s",
						companionStateNames[state], size.rows, glints, companionVolumeText(grid))
				}
			}

			// The glint is one pixel per eye and no more: the whole frame carries one per eye
			// that can glint, which is what makes it the loudest single thing on the screen
			// rather than a pattern.
			total := companionVolumeGridTones(grid)[companionToneGlint]
			want := 2
			if shut {
				want = 2
				if mark == companionEyeArc {
					want = 0
				}
			}
			if total != want {
				t.Errorf("state %s at %d rows carries %d glint pixels, want %d:\n%s",
					companionStateNames[state], size.rows, total, want, companionVolumeText(grid))
			}
		}
	}

	ink := companionInkFor(true)
	if ink.colour(companionToneGlint) != parseHexColour(string(Accent.Dark)) {
		t.Errorf("the glint is not drawn in the palette's accent, so the loudest tone is spent elsewhere")
	}
	for _, tone := range companionTones() {
		if tone == companionToneGlint {
			continue
		}
		if ink.colour(tone) == ink.colour(companionToneGlint) {
			t.Errorf("tone %s is drawn in the same colour as the glint, so the bright pixel is not the only one",
				companionToneNames[tone])
		}
	}
}

// TestCompanionVolumeEyePupilAdjacencyByRung pins the two-tier eye rule. At full size a 1x2
// slit travels through all three positions in a 3x4 sclera, with a complete ring at the centre
// and a sclera column on both sides at every stop; the small rung uses a 1x1 pupil with
// horizontal and vertical sclera adjacency in its 2x2 eye.
func TestCompanionVolumeEyePupilAdjacencyByRung(t *testing.T) {
	for _, size := range companionVolumeRungs() {
		if size.width == companionVolumeFullWidth && (size.eyeWidth != 3 || size.eyeRows != 4) {
			t.Fatalf("full-rung eye is %dx%d, want 3x4", size.eyeWidth, size.eyeRows)
		}
		for _, gazeY := range []int{-1, 0, 1} {
			gaze := companionGaze{Y: gazeY}
			if size.eyeRows >= 4 {
				wantStart := map[int]int{-1: 0, 0: 1, 1: 2}[gazeY]
				if got := companionPupilStartRow(size, gaze); got != wantStart {
					t.Errorf("vertical gaze %d places pupil at sclera row %d, want row %d", gazeY, got, wantStart)
				}
			}
			grid := companionVolumeGrid(companionIdleState, 0, gaze, size)
			pose := companionPoseFor(companionIdleState, 0, gaze, 0, size)
			for eye, centre := range companionEyeCentres(pose.head, size, companionVolumeScale(size)) {
				startRow := centre[1] - size.eyeRows/2 + companionPupilStartRow(size, gaze)
				pupilRows := 1
				if size.eyeRows >= 4 {
					pupilRows = 2
				}
				if size.eyeRows >= 4 {
					top, bottom := centre[1]-size.eyeRows/2, centre[1]+size.eyeRows/2-1
					if gazeY < 0 && startRow != top {
						t.Errorf("up gaze pupil starts at row %d, want top lid row %d", startRow, top)
					}
					if gazeY > 0 && startRow+pupilRows-1 != bottom {
						t.Errorf("down gaze pupil ends at row %d, want bottom lid row %d", startRow+pupilRows-1, bottom)
					}
				}
				for dy := 0; dy < pupilRows; dy++ {
					y, x := startRow+dy, centre[0]
					if grid[y][x] != companionTonePupil {
						t.Errorf("%d-row eye %d gaze %+v missing pupil at (%d,%d)", size.rows, eye, gaze, x, y)
					}
					if size.eyeRows >= 4 {
						for _, side := range []int{x - 1, x + 1} {
							if grid[y][side] != companionToneEye && grid[y][side] != companionToneGlint {
								t.Errorf("3x4 eye %d gaze %+v pupil lacks side sclera at (%d,%d)", eye, gaze, side, y)
							}
						}
					} else {
						if grid[y][x-1] != companionToneEye && grid[y][x-1] != companionToneGlint {
							t.Errorf("small-rung eye %d lacks horizontal sclera adjacency", eye)
						}
						vertical := y - 1
						if !companionEyeBox(centre, size, x, vertical) {
							vertical = y + 1
						}
						if !companionEyeBox(centre, size, x, vertical) || (grid[vertical][x] != companionToneEye && grid[vertical][x] != companionToneGlint) {
							t.Errorf("small-rung eye %d lacks vertical sclera adjacency", eye)
						}
					}
				}
				if size.eyeRows >= 4 && gazeY == 0 {
					for _, edgeY := range []int{startRow - 1, startRow + pupilRows} {
						if grid[edgeY][centre[0]] != companionToneEye && grid[edgeY][centre[0]] != companionToneGlint {
							t.Errorf("3x4 eye %d central gaze pupil lacks top/bottom ring at row %d", eye, edgeY)
						}
					}
				}
			}
		}
	}
}

// TestCompanionVolumeGazeTurnsTheHeadAndThePupils pins the volume model's head-turn rule:
// the pupils move in the direction looked at, while the head turn may also alter the fused
// silhouette and shading. Those changes must remain inside the creature's own rasterized
// box; a gaze is allowed to turn this volume, not to paint outside its body.
func TestCompanionVolumeGazeChangeStaysInsideTheSprite(t *testing.T) {
	size, ok := companionVolumeSizeFor(companionVolumeFullHeight)
	if !ok {
		t.Fatal("the full height is not a rung of the volume")
	}
	neutral := companionVolumeGrid(companionIdleState, 0, companionGaze{}, size)
	turned := companionVolumeGrid(companionIdleState, 0, companionGaze{X: 1}, size)
	moved := 0
	for py := range turned {
		for px := range turned[py] {
			if turned[py][px] == neutral[py][px] {
				continue
			}
			moved++
			scale := companionVolumeScale(size)
			x := companionWorldX(px, size, scale)
			y := companionWorldY(py, size, scale)
			boxHalfWidth := float64(size.width) / (2 * scale)
			boxBottom := companionWorldTop - float64(size.rows)/scale
			if math.Abs(x) > boxHalfWidth || y < boxBottom || y > companionWorldTop {
				t.Errorf("a gaze right changed pixel (%d, %d) outside the creature's world-space cell (x %.3f, y %.3f)",
					px, py, x, y)
			}
		}
	}
	if moved == 0 {
		t.Errorf("a gaze right changed nothing at all:\n%s", companionVolumeText(turned))
	}

	// Compare with the neutral pupils, not the head's world-space centre: turning the head
	// moves that centre too, so a fixed-centre comparison rejects a valid volumetric turn.
	pupilMean := func(looking companionGaze) float64 {
		grid := companionVolumeGrid(companionIdleState, 0, looking, size)
		sum, count := 0, 0
		for _, eye := range companionVolumeEyeBox(t, companionIdleState, size) {
			for dy := -2; dy <= 2; dy++ {
				for dx := -2; dx <= 2; dx++ {
					x, y := eye[0]+dx, eye[1]+dy
					if y >= 0 && y < len(grid) && x >= 0 && x < len(grid[y]) && grid[y][x] == companionTonePupil {
						sum, count = sum+x, count+1
					}
				}
			}
		}
		if count == 0 {
			t.Fatalf("a gaze %+v left no pupil to follow it:\n%s", looking, companionVolumeText(grid))
		}
		return float64(sum) / float64(count)
	}
	leftMean := pupilMean(companionGaze{X: -1})
	neutralMean := pupilMean(companionGaze{})
	rightMean := pupilMean(companionGaze{X: 1})
	if !(leftMean < neutralMean && neutralMean < rightMean) {
		t.Errorf("pupil mean columns left %.2f, neutral %.2f, right %.2f do not follow gaze direction", leftMean, neutralMean, rightMean)
	}
}

// TestCompanionEyeGazeIsolationAndBlinkPinsTheFaceContract checks the strict neutral-column
// contract: gaze changes only pupils inside the named eye boxes; the fixed highlight does not
// move; and a blink changes only the eyelid line while sclera remains visible.
func TestCompanionEyeGazeIsolationAndBlinkPinsTheFaceContract(t *testing.T) {
	size, _ := companionVolumeSizeFor(companionVolumeFullHeight)
	neutral := companionVolumeGrid(companionIdleState, 0, companionGaze{}, size)
	up := companionVolumeGrid(companionIdleState, 0, companionGaze{Y: -1}, size)
	centres := companionVolumeEyeBox(t, companionIdleState, size)
	allowed := func(x, y int) bool {
		for _, c := range centres {
			if companionEyeBox(c, size, x, y) {
				return true
			}
		}
		return false
	}
	for y := range neutral {
		for x := range neutral[y] {
			if neutral[y][x] == up[y][x] {
				continue
			}
			if !allowed(x, y) {
				t.Errorf("central gaze changed pixel (%d,%d) outside EYE BOX: %s -> %s", x, y, companionToneNames[neutral[y][x]], companionToneNames[up[y][x]])
			}
			if neutral[y][x] != companionTonePupil && up[y][x] != companionTonePupil {
				t.Errorf("central gaze changed non-pupil pixel (%d,%d): %s -> %s", x, y, companionToneNames[neutral[y][x]], companionToneNames[up[y][x]])
			}
		}
	}
	for i, c := range centres {
		// The highlight stays at the eye box's upper-left pixel across vertical pupil travel.
		highlightX, highlightY := c[0]-size.eyeWidth/2, c[1]-size.eyeRows/2
		if neutral[highlightY][highlightX] != companionToneGlint || up[highlightY][highlightX] != companionToneGlint {
			t.Errorf("eye %d highlight moved with gaze; fixed upper-left highlight not preserved", i)
		}
		for _, gaze := range []companionGaze{{X: -1, Y: -1}, {X: -1}, {X: -1, Y: 1}, {Y: -1}, {}, {Y: 1}, {X: 1, Y: -1}, {X: 1}, {X: 1, Y: 1}} {
			grid := companionVolumeGrid(companionIdleState, 0, gaze, size)
			pose := companionPoseFor(companionIdleState, 0, gaze, 0, size)
			for _, eyeCentre := range companionEyeCentres(pose.head, size, companionVolumeScale(size)) {
				start := eyeCentre[1] - size.eyeRows/2 + companionPupilStartRow(size, gaze)
				rows := 1
				if size.eyeRows >= 4 {
					rows = 2
				}
				for dy := 0; dy < rows; dy++ {
					if grid[start+dy][eyeCentre[0]] != companionTonePupil {
						t.Errorf("gaze %+v lost the pupil at (%d,%d)", gaze, eyeCentre[0], start+dy)
					}
				}
			}
		}
	}

	blink := companionVolumeGrid(companionBlinkingState, 0, companionGaze{}, size)
	changed := 0
	for y := range neutral {
		for x := range neutral[y] {
			if neutral[y][x] == blink[y][x] {
				continue
			}
			changed++
			lidRow := false
			for _, c := range centres {
				lidRow = lidRow || (y == c[1] && x >= c[0]-1 && x <= c[0]+1)
			}
			if !allowed(x, y) || !lidRow {
				t.Errorf("blink changed pixel (%d,%d) outside the eyelid line in EYE BOX", x, y)
			}
		}
	}
	if changed == 0 {
		t.Fatal("blink changed no eyelid pixels")
	}
	for _, c := range centres {
		if blink[c[1]][c[0]] != companionToneRamp2 {
			t.Errorf("blink did not draw a face-tone eyelid line through eye centre %v", c)
		}
		topX, topY := c[0]-size.eyeWidth/2, c[1]-size.eyeRows/2
		bottomX, bottomY := c[0]+size.eyeWidth/2, c[1]+size.eyeRows/2-1
		if blink[topY][topX] != neutral[topY][topX] || blink[bottomY][bottomX] != companionToneEye {
			t.Errorf("blink cleared the eye or moved its fixed highlight at %v", c)
		}
	}
}

// TestCompanionVolumeGazeTurnsTheHeadAndThePupils pins extreme gaze motion and its bounds.
func TestCompanionVolumeGazeTurnsTheHeadAndThePupils(t *testing.T) {
	size, _ := companionVolumeSizeFor(companionVolumeFullHeight)
	neutral := companionVolumeGrid(companionIdleState, 0, companionGaze{}, size)
	turned := companionVolumeGrid(companionIdleState, 0, companionGaze{X: 1}, size)
	if fmt.Sprint(neutral) == fmt.Sprint(turned) {
		t.Fatal("extreme gaze changed no pixels")
	}
	baselinePose := companionPoseFor(companionIdleState, 0, companionGaze{}, 0, size)
	turnedPose := companionPoseFor(companionIdleState, 0, companionGaze{X: 1}, 0, size)
	if math.Abs((turnedPose.head.x-baselinePose.head.x)*companionVolumeScale(size)-1) > 1e-9 {
		t.Errorf("extreme gaze shifted skull by %.3f pixels, want one", (turnedPose.head.x-baselinePose.head.x)*companionVolumeScale(size))
	}
	if turnedPose.ears[0].points[1][0] == baselinePose.ears[0].points[1][0] {
		t.Error("extreme gaze did not tilt the ears")
	}
	scale := companionVolumeScale(size)
	baselineEyes := companionEyeCentres(baselinePose.head, size, scale)
	turnedEyes := companionEyeCentres(turnedPose.head, size, scale)
	for y, row := range turned {
		for x, tone := range row {
			if tone == neutral[y][x] {
				continue
			}
			inEyeBox := false
			for _, centre := range append(baselineEyes[:], turnedEyes[:]...) {
				inEyeBox = inEyeBox || companionEyeBox(centre, size, x, y)
			}
			if !inEyeBox && !companionHeadOutlineBand(baselinePose, size, scale, x, y) && !companionHeadOutlineBand(turnedPose, size, scale, x, y) {
				t.Errorf("extreme gaze changed (%d,%d) outside EYE BOX and HEAD OUTLINE BAND: %s -> %s", x, y, companionToneNames[neutral[y][x]], companionToneNames[tone])
			}
		}
	}
	for name, grid := range map[string][][]companionTone{"neutral": neutral, "extreme": turned} {
		if len(grid) != size.rows {
			t.Errorf("%s gaze raster has %d rows, reserved block has %d", name, len(grid), size.rows)
		}
		for y, row := range grid {
			if len(row) != size.width {
				t.Errorf("%s gaze row %d has %d pixels, reserved block has %d", name, y, len(row), size.width)
			}
		}
	}
}

// TestCompanionVolumeGaitMovesThePawsAndCounterSwaysTheTail pins the walk where it is
// decided: in the pose. The four poses are a stride -- the front paw reaches forward and
// back, the tail's tip goes the other way, the body rides up over the passing poses and the
// paws stay on the ground while it does -- and the drawing has to show all of it, which the
// two contact poses' own pixels are checked for. A walk whose legs swapped places without
// the paws moving would pass a "the frame changed" test and fail this one.
func TestCompanionVolumeGaitMovesThePawsAndCounterSwaysTheTail(t *testing.T) {
	size, ok := companionVolumeSizeFor(companionVolumeFullHeight)
	if !ok {
		t.Fatal("the full height is not a rung of the volume")
	}
	scale := companionVolumeScale(size)
	standing := companionPoseFor(companionIdleState, 0, companionGaze{}, 0, size)

	var poses [4]companionPose
	for phase := range poses {
		poses[phase] = companionPoseFor(companionWalkingState, phase*companionGaitTicks, companionGaze{}, 0, size)
	}

	for _, phase := range []int{0, 2} {
		paw := poses[phase].legs[0].x1 - standing.legs[0].x1
		tail := poses[phase].tail[2].x1 - standing.tail[2].x1
		if paw == 0 {
			t.Errorf("pose %d left the front paw where it stands", phase)
		}
		if paw*tail >= 0 {
			t.Errorf("pose %d moved the front paw by %.3f and the tail's tip by %.3f, so the tail is not counter-swaying",
				phase, paw, tail)
		}
	}

	// The body rides up one pixel over the passing poses and not at all otherwise, and the
	// paws alternate in diagonal pairs: two on the ground and two off it at every pose,
	// which is a trot rather than a glide or a hop.
	for phase, pose := range poses {
		bob := pose.body.y - standing.body.y
		want := float64(companionBodyBobPx) * companionGaitBob[phase] / scale
		if math.Abs(bob-want) > 1e-9 {
			t.Errorf("pose %d bobbed the body by %.4f, want %.4f", phase, bob, want)
		}
		lifted := 0
		for i, leg := range pose.legs {
			switch {
			case leg.y1 > standing.legs[i].y1+1e-9:
				lifted++
			case leg.y1 < standing.legs[i].y1-1e-9:
				t.Errorf("pose %d pushed paw %d through the ground", phase, i)
			}
		}
		if lifted != 2 {
			t.Errorf("pose %d has %d paws off the ground, want the two of one diagonal pair", phase, lifted)
		}
	}

	// And the drawing shows it: the two contact poses are different frames.
	first := companionVolumeText(companionVolumeGrid(companionWalkingState, 0, companionGaze{}, size))
	second := companionVolumeText(companionVolumeGrid(companionWalkingState, 2*companionGaitTicks, companionGaze{}, size))
	if first == second {
		t.Errorf("the two contact poses of the walk draw the same frame, so the legs are not stepping:\n%s", first)
	}
}

// TestCompanionVolumeStatesAreDistinct pins the state set at the rung that has room for a
// face: every state draws a different frame at the same tick and the same gaze. It is the
// volume's version of the glyph art's "the state reads from the glyphs" rule, and it is
// what a state added without an eye mark or a pose of its own would fail.
func TestCompanionVolumeStatesAreDistinct(t *testing.T) {
	size, ok := companionVolumeSizeFor(companionVolumeFullHeight)
	if !ok {
		t.Fatal("the full height is not a rung of the volume")
	}
	drawn := map[string]companionState{}
	for _, state := range companionStates() {
		frame := companionVolumeText(companionVolumeGrid(state, 0, companionGaze{}, size))
		if other, clash := drawn[frame]; clash {
			t.Errorf("states %s and %s draw the same frame:\n%s",
				companionStateNames[other], companionStateNames[state], frame)
		}
		drawn[frame] = state
	}
}

// TestCompanionStartleTremblesThenHolds pins the one animated reaction's bound. A flinching
// creature shivers for companionShiverTicks frames after the last key and then holds the
// crouched pose, so an error screen nobody is touching settles into bytes the renderer stops
// writing. A tremble that ran for as long as the error did would repaint twelve rows eight
// times a second over a screen the user may be reading.
func TestCompanionStartleTremblesThenHolds(t *testing.T) {
	m := NewModel()
	isolateGoldenTest(t, &m)
	m.Screen = ScreenError
	m.ErrorMsg = "the step failed"
	m.Width, m.Height = 160, 50
	m.Animating, m.PixelSprite = true, true

	// The key that reached the error reset the idle counter, so the first frames shiver.
	resting := m.AnimTick % 2
	if m.companionShiverPx() == 0 {
		t.Fatalf("a just-startled creature is not trembling")
	}
	if m.View() == companionTick(t, m).View() {
		t.Errorf("the tremble did not reach the drawing")
	}

	// Once the stretch is over the pose holds: the same view from tick to tick.
	settled := companionTicks(t, m, companionShiverTicks)
	if settled.companionShiverPx() != 0 {
		t.Errorf("the creature is still trembling %d frames after the key", companionShiverTicks)
	}
	before := settled.View()
	after := companionTicks(t, settled, 8).View()
	if rows, bytes := companionChangedRows(before, after); len(rows) != 0 || bytes != 0 {
		t.Errorf("a settled flinch changed %d rows and %d bytes, want none", len(rows), bytes)
	}
	if m.AnimTick%2 == resting {
		t.Logf("the startle trembles for %d frames and then holds the crouched pose", companionShiverTicks)
	}
}

// TestCompanionVolumeSpriteIsTheLadderTopSteps pins the rung to terminal height,
// never to the rows a screen happened to leave: volume-full, volume-small, glyph
// cat, then compact glyph art. A terminal that cannot use volume falls through to
// the glyph rung, and the compact floor remains drawable without colour.
func TestCompanionVolumeSpriteIsTheLadderTopSteps(t *testing.T) {
	tests := []struct {
		name       string
		height     int
		sprite     bool
		wantHeight int
	}{
		{"height 34 selects volume-full", 34, true, companionVolumeFullHeight},
		{"height 33 selects volume-small", 33, true, companionVolumeSmallHeight},
		{"height 30 selects volume-small", 30, true, companionVolumeSmallHeight},
		{"height 29 falls through to glyph cat", 29, true, companionFullHeight},
		{"height 25 selects glyph cat", 25, false, companionFullHeight},
		{"gate off selects glyph cat", 40, false, companionFullHeight},
		{"height 24 selects compact glyph art", 24, true, companionCompactHeight},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{Width: 160, Height: tt.height, Animating: true, PixelSprite: tt.sprite, ink: companionInkFor(true)}
			wantRung := m.companionHeightNow()
			if wantRung != tt.wantHeight {
				t.Fatalf("terminal height %d selected rung %d, want %d", tt.height, wantRung, tt.wantHeight)
			}
			t.Logf("terminal height %d with pixel=%t selects a %d-row rung", tt.height, tt.sprite, wantRung)
			// Give placement more than enough blank rows: only the terminal height
			// and sprite mode may determine which art is drawn.
			placed := make([]string, tt.wantHeight+2)
			got := m.placeCompanion(placed, nil, companionStageWidth(m))
			if len(got) != len(placed) {
				t.Fatalf("placement drew %d rows on a %d-row frame", len(got), len(placed))
			}
			sprite := got[len(got)-tt.wantHeight:]
			blocks := 0
			for _, row := range sprite {
				if strings.ContainsAny(ansiEscape.ReplaceAllString(row, ""), "\u2580\u2584\u2588") {
					blocks++
				}
			}
			_, volume := companionVolumeSizeFor(tt.wantHeight)
			if volume && blocks != tt.wantHeight {
				t.Errorf("the sprite draws %d rows of half blocks, want %d", blocks, tt.wantHeight)
			}
			if !volume && blocks != 0 {
				t.Errorf("the sprite drew %d rows of half blocks, want the glyph art", blocks)
			}
		})
	}
}

// TestCompanionVolumeIsRefusedWithoutTrueColour pins the floor of the ladder as a decision
// rather than as a comment: the volume is drawn only where the terminal reports true colour,
// and a terminal with sixteen colours, eight or none is refused it -- sixteen colours cannot
// draw a five-step ramp, so the tier would arrive as five tones of whatever the terminal maps
// them to, which is a smear rather than a volume. Those terminals draw the glyph cat, which
// is why the refusal is a ladder step and not a lost companion.
func TestCompanionVolumeIsRefusedWithoutTrueColour(t *testing.T) {
	tests := []struct {
		name     string
		profile  termenv.Profile
		spriteOn string
		wanted   bool
	}{
		{"true colour draws the volume", termenv.TrueColor, "", true},
		{"DOTFILES_SPRITE=0 refuses it even in true colour", termenv.TrueColor, "0", false},
		{"a 256-colour terminal is refused", termenv.ANSI256, "", false},
		{"a 16-colour terminal is refused", termenv.ANSI, "", false},
		{"a terminal with no colour at all is refused", termenv.Ascii, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pixelSpriteAllowed(tt.profile, tt.spriteOn); got != tt.wanted {
				t.Errorf("pixelSpriteAllowed(%v, %q) = %v, want %v", tt.profile, tt.spriteOn, got, tt.wanted)
			}
		})
	}
}

// TestCompanionVolumeSpriteIsDeterministic pins the promise the glyph sprite keeps, in
// pixels: the same model, tick and gaze render the same bytes, the render is pure so
// calling it twice changes nothing, and the sprite a snapshot would pin is the full rung's
// twelve rows.
func TestCompanionVolumeSpriteIsDeterministic(t *testing.T) {
	build := func() Model {
		m := NewModel()
		isolateGoldenTest(t, &m)
		m.Screen = ScreenMainMenu
		m.Width, m.Height = 160, 50
		m.Animating, m.PixelSprite = true, true
		return m
	}

	first, second := build(), build()
	first = companionTicks(t, first, 3)
	second = companionTicks(t, second, 3)

	if first.View() != second.View() {
		t.Errorf("two models advanced by three ticks rendered different bytes with the volume sprite")
	}
	if first.View() != first.View() {
		t.Errorf("rendering the same model twice produced different bytes, so the render is not pure")
	}

	viewRows := strings.Split(first.View(), "\n")
	rows := companionOwnedRows(viewRows)
	if len(rows) != companionVolumeFullHeight {
		t.Errorf("the volume sprite owns %d rows, want %d", len(rows), companionVolumeFullHeight)
	}
	for _, i := range rows {
		if !strings.ContainsAny(plainRow(viewRows[i]), "\u2580\u2584\u2588") {
			t.Errorf("row %d of the volume sprite carries no half blocks: %q", i, plainRow(viewRows[i]))
		}
	}
}

// TestCompanionVolumeFrameBytesAreBounded declares the cost of the tier the way the
// repository declares costs: with a test rather than a comment. A frame of the full sprite
// is twelve rows of thirty-two cells, and the encoder writes an escape sequence only when a
// cell's style changes, so a whole frame fits in a few thousand bytes. The bound is generous
// on purpose: it is there to catch a change that starts emitting a sequence per cell, not to
// pin the exact number, which the test logs instead.
func TestCompanionVolumeFrameBytesAreBounded(t *testing.T) {
	const bound = 8192

	m := Model{Width: 160, Height: 50, Animating: true, PixelSprite: true, ink: companionInkFor(true)}
	m.Screen = ScreenMainMenu
	m.CompanionPos = 40

	biggest := 0
	for _, size := range companionVolumeRungs() {
		for _, state := range companionStates() {
			for _, gaze := range companionGazes() {
				grid := companionVolumeGrid(state, 0, gaze, size)
				total := 0
				for py := 0; py+1 < size.rows; py += 2 {
					total += len(m.ink.row(grid[py], grid[py+1]))
				}
				biggest = max(biggest, total)
				if total > bound {
					t.Errorf("state %s at %d rows and gaze %+v writes %d bytes in one frame, want at most %d",
						companionStateNames[state], size.rows, gaze, total, bound)
				}
			}
		}
	}
	t.Logf("the volume sprite writes at most %d bytes per frame (%d rows of %d cells)",
		biggest, companionVolumeFullHeight, companionVolumeFullWidth)
}

// TestCompanionVolumeReadsAsACreature pins the layout the eye reads the drawing by, which is
// the one thing about "it looks like a creature" that can be asserted rather than looked at:
// the body fills the cell it was given, the eyes are in the head -- above and in front of the
// body's own mass -- and the nose is in front of the eyes, on the muzzle. It also writes the
// two frames it checked into the log, because a frame of this sprite is colours and a reader
// of a test log is not, and the log is where a reviewer can see what is being drawn.
func TestCompanionVolumeReadsAsACreature(t *testing.T) {
	size, ok := companionVolumeSizeFor(companionVolumeFullHeight)
	if !ok {
		t.Fatal("the full height is not a rung of the volume")
	}
	grid := companionVolumeGrid(companionIdleState, 0, companionGaze{}, size)

	leftmost, rightmost := size.width, -1
	bodySum, bodyCount := 0, 0
	for py, row := range grid {
		for px, tone := range row {
			if tone == companionToneNone {
				continue
			}
			leftmost, rightmost = min(leftmost, px), max(rightmost, px)
			bodySum, bodyCount = bodySum+py, bodyCount+1
		}
	}
	if bodyCount == 0 {
		t.Fatal("the idle frame draws nothing at all")
	}
	if leftmost > size.width/4 || rightmost < 3*size.width/4 {
		t.Errorf("the creature spans columns %d to %d of %d, so it does not fill its own cell:\n%s",
			leftmost, rightmost, size.width, companionVolumeText(grid))
	}

	bodyMeanRow := float64(bodySum) / float64(bodyCount)
	for _, eye := range companionVolumeEyeBox(t, companionIdleState, size) {
		if float64(eye[1]) >= bodyMeanRow {
			t.Errorf("an eye sits at pixel row %d and the body's own mass is at %.1f, so the eyes are not in a head above it:\n%s",
				eye[1], bodyMeanRow, companionVolumeText(grid))
		}
	}

	glintSum, glintCount, noseSum, noseCount := 0, 0, 0, 0
	for _, row := range grid {
		for px, tone := range row {
			switch tone {
			case companionToneGlint:
				glintSum, glintCount = glintSum+px, glintCount+1
			case companionToneNose:
				noseSum, noseCount = noseSum+px, noseCount+1
			}
		}
	}
	if glintCount == 0 || noseCount == 0 {
		t.Fatalf("the idle frame has no face at all:\n%s", companionVolumeText(grid))
	}
	if noseSum/noseCount <= glintSum/glintCount {
		t.Errorf("the nose is at column %d and the eyes at %d, so the muzzle is not in front of them:\n%s",
			noseSum/noseCount, glintSum/glintCount, companionVolumeText(grid))
	}

	t.Logf("the full sprite at rest, ramp dark to light and the marks as letters:\n%s", companionVolumeText(grid))
	t.Logf("the same creature one step into its walk, looking right:\n%s",
		companionVolumeText(companionVolumeGrid(companionWalkingState, 2*companionGaitTicks, companionGaze{X: 1}, size)))

	if small, ok := companionVolumeSizeFor(companionVolumeSmallHeight); ok {
		t.Logf("the small sprite, which is the same field on fewer rows and columns:\n%s",
			companionVolumeText(companionVolumeGrid(companionIdleState, 0, companionGaze{X: 1}, small)))
	}
}

// BenchmarkCompanionVolumeFrame measures what drawing the volume costs in the render path:
// one View of a walking model with a live pointer, which is the worst case a live run
// reaches.
func BenchmarkCompanionVolumeFrame(b *testing.B) {
	m := Model{Width: 160, Height: 50, Animating: true, PixelSprite: true, Hovering: true, ink: companionInkFor(true)}
	m.Screen = ScreenMainMenu
	m.CompanionPos = 40

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		m.AnimTick = i
		_ = m.View()
	}
}

// BenchmarkCompanionGlyphFrame is the same frame drawn with the glyph cat, so the
// shaded sprite's share of the render can be read off the two benchmarks together:
// the difference between them is what the extra tier costs a live run.
func BenchmarkCompanionGlyphFrame(b *testing.B) {
	m := Model{Width: 160, Height: 50, Animating: true, Hovering: true, ink: companionInkFor(true)}
	m.Screen = ScreenMainMenu
	m.CompanionPos = 40

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		m.AnimTick = i
		_ = m.View()
	}
}

// TestCompanionStaysPutWhenNothingHappens pins the rest regime of the creature's
// cost: with no key and no pointer, nothing about the frame changes at all. Not
// "the creature does not move" but "the bytes do not change", which is what makes
// the renderer write nothing -- it repaints a line only when a byte in it changed,
// so a view identical from tick to tick is a terminal that is not written to. The
// creature used to stroll on its own and settle its gaze a tick after the target
// did, so two of its rows changed on every frame and the screen flickered; neither
// happens now. The horizon is shorter than companionBlinkTicks on purpose: a blink
// is a deliberate decoration with its own interval, and the regime this pins is the
// one between blinks.
func TestCompanionStaysPutWhenNothingHappens(t *testing.T) {
	fixtures := []companionScreenFixture{
		{name: "main menu 160x50", screen: ScreenMainMenu, width: 160, height: 50, framed: true},
		{name: "main menu 227x62", screen: ScreenMainMenu, width: 227, height: 62, framed: true},
		{name: "welcome 160x50", screen: ScreenWelcome, width: 160, height: 50, framed: true},
	}

	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			m := NewModel()
			isolateGoldenTest(t, &m)
			m.Screen = f.screen
			m.Width, m.Height = f.width, f.height
			m.Animating, m.PixelSprite = true, true

			before := m.View()
			after := companionTicks(t, m, 24).View()
			if rows, bytes := companionChangedRows(before, after); len(rows) != 0 || bytes != 0 {
				t.Errorf("twenty-four ticks with no input changed %d rows and %d bytes, want none:\nbefore:\n%s\nafter:\n%s",
					len(rows), bytes, before, after)
			}
		})
	}
}

// TestCompanionLightTerms pins the tone ramp and both spatial lighting terms on the
// rendered field. Contact pixels are checked against the named world-coordinate regions.
func TestCompanionLightTerms(t *testing.T) {
	size, ok := companionVolumeSizeFor(companionVolumeFullHeight)
	if !ok {
		t.Fatal("full volume size unavailable")
	}
	grid := companionVolumeTones(companionIdleState, 0, companionGaze{}, 0, size)
	counts := make([]int, companionRampSteps)
	mask := func(x, y int) bool {
		return y >= 0 && y < size.rows && x >= 0 && x < size.width && grid[y][x] != companionToneNone
	}
	contactDark, contactTerm, upperLeftRim, upperLeftRimEffects, lowerRightRim := 0, 0, 0, 0, 0
	pose := companionPoseFor(companionIdleState, 0, companionGaze{}, 0, size)
	heights := pose.heightField(size, companionVolumeScale(size))
	for y, row := range grid {
		for x, tone := range row {
			if tone >= companionToneRamp0 && tone <= companionToneRamp4 {
				counts[tone]++
			}
			wx, wy := companionWorldX(x, size, companionVolumeScale(size)), companionWorldY(y, size, companionVolumeScale(size))
			if tone == companionToneRamp0 && companionContactRegion(wx, wy) {
				contactDark++
			}
			if heights[y+1][x+1] > 0 {
				lit := companionShadeToneWithTerms(heights, x, y, companionVolumeScale(size), companionRim, true)
				withoutContact := companionShadeToneWithTerms(heights, x, y, companionVolumeScale(size), companionRim, false)
				if companionContactRegion(wx, wy) && int(withoutContact)-int(lit) == 2 {
					contactTerm++
				}
				boundary := !mask(x-1, y) || !mask(x+1, y) || !mask(x, y-1) || !mask(x, y+1)
				withoutRim := companionShadeToneWithTerms(heights, x, y, companionVolumeScale(size), 0, true)
				if boundary && x < size.width/2 && y < size.rows/2 && lit > withoutRim {
					upperLeftRimEffects++
				}
				if boundary && x < size.width/2 && y < size.rows/2 && lit == companionToneRamp4 && withoutRim < lit {
					upperLeftRim++
				}
				if boundary && x >= size.width/2 && y >= size.rows/2 && lit > withoutRim {
					lowerRightRim++
				}
			}
		}
	}
	seen := 0
	for _, count := range counts {
		if count > 0 {
			seen++
		}
	}
	if seen != companionRampSteps {
		t.Errorf("rendered tone histogram %v contains %d distinct ramp tones, want encoder ramp count %d", counts, seen, companionRampSteps)
	}
	if contactDark == 0 {
		t.Errorf("darkest ramp tone has no pixels in documented contact regions")
	}
	if contactTerm == 0 {
		t.Errorf("AO has no two-tone contact pixels")
	}
	if upperLeftRim == 0 {
		t.Errorf("rim tone is absent from the upper-left boundary")
	}
	if upperLeftRimEffects < 2 {
		t.Errorf("rim affects %d upper-left boundary pixels, want at least two", upperLeftRimEffects)
	}
	if lowerRightRim != 0 {
		t.Errorf("rim tone appears %d times on the lower-right boundary, want none", lowerRightRim)
	}
	t.Logf("encoder ramp histogram: %v (%d distinct tones)", counts, seen)
}

// TestCompanionBlockDependsOnlyOnTheTerminal guards the companion's reserved
// block against content-driven sizing. Every framed screen and each of the six
// trainer screens is rendered in two content states at each size, in both pixel
// and glyph modes. At the 80x24 floor the menu and trainer intentionally retain
// their separately pinned legacy blocks; above it every screen must use the
// mode's terminal-height rung at the same row position.
func TestCompanionBlockDependsOnlyOnTheTerminal(t *testing.T) {
	sizes := []struct {
		width, height int
	}{{80, 24}, {120, 40}, {160, 50}, {227, 62}}

	for _, size := range sizes {
		for _, pixel := range []bool{false, true} {
			positions := map[string]int{}
			for _, name := range installerFrameScreenNames {
				for state := 0; state < 2; state++ {
					m := installerFrameCase(t, name)
					m.Width, m.Height = size.width, size.height
					m.Animating, m.PixelSprite = true, pixel
					if options := m.GetCurrentOptions(); len(options) > 1 {
						m.Cursor = state
					}
					assertCompanionBlockContract(t, positions, fmt.Sprintf("%s/state-%d", name, state), m, size.height, pixel, true)
				}
			}
			for _, name := range trainerLeakScreenNames {
				for state := 0; state < 2; state++ {
					m := trainerLeakFrameCase(t, name)
					m.Width, m.Height = size.width, size.height
					m.Animating, m.PixelSprite = true, pixel
					if state == 1 {
						m.TrainerMessage = "Hint: use the lesson's suggested motion."
					}
					assertCompanionBlockContract(t, positions, fmt.Sprintf("%s/state-%d", name, state), m, size.height, pixel, false)
				}
			}
		}
	}
}

func assertCompanionBlockContract(t *testing.T, positions map[string]int, name string, m Model, height int, pixel, framed bool) {
	t.Helper()
	view := m.View()
	if height == trainerFloorHeight && name != "main-menu/state-0" && name != "main-menu/state-1" && !strings.HasPrefix(name, "trainer-lesson/") {
		return // The floor preserves each existing framed screen; only these two values are pinned.
	}
	art, rows := trainerViewCompanionArt(view)
	if rows == 0 {
		if framed {
			return // A framed screen whose unchanged body leaves no block does not show the creature.
		}
		t.Fatalf("%s at %dx%d pixel=%t has no companion block", name, m.Width, height, pixel)
	}
	first, _, ok := findCompanionRow(view)
	if !ok {
		t.Fatalf("%s at %dx%d pixel=%t has no companion row", name, m.Width, height, pixel)
	}
	if height == trainerFloorHeight {
		want := companionCompactHeight
		if !framed {
			want = companionMiniHeight
		}
		if rows != want {
			t.Errorf("%s at 80x24 has %d art rows, want pinned floor block %d:\n%s", name, rows, want, art)
		}
		return
	}

	want := companionFullHeight
	if pixel {
		switch {
		case height >= 34:
			want = companionVolumeFullHeight
		case height >= 30:
			want = companionVolumeSmallHeight
		default:
			want = companionFullHeight
		}
	} else if height < 25 {
		want = companionCompactHeight
	}
	if rows != want {
		t.Errorf("%s at %dx%d pixel=%t has %d art rows, want terminal rung %d:\n%s", name, m.Width, height, pixel, rows, want, art)
	}
	key := fmt.Sprintf("%dx%d/pixel=%t/%s", m.Width, height, pixel, strings.Split(name, "/")[0])
	if prior, ok := positions[key]; ok && first != prior {
		t.Errorf("%s at %dx%d pixel=%t starts companion at row %d, want same-screen row %d in the alternate content state", name, m.Width, height, pixel, first, prior)
	} else {
		positions[key] = first
	}
}

// TestCompanionCostHasTwoRegimes measures the two costs the creature has. The
// number a comment used to carry was the sprite's own bytes, which is not what the
// terminal is written: the renderer repaints a whole line whenever any byte in it
// changed, so a walking tick costs the sprite's row count times the line width. At
// rest the view is byte-identical and the renderer writes nothing. The walking
// figure is logged rather than asserted, because the terminal's width and the
// sprite's height set it and a constant here could not defend itself; what is
// asserted is the shape -- a walking tick changes the sprite's rows and only those,
// and a walk writes something on the frames it moves.
func TestCompanionCostHasTwoRegimes(t *testing.T) {
	m := NewModel()
	isolateGoldenTest(t, &m)
	m.Screen = ScreenMainMenu
	m.Width, m.Height = 227, 62
	m.Animating, m.PixelSprite = true, true

	// At rest: nothing changes at all.
	before := m.View()
	after := companionTicks(t, m, 24).View()
	if rows, bytes := companionChangedRows(before, after); len(rows) != 0 || bytes != 0 {
		t.Errorf("a resting creature changed %d rows and %d bytes, want none", len(rows), bytes)
	}

	// While walking: the cursor moves one row and the creature walks to it.
	options := m.GetCurrentOptions()
	m.Cursor = len(options) - 1
	m.armCompanionFollow()

	frames, moves, total, widest, lines, spriteLines := 0, 0, 0, 0, 0, 0
	for tick := 0; tick < 200 && (m.CompanionFollow || m.CompanionMoving); tick++ {
		next := companionTick(t, m)
		rows, bytes := companionChangedRows(m.View(), next.View())
		frames++
		if len(rows) > 0 {
			owned := companionOwnedRows(strings.Split(next.View(), "\n"))
			ownedRows := make(map[int]struct{}, len(owned))
			spriteLines = max(spriteLines, len(owned))
			for _, row := range owned {
				ownedRows[row] = struct{}{}
			}
			for _, row := range rows {
				if _, ok := ownedRows[row]; !ok {
					t.Fatalf("a walking tick changed row %d, which the sprite does not own", row)
				}
			}
			moves++
			total += bytes
			widest = max(widest, bytes)
			lines = max(lines, len(rows))
		}
		m = next
	}
	if moves == 0 {
		t.Fatalf("the armed walk wrote no frame")
	}
	t.Logf("walking at %d columns: %d sprite rows, up to %d changed lines per moving tick; %d moving frames out of %d (%.2f s), %d bytes total, %d bytes in the widest changed lines (%.1f KB/s at %d fps); rest writes 0",
		m.Width, spriteLines, lines, moves, frames, float64(frames)/animTicksPerSecond, total, widest, float64(widest)*animTicksPerSecond/1024, animTicksPerSecond)
}

// TestCompanionFollowStepIsBoundedByFollowCells pins the walk's distance rule: one
// row of a menu is companionFollowCells cells of walking and no more. The creature
// used to spread the whole menu index across the stage, so one arrow key on a wide
// terminal threw it tens of cells; a fixed number per row makes one keypress a
// short stroll, and the cap at the right edge only ever makes a step shorter.
func TestCompanionFollowStepIsBoundedByFollowCells(t *testing.T) {
	fixtures := []companionScreenFixture{
		{name: "main menu 160x50", screen: ScreenMainMenu, width: 160, height: 50},
		{name: "main menu 227x62", screen: ScreenMainMenu, width: 227, height: 62},
		{name: "os select 80x24", screen: ScreenOSSelect, width: 80, height: 24},
		{name: "restore confirm 100x30", screen: ScreenRestoreConfirm, width: 100, height: 30, backups: true},
	}

	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			m := NewModel()
			isolateGoldenTest(t, &m)
			m.Screen = f.screen
			m.Width, m.Height = f.width, f.height
			if f.backups {
				m.AvailableBackups = []system.BackupInfo{testBackupInfo()}
			}
			stage := companionStageWidth(m)
			options := m.GetCurrentOptions()
			if len(options) < 2 {
				t.Fatalf("the screen offers %d options, want at least two", len(options))
			}

			previous := -1
			for i := range options {
				m.Cursor = i
				cell, ok := m.companionTarget(stage)
				if !ok {
					t.Fatalf("option %d has no target cell", i)
				}
				if previous >= 0 {
					if step := companionDistance(previous, cell); step > companionFollowCells {
						t.Errorf("one menu row moved the target %d cells, want at most %d", step, companionFollowCells)
					}
				}
				previous = cell
			}
		})
	}
}
