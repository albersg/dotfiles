package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/golden"
)

// ---------------------------------------------------------------------------
// The utilities section: the machine against the record of the last install
// ---------------------------------------------------------------------------
//
// The installer writes a record when a run completes and already knows how to
// read the machine back against it -- `VerifyInstall` is what `dotfiles --verify`
// prints -- but that answer is only reachable by leaving the interface. The
// utility behind the section's own row is the second reader of the same
// verification: one walk of the record, two presenters. The guards below pin the
// offer, the facts on screen, the one-walk rule, the read-only promise and the
// wiring.

// installVerifyTestVerdict is a verification fixture built from a record rather
// than from the machine, so a guard measures the screen instead of whatever the
// runner happens to have installed. One path is still here, one is gone, and one
// backup is gone with its path still in place: the three facts the report exists
// to tell apart.
func installVerifyTestVerdict() installVerdict {
	record := &lastInstall{
		Timestamp: time.Date(2026, time.October, 15, 12, 24, 0, 0, time.UTC),
		Version:   "v0.9.0",
		Entries: []installEntry{
			{Key: "nvim", Path: "/home/testuser/.config/nvim", Action: installActionReplaced},
			{Key: "fish", Path: "/home/testuser/.config/fish", Action: installActionCreated},
			{
				Key:    "kitty",
				Path:   "/home/testuser/.config/kitty",
				Action: installActionReplaced,
				Backup: "/home/testuser/.dotfiles-backup-20261015-122400/kitty",
			},
		},
	}
	return installVerdict{
		Record: record,
		Findings: []installFinding{
			{Kind: findingPathPresent, Key: "nvim", Path: "/home/testuser/.config/nvim", Action: installActionReplaced},
			{Kind: findingPathMissing, Key: "fish", Path: "/home/testuser/.config/fish", Action: installActionCreated},
			{Kind: findingBackupMissing, Key: "kitty", Path: "/home/testuser/.dotfiles-backup-20261015-122400/kitty"},
			// A file the record can compare: the screen has to show a file that no
			// longer holds what the run left, which is the answer presence alone
			// could never give.
			{
				Kind:   findingFileChangedSinceTheRun,
				Key:    "zsh",
				Path:   "/home/testuser/.zshrc",
				Action: installActionReplaced,
				Detail: "expected 8f3a1c92b4de, found 1c07ba44e9d2",
			},
		},
	}
}

// installVerifyFrameCase builds the guarded screen with the fixture above, so
// the frame guard measures the screen carrying its rows rather than the reading
// state a runner with no record would produce.
func installVerifyFrameCase(t *testing.T) Model {
	t.Helper()

	m := installerFrameModel(t, ScreenInstallVerify)
	m.InstallVerify = installVerifyState{Resolved: true, Verdict: installVerifyTestVerdict()}
	return m
}

// installVerifyScreenText is the screen's body with its whitespace collapsed, so
// a guard reads a phrase a line break split as the one phrase a reader sees.
func installVerifyScreenText(m Model) string {
	return strings.Join(strings.Fields(strings.Join(m.installVerifyDescription(), " ")), " ")
}

// TestTheSectionOffersTheInstallVerification pins the offer and both halves of
// the section's own account: the row is there, the body names what it does, and
// the panel derives one entry for it. The row is offered everywhere -- there is
// always a machine to compare with the record, and where there is no record the
// report says exactly that -- so a runner with no state directory still gets it.
func TestTheSectionOffersTheInstallVerification(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenUtilities

	found := false
	for _, row := range m.GetCurrentOptions() {
		if row == utilitiesScreenRow(utilitiesVerifyRow) {
			found = true
		}
	}
	if !found {
		t.Fatalf("the utilities section does not offer %q: %v", utilitiesVerifyRow, m.GetCurrentOptions())
	}

	body := strings.Join(m.utilitiesDescription(), " ")
	if !strings.Contains(body, utilitiesVerifyRow) {
		t.Errorf("the section does not name the row it offers: %q", body)
	}
	// The row's own sentence is the one that is there, and it does not deny the
	// utility it offers: a sentence that says the verification is not available
	// would be the section contradicting its own list. The sentence is one row now
	// -- the section's body is measured against the 80x24 floor, and a wrapped line
	// pushed three jobs behind the cut marker -- and the claim it makes is the same
	// one: the report is about the paths that are gone.
	if !strings.Contains(body, "reports the paths that are gone") {
		t.Errorf("the section does not say what the verification reports: %q", body)
	}
	if strings.Contains(body, utilitiesVerifyRow+" is not") {
		t.Errorf("the section declared the verification unavailable while it offers the row: %q", body)
	}

	offered := map[string]string{}
	for _, entry := range m.utilitiesPanelEntries() {
		if entry.offered {
			offered[entry.label] = entry.value
		}
	}
	if _, ok := offered["Install check"]; !ok {
		t.Errorf("utilitiesPanelEntries does not offer the install check: %v", offered)
	}
}

