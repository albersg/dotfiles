// Package sshcfg reads, explains and edits the user's OpenSSH client
// configuration through the OpenSSH tools themselves.
//
// The package is deliberately split in two halves:
//
//   - The structural half reads the configuration files the way a text editor
//     would: comments, order, Include directives, patterns, Match blocks and
//     unknown options are preserved, and nothing is normalised. Everything the
//     user wrote stays byte-for-byte where it was.
//   - The effective half asks the client what it would really do, because the
//     precedence between the user's file, an included file and the system-wide
//     file is the client's rule and not this package's. `ssh -G <alias>` is that
//     answer, and it is used whenever the client is available.
//
// Two rules hold everywhere in this package:
//
//  1. A private key file is never opened, parsed, copied or logged. Only the
//     public half (`<key>.pub`), the fingerprint of an existing key through
//     `ssh-keygen -l`, and the agent's own listing are read. Generating a key
//     hands the passphrase to `ssh-keygen` over a pipe and keeps it out of every
//     argument list, log line and error message.
//  2. Nothing outside the block this tool owns is ever rewritten. An entry that
//     cannot be edited safely is reported with the reason instead.
package sshcfg

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ============================================================================
// External commands
// ============================================================================

// Command is one external invocation.
//
// Stdin carries a secret (a passphrase) in exactly one case, and it is written
// to the child's pipe: it is never part of Args, and nothing in this package
// ever formats Stdin into an error, a log line or a report.
type Command struct {
	Name  string
	Args  []string
	Stdin string
	// Env adds to the child's environment. It is used to make the child's
	// behaviour deterministic (no display, no askpass program) rather than to
	// carry a secret.
	Env map[string]string
	// Detach runs the command in its own session, without a controlling
	// terminal. It is what makes a passphrase read from Stdin rather than from
	// the terminal the interface is drawing on.
	Detach bool
}

// Result is what an external command produced. Code is the process exit status,
// or -1 when the command never started.
type Result struct {
	Stdout string
	Stderr string
	Code   int
	Err    error
}

// OK reports whether the command ran and exited zero.
func (r Result) OK() bool { return r.Err == nil && r.Code == 0 }

// Output is the standard output with trailing newlines removed, which is what
// every caller of a listing command wants.
func (r Result) Output() string { return strings.TrimRight(r.Stdout, "\n") }

