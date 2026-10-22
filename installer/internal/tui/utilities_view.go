package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ============================================================================
// THE UTILITIES SECTION
// ============================================================================

// Panel 5 of the mockup is this screen: a card of two-line rows beside the
// "System Overview" panel. Each row carries the job's icon in the gutter, its
// name on the first line and its one-line description under it, and the row
// under the cursor is filled across the card's width with the cursor's own
// marker in the icon's place so the name never moves. The panel states the
// machine the installer read -- its operating system, the environment it runs
// inside, its architecture, shell, package manager and home directory -- the
// resources it is spending, and the diagnosis' verdict.
//
// What is copied is the design, not the pixel widths. The card is measured
// against the column the body actually occupies -- the left column when the
// frame has room for the panel beside it, the whole composition when it does
// not -- and it does not exist below the width where a row still fits inside it.
// Two shapes, one preference: the mockup's two-line row is drawn where the frame
// can hold every row twice, and the same rows are drawn on one line where it
// cannot, which is what keeps 80x24 showing the whole list rather than half of
// it. A list longer than the frame is windowed around the cursor and says how
// many rows it hid, so no job is unreachable and no row is ever cut mid-word.
//
// Nothing here is invented. Every fact the panel states is read from the model:
// system.Detect's own fields, the same packageManager() the run dispatches
// through, the samples the metrics ring holds and the last doctor run's
// findings. A fact the model does not hold gets no row -- the mockup's Kernel and
// Uptime rows are left out because nothing in this installer reads them, and a
// row reading "unknown" would be the panel guessing rather than reporting.
//
// Two things the mockup draws are deliberately not row for row. Its first row,
// "System Overview / Check environment status", exists there because the mockup's
// right column is the selected row's detail; here the panel is contextual -- it
// states the same machine whatever the cursor is on -- so a row whose whole job
// would be to show it has no job, and the overview is the panel's own title (and,
// at one column, the card's title). Its Resources block is one card with the
// mockup's heading on the top border rather than a rounded box per meter: the
// same shape at two rows instead of six, which is what keeps the block and the
// verdict on screen together.
const (
	// utilitiesRowGutter is the columns a row spends before its text: the row's
	// icon, or the cursor's marker, and the blank that separates it from the name.
	utilitiesRowGutter = 2

	// utilitiesCardPad is the blank column a row keeps inside the card's border, so
	// a row never touches it and the selected row's fill reaches both edges.
	utilitiesCardPad = panelPad

	// utilitiesNotesMax is the most rows the section's reasons may take above the
	// card. The reasons say why a job is missing -- no desktop theme switch, no
	// .wslconfig, a theme list that could not be read -- and they are prose, so
	// they are the first reservation to give way when the frame is short: the list
	// is the screen, and a job the reader cannot reach is worse than a reason cut
	// with the usual marker.
	utilitiesNotesMax = 3

	// utilitiesListFloor is the least rows the card keeps for rows. The notes and
	// the notice yield before it does.
	utilitiesListFloor = 3

	// utilitiesNoticeRows is the notice band: the blank that separates it from the
	// card and the result line itself. A theme switch's outcome is one line, and
	// anything longer is cut with the same marker every other band uses.
	utilitiesNoticeRows = 2

	// utilitiesMeterCells is how many cells a resource meter spends. The mockup's
	// three meters are short bars, and the value that follows one is what a reader
	// takes the number from.
	utilitiesMeterCells = 10

	// utilitiesOverviewPanelLabel is the panel's name, from the mockup.
	utilitiesOverviewPanelLabel = "System Overview"

	// utilitiesResourcesCardLabel is the Resources block's heading, from the
	// mockup, set on the card's top border the way every other card in the frame is
	// titled.
	utilitiesResourcesCardLabel = "Resources"

	// utilitiesOverviewShort is the panel's name in a tab row or a summary line,
	// where the long one would take the room the facts beside it need.
	utilitiesOverviewShort = "Overview"

	// utilitiesDiagnosisCheck is the check the verdict line opens with. It is a
	// Nerd Font glyph, one cell, and decoration: the words say it too.
	utilitiesDiagnosisCheck = "\uf00c"
)

