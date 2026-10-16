package tui

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/albersg/dotfiles/installer/internal/diskclean"
	tea "github.com/charmbracelet/bubbletea"
)

// ============================================================================
// THE UTILITIES SECTION: WHAT THE DEVELOPMENT STORES ON THIS MACHINE HOLD
// ============================================================================
//
// This is the utility that removes things, so it is the one with the most
// rules around what it will not do. The rules live in internal/diskclean --
// the detector's evidence, the guard's refusals, the plan that is shown before
// anything runs -- and this file is the adapter: the state the screen draws,
// the rows the cursor moves through, and the two commands a press can start.
//
// Nothing here scans, measures or removes anything on the drawing path. A scan
// is a command, a cleanup is a command, and both run off the update loop with
// a context the user can cancel.

// diskCleanRow is one row of the screen, with the action it stands for. The
// cursor moves through these rather than through label strings: a row's label
// carries a count or a size that changes between frames, and matching on it
// would be matching on a thing this screen rewrites.
type diskCleanRow struct {
	label  string
	action diskCleanAction
	// item is the index into DiskClean.Items for an item row, or -1.
	item int
}

// diskCleanAction is what a row does when it is chosen.
type diskCleanAction int

const (
	diskCleanActionNone diskCleanAction = iota
	// diskCleanActionItem is an item row: space toggles its selection.
	diskCleanActionItem
	// diskCleanActionPreviewItem is an item row inside the preview: the cursor can
	// walk it and read its card, and no key changes the frozen plan through it.
	diskCleanActionPreviewItem
	// diskCleanActionScan starts a scan of the machine.
	diskCleanActionScan
	// diskCleanActionPreview builds the dry-run plan of the selection.
	diskCleanActionPreview
	// diskCleanActionConfirm runs that plan.
	diskCleanActionConfirm
	// diskCleanActionCancelPreview leaves the preview without running it.
	diskCleanActionCancelPreview
	// diskCleanActionCancelRun stops a scan or a cleanup in flight.
	diskCleanActionCancelRun
	// diskCleanActionAddRoot adds the directory the installer runs from to the
	// project scope.
	diskCleanActionAddRoot
	// diskCleanActionForgetRoot removes the last added project directory.
	diskCleanActionForgetRoot
	// diskCleanActionBack leaves the screen.
	diskCleanActionBack
)

// diskCleanPhase is which state of the screen is on display.
type diskCleanPhase int

const (
	// diskCleanList is the findings, with the selection on them.
	diskCleanList diskCleanPhase = iota
	// diskCleanPreview is the dry run, waiting for a yes.
	diskCleanPreview
	// diskCleanRunning is a cleanup in flight; the rows are inert except for
	// the one that stops it.
	diskCleanRunning
	// diskCleanResults is what the run did.
	diskCleanResults
)

// The rows this screen names. One constant per row, because the handler and
// the label it matches on have to be the same string.
const (
	diskCleanScanRow   = "Scan for development caches"
	diskCleanRescanRow = "Scan again"
	// diskCleanPreviewRowPrefix is the row that opens the dry run. The count is
	// in the label because the row is a statement about what would be cleaned.
	diskCleanPreviewRowPrefix = "Preview the "
	// diskCleanConfirmRowPrefix is the row that runs it, once the plan is on
	// screen. It exists only in the preview phase.
	diskCleanConfirmRowPrefix = "Clean the "
	diskCleanCancelRow        = "← Cancel and go back"
	diskCleanCancelRunRow     = "← Stop the run"
	diskCleanAddRootPrefix    = "Add the project directory "
	diskCleanForgetRootPrefix = "Forget the project directory "
)

