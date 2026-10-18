package tui

import (
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// The Utilities section: the obligations its own documentation and its own
// floor are held to
// ---------------------------------------------------------------------------
//
// Two guards an independent verifier asked for after reading this front. Both
// cover the same failure mode in different places: a claim about the section that
// nothing measured, so the claim could drift from the section and stay wrong
// until a user noticed. The job-count sentence had already drifted once before
// this front, and drifted again inside it -- the sentence said five jobs while
// the section offered six, omitting the shell's startup the same file documents
// two sections further down.

// TestTheUtilitiesDocNamesEveryJobTheSectionOffers holds the section's one
// job-count sentence to the jobs the section actually offers.
//
// The sentence is the only place the doc states how many jobs there are, and it
// is prose: it cannot be derived from `utilitiesPanelEntries`, so nothing but a
// guard keeps it true. `utilitiesPanelEntries` derives the panel, which is why a
// job cannot be missing *there* without a test failing; the sentence had no such
// protection, and the count and the list were both wrong in the same sentence --
// "five jobs" naming five of six.
//
// The jobs are asserted by the phrase the doc gives each one, so the guard reads
// what a user reads rather than an internal identifier.
//
// The count moved to eight when the WSL performance advisor joined the section, and
// to nine when the package update center did -- this branch's job, merged after the
// advisor. It is the section's own destination for the machine's package managers,
// so the sentence names it and this guard reads it the same way rather than leaving
// the sentence one job short of what the section offers. Merging this front with the
// disk cleaner moved it twice more: to ten for the **SSH configuration**, which the
// sentence had been one job short of since that section landed, and to eleven for the
// **developer disk cleaner**, which is the job this front added. Both are read by name
// below, because a count that no phrase backs is the drift this guard keeps finding.
func TestTheUtilitiesDocNamesEveryJobTheSectionOffers(t *testing.T) {
	doc := readRepoFile(t, filepath.Join("docs", "tui-installer.md"))

	start := strings.Index(doc, "It holds ")
	if start < 0 {
		t.Fatal("docs/tui-installer.md no longer states the section's job count, so this guard proves nothing")
	}
	end := strings.Index(doc[start:], ".")
	if end < 0 {
		t.Fatal("the job-count sentence has no end, so this guard cannot read it")
	}
	// The doc is hard-wrapped at 80 columns, so a phrase can carry a newline
	// inside it: the sentence is compared with its whitespace folded, or a job
	// that is named across a line break would read as a job that is missing.
	sentence := strings.Join(strings.Fields(doc[start:start+end]), " ")

	if !strings.Contains(sentence, "twelve jobs") {
		t.Errorf("the sentence no longer says the section holds twelve jobs: %q", sentence)
	}
	for _, job := range []string{
		"system theme switch",
		"dotfiles-theme",
		"WSL resources",
		"shell's startup",
		"terminal capability report",
		"install check",
		"network diagnosis",
		"package update center",
		"WSL performance advisor",
		"SSH configuration",
		"developer disk cleaner",
		"development environment's diagnosis",
	} {
		if !strings.Contains(sentence, job) {
			t.Errorf("the section's job-count sentence does not name %q: %q", job, sentence)
		}
	}
}

// TestUtilitiesBodyCarriesEveryJobAtTheFloor holds the 80x24 body to the same
// rule, in the frame rather than in prose.
//
// `TestUtilitiesGolden` pins the same frame, but a golden is regenerated when it
// changes: a job pushed past the body's cut would arrive as "… and N more" in a
// golden someone accepted, and the golden would then agree with the defect. This
// guard reads the frame's own text instead, so the cut marker and the missing job
// fail it whether or not a golden was regenerated around them. It is the guard
// that caught the fifth job's line being cut when this front's utility landed.
func TestUtilitiesBodyCarriesEveryJobAtTheFloor(t *testing.T) {
	m := NewModel()
	isolateGoldenTest(t, &m)
	m.Width, m.Height = 80, 24
	m.Screen = ScreenUtilities
	m.Animating = false
	// The desktop switch is forced away so the body's own sentences are the
	// non-switch ones, and the shell's startup is forced present so the job that
	// has been omitted twice is in the frame this guard reads.
	m.ThemeSwitch, m.ThemeSwitchFound = themeSwitch{}, false
	m.ShellAudit = shellAuditTestState()
	m.TerminalCapabilities = terminalCapabilitiesFrameCase(t).TerminalCapabilities
	m.UpdateCheck = updateState{}

	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	m.DotfilesThemes = defs

	view := ansiEscape.ReplaceAllString(m.View(), "")
	for _, job := range []string{
		utilitiesThemeRow,
		"WSL resources",
		utilitiesShellAuditRow,
		utilitiesVerifyRow,
		utilitiesTerminalRow,
		// The update center's row is the one this guard was widened for: it is the job
		// the merged section gained, and the body's floor budget is what its line had to
		// fit inside, so the row that cost the line is read by name here.
		utilitiesUpdatesRow,
		// The network diagnosis's body line yielded to the port manager's row, so the
		// row is what still names the job at the floor; the guard reads it here.
		utilitiesNetworkRow,
		// The port and process manager's row, added with the job itself: the guard
		// exists so a job cannot be named on a screen no one reads at the floor.
		utilitiesPortRow,
		// The disk cleaner's row came in with the other front's merge: its line is the
		// one that yields to the section's own body (a row of budget, as the update
		// center's did before it), so that job is read by the name the list gives it
		// rather than by a line the floor cuts.
		utilitiesDiskCleanerRow,
	} {
		if !strings.Contains(view, job) {
			t.Errorf("the 80x24 section does not name the %q job:\n%s", job, view)
		}
	}
	if strings.Contains(view, "… and ") {
		t.Errorf("the 80x24 section carries a cut marker, so a job's line was pushed past the cut:\n%s", view)
	}
}
