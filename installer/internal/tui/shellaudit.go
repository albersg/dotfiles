package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ============================================================================
// THE UTILITIES SECTION: HOW LONG THE SHELL'S OWN STARTUP TAKES
// ============================================================================
//
// This is the utility that changes nothing. The shell's start is something the
// user lives with every day and cannot see: a plugin that waits on the network,
// a completion dump rebuilt on every start and a prompt that reads the whole
// filesystem all look like "the terminal is slow". The utility starts the login
// shell the way a terminal does, several times, and reports the median of those
// starts beside the functions zsh's own profiler blames for them.
//
// Everything here is reading and reporting. No configuration file is opened for
// writing, nothing is disabled, and no recommended edit is applied: the screen
// names what is slow and leaves the decision to the user.

const (
	// shellAuditTimeout bounds every single start of the user's shell. A startup
	// can hang -- a plugin that waits on the network, a prompt that reads a
	// filesystem that is not answering -- and the utility must not hang with it.
	// A start that does not finish inside this bound is reported as a timeout,
	// never as a duration.
	shellAuditTimeout = 10 * time.Second

	// shellAuditWaitDelay bounds the I/O wait that survives that deadline.
	//
	// Cancelling the context kills the shell, but os/exec then waits for the
	// goroutines copying its output pipes: with WaitDelay at its zero value that
	// wait is unbounded, so a background process the startup left behind with the
	// inherited stdout pipe would keep the read blocked past shellAuditTimeout
	// and hang the screen. WaitDelay closes the pipes after the delay, which turns
	// that hang into a bounded failure -- a reported timeout. It must stay
	// positive and below shellAuditTimeout, or the worst case is no longer the
	// bound the screen advertises.
	shellAuditWaitDelay = 1 * time.Second

	// shellAuditRuns is how many times the measured command is started. Five is
	// odd, so the median is a run that really happened rather than the average of
	// two, and it is enough that one cold start -- the first run after boot, when
	// the completion dump is being rebuilt -- does not decide the answer. Five is
	// also the most this may cost: with every start hitting the timeout the whole
	// measurement is bounded by five times shellAuditTimeout.
	shellAuditRuns = 5

	// shellAuditNamedFunctions is how many rows of the shell's own controller the
	// screen names. The list is the instrument's own order, heaviest first, and the
	// count it was cut from is printed beside it, so the cut is declared rather
	// than silent.
	shellAuditNamedFunctions = 5

	// shellAuditVersionFlag is how every shell in the matrix is asked what it is.
	// All four answer it: zsh, bash, fish and nushell print their version for
	// `--version`. A shell that treats the flag as an error prints nothing with a
	// version in it, and its version is then unknown rather than zero.
	shellAuditVersionFlag = "--version"

	// shellAuditVersionTimeout bounds the version probe. It is short on purpose:
	// the probe is context, not the measurement, and no context line is worth a
	// wait on a shell that does not answer.
	shellAuditVersionTimeout = 2 * time.Second
)

// shellAuditFunction is one row the shell's own instrument named, with the
// numbers it printed for it. Total is the time the row and everything it called
// spent, Self is the time the row itself spent, and Percent is the instrument's
// own share cell for Total, kept verbatim so a number on screen can be found in
// the table it came from. The instrument's own order is preserved rather than
// re-sorted: it is the order of the tool that measured, not of this utility.
type shellAuditFunction struct {
	// Kind says what sort of thing the row names: a function the shell profiled, a
	// script it sourced, or a command it ran. Three shells name three different
	// sorts of thing, and a screen that called them all "functions" would be
	// describing two of them wrongly.
	Kind    shellAuditAttributionKind
	Name    string
	Calls   int
	Total   time.Duration
	Self    time.Duration
	Percent string
}

// shellAuditAttributionKind is what one attribution row names.
type shellAuditAttributionKind string

const (
	// shellAuditAttributionFunction is a shell function, the row zprof's own
	// table carries.
	shellAuditAttributionFunction shellAuditAttributionKind = "function"
	// shellAuditAttributionScript is a file the startup sourced, which is the row
	// both fish's profile and bash's trace can name.
	shellAuditAttributionScript shellAuditAttributionKind = "script"
	// shellAuditAttributionCommand is a single command the startup ran.
	shellAuditAttributionCommand shellAuditAttributionKind = "command"
)

