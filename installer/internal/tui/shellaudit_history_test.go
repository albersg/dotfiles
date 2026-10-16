package tui

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// The history: what the last runs said, and what a change in it means
// ---------------------------------------------------------------------------
//
// One measurement answers "how slow is this shell". The stored ones answer the
// question the user actually asks next -- "was it always like this, and did
// something I changed do it" -- and they answer it from the same kind of evidence
// the single measurement uses: medians, spreads, counts and the instrument's own
// names.
//
// The guard that carries the most weight is the one keeping this file thin. The
// history is a file on disk that outlives the session, so what goes into it is
// numbers and the names the shell's own instrument already redacted -- never the
// captured output, never a command line, never an environment variable.

// historyEntryFixture is one stored measurement of the interactive zsh start.
func historyEntryFixture(at time.Time, median time.Duration) shellAuditHistoryEntry {
	return shellAuditHistoryEntry{
		At:      at,
		Shell:   "zsh",
		Kind:    string(shellAuditKindZsh),
		Version: "5.9.2",
		Mode:    shellAuditModeInteractive.key(),
		Samples: 3,
		Median:  median,
		Fastest: median - 10*time.Millisecond,
		Slowest: median + 10*time.Millisecond,
	}
}

// TestShellAuditHistoryKeepsTheNumbersAndTheNamesOnly pins what is stored, field
// by field, as the set of keys that reaches the file. A field added to the entry
// fails this guard on purpose: the history is written to the user's disk and read
// back by later runs, so what it holds is a decision rather than an accident of
// whatever the record happened to carry.
func TestShellAuditHistoryKeepsTheNumbersAndTheNamesOnly(t *testing.T) {
	run := shellAuditRun{
		Kind:              shellAuditKindZsh,
		Shell:             "zsh",
		Path:              "/usr/bin/zsh",
		Version:           shellAuditVersion{Known: true, Number: "5.9.2", Major: 5, Minor: 9},
		Mode:              shellAuditModeInteractive,
		Command:           "zsh -i -c exit",
		MeasuredAt:        time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC),
		Samples:           []time.Duration{400 * time.Millisecond, 410 * time.Millisecond, 420 * time.Millisecond},
		Stats:             shellAuditStatsOf([]time.Duration{400 * time.Millisecond, 410 * time.Millisecond, 420 * time.Millisecond}),
		Timeouts:          1,
		Failures:          2,
		Failure:           "exit status 127",
		AttributionReason: strings.Repeat("a reason about the instrument ", 20),
		Attribution: []shellAuditFunction{
			{Name: "compinit", Kind: shellAuditAttributionFunction, Self: 300 * time.Millisecond, Calls: 1, Percent: "75.00%"},
			{Name: "source /home/alberto/.config/zsh/plugins.zsh", Kind: shellAuditAttributionScript, Self: 60 * time.Millisecond, Calls: 1, Percent: "15.00%"},
			{Name: "zoxide", Kind: shellAuditAttributionCommand, Self: 20 * time.Millisecond, Calls: 2, Percent: "5.00%"},
			{Name: "starship", Kind: shellAuditAttributionCommand, Self: 10 * time.Millisecond, Calls: 1, Percent: "2.50%"},
		},
	}
	clean := shellAuditRun{
		Kind:    shellAuditKindZsh,
		Shell:   "zsh",
		Path:    "/usr/bin/zsh",
		Version: shellAuditVersion{Known: true, Number: "5.9.2", Major: 5, Minor: 9},
		Mode:    shellAuditMode{Interactive: true, Clean: true},
		Samples: []time.Duration{140 * time.Millisecond, 150 * time.Millisecond, 160 * time.Millisecond},
	}
	clean.Stats = shellAuditStatsOf(clean.Samples)

	entry := shellAuditHistoryEntryOf(run, compareShellAuditBaseline(&run, &clean))

	if entry.At != run.MeasuredAt {
		t.Errorf("the entry is stamped %v, want the measurement's own %v", entry.At, run.MeasuredAt)
	}
	if entry.Shell != "zsh" || entry.Kind != string(shellAuditKindZsh) || entry.Version != "5.9.2" {
		t.Errorf("the entry names %q %q %q, want the shell, family and version", entry.Shell, entry.Kind, entry.Version)
	}
	if entry.Mode != shellAuditModeInteractive.key() {
		t.Errorf("the entry's start = %q, want the interactive start", entry.Mode)
	}
	if entry.Samples != 3 || entry.Median != 410*time.Millisecond {
		t.Errorf("the entry = %d runs, median %v, want 3 runs and 410 ms", entry.Samples, entry.Median)
	}
	if entry.Fastest != 400*time.Millisecond || entry.Slowest != 420*time.Millisecond {
		t.Errorf("the entry's range = %v..%v, want 400 ms..420 ms", entry.Fastest, entry.Slowest)
	}
	if entry.Timeouts != 1 || entry.Failures != 2 {
		t.Errorf("the entry = %d timeouts, %d failures, want the counted ones", entry.Timeouts, entry.Failures)
	}
	if entry.Clean != 150*time.Millisecond || entry.Overhead != 260*time.Millisecond {
		t.Errorf("the entry's clean comparison = %v clean, %v overhead, want 150 ms and 260 ms", entry.Clean, entry.Overhead)
	}

	// The profile is kept as at most the rows the screen names, and only their
	// name, kind and time: the share is the instrument's own cell for the run it
	// came from, and the history stores numbers.
	if len(entry.Heaviest) != shellAuditHistoryRows {
		t.Fatalf("%d stored profile rows, want the heaviest %d", len(entry.Heaviest), shellAuditHistoryRows)
	}
	if entry.Heaviest[0].Name != "compinit" || entry.Heaviest[0].Self != 300*time.Millisecond {
		t.Errorf("the heaviest stored row = %+v, want compinit at 300 ms", entry.Heaviest[0])
	}
	if entry.Heaviest[2].Name != "zoxide" {
		t.Errorf("the third stored row = %q, want the third heaviest", entry.Heaviest[2].Name)
	}

	// The reason a shell named nothing is prose and is cut to a length that stays
	// a sentence in a file, rather than a paragraph growing with every version of
	// the wording.
	// The reason is cut to the limit and the cut is then marked with the ellipsis
	// shellAuditTruncate adds, so the stored value is the limit plus that mark.
	if len(entry.Profile) > shellAuditHistoryReasonLimit+3 {
		t.Errorf("the stored reason is %d bytes, want at most %d plus the ellipsis",
			len(entry.Profile), shellAuditHistoryReasonLimit)
	}
	if !strings.HasSuffix(entry.Profile, "…") {
		t.Errorf("the stored reason does not say it was cut: %q", entry.Profile)
	}

	// What is stored is the set of keys this guard lists. A new field is a new
	// decision about what leaves the machine's memory, and it has to be made here.
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("the entry does not marshal: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatalf("the entry does not parse back: %v", err)
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	want := []string{"at", "clean", "fastest", "heaviest", "kind", "median", "mode", "profile", "shell", "slowest",
		"samples", "timeouts", "failures", "overhead", "version"}
	sort.Strings(want)
	if strings.Join(keys, ",") != strings.Join(want, ",") {
		t.Errorf("the stored fields are %v, want %v", keys, want)
	}
	if strings.Contains(string(data), "/usr/bin/zsh") || strings.Contains(string(data), "exit status") {
		t.Errorf("the entry carries more than numbers and names: %s", data)
	}
}

