package trainer

import (
	"fmt"
	"strings"
	"testing"
)

// corpusExercises walks every shipped exercise: the lessons of every module and every
// step of every boss fight. The guards below are about the corpus rather than about one
// screen, so they read the same content the player does.
func corpusExercises(t *testing.T) map[string]Exercise {
	t.Helper()
	all := map[string]Exercise{}
	for _, module := range GetAllModules() {
		for _, lesson := range GetLessons(module.ID) {
			if _, seen := all[lesson.ID]; seen {
				t.Errorf("two exercises share the id %q", lesson.ID)
			}
			all[lesson.ID] = lesson
		}
		if boss := GetBoss(module.ID); boss != nil {
			for _, step := range boss.Steps {
				if _, seen := all[step.Exercise.ID]; seen {
					t.Errorf("two exercises share the id %q", step.Exercise.ID)
				}
				all[step.Exercise.ID] = step.Exercise
			}
		}
	}
	if len(all) == 0 {
		t.Fatal("the corpus is empty")
	}
	return all
}

// TestCorpusLevelsAreInRange pins S7's contract against the shipped content: every
// exercise declares a difficulty the rules can read. The one exercise that declared
// level 11 was corrected rather than clamped, so this guard is about the content and
// not about a helper's tolerance.
func TestCorpusLevelsAreInRange(t *testing.T) {
	for _, exercise := range corpusExercises(t) {
		if exercise.Level < MinExerciseLevel || exercise.Level > MaxExerciseLevel {
			t.Errorf("%s declares level %d, outside the contract's %d..%d",
				exercise.ID, exercise.Level, MinExerciseLevel, MaxExerciseLevel)
		}
	}
}

// TestCorpusBossClockIsFair pins S8 against the shipped content: no step gives the
// player less time than its own shape needs. Before this guard the same shape of step
// was worth five seconds in one module and thirty in another, and a step whose mission
// ran to 254 characters had fifteen seconds to read it and answer.
func TestCorpusBossClockIsFair(t *testing.T) {
	checked := 0
	for _, module := range GetAllModules() {
		boss := GetBoss(module.ID)
		if boss == nil {
			continue
		}
		for _, step := range boss.Steps {
			checked++
			recommended := BossStepRecommendedSeconds(&step.Exercise)
			if step.TimeLimit < recommended {
				t.Errorf("%s gets %ds but its mission and answer need %ds: %q / %q",
					step.Exercise.ID, step.TimeLimit, recommended, step.Exercise.Mission, step.Exercise.Optimal)
			}
			if step.TimeLimit > BossStepMaxSeconds {
				t.Errorf("%s gets %ds, past the %ds a step may take; the fight is no longer a fight",
					step.Exercise.ID, step.TimeLimit, BossStepMaxSeconds)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no boss step was checked")
	}
}

// TestCorpusBossBonusTimeIsInRange pins the other half of S8: the extra time a won step
// grants. One fight declared none, which made its win streak worth nothing, and another
// declared two minutes, which made the clock decorative.
func TestCorpusBossBonusTimeIsInRange(t *testing.T) {
	checked := 0
	for _, module := range GetAllModules() {
		boss := GetBoss(module.ID)
		if boss == nil {
			continue
		}
		checked++
		if boss.BonusTime < BossBonusTimeMinSeconds || boss.BonusTime > BossBonusTimeMaxSeconds {
			t.Errorf("%s grants %ds for a won step, outside the %d..%ds range",
				boss.ID, boss.BonusTime, BossBonusTimeMinSeconds, BossBonusTimeMaxSeconds)
		}
	}
	if checked == 0 {
		t.Fatal("no boss fight was checked")
	}
}

// TestCorpusMissionDoesNotSpoilTheAnswer pins the hint rule at the other end: the
// mission may name the target, but it must not print the keystrokes. A mission that
// contains its own Optimal hands the answer over before the player has looked, which is
// what made several steps trivial.
func TestCorpusMissionDoesNotSpoilTheAnswer(t *testing.T) {
	for _, exercise := range corpusExercises(t) {
		if exercise.Optimal == "" {
			continue
		}
		if containsKeystrokes(exercise.Mission, exercise.Optimal) {
			t.Errorf("%s states its own answer %q in the mission: %q", exercise.ID, exercise.Optimal, exercise.Mission)
		}
	}
}

// containsKeystrokes reports whether a mission quotes its own optimal answer as a
// keystroke sequence. A one-character answer is deliberately exempt: "move to the 'e'"
// is the mission of every motion exercise in the corpus, and the letter is the target,
// not the answer.
func containsKeystrokes(mission, optimal string) bool {
	if len([]rune(optimal)) < 2 {
		return false
	}
	needle := fmt.Sprintf("`%s`", optimal)
	return strings.Contains(mission, needle)
}
