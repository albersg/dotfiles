package tui

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ============================================================================
// WHAT INSIDE THE STARTUP COSTS THE TIME
// ============================================================================
//
// The total answers "is this slow". The attribution answers "what is slow", and
// only the shell itself can answer it. Each family is asked in its own words:
// zsh loads zprof from a wrapper directory and reads the table it writes, bash
// turns its own trace on and is read line by line as it runs, and fish writes
// the profile of its startup to a file of the utility's. nushell and the POSIX
// shells are asked nothing, because they carry nothing to ask: their runs get a
// sentence saying only the total stands, which is what the utility said for
// every shell but zsh before this front.
//
// Every attribution is read-only. The wrapper files, the tables and the profile
// files all live in one directory of the utility's own that is removed before
// the screen draws, and the user's startup files are only ever sourced.
//
// The one rule that holds for all three: **nothing raw is kept**. The zsh and
// fish tables carry the user's own command text, and a bash trace carries its
// commands *expanded*, which is where a secret would end up on screen. So bash
// is parsed from its own stderr as it runs and never written down, and the rows
// of the other two are reduced to a name and a number before they leave the
// parser: a command is named by its first word, and a sourced file by its path.
// The secret of `set -gx API_KEY hunter2` is never one of them.

const (
	// shellAuditAttributionRows bounds the rows one instrument may return. The
	// screen names the heaviest five; the rest is kept for a reader who asks for
	// more, and a startup with a thousand traced commands must not be able to grow
	// the state without bound.
	shellAuditAttributionRows = 40

	// shellAuditTraceLineLimit bounds one line of a shell's trace. A trace line is
	// a command, and a command longer than this is not a command a screen can use:
	// the line is dropped rather than kept and drawn.
	shellAuditTraceLineLimit = 8 * 1024

	// shellAuditProfileFileLimit bounds how much of an instrument's own output is
	// read at all. zprof and fish both write a table proportional to the startup,
	// and a startup with a runaway loop could write without end.
	shellAuditProfileFileLimit = 4 * 1024 * 1024
)

// profileShellAudit runs the instrument this shell carries for one start and
// returns what it named, or the reason it named nothing.
//
// The instrument is loaded as part of the profiled start, so that start is an
// extra start of the shell and never one of the samples: zprof, the bash trace
// and fish's profiler all cost time of their own, and a median that included
// them would describe a shell nobody opens.
func profileShellAudit(run shellAuditRun, probe shellAuditProbe, ctx context.Context,
	profiler shellAuditProfiler, profilerReason string) ([]shellAuditFunction, string) {

	switch profiler {
	case shellAuditProfilerZprof:
		return profileShellAuditZprof(run, probe, ctx)
	case shellAuditProfilerXtrace:
		return profileShellAuditXtrace(run, probe, ctx)
	case shellAuditProfilerFishStartup:
		return profileShellAuditFish(run, probe, ctx)
	default:
		if profilerReason == "" {
			profilerReason = "this shell carries no instrument the utility can stand behind, so nothing inside the " +
				"startup can be named: only the total above is measurable."
		}
		return nil, profilerReason
	}
}

