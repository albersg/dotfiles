package tui

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
)

// ============================================================================
// THE SETUP TABLE
// ============================================================================

// Panel 3 of the mockup is this screen: the table's own title and subtitle over
// it, a rounded box with a rule between two rows, each row carrying an icon, its
// name at the left, the value the row holds at the right and the chevron that
// says Enter opens it, and the row under the cursor filled across the table's
// width. Under the table the mockup shows the pet and the selected row's
// description.
//
// What is copied is the design, not the pixel widths. The table is the body's
// whole width -- the setup screen offers no frame panel, because the mockup puts
// the description under the table rather than beside the body, and because the
// shell's and the editor's plugin-set rows need more columns than a two-column
// frame would leave -- so the rows are measured against the composition and not
// against a column. The values are the rows' own (setup_rows.go): the renderer
// prints what the choices hold and never decides anything, and the description
// under the table is the panel machinery's own panel, drawn by its own renderer,
// so the two cannot say different things.
//
// Two shapes, one preference. The mockup's boxed table is drawn where it costs
// the list no row -- at a tall terminal the box, the rule between every pair of
// rows and the panel under it all fit -- and the same rows are drawn without the
// box where it would hide one. At 80x24 the list is the floor's shape: every
// decision on one screen, the summary line of the panel under it, and the
// creature only where the frame has rows left. A row whose name and value cannot
// share a line at this width puts the value on the row's second line instead:
// at the floor the fish's plugin set cannot be cut down to one line without
// losing a word of its name or of its value, and the table says so rather than
// dropping either.
const (
	// setupTableTitle is the mockup's name for the table. The frame's header
	// already names the section (Setup) and the step (1/2), so the body's title
	// names the job instead of repeating either.
	setupTableTitle = "Configure your environment"

	// setupGutterWidth is the columns a row spends before its name: the row's icon,
	// or the cursor's marker, and the column that separates it from the name.
	setupGutterWidth = 2

	// setupChevronWidth is the chevron and the blank column before it, reserved on
	// every row that opens a screen so a table's values end on one edge whether or
	// not their row carries the mark.
	setupChevronWidth = 2

	// setupChevron is the mark on a row Enter opens: the mockup draws a chevron at
	// the end of its table's rows. It is the Nerd Font's angle-right rather than the
	// dingbat the mockup's own character is (U+276F): that one sits in the block the
	// emoji-box guard covers, because a terminal without an emoji font shows it as a
	// box, and this repository says a mark that does not draw everywhere is not a
	// mark. The glyph is one cell (the glyph guard holds every row icon to that) and
	// decoration -- the row's name and value are what the row says.
	setupChevron = "\uf105"

	// setupValueShare is the column a table's values start at, as a percentage of
	// the row's width. The mockup puts them a little past the middle of the table;
	// three fifths is that column, and it is a share rather than a fixed number so
	// the value column follows the terminal the way the rest of the frame does.
	setupValueShare = 60

	// setupLabelGap is the least blank run between a name and the value beside it,
	// so the two read as a name and a value instead of one run of text.
	setupLabelGap = 1

	// setupDetailWidth is the reading measure of the description band under the
	// table. The mockup gives the description half the panel; a paragraph is read
	// at 64 columns whatever the terminal's width, and the band is placed at the
	// right of the body where the mockup puts it once there are two band widths to
	// spare.
	setupDetailWidth = 64
)

// The row icons, in the order the mockup shows them. They are Nerd Font code
// points, one cell each, and decoration: a terminal without the font draws the
// row's name and value, which are the row. The shell's and the platform's icons
// are the dashboard's own, so a machine fact wears one glyph wherever it is drawn.
const (
	setupPlatformIcon    = dashboardOSIcon    // nf-fa-desktop: the machine the run targets
	setupShellIcon       = dashboardShellIcon // nf-fa-terminal: the shell's prompt
	setupTerminalIcon    = "\uf109"           // nf-fa-laptop: the terminal emulator
	setupFontIcon        = "\uf031"           // nf-fa-font
	setupMultiplexerIcon = "\uf00a"           // nf-fa-th: the panes a multiplexer splits a screen into
	setupEditorIcon      = "\uf121"           // nf-fa-code
	setupConfigsIcon     = "\uf115"           // nf-fa-folder-open-o
	setupPluginSetIcon   = "\uf013"           // nf-fa-gear: the set a tool's step provisions
	setupPlanIcon        = "\uf04b"           // nf-fa-play: the row that leaves for the plan
)

