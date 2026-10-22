// Package trainer implements a Vim mastery RPG-style trainer
package trainer

import "time"

// ModuleID identifies a training module
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
)

// ExerciseType defines the type of exercise
type ExerciseType string

const (
	ExerciseLesson   ExerciseType = "lesson"
	ExercisePractice ExerciseType = "practice"
	ExerciseBoss     ExerciseType = "boss"
)

// Position represents cursor position in code
type Position struct {
	Line int
	Col  int
}

// ExerciseStats tracks individual exercise performance for practice mode
type ExerciseStats struct {
	TotalAttempts    int   // Total times attempted
	TotalCorrect     int   // Total correct answers
	TotalWrong       int   // Total wrong answers
	ConsecutiveRight int   // Current streak of correct answers
	Mastered         bool  // True when mastered (removed from practice pool)
	LastAttempted    int64 // Unix timestamp
}

// IsMastered reports whether the exercise counts as mastered. It is the single
// mastery predicate: weighted practice selection and the trainer menu's mastery
// count both read it, so the number shown on the menu cannot drift from the
// number that drives practice. A nil receiver is not mastered, which lets
// display code query an exercise that has never been recorded.
func (stats *ExerciseStats) IsMastered() bool {
	return stats != nil && stats.Mastered
}

// Exercise represents a single training exercise
type Exercise struct {
	ID          string       // "horizontal_001"
	Module      ModuleID     // "horizontal", "textobjects", "cgn", etc.
	Level       int          // 1-10
	Type        ExerciseType // "lesson", "practice", "boss"
	Code        []string     // Lines of code to display
	CursorPos   Position     // Initial cursor position
	Mission     string       // "Move cursor to the 'N' in 'Name'"
	Solutions   []string     // ["w", "W", "fe"] - all valid solutions
	Optimal     string       // "w" - the best/shortest solution
	Hint        string       // Hint shown after timeout
	Explanation string       // Post-answer explanation
	TimeoutSecs int          // Seconds before showing solution
	Points      int          // Base points for completion
	// BufferVerified opts the exercise into the buffer judge: the answer is run
	// through the mutable editing engine (SimulateEditing) and compared to the
	// optimal's result instead of to the optimal's cursor and selection. It is
	// how undo/redo, put, indentation and every other command whose effect lives
	// in the buffer become verifiable. The zero value keeps the judge the
	// exercise was authored against -- the motion/selection simulator -- so the
	// shipped corpus is unaffected. Opt in only when the exercise's mission
	// states a result the buffer judge can check; an authored Solutions entry is
	// still accepted as ground truth.
	BufferVerified bool
}

// ModuleInfo contains display info for a module
//
// Icon is the module's own mark in the interface's glyph vocabulary -- a Nerd Font
// glyph, like the status glyphs, the action icons and the report marks -- and not an
// emoji: a terminal without an emoji font draws an emoji as a box, which is the rule
// the trainer's own guard and the installer's screen guard both hold. No renderer
// draws it today (the trainer's menu row carries the status glyph instead), so it is
// the module's identity rather than a row: it is kept, and kept in the vocabulary a
// future row can draw without a rewrite.
type ModuleInfo struct {
	ID          ModuleID
	Name        string
	Icon        string
	Description string
	BossName    string
}

// BossStep represents a single step in a boss fight
type BossStep struct {
	Exercise  Exercise
	TimeLimit int // Seconds for this specific step
}

// BossExercise represents a boss fight challenge
type BossExercise struct {
	ID     string
	Module ModuleID
	Name   string     // "The Line Walker"
	Lives  int        // 3
	Steps  []BossStep // Chain of missions
	// BonusTime is extra seconds granted to the step that FOLLOWS a won step:
	// winning a step lets the next one be answered for TimeLimit+BonusTime
	// seconds. The grant is per win and is not accumulated across steps, so
	// winning two steps in a row still grants BonusTime once, not twice. A step
	// that is lost grants nothing, because a lost step never advances the fight.
	BonusTime int
}

// ModuleProgress tracks progress within a module
type ModuleProgress struct {
	// Lessons
	LessonsCompleted int
	LessonsTotal     int

	// Practice
	PracticeAccuracy float64 // 0.0 - 1.0, over the whole history
	PracticeAttempts int
	PracticeCorrect  int
	// RecentResults is the last BossGateWindow practice answers, oldest first. It is
	// what the boss gate reads: the gate is a question about the player's current
	// form, not about a mistake made an hour ago. Persisted, and absent in profiles
	// written before the window existed (RecentAccuracy falls back to the lifetime
	// accuracy, so those profiles keep the gate they had).
	RecentResults []bool

	// Boss
	BossDefeated bool
	BossBestTime time.Duration
	// BossAttempts counts boss steps answered, so a lost fight is recorded too.
	// It is owned by the GameState answer recorders and must not be incremented
	// by the UI or by RecordBossVictory.
	BossAttempts  int
	BossLivesLeft int // Lives remaining on best run

	// Per-exercise tracking for intelligent practice
	ExerciseStats map[string]*ExerciseStats

	// Legacy fields (kept for compatibility)
	WeakExercises []string // IDs of exercises that fail most
	LastPracticed time.Time
}

