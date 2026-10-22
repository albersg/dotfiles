package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// ============================================================================
// SHELL STARTUP — the reference's fourth screen
// ============================================================================
//
// The reference draws the shell startup as a narrow column of art, then the
// explanation card with numbered questions, the action row (the [s] measure in
// the brand role beside the four plan options), a results card with stat boxes and the per-run
// bars, a recent-runs table, and the methodology. The footer's middle line says
// what the run is measuring.
//
// The wide shape is the reference's. The 80x24 floor keeps the section renderer,
// where the same rows, the same keys and the same numbers are drawn in one
// column. Nothing here invents a duration: the stat boxes read the model's own
// median, fastest, last sample and stored previous median; the bars are the
// samples' own relation to the slowest one; and a value the model does not hold
// gets no box rather than a zero.

// shellStartupArt is the screen's clock. It is one ring of blocks and two hands
// meeting on the centre -- a vertical hand up and a horizontal hand to the right.
// The art it replaces was a bent outline whose hand sat one column off the centre
// of its own face, so the rows read as broken pixels rather than as a clock. Every
// row here is the same width and the ring is symmetric about the centre column;
// shell_audit_test.go holds both.
const shellStartupArt = `     ▄▄▄▄▄▄▄     
   ▄█▀▀▀▀▀▀▀█▄   
  ██         ██  
 ██     █     ██ 
 ██     ███   ██ 
  ██         ██  
   ▀█▄     ▄█▀   
     ▀▀▀▀▀▀▀     `

// shellAuditWide reports whether the reference's two-column shell screen fits.
func (m Model) shellAuditWide(l layout, budget int) bool {
	if !l.TwoColumn || l.Right < 78 {
		return false
	}
	return budget >= 32
}

// shellAuditWideHints is the same key list the compact screen offers, so a press
// does not disappear when the frame widens.
func (m Model) shellAuditWideHints() []installerHint {
	back := hintBack
	if m.ShellAudit.Measuring {
		back = hintShellAuditStop
	}
	return []installerHint{hintUp, hintDown, hintShellAuditMeasure, hintShellAuditPlan, hintShellAuditExtras, back}
}

// shellAuditArtColumn is the left column: the clock art and, below it, the
// shell's name, so the picture names the thing it is about.
func (m Model) shellAuditArtColumn(width int) []string {
	art := renderEmblem(shellStartupArt)
	for i, line := range art {
		art[i] = CenterHorizontally(line, width)
	}
	out := append([]string{}, art...)
	out = append(out, "")
	if name := m.ShellAudit.Shell; name != "" {
		out = append(out, CenterHorizontally(BrandStyle.Render(truncate(name, width)), width))
	}
	if m.ShellAudit.Measuring {
		out = append(out, CenterHorizontally(MutedStyle.Render(truncate("measuring…", width)), width))
	}
	return out
}

// renderShellAuditWide draws the reference's shell startup screen. The bands are
// stacked the way the reference stacks them: the art and the numbered explanation,
// the action row across the whole width (it carries five presses and does not fit a
// narrow column), the results beside the recent runs, and the methodology under
// both. The frame still owns the header, the status rule and the footer.
func (m Model) renderShellAuditWide(l layout, budget int) string {
	st := m.ShellAudit
	plan := clampShellAuditPlan(st.Plan)
	composition := l.Composition
	artWidth := min(24, max(16, composition/7))
	explanationWidth := composition - artWidth - layoutGutter

	var body []string
	art := m.shellAuditArtColumn(artWidth)
	explanation := card("The shell's startup",
		"Measure how long "+shellNameOr(st.Shell, "your shell")+" takes to start.", explanationWidth,
		numberedStepLines(m.shellAuditExplanationSteps(plan), cardContentWidth(explanationWidth)))
	body = append(body, sideBySide(art, artWidth, explanation, explanationWidth, layoutGutter)...)
	body = append(body, "")

	body = append(body, m.shellAuditActionBand(composition, plan)...)
	body = append(body, "")
	// The way back rides directly under the actions, so a frame that cannot hold
	// the methodology still shows it: the key is in the footer and this line names
	// where it goes.
	body = append(body, padRight(truncate(InkStyle.Render("← Back")+MutedStyle.Render("  [Esc] returns to Utilities"), composition), composition))

	resultsWidth := composition * 55 / 100
	runsWidth := composition - resultsWidth - layoutGutter
	results := card("Results", "startup performance", resultsWidth, m.shellAuditResultsBody(cardContentWidth(resultsWidth), st))
	runs := card("Recent runs", "Most recent measurements", runsWidth, m.shellAuditRecentRunsBody(cardContentWidth(runsWidth), st))
	body = append(body, sideBySide(results, resultsWidth, runs, runsWidth, layoutGutter)...)
	body = append(body, "")

	// The Methodology card takes the rows the bands above it left, so it is as tall
	// as the frame allows and never taller: the card's own borders and the rule under
	// its chip are the rows that are not body (cardChromeRows), and the body is cut
	// by its own row count with a declared total rather than by the frame's slice.
	methodRows := budget - len(body) - cardChromeRows
	body = append(body, card("Methodology", "How the measurement works", composition,
		m.shellAuditMethodologyBody(cardContentWidth(composition), methodRows, st))...)

	if len(body) > budget {
		body = body[:budget]
	}
	// The bands are composed at the reading measure, which is narrower than a very
	// wide terminal: the body takes the frame's leading margin so the composition is
	// centred rather than hugging the left edge with the void on the right.
	return m.frameWithPanels(m.headerName(), "", centeredBody(body, l), m.shellAuditWideHints(), nil)
}

