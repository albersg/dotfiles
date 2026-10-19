package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// ============================================================================
// The doctor's screen
// ============================================================================
//
// These tests drive the screen the way a person does: a key in, a message back.
// The scan behind the run is always the fake machine, so nothing here reads or
// changes the machine the tests run on, and the two writes the screen can make
// -- the report and a fix -- land in a state directory of the test's own.

// doctorScreenModel is a model parked on the doctor with nothing run yet.
func doctorScreenModel(t *testing.T) Model {
	t.Helper()
	m := installerFrameModel(t, ScreenDoctor)
	m.Cursor = 0
	return m
}

// doctorScreenRunModel is the same model mid-run or after one: the run covers the
// checks it is given and reads the fake machine.
func doctorScreenRunModel(t *testing.T, host doctorHost, ids ...string) Model {
	t.Helper()
	m := doctorScreenModel(t)
	checks := make([]doctorCheck, 0, len(ids))
	order := make([]string, 0, len(ids))
	for _, id := range ids {
		check := doctorCheckForTest(t, id)
		checks = append(checks, check)
		order = append(order, check.ID)
	}
	m.Doctor.NextRunID = 1
	m.Doctor.Run = &doctorRun{
		ID:      1,
		Scan:    doctorTestScan(host, m.SystemInfo),
		Order:   order,
		Plan:    checks,
		Results: map[string]doctorCheckResult{},
		Started: time.Date(2026, time.January, 4, 12, 0, 0, 0, time.UTC),
	}
	m.Doctor.Run.Message = m.doctorProgressLine()
	return m
}

// doctorRunToTheEnd feeds the run's own messages back into the model until it
// finishes, which is what the update loop does in the application. The runtime
// runs every command in its own goroutine and expands a batch into the commands
// it holds; these tests run them one at a time instead, which is one of the
// orders a concurrent walk can take, so the walk is exercised without any test
// depending on which check wins a race. Commands already handed to somebody who
// threw them away can be passed in.
func doctorRunToTheEnd(t *testing.T, m Model, pending ...tea.Cmd) Model {
	t.Helper()
	if len(pending) == 0 && m.Doctor.Run != nil && m.Doctor.Run.running() {
		pending = append(pending, m.doctorScheduleCmd())
	}
	for step := 0; step < 4096; step++ {
		if m.Doctor.Run == nil || !m.Doctor.Run.running() {
			return m
		}
		if len(pending) == 0 {
			t.Fatalf("the run stopped before it finished: %d of %d answered",
				m.Doctor.Run.Answered, len(m.Doctor.Run.Plan))
		}
		cmd := pending[0]
		pending = pending[1:]
		if cmd == nil {
			continue
		}
		message := cmd()
		if batch, ok := message.(tea.BatchMsg); ok {
			pending = append(pending, batch...)
			continue
		}
		next, follow := m.Update(message)
		m = next.(Model)
		if follow != nil {
			pending = append(pending, follow)
		}
	}
	t.Fatal("the run did not finish in 4096 steps")
	return m
}

// TestDoctorScreenOffersEveryRunBeforeAnyRunIsStarted is the first thing the
// screen has to do: let the user choose a full run or one category, and say so
// without having read anything yet.
func TestDoctorScreenOffersEveryRunBeforeAnyRunIsStarted(t *testing.T) {
	m := doctorScreenModel(t)

	var labels []string
	for _, row := range m.doctorList() {
		if !row.separator {
			labels = append(labels, row.label)
		}
	}
	joined := strings.Join(labels, "\n")
	if !strings.Contains(joined, doctorRunAllRow) {
		t.Errorf("the list does not offer a full run:\n%s", joined)
	}
	for _, category := range doctorCategories() {
		if !strings.Contains(joined, category.row()) {
			t.Errorf("the list does not offer the category %q:\n%s", category.row(), joined)
		}
	}
	if strings.Contains(joined, doctorWriteReportRow) {
		t.Errorf("the list offers to write a report before anything was checked:\n%s", joined)
	}

	// The frame draws exactly the rows the key handler reads, so a drawn row
	// cannot be one the handler does not know. The separators are rows too.
	rows := m.doctorRows()
	if len(rows) != len(m.doctorList()) {
		t.Errorf("the frame draws %d rows and the list has %d", len(rows), len(m.doctorList()))
	}
}

