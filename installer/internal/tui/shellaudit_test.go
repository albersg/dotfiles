package tui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// The shell startup audit: the measurement engine
// ---------------------------------------------------------------------------
//
// The utility starts the user's login shell the way a terminal does, several
// times, and reports the median and the range. It attributes the number with
// zsh's own profiler, or says plainly that only the total is measurable. The
// guards below pin the three rules the utility is held to: every start is
// bounded by a timeout and a timeout is reported as itself, nothing outside the
// utility's own temporary directory is written, and a function is named only
// when zprof named it.

// zprofSampleOutput is a real zprof table, taken with the wrapper this utility
// generates (ZDOTDIR pointed at a directory holding it, the real .zshrc sourced
// from where ZDOTDIR pointed before). The rows the test names are the first four
// of that table, with the values zprof printed; the two omitted rows were the
// ones that follow compdef.
const zprofSampleOutput = `num  calls                time                       self            name
-----------------------------------------------------------------------------------
 1)    1        1899.98  1899.98   80.04%    819.82   819.82   34.53%  compinit
 2)    1         547.86   547.86   23.08%    547.86   547.86   23.08%  compdump
 3) 1027         330.19     0.32   13.91%    330.19     0.32   13.91%  compdef
 6)    1          68.37    68.37    2.88%     53.89    53.89    2.27%  (anon) [/home/alberto/.p10k.zsh:22]
`

// zprofHeaderOnly is zprof's table with no row under it: zmodload zsh/zprof ran,
// and nothing was profiled.
const zprofHeaderOnly = `num  calls                time                       self            name
-----------------------------------------------------------------------------------
`

// zprofRealShape is the shape zprof really writes, which the hand-trimmed table
// above does not show: the summary table, and then, for every function in it, a
// detail block that repeats the function's own row under the callers that
// reached it. The repeated rows are the ones a parser that keeps reading past the
// summary table would list a second time, so they are here to be left out.
const zprofRealShape = `num  calls                time                       self            name
-----------------------------------------------------------------------------------
 1)    1        1899.98  1899.98   80.04%    819.82   819.82   34.53%  compinit
 2)    1         547.86   547.86   23.08%    547.86   547.86   23.08%  compdump
 3) 1027         330.19     0.32   13.91%    330.19     0.32   13.91%  compdef

-----------------------------------------------------------------------------------

 1)    1        1899.98  1899.98   80.04%    819.82   819.82   34.53%  compinit
       1/2       255.16   255.16   10.75%      0.96     0.96             compaudit [4]
    1023/1027    277.14     0.27   11.67%    277.14     0.27             compdef [3]

-----------------------------------------------------------------------------------

       1/1       547.86   547.86   23.08%    547.86   547.86             compinit [1]
 2)    1         547.86   547.86   23.08%    547.86   547.86   23.08%  compdump
`

// fakeShellAuditProbe is one scripted machine: the environment, the PATH answer,
// whether stdout is a terminal, how long each start takes, which starts hang or
// fail, and what the profiled start writes. Every effect the measurement has is
// replaced, so no guard starts the runner's shell and no guard writes near the
// runner's configuration.
type fakeShellAuditProbe struct {
	env      map[string]string
	lookErr  error
	terminal bool
	// script is how much wall time each finished start spends, in the order the
	// finished starts run. A hanging start consumes no entry.
	script []time.Duration
	// hangAt holds the start indices that wait for the deadline instead of
	// finishing, the way a plugin waiting on the network does.
	hangAt map[int]bool
	// hangAll makes every start wait for the deadline.
	hangAll bool
	// failAt holds the start indices that cannot be started at all.
	failAt map[int]error
	// profileData is what the profiled start writes where zprof's table goes;
	// profileMissing makes it write no file at all.
	profileData    string
	profileMissing bool
	timeout        time.Duration
	runs           int

	clock   time.Time
	step    int
	calls   [][]string
	envs    [][]string
	writes  []string
	bodies  []string
	removed []string
	tempDir string
}