// shellAuditState is everything the utility draws: which shell is measured, the
// start it is measured in, the runs and their summary, and what the shell's own
// instrument named -- or the reason it named nothing.
//
// The environment half (Shell, ShellPath, Kind, Version, Terminal, Available,
// Reason) is resolved once when the model is built, because a render path must
// not read the environment. The measurement half is filled only when the row
// inside the screen is pressed: starting the user's shell several times is not
// something a screen may do while drawing.
//
// The flat fields (Shell, Command, Samples, Median, Timeouts, Attribution...)
// are the projection of the last measurement: every screen and every guard that
// already read them keeps its meaning, and the run they came from is the same
// record the history and the baseline comparison use.
type shellAuditState struct {
	// Resolved reports whether the environment half has been answered.
	Resolved bool
	// Available reports whether the shell can be measured here at all.
	Available bool
	// Reason is why it cannot be measured here, in the screen's own words, and
	// RunReason why the last measurement's own start was refused -- a start this
	// shell does not offer, where the utility itself is perfectly able to measure.
	// They are separate because the first decides whether the row is offered and
	// the second is a result.
	Reason    string
	RunReason string
	// Shell is the login shell's name, ShellPath the binary it resolves to, and
	// Command the exact line the runs execute, so the number can be reproduced
	// by hand.
	Shell     string
	ShellPath string
	Command   string
	// Kind is the shell's family, which decides every flag the measurement uses,
	// and Mode the start the last measurement describes.
	Kind shellAuditKind
	Mode shellAuditMode
	// Version is what the shell said about itself, for the context line and for
	// the two instruments that need a version.
	Version shellAuditVersion
	// Terminal reports whether a terminal was attached to the run, and
	// TerminalName the terminal the run happened under ($TERM, or $TERM_PROGRAM
	// when the terminal names itself). A measurement with no terminal is still
	// made and says so: the alternative was no number at all.
	Terminal     bool
	TerminalName string
	// Runs is how many starts the last measurement made and Timeout the bound each
	// one got.
	Runs    int
	Timeout time.Duration
	// Plan is what the next measurement will do.
	Plan shellAuditPlan

	// Measuring reports that a measurement is in flight, so the screen says so
	// and a second press cannot start a second one.
	Measuring bool
	// Measured reports that the measurement has run at least once.
	Measured bool
	// Progress is the start in flight, so the screen can say which one it is
	// waiting on instead of only that it is waiting.
	Progress shellAuditProgress
	// Cancelled reports that the last measurement was stopped by the user rather
	// than finished. The starts that had finished by then are still reported, and
	// the summary says how many of the plan they are.
	Cancelled bool
	// MeasuredAt is when the last measurement finished and Took how long the whole
	// measurement took, which is longer than the sum of the starts: it includes
	// the profiled start and the version probe.
	MeasuredAt time.Time
	Took       time.Duration

	// Samples are the starts that finished, in the order they ran. Timeouts
	// counts the starts killed at Timeout and Failures the ones that could not
	// be started or did not exit, with the first reason kept in Failure. A
	// timeout is a result of its own: it is never folded into the samples as a
	// duration.
	Samples  []time.Duration
	Timeouts int
	Failures int
	Failure  string
	// Warmup are the starts made before the samples and thrown away, so a reader
	// can see what the measurement cost rather than only what it kept.
	Warmup []time.Duration
	// Stats is the whole summary of Samples, including the numbers the median
	// alone cannot show: the spread, the percentile, the outliers and the median
	// of what is left once they are out.
	Stats shellAuditStats
	// Median, Fastest and Slowest summarize Samples. They are the projection of
	// Stats, kept so every reader of the state reads the same three numbers.
	Median  time.Duration
	Fastest time.Duration
	Slowest time.Duration

	// Attribution is what the shell's own instrument named, heaviest first, and
	// AttributionReason is why it named nothing when it did not. Exactly one of
	// the two is set.
	Attribution       []shellAuditFunction
	AttributionReason string
	// Note is what the start still includes although it was asked to be clean,
	// where the shell offers no way to leave it out.
	Note string
	// Baseline is the same start measured with none of the user's configuration,
	// when the plan asked for the comparison. It is nil when none was measured,
	// and the screen then says so instead of drawing a delta of zero.
	Baseline *shellAuditRun
}