// TestDoctorScreenSaysHowManyChecksAreDoneOutOfHowMany pins the progress line:
// the screen states how many checks have answered out of how many, because a run
// over a slow machine must be legible rather than frozen. It counted starts
// while the walk was one check per message; it counts answers now, because a
// pool with several checks out has finished none of them.
func TestDoctorScreenSaysHowManyChecksAreDoneOutOfHowMany(t *testing.T) {
	host := newDoctorFakeHost()
	host.addDir("/usr/bin")
	host.addDir("/bin")
	host.setenv("PATH", "/usr/bin:/bin")
	m := doctorScreenRunModel(t, host, "path.entries", "env.home")

	if m.Doctor.Run.Message == "" {
		t.Fatal("a run in flight says nothing about what it is doing")
	}
	if !strings.Contains(m.Doctor.Run.Message, "Checked 0 of 2") {
		t.Errorf("the progress line does not count the checks that have answered: %q", m.Doctor.Run.Message)
	}

	started := doctorStartedCommands(t, m.doctorScheduleCmd())
	if len(started) != 2 {
		t.Fatalf("the pool put %d of the run's 2 checks in flight", len(started))
	}

	next, follow := m.Update(started[0]())
	m = next.(Model)
	if follow != nil {
		t.Error("the run started work although both of its checks were already in flight")
	}
	if !strings.Contains(m.Doctor.Run.Message, "Checked 1 of 2") {
		t.Errorf("the progress line did not count the answer that came back: %q", m.Doctor.Run.Message)
	}
	if len(m.Doctor.Run.Results) != 1 {
		t.Errorf("the run kept %d answers after one answer, want 1", len(m.Doctor.Run.Results))
	}

	next, _ = m.Update(started[1]())
	m = next.(Model)
	if m.Doctor.Run.running() {
		t.Fatal("the run is still in flight after every check answered")
	}
	if !strings.Contains(m.Doctor.Run.Message, "2 checks") {
		t.Errorf("the closing line does not count the checks: %q", m.Doctor.Run.Message)
	}
	if !strings.Contains(m.Doctor.Run.Message, "ok") {
		t.Errorf("the closing line does not summarize what was found: %q", m.Doctor.Run.Message)
	}
	if m.Doctor.Run.ReportPath == "" {
		t.Fatal("the run wrote no report")
	}
	if _, err := os.Stat(m.Doctor.Run.ReportPath); err != nil {
		t.Errorf("the report the screen named is not there: %v", err)
	}
}

// TestDoctorScreenIgnoresAnAnswerFromARunThatWasReplaced is the stale-message
// guard: a result from a run that is no longer current must not be written into
// the run that replaced it.
func TestDoctorScreenIgnoresAnAnswerFromARunThatWasReplaced(t *testing.T) {
	host := doctorHealthyHost()
	m := doctorScreenRunModel(t, host, "env.home")
	before := m.Doctor.Run.Next

	next, _ := m.Update(doctorCheckDoneMsg{RunID: 99, Index: 0, Result: doctorCheckResult{ID: "env.home", Status: doctorOK}})
	m = next.(Model)
	if m.Doctor.Run.Next != before {
		t.Errorf("an answer from another run moved this run to %d, want %d", m.Doctor.Run.Next, before)
	}
	if len(m.Doctor.Run.Results) != 0 {
		t.Errorf("an answer from another run was kept: %v", m.Doctor.Run.Results)
	}
}

