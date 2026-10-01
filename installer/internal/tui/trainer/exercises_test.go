package trainer

import (
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// =============================================================================
// GET LESSONS
// =============================================================================

func TestGetLessons_Horizontal_ReturnsExercises(t *testing.T) {
	lessons := GetLessons(ModuleHorizontal)

	if len(lessons) == 0 {
		t.Error("GetLessons should return exercises for Horizontal module")
	}
}

func TestGetLessons_Horizontal_HasMinimum15(t *testing.T) {
	lessons := GetLessons(ModuleHorizontal)

	if len(lessons) < 15 {
		t.Errorf("Horizontal module should have at least 15 lessons, got %d", len(lessons))
	}
}

func TestGetLessons_Horizontal_AllHaveRequiredFields(t *testing.T) {
	lessons := GetLessons(ModuleHorizontal)

	for i, ex := range lessons {
		if ex.ID == "" {
			t.Errorf("Lesson %d: ID is empty", i)
		}
		if ex.Module != ModuleHorizontal {
			t.Errorf("Lesson %d: Module should be Horizontal, got %s", i, ex.Module)
		}
		if ex.Type != ExerciseLesson {
			t.Errorf("Lesson %d: Type should be Lesson, got %s", i, ex.Type)
		}
		if len(ex.Code) == 0 {
			t.Errorf("Lesson %d: Code is empty", i)
		}
		if ex.Mission == "" {
			t.Errorf("Lesson %d: Mission is empty", i)
		}
		if len(ex.Solutions) == 0 {
			t.Errorf("Lesson %d: Solutions is empty", i)
		}
		if ex.Optimal == "" {
			t.Errorf("Lesson %d: Optimal is empty", i)
		}
		// Hint is deliberately NOT a required field: the mission/hint rule in
		// exercises.go lets an exercise whose hint can only repeat its mission
		// drop the hint. What a hint that IS present must still do -- add a
		// count, sibling or consequence its mission does not give -- is guarded
		// by TestShippedHintsAddWhatTheirMissionDoesNot.
		if ex.Explanation == "" {
			t.Errorf("Lesson %d: Explanation is empty", i)
		}
		if ex.Points <= 0 {
			t.Errorf("Lesson %d: Points should be positive, got %d", i, ex.Points)
		}
	}
}

func TestGetLessons_Horizontal_OptimalIsInSolutions(t *testing.T) {
	lessons := GetLessons(ModuleHorizontal)

	for i, ex := range lessons {
		found := false
		for _, sol := range ex.Solutions {
			if sol == ex.Optimal {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Lesson %d (%s): Optimal %q not in Solutions %v", i, ex.ID, ex.Optimal, ex.Solutions)
		}
	}
}

func TestGetLessons_Horizontal_UniqueIDs(t *testing.T) {
	lessons := GetLessons(ModuleHorizontal)
	seen := make(map[string]bool)

	for _, ex := range lessons {
		if seen[ex.ID] {
			t.Errorf("Duplicate lesson ID: %s", ex.ID)
		}
		seen[ex.ID] = true
	}
}

func TestGetLessons_Horizontal_ProgressiveLevels(t *testing.T) {
	lessons := GetLessons(ModuleHorizontal)

	// First lesson should be level 1
	if lessons[0].Level != 1 {
		t.Errorf("First lesson should be level 1, got %d", lessons[0].Level)
	}

	// Levels should be non-decreasing (can stay same or increase)
	prevLevel := 0
	for i, ex := range lessons {
		if ex.Level < prevLevel {
			t.Errorf("Lesson %d: Level %d is less than previous %d", i, ex.Level, prevLevel)
		}
		prevLevel = ex.Level
	}
}

func TestGetLessons_Horizontal_CoversBasicMotions(t *testing.T) {
	lessons := GetLessons(ModuleHorizontal)

	// Collect all solutions taught
	allSolutions := make(map[string]bool)
	for _, ex := range lessons {
		for _, sol := range ex.Solutions {
			allSolutions[sol] = true
		}
	}

	// Should cover these basic motions
	requiredMotions := []string{"w", "W", "e", "b", "$", "^", "0"}
	for _, motion := range requiredMotions {
		if !allSolutions[motion] {
			t.Errorf("Horizontal lessons should cover motion %q", motion)
		}
	}
}

func TestGetLessons_Horizontal_CoversFindMotions(t *testing.T) {
	lessons := GetLessons(ModuleHorizontal)

	// At least one lesson should use f{char} pattern
	hasFind := false
	for _, ex := range lessons {
		for _, sol := range ex.Solutions {
			if len(sol) >= 2 && sol[0] == 'f' {
				hasFind = true
				break
			}
		}
	}
	if !hasFind {
		t.Error("Horizontal lessons should include f{char} motion")
	}
}

func TestGetLessons_UnknownModule_ReturnsEmpty(t *testing.T) {
	lessons := GetLessons(ModuleID("unknown"))

	if len(lessons) != 0 {
		t.Errorf("Unknown module should return empty slice, got %d lessons", len(lessons))
	}
}

// =============================================================================
// GET BOSS
// =============================================================================

func TestGetBoss_Horizontal_ReturnsBoss(t *testing.T) {
	boss := GetBoss(ModuleHorizontal)

	if boss == nil {
		t.Fatal("GetBoss should return boss for Horizontal module")
	}
}

func TestGetBoss_Horizontal_HasCorrectName(t *testing.T) {
	boss := GetBoss(ModuleHorizontal)

	if boss.Name != "The Line Walker" {
		t.Errorf("Horizontal boss should be 'The Line Walker', got %q", boss.Name)
	}
}

func TestGetBoss_Horizontal_Has3Lives(t *testing.T) {
	boss := GetBoss(ModuleHorizontal)

	if boss.Lives != 3 {
		t.Errorf("Boss should have 3 lives, got %d", boss.Lives)
	}
}

func TestGetBoss_Horizontal_Has5Steps(t *testing.T) {
	boss := GetBoss(ModuleHorizontal)

	if len(boss.Steps) != 5 {
		t.Errorf("Boss should have 5 steps, got %d", len(boss.Steps))
	}
}

func TestGetBoss_Horizontal_StepsHaveTimeLimits(t *testing.T) {
	boss := GetBoss(ModuleHorizontal)

	for i, step := range boss.Steps {
		if step.TimeLimit <= 0 {
			t.Errorf("Boss step %d: TimeLimit should be positive, got %d", i, step.TimeLimit)
		}
	}
}

func TestGetBoss_Horizontal_StepsHaveExercises(t *testing.T) {
	boss := GetBoss(ModuleHorizontal)

	for i, step := range boss.Steps {
		if step.Exercise.ID == "" {
			t.Errorf("Boss step %d: Exercise ID is empty", i)
		}
		if step.Exercise.Mission == "" {
			t.Errorf("Boss step %d: Exercise Mission is empty", i)
		}
		if len(step.Exercise.Solutions) == 0 {
			t.Errorf("Boss step %d: Exercise Solutions is empty", i)
		}
	}
}

func TestGetBoss_UnknownModule_ReturnsNil(t *testing.T) {
	boss := GetBoss(ModuleID("unknown"))

	if boss != nil {
		t.Error("Unknown module should return nil boss")
	}
}

// =============================================================================
// EXERCISE QUALITY CHECKS
// =============================================================================

func TestHorizontalExercises_MissionsAreInEnglish(t *testing.T) {
	lessons := GetLessons(ModuleHorizontal)

	// Check that missions use English (contain words like "Move", "to", "the", "using", etc.)
	englishIndicators := []string{"Move", "to", "the", "using", "Reach", "Go"}
	hasEnglish := false

	for _, ex := range lessons {
		for _, indicator := range englishIndicators {
			if containsString(ex.Mission, indicator) {
				hasEnglish = true
				break
			}
		}
		if hasEnglish {
			break
		}
	}

	if !hasEnglish {
		t.Error("Lessons should have missions in English")
	}
}

func TestHorizontalExercises_HintsAreHelpful(t *testing.T) {
	lessons := GetLessons(ModuleHorizontal)

	for i, ex := range lessons {
		// A hint is optional: the mission/hint rule in exercises.go lets an
		// exercise whose hint can only repeat its mission drop the hint, and the
		// shipped corpus may drop one without failing this test. What is still
		// pinned is that a hint which IS present says more than the answer alone,
		// because a hint that only echoed the command would cost the player a
		// keypress and teach nothing.
		if ex.Hint == "" {
			continue
		}
		if len(ex.Hint) <= len(ex.Optimal) {
			t.Errorf("Lesson %d: Hint should be more helpful than just the answer", i)
		}
	}
}

func TestHorizontalExercises_ExplanationsAreEducational(t *testing.T) {
	lessons := GetLessons(ModuleHorizontal)

	for i, ex := range lessons {
		// Explanation should be substantial (at least 20 characters)
		if len(ex.Explanation) < 20 {
			t.Errorf("Lesson %d: Explanation should be more educational (got %d chars)", i, len(ex.Explanation))
		}
	}
}

// Helper function
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (s[:len(substr)] == substr || containsString(s[1:], substr)))
}

// =============================================================================
// VERTICAL MODULE TESTS
// =============================================================================

func TestGetLessons_Vertical_ReturnsExercises(t *testing.T) {
	lessons := GetLessons(ModuleVertical)

	if len(lessons) == 0 {
		t.Error("GetLessons should return exercises for Vertical module")
	}
}

func TestGetLessons_Vertical_HasMinimum15(t *testing.T) {
	lessons := GetLessons(ModuleVertical)

	if len(lessons) < 15 {
		t.Errorf("Vertical module should have at least 15 lessons, got %d", len(lessons))
	}
}

func TestGetLessons_Vertical_AllHaveRequiredFields(t *testing.T) {
	lessons := GetLessons(ModuleVertical)

	for i, ex := range lessons {
		if ex.ID == "" {
			t.Errorf("Lesson %d: ID is empty", i)
		}
		if ex.Module != ModuleVertical {
			t.Errorf("Lesson %d: Module should be Vertical, got %s", i, ex.Module)
		}
		if ex.Type != ExerciseLesson {
			t.Errorf("Lesson %d: Type should be Lesson, got %s", i, ex.Type)
		}
		if len(ex.Code) == 0 {
			t.Errorf("Lesson %d: Code is empty", i)
		}
		if ex.Mission == "" {
			t.Errorf("Lesson %d: Mission is empty", i)
		}
		if len(ex.Solutions) == 0 {
			t.Errorf("Lesson %d: Solutions is empty", i)
		}
		if ex.Optimal == "" {
			t.Errorf("Lesson %d: Optimal is empty", i)
		}
		// Hint is deliberately NOT a required field here: see
		// TestShippedHintsAddWhatTheirMissionDoesNot for the guard over present
		// hints and exercises.go for the mission/hint rule that allows a hint to
		// be dropped.
		if ex.Points <= 0 {
			t.Errorf("Lesson %d: Points should be positive, got %d", i, ex.Points)
		}
	}
}

