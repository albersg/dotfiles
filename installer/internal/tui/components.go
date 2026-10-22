package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ============================================================================
// THE REFERENCE GRAMMAR
// ============================================================================
//
// The four reference screens (Home, Utilities, Updates, Shell Startup) share one
// vocabulary, and this file is that vocabulary as functions: a card whose title
// is a filled chip on its top border with a muted caption at the right and a rule
// under the pair; a row of icon, title, subtitle and chevron whose selected state
// is one filled bar; a numbered step; a stat box; an action button that carries
// its key; a fact row whose values share a column; a resource bar; a warning
// badge; and a grouped table with a scrollbar. Every screen that wants the same
// object calls the same function, so the grammar cannot drift from screen to
// screen.
//
// Nothing here chooses a colour of its own: every surface is drawn through a
// style the theme builder built (see styles.go), so a palette change reaches all
// of them at once. Nothing here invents a fact, a percentage or a duration; the
// callers pass the model's own values.

const (
	// grammarCardPad is the blank column a card keeps inside each border.
	grammarCardPad = 1

	// grammarGap is the blank columns between two objects on one row.
	grammarGap = 1

	// grammarRowGutter is what a row spends before its title: the icon or the
	// cursor's marker, and the blank after it.
	grammarRowGutter = 2

	// grammarChevron is the mark a row that opens something carries at its right
	// edge. It is one cell, so a terminal without the Nerd Font still reads the
	// row and the glyph never pushes the title off.
	grammarChevron = "›"

	// grammarMarker is the mark the row under the cursor carries in its gutter.
	grammarMarker = "▸"

	// grammarLabelMeasure is the fixed column a fact row's values start at.
	grammarLabelMeasure = 20
)

// cardContentWidth is the columns a card's body may use: the card minus its two
// borders and one blank column on each side.
func cardContentWidth(width int) int {
	return max(1, width-panelFrame-2*grammarCardPad)
}

// cardGroup is one section of a card: the rules the card sets off with a titled
// divider, and the lines it holds. A card has at least one group; a group with no
// title has no divider above it.
type cardGroup struct {
	Title string
	Lines []string
}

// card wraps body rows in the reference grammar's card: a filled chip set into
// the top border, a right-aligned muted caption on the same row, a rule under the
// pair and the body. A card with no title draws a plain rounded box; a card with
// no caption draws the chip and the rest of the border. The width is the whole
// card, borders included, and every row is exactly that wide, so two cards beside
// each other line up.
func card(title, caption string, width int, body []string) []string {
	return cardGroups(title, caption, width, []cardGroup{{Lines: body}})
}

// cardGroups is card with the body split into the sections a titled divider sets
// off. It is the same object as card -- the chip, the caption, the rule under the
// pair and the edges are drawn once -- so a menu that groups its rows and a card
// that does not are one card shape with and without dividers.
func cardGroups(title, caption string, width int, groups []cardGroup) []string {
	width = cardWidth(width)
	inner := cardInner(width)
	edge := func(s string) string { return PanelBorderStyle.Render(s) }

	var top string
	if title == "" {
		top = edge("╭" + strings.Repeat("─", inner) + "╮")
	} else {
		label := CardChipStyle.Render(title)
		tail := 1 // the closing corner
		if caption != "" {
			tail = 1 + lipgloss.Width(caption) + 3 // space, caption, space, rule, corner
		}
		room := inner - lipgloss.Width(label)
		if room < tail+1 {
			// The caption is the first thing given back: the chip names the card,
			// the caption repeats what the body or the frame already says.
			caption = ""
			tail = 1
			room = inner - lipgloss.Width(label)
		}
		if room < 1 {
			label = CardChipStyle.Render(truncate(title, max(1, inner-2)))
			room = inner - lipgloss.Width(label)
		}
		if room < 1 {
			room = 1
		}
		if caption == "" {
			top = edge("╭─") + label + edge(strings.Repeat("─", room-1)+"╮")
		} else {
			dash := inner - lipgloss.Width(label) - lipgloss.Width(caption) - 3
			if dash < 1 {
				dash = 1
			}
			top = edge("╭─") + label + edge(strings.Repeat("─", dash)) +
				" " + MutedStyle.Render(caption) + " " + edge("╮")
		}
	}

	content := inner - 2*grammarCardPad
	row := func(s string) string {
		return edge("│") + strings.Repeat(" ", grammarCardPad) +
			padRight(lipgloss.NewStyle().MaxWidth(content).Render(s), content) +
			strings.Repeat(" ", grammarCardPad) + edge("│")
	}

	out := make([]string, 0, len(groups)*2+3)
	out = append(out, top)
	out = append(out, edge("│")+RuleStyle.Render(strings.Repeat("─", inner))+edge("│"))
	for at, group := range groups {
		if at > 0 {
			// The divider carries the group's own name at the left edge and runs on
			// to the closing border, so a section reads as part of the same card.
			label := " " + TableHeaderStyle.Render(group.Title) + " "
			dash := inner - lipgloss.Width(label)
			if group.Title == "" || dash < 1 {
				label = ""
				dash = inner
			}
			out = append(out, edge("├─")+label+edge(strings.Repeat("─", max(0, dash-1))+"┤"))
		}
		for _, line := range group.Lines {
			// A body row may itself be several physical rows (a bordered badge or a
			// wrapped paragraph), so each physical line is padded and edged on its own
			// rather than letting an embedded newline escape the card.
			for _, physical := range strings.Split(line, "\n") {
				out = append(out, row(physical))
			}
		}
	}
	out = append(out, edge("╰"+strings.Repeat("─", inner)+"╯"))
	return out
}

