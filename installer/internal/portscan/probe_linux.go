//go:build linux

package portscan

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// linuxUserHZ is USER_HZ, the unit /proc counts process start times in. The
// kernel has fixed it at 100 for everything a user-space reader sees since
// 2.6, whatever the internal timer frequency is, so no sysconf call is
// needed and no cgo is pulled in for one number.
const linuxUserHZ = 100

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

// Parent reports a process's parent. /proc is the kernel's own answer and the
// field is fixed, so it is read directly rather than through a tool that would
// have to be parsed by locale.
func (defaultProbe) Parent(pid int) (int, bool) {
	if pid <= 1 {
		return 0, false
	}
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

// firstNonEmptyLine lives in parse.go: the readers and this probe both report
// a tool's first told word, and it names no platform path.

// readBootFacts reads /proc/stat and the kernel's boot id once.
func readBootFacts(p Probe) bootFacts {
	var facts bootFacts
	if data, err := p.ReadFile("/proc/stat"); err == nil {
		facts.at = parseBtime(data)
	}
	if data, err := p.ReadFile("/proc/sys/kernel/random/boot_id"); err == nil {
		facts.id = strings.TrimSpace(string(data))
	}
	// A kernel that hides the boot id still has a boot time; using its seconds
	// as the identity half keeps the token comparable across two readings.
	if facts.id == "" && !facts.at.IsZero() {
		facts.id = strconv.FormatInt(facts.at.Unix(), 10)
	}
	return facts
}

// parseBtime reads the boot time out of /proc/stat.
func parseBtime(data []byte) time.Time {
	for _, line := range strings.Split(string(data), "\n") {
		rest, found := strings.CutPrefix(line, "btime ")
		if !found {
			continue
		}
		seconds, err := strconv.ParseInt(strings.TrimSpace(rest), 10, 64)
		if err != nil {
			return time.Time{}
		}
		return time.Unix(seconds, 0)
	}
	return time.Time{}
}

// readProcProcess reads everything /proc will tell about one PID. Every field
// is best effort: another user's process still has a readable stat file, which
// is where the identity token comes from, while its command line may be
// withheld.
//
// It is the Linux implementation of a fact reader every platform declares with
// the same signature, so the untagged listener builder and stop path compile
// wherever the package is built. On macOS the same name reads ps instead.
func readProcProcess(p Probe, pid int, scope string, boot bootFacts) Process {
	proc := Process{PID: pid, Scope: scope}
	base := "/proc/" + strconv.Itoa(pid)

	if data, err := p.ReadFile(base + "/comm"); err == nil {
		proc.Name = strings.TrimSpace(string(data))
	}

	if data, err := p.ReadFile(base + "/cmdline"); err == nil {
		proc.Args = RedactArgs(splitNUL(data))
	}

	if data, err := p.ReadFile(base + "/status"); err == nil {
		if uid, ok := parseStatusUID(data); ok {
			proc.User = userLabel(p, uid)
		}
	}

	if exe, err := p.Readlink(base + "/exe"); err == nil {
		proc.Exe = strings.TrimSuffix(exe, " (deleted)")
	}

	if data, err := p.ReadFile(base + "/stat"); err == nil {
		if _, comm, startTicks, ok := parseProcStat(data); ok {
			if proc.Name == "" {
				proc.Name = comm
			}
			if boot.id != "" || startTicks > 0 {
				proc.token = boot.id + ":" + strconv.FormatUint(startTicks, 10)
			}
			if !boot.at.IsZero() {
				proc.StartedAt = boot.at.Add(time.Duration(startTicks) * (time.Second / linuxUserHZ))
			}
		}
	}

	// The executable's own name is the process's real name; comm is truncated
	// to fifteen characters by the kernel and would name a long binary wrongly.
	if proc.Exe != "" {
		if name := filepath.Base(proc.Exe); name != "" && name != "." {
			proc.Name = name
		}
	}
	if proc.Name == "" && len(proc.Args) > 0 {
		proc.Name = filepath.Base(proc.Args[0])
	}
	return proc
}

// parseProcStat reads the parent pid, the command name and the start ticks out
// of /proc/<pid>/stat.
//
// The command name is wrapped in parentheses and may itself contain spaces and
// parentheses, so the fields are split at the LAST closing parenthesis and the
// fixed-position fields after it are counted from there: rest[0] is field 3,
// which makes the parent field 4 at rest[1] and the start time field 22 at
// rest[19].
func parseProcStat(data []byte) (ppid int, comm string, startTicks uint64, ok bool) {
	text := string(data)
	open := strings.Index(text, "(")
	closing := strings.LastIndex(text, ")")
	if open < 0 || closing <= open {
		return 0, "", 0, false
	}
	comm = text[open+1 : closing]
	rest := strings.Fields(text[closing+1:])
	if len(rest) < 20 {
		return 0, comm, 0, false
	}

	parent, err := strconv.Atoi(rest[1])
	if err != nil {
		return 0, comm, 0, false
	}
	ticks, err := strconv.ParseUint(rest[19], 10, 64)
	if err != nil {
		return 0, comm, 0, false
	}
	return parent, comm, ticks, true
}

// parseStatusUID reads the real uid from /proc/<pid>/status. The line's four
// numbers are real, effective, saved and filesystem; the first is the account
// the process belongs to.
func parseStatusUID(data []byte) (int, bool) {
	for _, line := range strings.Split(string(data), "\n") {
		rest, found := strings.CutPrefix(line, "Uid:")
		if !found {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			return 0, false
		}
		uid, err := strconv.Atoi(fields[0])
		if err != nil {
			return 0, false
		}
		return uid, true
	}
	return 0, false
}

// splitNUL splits a NUL-separated list, which is how the kernel writes
// /proc/<pid>/cmdline. Empty parts are dropped: they are the separator's own
// tail, not empty arguments worth showing.
func splitNUL(data []byte) []string {
	var out []string
	for _, part := range strings.Split(string(data), "\x00") {
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}