// setupRowIcon is the glyph a row draws in its gutter. A plugin set gets the
// gear the mockup draws beside Oh My Zsh, so the set reads as the tool's work
// rather than as one more decision of the flow.
func setupRowIcon(row setupRow) string {
	if row.Kind == setupRowPluginSet {
		return setupPluginSetIcon
	}
	switch row.Screen {
	case ScreenOSSelect:
		return setupPlatformIcon
	case ScreenShellSelect:
		return setupShellIcon
	case ScreenTerminalSelect:
		return setupTerminalIcon
	case ScreenFontSelect:
		return setupFontIcon
	case ScreenWMSelect:
		return setupMultiplexerIcon
	case ScreenNvimSelect:
		return setupEditorIcon
	case ScreenBackupConfirm:
		return setupConfigsIcon
	}
	return setupPlanIcon
}

// setupRowOpensScreen reports whether Enter on a row leads somewhere. It is what
// the chevron means, and it is read from the row rather than written per row: a
// question row opens its detailed screen, the action row opens the plan, and a
// plugin set -- which has no screen of its own -- is toggled where it stands, so
// it carries no chevron, exactly as the mockup draws its two set rows.
func setupRowOpensScreen(row setupRow) bool {
	return row.Screen != 0 || row.Kind == setupRowAction
}

// setupRowDescription is the prose the panel under the table shows for a row: what
// the option is for, in the words the row's own screen would use. It states no
// value -- the row beside it carries the value -- so the panel and the table
// cannot disagree about the state, and it is copy rather than a fact: the
// platform's two sentences are the mockup's own, and the rest name what each
// choice installs or configures.
func setupRowDescription(row setupRow) string {
	switch {
	case row.Kind == setupRowAction:
		return "Review every unit this run installs before it starts."
	case row.Kind == setupRowPluginSet:
		// The registry is the only place a set's own reason lives, so the panel
		// reads it from there instead of restating it. The sentence that says the
		// run cannot leave the set out comes first, so the one-line summary of this
		// row states the verdict even where the reason is too long to fit beside the
		// set's name.
		if set, ok := pluginSetByID(row.SetID); ok && set.Because != "" {
			return "The run cannot leave it out. " + sentenceCase(set.Because) + "."
		}
		return "The plugin set the tool above it installs."
	}
	switch row.Screen {
	case ScreenOSSelect:
		return "Select your operating system. This will adjust the installation steps and packages."
	case ScreenShellSelect:
		return "The shell this run installs and configures for you."
	case ScreenTerminalSelect:
		return "The terminal emulator this run installs and configures."
	case ScreenFontSelect:
		return "Install the Nerd Font the prompt, the terminal and the editor draw with."
	case ScreenWMSelect:
		return "The terminal multiplexer this run sets up for managing sessions."
	case ScreenNvimSelect:
		return "Includes LSP, TreeSitter, and dotfiles config."
	case ScreenBackupConfirm:
		return "Whether this run backs up the configuration it replaces."
	}
	return ""
}

// setupDetail is the form the description under the table takes: the panel
// machinery's own body (the chip that names the row, the rule under it and the
// description), its one-line summary where the body would cost the list a row,
// or nothing where the frame has room for neither.
type setupDetail int

const (
	setupDetailNone setupDetail = iota
	setupDetailSummary
	setupDetailBody
)

// setupShape is how the table is drawn at one size: whether the mockup's box is
// drawn, which form the band under it takes, the columns a row is drawn at and
// the rows each decision needs. It is one value because the decisions depend on
// each other -- a row drawn inside the box is two columns narrower and may need a
// second line, and the box itself costs two rows and one between every pair.
type setupShape struct {
	Boxed       bool
	Detail      setupDetail
	Measure     int
	Pad         int
	Heights     []int
	DetailWidth int
	DetailRows  int
}