// TestDoctorScreenCancellingFillsInTheChecksThatNeverRan is the cancellation
// rule: Escape stops the run there and then -- the checks in flight cannot be
// interrupted, so waiting for them is what made the screen unreadable -- and the
// checks that did not answer are recorded as unknown with the reason, so the
// report says how far the run got rather than reading as a clean machine.
func TestDoctorScreenCancellingFillsInTheChecksThatNeverRan(t *testing.T) {
	host := doctorHealthyHost()
	m := doctorScreenRunModel(t, host, "env.home", "env.locale", "env.term")
	inFlight := doctorStartedCommands(t, m.doctorScheduleCmd())
	if len(inFlight) != len(m.Doctor.Run.Plan) {
		t.Fatalf("the run put %d of %d checks in flight", len(inFlight), len(m.Doctor.Run.Plan))
	}

	next, cmd := m.handleDoctorKeys("esc")
	m = next.(Model)
	if cmd != nil {
		t.Error("cancelling started some work")
	}
	if !m.Doctor.Run.Canceled {
		t.Fatal("escape did not mark the run as cancelled")
	}
	if m.Doctor.Run.running() {
		t.Fatal("escape left the run in flight, so the checks in flight were not cancelled")
	}
	if !strings.Contains(m.Doctor.Run.Message, "Cancelled after 0 of 3") {
		t.Errorf("the closing line does not say how far the run got: %q", m.Doctor.Run.Message)
	}
	for _, id := range []string{"env.home", "env.locale", "env.term"} {
		result, ok := m.Doctor.Run.Results[id]
		if !ok {
			t.Fatalf("the check %q that never ran has no result, so the report would leave it out", id)
		}
		if result.Status != doctorUnknown {
			t.Errorf("the check %q that never ran reports %s, want unknown", id, result.Status)
		}
		if len(result.Findings) == 0 || !strings.Contains(result.Findings[0].Evidence, "cancel") {
			t.Errorf("the check %q does not say why it did not run: %s", id, doctorFindingSummary(result.Findings))
		}
	}

	// The report holds the same answer as the screen.
	data, err := os.ReadFile(m.Doctor.Run.ReportPath)
	if err != nil {
		t.Fatalf("the cancelled run wrote no report: %v", err)
	}
	if !strings.Contains(string(data), "unknown") {
		t.Errorf("the report does not carry the checks that never ran:\n%s", data)
	}
}

// TestDoctorScreenOffersARetryOnlyForTheChecksThatCouldNotAnswer pins the two
// rules that make the retry honest: it appears only when something is unknown,
// and it runs only those checks.
func TestDoctorScreenOffersARetryOnlyForTheChecksThatCouldNotAnswer(t *testing.T) {
	host := doctorHealthyHost()
	m := doctorScreenRunModel(t, host, "env.home")
	m.Doctor.Run.Results["env.home"] = doctorCheckResult{ID: "env.home", Status: doctorOK, Findings: []doctorFinding{{
		Subject: "The home directory", Status: doctorOK, Evidence: "e", Impact: "i", Advice: "a",
	}}}
	m.Doctor.Run.Plan = nil
	m.Doctor.Run.Finished = time.Now()

	for _, row := range m.doctorList() {
		if row.label == doctorRunRetryRow {
			t.Fatal("a run with nothing unknown still offers a retry")
		}
	}

	m.Doctor.Run.Results["env.home"] = doctorCheckResult{ID: "env.home", Status: doctorUnknown}
	for _, row := range m.doctorList() {
		if row.label != doctorRunRetryRow {
			continue
		}
		checks := m.doctorUnknownChecks()
		if len(checks) != 1 || checks[0].ID != "env.home" {
			t.Fatalf("the retry would run %d checks, want only the one that could not answer", len(checks))
		}
		return
	}
	t.Fatal("a run with an unknown check offers no retry")
}

// TestDoctorScreenFilterHidesAndShowsTheCleanAnswers is the filter rule the brief
// asks for: the list starts with the problems, and the clean answers are one
// press away rather than mixed in.
func TestDoctorScreenFilterHidesAndShowsTheCleanAnswers(t *testing.T) {
	host := doctorHealthyHost()
	m := doctorScreenRunModel(t, host, "dotfiles.record", "env.home")
	m.Doctor.Run.Results["dotfiles.record"] = doctorCheckResult{
		ID: "dotfiles.record", Status: doctorNotApplicable,
		Findings: []doctorFinding{{Subject: "The record of the last installation", Status: doctorNotApplicable, Evidence: "e", Impact: "i", Advice: "a"}},
	}
	m.Doctor.Run.Results["env.home"] = doctorCheckResult{
		ID: "env.home", Status: doctorWarning,
		Findings: []doctorFinding{{Subject: "The home directory", Status: doctorWarning, Evidence: "e", Impact: "i", Advice: "a"}},
	}
	m.Doctor.Run.Plan = nil
	m.Doctor.Run.Finished = time.Now()

	problems := strings.Join(m.doctorRows(), "\n")
	if !strings.Contains(problems, "The home directory") {
		t.Errorf("the default list does not show the finding that needs attention:\n%s", problems)
	}
	if strings.Contains(problems, "The record of the last installation") {
		t.Errorf("the default list shows a clean answer:\n%s", problems)
	}
	if !strings.Contains(problems, doctorFilterAllRow) {
		t.Errorf("the list does not offer to show everything:\n%s", problems)
	}

	next, _ := m.handleDoctorKeys("t")
	m = next.(Model)
	every := strings.Join(m.doctorRows(), "\n")
	if !strings.Contains(every, "The record of the last installation") {
		t.Errorf("showing everything did not show the clean answer:\n%s", every)
	}
	if !strings.Contains(every, doctorFilterProblemsRow) {
		t.Errorf("with everything shown, the list does not offer to go back to the problems:\n%s", every)
	}
}

