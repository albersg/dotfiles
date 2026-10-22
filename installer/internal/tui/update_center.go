package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/albersg/dotfiles/installer/internal/updates"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ============================================================================
// THE UTILITIES SECTION: WHAT THE MACHINE'S OWN MANAGERS HAVE TO UPDATE
// ============================================================================
//
// This is the screen the update center lives on. It reads what every package
// manager and toolchain manager this machine really has says is out of date,
// shows every entry with the manager and the scope it belongs to, and moves only
// what the user selects -- through the manager that owns it.
//
// Four rules shape it, and each one is a defect this file exists to prevent:
//
//   - Nothing runs on its own. Opening the screen reads; it never applies, and
//     there is no "update everything" row anywhere. A selection is the only input
//     a command is built from.
//   - The plan is shown before anything happens. Confirming a selection opens a
//     state that names every command exactly, who it needs to be, what stays
//     pending afterwards and what going back would cost.
//   - What happened is what the manager says happened. The engine re-reads each
//     manager after its command, and the results distinguish requested,
//     executed and confirmed.
//   - Nothing is inferred. A channel that could not be asked says why; a manager
//     that is not installed is not an error; a version the manager did not print
//     is left empty rather than invented.

const (
	// utilitiesUpdatesRow is the section's row for this screen: the mockup's own
	// name for the installed tools the screen reads and updates, with the action as
	// the row's subtitle ("Check latest versions"), in the same voice as the
	// section's other subtitles.
	utilitiesUpdatesRow = "Installed Tools"

	// The rows of the screen itself, by prefix where the label carries a count or
	// a value: a row's label changes when the state changes, and a whole-string
	// match could not survive that.
	//
	// The three markers in front of an entry are rendered but never matched as
	// prefixes by the handler: the handler matches the whole row against the rows
	// the state would draw, so an entry whose name begins with a bracket cannot be
	// mistaken for a marker.
	updateCenterRowUnselected = "[ ] "
	updateCenterRowSelected   = "[x] "
	updateCenterRowHeld       = "[held] "

	// updateCenterCheckRow asks the managers again. It is always offered when
	// nothing is running: an answer that is stale, wrong or missing is exactly
	// what a second look is for.
	updateCenterCheckRow = "[r] Check for updates again"
	// updateCenterClearRow drops the whole selection. It is offered only when
	// there is one, so the row count never changes for a press that would do
	// nothing.
	updateCenterClearRow = "[c] Clear the selection"
	// updateCenterApplyRowPrefix leads the row that confirms a selection. Its
	// count is the selection's own count, never a number kept beside it.
	updateCenterApplyRowPrefix = "[a] Apply the selected updates ("
	// updateCenterSelectVisibleRowPrefix and updateCenterDropVisibleRowPrefix lead
	// the row that takes the category on screen in one press. Its count is how
	// many entries the filter is showing that can be selected -- a pinned entry is
	// shown and never taken -- so the row cannot claim a number the list does not
	// have. The second prefix is the same row once the whole category is already
	// selected: a row that says "select" and drops is a row that lies about itself.
	updateCenterSelectVisibleRowPrefix = "[s] Select the "
	updateCenterDropVisibleRowPrefix   = "[s] Drop the "
	// updateCenterFilterRowPrefix leads the one filter row, whose value cycles
	// through the managers that have entries. One row rather than one row per
	// manager: the filter is a view of the same list, not a second list.
	updateCenterFilterRowPrefix = "[f] Filter: "
	// updateCenterFilterEvery is the filter's value when every manager is shown.
	updateCenterFilterEvery = "every manager"
	// updateCenterConfirmRow and updateCenterCancelConfirmRow are the
	// confirmation state's two rows. Confirming is a state and not a keystroke:
	// the command the run would execute is on the screen while the question is
	// asked.
	updateCenterConfirmRow       = "[y] Yes, apply the plan"
	updateCenterCancelConfirmRow = "[n] No, go back to the list"
	// updateCenterStopRow is the running state's only row. It asks the manager to
	// stop the way an interrupt does -- finish what you are doing and stop -- and
	// the result of that request is what the screen reports. Its letter is the
	// interrupt it is, and the stop screen of the port manager's own `x` is a
	// different screen's command.
	updateCenterStopRow = "[i] Stop the update that is running"

	// updateCenterLogRows is how many lines of a running command's output the
	// screen shows. The end of the log is where the progress is, and the whole of
	// it is kept in the model for the result.
	updateCenterLogRows = 6

	// updateCenterLogKept is how many lines of output the model keeps. A package
	// manager can print megabytes; what a reader needs is the end of it, and the
	// engine's own result keeps its bounded copy beside this one.
	updateCenterLogKept = 400

	// updateCenterLineBuffer is how many output lines a running command may have
	// in flight before the reader falls behind. A full buffer drops lines from
	// the screen rather than blocking the manager: a package manager waiting on a
	// full channel would be a screen that hangs the machine's own update.
	updateCenterLineBuffer = 128
)

// updateCenterState is everything this screen draws and does.
//
// The reading half (Channels, Results, Order, Pending, InFlight, CheckedAt) is
// the machine's answer, replaced as each manager answers. The deciding half
// (Selected, Filter, Plans, LeftOut, Confirming) is the user's. The running half
// (Applying, Applied, Log, Lines, ctx, cancel) is one update at a time.
type updateCenterState struct {
	// Engine is the package that asks the managers. It is built when the screen is
	// first opened, which is what keeps the machine's package managers off the
	// startup path: a run that never opens this screen never asks one of them
	// anything.
	Engine *updates.Center
	// Channels is the inventory: every manager this build knows, with whether it
	// can be asked on this host and why not when it cannot. It is resolved once,
	// when the screen is first opened.
	Channels []updates.Channel
	// Resolved reports that the inventory has been asked for.
	Resolved bool

	// Results are the managers' answers, in the order they were asked for. A
	// manager that has not answered yet has no entry, which is the difference
	// between "not asked" and "asked and empty".
	Results []updates.Result
	// Order is the managers this pass asks, in the inventory's order. It holds
	// only the channels that can be asked here.
	Order []updates.ManagerID
	// Pending is the manager being read right now, "" when nothing is running.
	Pending updates.ManagerID
	// InFlight reports that a reading pass is running. One pass at a time, so the
	// answer that lands last is the answer to the question that was asked.
	InFlight bool
	// CheckedAt is when the last finished pass started, zero when none has.
	CheckedAt time.Time

	// Selected is the set of entries the user has chosen, by identity.
	Selected updates.Selection
	// Filter narrows the list to one manager, "" for every manager.
	Filter updates.ManagerID

	// Confirming reports that the plan is on the screen and the question is being
	// asked. Plans and LeftOut are what that state shows.
	Confirming bool
	Plans      []updates.Plan
	LeftOut    []updates.LeftOut
	// NotReached is what the run was asked to move and never got to: the entries
	// of the plans a stopped sequence did not start. They are not left out -- an
	// entry left out is one no command could carry -- and keeping the two apart
	// is what lets the report say which of the two actually happened.
	NotReached []updates.Item

	// Applying reports that the confirmed plans are running, PlanIndex is the one
	// running now, and Applied is what the finished ones did.
	Applying  bool
	PlanIndex int
	Applied   []updates.ApplyResult

	// Log is the tail of the running command's output, kept bounded.
	Log []string
	// Lines is the channel the running command writes its output to, and the one
	// the model reads a line at a time. It is nil when nothing is running.
	Lines chan string
	// ctx and cancel belong to the running pass. cancel is what the stop row and
	// the `c` key call: the engine sends the manager an interrupt, and the
	// command's own bound turns a manager that ignores it into a bounded stop.
	ctx    context.Context
	cancel context.CancelFunc

	// Notice is the one line of result the screen shows under the list. It is
	// cleared when the screen is left, exactly as the other screens' notices are.
	Notice string
}