// TestShellAuditHistoryKeepsTheNewestFirstAndStaysBounded pins the shape of the
// file: newest first, and bounded, so a utility that is run every day for a year
// does not grow a file nobody reads past the first screen of it.
func TestShellAuditHistoryKeepsTheNewestFirstAndStaysBounded(t *testing.T) {
	began := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	history := shellAuditHistory{Enabled: true}
	for i := 0; i < 3; i++ {
		history = history.add(historyEntryFixture(began.Add(time.Duration(i)*time.Hour), 200*time.Millisecond))
	}

	// A measurement that arrives with a stamp older than the newest takes the
	// place the stamp gives it: the history is a sequence of measurements, and one
	// of them being written late does not make it the newest.
	late := historyEntryFixture(began.Add(30*time.Minute), 210*time.Millisecond)
	history = history.add(late)
	// Newest first, so the measurement from between the two others belongs third:
	// the file is ordered by when a measurement was made, not by when it was
	// written.
	if len(history.Entries) != 4 || !history.Entries[2].At.Equal(late.At) {
		t.Errorf("a measurement from between two stored ones landed at %v", history.Entries[2].At)
	}
	if !history.Entries[0].At.Equal(began.Add(2*time.Hour)) || !history.Entries[3].At.Equal(began) {
		t.Errorf("the stored sequence is out of order: %v", history.Entries)
	}

	// The same measurement landing twice is one measurement: a screen that reads
	// its own result twice must not turn one run into a trend.
	history = history.add(late)
	if len(history.Entries) != 4 {
		t.Errorf("the same measurement was stored twice: %d entries", len(history.Entries))
	}
	if history.Entries[2].Median != late.Median {
		t.Errorf("the duplicate overwrote the stored measurement with %v", history.Entries[2].Median)
	}

	history = shellAuditHistory{Enabled: true}
	for i := 0; i < shellAuditHistoryLimit+10; i++ {
		history = history.add(historyEntryFixture(began.Add(time.Duration(i)*time.Hour), time.Duration(200+i)*time.Millisecond))
	}

	if len(history.Entries) != shellAuditHistoryLimit {
		t.Fatalf("the history holds %d measurements, want the bound of %d", len(history.Entries), shellAuditHistoryLimit)
	}
	if !history.Entries[0].At.After(history.Entries[len(history.Entries)-1].At) {
		t.Error("the history is not newest first")
	}
	if got, want := history.Entries[0].At, began.Add(time.Duration(shellAuditHistoryLimit+9)*time.Hour); !got.Equal(want) {
		t.Errorf("the newest stored measurement is %v, want %v", got, want)
	}

	// A measurement older than every stored one is the oldest of the sequence, and
	// when the file is full it is the one the bound drops: what is kept is the
	// newest measurements, not the last ones to be written.
	history = history.add(historyEntryFixture(began.Add(-time.Hour), 300*time.Millisecond))
	if len(history.Entries) != shellAuditHistoryLimit {
		t.Errorf("the history grew to %d entries", len(history.Entries))
	}
	if !history.Entries[0].At.Equal(began.Add(time.Duration(shellAuditHistoryLimit+9) * time.Hour)) {
		t.Errorf("the newest stored measurement is %v after the bound dropped one", history.Entries[0].At)
	}
}

