package tui

import (
	"fmt"
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
	"gai":         "Prefer IPv4; a libc-bin upgrade may reset it",
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
// The cursor may rest only on a row the key can act on: a heading, a plugin
// set's row and the units the run cannot do without are not switches, so the key
// does nothing there rather than offering a choice the run cannot honour. Both
// directions build a new slice rather than editing the one they were given --
// Update returns the model by value, so a slice shared with the previous copy
// must be replaced, not written through.
func (m *Model) toggleUnitAtCursor() bool {
	rows := m.planReviewRows()
	if m.UnitCursor < 0 || m.UnitCursor >= len(rows) {
		return false
	}
	row := rows[m.UnitCursor]
	if row.Kind != planRowStep || !row.Switchable {
		return false
	}
	return m.toggleUnit(row.StepID)
}

// toggleUnit turns one unit off or back on, and reports whether it changed
// anything. It refuses the units the run cannot do without, whatever the row
// that asked: the row and the key read the same catalogue flag.
func (m *Model) toggleUnit(id string) bool {
	if installUnitIsFixed(id) {
		return false
	}

	if m.unitIsDisabled(id) {
		kept := make([]string, 0, len(m.DisabledUnits))
		for _, disabled := range m.DisabledUnits {
			if disabled != id {
				kept = append(kept, disabled)
			}
		}
		m.DisabledUnits = kept
		return true
	}

	turnedOff := make([]string, 0, len(m.DisabledUnits)+1)
	turnedOff = append(turnedOff, m.DisabledUnits...)
	turnedOff = append(turnedOff, id)
	m.DisabledUnits = turnedOff
	return true
}

// moveUnitCursor moves the cursor to the next row it may rest on and wraps at
// both ends, so the list is a cycle the way the wizard's other lists are. It
// steps over the rows Space cannot switch -- an area heading, a plugin set's row
// and the units the run needs -- so every switchable unit is reachable and the
// cursor never rests on a row that cannot be switched.
func (m *Model) moveUnitCursor(delta int) {
	rows := m.planReviewRows()
	first := firstSwitchablePlanRow(rows)
	if first < 0 {
		return
	}
	if m.UnitCursor < 0 || m.UnitCursor >= len(rows) || !rows[m.UnitCursor].Switchable {
		m.UnitCursor = first
		return
	}

	next := m.UnitCursor
	for i := 0; i < len(rows); i++ {
		next += delta
		if next < 0 {
			next = len(rows) - 1
		}
		if next >= len(rows) {
			next = 0
		}
		if rows[next].Switchable {
			m.UnitCursor = next
			return
		}
	}
}

// snapUnitCursor puts the cursor on the first row a key can act on. It is what
// the screen does when it opens, so the cursor never starts on a heading or on
// one of the units the run needs.
func (m *Model) snapUnitCursor() {
	rows := m.planReviewRows()
	if first := firstSwitchablePlanRow(rows); first >= 0 {
		m.UnitCursor = first
	}
}

// firstSwitchablePlanRow is the index of the first row Space can switch, or -1
// when the plan holds none. It is a pure function of the rows so the cursor and
// its guard read the same answer.
func firstSwitchablePlanRow(rows []planReviewRow) int {
	for i, row := range rows {
		if row.Switchable {
			return i
		}
	}
	return -1
}

// installReviewCounts is how many steps of the plan the run will execute and how
// many the selection turned off. It counts the plan's steps, not the catalogue:
// the screen lists the plan, so the sentence under it describes the same list.
func (m Model) installReviewCounts() (kept, off int) {
	for _, step := range planFor(m.planOptions()) {
		if step.Status == StatusSkipped {
			off++
			continue
		}
		kept++
	}
	return kept, off
}

// planRowKind says what a row of the plan screen is: the name of an area, one of
// the plan's steps, or the plugin set a step provisions drawn directly under it.
// Only a step row whose Switchable is set is a cursor stop.
type planRowKind int

const (
	planRowHeading planRowKind = iota
	planRowStep
	planRowSet
)

// planReviewRow is one row of the plan screen. Text is what the screen draws;
// Kind and Switchable are what the keyboard and the guards read. A row carries
// no origin or area of its own: those come from the step the builder scheduled,
// so the screen cannot invent a second vocabulary for them (S5).
type planReviewRow struct {
	Kind       planRowKind
	Text       string
	StepID     string
	Switchable bool
}

// planReviewRows is the plan as the screen draws it: the run's own steps from
// planFor, in the order the run takes them, with each area named once before the
// first step it covers, and each step's plugin set indented directly under it.
//
// It is built from the plan rather than the catalogue, which is what S2 asks of
// the screen: a row exists because the run will act on it, or because the
// selection turned it off, not because the catalogue holds it. The origin and
// the area are the builder's answers on the step, so there is no second table
// here mapping step IDs to either.
func (m Model) planReviewRows() []planReviewRow {
	steps := planFor(m.planOptions())
	rows := make([]planReviewRow, 0, len(steps))
	area := ""
	for _, step := range steps {
		if step.Area != area {
			area = step.Area
			rows = append(rows, planReviewRow{Kind: planRowHeading, Text: area})
		}
		rows = append(rows, planReviewRow{
			Kind:       planRowStep,
			Text:       installReviewStepRow(step),
			StepID:     step.ID,
			Switchable: installUnitIsSwitchable(step.ID),
		})
		if step.PluginSet != nil {
			rows = append(rows, planReviewRow{Kind: planRowSet, Text: pluginSetRowText(step.PluginSet)})
		}
	}
	return rows
}

// fixedUnitClaim is what a row says about a unit the run cannot leave out. It
// is a claim about fixity, a separate fact from provenance: the catalogue's Fixed
// flag decides it (install_units.go), and only clone, setshell and cleanup carry
// it. A step's origin never uses this sentence, so the phrase cannot come to mean
// two things at once (S5 of odd/tasks/start-installation-flow.md).
const fixedUnitClaim = "the run needs it"

// installReviewStepRow is one step as the list draws it: what will happen to it,
// what it is, and where it came from. The label is the screen's own name for the
// unit. A unit the run cannot leave out says so, from the catalogue's Fixed flag
// rather than from its origin; every other row says where it came from -- the
// choice and the value it took, the machine, or the extra switch.
func installReviewStepRow(step InstallStep) string {
	marker := "[x]"
	if step.Status == StatusSkipped {
		marker = "[ ]"
	}
	row := marker + " " + installReviewLabel(step.ID)
	if installUnitIsFixed(step.ID) {
		return row + " — " + fixedUnitClaim
	}
	return row + " — " + stepOriginSentence(step)
}

// stepOriginSentence is where a step's row says it comes from, with the value a
// choice took where there was one. It is the builder's Origin read back, not a
// second answer kept beside the step.
func stepOriginSentence(step InstallStep) string {
	sentence := step.Origin.sentence()
	if sentence != "" && step.OriginValue != "" {
		sentence += ": " + step.OriginValue
	}
	return sentence
}

// pluginSetRowText draws the set a step provisions, indented under the step. A
// set a run can leave out reads as its switch -- [x] installed, [ ] left out --
// and a set a run cannot leave out says where the set comes from instead, so the
// row never offers a choice the run cannot honour.
// pluginSetProvenance is what a set's row says instead of a switch, for a set a run
// cannot leave out. It is short on purpose: the row also carries the set's name, and
// the longest of those leaves nineteen characters inside the narrowest terminal the
// frame matrix measures, so the full reason the registry holds would be cut off
// exactly where a user reads it. The reason is not lost -- it is what `--skip` refuses
// with and what the documentation explains -- and the registry's own guards keep it
// non-empty, which is what this row's guard checks beside the phrase.
const pluginSetProvenance = "the tool installs it"

func pluginSetRowText(set *PluginSetState) string {
	if set == nil {
		return ""
	}
	if set.Switchable {
		marker := "[x]"
		if set.Off {
			marker = "[ ]"
		}
		return "  " + marker + " " + set.Name
	}
	return "  " + set.Name + " — " + pluginSetProvenance
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

// listWindowMin is the floor for the list window. A terminal too short for
// even this many rows is one where the frame has already given up: drawing three rows
// and saying how many are hidden beats drawing none.
const listWindowMin = 3

// listWindowSize is how many list rows fit: what the body has left after the
// rows that are not the list, and never fewer than the floor.
//
// It is a pure function of three numbers, which is what lets its guard compare a
// short terminal against a tall one in one pass instead of rendering both.
func listWindowSize(budget, chrome, total int) int {
	if budget-chrome < listWindowMin {
		return listWindowMin
	}
	if budget-chrome > total {
		return total
	}
	return budget - chrome
}

// windowRowsFor is the window of rows to draw for a given cursor, as the half
// open range [start, end).
//
// It is a pure function of the list length, the cursor and the window size. The
// window is centred on the cursor and pushed back inside the list at both ends, so
// the cursor is always inside what is drawn.
func windowRowsFor(total, cursor, size int) (start, end int) {
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

// hintStartInstall is the review's own start key. The screen is a list of switches,
// and starting the run is a decision that changes everything that follows: Enter
// opens and chooses, so the decision takes a letter of its own, and the key that
// turns a unit off can never also start the run.
var hintStartInstall = installerHint{"[s]", "start"}

// installReviewCardMinContent is the narrowest card content the grammar's rows are
// drawn in: the gutter, a title worth reading and the line under it. Below it the
// screen keeps the compact list, which is the same rows one line each -- a card cut
// so short that it cannot show the sentence it was built to show is worse than the
// bare rows that do not cut it.
const installReviewCardMinContent = 32

// installReviewNoteRows is the rows the window's two notes hold: how many rows above
// the window and how many below it the screen hid. They are held whether or not the
// window draws them, so reaching the first or the last unit cannot change the card's
// height and move the frame around it.
const installReviewNoteRows = 2

// installReviewCardChrome is what a card spends before its body: the top border, the
// rule under the chip and the bottom border.
const installReviewCardChrome = 3

// installReviewWayBackRows is what the way-back block is given under the card: its
// chip and one wrapped line. The block is the screen's own promise -- what undoes
// this run -- so the card leaves it the rows it needs, and the block itself still
// says how many lines it could not show.
const installReviewWayBackRows = 2

// The area glyphs: a unit's row wears the mark of the area it belongs to, so the
// gutter says which part of the run the row is in and the words say which unit it
// is. The areas come from planFor, so the table is keyed by a fact the plan already
// carries rather than by eighteen meanings invented here.
const (
	installReviewPreparationIcon = "\uf0ad" // nf-fa-wrench
	installReviewDependencyIcon  = "\uf0c1" // nf-fa-link: what a unit needs first
	installReviewToolIcon        = "\uf0f3" // nf-fa-bell: a tool the run installs
	installReviewShellIcon       = dashboardShellIcon
	installReviewExtraIcon       = "\uf055" // nf-fa-plus-circle: an extra you turned on
	installReviewFinishIcon      = "\uf00c" // nf-fa-check: the last steps of the run
)

// installReviewAreaIcon names the area a row belongs to. An area this table does not
// name gets the tool mark rather than an empty gutter, so a new area cannot leave its
// rows indented a column differently from every other row in the card.
func installReviewAreaIcon(area string) string {
	switch area {
	case "Preparation":
		return installReviewPreparationIcon
	case "Dependencies":
		return installReviewDependencyIcon
	case "Shell":
		return installReviewShellIcon
	case "Extras":
		return installReviewExtraIcon
	case "Finishing up":
		return installReviewFinishIcon
	}
	return installReviewToolIcon
}

// installReviewRowLines draws one of the plan's rows in the grammar's shape. A step
// is the row the cursor can rest on: its own switch mark ("[x]" kept on, "[ ]"
// turned off) and its name on the title line, and where it came from -- or the one
// claim that it is a unit the run needs -- on the muted line under it. A plugin set
// keeps its own words and wears the set's gear, because that is what it is.
//
// Nothing is added and nothing is renamed: the two lines are the row's own text,
// split at the dash the row already carries, and a row with no dash keeps one line.
// The rows carry no chevron: a unit is switched where it stands, and this grammar
// spends the chevron on the rows that open something.
func installReviewRowLines(row planReviewRow, area string, width int, selected bool) []string {
	if row.Kind == planRowSet {
		return cardRowLines(cardRow{
			Icon:  setupPluginSetIcon,
			Title: strings.TrimSpace(row.Text),
		}, width, selected)
	}

	marker, rest := installReviewRowSplitMarker(row.Text)
	title, subtitle := installReviewRowSplitOrigin(rest)
	return cardRowLines(cardRow{
		Icon:     installReviewAreaIcon(area),
		Title:    strings.TrimSpace(marker + " " + title),
		Subtitle: subtitle,
	}, width, selected)
}

// installReviewRowSplitMarker splits a row into its switch mark and the words under
// it, so the mark stays at the head of the title where a reader looks for it.
func installReviewRowSplitMarker(text string) (marker, rest string) {
	for _, mark := range []string{"[x] ", "[ ] "} {
		if strings.HasPrefix(text, mark) {
			return strings.TrimSpace(mark), strings.TrimPrefix(text, mark)
		}
	}
	return "", text
}

// installReviewRowSplitOrigin splits a row's words at the dash that separates the
// unit from where it came from: the unit is the title and the provenance is the
// muted line under it.
func installReviewRowSplitOrigin(text string) (title, subtitle string) {
	if at := strings.Index(text, " — "); at > 0 {
		return text[:at], text[at+len(" — "):]
	}
	return text, ""
}

// installReviewRowName is the part of a row the card draws as its title: the unit's
// name, the area's name, or the plugin set's own words. It is what a reader scans the
// screen for, and what a guard compares when it asks whether a row is on the screen --
// the row's full text is split into two lines by the grammar, so the concatenation is
// no longer what a frame holds.
func installReviewRowName(row planReviewRow) string {
	switch row.Kind {
	case planRowHeading:
		return row.Text
	case planRowSet:
		return strings.TrimSpace(row.Text)
	}
	_, rest := installReviewRowSplitMarker(row.Text)
	title, _ := installReviewRowSplitOrigin(rest)
	return title
}

// installReviewRowHeight is how many lines installReviewRowLines draws for a row, so
// the window is measured before anything is drawn. A heading costs one row: the
// divider that names its area.
func installReviewRowHeight(row planReviewRow) int {
	switch row.Kind {
	case planRowHeading, planRowSet:
		return 1
	}
	_, rest := installReviewRowSplitMarker(row.Text)
	if _, subtitle := installReviewRowSplitOrigin(rest); subtitle != "" {
		return 2
	}
	return 1
}

// installReviewGroups turns the window of plan rows into the card's groups: an area
// heading opens a group named after it -- the card's own divider -- and the rows
// under it follow, with the plugin set still directly under its unit. The window can
// start inside an area: the first group then carries no name, because the name was
// the row above the window and the window's own note already says it hid rows.
func (m Model) installReviewGroups(rows []planReviewRow, start, end, content int) []cardGroup {
	area := ""
	for i := 0; i < start && i < len(rows); i++ {
		if rows[i].Kind == planRowHeading {
			area = rows[i].Text
		}
	}

	var groups []cardGroup
	for i := start; i < end && i < len(rows); i++ {
		row := rows[i]
		if row.Kind == planRowHeading {
			area = row.Text
			groups = append(groups, cardGroup{Title: area})
			continue
		}
		if len(groups) == 0 {
			groups = append(groups, cardGroup{})
		}
		at := len(groups) - 1
		groups[at].Lines = append(groups[at].Lines, installReviewRowLines(row, area, content, i == m.UnitCursor)...)
	}
	// A window can end between an area's name and the units under it, which would
	// draw a divider with nothing beneath it and read as an area of no work. The name
	// is dropped with the rows it no longer introduces, and the note under the card
	// says what the window hid.
	for len(groups) > 0 && len(groups[len(groups)-1].Lines) == 0 {
		groups = groups[:len(groups)-1]
	}
	// The card draws a group's name on the rule above it and draws nothing above the
	// first group, so a window that opens on an area would lose that area's name. An
	// empty group in front gives the name its rule, and it costs no row: a group with
	// no lines is its divider alone.
	if len(groups) > 0 && groups[0].Title != "" {
		groups = append([]cardGroup{{}}, groups...)
	}
	return groups
}

// installReviewCardBody draws the screen in the reference grammar: the screen's own
// title as the chip on the card's border, its own description as the caption beside
// it, one group per area with the area's name on the card's divider, the unit rows
// with their icons and their provenance, the row under the cursor filled across the
// card, and the selection's summary under a blank row. The way-back block keeps its
// rows under the card.
//
// It reports false when the frame cannot hold the card, and the screen then draws
// the compact body instead. The gate reads the width and the row budget only -- not
// the cursor and not the notice -- so the shape cannot change while the units are
// walked.
func (m Model) installReviewCardBody(width, budget int) ([]string, bool) {
	content := cardContentWidth(width)
	if content < installReviewCardMinContent {
		return nil, false
	}

	cardBudget := budget - installReviewWayBackRows
	if cardBudget < installReviewCardChrome+6 {
		return nil, false
	}

	rows := m.planReviewRows()
	heights := make([]int, len(rows))
	for i, row := range rows {
		heights[i] = installReviewRowHeight(row)
	}

	// The caption rides the card's border when the border has room for it, and moves
	// inside the card when it does not (questionCard, components.go). The rows it takes
	// there are the card's own, so they are measured here exactly as they are drawn.
	title, caption := stripStepPrefix(m.GetScreenTitle()), m.GetScreenDescription()
	head := 0
	if caption != "" && !cardBorderCarriesCaption(title, caption, width) {
		head = len(wrapText(caption, content, 0))
	}

	kept, off := m.installReviewCounts()
	summary := wrapText(m.installReviewCountLine(kept, off), content, 0)
	room := max(1, cardBudget-installReviewCardChrome-head)
	// The rows the card spends around the list: the blank above the count and the
	// count itself. The window's own notes are counted by the window arithmetic.
	fixed := len(summary) + 1
	entryRoom := room - fixed
	if entryRoom < 1 {
		return nil, false
	}

	start, end := sectionEntryWindow(heights, m.UnitCursor, entryRoom)
	windowed := start > 0 || end < len(rows)
	// Both notes are drawn when the list is windowed -- blank where the window has
	// nothing to report -- so the card keeps one height at every cursor position
	// instead of growing a row at each end of the list. The window gives back the row
	// a note it does not need would have added.
	for start < end && end-1 > m.UnitCursor && sum(heights[start:end])+fixed+installReviewNoteRows > room {
		end--
	}

	groups := m.installReviewGroups(rows, start, end, content)
	if len(groups) == 0 {
		return nil, false
	}

	var tail []string
	if windowed {
		above, below := "", ""
		if start > 0 {
			above = MutedStyle.Render(fmt.Sprintf("↑ %d more above", start))
		}
		if end < len(rows) {
			below = MutedStyle.Render(fmt.Sprintf("↓ %d more below", len(rows)-end))
		}
		tail = append(tail, above, below)
	}
	tail = append(tail, "")
	tail = append(tail, summary...)
	last := len(groups) - 1
	groups[last].Lines = append(groups[last].Lines, tail...)

	card := questionCard(title, caption, width, groups)
	// The way back keeps its reserved rows while the list is windowed -- the list is
	// what this screen is for, and the same sentence is on the failure screen -- and
	// spends the rows a list that fits did not need.
	wayBackRoom := budget - len(card)
	if windowed && wayBackRoom > installReviewWayBackRows {
		wayBackRoom = installReviewWayBackRows
	}
	card = append(card, wayBackRows(wayBackBeforeTheRun(m.Choices.CreateBackup), width, wayBackRoom)...)
	if len(card) > budget {
		return nil, false
	}
	return card, true
}

// renderInstallReview draws the screen: the run's own plan grouped by area, each row
// saying what will happen to it and where it came from, and how the selection
// stands. The rows come from planReviewRows, the same planFor the run will execute,
// so the list the user reviews and the list the run executes cannot disagree; the
// window and the cursor are the only things the renderer adds.
//
// The card is the reference's shape (installReviewCardBody); the compact body below
// is the frame that cannot hold it, where the same rows are drawn one line each. The
// window is sized from the terminal rather than from a number chosen here, and the
// rows that do not fit are reported rather than dropped: a list that silently hides
// rows is a list the user reviews by pressing Enter.
func (m Model) renderInstallReview() string {
	// The plan panel is composed beside the body on a wide frame, and the body is
	// then cut to the left column. Wrapping the description to the whole frame and
	// letting that cut finish the sentence is what put the "…" in the middle of
	// "Every unit this run can install": wrap to the column the text is drawn in.
	l := layoutFor(m)
	panels := m.panelsFor()
	hints := []installerHint{hintUp, hintDown, hintToggle, hintStartInstall, hintBack}
	budget := installerBodyRows(m.Height, m.footerReserveRows(l.Inner, m.panelHints(panels, hints)))

	// The card, drawn at the column it will be read in.
	cardWidth := l.Composition
	besidePanel := l.TwoColumn && len(panels) > 0
	if besidePanel {
		cardWidth = l.Left
	}
	body, ok := m.installReviewCardBody(cardWidth, budget)
	if !ok {
		body = m.installReviewCompactBody(cardWidth, budget)
	}
	if !besidePanel {
		body = centeredBody(body, l)
	}

	// The plan is the second of the wizard's two mandatory screens, so it carries the
	// meter's last step. Without it the header's vital sign disappears exactly where
	// the flow ends: the screen a user reads last would be the one screen that says
	// nothing about how far along they are.
	vital := ""
	if current, total := m.wizardProgress(); total > 0 {
		vital = meterRendered(current, total, 10)
	}
	return m.frame(m.headerName(), vital, body, hints)
}

// installReviewCompactBody is the frame too narrow for the card: the title, what the
// screen is for, the plan one line per row, the count and the way back. It is the
// shape this screen drew before the grammar reached it, kept because the rows are
// the screen and a card that cannot hold them is worse than the bare rows that can.
func (m Model) installReviewCompactBody(width, budget int) []string {
	description := wrapText(m.GetScreenDescription(), width, 0)
	kept, off := m.installReviewCounts()
	count := wrapText(m.installReviewCountLine(kept, off), width, 0)

	rows := m.planReviewRows()
	// The rows that are not the list: the title, the description, the blank under it,
	// the blank above the count, the count itself, and the two rows the "more above"
	// and "more below" lines may take.
	chrome := 1 + len(description) + 1 + 2 + 1 + len(count)
	size := listWindowSize(budget, chrome, len(rows))
	start, end := windowRowsFor(len(rows), m.UnitCursor, size)

	body := []string{BrandStyle.Render(m.GetScreenTitle())}
	body = append(body, description...)
	body = append(body, "")

	if start > 0 {
		body = append(body, MutedStyle.Render("↑ "+strconv.Itoa(start)+" more above"))
	}
	for i := start; i < end; i++ {
		// A row is one line, so an overlong one is cut with the visible marker rather
		// than allowed to widen the frame: the frame's contract is that overlong text
		// gives way in the open, not that it is clipped at the edge.
		text := truncate(rows[i].Text, width)
		switch {
		case i == m.UnitCursor:
			body = append(body, BrandStyle.Render(text))
		case rows[i].Kind == planRowHeading:
			// The area name is a heading, not a row the user acts on: it is muted so it
			// does not read as a switch, and it is never the cursor's target.
			body = append(body, SubtitleStyle.Render(text))
		default:
			body = append(body, text)
		}
	}
	if end < len(rows) {
		body = append(body, MutedStyle.Render("↓ "+strconv.Itoa(len(rows)-end)+" more below"))
	}

	body = append(body, "")
	body = append(body, count...)

	// The way back is named here, on the screen where a person can still decide not to
	// start, and it is placed in the rows the list did not need. A short terminal keeps
	// the units and the count and loses the sentence, which is the right way round: the
	// list is what this screen is for, and the same sentence is on the failure screen
	// and in the panel that offers the run.
	body = append(body, wayBackRows(wayBackBeforeTheRun(m.Choices.CreateBackup), width, budget-len(body))...)
	return body
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
	m.snapUnitCursor()
}

// beginInstallFromReview is the review screen's start key: it records the plan from the
// choices and the selection the user just reviewed, and starts the run. It is the
// only place a run starts, which is what keeps the screen from being decorative.
func (m Model) beginInstallFromReview() (tea.Model, tea.Cmd) {
	m.SetupInstallSteps()
	m.Screen = ScreenInstalling
	// The run opens with the log card beside the checklist, as the mockup draws it:
	// the step a person watches is the one writing those lines, so they are screen
	// rather than a box nobody opened. d still collapses the card where a reader
	// wants the checklist at the frame's full width.
	m.ShowDetails = true
	m.CurrentStep = 0
	m.Cursor = 0
	// A baseline belongs to the run that took it. A retry after a failure starts a
	// new run, and the state this one found is what its record has to compare
	// against; the first step takes it (captureRunBaseline). Without this the
	// record of the second run would name the first attempt's starting state,
	// while the backup it names holds the state this run actually found.
	m.ConfigFingerprints = nil
	m.ConfigFileIndex = nil
	return m, func() tea.Msg { return installStartMsg{} }
}

// handleInstallReviewKeys is the review screen's keyboard. Space is the switch and
// s is the decision, so the key that toggles a unit can never also start the run, and
// Enter is left to the screens that open something.
func (m Model) handleInstallReviewKeys(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		m.moveUnitCursor(-1)
	case "down", "j":
		m.moveUnitCursor(1)
	case " ":
		m.toggleUnitAtCursor()
	case "s":
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

// installUnitIsSwitchable reports whether Space can turn a planned step off. It
// is the catalogue's own answer -- a unit it names and does not mark Fixed -- so
// the row's cursor stop and the key that acts on it cannot disagree. A step the
// catalogue does not name is not switchable: the screen may draw it, but there is
// no flag to change and no command line name to change it by either.
func installUnitIsSwitchable(id string) bool {
	return installUnitIsCatalogued(id) && !installUnitIsFixed(id)
}

// installUnitIsCatalogued reports whether the catalogue names a unit at all, which
// installUnitIsFixed deliberately does not answer: an unknown ID is not a fixed one,
// and the two questions are asked separately when a command line is validated.
func installUnitIsCatalogued(id string) bool {
	for _, unit := range installUnits {
		if unit.ID == id {
			return true
		}
	}
	return false
}

// SwitchableUnitIDs names the units a run can leave out, in catalogue order: every
// unit except the three the run itself needs. It is what the command line offers and
// validates against, so a unit added to the catalogue is offered without anyone
// remembering to widen a written-out copy of the list.
func SwitchableUnitIDs() []string {
	ids := make([]string, 0, len(installUnits))
	for _, unit := range installUnits {
		if unit.Fixed {
			continue
		}
		ids = append(ids, unit.ID)
	}
	return ids
}

// ParseUnitSelection reads the comma-separated unit list a command line flag carries
// and returns the catalogue IDs it names, in the catalogue's order.
//
// An unknown name, or one of the three the run needs, is an error rather than an
// ignored token. A user who typed a unit the run will not leave out -- or a name
// nothing recognises -- has asked for something this run is not going to do, and
// installing it anyway while saying nothing is the worst of the three answers. The
// error names what was refused and what could have been asked for instead.
//
// The order comes from the catalogue rather than from the string, so the same
// selection reads the same way whichever route produced it.
func ParseUnitSelection(list string) ([]string, error) {
	wanted := map[string]bool{}
	for _, token := range strings.Split(list, ",") {
		name := strings.TrimSpace(token)
		if name == "" {
			continue
		}
		if installUnitIsFixed(name) {
			return nil, fmt.Errorf("%q is a unit the run needs and cannot leave out; this run can skip: %s",
				name, strings.Join(SwitchableUnitIDs(), ", "))
		}
		if !installUnitIsCatalogued(name) {
			return nil, fmt.Errorf("%q is not a unit this run installs; this run can skip: %s",
				name, strings.Join(SwitchableUnitIDs(), ", "))
		}
		wanted[name] = true
	}

	// Nothing named reads as no selection at all, rather than as an empty one: the set a
	// model starts with is nil, and an empty flag has to be indistinguishable from an
	// untouched selection instead of merely equal to it. The guard below caught exactly
	// this -- an empty slice and no slice plan the same steps and are not the same
	// value, and the parity guard compares values.
	if len(wanted) == 0 {
		return nil, nil
	}

	units := make([]string, 0, len(wanted))
	for _, unit := range installUnits {
		if wanted[unit.ID] {
			units = append(units, unit.ID)
		}
	}
	return units, nil
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

// wayBackRows renders the way back into the rows a screen did not need, and gives
// up before it takes one: a frame with no room shows no sentence rather than a row
// fewer of what the screen is for. The chip reports that the block continues
// rather than the sentence stopping mid-word.
func wayBackRows(lines []string, width, room int) []string {
	if room < 2 {
		return nil
	}

	block := []string{chip("Way back")}
	room--

	for _, line := range lines {
		for _, row := range wrapText(line, width-2, 2) {
			if room <= 0 {
				return block
			}
			block = append(block, gutteredBlock([]string{row})...)
			room--
		}
	}
	return block
}
