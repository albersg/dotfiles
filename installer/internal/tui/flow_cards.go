package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// ============================================================================
// THE FLOW'S CARDS — the backup question, the restore screens, the completion
// and the failure in the reference grammar
// ============================================================================
//
// The wizard's questions draw the page AS the card (components.go, and the six
// question files beside it): the screen's own title as a filled chip set into the
// card's top border, its own sentence at the right of that same row, a rule under
// the pair and every answer a row of icon, title, muted subtitle and chevron with
// the row under the cursor filled across the card. The four screens here carry the
// same material -- a title, a list of facts and the answers a person presses --
// and they drew it in the older panel vocabulary: a branded label row, a guttered
// block of prose and a separate bordered box for the answers, which is why they
// were the last pages whose rows did not read as the grammar's rows.
//
// This file is those four screens in that shape, through the shared card and the
// shared rows (card, cardGroups, cardRowLines, iconFactLines, components.go) --
// not a second renderer that would drift from them. Nothing here decides anything
// a screen did not already decide:
//
//   - the answers are m.GetCurrentOptions(), in the order the key handler reads
//     them, with the cursor at m.Cursor and no row reordered, renamed or added;
//   - the configurations the run would overwrite are m.ExistingConfigs, in its
//     order, with the note that counts the ones the frame could not show;
//   - the restore history is m.AvailableBackups with the same window the list
//     always had, and the way back is the same "← Back" row behind a divider;
//   - the confirmation's file list is the selected backup's own Files;
//   - the completion's facts are the same recorded answers the summary always
//     read, in the same order and with the same values (completionChoiceLabel,
//     fontSummaryValue), and its next step is the same command;
//   - the failure's message is m.ErrorMsg, its way back is
//     wayBackAfterAFailure(m.BackupDir) and its context is m.LogLines.
//
// The palette is the applied theme's: every surface is drawn through a role
// styles.go built, and nothing here names a colour of its own.
//
// None of these is a wide screen. The card is drawn at the composition the frame
// already gives a one-column page, and every renderer falls back to the compact
// body it always had when the frame cannot hold the card -- a content width its
// rows are readable in and a row budget that holds the composed card. The gate
// reads the frame and nothing else, so the shape cannot change while the cursor
// walks a screen, and the 80x24 floor keeps the compact renderers for the frames
// narrower or shorter than the card needs.

