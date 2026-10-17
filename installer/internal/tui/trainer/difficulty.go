package trainer

// Difficulty is a contract, not a comment. Every shipped exercise declares a Level from
// 1 to 10, the level is what a correct answer pays, and it is what steers the practice
// mix towards the next thing to learn.
//
// Before this file the Level field was authored on all 244 exercises and read by
// nothing: the points came from a hand-typed Points field that plateaued, so a level 6
// exercise paid exactly what a level 3 one paid, and the pool drew almost uniformly
// from everything the player had not mastered.

// MinExerciseLevel and MaxExerciseLevel bound the declared difficulty. They are the
// range the corpus guard enforces, and the range every reader clamps to.
const (
	MinExerciseLevel = 1
	MaxExerciseLevel = 10
)

// ClampExerciseLevel brings a level into the contract's range. Level 0 means "not
// declared" -- a synthetic exercise in a test, or one written before the field was
// required -- and is read as the easiest level rather than as the hardest.
func ClampExerciseLevel(level int) int {
	if level < MinExerciseLevel {
		return MinExerciseLevel
	}
	if level > MaxExerciseLevel {
		return MaxExerciseLevel
	}
	return level
}

// PointsForLevel is what a correct answer to an exercise of this level is worth before
// the optimal, speed and combo multipliers. It is strictly increasing, so difficulty
// always pays more than ease.
func PointsForLevel(level int) int {
	return 10 + 5*(ClampExerciseLevel(level)-1)
}

// ExercisePoints is the base points of one exercise: its level's own value, or the
// authored Points when the author deliberately paid more (a boss step, a module's
// capstone). The level is a floor rather than the whole rule so a hand-tuned reward
// survives, and a floor rather than nothing so a hard exercise can never pay less than
// an easy one.
func ExercisePoints(exercise *Exercise) int {
	if exercise == nil {
		return 0
	}
	if exercise.Points > PointsForLevel(exercise.Level) {
		return exercise.Points
	}
	return PointsForLevel(exercise.Level)
}

// PracticeFrontierWeight is what the pool adds to an exercise one level above the
// highest level the module has fully mastered: the frontier. It is what turns the
// practice mix into a progression instead of a lottery.
const PracticeFrontierWeight = 10

// PracticeBeyondFrontierWeight is what the pool adds to an exercise further out than the
// frontier. It is smaller than the frontier's own weight -- the next step is served most
// often -- but not zero: a player who is curious about what comes later still meets it.
const PracticeBeyondFrontierWeight = 4

// MasteredLevelFloor is the highest level of a module that is fully mastered, or 0 when
// no level is. A level counts as mastered only when every exercise at or below it is,
// which is what makes "the next level to learn" a fact about the player rather than a
// guess. It is read-only, like every other question the pool asks.
func MasteredLevelFloor(module ModuleID, progress *ModuleProgress) int {
	unmastered := map[int]bool{}
	for _, lesson := range GetLessons(module) {
		if !progress.ExerciseStatsOrNil(lesson.ID).IsMastered() {
			unmastered[ClampExerciseLevel(lesson.Level)] = true
		}
	}

	floor := 0
	for level := MinExerciseLevel; level <= MaxExerciseLevel; level++ {
		if unmastered[level] {
			break
		}
		floor = level
	}
	return floor
}

// frontierBonus is what the pool adds to an unmastered exercise at this level, given the
// level the module has fully mastered. A level already behind the player earns nothing:
// it is review, and the review weight is a question about the clock.
func frontierBonus(level, floor int) int {
	switch level = ClampExerciseLevel(level); {
	case level <= floor:
		return 0
	case level == floor+1:
		return PracticeFrontierWeight
	default:
		return PracticeBeyondFrontierWeight
	}
}
