package tui

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// The attribution: what the shell's own instrument named
// ---------------------------------------------------------------------------
//
// Three instruments, three shapes of output, one contract: a row is a name, a
// kind and the time the instrument measured, nothing invented, and nothing raw.
// The guards below pin the two parsers that this front added, and the rule that
// makes them safe: the command text an instrument prints -- which is where a
// secret would be, because a trace prints commands after expansion -- is reduced
// to a name before it leaves the parser.

// fishProfileSampleOutput is fish's own startup profile as `fish
// --profile-startup=FILE` writes it, taken on this machine from a configuration
// whose own command is the most expensive one in the table.
const fishProfileSampleOutput = `Time (μs)   Sum (μs)  Command
       468        468 > set -g IFS \n\ \t
       493        512 > status get-file /usr/share/fish/build_paths.fish | source
        44     300886 > builtin source /home/alberto/.config/fish/config.fish
    300824     300824 -> sleep 0.30
         2          2 > set -qg __fish_added_user_paths
        12         12 > source /home/alberto/.config/fish/dotfiles.d/aliases.fish
`

// TestShellAuditReadsFishsOwnProfile pins the fish parser: the header is
// required, the rows are aggregated by name, and the heaviest is first.
func TestShellAuditReadsFishsOwnProfile(t *testing.T) {
	rows, reported := parseFishProfile([]byte(fishProfileSampleOutput))
	if !reported {
		t.Fatal("fish's table was not recognized")
	}
	if len(rows) == 0 {
		t.Fatal("fish's table named nothing")
	}

	byName := map[string]shellAuditFunction{}
	for _, row := range rows {
		byName[row.Name] = row
	}

	source, ok := byName["source /home/alberto/.config/fish/config.fish"]
	if !ok {
		t.Fatalf("the config fish sourced was not named: %+v", rows)
	}
	if source.Kind != shellAuditAttributionScript {
		t.Errorf("a sourced file was named as a %q, want a script", source.Kind)
	}
	if source.Self != 44*time.Microsecond {
		t.Errorf("the config's own time = %v, want fish's 44 µs", source.Self)
	}
	if source.Total != 300886*time.Microsecond {
		t.Errorf("the config's total = %v, want fish's own sum of 300886 µs", source.Total)
	}

	sleep, ok := byName["sleep"]
	if !ok {
		t.Fatalf("the command that actually cost the time was not named: %+v", rows)
	}
	if sleep.Self != 300824*time.Microsecond {
		t.Errorf("the sleep's time = %v, want fish's 300824 µs", sleep.Self)
	}
	if sleep.Kind != shellAuditAttributionCommand {
		t.Errorf("a command was named as a %q, want a command", sleep.Kind)
	}

	// The two `set` rows are one row: a list of thirty `set` lines by cost names
	// nothing, which is why the aggregation exists.
	setRows := 0
	for _, row := range rows {
		if row.Name == "set" {
			setRows++
			if row.Calls != 2 {
				t.Errorf("the set rows were counted %d times, want the two in the table", row.Calls)
			}
		}
	}
	if setRows != 1 {
		t.Errorf("the set rows were listed %d times, want one aggregate row", setRows)
	}

	// The ranking is by self time, so nothing heavier than the sleep may come
	// first.
	if rows[0].Self < sleep.Self {
		t.Errorf("the first row = %+v, want the heaviest one", rows[0])
	}

	// No share is invented for fish. Every row's own sum already contains the rows
	// under it, so a denominator cannot be recovered from the table, and a
	// percentage nobody can re-derive is exactly the per-plugin number this
	// utility refuses to print.
	for _, row := range rows {
		if row.Percent != "" {
			t.Errorf("the row %q carries the invented share %q", row.Name, row.Percent)
		}
	}
	if sleep.Total != sleep.Self {
		t.Errorf("a leaf command reports total %v and self %v, want fish's own sum for both", sleep.Total, sleep.Self)
	}
}

