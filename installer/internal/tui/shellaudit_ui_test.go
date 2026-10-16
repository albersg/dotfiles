package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// ---------------------------------------------------------------------------
// The screen: the plan, the starts as they land, and the key that stops them
// ---------------------------------------------------------------------------
//
// The measurement in this screen starts the user's shell several times, and the
// guards below are about the two things that makes necessary: the keys have to
// change the plan the press really measures, and the screen has to be able to
// stop starts it has already begun. A guard proves the first by reading the
// arguments the probe was called with, and the second by cancelling while a start
// is in flight and checking that what finished is kept and nothing else is
// invented.

// shellAuditUIModel is the screen with the fake probe behind its own start and
// cancel path, so a guard can drive the keys without starting the runner's shell.
func shellAuditUIModel(t *testing.T, f *fakeShellAuditProbe) Model {
	t.Helper()

	m := installerFrameModel(t, ScreenShellAudit)
	m.ShellAudit = resolveShellAudit(f.probe())
	m.shellAuditTestProbe = f.probe
	m.Cursor = 0
	return m
}

// pressKey sends one key to the model and returns it with the command the press
// asked for.
func pressKey(t *testing.T, m Model, key string) (Model, tea.Cmd) {
	t.Helper()

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	next, ok := updated.(Model)
	if !ok {
		t.Fatalf("the update returned %T, want the model", updated)
	}
	return next, cmd
}

// shellAuditSeen is what the screen was told during one measurement: every start
// it watched, with how each one ended.
type shellAuditSeen struct {
	Progress []shellAuditProgress
	Outcomes []shellAuditOutcome
}

// finished is the starts that produced a duration, in the order they ended.
func (s shellAuditSeen) finished() []shellAuditProgress {
	var finished []shellAuditProgress
	for index, progress := range s.Progress {
		if s.Outcomes[index] == shellAuditOutcomeFinished {
			finished = append(finished, progress)
		}
	}
	return finished
}

// drainShellAudit reads the commands a press returned until nothing is left to
// read, and returns the model with what the screen was told on the way. It is how
// a guard watches a measurement from the press to the result: every event is one
// command, which is the whole reason the screen can draw progress at all.
func drainShellAudit(t *testing.T, m Model, cmd tea.Cmd) (Model, shellAuditSeen) {
	t.Helper()

	var seen shellAuditSeen
	for cmd != nil {
		msg := cmd()
		if msg == nil {
			break
		}
		if event, ok := msg.(shellAuditEventMsg); ok && !event.event.Done {
			seen.Progress = append(seen.Progress, event.event.Progress)
			seen.Outcomes = append(seen.Outcomes, event.event.Outcome)
		}
		updated, next := m.Update(msg)
		nextModel, ok := updated.(Model)
		if !ok {
			t.Fatalf("the update returned %T, want the model", updated)
		}
		m = nextModel
		cmd = next
	}
	return m, seen
}

