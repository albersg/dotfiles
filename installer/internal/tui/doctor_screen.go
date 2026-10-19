package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// ============================================================================
// THE DOCTOR'S SCREEN
// ============================================================================
//
// The screen is the section's fifth job, one level in from the Utilities
// section, and it follows the shape every other utilities screen follows: a
// title, a short body, a list, a notice line, and the keys in the legend.
//
// Three things are its own.
//
// **The list is the findings, and the body is the finding under the cursor.**
// Each row carries the state in a word and a plain mark (never colour alone),
// and the body states the evidence, the impact and what to do -- which is what
// makes a scan readable instead of a wall of rows.
//
// **The run is a small pool of commands.** Several checks are in flight at once,
// each of them its own command, so the interface never waits on one that hangs
// and one slow tool does not hold every answer behind it. The answers land in a
// map keyed by check ID, so the report is still built in the order the
// catalogue runs them. The notice states how many have answered out of how
// many, and Escape cancels: the checks that did not answer are recorded as
// unknown with the reason rather than disappearing.
//
// **A fix is confirmed on its own screen.** The confirmation shows the exact
// effect, the risk, the permission and the backup before anything is changed,
// and the row that applies it is the only row that changes the machine.

// ============================================================================
// State
// ============================================================================

// doctorRun is one scan: what it is running, what came of the checks that
// finished, and how the last press ended.
type doctorRun struct {
	// ID distinguishes this run from the one before it, so a message from a
	// cancelled run is never taken for the current one.
	ID uint64
	// Scan is the pass over this machine the checks read. It is built when the
	// run starts and holds the caches the checks share.
	Scan *doctorScan
	// Order is the check IDs the run covers, in the order the report presents
	// them: the catalogue's order for a full run, and the order a category or a
	// retry added them in.
	Order []string
	// Plan is what this run is still to execute. Next is the index of the next
	// check to start, InFlight is how many started checks have not answered, and
	// Answered is how many of Plan have answered -- which is the number the
	// notice counts, because a pool with four checks out has finished none.
	Plan     []doctorCheck
	Next     int
	InFlight int
	Answered int
	// Results is keyed by check ID so a retry replaces one answer instead of
	// renumbering the report.
	Results map[string]doctorCheckResult
	// Canceled reports that the user stopped the run: the checks that never ran
	// are recorded as unknown.
	Canceled bool
	// Started and Finished bracket the run; Finished is zero while it runs.
	Started  time.Time
	Finished time.Time
	// Message is the notice line: progress while the run is in flight, and the
	// summary once it has ended.
	Message string
	// ReportPath is where the report was written, ReportErr why it was not.
	ReportPath string
	ReportErr  string
}

// running reports whether the run still takes answers.
func (r *doctorRun) running() bool {
	return r != nil && r.Finished.IsZero()
}

// doctorState is everything the doctor's two screens hold.
type doctorState struct {
	// Run is the scan in flight or the last one that finished.
	Run *doctorRun
	// NextRunID is the id the next run gets.
	NextRunID uint64
	// AllFindings reports whether the list shows the clean answers too. The
	// default is the problems: a machine with twenty-seven passing checks would
	// otherwise bury the two that matter.
	AllFindings bool
	// Pending is the finding whose fix the confirmation screen is showing, and
	// PendingPreview the state that was read when it opened.
	Pending        *doctorFinding
	PendingPreview doctorFixPreview
	// FixNotice is the confirmation screen's own notice line.
	FixNotice string
	// Notice is the list screen's line when it is not a run's progress: the
	// export's path, or why it was not written.
	Notice string
}

// ============================================================================
// The list
// ============================================================================

// The rows that act. They are constants because the key handler matches them,
// and the screen draws them from the one list doctorList builds.
const (
	doctorRunAllRow         = "Run every check"
	doctorRunRetryRow       = "Retry the checks that could not run"
	doctorWriteReportRow    = "Write the report to a file"
	doctorFilterProblemsRow = "Show only the problems"
	doctorFilterAllRow      = "Show every finding"
	doctorInfoRow           = "No problem was found by the checks that ran"
)

