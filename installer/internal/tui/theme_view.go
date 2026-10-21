package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ============================================================================
// THE THEME SELECTOR
// ============================================================================

// Panel 6 of the mockup is this screen: a centred heading, its subtitle, the
// grid of theme cards with their miniature previews, the selected card marked,
// the tip line and the frame's "Preview mode" state.
//
// The palette is not a second one: every card's miniature is painted from the
// definition the switch would write -- its base, its selection, its text and the
// roles its palette strip names -- so the picture cannot show a colour the apply
// would not. The chrome around the cards stays the chrome the run wears, and the
// whole frame is still repainted in the theme under the cursor by View(), which
// is the live preview this screen has always had.
//
// What is copied is the design, not the pixel widths. Two cards to a row where
// the frame can hold every card and its miniature, one card to a row where it can
// hold them but not two abreast, and the same themes as one line each -- the
// marker, the name and the palette strip -- where the rows are too few for a
// picture. A list longer than any shape is windowed around the cursor and says
// how many rows it hid, so no theme is ever unreachable and no row is cut.
//
// Nothing here is invented. The cards are the complete definitions, in their own
// order; the names are the definitions' names; the tools a theme cannot paint are
// named on its card by the same coverage the option row is built from; and the
// actions (the refresh and the undo) keep the rows and the letters they had, so
// the flow and the policies are the ones that were already there.
const (
	// themePickerHeading and themePickerSubtitle are the mockup's, verbatim.
	themePickerHeading  = "Choose a theme for your installer"
	themePickerSubtitle = "This will change the colors and style of the interface."

	// themePickerTipLabel and themePickerTipText are the mockup's tip, which is
	// true of this screen: the list lives one level in from the utilities section,
	// so it is reachable again after the run.
	themePickerTipLabel = "Tip"
	themePickerTipText  = "You can change the theme later from the utilities menu."

	// themeTipGlyph is the mockup's lightbulb. It is a Nerd Font code point, one
	// cell, and decoration: the words say "Tip" without it.
	themeTipGlyph = "\uf0eb" // nf-fa-lightbulb-o

	// themeCardMarked and themeCardUnmarked are the mockup's radio marks. They are
	// geometric shapes rather than emoji, so a terminal with no emoji font draws
	// them, and the marked one is the only filled circle on the screen.
	themeCardMarked   = "●"
	themeCardUnmarked = "○"

	// themeRowMarker is the columns a row's marker spends before its text, and
	// themeRowGap the blank between the text and the palette strip.
	themeRowMarker = 2
	themeRowGap    = 2

	// themeGridGutter is the blank columns between two cards, and themeCardGap the
	// blank rows between two grid rows, so the cards read as separate objects
	// rather than as one block.
	themeGridGutter = 2
	themeCardGap    = 1

	// themeCardLabelRows is the row under a card that names it and carries the
	// mark.
	themeCardLabelRows = 1

	// themeGridMinMeasure is the narrowest body that can hold two cards and their
	// gutter. A card narrower than half of it cannot hold a theme's name beside
	// its mark, so the grid drops to one card to a row instead of cutting names.
	themeGridMinMeasure = 48

	// themeMiniMaxRows and themeMiniMinRows bound the miniature inside a card: the
	// mockup's picture is a small screen, and the tallest shape is tried first and
	// given up a row at a time, never below the three rows that hold its chip, its
	// selected row and the palette strip.
	themeMiniMaxRows = 4
	themeMiniMinRows = 3

	// themePickerTipRows is the blank and the tip line under the cards.
	themePickerTipRows = 2

	// themePickerNoticeRows is the blank and the result line at the foot of the
	// body. It is reserved on every state so a result cannot move the rows above
	// it; while a press is in flight the activity slot takes it, and the two never
	// coexist (a press clears the notice).
	themePickerNoticeRows = 2

	// themeSwatchCells is how many columns one palette role spends in a strip. Two
	// cells are enough to read a colour as a block and small enough that nine roles
	// fit beside a theme's name at the 80-column floor.
	themeSwatchCells = 2

	// themeMiniChipGlyph and themeMiniBarGlyph draw a miniature's chip and its
	// filled runs. Both are one cell and neither is an emoji, so the picture keeps
	// its shape on a terminal with no colour and no emoji font.
	themeMiniChipGlyph = "▄" // lower half block
	themeMiniBarGlyph  = "█" // full block

	// themePickerNameMin is the columns a list row keeps for the theme's name
	// before it gives the palette strip room, so the strip never pushes a name out
	// of the row.
	themePickerNameMin = 20
)

