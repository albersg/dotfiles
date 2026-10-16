package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/albersg/dotfiles/installer/internal/updates"
	tea "github.com/charmbracelet/bubbletea"
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
	// utilitiesUpdatesRow is the section's row for this screen. It names what the
	// screen does rather than the feature, in the same voice as the section's
	// other rows.
	utilitiesUpdatesRow = "Check the installed tools for updates"

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
	updateCenterCheckRow = "Check for updates again"
	// updateCenterClearRow drops the whole selection. It is offered only when
	// there is one, so the row count never changes for a press that would do
	// nothing.
	updateCenterClearRow = "Clear the selection"
	// updateCenterApplyRowPrefix leads the row that confirms a selection. Its
	// count is the selection's own count, never a number kept beside it.
	updateCenterApplyRowPrefix = "Apply the selected updates ("
	// updateCenterFilterRowPrefix leads the one filter row, whose value cycles
	// through the managers that have entries. One row rather than one row per
	// manager: the filter is a view of the same list, not a second list.
	updateCenterFilterRowPrefix = "Filter: "
	// updateCenterFilterEvery is the filter's value when every manager is shown.
	updateCenterFilterEvery = "every manager"
	// updateCenterConfirmRow and updateCenterCancelConfirmRow are the
	// confirmation state's two rows. Confirming is a state and not a keystroke:
	// the command the run would execute is on the screen while the question is
	// asked.
	updateCenterConfirmRow       = "Yes, apply the plan"
	updateCenterCancelConfirmRow = "No, go back to the list"
	// updateCenterStopRow is the running state's only row. It asks the manager to
	// stop the way an interrupt does -- finish what you are doing and stop -- and
	// the result of that request is what the screen reports.
	updateCenterStopRow = "Stop the update that is running"

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
	if state.Applying || state.Engine == nil || len(state.Plans) == 0 {
		return nil
	}

	state.Applying = true
	state.Confirming = false
	state.PlanIndex = 0
	state.Applied = nil
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

// ---------------------------------------------------------------------------
// The rows
// ---------------------------------------------------------------------------

// updateCenterRows builds the screen's rows from its state. The item rows come
// first, grouped by manager with a rule between the groups; the action rows
// follow under one rule; the way back is last. Nothing here is typed twice: the
// rows the screen draws and the rows the handler matches are the same function's
// output.
func (m Model) updateCenterRows() []string {
	state := m.UpdateCenter

	switch {
	case state.Applying:
		return []string{updateCenterStopRow}
	case state.Confirming:
		return []string{updateCenterConfirmRow, updateCenterCancelConfirmRow}
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
		rows = append(rows, updateCenterItemRow(item, state.Selected.Has(item)))
	}

	var actions []string
	if state.Selected.Count() > 0 {
		actions = append(actions, updateCenterApplyRow(state.Selected.Count()), updateCenterClearRow)
	}
	if !state.InFlight {
		actions = append(actions, updateCenterCheckRow)
	}
	if filter := m.updateCenterFilterRow(); filter != "" {
		actions = append(actions, filter)
	}
	if len(actions) > 0 {
		if len(rows) > 0 {
			rows = append(rows, m.menuSeparator())
		}
		rows = append(rows, actions...)
	}
	if len(rows) > 0 {
		rows = append(rows, m.menuSeparator())
	}
	return append(rows, "← Back")
}

