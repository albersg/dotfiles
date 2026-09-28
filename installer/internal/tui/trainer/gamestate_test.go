package trainer

import (
	"testing"
	"time"
)

// =============================================================================
// GAME STATE - Creation and Initialization
// =============================================================================

func TestNewGameState_Initialization(t *testing.T) {
	state := NewGameState()

	if state == nil {
		t.Fatal("NewGameState should not return nil")
	}
	if state.Stats == nil {
		t.Error("Stats should be initialized")
	}
	if state.CurrentStreak != 0 {
		t.Errorf("CurrentStreak should be 0, got %d", state.CurrentStreak)
	}
	if state.ComboMultiplier != 1 {
		t.Errorf("ComboMultiplier should be 1, got %d", state.ComboMultiplier)
	}
	if state.SessionScore != 0 {
		t.Errorf("SessionScore should be 0, got %d", state.SessionScore)
	}
}

func TestNewGameState_WithExistingStats(t *testing.T) {
	// Simulate loading existing stats
	stats := NewUserStats()
	stats.TotalScore = 1000
	stats.BestStreak = 10

	state := NewGameStateWithStats(stats)

	if state.Stats.TotalScore != 1000 {
		t.Errorf("Should preserve existing TotalScore, got %d", state.Stats.TotalScore)
	}
	if state.Stats.BestStreak != 10 {
		t.Errorf("Should preserve existing BestStreak, got %d", state.Stats.BestStreak)
	}
}

// =============================================================================
// GAME STATE - Starting Modes
// =============================================================================

func TestGameState_StartLesson(t *testing.T) {
	state := NewGameState()

	state.StartLesson(ModuleHorizontal)

	if state.CurrentModule != ModuleHorizontal {
		t.Errorf("CurrentModule should be Horizontal, got %s", state.CurrentModule)
	}
	if !state.IsLessonMode {
		t.Error("IsLessonMode should be true")
	}
	if state.IsPracticeMode {
		t.Error("IsPracticeMode should be false")
	}
	if state.IsBossMode {
		t.Error("IsBossMode should be false")
	}
	if state.ExerciseIndex != 0 {
		t.Errorf("ExerciseIndex should be 0, got %d", state.ExerciseIndex)
	}
	if len(state.Exercises) == 0 {
		t.Error("Exercises should be loaded")
	}
	if state.CurrentExercise == nil {
		t.Error("CurrentExercise should be set")
	}
}

func TestGameState_StartPractice(t *testing.T) {
	state := NewGameState()

	state.StartPractice(ModuleHorizontal)

	if state.CurrentModule != ModuleHorizontal {
		t.Errorf("CurrentModule should be Horizontal, got %s", state.CurrentModule)
	}
	if state.IsLessonMode {
		t.Error("IsLessonMode should be false")
	}
	if !state.IsPracticeMode {
		t.Error("IsPracticeMode should be true")
	}
	if state.IsBossMode {
		t.Error("IsBossMode should be false")
	}
	// In intelligent practice mode, we use random selection instead of loading all exercises
	// CurrentExercise should be set with a weighted random selection
	if state.CurrentExercise == nil {
		t.Error("CurrentExercise should be set for practice mode")
	}
}

func TestGameState_StartBoss(t *testing.T) {
	state := NewGameState()

	state.StartBoss(ModuleHorizontal)

	if !state.IsBossMode {
		t.Error("IsBossMode should be true")
	}
	if state.BossLives != 3 {
		t.Errorf("BossLives should be 3, got %d", state.BossLives)
	}
	if state.CurrentBoss == nil {
		t.Error("CurrentBoss should be set")
	}
	if state.BossStep != 0 {
		t.Errorf("BossStep should be 0, got %d", state.BossStep)
	}
}

// =============================================================================
// GAME STATE - Recording Answers
// =============================================================================

func TestGameState_RecordCorrectAnswer(t *testing.T) {
	state := NewGameState()
	state.StartLesson(ModuleHorizontal)
	initialScore := state.SessionScore

	state.RecordCorrectAnswer(5.0, true) // 5 seconds, optimal answer

	if state.CurrentStreak != 1 {
		t.Errorf("CurrentStreak should be 1 after correct answer, got %d", state.CurrentStreak)
	}
	if state.SessionScore <= initialScore {
		t.Error("SessionScore should increase after correct answer")
	}
}

func TestGameState_RecordCorrectAnswer_UpdatesStreak(t *testing.T) {
	state := NewGameState()
	state.StartLesson(ModuleHorizontal)

	// Answer correctly 5 times
	for i := 0; i < 5; i++ {
		state.RecordCorrectAnswer(3.0, false)
	}

	if state.CurrentStreak != 5 {
		t.Errorf("CurrentStreak should be 5, got %d", state.CurrentStreak)
	}
}

