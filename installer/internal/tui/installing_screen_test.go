package tui

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// The installing screen's guards. The screen is the one a person watches for
// minutes, so its shape is pinned the way the other redesign slices pin theirs: the
// mockup's stack of title, step and clock over the checklist and the log card, the
// bar under both, and the footer's one way out. What none of these guards may allow
// is a fact the run does not hold -- a per-step duration, a percentage no step
// reported, or a state glyph for a step that is not in that state.

// installingPlain strips the styling, so the assertions read what a person reads on
// a terminal with no colour.
func installingPlain(m Model) string {
	return ansiEscape.ReplaceAllString(m.View(), "")
}

// installingRow finds the one row of a plain view that contains want.
func installingRow(t *testing.T, view, want string) string {
	t.Helper()
	for _, row := range strings.Split(view, "\n") {
		if strings.Contains(row, want) {
			return row
		}
	}
	t.Fatalf("the view has no row containing %q:\n%s", want, view)
	return ""
}

// installingChecklistRow finds a step's own row. The step's name is also in the
// line that names the step the run is on, so the row is the checklist's: the one
// that carries the name and is not the step line.
func installingChecklistRow(t *testing.T, view, name string) string {
	t.Helper()
	for _, row := range strings.Split(view, "\n") {
		if strings.Contains(row, name) && !strings.Contains(row, "Step ") {
			return row
		}
	}
	t.Fatalf("the checklist has no row for %q:\n%s", name, view)
	return ""
}

// installingRowIndex is the row number a plain view draws want on, so a guard can
// say that two renders put the same thing on the same row.
func installingRowIndex(t *testing.T, view, want string) int {
	t.Helper()
	for i, row := range strings.Split(view, "\n") {
		if strings.Contains(row, want) {
			return i
		}
	}
	t.Fatalf("the view has no row containing %q:\n%s", want, view)
	return -1
}

// installingColumn is where a plain row's own first character starts: the frame
// pads every line two columns on each side, as View() does for every screen.
const installingColumn = viewPaddingCols

// installingStatefulModel is one step of the plan in each state the installer can
// put a step in: done, failed, running, skipped and pending. The screen has to say
// all five in a glyph, so a colour-blind or colourless terminal still reads them.
func installingStatefulModel(t *testing.T) Model {
	t.Helper()
	m := installerFrameModel(t, ScreenInstalling)
	m.Width, m.Height = 80, 24
	m.ShowDetails = false
	m.Steps = []InstallStep{
		{ID: "backup", Name: "Backup Existing Configs", Status: StatusDone, Progress: 1},
		{ID: "gai", Name: "Prefer IPv4", Status: StatusFailed, Error: errors.New("refused")},
		{ID: "deps", Name: "Install Dependencies", Description: "Installs base packages.", Status: StatusRunning, Progress: 0.4},
		{ID: "clone", Name: "Clone Repository", Status: StatusSkipped},
		{ID: "brew", Name: "Install Homebrew", Status: StatusPending},
	}
	m.CurrentStep = 2
	return m
}