// TestShellAuditRedactsTheCommandsItReads is the secret rule, and it is the
// reason the fish table and the bash trace are parsed rather than shown: a
// command's text is where an exported token is, and a name is all the utility
// keeps.
func TestShellAuditRedactsTheCommandsItReads(t *testing.T) {
	const table = `Time (μs)   Sum (μs)  Command
      9000       9000 > set -gx API_KEY hunter2-secret-value
      8000       8000 > curl -H 'Authorization: Bearer hunter2-secret-value' https://example.test
      1000       1000 > source /home/alberto/.config/fish/secrets.fish
`

	rows, reported := parseFishProfile([]byte(table))
	if !reported {
		t.Fatal("the table was not recognized")
	}

	kinds := map[string]shellAuditAttributionKind{}
	for _, row := range rows {
		if strings.Contains(row.Name, "hunter2") {
			t.Errorf("the row %q carries a secret", row.Name)
		}
		if strings.ContainsAny(row.Name, " \t") && !strings.HasPrefix(row.Name, "source ") {
			t.Errorf("the row %q kept more than a command word", row.Name)
		}
		kinds[row.Name] = row.Kind
	}
	if kinds["set"] != shellAuditAttributionCommand {
		t.Errorf("the set row was not reduced to its command word: %+v", rows)
	}
	if kinds["curl"] != shellAuditAttributionCommand {
		t.Errorf("the curl row was not reduced to its command word: %+v", rows)
	}
	if _, ok := kinds["source /home/alberto/.config/fish/secrets.fish"]; !ok {
		t.Errorf("the sourced file's path was lost: %+v", rows)
	}
}

// TestShellAuditFishProfileWithoutATableSaysWhy pins the refusal: a fish that
// wrote no table names nothing and says so, and a table without its header is
// not read as one.
func TestShellAuditFishProfileWithoutATableSaysWhy(t *testing.T) {
	if _, reported := parseFishProfile([]byte("not a profile\n")); reported {
		t.Error("a file that is not fish's table was read as one")
	}
	if rows, reported := parseFishProfile([]byte(fishProfileHeaderPrefix + "   Sum (μs)  Command\n")); !reported || len(rows) != 0 {
		t.Errorf("a header with no row gave %d rows (reported %v), want none reported", len(rows), reported)
	}
}

// bashTraceSampleOutput is bash's own trace as the wrapper makes it print, one
// line per traced command: the stamp, the file and line the command came from,
// the function it ran in, and the command text -- which the parser drops.
const bashTraceSampleOutput = `+1780000000.000000+/home/alberto/.bashrc:1+ sleep 0.20
+1780000000.200000+/home/alberto/.bashrc:2+ echo loading
+1780000000.250000+/home/alberto/lib/setup.sh:7+ _helper
+1780000000.280000+/home/alberto/lib/helpers.sh:3+_helper echo inside
+1780000000.900000+/home/alberto/lib/helpers.sh:4+_helper echo more
+1780000000.950000+/home/alberto/.bashrc:9+ export API_KEY=hunter2-secret-value
`