// TestTheVerificationPanelValueComesFromTheModel is the panel's derivation
// guard: before the screen has been opened the panel says so rather than
// answering a question nobody asked, and once the verification has run the value
// is the verdict's own count. A fact typed into the panel cannot survive this.
func TestTheVerificationPanelValueComesFromTheModel(t *testing.T) {
	unchecked := NewModel()
	uncheckedFlat := utilitiesPanelFlat(t, unchecked)
	if !strings.Contains(uncheckedFlat, "Install check") || !strings.Contains(uncheckedFlat, "not checked yet") {
		t.Errorf("the panel does not say the check has not run:\n%s", uncheckedFlat)
	}

	checked := NewModel()
	checked.InstallVerify = installVerifyState{Resolved: true, Verdict: installVerifyTestVerdict()}
	checkedFlat := utilitiesPanelFlat(t, checked)
	if !strings.Contains(checkedFlat, "2 not here") {
		t.Errorf("the panel did not pick up the verdict's own count:\n%s", checkedFlat)
	}
	if strings.Contains(checkedFlat, "not checked yet") {
		t.Errorf("the panel still says the check has not run:\n%s", checkedFlat)
	}
	if uncheckedFlat == checkedFlat {
		t.Error("filling the verdict did not change the panel, so the row is not read from the model")
	}
}

// TestTheVerificationScreenShowsWhatIsGoneAndSaysWhatIsStillHere is the screen's
// own guard: the read-only promise, the limit the record holds -- it names no
// digest -- the run it is talking about, and then the paths that are not here
// with what the run did to each, beside the count that is still here.
func TestTheVerificationScreenShowsWhatIsGoneAndSaysWhatIsStillHere(t *testing.T) {
	m := installerFrameModel(t, ScreenInstallVerify)
	m.InstallVerify = installVerifyState{Resolved: true, Verdict: installVerifyTestVerdict()}
	m.Width, m.Height = 200, 60

	view := ansiEscape.ReplaceAllString(m.View(), "")
	flat := installVerifyScreenText(m)
	for _, want := range []string{
		"Read-only",                                  // the promise
		"is here or gone",                            // exactly what is checked about a path
		"compared against the digest it held",        // and about a file
		"carries no digest",                          // what the record cannot support
		"Record from 2026-10-15 12:24:00 by v0.9.0.", // the run it describes
		"2 of the 4 paths and backups",               // how many are not here
		"missing fish /home/testuser/.config/fish",   // the gone path, named
		"backup of kitty",                            // the gone copy of the replaced path
		"no longer holds what the run left",          // the file that changed, named
		"Still what the run left: 1 of the 4.",       // the count that is
		"dotfiles --verify prints every one of them", // where the whole list is
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("the verification screen does not show %q:\n%s", want, view)
		}
	}
	// The path that is still here is not listed as a problem: the screen names
	// what is gone, and the count covers the rest.
	if strings.Contains(flat, "missing nvim") {
		t.Errorf("the screen reported a path that is still here as missing:\n%s", view)
	}
	for _, forbidden := range []string{"Writing", "Write ", "Undo", "Delete"} {
		if strings.Contains(view, forbidden) {
			t.Errorf("the read-only screen mentions %q, so it reads as something it is not:\n%s", forbidden, view)
		}
	}
}

