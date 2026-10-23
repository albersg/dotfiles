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

// ============================================================================
// THE KEYS
// ============================================================================

// The letters the two assistant screens' command rows carry. A run, a confirmation, a cancel, a stop
// and a restart are commands rather than openings, so each runs on its own letter -- said on its row
// and named in the footer -- and Enter is left to the one row on these screens that opens something,
// the way back. No letter here is a move key (there is no j, k, h or l), and no letter carries two
// commands: the preview's cancel and the copy's stop are deliberately different keys.
const (
	wslMeasureKey = "m"
	wslCopyKey    = "c"
	wslCancelKey  = "x"
	wslStopKey    = "s"
	wslAnotherKey = "a"
)

// The same letters as the footer spells them. The hint is a literal so the grammar guard in
// key_grammar_test.go reads it, and the test beside each screen presses the letter its row declares,
// so the row, the footer and the handler cannot drift apart.
var (
	// hintWSLMeasure starts a measurement, and repeats the last one.
	hintWSLMeasure = installerHint{"[m]", "measure"}
	// hintWSLCopyNow is the preview's one writing press.
	hintWSLCopyNow = installerHint{"[c]", "copy"}
	// hintWSLCancelCopy leaves the preview for the choice of destination.
	hintWSLCancelCopy = installerHint{"[x]", "cancel"}
	// hintWSLStopCopy stops a copy in flight. hintWSLStopCopyEscape is the same stop on the key
	// Escape carries while a copy runs, and the footer offers it in place of the way back so it says
	// which of the two Escape is doing.
	hintWSLStopCopy       = installerHint{"[s]", "stop the copy"}
	hintWSLStopCopyEscape = installerHint{"[Esc]", "stop the copy"}
	// hintWSLAnotherCopy starts the flow again from its first question.
	hintWSLAnotherCopy = installerHint{"[a]", "copy another"}
)

// wslCommandRow draws a row that acts where it stands: the key that runs it, in brackets, in front of
// the wording. It is the shape the Utilities section's own command rows carry, so a reader who has
// learnt one list has learnt the other, and the row itself says the press is a letter.
func wslCommandRow(key, label string) string { return "[" + key + "] " + label }

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

// renderWSLBenchmark draws the measurement screen. The footer names the letter that measures and the
// Enter that opens the way back; Space is not offered, because nothing on this screen is a switch.
func (m Model) renderWSLBenchmark() string {
	return m.renderThemeScreen(Model.wslBenchmarkDescription, m.wslBenchmarkNotice(), m.wslBenchmarkHints())
}

// wslBenchmarkHints is the measurement screen's footer: the two move keys, the Enter that opens the
// way back, the letter that measures, and the Escape that is the same way back.
// hintWSLBenchmarkStop is the key that ends a measurement in flight: Escape
// stops a run, and the footer names it while the run holds the disk.
var hintWSLBenchmarkStop = installerHint{"[Esc]", "stop the measurement"}

