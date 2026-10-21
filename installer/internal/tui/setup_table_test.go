package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// The setup table's guards. The screen is the mockup's panel 3 -- a table of the
// flow's decisions with the row under the cursor described under it -- so the
// guards hold the promises that panel makes: every row is drawn whole (no name
// and no value cut to make the table fit), the chevron is only on the rows Enter
// opens, the band under the table never repeats a row's value, and nothing on the
// screen moves when the cursor does.

// setupTableFrame builds the screen's worst case, the one the frame matrix
// measures: every decision present (the shell's, the multiplexer's and the
// editor's plugin sets included) and a configuration on the machine, so the
// backup row is offered too.
func setupTableFrame(t *testing.T) Model {
	t.Helper()
	m := installerFrameCase(t, "setup")
	m.Animating, m.PixelSprite = true, false
	m.AnimTick = 0
	return m
}

// TestSetupTableDrawsEveryRowWholeAtEveryMeasuredSize is the floor's guard: at
// every terminal the matrix measures, no name and no value on the screen is cut,
// the row under the cursor is always one of the rows drawn, and the rows a short
// terminal cannot show are counted rather than dropped.
func TestSetupTableDrawsEveryRowWholeAtEveryMeasuredSize(t *testing.T) {
	for _, size := range measuredTerminalSizes {
		size := size
		t.Run(size.name, func(t *testing.T) {
			m := setupTableFrame(t)
			m.Width, m.Height = size.width, size.height

			view := ansiEscape.ReplaceAllString(m.View(), "")
			for _, line := range strings.Split(view, "\n") {
				if strings.Contains(line, cutMarker) {
					t.Errorf("%s cuts a row: a name or a value was shortened to fit %q", size.name, line)
				}
			}

			selected, ok := m.setupSelectedRow()
			if !ok {
				t.Fatal("the setup screen has no row under the cursor")
			}
			if !strings.Contains(view, selected.Label) {
				t.Errorf("%s does not draw %q, the row the keys act on:\n%s", size.name, selected.Label, view)
			}

			drawn := 0
			for _, row := range m.setupRows() {
				if strings.Contains(view, row.Label) {
					drawn++
				}
			}
			if drawn < len(m.setupRows()) && !strings.Contains(view, "more") {
				t.Errorf("%s draws %d of %d rows without saying how many it left out:\n%s",
					size.name, drawn, len(m.setupRows()), view)
			}
		})
	}
}

// TestSetupTableTakesTheMockupsShapeWhereItFits holds the two shapes: at a frame
// tall enough for the box, the rule between every pair of rows and the panel under
// them, the mockup's boxed table is drawn; at the 80x24 floor it is not, because
// the box would spend rows the list needs -- and there every row is on screen.
func TestSetupTableTakesTheMockupsShapeWhereItFits(t *testing.T) {
	wide := setupTableFrame(t)
	wide.Width, wide.Height = 160, 40
	wideView := ansiEscape.ReplaceAllString(wide.View(), "")
	for _, glyph := range []string{"╭", "├", "╰"} {
		if !strings.Contains(wideView, glyph) {
			t.Errorf("the wide setup screen does not draw the mockup's table (%q is missing):\n%s", glyph, wideView)
		}
	}

	floor := setupTableFrame(t)
	floor.Width, floor.Height = 80, 24
	floorView := ansiEscape.ReplaceAllString(floor.View(), "")
	for _, glyph := range []string{"╭", "├", "╰"} {
		if strings.Contains(floorView, glyph) {
			t.Errorf("the 80x24 setup screen draws the box (%q), which spends rows its list needs:\n%s", glyph, floorView)
		}
	}
	for _, row := range floor.setupRows() {
		if !strings.Contains(floorView, row.Label) {
			t.Errorf("the 80x24 setup screen does not draw the %q row: the floor is for reaching every decision", row.Label)
		}
	}

	// The band under the table follows the same rule: the mockup's panel body at
	// the wide frame, the one-line summary on the floor.
	if !strings.Contains(floorView, "Platform  Select your operating system.") {
		t.Errorf("the 80x24 setup screen does not summarise the row under the cursor:\n%s", floorView)
	}
	if !strings.Contains(wideView, "Select your operating system. This will adjust the installation") {
		t.Errorf("the wide setup screen does not draw the panel's description:\n%s", wideView)
	}
}

// TestSetupRowsCarryTheChevronOnlyWhereEnterOpensSomething is the chevron's
// meaning, taken from the mockup's own table: the rows Enter opens carry it, and
// a plugin set -- which is toggled where it stands and has no screen of its own --
// does not.
func TestSetupRowsCarryTheChevronOnlyWhereEnterOpensSomething(t *testing.T) {
	m := setupTableFrame(t)
	rows := m.setupRows()
	if len(rows) == 0 {
		t.Fatal("the setup screen has no rows")
	}

	opened := 0
	for _, row := range rows {
		line := m.setupRowLines(row, false, 158, 0, 1)[0]
		has := strings.Contains(line, setupChevron)
		if has != setupRowOpensScreen(row) {
			t.Errorf("the %q row draws the chevron %v, want %v: it %s a screen",
				row.Label, has, setupRowOpensScreen(row), map[bool]string{true: "opens", false: "does not open"}[setupRowOpensScreen(row)])
		}
		if has {
			opened++
		}
	}
	if opened != len(rows)-countPluginSetRows(rows) {
		t.Errorf("%d of %d rows carry the chevron, want every row but the plugin sets", opened, len(rows))
	}
	if countPluginSetRows(rows) == 0 {
		t.Fatal("the fixture names no plugin set, so this guard checked nothing")
	}
}