// probe converts the fixture into the injected probe under test.
func (f *fakeShellAuditProbe) probe() shellAuditProbe {
	timeout := f.timeout
	if timeout == 0 {
		timeout = 50 * time.Millisecond
	}
	runs := f.runs
	if runs == 0 {
		runs = len(f.script)
		if runs == 0 {
			runs = shellAuditRuns
		}
	}

	return shellAuditProbe{
		getenv: func(key string) string { return f.env[key] },
		lookPath: func(name string) (string, error) {
			if f.lookErr != nil {
				return "", f.lookErr
			}
			return "/usr/bin/" + filepath.Base(name), nil
		},
		isTerminal: func() bool { return f.terminal },
		now:        func() time.Time { return f.clock },
		run: func(ctx context.Context, name string, args, env []string) ([]byte, error) {
			index := len(f.calls)
			f.calls = append(f.calls, append([]string{name}, args...))
			f.envs = append(f.envs, env)
			if f.hangAll || f.hangAt[index] {
				<-ctx.Done()
				return nil, ctx.Err()
			}
			if err, bad := f.failAt[index]; bad {
				return nil, err
			}
			if f.step < len(f.script) {
				f.clock = f.clock.Add(f.script[f.step])
			}
			f.step++
			return nil, nil
		},
		timeout: timeout,
		runs:    runs,
		mkdirTemp: func() (string, error) {
			f.tempDir = filepath.Join(os.TempDir(), "dotfiles-shell-audit-fixture")
			return f.tempDir, nil
		},
		writeFile: func(path string, data []byte) error {
			f.writes = append(f.writes, path)
			f.bodies = append(f.bodies, string(data))
			return nil
		},
		readFile: func(path string) ([]byte, error) {
			if f.profileMissing {
				return nil, os.ErrNotExist
			}
			return []byte(f.profileData), nil
		},
		removeAll: func(path string) error {
			f.removed = append(f.removed, path)
			return nil
		},
	}
}

// zshAuditFixture is the common case: zsh as the login shell, a terminal, and
// five measured starts.
func zshAuditFixture() *fakeShellAuditProbe {
	return &fakeShellAuditProbe{
		env:      map[string]string{"SHELL": "/usr/bin/zsh", "HOME": "/home/alberto"},
		terminal: true,
		script: []time.Duration{
			100 * time.Millisecond,
			300 * time.Millisecond,
			200 * time.Millisecond,
			900 * time.Millisecond,
			250 * time.Millisecond,
		},
	}
}

// TestShellAuditMeasuresTheMedianOfItsRuns pins the method and the number: five
// starts of the login shell's interactive form, the median of the five, the two
// ends of the range, and the exact command a reader can run by hand.
func TestShellAuditMeasuresTheMedianOfItsRuns(t *testing.T) {
	f := zshAuditFixture()
	p := f.probe()

	st := resolveShellAudit(p)
	if !st.Available {
		t.Fatalf("the fixture's shell is not measurable: %s", st.Reason)
	}
	if st.Command != "zsh -i -c exit" {
		t.Errorf("the reported command = %q, want %q: the number has to be reproducible by hand", st.Command, "zsh -i -c exit")
	}
	if st.Runs != shellAuditRuns || st.Timeout != 50*time.Millisecond {
		t.Errorf("the method on the state = %d runs of at most %v, want %d runs of at most %v",
			st.Runs, st.Timeout, shellAuditRuns, 50*time.Millisecond)
	}

	st = measureShellAudit(st, p)

	if !st.Measured {
		t.Error("the state does not report that it measured anything")
	}
	if len(st.Samples) != 5 {
		t.Fatalf("%d samples, want the five starts", len(st.Samples))
	}
	if st.Median != 250*time.Millisecond {
		t.Errorf("median = %v, want the middle of 100/300/200/900/250 ms = 250 ms", st.Median)
	}
	if st.Fastest != 100*time.Millisecond || st.Slowest != 900*time.Millisecond {
		t.Errorf("range = %v..%v, want 100 ms..900 ms", st.Fastest, st.Slowest)
	}
	if st.Timeouts != 0 || st.Failures != 0 {
		t.Errorf("a clean fixture reported %d timeouts and %d failures", st.Timeouts, st.Failures)
	}

	// Five starts for the total and one profiled start: the profile is a second
	// start, so its own cost is never part of the median.
	if len(f.calls) != shellAuditRuns+1 {
		t.Fatalf("the shell was started %d times, want %d measured runs plus the profiled one", len(f.calls), shellAuditRuns)
	}
	for i := 0; i < shellAuditRuns; i++ {
		got := strings.Join(f.calls[i], " ")
		if want := "/usr/bin/zsh -i -c exit"; got != want {
			t.Errorf("measured start %d = %q, want %q", i+1, got, want)
		}
	}
}

