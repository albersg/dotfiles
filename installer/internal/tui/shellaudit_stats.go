package tui

import (
	"math"
	"sort"
	"time"
)

// ============================================================================
// WHAT THE STARTS SAY TOGETHER
// ============================================================================
//
// A startup measured five times is five numbers, and the middle of them is the
// number the user wants. It is not, by itself, a measurement: a median of 250 ms
// drawn from 240, 245, 250, 255 and 260 ms is a startup that always takes a
// quarter of a second, while the same median drawn from 100, 120, 250, 900 and
// 1500 ms is a startup nobody can predict, and the screen would show the same
// 250 ms for both. The spread is what tells the two apart, and the outlier is
// what turns a "slow shell" into a cold cache or a plugin waiting on the
// network.
//
// Everything here is arithmetic on finished runs. A run that timed out has no
// duration and is never one of the samples, so nothing in this file has to
// defend against a zero that means "did not finish".

const (
	// shellAuditSpreadFloor is the number of finished starts under which the
	// spread is not reported at all. The standard deviation of two runs is the
	// distance between them wearing a statistic's name: it is a real number, and
	// it is not evidence of anything. Three is the first count at which the shape
	// of the runs can differ from the middle two.
	shellAuditSpreadFloor = 3

	// shellAuditPercentileFloor is the number of finished starts a percentile
	// needs before it is reported. The 95th percentile of five runs is the
	// slowest of the five with a percentage sign in front of it, which is a
	// number the range already prints; above eight runs it starts to say
	// something the range does not.
	shellAuditPercentileFloor = 8

	// shellAuditOutlierFence is Tukey's multiplier: a run is an outlier when it
	// sits more than one and a half times the interquartile range beyond the
	// quartiles. It is the fence the box plot uses, and it is stated here once so
	// the number a screen calls an outlier can be re-derived by hand.
	shellAuditOutlierFence = 1.5
)

// shellAuditStats is the whole summary of one set of finished starts.
//
// Every field that a count cannot support is reported as such rather than as a
// zero: SpreadReported, P95Reported and TrimmedMedianReported say whether the
// number beside them means anything, and a screen that ignores them would print
// "variability 0 ms" for a single run, which reads as a perfectly steady shell.
type shellAuditStats struct {
	// Count is how many starts the summary covers.
	Count int
	// Median is the middle start, Mean the arithmetic one, and Fastest and
	// Slowest the two ends of the range.
	Median  time.Duration
	Mean    time.Duration
	Fastest time.Duration
	Slowest time.Duration
	// FirstQuartile and ThirdQuartile are Tukey's hinges: the medians of the
	// lower and the upper half of the runs, with the middle run counted in both
	// halves when the count is odd.
	FirstQuartile time.Duration
	ThirdQuartile time.Duration
	// StdDev is the sample standard deviation, and SpreadReported says whether
	// Count supports it.
	StdDev         time.Duration
	SpreadReported bool
	// P95 is the 95th percentile by nearest rank, reported only from
	// shellAuditPercentileFloor runs up.
	P95         time.Duration
	P95Reported bool
	// Outliers are the runs beyond Tukey's fences, and TrimmedMedian is the
	// median of what is left once they are out -- the number that answers "what
	// does this shell cost when nothing unusual happens".
	Outliers              []time.Duration
	TrimmedMedian         time.Duration
	TrimmedMedianReported bool
}

// shellAuditStatsOf summarizes the finished starts. It never panics, and an
// empty set is a zero summary rather than a division by zero.
func shellAuditStatsOf(samples []time.Duration) shellAuditStats {
	st := shellAuditStats{Count: len(samples)}
	if st.Count == 0 {
		return st
	}

	sorted := append([]time.Duration(nil), samples...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	st.Fastest, st.Slowest = sorted[0], sorted[st.Count-1]
	st.Median = shellAuditMiddle(sorted)
	st.Mean = shellAuditMean(sorted)
	st.FirstQuartile, st.ThirdQuartile = shellAuditHinges(sorted)

	if st.Count >= shellAuditSpreadFloor {
		st.StdDev = shellAuditStdDev(sorted, st.Mean)
		st.SpreadReported = true
	}

	if st.Count >= shellAuditPercentileFloor {
		st.P95 = shellAuditNearestRank(sorted, 95)
		st.P95Reported = true
	}

	st.Outliers = shellAuditOutliers(sorted)
	if len(st.Outliers) > 0 {
		kept := shellAuditWithoutOutliers(sorted)
		if len(kept) > 0 {
			st.TrimmedMedian = shellAuditMiddle(kept)
			st.TrimmedMedianReported = true
		}
	}

	return st
}

// shellAuditMedianOf is the middle of a set of runs as a duration. It exists so
// the screens and the history can ask for the one number without building the
// whole summary.
func shellAuditMedianOf(samples []time.Duration) time.Duration {
	return shellAuditStatsOf(samples).Median
}

// shellAuditMiddle returns the median of a slice that is already sorted. An even
// count has no middle run, so the two that bracket the middle are averaged --
// which is what a measurement of four runs honestly has to report.
func shellAuditMiddle(sorted []time.Duration) time.Duration {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}

// shellAuditMean is the arithmetic mean of a sorted set of runs, computed in
// whole nanoseconds so it cannot drift with the magnitude of the durations.
func shellAuditMean(sorted []time.Duration) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	var sum int64
	for _, sample := range sorted {
		sum += int64(sample)
	}
	return time.Duration(sum / int64(len(sorted)))
}

