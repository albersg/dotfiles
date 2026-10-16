package tui

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

// ============================================================================
// WHAT THE LAST RUNS SAID
// ============================================================================
//
// One measurement answers "how slow is this shell". The question the user asks
// next is "was it always like this, and did something I changed do it", and no
// single run can answer it: the shell itself changes between versions, the
// machine changes under it, and the user's configuration changes because they
// edited it.
//
// The stored measurements answer it as far as measurements can. A trend is a
// comparison of medians of the same start, made the same way, and it says a
// change happened between two dates -- never that something caused it. The advice
// below is written from the same three kinds of evidence the single measurement
// uses (what was measured, what the shell's own instrument named, and what the
// clean comparison separated), and every item says which of the three it is.
//
// The file is the only thing in this utility that outlives the session, so it
// holds numbers and the names the instrument already redacted: a duration, a
// count, a version, the key of the start it was measured in, and at most the
// heaviest few rows of the profile. Nothing captured from a start is stored, and
// the switch is the user's: with history off, nothing here is written at all.

const (
	// shellAuditHistoryFileName is the record's name inside the state directory,
	// beside the install record. Both are state a program keeps between runs, so
	// both live where the XDG specification puts that.
	shellAuditHistoryFileName = "shell-audit-history.json"

	// shellAuditHistoryLimit bounds the file. A user who opens this screen every
	// day has a year of measurements in mind; the trend only ever reads the last
	// few of one start, so an unbounded file would grow to hold what nobody reads.
	shellAuditHistoryLimit = 50

	// shellAuditHistoryRows is how many profile rows a stored measurement keeps.
	// It is the number the screen names, so a stored measurement can still show
	// which entry the instrument put first without keeping a table.
	shellAuditHistoryRows = 3

	// shellAuditHistoryReasonLimit cuts the prose an instrument's absence is
	// explained with. A reason is a sentence, and a file that grows a paragraph
	// per measurement is a file that changes shape with the wording.
	shellAuditHistoryReasonLimit = 200

	// shellAuditTrendFloor is how many stored measurements of one start a trend
	// needs before it says anything. The floor is the same shape as the spread's:
	// below it, the thing being described is the machine's afternoon rather than
	// the shell's startup.
	shellAuditTrendFloor = 3

	// shellAuditTrendQuiet is the share under which a move between measurements is
	// reported as steady. A startup varies by a few percent between runs on a
	// machine that is doing other things, so a smaller move is not a change; the
	// number is printed either way, and the wording is what it decides.
	shellAuditTrendQuiet = 10.0

	// shellAuditHistoryTempSuffix is what the write goes through before it is
	// rolled onto the file, so a process that dies mid-write leaves the previous
	// history rather than half of a new one.
	shellAuditHistoryTempSuffix = ".tmp"
)

// shellAuditHistoryRow is one stored profile row: the name the instrument gave it
// and the time it measured. The share is not stored, because it is the
// instrument's own cell for the run it came from and every run prints a different
// one; the time is the number every run can be read for.
type shellAuditHistoryRow struct {
	Name string        `json:"name"`
	Kind string        `json:"kind"`
	Self time.Duration `json:"self"`
}

// shellAuditHistoryEntry is one measurement as the file keeps it. It is the whole
// of what leaves the machine's memory, which is why it is a type of its own
// rather than the record: the record holds the shell's path, the terminal, the
// error text and the full profile, and none of that is a thing to write to disk.
type shellAuditHistoryEntry struct {
	At      time.Time `json:"at"`
	Shell   string    `json:"shell"`
	Kind    string    `json:"kind"`
	Version string    `json:"version"`
	Mode    string    `json:"mode"`
	Samples int       `json:"samples"`
	// Median, Fastest and Slowest are the summary of the runs that finished.
	Median  time.Duration `json:"median"`
	Fastest time.Duration `json:"fastest"`
	Slowest time.Duration `json:"slowest"`
	// Timeouts and Failures are the starts that produced no duration, kept apart
	// from the ones that did so a stored median can never be read as "the shell
	// always takes this long".
	Timeouts int `json:"timeouts,omitempty"`
	Failures int `json:"failures,omitempty"`
	// Cancelled marks a measurement the user stopped. It is stored -- it happened
	// -- and it is never used as a point of comparison, because its median is
	// drawn from the runs that finished rather than from the plan.
	Cancelled bool `json:"cancelled,omitempty"`
	// Clean and Overhead are the clean comparison, when one was made.
	Clean    time.Duration `json:"clean,omitempty"`
	Overhead time.Duration `json:"overhead,omitempty"`
	// Heaviest is the top of the instrument's own table, and Profile is why it
	// named nothing when it did not.
	Heaviest []shellAuditHistoryRow `json:"heaviest,omitempty"`
	Profile  string                 `json:"profile,omitempty"`
}

