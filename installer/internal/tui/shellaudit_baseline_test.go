package tui

import (
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// The clean comparison: how much of the startup is the user's own doing
// ---------------------------------------------------------------------------
//
// "With and without configuration" is the only measurement here that separates
// the shell from what the user put in front of it. The guard that matters most is
// the refusal: two numbers that describe different starts -- a different shell, a
// different mode, a different number of runs, a measurement that was cancelled --
// are never subtracted, because the difference between them would be an artefact
// of the measurement presented as a finding about the configuration.

// comparisonRun is one finished measurement as the comparison sees it.
func comparisonRun(mode shellAuditMode, samples ...time.Duration) *shellAuditRun {
	return &shellAuditRun{
		Kind:    shellAuditKindZsh,
		Shell:   "zsh",
		Path:    "/usr/bin/zsh",
		Version: shellAuditVersion{Known: true, Raw: "zsh 5.9.2 (x86_64-pc-linux-gnu)", Number: "5.9.2", Major: 5, Minor: 9},
		Mode:    mode,
		Command: "zsh -i -c exit",
		Samples: samples,
		Stats:   shellAuditStatsOf(samples),
	}
}

// TestShellAuditComparesTheConfiguredStartWithTheCleanOne pins the comparison
// itself: the overhead is the difference of the two medians, the share is that
// difference against the configured start, and the sentence names both numbers so
// a reader can check the arithmetic.
func TestShellAuditComparesTheConfiguredStartWithTheCleanOne(t *testing.T) {
	configured := comparisonRun(shellAuditModeInteractive,
		390*time.Millisecond, 400*time.Millisecond, 410*time.Millisecond)
	clean := comparisonRun(shellAuditMode{Interactive: true, Clean: true},
		140*time.Millisecond, 150*time.Millisecond, 160*time.Millisecond)

	comparison := compareShellAuditBaseline(configured, clean)

	if comparison.Reason != "" {
		t.Fatalf("the comparison was refused: %s", comparison.Reason)
	}
	if comparison.Configured != 400*time.Millisecond {
		t.Errorf("the configured median = %v, want 400 ms", comparison.Configured)
	}
	if comparison.Clean != 150*time.Millisecond {
		t.Errorf("the clean median = %v, want 150 ms", comparison.Clean)
	}
	if comparison.Overhead != 250*time.Millisecond {
		t.Errorf("the overhead = %v, want the 250 ms between the two medians", comparison.Overhead)
	}
	if comparison.Share < 62.4 || comparison.Share > 62.6 {
		t.Errorf("the share = %.2f%%, want 62.5%% of the configured start", comparison.Share)
	}
	if comparison.Samples != 3 {
		t.Errorf("the comparison reports %d starts per side, want 3", comparison.Samples)
	}

	headline := comparison.headline()
	for _, want := range []string{"250 ms", "400 ms", "150 ms"} {
		if !strings.Contains(headline, want) {
			t.Errorf("the headline %q does not carry %q", headline, want)
		}
	}
	if verdict := comparison.verdict(); !strings.Contains(verdict, "configuration") {
		t.Errorf("the verdict %q does not say what the difference belongs to", verdict)
	}
}

// TestShellAuditSaysWhenTheStartupIsAlmostAllShell is the honesty rule for a
// small difference: a startup that costs two milliseconds more than a clean one
// has not found a configuration problem, and the screen must not make one out of
// it. The comparison is still reported -- the number is real -- and the verdict
// says the startup is the shell's own.
func TestShellAuditSaysWhenTheStartupIsAlmostAllShell(t *testing.T) {
	configured := comparisonRun(shellAuditModeInteractive,
		101*time.Millisecond, 102*time.Millisecond, 103*time.Millisecond)
	clean := comparisonRun(shellAuditMode{Interactive: true, Clean: true},
		100*time.Millisecond, 100*time.Millisecond, 101*time.Millisecond)

	comparison := compareShellAuditBaseline(configured, clean)

	if comparison.Reason != "" {
		t.Fatalf("the comparison was refused: %s", comparison.Reason)
	}
	if comparison.Overhead != 2*time.Millisecond {
		t.Errorf("the overhead = %v, want 2 ms", comparison.Overhead)
	}
	verdict := comparison.verdict()
	if !strings.Contains(verdict, "shell itself") {
		t.Errorf("the verdict %q does not report that almost all of the start is the shell's own", verdict)
	}
	for _, forbidden := range []string{"optimise", "optimize", "your configuration costs"} {
		if strings.Contains(strings.ToLower(verdict), forbidden) {
			t.Errorf("the verdict %q turned a 2 ms difference into advice", verdict)
		}
	}
}

// TestShellAuditDoesNotCallTheCleanStartFasterWhenItIsNot pins the other sign: a
// clean start that measured the same or slower is not a saving, and the
// comparison says the two starts are not separable rather than reporting a
// negative overhead as an improvement.
func TestShellAuditDoesNotCallTheCleanStartFasterWhenItIsNot(t *testing.T) {
	configured := comparisonRun(shellAuditModeInteractive,
		200*time.Millisecond, 210*time.Millisecond, 220*time.Millisecond)
	clean := comparisonRun(shellAuditMode{Interactive: true, Clean: true},
		215*time.Millisecond, 225*time.Millisecond, 235*time.Millisecond)

	comparison := compareShellAuditBaseline(configured, clean)

	if comparison.Reason != "" {
		t.Fatalf("the comparison was refused: %s", comparison.Reason)
	}
	if comparison.Overhead >= 0 {
		t.Errorf("the overhead = %v, want the measured difference of -5 ms", comparison.Overhead)
	}
	verdict := comparison.verdict()
	if !strings.Contains(verdict, "not separable") {
		t.Errorf("the verdict %q does not report that the two starts cannot be separated", verdict)
	}
	if !strings.Contains(verdict, "slower") {
		t.Errorf("the verdict %q does not say which of the two measured slower", verdict)
	}
	for _, forbidden := range []string{"saving", "faster", "improve"} {
		if strings.Contains(strings.ToLower(verdict), forbidden) {
			t.Errorf("the verdict %q reads the negative difference as %s", verdict, forbidden)
		}
	}
	if headline := comparison.headline(); strings.Contains(headline, "-") || strings.Contains(headline, "came from") {
		t.Errorf("the headline %q prints a negative difference as a configuration cost", headline)
	}
}

// TestShellAuditRefusesToCompareStartsThatAreNotTheSameThing is the refusal that
// makes the comparison worth printing: a subtraction between two measurements of
// different starts would look like a finding about the configuration and be
// nothing of the kind.
func TestShellAuditRefusesToCompareStartsThatAreNotTheSameThing(t *testing.T) {
	configured := comparisonRun(shellAuditModeInteractive,
		390*time.Millisecond, 400*time.Millisecond, 410*time.Millisecond)

	tests := []struct {
		name  string
		clean *shellAuditRun
		want  string
	}{
		{
			name: "another mode",
			clean: func() *shellAuditRun {
				run := comparisonRun(shellAuditMode{Login: true, Interactive: true, Clean: true},
					140*time.Millisecond, 150*time.Millisecond, 160*time.Millisecond)
				return run
			}(),
			want: "different start",
		},
		{
			name: "another version",
			clean: func() *shellAuditRun {
				run := comparisonRun(shellAuditMode{Interactive: true, Clean: true},
					140*time.Millisecond, 150*time.Millisecond, 160*time.Millisecond)
				run.Version = shellAuditVersion{Known: true, Raw: "zsh 5.8", Number: "5.8", Major: 5, Minor: 8}
				return run
			}(),
			want: "another version",
		},
		{
			name: "another shell",
			clean: func() *shellAuditRun {
				run := comparisonRun(shellAuditMode{Interactive: true, Clean: true},
					140*time.Millisecond, 150*time.Millisecond, 160*time.Millisecond)
				run.Shell, run.Path = "bash", "/usr/bin/bash"
				run.Kind = shellAuditKindBash
				return run
			}(),
			want: "another shell",
		},
		{
			name: "the clean measurement was cancelled",
			clean: func() *shellAuditRun {
				run := comparisonRun(shellAuditMode{Interactive: true, Clean: true},
					140*time.Millisecond, 150*time.Millisecond, 160*time.Millisecond)
				run.Cancelled = true
				return run
			}(),
			want: "cancelled",
		},
		{
			name: "the clean measurement could not be made",
			clean: func() *shellAuditRun {
				run := comparisonRun(shellAuditMode{Interactive: true, Clean: true},
					140*time.Millisecond, 150*time.Millisecond, 160*time.Millisecond)
				run.Reason = "a clean start needs a directory of the utility's own"
				return run
			}(),
			want: "could not be made",
		},
		{
			name: "too few starts on the clean side",
			clean: comparisonRun(shellAuditMode{Interactive: true, Clean: true},
				140*time.Millisecond, 150*time.Millisecond),
			want: "starts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			comparison := compareShellAuditBaseline(configured, tt.clean)

			if comparison.Reason == "" {
				t.Fatalf("the comparison was made anyway: %+v", comparison)
			}
			if !strings.Contains(comparison.Reason, tt.want) {
				t.Errorf("the reason %q does not name %q", comparison.Reason, tt.want)
			}
			if comparison.Overhead != 0 || comparison.Configured != 0 || comparison.Clean != 0 {
				t.Errorf("a refused comparison still carries numbers: %+v", comparison)
			}
			if comparison.headline() != "" || comparison.verdict() != "" {
				t.Errorf("a refused comparison still prints %q", comparison.headline())
			}
		})
	}
}

