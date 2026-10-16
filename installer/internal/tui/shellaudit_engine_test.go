package tui

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// The measurement: the plan, the starts, the cancel
// ---------------------------------------------------------------------------
//
// The engine is what replaced the single loop that ran one command five times.
// These guards pin the four things it added: a plan the user chose (which start,
// how many runs, how many warm-up runs, whether a clean baseline is measured
// too), a start reported as it lands, a cancel that keeps what finished and
// stops the rest, and a summary that is honest about a run that did not finish.

// engineFixture is one scripted machine whose zsh takes the given times.
func engineFixture(script ...time.Duration) *fakeShellAuditProbe {
	f := zshAuditFixture()
	f.script = script
	f.runs = len(script)
	return f
}

// engineEvents reads a session to its end and returns every event it reported.
func engineEvents(session *shellAuditSession) []shellAuditEvent {
	var events []shellAuditEvent
	for event := range session.events {
		events = append(events, event)
	}
	return events
}

// TestShellAuditWarmupIsMadeAndThrownAway pins the warm-up: the starts are made,
// they are reported, and none of them is a sample. A warm-up that leaked into the
// summary would make the median describe a shell somebody has already opened.
func TestShellAuditWarmupIsMadeAndThrownAway(t *testing.T) {
	f := zshAuditFixture()
	f.script = []time.Duration{900 * time.Millisecond, 300 * time.Millisecond, 200 * time.Millisecond, 100 * time.Millisecond}
	f.runs = 3

	st := resolveShellAudit(f.probe())
	st.Plan = shellAuditPlan{Mode: shellAuditModeInteractive, Samples: 3, Warmup: 1}
	st = measureShellAudit(st, f.probe())

	if len(st.Warmup) != 1 {
		t.Fatalf("%d warm-up starts, want the one the plan asked for", len(st.Warmup))
	}
	if st.Warmup[0] != 900*time.Millisecond {
		t.Errorf("the warm-up start = %v, want the 900 ms cold start", st.Warmup[0])
	}
	if len(st.Samples) != 3 {
		t.Fatalf("%d samples, want the three the plan asked for", len(st.Samples))
	}
	if st.Median != 200*time.Millisecond {
		t.Errorf("median = %v, want 200 ms: the cold start is a warm-up and must not be a sample", st.Median)
	}
	if st.Stats.Fastest != 100*time.Millisecond || st.Stats.Slowest != 300*time.Millisecond {
		t.Errorf("range = %v..%v, want 100 ms..300 ms", st.Stats.Fastest, st.Stats.Slowest)
	}
	// Three samples, one warm-up and one profiled start.
	if len(f.calls) != 5 {
		t.Errorf("the shell was started %d times, want three samples, one warm-up and the profiled start", len(f.calls))
	}
}