// shellAuditHistory is the stored file: the user's switch and the measurements,
// newest first.
type shellAuditHistory struct {
	Enabled bool                     `json:"enabled"`
	Entries []shellAuditHistoryEntry `json:"entries"`
}

// shellAuditTrendKind is what a measurement means against the stored ones.
type shellAuditTrendKind string

const (
	// shellAuditTrendFirst is a start this history has too few stored runs of to
	// say anything about, which includes every start measured for the first time.
	shellAuditTrendFirst shellAuditTrendKind = "first"
	// shellAuditTrendSteady is a move inside the movement a startup has between
	// runs. The number is reported and no conclusion is drawn from it.
	shellAuditTrendSteady shellAuditTrendKind = "steady"
	// shellAuditTrendRegression is a start that became slower by more than that
	// movement, and shellAuditTrendImprovement one that became faster.
	shellAuditTrendRegression  shellAuditTrendKind = "regression"
	shellAuditTrendImprovement shellAuditTrendKind = "improvement"
	// shellAuditTrendIncomparable is a history that holds runs of this start but
	// of another version of the shell, so the two medians are not about the same
	// program and subtracting them would be arithmetic about nothing.
	shellAuditTrendIncomparable shellAuditTrendKind = "incomparable"
)

// shellAuditTrend is where a measurement sits among the stored ones. Before is
// the median of the stored medians rather than their mean: one measurement taken
// while the machine was busy would drag a mean, and a median of medians is what
// "this start usually costs" means.
type shellAuditTrend struct {
	Kind   shellAuditTrendKind
	Before time.Duration
	After  time.Duration
	Delta  time.Duration
	Share  float64
	Runs   int
	// Since is when the oldest stored run the comparison uses was measured.
	Since time.Time
	// Reason is why there is no comparison, in the screen's own words.
	Reason string
}

// shellAuditAdviceKind is what an advice item is allowed to claim. The three
// kinds are the three kinds of evidence this utility has, and the distinction is
// the point: a profile names where the time went, which is not the same thing as
// why it went there, and only the clean comparison separates the user's own
// configuration from the shell's startup.
type shellAuditAdviceKind string

const (
	// shellAuditAdviceMeasured is a fact about the measurement itself: the spread,
	// the runs that did not finish, the limit of what could be profiled at all.
	shellAuditAdviceMeasured shellAuditAdviceKind = "measured"
	// shellAuditAdviceObserved is what the shell's own instrument named, in its
	// own order. It is a correlation with the cost, never a proven cause.
	shellAuditAdviceObserved shellAuditAdviceKind = "observed"
	// shellAuditAdviceConfirmed is the one thing the utility can separate: the
	// part of the start that the clean comparison shows the configuration
	// accounts for, as a whole -- never inside it.
	shellAuditAdviceConfirmed shellAuditAdviceKind = "confirmed"
)

// shellAuditAdvice is one item the screen's recommendations draw: the headline,
// what to do about it, and the measurement it came from. Evidence is required --
// an item whose evidence is empty is advice with nothing behind it, and that is
// the one thing this section must never print.
type shellAuditAdvice struct {
	Kind     shellAuditAdviceKind
	Headline string
	Detail   string
	Evidence string
}

// shellAuditReport is one measurement read as a whole: the record, the clean
// comparison, where the measurement sits among the stored ones and what the
// history holds. It is derived from the state and never stored, so the screen and
// the advice can never disagree with the numbers beside them.
type shellAuditReport struct {
	// Measured reports whether anything was measured: without it there is no
	// record to read, and every other field is empty rather than zero.
	Measured   bool
	Run        shellAuditRun
	Baseline   *shellAuditRun
	Comparison shellAuditComparison
	Trend      shellAuditTrend
	History    shellAuditHistory
}

