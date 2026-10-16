package tui

import (
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// What the screen says the instrument did, and which numbers it draws
// ---------------------------------------------------------------------------
//
// The measurement reports through one shell or another, and the paragraph that
// introduces the attribution is drawn for every attributable shell. These guards
// hold the two rules the independent verification found broken: the paragraph may
// not claim zsh's instrument measured a bash trace or a fish profile, and the
// numbers the plan promised the reader -- the mean, the variability, the
// percentile -- may not be computed and then left unread.

// TestShellAuditProfileParagraphNamesTheInstrumentThatRan holds the paragraph to
// the instrument that actually ran: bash's traced start is not zprof's table and
// fish's startup profile is not either, and a share the instrument does not print
// is not printed by the screen. Covers S2/S4.
func TestShellAuditProfileParagraphNamesTheInstrumentThatRan(t *testing.T) {
	tests := []struct {
		name    string
		state   shellAuditState
		absence string
	}{
		{
			name: "bash traced by its own xtrace",
			state: shellAuditState{
				Kind: shellAuditKindBash, Shell: "bash", Measured: true,
				Attribution: []shellAuditFunction{{
					Name: "_helper", Kind: shellAuditAttributionFunction,
					Self: 670 * time.Millisecond, Total: 670 * time.Millisecond, Calls: 1,
				}},
			},
			absence: "zsh/zprof",
		},
		{
			name: "fish profiled by its own flag",
			state: shellAuditState{
				Kind: shellAuditKindFish, Shell: "fish", Measured: true,
				Attribution: []shellAuditFunction{{
					Name: "sleep", Kind: shellAuditAttributionCommand,
					Self: 300 * time.Millisecond, Total: 300 * time.Millisecond, Calls: 1,
				}},
			},
			absence: "zsh/zprof",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shellAuditAttributionParagraph(tt.state)
			if strings.Contains(got, tt.absence) {
				t.Errorf("the paragraph names %q, which did not measure this start: %q", tt.absence, got)
			}
			if strings.Contains(got, ",  of the profiled start") {
				t.Errorf("the paragraph declares a share the instrument did not print: %q", got)
			}
		})
	}

	// The instrument each shell does have is named for it, because a paragraph
	// that only stops lying would leave the reader without what did measure.
	named := []struct {
		kind shellAuditKind
		want string
	}{
		{shellAuditKindZsh, "zsh/zprof"},
		{shellAuditKindBash, "trace"},
		{shellAuditKindFish, "profile"},
	}
	for _, one := range named {
		st := shellAuditState{
			Kind: one.kind, Shell: string(one.kind), Measured: true,
			Attribution: []shellAuditFunction{{
				Name: "_helper", Kind: shellAuditAttributionFunction,
				Self: 100 * time.Millisecond, Total: 100 * time.Millisecond, Calls: 1,
			}},
		}
		if got := shellAuditAttributionParagraph(st); !strings.Contains(strings.ToLower(got), one.want) {
			t.Errorf("the %s paragraph does not name %q: %q", one.kind, one.want, got)
		}
	}

	// The explanation of self time belongs to the table that prints both numbers,
	// and the trace's rows carry one number: a sentence about a total larger than
	// its self time would describe something the reader cannot see there.
	traced := shellAuditState{
		Kind: shellAuditKindBash, Shell: "bash", Measured: true,
		Attribution: []shellAuditFunction{{
			Name: "_helper", Kind: shellAuditAttributionFunction,
			Self: 100 * time.Millisecond, Total: 100 * time.Millisecond, Calls: 1,
		}},
	}
	if got := shellAuditAttributionParagraph(traced); strings.Contains(got, "larger than its self time") {
		t.Errorf("the traced paragraph explains a total the trace does not print: %q", got)
	}
}

// TestShellAuditScreenPresentsTheStatisticsItMeasured holds S3 to the screen: a
// measurement of ten runs supports the mean, the variability and the percentile,
// and a screen that computes them and draws none of them is a screen that took
// the samples for nothing. Covers S3.
func TestShellAuditScreenPresentsTheStatisticsItMeasured(t *testing.T) {
	samples := []time.Duration{
		100 * time.Millisecond, 110 * time.Millisecond, 120 * time.Millisecond,
		130 * time.Millisecond, 140 * time.Millisecond, 150 * time.Millisecond,
		160 * time.Millisecond, 170 * time.Millisecond, 180 * time.Millisecond,
		900 * time.Millisecond,
	}

	m := installerFrameModel(t, ScreenShellAudit)
	m.ShellAudit = shellAuditTestState()
	m.ShellAudit.Samples = samples
	m.ShellAudit.Runs = len(samples)
	m.ShellAudit.Stats = shellAuditStatsOf(samples)
	m.ShellAudit.Median = m.ShellAudit.Stats.Median
	m.ShellAudit.Fastest = m.ShellAudit.Stats.Fastest
	m.ShellAudit.Slowest = m.ShellAudit.Stats.Slowest

	all := strings.Join(m.shellAuditDescription(), "\n") + "\n" + strings.Join(shellAuditSummaryRows(m.ShellAudit), "\n")
	lowered := strings.ToLower(all)
	for _, want := range []string{"95th percentile", "trimmed median", "mean"} {
		if !strings.Contains(lowered, want) {
			t.Errorf("the screen does not present %q, which the sample count supports:\n%s", want, all)
		}
	}

	// Numbers that a count cannot support are named as missing rather than
	// printed: the same screen with three runs may not report a percentile.
	st := m.ShellAudit
	st.Samples = []time.Duration{100 * time.Millisecond, 200 * time.Millisecond, 600 * time.Millisecond}
	st.Runs = len(st.Samples)
	st.Stats = shellAuditStatsOf(st.Samples)
	st.Median, st.Fastest, st.Slowest = st.Stats.Median, st.Stats.Fastest, st.Stats.Slowest

	shortModel := installerFrameModel(t, ScreenShellAudit)
	shortModel.ShellAudit = st
	short := strings.Join(shortModel.shellAuditDescription(), "\n") + "\n" + strings.Join(shellAuditSummaryRows(st), "\n")
	if strings.Contains(strings.ToLower(short), "95th percentile") {
		t.Errorf("the screen reports a 95th percentile from three runs:\n%s", short)
	}
	if !strings.Contains(strings.ToLower(short), "8") {
		t.Errorf("the screen does not say how many runs the percentile would need:\n%s", short)
	}
}