// setupShapeFor decides the shape: the first candidate whose table and band fit
// the rows the frame leaves. The candidates run from the mockup's own (the boxed
// table with the panel under it) to the floor's (the plain list with the summary),
// so the richest shape the frame can hold is the one drawn, and a shape that
// would hide a row is never chosen over one that would not.
func (m Model) setupShapeFor(budget int, l layout) setupShape {
	measure := l.Composition
	detailWidth := min(measure, setupDetailWidth)

	// The rows above the table: the table's title and its subtitle, and the blank
	// between them and the box.
	chrome := 1 + len(wrapText(m.GetScreenDescription(), measure, 0)) + 1
	remaining := budget - chrome
	if remaining < 1 {
		remaining = 1
	}

	type candidate struct {
		boxed  bool
		detail setupDetail
	}
	candidates := []candidate{
		{boxed: true, detail: setupDetailBody},
		{boxed: true, detail: setupDetailSummary},
		{boxed: false, detail: setupDetailBody},
		{boxed: false, detail: setupDetailSummary},
		{boxed: false, detail: setupDetailNone},
	}

	var fallback setupShape
	for _, c := range candidates {
		shape := m.setupShape(c.boxed, c.detail, measure, detailWidth)
		if c.detail != setupDetailNone && !c.boxed {
			// A list needs a blank row between it and its description; the box's
			// bottom border is that separation where the box is drawn.
			shape.DetailRows++
		}
		fallback = shape
		if sum(shape.Heights)+shape.TableRows()+shape.DetailRows <= remaining {
			return shape
		}
	}
	// Nothing fits whole: the list is drawn and windowed, which is what keeps the
	// cursor's row reachable on a terminal too short for the screen.
	return fallback
}

// setupShape builds one candidate: the columns its rows are drawn at, the rows
// each decision needs and the rows the band under the table reserves. The band's
// reservation is the most any row's band needs, so it cannot change height as the
// cursor moves.
func (m Model) setupShape(boxed bool, detail setupDetail, measure, detailWidth int) setupShape {
	rows := m.setupRows()
	shape := setupShape{
		Boxed:       boxed,
		Detail:      detail,
		Measure:     measure,
		DetailWidth: detailWidth,
		DetailRows:  m.setupDetailRows(detail, rows, detailWidth),
	}
	if boxed {
		// The box's two borders are the row's frame, and its inner edge keeps one
		// blank column from the border so a row never touches it. Both are inside
		// the row, so the selected row's fill reaches the borders.
		shape.Measure -= panelFrame
		shape.Pad = 1
	}
	column := setupValueColumn(rows, m, shape.Measure, shape.Pad)
	shape.Heights = make([]int, len(rows))
	for i, row := range rows {
		// Selection changes the row's marker, not its measure, so the heights are
		// read without it: the window a table shows may not depend on the cursor.
		shape.Heights[i] = len(m.setupRowLines(row, false, shape.Measure, column, shape.Pad))
	}
	return shape
}

// TableRows is the lines the table's own edges spend: the two borders of the
// mockup's box and the rule between every pair of rows.
func (s setupShape) TableRows() int {
	if !s.Boxed {
		return 0
	}
	return panelFrame + max(0, len(s.Heights)-1)
}

// sum adds a slice of counts. It is the row-window arithmetic's own helper: the
// table's height is the sum of its rows' heights, never a count of rows.
func sum(counts []int) int {
	total := 0
	for _, n := range counts {
		total += n
	}
	return total
}

// setupValueColumn is the column a table's values start at, or 0 when the names
// and the values cannot share one line at this width. A shared column is what
// makes the rows read as a table -- the mockup's values all start at one
// column -- so it is used wherever it fits, and it is pushed right when the
// longest name needs the room; a width where the longest name and the widest
// value cannot both fit falls back to every value ending at the row's own right
// edge, which is the same statement said in less room.
func setupValueColumn(rows []setupRow, m Model, width, pad int) int {
	names, values := 0, 0
	for _, row := range rows {
		names = max(names, lipgloss.Width(row.Label))
		values = max(values, lipgloss.Width(m.setupRowValue(row)))
	}
	first := pad + setupGutterWidth + names + setupLabelGap
	last := width - pad - setupChevronWidth - values
	if first > last {
		return 0
	}
	return max(first, width*setupValueShare/100)
}

