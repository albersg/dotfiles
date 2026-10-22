package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/albersg/dotfiles/installer/internal/system"
	"github.com/charmbracelet/lipgloss"
)

// ============================================================================
// The utilities section, from the mockup's fifth panel
// ============================================================================
//
// The guards here hold the screen to the two things the design owes a reader:
// every row of the mockup is drawn with its own description and nothing else's,
// and every job stays reachable at every measured terminal -- including the 80x24
// floor, where the two-line row is the shape that must give way first, never a
// row. The panel's guards hold it to the other rule the whole interface follows:
// it states what the model holds and nothing else.

// utilitiesFrameModel is the section as a terminal sees it: the fixture's machine,
// the theme definitions the repository ships and the shell audit resolved, so the
// rows and the panel are a fixed set rather than whatever the runner offers.
func utilitiesFrameModel(t *testing.T) Model {
	t.Helper()
	m := installerFrameModel(t, ScreenUtilities)
	m.Cursor = 0
	m.Animating = false
	m.ThemeSwitch, m.ThemeSwitchFound = themeSwitch{}, false
	m.WSLState = wslResourceState{Resolved: true, Available: false, Reason: "no .wslconfig"}
	m.ShellAudit = shellAuditTestState()
	m.TerminalCapabilities = terminalCapabilitiesFrameCase(t).TerminalCapabilities
	m.UpdateCheck = updateState{}
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	m.DotfilesThemes = defs
	return m
}

// utilitiesPlain is the frame with its escape sequences taken off, which is what
// a reader sees.
func utilitiesPlain(m Model) string {
	return ansiEscape.ReplaceAllString(m.View(), "")
}

// utilitiesBudget is the rows the section's body may spend, measured the way the
// renderer measures them: the frame minus its chrome and the footer it reserves.
func utilitiesBudget(m Model, l layout) int {
	hints := []installerHint{hintUp, hintDown, hintOpen, hintBack}
	if m.ThemeSwitchFound {
		hints = append(hints, hintDark, hintLight)
	}
	if m.themeUndoAvailable() {
		hints = append(hints, hintUndo)
	}
	return installerBodyRows(m.Height, m.footerReserveRows(l.Inner, m.panelHints(m.panelsFor(), hints)))
}

// TestUtilitiesRowsDrawTheMockupsNamesAndDescriptions is the design guard: every
// row the mockup draws is drawn here with the mockup's name and its one-line
// description, and the jobs this installer has that the mockup does not show
// carry a description in the same voice. A row with no entry in the drawing table
// fails here, so a row added to the section cannot arrive with no icon and no
// description while the guard passes by looking at the others.
func TestUtilitiesRowsDrawTheMockupsNamesAndDescriptions(t *testing.T) {
	m := utilitiesFrameModel(t)
	// A desktop switch and a WSL instance are forced on so the rows the mockup
	// does not show are drawn too: this guard covers the whole table, not the
	// subset the runner happens to offer.
	target, ok := themeSwitchByID("gnome")
	if !ok {
		t.Fatal("the theme switch table no longer holds the gnome entry")
	}
	m.ThemeSwitch, m.ThemeSwitchFound = target, true
	m.ThemeRecord = &themeRecord{Target: target.ID}
	m.DotfilesThemeRecord = &dotfilesThemeRecord{Theme: "dotfiles"}
	m.WSLState = wslResourceState{Resolved: true, Available: true}
	m.WSLRuntime = system.WSLRuntime{Inside: true, Version: 2}
	m.SystemInfo.IsWSL = true
	m.Width, m.Height = 200, 60

	rows := m.utilitiesRows()
	seen := map[string]utilitiesRow{}
	for _, row := range rows {
		if row.Divider {
			continue
		}
		if row.Icon == "" && row.Title != "" {
			t.Errorf("the row %q draws no icon, so the drawing table is missing it: %v", row.Label, utilitiesRowParts)
		}
		seen[row.Title] = row
	}

	// The mockup's own jobs, in the mockup's order, with the mockup's words.
	for _, want := range []struct{ title, subtitle string }{
		{utilitiesDoctorRow, "Check common issues"},
		{utilitiesWSLAdvisorRow, "Check Linux issues"},
		{utilitiesShellAuditRow, "Measure startup time"},
		{utilitiesUpdatesRow, "Check latest versions"},
		{utilitiesSSHRow, "Manage SSH keys"},
		{utilitiesNetworkRow, "Check connectivity"},
		{utilitiesPortRow, "Find an open port"},
		{utilitiesDiskCleanerRow, "Remove temporary files"},
	} {
		row, ok := seen[want.title]
		if !ok {
			t.Fatalf("the section does not draw the mockup's %q row: %v", want.title, seen)
		}
		if row.Subtitle != want.subtitle {
			t.Errorf("the row %q says %q, want the mockup's %q", want.title, row.Subtitle, want.subtitle)
		}
	}

	// The way back is drawn with the mockup's arrow in the gutter and its name
	// beside it, so the card ends the way the mockup's card does.
	last := rows[len(rows)-1]
	if last.Title != "Back" || last.Icon != utilitiesBackIcon {
		t.Errorf("the last row is %+v, want the way back with the arrow in its gutter", last)
	}

	// And the whole list is on screen at this size: the descriptions are drawn,
	// not just held in the table.
	plain := utilitiesPlain(m)
	for _, row := range rows {
		if row.Divider {
			continue
		}
		if !strings.Contains(plain, row.Title) {
			t.Errorf("the frame does not draw the row %q:\n%s", row.Title, plain)
		}
		if row.Subtitle != "" && !strings.Contains(plain, row.Subtitle) {
			t.Errorf("the frame does not draw %q's description %q:\n%s", row.Title, row.Subtitle, plain)
		}
	}
}

