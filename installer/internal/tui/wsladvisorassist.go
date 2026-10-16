package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// ============================================================================
// THE TWO ASSISTANT SCREENS
// ============================================================================
//
// The benchmark screen measures and the copy screen copies, and they are the only two places in this
// utility where the installer does anything at all. Both are shaped the same way as the rest of the
// section -- a body of facts, a short list of rows, the legend at the bottom -- and both say what
// they are about to do before they do it, because a measurement writes to the user's disk and a copy
// is the one action in this utility whose result outlives the installer.
//
// The rows carry the meaning and the body carries the detail, exactly as on the report: a row that
// says "copy to ~/projects — 41.2 GiB free" is a decision someone can make, and a row that says "yes"
// is not.

// wslActionKind says what pressing a row of an assistant screen means.
type wslActionKind int

const (
	// wslActionSeparator is a rule; the cursor never rests on it.
	wslActionSeparator wslActionKind = iota
	// wslActionRun starts the screen's own operation.
	wslActionRun
	// wslActionPick chooses the listed item at Index.
	wslActionPick
	// wslActionConfirm is a confirmation that writes something.
	wslActionConfirm
	// wslActionRestart returns to the first question of the copy.
	wslActionRestart
	// wslActionCancel stops the operation in flight.
	wslActionCancel
	// wslActionBack leaves the screen or steps back one question.
	wslActionBack
)

// wslActionRow is one row of either assistant screen: what it means, what it says, and which item of
// the current list it chooses.
type wslActionRow struct {
	Kind  wslActionKind
	Label string
	Index int
}

// wslActionRowLabels turns the rows into the labels the menu draws.
func wslActionRowLabels(rows []wslActionRow) []string {
	labels := make([]string, 0, len(rows))
	for _, row := range rows {
		labels = append(labels, row.Label)
	}
	return labels
}

// wslActionRowAt is the row the cursor is on, with the guard a rule needs.
func wslActionRowAt(rows []wslActionRow, cursor int) (wslActionRow, bool) {
	if cursor < 0 || cursor >= len(rows) {
		return wslActionRow{}, false
	}
	row := rows[cursor]
	if row.Kind == wslActionSeparator {
		return wslActionRow{}, false
	}
	return row, true
}

// wslMoveCursor moves the cursor over one row list, stepping over the rules. It is the movement every
// list in this installer uses, so a rule can never be selected and Enter can never be pressed on a
// line.
func wslMoveCursor(rows []wslActionRow, cursor int, delta int) int {
	if len(rows) == 0 {
		return 0
	}
	next := cursor + delta
	for next >= 0 && next < len(rows) && rows[next].Kind == wslActionSeparator {
		next += delta
	}
	if next < 0 || next >= len(rows) {
		return cursor
	}
	return next
}

// ============================================================================
// THE BENCHMARK
// ============================================================================

// renderWSLBenchmark draws the measurement screen.
func (m Model) renderWSLBenchmark() string {
	return m.renderThemeScreen(m.wslBenchmarkDescription(), m.wslBenchmarkNotice(),
		[]installerHint{hintUp, hintDown, hintSelect, hintBack})
}

// wslBenchmarkNotice is the line above the list: the result of the last press, which is where a
// measurement in flight and its outcome belong, so the body below keeps its size while it runs.
func (m Model) wslBenchmarkNotice() string {
	state := m.WSLAdvisor.Benchmark
	if state.Running {
		if state.Notice != "" {
			return "Measuring storage… " + state.Notice
		}
		return "Measuring storage…"
	}
	return state.Notice
}

// wslBenchmarkRows are the screen's rows: the one press that measures, and the way back. The
// measurements themselves are facts, and facts live in the body where they can be read at length.
func (m Model) wslBenchmarkRows() []wslActionRow {
	label := "Measure the storage now"
	if m.WSLAdvisor.Benchmark.Ran {
		label = "Measure the storage again"
	}
	return []wslActionRow{
		{Kind: wslActionRun, Label: label},
		{Kind: wslActionSeparator, Label: m.menuSeparator()},
		{Kind: wslActionBack, Label: "← Back to the report"},
	}
}