const (
	// flowCardMinContent is the narrowest card content these four screens draw
	// their rows in: the row's gutter, a title worth reading and the blank beside
	// it. It is the questions' own floor (wmSelectCardMinContent), so the flow's
	// screens change shape at the frame its questions already do.
	flowCardMinContent = 32

	// backupConfigMark is the gutter every overwritten configuration starts with.
	// It is the mark the compact list has always drawn in front of a path.
	backupConfigMark = "!"

	// backupConfigCaption is the sentence that names what the card's first group
	// holds. It is the screen's own copy, moved onto the card's border where the
	// border row has room for it.
	backupConfigCaption = "The following configs will be overwritten:"

	// backupConfigNote is the reason the screen exists, in the words it always used.
	backupConfigNote = "Creating a backup allows you to restore later if needed."

	// restoreHistoryCaption is the restore list's own sentence, and restoreHistoryBack
	// the row that leaves it. Both are the screen's own words.
	restoreHistoryCaption = "Select a backup to restore or delete"
	restoreHistoryBack    = "← Back"

	// restoreConfirmCaption is the caption that names which backup is being restored:
	// the screen's own phrase with the backup's own timestamp.
	restoreConfirmCaption = "Backup from: "

	// restoreConfirmContentsTitle is the divider the confirmation's file list sits
	// under. It is the title the contents card carried before the list joined the
	// screen's own card, so no word here is new.
	restoreConfirmContentsTitle = "Contents"

	// restoreFileMark is the gutter every file in the backup starts with: the bullet
	// the compact list has always drawn.
	restoreFileMark = "•"

	// restoreConfirmWarning is the confirmation's own warning, unchanged.
	restoreConfirmWarning = "Restoring will overwrite your current configs!"

	// completeSummaryTitle is the completion summary's chip: the run's own verdict
	// mark and the word the block always carried. The check is decoration the card
	// does not have to earn; the facts under it are the run's own record.
	completeSummaryTitle = "✓ Summary"

	// completeNextStepTitle is the divider the command under the summary sits under,
	// and completeNextStepIntro the sentence above it. Both are the screen's own.
	completeNextStepTitle = "Next Step"
	completeNextStepIntro = "To use your new shell now, run:"

	// completeNothingChangedTitle is the dry run's divider: a rehearsal changed
	// nothing, so it has no next step to offer.
	completeNothingChangedTitle = "Nothing changed"
	completeNothingChangedNote  = "This was a rehearsal. Run the installer without DOTFILES_DRY_RUN to apply it."

	// completeDryRunNotice is the dry run's caption. It rides the summary's border
	// where the border row has room for it and moves under the rule where it has
	// not, so a rehearsal never reads as a finished install.
	completeDryRunNotice = "Dry run: no file, package or setting was changed."

	// errorCardTitle is the failure card's chip: the header already says which
	// screen this is, so the body names the block.
	errorCardTitle = "Error"

	// errorWayBackTitle and errorLogsTitle are the two sections under the failure:
	// the screen's own words for what the run left and for the log it kept.
	errorWayBackTitle = "Way back"
	errorLogsTitle    = "Recent logs"

	// errorMessageRows is how many rows the failure message may spend, and
	// errorWayBackRows how many each way-back line may. Both are the compact
	// screen's own limits: a long message or a long backup path is cut with the
	// frame's marker rather than allowed to run off the bottom, and the command
	// beside the path is never the row that is given up.
	errorMessageRows = 4
	errorWayBackRows = 2

	// flowOverflowFormat is the note that counts the rows a list could not show. It
	// is the note listRows and clipLines already draw, in the same words.
	flowOverflowFormat = "… and %d more"
)

// flowAnswerRows draws a screen's own answers in the grammar's row shape: the
// option string the key handler matches as the title and the row under the cursor
// filled and marked. None of these answers opens a page -- pressing one does what
// it says where it stands -- so no row carries the chevron, exactly as the main
// menu's way back does not. The three screens that call it answer in one group, so
// no label here is a separator and nothing here decides a divider.
func flowAnswerRows(options []string, cursor, content int) []string {
	var rows []string
	for i, label := range options {
		rows = append(rows, cardRowLines(cardRow{Title: label}, content, i == cursor)...)
	}
	return rows
}

// flowProseRows wraps one sentence to the card's content width and draws it in the
// role the screen gives it. A sentence is wrapped rather than cut, because the
// note under a list of paths is the reason the screen exists and half a reason
// reads as a warning with no subject.
func flowProseRows(text string, width int, style lipgloss.Style) []string {
	var rows []string
	for _, line := range wrapText(text, width, 0) {
		rows = append(rows, style.Render(line))
	}
	return rows
}

// flowCappedRows is flowProseRows with a row cap: the rows past the cap are
// replaced by the frame's cut marker, which is what the failure screen does with a
// message longer than the block it is given.
func flowCappedRows(text string, width, rows int, style lipgloss.Style) []string {
	var out []string
	for _, line := range wrapText(text, width, rows) {
		out = append(out, style.Render(line))
	}
	return out
}

// flowCardBudget is the rows a card drawn on this screen has: the frame's body,
// less the footer the screen will really spend. Every card body here asks it once
// and gates on the same number it draws with, so the rows a card is measured
// against and the rows it is allowed to spend cannot be two numbers.
func (m Model) flowCardBudget(l layout, hints []installerHint) int {
	return installerBodyRows(m.Height, m.footerReserveRows(l.Inner, hints))
}

// flowCardFits reports whether the frame can hold a card at this width at all.
// Every screen here asks it before it builds one, so a frame too narrow for a row
// keeps the compact body instead of drawing a card with no room for its own rows.
func (m Model) flowCardFits(l layout) (width, content int, ok bool) {
	width = l.Composition
	content = cardContentWidth(width)
	return width, content, content >= flowCardMinContent
}

