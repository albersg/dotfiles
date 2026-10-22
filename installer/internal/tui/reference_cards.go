package tui

import "strings"

// ============================================================================
// THE REFERENCE PAGES — the Learn pages, the keymap tables and the LazyVim guide
// in the shared grammar
// ============================================================================
//
// The Learn hub and its tool pages, the keymap reference and the LazyVim guide
// were the last screens drawing material the reference grammar already had a word
// for: a page's name as a plain bold row, its description as a sentence under it,
// a binding as a line of padded columns and a list of reasons as bullets behind a
// chip. The four reference screens, the six wizard questions and every section
// screen since draw the same material through components.go -- a card whose title
// is a filled chip on its top border with its description as the muted caption at
// the right of the same row, a rule under the pair and the rows inside -- and
// these pages take that shape through the same functions.
//
// Nothing here decides anything the page did not already decide:
//
//   - the rows are the page's own. A menu's rows are m.GetCurrentOptions() in the
//     order the key handler reads them, with the menu's separators as the card's
//     dividers; a Learn table's rows are its referenceRows; a keymap table's rows
//     are the category's own bindings, in their three columns; a tool page's
//     lists are info.Pros and info.Cons. Nothing is reordered, renamed or added.
//   - the title is the page's own name and the caption its own description, both
//     already on the screen.
//   - the window arithmetic is the one the pages already had. The card costs the
//     three rows the old head cost -- its top border, the rule under the chip and
//     its bottom border replace the title, the description and the blank under
//     them -- so a page spends the rows it always did and its keys scroll it the
//     same distance. A caption the border row has no room for moves under the rule
//     (referenceCard, below) and is charged to the window first, so a narrow frame
//     redraws fewer rows instead of drawing one row past its budget.
//
// The palette is the applied theme's: every surface is drawn through a role
// styles.go built, and nothing here names a colour of its own.

const (
	// referenceCardMinContent is the narrowest card content a reference page's
	// rows are drawn in: the row's gutter, a name worth reading, the muted
	// metadata beside it and a sentence under both. A tool page below it keeps the
	// older body (renderSingleToolInfo), which spends its rows on the facts rather
	// than on a border that would leave them no room.
	referenceCardMinContent = 32

	// referenceReasonMark is the gutter every pros and cons entry starts with. It
	// is the mark the tool pages have always drawn in front of a reason, kept so
	// the two lists read as two lists and not as a paragraph.
	referenceReasonMark = "• "

	// referenceProsTitle and referenceConsTitle are the two statements a tool page
	// makes, with the glyphs that carry the good/bad meaning for a reader on a
	// terminal without the Nerd Font. They are the page's own words: the chips over
	// the same two lists said exactly this.
	referenceProsTitle = "✓ Pros"
	referenceConsTitle = "✗ Cons"
)

// referenceCardWidth is the width a reference page draws its card in and the
// columns the card's body may use. A page with a panel of its own takes the left
// column, exactly as a menu does, so the card is never drawn wider than the column
// the frame gives it and the panel is never truncated in its place; the pages
// without a panel -- the tables, the keymap categories and the LazyVim topic -- take
// the whole composition, centred by the frame like every other one-column page.
func (m Model) referenceCardWidth() (width, content int) {
	l := layoutFor(m)
	width = l.Composition
	if l.TwoColumn && len(m.panelsFor()) > 0 {
		width = l.Left
	}
	return width, cardContentWidth(width)
}

// referenceCardHead is the rows a card's caption spends when the border row has no
// room for it beside the chip. It is the caption's own paragraph, wrapped at the
// card's content width with the breaks the caption already carries split first --
// the same rule wrapSectionParagraphs applies to a screen's prose, because a note
// written on two lines is two rows and a caption measured as one would cost the
// window a row the frame did not have.
func referenceCardHead(title, caption string, width int) []string {
	// A caption rides the border beside the chip, so a card with no title has no
	// border row to carry it: the caption moves inside rather than being dropped
	// with the title it would have sat beside.
	if caption == "" || (title != "" && cardBorderCarriesCaption(title, caption, width)) {
		return nil
	}
	return mutedLines(wrapSectionParagraphs([]string{caption}, cardContentWidth(cardWidth(width))))
}

// referenceCard draws a page's card through the shared grammar (cardGroups,
// components.go): the title as the filled chip on the top border, the caption at
// the right of that same row when there is room, the rule under the pair and the
// groups inside. A caption the border row cannot carry moves under the rule
// instead of being given back, exactly as questionCard does for a wizard question.
//
// It takes the head from referenceCardHead rather than letting the card compose it,
// because these pages have to know what the head costs BEFORE they draw: a menu and
// a table window their rows, and the rows their keys may scroll to are the rows the
// card really spends. A card that measured its head by one rule and drew it by
// another would put a window one row past the frame on a note it misjudged.
func referenceCard(title, caption string, width int, head []string, groups []cardGroup) []string {
	if len(head) > 0 {
		return cardGroups(title, "", width, cardGroupsWithHead(head, groups))
	}
	return cardGroups(title, caption, width, groups)
}

// referenceTableCard draws one read-only page: the table's own name as the card's
// chip, its description as the caption, and the window of entries under the rule.
// The table's own two shapes are unchanged inside the card -- the two-line entry
// for the pages that are read, the three columns for the pages that are compared
// -- so wrapping a page in the card did not become a second table renderer.
func referenceTableCard(table referenceTable, start, end, width, content int) []string {
	groups := []cardGroup{{Lines: referenceTableRowsOf(table, start, end, content)}}
	return referenceCard(table.Name, table.Description, width,
		referenceCardHead(table.Name, table.Description, width), groups)
}

