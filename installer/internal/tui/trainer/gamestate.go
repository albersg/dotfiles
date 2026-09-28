package trainer

import (
	"time"
)

// GameState manages the current game session state
type GameState struct {
	// Stats & Progress (persisted)
	Stats *UserStats

	// Current session state (not persisted)
	CurrentModule   ModuleID
	CurrentExercise *Exercise
	CurrentBoss     *BossExercise
	Exercises       []Exercise
	ExerciseIndex   int

	// Mode flags
	IsLessonMode   bool
	IsPracticeMode bool
	IsBossMode     bool

	// Streak and scoring
	CurrentStreak   int
	ComboMultiplier int
	SessionScore    int

	// Boss state
	BossLives int
	BossStep  int
	// IsBossDefeated is true once the player has defeated the current boss. It
	// is set only by RecordBossVictory. Losing a fight is not a defeat of the
	// boss: an exhausted fight is read from BossLives reaching zero, never from
	// this field.
	IsBossDefeated bool

	// Timing
	TimeElapsed time.Duration

	// bossStepDeadline is when the boss step on screen runs out of time. It is
	// stored rather than derived from the presentation time, because a retry
	// after expiry must get a fresh full window while ElapsedSeconds keeps
	// measuring the answer from the original presentation. presentBossStep and
	// RecordBossStepTimeout are its only writers.
	bossStepDeadline time.Time

	// bossStepBonus is the extra seconds the previous won step granted to the
	// current one. NextBossExercise assigns it per win, so it never accumulates
	// across steps.
	bossStepBonus int

	// now is the clock seam for answer timing: time.Now in production, replaced
	// through SetClock in tests so no test has to sleep to advance time.
	now func() time.Time

	// exerciseStartedAt is when the current exercise was presented on screen.
	// presentExercise is its only writer. It is deliberately not reset by a
	// wrong answer: ElapsedSeconds measures the time to solve the current
	// exercise, so a retry after a mistake is honestly slower.
	exerciseStartedAt time.Time
}

// NewGameState creates a new game state with fresh stats
func NewGameState() *GameState {
	return &GameState{
		Stats:           NewUserStats(),
		ComboMultiplier: 1,
		now:             time.Now,
	}
}

// NewGameStateWithStats creates a game state with existing stats
func NewGameStateWithStats(stats *UserStats) *GameState {
	if stats == nil {
		stats = NewUserStats()
	}
	return &GameState{
		Stats:           stats,
		ComboMultiplier: 1,
		now:             time.Now,
	}
}

// SetClock replaces the clock the state uses to measure answer time. It exists
// so tests can advance time without sleeping; production uses time.Now. Passing
// nil restores the default.
func (g *GameState) SetClock(now func() time.Time) {
	if now == nil {
		now = time.Now
	}
	g.now = now
}

// clockNow reads the injected clock, falling back to time.Now for a GameState
// that was not built through a constructor.
func (g *GameState) clockNow() time.Time {
	if g.now == nil {
		return time.Now()
	}
	return g.now()
}

// presentExercise is the single owner of the answer clock: it makes exercise
// the current one and starts its timer. Every path that puts an exercise in
// front of the player routes through it, so the elapsed time the scorer sees is
// measured from that one moment and the UI keeps no clock of its own.
func (g *GameState) presentExercise(exercise *Exercise) {
	g.CurrentExercise = exercise
	g.exerciseStartedAt = g.clockNow()
}

// ElapsedSeconds returns how many seconds have passed since the current
// exercise was presented, measured with the injected clock. An incorrect answer
// does not restart that measurement, so a retry after a mistake is honestly
// slower. It returns 0 when no exercise has been presented.
func (g *GameState) ElapsedSeconds() float64 {
	if g.exerciseStartedAt.IsZero() {
		return 0
	}
	return g.clockNow().Sub(g.exerciseStartedAt).Seconds()
}

// RemainingSeconds is how long the current exercise has left before its hint is
// due, measured through the same injected clock as ElapsedSeconds. It is 0 when
// the exercise declares no timeout, or once the deadline has passed, so a
// caller can treat a positive value as "the countdown is running".
func (g *GameState) RemainingSeconds() float64 {
	if g.CurrentExercise == nil || g.CurrentExercise.TimeoutSecs <= 0 {
		return 0
	}
	remaining := float64(g.CurrentExercise.TimeoutSecs) - g.ElapsedSeconds()
	if remaining < 0 {
		return 0
	}
	return remaining
}

