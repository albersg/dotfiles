package tui

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/albersg/dotfiles/installer/internal/tui/trainer"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/teatest"
)

// =============================================================================
// VIM TRAINER E2E TESTS - Playwright-style golden tests
// =============================================================================

// TestTrainerMenuGolden tests the trainer module selection screen
func TestTrainerMenuGolden(t *testing.T) {
	m := NewModel()
	isolateGoldenTest(t, &m)
	m.Width = 80
	m.Height = 24
	m.Screen = ScreenTrainerMenu
	m.TrainerStats = trainer.NewUserStats()
	m.TrainerModules = trainer.GetAllModules()
	m.TrainerCursor = 0

	// Seed real progress so the snapshot proves the numbers render. A snapshot
	// of an empty profile would match even if every count were broken.
	seedTrainerMenuProgress(t, &m)

	tm := teatest.NewTestModel(t, m,
		teatest.WithInitialTermSize(80, 24),
	)

	time.Sleep(100 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))

	out := readAll(t, tm.FinalOutput(t))
	teatest.RequireEqualOutput(t, out)
}

// TestTrainerLessonGolden tests a lesson exercise screen
func TestTrainerLessonGolden(t *testing.T) {
	m := NewModel()
	isolateGoldenTest(t, &m)
	m.Width = 80
	m.Height = 24
	m.Screen = ScreenTrainerLesson
	m.TrainerStats = trainer.NewUserStats()

	// Start a lesson for horizontal module (first unlocked module)
	m.TrainerGameState = trainer.NewGameStateWithStats(m.TrainerStats)
	// The exercise screen now renders a countdown derived from the injected
	// clock, so the golden pins that clock to keep the snapshot deterministic on
	// any host (the same reason isolateGoldenTest pins HOME and the platform).
	m.TrainerGameState.SetClock(func() time.Time {
		return time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	})
	m.TrainerGameState.StartLesson(trainer.ModuleHorizontal)
	m.TrainerInput = ""
	m.TrainerMessage = ""

	tm := teatest.NewTestModel(t, m,
		teatest.WithInitialTermSize(80, 24),
	)

	time.Sleep(100 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))

	out := readAll(t, tm.FinalOutput(t))
	teatest.RequireEqualOutput(t, out)
}

// TestTrainerResultCorrectGolden tests the result screen after correct answer
func TestTrainerResultCorrectGolden(t *testing.T) {
	m := NewModel()
	isolateGoldenTest(t, &m)
	m.Width = 80
	m.Height = 24
	m.Screen = ScreenTrainerResult
	m.TrainerStats = trainer.NewUserStats()
	m.TrainerGameState = trainer.NewGameStateWithStats(m.TrainerStats)
	m.TrainerGameState.StartLesson(trainer.ModuleHorizontal)
	m.TrainerLastCorrect = true
	m.TrainerMessage = "✨ Perfect! Optimal solution!"

	tm := teatest.NewTestModel(t, m,
		teatest.WithInitialTermSize(80, 24),
	)

	time.Sleep(100 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))

	out := readAll(t, tm.FinalOutput(t))
	teatest.RequireEqualOutput(t, out)
}

// TestTrainerBossGolden tests the boss fight screen
func TestTrainerBossGolden(t *testing.T) {
	m := NewModel()
	isolateGoldenTest(t, &m)
	m.Width = 80
	m.Height = 24
	m.Screen = ScreenTrainerBoss
	m.TrainerStats = trainer.NewUserStats()

	// Prepare stats so boss is ready
	progress := m.TrainerStats.GetModuleProgress(trainer.ModuleHorizontal)
	progress.LessonsCompleted = 15
	progress.LessonsTotal = 15
	progress.PracticeAccuracy = 85.0

	m.TrainerGameState = trainer.NewGameStateWithStats(m.TrainerStats)
	// The boss screen now renders a countdown derived from the step's TimeLimit
	// through the injected clock, so the golden pins that clock to keep the
	// snapshot deterministic on any host (the same reason the lesson golden pins
	// it and isolateGoldenTest pins HOME and the platform).
	m.TrainerGameState.SetClock(func() time.Time {
		return time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	})
	m.TrainerGameState.StartBoss(trainer.ModuleHorizontal)
	m.TrainerInput = ""
	m.TrainerMessage = ""

	tm := teatest.NewTestModel(t, m,
		teatest.WithInitialTermSize(80, 24),
	)

	time.Sleep(100 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))

	out := readAll(t, tm.FinalOutput(t))
	teatest.RequireEqualOutput(t, out)
}

// =============================================================================
// E2E NAVIGATION FLOW TESTS
// =============================================================================

// TestTrainerNavigationE2E tests navigating from main menu to trainer
func TestTrainerNavigationE2E(t *testing.T) {
	m := NewModel()
	m.Width = 80
	m.Height = 24

	tm := teatest.NewTestModel(t, m,
		teatest.WithInitialTermSize(80, 24),
	)

	// Welcome -> Enter
	time.Sleep(50 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	time.Sleep(50 * time.Millisecond)

	// Main Menu -> Navigate to Vim Trainer (index 4: Start, Learn, Keymaps, LazyVim, Vim Trainer)
	for i := 0; i < 4; i++ {
		tm.Send(tea.KeyMsg{Type: tea.KeyDown})
		time.Sleep(20 * time.Millisecond)
	}
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	time.Sleep(50 * time.Millisecond)

	// Should be at Trainer Menu now - verify we see modules
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Horizontal")) ||
			bytes.Contains(bts, []byte("Vertical")) ||
			bytes.Contains(bts, []byte("Module"))
	}, teatest.WithCheckInterval(50*time.Millisecond), teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

// TestTrainerStartLessonE2E tests starting a lesson from module menu
func TestTrainerStartLessonE2E(t *testing.T) {
	m := NewModel()
	m.Width = 80
	m.Height = 24
	m.Screen = ScreenTrainerMenu
	m.TrainerStats = trainer.NewUserStats()
	m.TrainerModules = trainer.GetAllModules()
	m.TrainerCursor = 0

	tm := teatest.NewTestModel(t, m,
		teatest.WithInitialTermSize(80, 24),
	)

	time.Sleep(50 * time.Millisecond)

	// Press Enter to start lesson on first module (Horizontal)
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	time.Sleep(100 * time.Millisecond)

	// Should show a lesson with code and mission
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Mission")) ||
			bytes.Contains(bts, []byte("Code")) ||
			bytes.Contains(bts, []byte("Lesson"))
	}, teatest.WithCheckInterval(50*time.Millisecond), teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

// TestTrainerInputE2E tests typing vim commands in a lesson
func TestTrainerInputE2E(t *testing.T) {
	m := NewModel()
	m.Width = 80
	m.Height = 24
	m.Screen = ScreenTrainerLesson
	m.TrainerStats = trainer.NewUserStats()
	m.TrainerGameState = trainer.NewGameStateWithStats(m.TrainerStats)
	m.TrainerGameState.StartLesson(trainer.ModuleHorizontal)
	m.TrainerInput = ""

	tm := teatest.NewTestModel(t, m,
		teatest.WithInitialTermSize(80, 24),
	)

	time.Sleep(50 * time.Millisecond)

	// Type "w" as input (common first lesson answer)
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	time.Sleep(50 * time.Millisecond)

	// Verify input is shown
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("w"))
	}, teatest.WithCheckInterval(50*time.Millisecond), teatest.WithDuration(2*time.Second))

	// Press Enter to submit
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	time.Sleep(100 * time.Millisecond)

	// Should go to result screen
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Correct")) ||
			bytes.Contains(bts, []byte("Incorrect")) ||
			bytes.Contains(bts, []byte("Perfect")) ||
			bytes.Contains(bts, []byte("Result"))
	}, teatest.WithCheckInterval(50*time.Millisecond), teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

// TestTrainerBackspaceE2E tests backspace functionality in input
func TestTrainerBackspaceE2E(t *testing.T) {
	m := NewModel()
	m.Width = 80
	m.Height = 24
	m.Screen = ScreenTrainerLesson
	m.TrainerStats = trainer.NewUserStats()
	m.TrainerGameState = trainer.NewGameStateWithStats(m.TrainerStats)
	m.TrainerGameState.StartLesson(trainer.ModuleHorizontal)
	m.TrainerInput = "ww" // Pre-set some input

	tm := teatest.NewTestModel(t, m,
		teatest.WithInitialTermSize(80, 24),
	)

	time.Sleep(50 * time.Millisecond)

	// Press backspace
	tm.Send(tea.KeyMsg{Type: tea.KeyBackspace})
	time.Sleep(50 * time.Millisecond)

	// Input should now be "w" (one character removed)
	// We can verify the screen still shows input area with answer label
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("answer")) ||
			bytes.Contains(bts, []byte("Your")) ||
			bytes.Contains(bts, []byte("w"))
	}, teatest.WithCheckInterval(50*time.Millisecond), teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

// TestTrainerModuleNavigationE2E tests navigating through modules with j/k
func TestTrainerModuleNavigationE2E(t *testing.T) {
	m := NewModel()
	m.Width = 80
	m.Height = 24
	m.Screen = ScreenTrainerMenu
	m.TrainerStats = trainer.NewUserStats()
	m.TrainerModules = trainer.GetAllModules()
	m.TrainerCursor = 0

	tm := teatest.NewTestModel(t, m,
		teatest.WithInitialTermSize(80, 24),
	)

	time.Sleep(50 * time.Millisecond)

	// Navigate down through modules using j
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	time.Sleep(50 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	time.Sleep(50 * time.Millisecond)

	// Navigate up using k
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	time.Sleep(50 * time.Millisecond)

	// Should still show modules menu
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Horizontal")) ||
			bytes.Contains(bts, []byte("Vertical"))
	}, teatest.WithCheckInterval(50*time.Millisecond), teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

// TestTrainerEscapeE2E tests escape key returns to menu
func TestTrainerEscapeE2E(t *testing.T) {
	m := NewModel()
	m.Width = 80
	m.Height = 24
	m.Screen = ScreenTrainerLesson
	m.TrainerStats = trainer.NewUserStats()
	m.TrainerGameState = trainer.NewGameStateWithStats(m.TrainerStats)
	m.TrainerGameState.StartLesson(trainer.ModuleHorizontal)
	m.TrainerInput = ""

	tm := teatest.NewTestModel(t, m,
		teatest.WithInitialTermSize(80, 24),
	)

	time.Sleep(50 * time.Millisecond)

	// Press Escape to go back to menu
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	time.Sleep(100 * time.Millisecond)

	// Should be back at trainer menu
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Horizontal")) ||
			bytes.Contains(bts, []byte("Module")) ||
			bytes.Contains(bts, []byte("Vim Trainer"))
	}, teatest.WithCheckInterval(50*time.Millisecond), teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

// TestTrainerLessonProgressE2E tests progressing through multiple lessons
func TestTrainerLessonProgressE2E(t *testing.T) {
	m := NewModel()
	m.Width = 80
	m.Height = 24
	m.Screen = ScreenTrainerLesson
	m.TrainerStats = trainer.NewUserStats()
	m.TrainerGameState = trainer.NewGameStateWithStats(m.TrainerStats)
	m.TrainerGameState.StartLesson(trainer.ModuleHorizontal)
	m.TrainerInput = ""

	tm := teatest.NewTestModel(t, m,
		teatest.WithInitialTermSize(80, 24),
	)

	// Complete first exercise
	time.Sleep(50 * time.Millisecond)

	// Get the current exercise's optimal solution
	exercise := m.TrainerGameState.CurrentExercise
	if exercise == nil {
		t.Fatal("No exercise loaded")
	}

	// Type the optimal answer
	for _, r := range exercise.Optimal {
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		time.Sleep(20 * time.Millisecond)
	}

	// Submit
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	time.Sleep(100 * time.Millisecond)

	// Should show result
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Correct")) ||
			bytes.Contains(bts, []byte("Perfect")) ||
			bytes.Contains(bts, []byte("Incorrect"))
	}, teatest.WithCheckInterval(50*time.Millisecond), teatest.WithDuration(2*time.Second))

	// Press Enter to continue
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	time.Sleep(100 * time.Millisecond)

	// Should be at next lesson or back at menu
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Mission")) ||
			bytes.Contains(bts, []byte("Module")) ||
			bytes.Contains(bts, []byte("Lesson"))
	}, teatest.WithCheckInterval(50*time.Millisecond), teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

// =============================================================================
// ALL MODULES RENDERING TESTS
// =============================================================================