// themePickerCard is one theme's place in the grid: the definition the card is
// drawn from. The card's index is its option row's index, so the card the screen
// marks and the row the key handler applies are one entry of one list.
type themePickerCard struct {
	Def themeDefinition
}

// themePickerShape is how the selector is drawn at one size: the cards to a row,
// whether they carry their miniature, the columns one card and the body's own
// rows are drawn at, and the rows the tip, the result and the notice reserve. It
// is one value because the decisions depend on each other -- a carded grid costs
// a miniature and a label, and the tip is what yields when the sum does not fit.
type themePickerShape struct {
	Columns  int // cards to a row: 2, or 1
	Cards    bool
	Mini     int // rows of miniature inside a card, borders excluded
	Card     int // one card's columns
	Measure  int // the body's measure: a list row, the tip, the actions, the notice
	Actions  int // rows the action block reserves at the foot of the list
	Intro    int // rows the heading's prose was given
	ListRows int // rows the list or the grid was given
	Activity int // rows the result of a press reserves under the list
	Tip      bool
	Notice   int
}

// cardRows is the rows one card spends: its box, its miniature and its label.
func (s themePickerShape) cardRows() int {
	return panelFrame + s.Mini + themeCardLabelRows
}

// cardInner is the columns a miniature has inside its card.
func (s themePickerShape) cardInner() int {
	return max(1, s.Card-panelFrame-2*panelPad)
}

// themePickerCards is the grid's content: one card per complete definition, in
// the definitions' order, which is also the option list's order -- the theme rows
// are its prefix, so card i is option i and the cursor needs no second index.
func (m Model) themePickerCards() []themePickerCard {
	var cards []themePickerCard
	for _, option := range m.GetCurrentOptions() {
		if def, ok := m.dotfilesThemeForRow(option); ok {
			cards = append(cards, themePickerCard{Def: def})
		}
	}
	return cards
}

// themePickerSelectedCard is the card the cursor is on, or -1 when it is on an
// action row. The cursor's index is the card's index because the theme rows open
// the option list, which is the same assumption themePickerCards reads the rows
// with.
func (m Model) themePickerSelectedCard(cards []themePickerCard) int {
	if m.Cursor >= 0 && m.Cursor < len(cards) {
		return m.Cursor
	}
	return -1
}

// themeCardText is a card's label: the theme's name, and the tools it cannot
// paint in the same words the option row uses. The exclusions are drawn where the
// choice is made, which is the reason dotfilesThemeRow carries them at all.
func themeCardText(def themeDefinition) string {
	if note := themeExclusionNote(def); note != "" {
		return def.Name + " (" + note + ")"
	}
	return def.Name
}

// themePickerHints is the footer this screen offers in its list state. The
// refresh and the undo are named only while their rows are offered, so the footer
// never advertises a press with no row behind it.
func (m Model) themePickerHints() []installerHint {
	hints := []installerHint{hintUp, hintDown, hintChoose}
	if len(m.DotfilesThemes) > 0 {
		hints = append(hints, hintRefresh)
	}
	if m.DotfilesThemeRecord != nil {
		hints = append(hints, hintUndo)
	}
	return append(hints, hintBack)
}

// themePickerIntro is what the screen says under its heading: the mockup's
// subtitle where there are themes to choose, and the screen's own account of
// itself where there are none -- a read still running, or the reason this host
// cannot switch the dotfiles theme. A failure the user cannot read is the one
// thing this screen may not drop, so the reason takes the subtitle's place
// instead of being cut to fit under it.
func (m Model) themePickerIntro(cards []themePickerCard) []string {
	if len(cards) > 0 {
		return []string{themePickerSubtitle}
	}
	return m.themePickerDescription()
}

