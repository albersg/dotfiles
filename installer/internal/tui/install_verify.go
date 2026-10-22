package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// ============================================================================
// THE UTILITIES SECTION: IS THIS MACHINE STILL WHAT THE LAST INSTALL LEFT?
// ============================================================================
//
// The installer writes a record when a run completes: one entry per configuration
// path, naming what the run did to it and where the copy it took went. That record
// was already read back against the machine, but only by the command line's
// `--verify`, so a person who ran the installer had to leave the interface to ask
// whether the machine is still what the run left.
//
// This screen is the second reader of the same verification, not a second
// verification. InspectInstall walks the record once; the command line's report
// and this screen are two presenters of the facts it returns. That is the whole
// reason the walk was lifted out of VerifyInstall: two walks of one record can
// disagree about the same file, and only one of them would be right.
//
// Everything here is reading and reporting. No configuration file is opened, no
// copy is restored, and no path is removed: the screen names what is gone and
// leaves the decision to the user.

// installVerifyState is what the screen draws: whether the check has run, and the
// verdict it produced. The check itself fills it, so a render never touches the
// filesystem.
type installVerifyState struct {
	// Resolved reports whether the check has run at least once.
	Resolved bool
	// Verdict is what comparing this machine with the record found.
	Verdict installVerdict
}

// installVerifyMsg carries a finished verdict back to the update loop.
type installVerifyMsg struct{ verdict installVerdict }

// installVerifyCmd reads the record and compares this machine with it. It is a
// command rather than a step in the key handler because it states the machine's
// paths, and a keystroke must not wait on a filesystem that is not answering --
// the same rule the capability report and the shell audit follow.
func installVerifyCmd() tea.Cmd {
	return func() tea.Msg { return installVerifyMsg{verdict: InspectInstall()} }
}

// installVerifyCmdIfNeeded arms the check once, when the screen is opened. The
// check is read-only: there is no draft to protect and no write to refresh from,
// so the first answer is the answer, and coming back to the screen does not stat
// the machine a second time.
func (m Model) installVerifyCmdIfNeeded() tea.Cmd {
	if m.InstallVerify.Resolved {
		return nil
	}
	return installVerifyCmd()
}

// installVerifyDescription is what the screen says: the read-only promise and the
// limit the record carries, the run it is talking about, the paths and copies
// that are no longer here with what the run did to each, and the count of what is
// still here -- which is the number the user came for and is cheaper to read than
// the list it stands for.
//
// The whole list lives on the command line and the screen says so rather than
// printing it: a record written by a normal run names every configuration path the
// installer touches, so the list is longer than any frame, and the prose that gives
// way to it would be cut on exactly the machines a user most wants an answer on.
func (m Model) installVerifyDescription() []string {
	paragraphs := []string{installVerifyReadOnlyPromise}

	if !m.InstallVerify.Resolved {
		return append(paragraphs, "Reading the record of the last install and checking this machine against it…")
	}

	verdict := m.InstallVerify.Verdict
	switch {
	case verdict.NoRecord:
		return append(paragraphs,
			"No record of an install on this machine: the installer writes one when a run completes, so "+
				"there is nothing to check yet.")
	case verdict.Unreadable:
		return append(paragraphs, "The record is here and could not be read: "+verdict.Reason)
	}

	paragraphs = append(paragraphs, fmt.Sprintf("Record from %s by %s.",
		verdict.Record.Timestamp.Format(panelBackupStamp), verdict.Record.Version))

	// A record that names no path is neither a pass nor a failure: there is
	// nothing the record and the machine can be compared on, and saying "every one
	// of the zero paths is here" would read as a pass nobody established.
	if len(verdict.Findings) == 0 {
		return append(paragraphs,
			"The record names no path, so there is nothing to compare with this machine.")
	}

	missing := 0
	changed := 0
	for _, finding := range verdict.Findings {
		switch {
		case finding.Kind.Missing():
			missing++
		case finding.Kind.ChangedSinceTheRun():
			changed++
		}
	}

	if missing == 0 && changed == 0 {
		return append(paragraphs, fmt.Sprintf(
			"Every one of the %d paths, copies and files the record names is here and holds what the run left.",
			len(verdict.Findings)))
	}

	if missing > 0 {
		paragraphs = append(paragraphs, fmt.Sprintf(
			"%d of the %d paths and backups the record names are not here:", missing, len(verdict.Findings)))
		for _, finding := range verdict.Findings {
			if finding.Kind.Missing() {
				paragraphs = append(paragraphs, installVerifyRowLine(finding))
			}
		}
	}

	// A file that changed after the install is not a path that is gone, and it is
	// not a pass either. It is the difference between "the machine is here" and
	// "the machine is what the install left", and this screen answers the second.
	if changed > 0 {
		paragraphs = append(paragraphs, fmt.Sprintf("%d file(s) no longer hold what the run left:", changed))
		for _, finding := range verdict.Findings {
			if finding.Kind.ChangedSinceTheRun() {
				paragraphs = append(paragraphs, installVerifyRowLine(finding))
			}
		}
	}

	paragraphs = append(paragraphs, fmt.Sprintf(
		"Still what the run left: %d of the %d. dotfiles --verify prints every one of them.",
		len(verdict.Findings)-missing-changed, len(verdict.Findings)))
	return paragraphs
}