// HintDue reports whether the current exercise declares a timeout and enough
// time has passed for its hint to be shown without the user asking. It is a
// hint, not a failure: a due hint never ends or fails the exercise, so the
// screen stays open and answerable.
func (g *GameState) HintDue() bool {
	if g.CurrentExercise == nil || g.CurrentExercise.TimeoutSecs <= 0 {
		return false
	}
	return g.ElapsedSeconds() >= float64(g.CurrentExercise.TimeoutSecs)
}

// BossStepTimeLimit is the effective number of seconds the boss step on screen
// allows the player: the step's own TimeLimit plus the bonus the previous won
// step granted. It is 0 when there is no boss step or the step declares no
// limit, so a caller can treat it as "this step has a clock".
func (g *GameState) BossStepTimeLimit() int {
	if g.CurrentBoss == nil || g.BossStep < 0 || g.BossStep >= len(g.CurrentBoss.Steps) {
		return 0
	}
	limit := g.CurrentBoss.Steps[g.BossStep].TimeLimit
	if limit <= 0 {
		return 0
	}
	return limit + g.bossStepBonus
}

// BossStepSecondsLeft is how long the current boss step has left before the
// clock charges a life, measured through the same injected clock as
// ElapsedSeconds. It is 0 when the fight is not running or the step has no
// deadline, so a positive value means the boss countdown is live.
func (g *GameState) BossStepSecondsLeft() float64 {
	if !g.IsBossMode || g.bossStepDeadline.IsZero() {
		return 0
	}
	remaining := g.bossStepDeadline.Sub(g.clockNow()).Seconds()
	if remaining < 0 {
		return 0
	}
	return remaining
}

// BossDeadlinePassed reports whether the boss step on screen has been left
// unanswered past its deadline. It is false for a fight that is not running and
// for a step that declares no TimeLimit.
func (g *GameState) BossDeadlinePassed() bool {
	if !g.IsBossMode || g.bossStepDeadline.IsZero() {
		return false
	}
	return !g.clockNow().Before(g.bossStepDeadline)
}

// presentBossStep is presentExercise for a boss step: it puts the step the fight
// is on in front of the player and arms that step's deadline. Every path that
// makes a boss step current routes through it, so a deadline is never left over
// from the step before.
func (g *GameState) presentBossStep() {
	g.presentExercise(&g.CurrentBoss.Steps[g.BossStep].Exercise)
	g.startBossDeadline()
}

// startBossDeadline arms the current boss step's deadline from its effective
// limit. A step with no positive TimeLimit gets no clock at all, so it can never
// expire; the deadline field is cleared so no earlier window lingers.
func (g *GameState) startBossDeadline() {
	limit := g.BossStepTimeLimit()
	if limit <= 0 {
		g.bossStepDeadline = time.Time{}
		return
	}
	g.bossStepDeadline = g.clockNow().Add(time.Duration(limit) * time.Second)
}

// StartLesson starts lesson mode for a module
func (g *GameState) StartLesson(module ModuleID) {
	g.CurrentModule = module
	g.IsLessonMode = true
	g.IsPracticeMode = false
	g.IsBossMode = false
	g.Exercises = GetLessons(module)
	g.ExerciseIndex = 0
	g.CurrentStreak = 0
	g.ComboMultiplier = 1

	if len(g.Exercises) > 0 {
		g.presentExercise(&g.Exercises[0])
	}

	// Initialize lesson total in stats
	progress := g.Stats.GetModuleProgress(module)
	progress.LessonsTotal = len(g.Exercises)
}

// StartPractice starts practice mode for a module using intelligent selection
func (g *GameState) StartPractice(module ModuleID) {
	g.CurrentModule = module
	g.IsLessonMode = false
	g.IsPracticeMode = true
	g.IsBossMode = false
	g.Exercises = nil // Not used in intelligent practice mode
	g.ExerciseIndex = 0
	g.CurrentStreak = 0
	g.ComboMultiplier = 1

	// Use weighted random selection for intelligent practice
	progress := g.Stats.GetModuleProgress(module)
	exercise := SelectRandomPracticeExercise(module, progress)
	g.presentExercise(exercise)
}

