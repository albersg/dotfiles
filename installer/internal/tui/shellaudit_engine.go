package tui

import (
	"context"
	"fmt"
	"time"
)

// ============================================================================
// THE MEASUREMENT ITSELF
// ============================================================================
//
// One measurement is a plan: which start, how many starts, how many of them are
// warm-up, and whether the same start is measured again with none of the user's
// configuration. It runs on its own goroutine and reports each start as it
// lands, so the screen can draw the run in flight, cancel the rest, and keep
// whatever finished before the cancel.
//
// Three rules hold from the first start to the last, and every one of them is a
// rule the old single-command measurement could not keep:
//
//   - A start that does not finish inside its bound is a timeout and never a
//     duration. The plan is bounded too, so the worst case of the whole
//     measurement is a number this file can state.
//   - A start the user cancels is neither: it is reported as cancelled, the runs
//     that finished are kept, and no summary is invented from the ones that did
//     not run.
//   - Nothing is profiled twice. The profiled start is an extra start of its
//     own, never one of the samples, because an instrument costs time of its own
//     and a median that included it would describe a shell nobody opens.

const (
	// shellAuditMaxSamples bounds the sample count a plan may ask for, so a
	// corrupt or hand-edited state cannot turn one press into an hour of starts.
	shellAuditMaxSamples = 20

	// shellAuditMaxWarmup bounds the discarded starts. One is what a cold start
	// needs (the completion dump being rebuilt, the disk cache being read); more
	// than a couple is a measurement of how long the machine takes to warm up.
	shellAuditMaxWarmup = 3

	// shellAuditThoroughRuns is the second sample size the screen offers. Fifteen
	// is the first count at which a 95th percentile is far enough from the
	// slowest run to say something the range does not, and it is one warm-up
	// start on top.
	shellAuditThoroughRuns = 15

	// shellAuditEventSlack is how many events beyond one start's worth the event
	// channel holds. The channel is big enough for the whole measurement, so the
	// goroutine that runs it can always finish: a goroutine blocked on a send to
	// a screen that stopped reading is a leak with a shell attached.
	shellAuditEventSlack = 8

	// shellAuditCancelledReason is what the screen says where an attribution
	// would be when the user stopped the measurement before the profiled start.
	shellAuditCancelledReason = "the measurement was cancelled before the profiled start, so nothing inside the " +
		"startup can be named: only the runs that finished above stand."
)

// shellAuditSampleSize is one of the sizes the screen cycles: how many starts are
// samples and how many are warm-up. It is a pair because they move together: the
// thorough measurement is the one that pays for a warm-up start.
type shellAuditSampleSize struct {
	Samples int
	Warmup  int
	// Label is the phrase the row draws, so the row and the plan cannot disagree.
	Label string
}

// shellAuditSampleSizes is the list, in the order the row cycles.
func shellAuditSampleSizes() []shellAuditSampleSize {
	return []shellAuditSampleSize{
		{Samples: shellAuditRuns, Warmup: 0, Label: fmt.Sprintf("%d runs", shellAuditRuns)},
		{Samples: shellAuditThoroughRuns, Warmup: 1, Label: fmt.Sprintf("%d runs and one warm-up", shellAuditThoroughRuns)},
	}
}

// shellAuditPlan is what the next measurement will do. It is the screen's state,
// not the measurement's: the run it produces is a shellAuditRun, and the plan
// can be changed while nothing is running.
type shellAuditPlan struct {
	// Mode is the start to measure: interactive, login or not, and with or
	// without the user's configuration.
	Mode shellAuditMode
	// Samples is how many starts the summary covers.
	Samples int
	// Warmup is how many starts are made and thrown away before the first
	// sample. Zero is a plan without a warm-up, which is the honest default: a
	// warm-up start costs a real start of the user's shell.
	Warmup int
	// Baseline asks for the same start measured again with none of the user's
	// configuration, for the one comparison that turns "the shell takes 900 ms"
	// into "your configuration costs 700 ms of it".
	Baseline bool
}