// The messages this screen's commands send back. Each one is a fact rather than
// an instruction: a manager's answer, one plan's result, one line of output, or
// the end of that output.
type updateCenterReadMsg struct {
	manager updates.ManagerID
	result  updates.Result
}

type updateCenterApplyDoneMsg struct {
	index  int
	result updates.ApplyResult
}

type updateCenterLineMsg string

type updateCenterLinesClosedMsg struct{}

// ---------------------------------------------------------------------------
// The commands
// ---------------------------------------------------------------------------

// startUpdateCenterScan begins one reading pass: it resolves the inventory the
// first time it runs, then asks every channel that can be asked, one at a time.
//
// One at a time is not a preference. apt, pacman, dnf and pkg keep locks over the
// same package state, and this program does not start two managers together on
// either path -- checking or applying.
func (m *Model) startUpdateCenterScan() tea.Cmd {
	state := &m.UpdateCenter

	// A pass that is still out is the whole answer: starting a second one beside
	// it would put two managers on the machine at once, which is the lock this
	// screen exists never to take. Asking again means waiting for this one -- on
	// the re-check key, on the section's row, and on the model that re-enters the
	// screen while a read is still outstanding.
	if state.InFlight {
		if state.Pending != "" {
			state.Notice = "Already reading " + string(state.Pending) + ": one manager at a time."
		} else {
			state.Notice = "A reading pass is already running: one manager at a time."
		}
		return nil
	}

	// A pass replaces the answers the plan was built from, so the confirmation is
	// dismissed: a plan about a list that is being read again must not stay on
	// screen, confirmable against answers that no longer exist.
	state.Confirming = false
	state.Plans = nil
	state.LeftOut = nil

	if state.Engine == nil {
		state.Engine = updates.Default(m.SystemInfo != nil && m.SystemInfo.IsTermux)
	}
	if !state.Resolved {
		state.Channels = state.Engine.Channels()
		state.Resolved = true
	}

	state.Results = nil
	state.Order = state.Order[:0]
	for _, channel := range state.Channels {
		if channel.Available {
			state.Order = append(state.Order, channel.Manager)
		}
	}
	state.CheckedAt = updateCenterClock()
	state.InFlight = true

	if len(state.Order) == 0 {
		state.InFlight = false
		state.Pending = ""
		state.Notice = "No package manager this screen knows is installed here, so there is nothing to check."
		return nil
	}

	state.Pending = state.Order[0]
	return m.updateCenterReadCmd(state.Pending)
}

// updateCenterReadCmd asks one manager. The read runs off the update loop, so a
// manager waiting on an unreachable mirror cannot hold the frame, and its own
// timeout is the bound.
func (m Model) updateCenterReadCmd(manager updates.ManagerID) tea.Cmd {
	engine := m.UpdateCenter.Engine
	return func() tea.Msg {
		return updateCenterReadMsg{manager: manager, result: engine.Read(context.Background(), manager)}
	}
}

// updateCenterNextManager is the manager after one in this pass, or "" when the
// pass is finished.
func (m *Model) updateCenterNextManager(manager updates.ManagerID) updates.ManagerID {
	for index, candidate := range m.UpdateCenter.Order {
		if candidate == manager && index+1 < len(m.UpdateCenter.Order) {
			return m.UpdateCenter.Order[index+1]
		}
	}
	return ""
}

// startUpdateCenterApply begins the confirmed plan. It opens the output channel
// once for the whole run, so the lines of every command arrive in one stream and
// the screen shows them as they come.
func (m *Model) startUpdateCenterApply() tea.Cmd {
	state := &m.UpdateCenter
	// A pass that is still running is refused here as well as on the row: the plan
	// is built from what the managers answered, and a plan confirmed from half an
	// answer is a command built from half a list.
	if state.Applying || state.InFlight || state.Engine == nil || len(state.Plans) == 0 {
		return nil
	}

	state.Applying = true
	state.Confirming = false
	state.PlanIndex = 0
	state.Applied = nil
	// NotReached belongs to the run that filled it: a run's report is read until the
	// next run starts, and a later run -- whose plan may well have moved the same
	// entry -- must not inherit the previous run's unreached entries.
	state.NotReached = nil
	state.Log = nil
	state.Notice = ""
	state.Lines = make(chan string, updateCenterLineBuffer)
	state.ctx, state.cancel = context.WithCancel(context.Background())

	return tea.Batch(
		m.updateCenterApplyPlanCmd(0),
		updateCenterWaitForLine(state.Lines),
	)
}

// updateCenterApplyPlanCmd runs one plan of the confirmed sequence.
func (m Model) updateCenterApplyPlanCmd(index int) tea.Cmd {
	state := &m.UpdateCenter
	engine := state.Engine
	ctx := state.ctx
	lines := state.Lines
	plan := state.Plans[index]
	return func() tea.Msg {
		result := engine.Apply(ctx, plan, func(line string) {
			// A full buffer drops a line from the screen rather than blocking the
			// manager: the result keeps its own bounded copy, and a package manager
			// waiting on a slow reader is a hang this screen must not cause.
			select {
			case lines <- line:
			default:
			}
		})
		return updateCenterApplyDoneMsg{index: index, result: result}
	}
}

// updateCenterWaitForLine reads one line of a running command's output. It is the
// ordinary Bubbletea shape for streaming: the command closes the channel when it
// is finished, and the closed channel is what ends the reads.
func updateCenterWaitForLine(lines <-chan string) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-lines
		if !ok {
			return updateCenterLinesClosedMsg{}
		}
		return updateCenterLineMsg(line)
	}
}

// stopUpdateCenterRun asks the running command to stop. The engine sends the
// manager an interrupt, which is what a manager traps to finish the transaction
// it is inside; the remaining plans of the sequence are not started, and the
// entries whose command never ran are reported as left out rather than as
// anything else.
func (m Model) stopUpdateCenterRun() (tea.Model, tea.Cmd) {
	if m.UpdateCenter.cancel == nil {
		return m, nil
	}
	m.UpdateCenter.cancel()
	m.UpdateCenter.Notice = "Stopping: the manager is asked to finish what it is doing before it is killed."
	return m, nil
}

// updateCenterClock is the clock the screen's timestamps come from. It is a
// variable so a guard pins a time instead of depending on when it ran.
var updateCenterClock = time.Now

// updateCenterFocusedItem is the entry the cursor is on, and false when it is on
// an action row or a group heading.
func (m Model) updateCenterFocusedItem() (updates.Item, bool) {
	options := m.GetCurrentOptions()
	if m.Cursor < 0 || m.Cursor >= len(options) {
		return updates.Item{}, false
	}
	row := options[m.Cursor]
	if !isUpdateCenterItemRow(row) {
		return updates.Item{}, false
	}
	return m.updateCenterItemForRow(row)
}