func TestGameState_RecordCorrectAnswer_UpdatesBestStreak(t *testing.T) {
	state := NewGameState()
	state.StartLesson(ModuleHorizontal)

	// Answer correctly 5 times
	for i := 0; i < 5; i++ {
		state.RecordCorrectAnswer(3.0, false)
	}

	if state.Stats.BestStreak < 5 {
		t.Errorf("BestStreak should be at least 5, got %d", state.Stats.BestStreak)
	}
}

func TestGameState_RecordIncorrectAnswer(t *testing.T) {
	state := NewGameState()
	state.StartLesson(ModuleHorizontal)
	state.CurrentStreak = 5

	state.RecordIncorrectAnswer()

	if state.CurrentStreak != 0 {
		t.Errorf("CurrentStreak should be 0 after incorrect answer, got %d", state.CurrentStreak)
	}
}

func TestGameState_RecordIncorrectAnswer_PreservesBestStreak(t *testing.T) {
	state := NewGameState()
	state.Stats.BestStreak = 10
	state.CurrentStreak = 5

	state.RecordIncorrectAnswer()

	if state.Stats.BestStreak != 10 {
		t.Errorf("BestStreak should be preserved at 10, got %d", state.Stats.BestStreak)
	}
}

func TestGameState_RecordIncorrectAnswer_InBossMode(t *testing.T) {
	state := NewGameState()
	state.StartBoss(ModuleHorizontal)
	initialLives := state.BossLives

	state.RecordIncorrectAnswer()

	if state.BossLives != initialLives-1 {
		t.Errorf("BossLives should decrease by 1, expected %d, got %d", initialLives-1, state.BossLives)
	}
}

func TestGameState_RecordIncorrectAnswer_ZeroBossLivesIsDefeatNotVictory(t *testing.T) {
	state := NewGameState()
	state.StartBoss(ModuleHorizontal)
	state.BossLives = 1

	state.RecordIncorrectAnswer()

	if state.BossLives != 0 {
		t.Errorf("BossLives should be 0, got %d", state.BossLives)
	}
	// IsBossDefeated names a victory. Running out of lives is a loss, so the
	// field must stay false and the loss is read from BossLives reaching zero.
	if state.IsBossDefeated {
		t.Error("IsBossDefeated should be false when lives reach 0")
	}
}

func TestGameState_RecordIncorrectAnswer_InBossMode_RecordsAttempt(t *testing.T) {
	state := NewGameState()
	state.StartBoss(ModuleHorizontal)
	initialLives := state.BossLives

	state.RecordIncorrectAnswer()

	progress := state.Stats.GetModuleProgress(ModuleHorizontal)
	if progress.BossAttempts != 1 {
		t.Errorf("BossAttempts = %d after a lost life, want 1", progress.BossAttempts)
	}
	if state.BossLives != initialLives-1 {
		t.Errorf("BossLives should decrease by 1, expected %d, got %d", initialLives-1, state.BossLives)
	}
	if state.IsBossDefeated {
		t.Error("IsBossDefeated should stay false while losing lives")
	}
}

func TestGameState_RecordCorrectAnswer_InBossMode_RecordsAttempt(t *testing.T) {
	state := NewGameState()
	state.StartBoss(ModuleHorizontal)

	state.RecordCorrectAnswer(3.0, true)

	progress := state.Stats.GetModuleProgress(ModuleHorizontal)
	if progress.BossAttempts != 1 {
		t.Errorf("BossAttempts = %d after a correct boss step, want 1", progress.BossAttempts)
	}
	if state.CurrentStreak != 1 {
		t.Errorf("CurrentStreak = %d after a correct boss step, want 1", state.CurrentStreak)
	}
	if state.SessionScore <= 0 {
		t.Errorf("SessionScore = %d after a correct boss step, want > 0", state.SessionScore)
	}
}

// =============================================================================
// GAME STATE - Exercise Progression
// =============================================================================

func TestGameState_NextExercise_InLessonMode(t *testing.T) {
	state := NewGameState()
	state.StartLesson(ModuleHorizontal)

	hasMore := state.NextExercise()

	if state.ExerciseIndex != 1 {
		t.Errorf("ExerciseIndex should be 1, got %d", state.ExerciseIndex)
	}
	if !hasMore && state.ExerciseIndex < len(state.Exercises) {
		t.Error("NextExercise should return true when more exercises exist")
	}
}

