package tui

// ============================================================================
// THE OS SELECT SCREEN — the wizard's first question in the reference grammar
// ============================================================================
//
// The wizard's questions drew a page title, the model's description under it and
// the answers as one-line bars inside the frame's card. The reference draws the
// page AS the card: the title is a filled chip set into the card's top border,
// its caption sits at the right of that same row, a rule closes the pair, and
// every answer is a row of icon, title, muted subtitle and chevron with the row
// under the cursor filled across the card. The card, the rows and the gate are the
// shared ones (components.go); this file is the OS question's own title, glyphs and
// subtitles. The terminal question draws the same shape (terminal_select_view.go),
// and the remaining questions keep the compact list until their own rows have
// subtitles (odd/tasks/installer-redesign.md, L16).
//
// Nothing here decides anything the screen did not already decide. The answers
// are m.GetCurrentOptions() in the order the key handler reads them; the title is
// the screen's own title without its step prefix, which the header's meter owns;
// the caption is the screen's own description; and the cursor is m.Cursor. Enter
// still records the row the marker is on and Esc still leaves -- this file draws,
// it answers nothing. The icons and the subtitles are the row's decoration, in
// the same shape the utilities card and the home cards draw: the glyph is never
// the label, and each subtitle says what that platform is installed through,
// which is the only thing the choice changes about the run.

const (
	osSelectMacIcon    = "\uf179" // nf-fa-apple
	osSelectLinuxIcon  = "\uf17c" // nf-fa-linux
	osSelectTermuxIcon = "\uf17b" // nf-fa-android

	// osSelectCardMinContent is the narrowest card content the grammar's rows are
	// drawn in: the gutter and the chevron, a title worth reading and the subtitle
	// beside them. Below it the compact list is the same answers in one line each,
	// which is a shape that cuts less than a card with no room for its own rows.
	osSelectCardMinContent = 32
)

// osSelectRowParts is one answer's icon and the muted line under it. The subtitle
// names the package manager that platform is installed through -- brewed on
// macOS, the distribution's own manager on Linux, pkg on Termux -- so the row says
// what choosing it changes without inventing a fact about the host: which
// distribution or shell a machine has is the plan panel's business, not this
// row's.
func osSelectRowParts(label string) (icon, subtitle string) {
	switch label {
	case "macOS":
		return osSelectMacIcon, "Homebrew and the Apple toolchain"
	case "Linux":
		return osSelectLinuxIcon, "Your distribution's package manager"
	case "Termux":
		return osSelectTermuxIcon, "Android's terminal, through pkg"
	}
	return "", ""
}

// osSelectGroups is the card's body: one group, because the OS question has no
// separator in its option list. The rows come from the shared builder, so this
// screen and the terminal question cannot draw two different row shapes.
func (m Model) osSelectGroups(content int) []cardGroup {
	return questionCardGroups(m.GetCurrentOptions(), m.Cursor, content, osSelectRowParts)
}

// osSelectCardBody draws the screen's body in the reference grammar, or reports
// that the frame cannot hold the card. The gate is the frame and nothing else --
// a width its rows are readable in and a row budget that holds every answer at
// once -- so the shape cannot change while the cursor walks the answers. The gate
// and the card are the shared ones (questionCard, components.go); what this file
// adds is the OS question's own title, answers and glyphs.
func (m Model) osSelectCardBody(hints []installerHint) ([]string, bool) {
	width, content, ok := m.questionCardWidth(osSelectCardMinContent)
	if !ok {
		return nil, false
	}
	return m.questionCardBody(stripStepPrefix(m.GetScreenTitle()), m.GetScreenDescription(),
		width, m.osSelectGroups(content), hints)
}
