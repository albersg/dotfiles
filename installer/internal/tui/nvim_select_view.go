package tui

// ============================================================================
// THE EDITOR (NEOVIM) SELECT SCREEN — the wizard's sixth question in the
// reference grammar
// ============================================================================
//
// The OS, terminal, font, shell and multiplexer questions draw the page AS the card
// (os_select_view.go, terminal_select_view.go, font_select_view.go,
// shell_select_view.go, wm_select_view.go): the title is a filled chip set into the
// card's top border, its caption sits at the right of that same row, a rule closes
// the pair, and every answer is a row of icon, title, muted subtitle and chevron
// with the row under the cursor filled across the card. This file is that shape for
// the editor question, through the same card and the same rows (questionCard,
// questionCardGroups, components.go) -- not a second renderer that would drift from
// them.
//
// Nothing here decides anything the screen did not already decide. The answers are
// m.GetCurrentOptions() in the order the key handler reads them, the separator the
// option list carries is the card's own divider, the title is the screen's own title
// without its step prefix (the header's meter owns the number), the caption is the
// screen's own description, and the cursor is m.Cursor. Enter still records the row
// the marker is on and Esc still leaves.
//
// The subtitles are the row's decoration and claim nothing the installer does not
// already state: the install row names what the step puts on the machine, which is
// the phrase the screen's own description and the tool page this installer ships
// (tools_info.go's GetNvimInfo) both carry -- the LSP servers, TreeSitter and the
// dotfiles configuration; the skip row says only what pressing it really does; and
// the three shortcut rows say only what pressing them opens.

const (
	// nvimSelectInstallIcon is the setup table's editor mark: one decision wears one
	// glyph wherever it is drawn (setup_view.go's setupEditorIcon).
	nvimSelectInstallIcon = setupEditorIcon // nf-fa-code
	nvimSelectSkipIcon    = "\uf05e"        // nf-fa-ban: the row that installs nothing
	nvimSelectLearnIcon   = "\uf05a"        // nf-fa-info-circle: the row that opens a page
	nvimSelectKeymapsIcon = "\uf11c"        // nf-fa-keyboard-o: the reference the keys are read in
	nvimSelectGuideIcon   = "\uf02d"        // nf-fa-book: the guide that explains the setup

	// nvimSelectLearnLabel, nvimSelectKeymapsLabel and nvimSelectGuideLabel are the
	// option strings the key handler matches to open each page. They are the labels
	// GetCurrentOptions returns, not copies.
	nvimSelectLearnLabel   = "Learn about Neovim"
	nvimSelectKeymapsLabel = "View Keymaps"
	nvimSelectGuideLabel   = "LazyVim Guide"

	// nvimSelectSubtitleMeasure is the longest subtitle the rows carry. A card row
	// cuts its subtitle to one line, and the left column of the two-column frame
	// leaves 50 columns for it at the 124-column floor, so a longer sentence would be
	// the card's only cut word. The guard beside this file holds every subtitle to
	// it.
	nvimSelectSubtitleMeasure = 50

	// nvimSelectCardMinContent is the narrowest card content the grammar's rows are
	// drawn in: the gutter and the chevron, a title worth reading and the subtitle
	// beside them. Below it the compact list is the same answers in one line each,
	// which is a shape that cuts less than a card with no room for its own rows. It
	// is the other five questions' own floor, so the pickers change shape at the same
	// frame.
	nvimSelectCardMinContent = 32
)

// nvimSelectRowParts is one answer's icon and the muted line under it, keyed by the
// option string the key handler records. The table is keyed by the label rather than
// by the row's position, so an answer cannot be drawn with another answer's subtitle.
// A row the table does not know is drawn as it is and claims nothing; the guard
// beside this file fails on a row the table is missing.
func nvimSelectRowParts(label string) (icon, subtitle string) {
	switch label {
	case "Yes, install Neovim with config":
		return nvimSelectInstallIcon, "LSP, TreeSitter and the dotfiles config"
	case "No, skip Neovim":
		return nvimSelectSkipIcon, "Leave Neovim on this machine alone"
	case nvimSelectLearnLabel:
		return nvimSelectLearnIcon, "The editor, LazyVim and this config"
	case nvimSelectKeymapsLabel:
		return nvimSelectKeymapsIcon, "The keys this editor config binds"
	case nvimSelectGuideLabel:
		return nvimSelectGuideIcon, "How this LazyVim setup works"
	}
	return "", ""
}

// nvimSelectGroups is the card's body: the two editor answers in one group and,
// where the option list puts its separator, the three shortcut rows in the group the
// card divides off. The rows are the shared builder's, so the six questions are one
// shape.
func (m Model) nvimSelectGroups(content int) []cardGroup {
	return questionCardGroups(m.GetCurrentOptions(), m.Cursor, content, nvimSelectRowParts)
}

// nvimSelectCardBody draws the screen's body in the reference grammar, or reports
// that the frame cannot hold the card. The gate is the frame and nothing else -- a
// width its rows are readable in and a row budget that holds the composed card,
// divider included -- so the shape cannot change while the cursor walks the answers.
// The editor question's note is one sentence, so on a frame whose border row cannot
// hold it the shared card keeps it inside itself.
func (m Model) nvimSelectCardBody(hints []installerHint) ([]string, bool) {
	width, content, ok := m.questionCardWidth(nvimSelectCardMinContent)
	if !ok {
		return nil, false
	}
	return m.questionCardBody(stripStepPrefix(m.GetScreenTitle()), m.GetScreenDescription(),
		width, m.nvimSelectGroups(content), hints)
}