func TestGameState_NextExercise_CompletesLessons(t *testing.T) {
	state := NewGameState()
	state.StartLesson(ModuleHorizontal)

	// Advance through all exercises
	for state.NextExercise() {
		// Keep going
	}

	// Check lessons are marked complete in stats
	progress := state.Stats.GetModuleProgress(ModuleHorizontal)
	if progress.LessonsCompleted != progress.LessonsTotal {
		t.Errorf("LessonsCompleted should equal LessonsTotal after completing all")
	}
}

func TestGameState_NextExercise_InBossMode(t *testing.T) {
	state := NewGameState()
	state.StartBoss(ModuleHorizontal)
	initialStep := state.BossStep

	hasMore := state.NextExercise()

	if state.BossStep != initialStep+1 {
		t.Errorf("BossStep should increase by 1")
	}
	if !hasMore && state.BossStep < len(state.CurrentBoss.Steps) {
		t.Error("NextExercise should return true when more boss steps exist")
	}
}

func TestGameState_NextExercise_IncreasesCombo(t *testing.T) {
	state := NewGameState()
	state.StartBoss(ModuleHorizontal)
	state.ComboMultiplier = 2

	state.NextExercise()

	if state.ComboMultiplier != 3 {
		t.Errorf("ComboMultiplier should increase to 3, got %d", state.ComboMultiplier)
	}
}

func TestGameState_NextExercise_ComboMaxAt4(t *testing.T) {
	state := NewGameState()
	state.StartBoss(ModuleHorizontal)
	state.ComboMultiplier = 4

	state.NextExercise()

	if state.ComboMultiplier != 4 {
		t.Errorf("ComboMultiplier should max at 4, got %d", state.ComboMultiplier)
	}
}

// =============================================================================
// GAME STATE - Stats Updates
// =============================================================================

func TestGameState_UpdatePracticeStats(t *testing.T) {
	state := NewGameState()
	state.StartPractice(ModuleHorizontal)

	progress := state.Stats.GetModuleProgress(ModuleHorizontal)

	// RecordCorrectAnswer/RecordIncorrectAnswer own streaks, score and lesson
	// progress, not practice accounting. Three answers here must leave the
	// practice counters untouched.
	state.RecordCorrectAnswer(3.0, false)
	state.RecordCorrectAnswer(4.0, true)
	state.RecordIncorrectAnswer()

	if progress.PracticeAttempts != 0 {
		t.Errorf("PracticeAttempts should stay 0 when GameState records answers, got %d", progress.PracticeAttempts)
	}
	if progress.PracticeCorrect != 0 {
		t.Errorf("PracticeCorrect should stay 0 when GameState records answers, got %d", progress.PracticeCorrect)
	}

	// ModuleProgress.RecordPracticeResult is the single owner of the practice
	// counters, so the same three results land there.
	exerciseID := state.CurrentExercise.ID
	progress.RecordPracticeResult(exerciseID, true)
	progress.RecordPracticeResult(exerciseID, true)
	progress.RecordPracticeResult(exerciseID, false)

	if progress.PracticeAttempts != 3 {
		t.Errorf("PracticeAttempts should be 3, got %d", progress.PracticeAttempts)
	}
	if progress.PracticeCorrect != 2 {
		t.Errorf("PracticeCorrect should be 2, got %d", progress.PracticeCorrect)
	}
}

func TestGameState_PracticeAccuracyCalculation(t *testing.T) {
	state := NewGameState()
	state.StartPractice(ModuleHorizontal)

	progress := state.Stats.GetModuleProgress(ModuleHorizontal)
	exerciseID := state.CurrentExercise.ID

	// 8 correct, 2 incorrect = 80% accuracy. Each submission is the composition
	// the UI handler runs: GameState records the streak/score side, and
	// RecordPracticeResult owns the practice counters.
	for i := 0; i < 8; i++ {
		state.RecordCorrectAnswer(3.0, false)
		progress.RecordPracticeResult(exerciseID, true)
	}
	for i := 0; i < 2; i++ {
		state.RecordIncorrectAnswer()
		progress.RecordPracticeResult(exerciseID, false)
	}

	expectedAccuracy := 0.80

	if progress.PracticeAccuracy < expectedAccuracy-0.01 || progress.PracticeAccuracy > expectedAccuracy+0.01 {
		t.Errorf("PracticeAccuracy should be ~0.80, got %f", progress.PracticeAccuracy)
	}
}