// The rows' icons, in the mockup's order. They are Nerd Font code points, one
// cell each, and decoration: a terminal without the font draws the row's name and
// its description, which are the row. Where a job names a fact the dashboard
// already draws -- the shell, the packages -- the dashboard's own glyph is used,
// so a machine fact wears one glyph wherever it is drawn.
const (
	utilitiesDiagnosticsIcon = "\uf0f1"           // nf-fa-stethoscope: the machine's check-up
	utilitiesWSLIcon         = "\uf17a"           // nf-fa-windows: the environment underneath
	utilitiesShellIcon       = dashboardShellIcon // nf-fa-terminal: the shell's prompt
	utilitiesToolsIcon       = dashboardPackagesIcon
	utilitiesSSHIcon         = "\uf084" // nf-fa-key
	utilitiesNetworkIcon     = "\uf1eb" // nf-fa-wifi: the mockup's connectivity mark
	utilitiesPortIcon        = "\uf1e6" // nf-fa-plug: what holds a port
	utilitiesCachesIcon      = "\uf1f8" // nf-fa-trash
	utilitiesThemeIcon       = "\uf1fc" // nf-fa-paint-brush
	utilitiesResourcesIcon   = "\uf233" // nf-fa-server: the resources the host offers
	utilitiesTerminalIcon    = "\uf2db" // nf-fa-microchip: the terminal's own abilities
	utilitiesVerifyIcon      = "\uf00c" // nf-fa-check: the record compared with the machine
	utilitiesDarkIcon        = "\uf186" // nf-fa-moon-o
	utilitiesLightIcon       = "\uf185" // nf-fa-sun-o
	utilitiesUndoIcon        = "\uf0e2" // nf-fa-undo
	utilitiesBackIcon        = "\uf177" // nf-fa-arrow-left
)

// utilitiesRow is one drawn row of the section: the option string the key
// handler matches, the words the row draws, its description and its icon. The
// label is never rewritten: it is the entry in GetCurrentOptions, so the cursor
// index, the handler's match and the drawn row are one list.
type utilitiesRow struct {
	// Label is the option string, verbatim: what GetCurrentOptions returned and
	// what handleUtilitiesKeys matches.
	Label string
	// Title is the words the row draws: the label with the section's arrow mark
	// taken off, because the aim of the arrow is now the icon's job.
	Title string
	// Subtitle is the mockup's one-line description of the job.
	Subtitle string
	// Icon is the row's Nerd Font glyph, empty on a row that draws none.
	Icon string
	// Divider marks the separator the option list uses to group its rows. It is
	// drawn as a rule inside the card and it is never a cursor position.
	Divider bool
}

// utilitiesRowParts is the table the section's rows are drawn from: one entry per
// option the section can offer, keyed by the same constant the handler matches, so
// a row cannot be drawn with another row's description or icon. The subtitles are
// the mockup's own where it has the job, and are written in the same voice where
// this installer has a job the mockup does not show.
var utilitiesRowParts = map[string]struct {
	icon     string
	subtitle string
}{
	utilitiesDoctorRow:      {utilitiesDiagnosticsIcon, "Check common issues"},
	utilitiesWSLAdvisorRow:  {utilitiesWSLIcon, "Check Linux issues"},
	utilitiesShellAuditRow:  {utilitiesShellIcon, "Measure startup time"},
	utilitiesUpdatesRow:     {utilitiesToolsIcon, "Check latest versions"},
	utilitiesSSHRow:         {utilitiesSSHIcon, "Manage SSH keys"},
	utilitiesNetworkRow:     {utilitiesNetworkIcon, "Check connectivity"},
	utilitiesPortRow:        {utilitiesPortIcon, "Find an open port"},
	utilitiesDiskCleanerRow: {utilitiesCachesIcon, "Remove temporary files"},
	utilitiesThemeRow:       {utilitiesThemeIcon, "Recolour the installer and the system"},
	utilitiesWSLRow:         {utilitiesResourcesIcon, "Set memory, processors and swap"},
	utilitiesTerminalRow:    {utilitiesTerminalIcon, "What this terminal can do"},
	utilitiesVerifyRow:      {utilitiesVerifyIcon, "Compare the machine with the record"},
	utilitiesDarkRow:        {utilitiesDarkIcon, "Use the desktop's dark variant"},
	utilitiesLightRow:       {utilitiesLightIcon, "Use the desktop's light variant"},
	utilitiesUndoRow:        {utilitiesUndoIcon, "Put back the setting the switch replaced"},
}