// TestShellAuditPlanKeysChangeOneSettingEach pins the plan keys: each one moves
// the setting it names and leaves the others where the user put them, and no key
// changes a start the shell does not offer.
func TestShellAuditPlanKeysChangeOneSettingEach(t *testing.T) {
	f := zshAuditFixture()
	m := shellAuditUIModel(t, f)

	before := m.ShellAudit.Plan
	m, _ = pressKey(t, m, "m")
	if m.ShellAudit.Plan.Mode == before.Mode {
		t.Error("the start key did not change the start")
	}
	if m.ShellAudit.Plan.Samples != before.Samples || m.ShellAudit.Plan.Warmup != before.Warmup ||
		m.ShellAudit.Plan.Baseline != before.Baseline {
		t.Errorf("the start key changed another setting: %+v", m.ShellAudit.Plan)
	}

	started := m.ShellAudit.Plan.Mode
	m, _ = pressKey(t, m, "n")
	if m.ShellAudit.Plan.Samples == before.Samples {
		t.Error("the runs key did not change the number of runs")
	}
	if m.ShellAudit.Plan.Mode != started {
		t.Errorf("the runs key changed the start to %q", m.ShellAudit.Plan.Mode.label())
	}

	runs := m.ShellAudit.Plan.Samples
	m, _ = pressKey(t, m, "w")
	if m.ShellAudit.Plan.Warmup == before.Warmup {
		t.Error("the warm-up key did not change the warm-up")
	}
	if m.ShellAudit.Plan.Samples != runs || m.ShellAudit.Plan.Mode != started {
		t.Error("the warm-up key changed another setting")
	}

	m, _ = pressKey(t, m, "b")
	if !m.ShellAudit.Plan.Baseline {
		t.Error("the comparison key did not ask for the clean comparison")
	}
	m, _ = pressKey(t, m, "b")
	if m.ShellAudit.Plan.Baseline {
		t.Error("the comparison key did not turn the clean comparison off again")
	}

	// The cycle stays inside the starts this shell has: an unknown family offers
	// the interactive start and nothing else, and a row that offered another one
	// would be a plan the run refuses.
	unknown := zshAuditFixture()
	unknown.env["SHELL"] = "/usr/bin/elvish"
	mu := shellAuditUIModel(t, unknown)
	for i := 0; i < len(shellAuditModes())+1; i++ {
		mu, _ = pressKey(t, mu, "m")
		if mu.ShellAudit.Plan.Mode != shellAuditModeInteractive {
			t.Fatalf("an unknown shell's plan cycled to the %s start", mu.ShellAudit.Plan.Mode.label())
		}
	}
	// The warm-up cycle is bounded by the plan's own bound.
	for i := 0; i < shellAuditMaxWarmup+2; i++ {
		mu, _ = pressKey(t, mu, "w")
		if mu.ShellAudit.Plan.Warmup > shellAuditMaxWarmup {
			t.Fatalf("the warm-up cycled to %d, past the bound the plan accepts", mu.ShellAudit.Plan.Warmup)
		}
	}
}

// TestShellAuditPlanReachesTheMeasurementItStarts is the rule that makes the plan
// worth drawing: what the screen says it will measure has to be what the press
// measures. The guard reads the arguments the probe was really called with.
func TestShellAuditPlanReachesTheMeasurementItStarts(t *testing.T) {
	f := zshAuditFixture()
	f.script = []time.Duration{100 * time.Millisecond}
	f.runs = 1

	m := shellAuditUIModel(t, f)
	m.ShellAudit.Plan = shellAuditPlan{Mode: shellAuditModeLogin, Samples: 1}

	cmd := func() tea.Cmd {
		next, c := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		var ok bool
		m, ok = next.(Model)
		if !ok {
			t.Fatalf("the update returned %T, want the model", next)
		}
		return c
	}()
	if cmd == nil {
		t.Fatal("the measure row started nothing")
	}
	m, _ = drainShellAudit(t, m, cmd)

	if len(f.calls) == 0 {
		t.Fatal("the shell was never started")
	}
	if !m.ShellAudit.Measured {
		t.Fatal("the measurement never landed on the state")
	}
	if m.ShellAudit.Mode != shellAuditModeLogin {
		t.Errorf("the measured start = %q, want the login start the plan asked for", m.ShellAudit.Mode.label())
	}
	if got := strings.Join(f.calls[0], " "); !strings.Contains(got, "-l") {
		t.Errorf("the first start was %q, want the login flag the plan asked for", got)
	}
	if m.ShellAudit.Runs != 1 {
		t.Errorf("the measurement made %d runs, want the one the plan asked for", m.ShellAudit.Runs)
	}
	if m.ShellAuditSession != nil {
		t.Error("the session was left on the model after the measurement ended")
	}
}