// setupRowLines draws one row: one line normally, and two when the row's name and
// value cannot share a line at this width. Neither is cut to make the table one
// row tall: the name is the row's subject and the value is the state it holds, so
// a row that does not fit puts its value under its name instead. Every line is
// exactly width columns, so the box's borders and the filled selection bar line
// up whatever the row says.
func (m Model) setupRowLines(row setupRow, selected bool, width, column, pad int) []string {
	value := m.setupRowValue(row)
	chevrons := setupRowChevrons(row)

	if value == "" || setupRowFitsOneLine(row.Label, value, width, column, chevrons, pad) {
		return []string{m.setupRowLine(row, value, selected, width, column, pad)}
	}

	// The value goes under the name, indented to the name's own column so the two
	// lines read as one row. The bar covers the name's line only: a second line on
	// the bar would be a two-row block of fill, and the value is state rather than
	// a subtitle.
	nameWidth := max(1, width-pad-setupGutterWidth-chevrons-pad)
	name := padRight(truncate(row.Label, nameWidth), nameWidth)
	chevron := ""
	if chevrons > 0 {
		chevron = strings.Repeat(" ", setupLabelGap) + setupChevron
	}
	tail := padRight(strings.Repeat(" ", pad+setupGutterWidth)+value, width)
	return []string{
		setupRowStyled(row, strings.Repeat(" ", pad), name, chevron, pad, selected),
		InkStyle.Render(tail),
	}
}

// setupRowChevrons is the columns a row reserves for the chevron: the glyph and
// the blank before it on the rows Enter opens, and nothing on the rows it does
// not, so a plugin set's value can end at the table's edge.
func setupRowChevrons(row setupRow) int {
	if !setupRowOpensScreen(row) {
		return 0
	}
	return setupChevronWidth
}

// setupRowFitsOneLine reports whether a row's name and value fit on one line: the
// value at the shared column when there is one, and at the row's own right edge
// when there is not. It is the one test the renderer and the window's arithmetic
// share, so a row cannot be measured as one line and drawn as two.
func setupRowFitsOneLine(name, value string, width, column, chevrons, pad int) bool {
	room := width - pad
	if column > 0 {
		return column+lipgloss.Width(value)+chevrons+pad <= room
	}
	return pad+setupGutterWidth+lipgloss.Width(name)+setupLabelGap+lipgloss.Width(value)+chevrons+pad <= room
}

// setupRowLine draws a row on one line: the gutter, the name, the value at the
// table's column (or at the row's right edge), and the chevron on the rows that
// open something. The blank between the name and the value is what makes the row
// read left to right, and on the selected row it is part of the bar.
func (m Model) setupRowLine(row setupRow, value string, selected bool, width, column, pad int) string {
	chevrons := setupRowChevrons(row)
	start := column
	if start == 0 {
		start = width - pad - chevrons - lipgloss.Width(value)
	}
	nameWidth := max(1, start-pad-setupGutterWidth)
	name := padRight(truncate(row.Label, nameWidth), nameWidth)
	middleWidth := max(1, width-pad-pad-setupGutterWidth-chevrons)
	trailing := max(0, middleWidth-nameWidth-lipgloss.Width(value))
	middle := name + value + strings.Repeat(" ", trailing)

	chevron := ""
	if chevrons > 0 {
		chevron = strings.Repeat(" ", setupLabelGap) + setupChevron
	}
	return setupRowStyled(row, strings.Repeat(" ", pad), middle, chevron, pad, selected)
}

