package tui

import (
	"fmt"
	"strconv"
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
	body = append(body, m.shellAuditMethodologyCard(composition, methodRows)...)

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

// shellAuditMethodologyCard is the Methodology card: the screen's detail areas the
// cards beside it do not already draw (shellAuditMethodologyGroups), set off by the
// card's own dividers. The card is budgeted by the rows the bands above it left, and
// a detail taller than that budget is cut by clipCardGroups with the frame's own
// marker rather than losing the attribution silently.
//
// The order is the screen's own: the recommendations come first when the user
// opened them, so a short card cuts the method rather than the answer they asked
// for, exactly as the compact body orders the two.
func (m Model) shellAuditMethodologyCard(width, rows int) []string {
	if rows < 1 {
		return nil
	}
	content := cardContentWidth(width)
	groups := m.shellAuditMethodologyGroups(content)
	if len(groups) == 0 {
		return nil
	}

	// The areas are drawn whole, in order, until the rows run out; the area that
	// crosses the budget is drawn as far as its own rows go; and every row the card
	// could not draw -- the rest of that area and all of the areas after it -- is
	// declared in ONE count. Two markers would be a card telling a reader to look in
	// two places for what it kept back, and a divider the card never draws is not a
	// row it kept back either.
	limit := rows - 1 // the marker's own row
	spent := 0
	drawn := make([]cardGroup, 0, len(groups))
	hidden := 0
	for i, group := range groups {
		cost := len(group.Lines)
		if len(drawn) > 0 {
			cost++ // the divider above this area
		}
		if spent+cost <= limit {
			drawn = append(drawn, group)
			spent += cost
			continue
		}
		room := limit - spent
		if len(drawn) > 0 {
			room-- // this area's divider is drawn only if a row of it is
		}
		if room > 0 {
			drawn = append(drawn, cardGroup{Title: group.Title, Lines: group.Lines[:room]})
			hidden = len(group.Lines) - room
		} else {
			hidden = len(group.Lines)
		}
		for _, rest := range groups[i+1:] {
			hidden += len(rest.Lines)
		}
		break
	}

	if hidden > 0 {
		marker := MutedStyle.Render(fmt.Sprintf("… and %d more", hidden))
		if len(drawn) == 0 {
			drawn = append(drawn, cardGroup{Lines: []string{marker}})
		} else {
			last := &drawn[len(drawn)-1]
			last.Lines = append(last.Lines, marker)
		}
	}
	return cardGroups("Methodology", "How the measurement works", width, drawn)
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

// ============================================================================
// THE SHELL STARTUP'S DETAIL — the method, the numbers and the promise as rows
// ============================================================================
//
// The detail of this screen used to be a list of paragraphs: the method, the plan
// and its keys, the statistics, the history, the context, the attribution, the
// comparison, the trend, the recommendations, the runs and the promise, each one
// a sentence the reader had to parse for the value inside it. At 80x24 that was
// 51 paragraphs behind a four-line band, and the band printed the first one and
// "… and 50 more".
//
// This section is the same material as facts_cards.go's rows: an []factArea whose
// areas are the card's dividers (shellAuditMethodologyGroups) and whose flatten is
// the compact body's prose (shellAuditDescription), so the floor and the wide
// frame read one list. Every row is the label that names the fact and the value
// the state holds; the only rows whose value is a sentence are the model's own --
// a refusal reason, a note, the argument behind a recommendation -- and those are
// the ones that cannot be shortened without inventing a claim (see
// factArgumentLabels).
//
// Nothing here is a second copy of the state. The plan is the plan the next press
// would really run (clampShellAuditPlan, shellAuditPlanCommand), the statistics are
// the state's own summary, and the attribution is the instrument's own rows in the
// instrument's own units, so no row can disagree with the number beside it.

// shellAuditAreas is the screen's detail divided into the areas its card sets off
// with a divider. The order is the order the reader needs them in, and it is the
// order the compact body prints: what the press will do, what the finished runs
// support, what was measured, what the shell's instrument named, the two
// comparisons, the recommendations, the starts themselves and the promise.
func (m Model) shellAuditAreas() []factArea {
	st := m.ShellAudit
	if reason := m.shellAuditUnavailableReason(); reason != "" {
		// The screen's verdict is one sentence and there is no datum behind it: the
		// reason the row that measures is not offered is the whole answer, and a
		// label in front of it would say "Reason" twice.
		return []factArea{{Name: "Why nothing can be measured", Facts: []iconFact{{Value: reason}}}}
	}

	plan := clampShellAuditPlan(st.Plan)
	report := shellAuditReportOf(st)

	var areas []factArea
	if st.AdviceOpen {
		// The recommendations come first when the user asked for them: they are the
		// part of this screen a reader comes back for, and the frame cuts from the
		// bottom.
		if area, ok := shellAuditAdviceArea(report); ok {
			areas = append(areas, area)
		}
	}

	areas = append(areas,
		factArea{Name: "The method", Facts: shellAuditMethodFacts(st, plan)},
		factArea{Name: "The plan", Facts: shellAuditPlanFacts(st, plan)},
	)
	if st.Measured && st.Stats.Count > 0 {
		areas = append(areas, factArea{Name: "What the runs support", Facts: shellAuditStatisticFacts(st)})
	}

	if !st.Measured {
		areas = append(areas, shellAuditNotMeasuredArea(st))
	} else {
		areas = append(areas,
			factArea{Name: "The measurement", Facts: shellAuditMeasurementFacts(st)},
			factArea{Name: "The attribution", Facts: shellAuditAttributionFacts(st)},
		)
		if area, ok := shellAuditComparisonArea(st, report); ok {
			areas = append(areas, area)
		}
		if area, ok := shellAuditTrendArea(st, report); ok {
			areas = append(areas, area)
		}
	}

	if !st.AdviceOpen {
		if area, ok := shellAuditAdviceArea(report); ok {
			areas = append(areas, area)
		}
	}
	if st.Measured {
		if area, ok := shellAuditRunArea(st); ok {
			areas = append(areas, area)
		}
	}

	return append(areas, factArea{Name: "What it will not do", Facts: shellAuditPromiseFacts()})
}

// shellAuditMethodFacts is the method the number comes from, as the rows a reader
// can check it against: the exact line, the bound every start ran under, the rule
// that the number is the middle start, what that number describes, and where the
// clean comparison is made. It is a function rather than a paragraph in the body
// because the wide frame's Methodology card draws the same rows.
func shellAuditMethodFacts(st shellAuditState, plan shellAuditPlan) []iconFact {
	timeout := shellAuditDuration(st.Timeout)
	if st.Timeout <= 0 {
		timeout = "its own bound"
	}
	return []iconFact{
		{Label: "The press", Value: "`" + shellAuditPlanCommand(st.Shell, st.Kind, plan.Mode) + "`"},
		{Label: "Each start", Value: "bounded by " + timeout},
		{Label: "The number", Value: "the median: the middle start, not an average and not a single run"},
		{Label: "It describes", Value: "the start you wait through when you open a terminal"},
		{Label: "The comparison", Value: "the one measurement here that separates the shell's own startup from what you put in front of it"},
		{Label: "Where it runs", Value: "a directory of the utility's own: your files are read, never moved and never rewritten"},
	}
}

// shellAuditPlanFacts is the plan the next press would really run -- the start, how
// many starts, the warm-up, the clean comparison, the keys that change each of
// those, and where a kept history would live -- so the settings are rows instead of
// a sentence with four keys in the middle of it.
func shellAuditPlanFacts(st shellAuditState, plan shellAuditPlan) []iconFact {
	facts := []iconFact{
		{Label: "The start", Value: plan.Mode.label()},
		{Label: "Runs", Value: reviewPlural(plan.Samples, "start", "starts")},
		{Label: "Warm-up", Value: shellAuditWarmupPhrase(plan.Warmup)},
		{Label: "Clean comparison", Value: shellAuditOnOff(plan.Baseline,
			"the same start measured again without your configuration",
			"not measured: [b] turns it on")},
		{Label: "Keys", Value: "[m] the start, [n] the runs, [w] the warm-up, [b] the clean comparison"},
	}
	if st.History.Enabled {
		return append(facts,
			iconFact{Label: "History", Value: defaultShellAuditHistoryPath()},
			iconFact{Label: "It holds", Value: "a stamp, the counts, the median and the top of the profile, and nothing else about this machine"},
			iconFact{Label: "To stop", Value: "[h] stops keeping them; what is already stored is kept"},
		)
	}
	return append(facts,
		iconFact{Label: "History", Value: "off: nothing is written to this machine"},
		iconFact{Label: "To keep it", Value: "[h] stores the runs beside the installer's own record"},
	)
}

// shellAuditStatisticFacts is what the finished runs support beyond the median and
// the range, and what they do not: the mean, the spread and the reading it gives,
// the percentile and the trimmed median. The floors are the ones the statistics
// layer gates on, so a row and the number beside it cannot disagree, and a number
// the count cannot support is named as missing rather than printed.
func shellAuditStatisticFacts(st shellAuditState) []iconFact {
	stats := st.Stats
	if stats.Count == 0 && len(st.Samples) > 0 {
		stats = shellAuditStatsOf(st.Samples)
	}
	if stats.Count == 0 {
		return nil
	}

	facts := []iconFact{
		{Label: "Behind these", Value: reviewPlural(stats.Count, "finished start", "finished starts")},
		{Label: "Mean", Value: shellAuditDuration(stats.Mean)},
	}
	if word, ratio, ok := stats.steadiness(); ok {
		facts = append(facts, iconFact{Label: "Spread", Value: fmt.Sprintf("%s (%.0f%% of the median): %s %s startup",
			shellAuditDuration(stats.StdDev), ratio*100, shellAuditArticle(word), word)})
	} else {
		facts = append(facts, iconFact{Label: "Spread", Value: fmt.Sprintf("no spread to read: it needs %s and this measurement has %d",
			reviewPlural(shellAuditSpreadFloor, "run", "runs"), stats.Count)})
	}
	if stats.P95Reported {
		facts = append(facts, iconFact{Label: "95th percentile", Value: shellAuditDuration(stats.P95)})
	} else {
		// The missing statistic is named without the words it would have been
		// printed with, so a reader scanning for a percentile does not find one that
		// was never taken: only what it would take to have one.
		facts = append(facts, iconFact{Label: "No percentile", Value: fmt.Sprintf("it is reported from %s and this measurement has %d",
			reviewPlural(shellAuditPercentileFloor, "run", "runs"), stats.Count)})
	}
	if stats.TrimmedMedianReported {
		facts = append(facts, iconFact{Label: "Trimmed median", Value: fmt.Sprintf("%s, with %s beyond %s times the interquartile range left out",
			shellAuditDuration(stats.TrimmedMedian), reviewPlural(len(stats.Outliers), "start", "starts"),
			strconv.FormatFloat(shellAuditOutlierFence, 'g', -1, 64))})
	}
	return facts
}

// shellAuditMeasurementFacts is the context of the number -- which shell, which
// start, which command, which terminal -- because a number whose context is missing
// is the number this utility exists not to print. A start that still includes
// something the plan asked to leave out says so in the model's own words.
func shellAuditMeasurementFacts(st shellAuditState) []iconFact {
	terminal := "none attached"
	if st.Terminal {
		terminal = shellNameOr(st.TerminalName, "attached")
	}
	facts := []iconFact{
		{Label: "Shell", Value: shellNameOr(st.Shell, "not named")},
	}
	if st.Version.Raw != "" {
		facts = append(facts, iconFact{Label: "Version", Value: st.Version.Raw})
	}
	facts = append(facts,
		iconFact{Label: "Start", Value: st.Mode.label()},
		iconFact{Label: "Command", Value: "`" + st.Command + "`"},
		iconFact{Label: "Terminal", Value: terminal},
	)
	if st.Note != "" {
		facts = append(facts, iconFact{Label: "Still included", Value: st.Note})
	}
	return facts
}

// shellAuditAttributionFacts is what the shell's own instrument named, one row per
// entry in the instrument's own order and units: the name, and the numbers the
// instrument printed for it. A share is only drawn where the instrument printed
// one, and the rows the screen does not name are counted rather than dropped
// silently.
func shellAuditAttributionFacts(st shellAuditState) []iconFact {
	if len(st.Attribution) == 0 {
		if st.AttributionReason == "" {
			return nil
		}
		return []iconFact{{Label: "Why nothing is named", Value: st.AttributionReason}}
	}

	facts := []iconFact{{Label: "Instrument", Value: shellAuditAttributionIntro(st)}}
	shown := min(len(st.Attribution), shellAuditNamedFunctions)
	for i, fn := range st.Attribution[:shown] {
		facts = append(facts, iconFact{
			Label: fmt.Sprintf("#%d", i+1),
			Value: fn.Name + " — " + shellAuditAttributionRow(st.Kind, fn),
		})
	}
	if cut := shellAuditAttributionCut(st); cut != "" {
		facts = append(facts, iconFact{Label: "Not named", Value: cut})
	}
	for _, fact := range shellAuditAttributionReadingFacts(st.Kind) {
		facts = append(facts, fact)
	}
	return append(facts, iconFact{
		Label: "The profiled start",
		Value: "a start of its own, so its numbers stand beside the median rather than inside it",
	})
}

// shellAuditComparisonArea is the clean comparison as rows: the two medians it is
// made of, the difference between them, how many runs each side is made of, and the
// reading the difference supports. The reading is kept apart from the arithmetic on
// purpose: the same number means "this is where your startup time goes" at sixty
// percent and "this is the shell's own startup" at two, and only one of those two
// readings is worth acting on.
func shellAuditComparisonArea(st shellAuditState, report shellAuditReport) (factArea, bool) {
	c := report.Comparison
	if c.Reason != "" {
		if !st.Plan.Baseline {
			// The comparison was never asked for: the plan says so, and a refusal for a
			// measurement nobody requested is not a fact about this run.
			return factArea{}, false
		}
		return factArea{Name: "The clean comparison", Facts: []iconFact{
			{Label: "Was asked for", Value: "and could not be made"},
			{Label: "Why", Value: c.Reason},
		}}, true
	}

	facts := []iconFact{
		{Label: "With your configuration", Value: shellAuditDuration(c.Configured)},
		{Label: "Without it", Value: shellAuditDuration(c.Clean)},
		{Label: "Median of", Value: reviewPlural(c.Samples, "run each", "runs each")},
	}
	if c.Overhead <= 0 {
		// A negative difference is not printed as a configuration that costs less
		// than nothing: the two runs measured the same start twice.
		facts = append(facts,
			iconFact{Label: "Your configuration", Value: fmt.Sprintf("nothing measurable: the clean start measured %s slower", shellAuditDuration(-c.Overhead))},
			iconFact{Label: "Reading it", Value: "the two starts are not separable here: a shell's own startup moves by that much between runs"},
		)
		return factArea{Name: "The clean comparison", Facts: facts}, true
	}

	facts = append(facts, iconFact{Label: "Your configuration", Value: fmt.Sprintf("%s, %.1f%% of the start", shellAuditDuration(c.Overhead), c.Share)})
	if c.Share < shellAuditComparisonSmallShare {
		facts = append(facts, iconFact{Label: "Reading it", Value: "almost all of this start is the shell itself: your configuration's share is inside the movement a startup has between runs"})
	} else {
		facts = append(facts, iconFact{Label: "Reading it", Value: "the part of the startup the utility measured as yours rather than the shell's, and where the profile is worth reading"})
	}
	return factArea{Name: "The clean comparison", Facts: facts}, true
}

// shellAuditTrendArea is where this measurement sits among the stored ones: the
// median before, the median now, the change between them, how many stored
// measurements it was read against and the date of the oldest one. It is drawn only
// where the history is on -- a trend read from a history the user turned off would
// be a comparison of nothing -- and a history that cannot be compared says why.
func shellAuditTrendArea(st shellAuditState, report shellAuditReport) (factArea, bool) {
	if !st.History.Enabled {
		return factArea{}, false
	}
	t := report.Trend
	if t.Kind == shellAuditTrendFirst || t.Kind == shellAuditTrendIncomparable {
		if t.Reason == "" {
			return factArea{}, false
		}
		return factArea{Name: "The trend", Facts: []iconFact{{Label: "No trend", Value: t.Reason}}}, true
	}

	change := fmt.Sprintf("%+.0f%% against %s", t.Share,
		reviewPlural(t.Runs, "stored measurement", "stored measurements"))
	if t.Kind != shellAuditTrendSteady {
		change = strings.TrimPrefix(change, "+")
	}
	facts := []iconFact{
		{Label: "Before", Value: shellAuditDuration(t.Before)},
		{Label: "Now", Value: shellAuditDuration(t.After)},
		{Label: "Change", Value: change},
	}
	if !t.Since.IsZero() {
		facts = append(facts, iconFact{Label: "Oldest", Value: t.Since.Format(shellAuditTrendStamp)})
	}
	switch t.Kind {
	case shellAuditTrendSteady:
		facts = append(facts, iconFact{Label: "Reading it", Value: "inside the movement a startup has between runs, so nothing changed here"})
	case shellAuditTrendRegression:
		facts = append(facts, iconFact{Label: "Reading it", Value: "this start got slower"})
	case shellAuditTrendImprovement:
		facts = append(facts, iconFact{Label: "Reading it", Value: "this start got faster"})
	}
	return factArea{Name: "The trend", Facts: facts}, true
}

// shellAuditAdviceArea is the recommendations, one row each with the measurement it
// came from: the kind of claim, the headline, the argument behind it and the
// evidence. The argument is the one value here that keeps its sentences -- it is
// the model's own measured claim, and cutting it to a row would either drop a fact
// or invent a shorter one (factArgumentLabels).
func shellAuditAdviceArea(report shellAuditReport) (factArea, bool) {
	advice := report.advice()
	if len(advice) == 0 {
		return factArea{}, false
	}

	facts := []iconFact{{Label: "Recommendations", Value: fmt.Sprintf("%d, each with the measurement it came from", len(advice))}}
	for index, item := range advice {
		if index == shellAuditAdviceListLimit {
			facts = append(facts, iconFact{Label: "More", Value: fmt.Sprintf("%d more, all of them from the same measurement", len(advice)-index)})
			break
		}
		facts = append(facts,
			shellAuditAdviceRow(strings.TrimSuffix(shellAuditAdvicePrefix(item.Kind), ":"), item.Headline),
			shellAuditAdviceRow("Why it matters", item.Detail),
			shellAuditAdviceRow("It comes from", item.Evidence),
		)
	}
	return factArea{Name: "Recommendations", Facts: facts}, true
}

// shellAuditAdviceRow is one recommendation fact. The value is the model's own
// sentence with its final full stop moved to the sentence factProse builds, so the
// label that names the kind of claim -- Measured, Observed, Confirmed -- is never
// dropped by a value that already reads as a whole sentence. The words are the
// model's; only the terminator moves.
func shellAuditAdviceRow(label, value string) iconFact {
	return iconFact{Label: label, Value: strings.TrimSuffix(strings.TrimSpace(value), ".")}
}

// shellAuditRunArea is the starts behind the median: every completed start in the
// order it ran, so the number can be reproduced by hand, and the starts that did
// not finish -- a timeout and a failure are left out of the median and said so,
// because a hung start has no time to report and writing one down would invent it.
func shellAuditRunArea(st shellAuditState) (factArea, bool) {
	var facts []iconFact
	if list := shellAuditRunList(st.Samples); list != "" {
		facts = append(facts,
			iconFact{Label: "The starts", Value: list},
			iconFact{Label: "Why they are listed", Value: shellAuditRunListLead},
		)
	}
	if st.Timeouts > 0 {
		facts = append(facts, iconFact{Label: "Killed at the bound", Value: fmt.Sprintf("%s, left out of the median: a hung start has no time to report",
			reviewPlural(st.Timeouts, "start", "starts"))})
	}
	if st.Failures > 0 {
		facts = append(facts, iconFact{Label: "Could not be started", Value: fmt.Sprintf("%s, left out of the median too",
			reviewPlural(st.Failures, "start", "starts"))})
		if st.Failure != "" {
			facts = append(facts, iconFact{Label: "The last one said", Value: st.Failure})
		}
	}
	if len(facts) == 0 {
		return factArea{}, false
	}
	return factArea{Name: "The starts", Facts: facts}, true
}

// shellAuditNotMeasuredArea is the state a first visit sees: the row above starts
// the runs, each one ends by itself or at the bound, and the screen stays usable
// while they run. A press the utility refused says why instead, because a run that
// will refuse again is a result about the start and not a missing measurement.
func shellAuditNotMeasuredArea(st shellAuditState) factArea {
	if st.RunReason != "" {
		return factArea{Name: "The last press", Facts: []iconFact{
			{Label: "Could not take the measurement", Value: st.RunReason},
			{Label: "Trying again", Value: "choosing the row above; each start ends by itself or at the bound, so the screen cannot be held open by your shell"},
		}}
	}
	return factArea{Name: "To measure", Facts: []iconFact{
		{Label: "State", Value: "nothing has been measured yet"},
		{Label: "To start", Value: "choosing the row above runs the starts"},
		{Label: "Each start", Value: "ends by itself or at the bound, so the screen cannot be held open by your shell"},
		{Label: "While they run", Value: "the screen stays usable, and [Esc] stops the ones that are left"},
	}}
}

// shellAuditPromiseFacts is the one thing every shape of this screen promises: the
// audit reads this machine and writes nothing to it, and a slower function is not a
// fault this screen will fix for the user. Each clause is its own row because each
// one is a thing the reader can rely on.
func shellAuditPromiseFacts() []iconFact {
	return []iconFact{
		{Label: "Written", Value: "nothing: no startup file is written and no plugin is disabled"},
		{Label: "A slow function", Value: "is not by itself a mistake"},
		{Label: "The fix", Value: "belongs in your own configuration, applied by you"},
	}
}

// shellAuditAreasTheCardsBesideTheMethodologyAlreadyHold names the areas the wide
// frame draws somewhere other than the Methodology card. The plan's settings are the
// action row's four boxes, the statistics are the Results card's boxes and summary
// rows, and the starts are the Recent runs table -- so a row a reader can already see
// on this frame is not repeated in the card that explains the number.
var shellAuditAreasTheCardsBesideTheMethodologyAlreadyHold = map[string]bool{
	"The plan":              true, // the action row's [m]/[n]/[w]/[b] boxes
	"What the runs support": true, // the Results card's boxes and summary rows
	"The starts":            true, // the Recent runs table
}

// shellAuditDrawnElsewhere reports whether the wide frame draws an area outside the
// Methodology card.
func shellAuditDrawnElsewhere(name string) bool {
	return shellAuditAreasTheCardsBesideTheMethodologyAlreadyHold[name]
}

// shellAuditMethodologyGroups is the wide frame's Methodology card: the areas the
// cards beside it do not already draw. The plan's settings are the action buttons,
// the statistics are the Results card and the starts are the Recent runs card, so
// the card that explains the number carries the method, the context, the
// attribution, the two comparisons, the recommendations and the promise -- the rows
// a reader cannot get anywhere else on this frame.
func (m Model) shellAuditMethodologyGroups(content int) []cardGroup {
	areas := m.shellAuditAreas()

	groups := make([]cardGroup, 0, len(areas))
	for _, area := range areas {
		if shellAuditDrawnElsewhere(area.Name) {
			continue
		}
		lines := factLines(area.Facts, content)
		if len(lines) == 0 {
			continue
		}
		groups = append(groups, cardGroup{Title: area.Name, Lines: lines})
	}
	return groups
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
