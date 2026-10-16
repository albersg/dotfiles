package tui

// The hub's two sections: the tools the wizard asks about, and what the run
// installs around them without asking. A row whose section differs from the one
// before it is set off by the menu's rule.
const (
	learnSectionChosen = iota
	learnSectionInstalled
)

// learnToolRow is one row of the Learn About Tools hub: the label the menu shows
// and the screen it opens. The menu is built from this table and the key handler
// resolves its answer through it, so a page cannot be offered without a row or
// opened without being offered (TestLearnHubRowsAndOptionsAgree).
type learnToolRow struct {
	Label   string
	Screen  Screen
	Section int
}

// learnToolRows is the hub in the order it reads: first the four tools the
// wizard asks about, then the font and the prompts it installs for them. It is
// the only way the main menu reaches these pages; before the hub the row named
// "Learn About Tools" opened the terminal list directly and the shells,
// multiplexers and Neovim pages were reachable only from inside a run.
func learnToolRows() []learnToolRow {
	return []learnToolRow{
		{"Terminals", ScreenLearnTerminals, learnSectionChosen},
		{"Shells", ScreenLearnShells, learnSectionChosen},
		{"Multiplexers", ScreenLearnWM, learnSectionChosen},
		{"Neovim", ScreenLearnNvim, learnSectionChosen},
		{"Font", ScreenLearnFont, learnSectionInstalled},
		{"Prompts", ScreenLearnPrompts, learnSectionInstalled},
		{"Shell tooling", ScreenLearnTooling, learnSectionInstalled},
		{"Plugins", ScreenLearnPlugins, learnSectionInstalled},
		{"Machine toolset", ScreenLearnToolset, learnSectionInstalled},
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

// learnToolsOptions is the hub's menu: one row per page, a rule where the
// section changes, and the way back.
func (m Model) learnToolsOptions() []string {
	rows := learnToolRows()
	options := make([]string, 0, len(rows)+3)
	section := learnSectionChosen
	for i, row := range rows {
		if i > 0 && row.Section != section {
			options = append(options, m.menuSeparator())
			section = row.Section
		}
		options = append(options, row.Label)
	}
	return append(options, m.menuSeparator(), "← Back")
}
