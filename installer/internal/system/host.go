package system

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// HostResources are the capacities of the Windows host behind WSL.
//
// The zero value means "unknown", not "empty": nothing here distinguishes a
// host that really has no memory from one whose capacities could not be read,
// and every caller treats zero as "omit the machine-derived setting".
type HostResources struct {
	MemoryBytes uint64
	LogicalCPUs int
}

const (
	// windowsPowerShellBinary is the interop entry point. WSL injects the
	// Windows PATH into the distribution, so the bare name resolves while
	// interop is enabled.
	windowsPowerShellBinary = "powershell.exe"

	// hostQueryTimeout bounds the single host query. Interop can hang when the
	// Windows side is busy or unresponsive, and an installer must not hang with
	// it: a timed-out query is an unknown host.
	hostQueryTimeout = 5 * time.Second

	// hostQueryWaitDelay bounds the I/O wait that survives that deadline.
	//
	// Cancelling the context kills powershell.exe, but os/exec then waits for the
	// goroutines copying its output pipes: with WaitDelay at its zero value that
	// wait is unbounded, so a child of powershell.exe that inherited the stdout
	// pipe and outlived the kill would keep Output() blocked past
	// hostQueryTimeout and hang the install step. WaitDelay closes the pipes
	// after the delay, which turns that hang into a bounded failure -- the
	// already-degraded "unknown host" path. It must stay non-zero and below
	// hostQueryTimeout, or the step's total worst case is no longer bounded by
	// the timeout it advertises.
	hostQueryWaitDelay = 1 * time.Second

	// envWSLHostCPUs overrides the detected logical CPU count with an integer
	// number of logical CPUs. It is the escape hatch for a host whose interop
	// is disabled, and it exists for tests too.
	envWSLHostCPUs = "DOTFILES_WSL_HOST_CPUS"

	// envWSLHostMemoryMB overrides the detected host memory with an integer
	// number of MiB. It has the same purpose as envWSLHostCPUs, and both
	// overrides must be present and valid for the query to be skipped.
	envWSLHostMemoryMB = "DOTFILES_WSL_HOST_MEMORY_MB"

	// hostMemoryMBCeiling is the largest whole-MiB count whose byte count still
	// fits in a uint64 (2^64 / 2^20 = 2^44, so the last value is 2^44 - 1). One
	// MiB more is garbage output or a bad override, never a real host.
	hostMemoryMBCeiling = (int64(1) << 44) - 1

	// hostCPUCeiling keeps the logical CPU count inside the range every Go
	// target can represent, so the conversion to int is exact everywhere.
	hostCPUCeiling = int64(1) << 20

	// utf8BOM is the encoding marker a redirected Windows PowerShell console
	// can prepend to its output. It is not whitespace, so it survives trimming.
	utf8BOM = "\ufeff"
)

// windowsHostQuery is the whole Windows-side program: one process prints both
// capacities, so the interop cost is paid once.
//
// Win32_ComputerSystem.TotalPhysicalMemory is the host total that WSL2 hides
// from /proc/meminfo, and Win32_Processor.NumberOfLogicalProcessors is the real
// logical CPU count that nproc hides behind the configured VM limit. The memory
// is divided down to whole MiB on the Windows side, because every consumer is
// integer MiB arithmetic.
const windowsHostQuery = `$cpus = (Get-CimInstance Win32_Processor | Measure-Object -Property NumberOfLogicalProcessors -Sum).Sum; ` +
	`$mem = (Get-CimInstance Win32_ComputerSystem).TotalPhysicalMemory; ` +
	`"cpus=$cpus"; "memmb=" + [int64][math]::Floor($mem / 1MB)`

// DetectHostResources queries the Windows host through WSL interop.
// A zero HostResources means "unknown"; detection never fails the caller.
func DetectHostResources() HostResources {
	return detectHostResources(defaultHostResourceProbe())
}

// detectHostResources is DetectHostResources with every host effect injected:
// the environment lookup, the PATH lookup, the single query and its timeout.
//
// classifyLinux is parameterised the same way and for the same reason: a test
// must be able to exercise the failure paths -- a missing binary, a refused
// query, garbage output, a timeout -- without spawning a Windows binary and
// without reading the real environment.
func detectHostResources(probe hostResourceProbe) HostResources {
	if overrides, ok := hostResourcesFromEnv(probe.getenv); ok {
		return overrides
	}

	ctx, cancel := context.WithTimeout(context.Background(), probe.timeout)
	defer cancel()

	if _, err := probe.lookPath(windowsPowerShellBinary); err != nil {
		return HostResources{}
	}

	output, err := probe.query(ctx)
	if err != nil {
		return HostResources{}
	}

	resources, ok := parseHostResourceLines(string(output))
	if !ok {
		return HostResources{}
	}

	return resources
}