// shellAuditPlanDefault is the plan a resolved state starts with: the
// interactive start, the shipped run count, no warm-up, no baseline. It is the
// measurement the utility made before this front, so nothing a reader of the
// old numbers expects changes under them.
func shellAuditPlanDefault(samples int) shellAuditPlan {
	if samples < 1 {
		samples = shellAuditRuns
	}
	return shellAuditPlan{Mode: shellAuditModeInteractive, Samples: samples}
}

// clampShellAuditPlan bounds what a plan may ask for and repairs a zeroed one, so
// a plan that survived a hand-edited state file or a future screen cannot start
// unbounded work.
func clampShellAuditPlan(plan shellAuditPlan) shellAuditPlan {
	if plan.Samples < 1 {
		plan.Samples = shellAuditRuns
	}
	if plan.Samples > shellAuditMaxSamples {
		plan.Samples = shellAuditMaxSamples
	}
	if plan.Warmup < 0 {
		plan.Warmup = 0
	}
	if plan.Warmup > shellAuditMaxWarmup {
		plan.Warmup = shellAuditMaxWarmup
	}
	return plan
}

// sizeLabel is the plan's size as the screen draws it. It is derived from the
// numbers rather than stored, so a plan that was clamped says what it will
// really do.
func (p shellAuditPlan) sizeLabel() string {
	label := fmt.Sprintf("%d runs", p.Samples)
	if p.Warmup > 0 {
		label += fmt.Sprintf(", %d warm-up", p.Warmup)
	}
	return label
}

// worstCase is the longest a whole measurement of this plan can take, including
// the version probe, the profiled start and the baseline. It is what the screen
// promises the user is the most one press can cost.
func (p shellAuditPlan) worstCase(timeout time.Duration) time.Duration {
	runs := p.Warmup + p.Samples
	if p.Baseline && !p.Mode.Clean {
		runs *= 2
	}
	// The version probe is bounded by its own timeout and the profiled start is
	// one more run at the same bound.
	return time.Duration(runs)*timeout + shellAuditVersionTimeout + timeout
}

// shellAuditPart names the part of the plan a start belongs to. The list is what
// the engine really does and nothing else, so a screen cannot report a part the
// measurement never ran.
type shellAuditPart string

const (
	shellAuditPartVersion        shellAuditPart = "version"
	shellAuditPartWarmup         shellAuditPart = "warm-up"
	shellAuditPartConfigured     shellAuditPart = "configured"
	shellAuditPartBaselineWarmup shellAuditPart = "baseline-warm-up"
	shellAuditPartBaseline       shellAuditPart = "baseline"
	shellAuditPartProfile        shellAuditPart = "profile"
	shellAuditPartDone           shellAuditPart = "done"
)

// shellAuditProgress is where a measurement is: which part, and the start's
// place among the starts of that part. It is derived from the events, so the
// screen draws the run in flight rather than only that something is running.
type shellAuditProgress struct {
	Part  shellAuditPart
	Run   int
	Total int
}

// label is the phrase the notice draws for the run in flight.
func (p shellAuditProgress) label() string {
	switch p.Part {
	case shellAuditPartVersion:
		return "reading the shell's version"
	case shellAuditPartWarmup:
		return fmt.Sprintf("warm-up %d of %d", p.Run, p.Total)
	case shellAuditPartBaselineWarmup:
		return fmt.Sprintf("clean baseline warm-up %d of %d", p.Run, p.Total)
	case shellAuditPartBaseline:
		return fmt.Sprintf("clean baseline, run %d of %d", p.Run, p.Total)
	case shellAuditPartProfile:
		return "the profiled start"
	case shellAuditPartConfigured:
		return fmt.Sprintf("run %d of %d", p.Run, p.Total)
	default:
		return ""
	}
}

// shellAuditOutcome is how one start ended. It is an enum rather than a boolean
// because the three ways of not producing a duration are three different facts:
// a timeout says the startup can hang, a failure says the shell could not be
// started, and a cancel says the user stopped asking.
type shellAuditOutcome string