// diskCleanState is everything the screen draws. The environment half is
// resolved once when the model is built -- a render must not call the system
// -- and the scan half is filled by the command's answer.
type diskCleanState struct {
	// Resolved reports whether the environment half has been answered.
	Resolved bool
	// Reason is why the user-level stores cannot be located here, when the
	// home directory is not set. The screen states it and still offers what
	// the system and container stores hold.
	Reason string
	// Phase is which state of the screen is on display.
	Phase diskCleanPhase
	// Scanning reports that a scan is in flight, and Scanned that one has
	// finished at least once.
	Scanning bool
	Scanned  bool
	// Notice is the footer line: what is running, or what the last answer said.
	Notice string
	// Progress is the item being measured or cleaned right now.
	Progress string
	// Items are the findings of the last scan, largest first.
	Items []diskclean.Item
	// Selection is the set of item IDs the user marked. It is empty after
	// every scan, so nothing is ever preselected -- delicate findings included.
	Selection diskclean.Selection
	// Plan is the dry run of the current selection, and Planned says whether
	// one has been built for this scan and selection.
	Plan    diskclean.Plan
	Planned bool
	// CleanReport is the last cleanup's outcome.
	CleanReport diskclean.CleanReport
	// ProjectRoots are the directories the user added to the scope. They are
	// the only place this utility looks for project-level stores.
	ProjectRoots []string
	// Cwd is the directory this installer was started from, offered as a
	// project root. It is resolved once, because a render must not read the
	// process's working directory.
	Cwd string
	// Probe is the machine interface. Nil means the real one; a test injects
	// its own so no test scans or removes anything real.
	Probe *diskclean.Probe
	// Cancel stops the command in flight, when one is running.
	Cancel context.CancelFunc
}

// The messages the two commands send back. Neither carries an error: every
// failure the machine can answer is a fact inside the value -- a scan that
// found nothing, a command that was refused -- and a screen that shows the
// fact does not need a second channel for it.
type diskCleanScanMsg struct {
	items     []diskclean.Item
	cancelled bool
}

type diskCleanProgressMsg struct {
	label string
}

type diskCleanCleanedMsg struct {
	report diskclean.CleanReport
}

// resolveDiskClean answers everything about the screen that does not need the
// machine: the directory the run was started from, and whether the user-level
// stores have a home to be found in.
func resolveDiskClean() diskCleanState {
	state := diskCleanState{Resolved: true, Selection: diskclean.Selection{}}
	if cwd, err := os.Getwd(); err == nil {
		state.Cwd = cwd
	}
	if os.Getenv("HOME") == "" {
		state.Reason = "the home directory is not named ($HOME is empty), so the stores the tools keep under it cannot " +
			"be located; the system package managers' and the container engines' own stores can still be measured."
	}
	return state
}

// diskCleanProbe is the machine interface a command uses.
func (m Model) diskCleanProbe() diskclean.Probe {
	if m.DiskClean.Probe != nil {
		return *m.DiskClean.Probe
	}
	return diskclean.DefaultProbe()
}

// diskCleanFacts is the platform the scan runs on, taken from the detection the
// installer already did rather than a second one.
func (m Model) diskCleanFacts() diskclean.Facts {
	if m.SystemInfo == nil {
		return diskclean.FactsFor(runtime.GOOS, false, false, "")
	}
	return diskclean.FactsFor(runtime.GOOS, m.SystemInfo.IsWSL, m.SystemInfo.IsTermux, m.SystemInfo.Prefix)
}

// diskCleanLabelRelay turns a running command's progress into a message the
// interface can show. It is nil when there is no program to send to, which is
// the case in a test that calls a command directly.
func (m Model) diskCleanLabelRelay() func(string) {
	if m.Program == nil {
		return nil
	}
	program := m.Program
	return func(label string) { program.Send(diskCleanProgressMsg{label: label}) }
}

// startDiskCleanScan begins a scan and returns the command that performs it.
func (m Model) startDiskCleanScan() (Model, tea.Cmd) {
	ctx, cancel := context.WithCancel(context.Background())
	m.DiskClean.Scanning = true
	m.DiskClean.Phase = diskCleanList
	m.DiskClean.Notice = "Scanning the stores this machine has…"
	m.DiskClean.Progress = ""
	m.DiskClean.Planned = false
	m.DiskClean.Plan = diskclean.Plan{}
	m.DiskClean.Cancel = cancel
	m.Cursor = 0
	return m, m.diskCleanScanCmd(ctx)
}