// cardWidth is the width a card is really drawn in: the caller's, raised to the
// narrowest box that can hold a border, one blank column on each side and two
// columns of body. It is the one place that clamp lives, so the caption test below
// and the drawing above cannot measure two different cards.
func cardWidth(width int) int {
	return max(width, panelFrame+2*grammarCardPad+2)
}

// cardInner is the columns a card's border encloses: everything but its two edges.
func cardInner(width int) int {
	return cardWidth(width) - panelFrame
}

// cardBorderCarriesCaption reports whether the card's top border has room for the
// chip, at least one column of rule, the caption and the closing corner on one row.
// It is the same arithmetic card uses to decide whether to give the caption back,
// asked before the card is drawn so a caller can keep the words instead of losing
// them. A caption that holds its own line break never rides the border: the border
// is one row.
func cardBorderCarriesCaption(title, caption string, width int) bool {
	if caption == "" || strings.Contains(caption, "\n") {
		return false
	}
	label := lipgloss.Width(CardChipStyle.Render(title))
	return cardInner(width)-label >= 1+lipgloss.Width(caption)+3+1
}

// questionCard draws a wizard question in the reference grammar: the screen's own
// title as the chip on the card's top border, its own description as the caption at
// the right of the same row, the rule under the pair and the groups of answers.
//
// A caption the border row has no room for is moved inside the card rather than
// given back. The card gives its caption up first (card, above) because the chip
// names the card; on a question the description is often the only fact the screen
// states -- the host was not detected, the run will skip terminal setup -- so the
// words move under the chip's rule instead of disappearing.
func questionCard(title, caption string, width int, groups []cardGroup) []string {
	if caption == "" || cardBorderCarriesCaption(title, caption, width) {
		return cardGroups(title, caption, width, groups)
	}
	head := mutedLines(wrapText(caption, cardContentWidth(cardWidth(width)), 0))
	return cardGroups(title, "", width, cardGroupsWithHead(head, groups))
}

// cardGroupsWithHead puts the headline rows above the first group's own, so a
// caption that moved inside the card still opens the card.
func cardGroupsWithHead(head []string, groups []cardGroup) []cardGroup {
	if len(groups) == 0 {
		return []cardGroup{{Lines: head}}
	}
	out := append([]cardGroup(nil), groups...)
	out[0].Lines = append(append([]string(nil), head...), out[0].Lines...)
	return out
}