const (
	// shellAuditOutcomeStarted is the event a start sends when it begins, before
	// there is an outcome to report.
	shellAuditOutcomeStarted shellAuditOutcome = "started"
	// shellAuditOutcomeFinished is a start with a duration.
	shellAuditOutcomeFinished shellAuditOutcome = "finished"
	// shellAuditOutcomeTimedOut is a start killed at the bound.
	shellAuditOutcomeTimedOut shellAuditOutcome = "timed out"
	// shellAuditOutcomeFailed is a start that could not be made at all.
	shellAuditOutcomeFailed shellAuditOutcome = "failed"
	// shellAuditOutcomeCancelled is a start the user stopped.
	shellAuditOutcomeCancelled shellAuditOutcome = "cancelled"
)

// shellAuditEvent is one thing that happened during a measurement. It is small
// on purpose: the screen needs the progress and, at the end, the result.
type shellAuditEvent struct {
	Progress shellAuditProgress
	// Duration is how long the start took, and zero for an event that is not a
	// finished start.
	Duration time.Duration
	Outcome  shellAuditOutcome
	// Err is why a failed start failed. It is the only place an error's text
	// reaches the screen, and it is the shell's own complaint, never output the
	// start printed.
	Err string
	// Done reports the last event of the measurement, which carries the result.
	Done bool
	// Result is the finished measurement, set on the last event only.
	Result shellAuditRun
	// Baseline is the clean measurement, when the plan asked for one and one was
	// made.
	Baseline *shellAuditRun
	// Reason is why the measurement produced no start at all, such as a clean
	// start that could not be prepared. It is set on the last event only.
	Reason string
	// Plan is the plan the measurement really ran: the state's plan after it was
	// clamped. It is set on the last event only, and it is what the next
	// measurement keeps, so a cancelled run cannot shrink a user's plan to the
	// starts that happened to finish.
	Plan shellAuditPlan
}

// shellAuditRun is one complete measurement of one start: the context it was
// made in, the starts that finished, the ones that did not, and what the shell's
// own instrument named.
//
// It is the record the state is a projection of, the record the history keeps a
// redacted summary of, and the record the baseline is compared against. There is
// one shape for a measurement, so the screen, the history and the comparison can
// never disagree about what was measured.
type shellAuditRun struct {
	Kind    shellAuditKind
	Shell   string
	Path    string
	Version shellAuditVersion
	Mode    shellAuditMode
	Command string
	// Timeout is the bound each start was given, recorded because a start killed
	// at it has no duration and the bound is what says so.
	Timeout      time.Duration
	Terminal     bool
	TerminalName string
	Note         string
	// Reason is why this start could not be made at all, in the screen's words.
	Reason string

	Samples  []time.Duration
	Warmup   []time.Duration
	Timeouts int
	Failures int
	Failure  string

	Stats shellAuditStats

	Attribution       []shellAuditFunction
	AttributionReason string

	MeasuredAt time.Time
	// Took is the wall clock of the whole measurement: the starts, the version
	// probe and the profiled start.
	Took time.Duration
	// Cancelled reports that the user stopped this measurement.
	Cancelled bool
}

// shellAuditSession is one measurement in flight. The events are buffered for
// the whole measurement, so the goroutine that runs it never waits on the
// screen: a screen that closed, a user who left, or a read that was never armed
// cannot keep a shell process alive.
type shellAuditSession struct {
	events chan shellAuditEvent
	cancel context.CancelFunc
}

