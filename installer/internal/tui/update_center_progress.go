package tui

import (
	"fmt"
	"strings"

	"github.com/albersg/dotfiles/installer/internal/updates"
	"github.com/charmbracelet/lipgloss"
)

// ============================================================================
// THE UPDATE CENTER'S LIVE PROGRESS
// ============================================================================
//
// A run is a sequence of commands -- one per manager, scope and kind, because two
// managers never run together -- and while it runs the screen has to say three
// things: which command is in flight and what it is moving, how much of the
// sequence is settled, and what refused. This file is that surface, and it is
// drawn twice from ONE set of facts: as the wide frame's card
// (updateCenterRunCard) and as the 80x24 floor's prose
// (updateCenterRunParagraphs). Both count the same entries in the same order, so
// the two shapes cannot report different numbers.
//
// Every number here is read from the run's own state: the plans the confirmation
// built, the results the engine returned for the commands that finished, and the
// tail of the output the manager printed. Nothing is timed, interpolated or
// estimated. A command that has not answered contributes nothing to the bar, so
// the bar never claims progress no manager has reported -- which is why the bar
// moves in whole entries and not in a smooth animation.
//
// The heights are fixed by the sequence and the reservations, never by what has
// arrived: a result landing or a log line appearing changes the words on a row
// and never the row's existence, so the composition cannot move under the reader
// as the asynchronous events come in.

const (
	// updateRunStepRows is the tallest checklist the run's card draws. A sequence
	// longer than this scrolls inside the card around the command in flight, and
	// a windowed list spends a row at each end naming what it hid rather than
	// growing the card.
	updateRunStepRows = 6

	// updateRunStepNoteRows is the rows a windowed checklist gives to the two
	// notes that say how many commands it could not show.
	updateRunStepNoteRows = 2

	// updateRunFailureRows is the rows the card reserves for the entries the run
	// did not move. It is reserved whether or not there is a failure yet, so the
	// row a failure lands on already exists and the card keeps its height.
	updateRunFailureRows = 2

	// updateRunLogRows is the rows the card reserves for the end of what the
	// manager printed: the freshest lines, where a package manager says what it
	// is doing right now.
	updateRunLogRows = 3

	// updateRunBarCells is the bar's width in the wide card and
	// updateRunFloorBarCells its width on the 80x24 floor, where the bar shares
	// its row with the prose width and has to stay short enough to read there.
	updateRunBarCells      = 20
	updateRunFloorBarCells = 10
)

// updateCenterRunStep is one command of the run as the progress surface draws it:
// the plan the confirmation built, the result the engine returned once it
// finished, and whether it is the command in flight.
type updateCenterRunStep struct {
	Plan   updates.Plan
	Result *updates.ApplyResult
	// Running reports the command the engine is on now. It is false for every
	// step of a run that has ended.
	Running bool
}

// updateCenterRunSteps is the sequence as the surface draws it: every command
// that finished with its result, then -- while the run is in flight -- the
// command it is on and the ones behind it. The order is the plan's own, so the
// rows and the sequence cannot disagree about what runs first.
//
// The finished half is read from the results rather than from Plans, because a
// run's report outlives the plan list: the re-read that follows a run replaces
// the plans with the managers' fresh answers, and the report of what the run did
// is the results.
func (m Model) updateCenterRunSteps() []updateCenterRunStep {
	state := m.UpdateCenter
	steps := make([]updateCenterRunStep, 0, len(state.Plans))
	for i := range state.Applied {
		steps = append(steps, updateCenterRunStep{Plan: state.Applied[i].Plan, Result: &state.Applied[i]})
	}
	if !state.Applying {
		return steps
	}
	for i := len(steps); i < len(state.Plans); i++ {
		steps = append(steps, updateCenterRunStep{Plan: state.Plans[i], Running: i == state.PlanIndex})
	}
	return steps
}

// updateCenterRunLive reports whether the screen has a run to describe: one in
// flight, or the report of the last one. The report is drawn until the next run
// starts, which is what keeps a failure readable after the sequence ends. The
// confirmation is the one state that hides it: the plan being asked about is the
// subject there, and a previous run's rows beside it would answer a question
// nobody asked.
func (m Model) updateCenterRunLive() bool {
	state := m.UpdateCenter
	return state.Applying || (!state.Confirming && len(state.Applied) > 0)
}

