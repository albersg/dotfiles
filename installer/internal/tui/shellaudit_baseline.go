package tui

import (
	"fmt"
	"time"
)

// ---------------------------------------------------------------------------
// The clean comparison
// ---------------------------------------------------------------------------
//
// The same shell, the same start, the same number of runs -- once with whatever
// the user has in their startup files and once with none of it. The difference
// between the two medians is the part of the startup that the user's own
// configuration accounts for, which is the only number in this whole utility
// that says where the time went rather than what it cost.
//
// Its whole value rests on the two measurements describing the same start. Two
// numbers from different shells, different modes, different versions or different
// numbers of runs do subtract cleanly; the result just is not a finding about the
// configuration, and printing it as one is the kind of number this front exists
// to avoid. So the comparison refuses more readily than it reports, and every
// refusal names what made the two measurement incomparable.

// shellAuditComparisonFloor is the number of finished starts each side of a
// comparison needs. It is the spread's own floor rather than a new number: a
// median that is not allowed to print a spread is not a median the utility will
// subtract from another one, because the difference of two such medians is
// indistinguishable from the noise one of them could not measure.
const shellAuditComparisonFloor = shellAuditSpreadFloor

// shellAuditComparisonSmallShare is the share of the start under which the
// difference is reported as a number but not as a configuration cost. A startup
// measured on a real machine moves by a few percent between runs even when
// nothing changed, so a difference below this share is inside the movement: it is
// printed, and the verdict says the start is the shell's own.
const shellAuditComparisonSmallShare = 10.0

// shellAuditComparison is what a clean measurement says about a configured one.
// Both medians are carried beside the difference so a reader can redo the
// arithmetic, and Reason is set instead of the numbers when the two measurements
// do not describe the same start.
type shellAuditComparison struct {
	// Configured and Clean are the two medians the difference is made of.
	Configured time.Duration
	Clean      time.Duration
	// Overhead is Configured minus Clean: the time the user's own configuration
	// adds. It is negative when the clean start measured slower, which is a
	// result about the measurement rather than a saving.
	Overhead time.Duration
	// Share is Overhead as a percentage of the configured median, and it carries
	// the same sign as Overhead.
	Share float64
	// Samples is how many finished starts each side contributed: the smaller of
	// the two, because that is the number the comparison is really made of.
	Samples int
	// Reason is why no comparison can be made, in the screen's own words.
	Reason string
}

// compareShellAuditBaseline compares a configured measurement with a clean one.
// Either side may be nil, which is the case of a plan that never asked for the
// clean comparison, and both are treated as "nothing to compare" rather than as
// an error.
func compareShellAuditBaseline(configured, clean *shellAuditRun) shellAuditComparison {
	refused := func(reason string) shellAuditComparison {
		return shellAuditComparison{Reason: reason}
	}

	switch {
	case configured == nil:
		return refused("the configured start was not measured, so there is nothing to compare: " +
			"measure the startup first, and the clean comparison is made in the same pass.")
	case clean == nil:
		return refused("the clean start was not measured, so there is nothing to compare it with: " +
			"a comparison needs both starts measured under the same plan.")
	}

	switch {
	case configured.Cancelled:
		return refused("the configured measurement was cancelled, so the comparison would be drawn from " +
			"the runs that happened to finish before the stop rather than from the plan.")
	case clean.Cancelled:
		return refused("the clean measurement was cancelled, so the comparison would be drawn from " +
			"the runs that happened to finish before the stop rather than from the plan.")
	case configured.Reason != "":
		return refused(fmt.Sprintf("the configured start could not be made (%s), so there is no measurement "+
			"to compare.", configured.Reason))
	case clean.Reason != "":
		return refused(fmt.Sprintf("the clean start could not be made (%s), so there is nothing to subtract "+
			"from the measured start.", clean.Reason))
	}

	// The two measurements have to be the same start. What is allowed to differ is
	// the configuration, which is what Clean marks; everything else is a fact
	// about which start was measured.
	switch {
	case configured.Kind != clean.Kind || configured.Shell != clean.Shell || configured.Path != clean.Path:
		return refused(fmt.Sprintf("the clean measurement was of another shell (%s, not %s), so the difference "+
			"between them says nothing about your configuration: different shells start in different ways.",
			clean.Shell, configured.Shell))
	case configured.Version.Number != clean.Version.Number:
		return refused(fmt.Sprintf("the clean measurement was of another version (%s, not %s), so the difference "+
			"between them says nothing about your configuration: a shell's own startup changes between versions.",
			shellAuditVersionLabel(clean.Version), shellAuditVersionLabel(configured.Version)))
	case configured.Mode.Login != clean.Mode.Login || configured.Mode.Interactive != clean.Mode.Interactive:
		return refused(fmt.Sprintf("the clean measurement was of a different start (%s, not %s), so the two "+
			"numbers measure different work and subtracting them would be arithmetic about nothing.",
			clean.Mode.label(), configured.Mode.label()))
	}

	if len(configured.Samples) < shellAuditComparisonFloor || len(clean.Samples) < shellAuditComparisonFloor {
		return refused(fmt.Sprintf("a comparison needs at least %d finished starts on each side, and this one has "+
			"%d configured and %d clean: below that the two medians move with the machine and the difference "+
			"between them would be read as a finding about your configuration.",
			shellAuditComparisonFloor, len(configured.Samples), len(clean.Samples)))
	}

	configuredMedian := shellAuditMedianOf(configured.Samples)
	cleanMedian := shellAuditMedianOf(clean.Samples)
	overhead := configuredMedian - cleanMedian

	return shellAuditComparison{
		Configured: configuredMedian,
		Clean:      cleanMedian,
		Overhead:   overhead,
		Share:      float64(overhead) / float64(configuredMedian) * 100,
		Samples:    min(len(configured.Samples), len(clean.Samples)),
	}
}