// shellAuditProbe carries every effect the measurement has, so each one can be
// replaced in a test: no guard may start the runner's own shell, and no guard may
// write near the runner's own configuration.
type shellAuditProbe struct {
	getenv     func(string) string
	lookPath   func(string) (string, error)
	isTerminal func() bool
	now        func() time.Time
	// run starts the shell and returns its output. The context carries the
	// deadline, so a run built on exec.CommandContext is killed when it expires.
	run func(ctx context.Context, name string, args, env []string) ([]byte, error)
	// version runs the shell's own version command, bounded by its own deadline.
	// It is separate from run because it is not a start of the shell's startup:
	// it does not read a configuration file, it must not be counted as a sample,
	// and a machine that cannot answer it loses nothing but the context line.
	// A nil hook leaves the version unknown.
	version func(ctx context.Context, path string) (string, error)
	// trace starts the shell with its own trace running and hands every line of it
	// to parse while the start is still running. It is a hook of its own because
	// the trace arrives on the start's stderr, which run deliberately drops: a
	// trace read after the start has finished is a trace read from a pipe nobody
	// drained, and the start would block on a full pipe instead of ending. A nil
	// hook means this build cannot read a trace, and the bash profiler says so.
	trace func(ctx context.Context, name string, args, env []string, parse func(string)) error
	// timeout bounds every single run and runs is how many the total uses.
	timeout time.Duration
	runs    int
	// mkdirTemp, writeFile, readFile and removeAll are the profiled start's file
	// effects: two wrapper files inside a directory of the utility's own, and the
	// table zprof writes there. The user's shell configuration is only ever read.
	mkdirTemp func() (string, error)
	writeFile func(path string, data []byte) error
	readFile  func(path string) ([]byte, error)
	removeAll func(path string) error
}

// defaultShellAuditProbe returns the real probe.
func defaultShellAuditProbe() shellAuditProbe {
	return shellAuditProbe{
		getenv:     os.Getenv,
		lookPath:   exec.LookPath,
		isTerminal: func() bool { return isCharDevice(os.Stdout) },
		now:        time.Now,
		run:        runShellAuditCommand,
		version:    runShellAuditVersionCommand,
		trace:      runShellAuditTrace,
		timeout:    shellAuditTimeout,
		runs:       shellAuditRuns,
		mkdirTemp:  func() (string, error) { return os.MkdirTemp("", "dotfiles-shell-audit-") },
		writeFile:  func(path string, data []byte) error { return os.WriteFile(path, data, 0o600) },
		readFile:   os.ReadFile,
		removeAll:  os.RemoveAll,
	}
}

// runShellAuditCommand starts the shell once. WaitDelay pairs with the context
// deadline: the deadline kills the shell, and the delay bounds the pipe wait that
// follows, so a background process holding the inherited stdout pipe cannot
// outlive the timeout.
func runShellAuditCommand(ctx context.Context, name string, args, env []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = shellAuditWaitDelay
	if len(env) > 0 {
		cmd.Env = shellAuditMergedEnv(os.Environ(), env)
	}
	return cmd.Output()
}

// runShellAuditVersionCommand asks the shell what it is. Its own bound keeps it
// from being the one call that stalls the screen: a shell whose version command
// waits on something -- a network mount, a broken wrapper -- is reported as
// unknown rather than waited on. The output is kept to the first line, and a
// failure is not an error the caller has to report: not knowing the version
// costs the context line, never the measurement.
func runShellAuditVersionCommand(ctx context.Context, path string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, shellAuditVersionTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, shellAuditVersionFlag)
	cmd.WaitDelay = shellAuditWaitDelay
	out, err := cmd.Output()
	if err != nil {
		return string(out), err
	}
	return string(out), nil
}

