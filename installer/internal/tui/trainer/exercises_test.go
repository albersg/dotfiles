package trainer

import (
	"reflect"
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
		if ex.Hint == "" {
			t.Errorf("Lesson %d: Hint is empty", i)
		}
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
		// Hint should be longer than just the solution
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
		if ex.Hint == "" {
			t.Errorf("Lesson %d: Hint is empty", i)
		}
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
		if ex.Hint == "" {
			t.Errorf("Lesson %d: Hint is empty", i)
		}
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
		if ex.Hint == "" {
			t.Errorf("Lesson %d: Hint is empty", i)
		}
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

func TestAllModules_HaveUniqueExerciseIDs(t *testing.T) {
	modules := []ModuleID{
		ModuleHorizontal,
		ModuleVertical,
		ModuleTextObjects,
		ModuleChangeRepeat,
		ModuleSubstitution,
		ModuleRegex,
		ModuleMacros,
		ModuleEditing,
		ModuleRegisters,
	}

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
	modules := []ModuleID{
		ModuleHorizontal,
		ModuleVertical,
		ModuleTextObjects,
		ModuleChangeRepeat,
		ModuleSubstitution,
		ModuleRegex,
		ModuleMacros,
		ModuleEditing,
		ModuleRegisters,
	}

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
	modules := []ModuleID{
		ModuleHorizontal,
		ModuleVertical,
		ModuleTextObjects,
		ModuleChangeRepeat,
		ModuleSubstitution,
		ModuleRegex,
		ModuleMacros,
		ModuleEditing,
		ModuleRegisters,
	}

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
var honestyRewordedExerciseIDs = map[string]string{
	"changerepeat_009": "Change the entire comment line",
	"substitution_007": "for replacement",
	"substitution_008": "preserves indent",
	"substitution_018": "using :%s",
	"macros_016":       "comment prefix to current line",
	"macros_017":       "Add semicolon to end of current line",
	"macros_018":       "Comment all lines with",
	"macros_019":       "Add semicolons to lines 2-4",
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

// missionMentionsRegisterPutOrNormal reports whether a mission talks about a
// result that only a buffer or a register could hold: a put, a register, or the
// :normal ex command. It is the trigger for the guard below, and it is the class
// of promise the content-honesty pass had to make honest.
func missionMentionsRegisterPutOrNormal(mission string) bool {
	m := strings.ToLower(mission)
	for _, marker := range []string{"register", "paste", "put ", ":normal", ":norm"} {
		if strings.Contains(m, marker) {
			return true
		}
	}
	return false
}

// missionClaimsABufferResult reports whether a mission states a buffer outcome.
// The two buffer-judged modules phrase every such claim as "the buffer must
// read ..." / "the line must read ...", so the modal "must" is the corpus's
// result-claim marker, and a reworded mission that claims only keystrokes never
// contains it.
func missionClaimsABufferResult(mission string) bool {
	return strings.Contains(strings.ToLower(mission), "must")
}

// TestNoMissionPromisesAResultItsJudgeCannotCheck is the guard that keeps the
// class of dishonesty the content-honesty pass fixed from coming back silently:
// an exercise whose mission mentions a put, a register or :normal must either
// opt into the buffer judge or not claim a buffer result. It enumerates every
// shipped lesson and boss step, so a new exercise cannot slip through.
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

func checkMissionClaims(t *testing.T, ex Exercise) {
	t.Helper()
	if !missionMentionsRegisterPutOrNormal(ex.Mission) {
		return
	}
	if ex.BufferVerified {
		return
	}
	if missionClaimsABufferResult(ex.Mission) {
		t.Errorf("%s mission mentions a put, a register or :normal and claims a buffer result, but the exercise does not opt into the buffer judge: %q", ex.ID, ex.Mission)
	}
}
