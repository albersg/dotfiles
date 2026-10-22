package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ============================================================================
// THE SECTION SCREENS — the reference's cards at the wide frame
// ============================================================================
//
// The doctor and every report one level in from the utilities section used to
// draw through the section renderer's bare grammar: a title, a band of prose, a
// list of one-line bars and a notice line, all open on the background. The four
// reference screens draw the same material in the shared grammar of
// components.go -- a card whose title is a filled chip set into its top border,
// a rule under it, the rows inside it, and a second column of cards beside it.
//
// This file is that composition, and it is one file because the section screens
// are one shape: the frame, the band arithmetic, the rows and the keys all
// already come from the section renderer, so what changes is only how the two
// bands are drawn once the frame is wide enough to hold two columns of cards.
//
// Nothing here decides anything new about the screen. It draws:
//
//   - the rows m.GetCurrentOptions returns, in the order the key handler reads
//     them, one card row each, with the row's own leading mark (a state glyph or
//     the way back's arrow) in the gutter and the rest of the label as the title.
//     Nothing is reordered, renamed or added, and the separator rows are still
//     rules;
//   - the tail the compact renderer pins (the actions and the way back) inside
//     the same card, under its own rule, measured against the tallest tail the
//     state can draw so an action that appears with a selection cannot move the
//     way back;
//   - the screen's own prose in the right column's card, which is the same
//     prose the compact screen draws above the list;
//   - the screen's own notice under that prose, in the same words;
//   - and, on a screen that is a report rather than a list, the prose in one
//     composition-wide card with the way back under the same rule.
//
// The 80x24 floor keeps the section renderer's own shape: two columns of cards
// cannot hold both a readable row and the prose there, and a card that has to cut
// a fact is worse than the bare rows that do not. The gate below is a width and a
// row budget, so the shape changes once, on a frame that can hold it, and never
// grows a second copy of its own arithmetic.

const (
	// sectionWideMinRows is the shortest body the two columns are drawn in. It is
	// the card's three chrome rows plus a window of rows on each side; below it the
	// section renderer's bands keep the rows for the list.
	sectionWideMinRows = 22

	// sectionWideMinRight is the narrowest the right column may be before its card
	// can no longer hold a fact row. A prose card narrower than this says less than
	// the band above the list already did.
	sectionWideMinRight = 48
)

// sectionWideScreen lists the section screens this composition draws: the
// doctor's two screens, the WSL reports and their two assistants, the capability
// and verification reports, the network diagnosis, the disk cleaner, the older
// update screen and the whole SSH manager.
//
// The screens that already compose their own wide view are deliberately not here
// -- the utilities section, the theme picker, the shell startup and the update
// center -- because each of them has a wide renderer of its own and a second one
// would be a second vocabulary for the same screen.
func (m Model) sectionWideScreen() bool {
	switch m.Screen {
	case ScreenDoctor, ScreenDoctorFix,
		ScreenWSLResources, ScreenWSLAdvisor, ScreenWSLBenchmark, ScreenWSLMigration,
		ScreenTerminalCapabilities, ScreenInstallVerify, ScreenNetworkDiagnostics,
		ScreenDiskCleaner, ScreenUpdate,
		ScreenSSHManager, ScreenSSHHost, ScreenSSHForm, ScreenSSHKeys,
		ScreenSSHAgent, ScreenSSHDiagnostics, ScreenSSHTest:
		return true
	}
	return false
}

// sectionWide reports whether the frame holds the reference's two columns for
// this screen. It is a width and a row budget and nothing else: no state, no
// cursor and no notice, so the shape cannot change while the user walks the
// list.
func (m Model) sectionWide(l layout, hints []installerHint) bool {
	if !m.sectionWideScreen() || !l.TwoColumn {
		return false
	}
	if len(m.panelsFor()) > 0 {
		return false
	}
	if l.Left < layoutLeftMin || l.Right < sectionWideMinRight {
		return false
	}
	return installerBodyRows(m.Height, m.footerReserveRows(l.Inner, hints)) >= sectionWideMinRows
}

