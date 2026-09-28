package trainer

import (
	"strings"
)

// ValidationResult contains detailed validation information
type ValidationResult struct {
	IsCorrect       bool     // Answer produces the same result as the optimal solution
	IsInSolutions   bool     // Answer is in the predefined solutions list
	IsOptimal       bool     // Answer is the optimal solution
	TargetPosition  Position // Where the answer should end up
	ActualPosition  Position // Where the answer actually ended up
	OptimalSolution string   // The best solution
	AllSolutions    []string // All predefined valid solutions

	// The fields below are filled only when the buffer judge decided the answer
	// (Exercise.BufferVerified). BufferVerified therefore records which judge
	// ran, so a caller can tell a buffer comparison from a motion comparison
	// even when both happen to leave the cursor where they found it, and so the
	// interface shows a buffer preview only where a buffer was compared.
	//
	// TargetBuffer and ActualBuffer are the buffers the optimal solution and the
	// answer produce in the engine, in that order; ActualBuffer is what the
	// result screen shows. TargetMode and ActualMode are the modes the two
	// answers leave behind. They are compared because an answer can end in insert
	// mode where the optimal returns to normal mode, so the same buffer and
	// cursor can still be a different result.
	BufferVerified bool
	TargetBuffer   []string
	ActualBuffer   []string
	TargetMode     Mode
	ActualMode     Mode
}

// MismatchSummary names the parts of a buffer-verified result that diverged
// from the optimal's result: the buffer, the cursor, the mode, or an English
// combination of them. It returns "" when the answer matched and for results
// the buffer judge did not decide. The summary is derived from the compared
// results, never from the raw keystrokes: the point of the buffer judge is that
// the keys do not matter.
func (r ValidationResult) MismatchSummary() string {
	if r.IsCorrect || !r.BufferVerified {
		return ""
	}

	var parts []string
	if !sameLines(r.TargetBuffer, r.ActualBuffer) {
		parts = append(parts, "buffer")
	}
	if r.TargetPosition != r.ActualPosition {
		parts = append(parts, "cursor")
	}
	if r.TargetMode != r.ActualMode {
		parts = append(parts, "mode")
	}

	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0] + " differs"
	default:
		return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1] + " differ"
	}
}

// ShouldSkipSimulation reports whether an exercise is validated and rendered
// without running the Vim simulator:
//  1. Ex commands (start with : / ?)
//  2. Substitution module (r, R, s, S, ~, etc. are edit commands)
//  3. Macros module (q, @, :normal, :g/ are not pure motions)
//  4. Regex module (/, ?, :vimgrep, etc.)
//
// It is the single owner of this predicate: ValidateAnswerDetailed and both
// render sites in the tui package call it, so the three cannot drift apart.
func ShouldSkipSimulation(exercise *Exercise) bool {
	isExCommand := len(exercise.Solutions) > 0 && len(exercise.Solutions[0]) > 0 &&
		(exercise.Solutions[0][0] == ':' || exercise.Solutions[0][0] == '/' || exercise.Solutions[0][0] == '?')
	isNonMotionModule := exercise.Module == ModuleSubstitution ||
		exercise.Module == ModuleMacros ||
		exercise.Module == ModuleRegex
	return isExCommand || isNonMotionModule
}

// ValidateAnswerDetailed performs comprehensive validation using the simulator
func ValidateAnswerDetailed(exercise *Exercise, answer string) ValidationResult {
	if exercise == nil {
		return ValidationResult{}
	}

	result := ValidationResult{
		OptimalSolution: exercise.Optimal,
		AllSolutions:    exercise.Solutions,
	}

	if answer == "" {
		return result
	}

	answer = strings.TrimSpace(answer)
	if answer == "" {
		return result
	}

	// Check if it's in the predefined solutions (normalize both for comparison)
	for _, sol := range exercise.Solutions {
		if answer == strings.TrimSpace(sol) {
			result.IsInSolutions = true
			break
		}
	}

	// Check if it's optimal (normalize for comparison)
	result.IsOptimal = answer == strings.TrimSpace(exercise.Optimal)

	// Exercises that are not pure motions are not simulated; correctness is
	// decided by the predefined solutions alone.
	if ShouldSkipSimulation(exercise) {
		// For non-motion exercises, correct if it matches any predefined solution
		result.IsCorrect = result.IsInSolutions
		result.TargetPosition = exercise.CursorPos
		result.ActualPosition = exercise.CursorPos
		return result
	}

	// Buffer-verified exercises are judged by the result the answer leaves in
	// the buffer, so an answer that reaches the same result by different keys is
	// correct. The bypass above still comes first: an exercise in a
	// skip-simulation module is decided by its authored solutions even when it
	// opts in, because the engine does not implement that module's commands yet.
	if exercise.BufferVerified {
		return validateViaBuffer(exercise, answer, result)
	}

	// Use the simulator to check the *result* the answer produces, not only the
	// final cursor position. For operator and text-object solutions the cursor
	// does not move, so the resulting selection is what distinguishes a real
	// answer from one the simulator could not parse.
	optimalResult := SimulateMotionsWithSelection(exercise.CursorPos, exercise.Code, exercise.Optimal)
	actualResult := SimulateMotionsWithSelection(exercise.CursorPos, exercise.Code, answer)
	result.TargetPosition = Position{Line: optimalResult.Position.Line, Col: optimalResult.Position.Col}
	result.ActualPosition = Position{Line: actualResult.Position.Line, Col: actualResult.Position.Col}

	switch {
	case result.IsInSolutions:
		// Fast path: predefined solutions are always accepted
		result.IsCorrect = true
	case !IsRecognizedInput(exercise.Code, answer):
		// An answer the simulator cannot fully parse can never be correct
		result.IsCorrect = false
	case optimalResult.Selection.Active:
		// Operator/text-object exercises: position AND selection must match
		result.IsCorrect = sameSimulatedPosition(actualResult.Position, optimalResult.Position) &&
			sameSelection(actualResult.Selection, optimalResult.Selection)
	default:
		// Pure motion exercises: the final cursor position decides
		result.IsCorrect = sameSimulatedPosition(actualResult.Position, optimalResult.Position)
	}

	return result
}