// TestAllModulesRenderE2E tests that each module can be accessed and renders properly
func TestAllModulesRenderE2E(t *testing.T) {
	modules := []trainer.ModuleID{
		trainer.ModuleHorizontal,
		trainer.ModuleVertical,
		trainer.ModuleTextObjects,
		trainer.ModuleChangeRepeat,
		trainer.ModuleSubstitution,
		trainer.ModuleRegex,
		trainer.ModuleMacros,
		trainer.ModuleEditing,
	}

	for _, moduleID := range modules {
		t.Run(string(moduleID), func(t *testing.T) {
			m := NewModel()
			m.Width = 80
			m.Height = 24
			m.Screen = ScreenTrainerLesson
			m.TrainerStats = trainer.NewUserStats()

			// Unlock all modules for testing
			for _, mod := range modules {
				progress := m.TrainerStats.GetModuleProgress(mod)
				progress.BossDefeated = true
			}

			m.TrainerGameState = trainer.NewGameStateWithStats(m.TrainerStats)
			m.TrainerGameState.StartLesson(moduleID)
			m.TrainerInput = ""

			// Verify we got an exercise
			if m.TrainerGameState.CurrentExercise == nil {
				t.Fatalf("Module %s has no exercises", moduleID)
			}

			tm := teatest.NewTestModel(t, m,
				teatest.WithInitialTermSize(80, 24),
			)

			time.Sleep(100 * time.Millisecond)

			// Verify screen renders with mission and code
			teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
				return bytes.Contains(bts, []byte("Mission")) &&
					bytes.Contains(bts, []byte("Code"))
			}, teatest.WithCheckInterval(50*time.Millisecond), teatest.WithDuration(2*time.Second))

			tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
			tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
		})
	}
}

// TestTextObjectsVisualSelectionE2E tests that text object exercises show visual selection
func TestTextObjectsVisualSelectionE2E(t *testing.T) {
	m := NewModel()
	m.Width = 80
	m.Height = 24
	m.Screen = ScreenTrainerLesson
	m.TrainerStats = trainer.NewUserStats()

	// Unlock TextObjects
	progress := m.TrainerStats.GetModuleProgress(trainer.ModuleHorizontal)
	progress.BossDefeated = true

	m.TrainerGameState = trainer.NewGameStateWithStats(m.TrainerStats)
	m.TrainerGameState.StartLesson(trainer.ModuleTextObjects)
	m.TrainerInput = ""

	tm := teatest.NewTestModel(t, m,
		teatest.WithInitialTermSize(80, 24),
	)

	time.Sleep(50 * time.Millisecond)

	// Type "iw" to see visual selection
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	time.Sleep(30 * time.Millisecond)
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	time.Sleep(100 * time.Millisecond)

	// Verify screen shows input
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("iw"))
	}, teatest.WithCheckInterval(50*time.Millisecond), teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

// TestBossFlowE2E tests the complete boss fight flow
func TestBossFlowE2E(t *testing.T) {
	m := NewModel()
	m.Width = 80
	m.Height = 24
	m.Screen = ScreenTrainerMenu
	m.TrainerStats = trainer.NewUserStats()
	m.TrainerModules = trainer.GetAllModules()

	// Make boss ready for horizontal module
	progress := m.TrainerStats.GetModuleProgress(trainer.ModuleHorizontal)
	progress.LessonsCompleted = 15
	progress.LessonsTotal = 15
	progress.PracticeAccuracy = 85.0
	progress.PracticeAttempts = 100

	m.TrainerCursor = 0

	tm := teatest.NewTestModel(t, m,
		teatest.WithInitialTermSize(80, 24),
	)

	time.Sleep(50 * time.Millisecond)

	// Press 'b' to start boss fight
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	time.Sleep(100 * time.Millisecond)

	// Should be in boss fight screen
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Boss")) ||
			bytes.Contains(bts, []byte("Lives")) ||
			bytes.Contains(bts, []byte("Step"))
	}, teatest.WithCheckInterval(50*time.Millisecond), teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

// TestLockedModuleMessageE2E tests that locked modules show appropriate message
func TestLockedModuleMessageE2E(t *testing.T) {
	m := NewModel()
	m.Width = 80
	m.Height = 24
	m.Screen = ScreenTrainerMenu
	m.TrainerStats = trainer.NewUserStats()
	m.TrainerModules = trainer.GetAllModules()
	m.TrainerCursor = 1 // Vertical module (locked by default)

	tm := teatest.NewTestModel(t, m,
		teatest.WithInitialTermSize(80, 24),
	)

	time.Sleep(50 * time.Millisecond)

	// Try to start locked module
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	time.Sleep(100 * time.Millisecond)

	// Should show locked message, still in menu
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("locked")) ||
			bytes.Contains(bts, []byte("Locked")) ||
			bytes.Contains(bts, []byte("Module"))
	}, teatest.WithCheckInterval(50*time.Millisecond), teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

// =============================================================================
// RESPONSIVE LAYOUT TESTS FOR TRAINER
// =============================================================================

// TestTrainerResponsiveE2E tests trainer screens at different terminal sizes
func TestTrainerResponsiveE2E(t *testing.T) {
	sizes := []struct {
		name   string
		width  int
		height int
	}{
		{"small_terminal", 60, 20},
		{"medium_terminal", 80, 24},
		{"large_terminal", 120, 40},
		{"wide_terminal", 160, 24},
	}

	for _, sz := range sizes {
		t.Run(sz.name, func(t *testing.T) {
			m := NewModel()
			m.Width = sz.width
			m.Height = sz.height
			m.Screen = ScreenTrainerLesson
			m.TrainerStats = trainer.NewUserStats()
			m.TrainerGameState = trainer.NewGameStateWithStats(m.TrainerStats)
			m.TrainerGameState.StartLesson(trainer.ModuleHorizontal)
			m.TrainerInput = "w"

			tm := teatest.NewTestModel(t, m,
				teatest.WithInitialTermSize(sz.width, sz.height),
			)

			time.Sleep(100 * time.Millisecond)
			tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
			tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))

			// Verify it produces output without panicking
			out := readAll(t, tm.FinalOutput(t))
			if len(out) == 0 {
				t.Error("Expected some output")
			}
		})
	}
}

// TestPracticeModeE2E tests practice mode functionality
func TestPracticeModeE2E(t *testing.T) {
	m := NewModel()
	m.Width = 80
	m.Height = 24
	m.Screen = ScreenTrainerMenu
	m.TrainerStats = trainer.NewUserStats()
	m.TrainerModules = trainer.GetAllModules()

	// Complete all lessons to unlock practice
	progress := m.TrainerStats.GetModuleProgress(trainer.ModuleHorizontal)
	progress.LessonsCompleted = 15
	progress.LessonsTotal = 15

	m.TrainerCursor = 0

	tm := teatest.NewTestModel(t, m,
		teatest.WithInitialTermSize(80, 24),
	)

	time.Sleep(50 * time.Millisecond)

	// Press 'p' to start practice
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	time.Sleep(100 * time.Millisecond)

	// Should be in practice mode
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return bytes.Contains(bts, []byte("Practice")) ||
			bytes.Contains(bts, []byte("Mission")) ||
			bytes.Contains(bts, []byte("Code"))
	}, teatest.WithCheckInterval(50*time.Millisecond), teatest.WithDuration(2*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	tm.WaitFinished(t, teatest.WithFinalTimeout(time.Second))
}

// =============================================================================
// PRACTICE ACCOUNTING REGRESSION
// =============================================================================

// newPracticeSubmissionModel builds a model parked on a known practice exercise,
// so a submission can be driven through the real UI handler.
func newPracticeSubmissionModel(t *testing.T) (Model, *trainer.Exercise) {
	t.Helper()

	// Isolate the stats file the handler writes on submit.
	t.Setenv("HOME", t.TempDir())

	m := NewModel()
	m.Width = 80
	m.Height = 24
	m.Screen = ScreenTrainerPractice
	m.TrainerStats = trainer.NewUserStats()
	m.TrainerGameState = trainer.NewGameStateWithStats(m.TrainerStats)
	m.TrainerGameState.StartPractice(trainer.ModuleHorizontal)

	lessons := trainer.GetLessons(trainer.ModuleHorizontal)
	if len(lessons) == 0 {
		t.Fatal("no horizontal lessons available")
	}
	exercise := lessons[0]
	m.TrainerGameState.SetPracticeExercise(&exercise)

	return m, m.TrainerGameState.CurrentExercise
}

// TestTrainerPracticeSubmissionCountsOnce is the regression test for the double count.
// The answer handler called RecordCorrectAnswer/RecordIncorrectAnswer and then
// RecordPracticeResult for the same submission, so PracticeAttempts and
// PracticeCorrect advanced twice per answer. The defect lives in the composition
// of two calls inside the UI handler, so the assertion has to run a real
// submission through Model.Update.
func TestTrainerPracticeSubmissionCountsOnce(t *testing.T) {
	t.Run("correct answer advances the counters once and records mastery", func(t *testing.T) {
		m, exercise := newPracticeSubmissionModel(t)

		m.TrainerInput = exercise.Optimal
		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = result.(Model)

		progress := m.TrainerStats.GetModuleProgress(trainer.ModuleHorizontal)
		if progress.PracticeAttempts != 1 {
			t.Errorf("PracticeAttempts = %d after one correct submission, want 1", progress.PracticeAttempts)
		}
		if progress.PracticeCorrect != 1 {
			t.Errorf("PracticeCorrect = %d after one correct submission, want 1", progress.PracticeCorrect)
		}

		// The per-exercise mastery is why RecordPracticeResult stays the owner;
		// the fix must not satisfy the counters by dropping mastery.
		exStats := progress.GetExerciseStats(exercise.ID)
		if exStats.TotalAttempts != 1 {
			t.Errorf("exercise %s TotalAttempts = %d after one submission, want 1", exercise.ID, exStats.TotalAttempts)
		}
		if exStats.TotalCorrect != 1 {
			t.Errorf("exercise %s TotalCorrect = %d after one correct submission, want 1", exercise.ID, exStats.TotalCorrect)
		}
	})

	t.Run("incorrect answer advances attempts once and no correct", func(t *testing.T) {
		m, exercise := newPracticeSubmissionModel(t)

		wrong := "ZZZZZZ"
		if trainer.ValidateAnswer(exercise, wrong) {
			t.Fatalf("test setup: %q must not validate for exercise %s", wrong, exercise.ID)
		}

		m.TrainerInput = wrong
		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = result.(Model)

		if m.TrainerLastCorrect {
			t.Fatal("test setup: the submission was accepted, expected a rejection")
		}

		progress := m.TrainerStats.GetModuleProgress(trainer.ModuleHorizontal)
		if progress.PracticeAttempts != 1 {
			t.Errorf("PracticeAttempts = %d after one incorrect submission, want 1", progress.PracticeAttempts)
		}
		if progress.PracticeCorrect != 0 {
			t.Errorf("PracticeCorrect = %d after one incorrect submission, want 0", progress.PracticeCorrect)
		}

		exStats := progress.GetExerciseStats(exercise.ID)
		if exStats.TotalAttempts != 1 {
			t.Errorf("exercise %s TotalAttempts = %d after one submission, want 1", exercise.ID, exStats.TotalAttempts)
		}
		if exStats.TotalWrong != 1 {
			t.Errorf("exercise %s TotalWrong = %d after one incorrect submission, want 1", exercise.ID, exStats.TotalWrong)
		}
	})
}

// =============================================================================
// BOSS BOOKKEEPING REGRESSION
// =============================================================================