// Message is the first non-empty line of the standard error, which is what an
// OpenSSH tool says when it refuses. It is the only part of a failure that is
// worth showing, and it never carries a passphrase: the tools print
// "Load key ...: incorrect passphrase supplied" rather than the passphrase.
func (r Result) Message() string {
	for _, line := range strings.Split(r.Stderr, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	if r.Err != nil {
		return r.Err.Error()
	}
	return ""
}

// messageSummaryLimit is how much of a tool's own message a sentence built for
// the user may carry. The tool's words are worth quoting -- a refusal read in the
// words of the program that refused is worth more than this package's paraphrase
// -- but a host or a program that answers with one enormous line would otherwise
// set the width of the screen the sentence is drawn on.
const messageSummaryLimit = 200

// messageSummary reduces a tool's own message to one bounded line: the first
// line that says something, cut at the limit with an ellipsis when it is longer.
func messageSummary(res Result) string {
	line := firstLine(res.Message())
	runes := []rune(line)
	if len(runes) <= messageSummaryLimit {
		return line
	}
	return string(runes[:messageSummaryLimit]) + "…"
}

// Runner runs external commands. Tests implement it to exercise the key,
// agent, diagnosis and connection paths without ssh, ssh-keygen or ssh-add, and
// without a real key or a real agent.
type Runner interface {
	Run(ctx context.Context, cmd Command) Result
}

// ExecRunner runs commands through os/exec. Every command is bounded by its
// context and by WaitDelay, so a tool that leaves a background process holding
// the output pipe cannot hang the interface.
type ExecRunner struct {
	// Timeout bounds one command when the context has no deadline of its own.
	Timeout time.Duration
}

// defaultRunnerTimeout is the bound an ExecRunner applies when Timeout is zero.
const defaultRunnerTimeout = 30 * time.Second

// Run implements Runner.
func (e ExecRunner) Run(ctx context.Context, cmd Command) Result {
	if cmd.Name == "" {
		return Result{Code: -1, Err: errors.New("no command to run")}
	}
	timeout := e.Timeout
	if timeout <= 0 {
		timeout = defaultRunnerTimeout
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	c := exec.CommandContext(ctx, cmd.Name, cmd.Args...)
	if cmd.Stdin != "" {
		c.Stdin = strings.NewReader(cmd.Stdin)
	}
	if len(cmd.Env) > 0 {
		env := os.Environ()
		for key, value := range cmd.Env {
			env = append(env, key+"="+value)
		}
		c.Env = env
	}
	if cmd.Detach {
		detachSysProcAttr(c)
	}
	var stdout, stderr bytes.Buffer
	c.Stdout = &stdout
	c.Stderr = &stderr
	// A cancelled context kills the process; WaitDelay closes the pipes the
	// process may have handed to something else, which is what turns an
	// unbounded wait into the bound this runner promises.
	c.WaitDelay = 2 * time.Second

	err := c.Run()
	res := Result{Stdout: stdout.String(), Stderr: stderr.String(), Err: err}
	switch {
	case err == nil:
		res.Code = 0
	case ctx.Err() != nil:
		res.Code = -1
		res.Err = ctx.Err()
	default:
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			res.Code = exitErr.ExitCode()
			res.Err = nil
		} else {
			res.Code = -1
		}
	}
	return res
}

// ============================================================================
// Paths
// ============================================================================

// Paths is every file and directory the manager reads or writes. Keeping them
// in one value is what lets a test point the whole manager at a temporary home
// and never touch the developer's own configuration.
type Paths struct {
	// Home is the home directory the client itself would use.
	Home string
	// SSHDir is Home/.ssh.
	SSHDir string
	// UserConfig is the user's client configuration file, Home/.ssh/config.
	UserConfig string
	// UserConfigDir is the directory UserConfig lives in; an Include in the
	// user's file is resolved against the client's own rules, and this is the
	// base the client uses for a relative one.
	UserConfigDir string
	// SystemConfig is the system-wide client configuration
	// (/etc/ssh/ssh_config or the platform's equivalent). It is read for the
	// precedence report and never written.
	SystemConfig string
	// KnownHosts is Home/.ssh/known_hosts.
	KnownHosts string
	// AgentSocket is SSH_AUTH_SOCK, empty when no agent is advertised.
	AgentSocket string
	// WSL is true when the process is running inside WSL. The Windows-side
	// configuration (`/mnt/c/Users/.../.ssh`) belongs to a different system: it
	// is named in the diagnosis and never opened, listed or written.
	WSL bool
}

// PathsFrom resolves the paths for one home directory. It never reads a file:
// detecting what exists is the diagnosis' job.
func PathsFrom(home string, getenv func(string) string) Paths {
	if home == "" {
		home = "."
	}
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	sshDir := filepath.Join(home, ".ssh")
	return Paths{
		Home:          home,
		SSHDir:        sshDir,
		UserConfig:    filepath.Join(sshDir, "config"),
		UserConfigDir: sshDir,
		SystemConfig:  systemConfigPath(),
		KnownHosts:    filepath.Join(sshDir, "known_hosts"),
		AgentSocket:   getenv("SSH_AUTH_SOCK"),
		WSL:           isWSL(getenv),
	}
}

// DefaultPaths resolves the paths for the user running this process.
func DefaultPaths() Paths {
	home, _ := os.UserHomeDir()
	return PathsFrom(home, os.Getenv)
}

// ============================================================================
// The client and its tools
// ============================================================================

// Caps is what this host really offers. Nothing in the interface promises a
// capability that is not here: a screen reads Caps and either offers the action
// or says why it cannot.
type Caps struct {
	// SSH, Keygen and Add name the tools found on PATH.
	SSH    string
	Keygen string
	Add    string
	// SSHVersion is the first line of `ssh -V`, empty when ssh is absent or
	// refuses to report.
	SSHVersion string
	// KeygenSupports is the key types this client can generate, as reported by
	// `ssh -Q key`. An empty list means the question could not be answered,
	// never that nothing is supported; a screen says which of the two it is.
	KeygenSupports []string
	// AgentSocket is the socket SSH_AUTH_SOCK advertises when this process
	// started, empty when none is advertised. It is copied from the environment
	// rather than probed, because a process cannot see an agent it was not told
	// about.
	AgentSocket string
}

// HasSSH reports whether the client itself is available.
func (c Caps) HasSSH() bool { return c.SSH != "" }

// HasKeygen reports whether key generation and fingerprints are available.
func (c Caps) HasKeygen() bool { return c.Keygen != "" }

// HasAgentControl reports whether this host can add or remove identities when
// an agent is running.
func (c Caps) HasAgentControl() bool { return c.Add != "" && c.AgentSocket != "" }

// Manager is the entry point of the package: the paths, the client's tools and
// the runner every operation goes through.
type Manager struct {
	Paths Paths
	Run   Runner
	Caps  Caps
}

// Open resolves the paths and probes the tools. The probe is bounded and touches
// nothing: it asks each tool for its version and asks the client which key types
// it supports.
func Open(ctx context.Context, run Runner) *Manager {
	if run == nil {
		run = ExecRunner{}
	}
	paths := DefaultPaths()
	return &Manager{Paths: paths, Run: run, Caps: detectCaps(ctx, run, paths)}
}
