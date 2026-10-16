package trainer

import "testing"

// TestPointsForLevelIsStrictlyIncreasing pins S7's first half: difficulty always pays
// more than ease, on every level of the contract and not only on the authored ones.
func TestPointsForLevelIsStrictlyIncreasing(t *testing.T) {
	previous := 0
	for level := MinExerciseLevel; level <= MaxExerciseLevel; level++ {
		points := PointsForLevel(level)
		if points <= previous {
			t.Errorf("level %d pays %d, which is not more than level %d's %d", level, points, level-1, previous)
		}
		previous = points
	}

	// Out-of-contract levels are clamped rather than trusted: a level 0 means "not
	// declared" and is read as the easiest, and nothing can pay more than level 10.
	if got := PointsForLevel(0); got != PointsForLevel(MinExerciseLevel) {
		t.Errorf("level 0 pays %d, want the level %d value %d", got, MinExerciseLevel, PointsForLevel(MinExerciseLevel))
	}
	if got := PointsForLevel(99); got != PointsForLevel(MaxExerciseLevel) {
		t.Errorf("level 99 pays %d, want the level %d value %d", got, MaxExerciseLevel, PointsForLevel(MaxExerciseLevel))
	}
}

// TestExercisePointsKeepTheLevelAsAFloor pins the rule the score is paid by: an
// exercise pays its level's value, and an authored reward above it survives. A harder
// exercise can therefore never pay less than an easier one, which is what was wrong
// when the points came from a hand-typed field alone.
func TestExercisePointsKeepTheLevelAsAFloor(t *testing.T) {
	hard := &Exercise{Level: 6, Points: 20}
	easy := &Exercise{Level: 3, Points: 20}
	if ExercisePoints(hard) <= ExercisePoints(easy) {
		t.Errorf("a level 6 exercise pays %d and a level 3 one %d; difficulty must pay more",
			ExercisePoints(hard), ExercisePoints(easy))
	}

	rewarded := &Exercise{Level: 5, Points: 50}
	if got := ExercisePoints(rewarded); got != 50 {
		t.Errorf("an authored reward of 50 points was overridden to %d", got)
	}

	if got := ExercisePoints(nil); got != 0 {
		t.Errorf("a nil exercise pays %d, want 0", got)
	}
}

// TestCalculatePointsUsesTheLevel pins that the rule reaches the score the player is
// actually paid, not just the helper.
func TestCalculatePointsUsesTheLevel(t *testing.T) {
	exercise := &Exercise{Level: 6, Points: 20, Optimal: "w"}
	if got, want := CalculatePoints(exercise, 10, false, 1), PointsForLevel(6); got != want {
		t.Errorf("a correct answer to a level 6 exercise paid %d, want the level's %d", got, want)
	}
}

// TestPracticeMixLeansOnTheFrontier pins S7's second half: the pool serves the next
// level to learn most often, the levels beyond it occasionally, and the levels already
// behind the player not at all.
func TestPracticeMixLeansOnTheFrontier(t *testing.T) {
	if PracticeFrontierWeight <= 0 {
		t.Fatalf("the frontier weight is %d; a frontier the pool does not favour is not a rule", PracticeFrontierWeight)
	}
	if PracticeBeyondFrontierWeight <= 0 || PracticeBeyondFrontierWeight >= PracticeFrontierWeight {
		t.Fatalf("the beyond-frontier weight %d must be positive and below the frontier's %d",
			PracticeBeyondFrontierWeight, PracticeFrontierWeight)
	}

	if got := frontierBonus(3, 2); got != PracticeFrontierWeight {
		t.Errorf("one level above the mastered floor is worth %d, want the frontier weight %d", got, PracticeFrontierWeight)
	}
	if got := frontierBonus(6, 2); got != PracticeBeyondFrontierWeight {
		t.Errorf("four levels above the mastered floor is worth %d, want the beyond weight %d", got, PracticeBeyondFrontierWeight)
	}
	if got := frontierBonus(1, 2); got != 0 {
		t.Errorf("a level behind the mastered floor is worth %d, want nothing", got)
	}
}

// TestMasteredLevelFloorCountsOnlyFullyMasteredLevels pins what the frontier is
// measured from: a level counts as mastered only when every exercise at or below it is.
func TestMasteredLevelFloorCountsOnlyFullyMasteredLevels(t *testing.T) {
	stats := NewUserStats()
	progress := stats.GetModuleProgress(ModuleTextObjects)
	lessons := GetLessons(ModuleTextObjects)
	if len(lessons) == 0 {
		t.Fatal("the text objects module has no lessons")
	}

	if got := MasteredLevelFloor(ModuleTextObjects, progress); got != 0 {
		t.Errorf("an untouched module has mastered floor %d, want 0", got)
	}

	// Master everything but one exercise of the module's hardest level: the floor is
	// the last level that is complete, not the hardest one touched.
	hardest := 0
	for _, lesson := range lessons {
		if lesson.Level > hardest {
			hardest = lesson.Level
		}
	}
	for _, lesson := range lessons {
		if lesson.Level == hardest {
			continue
		}
		progress.GetExerciseStats(lesson.ID).Mastered = true
	}
	if got, want := MasteredLevelFloor(ModuleTextObjects, progress), hardest-1; got != want {
		t.Errorf("mastered floor is %d, want %d: the hardest level still has an unmastered exercise", got, want)
	}

	for _, lesson := range lessons {
		progress.GetExerciseStats(lesson.ID).Mastered = true
	}
	if got := MasteredLevelFloor(ModuleTextObjects, progress); got < hardest {
		t.Errorf("mastered floor is %d with everything mastered, want at least the hardest level %d", got, hardest)
	}
}

// TestPoolServesTheFrontierMostOften pins the rule end to end through the pool's own
// weighting, which is what decides what the player actually sees.
func TestPoolServesTheFrontierMostOften(t *testing.T) {
	progress := &ModuleProgress{}
	lessons := GetLessons(ModuleTextObjects)

	// Master levels 1 and 2 so the frontier sits at 3.
	for _, lesson := range lessons {
		if lesson.Level <= 2 {
			progress.GetExerciseStats(lesson.ID).Mastered = true
			progress.GetExerciseStats(lesson.ID).LastAttempted = reviewNow.Unix()
		}
	}
	floor := MasteredLevelFloor(ModuleTextObjects, progress)
	if floor != 2 {
		t.Fatalf("the fixture is wrong: mastered floor is %d, want 2", floor)
	}

	// The next level to learn and one further out both exist in this module's corpus.
	if frontierBonus(3, floor) <= frontierBonus(5, floor) {
		t.Error("the level past the frontier is served at least as often as the next level to learn")
	}
	// And a level already behind the player carries no frontier weight at all.
	if got := frontierBonus(2, floor); got != 0 {
		t.Errorf("a mastered level is worth a bonus of %d, want nothing", got)
	}
}