// TestShellAuditReportsATimeoutInsteadOfANumber is the rule the whole utility
// stands on: a start that does not finish inside the bound is reported as a
// timeout. It is not a duration, and it is not zero.
func TestShellAuditReportsATimeoutInsteadOfANumber(t *testing.T) {
	f := &fakeShellAuditProbe{
		env:      map[string]string{"SHELL": "/usr/bin/zsh", "HOME": "/home/alberto"},
		terminal: true,
		timeout:  20 * time.Millisecond,
		runs:     5,
		script:   []time.Duration{100 * time.Millisecond, 200 * time.Millisecond, 300 * time.Millisecond},
		hangAt:   map[int]bool{1: true, 3: true},
	}

	st := measureShellAudit(resolveShellAudit(f.probe()), f.probe())

	if st.Timeouts != 2 {
		t.Errorf("timeouts = %d, want the two starts that never finished", st.Timeouts)
	}
	if len(st.Samples) != 3 {
		t.Fatalf("%d samples, want only the three starts that finished", len(st.Samples))
	}
	if st.Median != 200*time.Millisecond {
		t.Errorf("median = %v, want 200 ms: the median covers the runs that finished and says how many did", st.Median)
	}
	for _, sample := range st.Samples {
		if sample == f.timeout {
			t.Errorf("a timed-out start was recorded as the %v timeout, which reads as a duration", f.timeout)
		}
	}
	if st.Failures != 0 {
		t.Errorf("failures = %d, want the hung starts counted as timeouts rather than as failures", st.Failures)
	}
}

// TestShellAuditDoesNotHangWhenTheShellDoes is the utility's own survival: a
// shell that never returns must not keep the screen waiting. The fixture waits
// on the context exactly as a hung startup would, and the measurement has to come
// back bounded by its own timeout with every start reported as a timeout.
func TestShellAuditDoesNotHangWhenTheShellDoes(t *testing.T) {
	f := &fakeShellAuditProbe{
		env:      map[string]string{"SHELL": "/usr/bin/zsh", "HOME": "/home/alberto"},
		terminal: true,
		timeout:  20 * time.Millisecond,
		runs:     3,
		hangAll:  true,
	}

	start := time.Now()
	st := measureShellAudit(resolveShellAudit(f.probe()), f.probe())
	elapsed := time.Since(start)

	if st.Timeouts != 3 {
		t.Errorf("timeouts = %d, want every start reported as a timeout", st.Timeouts)
	}
	if len(st.Samples) != 0 || st.Median != 0 {
		t.Errorf("a shell that never finished produced %d samples and a median of %v, want no number at all", len(st.Samples), st.Median)
	}
	if st.AttributionReason == "" {
		t.Error("nothing was said about attribution after every start timed out")
	}
	if elapsed > 2*time.Second {
		t.Errorf("the measurement took %v: a cancelled start must not block the screen", elapsed)
	}
}