// TestTrainerBossAnswerAccounting is the regression test for the boss path doing
// its own bookkeeping. handleTrainerBossKeys decremented BossLives directly and
// never called RecordCorrectAnswer/RecordIncorrectAnswer, so a boss step
// contributed no streak, no score and no attempt, and the boss branch inside
// GameState.RecordIncorrectAnswer was dead. The assertions drive the real UI
// handler, because that is where the bypass lived.
func TestTrainerBossAnswerAccounting(t *testing.T) {
	t.Run("a correct boss step moves streak and score", func(t *testing.T) {
		m := newTrainerBossModel(t)

		step := m.TrainerGameState.CurrentBoss.Steps[m.TrainerGameState.BossStep]
		m.TrainerInput = step.Exercise.Optimal

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = result.(Model)

		if m.TrainerGameState.CurrentStreak != 1 {
			t.Errorf("CurrentStreak = %d after a correct boss step, want 1", m.TrainerGameState.CurrentStreak)
		}
		if m.TrainerGameState.SessionScore <= 0 {
			t.Errorf("SessionScore = %d after a correct boss step, want > 0", m.TrainerGameState.SessionScore)
		}
		if m.TrainerStats.TotalScore <= 0 {
			t.Errorf("TotalScore = %d after a correct boss step, want > 0", m.TrainerStats.TotalScore)
		}
		if m.TrainerGameState.BossLives != 3 {
			t.Errorf("BossLives = %d after a correct boss step, want 3", m.TrainerGameState.BossLives)
		}
	})

	t.Run("a non-optimal step names the step it judged", func(t *testing.T) {
		m := newTrainerBossModel(t)

		answered := m.TrainerGameState.CurrentBoss.Steps[m.TrainerGameState.BossStep]
		next := m.TrainerGameState.CurrentBoss.Steps[m.TrainerGameState.BossStep+1]
		if answered.Exercise.Optimal == next.Exercise.Optimal {
			t.Fatalf("test setup: the first two steps share the optimal %q", answered.Exercise.Optimal)
		}

		// Reaches the same result as the answered step's own optimal without being
		// it, so the answer takes the branch that names an optimal solution.
		m.TrainerInput = "wl"
		if !trainer.ValidateAnswer(&answered.Exercise, m.TrainerInput) {
			t.Fatalf("test setup: %q is not accepted for %s", m.TrainerInput, answered.Exercise.ID)
		}
		if trainer.IsOptimalAnswer(&answered.Exercise, m.TrainerInput) {
			t.Fatalf("test setup: %q is the optimal of %s", m.TrainerInput, answered.Exercise.ID)
		}

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = result.(Model)

		// BossStep has already advanced when the message is built, so naming the
		// next step's optimal would spoil the challenge the user has not seen.
		if !strings.Contains(m.TrainerMessage, answered.Exercise.Optimal) {
			t.Errorf("message %q does not name the optimal %q of the answered step %s",
				m.TrainerMessage, answered.Exercise.Optimal, answered.Exercise.ID)
		}
		if strings.Contains(m.TrainerMessage, next.Exercise.Optimal) {
			t.Errorf("message %q names the NEXT step's optimal %q",
				m.TrainerMessage, next.Exercise.Optimal)
		}
	})

	t.Run("a lost boss spends the lives and records each attempt", func(t *testing.T) {
		m := newTrainerBossModel(t)
		progress := m.TrainerStats.GetModuleProgress(trainer.ModuleHorizontal)
		lives := m.TrainerGameState.BossLives
		const wrong = "ZZZZZZ"

		for i := 0; i < lives; i++ {
			if m.Screen != ScreenTrainerBoss {
				t.Fatalf("screen = %v after %d wrong answers, want %v", m.Screen, i, ScreenTrainerBoss)
			}
			step := m.TrainerGameState.CurrentBoss.Steps[m.TrainerGameState.BossStep]
			if trainer.ValidateAnswer(&step.Exercise, wrong) {
				t.Fatalf("test setup: %q validated against %s", wrong, step.Exercise.ID)
			}
			m.TrainerInput = wrong
			result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			m = result.(Model)
		}

		if m.TrainerLastCorrect {
			t.Error("TrainerLastCorrect is true after losing the fight")
		}
		if m.Screen != ScreenTrainerBossResult {
			t.Fatalf("screen = %v after losing the fight, want %v", m.Screen, ScreenTrainerBossResult)
		}
		if m.TrainerGameState.BossLives != 0 {
			t.Errorf("BossLives = %d after losing the fight, want 0", m.TrainerGameState.BossLives)
		}
		if progress.BossAttempts != lives {
			t.Errorf("BossAttempts = %d after %d failed steps, want %d", progress.BossAttempts, lives, lives)
		}
		if m.TrainerGameState.IsBossDefeated {
			t.Error("IsBossDefeated is true after losing the fight; it must mean the player won")
		}
	})
}

// TestTrainerBossResultUnlockClaim covers the victory screen's unlock message.
// renderTrainerBossResult printed "Next module unlocked!" unconditionally, so
// the final boss claimed a module that does not exist.
func TestTrainerBossResultUnlockClaim(t *testing.T) {
	newVictoryModel := func(t *testing.T, module trainer.ModuleID) Model {
		t.Helper()
		t.Setenv("HOME", t.TempDir())

		m := NewModel()
		m.Screen = ScreenTrainerBossResult
		m.TrainerStats = trainer.NewUserStats()
		m.TrainerGameState = trainer.NewGameStateWithStats(m.TrainerStats)
		m.TrainerGameState.StartBoss(module)
		m.TrainerGameState.RecordBossVictory()
		m.TrainerLastCorrect = true
		return m
	}

	t.Run("the final module does not claim an unlock", func(t *testing.T) {
		m := newVictoryModel(t, trainer.ModuleEditing)

		out := m.renderTrainerBossResult()
		if strings.Contains(out, "unlocked") {
			t.Errorf("final boss result claims an unlock:\n%s", out)
		}
	})

	t.Run("a non-final module still claims the unlock", func(t *testing.T) {
		m := newVictoryModel(t, trainer.ModuleHorizontal)

		out := m.renderTrainerBossResult()
		if !strings.Contains(out, "unlocked") {
			t.Errorf("non-final boss result lost its unlock message:\n%s", out)
		}
	})
}

// =============================================================================
// SPACE KEY ROUTING REGRESSION
// =============================================================================

// newTrainerMenuModel builds a model parked on the trainer module menu.
func newTrainerMenuModel(t *testing.T) Model {
	t.Helper()
	t.Setenv("HOME", t.TempDir())

	m := NewModel()
	m.Screen = ScreenTrainerMenu
	m.TrainerStats = trainer.NewUserStats()
	m.TrainerModules = trainer.GetAllModules()
	m.TrainerCursor = 0
	return m
}

// newTrainerLessonModel builds a model parked on a live lesson exercise screen.
func newTrainerLessonModel(t *testing.T) Model {
	t.Helper()
	t.Setenv("HOME", t.TempDir())

	m := NewModel()
	m.Screen = ScreenTrainerLesson
	m.TrainerStats = trainer.NewUserStats()
	m.TrainerGameState = trainer.NewGameStateWithStats(m.TrainerStats)
	m.TrainerGameState.StartLesson(trainer.ModuleHorizontal)
	m.TrainerInput = ""
	return m
}

// newTrainerResultModel builds a model parked on the exercise result screen
// with a live lesson session behind it.
func newTrainerResultModel(t *testing.T) Model {
	t.Helper()
	t.Setenv("HOME", t.TempDir())

	m := NewModel()
	m.Screen = ScreenTrainerResult
	m.TrainerStats = trainer.NewUserStats()
	m.TrainerGameState = trainer.NewGameStateWithStats(m.TrainerStats)
	m.TrainerGameState.StartLesson(trainer.ModuleHorizontal)
	m.TrainerLastCorrect = true
	m.TrainerMessage = "✨ Perfect!"
	return m
}

// newTrainerBossModel builds a model parked on a live boss exercise screen.
func newTrainerBossModel(t *testing.T) Model {
	t.Helper()
	t.Setenv("HOME", t.TempDir())

	m := NewModel()
	m.Screen = ScreenTrainerBoss
	m.TrainerStats = trainer.NewUserStats()
	progress := m.TrainerStats.GetModuleProgress(trainer.ModuleHorizontal)
	progress.LessonsCompleted = 15
	progress.LessonsTotal = 15
	progress.PracticeAccuracy = 85.0
	m.TrainerGameState = trainer.NewGameStateWithStats(m.TrainerStats)
	m.TrainerGameState.StartBoss(trainer.ModuleHorizontal)
	m.TrainerInput = ""
	return m
}

// TestTrainerSpaceKeyRouting is the regression test for the leader-mode clash.
// The global key handler treated space as the leader-key prefix on every screen
// except the lesson, practice and boss exercise screens, so on the trainer menu
// and on the result screens it set LeaderMode instead of reaching
// handleTrainerMenuKeys / handleTrainerResultKeys / handleTrainerBossResultKeys,
// whose `case "enter", " "` arms were therefore unreachable for space. The
// assertions drive the real global handler through Model.Update, because that is
// where the routing, and the defect, lives.
func TestTrainerSpaceKeyRouting(t *testing.T) {
	t.Run("space starts the selected module from the trainer menu", func(t *testing.T) {
		m := newTrainerMenuModel(t)

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
		m = result.(Model)

		if m.LeaderMode {
			t.Fatal("space activated leader mode on the trainer menu")
		}
		if m.Screen != ScreenTrainerLesson {
			t.Fatalf("screen after space = %v, want %v", m.Screen, ScreenTrainerLesson)
		}
		if m.TrainerGameState == nil || m.TrainerGameState.CurrentExercise == nil {
			t.Fatal("space did not start a lesson exercise")
		}
	})

	t.Run("space advances from the exercise result screen", func(t *testing.T) {
		m := newTrainerResultModel(t)

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
		m = result.(Model)

		if m.LeaderMode {
			t.Fatal("space activated leader mode on the trainer result screen")
		}
		if m.Screen == ScreenTrainerResult {
			t.Fatal("space did not continue past the result screen")
		}
	})

	t.Run("space returns from the boss result screen", func(t *testing.T) {
		m := NewModel()
		m.Screen = ScreenTrainerBossResult
		m.TrainerStats = trainer.NewUserStats()

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
		m = result.(Model)

		if m.LeaderMode {
			t.Fatal("space activated leader mode on the boss result screen")
		}
		if m.Screen != ScreenTrainerMenu {
			t.Fatalf("screen after space = %v, want %v", m.Screen, ScreenTrainerMenu)
		}
	})

	t.Run("space stays ordinary input on the exercise screens", func(t *testing.T) {
		cases := []struct {
			name  string
			model func(*testing.T) Model
		}{
			{"lesson", newTrainerLessonModel},
			{"practice", func(t *testing.T) Model {
				m, _ := newPracticeSubmissionModel(t)
				return m
			}},
			{"boss", newTrainerBossModel},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				m := tc.model(t)
				m.TrainerInput = ""

				result, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
				m = result.(Model)

				if m.LeaderMode {
					t.Fatal("space activated leader mode on an exercise screen")
				}
				if m.TrainerInput != " " {
					t.Errorf("TrainerInput = %q after space, want %q", m.TrainerInput, " ")
				}
			})
		}
	})
}

// =============================================================================
// ESCAPE ROUTING REGRESSION
// =============================================================================

// TestTrainerEscapePersistsLessonProgress is the regression test for lost lesson
// progress. Esc is intercepted by the global handler in handleEscape before any
// screen-specific trainer handler runs, so the exercise screens never saved the
// session they were leaving. The assertions drive the real global handler
// through Model.Update, because that is where the interception, and the defect,
// lives.
func TestTrainerEscapePersistsLessonProgress(t *testing.T) {
	m := newTrainerLessonModel(t)

	exercise := m.TrainerGameState.CurrentExercise
	if exercise == nil {
		t.Fatal("no exercise loaded")
	}

	// Answer the first exercise so the session has progress worth keeping.
	m.TrainerInput = exercise.Optimal
	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = result.(Model)

	// Continue to the next exercise so Esc is pressed on the lesson screen
	// itself; the result screen already saved on its own esc path.
	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = result.(Model)
	if m.Screen != ScreenTrainerLesson {
		t.Fatalf("screen after continuing = %v, want %v", m.Screen, ScreenTrainerLesson)
	}

	result, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = result.(Model)

	if m.Screen != ScreenTrainerMenu {
		t.Fatalf("screen after esc = %v, want %v", m.Screen, ScreenTrainerMenu)
	}

	loaded := trainer.LoadStats()
	if loaded == nil {
		t.Fatal("stats file is missing after leaving a lesson with esc; the earned progress was lost")
	}
	progress := loaded.GetModuleProgress(trainer.ModuleHorizontal)
	if progress.LessonsCompleted < 1 {
		t.Errorf("LessonsCompleted = %d after answering one lesson exercise and leaving with esc, want >= 1", progress.LessonsCompleted)
	}
}

// TestTrainerEscapePersistsPracticeProgress covers the practice screen. The
// practice handler saves on every answer, but any stats still in memory must
// survive leaving the screen with esc, using the same save path.
func TestTrainerEscapePersistsPracticeProgress(t *testing.T) {
	m, _ := newPracticeSubmissionModel(t)

	m.TrainerStats.TotalScore = 42

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = result.(Model)

	if m.Screen != ScreenTrainerMenu {
		t.Fatalf("screen after esc = %v, want %v", m.Screen, ScreenTrainerMenu)
	}
	loaded := trainer.LoadStats()
	if loaded == nil {
		t.Fatal("stats file is missing after leaving practice with esc")
	}
	if loaded.TotalScore != 42 {
		t.Errorf("TotalScore = %d after leaving practice with esc, want 42", loaded.TotalScore)
	}
}

// TestTrainerEscapeAbandonsBossVisibly covers the boss screen. Esc must persist
// the run and report the abandon instead of leaving silently.
func TestTrainerEscapeAbandonsBossVisibly(t *testing.T) {
	m := newTrainerBossModel(t)

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = result.(Model)

	if m.Screen != ScreenTrainerMenu {
		t.Fatalf("screen after esc = %v, want %v", m.Screen, ScreenTrainerMenu)
	}
	if m.TrainerMessage != "Boss fight abandoned!" {
		t.Errorf("TrainerMessage after esc = %q, want %q", m.TrainerMessage, "Boss fight abandoned!")
	}
	if trainer.LoadStats() == nil {
		t.Error("stats file is missing after abandoning a boss with esc")
	}
}