// updateCenterDetailItem is the entry the screen describes: the one under the
// cursor, or -- when the cursor is on a row that is not a package -- the nearest
// package above it, and below it when it is on none above. The cursor only moves
// between packages (updateCenterStep), so this is the floor under the states that
// put it elsewhere: a filter that shrinks the list, a selection that adds an
// action row above the way back. The detail speaks for the entry nearest the
// cursor instead of going blank and taking the composition with it.
func (m Model) updateCenterDetailItem() (updates.Item, bool) {
	if item, ok := m.updateCenterFocusedItem(); ok {
		return item, true
	}
	options := m.GetCurrentOptions()
	for i := min(m.Cursor, len(options)) - 1; i >= 0; i-- {
		if item, ok := m.updateCenterItemAt(options, i); ok {
			return item, true
		}
	}
	for i := max(0, m.Cursor); i < len(options); i++ {
		if item, ok := m.updateCenterItemAt(options, i); ok {
			return item, true
		}
	}
	return updates.Item{}, false
}

// updateCenterItemAt resolves one option row to its entry, or reports that the
// row is not one.
func (m Model) updateCenterItemAt(options []string, index int) (updates.Item, bool) {
	if index < 0 || index >= len(options) || !isUpdateCenterItemRow(options[index]) {
		return updates.Item{}, false
	}
	return m.updateCenterItemForRow(options[index])
}

// updateCenterStep is the row the cursor moves to: the next package row in the
// direction pressed, or where it is when the list has none that way. The cursor
// rests only on rows the detail can speak for, so walking the list can never park
// it on a group rule, an action row or the way back -- the actions carry their own
// letters and the way back is Esc, so nothing becomes unreachable.
func (m Model) updateCenterStep(cursor, delta int) int {
	options := m.GetCurrentOptions()
	for i := cursor + delta; i >= 0 && i < len(options); i += delta {
		if isUpdateCenterItemRow(options[i]) {
			return i
		}
	}
	return cursor
}

// updateCenterItemFacts is one entry's complete record, one labelled fact per
// line: what it is, who owns it, both versions in full, why it is held and what
// the manager said about it. The row is a table cell and cuts; this is where the
// values are read.
func (m Model) updateCenterItemFacts(item updates.Item) []string {
	fact := func(label, value string) string {
		if value == "" {
			return ""
		}
		return fmt.Sprintf("%-11s %s", label, value)
	}
	installed := item.Current
	if installed == "" {
		installed = "not reported"
	}
	// The name and the owner are already on the row and on the card's title. The
	// detail states only what the row cannot: the full versions, the hold, the
	// manager's note and the exact command the answer was read with.
	facts := []string{
		fact("Installed", installed),
		fact("Available", item.Available),
	}
	if item.Pinned {
		reason := item.PinReason
		if reason == "" {
			reason = "held by the manager"
		}
		facts = append(facts, fact("Held", reason))
	}
	facts = append(facts, fact("Note", item.Note))
	facts = append(facts, fact("Read with", m.updateCenterCommandFor(item.Manager)))
	return facts
}

