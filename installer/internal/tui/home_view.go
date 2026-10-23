package tui

import (
	"fmt"
	"strings"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// ============================================================================
// HOME — the reference's first screen
// ============================================================================
//
// The reference draws the home screen as two columns. The left column is the
// lockup, its tagline, the four machine cards, the greeting, the environment
// sentence, the filled call to action and the creature. The right column is
// three cards: the machine the run would change, the plan's next steps with
// their numbers, and the newest backup (or nothing when there is none).
//
// The wide shape is the reference's; the 80x24 floor keeps the compact dashboard
// below (see dashboard.go), because the two columns cannot both hold their facts
// there and a card that cannot hold a fact is worse than the compact screen that
// can. Every value is the model's: detection's fields, planFor's own steps, the
// backup list the startup read loaded, and the build label.

const (
	homeTagline     = dashboardTagline
	homeTaglineWarm = "Fast. Consistent. Yours."
	homeStartLabel  = "Press [s] to start installation"
	homeExploreHint = "or explore the menu with [Tab]"
)

// The composition's two measures. They are named here rather than written at
// each block, because the screen's whole point is that the space between two
// things is ONE space: the rows above both columns are the same rows, and a gap
// between the lockup and the tagline is the same gap as the one between two
// cards. The gutter between the columns is not a third measure -- it is
// layoutGutter, the columns' own gutter on every two-column screen.
const (
	// homeTopMargin is the blank rows both columns open with, so the two columns
	// start on one row and neither block sits against the header's rule.
	homeTopMargin = 1

	// homeBlockGap is the gap between two blocks of a column, in rows. It is ONE
	// measure for the whole screen -- the rows under the lockup are the rows
	// between two cards -- so the composition has a rhythm instead of each block
	// floating the distance its own block asked for.
	homeBlockGap = 1
)

// homeWide reports whether the reference's two-column home fits the frame. It is
// a row budget as well as a width: below it the cards would have to drop a fact,
// which is the compact dashboard's job.
func (m Model) homeWide(l layout) bool {
	if !l.TwoColumn {
		return false
	}
	return installerBodyRows(m.Height, m.footerReserveRows(l.Inner, m.welcomeHints())) >= 30
}

// welcomeHints is the home screen's footer: the press that starts the run, the one
// that leaves, and the one that opens the menu. It is the screen's footer at both
// shapes, so the wide home and the 80x24 floor cannot promise different keys.
func (m Model) welcomeHints() []installerHint {
	return []installerHint{hintStart, hintQuit, {"[Tab]", "menu"}}
}

// renderHomeWide draws the reference's home screen. The body is composed from
// the two columns before the frame sees it, so the frame still owns the header,
// the status rule, the footer and the creature's rows.
//
// The composition keeps its internal rhythm from its top row: both columns open
// on the same blank rows (homeTopMargin), one gutter separates their blocks
// (homeBlockGap), and each block is drawn against its column's left edge. The
// block those measures make is then CENTRED between the frame's rule and the
// creature's block, which is where the whole composition sits on the reference
// screens: anchored to the rule it spent the difference as one band of void under
// the cards, and centred without the pet's reserve it would have centred through
// the creature instead. The reserve is taken off the budget before either column
// is measured, so the pet keeps the rows the terminal gave it on every frame.
func (m Model) renderHomeWide(l layout) string {
	budget := installerBodyRows(m.Height, m.footerReserveRows(l.Inner, m.welcomeHints()))

	// The creature's block is the frame's, so it is taken off the rows the
	// composition is centred in: the pet keeps the rows the terminal gave it
	// whether or not a column wanted them, and the block is centred in what is left
	// above it. A frame too tight to hold both keeps the whole budget for the
	// columns, which is the one case where the pet yields instead of the facts.
	reserve := m.companionBlockRows(l.Inner)
	rows := budget - reserve
	if rows < 1 {
		rows, reserve = budget, 0
	}

	left := stackHomeBlocks(m.homeLeftBlocks(l.Left), rows)
	right := stackHomeBlocks(m.homeRightBlocks(l.Right), rows)

	// The composed rows are the screen's rows, so the frame's own placement leaves
	// them where they are: the block is centred in the rows above the creature, and
	// the reserve is the tail under it that the creature is drawn in.
	body := placeHomeBody(composeColumns(left, right, l), rows)
	body = append(body, make([]string, reserve)...)
	return m.frameWithPanels(m.headerName(), "", body, m.welcomeHints(), nil)
}

// stackHomeBlocks draws a column: the shared top margin, then the blocks it was
// given one homeBlockGap apart. A block is never cut through its own rows, and the
// gutter is given back before a block is: the gap is the screen's taste, while a
// block is a fact. When even the shortest gutters do not fit, the blocks that do
// not fit whole are left out rather than sliced.
func stackHomeBlocks(blocks [][]string, rows int) []string {
	for gap := homeBlockGap; gap >= 0; gap-- {
		if out, ok := stackHomeBlocksAt(blocks, gap, rows); ok {
			return out
		}
	}
	out, _ := stackHomeBlocksAt(blocks, 0, rows)
	return out
}

// stackHomeBlocksAt is stackHomeBlocks at one gutter width, reporting whether
// every block fitted.
func stackHomeBlocksAt(blocks [][]string, gap, rows int) ([]string, bool) {
	out := make([]string, homeTopMargin)
	spent := homeTopMargin
	placed := 0
	for _, block := range blocks {
		if len(block) == 0 {
			continue
		}
		need := len(block)
		if placed > 0 {
			need += gap
		}
		if spent+need > rows {
			return out, false
		}
		if placed > 0 {
			out = append(out, make([]string, gap)...)
			spent += gap
		}
		out = append(out, block...)
		spent += len(block)
		placed++
	}
	return out, true
}

// placeHomeBody centres the composition in the rows it was given. The surplus the
// columns did not spend is split above and below the block in equal parts, with
// the odd row, if any, below -- the placement every framed screen uses, so the
// home screen's block reads as one composition between the header and the
// creature instead of content glued to the rule with one band of void under it.
//
// The block keeps its own first row (homeTopMargin), so both columns still start
// together and the block never sits against the rule; the caller reserves the
// creature's rows before calling this, so the space under the block is the
// creature's band rather than half of the centring.
func placeHomeBody(body []string, rows int) []string {
	if len(body) >= rows {
		return body
	}
	out := make([]string, rows)
	copy(out[(rows-len(body))/2:], body)
	return out
}

// homeLeftBlocks is the left column as blocks, in the reference's order: the
// lockup, the tagline, the machine chips, the greeting with the environment
// sentence, and the call to action with the hint that opens the menu. Every block
// is drawn against the column's own left edge -- the lockup included -- so the
// column has one left edge and the chips line up with the words above them, which
// is what the centred text under a flush-left band of cards did not.
func (m Model) homeLeftBlocks(width int) [][]string {
	var blocks [][]string

	var lockup []string
	if m.Height >= 46 && width >= wordmarkWidth() {
		lockup = append(lockup, renderEmblem(logo)...)
		// The blank between the emblem and the wordmark is the lockup's own
		// spacing, and it is the same one row as every gutter on the screen.
		lockup = append(lockup, "")
		lockup = append(lockup, renderWordmark(dotfilesText)...)
	} else {
		lockup = append(lockup, renderEmblem(compactLogo)...)
		lockup = append(lockup, renderWordmark("dotfiles")...)
	}
	blocks = append(blocks, lockup)

	blocks = append(blocks, []string{
		MutedStyle.Render(truncate(homeTagline, width)),
		SubtitleStyle.Render(truncate(homeTaglineWarm, width)),
	})

	// The machine chips are the dashboard's own four cards, drawn by the same
	// function at the floor: one card shape on both shapes of the screen.
	if chips := m.dashboardCardsRows(width); len(chips) > 0 {
		blocks = append(blocks, chips)
	}

	var greeting []string
	if word := m.dashboardGreeting(); word != "" {
		greeting = append(greeting, BrandStyle.Render(truncate(word, width)))
	}
	for _, line := range wrapText(m.environmentSentence(), width, 0) {
		greeting = append(greeting, MeterStyle.Render(line))
	}
	blocks = append(blocks, greeting)

	blocks = append(blocks, []string{
		ButtonStyle.Render("▶ " + homeStartLabel),
		MutedStyle.Render(truncate(homeExploreHint, width)),
	})

	// The creature is the frame's own: it takes the rows under both columns, which
	// is the reference's placement. Drawing it here as well would double it.
	return blocks
}

// homeRightBlocks is the reference's three cards as blocks: the machine, the next
// steps and the newest backup. A card is omitted when the model holds no fact for
// it -- a host that was never detected gets no machine card, a model with no
// backup gets no backup card -- rather than drawn with a guess.
func (m Model) homeRightBlocks(width int) [][]string {
	var blocks [][]string
	if facts := m.homeMachineFacts(width); len(facts) > 0 {
		blocks = append(blocks, card("Your machine", "This machine, now · Did you know?", width, facts))
	}
	if steps := m.homeNextSteps(); len(steps) > 0 {
		body := numberedStepLines(steps, cardContentWidth(width))
		blocks = append(blocks, card("Next steps", "What will happen? · Your trainer", width, body))
	}
	if backup, ok := newestBackup(m.AvailableBackups); ok {
		blocks = append(blocks, card("Recent backup", "Your safety net", width, m.homeBackupLines(backup, cardContentWidth(width))))
	}
	return blocks
}

// homeMachineFacts is the machine card's body: one fact per row, each from a
// field detection filled. The icons are decoration; the label and value are the
// card.
func (m Model) homeMachineFacts(width int) []string {
	info := m.SystemInfo
	if info == nil {
		return nil
	}
	content := cardContentWidth(width)
	facts := []iconFact{
		{Icon: dashboardOSIcon, Label: "OS", Value: info.OSName},
		{Icon: "\uf109", Label: "Host", Value: m.platformHost()},
		{Icon: dashboardArchIcon, Label: "Arch", Value: archName(info)},
		{Icon: dashboardShellIcon, Label: "Shell", Value: info.UserShell},
		{Icon: dashboardPackagesIcon, Label: "Packages", Value: m.packageManager()},
		{Icon: "\uf015", Label: "Home", Value: info.HomeDir},
	}
	return iconFactLines(facts, content)
}

// homeNextSteps is the plan as the reference's numbered list: one step per area
// the run meets, with the area's real name and the real steps under it. It is
// planFor's own plan, so the home screen cannot describe a run the wizard would
// not make.
func (m Model) homeNextSteps() []numberedStep {
	steps := planFor(m.planOptions())
	if len(steps) == 0 {
		return nil
	}

	var order []string
	byArea := map[string][]string{}
	for _, step := range steps {
		area := step.Area
		if area == "" {
			area = areaPreparation
		}
		if _, seen := byArea[area]; !seen {
			order = append(order, area)
		}
		byArea[area] = append(byArea[area], step.Name)
	}

	out := make([]numberedStep, 0, len(order))
	for _, area := range order {
		names := byArea[area]
		subtitle := names[0]
		switch {
		case len(names) == 2:
			subtitle = names[0] + " and " + names[1]
		case len(names) > 3:
			subtitle = strings.Join(names[:3], ", ") + fmt.Sprintf(" and %d more", len(names)-3)
		case len(names) == 3:
			subtitle = strings.Join(names, ", ")
		}
		out = append(out, numberedStep{Title: area, Subtitle: subtitle})
		if len(out) == 4 {
			break
		}
	}
	return out
}

// homeBackupLines is the newest backup card's body: when it was taken, what it
// holds, how old it is against the model's own clock, and where it lives. A backup
// with no files still says when it was taken; a zero reference clock leaves the
// age out rather than printing a moment nobody measured.
func (m Model) homeBackupLines(backup system.BackupInfo, width int) []string {
	stamp := backup.Timestamp.Format(panelBackupStamp)
	held := fileCount(len(backup.Files))
	if age := backupAge(backup.Timestamp, m.Now); age != "" && !m.Now.IsZero() {
		held += ", " + age
	}
	out := []string{
		AccentKeyStyle.Render("\uf017") + " " + InkStyle.Render(stamp),
		InkStyle.Render(held),
	}
	for _, line := range wrapText(backup.Path, width, 0) {
		out = append(out, MutedStyle.Render(line))
	}
	return out
}