// TestShellAuditReadsZprofsSummaryTableOnce pins the real shape of zprof's
// output: after the summary table it repeats every row in a per-function detail
// block, so a parse that keeps reading lists every function twice and the screen
// blames the heaviest one twice. Only the summary table is read.
func TestShellAuditReadsZprofsSummaryTableOnce(t *testing.T) {
	functions, reported := parseZprofProfile([]byte(zprofRealShape))
	if !reported {
		t.Fatal("the table's header was not recognized")
	}
	if len(functions) != 3 {
		t.Fatalf("%d functions, want the summary table's three: %+v", len(functions), functions)
	}

	seen := map[string]int{}
	for _, fn := range functions {
		seen[fn.Name]++
	}
	for name, count := range seen {
		if count != 1 {
			t.Errorf("%q appears %d times, want once: the detail blocks repeat the summary", name, count)
		}
	}
}

// TestShellAuditNamesTheFunctionsZprofBlames is the attribution guard: the
// functions on screen come from zprof's table, in zprof's own order, with the
// numbers zprof printed for them -- and a name zprof printed with spaces in it
// survives the parse.
func TestShellAuditNamesTheFunctionsZprofBlames(t *testing.T) {
	f := zshAuditFixture()
	f.profileData = zprofSampleOutput

	st := measureShellAudit(resolveShellAudit(f.probe()), f.probe())

	if len(st.Attribution) == 0 {
		t.Fatalf("zprof's table named nothing: %s", st.AttributionReason)
	}
	if st.AttributionReason != "" {
		t.Errorf("a tagged-on attribution kept a reason as well: %q", st.AttributionReason)
	}
	first := st.Attribution[0]
	if first.Name != "compinit" {
		t.Errorf("the heaviest function = %q, want compinit, which zprof put first", first.Name)
	}
	if first.Calls != 1 {
		t.Errorf("compinit's call count = %d, want zprof's 1", first.Calls)
	}
	if first.Total < 1899*time.Millisecond || first.Total > 1901*time.Millisecond {
		t.Errorf("compinit's total = %v, want zprof's 1899.98 ms", first.Total)
	}
	if first.Self < 819*time.Millisecond || first.Self > 821*time.Millisecond {
		t.Errorf("compinit's self time = %v, want zprof's 819.82 ms", first.Self)
	}
	if first.Percent != "80.04%" {
		t.Errorf("compinit's share = %q, want zprof's own cell %q", first.Percent, "80.04%")
	}
	if st.Attribution[1].Name != "compdump" {
		t.Errorf("the second function = %q, want compdump: zprof's order must be kept, not re-sorted here", st.Attribution[1].Name)
	}

	var spaced string
	for _, fn := range st.Attribution {
		if strings.HasPrefix(fn.Name, "(anon)") {
			spaced = fn.Name
		}
	}
	if spaced == "" {
		t.Errorf("the anonymous function zprof printed with a path was dropped: %+v", st.Attribution)
	}
}

// TestShellAuditSaysOnlyTheTotalIsMeasurableWithoutZprof is the honesty rule for
// a shell zprof cannot profile: the total is still measured, the screen says why
// nothing is named, and no function is invented to fill the gap.
func TestShellAuditSaysOnlyTheTotalIsMeasurableWithoutZprof(t *testing.T) {
	f := zshAuditFixture()
	f.env["SHELL"] = "/usr/bin/bash"

	st := measureShellAudit(resolveShellAudit(f.probe()), f.probe())

	if st.Shell != "bash" {
		t.Fatalf("the measured shell = %q, want bash", st.Shell)
	}
	if len(st.Samples) != 5 {
		t.Errorf("%d samples, want the total measured for bash too", len(st.Samples))
	}
	if len(st.Attribution) != 0 {
		t.Errorf("bash was given a culprit: %+v", st.Attribution)
	}
	if !strings.Contains(st.AttributionReason, "zprof") || !strings.Contains(st.AttributionReason, "bash") {
		t.Errorf("the reason does not say that zprof is zsh's and that bash has no equivalent: %q", st.AttributionReason)
	}
	for _, invented := range []string{"compinit", "compdump", "compdef"} {
		if strings.Contains(st.AttributionReason, invented) {
			t.Errorf("the reason names %q, which nothing measured: %q", invented, st.AttributionReason)
		}
	}
}

