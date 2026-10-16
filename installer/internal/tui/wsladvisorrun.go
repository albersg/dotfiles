package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// ============================================================================
// THE REPORT'S TWO ASSISTANTS
// ============================================================================
//
// This file is the seam between the advisor's screens and the two engines that do real work:
// internal/system's bounded storage benchmark and its assisted copy. Everything that names an engine
// type lives here -- the state each assistant draws, the targets and destinations it offers, and the
// commands that run them off the update loop -- so the screens stay a description of what was read
// and the engines stay a pure library the installer has no other way into.
//
// Two rules are enforced here rather than promised in prose:
//
//   - Nothing runs until a row is pressed. The offerings are derived from the report's own readings,
//     so a row exists only where the engine can finish, and a row that is absent is explained in the
//     body instead of failing when it is pressed.
//   - The copy is the only thing in this utility that changes anything on disk, and it is reached
//     through a preview that names the source, the destination, what would be copied, what would be
//     left behind and what the engine refuses to do. The source is never written to.

// wslBenchmarkRowLabel and wslMigrationRowLabel are the report's two acting rows. They are
// constants here, beside the assistants they open, so the report's list and the screens agree on one
// wording.
const (
	wslBenchmarkRowLabel = "Measure the storage this project is on"
	wslMigrationRowLabel = "Copy a project to the distribution's disk"
)

// wslMigrationProgressStride is how many copied entries pass between two progress messages. The
// engine reports every entry, and sending one message per entry would spend the whole run in the
// update loop; a stride keeps the bar moving without the copy competing with its own reporting. The
// last entry is always sent, so a finished copy never leaves a stale count on screen.
const wslMigrationProgressStride = 16

// wslOffer is whether an assistant can run here, and why not when it cannot. Every screen that
// offers one asks this rather than deciding for itself: a row and its precondition are then one
// fact, and the body can say the same thing the absent row would have implied.
type wslOffer struct {
	Offered bool
	Reason  string
}

// ============================================================================
// THE BENCHMARK
// ============================================================================

// wslBenchmarkState is what the measurement screen draws.
type wslBenchmarkState struct {
	// Running reports a measurement in flight, so the screen says it is measuring rather than
	// showing the previous result as if it were the current one.
	Running bool
	// Ran reports that a report is held, including one that measured nothing.
	Ran bool
	// Report is the last run, whole: it is what the screen's rows and body are built from.
	Report system.BenchReport
	// Notice is the one-line result of the last press.
	Notice string
}

// wslBenchmarkTargets are the places this run would measure, in the order they would be measured.
// The project's own directory comes first because it is the one the reader cares about; the
// distribution's own storage second, because a comparison between the two is the whole point.
//
// A target is only offered when its directory was actually read: the project must have been listed
// by the report, and the native parent must have answered a space query. Offering a path the report
// could not read would produce a row that fails on the press, which this installer does not do.
func (m Model) wslBenchmarkTargets() []system.BenchTarget {
	facts := m.WSLAdvisor.Facts
	var targets []system.BenchTarget
	if facts.Cwd != "" && facts.LocalProject.Err == "" {
		targets = append(targets, system.BenchTarget{Name: "this project", Path: facts.Cwd})
	}
	for _, parent := range facts.NativeParents {
		if !parent.SpaceKnown || parent.Path == facts.Cwd {
			continue
		}
		targets = append(targets, system.BenchTarget{Name: "the distribution's storage", Path: parent.Path})
		break
	}
	return targets
}

// wslBenchmarkOffer is whether the measurement can be offered, with the reason when it cannot. One
// target is enough to measure storage, but two are what makes the comparison worth showing, so a run
// with one target still runs and its body says there is nothing to compare against.
func (m Model) wslBenchmarkOffer() wslOffer {
	if !m.wslAdvisorOffered() {
		return wslOffer{Reason: m.wslAdvisorUnavailableReason()}
	}
	if !m.WSLAdvisor.Resolved {
		return wslOffer{Reason: "The machine has not been read yet, so there is no directory to measure in."}
	}
	targets := m.wslBenchmarkTargets()
	if len(targets) == 0 {
		return wslOffer{Reason: "Nothing can be measured here: neither this project's directory nor the " +
			"distribution's own storage answered when the report read them."}
	}
	return wslOffer{Offered: true}
}

// openWSLBenchmark is the row press: it clears the previous cursor and opens the screen. It does not
// start the run itself -- the screen's own row does that -- so opening a screen never writes
// anything, and the user sees what would be written before it is.
func (m Model) openWSLBenchmark() (tea.Model, tea.Cmd) {
	m.Screen = ScreenWSLBenchmark
	m.Cursor = 0
	m.WSLAdvisor.Benchmark.Notice = ""
	return m, nil
}