// utilitiesRowFor reads one option into the row the card draws. A row that opens
// a screen carries the arrow in the option string and loses it here, because the
// icon in the gutter says the same thing; a row that acts where it stands keeps
// the letter it carries, because that letter is the row's whole point. The way
// back keeps its arrow in the gutter, which is where the mockup draws it.
func utilitiesRowFor(label string) utilitiesRow {
	if strings.HasPrefix(label, menuSeparatorPrefix) {
		return utilitiesRow{Label: label, Divider: true}
	}
	title := strings.TrimPrefix(label, utilitiesScreenMark)
	if rest, ok := strings.CutPrefix(title, "← "); ok {
		return utilitiesRow{Label: label, Title: rest, Icon: utilitiesBackIcon}
	}
	// The table is keyed by the row's own name -- the option string without the
	// section's arrow -- so the mark a row opens with is not part of its identity.
	parts, ok := utilitiesRowParts[title]
	if !ok {
		// A row the table does not know is drawn as it is: the arrow is gone and
		// nothing is claimed about it. The guard beside this file fails on a row the
		// table is missing, so the fallback is a floor and not a habit.
		return utilitiesRow{Label: label, Title: title}
	}
	return utilitiesRow{Label: label, Title: title, Subtitle: parts.subtitle, Icon: parts.icon}
}

// utilitiesRows is every row the section draws, in the option list's own order
// and with the divider the list carries. It is derived from GetCurrentOptions on
// every render, so the cursor index a keypress reads and the row drawn under it
// are the same position in the same list.
func (m Model) utilitiesRows() []utilitiesRow {
	options := m.GetCurrentOptions()
	rows := make([]utilitiesRow, len(options))
	for i, option := range options {
		rows[i] = utilitiesRowFor(option)
	}
	return rows
}

// utilitiesShape is how the section is drawn at one size: whether the rows carry
// their description, whether they sit inside the mockup's box, the columns they
// are drawn at, the rows each one needs and the rows the notes and the notice
// reserve. It is one value because the decisions depend on each other: a boxed
// row is two columns narrower, a two-line row costs a second row, and the notes
// are what yields when the sum does not fit.
type utilitiesShape struct {
	TwoLine bool
	Boxed   bool
	// Measure is the columns the card occupies, borders included.
	Measure int
	// Content is the columns a row's text has inside the card.
	Content int
	// Heights is the drawn height of every row, in the option list's order.
	Heights []int
	Notes   int
	Notice  int
	// Summary is the panel said in one line. At one column it is drawn on the
	// card's top border, which is where the frame puts a collapsed panel's summary
	// and what the mockup's titled card looks like.
	Summary string
}

// utilitiesShapeFor decides the shape: the first candidate whose card, notes and
// notice fit the rows the frame leaves. The candidates run from the mockup's own
// (two-line rows in the box) to the floor's (one-line rows without it), so the
// richest shape the frame can hold is the one drawn.
func (m Model) utilitiesShapeFor(budget int, l layout) utilitiesShape {
	measure := l.Composition
	if l.TwoColumn && len(m.panelsFor()) > 0 {
		measure = l.Left
	}
	rows := m.utilitiesRows()

	notes := len(wrapParagraphs(m.utilitiesReasons(), measure))
	notice := utilitiesNoticeRows
	// The notes and the notice yield to the list rather than to each other: the
	// notice is a result the user asked for and stays, the notes are prose.
	if room := budget - notice - utilitiesListFloor; room < notes {
		notes = max(0, min(notes, room))
	}
	notes = min(notes, utilitiesNotesMax)

	summary := ""
	if !l.TwoColumn || len(m.panelsFor()) == 0 {
		summary = m.utilitiesOverviewCardTitle()
	}

	twoLineFits := utilitiesTwoLineFits(rows, measure)

	candidates := []struct{ twoLine, boxed bool }{
		{twoLine: true, boxed: true},
		{twoLine: false, boxed: true},
		{twoLine: false, boxed: false},
	}
	var fallback utilitiesShape
	for _, c := range candidates {
		if c.twoLine && !twoLineFits {
			continue
		}
		shape := m.utilitiesShape(c.twoLine, c.boxed, measure, rows)
		shape.Notes, shape.Notice, shape.Summary = notes, notice, summary
		fallback = shape
		if notes+shape.BoxedRows()+sum(shape.Heights)+notice <= budget {
			return shape
		}
	}
	// Nothing fits whole: the plain list is drawn and windowed around the cursor,
	// which is what keeps every row reachable on a terminal too short for the card.
	return fallback
}

