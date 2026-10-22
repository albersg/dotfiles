package tui

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/albersg/dotfiles/installer/internal/tui/trainer"
)

// ============================================================================
// THE TRAINER ON THE REFERENCE GRAMMAR
// ============================================================================
//
// These guards hold the half of the trainer's redesign that the 80x24 goldens
// cannot: the six screens draw the shared card vocabulary (components.go) where
// the frame holds two columns, and the compact body -- the one the floor and the
// trainer's own frame guard measure -- everywhere else. They read the screen the
// user gets, so a card that stopped being a card, a row that stopped naming its
// module, a window that stopped admitting what it hid, or a message that grew an
// emoji fails here rather than in a review.

// trainerCardWide is the frame the card guards measure: wide enough for the
// layout's two columns and tall enough for every screen's body. It is the same
// terminal the other converted screens' wide goldens use.
const trainerCardWide = "160x50"

// trainerCardModel builds one trainer screen at a frame, with the session behind
// it: the menu on its module list, the exercise screens on a live lesson, practice
// run or boss fight, and the two result screens on the state their handlers leave.
func trainerCardModel(t *testing.T, screen Screen, width, height int) Model {
	t.Helper()
	t.Setenv("HOME", t.TempDir())

	m := NewModel()
	m.Width, m.Height = width, height
	m.Screen = screen
	m.TrainerStats = trainer.NewUserStats()
	m.TrainerModules = trainer.GetAllModules()
	m.TrainerGameState = trainer.NewGameStateWithStats(m.TrainerStats)
	// The exercise and boss screens render a countdown and the result screens a
	// fight duration, both through the state's injected clock, so the guards pin it.
	m.TrainerGameState.SetClock(func() time.Time {
		return time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	})

	switch screen {
	case ScreenTrainerMenu:
		seedTrainerMenuProgress(t, &m)
	case ScreenTrainerLesson, ScreenTrainerResult:
		m.TrainerGameState.StartLesson(trainer.ModuleHorizontal)
	case ScreenTrainerPractice:
		exercises := trainer.GetLessons(trainer.ModuleHorizontal)
		if len(exercises) == 0 {
			t.Fatal("the horizontal module has no lessons to practise")
		}
		m.TrainerGameState.StartPractice(trainer.ModuleHorizontal)
		m.TrainerGameState.SetPracticeExercise(&exercises[0])
	case ScreenTrainerBoss, ScreenTrainerBossResult:
		m.TrainerGameState.StartBoss(trainer.ModuleHorizontal)
	}
	return m
}

// trainerScreens is the six the redesign converts, in the order the guards read
// them.
var trainerScreens = []Screen{
	ScreenTrainerMenu,
	ScreenTrainerLesson,
	ScreenTrainerPractice,
	ScreenTrainerBoss,
	ScreenTrainerResult,
	ScreenTrainerBossResult,
}

// trainerCardFrame is the shape a grammar card draws: the top border with the chip
// set into it, the rule under the pair, and the closing border.
var trainerCardFrame = []string{"╭", "│─", "╰"}

// TestTrainerScreensDrawTheReferenceCards pins the shape half of the conversion:
// at a frame the installer's own screens go two-column, every trainer screen
// draws the shared card's borders, its rule under the chip and its closing border;
// at the 80x24 floor none of them does, because the floor keeps the compact body.
func TestTrainerScreensDrawTheReferenceCards(t *testing.T) {
	for _, screen := range trainerScreens {
		t.Run(screen.String(), func(t *testing.T) {
			wide := trainerCardModel(t, screen, 160, 50)
			view := plainRow(wide.View())
			for _, mark := range trainerCardFrame {
				if !strings.Contains(view, mark) {
					t.Errorf("%s at %s does not draw the grammar's card (%q is missing):\n%s",
						screen, trainerCardWide, mark, view)
				}
			}
			// The card is the grammar's, not a box of the screen's own: its chip
			// carries the screen's own word on the top border.
			if !strings.Contains(view, "╭─ ") {
				t.Errorf("%s at %s draws no chip on its card's top border:\n%s", screen, trainerCardWide, view)
			}

			floor := trainerCardModel(t, screen, trainerFrameWidth, trainerFrameHeight)
			for _, mark := range trainerCardFrame {
				if strings.Contains(plainRow(floor.View()), mark) {
					t.Errorf("%s at %dx%d drew the card (%q) where the floor keeps the compact body",
						screen, trainerFrameWidth, trainerFrameHeight, mark)
				}
			}
		})
	}
}