// TestShellAuditTrendSaysNothingUntilThereAreEnoughStoredRuns pins the floor: two
// stored runs are not a trend, and the screen is told what is missing rather than
// shown a change of two milliseconds between two afternoons.
func TestShellAuditTrendSaysNothingUntilThereAreEnoughStoredRuns(t *testing.T) {
	began := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	current := historyEntryFixture(began.Add(48*time.Hour), 400*time.Millisecond)

	empty := shellAuditHistory{}.trend(current)
	if empty.Kind != shellAuditTrendFirst {
		t.Errorf("the trend of no history = %q, want the first measurement", empty.Kind)
	}
	if !strings.Contains(empty.Reason, "first") {
		t.Errorf("the reason %q does not say that this is the first measurement", empty.Reason)
	}
	if empty.After != 400*time.Millisecond {
		t.Errorf("the first measurement's own median = %v, want the measured 400 ms", empty.After)
	}
	if text := empty.sentence(); text != "" {
		t.Errorf("a trend with nothing to compare printed %q", text)
	}

	two := shellAuditHistory{Enabled: true}
	for i := 0; i < shellAuditTrendFloor-1; i++ {
		two = two.add(historyEntryFixture(began.Add(time.Duration(i)*time.Hour), 300*time.Millisecond))
	}
	thin := two.trend(current)
	if thin.Kind != shellAuditTrendFirst {
		t.Errorf("two stored runs gave a trend of %q, want nothing yet", thin.Kind)
	}
	if !strings.Contains(thin.Reason, fmt.Sprintf("%d", shellAuditTrendFloor)) {
		t.Errorf("the reason %q does not name the %d runs a trend needs", thin.Reason, shellAuditTrendFloor)
	}
	if thin.Before != 0 {
		t.Errorf("a trend with nothing to compare still carries a before of %v", thin.Before)
	}
}

// TestShellAuditTrendNamesTheMoveOnlyWhenItIsBiggerThanTheNoise pins the three
// verdicts and the floor that separates them: the same start has to move by more
// than its own run-to-run movement before it is called a change, because a
// startup that varies by five percent between runs will otherwise report a
// regression every other afternoon.
func TestShellAuditTrendNamesTheMoveOnlyWhenItIsBiggerThanTheNoise(t *testing.T) {
	began := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	history := shellAuditHistory{Enabled: true}
	for i, median := range []time.Duration{300 * time.Millisecond, 305 * time.Millisecond, 295 * time.Millisecond} {
		history = history.add(historyEntryFixture(began.Add(time.Duration(i)*time.Hour), median))
	}

	slow := history.trend(historyEntryFixture(began.Add(24*time.Hour), 400*time.Millisecond))
	if slow.Kind != shellAuditTrendRegression {
		t.Fatalf("a move from 300 ms to 400 ms = %q, want a regression", slow.Kind)
	}
	if slow.Before != 300*time.Millisecond || slow.After != 400*time.Millisecond {
		t.Errorf("the trend's two medians = %v and %v, want 300 ms and 400 ms", slow.Before, slow.After)
	}
	if slow.Delta != 100*time.Millisecond {
		t.Errorf("the trend's delta = %v, want 100 ms", slow.Delta)
	}
	if slow.Share < 33.3 || slow.Share > 33.4 {
		t.Errorf("the trend's share = %.2f%%, want 33.33%%", slow.Share)
	}
	if slow.Runs != 3 {
		t.Errorf("the trend is made of %d stored runs, want 3", slow.Runs)
	}
	if !slow.Since.Equal(began) {
		t.Errorf("the trend starts at %v, want the oldest stored run %v", slow.Since, began)
	}
	sentence := slow.sentence()
	for _, want := range []string{"300 ms", "400 ms", "33%"} {
		if !strings.Contains(sentence, want) {
			t.Errorf("the sentence %q does not carry %q", sentence, want)
		}
	}

	fast := history.trend(historyEntryFixture(began.Add(24*time.Hour), 250*time.Millisecond))
	if fast.Kind != shellAuditTrendImprovement {
		t.Errorf("a move from 300 ms to 250 ms = %q, want an improvement", fast.Kind)
	}
	if !strings.Contains(fast.sentence(), "-16") && !strings.Contains(fast.sentence(), "-17") {
		t.Errorf("the improvement's sentence %q does not carry its share", fast.sentence())
	}

	// Two percent is inside the movement of a startup on a real machine.
	steady := history.trend(historyEntryFixture(began.Add(24*time.Hour), 306*time.Millisecond))
	if steady.Kind != shellAuditTrendSteady {
		t.Errorf("a move of 2%% = %q, want nothing worth calling a change", steady.Kind)
	}
	if !strings.Contains(steady.sentence(), "steady") {
		t.Errorf("the steady sentence %q does not say so", steady.sentence())
	}
	if steady.Share < 1.9 || steady.Share > 2.1 {
		t.Errorf("the steady move's share = %.2f%%, want about 2%%", steady.Share)
	}
}

