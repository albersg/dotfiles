package tui

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/albersg/dotfiles/installer/internal/netdiag"
	tea "github.com/charmbracelet/bubbletea"
)

// networkDiagnosticsCaseName is the name the frame guards know the network
// diagnosis screen by.
const networkDiagnosticsCaseName = "network-diagnostics"

// netdiagTestReport is a report with one failure in the transport and one
// undetermined check, which is the shape the screen has to explain: the verdict
// names the stage, the failing row carries its evidence and its manual command,
// and the steps that answered read as the healthy ones they were.
func netdiagTestReport() netdiag.Report {
	return netdiag.Report{
		Target:   "api.github.com",
		Endpoint: "https://api.github.com/",
		Source:   "the host this installer already contacts for its own update check",
		Started:  time.Date(2026, time.October, 16, 12, 4, 5, 0, time.UTC),
		Elapsed:  1420 * time.Millisecond,
		Checks: []netdiag.Check{
			{ID: "env.interfaces", Stage: netdiag.StageLocal, Title: "Interfaces and addresses",
				Status: netdiag.StatusOK, Evidence: []string{"enp3s0 is up: 192.168.1.42/24"},
				Duration: 120 * time.Microsecond},
			{ID: "env.route", Stage: netdiag.StageLocal, Title: "Default route",
				Status: netdiag.StatusOK, Evidence: []string{"the default route is 192.168.1.1 through enp3s0"}},
			{ID: "dns.resolve", Stage: netdiag.StageResolver, Title: "Resolve api.github.com",
				Status: netdiag.StatusOK,
				Evidence: []string{
					"the system resolver answered 140.82.121.6",
					"proxy http://alice:s3cr3t@proxy.corp:3128 is configured",
				},
				Duration: 22 * time.Millisecond},
			{ID: "tcp.connect", Stage: netdiag.StageTransport, Title: "Connect to api.github.com:443",
				Status:   netdiag.StatusFail,
				Evidence: []string{"the connection to 140.82.121.6:443 was refused"},
				Cause:    "nothing is listening on that port",
				Next:     "check the port the service publishes",
				Manual:   "nc -vz api.github.com 443",
				Duration: 41 * time.Millisecond},
			{ID: "tls.handshake", Stage: netdiag.StageCertificate, Title: "TLS handshake",
				Status: netdiag.StatusSkipped, Evidence: []string{"skipped: no connection was established"}},
			{ID: "http.get", Stage: netdiag.StageApplication, Title: "HTTP request",
				Status:   netdiag.StatusUnknown,
				Evidence: []string{"the HTTP check did not run"},
				Cause:    "the transport did not answer"},
		},
	}
}

// networkDiagnosticsFrameCase builds the diagnosis screen carrying a finished
// report, because the guard has to measure the rows the report produces -- one
// per check -- rather than the empty screen a run that dialled nothing would.
func networkDiagnosticsFrameCase(t *testing.T) Model {
	t.Helper()

	m := installerFrameModel(t, ScreenNetworkDiagnostics)
	report := netdiagTestReport()
	m.Network.Report = &report
	m.Network.Input = "api.github.com"
	m.Network.ReportPath = "/home/testuser/.local/state/dotfiles/network-report-20261016T120405Z.txt"
	m.Network.Notice = "The run finished. Move the cursor onto a step to read what it measured."
	m.Cursor = 0
	return m
}

// networkPressRow puts the cursor on a row by its label and presses Enter. The
// label is how a user finds the row, so a test that pressed an index would pass
// while the list was ordered differently than the screen shows it.
func networkPressRow(t *testing.T, m Model, row string) (Model, tea.Cmd) {
	t.Helper()

	found := -1
	for i, option := range m.GetCurrentOptions() {
		if option == row {
			found = i
			break
		}
	}
	if found < 0 {
		t.Fatalf("the row %q is not offered: %v", row, m.GetCurrentOptions())
	}
	m.Cursor = found
	next, cmd := m.handleNetworkKeys("enter")
	return next.(Model), cmd
}