// TestShellAuditDoesNotAttributeWhenZprofReportsNothing covers the other half:
// zsh with a profile that reaches no table, or a table with no row, reports the
// reason instead of a culprit.
func TestShellAuditDoesNotAttributeWhenZprofReportsNothing(t *testing.T) {
	tests := []struct {
		name    string
		fixture func(*fakeShellAuditProbe)
		want    string
	}{
		{"no table at all", func(f *fakeShellAuditProbe) { f.profileData = "" }, "zprof"},
		{"an empty table", func(f *fakeShellAuditProbe) { f.profileData = zprofHeaderOnly }, "no function"},
		{"no profile file", func(f *fakeShellAuditProbe) { f.profileMissing = true }, "zprof"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			f := zshAuditFixture()
			f.profileData = zprofSampleOutput
			tt.fixture(f)

			st := measureShellAudit(resolveShellAudit(f.probe()), f.probe())

			if len(st.Attribution) != 0 {
				t.Errorf("%s produced an attribution anyway: %+v", tt.name, st.Attribution)
			}
			if !strings.Contains(strings.ToLower(st.AttributionReason), tt.want) {
				t.Errorf("%s: the reason %q does not name %q", tt.name, st.AttributionReason, tt.want)
			}
		})
	}
}

// TestShellAuditDoesNotAttributeWithoutADirectoryToProfile covers the broken
// environment: with neither $ZDOTDIR nor $HOME naming the directory the startup
// files live in, the wrapper could only name a relative path, so the profiled
// start is not attempted -- and no wrapper is written for it. The total is still
// measured.
func TestShellAuditDoesNotAttributeWithoutADirectoryToProfile(t *testing.T) {
	f := zshAuditFixture()
	f.env = map[string]string{"SHELL": "/usr/bin/zsh"}
	f.profileData = zprofSampleOutput

	st := measureShellAudit(resolveShellAudit(f.probe()), f.probe())

	if len(st.Samples) != 5 {
		t.Errorf("%d samples, want the total measured anyway", len(st.Samples))
	}
	if len(st.Attribution) != 0 {
		t.Errorf("a profiled start that could not be pointed at anything still named: %+v", st.Attribution)
	}
	if !strings.Contains(st.AttributionReason, "$HOME") {
		t.Errorf("the reason does not name what was missing: %q", st.AttributionReason)
	}
	if len(f.writes) != 0 {
		t.Errorf("the utility wrote a wrapper it could not use: %v", f.writes)
	}
}

// TestShellAuditDoesNotAttributeWhenTheProfiledStartTimesOut pins the timeout on
// the profiled start too: a start that hangs under zprof names nobody, and the
// reason says so rather than reading as "nothing was slow".
func TestShellAuditDoesNotAttributeWhenTheProfiledStartTimesOut(t *testing.T) {
	f := zshAuditFixture()
	f.profileData = zprofSampleOutput
	// The profiled start is the sixth: five measured runs come first.
	f.hangAt = map[int]bool{shellAuditRuns: true}

	st := measureShellAudit(resolveShellAudit(f.probe()), f.probe())

	if len(st.Attribution) != 0 {
		t.Errorf("a timed-out profiled start still named a function: %+v", st.Attribution)
	}
	if !strings.Contains(st.AttributionReason, "did not finish") {
		t.Errorf("the reason does not report the timeout: %q", st.AttributionReason)
	}
}