// referenceTableRowsOf is the rows one table page draws inside its card, in the
// shape the table asked for.
func referenceTableRowsOf(table referenceTable, start, end, content int) []string {
	if table.TwoLine {
		return referenceEntryRows(table, start, end, content)
	}
	return referenceTableRows(table, start, end, content)
}

// toolInfoCardBody draws one tool page in the shared grammar: the tool's name as
// the chip, its website as the caption, its description as the card's prose and
// the two lists under their own titled dividers. It reports whether the frame can
// hold the card; the caller keeps the older body when it cannot.
//
// The lists keep the arithmetic the page always used -- the rows left are split
// between them, and the rows the pros did not need go to the cons, so one short
// list and one long one still shows both whole -- and wrappedListRows draws each
// reason on as many rows as it needs, with the note that counts the ones the rows
// could not hold. Neither list is cut mid-sentence.
func (m Model) toolInfoCardBody(info ToolInfo, hints []installerHint) ([]string, bool) {
	width, content := m.referenceCardWidth()
	if content < referenceCardMinContent {
		return nil, false
	}
	budget := installerBodyRows(m.Height, m.footerReserveRows(contentWidth(m), hints))
	head := referenceCardHead(info.Name, info.Website, width)

	var prose []string
	if strings.TrimSpace(info.Description) != "" {
		for _, line := range wrapText(info.Description, content, 0) {
			prose = append(prose, InkStyle.Render(line))
		}
		prose = append(prose, "")
	}

	// The card's chrome, the caption that moved inside it, the prose above the
	// lists and the two dividers that name them are all spent before a reason is.
	lists := budget - cardChromeRows - len(head) - len(prose) - 2
	if lists < 2 {
		return nil, false
	}
	prosRows := lists / 2
	consRows := lists - prosRows
	pros := wrappedListRows(info.Pros, referenceReasonMark, prosRows, content, InfoStyle)
	cons := wrappedListRows(info.Cons, referenceReasonMark, consRows+prosRows-len(pros), content, MutedStyle)

	groups := []cardGroup{
		{Lines: prose},
		{Title: referenceProsTitle, Lines: pros},
		{Title: referenceConsTitle, Lines: cons},
	}
	body := referenceCard(info.Name, info.Website, width, head, groups)
	if len(body) > budget {
		return nil, false
	}
	return body, true
}

// referenceRowBudget is the rows a read-only page has for its entries, measured
// against the card the page is drawn in: the frame's body, less the card's three
// chrome rows, the caption when the border row could not carry it, and -- for the
// three-column shape -- the column header and the rule under it. The renderer and
// the page's scroll keys both ask for it, so the rows the keys may scroll to and
// the rows the page draws are one number.
func (m Model) referenceRowBudget(table referenceTable) int {
	width, _ := m.referenceCardWidth()
	rows := installerBodyRows(m.Height, m.footerReserveRows(contentWidth(m), referencePageHints()))
	rows -= cardChromeRows + len(referenceCardHead(table.Name, table.Description, width))
	if !table.TwoLine {
		rows -= 2
	}
	return max(keymapTableMinRows, rows)
}

// referencePageHints is the footer every read-only page offers: the two scroll
// keys, the way back and Enter, which returns to the page that opened it. It is
// the set renderReferenceTable draws, so the budget above reserves the footer the
// frame will really spend.
func referencePageHints() []installerHint {
	return []installerHint{hintUp, hintDown, hintOpen, hintBack}
}

// keymapCategoryTable is the read-only page one keymap category draws: its own
// bindings as the table's rows, in the page's three columns. The renderer and the
// scroll handlers both build it through this function, so the rows the screen
// draws and the rows the keys clamp are the same table.
func keymapCategoryTable(category KeymapCategory) referenceTable {
	cells := make([]referenceRow, 0, len(category.Keymaps))
	for _, km := range category.Keymaps {
		cells = append(cells, referenceRow{Name: km.Keys, Kind: km.Mode, What: km.Description})
	}
	return referenceTable{
		Name:        category.Name,
		Description: category.Description,
		Headers:     [3]string{"Keys", "Mode", "Description"},
		Rows:        cells,
	}
}

// keymapCategoryRows is how many binding rows the frame leaves for a category. The
// view and the scroll keys both ask for it, so the keys scroll exactly the rows the
// card draws.
func (m Model) keymapCategoryRows(category KeymapCategory) int {
	return m.referenceRowBudget(keymapCategoryTable(category))
}

// lazyVimTopicRows is the content rows the topic card shows: the frame's body,
// less the card's three chrome rows and the caption when it moved inside it. The
// view and the scroll keys both ask for it, so the window the keys scroll is the
// window the screen draws. It reserves the footer's worst case, footerMaxRows,
// because the scroll handler shares this function and has no width to compute the
// exact packing with.
func (m Model) lazyVimTopicRows(topic LazyVimTopic) int {
	width := layoutFor(m).Composition
	rows := installerBodyRows(m.Height, footerMaxRows) - cardChromeRows -
		len(referenceCardHead(topic.Title, topic.Description, width))
	if rows < 1 {
		rows = 1
	}
	return rows
}

// lazyVimTopicCard draws one topic in the shared grammar: the topic's own title as
// the chip, its description as the caption and the window of its lines under the
// rule. The lines arrive already styled and already cut to the card's content
// (lazyVimTopicLines), so the card adds the border and nothing else -- the example,
// the tips and the prose keep the marks they always had.
func lazyVimTopicCard(topic LazyVimTopic, lines []string, width int) []string {
	return referenceCard(topic.Title, topic.Description, width,
		referenceCardHead(topic.Title, topic.Description, width),
		[]cardGroup{{Lines: lines}})
}
