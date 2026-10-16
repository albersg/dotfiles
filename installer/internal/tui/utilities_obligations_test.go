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

	if !strings.Contains(sentence, "seven jobs") {
		t.Errorf("the sentence no longer says the section holds seven jobs: %q", sentence)
	}
	for _, job := range []string{
		"system theme switch",
		"dotfiles-theme",
		"WSL resources",
		"shell's startup",
		"terminal capability report",
		"install check",
		"network diagnosis",
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
	} {
		if !strings.Contains(view, job) {
			t.Errorf("the 80x24 section does not name the %q job:\n%s", job, view)
		}
	}
	if strings.Contains(view, "… and ") {
		t.Errorf("the 80x24 section carries a cut marker, so a job's line was pushed past the cut:\n%s", view)
	}
}
