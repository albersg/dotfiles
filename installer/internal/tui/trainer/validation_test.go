package trainer

import (
	"reflect"
	"strings"
	"testing"
)

// =============================================================================
// ANSWER VALIDATION
// =============================================================================

func TestValidateAnswer_ExactMatch(t *testing.T) {
	exercise := &Exercise{
		ID:        "test_001",
		Solutions: []string{"w"},
	}

	if !ValidateAnswer(exercise, "w") {
		t.Error("Exact match should be valid")
	}
}

func TestValidateAnswer_NoMatch(t *testing.T) {
	// Without proper exercise code/position, any answer that doesn't match
	// predefined solutions AND doesn't simulate correctly will fail
	exercise := &Exercise{
		ID:        "test_001",
		Code:      []string{"const foo = 'bar';"},
		CursorPos: Position{Line: 0, Col: 0},
		Solutions: []string{"w"},
		Optimal:   "w",
	}

	// 'b' from position 0 stays at 0, while 'w' goes to col 6
	// So 'b' should not be valid
	if ValidateAnswer(exercise, "b") {
		t.Error("'b' should be invalid (doesn't reach same position as 'w')")
	}
}

func TestValidateAnswer_MultipleSolutions(t *testing.T) {
	// Code: "const value = test;"
	// w from col 0 -> col 6 (value)
	// W from col 0 -> col 6 (value) - same as w here
	// fe from col 0 -> col 11 (e in value)
	// e from col 0 -> col 4 (t in const)
	// So we need exercises where solutions go to same place and 'e' goes somewhere else
	exercise := &Exercise{
		ID:        "test_001",
		Code:      []string{"const_value_test"},
		CursorPos: Position{Line: 0, Col: 0},
		Solutions: []string{"w", "W"}, // Both go to col 6 (after first _)
		Optimal:   "w",
	}

	// All should be valid
	if !ValidateAnswer(exercise, "w") {
		t.Error("'w' should be valid")
	}
	if !ValidateAnswer(exercise, "W") {
		t.Error("'W' should be valid")
	}

	// 'e' goes to end of first word (col 4), not same as w (col 6)
	// Actually e goes to col 5 (s in const), w goes to col 6 (_)
	// Let's use a clearer case
	exercise2 := &Exercise{
		ID:        "test_002",
		Code:      []string{"go to end here"},
		CursorPos: Position{Line: 0, Col: 0},
		Solutions: []string{"w"}, // w goes to col 3 (to)
		Optimal:   "w",
	}
	// 'b' from col 0 stays at col 0 - different from w's col 3
	if ValidateAnswer(exercise2, "b") {
		t.Error("'b' should be invalid (stays at col 0, not col 3)")
	}
}

func TestValidateAnswer_TrimsWhitespace(t *testing.T) {
	exercise := &Exercise{
		ID:        "test_001",
		Solutions: []string{"ciw"},
	}

	if !ValidateAnswer(exercise, " ciw ") {
		t.Error("Answer with whitespace should be trimmed and validated")
	}
	if !ValidateAnswer(exercise, "\tciw\n") {
		t.Error("Answer with tabs/newlines should be trimmed and validated")
	}
}

func TestValidateAnswer_EmptyAnswer(t *testing.T) {
	exercise := &Exercise{
		ID:        "test_001",
		Solutions: []string{"w"},
	}

	if ValidateAnswer(exercise, "") {
		t.Error("Empty answer should be invalid")
	}
	if ValidateAnswer(exercise, "   ") {
		t.Error("Whitespace-only answer should be invalid")
	}
}

func TestValidateAnswer_NilExercise(t *testing.T) {
	if ValidateAnswer(nil, "w") {
		t.Error("Nil exercise should return false")
	}
}

func TestValidateAnswer_EmptySolutions(t *testing.T) {
	// Exercise with no predefined solutions but with Code/CursorPos/Optimal
	// should still validate via simulator
	exercise := &Exercise{
		ID:        "test_001",
		Code:      []string{"hello world"},
		CursorPos: Position{Line: 0, Col: 0},
		Solutions: []string{},
		Optimal:   "w", // Optimal goes to col 6
	}

	// 'w' should be valid because it matches optimal
	if !ValidateAnswer(exercise, "w") {
		t.Error("Answer matching optimal should be valid even with empty solutions")
	}

	// 'llllll' (6 l's) should also be valid - reaches same position as optimal
	if !ValidateAnswer(exercise, "llllll") {
		t.Error("Creative solution reaching same position should be valid")
	}

	// 'b' stays at col 0, not col 6 - should be invalid
	if ValidateAnswer(exercise, "b") {
		t.Error("'b' should be invalid (doesn't reach target position)")
	}
}