// shellAuditMergedEnv adds the utility's own variables to the process
// environment, replacing any the environment already carries. It has to replace
// rather than append: ZDOTDIR is a variable the user may already have set -- and
// the utility reads it to find their startup files -- so the child would
// otherwise be handed two ZDOTDIR entries and read whichever one it happened to
// see last.
func shellAuditMergedEnv(base, extra []string) []string {
	overridden := make(map[string]bool, len(extra))
	for _, entry := range extra {
		if key, _, ok := strings.Cut(entry, "="); ok {
			overridden[key] = true
		}
	}

	merged := make([]string, 0, len(base)+len(extra))
	for _, entry := range base {
		if key, _, ok := strings.Cut(entry, "="); ok && overridden[key] {
			continue
		}
		merged = append(merged, entry)
	}
	return append(merged, extra...)
}

// resolveShellAudit answers everything about the measurement that does not need
// the measurement: which shell, whether it exists, and whether this run can start
// an interactive one at all.
//
// Every refusal is a reason in the screen's own words. A utility that is missing
// without saying why is the failure this shape exists to prevent, and the three
// refusals are different situations: no login shell named, a login shell this
// machine does not have, and a run with no terminal to reproduce the start on.
func resolveShellAudit(probe shellAuditProbe) shellAuditState {
	plan := shellAuditPlanDefault(probe.runs)
	st := shellAuditState{Resolved: true, Runs: plan.Samples, Timeout: probe.timeout, Plan: plan}

	shell := probe.getenv("SHELL")
	if shell == "" {
		st.Reason = "the login shell is not named: $SHELL is empty, so there is no shell to open and nothing to measure. " +
			"Set $SHELL to the shell you use and start the installer again."
		return st
	}
	st.Shell = filepath.Base(shell)
	st.Kind = shellAuditKindOf(st.Shell)

	path, err := probe.lookPath(shell)
	if err != nil {
		st.Reason = fmt.Sprintf("%s is the login shell named by $SHELL, but this machine has no such executable (%v), "+
			"so there is nothing to start: no total and no attribution.", shell, err)
		return st
	}
	st.ShellPath = path
	st.Command = shellAuditCommandLine(st.Shell, shellAuditStartForKind(st.Kind, plan.Mode))

	// The terminal is context, not a gate. A run with no terminal attached starts
	// the shell exactly the same way -- `-i -c exit` on our own stdin, which needs
	// no tty -- and the start is different from the one a terminal gets, because
	// job control and the plugins that read the terminal behave differently. The
	// number is still worth having and the screen says what it describes; refusing
	// outright was the case where a pipe, a container or a CI run got no number at
	// all and no explanation of what it was missing.
	st.Terminal = probe.isTerminal()
	st.TerminalName = shellAuditTerminalName(probe.getenv)

	st.Available = true
	return st
}

// shellAuditStartForKind is the start of one mode with no clean directory: it
// exists for the one caller that only needs the command line to draw, and it is
// careful to be the same answer the run gets. A mode whose start needs a clean
// directory returns the arguments it would use there, so the line on screen
// still names the flags.
func shellAuditStartForKind(kind shellAuditKind, mode shellAuditMode) shellAuditStart {
	if mode.Clean {
		start, _ := shellAuditStartFor(kind, mode, shellAuditCleanDirPlaceholder)
		return start
	}
	start, _ := shellAuditStartFor(kind, mode, "")
	return start
}

// shellAuditCleanDirPlaceholder is what the drawn command line shows where the
// run puts the directory of its own. The line is meant to be readable and
// reproducible by hand: a reader who pastes it makes the same start, and the
// path they choose is theirs.
const shellAuditCleanDirPlaceholder = "<empty directory>"

// shellAuditTerminalName is the terminal the run happened under, as the
// environment names it: $TERM_PROGRAM is the emulator's own name where it sets
// one, and $TERM is what everything has. It is context for the number, never a
// capability test.
func shellAuditTerminalName(getenv func(string) string) string {
	if program := getenv("TERM_PROGRAM"); program != "" {
		if version := getenv("TERM_PROGRAM_VERSION"); version != "" {
			return program + " " + version
		}
		return program
	}
	return getenv("TERM")
}