// ============================================================================
// The backup question
// ============================================================================

// backupConfirmCardBody draws the backup question in the reference grammar: the
// screen's own title as the chip, the sentence that names what the list holds at
// the right of the same row, the configurations the run would overwrite as rows,
// the note that says why the backup exists under the card's rule, and the answers
// under a rule of their own.
//
// It reports false when the frame cannot hold the card, and the screen then draws
// its compact body. The gate is the width and the row budget, so the shape cannot
// change while the cursor walks the three answers.
func (m Model) backupConfirmCardBody(l layout, hints []installerHint) ([]string, bool) {
	width, content, ok := m.flowCardFits(l)
	if !ok {
		return nil, false
	}
	budget := m.flowCardBudget(l, hints)

	options := m.GetCurrentOptions()
	answers := flowAnswerRows(options, m.Cursor, content)
	note := flowProseRows(backupConfigNote, content, InfoStyle)

	// The card spends its chrome, the rule above each group after the first, the
	// note and the answers before it spends a configuration: the list of what
	// would be overwritten is what the rows left are for.
	fixed := cardChromeRows + 2 + len(note) + len(answers)
	room := budget - fixed
	if room < 1 {
		return nil, false
	}

	configs := m.ExistingConfigs
	var overflow string
	if len(configs) > room {
		shown := max(0, room-1)
		overflow = fmt.Sprintf(flowOverflowFormat, len(configs)-shown)
		configs = configs[:shown]
	}
	var rows []string
	for _, config := range configs {
		rows = append(rows, cardRowLines(cardRow{Icon: backupConfigMark, Title: config}, content, false)...)
	}
	if overflow != "" {
		rows = append(rows, MutedStyle.Render(overflow))
	}

	groups := []cardGroup{
		{Lines: rows},
		{Lines: note},
		{Lines: answers},
	}
	// A caption the border row cannot carry moves under the chip's rule rather than
	// being given back, and it costs a row of the card's own body -- so the card is
	// measured as it is drawn, the moved rows included.
	body := questionCard(m.GetScreenTitle(), backupConfigCaption, width, groups)
	if len(body) > budget {
		return nil, false
	}
	return body, true
}

// ============================================================================
// The restore history
// ============================================================================

// restoreHistoryCardBody draws the restore list in the reference grammar: the
// screen's own title as the chip, its own sentence as the caption, one row per
// backup with its timestamp and the number of files it carries, and the way back
// behind the card's divider -- which is the shape every menu in the program draws
// its way out in.
//
// It returns the window it drew, so the header can name the backups it hid: the
// scroll range is the list's vital sign, not a row of the body, so the way back
// does not move as the history grows.
func (m Model) restoreHistoryCardBody(l layout, hints []installerHint) (body []string, start, end int, ok bool) {
	width, content, ok := m.flowCardFits(l)
	if !ok {
		return nil, 0, 0, false
	}
	budget := m.flowCardBudget(l, hints)

	// The card spends its chrome and the rule above the way back before it spends a
	// backup.
	room := budget - cardChromeRows - 1
	if room < 1 {
		return nil, 0, 0, false
	}

	var rows []string
	if len(m.AvailableBackups) == 0 {
		rows = []string{MutedStyle.Render("No backups found.")}
	} else {
		start, end = listWindow(m.Cursor, room, len(m.AvailableBackups))
		for i := start; i < end; i++ {
			backup := m.AvailableBackups[i]
			label := fmt.Sprintf("%s (%d items)",
				backup.Timestamp.Format("2006-01-02 15:04:05"), len(backup.Files))
			rows = append(rows, cardRowLines(cardRow{Title: label}, content, i == m.Cursor)...)
		}
	}

	// The way back is the last row of the option list, one place past the separator
	// that opens it, so the row the key handler answers is the row drawn here.
	backAt := len(m.AvailableBackups) + 1
	groups := []cardGroup{
		{Lines: rows},
		{Lines: cardRowLines(cardRow{Title: restoreHistoryBack}, content, m.Cursor == backAt)},
	}
	body = questionCard(m.GetScreenTitle(), restoreHistoryCaption, width, groups)
	if len(body) > budget {
		return nil, 0, 0, false
	}
	return body, start, end, true
}