func TestGetLessons_Vertical_OptimalIsInSolutions(t *testing.T) {
	lessons := GetLessons(ModuleVertical)

	for i, ex := range lessons {
		found := false
		for _, sol := range ex.Solutions {
			if sol == ex.Optimal {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Lesson %d (%s): Optimal %q not in Solutions %v", i, ex.ID, ex.Optimal, ex.Solutions)
		}
	}
}

func TestGetLessons_Vertical_UniqueIDs(t *testing.T) {
	lessons := GetLessons(ModuleVertical)
	seen := make(map[string]bool)

	for _, ex := range lessons {
		if seen[ex.ID] {
			t.Errorf("Duplicate lesson ID: %s", ex.ID)
		}
		seen[ex.ID] = true
	}
}

func TestGetLessons_Vertical_CoversBasicMotions(t *testing.T) {
	lessons := GetLessons(ModuleVertical)

	allSolutions := make(map[string]bool)
	for _, ex := range lessons {
		for _, sol := range ex.Solutions {
			allSolutions[sol] = true
		}
	}

	// Should cover these basic vertical motions
	requiredMotions := []string{"j", "k", "gg", "G", "{", "}"}
	for _, motion := range requiredMotions {
		if !allSolutions[motion] {
			t.Errorf("Vertical lessons should cover motion %q", motion)
		}
	}
}

func TestGetLessons_Vertical_HasMultiLineCode(t *testing.T) {
	lessons := GetLessons(ModuleVertical)

	// All vertical lessons should have multi-line code
	for i, ex := range lessons {
		if len(ex.Code) < 2 {
			t.Errorf("Lesson %d: Vertical exercises should have multi-line code, got %d lines", i, len(ex.Code))
		}
	}
}

func TestGetBoss_Vertical_ReturnsBoss(t *testing.T) {
	boss := GetBoss(ModuleVertical)

	if boss == nil {
		t.Error("GetBoss should return a boss for Vertical module")
	}
}

func TestGetBoss_Vertical_HasCorrectName(t *testing.T) {
	boss := GetBoss(ModuleVertical)

	if boss.Name != "The Code Climber" {
		t.Errorf("Vertical boss name should be 'The Code Climber', got %q", boss.Name)
	}
}

func TestGetBoss_Vertical_Has3Lives(t *testing.T) {
	boss := GetBoss(ModuleVertical)

	if boss.Lives != 3 {
		t.Errorf("Boss should have 3 lives, got %d", boss.Lives)
	}
}

func TestGetBoss_Vertical_Has5Steps(t *testing.T) {
	boss := GetBoss(ModuleVertical)

	if len(boss.Steps) != 5 {
		t.Errorf("Boss should have 5 steps, got %d", len(boss.Steps))
	}
}

func TestGetBoss_Vertical_StepsHaveExercises(t *testing.T) {
	boss := GetBoss(ModuleVertical)

	for i, step := range boss.Steps {
		if step.Exercise.ID == "" {
			t.Errorf("Boss step %d: Exercise ID is empty", i)
		}
		if step.Exercise.Mission == "" {
			t.Errorf("Boss step %d: Exercise Mission is empty", i)
		}
		if len(step.Exercise.Solutions) == 0 {
			t.Errorf("Boss step %d: Exercise Solutions is empty", i)
		}
	}
}

func TestVerticalExercises_SolutionsReachCorrectPosition(t *testing.T) {
	lessons := GetLessons(ModuleVertical)

	for i, ex := range lessons {
		// Simulate optimal solution
		targetPos := SimulateMotions(ex.CursorPos, ex.Code, ex.Optimal)

		// Check all solutions reach the same position
		for _, sol := range ex.Solutions {
			actualPos := SimulateMotions(ex.CursorPos, ex.Code, sol)
			if actualPos.Line != targetPos.Line || actualPos.Col != targetPos.Col {
				t.Errorf("Lesson %d (%s): Solution %q reaches (%d,%d) but optimal %q reaches (%d,%d)",
					i, ex.ID, sol, actualPos.Line, actualPos.Col, ex.Optimal, targetPos.Line, targetPos.Col)
			}
		}
	}
}

// =============================================================================
// TEXT OBJECTS MODULE TESTS
// =============================================================================

func TestGetLessons_TextObjects_ReturnsExercises(t *testing.T) {
	lessons := GetLessons(ModuleTextObjects)
	if len(lessons) == 0 {
		t.Error("GetLessons should return exercises for TextObjects module")
	}
}

func TestGetLessons_TextObjects_HasMinimum15(t *testing.T) {
	lessons := GetLessons(ModuleTextObjects)
	if len(lessons) < 15 {
		t.Errorf("TextObjects module should have at least 15 lessons, got %d", len(lessons))
	}
}

func TestGetLessons_TextObjects_AllHaveRequiredFields(t *testing.T) {
	lessons := GetLessons(ModuleTextObjects)
	for i, ex := range lessons {
		if ex.ID == "" {
			t.Errorf("Lesson %d has empty ID", i)
		}
		if ex.Module != ModuleTextObjects {
			t.Errorf("Lesson %d has wrong module: %s", i, ex.Module)
		}
		if len(ex.Solutions) == 0 {
			t.Errorf("Lesson %d (%s) has no solutions", i, ex.ID)
		}
		if ex.Optimal == "" {
			t.Errorf("Lesson %d (%s) has no optimal solution", i, ex.ID)
		}
	}
}

func TestGetBoss_TextObjects_ReturnsBoss(t *testing.T) {
	boss := GetBoss(ModuleTextObjects)
	if boss == nil {
		t.Error("GetBoss should return a boss for TextObjects module")
	}
}

func TestGetBoss_TextObjects_HasCorrectName(t *testing.T) {
	boss := GetBoss(ModuleTextObjects)
	if boss.Name != "The Text Surgeon" {
		t.Errorf("Boss name should be 'The Text Surgeon', got %s", boss.Name)
	}
}

// =============================================================================
// CHANGE/REPEAT MODULE TESTS
// =============================================================================

func TestGetLessons_ChangeRepeat_ReturnsExercises(t *testing.T) {
	lessons := GetLessons(ModuleChangeRepeat)
	if len(lessons) == 0 {
		t.Error("GetLessons should return exercises for ChangeRepeat module")
	}
}

func TestGetLessons_ChangeRepeat_HasMinimum15(t *testing.T) {
	lessons := GetLessons(ModuleChangeRepeat)
	if len(lessons) < 15 {
		t.Errorf("ChangeRepeat module should have at least 15 lessons, got %d", len(lessons))
	}
}

func TestGetLessons_ChangeRepeat_AllHaveRequiredFields(t *testing.T) {
	lessons := GetLessons(ModuleChangeRepeat)
	for i, ex := range lessons {
		if ex.ID == "" {
			t.Errorf("Lesson %d has empty ID", i)
		}
		if ex.Module != ModuleChangeRepeat {
			t.Errorf("Lesson %d has wrong module: %s", i, ex.Module)
		}
		if len(ex.Solutions) == 0 {
			t.Errorf("Lesson %d (%s) has no solutions", i, ex.ID)
		}
	}
}

func TestGetBoss_ChangeRepeat_ReturnsBoss(t *testing.T) {
	boss := GetBoss(ModuleChangeRepeat)
	if boss == nil {
		t.Error("GetBoss should return a boss for ChangeRepeat module")
	}
}

func TestGetBoss_ChangeRepeat_HasCorrectName(t *testing.T) {
	boss := GetBoss(ModuleChangeRepeat)
	if boss.Name != "The Change Master" {
		t.Errorf("Boss name should be 'The Change Master', got %s", boss.Name)
	}
}

// =============================================================================
// SUBSTITUTION MODULE TESTS
// =============================================================================

func TestGetLessons_Substitution_ReturnsExercises(t *testing.T) {
	lessons := GetLessons(ModuleSubstitution)
	if len(lessons) == 0 {
		t.Error("GetLessons should return exercises for Substitution module")
	}
}

func TestGetLessons_Substitution_HasMinimum15(t *testing.T) {
	lessons := GetLessons(ModuleSubstitution)
	if len(lessons) < 15 {
		t.Errorf("Substitution module should have at least 15 lessons, got %d", len(lessons))
	}
}

func TestGetLessons_Substitution_AllHaveRequiredFields(t *testing.T) {
	lessons := GetLessons(ModuleSubstitution)
	for i, ex := range lessons {
		if ex.ID == "" {
			t.Errorf("Lesson %d has empty ID", i)
		}
		if ex.Module != ModuleSubstitution {
			t.Errorf("Lesson %d has wrong module: %s", i, ex.Module)
		}
		if len(ex.Solutions) == 0 {
			t.Errorf("Lesson %d (%s) has no solutions", i, ex.ID)
		}
	}
}

func TestGetBoss_Substitution_ReturnsBoss(t *testing.T) {
	boss := GetBoss(ModuleSubstitution)
	if boss == nil {
		t.Error("GetBoss should return a boss for Substitution module")
	}
}

func TestGetBoss_Substitution_HasCorrectName(t *testing.T) {
	boss := GetBoss(ModuleSubstitution)
	if boss.Name != "The Transformer" {
		t.Errorf("Boss name should be 'The Transformer', got %s", boss.Name)
	}
}

// =============================================================================
// REGEX MODULE TESTS
// =============================================================================

func TestGetLessons_Regex_ReturnsExercises(t *testing.T) {
	lessons := GetLessons(ModuleRegex)
	if len(lessons) == 0 {
		t.Error("GetLessons should return exercises for Regex module")
	}
}

func TestGetLessons_Regex_HasMinimum15(t *testing.T) {
	lessons := GetLessons(ModuleRegex)
	if len(lessons) < 15 {
		t.Errorf("Regex module should have at least 15 lessons, got %d", len(lessons))
	}
}

func TestGetLessons_Regex_AllHaveRequiredFields(t *testing.T) {
	lessons := GetLessons(ModuleRegex)
	for i, ex := range lessons {
		if ex.ID == "" {
			t.Errorf("Lesson %d has empty ID", i)
		}
		if ex.Module != ModuleRegex {
			t.Errorf("Lesson %d has wrong module: %s", i, ex.Module)
		}
		if len(ex.Solutions) == 0 {
			t.Errorf("Lesson %d (%s) has no solutions", i, ex.ID)
		}
	}
}

func TestGetLessons_Regex_OptimalIsInSolutions(t *testing.T) {
	lessons := GetLessons(ModuleRegex)
	for i, ex := range lessons {
		found := false
		for _, sol := range ex.Solutions {
			if sol == ex.Optimal {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Lesson %d (%s): Optimal %q not in Solutions %v", i, ex.ID, ex.Optimal, ex.Solutions)
		}
	}
}

func TestGetLessons_Regex_UniqueIDs(t *testing.T) {
	lessons := GetLessons(ModuleRegex)
	seen := make(map[string]bool)
	for _, ex := range lessons {
		if seen[ex.ID] {
			t.Errorf("Duplicate lesson ID: %s", ex.ID)
		}
		seen[ex.ID] = true
	}
}

func TestGetLessons_Regex_CoversBasicSearchCommands(t *testing.T) {
	lessons := GetLessons(ModuleRegex)
	allSolutions := make(map[string]bool)
	for _, ex := range lessons {
		for _, sol := range ex.Solutions {
			allSolutions[sol] = true
		}
	}

	// Should cover basic search commands
	requiredCommands := []string{"n", "N", "*", "#"}
	for _, cmd := range requiredCommands {
		if !allSolutions[cmd] {
			t.Errorf("Regex lessons should cover command %q", cmd)
		}
	}
}

func TestGetLessons_Regex_CoversVimgrepAndQuickfix(t *testing.T) {
	lessons := GetLessons(ModuleRegex)

	hasVimgrep := false
	hasCopen := false
	hasCnext := false
	hasCprev := false
	hasGrep := false
	hasCclose := false

	for _, ex := range lessons {
		for _, sol := range ex.Solutions {
			if len(sol) >= 8 && sol[:8] == ":vimgrep" {
				hasVimgrep = true
			}
			if len(sol) >= 4 && sol[:4] == ":vim" {
				hasVimgrep = true
			}
			if sol == ":copen" || sol == ":copen\n" || sol == ":cope" || sol == ":cw" || sol == ":cw\n" || sol == ":cwindow" {
				hasCopen = true
			}
			if sol == ":cnext" || sol == ":cnext\n" || sol == ":cn" {
				hasCnext = true
			}
			if sol == ":cprev" || sol == ":cprev\n" || sol == ":cp" {
				hasCprev = true
			}
			if len(sol) >= 5 && sol[:5] == ":grep" {
				hasGrep = true
			}
			if sol == ":cclose" || sol == ":cclose\n" || sol == ":ccl" {
				hasCclose = true
			}
		}
	}

	if !hasVimgrep {
		t.Error("Regex lessons should cover :vimgrep command")
	}
	if !hasCopen {
		t.Error("Regex lessons should cover :copen/:cw command")
	}
	if !hasCnext {
		t.Error("Regex lessons should cover :cnext/:cn command")
	}
	if !hasCprev {
		t.Error("Regex lessons should cover :cprev/:cp command")
	}
	if !hasGrep {
		t.Error("Regex lessons should cover :grep command")
	}
	if !hasCclose {
		t.Error("Regex lessons should cover :cclose/:ccl command")
	}
}

func TestGetLessons_Regex_HasCorrectCount(t *testing.T) {
	lessons := GetLessons(ModuleRegex)
	// After adding vimgrep/quickfix exercises, should have 24 lessons
	if len(lessons) < 24 {
		t.Errorf("Regex module should have at least 24 lessons after adding vimgrep/quickfix, got %d", len(lessons))
	}
}

func TestGetBoss_Regex_ReturnsBoss(t *testing.T) {
	boss := GetBoss(ModuleRegex)
	if boss == nil {
		t.Error("GetBoss should return a boss for Regex module")
	}
}

func TestGetBoss_Regex_HasCorrectName(t *testing.T) {
	boss := GetBoss(ModuleRegex)
	if boss.Name != "The Pattern Hunter" {
		t.Errorf("Boss name should be 'The Pattern Hunter', got %s", boss.Name)
	}
}

func TestGetBoss_Regex_Has5Steps(t *testing.T) {
	boss := GetBoss(ModuleRegex)
	if len(boss.Steps) != 5 {
		t.Errorf("Boss should have 5 steps, got %d", len(boss.Steps))
	}
}

func TestGetBoss_Regex_StepsHaveTimeLimits(t *testing.T) {
	boss := GetBoss(ModuleRegex)
	for i, step := range boss.Steps {
		if step.TimeLimit <= 0 {
			t.Errorf("Boss step %d: TimeLimit should be positive, got %d", i, step.TimeLimit)
		}
	}
}

// =============================================================================
// MACROS MODULE TESTS
// =============================================================================

func TestGetLessons_Macros_ReturnsExercises(t *testing.T) {
	lessons := GetLessons(ModuleMacros)
	if len(lessons) == 0 {
		t.Error("GetLessons should return exercises for Macros module")
	}
}

func TestGetLessons_Macros_HasMinimum15(t *testing.T) {
	lessons := GetLessons(ModuleMacros)
	if len(lessons) < 15 {
		t.Errorf("Macros module should have at least 15 lessons, got %d", len(lessons))
	}
}

func TestGetLessons_Macros_AllHaveRequiredFields(t *testing.T) {
	lessons := GetLessons(ModuleMacros)
	for i, ex := range lessons {
		if ex.ID == "" {
			t.Errorf("Lesson %d has empty ID", i)
		}
		if ex.Module != ModuleMacros {
			t.Errorf("Lesson %d has wrong module: %s", i, ex.Module)
		}
		if len(ex.Solutions) == 0 {
			t.Errorf("Lesson %d (%s) has no solutions", i, ex.ID)
		}
	}
}

func TestGetLessons_Macros_OptimalIsInSolutions(t *testing.T) {
	lessons := GetLessons(ModuleMacros)
	for i, ex := range lessons {
		found := false
		for _, sol := range ex.Solutions {
			if sol == ex.Optimal {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Lesson %d (%s): Optimal %q not in Solutions %v", i, ex.ID, ex.Optimal, ex.Solutions)
		}
	}
}

func TestGetLessons_Macros_UniqueIDs(t *testing.T) {
	lessons := GetLessons(ModuleMacros)
	seen := make(map[string]bool)
	for _, ex := range lessons {
		if seen[ex.ID] {
			t.Errorf("Duplicate lesson ID: %s", ex.ID)
		}
		seen[ex.ID] = true
	}
}

func TestGetLessons_Macros_CoversBasicMacroCommands(t *testing.T) {
	lessons := GetLessons(ModuleMacros)
	allSolutions := make(map[string]bool)
	for _, ex := range lessons {
		for _, sol := range ex.Solutions {
			allSolutions[sol] = true
		}
	}

	// Should cover basic macro commands
	requiredCommands := []string{"qa", "q", "@a", "@@"}
	for _, cmd := range requiredCommands {
		if !allSolutions[cmd] {
			t.Errorf("Macros lessons should cover command %q", cmd)
		}
	}
}

func TestGetLessons_Macros_CoversNormalAndGlobalCommands(t *testing.T) {
	lessons := GetLessons(ModuleMacros)

	hasNormal := false
	hasNormalWithRange := false
	hasGlobal := false
	hasGlobalNormal := false
	hasInverseGlobal := false

	for _, ex := range lessons {
		for _, sol := range ex.Solutions {
			// Check for :normal command
			if len(sol) >= 7 && sol[:7] == ":normal" {
				hasNormal = true
			}
			if len(sol) >= 5 && sol[:5] == ":norm" {
				hasNormal = true
			}
			// Check for range + normal (like :% normal or :2,4 normal)
			if len(sol) >= 2 && sol[0] == ':' && (sol[1] == '%' || (sol[1] >= '0' && sol[1] <= '9')) {
				hasNormalWithRange = true
			}
			// Check for :g/pattern/
			if len(sol) >= 3 && sol[:3] == ":g/" {
				hasGlobal = true
			}
			// Check for :g/pattern/normal
			if len(sol) >= 3 && sol[:3] == ":g/" && (containsSubstring(sol, "normal") || containsSubstring(sol, "norm")) {
				hasGlobalNormal = true
			}
			// Check for :v/ (inverse global)
			if len(sol) >= 3 && sol[:3] == ":v/" {
				hasInverseGlobal = true
			}
		}
	}

	if !hasNormal {
		t.Error("Macros lessons should cover :normal command")
	}
	if !hasNormalWithRange {
		t.Error("Macros lessons should cover :normal with range (like :% normal)")
	}
	if !hasGlobal {
		t.Error("Macros lessons should cover :g/pattern/ global command")
	}
	if !hasGlobalNormal {
		t.Error("Macros lessons should cover :g/pattern/normal @a combination")
	}
	if !hasInverseGlobal {
		t.Error("Macros lessons should cover :v/pattern/ inverse global command")
	}
}

func TestGetLessons_Macros_HasCorrectCount(t *testing.T) {
	lessons := GetLessons(ModuleMacros)
	// After adding :normal and :g commands, should have 24 lessons
	if len(lessons) < 24 {
		t.Errorf("Macros module should have at least 24 lessons after adding :normal/:g, got %d", len(lessons))
	}
}

func TestGetBoss_Macros_ReturnsBoss(t *testing.T) {
	boss := GetBoss(ModuleMacros)
	if boss == nil {
		t.Error("GetBoss should return a boss for Macros module")
	}
}

func TestGetBoss_Macros_HasCorrectName(t *testing.T) {
	boss := GetBoss(ModuleMacros)
	if boss.Name != "The Automation Wizard" {
		t.Errorf("Boss name should be 'The Automation Wizard', got %s", boss.Name)
	}
}

func TestGetBoss_Macros_Has5Steps(t *testing.T) {
	boss := GetBoss(ModuleMacros)
	if len(boss.Steps) != 5 {
		t.Errorf("Boss should have 5 steps, got %d", len(boss.Steps))
	}
}

func TestGetBoss_Macros_StepsHaveTimeLimits(t *testing.T) {
	boss := GetBoss(ModuleMacros)
	for i, step := range boss.Steps {
		if step.TimeLimit <= 0 {
			t.Errorf("Boss step %d: TimeLimit should be positive, got %d", i, step.TimeLimit)
		}
	}
}

// =============================================================================
// EDITING & UNDO MODULE
// =============================================================================

func TestGetLessons_Editing_ReturnsExercises(t *testing.T) {
	lessons := GetLessons(ModuleEditing)
	if len(lessons) == 0 {
		t.Fatal("GetLessons should return exercises for the Editing module")
	}
}

func TestGetLessons_Editing_HasMinimum15(t *testing.T) {
	lessons := GetLessons(ModuleEditing)
	if len(lessons) < 15 {
		t.Errorf("Editing module should have at least 15 lessons, got %d", len(lessons))
	}
}

func TestGetLessons_Editing_AllHaveRequiredFields(t *testing.T) {
	lessons := GetLessons(ModuleEditing)
	for i, ex := range lessons {
		if ex.ID == "" {
			t.Errorf("Lesson %d: ID is empty", i)
		}
		if ex.Module != ModuleEditing {
			t.Errorf("Lesson %d: Module should be Editing, got %s", i, ex.Module)
		}
		if ex.Type != ExerciseLesson {
			t.Errorf("Lesson %d: Type should be Lesson, got %s", i, ex.Type)
		}
		if len(ex.Code) == 0 {
			t.Errorf("Lesson %d: Code is empty", i)
		}
		if ex.Mission == "" {
			t.Errorf("Lesson %d: Mission is empty", i)
		}
		if len(ex.Solutions) == 0 {
			t.Errorf("Lesson %d: Solutions is empty", i)
		}
		if ex.Optimal == "" {
			t.Errorf("Lesson %d: Optimal is empty", i)
		}
		// Hint is deliberately NOT a required field here: see
		// TestShippedHintsAddWhatTheirMissionDoesNot for the guard over present
		// hints and exercises.go for the mission/hint rule that allows a hint to
		// be dropped.
		if ex.Explanation == "" {
			t.Errorf("Lesson %d: Explanation is empty", i)
		}
		if ex.TimeoutSecs <= 0 {
			t.Errorf("Lesson %d: TimeoutSecs should be positive, got %d", i, ex.TimeoutSecs)
		}
		if ex.Points <= 0 {
			t.Errorf("Lesson %d: Points should be positive, got %d", i, ex.Points)
		}
	}
}

func TestGetLessons_Editing_OptimalIsInSolutions(t *testing.T) {
	for _, ex := range GetLessons(ModuleEditing) {
		found := false
		for _, sol := range ex.Solutions {
			if sol == ex.Optimal {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Lesson %s: Optimal %q not in Solutions %v", ex.ID, ex.Optimal, ex.Solutions)
		}
	}
}

func TestGetLessons_Editing_UniqueIDs(t *testing.T) {
	seen := make(map[string]bool)
	for _, ex := range GetLessons(ModuleEditing) {
		if seen[ex.ID] {
			t.Errorf("Duplicate lesson ID: %s", ex.ID)
		}
		seen[ex.ID] = true
	}
}

func TestGetLessons_Editing_CoversTheCommandsItTeaches(t *testing.T) {
	lessons := GetLessons(ModuleEditing)
	allSolutions := make(map[string]bool)
	for _, ex := range lessons {
		for _, sol := range ex.Solutions {
			allSolutions[sol] = true
		}
	}

	requiredCommands := []string{
		"u",         // undo
		"\x12",      // redo
		"xu",        // undo the last change
		"ddu\x12",   // undo then redo
		"i",         // insert entries, one per lesson
		"a",         //
		"I",         //
		"A!<Esc>",   //
		"o  return", //
		"Ox := 1",   //
		"dd",        // line delete
		"2dd",       // counted line delete
		"yyjp",      // yank and put
		"yyjdd\"0p", // register 0
		">>",        // indent
		"2<<",       // counted dedent
		"6x",        // counted character delete
		"D",         // delete to line end
		"dw",        // operator + delegated motion
		"%x",        // bracket motion + change
		"Gmaggdd`a", // marks
	}

	for _, cmd := range requiredCommands {
		matched := false
		for sol := range allSolutions {
			if containsSubstring(sol, cmd) {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("Editing lessons should cover command %q", cmd)
		}
	}
}

func TestGetBoss_Editing_ReturnsBoss(t *testing.T) {
	boss := GetBoss(ModuleEditing)
	if boss == nil {
		t.Fatal("GetBoss should return a boss for the Editing module")
	}
}

func TestGetBoss_Editing_HasCorrectName(t *testing.T) {
	boss := GetBoss(ModuleEditing)
	if boss == nil || boss.Name != "The Historian" {
		t.Errorf("Editing boss name should be 'The Historian', got %v", boss)
	}
}

func TestGetBoss_Editing_Has5Steps(t *testing.T) {
	boss := GetBoss(ModuleEditing)
	if boss == nil || len(boss.Steps) != 5 {
		t.Errorf("Editing boss should have 5 steps, got %v", boss)
	}
}

func TestGetBoss_Editing_StepsHaveTimeLimits(t *testing.T) {
	boss := GetBoss(ModuleEditing)
	if boss == nil {
		t.Fatal("no editing boss")
	}
	for i, step := range boss.Steps {
		if step.TimeLimit <= 0 {
			t.Errorf("Boss step %d: TimeLimit should be positive, got %d", i, step.TimeLimit)
		}
	}
}

// =============================================================================
// EDITING & UNDO MODULE - BUFFER JUDGE INVARIANTS
// =============================================================================

// editingExercises returns every exercise of the Editing & Undo module: its
// lessons and its boss steps, in order. The invariants below enumerate the real
// corpus so a lesson added without an optimal answer that validates cannot ship
// unnoticed: a module whose own optimal does not validate is a module nobody
// can finish.
func editingExercises() []Exercise {
	all := GetLessons(ModuleEditing)
	if boss := GetBoss(ModuleEditing); boss != nil {
		for _, step := range boss.Steps {
			all = append(all, step.Exercise)
		}
	}
	return all
}

// TestEditingModule_EveryOptimalValidates is the module's completion invariant:
// answering each lesson and boss step with its own Optimal must be judged
// correct, and the editing engine must recognize the optimal so the buffer
// judge can actually reproduce the result the mission states. The Solutions
// fast path alone would hide a broken optimal, which is why Recognized is
// checked separately.
func TestEditingModule_EveryOptimalValidates(t *testing.T) {
	exercises := editingExercises()
	if len(exercises) != 28 {
		t.Fatalf("enumerated %d editing exercises, want 28 (23 lessons + 5 boss steps)", len(exercises))
	}

	for _, ex := range exercises {
		t.Run(ex.ID, func(t *testing.T) {
			if !ex.BufferVerified {
				t.Error("exercise does not opt into the buffer judge; the editing module is judged by the buffer it produces")
			}

			result := ValidateAnswerDetailed(&ex, ex.Optimal)
			if !result.IsCorrect {
				t.Errorf("optimal %q is rejected; the module cannot be finished", ex.Optimal)
			}
			if !result.BufferVerified {
				t.Error("validation did not run the buffer judge")
			}

			engine := SimulateEditing(ex.Code, ex.CursorPos, ex.Optimal)
			if !engine.Recognized {
				t.Errorf("the editing engine does not recognize the optimal %q; the mission states a result it cannot reproduce", ex.Optimal)
			}
		})
	}
}

// TestEditingModule_PlausibleWrongAnswersAreRejected pins that the buffer judge
// actually discriminates: each entry below is a believable attempt that does
// not reach the mission's result, so it must be judged incorrect and named as
// diverging. The first entry is the one the task requires; the others protect
// the whole arc (undo, insert sessions, registers, marks) from drifting into
// exercises that accept any answer.
func TestEditingModule_PlausibleWrongAnswersAreRejected(t *testing.T) {
	byID := make(map[string]Exercise)
	for _, ex := range editingExercises() {
		byID[ex.ID] = ex
	}

	tests := []struct {
		id     string
		answer string
	}{
		{"editing_001", "dd"},
		{"editing_016", "xxu"},
		{"editing_017", "A!<Esc>ojunk<Esc>"},
		{"editing_020", "yyjddp"},
		{"editing_023", "Gmaggdd"},
		{"editing_boss_3", "yyjd"},
		{"editing_boss_5", "Gmaggdd`a"},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			ex, ok := byID[tt.id]
			if !ok {
				t.Fatalf("no exercise %s", tt.id)
			}
			if IsInSolutions(&ex, tt.answer) {
				t.Fatalf("%q is an authored solution of %s; pick a wrong answer", tt.answer, tt.id)
			}

			result := ValidateAnswerDetailed(&ex, tt.answer)
			if result.IsCorrect {
				t.Errorf("%s accepted the wrong answer %q", tt.id, tt.answer)
			}
			if !result.BufferVerified {
				t.Error("wrong answer was not judged by the buffer judge")
			}
			if result.MismatchSummary() == "" {
				t.Error("wrong answer has no mismatch summary to report to the player")
			}
		})
	}
}

// =============================================================================
// REGISTERS & INDENTATION MODULE
// =============================================================================

func TestGetLessons_Registers_ReturnsExercises(t *testing.T) {
	lessons := GetLessons(ModuleRegisters)
	if len(lessons) == 0 {
		t.Fatal("GetLessons should return exercises for the Registers module")
	}
}

func TestGetLessons_Registers_HasMinimum15(t *testing.T) {
	lessons := GetLessons(ModuleRegisters)
	if len(lessons) < 15 {
		t.Errorf("Registers module should have at least 15 lessons, got %d", len(lessons))
	}
}

func TestGetLessons_Registers_AllHaveRequiredFields(t *testing.T) {
	lessons := GetLessons(ModuleRegisters)
	for i, ex := range lessons {
		if ex.ID == "" {
			t.Errorf("Lesson %d: ID is empty", i)
		}
		if ex.Module != ModuleRegisters {
			t.Errorf("Lesson %d: Module should be Registers, got %s", i, ex.Module)
		}
		if ex.Type != ExerciseLesson {
			t.Errorf("Lesson %d: Type should be Lesson, got %s", i, ex.Type)
		}
		if len(ex.Code) == 0 {
			t.Errorf("Lesson %d: Code is empty", i)
		}
		if ex.Mission == "" {
			t.Errorf("Lesson %d: Mission is empty", i)
		}
		if len(ex.Solutions) == 0 {
			t.Errorf("Lesson %d: Solutions is empty", i)
		}
		if ex.Optimal == "" {
			t.Errorf("Lesson %d: Optimal is empty", i)
		}
		// Hint is deliberately NOT a required field here: see
		// TestShippedHintsAddWhatTheirMissionDoesNot for the guard over present
		// hints and exercises.go for the mission/hint rule that allows a hint to
		// be dropped.
		if ex.Explanation == "" {
			t.Errorf("Lesson %d: Explanation is empty", i)
		}
		if ex.TimeoutSecs <= 0 {
			t.Errorf("Lesson %d: TimeoutSecs should be positive, got %d", i, ex.TimeoutSecs)
		}
		if ex.Points <= 0 {
			t.Errorf("Lesson %d: Points should be positive, got %d", i, ex.Points)
		}
	}
}

func TestGetLessons_Registers_OptimalIsInSolutions(t *testing.T) {
	for _, ex := range GetLessons(ModuleRegisters) {
		found := false
		for _, sol := range ex.Solutions {
			if sol == ex.Optimal {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Lesson %s: Optimal %q not in Solutions %v", ex.ID, ex.Optimal, ex.Solutions)
		}
	}
}

func TestGetLessons_Registers_UniqueIDs(t *testing.T) {
	seen := make(map[string]bool)
	for _, ex := range GetLessons(ModuleRegisters) {
		if seen[ex.ID] {
			t.Errorf("Duplicate lesson ID: %s", ex.ID)
		}
		seen[ex.ID] = true
	}
}

func TestGetLessons_Registers_CoversTheCommandsItTeaches(t *testing.T) {
	lessons := GetLessons(ModuleRegisters)
	allSolutions := make(map[string]bool)
	for _, ex := range lessons {
		for _, sol := range ex.Solutions {
			allSolutions[sol] = true
		}
	}

	requiredCommands := []string{
		"yyp",            // yank a line, put it below
		"yyP",            // yank a line, put it above
		"yiwP",           // yank a word, put it before the cursor
		"yiwp",           // yank a word, put it after the cursor
		"\"ayyj\"ap",     // named register round trip
		"yyjdd\"0p",      // register 0 survives a delete
		"ddp",            // delete and put to move a line down
		"ddP",            // delete and put to move a line up
		">>",             // indent the current line
		"<<",             // dedent the current line
		"2>>",            // counted shift right
		"2<<",            // counted shift left
		"3>>",            // counted shift over a range
		"yjp",            // yank a linewise range
		"y$P",            // yank a character-wise range
		"yypA done<Esc>", // yank and insert together
		"\"ayyjyy\"ap",   // a named register survives another yank
		"yyjddp",         // the unnamed register follows the last delete
	}

	for _, cmd := range requiredCommands {
		matched := false
		for sol := range allSolutions {
			if containsSubstring(sol, cmd) {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("Registers lessons should cover command %q", cmd)
		}
	}
}

func TestGetBoss_Registers_ReturnsBoss(t *testing.T) {
	boss := GetBoss(ModuleRegisters)
	if boss == nil {
		t.Fatal("GetBoss should return a boss for the Registers module")
	}
}

func TestGetBoss_Registers_HasCorrectName(t *testing.T) {
	boss := GetBoss(ModuleRegisters)
	if boss == nil || boss.Name != "The Archivist" {
		t.Errorf("Registers boss name should be 'The Archivist', got %v", boss)
	}
}

func TestGetBoss_Registers_Has5Steps(t *testing.T) {
	boss := GetBoss(ModuleRegisters)
	if boss == nil || len(boss.Steps) != 5 {
		t.Errorf("Registers boss should have 5 steps, got %v", boss)
	}
}

func TestGetBoss_Registers_StepsHaveTimeLimits(t *testing.T) {
	boss := GetBoss(ModuleRegisters)
	if boss == nil {
		t.Fatal("no registers boss")
	}
	for i, step := range boss.Steps {
		if step.TimeLimit <= 0 {
			t.Errorf("Boss step %d: TimeLimit should be positive, got %d", i, step.TimeLimit)
		}
	}
}

// =============================================================================
// REGISTERS & INDENTATION MODULE - BUFFER JUDGE INVARIANTS
// =============================================================================

// registersExercises returns every exercise of the Registers & Indentation
// module: its lessons and its boss steps, in order. The invariants below
// enumerate the real corpus so a lesson added without an optimal answer that
// validates cannot ship unnoticed: a module whose own optimal does not validate
// is a module nobody can finish.
func registersExercises() []Exercise {
	all := GetLessons(ModuleRegisters)
	if boss := GetBoss(ModuleRegisters); boss != nil {
		for _, step := range boss.Steps {
			all = append(all, step.Exercise)
		}
	}
	return all
}

// TestRegistersModule_EveryOptimalValidates is the module's completion
// invariant: answering each lesson and boss step with its own Optimal must be
// judged correct, and the editing engine must recognize the optimal so the
// buffer judge can actually reproduce the result the mission states. The
// Solutions fast path alone would hide a broken optimal, which is why Recognized
// is checked separately.
func TestRegistersModule_EveryOptimalValidates(t *testing.T) {
	exercises := registersExercises()
	if len(exercises) != 25 {
		t.Fatalf("enumerated %d registers exercises, want 25 (20 lessons + 5 boss steps)", len(exercises))
	}

	for _, ex := range exercises {
		t.Run(ex.ID, func(t *testing.T) {
			if !ex.BufferVerified {
				t.Error("exercise does not opt into the buffer judge; the registers module is judged by the buffer it produces")
			}

			result := ValidateAnswerDetailed(&ex, ex.Optimal)
			if !result.IsCorrect {
				t.Errorf("optimal %q is rejected; the module cannot be finished", ex.Optimal)
			}
			if !result.BufferVerified {
				t.Error("validation did not run the buffer judge")
			}

			engine := SimulateEditing(ex.Code, ex.CursorPos, ex.Optimal)
			if !engine.Recognized {
				t.Errorf("the editing engine does not recognize the optimal %q; the mission states a result it cannot reproduce", ex.Optimal)
			}
		})
	}
}

// TestRegistersModule_PlausibleWrongAnswersAreRejected pins that the buffer
// judge actually discriminates: each entry below is a believable attempt that
// does not reach the mission's result, so it must be judged incorrect and named
// as diverging. The register lessons carry the weight: using the unnamed
// register where the answer needs 0, forgetting the put or the delete, and
// shifting one line where the mission asks for a range all leave a different
// buffer, and the judge names which part diverged.
func TestRegistersModule_PlausibleWrongAnswersAreRejected(t *testing.T) {
	byID := make(map[string]Exercise)
	for _, ex := range registersExercises() {
		byID[ex.ID] = ex
	}

	tests := []struct {
		id     string
		answer string
	}{
		{"registers_001", "yy"},
		{"registers_005", "dd"},
		{"registers_007", "\"ayy\"ap"},
		{"registers_008", "yyjddp"},
		{"registers_010", "<<"},
		{"registers_012", ">>"},
		{"registers_020", "yypA!<Esc>"},
		{"registers_boss_4", "ddp"},
		{"registers_boss_5", "\"ayyjyyjdd\"ap"},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			ex, ok := byID[tt.id]
			if !ok {
				t.Fatalf("no exercise %s", tt.id)
			}
			if IsInSolutions(&ex, tt.answer) {
				t.Fatalf("%q is an authored solution of %s; pick a wrong answer", tt.answer, tt.id)
			}

			result := ValidateAnswerDetailed(&ex, tt.answer)
			if result.IsCorrect {
				t.Errorf("%s accepted the wrong answer %q", tt.id, tt.answer)
			}
			if !result.BufferVerified {
				t.Error("wrong answer was not judged by the buffer judge")
			}
			if result.MismatchSummary() == "" {
				t.Error("wrong answer has no mismatch summary to report to the player")
			}
		})
	}
}

// Helper function to check if string contains substring
func containsSubstring(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// =============================================================================
// ALL MODULES - CROSS-MODULE TESTS
// =============================================================================

func allModuleIDs() []ModuleID {
	modules := GetAllModules()
	ids := make([]ModuleID, 0, len(modules))
	for _, module := range modules {
		ids = append(ids, module.ID)
	}
	return ids
}

func TestAllModules_HaveUniqueExerciseIDs(t *testing.T) {
	modules := allModuleIDs()

	allIDs := make(map[string]bool)
	for _, mod := range modules {
		lessons := GetLessons(mod)
		for _, ex := range lessons {
			if allIDs[ex.ID] {
				t.Errorf("Duplicate exercise ID found: %s", ex.ID)
			}
			allIDs[ex.ID] = true
		}
	}
}

func TestAllModules_BossesHave5Steps(t *testing.T) {
	modules := allModuleIDs()

	for _, mod := range modules {
		boss := GetBoss(mod)
		if boss == nil {
			t.Errorf("Module %s has no boss", mod)
			continue
		}
		if len(boss.Steps) != 5 {
			t.Errorf("Module %s boss should have 5 steps, got %d", mod, len(boss.Steps))
		}
	}
}

func TestAllModules_BossesHave3Lives(t *testing.T) {
	modules := allModuleIDs()

	for _, mod := range modules {
		boss := GetBoss(mod)
		if boss == nil {
			continue
		}
		if boss.Lives != 3 {
			t.Errorf("Module %s boss should have 3 lives, got %d", mod, boss.Lives)
		}
	}
}

func TestAllModules_HaveAGradedCourseOfLessons(t *testing.T) {
	modules := GetAllModules()
	if len(modules) == 0 {
		t.Fatal("GetAllModules returned no modules")
	}

	for _, module := range modules {
		lessons := GetLessons(module.ID)
		lessonCount := len(lessons)
		if boss := GetBoss(module.ID); boss == nil {
			t.Errorf("Module %s has %d lessons and no boss", module.ID, lessonCount)
		}
		if lessonCount < 15 {
			t.Errorf("Module %s has %d lessons; want at least 15", module.ID, lessonCount)
		}

		for i, lesson := range lessons {
			if lesson.ID == "" {
				t.Errorf("Module %s has %d lessons; lesson %d has an empty ID", module.ID, lessonCount, i)
			}
			if lesson.Mission == "" {
				t.Errorf("Module %s has %d lessons; lesson %d has an empty Mission", module.ID, lessonCount, i)
			}
			if lesson.Optimal == "" {
				t.Errorf("Module %s has %d lessons; lesson %d has an empty Optimal", module.ID, lessonCount, i)
			}
			if len(lesson.Solutions) == 0 {
				t.Errorf("Module %s has %d lessons; lesson %d has no Solutions", module.ID, lessonCount, i)
			}
		}
	}
}

// =============================================================================
// CONTENT HONESTY - THE EXERCISES THAT PROMISED WHAT THE JUDGE COULD NOT CHECK
// =============================================================================

// honestyBackedExerciseIDs are the exercises the content-honesty pass opted into
// the buffer judge after giving each a mission that states a result the buffer
// judge can check. Each one promised a capability in prose (a put, a register
// round trip) that the motion/selection judge could not verify because it only
// saw a cursor, a selection or an exact string. The list is explicit so a new
// opt-in arrives with a matching decision and tests instead of drifting in
// silently, and TestShippedExercises_OptInOnlyWhereTheMissionStatesABufferResult
// in types_test.go reads it as the policy for the shipped corpus.
var honestyBackedExerciseIDs = map[string]bool{
	"textobjects_020":  true,
	"textobjects_021":  true,
	"changerepeat_004": true,
	"macros_011":       true,
	"macros_012":       true,
	"macros_013":       true,
	"macros_014":       true,
	"macros_015":       true,
}

// honestyRewordedExerciseIDs are the exercises whose promised capability is an
// operation the buffer engine does not model (the c/cc and S change commands, or
// any ex command). Backing them by result is impossible today, so their mission,
// hint and explanation were rewritten to claim only the keystrokes the judge
// actually checks. The map holds, for each, a fragment of the old prose that
// claimed the unverifiable result; the test below asserts the claim is gone.
//
// The macros_020 to macros_024 entries are the global-command family the first
// pass missed: they described the result of :g/… and :v/… commands, which the
// engine cannot run, in words the old marker list ("register", "paste", "put",
// ":normal") did not contain. They now say the trainer checks the keystrokes,
// and the broadened detector below catches that class written the same way again.
var honestyRewordedExerciseIDs = map[string]string{
	"changerepeat_009": "Change the entire comment line",
	"substitution_007": "for replacement",
	"substitution_008": "preserves indent",
	"substitution_018": "using :%s",
	"macros_016":       "comment prefix to current line",
	"macros_017":       "Add semicolon to end of current line",
	"macros_018":       "Comment all lines with",
	"macros_019":       "Add semicolons to lines 2-4",
	"macros_020":       "Comment all lines containing",
	"macros_021":       "Delete all TODO lines",
	"macros_022":       "Run macro 'a' on all error lines",
	"macros_023":       "Apply macro 'b' to all DEBUG lines",
	"macros_024":       "Delete all lines NOT containing",
	"macros_boss_5":    "Paste from the yank register (\"0)",
}

// findExerciseByID returns the shipped exercise with the given ID, whether it is
// a lesson or a boss step, so the honesty tests below run against the real
// definitions rather than against a copy.
func findExerciseByID(t *testing.T, id string) Exercise {
	t.Helper()
	for _, module := range moduleUnlockOrder {
		for _, ex := range GetLessons(module) {
			if ex.ID == id {
				return ex
			}
		}
		if boss := GetBoss(module); boss != nil {
			for _, step := range boss.Steps {
				if step.Exercise.ID == id {
					return step.Exercise
				}
			}
		}
	}
	t.Fatalf("no shipped exercise with ID %q", id)
	return Exercise{}
}

// TestBackedExercises_EveryOptimalProducesTheResultItsMissionStates is the
// completion invariant for every exercise the honesty pass opted in: answering
// with the Optimal must be judged correct, the engine must recognize it, and the
// buffer and cursor it leaves must be exactly the result the mission states. The
// Solutions fast path alone would hide a broken optimal, which is why the engine
// result is checked directly as well.
func TestBackedExercises_EveryOptimalProducesTheResultItsMissionStates(t *testing.T) {
	tests := []struct {
		id     string
		buffer []string
		cursor Position
	}{
		{"textobjects_020", []string{"const importantValue = 42;", "const copy = importantValue;"}, Position{Line: 1, Col: 26}},
		{"textobjects_021", []string{`const url = "https://api.example.com";`, `const copy = "https://api.example.com";`}, Position{Line: 1, Col: 36}},
		{"changerepeat_004", []string{"const valid = true;", "const alsoValid = false;"}, Position{Line: 1, Col: 0}},
		{"macros_011", []string{"const importantValue = 'remember_this';", "importantValue", "// Need this value below"}, Position{Line: 1, Col: 13}},
		{"macros_012", []string{"myFunction", "const result = ();", "myFunction"}, Position{Line: 2, Col: 0}},
		{"macros_013", []string{"const API_KEY = 'abc123xyz789';", "abc123xyz789", "// Copy the key to share externally"}, Position{Line: 1, Col: 11}},
		{"macros_014", []string{"external_data", "const value = 'external_data';"}, Position{Line: 1, Col: 27}},
		{"macros_015", []string{"keep_this", "const target = 'keep_this';"}, Position{Line: 1, Col: 24}},
	}

	if len(tests) != len(honestyBackedExerciseIDs) {
		t.Fatalf("the table covers %d exercises but honestyBackedExerciseIDs names %d", len(tests), len(honestyBackedExerciseIDs))
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			ex := findExerciseByID(t, tt.id)
			if !ex.BufferVerified {
				t.Fatalf("%s does not opt into the buffer judge", tt.id)
			}

			result := ValidateAnswerDetailed(&ex, ex.Optimal)
			if !result.IsCorrect {
				t.Errorf("optimal %q is rejected; the exercise cannot be finished", ex.Optimal)
			}
			if !result.BufferVerified {
				t.Error("validation did not run the buffer judge")
			}

			engine := SimulateEditing(ex.Code, ex.CursorPos, ex.Optimal)
			if !engine.Recognized {
				t.Errorf("the engine does not recognize the optimal %q; the mission states a result it cannot reproduce", ex.Optimal)
			}
			if !reflect.DeepEqual(engine.Buffer, tt.buffer) {
				t.Errorf("the optimal leaves buffer %q, but the mission states %q", engine.Buffer, tt.buffer)
			}
			if engine.Cursor != tt.cursor {
				t.Errorf("the optimal leaves cursor %+v, but the mission states %+v", engine.Cursor, tt.cursor)
			}
			if !reflect.DeepEqual(result.TargetBuffer, engine.Buffer) {
				t.Errorf("TargetBuffer = %q, the engine produced %q", result.TargetBuffer, engine.Buffer)
			}
		})
	}
}

// TestBackedExercises_PlausibleWrongAnswersAreRejected pins that each opt-in
// actually discriminates: every answer below is a believable attempt that does
// not reach the mission's result, so it must be judged incorrect and named as
// diverging. The p/P pairs and the missing-register-prefix variants are exactly
// the mistakes the prose warned about before the result was checkable.
func TestBackedExercises_PlausibleWrongAnswersAreRejected(t *testing.T) {
	tests := []struct {
		id     string
		answer string
	}{
		{"textobjects_020", "yiwj$p"},
		{"textobjects_021", `yi"jlp`},
		{"changerepeat_004", "jdd"},
		{"macros_011", "yiwj\"ap"},
		{"macros_012", "\"ayyj\"aP"},
		{"macros_013", "\"+yi'j\"0p"},
		{"macros_014", "\"+yiwjf'l\"+p"},
		{"macros_015", "yiwjddf'l\"0p"},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			ex := findExerciseByID(t, tt.id)
			if IsInSolutions(&ex, tt.answer) {
				t.Fatalf("%q is an authored solution of %s; pick a wrong answer", tt.answer, tt.id)
			}

			result := ValidateAnswerDetailed(&ex, tt.answer)
			if result.IsCorrect {
				t.Errorf("%s accepted the wrong answer %q", tt.id, tt.answer)
			}
			if !result.BufferVerified {
				t.Error("the wrong answer was not judged by the buffer judge")
			}
			if result.MismatchSummary() == "" {
				t.Error("the wrong answer has no mismatch summary to report to the player")
			}
		})
	}
}