// themePickerIntroLines is what the screen says under its heading, wrapped to the
// measure the body is drawn at. The renderer and the shape arithmetic both read
// it here, so the rows the heading block reserves and the rows it draws are one
// number.
func (m Model) themePickerIntroLines(cards []themePickerCard, measure int) []string {
	return wrapText(strings.Join(m.themePickerIntro(cards), " "), measure, 0)
}

// renderThemePickerList draws the picker's list state: the mockup's heading and
// subtitle, the grid of cards (or the floor's list), the tip, the actions and the
// last result. The refresh review and its result are the screen's other two
// states and keep the section renderer, because they are prose and two rows.
func (m Model) renderThemePickerList() string {
	l := layoutFor(m)
	panels := m.panelsFor()
	hints := m.themePickerHints()
	frameHints := m.panelHints(panels, hints)
	budget := installerBodyRows(m.Height, m.footerReserveRows(l.Inner, frameHints))

	shape := m.themePickerShapeFor(budget, l)
	body, start, end, total := m.themePickerBodyLines(shape)
	// A two-column frame draws the palette board beside the body; a one-column
	// frame has no column for it, so the body is centred in the composition like
	// every other page.
	if !(l.TwoColumn && len(panels) > 0) {
		body = centeredBody(body, l)
	}

	// The header's vital names the slice of a windowed list that is on screen, so
	// a list longer than the frame says so instead of appearing to end where the
	// screen does.
	vital := ""
	if total > 0 && end-start < total {
		vital = scrollVital("Showing", start+1, end, total)
	}
	return m.frame(m.headerName(), vital, body, hints)
}

// themePickerShapeFor decides the shape: the first candidate whose heading, grid
// (or list), tip, notice and action block fit the rows the frame leaves. The
// candidates run from the mockup's own (two columns of carded miniatures with the
// tip under them) to the floor's (one line per theme, windowed), so the richest
// shape the frame can hold is the one drawn.
func (m Model) themePickerShapeFor(budget int, l layout) themePickerShape {
	// The body is the left column where the palette board has the right one, the
	// composition where it has not: the same measure every other section screen
	// uses.
	measure := l.Composition
	if l.TwoColumn && len(m.panelsFor()) > 0 {
		measure = l.Left
	}

	cards := m.themePickerCards()

	shape := themePickerShape{
		Columns: 1,
		Measure: measure,
		Card:    measure,
		Actions: m.pinnedTailReserve(),
		Notice:  themePickerNoticeRows,
	}

	fullIntro := len(m.themePickerIntroLines(cards, measure))
	shape.Intro = fullIntro
	// The heading, the intro and the blank under it stand above the list; the
	// action block and the notice stand below it. The prose is what yields when the
	// frame cannot hold all of it: it is cut with the frame's own marker rather than
	// pushing a row off the screen, and a frame that had to cut it has no rows to
	// spend on the tip.
	if room := budget - (1 + 1 + shape.Actions + shape.Notice + sectionMinEntryRows); shape.Intro > room {
		shape.Intro = max(1, room)
	}
	proseWhole := shape.Intro == fullIntro

	if len(cards) == 0 {
		// Nothing to choose: the reason the list is empty is the screen, so it takes
		// every row the heading, the actions and the notice leave, and there is no
		// grid to draw and no tip worth a row.
		shape.Cards = false
		shape.Tip = false
		shape.Intro = max(1, budget-(1+1+shape.Actions+shape.Notice))
		shape.ListRows = 0
		return shape
	}

	fixed := 1 + shape.Intro + 1
	room := max(1, budget-fixed-shape.Actions)

	if m.pickerActivity() != nil {
		// The result of a press owns the rows under the list: the tip and the notice
		// give way to it, and the list keeps every theme, because a press is not a
		// navigation. The slot is bounded above so it cannot become a second screen
		// and below so the pending line and its result are never cut to two rows.
		shape.Activity = min(themeActivitySlotFor(room, len(cards)), max(0, room-sectionMinEntryRows))
		shape.Notice = 0
		shape.ListRows = max(1, room-shape.Activity)
		return shape
	}

	// The mockup's grid, where the frame can hold every card: two columns first,
	// then one, each with the tallest miniature it can carry and the tip if there
	// is room for it. Every card is drawn or the shape is not taken -- a grid that
	// hid a theme behind a note would be a worse list than the floor's.
	for _, columns := range []int{2, 1} {
		card := measure
		if columns == 2 {
			if measure < themeGridMinMeasure {
				continue
			}
			card = (measure - themeGridGutter) / 2
		}
		for _, mini := range []int{themeMiniMaxRows, themeMiniMinRows} {
			grid := themeGridRows(columns, mini, len(cards))
			for _, tip := range []bool{true, false} {
				if tip && !proseWhole {
					continue
				}
				tipRows := 0
				if tip {
					tipRows = themePickerTipRows
				}
				if fixed+grid+tipRows+shape.Notice+shape.Actions <= budget {
					shape.Columns, shape.Cards, shape.Mini, shape.Card = columns, true, mini, card
					shape.Tip = tip
					shape.ListRows = grid
					return shape
				}
			}
		}
	}

	// The floor: the same themes as one line each, with the palette strip the card
	// carries, windowed around the cursor when the frame cannot hold them all. The
	// tip is prose, so it is drawn only where it does not cost the list a row.
	shape.ListRows = max(1, room-shape.Notice)
	if proseWhole && shape.ListRows-len(cards) >= themePickerTipRows {
		shape.Tip = true
		shape.ListRows -= themePickerTipRows
	}
	return shape
}