// diskCleanScanCmd detects and measures, off the update loop. The project roots
// are copied out of the model: the command runs in its own goroutine, and the
// model it was built from is a value that may already be stale by then.
func (m Model) diskCleanScanCmd(ctx context.Context) tea.Cmd {
	probe := m.diskCleanProbe()
	opts := diskclean.DetectOptions{
		Facts:        m.diskCleanFacts(),
		ProjectRoots: append([]string(nil), m.DiskClean.ProjectRoots...),
	}
	relay := m.diskCleanLabelRelay()
	var progress func(diskclean.Progress)
	if relay != nil {
		progress = func(p diskclean.Progress) { relay("Measuring " + p.ItemID + "…") }
	}

	return func() tea.Msg {
		items := diskclean.Detect(ctx, probe, opts)
		measured, _ := diskclean.Measure(ctx, probe, items, diskclean.MeasureOptions{Progress: progress})
		diskclean.SortBySize(measured)
		return diskCleanScanMsg{items: measured, cancelled: ctx.Err() != nil}
	}
}

// startDiskCleanPreview builds the dry run. It runs nothing: every removal path
// is put through the guard here, which is what lets the preview say a path
// cannot be removed before the user confirms anything.
func (m Model) startDiskCleanPreview() Model {
	chosen := m.DiskClean.Selection.Selected(m.DiskClean.Items)
	plan := diskclean.BuildPlan(chosen, diskclean.GuardFor(m.diskCleanProbe(), chosen))
	m.DiskClean.Plan = plan
	m.DiskClean.Planned = true
	m.DiskClean.Phase = diskCleanPreview
	m.DiskClean.Notice = ""
	m.Cursor = m.diskCleanFirstItemCursor()
	return m
}

