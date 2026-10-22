package tui

import (
	"fmt"
	"strings"

	"github.com/albersg/dotfiles/installer/internal/portscan"
)

// ============================================================================
// THE PORTS — the reading and the stop in the reference's cards
// ============================================================================
//
// The port manager and its stop confirmation were the last two screens of the
// utilities section still drawing the older vocabulary: a branded title row, a
// band of muted prose, one-line bars and a notice under them, all open on the
// background. Every other screen in the section draws the reference's cards now
// (components.go), so these two draw the same objects and own none of them: the
// list is cardRowLines, the selected socket's facts are iconFactLines, the
// confirmation's answers are the section's own rows under the card's rule, and
// the chip, the caption and the borders are card's and cardGroups'.
//
// Nothing here reads the machine. The rows are the reading the model already
// holds (portVisibleListeners), the facts are the selected listener's own
// fields, and the prose is the screen's own sentences (portScopeSentence,
// portStateLine, portStopParagraphs, portEmptyListLine). No port, owner, state,
// limit or duration is invented, and the 80x24 floor keeps the compact renderer
// it had, because two cards cannot hold a readable row and the facts beside it
// there.
//
// The gate is the frame and nothing else -- a width and a row budget, never the
// cursor, the filter or the notice -- so the shape cannot change while the user
// walks the reading or types a filter.

const (
	// portCardMinRows is the shortest body the two cards are drawn in: the card's
	// three chrome rows, the two lines a row costs and the facts beside it.
	portCardMinRows = 20

	// portCardMinDetail is the narrowest the facts column may be before its rows
	// stop being readable. A column narrower than this says less than the compact
	// screen's detail block already did.
	portCardMinDetail = 44

	// portFactsChip is the name on the facts column. It is the section's own word
	// for a column that holds the detail of the row under the cursor
	// (sectionPaneTitle), so the ports do not name a second one.
	portFactsChip = "Details"

	// portSelectedGroup is the divider that names the block the socket's own facts
	// sit under. "Selected" is the word the compact detail block already prints
	// ("Selected: :3000 tcp LISTEN 127.0.0.1"), so the divider repeats the screen's
	// own label rather than inventing one.
	portSelectedGroup = "Selected"
)

// portManagerWide reports whether the frame holds the reference's two cards for
// the manager: the reading at the left and its facts at the right.
func (m Model) portManagerWide(l layout, budget int) bool {
	if !l.TwoColumn || l.Right < portCardMinDetail {
		return false
	}
	return budget >= portCardMinRows
}

// portStopWide reports whether the frame holds the confirmation's card. The
// confirmation is one column of prose with the answers under it, so it needs the
// composition and the rows to draw the account whole, not a second column.
func (m Model) portStopWide(l layout, budget int) bool {
	return l.TwoColumn && budget >= portCardMinRows
}

// renderPortManagerWide draws the manager in the reference's grammar: the
// windowed reading in the card at the left, the reading's own facts -- and, once
// the row under the cursor is open, that socket's facts -- in the card beside it.
// The header still names the slice of the reading the window shows.
func (m Model) renderPortManagerWide(l layout, hints []installerHint, budget int) string {
	left, start, end := m.portManagerListCard(l.Left, budget)
	right := m.portManagerFactsCard(l.Right, budget)
	body := composeColumns(left, right, l)

	vital := ""
	if total := len(m.portVisibleListeners()); total > 0 && end-start < total {
		vital = scrollVital("Showing", start+1, end, total)
	}
	return m.frame(m.headerName(), vital, body, hints)
}

