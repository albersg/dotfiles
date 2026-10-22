package tui

// ============================================================================
// THE SHELL SELECT SCREEN — the wizard's third question in the reference
// grammar
// ============================================================================
//
// The OS and terminal questions draw the page AS the card (os_select_view.go,
// terminal_select_view.go): the title is a filled chip set into the card's top
// border, its caption sits at the right of that same row, a rule closes the pair,
// and every answer is a row of icon, title, muted subtitle and chevron with the row
// under the cursor filled across the card. This file is that shape for the shell
// question, through the same card and the same rows (questionCard,
// questionCardGroups, components.go) -- not a second renderer that would drift
// from them.
//
// Nothing here decides anything the screen did not already decide. The answers are
// m.GetCurrentOptions() in the order the key handler reads them, the separator the
// option list carries is the card's own divider, the title is the screen's own
// title without its step prefix (the header's meter owns the number), the caption
// is the screen's own description, and the cursor is m.Cursor. Enter still records
// the row the marker is on and Esc still leaves.
//
// The subtitles are the row's decoration and claim nothing the installer does not
// already state: fish, zsh and nushell each carry the phrase the installer's own
// shell pages and plugin registry use for what the run gives that shell (the
// fisher bootstrap, Oh My Zsh with Powerlevel10k and the plugin packages, the
// config.nu and env.nu it writes), and the Learn row says only what pressing it
// really does -- it opens the pages that hold each shell's pros and cons.

const (
	shellSelectFishIcon    = "\uf0d0" // nf-fa-magic: the shell that needs no configuration
	shellSelectZshIcon     = "\uf120" // nf-fa-terminal: the shell the shipped .zshrc is for
	shellSelectNushellIcon = "\uf1c0" // nf-fa-database: the shell that thinks in tables
	shellSelectLearnIcon   = "\uf05a" // nf-fa-info-circle: the row that opens a page

	// shellSelectLearnLabel is the option string the key handler matches to open
	// the shell pages. It is the label GetCurrentOptions returns, not a copy.
	shellSelectLearnLabel = "Learn about shells"

	// shellSelectSubtitleMeasure is the longest subtitle the rows carry. A card
	// row cuts its subtitle to one line, and the left column of the two-column frame
	// leaves 50 columns for it at the 124-column floor, so a longer sentence would be
	// the card's only cut word. The guard beside this file holds every subtitle to
	// it.
	shellSelectSubtitleMeasure = 50

	// shellSelectCardMinContent is the narrowest card content the grammar's rows
	// are drawn in: the gutter and the chevron, a title worth reading and the
	// subtitle beside them. Below it the compact list is the same answers in one
	// line each, which is a shape that cuts less than a card with no room for its
	// own rows. It is the other two questions' own floor, so the three questions
	// change shape at the same frame.
	shellSelectCardMinContent = 32
)

// shellSelectRowParts is one answer's icon and the muted line under it, keyed by
// the option string the key handler records. The table is keyed by the label rather
// than by the row's position, so an answer cannot be drawn with another answer's
// subtitle. A row the table does not know is drawn as it is and claims nothing; the
// guard beside this file fails on a row the table is missing.
func shellSelectRowParts(label string) (icon, subtitle string) {
	switch label {
	case "Fish":
		return shellSelectFishIcon, "Fisher and the functions this repository ships"
	case "Zsh":
		return shellSelectZshIcon, "Oh My Zsh, Powerlevel10k and the plugins"
	case "Nushell":
		return shellSelectNushellIcon, "config.nu and env.nu this installer writes"
	case shellSelectLearnLabel:
		return shellSelectLearnIcon, "Each shell's pros and cons"
	}
	return "", ""
}

// shellSelectGroups is the card's body: the shell answers in one group and, where
// the option list puts its separator, the Learn row in the group the card divides
// off. The rows are the shared builder's, so the three questions are one shape.
func (m Model) shellSelectGroups(content int) []cardGroup {
	return questionCardGroups(m.GetCurrentOptions(), m.Cursor, content, shellSelectRowParts)
}

// shellSelectCardBody draws the screen's body in the reference grammar, or reports
// that the frame cannot hold the card. The gate is the frame and nothing else -- a
// width its rows are readable in and a row budget that holds the composed card,
// divider and caption included -- so the shape cannot change while the cursor walks
// the answers. The shell question's note names the detected shell, and the shared
// card keeps those words inside itself rather than dropping them when the border
// row has no room.
func (m Model) shellSelectCardBody(hints []installerHint) ([]string, bool) {
	width, content, ok := m.questionCardWidth(shellSelectCardMinContent)
	if !ok {
		return nil, false
	}
	return m.questionCardBody(stripStepPrefix(m.GetScreenTitle()), m.GetScreenDescription(),
		width, m.shellSelectGroups(content), hints)
}