// startShellAuditSession starts one measurement on its own goroutine. The
// caller reads the events and re-arms the read after each one; the session must
// not be started for a state that is not available, because there would be
// nothing to start.
func startShellAuditSession(st shellAuditState, probe shellAuditProbe) *shellAuditSession {
	plan := clampShellAuditPlan(st.Plan)
	capacity := 2*(2*plan.Samples+2*plan.Warmup+2) + shellAuditEventSlack
	events := make(chan shellAuditEvent, capacity)
	ctx, cancel := context.WithCancel(context.Background())
	session := &shellAuditSession{events: events, cancel: cancel}

	go func() {
		defer close(events)
		run, baseline, reason := runShellAuditMeasurement(st, probe, ctx, func(event shellAuditEvent) {
			events <- event
		})
		events <- shellAuditEvent{
			Progress: shellAuditProgress{Part: shellAuditPartDone},
			Done:     true,
			Result:   run,
			Baseline: baseline,
			Reason:   reason,
			Plan:     plan,
		}
		// Released after the last event, never before: the cancel is what stops
		// the starts, and a cancel fired by this goroutine's own exit would mark a
		// finished measurement as cancelled.
		cancel()
	}()

	return session
}

// drainShellAuditSession reads a session to its end and applies the result. It
// is what a guard and the synchronous measurement use; the screen reads the
// channel itself so it can draw each start as it lands.
func drainShellAuditSession(st shellAuditState, session *shellAuditSession) shellAuditState {
	for event := range session.events {
		if event.Done {
			st = st.withShellAuditResult(event)
		}
	}
	return st
}

// withShellAuditResult folds a finished measurement into the state. It is the
// only place the flat fields are written from a run, so there is one projection
// and no screen can read a number the record does not hold.
func (st shellAuditState) withShellAuditResult(event shellAuditEvent) shellAuditState {
	run := event.Result

	st.Measuring = false
	st.Progress = shellAuditProgress{}
	st.Cancelled = run.Cancelled
	st.Reason = event.Reason
	st.RunReason = run.Reason

	if event.Reason != "" {
		// Nothing was measured: keep whatever the state already had, so a refused
		// clean start does not erase the measurement that was on screen.
		return st
	}

	st.Measured = true
	st.Shell = run.Shell
	st.ShellPath = run.Path
	st.Kind = run.Kind
	st.Version = run.Version
	st.Mode = run.Mode
	st.Command = run.Command
	st.Timeout = run.Timeout
	st.Terminal = run.Terminal
	st.TerminalName = run.TerminalName
	st.Note = run.Note
	st.Samples = run.Samples
	st.Warmup = run.Warmup
	st.Timeouts = run.Timeouts
	st.Failures = run.Failures
	st.Failure = run.Failure
	st.Stats = run.Stats
	st.Median, st.Fastest, st.Slowest = run.Stats.Median, run.Stats.Fastest, run.Stats.Slowest
	st.Attribution = run.Attribution
	st.AttributionReason = run.AttributionReason
	st.MeasuredAt = run.MeasuredAt
	st.Took = run.Took
	// The plan the measurement really ran is the plan the next one keeps, and the
	// run's own start is what the plan describes. The counts that happened are in
	// the samples, the timeouts and the failures, never in the plan: a cancelled
	// measurement that rewrote the plan to the two starts it managed would make
	// the next press measure two starts.
	st.Plan = clampShellAuditPlan(event.Plan)
	st.Plan.Mode = run.Mode
	st.Plan.Baseline = event.Baseline != nil || (st.Plan.Baseline && !run.Mode.Clean)
	st.Runs = st.Plan.Samples
	st.Baseline = event.Baseline

	// Where this measurement sits among the stored ones, and then the stored ones
	// themselves. The trend is read before the entry is added, and the entry it is
	// read against is the one with this measurement's own stamp, so reading the
	// state twice cannot turn one measurement into a trend of two.
	entry := shellAuditHistoryEntryOf(run, compareShellAuditBaseline(&run, st.Baseline))
	st.Trend = st.History.trend(entry)
	if st.History.Enabled {
		st.History = st.History.add(entry)
	}

	return st
}

