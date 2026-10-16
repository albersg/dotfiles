package updates

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Command is one program to run: a program and its arguments, and nothing else.
//
// There is no command string and no shell on this path, on purpose. Every name
// this package passes comes from a package manager's own output, and a shell
// would make a package name into a potential second command; an argv cannot. The
// one program that is ever composed rather than read is sudo, and it is composed
// here from the constant below.
type Command struct {
	// Argv is the program and its arguments. Argv[0] is the program.
	Argv []string
	// Elevate runs the command through the machine's administrator program,
	// non-interactively. A machine that would have to ask for a password refuses
	// instead, which is what turns into StatusPermission: this program never
	// shows a password prompt of its own, so it must never start one it cannot
	// finish.
	Elevate bool
	// Env are extra environment entries, in KEY=VALUE form, that replace any the
	// process already carries with the same key.
	Env []string
	// Dir is the working directory, empty for the process's own.
	Dir string
	// Timeout bounds the command. The zero value takes defaultCommandTimeout.
	Timeout time.Duration
}

// Output is what one command produced.
type Output struct {
	// Stdout and Stderr are the command's streams, each bounded by
	// maxKeptOutputBytes from the end: the tail is where a manager says what
	// happened.
	Stdout string
	Stderr string
	// ExitCode is the process's exit status, or -1 when it never started.
	ExitCode int
	// Err is why the command did not exit zero, nil when it did.
	Err error
	// Started reports whether the process was started at all.
	Started bool
	// Cancelled reports that the run was interrupted at the user's request,
	// either because the context was cancelled or because the user asked for it.
	Cancelled bool
	// TimedOut reports that the command was stopped at its own timeout.
	TimedOut bool
	// Duration is how long the command ran.
	Duration time.Duration
}

// Text is the command's output as one block, stderr after stdout, so a caller
// that only reads text does not have to know which stream a manager writes to. A
// package manager's refusals arrive on either one depending on the manager.
func (o Output) Text() string {
	var parts []string
	if strings.TrimSpace(o.Stdout) != "" {
		parts = append(parts, o.Stdout)
	}
	if strings.TrimSpace(o.Stderr) != "" {
		parts = append(parts, o.Stderr)
	}
	return strings.Join(parts, "\n")
}

// Status classifies this output as the state a result or a plan ended in. A
// successful command is StatusCurrent -- it exited zero, and "did something move"
// is a question only the manager's fresh answer can settle.
func (o Output) Status() (Status, string) {
	return Classify(o)
}

const (
	// defaultCommandTimeout bounds a command whose caller named none. It is long
	// enough for a package manager to resolve and install, and short enough that
	// a manager waiting on an unreachable source cannot hold the interface
	// forever.
	defaultCommandTimeout = 2 * time.Minute

	// commandWaitDelay bounds the wait for a command's output pipes after it has
	// been asked to stop. Sending SIGINT first is what makes cancellation
	// survivable -- a package manager traps it and finishes the transaction it is
	// inside -- and this delay is what turns a manager that ignores it into a
	// bounded stop rather than a hang.
	commandWaitDelay = 5 * time.Second

	// maxKeptOutputBytes is how much of each stream is kept. A manager's progress
	// output can run to megabytes; what a reader needs is the end of it.
	maxKeptOutputBytes = 64 << 10

	// sudoBinary is the elevation program. It is a constant and not a found
	// program: nothing else in this package names a privilege program, and a
	// second spelling of it is how a prompt path reappears.
	sudoBinary = "sudo"

	// sudoNonInteractive is the flag that makes sudo refuse rather than ask. It is
	// the whole reason elevation can be offered from inside a full-screen
	// interface: a refusal is a status this program can show, while a prompt is a
	// hang it cannot.
	sudoNonInteractive = "-n"
)

// Runner runs one command and reports what it produced. It is the seam every
// manager adapter and every test goes through: no adapter calls exec itself, so
// every path this package has can be exercised without a package manager
// touching the runner's machine.
type Runner interface {
	// Run executes cmd, handing each complete output line to onLine as it
	// arrives. onLine may be nil. The context is what cancels the command; a
	// cancelled run reports Cancelled rather than an error a user would read as
	// a failure.
	Run(ctx context.Context, cmd Command, onLine func(string)) Output
}

// ExecRunner is the real Runner. It starts the program directly, streams both of
// its streams, and stops it gracefully before it is killed.
type ExecRunner struct {
	// Env is the process environment the extra entries are merged into. It is a
	// field so a test can pin the environment; nil means os.Environ().
	Env []string
	// Interactive is false for every run this package makes, and it exists to be
	// pinned: a runner that set it true would let a command read the terminal the
	// interface is drawing on.
	Interactive bool
}

