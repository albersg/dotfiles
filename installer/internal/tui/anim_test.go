package tui

import (
	"os"
	"strings"
	"testing"
	"time"
)

// TestAnimationGateTurnsOffOnTheEnvironmentAndTheStream pins the whole gate: a
// non-terminal stdout (here a regular file) turns animation off by itself,
// DOTFILES_ANIM=0 and TERM=dumb each turn it off, and a character device with no
// override is the only combination that leaves it on. /dev/null stands in for a
// terminal because it is the one character device a test can rely on without
// opening the user's tty.
func TestAnimationGateTurnsOffOnTheEnvironmentAndTheStream(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "anim-gate")
	if err != nil {
		t.Fatalf("test setup: %v", err)
	}
	defer file.Close()

	if animationGate(file) {
		t.Error("a regular file was treated as a terminal: a redirected run would animate")
	}
	if animationGate(nil) {
		t.Error("a nil stream was treated as a terminal")
	}

	term, err := os.Open(os.DevNull)
	if err != nil {
		t.Skipf("no %s to stand in for a terminal: %v", os.DevNull, err)
	}
	defer term.Close()
	if !isCharDevice(term) {
		t.Skipf("%s is not a character device on this host, so it cannot stand in for a terminal", os.DevNull)
	}

	t.Setenv(envAnim, "")
	t.Setenv(envTerm, "")
	if !animationGate(term) {
		t.Error("a character device with no override did not animate")
	}

	t.Setenv(envAnim, "0")
	if animationGate(term) {
		t.Error("DOTFILES_ANIM=0 did not turn animation off")
	}

	t.Setenv(envAnim, "")
	t.Setenv(envTerm, "dumb")
	if animationGate(term) {
		t.Error("TERM=dumb did not turn animation off")
	}
}

// TestAnimTickIsArmedAndCountedOnlyWhileAnimating pins the scheduling rule both
// ways: an animation-off model arms no tick, re-arms none when one arrives, and
// keeps the counter at zero; an animation-on model arms the tick, advances the
// counter once per tick and re-arms. A gate forced off after the tick was armed
// stops the clock rather than letting it run on.
func TestAnimTickIsArmedAndCountedOnlyWhileAnimating(t *testing.T) {
	off := Model{Animating: false}
	if cmd := off.animTickCmdFor(); cmd != nil {
		t.Error("an animation-off model armed the frame tick")
	}
	next, cmd := off.Update(animTickMsg{})
	got := next.(Model)
	if got.AnimTick != 0 {
		t.Errorf("AnimTick = %d while animation is off, want 0", got.AnimTick)
	}
	if cmd != nil {
		t.Error("an animation-off model re-armed the frame tick")
	}

	on := Model{Animating: true}
	if cmd := on.animTickCmdFor(); cmd == nil {
		t.Error("an animation-on model did not arm the frame tick")
	}
	next, cmd = on.Update(animTickMsg{})
	got = next.(Model)
	if got.AnimTick != 1 {
		t.Errorf("AnimTick = %d after one tick, want 1", got.AnimTick)
	}
	if cmd == nil {
		t.Error("the frame tick did not re-arm while animation is on")
	}

	stopped := Model{Animating: false, AnimTick: 5}
	next, cmd = stopped.Update(animTickMsg{})
	got = next.(Model)
	if got.AnimTick != 5 || cmd != nil {
		t.Errorf("a gate forced off after arming = (AnimTick %d, cmd %v), want (5, nil)", got.AnimTick, cmd)
	}
}

// TestAnimTickCmdDeliversTheSlowTick pins the message identity: the trainer's
// deadline clock ticks on tickMsg, and the animation clock must not be confused
// with it.
func TestAnimTickCmdDeliversTheSlowTick(t *testing.T) {
	msg := animTickCmd()()
	if _, ok := msg.(animTickMsg); !ok {
		t.Errorf("animTickCmd delivered %T, want animTickMsg", msg)
	}
}