// countPluginSetRows counts the set rows in a row list.
func countPluginSetRows(rows []setupRow) int {
	count := 0
	for _, row := range rows {
		if row.Kind == setupRowPluginSet {
			count++
		}
	}
	return count
}

// TestSetupRowGlyphsAreOneCell holds S3's glyph rule for this screen: every row
// icon and the chevron is one cell wide, so the table's columns cannot shift on a
// terminal whose font draws one of them wide -- and the row's name, not the icon,
// is what the row says.
func TestSetupRowGlyphsAreOneCell(t *testing.T) {
	glyphs := map[string]string{
		"platform":    setupPlatformIcon,
		"shell":       setupShellIcon,
		"terminal":    setupTerminalIcon,
		"font":        setupFontIcon,
		"multiplexer": setupMultiplexerIcon,
		"editor":      setupEditorIcon,
		"configs":     setupConfigsIcon,
		"plugin set":  setupPluginSetIcon,
		"plan":        setupPlanIcon,
		"chevron":     setupChevron,
	}
	for name, glyph := range glyphs {
		if glyph == "" {
			t.Errorf("the %s glyph is empty", name)
			continue
		}
		if width := lipgloss.Width(glyph); width != 1 {
			t.Errorf("the %s glyph %q draws %d cells, want 1", name, glyph, width)
		}
	}
}

// TestSetupEveryRowCarriesItsDescriptionAndNoValue pins the band's content: every
// row of the table has a description to show, and none of them states the value
// the row beside it already carries. A panel that repeated the value would be a
// second copy of the state, and one that had nothing to say would leave the
// reader with a heading and no sentence.
func TestSetupEveryRowCarriesItsDescriptionAndNoValue(t *testing.T) {
	m := setupTableFrame(t)
	for _, row := range m.setupRows() {
		description := setupRowDescription(row)
		if strings.TrimSpace(description) == "" {
			t.Errorf("the %q row has no description for the panel under the table", row.Label)
		}
		if value := m.setupRowValue(row); value != "" && strings.Contains(description, value) {
			t.Errorf("the %q row's description repeats its value %q: %q", row.Label, value, description)
		}
	}
}

// TestSetupScreenDoesNotMoveWhenTheCursorMoves is the stability guard: walking the
// table changes what the rows and the band say, never where they are. The band's
// reservation is the widest row's, so a long description cannot take a row from
// the list, and the frame drawn at the last row is as tall as the one drawn at the
// first.
func TestSetupScreenDoesNotMoveWhenTheCursorMoves(t *testing.T) {
	for _, size := range measuredTerminalSizes {
		m := setupTableFrame(t)
		m.Width, m.Height = size.width, size.height

		m.SetupCursor = 0
		view := ansiEscape.ReplaceAllString(m.View(), "")
		wantRows := renderedRowCount(view)
		wantTableTop := strings.Index(view, "Configure your environment")

		for cursor := range m.setupRows() {
			m.SetupCursor = cursor
			view := ansiEscape.ReplaceAllString(m.View(), "")
			if got := renderedRowCount(view); got != wantRows {
				t.Fatalf("%s with the cursor on row %d draws %d rows, want the %d the first row drew: the screen moves as the cursor walks it",
					size.name, cursor, got, wantRows)
			}
			if got := strings.Index(view, "Configure your environment"); got != wantTableTop {
				t.Fatalf("%s with the cursor on row %d puts the table's title at %d, want %d",
					size.name, cursor, got, wantTableTop)
			}
		}
	}
}

// TestSetupRowWindowKeepsTheCursorRowAndCountsWhatItHides is the window's own
// arithmetic: a row that spends two lines is measured as two, the cursor's row is
// always inside the window, and the notes that say what the window hid are part of
// the lines it was given.
func TestSetupRowWindowKeepsTheCursorRowAndCountsWhatItHides(t *testing.T) {
	cases := []struct {
		name    string
		heights []int
		lines   int
	}{
		{"every row fits", []int{1, 1, 1, 1}, 8},
		{"one row spends two lines", []int{1, 1, 2, 1, 1}, 5},
		{"a window over a long list", []int{1, 1, 1, 1, 1, 1, 1}, 4},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for cursor := range c.heights {
				start, end := setupRowWindow(c.heights, cursor, c.lines, false)
				if cursor < start || cursor >= end {
					t.Fatalf("the window %d..%d leaves the cursor's row %d out", start, end, cursor)
				}
				spent := sum(c.heights[start:end])
				if start > 0 {
					spent++
				}
				if end < len(c.heights) {
					spent++
				}
				if spent > c.lines && !(end-start == 1) {
					t.Errorf("the window %d..%d spends %d lines, want at most %d", start, end, spent, c.lines)
				}
			}
		})
	}

	// The whole list is drawn when it fits, with no note spending a line.
	if start, end := setupRowWindow([]int{1, 1, 1}, 1, 3, false); start != 0 || end != 3 {
		t.Errorf("a list that fits the lines is drawn as the window %d..%d, want 0..3", start, end)
	}
}