// =============================================================================
// CONTROL KEY INPUT REGRESSION
// =============================================================================

// TestTrainerControlKeysReachSimulatorInput is the regression test for literal
// control key text in the answer. The exercise and boss handlers each declared
// their own accepted control set that listed ctrl+a, ctrl+e and ctrl+w, but the
// conversion switch only mapped ctrl+d/u/f/b. Every other accepted combination
// fell through to the default arm, which appended the raw key name, so pressing
// ctrl+a typed the six characters "ctrl+a" into the answer. That text can never
// validate, and since the simulator rejects unrecognized input the submission is
// lost. The handlers must instead ignore keys the simulator cannot parse, while
// still inserting the control characters it does model.
//
// ctrl+e is the one key whose mapping is not a control character: it types
// trainer.EscToken, the trainer's stand-in for the Esc key an insert answer has
// to use. The model still receives the token through TrainerInput, so the
// interface and the engine cannot disagree about what the token is.
func TestTrainerControlKeysReachSimulatorInput(t *testing.T) {
	t.Run("ctrl+a is ignored on the exercise screens", func(t *testing.T) {
		cases := []struct {
			name  string
			model func(*testing.T) Model
		}{
			{"lesson", newTrainerLessonModel},
			{"practice", func(t *testing.T) Model {
				m, _ := newPracticeSubmissionModel(t)
				return m
			}},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				m := tc.model(t)
				m.TrainerInput = ""

				result, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
				m = result.(Model)

				if m.TrainerInput != "" {
					t.Errorf("TrainerInput = %q after ctrl+a, want empty", m.TrainerInput)
				}
			})
		}
	})

	t.Run("ctrl+a is ignored in a boss fight", func(t *testing.T) {
		m := newTrainerBossModel(t)
		m.TrainerInput = ""

		result, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
		m = result.(Model)

		if m.TrainerInput != "" {
			t.Errorf("TrainerInput = %q after ctrl+a, want empty", m.TrainerInput)
		}
	})

	t.Run("ctrl+w is ignored too", func(t *testing.T) {
		cases := []struct {
			name  string
			model func(*testing.T) Model
		}{
			{"lesson", newTrainerLessonModel},
			{"boss", newTrainerBossModel},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				m := tc.model(t)
				m.TrainerInput = ""

				result, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlW})
				m = result.(Model)

				if m.TrainerInput != "" {
					t.Errorf("TrainerInput = %q after ctrl+w, want empty", m.TrainerInput)
				}
			})
		}
	})

	t.Run("ctrl+e types the escape token on every exercise screen", func(t *testing.T) {
		cases := []struct {
			name  string
			model func(*testing.T) Model
		}{
			{"lesson", newTrainerLessonModel},
			{"boss", newTrainerBossModel},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				m := tc.model(t)
				m.TrainerInput = "iX"

				result, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlE})
				m = result.(Model)

				// The literal token, not the key's name and not an escape byte.
				want := "iX" + trainer.EscToken
				if m.TrainerInput != want {
					t.Errorf("TrainerInput = %q after ctrl+e, want %q", m.TrainerInput, want)
				}

				// The engine parses exactly what the interface inserted: the answer
				// now leaves insert mode instead of staying open.
				exercise := m.TrainerGameState.CurrentExercise
				if exercise == nil {
					t.Fatal("no exercise loaded")
				}
				if got := trainer.SimulateEditing(exercise.Code, exercise.CursorPos, m.TrainerInput); got.Mode != trainer.ModeNormal {
					t.Errorf("SimulateEditing(%q).Mode = %v, want %v", m.TrainerInput, got.Mode, trainer.ModeNormal)
				}
			})
		}
	})

	t.Run("modelled control keys still insert their control character", func(t *testing.T) {
		controlKeys := []struct {
			name string
			key  tea.KeyType
			want string
		}{
			{"ctrl+d", tea.KeyCtrlD, "\x04"},
			{"ctrl+u", tea.KeyCtrlU, "\x15"},
			{"ctrl+f", tea.KeyCtrlF, "\x06"},
			{"ctrl+b", tea.KeyCtrlB, "\x02"},
			// Ctrl-r is the buffer engine's redo. The motion simulator knows no
			// such motion, so before the shared accepted set learned it the redo
			// lesson was unanswerable.
			{"ctrl+r", tea.KeyCtrlR, "\x12"},
		}
		cases := []struct {
			name  string
			model func(*testing.T) Model
		}{
			{"lesson", newTrainerLessonModel},
			{"boss", newTrainerBossModel},
		}

		for _, tc := range cases {
			for _, ck := range controlKeys {
				t.Run(tc.name+"/"+ck.name, func(t *testing.T) {
					m := tc.model(t)
					m.TrainerInput = ""

					result, _ := m.Update(tea.KeyMsg{Type: ck.key})
					m = result.(Model)

					if m.TrainerInput != ck.want {
						t.Errorf("TrainerInput = %q after %s, want %q", m.TrainerInput, ck.name, ck.want)
					}
				})
			}
		}
	})
}

// TestTrainerBackspaceDeletesOneTypedUnit pins the answer input's backspace
// behaviour, which is interface behaviour and not a Vim command: one press
// removes the last unit the player typed. A unit is one keystroke, so the escape
// token inserted by a single ctrl+e comes out whole, rather than leaving the
// half-token "<Es" behind for the engine to reject. The engine never sees a
// backspace at all, which is why insert mode reports one as unrecognized instead
// of deleting a rune.
func TestTrainerBackspaceDeletesOneTypedUnit(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "one typed command", input: "ww", want: "w"},
		{name: "the whole input", input: "w", want: ""},
		{name: "empty input stays empty", input: "", want: ""},
		{name: "the escape token goes as one unit", input: "iX" + trainer.EscToken, want: "iX"},
		{name: "only the escape token", input: "i" + trainer.EscToken, want: "i"},
		{name: "a half token is only text", input: "iX<Es", want: "iX<E"},
		{name: "a token that is not last is untouched", input: "iX" + trainer.EscToken + "aY", want: "iX" + trainer.EscToken + "a"},
	}

	cases := []struct {
		name  string
		model func(*testing.T) Model
	}{
		{"lesson", newTrainerLessonModel},
		{"boss", newTrainerBossModel},
	}

	for _, tc := range cases {
		for _, tt := range tests {
			t.Run(tc.name+"/"+tt.name, func(t *testing.T) {
				m := tc.model(t)
				m.TrainerInput = tt.input

				result, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
				m = result.(Model)

				if m.TrainerInput != tt.want {
					t.Errorf("TrainerInput = %q after backspace, want %q", m.TrainerInput, tt.want)
				}
			})
		}
	}
}

// TestTrainerExerciseHelpNamesTheEscapeToken is the visual half of the token
// contract: the exercise screens must tell the player which key types the token
// the engine parses, and the text they render comes from the engine's own
// constant so the two cannot drift apart. The help must not present backspace as
// a Vim command either, because it is an input edit the engine never sees.
func TestTrainerExerciseHelpNamesTheEscapeToken(t *testing.T) {
	cases := []struct {
		name  string
		model func(*testing.T) Model
	}{
		{"lesson", newTrainerLessonModel},
		{"boss", newTrainerBossModel},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.model(t)
			m.Width = 80
			m.Height = 24

			view := m.View()
			if !strings.Contains(view, "[Ctrl-e] type "+trainer.EscToken) {
				t.Errorf("exercise help does not document ctrl+e typing %s:\n%s", trainer.EscToken, view)
			}
			if !strings.Contains(view, "[Esc] ") {
				t.Errorf("exercise help lost the Esc key:\n%s", view)
			}
		})
	}
}

// =============================================================================
// ANSWER TIMING REGRESSION
// =============================================================================

// newTrainerTimedLessonModel builds a live lesson model whose answer clock is a
// fake the test can advance, so answer timing is exercised without sleeping.
// SetClock runs before StartLesson because presentation is what the clock is
// measured from.
func newTrainerTimedLessonModel(t *testing.T) (Model, *time.Time) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())

	m := NewModel()
	m.Screen = ScreenTrainerLesson
	m.TrainerStats = trainer.NewUserStats()
	m.TrainerGameState = trainer.NewGameStateWithStats(m.TrainerStats)

	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	m.TrainerGameState.SetClock(func() time.Time { return now })
	m.TrainerGameState.StartLesson(trainer.ModuleHorizontal)
	m.TrainerInput = ""
	return m, &now
}

// newTrainerTimedBossModel is newTrainerTimedLessonModel for a boss fight.
func newTrainerTimedBossModel(t *testing.T) (Model, *time.Time) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())

	m := NewModel()
	m.Screen = ScreenTrainerBoss
	m.TrainerStats = trainer.NewUserStats()
	progress := m.TrainerStats.GetModuleProgress(trainer.ModuleHorizontal)
	progress.LessonsCompleted = 15
	progress.LessonsTotal = 15
	progress.PracticeAccuracy = 85.0
	m.TrainerGameState = trainer.NewGameStateWithStats(m.TrainerStats)

	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	m.TrainerGameState.SetClock(func() time.Time { return now })
	m.TrainerGameState.StartBoss(trainer.ModuleHorizontal)
	m.TrainerInput = ""
	return m, &now
}

// submitNow types answer, advances the fake clock by elapsed and presses enter,
// reporting the model the handler produced.
func submitNow(t *testing.T, m Model, now *time.Time, answer string, elapsed time.Duration) Model {
	t.Helper()

	m.TrainerInput = answer
	*now = now.Add(elapsed)

	result, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	return result.(Model)
}

// TestTrainerAnswerTimeReachesScorer pins that the UI hands the time it measured
// from presentation to the scorer instead of a fixed placeholder. Both cases
// answer the same exercise optimally, so the scores can only differ if the
// elapsed time the fake clock reports reaches the score calculation.
func TestTrainerAnswerTimeReachesScorer(t *testing.T) {
	fastModel, now := newTrainerTimedLessonModel(t)
	exercise := fastModel.TrainerGameState.CurrentExercise
	if exercise == nil {
		t.Fatal("no lesson exercise presented")
	}
	fastModel = submitNow(t, fastModel, now, exercise.Optimal, 1500*time.Millisecond)

	slowModel, slowNow := newTrainerTimedLessonModel(t)
	slowExercise := slowModel.TrainerGameState.CurrentExercise
	if slowExercise == nil {
		t.Fatal("no lesson exercise presented")
	}
	if slowExercise.ID != exercise.ID {
		t.Fatalf("test setup: the two runs presented different exercises: %s and %s", exercise.ID, slowExercise.ID)
	}
	slowModel = submitNow(t, slowModel, slowNow, slowExercise.Optimal, 2500*time.Millisecond)

	if fastModel.Screen != ScreenTrainerResult || slowModel.Screen != ScreenTrainerResult {
		t.Fatalf("screens after submitting = %v and %v, want %v", fastModel.Screen, slowModel.Screen, ScreenTrainerResult)
	}

	wantFast := trainer.CalculatePoints(exercise, 1.5, true, 1)
	wantSlow := trainer.CalculatePoints(exercise, 2.5, true, 1)
	if wantFast <= wantSlow {
		t.Fatalf("test setup: the scorer gives no speed bonus for %s (fast %d, slow %d)", exercise.ID, wantFast, wantSlow)
	}

	if fastModel.TrainerGameState.SessionScore != wantFast {
		t.Errorf("SessionScore = %d for a 1.5s answer, want %d: the measured time did not reach the scorer",
			fastModel.TrainerGameState.SessionScore, wantFast)
	}
	if slowModel.TrainerGameState.SessionScore != wantSlow {
		t.Errorf("SessionScore = %d for a 2.5s answer, want %d: the measured time did not reach the scorer",
			slowModel.TrainerGameState.SessionScore, wantSlow)
	}
}