// TestTheAnimationRateIsNamedOnceAndEverythingDerivesFromIt pins the single-rate
// rule: the frame interval, the tip's hold and the companion's two clocks are all
// derived from animTicksPerSecond, so no cadence can keep an old value while the
// rate moves. The durations themselves are stated too, because they are the
// numbers a reader reasons about and a change to one of them is a design change
// rather than a mechanical one.
func TestTheAnimationRateIsNamedOnceAndEverythingDerivesFromIt(t *testing.T) {
	if got := animTickInterval * animTicksPerSecond; got != time.Second {
		t.Errorf("%d frames of %v make %v a second, want exactly one second",
			animTicksPerSecond, animTickInterval, got)
	}
	if animTicksPerSecond <= 1 {
		t.Errorf("the animation rate is %d frames a second, which is not an animation", animTicksPerSecond)
	}

	if got, want := ticksPerTip, animTicksPerSecond*10; got != want {
		t.Errorf("a tip holds for %d frames, want %d: ten seconds at %d frames a second",
			got, want, animTicksPerSecond)
	}
	if got, want := companionSleepTicks, companionSleepSeconds*animTicksPerSecond; got != want {
		t.Errorf("the sleep is %d frames, want %d: %d seconds at %d frames a second",
			got, want, companionSleepSeconds, animTicksPerSecond)
	}
	if got, want := companionPleasedTicks, companionPleasedSeconds*animTicksPerSecond; got != want {
		t.Errorf("the celebration is %d frames, want %d: %d second at %d frames a second",
			got, want, companionPleasedSeconds, animTicksPerSecond)
	}

	// The durations the frames are derived from: a minute asleep, about a
	// second pleased. The nap is stated as the named duration it is rather than
	// as a second copy of the number, so the design lives once -- in the named
	// duration -- and a rate change must move the frame counts and not these.
	if want := int(time.Minute / time.Second); companionSleepSeconds != want {
		t.Errorf("the sleep is %d seconds, want %d: a whole minute at %d frames a second",
			companionSleepSeconds, want, animTicksPerSecond)
	}
	if companionPleasedSeconds != 1 {
		t.Errorf("the celebration is %d seconds, want 1", companionPleasedSeconds)
	}
}

// TestTheTrainersDeadlineClockDoesNotMoveTheAnimation pins that the two clocks
// stay out of each other's way: the trainer's 100ms tick expires hints and boss
// steps and is not the animation clock, so fifty of its ticks -- five seconds of
// the trainer's time -- advance no animation frame and change no byte on a screen
// the animation owns. The frame tick is what moves AnimTick, and therefore the
// tip and the companion, and nothing else is allowed to.
func TestTheTrainersDeadlineClockDoesNotMoveTheAnimation(t *testing.T) {
	m := NewModel()
	isolateGoldenTest(t, &m)
	m.Screen = ScreenMainMenu
	m.Width, m.Height = 160, 50
	m.Animating = true

	before := m.View()
	tip := tipIndex(m)
	frame := m.AnimTick
	pos := m.CompanionPos
	idle := m.CompanionIdle

	for i := 0; i < 50; i++ {
		next, _ := m.Update(tickMsg(time.Time{}))
		got, ok := next.(Model)
		if !ok {
			t.Fatalf("Update returned %T, want Model", next)
		}
		m = got
	}

	if m.AnimTick != frame {
		t.Errorf("the trainer's clock advanced AnimTick from %d to %d", frame, m.AnimTick)
	}
	if got := tipIndex(m); got != tip {
		t.Errorf("the trainer's clock rotated the tip from %d to %d", tip, got)
	}
	if m.CompanionPos != pos {
		t.Errorf("the trainer's clock moved the companion from %d to %d", pos, m.CompanionPos)
	}
	if m.CompanionIdle != idle {
		t.Errorf("the trainer's clock aged the idle stretch from %d to %d", idle, m.CompanionIdle)
	}
	if got := m.View(); got != before {
		t.Errorf("the trainer's clock changed the screen:\nbefore:\n%s\nafter:\n%s", before, got)
	}
}