// updateCenterCommandFor is the command the owning manager's answer was read
// with, as the manager reported it.
func (m Model) updateCenterCommandFor(manager updates.ManagerID) string {
	for _, result := range m.UpdateCenter.Results {
		if result.Manager == manager {
			return result.Command
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// The rows
// ---------------------------------------------------------------------------

// updateCenterRows builds the screen's rows from its state. The item rows come
// first, grouped by manager with a rule between the groups; the action rows
// follow under one rule; the way back is last. Nothing here is typed twice: the
// rows the screen draws and the rows the handler matches are the same function's
// output.
func (m Model) updateCenterRows() []string {
	items := m.updateCenterItemRows()
	pinned := m.updateCenterPinnedRows()
	if len(items) == 0 {
		return pinned
	}
	rows := append([]string{}, items...)
	rows = append(rows, m.menuSeparator())
	return append(rows, pinned...)
}

// updateCenterItemRows is the part of the list the window scrolls: one row per
// entry, grouped by the manager that owns it.
func (m Model) updateCenterItemRows() []string {
	state := m.UpdateCenter
	switch {
	case state.Applying, state.Confirming:
		// These states replace the list with their own two rows, which are pinned.
		return nil
	}

	var rows []string
	manager := updates.ManagerID("")
	for _, item := range m.updateCenterVisibleItems() {
		if item.Manager != manager {
			if len(rows) > 0 {
				rows = append(rows, m.menuSeparator())
			}
			manager = item.Manager
		}
		rows = append(rows, updateCenterItemRow(item, state.Selected.Has(item), m.updateCenterRowMeasure()))
	}
	return rows
}

// updateCenterPinnedReserve is the tallest the action block gets in the list
// state: the select-the-shown row, apply and clear (which appear once something
// is selected), check, filter, the separator that opens the block, and the way
// back. The list band reserves it in every state so an action that appears with
// a selection spends the slack held above the tail instead of shrinking the
// window the entries scroll in.
const updateCenterPinnedReserve = 7

// updateCenterPinnedRows is the part of the list that never scrolls: what can be
// done with the selection, and the way back. They used to live at the end of the
// same list the entries are in, so the first screenful of a large list pushed the
// apply, clear and filter rows off the bottom while the footer still advertised
// them. The window now covers the entries and these rows are always drawn.
func (m Model) updateCenterPinnedRows() []string {
	state := m.UpdateCenter

	switch {
	case state.Applying:
		return []string{updateCenterStopRow}
	case state.Confirming:
		return []string{updateCenterConfirmRow, updateCenterCancelConfirmRow}
	}

	var rows []string
	// One row takes the category the filter is showing. It is offered before the
	// per-entry actions and only on a settled list: taking a manager's worth of
	// updates one entry at a time is the work a list exists to save, and doing it
	// while the list is still being read would take a number the read has not
	// finished.
	if selectable := m.updateCenterSelectableVisible(); !state.InFlight && len(selectable) > 1 {
		rows = append(rows, m.updateCenterSelectVisibleRow(selectable))
	}
	if state.Selected.Count() > 0 {
		// Confirming is offered only once every manager has answered: the plan is
		// built from the answers, so a list still being read is not a plan.
		if !state.InFlight {
			rows = append(rows, updateCenterApplyRow(state.Selected.Count()))
		}
		rows = append(rows, updateCenterClearRow)
	}
	if !state.InFlight {
		rows = append(rows, updateCenterCheckRow)
	}
	if filter := m.updateCenterFilterRow(); filter != "" {
		rows = append(rows, filter)
	}
	// The action block ends with the way back and no second rule: the separator
	// that opens the block is the only one, so the block is exactly the rows after
	// the last separator in the option list. That is what lets the frame pin it --
	// and pinning it is what keeps an action appearing with a selection from
	// resizing the window the entries scroll in.
	return append(rows, "← Back")
}

// updateCenterRowMeasure is the columns one entry row draws in: the list's row
// measure minus the two the cursor marker spends before the label.
func (m Model) updateCenterRowMeasure() int {
	return max(1, layoutFor(m).RowMeasure-2)
}

// updateCenterItemRow renders one entry as a table row: its state, its name, the
// version it is on and the version available, and where it belongs. The name and
// the versions are columns of their own, cut with the shared marker inside their
// column, so a long Debian version cannot push the owner off the row and the
// eye can compare installed against available down the list. The manager and the
// scope stay in the row rather than only in the group heading, because a row that
// does not say who owns it is a row that cannot be acted on with confidence --
// and because two managers that both carry a "git" are two entries this screen
// must never present as one.
func updateCenterItemRow(item updates.Item, selected bool, measure int) string {
	mark := updateCenterRowUnselected
	switch {
	case item.Pinned:
		mark = updateCenterRowHeld
	case selected:
		mark = updateCenterRowSelected
	}

	versions := item.Current
	if versions == "" {
		versions = "unknown"
	}
	if item.Available != "" {
		versions += " → " + item.Available
	}
	tag := item.Origin()

	// The columns share what the marker and the owner leave. The name gets the
	// smaller share: a version string is the thing being compared, and the name is
	// still readable when it is cut. A frame too narrow for three columns falls
	// back to the two that matter -- the name and the versions.
	const gap = 2
	room := measure - lipgloss.Width(mark) - lipgloss.Width(tag) - gap*2
	row := ""
	if room >= 24 {
		nameWidth := min(max(room*45/100, 12), 32)
		versionWidth := room - nameWidth
		row = mark +
			padRight(truncate(item.Name, nameWidth), nameWidth) + strings.Repeat(" ", gap) +
			padRight(truncate(versions, versionWidth), versionWidth) + strings.Repeat(" ", gap) +
			tag
	} else {
		room = measure - lipgloss.Width(mark) - gap
		nameWidth := min(max(room*40/100, 10), room-8)
		versionWidth := max(8, room-nameWidth)
		row = mark +
			padRight(truncate(item.Name, nameWidth), nameWidth) + strings.Repeat(" ", gap) +
			truncate(versions, versionWidth)
	}

	// What the manager said about this one entry that is neither a version nor a
	// hold -- a version it does not report, a cask the vendor maintains itself --
	// belongs on the row the reader decides about. It follows the table columns
	// and is cut to the list's own measure like every other row, so a narrow
	// terminal cuts the note rather than wrapping the list out of the frame; the
	// entry's detail carries it in full.
	if item.Note != "" {
		row += "  — " + item.Note
	}
	return row
}

// updateCenterSelectableVisible is what the filter is showing that this screen
// would actually move: a pinned entry is on the screen and is never selected,
// because selecting it would mean a row that accepts a press it will refuse.
func (m Model) updateCenterSelectableVisible() []updates.Item {
	var selectable []updates.Item
	for _, item := range m.updateCenterVisibleItems() {
		if !item.Pinned {
			selectable = append(selectable, item)
		}
	}
	return selectable
}

// updateCenterSelectVisibleRow is the category row as the current selection makes
// it read: taking the shown entries, or putting them back once they are all taken.
func (m Model) updateCenterSelectVisibleRow(selectable []updates.Item) string {
	prefix := updateCenterSelectVisibleRowPrefix
	if m.updateCenterAllVisibleSelected(selectable) {
		prefix = updateCenterDropVisibleRowPrefix
	}
	return fmt.Sprintf("%s%d shown", prefix, len(selectable))
}

// updateCenterAllVisibleSelected reports whether every selectable entry on screen
// is already selected. An empty group is not "all selected": there is nothing to
// put back.
func (m Model) updateCenterAllVisibleSelected(selectable []updates.Item) bool {
	if len(selectable) == 0 {
		return false
	}
	for _, item := range selectable {
		if !m.UpdateCenter.Selected.Has(item) {
			return false
		}
	}
	return true
}

// updateCenterToggleVisibleSelection takes the whole category or puts it back. It
// touches only what the filter is showing, so narrowing to one manager and taking
// the category cannot reach another manager's entries -- and it cannot reach a
// pinned one, which is left where the manager holds it.
func (m *Model) updateCenterToggleVisibleSelection() {
	selectable := m.updateCenterSelectableVisible()
	drop := m.updateCenterAllVisibleSelected(selectable)
	for _, item := range selectable {
		switch {
		case drop:
			m.UpdateCenter.Selected.Drop(item)
		case !m.UpdateCenter.Selected.Has(item):
			m.UpdateCenter.Selected.Toggle(item)
		}
	}
}

// updateCenterApplyRow is the row that opens the confirmation. Its count is the
// selection's own, so the row cannot claim a number the selection does not have.
func updateCenterApplyRow(count int) string {
	return fmt.Sprintf("%s%d)", updateCenterApplyRowPrefix, count)
}

// updateCenterFilterRow is the one filter row, or "" when there is nothing to
// filter: a filter over one manager is a row that would do nothing.
func (m Model) updateCenterFilterRow() string {
	if len(m.updateCenterManagersWithItems()) < 2 {
		return ""
	}
	value := updateCenterFilterEvery
	if m.UpdateCenter.Filter != "" {
		value = string(m.UpdateCenter.Filter)
	}
	return updateCenterFilterRowPrefix + value
}

// ---------------------------------------------------------------------------
// The data behind the rows
// ---------------------------------------------------------------------------

// updateCenterItems returns every entry the managers reported, in the inventory's
// order and then each manager's own order. It is the whole answer, filtered by
// nothing.
func (m Model) updateCenterItems() []updates.Item {
	var items []updates.Item
	for _, channel := range m.UpdateCenter.Channels {
		if result, ok := m.updateCenterResult(channel.Manager); ok {
			items = append(items, result.Items...)
		}
	}
	return items
}

// updateCenterVisibleItems is updateCenterItems narrowed by the filter.
func (m Model) updateCenterVisibleItems() []updates.Item {
	filter := m.UpdateCenter.Filter
	if filter == "" {
		return m.updateCenterItems()
	}
	items := m.updateCenterItems()
	visible := make([]updates.Item, 0, len(items))
	for _, item := range items {
		if item.Manager == filter {
			visible = append(visible, item)
		}
	}
	return visible
}

// updateCenterManagersWithItems lists the managers that have entries, in the
// inventory's order. It is what the filter cycles through.
func (m Model) updateCenterManagersWithItems() []updates.ManagerID {
	var managers []updates.ManagerID
	for _, channel := range m.UpdateCenter.Channels {
		if result, ok := m.updateCenterResult(channel.Manager); ok && len(result.Items) > 0 {
			managers = append(managers, channel.Manager)
		}
	}
	return managers
}

// updateCenterResult returns one manager's answer, or false when it has not
// answered.
func (m Model) updateCenterResult(manager updates.ManagerID) (updates.Result, bool) {
	for _, result := range m.UpdateCenter.Results {
		if result.Manager == manager {
			return result, true
		}
	}
	return updates.Result{}, false
}

// storeUpdateCenterResult replaces one manager's answer, keeping the results in
// the inventory's order rather than in the order they happened to arrive.
func (m *Model) storeUpdateCenterResult(result updates.Result) {
	for index, existing := range m.UpdateCenter.Results {
		if existing.Manager == result.Manager {
			m.UpdateCenter.Results[index] = result
			return
		}
	}
	m.UpdateCenter.Results = append(m.UpdateCenter.Results, result)
}

// updateCenterChannel finds one manager's inventory entry.
func (m Model) updateCenterChannel(manager updates.ManagerID) (updates.Channel, bool) {
	for _, channel := range m.UpdateCenter.Channels {
		if channel.Manager == manager {
			return channel, true
		}
	}
	return updates.Channel{}, false
}

// updateCenterItemForRow resolves the row under the cursor back to the entry it
// was drawn from. The match is the whole row, drawn by the same function the
// screen drew it with: a name that begins with a bracket, a name that carries a
// space, and two managers' entries of the same name are all told apart by it.
func (m Model) updateCenterItemForRow(row string) (updates.Item, bool) {
	for _, item := range m.updateCenterVisibleItems() {
		if updateCenterItemRow(item, m.UpdateCenter.Selected.Has(item), m.updateCenterRowMeasure()) == row {
			return item, true
		}
	}
	return updates.Item{}, false
}

// updateCenterCycleFilter advances the filter to the next manager, and back to
// every manager after the last one.
func (m *Model) updateCenterCycleFilter() {
	managers := m.updateCenterManagersWithItems()
	if len(managers) < 2 {
		return
	}
	if m.UpdateCenter.Filter == "" {
		m.UpdateCenter.Filter = managers[0]
		return
	}
	for index, manager := range managers {
		if manager != m.UpdateCenter.Filter {
			continue
		}
		if index+1 < len(managers) {
			m.UpdateCenter.Filter = managers[index+1]
		} else {
			m.UpdateCenter.Filter = ""
		}
		return
	}
	// The filtered manager has no entries any more -- the last read moved it --
	// so the filter goes back to every manager rather than showing an empty list
	// with a filter that cannot be left.
	m.UpdateCenter.Filter = ""
}

// updateCenterPlanSelection builds the commands the selection needs and moves to
// the confirmation state. A selection nothing can move -- every entry pinned --
// stays on the list with the reason, because a confirmation that leads to
// nothing would be a screen asking about a run that cannot happen.
func (m *Model) updateCenterPlanSelection() {
	engine := m.UpdateCenter.Engine
	if engine == nil {
		return
	}
	items := m.UpdateCenter.Selected.Selected(m.updateCenterItems())
	plans, leftOut := engine.PlansFor(items)

	m.UpdateCenter.Plans = plans
	m.UpdateCenter.LeftOut = leftOut
	// A new plan replaces the last run's report as well: what the previous plan could
	// not carry is no answer about this one.
	m.UpdateCenter.NotReached = nil
	if len(plans) == 0 {
		m.UpdateCenter.Confirming = false
		m.UpdateCenter.Notice = m.updateCenterNothingToApplyReason()
		m.Cursor = 0
		return
	}
	m.UpdateCenter.Confirming = true
	m.Cursor = 0
}

// updateCenterNothingToApplyReason says why a selection produced no command.
func (m Model) updateCenterNothingToApplyReason() string {
	if len(m.UpdateCenter.LeftOut) == 0 {
		return "Nothing was selected, so there is no command to run."
	}
	reasons := make([]string, 0, len(m.UpdateCenter.LeftOut))
	for _, leftOut := range m.UpdateCenter.LeftOut {
		reasons = append(reasons, fmt.Sprintf("%s (%s)", leftOut.Item.Name, leftOut.Reason))
	}
	return "Nothing selected can be moved: " + strings.Join(reasons, "; ")
}

// ---------------------------------------------------------------------------
// The description
// ---------------------------------------------------------------------------

// updateCenterDescription is what the screen says about itself. Every claim
// names its source: the manager that answered, the command it was asked with, and
// when. When nothing could be read, the reason is on the screen rather than
// behind a row that opens onto nothing.
func (m Model) updateCenterDescription() []string {
	state := m.UpdateCenter
	var paragraphs []string

	switch {
	case state.Applying:
		paragraphs = append(paragraphs, m.updateCenterRunningParagraphs()...)
		return append([]string{"Reads what this machine's managers say is out of date and moves only what you select; nothing changes until a selection is confirmed."}, paragraphs...)
	case state.Confirming:
		paragraphs = append(paragraphs, m.updateCenterPlanParagraphs()...)
		return append([]string{"Reads what this machine's managers say is out of date and moves only what you select; nothing changes until a selection is confirmed."}, paragraphs...)
	}

	// The entry nearest the cursor comes FIRST: it is the one thing the reader is
	// deciding about, and at the floor the detail band is four rows -- a generic
	// introduction would spend them before a single fact arrived.
	if item, ok := m.updateCenterDetailItem(); ok {
		paragraphs = append(paragraphs, m.updateCenterItemFacts(item)...)
	}
	paragraphs = append(paragraphs, "Reads what this machine's managers say is out of date and moves only what you select; nothing changes until a selection is confirmed.")

	if !state.Resolved {
		return append(paragraphs, "Reading which managers this machine has…")
	}

	if len(state.Channels) == 0 {
		return append(paragraphs, "No package manager or toolchain manager this build knows could be detected here.")
	}

	// The entry under the cursor was already placed before the introduction, so
	// there is no second copy of its facts here.

	for _, channel := range state.Channels {
		paragraphs = append(paragraphs, m.updateCenterChannelParagraphs(channel)...)
	}

	if !state.CheckedAt.IsZero() {
		paragraphs = append(paragraphs, fmt.Sprintf("Read at %s.", state.CheckedAt.Format("2006-01-02 15:04")))
	}
	if state.InFlight {
		if state.Pending != "" {
			paragraphs = append(paragraphs, "Reading "+string(state.Pending)+"…")
		}
	}
	if count := state.Selected.Count(); count > 0 {
		paragraphs = append(paragraphs, fmt.Sprintf(
			"%d entr%s selected. Applying runs the owning manager's own command for them, one manager at a time.",
			count, plural(count, "y is", "ies are")))
	}
	if len(state.LeftOut) > 0 {
		for _, leftOut := range state.LeftOut {
			paragraphs = append(paragraphs, fmt.Sprintf("Left alone: %s — %s.", leftOut.Item.Name, leftOut.Reason))
		}
	}
	return paragraphs
}

// updateCenterChannelParagraphs is one manager's part of the inventory: the line
// that says what it answered, with the command it was asked with, or why it could
// not be asked -- followed by the manager's own caveats about that answer.
//
// The caveats are not part of the answer's line on purpose. apt says which entries
// it is keeping back, pacman says when it could not read its ignore list,
// Homebrew says when its metadata was not refreshed, flatpak says when it assumed
// an installation: each one is a statement that the list is incomplete, stale or
// built on an assumption, and a screen that drops them presents an incomplete
// answer as a complete one.
func (m Model) updateCenterChannelParagraphs(channel updates.Channel) []string {
	if !channel.Available {
		kind := "is not installed here"
		if channel.Status == updates.StatusUnsupported {
			kind = "does not serve this host"
		}
		return []string{fmt.Sprintf("%s %s: %s.", channel.Label, kind, channel.Reason)}
	}

	result, answered := m.updateCenterResult(channel.Manager)
	if !answered {
		return []string{fmt.Sprintf("%s: not read yet.", channel.Label)}
	}

	var line string
	switch {
	case result.Status == updates.StatusUpdates:
		line = fmt.Sprintf("%s: %s available.", channel.Label,
			countedNoun(result.Updates(), "update is", "updates are"))
	case result.Status == updates.StatusCurrent:
		line = fmt.Sprintf("%s: nothing to update.", channel.Label)
	case result.Status == updates.StatusUnavailable:
		line = fmt.Sprintf("%s: not available here — %s.", channel.Label, result.Reason)
	default:
		line = fmt.Sprintf("%s: could not be read — %s.", channel.Label, result.Reason)
	}
	if result.Command != "" {
		line += " Read with " + result.Command + "."
	}

	paragraphs := []string{line}
	for _, note := range result.Notes {
		paragraphs = append(paragraphs, "   note: "+note)
	}
	return paragraphs
}

// updateCenterPlanParagraphs is the plan the confirmation state shows: every
// command exactly as it will run, what it moves, who it needs to be, what stays
// pending afterwards, and what going back would cost. It is the whole of the
// screen's promise -- that a change is shown before it happens.
func (m Model) updateCenterPlanParagraphs() []string {
	state := m.UpdateCenter
	paragraphs := []string{fmt.Sprintf(
		"%d command%s will run, one at a time, in this order. Nothing else on the machine is touched.",
		len(state.Plans), plural(len(state.Plans), "", "s"))}

	for index, plan := range state.Plans {
		paragraphs = append(paragraphs, fmt.Sprintf("%d. %s", index+1, plan.Display()))
		if len(plan.Items) > 0 {
			paragraphs = append(paragraphs, "   moves: "+updateCenterItemNames(plan.Items))
		}
		if plan.Elevation() {
			needs := "needs administrator permission, asked for non-interactively"
			if !m.updateCenterSudoAvailable() {
				needs = "needs administrator permission, and this machine has no sudo on PATH, so the command will be refused"
			}
			paragraphs = append(paragraphs, "   "+needs+".")
		}
		if plan.Restart != "" {
			paragraphs = append(paragraphs, "   afterwards: "+plan.Restart)
		}
		if plan.Rollback != "" {
			paragraphs = append(paragraphs, "   going back: "+plan.Rollback)
		}
		for _, note := range plan.Notes {
			paragraphs = append(paragraphs, "   note: "+note)
		}
	}

	for _, leftOut := range state.LeftOut {
		paragraphs = append(paragraphs, fmt.Sprintf("Left alone: %s — %s.", leftOut.Item.Name, leftOut.Reason))
	}
	return paragraphs
}

// updateCenterRunningParagraphs is what the running state shows: which command of
// the sequence is running, how to stop it, and the end of what it has printed.
func (m Model) updateCenterRunningParagraphs() []string {
	state := m.UpdateCenter
	paragraphs := []string{"An update is running. Esc does not leave this screen while it does: stop it with the row below, or wait for it to finish."}
	if state.PlanIndex >= 0 && state.PlanIndex < len(state.Plans) {
		paragraphs = append(paragraphs, fmt.Sprintf(
			"Running %d of %d: %s", state.PlanIndex+1, len(state.Plans), state.Plans[state.PlanIndex].Display()))
	}
	if lines := updates.Tail(strings.Join(state.Log, "\n"), updateCenterLogRows); len(lines) > 0 {
		paragraphs = append(paragraphs, lines...)
	} else {
		paragraphs = append(paragraphs, "Waiting for the manager's first line…")
	}
	if len(state.Applied) > 0 {
		paragraphs = append(paragraphs, m.updateCenterAppliedSummary()...)
	}
	return paragraphs
}

// updateCenterAppliedSummary is the running screen's running score, so the user
// can see what the finished commands did before the last one ends.
func (m Model) updateCenterAppliedSummary() []string {
	var lines []string
	for _, applied := range m.UpdateCenter.Applied {
		lines = append(lines, fmt.Sprintf("%s: %d confirmed, %d not moved.",
			updateCenterManagerLabel(m, applied.Plan.Manager), applied.Confirmed(), applied.Failed()))
	}
	return lines
}

// updateCenterManagerLabel names a manager the way its own channel does, falling
// back to its identity when the inventory does not know it.
func updateCenterManagerLabel(m Model, manager updates.ManagerID) string {
	if channel, ok := m.updateCenterChannel(manager); ok {
		return channel.Label
	}
	return string(manager)
}

// updateCenterSudoAvailable reports whether the machine carries an elevation
// program at all, so the confirmation can say what will happen instead of leaving
// it to the refusal.
func (m Model) updateCenterSudoAvailable() bool {
	if m.UpdateCenter.Engine == nil {
		return false
	}
	return m.UpdateCenter.Engine.Env.SudoAvailable
}

// updateCenterItemOutcomes is every entry this run was asked to move, with what
// became of it. The manager's own result is used where the command ran; an entry
// whose command was never started is reported as requested, because that is what
// it is -- a request the run did not reach, which is the state a stopped sequence
// leaves behind; and an entry no command carried at all is reported as left out
// with the reason its own manager gave.
//
// The four states are the distinction the screen exists to make. Reading them off
// one list is what keeps the summary from counting an entry that never ran as if
// the manager had refused it.
func (m Model) updateCenterItemOutcomes() []updates.Outcome {
	state := m.UpdateCenter
	var outcomes []updates.Outcome
	reported := map[string]bool{}
	for _, applied := range state.Applied {
		for _, outcome := range applied.Outcomes {
			outcomes = append(outcomes, outcome)
			reported[outcome.Item.Key()] = true
		}
	}
	for _, item := range state.NotReached {
		if reported[item.Key()] {
			continue
		}
		reported[item.Key()] = true
		outcomes = append(outcomes, updates.Outcome{
			Item:   item,
			Step:   updates.StepRequested,
			Detail: "the run was stopped before this command ran",
		})
	}
	for _, plan := range state.Plans {
		for _, item := range plan.Items {
			if reported[item.Key()] {
				continue
			}
			reported[item.Key()] = true
			outcomes = append(outcomes, updates.Outcome{
				Item:   item,
				Step:   updates.StepRequested,
				Detail: "the command that would move this has not run yet",
			})
		}
	}
	for _, leftOut := range state.LeftOut {
		if reported[leftOut.Item.Key()] {
			continue
		}
		reported[leftOut.Item.Key()] = true
		outcomes = append(outcomes, updates.Outcome{
			Item:   leftOut.Item,
			Step:   updates.StepLeftOut,
			Detail: leftOut.Reason,
		})
	}
	return outcomes
}

// updateCenterApplyNotice is the one line the screen shows once the sequence has
// finished: what was requested, what the managers confirmed, what was requested
// and never reached, and that the list is being read again.
func (m Model) updateCenterApplyNotice() string {
	confirmed, executed, failed, requested, leftOut := 0, 0, 0, 0, 0
	for _, outcome := range m.updateCenterItemOutcomes() {
		switch outcome.Step {
		case updates.StepConfirmed:
			confirmed++
		case updates.StepExecuted:
			executed++
		case updates.StepRequested:
			requested++
		case updates.StepLeftOut:
			leftOut++
		default:
			failed++
		}
	}
	parts := []string{fmt.Sprintf("%d confirmed", confirmed)}
	if executed > 0 {
		parts = append(parts, fmt.Sprintf("%d executed but not confirmed", executed))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d not moved", failed))
	}
	if requested > 0 {
		parts = append(parts, countedNoun(requested,
			"entry was requested and never run", "entries were requested and never run"))
	}
	if leftOut > 0 {
		parts = append(parts, fmt.Sprintf("%d left out", leftOut))
	}
	return "Finished: " + strings.Join(parts, ", ") + ". Reading the managers again."
}

// updateCenterScanNotice is the one line a finished reading pass shows: how many
// entries are available, or which channels could not be asked.
func (m Model) updateCenterScanNotice() string {
	items := m.updateCenterItems()
	trouble := 0
	for _, result := range m.UpdateCenter.Results {
		if !result.Status.Settled() {
			trouble++
		}
	}
	switch {
	case len(items) == 0 && trouble == 0:
		return "Every manager that answered is up to date with what it can see."
	case len(items) == 0:
		return fmt.Sprintf("%s could not be asked; the rest are up to date.",
			countedNoun(trouble, "channel", "channels"))
	case trouble == 0:
		return countedNoun(len(items), "update is available", "updates are available") + "."
	default:
		return fmt.Sprintf("%s, and %s could not be asked.",
			countedNoun(len(items), "update is available", "updates are available"),
			countedNoun(trouble, "channel", "channels"))
	}
}

// countedNoun writes a count with the noun form it needs: "1 update is", "3
// updates are". Only the singular spells its number as a word, which is what
// keeps a sentence like "One 1 update is available" from ever being built by
// accident.
func countedNoun(count int, singular, plural string) string {
	if count == 1 {
		return "1 " + singular
	}
	return fmt.Sprintf("%d %s", count, plural)
}

// plural picks between two endings for a count.
func plural(count int, singular, many string) string {
	if count == 1 {
		return singular
	}
	return many
}

// updateCenterItemNames lists the entries a command moves.
func updateCenterItemNames(items []updates.Item) string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Name)
	}
	return strings.Join(names, ", ")
}

