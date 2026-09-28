package system

import (
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"
)

// hostQueryOutput is the documented query shape: one cpus= line and one memmb=
// line. The values are the ones the 15.6 GB / 12 logical CPU host used for this
// work actually answers, including the CRLF line endings the Windows console
// emits, so the expected bytes are memoryBytes = 16016 MiB = 16016 << 20.
const hostQueryOutput = "cpus=12\r\nmemmb=16016\r\n"

// fakeHostProbe is one probe fixture. Every effect the detector normally
// performs -- the PATH lookup, the environment read and the single Windows
// query -- is replaced by a written answer, so no test spawns powershell.exe. It
// also records what the detector asked for, because "the query did not run" is
// part of the override contract.
type fakeHostProbe struct {
	env      map[string]string
	lookErr  error
	output   string
	queryErr error
	// hangUntilTimeout makes the query behave like exec.CommandContext: it waits
	// for the deadline in the context instead of returning output.
	hangUntilTimeout bool
	timeout          time.Duration

	gotLookPathName string
	lookPathCalls   int
	queryCalls      int
}

// probe converts the fixture into the injected probe under test. A zero timeout
// becomes a short one, so a mistake in a test cannot turn into a five second
// wait.
func (f *fakeHostProbe) probe() hostResourceProbe {
	timeout := f.timeout
	if timeout == 0 {
		timeout = 100 * time.Millisecond
	}
	return hostResourceProbe{
		getenv: func(key string) string { return f.env[key] },
		lookPath: func(name string) (string, error) {
			f.lookPathCalls++
			f.gotLookPathName = name
			if f.lookErr != nil {
				return "", f.lookErr
			}
			return `C:\Windows\System32\WindowsPowerShell\v1.0\` + name, nil
		},
		query: func(ctx context.Context) ([]byte, error) {
			f.queryCalls++
			if f.hangUntilTimeout {
				<-ctx.Done()
				return nil, ctx.Err()
			}
			if f.queryErr != nil {
				return nil, f.queryErr
			}
			return []byte(f.output), nil
		},
		timeout: timeout,
	}
}

func TestDetectHostResourcesParsesTheWindowsHostQuery(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   HostResources
	}{
		{
			name:   "the documented two line shape",
			output: hostQueryOutput,
			want:   HostResources{MemoryBytes: 16016 << 20, LogicalCPUs: 12},
		},
		{
			name:   "stray whitespace, blank lines, noise and a BOM are ignored",
			output: "\ufeffGet-CimInstance : Access is denied.\r\n\r\n  cpus = 12  \r\nother=1\n\tmemmb = 16016\t\n",
			want:   HostResources{MemoryBytes: 16016 << 20, LogicalCPUs: 12},
		},
		{
			name:   "a final line without a newline",
			output: "cpus=8\r\nmemmb=4096",
			want:   HostResources{MemoryBytes: 4096 << 20, LogicalCPUs: 8},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeHostProbe{output: tt.output}

			got := detectHostResources(fake.probe())

			if got != tt.want {
				t.Errorf("detectHostResources() with output %q = %+v, want %+v", tt.output, got, tt.want)
			}
			if fake.lookPathCalls != 1 || fake.gotLookPathName != windowsPowerShellBinary {
				t.Errorf("lookPath called %d times with %q, want 1 call with %q",
					fake.lookPathCalls, fake.gotLookPathName, windowsPowerShellBinary)
			}
			if fake.queryCalls != 1 {
				t.Errorf("query ran %d times, want exactly 1 (one process spawn for both values)", fake.queryCalls)
			}
		})
	}
}

func TestDetectHostResourcesUnknownWhenTheHostCannotBeRead(t *testing.T) {
	tests := []struct {
		name          string
		lookErr       error
		output        string
		queryErr      error
		wantQueryRuns int
	}{
		{
			name:          "powershell.exe is not on PATH (interop disabled)",
			lookErr:       exec.ErrNotFound,
			output:        hostQueryOutput,
			wantQueryRuns: 0,
		},
		{
			name:          "the query exits non-zero",
			output:        hostQueryOutput,
			queryErr:      errors.New("exit status 1"),
			wantQueryRuns: 1,
		},
		{
			name:          "garbage output",
			output:        "Get-CimInstance : Access is denied.\r\n",
			wantQueryRuns: 1,
		},
		{
			name:          "empty output",
			output:        "",
			wantQueryRuns: 1,
		},
		{
			name:          "only the cpu line is present",
			output:        "cpus=12\r\n",
			wantQueryRuns: 1,
		},
		{
			name:          "only the memory line is present",
			output:        "memmb=16000\r\n",
			wantQueryRuns: 1,
		},
		{
			name:          "both values are zero",
			output:        "cpus=0\r\nmemmb=0\r\n",
			wantQueryRuns: 1,
		},
		{
			name:          "a negative cpu count",
			output:        "cpus=-1\r\nmemmb=16000\r\n",
			wantQueryRuns: 1,
		},
		{
			name:          "the cpu count is not a number",
			output:        "cpus=twelve\r\nmemmb=16000\r\n",
			wantQueryRuns: 1,
		},
		{
			name:          "the memory value is not a number",
			output:        "cpus=12\r\nmemmb=lots\r\n",
			wantQueryRuns: 1,
		},
		{
			// The largest MiB count that survives the shift into uint64 bytes is
			// 2^44 - 1; 2^44 MiB must be treated as garbage, not as a wrapped
			// 16 TiB host.
			name:          "the memory value overflows the byte conversion",
			output:        "cpus=12\r\nmemmb=17592186044416\r\n",
			wantQueryRuns: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeHostProbe{output: tt.output, lookErr: tt.lookErr, queryErr: tt.queryErr}

			got := detectHostResources(fake.probe())

			if got != (HostResources{}) {
				t.Errorf("detectHostResources() = %+v, want the zero value (unknown host)", got)
			}
			if fake.queryCalls != tt.wantQueryRuns {
				t.Errorf("query ran %d times, want %d", fake.queryCalls, tt.wantQueryRuns)
			}
		})
	}
}

func TestDetectHostResourcesEnvironmentOverrides(t *testing.T) {
	// The query answers with different values, so a result that matches the
	// overrides cannot have come from the query.
	const queryOutput = "cpus=4\r\nmemmb=2048\r\n"
	fromQuery := HostResources{MemoryBytes: 2048 << 20, LogicalCPUs: 4}

	tests := []struct {
		name          string
		env           map[string]string
		want          HostResources
		wantQueryRuns int
	}{
		{
			name:          "both overrides set and valid skip the query entirely",
			env:           map[string]string{envWSLHostCPUs: "12", envWSLHostMemoryMB: "16000"},
			want:          HostResources{MemoryBytes: 16000 << 20, LogicalCPUs: 12},
			wantQueryRuns: 0,
		},
		{
			name:          "the cpu override alone still queries",
			env:           map[string]string{envWSLHostCPUs: "12"},
			want:          fromQuery,
			wantQueryRuns: 1,
		},
		{
			name:          "the memory override alone still queries",
			env:           map[string]string{envWSLHostMemoryMB: "16000"},
			want:          fromQuery,
			wantQueryRuns: 1,
		},
		{
			name:          "a non-integer cpu override falls back to the query",
			env:           map[string]string{envWSLHostCPUs: "twelve", envWSLHostMemoryMB: "16000"},
			want:          fromQuery,
			wantQueryRuns: 1,
		},
		{
			name:          "a zero cpu override is not a capacity",
			env:           map[string]string{envWSLHostCPUs: "0", envWSLHostMemoryMB: "16000"},
			want:          fromQuery,
			wantQueryRuns: 1,
		},
		{
			name:          "a negative memory override is not a capacity",
			env:           map[string]string{envWSLHostCPUs: "12", envWSLHostMemoryMB: "-1"},
			want:          fromQuery,
			wantQueryRuns: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeHostProbe{env: tt.env, output: queryOutput}

			got := detectHostResources(fake.probe())

			if got != tt.want {
				t.Errorf("detectHostResources() with env %v = %+v, want %+v", tt.env, got, tt.want)
			}
			if fake.queryCalls != tt.wantQueryRuns {
				t.Errorf("query ran %d times, want %d", fake.queryCalls, tt.wantQueryRuns)
			}
			if tt.wantQueryRuns == 0 && fake.lookPathCalls != 0 {
				t.Errorf("lookPath ran %d times, want 0: the query must be skipped entirely", fake.lookPathCalls)
			}
		})
	}
}

func TestDetectHostResourcesOverrideVariableNames(t *testing.T) {
	// The two names are the documented escape hatch for a host with interop
	// disabled, so they are part of the contract rather than an implementation
	// detail.
	if envWSLHostCPUs != "DOTFILES_WSL_HOST_CPUS" {
		t.Errorf("envWSLHostCPUs = %q, want %q", envWSLHostCPUs, "DOTFILES_WSL_HOST_CPUS")
	}
	if envWSLHostMemoryMB != "DOTFILES_WSL_HOST_MEMORY_MB" {
		t.Errorf("envWSLHostMemoryMB = %q, want %q", envWSLHostMemoryMB, "DOTFILES_WSL_HOST_MEMORY_MB")
	}
}

func TestDetectHostResourcesQueryTimeout(t *testing.T) {
	// The shipped probe is bounded by five seconds.
	if got := defaultHostResourceProbe().timeout; got != 5*time.Second {
		t.Errorf("defaultHostResourceProbe().timeout = %v, want %v", got, 5*time.Second)
	}

	// The probe under test uses a short timeout, so a query that waits for the
	// context is cancelled quickly instead of hanging the detection.
	fake := &fakeHostProbe{output: hostQueryOutput, hangUntilTimeout: true, timeout: 20 * time.Millisecond}

	start := time.Now()
	got := detectHostResources(fake.probe())
	elapsed := time.Since(start)

	if got != (HostResources{}) {
		t.Errorf("detectHostResources() after the timeout = %+v, want the zero value", got)
	}
	if fake.queryCalls != 1 {
		t.Errorf("query ran %d times, want exactly 1", fake.queryCalls)
	}
	if elapsed > 2*time.Second {
		t.Errorf("detection returned after %v; a cancelled query must not block the installer", elapsed)
	}
}

// TestDetectHostResourcesQueryWaitDelayBoundsThePipe pins the second half of the
// timeout contract: the deadline cancels the process, and the wait delay bounds
// the I/O wait that survives the cancellation.
//
// This is honest about what it proves: it asserts the constants, not the exec
// behaviour. A fake probe replaces the query entirely, and only a real
// powershell.exe whose child inherited the stdout pipe and outlived the kill
// could show the difference at runtime, which no test may spawn. What the
// assertion does catch is the regression that matters here -- someone dropping
// WaitDelay again, which is invisible while the pipes close on their own and
// hangs the install step when they do not.
func TestDetectHostResourcesQueryWaitDelayBoundsThePipe(t *testing.T) {
	if hostQueryWaitDelay <= 0 {
		t.Errorf("hostQueryWaitDelay = %v, want a positive bound on the post-deadline I/O wait", hostQueryWaitDelay)
	}
	if hostQueryWaitDelay >= hostQueryTimeout {
		t.Errorf("hostQueryWaitDelay = %v, want less than hostQueryTimeout = %v", hostQueryWaitDelay, hostQueryTimeout)
	}
}