// TestScreenNetworkDiagnosticsNamesTheStageAndTheEvidence is the screen's own
// guard: the verdict names the stage that stopped it, the counts account for
// every step, and the failing step carries its evidence, its cause, the next
// check and the command to run by hand.
func TestScreenNetworkDiagnosticsNamesTheStageAndTheEvidence(t *testing.T) {
	m := networkDiagnosticsFrameCase(t)
	m.Width, m.Height = 200, 60

	view := ansiEscape.ReplaceAllString(m.View(), "")
	flat := strings.Join(strings.Fields(view), " ")

	for _, want := range []string{
		"Network diagnostics",          // the screen's own title
		"the transport did not answer", // the verdict, by stage
		"6 checks: 3 ok, 0 noted, 1 failed, 1 undetermined, 1 skipped in 1.42 s",
		"Destination: https://api.github.com/", // the safe endpoint, never the query
		"Where it stopped: the transport",      // the fault, named
		"nothing is listening on that port",    // the cause of the failing step
		"check the port the service publishes", // the next check
		"nc -vz api.github.com 443",            // the manual command
		"[failed] transport — Connect to api.github.com:443",
		"[ok] local — Interfaces and addresses",
		"[skipped] certificate — TLS handshake",
		"[unknown] application — HTTP request",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("the diagnosis screen does not show %q:\n%s", want, view)
		}
	}

	// The step's own detail is reachable by putting the cursor on its row, which
	// is what makes a report longer than the frame readable at all.
	row := networkCheckRow(m.networkChecks()[3])
	if row == "" {
		t.Fatal("the failing check produced no row")
	}
	m.Cursor = indexOfRow(t, m, row)
	detail := strings.Join(strings.Fields(ansiEscape.ReplaceAllString(m.View(), "")), " ")
	for _, want := range []string{
		"the transport — Connect to api.github.com:443",
		"Status: failed.",
		"Took: 41 ms.",
		"the connection to 140.82.121.6:443 was refused",
		"By hand: nc -vz api.github.com 443",
	} {
		if !strings.Contains(detail, want) {
			t.Errorf("the step's own detail does not show %q:\n%s", want, detail)
		}
	}
}

// indexOfRow is the cursor position of a row, for the tests that need to put the
// cursor on it directly.
func indexOfRow(t *testing.T, m Model, row string) int {
	t.Helper()

	for i, option := range m.GetCurrentOptions() {
		if option == row {
			return i
		}
	}
	t.Fatalf("the row %q is not offered: %v", row, m.GetCurrentOptions())
	return 0
}

// TestNetworkScreenValidatesTheDestinationBeforeAnythingIsDialled pins the rule
// the whole utility rests on: a destination that cannot be diagnosed is refused
// with a reason, and nothing is dialled for it.
func TestNetworkScreenValidatesTheDestinationBeforeAnythingIsDialled(t *testing.T) {
	m := networkDiagnosticsFrameCase(t)
	m.Network.Report = nil
	m.Network.Input = "ftp://example.com"
	m.Network.InputActive = false

	next, cmd := networkPressRow(t, m, networkRunRow)
	m = next

	if cmd != nil {
		t.Error("a refused destination still produced a command, so something could have been dialled")
	}
	if m.Network.Running {
		t.Error("the screen says a run is in flight for a destination it refused")
	}
	if !m.Network.InputActive {
		t.Error("the field was not reopened, so the reason has nowhere to be corrected")
	}
	if !strings.Contains(m.Network.InputErr, "only http and https") {
		t.Errorf("the refusal is %q, want it to name what was wrong with the input", m.Network.InputErr)
	}
	if !strings.Contains(m.Network.Notice, "nothing was dialled") {
		t.Errorf("the notice is %q, want it to say nothing was dialled", m.Network.Notice)
	}
}