// TestTrainerScreensKeepTheirOwnWordsInTheCards pins that the conversion renamed
// nothing: every screen still says what it said, in the same place -- the header
// chip its name, the body its blocks and its keys.
func TestTrainerScreensKeepTheirOwnWordsInTheCards(t *testing.T) {
	cases := []struct {
		screen Screen
		want   []string
	}{
		{ScreenTrainerMenu, []string{"Vim Mastery Trainer", "Modules", "Modules", "Lessons", "Practice", "Boss"}},
		{ScreenTrainerLesson, []string{"Lesson", "Code", "Mission", "Answer", "Type command", "[Enter] submit"}},
		{ScreenTrainerPractice, []string{"Practice", "Code", "Mission", "Answer"}},
		{ScreenTrainerBoss, []string{"Boss", "Lives:", "Step 1/5", "Code", "Challenge", "Answer", "[Esc] forfeit"}},
		{ScreenTrainerResult, []string{"Incorrect", "Summary", "Explanation", "Module", "Level"}},
		{ScreenTrainerBossResult, []string{"Defeat", "Summary", "Bosses", "Score", "return to menu"}},
	}

	for _, tc := range cases {
		t.Run(tc.screen.String(), func(t *testing.T) {
			view := plainRow(trainerCardModel(t, tc.screen, 160, 50).View())
			for _, want := range tc.want {
				if !strings.Contains(view, want) {
					t.Errorf("%s at %s dropped %q:\n%s", tc.screen, trainerCardWide, want, view)
				}
			}
		})
	}
}

// TestTrainerMenuCardIsTheProfileItReads pins the menu's card as a reading of the
// recorded profile: every module is a row with its state word and its meters, and
// the module under the cursor is a card of its own with the recorded ladder.
func TestTrainerMenuCardIsTheProfileItReads(t *testing.T) {
	m := trainerCardModel(t, ScreenTrainerMenu, 160, 50)
	view := plainRow(m.View())

	for _, module := range m.TrainerModules {
		if !strings.Contains(view, module.Name) {
			t.Errorf("the menu card does not name %q:\n%s", module.Name, view)
		}
	}
	for _, want := range []string{
		"9 modules · 1 cleared",
		"cleared · w, W, e, E", // the seeded module's own state and its commands
		"locked · viw, vaw",    // a module behind it
		"Done", "7/15",         // the lessons meter and its count
		"Mastered", "2/", // the mastery meter and its count
		"7/15 completed",             // the ladder's first rung
		"60% over 10 answers",        // the practice reading
		"The Line Walker · defeated", // the boss rung
		"horizontal_003 (3✗)",        // the weakest exercise with its count
		"Accuracy",                   // the practice group's first fact
	} {
		if !strings.Contains(view, want) {
			t.Errorf("the menu card does not state %q:\n%s", want, view)
		}
	}

	// The module under the cursor is the card's subject, not every module's: the
	// other modules' descriptions are their rows' subtitles, and the ladder is the
	// selected module's alone.
	if got := strings.Count(view, "not played yet"); got > 1 {
		t.Errorf("the detail card states another module's practice reading (%d times): the ladder must be the selected module's", got)
	}
}

// TestTrainerMenuCardWindowsOutLoud pins the short wide frame: the list keeps the
// cursor's row, says how many rows it hid, and draws the same card whatever row
// the cursor is on -- the shape the frame's own guards require of every list.
func TestTrainerMenuCardWindowsOutLoud(t *testing.T) {
	m := trainerCardModel(t, ScreenTrainerMenu, 160, 24)
	if len(m.TrainerModules) < 3 {
		t.Fatal("the corpus has too few modules for this guard")
	}
	m.TrainerCursor = len(m.TrainerModules) - 1

	view := plainRow(m.View())
	if !strings.Contains(view, "more above") {
		t.Errorf("the menu card at 160x24 does not say what its window hid:\n%s", view)
	}
	if !strings.Contains(view, m.TrainerModules[m.TrainerCursor].Name) {
		t.Errorf("the menu card hid the row under the cursor:\n%s", view)
	}

	// The card's top border is the same at every stop, so walking the list cannot
	// resize it. The segment is measured up to its closing corner: the card beside
	// it is the module under the cursor's own and names that module.
	top := func(cursor int) string {
		m.TrainerCursor = cursor
		for _, line := range strings.Split(plainRow(m.View()), "\n") {
			at := strings.Index(line, "╭─ Modules")
			if at < 0 {
				continue
			}
			if end := strings.Index(line[at:], "╮"); end >= 0 {
				return line[at : at+end+1]
			}
		}
		return ""
	}
	first := top(0)
	if first == "" {
		t.Fatalf("no modules card at 160x24:\n%s", plainRow(m.View()))
	}
	for cursor := range m.TrainerModules {
		if got := top(cursor); got != first {
			t.Errorf("the modules card's top border moved at cursor %d:\n got %q\nwant %q", cursor, got, first)
		}
	}
}

