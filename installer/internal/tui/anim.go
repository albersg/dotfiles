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

// ============================================================================
// THE END-OF-RUN BURST
// ============================================================================
//
// The run's last moment is the one the user waited for, so it gets the one piece
// of pure delight the installer allows itself: a short fountain of particles and
// the companion joining in. It is bounded and honest: it lasts two seconds, it
// owns only the rows the body did not need, and its whole state -- whether it is
// running, which frame it is on and where every particle is -- lives on the
// model, so a snapshot pins it and the renderer only draws.
//
// The particles are not random and the renderer is not told to hurry: they are
// launched with a deterministic spread and advanced one step per frame tick, the
// same clock the companion walks on.

const (
	// celebrationFrames is how long the burst lasts. Two seconds, named through
	// the frame rate so it cannot drift from it.
	celebrationFrames = 2 * animTicksPerSecond

	// celebrationParticleCount is how many particles the fountain launches. It is
	// small on purpose: this is an exclamation at the end of a run, not a screen.
	celebrationParticleCount = 18

	// celebrationRowCount is the rows the burst may draw in, the rows nearest the
	// footer that the body did not need.
	celebrationRowCount = 3
)

// celebrationGlyphs are the shapes a particle can take. They are block and
// quadrant glyphs, all one cell wide, so the burst reads on a 16-colour terminal
// and on Termux.
var celebrationGlyphs = []rune("▘▝▖▗█")

// celebrationParticle is one particle of the end-of-run burst: a cell, a step
// and a glyph. Every position lives on the model, so a frame of the burst is
// pinned by the model's own fields rather than recomputed from a seed at render
// time.
type celebrationParticle struct {
	X, Y, VX, VY int
	Glyph        rune
}

// startCelebration launches the burst. It is a pure spread: particle i takes a
// column and a speed from i, so two runs on the same machine draw the same
// fountain.
func (m *Model) startCelebration() {
	width := m.Width
	if width < 20 {
		width = 80
	}
	m.Celebrating = true
	m.CelebrationTick = 0
	m.Particles = make([]celebrationParticle, celebrationParticleCount)
	for i := range m.Particles {
		m.Particles[i] = celebrationParticle{
			X:     (i*7 + 5) % width,
			Y:     i % celebrationRowCount,
			VX:    []int{-1, 0, 1}[i%3],
			VY:    1 + i%3,
			Glyph: celebrationGlyphs[i%len(celebrationGlyphs)],
		}
	}
}

// advanceCelebration is the burst's whole clock: it ages the burst, recycles the
// particles that left the top and stops the whole thing after a fixed number of
// frames. It is called from the frame tick and nowhere else.
func (m *Model) advanceCelebration() {
	if !m.Celebrating {
		return
	}
	m.CelebrationTick++
	if m.CelebrationTick >= celebrationFrames {
		m.Celebrating = false
		m.Particles = nil
		return
	}

	width := m.Width
	if width < 20 {
		width = 80
	}
	for i := range m.Particles {
		p := &m.Particles[i]
		p.X += p.VX
		if p.X < 0 {
			p.X = width - 1
		}
		if p.X >= width {
			p.X %= width
		}
		p.Y += p.VY
		if p.Y >= celebrationRowCount {
			// It left the visible rows, so it is launched again from the bottom at
			// a column derived from where it was: deterministic, and it keeps the
			// fountain alive for the whole burst.
			p.Y = 0
			p.X = (p.X*7 + 13) % width
		}
	}
}

// celebrationRows draws the burst in the rows nearest the footer. It returns
// nothing when the burst is over or the gate is off, so a screen with no
// celebration spends no row at all. Row 0 of the returned slice is the top line,
// so a particle's Y counts up from the bottom.
func (m Model) celebrationRows(width int) []string {
	if !m.Celebrating || width < 1 {
		return nil
	}
	canvas := make([][]rune, celebrationRowCount)
	for i := range canvas {
		canvas[i] = make([]rune, width)
		for j := range canvas[i] {
			canvas[i][j] = ' '
		}
	}
	for _, p := range m.Particles {
		if p.Y < 0 || p.Y >= celebrationRowCount || p.X < 0 || p.X >= width {
			continue
		}
		canvas[celebrationRowCount-1-p.Y][p.X] = p.Glyph
	}
	out := make([]string, celebrationRowCount)
	for i, row := range canvas {
		out[i] = HighlightStyle.Render(string(row))
	}
	return out
}
