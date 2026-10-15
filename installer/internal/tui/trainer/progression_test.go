package trainer

import (
	"math/rand"
	"testing"
	"time"
)

// =============================================================================
// T1 — PROGRESSION CORE
//
// The helpers below are the only place these tests touch the selection and
// recording signatures. They exist so the seams this work unit introduces (an
// injectable randomness source and an injectable clock) are one edit here
// instead of one edit per test.
// =============================================================================

func pickPractice(t *testing.T, module ModuleID, progress *ModuleProgress) *Exercise {
	t.Helper()
	return SelectRandomPracticeExercise(module, progress, nil, time.Now())
}

// recordPractice records one answer at the real clock. A fixture that answers "now"
// is the fixture of a player in the middle of a session, with nothing due for review
// yet, which is what these progression tests are about; the tests that need a stamp
// of their own pass one explicitly (see review_test.go).
func recordPractice(mp *ModuleProgress, exerciseID string, correct bool) {
	mp.RecordPracticeResult(exerciseID, correct, time.Now())
}

// completeLessons records the lesson stage of a module as finished, which is the
// state a player is in when they practise: LessonsTotal is only written when the
// lesson screen is opened, so a fixture that masters exercises without passing
// through StartLesson is not yet practice-ready.
func completeLessons(stats *UserStats, module ModuleID) {
	progress := stats.GetModuleProgress(module)
	progress.LessonsTotal = len(GetLessons(module))
	progress.LessonsCompleted = progress.LessonsTotal
}

// masterAll records the answers that master every lesson of a module: three
// consecutive correct answers per exercise. The caller decides the accuracy the
// module ends up with by adding wrong answers before each run.
func masterAll(mp *ModuleProgress, module ModuleID, wrongsPerExercise int) {
	for _, lesson := range GetLessons(module) {
		for i := 0; i < wrongsPerExercise; i++ {
			recordPractice(mp, lesson.ID, false)
		}
		for i := 0; i < MasteryThreshold; i++ {
			recordPractice(mp, lesson.ID, true)
		}
	}
}

// TestPracticeSelection_AllMasteredStillOffersReview pins S2: a module whose
// exercises are all mastered keeps offering practice while its boss gate is
// unmet. Before this rule the pool emptied and the player could neither practice
// nor fight the boss, and the only exit was the destructive module reset.
func TestPracticeSelection_AllMasteredStillOffersReview(t *testing.T) {
	mp := &ModuleProgress{}
	masterAll(mp, ModuleHorizontal, 0)

	if !mp.IsPracticeComplete(ModuleHorizontal) {
		t.Fatal("the fixture is wrong: every exercise should be mastered")
	}

	if ex := pickPractice(t, ModuleHorizontal, mp); ex == nil {
		t.Fatal("an all-mastered module offered no review exercise, so practice and the boss gate deadlock")
	}
}

// TestPracticeSelection_DoesNotCreateEntries pins S1: selecting a practice
// exercise reads the recorded per-exercise stats without manufacturing records
// for exercises the player has never attempted. Starting practice used to write
// one empty record per lesson into the profile.
func TestPracticeSelection_DoesNotCreateEntries(t *testing.T) {
	mp := &ModuleProgress{}

	if ex := pickPractice(t, ModuleHorizontal, mp); ex == nil {
		t.Fatal("expected a practice exercise")
	}

	if len(mp.ExerciseStats) != 0 {
		t.Errorf("selecting a practice exercise created %d profile entries, want none", len(mp.ExerciseStats))
	}
}

// TestIsPracticeComplete_DoesNotCreateEntries pins S1 for the completion check,
// which the trainer menu runs every time the player asks to practise.
func TestIsPracticeComplete_DoesNotCreateEntries(t *testing.T) {
	mp := &ModuleProgress{}

	mp.IsPracticeComplete(ModuleHorizontal)

	if len(mp.ExerciseStats) != 0 {
		t.Errorf("the completion check created %d profile entries, want none", len(mp.ExerciseStats))
	}
}

// TestGetPracticeWeight_NilStatsIsNeverAttempted pins that the weight of an
// exercise with no recorded stats is the never-attempted weight. Reading a
// module's pool asks this for every lesson, so a nil record is an ordinary
// question, not a crash.
func TestGetPracticeWeight_NilStatsIsNeverAttempted(t *testing.T) {
	var stats *ExerciseStats

	if got := stats.GetPracticeWeightAt(time.Now()); got != 15 {
		t.Errorf("a never-attempted exercise weighs %d, want 15", got)
	}
}