// TestTrainerExerciseCardKeepsTheExerciseWhole pins the playing screens: the code
// with its line-number gutter, the mission, the answer field and the feedback are
// all inside the two cards, and the scroll keys move the window the code card
// draws.
func TestTrainerExerciseCardKeepsTheExerciseWhole(t *testing.T) {
	for _, screen := range []Screen{ScreenTrainerLesson, ScreenTrainerPractice, ScreenTrainerBoss} {
		t.Run(screen.String(), func(t *testing.T) {
			m := trainerCardModel(t, screen, 160, 50)
			exercise := m.TrainerGameState.CurrentExercise
			if exercise == nil {
				t.Fatal("no exercise is on screen")
			}

			view := plainRow(m.View())
			if !strings.Contains(view, exercise.Mission) {
				t.Errorf("%s does not state the mission %q:\n%s", screen, exercise.Mission, view)
			}
			if !strings.Contains(view, fmt.Sprintf("%2d │ ", exercise.CursorPos.Line+1)) {
				t.Errorf("%s does not draw its code with the line-number gutter:\n%s", screen, view)
			}
			if !strings.Contains(view, " 1 │ ") {
				t.Errorf("%s does not start the code at its first line:\n%s", screen, view)
			}
			// The answer field, empty or not, is always there.
			if !strings.Contains(view, "...") {
				t.Errorf("%s does not draw the empty answer field:\n%s", screen, view)
			}

			// The scroll keys and the card read one budget: the first press of the
			// scroll key opens the window the card draws, and the card's caption
			// states which rows that is.
			if len(exercise.Code) <= 1 {
				return
			}
			scrolled := m
			scrolled.TrainerCodeScrollFor = ""
			scrolled.TrainerCodeScroll = 0
			rows := trainerScrollRows(t, scrolled, exercise)
			if rows >= len(exercise.Code) {
				return // the whole exercise is on screen; there is nothing to scroll
			}
			want := fmt.Sprintf("rows 2-%d of %d", min(1+rows, len(exercise.Code)), len(exercise.Code))
			if !strings.Contains(plainRow(scrolled.View()), want) {
				t.Errorf("%s does not name the window it shows (%q):\n%s", screen, want, plainRow(scrolled.View()))
			}
		})
	}
}

// trainerScrollRows is how many code rows the screen's window shows for an
// exercise: the budget the renderer and the scroll keys both read.
func trainerScrollRows(t *testing.T, m Model, exercise *trainer.Exercise) int {
	t.Helper()
	_, rows := m.trainerTextBudget(exercise)
	return rows
}

// TestTrainerResultCardKeepsTheVerdictItsFactsAndTheBuffers pins the result
// screen's card: what was answered, the verdict's message, the explanation and --
// for a buffer-verified exercise -- the buffer the answer produced.
func TestTrainerResultCardKeepsTheVerdictItsFactsAndTheBuffers(t *testing.T) {
	m := trainerCardModel(t, ScreenTrainerResult, 160, 50)
	exercise := m.TrainerGameState.CurrentExercise
	if exercise == nil {
		t.Fatal("no exercise behind the result screen")
	}
	m.TrainerMessage = "Correct! The optimal solution."
	m.TrainerValidation = &trainer.ValidationResult{
		IsCorrect:      false,
		BufferVerified: true,
		TargetBuffer:   []string{"const userName = 'dotfiles';"},
		ActualBuffer:   []string{"const userName = 'dotfiles'"},
	}
	bufferExercise := *exercise
	bufferExercise.BufferVerified = true
	m.TrainerGameState.CurrentExercise = &bufferExercise

	view := plainRow(m.View())
	for _, want := range []string{
		exercise.ID, string(exercise.Module), "Level",
		"Summary", m.TrainerMessage, "Explanation", exercise.Explanation,
		"Expected buffer", "Resulting buffer",
		"const userName = 'dotfiles'", // the produced buffer, drawn under its divider
	} {
		if want == "" {
			continue
		}
		if !strings.Contains(view, want) {
			t.Errorf("the result card does not state %q:\n%s", want, view)
		}
	}
}

