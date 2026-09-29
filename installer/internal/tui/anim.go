package tui

import (
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// The animation gate and its frame tick.
//
// Two things in the installer are time-driven but not information: the tip the
// panel rotates and the companion that arrives after it. Neither is allowed to
// schedule work when the run cannot show it, so the decision of whether to
// animate is made once, stored on the model as Animating, and read from there:
//
//   - DOTFILES_ANIM=0 turns animation off, and so does the --no-anim flag (which
//     sets the same variable before the model is built);
//   - a non-terminal stdout turns it off, because an animation nobody watches is
//     a stream of escapes into a file;
//   - TERM=dumb turns it off, because that terminal has no cursor addressing to
//     animate with.
//
// With animation off no tick is scheduled at all and the counter stays at zero,
// so a headless run and a dumb terminal do no periodic work and produce the same
// bytes a static render would. The gate is a field on the model rather than a
// package global so a test can force either side without touching the
// environment, and the counter it drives is a plain int so a snapshot can pin a
// frame instead of flaking on the clock.
//
// There are two clocks, and they do not fight because they drive different
// things. tickMsg is the trainer's 100ms deadline clock: it expires the exercise
// hint and the boss step, it is not decoration, and it keeps running whether or
// not the run animates, so gating it on the animation switch would silently
// disable a game mechanic. animTickMsg below is the frame tick the gate owns: it
// advances AnimTick, and AnimTick is the only thing the tip rotation and the
// companion read. Nothing in the trainer's clock advances AnimTick, and nothing
// in the frame tick warns a deadline; neither one can move the other's screen.
const (
	// animTicksPerSecond is the animation frame rate, named once so every cadence
	// below is derived from it and cannot drift: the interval, the tip's
	// ticks-per-tip and the companion's durations are all expressed through this
	// number, and changing it moves all of them together.
	animTicksPerSecond = 8

	// animTickInterval is how often the frame tick fires: one frame, derived from
	// the rate rather than written as a second, so the interval cannot disagree
	// with the rate that names it. One row of ASCII at this rate is the cost the
	// design bounded: the renderer returns early when a frame changes nothing, and
	// the companion changes one row when it does move.
	animTickInterval = time.Second / animTicksPerSecond

	// envAnim names the environment switch that turns animation off.
	envAnim = "DOTFILES_ANIM"

	// envTerm is the environment variable whose "dumb" value means the terminal
	// cannot address a cursor well enough to animate on.
	envTerm = "TERM"

	// envMouse names the environment switch that turns the pointer off. The
	// creature's gaze asks the terminal for mouse motion, and a terminal in mouse
	// reporting mode gives its own selection up to the application unless the user
	// holds the bypass key, so the pointer is a switch of its own beside the
	// animation one rather than something attached to it.
	envMouse = "DOTFILES_MOUSE"

	// envTermux is the variable a Termux session sets for itself. It is read rather
	// than guessing from TERM because it is the one the terminal exports, and a
	// touch screen has no pointer to hover with: Termux turns a finger drag into a
	// wheel report, so the gaze would cost the user the swipe and give nothing back.
	envTermux = "TERMUX_VERSION"

	// envMouseForce is the value of envMouse that overrides that Termux default. A
	// wired mouse on a Termux session with an external display is a real case, so
	// the Termux rule is a default and not a refusal.
	envMouseForce = "1"

	// envSprite names the environment switch that turns the shaded sprite off. It is
	// its own switch because the sprite is the one part of the creature that a
	// terminal cannot be asked to draw: a run on a terminal that reports true colour
	// but renders block glyphs badly can keep the glyph cat with this and nothing
	// else.
	envSprite = "DOTFILES_SPRITE"
)

// animTickMsg is the frame tick: one wakeup per animTickInterval while animation
// is on. It is a distinct message from tickMsg so the trainer's deadline clock
// and the animation clock cannot be confused for one another.
type animTickMsg struct{}

// animTickCmd schedules the next frame tick.
func animTickCmd() tea.Cmd {
	return tea.Tick(animTickInterval, func(time.Time) tea.Msg {
		return animTickMsg{}
	})
}

// animTickCmdFor is the scheduling decision in one place: the command a model
// arms, or nil when animation is off. Init and the tick handler both read it, so
// the two cannot disagree about whether the clock is running, and a test can
// read it instead of guessing from a batched command's identity.
func (m Model) animTickCmdFor() tea.Cmd {
	if !m.Animating {
		return nil
	}
	return animTickCmd()
}

// animationGate answers whether this run may animate, reading the environment and
// the stream it would draw to. It is called once, when the model is built, and
// its answer lives on the model afterwards; the render path itself never reads
// the environment, so a render stays pure.
func animationGate(stdout *os.File) bool {
	if os.Getenv(envAnim) == "0" {
		return false
	}
	if os.Getenv(envTerm) == "dumb" {
		return false
	}
	return isCharDevice(stdout)
}

// hoverGate answers whether this run may ask the terminal for pointer motion. It
// is the mouse's own switch, and it is deliberately not the animation gate: a run
// may animate with no pointer -- the creature then looks at the selection, which
// is what it did before the pointer existed -- and an operator may want the
// pointer off while the animation stays on.
//
// Three things turn it off: DOTFILES_MOUSE=0, a stdout that is not a terminal (a
// redirected run has nothing to hover on and would only stream escape sequences
// into a file), and a Termux session, where the pointer is the user's finger.
// DOTFILES_MOUSE=1 overrides the Termux default for a session with a real mouse
// attached.
func hoverGate(stdout *os.File) bool {
	switch os.Getenv(envMouse) {
	case "0":
		return false
	case envMouseForce:
		return isCharDevice(stdout)
	}
	if os.Getenv(envTermux) != "" {
		return false
	}
	return isCharDevice(stdout)
}

// hoverRequested reports whether this run should ask the terminal for mouse
// motion: there is a creature to look with and the terminal will report the
// pointer. It is the single decision behind both halves of the pointer -- the
// model's Hovering field, which decides whether a mouse message means anything,
// and the program's mouse option, which main reads off that field -- so the two
// cannot disagree the way two environment reads would.
//
// The animation gate is half of it because a frozen creature has no eyes to move:
// asking for the pointer without one would cost the user the terminal's selection
// and show nothing for it.
func hoverRequested() bool {
	return animationGate(os.Stdout) && hoverGate(os.Stdout)
}

// isCharDevice reports whether f is a terminal-like character device. A pipe or
// a regular file is not, which is how a redirected stdout turns animation off
// without anyone setting a variable.
func isCharDevice(f *os.File) bool {
	if f == nil {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
