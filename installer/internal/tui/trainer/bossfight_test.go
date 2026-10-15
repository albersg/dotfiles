package trainer

import (
	"strings"
	"testing"
	"time"
)

// fightClock is a clock a test moves by hand, so a fight's duration is a value rather
// than whatever the suite happened to take.
type fightClock struct{ now time.Time }

func (c *fightClock) read() time.Time         { return c.now }
func (c *fightClock) advance(d time.Duration) { c.now = c.now.Add(d) }
func newFightClock() *fightClock              { return &fightClock{now: time.Unix(1_700_000_000, 0)} }

// TestBossFightMeasuresItsOwnDuration pins S10: the fight has a clock of its own,
// measured from the moment the first step is presented. Before this rule TimeElapsed
// was never written and BossBestTime could only ever stay at zero.
func TestBossFightMeasuresItsOwnDuration(t *testing.T) {
	clock := newFightClock()
	state := NewGameState()
	state.SetClock(clock.read)
	state.StartBoss(ModuleHorizontal)

	if got := state.BossFightElapsed(); got != 0 {
		t.Errorf("a fight that just started has lasted %s, want 0", got)
	}

	clock.advance(30 * time.Second)
	if got, want := state.BossFightElapsed(), 30*time.Second; got != want {
		t.Errorf("the fight has lasted %s, want %s", got, want)
	}

	// A step's own countdown is a different clock: the bonus time a won step grants
	// must not be mistaken for time the fight did not take.
	if got, want := state.BossStepSecondsLeft(), float64(state.CurrentBoss.Steps[0].TimeLimit); got > want {
		t.Errorf("the step has %.0fs left after 30s of fight, more than its own %0.fs limit", got, want)
	}
}

// TestBossVictoryRecordsTheFightTimeAndTheBest pins the recording half of S10: the
// fight's own duration is what the module keeps as its best, and a slower fight never
// replaces a faster one.
func TestBossVictoryRecordsTheFightTimeAndTheBest(t *testing.T) {
	stats := NewUserStats()
	clock := newFightClock()

	win := func(t *testing.T, seconds int) *GameState {
		t.Helper()
		state := NewGameStateWithStats(stats)
		state.SetClock(clock.read)
		state.StartBoss(ModuleHorizontal)
		clock.advance(time.Duration(seconds) * time.Second)
		state.RecordBossVictory()
		return state
	}

	first := win(t, 40)
	progress := stats.GetModuleProgress(ModuleHorizontal)
	if got, want := progress.BossBestTime, 40*time.Second; got != want {
		t.Fatalf("the first victory recorded a best of %s, want %s", got, want)
	}
	if got, want := first.TimeElapsed, 40*time.Second; got != want {
		t.Errorf("the finished fight reports %s, want the %s it lasted", got, want)
	}

	win(t, 90)
	if got, want := progress.BossBestTime, 40*time.Second; got != want {
		t.Errorf("a slower fight replaced the best with %s, want it left at %s", got, want)
	}

	third := win(t, 20)
	if got, want := progress.BossBestTime, 20*time.Second; got != want {
		t.Errorf("a faster fight recorded a best of %s, want %s", got, want)
	}
	if got, want := third.TimeElapsed, 20*time.Second; got != want {
		t.Errorf("the fastest fight reports %s, want %s", got, want)
	}
}

// TestBossDefeatLeavesTheBestAlone pins the other side: losing a fight is not a defeat
// of the module's record, and the lost fight still knows how long it lasted, which is
// what the result screen shows.
func TestBossDefeatLeavesTheBestAlone(t *testing.T) {
	stats := NewUserStats()
	clock := newFightClock()

	won := NewGameStateWithStats(stats)
	won.SetClock(clock.read)
	won.StartBoss(ModuleHorizontal)
	clock.advance(40 * time.Second)
	won.RecordBossVictory()

	lost := NewGameStateWithStats(stats)
	lost.SetClock(clock.read)
	lost.StartBoss(ModuleHorizontal)
	clock.advance(7 * time.Second)
	for lost.BossLives > 0 {
		lost.RecordIncorrectAnswer()
	}

	if got, want := stats.GetModuleProgress(ModuleHorizontal).BossBestTime, 40*time.Second; got != want {
		t.Errorf("a lost fight moved the best time to %s, want it left at %s", got, want)
	}
	if got, want := lost.BossFightElapsed(), 7*time.Second; got != want {
		t.Errorf("the lost fight reports %s, want the %s it lasted", got, want)
	}
}

// TestBossStepNudgePrefersTheAuthoredHint pins S11's order: a step that ships a hint
// shows it, and the fallback exists only for a step that ships none.
func TestBossStepNudgePrefersTheAuthoredHint(t *testing.T) {
	authored := &Exercise{Optimal: "viw", Hint: "An inner text object takes the word."}
	if got := BossStepNudge(authored); got != authored.Hint {
		t.Errorf("the nudge is %q, want the step's own hint %q", got, authored.Hint)
	}
}

// TestBossStepNudgeNeverPrintsTheWholeAnswer pins the fallback: it gets the player
// started and stops there. A single-keystroke answer has no partial nudge to give --
// its first keystroke is its solution -- so the fallback says nothing rather than
// handing the step over.
func TestBossStepNudgeNeverPrintsTheWholeAnswer(t *testing.T) {
	nudge := BossStepNudge(&Exercise{Optimal: "viw"})
	if nudge == "" {
		t.Fatal("a step without a hint got no nudge, so Tab is dead on it")
	}
	if strings.Contains(nudge, "viw") {
		t.Errorf("the fallback prints the whole answer: %q", nudge)
	}
	if !strings.Contains(nudge, "vi") {
		t.Errorf("the fallback does not get the player started: %q", nudge)
	}

	if got := BossStepNudge(&Exercise{Optimal: "qa"}); strings.Contains(got, "qa") {
		t.Errorf("a two-keystroke answer is printed whole: %q", got)
	}
	if got := BossStepNudge(&Exercise{Optimal: "$"}); got != "" {
		t.Errorf("a one-keystroke answer got the nudge %q, which is the answer itself", got)
	}
	if got := BossStepNudge(nil); got != "" {
		t.Errorf("a nil exercise got the nudge %q, want nothing", got)
	}
}