// TestDoctorScreenWritesTheExportItSaysItWrote is the export rule: the screen
// names a file that is really there, and a dry run says nothing was written.
func TestDoctorScreenWritesTheExportItSaysItWrote(t *testing.T) {
	t.Setenv("DOTFILES_DRY_RUN", "0")
	host := doctorHealthyHost()
	m := doctorRunToTheEnd(t, doctorScreenRunModel(t, host, "env.home"))

	row := -1
	for index, candidate := range m.GetCurrentOptions() {
		if candidate == doctorWriteReportRow {
			row = index
		}
	}
	if row < 0 {
		t.Fatal("the finished list does not offer to write a report")
	}
	m.Cursor = row

	next, cmd := m.handleDoctorKeys("enter")
	m = next.(Model)
	if cmd == nil {
		t.Fatal("pressing the row wrote nothing")
	}
	written, ok := cmd().(doctorReportWrittenMsg)
	if !ok {
		t.Fatalf("the row produced %T, want a written report", cmd())
	}
	if written.Err != "" {
		t.Fatalf("the export failed: %s", written.Err)
	}

	next, _ = m.Update(written)
	m = next.(Model)
	if !strings.Contains(m.Doctor.Notice, doctorRelHomePath(written.Path)) {
		t.Errorf("the screen does not name the file it wrote: %q", m.Doctor.Notice)
	}
	if _, err := os.Stat(written.Path); err != nil {
		t.Errorf("the file the screen named is not there: %v", err)
	}
}

// TestDoctorScreenDryRunSaysNothingWasWritten is the other half: with the switch
// on, the screen still names the file it would write and says it did not.
func TestDoctorScreenDryRunSaysNothingWasWritten(t *testing.T) {
	t.Setenv("DOTFILES_DRY_RUN", "1")
	host := doctorHealthyHost()
	m := doctorRunToTheEnd(t, doctorScreenRunModel(t, host, "env.home"))

	next, cmd := m.handleDoctorKeys("w")
	m = next.(Model)
	if cmd == nil {
		t.Fatal("the dry run produced no answer at all")
	}
	next, _ = m.Update(cmd())
	m = next.(Model)
	if !strings.Contains(m.Doctor.Notice, "dry run") {
		t.Errorf("the screen does not say a dry run changed nothing: %q", m.Doctor.Notice)
	}
}

// TestDoctorFixScreenStatesTheEffectTheRiskThePermissionAndTheBackup is S9's
// preview rule: nothing is applied before the user sees all four, and leaving
// with Escape changes nothing.
func TestDoctorFixScreenStatesTheEffectTheRiskThePermissionAndTheBackup(t *testing.T) {
	host := doctorHealthyHost()
	host.addFile("/home/tester/.ssh", 0o755, true, "")
	m := doctorScreenRunModel(t, host, "git.sshauth")
	m = doctorRunToTheEnd(t, m)

	finding := doctorFindingWith(t, m.Doctor.Run.Results["git.sshauth"].Findings, "readable by others")
	if finding.Fix == nil {
		t.Fatal("the finding no longer offers a fix, so this guard proves nothing")
	}

	if cmd := m.openDoctorFix(finding); cmd != nil {
		t.Errorf("opening the confirmation screen started some work: %v", cmd)
	}
	if m.Screen != ScreenDoctorFix {
		t.Fatalf("opening the fix landed on %v, want the confirmation screen", m.Screen)
	}
	preview := m.Doctor.PendingPreview
	if len(preview.Effect) == 0 || preview.Risk == "" || preview.Permission == "" || preview.Backup == "" {
		t.Fatalf("the confirmation screen does not state the effect, the risk, the permission and the backup: %+v", preview)
	}
	if preview.Refusal != "" {
		t.Fatalf("the confirmation screen refuses a fix it should offer: %s", preview.Refusal)
	}

	view := m.View()
	for _, want := range []string{"before:", "after:", "0755", "0700"} {
		if !strings.Contains(view, want) {
			t.Errorf("the confirmation screen does not state %q:\n%s", want, view)
		}
	}

	next, _ := m.handleDoctorFixKeys("esc")
	m = next.(Model)
	if m.Screen != ScreenDoctor {
		t.Errorf("escape left the screen on %v, want the doctor's list", m.Screen)
	}
	if m.Doctor.Pending != nil {
		t.Error("escape left a fix pending")
	}
	if len(host.mutations) != 0 {
		t.Errorf("escape changed the machine: %v", host.mutations)
	}
}