// TestInstallingScreenCopiesTheMocksShape pins the panel the mockup draws: the
// run's sentence over the screen, the step it is on with the run's own clock, the
// checklist beside the log card, the bar under both with its percentage at the
// frame's right edge, and the footer's own way out. It is asserted at the 80x24
// floor and at the wide frame, because the two columns are drawn at both.
func TestInstallingScreenCopiesTheMocksShape(t *testing.T) {
	for _, size := range []struct {
		name          string
		width, height int
	}{
		{"80x24, the floor", 80, 24},
		{"160x50, the mockup's", 160, 50},
	} {
		size := size
		t.Run(size.name, func(t *testing.T) {
			m := installerFrameCase(t, "installing-details")
			m.Width, m.Height = size.width, size.height
			view := installingPlain(m)
			composition := layoutFor(m).Composition

			// The mockup's three lines over the two columns, centred in the body.
			title := installingRow(t, view, installingRunningTitle)
			wantColumn := (composition-len(installingRunningTitle))/2 + installingColumn
			if got := lipgloss.Width(title) - lipgloss.Width(strings.TrimLeft(title, " ")); got != wantColumn {
				t.Errorf("the run's sentence starts at column %d, want the centred %d:\n%s", got, wantColumn, view)
			}
			step := installingRow(t, view, "Step 3 of 16")
			if !strings.Contains(step, installingStepSeparator+"Install Dependencies") {
				t.Errorf("the step line does not name the step the run is on:\n%s", step)
			}
			// The clock is the run's own: an elapsed time, or the sentence saying it has
			// no start time to measure one from yet.
			if !strings.Contains(view, "Elapsed") && !strings.Contains(view, "Estimating the time remaining") {
				t.Errorf("the run's own clock is not on the screen:\n%s", view)
			}

			// The header chip is the mockup's word, and the status bar under the body
			// is the frame's state.
			if !strings.Contains(view, "Installing") || !strings.Contains(view, statusInstalling) {
				t.Errorf("the frame does not name the screen %q and the state %q:\n%s",
					"Installing", statusInstalling, view)
			}

			// The checklist's rows and the log card's borders are on the same band.
			row := installingChecklistRow(t, view, "Install Dependencies")
			if !strings.Contains(row, "●") {
				t.Errorf("the running step's row carries no running glyph: %q", row)
			}
			cardTop := installingRowIndex(t, view, "╭")
			cardBottom := installingRowIndex(t, view, "╰")
			if cardTop >= cardBottom {
				t.Errorf("the log card's borders are at rows %d and %d, want the top above the bottom:\n%s", cardTop, cardBottom, view)
			}
			if railAt := installingRowIndex(t, view, "✓ Backup Existing Configs"); railAt != cardTop {
				t.Errorf("the checklist starts at row %d and the log card at row %d, want them on one band:\n%s", railAt, cardTop, view)
			}
			if logAt := installingRowIndex(t, view, "> "); logAt != cardTop+1 {
				t.Errorf("the log card's first line is at row %d and its border at %d:\n%s", logAt, cardTop, view)
			}

			// The bar runs the composition and states the run's percentage at its end.
			bar := installingRow(t, view, "%")
			if !strings.HasSuffix(strings.TrimRight(bar, " "), fmt.Sprintf("%3.0f%%", m.installProgress()*100)) {
				t.Errorf("the bar's row does not end with the run's percentage: %q", bar)
			}
			if got, want := lipgloss.Width(strings.TrimRight(bar, " "))-installingColumn, composition; got != want {
				t.Errorf("the bar's row is %d columns, want the composition's %d: %q", got, want, bar)
			}

			// The footer puts the state at the left of the rule and the way out at the
			// right, as the mockup does.
			if !strings.Contains(view, "[q] Cancel") {
				t.Errorf("the footer does not offer the mockup's way out of the run:\n%s", view)
			}
			// The frame the terminal gives is the frame the screen uses: no row wider
			// than the terminal and none past its last row, at either size.
			if rows := renderedRowCount(view); rows > size.height {
				t.Errorf("the screen draws %d rows in a %d-row terminal", rows, size.height)
			}
			for _, line := range strings.Split(view, "\n") {
				if got := lipgloss.Width(line); got > size.width {
					t.Errorf("the screen draws a %d-column row in a %d-column terminal: %q", got, size.width, line)
				}
			}
		})
	}
}

// TestInstallingChecklistStatesEveryStepTheRunKnows pins the five states in the
// glyph the row carries. The glyph is what states a step's state on a terminal with
// no colour, so a row that lost it would leave the tone as the only signal.
func TestInstallingChecklistStatesEveryStepTheRunKnows(t *testing.T) {
	m := installingStatefulModel(t)
	view := installingPlain(m)

	for _, c := range []struct {
		step   string
		glyph  string
		meter  string
		reason string
	}{
		{"Backup Existing Configs", "✓", strings.Repeat("▓", installingRailMeterCells), "a completed step is full"},
		{"Prefer IPv4", "✗", strings.Repeat("░", installingRailMeterCells), "a failed step did not fill the run's progress"},
		{"Install Dependencies", "●", strings.Repeat("▓", 4) + strings.Repeat("░", installingRailMeterCells-4), "a running step carries the fraction it reported"},
		{"Clone Repository", "⊘", strings.Repeat("▓", installingRailMeterCells), "a skipped step counts towards the run's progress"},
		{"Install Homebrew", "○", strings.Repeat("░", installingRailMeterCells), "a pending step has no progress"},
	} {
		row := installingChecklistRow(t, view, c.step)
		if !strings.Contains(row, c.glyph) {
			t.Errorf("%s's row carries no %q glyph: %q", c.step, c.glyph, row)
		}
		if !strings.Contains(row, c.meter) {
			t.Errorf("%s's row does not carry the meter its own state earns (%s): %q", c.step, c.reason, row)
		}
	}

	// Every step of the plan is on screen at the floor, so nothing the model holds is
	// hidden by this state.
	for _, step := range m.Steps {
		if !strings.Contains(view, step.Name) {
			t.Errorf("the checklist does not draw the step %q:\n%s", step.Name, view)
		}
	}
}