func TestGameState_RecordBossVictory(t *testing.T) {
	state := NewGameState()
	state.StartBoss(ModuleHorizontal)
	state.BossLives = 2
	state.TimeElapsed = 25 * time.Second

	state.RecordBossVictory()

	if !state.IsBossDefeated {
		t.Error("IsBossDefeated should be true after the player wins the fight")
	}
	if !state.Stats.IsBossDefeated(ModuleHorizontal) {
		t.Error("Boss should be marked as defeated")
	}

	progress := state.Stats.GetModuleProgress(ModuleHorizontal)
	if !progress.BossDefeated {
		t.Error("Module progress should show boss defeated")
	}
	if progress.BossBestTime != 25*time.Second {
		t.Errorf("BossBestTime should be 25s, got %v", progress.BossBestTime)
	}
}

func TestGameState_RecordBossVictory_DoesNotDoubleCountAttempts(t *testing.T) {
	state := NewGameState()
	state.StartBoss(ModuleHorizontal)

	// The winning step goes through RecordCorrectAnswer, the single owner of
	// boss attempt accounting, so the victory bonus must not count it again.
	state.RecordCorrectAnswer(3.0, true)
	state.RecordBossVictory()

	progress := state.Stats.GetModuleProgress(ModuleHorizontal)
	if progress.BossAttempts != 1 {
		t.Errorf("BossAttempts = %d after one winning answer, want 1", progress.BossAttempts)
	}
}

func TestGameState_RecordBossVictory_AddsToDefeatedList(t *testing.T) {
	state := NewGameState()
	state.StartBoss(ModuleHorizontal)

	state.RecordBossVictory()

	found := false
	for _, boss := range state.Stats.BossesDefeated {
		if boss == ModuleHorizontal {
			found = true
			break
		}
	}
	if !found {
		t.Error("Horizontal should be in BossesDefeated list")
	}
}

func TestGameState_RecordBossVictory_DoesntDuplicateInList(t *testing.T) {
	state := NewGameState()
	state.Stats.BossesDefeated = []ModuleID{ModuleHorizontal} // Already defeated
	state.StartBoss(ModuleHorizontal)

	state.RecordBossVictory()

	count := 0
	for _, boss := range state.Stats.BossesDefeated {
		if boss == ModuleHorizontal {
			count++
		}
	}
	if count != 1 {
		t.Errorf("Horizontal should only appear once in BossesDefeated, got %d", count)
	}
}

// =============================================================================
// GAME STATE - Session Management
// =============================================================================

func TestGameState_Reset(t *testing.T) {
	state := NewGameState()
	state.StartLesson(ModuleHorizontal)
	state.CurrentStreak = 10
	state.SessionScore = 500
	state.ComboMultiplier = 3

	state.Reset()

	if state.CurrentStreak != 0 {
		t.Errorf("CurrentStreak should be 0 after reset, got %d", state.CurrentStreak)
	}
	if state.SessionScore != 0 {
		t.Errorf("SessionScore should be 0 after reset, got %d", state.SessionScore)
	}
	if state.ComboMultiplier != 1 {
		t.Errorf("ComboMultiplier should be 1 after reset, got %d", state.ComboMultiplier)
	}
	if state.IsLessonMode || state.IsPracticeMode || state.IsBossMode {
		t.Error("All modes should be false after reset")
	}
}

// =============================================================================
// GAME STATE - Intelligent Practice Mode
// =============================================================================

func TestGameState_SetPracticeExercise(t *testing.T) {
	state := NewGameState()
	exercise := &Exercise{
		ID:      "test-exercise",
		Mission: "Test mission",
	}

	state.SetPracticeExercise(exercise)

	if state.CurrentExercise != exercise {
		t.Error("SetPracticeExercise should set the current exercise")
	}
	if state.CurrentExercise.ID != "test-exercise" {
		t.Errorf("Expected exercise ID 'test-exercise', got '%s'", state.CurrentExercise.ID)
	}
}

func TestGameState_NextPracticeExercise_ReturnsTrue(t *testing.T) {
	state := NewGameState()
	state.StartPractice(ModuleHorizontal)

	// Should return true when there are unmastered exercises
	hasNext := state.NextPracticeExercise()

	if !hasNext {
		t.Error("NextPracticeExercise should return true when exercises remain")
	}
	if state.CurrentExercise == nil {
		t.Error("NextPracticeExercise should set a new exercise")
	}
}

func TestGameState_NextPracticeExercise_ReturnsFalseWhenNotPracticeMode(t *testing.T) {
	state := NewGameState()
	state.StartLesson(ModuleHorizontal)

	hasNext := state.NextPracticeExercise()

	if hasNext {
		t.Error("NextPracticeExercise should return false when not in practice mode")
	}
}