// shellAuditReportOf reads a state as a report.
func shellAuditReportOf(st shellAuditState) shellAuditReport {
	if !st.Measured {
		return shellAuditReport{
			Comparison: compareShellAuditBaseline(nil, nil),
			Trend: shellAuditTrend{
				Kind: shellAuditTrendFirst,
				Reason: "nothing has been measured here yet, so there is nothing to compare: " +
					"measure the shell first, and the stored measurements are what the next one is read against.",
			},
			History: st.History,
		}
	}

	run := st.shellAuditRun()
	comparison := compareShellAuditBaseline(&run, st.Baseline)
	return shellAuditReport{
		Measured:   true,
		Run:        run,
		Baseline:   st.Baseline,
		Comparison: comparison,
		Trend:      st.History.trend(shellAuditHistoryEntryOf(run, comparison)),
		History:    st.History,
	}
}

// shellAuditHistoryEntryOf reduces a measurement to what is stored. The
// comparison is passed in rather than recomputed so the stored numbers and the
// numbers on screen are the same read of the same record.
func shellAuditHistoryEntryOf(run shellAuditRun, comparison shellAuditComparison) shellAuditHistoryEntry {
	entry := shellAuditHistoryEntry{
		At:        run.MeasuredAt,
		Shell:     run.Shell,
		Kind:      string(run.Kind),
		Version:   run.Version.Number,
		Mode:      run.Mode.key(),
		Samples:   len(run.Samples),
		Median:    run.Stats.Median,
		Fastest:   run.Stats.Fastest,
		Slowest:   run.Stats.Slowest,
		Timeouts:  run.Timeouts,
		Failures:  run.Failures,
		Cancelled: run.Cancelled,
		Profile:   shellAuditTruncate(run.AttributionReason, shellAuditHistoryReasonLimit),
	}

	if comparison.Reason == "" {
		entry.Clean = comparison.Clean
		entry.Overhead = comparison.Overhead
	}

	for _, row := range run.Attribution {
		if len(entry.Heaviest) == shellAuditHistoryRows {
			break
		}
		entry.Heaviest = append(entry.Heaviest, shellAuditHistoryRow{
			Name: row.Name,
			Kind: string(row.Kind),
			Self: row.Self,
		})
	}

	return entry
}

// add stores one measurement, newest first, and bounds the file. The same
// measurement landing twice -- a screen that reads its own result again, a
// scheduled run that repeats -- is one measurement and is ignored rather than
// counted, and a measurement that arrives older than the newest takes the place
// its stamp gives it rather than being refused.
func (h shellAuditHistory) add(entry shellAuditHistoryEntry) shellAuditHistory {
	entries := make([]shellAuditHistoryEntry, 0, len(h.Entries)+1)
	placed := false

	for _, stored := range h.Entries {
		if stored.At.Equal(entry.At) {
			return h
		}
		if !placed && stored.At.Before(entry.At) {
			entries = append(entries, entry)
			placed = true
		}
		entries = append(entries, stored)
	}
	if !placed {
		entries = append(entries, entry)
	}

	if len(entries) > shellAuditHistoryLimit {
		entries = entries[:shellAuditHistoryLimit]
	}
	h.Entries = entries
	return h
}