// TestUtilitiesRowGlyphsAreOneCell pins the glyph rule the other row tables are
// held to: every icon is one cell wide, so a terminal without the font draws the
// name where the name belongs instead of shifting the row by a column.
func TestUtilitiesRowGlyphsAreOneCell(t *testing.T) {
	for label := range utilitiesRowParts {
		row := utilitiesRowFor(utilitiesScreenRow(label))
		if row.Icon == "" {
			t.Errorf("the row %q has no icon", label)
			continue
		}
		if width := lipgloss.Width(row.Icon); width != 1 {
			t.Errorf("the row %q draws a %d-cell glyph %q, want one cell", label, width, row.Icon)
		}
	}
	for _, row := range []utilitiesRow{
		utilitiesRowFor("← Back"), utilitiesRowFor(utilitiesDarkRow), utilitiesRowFor(utilitiesLightRow),
	} {
		if lipgloss.Width(row.Icon) != 1 {
			t.Errorf("the row %q draws a %d-cell glyph %q, want one cell", row.Title, lipgloss.Width(row.Icon), row.Icon)
		}
	}
	if got := lipgloss.Width(utilitiesDiagnosisCheck); got != 1 {
		t.Errorf("the verdict's check is %d cells wide, want one", got)
	}
}

// TestUtilitiesCardDrawsOnlyTheRowsTheHandlerMatches pins the one row of the
// mockup the section does not draw. Its first row, "System Overview / Check
// environment status", is there because the mockup's right column is the selected
// row's detail; here the panel is contextual, so a row whose only job would be to
// show it would be a row no key can act on. Every drawn row is therefore an entry
// of the option list the key handler matches, and the overview is drawn as the
// panel's title instead.
func TestUtilitiesCardDrawsOnlyTheRowsTheHandlerMatches(t *testing.T) {
	m := utilitiesFrameModel(t)
	options := m.GetCurrentOptions()
	matched := make(map[string]bool, len(options))
	for _, option := range options {
		matched[option] = true
	}

	for _, row := range m.utilitiesRows() {
		if !matched[row.Label] {
			t.Errorf("the card draws %q, which is not an option the handler matches: %v", row.Label, options)
		}
		if row.Title == utilitiesOverviewPanelLabel {
			t.Errorf("the card draws the panel's own name as a row: %+v", row)
		}
	}
}