// themeGridRows is the rows a grid of the given shape spends: every row of cards,
// the blank rows between them and nothing else.
func themeGridRows(columns, mini, cards int) int {
	if cards < 1 {
		return 0
	}
	rows := (cards + columns - 1) / columns
	card := panelFrame + mini + themeCardLabelRows
	return rows*card + (rows-1)*themeCardGap
}

// themeActivitySlotFor is the rows the result of a press reserves under the list:
// every row the list did not need, within the mockup's slot bounds. It reads the
// rows left and the number of themes, never what the activity says, so the
// pending line and its result occupy the same rows.
func themeActivitySlotFor(room, cards int) int {
	spare := max(0, room-cards)
	if spare < themeActivitySlotFloor {
		spare = themeActivitySlotFloor
	}
	return min(spare, themeActivitySlotMax)
}

// themePickerBodyLines composes the screen: the heading, the intro, the grid of
// cards or the windowed list, the result of a press, the tip, the actions and the
// last notice. Every line is at most the shape's measure, so the frame's own
// composition has nothing left to cut.
func (m Model) themePickerBodyLines(shape themePickerShape) (body []string, start, end, total int) {
	cards := m.themePickerCards()
	selected := m.themePickerSelectedCard(cards)

	body = append(body, CenterHorizontally(BrandStyle.Render(themePickerHeading), shape.Measure))
	intro := clipLines(m.themePickerIntroLines(cards, shape.Measure), shape.Intro, shape.Measure)
	for _, line := range intro {
		body = append(body, CenterHorizontally(MutedStyle.Render(line), shape.Measure))
	}
	body = append(body, "")

	total = len(cards)
	if shape.Cards {
		body = append(body, themeGridLines(shape, cards, selected)...)
		start, end = 0, len(cards)
	} else {
		start, end = themeRowWindow(len(cards), selected, shape.ListRows)
		if start > 0 {
			body = append(body, MutedStyle.Render(fmt.Sprintf("↑ %d more above", start)))
		}
		for i := start; i < end; i++ {
			body = append(body, themePickerListRow(cards[i], shape.Measure, i == selected))
		}
		if end < len(cards) {
			body = append(body, MutedStyle.Render(fmt.Sprintf("↓ %d more below", len(cards)-end)))
		}
	}

	if shape.Activity > 0 {
		body = append(body, clipLines(activityLines(m.pickerActivity(), shape.Measure, shape.Activity), shape.Activity, shape.Measure)...)
	}
	if shape.Tip {
		body = append(body, "")
		body = append(body, themeTipLine(shape.Measure))
	}
	body = append(body, m.themePickerActionLines(shape)...)
	if shape.Notice > 0 {
		body = append(body, noticeBand(m.ThemeNotice, shape.Notice, shape.Measure)...)
	}
	return body, start, end, total
}