// TestTrainerBossResultCardStatesTheFightsNumbers pins the fight's result: the
// stat boxes carry the run's numbers, the card carries the verdict, and the
// measured fight time and best are fact rows of their own.
func TestTrainerBossResultCardStatesTheFightsNumbers(t *testing.T) {
	m := trainerCardModel(t, ScreenTrainerBossResult, 160, 50)
	state := m.TrainerGameState
	module := state.CurrentModule
	m.TrainerStats.GetModuleProgress(module).BossBestTime = 20 * time.Second
	state.SetClock(func() time.Time {
		return time.Date(2026, 1, 1, 12, 0, 42, 0, time.UTC)
	})
	state.RecordBossVictory()
	m.TrainerLastCorrect = true

	view := plainRow(m.View())
	for _, want := range []string{
		"✓ Victory",
		"Score", strconv.Itoa(m.TrainerStats.TotalScore),
		"Bosses", "1/9",
		"Best", "20s",
		"You defeated " + state.CurrentBoss.Name,
		"Lives remaining:",
		"+500 bonus points!",
		"Next module unlocked!",
		"Fight time", "42s",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("the boss result card does not state %q:\n%s", want, view)
		}
	}
}

// TestTrainerFramesFitTheirTerminals pins the six screens inside every frame the
// card body is drawn at: no row past the terminal, no line past its width. The
// matrix guard measures the whole package; this one measures the shapes this file
// is about, including the wide-but-short frame that only has room for a window.
func TestTrainerFramesFitTheirTerminals(t *testing.T) {
	for _, size := range [][2]int{{160, 50}, {140, 44}, {200, 60}, {227, 62}, {160, 24}} {
		for _, screen := range trainerScreens {
			name := fmt.Sprintf("%s/%dx%d", screen, size[0], size[1])
			t.Run(name, func(t *testing.T) {
				m := trainerCardModel(t, screen, size[0], size[1])
				view := m.View()
				if rows := renderedRowCount(view); rows > size[1] {
					t.Errorf("%s renders %d rows, want at most %d:\n%s", name, rows, size[1], view)
				}
				for _, line := range strings.Split(ansiEscape.ReplaceAllString(view, ""), "\n") {
					if width := lipglossWidth(line); width > size[0] {
						t.Errorf("%s renders a %d-column line, want at most %d: %q", name, width, size[0], line)
					}
				}
			})
		}
	}
}

// TestTrainerScreensDrawNoEmoji pins the trainer's own rule where the grammar
// touches it: the redesign keeps Nerd Font glyphs and plain marks, so a terminal
// without an emoji font draws every screen. It reads the messages the handlers can
// put on a trainer screen straight out of update.go, sets each one, and renders
// the six screens at the floor and at the wide frame.
func TestTrainerScreensDrawNoEmoji(t *testing.T) {
	messages := trainerMessagesFromSource(t)
	if len(messages) < 8 {
		t.Fatalf("found only %d trainer messages in update.go, so this guard proves little: %q", len(messages), messages)
	}

	for _, message := range messages {
		for _, size := range [][2]int{{trainerFrameWidth, trainerFrameHeight}, {160, 50}} {
			for _, screen := range trainerScreens {
				m := trainerCardModel(t, screen, size[0], size[1])
				m.TrainerMessage = message
				if r, found := firstBoxEmoji(m.View()); found {
					t.Errorf("%s at %dx%d draws %q (%U) with the message %q: the trainer keeps plain glyphs",
						screen, size[0], size[1], r, r, message)
				}
			}
		}
	}
}