// TestShellAuditRefusesToMeasureWhatItCannotMeasure pins the three refusals: no
// login shell named, a login shell that is not on this machine, and a run with no
// terminal. Each says which one it is, and none of them starts anything.
func TestShellAuditRefusesToMeasureWhatItCannotMeasure(t *testing.T) {
	tests := []struct {
		name    string
		fixture func(*fakeShellAuditProbe)
		want    string
	}{
		{
			name:    "no login shell",
			fixture: func(f *fakeShellAuditProbe) { f.env = map[string]string{} },
			want:    "$SHELL",
		},
		{
			name: "a login shell that is not here",
			fixture: func(f *fakeShellAuditProbe) {
				f.env["SHELL"] = "/usr/bin/zsh"
				f.lookErr = errors.New("executable file not found in $PATH")
			},
			want: "/usr/bin/zsh",
		},
		{
			name:    "no terminal",
			fixture: func(f *fakeShellAuditProbe) { f.terminal = false },
			want:    "terminal",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			f := zshAuditFixture()
			tt.fixture(f)
			p := f.probe()

			st := resolveShellAudit(p)
			if st.Available {
				t.Fatalf("%s was reported as measurable", tt.name)
			}
			if !strings.Contains(st.Reason, tt.want) {
				t.Errorf("the reason %q does not name %q", st.Reason, tt.want)
			}

			st = measureShellAudit(st, p)
			if st.Measured {
				t.Errorf("%s measured something anyway", tt.name)
			}
			if len(f.calls) != 0 {
				t.Errorf("%s started the shell %d times: %v", tt.name, len(f.calls), f.calls)
			}
		})
	}
}

// TestShellAuditWritesNothingOutsideItsOwnTemporaryDirectory is the read-only
// rule. The user's startup files are read from where ZDOTDIR already pointed --
// named inside the generated wrapper, never copied, moved or rewritten -- and the
// only files the utility creates are the two wrappers and zprof's table, all
// inside one directory of its own that it removes again.
func TestShellAuditWritesNothingOutsideItsOwnTemporaryDirectory(t *testing.T) {
	f := zshAuditFixture()
	f.profileData = zprofSampleOutput

	measureShellAudit(resolveShellAudit(f.probe()), f.probe())

	if f.tempDir == "" || len(f.writes) == 0 {
		t.Fatalf("the fixture wrote no wrapper, so this guard proves nothing (temp %q, writes %v)", f.tempDir, f.writes)
	}
	for _, path := range f.writes {
		if !strings.HasPrefix(path, f.tempDir+string(os.PathSeparator)) {
			t.Errorf("the utility wrote %q, which is outside its own temporary directory %q", path, f.tempDir)
		}
	}
	for _, path := range f.writes {
		if strings.HasPrefix(path, "/home/alberto") {
			t.Errorf("the utility wrote %q, which is the user's own configuration", path)
		}
	}
	if len(f.removed) != 1 || f.removed[0] != f.tempDir {
		t.Errorf("the temporary directory was removed %v, want exactly %q", f.removed, f.tempDir)
	}

	// The user's own files are read from where they already are: the wrapper names
	// them, quoted, and that is the only place they appear.
	bodies := strings.Join(f.bodies, "\n")
	for _, want := range []string{
		shellAuditQuote("/home/alberto/.zshenv"),
		shellAuditQuote("/home/alberto/.zshrc"),
	} {
		if !strings.Contains(bodies, want) {
			t.Errorf("the wrapper does not source the user's own file %s:\n%s", want, bodies)
		}
	}

	// The profiled start is told one thing: where the wrapper is. Every other path
	// it needs is in the wrapper's own text, so there is no variable left for the
	// environment to shadow.
	profileEnv := f.envs[len(f.envs)-1]
	if len(profileEnv) != 1 || profileEnv[0] != "ZDOTDIR="+f.tempDir {
		t.Errorf("the profiled start was given %v, want only ZDOTDIR pointing at the wrapper", profileEnv)
	}
	for _, entry := range profileEnv {
		if strings.HasPrefix(entry, "DOTFILES_") {
			t.Errorf("the profiled start was given %q, which the branding guide would have to carry", entry)
		}
	}
}