// TestTrainerBossAnswerTimeReachesScorer covers the second answer path. It
// asserts the accumulated TotalTime, the observable that shows the recorder
// received the measured time regardless of how the points land. It also pins
// that a wrong boss answer does not restart the clock: the retry is measured
// from the presentation of the step, so it is honestly slower. Whether a fast
// boss answer earns the speed bonus is pinned separately by
// TestTrainerFastBossAnswerEarnsSpeedBonus.
func TestTrainerBossAnswerTimeReachesScorer(t *testing.T) {
	t.Run("a boss answer hands the measured time to the recorder", func(t *testing.T) {
		m, now := newTrainerTimedBossModel(t)
		exercise := m.TrainerGameState.CurrentExercise
		if exercise == nil {
			t.Fatal("no boss exercise presented")
		}
		if !trainer.ValidateAnswer(exercise, exercise.Optimal) {
			t.Fatalf("test setup: the optimal answer %q is rejected for %s", exercise.Optimal, exercise.ID)
		}

		m = submitNow(t, m, now, exercise.Optimal, 1500*time.Millisecond)

		if m.TrainerStats.TotalTime != 1500*time.Millisecond {
			t.Errorf("TotalTime = %v after a 1.5s boss answer, want 1.5s: the measured time did not reach the recorder",
				m.TrainerStats.TotalTime)
		}
	})

	t.Run("a wrong boss answer does not restart the clock", func(t *testing.T) {
		m, now := newTrainerTimedBossModel(t)
		exercise := m.TrainerGameState.CurrentExercise
		if exercise == nil {
			t.Fatal("no boss exercise presented")
		}

		const wrong = "ZZZZZZ"
		if trainer.ValidateAnswer(exercise, wrong) {
			t.Fatalf("test setup: %q validated against %s", wrong, exercise.ID)
		}

		m = submitNow(t, m, now, wrong, 1500*time.Millisecond)
		if m.Screen != ScreenTrainerBoss {
			t.Fatalf("screen after a wrong boss answer = %v, want %v", m.Screen, ScreenTrainerBoss)
		}

		m = submitNow(t, m, now, exercise.Optimal, 500*time.Millisecond)

		// Two seconds from presentation: the mistaken 1.5s plus the 0.5s retry.
		// A clock restarted by the wrong answer would have recorded half a second.
		const want = 2 * time.Second
		if m.TrainerStats.TotalTime != want {
			t.Errorf("TotalTime = %v after a wrong answer and a fast retry, want %v: the wrong answer restarted the clock",
				m.TrainerStats.TotalTime, want)
		}
	})
}

// =============================================================================
// EXERCISE DEADLINE / AUTOMATIC HINT
// =============================================================================

// tickTrainer delivers one animation tick, the same message the Bubbletea timer
// re-arms every 100ms, and returns the resulting model. The countdown reads the
// injected clock rather than the tick payload, so advancing the fake clock and
// delivering a tick is what moves the deadline in these tests.
func tickTrainer(m Model) Model {
	next, _ := m.Update(tickMsg(time.Now()))
	return next.(Model)
}

// typeRunes types answer one rune at a time, as a user would, so a test can
// prove the exercise still accepts input.
func typeRunes(t *testing.T, m Model, answer string) Model {
	t.Helper()
	for _, r := range answer {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(Model)
	}
	return m
}

// TestTrainerHintIsRevealedWhenDeadlinePasses pins both halves of the deadline
// contract: before the exercise's TimeoutSecs the screen shows the time
// remaining and no hint, and once that deadline passes the hint appears on its
// own while the user is idle.
func TestTrainerHintIsRevealedWhenDeadlinePasses(t *testing.T) {
	m, now := newTrainerTimedLessonModel(t)
	exercise := m.TrainerGameState.CurrentExercise
	if exercise == nil || exercise.TimeoutSecs <= 0 {
		t.Fatalf("test setup: %v is not a timed lesson exercise", exercise)
	}
	if exercise.Hint == "" {
		t.Fatalf("test setup: %s has no hint to reveal", exercise.ID)
	}

	// One second before the deadline: no hint yet, but the countdown is live.
	*now = now.Add(time.Duration(exercise.TimeoutSecs)*time.Second - time.Second)
	m = tickTrainer(m)
	if strings.Contains(m.TrainerMessage, "Hint") {
		t.Errorf("TrainerMessage = %q before the deadline, want no hint", m.TrainerMessage)
	}
	view := m.renderTrainerExercise("Lesson")
	if !strings.Contains(view, "Hint in") {
		t.Errorf("the exercise screen does not show the time remaining before the hint:\n%s", view)
	}

	// Crossing the deadline reveals the hint without a key press.
	*now = now.Add(time.Second)
	m = tickTrainer(m)
	want := "💡 Hint: " + exercise.Hint
	if m.TrainerMessage != want {
		t.Errorf("TrainerMessage = %q after the deadline, want %q", m.TrainerMessage, want)
	}
}

// TestTrainerExpiryLeavesTheExerciseOpenAndAnswerable pins that a passed
// deadline is a hint and not a failure: the hint appears, typing keeps working,
// and a correct answer is still accepted.
func TestTrainerExpiryLeavesTheExerciseOpenAndAnswerable(t *testing.T) {
	m, now := newTrainerTimedLessonModel(t)
	exercise := m.TrainerGameState.CurrentExercise
	if exercise == nil || exercise.TimeoutSecs <= 0 {
		t.Fatalf("test setup: %v is not a timed lesson exercise", exercise)
	}

	// The deadline passes while the user is idle.
	*now = now.Add(time.Duration(exercise.TimeoutSecs+5) * time.Second)
	m = tickTrainer(m)
	if m.Screen != ScreenTrainerLesson {
		t.Fatalf("screen after the deadline = %v, want %v: expiry closed the exercise", m.Screen, ScreenTrainerLesson)
	}
	if !strings.Contains(m.TrainerMessage, "Hint") {
		t.Fatalf("TrainerMessage = %q after the deadline, want the automatic hint", m.TrainerMessage)
	}

	// Typing still works once the hint is on screen.
	m = typeRunes(t, m, exercise.Optimal)
	if m.TrainerInput != exercise.Optimal {
		t.Fatalf("TrainerInput = %q after typing, want %q: the revealed hint blocked typing", m.TrainerInput, exercise.Optimal)
	}

	// And the answer is still accepted after expiry.
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.Screen != ScreenTrainerResult {
		t.Fatalf("screen after answering the expired exercise = %v, want %v", m.Screen, ScreenTrainerResult)
	}
	if !m.TrainerLastCorrect {
		t.Error("the expired exercise rejected a correct answer")
	}
}

// TestTrainerFastBossAnswerEarnsSpeedBonus pins the consequence of removing the
// TimeoutSecs precondition from the speed bonus. The horizontal boss steps
// declare no timeout (only the Change & Repeat boss's five do), so before the
// change a fast and a slow boss answer scored exactly the same; now the
// multiplier follows the measured time alone.
func TestTrainerFastBossAnswerEarnsSpeedBonus(t *testing.T) {
	fast, fastNow := newTrainerTimedBossModel(t)
	exercise := fast.TrainerGameState.CurrentExercise
	if exercise == nil {
		t.Fatal("no boss exercise presented")
	}
	if exercise.TimeoutSecs != 0 {
		t.Fatalf("test setup: boss step %s declares TimeoutSecs %d, the regression needs none", exercise.ID, exercise.TimeoutSecs)
	}

	fast = submitNow(t, fast, fastNow, exercise.Optimal, 1500*time.Millisecond)
	slow, slowNow := newTrainerTimedBossModel(t)
	slow = submitNow(t, slow, slowNow, exercise.Optimal, 2500*time.Millisecond)

	wantFast := trainer.CalculatePoints(exercise, 1.5, true, 1)
	wantSlow := trainer.CalculatePoints(exercise, 2.5, true, 1)
	if wantFast <= wantSlow {
		t.Fatalf("test setup: the scorer gives no speed bonus without a timeout (fast %d, slow %d)", wantFast, wantSlow)
	}

	if fast.TrainerGameState.SessionScore != wantFast {
		t.Errorf("fast boss SessionScore = %d, want %d: a fast boss answer did not earn the speed multiplier",
			fast.TrainerGameState.SessionScore, wantFast)
	}
	if slow.TrainerGameState.SessionScore != wantSlow {
		t.Errorf("slow boss SessionScore = %d, want %d", slow.TrainerGameState.SessionScore, wantSlow)
	}
}

// =============================================================================
// BOSS STEP DEADLINE
// =============================================================================

// TestTrainerBossStepExpiresOnItsDeadline pins the clock the boss fight was
// missing: a step left unanswered past its own BossStep.TimeLimit costs one
// life, shows the solution and leaves the player on that same step to retry it.
// The tick drives the deadline because no key press happens when time runs out.
func TestTrainerBossStepExpiresOnItsDeadline(t *testing.T) {
	m, now := newTrainerTimedBossModel(t)
	state := m.TrainerGameState
	step := state.CurrentBoss.Steps[state.BossStep]
	if step.TimeLimit <= 0 {
		t.Fatalf("test setup: boss step %s declares no TimeLimit", step.Exercise.ID)
	}
	lives := state.BossLives
	solutionHint := trainer.FormatSolutionsHint(&step.Exercise)

	// The player walks away without answering.
	*now = now.Add(time.Duration(step.TimeLimit)*time.Second + time.Second)
	m = tickTrainer(m)

	if got := m.TrainerGameState.BossLives; got != lives-1 {
		t.Errorf("BossLives = %d after the deadline passed, want %d: the clock cost no life", got, lives-1)
	}
	if m.Screen != ScreenTrainerBoss {
		t.Fatalf("screen = %v after the deadline passed, want %v: expiry left the step", m.Screen, ScreenTrainerBoss)
	}
	if got := m.TrainerGameState.BossStep; got != 0 {
		t.Errorf("BossStep = %d after the deadline passed, want 0: expiry advanced the step", got)
	}
	if got := m.TrainerGameState.CurrentExercise; got == nil || got.ID != step.Exercise.ID {
		t.Errorf("current exercise = %v after the deadline passed, want %s: expiry moved the retry to another step", got, step.Exercise.ID)
	}
	if !strings.Contains(m.TrainerMessage, solutionHint) {
		t.Errorf("TrainerMessage = %q after the deadline passed, want the solution %q", m.TrainerMessage, solutionHint)
	}
}

// TestTrainerBossDeadlineCostsOneLifePerWindow is the idempotence guard. The
// 100ms animation tick keeps arriving while the player is away, and the clock is
// held past the deadline for every one of them; a naive "elapsed >= limit"
// check would charge a life on each tick and drain the whole fight in under a
// second.
func TestTrainerBossDeadlineCostsOneLifePerWindow(t *testing.T) {
	m, now := newTrainerTimedBossModel(t)
	state := m.TrainerGameState
	limit := state.CurrentBoss.Steps[state.BossStep].TimeLimit
	lives := state.BossLives

	// Past the deadline and left there while the tick repeats.
	*now = now.Add(time.Duration(limit)*time.Second + time.Second)
	const ticks = 20
	for i := 0; i < ticks; i++ {
		m = tickTrainer(m)
	}

	if got := m.TrainerGameState.BossLives; got != lives-1 {
		t.Errorf("BossLives = %d after %d ticks past one deadline, want %d: the deadline charged once per tick",
			got, ticks, lives-1)
	}
}

// TestTrainerBossExpiryKeepsTheStepAndRestartsTheWindow pins that expiry is a
// retry and not an advance: the player stays on the failed step, and the
// deadline restarts so the retry gets a full TimeLimit rather than whatever was
// left when the first window ran out.
func TestTrainerBossExpiryKeepsTheStepAndRestartsTheWindow(t *testing.T) {
	m, now := newTrainerTimedBossModel(t)
	state := m.TrainerGameState
	step := state.CurrentBoss.Steps[state.BossStep]
	lives := state.BossLives

	// First window runs out.
	*now = now.Add(time.Duration(step.TimeLimit) * time.Second)
	m = tickTrainer(m)
	if got := m.TrainerGameState.BossLives; got != lives-1 {
		t.Fatalf("BossLives = %d at the deadline, want %d", got, lives-1)
	}
	if m.TrainerGameState.BossStep != 0 {
		t.Fatalf("BossStep = %d at the deadline, want 0: expiry advanced the step", m.TrainerGameState.BossStep)
	}
	if got := m.TrainerGameState.CurrentExercise; got == nil || got.ID != step.Exercise.ID {
		t.Fatalf("current exercise after expiry = %v, want %s", got, step.Exercise.ID)
	}

	// Ticks on the same instant, and the restarted window minus one second, are
	// still inside the fresh window.
	for i := 0; i < 5; i++ {
		m = tickTrainer(m)
	}
	*now = now.Add(time.Duration(step.TimeLimit)*time.Second - time.Second)
	for i := 0; i < 5; i++ {
		m = tickTrainer(m)
	}
	if got := m.TrainerGameState.BossLives; got != lives-1 {
		t.Fatalf("BossLives = %d before the restarted window elapsed, want %d: the retry did not get a full window",
			got, lives-1)
	}

	// Crossing the restarted deadline costs the next life.
	*now = now.Add(time.Second)
	m = tickTrainer(m)
	if got := m.TrainerGameState.BossLives; got != lives-2 {
		t.Errorf("BossLives = %d after the restarted deadline, want %d", got, lives-2)
	}
}