// TestNetworkScreenStartsOneRunAndKeepsItsDestination pins the wiring of a run:
// the row returns the command, the screen says a run is in flight, a second
// press starts no second run, and the destination that was typed is kept so it
// can be corrected rather than retyped.
func TestNetworkScreenStartsOneRunAndKeepsItsDestination(t *testing.T) {
	m := networkDiagnosticsFrameCase(t)
	m.Network.Report = nil
	m.Network.Input = "example.com:8443"

	next, cmd := networkPressRow(t, m, networkRunRow)
	m = next
	if cmd == nil {
		t.Fatal("the run row returned no command, so no check would ever run")
	}
	if !m.Network.Running {
		t.Error("the screen does not report a run in flight")
	}
	if m.Network.InputActive {
		t.Error("the field stays open while the run starts, so a keypress would edit a run already going")
	}
	if m.Network.Input != "example.com:8443" {
		t.Errorf("the destination became %q, want it kept for the next run", m.Network.Input)
	}
	if notice := m.networkNoticeLine(); !strings.Contains(notice, "Running the checks") {
		t.Errorf("the notice is %q, want it to say what is running", notice)
	}

	next, second := networkPressRow(t, m, networkRunRow)
	m = next
	if second != nil {
		t.Error("a second press while a run was in flight started a second one")
	}
}

// TestNetworkScreenKeepsWhatFinishedWhenTheRunIsStopped pins the promise Escape
// makes: the run is cancelled, the report still lands, and the screen says the
// remaining steps were skipped rather than pretending the run completed.
func TestNetworkScreenKeepsWhatFinishedWhenTheRunIsStopped(t *testing.T) {
	run := &networkRun{events: make(chan tea.Msg, networkEventBuffer)}
	run.ctx, run.cancel = context.WithCancel(context.Background())

	m := networkDiagnosticsFrameCase(t)
	m.Network.Report = nil
	m.Network.Running = true
	m.Network.Run = run

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if run.ctx.Err() == nil {
		t.Error("Escape did not cancel the run")
	}
	if m.Screen != ScreenNetworkDiagnostics {
		t.Errorf("Escape left the screen (%v); the first press belongs to the run it stopped", m.Screen)
	}
	if !strings.Contains(m.Network.Notice, "Stopping the run") {
		t.Errorf("the notice is %q, want it to say the run is stopping", m.Network.Notice)
	}

	// The report that arrives afterwards carries the checks that finished and the
	// steps that were skipped, and the screen says the run was stopped.
	stopped := netdiagTestReport()
	stopped.Stopped = true
	next, _ = m.Update(networkDoneMsg{report: stopped, stopped: true})
	m = next.(Model)
	if m.Network.Running {
		t.Error("the screen still reports a run in flight after the report landed")
	}
	if m.Network.Report == nil {
		t.Fatal("the stopped run's report was dropped")
	}
	if !strings.Contains(m.Network.Notice, "was stopped") {
		t.Errorf("the notice is %q, want it to say the run was stopped", m.Network.Notice)
	}

	// A second Escape leaves the screen, which is the way back the legend names.
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if m.Screen != ScreenUtilities {
		t.Errorf("Escape left the screen on %v, want ScreenUtilities", m.Screen)
	}
}