// portManagerListCard is the reading's card: the sockets the filter left, two
// lines each, windowed around the cursor when the frame cannot hold them, with
// the notes that name what the window hid and the screen's own notice under the
// card's rule. It returns the card and the window it drew, so the header can name
// the same range.
func (m Model) portManagerListCard(width, budget int) ([]string, int, int) {
	content := cardContentWidth(width)
	room := max(1, budget-cardChromeRows)

	var notice []string
	if strings.TrimSpace(m.Ports.Notice) != "" {
		notice = mutedLines(wrapText(m.Ports.Notice, content, 0))
	}
	// The notice is its own group, so it spends the rule above it as well as its
	// own rows before the window may have any.
	reserved := 0
	if len(notice) > 0 {
		reserved = 1 + len(notice)
	}

	visible := m.portVisibleListeners()
	heights := make([]int, len(visible))
	for i, listener := range visible {
		heights[i] = len(m.portCardRowLines(listener, content, false))
	}

	var rows []string
	start, end := 0, 0
	if len(visible) == 0 {
		// The empty list keeps its own row, so the card says what the empty reading
		// means instead of leaving a void under its chip.
		rows = append(rows, MutedStyle.Render(portEmptyListLine(m.Ports.Loaded, m.Ports.Loading)))
	} else {
		start, end = utilitiesRowWindow(heights, m.Ports.Cursor, max(1, room-reserved), false)
		if start > 0 {
			rows = append(rows, MutedStyle.Render(fmt.Sprintf("↑ %d more above", start)))
		}
		for i := start; i < end; i++ {
			rows = append(rows, m.portCardRowLines(visible[i], content, i == m.Ports.Cursor)...)
		}
		if end < len(visible) {
			rows = append(rows, MutedStyle.Render(fmt.Sprintf("↓ %d more below", len(visible)-end)))
		}
	}
	if len(rows) > room {
		rows = rows[:room]
	}

	groups := []cardGroup{{Lines: rows}}
	if len(notice) > 0 {
		// A group after the first is set off by the card's own rule, which is the
		// divider the compact screen draws above its notice.
		groups = append(groups, cardGroup{Lines: notice})
	}
	return cardGroups(portManagerTitle, "", width, groups), start, end
}

// portManagerFactsCard is the card beside the reading: what the reading is (its
// scope and its filter, sort and refresh state), and -- once Enter has opened the
// row under the cursor -- that socket's own facts. The card holds the rows its
// column was given, so opening the detail adds facts to the block the column
// already holds instead of moving the composition under the user.
func (m Model) portManagerFactsCard(width, budget int) []string {
	content := cardContentWidth(width)
	room := max(1, budget-cardChromeRows)

	head := mutedLines(wrapSectionParagraphs([]string{m.portScopeSentence(), m.portStateLine()}, content))
	head = clipLines(head, room, content)
	groups := []cardGroup{{Lines: head}}
	used := len(head)

	if m.Ports.Detail {
		if listener, ok := m.portSelectedListener(); ok {
			facts := iconFactLines(m.portFacts(listener), content)
			facts = clipLines(facts, max(1, room-used-1), content)
			groups = append(groups, cardGroup{Title: portSelectedGroup, Lines: facts})
			used += 1 + len(facts)
		}
	}

	// The column keeps its rows whether or not the detail is open, exactly as the
	// section screens' detail column does: a composition that grew by the facts
	// would climb under the user on the press that opened them.
	last := len(groups) - 1
	groups[last].Lines = padRows(groups[last].Lines, len(groups[last].Lines)+max(0, room-used))
	return cardGroups(portFactsChip, "", width, groups)
}

// portCardRowLines draws one socket in the grammar's row: the port first, the
// protocol, state and address beside it, and the owner and its reach on the muted
// line under them. The row wears the chevron because Enter opens its facts.
func (m Model) portCardRowLines(listener portscan.Listener, width int, selected bool) []string {
	return cardRowLines(m.portCardRow(listener), width, selected)
}

// portCardRow is one socket as the grammar's row.
func (m Model) portCardRow(listener portscan.Listener) cardRow {
	return cardRow{
		Title:    fmt.Sprintf(":%d  %s  %s  %s", listener.Port, listener.Protocol, listener.State, listener.Address),
		Subtitle: m.portCardRowSubtitle(listener),
		Chevron:  true,
	}
}

