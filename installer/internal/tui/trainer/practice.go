package trainer

import (
	"math/rand"
	"sort"
	"time"
)

// MasteryThreshold is the number of consecutive correct answers needed to master an exercise
const MasteryThreshold = 3

// MasteredReviewWeight is the weight a mastered exercise keeps in the practice
// pool while its review is not due. It is deliberately the smallest positive
// weight: mastered exercises are review rather than the main event, but they never
// leave the pool. When the mastered weight was zero the pool could empty, and a
// module whose exercises were all mastered but whose boss gate was unmet could
// neither be practised nor fought -- the deadlock this constant removes.
const MasteredReviewWeight = 1

// MasteredReviewDueWeight is the weight a mastered exercise takes once its review
// is due: high enough that a due review is served promptly rather than buried under
// exercises the player has never touched.
const MasteredReviewDueWeight = 12

// MasteryReviewInterval is how long a mastered exercise stays out of the way before
// it comes back for review. The trainer's own documentation has promised spaced
// repetition since it was written and the per-exercise timestamp it needs was
// recorded and never read; this is the rule that reads it. Twelve hours is long
// enough that a single session is not spent re-answering what was just mastered and
// short enough that the next session starts with it.
const MasteryReviewInterval = 12 * time.Hour

// IsReviewDue reports whether a mastered exercise has waited long enough to be
// reviewed again. An exercise that was never mastered is never "due": it is simply
// in the pool. A mastered exercise with no recorded attempt is due, because nothing
// says when it was last seen.
func (stats *ExerciseStats) IsReviewDue(now time.Time) bool {
	if !stats.IsMastered() {
		return false
	}
	if stats.LastAttempted == 0 {
		return true
	}
	return now.Sub(time.Unix(stats.LastAttempted, 0)) >= MasteryReviewInterval
}

// GetExerciseStats returns stats for a specific exercise, creating if needed.
// It is the write-side accessor: recording an answer needs a record to write to.
// A read path must use ExerciseStatsOrNil instead, or merely looking at progress
// writes to the player's profile.
func (mp *ModuleProgress) GetExerciseStats(exerciseID string) *ExerciseStats {
	if mp.ExerciseStats == nil {
		mp.ExerciseStats = make(map[string]*ExerciseStats)
	}
	if _, ok := mp.ExerciseStats[exerciseID]; !ok {
		mp.ExerciseStats[exerciseID] = &ExerciseStats{}
	}
	return mp.ExerciseStats[exerciseID]
}

// ExerciseStatsOrNil returns the recorded stats for an exercise without
// creating an entry, or nil when the exercise has never been attempted. It is
// the read-side accessor and the reason it exists: the trainer menu, the
// practice pool and the completion checks all ask about exercises the player may
// never have touched, and manufacturing an empty record for each of them wrote
// the player's profile from a screen that only looked at it.
//
// A nil result answers every question here the same way an empty record does,
// because every recorded counter starts at zero and IsMastered is nil-safe.
func (mp *ModuleProgress) ExerciseStatsOrNil(exerciseID string) *ExerciseStats {
	if mp == nil || mp.ExerciseStats == nil {
		return nil
	}
	return mp.ExerciseStats[exerciseID]
}

// RecordExerciseResult records one answer to one exercise: the per-exercise
// counters that mastery, the weighted pool and the weakest list read. It is
// called for an answer in every mode, because a lesson answer and a boss step
// are answers too; only RecordPracticeAttempt below is practice-specific.
//
// now is the answer's time, passed in rather than read from the wall clock: the
// session already injects a clock for every other timing decision, and a second
// hidden clock made practice timing untestable and inconsistent.
func (mp *ModuleProgress) RecordExerciseResult(exerciseID string, correct bool, now time.Time) {
	stats := mp.GetExerciseStats(exerciseID)
	stats.TotalAttempts++
	stats.LastAttempted = now.Unix()

	if correct {
		stats.TotalCorrect++
		stats.ConsecutiveRight++

		// Check for mastery
		if stats.ConsecutiveRight >= MasteryThreshold {
			stats.Mastered = true
		}
	} else {
		stats.TotalWrong++
		stats.ConsecutiveRight = 0
		// Un-master if they get it wrong
		stats.Mastered = false
	}
}

