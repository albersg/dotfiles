---
name: dotfiles-trainer
description: >
  Vim Trainer RPG system patterns for dotfiles.
  Trigger: When editing files in installer/internal/tui/trainer/, adding exercises, modules, or game mechanics.
license: Apache-2.0
metadata:
  author: albersg
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
    ModuleEditing      ModuleID = "editing"
    ModuleRegisters    ModuleID = "registers"
    // Add new modules here
)
```

### Pattern 2: Exercise Structure

Every exercise follows this structure:

```go
type Exercise struct {
    ID           string       // "horizontal_001"
    Module       ModuleID     // Parent module
    Level        int          // 1-10 difficulty: it is what the exercise pays
    Type         ExerciseType // lesson, practice, boss
    Code         []string     // Lines of code shown
    CursorPos    Position     // Initial cursor
    Mission      string       // What user must do
    Solutions    []string     // ALL valid solutions
    Optimal      string       // Best/shortest solution
    Hint         string       // Mechanism the mission does not give (see below)
    Explanation  string       // Post-answer teaching
    TimeoutSecs  int          // Before showing solution
    Points       int          // Base score; PointsForLevel is its floor
    // BufferVerified opts the exercise into the buffer judge: the answer is run
    // against a mutable buffer, which is how undo, put, indentation and any other
    // command whose effect lives in the buffer become verifiable. The zero value
    // keeps the judge the exercise was authored against, so shipped exercises are
    // unaffected; opt in only when the mission states a result the buffer judge
    // can check.
    BufferVerified bool
}
```

An exercise with `BufferVerified` set is judged by running the answer, not by matching the typed string,
so `Solutions` stays useful as ground truth for authoring and for the explanation but is no longer the
verdict. `installer/internal/tui/trainer/editor.go` is the engine; a guard test fails if an exercise claims
the buffer judge and its own answer does not verify against it.

Two rules the corpus guards enforce on every exercise, including boss steps:

- **`Level` is a contract, not a comment.** It stays in 1..10 and it drives the score:
  `ExercisePoints` is `max(Points, PointsForLevel(Level))`, so a hard exercise can never pay
  less than an easy one and a deliberate authored reward still survives. `Level` also steers the
  practice pool towards the frontier (`MasteredLevelFloor` plus `frontierBonus`), so write the level
  you actually mean.
- **A `Hint` adds what its `Mission` does not**: where the command lands, what it includes or
  leaves out, the count or flag it takes, how it compares with its sibling command, or what follows
  from it. `TestShippedHintsAddWhatTheirMissionDoesNot` rejects a hint that only restates the
  mission, and `TestCorpusHintsDoNotQuoteTheAnswer` rejects one that quotes its own answer.

### Pattern 3: Module Unlock Order

Modules unlock sequentially - user must defeat boss to unlock next:

```go
var moduleUnlockOrder = []ModuleID{
    ModuleHorizontal,   // Always unlocked
    ModuleVertical,     // After horizontal boss
    ModuleTextObjects,  // After vertical boss
    ModuleChangeRepeat, // After textobjects boss
    ModuleSubstitution, // After cgn boss
    ModuleRegex,        // After substitution boss
    ModuleMacros,       // After regex boss
    ModuleEditing,      // After macros boss
    ModuleRegisters,    // The last one: it has no successor
}
```

### Pattern 4: Progression Flow

```
Lessons (sequential) → Practice → Boss Fight → Next Module
```

- Practice unlocks when every lesson of the module is passed.
- The boss gate reads the **recent** run: at least `BossGateMinAttempts` (10) practice answers and
  `BossGateAccuracy` (80%) over the last `BossGateWindow` (20) of them. A fumbled first attempt at a
  module is not a tax the player pays forever.
- A mastered exercise comes back for review after `MasteryReviewInterval` (12 h): mastery is a
  resting state, not a grave. `PracticeComplete` -- the only state that ends a practice session --
  needs every exercise mastered, the boss unlocked, and no review due.
- A correct answer raises the combo in every mode (capped at `MaxComboMultiplier`) and a wrong one
  resets it; the headers show it once it is above 1.

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
        ID:        "newmodule_boss",
        Module:    ModuleNewModule,
        Name:      "The New Boss",
        Lives:     3,
        BonusTime: 30, // 15..60: what a won step grants the next one
        Steps: []BossStep{
            {
                Exercise: Exercise{
                    ID:        "newmodule_boss_1",
                    Module:    ModuleNewModule,
                    Level:     5,
                    Code:      []string{"challenge code here"},
                    CursorPos: Position{Line: 0, Col: 0},
                    Mission:   "First boss challenge",
                    Solutions: []string{"w", "W"},
                    Optimal:   "w",
                    Hint:      "The mechanism the mission does not give.",
                },
                // At least BossStepRecommendedSeconds(&step.Exercise): 4s + mission/20 + 1s per
                // answer character, capped at 90s. Compute it, do not guess it.
                TimeLimit: 7,
            },
            // ... more steps (5-7 total)
        },
    }
}
```

Every step needs a `Hint` (`TestCorpusBossStepsCarryAHint`), a `Level` in range, a fair
`TimeLimit`, and a `BonusTime` inside 15..60 on the fight (`corpus_contract_test.go` checks all
four). `Tab` shows the hint, or the first keystrokes of the answer when a step ships none.

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
4. **Progressive Difficulty**: Level 1-10 within module, and the level is what it pays
5. **Real Code**: Use realistic code snippets
6. **A Hint That Teaches**: the mechanism the mission leaves implicit, never the answer
7. **No Spoilers**: neither the mission nor the hint prints the answer in backticks

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
