package trainer

import (
	"testing"
	"time"
)

// reviewNow is the clock the review tests measure against, so the intervals they
// assert on are values rather than the moment the suite ran.
var reviewNow = time.Unix(1_700_000_000, 0)

// TestMasteredExerciseIsDueAfterTheReviewInterval pins S5's schedule: mastery is a
// state the creature is in, not a grave it stays in. A mastered exercise comes back
// once the interval has passed, and an exercise that was never mastered is never
// "due" -- it is simply in the pool.
func TestMasteredExerciseIsDueAfterTheReviewInterval(t *testing.T) {
	due := &ExerciseStats{Mastered: true, LastAttempted: reviewNow.Add(-MasteryReviewInterval - time.Minute).Unix()}
	if !due.IsReviewDue(reviewNow) {
		t.Error("a mastered exercise past the interval is not due for review")
	}

	resting := &ExerciseStats{Mastered: true, LastAttempted: reviewNow.Add(-time.Minute).Unix()}
	if resting.IsReviewDue(reviewNow) {
		t.Error("a mastered exercise answered a minute ago is already due for review")
	}

	neverMastered := &ExerciseStats{LastAttempted: reviewNow.Add(-100 * MasteryReviewInterval).Unix()}
	if neverMastered.IsReviewDue(reviewNow) {
		t.Error("an exercise that was never mastered is reported as a due review")
	}

	if !(&ExerciseStats{Mastered: true}).IsReviewDue(reviewNow) {
		t.Error("a mastered exercise with no recorded attempt is not due for review")
	}
}

// TestPracticeWeight_PrefersADueReview pins the pool's ordering: a due review is worth
// more than one that is still resting, and the resting one is still in the pool.
func TestPracticeWeight_PrefersADueReview(t *testing.T) {
	due := &ExerciseStats{Mastered: true, LastAttempted: reviewNow.Add(-MasteryReviewInterval - time.Minute).Unix()}
	resting := &ExerciseStats{Mastered: true, LastAttempted: reviewNow.Unix()}

	if due.GetPracticeWeightAt(reviewNow) <= resting.GetPracticeWeightAt(reviewNow) {
		t.Errorf("a due review weighs %d and a resting one %d; the due review must weigh more",
			due.GetPracticeWeightAt(reviewNow), resting.GetPracticeWeightAt(reviewNow))
	}
	if got := resting.GetPracticeWeightAt(reviewNow); got != MasteredReviewWeight {
		t.Errorf("a resting mastered exercise weighs %d, want the review weight %d", got, MasteredReviewWeight)
	}
	if got := due.GetPracticeWeightAt(reviewNow); got != MasteredReviewDueWeight {
		t.Errorf("a due review weighs %d, want the due weight %d", got, MasteredReviewDueWeight)
	}
}

// TestReviewsDueCountsOnlyMasteredExercisesPastTheInterval pins the number the menu
// shows, which is also what keeps a practice session alive.
func TestReviewsDueCountsOnlyMasteredExercisesPastTheInterval(t *testing.T) {
	stats := NewUserStats()
	progress := stats.GetModuleProgress(ModuleHorizontal)
	lessons := GetLessons(ModuleHorizontal)
	if len(lessons) < 2 {
		t.Skip("the horizontal module needs at least two lessons")
	}

	for _, lesson := range lessons {
		recorded := progress.GetExerciseStats(lesson.ID)
		recorded.Mastered = true
		recorded.LastAttempted = reviewNow.Unix()
	}
	if got := stats.ReviewsDue(ModuleHorizontal, reviewNow); got != 0 {
		t.Errorf("%d reviews due on a freshly mastered module, want 0", got)
	}

	progress.GetExerciseStats(lessons[0].ID).LastAttempted = reviewNow.Add(-MasteryReviewInterval - time.Minute).Unix()
	if got := stats.ReviewsDue(ModuleHorizontal, reviewNow); got != 1 {
		t.Errorf("%d reviews due after ageing one exercise, want 1", got)
	}
}