// setupRowStyled puts the frame's tones on a drawn row: the icon in the key tone,
// the name and its value in body ink, the chevron in the key tone, and the row
// under the cursor on the brand fill with the marker where the icon was. The row
// is composed plain and styled in four pieces -- the box's own pad, the gutter,
// the row's text and the chevron -- so no escape sequence is ever cut by the
// width arithmetic, and the selected row's fill reaches both of the row's edges.
func setupRowStyled(row setupRow, lead, middle, chevron string, rightPad int, selected bool) string {
	tail := chevron + strings.Repeat(" ", rightPad)
	if selected {
		return RowBarMarkerStyle.Render(lead+"▸ ") + RowBarStyle.Render(middle) + RowBarMarkerStyle.Render(tail)
	}
	icon := setupRowIcon(row)
	gutter := lead
	if icon != "" {
		gutter += AccentKeyStyle.Render(icon) + " "
	} else {
		gutter += "  "
	}
	return gutter + InkStyle.Render(middle) + AccentKeyStyle.Render(tail)
}

// setupTableBlock wraps the drawn rows in the mockup's table: the rounded box,
// and a rule between two rows. Every row handed in is already exactly width-2
// columns -- the two borders -- so the box cannot resize a row, and the selected
// row's bar reaches both edges exactly as the mockup draws it.
func setupTableBlock(width int, lines []string) []string {
	inner := max(1, width-panelFrame)
	edge := func(s string) string { return PanelBorderStyle.Render(s) }

	out := make([]string, 0, len(lines)+2)
	out = append(out, edge("╭"+strings.Repeat("─", inner)+"╮"))
	for i, line := range lines {
		if i > 0 {
			out = append(out, edge("├"+strings.Repeat("─", inner)+"┤"))
		}
		out = append(out, edge("│")+line+edge("│"))
	}
	out = append(out, edge("╰"+strings.Repeat("─", inner)+"╯"))
	return out
}