// hostResourceProbe carries the host effects the detector needs, so every one
// of them can be replaced in a test. defaultHostResourceProbe builds the real
// one.
type hostResourceProbe struct {
	// getenv reads the DOTFILES_WSL_HOST_* overrides.
	getenv func(string) string
	// lookPath resolves the Windows PowerShell binary. A distribution with
	// interop disabled has no such binary, which must degrade rather than fail.
	lookPath func(string) (string, error)
	// query runs the single Windows-side program and returns its output. The
	// context carries the deadline, so a query built on exec.CommandContext is
	// killed when that deadline expires.
	query func(context.Context) ([]byte, error)
	// timeout bounds query.
	timeout time.Duration
}

// defaultHostResourceProbe returns the real probe: the process environment, the
// real PATH and one powershell.exe query bounded by hostQueryTimeout.
func defaultHostResourceProbe() hostResourceProbe {
	return hostResourceProbe{
		getenv:   os.Getenv,
		lookPath: exec.LookPath,
		query:    queryWindowsHost,
		timeout:  hostQueryTimeout,
	}
}

// queryWindowsHost runs the single non-interactive PowerShell query. -NoProfile
// keeps a stranger's profile scripts out of the installer's path, and
// -NonInteractive makes a prompt fail the query instead of hanging it past the
// deadline. WaitDelay pairs with the context deadline: the deadline kills the
// process, and the delay bounds the pipe wait that follows, so a grandchild
// holding the inherited stdout pipe cannot outlive the timeout.
func queryWindowsHost(ctx context.Context) ([]byte, error) {
	cmd := exec.CommandContext(ctx, windowsPowerShellBinary,
		"-NoProfile", "-NonInteractive", "-Command", windowsHostQuery)
	cmd.WaitDelay = hostQueryWaitDelay
	return cmd.Output()
}

// hostResourcesFromEnv reads the DOTFILES_WSL_HOST_* overrides. Both must be
// present and positive: a half-set pair is not a capacity reading, and the query
// is a better source than a partial guess.
func hostResourcesFromEnv(getenv func(string) string) (HostResources, bool) {
	cpus, okCPUs := parseHostCount(getenv(envWSLHostCPUs), hostCPUCeiling)
	memoryMB, okMemory := parseHostCount(getenv(envWSLHostMemoryMB), hostMemoryMBCeiling)
	if !okCPUs || !okMemory {
		return HostResources{}, false
	}
	return hostResourcesFromCounts(cpus, memoryMB), true
}

// parseHostCount parses one base-10 host capacity. The query output and the
// environment overrides share this rule, so "garbage" means the same thing on
// both paths: an empty value, a non-number, a non-positive number and an
// implausibly large number are all refused.
func parseHostCount(value string, ceiling int64) (int64, bool) {
	count, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || count <= 0 || count > ceiling {
		return 0, false
	}
	return count, true
}

// hostResourcesFromCounts converts a validated logical CPU count and a validated
// whole-MiB memory count into host resources.
func hostResourcesFromCounts(cpus, memoryMB int64) HostResources {
	return HostResources{
		MemoryBytes: uint64(memoryMB) << 20,
		LogicalCPUs: int(cpus),
	}
}

// parseHostResourceLines reads the cpus= and memmb= lines from the query output.
//
// The parse is lenient about everything except the two values: unknown lines,
// blank lines, stray whitespace, spaces around the separator and a leading BOM
// are ignored, because a Windows PowerShell host can prepend console noise or an
// encoding marker. Both values must be present and positive before the result is
// trusted -- a partial reading is "unknown", never a half-filled struct.
func parseHostResourceLines(output string) (HostResources, bool) {
	var (
		cpus, memoryMB       int64
		haveCPUs, haveMemory bool
	)

	for _, line := range strings.Split(strings.TrimPrefix(output, utf8BOM), "\n") {
		key, value, found := strings.Cut(strings.TrimSpace(line), "=")
		if !found {
			continue
		}

		switch strings.TrimSpace(key) {
		case "cpus":
			if count, ok := parseHostCount(value, hostCPUCeiling); ok {
				cpus, haveCPUs = count, true
			}
		case "memmb":
			if count, ok := parseHostCount(value, hostMemoryMBCeiling); ok {
				memoryMB, haveMemory = count, true
			}
		}
	}

	if !haveCPUs || !haveMemory {
		return HostResources{}, false
	}

	return hostResourcesFromCounts(cpus, memoryMB), true
}