// TestPracticeCompleteWaitsForDueReviews pins the session-end rule against the new
// pool: everything mastered and the boss unlocked is still not "nothing left to do"
// while a review is due.
func TestPracticeCompleteWaitsForDueReviews(t *testing.T) {
	stats := NewUserStats()
	completeLessons(stats, ModuleHorizontal)
	progress := stats.GetModuleProgress(ModuleHorizontal)

	lesson := GetLessons(ModuleHorizontal)[0].ID
	for i := 0; i < BossGateMinAttempts; i++ {
		progress.RecordPracticeResult(lesson, true, reviewNow)
	}
	for _, l := range GetLessons(ModuleHorizontal) {
		recorded := progress.GetExerciseStats(l.ID)
		recorded.Mastered = true
		recorded.LastAttempted = reviewNow.Unix()
	}

	if !stats.PracticeComplete(ModuleHorizontal, reviewNow) {
		t.Fatal("a module with everything mastered, the boss unlocked and nothing due is not complete")
	}

	progress.GetExerciseStats(lesson).LastAttempted = reviewNow.Add(-MasteryReviewInterval - time.Minute).Unix()
	if stats.PracticeComplete(ModuleHorizontal, reviewNow) {
		t.Error("a practice session ended while a review was due")
	}
}

// TestBossGateUsesTheRecentWindow pins S6: the accuracy the gate reads is the recent
// run, so a bad first attempt at a module cannot tax the player forever.
func TestBossGateUsesTheRecentWindow(t *testing.T) {
	stats := NewUserStats()
	completeLessons(stats, ModuleHorizontal)
	progress := stats.GetModuleProgress(ModuleHorizontal)
	lesson := GetLessons(ModuleHorizontal)[0].ID

	// A bad first run: ten wrong answers, which alone can never clear the gate.
	for i := 0; i < BossGateMinAttempts; i++ {
		progress.RecordPracticeResult(lesson, false, reviewNow)
	}
	if stats.IsBossReady(ModuleHorizontal) {
		t.Fatal("the fixture is wrong: the gate should be locked after ten wrong answers")
	}

	// Then a clean run of the same length.
	for i := 0; i < BossGateWindow; i++ {
		progress.RecordPracticeResult(lesson, true, reviewNow)
	}
	if !stats.IsBossReady(ModuleHorizontal) {
		t.Errorf("the boss gate still reads the lifetime accuracy %.2f instead of the recent run",
			progress.PracticeAccuracy)
	}
}

// TestBossGateLocksOnABadRecentRun is the other side of S6: the window is what the
// gate reads, so a bad recent run locks a gate that a good history would have opened.
func TestBossGateLocksOnABadRecentRun(t *testing.T) {
	stats := NewUserStats()
	completeLessons(stats, ModuleHorizontal)
	progress := stats.GetModuleProgress(ModuleHorizontal)
	lesson := GetLessons(ModuleHorizontal)[0].ID

	for i := 0; i < BossGateWindow; i++ {
		progress.RecordPracticeResult(lesson, true, reviewNow)
	}
	if !stats.IsBossReady(ModuleHorizontal) {
		t.Fatal("the fixture is wrong: a clean run should clear the gate")
	}

	for i := 0; i < BossGateWindow; i++ {
		progress.RecordPracticeResult(lesson, false, reviewNow)
	}
	if stats.IsBossReady(ModuleHorizontal) {
		t.Error("the boss gate stayed open after a full window of wrong answers")
	}
}

// TestRecentWindowIsBounded pins that the window keeps only the last answers: a
// history longer than the window must not make the gate read the whole history.
func TestRecentWindowIsBounded(t *testing.T) {
	progress := &ModuleProgress{}
	lesson := "test_001"

	for i := 0; i < BossGateWindow; i++ {
		progress.RecordPracticeResult(lesson, false, reviewNow)
	}
	for i := 0; i < BossGateWindow; i++ {
		progress.RecordPracticeResult(lesson, true, reviewNow)
	}

	if len(progress.RecentResults) != BossGateWindow {
		t.Errorf("the window holds %d answers, want %d", len(progress.RecentResults), BossGateWindow)
	}
	accuracy, attempts := progress.RecentAccuracy()
	if attempts != BossGateWindow || accuracy != 1 {
		t.Errorf("the recent window reads %.2f over %d answers, want 1.00 over %d",
			accuracy, attempts, BossGateWindow)
	}
	if progress.PracticeAccuracy >= 1 {
		t.Errorf("the lifetime accuracy is %.2f; the fixture needs a mixed history", progress.PracticeAccuracy)
	}
}