// updateCenterRunTally is the run's own arithmetic: how many entries the finished
// commands settled, how many the sequence carries in all, and how many of the
// settled ones the managers did not move. An entry is settled when its command
// answered -- confirmed, executed, refused or unknown -- and unsettled while its
// command is in flight or behind it.
func updateCenterRunTally(steps []updateCenterRunStep) (done, total, failed int) {
	for _, step := range steps {
		total += len(step.Plan.Items)
		if step.Result == nil {
			continue
		}
		done += len(step.Result.Outcomes)
		failed += step.Result.Failed()
	}
	return done, total, failed
}

// updateCenterRunOutcomeCounts counts one result's entries by what the manager
// said about them. It is the same partition the engine's own Confirmed, Moved and
// Failed report, split so a row can name executed and unknown apart from refused:
// "not moved" and "we cannot say" are different facts and a row that folded them
// would be claiming to know which happened.
func updateCenterRunOutcomeCounts(result updates.ApplyResult) (confirmed, executed, failed, unknown int) {
	for _, outcome := range result.Outcomes {
		switch outcome.Step {
		case updates.StepConfirmed:
			confirmed++
		case updates.StepExecuted:
			executed++
		case updates.StepFailed:
			failed++
		case updates.StepUnknown:
			unknown++
		}
	}
	return confirmed, executed, failed, unknown
}

// updateCenterRunMoving names the entries the command in flight is moving, by the
// names the owning manager printed.
func updateCenterRunMoving(plan updates.Plan) string {
	return updateCenterItemNames(plan.Items)
}

// updateCenterRunResultWords is what one finished command did, in the screen's
// own words. Only the counts that are not zero are said, so a row never reads
// "0 confirmed, 0 executed, 3 not moved" at a reader who only needs the last one.
func updateCenterRunResultWords(result updates.ApplyResult) string {
	confirmed, executed, failed, unknown := updateCenterRunOutcomeCounts(result)
	var parts []string
	if confirmed > 0 {
		parts = append(parts, fmt.Sprintf("%d confirmed", confirmed))
	}
	if executed > 0 {
		parts = append(parts, fmt.Sprintf("%d executed", executed))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d not moved", failed))
	}
	if unknown > 0 {
		parts = append(parts, fmt.Sprintf("%d unknown", unknown))
	}
	if len(parts) == 0 {
		// A command that carried no entry still ran: the status the manager's own
		// output earned is the fact, and no count is invented for it.
		return "the command finished"
	}
	return strings.Join(parts, ", ")
}

// updateCenterRunStepText is one command's row without its glyph: which manager
// owns it, where it installs, and what it is doing or did. The running row names
// the entries it is moving, which is the screen's answer to "what is being
// installed right now".
func (m Model) updateCenterRunStepText(step updateCenterRunStep, width int) string {
	owner := updateCenterManagerLabel(m, step.Plan.Manager) + " (" + string(step.Plan.Scope) + ")"
	switch {
	case step.Running:
		return truncate(owner+" — moving "+updateCenterRunMoving(step.Plan), width)
	case step.Result != nil:
		return truncate(owner+" — "+updateCenterRunResultWords(*step.Result), width)
	default:
		return truncate(owner+" — "+countedNoun(len(step.Plan.Items), "entry", "entries")+" not started", width)
	}
}

// updateCenterRunStepGlyph is a step's state as a glyph and the tone it is drawn
// in, so a terminal with no colour still reads which command is done, in flight,
// refused or waiting -- the glyph carries the state and the tone repeats it.
func updateCenterRunStepGlyph(step updateCenterRunStep) (string, lipgloss.Style) {
	switch {
	case step.Running:
		return "▸", AccentKeyStyle
	case step.Result == nil:
		return "○", MutedStyle
	case step.Result.Failed() > 0:
		return "✗", ErrorStyle
	default:
		return "✓", SuccessStyle
	}
}

// updateRunStepWindow windows the checklist around the command in flight and
// spends a row at each end naming what it hid. The command in progress always
// stays: the reader is not moving anything here, so the window follows the run.
//
// The rows it returns are always exactly the rows it was given: a window that is
// at the top of the list has one note instead of two, and the block pads the
// difference, because a block that changed height as the run walked the list
// would move the card around it.
func updateRunStepWindow(lines []string, at, rows int) []string {
	visible := max(1, rows)
	if len(lines) <= visible {
		return lines
	}
	span := max(1, visible-updateRunStepNoteRows)
	start, end := listWindow(at, span, len(lines))
	out := make([]string, 0, visible)
	if start > 0 {
		out = append(out, MutedStyle.Render(fmt.Sprintf("↑ %d more above", start)))
	}
	out = append(out, lines[start:end]...)
	if end < len(lines) {
		out = append(out, MutedStyle.Render(fmt.Sprintf("↓ %d more below", len(lines)-end)))
	}
	return padRows(out, visible)
}