// renderSectionWide draws one section screen in the reference's grammar: the
// list's card at the left, the prose's card at the right, and the frame still
// owning the header, the status rule, the footer and the creature's rows.
func (m Model) renderSectionWide(l layout, paragraphs []string, screenNotice string, hints []installerHint) string {
	budget := installerBodyRows(m.Height, m.footerReserveRows(l.Inner, hints))
	options := m.GetCurrentOptions()
	tail := pinnedTailLen(options)
	entries := options[:len(options)-tail]
	tailRows := options[len(options)-tail:]
	entryTotal := len(entries)

	var body []string
	start, end := 0, 0
	// A screen whose only rows are the way back has no list to walk: its prose IS
	// the screen, so it takes one composition-wide card and the way back rides
	// under that card's rule. The capability and verification reports are the two
	// screens this covers, and their prose is the report.
	listRows := 0
	for _, label := range entries {
		if !strings.HasPrefix(label, "← ") {
			listRows++
		}
	}
	if listRows == 0 {
		actions := append(append([]string{}, entries...), tailRows...)
		body = centeredBody(m.sectionWideReportCard(paragraphs, screenNotice, actions, m.Cursor, l.Composition, budget), l)
	} else {
		left, first, last := m.sectionWideListCard(entries, tailRows, l.Left, budget)
		start, end = first, last
		right := m.sectionWideDetailCard(paragraphs, screenNotice, l.Right, budget)
		body = composeColumns(left, right, l)
	}

	// The header's vital names the slice of a windowed list that is on screen, so a
	// list longer than the frame says so instead of appearing to end where the
	// screen does -- the same rule the compact shape follows.
	vital := ""
	if end-start < entryTotal {
		vital = scrollVital("Showing", start+1, end, entryTotal)
	}
	if suffix := m.sectionVitalSuffix(); suffix != "" {
		if vital == "" {
			vital = suffix
		} else {
			vital += " · " + suffix
		}
	}
	return m.frameWithPanels(m.headerName(), vital, body, hints, nil)
}

// sectionWideTitle is the chip on the left card. It is the screen's own title --
// the name the compact shape prints above its prose -- because the header already
// names the section ("Utilities") and the card names the page, exactly as the
// reference's "System Utilities" card sits under its "Utilities" header.
func (m Model) sectionWideTitle() string {
	return m.GetScreenTitle()
}