// wslBenchmarkDoneMsg carries a finished measurement to the model.
type wslBenchmarkDoneMsg struct {
	report system.BenchReport
}

// runWSLBenchmarkCmd measures the targets off the update loop. The engine never returns an error: a
// target it could not measure carries its own reason, so the screen can show what worked beside what
// did not instead of one failure for the whole run.
func runWSLBenchmarkCmd(targets []system.BenchTarget) tea.Cmd {
	return func() tea.Msg {
		report := system.RunBenchmark(context.Background(), targets, system.BenchOptions{})
		return wslBenchmarkDoneMsg{report: report}
	}
}

// ============================================================================
// THE ASSISTED COPY
// ============================================================================

// wslMigrationPhase is which question of the assistant is on screen. The phases are separate
// screens' worth of state rather than separate screens because they are one decision in four steps,
// and the back key has to walk them in reverse.
type wslMigrationPhase int

const (
	// wslMigratePickSource is the list of directories that could be copied.
	wslMigratePickSource wslMigrationPhase = iota
	// wslMigratePickDestination is the list of places the copy could land.
	wslMigratePickDestination
	// wslMigrateReadingSource is the scan that builds the preview. It writes nothing.
	wslMigrateReadingSource
	// wslMigratePreview is the preview, and the only place the copy can be confirmed.
	wslMigratePreview
	// wslMigrateCopying is the copy in flight.
	wslMigrateCopying
	// wslMigrateDone is the outcome, verified or not.
	wslMigrateDone
)

// wslMigrationState is what the assisted copy screen draws. Sources and Destinations are the lists
// read when the screen opened, kept so the rows, the cursor and the preview all come from one
// reading rather than three.
type wslMigrationState struct {
	Phase        wslMigrationPhase
	Sources      []wslProjectFacts
	Destinations []wslNativeDestination
	// Source and Destination index into those lists. They are indices rather than paths so the body
	// can name the same thing the row does, including the storage verdict behind it.
	Source      int
	Destination int
	// Plan is the preview the source and destination produced.
	Plan    system.MigrationPlan
	PlanErr string
	// Running reports a copy in flight, which is what the screen must not let a second press start
	// twice.
	Running bool
	// Progress is the last progress message, and ProgressSeen says whether one arrived. The bar is
	// only drawn from a message the engine actually sent, never from a guess.
	Progress     system.MigrationProgress
	ProgressSeen bool
	// Report is what the run did, held even when it failed.
	Report    system.MigrationReport
	ReportErr string
	Notice    string
	// Cancel stops a copy in flight. It is held here so the screen owns the run it started.
	Cancel context.CancelFunc
}

// wslMigrationSourceOrder ranks a directory by how interesting it is to copy: a project on the
// Windows host is the case this utility exists for, so it leads; the distribution's own storage is
// next, because a copy between two places on it is still a real request; anything else follows.
func wslMigrationSourceOrder(project wslProjectFacts) int {
	switch {
	case project.Storage.Kind.HostBacked():
		return 0
	case project.Storage.Kind.NativeForTooling():
		return 1
	default:
		return 2
	}
}

// wslMigrationSources lists the directories the assistant can copy, ordered so the row that matters
// is at the top, and bounded because a menu is not a directory listing. A project the report could
// not read is left out: the engine would block on the first unreadable entry, and a row that ends in
// a blocker the reader cannot act on is not an offer.
func (m Model) wslMigrationSources() []wslProjectFacts {
	facts := m.WSLAdvisor.Facts
	var sources []wslProjectFacts
	seen := map[string]bool{}
	add := func(project wslProjectFacts) {
		if project.Path == "" || project.Err != "" || seen[project.Path] {
			return
		}
		seen[project.Path] = true
		sources = append(sources, project)
	}
	add(facts.LocalProject)
	for _, project := range facts.Projects {
		add(project)
	}
	sort.SliceStable(sources, func(i, j int) bool {
		return wslMigrationSourceOrder(sources[i]) < wslMigrationSourceOrder(sources[j])
	})
	if len(sources) > wslMigrationSourceLimit {
		sources = sources[:wslMigrationSourceLimit]
	}
	return sources
}

// wslMigrationSourceLimit bounds how many directories the first list offers. It is a bound on the
// menu, not on the search: the report still classified every project it found.
const wslMigrationSourceLimit = 6

// wslMigrationDestinations derives the destinations of the chosen source. It is the report's own
// reading, joined with the project's name, so the screen cannot offer a place the report did not
// classify as the distribution's own storage.
func (m Model) wslMigrationDestinations(source string) []wslNativeDestination {
	return wslNativeDestinations(m.WSLAdvisor.Facts, source)
}