// TestShellAuditRefusesToCompareTooFewConfiguredStarts pins the same floor on the
// configured side, and the case where there is no clean measurement at all.
func TestShellAuditRefusesToCompareTooFewConfiguredStarts(t *testing.T) {
	clean := comparisonRun(shellAuditMode{Interactive: true, Clean: true},
		140*time.Millisecond, 150*time.Millisecond, 160*time.Millisecond)

	thin := comparisonRun(shellAuditModeInteractive, 400*time.Millisecond, 410*time.Millisecond)
	if comparison := compareShellAuditBaseline(thin, clean); !strings.Contains(comparison.Reason, "starts") {
		t.Errorf("two configured starts were compared: %+v", comparison)
	}

	if comparison := compareShellAuditBaseline(clean, nil); !strings.Contains(comparison.Reason, "not measured") {
		t.Errorf("a missing clean measurement was compared: %+v", comparison)
	}
	if comparison := compareShellAuditBaseline(nil, clean); !strings.Contains(comparison.Reason, "not measured") {
		t.Errorf("a missing configured measurement was compared: %+v", comparison)
	}
}

// TestShellAuditComparisonFloorIsTheSpreadFloor pins the number the guards above
// depend on: the comparison asks for at least as many starts as the spread does,
// because a median drawn from fewer is a number without a spread to check it
// against.
func TestShellAuditComparisonFloorIsTheSpreadFloor(t *testing.T) {
	if shellAuditComparisonFloor != shellAuditSpreadFloor {
		t.Errorf("shellAuditComparisonFloor = %d, want the spread floor %d",
			shellAuditComparisonFloor, shellAuditSpreadFloor)
	}
	if shellAuditComparisonFloor < 3 {
		t.Errorf("shellAuditComparisonFloor = %d, want at least 3", shellAuditComparisonFloor)
	}
}