// TestShellAuditTrendRefusesToCompareAcrossStartsAndVersions pins the three
// measurements the history must not average together: another start of the same
// shell, another version of it, and a measurement that was cancelled.
func TestShellAuditTrendRefusesToCompareAcrossStartsAndVersions(t *testing.T) {
	began := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	login := func(at time.Time) shellAuditHistoryEntry {
		entry := historyEntryFixture(at, 300*time.Millisecond)
		entry.Mode = shellAuditMode{Login: true, Interactive: true}.key()
		return entry
	}

	// Another start of the same shell is not a stored run of this one, so three
	// login runs do not make an interactive trend.
	mixed := shellAuditHistory{Enabled: true}
	for i := 0; i < 3; i++ {
		mixed = mixed.add(login(began.Add(time.Duration(i) * time.Hour)))
	}
	if trend := mixed.trend(historyEntryFixture(began.Add(24*time.Hour), 400*time.Millisecond)); trend.Kind != shellAuditTrendFirst {
		t.Errorf("three runs of another start gave a trend of %q, want nothing yet", trend.Kind)
	}

	// A version change is named rather than averaged: the stored runs measured a
	// different shell from the one in front of the user.
	otherVersion := shellAuditHistory{Enabled: true}
	for i := 0; i < 3; i++ {
		entry := historyEntryFixture(began.Add(time.Duration(i)*time.Hour), 300*time.Millisecond)
		entry.Version = "5.8.1"
		otherVersion = otherVersion.add(entry)
	}
	changed := otherVersion.trend(historyEntryFixture(began.Add(24*time.Hour), 400*time.Millisecond))
	if changed.Kind != shellAuditTrendIncomparable {
		t.Fatalf("a version change = %q, want the comparison refused", changed.Kind)
	}
	for _, want := range []string{"5.8.1", "5.9.2"} {
		if !strings.Contains(changed.Reason, want) {
			t.Errorf("the reason %q does not name %s", changed.Reason, want)
		}
	}
	if changed.Before != 0 {
		t.Errorf("an incomparable trend carries a before of %v", changed.Before)
	}

	// A cancelled measurement is stored and is not a point of comparison: its
	// median is drawn from the runs that finished, not from the plan.
	cancelled := shellAuditHistory{Enabled: true}
	for i := 0; i < 3; i++ {
		entry := historyEntryFixture(began.Add(time.Duration(i)*time.Hour), 300*time.Millisecond)
		entry.Cancelled = true
		cancelled = cancelled.add(entry)
	}
	if trend := cancelled.trend(historyEntryFixture(began.Add(24*time.Hour), 400*time.Millisecond)); trend.Kind != shellAuditTrendFirst {
		t.Errorf("three cancelled measurements gave a trend of %q, want nothing yet", trend.Kind)
	}

	stopped := historyEntryFixture(began.Add(24*time.Hour), 400*time.Millisecond)
	stopped.Cancelled = true
	full := shellAuditHistory{Enabled: true}
	for i := 0; i < 3; i++ {
		full = full.add(historyEntryFixture(began.Add(time.Duration(i)*time.Hour), 300*time.Millisecond))
	}
	trend := full.trend(stopped)
	if trend.Kind != shellAuditTrendFirst {
		t.Errorf("a cancelled measurement was used as a trend point: %q", trend.Kind)
	}
	if !strings.Contains(trend.Reason, "cancelled") {
		t.Errorf("the reason %q does not say that this measurement was cancelled", trend.Reason)
	}
}

// TestShellAuditReportReadsTheStateAsOneThing pins the read model the screen
// draws: the record, the clean comparison, the stored history and where this
// measurement sits in it, all derived from the state so none of them can
// disagree with the numbers beside them.
func TestShellAuditReportReadsTheStateAsOneThing(t *testing.T) {
	began := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	f := zshAuditFixture()
	f.script = []time.Duration{400 * time.Millisecond, 410 * time.Millisecond, 420 * time.Millisecond}

	st := resolveShellAudit(f.probe())
	st.Plan = shellAuditPlan{Mode: shellAuditModeInteractive, Samples: 3}
	st = measureShellAudit(st, f.probe())

	report := shellAuditReportOf(st)
	if !report.Measured {
		t.Fatal("the report of a measured state is empty")
	}
	if report.Run.Stats.Median != 410*time.Millisecond {
		t.Errorf("the report's median = %v, want 410 ms", report.Run.Stats.Median)
	}
	if report.Comparison.Reason == "" {
		t.Error("the report of a measurement with no clean comparison claims one")
	}
	if report.Trend.Kind != shellAuditTrendFirst {
		t.Errorf("the trend of a state with no history = %q, want the first measurement", report.Trend.Kind)
	}
	if report.History.Enabled {
		t.Error("the report claims history is on for a state that never turned it on")
	}

	// A state that measured nothing has an empty report rather than a report full
	// of zeroes, which is what a screen needs to draw nothing.
	empty := shellAuditReportOf(resolveShellAudit(f.probe()))
	if empty.Measured {
		t.Error("a state that measured nothing produced a measurement")
	}
	if len(empty.advice()) != 0 {
		t.Errorf("a state that measured nothing produced advice: %+v", empty.advice())
	}
	if empty.Comparison.Reason == "" || empty.Trend.Reason == "" {
		t.Error("an empty report does not say why it is empty")
	}
	_ = began
}