// doctorRunCategoryPrefix is how a category's own row starts.
const doctorRunCategoryPrefix = "Run only: "

// doctorRowAction names what a row does, so the key handler switches on the
// action and never on the words the row happens to be drawn with.
type doctorRowAction int

const (
	doctorActionNone doctorRowAction = iota
	doctorActionRunAll
	doctorActionRunCategory
	doctorActionRetry
	doctorActionWriteReport
	doctorActionFilterProblems
	doctorActionFilterAll
	doctorActionBack
	doctorActionFix
)

// doctorListRow is one row of the screen's list: a finding, an action, or the
// separator between them. The list is built once for both the drawing and the
// keys, so a row cannot be drawn without the handler knowing what it is.
type doctorListRow struct {
	label     string
	action    doctorRowAction
	category  doctorCategory
	finding   *doctorFinding
	separator bool
}

// doctorList is every row of the screen in order.
func (m Model) doctorList() []doctorListRow {
	run := m.Doctor.Run
	if run == nil {
		rows := []doctorListRow{{label: doctorRunAllRow, action: doctorActionRunAll}}
		for _, category := range doctorCategories() {
			rows = append(rows, doctorListRow{
				label:    doctorRunCategoryPrefix + category.row(),
				action:   doctorActionRunCategory,
				category: category,
			})
		}
		rows = append(rows, doctorListRow{separator: true}, doctorListRow{label: "← Back", action: doctorActionBack})
		return rows
	}

	var rows []doctorListRow
	for _, id := range run.Order {
		result, ok := run.Results[id]
		if !ok {
			continue
		}
		for index := range result.Findings {
			finding := result.Findings[index]
			if !m.Doctor.AllFindings && !doctorFindingNeedsAttention(finding) {
				continue
			}
			rows = append(rows, doctorListRow{
				label:   doctorFindingRow(finding),
				finding: &finding,
			})
		}
	}
	if len(rows) == 0 {
		rows = append(rows, doctorListRow{label: doctorInfoRow})
	}

	rows = append(rows, doctorListRow{separator: true})
	if run.running() {
		rows = append(rows, doctorListRow{label: "← Back", action: doctorActionBack})
		return rows
	}
	rows = append(rows,
		doctorListRow{label: doctorRunAllRow, action: doctorActionRunAll},
		doctorListRow{label: doctorRunCategoryPrefix + "…", action: doctorActionRunCategory},
	)
	if m.doctorRetryableCount() > 0 {
		rows = append(rows, doctorListRow{label: doctorRunRetryRow, action: doctorActionRetry})
	}
	rows = append(rows, doctorListRow{label: doctorWriteReportRow, action: doctorActionWriteReport})
	if m.Doctor.AllFindings {
		rows = append(rows, doctorListRow{label: doctorFilterProblemsRow, action: doctorActionFilterProblems})
	} else {
		rows = append(rows, doctorListRow{label: doctorFilterAllRow, action: doctorActionFilterAll})
	}
	rows = append(rows, doctorListRow{separator: true}, doctorListRow{label: "← Back", action: doctorActionBack})
	return rows
}

// doctorFindingRow is one finding's row: the state as a word and a plain mark,
// then what the finding is about. The word is what makes the state readable on
// a terminal with no colour, which is the rule the whole interface follows.
func doctorFindingRow(finding doctorFinding) string {
	return fmt.Sprintf("%s %s: %s", finding.Status.mark(), finding.Status.label(), finding.Subject)
}

// doctorRows is the list as the frame draws it: the labels of doctorList, with
// the separators the menu style draws a rule for. It is derived from the one
// list the key handler reads, so a row cannot be drawn that the handler does not
// know what to do with.
func (m Model) doctorRows() []string {
	list := m.doctorList()
	rows := make([]string, 0, len(list))
	for _, row := range list {
		if row.separator {
			rows = append(rows, m.menuSeparator())
			continue
		}
		rows = append(rows, row.label)
	}
	return rows
}