// TestShellAuditReadsBashsTraceAsItArrives pins the trace parser: the elapsed
// time of each traced command is the distance to the next stamp, it is attributed
// both to the file the command came from and to the function it ran in, and the
// command text itself is never kept.
func TestShellAuditReadsBashsTraceAsItArrives(t *testing.T) {
	profile := newShellAuditTraceProfile()
	for _, line := range strings.Split(bashTraceSampleOutput, "\n") {
		if line == "" {
			continue
		}
		profile.line(line)
	}

	rows := profile.ranked()
	if len(rows) == 0 {
		t.Fatal("the trace named nothing")
	}

	byName := map[string]shellAuditFunction{}
	for _, row := range rows {
		byName[row.Name] = row
	}

	// A traced command costs the distance to the next stamp. .bashrc owns the
	// 0.20 s and the 0.05 s that follow its two lines; the line that runs in the
	// helper function is charged to the function, not to the file it was written
	// in, and the trace's last line has no line after it and so no duration.
	bashrc, ok := byName["/home/alberto/.bashrc"]
	if !ok {
		t.Fatalf(".bashrc was not named: %+v", rows)
	}
	if bashrc.Kind != shellAuditAttributionScript {
		t.Errorf(".bashrc was named as a %q, want a script", bashrc.Kind)
	}
	if bashrc.Self != 250*time.Millisecond {
		t.Errorf(".bashrc's self time = %v, want 250 ms (200 + 50)", bashrc.Self)
	}

	helper, ok := byName["_helper"]
	if !ok {
		t.Fatalf("the function the trace ran was not named: %+v", rows)
	}
	if helper.Kind != shellAuditAttributionFunction {
		t.Errorf("a function was named as a %q, want a function", helper.Kind)
	}
	if helper.Self != 670*time.Millisecond {
		t.Errorf("the function's self time = %v, want 670 ms (620 + 50)", helper.Self)
	}

	// The absolute winner is the function, so the ranking puts it first.
	if rows[0].Name != "_helper" {
		t.Errorf("the first row = %q, want the function the trace spent its time in", rows[0].Name)
	}

	for _, row := range rows {
		if strings.Contains(row.Name, "hunter2") {
			t.Errorf("the row %q carries a secret", row.Name)
		}
	}
	// The exported token is on the trace's last line, whose cost is not known
	// because no line follows it -- and even that line is never kept.
	if strings.Contains(strings.Join(traceNames(rows), " "), "export") {
		t.Errorf("a command text reached the attribution: %v", traceNames(rows))
	}
}

// traceNames is the names of the rows, for an assertion about what is *not* there.
func traceNames(rows []shellAuditFunction) []string {
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		names = append(names, row.Name)
	}
	return names
}

// TestShellAuditDropsWhatIsNotATraceRow pins the other half of the parse: the
// start's own output, a warning and a banner are not trace rows, and a startup
// that prints a paragraph does not end up in the attribution or on the screen.
func TestShellAuditDropsWhatIsNotATraceRow(t *testing.T) {
	profile := newShellAuditTraceProfile()
	for _, line := range []string{
		"bash: cannot set terminal process group (137848): Inappropriate ioctl for device",
		"Welcome to the machine, here is a paragraph of banner",
		"+1780000000.000000+/home/alberto/.bashrc:1+ source /etc/profile.d/thing.sh",
		"+1780000000.100000+/home/alberto/.bashrc:2+ echo done",
		"++1780000000.150000+/home/alberto/nested.sh:1+ echo nested",
		"+1780000000.400000+/home/alberto/nested.sh:2+ echo end",
		"total 4",
		// A line with no stamp, and one whose stamp is not a number: both are
		// output rather than trace rows.
		"+/home/alberto/.bashrc:3+ echo no-stamp",
		"+nope+/home/alberto/.bashrc:4+ echo not-a-number",
	} {
		profile.line(line)
	}

	rows := profile.ranked()
	names := traceNames(rows)
	for _, row := range rows {
		for _, forbidden := range []string{"Welcome", "cannot set terminal", "total 4", "no-stamp", "not-a-number", "Inappropriate"} {
			if strings.Contains(row.Name, forbidden) {
				t.Errorf("the row %q came from a line that is not a trace row: %v", row.Name, forbidden)
			}
		}
	}
	if strings.Join(names, " ") != "/home/alberto/nested.sh /home/alberto/.bashrc" {
		t.Errorf("the named rows are %v, want the two files the trace covered", names)
	}
}