// TestTrainerBossWrongAnswerRearmsTheWindow is the regression for the life that
// cost two: a wrong answer inside the window did not re-arm the step deadline,
// so the player lost a life for the mistake and then lost a second one when that
// same window expired, without ever getting a fresh window for the retry. A lost
// life is now the single owner of the re-arm, so the wrong-answer path leaves
// the same full window the expiry path does.
func TestTrainerBossWrongAnswerRearmsTheWindow(t *testing.T) {
	m, now := newTrainerTimedBossModel(t)
	state := m.TrainerGameState
	step := state.CurrentBoss.Steps[state.BossStep]
	limit := step.TimeLimit
	if limit <= 0 {
		t.Fatalf("test setup: boss step %s declares no TimeLimit", step.Exercise.ID)
	}
	lives := state.BossLives

	// Answer wrong one second before the original deadline.
	const wrong = "ZZZZZZ"
	if trainer.ValidateAnswer(&step.Exercise, wrong) {
		t.Fatalf("test setup: %q validated against %s", wrong, step.Exercise.ID)
	}
	m = submitNow(t, m, now, wrong, time.Duration(limit-1)*time.Second)
	if got := m.TrainerGameState.BossLives; got != lives-1 {
		t.Fatalf("BossLives = %d after a wrong answer, want %d", got, lives-1)
	}

	// Drive ticks across the original deadline. The spent life must have re-armed
	// the window, so exactly one life is gone in total and the retry is still
	// open.
	*now = now.Add(time.Second)
	for i := 0; i < 20; i++ {
		m = tickTrainer(m)
	}
	if got := m.TrainerGameState.BossLives; got != lives-1 {
		t.Errorf("BossLives = %d after ticks across the original deadline, want %d: the wrong answer did not re-arm the window and charged a second life",
			got, lives-1)
	}
	if m.Screen != ScreenTrainerBoss {
		t.Fatalf("screen = %v after the original deadline, want %v: the retry window closed", m.Screen, ScreenTrainerBoss)
	}
	if got := m.TrainerGameState.BossStepSecondsLeft(); got <= 0 {
		t.Errorf("BossStepSecondsLeft = %v after the original deadline, want a live retry window", got)
	}

	// Crossing the retry window itself costs exactly one more life.
	*now = now.Add(time.Duration(limit) * time.Second)
	m = tickTrainer(m)
	if got := m.TrainerGameState.BossLives; got != lives-2 {
		t.Errorf("BossLives = %d after the retry window, want %d", got, lives-2)
	}
}

// TestTrainerBossDeadlineOffTheBossScreenCostsNoLife pins the screen guard that
// keeps a re-armed deadline from charging once the fight has left the boss
// screen. It matters because losing the last life re-arms the window and then
// moves to the defeat screen, so without the guard the very next tick would
// spend a life the fight no longer has. BossLives clamps at zero, so the boss
// attempt count is the observable that would move.
func TestTrainerBossDeadlineOffTheBossScreenCostsNoLife(t *testing.T) {
	m, now := newTrainerTimedBossModel(t)
	state := m.TrainerGameState
	step := state.CurrentBoss.Steps[state.BossStep]
	if step.TimeLimit <= 0 {
		t.Fatalf("test setup: boss step %s declares no TimeLimit", step.Exercise.ID)
	}
	state.BossLives = 1

	// The clock takes the last life and the fight moves to the defeat screen.
	*now = now.Add(time.Duration(step.TimeLimit)*time.Second + time.Second)
	m = tickTrainer(m)
	if m.Screen != ScreenTrainerBossResult {
		t.Fatalf("screen after the last life = %v, want %v", m.Screen, ScreenTrainerBossResult)
	}
	progress := m.TrainerGameState.Stats.GetModuleProgress(trainer.ModuleHorizontal)
	attempts := progress.BossAttempts
	if attempts != 1 {
		t.Fatalf("BossAttempts = %d after the last life, want 1", attempts)
	}

	// Ticks well past the re-armed deadline are inert off the boss screen.
	*now = now.Add(time.Duration(step.TimeLimit)*time.Second + time.Second)
	for i := 0; i < 20; i++ {
		m = tickTrainer(m)
	}
	if got := m.TrainerGameState.Stats.GetModuleProgress(trainer.ModuleHorizontal).BossAttempts; got != attempts {
		t.Errorf("BossAttempts = %d after ticks off the boss screen, want %d: the re-armed deadline kept charging after the fight left the screen",
			got, attempts)
	}
	if m.Screen != ScreenTrainerBossResult {
		t.Errorf("screen = %v after ticks off the boss screen, want %v", m.Screen, ScreenTrainerBossResult)
	}
}

// TestTrainerUntimedBossStepNeverExpires pins the data guard on the tick path: a
// boss step whose TimeLimit is zero gets no deadline at all, so no amount of
// elapsed time can charge a life. The state-level predicate is pinned by
// TestGameState_UntimedBossStepHasNoDeadline; this drives the same guard through
// expireBossStepOnDeadline, which is what the animation tick actually runs.
func TestTrainerUntimedBossStepNeverExpires(t *testing.T) {
	m, now := newTrainerTimedBossModel(t)
	state := m.TrainerGameState

	// Every real module gives each step a TimeLimit, so present an untimed step
	// through the exported advance path to reach the guard.
	state.CurrentBoss = &trainer.BossExercise{
		ID:     "untimed_boss",
		Module: trainer.ModuleHorizontal,
		Lives:  3,
		Steps: []trainer.BossStep{{
			Exercise: trainer.Exercise{
				ID:        "untimed_step",
				Module:    trainer.ModuleHorizontal,
				Optimal:   "w",
				Solutions: []string{"w"},
			},
		}},
	}
	state.BossStep = -1
	state.BossLives = 3
	if !state.NextBossExercise() {
		t.Fatal("test setup: the untimed step was not presented")
	}
	if limit := state.BossStepTimeLimit(); limit != 0 {
		t.Fatalf("test setup: the untimed step reports a limit of %d", limit)
	}
	attempts := state.Stats.GetModuleProgress(trainer.ModuleHorizontal).BossAttempts

	// A long idle stretch with the tick still arriving.
	*now = now.Add(10 * time.Minute)
	for i := 0; i < 20; i++ {
		m = tickTrainer(m)
	}

	if got := m.TrainerGameState.BossLives; got != 3 {
		t.Errorf("BossLives = %d after 10 minutes on a step with no TimeLimit, want 3", got)
	}
	if got := m.TrainerGameState.Stats.GetModuleProgress(trainer.ModuleHorizontal).BossAttempts; got != attempts {
		t.Errorf("BossAttempts = %d after 10 minutes on a step with no TimeLimit, want %d", got, attempts)
	}
	if m.Screen != ScreenTrainerBoss {
		t.Errorf("screen = %v after 10 minutes on a step with no TimeLimit, want %v", m.Screen, ScreenTrainerBoss)
	}
}

// TestTrainerBossWonStepGrantsItsBonusToTheNextStep pins the bonus by its
// effect on the clock: after winning a step, the next step is answerable for
// TimeLimit+BonusTime seconds and not for the bare TimeLimit. The clock is
// advanced to one second before that effective window and then across it, so a
// bonus that was stored but never applied fails the last assertion.
func TestTrainerBossWonStepGrantsItsBonusToTheNextStep(t *testing.T) {
	m, now := newTrainerTimedBossModel(t)
	state := m.TrainerGameState
	boss := state.CurrentBoss
	if boss.BonusTime <= 0 {
		t.Fatalf("test setup: %s declares no BonusTime", boss.ID)
	}
	first := boss.Steps[state.BossStep]
	next := boss.Steps[state.BossStep+1]
	wantWindow := time.Duration(next.TimeLimit+boss.BonusTime) * time.Second
	if wantWindow <= time.Duration(next.TimeLimit)*time.Second {
		t.Fatalf("test setup: the bonus does not extend step %s (%ds + %ds)", next.Exercise.ID, next.TimeLimit, boss.BonusTime)
	}

	// Win the first step without any time pressure.
	m = submitNow(t, m, now, first.Exercise.Optimal, time.Second)
	if m.TrainerGameState.BossStep != 1 {
		t.Fatalf("BossStep = %d after winning the first step, want 1", m.TrainerGameState.BossStep)
	}

	// Inside the effective window the next step is safe...
	*now = now.Add(wantWindow - time.Second)
	m = tickTrainer(m)
	if got := m.TrainerGameState.BossLives; got != boss.Lives {
		t.Fatalf("BossLives = %d one second before the effective window (%v), want %d", got, wantWindow, boss.Lives)
	}

	// ...and crossing it is what costs the life, so the window really is
	// TimeLimit+BonusTime.
	*now = now.Add(time.Second)
	m = tickTrainer(m)
	if got := m.TrainerGameState.BossLives; got != boss.Lives-1 {
		t.Errorf("BossLives = %d after the effective window %v, want %d: the won step's bonus never reached the next step",
			got, wantWindow, boss.Lives-1)
	}
}

// TestTrainerBossLastLifeLostToTheClockEndsTheFight pins that the clock and a
// wrong answer cost the last life identically: same result screen, same defeat
// message, and IsBossDefeated still false because the player did not win.
func TestTrainerBossLastLifeLostToTheClockEndsTheFight(t *testing.T) {
	m, now := newTrainerTimedBossModel(t)
	state := m.TrainerGameState
	limit := state.CurrentBoss.Steps[state.BossStep].TimeLimit
	state.BossLives = 1

	*now = now.Add(time.Duration(limit)*time.Second + time.Second)
	m = tickTrainer(m)

	if got := m.TrainerGameState.BossLives; got != 0 {
		t.Fatalf("BossLives = %d after the last life, want 0", got)
	}
	if m.TrainerLastCorrect {
		t.Error("TrainerLastCorrect = true after losing the last life to the clock")
	}
	if m.Screen != ScreenTrainerBossResult {
		t.Fatalf("screen = %v after losing the last life to the clock, want %v", m.Screen, ScreenTrainerBossResult)
	}
	if !strings.Contains(m.TrainerMessage, "DEFEATED") {
		t.Errorf("TrainerMessage = %q after losing the last life to the clock, want the defeat message", m.TrainerMessage)
	}
	if m.TrainerGameState.IsBossDefeated {
		t.Error("IsBossDefeated = true after losing the fight to the clock; it must mean the player won")
	}
}

// TestTrainerBossAnswerInsideTheLimitCostsNoLife is the anti-regression half of
// the deadline: an answer submitted before the step's TimeLimit loses nothing
// and advances the fight, and the following tick cannot retroactively charge
// for the answered step.
func TestTrainerBossAnswerInsideTheLimitCostsNoLife(t *testing.T) {
	m, now := newTrainerTimedBossModel(t)
	state := m.TrainerGameState
	step := state.CurrentBoss.Steps[state.BossStep]
	if step.TimeLimit <= 0 {
		t.Fatalf("test setup: boss step %s declares no TimeLimit", step.Exercise.ID)
	}
	lives := state.BossLives

	// One second inside the limit, then a tick on the same moment.
	m = submitNow(t, m, now, step.Exercise.Optimal, time.Duration(step.TimeLimit)*time.Second-time.Second)
	m = tickTrainer(m)

	if got := m.TrainerGameState.BossLives; got != lives {
		t.Errorf("BossLives = %d after answering inside the limit, want %d", got, lives)
	}
	if got := m.TrainerGameState.BossStep; got != 1 {
		t.Errorf("BossStep = %d after a correct answer inside the limit, want 1", got)
	}
	if m.Screen != ScreenTrainerBoss {
		t.Errorf("screen = %v after answering inside the limit, want %v", m.Screen, ScreenTrainerBoss)
	}
}

// TestTrainerBossScreenShowsTheStepCountdown pins requirement three: the boss
// screen shows the time left for the current step, in the same visual language
// the exercise screen uses, and the number follows the injected clock.
func TestTrainerBossScreenShowsTheStepCountdown(t *testing.T) {
	m, now := newTrainerTimedBossModel(t)
	limit := m.TrainerGameState.CurrentBoss.Steps[m.TrainerGameState.BossStep].TimeLimit

	view := m.renderTrainerBoss()
	if want := fmt.Sprintf("⏳ Time left: %ds", limit); !strings.Contains(view, want) {
		t.Errorf("the boss screen does not show %q:\n%s", want, view)
	}

	// The countdown reads the injected clock, so it drops as time passes.
	*now = now.Add(2 * time.Second)
	view = m.renderTrainerBoss()
	if want := fmt.Sprintf("⏳ Time left: %ds", limit-2); !strings.Contains(view, want) {
		t.Errorf("the boss countdown did not follow the clock, want %q:\n%s", want, view)
	}
}

// =============================================================================
// TRAINER MENU PROGRESS
// =============================================================================