// installVerifyRowLine renders one finding the machine no longer has as the row
// the screen shows: the outcome, what the record named, the path, and -- for a
// path rather than a copy -- what the run did to it. The outcome is the first
// column so the rows read as a list of one thing, the way the command line's own
// report is aligned.
func installVerifyRowLine(finding installFinding) string {
	switch {
	case finding.Kind.Copy():
		return fmt.Sprintf("missing  backup of %s  %s", finding.Key, finding.Path)
	case finding.Kind.Missing():
		return fmt.Sprintf("missing  %s  %s  %s", finding.Key, finding.Path, verifyActionPhrase(finding.Action))
	case finding.Kind == findingFileUnreadable:
		return fmt.Sprintf("unknown  %s  %s  the file could not be read", finding.Key, finding.Path)
	default:
		row := fmt.Sprintf("changed  %s  %s  no longer holds what the run left", finding.Key, finding.Path)
		if finding.Detail != "" {
			row += ": " + finding.Detail
		}
		return row
	}
}

// renderInstallVerify draws the verification screen: what it checked, what the
// record cannot say, and which paths are gone. It is the same body shape as the
// other read-only report in the section, and the only key is the way back.
func (m Model) renderInstallVerify() string {
	return m.renderThemeScreen(Model.installVerifyDescription, "",
		[]installerHint{hintBack})
}

// handleInstallVerifyKeys drives the verification screen. It only ever changes the
// screen, and the only key that does is the way back its footer names: there is
// nothing to select and nothing to write, so every other key does nothing. Enter is
// not a second way back -- it opens, and there is nothing here to open.
func (m Model) handleInstallVerifyKeys(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.Screen = ScreenUtilities
		m.Cursor = 0
	}
	return m, nil
}

// installVerifyPanelValue is what the Utilities panel says the verification row
// holds. It reads the model and nothing else: the panel is drawn on every
// keypress, and reading the record there would stat the machine behind a
// keystroke. Before the check has run it says so rather than answering with a
// fact nobody established, which is the same rule the theme list follows.
func installVerifyPanelValue(st installVerifyState) string {
	if !st.Resolved {
		return "not checked yet"
	}

	verdict := st.Verdict
	switch {
	case verdict.NoRecord:
		return "no record of an install"
	case verdict.Unreadable:
		return "the record cannot be read"
	}
	if len(verdict.Findings) == 0 {
		return "nothing to check"
	}
	if missing := verdict.Missing(); missing > 0 {
		return fmt.Sprintf("%d not here", missing)
	}
	if changed := verdict.Divergent(); changed > 0 {
		return fmt.Sprintf("%d changed since the install", changed)
	}
	return fmt.Sprintf("all %d as the run left them", len(verdict.Findings))
}