// wslBenchmarkDescription is what the screen says: what will be written, and then what was measured.
func (m Model) wslBenchmarkDescription() []string {
	state := m.WSLAdvisor.Benchmark
	limits := system.DefaultBenchLimits()
	targets := m.wslBenchmarkTargets()
	offer := m.wslBenchmarkOffer()

	var paragraphs []string
	if !offer.Offered {
		// The row that opened this screen only exists when the run can finish, so this paragraph is
		// the one case where the answer changed between drawing the report and pressing it: a
		// directory that was there when the report was taken and is gone now.
		return []string{offer.Reason}
	}

	paragraphs = append(paragraphs, fmt.Sprintf(
		"The measurement writes its own scratch directory inside each target, reads it back, states "+
			"it, creates many small files and removes them again. One run writes %d files of %s into "+
			"each target's scratch directory, inside a whole-run bound of %s and a ceiling of %s in one "+
			"target, and it removes everything it made -- including after a failure or the bound being "+
			"reached, which its own report states.",
		limits.Files, humanBytes(uint64(limits.FileSize)), wslDurationLabel(limits.Timeout),
		humanBytes(uint64(limits.MaxBytes))))

	var names []string
	for _, target := range targets {
		names = append(names, target.Name+" ("+target.Path+")")
	}
	paragraphs = append(paragraphs, "Targets, measured in this order: "+strings.Join(names, "; ")+".")

	paragraphs = append(paragraphs, "The numbers compare these paths on this machine at this moment. "+
		"They are not a promise about a build, a test suite or a Git operation, and the report claims no "+
		"improvement it did not measure: a faster copy here does not prove that anything the project "+
		"does will be faster.")

	if !state.Ran {
		if state.Running {
			paragraphs = append(paragraphs, "The run is in flight. Everything it wrote is removed "+
				"before it reports, so the answer is the last thing to arrive.")
		} else {
			paragraphs = append(paragraphs, "Nothing has been measured yet. The row above starts the "+
				"run; nothing is written until it is pressed.")
		}
		return paragraphs
	}

	paragraphs = append(paragraphs, wslBenchmarkResultsParagraphs(state.Report)...)
	return paragraphs
}

// wslBenchmarkResultsParagraphs renders the report: one paragraph per target with its operations,
// then what it could not measure, then what the run admits about itself.
func wslBenchmarkResultsParagraphs(report system.BenchReport) []string {
	var paragraphs []string
	for _, result := range report.Results {
		header := result.Target.Name + " (" + result.Target.Path + ")"
		if result.Err != "" {
			paragraphs = append(paragraphs, header+" could not be measured: "+result.Err+".")
			continue
		}
		var lines []string
		for _, measurement := range result.Measurements {
			lines = append(lines, fmt.Sprintf("%s in %s", wslOperationLabel(measurement.Operation),
				wslBenchRateLabel(measurement)))
		}
		if len(lines) == 0 {
			paragraphs = append(paragraphs, header+" produced no measurement.")
		} else {
			paragraphs = append(paragraphs, header+": "+strings.Join(lines, "; ")+".")
		}
		if len(result.Notes) > 0 {
			paragraphs = append(paragraphs, "What "+result.Target.Name+" did not measure: "+
				strings.Join(result.Notes, " ")+".")
		}
	}

	if len(report.Results) == 1 {
		paragraphs = append(paragraphs, "Only one target could be measured, so there is nothing to "+
			"compare it with: a number on its own says how fast this path is, not whether another would "+
			"be faster.")
	}

	cleaned := "Every scratch directory this run created was removed."
	if !report.Cleaned {
		cleaned = "A scratch directory this run created is still on disk: the run says so rather than " +
			"reporting a clean disk it cannot prove, and its own report names the directory it could not " +
			"remove."
	}
	paragraphs = append(paragraphs, cleaned+" The whole run took "+wslDurationLabel(report.Duration)+".")

	if len(report.Contamination) > 0 {
		paragraphs = append(paragraphs, "What muddies these numbers: "+strings.Join(report.Contamination, " "))
	}
	if len(report.Notes) > 0 {
		paragraphs = append(paragraphs, strings.Join(report.Notes, " "))
	}
	return paragraphs
}