// sameSimulatedPosition reports whether two simulated positions are identical.
func sameSimulatedPosition(a, b SimulatedPosition) bool {
	return a.Line == b.Line && a.Col == b.Col
}

// validateViaBuffer decides a buffer-verified exercise by running the optimal
// and the answer through the mutable editing engine from the exercise's start
// position and comparing the results. It fills the validation result with both
// results so the interface can show what the answer produced and name what
// diverged.
//
// The authored solutions stay first, exactly as on the motion path: a solution
// listed in Solutions is ground truth even where the engine cannot reproduce it
// yet, and an answer the engine cannot fully parse can never be correct on its
// own.
func validateViaBuffer(exercise *Exercise, answer string, result ValidationResult) ValidationResult {
	optimalResult := SimulateEditing(exercise.Code, exercise.CursorPos, exercise.Optimal)
	actualResult := SimulateEditing(exercise.Code, exercise.CursorPos, answer)

	result.BufferVerified = true
	result.TargetPosition = optimalResult.Cursor
	result.ActualPosition = actualResult.Cursor
	result.TargetBuffer = optimalResult.Buffer
	result.ActualBuffer = actualResult.Buffer
	result.TargetMode = optimalResult.Mode
	result.ActualMode = actualResult.Mode

	switch {
	case result.IsInSolutions:
		// Fast path: predefined solutions are always accepted
		result.IsCorrect = true
	case !actualResult.Recognized:
		// An answer the engine cannot fully parse can never be correct
		result.IsCorrect = false
	default:
		result.IsCorrect = bufferResultMatches(optimalResult, actualResult)
	}

	return result
}

// bufferResultMatches reports whether an answer produced the same result as the
// optimal solution: the same buffer text, the same cursor and the same mode.
// The mode clause is real now that insert mode exists: an answer that never
// leaves insert mode diverges from an optimal that does.
func bufferResultMatches(optimal, actual EditingResult) bool {
	return sameLines(optimal.Buffer, actual.Buffer) &&
		optimal.Cursor == actual.Cursor &&
		optimal.Mode == actual.Mode
}

// sameSelection reports whether two selections cover the same range.
func sameSelection(a, b Selection) bool {
	return a.Active == b.Active &&
		a.StartLine == b.StartLine &&
		a.StartCol == b.StartCol &&
		a.EndLine == b.EndLine &&
		a.EndCol == b.EndCol
}

// ValidateAnswer checks if an answer is valid for an exercise.
// It is the boolean view of ValidateAnswerDetailed, so both always agree.
func ValidateAnswer(exercise *Exercise, answer string) bool {
	return ValidateAnswerDetailed(exercise, answer).IsCorrect
}

// IsOptimalAnswer checks if the answer is the optimal solution
func IsOptimalAnswer(exercise *Exercise, answer string) bool {
	if exercise == nil {
		return false
	}

	answer = strings.TrimSpace(answer)
	return answer == exercise.Optimal
}

// IsInSolutions checks if the answer is in the predefined solutions list
func IsInSolutions(exercise *Exercise, answer string) bool {
	if exercise == nil {
		return false
	}

	answer = strings.TrimSpace(answer)
	for _, sol := range exercise.Solutions {
		if answer == strings.TrimSpace(sol) {
			return true
		}
	}
	return false
}

// FormatSolutionsHint returns a formatted string with all solutions
func FormatSolutionsHint(exercise *Exercise) string {
	if exercise == nil || len(exercise.Solutions) == 0 {
		return ""
	}

	if len(exercise.Solutions) == 1 {
		return exercise.Solutions[0]
	}

	// Format as "optimal (or alt1, alt2)"
	var alternatives []string
	for _, sol := range exercise.Solutions {
		if sol != exercise.Optimal {
			alternatives = append(alternatives, sol)
		}
	}

	if len(alternatives) == 0 {
		return exercise.Optimal
	}

	return exercise.Optimal + " (or " + strings.Join(alternatives, ", ") + ")"
}

// CalculatePoints calculates points earned for an exercise. timeSeconds is the
// time taken to answer it, as measured by GameState.ElapsedSeconds, so the
// under-two-second speed bonus below reflects a real measurement rather than a
// fixed placeholder. The bonus depends on the measured time alone: every answer
// is timed now, so requiring the exercise to declare a TimeoutSecs wrongly
// denied it to the boss steps that leave that field unset (thirty of the
// thirty-five; only the Change & Repeat boss's five steps declare one).
// TimeoutSecs decides when a lesson or practice hint appears, never how an
// answer scores.
func CalculatePoints(exercise *Exercise, timeSeconds float64, isOptimal bool, comboMultiplier int) int {
	if exercise == nil {
		return 0
	}

	// Ensure combo is at least 1
	if comboMultiplier < 1 {
		comboMultiplier = 1
	}

	points := float64(exercise.Points)

	// Optimal bonus: 50%
	if isOptimal {
		points *= 1.5
	}

	// Speed bonus: up to 25% for very fast answers (under 2 seconds)
	if timeSeconds < 2.0 {
		points *= 1.25
	}

	// Apply combo multiplier
	points *= float64(comboMultiplier)

	return int(points)
}