// doctorFindingNeedsAttention reports whether a finding is one of the three
// states a reader has to see: an error, a warning, or a question the doctor
// could not answer. The clean and the not-applicable rows are the ones the
// filter hides.
func doctorFindingNeedsAttention(finding doctorFinding) bool {
	switch finding.Status {
	case doctorError, doctorWarning, doctorUnknown:
		return true
	default:
		return false
	}
}

// doctorRetryableCount is how many checks ended without an answer.
func (m Model) doctorRetryableCount() int {
	run := m.Doctor.Run
	if run == nil {
		return 0
	}
	count := 0
	for _, result := range run.Results {
		if result.Status == doctorUnknown {
			count++
		}
	}
	return count
}

// doctorCursorRow is the row the cursor is on, or nil when the cursor is past
// the list (which the guards settle before this is asked).
func (m Model) doctorCursorRow() *doctorListRow {
	list := m.doctorList()
	if m.Cursor < 0 || m.Cursor >= len(list) {
		return nil
	}
	return &list[m.Cursor]
}

// doctorCursorFinding is the finding under the cursor, or nil on an action row.
func (m Model) doctorCursorFinding() *doctorFinding {
	row := m.doctorCursorRow()
	if row == nil {
		return nil
	}
	return row.finding
}

// settleDoctorCursor moves the cursor to a row the frame can still draw: after a
// filter change or an action row that vanished, the cursor would otherwise point
// past the list. It prefers the nearest row that is a finding, so the body keeps
// describing something.
func (m *Model) settleDoctorCursor() {
	list := m.doctorList()
	if m.Cursor > len(list)-1 {
		m.Cursor = len(list) - 1
	}
	if m.Cursor < 0 {
		m.Cursor = 0
	}
	if row := m.doctorCursorRow(); row != nil && row.separator {
		for index := m.Cursor + 1; index < len(list); index++ {
			if !list[index].separator {
				m.Cursor = index
				return
			}
		}
		for index := m.Cursor - 1; index >= 0; index-- {
			if !list[index].separator {
				m.Cursor = index
				return
			}
		}
	}
}

// ============================================================================
// The run
// ============================================================================

// doctorConcurrency is how many checks the walk keeps in flight at once. Four
// is enough that one slowed check holds back three answers instead of every
// answer behind it -- the walk measured 27 checks and 2.3-3.1 s one at a time,
// and the worst case, every check reaching its own bound, was 27 x 10 s -- and
// small enough that a walk still reads as one walk: the machine sees four
// bounded commands, never an unbounded scan. The per-check bounds
// (doctorVersionTimeout, doctorShellStartTimeout, doctorProbeTimeout) do not
// move with it; the whole walk is bounded by ceil(checks/concurrency) x the
// longest bound.
const doctorConcurrency = 4

// doctorCheckDoneMsg carries one finished check back to the update loop.
type doctorCheckDoneMsg struct {
	RunID  uint64
	Index  int
	Result doctorCheckResult
}

// doctorFixDoneMsg carries a fix's outcome back to the update loop.
type doctorFixDoneMsg struct {
	Outcome doctorFixOutcome
	Finding doctorFinding
}

// doctorReportWrittenMsg carries the export's path -- or its failure -- back.
type doctorReportWrittenMsg struct {
	Path string
	Err  string
}

