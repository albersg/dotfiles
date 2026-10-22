package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

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

// homeWide reports whether the reference's two-column home fits the frame. It is
// a row budget as well as a width: below it the cards would have to drop a fact,
// which is the compact dashboard's job.
func (m Model) homeWide(l layout) bool {
	if !l.TwoColumn {
		return false
	}
	return installerBodyRows(m.Height, m.footerReserveRows(l.Inner, m.welcomeHints())) >= 30
}

// welcomeHints is the home screen's footer: the two presses that move it.
func (m Model) welcomeHints() []installerHint {
	return []installerHint{hintStart, hintQuit, {"[Tab]", "menu"}}
}

// renderHomeWide draws the reference's home screen. The body is composed from
// the two columns before the frame sees it, so the frame still owns the header,
// the status rule, the footer and the creature's rows.
func (m Model) renderHomeWide(l layout) string {
	budget := installerBodyRows(m.Height, m.footerReserveRows(l.Inner, m.welcomeHints()))
	leftWidth, rightWidth := l.Left, l.Right
	left := m.homeLeftColumn(leftWidth)
	right := m.homeRightColumn(rightWidth, budget)
	// The spacing is the first thing given back when the column is one row too
	// tall: the blank rows between the lockup, the cards and the creature go before
	// the creature does, and a block that still does not fit is cut rather than
	// allowed to push the frame's header off the terminal.
	for len(left) > budget {
		cut := -1
		for i := len(left) - 2; i >= 0; i-- {
			if left[i] == "" {
				cut = i
				break
			}
		}
		if cut < 0 {
			break
		}
		left = append(left[:cut], left[cut+1:]...)
	}
	if len(left) > budget {
		left = left[:budget]
	}
	return m.frameWithPanels(m.headerName(), "", composeColumns(left, right, l), m.welcomeHints(), nil)
}

// homeLeftColumn is the lockup and everything under it, measured against the
// column it will occupy. The creature is drawn into the column's own bottom rows
// so it can sit beside the right column, as the reference draws it.
func (m Model) homeLeftColumn(width int) []string {
	var body []string

	if m.Height >= 46 && width >= wordmarkWidth() {
		body = append(body, renderEmblem(logo)...)
		body = append(body, "")
		body = append(body, renderWordmark(dotfilesText)...)
	} else {
		body = append(body, renderEmblem(compactLogo)...)
		// The blank between the emblem and the wordmark is the full lockup's own
		// spacing; the compact lockup spends it on the creature instead.
		body = append(body, renderWordmark("dotfiles")...)
	}

	body = append(body, CenterHorizontally(MutedStyle.Render(truncate(homeTagline, width)), width))
	body = append(body, CenterHorizontally(SubtitleStyle.Render(truncate(homeTaglineWarm, width)), width))
	body = append(body, "")

	if band := m.homeMachineCards(width); len(band) > 0 {
		body = append(body, band...)
	}
	body = append(body, "")

	if greeting := m.dashboardGreeting(); greeting != "" {
		body = append(body, CenterHorizontally(BrandStyle.Render(truncate(greeting, width)), width))
	}
	for _, line := range wrapText(m.environmentSentence(), width, 0) {
		body = append(body, CenterHorizontally(MeterStyle.Render(line), width))
	}
	body = append(body, "")

	button := ButtonStyle.Render("▶ " + homeStartLabel)
	body = append(body, CenterHorizontally(button, width))
	body = append(body, CenterHorizontally(MutedStyle.Render(truncate(homeExploreHint, width)), width))

	// The creature is the frame's own: it takes the blank rows under this column,
	// which is the reference's placement. Drawing it here as well would double it.
	return body
}

// homeMachineCards draws the four machine cards as the reference does: a small
// bordered card per fact with the icon and the label on one row and the value on
// the next. Where the cards cannot fit their own values the same facts are drawn
// in one wrapped row, which loses no fact.
func (m Model) homeMachineCards(width int) []string {
	cards := m.dashboardCards()
	if len(cards) == 0 {
		return nil
	}
	each := (width - grammarGap*(len(cards)-1)) / len(cards)
	need := 0
	for _, card := range cards {
		need = max(need, lipgloss.Width(card.label), lipgloss.Width(card.value))
	}
	need += 2*grammarCardPad + panelFrame
	if each < need {
		return m.dashboardCardFacts(width)
	}

	const rows = 4
	out := make([]string, rows)
	for _, machine := range cards {
		box := machineCardOf(machine, each)
		for r, line := range box {
			if out[r] == "" {
				out[r] = line
				continue
			}
			out[r] += strings.Repeat(" ", grammarGap) + line
		}
	}
	return out
}

// machineCardOf draws one machine card: the top border, the icon and label, the
// value and the closing border.
func machineCardOf(card dashboardCard, width int) []string {
	edge := func(s string) string { return PanelBorderStyle.Render(s) }
	inner := max(1, width-panelFrame)
	content := max(1, inner-2*grammarCardPad)
	row := func(s string) string {
		return edge("│") + strings.Repeat(" ", grammarCardPad) +
			padRight(truncate(s, content), content) + strings.Repeat(" ", grammarCardPad) + edge("│")
	}
	return []string{
		edge("╭" + strings.Repeat("─", inner) + "╮"),
		row(AccentKeyStyle.Render(card.icon) + " " + MutedStyle.Render(card.label)),
		row(InkStyle.Render(card.value)),
		edge("╰" + strings.Repeat("─", inner) + "╯"),
	}
}

// homeRightColumn is the reference's three cards, stacked: the machine, the next
// steps and the newest backup. A card is omitted when the model holds no fact
// for it -- a host that was never detected gets no machine card, a model with no
// backup gets no backup card -- rather than drawn with a guess.
func (m Model) homeRightColumn(width, budget int) []string {
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
	return fitColumnBlocks(blocks, budget)
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