// wslMigrationOffer is whether the assisted copy can be offered, with the reason when it cannot.
func (m Model) wslMigrationOffer() wslOffer {
	if !m.wslAdvisorOffered() {
		return wslOffer{Reason: m.wslAdvisorUnavailableReason()}
	}
	if !m.WSLAdvisor.Resolved {
		return wslOffer{Reason: "The machine has not been read yet, so there is no project to copy."}
	}
	sources := m.wslMigrationSources()
	if len(sources) == 0 {
		return wslOffer{Reason: "No readable project directory was found here to copy."}
	}
	if len(m.wslMigrationDestinations(sources[0].Path)) == 0 {
		return wslOffer{Reason: "No destination on the distribution's own storage was found for a copy: " +
			"the report found no readable directory outside the Windows mounts."}
	}
	return wslOffer{Offered: true}
}

// openWSLMigration is the row press: it reads the two lists and opens on the first question. The
// scan that builds a preview is a separate press, so nothing reads a whole tree from here either.
func (m Model) openWSLMigration() (tea.Model, tea.Cmd) {
	sources := m.wslMigrationSources()
	state := wslMigrationState{Phase: wslMigratePickSource, Sources: sources, Notice: ""}
	if len(sources) > 0 {
		state.Destinations = m.wslMigrationDestinations(sources[0].Path)
	}
	m.WSLAdvisor.Migration = state
	m.Screen = ScreenWSLMigration
	m.Cursor = 0
	return m, nil
}

// wslMigrationPlannedMsg carries the preview a scan produced.
type wslMigrationPlannedMsg struct {
	plan system.MigrationPlan
	err  string
}

// wslMigrationProgressMsg is one progress message from a copy in flight.
type wslMigrationProgressMsg struct {
	progress system.MigrationProgress
}

// wslMigrationCopiedMsg carries the outcome of a copy, failed or not.
type wslMigrationCopiedMsg struct {
	report system.MigrationReport
	err    string
}

// wslMigrationDefaultExclusions are the dependency and cache directories the copy leaves behind by
// default. Every one of them is regenerable from the source's own manifest, which is why leaving it
// is safe, and the preview names the list verbatim and counts what it skipped: a reader who wants
// one of them copied would have to be told the pattern, not the intention.
var wslMigrationDefaultExclusions = []string{
	"node_modules",
	".venv",
	"venv",
	"__pycache__",
	".pytest_cache",
	".mypy_cache",
	".ruff_cache",
	".gradle",
}

// planWSLMigrationCmd scans the source and builds the preview off the update loop. The scan reads
// every entry but writes nothing: the plan is a description, and the engine refuses to run one that
// is not runnable.
func planWSLMigrationCmd(source, destination string) tea.Cmd {
	return func() tea.Msg {
		plan, err := system.PlanMigration(source, destination, system.MigrationOptions{
			Exclusions: wslMigrationDefaultExclusions,
		})
		if err != nil {
			return wslMigrationPlannedMsg{err: err.Error()}
		}
		return wslMigrationPlannedMsg{plan: plan}
	}
}

// sendWSLMigrationProgress hands one progress message to the running program. It is a no-op when
// there is no program (the tests build models without one), which is why the screen keeps the
// outcome even when no progress ever arrived.
func sendWSLMigrationProgress(progress system.MigrationProgress) {
	if globalProgram == nil {
		return
	}
	globalProgram.Send(wslMigrationProgressMsg{progress: progress})
}

// startWSLMigrationCopy begins the copy the preview confirmed. It is only reached from the
// confirmation row on the preview, and it refuses to start when the plan is not runnable, so a
// blocker can never be bypassed by pressing twice.
func (m *Model) startWSLMigrationCopy() (tea.Model, tea.Cmd) {
	migration := &m.WSLAdvisor.Migration
	if migration.Running || !migration.Plan.Runnable() {
		return m, nil
	}
	plan := migration.Plan
	ctx, cancel := context.WithCancel(context.Background())
	migration.Cancel = cancel
	migration.Running = true
	migration.ProgressSeen = false
	migration.Progress = system.MigrationProgress{}
	migration.Report = system.MigrationReport{}
	migration.ReportErr = ""
	migration.Notice = ""

	// The progress callback reports at most one message per stride so the update loop is not asked to
	// draw a message per entry, and always reports the last one so a finished copy cannot leave its
	// bar behind.
	last := 0
	progress := func(report system.MigrationProgress) {
		if report.Done != report.Total && report.Done-last < wslMigrationProgressStride {
			return
		}
		last = report.Done
		sendWSLMigrationProgress(report)
	}

	return m, func() tea.Msg {
		report, err := system.RunMigration(ctx, plan, progress)
		if err != nil {
			return wslMigrationCopiedMsg{report: report, err: err.Error()}
		}
		return wslMigrationCopiedMsg{report: report}
	}
}

