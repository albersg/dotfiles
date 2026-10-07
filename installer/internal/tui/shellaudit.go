package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
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

	// shellAuditNamedFunctions is how many rows of zprof's table the screen names.
	// The list is zprof's own order, heaviest first, and the count it was cut from
	// is printed beside it, so the cut is declared rather than silent.
	shellAuditNamedFunctions = 5
)

// shellAuditFunction is one function zprof named, with the numbers zprof
// printed for it. Total is the time the function and everything it called spent,
// Self is the time the function itself spent, and Percent is zprof's own share
// cell for Total. zprof sorts its table by Self, and the order is preserved here
// rather than re-sorted.
type shellAuditFunction struct {
	Name    string
	Calls   int
	Total   time.Duration
	Self    time.Duration
	Percent string
}

// shellAuditState is everything the utility draws: which shell is measured, the
// method it is measured with, the runs and their summary, and the functions
// zprof named -- or the reason it could not name any.
//
// The environment half (Shell, ShellPath, Command, Runs, Timeout, Available,
// Reason) is resolved once when the model is built, because a render path must
// not read the environment. The measurement half is filled only when the row
// inside the screen is pressed: starting the user's shell five times is not
// something a screen may do while drawing.
type shellAuditState struct {
	// Resolved reports whether the environment half has been answered.
	Resolved bool
	// Available reports whether the shell can be measured here at all.
	Available bool
	// Reason is why it cannot be measured, in the screen's own words.
	Reason string
	// Shell is the login shell's name, ShellPath the binary it resolves to, and
	// Command the exact line the runs execute, so the number can be reproduced
	// by hand.
	Shell     string
	ShellPath string
	Command   string
	// Runs is how many starts the measurement makes and Timeout the bound each
	// one gets.
	Runs    int
	Timeout time.Duration

	// Measuring reports that a measurement is in flight, so the screen says so
	// and a second press cannot start a second one.
	Measuring bool
	// Measured reports that the measurement has run at least once.
	Measured bool

	// Samples are the starts that finished, in the order they ran. Timeouts
	// counts the starts killed at Timeout and Failures the ones that could not
	// be started or did not exit, with the first reason kept in Failure. A
	// timeout is a result of its own: it is never folded into the samples as a
	// duration.
	Samples  []time.Duration
	Timeouts int
	Failures int
	Failure  string
	// Median, Fastest and Slowest summarize Samples.
	Median  time.Duration
	Fastest time.Duration
	Slowest time.Duration

	// Attribution is what zprof named, heaviest first, and AttributionReason is
	// why it named nothing when it did not. Exactly one of the two is set.
	Attribution       []shellAuditFunction
	AttributionReason string
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

// shellAuditArgs is the command every run executes: the shell started the way a
// terminal starts it, and asked to exit. It is one function so the line on screen
// and the line that runs cannot disagree.
func shellAuditArgs() []string {
	return []string{"-i", "-c", "exit"}
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
	st := shellAuditState{Resolved: true, Runs: probe.runs, Timeout: probe.timeout}

	shell := probe.getenv("SHELL")
	if shell == "" {
		st.Reason = "the login shell is not named: $SHELL is empty, so there is no shell to open and nothing to measure. " +
			"Set $SHELL to the shell you use and start the installer again."
		return st
	}
	st.Shell = filepath.Base(shell)

	path, err := probe.lookPath(shell)
	if err != nil {
		st.Reason = fmt.Sprintf("%s is the login shell named by $SHELL, but this machine has no such executable (%v), "+
			"so there is nothing to start: no total and no attribution.", shell, err)
		return st
	}
	st.ShellPath = path
	st.Command = st.Shell + " -i -c exit"

	if !probe.isTerminal() {
		st.Reason = fmt.Sprintf("this run has no terminal attached, so %s cannot be started the way a terminal starts it. "+
			"Without one, job control and the plugins that read the terminal behave differently, and the number would "+
			"describe a start you never get. Run the installer from a terminal to measure it.", st.Shell)
		return st
	}

	st.Available = true
	return st
}

// measureShellAudit starts the shell probe.runs times for the total and then once
// more under zprof for the attribution. It returns early when the shell was never
// available, so an unavailable state is never measured.
//
// Every start is bounded by probe.timeout, and a start that does not finish is
// counted as a timeout rather than folded into the samples: a timeout has no
// duration, and reporting one as a number would be the one lie this utility
// cannot afford. The profiled start is deliberately not a sample -- zprof costs
// time of its own -- so the median describes the starts a terminal makes.
func measureShellAudit(st shellAuditState, probe shellAuditProbe) shellAuditState {
	if !st.Available {
		return st
	}

	st.Measured = true
	st.Measuring = false
	st.Samples = nil
	st.Timeouts, st.Failures, st.Failure = 0, 0, ""

	for i := 0; i < probe.runs; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), probe.timeout)
		start := probe.now()
		_, err := probe.run(ctx, st.ShellPath, shellAuditArgs(), nil)
		elapsed := probe.now().Sub(start)
		// Whether the deadline fired has to be read before the cancellation below,
		// or a finished run would be classified by the cancel this loop itself
		// caused.
		timedOut := shellAuditRunTimedOut(ctx, err)
		cancel()

		switch {
		case err == nil:
			st.Samples = append(st.Samples, elapsed)
		case timedOut:
			st.Timeouts++
		default:
			st.Failures++
			if st.Failure == "" {
				st.Failure = err.Error()
			}
		}
	}

	st.Median, st.Fastest, st.Slowest = shellAuditSummary(st.Samples)
	st.Attribution, st.AttributionReason = profileShellAudit(st, probe)
	return st
}