// TestShellAuditAdviceCarriesEvidenceAndSeparatesObservationFromCause is the rule
// the whole advice section rests on. The utility can prove one thing -- the part
// of the start that the clean comparison shows the configuration accounts for --
// and it can show what the shell's own instrument named. It cannot prove that a
// named plugin is the reason, and every item says which of the two it is.
func TestShellAuditAdviceCarriesEvidenceAndSeparatesObservationFromCause(t *testing.T) {
	began := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	f := zshAuditFixture()
	// Runs that move by half the median: this startup is not steady, and the
	// advice has to say so before it says anything about what is in it.
	f.script = []time.Duration{
		200 * time.Millisecond, 400 * time.Millisecond, 900 * time.Millisecond, // the configured runs
		100 * time.Millisecond,                                                 // the profiled start
		100 * time.Millisecond, 110 * time.Millisecond, 120 * time.Millisecond, // the clean runs
	}
	f.profileData = zprofSampleOutput
	// The stored measurements are of this version of zsh: a history of another
	// version is deliberately not a point of comparison, and that is its own guard.
	f.versionRaw = "zsh 5.9.2 (x86_64-pc-linux-gnu)\n"

	st := resolveShellAudit(f.probe())
	st.Plan = shellAuditPlan{Mode: shellAuditModeInteractive, Samples: 3, Baseline: true}
	st.History = shellAuditHistory{Enabled: true}
	for i, median := range []time.Duration{250 * time.Millisecond, 260 * time.Millisecond, 255 * time.Millisecond} {
		st.History = st.History.add(historyEntryFixture(began.Add(time.Duration(i)*time.Hour), median))
	}
	st.MeasuredAt = began.Add(72 * time.Hour)
	st = measureShellAudit(st, f.probe())

	report := shellAuditReportOf(st)
	advice := report.advice()
	if len(advice) == 0 {
		t.Fatal("a measurement with a spread, a profile and a history produced no advice")
	}

	var kinds []shellAuditAdviceKind
	for _, item := range advice {
		if item.Headline == "" {
			t.Errorf("an advice item has no headline: %+v", item)
		}
		if item.Evidence == "" {
			t.Errorf("the item %q cites no evidence", item.Headline)
		}
		if item.Kind != shellAuditAdviceMeasured && item.Kind != shellAuditAdviceObserved && item.Kind != shellAuditAdviceConfirmed {
			t.Errorf("the item %q has the kind %q", item.Headline, item.Kind)
		}
		// No item may claim a cause in prose. Only the clean comparison may be
		// reported as a cause, and it says so in its own wording rather than by
		// asserting one.
		for _, forbidden := range []string{"because", "is caused by", "the reason is", "due to", "makes your"} {
			if strings.Contains(strings.ToLower(item.Headline), forbidden) || strings.Contains(strings.ToLower(item.Detail), forbidden) {
				t.Errorf("the item %q claims a cause: %q", item.Headline, forbidden)
			}
		}
		kinds = append(kinds, item.Kind)
	}

	// The spread is a measured fact and it comes first: advice about what is in
	// the startup is worth little if the number it rests on moves between runs.
	if kinds[0] != shellAuditAdviceMeasured {
		t.Errorf("the first advice is a %q, want the measurement's own facts first", kinds[0])
	}
	spread := advice[0]
	for _, want := range []string{"200 ms", "900 ms"} {
		if !strings.Contains(spread.Evidence, want) {
			t.Errorf("the spread's evidence %q does not carry %q", spread.Evidence, want)
		}
	}

	// The configuration's share is the one confirmed item, and it names the two
	// medians it came from.
	var confirmed []shellAuditAdvice
	for _, item := range advice {
		if item.Kind == shellAuditAdviceConfirmed {
			confirmed = append(confirmed, item)
		}
	}
	if len(confirmed) != 1 {
		t.Fatalf("%d confirmed items, want exactly the clean comparison", len(confirmed))
	}
	if !strings.Contains(confirmed[0].Evidence, "clean") {
		t.Errorf("the confirmed item's evidence %q does not name the clean measurement", confirmed[0].Evidence)
	}

	// The profile is an observation: it names what the instrument measured, and
	// the item says that naming is not proving.
	var observed []shellAuditAdvice
	for _, item := range advice {
		if item.Kind == shellAuditAdviceObserved {
			observed = append(observed, item)
		}
	}
	if len(observed) != 1 {
		t.Fatalf("%d observed items, want the profile's rows", len(observed))
	}
	if !strings.Contains(observed[0].Headline, "compinit") {
		t.Errorf("the observed item %q does not name the heaviest row", observed[0].Headline)
	}
	if !strings.Contains(observed[0].Evidence, "zsh's own profile") {
		t.Errorf("the observed item's evidence %q does not say where the names came from", observed[0].Evidence)
	}
	if !strings.Contains(strings.ToLower(observed[0].Detail), "not the same as why") {
		t.Errorf("the observed item's detail %q does not separate what was measured from what caused it", observed[0].Detail)
	}

	// The move is reported as the user's own history, and the detail says what
	// the utility cannot know: what changed in between.
	var moved []shellAuditAdvice
	for _, item := range advice {
		if strings.Contains(item.Headline, "stored") {
			moved = append(moved, item)
		}
	}
	if len(moved) != 1 {
		t.Fatalf("%d history items, want the movement of this start", len(moved))
	}
	if !strings.Contains(strings.ToLower(moved[0].Detail), "cannot say what changed") {
		t.Errorf("the history item's detail %q does not say what the utility cannot know", moved[0].Detail)
	}
	if !strings.Contains(moved[0].Evidence, "3 stored") {
		t.Errorf("the history item's evidence %q does not name the stored runs", moved[0].Evidence)
	}
}