// TestInstallingDoesNotInventStepTimes pins the one fact the mockup's checklist
// carries and this installer does not: a duration per step. The run keeps a start
// time, not a per-step clock, so no checklist row may state one -- the trailing
// column is the meter of the fraction the step really reported.
func TestInstallingDoesNotInventStepTimes(t *testing.T) {
	m := installingStatefulModel(t)
	m.ShowDetails = true
	m.LogLines = []string{"Installing base packages with apt-get.", "Reading package lists..."}
	// A run with a start time and a later tick, so the screen has a real clock to
	// show: the guard's point is that the clock is the run's, not the rows'.
	m.InstallStartedAt = goldenGreetingTime
	m.Now = goldenGreetingTime.Add(2*time.Minute + 14*time.Second)
	view := installingPlain(m)

	duration := regexp.MustCompile(`\b\d+(\.\d+)?s\b`)
	for _, step := range m.Steps {
		row := installingChecklistRow(t, view, step.Name)
		if got := duration.FindString(row); got != "" {
			t.Errorf("%s's row states the duration %q, which this run does not record: %q", step.Name, got, row)
		}
	}

	// The run's clock is shown, because the run really has one.
	if !strings.Contains(view, "Elapsed 2m 14s") {
		t.Errorf("the screen does not state the run's own elapsed time:\n%s", view)
	}
}

// TestInstallingListRowsSayWhatTheyMeasure pins the meter's arithmetic against the
// numbers the model holds, so a row cannot round a fraction into a state it does
// not have: the filled cells are the step's own fraction of the meter's width.
func TestInstallingListRowsSayWhatTheyMeasure(t *testing.T) {
	for _, progress := range []float64{0, 0.24, 0.5, 0.99, 1} {
		progress := progress
		t.Run(fmt.Sprintf("%.2f", progress), func(t *testing.T) {
			step := InstallStep{Name: "Install Dependencies", Status: StatusRunning, Progress: progress}
			row := ansiEscape.ReplaceAllString(installingRailRow(step, 60, installingRailMeterCells, true), "")
			filled := strings.Count(row, "▓")
			want := int(progress*installingRailMeterCells + 0.5)
			if filled != want {
				t.Errorf("a step at %v fills %d cells, want %d: %q", progress, filled, want, row)
			}
			if got := lipgloss.Width(row); got != 60 {
				t.Errorf("the row is %d columns in a 60-column checklist: %q", got, row)
			}
		})
	}
}

// TestInstallingFloorKeepsTheCurrentStepAndTheLog pins the floor's two contracts:
// the step in progress is always on screen whatever the log does, and the log card
// shows its freshest lines and says how many it could not. A longer history must
// not take the current step's row.
func TestInstallingFloorKeepsTheCurrentStepAndTheLog(t *testing.T) {
	m := installerFrameCase(t, "installing-details-many")
	m.Width, m.Height = 80, 24
	view := installingPlain(m)

	if !strings.Contains(view, "Install Dependencies") {
		t.Errorf("the step the run is on is not on screen at the floor:\n%s", view)
	}
	if !strings.Contains(view, "> log line 40") {
		t.Errorf("the log card dropped the freshest line it holds:\n%s", view)
	}
	if !strings.Contains(view, "earlier lines") {
		t.Errorf("the log card did not name the lines it could not show:\n%s", view)
	}
	// What the floor may not do is cut a step's name: every row of the checklist it
	// draws is drawn whole, and the window says how many it left out through the
	// header's count instead of a half name. The row's right half is the log card's,
	// whose own note legitimately starts with the cut glyph, so only the checklist's
	// columns are read here.
	for _, step := range m.Steps {
		if !strings.Contains(view, step.Name) {
			continue
		}
		rail := installingChecklistRow(t, view, step.Name)
		if at := strings.Index(rail, "│"); at >= 0 {
			rail = rail[:at]
		}
		if strings.Contains(rail, cutMarker) {
			t.Errorf("the checklist cuts %q at the floor: %q", step.Name, rail)
		}
	}
	assertInstallerScreenFits(t, "installing-floor-with-log", m)
}