// questionCardGroups builds a question's card body from the option list the key
// handler reads: one grammar row per answer, the row under the cursor filled and
// marked, and -- where the option list carries one -- the separator drawn as the
// divider between two groups rather than as a row. parts supplies each answer's
// icon and muted subtitle, so a screen adds its copy and never a second renderer.
func questionCardGroups(options []string, cursor, content int, parts func(label string) (string, string)) []cardGroup {
	groups := []cardGroup{{}}
	for i, label := range options {
		if strings.HasPrefix(label, menuSeparatorPrefix) {
			groups = append(groups, cardGroup{})
			continue
		}
		icon, subtitle := parts(label)
		at := len(groups) - 1
		groups[at].Lines = append(groups[at].Lines, cardRowLines(cardRow{
			Icon:     icon,
			Title:    label,
			Subtitle: subtitle,
			Chevron:  true,
		}, content, i == cursor)...)
	}
	return groups
}

// questionCardWidth is the width a question's card is drawn in and the columns its
// rows may use: the left column when the frame holds a panel beside it, the whole
// composition otherwise. It reports false when the rows would not be readable in
// that width, which is the caller's signal to draw the compact list instead.
func (m Model) questionCardWidth(minContent int) (width, content int, ok bool) {
	l := layoutFor(m)
	width = l.Composition
	if l.TwoColumn {
		width = l.Left
	}
	content = cardContentWidth(width)
	return width, content, content >= minContent
}

// questionCardBody draws the card and reports whether the frame can hold it. The
// gate is the frame and nothing else -- a content width its rows are readable in and
// a row budget that holds the composed card at once -- so the shape cannot change
// while the cursor walks the answers. The card is measured as it is drawn, borders,
// dividers and any caption that moved inside it included, so a question that grew a
// row cannot be drawn one row past the frame.
func (m Model) questionCardBody(title, caption string, width int, groups []cardGroup, hints []installerHint) ([]string, bool) {
	lines := questionCard(title, caption, width, groups)
	if installerBodyRows(m.Height, m.footerReserveRows(layoutFor(m).Inner, hints)) < len(lines) {
		return nil, false
	}
	return lines, true
}

// cardRow is one row of the grammar's list: the icon in the gutter, the title,
// the muted subtitle under it and the chevron that says the row opens something.
// The selected row replaces the icon with the cursor's marker and fills the whole
// row, subtitle included, so the state is one bar rather than an indent.
type cardRow struct {
	Icon     string
	Title    string
	Subtitle string
	// Chevron draws the mark that opens something. A row that acts where it
	// stands leaves it false and keeps its own key in the title.
	Chevron bool
	// Trailing is a short right-aligned fact on the title line, before the
	// chevron. It is used by the table rows that need a manager or a count.
	Trailing string
}

// cardRowLines draws one row in the card's content width: one line when it has
// no subtitle, two when it has. A selected row's bar spans both lines and both
// carry on-fill ink, as the reference draws it.
func cardRowLines(row cardRow, width int, selected bool) []string {
	width = max(1, width)
	chevronWidth := 0
	if row.Chevron {
		chevronWidth = 2 // the chevron and the blank before it
	}
	trailingWidth := 0
	if row.Trailing != "" {
		trailingWidth = lipgloss.Width(row.Trailing) + 2
	}
	textWidth := max(1, width-grammarRowGutter-chevronWidth-trailingWidth)

	title := padRight(truncate(row.Title, textWidth), textWidth)
	trailing := ""
	if row.Trailing != "" {
		trailing = "  " + truncate(row.Trailing, trailingWidth-2)
	}
	chevron := ""
	if row.Chevron {
		chevron = " " + grammarChevron
	}

	var head string
	if selected {
		head = RowBarMarkerStyle.Render(grammarMarker+" ") +
			RowBarStyle.Render(title)
		if trailing != "" {
			head += RowBarStyle.Render(trailing)
		}
		if chevron != "" {
			head += RowBarStyle.Render(chevron)
		}
	} else {
		gutter := strings.Repeat(" ", grammarRowGutter)
		if row.Icon != "" {
			gutter = AccentKeyStyle.Render(row.Icon) + " "
		}
		head = gutter + InkStyle.Render(title)
		if trailing != "" {
			head += MutedStyle.Render(trailing)
		}
		if chevron != "" {
			head += MutedStyle.Render(chevron)
		}
	}

	if row.Subtitle == "" {
		return []string{head}
	}

	subWidth := max(1, width-grammarRowGutter)
	subtitle := padRight(truncate(row.Subtitle, subWidth), subWidth)
	if selected {
		return []string{head, OnFillMutedStyle.Render("  " + subtitle)}
	}
	return []string{head, MutedStyle.Render("  " + subtitle)}
}