// TestShellAuditMeasureRowReportsEveryStartThenTheResult pins the screen's
// progress: every start is reported as it lands and the result arrives once, at
// the end. A screen that only learned about the result could not say which start
// it was waiting on, which is the whole difference between a wait and a number.
func TestShellAuditMeasureRowReportsEveryStartThenTheResult(t *testing.T) {
	f := zshAuditFixture()
	f.script = []time.Duration{100 * time.Millisecond, 200 * time.Millisecond, 300 * time.Millisecond}
	f.runs = 3

	m := shellAuditUIModel(t, f)
	m.ShellAudit.Plan = shellAuditPlan{Mode: shellAuditModeInteractive, Samples: 3, Warmup: 1}

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, ok := next.(Model)
	if !ok {
		t.Fatalf("the update returned %T, want the model", next)
	}
	if !m.ShellAudit.Measuring {
		t.Fatal("the screen does not report that a measurement is running")
	}
	if cmd == nil {
		t.Fatal("the measure row started nothing")
	}

	// Every start reports its beginning and its end, and the ends are what the
	// guard counts: those are the starts the user is waiting through. A start that
	// began and never ended would be a start the screen reported as running.
	m, seen := drainShellAudit(t, m, cmd)

	// The version is announced once, before any start, and the profiled start is
	// announced as its own part: both are progress the screen draws.
	if len(seen.Progress) == 0 || seen.Progress[0].Part != shellAuditPartVersion {
		t.Fatalf("the first progress was %+v, want the version being read", seen.Progress)
	}
	profiled := false
	for _, progress := range seen.Progress {
		if progress.Part == shellAuditPartProfile {
			profiled = true
		}
	}
	if !profiled {
		t.Error("the screen was never told about the profiled start")
	}

	finished := seen.finished()

	if len(finished) != 4 {
		t.Errorf("the screen was told about %d starts, want the one warm-up and the three runs", len(finished))
	}
	if finished[0].Part != shellAuditPartWarmup || finished[1].Part != shellAuditPartConfigured {
		t.Errorf("the finished starts were %q and %q, want the warm-up then the runs",
			finished[0].Part, finished[1].Part)
	}
	if last := finished[len(finished)-1]; last.Run != 3 || last.Total != 3 {
		t.Errorf("the last run was reported as %d of %d", last.Run, last.Total)
	}

	if m.ShellAudit.Measuring {
		t.Error("the screen still reports a measurement after the result landed")
	}
	if m.ShellAudit.Progress != (shellAuditProgress{}) {
		t.Errorf("the progress was left at %+v after the measurement", m.ShellAudit.Progress)
	}
	if m.ShellAudit.Median != 200*time.Millisecond {
		t.Errorf("the median = %v, want the middle of the three runs", m.ShellAudit.Median)
	}

	// A second press while the first measurement is in flight starts nothing: it
	// would start more of the user's shell beside the ones already running.
	mid, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	running, ok := mid.(Model)
	if !ok {
		t.Fatalf("the update returned %T, want the model", mid)
	}
	_ = cmd
	if !running.ShellAudit.Measuring {
		t.Fatal("the measurement is not running, so this guard proves nothing")
	}
	if _, again := running.Update(tea.KeyMsg{Type: tea.KeyEnter}); again != nil {
		t.Error("a second press started a second measurement")
	}
}

