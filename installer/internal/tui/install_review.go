package tui

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// installReviewLabels names every unit of the catalogue for the screen that shows
// them, in the user's terms rather than the executor's.
//
// It is separate from the catalogue on purpose. install_units.go carries identity,
// order and what the run cannot do without; how a unit reads to a person is the
// screen's business, and the plan's own step names are not usable here: several of
// them are built from the choices ("Install " + the terminal the user picked) and
// the whole point of this list is to name units the plan may not schedule at all,
// because the user is about to turn them off.
//
// The guard in install_review_test.go holds every catalogued unit to a label here,
// so a unit added to the catalogue fails by name instead of appearing as its own
// ID on screen.
var installReviewLabels = map[string]string{
	"backup":      "Back up the configuration it would replace",
	"deps":        "Install the base dependencies",
	"xcode":       "Install the Apple developer tools",
	"clone":       "Clone the dotfiles repository",
	"homebrew":    "Install Homebrew",
	"terminal":    "Install the terminal emulator",
	"font":        "Install the Nerd Font",
	"shell":       "Install the shell and its plugins",
	"wm":          "Install the terminal multiplexer",
	"nvim":        "Install Neovim with your configuration",
	"toolset":     "Install the command-line toolset",
	"agentskills": "Install the Pi agent skills",
	"officecli":   "Install OfficeCLI",
	"wslconfig":   "Configure WSL",
	"theme":       "Restore the theme this machine has applied",
	"setshell":    "Set your shell as the default",
	"cleanup":     "Remove the temporary files the run created",
}

// installReviewLabel is what the screen calls a unit. A unit the catalogue gained
// and this file has not named yet reads as its own ID, which is ugly on purpose:
// the guard fails first, and until it is fixed the screen says which unit it
// cannot name rather than showing an empty row.
func installReviewLabel(id string) string {
	if label, ok := installReviewLabels[id]; ok {
		return label
	}
	return id
}

// unitIsDisabled reports whether the user turned a unit off. The empty slice --
// what every model starts with -- answers false for everything, which is what
// makes an untouched selection today's plan exactly (S4).
func (m Model) unitIsDisabled(id string) bool {
	for _, disabled := range m.DisabledUnits {
		if disabled == id {
			return true
		}
	}
	return false
}

// toggleUnitAtCursor turns the unit under the cursor off or back on, and reports
// whether it changed anything.
//
// The three units the run cannot do without are left alone: turning one off would
// offer a choice the run cannot honour, so the key does nothing there and the row
// says why. Both directions build a new slice rather than editing the one they
// were given -- Update returns the model by value, so a slice shared with the
// previous copy must be replaced, not written through.
func (m *Model) toggleUnitAtCursor() bool {
	if m.UnitCursor < 0 || m.UnitCursor >= len(installUnits) {
		return false
	}
	unit := installUnits[m.UnitCursor]
	if unit.Fixed {
		return false
	}

	if m.unitIsDisabled(unit.ID) {
		kept := make([]string, 0, len(m.DisabledUnits))
		for _, id := range m.DisabledUnits {
			if id != unit.ID {
				kept = append(kept, id)
			}
		}
		m.DisabledUnits = kept
		return true
	}

	turnedOff := make([]string, 0, len(m.DisabledUnits)+1)
	turnedOff = append(turnedOff, m.DisabledUnits...)
	turnedOff = append(turnedOff, unit.ID)
	m.DisabledUnits = turnedOff
	return true
}

// moveUnitCursor moves the cursor through the catalogue and wraps at both ends, so
// the list is a cycle the way the wizard's other lists are.
func (m *Model) moveUnitCursor(delta int) {
	if len(installUnits) == 0 {
		return
	}
	next := m.UnitCursor + delta
	if next < 0 {
		next += len(installUnits)
	}
	m.UnitCursor = next % len(installUnits)
}

// installReviewCounts is how many units the selection keeps and how many it turns
// off. The count is of the catalogue, not of the plan: the plan depends on the
// machine as well, and this screen is answering what the user asked for, not what
// this host will end up running.
func (m Model) installReviewCounts() (kept, off int) {
	for _, unit := range installUnits {
		if m.unitIsDisabled(unit.ID) {
			off++
			continue
		}
		kept++
	}
	return kept, off
}