// RecordPracticeAttempt records one practice answer in the module-level counters the
// accuracy line reads, and in the recent window the boss gate reads. It is
// practice-only on purpose: a lesson answer or a boss step must not move the accuracy
// the practice gate is measured with.
func (mp *ModuleProgress) RecordPracticeAttempt(correct bool, now time.Time) {
	mp.PracticeAttempts++
	if correct {
		mp.PracticeCorrect++
	}
	if mp.PracticeAttempts > 0 {
		mp.PracticeAccuracy = float64(mp.PracticeCorrect) / float64(mp.PracticeAttempts)
	}
	mp.LastPracticed = now

	// The recent window is what the boss gate reads, so a bad first run cannot tax
	// the player forever. It keeps the last BossGateWindow answers and nothing older.
	if len(mp.RecentResults) >= BossGateWindow {
		mp.RecentResults = mp.RecentResults[len(mp.RecentResults)-BossGateWindow+1:]
	}
	mp.RecentResults = append(mp.RecentResults, correct)
}

// RecentAccuracy is the accuracy over the last BossGateWindow practice answers, and
// how many answers that window holds. A profile written before the window existed has
// none, so it falls back to the lifetime accuracy: the gate then behaves exactly as it
// did before this rule, instead of reading a fresh player's history as 0%.
func (mp *ModuleProgress) RecentAccuracy() (float64, int) {
	if mp == nil {
		return 0, 0
	}
	if len(mp.RecentResults) == 0 {
		return mp.PracticeAccuracy, mp.PracticeAttempts
	}
	correct := 0
	for _, ok := range mp.RecentResults {
		if ok {
			correct++
		}
	}
	return float64(correct) / float64(len(mp.RecentResults)), len(mp.RecentResults)
}

// ReviewsDue counts the module's mastered exercises whose review interval has passed
// at now. It is read-only, and it is the number the trainer menu shows and the rule
// that keeps a practice session alive after everything is mastered.
func (mp *ModuleProgress) ReviewsDue(module ModuleID, now time.Time) int {
	due := 0
	for _, lesson := range GetLessons(module) {
		if mp.ExerciseStatsOrNil(lesson.ID).IsReviewDue(now) {
			due++
		}
	}
	return due
}

// RecordPracticeResult records the result of a practice attempt: the exercise's
// own record plus the module-level practice counters, through the two single
// owners above. It is the composition callers use when they only hold a
// ModuleProgress; the game state calls the two halves separately, so a lesson or
// boss answer records its exercise without touching the practice counters.
//
// Callers must not also count the submission elsewhere.
func (mp *ModuleProgress) RecordPracticeResult(exerciseID string, correct bool, now time.Time) {
	mp.RecordExerciseResult(exerciseID, correct, now)
	mp.RecordPracticeAttempt(correct, now)
}

// GetPracticeWeightAt calculates the weight for an exercise at one moment (higher =
// more likely to appear). now is a parameter for the same reason every other timing
// decision in this package takes one: a due review is a question about the clock, and
// a hidden clock would make the pool untestable.
//
// A nil receiver is an exercise that has never been attempted, which is the same
// question as an empty record and must not be a nil dereference: the pool asks
// this for every lesson of a module the player may have never opened.
func (stats *ExerciseStats) GetPracticeWeightAt(now time.Time) int {
	if stats == nil {
		return 15 // Never attempted: medium-high priority.
	}

	if stats.IsMastered() {
		if stats.IsReviewDue(now) {
			return MasteredReviewDueWeight
		}
		return MasteredReviewWeight // Resting review; never zero.
	}

	// Base weight
	weight := 10

	// Add weight based on wrong answers (more wrong = higher weight)
	weight += stats.TotalWrong * 5

	// Reduce weight based on consecutive correct (getting better)
	weight -= stats.ConsecutiveRight * 2

	// Never practiced = medium-high priority
	if stats.TotalAttempts == 0 {
		weight = 15
	}

	// Ensure minimum weight of 1 for non-mastered
	if weight < 1 {
		weight = 1
	}

	return weight
}

// defaultPracticeRand is the source used when a caller has none. It is seeded
// once from the clock; a caller that owns a session passes its own source so a
// test can pin the selection instead of hoping.
var defaultPracticeRand = rand.New(rand.NewSource(time.Now().UnixNano()))