func TestGameState_NextPracticeExercise_ReturnsFalseWhenAllMastered(t *testing.T) {
	state := NewGameState()
	state.StartPractice(ModuleHorizontal)

	// Mark all exercises as mastered
	progress := state.Stats.GetModuleProgress(ModuleHorizontal)
	lessons := GetLessons(ModuleHorizontal)
	for _, lesson := range lessons {
		exStats := progress.GetExerciseStats(lesson.ID)
		exStats.Mastered = true
	}

	hasNext := state.NextPracticeExercise()

	if hasNext {
		t.Error("NextPracticeExercise should return false when all exercises are mastered")
	}
}

// =============================================================================
// GAME STATE - Answer timing
// =============================================================================

// timedExercise returns an exercise with a known base score and speed gate, so
// the timing assertions can name exact point values instead of re-deriving the
// scoring formula.
func timedExercise() *Exercise {
	return &Exercise{
		ID:          "timed",
		Module:      ModuleHorizontal,
		Type:        ExercisePractice,
		Points:      100,
		TimeoutSecs: 30,
		Optimal:     "w",
		Solutions:   []string{"w"},
	}
}

// TestGameState_ElapsedAnswerTimeDrivesSpeedBonus pins that the answer clock is
// real: the points earned come from the time the injected clock reports, so the
// under-two-second speed multiplier fires for a fast answer and not for a slow
// one. The score, not the elapsed field, is the assertion.
func TestGameState_ElapsedAnswerTimeDrivesSpeedBonus(t *testing.T) {
	tests := []struct {
		name      string
		elapsed   time.Duration
		wantScore int
	}{
		{
			name:      "an answer under two seconds earns the speed bonus",
			elapsed:   1500 * time.Millisecond,
			wantScore: 187, // 100 base, +50% optimal, +25% speed: 187.5 truncated
		},
		{
			name:      "an answer at the two second threshold does not",
			elapsed:   2 * time.Second,
			wantScore: 150, // 100 base, +50% optimal, no speed bonus
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
			state := NewGameState()
			state.SetClock(func() time.Time { return now })
			state.SetPracticeExercise(timedExercise())

			now = now.Add(tt.elapsed)
			state.RecordCorrectAnswer(state.ElapsedSeconds(), true)

			if state.SessionScore != tt.wantScore {
				t.Errorf("SessionScore = %d for a %v answer, want %d",
					state.SessionScore, tt.elapsed, tt.wantScore)
			}
		})
	}
}

// TestGameState_IncorrectAnswerDoesNotRestartClock pins that the measured time
// is the time to solve the current exercise: a wrong answer leaves the clock
// running, so a fast correction after a mistake is honestly slower and must not
// earn the speed bonus.
func TestGameState_IncorrectAnswerDoesNotRestartClock(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	state := NewGameState()
	state.SetClock(func() time.Time { return now })
	state.SetPracticeExercise(timedExercise())

	// A mistake 1.5s in, then a correct answer half a second later: two seconds
	// from presentation, so the answer is not fast any more.
	now = now.Add(1500 * time.Millisecond)
	state.RecordIncorrectAnswer()
	now = now.Add(500 * time.Millisecond)
	state.RecordCorrectAnswer(state.ElapsedSeconds(), true)

	// 150 is the optimal-only score. A clock restarted by the wrong answer would
	// have reported 0.5s and added the 25% speed multiplier instead.
	if state.SessionScore != 150 {
		t.Errorf("SessionScore = %d after a wrong answer and a fast correction, want 150: the clock restarted on the wrong answer",
			state.SessionScore)
	}
}

// TestGameState_AdvanceRestartsTheClock triangulates the presentation rule: an
// advance to the next exercise presents it and starts its timer, so the next
// answer is measured from the new exercise rather than from the one before it.
// The assertion is the score of the next exercise's own answer.
func TestGameState_AdvanceRestartsTheClock(t *testing.T) {
	tests := []struct {
		name    string
		start   func(*GameState)
		advance func(*GameState) bool
	}{
		{
			name:    "lesson",
			start:   func(state *GameState) { state.StartLesson(ModuleHorizontal) },
			advance: func(state *GameState) bool { return state.NextExercise() },
		},
		{
			name:    "practice",
			start:   func(state *GameState) { state.StartPractice(ModuleHorizontal) },
			advance: func(state *GameState) bool { return state.NextPracticeExercise() },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
			state := NewGameState()
			state.SetClock(func() time.Time { return now })
			tt.start(state)

			// Well past the speed threshold before the advance, so an unreset clock
			// would deny the bonus to the exercise that follows.
			now = now.Add(10 * time.Second)
			if !tt.advance(state) {
				t.Fatal("advance returned false, the test needs a next exercise")
			}
			next := state.CurrentExercise
			if next == nil {
				t.Fatal("advance did not present the next exercise")
			}

			now = now.Add(1500 * time.Millisecond)
			state.RecordCorrectAnswer(state.ElapsedSeconds(), true)

			want := CalculatePoints(next, 1.5, true, 1)
			if want == CalculatePoints(next, 11.5, true, 1) {
				t.Fatalf("test setup: %s scores the same fast and slow, the assertion cannot detect a stale clock", next.ID)
			}
			if state.SessionScore != want {
				t.Errorf("SessionScore = %d after answering the exercise the advance presented, want %d: the clock was not restarted by the advance",
					state.SessionScore, want)
			}
		})
	}
}