// installReviewRow is one unit as the list draws it: what will happen to it, what
// it is, and -- for the three the run needs -- that the choice is not the user's.
func (m Model) installReviewRow(unit installUnit) string {
	marker := "[x]"
	if m.unitIsDisabled(unit.ID) {
		marker = "[ ]"
	}
	row := marker + " " + installReviewLabel(unit.ID)
	if unit.Fixed {
		row += " — the run needs it"
	}
	return row
}

// installReviewCountLine is the sentence under the list: how many units the
// selection keeps, and how many it turns off, in the words the report will use.
func (m Model) installReviewCountLine(kept, off int) string {
	switch off {
	case 0:
		return "Everything is on: the run installs all " + reviewPlural(kept, "unit", "units") + "."
	case 1:
		return "One unit is off; the other " + reviewPlural(kept, "unit", "units") + " stay on."
	default:
		return strconv.Itoa(off) + " units are off; the other " + reviewPlural(kept, "unit", "units") + " stay on."
	}
}

// reviewPlural renders a count with the right form of a word, so the counts read
// as sentences rather than as "1 units".
func reviewPlural(count int, one, many string) string {
	if count == 1 {
		return strconv.Itoa(count) + " " + one
	}
	return strconv.Itoa(count) + " " + many
}

// installReviewWindowMin is the floor for the list window. A terminal too short for
// even this many rows is one where the frame has already given up: drawing three rows
// and saying how many are hidden beats drawing none.
const installReviewWindowMin = 3

// installReviewWindowSize is how many list rows fit: what the body has left after the
// rows that are not the list, and never fewer than the floor.
//
// It is a pure function of three numbers, which is what lets its guard compare a
// short terminal against a tall one in one pass instead of rendering both.
func installReviewWindowSize(budget, chrome, total int) int {
	if budget-chrome < installReviewWindowMin {
		return installReviewWindowMin
	}
	if budget-chrome > total {
		return total
	}
	return budget - chrome
}

// reviewWindowRows is the window of rows to draw for a given cursor, as the half
// open range [start, end).
//
// It is a pure function of the list length, the cursor and the window size. The
// window is centred on the cursor and pushed back inside the list at both ends, so
// the cursor is always inside what is drawn.
func reviewWindowRows(total, cursor, size int) (start, end int) {
	if size <= 0 || total <= size {
		return 0, total
	}
	start = cursor - size/2
	if start < 0 {
		start = 0
	}
	if start+size > total {
		start = total - size
	}
	return start, start + size
}

// renderInstallReview draws the screen: the title, what the screen is for, a window
// of the catalogue with its switches, and how the selection stands. The rows come
// from GetCurrentOptions so the list the menu would draw and the list this draws
// cannot disagree; the window and the cursor are the only things added here.
//
// The window is sized from the terminal rather than from a number chosen here, and
// the rows that do not fit are reported rather than dropped: a list that silently
// hides rows is a list the user reviews by pressing Enter. Both of those were
// measured, not designed -- the frame guard reported this screen drawing 28 rows on a
// 24-row terminal, and nine rows fit at 80x24 and overflow the twenty-row terminal
// the matrix also measures.
func (m Model) renderInstallReview() string {
	width := contentWidth(m)

	hints := []installerHint{hintUp, hintDown, hintToggle, hintStart, hintBack}
	footer := footerHints(layoutFor(m).Inner, m.panelHints(m.panelsFor(), hints))
	budget := installerBodyRows(m.Height, len(footer))

	description := wrapText(m.GetScreenDescription(), width, 0)
	kept, off := m.installReviewCounts()
	count := wrapText(m.installReviewCountLine(kept, off), width, 0)

	rows := m.GetCurrentOptions()
	// The rows that are not the list: the title, the description, the blank under it,
	// the blank above the count, the count itself, and the two rows the "more above"
	// and "more below" lines may take.
	chrome := 1 + len(description) + 1 + 2 + 1 + len(count)
	size := installReviewWindowSize(budget, chrome, len(rows))
	start, end := reviewWindowRows(len(rows), m.UnitCursor, size)

	body := []string{BrandStyle.Render(m.GetScreenTitle())}
	body = append(body, description...)
	body = append(body, "")

	if start > 0 {
		body = append(body, MutedStyle.Render("↑ "+strconv.Itoa(start)+" more above"))
	}
	for i := start; i < end; i++ {
		if i == m.UnitCursor {
			body = append(body, BrandStyle.Render(rows[i]))
			continue
		}
		body = append(body, rows[i])
	}
	if end < len(rows) {
		body = append(body, MutedStyle.Render("↓ "+strconv.Itoa(len(rows)-end)+" more below"))
	}

	body = append(body, "")
	body = append(body, count...)

	return m.frame(m.headerName(), "", body, hints)
}