// TestDoctorFixScreenAppliesAndReChecksTheFinding is the whole cycle on the
// screen: the fix is applied, the finding it came from is re-checked on its own,
// and the answer that comes back replaces the old one.
func TestDoctorFixScreenAppliesAndReChecksTheFinding(t *testing.T) {
	t.Setenv("DOTFILES_DRY_RUN", "0")
	host := doctorHealthyHost()
	host.addFile("/home/tester/.ssh", 0o755, true, "")
	m := doctorRunToTheEnd(t, doctorScreenRunModel(t, host, "git.sshauth"))

	finding := doctorFindingWith(t, m.Doctor.Run.Results["git.sshauth"].Findings, "readable by others")
	m.openDoctorFix(finding)

	next, cmd := m.handleDoctorFixKeys("enter")
	m = next.(Model)
	if cmd == nil {
		t.Fatal("pressing the action applied nothing")
	}
	if !strings.Contains(m.Doctor.FixNotice, "Applying") {
		t.Errorf("the screen does not say it is applying the fix: %q", m.Doctor.FixNotice)
	}

	outcome, ok := cmd().(doctorFixDoneMsg)
	if !ok {
		t.Fatalf("the fix produced %T, want an outcome", cmd())
	}
	if !outcome.Outcome.Applied {
		t.Fatalf("the fix was not applied: %s", outcome.Outcome.Notice)
	}
	if len(host.mutations) == 0 {
		t.Fatal("the fix reported success without changing the machine")
	}

	next, cmd = m.Update(outcome)
	m = next.(Model)
	if m.Screen != ScreenDoctor {
		t.Fatalf("after applying, the screen is on %v", m.Screen)
	}
	if m.Doctor.Pending != nil {
		t.Error("the pending fix was not cleared")
	}
	if !strings.Contains(m.Doctor.FixNotice, "Done") {
		t.Errorf("the outcome is not stated on the list: %q", m.Doctor.FixNotice)
	}

	// The re-check runs the one check the finding came from, and the machine is
	// now in the state the fix produced. The command the update loop was handed
	// is the re-check itself, so it is passed on rather than thrown away.
	if m.Doctor.Run == nil || len(m.Doctor.Run.Plan) != 1 || m.Doctor.Run.Plan[0].ID != "git.sshauth" {
		t.Fatalf("the re-check runs %+v, want the one check the fix came from", m.Doctor.Run.Plan)
	}
	m = doctorRunToTheEnd(t, m, cmd)
	after := m.Doctor.Run.Results["git.sshauth"]
	for _, leftover := range after.Findings {
		if leftover.Status == doctorWarning && strings.Contains(leftover.Subject, "readable by others") {
			t.Errorf("the re-check still reports the fault that was fixed:\n%s", doctorFindingSummary(after.Findings))
		}
	}
}