// TestGameState_TotalTimeAccumulatesAndPersists pins that answered exercises
// feed UserStats.TotalTime and LastPlayed, and that the accumulated total keeps
// the persisted meaning across a save/load round trip.
func TestGameState_TotalTimeAccumulatesAndPersists(t *testing.T) {
	originalPath := statsConfigPath
	statsConfigPath = t.TempDir()
	defer func() { statsConfigPath = originalPath }()

	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	state := NewGameState()
	state.SetClock(func() time.Time { return now })

	// Two answered exercises: three seconds, then four.
	state.SetPracticeExercise(timedExercise())
	now = now.Add(3 * time.Second)
	state.RecordCorrectAnswer(state.ElapsedSeconds(), true)

	state.SetPracticeExercise(timedExercise())
	now = now.Add(4 * time.Second)
	state.RecordCorrectAnswer(state.ElapsedSeconds(), true)

	if state.Stats.TotalTime != 7*time.Second {
		t.Errorf("TotalTime = %v after two answers, want 7s", state.Stats.TotalTime)
	}
	if !state.Stats.LastPlayed.Equal(now) {
		t.Errorf("LastPlayed = %v after the second answer, want %v", state.Stats.LastPlayed, now)
	}

	if err := SaveStats(state.Stats); err != nil {
		t.Fatalf("SaveStats failed: %v", err)
	}

	loaded := LoadStats()
	if loaded == nil {
		t.Fatal("LoadStats returned nil after SaveStats")
	}
	if loaded.TotalTime != 7*time.Second {
		t.Errorf("TotalTime = %v after a save/load round trip, want 7s", loaded.TotalTime)
	}
	if !loaded.LastPlayed.Equal(state.Stats.LastPlayed) {
		t.Errorf("LastPlayed = %v after a save/load round trip, want %v", loaded.LastPlayed, state.Stats.LastPlayed)
	}
}

// TestGameState_HintIsDueOnlyAfterTimeout pins the deadline the exercise screen
// reads. RemainingSeconds counts down from the exercise's own TimeoutSecs on the
// injected clock, and HintDue flips only once that deadline is reached, so the
// automatic hint cannot arrive early.
func TestGameState_HintIsDueOnlyAfterTimeout(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	state := NewGameState()
	state.SetClock(func() time.Time { return now })
	state.SetPracticeExercise(timedExercise()) // TimeoutSecs 30

	now = now.Add(29 * time.Second)
	if state.HintDue() {
		t.Error("HintDue = true one second before the deadline")
	}
	if got := state.RemainingSeconds(); got != 1 {
		t.Errorf("RemainingSeconds = %v one second before the deadline, want 1", got)
	}

	now = now.Add(time.Second)
	if !state.HintDue() {
		t.Error("HintDue = false once the deadline passed")
	}
	if got := state.RemainingSeconds(); got != 0 {
		t.Errorf("RemainingSeconds = %v at the deadline, want 0", got)
	}

	// Past the deadline the countdown stays clamped at zero.
	now = now.Add(time.Minute)
	if got := state.RemainingSeconds(); got != 0 {
		t.Errorf("RemainingSeconds = %v after the deadline, want 0", got)
	}
	if !state.HintDue() {
		t.Error("HintDue = false after the deadline, want true")
	}
}

// TestGameState_UntimedExerciseHasNoDeadline pins that a boss-style exercise
// without TimeoutSecs never reports a countdown or an automatic hint, so the
// boss screen is untouched by the lesson/practice deadline.
func TestGameState_UntimedExerciseHasNoDeadline(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	state := NewGameState()
	state.SetClock(func() time.Time { return now })
	state.SetPracticeExercise(&Exercise{ID: "untimed", Points: 50})

	now = now.Add(10 * time.Minute)
	if state.HintDue() {
		t.Error("HintDue = true for an exercise that declares no timeout")
	}
	if got := state.RemainingSeconds(); got != 0 {
		t.Errorf("RemainingSeconds = %v for an exercise that declares no timeout, want 0", got)
	}
}

