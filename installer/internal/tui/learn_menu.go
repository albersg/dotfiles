package tui

// learnToolRow is one row of the Learn About Tools hub: the label the menu shows
// and the screen it opens. The menu is built from this table and the key handler
// resolves its answer through it, so a page cannot be offered without a row or
// opened without being offered (TestLearnHubRowsAndOptionsAgree).
type learnToolRow struct {
	Label  string
	Screen Screen
}

// learnToolRows is the hub in the order it reads: first the four tools the
// wizard asks about, then -- added below as the pages land -- what the run
// installs without asking. It is the only way the main menu reaches these pages;
// before the hub the row named "Learn About Tools" opened the terminal list
// directly and the shells, multiplexers and Neovim pages were reachable only from
// inside a run.
func learnToolRows() []learnToolRow {
	return []learnToolRow{
		{"Terminals", ScreenLearnTerminals},
		{"Shells", ScreenLearnShells},
		{"Multiplexers", ScreenLearnWM},
		{"Neovim", ScreenLearnNvim},
	}
}

// learnToolRowByLabel resolves a selected menu row. The label is the row's
// identity, never its index: a row added above the cursor would otherwise make
// Enter open the page under it.
func learnToolRowByLabel(label string) (learnToolRow, bool) {
	for _, row := range learnToolRows() {
		if row.Label == label {
			return row, true
		}
	}
	return learnToolRow{}, false
}

// learnToolsOptions is the hub's menu: one row per page, then the way back.
func (m Model) learnToolsOptions() []string {
	rows := learnToolRows()
	options := make([]string, 0, len(rows)+2)
	for _, row := range rows {
		options = append(options, row.Label)
	}
	return append(options, m.menuSeparator(), "← Back")
}