// trend says where a measurement sits among the stored ones.
//
// The runs it compares are the stored runs of the same start of the same shell
// version. Anything else is not a point of comparison: another start measures
// other work, another version measures another program, and a cancelled run's
// median comes from the runs that finished rather than from the plan. A stored
// run of another version is not silently dropped either -- it is the reason there
// is no trend, and the reason says so.
func (h shellAuditHistory) trend(entry shellAuditHistoryEntry) shellAuditTrend {
	current := shellAuditTrend{After: entry.Median}

	switch {
	case entry.Cancelled:
		current.Kind = shellAuditTrendFirst
		current.Reason = "this measurement was cancelled, so its median is drawn from the runs that finished " +
			"rather than from the plan: it is stored, and it is not used to compare with."
		return current
	case entry.Samples == 0 || entry.Median <= 0:
		current.Kind = shellAuditTrendFirst
		current.Reason = "this measurement produced no finished run to compare with, so there is no trend to read " +
			"from it: what it did produce is the reason beside the measurement."
		return current
	}

	var same, otherVersion []shellAuditHistoryEntry
	for _, stored := range h.Entries {
		if stored.At.Equal(entry.At) || stored.Shell != entry.Shell || stored.Mode != entry.Mode {
			continue
		}
		if stored.Version != entry.Version {
			otherVersion = append(otherVersion, stored)
			continue
		}
		if stored.Cancelled || stored.Samples == 0 || stored.Median <= 0 {
			continue
		}
		same = append(same, stored)
	}

	if len(same) < shellAuditTrendFloor {
		if len(otherVersion) > 0 {
			current.Kind = shellAuditTrendIncomparable
			current.Reason = fmt.Sprintf("the %d stored measurements of this start were made with %s %s and this one "+
				"with %s: a median from another version of the shell is not a point to compare with, so there is no "+
				"trend until this version has %d measurements of its own.",
				len(otherVersion), entry.Shell, otherVersion[0].Version, entry.Version, shellAuditTrendFloor)
			return current
		}
		current.Kind = shellAuditTrendFirst
		if len(same) == 0 {
			current.Reason = fmt.Sprintf("this is the first stored measurement of this start: a trend needs at least "+
				"%d of them, and this one has none.", shellAuditTrendFloor)
			return current
		}
		current.Reason = fmt.Sprintf("a trend of this start needs at least %d stored measurements and there %s only %d.",
			shellAuditTrendFloor, map[bool]string{true: "is", false: "are"}[len(same) == 1], len(same))
		return current
	}

	medians := make([]time.Duration, 0, len(same))
	since := same[0].At
	for _, stored := range same {
		medians = append(medians, stored.Median)
		if stored.At.Before(since) {
			since = stored.At
		}
	}

	before := shellAuditMedianOf(medians)
	current.Before = before
	current.Delta = entry.Median - before
	current.Share = float64(current.Delta) / float64(before) * 100
	current.Runs = len(same)
	current.Since = since

	switch {
	case math.Abs(current.Share) < shellAuditTrendQuiet:
		current.Kind = shellAuditTrendSteady
	case current.Share > 0:
		current.Kind = shellAuditTrendRegression
	default:
		current.Kind = shellAuditTrendImprovement
	}

	return current
}

// sentence is the one line the trend prints, and it is empty whenever there is
// nothing to compare -- the reason is then the thing to read, and a screen that
// printed a change of zero would be reporting a measurement it does not have.
func (t shellAuditTrend) sentence() string {
	switch t.Kind {
	case shellAuditTrendSteady:
		return fmt.Sprintf("This start is steady: the median moved from %s to %s (%+.0f%%) against the %d stored "+
			"measurements of it, which is inside the movement a startup has between runs.",
			shellAuditDuration(t.Before), shellAuditDuration(t.After), t.Share, t.Runs)
	case shellAuditTrendRegression:
		return fmt.Sprintf("This start got slower: the median moved from %s to %s (+%.0f%%) against the %d stored "+
			"measurements of it, the oldest from %s.",
			shellAuditDuration(t.Before), shellAuditDuration(t.After), t.Share, t.Runs, t.Since.Format(shellAuditTrendStamp))
	case shellAuditTrendImprovement:
		return fmt.Sprintf("This start got faster: the median moved from %s to %s (%.0f%%) against the %d stored "+
			"measurements of it, the oldest from %s.",
			shellAuditDuration(t.Before), shellAuditDuration(t.After), t.Share, t.Runs, t.Since.Format(shellAuditTrendStamp))
	default:
		return ""
	}
}

// shellAuditTrendStamp is how a stored measurement's date is printed: the day it
// was measured, because the trend is about days and not about seconds.
const shellAuditTrendStamp = "2 January 2006"