// TestInstallingColumnsSplitTheFrame pins the two columns' arithmetic: the checklist
// and the log card fill the composition exactly, with one gap between them, and the
// card is never drawn at a width it cannot read a line in.
func TestInstallingColumnsSplitTheFrame(t *testing.T) {
	for _, size := range measuredTerminalSizes {
		size := size
		t.Run(size.name, func(t *testing.T) {
			m := installerFrameCase(t, "installing-details")
			m.Width, m.Height = size.width, size.height

			composition := layoutFor(m).Composition
			band := m.installingBandFor(composition, 10)
			if band.RailWidth > composition {
				t.Fatalf("the checklist is %d columns in a %d-column composition", band.RailWidth, composition)
			}
			if !band.ShowLog {
				if band.RailWidth != composition {
					t.Errorf("without a log card the checklist takes %d of the %d columns", band.RailWidth, composition)
				}
				return
			}
			if got := band.RailWidth + installingColumnGap + band.LogWidth; got != composition {
				t.Errorf("the two columns take %d of the composition's %d columns", got, composition)
			}
			if band.LogWidth < installingLogMinWidth {
				t.Errorf("the log card is %d columns wide, narrower than the %d a line needs", band.LogWidth, installingLogMinWidth)
			}
			if band.RailWidth < installingRailMinWidth {
				t.Errorf("the checklist is %d columns wide, narrower than the %d its rows need", band.RailWidth, installingRailMinWidth)
			}
		})
	}
}

// TestInstallingLayoutDoesNotMoveAsStepsAdvance pins the frame's stability: the run
// advances a step at a time, and every row above the bar has to stay where it is
// while it does. The band's height is the frame's, not the log's and not the
// checklist's, which is what makes that true.
func TestInstallingLayoutDoesNotMoveAsStepsAdvance(t *testing.T) {
	m := installingStatefulModel(t)
	m.ShowDetails = true
	m.LogLines = []string{"line one", "line two", "line three"}

	before := installingPlain(m)
	barBefore := installingRowIndex(t, before, "%")
	cardBefore := installingRowIndex(t, before, "╭")
	rowsBefore := renderedRowCount(before)

	// The same run, one step further on: the step that was running is done and the
	// next one started, with one more log line behind it.
	m.Steps[2].Status = StatusDone
	m.Steps[2].Progress = 1
	m.Steps[3].Status = StatusRunning
	m.Steps[3].Progress = 0.2
	m.CurrentStep = 3
	m.LogLines = append(m.LogLines, "line four")

	after := installingPlain(m)
	if got := installingRowIndex(t, after, "%"); got != barBefore {
		t.Errorf("the bar moved from row %d to row %d as the run advanced:\n%s", barBefore, got, after)
	}
	if got := installingRowIndex(t, after, "╭"); got != cardBefore {
		t.Errorf("the log card moved from row %d to row %d as the run advanced:\n%s", cardBefore, got, after)
	}
	if got := renderedRowCount(after); got != rowsBefore {
		t.Errorf("the screen changed from %d rows to %d as the run advanced", rowsBefore, got)
	}
}