// doctorStartRun schedules checks. A fresh run forgets every previous answer;
// a retry or a single check keeps the answers it does not replace, which is what
// lets a fixed finding be re-checked on its own.
func (m *Model) doctorStartRun(checks []doctorCheck, fresh bool) tea.Cmd {
	if len(checks) == 0 {
		return nil
	}
	if m.Doctor.Run == nil || fresh {
		m.Doctor.NextRunID++
		m.Doctor.Run = &doctorRun{ID: m.Doctor.NextRunID, Results: map[string]doctorCheckResult{}}
	} else {
		// The plan is being replaced on the run that is already there -- a retry, or
		// a category run -- and the commands of the plan it replaces may still be in
		// flight. They carry the ID it was started with, so a new ID is what drops
		// their answers instead of counting them against a plan they were not
		// started for: counted, they can close the new plan before its own checks
		// have answered.
		m.Doctor.NextRunID++
		m.Doctor.Run.ID = m.Doctor.NextRunID
	}
	run := m.Doctor.Run
	if fresh {
		run.Order = nil
	}
	run.Scan = newDoctorScan(osDoctorHost{}, m.SystemInfo, time.Now())
	run.Plan = checks
	run.Next, run.InFlight, run.Answered = 0, 0, 0
	run.Canceled = false
	run.Started = time.Now()
	run.Finished = time.Time{}
	run.ReportPath, run.ReportErr = "", ""
	for _, check := range checks {
		seen := false
		for _, id := range run.Order {
			if id == check.ID {
				seen = true
				break
			}
		}
		if !seen {
			run.Order = append(run.Order, check.ID)
		}
		delete(run.Results, check.ID)
	}
	cmd := m.doctorScheduleCmd()
	run.Message = m.doctorProgressLine()
	m.Doctor.Notice = ""
	return cmd
}

// doctorProgressLine is what the notice says while a run is in flight: how many
// checks have answered out of how many, how many are out at once, and the check
// the pool started last, so a slow machine is legible rather than frozen. The
// count is answers and not starts: a pool with four checks out has finished
// none of them, and a line that claimed otherwise would be the old frozen one
// in different words.
func (m Model) doctorProgressLine() string {
	run := m.Doctor.Run
	if run == nil || len(run.Plan) == 0 {
		return ""
	}
	started := run.Next - 1
	if started < 0 {
		started = 0
	}
	if started > len(run.Plan)-1 {
		started = len(run.Plan) - 1
	}
	if run.InFlight > 1 {
		return fmt.Sprintf("Checked %d of %d, %d at once: %s…",
			run.Answered, len(run.Plan), run.InFlight, run.Plan[started].Title)
	}
	return fmt.Sprintf("Checked %d of %d: %s…", run.Answered, len(run.Plan), run.Plan[started].Title)
}

// doctorScheduleCmd starts as many pending checks as the pool has room for, and
// hands them to the update loop together. Each check is its own command, so the
// loop never waits on one that hangs; the batch is what lets several be in
// flight at once, because the runtime runs every command of a batch in its own
// goroutine. It starts none when the run has ended, was cancelled, or has
// nothing left, which is what keeps a cancelled run from starting more work.
func (m Model) doctorScheduleCmd() tea.Cmd {
	run := m.Doctor.Run
	if run == nil || !run.running() || run.Canceled {
		return nil
	}
	var cmds []tea.Cmd
	for run.InFlight < doctorConcurrency && run.Next < len(run.Plan) {
		cmds = append(cmds, m.doctorCheckCmdAt(run.Next))
		run.Next++
		run.InFlight++
	}
	switch len(cmds) {
	case 0:
		return nil
	case 1:
		return cmds[0]
	}
	return tea.Batch(cmds...)
}

// doctorCheckCmdAt runs one check off the update loop. The index is the check's
// place in the plan, and it travels with the answer so the run can say where an
// answer came from even though the answers arrive out of order.
func (m Model) doctorCheckCmdAt(index int) tea.Cmd {
	run := m.Doctor.Run
	if run == nil || index < 0 || index >= len(run.Plan) {
		return nil
	}
	scan := run.Scan
	check := run.Plan[index]
	runID := run.ID
	return func() tea.Msg {
		return doctorCheckDoneMsg{RunID: runID, Index: index, Result: runDoctorCheck(scan, check)}
	}
}

