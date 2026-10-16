//go:build !windows

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

// defaultProbe is the real machine surface. Every method is a leaf over the
// standard library, and the two that can change the machine -- Signal and
// Alive -- are the only ones that touch a process at all.
type defaultProbe struct{}

func (defaultProbe) Now() time.Time { return time.Now() }

func (defaultProbe) GOOS() string { return runtime.GOOS }

// IsWSL reports whether this run is a guest inside Windows. The environment
// variable is the cheap answer and the kernel version the fallback, so a distro
// that does not export the variable is still recognized.
func (defaultProbe) IsWSL() bool {
	if os.Getenv("WSL_DISTRO_NAME") != "" {
		return true
	}
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return isWSLKernelText(string(data))
}

// isWSLKernelText reports whether a kernel version string identifies a WSL
// kernel. It is a separate function so the detection can be tested on any host.
func isWSLKernelText(version string) bool {
	lower := strings.ToLower(version)
	return strings.Contains(lower, "microsoft") || strings.Contains(lower, "wsl")
}

func (defaultProbe) SelfPID() int { return os.Getpid() }

// Parent reports a process's parent. Linux answers from /proc, where the third
// field after the command is the ppid; every other Unix asks ps, with the same
// locale-forced argv the readers use so the number is parsed the same way
// everywhere.
func (defaultProbe) Parent(pid int) (int, bool) {
	if pid <= 1 {
		return 0, false
	}
	if runtime.GOOS == "linux" {
		data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
		if err != nil {
			return 0, false
		}
		ppid, _, _, ok := parseProcStat(data)
		if !ok || ppid <= 0 {
			return 0, false
		}
		return ppid, true
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

// firstNonEmptyLine returns the first line of a tool's stderr that carries
// text, trimmed. It is what a failure reports instead of a blank message.
func firstNonEmptyLine(data []byte) string {
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}
