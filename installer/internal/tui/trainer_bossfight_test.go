package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/albersg/dotfiles/installer/internal/tui/trainer"
)

// trainerBossFightModel is a model in a boss fight, with the fight's own clock in the
// test's hand: the fight's duration is a value the test chooses rather than however long
// the suite happened to take.
func trainerBossFightModel(t *testing.T) (Model, *time.Time) {
	t.Helper()
	m := NewModel()
	m.Width, m.Height = 80, 24
	isolateGoldenTest(t, &m)
	m.TrainerStats = trainer.NewUserStats()
	m.TrainerGameState = trainer.NewGameStateWithStats(m.TrainerStats)

	now := time.Unix(1_700_000_000, 0)
	m.TrainerGameState.SetClock(func() time.Time { return now })
	m.TrainerGameState.StartBoss(trainer.ModuleHorizontal)
	m.Screen = ScreenTrainerBossResult
	return m, &now
}

// TestTrainerBossResultShowsTheFightTimeAndTheBest pins S10's visible half: the result
// screen says how long the fight took and what the module's best is, so the clock the
// fight now measures is a fact the player can read rather than bookkeeping.
func TestTrainerBossResultShowsTheFightTimeAndTheBest(t *testing.T) {
	m, now := trainerBossFightModel(t)

	// The module already has a best of 20s, and this fight takes 42s: the screen has to
	// report both numbers, and the best must not move.
	m.TrainerStats.GetModuleProgress(trainer.ModuleHorizontal).BossBestTime = 20 * time.Second
	*now = now.Add(42 * time.Second)
	m.TrainerGameState.RecordBossVictory()
	m.TrainerLastCorrect = true

	screen := plainRow(m.View())
	if !strings.Contains(screen, "Fight time: 42s") {
		t.Errorf("the result screen does not report the fight's own 42s:\n%s", screen)
	}
	if !strings.Contains(screen, "Best: 20s") {
		t.Errorf("the result screen does not report the module's best of 20s:\n%s", screen)
	}
}

// TestTrainerBossResultKeepsTheFrameAtTheFloor pins that the new facts cost no row at
// the documented 80x24 floor: the result screen's body, its rule and its footer all
// still fit, so reporting the time cannot push the legend off the screen.
func TestTrainerBossResultKeepsTheFrameAtTheFloor(t *testing.T) {
	m, now := trainerBossFightModel(t)
	*now = now.Add(42 * time.Second)
	m.TrainerGameState.RecordBossVictory()
	m.TrainerLastCorrect = true

	lines := strings.Split(m.View(), "\n")
	if len(lines) > 24 {
		t.Errorf("the boss result screen is %d rows tall at 80x24:\n%s", len(lines), plainRow(m.View()))
	}
	for i, line := range lines {
		if width := len([]rune(plainRow(line))); width > 80 {
			t.Errorf("row %d is %d columns wide at 80:\n%s", i, width, plainRow(m.View()))
		}
	}
}

// TestTrainerBossTabRevealsANudge pins S11 on the screen: Tab on a step that ships no
// hint gets the player started instead of doing nothing, which is what it did on all 35
// shipped steps before this rule.
func TestTrainerBossTabRevealsANudge(t *testing.T) {
	m, _ := trainerBossFightModel(t)
	m.Screen = ScreenTrainerBoss
	step := m.TrainerGameState.CurrentExercise
	if step == nil {
		t.Fatal("the fixture is wrong: no boss step is on screen")
	}

	// The shipped corpus authors a hint on every step; this is the content the
	// fallback exists for, so the test writes its own step rather than mutating the
	// shared one.
	m.TrainerGameState.CurrentExercise = &trainer.Exercise{
		ID:        step.ID,
		Module:    step.Module,
		Level:     step.Level,
		Mission:   step.Mission,
		Solutions: step.Solutions,
		Optimal:   step.Optimal,
	}

	res, _ := m.handleTrainerBossKeys("tab")
	got := res.(Model)
	if got.TrainerMessage == "" {
		t.Fatal("Tab is dead on a boss step that ships no hint")
	}
	if strings.Contains(got.TrainerMessage, step.Optimal) {
		t.Errorf("Tab printed the whole answer %q: %q", step.Optimal, got.TrainerMessage)
	}
}

// TestTrainerBossTabShowsTheAuthoredHint is the other half: when the step has a hint,
// that is what Tab shows.
func TestTrainerBossTabShowsTheAuthoredHint(t *testing.T) {
	m, _ := trainerBossFightModel(t)
	m.Screen = ScreenTrainerBoss
	hint := m.TrainerGameState.CurrentExercise.Hint
	if hint == "" {
		t.Fatal("the fixture is wrong: the shipped step carries no hint")
	}

	res, _ := m.handleTrainerBossKeys("tab")
	got := res.(Model)
	if !strings.Contains(got.TrainerMessage, hint) {
		t.Errorf("Tab showed %q, want the step's own hint %q", got.TrainerMessage, hint)
	}
}