// openInstallReview puts the review screen in front of the run, with the screen it
// came from remembered so Escape goes back to it.
//
// Every path that used to start the run outright calls this instead: the wizard's
// last question, the backup question's two answers, and the main menu's own
// install row. One of them reaching ScreenInstalling without passing through here
// is the defect this function exists to make impossible to write by hand, because
// there is no longer a second place that builds the transition.
func (m *Model) openInstallReview() {
	m.PrevScreen = m.Screen
	m.Screen = ScreenInstallReview
	m.UnitCursor = 0
}

// beginInstallFromReview is the review screen's Enter: it records the plan from the
// choices and the selection the user just reviewed, and starts the run. It is the
// only place a run starts, which is what keeps the screen from being decorative.
func (m Model) beginInstallFromReview() (tea.Model, tea.Cmd) {
	m.SetupInstallSteps()
	m.Screen = ScreenInstalling
	m.CurrentStep = 0
	m.Cursor = 0
	return m, func() tea.Msg { return installStartMsg{} }
}

// handleInstallReviewKeys is the review screen's keyboard. Space is the switch and
// Enter is the decision, so a key that toggles can never also start the run.
func (m Model) handleInstallReviewKeys(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		m.moveUnitCursor(-1)
	case "down", "j":
		m.moveUnitCursor(1)
	case " ":
		m.toggleUnitAtCursor()
	case "enter":
		return m.beginInstallFromReview()
	case "esc":
		back := m.PrevScreen
		if back == ScreenInstallReview || back == ScreenInstalling {
			back = ScreenNvimSelect
		}
		m.Screen = back
		m.Cursor = 0
	}
	return m, nil
}

// reviewSelectedUnitIDs is the catalogue's IDs with the turned-off ones removed, in
// order. It is what the plan reads (T4) and what the count line summarises today.
func (m Model) reviewSelectedUnitIDs() []string {
	kept := make([]string, 0, len(installUnits))
	for _, unit := range installUnits {
		if !m.unitIsDisabled(unit.ID) {
			kept = append(kept, unit.ID)
		}
	}
	return kept
}

// disabledUnitSummary names the turned-off units in the catalogue's order, or an
// empty string when everything is on. The report uses it so a skipped unit is
// named rather than counted.
func (m Model) disabledUnitSummary() string {
	var names []string
	for _, unit := range installUnits {
		if m.unitIsDisabled(unit.ID) {
			names = append(names, installReviewLabel(unit.ID))
		}
	}
	return strings.Join(names, ", ")
}

// installUnitIsFixed reports whether the run needs a unit whatever the selection
// says. An ID the catalogue does not name answers false: the fixed set is the
// catalogue's, and a step that is not in the table is not one of the three.
func installUnitIsFixed(id string) bool {
	for _, unit := range installUnits {
		if unit.ID == id {
			return unit.Fixed
		}
	}
	return false
}

// unitIsDisabled reports whether the user turned a unit off, read over the plan's own
// copy of the set rather than the model's: the plan is a pure function of its options,
// and this keeps it one.
func (opts planOptions) unitIsDisabled(id string) bool {
	for _, disabled := range opts.DisabledUnits {
		if disabled == id {
			return true
		}
	}
	return false
}

// stepIsSkipped reports whether a planned step is one the user turned off. It is the
// single predicate both runners share, so "not executed" cannot come to mean two
// different things on the two routes.
func stepIsSkipped(step InstallStep) bool {
	return step.Status == StatusSkipped
}