// =============================================================================
// GAME STATE - BOSS STEP DEADLINE
// =============================================================================

// TestGameState_BossStepTimeLimitIncludesThePreviousWinBonus asserts the
// effective limit directly rather than reading the raw TimeLimit field: the
// first step gets no bonus, and each step after a win gets its own TimeLimit
// plus BonusTime. Winning twice in a row still grants the bonus once, because
// the grant is per win and must not accumulate across steps.
func TestGameState_BossStepTimeLimitIncludesThePreviousWinBonus(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	state := NewGameState()
	state.SetClock(func() time.Time { return now })
	state.StartBoss(ModuleHorizontal)

	boss := state.CurrentBoss
	if boss == nil || len(boss.Steps) < 3 {
		t.Fatalf("test setup: the horizontal boss no longer has three steps")
	}
	if boss.BonusTime <= 0 {
		t.Fatalf("test setup: %s declares no BonusTime", boss.ID)
	}

	// The first step is granted nothing: no step before it was won.
	if got := state.BossStepTimeLimit(); got != boss.Steps[0].TimeLimit {
		t.Errorf("step 0 effective limit = %d, want its own TimeLimit %d", got, boss.Steps[0].TimeLimit)
	}

	// Win step 0: step 1's effective limit must include the bonus.
	state.RecordCorrectAnswer(1, true)
	if !state.NextBossExercise() {
		t.Fatal("NextBossExercise = false after step 0, want the fight to continue")
	}
	if got, want := state.BossStepTimeLimit(), boss.Steps[1].TimeLimit+boss.BonusTime; got != want {
		t.Errorf("step 1 effective limit = %d, want TimeLimit %d + BonusTime %d = %d",
			got, boss.Steps[1].TimeLimit, boss.BonusTime, want)
	}
	if got := state.BossStepSecondsLeft(); got != float64(state.BossStepTimeLimit()) {
		t.Errorf("step 1 seconds left = %v right after presentation, want the full effective limit %d",
			got, state.BossStepTimeLimit())
	}

	// Win step 1 too: step 2 gets one bonus, not two.
	state.RecordCorrectAnswer(1, true)
	if !state.NextBossExercise() {
		t.Fatal("NextBossExercise = false after step 1, want the fight to continue")
	}
	if got, want := state.BossStepTimeLimit(), boss.Steps[2].TimeLimit+boss.BonusTime; got != want {
		t.Errorf("step 2 effective limit = %d, want TimeLimit %d + one BonusTime %d = %d (the bonus accumulated)",
			got, boss.Steps[2].TimeLimit, boss.BonusTime, want)
	}
}

// TestGameState_BossStepTimeoutSpendsOneLifeAndRearms pins the state half of
// the clock contract: the canonical recorder costs one life and one attempt,
// the player stays on the same step, and the deadline is re-armed a full
// effective limit ahead so the retry is not charged immediately.
func TestGameState_BossStepTimeoutSpendsOneLifeAndRearms(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	state := NewGameState()
	state.SetClock(func() time.Time { return now })
	state.StartBoss(ModuleHorizontal)

	limit := state.BossStepTimeLimit()
	if limit <= 0 {
		t.Fatalf("test setup: boss step 0 declares no effective limit")
	}
	lives := state.BossLives
	attempts := state.Stats.GetModuleProgress(ModuleHorizontal).BossAttempts
	stepID := state.CurrentExercise.ID
	state.CurrentStreak = 5

	// Past the deadline.
	now = now.Add(time.Duration(limit)*time.Second + time.Second)
	if !state.BossDeadlinePassed() {
		t.Fatalf("BossDeadlinePassed = false after %ds on a %ds step", limit+1, limit)
	}
	state.RecordBossStepTimeout()

	if got := state.BossLives; got != lives-1 {
		t.Errorf("BossLives = %d after the deadline, want %d", got, lives-1)
	}
	if got := state.Stats.GetModuleProgress(ModuleHorizontal).BossAttempts; got != attempts+1 {
		t.Errorf("BossAttempts = %d after the deadline, want %d", got, attempts+1)
	}
	if state.CurrentStreak != 0 {
		t.Errorf("CurrentStreak = %d after the deadline, want 0", state.CurrentStreak)
	}
	if state.CurrentExercise == nil || state.CurrentExercise.ID != stepID {
		t.Errorf("current exercise after the deadline = %v, want %s: expiry moved the player off the step", state.CurrentExercise, stepID)
	}
	if state.BossStep != 0 {
		t.Errorf("BossStep = %d after the deadline, want 0", state.BossStep)
	}

	// The retry starts with a full window, and the same instant is inside it.
	if state.BossDeadlinePassed() {
		t.Error("BossDeadlinePassed = true immediately after the timeout re-armed the deadline")
	}
	if got := state.BossStepSecondsLeft(); got != float64(limit) {
		t.Errorf("seconds left after the timeout = %v, want a full %d", got, limit)
	}

	// One second before the fresh window ends nothing more is charged...
	now = now.Add(time.Duration(limit)*time.Second - time.Second)
	if state.BossDeadlinePassed() {
		t.Fatal("BossDeadlinePassed = true one second before the re-armed deadline")
	}
	// ...and crossing it charges the next life through the same recorder.
	now = now.Add(time.Second)
	if !state.BossDeadlinePassed() {
		t.Fatal("BossDeadlinePassed = false at the re-armed deadline")
	}
	state.RecordBossStepTimeout()
	if got := state.BossLives; got != lives-2 {
		t.Errorf("BossLives = %d after the second deadline, want %d", got, lives-2)
	}
	if got := state.Stats.GetModuleProgress(ModuleHorizontal).BossAttempts; got != attempts+2 {
		t.Errorf("BossAttempts = %d after two deadlines, want %d", got, attempts+2)
	}
}

