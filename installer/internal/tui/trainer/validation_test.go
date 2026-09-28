package trainer

import (
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
