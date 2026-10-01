package trainer

import (
	"testing"
)

// =============================================================================
// TIPOS BASE: ModuleID, Position, ExerciseType
// =============================================================================

func TestModuleID_Constants(t *testing.T) {
	// Verificar que todos los módulos están definidos correctamente
	modules := allModuleIDs()
	if len(modules) == 0 {
		t.Fatal("GetAllModules returned no modules")
	}

	// Verificar valores únicos
	seen := make(map[ModuleID]bool)
	for _, m := range modules {
		if seen[m] {
			t.Errorf("Duplicate module ID: %s", m)
		}
		seen[m] = true
	}
}

func TestModuleID_StringValues(t *testing.T) {
	tests := []struct {
		module   ModuleID
		expected string
	}{
		{ModuleHorizontal, "horizontal"},
		{ModuleVertical, "vertical"},
		{ModuleTextObjects, "textobjects"},
		{ModuleChangeRepeat, "cgn"},
		{ModuleSubstitution, "substitution"},
		{ModuleRegex, "regex"},
		{ModuleMacros, "macros"},
		{ModuleEditing, "editing"},
		{ModuleRegisters, "registers"},
	}

	for _, tt := range tests {
		if string(tt.module) != tt.expected {
			t.Errorf("ModuleID %v: expected %q, got %q", tt.module, tt.expected, string(tt.module))
		}
	}
}

func TestExerciseType_Constants(t *testing.T) {
	// Verificar tipos de ejercicio
	types := []ExerciseType{
		ExerciseLesson,
		ExercisePractice,
		ExerciseBoss,
	}

	if len(types) != 3 {
		t.Errorf("Expected 3 exercise types, got %d", len(types))
	}

	// Verificar valores string
	if string(ExerciseLesson) != "lesson" {
		t.Errorf("ExerciseLesson should be 'lesson', got %q", ExerciseLesson)
	}
	if string(ExercisePractice) != "practice" {
		t.Errorf("ExercisePractice should be 'practice', got %q", ExercisePractice)
	}
	if string(ExerciseBoss) != "boss" {
		t.Errorf("ExerciseBoss should be 'boss', got %q", ExerciseBoss)
	}
}

func TestPosition_Creation(t *testing.T) {
	pos := Position{Line: 5, Col: 10}

	if pos.Line != 5 {
		t.Errorf("Position.Line: expected 5, got %d", pos.Line)
	}
	if pos.Col != 10 {
		t.Errorf("Position.Col: expected 10, got %d", pos.Col)
	}
}

func TestPosition_ZeroValue(t *testing.T) {
	var pos Position

	if pos.Line != 0 || pos.Col != 0 {
		t.Errorf("Zero Position should be {0, 0}, got {%d, %d}", pos.Line, pos.Col)
	}
}

// =============================================================================
// EXERCISE STRUCT
// =============================================================================

func TestExercise_Creation(t *testing.T) {
	exercise := Exercise{
		ID:          "horizontal_001",
		Module:      ModuleHorizontal,
		Level:       1,
		Type:        ExerciseLesson,
		Code:        []string{"const foo = 'bar';"},
		CursorPos:   Position{Line: 0, Col: 0},
		Mission:     "Move to 'foo'",
		Solutions:   []string{"w", "W"},
		Optimal:     "w",
		Hint:        "Use word motion",
		Explanation: "w moves to next word",
		TimeoutSecs: 30,
		Points:      10,
	}

	if exercise.ID != "horizontal_001" {
		t.Errorf("Exercise.ID: expected 'horizontal_001', got %q", exercise.ID)
	}
	if exercise.Module != ModuleHorizontal {
		t.Errorf("Exercise.Module: expected ModuleHorizontal, got %v", exercise.Module)
	}
	if len(exercise.Solutions) != 2 {
		t.Errorf("Exercise.Solutions: expected 2 solutions, got %d", len(exercise.Solutions))
	}
}

// =============================================================================
// MODULE INFO
// =============================================================================

func TestGetAllModules_ReturnsCorrectCount(t *testing.T) {
	modules := GetAllModules()

	if len(modules) != 9 {
		t.Errorf("Expected 9 modules, got %d", len(modules))
	}
}