// UserStats contains all user statistics
type UserStats struct {
	TotalScore    int
	CurrentStreak int
	BestStreak    int
	// TotalTime is the accumulated time spent solving exercises. It is persisted
	// as whole seconds (totalTimeSeconds), so a save/load round trip keeps the
	// seconds and drops any sub-second remainder.
	TotalTime      time.Duration
	ModuleProgress map[ModuleID]*ModuleProgress
	BossesDefeated []ModuleID
	// LastPlayed is when an answer was last recorded, correct or not.
	LastPlayed time.Time
}

// NewUserStats creates a new UserStats with defaults
func NewUserStats() *UserStats {
	return &UserStats{
		ModuleProgress: make(map[ModuleID]*ModuleProgress),
		BossesDefeated: []ModuleID{},
	}
}

// GetModuleProgress returns progress for a module, creating if needed
func (s *UserStats) GetModuleProgress(module ModuleID) *ModuleProgress {
	if s.ModuleProgress == nil {
		s.ModuleProgress = make(map[ModuleID]*ModuleProgress)
	}
	if _, ok := s.ModuleProgress[module]; !ok {
		s.ModuleProgress[module] = &ModuleProgress{}
	}
	return s.ModuleProgress[module]
}

// ModuleProgressOrNil returns the recorded progress for a module without
// creating an entry, or nil when the module has never been opened. The trainer
// menu asks about every module on each render, so the questions it asks must
// read progress without manufacturing the empty records a later save would
// persist; a nil result means the same as an empty record for every question
// here, because all of the recorded counters start at zero.
func (s *UserStats) ModuleProgressOrNil(module ModuleID) *ModuleProgress {
	if s.ModuleProgress == nil {
		return nil
	}
	return s.ModuleProgress[module]
}