// TestGetPracticeWeight_MasteredIsReviewWeight pins S2 at the weight level: a
// mastered exercise stays in the pool with the review weight instead of leaving
// it, which is what keeps the pool non-empty.
func TestGetPracticeWeight_MasteredIsReviewWeight(t *testing.T) {
	stats := &ExerciseStats{Mastered: true, TotalAttempts: 3, TotalCorrect: 3}

	if got := stats.GetPracticeWeightAt(time.Now()); got <= 0 {
		t.Errorf("a mastered exercise weighs %d, want a positive review weight", got)
	}
}

// TestNextPracticeExercise_ContinuesWhileBossGateUnmet is the end-to-end view of
// S2 through the game state, which is the path the screen takes: every exercise
// mastered, accuracy below the gate, and practice must keep going.
func TestNextPracticeExercise_ContinuesWhileBossGateUnmet(t *testing.T) {
	stats := NewUserStats()
	completeLessons(stats, ModuleHorizontal)
	progress := stats.GetModuleProgress(ModuleHorizontal)
	masterAll(progress, ModuleHorizontal, 1) // one wrong each: 75% accuracy

	if stats.IsBossReady(ModuleHorizontal) {
		t.Fatalf("the fixture is wrong: accuracy %.2f should not clear the boss gate", progress.PracticeAccuracy)
	}

	state := NewGameStateWithStats(stats)
	state.StartPractice(ModuleHorizontal)

	if state.CurrentExercise == nil {
		t.Fatal("practice started with no exercise, so the player cannot reach the boss gate")
	}
	if !state.NextPracticeExercise() {
		t.Error("practice ended while the boss gate was still unmet, which is the deadlock")
	}
}

// TestNextPracticeExercise_EndsOnlyWhenBossGateMet is the other half of S2: the
// word "complete" is reserved for the state in which nothing is left to do, so
// the session ends exactly when every exercise is mastered AND the boss is
// unlocked.
func TestNextPracticeExercise_EndsOnlyWhenBossGateMet(t *testing.T) {
	stats := NewUserStats()
	completeLessons(stats, ModuleHorizontal)
	progress := stats.GetModuleProgress(ModuleHorizontal)
	masterAll(progress, ModuleHorizontal, 0)

	if !stats.IsBossReady(ModuleHorizontal) {
		t.Fatalf("the fixture is wrong: accuracy %.2f should clear the boss gate", progress.PracticeAccuracy)
	}

	state := NewGameStateWithStats(stats)
	state.StartPractice(ModuleHorizontal)

	if state.NextPracticeExercise() {
		t.Error("practice continued after everything was mastered and the boss was unlocked")
	}
}

// TestGameState_RecordsLessonAnswerPerExercise pins S4 for lessons: the
// per-exercise record the pool, the mastery count and the weakest list read is
// moved by a lesson answer too, not only by a practice answer.
func TestGameState_RecordsLessonAnswerPerExercise(t *testing.T) {
	stats := NewUserStats()
	state := NewGameStateWithStats(stats)
	state.StartLesson(ModuleHorizontal)

	exercise := state.CurrentExercise
	if exercise == nil {
		t.Fatal("expected a first lesson")
	}

	state.RecordCorrectAnswer(1.0, true)

	recorded := stats.GetModuleProgress(ModuleHorizontal).ExerciseStatsOrNil(exercise.ID)
	if recorded == nil {
		t.Fatal("a lesson answer recorded nothing for its own exercise")
	}
	if recorded.TotalCorrect != 1 || recorded.TotalAttempts != 1 {
		t.Errorf("a lesson answer recorded %d correct of %d attempts, want 1 of 1",
			recorded.TotalCorrect, recorded.TotalAttempts)
	}
}