// TestRewordedExercises_ClaimOnlyWhatIsJudged is the other half of the honesty
// pass. For every exercise whose command the engine cannot run, its mission must
// no longer claim the unverifiable result, must say the trainer checks the
// keystrokes, and must not claim a buffer result. The old prose fragment being
// gone is what pins that the specific promise was withdrawn rather than merely
// rephrased.
func TestRewordedExercises_ClaimOnlyWhatIsJudged(t *testing.T) {
	for id, oldClaim := range honestyRewordedExerciseIDs {
		t.Run(id, func(t *testing.T) {
			ex := findExerciseByID(t, id)
			if ex.BufferVerified {
				t.Errorf("%s opted into the buffer judge, but its command is not modelled; it cannot be judged by result", id)
			}
			if strings.Contains(ex.Mission, oldClaim) {
				t.Errorf("%s mission still claims the unverifiable result %q: %q", id, oldClaim, ex.Mission)
			}
			if !strings.Contains(ex.Mission, "keystrokes") {
				t.Errorf("%s mission does not say the trainer checks the keystrokes: %q", id, ex.Mission)
			}
			if missionClaimsABufferResult(ex.Mission) {
				t.Errorf("%s mission still claims a buffer result: %q", id, ex.Mission)
			}
		})
	}
}