// TestShellAuditAdviceReportsTheLimitsTheMeasurementHit pins the plain facts a
// screen owes the user when a measurement went badly: the runs that hit the
// bound, the starts that could not be made, and the shell whose startup cannot be
// broken down here. None of them is dressed up as advice about configuration.
func TestShellAuditAdviceReportsTheLimitsTheMeasurementHit(t *testing.T) {
	f := &fakeShellAuditProbe{
		env:      map[string]string{"SHELL": "/bin/dash", "HOME": "/home/alberto"},
		terminal: true,
		timeout:  20 * time.Millisecond,
		script:   []time.Duration{30 * time.Millisecond, 35 * time.Millisecond},
		runs:     4,
		hangAt:   map[int]bool{2: true},
		failAt:   map[int]error{3: errors.New("exit status 2")},
	}

	st := resolveShellAudit(f.probe())
	// Four starts: two finish, the third waits for the bound and the fourth cannot
	// be started at all, so both limits are facts of the same measurement.
	st.Plan = shellAuditPlan{Mode: shellAuditModeInteractive, Samples: 4}
	st = measureShellAudit(st, f.probe())

	advice := shellAuditReportOf(st).advice()
	if len(advice) == 0 {
		t.Fatal("a measurement with a timeout and a failure produced no advice")
	}

	joined := ""
	for _, item := range advice {
		joined += item.Headline + " " + item.Detail + " " + item.Evidence + "\n"
	}
	if !strings.Contains(joined, "20 ms") {
		t.Errorf("the advice does not name the bound a run hit:\n%s", joined)
	}
	if !strings.Contains(joined, "could not be started") {
		t.Errorf("the advice does not report the start that could not be made:\n%s", joined)
	}
	// Nothing inside this shell's startup can be named, and the advice says so
	// rather than suggesting a plugin to look at.
	if !strings.Contains(joined, "cannot be broken down") {
		t.Errorf("the advice does not state the profiling limitation:\n%s", joined)
	}
	for _, forbidden := range []string{"plugin", "disable", "remove"} {
		if strings.Contains(strings.ToLower(joined), forbidden) {
			t.Errorf("the advice names %q for a shell it never profiled:\n%s", forbidden, joined)
		}
	}

	// With no samples at all there is nothing to advise about, only what was
	// measured: the reason and the count of runs that did not finish.
	st.Samples = nil
	st.Stats = shellAuditStats{}
	st.Attribution = nil
	st.Measured = true
	empty := shellAuditReportOf(st).advice()
	for _, item := range empty {
		if item.Kind == shellAuditAdviceObserved || item.Kind == shellAuditAdviceConfirmed {
			t.Errorf("a measurement with no duration still drew a conclusion: %+v", item)
		}
	}
}

// TestShellAuditHistoryPathFollowsTheStateDirectory pins where the file lives: the
// same directory as the install record, because it is the same kind of thing --
// state the program keeps between runs -- and never the configuration directory,
// which is for files the user edits.
func TestShellAuditHistoryPathFollowsTheStateDirectory(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{
			name: "the XDG state directory",
			env:  map[string]string{"XDG_STATE_HOME": "/state", "HOME": "/home/alberto"},
			want: "/state/dotfiles/" + shellAuditHistoryFileName,
		},
		{
			name: "the home fallback",
			env:  map[string]string{"HOME": "/home/alberto"},
			want: "/home/alberto/.local/state/dotfiles/" + shellAuditHistoryFileName,
		},
		{
			name: "neither",
			env:  map[string]string{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shellAuditHistoryPath(func(key string) string { return tt.env[key] })
			if got != tt.want {
				t.Errorf("the history path = %q, want %q", got, tt.want)
			}
		})
	}

	// The production path is the install record's own state directory: the two
	// files are neighbours, and one of them moving is a change someone has to
	// make twice on purpose.
	if got, want := shellAuditHistoryPath(os.Getenv), filepath.Join(stateDir(), shellAuditHistoryFileName); got != want {
		t.Errorf("the real history path = %q, want the install record's directory %q", got, want)
	}
}