// TestShellAuditProfilesBashWithItsOwnTrace pins the whole bash path: the
// profiled start is an extra start with the wrapper's rc file, the trace is read
// from it, and the attribution the screen draws comes from those lines.
func TestShellAuditProfilesBashWithItsOwnTrace(t *testing.T) {
	f := &fakeShellAuditProbe{
		env:        map[string]string{"SHELL": "/usr/bin/bash", "HOME": "/home/alberto"},
		terminal:   true,
		script:     []time.Duration{100 * time.Millisecond, 120 * time.Millisecond, 90 * time.Millisecond},
		runs:       3,
		versionRaw: "GNU bash, version 5.3.20(1)-release\n",
		traceLines: strings.Split(strings.TrimRight(bashTraceSampleOutput, "\n"), "\n"),
	}

	st := measureShellAudit(resolveShellAudit(f.probe()), f.probe())

	if st.Kind != shellAuditKindBash {
		t.Fatalf("the measured family = %q, want bash", st.Kind)
	}
	if len(st.Samples) != 3 || st.Median != 100*time.Millisecond {
		t.Fatalf("%d samples (median %v), want the three measured starts", len(st.Samples), st.Median)
	}
	if len(st.Attribution) == 0 {
		t.Fatalf("bash's own trace named nothing: %s", st.AttributionReason)
	}
	if st.AttributionReason != "" {
		t.Errorf("an attribution kept a reason as well: %q", st.AttributionReason)
	}
	// The profiled start is the fourth call, and it is the one that carried the
	// trace: it must not be one of the samples.
	if f.traceCalls != 1 {
		t.Errorf("bash was traced %d times, want the one profiled start", f.traceCalls)
	}
	if len(f.calls) != 4 {
		t.Errorf("the shell was started %d times, want three samples and the profiled start", len(f.calls))
	}
	profiled := strings.Join(f.calls[len(f.calls)-1], " ")
	if !strings.Contains(profiled, "--rcfile") {
		t.Errorf("the profiled start = %q, want it to read the wrapper's rc file", profiled)
	}
	if st.Attribution[0].Name != "_helper" {
		t.Errorf("the heaviest row = %q, want the function the trace spent its time in", st.Attribution[0].Name)
	}

	// Nothing raw was written: the only file the profiler creates is the wrapper,
	// and the trace itself never reached a disk.
	if len(f.writes) != 1 || !strings.Contains(f.bodies[0], "EPOCHREALTIME") {
		t.Errorf("the bash profiler wrote %v, want only its wrapper", f.writes)
	}
	if strings.Contains(strings.Join(f.bodies, "\n"), "hunter2") {
		t.Errorf("a trace line reached a file the profiler wrote")
	}
}

// TestShellAuditBashProfileSaysWhyWhenThereIsNoTrace pins the two ways a bash
// trace names nothing: a start that failed, and a trace that carried no rows.
func TestShellAuditBashProfileSaysWhyWhenThereIsNoTrace(t *testing.T) {
	tests := []struct {
		name      string
		fixture   func(*fakeShellAuditProbe)
		wantIn    string
		wantCalls int
	}{
		{
			name:      "a trace with no row",
			fixture:   func(f *fakeShellAuditProbe) { f.traceLines = nil },
			wantIn:    "EPOCHREALTIME",
			wantCalls: 1,
		},
		{
			name: "a profiled start that failed",
			fixture: func(f *fakeShellAuditProbe) {
				f.traceErr = errors.New("exit status 2")
			},
			wantIn:    "failed",
			wantCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeShellAuditProbe{
				env:        map[string]string{"SHELL": "/usr/bin/bash", "HOME": "/home/alberto"},
				terminal:   true,
				script:     []time.Duration{100 * time.Millisecond, 120 * time.Millisecond},
				runs:       2,
				versionRaw: "GNU bash, version 5.3.20(1)-release\n",
				traceLines: []string{"+1780000000.000000+/home/alberto/.bashrc:1+ echo hi"},
			}
			tt.fixture(f)

			st := measureShellAudit(resolveShellAudit(f.probe()), f.probe())

			if len(st.Samples) != 2 {
				t.Errorf("%d samples, want the total measured anyway", len(st.Samples))
			}
			if len(st.Attribution) != 0 {
				t.Errorf("a trace with nothing in it still named: %+v", st.Attribution)
			}
			if !strings.Contains(st.AttributionReason, tt.wantIn) {
				t.Errorf("the reason %q does not name %q", st.AttributionReason, tt.wantIn)
			}
			if f.traceCalls != tt.wantCalls {
				t.Errorf("the trace was read %d times, want %d", f.traceCalls, tt.wantCalls)
			}
		})
	}
}