// doctorFinishRun closes the run: it fills in the checks that never ran, builds
// the report, writes it, and sets the notice.
func (m *Model) doctorFinishRun(reason string) {
	run := m.Doctor.Run
	if run == nil {
		return
	}
	run.Finished = time.Now()

	for _, id := range run.Order {
		if _, ok := run.Results[id]; ok {
			continue
		}
		check, found := doctorCheckByID(id)
		if !found {
			continue
		}
		why := reason
		if why == "" {
			why = "the run ended before this check ran"
		}
		run.Results[id] = notRunResult(check, why)
	}

	report := run.report()
	path, writeErr := report.write(doctorReportPath())
	run.ReportPath, run.ReportErr = path, ""
	if writeErr != nil {
		run.ReportErr = writeErr.Error()
	}
	counts := report.counts()
	run.Message = fmt.Sprintf("%s: %s", doctorCounted(len(run.Order), "check", "checks"), counts.summary())
	if run.ReportErr != "" {
		run.Message += ". The report could not be written (" + run.ReportErr + ")"
	} else if run.ReportPath != "" {
		run.Message += ". Kept at " + doctorRelHomePath(run.ReportPath)
	}
	if run.Canceled {
		run.Message = "Cancelled after " + fmt.Sprintf("%d of %d", run.Answered, len(run.Plan)) + ". " + run.Message
	}
	m.settleDoctorCursor()
}

// report is the run as a report, in the catalogue's order.
func (r *doctorRun) report() doctorReport {
	report := doctorReport{
		Time:    r.Started,
		Build:   BuildLabel(),
		Checked: len(r.Order),
	}
	if r.Scan != nil {
		report.Platform = r.Scan.platformLabel()
	}
	for _, id := range r.Order {
		result, ok := r.Results[id]
		if !ok {
			if check, found := doctorCheckByID(id); found {
				result = notRunResult(check, "the run ended before this check ran")
			} else {
				continue
			}
		}
		report.Results = append(report.Results, result)
	}
	return report
}

// doctorCheckByID finds a check in the catalogue.
func doctorCheckByID(id string) (doctorCheck, bool) {
	for _, check := range doctorChecks() {
		if check.ID == id {
			return check, true
		}
	}
	return doctorCheck{}, false
}

// doctorChecksForCategory is one category's checks, for the section's own rows.
func doctorChecksForCategory(category doctorCategory) []doctorCheck {
	var checks []doctorCheck
	for _, check := range doctorChecks() {
		if check.Category == category {
			checks = append(checks, check)
		}
	}
	return checks
}

// doctorUnknownChecks is the checks whose last answer was unknown: the retry
// row's plan.
func (m Model) doctorUnknownChecks() []doctorCheck {
	run := m.Doctor.Run
	if run == nil {
		return nil
	}
	var checks []doctorCheck
	for _, id := range run.Order {
		result, ok := run.Results[id]
		if !ok || result.Status != doctorUnknown {
			continue
		}
		if check, found := doctorCheckByID(id); found {
			checks = append(checks, check)
		}
	}
	return checks
}

// ============================================================================
// The commands
// ============================================================================

// doctorWriteReportCmd writes the exported report. The export is the same
// renderer as the kept report -- one function, so the file on screen and the
// file on disk cannot diverge -- with the stamp in the name so two exports are
// two files.
func (m Model) doctorWriteReportCmd() tea.Cmd {
	run := m.Doctor.Run
	if run == nil {
		return nil
	}
	report := run.report()
	return func() tea.Msg {
		path := doctorExportPath(time.Now())
		if path == "" {
			return doctorReportWrittenMsg{Err: "the state directory could not be determined"}
		}
		if _, err := report.write(path); err != nil {
			return doctorReportWrittenMsg{Path: path, Err: err.Error()}
		}
		return doctorReportWrittenMsg{Path: path}
	}
}

// doctorFixCmd applies the pending fix off the update loop. It re-checks the
// state inside apply, so a state that moved between the preview and the press is
// refused rather than written over.
func (m Model) doctorFixCmd() tea.Cmd {
	finding := m.Doctor.Pending
	if finding == nil || finding.Fix == nil {
		return nil
	}
	fix := *finding.Fix
	scan := m.Doctor.Run.Scan
	if scan == nil {
		scan = newDoctorScan(osDoctorHost{}, m.SystemInfo, time.Now())
	}
	pending := *finding
	return func() tea.Msg {
		return doctorFixDoneMsg{Outcome: fix.apply(scan), Finding: pending}
	}
}

// ============================================================================
// The keys
// ============================================================================