// exCommandToken matches the start of an Ex command as a mission writes one: a
// colon followed directly by a range character or a command letter, the shape of
// ":g/…", ":%s/…", ":2,4s/…" and ":normal …". A prose colon is followed by a
// space ("Note: …") and does not match.
var exCommandToken = regexp.MustCompile(`:[%$0-9a-z]`)

// missionMentionsUnmodelledEffect reports whether a mission names an operation
// whose result the judge cannot verify: a register or put the buffer judge would
// have to observe, or an Ex command, which the engine does not execute. The
// previous detector enumerated a few literal words (":normal", ":norm",
// "register", "paste", "put ") and so missed the :g/…/normal @a and :v/…/d
// lessons, which make the same promise in other words. This one keys on the
// Ex-command form itself, so a new global or range command is covered without
// its exact name being listed here.
//
// The detector is a text heuristic over English mission prose. It keys on: the
// Ex-command form or a register word, plus a result claim (the modal "must" or a
// line-scope phrase), unless the mission disclaims the result by saying it checks
// the keystrokes. It still cannot catch:
//   - a promise that names no mechanism and no line scope, such as "make every
//     console line a comment", because there is no word to key on;
//   - a single-line or range substitute phrased without "all … lines" or
//     "must", such as "replace 'foo' with 'bar' on this line using :s/…": it
//     names the Ex command, but the claim is not one of the matched shapes;
//   - a promise written in another language, because the markers are English.
func missionMentionsUnmodelledEffect(mission string) bool {
	m := strings.ToLower(mission)
	for _, marker := range []string{"register", "paste", "put "} {
		if strings.Contains(m, marker) {
			return true
		}
	}
	return exCommandToken.MatchString(m)
}