// TestNetworkScreenWritesOnlyFromThePreviewAndOnlyWhatItShows pins the consent
// the export runs under: the save row is not offered before the report has been
// read, the write goes into the installer's own state directory, and the file is
// exactly the text that was reviewed -- with the credentials masked.
func TestNetworkScreenWritesOnlyFromThePreviewAndOnlyWhatItShows(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	m := networkDiagnosticsFrameCase(t)
	m.Network.ReportPath = networkReportPath(m.Network.Report.Started)

	for _, row := range m.GetCurrentOptions() {
		if row == networkSaveRow {
			t.Fatal("the save row is offered before the report has been reviewed")
		}
	}

	next, cmd := networkPressRow(t, m, networkPreviewRow)
	m = next
	if cmd != nil {
		t.Error("opening the preview returned a command; it reads nothing")
	}
	if !m.Network.Preview {
		t.Fatal("the preview row did not open the preview")
	}
	previewed := m.Network.ReportText
	if previewed == "" {
		t.Fatal("opening the preview built no text to show")
	}
	if !strings.Contains(previewed, "the transport did not answer") {
		t.Errorf("the preview does not carry the report:\n%s", previewed)
	}
	if strings.Contains(previewed, "s3cr3t") || strings.Contains(previewed, "alice") {
		t.Errorf("the preview carried a proxy credential:\n%s", previewed)
	}
	if !strings.Contains(previewed, "http://***:***@proxy.corp:3128") {
		t.Errorf("the preview does not show the masked proxy:\n%s", previewed)
	}

	next, _ = networkPressRow(t, m, networkSaveRow)
	m = next
	if !strings.Contains(m.Network.Notice, "Saved") {
		t.Fatalf("the save row did not report a write: %q", m.Network.Notice)
	}

	saved, err := os.ReadFile(m.Network.ReportPath)
	if err != nil {
		t.Fatalf("the report was not written to %s: %v", m.Network.ReportPath, err)
	}
	if string(saved) != previewed {
		t.Errorf("the file differs from the text that was reviewed:\n--- file ---\n%s\n--- preview ---\n%s", saved, previewed)
	}
	if !strings.HasPrefix(m.Network.ReportPath, os.Getenv("XDG_STATE_HOME")) {
		t.Errorf("the report was written outside the installer's own state directory: %s", m.Network.ReportPath)
	}
	info, err := os.Stat(m.Network.ReportPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != networkReportMode {
		t.Errorf("the report was written with mode %v, want %v: it names this machine's addresses",
			info.Mode().Perm(), os.FileMode(networkReportMode))
	}
}

// TestNetworkScreenAnonymizesWhatItSaves pins the option the support case needs:
// with it on, the addresses and the names are gone from the file that is written.
func TestNetworkScreenAnonymizesWhatItSaves(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	m := networkDiagnosticsFrameCase(t)
	m.Network.ReportPath = networkReportPath(m.Network.Report.Started)

	// The rows are how a reader does this, and pressing them is also what builds the
	// text the write uses: the preview first, then the toggle, then the save.
	for _, row := range []string{networkPreviewRow, networkAnonymizeRow} {
		next, _ := networkPressRow(t, m, row)
		m = next
	}
	if !m.Network.Anonymize {
		t.Fatal("the anonymize row did not turn anonymization on")
	}

	next, _ := networkPressRow(t, m, networkSaveRow)
	m = next

	saved, err := os.ReadFile(m.Network.ReportPath)
	if err != nil {
		t.Fatalf("the report was not written: %v", err)
	}
	for _, secret := range []string{"192.168.1.42", "140.82.121.6", "s3cr3t", "alice"} {
		if strings.Contains(string(saved), secret) {
			t.Errorf("the anonymized report still contains %q:\n%s", secret, saved)
		}
	}
	for _, want := range []string{"<ipv4>", "the transport did not answer"} {
		if !strings.Contains(string(saved), want) {
			t.Errorf("the anonymized report lost %q:\n%s", want, saved)
		}
	}
}

// TestNetworkScreenSaysWhatAnonymizationKeeps pins the promise the copy makes. The
// anonymized form is safe to send because it drops this machine's names and every
// address; the host being diagnosed stays, and a reader has to be told that before
// the write rather than discover it afterwards.
func TestNetworkScreenSaysWhatAnonymizationKeeps(t *testing.T) {
	m := networkDiagnosticsFrameCase(t)
	m.Network.ReportPath = networkReportPath(m.Network.Report.Started)

	for _, row := range []string{networkPreviewRow, networkAnonymizeRow} {
		next, _ := networkPressRow(t, m, row)
		m = next
	}

	body := strings.Join(strings.Fields(strings.Join(m.networkDescription(), " ")), " ")
	if !strings.Contains(body, "The host being diagnosed is kept") {
		t.Errorf("the anonymized preview does not say what it keeps:\n%s", body)
	}
}

// TestNetworkScreenReportsWhenThereIsNowhereToSave pins the honest gap: a machine
// with no state directory offers the review without the save, and says why
// instead of failing at the press.
func TestNetworkScreenReportsWhenThereIsNowhereToSave(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", "")

	m := networkDiagnosticsFrameCase(t)
	m.Network.ReportPath = networkReportPath(m.Network.Report.Started)
	if m.Network.ReportPath != "" {
		t.Skipf("this host resolves a state directory anyway (%s), so the no-directory state cannot be built here", m.Network.ReportPath)
	}

	m.Network.Preview = true
	m.Network.Anonymize = false
	m.Network.Preview = true
	body := strings.Join(m.networkDescription(), " ")
	if !strings.Contains(body, "no state directory") {
		t.Errorf("the preview does not say why it cannot be saved:\n%s", body)
	}

	// The write is asked for directly, because the state this test builds has no
	// save row to press: the row is offered only when there is a file to write.
	m.saveNetworkReport()
	if !strings.Contains(m.Network.Notice, "no state directory") {
		t.Errorf("the notice is %q, want it to name the missing directory", m.Network.Notice)
	}
}

// TestUtilitiesSectionOpensTheDiagnosisWithoutDialling pins the other end of the
// wiring: choosing the row lands on the screen and starts nothing, because the
// checks are a choice made on that screen.
func TestUtilitiesSectionOpensTheDiagnosisWithoutDialling(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenUtilities
	m.Cursor = indexOfRow(t, m, utilitiesNetworkRow)

	next, cmd := m.handleUtilitiesKeys("enter")
	m = next.(Model)
	if m.Screen != ScreenNetworkDiagnostics {
		t.Fatalf("selecting the row landed on %v, want ScreenNetworkDiagnostics", m.Screen)
	}
	if cmd != nil {
		t.Error("opening the screen started a run")
	}
	if m.Network.Running {
		t.Error("the screen says a run is in flight before its own row was pressed")
	}
}

// TestNetworkScreenSaysItChangesNothingAndWhatItReachesFor pins the two promises
// the intro makes before anything runs: the checks are bounded and read-only, and
// the destination an overview uses is named rather than hidden.
func TestNetworkScreenSaysItChangesNothingAndWhatItReachesFor(t *testing.T) {
	m := installerFrameModel(t, ScreenNetworkDiagnostics)
	m.Width, m.Height = 200, 60

	body := strings.Join(strings.Fields(strings.Join(m.networkDescription(), " ")), " ")
	for _, want := range []string{
		"Nothing is changed",
		"no verification is skipped",
		"no tool is installed",
		"the TCP handshake's, never a ping's",
		"api.github.com", // the host the overview reaches for, named
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the intro does not say %q:\n%s", want, body)
		}
	}
}