// TestGameState_WrongAnswerRearmsTheBossWindow pins the owner: a lost life
// re-arms the step window inside RecordIncorrectAnswer, so a wrong answer gets
// the same fresh full window an expiry does. Before this, only expiry re-armed,
// and a wrong answer near the old deadline cost a second life when that same
// window expired.
func TestGameState_WrongAnswerRearmsTheBossWindow(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	state := NewGameState()
	state.SetClock(func() time.Time { return now })
	state.StartBoss(ModuleHorizontal)

	limit := state.BossStepTimeLimit()
	if limit <= 0 {
		t.Fatal("test setup: boss step 0 declares no effective limit")
	}

	// A wrong answer late in the window: the spent life re-arms it in full.
	now = now.Add(time.Duration(limit-1) * time.Second)
	state.RecordIncorrectAnswer()
	if state.BossDeadlinePassed() {
		t.Fatal("BossDeadlinePassed = true right after a wrong answer, want a fresh window")
	}
	if got := state.BossStepSecondsLeft(); got != float64(limit) {
		t.Errorf("seconds left after a wrong answer = %v, want a full %d", got, limit)
	}

	// The original deadline arrives with no second charge, because the window now
	// ends a full limit after the wrong answer.
	now = now.Add(time.Second)
	if state.BossDeadlinePassed() {
		t.Error("BossDeadlinePassed = true at the original deadline after the wrong answer re-armed the window")
	}
}

// TestGameState_UntimedBossStepHasNoDeadline pins that a boss step which
// declares no TimeLimit is never charged by the clock, so the deadline cannot
// drain a fight whose data never asked for one.
func TestGameState_UntimedBossStepHasNoDeadline(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	state := NewGameState()
	state.SetClock(func() time.Time { return now })
	state.IsBossMode = true
	state.BossLives = 3
	state.CurrentBoss = &BossExercise{
		ID:        "untimed_boss",
		Module:    ModuleHorizontal,
		Lives:     3,
		BonusTime: 30,
		Steps:     []BossStep{{Exercise: Exercise{ID: "untimed_step_1"}}},
	}
	state.presentBossStep()

	now = now.Add(10 * time.Minute)

	if got := state.BossStepTimeLimit(); got != 0 {
		t.Errorf("BossStepTimeLimit = %d for a step with no TimeLimit, want 0", got)
	}
	if state.BossDeadlinePassed() {
		t.Error("BossDeadlinePassed = true for a step with no TimeLimit")
	}
	if got := state.BossStepSecondsLeft(); got != 0 {
		t.Errorf("BossStepSecondsLeft = %v for a step with no TimeLimit, want 0", got)
	}
}

// TestGameState_BossStepTimeoutOutsideBossModeDoesNothing is the negative guard
// for the recorder: it is a boss mechanic and must be inert in lesson or
// practice mode, where losing a life has no meaning.
func TestGameState_BossStepTimeoutOutsideBossModeDoesNothing(t *testing.T) {
	state := NewGameState()
	state.StartLesson(ModuleHorizontal)

	state.RecordBossStepTimeout()

	if state.BossLives != 0 {
		t.Errorf("BossLives = %d after a timeout outside boss mode, want 0", state.BossLives)
	}
	if state.IsBossDefeated {
		t.Error("IsBossDefeated = true after a timeout outside boss mode")
	}
	if state.CurrentExercise == nil {
		t.Error("CurrentExercise = nil after a timeout outside boss mode: the lesson was disturbed")
	}
}