// missionSaysItChecksKeystrokes reports whether a mission explicitly downgrades
// itself to the keystrokes it can check. That disclaimer is what makes a mission
// that still names the Ex command honest: it tells the player that the trainer
// types the command but does not run it.
func missionSaysItChecksKeystrokes(mission string) bool {
	return strings.Contains(strings.ToLower(mission), "keystrokes")
}

// lineScopeResultClaim matches a claim that a command changes a run of lines at
// once ("all lines", "all TODO lines", "all error lines"), which is the other
// way the corpus phrases a buffer result. It is deliberately plural and scoped:
// "this line" is a single-command claim, not a result the judge could still
// fail to check, so a per-line substitute is not swept up here.
var lineScopeResultClaim = regexp.MustCompile(`\ball\b[^.?!\n]*?\blines\b`)

// missionClaimsABufferResult reports whether a mission states a buffer outcome.
// The two buffer-judged modules phrase every such claim as "the buffer must
// read ..." / "the line must read ...", so the modal "must" is one result-claim
// marker; lineScopeResultClaim catches the same claim written as "all … lines".
func missionClaimsABufferResult(mission string) bool {
	m := strings.ToLower(mission)
	return strings.Contains(m, "must") || lineScopeResultClaim.MatchString(m)
}

// TestNoMissionPromisesAResultItsJudgeCannotCheck is the guard that keeps the
// class of dishonesty the content-honesty pass fixed from coming back silently:
// an exercise whose mission names an operation the judge cannot model must
// either opt into the buffer judge, say it checks only the keystrokes, or not
// claim a result. It enumerates every shipped lesson and boss step, so a new
// exercise cannot slip through.
func TestNoMissionPromisesAResultItsJudgeCannotCheck(t *testing.T) {
	for _, module := range moduleUnlockOrder {
		for _, ex := range GetLessons(module) {
			checkMissionClaims(t, ex)
		}
		if boss := GetBoss(module); boss != nil {
			for _, step := range boss.Steps {
				checkMissionClaims(t, step.Exercise)
			}
		}
	}
}