// TestUtilitiesTwoLineRowsAreTheMockupsAndCostWhatTheyMeasure checks the shape:
// the mockup's two-line row is drawn where every name and every description fit
// whole, and the one-line row replaces it where they do not. A description cut
// mid-word would be worse than no description, which is why the width decides and
// not the height.
func TestUtilitiesTwoLineRowsAreTheMockupsAndCostWhatTheyMeasure(t *testing.T) {
	m := utilitiesFrameModel(t)

	wide := m
	wide.Width, wide.Height = 200, 60
	wideLayout := layoutFor(wide)
	shape := wide.utilitiesShapeFor(utilitiesBudget(wide, wideLayout), wideLayout)
	if !shape.TwoLine || !shape.Boxed {
		t.Fatalf("the wide frame is drawn %+v, want the mockup's two-line rows in the card", shape)
	}
	for i, height := range shape.Heights {
		row := m.utilitiesRows()[i]
		if row.Divider {
			continue
		}
		want := 2
		if row.Subtitle == "" {
			want = 1
		}
		if height != want {
			t.Errorf("the row %q is drawn %d rows tall, want %d", row.Title, height, want)
		}
	}

	floor := m
	floor.Width, floor.Height = 80, 24
	floorLayout := layoutFor(floor)
	shape = floor.utilitiesShapeFor(utilitiesBudget(floor, floorLayout), floorLayout)
	t.Logf("floor shape: %+v budget=%d heights=%v", shape, utilitiesBudget(floor, floorLayout), shape.Heights)
	if shape.TwoLine {
		t.Errorf("the 80x24 floor is drawn with two-line rows, which spend the rows every job needs")
	}
	if !shape.Boxed {
		t.Errorf("the 80x24 floor lost the card, which fits at that size")
	}
}

// TestUtilitiesEveryJobStaysReachable walks the cursor over every row the section
// offers at every measured terminal: the row under the cursor is always drawn,
// and no frame draws a line wider than the terminal or a row taller than its
// height. It is the guard the design's one real risk needs -- a windowed list can
// hide the row it is supposed to be showing.
func TestUtilitiesEveryJobStaysReachable(t *testing.T) {
	base := utilitiesFrameModel(t)
	target, ok := themeSwitchByID("gnome")
	if !ok {
		t.Fatal("the theme switch table no longer holds the gnome entry")
	}
	// The tallest list the section can offer: a desktop's two switches, the undo,
	// the theme list, the WSL rows above one Level, and every job.
	base.ThemeSwitch, base.ThemeSwitchFound = target, true
	base.ThemeRecord = &themeRecord{Target: target.ID}
	base.DotfilesThemeRecord = &dotfilesThemeRecord{Theme: "dotfiles"}
	base.WSLState = wslResourceState{Resolved: true, Available: true}
	base.SystemInfo.IsWSL = true
	base.WSLRuntime = system.WSLRuntime{Inside: true, Version: 2}

	for _, size := range measuredTerminalSizes {
		m := base
		m.Width, m.Height = size.width, size.height
		options := m.GetCurrentOptions()
		for cursor := range options {
			m.Cursor = cursor
			view := m.View()
			assertScreenFitsTerminal(t, "utilities/"+size.name, size.width, size.height, view)

			plain := ansiEscape.ReplaceAllString(view, "")
			title := utilitiesRowFor(options[cursor]).Title
			if title == "" {
				continue
			}
			if !strings.Contains(plain, title) {
				t.Fatalf("at %s the row under the cursor (%q) is not on screen:\n%s", size.name, title, plain)
			}
		}
	}
}

// TestUtilitiesWindowNamesWhatItHid is the other half of the window: a list too
// tall for the frame says how many rows it kept out of sight, above and below,
// instead of ending where the screen ends.
func TestUtilitiesWindowNamesWhatItHid(t *testing.T) {
	m := utilitiesFrameModel(t)
	target, ok := themeSwitchByID("gnome")
	if !ok {
		t.Fatal("the theme switch table no longer holds the gnome entry")
	}
	m.ThemeSwitch, m.ThemeSwitchFound = target, true
	m.ThemeRecord = &themeRecord{Target: target.ID}
	m.DotfilesThemeRecord = &dotfilesThemeRecord{Theme: "dotfiles"}
	m.WSLState = wslResourceState{Resolved: true, Available: true}
	m.SystemInfo.IsWSL = true
	m.WSLRuntime = system.WSLRuntime{Inside: true, Version: 2}
	m.Width, m.Height = 80, 24
	m.Cursor = 0

	plain := utilitiesPlain(m)
	if !strings.Contains(plain, "more below") {
		t.Errorf("the floor does not say how many rows it hid below the window:\n%s", plain)
	}

	// With the cursor at the last row the note moves above it, and the row the
	// keys act on stays drawn.
	m.Cursor = len(m.GetCurrentOptions()) - 1
	plain = utilitiesPlain(m)
	if !strings.Contains(plain, "more above") {
		t.Errorf("the window at the last row does not say what it hid above:\n%s", plain)
	}
	if !strings.Contains(plain, "Back") {
		t.Errorf("the window hid the row under the cursor:\n%s", plain)
	}
}