// utilitiesShape builds one candidate: the columns its rows are drawn at and the
// rows each one needs. Selection is not passed in, because selection changes a
// row's marker and never its measure -- the window a list shows may not depend on
// a cursor.
func (m Model) utilitiesShape(twoLine, boxed bool, measure int, rows []utilitiesRow) utilitiesShape {
	shape := utilitiesShape{
		TwoLine: twoLine,
		Boxed:   boxed,
		Measure: measure,
		Content: measure,
	}
	if boxed {
		shape.Content = measure - panelFrame - 2*utilitiesCardPad
	}
	shape.Heights = make([]int, len(rows))
	for i, row := range rows {
		shape.Heights[i] = utilitiesRowHeight(row, twoLine)
	}
	return shape
}

// BoxedRows is the rows the card's own edges spend.
func (s utilitiesShape) BoxedRows() int {
	if !s.Boxed {
		return 0
	}
	return panelFrame
}

// utilitiesRowHeight is the rows a row costs: the divider is a rule and costs one,
// and a two-line row costs two where it has a description to put on the second.
// A row with no description -- the way back -- is one row in both shapes, so the
// shape never spends a blank line to keep a rhythm.
func utilitiesRowHeight(row utilitiesRow, twoLine bool) int {
	if row.Divider || !twoLine || row.Subtitle == "" {
		return 1
	}
	return 2
}

// utilitiesTwoLineFits reports whether the mockup's two-line row can be drawn at
// this width: every name and every description has to fit the card's content
// whole, because a description cut mid-word is worse than a list without one. A
// terminal too narrow for the longest one keeps the one-line row.
func utilitiesTwoLineFits(rows []utilitiesRow, measure int) bool {
	content := measure - panelFrame - 2*utilitiesCardPad - utilitiesRowGutter
	if content < 1 {
		return false
	}
	for _, row := range rows {
		if row.Divider {
			continue
		}
		if lipgloss.Width(row.Title) > content || lipgloss.Width(row.Subtitle) > content {
			return false
		}
	}
	return true
}

// utilitiesBodyLines composes the section: the notes above the card, the card of
// rows windowed around the cursor when the frame cannot hold all of them, and the
// result line. Every line is at most the shape's measure, so the frame's own
// centring has nothing left to cut. The window is returned beside the body so the
// header's vital can name the slice on screen.
func (m Model) utilitiesBodyLines(shape utilitiesShape, budget int) (body []string, start, end int) {
	body = clipLines(inkParagraphs(wrapParagraphs(m.utilitiesReasons(), shape.Measure)), shape.Notes, shape.Measure)

	lines := max(1, budget-len(body)-shape.Notice)
	rows := m.utilitiesRows()
	start, end = utilitiesRowWindow(shape.Heights, m.Cursor, lines, shape.Boxed)

	// The notes above and below say what the window hid, and they are inside the
	// budget the window was given: a list that degraded out loud is a list that
	// never quietly dropped a row.
	if start > 0 {
		body = append(body, MutedStyle.Render("↑ "+strconv.Itoa(start)+" more above"))
	}
	drawn := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		drawn = append(drawn, m.utilitiesRowLines(rows[i], i == m.Cursor, shape)...)
	}
	if shape.Boxed {
		drawn = utilitiesCardBlock(shape.Measure, shape.Summary, drawn)
	}
	body = append(body, drawn...)
	if end < len(rows) {
		body = append(body, MutedStyle.Render("↓ "+strconv.Itoa(len(rows)-end)+" more below"))
	}

	body = append(body, noticeBand(m.ThemeNotice, shape.Notice, shape.Measure)...)
	return body, start, end
}

