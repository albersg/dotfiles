package tui

// ============================================================================
// THE TERMINAL SELECT SCREEN — the wizard's second question in the reference
// grammar
// ============================================================================
//
// The OS question draws the page AS the card (os_select_view.go): the title is a
// filled chip set into the card's top border, its caption sits at the right of
// that same row, a rule closes the pair, and every answer is a row of icon, title,
// muted subtitle and chevron with the row under the cursor filled across the card.
// This file is that shape for the terminal question, through the same card and the
// same rows (questionCard, questionCardGroups, components.go) -- not a second
// renderer that would drift from it.
//
// Nothing here decides anything the screen did not already decide. The answers are
// m.GetCurrentOptions() in the order the key handler reads them, the separator the
// option list carries is the card's own divider, the title is the screen's own
// title without its step prefix (the header's meter owns the number), the caption
// is the screen's own description, and the cursor is m.Cursor. Enter still records
// the row the marker is on and Esc still leaves.
//
// The subtitles are the row's decoration and claim nothing the installer does not
// already state: the four terminals carry a phrase from the tool page this
// installer ships for them (tools_info.go), and the two rows that are not a
// terminal say only what pressing them really does -- `None` installs no terminal,
// so the run leaves the one you have alone, and the Learn row opens the pages that
// hold each terminal's pros and cons.

const (
	terminalSelectAlacrittyIcon = "\uf0e7" // nf-fa-bolt: the GPU rendering it is chosen for
	terminalSelectWezTermIcon   = "\uf120" // nf-fa-terminal: the multiplexer it carries
	terminalSelectKittyIcon     = "\uf1b0" // nf-fa-paw
	terminalSelectGhosttyIcon   = "\uf2ab" // nf-fa-snapchat-ghost
	terminalSelectNoneIcon      = "\uf05e" // nf-fa-ban: the step that installs nothing
	terminalSelectLearnIcon     = "\uf05a" // nf-fa-info-circle: the row that opens a page

	// terminalSelectLearnLabel is the option string the key handler matches to open
	// the terminal pages. It is the label GetCurrentOptions returns, not a copy.
	terminalSelectLearnLabel = "Learn about terminals"

	// terminalSelectSubtitleMeasure is the longest subtitle the rows carry. A card
	// row cuts its subtitle to one line, and the left column of the two-column frame
	// leaves 50 columns for it at the 124-column floor, so a longer sentence would be
	// the card's only cut word. The guard beside this file holds every subtitle to
	// it.
	terminalSelectSubtitleMeasure = 50

	// terminalSelectCardMinContent is the narrowest card content the grammar's rows
	// are drawn in: the gutter and the chevron, a title worth reading and the
	// subtitle beside them. Below it the compact list is the same answers in one
	// line each, which is a shape that cuts less than a card with no room for its
	// own rows. It is the OS question's own floor, so the two questions change
	// shape at the same frame.
	terminalSelectCardMinContent = 32
)

// terminalSelectRowParts is one answer's icon and the muted line under it, keyed by
// the option string the key handler records. The table is keyed by the label rather
// than by the row's position, so an answer cannot be drawn with another answer's
// subtitle. A row the table does not know is drawn as it is and claims nothing; the
// guard beside this file fails on a row the table is missing.
func terminalSelectRowParts(label string) (icon, subtitle string) {
	switch label {
	case "Alacritty":
		return terminalSelectAlacrittyIcon, "GPU rendering and a TOML configuration"
	case "WezTerm":
		return terminalSelectWezTermIcon, "Lua configuration with built-in panes"
	case "Kitty":
		return terminalSelectKittyIcon, "Fast, with its own graphics protocol"
	case "Ghostty":
		return terminalSelectGhosttyIcon, "Native performance, not Electron"
	case "None":
		return terminalSelectNoneIcon, "Install no terminal"
	case terminalSelectLearnLabel:
		return terminalSelectLearnIcon, "Each terminal's pros and cons"
	}
	return "", ""
}

// terminalSelectGroups is the card's body: the terminal answers in one group and,
// where the option list puts its separator, the Learn row in the group the card
// divides off. The rows are the shared builder's, so the OS question and this one
// are one shape.
func (m Model) terminalSelectGroups(content int) []cardGroup {
	return questionCardGroups(m.GetCurrentOptions(), m.Cursor, content, terminalSelectRowParts)
}

// terminalSelectCardBody draws the screen's body in the reference grammar, or
// reports that the frame cannot hold the card. The gate is the frame and nothing
// else -- a width its rows are readable in and a row budget that holds the
// composed card, divider and caption included -- so the shape cannot change while
// the cursor walks the answers. The terminal question is the one with a note long
// enough to move inside the card (the WSL sentence, the Alacritty build cost), and
// the shared card keeps those words rather than dropping them.
func (m Model) terminalSelectCardBody(hints []installerHint) ([]string, bool) {
	width, content, ok := m.questionCardWidth(terminalSelectCardMinContent)
	if !ok {
		return nil, false
	}
	return m.questionCardBody(stripStepPrefix(m.GetScreenTitle()), m.GetScreenDescription(),
		width, m.terminalSelectGroups(content), hints)
}