// TestGameState_RecordsBossAnswerPerExercise pins S4 for the boss fight: a boss
// step is an answer to a real exercise and has to count like one.
func TestGameState_RecordsBossAnswerPerExercise(t *testing.T) {
	stats := NewUserStats()
	state := NewGameStateWithStats(stats)
	state.StartBoss(ModuleHorizontal)

	exercise := state.CurrentExercise
	if exercise == nil {
		t.Fatal("expected a first boss step")
	}

	state.RecordCorrectAnswer(1.0, true)

	recorded := stats.GetModuleProgress(ModuleHorizontal).ExerciseStatsOrNil(exercise.ID)
	if recorded == nil {
		t.Fatal("a boss answer recorded nothing for its own exercise")
	}
	if recorded.TotalCorrect != 1 {
		t.Errorf("a boss answer recorded %d correct, want 1", recorded.TotalCorrect)
	}
}

// TestGameState_PracticeAnswerCountsOnce pins S3's ownership move: the practice
// counters are moved by the answer recorder that owns the answer, exactly once,
// with the same call the screens already make for every other answer.
func TestGameState_PracticeAnswerCountsOnce(t *testing.T) {
	stats := NewUserStats()
	state := NewGameStateWithStats(stats)
	state.StartPractice(ModuleHorizontal)
	state.IsPracticeMode = true

	state.RecordCorrectAnswer(1.0, true)

	progress := stats.GetModuleProgress(ModuleHorizontal)
	if progress.PracticeAttempts != 1 {
		t.Errorf("one practice answer moved PracticeAttempts to %d, want 1", progress.PracticeAttempts)
	}
	if progress.PracticeCorrect != 1 {
		t.Errorf("one correct practice answer moved PracticeCorrect to %d, want 1", progress.PracticeCorrect)
	}
}

// TestRecordPracticeResult_StampsTheInjectedClock pins S3: the practice stamps
// come from the clock the caller passes, not from a second wall clock hidden in
// the recorder. Before this the recorder called time.Now() while every other
// timing decision in the session went through the injected clock.
func TestRecordPracticeResult_StampsTheInjectedClock(t *testing.T) {
	mp := &ModuleProgress{}
	when := time.Unix(1_234_567_890, 0)

	mp.RecordPracticeResult("test_001", true, when)

	stats := mp.ExerciseStatsOrNil("test_001")
	if stats == nil {
		t.Fatal("recording an answer created no exercise record")
	}
	if stats.LastAttempted != when.Unix() {
		t.Errorf("LastAttempted = %d, want the injected %d", stats.LastAttempted, when.Unix())
	}
	if !mp.LastPracticed.Equal(when) {
		t.Errorf("LastPracticed = %v, want the injected %v", mp.LastPracticed, when)
	}
}

// TestSelectPracticeExercise_UsesTheInjectedSource pins the other half of S3:
// selection draws from the source it is given, so a session can be replayed and
// a test can pin the order instead of flaking on the global source.
func TestSelectPracticeExercise_UsesTheInjectedSource(t *testing.T) {
	progress := &ModuleProgress{}

	sequence := func() []string {
		rng := rand.New(rand.NewSource(7))
		ids := make([]string, 0, 5)
		for i := 0; i < 5; i++ {
			ex := SelectRandomPracticeExercise(ModuleHorizontal, progress, rng, time.Now())
			if ex == nil {
				t.Fatal("expected a practice exercise")
			}
			ids = append(ids, ex.ID)
		}
		return ids
	}

	first, second := sequence(), sequence()
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("the same seed produced different picks at %d: %s vs %s", i, first[i], second[i])
		}
	}
}

// TestGameState_RecordsWrongAnswerPerExercise pins the other direction of S4: a
// wrong answer is what makes an exercise weak, and a lesson or boss mistake has
// to be visible to the pool.
func TestGameState_RecordsWrongAnswerPerExercise(t *testing.T) {
	stats := NewUserStats()
	state := NewGameStateWithStats(stats)
	state.StartLesson(ModuleHorizontal)

	exercise := state.CurrentExercise
	if exercise == nil {
		t.Fatal("expected a first lesson")
	}

	state.RecordIncorrectAnswer()

	recorded := stats.GetModuleProgress(ModuleHorizontal).ExerciseStatsOrNil(exercise.ID)
	if recorded == nil {
		t.Fatal("a wrong lesson answer recorded nothing for its own exercise")
	}
	if recorded.TotalWrong != 1 {
		t.Errorf("a wrong answer recorded %d wrong, want 1", recorded.TotalWrong)
	}
}