// handleWSLBenchmarkKeys drives the measurement screen.
func (m Model) handleWSLBenchmarkKeys(key string) (tea.Model, tea.Cmd) {
	rows := m.wslBenchmarkRows()
	switch key {
	case "up", "k":
		m.Cursor = wslMoveCursor(rows, m.Cursor, -1)
	case "down", "j":
		m.Cursor = wslMoveCursor(rows, m.Cursor, 1)
	case "enter", " ":
		row, ok := wslActionRowAt(rows, m.Cursor)
		if !ok {
			break
		}
		switch row.Kind {
		case wslActionRun:
			if m.WSLAdvisor.Benchmark.Running {
				break
			}
			m.WSLAdvisor.Benchmark.Running = true
			m.WSLAdvisor.Benchmark.Notice = "the run is bounded, and it removes what it writes"
			return m, runWSLBenchmarkCmd(m.wslBenchmarkTargets())
		case wslActionBack:
			m.Screen = ScreenWSLAdvisor
			m.Cursor = 0
		}
	}
	return m, nil
}

// ============================================================================
// THE ASSISTED COPY
// ============================================================================

// renderWSLMigration draws whichever question of the copy the screen is on.
func (m Model) renderWSLMigration() string {
	return m.renderThemeScreen(m.wslMigrationDescription(), m.wslMigrationNotice(),
		[]installerHint{hintUp, hintDown, hintSelect, hintBack})
}

// wslMigrationNotice is the line above the list: the progress of a copy in flight, or the result of
// the last press. The progress lives here rather than in the body so the rows below keep their place
// while hundreds of entries go past.
func (m Model) wslMigrationNotice() string {
	state := m.WSLAdvisor.Migration
	switch state.Phase {
	case wslMigrateCopying:
		if state.ProgressSeen {
			line := "Copying " + wslCountProgress(state.Progress)
			if state.Progress.Path != "" {
				line += " — " + state.Progress.Path
			}
			return line
		}
		return "Copying: the first entries are on their way. Nothing is deleted from the source."
	case wslMigrateReadingSource:
		return "Reading the source to count what a copy would take. Nothing is written."
	}
	return state.Notice
}

// wslMigrationRows are the current phase's rows.
func (m Model) wslMigrationRows() []wslActionRow {
	state := m.WSLAdvisor.Migration
	switch state.Phase {
	case wslMigratePickSource:
		rows := make([]wslActionRow, 0, len(state.Sources)+2)
		for i, source := range state.Sources {
			rows = append(rows, wslActionRow{
				Kind:  wslActionPick,
				Index: i,
				Label: source.Label + " — " + source.Storage.Kind.String(),
			})
		}
		rows = append(rows,
			wslActionRow{Kind: wslActionSeparator, Label: m.menuSeparator()},
			wslActionRow{Kind: wslActionBack, Label: "← Back to the report"})
		return rows

	case wslMigratePickDestination:
		rows := make([]wslActionRow, 0, len(state.Destinations)+2)
		for i, destination := range state.Destinations {
			rows = append(rows, wslActionRow{
				Kind:  wslActionPick,
				Index: i,
				Label: destination.Label,
			})
		}
		rows = append(rows,
			wslActionRow{Kind: wslActionSeparator, Label: m.menuSeparator()},
			wslActionRow{Kind: wslActionBack, Label: "← Choose another project"})
		return rows

	case wslMigratePreview:
		// The confirmation exists only when the plan is runnable: a blocked plan shows its blockers
		// with no row that could start it, so a blocker cannot be pressed past.
		if !state.Plan.Runnable() {
			return []wslActionRow{
				{Kind: wslActionSeparator, Label: m.menuSeparator()},
				{Kind: wslActionBack, Label: "← Choose another destination"},
			}
		}
		return []wslActionRow{
			{Kind: wslActionConfirm, Label: "Copy and verify now"},
			{Kind: wslActionCancel, Label: "Cancel"},
			{Kind: wslActionSeparator, Label: m.menuSeparator()},
			{Kind: wslActionBack, Label: "← Choose another destination"},
		}

	case wslMigrateCopying:
		return []wslActionRow{
			{Kind: wslActionCancel, Label: "Stop the copy"},
			{Kind: wslActionSeparator, Label: m.menuSeparator()},
			{Kind: wslActionBack, Label: "← Back to the report"},
		}

	case wslMigrateDone:
		return []wslActionRow{
			{Kind: wslActionRestart, Label: "Copy another project"},
			{Kind: wslActionSeparator, Label: m.menuSeparator()},
			{Kind: wslActionBack, Label: "← Back to the report"},
		}
	}
	return nil
}

