package tui

import (
	"strings"
	"testing"
)

// ============================================================================
// THE TWO REPORTS IN THE GRAMMAR — the guards
// ============================================================================
//
// The capability reading and the verification draw their facts as the grammar's
// rows on a frame that holds the card (report_cards.go). These guards hold what
// that costs: every answer, source, path and outcome the report holds is still on
// the screen, the rows are the model's own values and not a second copy of them,
// and the 80x24 floor keeps the prose it always drew.

// TestTerminalCapabilitiesDrawTheAnswerRows is the capability report's data guard:
// one row per capability with its name as the title, its answer at the row's right
// edge, its source and consequence under both, the reason and the way to check by
// hand for an answer the probe could not determine -- and the state's mark in the
// gutter.
func TestTerminalCapabilitiesDrawTheAnswerRows(t *testing.T) {
	m := terminalCapabilitiesFrameCase(t)
	m.Width, m.Height = 160, 50
	m.Animating = false

	plain := cardText(m.View())
	for _, want := range []string{
		capabilityReadOnlyPromise,
		"✓ Colour depth", "truecolor (24-bit)",
		"✓ Clipboard (OSC 52)", "supported",
		"✗ Synchronized output (mode 2026)", "not supported",
		"? Nerd Font glyphs", "unknown",
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("the capability card does not state %q:\n%s", want, plain)
		}
	}
	// Every answer's own sentence is the model's: the source, the consequence and --
	// for an unknown answer -- the reason and the manual check, in the same words
	// the compact report prints.
	for _, row := range m.TerminalCapabilities.rows() {
		for _, want := range []string{row.Name, row.Answer.Value, row.Answer.Source, row.Implication} {
			if !strings.Contains(plain, squash(want)) {
				t.Errorf("the capability card does not state %q:\n%s", want, plain)
			}
		}
		if row.Answer.State == capabilityUnknown && row.Answer.Reason != "" {
			for _, want := range []string{row.Answer.Reason, row.Answer.Manual} {
				if want != "" && !strings.Contains(plain, squash(want)) {
					t.Errorf("the unknown answer does not carry %q:\n%s", want, plain)
				}
			}
		}
	}
}

// TestInstallVerifyDrawsTheFindingsAsRows is the verification's data guard: the run
// the record names, one row per finding with the machine's verdict in the gutter
// and the path as its title, what happened to the path or the comparison behind a
// file's finding on the row's muted line, and the count of what is still as the run
// left it.
func TestInstallVerifyDrawsTheFindingsAsRows(t *testing.T) {
	m := installVerifyFrameCase(t)
	m.Width, m.Height = 160, 50
	m.Animating = false

	verdict := m.InstallVerify.Verdict
	plain := cardText(m.View())
	for _, want := range []string{
		installVerifyReadOnlyPromise,
		"Record from " + verdict.Record.Timestamp.Format(panelBackupStamp) + " by " + verdict.Record.Version + ".",
		installVerifyMissingGroup, installVerifyChangedGroup,
		"2 of the 4 paths and backups the record names are not here:",
		"1 file(s) no longer hold what the run left:",
		"Still what the run left: 1 of the 4. dotfiles --verify prints every one of them.",
		// The way back is the one row the report offers, so the cursor is on it and
		// its arrow has given the gutter to the marker.
		"Back",
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("the verification card does not state %q:\n%s", want, plain)
		}
	}

	// Every finding the verdict holds is a row, with its own path, name and outcome.
	for _, finding := range verdict.Findings {
		if finding.Kind == findingPathPresent || finding.Kind == findingBackupPresent {
			continue
		}
		for _, want := range []string{finding.Path, finding.Key, installVerifyOutcome(finding)} {
			if !strings.Contains(plain, squash(want)) {
				t.Errorf("the verification card does not state %q:\n%s", want, plain)
			}
		}
		if finding.Detail != "" && !strings.Contains(plain, squash(finding.Detail)) {
			t.Errorf("the verification card does not state %q:\n%s", finding.Detail, plain)
		}
	}
}

// TestReportRowsAreTheModelsOwnFacts is the no-second-copy guard: the row shapes
// are built from the report's own values, so a change to an answer or a finding
// reaches the card without a second list to keep in step.
func TestReportRowsAreTheModelsOwnFacts(t *testing.T) {
	caps := terminalCapabilitiesFrameCase(t)
	caps.Width, caps.Height = 160, 50
	caps.Animating = false
	// A terminal whose answers differ from the fixture's must draw its own.
	caps.TerminalCapabilities.Color = terminalAnswer{State: capabilitySupported, Value: colorValue256, Source: "TERM=xterm-256color"}
	plain := cardText(caps.View())
	if !strings.Contains(plain, colorValue256) {
		t.Errorf("the capability card does not draw the model's own answer %q:\n%s", colorValue256, plain)
	}
	if strings.Contains(plain, colorValueTrueColor) {
		t.Errorf("the capability card draws an answer the model does not hold:\n%s", plain)
	}

	verify := installVerifyFrameCase(t)
	verify.Width, verify.Height = 160, 50
	verify.Animating = false
	verify.InstallVerify.Verdict.NoRecord = true
	verify.InstallVerify.Verdict.Record = nil
	verify.InstallVerify.Verdict.Findings = nil
	noRecord := cardText(verify.View())
	if !strings.Contains(noRecord, "No record of an install on this machine") {
		t.Errorf("the verification card does not draw the verdict the model holds:\n%s", noRecord)
	}
}

// TestReportsFallBackToTheCompactBodyAtTheFloor is the floor guard: the 80x24
// frame draws the section renderer's prose and no card, so the two shapes are told
// apart by the chrome and the compact report is unchanged.
func TestReportsFallBackToTheCompactBodyAtTheFloor(t *testing.T) {
	for _, frame := range []struct {
		name  string
		build func(*testing.T) Model
	}{
		{"terminal-capabilities", terminalCapabilitiesFrameCase},
		{"install-verify", installVerifyFrameCase},
	} {
		frame := frame
		t.Run(frame.name, func(t *testing.T) {
			m := frame.build(t)
			m.Width, m.Height = 80, 24
			m.Animating = false

			if sectionCardDrawn(m, m.GetScreenTitle()) {
				t.Fatalf("%s draws the card at the 80x24 floor:\n%s", frame.name, ansiEscape.ReplaceAllString(m.View(), ""))
			}
			plain := ansiEscape.ReplaceAllString(m.View(), "")
			if !strings.Contains(plain, "← Back") {
				t.Errorf("%s has no way back at the floor:\n%s", frame.name, plain)
			}
		})
	}
}

// TestTheReportCardDoesNotAdvertiseAWindowItDoesNotDraw is the vital guard: a
// report has no list to window, so its header must not claim a range of rows --
// the report screen printed "Showing 1-0 of 1" while drawing its way back.
func TestTheReportCardDoesNotAdvertiseAWindowItDoesNotDraw(t *testing.T) {
	for _, frame := range []struct {
		name  string
		build func(*testing.T) Model
	}{
		{"terminal-capabilities", terminalCapabilitiesFrameCase},
		{"install-verify", installVerifyFrameCase},
	} {
		frame := frame
		t.Run(frame.name, func(t *testing.T) {
			m := frame.build(t)
			m.Width, m.Height = 160, 50
			m.Animating = false

			plain := ansiEscape.ReplaceAllString(m.View(), "")
			if strings.Contains(plain, "Showing") {
				t.Errorf("%s advertises a window it does not draw:\n%s", frame.name, plain)
			}
		})
	}
}