// Run starts the command and waits for it. Nothing is written to the interface's
// terminal, and the process is never given a terminal to read: the interface owns
// the screen for the whole run.
func (r ExecRunner) Run(ctx context.Context, cmd Command, onLine func(string)) Output {
	return runCommand(ctx, cmd, onLine, r.Env)
}

// runCommand is ExecRunner.Run with the environment handed in.
func runCommand(ctx context.Context, cmd Command, onLine func(string), baseEnv []string) Output {
	started := time.Now()
	if len(cmd.Argv) == 0 {
		return Output{ExitCode: -1, Err: errors.New("no command to run"), Duration: time.Since(started)}
	}

	timeout := cmd.Timeout
	if timeout <= 0 {
		timeout = defaultCommandTimeout
	}

	argv := cmd.Argv
	if cmd.Elevate {
		// sudo -n -- program args... : -n refuses instead of prompting, and --
		// ends sudo's own options so an argument that begins with a dash is still
		// the program's.
		argv = append([]string{sudoBinary, sudoNonInteractive, "--"}, argv...)
	}

	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	process := exec.CommandContext(cctx, argv[0], argv[1:]...)
	process.Dir = cmd.Dir
	if len(baseEnv) == 0 {
		baseEnv = os.Environ()
	}
	process.Env = mergeCommandEnv(baseEnv, cmd.Env)

	// Interrupt first, kill only after the delay. A package manager that is
	// killed mid-transaction leaves a lock and a half-configured database
	// behind; one that is interrupted finishes what it is doing and says so.
	process.Cancel = func() error {
		if process.Process == nil {
			return nil
		}
		return process.Process.Signal(os.Interrupt)
	}
	process.WaitDelay = commandWaitDelay

	stdoutPipe, err := process.StdoutPipe()
	if err != nil {
		return Output{ExitCode: -1, Err: err, Duration: time.Since(started)}
	}
	stderrPipe, err := process.StderrPipe()
	if err != nil {
		return Output{ExitCode: -1, Err: err, Duration: time.Since(started)}
	}

	if err := process.Start(); err != nil {
		return Output{ExitCode: -1, Started: false, Err: err, Duration: time.Since(started)}
	}

	var (
		wg             sync.WaitGroup
		stdout, stderr tailBuffer
		redactionMu    sync.Mutex
	)
	consume := func(reader io.Reader, sink *tailBuffer) {
		defer wg.Done()
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			sink.WriteLine(line)
			if onLine == nil {
				continue
			}
			redactionMu.Lock()
			onLine(RedactLine(line))
			redactionMu.Unlock()
		}
	}
	wg.Add(2)
	go consume(stdoutPipe, &stdout)
	go consume(stderrPipe, &stderr)

	waitErr := process.Wait()
	wg.Wait()

	output := Output{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Started:  true,
		Duration: time.Since(started),
	}
	if process.ProcessState != nil {
		output.ExitCode = process.ProcessState.ExitCode()
	} else {
		output.ExitCode = -1
	}
	if waitErr == nil {
		output.ExitCode = 0
		output.Err = nil
		return output
	}
	output.Err = waitErr
	if errors.Is(cctx.Err(), context.DeadlineExceeded) {
		output.TimedOut = true
	}
	if ctx.Err() != nil {
		output.Cancelled = true
	}
	return output
}

// mergeCommandEnv adds extra KEY=VALUE entries to a base environment, replacing
// any entry the base already carries under the same key. It has to replace rather
// than append: an adapter that sets LC_ALL=C for a parseable answer must not hand
// its child two LC_ALL entries and leave the child to read whichever one it
// happened to see last.
func mergeCommandEnv(base, extra []string) []string {
	if len(extra) == 0 {
		return base
	}
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

// tailBuffer keeps the end of a stream, up to maxKeptOutputBytes. The beginning
// of a package manager's output is its banner and the beginning of a download
// bar; the end is what happened. It writes whole lines, so a reader never sees
// half a line that was never printed.
type tailBuffer struct {
	lines []string
	bytes int
}

// WriteLine appends one line and drops the oldest lines until the buffer fits.
func (b *tailBuffer) WriteLine(line string) {
	b.lines = append(b.lines, line)
	b.bytes += len(line) + 1
	for b.bytes > maxKeptOutputBytes && len(b.lines) > 1 {
		b.bytes -= len(b.lines[0]) + 1
		b.lines = b.lines[1:]
	}
}

// String returns the kept lines, oldest first. It is the last value of the
// stream rather than the first, and for a stream whose single line is longer than
// the bound the line is truncated from the front.
func (b *tailBuffer) String() string {
	if len(b.lines) == 0 {
		return ""
	}
	joined := strings.Join(b.lines, "\n")
	if len(joined) > maxKeptOutputBytes {
		joined = joined[len(joined)-maxKeptOutputBytes:]
	}
	return joined
}
