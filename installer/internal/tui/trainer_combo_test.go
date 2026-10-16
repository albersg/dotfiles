package tui

import (
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/tui/trainer"
)

// trainerComboModel is a model with a live session in one mode, at the width the
// header has to fit in. The combo is a fact about the session, so the tests below ask
// the same two questions of every mode's header: nothing at one, and the multiplier
// once the run is real.
func trainerComboModel(t *testing.T, boss bool) Model {
	t.Helper()
	m := NewModel()
	m.Width, m.Height = 100, 30
	m.TrainerStats = trainer.NewUserStats()
	m.TrainerGameState = trainer.NewGameStateWithStats(m.TrainerStats)
	if boss {
		m.TrainerGameState.StartBoss(trainer.ModuleHorizontal)
		m.Screen = ScreenTrainerBoss
	} else {
		m.TrainerGameState.StartLesson(trainer.ModuleHorizontal)
		m.Screen = ScreenTrainerLesson
	}
	if m.TrainerGameState.CurrentExercise == nil {
		t.Fatal("the fixture is wrong: no exercise is on screen")
	}
	return m
}

// TestTrainerBossHeaderShowsTheCombo pins S9 on the boss screen: the multiplier the
// score is paid by is visible while the fight is on, and absent when there is no run
// to speak of.
func TestTrainerBossHeaderShowsTheCombo(t *testing.T) {
	m := trainerComboModel(t, true)

	if got := plainRow(m.trainerBossVital()); strings.Contains(got, "Combo") {
		t.Errorf("the boss header names a combo at multiplier 1: %q", got)
	}

	m.TrainerGameState.ComboMultiplier = 3
	got := plainRow(m.trainerBossVital())
	if !strings.Contains(got, "Combo") || !strings.Contains(got, "×3") {
		t.Errorf("the boss header does not show the multiplier 3 the score is paid by: %q", got)
	}
}

// TestTrainerExerciseHeaderShowsTheCombo is the same rule on the lesson and practice
// screens, whose header carries the score and the streak.
func TestTrainerExerciseHeaderShowsTheCombo(t *testing.T) {
	m := trainerComboModel(t, false)

	if got := plainRow(m.trainerExerciseVital()); strings.Contains(got, "Combo") {
		t.Errorf("the exercise header names a combo at multiplier 1: %q", got)
	}

	m.TrainerGameState.ComboMultiplier = 2
	got := plainRow(m.trainerExerciseVital())
	if !strings.Contains(got, "Combo") || !strings.Contains(got, "×2") {
		t.Errorf("the exercise header does not show the multiplier 2 the score is paid by: %q", got)
	}
}

// TestTrainerComboSurvivesTheDocumentedFloor pins the header budget: at the trainer's
// documented 80x24 floor the multiplier is still on the screen. The boss header is the
// tight one -- lives, step and countdown are already chips -- so it is the one that
// decides whether the combo is a fact the player can see or a chip that is silently
// truncated away.
func TestTrainerComboSurvivesTheDocumentedFloor(t *testing.T) {
	m := trainerComboModel(t, true)
	m.Width, m.Height = 80, 24
	m.TrainerGameState.ComboMultiplier = 4

	header := plainRow(m.trainerBossVital())
	if !strings.Contains(header, "×4") {
		t.Errorf("the boss header loses the multiplier at 80 columns: %q", header)
	}
	if width := len([]rune(header)); width > 80 {
		t.Errorf("the boss vital is %d columns wide at 80 columns: %q", width, header)
	}
}