// updateCenterRunBarRow is the wide card's first row: the bar the run's settled
// entries fill, and the count they fill it to. The count is done/total entries
// and nothing else, so the number, the bar and the rows under them are one fact.
func (m Model) updateCenterRunBarRow(content int) string {
	done, total, _ := updateCenterRunTally(m.updateCenterRunSteps())
	if total == 0 {
		return MutedStyle.Render(truncate("No entry is in this run.", content))
	}
	cells := min(updateRunBarCells, max(4, content-12))
	return meterRendered(done, total, cells) + " " + MutedStyle.Render("entries")
}

// updateCenterRunBarLine is the same fact on the floor, where the prose is drawn
// in one ink and the bar has to carry its own fill: the glyphs say how far the
// run has got and the count repeats it in digits.
func (m Model) updateCenterRunBarLine() string {
	done, total, _ := updateCenterRunTally(m.updateCenterRunSteps())
	if total == 0 {
		return "No entry is in this run."
	}
	fraction := float64(done) / float64(total)
	return meterCellsPlain(fraction, updateRunFloorBarCells) + fmt.Sprintf(" %d/%d entries", done, total)
}

// updateCenterRunStepRows is the wide card's checklist: one row per command of
// the run, windowed around the one in flight.
func (m Model) updateCenterRunStepRows(content int) []string {
	steps := m.updateCenterRunSteps()
	if len(steps) == 0 {
		return []string{MutedStyle.Render(truncate("No command is in this run.", content))}
	}
	at := 0
	lines := make([]string, len(steps))
	for i, step := range steps {
		if step.Running {
			at = i
		}
		glyph, style := updateCenterRunStepGlyph(step)
		lines[i] = style.Render(glyph) + " " +
			InkStyle.Render(m.updateCenterRunStepText(step, max(1, content-2)))
	}
	return updateRunStepWindow(lines, at, min(len(lines), updateRunStepRows))
}

// updateCenterRunFailure is one entry the run did not move, with what the engine
// read about it: the entry, the outcome and the reason the manager gave.
type updateCenterRunFailure struct {
	Item   updates.Item
	Step   updates.OutcomeStep
	Detail string
}

// updateCenterRunFailures is every entry the finished commands did not move, in
// the order the sequence ran them. A refused entry and one the run was stopped
// before the manager could answer for are both here and stay apart: their glyph
// and their detail say which of the two happened.
func (m Model) updateCenterRunFailures() []updateCenterRunFailure {
	var failures []updateCenterRunFailure
	for _, result := range m.UpdateCenter.Applied {
		for _, outcome := range result.Outcomes {
			if outcome.Step != updates.StepFailed && outcome.Step != updates.StepUnknown {
				continue
			}
			failures = append(failures, updateCenterRunFailure{
				Item:   outcome.Item,
				Step:   outcome.Step,
				Detail: outcome.Detail,
			})
		}
	}
	return failures
}

// glyph is the mark the row leads with: the refusal it is, or the question the
// stopped run left.
func (f updateCenterRunFailure) glyph() (string, lipgloss.Style) {
	if f.Step == updates.StepUnknown {
		return "?", WarningStyle
	}
	return "✗", ErrorStyle
}

// row is one failure in the wide card: the entry the manager named, then its own
// reason for it.
func (f updateCenterRunFailure) row(content int) string {
	glyph, style := f.glyph()
	room := max(1, content-2)
	name := truncate(f.Item.Name, max(4, room/3))
	detail := truncate(f.Detail, max(1, room-lipgloss.Width(name)-3))
	return style.Render(glyph) + " " + InkStyle.Render(name) + MutedStyle.Render(" — "+detail)
}

// line is the same failure on the floor.
func (f updateCenterRunFailure) line(width int) string {
	glyph, _ := f.glyph()
	return truncate(glyph+" "+f.Item.Name+" — "+f.Detail, width)
}