// sectionWideListCard draws the list's card: the rows the key handler matches,
// the window that keeps the cursor's row on screen, the notes that name what the
// window hid, and the pinned tail under its own rule.
//
// It returns the card and the window it drew, so the caller's header can name the
// same range the card shows.
func (m Model) sectionWideListCard(entries, tail []string, width, budget int) ([]string, int, int) {
	content := cardContentWidth(width)
	// The card spends its two borders and the rule under the chip before its body.
	room := max(1, budget-cardChromeRows)
	// The tail is measured against the tallest it can get in this state, not
	// against the tail on screen: an action that appears with a selection then
	// spends the rows already held for it instead of pushing the way back down.
	reserve := max(len(tail), m.pinnedTailReserve())
	entryRoom := max(1, room-reserve)

	heights := make([]int, len(entries))
	for i, label := range entries {
		heights[i] = sectionRowHeight(label, content)
	}
	start, end := sectionEntryWindow(heights, m.Cursor, entryRoom)
	// A windowed list spends one row at each end naming what it hid, so the window
	// is measured again with those rows gone. The notes only appear when the list
	// is actually windowed, which is a function of the list and not the cursor.
	if start > 0 || end < len(entries) {
		notes := 0
		if start > 0 {
			notes++
		}
		if end < len(entries) {
			notes++
		}
		start, end = sectionEntryWindow(heights, m.Cursor, max(1, entryRoom-notes))
	}

	var body []string
	if start > 0 {
		body = append(body, MutedStyle.Render(fmt.Sprintf("↑ %d more above", start)))
	}
	for i := start; i < end; i++ {
		if strings.HasPrefix(entries[i], menuSeparatorPrefix) {
			body = append(body, rule(content))
			continue
		}
		body = append(body, sectionRowLines(entries[i], content, i == m.Cursor)...)
	}
	if end < len(entries) {
		body = append(body, MutedStyle.Render(fmt.Sprintf("↓ %d more below", len(entries)-end)))
	}
	// The blank rows a short list leaves go above the actions, not under them: the
	// tail is drawn at the row its reservation holds, so a selection that adds an
	// action cannot move the way back. A card whose tail cannot grow is drawn
	// compact instead, with its actions right under its last row.
	if reserve > len(tail) {
		body = padRows(body, entryRoom)
	}

	for i, row := range tail {
		if strings.HasPrefix(row, menuSeparatorPrefix) {
			body = append(body, rule(content))
			continue
		}
		body = append(body, sectionRowLines(row, content, m.Cursor == len(entries)+i)...)
	}
	if len(body) > room {
		// A body past the card's rows keeps the rows that end the screen -- the way
		// back is the last of them -- the same way the compact shape does.
		body = body[len(body)-room:]
	}
	return card(m.sectionWideTitle(), "", width, body), start, end
}

// sectionWideDetailCard draws the right column's card: the screen's own prose,
// cut to the rows the frame leaves with the usual marker, and the screen's own
// notice under it.
//
// The card holds the rows the column was given, not the rows the current prose
// happens to need. A section screen's prose follows the cursor -- the disk
// cleaner puts the finding under it first -- so a card measured from the current
// row would change the composition's height as the cursor walks, and the centred
// body would climb and fall with it. Holding the column's rows is also what the
// reference draws: its cards run to the bottom of the body, not to the end of
// their text.
func (m Model) sectionWideDetailCard(paragraphs []string, screenNotice string, width, budget int) []string {
	content := cardContentWidth(width)

	var notice []string
	if strings.TrimSpace(screenNotice) != "" {
		notice = mutedLines(wrapText(screenNotice, content, 0))
	}
	fixed := cardChromeRows
	if len(notice) > 0 {
		fixed += grammarGap + len(notice)
	}
	prose := inkParagraphs(wrapSectionParagraphs(paragraphs, content))
	prose = clipLines(prose, max(1, budget-fixed), content)

	body := append([]string{}, prose...)
	if len(notice) > 0 {
		if len(body) > 0 {
			body = append(body, "")
		}
		body = append(body, notice...)
	}
	if len(body) == 0 {
		return nil
	}
	body = padRows(body, max(len(body), budget-cardChromeRows))
	return card(m.sectionPaneTitle(), "", width, body)
}

// sectionWideReportCard draws a report -- a screen with prose and no list to walk
// -- as one card across the composition, with the way back under its rule.
func (m Model) sectionWideReportCard(paragraphs []string, screenNotice string, tail []string, cursor, width, budget int) []string {
	content := cardContentWidth(width)

	var notice []string
	if strings.TrimSpace(screenNotice) != "" {
		notice = mutedLines(wrapText(screenNotice, content, 0))
	}
	fixed := cardChromeRows
	if len(tail) > 0 {
		fixed += 1 + len(tail) // the rule and the way back
	}
	if len(notice) > 0 {
		fixed += grammarGap + len(notice)
	}
	body := inkParagraphs(wrapSectionParagraphs(paragraphs, content))
	body = clipLines(body, max(1, budget-fixed), content)

	if len(tail) > 0 {
		body = append(body, rule(content))
		for i, row := range tail {
			// The cursor walks the rows the report does offer -- the way back -- so
			// the row it is on carries the marker here too.
			body = append(body, sectionRowLines(row, content, i == cursor)...)
		}
	}
	if len(notice) > 0 {
		if len(body) > 0 {
			body = append(body, "")
		}
		body = append(body, notice...)
	}
	return card(m.sectionWideTitle(), "", width, body)
}