// TestScreensRenderIdenticallyWithAnimationOnAndOffExceptTheCompanionRow pins
// that the gate changes scheduling and, now that the companion exists, only the
// companion's own rows. The previous slice's version of this guard asserted that
// every framed screen rendered the same bytes with animation on and with it off,
// because then no screen showed the counter at all -- "until the slices that place
// a tip or a companion", as its own comment said. The companion is that slice, so
// the guard is restated for the new truth rather than dropped: every row a body
// needs is still byte-identical, the frame neither gains nor loses a row (a
// companion that cost it one would push a body row off the screen the moment
// animation was turned on), and the rows that may differ are the last rows the
// body did not need, immediately above the footer rule. The bound is not a number
// written here: the companion's height is read off the render, so the tail this
// guard allows is the summary's rows plus the art that screen's ladder actually
// chose, and a fourth height added tomorrow cannot slip past it.
// TestCompanionTicksChangeOnlyItsOwnRows pins the same budget on the tick axis.
func TestScreensRenderIdenticallyWithAnimationOnAndOffExceptTheCompanionRow(t *testing.T) {
	screens := []struct {
		screen Screen
		// framed is true for the screens that draw inside the installer's frame,
		// whose body ends with a rule above the footer. The trainer's screens
		// compose their own rows, so no rule follows their companion row.
		framed bool
	}{
		{ScreenWelcome, true},
		{ScreenMainMenu, true},
		{ScreenOSSelect, true},
		{ScreenTerminalSelect, true},
		{ScreenFontSelect, true},
		{ScreenShellSelect, true},
		{ScreenWMSelect, true},
		{ScreenNvimSelect, true},
		{ScreenGhosttyWarning, true},
		{ScreenLearnTerminals, true},
		{ScreenKeymaps, true},
		{ScreenInstalling, true},
		{ScreenComplete, true},
		{ScreenError, true},
		{ScreenBackupConfirm, true},
		{ScreenRestoreBackup, true},
		{ScreenTrainerMenu, false},
		{ScreenTrainerLesson, false},
	}
	sizes := []struct {
		name          string
		width, height int
	}{
		{"80x24", 80, 24},
		{"100x24", 100, 24},
		{"160x50", 160, 50},
	}

	for _, size := range sizes {
		for _, entry := range screens {
			size, entry := size, entry
			t.Run(size.name, func(t *testing.T) {
				base := NewModel()
				isolateGoldenTest(t, &base)
				base.Screen = entry.screen
				base.Width, base.Height = size.width, size.height

				base.Animating, base.AnimTick = false, 0
				still := base.View()
				base.Animating, base.AnimTick = true, 7
				moving := base.View()

				stillRows := strings.Split(still, "\n")
				movingRows := strings.Split(moving, "\n")
				if len(stillRows) != len(movingRows) {
					t.Fatalf("screen %v renders %d rows with animation off and %d with it on at %s",
						entry.screen, len(stillRows), len(movingRows), size.name)
				}

				var changed []int
				for i := range stillRows {
					if stillRows[i] != movingRows[i] {
						changed = append(changed, i)
					}
				}
				if len(changed) == 0 {
					// A screen whose body fills its frame has no spare row for a
					// companion, and nothing else on any screen reads the counter.
					return
				}

				if entry.framed {
					// The frame's last rule is the row above its footer; above it
					// the body ends. The gate may fill the decorated tail the body
					// did not need -- the summary and the companion -- and nothing
					// above it: not one row of the body. How tall that tail is comes
					// from the render, not from this test: the rows immediately above
					// the rule that carry art are the height the ladder chose for this
					// screen and size.
					lastRule := -1
					for i := len(movingRows) - 1; i >= 0; i-- {
						if isRuleRow(movingRows[i]) {
							lastRule = i
							break
						}
					}
					if lastRule < 0 {
						t.Fatalf("screen %v has no footer rule to place the companion above:\n%s",
							entry.screen, moving)
					}
					artRows := companionArtRun(movingRows, lastRule)
					if artRows == 0 {
						// Something changed with animation on but no creature is on screen.
						// That is a defect -- a row changing with nothing drawing is
						// exactly the cost this guard exists to bound -- and not a pass.
						t.Fatalf("screen %v changes %d rows with animation on at %s without drawing the companion above the footer rule:\nstill:\n%s\nmoving:\n%s",
							entry.screen, len(changed), size.name, still, moving)
					}
					tail := rotatorMaxRows + artRows // the summary's rows plus the art this screen drew
					if len(changed) > tail {
						t.Fatalf("screen %v changes %d rows with animation on at %s, want at most the decorated tail's %d:\nstill:\n%s\nmoving:\n%s",
							entry.screen, len(changed), size.name, tail, still, moving)
					}
					for _, i := range changed {
						if i < lastRule-tail {
							t.Errorf("screen %v changes a body row at %s: %q became %q",
								entry.screen, size.name, plainRow(stillRows[i]), plainRow(movingRows[i]))
						}
					}
					if row := movingRows[lastRule-1]; !companionRowHasArt(row) {
						t.Errorf("screen %v does not put the companion on the last row the body did not need at %s: %q",
							entry.screen, size.name, plainRow(row))
					}
					return
				}

				// The trainer's screens compose their own rows, so the companion is
				// drawn in a block the trainer reserves above its legend. The tick may
				// redraw the creature's own rows and nothing else - the block is as
				// tall as the rung the terminal calls for, so this counts the art
				// the render actually drew rather than assuming the one-row animal
				// the trainer used to show. A row that carries content in both views
				// is a row the gate must not have touched.
				art := 0
				for _, row := range movingRows {
					if companionRowHasArt(row) {
						art++
					}
				}
				if art == 0 {
					t.Fatalf("screen %v changes %d rows with animation on at %s without drawing the companion:\nstill:\n%s\nmoving:\n%s",
						entry.screen, len(changed), size.name, still, moving)
				}
				if len(changed) > art {
					t.Fatalf("screen %v changes %d rows with animation on at %s, want at most the companion's %d art rows:\nstill:\n%s\nmoving:\n%s",
						entry.screen, len(changed), size.name, art, still, moving)
				}
				for _, row := range changed {
					if strings.TrimSpace(plainRow(stillRows[row])) != "" &&
						strings.TrimSpace(plainRow(movingRows[row])) != "" &&
						!companionRowHasArt(movingRows[row]) {
						t.Errorf("screen %v changes a row that carries content either way at %s: %q became %q",
							entry.screen, size.name, plainRow(stillRows[row]), plainRow(movingRows[row]))
					}
				}
			})
		}
	}
}