// ---------------------------------------------------------------------------
// The screen
// ---------------------------------------------------------------------------

// renderUpdateCenter draws the screen. Its hints name the keys it honours, and
// they are the rows': a switch row carries its state and Space toggles it, so
// Space is offered only while the list has entries; every action row carries its
// own letter, so the footer names that letter; and while the plan is being
// confirmed the two answers carry `y` and `n` with Esc still the way back. Enter
// is not offered for anything: this screen opens no screen of its own and the
// list's rows are switches, not details.
func (m Model) renderUpdateCenter() string {
	// The reference's two-column table and detail card where the frame holds them;
	// the section renderer's one column at the 80x24 floor, where the same rows
	// and the same keys are drawn.
	l := layoutFor(m)
	hints := m.updateCenterHints()
	budget := installerBodyRows(m.Height, m.footerReserveRows(l.Inner, hints))
	if m.updateCenterWide(l, budget) {
		return m.renderUpdateCenterWide(l, budget)
	}
	return m.renderThemeScreen(Model.updateCenterDescription, m.UpdateCenter.Notice, hints)
}

// updateCenterHints is the footer of the state the screen is in. It names the
// keys the handler accepts in that state and nothing else: an action whose row is
// not drawn is not advertised, because a key the footer offers and the screen
// refuses is the same lie as a row that does not say how it is pressed.
func (m Model) updateCenterHints() []installerHint {
	state := m.UpdateCenter
	switch {
	case state.Applying:
		return []installerHint{updateCenterHintStop}
	case state.Confirming:
		return []installerHint{updateCenterHintConfirm, updateCenterHintCancel, hintCancel}
	}

	// The list's own navigation and its one switch come first, in the canonical
	// order: navigation, then the action, then the way back.
	hints := []installerHint{hintMove, updateCenterHintPick}
	if !state.InFlight && len(m.updateCenterSelectableVisible()) > 1 {
		hints = append(hints, updateCenterHintSelectShown)
	}
	if count := state.Selected.Count(); count > 0 && !state.InFlight {
		hints = append(hints, updateCenterHintApply, updateCenterHintClear)
	}
	if !state.InFlight {
		hints = append(hints, updateCenterHintCheck)
	}
	if filter := m.updateCenterFilterRow(); filter != "" {
		hints = append(hints, updateCenterHintFilter)
	}
	return append(hints, hintBack)
}