// handleDoctorKeys drives the doctor's list.
//
// Nothing here looks at the machine: the run is a command, the export is a
// command and the fix is a command, so a slow disk or a command that hangs
// cannot stop the update loop. Escape cancels a run in flight rather than
// leaving the screen, because a cancelled run is the only way to stop work the
// user just started.
func (m Model) handleDoctorKeys(key string) (tea.Model, tea.Cmd) {
	run := m.Doctor.Run
	switch key {
	case "up", "k":
		m.doctorMoveCursor(-1)
		return m, nil
	case "down", "j":
		m.doctorMoveCursor(1)
		return m, nil
	case "t":
		if run != nil {
			m.Doctor.AllFindings = !m.Doctor.AllFindings
			m.settleDoctorCursor()
		}
		return m, nil
	case "w":
		if run != nil {
			return m, m.doctorWriteReportCmd()
		}
		return m, nil
	case "r":
		if checks := m.doctorUnknownChecks(); len(checks) > 0 {
			return m, m.doctorStartRun(checks, false)
		}
		return m, nil
	case "esc":
		if run != nil && run.running() {
			// The checks in flight cannot be interrupted -- a command already
			// handed to the runtime runs to its own bound -- so the run ends here
			// and the answers still on their way are dropped by the guard in the
			// update loop. What did not answer is recorded as unknown with the
			// reason rather than disappearing.
			run.Canceled = true
			m.doctorFinishRun("the run was cancelled before this check answered")
			return m, nil
		}
		m.Screen = ScreenUtilities
		m.Cursor = 0
		m.Doctor.Notice = ""
		return m, nil
	case "enter", " ":
		row := m.doctorCursorRow()
		if row == nil {
			return m, nil
		}
		switch row.action {
		case doctorActionRunAll:
			if run != nil && run.running() {
				return m, nil
			}
			return m, m.doctorStartRun(doctorChecks(), true)
		case doctorActionRunCategory:
			if run != nil && run.running() {
				return m, nil
			}
			if row.label == doctorRunCategoryPrefix+"…" {
				// The single "one category" row opens the same list the first press
				// did: the categories are the rows above it when no run has happened
				// yet. With a run done, the row is a pointer back to that list, so it
				// starts with the first category.
				return m, m.doctorStartRun(doctorChecksForCategory(doctorCategories()[0]), false)
			}
			return m, m.doctorStartRun(doctorChecksForCategory(row.category), false)
		case doctorActionRetry:
			if checks := m.doctorUnknownChecks(); len(checks) > 0 {
				return m, m.doctorStartRun(checks, false)
			}
			return m, nil
		case doctorActionWriteReport:
			return m, m.doctorWriteReportCmd()
		case doctorActionFilterProblems:
			m.Doctor.AllFindings = false
			m.settleDoctorCursor()
			return m, nil
		case doctorActionFilterAll:
			m.Doctor.AllFindings = true
			m.settleDoctorCursor()
			return m, nil
		case doctorActionBack:
			m.Screen = ScreenUtilities
			m.Cursor = 0
			m.Doctor.Notice = ""
			return m, nil
		}
		if row.finding != nil && row.finding.Fix != nil {
			return m, m.openDoctorFix(*row.finding)
		}
		return m, nil
	}
	return m, nil
}

// doctorMoveCursor moves one row, skipping the separators the list uses to group
// its actions, the same way the utilities section skips its own.
func (m *Model) doctorMoveCursor(delta int) {
	list := m.doctorList()
	next := m.Cursor + delta
	for next >= 0 && next < len(list) && list[next].separator {
		next += delta
	}
	if next < 0 || next >= len(list) {
		return
	}
	m.Cursor = next
}

// openDoctorFix opens the confirmation screen for one finding, reading the state
// the fix would replace. The preview is read here rather than when the screen
// draws, because a render must not touch the filesystem.
func (m *Model) openDoctorFix(finding doctorFinding) tea.Cmd {
	if finding.Fix == nil {
		return nil
	}
	scan := m.Doctor.Run.Scan
	if scan == nil {
		scan = newDoctorScan(osDoctorHost{}, m.SystemInfo, time.Now())
	}
	m.Doctor.Pending = &finding
	m.Doctor.PendingPreview = finding.Fix.preview(scan)
	m.Doctor.FixNotice = ""
	m.Screen = ScreenDoctorFix
	m.Cursor = 0
	return nil
}