// measureShellAudit runs one measurement to its end on this goroutine and
// returns the state it leaves. The screen does not use this shape -- it runs the
// measurement on a session and draws each start as it lands -- but a guard wants
// the finished state without a channel, and the two share the one engine so a
// guard can never pass against an engine the screen does not run.
func measureShellAudit(st shellAuditState, probe shellAuditProbe) shellAuditState {
	if !st.Available {
		return st
	}
	return drainShellAuditSession(st, startShellAuditSession(st, probe))
}

// shellAuditSummary reduces the finished runs to the median and the two ends of
// the range. It is the projection of the whole summary, kept because the three
// numbers are what a caller who does not want the spread asks for.
func shellAuditSummary(samples []time.Duration) (median, fastest, slowest time.Duration) {
	st := shellAuditStatsOf(samples)
	return st.Median, st.Fastest, st.Slowest
}

// shellAuditRunTimedOut reports whether a run was cut off rather than finished.
// The wait delay is its own answer: a start whose process exited but whose pipe
// was still held past the deadline did not finish within the bound either.
func shellAuditRunTimedOut(ctx context.Context, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, exec.ErrWaitDelay) {
		return true
	}
	if ctx.Err() != nil {
		return true
	}
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
}

// parseZprofProfile reads zprof's table. The boolean reports whether the table's
// header was seen at all, which is what separates "zprof did not report" from
// "zprof reported no function".
//
// Only the summary table is read. zprof follows it with a detail block per
// function, each one repeating that function's own row under the callers that
// reached it, so a parse that kept reading would list every function twice and
// the screen would blame the heaviest one twice. The summary table ends at the
// first line that is not one of its rows.
//
// The rows are kept in zprof's own order, which is by the time each function
// spent on itself, and the values are zprof's own rather than re-derived here.
func parseZprofProfile(data []byte) ([]shellAuditFunction, bool) {
	var functions []shellAuditFunction
	reported := false

	for _, line := range strings.Split(string(data), "\n") {
		if !reported {
			if strings.HasPrefix(strings.TrimSpace(line), zprofHeaderPrefix) {
				reported = true
			}
			continue
		}

		if isZprofRule(line) {
			if len(functions) == 0 {
				// The rule and blank lines under the header, before the first row.
				continue
			}
			// A rule after rows have been read closes the summary table.
			break
		}
		fn, ok := parseZprofLine(line)
		if !ok {
			break
		}
		functions = append(functions, fn)
	}

	return functions, reported
}

// shellAuditMilliseconds converts one of zprof's millisecond cells.
func shellAuditMilliseconds(ms float64) time.Duration {
	return time.Duration(ms * float64(time.Millisecond))
}

// zprofHeaderPrefix is how zprof's table announces itself. A profile that never
// reaches it did not run zprof, and saying so is more honest than blaming a
// function nobody profiled.
const zprofHeaderPrefix = "num  calls"

// shellAuditDuration renders a measured duration: whole milliseconds below a
// second, and seconds above it, at two decimals unless the second is whole -- a
// ten-second bound reads as "10 s" rather than "10.00 s".
func shellAuditDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.0f ms", float64(d)/float64(time.Millisecond))
	}
	if d%time.Second == 0 {
		return fmt.Sprintf("%.0f s", d.Seconds())
	}
	return fmt.Sprintf("%.2f s", d.Seconds())
}

// shellAuditZprofTime renders one of zprof's own cells at zprof's own precision,
// so a number on screen can be found in the table it came from.
func shellAuditZprofTime(ms float64) string {
	if ms >= 1000 {
		return fmt.Sprintf("%.2f s", ms/1000)
	}
	return fmt.Sprintf("%.2f ms", ms)
}

// shellAuditQuote wraps a path in single quotes for the generated wrapper,
// closing and reopening the quote around any single quote the path contains, so a
// path with a space, a quote or a dollar sign in it is one word rather than shell
// syntax.
func shellAuditQuote(path string) string {
	return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
}
