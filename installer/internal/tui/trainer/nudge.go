package trainer

import "fmt"

// BossNudgeKeystrokes is how many keystrokes the fallback nudge reveals.
const BossNudgeKeystrokes = 2

// BossStepNudge is what the boss screen reveals when the player asks for help: the
// step's own hint when it has one, and otherwise the first keystrokes of its answer.
//
// The fallback exists because `Tab` was dead on every step that shipped no hint, which
// was all 35 of them. It never prints the whole answer: a two-keystroke answer gives its
// first key, and a single-keystroke answer -- whose first keystroke is its solution --
// gives nothing rather than handing the step over. The shipped corpus authors a hint on
// every step, so this is the rule for the content that comes next.
func BossStepNudge(exercise *Exercise) string {
	if exercise == nil {
		return ""
	}
	if exercise.Hint != "" {
		return exercise.Hint
	}

	answer := []rune(exercise.Optimal)
	reveal := BossNudgeKeystrokes
	if reveal >= len(answer) {
		reveal = len(answer) - 1
	}
	if reveal <= 0 {
		return ""
	}
	return fmt.Sprintf("Start with `%s`", string(answer[:reveal]))
}