// TestShellAuditHistoryRoundTripsThroughItsFile pins the file: what is written is
// what is read back, and the write lands on a temporary file that is then renamed
// onto the real one, so a process that dies mid-write leaves the previous history
// in place rather than half of a new one.
func TestShellAuditHistoryRoundTripsThroughItsFile(t *testing.T) {
	const path = "/home/alberto/.local/state/dotfiles/" + shellAuditHistoryFileName

	began := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	history := shellAuditHistory{Enabled: true}
	for i := 0; i < 4; i++ {
		entry := historyEntryFixture(began.Add(time.Duration(i)*time.Hour), time.Duration(300+i)*time.Millisecond)
		entry.Profile = "the instrument named nothing"
		history = history.add(entry)
	}

	f := zshAuditFixture()
	probe := f.probe()
	probe.rename = func(oldPath, newPath string) error {
		f.renames = append(f.renames, oldPath+" -> "+newPath)
		return nil
	}

	if err := writeShellAuditHistory(probe, path, history); err != nil {
		t.Fatalf("writing the history failed: %v", err)
	}
	if len(f.writes) != 1 {
		t.Fatalf("the write touched %v, want one file", f.writes)
	}
	if f.writes[0] == path {
		t.Error("the history was written straight onto the file, so a process that died mid-write would leave half a history there")
	}
	if len(f.renames) != 1 || f.renames[0] != f.writes[0]+" -> "+path {
		t.Errorf("the write was not rolled onto the file: %v", f.renames)
	}

	stored := f.bodies[len(f.bodies)-1]
	f.readData = []byte(stored)
	back, err := readShellAuditHistory(probe, path)
	if err != nil {
		t.Fatalf("reading back the written history failed: %v", err)
	}
	if !back.Enabled {
		t.Error("the round trip lost the user's choice to keep history")
	}
	if len(back.Entries) != len(history.Entries) {
		t.Fatalf("the round trip kept %d of %d measurements", len(back.Entries), len(history.Entries))
	}
	// The comparison is the whole entry: a field the file stopped carrying -- the
	// fastest run, the version, the counts, the comparison -- would be a stored
	// measurement read back as something narrower than it was, and a guard that
	// named five fields would keep passing after the sixth was lost.
	for i := range back.Entries {
		got, want := back.Entries[i], history.Entries[i]
		if !reflect.DeepEqual(got, want) {
			t.Errorf("measurement %d came back as %+v, want %+v", i, got, want)
		}
	}

	// Only the measurements reach the file: the switch is stored with them, and
	// nothing else about the machine is.
	if !json.Valid([]byte(stored)) {
		t.Errorf("the history file is not JSON: %s", stored)
	}
	for _, forbidden := range []string{"/usr/bin/zsh", "exit status", "PATH="} {
		if strings.Contains(string(stored), forbidden) {
			t.Errorf("the history file carries %q: %s", forbidden, stored)
		}
	}
}

// TestShellAuditHistoryReadsWhatItCannotIntoAnEmptyHistory pins the two failures
// a reader has to tell apart: no file at all is the normal first run, and a file
// that is there and unreadable is something the user should be told about rather
// than silently overwritten with a fresh history.
func TestShellAuditHistoryReadsWhatItCannotIntoAnEmptyHistory(t *testing.T) {
	f := zshAuditFixture()
	probe := f.probe()
	f.readErr = fs.ErrNotExist

	history, err := readShellAuditHistory(probe, "/home/alberto/.local/state/dotfiles/"+shellAuditHistoryFileName)
	if err != nil {
		t.Errorf("a machine with no history file reported %v, want no history and no complaint", err)
	}
	if history.Enabled || len(history.Entries) != 0 {
		t.Errorf("a machine with no history file read %+v", history)
	}

	f.readErr = nil
	f.readData = []byte("{ this is not the file the utility wrote")
	history, err = readShellAuditHistory(probe, "/home/alberto/.local/state/dotfiles/"+shellAuditHistoryFileName)
	if err == nil {
		t.Error("an unreadable history file was read as an empty one without complaint")
	}
	if history.Enabled || len(history.Entries) != 0 {
		t.Errorf("an unreadable history file read %+v", history)
	}
}

// TestShellAuditKeepsNoHistoryUntilTheUserSaysSo pins the switch: nothing is
// written to the user's disk until they turn history on, a machine with no file
// is not a machine with an error, and turning it on is what makes the next
// measurement a stored one.
func TestShellAuditKeepsNoHistoryUntilTheUserSaysSo(t *testing.T) {
	f := zshAuditFixture()
	f.readErr = fs.ErrNotExist

	st := resolveShellAudit(f.probe())
	if st.History.Enabled {
		t.Error("history is on for a user who never turned it on")
	}
	if st.HistoryError != "" {
		t.Errorf("a machine with no history file reported %q", st.HistoryError)
	}

	// Off: a measurement is measured and stored nowhere.
	st.Plan = shellAuditPlan{Mode: shellAuditModeInteractive, Samples: 3}
	st = measureShellAudit(st, f.probe())
	if len(st.History.Entries) != 0 {
		t.Errorf("a measurement was stored with history off: %+v", st.History.Entries)
	}
	writes := len(f.writes)
	if err := saveShellAuditHistory(st, f.probe()); err != nil {
		t.Fatalf("saving with history off failed: %v", err)
	}
	if len(f.writes) != writes {
		t.Errorf("saving with history off wrote %v", f.writes[writes:])
	}

	// On: the measurement is the first stored one, and the trend says so instead
	// of comparing it with nothing.
	st.History = shellAuditHistory{Enabled: true}
	st = measureShellAudit(st, f.probe())
	if len(st.History.Entries) != 1 {
		t.Fatalf("the measurement with history on stored %d entries", len(st.History.Entries))
	}
	if st.History.Entries[0].Median != st.Median {
		t.Errorf("the stored median = %v, want the measured %v", st.History.Entries[0].Median, st.Median)
	}
	if st.Trend.Kind != shellAuditTrendFirst {
		t.Errorf("the first stored measurement's trend = %q, want nothing to compare yet", st.Trend.Kind)
	}
	if err := saveShellAuditHistory(st, f.probe()); err != nil {
		t.Fatalf("saving the history failed: %v", err)
	}
	if len(f.writes) == writes {
		t.Error("saving with history on wrote nothing")
	}

	// The stored measurements are what the next one is compared with: four of
	// them, and the trend of the last has three stored runs behind it.
	f2 := zshAuditFixture()
	st2 := resolveShellAudit(f2.probe())
	st2.History = shellAuditHistory{Enabled: true}
	st2.Plan = shellAuditPlan{Mode: shellAuditModeInteractive, Samples: 3}
	// Every measurement's runs are scripted up front, in the order they run: three
	// samples and the profiled start's own entry each, getting 50 ms slower from
	// one measurement to the next.
	f2.script = nil
	f2.runs = 3
	for i := 0; i < shellAuditTrendFloor+1; i++ {
		median := time.Duration(300+i*50) * time.Millisecond
		f2.script = append(f2.script, median, median, median, time.Millisecond)
	}
	for i := 0; i < shellAuditTrendFloor+1; i++ {
		st2 = measureShellAudit(st2, f2.probe())
	}
	if st2.Trend.Kind != shellAuditTrendRegression {
		t.Errorf("four stored measurements of a start that got 150 ms slower = %q, want a regression", st2.Trend.Kind)
	}
	if st2.Trend.Runs != shellAuditTrendFloor {
		t.Errorf("the trend is made of %d stored runs, want %d", st2.Trend.Runs, shellAuditTrendFloor)
	}
}