// TestShellAuditReportsEachStartAsItLands pins the progress the screen draws: one
// event when a start begins and one when it ends, each carrying the part of the
// plan it belongs to and its place among that part's starts. Without them the
// screen can only say "measuring".
func TestShellAuditReportsEachStartAsItLands(t *testing.T) {
	f := zshAuditFixture()
	f.script = []time.Duration{50 * time.Millisecond, 60 * time.Millisecond, 70 * time.Millisecond, 80 * time.Millisecond}
	f.runs = 3

	st := resolveShellAudit(f.probe())
	st.Plan = shellAuditPlan{Mode: shellAuditModeInteractive, Samples: 3, Warmup: 1}

	session := startShellAuditSession(st, f.probe())
	events := engineEvents(session)

	var parts []shellAuditPart
	var started, finished int
	for _, event := range events {
		switch event.Outcome {
		case shellAuditOutcomeStarted:
			started++
			if event.Progress.Run < 1 || event.Progress.Run > event.Progress.Total {
				t.Errorf("a start reported its place as %d of %d", event.Progress.Run, event.Progress.Total)
			}
		case shellAuditOutcomeFinished:
			finished++
			if event.Duration <= 0 {
				t.Errorf("a finished start reported the duration %v", event.Duration)
			}
		}
		if event.Done {
			parts = append(parts, event.Progress.Part)
		}
	}

	if started != 4 || finished != 4 {
		t.Errorf("%d starts began and %d finished, want the one warm-up and the three runs", started, finished)
	}

	wantParts := []shellAuditPart{
		shellAuditPartVersion,
		shellAuditPartWarmup, shellAuditPartWarmup,
		shellAuditPartConfigured, shellAuditPartConfigured,
		shellAuditPartConfigured, shellAuditPartConfigured,
		shellAuditPartConfigured, shellAuditPartConfigured,
		shellAuditPartProfile,
		shellAuditPartDone,
	}
	got := make([]shellAuditPart, 0, len(events))
	for _, event := range events {
		got = append(got, event.Progress.Part)
	}

	// The sequence is compared as it was reported, not as a set: a screen told
	// the wrong order would draw a warm-up as a sample.
	if strings.Join(partNames(got), ",") != strings.Join(partNames(wantParts), ",") {
		t.Errorf("the parts reported were %v, want %v", partNames(got), partNames(wantParts))
	}
	if len(parts) != 1 || parts[0] != shellAuditPartDone {
		t.Errorf("the measurement reported its end as %v", parts)
	}

	// The progress labels are what the notice draws, so an empty one would be a
	// blank line where the run in flight should be.
	for _, progress := range []shellAuditProgress{
		{Part: shellAuditPartVersion},
		{Part: shellAuditPartWarmup, Run: 1, Total: 1},
		{Part: shellAuditPartConfigured, Run: 2, Total: 5},
		{Part: shellAuditPartBaseline, Run: 3, Total: 5},
		{Part: shellAuditPartProfile},
	} {
		if progress.label() == "" {
			t.Errorf("the progress %+v has no label", progress)
		}
	}
}

// partNames turns parts into comparable strings.
func partNames(parts []shellAuditPart) []string {
	names := make([]string, 0, len(parts))
	for _, part := range parts {
		names = append(names, string(part))
	}
	return names
}

// TestShellAuditCancelsTheStartsThatAreLeft is the cancel: the runs that finished
// are kept, nothing that did not run is invented, and the runs that were stopped
// are neither timeouts nor failures.
func TestShellAuditCancelsTheStartsThatAreLeft(t *testing.T) {
	f := &fakeShellAuditProbe{
		env:      map[string]string{"SHELL": "/usr/bin/zsh", "HOME": "/home/alberto"},
		terminal: true,
		timeout:  time.Second,
		runs:     5,
		script:   []time.Duration{100 * time.Millisecond, 120 * time.Millisecond},
		// The third start waits for the deadline the way a plugin waiting on the
		// network does, so the cancel arrives while a start is really in flight.
		hangAt: map[int]bool{2: true, 3: true, 4: true},
	}

	st := resolveShellAudit(f.probe())
	session := startShellAuditSession(st, f.probe())

	finished := 0
	for event := range session.events {
		if event.Outcome == shellAuditOutcomeFinished && event.Progress.Part == shellAuditPartConfigured {
			finished++
			if finished == 2 {
				session.cancel()
			}
		}
		if event.Done {
			st = st.withShellAuditResult(event)
		}
	}

	if !st.Cancelled {
		t.Error("the measurement does not report that the user stopped it")
	}
	if len(st.Samples) != 2 {
		t.Fatalf("%d samples, want the two starts that finished before the cancel", len(st.Samples))
	}
	if st.Timeouts != 0 || st.Failures != 0 {
		t.Errorf("the cancel was counted as %d timeouts and %d failures, want neither: a stop says nothing about the shell",
			st.Timeouts, st.Failures)
	}
	if len(f.calls) != 3 {
		t.Errorf("the shell was started %d times, want the two runs and the one the cancel cut", len(f.calls))
	}
	if st.AttributionReason == "" {
		t.Error("a cancelled measurement kept no reason for having no attribution")
	}
	if st.Measured && st.Stats.Median != 110*time.Millisecond {
		t.Errorf("the median of the two finished runs = %v, want 110 ms", st.Stats.Median)
	}
}