// TestMissionHonestyDetector pins what the broadened guard keys on. It feeds the
// detector the two shapes of the macros_020–024 promise, a register round trip
// with no result claim, the reworded form that disclaims the result, and two
// missions the detector does not catch, so a future edit that narrows or widens
// it has to change this table on purpose.
func TestMissionHonestyDetector(t *testing.T) {
	cases := []struct {
		name    string
		mission string
		mention bool
		claim   bool
	}{
		{
			name:    "a global command that claims every matching line",
			mission: "Comment all lines containing 'console': :g/console/normal I// ",
			mention: true,
			claim:   true,
		},
		{
			name:    "the inverse global command",
			mission: "Delete all lines NOT containing 'keep': :v/keep/d",
			mention: true,
			claim:   true,
		},
		{
			name:    "a register round trip with no result claim",
			mission: "Play the macro stored in register 'a' using @a",
			mention: true,
			claim:   false,
		},
		{
			name:    "the reworded form disclaims the result",
			mission: "Type the ex command that would comment the lines containing 'console': :g/console/normal I//  (the trainer checks the keystrokes; it does not execute ex commands)",
			mention: true,
			claim:   false,
		},
		{
			name:    "a plain motion names no unmodelled effect",
			mission: "Move to the start of 'userName' using w (word)",
			mention: false,
			claim:   false,
		},
		{
			name:    "the detector still misses a result claim with no command and no line scope",
			mission: "Make the console calls comments",
			mention: false,
			claim:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := missionMentionsUnmodelledEffect(tc.mission); got != tc.mention {
				t.Errorf("missionMentionsUnmodelledEffect(%q) = %v, want %v", tc.mission, got, tc.mention)
			}
			if got := missionClaimsABufferResult(tc.mission); got != tc.claim {
				t.Errorf("missionClaimsABufferResult(%q) = %v, want %v", tc.mission, got, tc.claim)
			}
			if missionSaysItChecksKeystrokes(tc.mission) && tc.claim {
				t.Errorf("%q both disclaims the result and claims it", tc.mission)
			}
		})
	}
}