// shellAuditRun rebuilds the record a state is a projection of. The state and the
// record are two views of one measurement: the screen draws the flat fields, and
// the comparison, the history and the stored trend all read the record.
//
// The inverse lives here, next to withShellAuditResult, because the two have to
// stay each other's inverse; a guard round-trips a run through both so a field
// added to one is a failing test rather than a silently dropped number.
func (st shellAuditState) shellAuditRun() shellAuditRun {
	return shellAuditRun{
		Kind:         st.Kind,
		Shell:        st.Shell,
		Path:         st.ShellPath,
		Version:      st.Version,
		Mode:         st.Mode,
		Command:      st.Command,
		Timeout:      st.Timeout,
		Terminal:     st.Terminal,
		TerminalName: st.TerminalName,
		Note:         st.Note,
		Reason:       st.RunReason,

		Samples:  st.Samples,
		Warmup:   st.Warmup,
		Timeouts: st.Timeouts,
		Failures: st.Failures,
		Failure:  st.Failure,

		Stats: st.Stats,

		Attribution:       st.Attribution,
		AttributionReason: st.AttributionReason,

		MeasuredAt: st.MeasuredAt,
		Took:       st.Took,
		Cancelled:  st.Cancelled,
	}
}

// runShellAuditMeasurement is the engine: it makes every start the plan asks
// for, emits one event per start, and returns the record. The third return is a
// reason the measurement could not be made at all, which is a different thing
// from a start that failed.
func runShellAuditMeasurement(st shellAuditState, probe shellAuditProbe, ctx context.Context,
	emit func(shellAuditEvent)) (shellAuditRun, *shellAuditRun, string) {

	plan := clampShellAuditPlan(st.Plan)
	began := probe.now()

	run := shellAuditRun{
		Kind:         shellAuditKindOf(st.Shell),
		Shell:        st.Shell,
		Path:         st.ShellPath,
		Mode:         plan.Mode,
		Timeout:      probe.timeout,
		Terminal:     st.Terminal,
		TerminalName: st.TerminalName,
		MeasuredAt:   began,
	}

	// The version is asked once, before any start, because two of the four
	// instruments are only available above a version and the context line names
	// the one this measurement was made with.
	emit(shellAuditEvent{Progress: shellAuditProgress{Part: shellAuditPartVersion}})
	run.Version = shellAuditVersionOf(probe, ctx, st.ShellPath)
	profiler, profilerReason := shellAuditProfilerFor(run.Kind, run.Version, run.Mode)

	// A clean start -- the plan's own, or the baseline's -- is pointed at an
	// empty directory of the utility's. It is never the user's configuration
	// moved aside, and it is never an empty path: an empty ZDOTDIR is the home
	// directory to a shell.
	cleanDir := ""
	if plan.Mode.Clean || plan.Baseline {
		dir, err := probe.mkdirTemp()
		if err != nil {
			return run, nil, fmt.Sprintf("a clean start needs a directory of the utility's own to be pointed at, and "+
				"one could not be prepared (%v), so nothing is measurable: the run would have read the user's own files "+
				"instead.", err)
		}
		cleanDir = dir
		defer probe.removeAll(cleanDir)
	}

	runner := &shellAuditRunner{state: st, probe: probe, ctx: ctx, emit: emit, cleanDir: cleanDir}
	run.Cancelled = runner.measure(&run, plan.Mode, plan.Warmup, plan.Samples)

	if run.Reason == "" {
		if run.Cancelled {
			run.AttributionReason = shellAuditCancelledReason
		} else {
			// The instrument is loaded as part of the start it measures, so the
			// profiled start is a start of its own and never one of the samples.
			emit(shellAuditEvent{Progress: shellAuditProgress{Part: shellAuditPartProfile, Run: 1, Total: 1}})
			run.Attribution, run.AttributionReason = profileShellAudit(run, probe, ctx, profiler, profilerReason)
		}
	} else if profilerReason != "" {
		run.AttributionReason = profilerReason
	}

	var baseline *shellAuditRun
	if plan.Baseline && !plan.Mode.Clean && !run.Cancelled && run.Reason == "" {
		clean := shellAuditRun{
			Kind:         run.Kind,
			Shell:        run.Shell,
			Path:         run.Path,
			Version:      run.Version,
			Terminal:     run.Terminal,
			TerminalName: run.TerminalName,
			MeasuredAt:   probe.now(),
		}
		baselineBegan := clean.MeasuredAt
		cleanMode := shellAuditMode{Interactive: plan.Mode.Interactive, Login: plan.Mode.Login, Clean: true}
		clean.Cancelled = runner.measureBaseline(&clean, cleanMode, plan.Warmup, plan.Samples)
		clean.Stats = shellAuditStatsOf(clean.Samples)
		clean.Took = probe.now().Sub(baselineBegan)
		baseline = &clean
		run.Cancelled = clean.Cancelled
	}

	run.Stats = shellAuditStatsOf(run.Samples)
	run.Took = probe.now().Sub(began)

	return run, baseline, ""
}

