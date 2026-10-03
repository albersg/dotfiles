package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/tui/trainer"
	tea "github.com/charmbracelet/bubbletea"
)

// trainerSaveWarning is what a failed progress save has to say. The trainer
// renders TrainerMessage on its menu, exercise and result screens, and
// clearTrainerProfile already reports its own save failure in this shape; the
// defect was every other save discarding the error, so a read-only HOME lost the
// player's progress without a word.
const trainerSaveWarning = "⚠️ Could not save trainer progress"

// unwritableTrainerHome points $HOME at a regular file, so trainer.SaveStats
// cannot create ~/.config/dotfiles-trainer and fails with ENOTDIR. A read-only
// directory would not fail for a root test run; a file in the path is the
// read-only-HOME case the defect is about and fails for every user.
func unwritableTrainerHome(t *testing.T) {
	t.Helper()

	home := t.TempDir()
	asFile := filepath.Join(home, "home-is-a-file")
	if err := os.WriteFile(asFile, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", asFile)
}

// failingTrainerSaveModel is a trainer model whose progress cannot be persisted:
// TrainerStats is present, so every save site reaches SaveStats, and HOME cannot
// hold the profile directory.
func failingTrainerSaveModel(t *testing.T) Model {
	t.Helper()
	unwritableTrainerHome(t)

	m := NewModel()
	m.Width = 80
	m.Height = 24
	m.TrainerStats = trainer.NewUserStats()
	m.TrainerModules = trainer.GetAllModules()
	m.TrainerCursor = 0
	return m
}

func assertTrainerSaveWarning(t *testing.T, m Model, site string) {
	t.Helper()
	if !strings.Contains(m.TrainerMessage, trainerSaveWarning) {
		t.Errorf("%s discarded a failed progress save: TrainerMessage = %q, want it to warn", site, m.TrainerMessage)
	}
}

// TestTrainerMenuEscapeReportsFailedSave covers update.go:738: escape from the
// trainer menu saved the profile and left for the main menu without checking the
// result.
func TestTrainerMenuEscapeReportsFailedSave(t *testing.T) {
	m := failingTrainerSaveModel(t)
	m.Screen = ScreenTrainerMenu

	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := res.(Model)

	assertTrainerSaveWarning(t, got, "escape from the trainer menu")
}

// TestTrainerExerciseEscapeReportsFailedSave covers update.go:747: escape from a
// lesson or practice exercise owned the save for both screens.
func TestTrainerExerciseEscapeReportsFailedSave(t *testing.T) {
	for _, screen := range []struct {
		name string
		id   Screen
	}{
		{"lesson", ScreenTrainerLesson},
		{"practice", ScreenTrainerPractice},
	} {
		t.Run(screen.name, func(t *testing.T) {
			m := failingTrainerSaveModel(t)
			m.Screen = screen.id

			res, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
			got := res.(Model)

			assertTrainerSaveWarning(t, got, "escape from a "+screen.name+" exercise")
		})
	}
}

// TestTrainerBossEscapeReportsFailedSave covers update.go:754: escaping a boss
// fight saved the run and reported the abandoned fight, but discarded a failed
// save.
func TestTrainerBossEscapeReportsFailedSave(t *testing.T) {
	m := failingTrainerSaveModel(t)
	m.Screen = ScreenTrainerBoss

	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := res.(Model)

	assertTrainerSaveWarning(t, got, "escape from a boss fight")
}

// TestTrainerResultEscapeReportsFailedSave covers update.go:761: escape from a
// result screen saved the session and cleared the message, including a failed
// save.
func TestTrainerResultEscapeReportsFailedSave(t *testing.T) {
	for _, screen := range []struct {
		name string
		id   Screen
	}{
		{"lesson result", ScreenTrainerResult},
		{"boss result", ScreenTrainerBossResult},
	} {
		t.Run(screen.name, func(t *testing.T) {
			m := failingTrainerSaveModel(t)
			m.Screen = screen.id

			res, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
			got := res.(Model)

			assertTrainerSaveWarning(t, got, "escape from the "+screen.name)
		})
	}
}

// TestTrainerMenuResetReportsFailedSave covers update.go:1972: resetting a
// module's practice progress saved without checking the result.
func TestTrainerMenuResetReportsFailedSave(t *testing.T) {
	m := failingTrainerSaveModel(t)
	m.Screen = ScreenTrainerMenu
	m.TrainerCursor = 0 // the first module is always unlocked

	res, _ := m.handleTrainerMenuKeys("r")
	got := res.(Model)

	assertTrainerSaveWarning(t, got, "resetting a module's practice progress")
}

// TestTrainerMenuQuitReportsFailedSave covers update.go:2000: q on the trainer
// menu saved before returning to the main menu without checking the result.
func TestTrainerMenuQuitReportsFailedSave(t *testing.T) {
	m := failingTrainerSaveModel(t)
	m.Screen = ScreenTrainerMenu

	res, _ := m.handleTrainerMenuKeys("q")
	got := res.(Model)

	assertTrainerSaveWarning(t, got, "quitting the trainer menu")
}

// TestTrainerPracticeSubmitReportsFailedSave covers update.go:2144: recording a
// practice answer saved the profile without checking the result.
func TestTrainerPracticeSubmitReportsFailedSave(t *testing.T) {
	m := failingTrainerSaveModel(t)
	m.Screen = ScreenTrainerPractice

	exercise := trainer.GetLessons(trainer.ModuleHorizontal)[0]
	m.TrainerGameState = trainer.NewGameStateWithStats(m.TrainerStats)
	m.TrainerGameState.IsPracticeMode = true
	m.TrainerGameState.CurrentModule = trainer.ModuleHorizontal
	m.TrainerGameState.SetPracticeExercise(&exercise)
	m.TrainerInput = "w"

	res, _ := m.handleTrainerExerciseKeys("enter")
	got := res.(Model)

	assertTrainerSaveWarning(t, got, "submitting a practice answer")
}

// TestTrainerSessionCompleteReportsFailedSave covers update.go:2308: finishing a
// lesson or practice session saved the completed profile without checking the
// result, then overwrote the message with the celebration.
func TestTrainerSessionCompleteReportsFailedSave(t *testing.T) {
	t.Run("lesson", func(t *testing.T) {
		m := failingTrainerSaveModel(t)
		m.Screen = ScreenTrainerResult

		m.TrainerGameState = trainer.NewGameStateWithStats(m.TrainerStats)
		m.TrainerGameState.StartLesson(trainer.ModuleHorizontal)
		// Past the last exercise, so the result screen takes the
		// session-complete branch.
		m.TrainerGameState.ExerciseIndex = len(m.TrainerGameState.Exercises)

		res, _ := m.handleTrainerResultKeys("enter")
		got := res.(Model)

		assertTrainerSaveWarning(t, got, "completing the lesson session")
	})

	t.Run("practice", func(t *testing.T) {
		m := failingTrainerSaveModel(t)
		m.Screen = ScreenTrainerResult

		// Practice ends only once every exercise is mastered, which is what
		// makes NextPracticeExercise report there is nothing left to present.
		progress := m.TrainerStats.GetModuleProgress(trainer.ModuleHorizontal)
		for _, lesson := range trainer.GetLessons(trainer.ModuleHorizontal) {
			progress.GetExerciseStats(lesson.ID).Mastered = true
		}
		m.TrainerGameState = trainer.NewGameStateWithStats(m.TrainerStats)
		m.TrainerGameState.IsPracticeMode = true
		m.TrainerGameState.CurrentModule = trainer.ModuleHorizontal

		res, _ := m.handleTrainerResultKeys("enter")
		got := res.(Model)

		assertTrainerSaveWarning(t, got, "completing the practice session")
	})
}

// TestTrainerResultQuitReportsFailedSave covers update.go:2322: q on the result
// screen saved before returning to the menu without checking the result.
func TestTrainerResultQuitReportsFailedSave(t *testing.T) {
	m := failingTrainerSaveModel(t)
	m.Screen = ScreenTrainerResult

	res, _ := m.handleTrainerResultKeys("q")
	got := res.(Model)

	assertTrainerSaveWarning(t, got, "quitting a result screen")
}

// TestTrainerBossResultReportsFailedSave covers update.go:2336: leaving the boss
// result screen saved before returning to the menu without checking the result.
func TestTrainerBossResultReportsFailedSave(t *testing.T) {
	m := failingTrainerSaveModel(t)
	m.Screen = ScreenTrainerBossResult

	res, _ := m.handleTrainerBossResultKeys("enter")
	got := res.(Model)

	assertTrainerSaveWarning(t, got, "leaving the boss result screen")
}

// TestTrainerSaveReportsSuccessAndKeepsTheNormalMessage is the positive control:
// when HOME can hold the profile, a save must succeed and the screens must keep
// exactly the messages they had, so the warning is not printed unconditionally.
func TestTrainerSaveReportsSuccessAndKeepsTheNormalMessage(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	m := NewModel()
	m.Width = 80
	m.Height = 24
	m.TrainerStats = trainer.NewUserStats()
	m.TrainerModules = trainer.GetAllModules()
	m.TrainerCursor = 0
	m.Screen = ScreenTrainerBoss

	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := res.(Model)

	if strings.Contains(got.TrainerMessage, trainerSaveWarning) {
		t.Errorf("a successful save warned anyway: %q", got.TrainerMessage)
	}
	if got.TrainerMessage != "Boss fight abandoned!" {
		t.Errorf("a successful save changed the message: %q, want the ordinary one", got.TrainerMessage)
	}
	if onDisk := trainer.LoadStats(); onDisk == nil {
		t.Error("a successful save left no profile on disk")
	}
}