// shellAuditSummary reduces the finished runs to the median and the two ends of
// the range. The median of an odd count is the middle run; of an even count it is
// the mean of the two middle runs, which is what happens when some runs timed out
// and were not samples.
func shellAuditSummary(samples []time.Duration) (median, fastest, slowest time.Duration) {
	if len(samples) == 0 {
		return 0, 0, 0
	}

	sorted := append([]time.Duration(nil), samples...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	fastest, slowest = sorted[0], sorted[len(sorted)-1]
	if len(sorted)%2 == 1 {
		return sorted[len(sorted)/2], fastest, slowest
	}
	return (sorted[len(sorted)/2-1] + sorted[len(sorted)/2]) / 2, fastest, slowest
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

// profileShellAudit takes one profiled start of zsh and returns the functions
// zprof named, or the reason it named none.
//
// It is read-only: the user's startup files are sourced from the directory
// ZDOTDIR already pointed at, and the only files written are the two wrappers and
// zprof's table, all inside a directory of the utility's own that is removed
// afterwards. A function is named only when zprof named it -- no shell's startup
// is diagnosed by guessing.
func profileShellAudit(st shellAuditState, probe shellAuditProbe) ([]shellAuditFunction, string) {
	if st.Shell != "zsh" {
		return nil, fmt.Sprintf("zprof is zsh's own module, and %s has no equivalent built in, so only the total is "+
			"measurable here. Nothing is named in its place: the only number this utility can stand behind for %s is "+
			"the median above.", st.Shell, st.Shell)
	}

	// ZDOTDIR is where the user's own startup files already live; HOME is where zsh
	// looks when it is unset. The wrapper sources them from there, so nothing of the
	// user's is copied, moved or rewritten. Without either, the wrapper could only
	// name a relative path, so the profiled start is not attempted at all.
	realZDOTDIR := probe.getenv("ZDOTDIR")
	if realZDOTDIR == "" {
		realZDOTDIR = probe.getenv("HOME")
	}
	if realZDOTDIR == "" {
		return nil, "neither $ZDOTDIR nor $HOME names the directory this shell's startup files live in, so the " +
			"profiled start cannot be pointed at them: no function can be named, and only the total above stands."
	}

	dir, err := probe.mkdirTemp()
	if err != nil {
		return nil, "the profiled start could not be prepared (" + err.Error() + "), so no function can be named: only the total above stands."
	}
	defer probe.removeAll(dir)

	profilePath := filepath.Join(dir, "zprof.txt")
	for _, wrapper := range []struct{ name, body string }{
		{".zshenv", shellAuditZshEnvWrapper(realZDOTDIR)},
		{".zshrc", shellAuditZshRCWrapper(realZDOTDIR, profilePath)},
	} {
		if err := probe.writeFile(filepath.Join(dir, wrapper.name), []byte(wrapper.body)); err != nil {
			return nil, "the profiled start could not be prepared (" + err.Error() + "), so no function can be named: only the total above stands."
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), probe.timeout)
	defer cancel()
	// ZDOTDIR is the only variable the profiled start needs: it is where zsh looks
	// for .zshenv and .zshrc, and the wrappers carry every other path they need in
	// their own text.
	if _, err := probe.run(ctx, st.ShellPath, shellAuditArgs(), []string{"ZDOTDIR=" + dir}); err != nil {
		if shellAuditRunTimedOut(ctx, err) {
			return nil, fmt.Sprintf("the profiled start of %s did not finish within %v, so no function can be named: a "+
				"start that does not end is reported as a timeout, and only the total above stands.", st.Shell, probe.timeout)
		}
		return nil, fmt.Sprintf("the profiled start of %s failed (%v), so no function can be named: only the total "+
			"above stands.", st.Shell, err)
	}

	data, err := probe.readFile(profilePath)
	if err != nil {
		return nil, "the profiled start wrote no table (" + err.Error() + "), so no function can be named: zsh/zprof " +
			"reported nothing here, and only the total above stands."
	}

	functions, reported := parseZprofProfile(data)
	if !reported {
		return nil, "zsh/zprof reported no table at all, so no function can be named: the module could not be loaded " +
			"in this zsh, or the startup did not reach the end of .zshrc. Only the total above stands, and nothing is " +
			"guessed at in its place."
	}
	if len(functions) == 0 {
		return nil, "zsh/zprof reported no function, so no function can be named: only the total above stands, and " +
			"nothing is guessed at in its place."
	}

	return functions, ""
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

// isZprofRule reports whether a line of zprof's output is one of its horizontal
// rules or a blank line, rather than a row.
func isZprofRule(line string) bool {
	trimmed := strings.TrimSpace(line)
	return trimmed == "" || strings.Trim(trimmed, "-") == ""
}

// parseZprofLine reads one row of zprof's table. The columns are the table's
// own: an index, the call count, the total time and its share, the self time and
// its share, and then the name -- which can contain spaces, as the rows for
// anonymous functions do, so the tail is joined rather than taken as one field.
func parseZprofLine(line string) (shellAuditFunction, bool) {
	fields := strings.Fields(line)
	if len(fields) < 9 {
		return shellAuditFunction{}, false
	}

	index := strings.TrimSuffix(fields[0], ")")
	if index == fields[0] {
		return shellAuditFunction{}, false
	}
	if _, err := strconv.Atoi(index); err != nil {
		return shellAuditFunction{}, false
	}

	calls, err := strconv.Atoi(fields[1])
	if err != nil {
		return shellAuditFunction{}, false
	}
	total, err := strconv.ParseFloat(fields[2], 64)
	if err != nil {
		return shellAuditFunction{}, false
	}
	self, err := strconv.ParseFloat(fields[5], 64)
	if err != nil {
		return shellAuditFunction{}, false
	}

	name := strings.Join(fields[8:], " ")
	if name == "" {
		return shellAuditFunction{}, false
	}

	return shellAuditFunction{
		Name:  name,
		Calls: calls,
		Total: shellAuditMilliseconds(total),
		Self:  shellAuditMilliseconds(self),
		// The share that belongs to Total. The table prints a second share under
		// self; the one shown beside the total is the one that adds up to the
		// startup, so it is the one kept.
		Percent: fields[4],
	}, true
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

// shellAuditZshEnvWrapper renders the .zshenv the profiled start reads first: it
// loads zprof before anything else, so every function the real startup defines is
// counted, and then sources the user's own .zshenv from the directory ZDOTDIR
// pointed at before this run moved it.
//
// The paths are baked into the text rather than passed in the environment: the
// wrapper is the only thing that needs them, and a path in a file cannot be
// shadowed by a variable the user already set. Every path goes through
// shellAuditQuote, so a home directory with a space, a quote or a dollar sign in
// it stays one word instead of becoming shell syntax.
func shellAuditZshEnvWrapper(realZDOTDIR string) string {
	userEnv := shellAuditQuote(filepath.Join(realZDOTDIR, ".zshenv"))
	return "# written by the dotfiles installer for one profiled start; the directory is\n" +
		"# removed afterwards. It is not the user's shell configuration and nothing is\n" +
		"# written back to it.\n" +
		"builtin zmodload zsh/zprof 2>/dev/null\n" +
		"if [[ -r " + userEnv + " ]]; then\n" +
		"  builtin source " + userEnv + "\n" +
		"fi\n"
}

// shellAuditZshRCWrapper renders the .zshrc the profiled start reads: the user's
// own .zshrc is sourced from the directory ZDOTDIR pointed at before this run
// moved it, and zprof writes its table outside the user's tree.
func shellAuditZshRCWrapper(realZDOTDIR, profilePath string) string {
	userRC := shellAuditQuote(filepath.Join(realZDOTDIR, ".zshrc"))
	return "# written by the dotfiles installer for one profiled start; the directory is\n" +
		"# removed afterwards. The user's own .zshrc is sourced from where ZDOTDIR pointed\n" +
		"# before this run moved it, and nothing is written into the user's tree.\n" +
		"if [[ -r " + userRC + " ]]; then\n" +
		"  builtin source " + userRC + "\n" +
		"fi\n" +
		"builtin zprof > " + shellAuditQuote(profilePath) + "\n"
}

// shellAuditQuote wraps a path in single quotes for the generated wrapper,
// closing and reopening the quote around any single quote the path contains, so a
// path with a space, a quote or a dollar sign in it is one word rather than shell
// syntax.
func shellAuditQuote(path string) string {
	return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
}