// This screen's hints, one per command. The letter on each one is the letter its
// row carries, so the footer and the row cannot name different keys, and no
// letter carries two commands on this screen: the entries are switches, the
// actions have their own letters, and `y`/`n` answer the confirmation. None of
// them is a move key: the cursor moves on ↑/k and ↓/j.
var (
	// updateCenterHintPick is this screen's Space hint. The section's other screens
	// use Space to turn a switch off or on; here it takes one entry of the list or
	// puts it back, and the verb says which of the two it is.
	updateCenterHintPick = installerHint{"[Space]", "select or drop"}
	// updateCenterHintCheck asks the managers again. "check" rather than "refresh"
	// because the row asks rather than re-reads what it already has.
	updateCenterHintCheck = installerHint{"[r]", "check"}
	// updateCenterHintSelectShown takes the whole category the filter is showing,
	// or puts it back once it is all taken: the same row, and the same key, doing
	// whichever of the two the selection makes it read.
	updateCenterHintSelectShown = installerHint{"[s]", "select the shown"}
	updateCenterHintApply       = installerHint{"[a]", "apply"}
	updateCenterHintClear       = installerHint{"[c]", "clear"}
	updateCenterHintFilter      = installerHint{"[f]", "filter"}
	// updateCenterHintStop is the running state's key. Escape stops the run too,
	// and the footer names both.
	updateCenterHintStop = installerHint{"[i/Esc]", "stop"}
	// The confirmation's two answers. They are letters rather than Enter because
	// applying is not opening or choosing, and Esc stays the cancel the footer
	// names beside them.
	updateCenterHintConfirm = installerHint{"[y]", "apply the plan"}
	updateCenterHintCancel  = installerHint{"[n]", "go back to the list"}
)

