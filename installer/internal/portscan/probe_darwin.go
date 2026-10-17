//go:build darwin

package portscan

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// defaultProbe is the real machine surface on macOS. Every method is a leaf
// over the standard library, and the two that can change the machine -- Signal
// and Alive -- are the only ones that touch a process at all.
//
// It is a separate implementation from the Linux one because the fact readers
// below are: the process facts macOS can honestly give come from ps, not from
// /proc, and keeping a runtime branch for both in one file is what let a
// symbol that only exists behind a GOOS suffix break the other platform's
// build. The Probe seam and its method set stay identical, so nothing that
// consumes the probe has to know which file answered.
type defaultProbe struct{}

func (defaultProbe) Now() time.Time { return time.Now() }

func (defaultProbe) GOOS() string { return runtime.GOOS }

// IsWSL is false: a macOS process is not a guest inside Windows, and this
// platform has no /proc/version to consult.
func (defaultProbe) IsWSL() bool { return false }

func (defaultProbe) SelfPID() int { return os.Getpid() }

// Parent asks ps for the parent pid, with the same locale-forced argv the
// readers use so the number is parsed the same way everywhere.
func (defaultProbe) Parent(pid int) (int, bool) {
	if pid <= 1 {
		return 0, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := (defaultProbe{}).Run(ctx, "ps", []string{"-o", "ppid=", "-p", strconv.Itoa(pid)}, []string{"LC_ALL=C"})
	if err != nil {
		return 0, false
	}
	parent, err := strconv.Atoi(strings.TrimSpace(string(result.Stdout)))
	if err != nil || parent <= 0 {
		return 0, false
	}
	return parent, true
}

func (defaultProbe) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

func (defaultProbe) ReadDir(name string) ([]os.DirEntry, error) {
	return os.ReadDir(name)
}

func (defaultProbe) Readlink(name string) (string, error) {
	return os.Readlink(name)
}

func (defaultProbe) LookPath(name string) (string, error) {
	return exec.LookPath(name)
}

// Run executes one argv with no shell in between. The environment is the
// process's own plus whatever the caller adds, so a parser can force LC_ALL=C
// without losing PATH. Both streams are returned: a tool that warns on stderr
// while answering on stdout is reporting a limitation, and the reader has to be
// able to say so.
func (defaultProbe) Run(ctx context.Context, name string, args []string, env []string) (CommandResult, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), env...)

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := CommandResult{Stdout: []byte(stdout.String()), Stderr: []byte(stderr.String())}
	if err != nil {
		// The exit status is wrapped with the tool's own last words, so a
		// reader can report why the tool refused instead of "exit status 1".
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return result, fmt.Errorf("%s: %w: %s", name, err, firstNonEmptyLine(result.Stderr))
		}
		return result, fmt.Errorf("%s: %w", name, err)
	}
	return result, nil
}

// Alive reports whether a PID exists. Signal 0 asks the kernel the question
// without delivering anything: success means the process exists and is ours or
// ours to signal, EPERM means it exists and is not, and ESRCH means it is gone.
//
// PID 0 and negative PIDs are never asked about. To the kernel, kill(-1, 0)
// asks about every process the caller may signal, so a guard here is the
// difference between a question and a broadcast.
func (defaultProbe) Alive(pid int) (bool, error) {
	if pid <= 0 {
		return false, nil
	}
	err := syscall.Kill(pid, 0)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, syscall.EPERM):
		// It exists and it is not ours to signal. That is an answer about the
		// process, not a failure of the question.
		return true, nil
	case errors.Is(err, syscall.ESRCH):
		return false, nil
	default:
		return false, fmt.Errorf("ask the kernel about pid %d: %w", pid, err)
	}
}

// Signal sends one signal to one PID. PID 1 and below are never signalled: the
// guard belongs here as well as in the stop path, because this is the last
// place before the kernel.
func (defaultProbe) Signal(pid int, sig Signal) error {
	if pid <= 1 {
		return fmt.Errorf("%w: refusing to signal pid %d", ErrNoSuchProcess, pid)
	}
	native := syscall.SIGTERM
	if sig == SignalKill {
		native = syscall.SIGKILL
	}
	if err := syscall.Kill(pid, native); err != nil {
		switch {
		case errors.Is(err, syscall.ESRCH):
			return fmt.Errorf("%w: %v", ErrNoSuchProcess, err)
		case errors.Is(err, syscall.EPERM):
			return fmt.Errorf("%w: %v", ErrPermissionDenied, err)
		default:
			return fmt.Errorf("send %s to pid %d: %w", sig, pid, err)
		}
	}
	return nil
}

// LookupUser resolves a numeric uid to an account name. A host without a passwd
// entry for the uid answers with an error and the caller keeps the number.
func (defaultProbe) LookupUser(uid int) (string, error) {
	account, err := user.LookupId(strconv.Itoa(uid))
	if err != nil {
		return "", err
	}
	return account.Username, nil
}

// readBootFacts is the zero value here on purpose. macOS has no boot id and no
// /proc/stat to read one from; its identity token is the raw `ps -o lstart=`
// string, which one-second granularity makes comparable between two readings
// (D6). Pretending to a boot id would be a token no macOS reading could carry.
func readBootFacts(Probe) bootFacts { return bootFacts{} }

// parseProcStat is the /proc/<pid>/stat parser, and macOS has no /proc. The
// signature is kept alongside the other platform fact readers so every
// platform declares the same set; the only callers are the Linux fact reader
// and the Linux probe, so this answer is never reached here.
func parseProcStat([]byte) (ppid int, comm string, startTicks uint64, ok bool) {
	return 0, "", 0, false
}

// readProcProcess reads one process's identity the way macOS can: through ps.
// The untagged readDarwinProcess does the reading so the path is exercised on
// any host through the Probe, and this wrapper only supplies the deadline the
// reader commands use. The boot argument is accepted for signature parity with
// Linux and is unused, because macOS has no boot facts to add.
func readProcProcess(p Probe, pid int, scope string, _ bootFacts) Process {
	ctx, cancel := context.WithTimeout(context.Background(), darwinReaderTimeout)
	defer cancel()
	return readDarwinProcess(ctx, p, pid, scope)
}