// seedTrainerMenuProgress gives the trainer menu a profile with known progress:
// Horizontal lessons partly done, two exercises mastered, its boss defeated,
// and two exercises answered wrong (the first more often than the second, so
// the weakest list has an order to check).
func seedTrainerMenuProgress(t *testing.T, m *Model) {
	t.Helper()

	progress := m.TrainerStats.GetModuleProgress(trainer.ModuleHorizontal)
	progress.LessonsCompleted = 7
	progress.LessonsTotal = 15

	lessons := trainer.GetLessons(trainer.ModuleHorizontal)
	if len(lessons) < 4 {
		t.Fatalf("need at least 4 horizontal lessons, got %d", len(lessons))
	}

	// Master the first two exercises.
	for i := 0; i < 2; i++ {
		for j := 0; j < trainer.MasteryThreshold; j++ {
			progress.RecordPracticeResult(lessons[i].ID, true)
		}
	}

	// The third exercise is missed most, the fourth once.
	for i := 0; i < 3; i++ {
		progress.RecordPracticeResult(lessons[2].ID, false)
	}
	progress.RecordPracticeResult(lessons[3].ID, false)

	m.TrainerStats.BossesDefeated = append(m.TrainerStats.BossesDefeated, trainer.ModuleHorizontal)
	progress.BossDefeated = true
}

// newTrainerProgressModel parks the model on the trainer menu with the seeded
// progress profile above.
func newTrainerProgressModel(t *testing.T) Model {
	t.Helper()
	t.Setenv("HOME", t.TempDir())

	m := NewModel()
	m.Width = 80
	m.Height = 24
	m.Screen = ScreenTrainerMenu
	m.TrainerStats = trainer.NewUserStats()
	m.TrainerModules = trainer.GetAllModules()
	m.TrainerCursor = 0
	seedTrainerMenuProgress(t, &m)

	return m
}

// TestTrainerMenuShowsModuleProgress pins that the menu renders the seeded
// numbers for each module: lessons completed against total, mastered
// exercises, and whether the boss is defeated.
func TestTrainerMenuShowsModuleProgress(t *testing.T) {
	m := newTrainerProgressModel(t)
	view := m.View()

	lessons := trainer.GetLessons(trainer.ModuleHorizontal)
	want := []string{
		"Lessons 7/15",
		fmt.Sprintf("Mastered 2/%d", len(lessons)),
		"Boss ✓",
		"Acc 60%",
	}
	for _, w := range want {
		if !strings.Contains(view, w) {
			t.Errorf("trainer menu does not show %q:\n%s", w, view)
		}
	}
}

// TestTrainerMenuRenderIsReadOnly pins that merely opening the trainer menu does
// not manufacture records in the player's profile. The menu asks every module
// whether its lessons are complete and whether practice and the boss are ready,
// so the predicates behind those questions must look the progress up without
// creating an empty MODULE record (the exercise half was made read-only in the
// previous slice). Measured the way the defect was found: render on a fresh
// profile, then check both memory and the saved file.
func TestTrainerMenuRenderIsReadOnly(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	m := NewModel()
	m.Width = 80
	m.Height = 24
	m.Screen = ScreenTrainerMenu
	m.TrainerStats = trainer.NewUserStats()
	m.TrainerModules = trainer.GetAllModules()
	m.TrainerCursor = 0

	// A fresh profile saved before the render is the baseline the post-render
	// save must match byte for byte: opening the menu may not gain it anything.
	if err := trainer.SaveStats(m.TrainerStats); err != nil {
		t.Fatalf("seeding a fresh profile failed: %v", err)
	}
	before := readTrainerStatsFile(t)

	_ = m.View()

	if got := len(m.TrainerStats.ModuleProgress); got != 0 {
		t.Errorf("rendering the menu created %d module records in memory, want 0", got)
	}
	if got := countTrainerExerciseRecords(m.TrainerStats); got != 0 {
		t.Errorf("rendering the menu created %d exercise records in memory, want 0", got)
	}

	if err := trainer.SaveStats(m.TrainerStats); err != nil {
		t.Fatalf("saving after the render failed: %v", err)
	}
	if after := readTrainerStatsFile(t); after != before {
		t.Errorf("rendering the menu changed the saved profile:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func readTrainerStatsFile(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(trainer.GetStatsPath())
	if err != nil {
		t.Fatalf("reading the trainer stats file failed: %v", err)
	}
	return string(data)
}

func countTrainerExerciseRecords(stats *trainer.UserStats) int {
	count := 0
	for _, progress := range stats.ModuleProgress {
		count += len(progress.ExerciseStats)
	}
	return count
}

// TestTrainerMenuFitsEightyColumns pins the menu inside the 80x24 frame it
// documents. The layout sizes itself from its longest line, so a line over 80
// columns would wrap on the real terminal and break the screen. The selected
// module carries its practice accuracy, so the worst case is the longest module
// name with a three-digit accuracy.
func TestTrainerMenuFitsEightyColumns(t *testing.T) {
	cases := []struct {
		name     string
		accuracy float64
	}{
		{"the seeded sixty percent", 0.60},
		{"a three-digit accuracy", 1.00},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newTrainerProgressModel(t)
			m.TrainerStats.GetModuleProgress(trainer.ModuleHorizontal).PracticeAccuracy = tc.accuracy

			for _, line := range strings.Split(m.View(), "\n") {
				if got := lipgloss.Width(line); got > 80 {
					t.Errorf("trainer menu line is %d columns wide, want <= 80: %q", got, line)
				}
			}
		})
	}
}

// TestTrainerMenuShowsUnopenedModuleLessonTotal pins that a module that has
// never been opened still shows its real lesson total instead of 0/0, which
// would contradict the mastery count shown next to it.
func TestTrainerMenuShowsUnopenedModuleLessonTotal(t *testing.T) {
	m := newTrainerProgressModel(t)
	view := m.View()

	lessons := trainer.GetLessons(trainer.ModuleVertical)
	if len(lessons) == 0 {
		t.Fatal("no vertical lessons available")
	}
	want := fmt.Sprintf("Lessons 0/%d", len(lessons))
	if !strings.Contains(view, want) {
		t.Errorf("unopened module does not show %q:\n%s", want, view)
	}
}

// TestTrainerMenuShowsWeakestExercisesInOrder pins that the menu surfaces the
// exercises answered wrong most often, most-missed first, and labels them.
func TestTrainerMenuShowsWeakestExercisesInOrder(t *testing.T) {
	m := newTrainerProgressModel(t)
	view := m.View()

	lessons := trainer.GetLessons(trainer.ModuleHorizontal)
	mostMissed := lessons[2].ID
	lessMissed := lessons[3].ID

	mostIdx := strings.Index(view, mostMissed)
	lessIdx := strings.Index(view, lessMissed)
	if mostIdx == -1 || lessIdx == -1 {
		t.Fatalf("weakest exercises missing from menu (most=%d, less=%d):\n%s", mostIdx, lessIdx, view)
	}
	if mostIdx > lessIdx {
		t.Errorf("weakest exercises out of order: %s should precede %s:\n%s", mostMissed, lessMissed, view)
	}
	if !strings.Contains(view, "Weakest:") {
		t.Errorf("trainer menu does not label the weakest exercises:\n%s", view)
	}
}

// TestTrainerWeakExerciseTextStaysWithinWidth pins that the weakest-exercises
// line never grows past the 80-column terminal's inner width, even with three
// long identifiers and their wrong counts. The list degrades by dropping the
// least-missed entries instead of wrapping and breaking the layout.
func TestTrainerWeakExerciseTextStaysWithinWidth(t *testing.T) {
	m := newTrainerProgressModel(t)
	progress := m.TrainerStats.GetModuleProgress(trainer.ModuleHorizontal)
	lessons := trainer.GetLessons(trainer.ModuleHorizontal)
	if len(lessons) < 5 {
		t.Fatalf("need at least 5 horizontal lessons, got %d", len(lessons))
	}

	// Give a third exercise misses so the list has three entries.
	progress.RecordPracticeResult(lessons[4].ID, false)

	practice := trainer.GetPracticeStatsForModule(trainer.ModuleHorizontal, progress)
	if len(practice.WeakestExercises) != 3 {
		t.Fatalf("expected 3 weak exercises, got %d", len(practice.WeakestExercises))
	}

	const innerWidth = 80 - 4 // global left/right padding
	line := trainerWeakExerciseText(progress, practice, innerWidth)
	if w := lipgloss.Width(line); w > innerWidth {
		t.Errorf("weakest-exercises line is %d columns wide, want <= %d: %q", w, innerWidth, line)
	}
	// The most-missed exercise must survive the degradation.
	if !strings.Contains(line, lessons[2].ID) {
		t.Errorf("weakest-exercises line dropped the most-missed exercise: %q", line)
	}
}

// TestTrainerMenuEmptyStatsOmitsWeakestList pins that an empty profile renders
// a sane menu without the weakest-exercises list.
func TestTrainerMenuEmptyStatsOmitsWeakestList(t *testing.T) {
	m := NewModel()
	m.Width = 80
	m.Height = 24
	m.Screen = ScreenTrainerMenu
	m.TrainerStats = trainer.NewUserStats()
	m.TrainerModules = trainer.GetAllModules()
	m.TrainerCursor = 0

	view := m.View()
	if strings.Contains(view, "Weakest:") {
		t.Errorf("an empty profile should not render a weakest-exercises list:\n%s", view)
	}
	if !strings.Contains(view, "Horizontal Motions") {
		t.Errorf("empty stats should still render the module list:\n%s", view)
	}
}

// TestTrainerMenuUnreadableStatsRendersSaneMenu pins that a nil stats value
// (what LoadStats returns for a missing or unreadable file) renders the module
// list instead of panicking or showing junk.
func TestTrainerMenuUnreadableStatsRendersSaneMenu(t *testing.T) {
	m := NewModel()
	m.Width = 80
	m.Height = 24
	m.Screen = ScreenTrainerMenu
	m.TrainerStats = nil
	m.TrainerModules = trainer.GetAllModules()
	m.TrainerCursor = 0

	view := m.View()
	if !strings.Contains(view, "Horizontal Motions") {
		t.Errorf("nil stats should still render the module list:\n%s", view)
	}
	if strings.Contains(view, "Weakest:") {
		t.Errorf("nil stats should not render a weakest-exercises list:\n%s", view)
	}
}

// TestTrainerMenuNavigationUnchanged pins that the progress text is
// display-only: the number of selectable entries is the same, the cursor still
// clamps at both ends, and selecting an entry still starts that module.
func TestTrainerMenuNavigationUnchanged(t *testing.T) {
	m := newTrainerProgressModel(t)

	if got := len(m.TrainerModules); got != 8 {
		t.Fatalf("selectable module count changed: got %d, want 7", got)
	}

	down := func(m Model) Model {
		res, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		return res.(Model)
	}
	up := func(m Model) Model {
		res, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
		return res.(Model)
	}

	m = down(m)
	if m.TrainerCursor != 1 {
		t.Errorf("j moved the cursor to %d, want 1", m.TrainerCursor)
	}
	m = up(m)
	if m.TrainerCursor != 0 {
		t.Errorf("k moved the cursor to %d, want 0", m.TrainerCursor)
	}
	m = up(m)
	if m.TrainerCursor != 0 {
		t.Errorf("k at the top moved the cursor to %d, want 0", m.TrainerCursor)
	}
	for i := 0; i < 10; i++ {
		m = down(m)
	}
	if want := len(m.TrainerModules) - 1; m.TrainerCursor != want {
		t.Errorf("j at the bottom moved the cursor to %d, want %d", m.TrainerCursor, want)
	}

	// Selecting still starts the lesson for the module under the cursor.
	m.TrainerCursor = 0
	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(Model)
	if m.Screen != ScreenTrainerLesson {
		t.Errorf("selecting Horizontal reached screen %v, want %v", m.Screen, ScreenTrainerLesson)
	}
	if m.TrainerGameState == nil || m.TrainerGameState.CurrentModule != trainer.ModuleHorizontal {
		t.Errorf("selecting index 0 did not start the Horizontal module")
	}
}

// =============================================================================
// WHOLE-PROFILE RESET
// =============================================================================

// trainerHasProgress reports whether a profile holds anything a whole-profile
// reset must erase. The seeded menu profile sets none of the top-level counters,
// so the check also looks at the per-module records and the defeated bosses.
func trainerHasProgress(stats *trainer.UserStats) bool {
	if stats == nil {
		return false
	}
	if stats.TotalScore != 0 || stats.CurrentStreak != 0 || stats.BestStreak != 0 {
		return true
	}
	return len(stats.ModuleProgress) != 0 || len(stats.BossesDefeated) != 0
}

// trainerResetKeyMsg is the key press that arms and confirms the whole-profile
// reset. Shifted R is a distinct rune from the per-module [r], so bubbletea
// delivers it without a modifier chord.
func trainerResetKeyMsg() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}}
}