// utilitiesRowLines draws one row: its name on the first line and its description
// under it in the two-line shape. Every line is exactly the card's content wide,
// so the box's borders and the filled selection bar line up whatever the row says.
func (m Model) utilitiesRowLines(row utilitiesRow, selected bool, shape utilitiesShape) []string {
	if row.Divider {
		return []string{rule(shape.Content)}
	}
	head := utilitiesRowTitleLine(row, selected, shape.Content)
	if utilitiesRowHeight(row, shape.TwoLine) == 1 {
		return []string{head}
	}
	indent := strings.Repeat(" ", utilitiesRowGutter)
	subtitle := padRight(truncate(row.Subtitle, shape.Content-utilitiesRowGutter), max(1, shape.Content-utilitiesRowGutter))
	return []string{head, indent + MutedStyle.Render(subtitle)}
}

// utilitiesRowWindow picks the rows the card shows: the window around the cursor
// that fits the lines it has, with the two notes that say what it hid counted in
// the budget. It grows from the cursor's row outward -- downward first, so the
// first row of the list keeps the top of the card -- and it never leaves the
// cursor's row out, because a window that hid the row the keys act on would be a
// window that lies.
//
// A row is not a line here: a two-line row spends two, which is why the window is
// measured in the lines the rows will draw rather than in rows. The card's own
// edges are counted once and there is no rule between the rows -- this card groups
// by description, not by line -- so the arithmetic is the setup table's minus the
// divider it draws between every pair of rows.
//
// A cursor outside the list is read as the first row rather than rejected: the
// window is a pure function of the heights it is given, and a caller's cursor can
// still point at a row a previous, longer list held. The clamp only decides this
// call's window; it never writes back to the model.
func utilitiesRowWindow(heights []int, cursor, lines int, boxed bool) (start, end int) {
	if len(heights) == 0 {
		return 0, 0
	}
	if cursor < 0 || cursor >= len(heights) {
		cursor = 0
	}
	edges := 0
	if boxed {
		edges = panelFrame
	}
	cost := func(start, end int) int {
		total := edges + sum(heights[start:end])
		if start > 0 {
			total++ // the note above
		}
		if end < len(heights) {
			total++ // the note below
		}
		return total
	}

	start, end = cursor, cursor+1
	for {
		switch {
		case end < len(heights) && cost(start, end+1) <= lines:
			end++
		case start > 0 && cost(start-1, end) <= lines:
			start--
		default:
			return start, end
		}
	}
}

// utilitiesRowTitleLine draws a row's first line: the gutter, then the name at the
// row's measure. The marker takes the icon's place on the row under the cursor, so
// the name never moves between the two states, and the bar covers that line only
// -- the description under it is read, not echoed.
func utilitiesRowTitleLine(row utilitiesRow, selected bool, width int) string {
	textWidth := max(1, width-utilitiesRowGutter)
	text := padRight(truncate(row.Title, textWidth), textWidth)
	if selected {
		return RowBarMarkerStyle.Render("▸ ") + RowBarStyle.Render(text)
	}
	if row.Icon == "" {
		return InkStyle.Render(strings.Repeat(" ", utilitiesRowGutter) + text)
	}
	return AccentKeyStyle.Render(row.Icon+" ") + InkStyle.Render(text)
}