// wslMigrationDescription is what the current phase says.
func (m Model) wslMigrationDescription() []string {
	state := m.WSLAdvisor.Migration
	switch state.Phase {
	case wslMigratePickSource:
		return wslMigrationSourceParagraphs(state)
	case wslMigratePickDestination:
		return wslMigrationDestinationParagraphs(state)
	case wslMigrateReadingSource:
		source, ok := m.wslMigrationChosenSource()
		if !ok {
			return []string{"Reading the source. Nothing is written."}
		}
		return []string{fmt.Sprintf("Reading %s to count what a copy would take. This reads every entry "+
			"and writes nothing: the preview that follows is a description, and the engine refuses to "+
			"run one that is not runnable.", source.Path)}
	case wslMigratePreview:
		return wslMigrationPreviewParagraphs(state)
	case wslMigrateCopying:
		return wslMigrationCopyingParagraphs(state)
	case wslMigrateDone:
		return wslMigrationOutcomeParagraphs(state)
	}
	return nil
}

// wslMigrationSourceParagraphs introduces the first question.
func wslMigrationSourceParagraphs(state wslMigrationState) []string {
	if len(state.Sources) == 0 {
		return []string{"No readable project directory was found to copy."}
	}
	return []string{
		"The assisted copy starts by choosing a directory. The list leads with the projects that live " +
			"on the Windows host, because every metadata operation there crosses the boundary; the " +
			"storage each one is on is named on its own row, from the filesystem its mount exposes and " +
			"never from its path.",
		"Nothing has been written or read beyond the report's own listing. This choice does not copy " +
			"anything: the next screen chooses where the copy would land, and the screen after that is a " +
			"preview that has to be confirmed.",
	}
}

// wslMigrationDestinationParagraphs introduces the second question.
func wslMigrationDestinationParagraphs(state wslMigrationState) []string {
	paragraphs := []string{
		"Every destination here is on the distribution's own filesystem, judged by the filesystem each " +
			"mount exposes. A destination on the Windows side is not offered: a copy that landed there " +
			"would recreate the cost this utility exists to remove.",
	}
	if len(state.Destinations) == 0 {
		return append(paragraphs, "No destination on the distribution's own storage was found for this "+
			"project, so there is nowhere to copy it to. Nothing has been changed.")
	}
	paragraphs = append(paragraphs,
		"A copy goes into a directory of its own inside the destination: nothing on either side is "+
			"overwritten, and nothing in the source is deleted, moved or renamed. Choosing a row here "+
			"reads the source and shows the preview; it still copies nothing.",
		"Left behind by default, and counted in the preview: "+
			strings.Join(wslMigrationDefaultExclusions, ", ")+". Every one of them is regenerable from the "+
			"source's own manifest, and the preview names the entries the pattern matched rather than the "+
			"intention behind it.")
	return paragraphs
}

// wslMigrationPreviewParagraphs is the confirmation's own account of what it would do.
func wslMigrationPreviewParagraphs(state wslMigrationState) []string {
	if state.PlanErr != "" {
		return []string{"The preview could not be read: " + state.PlanErr + ". Nothing has been written."}
	}
	plan := state.Plan
	source, destination := plan.Source, plan.Destination

	paragraphs := []string{fmt.Sprintf("Copy %s to %s?", source, destination)}
	if len(plan.Blockers) > 0 {
		paragraphs = append(paragraphs, "This copy cannot run: "+strings.Join(plan.Blockers, " ")+
			" Nothing has been written, and there is no row here that could start it.")
	} else {
		paragraphs = append(paragraphs, "Would be copied: "+plan.Stats.Summary()+".")
	}

	if plan.SpaceKnown {
		estimate := "The copy would take about " + humanBytes(plan.Stats.Bytes) +
			", which counts the files' sizes and not the block rounding a filesystem applies."
		if plan.Stats.Bytes > plan.FreeBytes {
			estimate = "The copy would take about " + humanBytes(plan.Stats.Bytes) + ", and only " +
				humanBytes(plan.FreeBytes) + " is free behind the destination."
		}
		paragraphs = append(paragraphs, estimate+" "+humanBytes(plan.FreeBytes)+" is free behind "+
			destination+".")
	} else {
		paragraphs = append(paragraphs, "The space behind the destination could not be read, so this "+
			"preview cannot say whether the copy would fit.")
	}

	if len(plan.Exclusions) > 0 {
		paragraphs = append(paragraphs, "Left behind: "+strings.Join(plan.Exclusions, ", ")+".")
	}
	if len(plan.Warnings) > 0 {
		paragraphs = append(paragraphs, "Worth knowing: "+strings.Join(plan.Warnings, " "))
	}

	if len(plan.Instructions) > 0 {
		paragraphs = append(paragraphs, "The same work by hand, quoted and never executed by this "+
			"installer: "+strings.Join(plan.Instructions, " "))
	}

	paragraphs = append(paragraphs, "The source is not modified: nothing is deleted, moved or "+
		"renamed, and every byte is written into the destination directory above. After the copy, each "+
		"copied file is compared against the source; a difference is reported, never repaired quietly.")
	return paragraphs
}

