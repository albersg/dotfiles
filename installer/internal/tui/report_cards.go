package tui

import (
	"fmt"
	"strings"
)

// ============================================================================
// THE TWO REPORTS — the capability reading and the last install, in the grammar
// ============================================================================
//
// The capability report and the verification are the section's two report
// screens: a screen whose only row is the way back, because the prose IS the
// screen. They already drew in the reference's card on a frame that holds it
// (section_wide.go), but their bodies were still paragraphs -- one line per fact
// and one line under it -- where the grammar has a word for exactly that shape: a
// row whose title is the fact, whose trailing value is the answer and whose muted
// second line is where the answer came from, or a row per finding with the
// machine's own verdict in its gutter.
//
// This file is that translation and nothing else. It reads the two screens'
// existing reports -- terminalCapabilities.rows(), installVerdict.Findings -- and
// draws them through cardRowLines, cardGroups and iconFactLines, the same
// functions the reference screens, the wizard's questions and the other section
// screens call. No sentence is rewritten, no capability, path, key or outcome is
// invented, and the compact renderer keeps the prose it always drew at the 80x24
// floor.
//
// The card's own composition -- the chip, the rule, the way back under its rule
// and the notice -- stays in section_wide.go: these two screens answer "what does
// my report look like in rows", not "how is a report card built".

const (
	// capabilityReadOnlyPromise is the one sentence the capability report opens
	// with. It is the screen's own line and it is kept here, beside the rows it
	// introduces, so the prose the floor draws and the card the wide frame draws
	// cannot say it twice in two different ways.
	capabilityReadOnlyPromise = "Read-only: nothing here is changed and nothing is written, and every answer names where it came from."

	// installVerifyReadOnlyPromise is the verification's equivalent: what the check
	// does, and the limit the record carries.
	installVerifyReadOnlyPromise = "Read-only, and it writes nothing. Every path and backup the record names is here or gone, and every " +
		"file the run changed is compared against the digest it held when the run finished: still holding it, changed " +
		"since, or unreadable. Where the record carries no digest, being here is all this can say."

	// installVerifyMissingGroup and installVerifyChangedGroup name the two groups a
	// verdict's findings are drawn in. They are the words the rows themselves carry
	// ("missing …", "changed …"), so a divider repeats the screen's own vocabulary
	// rather than naming a third category.
	installVerifyMissingGroup = "missing"
	installVerifyChangedGroup = "changed"
)

// sectionReportGroups returns the report's own content as the grammar's groups,
// and false for every screen that is not one of the two reports. It is asked by
// sectionWideReportCard before it composes the card, so the card keeps its chip,
// its rule, its way back and its notice and only the body changes shape.
func (m Model) sectionReportGroups(content int) ([]cardGroup, bool) {
	switch m.Screen {
	case ScreenTerminalCapabilities:
		return m.terminalCapabilityGroups(content), true
	case ScreenInstallVerify:
		return m.installVerifyGroups(content), true
	}
	return nil, false
}

// sectionWideGroupsCard draws a report whose body is already the grammar's rows:
// the groups as they arrive, the way back under the card's own rule and the
// notice under that. The rows the report could not fit are cut with the same
// marker the prose block uses, and the cut comes out of the report's own rows
// first -- the way back is the one row a screen cannot do without.
func (m Model) sectionWideGroupsCard(groups []cardGroup, screenNotice string, tail []string, cursor, width, budget int) []string {
	content := cardContentWidth(width)
	rows := max(1, budget-cardChromeRows)

	var notice []string
	if strings.TrimSpace(screenNotice) != "" {
		notice = mutedLines(wrapText(screenNotice, content, 0))
	}

	// The way back and the notice are the card's own groups: a group after the
	// first is set off by the card's rule, which is the divider the report drew
	// above its way back.
	reserved := 0
	if len(tail) > 0 {
		reserved += 1 + len(tail)
	}
	if len(notice) > 0 {
		reserved += 1 + len(notice)
	}
	groups = clipCardGroups(groups, max(1, rows-reserved), content)

	if len(tail) > 0 {
		lines := make([]string, 0, len(tail))
		for i, row := range tail {
			// The cursor walks the rows the report does offer -- the way back -- so
			// the row it is on carries the marker here too.
			lines = append(lines, sectionRowLines(row, content, i == cursor)...)
		}
		groups = append(groups, cardGroup{Lines: lines})
	}
	if len(notice) > 0 {
		groups = append(groups, cardGroup{Lines: notice})
	}
	return cardGroups(m.sectionWideTitle(), "", width, groups)
}