// TestInstallingBarIsTheRunsOwnProgress pins the bar against installProgress: the
// percentage is the run's, the filled cells are that fraction of the bar, and a bar
// with no run behind it reads complete rather than inventing a step.
func TestInstallingBarIsTheRunsOwnProgress(t *testing.T) {
	for _, progress := range []float64{0, 0.25, 0.5, 1} {
		progress := progress
		t.Run(fmt.Sprintf("step at %.2f", progress), func(t *testing.T) {
			m := installingClockModel(t, progress)
			plain := installingPlain(m)
			row := installingRow(t, plain, "%")

			want := fmt.Sprintf("%3.0f%%", m.installProgress()*100)
			if !strings.HasSuffix(strings.TrimRight(row, " "), want) {
				t.Errorf("the bar states %q, want the run's own %q: %q", strings.TrimRight(row, " "), want, row)
			}
			barWidth := layoutFor(m).Composition - 5
			filled := strings.Count(row, "█")
			if got, wantFilled := filled, int(m.installProgress()*float64(barWidth)+0.5); got != wantFilled {
				t.Errorf("the bar fills %d of %d cells at %v, want %d", got, barWidth, m.installProgress(), wantFilled)
			}
			if empty := strings.Count(row, "░"); filled+empty != barWidth {
				t.Errorf("the bar's cells number %d, want the %d the frame leaves", filled+empty, barWidth)
			}
		})
	}

	// A run with no plan is complete, not 0%: there is nothing left to do.
	empty := installerFrameModel(t, ScreenInstalling)
	if row := installingRow(t, installingPlain(empty), "%"); !strings.Contains(row, "100%") {
		t.Errorf("a run with no steps does not read complete: %q", row)
	}
}

// TestInstallingFooterStopsTheRunOnQ pins the key the footer names. The footer says
// "[q] Cancel", so q has to stop the run, and it has to stop it the way ctrl+c
// always has -- the program quits, the step stops where it is and no record is
// written, because nothing failed.
func TestInstallingFooterStopsTheRunOnQ(t *testing.T) {
	m := installerFrameModel(t, ScreenInstalling)
	m.Steps = []InstallStep{{ID: "deps", Name: "Install Dependencies", Status: StatusRunning}}

	if view := installingPlain(m); !strings.Contains(view, "[q] Cancel") {
		t.Fatalf("the installing footer does not name the key this guard presses:\n%s", view)
	}

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if !next.(Model).Quitting {
		t.Error("q did not stop the run the footer offers it for")
	}
	if cmd == nil {
		t.Error("q stopped the run but asked the program to keep running")
	}

	// The same stop, from the key that always had it.
	ctrlC, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !ctrlC.(Model).Quitting {
		t.Error("ctrl+c no longer stops the run")
	}

	// A step that is waiting on the Windows Terminal offer is still stoppable: the
	// offer owns y, n and Esc, not the way out of the whole run.
	offer := m
	offer.WSLTerminalOffer = &windowsTerminalOffer{}
	offer.WSLTerminalOfferLines = []string{"A line of the offer."}
	if next, _ := offer.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}); !next.(Model).Quitting {
		t.Error("q did not stop a run that is paused on the WSL offer")
	}
}

// TestInstallingLogKeepsTheMockupsPrompt pins the log card's rows: the mockup leads
// every line with a prompt, the freshest lines are the ones kept, and a run that has
// logged nothing yet says so rather than drawing an empty box.
func TestInstallingLogKeepsTheMockupsPrompt(t *testing.T) {
	rows := Model{LogLines: []string{"first", "second", "third"}}.installingLogRows(20, 4)
	if len(rows) != 3 {
		t.Fatalf("three log lines in a four-row card drew %d rows: %v", len(rows), rows)
	}
	for i, row := range rows {
		plain := ansiEscape.ReplaceAllString(row, "")
		if !strings.HasPrefix(plain, installingLogPrompt) {
			t.Errorf("log row %d carries no prompt: %q", i, plain)
		}
	}
	if plain := ansiEscape.ReplaceAllString(rows[2], ""); !strings.Contains(plain, "third") {
		t.Errorf("the card dropped its freshest line: %q", plain)
	}

	quiet := Model{}.installingLogRows(30, 4)
	if len(quiet) != 1 || !strings.Contains(ansiEscape.ReplaceAllString(quiet[0], ""), "Waiting for the first line") {
		t.Errorf("a run with no log drew %v, want one row saying it is waiting", quiet)
	}
}

