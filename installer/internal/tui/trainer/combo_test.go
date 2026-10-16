package trainer

import "testing"

// comboModes is how the suite reaches each of the trainer's three modes. The combo
// rule is one rule, so the tests below run the same assertions through all three.
var comboModes = []struct {
	name  string
	start func(*GameState)
	next  func(*GameState) bool
}{
	{"lesson", func(g *GameState) { g.StartLesson(ModuleHorizontal) }, func(g *GameState) bool { return g.NextExercise() }},
	{"practice", func(g *GameState) { g.StartPractice(ModuleHorizontal) }, func(g *GameState) bool { return g.NextPracticeExercise() }},
	{"boss", func(g *GameState) { g.StartBoss(ModuleHorizontal) }, func(g *GameState) bool { return g.NextExercise() }},
}

// TestComboRisesOnEveryCorrectAnswer pins S9's first half: the multiplier the score is
// paid by is a property of a correct answer, not of the boss screen. Before this rule
// only the boss advance raised it, so a lesson or practice run was multiplied by one
// for its whole length while the screen never said so either way.
func TestComboRisesOnEveryCorrectAnswer(t *testing.T) {
	for _, mode := range comboModes {
		t.Run(mode.name, func(t *testing.T) {
			state := NewGameState()
			mode.start(state)
			if state.CurrentExercise == nil {
				t.Fatal("the fixture is wrong: no exercise is on screen")
			}
			if state.ComboMultiplier != 1 {
				t.Fatalf("a fresh session starts at multiplier %d, want 1", state.ComboMultiplier)
			}

			want := 1
			for answer := 0; answer < MaxComboMultiplier+2; answer++ {
				state.RecordCorrectAnswer(5, false)
				want++
				if want > MaxComboMultiplier {
					want = MaxComboMultiplier
				}
				if state.ComboMultiplier != want {
					t.Fatalf("after %d correct answers the multiplier is %d, want %d",
						answer+1, state.ComboMultiplier, want)
				}
				if state.CurrentExercise == nil {
					break
				}
				if !mode.next(state) || state.CurrentExercise == nil {
					break
				}
			}
			if want < MaxComboMultiplier {
				t.Fatalf("the fixture is wrong: only %d answers were served", want)
			}
		})
	}
}

// TestWrongAnswerResetsTheComboInEveryMode pins the other half: a mistake ends the run
// wherever it happens. A boss step that times out is a mistake too, which is why the
// timeout routes through the same recorder.
func TestWrongAnswerResetsTheComboInEveryMode(t *testing.T) {
	for _, mode := range comboModes {
		t.Run(mode.name, func(t *testing.T) {
			state := NewGameState()
			mode.start(state)

			state.RecordCorrectAnswer(5, false)
			state.RecordCorrectAnswer(5, false)
			if state.ComboMultiplier <= 1 {
				t.Fatalf("the fixture is wrong: two correct answers left the multiplier at %d", state.ComboMultiplier)
			}

			state.RecordIncorrectAnswer()
			if state.ComboMultiplier != 1 {
				t.Errorf("a wrong answer in %s left the multiplier at %d, want 1", mode.name, state.ComboMultiplier)
			}
		})
	}
}

// TestBossTimeoutResetsTheCombo pins the clock's own failure: an expired window is a
// lost life and a lost run, exactly like a wrong answer.
func TestBossTimeoutResetsTheCombo(t *testing.T) {
	state := NewGameState()
	state.StartBoss(ModuleHorizontal)
	state.RecordCorrectAnswer(5, false)
	if state.ComboMultiplier <= 1 {
		t.Fatal("the fixture is wrong: a correct answer did not raise the multiplier")
	}

	state.RecordBossStepTimeout()
	if state.ComboMultiplier != 1 {
		t.Errorf("an expired step left the multiplier at %d, want 1", state.ComboMultiplier)
	}
}

// TestTheScoreIsPaidWithTheCombo pins that the multiplier the screens now show is the
// one the score is multiplied by: the second correct answer is worth more than the
// first, at the same speed and with the same optimality.
func TestTheScoreIsPaidWithTheCombo(t *testing.T) {
	state := NewGameState()
	state.StartLesson(ModuleHorizontal)

	before := state.SessionScore
	state.RecordCorrectAnswer(10, false)
	first := state.SessionScore - before

	state.NextExercise()
	before = state.SessionScore
	state.RecordCorrectAnswer(10, false)
	second := state.SessionScore - before

	if state.ComboMultiplier != 3 {
		t.Fatalf("the fixture is wrong: the multiplier is %d after two answers, want 3", state.ComboMultiplier)
	}
	if second <= first {
		t.Errorf("the second answer paid %d and the first %d; the multiplier is shown but not paid",
			second, first)
	}
}