func checkMissionClaims(t *testing.T, ex Exercise) {
	t.Helper()
	if !missionMentionsUnmodelledEffect(ex.Mission) {
		return
	}
	if ex.BufferVerified || missionSaysItChecksKeystrokes(ex.Mission) {
		return
	}
	if missionClaimsABufferResult(ex.Mission) {
		t.Errorf("%s mission names an operation the judge cannot model and claims a result, but the exercise neither opts into the buffer judge nor says it checks the keystrokes: %q", ex.ID, ex.Mission)
	}
}

// =============================================================================
// HINT COHERENCE - A HINT MUST ADD WHAT ITS MISSION DOES NOT SAY
// =============================================================================

// The rule these guards enforce is stated where the content lives, above
// GetLessons in exercises.go, and in docs/vim-trainer-spec.md: the mission states
// the goal and the hint states the mechanism the mission does not give. A hint
// that only repeats its mission costs the player the keypress that asked for it.

// hintTokenPattern is how both sides of the comparison are read: runs of letters,
// runs of digits, and every other non-blank character on its own, so "yiwjlp",
// "2dd", ":s/foo/bar/g" and "<Esc>" all decompose into the same pieces.
var hintTokenPattern = regexp.MustCompile(`[A-Za-z]+|[0-9]+|[^A-Za-z0-9\s]`)

var (
	// hintCountDetail matches a count or a flag: what 6x, 2<<, 3@a and the g of
	// :s/foo/bar/g tell the player that the mission did not.
	hintCountDetail = regexp.MustCompile(`[0-9]`)
	// hintComparisonDetail matches a hint that relates its command to the sibling
	// the player is most likely to confuse it with.
	hintComparisonDetail = regexp.MustCompile(`(?i)\b(like|unlike|same as|instead|opposite|whereas|vs)\b`)
	// hintConsequenceDetail matches the "why" half of a mechanism: what follows
	// from the command, which a restatement of the mission never states.
	hintConsequenceDetail = regexp.MustCompile(`(?i)\b(so|because|since|therefore|means|which is why|that is why)\b`)
)

// shippedExercises returns every lesson and boss step the trainer ships, in
// module order, so a content guard covers the same corpus the player plays.
func shippedExercises() []Exercise {
	var all []Exercise
	for _, module := range moduleUnlockOrder {
		all = append(all, GetLessons(module)...)
		if boss := GetBoss(module); boss != nil {
			for _, step := range boss.Steps {
				all = append(all, step.Exercise)
			}
		}
	}
	return all
}

// hintTokens lowercases and splits a piece of exercise copy into the tokens the
// hint checker compares.
func hintTokens(s string) []string {
	return hintTokenPattern.FindAllString(strings.ToLower(s), -1)
}

// hintCommandVocabulary is the command vocabulary the checker keys on: every
// command token the shipped solutions and optimals are written from. Deriving it
// from the corpus rather than listing commands by hand means a new exercise that
// ships a new command token also teaches the checker about it.
func hintCommandVocabulary() map[string]bool {
	vocab := map[string]bool{}
	for _, ex := range shippedExercises() {
		for _, tok := range hintTokens(ex.Optimal) {
			addHintCommandToken(vocab, tok)
		}
		for _, sol := range ex.Solutions {
			for _, tok := range hintTokens(sol) {
				addHintCommandToken(vocab, tok)
			}
		}
	}
	return vocab
}

// addHintCommandToken records one token from the corpus as a command. Bare
// punctuation is not a command -- a hyphen or a sentence-final period in prose
// would otherwise count as one, and a hint containing either would look like it
// added something. Neither is a bare number, which the count check already
// covers on its own. The article "a" and the pronoun "i" are dropped too: as
// tokens they are far more often English than the append and insert commands
// they also name, so accepting them would credit any sentence with an "a" in it.
func addHintCommandToken(vocab map[string]bool, tok string) {
	if !strings.ContainsAny(tok, "abcdefghijklmnopqrstuvwxyz") {
		return
	}
	if tok == "a" || tok == "i" {
		return
	}
	vocab[tok] = true
}

// hintMechanismDetails lists what a hint adds to its mission: a count or flag, a
// command token from the trainer's own vocabulary that the mission does not name,
// a comparison with a sibling command, or a consequence. An empty result means
// the hint only says again what the mission already said.
//
// The detector reads English prose, so a hint written in another language is not
// covered. It also cannot see a restatement that borrows a count or a comparison
// word, or one whose only addition is the meaning of a command the mission names
// without explaining: the explicit list below is what pins those.
func hintMechanismDetails(mission, hint string, vocab map[string]bool) []string {
	missionTokens := map[string]bool{}
	for _, tok := range hintTokens(mission) {
		missionTokens[tok] = true
	}

	var details []string
	if hintCountDetail.MatchString(hint) {
		details = append(details, "a count or flag")
	}
	for _, tok := range hintTokens(hint) {
		if vocab[tok] && !missionTokens[tok] {
			details = append(details, "the command "+tok)
			break
		}
	}
	if hintComparisonDetail.MatchString(hint) {
		details = append(details, "a comparison with a sibling command")
	}
	if hintConsequenceDetail.MatchString(hint) {
		details = append(details, "a consequence")
	}
	return details
}

// TestHintEchoDetector pins what the guard keys on with constructed pairs. The
// first case is the shipped copy of the reported defect, verbatim, so the guard
// cannot rot into a tautology that accepts the sentence that started this pass:
// if the detector stops flagging it, this test fails on its own.
func TestHintEchoDetector(t *testing.T) {
	vocab := hintCommandVocabulary()

	cases := []struct {
		name     string
		mission  string
		hint     string
		expected bool // the hint adds at least one mechanism detail
	}{
		{
			name:     "the reported echo: the mission says w and the hint says w",
			mission:  "Move to the start of 'userName' using w (word)",
			hint:     "w moves to the start of the next word",
			expected: false,
		},
		{
			name:     "the same restatement with the command spelled out as a name",
			mission:  "Jump to the top of the visible screen using H (High)",
			hint:     "H takes you to the Highest line on screen",
			expected: false,
		},
		{
			name:     "a hint that adds the count the command takes",
			mission:  "Delete the first six characters 'const ' with a single counted command.",
			hint:     "x takes a count: 6x deletes six characters in one command.",
			expected: true,
		},
		{
			name:     "a hint that adds the register the mission leaves out",
			mission:  "Delete 'oldName' - just the word, not the parenthesis",
			hint:     "de deletes to the END of the word, not including the next character",
			expected: true,
		},
		{
			name:     "a hint that compares with a sibling command",
			mission:  "Convert 'getUser' to 'GETUSER' using gUiw",
			hint:     "gU with a word motion: gUiw upper-cases one word, unlike gu which lowers it.",
			expected: true,
		},
		{
			name:     "a hint that states a consequence",
			mission:  "Open a new line below line 1 with o and type '  return', then leave insert mode.",
			hint:     "o then <Esc> leaves the new line empty, so nothing of it is written to the file.",
			expected: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := len(hintMechanismDetails(tc.mission, tc.hint, vocab)) > 0
			if got != tc.expected {
				t.Errorf("hintMechanismDetails(%q, %q) found a detail = %v, want %v",
					tc.mission, tc.hint, got, tc.expected)
			}
		})
	}
}