// trainerMessagesFromSource reads the strings update.go can assign to
// m.TrainerMessage -- the literals of the assignments and of the named constants
// they concatenate -- so the emoji guard measures the copy the handlers really
// produce instead of a second copy written by hand.
func trainerMessagesFromSource(t *testing.T) []string {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "update.go", nil, 0)
	if err != nil {
		t.Fatalf("parse update.go: %v", err)
	}

	consts := map[string][]string{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range value.Names {
				if i < len(value.Values) {
					consts[name.Name] = stringLiterals(value.Values[i])
				}
			}
		}
	}

	seen := map[string]bool{}
	var out []string
	add := func(literals []string) {
		for _, literal := range literals {
			if literal == "" || seen[literal] {
				continue
			}
			seen[literal] = true
			out = append(out, literal)
		}
	}

	ast.Inspect(file, func(node ast.Node) bool {
		assign, ok := node.(*ast.AssignStmt)
		if !ok {
			return true
		}
		target := false
		for _, lhs := range assign.Lhs {
			if sel, ok := lhs.(*ast.SelectorExpr); ok && sel.Sel.Name == "TrainerMessage" {
				target = true
			}
		}
		if !target {
			return true
		}
		for _, rhs := range assign.Rhs {
			add(stringLiterals(rhs))
			ast.Inspect(rhs, func(inner ast.Node) bool {
				if ident, ok := inner.(*ast.Ident); ok {
					add(consts[ident.Name])
				}
				return true
			})
		}
		return true
	})
	return out
}

// stringLiterals returns the string literals of an expression, in the order they
// appear, so a message built by concatenation is read as the pieces it is built
// from.
func stringLiterals(node ast.Node) []string {
	var out []string
	ast.Inspect(node, func(inner ast.Node) bool {
		lit, ok := inner.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		value, err := strconv.Unquote(lit.Value)
		if err == nil {
			out = append(out, value)
		}
		return true
	})
	return out
}

// lipglossWidth measures a plain line: the guards here have already stripped the
// escape sequences, so counting runes is the same measurement lipgloss makes of a
// styled string.
func lipglossWidth(line string) int {
	return len([]rune(line))
}

// ============================================================================
// THE WIDE GOLDENS
// ============================================================================
//
// The four trainer goldens that already exist are the 80x24 floor. These five pin
// the other shape: the same screens drawn as the reference grammar's cards at the
// frame the installer's own wide goldens use. Each model is built with a pinned
// clock, so the header's countdown and the fight's duration are the same numbers on
// every runner, and each capture is the model's own render rather than the runner's
// event stream.

// TestTrainerMenuWideGolden pins the module menu as two cards: the modules as
// grammar rows with their state in the gutter and their meters trailing, and the
// module under the cursor as its own card with the recorded ladder and readings.
func TestTrainerMenuWideGolden(t *testing.T) {
	skipIfTermux(t)
	m := trainerCardModel(t, ScreenTrainerMenu, 160, 50)
	requireGoldenCapture(t, []byte(m.View()))
}

// TestTrainerLessonWideGolden pins a lesson as the reference draws it: the code
// window in its card at the left and the mission, the answer and the feedback in
// the card at the right.
func TestTrainerLessonWideGolden(t *testing.T) {
	skipIfTermux(t)
	m := trainerCardModel(t, ScreenTrainerLesson, 160, 50)
	requireGoldenCapture(t, []byte(m.View()))
}

// TestTrainerBossWideGolden pins a boss step: the same two cards with the fight's
// lives, step and countdown in the header chips and the screen's own word for the
// mission block on the card.
func TestTrainerBossWideGolden(t *testing.T) {
	skipIfTermux(t)
	m := trainerCardModel(t, ScreenTrainerBoss, 160, 50)
	requireGoldenCapture(t, []byte(m.View()))
}

// TestTrainerResultWideGolden pins the exercise result: what was answered as fact
// rows, the verdict's message and the exercise's explanation under the card's own
// dividers.
func TestTrainerResultWideGolden(t *testing.T) {
	skipIfTermux(t)
	m := trainerCardModel(t, ScreenTrainerResult, 160, 50)
	m.TrainerLastCorrect = true
	m.TrainerMessage = "Correct! The optimal solution."
	requireGoldenCapture(t, []byte(m.View()))
}

// TestTrainerBossResultWideGolden pins the fight's result: the run's numbers in the
// stat boxes and the verdict, the fight's measured time and its best in the card.
func TestTrainerBossResultWideGolden(t *testing.T) {
	skipIfTermux(t)
	m := trainerCardModel(t, ScreenTrainerBossResult, 160, 50)
	state := m.TrainerGameState
	m.TrainerStats.GetModuleProgress(state.CurrentModule).BossBestTime = 20 * time.Second
	state.SetClock(func() time.Time {
		return time.Date(2026, 1, 1, 12, 0, 42, 0, time.UTC)
	})
	state.RecordBossVictory()
	m.TrainerLastCorrect = true
	requireGoldenCapture(t, []byte(m.View()))
}