// wrappedCardRowLines draws a grammar row whose muted line is longer than the
// card is wide: the row as cardRowLines draws it, then the rest of the sentence
// under it at the same indent and in the same muted role. A capability's reason
// and the way to check it by hand are the tail of one sentence, and a row that cut
// them would lose the only part of an unknown answer a reader can act on.
func wrappedCardRowLines(row cardRow, width int, selected bool) []string {
	if row.Subtitle == "" {
		return cardRowLines(row, width, selected)
	}
	wrapped := wrapText(row.Subtitle, max(1, width-grammarRowGutter), 0)
	if len(wrapped) <= 1 {
		return cardRowLines(row, width, selected)
	}
	row.Subtitle = wrapped[0]
	lines := cardRowLines(row, width, selected)
	for _, extra := range wrapped[1:] {
		lines = append(lines, MutedStyle.Render("  "+extra))
	}
	return lines
}

// clipCardGroups cuts groups to the rows a card has for them, counting the rule
// the card draws above every group after the first. A group that cannot show a
// row is dropped, and the group that gives way says how many rows it could not
// show, so a report never ends in silence.
func clipCardGroups(groups []cardGroup, rows, width int) []cardGroup {
	out := make([]cardGroup, 0, len(groups))
	left := rows
	for _, group := range groups {
		if len(group.Lines) == 0 {
			// A group with no rows would draw its divider over nothing.
			continue
		}
		if len(out) > 0 {
			left-- // the rule above this group
		}
		if left < 1 {
			break
		}
		group.Lines = clipLines(group.Lines, left, width)
		out = append(out, group)
		left -= len(group.Lines)
		if left <= 0 {
			break
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// The capability report
// ---------------------------------------------------------------------------

// terminalCapabilityGroups is the report in the grammar: the read-only promise
// under the chip, and one row per capability -- its name, the answer at the row's
// right edge and where the answer came from under both -- with the state's mark
// in the gutter.
func (m Model) terminalCapabilityGroups(content int) []cardGroup {
	if !m.TerminalCapabilities.Resolved {
		// The screen's own sentence: it is reading, and it has no answer to draw yet.
		return []cardGroup{{Lines: mutedLines(wrapSectionParagraphs(m.terminalCapabilitiesDescription(), content))}}
	}

	groups := []cardGroup{{Lines: mutedLines(wrapSectionParagraphs([]string{capabilityReadOnlyPromise}, content))}}
	rows := make([]string, 0, 2*len(m.TerminalCapabilities.rows()))
	for _, row := range m.TerminalCapabilities.rows() {
		rows = append(rows, terminalCapabilityRowLines(row, content)...)
	}
	return append(groups, cardGroup{Lines: rows})
}

// terminalCapabilityRowLines draws one capability as the grammar's row: the
// capability as the title, the answer at the row's right edge, and the source and
// consequence on the muted line under them. The mark in the gutter is the state
// the answer's own word already carries.
func terminalCapabilityRowLines(row terminalCapabilityRow, content int) []string {
	return wrappedCardRowLines(cardRow{
		Icon:     capabilityMark(row.Answer.State),
		Title:    row.Name,
		Trailing: row.Answer.Value,
		Subtitle: terminalCapabilityDetail(row),
	}, content, false)
}

// terminalCapabilityDetail is the sentence under a capability's answer: where the
// answer came from and what it means, with the reason and the way to check by
// hand when the probe could not determine it. It is the report's own sentence --
// the compact screen prints it too -- so the two shapes cannot explain one answer
// in two different ways.
func terminalCapabilityDetail(row terminalCapabilityRow) string {
	detail := fmt.Sprintf("Source: %s — %s.", row.Answer.Source, row.Implication)
	if row.Answer.State == capabilityUnknown && row.Answer.Reason != "" {
		detail += " Why unknown: " + row.Answer.Reason + "."
		if row.Answer.Manual != "" {
			detail += " Check: " + row.Answer.Manual + "."
		}
	}
	return detail
}

// capabilityMark is the state's mark in a row's gutter: the two marks the section
// screens already use for a check that passed or failed, and the question mark
// they use for one that could not be answered.
func capabilityMark(state capabilityState) string {
	switch state {
	case capabilitySupported:
		return "✓"
	case capabilityUnsupported:
		return "✗"
	}
	return "?"
}

// ---------------------------------------------------------------------------
// The verification
// ---------------------------------------------------------------------------

// installVerifyGroups is the verification in the grammar: the read-only promise
// and the run the record names, then the findings the machine no longer matches
// -- one row each, the outcome in the gutter, the path as the title, the
// configuration's own name at the row's right edge and what happened to it under
// both -- and the count of what is still as the run left it.
func (m Model) installVerifyGroups(content int) []cardGroup {
	groups := []cardGroup{{Lines: mutedLines(wrapSectionParagraphs([]string{installVerifyReadOnlyPromise}, content))}}

	if !m.InstallVerify.Resolved {
		return append(groups, cardGroup{Lines: mutedLines(wrapSectionParagraphs(
			[]string{"Reading the record of the last install and checking this machine against it…"}, content))})
	}

	verdict := m.InstallVerify.Verdict
	switch {
	case verdict.NoRecord:
		return append(groups, cardGroup{Lines: mutedLines(wrapSectionParagraphs(
			[]string{"No record of an install on this machine: the installer writes one when a run completes, so " +
				"there is nothing to check yet."}, content))})
	case verdict.Unreadable:
		return append(groups, cardGroup{Lines: mutedLines(wrapSectionParagraphs(
			[]string{"The record is here and could not be read: " + verdict.Reason}, content))})
	}

	// The record the check was made against, in the screen's own words.
	groups = append(groups, cardGroup{Lines: mutedLines(wrapSectionParagraphs([]string{
		fmt.Sprintf("Record from %s by %s.",
			verdict.Record.Timestamp.Format(panelBackupStamp), verdict.Record.Version),
	}, content))})

	// A record that names no path is neither a pass nor a failure: there is
	// nothing the record and the machine can be compared on, and saying "every one
	// of the zero paths is here" would read as a pass nobody established.
	if len(verdict.Findings) == 0 {
		return append(groups, cardGroup{Lines: mutedLines(wrapSectionParagraphs(
			[]string{"The record names no path, so there is nothing to compare with this machine."}, content))})
	}

	missing := 0
	changed := 0
	for _, finding := range verdict.Findings {
		switch {
		case finding.Kind.Missing():
			missing++
		case finding.Kind.ChangedSinceTheRun():
			changed++
		}
	}

	if missing > 0 {
		groups = append(groups, m.installVerifyFindingGroup(verdict.Findings, installVerifyMissingGroup,
			fmt.Sprintf("%d of the %d paths and backups the record names are not here:", missing, len(verdict.Findings)),
			func(finding installFinding) bool { return finding.Kind.Missing() }, content))
	}
	if changed > 0 {
		groups = append(groups, m.installVerifyFindingGroup(verdict.Findings, installVerifyChangedGroup,
			fmt.Sprintf("%d file(s) no longer hold what the run left:", changed),
			func(finding installFinding) bool { return finding.Kind.ChangedSinceTheRun() }, content))
	}

	// The number the user came for, and the command that prints the whole list: a
	// record written by a normal run names every configuration path the installer
	// touches, which is longer than any frame.
	return append(groups, cardGroup{Lines: mutedLines(wrapSectionParagraphs([]string{fmt.Sprintf(
		"Still what the run left: %d of the %d. dotfiles --verify prints every one of them.",
		len(verdict.Findings)-missing-changed, len(verdict.Findings))}, content))})
}

// installVerifyFindingGroup is one group of findings: the sentence that counts
// them under the group's own name, then one row per finding the predicate picks.
func (m Model) installVerifyFindingGroup(findings []installFinding, title, sentence string, pick func(installFinding) bool, content int) cardGroup {
	lines := mutedLines(wrapSectionParagraphs([]string{sentence}, content))
	for _, finding := range findings {
		if !pick(finding) {
			continue
		}
		lines = append(lines, installVerifyRowLines(finding, content)...)
	}
	return cardGroup{Title: title, Lines: lines}
}

// installVerifyRowLines draws one finding as the grammar's row: the machine's
// verdict in the gutter, the path the record named as the title, the
// configuration's own name at the row's right edge, and what happened to it on
// the muted line under both.
func installVerifyRowLines(finding installFinding, content int) []string {
	return wrappedCardRowLines(installVerifyRow(finding), content, false)
}

// installVerifyRow is one finding in the grammar's row shape. Every fact the
// compact row printed is here: the outcome, the configuration's name, the path
// and what the run did to it, or the comparison behind a file's finding.
func installVerifyRow(finding installFinding) cardRow {
	row := cardRow{
		Icon:     installVerifyMark(finding),
		Title:    finding.Path,
		Trailing: finding.Key,
		Subtitle: installVerifyOutcome(finding),
	}
	switch {
	case finding.Kind.Copy():
		row.Subtitle += "  backup of " + finding.Key
	case finding.Kind.Missing():
		row.Subtitle += "  " + verifyActionPhrase(finding.Action)
	case finding.Kind == findingFileUnreadable:
		row.Subtitle += "  the file could not be read"
	default:
		row.Subtitle += "  no longer holds what the run left"
		if finding.Detail != "" {
			row.Subtitle += ": " + finding.Detail
		}
	}
	return row
}

// installVerifyOutcome is the word the screen uses for what the machine says
// about one finding. It is the first column of the compact row and the first word
// of the card row's muted line, so the two shapes name one outcome one way.
func installVerifyOutcome(finding installFinding) string {
	switch {
	case finding.Kind.Missing():
		return "missing"
	case finding.Kind == findingFileUnreadable:
		return "unknown"
	default:
		return "changed"
	}
}

// installVerifyMark is the outcome's mark in a finding's gutter. It is the same
// three marks the report screens use for a check that failed, one that needs
// attention and one that could not be answered.
func installVerifyMark(finding installFinding) string {
	switch {
	case finding.Kind.Missing():
		return "✗"
	case finding.Kind == findingFileUnreadable:
		return "?"
	default:
		return "!"
	}
}