// TestDoctorFixScreenRefusesASecondPress pins the double-press rule: the outcome
// is reported once, and pressing again does not apply the same change twice.
func TestDoctorFixScreenRefusesASecondPress(t *testing.T) {
	t.Setenv("DOTFILES_DRY_RUN", "0")
	host := doctorHealthyHost()
	host.addFile("/home/tester/.ssh", 0o755, true, "")
	m := doctorRunToTheEnd(t, doctorScreenRunModel(t, host, "git.sshauth"))
	finding := doctorFindingWith(t, m.Doctor.Run.Results["git.sshauth"].Findings, "readable by others")

	m.openDoctorFix(finding)
	next, cmd := m.handleDoctorFixKeys("enter")
	m = next.(Model)
	next, _ = m.Update(cmd())
	m = next.(Model)
	changes := len(host.mutations)

	next, again := m.handleDoctorFixKeys("enter")
	m = next.(Model)
	if again != nil {
		if message, ok := again().(doctorFixDoneMsg); ok && message.Outcome.Applied {
			t.Error("a second press applied the fix again")
		}
	}
	if len(host.mutations) != changes {
		t.Errorf("a second press changed the machine again: %v", host.mutations)
	}
}

// TestDoctorRenderingTouchesNothing is the rule the whole design rests on: a
// frame is drawn from the model, so it reads no file, runs no command and writes
// nothing. The run's own scan records every look, and the frame must produce no
// look at all.
func TestDoctorRenderingTouchesNothing(t *testing.T) {
	host := doctorHealthyHost()
	m := doctorRunToTheEnd(t, doctorScreenRunModel(t, host, "env.home"))
	reads, commands, mutations := len(host.reads), len(host.commandsRun), len(host.mutations)

	for _, screen := range []Screen{ScreenDoctor, ScreenDoctorFix} {
		m.Screen = screen
		if screen == ScreenDoctorFix {
			m.Doctor.Pending = &doctorFinding{Subject: "The home directory", Status: doctorWarning, Fix: &doctorFix{Kind: doctorFixMakeDir, Path: "/home/tester/.local/state"}}
		}
		for i := 0; i < 3; i++ {
			if view := m.View(); view == "" {
				t.Fatal("the frame is empty")
			}
		}
	}
	if got := len(host.reads); got != reads {
		t.Errorf("drawing the frames read the machine %d more time(s)", got-reads)
	}
	if got := len(host.commandsRun); got != commands {
		t.Errorf("drawing the frames ran %d command(s)", got-commands)
	}
	if got := len(host.mutations); got != mutations {
		t.Errorf("drawing the frames changed the machine %d time(s)", got-mutations)
	}
}

// TestDoctorScreenStatesAreWordsNotOnlyColours is the colour rule the interface
// follows everywhere: every state is a word and a plain glyph, so the five
// answers are told apart on a terminal with no colour at all.
func TestDoctorScreenStatesAreWordsNotOnlyColours(t *testing.T) {
	host := doctorHealthyHost()
	m := doctorScreenRunModel(t, host, "env.home")
	m.Doctor.AllFindings = true
	m.Doctor.Run.Results["env.home"] = doctorCheckResult{ID: "env.home", Status: doctorError, Findings: []doctorFinding{
		{Subject: "an error", Status: doctorError, Evidence: "e", Impact: "i", Advice: "a"},
		{Subject: "a warning", Status: doctorWarning, Evidence: "e", Impact: "i", Advice: "a"},
		{Subject: "an unknown", Status: doctorUnknown, Evidence: "e", Impact: "i", Advice: "a"},
		{Subject: "not applicable", Status: doctorNotApplicable, Evidence: "e", Impact: "i", Advice: "a"},
		{Subject: "clean", Status: doctorOK, Evidence: "e", Impact: "i", Advice: "a"},
	}}
	m.Doctor.Run.Plan = nil
	m.Doctor.Run.Finished = time.Now()

	rows := strings.Join(m.doctorRows(), "\n")
	for _, want := range []string{"Error", "Warning", "Unknown", "Not applicable", "OK"} {
		if !strings.Contains(rows, want) {
			t.Errorf("the list does not state %q as a word:\n%s", want, rows)
		}
	}
	view := m.View()
	for _, state := range []doctorStatus{doctorError, doctorWarning, doctorUnknown, doctorNotApplicable, doctorOK} {
		if !strings.Contains(view, state.label()) {
			t.Errorf("the rendered frame does not state the state %q as a word", state.label())
		}
	}
}