func (m Model) wslBenchmarkHints() []installerHint {
	if m.WSLAdvisor.Benchmark.Running {
		// Escape stops the run while it is in flight, and the footer says so
		// instead of offering the way back: the back row still opens with Enter.
		return []installerHint{hintUp, hintDown, hintOpen, hintWSLBenchmarkStop}
	}
	return []installerHint{hintUp, hintDown, hintOpen, hintWSLMeasure, hintBack}
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
// The measuring row acts where it stands, so it carries its letter; the way back opens, so it carries
// the arrow the utilities section draws on the rows that do.
func (m Model) wslBenchmarkRows() []wslActionRow {
	label := "Measure the storage now"
	if m.WSLAdvisor.Benchmark.Ran {
		label = "Measure the storage again"
	}
	return []wslActionRow{
		{Kind: wslActionRun, Label: wslCommandRow(wslMeasureKey, label)},
		{Kind: wslActionSeparator, Label: m.menuSeparator()},
		{Kind: wslActionBack, Label: "← Back to the report"},
	}
}

// wslBenchmarkDescription is the measurement screen's compact body, derived from the facts the wide
// frame draws: what will be written, and then what was measured.
func (m Model) wslBenchmarkDescription() []string {
	return factProse(m.wslBenchmarkFacts())
}

// wslBenchmarkFacts is that answer as one list of facts, which is what the compact body's prose is
// derived from: a flatten of wslBenchmarkAreas, so the floor and the card read one list.
func (m Model) wslBenchmarkFacts() []iconFact {
	return factAreaFacts(m.wslBenchmarkAreas())
}

// wslBenchmarkAreas is the measurement screen's detail as areas: what a run writes and removes, the
// targets in the order they are measured, the run's own state, and then one area per measured target
// with one row per operation. The operations used to be five clauses of a single sentence per target
// ("write in 420 ms, 150 files/s, 152.4 KiB/s; read back in 96 ms, ..."), which is the shape a reader
// cannot scan down a column; each is a row now, so the two targets' stat rows sit beside each other.
func (m Model) wslBenchmarkAreas() []factArea {
	state := m.WSLAdvisor.Benchmark
	limits := system.DefaultBenchLimits()
	targets := m.wslBenchmarkTargets()
	offer := m.wslBenchmarkOffer()

	if !offer.Offered {
		// The row that opened this screen only exists when the run can finish, so this fact is
		// the one case where the answer changed between drawing the report and pressing it: a
		// directory that was there when the report was taken and is gone now.
		return []factArea{{Facts: []iconFact{{Icon: wslFactIconLimit, Label: "Measurement", Value: offer.Reason}}}}
	}

	written := []iconFact{
		{Icon: wslFactIconDisk, Label: "Writes", Value: fmt.Sprintf("%d files of %s into each target's own scratch directory",
			limits.Files, humanBytes(uint64(limits.FileSize)))},
		{Icon: wslFactIconDisk, Label: "Removes", Value: "everything it wrote, including after a failure"},
		{Icon: wslFactIconLimit, Label: "Bound", Value: fmt.Sprintf("%s for the run, %s in one target",
			wslDurationLabel(limits.Timeout), humanBytes(uint64(limits.MaxBytes)))},
	}

	targetRows := []iconFact{{Icon: wslFactIconFolder, Label: "Order", Value: "measured from top to bottom"}}
	for _, target := range targets {
		targetRows = append(targetRows, iconFact{Icon: wslFactIconFolder, Label: target.Name, Value: target.Path})
	}

	areas := []factArea{
		{Name: "The measurement", Facts: written},
		{Name: "Targets", Facts: targetRows},
		// The numbers are not a promise, and the row says so in one clause instead of a
		// paragraph: a faster copy here is not a faster build.
		{Name: "Read the numbers as", Facts: []iconFact{
			{Icon: wslFactIconWhy, Label: "Scope", Value: "this machine, these paths, this moment"},
			{Icon: wslFactIconLimit, Label: "Not a promise", Value: "no build, test suite or Git operation was measured"},
		}},
	}

	if !state.Ran {
		run := "nothing measured yet; the row above starts the run"
		if state.Running {
			run = "in flight; everything it wrote is removed before it reports"
		}
		return append(areas, factArea{Name: "Run", Facts: []iconFact{
			{Icon: wslFactIconBenefit, Label: "Run", Value: run},
		}})
	}

	return append(areas, wslBenchmarkResultsAreas(state.Report)...)
}

// wslBenchmarkResultsParagraphs renders the report as the compact body's paragraphs. It is the prose
// of wslBenchmarkResultsFacts, so the two shapes report one measurement the same way.
func wslBenchmarkResultsParagraphs(report system.BenchReport) []string {
	return factProse(wslBenchmarkResultsFacts(report))
}

// wslBenchmarkResultsFacts is the report as one list of facts, for its own prose.
func wslBenchmarkResultsFacts(report system.BenchReport) []iconFact {
	return factAreaFacts(wslBenchmarkResultsAreas(report))
}

// wslBenchmarkResultsAreas renders the report as areas: one per target -- its name on the divider,
// one row per operation, the notes the run took under it -- and then what the run admits about
// itself.
func wslBenchmarkResultsAreas(report system.BenchReport) []factArea {
	areas := make([]factArea, 0, len(report.Results)+2)
	for _, result := range report.Results {
		header := result.Target.Name
		if result.Err != "" {
			areas = append(areas, factArea{Name: header, Facts: []iconFact{
				{Icon: wslFactIconLimit, Label: "Failed", Value: factClause(result.Err)},
			}})
			continue
		}
		rows := make([]iconFact, 0, len(result.Measurements)+len(result.Notes))
		for _, measurement := range result.Measurements {
			rows = append(rows, iconFact{Icon: wslFactIconMemory,
				Label: wslFactLabel(wslOperationLabel(measurement.Operation)),
				Value: wslBenchRateLabel(measurement)})
		}
		if len(rows) == 0 {
			rows = append(rows, iconFact{Icon: wslFactIconLimit, Label: "Measured", Value: "no measurement"})
		}
		for _, note := range result.Notes {
			rows = append(rows, iconFact{Icon: wslFactIconLimit, Label: "Not measured", Value: factClause(note)})
		}
		rows = append(rows, iconFact{Icon: wslFactIconFolder, Label: "Path", Value: result.Target.Path})
		areas = append(areas, factArea{Name: header, Facts: rows})
	}

	var run []iconFact
	if len(report.Results) == 1 {
		run = append(run, iconFact{Icon: wslFactIconLimit, Label: "Comparison",
			Value: "one target, so there is nothing to compare it with"})
	}
	cleaned := "every scratch directory this run created was removed"
	if !report.Cleaned {
		cleaned = "a scratch directory is still on disk; the report names it"
	}
	run = append(run, iconFact{Icon: wslFactIconDisk, Label: "Disk", Value: cleaned})
	run = append(run, iconFact{Icon: wslFactIconShell, Label: "Took", Value: wslDurationLabel(report.Duration)})
	for _, contamination := range report.Contamination {
		run = append(run, iconFact{Icon: wslFactIconLimit, Label: "Contamination", Value: factClause(contamination)})
	}
	for _, note := range report.Notes {
		run = append(run, iconFact{Icon: wslFactIconLimit, Label: "Note", Value: factClause(note)})
	}
	return append(areas, factArea{Name: "The run", Facts: run})
}

// wslFactLabel capitalises the first letter of a name the engine spells for a sentence, so a row's
// label reads like the label column above and below it.
func wslFactLabel(name string) string {
	if name == "" {
		return name
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

// handleWSLBenchmarkKeys drives the measurement screen. The measurement runs on its own letter and
// Enter only opens the way back: a press that writes to the disk is not an opening. Escape is the
// way back too, handled by the shared key switch, and it leaves the run to finish where it is -- the
// screen's own body says the report lands here even when the reader has stepped away.
func (m Model) handleWSLBenchmarkKeys(key string) (tea.Model, tea.Cmd) {
	rows := m.wslBenchmarkRows()
	switch key {
	case "up", "k":
		m.Cursor = wslMoveCursor(rows, m.Cursor, -1)
	case "down", "j":
		m.Cursor = wslMoveCursor(rows, m.Cursor, 1)
	case wslMeasureKey:
		// A run already in flight is not started a second time: the screen says it is measuring
		// already, and the second press would spend the disk again for a number on its way.
		if m.WSLAdvisor.Benchmark.Running {
			break
		}
		return m, (&m).startWSLBenchmarkRun()
	case "enter":
		row, ok := wslActionRowAt(rows, m.Cursor)
		if !ok {
			break
		}
		if row.Kind == wslActionBack {
			m.Screen = ScreenWSLAdvisor
			m.Cursor = 0
		}
	}
	return m, nil
}

// ============================================================================
// THE ASSISTED COPY
// ============================================================================

// renderWSLMigration draws whichever question of the copy the screen is on. The footer is built from
// the phase, because the keys this screen accepts are: while a copy runs, Escape stops it rather than
// leaving the question, and the footer says so instead of offering the way back it is not taking.
func (m Model) renderWSLMigration() string {
	return m.renderThemeScreen(Model.wslMigrationDescription, m.wslMigrationNotice(), m.wslMigrationHints())
}

// wslMigrationHints is the footer for the question on screen. It names the letters the rows of that
// question carry and nothing else; the pick questions offer Enter as the choose it is, and the three
// that follow offer it as the opening of the way back.
func (m Model) wslMigrationHints() []installerHint {
	state := m.WSLAdvisor.Migration
	switch state.Phase {
	case wslMigratePickSource, wslMigratePickDestination:
		return []installerHint{hintUp, hintDown, hintChoose, hintBack}
	case wslMigratePreview:
		if !state.Plan.Runnable() {
			// A blocked plan offers no row that could start it, so the footer names no way to start
			// one either.
			return []installerHint{hintUp, hintDown, hintOpen, hintBack}
		}
		return []installerHint{hintUp, hintDown, hintOpen, hintWSLCopyNow, hintWSLCancelCopy, hintBack}
	case wslMigrateCopying:
		return []installerHint{hintUp, hintDown, hintOpen, hintWSLStopCopy, hintWSLStopCopyEscape}
	case wslMigrateDone:
		return []installerHint{hintUp, hintDown, hintOpen, hintWSLAnotherCopy, hintBack}
	}
	return []installerHint{hintUp, hintDown, hintBack}
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
			{Kind: wslActionConfirm, Label: wslCommandRow(wslCopyKey, "Copy and verify now")},
			{Kind: wslActionCancel, Label: wslCommandRow(wslCancelKey, "Cancel")},
			{Kind: wslActionSeparator, Label: m.menuSeparator()},
			{Kind: wslActionBack, Label: "← Choose another destination"},
		}

	case wslMigrateCopying:
		return []wslActionRow{
			{Kind: wslActionCancel, Label: wslCommandRow(wslStopKey, "Stop the copy")},
			{Kind: wslActionSeparator, Label: m.menuSeparator()},
			{Kind: wslActionBack, Label: "← Back to the report"},
		}

	case wslMigrateDone:
		return []wslActionRow{
			{Kind: wslActionRestart, Label: wslCommandRow(wslAnotherKey, "Copy another project")},
			{Kind: wslActionSeparator, Label: m.menuSeparator()},
			{Kind: wslActionBack, Label: "← Back to the report"},
		}
	}
	return nil
}

// wslMigrationDescription is the copy's compact body, derived from the facts the wide frame draws for
// whichever question the screen is on.
func (m Model) wslMigrationDescription() []string {
	return factProse(m.wslMigrationFacts())
}

// wslMigrationFacts is what the current phase says, as one list of facts, which is what the compact
// body's prose is derived from: a flatten of wslMigrationAreas.
func (m Model) wslMigrationFacts() []iconFact {
	return factAreaFacts(m.wslMigrationAreas())
}

// wslMigrationAreas is the assisted copy's detail as the areas its phase divides into. Every row is one
// short fact about the copy: what it writes, where it lands, what it leaves behind, and -- on the
// screens that ask a question -- nothing yet. The body used to state these as paragraphs ("Nothing has
// been written or read beyond the report's own listing. This choice does not copy anything: the next
// screen chooses where the copy would land, and the screen after that is a preview that has to be
// confirmed."), and a reader deciding whether to press a key had to read the whole paragraph to learn
// the one thing that mattered: that the press writes nothing.
func (m Model) wslMigrationAreas() []factArea {
	state := m.WSLAdvisor.Migration
	switch state.Phase {
	case wslMigratePickSource:
		return wslMigrationSourceAreas(state)
	case wslMigratePickDestination:
		return wslMigrationDestinationAreas(state)
	case wslMigrateReadingSource:
		source, ok := m.wslMigrationChosenSource()
		if !ok {
			return []factArea{{Facts: []iconFact{{Icon: wslFactIconLimit, Label: "Reading", Value: "nothing is written"}}}}
		}
		return []factArea{{Name: "Reading", Facts: []iconFact{
			{Icon: wslFactIconFolder, Label: "Source", Value: source.Path},
			{Icon: wslFactIconReadOnly, Label: "Writes", Value: "nothing: this counts what a copy would take"},
			{Icon: wslFactIconAction, Label: "Next", Value: "a preview that has to be confirmed"},
		}}}
	case wslMigratePreview:
		return wslMigrationPreviewAreas(state)
	case wslMigrateCopying:
		return wslMigrationCopyingAreas(state)
	case wslMigrateDone:
		return wslMigrationOutcomeAreas(state)
	}
	return nil
}

// wslMigrationSourceParagraphs introduces the first question as the compact body's paragraphs. It is
// the prose of wslMigrationSourceFacts.
func wslMigrationSourceParagraphs(state wslMigrationState) []string {
	return factProse(wslMigrationSourceFacts(state))
}

// wslMigrationSourceFacts introduces the first question.
func wslMigrationSourceFacts(state wslMigrationState) []iconFact {
	return factAreaFacts(wslMigrationSourceAreas(state))
}

// wslMigrationSourceAreas introduces the first question: what the list leads with, how the storage was
// judged, and that nothing has been written yet.
func wslMigrationSourceAreas(state wslMigrationState) []factArea {
	if len(state.Sources) == 0 {
		return []factArea{{Facts: []iconFact{{Icon: wslFactIconLimit, Label: "Sources",
			Value: "no readable project directory to copy"}}}}
	}
	return []factArea{{Name: "Choosing", Facts: []iconFact{
		{Icon: wslFactIconFolder, Label: "Order", Value: "projects on the Windows host first"},
		{Icon: wslFactIconDisk, Label: "Storage", Value: "named from the mount's filesystem, never from its path"},
		{Icon: wslFactIconReadOnly, Label: "Writes", Value: "nothing: this choice copies nothing"},
		{Icon: wslFactIconAction, Label: "Next", Value: "where the copy would land"},
	}}}
}

// wslMigrationDestinationParagraphs introduces the second question as the compact body's paragraphs.
// It is the prose of wslMigrationDestinationFacts.
func wslMigrationDestinationParagraphs(state wslMigrationState) []string {
	return factProse(wslMigrationDestinationFacts(state))
}

// wslMigrationDestinationFacts introduces the second question.
func wslMigrationDestinationFacts(state wslMigrationState) []iconFact {
	return factAreaFacts(wslMigrationDestinationAreas(state))
}

// wslMigrationDestinationAreas introduces the second question: where a copy may land, what the press
// does, and what is left behind by default.
func wslMigrationDestinationAreas(state wslMigrationState) []factArea {
	areas := []factArea{{Name: "Destinations", Facts: []iconFact{
		{Icon: wslFactIconDisk, Label: "Allowed", Value: "the distribution's own filesystem only"},
		{Icon: wslFactIconWhy, Label: "Judged by", Value: "the filesystem each mount exposes"},
		{Icon: wslFactIconLimit, Label: "Not offered", Value: "a destination on the Windows side"},
		{Icon: wslFactIconFolder, Label: "Copy", Value: "into a directory of its own; nothing overwritten, nothing deleted"},
	}}}
	if len(state.Destinations) == 0 {
		return append(areas, factArea{Name: "Nowhere to copy", Facts: []iconFact{
			{Icon: wslFactIconLimit, Label: "Nothing found",
				Value: "no destination on the distribution's own storage; nothing has been changed"},
		}})
	}
	return append(areas,
		factArea{Name: "Choosing", Facts: []iconFact{
			{Icon: wslFactIconReadOnly, Label: "Writes", Value: "nothing: a choice reads the source and shows the preview"},
		}},
		factArea{Name: "Left behind by default", Facts: []iconFact{
			{Icon: wslFactIconFolder, Label: "Exclusions", Value: strings.Join(wslMigrationDefaultExclusions, ", ")},
			{Icon: wslFactIconWhy, Label: "Why", Value: "each one is regenerable from the source's own manifest"},
		}},
	)
}

// wslMigrationPreviewParagraphs is the confirmation's own account of what it would do, as the compact
// body's paragraphs. It is the prose of wslMigrationPreviewFacts.
func wslMigrationPreviewParagraphs(state wslMigrationState) []string {
	return factProse(wslMigrationPreviewFacts(state))
}

// wslMigrationPreviewFacts is the confirmation's own account of what it would do.
func wslMigrationPreviewFacts(state wslMigrationState) []iconFact {
	return factAreaFacts(wslMigrationPreviewAreas(state))
}

// wslMigrationPreviewAreas is the confirmation's own account of what it would do: the question, what
// it would copy, whether it fits, what is left behind, what to know before pressing, the same work by
// hand, and the promise about the source. Each warning and each by-hand command is its own row, so a
// reader checks the one that applies instead of reading a paragraph of them.
func wslMigrationPreviewAreas(state wslMigrationState) []factArea {
	if state.PlanErr != "" {
		return []factArea{{Facts: []iconFact{{Icon: wslFactIconLimit, Label: "Preview",
			Value: "could not be read: " + state.PlanErr + "; nothing has been written"}}}}
	}
	plan := state.Plan
	source, destination := plan.Source, plan.Destination

	areas := []factArea{{Name: "The question", Facts: []iconFact{
		{Icon: wslFactIconAction, Label: "Confirm", Value: fmt.Sprintf("copy %s to %s", source, destination)},
	}}}

	if len(plan.Blockers) > 0 {
		blocked := make([]iconFact, 0, len(plan.Blockers)+1)
		for _, blocker := range plan.Blockers {
			blocked = append(blocked, iconFact{Icon: wslFactIconLimit, Label: "Blocked", Value: factClause(blocker)})
		}
		blocked = append(blocked, iconFact{Icon: wslFactIconReadOnly, Label: "Writes",
			Value: "nothing, and no row here could start it"})
		return append(areas, factArea{Name: "Cannot run", Facts: blocked})
	}

	areas[0].Facts = append(areas[0].Facts, iconFact{Icon: wslFactIconFolder, Label: "Would copy",
		Value: factClause(plan.Stats.Summary())})

	// The space is two numbers and one comparison: what the copy would take, and what is free behind
	// the destination. The block rounding a filesystem applies is the one caveat that belongs to the
	// number, so it keeps its own row beside it instead of becoming a clause of a sentence.
	if plan.SpaceKnown {
		free := humanBytes(plan.FreeBytes) + " free behind the destination"
		if plan.Stats.Bytes > plan.FreeBytes {
			free = "only " + humanBytes(plan.FreeBytes) + " free: it would not fit"
		}
		areas = append(areas, factArea{Name: "Space", Facts: []iconFact{
			{Icon: wslFactIconDisk, Label: "Would take", Value: "about " + humanBytes(plan.Stats.Bytes)},
			{Icon: wslFactIconDisk, Label: "Free", Value: free},
			{Icon: wslFactIconLimit, Label: "Counts", Value: "the files' sizes, not the block rounding"},
		}})
	} else {
		areas = append(areas, factArea{Name: "Space", Facts: []iconFact{
			{Icon: wslFactIconLimit, Label: "Not read", Value: "whether the copy would fit is unknown"},
		}})
	}

	if len(plan.Exclusions) > 0 {
		areas = append(areas, factArea{Name: "Left behind", Facts: []iconFact{
			{Icon: wslFactIconFolder, Label: "Excluded", Value: strings.Join(plan.Exclusions, ", ")},
		}})
	}
	if len(plan.Warnings) > 0 {
		rows := make([]iconFact, 0, len(plan.Warnings))
		for _, warning := range plan.Warnings {
			rows = append(rows, iconFact{Icon: wslFactIconLimit, Label: "Warning", Value: factClause(warning)})
		}
		areas = append(areas, factArea{Name: "Worth knowing", Facts: rows})
	}
	if len(plan.Instructions) > 0 {
		rows := make([]iconFact, 0, len(plan.Instructions))
		for _, instruction := range plan.Instructions {
			rows = append(rows, iconFact{Icon: wslFactIconAction, Label: "By hand", Value: instruction})
		}
		areas = append(areas, factArea{Name: "The same work by hand", Facts: rows})
	}

	return append(areas, factArea{Name: "The source", Facts: []iconFact{
		{Icon: wslFactIconReadOnly, Label: "Untouched", Value: "nothing deleted, moved or renamed"},
		{Icon: wslFactIconEvidence, Label: "Verified", Value: "each copied file compared against the source afterwards"},
	}})
}

// wslMigrationCopyingParagraphs is what the screen says while the copy runs, as the compact body's
// paragraphs. It is the prose of wslMigrationCopyingFacts.
func wslMigrationCopyingParagraphs(state wslMigrationState) []string {
	return factProse(wslMigrationCopyingFacts(state))
}

// wslMigrationCopyingFacts is what the screen says while the copy runs.
func wslMigrationCopyingFacts(state wslMigrationState) []iconFact {
	return factAreaFacts(wslMigrationCopyingAreas(state))
}

// wslMigrationCopyingAreas is what the screen says while the copy runs: where it writes, how far it is,
// and what stopping it does.
func wslMigrationCopyingAreas(state wslMigrationState) []factArea {
	rows := []iconFact{{Icon: wslFactIconWrites, Label: "In flight",
		Value: "writes only inside the destination, never the source"}}
	if state.ProgressSeen && state.Progress.Total > 0 {
		rows = append(rows, iconFact{Icon: wslFactIconBenefit, Label: "Progress",
			Value: fmt.Sprintf("%d of %d entries placed", state.Progress.Done, state.Progress.Total)})
	}
	rows = append(rows, iconFact{Icon: wslFactIconLimit, Label: "Stopping",
		Value: "leaves what is already copied where it is; a half-copy is evidence"})
	return []factArea{{Name: "Copying", Facts: rows}}
}

// wslMigrationOutcomeParagraphs is the run's own report, as the compact body's paragraphs. It is the
// prose of wslMigrationOutcomeFacts.
func wslMigrationOutcomeParagraphs(state wslMigrationState) []string {
	return factProse(wslMigrationOutcomeFacts(state))
}

// wslMigrationOutcomeFacts is the run's own report, verified or not.
func wslMigrationOutcomeFacts(state wslMigrationState) []iconFact {
	return factAreaFacts(wslMigrationOutcomeAreas(state))
}

// wslMigrationOutcomeAreas is the run's own report as areas: the verdict, the verification's own
// limits, and what the run left for the reader to finish by hand. Every difference and every note is
// one row, so a copy that went wrong names each one instead of folding them into a sentence.
func wslMigrationOutcomeAreas(state wslMigrationState) []factArea {
	report := state.Report

	verdict := make([]iconFact, 0, 4)
	if state.ReportErr != "" {
		verdict = append(verdict, iconFact{Icon: wslFactIconLimit, Label: "Stopped", Value: factClause(state.ReportErr)})
	}
	if report.Cancelled {
		verdict = append(verdict, iconFact{Icon: wslFactIconLimit, Label: "Cancelled",
			Value: "what was copied is still in " + report.Destination + "; the source is as it was"})
	}
	switch {
	case report.Verified:
		verdict = append(verdict, iconFact{Icon: wslFactIconEvidence, Label: "Verification",
			Value: "every copied entry matches the source, " + humanBytes(report.VerifiedBytes) + " compared"})
	case len(report.Failures) > 0:
		verdict = append(verdict, iconFact{Icon: wslFactIconLimit, Label: "Verification",
			Value: "differences against the source, named below"})
	case state.ReportErr == "" && !report.Cancelled:
		verdict = append(verdict, iconFact{Icon: wslFactIconLimit, Label: "Verification",
			Value: "not verified: this screen does not claim the copy matches"})
	}
	verdict = append(verdict, iconFact{Icon: wslFactIconFolder, Label: "Placed",
		Value: factClause(report.Copied.Summary() + " in " + report.Destination)})

	areas := []factArea{{Name: "The copy", Facts: verdict}}

	var limits []iconFact
	if report.SizeOnlyFiles > 0 {
		limits = append(limits, iconFact{Icon: wslFactIconLimit, Label: "Limit",
			Value: fmt.Sprintf("%d files were covered by size and mode alone", report.SizeOnlyFiles)})
	}
	for _, note := range report.VerificationNotes {
		limits = append(limits, iconFact{Icon: wslFactIconEvidence, Label: "Note", Value: factClause(note)})
	}
	for _, failure := range report.Failures {
		limits = append(limits, iconFact{Icon: wslFactIconLimit, Label: "Difference", Value: factClause(failure)})
	}
	if len(limits) > 0 {
		areas = append(areas, factArea{Name: "The verification's limits", Facts: limits})
	}

	toDo := []iconFact{{Icon: wslFactIconReadOnly, Label: "Source",
		Value: "never written to: nothing deleted, moved or renamed"}}
	for _, instruction := range report.Instructions {
		toDo = append(toDo, iconFact{Icon: wslFactIconAction, Label: "By hand", Value: instruction})
	}
	if len(report.Instructions) > 0 {
		toDo = append(toDo, iconFact{Icon: wslFactIconWhy, Label: "When", Value: "once you have checked the copy yourself"})
	}
	return append(areas, factArea{Name: "Left for you", Facts: toDo})
}

// handleWSLMigrationKeys drives the copy's phases. Each row that acts where it stands runs on the
// letter it carries, and Enter is left to the two things it may do on this screen: choosing a source
// or a destination, and opening the way back. Escape is the copy's own key while a copy runs -- it
// stops the copy rather than leaving the question -- and wslMigrationEscape is where that is decided.
func (m Model) handleWSLMigrationKeys(key string) (tea.Model, tea.Cmd) {
	migration := &m.WSLAdvisor.Migration
	rows := m.wslMigrationRows()
	row, onRow := wslActionRowAt(rows, m.Cursor)

	switch key {
	case "up", "k":
		m.Cursor = wslMoveCursor(rows, m.Cursor, -1)
		return m, nil
	case "down", "j":
		m.Cursor = wslMoveCursor(rows, m.Cursor, 1)
		return m, nil
	case wslCopyKey:
		// The confirmation is the one press here that writes, so it is the one letter behind it.
		if onRow && row.Kind == wslActionConfirm {
			return m.startWSLMigrationCopy()
		}
		return m, nil
	case wslCancelKey:
		// The preview's cancel is the safe default: it puts the choice of destination back.
		if !onRow || row.Kind != wslActionCancel || migration.Running {
			return m, nil
		}
		migration.Phase = wslMigratePickDestination
		m.Cursor = migration.Destination
		return m, nil
	case wslStopKey:
		// The copy's stop is its own letter rather than the preview's cancel, so one letter never
		// carries two commands on one screen.
		if !onRow || row.Kind != wslActionCancel || !migration.Running {
			return m, nil
		}
		return m.cancelWSLMigration(), nil
	case wslAnotherKey:
		if !onRow || row.Kind != wslActionRestart {
			return m, nil
		}
		migration.Phase = wslMigratePickSource
		migration.Report = system.MigrationReport{}
		migration.ReportErr = ""
		migration.ProgressSeen = false
		migration.Notice = ""
		m.Cursor = 0
		return m, nil
	case "enter":
		// Enter chooses a row of either list and opens the way back, and it never confirms, cancels,
		// stops or restarts: those are the letters above.
		if !onRow {
			return m, nil
		}
		switch row.Kind {
		case wslActionPick:
			return m.wslMigrationPick(row.Index)
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
				// Leaving a running copy is not offered as "back": the row says "back to the
				// report", and the copy keeps running with its answer landing on the model, exactly
				// as a measurement does. Stopping it is its own row and its own letter.
				m.Screen = ScreenWSLAdvisor
				m.Cursor = 0
			}
			return m, nil
		}
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