// handleDoctorFixKeys drives the confirmation. The only row that changes
// anything is the action; Escape and Back leave with nothing written.
func (m Model) handleDoctorFixKeys(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.Screen = ScreenDoctor
		m.Doctor.Pending = nil
		m.Doctor.FixNotice = ""
		m.settleDoctorCursor()
		return m, nil
	case "enter", " ":
		pending := m.Doctor.Pending
		if pending == nil || pending.Fix == nil {
			m.Screen = ScreenDoctor
			return m, nil
		}
		if m.Doctor.PendingPreview.Refusal != "" || m.Doctor.FixNotice != "" {
			// A refused fix has nothing to press, and a fix that already reported its
			// outcome is not applied twice by a second press.
			if m.Doctor.PendingPreview.Refusal != "" {
				return m, nil
			}
		}
		m.Doctor.FixNotice = "Applying " + pending.Fix.summary() + "…"
		return m, m.doctorFixCmd()
	}
	return m, nil
}

// ============================================================================
// The drawing
// ============================================================================

// renderDoctor draws the doctor's list.
func (m Model) renderDoctor() string {
	return m.renderThemeScreen(m.doctorDescription(), m.doctorNotice(), m.doctorHints())
}

// renderDoctorFix draws the confirmation for one fix.
func (m Model) renderDoctorFix() string {
	return m.renderThemeScreen(m.doctorFixDescription(), m.Doctor.FixNotice, []installerHint{hintSelect, hintBack})
}

// doctorHints is the legend, and it tells the truth per row: the fix key appears
// only when the row under the cursor offers one.
func (m Model) doctorHints() []installerHint {
	run := m.Doctor.Run
	if run != nil && run.running() {
		return []installerHint{hintMove, hintCancel}
	}
	hints := []installerHint{hintMove, hintSelect}
	if finding := m.doctorCursorFinding(); finding != nil && finding.Fix != nil {
		hints = append(hints, installerHint{"[Enter]", "see the fix"})
	}
	if run != nil {
		hints = append(hints, installerHint{"[t]", "problems/all"}, installerHint{"[w]", "write the report"})
		if m.doctorRetryableCount() > 0 {
			hints = append(hints, installerHint{"[r]", "retry unknown"})
		}
	}
	return append(hints, hintBack)
}

// doctorNotice is the screen's result line: the run's own progress or summary,
// and the export's outcome when one was asked for.
func (m Model) doctorNotice() string {
	if m.Doctor.Notice != "" {
		return m.Doctor.Notice
	}
	if run := m.Doctor.Run; run != nil {
		return run.Message
	}
	return ""
}

// doctorDescription is the body: what the screen does, where the answers come
// from, and -- once a run has happened -- the finding under the cursor in full.
//
// A finding is what the brief says a result has to be: the state, the evidence
// it was read from, what it means, and what to do. The list carries the state
// and the subject; this carries the rest.
func (m Model) doctorDescription() []string {
	run := m.Doctor.Run
	if run == nil {
		return []string{
			"Reads this machine and reports what it finds: the PATH a shell resolves, the shells' configuration, " +
				"the tools this installer manages, the dotfiles it owns, the basics of git and SSH, and the " +
				"environment faults that break a run.",
			"Nothing is changed by a scan. Each answer states its evidence, what it means and what to do; a check " +
				"that could not look says unknown instead of ok, and a configuration this installer does not own is " +
				"named rather than claimed.",
		}
	}

	paragraphs := []string{
		fmt.Sprintf("%s ran, %s finished. The list is %s.",
			doctorCounted(len(run.Order), "check", "checks"),
			doctorCounted(len(run.Results), "answer", "answers"),
			func() string {
				if m.Doctor.AllFindings {
					return "every finding"
				}
				return "the problems and the unknown answers only; press t for all of them"
			}()),
	}
	if run.running() {
		paragraphs = append(paragraphs, "Escape cancels: the checks that did not run are recorded as unknown with the reason.")
	}

	finding := m.doctorCursorFinding()
	if finding == nil {
		if len(paragraphs) < 2 {
			paragraphs = append(paragraphs, "Move the cursor onto a finding to read its evidence, its impact and what to do.")
		}
		return paragraphs
	}

	paragraphs = append(paragraphs,
		fmt.Sprintf("%s — %s", finding.Status.label(), finding.Subject),
		"Evidence: "+finding.Evidence,
		"Impact: "+finding.Impact,
		"What to do: "+finding.Advice,
	)
	if finding.Fix != nil {
		paragraphs = append(paragraphs, "The doctor can also do this for you: "+finding.Fix.summary()+". Press Enter to see exactly what would change.")
	}
	return paragraphs
}