// shellAuditVersionOf asks the shell what it is. A probe with no version hook --
// a guard's fixture, or a build without one -- answers an unknown version: not
// knowing costs the context line, never the measurement.
func shellAuditVersionOf(probe shellAuditProbe, ctx context.Context, path string) shellAuditVersion {
	if probe.version == nil {
		return shellAuditVersion{}
	}
	raw, _ := probe.version(ctx, path)
	return shellAuditParseVersion(raw)
}

// shellAuditRunner is one measurement in flight: the state it was started for,
// the probe, the context that cancels it and the way it reports progress.
type shellAuditRunner struct {
	state    shellAuditState
	probe    shellAuditProbe
	ctx      context.Context
	emit     func(shellAuditEvent)
	cleanDir string
}

// measure makes the starts of one mode: the warm-up ones and then the samples.
// It returns whether the user cancelled, and it fills the run it is given --
// including the reason, when the shell does not offer the start at all.
func (r *shellAuditRunner) measure(run *shellAuditRun, mode shellAuditMode, warmup, samples int) bool {
	start, reason := shellAuditStartFor(run.Kind, mode, r.cleanDir)
	if reason != "" {
		run.Reason = reason
		return false
	}

	run.Mode = mode
	run.Command = shellAuditCommandLine(run.Shell, start)
	run.Note = start.Note

	for index := 1; index <= warmup; index++ {
		if r.ctx.Err() != nil {
			return true
		}
		outcome, elapsed, err := r.startOnce(start, shellAuditPartWarmup, index, warmup)
		if outcome == shellAuditOutcomeCancelled {
			return true
		}
		if outcome == shellAuditOutcomeFinished {
			run.Warmup = append(run.Warmup, elapsed)
			continue
		}
		// A warm-up start that timed out or failed is still a fact about the
		// shell, so it is counted where every other non-start is counted.
		r.countNotStarted(run, outcome, err)
	}

	for index := 1; index <= samples; index++ {
		if r.ctx.Err() != nil {
			return true
		}
		outcome, elapsed, err := r.startOnce(start, shellAuditPartConfigured, index, samples)
		if outcome == shellAuditOutcomeCancelled {
			return true
		}
		if outcome == shellAuditOutcomeFinished {
			run.Samples = append(run.Samples, elapsed)
			continue
		}
		r.countNotStarted(run, outcome, err)
	}

	return false
}

// measureBaseline is measure for the clean comparison, reported under its own
// part so the screen can tell which of the two it is drawing.
func (r *shellAuditRunner) measureBaseline(run *shellAuditRun, mode shellAuditMode, warmup, samples int) bool {
	for index := 1; index <= warmup; index++ {
		if r.ctx.Err() != nil {
			return true
		}
		start, reason := shellAuditStartFor(run.Kind, mode, r.cleanDir)
		if reason != "" {
			run.Reason = reason
			return false
		}
		outcome, _, err := r.startOnce(start, shellAuditPartBaselineWarmup, index, warmup)
		if outcome == shellAuditOutcomeCancelled {
			return true
		}
		r.countNotStarted(run, outcome, err)
	}

	for index := 1; index <= samples; index++ {
		if r.ctx.Err() != nil {
			return true
		}
		start, reason := shellAuditStartFor(run.Kind, mode, r.cleanDir)
		if reason != "" {
			run.Reason = reason
			return false
		}
		if index == 1 {
			run.Mode = mode
			run.Command = shellAuditCommandLine(run.Shell, start)
			run.Note = start.Note
		}
		outcome, elapsed, err := r.startOnce(start, shellAuditPartBaseline, index, samples)
		if outcome == shellAuditOutcomeCancelled {
			return true
		}
		if outcome == shellAuditOutcomeFinished {
			run.Samples = append(run.Samples, elapsed)
			continue
		}
		r.countNotStarted(run, outcome, err)
	}

	return false
}

