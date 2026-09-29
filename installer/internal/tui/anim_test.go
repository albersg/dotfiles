package tui

import (
	"os"
	"testing"
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
		t.Error("an animation-off model armed the slow tick")
	}
	next, cmd := off.Update(animTickMsg{})
	got := next.(Model)
	if got.AnimTick != 0 {
		t.Errorf("AnimTick = %d while animation is off, want 0", got.AnimTick)
	}
	if cmd != nil {
		t.Error("an animation-off model re-armed the slow tick")
	}

	on := Model{Animating: true}
	if cmd := on.animTickCmdFor(); cmd == nil {
		t.Error("an animation-on model did not arm the slow tick")
	}
	next, cmd = on.Update(animTickMsg{})
	got = next.(Model)
	if got.AnimTick != 1 {
		t.Errorf("AnimTick = %d after one tick, want 1", got.AnimTick)
	}
	if cmd == nil {
		t.Error("the slow tick did not re-arm while animation is on")
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

// TestExistingScreensRenderIdenticallyWithAnimationOnAndOff pins that the gate
// changes scheduling and nothing else: every framed screen renders the same bytes
// with animation on and with it off, because the counter it drives is not shown
// anywhere until the slices that place a tip or a companion.
func TestExistingScreensRenderIdenticallyWithAnimationOnAndOff(t *testing.T) {
	screens := []Screen{
		ScreenWelcome,
		ScreenMainMenu,
		ScreenOSSelect,
		ScreenTerminalSelect,
		ScreenFontSelect,
		ScreenShellSelect,
		ScreenWMSelect,
		ScreenNvimSelect,
		ScreenGhosttyWarning,
		ScreenLearnTerminals,
		ScreenKeymaps,
		ScreenInstalling,
		ScreenComplete,
		ScreenError,
		ScreenBackupConfirm,
		ScreenRestoreBackup,
		ScreenTrainerMenu,
		ScreenTrainerLesson,
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
		for _, screen := range screens {
			size, screen := size, screen
			t.Run(size.name, func(t *testing.T) {
				base := NewModel()
				isolateGoldenTest(t, &base)
				base.Screen = screen
				base.Width, base.Height = size.width, size.height

				base.Animating, base.AnimTick = false, 0
				still := base.View()
				base.Animating, base.AnimTick = true, 7
				moving := base.View()

				if still != moving {
					t.Errorf("screen %v renders differently with animation off and on at %s:\nstill:\n%s\nmoving:\n%s",
						screen, size.name, still, moving)
				}
			})
		}
	}
}