// utilitiesCardBlock wraps the drawn rows in the mockup's card: the rounded box,
// one blank column inside each border, and the panel's summary on the top border
// where a one-column frame has no column left to draw it in. Every row handed in is
// already exactly content wide, so the box cannot resize a row and the selected
// row's bar reaches the padding on both sides.
func utilitiesCardBlock(width int, summary string, lines []string) []string {
	inner := max(1, width-panelFrame)
	edge := func(s string) string { return PanelBorderStyle.Render(s) }
	fill := func(s string) string {
		return strings.Repeat(" ", utilitiesCardPad) + padRight(s, inner-2*utilitiesCardPad) + strings.Repeat(" ", utilitiesCardPad)
	}

	top := edge("╭" + strings.Repeat("─", inner) + "╮")
	if summary != "" {
		label := " " + truncate(summary, max(1, inner-3)) + " "
		top = edge("╭─") + MutedStyle.Render(label) + edge(strings.Repeat("─", max(0, inner-1-lipgloss.Width(label)))+"╮")
	}

	out := make([]string, 0, len(lines)+2)
	out = append(out, top)
	for _, line := range lines {
		out = append(out, edge("│")+fill(line)+edge("│"))
	}
	out = append(out, edge("╰"+strings.Repeat("─", inner)+"╯"))
	return out
}

// ============================================================================
// The System Overview panel
// ============================================================================

// utilitiesOverviewPanel is the section's right column, from the mockup: the
// machine the installer read, the resources it is spending and the diagnosis'
// verdict. It is a panel like any other -- its own Title, Headline and Facts -- so
// the frame draws it in a column where there is room for one, and collapses it to
// the card's title line where there is not.
func utilitiesOverviewPanel() panel {
	return panel{
		ID:       panelUtilities,
		Title:    utilitiesOverviewPanelLabel,
		Short:    utilitiesOverviewShort,
		Headline: func(m Model) string { return m.utilitiesOverviewHeadline() },
		Facts:    func(m Model, l layout) []string { return m.utilitiesOverviewFacts(l) },
	}
}

// utilitiesOverviewHeadline is the panel in one line, for the narrow frame's card
// title: the machine's operating system, the environment it runs inside, its
// architecture, its shell and the package manager the run would install through.
// A fact the model does not hold is left out, exactly as the panel leaves it out.
func (m Model) utilitiesOverviewHeadline() string {
	parts := []string{}
	if head := m.machineHeadline(); head != "" {
		parts = append(parts, head)
	}
	if info := m.SystemInfo; info != nil && info.UserShell != "" {
		parts = append(parts, info.UserShell)
	}
	if manager := m.packageManager(); manager != "" {
		parts = append(parts, manager)
	}
	return strings.Join(parts, panelTabSeparator)
}

// utilitiesOverviewCardTitle is the panel's summary as the card's title on a
// one-column frame: the panel's name and the fact line under it. The frame draws a
// collapsed panel as one row of the same two parts, which is what keeps the
// summary recognizable whether it lands in a row or on a border.
func (m Model) utilitiesOverviewCardTitle() string {
	headline := m.utilitiesOverviewHeadline()
	if headline == "" {
		return utilitiesOverviewPanelLabel
	}
	return utilitiesOverviewPanelLabel + panelTabSeparator + headline
}

// utilitiesSelectedRow is the row the cursor is on, so a panel that answers
// about the section can name the job the keys would run rather than describing
// the same machine whatever the cursor is on. A cursor on a separator has no job
// under it and reports false.
func (m Model) utilitiesSelectedRow() (utilitiesRow, bool) {
	rows := m.utilitiesRows()
	if m.Cursor < 0 || m.Cursor >= len(rows) || rows[m.Cursor].Divider {
		return utilitiesRow{}, false
	}
	return rows[m.Cursor], true
}

