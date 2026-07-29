---
name: dotfiles-trainer
description: >
  Vim Trainer RPG system patterns for dotfiles.
  Trigger: When editing files in installer/internal/tui/trainer/, adding exercises, modules, or game mechanics.
license: Apache-2.0
metadata:
  author: dotfiles-programming
  version: "1.0"
---

## When to Use

Use this skill when:
- Adding new Vim training modules
- Creating exercises or boss fights
- Modifying progression/unlock system
- Working on the Vim command simulator
- Adding practice mode features

---

## Critical Patterns

### Pattern 1: ModuleID Constants

All modules MUST be defined as `ModuleID` constants in `types.go`:

```go
type ModuleID string

const (
    ModuleHorizontal   ModuleID = "horizontal"
    ModuleVertical     ModuleID = "vertical"
    ModuleTextObjects  ModuleID = "textobjects"
    ModuleChangeRepeat ModuleID = "cgn"
    ModuleSubstitution ModuleID = "substitution"
    ModuleRegex        ModuleID = "regex"
    ModuleMacros       ModuleID = "macros"
    // Add new modules here
)
```

### Pattern 2: Exercise Structure

Every exercise follows this structure:

```go
type Exercise struct {
    ID           string       // "horizontal_001"
    Module       ModuleID     // Parent module
    Level        int          // 1-10 difficulty
    Type         ExerciseType // lesson, practice, boss
    Code         []string     // Lines of code shown
    CursorPos    Position     // Initial cursor
    CursorTarget *Position    // Target position (movement exercises)
    Mission      string       // What user must do
    Solutions    []string     // ALL valid solutions
    Optimal      string       // Best/shortest solution
    Hint         string       // Help text
    Explanation  string       // Post-answer teaching
    TimeoutSecs  int          // Before showing solution
    Points       int          // Base score
}
```

### Pattern 3: Module Unlock Order

Modules unlock sequentially - user must defeat boss to unlock next:

```go
var moduleUnlockOrder = []ModuleID{
    ModuleHorizontal,   // Always unlocked
    ModuleVertical,     // After horizontal boss
    ModuleTextObjects,  // After vertical boss
    ModuleChangeRepeat, // After textobjects boss
    // ... etc
}
```

### Pattern 4: Progression Flow

```
Lessons (sequential) → Practice (80% accuracy) → Boss Fight → Next Module
```

---

## Decision Tree

```
Adding new module?
├── Add ModuleID constant in types.go
├── Add to moduleUnlockOrder slice
├── Add ModuleInfo in GetAllModules()
├── Create exercises_{module}.go file
├── Implement GetLessons(moduleID)
├── Implement GetBoss(moduleID)
└── Add practice exercises

Adding exercises?
├── Create Exercise with unique ID format: "{module}_{number}"
├── Provide multiple Solutions (all valid answers)
├── Set Optimal to shortest/best solution
├── Include Hint for learning
└── Add Explanation for post-answer

Adding boss fight?
├── Create BossExercise in exercises_{module}.go
├── Add 5-7 BossSteps (exercise chain)
├── Set Lives (usually 3)
├── Include variety of module skills
└── Return from GetBoss(moduleID)
```

---

## Code Examples

### Example 1: Creating a Module's Exercises File

```go
// exercises_newmodule.go
package trainer

// NewModule lessons
func getNewModuleLessons() []Exercise {
    return []Exercise{
        {
            ID:        "newmodule_001",
            Module:    ModuleNewModule,
            Level:     1,
            Type:      ExerciseLesson,
            Code:      []string{"function example() {", "  return true;", "}"},
            CursorPos: Position{Line: 0, Col: 0},
            Mission:   "Use 'xx' to delete two characters",
            Solutions: []string{"xx", "2x", "dl dl"},
            Optimal:   "xx",
            Hint:      "x deletes character under cursor",
            Explanation: "x is Vim's character delete. 2x or xx deletes two.",
            Points:    10,
        },
        // ... more exercises
    }
}
```

### Example 2: Registering Module in GetAllModules

```go
func GetAllModules() []ModuleInfo {
    return []ModuleInfo{
        // ... existing modules
        {
            ID:          ModuleNewModule,
            Name:        "New Module",
            Icon:        "🆕",
            Description: "Commands: xx, yy, zz",
            BossName:    "The New Boss",
        },
    }
}
```

### Example 3: Boss Fight Structure

```go
func getNewModuleBoss() *BossExercise {
    return &BossExercise{
        ID:     "newmodule_boss",
        Module: ModuleNewModule,
        Name:   "The New Boss",
        Lives:  3,
        Steps: []BossStep{
            {
                Exercise: Exercise{
                    ID:        "newmodule_boss_1",
                    Module:    ModuleNewModule,
                    Code:      []string{"challenge code here"},
                    CursorPos: Position{Line: 0, Col: 0},
                    Mission:   "First boss challenge",
                    Solutions: []string{"w", "W"},
                    Optimal:   "w",
                },
                TimeLimit: 10,
            },
            // ... more steps (5-7 total)
        },
    }
}
```

### Example 4: Exercise Validation

```go
// Validation checks if answer is in Solutions
func ValidateAnswer(exercise *Exercise, answer string) bool {
    answer = strings.TrimSpace(answer)
    for _, solution := range exercise.Solutions {
        if answer == solution {
            return true
        }
    }
    // Also check via simulator for creative solutions
    return validateViaSimulator(exercise, answer)
}
```

---

## Exercise Guidelines

### Good Exercise Design

1. **Clear Mission**: User knows exactly what to do
2. **Multiple Solutions**: Accept all valid Vim ways
3. **Optimal Marked**: Teach the best approach
4. **Progressive Difficulty**: Level 1-10 within module
5. **Real Code**: Use realistic code snippets

### Solutions Array Rules

```go
// GOOD: Accept all valid variations
Solutions: []string{"w", "W", "e", "E", "f "},

// BAD: Only accept one way
Solutions: []string{"w"},
```

### Exercise ID Format

```
{module}_{number}      → "horizontal_001"
{module}_boss_{step}   → "horizontal_boss_1"
```

---

## Commands

```bash
cd installer && go test ./internal/tui/trainer/...     # Run all trainer tests
cd installer && go test -run TestExercise              # Test exercises
cd installer && go test -run TestSimulator             # Test Vim simulator
cd installer && go test -run TestProgression           # Test unlock system
```

---

## Resources

- **Types**: See `installer/internal/tui/trainer/types.go` for data structures
- **Exercises**: See `installer/internal/tui/trainer/exercises_*.go` for patterns
- **Simulator**: See `installer/internal/tui/trainer/simulator.go` for Vim emulation
- **Validation**: See `installer/internal/tui/trainer/validation.go` for answer checking
- **Stats**: See `installer/internal/tui/trainer/stats.go` for persistence