// wslMigrationChosenSource is the source the current phase is about, and whether there is one.
func (m Model) wslMigrationChosenSource() (wslProjectFacts, bool) {
	state := m.WSLAdvisor.Migration
	if state.Source < 0 || state.Source >= len(state.Sources) {
		return wslProjectFacts{}, false
	}
	return state.Sources[state.Source], true
}

// wslMigrationChosenDestination is the destination the current phase is about, and whether there is
// one.
func (m Model) wslMigrationChosenDestination() (wslNativeDestination, bool) {
	state := m.WSLAdvisor.Migration
	if state.Destination < 0 || state.Destination >= len(state.Destinations) {
		return wslNativeDestination{}, false
	}
	return state.Destinations[state.Destination], true
}

// wslMigrationSummary is the one line the Utilities panel shows for a copy in flight or finished,
// so a long copy is visible from the section without opening the screen again.
func wslMigrationSummary(state wslMigrationState) string {
	switch {
	case state.Running && state.ProgressSeen:
		return "copying " + wslCountProgress(state.Progress)
	case state.Running:
		return "copying"
	case state.Report.Verified:
		return "verified copy of " + wslCountLabel(state.Report.Copied.Files, "file", "files")
	case state.ReportErr != "" || len(state.Report.Failures) > 0:
		return "copy finished with problems"
	}
	return ""
}

// wslCountProgress renders a progress report as "12 of 340 entries".
func wslCountProgress(progress system.MigrationProgress) string {
	if progress.Total <= 0 {
		return wslCountLabel(progress.Done, "entry", "entries")
	}
	return wslCountLabel(progress.Done, "entry", "entries") + " of " +
		wslCountLabel(progress.Total, "entry", "entries")
}

// wslBenchmarkPanelValue summarises the last measurement for the Utilities panel and the benchmark
// screen's own rows: the operations that were measured, and the reason when none was.
func wslBenchmarkPanelValue(state wslBenchmarkState) string {
	if !state.Ran {
		return "not measured yet"
	}
	measured := 0
	for _, result := range state.Report.Results {
		measured += len(result.Measurements)
	}
	if measured == 0 {
		return "nothing measurable"
	}
	return wslCountLabel(len(state.Report.Results), "target", "targets") + ", " +
		wslCountLabel(measured, "operation", "operations")
}

// wslOperationLabel names an operation the way the report reads it. The engine's own names are the
// vocabulary, so this only spells the ones the report shows and never invents a step.
func wslOperationLabel(operation system.BenchOperation) string {
	switch operation {
	case system.BenchWrite:
		return "write"
	case system.BenchRead:
		return "read back"
	case system.BenchStat:
		return "stat"
	case system.BenchCreate:
		return "create small files"
	case system.BenchRemove:
		return "remove them"
	}
	return string(operation)
}

// wslBenchRateLabel renders one measurement as the numbers a reader can compare: how long the pass
// took, and the rates that fall out of it. A pass that moved no data (stat) reports files only, and
// its zero byte count is omitted rather than printed as a rate of zero.
func wslBenchRateLabel(measurement system.BenchMeasurement) string {
	parts := []string{wslDurationLabel(measurement.Duration)}
	if measurement.Files > 0 {
		parts = append(parts, wslBenchCountRate(measurement.PerSecond, "file", "files"))
	}
	if measurement.Bytes > 0 && measurement.BytesPerSecond > 0 {
		parts = append(parts, humanBytes(uint64(measurement.BytesPerSecond))+"/s")
	}
	return strings.Join(parts, ", ")
}

// wslBenchCountRate renders a per-second count without pretending to a precision the timer does not
// have: a rate under ten thousand is rounded to the nearest ten, and a larger one to three
// significant figures.
func wslBenchCountRate(rate float64, singular, plural string) string {
	switch {
	case rate <= 0:
		return "no " + plural + " per second"
	case rate < 10000:
		return wslCountLabel(int(rate+5)/10*10, singular, plural) + "/s"
	case rate < 1000000:
		return wslCountLabel(int(rate+500)/1000, "thousand "+plural, "thousand "+plural) + "/s"
	default:
		return wslCountLabel(int(rate+500000)/1000000, "million "+plural, "million "+plural) + "/s"
	}
}

// wslDurationLabel renders a measured duration at a precision that matches what a human can compare:
// milliseconds for anything under a second, and hundredths of a second above it. It never renders a
// positive duration as "0 ms", because a pass that took time did not take none.
func wslDurationLabel(duration time.Duration) string {
	switch {
	case duration <= 0:
		return "not timed"
	case duration < time.Millisecond:
		return fmt.Sprintf("%.1f ms", float64(duration.Microseconds())/1000)
	case duration < time.Second:
		return fmt.Sprintf("%.0f ms", duration.Seconds()*1000)
	default:
		return fmt.Sprintf("%.2f s", duration.Seconds())
	}
}