// TestShellAuditReportsTheCleanComparisonOfAWholeMeasurement pins the comparison
// against a real measurement: the configured median against the clean one, both
// made by the engine, with the numbers the screen prints.
func TestShellAuditReportsTheCleanComparisonOfAWholeMeasurement(t *testing.T) {
	f := zshAuditFixture()
	// The configured starts take 400 ms; the clean ones 150 ms. The plan asks for
	// the clean comparison, so the engine makes both sets in one measurement.
	f.script = []time.Duration{
		400 * time.Millisecond, 400 * time.Millisecond, 400 * time.Millisecond,
		150 * time.Millisecond, 150 * time.Millisecond, 150 * time.Millisecond,
	}

	st := resolveShellAudit(f.probe())
	st.Plan = shellAuditPlan{Mode: shellAuditModeInteractive, Samples: 3, Baseline: true}
	st = measureShellAudit(st, f.probe())

	if st.Baseline == nil {
		t.Fatalf("the plan asked for a clean comparison and none was made: %s", st.Note)
	}
	configured := st.shellAuditRun()
	comparison := compareShellAuditBaseline(&configured, st.Baseline)
	if comparison.Reason != "" {
		t.Fatalf("the comparison of a real measurement was refused: %s", comparison.Reason)
	}
	if comparison.Configured != 400*time.Millisecond || comparison.Clean != 150*time.Millisecond {
		t.Errorf("the comparison = configured %v, clean %v, want 400 ms and 150 ms",
			comparison.Configured, comparison.Clean)
	}
	if comparison.Overhead != 250*time.Millisecond {
		t.Errorf("the overhead = %v, want 250 ms", comparison.Overhead)
	}
	// The clean start is the same shell, the same mode and the same number of
	// runs: what differs is the configuration, which is the whole point.
	if st.Baseline.Kind != st.Kind || !st.Baseline.Mode.Clean || st.Baseline.Mode.Login != st.Mode.Login {
		t.Errorf("the clean start was not the same start without configuration: %+v", st.Baseline.Mode)
	}
	if len(st.Baseline.Samples) != 3 {
		t.Errorf("the clean measurement made %d starts, want the plan's 3", len(st.Baseline.Samples))
	}
	if st.Baseline.Stats.Median != 150*time.Millisecond {
		t.Errorf("the clean median = %v, want 150 ms", st.Baseline.Stats.Median)
	}
}

// TestShellAuditCleanComparisonNeedsNoCleanBaselineOfItsOwn pins the case that
// makes the clean start and the clean comparison different things: measuring the
// clean start IS the comparison, so a plan that asks for the clean start does not
// pay for a second clean measurement.
func TestShellAuditCleanComparisonNeedsNoCleanBaselineOfItsOwn(t *testing.T) {
	f := zshAuditFixture()
	f.script = []time.Duration{100 * time.Millisecond, 110 * time.Millisecond, 120 * time.Millisecond}

	st := resolveShellAudit(f.probe())
	st.Plan = shellAuditPlan{Mode: shellAuditModeClean, Samples: 3, Baseline: true}
	st = measureShellAudit(st, f.probe())

	if st.Baseline != nil {
		t.Errorf("a clean plan paid for a second clean measurement: %+v", st.Baseline)
	}
	// Three clean runs and the one profiled start. A second clean measurement --
	// the thing this guard is about -- would make it seven.
	if len(f.calls) != 4 {
		t.Errorf("the shell was started %d times, want the three clean runs and the profiled start", len(f.calls))
	}
	measured := st.shellAuditRun()
	if comparison := compareShellAuditBaseline(&measured, &measured); comparison.Reason != "" {
		t.Errorf("a clean measurement cannot be compared with itself: %s", comparison.Reason)
	}
}