// shellAuditActionBand is the action row across the composition: the measure press
// and the four plan options. A frame too narrow for all five boxes keeps the
// measure press on one row and the plan on the next, and a frame too narrow for
// that states the same presses as rows.
func (m Model) shellAuditActionBand(width int, plan shellAuditPlan) []string {
	primary := actionSpec{Icon: "▶", Title: shellAuditMeasureRow}
	planActions := m.shellAuditPlanActions(plan)
	if actions := actionButtonRow(primary, planActions, width); len(actions) > 0 {
		return actions
	}
	if alone := actionButtonRow(primary, nil, width); len(alone) > 0 {
		out := append([]string{}, alone...)
		if rest := actionButtonRowAll(planActions, width); len(rest) > 0 {
			return append(out, rest...)
		}
		return append(out, m.shellAuditPlanRows(plan, width)...)
	}
	return m.shellAuditPlanRows(plan, width)
}

// shellNameOr names the shell in a sentence, with a neutral phrase when the model
// has not resolved one.
func shellNameOr(name, fallback string) string {
	if strings.TrimSpace(name) == "" {
		return fallback
	}
	return name
}

// shellAuditExplanationSteps is the reference's numbered explanation, built from
// the plan the next press would really run: what is measured, how many starts,
// and which keys control it.
func (m Model) shellAuditExplanationSteps(plan shellAuditPlan) []numberedStep {
	st := m.ShellAudit
	command := shellAuditPlanCommand(st.Shell, st.Kind, plan.Mode)
	timeout := shellAuditDuration(st.Timeout)
	if st.Timeout <= 0 {
		timeout = "its own bound"
	}
	return []numberedStep{
		{
			Title: "What is measured?",
			Subtitle: fmt.Sprintf("The time it takes for %s to start, including your configuration: `%s`.",
				shellNameOr(st.Shell, "the shell"), command),
		},
		{
			Title: "How many runs?",
			Subtitle: fmt.Sprintf("%d starts are performed (each bounded by %s). The median is used as the result.",
				plan.Samples, timeout),
		},
		{
			Title:    "Controls?",
			Subtitle: "Press [s] to start, or adjust the options below. Use [h] to keep the run history.",
		},
	}
}

// shellAuditPlanActions is the action row's four secondary boxes: the start, the
// number of runs, the warm-up and the clean comparison, each carrying the key it
// really runs on and the value the plan really holds.
func (m Model) shellAuditPlanActions(plan shellAuditPlan) []actionSpec {
	warmup := "Off"
	if plan.Warmup > 0 {
		warmup = fmt.Sprintf("%d", plan.Warmup)
	}
	comparison := "Off"
	if plan.Baseline {
		comparison = "On"
	}
	return []actionSpec{
		{Key: "m", Title: shellAuditModeTitle(plan.Mode), Subtitle: plan.Mode.label(), Icon: "\uf013"},
		{Key: "n", Title: fmt.Sprintf("Runs: %d", plan.Samples), Subtitle: "Number of measurements", Icon: "#"},
		{Key: "w", Title: "Warm-up: " + warmup, Subtitle: "Discard initial runs", Icon: "\uf06d"},
		{Key: "b", Title: "Clean comparison", Subtitle: comparison, Icon: "\uf1c0"},
	}
}