// ---------------------------------------------------------------------------
// The keys
// ---------------------------------------------------------------------------

// handleUpdateCenterKeys drives the screen. It only ever moves the cursor, the
// selection, the filter or the state machine; every command is returned as a
// tea.Cmd, so a manager that takes a minute to answer cannot block the frame.
//
// Every command runs on its own letter, the letter its row carries, and Space
// does the one thing the list's rows are: it flips the switch under the cursor.
// Enter is deliberately not a command here: this screen opens nothing and the
// list holds switches, not details, so Enter has nothing to be.
func (m Model) handleUpdateCenterKeys(key string) (tea.Model, tea.Cmd) {
	options := m.GetCurrentOptions()
	state := &m.UpdateCenter

	switch key {
	case "r":
		// The key the shell audit uses for its own re-run: asking again is the
		// same question whatever is being read.
		if !state.Applying {
			scan := m.startUpdateCenterScan()
			return m, scan
		}
	case "s":
		// The category row: the whole of what the filter shows, or nothing of it
		// once it is all taken. It is not offered mid-read, where the count it
		// would take is a number the read has not finished.
		if state.Applying || state.Confirming || state.InFlight || len(m.updateCenterSelectableVisible()) < 2 {
			return m, nil
		}
		m.updateCenterToggleVisibleSelection()
	case "a":
		// The row is not drawn while a manager is still being read, and the
		// handler refuses it anyway: a row the screen declines to draw is not a
		// guarantee, and this is the one action that runs commands.
		if state.Selected.Count() == 0 || state.Applying || state.Confirming || state.InFlight {
			return m, nil
		}
		m.updateCenterPlanSelection()
	case "c":
		if state.Applying || state.Confirming || state.Selected.Count() == 0 {
			return m, nil
		}
		state.Selected.Clear()
	case "f":
		if state.Applying || state.Confirming {
			return m, nil
		}
		m.updateCenterCycleFilter()
		m.Cursor = 0
	case "i":
		// The stop is the running state's one action, and only while it runs.
		if state.Applying {
			return m.stopUpdateCenterRun()
		}
	case "y":
		if !state.Confirming {
			return m, nil
		}
		apply := m.startUpdateCenterApply()
		return m, apply
	case "n":
		if !state.Confirming {
			return m, nil
		}
		state.Confirming = false
		state.Plans = nil
		state.LeftOut = nil
		m.Cursor = 0
	case "up", "k":
		m.Cursor = m.updateCenterStep(m.Cursor, -1)
	case "down", "j":
		m.Cursor = m.updateCenterStep(m.Cursor, 1)
	case " ":
		// Space is the switch, and only the switch: on a command row, a
		// separator or the way back it does nothing rather than becoming a
		// second Enter.
		if m.Cursor < 0 || m.Cursor >= len(options) {
			return m, nil
		}
		selected := options[m.Cursor]
		if !isUpdateCenterItemRow(selected) {
			return m, nil
		}
		item, ok := m.updateCenterItemForRow(selected)
		if !ok {
			return m, nil
		}
		if item.Pinned {
			// A pinned entry is shown, and selecting it would be selecting
			// something this program will not move. The refusal is the
			// manager's own hold, said in its own words.
			state.Notice = fmt.Sprintf("%s is %s.", item.Name, item.PinReason)
			return m, nil
		}
		state.Selected.Toggle(item)
	case "esc":
		switch {
		case state.Confirming:
			// The confirmation is one level in: Esc is the cancel its footer
			// names, and it puts the list back exactly as the selection left it.
			state.Confirming = false
			state.Plans = nil
			state.LeftOut = nil
			m.Cursor = 0
		case state.Applying:
			// Escape stops a run in progress, the way it does on the doctor and the
			// shell audit: leaving would hide the only screen that can say what is
			// happening, so the key that would leave stops it instead.
			return m.stopUpdateCenterRun()
		default:
			m.Screen = ScreenUtilities
			m.Cursor = 0
			state.Notice = ""
		}
	}

	return m, nil
}