// TestUtilitiesOverviewStatesTheFactsTheModelHolds pins the panel's facts against
// the model: what system.Detect filled is drawn, and a fact the installer does
// not read gets no row at all. Kernel and Uptime are the mockup's and not this
// program's, so the panel draws neither -- a row reading "unknown" would be the
// panel guessing.
func TestUtilitiesOverviewStatesTheFactsTheModelHolds(t *testing.T) {
	m := utilitiesFrameModel(t)
	m.SystemInfo = &system.SystemInfo{
		OS:         system.OSLinux,
		OSName:     "Debian/Ubuntu",
		Arch:       "x86_64",
		IsWSL:      true,
		WSLVersion: 2,
		HomeDir:    "/home/alberto",
		UserShell:  "zsh",
		HasBrew:    true,
	}
	m.DotfilesThemes = nil

	flat := ansiEscape.ReplaceAllString(strings.Join(m.utilitiesOverviewFacts(layout{Right: 80}), "\n"), "")
	for _, want := range []string{"Debian/Ubuntu", "WSL 2", "x86_64", "zsh", "brew", "/home/alberto"} {
		if !strings.Contains(flat, want) {
			t.Errorf("the overview does not state %q:\n%s", want, flat)
		}
	}
	for _, label := range []string{"OS", "Host", "Arch", "Shell", "Packages", "Home"} {
		if !strings.Contains(flat, label) {
			t.Errorf("the overview does not carry the %q row:\n%s", label, flat)
		}
	}
	for _, invented := range []string{"Kernel", "Uptime"} {
		if strings.Contains(flat, invented) {
			t.Errorf("the overview states a %q the model does not hold:\n%s", invented, flat)
		}
	}

	// A machine the model cannot name gets no row rather than a guess: the panel
	// of a model with no SystemInfo is empty, not a page of "unknown".
	empty := utilitiesFrameModel(t)
	empty.SystemInfo = nil
	if facts := empty.utilitiesOverviewFacts(layout{Right: 80}); len(facts) != 0 {
		t.Errorf("a model with no machine states %v, want nothing", facts)
	}
}

// TestUtilitiesResourcesFollowTheSamples pins the Resources block's honesty: a
// reading is drawn only where its own flag is set, and the block is absent while
// the sampler is gated off, because a stale sample drawn as a current reading is
// the lie the gate exists to prevent.
func TestUtilitiesResourcesFollowTheSamples(t *testing.T) {
	sample := system.Metrics{
		CPUOK:     true,
		CPUBusy:   0.41,
		MemOK:     true,
		MemUsed:   4 * 1024 * 1024 * 1024,
		MemTotal:  16 * 1024 * 1024 * 1024,
		DiskOK:    true,
		DiskFree:  300 * 1024 * 1024 * 1024,
		DiskTotal: 512 * 1024 * 1024 * 1024,
		ProcOK:    true,
		ProcCount: 311,
	}

	live := utilitiesFrameModel(t)
	live.Animating = true
	live.Metrics = []system.Metrics{sample}
	facts := live.utilitiesOverviewFacts(layout{Right: 80})
	flat := ansiEscape.ReplaceAllString(strings.Join(facts, "\n"), "")
	for _, want := range []string{"Resources", "CPU", "41%", "Memory", "4.0 GiB / 16.0 GiB", "Disk", "212.0 GiB / 512.0 GiB"} {
		if !strings.Contains(flat, want) {
			t.Errorf("the resources block does not state %q:\n%s", want, flat)
		}
	}
	// The block is the mockup's card: the heading on the top border and the meters
	// inside it, so the meters cannot read as rows of the facts above them.
	boxed := false
	for _, row := range facts {
		if strings.Contains(row, "╭─") && strings.Contains(row, utilitiesResourcesCardLabel) {
			boxed = true
		}
	}
	if !boxed {
		t.Errorf("the resources block is not drawn as the mockup's card:\n%s", flat)
	}
	// The process count is a reading the utilities panel does not draw: the mockup
	// asks for CPU, memory and disk, and a fourth meter would be a different panel.
	if strings.Contains(flat, "311") {
		t.Errorf("the resources block draws a reading the mockup does not ask for:\n%s", flat)
	}

	// A host that reports only memory draws memory and nothing dressed up as a
	// measurement beside it.
	partial := live
	partial.Metrics = []system.Metrics{{MemOK: true, MemUsed: 1024, MemTotal: 2048}}
	flat = ansiEscape.ReplaceAllString(strings.Join(partial.utilitiesOverviewFacts(layout{Right: 80}), "\n"), "")
	if !strings.Contains(flat, "Memory") {
		t.Errorf("the block dropped a reading the host does report:\n%s", flat)
	}
	for _, absent := range []string{"CPU", "Disk"} {
		if strings.Contains(flat, absent) {
			t.Errorf("the block states %q on a host that did not report it:\n%s", absent, flat)
		}
	}

	// The gate off means no sample is trusted, so no resource is stated.
	gated := live
	gated.Animating = false
	if facts := gated.utilitiesOverviewFacts(layout{Right: 80}); strings.Contains(strings.Join(facts, "\n"), "Resources") {
		t.Errorf("the block states a reading while the sampler is gated off: %v", facts)
	}
}