// setupRowWindow picks the rows a table shows: the window around the cursor that
// fits the lines it has, with the two notes that say what it hid counted in the
// budget. It grows from the cursor's row outward, downward first so the first row
// of the list keeps the top of the table, and it never leaves the cursor's row
// out -- a window that hid the row the keys act on would be a window that lies.
//
// A row is not a line here: a row whose name and value did not fit on one line
// spends two, so the window is measured in the lines the rows will draw, which is
// why it cannot be the frame's listWindow.
func setupRowWindow(heights []int, cursor, lines int, boxed bool) (start, end int) {
	if len(heights) == 0 {
		return 0, 0
	}
	if cursor < 0 || cursor >= len(heights) {
		cursor = 0
	}
	cost := func(start, end int) int {
		total := sum(heights[start:end])
		if boxed {
			total += panelFrame + max(0, end-start-1)
		}
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

// setupDetailPanel is the setup screen's panel: the row under the cursor and what
// it is for. It is a panel like any other -- its own Title, Headline and Facts --
// but the frame does not draw it in a column: the mockup puts it under the table,
// so the body draws it with the panel machinery's own renderer. Its facts are the
// description alone; the value the row holds is already on the row, and a panel
// that repeated it would be a second copy of the state the table carries.
func (m Model) setupDetailPanel(width int) panel {
	row, _ := m.setupSelectedRow()
	return setupRowPanel(row, width)
}

// setupRowPanel is the panel for one row, so the band's reservation can measure
// every row's description instead of only the one under the cursor.
func setupRowPanel(row setupRow, width int) panel {
	description := setupRowDescription(row)
	return panel{
		ID:       panelSetupDetail,
		Title:    row.Label,
		Short:    row.Label,
		Headline: func(Model) string { return setupFirstSentence(description) },
		Facts: func(Model, layout) []string {
			// The band's measure is the width the caller gave it, not the frame's
			// column: the description is wrapped where it is drawn.
			lines := wrapText(description, width, 0)
			out := make([]string, len(lines))
			for i, line := range lines {
				out[i] = InkStyle.Render(line)
			}
			return out
		},
	}
}

// sentenceCase capitalises a registry reason so it can open a sentence. The
// reasons are written to follow "because", so the first letter is lowercase.
func sentenceCase(text string) string {
	r := []rune(text)
	if len(r) == 0 {
		return text
	}
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// setupFirstSentence is a description up to its first full stop. The summary line
// has two rows to spend and the panel's name takes part of the first, so it
// states the sentence the panel opens with rather than a sentence cut in half.
func setupFirstSentence(text string) string {
	if i := strings.Index(text, ". "); i >= 0 {
		return text[:i+1]
	}
	return text
}

// setupDetailRows is the rows a band reserves: the most any row's band needs at
// this width. The panel under the table may not change height as the cursor
// moves, so the reservation is the widest row's, not the selected row's.
func (m Model) setupDetailRows(detail setupDetail, rows []setupRow, width int) int {
	switch detail {
	case setupDetailBody:
		most := 0
		for _, row := range rows {
			most = max(most, len(wrapText(setupRowDescription(row), width, 0)))
		}
		return 2 + most // the chip that names the row and the rule under it
	case setupDetailSummary:
		most := 0
		for _, row := range rows {
			most = max(most, len(m.rotatorLines([]panel{setupRowPanel(row, width)}, width)))
		}
		return most
	}
	return 0
}

// setupDetailBand draws the band under the table at the reserved height: the
// panel body at wide widths, its summary line where the body would cost the list
// a row, and blanks where a row's own band came out shorter than the reservation,
// so nothing below the band can move when the cursor moves.
func (m Model) setupDetailBand(shape setupShape) []string {
	if shape.Detail == setupDetailNone || shape.DetailRows <= 0 {
		return nil
	}
	width := shape.DetailWidth
	indent := 0
	if shape.Measure > 2*width {
		// The mockup puts the description in the right half, with the pet's room at
		// the left; a body too narrow for two bands keeps it at the left, where it
		// lines up with the table's own edge.
		indent = shape.Measure - width
	}

	var band []string
	switch shape.Detail {
	case setupDetailBody:
		p := m.setupDetailPanel(width)
		// The panel's facts are wrapped at the band's own width when the panel is
		// built, so the frame's layout is not read here.
		band = panelBody(p.Title, p.Facts(m, layout{}), width, shape.DetailRows)
	case setupDetailSummary:
		p := m.setupDetailPanel(width)
		band = m.rotatorLines([]panel{p}, width)
	}

	out := make([]string, 0, shape.DetailRows)
	if !shape.Boxed {
		// The blank between a list and its description: the mockup's box already
		// separates the two where it is drawn, and its bottom border is the row a
		// blank would spend.
		out = append(out, "")
	}
	for _, line := range band {
		out = append(out, strings.Repeat(" ", indent)+line)
	}
	for len(out) < shape.DetailRows {
		out = append(out, "")
	}
	if len(out) > shape.DetailRows {
		out = out[:shape.DetailRows]
	}
	return out
}

// setupBodyLines composes the screen: the table's title and its subtitle, the
// table itself -- windowed around the cursor when the frame cannot hold every row
// -- and the band under it. Every line is at most the composition's width, so the
// frame's own centring has nothing left to cut.
func (m Model) setupBodyLines(shape setupShape, budget int, l layout) []string {
	body := []string{CenterHorizontally(BrandStyle.Render(setupTableTitle), l.Composition)}
	for _, line := range wrapText(m.GetScreenDescription(), l.Composition, 0) {
		body = append(body, CenterHorizontally(MutedStyle.Render(line), l.Composition))
	}
	body = append(body, "")

	rows := m.setupRows()
	column := setupValueColumn(rows, m, shape.Measure, shape.Pad)
	lines := max(1, budget-len(body)-shape.DetailRows)
	start, end := setupRowWindow(shape.Heights, m.SetupCursor, lines, shape.Boxed)

	if start > 0 {
		body = append(body, MutedStyle.Render("↑ "+strconv.Itoa(start)+" more above"))
	}
	drawn := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		drawn = append(drawn, m.setupRowLines(rows[i], i == m.SetupCursor, shape.Measure, column, shape.Pad)...)
	}
	if shape.Boxed {
		drawn = setupTableBlock(shape.Measure+panelFrame, drawn)
	}
	body = append(body, drawn...)
	if end < len(rows) {
		body = append(body, MutedStyle.Render("↓ "+strconv.Itoa(len(rows)-end)+" more below"))
	}

	body = append(body, m.setupDetailBand(shape)...)
	return body
}