// themePickerActionLines draws the action block that ends the list: the rule that
// opens it, the rows that act where they stand, and the way back. They are the
// rows the handler matches, drawn with the row grammar every other list uses, and
// the block is padded to the height the shape reserved for it so a row that
// appears with a selection cannot move the way back.
func (m Model) themePickerActionLines(shape themePickerShape) []string {
	options := m.GetCurrentOptions()
	tail := min(pinnedTailLen(options), len(options))
	var lines []string
	for i := len(options) - tail; i < len(options); i++ {
		if strings.HasPrefix(options[i], menuSeparatorPrefix) {
			lines = append(lines, rule(shape.Measure))
			continue
		}
		lines = append(lines, m.rowBarWidth(options[i], i == m.Cursor, "", shape.Measure))
	}
	return padRows(lines, shape.Actions)
}

// themeRowWindow picks the rows the floor's list shows: the window around the
// cursor that fits the lines it has, with the notes that say what it hid counted
// in the budget. It grows from the cursor's row outward -- downward first, so the
// first row of the list keeps the top -- and it never leaves the cursor's row
// out, because a window that hid the row the keys act on would be a window that
// lies.
func themeRowWindow(count, cursor, lines int) (start, end int) {
	if count < 1 {
		return 0, 0
	}
	if cursor < 0 || cursor >= count {
		cursor = 0
	}
	cost := func(start, end int) int {
		total := end - start
		if start > 0 {
			total++ // the note above
		}
		if end < count {
			total++ // the note below
		}
		return total
	}

	start, end = cursor, cursor+1
	for {
		switch {
		case end < count && cost(start, end+1) <= lines:
			end++
		case start > 0 && cost(start-1, end) <= lines:
			start--
		default:
			return start, end
		}
	}
}

// themeGridLines lays the cards out in the mockup's grid: as many to a row as the
// frame holds, the gutter between them, one blank row under each grid row so the
// cards read as separate objects, and the last row padded when the count is odd.
func themeGridLines(shape themePickerShape, cards []themePickerCard, selected int) []string {
	var out []string
	for start := 0; start < len(cards); start += shape.Columns {
		blocks := make([][]string, 0, shape.Columns)
		for col := 0; col < shape.Columns; col++ {
			if idx := start + col; idx < len(cards) {
				blocks = append(blocks, themeCardLines(cards[idx], shape, idx == selected))
				continue
			}
			blocks = append(blocks, make([]string, shape.cardRows()))
		}
		for row := 0; row < shape.cardRows(); row++ {
			parts := make([]string, 0, shape.Columns)
			for _, block := range blocks {
				line := ""
				if row < len(block) {
					line = block[row]
				}
				parts = append(parts, padRight(line, shape.Card))
			}
			out = append(out, strings.Join(parts, strings.Repeat(" ", themeGridGutter)))
		}
		if start+shape.Columns < len(cards) {
			out = append(out, "")
		}
	}
	return out
}

// themeCardLines draws one card: the mockup's miniature inside a box, and the
// theme's name under it with the mark that says whether it is the card the cursor
// is on.
func themeCardLines(card themePickerCard, shape themePickerShape, selected bool) []string {
	box := panelBlock("", shape.Card, themeMiniatureLines(card.Def, shape.cardInner(), shape.Mini))
	return append(box, themeCardLabel(card.Def, shape.Card, selected))
}