// profileShellAuditZprof profiles one zsh start with zsh's own zprof module.
//
// It takes one profiled start of zsh and returns the functions zprof named, or
// the reason it named none.
//
// It is read-only: the user's startup files are sourced from the directory
// ZDOTDIR already pointed at, and the only files written are the wrappers and
// zprof's table, all inside a directory of the utility's own that is removed
// afterwards. A function is named only when zprof named it -- no shell's startup
// is diagnosed by guessing.
func profileShellAuditZprof(run shellAuditRun, probe shellAuditProbe, ctx context.Context) ([]shellAuditFunction, string) {
	shell := run.Shell

	// ZDOTDIR is where the user's own startup files already live; HOME is where zsh
	// looks when it is unset. The wrapper sources them from there, so nothing of
	// the user's is copied, moved or rewritten. Without either, the wrapper could
	// only name a relative path, so the profiled start is not attempted at all.
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
	// The wrappers are the four files zsh reads, each one a transparent step from
	// the directory this run moved ZDOTDIR to and back to the user's own. Without
	// the profile and login steps, a login start would be measured with the user's
	// own .zprofile missing from it, which is a different startup.
	for _, wrapper := range []struct {
		name string
		body string
	}{
		{".zshenv", shellAuditZshEnvWrapper(realZDOTDIR, profilePath)},
		{".zprofile", shellAuditZshStepWrapper(realZDOTDIR, ".zprofile")},
		{".zshrc", shellAuditZshRCWrapper(realZDOTDIR, profilePath)},
	} {
		if err := probe.writeFile(filepath.Join(dir, wrapper.name), []byte(wrapper.body)); err != nil {
			return nil, "the profiled start could not be prepared (" + err.Error() + "), so no function can be named: only the total above stands."
		}
	}

	start, reason := shellAuditStartFor(run.Kind, run.Mode, dir)
	if reason != "" {
		return nil, reason
	}

	runCtx, cancel := context.WithTimeout(ctx, probe.timeout)
	defer cancel()
	// ZDOTDIR is the only variable the profiled start needs: it is where zsh looks
	// for its startup files, and the wrappers carry every other path they need in
	// their own text.
	if _, err := probe.run(runCtx, run.Path, start.Args, []string{"ZDOTDIR=" + dir}); err != nil {
		if shellAuditRunTimedOut(runCtx, err) {
			return nil, fmt.Sprintf("the profiled start of %s did not finish within %v, so no function can be named: a "+
				"start that does not end is reported as a timeout, and only the total above stands.", shell, probe.timeout)
		}
		return nil, fmt.Sprintf("the profiled start of %s failed (%v), so no function can be named: only the total "+
			"above stands.", shell, err)
	}

	if refusal := shellAuditProfileTooLarge("zsh/zprof", probe, profilePath); refusal != "" {
		return nil, refusal
	}

	data, err := probe.readFile(profilePath)
	if err != nil {
		return nil, "the profiled start wrote no table (" + err.Error() + "), so no function can be named: zsh/zprof " +
			"reported nothing here, and only the total above stands."
	}
	if len(data) > shellAuditProfileFileLimit {
		return nil, shellAuditProfileTooLargeReason("zsh/zprof")
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

// profileShellAuditXtrace profiles one bash start through bash's own trace.
//
// The trace is read from the start's stderr **while it runs** and only the
// elapsed time between two traced commands is kept, together with the file and
// line the earlier one came from. The command text itself is dropped as it
// arrives: a bash trace prints each command after expansion, so `export
// API_KEY=...` arrives with the secret in it, and a parser that kept the text
// would put it on the screen and in the history.
func profileShellAuditXtrace(run shellAuditRun, probe shellAuditProbe, ctx context.Context) ([]shellAuditFunction, string) {
	shell := run.Shell

	dir, err := probe.mkdirTemp()
	if err != nil {
		return nil, "the profiled start could not be prepared (" + err.Error() + "), so no function can be named: only the total above stands."
	}
	defer probe.removeAll(dir)

	wrapper := filepath.Join(dir, "trace.bashrc")
	if err := probe.writeFile(wrapper, []byte(shellAuditBashTraceWrapper(probe.getenv("HOME")))); err != nil {
		return nil, "the profiled start could not be prepared (" + err.Error() + "), so no function can be named: only the total above stands."
	}

	start, reason := shellAuditBashTraceStart(run.Mode, wrapper)
	if reason != "" {
		return nil, reason
	}

	if probe.trace == nil {
		return nil, "this build cannot read bash's own trace, so no file or function can be named: only the total above stands."
	}

	profile := newShellAuditTraceProfile()
	runCtx, cancel := context.WithTimeout(ctx, probe.timeout)
	defer cancel()

	if err := probe.trace(runCtx, run.Path, start.Args, start.Env, profile.line); err != nil {
		if shellAuditRunTimedOut(runCtx, err) {
			return nil, fmt.Sprintf("the profiled start of %s did not finish within %v, so nothing can be named: a "+
				"start that does not end is reported as a timeout, and only the total above stands.", shell, probe.timeout)
		}
		return nil, fmt.Sprintf("the profiled start of %s failed (%v), so nothing can be named: only the total above "+
			"stands.", shell, err)
	}

	rows := profile.ranked()
	if len(rows) == 0 {
		return nil, "bash's own trace named no file or function, so nothing can be named: the trace carried no " +
			"timestamps this bash can be read by (EPOCHREALTIME arrived in bash 5.0), and only the total above stands."
	}
	return rows, ""
}

// shellAuditBashTraceStart is the profiled bash start of one mode. It is its own
// function because it is genuinely a different start from the measured one: the
// rc file is replaced by the wrapper, and the wrapper is where the trace is
// turned on.
//
// A login bash is refused: it reads its profile files instead of any rc file the
// utility could point at, and moving HOME to a wrapper would measure a different
// startup than the one the total came from.
func shellAuditBashTraceStart(mode shellAuditMode, wrapper string) (shellAuditStart, string) {
	if mode.Login {
		return shellAuditStart{}, "a bash login shell reads its profile files instead of an rc file the utility can " +
			"point its trace at, and pointing HOME elsewhere would describe a start nobody gets, so no file or " +
			"function can be named for this start: only the total above stands."
	}
	if mode.Clean {
		return shellAuditStart{}, "a clean bash start has no configuration to trace, so there is nothing inside it to " +
			"name: only the total above stands."
	}
	if mode.Interactive {
		return shellAuditStart{
			Args: []string{"--rcfile", wrapper, "-i", "-c", "exit"},
		}, ""
	}
	// A non-interactive bash reads $BASH_ENV instead of an rc file, and it is the
	// only hook it offers.
	return shellAuditStart{
		Args: []string{"-c", "exit"},
		Env:  []string{"BASH_ENV=" + wrapper},
	}, ""
}

// profileShellAuditFish profiles one fish start through fish's own
// `--profile-startup`.
//
// It is `--profile-startup` and not `--profile`: fish's older flag begins after
// the configuration has been read, so a table from it would answer a different
// question than the total above it. The flag arrived in fish 3.6, and the
// capability matrix refuses it below that rather than showing a table of the
// wrong thing.
func profileShellAuditFish(run shellAuditRun, probe shellAuditProbe, ctx context.Context) ([]shellAuditFunction, string) {
	shell := run.Shell

	dir, err := probe.mkdirTemp()
	if err != nil {
		return nil, "the profiled start could not be prepared (" + err.Error() + "), so no script or command can be " +
			"named: only the total above stands."
	}
	defer probe.removeAll(dir)

	profilePath := filepath.Join(dir, "fish-profile.txt")
	cleanDir := ""
	if run.Mode.Clean {
		clean, err := probe.mkdirTemp()
		if err != nil {
			return nil, "the profiled start could not be prepared (" + err.Error() + "), so no script or command can " +
				"be named: only the total above stands."
		}
		cleanDir = clean
		defer probe.removeAll(cleanDir)
	}

	start, reason := shellAuditStartFor(run.Kind, run.Mode, cleanDir)
	if reason != "" {
		return nil, reason
	}
	// The flag goes first so the line on screen reads as the command a reader can
	// run by hand.
	start.Args = append([]string{"--profile-startup=" + profilePath}, start.Args...)

	runCtx, cancel := context.WithTimeout(ctx, probe.timeout)
	defer cancel()
	if _, err := probe.run(runCtx, run.Path, start.Args, start.Env); err != nil {
		if shellAuditRunTimedOut(runCtx, err) {
			return nil, fmt.Sprintf("the profiled start of %s did not finish within %v, so nothing can be named: a "+
				"start that does not end is reported as a timeout, and only the total above stands.", shell, probe.timeout)
		}
		return nil, fmt.Sprintf("the profiled start of %s failed (%v), so nothing can be named: only the total above "+
			"stands.", shell, err)
	}

	if refusal := shellAuditProfileTooLarge("fish", probe, profilePath); refusal != "" {
		return nil, refusal
	}

	data, err := probe.readFile(profilePath)
	if err != nil {
		return nil, "the profiled start wrote no table (" + err.Error() + "), so nothing can be named: fish reported " +
			"nothing here, and only the total above stands."
	}
	if len(data) > shellAuditProfileFileLimit {
		return nil, shellAuditProfileTooLargeReason("fish")
	}

	rows, reported := parseFishProfile(data)
	if !reported {
		return nil, "fish's own profile reported no table at all, so nothing can be named: the flag did not take " +
			"effect in this fish, or the startup did not reach its end. Only the total above stands, and nothing is " +
			"guessed at in its place."
	}
	if len(rows) == 0 {
		return nil, "fish's own profile reported no command, so nothing can be named: only the total above stands, " +
			"and nothing is guessed at in its place."
	}
	return rows, ""
}

// ============================================================================
// READING THE TWO TABLES
// ============================================================================

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
		Kind:  shellAuditAttributionFunction,
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

// fishProfileRow is one row of fish's own profile as written: the time the
// command spent on itself, the time it and everything it called spent, and the
// command text -- which is redacted before it is kept anywhere.
var fishProfileRowPattern = regexp.MustCompile(`^\s*(\d+)\s+(\d+)\s+(?:->|>)\s*(.*)$`)

// parseFishProfile reads fish's own startup profile. The boolean reports whether
// the table's header was seen at all, which is what separates "fish did not
// report" from "fish reported no command".
//
// Rows are aggregated by their redacted name: a startup runs hundreds of `set`
// commands, and a list of them by cost names nothing. The name is the command's
// first word -- plus the path when the command is a `source`, which is the one
// case where the second word is the useful part and never a secret.
func parseFishProfile(data []byte) ([]shellAuditFunction, bool) {
	reported := false
	rows := map[string]*shellAuditFunction{}

	for _, line := range strings.Split(string(data), "\n") {
		if !reported {
			if strings.HasPrefix(strings.TrimSpace(line), fishProfileHeaderPrefix) {
				reported = true
			}
			continue
		}

		match := fishProfileRowPattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		self, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil {
			continue
		}
		sum, err := strconv.ParseInt(match[2], 10, 64)
		if err != nil {
			continue
		}

		name, kind := shellAuditRedactCommand(match[3])
		if name == "" {
			continue
		}

		row, ok := rows[name]
		if !ok {
			row = &shellAuditFunction{Kind: kind, Name: name}
			rows[name] = row
		}
		row.Calls++
		row.Self += time.Duration(self) * time.Microsecond
		if total := time.Duration(sum) * time.Microsecond; total > row.Total {
			row.Total = total
		}
	}

	if !reported {
		return nil, false
	}
	// No share is computed for fish: the denominator would be a number the table
	// does not print -- every row's own sum already includes the rows under it, so
	// adding them up counts the same time again and again. The self time and the
	// total each row does print are shown instead, and a share nobody can
	// re-derive is left out rather than invented.
	return shellAuditRankedFunctions(shellAuditRowsOf(rows), 0), true
}

// shellAuditRowsOf turns a by-name aggregation into the list the ranking takes.
func shellAuditRowsOf(rows map[string]*shellAuditFunction) []shellAuditFunction {
	functions := make([]shellAuditFunction, 0, len(rows))
	for _, row := range rows {
		functions = append(functions, *row)
	}
	return functions
}

// fishProfileHeaderPrefix is how fish's profile announces itself.
const fishProfileHeaderPrefix = "Time (μs)"

// shellAuditRedactCommand reduces one command of a shell's own output to a name
// that can be drawn and stored.
//
// The rule is deliberately blunt: the first word only, and the second only when
// the first is a source, where the second is a path. `set -gx API_KEY hunter2`
// is named `set`; a command whose arguments carry a token, a password or a URL
// is named by its command word alone. The cost is right and the secret is not
// there.
// shellAuditProfileTooLarge refuses a table past the bound before it is read, when
// the probe can answer a file's size at all. The empty string means the table is
// small enough to read or that this probe cannot say, and the read path checks the
// bytes it got either way.
func shellAuditProfileTooLarge(instrument string, probe shellAuditProbe, path string) string {
	if probe.fileSize == nil {
		return ""
	}
	size, err := probe.fileSize(path)
	if err != nil {
		// A file that cannot be stat-ed is not a verdict: the read path reports what
		// happened to it, and a missing table has its own reason.
		return ""
	}
	if size > int64(shellAuditProfileFileLimit) {
		return shellAuditProfileTooLargeReason(instrument)
	}
	return ""
}

// shellAuditProfileTooLargeReason is what the screen says instead of a table it
// will not read, and it is one sentence for both orders: the table is refused
// because of its size, not because anything inside it was judged.
func shellAuditProfileTooLargeReason(instrument string) string {
	return fmt.Sprintf("the table %s wrote is larger than the %s this utility reads, so nothing is named from it: "+
		"only the total above stands. A table that size is not a startup's cost, and a table this utility refuses "+
		"is refused before it is parsed.", instrument, shellAuditSizeLabel(shellAuditProfileFileLimit))
}

// shellAuditSizeLabel writes a byte count the way the screen says it out loud.
func shellAuditSizeLabel(size int) string {
	if size >= 1024*1024 {
		return fmt.Sprintf("%d MB", size/(1024*1024))
	}
	return fmt.Sprintf("%d KB", size/1024)
}

func shellAuditRedactCommand(command string) (string, shellAuditAttributionKind) {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return "", ""
	}

	head := fields[0]
	if head == "builtin" && len(fields) > 1 {
		fields = fields[1:]
		head = fields[0]
	}

	switch head {
	case "source", ".":
		if len(fields) > 1 && strings.ContainsAny(fields[1], "/.") {
			return "source " + fields[1], shellAuditAttributionScript
		}
		return "source", shellAuditAttributionScript
	default:
		return head, shellAuditAttributionCommand
	}
}

// shellAuditRankedFunctions turns an aggregation map into the list a screen
// draws: heaviest self time first, ties broken by name so the order is stable
// across runs, and cut to the row limit.
func shellAuditRankedFunctions(functions []shellAuditFunction, total time.Duration) []shellAuditFunction {
	if total > 0 {
		for i := range functions {
			share := float64(functions[i].Self) / float64(total) * 100
			functions[i].Percent = fmt.Sprintf("%.2f%%", share)
		}
	}

	sort.Slice(functions, func(i, j int) bool {
		if functions[i].Self != functions[j].Self {
			return functions[i].Self > functions[j].Self
		}
		return functions[i].Name < functions[j].Name
	})

	if len(functions) > shellAuditAttributionRows {
		functions = functions[:shellAuditAttributionRows]
	}
	return functions
}

// ============================================================================
// BASH'S OWN TRACE, READ AS IT RUNS
// ============================================================================

// bashTraceRowPattern is one line of bash's trace as the wrapper makes it print:
// the elapsed-realtime stamp, the file and line the command came from, and the
// function it was in when it was inside one.
//
// The command text after the final space is matched and dropped: it is the part
// that carries expanded variables, and nothing in this utility keeps it.
var bashTraceRowPattern = regexp.MustCompile(`^\+*([0-9]+\.[0-9]+)\+(.*):([0-9]+)\+([^ ]*) `)

// shellAuditTraceProfile accumulates bash's trace as it arrives. The elapsed time
// of one traced command is the distance to the next trace line, which is why the
// previous line is held until the following one arrives: a line's own cost is
// only known when the next line is stamped.
type shellAuditTraceProfile struct {
	seen    int64
	haveOne bool
	lastAt  int64
	lastKey shellAuditTraceKey
	byKey   map[shellAuditTraceKey]*shellAuditFunction
	total   time.Duration
}

// shellAuditTraceKey is one attributed identity of bash's trace: a file the
// command came from, or a function it ran inside. A traced command inside a
// function is counted under both, which is stated on the screen: the rows are a
// ranking of where the time went, not a partition of it.
type shellAuditTraceKey struct {
	kind shellAuditAttributionKind
	name string
}

// newShellAuditTraceProfile returns the accumulator a trace is fed into.
func newShellAuditTraceProfile() *shellAuditTraceProfile {
	return &shellAuditTraceProfile{byKey: map[shellAuditTraceKey]*shellAuditFunction{}}
}

// line consumes one line of the trace. It is the function the probe calls for
// every line bash writes to its stderr, and it never keeps the line.
func (p *shellAuditTraceProfile) line(text string) {
	match := bashTraceRowPattern.FindStringSubmatch(text)
	if match == nil {
		// A line bash wrote that is not a trace row: a warning, a banner, or the
		// output of a command the startup ran. It is dropped here, which is also
		// what keeps a startup's own output out of the utility.
		return
	}
	if len(text) > shellAuditTraceLineLimit {
		return
	}

	stamp, ok := shellAuditTraceStamp(match[1])
	if !ok {
		return
	}
	key, ok := shellAuditTraceKeyOf(match[2], match[4])
	if !ok {
		return
	}

	if p.haveOne && stamp > p.lastAt {
		elapsed := time.Duration(stamp - p.lastAt)
		p.add(p.lastKey, elapsed)
		p.total += elapsed
	}
	p.lastAt, p.lastKey, p.haveOne = stamp, key, true
	p.seen++
}

// add adds one command's elapsed time to the identities that command belongs to.
func (p *shellAuditTraceProfile) add(key shellAuditTraceKey, elapsed time.Duration) {
	if elapsed <= 0 || key.name == "" {
		return
	}
	row, ok := p.byKey[key]
	if !ok {
		row = &shellAuditFunction{Kind: key.kind, Name: key.name}
		p.byKey[key] = row
	}
	row.Calls++
	row.Self += elapsed
	row.Total += elapsed
}

// shellAuditTraceStamp reads one of bash's own EPOCHREALTIME stamps into whole
// nanoseconds: the seconds are the integer part and the fraction is padded to
// nine digits, so the elapsed time between two commands is an exact subtraction
// rather than a float that drifts by a nanosecond per line.
func shellAuditTraceStamp(text string) (int64, bool) {
	whole, fraction, ok := strings.Cut(text, ".")
	if !ok {
		return 0, false
	}
	seconds, err := strconv.ParseInt(whole, 10, 64)
	if err != nil || seconds < 0 {
		return 0, false
	}
	if len(fraction) > 9 {
		fraction = fraction[:9]
	}
	for len(fraction) < 9 {
		fraction += "0"
	}
	nanos, err := strconv.ParseInt(fraction, 10, 64)
	if err != nil {
		return 0, false
	}
	return seconds*int64(time.Second) + nanos, true
}

// shellAuditTraceKeyOf turns the file and the function a traced command came
// from into the identity it is counted under. A line bash could not name -- an
// anonymous command, the trailing `exit` the wrapper runs -- is counted under
// nothing rather than under a guess.
func shellAuditTraceKeyOf(source, function string) (shellAuditTraceKey, bool) {
	if function != "" {
		return shellAuditTraceKey{kind: shellAuditAttributionFunction, name: function}, true
	}
	source = strings.TrimSpace(source)
	if source == "" || source == "?" {
		return shellAuditTraceKey{}, false
	}
	return shellAuditTraceKey{kind: shellAuditAttributionScript, name: source}, true
}

// ============================================================================
// THE WRAPPERS THE INSTRUMENTS ARE LOADED FROM
// ============================================================================

// shellAuditZshEnvWrapper renders the .zshenv the profiled start reads first: it
// loads zprof before anything else, so every function the real startup defines is
// counted, arranges for the table to be written when the shell exits, and then
// sources the user's own .zshenv from the directory ZDOTDIR pointed at before
// this run moved it.
//
// The dump is at exit rather than at the end of .zshrc because a zsh that is not
// interactive never reads .zshrc: with the table written on the way out, the
// non-interactive and the login starts are attributed too, and the same wrapper
// serves every mode.
//
// The paths are baked into the text rather than passed in the environment: the
// wrapper is the only thing that needs them, and a path in a file cannot be
// shadowed by a variable the user already set. Every path goes through
// shellAuditQuote, so a home directory with a space, a quote or a dollar sign in
// it stays one word instead of becoming shell syntax.
func shellAuditZshEnvWrapper(realZDOTDIR, profilePath string) string {
	userEnv := shellAuditQuote(filepath.Join(realZDOTDIR, ".zshenv"))
	return "# written by the dotfiles installer for one profiled start; the directory is\n" +
		"# removed afterwards. It is not the user's shell configuration and nothing is\n" +
		"# written back to it.\n" +
		"builtin zmodload zsh/zprof 2>/dev/null\n" +
		"function _dotfiles_shell_audit_dump() { builtin zprof > " + shellAuditQuote(profilePath) + " }\n" +
		"function zshexit() { _dotfiles_shell_audit_dump }\n" +
		"if [[ -r " + userEnv + " ]]; then\n" +
		"  builtin source " + userEnv + "\n" +
		"fi\n"
}

// shellAuditZshRCWrapper renders the .zshrc the profiled start reads: the user's
// own .zshrc is sourced from the directory ZDOTDIR pointed at before this run
// moved it, and the exit hook is re-stated after it. A user whose own .zshenv or
// .zshrc defines zshexit would otherwise replace the one that writes the table,
// and the screen would report that zprof named nobody.
func shellAuditZshRCWrapper(realZDOTDIR, profilePath string) string {
	userRC := shellAuditQuote(filepath.Join(realZDOTDIR, ".zshrc"))
	return "# written by the dotfiles installer for one profiled start; the directory is\n" +
		"# removed afterwards. The user's own .zshrc is sourced from where ZDOTDIR pointed\n" +
		"# before this run moved it, and nothing is written into the user's tree.\n" +
		"if [[ -r " + userRC + " ]]; then\n" +
		"  builtin source " + userRC + "\n" +
		"fi\n" +
		"function zshexit() { _dotfiles_shell_audit_dump }\n"
}

// shellAuditZshStepWrapper renders a startup file zsh reads later than .zshenv
// -- .zprofile for a login shell, and any other step a future mode needs -- as
// the one step from the wrapper's directory to the user's own file. Without it
// the wrapper would hide that file from a login start and the profile would
// describe a startup the user never gets.
func shellAuditZshStepWrapper(realZDOTDIR, name string) string {
	user := shellAuditQuote(filepath.Join(realZDOTDIR, name))
	return "# written by the dotfiles installer for one profiled start; the directory is\n" +
		"# removed afterwards. It is the one step from this directory to the user's own\n" +
		"# " + name + ".\n" +
		"if [[ -r " + user + " ]]; then\n" +
		"  builtin source " + user + "\n" +
		"fi\n"
}

// shellAuditBashTraceWrapper renders the rc file the profiled bash start reads
// instead of the user's own: it points every trace line at the file and line it
// came from, turns the trace on, and then sources the user's own rc file, so the
// startup that is traced is the user's own startup.
//
// The user's rc file is named by path in the wrapper's text and read from there:
// it is never copied, moved or rewritten. prog is the prefix the trace lines
// carry, and it is baked into a single-quoted string so no part of it is a
// command.
func shellAuditBashTraceWrapper(home string) string {
	var source string
	if home != "" {
		rc := shellAuditQuote(filepath.Join(home, ".bashrc"))
		source = "if [[ -r " + rc + " ]]; then\n  builtin source " + rc + "\nfi\n"
	}
	return "# written by the dotfiles installer for one profiled start; the directory is\n" +
		"# removed afterwards. It is not the user's shell configuration and nothing is\n" +
		"# written back to it. The user's own .bashrc is sourced from where it already is.\n" +
		"PS4=" + shellAuditQuote(shellAuditBashPromptPrefix) + "\n" +
		"builtin set -x\n" +
		source
}

// shellAuditBashPromptPrefix is what bash prints before every traced command:
// the elapsed real time, the file and line the command came from, and the
// function it ran in when it was in one. It is the whole reason the trace can be
// attributed at all, and it is read by bashTraceRowPattern.
const shellAuditBashPromptPrefix = `+${EPOCHREALTIME}+${BASH_SOURCE:-?}:${LINENO}+${FUNCNAME[0]:-} `

// ============================================================================
// THE EFFECTS THE PROFILERS NEED
// ============================================================================

// runShellAuditTrace runs the profiled bash start with its trace read from the
// child's stderr while it runs, handing every line to parse and keeping none of
// them.
//
// stderr is read on this goroutine rather than a pump of its own, because the
// read has to finish before Wait: a pipe nobody reads is the deadlock this
// function exists to avoid. WaitDelay pairs with the deadline the same way it
// does on a measured start, so a background process holding the inherited pipe
// cannot outlive the bound.
func runShellAuditTrace(ctx context.Context, name string, args, env []string, parse func(string)) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = shellAuditWaitDelay
	if len(env) > 0 {
		cmd.Env = shellAuditMergedEnv(os.Environ(), env)
	}
	// The start's own stdout is dropped: a startup that prints a banner prints it
	// here, and nothing of it belongs on a screen.
	cmd.Stdout = io.Discard
	// stdin is left at os/exec's own default, which is the null device: a hook that
	// asks for input gets end-of-file and the start ends, which is the same
	// guarantee the measured starts have.

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	scanner := bufio.NewScanner(stderr)
	scanner.Buffer(make([]byte, 0, 64*1024), shellAuditTraceLineLimit)
	for scanner.Scan() {
		parse(scanner.Text())
	}
	return cmd.Wait()
}

// ranked is the finished list: every file and function the trace named, heaviest
// first, with the share each one had of the time the trace covered. It is what
// the screen draws, so the ranking is done once, here.
func (p *shellAuditTraceProfile) ranked() []shellAuditFunction {
	functions := make([]shellAuditFunction, 0, len(p.byKey))
	for _, row := range p.byKey {
		functions = append(functions, *row)
	}
	return shellAuditRankedFunctions(functions, p.total)
}