// TestShippedHintsAddWhatTheirMissionDoesNot is the guard over the real corpus:
// every shipped hint, in every module and boss fight, must add a mechanism detail
// its mission does not give.
func TestShippedHintsAddWhatTheirMissionDoesNot(t *testing.T) {
	vocab := hintCommandVocabulary()
	checked := 0

	for _, ex := range shippedExercises() {
		if ex.Hint == "" {
			continue
		}
		checked++
		if len(hintMechanismDetails(ex.Mission, ex.Hint, vocab)) == 0 {
			t.Errorf("%s hint repeats its mission and adds no count, sibling or consequence\n  mission: %q\n  hint:    %q",
				ex.ID, ex.Mission, ex.Hint)
		}
	}

	if checked < 200 {
		t.Fatalf("only %d hints were checked; the sweep is not reaching the corpus", checked)
	}
}

// hintEchoRewrites holds, for every exercise whose hint was a restatement of its
// mission, the exact copy that was withdrawn. The test below asserts each echo is
// gone, so restoring the old sentence fails here instead of reaching the player.
var hintEchoRewrites = map[string]string{
	"horizontal_001":          "w moves to the start of the next word",
	"horizontal_003":          "e moves to the end of the current or next word",
	"horizontal_004":          "b moves to the start of the previous word",
	"horizontal_005":          "$ moves to the last character of the line",
	"vertical_001":            "j moves the cursor down one line",
	"vertical_002":            "k moves the cursor up one line",
	"vertical_004":            "gg goes to the first line of the file",
	"vertical_009":            "{ moves to the previous blank line",
	"vertical_010":            "+ moves down and to the first non-blank character",
	"vertical_011":            "- moves up and to the first non-blank character",
	"vertical_014":            "You can chain any motions together",
	"vertical_015":            "Sometimes going to the end and moving back is faster",
	"vertical_016":            "H takes you to the Highest line on screen",
	"vertical_017":            "M takes you to the Middle line on screen",
	"vertical_018":            "L takes you to the Lowest line on screen",
	"vertical_019":            "Ctrl+d scrolls Down half a page",
	"vertical_020":            "Ctrl+u scrolls Up half a page",
	"vertical_021":            "Ctrl+f scrolls Forward a full page",
	"vertical_022":            "Ctrl+b scrolls Backward a full page",
	"changerepeat_011":        "x deletes the character under the cursor",
	"changerepeat_013":        "dd deletes the current line. The DOT (.) in real Vim would repeat this command!",
	"changerepeat_016":        "gn goes to the Next match and Visually selects it",
	"changerepeat_020":        "* searches forward for the word under cursor",
	"changerepeat_021":        "# searches backward for the word under cursor",
	"changerepeat_022":        "n repeats the last search in the same direction",
	"changerepeat_024":        ". repeats your last change command",
	"changerepeat_boss_step1": "dd deletes the entire current line",
	"changerepeat_boss_step3": "dd deletes the entire current line",
	"changerepeat_boss_step4": "D deletes from cursor to end of line in one keystroke",
	"changerepeat_boss_step5": "dd deletes the entire current line",
	"editing_002":             "dd deletes the whole current line.",
	"editing_004":             "Ctrl-r reapplies the change u just undid.",
	"editing_008":             ">> shifts the current line one 'shiftwidth' to the right.",
	"editing_009":             "I starts insert mode at the first non-blank, after the indentation.",
	"editing_010":             "A jumps to the end of the line and starts insert mode there.",
	"editing_011":             "o opens a new line below the cursor and starts insert mode on it.",
	"editing_019":             "Ctrl-r restores the whole insert session u removed, in one step.",
	"macros_003":              "Just press q to stop recording",
	"macros_005":              "@ followed by register letter plays that macro",
	"macros_007":              "@@ repeats the most recently executed macro",
	"macros_008":              "Double @ replays whatever macro you last executed",
	"macros_012":              "\"a before the yank stores the line in register a; \"ap puts that named register.",
	"macros_020":              ":g/pattern/command runs command on all lines matching pattern",
	"macros_021":              ":g/pattern/d deletes all lines matching pattern",
	"macros_022":              "Combine :g (global) with :normal @a to run macro on matching lines",
	"regex_007":               "* searches forward for the word under the cursor",
	"regex_008":               "* searches forward for the exact word under cursor",
	"regex_017":               "**/* matches all files in all subdirectories",
	"regex_018":               ":copen opens the quickfix window to see all search results",
	"regex_023":               "Quote patterns with special characters",
	"regex_boss_step2":        "* searches for word under cursor",
	"regex_boss_step3":        "Use ? to search backward",
	"regex_boss_step4":        "Use \\v for very magic mode with regex",
	"regex_boss_step5":        "Use word boundaries \\< and \\>",
	"registers_007":           "The \"a prefix names register a for both the yank and the put.",
	"registers_010":           ">> shifts the current line one 'shiftwidth' to the right.",
	"registers_011":           "<< removes one shift of indentation from the current line.",
	"substitution_001":        "r replaces single character without entering insert mode",
	"substitution_005":        "s deletes the character under cursor and enters insert mode",
	"substitution_006":        "s removes one character and enters insert mode for replacement",
	"substitution_009":        "~ toggles the case of the character under cursor and moves right",
	"substitution_011":        "gu followed by a motion lowercases the text covered by that motion",
	"substitution_012":        "guu lowercases the entire current line",
	"substitution_013":        "gU followed by a motion uppercases the text covered by that motion",
	"substitution_014":        "gU with word motions uppercases entire words",
	"substitution_016":        ":s/old/new/ substitutes 'old' with 'new' on current line",
	"textobjects_002":         "iw selects the entire word even if cursor is in the middle",
	"textobjects_004":         "i\" selects everything INSIDE the double quotes",
	"textobjects_015":         "at selects the entire tag including opening and closing tags",

	// The echoes the pass's second sweep found once the guard's command vocabulary
	// was tightened: the first version counted bare punctuation as a command, so a
	// hint carrying a hyphen or a sentence-final period looked like it added
	// something. Both groups were rewritten in the same pass, and the stricter
	// sweep now runs over every shipped hint.
	"horizontal_008":          "f followed by a character takes you to that character",
	"horizontal_014":          "You can put a number before any motion to repeat it",
	"horizontal_015":          "There are two parentheses - how do you get to the second directly?",
	"horizontal_016":          "E moves to the end of the current WORD (space-separated)",
	"vertical_005":            "G (capital) goes to the last line",
	"vertical_008":            "} moves to the next blank line (paragraph boundary)",
	"vertical_013":            "You can use counts with } and { too",
	"changerepeat_003":        "$ moves to end of line - combine with d to delete to end",
	"changerepeat_004":        "dd = delete + delete = delete entire line",
	"changerepeat_005":        "D is the uppercase shortcut - deletes to end of line",
	"changerepeat_007":        "ciw = change inner word. Works even if cursor is in MIDDLE of word!",
	"changerepeat_010":        "C is the uppercase shortcut for c$ - changes to end of line",
	"changerepeat_017":        "cgn = change + go to next match. It changes the next occurrence!",
	"changerepeat_018":        "dgn = delete + go to next match. Deletes the next occurrence!",
	"changerepeat_019":        "cgn starts the change, then . repeats it on each subsequent match",
	"changerepeat_boss_step2": "cw changes from cursor to end of word, entering insert mode",
	"editing_006":             "a is i shifted one character right: it inserts after the cursor.",
	"editing_007":             "D is d$: delete to the end of the line.",
	"editing_014":             "yy copies a line; p pastes it below the cursor.",
	"editing_015":             "o then <Esc> leaves the new line empty; the auto-indentation is dropped again.",
	"editing_022":             "% is a motion: it moves to the bracket matching the one under the cursor.",
	"macros_006":              "@ + register letter executes the recorded keystrokes",
	"macros_011":              "\"a before a yank stores the text in register a; a later \"ap puts that named register.",
	"regex_002":               "Use /pattern to search forward - the pattern is 'error'",
	"regex_004":               "Use ?pattern to search backward - you're at the bottom looking up",
	"regex_005":               "After a search, n jumps to the next match in the same direction",
	"regex_010":               "# searches backward - perfect for finding where something was defined",
	"regex_011":               "\\v enables 'very magic' mode where special chars work without escaping",
	"regex_012":               "\\v lets you use \\w+ (one or more word chars) without extra escaping",
	"regex_014":               "\\w matches word characters: letters, digits, and underscore",
	"regex_016":               ":vimgrep /pattern/ files - searches pattern in multiple files",
	"regex_019":               ":cw is a shorter alias that only opens if there are entries",
	"regex_boss_step1":        "Use / to search forward",
	"registers_018":           "A named yank fills the named register and the unnamed one; a later unnamed yank cannot touch the named register.",
	"registers_020":           "Put the copy first, then edit it: A appends at the end of the line the cursor is on.",
	"substitution_003":        "R enters replace mode - each character you type overwrites the existing one",
	"substitution_015":        "J joins the current line with the next line, adding a space between them",
	"textobjects_005":         "a\" selects the quotes AND everything inside them",
	"textobjects_007":         "a' includes the quote characters in the selection",
	"textobjects_008":         "i( or i) or ib all select inside parentheses",
	"textobjects_009":         "a( includes the parentheses themselves",
	"textobjects_010":         "i{ or i} or iB selects inside curly braces",
	"textobjects_011":         "a{ includes the curly braces in the selection",
	"textobjects_012":         "i[ or i] selects inside square brackets",
	"textobjects_013":         "a[ includes the square brackets",
	"textobjects_014":         "it selects inside XML/HTML tags",
	"textobjects_016":         "diw = delete inner word. Works from any position in the word!",
	"textobjects_017":         "daw = delete a word including surrounding whitespace",
	"textobjects_018":         "ci\" = change inside quotes. Deletes content and enters insert mode",
	"textobjects_019":         "di{ deletes content inside {} but keeps the braces",
	"textobjects_020":         "yiw = yank inner word. It fills a register without changing the buffer; the following P is what makes the yank visible.",
	"textobjects_021":         "yi\" = yank inside quotes. It copies the quoted content without the quotes; P puts it back between a pair of them.",
}

// TestRewrittenHintsWithdrewTheRepeatedMechanism pins that the specific echoes
// this pass withdrew are gone, so a later edit that restores the old sentence --
// or the same sentence with the command named in different words -- fails here
// rather than silently costing the player their keypress again.
func TestRewrittenHintsWithdrewTheRepeatedMechanism(t *testing.T) {
	for id, withdrawn := range hintEchoRewrites {
		t.Run(id, func(t *testing.T) {
			ex := findExerciseByID(t, id)
			if ex.Hint == "" {
				t.Fatalf("%s has no hint at all; the pass replaced the echo instead of dropping it", id)
			}
			if strings.Contains(ex.Hint, withdrawn) {
				t.Errorf("%s hint still carries the withdrawn echo %q\n  hint: %q", id, withdrawn, ex.Hint)
			}
		})
	}
}