// themeCardLabel is the row under a card: the mark and the theme's name, in the
// brand tone when the card is the selected one and in body ink when it is not. The
// mark is the mockup's filled circle, and it is what carries the selection on a
// terminal with no colour.
func themeCardLabel(def themeDefinition, width int, selected bool) string {
	textWidth := max(1, width-themeRowMarker)
	text := truncate(themeCardText(def), textWidth)
	mark := themeCardUnmarked
	if selected {
		return BrandStyle.Render(themeCardMarked) + strings.Repeat(" ", max(0, themeRowMarker-lipgloss.Width(themeCardMarked))) +
			BrandStyle.Render(padRight(text, textWidth))
	}
	return MutedStyle.Render(mark) + strings.Repeat(" ", max(0, themeRowMarker-lipgloss.Width(mark))) +
		InkStyle.Render(padRight(text, textWidth))
}

// themePickerListRow draws one theme as the floor's row: the mark that says
// whether it is the selected one, the theme's name and the tools it leaves out,
// and the palette strip the card's miniature carries at the wide frame. The
// selected row is one bar across the name -- the row grammar every list in the
// frame uses -- and the strip follows it in the themes' own colours, so the row
// says both what is selected and what it would paint.
func themePickerListRow(card themePickerCard, measure int, selected bool) string {
	stripRoom := max(0, measure-themeRowMarker-themePickerNameMin-themeRowGap)
	strip := themeSwatchStrip(card.Def, stripRoom)
	textWidth := max(1, measure-themeRowMarker-themeRowGap-lipgloss.Width(strip))
	text := truncate(themeCardText(card.Def), textWidth)

	mark := themeCardUnmarked
	markStyle := MutedStyle
	if selected {
		mark = themeCardMarked
		markStyle = RowBarMarkerStyle
	}
	row := markStyle.Render(mark) + strings.Repeat(" ", max(0, themeRowMarker-lipgloss.Width(mark)))
	if selected {
		row += RowBarStyle.Render(padRight(text, textWidth))
	} else {
		row += InkStyle.Render(padRight(text, textWidth))
	}
	return row + strings.Repeat(" ", themeRowGap) + strip
}

// themeTipLine is the mockup's tip: the lightbulb, its label and the sentence, on
// one row, in the frame's own grammar -- a tone for the mark and the quiet ink for
// the sentence. The sentence is cut to the measure before it is styled, so the
// cut lands on a whole word boundary of plain text rather than inside an escape.
func themeTipLine(measure int) string {
	label := themeTipGlyph + " " + themePickerTipLabel
	text := themePickerTipText
	if room := measure - lipgloss.Width(label) - 3; lipgloss.Width(text) > room {
		text = truncate(text, room)
	}
	return WarningStyle.Render(label) + MutedStyle.Render(" — "+text)
}

// themeMiniatureLines paints a card's miniature: a tiny stand-in for this very
// screen -- a title chip, the selected row, a line of body and the palette -- in
// the theme's own colours on the theme's own base. A definition with no base
// cannot paint one, so the card keeps its empty rows instead of guessing a
// colour.
func themeMiniatureLines(def themeDefinition, inner, rows int) []string {
	if inner < 4 || rows < 1 {
		return nil
	}
	base := def.Palette["base"]
	if !themeHexRE.MatchString(base) {
		return nil
	}

	chip := max(3, min(inner/5, 12))
	bar := max(4, inner-4)
	plan := []func(*themeMiniCanvas){
		func(c *themeMiniCanvas) { c.chip(def.Palette["blue"], chip) },
		func(c *themeMiniCanvas) { c.bar(def.Palette["selection"], bar) },
		func(c *themeMiniCanvas) {
			c.ink(def.Palette["text"], strings.Repeat("─", max(2, inner/2)))
		},
		func(c *themeMiniCanvas) {
			// The palette: two cells a role, a cell of the base between them, so the
			// strip reads as a row of swatches rather than as one more filled bar.
			for i, role := range themePreviewSwatchRoles {
				if c.used+themeSwatchCells > inner {
					break
				}
				if i > 0 {
					c.ink("", " ")
				}
				c.bar(def.Palette[role], themeSwatchCells)
			}
		},
	}
	if rows < len(plan) {
		// The palette is the miniature's last row in every shape, and the body line
		// is the one given up first.
		plan = append(plan[:2], plan[len(plan)-1])
	}

	out := make([]string, 0, rows)
	for i := 0; i < rows && i < len(plan); i++ {
		canvas := newThemeMiniCanvas(inner, base)
		plan[i](canvas)
		out = append(out, canvas.done())
	}
	return out
}