// ============================================================================
// The restore confirmation
// ============================================================================

// restoreConfirmCardBody draws the confirmation in the reference grammar: the
// screen's own title as the chip, which backup it is as the caption, the files the
// backup carries under the card's own "Contents" divider, the warning under the
// next, and the three answers under the last.
//
// Nothing is decided here: the files are the selected backup's own list, the
// answers are m.GetCurrentOptions() and the warning is the sentence the screen has
// always carried. The card only puts them in the one shape the rest of the program
// draws a list and its answers in.
func (m Model) restoreConfirmCardBody(backup system.BackupInfo, l layout, hints []installerHint) ([]string, bool) {
	width, content, ok := m.flowCardFits(l)
	if !ok {
		return nil, false
	}
	budget := m.flowCardBudget(l, hints)

	answers := flowAnswerRows(m.GetCurrentOptions(), m.Cursor, content)
	warning := flowProseRows(restoreConfirmWarning, content, WarningStyle)

	// The card spends its chrome, the rule that gives the contents group its name,
	// the two rules under it, the warning and the answers before it spends a file.
	fixed := cardChromeRows + 3 + len(warning) + len(answers)
	room := budget - fixed
	if room < 1 {
		return nil, false
	}

	files := backup.Files
	var overflow string
	if len(files) > room {
		shown := max(0, room-1)
		overflow = fmt.Sprintf(flowOverflowFormat, len(files)-shown)
		files = files[:shown]
	}
	var fileRows []string
	for _, file := range files {
		fileRows = append(fileRows, cardRowLines(cardRow{Icon: restoreFileMark, Title: file}, content, false)...)
	}
	if overflow != "" {
		fileRows = append(fileRows, MutedStyle.Render(overflow))
	}

	groups := []cardGroup{
		// The card draws a group's name on the rule above it and nothing above the
		// first group, so an empty group in front gives "Contents" its rule and costs
		// the divider alone -- the same way the plan screen names its first area.
		{},
		{Title: restoreConfirmContentsTitle, Lines: fileRows},
		{Lines: warning},
		{Lines: answers},
	}
	body := questionCard(m.GetScreenTitle(), restoreConfirmCaption+backup.Timestamp.Format("2006-01-02 15:04:05"),
		width, groups)
	if len(body) > budget {
		return nil, false
	}
	return body, true
}

// ============================================================================
// The completion
// ============================================================================

// completionFacts is the summary the completion screen states: the answers the run
// recorded, in the wizard's own vocabulary, with the glyph the setup table draws
// for the same decision so one choice wears one mark wherever it is shown. A
// question the run never answered says so rather than presenting a guess, and the
// font and the editor appear only when the run installed them.
func (m Model) completionFacts() []iconFact {
	facts := []iconFact{
		{Icon: setupPlatformIcon, Label: "OS", Value: m.completionChoiceLabel(ScreenOSSelect, m.Choices.OS)},
		{Icon: setupTerminalIcon, Label: "Terminal", Value: m.completionChoiceLabel(ScreenTerminalSelect, m.Choices.Terminal)},
		{Icon: setupShellIcon, Label: "Shell", Value: m.completionChoiceLabel(ScreenShellSelect, m.Choices.Shell)},
		{Icon: setupMultiplexerIcon, Label: "Multiplexer", Value: m.completionChoiceLabel(ScreenWMSelect, m.Choices.WindowMgr)},
	}
	if m.Choices.InstallFont {
		facts = append(facts, iconFact{Icon: setupFontIcon, Label: "Font", Value: m.fontSummaryValue()})
	}
	if m.Choices.InstallNvim {
		facts = append(facts, iconFact{Icon: setupEditorIcon, Label: "Editor", Value: "Neovim with dotfiles config"})
	}
	return facts
}