// portCardRowSubtitle is the muted line under a socket: who holds it and what the
// bind covers, in the words the compact row already uses.
func (m Model) portCardRowSubtitle(listener portscan.Listener) string {
	parts := []string{portOwnerLabel(listener)}
	switch {
	case listener.Loopback():
		parts = append(parts, "loopback only")
	case listener.Wildcard():
		parts = append(parts, "all interfaces")
	}
	if m.portShared(listener) {
		parts = append(parts, "shared")
	}
	return strings.Join(parts, "  ")
}

// portFacts is the selected socket's own facts, one labelled value each: the
// socket, where the reading found it, the process behind it and the reading's own
// limits. Every value is a field of the reading; a fact the reading could not
// establish says so in the screen's own words instead of being left out.
func (m Model) portFacts(listener portscan.Listener) []iconFact {
	facts := []iconFact{
		{Label: "Port", Value: fmt.Sprintf(":%d %s %s %s", listener.Port, listener.Protocol, listener.State, listener.Address)},
		{Label: "Family", Value: string(listener.Protocol.Family())},
		{Label: "State", Value: string(listener.State)},
		{Label: "Source", Value: listener.Source},
	}

	if listener.Owner == nil {
		reason := listener.OwnerErr
		if reason == "" {
			reason = "no process this run can see holds this socket."
		}
		return append(facts,
			iconFact{Label: "Owner", Value: "not readable. " + reason},
			iconFact{Label: "Note", Value: "A port whose owner is unknown is not a free port."},
		)
	}

	owner := *listener.Owner
	facts = append(facts, iconFact{
		Label: "Owner",
		Value: fmt.Sprintf("%s (pid %d, user %s)", portOwnerName(owner), owner.PID, portUserLabel(owner)),
	})
	if owner.Exe != "" {
		facts = append(facts, iconFact{Label: "Executable", Value: owner.Exe})
	}
	if len(owner.Args) > 0 {
		// The reading redacts the arguments before they reach the model, so this is
		// the same text the compact screen prints and the copy sends.
		facts = append(facts, iconFact{Label: "Arguments", Value: "(redacted) " + strings.Join(owner.Args, " ")})
	}
	if started := owner.StartedText(); started != "" {
		facts = append(facts, iconFact{Label: "Started", Value: started})
	}
	if !owner.IdentityKnown() {
		facts = append(facts, iconFact{
			Label: "Identity",
			Value: "cannot be revalidated on this platform; a stop checks liveness only.",
		})
	}
	for _, limit := range m.Ports.Snapshot.Limits {
		facts = append(facts, iconFact{Label: "Limitation", Value: limit})
	}
	return facts
}

// renderPortStopWide draws the confirmation as one card: the account of what the
// stop would do -- the identity, the owner, every port the process holds, the
// consequences and the outcome once one has landed -- and the answers it offers
// under the card's own rule. Nothing is signalled while it draws.
func (m Model) renderPortStopWide(l layout, hints []installerHint, budget int) string {
	content := cardContentWidth(l.Composition)
	rows := m.portStopRows()

	// The answers keep their rows; the prose is what gives way on a short frame,
	// because the action is what the screen exists to offer.
	prose := mutedLines(wrapSectionParagraphs(m.portStopParagraphs(), content))
	prose = clipLines(prose, max(1, budget-cardChromeRows-1-len(rows)), content)

	groups := []cardGroup{{Lines: prose}}
	if len(rows) > 0 {
		answers := make([]string, 0, len(rows))
		for _, row := range rows {
			// The rows act where they stand and carry their own letter, so none of
			// them is the row under a cursor and none wears the chevron.
			answers = append(answers, sectionRowLines(row, content, false)...)
		}
		groups = append(groups, cardGroup{Lines: answers})
	}
	body := cardGroups(portStopTitle, "", l.Composition, groups)
	return m.frame(m.headerName(), "", centeredBody(body, l), hints)
}