// TestShellAuditKeepsThePlanACancelledRunDidNotFinish pins the plan a cancel
// leaves behind: the plan is what the user chose, never what a stopped
// measurement managed to do. A plan rewritten to two runs would make the next
// press measure two runs.
func TestShellAuditKeepsThePlanACancelledRunDidNotFinish(t *testing.T) {
	f := &fakeShellAuditProbe{
		env:      map[string]string{"SHELL": "/usr/bin/zsh", "HOME": "/home/alberto"},
		terminal: true,
		timeout:  time.Second,
		runs:     5,
		script:   []time.Duration{10 * time.Millisecond},
		hangAt:   map[int]bool{1: true, 2: true, 3: true, 4: true},
	}

	st := resolveShellAudit(f.probe())
	session := startShellAuditSession(st, f.probe())

	for event := range session.events {
		if event.Outcome == shellAuditOutcomeFinished {
			session.cancel()
		}
		if event.Done {
			st = st.withShellAuditResult(event)
		}
	}

	if st.Plan.Samples != 5 {
		t.Errorf("the plan after the cancel = %d runs, want the five the user chose", st.Plan.Samples)
	}
	if st.Runs != 5 {
		t.Errorf("the state reports %d runs, want the plan's five", st.Runs)
	}
	if len(st.Samples) != 1 {
		t.Errorf("%d samples, want the one that finished", len(st.Samples))
	}
}

// TestShellAuditCountsAWarmupThatDidNotFinish pins the warm-up's own failures:
// a warm-up start that timed out is still a fact about the shell, so it is
// counted where every other start that produced no duration is counted, and it
// never becomes a sample.
func TestShellAuditCountsAWarmupThatDidNotFinish(t *testing.T) {
	f := &fakeShellAuditProbe{
		env:      map[string]string{"SHELL": "/usr/bin/zsh", "HOME": "/home/alberto"},
		terminal: true,
		timeout:  20 * time.Millisecond,
		runs:     2,
		script:   []time.Duration{100 * time.Millisecond, 200 * time.Millisecond},
		hangAt:   map[int]bool{0: true},
	}

	st := resolveShellAudit(f.probe())
	st.Plan = shellAuditPlan{Mode: shellAuditModeInteractive, Samples: 2, Warmup: 1}
	st = measureShellAudit(st, f.probe())

	if len(st.Warmup) != 0 {
		t.Errorf("a warm-up start that timed out was recorded as %v", st.Warmup)
	}
	if st.Timeouts != 1 {
		t.Errorf("timeouts = %d, want the warm-up start that did not finish", st.Timeouts)
	}
	if len(st.Samples) != 2 {
		t.Errorf("%d samples, want the two the plan asked for", len(st.Samples))
	}
}