// updateCenterRunFailureRows is the wide card's failure block: the entries the
// run did not move, cut with the frame's own marker when there are more of them
// than the block holds, and padded to the rows it reserved. A run with nothing
// refused says so instead of drawing an empty block, so a card that reserved the
// rows is never mistaken for a card that failed to draw.
func (m Model) updateCenterRunFailureRows(content int) []string {
	failures := m.updateCenterRunFailures()
	if len(failures) == 0 {
		return padRows([]string{MutedStyle.Render(truncate("No command has refused.", content))}, updateRunFailureRows)
	}
	lines := make([]string, 0, len(failures))
	for _, failure := range failures {
		lines = append(lines, failure.row(content))
	}
	return padRows(clipLines(lines, updateRunFailureRows, content), updateRunFailureRows)
}

// updateCenterRunLogRows is the wide card's output block: the end of what the
// manager printed, each line behind the same prompt the installing screen's log
// card draws, padded to the rows it reserved.
func (m Model) updateCenterRunLogRows(content int) []string {
	tail := updates.Tail(strings.Join(m.UpdateCenter.Log, "\n"), updateRunLogRows)
	lines := make([]string, 0, updateRunLogRows)
	room := max(1, content-lipgloss.Width(installingLogPrompt))
	for _, line := range tail {
		lines = append(lines, MutedStyle.Render(installingLogPrompt)+InfoStyle.Render(truncate(line, room)))
	}
	if len(lines) == 0 {
		lines = append(lines, MutedStyle.Render(truncate("Waiting for the manager's first line…", content)))
	}
	return padRows(lines, updateRunLogRows)
}

// updateCenterRunCaption is the card's caption: which command of how many the
// engine is on, or that the sequence it is reporting has ended.
func (m Model) updateCenterRunCaption() string {
	state := m.UpdateCenter
	if !state.Applying {
		return "finished"
	}
	if state.PlanIndex >= 0 && state.PlanIndex < len(state.Plans) {
		return fmt.Sprintf("step %d of %d", state.PlanIndex+1, len(state.Plans))
	}
	return "running"
}

// updateCenterRunCard is the live progress surface: the bar and its count, the
// sequence with the command in flight naming what it moves, the entries that did
// not move, and the end of the manager's output. It is one card in the reference
// grammar, with a titled divider per block, so every block and every row is the
// same object the rest of the installer draws.
//
// Its height is a function of the sequence and the reservations alone -- the
// number of commands, the capped checklist, the failure rows and the log rows --
// so a result landing or a line arriving rewrites rows that already exist.
func (m Model) updateCenterRunCard(width int) []string {
	content := cardContentWidth(width)
	return cardGroups("Progress", m.updateCenterRunCaption(), width, []cardGroup{
		{Lines: []string{m.updateCenterRunBarRow(content)}},
		{Title: "Commands", Lines: m.updateCenterRunStepRows(content)},
		{Title: "Not moved", Lines: m.updateCenterRunFailureRows(content)},
		{Title: "Output", Lines: m.updateCenterRunLogRows(content)},
	})
}

// updateCenterRunParagraphs is the running state at the 80x24 floor, in exactly
// the four rows the section's detail band holds there: the bar and its count, the
// step the engine is on, the entries it is moving, and the newest thing that has
// happened -- a failure first, then a finished command's score, then the last line
// the manager printed.
//
// It is four lines and never more because the band is four rows on every frame
// and the fourth row of a longer list is the frame's cut marker: a fifth fact
// would cost the failure its row. The wide frame's card carries the whole
// sequence, the whole failure list and the whole output block; the floor carries
// the four facts a run in flight is made of.
//
// The old running prose is gone with it: the sentence that said an update was
// running, and that Escape does not leave the screen, spent the band's whole width
// on a fact the frame's status bar (`Installing…`), the stop row and the footer
// already state -- and it pushed the progress off the screen.
func (m Model) updateCenterRunParagraphs() []string {
	width := layoutFor(m).Composition
	lines := []string{
		m.updateCenterRunBarLine(),
		m.updateCenterRunStepHeading(),
	}

	steps := m.updateCenterRunSteps()
	moving := ""
	for _, step := range steps {
		if step.Running {
			moving = truncate("Moving "+updateCenterRunMoving(step.Plan), width)
			break
		}
	}
	if moving != "" {
		lines = append(lines, moving)
	}
	return append(lines, m.updateCenterRunLatestEvent(width))
}

// updateCenterRunLatestEvent is the floor's fourth row: the newest fact the run
// has produced. A failure is the fact a reader must not miss, so it comes first;
// then the last finished command's score; then the end of the manager's output.
func (m Model) updateCenterRunLatestEvent(width int) string {
	if failures := m.updateCenterRunFailures(); len(failures) > 0 {
		return failures[len(failures)-1].line(width)
	}
	if result, ok := m.updateCenterRunLastResult(); ok {
		glyph, _ := updateCenterRunStepGlyph(updateCenterRunStep{Result: &result})
		return truncate(glyph+" "+updateCenterRunResultWords(result), width)
	}
	if lines := m.updateCenterRunLogParagraphs(width); len(lines) > 0 {
		return lines[len(lines)-1]
	}
	return "Waiting for the manager's first line…"
}