// advice writes the recommendations the screen shows, in the order they are worth
// reading: what limits the measurement first, then where the time went, then the
// one part of it the utility can separate from the shell, then what changed since
// the stored runs.
//
// Every item carries the measurement it came from, and no item claims a cause
// that was not measured. That is the whole discipline of this section: the
// profile is a correlation with the cost, the clean comparison is a confirmed
// attribution to the configuration as a whole, and the history is a change
// between two dates about which nothing here knows what happened.
func (r shellAuditReport) advice() []shellAuditAdvice {
	if !r.Measured {
		return nil
	}

	run := r.Run
	planned := len(run.Samples) + run.Timeouts + run.Failures
	var advice []shellAuditAdvice

	if run.Timeouts > 0 {
		advice = append(advice, shellAuditAdvice{
			Kind: shellAuditAdviceMeasured,
			Headline: fmt.Sprintf("%d of the %d starts ended at the %s bound without finishing.",
				run.Timeouts, planned, shellAuditDuration(run.Timeout)),
			Detail: "A startup that reaches the bound is not a slow startup: it is one waiting for something that " +
				"never answered. The median above is made of the runs that did finish, so the real cost of this " +
				"shell is at least that number and possibly the bound.",
			Evidence: fmt.Sprintf("%d of %d planned starts produced no duration at all.", run.Timeouts, planned),
		})
	}

	if run.Failures > 0 {
		last := run.Failure
		if last == "" {
			last = "the shell gave no message"
		}
		advice = append(advice, shellAuditAdvice{
			Kind:     shellAuditAdviceMeasured,
			Headline: fmt.Sprintf("%d of the %d starts could not be started at all.", run.Failures, planned),
			Detail: "Those starts say nothing about how slow the shell is, and a shell that cannot always be " +
				"started is worth reading about before its speed is: the message is above the median, not in it.",
			Evidence: fmt.Sprintf("the last one said: %s", last),
		})
	}

	// The spread comes before anything about what is inside the startup. Advice
	// about a named function is worth little when the number it rests on moves by
	// half its own size between runs.
	if word, ratio, ok := run.Stats.steadiness(); ok && word != "steady" {
		advice = append(advice, shellAuditAdvice{
			Kind: shellAuditAdviceMeasured,
			Headline: fmt.Sprintf("This start is %s across the %d runs that finished: %s to %s, median %s.",
				word, run.Stats.Count, shellAuditDuration(run.Stats.Fastest),
				shellAuditDuration(run.Stats.Slowest), shellAuditDuration(run.Stats.Median)),
			Detail: "A startup that moves this much is measured by its range rather than by its middle: run it " +
				"again with more samples, and read the profile below knowing that a part of what it names is the " +
				"spread rather than the cost.",
			Evidence: fmt.Sprintf("%d finished starts, fastest %s, slowest %s, the standard deviation is %.0f%% of "+
				"the median.", run.Stats.Count, shellAuditDuration(run.Stats.Fastest),
				shellAuditDuration(run.Stats.Slowest), ratio*100),
		})
	}

	if r.Comparison.Reason == "" && r.Comparison.Overhead > 0 && r.Comparison.Share >= shellAuditComparisonSmallShare {
		advice = append(advice, shellAuditAdvice{
			Kind: shellAuditAdviceConfirmed,
			Headline: fmt.Sprintf("Your configuration accounts for %.0f%% of this start: %s of %s.",
				r.Comparison.Share, shellAuditDuration(r.Comparison.Overhead),
				shellAuditDuration(r.Comparison.Configured)),
			Detail: "This is the only part of the startup the utility can separate from the shell's own: the " +
				"profile below says what inside the configuration costs the time, and this number says how much of " +
				"the start is worth reading about at all.",
			Evidence: fmt.Sprintf("the same start measured clean: %s with the configuration and %s without, median "+
				"of %d runs each.", shellAuditDuration(r.Comparison.Configured),
				shellAuditDuration(r.Comparison.Clean), r.Comparison.Samples),
		})
	}

	if len(run.Attribution) > 0 {
		heaviest := run.Attribution[0]
		// The evidence names the same rows the list above names and no more. A screen
		// that said "1 more were profiled and are not named here" in one paragraph and
		// then named that row in the next would be reading two claims against each
		// other, and the reader has no way to tell which one the instrument supports.
		rest, unnamed := shellAuditNamedRows(run.Attribution[1:])
		advice = append(advice, shellAuditAdvice{
			Kind: shellAuditAdviceObserved,
			Headline: fmt.Sprintf("%s is the heaviest entry in this start: %s.", heaviest.Name,
				shellAuditRowCost(heaviest)),
			Detail: "This is where the time went, which is not the same as why: the instrument named a function " +
				"or a command, and what is inside it is what to read next. Nothing was changed for you, and " +
				"anything you change in your startup files is yours to review.",
			Evidence: fmt.Sprintf("%s's own profile: %s first, then %s.%s The names and the times are the "+
				"instrument's own, read from the table it writes itself.",
				run.Shell, heaviest.Name, shellAuditNamedList(rest), shellAuditUnnamedCount(unnamed)),
		})
	} else if run.AttributionReason != "" {
		advice = append(advice, shellAuditAdvice{
			Kind:     shellAuditAdviceMeasured,
			Headline: "This shell's startup cannot be broken down here.",
			Detail: "The total is still measured, and it is still comparable with the clean start and with the " +
				"stored measurements. What the utility cannot do for this shell is name what inside the startup " +
				"costs the time, and it says so rather than naming something it did not read.",
			Evidence: run.AttributionReason,
		})
	}

	if sentence := r.Trend.sentence(); sentence != "" {
		advice = append(advice, shellAuditAdvice{
			Kind:     shellAuditAdviceMeasured,
			Headline: fmt.Sprintf("Against the %d stored measurements of this start: %s", r.Trend.Runs, sentence),
			Detail: "The stored runs say the start changed between them and this one; they cannot say what changed " +
				"in between: nothing here watches what you edit, so read this beside what you changed rather than " +
				"as its explanation.",
			Evidence: fmt.Sprintf("%d stored measurements of this start, from %s to %s; their medians are %s.",
				r.Trend.Runs, r.Trend.Since.Format(shellAuditTrendStamp),
				r.History.newest().Format(shellAuditTrendStamp), shellAuditDurationList(r.trendMedians())),
		})
	}

	return advice
}