// TestUtilitiesPanelNamesTheDoctorFromTheModelState is the panel's derivation
// rule for this entry: the doctor's fact comes from the field the section reads,
// so a run changes the panel and an empty model says there is nothing yet.
func TestUtilitiesPanelNamesTheDoctorFromTheModelState(t *testing.T) {
	before := contextualMainMenuModel()
	before.SystemInfo = doctorTestLinuxInfo()
	beforeFlat := utilitiesPanelFlat(t, before)
	if !strings.Contains(beforeFlat, "Doctor") {
		t.Errorf("the panel does not name the doctor at all:\n%s", beforeFlat)
	}

	after := contextualMainMenuModel()
	after.SystemInfo = doctorTestLinuxInfo()
	after.Doctor = doctorState{Run: &doctorRun{
		Order:    []string{"env.home"},
		Finished: time.Now(),
		Message:  "1 check: 1 warning",
		Results: map[string]doctorCheckResult{"env.home": {
			ID: "env.home", Status: doctorWarning,
			Findings: []doctorFinding{{Subject: "The home directory", Status: doctorWarning, Evidence: "e", Impact: "i", Advice: "a"}},
		}},
	}}
	afterFlat := utilitiesPanelFlat(t, after)
	if !strings.Contains(afterFlat, "1 warning") {
		t.Errorf("the panel did not pick up the run's own line:\n%s", afterFlat)
	}
	if beforeFlat == afterFlat {
		t.Error("the panel reads the same with a run and without one, so its fact is not derived from the model")
	}
}

// TestTheUtilitiesSectionOpensTheDoctor pins the wiring end to end: the row the
// section offers opens the doctor and starts nothing.
func TestTheUtilitiesSectionOpensTheDoctor(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenUtilities
	m.Cursor = 0
	for index, row := range m.GetCurrentOptions() {
		if row == utilitiesScreenRow(utilitiesDoctorRow) {
			m.Cursor = index
		}
	}
	if row := m.GetCurrentOptions()[m.Cursor]; row != utilitiesScreenRow(utilitiesDoctorRow) {
		t.Fatalf("the utilities list has no row %q", utilitiesDoctorRow)
	}

	next, cmd := m.handleUtilitiesKeys("enter")
	m = next.(Model)
	if m.Screen != ScreenDoctor {
		t.Fatalf("selecting the row landed on %v, want the doctor", m.Screen)
	}
	if cmd != nil {
		t.Error("opening the doctor started a scan; a scan is a choice made on the screen")
	}

	// And the way back.
	next, _ = m.handleDoctorKeys("esc")
	m = next.(Model)
	if m.Screen != ScreenUtilities {
		t.Errorf("escape left the doctor on %v, want the utilities section", m.Screen)
	}
}

// TestDoctorStateSurvivesTheRoundTripThroughTheSection is what a returning user
// sees: the answers are still there, with the cursor on a row that exists.
func TestDoctorStateSurvivesTheRoundTripThroughTheSection(t *testing.T) {
	host := doctorHealthyHost()
	m := doctorRunToTheEnd(t, doctorScreenRunModel(t, host, "env.home"))

	m.Screen = ScreenUtilities
	m.Cursor = 0
	next, _ := m.handleDoctorKeys("esc")
	m = next.(Model)
	if m.Doctor.Run == nil || m.Doctor.Run.Finished.IsZero() {
		t.Fatal("leaving the screen threw the run away")
	}

	m.Screen = ScreenDoctor
	m.settleDoctorCursor()
	row := m.doctorCursorRow()
	if row == nil {
		t.Fatal("the cursor sits on no row after the round trip")
	}
	if row.separator {
		t.Errorf("the cursor sits on a separator after the round trip")
	}
}

// TestDoctorReportIsWrittenBesideTheInstallersOwnState pins where the report
// lands: the installer's own state directory, not the user's home.
func TestDoctorReportIsWrittenBesideTheInstallersOwnState(t *testing.T) {
	m := doctorScreenRunModel(t, doctorHealthyHost(), "env.home")
	// The state directory is named after the model is built, because building it
	// isolates the run into a temporary directory of its own.
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	m = doctorRunToTheEnd(t, m)

	want := filepath.Join(state, stateAppDir, "doctor-report.txt")
	if m.Doctor.Run.ReportPath != want {
		t.Errorf("the report was written to %q, want %q", m.Doctor.Run.ReportPath, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Errorf("the report is not in the installer's state directory: %v", err)
	}
}