// shellAuditModeTitle is the short name of a start for the action box.
func shellAuditModeTitle(mode shellAuditMode) string {
	switch {
	case mode.Clean:
		return "Clean"
	case mode.Login:
		return "Login"
	case mode.Interactive:
		return "Interactive"
	default:
		return "Non-interactive"
	}
}

// shellAuditPlanRows is the fallback for a frame too narrow for the action boxes:
// the same four settings as rows, so no press is lost.
func (m Model) shellAuditPlanRows(plan shellAuditPlan, width int) []string {
	out := []string{InkStyle.Render(shellAuditMeasureRow)}
	for _, action := range m.shellAuditPlanActions(plan) {
		out = append(out, cardRowLines(cardRow{
			Icon:     action.Icon,
			Title:    "[" + action.Key + "] " + action.Title,
			Subtitle: action.Subtitle,
		}, cardContentWidth(width), false)...)
	}
	return out
}

// shellAuditResultsBody is the Results card: the four stat boxes the model can
// fill (current median, the session's best, the stored previous median and the
// last completed start), then the run durations with their bars and the summary
// statistics.
func (m Model) shellAuditResultsBody(width int, st shellAuditState) []string {
	boxes := m.shellAuditStatBoxes(st)
	body := statBoxLines(boxes, width)
	if len(body) == 0 {
		for _, box := range boxes {
			body = append(body, InkStyle.Render(box.Label+": "+box.Value))
		}
	}
	body = append(body, "")
	// The same summary rows the compact screen prints: the median with its count,
	// the range, the starts that did not finish and the instrument's first entry.
	// They are the data the guard requires at every size, and the stat boxes above
	// are the reference's at-a-glance shape of the same numbers.
	for _, row := range shellAuditSummaryRows(st) {
		body = append(body, InkStyle.Render(truncate(row, width)))
	}
	body = append(body, "")
	body = append(body, m.shellAuditDurationRows(st, width)...)
	return body
}

// shellAuditStatBoxes is the four stat boxes: every value is a number the state
// holds. The previous median comes from the newest stored measurement that is not
// this one, and the box is absent when the history holds nothing to compare.
func (m Model) shellAuditStatBoxes(st shellAuditState) []statBox {
	var boxes []statBox
	if st.Median > 0 {
		boxes = append(boxes, statBox{Label: "Current median", Value: shellAuditDuration(st.Median), Tone: BrandStyle})
	}
	if st.Fastest > 0 {
		boxes = append(boxes, statBox{Label: "Best (session)", Value: shellAuditDuration(st.Fastest), Tone: InkStyle})
	}
	if previous, ok := st.History.previousMedian(); ok {
		boxes = append(boxes, statBox{Label: "Previous median", Value: shellAuditDuration(previous), Tone: InkStyle})
	}
	if len(st.Samples) > 0 {
		last := st.Samples[len(st.Samples)-1]
		boxes = append(boxes, statBox{Label: "Last run", Value: "✓ " + shellAuditDuration(last), Tone: SuccessStyle})
	}
	return boxes
}

// previousMedian is the newest stored measurement's median that is older than the
// measurement on screen. A history that is off, empty, or only holds this run has
// no previous median to state.
func (h shellAuditHistory) previousMedian() (time.Duration, bool) {
	for _, entry := range h.Entries {
		if entry.Median > 0 && !entry.Cancelled {
			return entry.Median, true
		}
	}
	return 0, false
}

// shellAuditDurationRows is the per-run list: each completed start's own
// duration, its bar, and the summary values beside it. The bar's length is the
// sample's share of the slowest start, a relation between two real numbers; no
// time is invented and a start that did not finish is not given a duration.
func (m Model) shellAuditDurationRows(st shellAuditState, width int) []string {
	if len(st.Samples) == 0 {
		if st.Measuring {
			return []string{MutedStyle.Render("Waiting for the first start to finish…")}
		}
		return []string{MutedStyle.Render("No start has finished yet.")}
	}

	slowest := st.Slowest
	if slowest <= 0 {
		slowest = st.Samples[0]
		for _, sample := range st.Samples {
			if sample > slowest {
				slowest = sample
			}
		}
	}

	barCells := max(6, min(18, width/4))
	out := []string{MutedStyle.Render("Run durations (ms)")}
	for i, sample := range st.Samples {
		fraction := 0.0
		if slowest > 0 {
			fraction = float64(sample) / float64(slowest)
		}
		filled := int(fraction*float64(barCells) + 0.5)
		filled = min(max(filled, 1), barCells)
		bar := MeterFilledStyle.Render(strings.Repeat("█", filled)) +
			MeterStyle.Render(strings.Repeat("░", barCells-filled))
		line := fmt.Sprintf("%2d  %6s  %s", i+1, shellAuditDuration(sample), bar)
		if i < 4 {
			line += "   " + m.shellAuditSummaryCell(i, st)
		}
		out = append(out, InkStyle.Render(truncate(line, width)))
	}
	if stats := m.shellAuditStatsLine(st); stats != "" {
		out = append(out, MutedStyle.Render(truncate(stats, width)))
	}
	return out
}