// doctorFixDescription is the confirmation's body: the effect as a before/after,
// the risk, the permission and the backup -- read from the state before anything
// is changed -- or the refusal with its reason.
func (m Model) doctorFixDescription() []string {
	pending := m.Doctor.Pending
	if pending == nil || pending.Fix == nil {
		return []string{"There is no fix to confirm. Press Escape to go back to the findings."}
	}
	preview := m.Doctor.PendingPreview

	paragraphs := []string{
		fmt.Sprintf("%s offers one deterministic change, and this is exactly it. Nothing else on the machine is touched.", pending.Subject),
	}
	if preview.Refusal != "" {
		paragraphs = append(paragraphs, "This cannot be applied: "+preview.Refusal)
		paragraphs = append(paragraphs, "Use the manual steps on the findings list instead.")
		return paragraphs
	}
	for _, line := range preview.Effect {
		paragraphs = append(paragraphs, line)
	}
	paragraphs = append(paragraphs,
		"Risk: "+preview.Risk,
		"Permission needed: "+preview.Permission,
		"Backup: "+preview.Backup,
	)
	if m.Doctor.FixNotice == "" {
		paragraphs = append(paragraphs, "Press Enter to apply it, or Escape to leave without changing anything.")
	}
	return paragraphs
}

// doctorFixRows is the confirmation's list: the one action, and the way out.
// A refused fix has no action row at all, so the press that would have been
// refused is never offered.
func (m Model) doctorFixRows() []string {
	pending := m.Doctor.Pending
	if pending == nil || pending.Fix == nil || m.Doctor.PendingPreview.Refusal != "" || m.Doctor.FixNotice != "" {
		return []string{"← Back"}
	}
	return []string{pending.Fix.action(), "← Back"}
}

// doctorPanelValue is what the main menu's Utilities panel says the doctor's row
// holds. It reads the model's state and scans nothing: a panel is drawn on every
// keypress, and no check may run during a render.
func doctorPanelValue(state doctorState) string {
	run := state.Run
	switch {
	case run == nil:
		return "not run yet"
	case run.running():
		// The panel counts the answers, the same number the run's own notice
		// counts: with several checks out at once, "started" would claim work is
		// finished that is still running.
		return fmt.Sprintf("checking %d of %d", run.Answered, len(run.Plan))
	default:
		counts := run.report().counts()
		worst := counts.worst()
		if worst == doctorOK {
			return fmt.Sprintf("%d checks: everything ok", len(run.Order))
		}
		return fmt.Sprintf("%d checks: %s", len(run.Order), counts.summary())
	}
}

// doctorRow is the utilities section's row for this screen, and it is offered
// everywhere: there is always a machine to diagnose.
const utilitiesDoctorRow = "Diagnose the development environment"

// doctorPlatformLine names the machine this doctor reads: the model's own
// platform detection, so the section can say what it is about to look at
// without probing anything during a render.
func (m Model) doctorPlatformLine() string {
	if m.SystemInfo == nil {
		return ""
	}
	return newDoctorScan(nil, m.SystemInfo, time.Now()).platformLabel()
}
