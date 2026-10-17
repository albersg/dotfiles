//go:build windows

package portscan

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"time"
)

// defaultProbe on Windows answers only the questions a Windows host can answer.
// The installer does not target Windows itself -- it runs inside WSL, where the
// Unix probe reads the guest -- so this file exists to keep the package
// compiling and to make Discover's unsupported-platform answer a real one
// rather than a build failure.
type defaultProbe struct{}

func (defaultProbe) Now() time.Time { return time.Now() }

func (defaultProbe) GOOS() string { return runtime.GOOS }

// IsWSL is false: this is a native Windows process, not a guest inside one.
func (defaultProbe) IsWSL() bool { return false }

func (defaultProbe) SelfPID() int { return os.Getpid() }

func (defaultProbe) Parent(pid int) (int, bool) { return 0, false }

func (defaultProbe) ReadFile(name string) ([]byte, error) { return os.ReadFile(name) }

func (defaultProbe) ReadDir(name string) ([]os.DirEntry, error) { return os.ReadDir(name) }

func (defaultProbe) Readlink(name string) (string, error) { return os.Readlink(name) }

func (defaultProbe) LookPath(name string) (string, error) { return exec.LookPath(name) }

func (defaultProbe) Run(ctx context.Context, name string, args []string, env []string) (CommandResult, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.Output()
	return CommandResult{Stdout: out}, err
}

// Alive reports false with the reason attached: a "no" that this file cannot
// stand behind is worse than an error the caller has to report.
func (defaultProbe) Alive(pid int) (bool, error) {
	return false, errors.New("this platform cannot ask whether a pid exists")
}

func (defaultProbe) Signal(pid int, sig Signal) error {
	return errors.New("this platform cannot signal a pid")
}

func (defaultProbe) LookupUser(uid int) (string, error) { return strconv.Itoa(uid), nil }

// The platform fact readers below exist so the untagged listener builder and
// the stop path compile on this platform too. They deliberately answer
// nothing: Windows has no /proc to read and this package has no Windows
// process API, so a reading here cannot claim an identity it never had. The
// empty token is what makes the stop path refuse with "identity could not be
// read again" instead of pretending a revalidation happened.

// readBootFacts answers the zero value: there is no boot id or boot time this
// probe can read, and inventing one would be an identity no second reading
// could match.
func readBootFacts(Probe) bootFacts { return bootFacts{} }

// parseProcStat is the /proc/<pid>/stat parser, and this platform has no
// /proc. The signature matches the other platforms' readers; the answer is a
// refusal rather than a guess.
func parseProcStat([]byte) (ppid int, comm string, startTicks uint64, ok bool) {
	return 0, "", 0, false
}

// readProcProcess cannot read a process here. It names the PID it was asked
// about and leaves every fact and the identity token empty, so the caller
// reports that the identity cannot be revalidated instead of showing made-up
// fields.
func readProcProcess(_ Probe, pid int, scope string, _ bootFacts) Process {
	return Process{PID: pid, Scope: scope}
}
