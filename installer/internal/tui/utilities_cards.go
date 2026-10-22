package tui

import (
	"fmt"
	"strings"
)

// ============================================================================
// UTILITIES — the reference's second screen
// ============================================================================
//
// The reference draws the section as a card of two-line rows with a chevron at
// the right edge and the row under the cursor filled, beside a column of four
// cards: the theme's availability with its amber badge, the machine's facts, the
// live resources and the recommended next actions. The left column ends with the
// creature.
//
// The wide shape is the reference's. The 80x24 floor keeps the compact card and
// the collapsed panel (utilities_view.go), because four stacked cards cannot hold
// their facts and the job list at once there. Every card's values come from the
// model: detection's fields, the desktop switch the section found, the metrics
// ring and the job list the handler matches.

// utilitiesWide reports whether the reference's two-column section fits the
// frame. It is decided from rows as well as width, so a wide but short terminal
// keeps the compact card rather than drawing a card past its frame.
func (m Model) utilitiesWide(l layout, budget int) bool {
	if !l.TwoColumn || l.Left < 56 || l.Right < 60 {
		return false
	}
	return budget >= 34
}

// renderUtilitiesWide draws the reference's section: the card of jobs at the
// left and the four cards at the right, with the reasons a job is missing folded
// into the theme card rather than floating above the list.
func (m Model) renderUtilitiesWide(l layout, budget int) string {
	hints := m.utilitiesHints()
	// The frame that draws this body carries these hints and no panel, so its body
	// budget is measured here with the same list: a column measured against the
	// caller's panel-aware reservation would be one row taller than the frame.
	budget = installerBodyRows(m.Height, m.footerReserveRows(l.Inner, hints))
	left := m.utilitiesJobsCard(l.Left, budget)
	right := m.utilitiesRightColumn(l.Right, budget)
	if len(left) > budget {
		left = left[:budget]
	}
	if len(right) > budget {
		right = right[:budget]
	}
	body := composeColumns(left, right, l)

	vital := ""
	rows := m.utilitiesRows()
	if end := len(rows); end > 0 {
		// The header names the machine, as the reference's does, rather than a
		// window it is not windowing on the wide frame.
		vital = m.utilitiesOverviewHeadline()
	}
	return m.frameWithPanels(m.headerName(), vital, body, hints, nil)
}

// utilitiesHints is the section's footer, the same set the compact renderer
// offers, so a key does not appear in one shape and vanish in the other.
func (m Model) utilitiesHints() []installerHint {
	hints := []installerHint{hintUp, hintDown, hintOpen}
	if m.ThemeSwitchFound {
		hints = append(hints, hintDark, hintLight)
	}
	if m.themeUndoAvailable() {
		hints = append(hints, hintUndo)
	}
	return append(hints, hintBack)
}

// utilitiesJobsCard is the reference's left card: the section's own rows, two
// lines each, windowed around the cursor when the frame cannot hold them all. The
// notes above and below name what the window hid, so a long list degrades out
// loud instead of ending where the screen happens to end.
func (m Model) utilitiesJobsCard(width, budget int) []string {
	rows := m.utilitiesRows()
	content := cardContentWidth(width)
	room := max(1, budget-3) // the card's borders and its rule

	// Each row costs its height; the window is measured in lines, not entries.
	heights := make([]int, len(rows))
	for i, row := range rows {
		if row.Divider || row.Subtitle == "" {
			heights[i] = 1
		} else {
			heights[i] = 2
		}
	}
	start, end := utilitiesRowWindow(heights, m.Cursor, room, false)
	// A windowed list spends one line at each end on its count.
	if start > 0 {
		room--
	}
	if end < len(rows) {
		room--
	}
	start, end = utilitiesRowWindow(heights, m.Cursor, room, false)

	var body []string
	if start > 0 {
		body = append(body, MutedStyle.Render(fmt.Sprintf("↑ %d more above", start)))
	}
	for i := start; i < end; i++ {
		row := rows[i]
		if row.Divider {
			body = append(body, rule(content))
			continue
		}
		body = append(body, cardRowLines(cardRow{
			Icon:     row.Icon,
			Title:    row.Title,
			Subtitle: row.Subtitle,
			Chevron:  true,
		}, content, i == m.Cursor)...)
	}
	if end < len(rows) {
		body = append(body, MutedStyle.Render(fmt.Sprintf("↓ %d more below", len(rows)-end)))
	}

	if len(body) > max(1, budget-3) {
		body = body[:max(1, budget-3)]
	}
	return card("System Utilities", "Maintenance, diagnostics and configuration tools for your environment.", width, body)
}