func TestValidateAnswer_CaseSensitive(t *testing.T) {
	// Test that Vim commands are case-sensitive
	// w = word forward, W = WORD forward (includes punctuation)
	// On "hello-world test", w stops at '-', W skips to 'test'

	exercise := &Exercise{
		ID:        "test_001",
		Code:      []string{"hello-world test"},
		CursorPos: Position{Line: 0, Col: 0},
		Solutions: []string{"w"}, // w goes to col 5 (the '-')
		Optimal:   "w",
	}

	// 'w' goes to col 5 (before -)
	if !ValidateAnswer(exercise, "w") {
		t.Error("'w' should be valid")
	}

	// 'W' goes to col 12 (test) - DIFFERENT position!
	if ValidateAnswer(exercise, "W") {
		t.Error("'W' should be invalid (goes to col 12, not col 5)")
	}
}

func TestValidateAnswer_ComplexCommands(t *testing.T) {
	tests := []struct {
		name      string
		code      string
		cursorCol int
		solutions []string
		optimal   string
		input     string
		valid     bool
	}{
		{
			name:      "find char",
			code:      "const = value",
			cursorCol: 0,
			solutions: []string{"f="},
			optimal:   "f=",
			input:     "f=",
			valid:     true,
		},
		{
			name:      "find char wrong",
			code:      "const = value", // No '-' in this string!
			cursorCol: 0,
			solutions: []string{"f="},
			optimal:   "f=",
			input:     "f-", // f- will fail (no '-' to find)
			valid:     false,
		},
		{
			name:      "change inner",
			code:      `say "hello"`,
			cursorCol: 5,
			solutions: []string{"ci\""},
			optimal:   "ci\"",
			input:     "ci\"",
			valid:     true,
		},
		{
			name:      "change around",
			code:      "fn(arg) { code }",
			cursorCol: 10,
			solutions: []string{"ca{"},
			optimal:   "ca{",
			input:     "ca{",
			valid:     true,
		},
		{
			name:      "numbered motion",
			code:      "one two three four",
			cursorCol: 0,
			solutions: []string{"3w"},
			optimal:   "3w",
			input:     "3w",
			valid:     true,
		},
		{
			name:      "numbered motion alt",
			code:      "one two three four",
			cursorCol: 0,
			solutions: []string{"3w", "www"},
			optimal:   "3w",
			input:     "www",
			valid:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exercise := &Exercise{
				ID:        "test",
				Code:      []string{tt.code},
				CursorPos: Position{Line: 0, Col: tt.cursorCol},
				Solutions: tt.solutions,
				Optimal:   tt.optimal,
			}
			result := ValidateAnswer(exercise, tt.input)
			if result != tt.valid {
				t.Errorf("ValidateAnswer(%q) = %v, want %v", tt.input, result, tt.valid)
			}
		})
	}
}

// =============================================================================
// IS OPTIMAL CHECK
// =============================================================================

func TestIsOptimalAnswer_ReturnsTrue(t *testing.T) {
	exercise := &Exercise{
		ID:        "test_001",
		Solutions: []string{"w", "W", "fe"},
		Optimal:   "w",
	}

	if !IsOptimalAnswer(exercise, "w") {
		t.Error("'w' should be optimal")
	}
}

func TestIsOptimalAnswer_ReturnsFalse(t *testing.T) {
	exercise := &Exercise{
		ID:        "test_001",
		Solutions: []string{"w", "W", "fe"},
		Optimal:   "w",
	}

	if IsOptimalAnswer(exercise, "W") {
		t.Error("'W' should not be optimal (valid but not optimal)")
	}
	if IsOptimalAnswer(exercise, "fe") {
		t.Error("'fe' should not be optimal")
	}
}

func TestIsOptimalAnswer_TrimsWhitespace(t *testing.T) {
	exercise := &Exercise{
		ID:      "test_001",
		Optimal: "w",
	}

	if !IsOptimalAnswer(exercise, " w ") {
		t.Error("Whitespace should be trimmed")
	}
}

func TestIsOptimalAnswer_NilExercise(t *testing.T) {
	if IsOptimalAnswer(nil, "w") {
		t.Error("Nil exercise should return false")
	}
}

// =============================================================================
// CALCULATE POINTS
// =============================================================================

func TestCalculatePoints_BasePoints(t *testing.T) {
	exercise := &Exercise{
		ID:     "test",
		Points: 100,
	}

	points := CalculatePoints(exercise, 5.0, false, 1)
	if points != 100 {
		t.Errorf("Base points should be 100, got %d", points)
	}
}

func TestCalculatePoints_OptimalBonus(t *testing.T) {
	exercise := &Exercise{
		ID:     "test",
		Points: 100,
	}

	// Optimal answer gives 50% bonus
	points := CalculatePoints(exercise, 5.0, true, 1)
	if points != 150 {
		t.Errorf("Optimal answer should give 150 points (100 + 50%%), got %d", points)
	}
}