// updateCenterRunStepHeading is the floor's step line: which command of how many
// the engine is on and who owns it.
func (m Model) updateCenterRunStepHeading() string {
	state := m.UpdateCenter
	if len(state.Plans) == 0 {
		return ""
	}
	at := state.PlanIndex
	if at < 0 || at >= len(state.Plans) {
		return ""
	}
	return fmt.Sprintf("Step %d of %d · %s (%s)", at+1, len(state.Plans),
		updateCenterManagerLabel(m, state.Plans[at].Manager), state.Plans[at].Scope)
}

// updateCenterRunLogParagraphs is the end of what the manager printed, on the
// floor: the freshest lines, behind the prompt the log card uses.
func (m Model) updateCenterRunLogParagraphs(width int) []string {
	tail := updates.Tail(strings.Join(m.UpdateCenter.Log, "\n"), updateRunLogRows)
	lines := make([]string, 0, len(tail))
	for _, line := range tail {
		lines = append(lines, truncate(installingLogPrompt+line, width))
	}
	if len(lines) == 0 {
		lines = append(lines, "Waiting for the manager's first line…")
	}
	return lines
}

// updateCenterRunLastResult is the most recent command that finished, so the
// floor can say what it did before the next one ends.
func (m Model) updateCenterRunLastResult() (updates.ApplyResult, bool) {
	applied := m.UpdateCenter.Applied
	if len(applied) == 0 {
		return updates.ApplyResult{}, false
	}
	return applied[len(applied)-1], true
}

// updateCenterRunReportParagraphs is the floor's report once the sequence has
// ended: the entries the run did not move, with the managers' own reasons, so a
// failure stays readable after the run rather than only while it happened. The
// rows carry the whole report -- an entry with its reason -- and the notice band
// under them carries the score, so there is no headline claiming anything about
// an entry whose outcome the manager could not settle. A run that moved every
// entry it carried says nothing here: the notice already reports it, and a second
// copy would be a row a fact could have used.
func (m Model) updateCenterRunReportParagraphs() []string {
	failures := m.updateCenterRunFailures()
	stopped := m.UpdateCenter.NotReached
	if len(failures) == 0 && len(stopped) == 0 {
		return nil
	}

	width := layoutFor(m).Composition
	lines := make([]string, 0, len(failures)+len(stopped))
	for _, failure := range failures {
		lines = append(lines, failure.line(width))
	}
	for _, item := range stopped {
		lines = append(lines, truncate("- "+item.Name+" — the run was stopped before its command ran", width))
	}
	return lines
}

// updateCenterRunFooterLine is the footer's middle while the sequence runs: the
// command in flight and the one it is on. It is the same fact the card's caption
// carries, in the row the frame has for what the screen is doing.
func (m Model) updateCenterRunFooterLine() string {
	state := m.UpdateCenter
	if len(state.Plans) == 0 {
		return "Updating…"
	}
	at := state.PlanIndex
	if at < 0 || at >= len(state.Plans) {
		return "Updating…"
	}
	return fmt.Sprintf("Running %d of %d · %s (%s)", at+1, len(state.Plans),
		updateCenterManagerLabel(m, state.Plans[at].Manager), state.Plans[at].Scope)
}

// updateCenterRunResultLine is the footer's middle once the sequence has ended:
// what the finished commands did, counted from the results themselves, so the
// wide frame -- which draws no notice band -- still reports the run.
func (m Model) updateCenterRunResultLine() string {
	confirmed, executed, failed := 0, 0, 0
	for _, result := range m.UpdateCenter.Applied {
		confirmed += result.Confirmed()
		executed += result.Moved() - result.Confirmed()
		failed += result.Failed()
	}
	parts := []string{fmt.Sprintf("%d confirmed", confirmed)}
	if executed > 0 {
		parts = append(parts, fmt.Sprintf("%d executed", executed))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d not moved", failed))
	}
	if stopped := len(m.UpdateCenter.NotReached); stopped > 0 {
		parts = append(parts, countedNoun(stopped, "never run", "never run"))
	}
	return "Finished: " + strings.Join(parts, ", ")
}