// utilitiesRightColumn is the reference's right column: the theme card, the
// machine's facts, the live resources and the recommended actions, each separated
// by one blank row so the cards read as separate objects.
func (m Model) utilitiesRightColumn(width, budget int) []string {
	var blocks [][]string
	blocks = append(blocks, card("Theme and System Styling", "Personalise your environment", width,
		m.utilitiesThemeCardBody(cardContentWidth(width))))

	if facts := m.utilitiesOverviewFactsFor(cardContentWidth(width)); len(facts) > 0 {
		blocks = append(blocks, card("System Overview", "Information about this machine", width, facts))
	}
	if resources := m.utilitiesResourcesCardBody(cardContentWidth(width)); len(resources) > 0 {
		blocks = append(blocks, card("Resources", "Live system resource usage", width, resources))
	}
	if steps := m.utilitiesRecommendedSteps(); len(steps) > 0 {
		blocks = append(blocks, card("Recommended next actions", "Make the most of your environment", width,
			numberedStepLines(steps, cardContentWidth(width))))
	}
	return fitColumnBlocks(blocks, budget)
}

// utilitiesThemeCardBody is the theme card: what the switch does when it is
// available, and the amber badge with the reason when it is not. The reason is
// the section's own sentence, not a second copy typed here.
func (m Model) utilitiesThemeCardBody(width int) []string {
	body := []string{InkStyle.Render("Recolour the dotfiles installer and the system using your preferred theme.")}

	if !m.ThemeSwitchFound {
		body = append(body, "")
		body = append(body, warningBadge("Not available on this host", width)...)
		// The section's own reason is the sentence, so the card cannot say a
		// different why from the compact screen above the list.
		reason := ""
		for _, candidate := range m.utilitiesReasons() {
			if strings.Contains(candidate, "desktop theme switch") || strings.Contains(candidate, "No desktop") {
				reason = candidate
				break
			}
		}
		if reason == "" {
			reason = "Desktop theme switching is not available in this environment because there is no GNOME, KDE or macOS session running."
		}
		reason += " This installer is currently running in a headless/terminal (WSL) environment."
		body = append(body, "")
		for _, line := range wrapText(reason, width, 0) {
			body = append(body, MutedStyle.Render(line))
		}
		return body
	}

	body = append(body, "")
	body = append(body, SuccessStyle.Render("✓ Available on this host"))
	if m.ThemeSwitch.Name != "" {
		body = append(body, InkStyle.Render("The switch runs through "+m.ThemeSwitch.Name+"'s own tool."))
	}
	// The section's own reason names what the switch writes and how it is undone;
	// it is the same source the compact card draws above the list.
	for _, reason := range m.utilitiesReasons() {
		if strings.Contains(reason, "desktop theme switch") {
			for _, line := range wrapText(reason, width, 0) {
				body = append(body, MutedStyle.Render(line))
			}
			break
		}
	}
	return body
}

