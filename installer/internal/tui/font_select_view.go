package tui

import "strings"

// ============================================================================
// THE FONT SELECT SCREEN — the wizard's third question in the reference
// grammar, and the fourth picker the grammar draws
// ============================================================================
//
// The OS, terminal and shell questions draw the page AS the card
// (os_select_view.go, terminal_select_view.go, shell_select_view.go): the title is
// a filled chip set into the card's top border, its caption sits at the right of
// that same row, a rule closes the pair, and every answer is a row of icon, title,
// muted subtitle and chevron with the row under the cursor filled across the card.
// This file is that shape for the font question, through the same card and the
// same rows (questionCard, questionCardGroups, components.go) -- not a second
// renderer that would drift from them.
//
// Nothing here decides anything the screen did not already decide. The answers are
// m.GetCurrentOptions() in the order the key handler reads them, the title is the
// screen's own title without its step prefix (the header's meter owns the number),
// the caption is the screen's own description -- which is the measurement of what
// this machine already has, so the shared card keeps those words inside itself when
// the border row has no room for them -- and the cursor is m.Cursor. Enter still
// records the row the marker is on and Esc still leaves.
//
// The subtitles are the row's decoration and claim nothing the installer does not
// already state: the install row names the face the step puts on the machine (the
// Nerd Font patch of Meslo, the family the pinned archive declares), and the skip
// row says only what pressing it really does -- the run leaves the fonts this
// machine already has alone, which is true whether the family is installed or not.

const (
	// fontSelectInstallIcon and fontSelectSkipIcon are the rows' gutter glyphs. The
	// install row wears the setup table's font mark, so one decision wears one glyph
	// wherever it is drawn; the skip row wears the ban the terminal question's own
	// "install nothing" row wears.
	fontSelectInstallIcon = setupFontIcon // nf-fa-font
	fontSelectSkipIcon    = "\uf05e"      // nf-fa-ban: the row that installs nothing

	// fontSelectSubtitleMeasure is the longest subtitle the rows carry. A card row
	// cuts its subtitle to one line, and the left column of the two-column frame
	// leaves 50 columns for it at the 124-column floor, so a longer sentence would be
	// the card's only cut word. The guard beside this file holds every subtitle to
	// it.
	fontSelectSubtitleMeasure = 50

	// fontSelectCardMinContent is the narrowest card content the grammar's rows are
	// drawn in: the gutter and the chevron, a title worth reading and the subtitle
	// beside them. Below it the compact list is the same answers in one line each,
	// which is a shape that cuts less than a card with no room for its own rows. It is
	// the other three questions' own floor, so the pickers change shape at the same
	// frame.
	fontSelectCardMinContent = 32
)

// fontSelectRowParts is one answer's icon and the muted line under it, keyed by the
// option string the key handler records. The table is keyed by the label rather
// than by the row's position, so an answer cannot be drawn with another answer's
// subtitle. The labels are read from the model rather than typed here: the first
// one carries the family's name, which the step owns, so a table of literals would
// be a second place to keep the face true.
func (m Model) fontSelectRowParts(label string) (icon, subtitle string) {
	options := m.fontOptions()
	switch label {
	case options[0]:
		return fontSelectInstallIcon, "The Nerd Font patch of Meslo"
	case options[1]:
		return fontSelectSkipIcon, "Leave the fonts on this machine alone"
	}
	return "", ""
}

// fontSelectGroups is the card's body: one group, because the font question's
// option list carries no separator. The rows come from the shared builder, so this
// screen and the other three questions cannot draw two different row shapes.
func (m Model) fontSelectGroups(content int) []cardGroup {
	return questionCardGroups(m.GetCurrentOptions(), m.Cursor, content, m.fontSelectRowParts)
}

// fontSelectCardBody draws the screen's body in the reference grammar, or reports
// that the frame cannot hold the card. The gate is the frame and nothing else -- a
// width its rows are readable in and a row budget that holds the composed card
// included -- so the shape cannot change while the cursor walks the answers. The
// font question is the one whose note is a measurement, and the shared card keeps
// those words rather than dropping them when the border row has no room.
func (m Model) fontSelectCardBody(hints []installerHint) ([]string, bool) {
	width, content, ok := m.questionCardWidth(fontSelectCardMinContent)
	if !ok {
		return nil, false
	}
	// The step's note is two sentences on a host it measured, and the model writes
	// them on two lines. The shared card wraps a caption as one block, so the line
	// break becomes a space here: the wrap then lands on the sentence boundary
	// rather than mid-sentence, and not one word of the note changes.
	note := strings.Join(strings.Split(m.GetScreenDescription(), "\n"), " ")
	return m.questionCardBody(stripStepPrefix(m.GetScreenTitle()), note,
		width, m.fontSelectGroups(content), hints)
}
