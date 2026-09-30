package tui

import (
	"math"
	"strings"
)

// Hand-drawn charts, in glyphs a terminal already has.
//
// The installer draws its graphs from text, not from a plotting library and not
// from colour. The block glyphs below carry an eight-step level, and the
// sparkline is a row of them, so a 16-colour terminal and a colourless one read
// exactly the same chart. Colour, where a caller adds it, only decorates: the
// shape is the information.
//
// The renderers are pure functions of the numbers they are handed. A chart reads
// no clock and no file, so a snapshot can pin a series and the bytes it draws.

// sparkBlocks are the eight vertical levels a block sparkline draws, lowest
// first. They are Block Elements, present in every font the installer targets,
// including the one a 16-colour terminal and Termux ship with.
var sparkBlocks = []rune("▁▂▃▄▅▆▇█")

// sparkline renders a series as a row of block glyphs of at most width cells,
// scaled between min and max. When the series is longer than the width it keeps
// the newest values, so the row always ends at "now". A flat series at min draws
// the lowest glyph rather than a level that would invent a shape.
func sparkline(values []float64, min, max float64, width int) string {
	if width < 1 || len(values) == 0 {
		return ""
	}
	if len(values) > width {
		values = values[len(values)-width:]
	}

	var b strings.Builder
	for _, v := range values {
		b.WriteRune(sparkBlocks[valueLevel(v, min, max, len(sparkBlocks))])
	}
	return b.String()
}

// valueLevel is where one value lands on a scale of levels steps: 0 at the
// bottom of the range and levels-1 at the top. A range with no width (min equal
// to max), or one the value sits outside of, is clamped rather than allowed to
// index past the glyph table.
func valueLevel(v, min, max float64, levels int) int {
	if levels < 2 {
		return 0
	}
	if max <= min {
		return 0
	}
	f := (v - min) / (max - min)
	if f <= 0 {
		return 0
	}
	if f >= 1 {
		return levels - 1
	}
	return int(math.Round(f * float64(levels-1)))
}