// TestUtilitiesOverviewSaysEverythingLooksGoodOnlyWhenTheDiagnosisSaysSo pins the
// verdict line to the doctor's own findings. Nothing else may print an all-clear:
// a run that has not happened, one still walking, one cancelled, one that left a
// check unanswered and one that found a warning are all states in which the
// machine has not been shown to be fine.
func TestUtilitiesOverviewSaysEverythingLooksGoodOnlyWhenTheDiagnosisSaysSo(t *testing.T) {
	finished := time.Date(2026, time.January, 4, 12, 0, 30, 0, time.UTC)
	clear := utilitiesCleanDoctorRun
	verdict := func(m Model) bool {
		return strings.Contains(strings.Join(m.utilitiesOverviewFacts(layout{Right: 80}), "\n"), "Everything looks good!")
	}

	withRun := utilitiesFrameModel(t)
	withRun.Doctor.Run = clear()
	if !verdict(withRun) {
		t.Error("a clean finished run does not draw the all-clear")
	}

	cases := []struct {
		name  string
		build func(model *Model)
	}{
		{name: "no run", build: func(m *Model) { m.Doctor.Run = nil }},
		{name: "a run still walking", build: func(m *Model) {
			m.Doctor.Run = clear()
			m.Doctor.Run.Finished = time.Time{}
		}},
		{name: "a cancelled run", build: func(m *Model) {
			m.Doctor.Run = clear()
			m.Doctor.Run.Canceled = true
		}},
		{name: "a check that never answered", build: func(m *Model) {
			m.Doctor.Run = clear()
			m.Doctor.Run.Order = append(m.Doctor.Run.Order, "tools")
		}},
		{name: "a warning", build: func(m *Model) {
			m.Doctor.Run = clear()
			m.Doctor.Run.Results["path"] = doctorCheckResult{
				ID: "path", Status: doctorWarning, Findings: []doctorFinding{{Status: doctorWarning}},
			}
		}},
		{name: "a run over nothing", build: func(m *Model) {
			m.Doctor.Run = &doctorRun{
				Results:  map[string]doctorCheckResult{},
				Started:  finished.Add(-time.Second),
				Finished: finished,
			}
		}},
	}
	for _, c := range cases {
		m := utilitiesFrameModel(t)
		c.build(&m)
		if verdict(m) {
			t.Errorf("the panel says everything looks good on %s", c.name)
		}
	}
}

