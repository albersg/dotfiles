package tui

import "strings"

// ============================================================================
// THE OS SELECT SCREEN — the wizard's first question in the reference grammar
// ============================================================================
//
// The wizard's questions drew a page title, the model's description under it and
// the answers as one-line bars inside the frame's card. The reference draws the
// page AS the card: the title is a filled chip set into the card's top border,
// its caption sits at the right of that same row, a rule closes the pair, and
// every answer is a row of icon, title, muted subtitle and chevron with the row
// under the cursor filled across the card. This file is that shape for the OS
// step, and for it alone: the other questions keep the compact list until their
// own rows have subtitles (odd/tasks/installer-redesign.md, L16).
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

// osSelectRowLines draws every answer as one of the grammar's rows, in the order
// the key handler reads them, with the cursor's row filled and marked. A row the
// cursor is not on keeps its own icon in the gutter, so the two states differ by
// a bar and a marker rather than by an indent.
func (m Model) osSelectRowLines(content int) []string {
	options := m.GetCurrentOptions()
	lines := make([]string, 0, len(options)*2)
	for i, label := range options {
		icon, subtitle := osSelectRowParts(label)
		lines = append(lines, cardRowLines(cardRow{
			Icon:     icon,
			Title:    label,
			Subtitle: subtitle,
			Chevron:  true,
		}, content, i == m.Cursor)...)
	}
	return lines
}

// osSelectCard wraps the rows in the reference's card: the screen's own title as
// the chip, its own description as the caption, the rule under the pair and the
// rows.
//
// A caption the border row has no room for moves inside the card rather than
// being dropped. On a host where detection found nothing, the description is the
// sentence that says so and that the step is the user's to answer; the card gives
// its caption back first (card, components.go), so the caller is what has to keep
// the fact.
func (m Model) osSelectCard(width int, rows []string) []string {
	title := stripStepPrefix(m.GetScreenTitle())
	caption := m.GetScreenDescription()
	lines := card(title, caption, width, rows)
	if caption == "" || strings.Contains(lines[0], caption) {
		return lines
	}
	body := append(mutedLines(wrapText(caption, cardContentWidth(width), 0)), rows...)
	return card(title, "", width, body)
}

// osSelectCardBody draws the screen's body in the reference grammar, or reports
// that the frame cannot hold the card. The gate is the frame and nothing else --
// a width its rows are readable in and a row budget that holds every answer at
// once -- so the shape cannot change while the cursor walks the answers.
//
// The gate measures the rows against the whole list rather than windowing it: the
// OS question offers three answers, and a card that hid one behind its own bottom
// border would be a row the cursor can reach and the reader cannot.
func (m Model) osSelectCardBody(hints []installerHint) ([]string, bool) {
	l := layoutFor(m)
	width := l.Composition
	if l.TwoColumn {
		width = l.Left
	}
	if width-panelFrame-2*grammarCardPad < osSelectCardMinContent {
		return nil, false
	}
	rows := m.osSelectRowLines(cardContentWidth(width))
	if installerBodyRows(m.Height, m.footerReserveRows(l.Inner, hints)) < cardChromeRows+len(rows) {
		return nil, false
	}
	return m.osSelectCard(width, rows), true
}