// cardChromeRows is what a card spends before its body: its top border, the rule
// under the chip and its bottom border.
const cardChromeRows = 3

// sectionEntryWindow is the window of entries to draw, measured in the lines each
// entry really costs rather than in entries. It is the shared arithmetic the
// utilities card already uses, so a one-line entry and a label that continues on
// its second line are windowed by the same rule.
func sectionEntryWindow(heights []int, cursor, lines int) (start, end int) {
	return utilitiesRowWindow(heights, cursor, lines, false)
}

// sectionRowLines draws one of the screen's own rows in the grammar's shape: the
// row's leading mark in the gutter, the label as the title, and -- when the label
// is longer than the card's content -- the rest of it on the muted second line
// instead of being cut mid-word.
//
// The row is the option label the key handler matches, so nothing is renamed and
// nothing is added: a finding's state still reads as its own word, an action still
// carries its own key, and the way back still says where it goes.
func sectionRowLines(label string, width int, selected bool) []string {
	mark, title := sectionRowMark(label)
	// A label longer than the card's content continues on the muted second line
	// instead of being cut mid-word; the mark keeps the gutter on the first line.
	if lipgloss.Width(title) > max(1, width-grammarRowGutter) {
		if lines := wrapText(title, max(1, width-grammarRowGutter), 2); len(lines) == 2 {
			return cardRowLines(cardRow{Icon: mark, Title: lines[0], Subtitle: lines[1]}, width, selected)
		}
	}
	return cardRowLines(cardRow{Icon: mark, Title: title}, width, selected)
}

// sectionRowHeight is how many lines sectionRowLines will draw for a label, so
// the window can be measured before anything is drawn.
func sectionRowHeight(label string, width int) int {
	if strings.HasPrefix(label, menuSeparatorPrefix) {
		return 1
	}
	_, title := sectionRowMark(label)
	if lipgloss.Width(title) > max(1, width-grammarRowGutter) {
		if lines := wrapText(title, max(1, width-grammarRowGutter), 2); len(lines) == 2 {
			return 2
		}
	}
	return 1
}

// sectionRowMarks are the leading marks a section screen's own row can carry: the
// doctor's five states, the way back's arrow and the bullets the prose lists use.
// They are the row's own characters, moved into the gutter so the title starts at
// one column on every row -- a mark is decoration and the word beside it is the
// state, which is why a selected row can show the cursor's marker in its place
// without losing anything a reader needs.
var sectionRowMarks = map[rune]bool{
	'✓': true, '✗': true, '✘': true, '⊘': true, '?': true, '!': true,
	'←': true, '→': true, '•': true, '▸': true,
}

// sectionRowMark splits a row's leading mark from the words it labels. A label
// that does not start with a mark is returned whole, so a row that begins with a
// key ("[a] Run every check") keeps its key as the first thing read.
func sectionRowMark(label string) (mark, title string) {
	runes := []rune(label)
	if len(runes) < 3 || runes[1] != ' ' || !sectionRowMarks[runes[0]] {
		return "", label
	}
	return string(runes[0]), string(runes[2:])
}

// wrapSectionParagraphs wraps a screen's prose to a column, one paragraph at a
// time, and splits a paragraph that carries its own line breaks first: a break is
// a row the caller did not count, and a card that draws one more row than its
// budget is a card that pushes the footer off the frame.
func wrapSectionParagraphs(paragraphs []string, width int) []string {
	var lines []string
	for _, paragraph := range paragraphs {
		for _, physical := range strings.Split(paragraph, "\n") {
			lines = append(lines, wrapText(physical, width, 0)...)
		}
	}
	return lines
}