// TestShellAuditEscStopsTheStartsThatAreLeft is the cancel: the starts that
// finished are kept, the one in flight is cut, and the screen stays where the
// user is rather than leaving a measurement running behind it.
func TestShellAuditEscStopsTheStartsThatAreLeft(t *testing.T) {
	f := zshAuditFixture()
	f.timeout = 30 * time.Second
	f.script = []time.Duration{100 * time.Millisecond, 120 * time.Millisecond}
	f.runs = 5
	f.hangAt = map[int]bool{2: true, 3: true, 4: true}

	m := shellAuditUIModel(t, f)
	m.ShellAudit.Plan = shellAuditPlan{Mode: shellAuditModeInteractive, Samples: 5}

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, ok := next.(Model)
	if !ok {
		t.Fatalf("the update returned %T, want the model", next)
	}
	if m.ShellAuditSession == nil {
		t.Fatal("the measurement is running without a session the cancel could reach")
	}

	// Read the first two finished runs, then stop it.
	finished := 0
	for cmd != nil && finished < 2 {
		msg := cmd()
		if event, isEvent := msg.(shellAuditEventMsg); isEvent && event.event.Outcome == shellAuditOutcomeFinished {
			finished++
		}
		updated, next := m.Update(msg)
		var okUpdated bool
		m, okUpdated = updated.(Model)
		if !okUpdated {
			t.Fatalf("the update returned %T, want the model", updated)
		}
		cmd = next
	}
	if finished != 2 {
		t.Fatalf("only %d runs finished before the cancel, so this guard proves nothing", finished)
	}

	// The press is taken while a read of the event stream is already in flight. That
	// read is what brings the events the stop produces -- the cut start, then the
	// result -- and it is why the press itself starts nothing: the samples that
	// finished before it are folded in by the read the press did not stop.
	var inFlight tea.Cmd = cmd
	updated, cancelCmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("esc")})
	m, ok = updated.(Model)
	if !ok {
		t.Fatalf("the update returned %T, want the model", updated)
	}
	if cancelCmd != nil {
		t.Error("the stop asked for another read, and two reads of one stream split it")
	}
	if m.Screen != ScreenShellAudit {
		t.Error("the cancel left the screen, so the measurement is running behind it")
	}
	m, _ = drainShellAudit(t, m, inFlight)

	if !m.ShellAudit.Cancelled {
		t.Error("the state does not report that the user stopped the measurement")
	}
	if len(m.ShellAudit.Samples) != 2 {
		t.Errorf("%d samples, want the two runs that finished before the stop", len(m.ShellAudit.Samples))
	}
	if m.ShellAudit.Timeouts != 0 || m.ShellAudit.Failures != 0 {
		t.Errorf("the stop was counted as %d timeouts and %d failures, and it is neither",
			m.ShellAudit.Timeouts, m.ShellAudit.Failures)
	}
	if m.ShellAudit.Measuring || m.ShellAuditSession != nil {
		t.Error("the screen still reports the stopped measurement as running")
	}

	// With nothing running, the same key is the way back the footer names.
	idle := shellAuditUIModel(t, zshAuditFixture())
	idle, _ = pressKey(t, idle, "esc")
	if idle.Screen != ScreenUtilities {
		t.Errorf("Esc with nothing running left the screen at %v, want the utilities section", idle.Screen)
	}
}