// wslMigrationCopyingParagraphs is what the screen says while the copy runs.
func wslMigrationCopyingParagraphs(state wslMigrationState) []string {
	paragraphs := []string{
		"The copy is in flight. It writes only inside the destination directory and never opens a " +
			"source path for writing. Stopping it leaves the entries already copied where they are: this " +
			"installer does not delete a partial copy, because a half-copy is evidence about what went " +
			"wrong and removing it would destroy that evidence.",
	}
	if state.ProgressSeen && state.Progress.Total > 0 {
		paragraphs = append(paragraphs, fmt.Sprintf("%d of %d entries have been placed.",
			state.Progress.Done, state.Progress.Total))
	}
	return paragraphs
}

// wslMigrationOutcomeParagraphs is the run's own report, verified or not.
func wslMigrationOutcomeParagraphs(state wslMigrationState) []string {
	report := state.Report
	var paragraphs []string

	if state.ReportErr != "" {
		paragraphs = append(paragraphs, "The copy stopped: "+state.ReportErr+".")
	}
	if report.Cancelled {
		paragraphs = append(paragraphs, "The copy was stopped before it finished. What was copied "+
			"before that is still in "+report.Destination+", and the source is exactly as it was.")
	}

	switch {
	case report.Verified:
		paragraphs = append(paragraphs, "Verified: every copied entry matches the source. "+
			humanBytes(report.VerifiedBytes)+" were compared byte-for-byte.")
	case len(report.Failures) > 0:
		paragraphs = append(paragraphs, "Not verified: the copy has differences against the source, "+
			"named below. Nothing has been deleted or changed in the source, and nothing was repaired "+
			"silently.")
	case state.ReportErr == "" && !report.Cancelled:
		paragraphs = append(paragraphs, "The copy finished but could not be verified against the "+
			"source, so this screen does not claim it matches.")
	}

	paragraphs = append(paragraphs, "Placed: "+report.Copied.Summary()+" in "+report.Destination+".")
	if report.SizeOnlyFiles > 0 {
		paragraphs = append(paragraphs, fmt.Sprintf("%d files were larger than the per-file comparison "+
			"budget and were covered by size and mode alone; the report states that as a limit rather "+
			"than counting them as compared.", report.SizeOnlyFiles))
	}
	if len(report.VerificationNotes) > 0 {
		paragraphs = append(paragraphs, strings.Join(report.VerificationNotes, " "))
	}
	if len(report.Failures) > 0 {
		paragraphs = append(paragraphs, "Differences and failures: "+strings.Join(report.Failures, " "))
	}
	paragraphs = append(paragraphs, "The source was never written to: this installer does not delete, "+
		"move or rename anything, and no instruction below has been run.")

	if len(report.Instructions) > 0 {
		paragraphs = append(paragraphs, "To finish by hand, when you have checked the copy yourself: "+
			strings.Join(report.Instructions, " "))
	}
	if len(report.VerificationNotes) == 0 && len(report.Failures) == 0 && !report.Verified {
		paragraphs = append(paragraphs, "Nothing in the source has been changed.")
	}
	return paragraphs
}