// shellAuditVersionLabel names a version for a refusal, where an unknown one has
// to read as unknown rather than as an empty space in the sentence.
func shellAuditVersionLabel(version shellAuditVersion) string {
	if !version.Known && version.Raw == "" {
		return "a version it did not name"
	}
	if version.Raw != "" {
		return version.Raw
	}
	return version.Number
}

// headline is the line that carries the arithmetic: the difference first, then
// both medians, then how many runs each is made of, so a reader who doubts the
// number can check it against the numbers it came from.
func (c shellAuditComparison) headline() string {
	if c.Reason != "" {
		return ""
	}
	if c.Overhead <= 0 {
		// A negative difference is not printed as a configuration that costs less
		// than nothing: the two runs measured the same start twice, and the honest
		// reading is that nothing separates them.
		return fmt.Sprintf("no difference to report between the start with your configuration (%s) and the "+
			"clean one (%s), median of %d runs each.",
			shellAuditDuration(c.Configured), shellAuditDuration(c.Clean), c.Samples)
	}
	return fmt.Sprintf("%s of the start came from your configuration: %s with it, %s without, median of %d runs each.",
		shellAuditDuration(c.Overhead), shellAuditDuration(c.Configured), shellAuditDuration(c.Clean), c.Samples)
}

// verdict is the sentence that says what the difference means, and it is kept
// apart from the arithmetic on purpose: the same number means "this is where your
// startup time goes" at sixty percent and "this is the shell's own startup" at
// two, and only one of those two readings is worth acting on.
func (c shellAuditComparison) verdict() string {
	switch {
	case c.Reason != "":
		return ""
	case c.Overhead <= 0:
		return fmt.Sprintf("The clean start measured %s slower than the configured one, a difference of %.1f%%, "+
			"so the two starts are not separable here: a shell's own startup moves by that much between runs, and "+
			"nothing in this comparison says your configuration costs or saves anything.",
			shellAuditDuration(-c.Overhead), -c.Share)
	case c.Share < shellAuditComparisonSmallShare:
		return fmt.Sprintf("Almost all of this start is the shell itself: your configuration accounts for %.1f%% of it, "+
			"which is inside the movement a startup has between runs, so there is nothing here to change.", c.Share)
	default:
		return fmt.Sprintf("Your configuration accounts for %.1f%% of this start -- %s of the %s median. "+
			"That is the part of the startup the utility measured as being yours rather than the shell's, and it is "+
			"where the profile below is worth reading.", c.Share, shellAuditDuration(c.Overhead),
			shellAuditDuration(c.Configured))
	}
}