// TestTheVerificationScreenSaysWhenNothingIsGone pins the other outcome: a
// machine that matches the record is a result and it is said in words, not left
// to be inferred from an absence.
func TestTheVerificationScreenSaysWhenNothingIsGone(t *testing.T) {
	verdict := installVerifyTestVerdict()
	for i := range verdict.Findings {
		verdict.Findings[i].Kind = findingPathPresent
	}

	m := installerFrameModel(t, ScreenInstallVerify)
	m.InstallVerify = installVerifyState{Resolved: true, Verdict: verdict}
	m.Width, m.Height = 200, 60

	flat := installVerifyScreenText(m)
	if !strings.Contains(flat, "Every one of the 4 paths, copies and files") {
		t.Errorf("a machine that matches the record does not say so: %q", flat)
	}
	if strings.Contains(flat, "Not here") || strings.Contains(flat, "missing") {
		t.Errorf("a machine that matches the record still reports something missing: %q", flat)
	}
}

// TestTheVerificationScreenSaysWhatItCannotCheck pins the three states that are
// not a verdict: no record at all, a record that is here and cannot be read, and
// the read still in flight. Each says which one it is, and none of them invents a
// path count.
func TestTheVerificationScreenSaysWhatItCannotCheck(t *testing.T) {
	tests := []struct {
		name  string
		state installVerifyState
		want  string
	}{
		{
			"no record",
			installVerifyState{Resolved: true, Verdict: installVerdict{NoRecord: true}},
			"nothing to check yet",
		},
		{
			"a record that cannot be read",
			installVerifyState{Resolved: true, Verdict: installVerdict{
				Unreadable: true,
				Reason:     "parse /home/testuser/.local/state/dotfiles/last-install.json: unexpected end of JSON input",
			}},
			"could not be read",
		},
		{
			"a record that names no path",
			installVerifyState{Resolved: true, Verdict: installVerdict{
				Record: &lastInstall{Timestamp: time.Now(), Version: "v0.9.0"},
			}},
			"names no path",
		},
		{
			"the read in flight",
			installVerifyState{},
			"Reading the record",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			m := installerFrameModel(t, ScreenInstallVerify)
			m.InstallVerify = tt.state
			m.Width, m.Height = 200, 60

			flat := installVerifyScreenText(m)
			if !strings.Contains(flat, tt.want) {
				t.Errorf("the screen does not say %q: %q", tt.want, flat)
			}
			// Whatever the state, the promise holds: nothing is written.
			if !strings.Contains(flat, "Read-only") {
				t.Errorf("the screen dropped its read-only promise: %q", flat)
			}
		})
	}
}