// numberedStep is one item of the grammar's numbered list: a filled circle with
// its number, a title and the muted line under it. The reference uses it for the
// home screen's plan and for the shell startup explanation.
type numberedStep struct {
	Title    string
	Subtitle string
}

// numberedStepLines draws the steps one after another, each number aligned with
// the others and each subtitle under its title at the same indent.
func numberedStepLines(steps []numberedStep, width int) []string {
	out := make([]string, 0, len(steps)*2)
	for i, step := range steps {
		number := StepNumberStyle.Render(fmt.Sprintf("%d", i+1))
		indent := grammarRowGutter + 1 // number block and the blank after it
		textWidth := max(1, width-indent)
		head := number + " " + InkStyle.Render(padRight(truncate(step.Title, textWidth), textWidth))
		out = append(out, head)
		if step.Subtitle != "" {
			for _, line := range wrapText(step.Subtitle, max(1, textWidth-2), 2) {
				out = append(out, strings.Repeat(" ", indent)+MutedStyle.Render(line))
			}
		}
	}
	return out
}

// statBox is one small bordered box with a label and a large value.
type statBox struct {
	Label string
	Value string
	Tone  lipgloss.Style
}

const statBoxHeight = 4

// statBoxLines draws the boxes side by side across width, each the same width so
// the labels line up. It returns nil when even one box cannot hold its label and
// value, which is the caller's signal to state the same facts as plain rows
// rather than cut them.
func statBoxLines(boxes []statBox, width int) []string {
	if len(boxes) == 0 || width < 1 {
		return nil
	}
	each := (width - grammarGap*(len(boxes)-1)) / len(boxes)
	need := 0
	for _, box := range boxes {
		need = max(need, lipgloss.Width(box.Label), lipgloss.Width(box.Value))
	}
	need += 2*grammarCardPad + panelFrame
	if each < need {
		return nil
	}

	widths := make([]int, len(boxes))
	for i := range widths {
		widths[i] = each
	}
	for i, extra := 0, (width-grammarGap*(len(boxes)-1))-each*len(boxes); i < extra; i++ {
		widths[i%len(boxes)]++
	}

	rows := make([]string, statBoxHeight)
	for i, box := range boxes {
		lines := statBoxOf(box, widths[i])
		for r, line := range lines {
			if rows[r] == "" {
				rows[r] = line
				continue
			}
			rows[r] += strings.Repeat(" ", grammarGap) + line
		}
	}
	return rows
}

// statBoxOf draws one box in exactly statBoxHeight rows: the top border, the
// label, the large value and the closing border.
func statBoxOf(box statBox, width int) []string {
	edge := func(s string) string { return PanelBorderStyle.Render(s) }
	inner := max(1, width-panelFrame)
	content := max(1, inner-2*grammarCardPad)
	tone := box.Tone
	if tone.GetForeground() == (lipgloss.NoColor{}) {
		tone = InkStyle
	}
	return []string{
		edge("╭" + strings.Repeat("─", inner) + "╮"),
		edge("│") + strings.Repeat(" ", grammarCardPad) +
			MutedStyle.Render(padRight(truncate(box.Label, content), content)) +
			strings.Repeat(" ", grammarCardPad) + edge("│"),
		edge("│") + strings.Repeat(" ", grammarCardPad) +
			tone.Bold(true).Render(padRight(truncate(box.Value, content), content)) +
			strings.Repeat(" ", grammarCardPad) + edge("│"),
		edge("╰" + strings.Repeat("─", inner) + "╯"),
	}
}

// actionSpec is one press the screen offers: the key it carries, the words it
// draws, the muted line under them and a glyph. The primary action is filled;
// the others are bordered.
type actionSpec struct {
	Key      string
	Title    string
	Subtitle string
	Icon     string
}

const (
	actionButtonHeight = 4
	actionButtonGap    = 2
)