func TestGetAllModules_CorrectOrder(t *testing.T) {
	modules := GetAllModules()

	expectedOrder := []ModuleID{
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

	for i, expected := range expectedOrder {
		if modules[i].ID != expected {
			t.Errorf("Module %d: expected %s, got %s", i, expected, modules[i].ID)
		}
	}
}

func TestGetAllModules_HasRequiredFields(t *testing.T) {
	modules := GetAllModules()

	for _, mod := range modules {
		if mod.ID == "" {
			t.Error("Module ID should not be empty")
		}
		if mod.Name == "" {
			t.Errorf("Module %s: Name should not be empty", mod.ID)
		}
		if mod.Icon == "" {
			t.Errorf("Module %s: Icon should not be empty", mod.ID)
		}
		if mod.Description == "" {
			t.Errorf("Module %s: Description should not be empty", mod.ID)
		}
		if mod.BossName == "" {
			t.Errorf("Module %s: BossName should not be empty", mod.ID)
		}
	}
}

func TestGetAllModules_HorizontalIsFirst(t *testing.T) {
	modules := GetAllModules()

	if modules[0].ID != ModuleHorizontal {
		t.Errorf("First module should be Horizontal, got %s", modules[0].ID)
	}
	if modules[0].Icon != "🏃" {
		t.Errorf("Horizontal icon should be 🏃, got %s", modules[0].Icon)
	}
}

func TestGetAllModules_BossNames(t *testing.T) {
	modules := GetAllModules()

	expectedBosses := map[ModuleID]string{
		ModuleHorizontal:   "The Line Walker",
		ModuleVertical:     "The Code Tower",
		ModuleTextObjects:  "The Bracket Demon",
		ModuleChangeRepeat: "The Clone Army",
		ModuleSubstitution: "The Transformer",
		ModuleRegex:        "The Pattern Master",
		ModuleMacros:       "The Automaton",
		ModuleEditing:      "The Historian",
		ModuleRegisters:    "The Archivist",
	}

	for _, mod := range modules {
		expected, ok := expectedBosses[mod.ID]
		if !ok {
			t.Errorf("Unexpected module ID: %s", mod.ID)
			continue
		}
		if mod.BossName != expected {
			t.Errorf("Module %s: expected boss %q, got %q", mod.ID, expected, mod.BossName)
		}
	}
}

// =============================================================================
// MODULE UNLOCK ORDER
// =============================================================================

func TestNextModule_FollowsUnlockOrder(t *testing.T) {
	tests := []struct {
		name   string
		module ModuleID
		want   ModuleID
		ok     bool
	}{
		{"horizontal unlocks vertical", ModuleHorizontal, ModuleVertical, true},
		{"vertical unlocks textobjects", ModuleVertical, ModuleTextObjects, true},
		{"textobjects unlocks cgn", ModuleTextObjects, ModuleChangeRepeat, true},
		{"cgn unlocks substitution", ModuleChangeRepeat, ModuleSubstitution, true},
		{"substitution unlocks regex", ModuleSubstitution, ModuleRegex, true},
		{"regex unlocks macros", ModuleRegex, ModuleMacros, true},
		{"macros unlocks editing", ModuleMacros, ModuleEditing, true},
		{"editing unlocks registers", ModuleEditing, ModuleRegisters, true},
		{"registers is the final module", ModuleRegisters, "", false},
		{"unknown module has no successor", ModuleID("nope"), "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := NextModule(tt.module)
			if ok != tt.ok || got != tt.want {
				t.Errorf("NextModule(%q) = (%q, %v), want (%q, %v)", tt.module, got, ok, tt.want, tt.ok)
			}
		})
	}
}

// =============================================================================
// EXERCISE STATS MASTERY PREDICATE
// =============================================================================

// TestExerciseStats_IsMastered pins the single mastery predicate weighted
// practice selection and the trainer menu both read. A nil record counts as not
// mastered so display code can query an exercise that was never recorded.
func TestExerciseStats_IsMastered(t *testing.T) {
	tests := []struct {
		name  string
		stats *ExerciseStats
		want  bool
	}{
		{"nil record is not mastered", nil, false},
		{"unmastered record is not mastered", &ExerciseStats{Mastered: false}, false},
		{"mastered record is mastered", &ExerciseStats{Mastered: true}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.stats.IsMastered(); got != tt.want {
				t.Errorf("IsMastered() = %v, want %v", got, tt.want)
			}
		})
	}
}