func TestCalculatePoints_SpeedBonus(t *testing.T) {
	exercise := &Exercise{
		ID:          "test",
		Points:      100,
		TimeoutSecs: 30,
	}

	// Very fast answer (under 2 seconds) gets speed bonus
	points := CalculatePoints(exercise, 1.5, false, 1)
	if points <= 100 {
		t.Errorf("Fast answer should get speed bonus, got %d", points)
	}
}

// TestCalculatePoints_SpeedBonusDoesNotRequireTimeout pins that the speed bonus
// follows the measured time alone. Boss steps declare no TimeoutSecs, so a
// precondition on the timeout wrongly denied them the bonus even when the
// player answered instantly: before this change the two answers below scored
// the same.
func TestCalculatePoints_SpeedBonusDoesNotRequireTimeout(t *testing.T) {
	// A boss step: base points, no declared timeout.
	exercise := &Exercise{
		ID:     "horizontal_boss_like",
		Points: 50,
	}

	fast := CalculatePoints(exercise, 1.5, false, 1)
	slow := CalculatePoints(exercise, 2.5, false, 1)

	if fast == slow {
		t.Fatalf("a fast and a slow answer both scored %d: the speed bonus ignored the measured time", fast)
	}
	// 50 base, +25% speed: 62.5 truncated.
	if fast != 62 {
		t.Errorf("fast answer scored %d, want 62 (50 base + 25%% speed)", fast)
	}
	if slow != 50 {
		t.Errorf("slow answer scored %d, want 50 (no speed bonus)", slow)
	}
}

func TestCalculatePoints_ComboMultiplier(t *testing.T) {
	exercise := &Exercise{
		ID:     "test",
		Points: 100,
	}

	// x2 combo
	points := CalculatePoints(exercise, 5.0, false, 2)
	if points != 200 {
		t.Errorf("x2 combo should give 200 points, got %d", points)
	}

	// x4 combo
	points = CalculatePoints(exercise, 5.0, false, 4)
	if points != 400 {
		t.Errorf("x4 combo should give 400 points, got %d", points)
	}
}

func TestCalculatePoints_AllBonusesCombined(t *testing.T) {
	exercise := &Exercise{
		ID:          "test",
		Points:      100,
		TimeoutSecs: 30,
	}

	// Optimal + fast + x2 combo
	points := CalculatePoints(exercise, 1.0, true, 2)
	// Base: 100, Optimal: +50%, Speed: +25% (under 2s), Combo: x2
	// (100 * 1.5 * 1.25) * 2 = 375
	if points < 300 {
		t.Errorf("Combined bonuses should give significant points, got %d", points)
	}
}

func TestCalculatePoints_NilExercise(t *testing.T) {
	points := CalculatePoints(nil, 5.0, false, 1)
	if points != 0 {
		t.Errorf("Nil exercise should give 0 points, got %d", points)
	}
}

func TestCalculatePoints_ZeroCombo(t *testing.T) {
	exercise := &Exercise{
		ID:     "test",
		Points: 100,
	}

	// Combo 0 should be treated as 1
	points := CalculatePoints(exercise, 5.0, false, 0)
	if points != 100 {
		t.Errorf("Zero combo should be treated as 1, got %d", points)
	}
}

// =============================================================================
// SIMULATOR-BASED VALIDATION
// =============================================================================

func TestValidateAnswer_AcceptsCreativeSolutions(t *testing.T) {
	// An exercise where 'w' is the optimal solution to reach col 6
	exercise := &Exercise{
		ID:        "test_001",
		Code:      []string{"const userName = 'value';"},
		CursorPos: Position{Line: 0, Col: 0},
		Solutions: []string{"w"},
		Optimal:   "w",
	}

	// 'w' should work (predefined)
	if !ValidateAnswer(exercise, "w") {
		t.Error("Predefined solution 'w' should be valid")
	}

	// 'llllll' (6 times l) should also reach col 6 - creative solution!
	if !ValidateAnswer(exercise, "llllll") {
		t.Error("Creative solution 'llllll' should be valid (reaches same position)")
	}
}

func TestValidateAnswerDetailed_ReturnsFullInfo(t *testing.T) {
	exercise := &Exercise{
		ID:        "test_001",
		Code:      []string{"const userName = 'value';"},
		CursorPos: Position{Line: 0, Col: 0},
		Solutions: []string{"w", "fe"},
		Optimal:   "w",
	}

	// Test optimal answer
	result := ValidateAnswerDetailed(exercise, "w")
	if !result.IsCorrect {
		t.Error("Should be correct")
	}
	if !result.IsOptimal {
		t.Error("'w' should be optimal")
	}
	if !result.IsInSolutions {
		t.Error("'w' should be in solutions list")
	}

	// Test valid but not optimal
	result = ValidateAnswerDetailed(exercise, "llllll")
	if !result.IsCorrect {
		t.Error("Should be correct (reaches same position)")
	}
	if result.IsOptimal {
		t.Error("'llllll' should not be optimal")
	}
	if result.IsInSolutions {
		t.Error("'llllll' should not be in predefined solutions")
	}

	// Test incorrect answer
	result = ValidateAnswerDetailed(exercise, "b")
	if result.IsCorrect {
		t.Error("'b' should not be correct (doesn't reach target)")
	}
}