// TestShellAuditHistoryKeyKeepsTheMeasurementsAndStoresTheChoice pins the one
// setting on this screen that writes to the user's disk: it is off until the user
// turns it on, turning it on stores the choice, and a measurement is stored with
// it. The file itself is the probe's, so no guard touches a real one.
func TestShellAuditHistoryKeyStoresTheMeasurementsAndTheChoice(t *testing.T) {
	f := zshAuditFixture()
	f.runs = 2
	f.script = []time.Duration{100 * time.Millisecond, 200 * time.Millisecond}

	m := shellAuditUIModel(t, f)
	if m.ShellAudit.History.Enabled {
		t.Fatal("the history is on for a user who never turned it on")
	}

	// Off: no measurement is stored and the file is not written.
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, ok := next.(Model)
	if !ok {
		t.Fatalf("the update returned %T, want the model", next)
	}
	m, _ = drainShellAudit(t, m, cmd)
	if len(m.ShellAudit.History.Entries) != 0 {
		t.Errorf("a measurement was stored with the history off: %+v", m.ShellAudit.History.Entries)
	}
	if f.historyWrites() != 0 {
		t.Errorf("the history was written with the switch off: %v", f.historyWrites())
	}

	// On: the choice is stored, and so is the next measurement.
	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	m, ok = next.(Model)
	if !ok {
		t.Fatalf("the update returned %T, want the model", next)
	}
	if !m.ShellAudit.History.Enabled {
		t.Fatal("the history key did not turn the history on")
	}
	m, _ = drainShellAudit(t, m, cmd)
	if f.historyWrites() == 0 {
		t.Fatal("turning the history on wrote nothing, so the choice would not survive a restart")
	}
	for _, path := range f.writes {
		if !strings.Contains(path, shellAuditHistoryFileName) {
			continue
		}
		// The write goes to a temporary neighbour and is rolled onto the name, so
		// nothing is ever written onto the history itself: a reader either sees the
		// history it had or one whole new one.
		if strings.HasSuffix(path, shellAuditHistoryFileName) {
			t.Errorf("the history was written straight onto its name: %v", f.writes)
		}
	}

	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, ok = next.(Model)
	if !ok {
		t.Fatalf("the update returned %T, want the model", next)
	}
	m, _ = drainShellAudit(t, m, cmd)
	if len(m.ShellAudit.History.Entries) != 1 {
		t.Fatalf("the measurement stored %d entries, want the one it made", len(m.ShellAudit.History.Entries))
	}
	if m.ShellAudit.History.Entries[0].Median != m.ShellAudit.Median {
		t.Errorf("the stored median = %v, want the measured %v",
			m.ShellAudit.History.Entries[0].Median, m.ShellAudit.Median)
	}

	// Turning it off keeps what is stored and writes the choice down.
	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	m, ok = next.(Model)
	if !ok {
		t.Fatalf("the update returned %T, want the model", next)
	}
	if m.ShellAudit.History.Enabled {
		t.Error("the history key did not turn the history off")
	}
	if len(m.ShellAudit.History.Entries) != 1 {
		t.Error("turning the history off threw away the stored measurements")
	}
	if _, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")}); cmd == nil {
		t.Error("turning the history off did not store the choice")
	}
}

// TestShellAuditAdviceKeyOpensTheRecommendations pins the key the mission calls
// opening the recommendations: they lead the text when it is pressed, so a frame
// too short to draw the whole text cannot cut them away first, and the key closes
// them again.
func TestShellAuditAdviceKeyOpensTheRecommendations(t *testing.T) {
	f := zshAuditFixture()
	m := shellAuditUIModel(t, f)
	m.ShellAudit = shellAuditTestState()

	closed := m.shellAuditDescription()
	if len(closed) == 0 {
		t.Fatal("the screen has no text at all")
	}
	if strings.HasPrefix(closed[0], "Recommendations") {
		t.Fatal("the recommendations lead the text before the key was pressed")
	}

	m, _ = pressKey(t, m, "r")
	if !m.ShellAudit.AdviceOpen {
		t.Fatal("the key did not open the recommendations")
	}
	opened := m.shellAuditDescription()
	if !strings.HasPrefix(opened[0], "Recommendations") {
		t.Errorf("the opened text starts with %q, want the recommendations", opened[0])
	}
	joined := strings.Join(opened, "\n")
	for _, want := range []string{"Measured:", "It comes from:"} {
		if !strings.Contains(joined, want) {
			t.Errorf("the opened recommendations do not carry %q", want)
		}
	}

	// The advice list is bounded, so a screen with many findings cannot push the
	// numbers off the frame.
	if strings.Count(joined, "It comes from:") > shellAuditAdviceListLimit {
		t.Errorf("the text lists %d recommendations, want at most %d",
			strings.Count(joined, "It comes from:"), shellAuditAdviceListLimit)
	}

	m, _ = pressKey(t, m, "r")
	if m.ShellAudit.AdviceOpen {
		t.Error("the key did not close the recommendations again")
	}
}

