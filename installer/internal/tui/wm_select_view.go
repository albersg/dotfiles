package tui

// ============================================================================
// THE MULTIPLEXER SELECT SCREEN — the wizard's fifth question in the reference
// grammar
// ============================================================================
//
// The OS, terminal, font and shell questions draw the page AS the card
// (os_select_view.go, terminal_select_view.go, font_select_view.go,
// shell_select_view.go): the title is a filled chip set into the card's top
// border, its caption sits at the right of that same row, a rule closes the pair,
// and every answer is a row of icon, title, muted subtitle and chevron with the row
// under the cursor filled across the card. This file is that shape for the
// multiplexer question, through the same card and the same rows (questionCard,
// questionCardGroups, components.go) -- not a second renderer that would drift
// from them.
//
// Nothing here decides anything the screen did not already decide. The answers are
// m.GetCurrentOptions() in the order the key handler reads them, the separator the
// option list carries is the card's own divider, the title is the screen's own
// title without its step prefix (the header's meter owns the number), the caption
// is the screen's own description, and the cursor is m.Cursor. Enter still records
// the row the marker is on and Esc still leaves. The three offered multiplexers are
// the model's own list, so a Termux host -- where Herdr is not packaged -- draws
// three answers and no fourth.
//
// The subtitles are the row's decoration and claim nothing the installer does not
// already state: the three multiplexers each carry a phrase from the tool page this
// installer ships for them (tools_info.go's GetWMInfo), `None` says only what
// pressing it really does -- the step installs no multiplexer -- and the Learn row
// says only what pressing it opens, the pages that hold each one's pros and cons.

const (
	// wmSelectTmuxIcon is the setup table's multiplexer mark: one decision wears
	// one glyph wherever it is drawn (setup_view.go's setupMultiplexerIcon).
	wmSelectTmuxIcon   = setupMultiplexerIcon // nf-fa-th: the panes a multiplexer splits a screen into
	wmSelectZellijIcon = "\uf009"             // nf-fa-th-large: the layouts it arranges them with
	wmSelectHerdrIcon  = "\uf0c0"             // nf-fa-users: the agent sessions it is built for
	wmSelectNoneIcon   = "\uf05e"             // nf-fa-ban: the step that installs nothing
	wmSelectLearnIcon  = "\uf05a"             // nf-fa-info-circle: the row that opens a page

	// wmSelectLearnLabel is the option string the key handler matches to open the
	// multiplexer pages. It is the label GetCurrentOptions returns, not a copy.
	wmSelectLearnLabel = "Learn about multiplexers"

	// wmSelectSubtitleMeasure is the longest subtitle the rows carry. A card row
	// cuts its subtitle to one line, and the left column of the two-column frame
	// leaves 50 columns for it at the 124-column floor, so a longer sentence would be
	// the card's only cut word. The guard beside this file holds every subtitle to
	// it.
	wmSelectSubtitleMeasure = 50

	// wmSelectCardMinContent is the narrowest card content the grammar's rows are
	// drawn in: the gutter and the chevron, a title worth reading and the subtitle
	// beside them. Below it the compact list is the same answers in one line each,
	// which is a shape that cuts less than a card with no room for its own rows. It
	// is the other four questions' own floor, so the pickers change shape at the same
	// frame.
	wmSelectCardMinContent = 32
)

// wmSelectRowParts is one answer's icon and the muted line under it, keyed by the
// option string the key handler records. The table is keyed by the label rather
// than by the row's position, so an answer cannot be drawn with another answer's
// subtitle. A row the table does not know is drawn as it is and claims nothing; the
// guard beside this file fails on a row the table is missing.
func wmSelectRowParts(label string) (icon, subtitle string) {
	switch label {
	case "Tmux":
		return wmSelectTmuxIcon, "Sessions, windows and panes"
	case "Zellij":
		return wmSelectZellijIcon, "Floating panes, tabs and built-in hints"
	case "Herdr":
		return wmSelectHerdrIcon, "Multiplexer for AI coding sessions"
	case "None":
		return wmSelectNoneIcon, "Install no multiplexer"
	case wmSelectLearnLabel:
		return wmSelectLearnIcon, "Each multiplexer's pros and cons"
	}
	return "", ""
}

// wmSelectGroups is the card's body: the multiplexer answers in one group and,
// where the option list puts its separator, the Learn row in the group the card
// divides off. The rows are the shared builder's, so the five questions are one
// shape.
func (m Model) wmSelectGroups(content int) []cardGroup {
	return questionCardGroups(m.GetCurrentOptions(), m.Cursor, content, wmSelectRowParts)
}

// wmSelectCardBody draws the screen's body in the reference grammar, or reports
// that the frame cannot hold the card. The gate is the frame and nothing else -- a
// width its rows are readable in and a row budget that holds the composed card,
// divider included -- so the shape cannot change while the cursor walks the
// answers. The multiplexer question's note is one sentence, so on a frame whose
// border row cannot hold it the shared card keeps it inside itself.
func (m Model) wmSelectCardBody(hints []installerHint) ([]string, bool) {
	width, content, ok := m.questionCardWidth(wmSelectCardMinContent)
	if !ok {
		return nil, false
	}
	return m.questionCardBody(stripStepPrefix(m.GetScreenTitle()), m.GetScreenDescription(),
		width, m.wmSelectGroups(content), hints)
}