// TestShellAuditBoundsEveryStart pins the shipped bound and the shipped count.
// This is the assertion that catches a dropped timeout: with no bound the utility
// waits on the user's shell forever, which is the failure the whole rule exists
// to prevent.
func TestShellAuditBoundsEveryStart(t *testing.T) {
	probe := defaultShellAuditProbe()
	if probe.timeout != shellAuditTimeout {
		t.Errorf("the shipped probe's timeout = %v, want %v", probe.timeout, shellAuditTimeout)
	}
	if shellAuditTimeout <= 0 {
		t.Fatalf("shellAuditTimeout = %v: every start of the user's shell must be bounded", shellAuditTimeout)
	}
	if shellAuditWaitDelay <= 0 {
		t.Errorf("shellAuditWaitDelay = %v, want a positive bound on the post-deadline I/O wait", shellAuditWaitDelay)
	}
	if shellAuditWaitDelay >= shellAuditTimeout {
		t.Errorf("shellAuditWaitDelay = %v, want less than shellAuditTimeout = %v", shellAuditWaitDelay, shellAuditTimeout)
	}
	if probe.runs != shellAuditRuns {
		t.Errorf("the shipped probe's run count = %d, want %d", probe.runs, shellAuditRuns)
	}
	if shellAuditRuns%2 == 0 {
		t.Errorf("shellAuditRuns = %d, want an odd count so the median is a run that happened", shellAuditRuns)
	}
	if probe.run == nil || probe.isTerminal == nil || probe.lookPath == nil || probe.getenv == nil {
		t.Error("the shipped probe is missing an effect it needs")
	}
}

// TestShellAuditReplacesTheEnvironmentItOverrides pins the merge: a user who
// already exports ZDOTDIR must not hand the profiled start two ZDOTDIR entries,
// or zsh would read whichever one it happened to see last -- and the utility
// reads the user's own ZDOTDIR to find their startup files, so this is the normal
// case, not an exotic one.
func TestShellAuditReplacesTheEnvironmentItOverrides(t *testing.T) {
	base := []string{"PATH=/usr/bin", "ZDOTDIR=/home/alberto", "HOME=/home/alberto"}
	merged := shellAuditMergedEnv(base, []string{"ZDOTDIR=/tmp/wrapper"})

	zdotdirs := 0
	for _, entry := range merged {
		if strings.HasPrefix(entry, "ZDOTDIR=") {
			zdotdirs++
			if entry != "ZDOTDIR=/tmp/wrapper" {
				t.Errorf("ZDOTDIR = %q, want the wrapper's directory", entry)
			}
		}
	}
	if zdotdirs != 1 {
		t.Errorf("the merged environment carries %d ZDOTDIR entries, want exactly 1: %v", zdotdirs, merged)
	}
	for _, kept := range []string{"PATH=/usr/bin", "HOME=/home/alberto"} {
		found := false
		for _, entry := range merged {
			if entry == kept {
				found = true
			}
		}
		if !found {
			t.Errorf("the merge dropped %q: %v", kept, merged)
		}
	}
}