// TestShellAuditScreenFitsWithAMeasurementInFlight pins the frame while the
// screen is at its busiest: a measurement running, a result already behind it, and
// the recommendations opened. The measured sizes are the fit guard's own, so this
// is the same promise measured with the new text in place.
func TestShellAuditScreenFitsWithAMeasurementInFlight(t *testing.T) {
	for _, size := range measuredTerminalSizes {
		f := zshAuditFixture()
		f.runs = 3
		f.script = []time.Duration{100 * time.Millisecond}

		m := shellAuditUIModel(t, f)
		m.ShellAudit = shellAuditTestState()
		m.ShellAudit.Measuring = true
		m.ShellAudit.Progress = shellAuditProgress{Part: shellAuditPartConfigured, Run: 3, Total: 5}
		m.ShellAudit.AdviceOpen = true
		m.ShellAudit.Plan = shellAuditPlan{Mode: shellAuditModeInteractive, Samples: 5, Baseline: true}
		m.Width, m.Height = size.width, size.height

		view := m.View()
		rows := strings.Count(view, "\n")
		if rows > size.height {
			t.Errorf("the measuring screen at %dx%d drew %d rows", size.width, size.height, rows)
		}
		for _, want := range []string{utilitiesShellAuditRow, "← Back", "Measuring:"} {
			if !strings.Contains(view, want) {
				t.Errorf("the measuring screen at %dx%d dropped %q", size.width, size.height, want)
			}
		}
	}
}

// TestShellAuditClosedSessionStopsClaimingToMeasure pins the case of a
// measurement that died with its process: the screen stops saying it is measuring
// rather than waiting for an answer that is not coming, so the row can be pressed
// again.
func TestShellAuditClosedSessionStopsClaimingToMeasure(t *testing.T) {
	f := zshAuditFixture()
	m := shellAuditUIModel(t, f)
	m.ShellAudit.Measuring = true
	m.ShellAuditSession = &shellAuditSession{events: make(chan shellAuditEvent), cancel: func() {}}

	updated, cmd := m.Update(shellAuditClosedMsg{})
	next, ok := updated.(Model)
	if !ok {
		t.Fatalf("the update returned %T, want the model", updated)
	}
	if next.ShellAudit.Measuring {
		t.Error("the screen still claims to be measuring after the events ended")
	}
	if next.ShellAuditSession != nil {
		t.Error("the dead session was left on the model")
	}
	if cmd != nil {
		t.Error("a closed measurement asked for another read")
	}
}

// TestShellAuditNoticeNamesTheStartInFlight pins the line the frame draws above
// its rule: it names the start being measured rather than only that something is
// running, and it says which key stops it.
func TestShellAuditNoticeNamesTheStartInFlight(t *testing.T) {
	f := zshAuditFixture()
	m := shellAuditUIModel(t, f)
	m.ShellAudit.Plan = shellAuditPlan{Mode: shellAuditModeInteractive, Samples: 5}

	if notice := m.shellAuditNotice(); notice != "" {
		t.Errorf("the screen draws the notice %q with nothing running", notice)
	}

	m.ShellAudit.Measuring = true
	m.ShellAudit.Progress = shellAuditProgress{Part: shellAuditPartConfigured, Run: 2, Total: 5}
	notice := m.shellAuditNotice()
	for _, want := range []string{"run 2 of 5", "5 starts", "Esc"} {
		if !strings.Contains(notice, want) {
			t.Errorf("the notice %q does not carry %q", notice, want)
		}
	}

	// What the last thing to go wrong was is drawn where the progress goes: a
	// history that could not be written is worth saying, and the screen has one
	// line for it.
	m.ShellAudit.Measuring = false
	m.ShellAudit.HistoryError = "the stored measurements could not be read"
	if notice := m.shellAuditNotice(); !strings.Contains(notice, "could not be read") {
		t.Errorf("the notice %q does not report the history problem", notice)
	}
}