// TestInstallingChecklistNeverCutsAStepName pins the ladder the checklist's trailing
// meter takes: the full meter where the longest step name still fits beside it, the
// narrow one where only that fits, and none at all below it. A column that cannot
// hold both keeps the name -- a cut name is a lost fact -- and the run's own bar
// still carries the progress. The worst name the plan builder can produce is the one
// measured, not the golden host's plan.
func TestInstallingChecklistNeverCutsAStepName(t *testing.T) {
	for _, width := range []int{30, installingRailNarrowWidth, installingRailWideWidth, 60} {
		step := InstallStep{Name: strings.Repeat("M", installingRailLongestName), Status: StatusPending}
		row := ansiEscape.ReplaceAllString(installingRailRow(step, width, installingRailCells(width), false), "")
		if strings.Contains(row, cutMarker) {
			t.Errorf("a %d-column checklist cuts the longest step name it can hold: %q", width, row)
		}
		if got := lipgloss.Width(row); got != width {
			t.Errorf("a %d-column checklist drew a %d-column row: %q", width, got, row)
		}
	}

	// The ladder itself, at the widths the measured terminals produce.
	for _, c := range []struct {
		width, cells int
	}{
		{installingRailWideWidth, installingRailMeterCells},
		{installingRailWideWidth - 1, installingRailMeterCellsNarrow},
		{installingRailNarrowWidth - 1, 0},
	} {
		if got := installingRailCells(c.width); got != c.cells {
			t.Errorf("a %d-column checklist takes a %d-cell meter, want %d", c.width, got, c.cells)
		}
	}
}

// TestInstallingEveryMeasuredFrameDrawsTheWholePlan pins the class the floor guard
// covers at one size: at every terminal the frame is measured at, the step in
// progress is on screen and no checklist row it draws is cut.
func TestInstallingEveryMeasuredFrameDrawsTheWholePlan(t *testing.T) {
	for _, size := range measuredTerminalSizes {
		size := size
		t.Run(size.name, func(t *testing.T) {
			m := installerFrameCase(t, "installing-details")
			m.Width, m.Height = size.width, size.height
			view := installingPlain(m)

			current := m.Steps[m.CurrentStep].Name
			if !strings.Contains(view, current) {
				t.Fatalf("the step the run is on (%q) is not on screen at %s:\n%s", current, size.name, view)
			}
			// The log the run is writing is on screen too: the card keeps the freshest
			// line at every measured size, which is what makes it worth a column.
			if freshest := m.LogLines[len(m.LogLines)-1]; !strings.Contains(view, freshest) {
				t.Errorf("%s does not show the freshest log line (%q):\n%s", size.name, freshest, view)
			}
			for _, step := range m.Steps {
				if !strings.Contains(view, step.Name) {
					continue
				}
				rail := installingChecklistRow(t, view, step.Name)
				if at := strings.Index(rail, "│"); at >= 0 {
					rail = rail[:at]
				}
				if strings.Contains(rail, cutMarker) {
					t.Errorf("%s cuts the step %q: %q", size.name, step.Name, rail)
				}
			}
		})
	}
}

// TestInstallingHeadAlwaysCostsTheSameRows pins the stack's fixed height: a run
// whose plan is not built yet still spends its step row, so the columns under it
// cannot move when the plan arrives.
func TestInstallingHeadAlwaysCostsTheSameRows(t *testing.T) {
	for _, m := range []Model{
		{},
		{Steps: []InstallStep{{Name: "Install Dependencies"}}, CurrentStep: 0},
	} {
		if got := len(m.installingHeadRows(76)); got != installingHeadRows {
			t.Errorf("the stack spends %d rows, want the fixed %d", got, installingHeadRows)
		}
	}
	for _, row := range (Model{}).installingHeadRows(76) {
		if lipgloss.Width(row) > 76 {
			t.Errorf("a row of the stack is %d columns in a 76-column composition: %q", lipgloss.Width(row), row)
		}
	}
}

// TestInstallingStepCounterStaysInsideThePlan pins the counter's bounds: a model
// whose current step is past the plan cannot index a step that is not there, which
// is what a run that finished its last step looks like.
func TestInstallingStepCounterStaysInsideThePlan(t *testing.T) {
	m := Model{Steps: []InstallStep{{Name: "only step"}}, CurrentStep: 7}
	if got := m.installingStepText(); !strings.Contains(got, "Step 1 of 1") {
		t.Errorf("a step index past the plan reads %q, want the last step", got)
	}
	if got := strconv.Itoa(len(m.Steps)); got != "1" {
		t.Fatalf("test setup: %s", got)
	}
}