// actionButtonRow draws the screen's actions in one row of equal boxes: the
// primary action filled in the brand role, the others bordered. It returns nil
// when the boxes cannot hold their titles, so the caller can fall back to rows
// and lose no press.
// actionButtonNeed is the narrowest a box can be: its key, title, subtitle and the
// border and padding. The key is counted because it is drawn in the same line as
// the title and a box measured without it cuts the last word off.
func actionButtonNeed(action actionSpec) int {
	key := 0
	if action.Key != "" {
		key = lipgloss.Width(action.Key) + 3
	}
	return max(lipgloss.Width(action.Title)+key, lipgloss.Width(action.Subtitle)) + 2*grammarCardPad + panelFrame
}

func actionButtonRow(primary actionSpec, others []actionSpec, width int) []string {
	all := append([]actionSpec{primary}, others...)
	if len(all) == 0 || width < 1 {
		return nil
	}
	// Each action gets the width its own two lines need, and the spare columns are
	// shared out afterwards. An equal split made the filled primary -- the longest
	// label -- set the width of every box, so the whole row was refused on a frame
	// that had room for all five labels.
	widths := make([]int, len(all))
	used := actionButtonGap * (len(all) - 1)
	for i, action := range all {
		need := actionButtonNeed(action)
		widths[i] = need
		used += need
	}
	if used > width {
		return nil
	}
	for i := 0; used < width; i = (i + 1) % len(all) {
		widths[i]++
		used++
	}

	rows := make([]string, actionButtonHeight)
	for i, action := range all {
		lines := actionButtonOf(action, widths[i], i == 0)
		for r, line := range lines {
			if rows[r] == "" {
				rows[r] = line
				continue
			}
			rows[r] += strings.Repeat(" ", actionButtonGap) + line
		}
	}
	return rows
}

// actionButtonOf draws one action box: filled for the primary, bordered for the
// rest, with the key and the title on one row and the subtitle under it.
func actionButtonOf(action actionSpec, width int, primary bool) []string {
	inner := max(1, width-panelFrame)
	content := max(1, inner-2*grammarCardPad)
	keyPrefix := ""
	if action.Key != "" {
		keyPrefix = "[" + action.Key + "] "
	}
	head := strings.TrimSpace(action.Icon + " " + keyPrefix + action.Title)
	subtitle := truncate(action.Subtitle, content)

	if primary {
		fill := func(s string) string {
			return RowBarStyle.Render(padRight(s, inner))
		}
		return []string{
			fill(" " + padRight(head, inner-2) + " "),
			fill(" " + padRight("", inner-2) + " "),
			fill(" " + padRight(subtitle, inner-2) + " "),
			fill(strings.Repeat(" ", inner)),
		}
	}

	edge := func(s string) string { return PanelBorderStyle.Render(s) }
	row := func(s string) string {
		return edge("│") + strings.Repeat(" ", grammarCardPad) +
			padRight(truncate(s, content), content) + strings.Repeat(" ", grammarCardPad) + edge("│")
	}
	return []string{
		edge("╭" + strings.Repeat("─", inner) + "╮"),
		row(AccentKeyStyle.Render(head)),
		row(MutedStyle.Render(subtitle)),
		edge("╰" + strings.Repeat("─", inner) + "╯"),
	}
}

// iconFact is one labelled fact of the reference's detail cards: a small glyph,
// the label at a fixed column, and the value wrapped under it.
type iconFact struct {
	Icon  string
	Label string
	Value string
}

// iconFactLines draws the facts with their values at one shared column. The icon
// and the label are decoration and metadata; the value is the fact and is wrapped
// rather than cut, so a long path or a wrapped note stays readable.
func iconFactLines(facts []iconFact, width int) []string {
	var out []string
	for _, fact := range facts {
		if strings.TrimSpace(fact.Value) == "" {
			continue
		}
		label := fact.Label
		measure := grammarLabelMeasure
		if width < grammarLabelMeasure+12 {
			measure = min(grammarLabelMeasure, max(1, width/2))
		}
		valueWidth := max(1, width-measure-1)
		lines := wrapText(fact.Value, valueWidth, 0)
		if len(lines) == 0 {
			continue
		}
		gutter := fact.Icon
		if gutter == "" {
			gutter = " "
		}
		head := AccentKeyStyle.Render(gutter) + " " +
			MutedStyle.Render(padRight(truncate(label, measure-2), measure-2)) + " "
		indent := strings.Repeat(" ", measure+1)
		out = append(out, head+InkStyle.Render(lines[0]))
		for _, line := range lines[1:] {
			out = append(out, indent+InkStyle.Render(line))
		}
	}
	return out
}

