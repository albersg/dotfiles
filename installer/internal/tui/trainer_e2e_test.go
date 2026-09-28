package tui

import (
	"bytes"
	"testing"
	"time"

	"github.com/albersg/dotfiles/installer/internal/tui/trainer"
	tea "github.com/charmbracelet/bubbletea"
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

	t.Run("ctrl+e and ctrl+w are ignored too", func(t *testing.T) {
		cases := []struct {
			name  string
			model func(*testing.T) Model
		}{
			{"lesson", newTrainerLessonModel},
			{"boss", newTrainerBossModel},
		}

		for _, tc := range cases {
			for keyName, keyType := range map[string]tea.KeyType{
				"ctrl+e": tea.KeyCtrlE,
				"ctrl+w": tea.KeyCtrlW,
			} {
				t.Run(tc.name+"/"+keyName, func(t *testing.T) {
					m := tc.model(t)
					m.TrainerInput = ""

					result, _ := m.Update(tea.KeyMsg{Type: keyType})
					m = result.(Model)

					if m.TrainerInput != "" {
						t.Errorf("TrainerInput = %q after %s, want empty", m.TrainerInput, keyName)
					}
				})
			}
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