// TestShellAuditBashLoginStartIsNotTraced pins the mode-dependent refusal: a bash
// login shell reads its profile files instead of the rc file the trace can be
// pointed at, so it is not traced at all -- the total stands and the reason says
// why, rather than a trace of a different start being shown.
func TestShellAuditBashLoginStartIsNotTraced(t *testing.T) {
	f := &fakeShellAuditProbe{
		env:        map[string]string{"SHELL": "/usr/bin/bash", "HOME": "/home/alberto"},
		terminal:   true,
		script:     []time.Duration{100 * time.Millisecond, 120 * time.Millisecond},
		runs:       2,
		versionRaw: "GNU bash, version 5.3.20(1)-release\n",
	}

	st := resolveShellAudit(f.probe())
	st.Plan = shellAuditPlan{Mode: shellAuditModeLogin, Samples: 2}
	st = measureShellAudit(st, f.probe())

	if len(st.Samples) != 2 {
		t.Errorf("%d samples, want the login start measured", len(st.Samples))
	}
	if len(st.Attribution) != 0 {
		t.Errorf("a bash login start was attributed anyway: %+v", st.Attribution)
	}
	if !strings.Contains(st.AttributionReason, "profile files") {
		t.Errorf("the reason %q does not name why a login shell is not traced", st.AttributionReason)
	}
	if f.traceCalls != 0 {
		t.Errorf("a bash login start was traced %d times", f.traceCalls)
	}
}

// TestShellAuditProfilesFishWithItsStartupFlag pins the fish path and the one
// detail that makes it correct: the flag is `--profile-startup`, which profiles
// the startup, and not `--profile`, which begins after the configuration has been
// read and would describe a different thing.
func TestShellAuditProfilesFishWithItsStartupFlag(t *testing.T) {
	f := &fakeShellAuditProbe{
		env:         map[string]string{"SHELL": "/usr/bin/fish", "HOME": "/home/alberto"},
		terminal:    true,
		script:      []time.Duration{200 * time.Millisecond, 220 * time.Millisecond},
		runs:        2,
		versionRaw:  "fish, version 4.9.3\n",
		profileData: fishProfileSampleOutput,
	}

	st := measureShellAudit(resolveShellAudit(f.probe()), f.probe())

	if st.Kind != shellAuditKindFish {
		t.Fatalf("the measured family = %q, want fish", st.Kind)
	}
	if len(st.Samples) != 2 {
		t.Errorf("%d samples, want the two measured starts", len(st.Samples))
	}
	if len(st.Attribution) == 0 {
		t.Fatalf("fish's own profile named nothing: %s", st.AttributionReason)
	}

	profiled := f.calls[len(f.calls)-1]
	joined := strings.Join(profiled, " ")
	if !strings.Contains(joined, "--profile-startup=") {
		t.Errorf("the profiled start = %q, want fish's startup profile", joined)
	}
	if strings.Contains(joined, "--profile=") {
		t.Errorf("the profiled start = %q: `--profile` begins after the configuration and would describe a different start", joined)
	}
	// The fish table is read from a file of the utility's own, which is removed
	// with its directory.
	if len(f.removed) != 1 {
		t.Errorf("the profiler removed %v, want the one directory it made", f.removed)
	}
	for _, row := range st.Attribution {
		if strings.Contains(row.Name, "hunter2") {
			t.Errorf("the row %q carries a secret", row.Name)
		}
	}
}

// TestShellAuditFishProfileWithoutAFileSaysWhy pins the third refusal shape: a
// fish that wrote no profile names nothing and says so.
func TestShellAuditFishProfileWithoutAFileSaysWhy(t *testing.T) {
	f := &fakeShellAuditProbe{
		env:            map[string]string{"SHELL": "/usr/bin/fish", "HOME": "/home/alberto"},
		terminal:       true,
		script:         []time.Duration{200 * time.Millisecond},
		runs:           1,
		versionRaw:     "fish, version 4.9.3\n",
		profileMissing: true,
	}

	st := measureShellAudit(resolveShellAudit(f.probe()), f.probe())

	if len(st.Samples) != 1 {
		t.Errorf("%d samples, want the total measured anyway", len(st.Samples))
	}
	if len(st.Attribution) != 0 {
		t.Errorf("a missing profile still named: %+v", st.Attribution)
	}
	if !strings.Contains(st.AttributionReason, "wrote no table") {
		t.Errorf("the reason %q does not report that fish wrote nothing", st.AttributionReason)
	}
}