// resourceBarRow draws one resource: the label, the filled violet bar, the
// hatched remainder, and the value the sample reported. The fraction comes from
// the caller's sample; nothing here computes a percentage the model does not
// hold.
func resourceBarRow(label string, fraction float64, value string, width int) string {
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}
	const labelWidth = 8
	valueWidth := lipgloss.Width(value)
	cells := max(4, width-labelWidth-1-valueWidth-1)
	if cells > 20 {
		cells = 20
	}
	filled := int(fraction*float64(cells) + 0.5)
	filled = min(max(filled, 0), cells)
	bar := MeterFilledStyle.Render(strings.Repeat("█", filled)) +
		MeterStyle.Render(strings.Repeat("░", cells-filled))
	return MutedStyle.Render(padRight(truncate(label, labelWidth), labelWidth)) + " " +
		bar + " " + InkStyle.Render(value)
}

// warningBadge draws the reference's amber badge. A badge whose text does not
// fit the width is cut with the frame's marker rather than wrapping the card out
// of its budget; the caller's prose under it carries the whole sentence.
func warningBadge(text string, width int) []string {
	// The badge's border and padding spend four columns, so the label is cut to
	// leave them room: an over-wide badge would be clipped by its card and lose
	// the corner that says it is a badge.
	label := "⚠ " + truncate(text, max(1, width-6))
	return []string{
		BadgeStyle.Render(label),
	}
}

// groupedTableHeader is the table's heading row: a checkbox column, then the
// columns that name what the rows hold. The widths are the caller's, so the
// header and the rows share one measure.
func groupedTableHeader(columns []string, widths []int, width int) string {
	var b strings.Builder
	for i, column := range columns {
		w := 0
		if i < len(widths) {
			w = widths[i]
		}
		if w < 1 {
			w = lipgloss.Width(column)
		}
		if i > 0 {
			b.WriteString(strings.Repeat(" ", grammarGap*2))
		}
		b.WriteString(TableHeaderStyle.Render(padRight(truncate(column, w), w)))
	}
	return truncate(b.String(), width)
}

// resourcePercent renders a 0..1 fraction as the whole-number percentage the
// reference shows. It is used only where the caller already holds a fraction the
// model measured.
func resourcePercent(fraction float64) string {
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}
	return fmt.Sprintf("%.0f%%", fraction*100)
}

// fitColumnBlocks stacks the blocks a column can hold within budget rows,
// separated by one blank row, and drops the blocks that do not fit whole. A
// column is never cut through a card's border: the lower priority block is left
// out entirely rather than drawn half, which is what keeps the frame's own
// header and footer from scrolling off a screen whose content grew.
func fitColumnBlocks(blocks [][]string, budget int) []string {
	var out []string
	for _, block := range blocks {
		if len(block) == 0 {
			continue
		}
		add := len(block)
		if len(out) > 0 {
			add++ // the separating blank row
		}
		if len(out)+add > budget {
			break
		}
		if len(out) > 0 {
			out = append(out, "")
		}
		out = append(out, block...)
	}
	return out
}

// actionButtonRowAll draws every action as a bordered box, for the frame where
// the filled primary has moved to its own row and the remaining presses share
// this one. It is the same box as actionButtonRow's secondary actions.
func actionButtonRowAll(actions []actionSpec, width int) []string {
	if len(actions) == 0 || width < 1 {
		return nil
	}
	each := (width - actionButtonGap*(len(actions)-1)) / len(actions)
	for _, action := range actions {
		need := actionButtonNeed(action)
		if each < need {
			return nil
		}
	}
	widths := make([]int, len(actions))
	for i := range widths {
		widths[i] = each
	}
	for i, extra := 0, (width-actionButtonGap*(len(actions)-1))-each*len(actions); i < extra; i++ {
		widths[i%len(actions)]++
	}
	rows := make([]string, actionButtonHeight)
	for i, action := range actions {
		lines := actionButtonOf(action, widths[i], false)
		for r, line := range lines {
			if rows[r] == "" {
				rows[r] = line
				continue
			}
			rows[r] += strings.Repeat(" ", actionButtonGap) + line
		}
	}
	return rows
}