// utilitiesOverviewFactsFor is the System Overview card's body: the machine's
// own facts with their icons and their values at one shared column. The compact
// panel's facts are reused, so the two shapes cannot disagree about a machine.
func (m Model) utilitiesOverviewFactsFor(width int) []string {
	info := m.SystemInfo
	if info == nil {
		return nil
	}
	facts := []iconFact{}
	if selected, ok := m.utilitiesSelectedRow(); ok {
		value := selected.Title
		if selected.Subtitle != "" {
			value += " — " + selected.Subtitle
		}
		facts = append(facts, iconFact{Icon: grammarMarker, Label: "Selected", Value: value})
	}
	facts = append(facts,
		iconFact{Icon: dashboardOSIcon, Label: "OS", Value: info.OSName},
		iconFact{Icon: "\uf109", Label: "Host", Value: m.platformHost()},
		iconFact{Icon: dashboardArchIcon, Label: "Arch", Value: archName(info)},
		iconFact{Icon: dashboardShellIcon, Label: "Shell", Value: info.UserShell},
		iconFact{Icon: dashboardPackagesIcon, Label: "Packages", Value: m.packageManager()},
		iconFact{Icon: "\uf015", Label: "Home", Value: info.HomeDir},
	)
	return iconFactLines(facts, width)
}

// utilitiesResourcesCardBody is the Resources card: one bar and its own reading
// per sampled resource. A resource appears only where its own flag says the
// sample reported it, and the card is absent while the sampler is gated off -- a
// stale reading drawn as a current one is the lie the gate exists to prevent.
func (m Model) utilitiesResourcesCardBody(width int) []string {
	if !m.Animating || len(m.Metrics) == 0 {
		return nil
	}
	last := m.Metrics[len(m.Metrics)-1]
	if !metricsHaveAny(last) {
		return nil
	}

	var rows []string
	if last.CPUBusy >= 0 && last.CPUOK {
		value := fmt.Sprintf("%.0f%%", last.CPUBusy*100)
		rows = append(rows, resourceBarRow("CPU", last.CPUBusy, value, width))
	}
	if last.MemOK && last.MemTotal > 0 {
		used := min(last.MemUsed, last.MemTotal)
		fraction := float64(used) / float64(last.MemTotal)
		value := fmt.Sprintf("%s / %s  %s", humanBytes(used), humanBytes(last.MemTotal), resourcePercent(fraction))
		rows = append(rows, resourceBarRow("Memory", fraction, value, width))
	}
	if last.DiskOK && last.DiskTotal > 0 {
		used := last.DiskTotal - min(last.DiskFree, last.DiskTotal)
		fraction := float64(used) / float64(last.DiskTotal)
		value := fmt.Sprintf("%s / %s  %s", humanBytes(used), humanBytes(last.DiskTotal), resourcePercent(fraction))
		rows = append(rows, resourceBarRow("Disk", fraction, value, width))
	}
	return rows
}

// utilitiesRecommendedSteps is the recommended actions card: three numbered
// actions chosen from the jobs the section is really offering, so a suggestion
// cannot name a row that is not there. Each title is the mockup's sentence for
// that job, and the order is the order a reader benefits from.
func (m Model) utilitiesRecommendedSteps() []numberedStep {
	offered := func(label string) bool {
		for _, option := range m.GetCurrentOptions() {
			if option == utilitiesScreenRow(label) {
				return true
			}
		}
		return false
	}

	var steps []numberedStep
	if offered(utilitiesTerminalRow) {
		steps = append(steps, numberedStep{Title: "Inspect terminal capabilities to see what features are supported."})
	}
	if offered(utilitiesUpdatesRow) {
		steps = append(steps, numberedStep{Title: "Check installed tools and verify your setup."})
	}
	if offered(utilitiesDoctorRow) {
		steps = append(steps, numberedStep{Title: "Continue to diagnostics if you're troubleshooting issues."})
	}
	if len(steps) < 3 && offered(utilitiesShellAuditRow) {
		steps = append(steps, numberedStep{Title: "Measure the shell's startup to see where the time goes."})
	}
	if len(steps) > 3 {
		steps = steps[:3]
	}
	return steps
}