// shellAuditSummaryCell is the right-hand column of the durations card: one of
// min, max, average and median per row.
func (m Model) shellAuditSummaryCell(index int, st shellAuditState) string {
	stats := st.Stats
	if stats.Count == 0 {
		return ""
	}
	switch index {
	case 0:
		return fmt.Sprintf("Min     %s", shellAuditDuration(st.Fastest))
	case 1:
		return fmt.Sprintf("Max     %s", shellAuditDuration(st.Slowest))
	case 2:
		return fmt.Sprintf("Average %s", shellAuditDuration(stats.Mean))
	case 3:
		return fmt.Sprintf("Median  %s", shellAuditDuration(st.Median))
	}
	return ""
}

// shellAuditStatsLine is the one-line summary under the bars, for a card too
// narrow to place the values in a column.
func (m Model) shellAuditStatsLine(st shellAuditState) string {
	stats := st.Stats
	if stats.Count == 0 {
		return ""
	}
	return fmt.Sprintf("Min %s · Max %s · Average %s · Median %s",
		shellAuditDuration(st.Fastest), shellAuditDuration(st.Slowest),
		shellAuditDuration(stats.Mean), shellAuditDuration(st.Median))
}

// shellAuditRecentRunsBody is the recent runs table: the session's own starts,
// newest first, each with the status the state really gives it and a note naming
// which start of plan it was. A timeout or a failure is a row of its own with no
// duration, because the state keeps those apart from the samples.
func (m Model) shellAuditRecentRunsBody(width int, st shellAuditState) []string {
	total := len(st.Samples) + st.Timeouts + st.Failures
	if total == 0 {
		return []string{MutedStyle.Render("No measurement has finished yet.")}
	}

	const (
		indexWidth  = 3
		durWidth    = 8
		statusWidth = 11
	)
	noteWidth := max(1, width-indexWidth-durWidth-statusWidth-3)

	out := []string{
		TableHeaderStyle.Render(padRight("#", indexWidth)) + " " +
			TableHeaderStyle.Render(padRight("Duration", durWidth)) + " " +
			TableHeaderStyle.Render(padRight("Status", statusWidth)) + " " +
			TableHeaderStyle.Render("Notes"),
	}

	// The samples are the completed starts; they are listed newest first.
	for i := len(st.Samples) - 1; i >= 0; i-- {
		sample := st.Samples[i]
		note := fmt.Sprintf("start %d of %d", i+1, max(len(st.Samples), st.Runs))
		line := MutedStyle.Render(padRight(fmt.Sprintf("%d", i+1), indexWidth)) + " " +
			InkStyle.Render(padRight(shellAuditDuration(sample), durWidth)) + " " +
			SuccessStyle.Render(padRight("✓ completed", statusWidth)) + " " +
			MutedStyle.Render(truncate(note, noteWidth))
		out = append(out, truncate(line, width))
	}
	for i := 0; i < st.Timeouts; i++ {
		line := MutedStyle.Render(padRight("-", indexWidth)) + " " +
			MutedStyle.Render(padRight("—", durWidth)) + " " +
			WarningStyle.Render(padRight("timed out", statusWidth)) + " " +
			MutedStyle.Render(truncate(fmt.Sprintf("bounded by %s", shellAuditDuration(st.Timeout)), noteWidth))
		out = append(out, truncate(line, width))
	}
	for i := 0; i < st.Failures; i++ {
		line := MutedStyle.Render(padRight("-", indexWidth)) + " " +
			MutedStyle.Render(padRight("—", durWidth)) + " " +
			ErrorStyle.Render(padRight("failed", statusWidth)) + " " +
			MutedStyle.Render(truncate("could not be started", noteWidth))
		out = append(out, truncate(line, width))
	}
	return out
}