// startDiskCleanClean runs the plan that is on screen.
func (m Model) startDiskCleanClean() (Model, tea.Cmd) {
	if !m.DiskClean.Planned || len(m.DiskClean.Plan.Items) == 0 {
		return m, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.DiskClean.Phase = diskCleanRunning
	m.DiskClean.Cancel = cancel
	m.DiskClean.Notice = "Cleaning the items you approved…"
	m.Cursor = 0
	return m, m.diskCleanCleanCmd(ctx)
}

// diskCleanCleanCmd runs the plan, off the update loop. The guard is rebuilt
// from the planned items, so the command verifies every removal against the
// same scopes the preview was checked against.
func (m Model) diskCleanCleanCmd(ctx context.Context) tea.Cmd {
	probe := m.diskCleanProbe()
	plan := m.DiskClean.Plan
	items := make([]diskclean.Item, 0, len(plan.Items))
	for _, planned := range plan.Items {
		items = append(items, planned.Item)
	}
	guard := diskclean.GuardFor(probe, items)
	relay := m.diskCleanLabelRelay()

	opts := diskclean.CleanOptions{}
	if relay != nil {
		opts.Progress = func(done, total int, item diskclean.Item) {
			relay(fmt.Sprintf("Cleaning %s… (%d of %d)", item.Title, done, total))
		}
	}
	return func() tea.Msg {
		return diskCleanCleanedMsg{report: diskclean.Clean(ctx, probe, guard, plan, opts)}
	}
}

// diskCleanRows builds the screen's rows from its state: one row per finding
// with its selection, the rows that act on the selection, the project scope,
// and the way back. The rows and the prose are both drawn from the same state,
// so they cannot disagree about what is selected or what a row would do.
func (m Model) diskCleanRows() []diskCleanRow {
	st := m.DiskClean

	switch st.Phase {
	case diskCleanRunning:
		return []diskCleanRow{{label: diskCleanCancelRunRow, action: diskCleanActionCancelRun}}
	case diskCleanPreview:
		// The preview lists the items it is about, in the plan's own order, so the
		// cursor can walk them and read each one's exact action; the confirmation is
		// the only row that runs anything. The rows carry the item's index into the
		// last scan, which is how the card knows what the cursor is looking at.
		rows := make([]diskCleanRow, 0, len(st.Plan.Items)+3)
		for _, planned := range st.Plan.Items {
			index := diskCleanItemIndex(st.Items, planned.Item.ID)
			if index < 0 {
				continue
			}
			rows = append(rows, diskCleanRow{
				label:  diskCleanPreviewItemRow(planned),
				action: diskCleanActionPreviewItem,
				item:   index,
			})
		}
		rows = append(rows, diskCleanRow{label: m.menuSeparator(), action: diskCleanActionNone, item: -1})
		if len(st.Plan.Items) > 0 {
			rows = append(rows, diskCleanRow{
				label:  diskCleanConfirmRowPrefix + diskCleanCount(len(selectedRunnable(st.Plan.Items))) + " selected",
				action: diskCleanActionConfirm,
				item:   -1,
			})
		}
		return append(rows, diskCleanRow{label: diskCleanCancelRow, action: diskCleanActionCancelPreview, item: -1})
	case diskCleanResults:
		return []diskCleanRow{
			{label: diskCleanRescanRow, action: diskCleanActionScan},
			{label: "← Back", action: diskCleanActionBack},
		}
	}

	rows := []diskCleanRow{}
	scanLabel := diskCleanScanRow
	if st.Scanned {
		scanLabel = diskCleanRescanRow
	}
	rows = append(rows, diskCleanRow{label: scanLabel, action: diskCleanActionScan, item: -1})

	if len(st.Items) > 0 {
		rows = append(rows, diskCleanRow{label: m.menuSeparator(), action: diskCleanActionNone, item: -1})
		for index, item := range st.Items {
			rows = append(rows, diskCleanRow{label: m.diskCleanItemRow(item), action: diskCleanActionItem, item: index})
		}
	}

	selected := st.Selection.Count(st.Items)
	if selected > 0 {
		rows = append(rows,
			diskCleanRow{label: m.menuSeparator(), action: diskCleanActionNone, item: -1},
			diskCleanRow{label: diskCleanPreviewRowPrefix + diskCleanCount(selected) + " selected", action: diskCleanActionPreview, item: -1},
		)
	}

	rows = append(rows, diskCleanRow{label: m.menuSeparator(), action: diskCleanActionNone, item: -1})
	if st.Cwd != "" && !diskCleanHasRoot(st.ProjectRoots, st.Cwd) {
		rows = append(rows, diskCleanRow{label: diskCleanAddRootPrefix + st.Cwd, action: diskCleanActionAddRoot, item: -1})
	}
	if len(st.ProjectRoots) > 0 {
		rows = append(rows, diskCleanRow{
			label:  diskCleanForgetRootPrefix + st.ProjectRoots[len(st.ProjectRoots)-1],
			action: diskCleanActionForgetRoot,
			item:   -1,
		})
	}
	rows = append(rows, diskCleanRow{label: "← Back", action: diskCleanActionBack, item: -1})
	return rows
}

// diskCleanItemRow is one finding as a row: whether it is selected, what it is,
// how much it holds, and how its removal would feel. The size is the
// reclaimable one with the measured source beside it, because a row that
// printed a number without saying where it came from would be the first place
// this utility could mislead.
func (m Model) diskCleanItemRow(item diskclean.Item) string {
	box := "[ ]"
	if m.DiskClean.Selection.Has(item.ID) {
		box = "[x]"
	}
	size := "nothing measured"
	switch {
	case item.Size.Missing:
		size = "not present"
	case item.Size.Err != "":
		size = "not measured"
	case item.Reclaimable() > 0:
		size = humanBytes(uint64(item.Reclaimable()))
		if item.Size.Source == diskclean.SourceTool {
			size += " (tool-reported)"
		}
	}
	row := fmt.Sprintf("%s %s — %s — %s", box, item.Title, size, item.Risk.Label())
	if len(item.Overlaps) > 0 {
		row += " — shares bytes with an item above"
	}
	return row
}

// diskCleanRowLabels is what the menu renderer draws.
func diskCleanRowLabels(rows []diskCleanRow) []string {
	labels := make([]string, 0, len(rows))
	for _, row := range rows {
		labels = append(labels, row.label)
	}
	return labels
}

// diskCleanPreviewItemRow is one item as a preview row: what it is, what it is
// expected to free, and -- in the row itself, so the list reads as the plan --
// the exact thing it would do.
func diskCleanPreviewItemRow(planned diskclean.PlannedItem) string {
	label := planned.Item.Title + " — " + humanBytes(uint64(planned.Item.Reclaimable()))
	switch {
	case !planned.Runs():
		return label + " — nothing would run"
	case planned.Item.Mechanism.Kind == diskclean.MechanismTool:
		return label + " — would run " + planned.Item.Mechanism.Command()
	case planned.Item.Mechanism.Kind == diskclean.MechanismRemove:
		return label + " — would remove " + diskCleanPathCount(len(planned.Item.Paths))
	default:
		return label + " — reported only"
	}
}

// diskCleanItemIndex is where a finding sits in the last scan, so a preview row
// and the card under the cursor can name the same one.
func diskCleanItemIndex(items []diskclean.Item, id string) int {
	for index, item := range items {
		if item.ID == id {
			return index
		}
	}
	return -1
}

// diskCleanFirstItemCursor is the row of the first finding, so the evidence card
// is on screen the moment a scan lands rather than only once the cursor moves.
func (m Model) diskCleanFirstItemCursor() int {
	for index, row := range m.diskCleanRows() {
		if row.action == diskCleanActionItem || row.action == diskCleanActionPreviewItem {
			return index
		}
	}
	return 0
}

// diskCleanHasRoot reports whether a directory is already in the project scope.
func diskCleanHasRoot(roots []string, dir string) bool {
	for _, root := range roots {
		if root == dir {
			return true
		}
	}
	return false
}

// diskCleanCount names a number of items the way a row reads: "1 item",
// "3 items".
func diskCleanCount(n int) string {
	if n == 1 {
		return "1 item"
	}
	return fmt.Sprintf("%d items", n)
}

// diskCleanPathCount names a number of paths the way a row reads.
func diskCleanPathCount(n int) string {
	if n == 1 {
		return "1 path"
	}
	return fmt.Sprintf("%d paths", n)
}

// selectedRunnable filters a plan down to the items that would actually run, so
// a count on a row is about the work and not about the selection.
func selectedRunnable(items []diskclean.PlannedItem) []diskclean.PlannedItem {
	var runnable []diskclean.PlannedItem
	for _, item := range items {
		if item.Runs() {
			runnable = append(runnable, item)
		}
	}
	return runnable
}

// handleDiskCleanerKeys drives the disk cleaner. It starts scans and cleanups as
// commands and never touches the filesystem itself; the two keys the rest of
// the installer does not use here are `a`, which selects the items this utility
// considers safe, and `n`, which clears the selection.
func (m Model) handleDiskCleanerKeys(key string) (tea.Model, tea.Cmd) {
	rows := m.diskCleanRows()

	move := func(delta int) {
		if len(rows) == 0 {
			return
		}
		next := m.Cursor + delta
		for next >= 0 && next < len(rows) && rows[next].action == diskCleanActionNone {
			next += delta
		}
		if next < 0 || next >= len(rows) {
			return
		}
		m.Cursor = next
	}

	switch key {
	case "up", "k":
		move(-1)
	case "down", "j":
		move(1)
	case "esc":
		return m.diskCleanerEscape()
	case " ":
		return m.diskCleanerSpace(rows)
	case "a":
		m = m.diskCleanerSelectSafe()
	case "n":
		m.DiskClean.Selection = diskclean.Selection{}
		m.DiskClean.Notice = "Nothing is selected."
	case "enter":
		if len(rows) == 0 || m.Cursor < 0 || m.Cursor >= len(rows) {
			return m, nil
		}
		return m.diskCleanerChoose(rows[m.Cursor])
	}
	return m, nil
}

// diskCleanerSpace is the space key: on an item row it toggles the selection,
// and anywhere else it does what enter does, which is the convention the rest
// of the installer uses.
func (m Model) diskCleanerSpace(rows []diskCleanRow) (tea.Model, tea.Cmd) {
	if len(rows) == 0 || m.Cursor < 0 || m.Cursor >= len(rows) {
		return m, nil
	}
	row := rows[m.Cursor]
	if row.action == diskCleanActionItem && m.DiskClean.Phase == diskCleanList {
		return m.toggleDiskCleanItem(row.item), nil
	}
	return m.diskCleanerChoose(row)
}

// toggleDiskCleanItem flips one finding's selection. An item nothing can be
// done about cannot be selected at all: a checkbox that does nothing would be
// a promise the run does not keep.
func (m Model) toggleDiskCleanItem(index int) Model {
	if m.DiskClean.Phase != diskCleanList {
		return m
	}
	if index < 0 || index >= len(m.DiskClean.Items) {
		return m
	}
	item := m.DiskClean.Items[index]
	if !diskCleanSelectable(item) {
		m.DiskClean.Notice = fmt.Sprintf("%s cannot be cleaned from here: %s", item.Title, diskCleanUnselectableReason(item))
		return m
	}
	if m.DiskClean.Selection == nil {
		m.DiskClean.Selection = diskclean.Selection{}
	}
	if m.DiskClean.Selection.Has(item.ID) {
		delete(m.DiskClean.Selection, item.ID)
	} else {
		m.DiskClean.Selection[item.ID] = true
	}
	// The selection changed, so the plan that was shown for the old one is no
	// longer the plan: it is dropped rather than left to be confirmed.
	m.DiskClean.Planned = false
	m.DiskClean.Plan = diskclean.Plan{}
	m.DiskClean.Notice = ""
	return m
}

// diskCleanerSelectSafe selects the findings this utility considers safe to
// rebuild. It never selects a careful or delicate one: a bulk key that swept a
// project's dependencies or a container volume along with the caches would be
// the exact failure this utility exists to avoid.
func (m Model) diskCleanerSelectSafe() Model {
	if m.DiskClean.Selection == nil {
		m.DiskClean.Selection = diskclean.Selection{}
	}
	selected := 0
	for _, item := range m.DiskClean.Items {
		if item.Risk != diskclean.RiskSafe || !diskCleanSelectable(item) {
			continue
		}
		m.DiskClean.Selection[item.ID] = true
		selected++
	}
	m.DiskClean.Planned = false
	m.DiskClean.Plan = diskclean.Plan{}
	if selected == 0 {
		m.DiskClean.Notice = "Nothing here is safe to rebuild without a decision from you, so nothing was selected."
		return m
	}
	m.DiskClean.Notice = fmt.Sprintf("Selected the %s that are safe to rebuild; everything careful or delicate is still yours to choose.", diskCleanCount(selected))
	return m
}

// diskCleanerChoose runs the action of the chosen row.
func (m Model) diskCleanerChoose(row diskCleanRow) (tea.Model, tea.Cmd) {
	switch row.action {
	case diskCleanActionItem:
		return m.toggleDiskCleanItem(row.item), nil
	case diskCleanActionScan:
		if m.DiskClean.Scanning {
			// One scan at a time: a second one would measure the same machine
			// twice and leave whichever answer arrived last on screen.
			return m, nil
		}
		return m.startDiskCleanScan()
	case diskCleanActionPreview:
		return m.startDiskCleanPreview(), nil
	case diskCleanActionConfirm:
		return m.startDiskCleanClean()
	case diskCleanActionCancelPreview:
		m.DiskClean.Phase = diskCleanList
		m.DiskClean.Planned = false
		m.DiskClean.Plan = diskclean.Plan{}
		m.DiskClean.Notice = ""
		m.Cursor = 0
		return m, nil
	case diskCleanActionCancelRun:
		if m.DiskClean.Cancel != nil {
			m.DiskClean.Cancel()
			m.DiskClean.Cancel = nil
		}
		m.DiskClean.Notice = "Stopping…"
		return m, nil
	case diskCleanActionAddRoot:
		if m.DiskClean.Cwd == "" || diskCleanHasRoot(m.DiskClean.ProjectRoots, m.DiskClean.Cwd) {
			return m, nil
		}
		m.DiskClean.ProjectRoots = append(m.DiskClean.ProjectRoots, m.DiskClean.Cwd)
		m.DiskClean.Notice = fmt.Sprintf("%s is in the scope now: its build artifacts and dependency trees are part of the next scan. Nothing outside it is searched.", m.DiskClean.Cwd)
		// The new scope changes what a scan would find, so the stale findings
		// are dropped and a fresh scan is started.
		m.DiskClean.Items = nil
		m.DiskClean.Selection = diskclean.Selection{}
		m.DiskClean.Scanned = false
		return m.startDiskCleanScan()
	case diskCleanActionForgetRoot:
		if len(m.DiskClean.ProjectRoots) > 0 {
			removed := m.DiskClean.ProjectRoots[len(m.DiskClean.ProjectRoots)-1]
			m.DiskClean.ProjectRoots = m.DiskClean.ProjectRoots[:len(m.DiskClean.ProjectRoots)-1]
			m.DiskClean.Notice = removed + " is out of the scope; the next scan does not look there."
			m.DiskClean.Items = nil
			m.DiskClean.Selection = diskclean.Selection{}
			m.DiskClean.Scanned = false
			return m.startDiskCleanScan()
		}
	case diskCleanActionBack:
		m.Screen = ScreenUtilities
		m.Cursor = 0
		m.DiskClean.Notice = ""
		return m, nil
	}
	return m, nil
}

// diskCleanerEscape leaves the screen, or walks back one level: escape cancels
// a run in flight, leaves a preview without running it, and otherwise goes back
// to the section.
func (m Model) diskCleanerEscape() (tea.Model, tea.Cmd) {
	switch {
	case m.DiskClean.Scanning || m.DiskClean.Phase == diskCleanRunning:
		if m.DiskClean.Cancel != nil {
			m.DiskClean.Cancel()
			m.DiskClean.Cancel = nil
		}
		m.DiskClean.Notice = "Stopping…"
		return m, nil
	case m.DiskClean.Phase == diskCleanPreview:
		m.DiskClean.Phase = diskCleanList
		m.DiskClean.Planned = false
		m.DiskClean.Plan = diskclean.Plan{}
		m.DiskClean.Notice = ""
		m.Cursor = 0
		return m, nil
	default:
		m.Screen = ScreenUtilities
		m.Cursor = 0
		m.DiskClean.Notice = ""
		return m, nil
	}
}

// diskCleanSelectable reports whether an item can be marked at all: it has a
// mechanism that runs here, and there is something measured to free.
func diskCleanSelectable(item diskclean.Item) bool {
	return item.Cleanable() && item.Size.Err == "" && !item.Size.Missing && item.Reclaimable() > 0
}

// diskCleanUnselectableReason says why a finding's checkbox is inert.
func diskCleanUnselectableReason(item diskclean.Item) string {
	switch {
	case !item.Cleanable():
		return item.Mechanism.Reason
	case item.Size.Missing:
		return "the store is not there any more"
	case item.Size.Err != "":
		return "its size could not be measured: " + item.Size.Err
	default:
		return "it holds nothing to free"
	}
}

// diskCleanScanNotice is the one line a finished scan leaves on the screen: how
// many stores were found and what they hold together. The totals are the
// measured ones, and a scan that found nothing says so rather than leaving the
// footer empty, because an empty footer reads as an unfinished scan.
func diskCleanScanNotice(items []diskclean.Item) string {
	if len(items) == 0 {
		return "No development stores were found on this machine. Nothing here is looked for beyond the stores the " +
			"tools on the PATH keep, the package managers' own caches, the container engines' own report, and the " +
			"project directories you add."
	}
	total := int64(0)
	for _, item := range items {
		total += item.Reclaimable()
	}
	return fmt.Sprintf("Found %s holding %s that could be freed. Nothing is selected: mark what you want, then preview it.",
		diskCleanCount(len(items)), humanBytes(uint64(total)))
}

// diskCleanResultNotice is the one line a finished cleanup leaves: the
// filesystem's own release, or the reason it could not be measured, and how
// many items were not completed.
func diskCleanResultNotice(report diskclean.CleanReport) string {
	lead := "How much was released could not be measured."
	if report.FreedMeasured {
		lead = fmt.Sprintf("The filesystem released %s.", humanBytes(uint64(report.FreedBytes)))
	} else if report.FreedNote != "" {
		lead = report.FreedNote
	}
	if report.Cancelled {
		return lead + " The run was stopped, so the items it had not reached were left alone."
	}
	if failed := report.Failed(); len(failed) > 0 {
		return fmt.Sprintf("%s %s could not be completed; the list below names each one and why.", lead, diskCleanCount(len(failed)))
	}
	return lead + " Everything that ran is listed below."
}

// diskCleanScanCancelledNotice is what a scan that was stopped leaves. What it
// had reached is not a result: it is a part of one, and the line says so rather
// than letting a shorter list read as the whole machine.
func diskCleanScanCancelledNotice(items []diskclean.Item) string {
	if len(items) == 0 {
		return "The scan was cancelled before it found anything, so nothing is listed and nothing here is complete."
	}
	return fmt.Sprintf("The scan was cancelled after %s, so what is listed is only what it had reached: nothing here is complete.",
		diskCleanCount(len(items)))
}

// diskCleanUnavailableReason is why the section would not offer this utility.
// It is empty in every case this build knows: the screen is offered wherever
// the process can read its own working directory, and a host without a home
// directory is told what that means rather than refused.
func (m Model) diskCleanUnavailableReason() string {
	if !m.DiskClean.Resolved {
		return "The development caches are not offered yet: the screen is still being resolved."
	}
	if strings.TrimSpace(m.DiskClean.Reason) != "" {
		return m.DiskClean.Reason
	}
	return ""
}