func TestIsInSolutions(t *testing.T) {
	exercise := &Exercise{
		ID:        "test",
		Solutions: []string{"w", "W", "fe"},
	}

	if !IsInSolutions(exercise, "w") {
		t.Error("'w' should be in solutions")
	}
	if !IsInSolutions(exercise, "fe") {
		t.Error("'fe' should be in solutions")
	}
	if IsInSolutions(exercise, "llllll") {
		t.Error("'llllll' should not be in solutions")
	}
}

func TestFormatSolutionsHint(t *testing.T) {
	tests := []struct {
		name      string
		solutions []string
		optimal   string
		expected  string
	}{
		{
			name:      "single solution",
			solutions: []string{"w"},
			optimal:   "w",
			expected:  "w",
		},
		{
			name:      "multiple solutions",
			solutions: []string{"w", "fe", "W"},
			optimal:   "w",
			expected:  "w (or fe, W)",
		},
		{
			name:      "optimal is only solution",
			solutions: []string{"$"},
			optimal:   "$",
			expected:  "$",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exercise := &Exercise{
				Solutions: tt.solutions,
				Optimal:   tt.optimal,
			}
			result := FormatSolutionsHint(exercise)
			if result != tt.expected {
				t.Errorf("FormatSolutionsHint() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFormatSolutionsHint_NilExercise(t *testing.T) {
	result := FormatSolutionsHint(nil)
	if result != "" {
		t.Errorf("Nil exercise should return empty string, got %q", result)
	}
}

// =============================================================================
// RESULT-BASED VALIDATION
// =============================================================================

// findLesson locates a shipped lesson by module and ID so these tests exercise
// the real exercise fixtures instead of hand-built structs.
func findLesson(t *testing.T, module ModuleID, id string) *Exercise {
	t.Helper()
	lessons := GetLessons(module)
	for i := range lessons {
		if lessons[i].ID == id {
			return &lessons[i]
		}
	}
	t.Fatalf("lesson %q not found in module %q", id, module)
	return nil
}

// textobjects_001 (optimal "viw") leaves the cursor where it started, so the old
// position-only check accepted any answer the simulator could not parse.
func TestValidateAnswer_RejectsUnrecognizedTextObjectAnswer(t *testing.T) {
	exercise := findLesson(t, ModuleTextObjects, "textobjects_001")

	for _, answer := range []string{"q", "x", "zzz"} {
		if ValidateAnswer(exercise, answer) {
			t.Errorf("%s: ValidateAnswer(%q) = true, want false (unrecognized input)", exercise.ID, answer)
		}
		if result := ValidateAnswerDetailed(exercise, answer); result.IsCorrect {
			t.Errorf("%s: ValidateAnswerDetailed(%q).IsCorrect = true, want false", exercise.ID, answer)
		}
	}
}

// changerepeat_015 (optimal "dd") has the same cursor-does-not-move shape.
func TestValidateAnswer_RejectsUnrecognizedChangeRepeatAnswer(t *testing.T) {
	exercise := findLesson(t, ModuleChangeRepeat, "changerepeat_015")

	for _, answer := range []string{"q", "x", "zzz"} {
		if ValidateAnswer(exercise, answer) {
			t.Errorf("%s: ValidateAnswer(%q) = true, want false (unrecognized input)", exercise.ID, answer)
		}
		if result := ValidateAnswerDetailed(exercise, answer); result.IsCorrect {
			t.Errorf("%s: ValidateAnswerDetailed(%q).IsCorrect = true, want false", exercise.ID, answer)
		}
	}
}

// The positive direction must keep working: the optimal selection answer is
// correct, and so is a creative answer reaching the same result by other means.
func TestValidateAnswer_AcceptsOptimalAndCreativeSelectionAnswers(t *testing.T) {
	optimal := findLesson(t, ModuleTextObjects, "textobjects_001")
	if !ValidateAnswer(optimal, optimal.Optimal) {
		t.Errorf("%s: optimal %q should be accepted", optimal.ID, optimal.Optimal)
	}
	if result := ValidateAnswerDetailed(optimal, optimal.Optimal); !result.IsCorrect || !result.IsOptimal {
		t.Errorf("%s: optimal %q should be correct and optimal, got %+v", optimal.ID, optimal.Optimal, result)
	}

	// changerepeat_001 has optimal "dw" and Solutions {"dw", "de"}.
	// "d1w" is a genuine alternative that yields the same cursor position and
	// selection as "dw" but is not in the predefined list.
	exercise := findLesson(t, ModuleChangeRepeat, "changerepeat_001")
	alternative := "d1w"
	if IsInSolutions(exercise, alternative) {
		t.Fatalf("%s: %q unexpectedly in Solutions %v", exercise.ID, alternative, exercise.Solutions)
	}
	if !ValidateAnswer(exercise, alternative) {
		t.Errorf("%s: creative alternative %q should be accepted (same result as %q)", exercise.ID, alternative, exercise.Optimal)
	}
	if result := ValidateAnswerDetailed(exercise, alternative); !result.IsCorrect {
		t.Errorf("%s: ValidateAnswerDetailed(%q).IsCorrect = false, want true", exercise.ID, alternative)
	}
}

// An answer that produces a different selection than the optimal operator must
// be rejected even when the cursor does not move.
func TestValidateAnswer_RejectsDifferentSelectionFromOptimal(t *testing.T) {
	// changerepeat_001 optimal "dw" selects the word plus trailing space; "dh"
	// selects the single character before the cursor and is not a predefined
	// solution, yet it leaves the cursor where it started.
	exercise := findLesson(t, ModuleChangeRepeat, "changerepeat_001")
	answer := "dh"
	if IsInSolutions(exercise, answer) {
		t.Fatalf("%s: %q unexpectedly in Solutions %v", exercise.ID, answer, exercise.Solutions)
	}
	if ValidateAnswer(exercise, answer) {
		t.Errorf("%s: %q must be rejected (different selection than %q)", exercise.ID, answer, exercise.Optimal)
	}
	if result := ValidateAnswerDetailed(exercise, answer); result.IsCorrect {
		t.Errorf("%s: ValidateAnswerDetailed(%q).IsCorrect = true, want false", exercise.ID, answer)
	}
}

// ShouldSkipSimulation is the single owner of the "do not run the simulator for
// this exercise" predicate. Validation and both render sites used to spell it
// out separately, so a change to one could drift from the others silently.
// These cases pin the behaviour all three call sites relied on.
func TestShouldSkipSimulation(t *testing.T) {
	tests := []struct {
		name     string
		exercise *Exercise
		want     bool
	}{
		{"ex command colon", &Exercise{Module: ModuleHorizontal, Solutions: []string{":%s/foo/bar/g"}}, true},
		{"ex command slash", &Exercise{Module: ModuleHorizontal, Solutions: []string{"/foo"}}, true},
		{"ex command question mark", &Exercise{Module: ModuleHorizontal, Solutions: []string{"?foo"}}, true},
		{"substitution module", &Exercise{Module: ModuleSubstitution, Solutions: []string{"ciw"}}, true},
		{"macros module", &Exercise{Module: ModuleMacros, Solutions: []string{"w"}}, true},
		{"regex module", &Exercise{Module: ModuleRegex, Solutions: []string{"n"}}, true},
		{"horizontal motion", &Exercise{Module: ModuleHorizontal, Solutions: []string{"w"}}, false},
		{"vertical motion", &Exercise{Module: ModuleVertical, Solutions: []string{"j"}}, false},
		{"text objects", &Exercise{Module: ModuleTextObjects, Solutions: []string{"diw"}}, false},
		{"change and repeat", &Exercise{Module: ModuleChangeRepeat, Solutions: []string{"dd"}}, false},
		{"no solutions", &Exercise{Module: ModuleHorizontal}, false},
		{"empty first solution", &Exercise{Module: ModuleHorizontal, Solutions: []string{""}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldSkipSimulation(tt.exercise); got != tt.want {
				t.Errorf("ShouldSkipSimulation(%v) = %v, want %v", tt.exercise, got, tt.want)
			}
		})
	}
}

// When ShouldSkipSimulation says to skip the simulator, validation accepts
// exactly the predefined solutions: an answer outside the list must be rejected
// even if the simulator could parse it. This is the behaviour the validation
// call site relied on before the predicate was extracted.
func TestValidateAnswerDetailed_SkipSimulationUsesSolutionsOnly(t *testing.T) {
	exercise := &Exercise{
		Module:    ModuleSubstitution,
		Code:      []string{"const foo = 'bar';"},
		CursorPos: Position{Line: 0, Col: 0},
		Solutions: []string{":%s/foo/baz/g"},
		Optimal:   ":%s/foo/baz/g",
	}

	if !ShouldSkipSimulation(exercise) {
		t.Fatal("substitution exercise should skip simulation")
	}

	if result := ValidateAnswerDetailed(exercise, ":%s/foo/baz/g"); !result.IsCorrect {
		t.Errorf("predefined solution should be correct when simulation is skipped: %+v", result)
	}

	if result := ValidateAnswerDetailed(exercise, "w"); result.IsCorrect {
		t.Errorf("answer outside the solutions list must be rejected when simulation is skipped: %+v", result)
	}
}

// =============================================================================
// BUFFER-VERIFIED JUDGING
// =============================================================================

// bufferVerifiedExercise is the opt-in fixture the buffer judge tests share: a
// three-line buffer whose mission deletes the middle line. The optimal is
// "2Gdd" (line two of the buffer is line 2 in Vim's 1-based count) and both
// routes leave the cursor on the first character of the remaining "three" line.
func bufferVerifiedExercise() *Exercise {
	return &Exercise{
		ID:             "buffer_test_001",
		Module:         ModuleChangeRepeat,
		Level:          1,
		Type:           ExerciseLesson,
		Code:           []string{"one", "two", "three"},
		CursorPos:      Position{Line: 0, Col: 0},
		Mission:        "Delete the middle line",
		Solutions:      []string{"2Gdd"},
		Optimal:        "2Gdd",
		BufferVerified: true,
	}
}

// An opted-in exercise is judged by the result its answer produces: the optimal
// is correct, and a different answer that reaches the same buffer, cursor and
// mode is correct too. That equivalence is the point of the feature, so it is
// built concretely from two different routes: "2Gdd" (motion, then delete) and
// "jdd" (motion, then delete) on the three-line fixture.
func TestValidateAnswerDetailed_BufferVerifiedJudgesTheResult(t *testing.T) {
	exercise := bufferVerifiedExercise()

	optimal := ValidateAnswerDetailed(exercise, "2Gdd")
	if !optimal.BufferVerified {
		t.Errorf("ValidateAnswerDetailed(%q).BufferVerified = false, want true", exercise.Optimal)
	}
	if !optimal.IsCorrect {
		t.Errorf("ValidateAnswerDetailed(%q).IsCorrect = false, want true", exercise.Optimal)
	}
	if !optimal.IsInSolutions {
		t.Errorf("ValidateAnswerDetailed(%q).IsInSolutions = false, want true", exercise.Optimal)
	}
	if want := []string{"one", "three"}; !reflect.DeepEqual(optimal.TargetBuffer, want) {
		t.Errorf("TargetBuffer = %#v, want %#v", optimal.TargetBuffer, want)
	}
	if want := (Position{Line: 1, Col: 0}); optimal.TargetPosition != want {
		t.Errorf("TargetPosition = %+v, want %+v", optimal.TargetPosition, want)
	}
	if optimal.TargetMode != ModeNormal {
		t.Errorf("TargetMode = %v, want %v", optimal.TargetMode, ModeNormal)
	}

	// "jdd" is not an authored solution: it earns correctness from the result
	// it produces, not from the keys it used.
	answer := ValidateAnswerDetailed(exercise, "jdd")
	if answer.IsInSolutions {
		t.Fatalf("fixture is wrong: %q is in Solutions %v", "jdd", exercise.Solutions)
	}
	if !answer.BufferVerified {
		t.Error("ValidateAnswerDetailed(\"jdd\").BufferVerified = false, want true")
	}
	if !answer.IsCorrect {
		t.Errorf("ValidateAnswerDetailed(\"jdd\") rejected an answer that reaches the same result: %+v", answer)
	}
	if want := optimal.TargetBuffer; !reflect.DeepEqual(answer.ActualBuffer, want) {
		t.Errorf("ActualBuffer = %#v, want %#v", answer.ActualBuffer, want)
	}
	if answer.ActualPosition != optimal.TargetPosition {
		t.Errorf("ActualPosition = %+v, want %+v", answer.ActualPosition, optimal.TargetPosition)
	}
	if answer.ActualMode != optimal.TargetMode {
		t.Errorf("ActualMode = %v, want %v", answer.ActualMode, optimal.TargetMode)
	}
	if got := answer.MismatchSummary(); got != "" {
		t.Errorf("MismatchSummary() = %q for a correct answer, want empty", got)
	}
}

// A rejected answer must name what differed, and the result must carry the
// buffer the answer produced so the interface can show it. "jx" deletes the
// first rune of the middle line: the buffer differs while the cursor stays on
// the row and column the optimal reaches, so only the buffer may be named.
func TestValidateAnswerDetailed_BufferVerifiedRejectsDifferentBuffer(t *testing.T) {
	exercise := bufferVerifiedExercise()

	result := ValidateAnswerDetailed(exercise, "jx")
	if result.IsCorrect {
		t.Fatalf("ValidateAnswerDetailed(\"jx\").IsCorrect = true, want false: %+v", result)
	}
	if !result.BufferVerified {
		t.Error("BufferVerified = false, want true")
	}
	if want := []string{"one", "three"}; !reflect.DeepEqual(result.TargetBuffer, want) {
		t.Errorf("TargetBuffer = %#v, want %#v", result.TargetBuffer, want)
	}
	if want := []string{"one", "wo", "three"}; !reflect.DeepEqual(result.ActualBuffer, want) {
		t.Errorf("ActualBuffer = %#v, want %#v", result.ActualBuffer, want)
	}
	if result.ActualPosition != result.TargetPosition {
		t.Fatalf("fixture is wrong: cursor %+v differs from %+v, so more than the buffer diverged",
			result.ActualPosition, result.TargetPosition)
	}
	got := result.MismatchSummary()
	if !strings.Contains(got, "buffer") {
		t.Errorf("MismatchSummary() = %q, want it to name the buffer", got)
	}
	if strings.Contains(got, "cursor") || strings.Contains(got, "mode") {
		t.Errorf("MismatchSummary() = %q, want only the buffer named", got)
	}
}

// A right buffer with the wrong cursor is rejected and named. "2Gddkk" deletes
// the same line as the optimal and then walks the cursor up, so the buffer
// matches and only the cursor differs.
func TestValidateAnswerDetailed_BufferVerifiedRejectsDifferentCursor(t *testing.T) {
	exercise := bufferVerifiedExercise()

	result := ValidateAnswerDetailed(exercise, "2Gddkk")
	if result.IsCorrect {
		t.Fatalf("ValidateAnswerDetailed(\"2Gddkk\").IsCorrect = true, want false: %+v", result)
	}
	if !reflect.DeepEqual(result.ActualBuffer, result.TargetBuffer) {
		t.Fatalf("fixture is wrong: buffers differ (%#v vs %#v)", result.ActualBuffer, result.TargetBuffer)
	}
	if result.ActualPosition == result.TargetPosition {
		t.Fatalf("fixture is wrong: cursor %+v did not diverge", result.ActualPosition)
	}
	got := result.MismatchSummary()
	if !strings.Contains(got, "cursor") {
		t.Errorf("MismatchSummary() = %q, want it to name the cursor", got)
	}
}

// The mode is part of the compared result, not an assumption: the same buffer
// and cursor in a different mode is a different result. Today's engine returns
// ModeNormal for every command it understands, so this pins the comparison rule
// directly; the first command that leaves another mode (insert mode, E4) is
// then rejected by the judge without a change here.
func TestBufferResultMatches_RejectsADifferentMode(t *testing.T) {
	expected := EditingResult{
		Buffer:     []string{"one"},
		Cursor:     Position{Line: 0, Col: 0},
		Mode:       ModeNormal,
		Recognized: true,
	}

	if !bufferResultMatches(expected, expected) {
		t.Error("bufferResultMatches rejected an identical result")
	}

	differentMode := expected
	differentMode.Mode = ModeInsert
	if bufferResultMatches(expected, differentMode) {
		t.Error("bufferResultMatches accepted the same buffer in a different mode")
	}

	differentBuffer := expected
	differentBuffer.Buffer = []string{"two"}
	if bufferResultMatches(expected, differentBuffer) {
		t.Error("bufferResultMatches accepted a different buffer")
	}

	differentCursor := expected
	differentCursor.Cursor = Position{Line: 1, Col: 0}
	if bufferResultMatches(expected, differentCursor) {
		t.Error("bufferResultMatches accepted the same buffer at a different cursor")
	}
}

// The mismatch report names the mode when the mode is the divergence, and stays
// silent for results the buffer judge did not decide.
func TestValidationResult_MismatchSummaryNamesTheDivergence(t *testing.T) {
	tests := []struct {
		name   string
		result ValidationResult
		want   []string
		absent []string
	}{
		{
			name: "a correct result has nothing to report",
			result: ValidationResult{
				IsCorrect: true, BufferVerified: true,
				TargetBuffer: []string{"one"}, ActualBuffer: []string{"one"},
				TargetMode: ModeNormal, ActualMode: ModeInsert,
			},
		},
		{
			name: "a result the buffer judge did not decide has nothing to report",
			result: ValidationResult{
				TargetPosition: Position{Line: 0, Col: 0}, ActualPosition: Position{Line: 3, Col: 2},
			},
			absent: []string{"buffer", "cursor", "mode"},
		},
		{
			name: "the buffer and the cursor can diverge together",
			result: ValidationResult{
				BufferVerified: true,
				TargetBuffer:   []string{"one", "three"}, ActualBuffer: []string{"one", "two", "three"},
				TargetPosition: Position{Line: 1, Col: 0}, ActualPosition: Position{Line: 2, Col: 0},
				TargetMode: ModeNormal, ActualMode: ModeNormal,
			},
			want:   []string{"buffer", "cursor"},
			absent: []string{"mode"},
		},
		{
			name: "the mode alone can diverge",
			result: ValidationResult{
				BufferVerified: true,
				TargetBuffer:   []string{"one"}, ActualBuffer: []string{"one"},
				TargetPosition: Position{Line: 0, Col: 0}, ActualPosition: Position{Line: 0, Col: 0},
				TargetMode: ModeNormal, ActualMode: ModeVisual,
			},
			want:   []string{"mode"},
			absent: []string{"buffer", "cursor"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.result.MismatchSummary()
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Errorf("MismatchSummary() = %q, want it to name %q", got, want)
				}
			}
			for _, absent := range tt.absent {
				if strings.Contains(got, absent) {
					t.Errorf("MismatchSummary() = %q, want %q left unnamed", got, absent)
				}
			}
		})
	}
}

// The exact-match fast path stays first for opted-in exercises: an authored
// solution is ground truth even when the engine cannot reproduce it yet.
// "daw" is an operator plus text object, which the buffer engine does not
// implement (E5 owns registers and operator spans), so the engine cannot turn
// it into the optimal's buffer; the same answer is rejected for an exercise
// that does not author it.
func TestValidateAnswerDetailed_BufferVerifiedExactMatchWins(t *testing.T) {
	exercise := &Exercise{
		ID:             "buffer_test_002",
		Module:         ModuleChangeRepeat,
		Level:          2,
		Type:           ExerciseLesson,
		Code:           []string{"one", "two", "three"},
		CursorPos:      Position{Line: 0, Col: 0},
		Mission:        "Delete the middle line",
		Solutions:      []string{"2Gdd", "daw"},
		Optimal:        "2Gdd",
		BufferVerified: true,
	}

	result := ValidateAnswerDetailed(exercise, "daw")
	if !result.IsInSolutions {
		t.Fatalf("fixture is wrong: %q is not in Solutions %v", "daw", exercise.Solutions)
	}
	if !result.IsCorrect {
		t.Errorf("ValidateAnswerDetailed(\"daw\") rejected an authored solution: %+v", result)
	}

	// The same answer is not authored by the shared fixture, so the result judge
	// decides it -- and rejects it, because the buffer does not match.
	if ValidateAnswer(bufferVerifiedExercise(), "daw") {
		t.Error("ValidateAnswer(\"daw\") = true for an exercise that does not author it, want false")
	}
}

// The judge's documented order is exact match, then the skip-simulation bypass,
// then the buffer judge. An exercise in a skip-simulation module is therefore
// decided by its authored solutions even when it opts in, because the buffer
// engine does not implement that module's commands yet; the opt-in only takes
// effect on exercises that are simulated at all. ValidationResult.BufferVerified
// records which judge actually ran, so this precedence is observable rather than
// implied.
func TestValidateAnswerDetailed_BufferOptInDoesNotOverrideSkipSimulation(t *testing.T) {
	exercise := &Exercise{
		ID:             "buffer_test_003",
		Module:         ModuleSubstitution,
		Code:           []string{"  value = 1"},
		CursorPos:      Position{Line: 0, Col: 0},
		Solutions:      []string{"S"},
		Optimal:        "S",
		BufferVerified: true,
	}

	if !ShouldSkipSimulation(exercise) {
		t.Fatal("a substitution exercise must skip simulation")
	}

	authored := ValidateAnswerDetailed(exercise, "S")
	if !authored.IsCorrect {
		t.Errorf("authored solution rejected: %+v", authored)
	}
	if authored.BufferVerified {
		t.Error("the buffer judge ran for a skip-simulation exercise")
	}
	if authored.TargetBuffer != nil || authored.ActualBuffer != nil {
		t.Errorf("a skip-simulation result carries buffers: %+v", authored)
	}

	if result := ValidateAnswerDetailed(exercise, "ciw"); result.IsCorrect {
		t.Errorf("an answer outside the solutions list must still be rejected: %+v", result)
	}
}

// ValidateAnswer is the boolean view of ValidateAnswerDetailed, so the buffer
// path must agree through both entry points.
func TestValidateAnswer_BufferVerifiedAgreesWithDetailed(t *testing.T) {
	exercise := bufferVerifiedExercise()

	for _, answer := range []string{"2Gdd", "jdd", "jx", "2Gddkk", "q"} {
		detailed := ValidateAnswerDetailed(exercise, answer)
		if got := ValidateAnswer(exercise, answer); got != detailed.IsCorrect {
			t.Errorf("ValidateAnswer(%q) = %v, ValidateAnswerDetailed(%q).IsCorrect = %v",
				answer, got, answer, detailed.IsCorrect)
		}
	}
}