// shellAuditMethodologyBody is the Methodology card: the method the number comes
// from and the attribution the instrument returned, drawn from the same paragraphs
// the compact body draws -- the plan, what zprof (or bash, or fish) blamed, the
// starts that support the median, and the promise that nothing was changed. It is
// the same words in both shapes for the same reason the plan paragraph is a
// function: one measurement, one explanation. The recommendations come first when
// the user opened them, so a short card cuts the method rather than the answer they
// asked for, exactly as the compact body orders the two.
//
// The card carries the method and the attribution, not the compact body's whole
// prose: the wizard's plan, the statistics, the history and the comparison are the
// floor's shape, and the results and recent-runs cards beside this one already draw
// the numbers those paragraphs describe. What the card does carry, it carries whole
// -- the attribution's own rows, the count of rows the instrument reported and this
// screen does not name, and the promise -- in blocks, so the declared cut is a
// sentence of its own rather than a clause a wrap can split.
//
// The card is budgeted by the rows the bands above it left, and a card taller than
// that budget ends with the count of the lines it could not draw -- the rule the
// description and the notice follow -- rather than losing the attribution silently.
// The paragraphs are ordered so the cut falls on the last line of the promise: the
// method, the instrument's own rows and the declared number of rows it reported all
// come before it.
func (m Model) shellAuditMethodologyBody(width, rows int, st shellAuditState) []string {
	if rows < 1 {
		return nil
	}

	var paragraphs []string
	if st.AdviceOpen {
		paragraphs = append(paragraphs, shellAuditAdviceParagraphs(shellAuditReportOf(st))...)
	}

	paragraphs = append(paragraphs, shellAuditPlanParagraph(st))
	if len(st.Attribution) == 0 {
		if st.AttributionReason != "" {
			paragraphs = append(paragraphs, st.AttributionReason)
		}
	} else {
		// The attribution is drawn in its own blocks -- the instrument's rows, the
		// count of rows it reported and this screen does not name, and what the
		// numbers mean -- so the declared cut starts on its own line instead of
		// falling half-way through a wrapped clause.
		paragraphs = append(paragraphs, shellAuditAttributionTable(st))
		if cut := shellAuditAttributionCut(st); cut != "" {
			paragraphs = append(paragraphs, cut)
		}
		paragraphs = append(paragraphs, shellAuditAttributionClosing(st.Kind))
	}
	if runList := shellAuditRunList(st.Samples); runList != "" {
		paragraphs = append(paragraphs,
			shellAuditRunListLead+runList)
	}
	paragraphs = append(paragraphs, shellAuditReadOnlyPromise)

	var lines []string
	for _, paragraph := range paragraphs {
		if paragraph == "" {
			continue
		}
		for i, line := range wrapText(paragraph, max(1, width-2), 0) {
			if i == 0 {
				lines = append(lines, AccentKeyStyle.Render("• ")+InkStyle.Render(line))
				continue
			}
			lines = append(lines, "  "+InkStyle.Render(line))
		}
	}
	if len(lines) > rows {
		hidden := len(lines) - rows + 1
		lines = append(lines[:rows-1], MutedStyle.Render(fmt.Sprintf("… and %d more", hidden)))
	}
	return lines
}

// shellAuditFooterStatus is the footer's middle line: what the run is measuring,
// or the median and its count once a measurement is on screen. It is the same
// notice the compact screen prints in its notice band, so the two shapes say the
// same thing.
func (m Model) shellAuditFooterStatus() string {
	st := m.ShellAudit
	if st.Measuring {
		return m.shellAuditNotice()
	}
	if st.Median > 0 {
		return fmt.Sprintf("Median %s over %d of %d runs · fastest %s",
			shellAuditDuration(st.Median), len(st.Samples), st.Runs, shellAuditDuration(st.Fastest))
	}
	return ""
}

// sideBySide places two card blocks beside each other, each padded to its own
// width, so the two borders line up on every row.
func sideBySide(left []string, leftWidth int, right []string, rightWidth, gap int) []string {
	rows := max(len(left), len(right))
	out := make([]string, rows)
	spaces := strings.Repeat(" ", gap)
	for i := 0; i < rows; i++ {
		l := ""
		if i < len(left) {
			l = left[i]
		}
		r := ""
		if i < len(right) {
			r = right[i]
		}
		out[i] = padRight(lipgloss.NewStyle().MaxWidth(leftWidth).Render(l), leftWidth) +
			spaces + lipgloss.NewStyle().MaxWidth(rightWidth).Render(r)
	}
	return out
}