// TestTheVerificationScreenIsReadOnly pins the only key the screen offers: there
// is nothing to select and nothing to write, so every key either does nothing or
// steps back to the utilities section.
func TestTheVerificationScreenIsReadOnly(t *testing.T) {
	m := installVerifyFrameCase(t)
	m.Screen = ScreenInstallVerify

	if got := m.GetCurrentOptions(); len(got) != 1 || !strings.Contains(got[0], "Back") {
		t.Fatalf("the verification screen offers %v, want only the way back", got)
	}

	next, cmd := m.handleInstallVerifyKeys("x")
	if cmd != nil {
		t.Error("an unrelated key armed a command on the read-only screen")
	}
	if !next.(Model).InstallVerify.Resolved {
		t.Error("a key cleared the verdict")
	}

	// Enter and Space are not second ways back: they open and toggle, and there is
	// nothing here to open or toggle.
	for _, key := range []string{"enter", " "} {
		next, cmd := m.handleInstallVerifyKeys(key)
		if cmd != nil {
			t.Errorf("%q armed a command on the read-only screen", key)
		}
		if got := next.(Model).Screen; got != ScreenInstallVerify {
			t.Errorf("%q left the read-only screen on %v; Esc is its only way back", key, got)
		}
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if got := next.(Model).Screen; got != ScreenUtilities {
		t.Errorf("Esc went to %v, want ScreenUtilities", got)
	}
}

// TestChoosingTheVerificationRowRunsTheCheckOffTheUpdateLoop pins the wiring and
// the bubbletea rule the section already follows: the row returns a command
// rather than reading the disk while the key handler runs, nothing is checked at
// startup, and the answer fills the state whole.
func TestChoosingTheVerificationRowRunsTheCheckOffTheUpdateLoop(t *testing.T) {
	fresh := NewModel()
	if fresh.InstallVerify.Resolved {
		t.Fatal("a fresh model already holds a verdict, so something read the record at startup")
	}

	m := NewModel()
	m.Screen = ScreenUtilities
	cursor := -1
	for i, row := range m.GetCurrentOptions() {
		if row == utilitiesScreenRow(utilitiesVerifyRow) {
			cursor = i
		}
	}
	if cursor < 0 {
		t.Fatalf("the section does not offer %q: %v", utilitiesVerifyRow, m.GetCurrentOptions())
	}
	m.Cursor = cursor

	next, cmd := m.handleUtilitiesKeys("enter")
	got := next.(Model)
	if got.Screen != ScreenInstallVerify {
		t.Fatalf("choosing the row landed on %v, want ScreenInstallVerify", got.Screen)
	}
	if got.InstallVerify.Resolved {
		t.Error("the record was read synchronously; the check must run off the update loop")
	}
	if cmd == nil {
		t.Fatal("choosing the row armed no command, so nothing would ever be checked")
	}

	// The command's answer fills the state whole, the way a measurement does.
	updated, _ := got.Update(cmd())
	if final := updated.(Model); !final.InstallVerify.Resolved {
		t.Error("the answer did not fill the verdict")
	}
}

// TestTheVerificationReadsTheVerdictRatherThanTheDiskAgain is the one-walk rule
// as a guard with teeth: the screen is handed findings that name a path nothing
// on this machine has ever seen, and it must show them. A screen that walked the
// record again, or stated a count of its own, would disagree with the verdict it
// was given -- which is the second walk this rule exists to prevent.
func TestTheVerificationReadsTheVerdictRatherThanTheDiskAgain(t *testing.T) {
	verdict := installVerdict{
		Record: &lastInstall{
			Timestamp: time.Date(2026, time.October, 15, 12, 24, 0, 0, time.UTC),
			Version:   "v0.9.0",
		},
		Findings: []installFinding{
			{Kind: findingPathMissing, Key: "ghost", Path: "/nowhere/at/all/ghost", Action: installActionCreated},
			{Kind: findingBackupMissing, Key: "ghost", Path: "/nowhere/at/all/ghost.bak"},
		},
	}

	m := installerFrameModel(t, ScreenInstallVerify)
	m.InstallVerify = installVerifyState{Resolved: true, Verdict: verdict}
	m.Width, m.Height = 200, 60

	flat := installVerifyScreenText(m)
	if !strings.Contains(flat, "/nowhere/at/all/ghost") {
		t.Errorf("the screen did not show the finding it was handed, so it read the machine again: %q", flat)
	}
	if !strings.Contains(flat, "2 of the 2 paths and backups") {
		t.Errorf("the screen did not take the count from the verdict it was handed: %q", flat)
	}

	// The command line reads the same findings: one walk, two presenters.
	report := verifyReportText(VerifyReport{
		Lines:      verifyLines(verdict),
		Consistent: verdict.Consistent(),
	})
	for _, want := range []string{"/nowhere/at/all/ghost", "missing  ghost"} {
		if !strings.Contains(report, want) {
			t.Errorf("the command line report does not read the same finding (%q):\n%s", want, report)
		}
	}
}

// TestInstallVerifyGolden pins the frame of the verification screen at the
// documented floor, on a fixture verdict rather than on the machine's own record:
// the state directory is a temporary one, so the snapshot can never read whatever
// the runner installed.
func TestInstallVerifyGolden(t *testing.T) {
	skipIfTermux(t)

	m := installerFrameModel(t, ScreenInstallVerify)
	m.Screen = ScreenInstallVerify
	m.Animating = false
	m.InstallVerify = installVerifyState{Resolved: true, Verdict: installVerifyTestVerdict()}

	golden.RequireEqual(t, []byte(m.View()))
}

// TestInstallVerifyWideGolden pins the verification in the reference grammar: the
// read-only promise under the chip, the run the record names, one row per finding
// with the machine's verdict in its gutter and the path as its title, and the
// count of what is still as the run left it -- with the way back under the card's
// own rule. The 80x24 floor is pinned by TestInstallVerifyGolden and is unchanged.
func TestInstallVerifyWideGolden(t *testing.T) {
	skipIfTermux(t)

	m := installerFrameModel(t, ScreenInstallVerify)
	m.Width, m.Height = 160, 50
	m.Screen = ScreenInstallVerify
	m.Animating = false
	m.InstallVerify = installVerifyState{Resolved: true, Verdict: installVerifyTestVerdict()}

	golden.RequireEqual(t, []byte(m.View()))
}