// =============================================================================
// BUFFER JUDGE OPT-IN
// =============================================================================

// shippedLessons returns the lessons of every module in unlock order, and
// shippedBossSteps every boss step, so the regression guards below enumerate the
// real shipped corpus instead of a hand-copied list that could drift from it.
func shippedLessons() []Exercise {
	var all []Exercise
	for _, module := range moduleUnlockOrder {
		all = append(all, GetLessons(module)...)
	}
	return all
}

func shippedBossSteps() []Exercise {
	var all []Exercise
	for _, module := range moduleUnlockOrder {
		if boss := GetBoss(module); boss != nil {
			for _, step := range boss.Steps {
				all = append(all, step.Exercise)
			}
		}
	}
	return all
}

// Exercise.BufferVerified is the opt-in for the buffer judge. Its zero value
// keeps the judge an exercise was written against, so an exercise opts in only
// when its mission states a result the buffer judge can check. Editing & Undo
// and Registers & Indentation are built for the buffer judge, so every one of
// their exercises opts in. The content-honesty pass opted in the buffer-visible
// exercises named in honestyBackedExerciseIDs (see exercises_test.go) after
// giving each a mission that states a buffer result; every other exercise keeps
// the judge it was authored against. The count assertions also prove the
// enumeration found the real corpus rather than an empty list.
func TestShippedExercises_OptInOnlyWhereTheMissionStatesABufferResult(t *testing.T) {
	lessons := shippedLessons()
	if len(lessons) != 199 {
		t.Fatalf("enumerated %d shipped lessons, want 199", len(lessons))
	}

	bossSteps := shippedBossSteps()
	if len(bossSteps) != 45 {
		t.Fatalf("enumerated %d shipped boss steps, want 45", len(bossSteps))
	}

	for _, exercise := range append(lessons, bossSteps...) {
		want := exercise.Module == ModuleEditing ||
			exercise.Module == ModuleRegisters ||
			honestyBackedExerciseIDs[exercise.ID]
		switch {
		case want && !exercise.BufferVerified:
			t.Errorf("buffer-judged exercise %s does not opt into the buffer judge; its mission states a result the buffer judge can check", exercise.ID)
		case !want && exercise.BufferVerified:
			t.Errorf("shipped exercise %s opts into the buffer judge without a mission that states a buffer result", exercise.ID)
		}
	}
}

// The shipped corpus keeps validating exactly as before while the opt-in exists.
// A sample spanning the three judge paths (motion, selection and the
// skip-simulation modules) pins that the new field did not reroute any of them.
func TestShippedExercises_KeepTheirJudge(t *testing.T) {
	tests := []struct {
		name     string
		exercise *Exercise
		answer   string
		want     bool
	}{
		{
			name:     "a motion lesson accepts its optimal",
			exercise: findLesson(t, ModuleHorizontal, "horizontal_001"),
			answer:   findLesson(t, ModuleHorizontal, "horizontal_001").Optimal,
			want:     true,
		},
		{
			name:     "a motion lesson still rejects a different position",
			exercise: findLesson(t, ModuleHorizontal, "horizontal_001"),
			answer:   "b",
			want:     false,
		},
		{
			name:     "a selection lesson accepts its optimal",
			exercise: findLesson(t, ModuleChangeRepeat, "changerepeat_001"),
			answer:   findLesson(t, ModuleChangeRepeat, "changerepeat_001").Optimal,
			want:     true,
		},
		{
			name:     "a skip-simulation lesson accepts only its solutions",
			exercise: findLesson(t, ModuleSubstitution, "substitution_001"),
			answer:   findLesson(t, ModuleSubstitution, "substitution_001").Optimal,
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateAnswer(tt.exercise, tt.answer); got != tt.want {
				t.Errorf("ValidateAnswer(%s, %q) = %v, want %v", tt.exercise.ID, tt.answer, got, tt.want)
			}
		})
	}
}