// TestShellAuditZprofIsStillReadForEveryZshStart pins what this front kept: the
// zsh path still reads zprof's own table, and it does so for the login and
// non-interactive starts too, because the wrapper directory is where zsh looks
// for its startup files whatever the mode.
func TestShellAuditZprofIsStillReadForEveryZshStart(t *testing.T) {
	for _, mode := range shellAuditModes() {
		mode := mode
		t.Run(mode.label(), func(t *testing.T) {
			f := zshAuditFixture()
			f.profileData = zprofSampleOutput

			st := resolveShellAudit(f.probe())
			st.Plan = shellAuditPlan{Mode: mode, Samples: 2}
			st = measureShellAudit(st, f.probe())

			if len(st.Attribution) == 0 {
				t.Fatalf("zprof named nothing for %s: %s", mode.label(), st.AttributionReason)
			}
			if st.Attribution[0].Name != "compinit" {
				t.Errorf("the heaviest function = %q, want compinit, which zprof put first", st.Attribution[0].Name)
			}
			if st.Attribution[0].Kind != shellAuditAttributionFunction {
				t.Errorf("zprof's row was named as a %q, want a function", st.Attribution[0].Kind)
			}
			// The profiled start is the last call, and it is pointed at the wrapper
			// directory in every mode.
			profiled := f.envs[len(f.envs)-1]
			if len(profiled) != 1 || !strings.HasPrefix(profiled[0], "ZDOTDIR=") {
				t.Errorf("the profiled start of %s was given %v, want only ZDOTDIR", mode.label(), profiled)
			}
			// Every startup file zsh reads in this mode is stepped through to the
			// user's own, so the profiled startup is the one the total describes.
			for _, name := range []string{".zshenv", ".zprofile", ".zshrc"} {
				found := false
				for _, path := range f.writes {
					if strings.HasSuffix(path, "/"+name) {
						found = true
					}
				}
				if !found {
					t.Errorf("the wrapper directory has no %s: %v", name, f.writes)
				}
			}
			if !strings.Contains(strings.Join(f.bodies, "\n"), "zshexit") {
				t.Error("no wrapper arranges for the table to be written when the shell exits")
			}
		})
	}
}

// TestShellAuditProfilerReadsNoMoreThanItMust pins the two file bounds the
// profilers rely on, so a runaway startup cannot make the utility read forever.
func TestShellAuditProfilerReadsNoMoreThanItMust(t *testing.T) {
	if shellAuditProfileFileLimit <= 0 {
		t.Errorf("shellAuditProfileFileLimit = %d, want a positive bound", shellAuditProfileFileLimit)
	}
	if shellAuditTraceLineLimit <= 0 {
		t.Errorf("shellAuditTraceLineLimit = %d, want a positive bound", shellAuditTraceLineLimit)
	}
	if shellAuditAttributionRows <= shellAuditNamedFunctions {
		t.Errorf("shellAuditAttributionRows = %d, want more than the %d the screen names",
			shellAuditAttributionRows, shellAuditNamedFunctions)
	}

	// A table larger than the bound is refused rather than parsed, and the refusal
	// says the size: a guard that only read the constant would pass while the read
	// path took everything the file held.
	oversized := zshAuditFixture()
	oversized.profileData = strings.Repeat("x", shellAuditProfileFileLimit+1)

	st := resolveShellAudit(oversized.probe())
	st.Plan = shellAuditPlan{Mode: shellAuditModeInteractive, Samples: 2}
	st = measureShellAudit(st, oversized.probe())
	if st.Median <= 0 {
		t.Fatalf("the oversized table took the total with it: %s", st.AttributionReason)
	}
	if len(st.Attribution) != 0 {
		t.Errorf("a table past the %d-byte bound named %d rows", shellAuditProfileFileLimit, len(st.Attribution))
	}
	if !strings.Contains(st.AttributionReason, shellAuditSizeLabel(shellAuditProfileFileLimit)) {
		t.Errorf("the refusal does not name the bound: %q", st.AttributionReason)
	}

	// A trace line longer than the bound is dropped rather than kept.
	profile := newShellAuditTraceProfile()
	profile.line("+1780000000.000000+/home/alberto/.bashrc:1+ " + strings.Repeat("x", shellAuditTraceLineLimit+1))
	profile.line("+1780000001.000000+/home/alberto/.bashrc:2+ echo done")
	if rows := profile.ranked(); len(rows) != 0 {
		t.Errorf("an oversized trace line produced %d rows: %+v", len(rows), rows)
	}
}