// TestShellAuditHistoryFileIsWrittenForRealWithoutLeavingATempBehind pins the
// production write against a real filesystem, in a directory of the test's own:
// the file is there afterwards, it parses, and the temporary file the write went
// through is gone.
func TestShellAuditHistoryFileIsWrittenForRealWithoutLeavingATempBehind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, shellAuditHistoryFileName)

	history := shellAuditHistory{Enabled: true}
	history = history.add(historyEntryFixture(time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC), 400*time.Millisecond))

	if err := writeShellAuditHistory(defaultShellAuditProbe(), path, history); err != nil {
		t.Fatalf("writing to %s failed: %v", path, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the history file is not there: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s failed: %v", dir, err)
	}
	if len(entries) != 1 || entries[0].Name() != shellAuditHistoryFileName {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		t.Errorf("%s holds %v, want only the history file", dir, names)
	}

	back, err := readShellAuditHistory(defaultShellAuditProbe(), path)
	if err != nil {
		t.Fatalf("reading %s back failed: %v", path, err)
	}
	if len(back.Entries) != 1 || back.Entries[0].Median != 400*time.Millisecond {
		t.Errorf("the file holds %+v, want the measurement that was written", back.Entries)
	}
}

// TestShellAuditAdviceNamesTheSameRowsTheScreenNames is the guard for the cut the
// profile paragraph declares. When the instrument reports more functions than the
// screen names, the list says how many it left out -- and the evidence under the
// recommendation may not name one of them. Two paragraphs on one screen making
// opposite claims about the same table is a contradiction the reader cannot
// resolve, because only one of them can be what the instrument holds.
func TestShellAuditAdviceNamesTheSameRowsTheScreenNames(t *testing.T) {
	st := shellAuditTestState()
	if len(st.Attribution) <= shellAuditNamedFunctions {
		t.Fatalf("the fixture holds %d functions, and this guard needs more than the %d the screen names",
			len(st.Attribution), shellAuditNamedFunctions)
	}

	var observed string
	for _, item := range shellAuditReportOf(st).advice() {
		if item.Kind == shellAuditAdviceObserved {
			observed = item.Evidence
		}
	}
	if observed == "" {
		t.Fatal("the profile produced no observed advice to read")
	}

	named := st.Attribution[:shellAuditNamedFunctions]
	unnamed := st.Attribution[shellAuditNamedFunctions:]
	for _, row := range named {
		if !strings.Contains(observed, row.Name) {
			t.Errorf("the evidence left out %q, which the profile paragraph names: %q", row.Name, observed)
		}
	}
	for _, row := range unnamed {
		if strings.Contains(observed, row.Name) {
			t.Errorf("the evidence names %q, which the profile paragraph says is not named here: %q", row.Name, observed)
		}
	}
	if want := fmt.Sprintf("%d more were profiled and are not named here.", len(unnamed)); !strings.Contains(observed, want) {
		t.Errorf("the evidence does not declare the cut with %q: %q", want, observed)
	}
}

// TestShellAuditHistoryWritesUnderANameOfItsOwn pins the write's temporary name.
// The saves are started from the update loop and two of them can be in flight at
// once -- the switch being turned on while a measurement lands -- so a name two
// writers shared would let them interleave their bytes into the file that is
// about to become the history. The guard also pins that the write never lands on
// the history's own name: the rename is what makes the file a whole history.
func TestShellAuditHistoryWritesUnderANameOfItsOwn(t *testing.T) {
	path := "/state/dotfiles/shell-audit-history.json"
	first := shellAuditHistoryTempPath(path)
	second := shellAuditHistoryTempPath(path)

	if first == second {
		t.Errorf("two writes share the temporary name %q", first)
	}
	for _, temp := range []string{first, second} {
		if temp == path {
			t.Errorf("the temporary name is the history's own name: %q", temp)
		}
		if !strings.HasPrefix(temp, path) || !strings.HasSuffix(temp, shellAuditHistoryTempSuffix) {
			t.Errorf("the temporary name %q is not a neighbour of %q", temp, path)
		}
	}
}