// TestNetworkDiagnosticsFrameFitIsMeasuredAtEverySize measures the screen at the
// twelve terminals the fit guard uses, with the rows a finished report produces.
// It is the check that a report of six steps still fits a small terminal: the
// list windows around the cursor, so the number of checks is not bounded by the
// height of the frame.
func TestNetworkDiagnosticsFrameFitIsMeasuredAtEverySize(t *testing.T) {
	for _, size := range measuredTerminalSizes {
		m := networkDiagnosticsFrameCase(t)
		m.Width, m.Height = size.width, size.height
		view := m.View()

		plain := ansiEscape.ReplaceAllString(view, "")
		if strings.Contains(plain, "%!") {
			t.Errorf("at %dx%d the screen renders an unstyled format verb:\n%s", size.width, size.height, plain)
		}
		assertScreenFitsTerminal(t, networkDiagnosticsCaseName, size.width, size.height, view)

		// The two rows that act are at the top of the list, and the way back is the
		// legend's Esc: the list itself windows around the cursor, so a small frame
		// shows a slice of it and the note beside the header names what is not shown.
		flat := strings.Join(strings.Fields(plain), " ")
		for _, want := range []string{networkRunRow, "[Esc] back"} {
			if !strings.Contains(flat, want) {
				t.Errorf("at %dx%d the screen does not show %q:\n%s", size.width, size.height, want, view)
			}
		}
	}
}