// TestTrainerResetAllArmsWithoutClearing pins that a single press of the reset
// key only arms the reset and says what it is waiting for: the profile, in
// memory and on disk, must be untouched.
func TestTrainerResetAllArmsWithoutClearing(t *testing.T) {
	m := newTrainerProgressModel(t)
	m.TrainerStats.TotalScore = 250
	if err := trainer.SaveStats(m.TrainerStats); err != nil {
		t.Fatalf("seeding stats failed: %v", err)
	}

	res, _ := m.Update(trainerResetKeyMsg())
	m = res.(Model)

	if !trainerHasProgress(m.TrainerStats) {
		t.Fatal("the first reset key press cleared the in-memory profile")
	}
	if onDisk := trainer.LoadStats(); !trainerHasProgress(onDisk) {
		t.Fatal("the first reset key press cleared the profile on disk")
	}
	if !strings.Contains(m.TrainerMessage, "[R]") || !strings.Contains(strings.ToLower(m.TrainerMessage), "again") {
		t.Errorf("armed reset does not say it is waiting for a confirmation: %q", m.TrainerMessage)
	}
}

// TestTrainerResetAllConfirmingClearsProfile pins that the second press clears
// the whole profile: the counters are zero in memory and the stats file on disk
// holds the erased state, so the menu reflects it without a restart.
func TestTrainerResetAllConfirmingClearsProfile(t *testing.T) {
	m := newTrainerProgressModel(t)
	m.TrainerStats.TotalScore = 250
	m.TrainerStats.BestStreak = 9
	if err := trainer.SaveStats(m.TrainerStats); err != nil {
		t.Fatalf("seeding stats failed: %v", err)
	}

	res, _ := m.Update(trainerResetKeyMsg())
	m = res.(Model)
	res, _ = m.Update(trainerResetKeyMsg())
	m = res.(Model)

	if trainerHasProgress(m.TrainerStats) {
		t.Errorf("profile still has progress in memory after confirming: %+v", m.TrainerStats)
	}

	onDisk := trainer.LoadStats()
	if onDisk == nil {
		t.Fatal("no stats file after confirming; the cleared state was not persisted")
	}
	if trainerHasProgress(onDisk) {
		t.Errorf("stats file still holds progress after confirming: %+v", onDisk)
	}

	// The menu must reflect the wipe from the same model, without a restart.
	view := m.View()
	if strings.Contains(view, "Bosses: 1/8") {
		t.Errorf("menu still shows the erased boss count after confirming:\n%s", view)
	}
	if !strings.Contains(view, "Bosses: 0/8") {
		t.Errorf("menu does not show the cleared boss count after confirming:\n%s", view)
	}
}

// TestTrainerResetAllCancelKeepsProfile pins that any other key disarms the
// reset and leaves the profile alone, so a single stray keystroke can neither
// clear nor half-commit the wipe, and the next reset key press only re-arms.
func TestTrainerResetAllCancelKeepsProfile(t *testing.T) {
	m := newTrainerProgressModel(t)
	m.TrainerStats.TotalScore = 250
	if err := trainer.SaveStats(m.TrainerStats); err != nil {
		t.Fatalf("seeding stats failed: %v", err)
	}

	res, _ := m.Update(trainerResetKeyMsg())
	m = res.(Model)
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = res.(Model)

	if !trainerHasProgress(m.TrainerStats) {
		t.Error("an unrelated key cleared the in-memory profile")
	}
	if onDisk := trainer.LoadStats(); !trainerHasProgress(onDisk) {
		t.Error("an unrelated key cleared the profile on disk")
	}

	// Cancelling must disarm: the next reset key press only arms again.
	res, _ = m.Update(trainerResetKeyMsg())
	m = res.(Model)
	if !trainerHasProgress(m.TrainerStats) {
		t.Error("a single reset key press after a cancel cleared the profile; the cancel did not disarm")
	}
}

// TestTrainerResetAllEscapeCancels pins that escape disarms the pending reset
// instead of leaving it armed behind the menu, and never clears the profile.
func TestTrainerResetAllEscapeCancels(t *testing.T) {
	m := newTrainerProgressModel(t)
	m.TrainerStats.TotalScore = 250
	if err := trainer.SaveStats(m.TrainerStats); err != nil {
		t.Fatalf("seeding stats failed: %v", err)
	}

	res, _ := m.Update(trainerResetKeyMsg())
	m = res.(Model)
	res, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = res.(Model)

	if strings.Contains(m.TrainerMessage, "[R]") {
		t.Errorf("escape left the reset armed: message = %q", m.TrainerMessage)
	}
	if onDisk := trainer.LoadStats(); !trainerHasProgress(onDisk) {
		t.Error("escape cleared the profile on disk")
	}
}

// TestTrainerResetAllWithMissingProfile pins that the reset path does not panic
// when the stats file is missing or unreadable (LoadStats returns nil) and that
// it still leaves a clean, empty profile behind.
func TestTrainerResetAllWithMissingProfile(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	m := NewModel()
	m.Screen = ScreenTrainerMenu
	m.TrainerStats = nil // what LoadStats returns for a missing or corrupt file
	m.TrainerModules = trainer.GetAllModules()
	m.TrainerCursor = 0

	res, _ := m.Update(trainerResetKeyMsg())
	m = res.(Model)
	res, _ = m.Update(trainerResetKeyMsg())
	m = res.(Model)

	if m.TrainerStats == nil {
		t.Fatal("confirming with a missing profile left TrainerStats nil")
	}
	if trainerHasProgress(m.TrainerStats) {
		t.Errorf("cleared profile is not empty: %+v", m.TrainerStats)
	}
}

// TestTrainerMenuHelpMentionsResetKeys pins that both reset keys are
// discoverable from the menu's help line.
func TestTrainerMenuHelpMentionsResetKeys(t *testing.T) {
	m := newTrainerProgressModel(t)
	view := m.View()

	for _, want := range []string{"[r] reset module", "[R] reset all"} {
		if !strings.Contains(view, want) {
			t.Errorf("trainer menu help does not mention %q:\n%s", want, view)
		}
	}
}

// TestTrainerModuleResetKeyStillScopedToModule pins that the existing [r]
// shortcut keeps clearing only the selected module's practice data: the module
// keeps its lessons and boss, and the other modules are untouched.
func TestTrainerModuleResetKeyStillScopedToModule(t *testing.T) {
	m := newTrainerProgressModel(t)
	selected := m.TrainerModules[m.TrainerCursor] // Horizontal
	other := m.TrainerModules[1]                  // Vertical

	otherProgress := m.TrainerStats.GetModuleProgress(other.ID)
	otherProgress.BossDefeated = true

	horizontal := m.TrainerStats.ModuleProgress[selected.ID]
	if len(horizontal.ExerciseStats) == 0 {
		t.Fatal("seed did not create exercise stats")
	}
	lessonsBefore := horizontal.LessonsCompleted
	bossBefore := horizontal.BossDefeated

	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m = res.(Model)

	horizontal = m.TrainerStats.ModuleProgress[selected.ID]
	if len(horizontal.ExerciseStats) != 0 {
		t.Errorf("module reset left %d exercise records behind", len(horizontal.ExerciseStats))
	}
	if horizontal.LessonsCompleted != lessonsBefore || horizontal.BossDefeated != bossBefore {
		t.Error("module reset changed the selected module's lessons or boss")
	}
	if got := m.TrainerStats.ModuleProgress[other.ID]; got == nil || !got.BossDefeated {
		t.Error("module reset touched another module")
	}
}

// =============================================================================
// BUFFER-VERIFIED JUDGING THROUGH THE UI
// =============================================================================

// bufferVerifiedLessonModel builds a live lesson around an opted-in exercise
// whose optimal is "x" on a two-rune buffer, so an answer can reach the same
// result by different keys. The exercise is installed directly because the
// shipped corpus deliberately does not opt in yet: the migration guard in the
// trainer package pins that.
func bufferVerifiedLessonModel(t *testing.T) Model {
	t.Helper()
	t.Setenv("HOME", t.TempDir())

	m := NewModel()
	m.Width = 80
	m.Height = 24
	m.Screen = ScreenTrainerLesson
	m.TrainerStats = trainer.NewUserStats()
	m.TrainerGameState = trainer.NewGameStateWithStats(m.TrainerStats)
	m.TrainerGameState.StartLesson(trainer.ModuleHorizontal)

	m.TrainerGameState.CurrentExercise = &trainer.Exercise{
		ID:             "buffer_ui_001",
		Module:         trainer.ModuleChangeRepeat,
		Level:          1,
		Type:           trainer.ExerciseLesson,
		Code:           []string{"ab"},
		CursorPos:      trainer.Position{Line: 0, Col: 0},
		Mission:        "Delete the first character",
		Solutions:      []string{"x"},
		Optimal:        "x",
		BufferVerified: true,
	}
	m.TrainerInput = ""
	m.TrainerValidation = nil
	return m
}

// typeTrainerKeys drives the real Update handler one key at a time, which is
// where the input routing lives.
func typeTrainerKeys(m Model, keys ...tea.KeyMsg) Model {
	for _, key := range keys {
		res, _ := m.Update(key)
		m = res.(Model)
	}
	return m
}

// TestTrainerCtrlRReachesTheEngine pins the end-to-end path of the redo key:
// typed on the exercise screen it becomes the control character the buffer
// engine parses, and the answer it completes is judged by the buffer it
// produces. "xu" plus Ctrl-r deletes the first rune, undoes it and redoes it,
// reaching the same buffer as the optimal "x" by different keys.
func TestTrainerCtrlRReachesTheEngine(t *testing.T) {
	m := bufferVerifiedLessonModel(t)

	m = typeTrainerKeys(m,
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}},
		tea.KeyMsg{Type: tea.KeyCtrlR},
	)
	if want := "xu\x12"; m.TrainerInput != want {
		t.Fatalf("TrainerInput = %q, want %q", m.TrainerInput, want)
	}

	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(Model)

	if !m.TrainerLastCorrect {
		t.Fatalf("the redo answer was rejected: message = %q", m.TrainerMessage)
	}
	if m.TrainerValidation == nil {
		t.Fatal("no validation result was kept for the result screen")
	}
	if want := []string{"b"}; !reflect.DeepEqual(m.TrainerValidation.ActualBuffer, want) {
		t.Errorf("ActualBuffer = %#v, want %#v", m.TrainerValidation.ActualBuffer, want)
	}
}

// TestTrainerResultShowsTheResultingBufferForOptedInExercises pins the result
// screen contract: an opted-in exercise shows the buffer its answer produced,
// and a shipped exercise renders exactly as before.
func TestTrainerResultShowsTheResultingBufferForOptedInExercises(t *testing.T) {
	t.Run("an opted-in exercise shows the resulting buffer", func(t *testing.T) {
		m := bufferVerifiedLessonModel(t)
		m = typeTrainerKeys(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})

		res, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = res.(Model)
		if m.Screen != ScreenTrainerResult {
			t.Fatalf("Screen = %v, want ScreenTrainerResult", m.Screen)
		}

		view := m.View()
		if !strings.Contains(view, "Resulting buffer") {
			t.Errorf("result screen does not show the resulting buffer:\n%s", view)
		}
		if !strings.Contains(view, "1 │ b") {
			t.Errorf("result screen does not render the resulting line:\n%s", view)
		}
	})

	t.Run("a shipped exercise renders as before", func(t *testing.T) {
		m := newTrainerResultModel(t)
		if view := m.View(); strings.Contains(view, "Resulting buffer") {
			t.Errorf("a shipped exercise shows the buffer preview:\n%s", view)
		}
	})
}

// A rejected buffer-verified answer names what differed on the result screen,
// derived from the buffers the engine produced rather than from the keystrokes.
func TestTrainerResultNamesWhatDifferedForOptedInExercises(t *testing.T) {
	m := bufferVerifiedLessonModel(t)

	// "u" with an empty undo history is a no-op: the buffer keeps its text and
	// only the buffer diverges from the optimal's result.
	m = typeTrainerKeys(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(Model)

	if m.TrainerLastCorrect {
		t.Fatal("a no-op answer was accepted")
	}
	if !strings.Contains(m.TrainerMessage, "buffer") {
		t.Errorf("TrainerMessage = %q, want it to name the buffer", m.TrainerMessage)
	}
	if strings.Contains(m.TrainerMessage, "\x15") {
		t.Errorf("TrainerMessage = %q, want no raw keystrokes", m.TrainerMessage)
	}
	if m.TrainerValidation == nil {
		t.Fatal("no validation result was kept for the result screen")
	}
	if want := []string{"ab"}; !reflect.DeepEqual(m.TrainerValidation.ActualBuffer, want) {
		t.Errorf("ActualBuffer = %#v, want %#v", m.TrainerValidation.ActualBuffer, want)
	}

	view := m.View()
	if !strings.Contains(view, "Expected buffer") {
		t.Errorf("rejected result screen does not show the expected buffer:\n%s", view)
	}
	if !strings.Contains(view, "Resulting buffer") {
		t.Errorf("rejected result screen does not show the produced buffer:\n%s", view)
	}
}
