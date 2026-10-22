package tui

import (
	"strings"
	"testing"
)

// planStepByID finds a step of a plan by its unit ID.
func planStepByID(steps []InstallStep, id string) (InstallStep, bool) {
	for _, step := range steps {
		if step.ID == id {
			return step, true
		}
	}
	return InstallStep{}, false
}

// TestEveryPlanStepCarriesItsOriginAndItsArea is the builder half of S2 and S5:
// the screen groups its rows by the area a step belongs to and says where each
// one came from, so the builder that schedules a step has to say both where the
// row will read them. It checks the full plan and a minimal one, because a step
// that only appears on one of the two is a step whose row could arrive blank.
func TestEveryPlanStepCarriesItsOriginAndItsArea(t *testing.T) {
	full := planFixtures()[0].fixtureModel().planOptions()

	for _, tc := range []struct {
		name string
		opts planOptions
	}{
		{"the full plan", full},
		{"a minimal plan", planOptions{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			steps := planFor(tc.opts)
			if len(steps) == 0 {
				t.Fatal("the plan is empty, so this guard proves nothing")
			}
			for _, step := range steps {
				if step.Origin == stepOriginUnknown {
					t.Errorf("step %q is on the plan without an origin, so its row cannot say where it came from", step.ID)
				}
				if step.Area == "" {
					t.Errorf("step %q is on the plan without an area, so the screen cannot group its row", step.ID)
				}
				// The value is the choice's, and only a choice has one: a step the
				// run or the machine requires, and an extra that was turned on,
				// have no value to show and a stray one would be a second answer.
				if step.Origin == stepOriginChoice && step.OriginValue == "" {
					t.Errorf("step %q was decided by a choice and does not say the value that choice took", step.ID)
				}
				if step.Origin != stepOriginChoice && step.OriginValue != "" {
					t.Errorf("step %q carries the value %q but no choice decided it", step.ID, step.OriginValue)
				}
			}
		})
	}
}

// TestAChoiceDecidedStepNamesTheValueTheChoiceTook holds the value to the choice
// it came from, so a row cannot say "your choice: kitty" while the plan installs
// Ghostty. The three named here are the choices that took a name; the three
// switches that took "on" are held by the plan golden and the extra origin.
func TestAChoiceDecidedStepNamesTheValueTheChoiceTook(t *testing.T) {
	opts := planFixtures()[0].fixtureModel().planOptions()
	steps := planFor(opts)

	want := map[string]string{
		"terminal": opts.Terminal,
		"shell":    opts.Shell,
		"wm":       opts.WindowMgr,
	}
	if len(want) == 0 {
		t.Fatal("no choice decided a step, so this guard proves nothing")
	}

	for id, value := range want {
		step, ok := planStepByID(steps, id)
		if !ok {
			t.Fatalf("the full plan has no %q step, so this guard cannot check its value", id)
		}
		if step.Origin != stepOriginChoice {
			t.Errorf("the %q step is %v, want a choice", id, step.Origin)
		}
		if step.OriginValue != value {
			t.Errorf("the %q step says its choice took %q, want %q", id, step.OriginValue, value)
		}
	}
}