// trendMedians is the stored medians the trend compared with, for the evidence
// line: the numbers themselves, so a reader can see the change rather than take
// it on trust.
func (r shellAuditReport) trendMedians() []time.Duration {
	entry := shellAuditHistoryEntryOf(r.Run, r.Comparison)
	var medians []time.Duration
	for _, stored := range r.History.Entries {
		if stored.At.Equal(entry.At) || stored.Shell != entry.Shell || stored.Mode != entry.Mode {
			continue
		}
		if stored.Version != entry.Version || stored.Cancelled || stored.Samples == 0 || stored.Median <= 0 {
			continue
		}
		medians = append(medians, stored.Median)
	}
	sort.Slice(medians, func(i, j int) bool { return medians[i] < medians[j] })
	return medians
}

// newest is when the most recent stored measurement was made, and the zero time
// when nothing is stored.
func (h shellAuditHistory) newest() time.Time {
	if len(h.Entries) == 0 {
		return time.Time{}
	}
	return h.Entries[0].At
}

// shellAuditRowCost renders one instrument row: what it measured and how often it
// was called. The share is printed only when the instrument gave one, because a
// share this utility computed itself would be a number from a denominator the
// instrument never stated.
func shellAuditRowCost(row shellAuditFunction) string {
	cost := fmt.Sprintf("%s self", shellAuditDuration(row.Self))
	if row.Calls > 0 {
		cost += fmt.Sprintf(" over %d call(s)", row.Calls)
	}
	if row.Percent != "" {
		cost += fmt.Sprintf(", %s of the table as the instrument measured it", row.Percent)
	}
	return cost
}

// shellAuditNamedRows cuts the instrument's rows to the number the screen names
// and says how many it left out. The list in the profile paragraph and the
// evidence under the recommendation are the same cut, so the two cannot disagree
// about what was named and what the instrument holds unread.
func shellAuditNamedRows(rows []shellAuditFunction) (named []shellAuditFunction, unnamed int) {
	named = rows
	limit := shellAuditNamedFunctions - 1
	if limit < 0 {
		limit = 0
	}
	if len(named) > limit {
		unnamed = len(named) - limit
		named = named[:limit]
	}
	return named, unnamed
}

// shellAuditUnnamedCount says how many rows the cut left out, in the same words
// the profile paragraph uses, so a reader who counts the names in both places
// reads one cut and not two.
func shellAuditUnnamedCount(unnamed int) string {
	if unnamed <= 0 {
		return ""
	}
	return fmt.Sprintf(" %d more were profiled and are not named here.", unnamed)
}

// shellAuditNamedList names the rest of the instrument's rows for an evidence
// line, and says so when there were none rather than leaving a dangling "then".
func shellAuditNamedList(rows []shellAuditFunction) string {
	if len(rows) == 0 {
		return "nothing else worth naming"
	}
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		names = append(names, fmt.Sprintf("%s (%s)", row.Name, shellAuditDuration(row.Self)))
	}
	return strings.Join(names, ", ")
}

