package tui

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/albersg/dotfiles/installer/internal/system"
	"github.com/albersg/dotfiles/installer/internal/tui/trainer"

	tea "github.com/charmbracelet/bubbletea"
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
// frames, stripped of styling: the art is what the eye reads, so the art is what
// a test looks for.
func companionRowHasArt(row string) bool {
	plain := ansiEscape.ReplaceAllString(row, "")
	for _, frame := range companionFrames {
		if strings.Contains(plain, frame.art) {
			return true
		}
	}
	return false
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

// TestCompanionArtIsOneRowOfPrintableASCII pins the drawing rules the art was
// chosen for. Every frame is one row of printable ASCII, so Termux, a 16-colour
// terminal and a terminal whose font lacks the glyphs all draw the creature; no
// frame is wider than the cell it is placed in, so a one-cell step is always one
// column; and the walk has two frames that differ, because a walk with one frame
// is a slide.
func TestCompanionArtIsOneRowOfPrintableASCII(t *testing.T) {
	states := map[companionState][]string{}
	for _, frame := range companionFrames {
		if frame.art == "" {
			t.Errorf("state %d has an empty frame", frame.state)
			continue
		}
		for _, r := range frame.art {
			if r > 0x7E || r < 0x20 {
				t.Errorf("frame %q of state %d carries %q, which is not printable ASCII",
					frame.art, frame.state, r)
			}
		}
		if width := len(frame.art); width > companionCellWidth {
			t.Errorf("frame %q is %d columns wide, want at most the cell's %d",
				frame.art, width, companionCellWidth)
		}
		if strings.ContainsAny(frame.art, "\n\t") {
			t.Errorf("frame %q is not one row", frame.art)
		}
		states[frame.state] = append(states[frame.state], frame.art)
	}

	for _, state := range []companionState{
		companionIdleState, companionWalkingState, companionAsleepState,
		companionAlertState, companionPleasedState, companionFlinchingState,
	} {
		if len(states[state]) == 0 {
			t.Errorf("state %d draws nothing at all", state)
		}
		if got := companionArtFor(state, 0); got != states[state][0] {
			t.Errorf("state %d draws %q at tick 0, want its first frame %q", state, got, states[state][0])
		}
	}

	if len(states[companionWalkingState]) < 2 {
		t.Fatalf("the walk has %d frames, want at least 2 to alternate",
			len(states[companionWalkingState]))
	}
	// The walk cycles through its frames on the tick's parity, so the art changes
	// on every tick it walks and returns to the first frame after a full cycle.
	walk := states[companionWalkingState]
	for tick := 0; tick < 2*len(walk); tick++ {
		want := walk[tick%len(walk)]
		if got := companionArtFor(companionWalkingState, tick); got != want {
			t.Errorf("the walk draws %q at tick %d, want %q", got, tick, want)
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.model.companionStateNow(); got != tt.want {
				t.Errorf("state = %d, want %d", got, tt.want)
			}
			// The state has to reach the pixels: the rendered row carries the art
			// of the state the model is in, and no colour is needed to read it.
			row := tt.model.companionRow(80)
			if row == "" {
				t.Fatalf("the model draws no companion row at all")
			}
			want := companionArtFor(tt.want, tt.model.AnimTick)
			if got := strings.TrimLeft(plainRow(row), " "); got != want {
				t.Errorf("the row draws %q, want %q", got, want)
			}
		})
	}
}