// completionShellCommand is the command that puts the run's shell in front of the
// reader: the shell the wizard recorded, under the name it is invoked by. Nushell
// is the one whose binary is not its question's answer, so it is spelled here once
// and both of the screen's shapes read it from here.
func (m Model) completionShellCommand() string {
	if m.Choices.Shell == "nushell" {
		return "nu"
	}
	return m.Choices.Shell
}

// completeCardBody draws the completion screen in the reference grammar: the
// run's recorded answers as the card's fact rows, and the thing to do next under
// the card's own divider -- or, in a rehearsal, what the run did not change.
//
// The dry run's notice is the caption: it rides the summary's border where the
// border row has room and moves under the rule where it has not, so a rehearsal
// never loses the sentence that says nothing was written, and the screen does not
// claim a next step it never set up.
func (m Model) completeCardBody(l layout, hints []installerHint) ([]string, bool) {
	width, content, ok := m.flowCardFits(l)
	if !ok {
		return nil, false
	}
	budget := m.flowCardBudget(l, hints)

	caption := ""
	next := cardGroup{
		Title: completeNextStepTitle,
		Lines: []string{
			InfoStyle.Render(completeNextStepIntro),
			HighlightStyle.Render(fmt.Sprintf("exec %s", m.completionShellCommand())),
		},
	}
	if dryRun() {
		caption = completeDryRunNotice
		next = cardGroup{
			Title: completeNothingChangedTitle,
			Lines: []string{InfoStyle.Render(completeNothingChangedNote)},
		}
	}
	groups := []cardGroup{
		{Lines: iconFactLines(m.completionFacts(), content)},
		next,
	}
	body := questionCard(completeSummaryTitle, caption, width, groups)
	if len(body) > budget {
		return nil, false
	}
	return body, true
}

// ============================================================================
// The failure
// ============================================================================

// errorWayBackRows is the failure's way back in the card: the same sentences
// wayBackAfterAFailure returns, each wrapped to the card's content and cut to the
// rows it may spend. The command is the reason the block is there, so the line
// that carries it is drawn before the line under it gets any rows, and a path
// longer than its row is cut with the frame's marker rather than shown as a run
// the reader was never given.
func (m Model) errorWayBackRows(content int) []string {
	var rows []string
	for _, line := range wayBackAfterAFailure(m.BackupDir) {
		rows = append(rows, mutedLines(wrapText(line, content, errorWayBackRows))...)
	}
	return rows
}

// errorCardBody draws the failure screen in the reference grammar: the message as
// the card's opening section, the way back under a divider named for it, and the
// freshest log lines under a divider of their own when the frame leaves room for
// them.
//
// The message is capped and the way back is charged for before the log is: a
// failure is where a person needs to know what the run already changed and how to
// put it back, and the log is context for that rather than a substitute for it --
// which is the order the screen has always drawn them in.
func (m Model) errorCardBody(l layout, hints []installerHint) ([]string, bool) {
	width, content, ok := m.flowCardFits(l)
	if !ok {
		return nil, false
	}
	budget := m.flowCardBudget(l, hints)

	message := flowCappedRows(m.ErrorMsg, content, errorMessageRows, ErrorStyle)
	wayBack := m.errorWayBackRows(content)

	// The card spends its chrome, the rule above the way back, the message and the
	// way back before it spends a log line.
	room := budget - cardChromeRows - 1 - len(message) - len(wayBack)
	if room < 0 {
		return nil, false
	}

	groups := []cardGroup{
		{Lines: message},
		{Title: errorWayBackTitle, Lines: wayBack},
	}
	// The log keeps its own rule and at least one line under it: a frame that has
	// the rows for the rule and nothing to put under it shows no panel at all rather
	// than an empty one.
	if len(m.LogLines) > 0 && room-1 >= 2 {
		groups = append(groups, cardGroup{
			Title: errorLogsTitle,
			Lines: logTail(m.LogLines, room-1, content, InfoStyle),
		})
	}

	body := questionCard(errorCardTitle, "", width, groups)
	if len(body) > budget {
		return nil, false
	}
	return body, true
}