// themeMiniCanvas paints one row of a card's miniature. Every run is drawn in the
// theme's own role on the theme's own base, and the row is filled to the card's
// inner width with that base, so the picture is exactly as wide as the card and
// the colours are the definition's -- never a second palette kept for drawing.
type themeMiniCanvas struct {
	inner int
	base  string
	sb    strings.Builder
	used  int
}

func newThemeMiniCanvas(inner int, base string) *themeMiniCanvas {
	return &themeMiniCanvas{inner: inner, base: base}
}

// bar paints a filled run: the role as the fill and the theme's own base as the
// ink of the glyph. A miniature never paints a palette role as text ink, which is
// what keeps the screen's ink the chrome's: the live preview is the only thing
// that repaints the frame in the theme under the cursor, and a card may not read
// as if it had done so. The glyph is drawn rather than left blank, so the picture
// keeps its shape on a terminal with no colour and no emoji font.
func (c *themeMiniCanvas) bar(role string, cells int) {
	c.fill(themeMiniBarGlyph, role, cells)
}

// chip paints a short run of half blocks in a role's fill: the miniature's title
// mark, and the one run that reads as a shape rather than as a filled bar.
func (c *themeMiniCanvas) chip(role string, cells int) {
	c.fill(themeMiniChipGlyph, role, cells)
}

// fill paints cells of one glyph with role as their fill and the theme's base as
// their ink.
func (c *themeMiniCanvas) fill(glyph, role string, cells int) {
	cells = min(cells, c.inner-c.used)
	if cells < 1 || !themeHexRE.MatchString(role) {
		return
	}
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color(c.base)).
		Background(lipgloss.Color(role))
	c.sb.WriteString(style.Render(strings.Repeat(glyph, cells)))
	c.used += cells
}

// ink paints a run of glyphs in a role's foreground on the base. It is the one
// run a miniature draws as ink rather than as fill: a line of the theme's own body
// text, in the theme's own text colour, on the theme's own base -- which is what
// the theme would look like, and nothing the screen's own ink has to be confused
// with.
func (c *themeMiniCanvas) ink(role, glyphs string) {
	glyphs = truncateRunes(glyphs, c.inner-c.used)
	if glyphs == "" {
		return
	}
	style := lipgloss.NewStyle()
	if themeHexRE.MatchString(role) {
		style = style.Foreground(lipgloss.Color(role))
	}
	style = style.Background(lipgloss.Color(c.base))
	c.sb.WriteString(style.Render(glyphs))
	c.used += lipgloss.Width(glyphs)
}

// done fills the rest of the row with the theme's base and returns it.
func (c *themeMiniCanvas) done() string {
	if rest := c.inner - c.used; rest > 0 {
		c.sb.WriteString(lipgloss.NewStyle().Background(lipgloss.Color(c.base)).Render(strings.Repeat(" ", rest)))
	}
	return c.sb.String()
}

// themeSwatchStrip paints a theme's palette as a strip of two-cell swatches, one
// per role it holds, and drops whole swatches rather than splitting one when the
// columns run out. It is the floor's miniature and the card's last row: the
// definition's own colours, never a copy kept for drawing. A swatch is a filled
// cell -- the role as the fill, the theme's base as the ink -- rather than a blank
// background, so the strip keeps its shape on a terminal that cannot paint the
// palette at all, and it never paints a role as text ink.
func themeSwatchStrip(def themeDefinition, cells int) string {
	base := def.Palette["base"]
	var b strings.Builder
	used := 0
	for _, role := range themePreviewSwatchRoles {
		hex := def.Palette[role]
		if hex == "" {
			continue
		}
		if used+themeSwatchCells > cells {
			break
		}
		b.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color(base)).
			Background(lipgloss.Color(hex)).
			Render(strings.Repeat(themeMiniBarGlyph, themeSwatchCells)))
		used += themeSwatchCells
	}
	return b.String()
}