// SetPracticeExercise sets a specific exercise for practice mode
func (g *GameState) SetPracticeExercise(exercise *Exercise) {
	g.presentExercise(exercise)
}

// NextPracticeExercise selects the next random exercise for practice
// Returns false if practice is complete (all mastered)
func (g *GameState) NextPracticeExercise() bool {
	if !g.IsPracticeMode {
		return false
	}

	progress := g.Stats.GetModuleProgress(g.CurrentModule)
	exercise := SelectRandomPracticeExercise(g.CurrentModule, progress)

	if exercise == nil {
		// All exercises mastered - practice complete!
		return false
	}

	g.presentExercise(exercise)
	return true
}

// StartBoss starts boss fight for a module
func (g *GameState) StartBoss(module ModuleID) {
	g.CurrentModule = module
	g.IsLessonMode = false
	g.IsPracticeMode = false
	g.IsBossMode = true
	g.CurrentBoss = GetBoss(module)
	g.BossStep = 0
	g.bossStepBonus = 0
	g.bossStepDeadline = time.Time{}
	g.CurrentStreak = 0
	g.ComboMultiplier = 1
	g.IsBossDefeated = false

	if g.CurrentBoss != nil {
		g.BossLives = g.CurrentBoss.Lives
		if len(g.CurrentBoss.Steps) > 0 {
			g.presentBossStep()
		}
	}
}

// RecordCorrectAnswer records a correct answer and updates stats. timeSeconds
// is the time taken to solve the current exercise, normally the value from
// ElapsedSeconds.
func (g *GameState) RecordCorrectAnswer(timeSeconds float64, isOptimal bool) {
	g.CurrentStreak++
	if g.CurrentStreak > g.Stats.BestStreak {
		g.Stats.BestStreak = g.CurrentStreak
	}
	g.Stats.CurrentStreak = g.CurrentStreak

	// Calculate and add points
	points := CalculatePoints(g.CurrentExercise, timeSeconds, isOptimal, g.ComboMultiplier)
	g.SessionScore += points
	g.Stats.TotalScore += points

	// TotalTime is the time spent solving exercises, so it grows only here. An
	// incorrect answer does not restart the clock (see ElapsedSeconds), so the
	// solve time recorded here already covers the recovery from any mistake and
	// adding the wrong answer's time as well would count those seconds twice.
	g.Stats.TotalTime += time.Duration(timeSeconds * float64(time.Second))
	g.Stats.LastPlayed = g.clockNow()

	// Practice attempt accounting lives in ModuleProgress.RecordPracticeResult,
	// the single owner of PracticeAttempts/PracticeCorrect and per-exercise
	// mastery. Counting it here too double-counted every practice submission.

	// Update lesson progress
	if g.IsLessonMode {
		progress := g.Stats.GetModuleProgress(g.CurrentModule)
		if g.ExerciseIndex+1 > progress.LessonsCompleted {
			progress.LessonsCompleted = g.ExerciseIndex + 1
		}
	}

	// Boss progress
	if g.IsBossMode {
		g.recordBossAttempt()
	}
}

// recordBossAttempt counts one boss step answered. ModuleProgress.BossAttempts
// is the single counter for boss attempts, owned by the two answer recorders;
// the UI and RecordBossVictory must not increment it, or the winning step is
// counted twice.
func (g *GameState) recordBossAttempt() {
	g.Stats.GetModuleProgress(g.CurrentModule).BossAttempts++
}

// RecordIncorrectAnswer records an incorrect answer
func (g *GameState) RecordIncorrectAnswer() {
	g.CurrentStreak = 0
	g.Stats.CurrentStreak = 0
	g.ComboMultiplier = 1

	// LastPlayed marks the last recorded answer, so a wrong one updates it too.
	// TotalTime stays untouched: the solve time recorded by the later correct
	// answer already covers the seconds spent on this mistake.
	g.Stats.LastPlayed = g.clockNow()

	// Practice attempt accounting lives in ModuleProgress.RecordPracticeResult,
	// the single owner of PracticeAttempts/PracticeCorrect.

	// Boss mode: a lost life is one failed boss step.
	if g.IsBossMode {
		g.recordBossAttempt()
		g.BossLives--
		if g.BossLives < 0 {
			g.BossLives = 0
		}
	}
}