// countNotStarted records a start that produced no duration. Neither a timeout
// nor a failure is ever folded into the samples as one: the two are counted, the
// first failure's own words are kept, and a screen that wants to say how many
// starts did not finish has the numbers to say it with.
func (r *shellAuditRunner) countNotStarted(run *shellAuditRun, outcome shellAuditOutcome, err error) {
	switch outcome {
	case shellAuditOutcomeTimedOut:
		run.Timeouts++
	case shellAuditOutcomeFailed:
		run.Failures++
		if run.Failure == "" && err != nil {
			run.Failure = err.Error()
		}
	}
}

// startOnce makes one start of the shell, bounded by its own deadline, and says
// how it ended.
//
// The timing comes from the probe's clock, which is time.Now: it carries a
// monotonic reading, so a wall-clock adjustment in the middle of a measurement
// -- an NTP step, a laptop waking up -- cannot produce a negative sample or an
// inflated one. The start gets our own stdin, so a hook that reads the terminal
// reads nothing and ends, and a hook that waits is killed at the bound.
func (r *shellAuditRunner) startOnce(start shellAuditStart, part shellAuditPart, index, total int) (shellAuditOutcome, time.Duration, error) {
	progress := shellAuditProgress{Part: part, Run: index, Total: total}
	r.emit(shellAuditEvent{Progress: progress, Outcome: shellAuditOutcomeStarted})

	ctx, cancel := context.WithTimeout(r.ctx, r.probe.timeout)
	began := r.probe.now()
	_, err := r.probe.run(ctx, r.state.ShellPath, start.Args, start.Env)
	elapsed := r.probe.now().Sub(began)
	// Whether the deadline fired has to be read before the cancellation below, or
	// a finished start would be classified by the cancel this call itself caused.
	timedOut := shellAuditRunTimedOut(ctx, err)
	cancel()

	outcome := shellAuditOutcomeFinished
	switch {
	case err == nil:
		outcome = shellAuditOutcomeFinished
	case r.ctx.Err() != nil:
		// The user stopped the measurement. A start cut by the cancel is not a
		// timeout and not a failure: it is the one outcome that says nothing about
		// the shell.
		outcome = shellAuditOutcomeCancelled
	case timedOut:
		outcome = shellAuditOutcomeTimedOut
	default:
		outcome = shellAuditOutcomeFailed
	}

	r.emit(shellAuditEvent{
		Progress: progress,
		Duration: elapsed,
		Outcome:  outcome,
		Err:      shellAuditErrorText(err),
	})
	return outcome, elapsed, err
}

// shellAuditErrorText is the message a start's error carries, and nothing else.
// os/exec's ExitError already holds the start's own stderr, and that stderr is
// the shell's configuration talking: it can name a variable, a token or a path
// the user meant to keep out of a screen. Only the error's own sentence is kept,
// and the captured output is never read.
func shellAuditErrorText(err error) string {
	if err == nil {
		return ""
	}
	text := err.Error()
	if len(text) > shellAuditErrorLimit {
		text = shellAuditTruncate(text, shellAuditErrorLimit)
	}
	return text
}

// shellAuditErrorLimit is how much of an error's own text a screen draws. It is
// a sentence's length: a start that failed says what failed, not a transcript.
const shellAuditErrorLimit = 160