// TestUtilitiesFloorKeepsTheCardAndTheFacts is the floor guard: at 80x24 the
// section is the card of jobs, the summary of the panel on the card's title and
// the result line under it, and every job it offers is drawn -- no row is cut and
// no job is pushed past the frame.
func TestUtilitiesFloorKeepsTheCardAndTheFacts(t *testing.T) {
	m := utilitiesFrameModel(t)
	m.Width, m.Height = 80, 24

	plain := utilitiesPlain(m)
	assertScreenFitsTerminal(t, "utilities", 80, 24, m.View())

	if !strings.Contains(plain, "System Overview") {
		t.Errorf("the floor draws no summary of the panel:\n%s", plain)
	}
	for _, want := range []string{"Debian/Ubuntu", "zsh"} {
		if !strings.Contains(plain, want) && !strings.Contains(plain, "Linux") {
			t.Errorf("the floor's summary does not state %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "… and ") {
		t.Errorf("the floor cut a block with the marker, so a row or a reason was pushed past it:\n%s", plain)
	}
	for _, option := range m.GetCurrentOptions() {
		row := utilitiesRowFor(option)
		if row.Divider {
			continue
		}
		if !strings.Contains(plain, row.Title) {
			t.Errorf("the floor does not draw the row %q:\n%s", row.Title, plain)
		}
	}
}

// utilitiesCleanDoctorRun is a finished diagnosis with nothing to report: two
// checks, both answered, every finding clean. It is the one state the panel's
// all-clear line is allowed to be drawn in.
func utilitiesCleanDoctorRun() *doctorRun {
	finished := time.Date(2026, time.January, 4, 12, 0, 30, 0, time.UTC)
	return &doctorRun{
		Order: []string{"path", "shells"},
		Results: map[string]doctorCheckResult{
			"path":   {ID: "path", Status: doctorOK, Findings: []doctorFinding{{Status: doctorOK}}},
			"shells": {ID: "shells", Status: doctorOK, Findings: []doctorFinding{{Status: doctorOK}}},
		},
		Started:  finished.Add(-30 * time.Second),
		Finished: finished,
	}
}

// TestTheThemeCardSpeaksForTheThemeTheRowOpens is the guard for the card that
// warned "Not available on this host": the desktop theme switch (GNOME's
// gsettings, KDE's tool, macOS's defaults) is absent under WSL, but the row above
// the card opens the installer's OWN theme picker, which works either way. The card
// denied the action it described. It states the picker's own state now, and the
// desktop switch is named in the section's own words rather than as a badge over
// the whole card.
func TestTheThemeCardSpeaksForTheThemeTheRowOpens(t *testing.T) {
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}

	noDesktop := utilitiesFrameModel(t)
	noDesktop.DotfilesThemes = defs
	noDesktop.Width, noDesktop.Height = 160, 50
	noDesktop.Animating = false
	plain := utilitiesPlain(noDesktop)

	if strings.Contains(plain, "Not available on this host") {
		t.Errorf("the theme card still denies the row above it:\n%s", plain)
	}
	if !anyOptionContains(noDesktop.GetCurrentOptions(), utilitiesThemeRow) {
		t.Fatalf("the theme row is not offered, so the card cannot be measured against it: %v", noDesktop.GetCurrentOptions())
	}
	if !strings.Contains(plain, themeCardCountSentence(defs)) {
		t.Errorf("the card does not state how many themes the picker offers (%q):\n%s", themeCardCountSentence(defs), plain)
	}
	// The desktop switch is a different fact, and the section's own sentence says so.
	flat := strings.Join(strings.Fields(plain), " ")
	if reason := noDesktop.utilitiesDesktopSwitchNote(); reason == "" {
		t.Error("the section does not name the desktop switch it could not find")
	} else if !strings.Contains(flat, strings.Join(strings.Fields(reason), " ")) {
		t.Errorf("the card does not carry the section's own reason %q:\n%s", reason, plain)
	}

	// With a desktop detected the card states the picker's state and says nothing
	// about a missing switch.
	target, ok := themeSwitchByID("gnome")
	if !ok {
		t.Fatal("the theme switch table no longer holds the gnome entry")
	}
	withDesktop := noDesktop
	withDesktop.ThemeSwitch, withDesktop.ThemeSwitchFound = target, true
	if note := withDesktop.utilitiesDesktopSwitchNote(); note != "" {
		t.Errorf("a detected desktop still draws the absence note %q", note)
	}
	if strings.Contains(utilitiesPlain(withDesktop), "No desktop theme switch") {
		t.Errorf("a detected desktop's card says no switch is available:\n%s", utilitiesPlain(withDesktop))
	}
}

// themeCardCountSentence is how the card writes the theme list's size, so the guard
// compares the card against the model's own count rather than a number typed here.
func themeCardCountSentence(defs []themeDefinition) string {
	return themeCountLabel(len(defs)) + " are read from the repository"
}