// TestShellAuditWrapperQuotesThePathsItBakesIn pins the one thing about the
// generated wrapper that no compile can check: the user's directory and the
// profile's destination are baked into its text, so they have to be quoted. A
// home directory with a space or a quote in it would otherwise turn the wrapper
// into shell syntax, the profiled start would source the wrong file or nothing,
// and the screen would report that zprof named nobody.
func TestShellAuditWrapperQuotesThePathsItBakesIn(t *testing.T) {
	home := "/home/a user's files"
	profile := "/tmp/it's a profile/zprof.txt"

	env := shellAuditZshEnvWrapper(home)
	rc := shellAuditZshRCWrapper(home, profile)

	for _, want := range []string{
		shellAuditQuote(home + "/.zshenv"),
		"zmodload zsh/zprof",
	} {
		if !strings.Contains(env, want) {
			t.Errorf("the .zshenv wrapper does not carry %q:\n%s", want, env)
		}
	}
	for _, want := range []string{
		shellAuditQuote(home + "/.zshrc"),
		"builtin zprof > " + shellAuditQuote(profile),
	} {
		if !strings.Contains(rc, want) {
			t.Errorf("the .zshrc wrapper does not carry %q:\n%s", want, rc)
		}
	}
	if strings.Contains(env, home+"/.zshenv") || strings.Contains(rc, home+"/.zshrc") {
		t.Errorf("an unquoted path reached the wrapper, so a space or a quote in it is shell syntax:\n%s\n%s", env, rc)
	}
}

// TestShellAuditQuoteIsShellSafe pins the quoting itself: one word, whatever the
// path holds.
func TestShellAuditQuoteIsShellSafe(t *testing.T) {
	tests := []struct{ path, want string }{
		{"/home/alberto", `'/home/alberto'`},
		{"/a b", `'/a b'`},
		{"/a'b", `'/a'\''b'`},
		{"/a$b", `'/a$b'`},
		{"/a`b", `'/a` + "`" + `b'`},
	}
	for _, tt := range tests {
		if got := shellAuditQuote(tt.path); got != tt.want {
			t.Errorf("shellAuditQuote(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

// TestShellAuditMeasuresThisMachinesShellLive is the opt-in check that keeps the
// utility honest about the real thing: it starts the runner's own shell with the
// real wrapper and zprof. It is skipped by default because it starts a real login
// shell with the runner's own configuration -- a guard CI must not depend on --
// and it forces the terminal answer, because a test's stdout is a pipe and the
// refusal it would otherwise hit is already covered above.
//
// Run it with: DOTFILES_SHELL_AUDIT_LIVE=1 go test ./internal/tui/ -run TestShellAuditMeasuresThisMachinesShellLive -v
func TestShellAuditMeasuresThisMachinesShellLive(t *testing.T) {
	if os.Getenv("DOTFILES_SHELL_AUDIT_LIVE") == "" {
		t.Skip("set DOTFILES_SHELL_AUDIT_LIVE=1 to measure this machine's real login shell")
	}
	if _, err := os.Stat(os.Getenv("SHELL")); err != nil {
		t.Skipf("this machine names no login shell: %v", err)
	}

	probe := defaultShellAuditProbe()
	probe.isTerminal = func() bool { return true }

	st := resolveShellAudit(probe)
	if !st.Available {
		t.Fatalf("this machine's shell is not measurable: %s", st.Reason)
	}
	st = measureShellAudit(st, probe)

	t.Logf("command: %s (timeout %v)", st.Command, st.Timeout)
	t.Logf("runs: %d, samples: %d, timeouts: %d, failures: %d %s", st.Runs, len(st.Samples), st.Timeouts, st.Failures, st.Failure)
	t.Logf("median: %v, range: %v..%v", st.Median, st.Fastest, st.Slowest)
	for i, sample := range st.Samples {
		t.Logf("  run %d: %v", i+1, sample)
	}
	if st.AttributionReason != "" {
		t.Logf("attribution: %s", st.AttributionReason)
	}
	for _, fn := range st.Attribution {
		t.Logf("  %-28s calls %5d total %8s self %8s %s",
			fn.Name, fn.Calls, shellAuditZprofTime(float64(fn.Total)/float64(time.Millisecond)),
			shellAuditZprofTime(float64(fn.Self)/float64(time.Millisecond)), fn.Percent)
	}

	if st.Measured && len(st.Samples) == 0 && st.Timeouts == 0 && st.Failures == 0 {
		t.Error("the live measurement reported neither a sample, nor a timeout, nor a failure")
	}
}