// RecordBossStepTimeout charges the clock's own failure: the boss step on screen
// ran out of time. It spends one life through RecordIncorrectAnswer, the same
// canonical recorder a wrong answer uses, so the streak, the attempt and the
// life cost are identical either way, and it then re-arms the deadline so the
// retry gets a full window. Re-arming is what makes the 100ms tick idempotent:
// after this call the deadline is a whole limit in the future, so a burst of
// ticks past one deadline charges exactly one life instead of draining the
// fight.
func (g *GameState) RecordBossStepTimeout() {
	if !g.IsBossMode {
		return
	}
	g.RecordIncorrectAnswer()
	g.startBossDeadline()
}

// NextExercise advances to the next exercise
func (g *GameState) NextExercise() bool {
	if g.IsBossMode {
		if g.ComboMultiplier < 4 {
			g.ComboMultiplier++
		}
		return g.NextBossExercise()
	}

	g.ExerciseIndex++

	if g.ExerciseIndex >= len(g.Exercises) {
		// Complete lessons if in lesson mode
		if g.IsLessonMode {
			progress := g.Stats.GetModuleProgress(g.CurrentModule)
			progress.LessonsCompleted = progress.LessonsTotal
		}
		return false
	}

	g.presentExercise(&g.Exercises[g.ExerciseIndex])
	return true
}

// NextBossExercise advances the boss fight past the step that was just answered
// and presents the next one, starting its timer. It reports whether another step
// is now on screen; false means the answered step was the last and the fight is
// over. It deliberately leaves ComboMultiplier alone: the boss advance never
// raised it, and this path only makes the answer clock real.
func (g *GameState) NextBossExercise() bool {
	g.BossStep++
	if g.CurrentBoss == nil || g.BossStep >= len(g.CurrentBoss.Steps) {
		return false
	}
	// A won step grants the fight's BonusTime to the step that follows it. It is
	// assigned rather than added, so the bonus is per win and never accumulates
	// across steps; a lost step never reaches this path, so it grants nothing.
	g.bossStepBonus = g.CurrentBoss.BonusTime
	g.presentBossStep()
	return true
}

// RecordBossVictory records defeating a boss
func (g *GameState) RecordBossVictory() {
	// The player defeated the boss this session.
	g.IsBossDefeated = true

	// Add to defeated list if not already
	alreadyDefeated := false
	for _, boss := range g.Stats.BossesDefeated {
		if boss == g.CurrentModule {
			alreadyDefeated = true
			break
		}
	}
	if !alreadyDefeated {
		g.Stats.BossesDefeated = append(g.Stats.BossesDefeated, g.CurrentModule)
	}

	// Update module progress. BossAttempts is owned by the answer recorders, so
	// the victory bonus must not count the winning step again.
	progress := g.Stats.GetModuleProgress(g.CurrentModule)
	progress.BossDefeated = true

	if progress.BossBestTime == 0 || g.TimeElapsed < progress.BossBestTime {
		progress.BossBestTime = g.TimeElapsed
	}
	progress.BossLivesLeft = g.BossLives

	// Boss victory bonus
	g.Stats.TotalScore += 500
	g.SessionScore += 500
}

// Reset resets the game state for a new session
func (g *GameState) Reset() {
	g.CurrentModule = ""
	g.CurrentExercise = nil
	g.CurrentBoss = nil
	g.Exercises = nil
	g.ExerciseIndex = 0

	g.IsLessonMode = false
	g.IsPracticeMode = false
	g.IsBossMode = false

	g.CurrentStreak = 0
	g.ComboMultiplier = 1
	g.SessionScore = 0

	g.BossLives = 0
	g.BossStep = 0
	g.bossStepBonus = 0
	g.bossStepDeadline = time.Time{}
	g.IsBossDefeated = false

	g.TimeElapsed = 0
	g.exerciseStartedAt = time.Time{}
}