// TestShellAuditClampsAPlanItCannotHonour pins the bounds: a plan that survived a
// hand-edited state file cannot ask for a thousand starts, and a zeroed plan is
// repaired to a real one instead of measuring nothing.
//
// The mode is deliberately not repaired. A non-interactive start is the zero
// value of shellAuditMode, so "the plan is zeroed, make it the interactive one"
// would silently measure a different start than the one a user who chose the
// non-interactive start asked for.
func TestShellAuditClampsAPlanItCannotHonour(t *testing.T) {
	tests := []struct {
		name string
		plan shellAuditPlan
		want shellAuditPlan
	}{
		{"a zeroed plan", shellAuditPlan{}, shellAuditPlan{Mode: shellAuditModeNonInteractive, Samples: shellAuditRuns}},
		{"a non-interactive plan keeps its mode", shellAuditPlan{Mode: shellAuditModeNonInteractive, Samples: 3},
			shellAuditPlan{Mode: shellAuditModeNonInteractive, Samples: 3}},
		{"more samples than the bound", shellAuditPlan{Samples: 999}, shellAuditPlan{Samples: shellAuditMaxSamples}},
		{"a negative warm-up", shellAuditPlan{Samples: 3, Warmup: -4}, shellAuditPlan{Samples: 3, Warmup: 0}},
		{"more warm-up than the bound", shellAuditPlan{Samples: 3, Warmup: 99}, shellAuditPlan{Samples: 3, Warmup: shellAuditMaxWarmup}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clampShellAuditPlan(tt.plan)
			if got != tt.want {
				t.Errorf("the clamped plan = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestShellAuditPlanWorstCaseIsBounded pins the promise the screen makes about
// what one press can cost: every start, the version probe and the profiled start
// are bounded, and the bound is a number this guard can re-derive.
func TestShellAuditPlanWorstCaseIsBounded(t *testing.T) {
	const timeout = 10 * time.Second

	quick := shellAuditPlan{Mode: shellAuditModeInteractive, Samples: 5}
	if got, want := quick.worstCase(timeout), 5*timeout+shellAuditVersionTimeout+timeout; got != want {
		t.Errorf("the quick plan's worst case = %v, want %v", got, want)
	}

	thorough := shellAuditPlan{Mode: shellAuditModeInteractive, Samples: 15, Warmup: 1, Baseline: true}
	if got, want := thorough.worstCase(timeout), 2*(16*timeout)+shellAuditVersionTimeout+timeout; got != want {
		t.Errorf("the thorough plan's worst case = %v, want %v", got, want)
	}

	// A clean start's baseline is the start itself, so it is not paid for twice.
	clean := shellAuditPlan{Mode: shellAuditModeClean, Samples: 5, Baseline: true}
	if got, want := clean.worstCase(timeout), 5*timeout+shellAuditVersionTimeout+timeout; got != want {
		t.Errorf("the clean plan's worst case = %v, want %v", got, want)
	}
}

// TestShellAuditRefusesAStartTheShellDoesNotOffer pins the engine's own refusal:
// a plan asking for a start this shell does not have measures nothing, says why,
// and starts no shell at all.
func TestShellAuditRefusesAStartTheShellDoesNotOffer(t *testing.T) {
	f := zshAuditFixture()
	f.env["SHELL"] = "/usr/bin/elvish"

	st := resolveShellAudit(f.probe())
	if st.Kind != shellAuditKindUnknown {
		t.Fatalf("the fixture's shell = %q, want an unknown family", st.Kind)
	}
	st.Plan = shellAuditPlan{Mode: shellAuditModeLogin, Samples: 3}
	st = measureShellAudit(st, f.probe())

	if len(st.Samples) != 0 {
		t.Errorf("%d samples, want none: the start was refused", len(st.Samples))
	}
	if st.RunReason == "" {
		t.Error("the refusal kept no reason")
	}
	if st.Reason != "" {
		t.Errorf("the refusal was recorded as an availability reason (%q), which would take the row away", st.Reason)
	}
	// The reason names the cause, which is the shell rather than the mode: an
	// unknown shell offers one start and the plan asked for another.
	if !strings.Contains(st.RunReason, "unknown shell") {
		t.Errorf("the reason %q does not say that this shell's starts are unknown", st.RunReason)
	}
	if st.Plan.Mode != shellAuditModeLogin {
		t.Errorf("the refused plan = %s, want the login start it asked for", st.Plan.Mode.label())
	}
	if len(f.calls) != 0 {
		t.Errorf("the shell was started %d times for a start it does not offer", len(f.calls))
	}
}

// TestShellAuditMeasuresTheStartItWasAskedFor pins the mode the run really makes:
// the login start passes -l, the non-interactive one passes no -i, and the line
// on screen is the line that ran.
func TestShellAuditMeasuresTheStartItWasAskedFor(t *testing.T) {
	tests := []struct {
		name     string
		mode     shellAuditMode
		wantLine string
	}{
		{"interactive", shellAuditModeInteractive, "zsh -i -c exit"},
		{"login", shellAuditModeLogin, "zsh -l -i -c exit"},
		{"non-interactive", shellAuditModeNonInteractive, "zsh -c exit"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := zshAuditFixture()
			st := resolveShellAudit(f.probe())
			st.Plan = shellAuditPlan{Mode: tt.mode, Samples: 2}
			st = measureShellAudit(st, f.probe())

			if st.Mode != tt.mode {
				t.Errorf("the measured start = %q, want %q", st.Mode.label(), tt.mode.label())
			}
			if st.Command != tt.wantLine {
				t.Errorf("the command on screen = %q, want %q", st.Command, tt.wantLine)
			}
			for i := 0; i < 2; i++ {
				if got := strings.Join(f.calls[i], " "); got != "/usr/bin/zsh "+strings.TrimPrefix(tt.wantLine, "zsh ") {
					t.Errorf("run %d = %q, want the command the screen printed", i+1, got)
				}
			}
		})
	}
}

// TestShellAuditRecordsTheVersionItRanWith pins the context: the version the
// shell printed is kept on the state, and it is what the capability matrix
// decides the instrument with.
func TestShellAuditRecordsTheVersionItRanWith(t *testing.T) {
	f := zshAuditFixture()
	f.versionRaw = "zsh 5.9.2 (x86_64-pc-linux-gnu)\n"

	st := measureShellAudit(resolveShellAudit(f.probe()), f.probe())

	if !st.Version.Known || st.Version.Number != "5.9.2" {
		t.Errorf("the version = %+v, want zsh 5.9.2", st.Version)
	}
	if st.Kind != shellAuditKindZsh {
		t.Errorf("the family = %q, want zsh", st.Kind)
	}
}

// TestShellAuditStateAndRecordStayEachOthersInverse pins the projection and its
// inverse against each other. The screen draws the flat fields, the comparison,
// the history and the stored trend read the record, so a field that one of them
// carries and the other drops would be a measurement the utility has and cannot
// print -- and it would be dropped silently, which is the worst way for a number
// to go missing.
func TestShellAuditStateAndRecordStayEachOthersInverse(t *testing.T) {
	run := shellAuditRun{
		Kind:         shellAuditKindZsh,
		Shell:        "zsh",
		Path:         "/usr/bin/zsh",
		Version:      shellAuditVersion{Known: true, Raw: "zsh 5.9.2", Number: "5.9.2", Major: 5, Minor: 9},
		Mode:         shellAuditMode{Login: true, Interactive: true},
		Command:      "zsh -l -i -c exit",
		Terminal:     true,
		TerminalName: "iTerm.app 3.5.0",
		Note:         "a note the start carries",
		Reason:       "",

		Samples:  []time.Duration{400 * time.Millisecond, 410 * time.Millisecond, 420 * time.Millisecond},
		Warmup:   []time.Duration{900 * time.Millisecond},
		Timeouts: 1,
		Failures: 2,
		Failure:  "exit status 127",

		Stats: shellAuditStatsOf([]time.Duration{400 * time.Millisecond, 410 * time.Millisecond, 420 * time.Millisecond}),

		Attribution: []shellAuditFunction{
			{Name: "compinit", Kind: shellAuditAttributionFunction, Self: 300 * time.Millisecond, Calls: 1, Percent: "75.00%"},
		},
		AttributionReason: "",

		MeasuredAt: time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC),
		Took:       3 * time.Second,
		Cancelled:  true,
	}

	// The state is a projection of the record, so the way back has to be the
	// record itself -- for every field, including the ones only the history reads.
	state := (shellAuditState{}).withShellAuditResult(shellAuditEvent{Result: run, Plan: shellAuditPlan{Samples: 3}})
	if got := state.shellAuditRun(); !reflect.DeepEqual(got, run) {
		t.Errorf("the round trip lost or changed a field:\n got %+v\nwant %+v", got, run)
	}
}
