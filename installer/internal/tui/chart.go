package tui

// Braille dot-matrix charts.
//
// A braille glyph is a 2x4 grid of dots in one cell, so a chart of h rows
// resolves 4h levels -- four times the block sparkline's eight -- in the same
// columns. That is where the density the issue asks for comes from: a heartbeat
// that would be a flat line in block glyphs shows its shape in braille.
//
// Like the sparkline, this is a pure function of the numbers handed to it: no
// clock, no I/O, no colour. The dots are the information.

// brailleBase is the code point of an empty braille cell (U+2800); each set dot
// adds its bit.
const brailleBase = 0x2800

// brailleDotBit maps a dot inside one cell to its bit in the braille pattern:
// the first index is the horizontal half (0 left, 1 right), the second the row
// (0 top .. 3 bottom). The mapping is the standard Unicode braille ordering,
// which is why it is a table and not arithmetic.
var brailleDotBit = [2][4]byte{
	{0x01, 0x02, 0x04, 0x40},
	{0x08, 0x10, 0x20, 0x80},
}

// brailleChart renders a series as a filled dot-matrix chart: width cells
// across and height rows down. Each cell holds two dot columns and four dot
// rows, so the chart resolves width*2 dot columns and height*4 levels.
//
// The newest values are kept when the series is longer than the chart, and a
// series shorter than the chart is drawn from the left, so the reader always
// reads "older on the left, now on the right". A flat series is clamped to the
// bottom row rather than invented into a shape.
func brailleChart(values []float64, min, max float64, width, height int) []string {
	if width < 1 || height < 1 || len(values) == 0 {
		return nil
	}

	dotColumns := width * 2
	if len(values) > dotColumns {
		values = values[len(values)-dotColumns:]
	}
	levels := height * 4

	cells := make([][]byte, height)
	for r := range cells {
		cells[r] = make([]byte, width)
	}

	for i, v := range values {
		level := valueLevel(v, min, max, levels)
		// Fill from the bottom dot up to the value's level, so the chart reads
		// as an area chart rather than as a scatter of points.
		for d := 0; d <= level && d < levels; d++ {
			row := height - 1 - d/4
			dotRow := d % 4
			col := i / 2
			dotCol := i % 2
			cells[row][col] |= brailleDotBit[dotCol][dotRow]
		}
	}

	out := make([]string, height)
	for r := range cells {
		runes := make([]rune, width)
		for c, bits := range cells[r] {
			runes[c] = rune(brailleBase + int(bits))
		}
		out[r] = string(runes)
	}
	return out
}