// TestShellAuditTracesNothingWhenTheBuildCannot pins the graceful degradation of
// the trace hook itself: a probe without one measures the total and says why
// nothing inside it is named.
func TestShellAuditTracesNothingWhenTheBuildCannot(t *testing.T) {
	f := &fakeShellAuditProbe{
		env:        map[string]string{"SHELL": "/usr/bin/bash", "HOME": "/home/alberto"},
		terminal:   true,
		script:     []time.Duration{100 * time.Millisecond},
		runs:       1,
		versionRaw: "GNU bash, version 5.3.20(1)-release\n",
	}
	probe := f.probe()
	probe.trace = nil

	st := measureShellAudit(resolveShellAudit(probe), probe)

	if len(st.Samples) != 1 {
		t.Errorf("%d samples, want the total measured anyway", len(st.Samples))
	}
	if !strings.Contains(st.AttributionReason, "cannot read bash's own trace") {
		t.Errorf("the reason %q does not report the missing hook", st.AttributionReason)
	}
}

// TestShellAuditWritesNothingOutsideTheProfilerSDirectory is the read-only rule
// for the two profilers this front added: every file they create lives in the
// utility's own temporary directory and is removed with it, and neither of them
// ever writes beside the user's configuration.
func TestShellAuditWritesNothingOutsideTheProfilerSDirectory(t *testing.T) {
	tests := []struct {
		shell string
		// A wrapper is needed where the instrument is a shell file the startup has
		// to read. Fish carries the profile in its own flag and needs no file of
		// the utility's at all, which is the stronger form of the same rule.
		wantWrapper bool
	}{
		{"/usr/bin/bash", true},
		{"/usr/bin/fish", false},
	}

	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			f := &fakeShellAuditProbe{
				env:         map[string]string{"SHELL": tt.shell, "HOME": "/home/alberto"},
				terminal:    true,
				script:      []time.Duration{100 * time.Millisecond},
				runs:        1,
				versionRaw:  "fish, version 4.9.3\n",
				profileData: fishProfileSampleOutput,
				traceLines:  strings.Split(strings.TrimRight(bashTraceSampleOutput, "\n"), "\n"),
			}
			if tt.shell == "/usr/bin/bash" {
				f.versionRaw = "GNU bash, version 5.3.20(1)-release\n"
			}

			st := measureShellAudit(resolveShellAudit(f.probe()), f.probe())

			if tt.wantWrapper && len(f.writes) == 0 {
				t.Fatalf("the profiler wrote no wrapper, so this guard proves nothing: %s", st.AttributionReason)
			}
			for _, path := range f.writes {
				if !strings.HasPrefix(path, f.tempDir+string(os.PathSeparator)) {
					t.Errorf("the profiler wrote %q, outside its own directory %q", path, f.tempDir)
				}
			}
			// The user's own configuration is read, never written: nothing this
			// utility creates has a path under the home directory it measured.
			if len(f.removed) == 0 {
				t.Error("the profiler left its directory behind")
			}
			for _, path := range f.removed {
				if !strings.HasPrefix(path, f.tempDir) {
					t.Errorf("the profiler removed %q, which is not its own directory", path)
				}
			}
		})
	}
}