// TestCompanionStatesAreDistinctWithoutColour pins that nothing about the state
// depends on the tone: the strip of styling off each state's row is a different
// glyph run from every other state's, so a terminal with no colour loses no
// information.
func TestCompanionStatesAreDistinctWithoutColour(t *testing.T) {
	plain := map[string]companionState{}
	for _, frame := range companionFrames {
		// Two states may share a frame only if they are the same state: the walk's
		// two frames are the exception, and they differ from each other.
		if other, ok := plain[frame.art]; ok && other != frame.state {
			t.Errorf("states %d and %d draw the same frame %q", other, frame.state, frame.art)
		}
		plain[frame.art] = frame.state
	}

	// The idle, asleep, alert, pleased and flinch frames are five different
	// glyph runs: the eyes, the z and the ! are the state.
	distinct := map[string]companionState{
		"(o.o)":    companionIdleState,
		"(-.-) z":  companionAsleepState,
		"(O.O) !":  companionAlertState,
		"\\(o.o)/": companionPleasedState,
		"(>.<)":    companionFlinchingState,
	}
	for art, state := range distinct {
		if got := companionArtFor(state, 0); got != art {
			t.Errorf("state %d draws %q, want %q", state, got, art)
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

	// The celebration is a few ticks, not a mode: it settles by itself.
	settled := companionTicks(t, done, companionPleasedTicks)
	if settled.CompanionPleased != 0 {
		t.Errorf("CompanionPleased = %d after %d ticks, want 0",
			settled.CompanionPleased, companionPleasedTicks)
	}
	if !settled.CompanionMoving {
		t.Errorf("the companion stopped strolling after its celebration")
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
	if walking := companionTick(t, woken); !walking.CompanionMoving {
		t.Errorf("the companion did not stroll after being woken")
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

// TestCompanionTicksChangeOnlyItsOwnRow is the cost bound. The creature draws in
// one row, so a tick may change exactly that row and nothing else: the frame is
// not re-laid out, no body row is touched and the row count does not move. It is
// asserted over several screens and terminal sizes, on ticks that are not a tip
// boundary, because a tip rotating is the one other thing the slow clock shows.
func TestCompanionTicksChangeOnlyItsOwnRow(t *testing.T) {
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
			m := NewModel()
			isolateGoldenTest(t, &m)
			m.Screen = f.screen
			m.Width, m.Height = f.width, f.height
			m.Animating, m.AnimTick = true, firstTick
			if f.backups {
				m.AvailableBackups = []system.BackupInfo{testBackupInfo()}
				m.SelectedBackup = 0
			}

			before := m.View()
			after := companionTick(t, m).View()

			beforeRows := strings.Split(before, "\n")
			afterRows := strings.Split(after, "\n")
			if len(beforeRows) != len(afterRows) {
				t.Fatalf("a tick changed the row count from %d to %d",
					len(beforeRows), len(afterRows))
			}

			var changed []int
			for i := range beforeRows {
				if beforeRows[i] != afterRows[i] {
					changed = append(changed, i)
				}
			}
			if len(changed) != 1 {
				t.Fatalf("a tick changed %d rows, want exactly the companion's one:\nbefore:\n%s\nafter:\n%s",
					len(changed), before, after)
			}
			if !companionRowHasArt(afterRows[changed[0]]) {
				t.Errorf("the changed row carries no companion art: %q", plainRow(afterRows[changed[0]]))
			}
			if f.framed {
				next := changed[0] + 1
				switch {
				case next >= len(afterRows):
					t.Errorf("the companion row is the frame's last row, so no footer rule follows it")
				case !isRuleRow(afterRows[next]):
					t.Errorf("the companion row is not the row immediately above the footer rule; the row under it is %q",
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
		index, row := companionRenderedRow(t, view)
		rows := strings.Split(view, "\n")
		if index+1 >= len(rows) || !isRuleRow(rows[index+1]) {
			t.Fatalf("the companion row is not immediately above the footer rule:\n%s", view)
		}
		if !strings.Contains(plainRow(row), "(o.o)") {
			t.Errorf("the frame's first companion row is not the idle frame: %q", plainRow(row))
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
		// A narrow screen with one row to spare shows the panel's summary and no
		// companion: the summary is the fact the panel was answering with and the
		// creature is decoration, so the creature is the one that gives way. With
		// two rows to spare both fit, the summary above the creature.
		panels := []panel{
			testPanel(panelMachine, "Machine", "Linux · x86_64", "OS  Linux"),
			testPanel(panelPlan, "Plan", "8 steps", "Steps  8"),
		}
		hints := []installerHint{hintUp, hintDown, hintSelect, hintQuit}

		for _, spare := range []int{1, 2} {
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
			if spare == 1 && companion {
				t.Errorf("with one spare row the companion took the row from the summary:\n%s", view)
			}
			if spare == 2 && !companion {
				t.Errorf("with two spare rows the companion did not draw:\n%s", view)
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
func TestCompanionTicksOnlyWithinTheStage(t *testing.T) {
	m := NewModel()
	isolateGoldenTest(t, &m)
	m.Screen = ScreenComplete
	m.Width, m.Height = 100, 24
	m.Animating = true

	stage := companionStageWidth(m)
	limit := stage - companionCellWidth

	lowest, highest := m.CompanionPos, m.CompanionPos
	reversals := 0
	previousDir := m.CompanionDir
	for tick := 0; tick < 200; tick++ {
		if tick%10 == 0 {
			// The creature sleeps after twenty quiet seconds, and a sleeping one is
			// still; a key every ten ticks keeps this test about walking.
			m = companionKey(t, m, "x")
		}
		m = companionTick(t, m)
		if m.CompanionPos < 0 || m.CompanionPos > limit {
			t.Fatalf("tick %d left the companion at cell %d, outside 0..%d",
				tick, m.CompanionPos, limit)
		}
		lowest = min(lowest, m.CompanionPos)
		highest = max(highest, m.CompanionPos)
		if m.CompanionDir != previousDir {
			reversals++
			previousDir = m.CompanionDir
		}
		if row := m.companionRow(stage); len([]rune(plainRow(row))) > stage {
			t.Fatalf("tick %d drew a %d-column row on a %d-column stage",
				tick, len([]rune(plainRow(row))), stage)
		}
	}

	// It walks the row to both ends and turns instead of leaving it: it lands on
	// the left edge and stops within one stroll step of the right one, because a
	// step that would leave the row is a step it does not take.
	if lowest != 0 {
		t.Errorf("the companion never came back to the left edge: its lowest cell was %d", lowest)
	}
	if want := limit - companionStrollCells + 1; highest < want {
		t.Errorf("the companion turned at cell %d, want within one stroll step of the right edge %d",
			highest, want)
	}
	if reversals < 2 {
		t.Errorf("the companion turned %d times in 200 ticks, want at least 2", reversals)
	}
}

// TestCompanionStrollsOneCellPerFrame pins the stroll's pace at the frame rate
// the animation now runs at: one cell per frame, so the creature walks rather
// than dashing, and it logs three consecutive frames' rows so the movement can be
// read instead of trusted. The pace is a frame count, not a duration: eight cells
// a second is animTicksPerSecond frames of one cell, and moving the rate moves the
// speed with it.
func TestCompanionStrollsOneCellPerFrame(t *testing.T) {
	if companionStrollCells != 1 {
		t.Fatalf("the stroll is %d cells a frame, want exactly one", companionStrollCells)
	}

	m := NewModel()
	isolateGoldenTest(t, &m)
	m.Screen = ScreenMainMenu
	m.Width, m.Height = 160, 50
	m.Animating = true
	stage := companionStageWidth(m)

	first := companionTick(t, m)
	if first.CompanionPos != m.CompanionPos+1 {
		t.Fatalf("one frame moved the companion from cell %d to %d, want one cell",
			m.CompanionPos, first.CompanionPos)
	}
	second := companionTick(t, first)
	if second.CompanionPos != first.CompanionPos+1 {
		t.Fatalf("the next frame moved the companion from cell %d to %d, want one cell",
			first.CompanionPos, second.CompanionPos)
	}

	for _, f := range []Model{m, first, second} {
		t.Logf("frame %d, cell %d: %q", f.AnimTick, f.CompanionPos, plainRow(f.companionRow(stage)))
	}
}

// --- following the selection ----------------------------------------------

// TestCompanionWalksTowardWhatTheCursorPointsAt is the behaviour the user asked
// for by name: moving the cursor moves the creature, over the ticks that follow
// rather than in the same frame, and the walk ends where the creature can see the
// row it was pointed at. A key that moves nothing leaves it strolling.
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

	// A key that changes nothing must not send it anywhere: it is strolling.
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
	if target != stage-companionCellWidth {
		t.Errorf("the last option points at cell %d, want the right edge %d",
			target, stage-companionCellWidth)
	}

	previousDistance := companionDistance(moved.CompanionPos, target)
	arrivedAfter := -1
	for tick := 1; tick <= 40; tick++ {
		moved = companionTick(t, moved)
		distance := companionDistance(moved.CompanionPos, target)
		if distance >= previousDistance {
			t.Fatalf("tick %d did not move the companion toward the row the cursor points at: %d -> %d",
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

	// The tick after the one that lands is the tick it stands still on: the walk
	// is over, the frame is the idle one, and the stroll resumes from there.
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
	if strolling := companionTick(t, stood); !strolling.CompanionMoving {
		t.Errorf("the companion did not resume strolling after arriving")
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
// exercises -- has no cursor to point with, so the creature keeps strolling
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
		return fmt.Sprintf("%d/%d/%v/%v/%d/%d/%d",
			m.CompanionPos, m.CompanionDir, m.CompanionFollow, m.CompanionMoving,
			m.CompanionIdle, m.CompanionPleased, m.AnimTick)
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
	if !strings.Contains(first.View(), companionArtFor(first.companionStateNow(), 6)) {
		t.Errorf("the sixth tick's art is not on screen:\n%s", first.View())
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