// handleWSLMigrationKeys drives the copy's phases.
func (m Model) handleWSLMigrationKeys(key string) (tea.Model, tea.Cmd) {
	migration := &m.WSLAdvisor.Migration
	rows := m.wslMigrationRows()

	switch key {
	case "up", "k":
		m.Cursor = wslMoveCursor(rows, m.Cursor, -1)
		return m, nil
	case "down", "j":
		m.Cursor = wslMoveCursor(rows, m.Cursor, 1)
		return m, nil
	}

	if key != "enter" && key != " " {
		return m, nil
	}
	row, ok := wslActionRowAt(rows, m.Cursor)
	if !ok {
		return m, nil
	}

	switch row.Kind {
	case wslActionPick:
		return m.wslMigrationPick(row.Index)
	case wslActionConfirm:
		return m.startWSLMigrationCopy()
	case wslActionCancel:
		if migration.Running {
			return m.cancelWSLMigration(), nil
		}
		// The preview's cancel is the safe default: it puts the choice of destination back.
		migration.Phase = wslMigratePickDestination
		m.Cursor = migration.Destination
		return m, nil
	case wslActionRestart:
		migration.Phase = wslMigratePickSource
		migration.Report = system.MigrationReport{}
		migration.ReportErr = ""
		migration.ProgressSeen = false
		migration.Notice = ""
		m.Cursor = 0
		return m, nil
	case wslActionBack:
		migration.Notice = ""
		switch migration.Phase {
		case wslMigratePickSource, wslMigrateDone:
			m.Screen = ScreenWSLAdvisor
			m.Cursor = 0
		case wslMigratePickDestination, wslMigratePreview:
			migration.Phase = wslMigratePickSource
			m.Cursor = migration.Source
		case wslMigrateReadingSource:
			migration.Phase = wslMigratePickDestination
			m.Cursor = migration.Destination
		case wslMigrateCopying:
			// Leaving a running copy is not offered as "back": the row says "back to the report", and
			// the copy keeps running with its answer landing on the model, exactly as a measurement
			// does. Stopping it is its own row.
			m.Screen = ScreenWSLAdvisor
			m.Cursor = 0
		}
		return m, nil
	}
	return m, nil
}

// wslMigrationPick advances the flow from one of the two lists.
func (m Model) wslMigrationPick(index int) (tea.Model, tea.Cmd) {
	migration := &m.WSLAdvisor.Migration
	switch migration.Phase {
	case wslMigratePickSource:
		if index < 0 || index >= len(migration.Sources) {
			return m, nil
		}
		migration.Source = index
		migration.Destinations = m.wslMigrationDestinations(migration.Sources[index].Path)
		migration.Destination = 0
		migration.Phase = wslMigratePickDestination
		m.Cursor = 0
		return m, nil
	case wslMigratePickDestination:
		if index < 0 || index >= len(migration.Destinations) {
			return m, nil
		}
		source, ok := m.wslMigrationChosenSource()
		if !ok {
			return m, nil
		}
		migration.Destination = index
		migration.Phase = wslMigrateReadingSource
		migration.Plan = system.MigrationPlan{}
		migration.PlanErr = ""
		m.Cursor = 0
		return m, planWSLMigrationCmd(source.Path, migration.Destinations[index].Path)
	}
	return m, nil
}

// cancelWSLMigration stops a copy in flight. The report still arrives and is shown: a stopped copy
// has an outcome, and saying what was placed before the stop is the honest version of it.
func (m Model) cancelWSLMigration() Model {
	migration := &m.WSLAdvisor.Migration
	if migration.Cancel != nil {
		migration.Cancel()
		migration.Cancel = nil
	}
	migration.Notice = "Stopping the copy. What was already placed stays where it is."
	return m
}

// wslMigrationEscape is what Escape does on the copy screen: one question back at every phase, and
// a stop when a copy is running.
func (m Model) wslMigrationEscape() (tea.Model, tea.Cmd) {
	migration := &m.WSLAdvisor.Migration
	switch migration.Phase {
	case wslMigratePickSource, wslMigrateDone:
		m.Screen = ScreenUtilities
		m.Cursor = 0
		m.WSLAdvisor.Notice = ""
	case wslMigratePickDestination, wslMigratePreview:
		migration.Phase = wslMigratePickSource
		m.Cursor = migration.Source
	case wslMigrateReadingSource:
		migration.Phase = wslMigratePickDestination
		m.Cursor = migration.Destination
	case wslMigrateCopying:
		// Escape during a copy stops it rather than leaving it running behind a screen the user has
		// left: the copy is the one action here that keeps costing something while it runs.
		return m.cancelWSLMigration(), nil
	}
	return m, nil
}