// shellAuditDurationList renders a list of durations for an evidence line.
func shellAuditDurationList(values []time.Duration) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, shellAuditDuration(value))
	}
	return strings.Join(parts, ", ")
}

// ---------------------------------------------------------------------------
// The file
// ---------------------------------------------------------------------------

// shellAuditHistoryPath is the file the measurements are stored in. It is the
// install record's own state directory: $XDG_STATE_HOME/dotfiles, falling back to
// ~/.local/state/dotfiles, which is the directory the XDG base-directory
// specification reserves for state a program keeps between runs. An empty string
// means neither could be determined, and then nothing is read and nothing written.
func shellAuditHistoryPath(getenv func(string) string) string {
	dir := ""
	if xdg := getenv("XDG_STATE_HOME"); xdg != "" {
		dir = filepath.Join(xdg, stateAppDir)
	} else if home := getenv("HOME"); home != "" {
		dir = filepath.Join(home, ".local", "state", stateAppDir)
	}
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, shellAuditHistoryFileName)
}

// readShellAuditHistory reads the stored measurements. A machine with no file is
// a machine that has never kept history, which is not a failure; a file that is
// there and cannot be read is reported, because the user is about to lose the
// trend they turned on and should be told rather than silently started over.
func readShellAuditHistory(probe shellAuditProbe, path string) (shellAuditHistory, error) {
	if probe.readFile == nil {
		return shellAuditHistory{}, nil
	}
	data, err := probe.readFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return shellAuditHistory{}, nil
		}
		return shellAuditHistory{}, err
	}

	var history shellAuditHistory
	if err := json.Unmarshal(data, &history); err != nil {
		return shellAuditHistory{}, err
	}
	return history, nil
}

// writeShellAuditHistory writes the measurements through a temporary file of the
// same directory, which is then rolled onto the real one: a process that dies
// mid-write leaves the history it had rather than half of a new one, and the
// rename is what the filesystem guarantees to be whole.
//
// The temporary name is unique per write. The writes are started from the update
// loop, and two of them can be in flight at once -- the switch being turned on
// while a measurement lands, two presses a moment apart -- so a name two writers
// shared would let them interleave their bytes into the one file that is about to
// become the history. The rename stays last, so the file that readers see is
// always one whole history: whichever write lands last.
func writeShellAuditHistory(probe shellAuditProbe, path string, history shellAuditHistory) error {
	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return err
	}
	if err := probe.mkdirAll(filepath.Dir(path)); err != nil {
		return err
	}
	temp := shellAuditHistoryTempPath(path)
	if err := probe.writeFile(temp, data); err != nil {
		return err
	}
	if err := probe.rename(temp, path); err != nil {
		_ = probe.removeAll(temp)
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}

// shellAuditHistoryWrites counts the writes this process has started, and with the
// process id it is what makes each temporary name its own. A counter rather than a
// random name because the file has to be readable in a test without a seam for
// chance, and the process id beside it because two installers can be open at once
// and both write the same history.
var shellAuditHistoryWrites atomic.Uint64

// shellAuditHistoryTempPath is the name one write uses until it lands.
func shellAuditHistoryTempPath(path string) string {
	return fmt.Sprintf("%s.%d-%d%s", path, os.Getpid(), shellAuditHistoryWrites.Add(1), shellAuditHistoryTempSuffix)
}

// saveShellAuditHistory stores the measurements this state holds, and does
// nothing at all when the user has not turned history on. Its error is reported
// rather than ignored: a user who asked for history and is getting none should
// hear about the disk, and a measurement that was taken is not made worse by it.
func saveShellAuditHistory(st shellAuditState, probe shellAuditProbe) error {
	if !st.History.Enabled {
		return nil
	}
	path := shellAuditHistoryPath(probe.getenv)
	if path == "" {
		return errors.New("the state directory could not be determined, so the measurements were not stored")
	}
	return writeShellAuditHistory(probe, path, st.History)
}

// defaultShellAuditHistoryPath is the file the running utility reads and writes.
// It is a function rather than a constant so the path is derived from the
// environment once, where the environment is the real one.
func defaultShellAuditHistoryPath() string {
	return shellAuditHistoryPath(os.Getenv)
}
