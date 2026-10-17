package tui

import (
	"math"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// The statistics layer: what a set of finished starts says together
// ---------------------------------------------------------------------------
//
// The guards below pin the four rules the layer is held to: the middle, the mean
// and the two ends are reported; the spread is reported only when the count can
// support it, and a percentile only when there are enough runs for it to differ
// from the range; a run beyond Tukey's fences is named as an outlier and the
// trimmed median is reported beside the plain one; and an empty set is a zero
// summary rather than a division by zero.

// shellAuditSampleMillis turns a list of millisecond counts into the durations
// the statistics are computed over, so the expected values in these guards are
// the numbers a reader can add up by hand.
func shellAuditSampleMillis(values ...int) []time.Duration {
	samples := make([]time.Duration, 0, len(values))
	for _, value := range values {
		samples = append(samples, time.Duration(value)*time.Millisecond)
	}
	return samples
}

// nearDuration reports whether got is within tolerance of want, so a value that
// comes out of a square root is compared as the number it is rather than as the
// floating-point rounding of it.
func nearDuration(got, want time.Duration, tolerance time.Duration) bool {
	difference := got - want
	if difference < 0 {
		difference = -difference
	}
	return difference <= tolerance
}

// TestShellAuditStatsReportsTheMiddleTheSpreadAndTheOutlier pins the five-number
// summary of a set with one slow run in it: a median of 250 ms drawn from runs
// that really are a quarter of a second, a mean that the 900 ms run pulls to
// 350 ms, the two Tukey hinges, the outlier, and the median of what is left once
// it is out.
func TestShellAuditStatsReportsTheMiddleTheSpreadAndTheOutlier(t *testing.T) {
	st := shellAuditStatsOf(shellAuditSampleMillis(900, 250, 100, 300, 200))

	if st.Count != 5 {
		t.Fatalf("the summary covers %d runs, want the five", st.Count)
	}
	if st.Median != 250*time.Millisecond {
		t.Errorf("median = %v, want 250 ms: the middle of 100/200/250/300/900 ms", st.Median)
	}
	if st.Mean != 350*time.Millisecond {
		t.Errorf("mean = %v, want 350 ms: 1750 ms over five runs", st.Mean)
	}
	if st.Fastest != 100*time.Millisecond || st.Slowest != 900*time.Millisecond {
		t.Errorf("range = %v..%v, want 100 ms..900 ms", st.Fastest, st.Slowest)
	}
	if st.FirstQuartile != 200*time.Millisecond || st.ThirdQuartile != 300*time.Millisecond {
		t.Errorf("hinges = %v..%v, want 200 ms..300 ms: the medians of the lower three and the upper three runs",
			st.FirstQuartile, st.ThirdQuartile)
	}
	if !st.SpreadReported {
		t.Fatal("five runs reported no spread, but three runs are enough to have one")
	}
	// The sample standard deviation of 100/200/250/300/900 ms about a mean of
	// 350 ms: sqrt(400000/4) ms = sqrt(100000) ms.
	if want := time.Duration(math.Sqrt(100000) * float64(time.Millisecond)); !nearDuration(st.StdDev, want, time.Microsecond) {
		t.Errorf("standard deviation = %v, want %v", st.StdDev, want)
	}

	if len(st.Outliers) != 1 || st.Outliers[0] != 900*time.Millisecond {
		t.Fatalf("outliers = %v, want the single 900 ms run: 100/200/250/300 ms sit inside the fences 50 ms..450 ms",
			st.Outliers)
	}
	if !st.TrimmedMedianReported || st.TrimmedMedian != 225*time.Millisecond {
		t.Errorf("trimmed median = %v (reported %v), want 225 ms: the middle of 100/200/250/300 ms",
			st.TrimmedMedian, st.TrimmedMedianReported)
	}

	if word, ratio, ok := st.steadiness(); !ok || word != "erratic" {
		t.Errorf("steadiness = %q (ratio %v, reported %v), want erratic: a 316 ms deviation about a 250 ms median is not a steady shell",
			word, ratio, ok)
	}
}

// TestShellAuditStatsRefusesAPercentileItCannotSupport is the rule that keeps the
// screen from printing the slowest run twice: below the floor the percentile is
// not a number at all, and at the floor it is the run the nearest rank names.
func TestShellAuditStatsRefusesAPercentileItCannotSupport(t *testing.T) {
	seven := shellAuditStatsOf(shellAuditSampleMillis(100, 110, 120, 130, 140, 150, 160))
	if seven.P95Reported {
		t.Errorf("seven runs reported a 95th percentile of %v: the nearest rank of seven runs names the slowest one, which the range already prints",
			seven.P95)
	}
	if seven.P95 != 0 {
		t.Errorf("the unreported percentile = %v, want the zero value rather than a number a screen could print", seven.P95)
	}

	ten := shellAuditStatsOf(shellAuditSampleMillis(100, 110, 120, 130, 140, 150, 160, 170, 180, 190))
	if !ten.P95Reported {
		t.Fatal("ten runs reported no 95th percentile, but ten runs are enough for one")
	}
	if ten.P95 != 190*time.Millisecond {
		t.Errorf("the 95th percentile of ten runs = %v, want 190 ms: the nearest rank of ten is the tenth", ten.P95)
	}
}

// TestShellAuditStatsOfNothingIsZero pins the empty set: no panic, no division by
// zero, and every unfounded number left at zero.
func TestShellAuditStatsOfNothingIsZero(t *testing.T) {
	st := shellAuditStatsOf(nil)

	if st.Count != 0 {
		t.Errorf("the empty summary covers %d runs, want none", st.Count)
	}
	if st.Median != 0 || st.Mean != 0 || st.Fastest != 0 || st.Slowest != 0 || st.StdDev != 0 || st.TrimmedMedian != 0 {
		t.Errorf("the empty summary is not zero: %+v", st)
	}
	if st.SpreadReported || st.P95Reported || st.TrimmedMedianReported {
		t.Errorf("the empty summary reports a statistic it has no runs for: %+v", st)
	}
	if len(st.Outliers) != 0 {
		t.Errorf("the empty summary names %d outliers", len(st.Outliers))
	}
}

// TestShellAuditStatsOfOneRunHasNoSpread pins the small counts: one run is its
// own middle and has no spread to report, and two runs have a spread only
// because two numbers always differ.
func TestShellAuditStatsOfOneRunHasNoSpread(t *testing.T) {
	one := shellAuditStatsOf(shellAuditSampleMillis(120))
	if one.Count != 1 || one.Median != 120*time.Millisecond || one.Mean != 120*time.Millisecond {
		t.Errorf("one run: %+v, want a single 120 ms sample as its own middle", one)
	}
	if one.SpreadReported {
		t.Error("one run reported a spread: a single sample has none, and a zero beside it reads as a perfectly steady shell")
	}
	if one.Fastest != 120*time.Millisecond || one.Slowest != 120*time.Millisecond {
		t.Errorf("one run: range %v..%v, want the sample at both ends", one.Fastest, one.Slowest)
	}

	two := shellAuditStatsOf(shellAuditSampleMillis(100, 300))
	if two.SpreadReported {
		t.Error("two runs reported a spread: the standard deviation of two runs is the distance between them wearing a statistic's name")
	}
	if two.Median != 200*time.Millisecond {
		t.Errorf("the median of 100 ms and 300 ms = %v, want the mean of the two middle runs = 200 ms", two.Median)
	}
}

// TestShellAuditStatsCallsAFlatRunFlat pins the other end: five identical runs
// have no spread, and their interquartile range is zero -- which must not make
// every run its own outlier, because a run cannot sit beyond a fence that is the
// value itself.
func TestShellAuditStatsCallsAFlatRunFlat(t *testing.T) {
	st := shellAuditStatsOf(shellAuditSampleMillis(240, 240, 240, 240, 240))

	if !st.SpreadReported || st.StdDev != 0 {
		t.Errorf("identical runs reported a spread of %v (reported %v), want a reported zero", st.StdDev, st.SpreadReported)
	}
	if len(st.Outliers) != 0 {
		t.Errorf("identical runs named %v as outliers", st.Outliers)
	}
	if word, _, ok := st.steadiness(); !ok || word != "steady" {
		t.Errorf("identical runs read as %q (reported %v), want steady", word, ok)
	}
}

// TestShellAuditStatsKeepsTheColdStartOutOfTheTrimmedMedian is the shape the
// utility exists for: five normal starts and one that waited on the network. The
// median says 252.5 ms, the trimmed median says 250 ms, and the difference
// between them is the honest cost of the cold start.
func TestShellAuditStatsKeepsTheColdStartOutOfTheTrimmedMedian(t *testing.T) {
	st := shellAuditStatsOf(shellAuditSampleMillis(250, 240, 245, 255, 260, 3000))

	if st.Median != 252500*time.Microsecond {
		t.Errorf("median = %v, want 252.5 ms: the mean of the two middle runs of 240/245/250/255/260/3000 ms", st.Median)
	}
	if len(st.Outliers) != 1 || st.Outliers[0] != 3*time.Second {
		t.Fatalf("outliers = %v, want the single 3 s cold start", st.Outliers)
	}
	if !st.TrimmedMedianReported || st.TrimmedMedian != 250*time.Millisecond {
		t.Errorf("trimmed median = %v (reported %v), want 250 ms: the middle of the five normal starts",
			st.TrimmedMedian, st.TrimmedMedianReported)
	}
	// The other five runs sit inside the fences 222.5 ms..282.5 ms, so the guard
	// has teeth: a summary that named a 260 ms run as unusual would fail here.
	for _, sample := range shellAuditSampleMillis(240, 245, 250, 255, 260) {
		if sample == st.Outliers[0] {
			t.Errorf("%v was named as the outlier instead of the cold start", sample)
		}
	}
}

// TestShellAuditStatsBandsTheSpread pins the three words a screen uses for the
// spread, so "variable" cannot be reached by a run the fences would call normal.
func TestShellAuditStatsBandsTheSpread(t *testing.T) {
	steady := shellAuditStatsOf(shellAuditSampleMillis(100, 102, 98, 101, 99))
	if word, ratio, ok := steady.steadiness(); !ok || word != "steady" {
		t.Errorf("a 1.6%% deviation read as %q (ratio %v), want steady", word, ratio)
	}

	variable := shellAuditStatsOf(shellAuditSampleMillis(60, 90, 120, 150, 180))
	if word, ratio, ok := variable.steadiness(); !ok || word != "variable" {
		t.Errorf("a 39%% deviation read as %q (ratio %v), want variable", word, ratio)
	}

	erratic := shellAuditStatsOf(shellAuditSampleMillis(100, 500, 200, 3000, 150))
	if word, ratio, ok := erratic.steadiness(); !ok || word != "erratic" {
		t.Errorf("a 158%% deviation read as %q (ratio %v), want erratic", word, ratio)
	}

	if _, _, ok := (shellAuditStats{Count: 1, Median: time.Second}).steadiness(); ok {
		t.Error("a single run reported steadiness, want not established")
	}
}

// TestShellAuditMedianOfIsTheMiddle pins the one-number entry point the history
// and the comparison use, so a caller that only wants the median cannot drift
// from the summary the screen draws.
func TestShellAuditMedianOfIsTheMiddle(t *testing.T) {
	if got := shellAuditMedianOf(shellAuditSampleMillis(300, 100, 200)); got != 200*time.Millisecond {
		t.Errorf("median = %v, want the middle of 100/200/300 ms", got)
	}
	if got := shellAuditMedianOf(nil); got != 0 {
		t.Errorf("the median of no runs = %v, want zero", got)
	}
}