// isUpdateCenterItemRow reports whether a row is one of the list's entries: the
// three markers are the only thing an entry row starts with, and the marker is
// the switch Space turns.
func isUpdateCenterItemRow(row string) bool {
	return strings.HasPrefix(row, updateCenterRowUnselected) ||
		strings.HasPrefix(row, updateCenterRowSelected) ||
		strings.HasPrefix(row, updateCenterRowHeld)
}

// ---------------------------------------------------------------------------
// The messages
// ---------------------------------------------------------------------------

// updateCenterHandleRead stores one manager's answer and either asks the next
// manager or finishes the pass. The selection is pruned against the fresh answer,
// so an entry the managers no longer report cannot stay selected behind a command
// that would no longer move it.
func (m Model) updateCenterHandleRead(msg updateCenterReadMsg) (tea.Model, tea.Cmd) {
	state := &m.UpdateCenter
	m.storeUpdateCenterResult(msg.result)

	next := m.updateCenterNextManager(msg.manager)
	if next != "" {
		state.Pending = next
		return m, m.updateCenterReadCmd(next)
	}

	state.Pending = ""
	state.InFlight = false
	state.Selected.Retain(m.updateCenterItems())
	if state.Filter != "" && len(m.updateCenterVisibleItems()) == 0 {
		state.Filter = ""
	}
	state.Notice = m.updateCenterScanNotice()
	return m, nil
}

// updateCenterHandleApplyDone stores one plan's result and either runs the next
// plan or ends the sequence.
//
// A cancelled plan ends the sequence rather than starting the next one: the user
// asked for the whole run to stop, and continuing with the rest would be doing
// exactly what they just stopped. The plans that never ran are left out with the
// reason, so the report says which commands were not reached.
func (m Model) updateCenterHandleApplyDone(msg updateCenterApplyDoneMsg) (tea.Model, tea.Cmd) {
	state := &m.UpdateCenter
	state.Applied = append(state.Applied, msg.result)

	if msg.result.Status == updates.StatusCancelled {
		// The plans that were never started are requests the run did not reach,
		// not entries left out: nothing refused them, the stop did.
		for _, plan := range state.Plans[msg.index+1:] {
			state.NotReached = append(state.NotReached, plan.Items...)
		}
	}

	if msg.result.Status != updates.StatusCancelled && msg.index+1 < len(state.Plans) {
		state.PlanIndex = msg.index + 1
		return m, m.updateCenterApplyPlanCmd(msg.index + 1)
	}

	state.Applying = false
	state.PlanIndex = 0
	if state.cancel != nil {
		state.cancel()
	}
	state.cancel, state.ctx = nil, nil
	if state.Lines != nil {
		close(state.Lines)
		state.Lines = nil
	}
	state.Notice = m.updateCenterApplyNotice()

	// The inventory is read again from the managers themselves: what the screen
	// shows next must be the machine's answer, not the plan's expectation, and a
	// plan that failed is exactly the case where the two differ.
	scan := m.startUpdateCenterScan()
	return m, scan
}

// updateCenterAppendLine keeps the tail of a running command's output.
func (m *Model) updateCenterAppendLine(line string) {
	trimmed := strings.TrimRight(line, "\r")
	if strings.TrimSpace(trimmed) == "" {
		return
	}
	m.UpdateCenter.Log = append(m.UpdateCenter.Log, trimmed)
	if len(m.UpdateCenter.Log) > updateCenterLogKept {
		m.UpdateCenter.Log = m.UpdateCenter.Log[len(m.UpdateCenter.Log)-updateCenterLogKept:]
	}
}

// updateCenterPanelValue is the Utilities panel's one line about this screen: what
// the last reading pass found, or that nothing has been read yet. It reads the
// state rather than asking a manager, because the panel is drawn on the section's
// frame and a render must not start a read.
func updateCenterPanelValue(state updateCenterState) string {
	switch {
	case !state.Resolved:
		// "read", not "checked": this screen reads what the managers answer, and the
		// install check's own row already says "not checked yet" about a different
		// question. The two words are the same fact said about different screens, and
		// the panel is read as one list -- install_verify_test.go's guard scans the
		// whole panel -- so one of them has to say it its own way.
		return "not read yet"
	case state.Applying:
		return "an update is running"
	case state.InFlight:
		return "reading the managers…"
	case state.Selected.Count() > 0:
		return fmt.Sprintf("%d selected", state.Selected.Count())
	}

	// An answer and a failure are not the same fact, so they are not counted
	// together: "nothing to update" may only be said about channels that said so
	// themselves. A channel that could not be read is reported beside whatever
	// the others answered.
	available, unread := 0, 0
	for _, result := range state.Results {
		available += result.Updates()
		switch result.Status {
		case updates.StatusUpdates, updates.StatusCurrent:
			// A channel that answered.
		default:
			unread++
		}
	}

	switch {
	case available == 0 && unread == 0:
		return "nothing to update"
	case available == 0:
		return fmt.Sprintf("%d could not be read", unread)
	case unread > 0:
		return fmt.Sprintf("%d update%s available, %d could not be read",
			available, plural(available, "", "s"), unread)
	}
	if available == 1 {
		return "1 update available"
	}
	return fmt.Sprintf("%d updates available", available)
}
