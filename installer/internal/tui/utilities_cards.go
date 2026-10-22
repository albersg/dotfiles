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
// cards: the theme row and the state of the picker it opens, the machine's facts,
// the live resources and the recommended next actions. The left column ends with
// the creature.
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

// utilitiesThemeCardBody is the theme card: what the theme row does, and the state
// of the action it opens -- the theme list read from the repository and the change
// the record names that can be put back. Both facts are the model's, and the card
// states neither an availability nor a state it does not hold.
//
// The card used to carry an amber "Not available on this host" badge and a
// paragraph about GNOME, KDE and macOS, because no DESKTOP session was detected
// under WSL. That badge described a different switch from the row it sat beside:
// the row opens the installer's own theme picker, which works whether or not a
// desktop is running, so the card denied the action drawn directly above it. The
// desktop switch -- its rows, its keys and its own sentence in the section's
// reasons -- is unchanged; it just no longer speaks for the installer's theme.
func (m Model) utilitiesThemeCardBody(width int) []string {
	body := []string{InkStyle.Render("Recolour the dotfiles installer and the system using your preferred theme.")}
	body = append(body, "")
	for _, line := range wrapText(m.utilitiesThemeState(), width, 0) {
		body = append(body, MutedStyle.Render(line))
	}
	// The desktop switch is a different switch from the row above, and the section
	// still owes the reader why its rows are absent. The reason is the section's own
	// sentence -- never a second copy typed here -- and it names the desktop, so it
	// cannot be read as a verdict on the installer's own themes.
	if note := m.utilitiesDesktopSwitchNote(); note != "" {
		body = append(body, "")
		for _, line := range wrapText(note, width, 0) {
			body = append(body, MutedStyle.Render(line))
		}
	}
	return body
}

// utilitiesDesktopSwitchNote is what the card says about the desktop switch this
// host does not have: the section's own reason, or nothing when a desktop was
// detected. It is a note and not a warning badge, because nothing on this screen is
// broken: the installer's theme picker works either way.
func (m Model) utilitiesDesktopSwitchNote() string {
	if m.ThemeSwitchFound {
		return ""
	}
	for _, reason := range m.utilitiesReasons() {
		if strings.Contains(reason, "desktop theme switch") {
			return reason
		}
	}
	return ""
}

// utilitiesThemeState is the theme card's one sentence of state. Every case is a
// fact the model holds: how many themes were read, which one the installer wears,
// or -- when the list cannot be read -- the reason the section itself states, so
// the card and the section cannot say two different things about one read.
func (m Model) utilitiesThemeState() string {
	themes := m.dotfilesThemeOptions()
	switch {
	case len(themes) > 0 && m.DotfilesThemeRecord != nil:
		return fmt.Sprintf("%s are read from the repository, and the installer is wearing %s.",
			themeCountLabel(len(themes)), m.DotfilesThemeRecord.Theme)
	case len(themes) > 0:
		return fmt.Sprintf("%s are read from the repository; the picker shows each one on the interface before it is applied.",
			themeCountLabel(len(themes)))
	case m.DotfilesThemeRecord != nil:
		return fmt.Sprintf("No theme can be applied here, so this row offers the change the record names (%s) to put back.",
			m.DotfilesThemeRecord.Theme)
	case m.dotfilesThemesPending():
		return "Reading the dotfiles' theme definitions from the repository checkout…"
	default:
		reason := m.DotfilesThemesErr
		if reason == "" {
			reason = themeDefinitionsNotFoundMessage()
		}
		return "The dotfiles' own theme definitions cannot be read here: " + reason
	}
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