// shellAuditHinges returns Tukey's quartiles of a sorted set of runs. The halves
// share the middle run when the count is odd, which is the convention the box
// plot draws and the one that keeps the two hinges from moving when one more run
// is added.
func shellAuditHinges(sorted []time.Duration) (low, high time.Duration) {
	n := len(sorted)
	if n == 0 {
		return 0, 0
	}
	half := (n + 1) / 2
	return shellAuditMiddle(sorted[:half]), shellAuditMiddle(sorted[n-half:])
}

// shellAuditStdDev is the sample standard deviation: the sum of the squared
// deviations from the mean, divided by one less than the count. The divisor is
// n-1 because the runs are a sample of a startup that can be different every
// time, not the whole population of the only startup this shell will ever make.
func shellAuditStdDev(sorted []time.Duration, mean time.Duration) time.Duration {
	if len(sorted) < 2 {
		return 0
	}
	var sum float64
	for _, sample := range sorted {
		deviation := float64(sample - mean)
		sum += deviation * deviation
	}
	return time.Duration(math.Sqrt(sum / float64(len(sorted)-1)))
}

// shellAuditNearestRank returns the percentile p of a sorted set of runs. The
// nearest-rank method is the one a reader can reproduce without a statistics
// library: with ten runs, the 95th percentile is the tenth.
func shellAuditNearestRank(sorted []time.Duration, p float64) time.Duration {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	rank := int(math.Ceil(p / 100 * float64(n)))
	if rank < 1 {
		rank = 1
	}
	if rank > n {
		rank = n
	}
	return sorted[rank-1]
}

// shellAuditOutliers returns the runs beyond Tukey's fences, in the order they
// finished, so a screen can name them beside the median they were left out of.
func shellAuditOutliers(sorted []time.Duration) []time.Duration {
	low, high, ok := shellAuditFences(sorted)
	if !ok {
		return nil
	}

	var outliers []time.Duration
	for _, sample := range sorted {
		if float64(sample) < low || float64(sample) > high {
			outliers = append(outliers, sample)
		}
	}
	return outliers
}

// shellAuditWithoutOutliers is the set of runs inside the fences: the sample the
// trimmed median is drawn from.
func shellAuditWithoutOutliers(sorted []time.Duration) []time.Duration {
	low, high, ok := shellAuditFences(sorted)
	if !ok {
		return append([]time.Duration(nil), sorted...)
	}

	kept := make([]time.Duration, 0, len(sorted))
	for _, sample := range sorted {
		if float64(sample) >= low && float64(sample) <= high {
			kept = append(kept, sample)
		}
	}
	return kept
}

// shellAuditFences returns the two Tukey fences, in nanoseconds as floats so a
// very large interquartile range cannot overflow the arithmetic. The boolean is
// false when the fences cannot exist yet: fewer than shellAuditSpreadFloor runs
// have no quartiles worth the name, and a zero interquartile range would make
// every run beyond the median its own outlier.
func shellAuditFences(sorted []time.Duration) (low, high float64, ok bool) {
	if len(sorted) < shellAuditSpreadFloor {
		return 0, 0, false
	}
	first, third := shellAuditHinges(sorted)
	iqr := float64(third - first)
	if iqr <= 0 {
		return 0, 0, false
	}
	return float64(first) - shellAuditOutlierFence*iqr, float64(third) + shellAuditOutlierFence*iqr, true
}

// shellAuditSpreadRatio is the spread as a fraction of the middle: 0.5 means the
// standard deviation is half the median. It is the shape a screen asks about
// when it has to decide whether a startup is "steady" or "erratic", and it is
// reported as not established when either number is missing.
func (st shellAuditStats) spreadRatio() (float64, bool) {
	if !st.SpreadReported || st.Median <= 0 {
		return 0, false
	}
	return float64(st.StdDev) / float64(st.Median), true
}

// shellAuditSteadiness is how a screen words the spread: the ratio, and the word
// for it. The bands are wide on purpose. A startup that varies by a fifth from
// one run to the next is still a startup the user waits through, and calling a
// 5% wobble "erratic" would send someone to their configuration for nothing.
func (st shellAuditStats) steadiness() (string, float64, bool) {
	ratio, ok := st.spreadRatio()
	if !ok {
		return "", 0, false
	}
	switch {
	case ratio < 0.2:
		return "steady", ratio, true
	case ratio < 0.5:
		return "variable", ratio, true
	default:
		return "erratic", ratio, true
	}
}
