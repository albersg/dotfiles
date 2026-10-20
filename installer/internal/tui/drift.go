package tui

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// The drifting background.
//
// One paint point carries the background for every screen. View assembles the
// frame and hands it to paintDriftBackground once, at the tail every screen
// already passes through -- the forty cases of its switch, the utilities section
// and the trainer alike -- so a screen added later is painted without doing
// anything and no screen can opt out.
//
// The paint is a vertical gradient built from the theme's own colours: the
// theme's background at one end and a lightly accent-tinted variant at the
// other, interpolated per row. The band it draws is a smooth fold of that ramp,
// so it reads as texture behind the text rather than a stripe beside it, and it
// scrolls as the frame counter advances. The drift is a pure function of
// Model.AnimTick: it reads no clock and no environment, so two renders at the
// same counter are byte-identical.
//
// The painter only ever inserts and removes escape sequences. The visible text,
// its rows and each row's width are exactly what they were, which is what lets a
// drift-off run produce the bytes the package has always produced and a
// drift-on run differ by colour alone.
const (
	// driftAccentTint is how far the ramp's far end is pulled from the theme's
	// background toward its accent. It is small on purpose: the gradient sits
	// behind the text, and the text must stay exactly as legible on it as it is
	// on the plain background.
	driftAccentTint = 0.20

	// driftRowsPerTick is how far the band scrolls each frame tick. Half a row is
	// a slow walk -- four rows a second at animTicksPerSecond -- so the whole
	// screen takes several seconds to cross, and no single tick can look like a
	// jump.
	driftRowsPerTick = 0.5

	// driftReset is the sequence a lipgloss style closes a segment with. A row
	// carries many of them once the true-colour chrome is on, and each one would
	// otherwise clear the background for the rest of the row, so the painter
	// restores its background after every reset it finds.
	driftReset = "\x1b[0m"
)

// driftGate answers whether this run paints the drifting background at all: the
// animation gate the frame tick already rides, which the caller has already
// asked once, and a terminal that can show the ramp's 24-bit colours. The sprite
// asks the terminal the same question for the same reason (pixelSpriteAllowed,
// companion.go): where the profile cannot carry the ramp, the gradient would be
// approximated or dropped, and a background that is not the one the theme chose
// is worse than the theme's own. It is answered once, when the model is built,
// so the render path never reads the environment and the painter can trust the
// model's answer alone.
func driftGate(animating bool) bool {
	return animating && termenv.ColorProfile() == termenv.TrueColor
}

// paintDriftBackground paints the drifting gradient behind a fully assembled
// frame, one background colour per row. With the drift off it returns the frame
// byte for byte, so a drift-off run is today's render and the package's existing
// frame comparisons stay untouched.
func paintDriftBackground(frame string, m Model) string {
	if !m.Drifting {
		return frame
	}

	base := driftThemeColour(Background)
	accent := driftThemeColour(Accent)
	tinted := blendColour(base, accent, driftTintCap(base, accent))

	lines := strings.Split(frame, "\n")
	var out strings.Builder
	out.Grow(len(frame) + len(lines)*len(driftReset))
	for i, line := range lines {
		if i > 0 {
			out.WriteByte('\n')
		}
		paintDriftRow(&out, line, driftRowColour(base, tinted, i, len(lines), m.AnimTick))
	}
	return out.String()
}

// paintDriftRow writes one row with its own background. It paints the whole
// row, the padding columns included, so no edge is left on the plain background,
// and it re-arms the background after every reset the row already carries so a
// styled segment cannot punch a hole in the gradient.
func paintDriftRow(out *strings.Builder, line string, background color.RGBA) {
	sequence := string(appendColour(nil, background, false))
	out.WriteString(sequence)
	for {
		reset := strings.Index(line, driftReset)
		if reset < 0 {
			out.WriteString(line)
			break
		}
		end := reset + len(driftReset)
		out.WriteString(line[:end])
		out.WriteString(sequence)
		line = line[end:]
	}
	out.WriteString(driftReset)
}

// driftRowColour is the gradient's value at one row: a smooth fold between the
// theme's background and the accent-tinted end, advanced by the frame counter so
// the band scrolls as the tick moves. The counter is the only input besides the
// theme, so the render stays pure.
func driftRowColour(base, tinted color.RGBA, row, rows, tick int) color.RGBA {
	if rows < 1 {
		rows = 1
	}
	position := (float64(row) + float64(tick)*driftRowsPerTick) / float64(rows)
	fold := 0.5 - 0.5*math.Cos(2*math.Pi*position)
	return blendColour(base, tinted, fold)
}

// driftTintCap is how far the ramp may pull the base toward the accent without
// making the muted text less legible than it is on the plain base. The drift is
// decoration, and its contract is that the text reads on it exactly as it reads
// on the background; the fixed driftAccentTint broke that for the palettes whose
// muted role sits close to their base. The cap is derived from the palette in
// use, so a theme change cannot leave a stale constant behind.
func driftTintCap(base, accent color.RGBA) float64 {
	return driftTintCapFor(base, accent, hexColour(driftThemeColour(TextMuted)))
}

// driftTintCapFor is driftTintCap with the ink given explicitly, so a test can
// measure a theme's own muted role rather than the chrome the process happens to
// be wearing.
func driftTintCapFor(base, accent color.RGBA, inkHex string) float64 {
	plain := themeContrastRatio(inkHex, hexColour(base))
	if plain <= 0 {
		return driftAccentTint
	}
	// The drift may not take the muted role below the floor its palette is held
	// to: 4.5:1 when the plain background already clears the body floor, and
	// 3.0:1 for the themes that draw the role dimmer by design. The first version
	// required no loss at all, which flattened the gradient completely on a
	// palette whose muted role sat just under the threshold (Nocturne's 7.07:1
	// lost 0.03 with the first step, so the cap was zero). A floor keeps the
	// motion and the legibility at once.
	floor := 4.5
	if plain < 4.5 {
		floor = 3.0
	}
	if floor > plain {
		floor = plain
	}
	// Contrast is not monotone in a blend: a bright accent can carry the tint
	// past the ink's own luminance, where the contrast dips even though both ends
	// read well. The cap is the last step before the first dip, sampled rather
	// than searched, so the whole gradient stays above the floor.
	const steps = 32
	last := 0.0
	for i := 1; i <= steps; i++ {
		tint := driftAccentTint * float64(i) / steps
		if themeContrastRatio(inkHex, hexColour(blendColour(base, accent, tint))) < floor {
			return last
		}
		last = tint
	}
	return driftAccentTint
}

// hexColour is a colour as the #rrggbb string the contrast helpers read.
func hexColour(c color.RGBA) string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

// driftThemeColour resolves an adaptive theme colour against the terminal's
// background, the way companionInkFor does once at build: the theme's own value
// for this terminal, as straight bytes.
func driftThemeColour(c lipgloss.AdaptiveColor) color.RGBA {
	if lipgloss.HasDarkBackground() {
		return parseHexColour(c.Dark)
	}
	return parseHexColour(c.Light)
}