// utilitiesOverviewFacts is the panel's body: the machine's own facts, then the
// resources block, then the diagnosis' verdict.
//
// The rows are the mockup's, minus the two this installer cannot state. Its Kernel
// and Uptime rows have no reader anywhere in this program -- detection reads the
// kernel's own string once to recognise WSL and keeps none of it, and nothing
// reads /proc/uptime -- so the panel draws neither rather than a guess or a zero.
// Everything else comes from system.Detect's fields, the package manager the
// dependency step dispatches through, the metrics ring and the doctor's run.
func (m Model) utilitiesOverviewFacts(l layout) []string {
	width := l.Right
	if width < 1 {
		width = l.Inner
	}

	var rows []string
	if info := m.SystemInfo; info != nil {
		if selected, ok := m.utilitiesSelectedRow(); ok {
			value := selected.Title
			if selected.Subtitle != "" {
				value += " — " + selected.Subtitle
			}
			rows = append(rows, panelFact("Selected", value, width)...)
		}
		rows = append(rows, panelFact("OS", info.OSName, width)...)
		rows = append(rows, panelFact("Host", m.platformHost(), width)...)
		rows = append(rows, panelFact("Arch", archName(info), width)...)
		rows = append(rows, panelFact("Shell", info.UserShell, width)...)
		rows = append(rows, panelFact("Packages", m.packageManager(), width)...)
		rows = append(rows, panelFact("Home", info.HomeDir, width)...)
	}

	// The Resources block is the mockup's: its heading on a card's top border and
	// the meters inside. Its rows are built against the card's inner width, so a
	// meter and its number never reach the border and never wrap out of the card.
	if resources := m.utilitiesResourceFacts(width - panelFrame - 2*panelPad); len(resources) > 0 {
		rows = append(rows, "")
		rows = append(rows, panelBlock(BrandStyle.Render(utilitiesResourcesCardLabel), width, resources)...)
	}

	if m.utilitiesDiagnosisClear() {
		rows = append(rows, "")
		rows = append(rows, SuccessStyle.Render(utilitiesDiagnosisCheck+" Everything looks good!"))
	}
	return rows
}

// utilitiesResourceFacts is the panel's Resources block: the CPU, memory and disk
// readings the model holds, each as a meter and the number behind it. The width it
// is given is the card's inner width, because these rows are drawn inside the
// mockup's Resources card. A row appears only where its own flag is set, so a host
// that does not report one gets the two it does rather than a zero dressed as a
// measurement -- and the block is absent altogether while the sampler is gated off,
// because the installer's rule is that a stale reading is never drawn as a current
// one.
func (m Model) utilitiesResourceFacts(width int) []string {
	if !m.Animating || len(m.Metrics) == 0 {
		return nil
	}
	last := m.Metrics[len(m.Metrics)-1]
	if !metricsHaveAny(last) {
		return nil
	}

	var rows []string
	if last.CPUOK {
		rows = append(rows, panelFact("CPU",
			fmt.Sprintf("%s %3.0f%%", meterBar(last.CPUBusy, utilitiesMeterCells), last.CPUBusy*100), width)...)
	}
	if last.MemOK && last.MemTotal > 0 {
		used := min(last.MemUsed, last.MemTotal)
		rows = append(rows, panelFact("Memory", fmt.Sprintf("%s %s / %s",
			meterBar(float64(used)/float64(last.MemTotal), utilitiesMeterCells),
			humanBytes(used), humanBytes(last.MemTotal)), width)...)
	}
	if last.DiskOK && last.DiskTotal > 0 {
		used := last.DiskTotal - min(last.DiskFree, last.DiskTotal)
		rows = append(rows, panelFact("Disk", fmt.Sprintf("%s %s / %s",
			meterBar(float64(used)/float64(last.DiskTotal), utilitiesMeterCells),
			humanBytes(used), humanBytes(last.DiskTotal)), width)...)
	}
	return rows
}

// utilitiesDiagnosisClear reports whether the diagnosis says the machine is fine:
// a run that finished, did not stop early, answered every check it planned, and
// whose every finding is clean. Anything else -- no run yet, a run still walking,
// a cancelled run, a check that never answered, one warning -- is not the panel's
// line to say, so the line is not drawn. The doctor's own screen states what it
// found; the panel only reports the all-clear.
func (m Model) utilitiesDiagnosisClear() bool {
	run := m.Doctor.Run
	if run == nil || run.running() || run.Canceled || len(run.Order) == 0 {
		return false
	}
	for _, id := range run.Order {
		result, answered := run.Results[id]
		if !answered {
			return false
		}
		for _, finding := range result.Findings {
			if doctorFindingNeedsAttention(finding) {
				return false
			}
		}
	}
	return true
}