// moduleUnlockOrder defines the order modules are unlocked
var moduleUnlockOrder = []ModuleID{
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

// NextModule returns the module that follows the given one in the unlock
// order, and whether such a module exists. The final module has no successor,
// so callers can tell whether defeating a boss unlocks anything.
func NextModule(module ModuleID) (ModuleID, bool) {
	for i, m := range moduleUnlockOrder {
		if m == module && i+1 < len(moduleUnlockOrder) {
			return moduleUnlockOrder[i+1], true
		}
	}
	return "", false
}

// IsModuleUnlocked checks if a module is unlocked
func (s *UserStats) IsModuleUnlocked(module ModuleID) bool {
	// First module is always unlocked
	if module == ModuleHorizontal {
		return true
	}

	// Find position of requested module
	var moduleIdx int = -1
	for i, m := range moduleUnlockOrder {
		if m == module {
			moduleIdx = i
			break
		}
	}

	// Unknown module
	if moduleIdx == -1 {
		return false
	}

	// Need to have defeated the previous boss
	if moduleIdx > 0 {
		prevModule := moduleUnlockOrder[moduleIdx-1]
		return s.IsBossDefeated(prevModule)
	}

	return false
}

// IsBossDefeated checks if a module's boss has been defeated
func (s *UserStats) IsBossDefeated(module ModuleID) bool {
	for _, defeated := range s.BossesDefeated {
		if defeated == module {
			return true
		}
	}
	return false
}

// IsLessonsComplete checks if lessons are 100% complete for a module. It is
// read-only: an unopened module has no recorded progress, which is the same as
// zero completed lessons out of zero total.
func (s *UserStats) IsLessonsComplete(module ModuleID) bool {
	progress := s.ModuleProgressOrNil(module)
	return progress != nil && progress.LessonsTotal > 0 && progress.LessonsCompleted >= progress.LessonsTotal
}

// IsPracticeReady checks if practice mode is unlocked (lessons complete)
func (s *UserStats) IsPracticeReady(module ModuleID) bool {
	return s.IsModuleUnlocked(module) && s.IsLessonsComplete(module)
}

// BossGateAccuracy is the accuracy the recent window must show before the boss opens.
const BossGateAccuracy = 0.80

// BossGateMinAttempts is the fewest practice answers the gate will read at all: a
// player who has answered three questions correctly has not shown anything yet.
const BossGateMinAttempts = 10

// BossGateWindow is how many of the most recent practice answers the gate reads.
const BossGateWindow = 20

// IsBossReady checks if the boss fight is unlocked: at least BossGateMinAttempts
// practice answers, and an accuracy of BossGateAccuracy over the most recent
// BossGateWindow of them. It is read-only for the same reason as IsLessonsComplete.
//
// Reading the recent window rather than the lifetime accuracy is the difference
// between a gate and a tax: with the lifetime figure a player who fumbled the first
// few exercises of a module carried that average into every later attempt, and the
// only way out was to answer enough questions to dilute it.
func (s *UserStats) IsBossReady(module ModuleID) bool {
	if !s.IsPracticeReady(module) {
		return false
	}
	progress := s.ModuleProgressOrNil(module)
	if progress == nil || progress.PracticeAttempts < BossGateMinAttempts {
		return false
	}
	accuracy, _ := progress.RecentAccuracy()
	return accuracy >= BossGateAccuracy
}

// ReviewsDue counts the module's mastered exercises whose review interval has passed
// at now. It is read-only, like every other question the trainer menu asks.
func (s *UserStats) ReviewsDue(module ModuleID, now time.Time) int {
	return s.ModuleProgressOrNil(module).ReviewsDue(module, now)
}

// IsPracticeComplete reports whether every exercise of a module is mastered. It
// reads the recorded progress without creating it, which is what the trainer
// menu needs: the question is asked on every keypress.
func (s *UserStats) IsPracticeComplete(module ModuleID) bool {
	return s.ModuleProgressOrNil(module).IsPracticeComplete(module)
}

// PracticeComplete reports whether a module has nothing left to practise: every
// exercise mastered AND the boss gate already met. It is the rule that ends a
// practice session, and it is deliberately stricter than IsPracticeComplete. A
// module whose exercises are all mastered but whose boss is still locked keeps
// offering practice -- review is what lets the player reach the gate -- instead
// of refusing to start and leaving the boss as the only exit, which was a
// deadlock. A due review is the same kind of "still something to do": the trainer
// promised spaced repetition, so mastery that has come due is work, not a finished
// module.
func (s *UserStats) PracticeComplete(module ModuleID, now time.Time) bool {
	return s.IsPracticeComplete(module) && s.IsBossReady(module) && s.ReviewsDue(module, now) == 0
}

// GetAllModules returns info for all modules in order
func GetAllModules() []ModuleInfo {
	return []ModuleInfo{
		{
			ID:          ModuleHorizontal,
			Name:        "Horizontal Motions",
			Icon:        "\uf07e", // nf-fa-arrows-h: the motions that move along a line
			Description: "w, W, e, E, b, B, f, F, t, T, ;, ,, 0, $, ^",
			BossName:    "The Line Walker",
		},
		{
			ID:          ModuleVertical,
			Name:        "Vertical Motions",
			Icon:        "\uf07d", // nf-fa-arrows-v: the motions that move between lines
			Description: "j, k, gg, G, {, }, H, M, L, ctrl+d/u/f/b",
			BossName:    "The Code Tower",
		},
		{
			ID:          ModuleTextObjects,
			Name:        "Text Objects",
			Icon:        "\uf125", // nf-fa-crop: the selection a text object names
			Description: "viw, vaw, vi\", va\", vi{, diw, daw, ci\", di{, yiw, yi\"",
			BossName:    "The Bracket Demon",
		},
		{
			ID:          ModuleChangeRepeat,
			Name:        "Change & Repeat",
			Icon:        "\uf01e", // nf-fa-repeat: the edit that repeats
			Description: "d, c, dd, D, C, x, *, #, n, N, gn, cgn, dgn, .",
			BossName:    "The Clone Army",
		},
		{
			ID:          ModuleSubstitution,
			Name:        "Substitution",
			Icon:        "\uf0ec", // nf-fa-exchange: the substitution
			Description: "r, R, s, S, ~, gu, gU, J, :s, :%s, flags (g, c, i)",
			BossName:    "The Transformer",
		},
		{
			ID:          ModuleRegex,
			Name:        "Regex & Vimgrep",
			Icon:        "\uf002", // nf-fa-search: the pattern it searches with
			Description: "/, ?, n, N, *, #, \\v, :vimgrep, :copen, :cnext",
			BossName:    "The Pattern Master",
		},
		{
			ID:          ModuleMacros,
			Name:        "Macros",
			Icon:        "\uf085", // nf-fa-cogs: the automaton that replays the keys
			Description: "qa, q, @a, @@, :normal, :g/pattern/",
			BossName:    "The Automaton",
		},
		{
			ID:          ModuleEditing,
			Name:        "Editing & Undo",
			Icon:        "\uf0e2", // nf-fa-undo: the edit and the undo of it
			Description: "i, a, I, A, o, O, <Esc>, u, Ctrl-r, dd, yy, p, P, >>, <<, x, D, %, marks",
			BossName:    "The Historian",
		},
		{
			ID:          ModuleRegisters,
			Name:        "Registers & Indentation",
			Icon:        "\uf0ea", // nf-fa-clipboard: the registers and the indent it keeps
			Description: "yy, yiw, y$, yw, yj, p, P, \"a-\"z, \"0, dd, x, D, >>, <<",
			BossName:    "The Archivist",
		},
	}
}