// TestNoScheduledStepArrivesWithoutAnOriginOrAnArea closes the builder half for
// every host and every set of choices the installer supports, not only the two
// plans above: a step a planner can schedule and the builder forgets to label is
// a row the screen can only show as its own ID, which is the defect S5 exists to
// remove.
func TestNoScheduledStepArrivesWithoutAnOriginOrAnArea(t *testing.T) {
	checked := 0
	for _, info := range allSystemInfos() {
		for _, choices := range allUserChoices() {
			m := &Model{SystemInfo: info, Choices: choices, ExistingConfigs: []string{".zshrc"}}
			for _, step := range planFor(m.planOptions()) {
				checked++
				if step.Origin == stepOriginUnknown {
					t.Errorf("planning %+v %+v scheduled %q without an origin", info, choices, step.ID)
				}
				if step.Area == "" {
					t.Errorf("planning %+v %+v scheduled %q without an area", info, choices, step.ID)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no configuration scheduled a step, so this guard proves nothing")
	}
}

// planReviewStepRows is the step rows of a plan review, in the order the screen
// draws them, so a guard can compare the list to the plan without knowing how a
// row is drawn.
func planReviewStepRows(rows []planReviewRow) []planReviewRow {
	steps := make([]planReviewRow, 0, len(rows))
	for _, row := range rows {
		if row.Kind == planRowStep {
			steps = append(steps, row)
		}
	}
	return steps
}

// TestThePlanScreenShowsThePlanByAreaInTheRunsOrder is S2 and S5 through the
// screen: the rows are the plan's steps, in the plan's order, each saying where
// it came from line by line, and the areas are named once, in the order the run
// meets them. It also checks the rendered frame, so the list the model builds
// and the list the user reads cannot disagree.
func TestThePlanScreenShowsThePlanByAreaInTheRunsOrder(t *testing.T) {
	m := planFixtures()[0].fixtureModel()
	m.Screen = ScreenInstallReview
	// A single-column width wide enough for the longest row, so the frame draws
	// every row whole and this guard compares the plan's text, not a cut of it.
	m.Width, m.Height = 100, 60

	steps := planFor(m.planOptions())
	rows := m.planReviewRows()

	// Every step exactly once, in the plan's order.
	var gotIDs []string
	for _, row := range planReviewStepRows(rows) {
		gotIDs = append(gotIDs, row.StepID)
	}
	if got, want := strings.Join(gotIDs, ","), planIDList(steps); got != want {
		t.Fatalf("the screen's step rows are %s, want the plan's %s", got, want)
	}

	// Every step's row says where it came from: a unit the run cannot leave out
	// says so (fixity is the catalogue's Fixed flag, not an origin), and every
	// other row shows its provenance sentence and the value a choice took.
	for _, row := range planReviewStepRows(rows) {
		step, ok := planStepByID(steps, row.StepID)
		if !ok {
			t.Fatalf("the screen draws a row for %q, which the plan does not schedule", row.StepID)
		}
		if installUnitIsFixed(step.ID) {
			if !strings.Contains(row.Text, fixedUnitClaim) {
				t.Errorf("the row for %q does not say the run needs it: %q", step.ID, row.Text)
			}
			continue
		}
		if sentence := step.Origin.sentence(); sentence == "" || !strings.Contains(row.Text, sentence) {
			t.Errorf("the row for %q does not say where it came from (%q): %q", step.ID, sentence, row.Text)
		}
		if step.OriginValue != "" && !strings.Contains(row.Text, step.OriginValue) {
			t.Errorf("the row for %q does not carry the value its choice took (%q): %q", step.ID, step.OriginValue, row.Text)
		}
	}

	// The areas appear once each, in the order the plan meets them, before the
	// first step of the area.
	var wantAreas []string
	for _, step := range steps {
		if len(wantAreas) == 0 || wantAreas[len(wantAreas)-1] != step.Area {
			wantAreas = append(wantAreas, step.Area)
		}
	}
	var gotAreas []string
	stepsSeen := 0
	for _, row := range rows {
		switch row.Kind {
		case planRowHeading:
			gotAreas = append(gotAreas, row.Text)
		case planRowStep:
			if len(gotAreas) == 0 {
				t.Fatalf("the step %q is drawn before any area heading", row.StepID)
			}
			if gotAreas[len(gotAreas)-1] != steps[stepsSeen].Area {
				t.Fatalf("the step %q sits under the heading %q, want %q", row.StepID, gotAreas[len(gotAreas)-1], steps[stepsSeen].Area)
			}
			stepsSeen++
		}
	}
	if strings.Join(gotAreas, "|") != strings.Join(wantAreas, "|") {
		t.Errorf("the screen names the areas %v, want %v", gotAreas, wantAreas)
	}

	// And the rendered frame draws the built rows in that same order. The card puts a
	// row's name on its title line and where it came from on the muted line under it,
	// so the guard matches the row's own name here rather than the one-line
	// concatenation the compact shape used to draw.
	plain := ansiEscape.ReplaceAllString(m.renderInstallReview(), "")
	at := 0
	for _, row := range rows {
		name := installReviewRowName(row)
		found := strings.Index(plain[at:], name)
		if found < 0 {
			t.Fatalf("the frame does not draw the row %q:\n%s", name, plain)
		}
		at += found + len(name)
	}

	// And every row fits the narrowest terminal the frame matrix measures: the frame
	// draws a row, it does not wrap one, so a row wider than the content is read as a
	// row that says something else -- which is how the set rows were found drawing a
	// sentence the frame cut in half.
	width := contentWidth(Model{Width: 80})
	for _, row := range rows {
		if len(row.Text) > width {
			t.Errorf("the row %q is %d characters and the narrowest measured terminal leaves %d", row.Text, len(row.Text), width)
		}
	}
}

// TestAPluginSetRowSitsUnderTheStepThatCarriesIt is S3's screen half: the set a
// step provisions is drawn directly under that step, saying either the switch's
// state or, for a set a run cannot leave out, where the set comes from.
func TestAPluginSetRowSitsUnderTheStepThatCarriesIt(t *testing.T) {
	m := planFixtures()[0].fixtureModel()
	m.Screen = ScreenInstallReview

	steps := planFor(m.planOptions())
	rows := m.planReviewRows()

	found := 0
	for i, row := range rows {
		if row.Kind != planRowStep {
			continue
		}
		step, ok := planStepByID(steps, row.StepID)
		if !ok || step.PluginSet == nil {
			continue
		}
		found++
		if i+1 >= len(rows) || rows[i+1].Kind != planRowSet {
			t.Errorf("the %s step carries the set %q and no set row follows it", step.ID, step.PluginSet.ID)
			continue
		}
		setRow := rows[i+1]
		if !strings.Contains(setRow.Text, step.PluginSet.Name) {
			t.Errorf("the set row under %q does not name its set %q: %q", step.ID, step.PluginSet.Name, setRow.Text)
		}

		if step.PluginSet.Switchable {
			if !strings.Contains(setRow.Text, "[x]") {
				t.Errorf("the set row under %q does not read as installed while its switch is on: %q", step.ID, setRow.Text)
			}
			// Turning the set off on the setup screen has to move the row: the row
			// is the switch's state, not a copy of the default.
			m.Choices.DisabledPluginSets = []string{step.PluginSet.ID}
			off := m.planReviewRows()[i+1]
			if !strings.Contains(off.Text, "[ ]") {
				t.Errorf("the set row under %q still reads as installed after the switch was turned off: %q", step.ID, off.Text)
			}
			m.Choices.DisabledPluginSets = nil
			continue
		}

		// The registry must still carry the reason -- it is what `--skip` refuses with
		// and what the documentation explains -- even though the row says it short.
		if step.PluginSet.Because == "" {
			t.Errorf("the %s step carries a set a run cannot leave out and the registry has no reason for it", step.ID)
		}
		if !strings.Contains(setRow.Text, pluginSetProvenance) {
			t.Errorf("the set row under %q does not say where the set comes from (%q): %q", step.ID, pluginSetProvenance, setRow.Text)
		}
		// The row is drawn, not wrapped, so a phrase longer than the frame is cut off
		// exactly where the user reads it. This is the defect that produced the short
		// phrase: the full reason was longer than the row, and the guard below that
		// renders the screen reported the row as never drawn.
		if width := contentWidth(Model{Width: 80}); len(setRow.Text) > width {
			t.Errorf("the set row under %q is %d characters and the narrowest measured terminal leaves %d: %q", step.ID, len(setRow.Text), width, setRow.Text)
		}
	}
	if found == 0 {
		t.Fatal("no step of the full plan carries a plugin set, so this guard proves nothing")
	}
}

// TestTheCursorVisitsEverySwitchableRowAndNothingElse is the keyboard's honesty:
// Space toggles the unit under the cursor, so the cursor may rest only on rows it
// can switch. Walking the whole list must reach every switchable step and never
// land on a heading, a set row or a step the run needs.
func TestTheCursorVisitsEverySwitchableRowAndNothingElse(t *testing.T) {
	m := planFixtures()[0].fixtureModel()
	m.Screen = ScreenInstallReview
	m.Width, m.Height = 80, 60
	m.snapUnitCursor()

	rows := m.planReviewRows()
	switchable := map[int]bool{}
	for i, row := range rows {
		if row.Kind == planRowStep && row.Switchable {
			switchable[i] = true
		}
	}
	if len(switchable) == 0 {
		t.Fatal("the full plan has no switchable step, so this guard proves nothing")
	}
	if !switchable[m.UnitCursor] {
		t.Fatalf("the cursor starts on row %d (%q), which is not a switchable step", m.UnitCursor, rows[m.UnitCursor].Text)
	}

	visited := map[int]bool{m.UnitCursor: true}
	for i := 0; i < len(rows); i++ {
		m.moveUnitCursor(1)
		row := m.planReviewRows()[m.UnitCursor]
		if row.Kind != planRowStep || !row.Switchable {
			t.Fatalf("moving the cursor landed on %q (kind %v, switchable %v), which Space cannot switch", row.Text, row.Kind, row.Switchable)
		}
		visited[m.UnitCursor] = true
	}

	for i := range switchable {
		if !visited[i] {
			t.Errorf("the cursor never reached the switchable row %d (%q)", i, rows[i].Text)
		}
	}
}

// TestARowClaimsTheRunNeedsItsUnitIfAndOnlyIfTheUnitIsFixed is the invariant the
// current tree violates: eight rows say "the run needs it" for units the cursor
// can switch off, so one phrase carries both "cannot leave it out" and "happens to
// be planned" (S5). It runs over every host and every choice, because a step that
// appears in only one plan is exactly the one nobody checks.
func TestARowClaimsTheRunNeedsItsUnitIfAndOnlyIfTheUnitIsFixed(t *testing.T) {
	checked := 0
	for _, info := range allSystemInfos() {
		for _, choices := range allUserChoices() {
			m := &Model{SystemInfo: info, Choices: choices, ExistingConfigs: []string{".zshrc"}}
			for _, step := range planFor(m.planOptions()) {
				checked++
				row := installReviewStepRow(step)
				claims := strings.Contains(row, fixedUnitClaim)
				if claims == installUnitIsFixed(step.ID) {
					continue
				}
				if claims {
					t.Errorf("the row for %q claims the run needs it, but the cursor can switch that unit off: %q", step.ID, row)
				} else {
					t.Errorf("the row for %q does not say the run needs it, but %q cannot be turned off: %q", step.ID, step.ID, row)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no configuration scheduled a step, so this guard proves nothing")
	}
}

// TestTheRenderedRowClaimsTheRunNeedsItsUnitIfAndOnlyIfTheUnitIsFixed is the same
// invariant through the rows the screen actually draws, for a full plan and a
// minimal one: the sentence a row shows is a claim about the unit, and it must
// match the catalogue's Fixed flag rather than the reason the step was scheduled.
func TestTheRenderedRowClaimsTheRunNeedsItsUnitIfAndOnlyIfTheUnitIsFixed(t *testing.T) {
	for _, tc := range []struct {
		name string
		m    *Model
	}{
		{"the full plan", planFixtures()[0].fixtureModel()},
		{"a minimal plan", &Model{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.m.Screen = ScreenInstallReview
			rows := planReviewStepRows(tc.m.planReviewRows())
			if len(rows) == 0 {
				t.Fatal("the plan has no step rows, so this guard proves nothing")
			}
			for _, row := range rows {
				claims := strings.Contains(row.Text, fixedUnitClaim)
				if claims == installUnitIsFixed(row.StepID) {
					continue
				}
				if claims {
					t.Errorf("the row for %q claims the run needs it, but the cursor can switch that unit off: %q", row.StepID, row.Text)
				} else {
					t.Errorf("the row for %q does not say the run needs it, but %q cannot be turned off: %q", row.StepID, row.StepID, row.Text)
				}
			}
		})
	}
}

// TestEveryProvenanceSentenceIsOneOfFourAndNoTwoShareOne holds the vocabulary
// itself: four honest provenances, four distinct sentences, and no fifth hiding
// in the zero value. Two origins sharing a sentence would read as the same
// reason, and an unknown origin with a sentence would make an unlabelled step
// look labelled.
func TestEveryProvenanceSentenceIsOneOfFourAndNoTwoShareOne(t *testing.T) {
	origins := []stepOrigin{stepOriginChoice, stepOriginExtra, stepOriginMachine, stepOriginAlways}
	known := map[string]bool{}
	for _, origin := range origins {
		sentence := origin.sentence()
		if sentence == "" {
			t.Errorf("origin %d has no sentence", origin)
			continue
		}
		if known[sentence] {
			t.Errorf("two origins share the sentence %q", sentence)
		}
		known[sentence] = true
	}
	if len(known) != len(origins) {
		t.Errorf("the four origins produced %d distinct sentences, want %d", len(known), len(origins))
	}
	if sentence := stepOriginUnknown.sentence(); sentence != "" {
		t.Errorf("the unknown origin has the sentence %q, so an unlabelled step would read as labelled", sentence)
	}

	checked := 0
	for _, info := range allSystemInfos() {
		for _, choices := range allUserChoices() {
			m := &Model{SystemInfo: info, Choices: choices, ExistingConfigs: []string{".zshrc"}}
			for _, step := range planFor(m.planOptions()) {
				checked++
				if !known[step.Origin.sentence()] {
					t.Errorf("the %q step carries the sentence %q, which is not one of the four provenances", step.ID, step.Origin.sentence())
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no configuration scheduled a step, so this guard proves nothing")
	}
}
