package trainer

import "unicode/utf8"

// The boss clock is calibrated, not guessed. Before this rule the same shape of step was
// given five seconds in one module and thirty in another, and a step whose mission ran
// to 254 characters had fifteen seconds to read it and answer: those fights were lost to
// the clock rather than to the player, which is the opposite of what a boss is for.
//
// The rule below is the only owner of "how long should this step take": the corpus guard
// checks every shipped step against it, and an author tuning a step by hand is tuning a
// number this rule already floors.

const (
	// BossStepBaseSeconds is what every step gets before its own shape is counted:
	// find the cursor, read the first words of the mission, decide.
	BossStepBaseSeconds = 4
	// BossStepReadCharsPerSecond is how fast the mission is read. Twenty characters a
	// second is comfortable reading, not speed-reading.
	BossStepReadCharsPerSecond = 20
	// BossStepAnswerSecondsPerChar is what each character of the answer costs to type.
	BossStepAnswerSecondsPerChar = 1
	// BossStepMaxSeconds caps the recommendation: past a minute and a half a step is
	// no longer a fight, whatever its shape.
	BossStepMaxSeconds = 90

	// BossBonusTimeMinSeconds and BossBonusTimeMaxSeconds bound the extra seconds a
	// won step grants the step that follows it. A fight that grants none makes a win
	// streak worth nothing; a fight that grants minutes makes its clock decorative.
	BossBonusTimeMinSeconds = 15
	BossBonusTimeMaxSeconds = 60
)

// BossStepRecommendedSeconds is the least time a step's own shape needs: the mission's
// reading time plus the answer's typing time, over the base.
func BossStepRecommendedSeconds(exercise *Exercise) int {
	if exercise == nil {
		return BossStepBaseSeconds
	}

	read := utf8.RuneCountInString(exercise.Mission) / BossStepReadCharsPerSecond
	answer := utf8.RuneCountInString(exercise.Optimal) * BossStepAnswerSecondsPerChar

	recommended := BossStepBaseSeconds + read + answer
	if recommended > BossStepMaxSeconds {
		return BossStepMaxSeconds
	}
	return recommended
}

// BossStepTimeLimitIsFair reports whether a step's declared limit gives it at least the
// time its shape needs. It is what the corpus guard asks about every shipped step.
func BossStepTimeLimitIsFair(step BossStep) bool {
	return step.TimeLimit >= BossStepRecommendedSeconds(&step.Exercise)
}