// updateCenterItemRow renders one entry: its selection, its name, the version it
// is on and the version available, and where it belongs. The manager and the
// scope are in the row rather than only in a group header, because a row that
// does not say who owns it is a row that cannot be acted on with confidence --
// and because two managers that both carry a "git" are two entries this screen
// must never present as one.
func updateCenterItemRow(item updates.Item, selected bool) string {
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
	return fmt.Sprintf("%s%s  %s  %s", mark, item.Name, versions, item.Origin())
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
		if updateCenterItemRow(item, m.UpdateCenter.Selected.Has(item)) == row {
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
	paragraphs := []string{
		"This reads what the package managers and toolchain managers installed on this machine say is out of " +
			"date, and moves only what you select, through the manager that owns each entry. Nothing is " +
			"installed, removed or changed until a selection is confirmed, and nothing is updated on its own.",
	}

	switch {
	case state.Applying:
		paragraphs = append(paragraphs, m.updateCenterRunningParagraphs()...)
		return paragraphs
	case state.Confirming:
		paragraphs = append(paragraphs, m.updateCenterPlanParagraphs()...)
		return paragraphs
	}

	if !state.Resolved {
		return append(paragraphs, "Reading which managers this machine has…")
	}

	if len(state.Channels) == 0 {
		return append(paragraphs, "No package manager or toolchain manager this build knows could be detected here.")
	}

	for _, channel := range state.Channels {
		paragraphs = append(paragraphs, m.updateCenterChannelParagraph(channel))
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

// updateCenterChannelParagraph is one manager's line of the inventory: what it
// answered, with the command it was asked with, or why it could not be asked.
func (m Model) updateCenterChannelParagraph(channel updates.Channel) string {
	if !channel.Available {
		kind := "is not installed here"
		if channel.Status == updates.StatusUnsupported {
			kind = "does not serve this host"
		}
		return fmt.Sprintf("%s %s: %s.", channel.Label, kind, channel.Reason)
	}

	result, answered := m.updateCenterResult(channel.Manager)
	if !answered {
		return fmt.Sprintf("%s: not read yet.", channel.Label)
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
	return line
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

// updateCenterApplyNotice is the one line the screen shows once the sequence has
// finished: what was requested, what the managers confirmed, and that the list is
// being read again.
func (m Model) updateCenterApplyNotice() string {
	confirmed, executed, failed, leftOut := 0, 0, 0, 0
	for _, applied := range m.UpdateCenter.Applied {
		for _, outcome := range applied.Outcomes {
			switch outcome.Step {
			case updates.StepConfirmed:
				confirmed++
			case updates.StepExecuted:
				executed++
			default:
				failed++
			}
		}
	}
	for _, outcome := range m.UpdateCenter.LeftOut {
		_ = outcome
		leftOut++
	}
	parts := []string{fmt.Sprintf("%d confirmed", confirmed)}
	if executed > 0 {
		parts = append(parts, fmt.Sprintf("%d executed but not confirmed", executed))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d not moved", failed))
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

// renderUpdateCenter draws the screen. Its hints name the keys it honours: while
// the plan is being confirmed, Enter applies it and Esc cancels; while an update
// runs, the stop row and `c` are the only actions, so the footer says so rather
// than advertising a way back that the handler refuses.
func (m Model) renderUpdateCenter() string {
	hints := []installerHint{hintUp, hintDown, updateCenterHintPick, hintSelect, hintBack}
	switch {
	case m.UpdateCenter.Applying:
		hints = []installerHint{updateCenterHintStop}
	case m.UpdateCenter.Confirming:
		hints = []installerHint{updateCenterHintConfirm, hintCancel}
	}
	return m.renderThemeScreen(m.updateCenterDescription(), m.UpdateCenter.Notice, hints)
}

// updateCenterHintPick is this screen's Space hint. The section's other screens
// use Space to turn a switch off or on; here it selects one entry of a list, and
// the verb says which of the two it is.
var updateCenterHintPick = installerHint{"[Space]", "select or drop"}

// updateCenterHintConfirm is the confirmation state's Enter. It applies a plan
// rather than selecting a row, so it names that instead of borrowing "select".
var updateCenterHintConfirm = installerHint{"[Enter]", "apply the plan"}

// updateCenterHintStop is the running state's only key.
var updateCenterHintStop = installerHint{"[c]", "stop the running update"}

// ---------------------------------------------------------------------------
// The keys
// ---------------------------------------------------------------------------

// handleUpdateCenterKeys drives the screen. It only ever moves the cursor, the
// selection, the filter or the state machine; every command is returned as a
// tea.Cmd, so a manager that takes a minute to answer cannot block the frame.
func (m Model) handleUpdateCenterKeys(key string) (tea.Model, tea.Cmd) {
	options := m.GetCurrentOptions()
	state := &m.UpdateCenter

	switch key {
	case "c":
		if state.Applying {
			return m.stopUpdateCenterRun()
		}
	case "r":
		// The key the shell audit uses for its own re-run: asking again is the
		// same question whatever is being read.
		if !state.Applying {
			scan := m.startUpdateCenterScan()
			return m, scan
		}
	case "up", "k":
		if m.Cursor > 0 {
			m.Cursor--
			if strings.HasPrefix(options[m.Cursor], menuSeparatorPrefix) && m.Cursor > 0 {
				m.Cursor--
			}
		}
	case "down", "j":
		if m.Cursor < len(options)-1 {
			m.Cursor++
			if strings.HasPrefix(options[m.Cursor], menuSeparatorPrefix) && m.Cursor < len(options)-1 {
				m.Cursor++
			}
		}
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
			// Leaving while a manager is mid-transaction would hide the one
			// screen that can say what is happening. The refusal says what to do
			// instead.
			state.Notice = "An update is running: stop it with the row below, or wait for it to finish."
		default:
			m.Screen = ScreenUtilities
			m.Cursor = 0
			state.Notice = ""
		}
	case "enter", " ":
		if m.Cursor < 0 || m.Cursor >= len(options) {
			return m, nil
		}
		selected := options[m.Cursor]
		switch {
		case strings.HasPrefix(selected, updateCenterRowUnselected),
			strings.HasPrefix(selected, updateCenterRowSelected),
			strings.HasPrefix(selected, updateCenterRowHeld):
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
		case selected == updateCenterCheckRow:
			scan := m.startUpdateCenterScan()
			return m, scan
		case strings.HasPrefix(selected, updateCenterApplyRowPrefix):
			if state.Selected.Count() == 0 || state.Applying {
				return m, nil
			}
			m.updateCenterPlanSelection()
		case selected == updateCenterClearRow:
			state.Selected.Clear()
		case selected == updateCenterConfirmRow:
			apply := m.startUpdateCenterApply()
			return m, apply
		case selected == updateCenterCancelConfirmRow:
			state.Confirming = false
			state.Plans = nil
			state.LeftOut = nil
			m.Cursor = 0
		case selected == updateCenterStopRow:
			return m.stopUpdateCenterRun()
		case strings.HasPrefix(selected, updateCenterFilterRowPrefix):
			m.updateCenterCycleFilter()
			m.Cursor = 0
		case strings.Contains(selected, "Back"):
			m.Screen = ScreenUtilities
			m.Cursor = 0
			state.Notice = ""
		}
	}

	return m, nil
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
		for _, plan := range state.Plans[msg.index+1:] {
			for _, item := range plan.Items {
				state.LeftOut = append(state.LeftOut, updates.LeftOut{
					Item:   item,
					Reason: "the run was stopped before this command ran",
				})
			}
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
		return "not checked yet"
	case state.Applying:
		return "an update is running"
	case state.InFlight:
		return "reading the managers…"
	case state.Selected.Count() > 0:
		return fmt.Sprintf("%d selected", state.Selected.Count())
	}

	available := 0
	for _, result := range state.Results {
		available += result.Updates()
	}
	if available == 0 {
		return "nothing to update"
	}
	if available == 1 {
		return "1 update available"
	}
	return fmt.Sprintf("%d updates available", available)
}