// SelectRandomPracticeExercise selects an exercise using weighted random
// selection. rng is the randomness source and nil means the package-level one.
//
// It is read-only and it never returns nil for a module that has lessons: a
// mastered exercise keeps MasteredReviewWeight, so the pool is empty exactly
// when the module has nothing to practise. nil is therefore reserved for a
// module with no exercises at all.
func SelectRandomPracticeExercise(module ModuleID, progress *ModuleProgress, rng *rand.Rand, now time.Time) *Exercise {
	lessons := GetLessons(module)
	if len(lessons) == 0 {
		return nil
	}

	// Build weighted pool
	type weightedEx struct {
		exercise Exercise
		weight   int
	}

	pool := make([]weightedEx, 0, len(lessons))
	totalWeight := 0

	// The mix leans on the frontier (difficulty.go): the next level to learn is served
	// most often, the levels beyond it occasionally, and the levels the player has
	// already mastered not at all -- those are review, and their weight is a question
	// about the clock rather than about difficulty.
	floor := MasteredLevelFloor(module, progress)

	for _, lesson := range lessons {
		stats := progress.ExerciseStatsOrNil(lesson.ID)
		weight := stats.GetPracticeWeightAt(now)
		if !stats.IsMastered() {
			weight += frontierBonus(lesson.Level, floor)
		}

		if weight > 0 {
			pool = append(pool, weightedEx{exercise: lesson, weight: weight})
			totalWeight += weight
		}
	}

	// No selectable exercise: only reachable if a future weight rule returns
	// zero for every lesson, never with the shipped weights.
	if totalWeight == 0 {
		return nil
	}

	if rng == nil {
		rng = defaultPracticeRand
	}

	// Weighted random selection
	r := rng.Intn(totalWeight)
	cumulative := 0

	for _, we := range pool {
		cumulative += we.weight
		if r < cumulative {
			ex := we.exercise
			ex.Type = ExercisePractice
			return &ex
		}
	}

	// Fallback (shouldn't happen)
	if len(pool) > 0 {
		ex := pool[0].exercise
		ex.Type = ExercisePractice
		return &ex
	}

	return nil
}

// GetPracticeStats returns summary stats for practice mode
type PracticeStats struct {
	TotalExercises   int
	MasteredCount    int
	RemainingCount   int
	OverallAccuracy  float64
	WeakestExercises []string // IDs of exercises with most errors
	// PracticeComplete means every exercise is mastered. It is NOT the rule that
	// ends a practice session: a module with everything mastered and an unmet boss
	// gate still has review to do, which is what UserStats.PracticeComplete asks.
	PracticeComplete bool
}

// GetPracticeStatsForModule calculates practice statistics for a module.
//
// It is read-only: the trainer menu queries it for every module on each render,
// so it reads the recorded per-exercise stats without creating entries for
// exercises that were never attempted. Mastery is read through
// ExerciseStats.IsMastered, the same predicate weighted practice selection uses.
func GetPracticeStatsForModule(module ModuleID, progress *ModuleProgress) PracticeStats {
	lessons := GetLessons(module)

	stats := PracticeStats{
		TotalExercises:   len(lessons),
		WeakestExercises: make([]string, 0),
	}

	if len(lessons) == 0 {
		return stats
	}

	// Collect exercise stats
	type exError struct {
		id     string
		errors int
	}
	errorList := make([]exError, 0, len(lessons))

	for _, lesson := range lessons {
		var exStats *ExerciseStats
		if progress != nil && progress.ExerciseStats != nil {
			exStats = progress.ExerciseStats[lesson.ID]
		}
		if exStats.IsMastered() {
			stats.MasteredCount++
		}
		if exStats != nil && exStats.TotalWrong > 0 {
			errorList = append(errorList, exError{id: lesson.ID, errors: exStats.TotalWrong})
		}
	}

	stats.RemainingCount = stats.TotalExercises - stats.MasteredCount
	stats.PracticeComplete = stats.RemainingCount == 0
	if progress != nil {
		stats.OverallAccuracy = progress.PracticeAccuracy
	}

	// Sort by errors descending
	sort.Slice(errorList, func(i, j int) bool {
		return errorList[i].errors > errorList[j].errors
	})

	// Take top 3 weakest
	for i := 0; i < len(errorList) && i < 3; i++ {
		stats.WeakestExercises = append(stats.WeakestExercises, errorList[i].id)
	}

	return stats
}

// ResetModulePractice resets all practice data for a module
func (mp *ModuleProgress) ResetModulePractice() {
	mp.ExerciseStats = make(map[string]*ExerciseStats)
	mp.PracticeAttempts = 0
	mp.PracticeCorrect = 0
	mp.PracticeAccuracy = 0
	mp.RecentResults = nil
	mp.WeakExercises = []string{}
	mp.LastPracticed = time.Time{}
}

// IsPracticeComplete returns true if all exercises are mastered. It is
// read-only, so asking it cannot write an empty record per lesson into the
// player's profile, and a nil receiver is not complete.
func (mp *ModuleProgress) IsPracticeComplete(module ModuleID) bool {
	lessons := GetLessons(module)
	if len(lessons) == 0 {
		return false
	}

	for _, lesson := range lessons {
		if !mp.ExerciseStatsOrNil(lesson.ID).IsMastered() {
			return false
		}
	}

	return true
}
