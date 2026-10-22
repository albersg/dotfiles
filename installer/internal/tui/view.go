package tui

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/albersg/dotfiles/installer/internal/system"
	"github.com/albersg/dotfiles/installer/internal/tui/trainer"
	"github.com/charmbracelet/lipgloss"
)

// formatControlChars converts control characters to readable format for display
func formatControlChars(input string) string {
	result := input
	result = strings.ReplaceAll(result, "\x04", "<C-d>")
	result = strings.ReplaceAll(result, "\x15", "<C-u>")
	result = strings.ReplaceAll(result, "\x06", "<C-f>")
	result = strings.ReplaceAll(result, "\x02", "<C-b>")
	result = strings.ReplaceAll(result, "\x12", "<C-r>")
	result = strings.ReplaceAll(result, "\x16", "<C-v>")
	return result
}

// Every footer in the program is one list of installerHint values -- the keys
// that trigger an action and the verb that names it -- packed and styled by
// footerHints below. One notation and one order is the rule: every key is
// bracketed, and a screen lists its hints navigation first, then the action,
// then the way back or out. The trainer screens used to spell their legends as
// plain strings joined by their own notation, which is why their keys did not
// look like keys; they go through the same component now, and their hints are
// declared beside the trainer views.

// deadEndScreen renders a screen with nothing to show: what is missing, and the
// way out of it. The screens used to name the fact and stop ("Category not
// found", "No backup selected"), which left the reader with no next step, and
// they drew it as two bare lines: no header chip, no state, no legend -- the one
// shape in the program that was not the frame. A dead end is still a screen, so
// it wears the frame like every other one and the legend carries the way out.
func (m Model) deadEndScreen(message, action string) string {
	body := []string{ErrorStyle.Render(message)}
	if wayOut := deadEndWayOut(action); wayOut != "" {
		body = append(body, "", MutedStyle.Render(wayOut))
	}
	return m.frame(m.headerName(), "", body, []installerHint{hintBack})
}

// deadEndWayOut turns the way-out sentence into the line under the title. The
// key it used to spell ("Press [Esc] to ...") is the legend's now, so the
// sentence keeps only where the key goes -- the footer names the key once, which
// is the notation every other screen uses.
func deadEndWayOut(action string) string {
	const prefix = "Press [Esc] to "
	rest := strings.TrimPrefix(strings.TrimSpace(action), prefix)
	if rest == "" {
		return ""
	}
	runes := []rune(rest)
	if unicode.IsLower(runes[0]) {
		runes[0] = unicode.ToUpper(runes[0])
	}
	return string(runes)
}

// trainerDeadEnd is deadEndScreen for the trainer's own frame. The trainer
// composes its chrome row by row instead of going through the installer's frame
// (see the installer frame's comment), so an empty trainer screen composes the
// same header, rule and footerBar the exercise and boss screens do.
func (m Model) trainerDeadEnd(heading, message, action string) string {
	inner := trainerInnerWidth(m)
	rows := []string{
		headerRow(heading, "", inner),
		headerRule(inner),
		"",
		ErrorStyle.Render(message),
	}
	if wayOut := deadEndWayOut(action); wayOut != "" {
		rows = append(rows, "", MutedStyle.Render(wayOut))
	}
	rows = append(rows, "")
	rows = append(rows, m.footerBar(inner, []installerHint{hintBack})...)
	return strings.Join(rows, "\n")
}

// ============================================================================
// THE INSTALLER FRAME
// ============================================================================
//
// Every non-trainer screen is drawn inside one frame: a header row with the
// section name on the left and the screen's vital sign on the right, a rule
// under it, the body, a rule above the footer, and the footer's help line. The
// frame is why the installer's screens read as one program instead of twenty
// layouts: the same rows land in the same places on all of them.
//
// The installer screens go through this frame, which pads and centres their body
// so the rule above the footer lands on the same row on every one of them and
// the content sits in the middle of the rows they leave instead of against the
// header. The trainer screens cannot: they size their code window from the exact
// number of rows the chrome leaves, so they compose the same parts -- headerRow,
// rule, chip and gutteredBlock -- directly, one row per element, without
// placeBody's padding. The trainer's legends go through footerHints too, one row
// per packed line, so their keys and verbs read the same there as on every
// installer screen.

// installerHint is one footer entry: the keys that trigger an action and the
// verb that names it. The frame draws the keys in the palette's key tone and the
// verb in InkDim, so the eye finds the keys without reading the sentence. It is how every
// footer, the trainer's included, spells one hint.
type installerHint struct {
	keys string
	verb string
}

// The installer's hints, one per action, in the canonical order the previous
// slice fixed: navigation first, then the action, then the way back or out. A
// screen lists the actions it honours by name instead of retyping a sentence.
//
// The grammar these spell is fixed in odd/tasks/key-grammar.md and guarded by
// TestEveryHintObeysTheKeyGrammar: Enter opens or chooses, Space toggles a row
// that is a switch, a command runs on its own letter, Esc goes back, and a text
// field commits with Enter because its letters are its text.
var (
	hintUp     = installerHint{"↑/k", "up"}
	hintDown   = installerHint{"↓/j", "down"}
	hintSelect = installerHint{"[Enter]", "select"}
	// hintOpen and hintChoose are the two spellings of Enter's one meaning, and
	// the pass moves each screen onto the one that names what it does: hintOpen
	// for a row that leads to a screen, hintChoose for a picker whose rows are
	// values. hintSelect names neither, and it is what the screens the pass has
	// not reached yet still say.
	hintOpen   = installerHint{"[Enter]", "open"}
	hintChoose = installerHint{"[Enter]", "choose"}
	hintBack   = installerHint{"[Esc]", "back"}
	// The Utilities section's own commands. The desktop theme switch and the undo
	// act where they stand, so each runs on its own letter and carries it on its
	// row; Enter is left to the rows that open a screen.
	hintDark  = installerHint{"[d]", "dark theme"}
	hintLight = installerHint{"[l]", "light theme"}
	hintUndo  = installerHint{"[u]", "undo"}
	// The password screen's Enter is an attempt rather than a selection, so it names
	// its own pair instead of borrowing the wizard's: "select" there would say the
	// opposite of what the key does.
	hintValidate = installerHint{"[Enter]", "validate"}
	hintTerminal = installerHint{"[Esc]", "ask in the terminal"}
	hintCancel   = installerHint{"[Esc]", "cancel"}
	// hintQuit quits the whole program and hintExit ends a finished run. Both run
	// on q, their own letter: Enter opens and chooses, so it cannot also leave.
	hintQuit    = installerHint{"[q]", "quit"}
	hintExit    = installerHint{"[q]", "exit"}
	hintDetails = installerHint{"[d]", "details"}
	// hintStopRun is the installing screen's way out of a run in flight. The mockup
	// names it Cancel, and it reaches the path ctrl+c has always taken: the program
	// quits, the step stops where it is, and no record is written, because nothing
	// failed. The flow is the same one; this is the key the footer spells for it.
	hintStopRun = installerHint{"[q]", "Cancel"}
	hintPage    = installerHint{"[PgUp/PgDn]", "page"}
	hintRetry   = installerHint{"[r]", "retry"}
	// hintUpdateInstall is the update screen's own install key. Installing the
	// release writes the running binary, so it carries its own letter rather than
	// Enter; the check beside it runs on hintRefresh.
	hintUpdateInstall = installerHint{"[i]", "install"}
	// hintStart is the welcome screen's start key. Starting the wizard carries its
	// own letter for the same reason the review's does: Enter opens and chooses.
	hintStart = installerHint{"[s]", "start"}
	// The review screen's switch. It is its own key rather than Enter, because a key
	// that turns a unit off and a key that starts the run cannot be the same one.
	hintToggle = installerHint{"[Space]", "turn it off or on"}
	// hintAdjust and hintRecommended are the WSL resource screen's own keys: the
	// arrows move the value under the cursor by one step, and `r` puts every row
	// back on the host's recommendation. hintMove is the same screen's compact
	// navigation pair: it packs the two cursor hints into one so the screen's six
	// hints fit on a single footer row at 80 columns, where the separate pairs
	// wrapped and left the way back alone on a line of its own.
	hintAdjust      = installerHint{"[←/→]", "adjust"}
	hintRecommended = installerHint{"[r]", "recommended"}
	// The resource screen's two commands act where they stand -- they write the file
	// or repair the guard -- so they run on their own letters, and Enter is left to
	// the rows that open something.
	hintWriteWSL    = installerHint{"[w]", "write"}
	hintRepairGuard = installerHint{"[g]", "repair guard"}
	hintMove        = installerHint{"↑/k ↓/j", "move"}
	// hintType is the hint for a screen that has a text field: the keyboard writes
	// into the field, and the arrows move between fields, so no key can be named for
	// typing itself. It is the same shape as the trainer's "Type command".
	hintType = installerHint{"", "type"}
	// hintTab cycles the panels of a screen that offers more than one. It is only
	// ever added to a screen that has something to cycle, so the footer never
	// advertises a key that does nothing.
	hintTab = installerHint{"[Tab]", "panel"}
	// The port manager's own keys. hintFilter opens the filter field and
	// hintFilterTyping is what the footer says while the field owns the keys;
	// hintCycle names the three value cycles (transport, owner, sort), hintRefresh
	// asks for an immediate reading, hintAuto toggles the periodic one, hintStop
	// opens the confirmation and hintCopy writes the diagnostic. The stop
	// confirmation declares its own two hints beside its rows, where the rows are.
	hintFilter       = installerHint{"[/]", "filter"}
	hintFilterTyping = installerHint{"[Enter/Esc]", "keep filter"}
	hintRefresh      = installerHint{"[r]", "refresh"}
	hintAuto         = installerHint{"[p]", "auto"}
	hintStop         = installerHint{"[x]", "stop"}
	hintCopy         = installerHint{"[y]", "copy"}
	hintCycle        = installerHint{"[f/o/s]", "cycle"}
	// hintWSLTerminalOffer is the installing screen's hint while the interactive
	// WSL step waits on the Windows Terminal offer. It is one hint rather than two
	// so it always fits one footer row beside the details hint, and it spells both
	// answers because the offer writes nothing until one of them is pressed.
	hintWSLTerminalOffer = installerHint{"[y/n]", "configure Windows Terminal or leave it unchanged"}
	// hintShellAuditMeasure is the shell startup screen's own measure key. The row
	// starts the user's shell several times, so it carries a letter of its own and
	// Enter is left to the rows that open something.
	hintShellAuditMeasure = installerHint{"[s]", "measure"}
	// The theme refresh review's two answers. The confirmation writes files and the
	// cancel leaves every one of them alone, so each names its own letter instead of
	// Enter, which opens and chooses rather than applying a write.
	hintThemeRefreshApply  = installerHint{"[y]", "refresh the files"}
	hintThemeRefreshCancel = installerHint{"[n]", "leave them alone"}
	// The shell startup screen's own keys. The plan is four settings in one hint:
	// they are the same kind of decision, they are named together where the plan is
	// described, and four separate pairs would take the footer row the way back
	// needs on a narrow terminal.
	hintShellAuditPlan   = installerHint{"[m/n/w/b]", "what to measure"}
	hintShellAuditExtras = installerHint{"[h/r]", "history / recommendations"}
	// hintShellAuditStop is the Esc hint while a measurement is running, where the
	// key stops the starts that are left rather than leaving the screen.
	hintShellAuditStop = installerHint{"[Esc]", "stop the runs that are left"}
)

// helpHintSeparator is the one separator the installer footer shares with the
// notation the rest of the program uses.
const helpHintSeparator = " • "

// hintPlainWidth is the columns one hint takes: keys, the space between them,
// and the verb. A hint with no keys -- an instruction like the trainer's "Type
// command" -- is just its verb.
func hintPlainWidth(h installerHint) int {
	if h.keys == "" {
		return lipgloss.Width(h.verb)
	}
	return lipgloss.Width(h.keys) + 1 + lipgloss.Width(h.verb)
}

// renderHint draws one hint: keys in the palette's key tone, verb in InkDim. A
// hint with no keys renders as its verb alone, with no leading space for the key
// that is not there.
func renderHint(h installerHint) string {
	if h.keys == "" {
		return HelpVerbStyle.Render(h.verb)
	}
	return AccentKeyStyle.Render(h.keys) + " " + HelpVerbStyle.Render(h.verb)
}

// footerMaxRows is the most rows the frame's footer may spend, whatever a screen
// lists. It is a constant because the packing cap and a screen's own body budget
// have to agree: a screen that reserved one footer row while the footer packed
// two drew one row past the frame.
const footerMaxRows = 2

// The frame's status bar. The mockup's last row begins with a filled dot and the
// state the screen is in, and the same row carries the keys: the state is what
// the frame is doing, the keys are what the reader can do about it.
//
// The words are the mockup's five, and they are words rather than glyphs, so a
// terminal with no colour and no Nerd Font still reads the state. The dot is the
// decoration.
const (
	statusReady      = "Ready"
	statusInstalling = "Installing…"
	statusPreview    = "Preview mode"
	statusDone       = "Done"
	statusFailed     = "Failed"

	// statusDotGlyph is the dot the status bar leads with. It is one cell, so the
	// bar cannot push the hints off the row on a terminal whose font draws it wide.
	statusDotGlyph = "●"

	// statusBarGap is the columns between the state block and the key hints, so the
	// two read as the row's two ends rather than as one run of text. It is the
	// minimum: a row with room to spare spreads the two ends apart.
	statusBarGap = 1
)

// statusBar is the frame's state: the word for what the screen is doing and the
// style its dot is drawn in. A screen states the state it is in and nothing else:
// the failed run says Failed, the completed one Done, the theme picker is the
// frame's only preview, and every other page is Ready.
func (m Model) statusBar() (string, lipgloss.Style) {
	switch m.Screen {
	case ScreenInstalling:
		return statusInstalling, BrandStyle
	case ScreenThemePicker:
		return statusPreview, HighlightStyle
	case ScreenComplete:
		return statusDone, SuccessStyle
	case ScreenError:
		return statusFailed, ErrorStyle
	}
	return statusReady, SuccessStyle
}

// footerCenterStatus is the optional middle of the reference's footer: the one
// line a screen has to say about what it is doing. It is empty everywhere the
// frame has nothing to add, so the rule runs whole and no screen grows a row for
// a sentence it did not need.
func (m Model) footerCenterStatus() string {
	switch m.Screen {
	case ScreenUpdateCenter:
		return m.updateCenterFooterStatus()
	case ScreenShellAudit:
		return m.shellAuditFooterStatus()
	}
	return ""
}

// statusBarBlock draws the state block itself: the dot in the state's tone and
// the word in body ink, so the word keeps the 4.5:1 floor the tone may not.
func statusBarBlock(word string, dot lipgloss.Style) string {
	return dot.Render(statusDotGlyph) + " " + InkStyle.Render(word)
}

// footerStatusRow is the installer frame's status bar: the state block at the
// left of the rule that closes the body, with the rule carrying on after it.
//
// The state rides the rule rather than the legend's own row because the legend is
// where the frame is tightest: at 80 columns six screens' footers already fill a
// row, and a block sharing it wrapped them onto a second and took a body row --
// the review screen's "Way back" marker among them, which is the row the floor
// exists to keep. The rule has the whole width and nothing else to say, so the
// state costs the frame nothing here, and the keys keep their row and their full
// measure.
func (m Model) footerStatusRow(inner int) string {
	word, dot := m.statusBar()
	block := statusBarBlock(word, dot)
	center := m.footerCenterStatus()
	room := inner - lipgloss.Width(block) - 1
	if room < 1 {
		return block
	}
	if center == "" {
		return block + " " + rule(room)
	}
	// The middle costs two spaces around the text, so the rule and the text share
	// what is left: one column more than the room and the footer would clip the
	// hints off the frame's right edge.
	textRoom := max(1, room-2)
	if lipgloss.Width(center) > textRoom-2 {
		center = truncate(center, max(1, textRoom-2))
	}
	left := (textRoom - lipgloss.Width(center)) / 2
	if left < 1 {
		left = 1
	}
	right := textRoom - left - lipgloss.Width(center)
	if right < 1 {
		right = 1
	}
	if left+lipgloss.Width(center)+right < textRoom {
		right += textRoom - (left + lipgloss.Width(center) + right)
	}
	return block + " " + rule(left) + " " + MutedStyle.Render(center) + " " + rule(right)
}

// statusBarReserve is the columns this frame's status block takes when it shares
// a row with the key hints, as the trainer's frame does: the dot, its gap, the
// state word, and the gap that separates the block from the hints. It is measured
// from the model because the rows a footer spends are also the rows a screen's
// body budget was computed from: a budget derived from a width the footer then
// exceeded would draw one row past the frame.
func (m Model) statusBarReserve() int {
	word, _ := m.statusBar()
	return 1 + 1 + lipgloss.Width(word) + statusBarGap
}

// footerHints packs hints into at most footerMaxRows rows within inner columns,
// the rows the frame's footer may spend. The previous slice's budget is one row
// when it fits and two at 80 columns, and it never reaches three: a screen that
// would need a third has too many hints, not a taller footer.
//
// The packing width is the whole row. The installer's status bar rides the rule
// above the footer rather than the legend's own row (see footerStatusRow), so it
// costs the hints no columns and the body no rows; a footer that wrapped because
// the chrome had taken its width would lose the list row the floor is for.
func (m Model) footerHints(inner int, hints []installerHint) []string {
	if inner < 1 {
		inner = 1
	}
	var rows []string
	var row []installerHint
	width := 0
	flush := func() {
		if len(row) == 0 {
			return
		}
		parts := make([]string, len(row))
		for i, h := range row {
			parts[i] = renderHint(h)
		}
		rows = append(rows, strings.Join(parts, HelpVerbStyle.Render(helpHintSeparator)))
		row = row[:0]
		width = 0
	}
	for _, h := range hints {
		w := hintPlainWidth(h)
		if len(row) > 0 {
			w += lipgloss.Width(helpHintSeparator)
		}
		if len(row) > 0 && width+w > inner {
			flush()
			w = hintPlainWidth(h)
		}
		row = append(row, h)
		width += w
	}
	flush()
	if len(rows) > footerMaxRows {
		rows = rows[:footerMaxRows]
	}
	return rows
}

// footerRowCount is how many rows the footer spends for a screen. It is the
// same packing footerHints draws, so a screen's body budget and the rows the
// footer actually takes cannot disagree.
func (m Model) footerRowCount(inner int, hints []installerHint) int {
	// The status bar is on every frame, so a screen with no hints still spends the
	// one row that carries it.
	return max(1, len(m.footerHints(inner, hints)))
}

// legendRows packs a screen's hints into the rows the frame's footer may spend
// and moves each row to the right edge, so the keys sit under the status rule's
// end instead of starting wherever the packing happened to stop.
func (m Model) legendRows(inner int, hints []installerHint) []string {
	rows := m.footerHints(inner, hints)
	for i, row := range rows {
		if pad := inner - lipgloss.Width(row); pad > 0 {
			rows[i] = strings.Repeat(" ", pad) + row
		}
	}
	return rows
}

// footerBarRows is how many rows a footerBar spends. It is the count a screen
// that shares its legend row with the status block measures its body against.
func (m Model) footerBarRows(inner int, hints []installerHint) int {
	return max(1, len(m.footerHints(inner-m.statusBarReserve(), hints)))
}

// footerBar composes a footer whose last row shares its space with the status
// block: the block at the left, the key hints right-aligned in the rows the
// packing gave them. The trainer's frame composes its own rows and has no rule
// above its legend to carry the state, so it takes this shape; the installer's
// frame puts the state on that rule (footerStatusRow) and gives the whole width
// to the keys.
//
// A screen with no hints at all still gets its status bar, because the bar says
// what the frame is doing and not what can be pressed.
func (m Model) footerBar(inner int, hints []installerHint) []string {
	word, dot := m.statusBar()
	block := statusBarBlock(word, dot)
	width := max(1, inner-m.statusBarReserve())
	out := m.legendRows(width, hints)
	if len(out) == 0 {
		return []string{block}
	}
	out[len(out)-1] = block + strings.Repeat(" ", statusBarGap) + out[len(out)-1]
	return out
}

// installerBodyRows is the rows a screen may draw between the header rule and
// the rule above the footer: the frame minus the global top padding View()
// adds, the header row, the two rules, and the footer's own rows.
func installerBodyRows(height, footerRows int) int {
	rows := height - viewPaddingRows - 3 - footerRows
	if rows < 1 {
		rows = 1
	}
	return rows
}

// headerRow draws the frame's header: the section name in Brand on the left and
// the screen's vital sign in the meter tone on the right, justified across the
// inner width. A screen with nothing to report passes an empty vital and gets
// no filler.
//
// The header is ONE row, so the name and the vital have to share the inner width,
// with at least a one-column gap while there is room for one. The installer's
// headers are short enough that they always did; the trainer's were not -- a
// lesson's vital packs a meter, the score, the streak and a countdown -- and
// nothing capped the sum, so a 60-column terminal drew a 72- to 78-column header
// and the terminal clipped the tail silently. The vital gives way first: the name
// says which screen this is, while the vital repeats counts the body already
// carries.
func headerRow(name, vital string, inner int) string {
	return headerRowStyle(HeaderChipStyle, name, vital, inner)
}

// headerRowStyle is headerRow with the tab's fill given explicitly. The frame
// uses it to turn the completion screen's crown green; every other caller takes
// the brand tab through headerRow.
func headerRowStyle(chipStyle lipgloss.Style, name, vital string, inner int) string {
	// The name is a filled tab: two columns of the chip's own padding sit around
	// it, so the room the name may use is the inner width less that padding. The
	// chip's rendered width is measured rather than derived from the name, so the
	// vital is placed against the block that is actually drawn.
	if inner > 0 && lipgloss.Width(name) > inner-headerChipPadding {
		name = truncate(name, inner-headerChipPadding)
	}
	left := chipStyle.Render(name)
	if vital == "" {
		return left
	}
	// The vital is plain text unless the screen packed its own styled meter into
	// it. A plain vital takes the muted tone: rendered bare it inherits the
	// terminal's default ink, which is white on a light theme's background and
	// disappears in the preview. The style goes on before the width math, which
	// measures ANSI by its visible width anyway.
	if !strings.Contains(vital, "\x1b") {
		vital = MutedStyle.Render(vital)
	}

	nameWidth := lipgloss.Width(left)
	available := inner - nameWidth - 1
	if available < 0 {
		// The tab alone leaves no room for a gap, let alone a vital; the name is
		// the fact and stays, the vital is dropped.
		return left
	}
	if lipgloss.Width(vital) > available {
		vital = truncateHeaderVital(vital, available)
	}

	gap := inner - nameWidth - lipgloss.Width(vital)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + vital
}

// headerChipPadding is the columns HeaderChipStyle spends around the header
// name: one column of padding on each side. headerRow subtracts it from the room
// the name may use, so a long name cannot push the chip past the inner width.
const headerChipPadding = 2

// truncateHeaderVital shortens a header's vital to width columns without
// splitting its escape sequences, and ends with the shared cut marker in a style
// whose own reset retires whatever colour the cut left active. It is the header's
// counterpart to truncate: that one measures a string one rune at a time, which
// is safe for the plain lines a body is counted in but would split the escaped
// vital the trainer packs, so the header goes through lipgloss's truncation and
// then marks the cut. The marker carries a meter tone rather than a bare glyph,
// so its reset is always the line's last escape.
func truncateHeaderVital(vital string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(vital) <= width {
		return vital
	}
	if width <= lipgloss.Width(cutMarker) {
		return MeterStyle.Render(cutMarker)
	}
	return lipgloss.NewStyle().MaxWidth(width-lipgloss.Width(cutMarker)).Render(vital) +
		MeterStyle.Render(cutMarker)
}

// placeBody fits a screen's body into the rows the frame leaves. The body is
// centred in those rows, so a screen shorter than its frame reads as designed
// space above and below the content instead of a void between the content and
// the pinned footer. The blank rows go above and below in equal parts, with the
// odd one, if any, below: the same placement the splash has always used, so a
// screen that does not fill its rows cannot drift from it.
//
// The centring is uncapped on purpose. An earlier pass capped the margin above
// the body at six rows, because a nine-row body centred in a 57-row frame sat
// halfway down the screen; anchored to the rule instead, every screen glued its
// content to the header and left one large void above the creature, which is the
// shape the reference screens do not draw. The body is centred in the rows
// between the header and the footer now -- and above the creature's reserved
// block (placeBodyAboveCompanion), so the pet keeps the rows it always had.
//
// Centring is only safe while a body keeps its height as the cursor walks it: a
// body that grows by a row moves itself by half a row. Every screen that centres
// here draws a fixed-height block -- a windowed list, a panel card measured at the
// tallest its facts get, a card built from the state's own groups -- so the anchor
// cannot move under the cursor, and the guard in teatest_test.go walks a list to
// prove it.
//
// A body that fills or overflows the rows is left exactly as it is, and is not
// truncated: a screen that draws more rows than it reserved must fail the frame
// guard, not be quietly clipped here.
func placeBody(body []string, rows int) []string {
	top := (rows - len(body)) / 2
	out := make([]string, 0, max(len(body), rows))
	for i := 0; i < top; i++ {
		out = append(out, "")
	}
	out = append(out, body...)
	for len(out) < rows {
		out = append(out, "")
	}
	return out
}

// placeBodyAboveCompanion places a screen's body in the rows above the
// companion's reserved block, leaving the last `reserve` rows blank for the
// creature and its summary. A screen that reserves those rows centres its body in
// the rows the block leaves, so the content is centred between the header and the
// creature rather than between the header and the footer; a body too tall for the
// rows left is left where placeBody put it, because a body that does not fit
// cannot be made to fit by moving it. The creature's own row does not move either
// way -- it is always the last rows of the frame -- so the block stays a function
// of the terminal and not of content.
func placeBodyAboveCompanion(body []string, rows, reserve int) []string {
	if reserve <= 0 || len(body) > rows-reserve {
		return placeBody(body, rows)
	}
	out := placeBody(body, rows-reserve)
	return append(out, make([]string, reserve)...)
}

// composeColumns places a screen's body in the left column and the panel it was
// given in the right one, the gutter columns apart, and indents the whole
// composition by the leading margin, so a terminal wider than the composition
// cap gets two symmetric margins instead of one strip of void on the right.
//
// Every line it returns is exactly Inner columns wide -- the leading margin, the
// left column, the gutter, the right column and the matching right margin -- so
// the composed body covers the same columns as the frame's rules above and below
// it and the odd column, when the margins cannot be equal, falls to the right.
// Nothing in the left column can reach under the panel: a line longer than its
// column is cut by the shared truncate, with its marker, rather than allowed to
// cross the gutter and collide with the column beside it.
func composeColumns(body, panel []string, l layout) []string {
	rows := max(len(body), len(panel))
	out := make([]string, rows)
	indent := strings.Repeat(" ", l.Leading)
	gutter := strings.Repeat(" ", layoutGutter)
	for i := 0; i < rows; i++ {
		left, right := "", ""
		if i < len(body) {
			left = body[i]
		}
		if i < len(panel) {
			right = panel[i]
		}
		out[i] = padRight(indent+
			padRight(truncate(left, l.Left), l.Left)+gutter+
			padRight(truncate(right, l.Right), l.Right), l.Inner)
	}
	return out
}

// centeredBody indents a one-column body to the layout's leading margin and cuts
// it to the composition. The two-column pages are composed with the same margin
// by composeColumns; without this a single-column page hugged the frame's left
// edge while the pages beside it were centred, so the same terminal had two left
// edges (ports and setup at column 2, LazyVim at column 20).
func centeredBody(lines []string, l layout) []string {
	if l.Leading <= 0 {
		return lines
	}
	indent := strings.Repeat(" ", l.Leading)
	out := make([]string, len(lines))
	for i, line := range lines {
		if line == "" {
			out[i] = ""
			continue
		}
		out[i] = indent + truncate(line, l.Composition)
	}
	return out
}

// frame wraps a screen body in the persistent frame, asking the registry what
// panels the screen offers. The header stays on the top row and the footer on the
// bottom one, and the body is centred in the rows between them: a short screen
// and a full one put their header and their help on the same rows, and neither
// leaves a void above the footer.
func (m Model) frame(name, vital string, body []string, hints []installerHint) string {
	return m.frameWithPanels(name, vital, body, hints, m.panelsFor())
}

// frameWithPanels is frame with the screen's panels given explicitly. Every
// panel decision lives here, so a test can render a screen against a list the
// shipped registry does not yet build -- two panels on one screen -- without a
// second copy of the placement arithmetic.
//
// Where there is room for two columns the active panel is composed beside the
// body; below that floor the active panel collapses to a one-line summary in the
// rows the body did not need. A screen that offers no panel -- everything but the
// welcome, the main menu and the wizard's own questions -- takes the exact path
// it took before the registry existed.
func (m Model) frameWithPanels(name, vital string, body []string, hints []installerHint, panels []panel) string {
	l := layoutFor(m)
	inner := l.Inner
	hints = m.panelHints(panels, hints)
	footer := m.legendRows(inner, hints)
	// The body is measured against the footer's RESERVED rows, not the rows it
	// draws: an action that appears with a selection must not take its row from
	// the body, because taking a row from the body is what moved the list. The
	// reservation is drawn last, so the frame's last row is still the legend.
	footerRows := m.footerReserveRows(inner, hints)
	rows := installerBodyRows(m.Height, footerRows)

	// The panel summary of a narrow terminal and the companion both live in the
	// rows the body did not need, and they are placed together so neither can
	// displace the other or a body row. The summary keeps its rows first -- the
	// facts beat a decoration -- and the companion takes the rows nearest the
	// footer that are left, at the rung the terminal selected: a screen whose body
	// fills its frame shows no companion at all, exactly as it shows no summary,
	// and a screen with one row to spare shows the facts rather than the creature.
	var summary []string
	if !l.TwoColumn && len(panels) > 1 {
		summary = m.rotatorLines(panels, inner)
	}
	// The end-of-run burst shares the rows nobody needed, above the creature and
	// below any summary of the panel. It is decoration, so it comes after the
	// facts and is dropped entirely when the frame leaves it no room.
	if burst := m.celebrationRows(inner); len(burst) > 0 {
		summary = append(summary, burst...)
	}

	// The creature's block is reserved before the panel and the body are placed.
	// The rung comes from the terminal, not from the rows this screen happens to
	// leave, so a body centred through the block would push the creature off a
	// frame that can hold it. Reserving the block lets the panel yield instead: a
	// panel taller than the rows left for it clips with its own count, and the
	// body is placed above the block rather than through it. Only a screen that
	// draws the plan panel reserves it -- the plan is the panel whose rows and the
	// creature compete for the same frame, and the plan yields rather than the
	// creature -- so the machine, live and tip panels keep their own placement.
	sprite := m.companionSprite(inner, m.companionHeightNow())
	reserve := 0
	if len(sprite) > 0 && panelsOfferPlan(panels) {
		reserve = len(sprite) + len(summary)
	}

	if l.TwoColumn && len(panels) > 0 {
		budget := rows
		if reserve > 0 && rows-reserve > 0 {
			budget = rows - reserve
		}
		body = composeColumns(body, m.panelColumn(panels, l, budget), l)
	}

	// The body is centred in the rows between the header and the footer -- and,
	// where the creature's block is reserved, in the rows above that block, so the
	// pet keeps the rows the terminal gave it. A screen that reserves nothing keeps
	// the whole frame between the rules: placeBody splits the surplus above and
	// below the body, so a short screen reads as designed space instead of a void
	// above the creature.
	placed := placeBody(body, rows)
	if reserve > 0 {
		placed = placeBodyAboveCompanion(body, rows, reserve)
	}
	placed = m.placeCompanion(placed, summary, inner)

	// The build label rides every frame's vital sign. It used to be the main menu's
	// alone; a report asks for it first, and one helper puts it on every page
	// without a screen having to remember it.
	vital = frameVital(vital)

	var b strings.Builder
	chipStyle, ruleStyle := HeaderChipStyle, HeaderRuleStyle
	if m.Screen == ScreenComplete {
		// The run's own verdict is the screen's subject, so its crown wears the
		// success tone rather than the brand one: the completion screen is the one
		// frame the installer is allowed to make celebratory.
		chipStyle, ruleStyle = HeaderChipSuccessStyle, HeaderRuleSuccessStyle
	}
	b.WriteString(headerRowStyle(chipStyle, name, vital, inner))
	b.WriteString("\n")
	b.WriteString(headerRuleStyle(ruleStyle, inner))
	b.WriteString("\n")
	for _, line := range placed {
		b.WriteString(line)
		b.WriteString("\n")
	}
	// The rule above the legend is the frame's status bar: the state at its left,
	// the rule carrying on to the edge. It draws the row the rule always drew, so
	// the chrome costs the body nothing.
	b.WriteString(m.footerStatusRow(inner))
	b.WriteString("\n")
	// The rows the footer reserved but did not need are left between the rule and
	// the legend, so the rule, the body above it and the legend's last row all
	// keep the place the reservation gave them.
	for i := len(footer); i < footerRows; i++ {
		b.WriteString("\n")
	}
	b.WriteString(strings.Join(footer, "\n"))
	return b.String()
}

// headerName is the app or section name the frame's header carries. It is short
// on purpose: the screen's own title still names the screen in the body, and a
// header that repeated it would be a second label row for the same fact.
func (m Model) headerName() string {
	switch m.Screen {
	case ScreenWelcome:
		return "dotfiles"
	case ScreenMainMenu:
		return "Main Menu"
	case ScreenOSSelect, ScreenTerminalSelect, ScreenFontSelect,
		ScreenShellSelect, ScreenWMSelect, ScreenNvimSelect, ScreenGhosttyWarning,
		ScreenInstallReview, ScreenSetup:
		return "Setup"
	case ScreenInstalling:
		// The mockup's word for the screen, and the same word the status bar under it
		// uses, so the header says what the frame is doing instead of repeating the
		// product name.
		return "Installing"
	case ScreenAdminPassword:
		return "Administrator access"
	case ScreenComplete:
		return "Installation complete"
	case ScreenError:
		return "Installation failed"
	case ScreenLearnTerminals, ScreenLearnShells, ScreenLearnWM, ScreenLearnNvim, ScreenLearnTools, ScreenLearnFont, ScreenLearnPrompts, ScreenLearnTooling, ScreenLearnPlugins, ScreenLearnToolset:
		return "Learn"
	case ScreenKeymaps, ScreenKeymapCategory, ScreenKeymapsMenu,
		ScreenKeymapsTmux, ScreenKeymapsTmuxCat, ScreenKeymapsZellij,
		ScreenKeymapsZellijCat, ScreenKeymapsGhostty, ScreenKeymapsGhosttyCat,
		ScreenKeymapsHerdr, ScreenKeymapsHerdrCat, ScreenKeymapsKitty,
		ScreenKeymapsKittyCat, ScreenKeymapsWezTerm, ScreenKeymapsAlacritty:
		return "Keymaps"
	case ScreenLearnLazyVim, ScreenLazyVimTopic:
		return "LazyVim"
	case ScreenVimHub:
		// The hub is the one place the main menu reaches the trainer, the keymaps and
		// the guide from, so its header names the subject rather than a section.
		return "Vim"
	case ScreenBackupConfirm, ScreenRestoreBackup, ScreenRestoreConfirm:
		return "Backups"
	case ScreenUtilities:
		return "Utilities"
	case ScreenWSLResources, ScreenTerminalCapabilities, ScreenThemePicker:
		// These screens are one level in from the utilities section, and their
		// header says so rather than reading as a section of their own. The theme
		// picker is the same level in as the WSL and capability screens, so it names
		// the same section instead of falling back to the program name.
		return "Utilities"
	case ScreenShellAudit, ScreenWSLAdvisor, ScreenWSLBenchmark, ScreenWSLMigration:
		// The shell's startup screen, the performance advisor and its two assistants
		// are all one level in from the same section, so they name it rather than
		// reading as sections of their own.
		return "Utilities"
	case ScreenInstallVerify:
		// The verification screen is one level in from the same section.
		return "Utilities"
	case ScreenNetworkDiagnostics:
		// The diagnosis screen is one level in from the same section.
		return "Utilities"
	case ScreenUpdateCenter:
		// The update center is one level in from the same section, so it names the
		// section rather than a heading of its own.
		return "Utilities"
	case ScreenSSHManager, ScreenSSHHost, ScreenSSHForm, ScreenSSHKeys,
		ScreenSSHAgent, ScreenSSHDiagnostics, ScreenSSHTest:
		// The SSH manager is the section's own job: its hub is one level in, like
		// the other utility screens, and everything it opens stays under the same
		// heading. A header that changed as the user walked into a host's detail
		// would name a section that does not exist in the menu.
		return "Utilities"
	case ScreenDiskCleaner:
		// The disk cleaner is one level in from the same section.
		return "Utilities"
	case ScreenUpdate:
		// The update screen is one level in from the same section.
		return "Utilities"
	case ScreenPortManager, ScreenPortStop, ScreenDoctor, ScreenDoctorFix:
		// All four are one level in from the utilities section, exactly as the
		// shell audit and the update screen are: the port screens belong to that
		// section's own row, and so do the doctor and its confirmation.
		return "Utilities"
	default:
		return "dotfiles"
	}
}

// wizardProgress is the current step and the number of steps the wizard shows
// for this run. The meter counts the two mandatory screens: the setup screen,
// where every decision is a row, and the plan. The rich per-question screens are
// optional detours of the setup screen now, so they inherit its meter rather
// than claiming a step of their own; a screen that is not part of the wizard (the
// main menu, the backup confirmation screen the flow no longer walks through)
// reports 0/0 so its header gets no filler meter.
func (m Model) wizardProgress() (current, total int) {
	switch m.Screen {
	case ScreenSetup:
		return 1, 2
	case ScreenInstallReview:
		return 2, 2
	case ScreenOSSelect, ScreenTerminalSelect, ScreenFontSelect,
		ScreenShellSelect, ScreenWMSelect, ScreenNvimSelect, ScreenGhosttyWarning:
		// A detour inherits the meter of the screen it returns to, which is the
		// setup screen and the first of the two mandatory screens.
		return 1, 2
	}
	return 0, 0
}

// meterRendered draws a progress meter of cells with filled of total done: the
// filled cells in brand chrome, the empty ones in the dim tone, and the count
// after it. The filled cells are glyphs, so the meter still reads on a terminal
// with no colour at all.
func meterRendered(filled, total, cells int) string {
	if total < 1 {
		total = 1
	}
	if filled < 0 {
		filled = 0
	}
	if filled > total {
		filled = total
	}
	if cells < 1 {
		cells = 1
	}
	n := int(math.Round(float64(filled) / float64(total) * float64(cells)))
	if n > cells {
		n = cells
	}
	if n < 0 {
		n = 0
	}
	return MeterFilledStyle.Render(strings.Repeat("▓", n)) +
		MeterStyle.Render(strings.Repeat("░", cells-n)) +
		MeterStyle.Render(fmt.Sprintf(" %d/%d", filled, total))
}

// meterBar draws the bar alone, without a count: the trailing meter a row that
// has progress carries. The filled cells are brand chrome and the empty ones are
// the dim tone, and both are glyphs, so the bar reads on a terminal with no
// colour.
func meterBar(fraction float64, cells int) string {
	if cells < 1 {
		cells = 1
	}
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}
	n := int(math.Round(fraction * float64(cells)))
	if n > cells {
		n = cells
	}
	if n < 0 {
		n = 0
	}
	return MeterFilledStyle.Render(strings.Repeat("▓", n)) +
		MeterStyle.Render(strings.Repeat("░", cells-n))
}

// meterCellsPlain is meterBar's glyph run with no style on it. The shared rowBar
// paints its own trailing meter in one tone -- InkDim, or the bar's own tone on a
// selected row -- so a meter it is handed has to arrive plain; a nested style
// would fight the bar's background. The fill is still readable because it is
// carried by the ▓/░ glyphs rather than by the colour.
func meterCellsPlain(fraction float64, cells int) string {
	if cells < 1 {
		cells = 1
	}
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}
	n := int(math.Round(fraction * float64(cells)))
	if n > cells {
		n = cells
	}
	if n < 0 {
		n = 0
	}
	return strings.Repeat("▓", n) + strings.Repeat("░", cells-n)
}

// scrollVital is the header's vital sign for a screen that is showing part of a
// list: which rows of how many are on screen. It carries the same "Showing" /
// "Lines" wording the body's old scroll row did, so the scroll reachability
// tests read the range back out of the header.
func scrollVital(label string, first, last, total int) string {
	return MeterStyle.Render(fmt.Sprintf("%s %d-%d of %d", label, first, last, total))
}

// chip renders a section label in brand bold: the head of a block.
func chip(label string) string {
	return BrandStyle.Render(label)
}

// blockGutterWidth is the columns the "│ " gutter gutteredBlock draws takes.
// A caller that measures text against the frame has to subtract it, or the
// guttered line is two columns wider than the budget it was measured against.
const blockGutterWidth = 2

// gutteredBlock puts a Rule-coloured │ gutter in front of a block's lines, so
// the lines read as one object under their chip instead of as loose rows. No
// box is drawn: the frame already spends the rows a border would cost.
func gutteredBlock(lines []string) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = RuleStyle.Render("│ ") + line
	}
	return out
}

// panelFrame is the columns a panel spends on its left and right border: one
// each, and panelPad is the blank column each border keeps from its content, so a
// row never touches the edge. A caller that measures text against a panel's
// inner width subtracts panelFrame + panelPad*2.
const (
	panelFrame = 2
	panelPad   = 1
)

// panelBlock wraps lines in a rounded box and sets title into the top border. It
// is the frame's inner edge: the header rule and the footer rule bound the
// screen, and a panel bounds the block inside it, so a body reads as a card
// rather than as rows floating in the void.
//
// The edges carry the theme's border role and the title is rendered by the
// caller in the tone the block's subject earns, while the box glyphs carry the
// shape on a terminal with no colour, so nothing here is invented. Each line is
// clipped to the panel's inner width with lipgloss's ANSI-aware width, so a
// caller passes rows already measured for the screen and the border replaces the
// columns the clip frees. The title may be styled: its width is measured, not
// its byte length.
func panelBlock(title string, width int, lines []string) []string {
	if width < panelFrame+panelPad*2+2 {
		width = panelFrame + panelPad*2 + 2
	}
	// One column of padding inside each border, so a row never touches the edge.
	content := width - panelFrame - panelPad*2
	edge := func(s string) string { return PanelBorderStyle.Render(s) }
	fill := func(s string) string { return " " + padRight(s, content) + " " }

	out := make([]string, 0, len(lines)+2)
	if title == "" {
		out = append(out, edge("╭"+strings.Repeat("─", content+panelPad*2)+"╮"))
	} else {
		label := " " + title + " "
		dash := content + 1 - lipgloss.Width(label)
		if dash < 1 {
			// The title is wider than the panel: cut it so the box still closes,
			// leaving at least one rule cell after it. MaxWidth is the ANSI-aware
			// clip, because the title may be styled.
			label = " " + lipgloss.NewStyle().MaxWidth(content-2).Render(title) + " "
			dash = 1
		}
		out = append(out, edge("╭─")+label+edge(strings.Repeat("─", dash)+"╮"))
	}
	for _, line := range lines {
		clipped := lipgloss.NewStyle().MaxWidth(content).Render(line)
		out = append(out, edge("│")+fill(clipped)+edge("│"))
	}
	out = append(out, edge("╰"+strings.Repeat("─", content+panelPad*2)+"╯"))
	return out
}

// panelBlockGrouped draws the frame's card with one titled divider between its
// groups: a menu of jobs reads as sections instead of a flat list of screens.
// The edge, the padding and the clipping are panelBlock's, so the two cards are
// the same object drawn with and without dividers, and a divider fits the width
// by measuring its styled label rather than its byte length.
func panelBlockGrouped(title string, width int, groups []mainMenuGroup) []string {
	if width < panelFrame+panelPad*2+2 {
		width = panelFrame + panelPad*2 + 2
	}
	content := width - panelFrame - panelPad*2
	edge := func(s string) string { return PanelBorderStyle.Render(s) }
	fill := func(s string) string { return " " + padRight(s, content) + " " }
	inner := content + panelPad*2

	out := make([]string, 0, len(groups)*2+2)
	if title == "" {
		out = append(out, edge("╭"+strings.Repeat("─", inner)+"╮"))
	} else {
		label := " " + title + " "
		dash := content + 1 - lipgloss.Width(label)
		if dash < 1 {
			label = " " + lipgloss.NewStyle().MaxWidth(content-2).Render(title) + " "
			dash = 1
		}
		out = append(out, edge("╭─")+label+edge(strings.Repeat("─", dash)+"╮"))
	}
	for at, group := range groups {
		if at > 0 {
			label := " " + BrandStyle.Render(group.Title) + " "
			dash := content + 1 - lipgloss.Width(label)
			if dash < 1 {
				label = " " + lipgloss.NewStyle().MaxWidth(content-2).Render(group.Title) + " "
				dash = 1
			}
			out = append(out, edge("├─")+label+edge(strings.Repeat("─", dash)+"┤"))
		}
		for _, line := range group.Lines {
			// A two-line row is one logical row but two physical ones; the card
			// borders each, so the subtitle cannot escape the box.
			for _, physical := range strings.Split(line, "\n") {
				out = append(out, edge("│")+fill(lipgloss.NewStyle().MaxWidth(content).Render(physical))+edge("│"))
			}
		}
	}
	out = append(out, edge("╰"+strings.Repeat("─", inner)+"╯"))
	return out
}

// rowBar renders one menu row. The selected row is a bar with the ▸ in Accent
// and the text in Paper on BrandSoft; the unselected row is the same text in Ink
// at the SAME indent, so state comes from the bar and the weight rather than
// from an indent that moves the text. A row may carry a trailing meter for
// anything with progress.
//
// The row is measured, not stretched: it runs the layout's RowMeasure columns,
// which is the whole room up to the reading cap and the left column when there
// are two. A 227-column terminal used to get a 227-column slab of bar behind
// twenty characters of text; the bar now ends where the row does, and a
// trailing meter sits at the right edge of that measure. At the 80x24 floor the
// measure is the content width, so the rows are byte-for-byte what they were.
func (m Model) rowBar(label string, selected bool, meterPlain string) string {
	return m.rowBarWidth(label, selected, meterPlain, m.rowMeasure())
}

// rowMeasure is the columns a row may use on this screen. A screen with a right
// column (a panel or the section's detail pane) gives its rows the left column;
// a screen with neither gives them the whole composition. The fixed row measure
// used to be the left column for every two-column layout, which truncated the
// ports table at half the frame while the other half stayed empty.
func (m Model) rowMeasure() int {
	l := layoutFor(m)
	if !l.TwoColumn {
		return l.RowMeasure
	}
	if len(m.panelsFor()) > 0 || m.sectionDetailPane() {
		return l.Left
	}
	return l.Composition
}

// rowBarWidth is rowBar with the measure given explicitly, for the screens whose
// rows are not the installer's lists (the trainer's menu draws its own frame).
func (m Model) rowBarWidth(label string, selected bool, meterPlain string, measure int) string {
	const markerWidth = 2
	meterWidth := 0
	if meterPlain != "" {
		meterWidth = 1 + lipgloss.Width(meterPlain)
	}
	textWidth := measure - markerWidth - meterWidth
	if textWidth < 1 {
		textWidth = 1
	}
	label = truncate(label, textWidth)
	pad := textWidth - lipgloss.Width(label)

	if selected {
		row := RowBarMarkerStyle.Render("▸ ")
		row += RowBarStyle.Render(label + strings.Repeat(" ", pad))
		if meterPlain != "" {
			row += MeterOnBarStyle.Render(" " + meterPlain)
		}
		return row
	}
	row := InkStyle.Render("  " + label + strings.Repeat(" ", pad))
	if meterPlain != "" {
		row += MeterStyle.Render(" " + meterPlain)
	}
	return row
}

// viewPaddingRows is the one blank row the global padding in View() adds above
// every screen. A screen that sizes itself to the frame subtracts it, so its own
// content plus the padding fills the terminal exactly and no more: the welcome
// screen used to center itself in the full height and then lose its last row to
// this padding.
const viewPaddingRows = 1

// viewPaddingCols is the columns that same padding adds on each side, and it is
// named for the same reason the row count is: the companion's pointer turns a
// mouse column into a column of the stage by subtracting it, so the number the
// padding applies and the number the pointer subtracts have to be one number.
const viewPaddingCols = 2

// contentWidth is the columns a screen can render in: the model width minus the
// two columns of left and right padding View() applies to every screen.
func contentWidth(m Model) int {
	inner := m.Width - 4
	if inner < 20 {
		inner = 20
	}
	return inner
}

// listWindow returns the bounds of a window of at most rows entries that keeps
// selected visible. It CENTRES the window on selected, so the value it takes is
// a position inside the list (a cursor), not a top-row offset. Lists sized to
// their frame use it so a long list scrolls instead of running off the bottom of
// the terminal, and so the entry under the cursor is always one of the rows on
// screen. Screens whose stored scroll value is itself the top row must use
// offsetWindow instead, and clamp that value with offsetWindowMax.
func listWindow(selected, rows, total int) (start, end int) {
	if rows < 1 {
		rows = 1
	}
	if total <= rows {
		return 0, total
	}
	start = selected - rows/2
	if start < 0 {
		start = 0
	}
	if start > total-rows {
		start = total - rows
	}
	return start, start + rows
}

// offsetWindow returns the bounds of a window of at most rows entries whose TOP
// ROW is offset. The value it takes is a top-row offset, not a centred cursor:
// offset 0 shows the first row and offsetWindowMax shows the last, so the tail of
// a long list is reachable. It is the counterpart to listWindow; the keymap
// tables and the LazyVim topic render through it and their handlers clamp the
// same scroll value with offsetWindowMax, so the window a screen draws and the
// bound its keys enforce cannot disagree.
func offsetWindow(offset, rows, total int) (start, end int) {
	if rows < 1 {
		rows = 1
	}
	if offset > total-rows {
		offset = total - rows
	}
	if offset < 0 {
		offset = 0
	}
	end = offset + rows
	if end > total {
		end = total
	}
	return offset, end
}

// offsetWindowMax is the greatest top-row offset offsetWindow accepts: the one
// that puts the last row on screen. A scrollable screen's key handler clamps its
// scroll value to it, and it is derived from the same rows count as the window,
// so the last entry is reachable instead of stopping rows/2 short of the end.
func offsetWindowMax(rows, total int) int {
	if rows < 1 {
		rows = 1
	}
	if max := total - rows; max > 0 {
		return max
	}
	return 0
}

// menuRows renders a menu's options: one row per choice, the cursor marked with
// ▸, and a separator rendered as a rule at the same measure as the rows. Every
// menu in the TUI goes through it, so the marker, the cursor style and the
// divider cannot drift from screen to screen, and the divider follows the
// measure the rows are drawn in instead of running wider than the list it
// groups. Each option is a rowBar, so the selected row is a bar across the
// measure and the unselected rows keep the same indent.
func (m Model) menuRows(options []string, cursor int) []string {
	rows := make([]string, 0, len(options))
	for i := range options {
		rows = append(rows, m.menuRowAt(options, i, cursor))
	}
	return rows
}

// menuRowAt draws one row of a menu: a separator as a rule, a row struck by the
// main menu's gag as the gag (in the row's own slot, at the row's own width), and
// every other row as a bar. It is one function because the grouped main-menu card
// and the plain menu list both draw the same rows, and a gag that only one of them
// honoured would be a decoration that disappears from the screen it belongs to.
func (m Model) menuRowAt(options []string, i, cursor int) string {
	opt := options[i]
	if strings.HasPrefix(opt, menuSeparatorPrefix) {
		return rule(m.rowMeasure())
	}
	if m.Screen == ScreenMainMenu && m.MenuGagActive && i == m.MenuGagRow {
		// The gag occupies the original menu row exactly: its slot and bar width
		// are unchanged, and its small puff is a pure function of the model's tick
		// state. With animation gated off, show one composed pose.
		puff := "////   .  o  ."
		if m.Animating {
			poses := [...]string{"////   .  o  .", "////    o  .", "////     ."}
			pose := m.MenuGagTicks / 2
			if pose >= len(poses) {
				pose = len(poses) - 1
			}
			puff = poses[pose]
		}
		return m.rowBar(puff, i == cursor, "")
	}
	return m.rowBar(opt, i == cursor, "")
}

// wrappedSectionRows draws a section's entries with their long facts wrapped onto
// a second line instead of cut. It is used only where the frame has the rows for
// the taller list: the caller measures the result against the list band and keeps
// the one-line rows when it does not fit. Separators remain rules; a selected
// entry's first line is the filled bar and its continuation stays on the fill,
// so the selection reads as one object however many lines it spends.
func (m Model) wrappedSectionRows(options []string, cursor, width int) ([]string, bool) {
	rows := make([]string, 0, len(options))
	for i, option := range options {
		if strings.HasPrefix(option, menuSeparatorPrefix) {
			rows = append(rows, rule(width))
			continue
		}
		textWidth := max(1, width-2)
		lines := wrapText(option, textWidth, 2)
		if len(lines) <= 1 {
			rows = append(rows, m.rowBarWidth(option, i == cursor, "", width))
			continue
		}
		for j, line := range lines {
			padded := padRight(truncate(line, textWidth), textWidth)
			switch {
			case i == cursor && j == 0:
				rows = append(rows, RowBarMarkerStyle.Render("▸ ")+RowBarStyle.Render(padded))
			case i == cursor:
				rows = append(rows, OnFillMutedStyle.Render("  "+padded))
			case j == 0:
				rows = append(rows, InkStyle.Render("  "+padded))
			default:
				rows = append(rows, MutedStyle.Render("  "+padded))
			}
		}
	}
	return rows, true
}

// listRows renders at most rows entries of a list, followed by a note naming the
// entries the frame could not show, so the note is part of the row budget and a
// long list degrades out loud instead of running off the bottom of the screen.
func listRows(entries []string, prefix string, rows, width int, style lipgloss.Style) []string {
	if rows < 1 {
		rows = 1
	}
	shown := len(entries)
	if shown > rows {
		shown = rows - 1
	}
	out := make([]string, 0, rows)
	for _, entry := range entries[:shown] {
		out = append(out, style.Render(truncate(prefix+entry, width)))
	}
	if hidden := len(entries) - shown; hidden > 0 {
		out = append(out, MutedStyle.Render(fmt.Sprintf("     … and %d more", hidden)))
	}
	return out
}

// wrappedListRows is listRows for a list of sentences: each entry is wrapped to
// the width it is drawn at instead of being cut, and the budget counts the rows
// the wrapped text really spends. The rows the budget could not show are named in
// one note, exactly as the cut version names them.
//
// It exists for the pros and cons of a tool page: those bullets are the reasons a
// reader opened the page, and they are the longest strings on it, so the frame at
// its floor cut every one of them mid-sentence ("The other AI plugins (Copilot,
// Avante, OpenCode, Claude Code, CodeCompa…"). A list of reasons that cannot be
// read is a list of facts the screen dropped; wrapping spends the same rows on the
// same words and keeps them.
func wrappedListRows(entries []string, prefix string, rows, width int, style lipgloss.Style) []string {
	if rows < 1 {
		rows = 1
	}
	prefixWidth := lipgloss.Width(prefix)
	indent := strings.Repeat(" ", prefixWidth)
	out := make([]string, 0, rows)
	shown := 0
	for i, entry := range entries {
		lines := wrapText(entry, width-prefixWidth, 0)
		// An entry is drawn whole or not at all, and while there are entries behind
		// this one the note that counts them keeps a row of its own: an entry that
		// spends the last row and pushes the note off would hide the rest silently.
		reserve := 0
		if i < len(entries)-1 {
			reserve = 1
		}
		if len(out)+len(lines)+reserve > rows {
			break
		}
		for j, line := range lines {
			if j == 0 {
				out = append(out, style.Render(truncate(prefix+line, width)))
				continue
			}
			out = append(out, style.Render(truncate(indent+line, width)))
		}
		shown++
	}
	if hidden := len(entries) - shown; hidden > 0 {
		out = append(out, MutedStyle.Render(truncate(indent+fmt.Sprintf("… and %d more", hidden), width)))
	}
	return out
}

const logo = `
     ░▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓░     
    ▒███████████████████▒    
  ▒███████████████████████▒  
 ▒▓▓▒▒▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▒▒▓▓▒ 
 ░▓██ ▒███████████████▒ ██▓░ 
   ▒██░▒█████████████▒░██▒   
     ▓█░░███████████░░█▓     
      ▒█░░█████████░░█▒      
        ▓▒░███████░▒▓        
         ░▒ ▓███▓ ▒░         
           ░▓███▓░           
            ░███░            
              ▒              
`

const compactLogo = `
   ░▓▓▓▓▓▓▓▓▓▓▓░   
 ░▓█████████████▓░ 
 ▓▓░▓▓▓▓▓▓▓▓▓▓▓░▓▓ 
 ░▓░░█████████░░▓░ 
   ▒▒░███████░▒▒   
    ░▒ ▓███▓ ▒░    
      ░ ▓█▓ ░      
        ▓█▓        
         ▒         
`

const dotfilesText = `
██████╗   ██████╗  ████████╗ ███████╗ ██╗ ██╗      ███████╗ ███████╗
██╔══██╗ ██╔═══██╗ ╚══██╔══╝ ██╔════╝ ██║ ██║      ██╔════╝ ██╔════╝
██║  ██║ ██║   ██║    ██║    █████╗   ██║ ██║      █████╗   ███████╗
██║  ██║ ██║   ██║    ██║    ██╔══╝   ██║ ██║      ██╔══╝   ╚════██║
██████╔╝ ╚██████╔╝    ██║    ██║      ██║ ███████╗ ███████╗ ███████║
╚═════╝   ╚═════╝     ╚═╝    ╚═╝      ╚═╝ ╚══════╝ ╚══════╝ ╚══════╝
`

// View implements tea.Model
func (m Model) View() string {
	if m.Quitting {
		return ""
	}

	// The chrome the model says the run wears. An applied theme is derived from the
	// record and the definitions every time rather than kept as a second palette, so
	// the record stays the one place the applied theme lives and cannot drift. It is
	// only painted on a terminal that can show a 24-bit palette; the default chrome
	// is restored before View returns, so a run with no theme applied renders
	// byte-for-byte the default and no palette leaks into the next render.
	if uiThemeRepaintAllowed(lipgloss.ColorProfile()) {
		if colors, ok := m.appliedThemeColors(); ok {
			previous := currentUIColors
			applyUIColors(colors)
			defer applyUIColors(previous)
		}
	}

	// The live preview is a layer on top of that chrome: it repaints the whole
	// interface in the theme under the cursor and restores what was underneath
	// before View returns. It writes nothing, so a screen with no preview active is
	// byte-for-byte the chrome the run wears.
	if def, ok := m.previewThemeDef(); ok {
		if restore, err := applyPreviewTheme(def); err == nil {
			defer restore()
		}
	}

	var s strings.Builder

	switch m.Screen {
	case ScreenWelcome:
		s.WriteString(m.renderWelcome())
	case ScreenMainMenu:
		s.WriteString(m.renderMainMenu())
	case ScreenOSSelect, ScreenTerminalSelect, ScreenFontSelect, ScreenShellSelect, ScreenWMSelect, ScreenNvimSelect, ScreenGhosttyWarning:
		s.WriteString(m.renderSelection())
	case ScreenSetup:
		s.WriteString(m.renderSetup())
	case ScreenInstallReview:
		s.WriteString(m.renderInstallReview())
	case ScreenLearnTerminals:
		s.WriteString(m.renderLearnTerminals())
	case ScreenLearnShells:
		s.WriteString(m.renderLearnShells())
	case ScreenLearnWM:
		s.WriteString(m.renderLearnWM())
	case ScreenLearnNvim:
		s.WriteString(m.renderLearnNvim())
	case ScreenLearnTools:
		s.WriteString(m.renderLearnTools())
	case ScreenVimHub:
		s.WriteString(m.renderVimHub())
	case ScreenLearnFont:
		s.WriteString(m.renderLearnFont())
	case ScreenLearnPrompts:
		s.WriteString(m.renderLearnPrompts())
	case ScreenLearnTooling:
		s.WriteString(m.renderLearnTooling())
	case ScreenLearnPlugins:
		s.WriteString(m.renderLearnPlugins())
	case ScreenLearnToolset:
		s.WriteString(m.renderLearnToolset())
	case ScreenKeymaps:
		s.WriteString(m.renderKeymapsMenu())
	case ScreenKeymapCategory:
		s.WriteString(m.renderKeymapCategory())
	case ScreenKeymapsMenu:
		s.WriteString(m.renderToolKeymapsMenu())
	case ScreenKeymapsTmux:
		s.WriteString(m.renderTmuxKeymapsMenu())
	case ScreenKeymapsTmuxCat:
		s.WriteString(m.renderTmuxKeymapCategory())
	case ScreenKeymapsZellij:
		s.WriteString(m.renderZellijKeymapsMenu())
	case ScreenKeymapsZellijCat:
		s.WriteString(m.renderZellijKeymapCategory())
	case ScreenKeymapsGhostty:
		s.WriteString(m.renderGhosttyKeymapsMenu())
	case ScreenKeymapsGhosttyCat:
		s.WriteString(m.renderGhosttyKeymapCategory())
	case ScreenKeymapsHerdr:
		s.WriteString(m.renderHerdrKeymapsMenu())
	case ScreenKeymapsHerdrCat:
		s.WriteString(m.renderHerdrKeymapCategory())
	case ScreenKeymapsKitty:
		s.WriteString(m.renderKittyKeymapsMenu())
	case ScreenKeymapsKittyCat:
		s.WriteString(m.renderKittyKeymapCategory())
	case ScreenKeymapsWezTerm:
		s.WriteString(m.renderWezTermReference())
	case ScreenKeymapsAlacritty:
		s.WriteString(m.renderAlacrittyReference())
	case ScreenLearnLazyVim:
		s.WriteString(m.renderLazyVimMenu())
	case ScreenLazyVimTopic:
		s.WriteString(m.renderLazyVimTopic())
	case ScreenBackupConfirm:
		s.WriteString(m.renderBackupConfirm())
	case ScreenRestoreBackup:
		s.WriteString(m.renderRestoreBackup())
	case ScreenRestoreConfirm:
		s.WriteString(m.renderRestoreConfirm())
	case ScreenUtilities:
		s.WriteString(m.renderUtilities())
	case ScreenThemePicker:
		s.WriteString(m.renderThemePicker())
	case ScreenWSLResources:
		s.WriteString(m.renderWSLResources())
	case ScreenWSLAdvisor:
		s.WriteString(m.renderWSLAdvisor())
	case ScreenWSLBenchmark:
		s.WriteString(m.renderWSLBenchmark())
	case ScreenWSLMigration:
		s.WriteString(m.renderWSLMigration())
	case ScreenTerminalCapabilities:
		s.WriteString(m.renderTerminalCapabilities())
	case ScreenShellAudit:
		s.WriteString(m.renderShellAudit())
	case ScreenDiskCleaner:
		s.WriteString(m.renderDiskCleaner())
	case ScreenUpdateCenter:
		s.WriteString(m.renderUpdateCenter())
	case ScreenInstallVerify:
		s.WriteString(m.renderInstallVerify())
	case ScreenNetworkDiagnostics:
		s.WriteString(m.renderNetworkDiagnostics())
	case ScreenSSHManager:
		s.WriteString(m.renderSSHManager())
	case ScreenSSHHost:
		s.WriteString(m.renderSSHHost())
	case ScreenSSHForm:
		s.WriteString(m.renderSSHForm())
	case ScreenSSHKeys:
		s.WriteString(m.renderSSHKeys())
	case ScreenSSHAgent:
		s.WriteString(m.renderSSHAgent())
	case ScreenSSHDiagnostics:
		s.WriteString(m.renderSSHDiagnostics())
	case ScreenSSHTest:
		s.WriteString(m.renderSSHTest())
	case ScreenPortManager:
		s.WriteString(m.renderPortManager())
	case ScreenPortStop:
		s.WriteString(m.renderPortStop())
	case ScreenUpdate:
		s.WriteString(m.renderUpdate())
	case ScreenDoctor:
		s.WriteString(m.renderDoctor())
	case ScreenDoctorFix:
		s.WriteString(m.renderDoctorFix())
	case ScreenInstalling:
		s.WriteString(m.renderInstalling())
	case ScreenAdminPassword:
		s.WriteString(m.renderAdminPassword())
	case ScreenComplete:
		s.WriteString(m.renderComplete())
	case ScreenError:
		s.WriteString(m.renderError())
	// Trainer screens
	case ScreenTrainerMenu:
		s.WriteString(m.renderTrainerMenu())
	case ScreenTrainerLesson:
		s.WriteString(m.renderTrainerExercise("Lesson"))
	case ScreenTrainerPractice:
		s.WriteString(m.renderTrainerExercise("Practice"))
	case ScreenTrainerBoss:
		s.WriteString(m.renderTrainerBoss())
	case ScreenTrainerResult:
		s.WriteString(m.renderTrainerResult())
	case ScreenTrainerBossResult:
		s.WriteString(m.renderTrainerBossResult())
	}

	// Leader mode takes over the legend slot instead of being appended under it.
	// Appending the banner to a screen that already fills the frame would put it
	// past the terminal's last row, where a mode indicator cannot be read; and in
	// leader mode the screen's own keys are suspended, so its legend is the row
	// the banner should replace. Its notation matches every other legend: keys in
	// brackets, one separator, one order.
	content := s.String()
	if m.LeaderMode {
		content = replaceLastLine(content, WarningStyle.Render("▶ Leader mode: [q] quit"))
	}

	// Apply global padding (top: 1, right: 2, bottom: 0, left: 2), then paint the
	// drifting background across the whole frame. This tail is the one paint point
	// every screen passes through, so no screen can be added or kept that opts out.
	paddedStyle := lipgloss.NewStyle().Padding(viewPaddingRows, viewPaddingCols, 0, viewPaddingCols)
	return paintDriftBackground(paddedStyle.Render(content), m)
}

// replaceLastLine returns s with its last non-empty line replaced by line. It is
// how the leader-mode banner takes over a screen's legend without growing the
// screen past the frame it is drawn in.
func replaceLastLine(s, line string) string {
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if lipgloss.Width(lines[i]) > 0 {
			lines[i] = line
			break
		}
	}
	return strings.Join(lines, "\n")
}

// welcomeFullLockupHeight is the threshold the emblem is chosen by: at or above
// it the splash keeps the ASCII wordmark, below it the version without the
// wordmark. The wordmark is the part that is dropped because it repeats what the
// emblem already says. The number is measured, not estimated:
// TestWelcomeLockupFitsWhenFullEmblemIsChosen renders both branches and fails if
// this constant stops matching the art, and TestArtConstantsRenderWithinTerminalWidth
// and TestEmblemArtIsBilaterallySymmetric pin the art's own geometry.
//
// The frame added a header and a footer around the splash, but the threshold is
// unchanged: 32 is still the smallest frame that holds the full lockup with its
// wordmark inside the rows the frame leaves. The threshold and the geometry
// tests around it are measured decisions, so this slice leaves them exactly as
// they were and only changed how the emblem is coloured and where the version
// line sits.
const welcomeFullLockupHeight = 32

// renderEmblem draws the emblem one row at a time with a vertical ramp through
// the palette, so it is no longer one flat colour, and returns one string per
// row so the frame can count and center it. The ramp is a rebuilt style rather
// than a build of its own: the emblem is the first thing a themed run shows, so
// it has to wear the palette applyUIColors last put on the chrome.
func renderEmblem(art string) []string {
	rows := strings.Split(strings.Trim(art, "\n"), "\n")
	out := make([]string, len(rows))
	for i, row := range rows {
		if len(EmblemRampStyles) == 0 {
			// A chrome with no ramp built yet draws the lockup plain rather than
			// indexing a stop that is not there.
			out[i] = row
			continue
		}
		idx := i * len(EmblemRampStyles) / len(rows)
		if idx >= len(EmblemRampStyles) {
			idx = len(EmblemRampStyles) - 1
		}
		out[i] = EmblemRampStyles[idx].Render(row)
	}
	return out
}

// wordmarkWidth is the columns the ASCII wordmark occupies: the widest row of
// dotfilesText. It is measured from the art rather than written down, so an edit
// to the wordmark cannot leave the splash's fit check stale. The splash keeps the
// wordmark only while the body it will occupy is at least this wide; below that,
// CenterHorizontally word-wraps each row (lipgloss wraps when a style is given a
// width), which doubled the six-row wordmark into twelve and pushed the bottom
// rule and the help footer off the terminal.
func wordmarkWidth() int {
	w := 0
	for _, line := range strings.Split(strings.Trim(dotfilesText, "\n"), "\n") {
		w = max(w, lipgloss.Width(line))
	}
	return w
}

// renderWordmark draws a multi-line wordmark one row at a time through the
// title style, so the frame counts six rows rather than one six-row string.
func renderWordmark(text string) []string {
	rows := strings.Split(strings.Trim(text, "\n"), "\n")
	out := make([]string, len(rows))
	for i, row := range rows {
		out[i] = BrandStyle.Render(row)
	}
	return out
}

// greetingFor is the greeting for a time of day. It is pure: the same instant
// always yields the same words, so a test can pin every part of the day instead
// of waiting for one to arrive. A zero time has no greeting, so a model built
// without a creation time adds no line rather than guessing an hour.
func greetingFor(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	switch h := t.Hour(); {
	case h >= 5 && h < 12:
		return "Good morning"
	case h >= 12 && h < 18:
		return "Good afternoon"
	default:
		return "Good evening"
	}
}

// greeting is the welcome and main menu's added dim line. It reads the time the
// model was created with, never time.Now: a greeting read from the clock while
// drawing would change the bytes when nothing else did, and no snapshot could
// pin it.
func (m Model) greeting() string {
	return greetingFor(m.CreatedAt)
}

func (m Model) renderWelcome() string {
	l := layoutFor(m)
	inner := contentWidth(m)

	// The reference's two-column home, where the frame holds it: the lockup, the
	// cards and the call to action at the left, the machine, the plan and the
	// newest backup at the right. The compact dashboard below is the floor's
	// shape, where both columns cannot keep their facts.
	if m.homeWide(l) {
		return m.renderHomeWide(l)
	}

	// The welcome screen is the one that asks "where am I", so it offers the
	// machine panel. The frame takes it only where there is room for two columns,
	// and the body is centred in the column it will actually occupy: centring it
	// across the whole room and then cutting it to the left column would slice the
	// lockup in half. The frame asks the same registry the body's width is
	// measured against, so the two cannot disagree about whether a column exists.
	bodyWidth := inner
	if l.TwoColumn && len(m.panelsFor()) > 0 {
		bodyWidth = l.Left
	}

	// Emblem plus wordmark. Only the colouring changed: the glyphs and the
	// full/compact threshold are the measured decisions the geometry tests pin.
	//
	// The wordmark only fits where the body has as many columns as the art does.
	// The two-column left column can be narrower than it -- 61 columns at a
	// 140-column terminal -- and CenterHorizontally word-wraps a line wider than
	// the width it is given, so the sixth-row-each wordmark doubled to twelve rows
	// and took the bottom rule and the help footer off the screen. Where the
	// wordmark cannot fit, the compact lockup is the one that can: the wordmark is
	// dropped rather than the emblem, and the two-column floor below stays exactly
	// as the layout tests pin it, because raising that floor would re-lay every
	// screen at 124-150 columns instead of this one splash.
	var emblem, wordmark []string
	if m.Height >= welcomeFullLockupHeight && bodyWidth >= wordmarkWidth() {
		emblem = renderEmblem(logo)
		wordmark = renderWordmark(dotfilesText)
	} else {
		emblem = renderEmblem(compactLogo)
		wordmark = []string{BrandStyle.Render("dotfiles")}
	}

	// Under the lockup the screen is the mockup's dashboard: the tagline, the
	// machine's own four cards, the greeting by name, the environment sentence and
	// the call to action. Every one of those rows is measured against the columns
	// the body actually occupies -- the left column when a panel takes the right
	// one -- so a fact the wide screen states whole cannot wrap in the narrow
	// column, and dashboard.go's own guard says what each row is.
	dashboard := m.dashboardBody(bodyWidth)

	// The blank row between the emblem and its name is the lockup's own spacing,
	// and it is the first row the dashboard gives back when the frame is too tight
	// for the whole block: the name says what the emblem says, so the gap goes and
	// every fact stays. Without it the 60x20 frame loses its footer to a blank line.
	body := append([]string{}, emblem...)
	if m.welcomeBodyRows()-len(emblem)-len(wordmark)-len(dashboard) >= 1 {
		body = append(body, "")
	}
	body = append(body, wordmark...)
	body = append(body, dashboard...)

	// Center each row horizontally within the columns the body has. The frame
	// anchors the block to the rule and leaves the rows it does not need under it,
	// so the lockup sits under the header and the creature at the bottom, as the
	// mockup draws them; a full-height body still lands on the terminal's last row
	// instead of one past it.
	centered := make([]string, len(body))
	for i, line := range body {
		centered[i] = CenterHorizontally(line, bodyWidth)
	}
	return m.frame(m.headerName(), "", centered, []installerHint{hintStart, hintQuit})
}

// mainMenuTrainerNotice is the one line the main menu shows for a trainer save
// warning the player carried here. renderMainMenu would otherwise drop
// TrainerMessage: the trainer's own screens render it, the main menu did not, so
// a failed save on the way back told the player about lost progress on a screen
// they had already left. It is a status line and not a log -- the next key clears
// it (handleKeyPress) -- and it is wrapped to a single row so a long path in the
// error cannot grow the menu past its frame.
func (m Model) mainMenuTrainerNotice() string {
	if m.TrainerMessage == "" {
		return ""
	}
	rows := wrapText(m.TrainerMessage, contentWidth(m), 1)
	if len(rows) == 0 {
		return ""
	}
	return WarningStyle.Render(rows[0])
}

// mainMenuUpdateNotice is the one line the main menu shows for the self-update
// the row under the cursor just ran: the download in progress, the path the
// replaced binary was kept at, or why nothing was replaced. The result lands on
// the screen the user is already on, in the same slot the row lives in, rather
// than taking the run to a result screen -- the lesson the theme switch paid for.
// Like the trainer's line it is wrapped to a single row, so a long path cannot
// grow the menu past its frame.
func (m Model) mainMenuUpdateNotice() string {
	if m.UpdateCheck.Notice == "" {
		return ""
	}
	rows := wrapText(m.UpdateCheck.Notice, contentWidth(m), 1)
	if len(rows) == 0 {
		return ""
	}
	return InfoStyle.Render(rows[0])
}

// renderUtilities draws the utilities section: the mockup's card of jobs -- the
// job's icon, its name and its one-line description, the row under the cursor
// filled across the card -- beside the System Overview panel, with the reasons a
// job is missing above the card and the last result under it. The themes
// themselves live one level in, on the theme picker.
func (m Model) renderUtilities() string {
	// Enter opens a screen here; the rows that act where they stand run on their
	// own letter, and the footer names those letters only while their rows are
	// offered. A footer that named a key with no row behind it would be offering a
	// press that does nothing.
	hints := []installerHint{hintUp, hintDown, hintOpen}
	if m.ThemeSwitchFound {
		hints = append(hints, hintDark, hintLight)
	}
	if m.themeUndoAvailable() {
		hints = append(hints, hintUndo)
	}
	hints = append(hints, hintBack)

	l := layoutFor(m)
	panels := m.panelsFor()
	frameHints := m.panelHints(panels, hints)
	// The footer's RESERVATION, not the rows it draws: the three theme keys appear
	// with the desktop they switch, and an action that appears once must not take
	// its row from the body -- taking it from the body is what moved the list.
	budget := installerBodyRows(m.Height, m.footerReserveRows(l.Inner, frameHints))

	// The reference's two-column section where the frame holds it: the card of
	// jobs at the left and the theme, machine, resources and recommended actions
	// at the right. The compact card below is the floor's shape.
	if m.utilitiesWide(l, budget) {
		return m.renderUtilitiesWide(l, budget)
	}

	// The shape is decided once from the frame's rows and the width, and the body is
	// composed against it: the mockup's two-line rows where they fit, the same rows
	// on one line where they would not, and the card around them wherever there is
	// width for it.
	shape := m.utilitiesShapeFor(budget, l)
	body, start, end := m.utilitiesBodyLines(shape, budget)

	// A two-column frame draws the overview beside the body; a one-column frame has
	// no column for it, so the panel's summary rides the card's top border (the
	// shape built it) and the body is centred in the composition like every other
	// page.
	if !(l.TwoColumn && len(panels) > 0) {
		body = centeredBody(body, l)
	}

	// The header's vital names the slice of a windowed list that is on screen, so a
	// list longer than the frame says so instead of appearing to end where the
	// screen does. A list that fits has nothing to report and gets no filler.
	vital := ""
	if end-start < len(shape.Heights) {
		vital = scrollVital("Showing", start+1, end, len(shape.Heights))
	}
	return m.frame(m.headerName(), vital, body, hints)
}

// utilitiesReasons is the utilities section's body. A row names itself and says
// what it opens, so the prose carries only what no row can: the theme list that
// cannot be read, the WSL screens this host cannot open. One sentence per missing
// job, in the order the rows would have had.
func (m Model) utilitiesReasons() []string {
	var reasons []string
	// What the switch will do, and through which desktop's own tool: the line the
	// section cannot lose, because it names the desktop it detected. A host with no
	// switch adds no line at all. A sentence about the DESKTOP theme sitting above
	// the row that opens the installer's OWN theme picker read as a verdict on that
	// picker, which works here either way, so the absent rows are left to speak for
	// themselves.
	if m.ThemeSwitchFound {
		reasons = append(reasons, fmt.Sprintf(
			"The desktop theme switch changes the light/dark theme through %s's own tool, and records "+
				"the setting it replaces in theme.json so the switch can be undone.", m.ThemeSwitch.Name))
	}
	// The theme row is offered when there are definitions or a record to put back,
	// so a reason is due only when neither exists.
	if len(m.dotfilesThemeOptions()) == 0 && m.DotfilesThemeRecord == nil {
		if m.dotfilesThemesPending() {
			reasons = append(reasons,
				"Reading the dotfiles' theme definitions from the repository checkout…")
		} else {
			reason := m.DotfilesThemesErr
			if reason == "" {
				reason = themeDefinitionsNotFoundMessage()
			}
			reasons = append(reasons, fmt.Sprintf("The dotfiles' own theme is not switchable here: %s.", reason))
		}
	}
	if reason := m.wslResourcesUnavailableReason(); reason != "" {
		reasons = append(reasons, reason)
	}
	if reason := m.shellAuditUnavailableReason(); reason != "" {
		reasons = append(reasons, reason)
	}
	if !m.wslAdvisorOffered() && m.SystemInfo != nil && m.SystemInfo.IsWSL {
		reasons = append(reasons, m.wslAdvisorUnavailableReason())
	}
	return reasons
}

// renderUpdate draws this installer's own release screen: what this build is,
// what is published, where that answer came from and when it was read, who owns
// the file an update would replace, and the two rows that act on it. It has no
// golden of its own and no panel: it is one column of facts and two presses.
func (m Model) renderUpdate() string {
	// The two rows that act here run on their own letters -- [i] installs the
	// release, [r] asks again -- and Enter is left to the way back, which is a
	// screen the row opens. The footer names a letter only while its row is
	// offered, so a blocked install is never advertised.
	hints := []installerHint{hintUp, hintDown, hintOpen, hintBack}
	if m.UpdateInstallable() {
		hints = append(hints, hintUpdateInstall)
	}
	if !m.UpdateCheck.InFlight {
		hints = append(hints, hintRefresh)
	}
	return m.renderThemeScreen(m.updateDescription(), m.UpdateCheck.Notice, hints)
}

// renderWSLResources draws the WSL resource screen: the host's capacities, the
// recommendation derived from them, what the file holds now, the three editable
// values and the write row. It is the same shape as the other utilities screens,
// so it uses the same budget arithmetic; the arrows are the extra keys, and they
// are in the legend rather than in prose.
func (m Model) renderWSLResources() string {
	return m.renderThemeScreen(m.wslResourceDescription(), m.wslNoticeLine(),
		[]installerHint{hintMove, hintAdjust, hintRecommended, hintWriteWSL, hintRepairGuard, hintBack})
}

// wslNoticeLine is the WSL screen's result line: the write's outcome, and while
// the values are being read back from the file, a mark that the refresh is
// running. The mark lives in the notice slot rather than the body so the table
// below it keeps its size and place through the refresh: the outcome lands where
// the press put it, and the re-read only changes values in place.
func (m Model) wslNoticeLine() string {
	if m.WSLState.Refreshing && m.WSLNotice != "" {
		return "Refreshing the values from the file… " + m.WSLNotice
	}
	return m.WSLNotice
}

// renderTerminalCapabilities draws the capability report: one row per
// capability with the answer, where it came from and what it implies, or the
// one honest line that the probe is still reading. The screen changes nothing,
// so its only key is the way back.
func (m Model) renderTerminalCapabilities() string {
	return m.renderThemeScreen(m.terminalCapabilitiesDescription(), "",
		[]installerHint{hintBack})
}

// terminalCapabilitiesDescription is what the screen says: for each capability
// the answer, its source, its consequence, and -- when the probe could not
// determine it -- why not and how to check by hand. The rows come from the
// report's own list, so there is no second copy of the capabilities here.
func (m Model) terminalCapabilitiesDescription() []string {
	if !m.TerminalCapabilities.Resolved {
		return []string{"Reading what this terminal can do and what it means for the themes and the interface…"}
	}

	paragraphs := []string{capabilityReadOnlyPromise}
	for _, row := range m.TerminalCapabilities.rows() {
		// The answer is its own row and the explanation is the row after it, so the
		// three facts of a capability read as three units instead of one paragraph
		// and the value is the first thing seen. The source and the consequence are
		// joined by a dash rather than a full stop: the consequence is a lower-case
		// clause, and a period before it read like a typo in the sentence above.
		//
		// The sentence under the answer is terminalCapabilityDetail's, which the
		// wide frame's row draws as its muted second line (report_cards.go), so the
		// two shapes explain one answer in the same words.
		paragraphs = append(paragraphs, fmt.Sprintf("%s: %s", row.Name, row.Answer.Value))
		paragraphs = append(paragraphs, terminalCapabilityDetail(row))
	}
	return paragraphs
}

// renderShellAudit draws the shell startup screen: the method the number comes
// from, the runs and their median, the functions zprof blamed or the reason none
// can be named, and the one row that starts the measurement. It is the same shape
// as the other utilities screens, so it uses the same budget arithmetic.
func (m Model) renderShellAudit() string {
	// The way back is the way to stop while the runs are still coming: Esc leaves
	// the screen when nothing is running and stops the starts that are left when
	// something is, so the footer names the one the press will really do.
	back := hintBack
	if m.ShellAudit.Measuring {
		back = hintShellAuditStop
	}
	hints := []installerHint{hintUp, hintDown, hintShellAuditMeasure, hintShellAuditPlan, hintShellAuditExtras, back}

	// The reference's two-column screen where the frame holds it: the clock art,
	// the numbered explanation, the action row, the results and recent runs, and
	// the methodology. The section renderer below is the 80x24 floor's shape.
	l := layoutFor(m)
	budget := installerBodyRows(m.Height, m.footerReserveRows(l.Inner, hints))
	if m.shellAuditWide(l, budget) {
		return m.renderShellAuditWide(l, budget)
	}
	return m.renderThemeScreen(m.shellAuditDescription(), m.shellAuditNotice(), hints)
}

// renderThemePicker draws the dotfiles theme list: the mockup's grid of theme
// cards, each carrying a miniature of the palette it would paint and naming the
// tools it leaves out, the undo row when there is a change to put back, and the
// live preview of the theme under the cursor.
func (m Model) renderThemePicker() string {
	// The refresh review is a second state of the picker: its confirmation writes
	// files, so both answers run on their own letters, and Enter is left to the way
	// back the result state offers. The list itself chooses a theme on Enter and runs
	// its commands on their letters, named in the footer only while their rows are.
	if m.ThemeRefreshReview {
		if m.ThemeRefreshDone {
			return m.renderThemeScreen(m.themePickerDescription(), m.ThemeNotice,
				[]installerHint{hintOpen, hintBack})
		}
		return m.renderThemeScreen(m.themePickerDescription(), m.ThemeNotice,
			[]installerHint{hintUp, hintDown, hintThemeRefreshApply, hintThemeRefreshCancel})
	}

	return m.renderThemePickerList()
}

// renderThemeScreen draws a screen whose body is a description, a menu, the live
// preview and the last notice, sized to the frame. Every band is measured from
// the frame and the screen's own reservation (section_bands.go), never from the
// row under the cursor: moving the cursor changes what the rows say, never where
// they are. The notice and the hints are parameters rather than fields so a
// screen's footer says what that screen's keys do.
func (m Model) renderThemeScreen(paragraphs []string, screenNotice string, hints []installerHint) string {
	// On a frame with room for two columns, the screens whose detail is context for
	// a list draw it beside the list instead of above it: the list keeps the whole
	// body, and the detail gets the column the wide frame has going spare.
	l := layoutFor(m)
	panels := m.panelsFor()
	// On a frame that holds two columns of cards, the doctor and the section's own
	// reports draw in the reference's grammar instead of the bare bands below: the
	// list in a card, the screen's prose in the card beside it. The floor and every
	// frame too short for both keep the bands, and the gate reads only the frame, so
	// the screen cannot change shape while the user walks it.
	if m.sectionWide(l, hints) {
		return m.renderSectionWide(l, paragraphs, screenNotice, hints)
	}
	// A section screen whose right column comes from the panel registry lets the
	// frame compose it; only the screens with no registered panel draw their detail
	// through the section's own pane. The utilities board is registered, so its body
	// is the reason prose plus the list and its panel is the machine's state.
	pane := l.TwoColumn && m.sectionDetailPane() && len(panels) == 0
	width := l.Inner
	if l.TwoColumn && (pane || len(panels) > 0) {
		width = l.Left
	}
	if !pane && len(panels) == 0 && l.TwoColumn {
		// A one-column page shares the centred composition, not the full inner
		// width: prose and rows have one measure on every page.
		width = l.Composition
	}
	detail := inkParagraphs(wrapParagraphs(paragraphs, width))
	options := m.GetCurrentOptions()
	detailLines := len(detail)
	if pane {
		detailLines = 0
	}
	bands := m.sectionBandsFor(width, hints, detailLines, len(options), pane)

	menu := m.menuRows(options, m.Cursor)
	tail := pinnedTailLen(options)
	pinned := menu[len(menu)-tail:]
	entries := menu[:len(menu)-tail]
	entryTotal := len(options) - tail

	// The doctor and the update center carry the longest single facts in the
	// program, and their rows used to be cut mid-word. Where the frame has the rows,
	// the same entries are wrapped onto a second line instead; where it does not,
	// the one-line rows are kept, so the wrap never costs a row the list needs.
	wrappedEntries := false
	if m.Screen == ScreenDoctor || m.Screen == ScreenUpdateCenter {
		if wrapped, ok := m.wrappedSectionRows(options[:entryTotal], m.Cursor, width); ok {
			if len(wrapped) <= max(1, bands.List-len(pinned)) {
				entries = wrapped
				wrappedEntries = true
			}
		}
	}

	// The live preview: while the cursor is on a theme row, the screen shows that
	// theme's real palette. It writes nothing; the values come from the same
	// definition the apply would write. It sits under the entries, in the list
	// band's slack, so it cannot move a row above it.
	preview := clipLines(m.themePreviewLines(width), bands.Preview, width)

	// The activity slot's rows come from the band arithmetic (section_bands.go),
	// which allocates them from the frame and the list alone: the window the cursor
	// is in is identical from the press to the result.
	activityRows := bands.Activity
	activity := clipLines(activityLines(m.pickerActivity(), width, activityRows), activityRows, width)
	// The preview and the activity under the entries must not eat the window, so a
	// band with no room for both keeps the window and drops the decoration.
	if bands.List-bands.Pinned-len(activity) < 1 {
		preview = nil
	}
	entryWindow := bands.entryRows(len(activity))

	var body []string
	if bands.Title > 0 {
		// The title is the brand role and nothing else. While a preview is active the
		// chrome has already been repainted in the theme under the cursor -- including
		// this role, which maps to the definition's blue -- so a title built from the
		// definition here would be the same colour drawn through a second path, and
		// the one path that could disagree with the chrome around it.
		body = append(body, BrandStyle.Render(m.sectionBodyTitle()))
	}
	body = append(body, clipLines(detail, bands.Detail, width)...)
	body = padRows(body, bands.Title+bands.Detail)
	for i := 0; i < bands.Gap; i++ {
		body = append(body, "")
	}

	// The entries are the part of the list the frame windows; the tail is always
	// drawn, at the foot of the band, so the way back and the actions never move.
	scrollable := entryTotal
	var start, end int
	var visible []string
	if wrappedEntries {
		start, end = 0, scrollable
		visible = entries
	} else if m.Cursor < scrollable {
		start, end = listWindow(m.Cursor, entryWindow, scrollable)
		visible = entries[start:end]
	} else {
		end = scrollable
		start = max(0, end-entryWindow)
		visible = entries[start:end]
	}

	listed := append([]string{}, visible...)
	listed = append(listed, preview...)
	listed = append(listed, activity...)
	// The action tail follows the entries it acts on. The band still reserves the
	// tail's height and the entries' window, so the tail cannot move when an action
	// appears with a selection; what moves is only the spare rows, and they go
	// below the tail instead of above it. Pinning the tail to the band's foot left
	// a short list with its actions a screenful away from it.
	listed = append(listed, pinned...)
	listed = append(listed, padRows(nil, bands.List-len(listed))...)
	if len(listed) > bands.List {
		// A tail taller than its reservation keeps the rows that end the screen --
		// the way back is the last of them -- and gives up the top of the block. The
		// header's range is measured from what is left, so it can never advertise a
		// row the frame dropped.
		over := len(listed) - bands.List
		listed = listed[len(listed)-bands.List:]
		if !wrappedEntries {
			if dropped := min(over, max(0, end-start)); dropped > 0 {
				end -= dropped
			}
		}
	}
	listed = padRows(listed, bands.List)
	body = append(body, listed...)
	body = append(body, noticeBand(screenNotice, bands.Notice, width)...)
	body = padRows(body, bands.Body)
	if pane {
		// The detail column is drawn through the same panel assembly the main menu's
		// panel uses, so the chip, the rule and the cut marker are the ones every
		// other column already shows.
		column := panelBody(m.sectionPaneTitle(), inkParagraphs(wrapParagraphs(paragraphs, l.Right)), l.Right, bands.Body)
		body = composeColumns(body, column, l)
	} else if len(panels) == 0 {
		body = centeredBody(body, l)
	}

	// The header's vital names the slice of a windowed list that is on screen, so
	// a list longer than the frame says so instead of appearing to end where the
	// screen does. A list that fits has nothing to report and gets no filler, the
	// same rule the keymap menus and the restore list follow.
	vital := ""
	if end-start < scrollable {
		vital = scrollVital("Showing", start+1, end, scrollable)
	}
	if suffix := m.sectionVitalSuffix(); suffix != "" {
		if vital == "" {
			vital = suffix
		} else {
			vital += " · " + suffix
		}
	}
	return m.frame(m.headerName(), vital, body, hints)
}

// sectionVitalSuffix is what a section screen's header says beside the range of
// the list: the selection the actions would move, measured from the same state
// the rows are drawn from.
func (m Model) sectionVitalSuffix() string {
	if m.Screen != ScreenUpdateCenter {
		return ""
	}
	if count := m.UpdateCenter.Selected.Count(); count > 0 {
		return fmt.Sprintf("%d selected", count)
	}
	return ""
}

// descriptionLines is a screen's prose wrapped to the width it will be drawn at
// and clipped to the rows the frame leaves. A description longer than its budget
// keeps the lines that fit and says how many it could not show, the same rule the
// panel and list bodies follow.
func descriptionLines(paragraphs []string, width, budget int) []string {
	if budget < 1 {
		// No room for prose at all. The rows below the description still say what
		// the screen offers, so this is a cut rather than a failure.
		return nil
	}

	var lines []string
	for _, paragraph := range paragraphs {
		lines = append(lines, wrapText(paragraph, width, 0)...)
	}
	if len(lines) > budget {
		hidden := len(lines) - budget + 1
		lines = append(lines[:budget-1], fmt.Sprintf("… and %d more", hidden))
	}
	return lines
}

// themeActivitySlotFloor is the floor the activity slot keeps while a switch is
// running or its result is on screen, on a frame too short to hold the theme list
// whole: the list gives those rows up so the press still says what it is doing,
// and the slot's height stays a function of the frame and the list alone. It is a
// floor and not a cap -- a frame with room to spare gives the slot every row the
// list did not need, so the outcome can name its tools. Five rows is the smallest
// slot a result can be read in: three lines of method, one named tool and the cut
// marker.
const themeActivitySlotFloor = 5

// pickerActivity is the picker's activity slot, and nil on every other screen.
// A switch left running while the user walked away keeps its outcome in the
// model, but it cannot put its slot on the WSL screen or the section: the slot
// belongs to the list the press was made on.
func (m Model) pickerActivity() *themeActivity {
	if m.Screen != ScreenThemePicker {
		return nil
	}
	return m.ThemeActivity
}

// activityLines renders the picker's activity: the pending line while the switch
// runs, or the outcome once it lands, cut to rows rows with a count when the
// outcome is longer than the frame left for it -- the same rule the description
// and the notice follow. It is one function for both states on purpose: the cut
// comes from the caller's budget, never from which of the two is being drawn, so
// the pending line and the result occupy the same rows. The rows the activity did
// not use are the caller's to place, which is why this returns no padding.
func activityLines(activity *themeActivity, width, rows int) []string {
	if activity == nil || rows < 1 {
		return nil
	}
	paragraphs := activity.Result
	if len(paragraphs) == 0 && activity.Pending != "" {
		paragraphs = []string{activity.Pending}
	}
	var lines []string
	for _, paragraph := range paragraphs {
		lines = append(lines, wrapText(paragraph, width, 0)...)
	}
	if len(lines) > rows {
		hidden := len(lines) - rows + 1
		lines = append(lines[:rows-1], fmt.Sprintf("… and %d more", hidden))
	}
	return lines
}

// dotfilesThemesPending reports whether the definitions have never finished a
// read: nothing is held and no failure has been recorded, so a read is the only
// thing that can be in flight. It is what lets the section say it is reading
// instead of claiming the theme is not switchable -- a fact only a finished read
// can establish. A read that ends either way, with definitions or with the
// reason it could not, is no longer pending.
func (m Model) dotfilesThemesPending() bool {
	return m.DotfilesThemes == nil && m.DotfilesThemesErr == ""
}

// utilitiesDescription is what the section says about itself, in the installer's
// own voice: one short line per job it offers, and the honest reason for each one
// it does not. The rows are the data the user came for, so each is named in the
// order it appears and the prose stays short enough to leave them on screen: a
// wall of paragraphs above a list pushed the list and its own explanations past
// the cut. Adding a fifth job is what made that budget real -- at the 80-column
// floor the longer sentences this description used to carry no longer fit -- so
// each line now names its job and leaves the detail to the screen that job opens,
// where there is room for it. The file it names is the installer's own record; the
// desktop's store belongs to the desktop's tool.
func (m Model) utilitiesDescription() []string {
	var paragraphs []string
	// The desktop switch, in the sentence that names the desktop it detected. A host
	// with no switch adds no sentence: the rows are simply absent, and a paragraph
	// about the DESKTOP theme directly above the row that opens the installer's OWN
	// theme picker read as one verdict on that picker, which works here either way
	// (TestUtilitiesUnavailableIsQuietAndFitsTheFrame, and
	// TestUtilitiesUnavailableIsAboutTheModelNotTheHost for the model's half).
	if m.ThemeSwitchFound {
		paragraphs = append(paragraphs, fmt.Sprintf(
			"The desktop theme switch changes the light/dark theme through %s's own tool, and records "+
				"the setting it replaces in theme.json so the switch can be undone.", m.ThemeSwitch.Name))
	}

	// The dotfiles' own theme. Its definitions are resolved from $DOTFILES_DIR,
	// the clone, the working directory and its parents, then ~/dotfiles and
	// ~/.dotfiles, and last from the per-user copy the installer made; when none
	// of them holds themes/*.toml the section says it cannot be switched here
	// instead of showing rows that would fail.
	switch {
	case len(m.DotfilesThemes) > 0:
		// The line this case used to add -- that the row opens the list of complete
		// themes -- only restated the row drawn directly above it, and it is the line
		// that yields now that the section carries the doctor as its tenth job: the row
		// is one line of budget the body no longer has, and a row that names the job
		// and opens it cannot be missed, while the sentences below say why a row is
		// absent and no row can carry that. Every other sentence here earns its line
		// the same way, and TestUtilitiesBodyCarriesEveryJobAtTheFloor fails if one of
		// them is pushed behind the cut instead.
	case m.dotfilesThemesPending():
		// The definitions are read when the section opens, so an empty list with
		// no recorded failure is a read in flight, not a verdict. The section says
		// only that it is reading: "not switchable" is a claim about the result,
		// and there is no result yet.
		paragraphs = append(paragraphs, "Reading the dotfiles' theme definitions from the repository checkout…")
	default:
		reason := m.DotfilesThemesErr
		if reason == "" {
			reason = themeDefinitionsNotFoundMessage()
		}
		// The row is offered when there are definitions OR when there is a record
		// to put back, so this branch is reached in the one state where the row is
		// still there with nothing to apply: the list cannot be read, and the undo
		// the record names can still be offered. "Not switchable" would deny the
		// row the list below it draws, which is the contradiction this sentence is
		// worded from the row's own condition to avoid.
		if m.DotfilesThemeRecord != nil {
			paragraphs = append(paragraphs, fmt.Sprintf(
				"The dotfiles' theme definitions cannot be read here (%s), so no theme can be applied; "+
					"the change the record names can still be undone.", reason))
			break
		}
		paragraphs = append(paragraphs, fmt.Sprintf("The dotfiles' own theme is not switchable here: %s.", reason))
	}

	// The WSL resources, in one line when they are offered and with the reason
	// when they are not. The section says why, so the gap is declared rather than
	// left to read as an oversight.
	if reason := m.wslResourcesUnavailableReason(); reason != "" {
		paragraphs = append(paragraphs, reason)
	} else {
		paragraphs = append(paragraphs, fmt.Sprintf("%s shows what %s holds today.", utilitiesWSLRow, m.WSLState.Path))
	}

	if reason := m.shellAuditUnavailableReason(); reason != "" {
		paragraphs = append(paragraphs, reason)
	} else {
		paragraphs = append(paragraphs, fmt.Sprintf("%s times %s; it changes nothing.",
			utilitiesShellAuditRow, m.ShellAudit.Command))
	}

	// The WSL performance advisor, in one line when it is offered and with the reason when it is not
	// -- but only on a host the installer already classified as WSL. On any other host, naming a WSL
	// utility the section cannot offer would spend the body on a question nobody asked.
	switch {
	case m.wslAdvisorOffered():
		paragraphs = append(paragraphs, fmt.Sprintf("%s reads this instance's mounts, memory, storage "+
			"and resolver and states what it found, with the evidence behind every finding; the "+
			"measurement and the assisted copy are one level in, and the report itself changes nothing.",
			utilitiesWSLAdvisorRow))
	case m.SystemInfo != nil && m.SystemInfo.IsWSL:
		paragraphs = append(paragraphs, m.wslAdvisorUnavailableReason())
	}

	// The SSH manager, in one line: what it shows and what it may change. It is
	// offered everywhere, so no reason line is needed here; the screen itself says
	// what a host without an ssh client can still do. It is kept to a single line,
	// like the rows it sits beside: the section's prose shares a row budget with the
	// list, and a longer paragraph is what pushes another row's own line behind the
	// "… and n more" marker.
	paragraphs = append(paragraphs, fmt.Sprintf("%s shows the hosts and keys the client uses.", utilitiesSSHRow))

	// The terminal capability report. It is read-only and offered everywhere;
	// where there is no terminal to read, the screen itself says so rather than
	// the section leaving a hole. Naming it here is what makes the row findable.
	paragraphs = append(paragraphs, fmt.Sprintf(
		"%s reads the colour depth and OSC 52.", utilitiesTerminalRow))

	// The development disk cleaner is the section's newest job, and it is the one that
	// yields here for the same reason the update center's line does: the section's body
	// shares the frame with its rows, its own guard fails on the cut marker a longer
	// body produces, and every line above is one row and earns its place. What its line
	// would have said -- that a store is reported only when the machine shows evidence
	// of it, that nothing is cleaned that was not marked, and that a store a tool owns
	// is cleaned through the tool's own command -- is stated on the screen itself, in
	// diskCleanDescription, where the marks are made rather than where the section sums
	// the jobs up. The list still names it (utilitiesDiskCleanerRow), and its own panel
	// entry reports what the last scan measured.

	// The record of the last install. It is read-only and offered everywhere for
	// the same reason: the screen itself says when there is no record, so the
	// section never leaves a hole where a utility should be, and it never promises
	// more than presence -- the record holds no digest.
	//
	// The sentence is one row and has to stay one: the section's body shares the
	// frame with its rows, TestUtilitiesBodyCarriesEveryJobAtTheFloor reads the
	// 80x24 frame, and the update center's line is the job that spent the last row
	// this section had (main's own fix gave the SSH manager's line back by making
	// the WSL reason one row). The longer sentence -- "reports which recorded paths
	// are gone" -- wrapped to two rows, and the wrap put three job lines behind the
	// cut marker; the claim is the same one and it fits in fewer words.
	paragraphs = append(paragraphs, fmt.Sprintf("%s reports the paths that are gone.", utilitiesVerifyRow))

	// The network diagnosis is the third job whose body line the floor budget asks
	// for. It yields where the two below do, and for the same reason: the rows list
	// names it, and what its line would have said -- the host an overview reaches
	// for, and that every check is bounded -- is stated on the screen itself, where
	// the run that needs it is started.

	// The update center, the network diagnosis and the port and process manager are
	// the jobs with no line of their own here. The section's body shares the frame
	// with its rows, and the rows take their budget first (renderThemeScreen:
	// descBudget = available - len(menu)); the body is cut rather than allowed to run
	// off the frame, and TestUtilitiesBodyCarriesEveryJobAtTheFloor fails on the cut
	// marker a body longer than the budget produces. Every line above is one line and
	// earns its place, so the line that yields is the newest one rather than a peer's.
	// The update center was the first job to arrive without a line, the network
	// diagnosis gave up its line when the port manager's row took the last one the
	// body had, and the port manager itself arrived without one.
	//
	// Nothing a user needs is lost: the rows list names them, and what their lines
	// would have said -- that a command runs only for what was selected, after a
	// confirmation; that a stop is confirmed on its own screen after the process
	// and its ports are shown and revalidated -- is stated on the screens
	// themselves, in updateCenterDescription and on the manager's own confirmation,
	// where the choices are made rather than where the section sums them up.

	return paragraphs
}

// themePickerDescription is what the theme list says about itself: the palette
// is defined once, a theme is applied only when one is chosen, and the tools a
// theme cannot paint are named on its row. It is the same honest account the
// utilities section gave when the list lived there.
func (m Model) themePickerDescription() []string {
	if m.ThemeRefreshReview {
		if m.ThemeRefreshDone {
			return m.ThemeRefreshResult
		}
		return themeRefreshReviewParagraphs(m.ThemeRefreshCandidates)
	}
	if len(m.DotfilesThemes) == 0 {
		if m.dotfilesThemesPending() {
			return []string{"Reading the dotfiles' theme definitions from the repository checkout…"}
		}
		reason := m.DotfilesThemesErr
		if reason == "" {
			reason = themeDefinitionsNotFoundMessage()
		}
		return []string{fmt.Sprintf("The dotfiles' own theme is not switchable here: %s.", reason)}
	}
	return []string{"The dotfiles' own theme is defined once in themes/. Choosing a theme applies it to every " +
		"tool it can paint and names the tools it leaves out. Moving the cursor previews a theme without " +
		"applying anything; the switch records what it replaces, so it can be undone."}
}

// dotfilesThemeUndoRow is the row that puts back the blocks the last dotfiles
// theme change replaced. It is named distinctly from the desktop switch's undo
// row so the two are not confused on the same screen, and it runs on its own
// letter: it acts where it stands rather than opening anything.
const dotfilesThemeUndoRow = "[u] Undo the last dotfiles theme change"

// themeRefreshRow is the row that detects the installed theme files that are
// not up to date. It only opens the review; nothing is written until the
// confirmation on that review is chosen, so the row runs on its own letter and
// says so.
const themeRefreshRow = "[r] Refresh outdated theme files"

// themeRefreshCancelRow is the safe default of the review: it leaves every file
// exactly as it is. It carries its own letter because the row beside it writes.
const themeRefreshCancelRow = "[n] Cancel"

// themeRefreshConfirmRow is the review's confirmation, naming how many files it
// would write. It writes, so it names its own letter rather than Enter.
func themeRefreshConfirmRow(candidates []themeRefreshCandidate) string {
	n := 0
	for _, cand := range candidates {
		if cand.Problem == "" {
			n++
		}
	}
	return fmt.Sprintf("[y] Yes, refresh %d file(s)", n)
}

// settleThemePickerCursor keeps the cursor on a selectable row after an outcome
// has changed the list under it. An undo is the case: it removes the undo row it
// was pressed on, and the index that row had then falls on a separator, which is
// drawn as a rule with no cursor marker. The cursor steps to the nearest
// selectable row instead, so the focus is always somewhere it can be seen and
// explained. It is a no-op when the cursor already points at a selectable row,
// which is what an apply does.
func (m *Model) settleThemePickerCursor() {
	options := m.GetCurrentOptions()
	selectable := func(i int) bool {
		return i >= 0 && i < len(options) && !strings.HasPrefix(options[i], menuSeparatorPrefix)
	}
	if selectable(m.Cursor) {
		return
	}
	for i := m.Cursor - 1; i >= 0; i-- {
		if selectable(i) {
			m.Cursor = i
			return
		}
	}
	for i := m.Cursor + 1; i < len(options); i++ {
		if selectable(i) {
			m.Cursor = i
			return
		}
	}
}

// resetThemeRefresh clears the review state and the activity slot. It is called
// when the picker is left or a review is dismissed, so a later visit starts from
// a clean list; a switch's outcome is a fact about the list the user is standing
// on, and leaving the list leaves the fact with it.
func (m *Model) resetThemeRefresh() {
	m.ThemeRefreshCandidates = nil
	m.ThemeRefreshReview = false
	m.ThemeRefreshDone = false
	m.ThemeRefreshResult = nil
	m.ThemeActivity = nil
}

// utilitiesScreenMark is the mark in front of a row that opens a screen. It is a
// constant because the section's handler takes it back off to match the row's
// label, and the two spellings have to be one.
const utilitiesScreenMark = "→ "

// utilitiesScreenRow draws a row that opens a screen. The Utilities section is a
// list of jobs, and most of them open a screen, so the arrow is what says a row
// does that rather than acting where it stands; the way back carries the same
// arrow pointing the other way. A row that acts where it stands carries the
// letter that runs it in brackets instead, and a switch row shows its state, the
// way the lists inside the section already do.
func utilitiesScreenRow(label string) string { return utilitiesScreenMark + label }

// The Utilities section's rows that act where they stand. They run on their own
// letter -- Enter is left to the rows that open a screen -- and the section's
// footer names the same letters while the rows are offered, so the row and the
// footer cannot disagree about the key that presses it.
const (
	utilitiesDarkRow  = "[d] Switch to the dark theme"
	utilitiesLightRow = "[l] Switch to the light theme"
	utilitiesUndoRow  = "[u] Undo the last theme change"
)

// utilitiesThemeRow is the utilities section's single row for the dotfiles'
// own theme. The themes themselves live one level in, on the theme picker, so
// the section reads as a list of jobs rather than a list of themes.
const utilitiesThemeRow = "Change the dotfiles theme"

// utilitiesWSLRow is the section's row for the WSL resources. It is offered only
// where there is a .wslconfig to edit, and the section's body names the reason
// everywhere else.
const utilitiesWSLRow = "Adjust the WSL resources"

// utilitiesTerminalRow is the section's row for the terminal capability report.
// It is offered everywhere -- there is always a terminal to describe, and where
// there is none the screen says so -- and it is read-only.
const utilitiesTerminalRow = "Report the terminal capabilities"

// utilitiesDiskCleanerRow is the section's row for the developer disk cleaner.
// It is offered everywhere: a machine a development tool is installed on always
// keeps something, and where the user-level stores cannot be located the
// screen's own body names the reason rather than the row going missing.
const utilitiesDiskCleanerRow = "Clean Dev Caches"

// utilitiesVerifyRow is the section's row for the record of the last install,
// checked against this machine. It is offered everywhere -- there is always a
// machine to compare with the record, and where there is no record the screen
// says exactly that -- and it is read-only, like the report beside it.
const utilitiesVerifyRow = "Check the machine against the last install"

// utilitiesPortRow is the section's row for the port and process manager. It is
// offered everywhere: reading what holds a port is always safe, and a platform
// with no reader says so in the screen's own body rather than the row vanishing.
const utilitiesPortRow = "Find Available Port"

// The rows the arrow keys act on. The indices are part of the screen's contract:
// the key handler adjusts the value under the cursor by its index, and the frame
// guards measure the list this function returns.
const (
	wslResourceRowMemory = iota
	wslResourceRowProcessors
	wslResourceRowSwap
)

// wslWriteRow is the one row that touches the file. Its letter is on the row
// because the write is a command, not an opening: Enter opens and chooses, so the
// row that writes says which key writes.
const wslWriteRow = "[w] Write the .wslconfig"

// wslValueLabel renders one value for the screen. Zero is named rather than
// printed as a zero, which would read as a real limit; the caller says which
// zero it is, because a value the plan omits and a value a row has not set read
// differently to a person even though they are the same number here.
func wslValueLabel(value int, unit, zero string) string {
	if value <= 0 {
		return zero
	}
	if unit == "" {
		return strconv.Itoa(value)
	}
	return fmt.Sprintf("%d %s", value, unit)
}

// wslResourceRows builds the WSL resource screen's rows from its state: one row
// per managed key carrying the draft value, the write row, and the way back. The
// values are the draft's, so the rows always show what the write would apply.
func (m Model) wslResourceRows() []string {
	draft := m.WSLState.Draft
	return []string{
		"Memory: " + wslValueLabel(draft.MemoryMB, "MB", wslDraftZero) + " [←/→]",
		"Processors: " + wslValueLabel(draft.Processors, "", wslDraftZero) + " [←/→]",
		"Swap: " + wslValueLabel(draft.SwapMB, "MB", wslDraftZero) + " [←/→]",
		m.menuSeparator(),
		wslWriteRow,
		m.menuSeparator(),
		wslGuardRepairRow,
		m.menuSeparator(),
		"← Back",
	}
}

// wslDraftZero is how a row says it holds no value: the key is left exactly as
// the file has it, because the utility updates the keys the template manages and
// never deletes one.
const wslDraftZero = "not set (left as it is)"

// wslOmittedZero is how the recommendation names a key the template omits.
const wslOmittedZero = "omitted (WSL decides)"

// wslResourceDescription is the WSL resource screen's compact body, derived from the
// facts the wide frame draws: the capacities the recommendation came from, the
// recommendation and the proportions it follows, what the file holds today, what the
// memory guard holds on the running machine, what a write preserves, and the one
// step the screen deliberately does not take.
func (m Model) wslResourceDescription() []string {
	return factProse(m.wslResourceFacts())
}

// wslResourceFacts is that answer as facts.
func (m Model) wslResourceFacts() []iconFact {
	state := m.WSLState
	if reason := m.wslResourcesUnavailableReason(); reason != "" {
		return []iconFact{{Label: "WSL resources", Value: reason}}
	}

	// The values come first: the host's capacities, the recommendation, and what
	// the file holds today, so the three numbers the screen exists for are read
	// before the derivation that produced them. The guard's report follows, ahead of
	// the instruction that applies the change, because it is the one line here about
	// the machine the user is working on rather than the file being edited, and it
	// is the half with no second channel: the apply instruction is repeated by the
	// write's own notice, so a short frame can afford to cut it, while a missing
	// protection can stop the distribution before the write is ever made. Then the
	// formula compressed to one sentence, then the file's own rules.
	plan := state.Plan
	facts := []iconFact{
		{Label: "Host", Value: wslHostParagraph(state.Host)},
		{Label: "Plan", Value: fmt.Sprintf("Recommended for this host: memory %s, processors %s and swap %s.",
			wslUnitLabel(plan.MemoryMB, "MB"), wslUnitLabel(plan.Processors, ""), wslUnitLabel(plan.SwapMB, "MB"))},
		{Label: "File", Value: wslCurrentParagraph(state)},
	}
	// The guard follows the numbers and comes before how to apply them: what it
	// reports is the running distribution's own limits, and the failure it names
	// is the one a short frame must not cut.
	for _, paragraph := range wslGuardParagraphs(state.Guard) {
		facts = append(facts, iconFact{Label: "Memory guard", Value: paragraph})
	}
	facts = append(facts,
		iconFact{Label: "Apply", Value: "WSL reads .wslconfig when the VM starts, so run `wsl --shutdown` on Windows and reopen the terminal " +
			"to apply it. This screen does not run it: that would close the session you are working in."},
		iconFact{Label: "Rule", Value: fmt.Sprintf("Memory is three quarters of the host's RAM rounded down to %d MB, capped so Windows "+
			"keeps 4 GiB wherever that can be honoured, and floored at half the host, which is what decides on "+
			"a host too small to honour both; processors are every logical CPU the host reports except one, so "+
			"Windows keeps one; swap is a quarter of that memory.",
			wslMemoryStepMB)},
		iconFact{Label: "Write", Value: "Only the keys the shipped template manages are written. Any other setting in your .wslconfig is kept " +
			"exactly as it is, and the previous file is backed up beside it as .wslconfig.bak-dotfiles-<stamp> " +
			"before the write."})
	return facts
}

// wslGuardRepairRow is the screen's row for the memory guard. It is the one row
// here that is not about the .wslconfig: pressing its letter installs the guard's
// two files, restarts its unit, and then reports what the host holds.
const wslGuardRepairRow = "[g] Repair the WSL memory guard"

// wslGuardParagraphs is what the screen says about the memory guard. It names
// what each missing protection costs rather than warning in general terms,
// because the two halves fail in opposite directions and both were measured on
// 2026-10-16: OOMPolicy=stop turned one OOM kill into the whole distribution
// stopping, twice, and a numeric memory.high turned a spike into a six-minute
// freeze instead of a kill.
func wslGuardParagraphs(g wslGuardState) []string {
	if !g.Installed {
		reason := g.Reason
		if reason == "" {
			reason = "its payload and its unit are not where systemd reads them"
		}
		return []string{fmt.Sprintf("The WSL memory guard is not installed on this host: %s. It is what caps the "+
			"distribution's own scope and what keeps one killed process from stopping it; the installation step "+
			"installs it, and the row below installs it here without a full run.", reason)}
	}

	paragraphs := []string{fmt.Sprintf("The memory guard is installed and %s. init.scope carries memory.high=%s, "+
		"memory.max=%s and OOMPolicy=%s.",
		wslGuardActiveLabel(g), wslGuardText(g.HighBytes), wslGuardCapLabel(g.MaxBytes), wslGuardText(g.OOMPolicy))}

	if failure := wslGuardFailureMode(g); failure != "" {
		paragraphs = append(paragraphs, failure)
	}
	return paragraphs
}

// wslGuardFailureMode names what a missing protection costs, in the terms the
// incident was measured in. It is empty when nothing is missing.
func wslGuardFailureMode(g wslGuardState) string {
	switch {
	case g.OOMPolicy != "" && g.OOMPolicy != wslGuardWantOOMPolicy:
		return fmt.Sprintf("OOMPolicy reads %s, so one OOM kill inside init.scope still stops the scope, and in "+
			"WSL every process in the distribution lives in that one scope. On 2026-10-16 that killed the whole "+
			"distribution twice.", g.OOMPolicy)
	case g.HighBytes != "" && g.HighBytes != "max":
		return fmt.Sprintf("memory.high is %s, which is a throttle rather than a limit: a spike is stalled "+
			"instead of being killed, which is the six-minute freeze of 2026-10-16 rather than a crash.", g.HighBytes)
	case g.MaxBytes == 0:
		return "memory.max is unlimited, so nothing bounds the distribution's own scope and the OOM killer " +
			"picks its victim by its own rules. On 2026-10-16 the victim was the distribution itself."
	}
	return ""
}

// wslGuardActiveLabel says whether systemd is running the guard, because
// installed files that nothing runs hold no values.
func wslGuardActiveLabel(g wslGuardState) string {
	if g.Active {
		return "systemd is running it"
	}
	return "systemd is not running it, so nothing is holding those values"
}

// wslGuardText renders a value the host may not have reported. An unread
// attribute printed as an empty string would read as "unlimited" rather than
// "not read".
func wslGuardText(value string) string {
	if value == "" {
		return "unread"
	}
	return value
}

// wslGuardCapLabel renders the scope's memory cap. Zero is the reader's "not a
// finite cap", which the cgroup spells "max".
func wslGuardCapLabel(n uint64) string {
	if n == 0 {
		return "unlimited"
	}
	if n%bytesPerMiB == 0 {
		return fmt.Sprintf("%d MB", n/bytesPerMiB)
	}
	return fmt.Sprintf("%d bytes", n)
}

// wslGuardRepairNotice is the one line the screen shows after a repair attempt.
// It reports the state read afterwards, which is what decides whether the repair
// worked; the install's own word for it is not enough.
func wslGuardRepairNotice(g wslGuardState) string {
	switch {
	case !g.Installed:
		return "The WSL memory guard is still not installed; the section above names why."
	case !g.Active:
		return "The WSL memory guard is installed but systemd is not running it, so nothing is holding those " +
			"values; systemctl status wsl-memory-guard names the reason."
	case g.OOMPolicy == wslGuardWantOOMPolicy && g.HighBytes == "max" && g.MaxBytes > 0:
		return fmt.Sprintf("The WSL memory guard is in place: init.scope stops at %s with no throttle band, and "+
			"OOMPolicy=%s.", wslGuardCapLabel(g.MaxBytes), g.OOMPolicy)
	}
	if failure := wslGuardFailureMode(g); failure != "" {
		return "The WSL memory guard is installed, but " + failure
	}
	return "The WSL memory guard is installed; the section above says what the host holds."
}

// wslUnitLabel renders one planned value for prose, where a zero is a key the
// template decided to omit rather than a row the user has not set.
func wslUnitLabel(value int, unit string) string {
	return wslValueLabel(value, unit, wslOmittedZero)
}

// wslHostParagraph names the capacities the recommendation came from, so the
// numbers on screen can be checked against the machine they describe.
func wslHostParagraph(host system.HostResources) string {
	if host.MemoryBytes == 0 && host.LogicalCPUs == 0 {
		return "The Windows host's capacities could not be read, so there is nothing to recommend. Set DOTFILES_WSL_HOST_MEMORY_MB " +
			"and DOTFILES_WSL_HOST_CPUS to name them, or set the values by hand below."
	}
	return fmt.Sprintf("Windows reports %d MiB of RAM and %d logical processors; both numbers come from the same host detector the "+
		"installation step reads them through.", host.MemoryBytes/bytesPerMiB, host.LogicalCPUs)
}

// wslCurrentParagraph says what the user's own file holds today, so the screen
// separates what is there from what is suggested.
func wslCurrentParagraph(state wslResourceState) string {
	if !state.HasFile {
		return fmt.Sprintf("There is no .wslconfig at %s yet; writing one creates it.", state.Path)
	}
	current := state.Current
	return fmt.Sprintf("Your .wslconfig at %s holds memory=%s, processors=%s and swap=%s today. A key that is absent is "+
		"omitted rather than guessed at: WSL's own proportional default applies to it.",
		state.Path, wslResourceLabel(current.MemoryMB, "MB"), wslResourceLabel(current.Processors, ""),
		wslResourceLabel(current.SwapMB, "MB"))
}

// wslResourcesUnavailableReason says why the section offers no WSL resource row,
// and is empty when it does. The reason is always stated: a gap the user cannot
// see the reason for is the failure this exists to prevent, and on a machine that
// is not WSL the answer names that machine.
//
// The sentence stays one row for every host name a machine reports in practice,
// and that is a budget rule rather than a style: the section's body shares the
// frame with its rows, so each job's line has to fit at 80x24 without the cut
// marker (the guard that reads that frame is
// TestUtilitiesBodyCarriesEveryJobAtTheFloor), and the SSH manager's own line
// spent the last row the section had. It used to name the host *and* the file's
// nature -- "and .wslconfig is a Windows file, and %s is not WSL" -- which
// wrapped it to two rows on every host; the host is the half that says why the
// utility cannot run here.
func (m Model) wslResourcesUnavailableReason() string {
	if m.SystemInfo == nil || !m.SystemInfo.IsWSL {
		// The reason names the host: util_screen_test.go's own guard reads the body
		// for the machine's name, and a sentence about "not WSL" that never says which
		// machine it is talking about is the hole that guard exists to prevent. It
		// stays one row for the host names a machine reports in practice, which is what
		// the section's floor budget requires
		// (TestUtilitiesBodyCarriesEveryJobAtTheFloor); the version that also quoted
		// .wslconfig wrapped to two rows on every host, and that wrap is what put three
		// job lines behind the cut marker before the update center's own line yielded.
		host := "this machine"
		if m.SystemInfo != nil && m.SystemInfo.OSName != "" {
			host = m.SystemInfo.OSName
		}
		return fmt.Sprintf("WSL resources are not adjustable here: %s is not WSL.", host)
	}
	if !m.WSLState.Resolved {
		return "WSL resources are not adjustable here yet: reading the Windows profile and the host's capacities."
	}
	if !m.WSLState.Available {
		return fmt.Sprintf("WSL resources are not adjustable here: %s.", m.WSLState.Reason)
	}
	return ""
}

// utilitiesShellAuditRow is the section's row for the login shell's startup, and
// the row on the screen it opens that starts the runs. One label, one constant:
// the handler matches the row the screen drew. It is offered wherever the shell
// and a terminal were resolved, and the section's body names the reason
// everywhere else.
const utilitiesShellAuditRow = "Shell Startup"

// shellAuditMeasureRow is the shell startup screen's own measure row. It is the
// same work the section's row describes, but here the row runs it where it
// stands, so it carries its own letter and Enter is left to the row that opens
// the section. The label is kept apart from utilitiesShellAuditRow because the
// section row is an opening, named by its own label, and a letter there would
// name a key the section does not read.
const shellAuditMeasureRow = "[s] Measure the shell's startup"

// shellAuditAdviceListLimit is how many recommendations the screen's text lists,
// and shellAuditAdviceOpenKey is the key that opens them.
const (
	shellAuditAdviceListLimit = 3
	shellAuditAdviceOpenKey   = "r"
)

// utilitiesSSHRow is the section's row for the SSH manager. It is offered on
// every host because the manager's own screen says what it cannot do -- a host
// with no ssh client still has a configuration file worth reading -- and the
// gap is declared on that screen rather than by a missing row.
const utilitiesSSHRow = "SSH Configuration"

// updateInstallerRow is the main menu's button for this installer's own release.
// It is offered only when a later release is published and the running file is
// the installer's own to replace, and pressing it runs the same verified swap
// `dotfiles --self-update` runs. The check that publishes it is armed when
// dotfiles opens, so the row arrives on its own rather than waiting for a visit
// to a utilities section.
const updateInstallerRow = "Update this installer"

// updateInstallerLabel is the main menu's button for this installer's own update.
// The dev channel builds the last commit on main instead of downloading a release,
// and the row says so: it is a different act, and it needs Go on the machine.
func updateInstallerLabel(channel releaseChannel) string {
	if channel == channelDev {
		return "Rebuild from the latest main commit"
	}
	return updateInstallerRow
}

// updateChannelRowPrefix is the main menu's channel row. The label names the
// channel the run follows, so the row reads as the state it changes, and the
// prefix is what the handler matches: the label's channel half changes when it is
// pressed, which a whole-string match could not survive.
const updateChannelRowPrefix = "Update channel: "

// updateChannelRow is the main menu's channel row for a channel. It is offered
// once the record names a channel -- which every check writes, success or failure
// -- so a run that has never checked shows nothing rather than a row about a
// choice it has not made.
func updateChannelRow(channel releaseChannel) string {
	return updateChannelRowPrefix + string(channel)
}

// updateCheckRow re-reads the published release. It is offered on every state of
// the screen, because asking again is what a user does when the answer is old,
// wrong or missing -- and "the check failed" is exactly when the answer is worth
// asking for again.
const updateCheckRow = "[r] Check for the latest release again"

// updateInstallRow installs the published release over this binary. It is offered
// only when there is a newer release to install and the file is this installer's
// own to replace; a copy a package manager owns gets the reason instead, in the
// screen's own words, and never a row that would refuse when it was pressed. It
// writes the running binary, so it names its own install key rather than Enter.
const updateInstallRow = "[i] Download and install the latest release"

// updateRows is what the update screen offers, derived from the state rather than
// typed: installing is there when it can be done, asking again is there whenever
// a check is not already running, and the way back is last.
func (m Model) updateRows() []string {
	var rows []string
	if m.UpdateInstallable() {
		rows = append(rows, updateInstallRow)
	}
	if !m.UpdateCheck.InFlight {
		rows = append(rows, updateCheckRow)
	}
	if len(rows) > 0 {
		rows = append(rows, m.menuSeparator())
	}
	return append(rows, "← Back")
}

// UpdateInstallable reports whether this run may replace its own binary: a newer
// release is published, the running file was resolved, and no package manager
// owns it. It is updateBlockedReason's own answer, so the question "may this run
// replace itself" and the sentence that says why it may not are one thing.
func (m Model) UpdateInstallable() bool {
	return m.UpdateCheck.Newer() && m.updateBlockedReason() == ""
}

// UpdateRowOffered reports whether the main menu has something to say about the
// published release: a later release was found, or the last check ended with the
// reason it could not read one. A run that found nothing and failed at nothing
// shows no row rather than one about an answer it does not have.
//
// It is deliberately not UpdateInstallable: a newer release that a package
// manager owns, or one whose target could not be resolved, still gets a row,
// because hiding it would read as "nothing new".
func (m Model) UpdateRowOffered() bool {
	return m.UpdateCheck.Newer() || m.UpdateCheck.Failed != ""
}

// updateBlockedReason is the one place the reason this run may not replace its
// own binary is worded: the running file could not be resolved, or a package
// manager owns it. The update screen and the main menu row's panel both show this
// sentence, so they cannot disagree about the reason. It is "" when the file is
// the installer's own to replace.
func (m Model) updateBlockedReason() string {
	switch owner := packageManagerOwner(m.UpdateTarget.Path); {
	case m.UpdateTargetErr != "":
		return fmt.Sprintf("This binary is not installable from here: %s.", m.UpdateTargetErr)
	case owner != "":
		return fmt.Sprintf(
			"This copy lives at %s and was installed by %s, so it is not the installer's to replace. "+
				"The update belongs to the package manager: run %q. %s reports the same release, and the "+
				"formula's own checksum is the release's own SHA256SUMS line for that asset.",
			m.UpdateTarget.Path, owner, homebrewUpgradeCommand, owner)
	}
	return ""
}

// updateNotInstallableReason is what the main menu row's panel says when the row
// is offered but cannot act: the ownership or target sentence when that is the
// reason, and otherwise the check's own reason it could not read a release. The
// row is never allowed to appear without naming why it will not install.
func (m Model) updateNotInstallableReason() string {
	if reason := m.updateBlockedReason(); reason != "" {
		return reason
	}
	if m.UpdateCheck.Failed != "" {
		return fmt.Sprintf("The published release could not be read: %s.", m.UpdateCheck.Failed)
	}
	return ""
}

// updateDescription is what the update screen says about itself. Every claim
// names where it comes from: the build's own identity, the tag the API answered
// with, when it was read, and -- when nothing could be read -- why not. When a
// package manager owns the file, the ownership is stated before anything is
// offered, because that is the whole answer to "can this installer update itself
// here".
func (m Model) updateDescription() []string {
	paragraphs := []string{
		"The installer is a release of its own: this reads which release is the latest and can install it " +
			"over this binary. Nothing here touches your configuration.",
	}

	// The facts, one per line, from the state's own rows, so the screen and the
	// state cannot disagree about what is known.
	for _, row := range m.UpdateCheck.displayRows() {
		if row.Value == "" {
			continue
		}
		paragraphs = append(paragraphs, row.Label+": "+row.Value)
	}

	if m.UpdateCheck.Failed == "" {
		if m.UpdateCheck.Present() {
			paragraphs = append(paragraphs, fmt.Sprintf(
				"Read from %s, and cached in update-check.json beside the installer's other state for %s, "+
					"so a start does not have to ask again.", dotfilesReleaseAPIURL, updateCheckTTL))
		}
	} else {
		paragraphs = append(paragraphs, fmt.Sprintf(
			"That is not an answer, and this screen will not pretend it is one: nothing was read from %s, "+
				"so which release is the latest is unknown here rather than current. Ask again with %q once "+
				"the network is back.", dotfilesReleaseAPIURL, updateCheckRow))
	}

	// Who owns the file an update would replace, and what that means here. It is
	// the same question the theme switch asks of a config file, asked of a binary.
	// The sentence comes from updateBlockedReason, the one place it is worded, so
	// the screen and the main menu row's panel cannot disagree about the reason.
	switch reason := m.updateBlockedReason(); {
	case reason != "":
		paragraphs = append(paragraphs, reason)
	case m.UpdateCheck.Newer():
		paragraphs = append(paragraphs, fmt.Sprintf(
			"%s would replace %s after the download is checked against that release's %s. The binary being "+
				"replaced is kept beside it as %s, so the release you have now can be put back by hand.",
			updateInstallRow, m.UpdateTarget.Path, releaseSumsAssetName, m.UpdateTarget.Path+updatePreviousSuffix))
	}

	return paragraphs
}

// shellAuditRows builds the shell startup screen's rows from its state: the one
// row that measures, the last measurement's summary when there is one, and the
// way back. The result rows are drawn from the state, so the rows and the prose
// cannot disagree about the number.
//
// The plan the press will measure is not a row, and deliberately so. The frame
// gives its rows to the numbers and its prose to everything a row cannot say --
// what will be measured, what the clean comparison measures against, what each
// recommendation rests on -- and the plan is four rows of settings that would
// take the sentences away from the numbers they explain. It lives in the text
// below with keys of its own, and the keys are named where it is described.
func (m Model) shellAuditRows() []string {
	rows := []string{shellAuditMeasureRow}
	if m.ShellAudit.Measured {
		rows = append(rows, m.menuSeparator())
		rows = append(rows, shellAuditSummaryRows(m.ShellAudit)...)
	}
	return append(rows, m.menuSeparator(), "← Back")
}

// shellAuditWarmupPhrase is the warm-up inside a sentence. A sentence that read
// "0 warm-up starts" is a count where a word belongs, and the count is the thing
// the plan does not do.
func shellAuditWarmupPhrase(warmup int) string {
	switch {
	case warmup <= 0:
		return "no warm-up first"
	case warmup == 1:
		return "one warm-up start first, thrown away"
	default:
		return fmt.Sprintf("%d warm-up starts first, thrown away", warmup)
	}
}

// shellAuditOnOff is a setting inside a sentence: a phrase for each state, because
// "on"/"off" alone does not say what is on.
func shellAuditOnOff(on bool, whenOn, whenOff string) string {
	if on {
		return whenOn
	}
	return whenOff
}

// shellAuditAdvicePrefix names what kind of claim a recommendation is. A reader
// has to be able to tell a fact about the measurement from an observation of the
// profile and from the one thing the clean comparison separated, and this prefix
// is where that is said once for all of them.
func shellAuditAdvicePrefix(kind shellAuditAdviceKind) string {
	switch kind {
	case shellAuditAdviceObserved:
		return "Observed:"
	case shellAuditAdviceConfirmed:
		return "Confirmed:"
	default:
		return "Measured:"
	}
}

// shellAuditSummaryRows is the last measurement as rows: the median with the
// count it covers, the range, the starts that timed out or failed, and the one
// function zprof put first. The rows are the data, so they win the frame's budget
// over the prose, and the prose keeps the method and the whole list.
func shellAuditSummaryRows(st shellAuditState) []string {
	var rows []string
	if st.Median > 0 {
		// The count is on the row because a median without one is not a
		// measurement: the reader has to be able to see how many starts it covers
		// and how many did not finish. The mean rides the same row: it is supported
		// by the same runs, and a row of its own would cost a line on the terminals
		// where the rows are the scarce part of the frame.
		stats := st.Stats
		if stats.Count == 0 && len(st.Samples) > 0 {
			stats = shellAuditStatsOf(st.Samples)
		}
		mean := ""
		if stats.Count > 1 {
			mean = fmt.Sprintf(" (mean %s)", shellAuditDuration(stats.Mean))
		}
		if len(st.Samples) == st.Runs {
			rows = append(rows, fmt.Sprintf("Median of %d runs: %s%s", st.Runs, shellAuditDuration(st.Median), mean))
		} else {
			rows = append(rows, fmt.Sprintf("Median of %d of %d runs: %s%s", len(st.Samples), st.Runs, shellAuditDuration(st.Median), mean))
		}
		rows = append(rows, fmt.Sprintf("Range: %s to %s", shellAuditDuration(st.Fastest), shellAuditDuration(st.Slowest)))
	} else {
		rows = append(rows, "No run finished, so there is no median")
	}

	// One row for every start that did not finish, because the two ways of not
	// finishing have one thing in common that matters on the screen: neither has a
	// duration. The reason each one failed is in the prose below.
	if st.Timeouts > 0 || st.Failures > 0 {
		var reasons []string
		if st.Timeouts > 0 {
			reasons = append(reasons, fmt.Sprintf("%d timed out at %s", st.Timeouts, shellAuditDuration(st.Timeout)))
		}
		if st.Failures > 0 {
			reasons = append(reasons, fmt.Sprintf("%d could not be started", st.Failures))
		}
		rows = append(rows, fmt.Sprintf("%d of %d runs did not finish: %s", st.Timeouts+st.Failures, st.Runs, strings.Join(reasons, ", ")))
	}

	if len(st.Attribution) == 0 {
		rows = append(rows, "No function is named; the screen says why")
	} else {
		// The row is the name and the two numbers a row can hold at 80 columns: the
		// share the instrument printed and the way it measured are in the prose below,
		// where the table it came from is described. A row wider than the frame is a
		// row the shortest frame drops, and this one is the data.
		first := st.Attribution[0]
		rows = append(rows, fmt.Sprintf("Heaviest entry: %s (%s self, %d call(s))",
			first.Name, shellAuditDuration(first.Self), first.Calls))
	}

	return rows
}

// shellAuditDescription is what the shell startup screen says about itself: the
// exact method the number comes from, every start that finished, the starts that
// did not, what zprof blamed or why nothing can be blamed, and the one thing the
// screen deliberately does not do.
func (m Model) shellAuditDescription() []string {
	st := m.ShellAudit
	if reason := m.shellAuditUnavailableReason(); reason != "" {
		return []string{reason}
	}

	// The plan as the measurement would really run it: a state whose plan was never
	// set draws the plan that would run rather than a zero, because the line on
	// screen and the press may not promise two different measurements.
	plan := clampShellAuditPlan(st.Plan)
	report := shellAuditReportOf(st)

	// The recommendations, when the user asked to open them, come before everything
	// else: they are the part of this screen a reader comes back for, and the frame
	// cuts prose from the bottom, so a list that stayed where it belongs would be
	// the first thing a short frame took away.
	var paragraphs []string
	if st.AdviceOpen {
		paragraphs = append(paragraphs, shellAuditAdviceParagraphs(report)...)
	}

	paragraphs = append(paragraphs, fmt.Sprintf("%s is started the way a terminal starts it -- `%s`, each start bounded by %s -- "+
		"so the number describes the start you wait through when you open a terminal. The plan runs %d such starts, and "+
		"the median is the middle one: not an average, and not a single run.",
		st.Shell, shellAuditPlanCommand(st.Shell, st.Kind, plan.Mode), shellAuditDuration(st.Timeout), plan.Samples))

	// The plan and the keys that change it. It is prose rather than rows because of
	// what a row cannot say: why a warm-up exists, what a clean comparison measures
	// against, and where a history would be written.
	paragraphs = append(paragraphs, fmt.Sprintf("What the press will measure: the %s start, %d runs, %s, and %s. "+
		"[m] changes the start, [n] the number of runs, [w] the warm-up, and [b] the clean comparison. That comparison "+
		"is the one measurement here that separates the shell's own startup from what you put in front of it, and it is "+
		"made in a directory of the utility's own: your files are read, never moved and never rewritten.",
		plan.Mode.label(), plan.Samples, shellAuditWarmupPhrase(plan.Warmup),
		shellAuditOnOff(plan.Baseline, "the same start is measured again without your configuration",
			"no clean comparison is measured")))

	// The statistics the finished runs support come after the plan, not before it:
	// the keys are what the reader acts on, and on a short frame the paragraph a
	// reader can do something with is the one that stays.
	if st.Measured && st.Stats.Count > 0 {
		paragraphs = append(paragraphs, shellAuditStatisticsParagraph(st))
	}

	if st.History.Enabled {
		paragraphs = append(paragraphs, fmt.Sprintf("The measurements are stored in %s beside the installer's own "+
			"record: a stamp, the counts, the median and the top of the profile, and nothing else about this machine. "+
			"[h] stops keeping them, and what is already stored is kept.", defaultShellAuditHistoryPath()))
	} else {
		paragraphs = append(paragraphs, "Nothing is written to this machine: this screen keeps a history of the runs "+
			"only if you ask it to, with [h].")
	}

	if !st.Measured {
		// A run the utility refused to make is a result about the start, not a
		// missing measurement: the reason is the answer to the press, and a screen
		// that said only "nothing has been measured yet" would leave the user
		// pressing a row that will refuse again without saying why.
		if st.RunReason != "" {
			return append(paragraphs, "The last press could not take the measurement: "+st.RunReason+" "+
				"Choosing the row above tries again; each start ends by itself or at the timeout, so the screen "+
				"cannot be held open by your shell.")
		}
		return append(paragraphs, "Nothing has been measured yet. Choosing the row above starts the runs; each one ends "+
			"by itself or at the timeout, so the screen cannot be held open by your shell, it stays usable while they "+
			"run, and [Esc] stops the ones that are left.")
	}

	// The same refusal after an earlier measurement: the numbers on screen are the
	// ones measured before the press that could not be made, and a screen that kept
	// them without saying so would present a rejected start as a fresh result.
	if st.RunReason != "" {
		paragraphs = append(paragraphs, "The last press could not take the measurement: "+st.RunReason+" "+
			"The numbers above are the measurement before it.")
	}

	// The context of the measurement: which shell it was, which start, and whether
	// a terminal was attached. A number whose context is missing is the number this
	// utility exists not to print.
	context := fmt.Sprintf("Measured: %s", st.Shell)
	if st.Version.Raw != "" {
		context += fmt.Sprintf(" (%s)", st.Version.Raw)
	}
	context += fmt.Sprintf(", the %s start, `%s`", st.Mode.label(), st.Command)
	if st.Terminal {
		if st.TerminalName != "" {
			context += fmt.Sprintf(", under %s", st.TerminalName)
		}
	} else {
		context += ", with no terminal attached"
	}
	if st.Note != "" {
		context += ". " + st.Note
	}
	paragraphs = append(paragraphs, context+".")

	// The attribution comes right after the context, before the runs behind the
	// number: it is the answer to "why is the start slow", and the median and its
	// range are already rows below. A cut on a short frame then falls on the runs
	// that support the number rather than on the one thing the user can act on.
	if len(st.Attribution) == 0 {
		paragraphs = append(paragraphs, st.AttributionReason)
	} else {
		paragraphs = append(paragraphs, shellAuditAttributionParagraph(st))
	}

	// The clean comparison and the stored trend: what part of the start is the
	// user's own doing, and whether it changed since the runs already stored. Both
	// say what they measured and neither claims more: the comparison separates the
	// configuration as a whole, and the trend is a change between two dates.
	if report.Comparison.Reason == "" {
		paragraphs = append(paragraphs, report.Comparison.headline(), report.Comparison.verdict())
	} else if st.Plan.Baseline {
		paragraphs = append(paragraphs, "The clean comparison was asked for and could not be made: "+report.Comparison.Reason)
	}

	if sentence := report.Trend.sentence(); sentence != "" {
		paragraphs = append(paragraphs, sentence)
	} else if report.Trend.Reason != "" && st.History.Enabled {
		paragraphs = append(paragraphs, "No trend to read: "+report.Trend.Reason)
	}

	if !st.AdviceOpen {
		paragraphs = append(paragraphs, shellAuditAdviceParagraphs(report)...)
	}

	// The runs behind the median, listed so the number can be reproduced by hand.
	paragraphs = append(paragraphs, "Every completed start is listed so the number can be reproduced by hand."+
		shellAuditRunList(st.Samples))

	if st.Timeouts > 0 {
		paragraphs = append(paragraphs, fmt.Sprintf("A start killed at %s is left out of the median: a hung start "+
			"has no time to report, and writing one down as a number would be inventing it.",
			shellAuditDuration(st.Timeout)))
	}
	if st.Failures > 0 {
		paragraphs = append(paragraphs, fmt.Sprintf("A start that could not be completed (%s) is left out too.", st.Failure))
	}

	return append(paragraphs, "Nothing here is changed: no startup file is written and no plugin is disabled. A "+
		"slower function is not by itself a mistake, and the fix belongs in your own configuration, applied by you.")
}

// shellAuditAdviceParagraphs renders the recommendations with the measurement each
// one came from. The kind prefix is part of the sentence on purpose: a reader has
// to be able to tell a fact about the measurement from an observation of the
// profile and from the one attribution the clean comparison separated.
func shellAuditAdviceParagraphs(report shellAuditReport) []string {
	advice := report.advice()
	if len(advice) == 0 {
		return nil
	}

	paragraphs := []string{fmt.Sprintf("Recommendations (%d), each with the measurement it came from:", len(advice))}
	for index, item := range advice {
		if index == shellAuditAdviceListLimit {
			paragraphs = append(paragraphs, fmt.Sprintf("… and %d more, all of them from the same measurement.",
				len(advice)-index))
			break
		}
		paragraphs = append(paragraphs, fmt.Sprintf("%s %s %s It comes from: %s",
			shellAuditAdvicePrefix(item.Kind), item.Headline, item.Detail, item.Evidence))
	}
	return paragraphs
}

// shellAuditRunList names every start that finished, in the order it ran, so the
// median can be checked against the runs it came from.
func shellAuditRunList(samples []time.Duration) string {
	if len(samples) == 0 {
		return ""
	}
	parts := make([]string, 0, len(samples))
	for _, sample := range samples {
		parts = append(parts, shellAuditDuration(sample))
	}
	return " The starts, in the order they ran: " + strings.Join(parts, ", ") + "."
}

// shellAuditStatisticsParagraph reports what the finished runs support beyond the
// median and the range, and names what they do not. The floors are the ones the
// statistics layer gates on, so the sentence and the number cannot disagree: a
// spread from three runs, a percentile from eight, and a trimmed median whenever
// there is an outlier to leave out.
func shellAuditStatisticsParagraph(st shellAuditState) string {
	stats := st.Stats
	if stats.Count == 0 && len(st.Samples) > 0 {
		stats = shellAuditStatsOf(st.Samples)
	}
	if stats.Count == 0 {
		return ""
	}

	// The count is carried by the plural helper, which prints it with its word:
	// adding the number again beside it produced "Across the 10 finished 10
	// starts", a sentence that reads as a measurement nobody made.
	var b strings.Builder
	fmt.Fprintf(&b, "Across the %s: mean %s", reviewPlural(stats.Count, "finished start", "finished starts"),
		shellAuditDuration(stats.Mean))

	if word, ratio, ok := stats.steadiness(); ok {
		fmt.Fprintf(&b, ", spread %s (%.0f%% of the median), which reads as %s %s startup",
			shellAuditDuration(stats.StdDev), ratio*100, shellAuditArticle(word), word)
	} else {
		fmt.Fprintf(&b, ", with no spread to read: a spread needs %s and this measurement has %d",
			reviewPlural(shellAuditSpreadFloor, "run", "runs"), stats.Count)
	}

	if stats.P95Reported {
		fmt.Fprintf(&b, ", 95th percentile %s", shellAuditDuration(stats.P95))
	} else {
		// The missing statistic is named without the words it would have been
		// printed with, so a reader scanning for a percentile does not find one that
		// was never taken: only what it would take to have one.
		fmt.Fprintf(&b, ", and no percentile: it is reported from %s and this measurement has %d",
			reviewPlural(shellAuditPercentileFloor, "run", "runs"), stats.Count)
	}

	if stats.TrimmedMedianReported {
		fmt.Fprintf(&b, ", and a trimmed median of %s, with %s beyond %s times the interquartile range left out",
			shellAuditDuration(stats.TrimmedMedian),
			reviewPlural(len(stats.Outliers), "start", "starts"), strconv.FormatFloat(shellAuditOutlierFence, 'g', -1, 64))
	}
	b.WriteString(". A number the runs cannot support is named here rather than printed in the rows.")

	return b.String()
}

// shellAuditAttributionParagraph names what the instrument that actually ran put
// first, heaviest by its own time, and says how many rows it reported, so the cut
// is declared rather than silent. The rows come before the explanation of what
// they mean: on a short frame the prose is what is cut, and the named entries are
// the part the reader can act on.
//
// The paragraph is drawn for every shell, so it belongs to the instrument of the
// state it describes: zsh's zprof table carries a total, a self time and a share,
// bash's trace carries one number per identity -- the function a traced command
// ran inside, or the file it came from -- and fish prints its own table with no
// denominator a reader could re-derive. A sentence borrowed from another shell's
// instrument would describe a measurement that did not happen.
func shellAuditAttributionParagraph(st shellAuditState) string {
	var b strings.Builder
	b.WriteString(shellAuditAttributionIntro(st))

	shown := len(st.Attribution)
	if shown > shellAuditNamedFunctions {
		shown = shellAuditNamedFunctions
	}
	for i, fn := range st.Attribution[:shown] {
		b.WriteString(shellAuditAttributionRow(st.Kind, i+1, fn))
	}
	if len(st.Attribution) > shown {
		fmt.Fprintf(&b, " %d more were %s and are not named here.", len(st.Attribution)-shown,
			shellAuditAttributionVerb(st.Kind))
	}
	b.WriteString(" " + shellAuditAttributionClosing(st.Kind))
	return b.String()
}

// shellAuditAttributionIntro says which instrument took the attribution and how
// its order was decided, in that instrument's own terms.
func shellAuditAttributionIntro(st shellAuditState) string {
	switch st.Kind {
	case shellAuditKindZsh:
		return fmt.Sprintf("zsh/zprof profiled one further start and reported %d functions, heaviest by the time "+
			"each spent on itself, in zprof's own order:", len(st.Attribution))
	case shellAuditKindBash:
		return fmt.Sprintf("bash's own trace of one further start reported %d entries -- the function a traced "+
			"command ran inside, or the file it came from -- ranked by the time between their trace lines:",
			len(st.Attribution))
	case shellAuditKindFish:
		return fmt.Sprintf("fish's own --profile-startup profiled one further start and reported %d entries, "+
			"heaviest by the time each spent on itself:", len(st.Attribution))
	default:
		return fmt.Sprintf("one further start was profiled and reported %d entries, heaviest first:",
			len(st.Attribution))
	}
}

// shellAuditAttributionVerb is what the instrument did to the rows it did not
// report here, so the declared cut is in the paragraph's own words.
func shellAuditAttributionVerb(kind shellAuditKind) string {
	switch kind {
	case shellAuditKindZsh, shellAuditKindFish:
		return "profiled"
	case shellAuditKindBash:
		return "traced"
	default:
		return "reported"
	}
}

// shellAuditAttributionRow is one named entry as its own instrument reports it:
// zsh's table prints a total, a self time, a call count and a share, and the
// other two print a time of their own and a count. A share is only drawn when the
// instrument printed one.
func shellAuditAttributionRow(kind shellAuditKind, index int, fn shellAuditFunction) string {
	if kind == shellAuditKindZsh {
		row := fmt.Sprintf(" %d. %s -- %s total, %s self, %d call(s)", index, fn.Name,
			shellAuditZprofTime(shellAuditMillis(fn.Total)), shellAuditZprofTime(shellAuditMillis(fn.Self)), fn.Calls)
		if fn.Percent != "" {
			row += fmt.Sprintf(", %s of the profiled start", fn.Percent)
		}
		return row + "."
	}

	row := fmt.Sprintf(" %d. %s -- %s", index, fn.Name, shellAuditDuration(fn.Self))
	if fn.Total > fn.Self {
		row += fmt.Sprintf(", %s counting everything it called", shellAuditDuration(fn.Total))
	}
	return fmt.Sprintf("%s, %d call(s).", row, fn.Calls)
}

// shellAuditAttributionClosing says what the numbers mean for this instrument:
// the self time of a table that prints a total, or the double counting of a
// trace, which is a ranking rather than a partition.
func shellAuditAttributionClosing(kind shellAuditKind) string {
	closing := "The profiled start is a start of its own, so its numbers stand beside the median rather than inside it."
	switch kind {
	case shellAuditKindZsh:
		return "The self time is the function's own cost: a function whose total is larger than its self " +
			"time is waiting on the ones it called. " + closing
	case shellAuditKindBash:
		return "A traced command inside a function is counted under both the function and the file it came " +
			"from, so these rows are a ranking of where the time went rather than a partition of it. " + closing
	case shellAuditKindFish:
		return "No share is printed for this table: every row's own sum already counts the rows under it, " +
			"so a percentage taken from those sums would count the same time again. " + closing
	default:
		return closing
	}
}

// shellAuditArticle is the indefinite article for the word the screen prints after
// it, so the sentence reads as English rather than as a template with a slot in
// it. Only the words this screen actually prints need it, and the rule is the one
// a reader applies.
func shellAuditArticle(word string) string {
	if word == "" {
		return "a"
	}
	switch word[0] {
	case 'a', 'e', 'i', 'o', 'u':
		return "an"
	}
	return "a"
}

// shellAuditNotice is the footer's result line: what is running, while something
// is. It is empty the rest of the time, so a finished measurement spends no row
// on a stale "done".
// shellAuditNotice is the line above the frame's rule: the measurement in flight,
// start by start, and how to stop it; or, when nothing is running, what the last
// thing this screen had to write down could not do.
func (m Model) shellAuditNotice() string {
	st := m.ShellAudit
	if !st.Measuring {
		return st.HistoryError
	}

	progress := st.Progress
	if progress.Part == "" {
		progress.Part = shellAuditPartVersion
	}
	return fmt.Sprintf("Measuring: %s -- %d starts of %s, each bounded by %s. [Esc] stops the starts that are left.",
		progress.label(), st.Runs, st.Shell, shellAuditDuration(st.Timeout))
}

// shellAuditUnavailableReason says why the section offers no shell startup row,
// and is empty when it does. A gap the user cannot see the reason for is the
// failure this exists to prevent.
func (m Model) shellAuditUnavailableReason() string {
	st := m.ShellAudit
	if !st.Resolved {
		return "The shell's startup is not measurable here yet: the login shell is still being resolved."
	}
	if !st.Available {
		return fmt.Sprintf("The shell's startup is not measurable here: %s", st.Reason)
	}
	return ""
}

// shellAuditMillis converts a measured duration into zprof's unit, so the two
// kinds of number the screen shows are formatted by the same two helpers.
func shellAuditMillis(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}

// themePreviewSwatchRoles is the palette roles the live preview paints, in the
// order it paints them. They are the roles every offered theme has.
var themePreviewSwatchRoles = []string{
	"base", "selection", "text", "red", "green", "yellow", "blue", "magenta", "cyan",
}

// appliedThemeColors is the chrome the run wears: the palette of the theme the
// record names, read from the definitions. It is derived every time rather than
// kept as a second palette, so the record is the one place the applied theme
// lives. A run with no record, one whose definition has not been read yet, or one
// whose record names the default theme keeps the installer's own adaptive chrome
// -- the default theme is that chrome, so applying it changes nothing and there
// is no single dark value elbowing the adaptive one aside.
func (m Model) appliedThemeColors() (uiColors, bool) {
	if m.DotfilesThemeRecord == nil || m.DotfilesThemeRecord.Theme == "" || m.DotfilesThemeRecord.Theme == defaultThemeID {
		return uiColors{}, false
	}
	def, ok := themeByID(m.DotfilesThemes, m.DotfilesThemeRecord.Theme)
	if !ok {
		return uiColors{}, false
	}
	colors, err := themePreviewColors(def)
	if err != nil {
		return uiColors{}, false
	}
	return colors, true
}

// previewThemeDef is the theme the cursor is on, when it is on a theme row. The
// preview restyles only what it can reach from the model: the values are the
// definition's own, never a second copy kept for drawing.
func (m Model) previewThemeDef() (themeDefinition, bool) {
	options := m.GetCurrentOptions()
	if m.Cursor < 0 || m.Cursor >= len(options) {
		return themeDefinition{}, false
	}
	return m.dotfilesThemeForRow(options[m.Cursor])
}

// themePreviewLines is the one preview row: a label that says the colours are a
// preview and that nothing has been applied, then a swatch for each role the
// theme holds, painted in the theme's real colour. It is built from the same
// definition the apply writes, so the preview cannot show a colour the switch
// would not, and it is trimmed to the width it is drawn at by dropping whole
// swatches rather than splitting a styled cell mid-sequence.
func (m Model) themePreviewLines(width int) []string {
	def, ok := m.previewThemeDef()
	if !ok {
		return nil
	}

	label := "Preview (nothing applied) — " + def.Name + ":"
	if lipgloss.Width(label) > width {
		label = truncateRunes(label, width)
	}

	// "Preview" is the block's chip and the rest is its dim metadata, so the one
	// word that says the colours below are a preview carries the brand tone while
	// the sentence that qualifies it stays quiet. It counts the same columns
	// either way, so the swatch budget is unchanged.
	const previewChip = "Preview"
	var b strings.Builder
	if lipgloss.Width(label) <= lipgloss.Width(previewChip) {
		b.WriteString(MutedStyle.Render(label))
	} else {
		b.WriteString(chip(previewChip))
		b.WriteString(MutedStyle.Render(strings.TrimPrefix(label, previewChip)))
	}
	for _, role := range themePreviewSwatchRoles {
		hex := def.Palette[role]
		if hex == "" {
			continue
		}
		if lipgloss.Width(b.String())+2 > width {
			break
		}
		b.WriteString(lipgloss.NewStyle().Background(lipgloss.Color(hex)).Render("  "))
	}
	return []string{b.String()}
}

// truncateRunes cuts s to at most width columns without splitting a rune, so a
// narrow terminal trims the label instead of drawing past the frame.
func truncateRunes(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	var b strings.Builder
	used := 0
	for _, r := range s {
		w := lipgloss.Width(string(r))
		if used+w > width {
			break
		}
		b.WriteRune(r)
		used += w
	}
	return b.String()
}

// dotfilesThemeRow is the menu row for one theme. It names the tools the theme
// cannot paint, so the exclusion list is drawn where the choice is made rather
// than only in the guard's log.
func dotfilesThemeRow(def themeDefinition) string {
	if note := themeExclusionNote(def); note != "" {
		return "Apply the " + def.Name + " theme (" + note + ")"
	}
	return "Apply the " + def.Name + " theme"
}

// themeExclusionNote is the tools a theme cannot paint, said the way the option
// row says them, and empty when it paints them all. The row and the card both
// read it here, so the list the key handler matches and the card the screen draws
// cannot disagree about what a theme leaves out.
func themeExclusionNote(def themeDefinition) string {
	_, uncovered := themeCoverage(def)
	if len(uncovered) == 0 {
		return ""
	}
	return "not " + strings.Join(uncovered, ", ")
}

// dotfilesThemeForRow finds the theme a row names, so the handler switches on the
// definition rather than on a label it re-parses.
func (m Model) dotfilesThemeForRow(row string) (themeDefinition, bool) {
	for _, def := range m.DotfilesThemes {
		if def.Complete() && dotfilesThemeRow(def) == row {
			return def, true
		}
	}
	return themeDefinition{}, false
}

// dotfilesThemeOptions is one row per complete theme, in the definitions' order.
func (m Model) dotfilesThemeOptions() []string {
	var rows []string
	for _, def := range m.DotfilesThemes {
		if def.Complete() {
			rows = append(rows, dotfilesThemeRow(def))
		}
	}
	return rows
}

func (m Model) renderMainMenu() string {
	// The mockup's main menu is the card of two-line rows beside the selected
	// row's detail panel: no greeting and no card title above the list. The
	// greeting belongs to the dashboard, and the header already names the screen.
	hints := m.mainMenuHints()

	// A two-line row costs two rows, so the shape is taken only where the frame can
	// still hold the rows it spends on everything else. A floor that cannot keeps
	// the one-line row, so 80x24 still shows the whole menu; the wide screen takes
	// the mockup's two-line shape.
	twoLine := m.mainMenuUsesTwoLineRows()

	body := []string{}
	// The mockup's wide main menu is the card beside the detail panel: no greeting
	// and no card title above it. The floor keeps the one-line shape and its
	// greeting, because there the greeting is the only line that says what the
	// screen is asking and the card's title is its only brand mark.
	if !twoLine {
		if g := m.greeting(); g != "" {
			body = append(body, MutedStyle.Render(g))
		}
		body = append(body, MutedStyle.Render("What would you like to do?"))
	}
	if notice := m.mainMenuTrainerNotice(); notice != "" {
		body = append(body, notice)
	}
	if notice := m.mainMenuUpdateNotice(); notice != "" {
		body = append(body, notice)
	}
	cardTitle := ""
	if !twoLine {
		cardTitle = BrandStyle.Render("dotfiles")
	}
	body = append(body, panelBlockGrouped(cardTitle, layoutFor(m).RowMeasure, m.mainMenuGroups(twoLine))...)

	// The header's vital names the build the run would install from: the one fact
	// about this screen that is not a choice, and the thing a bug report asks for
	// first. The frame's header already carries the screen's name, so nothing is
	// repeated to say it.
	//
	// The main menu is the screen that asks what is about to happen, so it offers
	// the plan panel. The frame takes it only where there is room for it and drops
	// it below the two-column floor.
	return m.frame(m.headerName(), VersionLabel(), body, hints)
}

// mainMenuRowsNeeded is the card's own height for one of its two row shapes: the
// two borders, the group divider before each group after the first, and one or
// two rows per option. It is what makes the two-line row conditional on the
// floor instead of on a height the thresholds would have to repeat.
func (m Model) mainMenuRowsNeeded(twoLine bool) int {
	rows := 2 // the card's top and bottom borders
	for _, option := range m.GetCurrentOptions() {
		if strings.HasPrefix(option, menuSeparatorPrefix) {
			rows++ // the divider between two groups
			continue
		}
		if twoLine {
			rows += 2
		} else {
			rows++
		}
	}
	return rows
}

// mainMenuHints is the footer the main menu hands the frame.
func (m Model) mainMenuHints() []installerHint {
	return []installerHint{hintUp, hintDown, hintOpen, hintQuit}
}

// mainMenuRowsBudget is the rows the main menu's body may spend, measured the way
// the renderer measures it: the frame minus its chrome and the footer it reserves.
func (m Model) mainMenuRowsBudget() int {
	panels := m.panelsFor()
	return installerBodyRows(m.Height, m.footerReserveRows(contentWidth(m), m.panelHints(panels, m.mainMenuHints())))
}

// mainMenuUsesTwoLineRows reports whether this terminal draws the mockup's
// two-line rows. It is the one place the shape is decided, so a guard can ask
// which shape a size gets without repeating the arithmetic.
func (m Model) mainMenuUsesTwoLineRows() bool {
	return m.mainMenuTwoLine(m.mainMenuRowsBudget())
}

// mainMenuTwoLine reports whether the mockup's two-line rows fit the frame the
// menu was given. The card is not the only thing the rows have to leave room for:
// on a one-column terminal the frame draws the active panel's summary and the
// creature in the rows the body did not need, and both are facts the screen owes
// the reader -- the summary is the plan's own answer, and the creature is the
// frame's. A two-line row that spends those rows is the mockup's shape winning
// over the mockup's own evidence, so the one-line row keeps the floor instead.
//
// The two-column frame spends its panel beside the body rather than under it, so
// there only the creature's reserved block competes with the card.
func (m Model) mainMenuTwoLine(rowsBudget int) bool {
	l := layoutFor(m)
	spare := m.companionHeightNow()
	if !l.TwoColumn {
		spare += len(m.rotatorLines(m.panelsFor(), l.Inner))
	}
	// The notices above the card spend body rows too. At the first height the
	// card fits, a notice arriving had no slack and could push the footer off.
	return rowsBudget-m.mainMenuNoticeRows() >= m.mainMenuRowsNeeded(true)+spare
}

// mainMenuNoticeRows is the rows the notices above the card take from the body.
func (m Model) mainMenuNoticeRows() int {
	rows := 0
	if m.mainMenuTrainerNotice() != "" {
		rows++
	}
	if m.mainMenuUpdateNotice() != "" {
		rows++
	}
	return rows
}

// mainMenuGroup is one titled group inside the main menu's card: the rows it
// holds, already drawn as bars with the flat cursor index they came from, and
// the title its divider carries.
type mainMenuGroup struct {
	Title string
	Lines []string
}

// mainMenuGroups splits the main menu's options into the groups its card draws.
// The split is the separators in the option list itself, so the rows the card
// groups, the rows the handler matches and the indices the cursor uses are one
// list; a row added to the menu lands in the group its separator puts it in.
func (m Model) mainMenuGroups(twoLine bool) []mainMenuGroup {
	options := m.GetCurrentOptions()
	// The first group is the card's own title: the card is the product's, and a
	// second label above the first row would be two names for it.
	titles := []string{"", "Learn", "Session"}
	groups := []mainMenuGroup{{Title: titles[0]}}
	for i, option := range options {
		if strings.HasPrefix(option, menuSeparatorPrefix) {
			group := len(groups)
			title := "More"
			if group < len(titles) {
				title = titles[group]
			}
			groups = append(groups, mainMenuGroup{Title: title})
			continue
		}
		at := len(groups) - 1
		// The DD gag owns its row in both shapes: it replaces the option's own
		// content, so the two-line renderer must not draw the row it struck.
		line := m.menuRowAt(options, i, m.Cursor)
		if twoLine && !(m.MenuGagActive && i == m.MenuGagRow) {
			line = m.mainMenuRowLine(options[i], i == m.Cursor, true)
		}
		groups[at].Lines = append(groups[at].Lines, line)
	}
	return groups
}

// mainMenuRowParts is the main menu's icon and one-line description for a row.
// The icon is decoration: a terminal without the Nerd Font still draws the label,
// and the glyph is never the row's name. The subtitle is the mockup's copy.
func mainMenuRowParts(label string) (icon, subtitle string) {
	switch {
	case label == "Start Installation":
		return "\uf135", "Install or update your dotfiles"
	case strings.HasPrefix(label, "Update channel:"):
		return "\uf021", "Switch between dev / stable"
	case strings.HasPrefix(label, "Update this installer"), strings.HasPrefix(label, "Update to "):
		return "\uf019", "Install the release this run found"
	case label == "Restore from Backup":
		return "\uf1da", "Rollback to a previous setup"
	case label == "Vim & Neovim":
		return "\uf02d", "Vim & Neovim, tools, tips..."
	case label == "Learn About Tools":
		return "\uf0ad", "The tools this run would install"
	case label == "Utilities":
		return "\uf120", "Utilities and advanced tools"
	case label == "Exit":
		return "\uf08b", "Goodbye!"
	}
	return "", ""
}

// mainMenuRowLine draws one main-menu row. The one-line shape is the floor's and
// the generic menu's; the two-line shape is the mockup's: the icon and the title
// on the row, the muted description under it. The filled bar covers the title
// line only, so the subtitle is read, not echoed.
//
// The bar keeps the selected marker the one-line row draws, as the mockup's filled
// row keeps its arrow: a bar that differs from its neighbours by colour alone is
// invisible on a terminal without colour, and the marker is what says where the
// cursor is there.
func (m Model) mainMenuRowLine(label string, selected, twoLine bool) string {
	if !twoLine {
		return m.rowBar(label, selected, "")
	}
	icon, subtitle := mainMenuRowParts(label)
	measure := m.rowMeasure()
	// The marker costs the same two columns rowBar's own marker does, so the two
	// shapes put their text on the same column.
	const markerWidth = 2
	textWidth := measure - markerWidth
	if textWidth < 1 {
		textWidth = 1
	}
	title := label
	if icon != "" {
		title = icon + " " + label
	}
	title = truncate(title, textWidth)
	pad := textWidth - lipgloss.Width(title)
	var line string
	switch {
	case selected:
		line = RowBarMarkerStyle.Render("▸ ") + RowBarStyle.Render(title+strings.Repeat(" ", pad))
	case icon != "":
		line = "  " + AccentKeyStyle.Render(icon) + " " + InkStyle.Render(truncate(label, textWidth-lipgloss.Width(icon)-1)+strings.Repeat(" ", pad))
	default:
		line = InkStyle.Render("  " + title + strings.Repeat(" ", pad))
	}
	if subtitle == "" {
		return line
	}
	// The description is indented under the title's own text, not under the icon:
	// the mockup's second line starts where the row's name starts, so the icon and
	// the marker read as the row's gutter.
	indent := strings.Repeat(" ", markerWidth)
	if icon != "" {
		indent += "  "
	}
	return line + "\n" + indent + MutedStyle.Render(subtitle)
}

// stripStepPrefix removes a leading "Step N: " from a wizard title, so the step
// number is not stated twice: the frame's header owns it now, as a bar and a
// count. The title keeps the part that names the choice.
func stripStepPrefix(title string) string {
	if strings.HasPrefix(title, "Step ") {
		if i := strings.Index(title, ": "); i != -1 {
			return title[i+2:]
		}
	}
	return title
}

// renderSetup draws the one screen the wizard's questions moved to: the mockup's
// table of decisions, each row carrying the value the choices hold, with the
// cursor on the row the value keys act on and the row's description under the
// table. The rows and their values come from setup_rows.go and the drawing from
// setup_view.go, so a decision cannot lose its row by being left out of a
// renderer written a second time.
func (m Model) renderSetup() string {
	l := layoutFor(m)

	hints := m.setupHints()
	frameHints := m.panelHints(m.panelsFor(), hints)
	// The footer's RESERVATION, not the rows it draws: the Space hint appears when the
	// cursor lands on a switch row, and an action that appears must not take its row
	// from the body -- taking it from the body changes the body's height, which moves
	// the centred table and the band under it.
	budget := installerBodyRows(m.Height, m.footerReserveRows(l.Inner, frameHints))

	// The shape is decided once from the frame's rows and the width, and the body
	// is composed against it: the mockup's boxed table where it fits, the same rows
	// without the box where it would hide one, and the band under it in whichever
	// form the frame has room for.
	body := m.setupBodyLines(m.setupShapeFor(budget, l), budget, l)

	// The step counter is the wizard's vital sign: the setup screen is the first
	// of the two mandatory screens, and the detours of this screen inherit the
	// same count rather than claiming a step the flow does not have.
	vital := ""
	if current, total := m.wizardProgress(); total > 0 {
		vital = meterRendered(current, total, 10)
	}
	return m.frame(m.headerName(), vital, centeredBody(body, l), hints)
}

// renderSelection draws the wizard's own questions: the pickers for the
// operating system, the terminal, the shell, the font, the multiplexer, the
// editor and the terminal warning. They are menus like the hubs are, so they
// draw through the same body (menuCardBody): the question as the page's title,
// the model's own description under it, and the answers in the frame's card.
//
// They used to draw a bare list, which made a wizard question a different shape
// from every other list in the program -- and the shape a person sees while they
// are answering the run should be the one they already know. The answers, their
// order, the keys, the panels and the wizard's step counter are unchanged.
//
// The six questions -- OS, terminal, font, shell, multiplexer and editor -- are
// drawn in the reference grammar instead (os_select_view.go,
// terminal_select_view.go, font_select_view.go, shell_select_view.go,
// wm_select_view.go, nvim_select_view.go): the page is the card, its title a chip
// on the border and its description the caption beside it, with the answers as
// two-line rows. They keep this renderer's frame, hints and step counter, and they
// fall back to the compact body below when the frame cannot hold the card.
func (m Model) renderSelection() string {
	hints := []installerHint{hintUp, hintDown}
	if m.Cursor >= 0 {
		// With no recorded answer nothing is highlighted; naming Enter there would
		// advertise a press that chooses nothing. The arrows are the way to pick.
		hints = append(hints, hintChoose)
	}
	hints = append(hints, hintBack)

	// The description is the question's own note (the WSL terminal note is the
	// longest), so it spends the rows it needs instead of being capped: the card's
	// window is measured from what the head left, so a long note shortens the list
	// rather than pushing it off the frame.
	panels := m.panelsFor()
	bodyHints := m.panelHints(panels, hints)
	if card, ok := m.selectionQuestionCard(bodyHints); ok {
		return m.frame(m.headerName(), m.selectionVital(), card, hints)
	}
	body, _, _, _ := m.menuCardBody(stripStepPrefix(m.GetScreenTitle()), m.GetScreenDescription(), bodyHints)

	return m.frame(m.headerName(), m.selectionVital(), body, hints)
}

// selectionQuestionCard draws a question in the reference grammar's card, or
// reports that this screen does not draw one -- or that this frame cannot hold it.
// The screens that draw a card are the six whose answers have a subtitle to put
// under the title; the rest keep the compact list.
func (m Model) selectionQuestionCard(hints []installerHint) ([]string, bool) {
	switch m.Screen {
	case ScreenOSSelect:
		return m.osSelectCardBody(hints)
	case ScreenTerminalSelect:
		return m.terminalSelectCardBody(hints)
	case ScreenFontSelect:
		return m.fontSelectCardBody(hints)
	case ScreenShellSelect:
		return m.shellSelectCardBody(hints)
	case ScreenWMSelect:
		return m.wmSelectCardBody(hints)
	case ScreenNvimSelect:
		return m.nvimSelectCardBody(hints)
	}
	return nil, false
}

// selectionVital is the wizard's step counter in the header: the pickers are
// detours of the setup screen and inherit its meter rather than claiming a step
// of their own. It lives beside renderSelection because both of that screen's
// shapes carry it, so the two cannot disagree about the count.
func (m Model) selectionVital() string {
	if current, total := m.wizardProgress(); total > 0 {
		return meterRendered(current, total, 10)
	}
	return ""
}

// renderLearnTools draws the hub: one row per page of the in-installer wiki,
// built from learnToolRows() so the menu and the key handler read one table.
func (m Model) renderLearnTools() string {
	return m.renderMenu("Select what to learn about")
}

// renderVimHub draws the Vim hub: the trainer, the keymaps reference, the
// LazyVim guide and the Neovim page, under one description. It is the screen the
// main menu's single Vim row opens, and it is a menu like the Learn hub, so it
// draws through the same renderer.
func (m Model) renderVimHub() string {
	return m.renderMenu(m.GetScreenDescription())
}

// renderLearnTooling, renderLearnPlugins and renderLearnToolset draw the three
// read-only reference pages. They share one renderer and one scroll value: only
// one of them is on screen at a time, and the value is reset when the page is
// opened and when it is left.
func (m Model) renderLearnTooling() string {
	return m.renderReferenceTable(shellToolingTable(), m.ReferenceTableScroll)
}

func (m Model) renderLearnPlugins() string {
	return m.renderReferenceTable(pluginSetsTable(), m.ReferenceTableScroll)
}

func (m Model) renderLearnToolset() string {
	return m.renderReferenceTable(machineToolsetTable(), m.ReferenceTableScroll)
}

// renderLearnPrompts draws the prompt the reader is reading, or the list to pick
// one from. The list keys its pages by the lowercased label the learn handler
// already writes to ViewingTool.
func (m Model) renderLearnPrompts() string {
	if m.ViewingTool != "" {
		return m.renderToolInfo(GetPromptInfo(), m.ViewingTool)
	}
	return m.renderLearnMenu("Select a prompt to learn more about it")
}

// renderLearnFont draws the font page. The run asks one font question and this is
// the answer to it, so there is no list to pick from.
func (m Model) renderLearnFont() string {
	return m.renderSingleToolInfo(GetFontInfo())
}

func (m Model) renderLearnTerminals() string {
	if m.ViewingTool != "" {
		return m.renderToolInfo(GetTerminalInfo(), m.ViewingTool)
	}
	return m.renderLearnMenu("Select a terminal to learn more about it")
}

func (m Model) renderLearnShells() string {
	if m.ViewingTool != "" {
		return m.renderToolInfo(GetShellInfo(), m.ViewingTool)
	}
	return m.renderLearnMenu("Select a shell to learn more about it")
}

func (m Model) renderLearnWM() string {
	if m.ViewingTool != "" {
		return m.renderToolInfo(GetWMInfo(), m.ViewingTool)
	}
	return m.renderLearnMenu("Select a window manager to learn more about it")
}

func (m Model) renderLearnNvim() string {
	if m.ViewingTool == "features" {
		return m.renderSingleToolInfo(GetNvimInfo())
	}
	return m.renderLearnMenu("Explore Neovim features and keybindings")
}

// renderLearnMenu renders one of the learn screens' menus. The four screens
// differ only in their description and the tool table they look up, so the
// title, the menu and the legend are shared and cannot drift apart on the keys
// they promise.
func (m Model) renderLearnMenu(description string) string {
	return m.renderMenu(description)
}

func (m Model) renderToolInfo(tools map[string]ToolInfo, toolKey string) string {
	info, exists := tools[toolKey]
	if !exists {
		return m.deadEndScreen("Tool not found", "Press [Esc] to go back.")
	}

	return m.renderSingleToolInfo(info)
}

// renderSingleToolInfo draws one tool page in the reference grammar: the tool's
// name as the card's chip, its website as the caption, the description as the
// card's prose and the pros and cons under their own titled dividers
// (toolInfoCardBody, reference_cards.go). A frame too narrow or too short for the
// card keeps the older body below, which spends its rows on the facts instead of
// on a border.
func (m Model) renderSingleToolInfo(info ToolInfo) string {
	hints := []installerHint{hintBack, hintQuit}
	if body, ok := m.toolInfoCardBody(info, hints); ok {
		return m.frame(m.headerName(), "", body, hints)
	}

	width := contentWidth(m)
	bodyRows := installerBodyRows(m.Height, m.footerRowCount(width, hints))

	desc := wrapText(info.Description, width, 2)
	// Fixed rows: title, description, website, blank, pros chip, blank, cons chip.
	fixed := 1 + len(desc) + 1 + 1 + 1 + 1 + 1
	lists := bodyRows - fixed
	if lists < 2 {
		lists = 2
	}
	prosRows := lists / 2
	consRows := lists - prosRows

	var body []string
	body = append(body, BrandStyle.Render(info.Name))
	for _, row := range desc {
		body = append(body, MutedStyle.Render(row))
	}
	body = append(body, MutedStyle.Render(truncate(info.Website, width)))
	body = append(body, "")

	// Pros and cons are blocks: a chip names each one and its entries sit behind a
	// Rule-coloured gutter. The glyph carries the good/bad meaning so the chip
	// stays brand chrome and neither green nor amber is used as decoration. Each
	// entry is wrapped rather than cut, so a reason is read instead of guessed at.
	//
	// The split is the fair half each, and the rows the pros did not need go to the
	// cons: one list short and the other long is the normal case, and a frame that
	// leaves four rows empty under a note that says "and 3 more" is a frame that
	// hid facts it had the room to show.
	pros := wrappedListRows(info.Pros, referenceReasonMark, prosRows, width-2, InfoStyle)
	body = append(body, SuccessStyle.Render("✓")+" "+chip("Pros"))
	body = append(body, gutteredBlock(pros)...)
	body = append(body, "")
	cons := wrappedListRows(info.Cons, referenceReasonMark, consRows+prosRows-len(pros), width-2, MutedStyle)
	body = append(body, WarningStyle.Render("✗")+" "+chip("Cons"))
	body = append(body, gutteredBlock(cons)...)

	return m.frame(m.headerName(), "", body, hints)
}

// keymapTableMinRows keeps the table readable when the frame is at its floor.
const keymapTableMinRows = 3

// keymapTableColumns derives the three column widths of a keymap table from the
// frame and the category's own keys: the keys column is the wider of a quarter
// of the frame and the category's longest key, so a 37-column Herdr binding is
// shown whole instead of cut; the mode column fits the longest mode name; and
// the description keeps at least a third of the frame. The five tables used to
// hard-code %-15s, %-18s, %-20s and the like with no rule between them.
func keymapTableColumns(width, longestKey int) (keys, mode, description int) {
	keys = width / 4
	if keys < longestKey {
		keys = longestKey
	}
	mode = 8
	maxKeys := width - mode - width/3 - 2
	if maxKeys < 4 {
		maxKeys = 4
	}
	if keys > maxKeys {
		keys = maxKeys
	}
	description = width - keys - mode - 2
	if description < 1 {
		description = 1
	}
	return keys, mode, description
}

// renderKeymapTable renders one keymap category through the shared table
// renderer: a keymap table is a three-column table whose headers happen to be
// Keys, Mode and Description. The category's own rows are built by
// keymapCategoryTable, which the scroll keys also read, so the page the reader
// scrolls and the page the keys clamp are one table.
func (m Model) renderKeymapTable(category KeymapCategory, scroll int) string {
	return m.renderReferenceTable(keymapCategoryTable(category), scroll)
}

// renderReferenceTable renders one read-only table page in the reference grammar:
// the table's own name as the card's chip, its description as the caption and the
// rows the frame has room for inside the card. Its scroll value IS the top-row
// offset (0 shows the first row, offsetWindowMax shows the last), so it windows
// through the same helpers the key handlers clamp with; the last row is reachable.
//
// The shape is the table's own (see referenceTable.TwoLine). A learn page draws
// each entry as the grammar's two-line row -- the name with its kind trailing at the
// right edge, the sentence about it on the rows under them, whole -- because a
// learn page exists to be read and its sentences are longer than any column the
// frame can spare. A keymap table keeps its three columns, where the columns are
// the page.
func (m Model) renderReferenceTable(table referenceTable, scroll int) string {
	width, content := m.referenceCardWidth()
	rows := m.referenceRowBudget(table)
	start, end := referenceWindow(table, content, rows, scroll)

	body := referenceTableCard(table, start, end, width, content)

	// The scroll range is the table's vital sign, so it lives in the header and
	// costs the body no row: the old body held a scroll row even when the list
	// fit, and reading it back out of the header still satisfies the scroll
	// reachability tests.
	vital := ""
	if end-start < len(table.Rows) {
		vital = scrollVital("Showing", start+1, end, len(table.Rows))
	}
	return m.frame(m.headerName(), vital, body, referencePageHints())
}

// referenceEntryRows draws the two-line entries of a learn page in the grammar's
// row: the name as the row's title with its kind trailing at the right edge, and
// the sentence under both, wrapped to the card's content rather than cut at a
// column. The sentence is the fact the reader came for, so it is drawn whole on
// as many rows as it needs; the name and the kind are the row's metadata and share
// one line.
func referenceEntryRows(table referenceTable, start, end, width int) []string {
	var rows []string
	for i := start; i < end; i++ {
		row := table.Rows[i]
		rows = append(rows, cardRowLines(cardRow{Title: row.Name, Trailing: row.Kind}, width, false)...)
		for _, line := range wrapText(row.What, max(1, width-grammarRowGutter), 0) {
			rows = append(rows, MutedStyle.Render(strings.Repeat(" ", grammarRowGutter)+line))
		}
	}
	return rows
}

// referenceTableRows draws the three-column table the keymap pages keep: the
// columns sized from the frame and the page's own keys, every cell in one row.
// The columns are the page here, so they keep their own measure; what the card
// gave them is the grammar's header role above them.
func referenceTableRows(table referenceTable, start, end, width int) []string {
	longestName := 0
	for _, row := range table.Rows {
		longestName = max(longestName, lipgloss.Width(row.Name))
	}
	name, kind, what := keymapTableColumns(width, longestName)

	rows := []string{TableHeaderStyle.Render(truncate(padRight(table.Headers[0], name)+" "+
		padRight(table.Headers[1], kind)+" "+table.Headers[2], width))}
	rows = append(rows, rule(width))
	for i := start; i < end; i++ {
		row := table.Rows[i]
		rows = append(rows, KeyStyle.Render(padRight(truncate(row.Name, name), name))+" "+
			MutedStyle.Render(padRight(truncate(row.Kind, kind), kind))+" "+
			InfoStyle.Render(truncate(row.What, what)))
	}
	return rows
}

// referenceEntryCost is the rows one entry spends in the table's shape: one for the
// name and its kind, and one for each row of the sentence under them. A
// three-column table spends one per entry.
func referenceEntryCost(table referenceTable, row referenceRow, width int) int {
	if !table.TwoLine {
		return 1
	}
	return 1 + len(wrapText(row.What, max(1, width-2), 0))
}

// referenceWindow is the entries that fit the budget from an offset. It is the
// window the page draws and the same one its keys may scroll to, so the rows a
// page spends and the rows its keys count are one number: a two-line entry costs
// two rows, and a count of entries cannot say that.
func referenceWindow(table referenceTable, width, budget, offset int) (start, end int) {
	if !table.TwoLine {
		// The three-column table spends two rows on its header and rule, which the
		// body budget already counts.
		return offsetWindow(offset, budget, len(table.Rows))
	}
	if offset < 0 {
		offset = 0
	}
	if offset > len(table.Rows) {
		offset = len(table.Rows)
	}
	used := 0
	end = offset
	for end < len(table.Rows) {
		cost := referenceEntryCost(table, table.Rows[end], width)
		if used+cost > budget {
			break
		}
		used += cost
		end++
	}
	if end == offset && offset < len(table.Rows) {
		// A frame too short for even one entry still draws it whole rather than
		// hiding the page: the entry is a fact and the frame has failed to fit it.
		end = offset + 1
	}
	return offset, end
}

// referenceMaxScroll is the greatest top-row offset a page accepts: the one that
// puts the last entry on screen. It walks the entries backwards from the end until
// the budget is spent, so a page of two-line entries is scrolled by the rows it
// really draws instead of by a count it does not.
func referenceMaxScroll(table referenceTable, width, budget int) int {
	if !table.TwoLine {
		return offsetWindowMax(budget, len(table.Rows))
	}
	used := 0
	first := len(table.Rows)
	for first > 0 {
		cost := referenceEntryCost(table, table.Rows[first-1], width)
		if used+cost > budget {
			break
		}
		used += cost
		first--
	}
	return first
}

// frameVital appends the build label to the screen's own vital sign. The build
// is the one fact every page shares, and it used to be on the main menu alone: a
// report asks for it first, and a screenshot of any screen should answer it. A
// screen that already names it keeps its own string, so the main menu does not
// read it twice.
func frameVital(vital string) string {
	label := VersionLabel()
	if vital == "" {
		return label
	}
	if strings.Contains(vital, label) {
		return vital
	}
	return vital + panelTabSeparator + label
}

// renderMenu renders the menus of the keymaps and learn sections. They differ
// only in their description, so the title, the card, the panel and the legend are
// shared: six copies of the same menu used to spell the legend three different
// ways, and the hubs had no card or panel at all. The list is windowed around the
// cursor, so a section with more categories than the frame has rows scrolls
// instead of overflowing.
func (m Model) renderMenu(description string) string {
	panels := m.panelsFor()
	hints := m.panelHints(panels, []installerHint{hintUp, hintDown, hintOpen, hintBack})
	body, start, end, total := m.menuCardBody(m.GetScreenTitle(), description, hints)

	// The scroll range is the menu's vital sign in the header. A menu whose list
	// fits has nothing to report and gets no filler.
	vital := ""
	if end-start < total {
		vital = scrollVital("Showing", start+1, end, total)
	}
	return m.frame(m.headerName(), vital, body, hints)
}

// menuCardBody is the body every menu draws, and it is one body so the Learn hub,
// its tool pages, the keymap menus and the LazyVim guide cannot be four shapes:
// the page's own name as the card's chip, its own description as the caption, the
// rule under the pair and the options in the card, windowed around the cursor.
//
// It draws through the reference grammar (referenceCard and cardRowLines,
// reference_cards.go and components.go) -- the same card and the same rows every
// other screen uses -- so the sentence a menu says about itself and the shape of
// its rows are the shared vocabulary and not a second one.
//
// The card's chrome is three rows -- its top border, the rule under the chip and
// its bottom border -- where the old shape spent three on the title, the
// description and the blank under them, so the page keeps the rows it always had and
// its scroll keys keep their distance. A caption the border row has no room for is
// moved under the rule by referenceCard, and its rows come off the window first: a
// narrow frame draws fewer rows rather than one row past its budget.
func (m Model) menuCardBody(title, description string, hints []installerHint) (body []string, start, end, total int) {
	l := layoutFor(m)
	width := l.Composition
	if l.TwoColumn && len(m.panelsFor()) > 0 {
		width = l.Left
	}
	content := cardContentWidth(width)
	rowsBudget := installerBodyRows(m.Height, m.footerReserveRows(contentWidth(m), hints))
	options := m.GetCurrentOptions()

	// The Learn hub's first section names the card itself: a group title only draws
	// on the divider before its group, and the first group has no divider above it.
	// Naming it on the top border is what makes "What the run chooses" label the
	// rows under it instead of the section after them. The chip asks for that name
	// only while the whole list fits under it, so a window that starts past the
	// section is never labelled with it -- and the decision reads the list, not the
	// cursor, so the card cannot change shape as the cursor walks.
	cardTitle := title
	if m.Screen == ScreenLearnTools {
		head := referenceCardHead(learnSectionChosenTitle, description, width)
		if len(options) <= max(1, rowsBudget-cardChromeRows-len(head)) {
			cardTitle = learnSectionChosenTitle
		}
	}

	head := referenceCardHead(cardTitle, description, width)
	room := max(1, rowsBudget-cardChromeRows-len(head))
	start, end = listWindow(m.Cursor, room, len(options))

	body = referenceCard(cardTitle, description, width, head, m.menuCardGroups(options, start, end, content))
	return body, start, end, len(options)
}

// menuCardGroups renders the visible option rows as the groups the card draws:
// a separator becomes the divider between two groups, never a row of its own, and
// a group the window ends on with no rows under it is dropped so the divider it
// would draw cannot spend a row the window did not count.
//
// The Learn hub is the one menu whose sections are named, because its two sections
// answer different questions -- what the run chooses and what it then installs --
// and a bare rule between them did not say which was which.
func (m Model) menuCardGroups(options []string, start, end, content int) []cardGroup {
	groups := []cardGroup{{}}
	for i := start; i < end; i++ {
		if strings.HasPrefix(options[i], menuSeparatorPrefix) {
			groups = append(groups, cardGroup{})
			continue
		}
		at := len(groups) - 1
		// A row opens the page it names; the way back acts where it stands, so it
		// keeps its own arrow in the gutter and carries no chevron.
		groups[at].Lines = append(groups[at].Lines, cardRowLines(cardRow{
			Title:   options[i],
			Chevron: !strings.HasPrefix(options[i], "←"),
		}, content, i == m.Cursor)...)
	}
	for len(groups) > 1 && len(groups[len(groups)-1].Lines) == 0 {
		groups = groups[:len(groups)-1]
	}
	if m.Screen == ScreenLearnTools && start == 0 && len(groups) > 1 {
		groups[1].Title = learnSectionInstalledTitle
	}
	return groups
}

func (m Model) renderKeymapsMenu() string {
	return m.renderMenu("Select a category to view keybindings")
}

func (m Model) renderKeymapCategory() string {
	if m.SelectedCategory >= len(m.KeymapCategories) {
		return m.deadEndScreen("Category not found", "Press [Esc] to pick another category.")
	}
	return m.renderKeymapTable(m.KeymapCategories[m.SelectedCategory], m.KeymapScroll)
}

// renderToolKeymapsMenu renders the tool selection menu (Neovim, Tmux, Zellij, Herdr, Ghostty)
func (m Model) renderToolKeymapsMenu() string {
	return m.renderMenu("Select a tool to view its keybindings")
}

// renderTmuxKeymapsMenu renders the Tmux keymap categories menu
func (m Model) renderTmuxKeymapsMenu() string {
	return m.renderMenu("Select a category to view Tmux keybindings")
}

// renderTmuxKeymapCategory renders a specific Tmux keymap category
func (m Model) renderTmuxKeymapCategory() string {
	if m.TmuxSelectedCategory >= len(m.TmuxKeymapCategories) {
		return m.deadEndScreen("Category not found", "Press [Esc] to pick another category.")
	}
	return m.renderKeymapTable(m.TmuxKeymapCategories[m.TmuxSelectedCategory], m.TmuxKeymapScroll)
}

// renderZellijKeymapsMenu renders the Zellij keymap categories menu
func (m Model) renderZellijKeymapsMenu() string {
	return m.renderMenu("Select a category to view Zellij keybindings")
}

// renderZellijKeymapCategory renders a specific Zellij keymap category
func (m Model) renderZellijKeymapCategory() string {
	if m.ZellijSelectedCategory >= len(m.ZellijKeymapCategories) {
		return m.deadEndScreen("Category not found", "Press [Esc] to pick another category.")
	}
	return m.renderKeymapTable(m.ZellijKeymapCategories[m.ZellijSelectedCategory], m.ZellijKeymapScroll)
}

// renderGhosttyKeymapsMenu renders the Ghostty keymap categories menu
func (m Model) renderGhosttyKeymapsMenu() string {
	return m.renderMenu("Select a category to view Ghostty keybindings")
}

// renderGhosttyKeymapCategory renders a specific Ghostty keymap category
func (m Model) renderGhosttyKeymapCategory() string {
	if m.GhosttySelectedCategory >= len(m.GhosttyKeymapCategories) {
		return m.deadEndScreen("Category not found", "Press [Esc] to pick another category.")
	}
	return m.renderKeymapTable(m.GhosttyKeymapCategories[m.GhosttySelectedCategory], m.GhosttyKeymapScroll)
}

// renderHerdrKeymapsMenu renders the Herdr keymap categories menu
func (m Model) renderHerdrKeymapsMenu() string {
	return m.renderMenu("Herdr is mouse-first; keyboard is optional. Select a category")
}

// renderHerdrKeymapCategory renders a specific Herdr keymap category
func (m Model) renderHerdrKeymapCategory() string {
	if m.HerdrSelectedCategory >= len(m.HerdrKeymapCategories) {
		return m.deadEndScreen("Category not found", "Press [Esc] to pick another category.")
	}
	return m.renderKeymapTable(m.HerdrKeymapCategories[m.HerdrSelectedCategory], m.HerdrKeymapScroll)
}

// renderKittyKeymapsMenu renders the Kitty keymap categories menu
func (m Model) renderKittyKeymapsMenu() string {
	return m.renderMenu("Select a category to view Kitty keybindings")
}

// renderKittyKeymapCategory renders a specific Kitty keymap category
func (m Model) renderKittyKeymapCategory() string {
	if m.KittySelectedCategory >= len(m.KittyKeymapCategories) {
		return m.deadEndScreen("Category not found", "Press [Esc] to pick another category.")
	}
	return m.renderKeymapTable(m.KittyKeymapCategories[m.KittySelectedCategory], m.KittyKeymapScroll)
}

// renderWezTermReference and renderAlacrittyReference answer the reference's
// question for the two terminals whose configurations declare no bindings: they
// draw the settings those files do carry, under a header that says so rather than
// under one that promises key bindings.
func (m Model) renderWezTermReference() string {
	return m.renderReferenceTable(weztermConfigTable(), m.ReferenceTableScroll)
}

func (m Model) renderAlacrittyReference() string {
	return m.renderReferenceTable(alacrittyConfigTable(), m.ReferenceTableScroll)
}

func (m Model) renderLazyVimMenu() string {
	return m.renderMenu("Learn how to use and customize LazyVim")
}

// lazyVimTopicLines is the topic's content exactly as the screen draws it: the
// body, the code example and the tips, each line cut to the card's content. The
// view draws these lines and the scroll keys measure them, so the two cannot
// disagree about how far the topic scrolls. It was the scroll keys' own, different
// count that let the closing lines sit behind a bound the renderer never reached.
func (m Model) lazyVimTopicLines(topic LazyVimTopic) []string {
	width := cardContentWidth(layoutFor(m).Composition)

	var allLines []string
	for _, line := range topic.Content {
		allLines = append(allLines, truncate(line, width))
	}
	allLines = append(allLines, "") // Empty line

	if topic.CodeExample != "" {
		allLines = append(allLines, "Example:", "")
		for _, line := range strings.Split(topic.CodeExample, "\n") {
			allLines = append(allLines, truncate(line, width))
		}
		allLines = append(allLines, "") // Empty line
	}

	if len(topic.Tips) > 0 {
		allLines = append(allLines, "Tips:")
		for _, tip := range topic.Tips {
			allLines = append(allLines, truncate("  • "+tip, width))
		}
	}

	return allLines
}

// The card the topic is drawn in spends the three rows the old body spent on its
// title, its description and the blank under them -- its top border, the rule
// under the chip and its bottom border -- which is why the window arithmetic did
// not change when the topic took the card. lazyVimTopicRows is the content rows
// the topic shows (reference_cards.go): the frame minus that chrome and the
// caption when it moved inside the card. The view and the scroll keys both ask for
// it, so the window the keys scroll is exactly the window the screen draws -- which
// is why it has to reserve the footer the frame will actually draw. Because the
// scroll handler shares this function and has no width to compute the exact packing
// with, it reserves the footer's worst case, footerMaxRows; the frame caps the
// footer at that many rows, so the body can never exceed the frame.

// renderLazyVimTopic renders one topic in the reference grammar: the topic's own
// title as the card's chip, its description as the caption and the window of its
// lines under the rule. Its scroll value IS the top-line offset (0 shows the first
// line, offsetWindowMax shows the last), so it windows through offsetWindow and its
// key handler clamps the same value with offsetWindowMax; the topic's closing
// lines are reachable.
func (m Model) renderLazyVimTopic() string {
	if m.SelectedLazyVimTopic >= len(m.LazyVimTopics) {
		return m.deadEndScreen("Topic not found", "Press [Esc] to return to the guide.")
	}

	topic := m.LazyVimTopics[m.SelectedLazyVimTopic]
	// The topic is prose and belongs on the same centred composition as every
	// other page, not against the frame's left edge with 227-column lines.
	width, _ := m.referenceCardWidth()

	allLines := m.lazyVimTopicLines(topic)
	viewHeight := m.lazyVimTopicRows(topic)
	start, end := offsetWindow(m.LazyVimScroll, viewHeight, len(allLines))

	lines := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		line := allLines[i]
		// Style code lines differently
		switch {
		case strings.HasPrefix(line, "--"), strings.HasPrefix(line, "local"),
			strings.HasPrefix(line, "return"), strings.HasPrefix(line, "{"),
			strings.HasPrefix(line, "}"), strings.HasPrefix(line, "  "),
			strings.HasPrefix(line, "map("), strings.HasPrefix(line, "vim."),
			strings.HasPrefix(line, "require"):
			lines = append(lines, CodeStyle.Render(line))
		case strings.HasPrefix(line, "Example:"), strings.HasPrefix(line, "Tips:"):
			lines = append(lines, chip(line))
		case strings.HasPrefix(line, "  •"):
			lines = append(lines, InkStyle.Render(line))
		case strings.HasPrefix(line, "•"):
			lines = append(lines, MutedStyle.Render(line))
		default:
			// Prose is body text, not a status colour: the old InfoStyle made the
			// guide dimmer than every other page on the same palette.
			lines = append(lines, InkStyle.Render(line))
		}
	}

	// The card is the page's frame now, so the body is not indented here: the
	// frame centres the composition, exactly as it does for every other card.
	body := lazyVimTopicCard(topic, lines, width)

	vital := ""
	if len(allLines) > viewHeight {
		vital = scrollVital("Lines", start+1, end, len(allLines))
	}
	// The topic is prose, not a menu: Enter opens nothing here, so the footer
	// names only the scroll keys and the way back.
	return m.frame(m.headerName(), vital, body, []installerHint{hintUp, hintDown, hintPage, hintBack})
}

// Installing screen rows. The mockup's shape is a stack: the run's title, the step
// it is on with the run's own clock, the checklist of the plan beside the log
// card, and the progress bar under both. The checklist is windowed around the
// step in progress, so a run with twenty steps cannot push the bar off a 24-row
// terminal, and the band's height comes from the frame rather than from the log's
// length, so the bar cannot move while the run writes.
const (
	// installingRunningTitle is the mockup's line over the whole screen. It is the
	// screen's one sentence: the header chip names the screen and this says what it
	// is doing.
	installingRunningTitle = "Running installation..."

	// installingHeadRows is what the body spends above its two columns: the title,
	// the step line and the run's clock. The count is fixed so the columns below it
	// never move between two renders of the same frame.
	installingHeadRows = 3

	// installingBarRows is the progress bar's row, the last thing the body draws.
	installingBarRows = 1

	// installingColumnGap is the blank columns between the checklist and the log
	// card, so the two read as two objects rather than as one run of text.
	installingColumnGap = 2

	// installingLogShare is the log card's share of the body's columns, before the
	// floors below clamp it. The mockup draws the two columns at roughly the same
	// width; the log's rows are the longer of the two, so it takes the larger share
	// of what the checklist's names leave.
	installingLogShare = 40

	// installingLogMinWidth and installingRailMinWidth are the narrowest either
	// column may be. A log column too narrow to read a line in is worse than a
	// checklist with the whole frame, so the card is dropped whole instead.
	installingLogMinWidth  = 24
	installingRailMinWidth = 28

	// installingBandMaxRows caps the two columns' height on a tall frame. The height
	// is the frame's, never the log's, so the bar under the band cannot move as the
	// run writes; the cap keeps a 50-row terminal from drawing a sixteen-row
	// checklist over twenty blank rows, and gives the rows it saves to the creature
	// the frame places below the body.
	installingBandMaxRows = 22

	// installingMinBandRows is the shortest band still worth a log column: its two
	// borders and one line of log.
	installingMinBandRows = 3

	// installingRailMeterCells is the trailing meter's width, and the narrow rungs
	// are what a column too small for it keeps: five cells where the meter still fits
	// beside the longest step name, and none at all below that. The meter is the
	// step's own progress, so it may narrow -- but a name is a fact and may not be
	// cut to make room for a decoration, which is why the widest name decides which
	// rung a column takes.
	installingRailMeterCells       = 10
	installingRailMeterCellsNarrow = 5
	installingRailWideWidth        = 2 + installingRailLongestName + installingColumnGap + installingRailMeterCells
	installingRailNarrowWidth      = 2 + installingRailLongestName + installingColumnGap + installingRailMeterCellsNarrow

	// installingRailLongestName is the longest step name the plan builder can
	// produce ("Restore the Applied Theme"), so the rungs above are measured against
	// the worst the checklist can hold rather than against the golden host's plan.
	installingRailLongestName = 25

	// installingLogPrompt leads every log line, as the mockup draws it: the mark
	// says the row is output rather than a step, and the glyph is plain ASCII so a
	// terminal with no Nerd Font still shows it.
	installingLogPrompt = "> "

	// installingStepSeparator is the mockup's ` - ` between the step's counter and
	// its name, so the two read as one sentence rather than as two facts sharing a
	// row.
	installingStepSeparator = " - "
)

// installProgress is the fraction of the run the bar fills: completed and
// skipped steps count fully, the running step counts the fraction it has
// reported through InstallStep.Progress, and pending steps count zero. The
// installer reports no sub-step progress today, so the bar advances per step;
// a step that starts reporting moves it within the step too, which is why
// InstallStep.Progress is read here rather than replaced by a step count.
func (m Model) installProgress() float64 {
	if len(m.Steps) == 0 {
		return 1
	}
	done := 0.0
	for _, step := range m.Steps {
		switch step.Status {
		case StatusDone, StatusSkipped:
			done++
		case StatusRunning:
			done += step.Progress
		}
	}
	if done > float64(len(m.Steps)) {
		done = float64(len(m.Steps))
	}
	return done / float64(len(m.Steps))
}

// renderProgressBar draws a bar of width cells filled to progress. The cells are
// text glyphs (█, ▒ and ░) rather than a background colour, so the bar still
// reads on a 16-colour terminal and in a terminal with no colour at all -- the
// same reason the step rail's glyphs carry its state. A highlight of -1 draws no
// traveller; a non-negative one draws that filled cell as the lighter ▒, so the
// highlight is a glyph difference and not a colour difference and a colourless
// terminal still sees it move.
func renderProgressBar(width int, progress float64, highlight int) string {
	if width < 1 {
		width = 1
	}
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	filled := int(math.Round(progress * float64(width)))
	if filled > width {
		filled = width
	}

	var b strings.Builder
	for i := 0; i < width; i++ {
		switch {
		case i < filled && i == highlight:
			b.WriteString(ProgressBarFilled.Render("▒"))
		case i < filled:
			b.WriteString(ProgressBarFilled.Render("█"))
		default:
			b.WriteString(ProgressBarEmpty.Render("░"))
		}
	}
	return b.String()
}

// stepGlyph names a step's state with a glyph and a colour. The glyph is what
// carries the state on a terminal with no colour: ✓ done, ● running, ○ pending,
// ✗ failed, ⊘ skipped. The running step is drawn in Accent because it is the one
// step to act on now, not a status the run is in; the other states keep the
// status colours, which are used for state and nothing else.
func stepGlyph(step InstallStep) (string, lipgloss.Style) {
	switch step.Status {
	case StatusRunning:
		return "●", AccentKeyStyle
	case StatusDone:
		return "✓", SuccessStyle
	case StatusFailed:
		return "✗", ErrorStyle
	case StatusSkipped:
		return "⊘", MutedStyle
	default:
		return "○", MutedStyle
	}
}

// installingBand is the two columns' geometry: how wide the checklist is, and how
// wide and how tall the log card beside it is. It is measured from the frame and
// never from the log's length, so the bar under the band cannot move while the run
// writes.
type installingBand struct {
	// RailWidth is the checklist's columns. The checklist is drawn bare -- the
	// mockup draws no box around it -- so its width is the whole column.
	RailWidth int
	// LogWidth is the log card's columns, its two borders included.
	LogWidth int
	// LogRows is the rows inside the card's border: the height of the band less the
	// two rows the border spends.
	LogRows int
	// ShowLog says whether the band has a second column at all.
	ShowLog bool
}

// installingBandFor measures the band: the checklist takes the whole composition
// when the log has no column, and otherwise the two split it, the log by share and
// the checklist by what is left. A log column narrower than its floor is dropped
// whole, because a card too narrow to read a line in is worse than a checklist
// with the room -- the same trade the frame makes everywhere it can hold no
// second column.
func (m Model) installingBandFor(composition, rows int) installingBand {
	band := installingBand{RailWidth: composition}
	if !m.ShowDetails || rows < installingMinBandRows {
		return band
	}
	logWidth := composition * installingLogShare / 100
	if logWidth < installingLogMinWidth {
		logWidth = installingLogMinWidth
	}
	railWidth := composition - installingColumnGap - logWidth
	if railWidth < installingRailMinWidth {
		return band
	}
	band.RailWidth = railWidth
	band.LogWidth = logWidth
	band.LogRows = rows - panelFrame
	band.ShowLog = true
	return band
}

// logTailCount is how many of a log's freshest lines fit in a box of rows, and
// how many earlier lines are left out. One of the rows is reserved for the dim
// note that names the ones left out, so a log longer than its box degrades out
// loud rather than dropping lines silently. It is the log's counterpart to
// listRows.
func logTailCount(total, rows int) (shown, hidden int) {
	if rows < 2 || total < 1 {
		return 0, total
	}
	if total <= rows {
		return total, 0
	}
	return rows - 1, total - (rows - 1)
}

// logTail renders at most rows of a log's tail: the freshest lines are kept and
// one dim row says how many earlier lines the screen could not show.
func logTail(lines []string, rows, width int, style lipgloss.Style) []string {
	shown, hidden := logTailCount(len(lines), rows)
	if shown <= 0 && hidden <= 0 {
		return nil
	}
	out := make([]string, 0, shown+1)
	for _, line := range lines[len(lines)-shown:] {
		out = append(out, style.Render(truncate(line, width)))
	}
	if hidden > 0 {
		out = append(out, MutedStyle.Render(fmt.Sprintf("… %d earlier lines", hidden)))
	}
	return out
}

// installElapsed is how long the run has taken, read from the model's own start
// timestamp and latest tick. It reports false when there is no start time or no
// tick has advanced the clock yet, which is what makes the screen say it is
// still estimating rather than show a number it cannot justify.
func (m Model) installElapsed() (time.Duration, bool) {
	if m.InstallStartedAt.IsZero() || m.Now.IsZero() || m.Now.Before(m.InstallStartedAt) {
		return 0, false
	}
	return m.Now.Sub(m.InstallStartedAt), true
}

// installETA estimates what is left from this run's own elapsed time and
// progress: the time already spent, scaled by the fraction still to do. It is
// deliberately unable to guess: with nothing complete (progress 0) there is no
// rate to scale, so it reports false, and it never reads a per-step constant or
// anything about the machine.
func installETA(progress float64, elapsed time.Duration) (time.Duration, bool) {
	if progress <= 0 || progress >= 1 || elapsed <= 0 {
		return 0, false
	}
	return time.Duration(float64(elapsed) * (1 - progress) / progress), true
}

// humanDuration states a duration the way the installing screen does: whole
// seconds under a minute, minutes and seconds under an hour, hours and minutes
// above it. No fractions, because a run's clock does not need them.
func humanDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm %02ds", int(d.Minutes()), int(d.Seconds())%60)
	default:
		return fmt.Sprintf("%dh %02dm", int(d.Hours()), int(d.Minutes())%60)
	}
}

// installingStepText names the step the run is on and where it is in the plan.
// The step counter and the step's name already exist on the model; showing them
// invents nothing.
func (m Model) installingStepText() string {
	total := len(m.Steps)
	if total == 0 {
		return ""
	}
	current, _, _ := m.installingPosition()
	if current < 1 {
		current = 1
	}
	if current > total {
		current = total
	}
	text := fmt.Sprintf("Step %d of %d", current, total)
	if name := m.Steps[current-1].Name; name != "" {
		// The mockup spells the step's name after a dash, so the counter and the
		// name read as one sentence rather than as two facts sharing a row.
		text += installingStepSeparator + name
	}
	return text
}

// installingPosition derives the step counter from the same progress the bar
// draws, so the sentence and the percentage cannot disagree: the current step is
// the first one the progress has not completed, and a run whose every step is
// done reports the last step at 100% rather than a counter the bar contradicts.
func (m Model) installingPosition() (current, total int, percent float64) {
	total = len(m.Steps)
	if total == 0 {
		return 0, 0, 1
	}
	percent = m.installProgress()
	done := int(percent * float64(total))
	if done >= total {
		current = total
	} else {
		current = done + 1
	}
	return current, total, percent
}

// installingTimeText is the run's clock: how long it has taken and how much is
// left. Everything it states is derived from the model, and a run with no basis
// for an estimate says so instead of showing a number.
func (m Model) installingTimeText() string {
	elapsed, ok := m.installElapsed()
	if !ok {
		return "Estimating the time remaining…"
	}
	text := "Elapsed " + humanDuration(elapsed)
	if eta, ok := installETA(m.installProgress(), elapsed); ok {
		text += panelTabSeparator + "~" + humanDuration(eta) + " left"
	} else {
		text += panelTabSeparator + "estimating…"
	}
	return text
}

// installingHeadRows is the mockup's stack above the two columns: what the run is
// doing, the step it is on, and the run's own clock. Every row is centred in the
// composition, as the mockup centres them, and every row is cut to a whole word
// boundary of plain text before it is styled, so a long step name cannot widen the
// frame. The count is fixed -- a run with no plan yet keeps the step row blank -- so
// the columns below never move. Both texts are pure reads of the model, which is why
// a snapshot is stable and why the estimate cannot change when nothing else did.
func (m Model) installingHeadRows(composition int) []string {
	rows := []string{
		centredLine(BrandStyle.Render(truncate(installingRunningTitle, composition)), composition),
	}
	if step := m.installingStepText(); step != "" {
		rows = append(rows, centredLine(InkStyle.Render(truncate(step, composition)), composition))
	} else {
		rows = append(rows, "")
	}
	return append(rows, centredLine(MutedStyle.Render(truncate(m.installingTimeText(), composition)), composition))
}

// centredLine indents one already-styled line so it is centred in the composition.
// The caller cuts the text to the width first: a cut taken after styling would land
// inside an escape sequence, and a cut taken on the plain text is the whole reason
// this helper pads and does not measure the line against the frame.
func centredLine(line string, composition int) string {
	if line == "" || composition < 1 {
		return line
	}
	if pad := composition - lipgloss.Width(line); pad > 1 {
		return strings.Repeat(" ", pad/2) + line
	}
	return line
}

// installingColumns draws the mockup's two columns side by side: the checklist of
// the plan at the left and the log card at the right, both measured against the
// same rows. Every row is padded to the composition, so the card's right border
// lands on the frame's edge instead of wherever its content happened to stop.
// Where the frame has no column for the log, the checklist takes the whole width --
// the shape yields, the steps never do.
func (m Model) installingColumns(composition, rows int) []string {
	band := m.installingBandFor(composition, rows)
	rail := m.installingRailRows(band.RailWidth, rows)
	if !band.ShowLog {
		// The band is the frame's height with or without a second column, so the bar
		// under it keeps its row when the log is collapsed.
		out := append(make([]string, 0, rows), rail...)
		for len(out) < rows {
			out = append(out, "")
		}
		return out
	}

	// The card's content is padded to its height, so the bottom border is drawn
	// even while the run has fewer lines to show than the frame has rows for them.
	lines := m.installingLogRows(band.LogWidth-panelFrame-panelPad*2, band.LogRows)
	for len(lines) < band.LogRows {
		lines = append(lines, "")
	}
	card := panelBlock("", band.LogWidth, lines)

	out := make([]string, 0, rows)
	for i := 0; i < rows; i++ {
		left, right := "", ""
		if i < len(rail) {
			left = rail[i]
		}
		if i < len(card) {
			right = card[i]
		}
		out = append(out, padRight(left, band.RailWidth)+strings.Repeat(" ", installingColumnGap)+padRight(right, band.LogWidth))
	}
	return out
}

// installingRailRows draws the checklist: one row per step, windowed around the
// step the run is on so a long plan keeps the step in progress on screen. The
// window follows the running step rather than a cursor -- the reader is not moving
// anything here -- and the header's vital says which step of how many, so a window
// that hides a row still says how many rows there are.
func (m Model) installingRailRows(width, rows int) []string {
	if width < 1 || rows < 1 || len(m.Steps) == 0 {
		return nil
	}
	cells := installingRailCells(width)
	running := 0
	lines := make([]string, len(m.Steps))
	for i, step := range m.Steps {
		if i == m.CurrentStep {
			running = i
		}
		lines[i] = installingRailRow(step, width, cells, i == m.CurrentStep)
	}

	// The window is measured in lines that include the two notes naming what it
	// hid, so a short frame says how many steps it could not show instead of
	// appearing to hold a plan that ended early. The running step always stays.
	visible := max(1, rows)
	start, end := listWindow(running, visible, len(lines))
	trim := func() {
		notes := 0
		if start > 0 {
			notes++
		}
		if end < len(lines) {
			notes++
		}
		if len(lines[start:end])+notes > visible {
			end -= len(lines[start:end]) + notes - visible
			if end <= start {
				end = start + 1
			}
		}
	}
	trim()

	out := make([]string, 0, visible)
	if start > 0 {
		out = append(out, MutedStyle.Render(fmt.Sprintf("↑ %d more above", start)))
	}
	out = append(out, lines[start:end]...)
	if end < len(lines) {
		out = append(out, MutedStyle.Render(fmt.Sprintf("↓ %d more below", len(lines)-end)))
	}
	if len(out) > visible {
		// A frame too tight for both notes and a row keeps the rows: the note is a
		// decoration, the step in progress is the screen.
		s, e := listWindow(running, visible, len(lines))
		out = lines[s:e]
	}
	return out
}

// installingRailCells is the meter's width for a checklist column: the full meter
// where the longest step name still fits beside it, five cells where only that fits,
// and none at all below it. A column with no room for both keeps the name: a cut
// name is a lost fact, and the run's own bar still carries the progress.
func installingRailCells(width int) int {
	switch {
	case width >= installingRailWideWidth:
		return installingRailMeterCells
	case width >= installingRailNarrowWidth:
		return installingRailMeterCellsNarrow
	default:
		return 0
	}
}

// installingRailRow draws one step of the checklist: its state glyph, its name and
// the meter of the progress it reported, at the right edge. The step the run is on
// is one filled bar across the column, as the mockup draws it, with its glyph riding
// the bar; the other rows are plain. Every state is said by the glyph as well as by
// the tone, so a terminal with no colour still reads which step is done, running,
// skipped or failed. Nothing else is added to the row: the installer keeps no
// per-step duration, so no row shows one -- the meter is the fraction the step's own
// status and Progress really hold.
func installingRailRow(step InstallStep, width, cells int, running bool) string {
	glyph, style := stepGlyph(step)
	// The glyph and the blank after it, then the gap and the meter where the column
	// has room for one.
	room := width - 2
	tail := ""
	if cells > 0 {
		room -= installingColumnGap + cells
	}
	if room < 1 {
		room = 1
	}
	name := padRight(truncate(step.Name, room), room)
	fraction := stepFraction(step)
	if cells > 0 {
		tail = strings.Repeat(" ", installingColumnGap) + meterCellsPlain(fraction, cells)
	}
	if running {
		return RowBarMarkerStyle.Render(glyph) + RowBarStyle.Render(" "+name) + MeterOnBarStyle.Render(tail)
	}
	if cells == 0 {
		return style.Render(glyph) + InkStyle.Render(" "+name)
	}
	return style.Render(glyph) + InkStyle.Render(" "+name) + strings.Repeat(" ", installingColumnGap) + meterBar(fraction, cells)
}

// installingLogRows draws the log card's content: the freshest lines the card's
// rows can hold, each led by the mockup's `> ` prompt, and one dim row naming the
// earlier lines the card could not show rather than dropping them in silence. A run
// that has logged nothing yet says so instead of drawing an empty box, so the card
// the frame reserved is never mistaken for a card that failed to draw.
func (m Model) installingLogRows(content, rows int) []string {
	if content <= lipgloss.Width(installingLogPrompt) || rows < 1 {
		return nil
	}
	if len(m.LogLines) == 0 {
		return []string{MutedStyle.Render(truncate("Waiting for the first line…", content))}
	}
	shown, hidden := logTailCount(len(m.LogLines), rows)
	out := make([]string, 0, shown+1)
	for _, line := range m.LogLines[len(m.LogLines)-shown:] {
		out = append(out, MutedStyle.Render(installingLogPrompt)+
			InfoStyle.Render(truncate(line, content-lipgloss.Width(installingLogPrompt))))
	}
	if hidden > 0 {
		out = append(out, MutedStyle.Render(truncate(fmt.Sprintf("… %d earlier lines", hidden), content)))
	}
	return out
}

// installingProgressRow is the run's bar: the whole run's progress filled in brand
// chrome with its percentage at the frame's right edge, as the mockup draws it. The
// fraction is installProgress's, so the bar counts every step the run has finished,
// skipped or is part-way through, and nothing else.
func (m Model) installingProgressRow(composition int) string {
	// The percentage is ` %3.0f%%`: a space and four columns, so the bar ends where
	// the number begins and the number ends at the frame's edge.
	barWidth := composition - 5
	if barWidth < 10 {
		barWidth = 10
	}
	progress := m.installProgress()
	// The traveller is the lighter cell that walks the filled part of the bar during
	// a long step. It only draws while a run is actually in flight and the bar is not
	// already full, so a screen with no run behind it is byte-for-byte the screen it
	// was. Its position comes from the model's own frame tick, never from the
	// renderer's clock.
	highlight := -1
	if m.Animating && len(m.Steps) > 0 && progress < 1 {
		if filled := int(math.Round(progress * float64(barWidth))); filled > 0 {
			highlight = m.AnimTick % filled
		}
	}
	return renderProgressBar(barWidth, progress, highlight) + " " + BrandStyle.Render(fmt.Sprintf("%3.0f%%", progress*100))
}

// installingVital is the installing screen's header vital sign: which step of
// the run is in progress, as the same bar-and-count the wizard uses at the top
// of the flow.
func (m Model) installingVital() string {
	current, total, _ := m.installingPosition()
	if total == 0 {
		return ""
	}
	return meterRendered(current, total, 10)
}

// stepFraction is a step's own progress: done and skipped count fully, the
// running step counts what it has reported through InstallStep.Progress, and a
// step that has not started counts zero.
func stepFraction(step InstallStep) float64 {
	switch step.Status {
	case StatusDone, StatusSkipped:
		return 1
	case StatusRunning:
		return step.Progress
	default:
		return 0
	}
}

// renderInstalling draws the run in the mockup's shape: the sentence over the
// screen, the step it is on with the run's own clock, the checklist of the plan
// beside the log card, and the progress bar under both. Nothing on it is guessed:
// the checklist's glyphs and trailing meters are each step's own status and the
// fraction it reported, the bar is installProgress, and the clock is the model's
// start time against its latest tick. The installer keeps no per-step duration, so
// no row shows one -- the mockup's trailing time column is the meter a step really
// has, which is the same meter the header uses for the run.
func (m Model) renderInstalling() string {
	width := contentWidth(m)
	composition := layoutFor(m).Composition
	hints := []installerHint{hintDetails, hintStopRun}
	// The offer's own hint leads while the run is paused on it: it is the action
	// the step is waiting for, and the other two stay so the keys already on the
	// screen do not vanish with the question.
	if m.WSLTerminalOffer != nil {
		hints = []installerHint{hintWSLTerminalOffer, hintDetails, hintStopRun}
	}
	bodyRows := installerBodyRows(m.Height, m.footerRowCount(width, hints))

	head := m.installingHeadRows(composition)
	// A frame too short for the stack and the bar has no room for a column or for the
	// machine's pulse. The stack is what the screen is for, so the rest yields rather
	// than the body drawing past the frame.
	if bodyRows <= len(head)+installingBarRows {
		if keep := max(1, bodyRows-installingBarRows); keep < len(head) {
			head = head[:keep]
		}
		body := append(append([]string(nil), head...), m.installingProgressRow(composition))
		return m.frame(m.headerName(), m.installingVital(), body, hints)
	}

	// The machine's pulse, and the run's own progress over time, sit directly over
	// the bar. The block appears only once a sample lands and only in the rows the
	// frame left it, so it can never push the two columns off the frame, and a model
	// the gate never sampled draws the screen exactly as it was.
	live := m.installingLiveRows(composition)
	bandRows := bodyRows - len(head) - installingBarRows - len(live)
	if bandRows < 1 {
		// The two columns are what the screen is for, so the pulse is what yields when
		// the frame cannot hold both.
		live, bandRows = nil, 1
	}
	if bandRows > installingBandMaxRows {
		bandRows = installingBandMaxRows
	}

	body := append([]string(nil), head...)
	if m.WSLTerminalOffer != nil {
		// The interactive WSL step's offer is a question the run is blocked on, so it
		// takes the rows the checklist and the log would use while it waits. It is
		// drawn whether or not the details panel is on: a question asked inside a box
		// the user had not opened is a question nobody read, and this is the screen
		// that stands between the run and a write to settings.json.
		body = append(body, m.wslTerminalOfferRows(composition, bandRows)...)
	} else {
		body = append(body, m.installingColumns(composition, bandRows)...)
	}
	body = append(body, live...)
	body = append(body, m.installingProgressRow(composition))

	return m.frame(m.headerName(), m.installingVital(), body, hints)
}

// wslTerminalOfferRows renders the pending Windows Terminal offer into the
// installing body. Every line is wrapped to the panel's inner width and the rows
// the body has left bound it, so a long settings path or a long font family is
// read whole rather than clipped, and the block can never grow past the frame. A
// frame too short for a panel falls back to plain wrapped rows so the file and
// the value at stake are still named instead of dropped in silence.
func (m Model) wslTerminalOfferRows(width, rows int) []string {
	if rows < 1 || len(m.WSLTerminalOfferLines) == 0 {
		return nil
	}

	if rows < panelFrame+1 {
		var out []string
		for _, line := range m.WSLTerminalOfferLines {
			if len(out) >= rows {
				break
			}
			out = append(out, wrapText(line, width, rows-len(out))...)
		}
		return out
	}

	content := width - panelFrame - panelPad*2
	inner := rows - panelFrame
	wrapped := make([]string, 0, inner)
	remaining := inner
	for _, line := range m.WSLTerminalOfferLines {
		if remaining <= 0 {
			break
		}
		chunk := wrapText(line, content, remaining)
		wrapped = append(wrapped, chunk...)
		remaining -= len(chunk)
	}
	return panelBlock(BrandStyle.Render("Windows Terminal"), width, wrapped)
}

func (m Model) renderComplete() string {
	l := layoutFor(m)
	hints := []installerHint{hintExit}

	// The screen draws in the reference grammar (completeCardBody, flow_cards.go):
	// the answers the run recorded as the frame's fact rows, under the summary's own
	// chip, and the next step under a divider named for it. A frame too narrow or too
	// short for the card keeps the compact body below, which is the same facts in the
	// same order.
	if card, ok := m.completeCardBody(l, hints); ok {
		return m.frame(m.headerName(), "", centeredBody(card, l), hints)
	}

	// The screen's name is the frame's header now, so the body opens on the first
	// block instead of repeating it as a second label row. The block carries the
	// run's verdict too: the check in the success tone before the label is the one
	// place the completion screen says "done" in the palette without spending a
	// row, and the companion and the celebration are both measured against the
	// rows this screen leaves.
	// The summary reads the labels the wizard showed, not the internal values:
	// "linux" beside "Zsh" was a different vocabulary from the questions that
	// produced it. The multiplexer is a multiplexer (tmux, Zellij, Herdr), not a
	// window manager, and a question left unanswered says so.
	//
	// The rows are the frame's fact rows (panelFact), the same pair the utilities
	// panel and the review draw: the label in the muted tone at the fixed measure
	// and the value in body ink beside it. The summary used to spell each row as
	// one sentence ("OS: macOS"), which is the one shape the rest of the program's
	// fact blocks do not use -- and the mockup's own panels line their labels up
	// in a column, so a reader compares machines instead of reading six sentences.
	//
	// The facts come from completionFacts and the command from
	// completionShellCommand, so the two shapes of this screen cannot state two
	// different records.
	width := contentWidth(m)
	var rendered []string
	for _, fact := range m.completionFacts() {
		rendered = append(rendered, panelFact(fact.Label, fact.Value, width-panelFrame)...)
	}

	// The two blocks are cards, like the main menu: the panel edge gives the
	// completion screen a body that reads as designed space instead of a gutter
	// under a chip, and the summary's title keeps the success check that says the
	// run finished while the edges wear the frame's border role.
	// A dry run changes nothing, so "Installation complete" and "exec zsh" are
	// true only in the loosest sense. Say what happened and do not send the reader
	// to a shell that was never configured.
	dry := dryRun()
	body := []string{}
	if dry {
		body = append(body, WarningStyle.Render(completeDryRunNotice), "")
	}
	body = append(body, panelBlock(SuccessStyle.Render("✓")+" "+BrandStyle.Render("Summary"), width, rendered)...)
	body = append(body, "")
	if dry {
		body = append(body, panelBlock(BrandStyle.Render(completeNothingChangedTitle), width, []string{
			InfoStyle.Render(completeNothingChangedNote),
		})...)
	} else {
		body = append(body, panelBlock(BrandStyle.Render(completeNextStepTitle), width, []string{
			InfoStyle.Render(completeNextStepIntro),
			HighlightStyle.Render(fmt.Sprintf("exec %s", m.completionShellCommand())),
		})...)
	}

	return m.frame(m.headerName(), "", body, hints)
}

// completionChoiceLabel is the label the wizard showed for a recorded answer, so
// the completion summary speaks the questions' vocabulary instead of the internal
// choice keys. An unanswered question says so.
func (m Model) completionChoiceLabel(screen Screen, value string) string {
	if value == "" {
		return "Not chosen"
	}
	for _, option := range m.questionValueOptions(screen) {
		if questionAnswerKey(screen, option) == value {
			return plainOptionName(option)
		}
	}
	return value
}

func (m Model) renderError() string {
	l := layoutFor(m)
	hints := []installerHint{hintRetry, hintQuit}

	// The screen draws in the reference grammar (errorCardBody, flow_cards.go): the
	// message, the way back under a divider named for it and the freshest log lines
	// under a divider of their own. A frame too narrow or too short for the card
	// keeps the compact body below.
	if card, ok := m.errorCardBody(l, hints); ok {
		return m.frame(m.headerName(), "", centeredBody(card, l), hints)
	}

	width := contentWidth(m)
	bodyRows := installerBodyRows(m.Height, m.footerRowCount(width, hints))

	// The screen's name is the frame's header now, so the body opens on the failure
	// block instead of repeating it as a second label row.
	body := []string{chip(errorCardTitle)}

	// The message is wrapped and capped so a long failure cannot run off the
	// bottom of the screen; the last row ends with the cut marker.
	errLines := make([]string, 0, 4)
	for _, row := range wrapText(m.ErrorMsg, width-2, errorMessageRows) {
		errLines = append(errLines, ErrorStyle.Render(row))
	}
	body = append(body, gutteredBlock(errLines)...)

	// The way back is named before the log: a failure is where a person needs to
	// know what the run already changed and how to put it back, and the log is
	// context for that rather than a substitute for it. The block is three rows so
	// it cannot be the reason the log disappears from a short frame.
	body = append(body, "", chip(errorWayBackTitle))
	for _, line := range wayBackAfterAFailure(m.BackupDir) {
		body = append(body, gutteredBlock(wrapText(line, width-2, errorWayBackRows))...)
	}

	// Show the freshest log lines for context, as many as the frame leaves rather
	// than a fixed five. One dim row names the earlier lines the panel could not
	// show, so a log longer than the panel degrades out loud instead of dropping
	// them silently. The blank above the panel and its chip are spent first, and a
	// frame too small for even one line and the note shows no panel at all rather
	// than an empty one.
	if len(m.LogLines) > 0 {
		logRows := bodyRows - len(body) - 2
		if logRows >= 2 {
			body = append(body, "", chip(errorLogsTitle))
			body = append(body, gutteredBlock(logTail(m.LogLines, logRows, width-2, InfoStyle))...)
		}
	}

	return m.frame(m.headerName(), "", body, hints)
}

// backupConfirmFixed is the rows the backup-confirmation body spends around the
// config list: the title, the description, a blank, a blank, the note, a blank
// and the card's two borders. The list gets what is left, so a machine with every
// config path present cannot push the options off the bottom of the screen.
const backupConfirmFixed = 8

func (m Model) renderBackupConfirm() string {
	l := layoutFor(m)
	hints := []installerHint{hintUp, hintDown, hintChoose, hintBack}

	// The screen draws in the reference grammar (backupConfirmCardBody,
	// flow_cards.go): the configurations the run would overwrite, the note that says
	// why the backup exists and the three answers, one card. A frame too narrow or
	// too short for it keeps the compact body below.
	if card, ok := m.backupConfirmCardBody(l, hints); ok {
		return m.frame(m.headerName(), "", centeredBody(card, l), hints)
	}

	width := l.Composition
	options := m.GetCurrentOptions()
	bodyRows := installerBodyRows(m.Height, m.footerRowCount(l.Inner, hints))

	// The config list is bounded to the frame: a machine with all sixteen config
	// paths present used to push the options off the bottom of the screen.
	configRows := bodyRows - backupConfirmFixed - len(options)
	if configRows < 1 {
		configRows = 1
	}

	body := []string{
		BrandStyle.Render(m.GetScreenTitle()),
		MutedStyle.Render(backupConfigCaption),
		"",
	}
	body = append(body, listRows(m.ExistingConfigs, "  "+backupConfigMark+" ", configRows, width, WarningStyle)...)
	body = append(body, "")
	body = append(body, InfoStyle.Render(backupConfigNote))
	body = append(body, "")
	// The answers are the screen's actions, so they are drawn in the frame's card
	// like every other menu's rows: the list above is the content and the card is
	// what can be pressed, which is the two-part body every screen in the program
	// now has. The rows, their order and the keys are unchanged.
	body = append(body, panelBlock("", width, m.menuRows(options, m.Cursor))...)

	return m.frame(m.headerName(), "", centeredBody(body, l), hints)
}

// restoreBackupBodyFixed is the part of the restore list's body that is not a
// backup row: the title, the description, a blank, the card's two borders, the
// divider and the Back row. Back is part of the reservation, not part of the list,
// so a long history windows itself instead of pushing the way out off the frame.
const restoreBackupBodyFixed = 7

func (m Model) renderRestoreBackup() string {
	l := layoutFor(m)
	hints := []installerHint{hintUp, hintDown, hintOpen, hintBack}

	// The screen draws in the reference grammar (restoreHistoryCardBody,
	// flow_cards.go): the history as the card's rows and the way back under the
	// card's own rule. A frame too narrow or too short for it keeps the compact body
	// below.
	if card, start, end, ok := m.restoreHistoryCardBody(l, hints); ok {
		// The scroll range is the list's vital sign in the header, so the body does
		// not hold a scroll row that would move the Back row as the history grows.
		vital := ""
		if end-start < len(m.AvailableBackups) {
			vital = scrollVital("Showing", start+1, end, len(m.AvailableBackups))
		}
		return m.frame(m.headerName(), vital, centeredBody(card, l), hints)
	}

	width := contentWidth(m)
	bodyRows := installerBodyRows(m.Height, m.footerRowCount(width, hints))

	// The list is bounded and follows the cursor, so a long history scrolls
	// instead of pushing the Back row off the frame.
	listBudget := bodyRows - restoreBackupBodyFixed
	if listBudget < 1 {
		listBudget = 1
	}

	body := []string{
		BrandStyle.Render(m.GetScreenTitle()),
		MutedStyle.Render(restoreHistoryCaption),
		"",
	}

	// The backups and the way back are one card, with the shared divider between
	// the list and the row that leaves it: the history is the content and Back is
	// the last row, which is how every menu in the program draws its way out.
	entries := mainMenuGroup{}
	start, end := 0, 0
	if len(m.AvailableBackups) == 0 {
		entries.Lines = append(entries.Lines, InkStyle.Render("No backups found."))
	} else {
		start, end = listWindow(m.Cursor, listBudget, len(m.AvailableBackups))
		for i := start; i < end; i++ {
			backup := m.AvailableBackups[i]
			label := fmt.Sprintf("%s (%d items)", backup.Timestamp.Format("2006-01-02 15:04:05"), len(backup.Files))
			entries.Lines = append(entries.Lines, m.rowBar(label, i == m.Cursor, ""))
		}
	}

	backIdx := len(m.AvailableBackups) + 1
	back := mainMenuGroup{Lines: []string{m.rowBar(restoreHistoryBack, m.Cursor == backIdx, "")}}
	body = append(body, panelBlockGrouped("", width, []mainMenuGroup{entries, back})...)

	// The scroll range is the list's vital sign in the header, so the body does
	// not hold a scroll row that would move the Back row as the history grows.
	vital := ""
	if len(m.AvailableBackups) > listBudget {
		vital = scrollVital("Showing", start+1, end, len(m.AvailableBackups))
	}
	return m.frame(m.headerName(), vital, body, hints)
}

// restoreConfirmBodyFixed is the part of the restore confirmation's body that is
// not a file row: the title, the description, a blank, the contents card's two
// borders, the blank, the warning, the blank and the answers card's two borders.
const restoreConfirmBodyFixed = 10

func (m Model) renderRestoreConfirm() string {
	if m.SelectedBackup >= len(m.AvailableBackups) {
		return m.deadEndScreen("No backup selected", "Press [Esc] to go back.")
	}

	backup := m.AvailableBackups[m.SelectedBackup]
	l := layoutFor(m)
	hints := []installerHint{hintUp, hintDown, hintChoose, hintCancel}

	// The screen draws in the reference grammar (restoreConfirmCardBody,
	// flow_cards.go): which backup it is as the caption, the files it carries under
	// the card's own Contents divider, the warning and the three answers. A frame
	// too narrow or too short for the card keeps the compact body below.
	if card, ok := m.restoreConfirmCardBody(backup, l, hints); ok {
		return m.frame(m.headerName(), "", centeredBody(card, l), hints)
	}

	width := l.Composition
	options := m.GetCurrentOptions()
	bodyRows := installerBodyRows(m.Height, m.footerRowCount(l.Inner, hints))

	// List files in backup, bounded to the frame so a backup with a long file
	// list cannot push the options off the bottom of the screen.
	fileRows := bodyRows - restoreConfirmBodyFixed - len(options)
	if fileRows < 1 {
		fileRows = 1
	}

	body := []string{
		BrandStyle.Render(m.GetScreenTitle()),
		MutedStyle.Render(restoreConfirmCaption + backup.Timestamp.Format("2006-01-02 15:04:05")),
		"",
	}
	// The contents are one card with its own title on the border, and the answers
	// are the card under it: the screen reads as what would change and then what
	// to do about it, and the card carries the block's edge instead of a chip and a
	// gutter spending three rows to say the same thing.
	body = append(body, panelBlock(BrandStyle.Render(restoreConfirmContentsTitle), width,
		listRows(backup.Files, restoreFileMark+" ", fileRows, width-panelFrame, InfoStyle))...)
	body = append(body, "")
	body = append(body, WarningStyle.Render(restoreConfirmWarning))
	body = append(body, "")
	body = append(body, panelBlock("", width, m.menuRows(options, m.Cursor))...)

	return m.frame(m.headerName(), "", centeredBody(body, l), hints)
}

// ============================================================================
// Trainer Views
// ============================================================================

// The trainer screens budget their rows one terminal row per element: every
// string they assemble is a single row, and the code window receives the rows
// that are left instead of the rows the author hoped for. That is the fix for
// the defect they shipped with -- renderTrainerExercise emitted about 22+N rows
// and renderTrainerBoss about 21+N, where N is the number of code lines, so
// from three code lines up the bottom of the screen fell outside the 24-row
// terminal the trainer claims to support. The corpus is worse than that floor:
// regex_001 has seven code lines, macros_017 has ten, and the regex boss steps
// have fourteen, which is 35 rows in a 24-row frame.
// TestTrainerScreensFitTheFrame renders every lesson and every boss step of
// every module at 80x24 and fails when a screen does not fit.
//
// Two rules keep the arithmetic true: a legend is packed by the shared footer
// component into the two rows trainerFrameRows reserves for it, and a wrapped
// block is capped to a fixed number of elements so it cannot consume rows the
// code window was promised.

const (
	// trainerGutterWidth is the columns the "%2d | " line-number prefix takes in
	// front of a code line.
	trainerGutterWidth = 5
	// trainerMissionMaxRows caps the mission text, which cannot be scrolled. The
	// cap is visible: the last row ends with the cut marker, so a mission that is
	// longer than the frame allows loses its tail out loud. The longest mission in
	// the corpus needs four rows at the 76 columns of an 80-column frame.
	trainerMissionMaxRows = 4
	// trainerMessageRows is the height of the feedback area, held even when there
	// is nothing to say: a code window that changes size when a hint arrives
	// mid-exercise moves every line under the player's eyes.
	trainerMessageRows = 2
	// trainerCodeMinRows is the floor the code window is never taken below.
	trainerCodeMinRows = 1
	// trainerFloorHeight is the smallest supported trainer terminal. At this
	// height the companion keeps its original one-row mini.
	trainerFloorHeight = 24
	// trainerModuleStatusWidth is the column width of the trainer menu's status
	// field. It fits the longest status ("● practice"), so a row's module name
	// starts on the same column whatever state the module is in.
	trainerModuleStatusWidth = 10
	// trainerRowMeterCells is how many cells each of a menu row's trailing meters
	// spends. Two labelled meters plus the status, the icon, the 23-column longest
	// module name and the row marker have to share 76 inner columns, which leaves
	// three cells each; the count after each bar carries the exact number.
	trainerRowMeterCells = 3
	// trainerHeaderMeterCells is the width of the position meter in an exercise
	// screen's header, sharing that row with the score, the streak and the
	// countdown.
	trainerHeaderMeterCells = 6
	// trainerFrameRows is what the exercise and boss screens spend on rows that
	// are neither the mission text nor the code window: the global top padding
	// View() adds, the header row, its rule, the mission chip, the code chip, the
	// answer chip and its guttered line, the reserved feedback area, the blank
	// above the legend and the two-line legend. The mission and the code window
	// share what is left, which is 24-12 = 12 rows at 80x24. The header carries what used to be three stacked rows (the mode
	// title, the progress or lives line and the countdown) and the code block's two
	// rules are gone -- the code rows are guttered now -- so the code window is
	// four rows taller than before and the mission text keeps its own cap. It is
	// checked rather than trusted: TestTrainerScreensFitTheFrame renders every
	// lesson and every boss step at 80x24, so a screen that spends a row this
	// constant does not count fails that test instead of quietly losing the bottom
	// of the screen.
	trainerFrameRows = 12
)

// The hints the trainer screens share, in the shared installerHint form so they
// pack and style through footerHints exactly like the installer's. Each one is a
// key and its verb, so the menu, the lesson, the practice and the boss legends
// cannot drift apart on the keys they promise: Ctrl-e types the token an insert
// answer needs to leave insert mode, Ctrl-v types the byte that opens a
// blockwise visual selection, Esc is the trainer's own exit key, Backspace is an
// input edit the engine never sees, and PgUp/PgDn scroll the code window. The
// scroll keys are key names rather than printable characters, so they cannot
// collide with typing, with the hint key, with submit or with back. "Type
// command" is the one instruction with no key of its own, so it carries an empty
// key token and renders as its verb alone.
var (
	trainerHintLesson   = installerHint{"[Enter/l]", "lesson"}
	trainerHintPractice = installerHint{"[p]", "practice"}
	trainerHintBoss     = installerHint{"[b]", "boss"}
	trainerHintReset    = installerHint{"[r]", "reset module"}
	trainerHintResetAll = installerHint{"[R]", "reset all"}
	trainerHintBack     = installerHint{"[q/Esc]", "back"}
	trainerHintContinue = installerHint{"[Enter]", "continue"}
	// trainerHintReturn is the boss result's way back. The trainer's keys are its
	// lesson (docs/vim-trainer-spec.md), so its return keeps the spelling the rest
	// of the trainer uses and is not moved onto the installer's new grammar.
	trainerHintReturn = installerHint{"[Enter/Space/Esc/q]", "return to menu"}

	trainerHintTypeCommand = installerHint{"", "Type command"}
	trainerHintSubmit      = installerHint{"[Enter]", "submit"}
	trainerHintHint        = installerHint{"[Tab]", "hint"}
	trainerHintScroll      = installerHint{"[PgUp/PgDn]", "scroll"}
	trainerHintDelete      = installerHint{"[Backspace]", "delete"}
	trainerHintEscToken    = installerHint{"[Ctrl-e]", "type " + trainer.EscToken}
	trainerHintBlockVisual = installerHint{"[Ctrl-v]", "block"}
	trainerHintBackToMenu  = installerHint{"[Esc]", "back"}
	trainerHintQuit        = installerHint{"[Esc]", "quit"}
	trainerHintForfeit     = installerHint{"[Esc]", "forfeit"}
)

// trainerAnswer is the typed answer as the screen shows it: control characters
// the engine parses are spelled out, and an empty answer keeps the field
// visible instead of collapsing it.
func (m Model) trainerAnswer() string {
	answer := formatControlChars(m.TrainerInput)
	if answer == "" {
		return "..."
	}
	return answer
}

// cutMarker ends a string that had to be shortened to fit the frame. It is a
// plain text glyph on purpose: it has to be readable on a 16-colour terminal
// and in a terminal without an emoji font. Its whole job is to say that
// something was dropped, which the frame edge used to do silently.
const cutMarker = "…"

// trainerInnerWidth is the trainer's name for the width every screen shares.
func trainerInnerWidth(m Model) int {
	return contentWidth(m)
}

// trainerCurrentExercise returns the exercise the exercise or boss screen is
// showing, or nil when there is none.
func (m Model) trainerCurrentExercise() *trainer.Exercise {
	if m.TrainerGameState == nil {
		return nil
	}
	return m.TrainerGameState.CurrentExercise
}

// ruleText returns the separator a rule is made of, sized to the frame. It is
// the raw string so a caller that stores a separator in data (the menu options)
// and a caller that renders one (rule) draw the same rule instead of two
// separators that disagree: a 13-glyph string used to sit in the menus while the
// keymap tables hard-coded a 60-glyph rule that overflowed a 60-column terminal.
func ruleText(width int) string {
	if width < 1 {
		width = 1
	}
	return strings.Repeat("─", width)
}

// rule renders the separator a trainer code block is wrapped in. It replaces
// the 60-column rule the exercise and boss screens each wrote out by hand, so
// the two cannot drift apart and the rule follows the frame instead of being
// fixed at a width the frame may not have.
func rule(width int) string {
	return MutedStyle.Render(ruleText(width))
}

// headerRule draws the frame's top rule in the brand tone, the counterpart to
// the quiet rule above the footer. It is a heavy glyph run, where every other
// separator is the light one, so the two edges of the frame differ by weight as
// well as by tone and the eye lands on the header first. The glyph keeps the
// difference visible on a terminal with no colour.
func headerRule(width int) string {
	return headerRuleStyle(HeaderRuleStyle, width)
}

// headerRuleStyle is headerRule in a given tone, so the frame can crown the
// completion screen in the success colour without a second copy of the glyph run.
func headerRuleStyle(style lipgloss.Style, width int) string {
	if width < 1 {
		width = 1
	}
	return style.Render(strings.Repeat("━", width))
}

// cutToWidth returns as much of s as fits in width columns, in whole runes. It
// cuts without a marker, for callers that need to know where the cut landed.
func cutToWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	used := 0
	for i, r := range s {
		w := lipgloss.Width(string(r))
		if used+w > width {
			return s[:i]
		}
		used += w
	}
	return s
}

// truncate returns s cut to fit width columns, marked when anything was
// dropped, so an overlong line reads as shortened instead of looking like a
// line that happens to end at the frame edge.
func truncate(s string, width int) string {
	if width < 1 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	return cutToWidth(s, width-1) + cutMarker
}

// tailToWidth keeps the last columns of s, marking the cut at the front. The
// answer field shows the end of what the player typed, the way a text field
// does, rather than the beginning that was already committed.
func tailToWidth(s string, width int) string {
	if width < 1 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	runes := []rune(s)
	used := 0
	start := len(runes)
	for i := len(runes) - 1; i >= 0; i-- {
		w := lipgloss.Width(string(runes[i]))
		if used+w > width-1 {
			break
		}
		used += w
		start = i
	}
	return cutMarker + string(runes[start:])
}

// padRight pads s to width columns, counting columns rather than bytes, so a
// glyph in a field does not shift the column after it.
func padRight(s string, width int) string {
	if gap := width - lipgloss.Width(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s
}

// wrapText wraps text into rows of at most width columns, preferring a space so
// words stay whole and breaking inside a word only when one is wider than a row.
// It returns at most maxRows rows (0 means as many as the text needs); when the
// text does not fit, the last row ends with the cut marker, so the reader can
// see that words were dropped instead of guessing at a silent clip.
func wrapText(text string, width, maxRows int) []string {
	if width < 1 {
		width = 1
	}
	rest := strings.TrimSpace(text)
	if rest == "" {
		return nil
	}

	var rows []string
	for rest != "" {
		row, remainder := splitAtWidth(rest, width)
		rows = append(rows, row)
		rest = strings.TrimSpace(remainder)
	}

	if maxRows > 0 && len(rows) > maxRows {
		rows = rows[:maxRows]
		rows[maxRows-1] = cutToWidth(rows[maxRows-1], width-1) + cutMarker
	}
	return rows
}

// splitAtWidth splits text into a row of at most width columns and the rest.
func splitAtWidth(text string, width int) (string, string) {
	if width < 1 {
		width = 1
	}
	used := 0
	cut := len(text)
	for i, r := range text {
		w := lipgloss.Width(string(r))
		if used+w > width {
			cut = i
			break
		}
		used += w
	}
	if cut == 0 {
		// A single glyph wider than the row still has to advance, or wrapping
		// would never terminate.
		_, size := utf8.DecodeRuneInString(text)
		return text[:size], text[size:]
	}
	if cut < len(text) {
		// A cut that lands on a space is already a word boundary; anything else
		// backs up to the last space so the word stays whole.
		if text[cut] == ' ' {
			return text[:cut], text[cut+1:]
		}
		if space := strings.LastIndex(text[:cut], " "); space > 0 {
			return text[:space], text[space+1:]
		}
	}
	return text[:cut], text[cut:]
}

// trainerLivesGlyphs renders a boss's remaining lives as filled hearts and its
// spent lives as outlined ones, beside the "Lives 3/5" count. Both are text
// presentation characters: the screen used to encode the lives as red and black
// heart EMOJI, which a terminal without an emoji font draws as two boxes, so
// the lives vanished exactly where the installer is meant to survive. The count
// carries the meaning and the glyphs make it scannable.
func trainerLivesGlyphs(remaining, total int) string {
	if remaining < 0 {
		remaining = 0
	}
	if remaining > total {
		remaining = total
	}
	return strings.Repeat("♥", remaining) + strings.Repeat("♡", total-remaining)
}

// trainerModuleStatus names a module's state in the trainer menu as a glyph
// plus a word, never as a glyph alone. The emoji it replaced (a lock, crossed
// swords, a bullseye) became a tofu box in any terminal without an emoji font,
// which took the state with it, and the installer's other screens encode state
// in a glyph or a word for exactly that reason. The ladder is the order the
// module's gates open: locked, lessons, practice, boss, cleared. Lessons
// complete has no rung of its own because IsPracticeReady is defined as
// unlocked-and-lessons-complete, so the practice rung already covers it -- the
// emoji ladder this replaces had a dead 📚 branch for the same reason.
func trainerModuleStatus(unlocked, bossDefeated, bossReady, practiceReady bool) string {
	switch {
	case !unlocked:
		return "✗ locked"
	case bossDefeated:
		return "✓ cleared"
	case bossReady:
		return "● boss"
	case practiceReady:
		return "● practice"
	default:
		return "○ lessons"
	}
}

// trainerModuleMeterText renders the per-module progress the trainer menu shows
// as the two trailing meters a row can carry: a bar and a count for the lessons
// completed and another for the exercises mastered. Mastery comes from
// GetPracticeStatsForModule, which reads the same ExerciseStats.IsMastered
// predicate weighted practice selection uses, so the count on screen cannot
// drift from the practice pool. The text is plain because the shared rowBar
// paints the whole trailing meter in one tone; the fill is carried by the ▓/░
// glyphs, not by a colour, so the meter still reads on a monochrome terminal.
// The boss is not repeated here: the row's status field already says whether the
// boss is cleared, and saying it twice is what cost the row its room inside 80
// columns.
func trainerModuleMeterText(progress *trainer.ModuleProgress, practice trainer.PracticeStats) string {
	lessonsCompleted, lessonsTotal := 0, 0
	if progress != nil {
		lessonsCompleted = progress.LessonsCompleted
		lessonsTotal = progress.LessonsTotal
	}
	// LessonsTotal is only recorded once the module is opened, so a module that
	// has never been started falls back to the real lesson count instead of
	// showing 0/0 next to a mastery count that already knows the total.
	if lessonsTotal == 0 {
		lessonsTotal = practice.TotalExercises
	}

	lessonsFraction := 0.0
	if lessonsTotal > 0 {
		lessonsFraction = float64(lessonsCompleted) / float64(lessonsTotal)
	}
	masteryFraction := 0.0
	if practice.TotalExercises > 0 {
		masteryFraction = float64(practice.MasteredCount) / float64(practice.TotalExercises)
	}

	return fmt.Sprintf("Done %s %d/%d Mastered %s %d/%d",
		meterCellsPlain(lessonsFraction, trainerRowMeterCells), lessonsCompleted, lessonsTotal,
		meterCellsPlain(masteryFraction, trainerRowMeterCells), practice.MasteredCount, practice.TotalExercises)
}

// trainerPracticeAccuracyText renders the selected module's practice accuracy,
// the percentage the boss unlock threshold is measured against, and how many
// mastered exercises have come due for review. It is omitted until the module has
// a recorded attempt, so a fresh module does not claim 0%. It names itself because
// it is now a line of the selected module's detail block rather than a suffix on
// the row, where "Acc 60%" had to be terse. The due count shares the line rather
// than taking a row of its own: the menu's row budget is fixed, and a new row here
// would push the legend off an 80x24 screen.
func trainerPracticeAccuracyText(progress *trainer.ModuleProgress, module trainer.ModuleID, now time.Time) string {
	if progress == nil || progress.PracticeAttempts == 0 {
		return ""
	}
	text := fmt.Sprintf("Practice accuracy: %.0f%%", progress.PracticeAccuracy*100)
	if due := progress.ReviewsDue(module, now); due > 0 {
		word := "reviews"
		if due == 1 {
			word = "review"
		}
		text += fmt.Sprintf(" · %d mastered %s due", due, word)
	}
	return text
}

// trainerWeakExerciseText renders the exercises a module misses most, in the
// order GetPracticeStatsForModule returns them (most wrong first), with the
// recorded wrong count. Entries are added while the line fits maxWidth (0 means
// no limit), so a long list degrades by dropping the least-missed entries
// instead of wrapping and breaking the layout. It reads the recorded
// per-exercise stats and adds no bookkeeping of its own; an empty profile
// yields "", so the menu omits the line instead of showing junk.
func trainerWeakExerciseText(progress *trainer.ModuleProgress, practice trainer.PracticeStats, maxWidth int) string {
	if len(practice.WeakestExercises) == 0 {
		return ""
	}

	const prefix = "⚠️ Weakest: "
	const separator = " · "

	entries := make([]string, 0, len(practice.WeakestExercises))
	for _, id := range practice.WeakestExercises {
		entry := id
		if progress != nil && progress.ExerciseStats != nil {
			if exStats := progress.ExerciseStats[id]; exStats != nil && exStats.TotalWrong > 0 {
				entry = fmt.Sprintf("%s (%d✗)", id, exStats.TotalWrong)
			}
		}
		entries = append(entries, entry)
	}

	// Keep at least one entry so a single over-long identifier still surfaces.
	for len(entries) > 1 {
		if maxWidth <= 0 || lipgloss.Width(prefix+strings.Join(entries, separator)) <= maxWidth {
			break
		}
		entries = entries[:len(entries)-1]
	}

	return prefix + strings.Join(entries, separator)
}

// trainerMenuVital is the trainer menu's vital sign: the run's boss progress as
// a meter, then the score, the streak and the boss count that used to be a
// separate stats row. A nil profile has nothing to report and gets no filler.
func (m Model) trainerMenuVital() string {
	if m.TrainerStats == nil {
		return ""
	}
	total := len(trainer.GetAllModules())
	bosses := len(m.TrainerStats.BossesDefeated)
	return meterRendered(bosses, total, trainerHeaderMeterCells) +
		MeterStyle.Render(fmt.Sprintf(" · Score %d · Streak %d · Bosses %d/%d",
			m.TrainerStats.TotalScore, m.TrainerStats.CurrentStreak, bosses, total))
}

// renderTrainerMenu renders the module list inside the trainer's header frame.
// Each module is one row bar carrying its status (a glyph and a word), its name
// and its two trailing meters -- lessons completed and exercises mastered -- so
// the progress that used to run the row past 80 columns is now a glyph run and
// its count. The selected module's commands, practice accuracy and most-missed
// exercises are a guttered block under its row instead of loose indented lines,
// which is what makes them read as that module's detail rather than as three
// more list entries.
func (m Model) renderTrainerMenu() string {
	inner := trainerInnerWidth(m)

	rows := []string{
		headerRow("Vim Mastery Trainer", m.trainerMenuVital(), inner),
		headerRule(inner),
		// The selected row's bar and the brand rule are near-identical tones; without
		// a blank row between them the top of the list and the header merge into one
		// lavender band exactly where the separator is needed.
		"",
	}

	for i, module := range m.TrainerModules {
		isUnlocked := m.TrainerStats != nil && m.TrainerStats.IsModuleUnlocked(module.ID)
		isBossDefeated := m.TrainerStats != nil && m.TrainerStats.IsBossDefeated(module.ID)
		isPracticeReady := m.TrainerStats != nil && m.TrainerStats.IsPracticeReady(module.ID)
		isBossReady := m.TrainerStats != nil && m.TrainerStats.IsBossReady(module.ID)

		// The status is a glyph plus a word, so a monochrome terminal and one with
		// no emoji font lose nothing. It is padded so every module name starts on
		// the same column whatever state the module is in.
		status := padRight(trainerModuleStatus(isUnlocked, isBossDefeated, isBossReady, isPracticeReady), trainerModuleStatusWidth)
		// The module icon was an emoji: on a terminal without an emoji font it drew
		// a blank cell, and the status word already says the state. The name is the
		// row.
		label := fmt.Sprintf("%s %s", status, module.Name)

		// Progress is display-only text attached to the existing entry. The
		// unlock/ready predicates above and this direct map read both look the
		// recorded progress up without creating records, so rendering the menu
		// neither manufactures module records nor exercise records in the
		// persisted stats.
		var progress *trainer.ModuleProgress
		var practice trainer.PracticeStats
		meter := ""
		if m.TrainerStats != nil {
			progress = m.TrainerStats.ModuleProgress[module.ID]
			practice = trainer.GetPracticeStatsForModule(module.ID, progress)
			meter = trainerModuleMeterText(progress, practice)
		}

		rows = append(rows, m.rowBarWidth(label, i == m.TrainerCursor, meter, inner))

		// The selected module's detail block: its commands, the accuracy the row
		// no longer carries, and the exercises it misses most. All three are
		// display-only lines attached to the entry and all three sit behind one
		// gutter.
		if i == m.TrainerCursor {
			detail := []string{InkStyle.Render(truncate(module.Description, inner-blockGutterWidth))}
			if accuracy := trainerPracticeAccuracyText(progress, module.ID, time.Now()); accuracy != "" {
				detail = append(detail, InfoStyle.Render(accuracy))
			}
			if weak := trainerWeakExerciseText(progress, practice, inner-blockGutterWidth); weak != "" {
				detail = append(detail, WarningStyle.Render(weak))
			}
			rows = append(rows, gutteredBlock(detail)...)
		}
	}

	// Feedback. The rows are held even when there is nothing to say, so the
	// module rows above never move when a message arrives, and they double as the
	// gap above the legend.
	message := wrapText(m.TrainerMessage, inner, trainerMessageRows)
	for i := 0; i < trainerMessageRows; i++ {
		if i < len(message) {
			rows = append(rows, WarningStyle.Render(message[i]))
		} else {
			rows = append(rows, "")
		}
	}

	// Legend, packed by the shared footer component: the keys read as keys and
	// the verbs as verbs, the same way every installer screen's footer does. The
	// menu's eight hints pack to two rows at the 80-column floor, the height the
	// rows above already reserve.
	footer := m.footerBar(inner, []installerHint{
		hintUp, hintDown,
		trainerHintLesson, trainerHintPractice, trainerHintBoss,
		trainerHintReset, trainerHintResetAll, trainerHintBack,
	})
	if m.Height > trainerFloorHeight {
		rows = append(rows, m.trainerCompanionRows(inner, len(rows), len(footer))...)
	}
	rows = append(rows, footer...)

	return strings.Join(rows, "\n")
}

// trainerTextBudget splits the rows the chrome leaves (trainerFrameRows)
// between the mission text, which cannot be scrolled, and the code window, which
// can. The renderer and the scroll key both ask for it, so the window the keys
// scroll is exactly the window the screen draws.
//
// On an exercise screen that composes two columns it returns the two-column
// budget instead: the mission wraps in the right column and the code window
// keeps every row the left column can hold. trainerTwoColumnLayout is the one
// predicate that decides which budget applies, so the renderer, the scroll keys
// and the frame guard cannot disagree about the shape of the screen.
func (m Model) trainerTextBudget(exercise *trainer.Exercise) (missionRows []string, codeRows int) {
	l := layoutFor(m)
	if m.trainerTwoColumnLayout(l) {
		return m.trainerWideTextBudget(exercise, l)
	}

	reserve := 0
	if m.Height > trainerFloorHeight {
		reserve = m.companionHeightNow()
	}
	flex := m.Height - trainerFrameRows - reserve
	if flex < trainerCodeMinRows+1 {
		flex = trainerCodeMinRows + 1
	}

	maxMission := trainerMissionMaxRows
	if limit := flex - trainerCodeMinRows; maxMission > limit {
		maxMission = limit
	}
	missionRows = wrapText(exercise.Mission, trainerInnerWidth(m)-blockGutterWidth, maxMission)

	codeRows = flex - len(missionRows)
	if codeRows < trainerCodeMinRows {
		codeRows = trainerCodeMinRows
	}
	return missionRows, codeRows
}

// trainerRightColumnChrome is the right column's fixed rows: the mission chip,
// the answer chip, the answer line and the fixed feedback area. One mission row
// on top of it is the shortest the column can be.
const trainerRightColumnChrome = 5

// trainerWideTextBudget is the two-column budget for an exercise screen. The
// footer the screen will actually draw decides how many body rows the frame
// leaves, the code window takes them all in its own column, and the mission
// wraps in the right column with the rows the answer and the feedback do not
// need. The mission is capped at what the right column can hold, which is the
// existing cut marker's job at an extreme height, not a normal case.
func (m Model) trainerWideTextBudget(exercise *trainer.Exercise, l layout) (missionRows []string, codeRows int) {
	// The budget reserves the full hint set, including the scroll key, so the
	// body arithmetic cannot change when the drawn footer drops it on an exercise
	// whose code fits. The screen may spend one row less than the budget in that
	// case; it never spends more than the terminal.
	bodyRows := installerBodyRows(m.Height, m.footerBarRows(l.Inner, m.trainerExerciseHints(true)))
	if m.Height > trainerFloorHeight {
		bodyRows -= m.companionHeightNow()
	}

	// The code window owns the left column: the column minus its chip.
	codeRows = bodyRows - 1
	if codeRows < trainerCodeMinRows {
		codeRows = trainerCodeMinRows
	}

	maxMission := bodyRows - trainerRightColumnChrome
	if maxMission < 1 {
		maxMission = 1
	}
	missionRows = wrapText(exercise.Mission, l.Right-blockGutterWidth, maxMission)
	return missionRows, codeRows
}

// trainerCodeOffset clamps a window offset to the code it is shown over, so an
// offset left over from a longer exercise cannot leave a shorter one blank.
func trainerCodeOffset(offset, rows, total int) int {
	if offset < 0 {
		return 0
	}
	if rows < 1 {
		rows = 1
	}
	if max := total - rows; offset > max {
		if max < 0 {
			return 0
		}
		return max
	}
	return offset
}

// trainerCodeAnchor is where a freshly presented exercise's window starts: the
// top of the code, moved down only as far as the start cursor needs to stay
// visible on entry. Entering a regex boss step whose cursor sits on line ten
// therefore shows the cell the step begins from, instead of the top of the code
// with the cursor hidden below the fold.
func trainerCodeAnchor(startLine, rows, total int) int {
	if rows < 1 {
		rows = 1
	}
	if startLine < rows {
		return 0
	}
	return trainerCodeOffset(startLine-rows+1, rows, total)
}

// codeViewport is the visible window over an exercise's code, plus the cursor
// and selection state drawn into it. renderTrainerExercise and renderTrainerBoss
// both build one: they used to carry two copies of this rendering, about 170
// lines of it, so every fix had to be made twice and the two screens drifted.
type codeViewport struct {
	exercise *trainer.Exercise
	// start is the cell the exercise begins from, current is where the player's
	// input has moved the cursor, and selection is the visual range it opened.
	start     trainer.Position
	current   trainer.SimulatedPosition
	selection trainer.Selection
	// skipSimulation draws only the start cursor, for the exercises the simulator
	// neither follows nor judges.
	skipSimulation bool
	// offset is the first visible code row, already clamped to the code.
	offset int
	// rows is how many code rows the window shows.
	rows int
	// width is the columns a code line may use, excluding the line-number gutter.
	// A longer line is cut and marked rather than clipped by the frame edge.
	width int
}

// trainerCodeViewport builds the code window for an exercise.
func (m Model) trainerCodeViewport(exercise *trainer.Exercise, codeRows, width int) codeViewport {
	// The window opens where the exercise starts -- the code row its own cursor is
	// on -- until the player scrolls this exercise, and it is clamped to the code
	// either way, so an offset left over from a longer exercise cannot blank a
	// shorter one.
	offset := trainerCodeAnchor(exercise.CursorPos.Line, codeRows, len(exercise.Code))
	if m.trainerCodeScrolled(exercise) {
		offset = trainerCodeOffset(m.TrainerCodeScroll, codeRows, len(exercise.Code))
	}

	viewport := codeViewport{
		exercise: exercise,
		start:    exercise.CursorPos,
		offset:   offset,
		rows:     codeRows,
		width:    width,
	}

	// Exercises that are not pure motions are neither simulated nor validated by
	// the simulator; the shared predicate keeps this render site in step with
	// ValidateAnswerDetailed, and the cursor then stays where the exercise put it.
	viewport.skipSimulation = trainer.ShouldSkipSimulation(exercise)
	if viewport.skipSimulation {
		viewport.current = trainer.SimulatedPosition{Line: exercise.CursorPos.Line, Col: exercise.CursorPos.Col}
		return viewport
	}

	simResult := trainer.SimulateMotionsWithSelection(exercise.CursorPos, exercise.Code, m.TrainerInput)
	viewport.current = simResult.Position
	viewport.selection = simResult.Selection
	return viewport
}

// label names the code block and, when the window is not showing all of the
// exercise, which rows of it are on screen: the scroll keys change which lines
// appear under the mission, so the window has to say where it is. The chip is
// the block's head; the range is the block's metadata, in the dim tone the rest
// of the program gives a count.
func (v codeViewport) label() string {
	if v.rows >= len(v.exercise.Code) {
		return chip("Code")
	}
	last := v.offset + v.rows
	if last > len(v.exercise.Code) {
		last = len(v.exercise.Code)
	}
	return chip("Code") + MeterStyle.Render(fmt.Sprintf("  rows %d-%d of %d", v.offset+1, last, len(v.exercise.Code)))
}

// render returns the visible code rows, one string per row.
func (v codeViewport) render() []string {
	rows := make([]string, 0, v.rows)
	for i := 0; i < v.rows; i++ {
		lineNum := v.offset + i
		if lineNum >= len(v.exercise.Code) {
			break
		}

		// An overlong line is cut and marked: the frame edge used to clip it
		// silently, mid-call, so a reader could not tell that anything was missing.
		line := v.exercise.Code[lineNum]
		kept, cut := line, false
		if lipgloss.Width(line) > v.width {
			kept, cut = cutToWidth(line, v.width-1), true
		}

		rows = append(rows, MutedStyle.Render(fmt.Sprintf("%2d │ ", lineNum+1))+v.renderLine(lineNum, kept, cut))
	}
	return rows
}

// renderLine renders one code row: the code with the start cursor, the current
// cursor and the visual selection drawn on it. kept is the part of the line the
// window shows, and cut says whether the rest of it was dropped.
func (v codeViewport) renderLine(lineNum int, kept string, cut bool) string {
	// The simulator neither follows nor judges these exercises, so only the cell
	// the exercise starts from is marked.
	if v.skipSimulation {
		if !v.cursorVisible(lineNum == v.start.Line, v.start.Col, kept, cut) {
			return v.cut(CodeStyle.Render(kept), cut)
		}
		return v.cut(withCursor(kept, v.start.Col, StartCursorStyle), cut)
	}

	if v.selection.Active && lineNum == v.selection.StartLine {
		// The marker stays outside the selection: a selection that runs to the
		// frame edge should not paint the ellipsis as if it were selected text.
		return v.cut(renderLineWithSelection(kept, v.start, v.selection), cut)
	}

	startVisible := v.cursorVisible(lineNum == v.start.Line, v.start.Col, kept, cut)
	currentVisible := v.cursorVisible(lineNum == v.current.Line, v.current.Col, kept, cut)

	switch {
	case startVisible && currentVisible && v.start.Col == v.current.Col:
		// The input has not moved the cursor off the cell the exercise starts
		// from, so the two marks are one mark and the start one wins.
		return v.cut(withCursor(kept, v.start.Col, StartCursorStyle), cut)
	case startVisible && currentVisible:
		return v.cut(renderLineWithTwoCursors(kept, v.start.Col, v.current.Col), cut)
	case startVisible:
		return v.cut(withCursor(kept, v.start.Col, StartCursorStyle), cut)
	case currentVisible:
		return v.cut(withCursor(kept, v.current.Col, CurrentCursorStyle), cut)
	default:
		return v.cut(CodeStyle.Render(kept), cut)
	}
}

// cut appends the cut marker to a rendered row when the line it came from did
// not fit the window.
func (v codeViewport) cut(rendered string, cut bool) string {
	if cut {
		return rendered + MutedStyle.Render(cutMarker)
	}
	return rendered
}

// cursorVisible reports whether a cursor on col is drawn: it has to sit on a
// cell the window shows. A cursor past the cut of an overlong line is not drawn
// at all, because a block at the truncation point would claim the cursor sits
// there.
func (v codeViewport) cursorVisible(onLine bool, col int, kept string, cut bool) bool {
	if !onLine {
		return false
	}
	if col < len(kept) {
		return true
	}
	// The cell past the end of a line is a real cell while the line is shown in
	// full: that is where Vim puts the cursor at the end of the code.
	return col == len(kept) && !cut
}

// withCursor draws text with the cell at col carrying style, the way Vim draws
// its cursor block. A col at the end of the text draws a highlighted space,
// which is the virtual cell after the last character.
func withCursor(text string, col int, style lipgloss.Style) string {
	if col >= len(text) {
		return CodeStyle.Render(text) + style.Render(" ")
	}
	return CodeStyle.Render(text[:col]) + style.Render(text[col:col+1]) + CodeStyle.Render(text[col+1:])
}

// trainerFeedbackRows renders the feedback area: the current message wrapped to
// fit, in a fixed number of rows so the layout above it never moves when a
// message arrives or goes away.
func (m Model) trainerFeedbackRows(style lipgloss.Style) []string {
	return m.trainerFeedbackRowsWidth(style, trainerInnerWidth(m))
}

// trainerFeedbackRowsWidth is trainerFeedbackRows measured against a given
// width, so the two-column right column wraps the feedback to its own column
// instead of the whole room.
func (m Model) trainerFeedbackRowsWidth(style lipgloss.Style, width int) []string {
	rows := make([]string, trainerMessageRows)
	for i, row := range wrapText(m.TrainerMessage, width, trainerMessageRows) {
		rows[i] = style.Render(row)
	}
	return rows
}

// trainerExerciseVital is the exercise screen's vital sign, carried by its header
// instead of three stacked rows: where the player is in the module (a position
// meter, in lesson mode), the session's score and streak, and the live countdown
// to the automatic hint. The countdown is derived from the exercise's own
// TimeoutSecs through the game state's injected clock, so it keeps counting while
// the animation tick re-renders the screen; once the deadline passes the countdown
// leaves the header and the hint itself appears in the feedback area. The row
// stays either way, so nothing below it moves when that happens.
func (m Model) trainerExerciseVital() string {
	state := m.TrainerGameState
	parts := make([]string, 0, 3)
	if state.IsLessonMode {
		parts = append(parts, meterRendered(state.ExerciseIndex+1, len(state.Exercises), trainerHeaderMeterCells))
	}
	parts = append(parts, MeterStyle.Render(fmt.Sprintf("Score %d · Streak %d", state.SessionScore, state.CurrentStreak)))
	if combo := trainerComboText(state.ComboMultiplier); combo != "" {
		parts = append(parts, MeterStyle.Render(combo))
	}
	if remaining := state.RemainingSeconds(); remaining > 0 {
		parts = append(parts, WarningStyle.Render(fmt.Sprintf("Hint in %ds", int(math.Ceil(remaining)))))
	}
	return strings.Join(parts, "  ")
}

// trainerExerciseHints are the legend entries a lesson or practice screen
// packs, in the canonical order every footer uses. It is one list so the footer
// and the two-column row budget agree on how many rows the legend spends, and
// the hint key is listed because the screen honours it (see
// handleTrainerExerciseKeys).
func (m Model) trainerExerciseHints(scrolls bool) []installerHint {
	hints := []installerHint{trainerHintTypeCommand, trainerHintSubmit, trainerHintHint}
	if scrolls {
		// The key is named only while the code window has rows off screen; an
		// exercise whose code fits does not advertise a scroll that cannot move.
		hints = append(hints, trainerHintScroll)
	}
	// Esc saves the exercise and returns to the trainer menu; it does not quit the
	// program. The footer says what the key does.
	return append(hints, trainerHintDelete, trainerHintEscToken, trainerHintBlockVisual, trainerHintBackToMenu)
}

// renderTrainerExercise renders a lesson or practice exercise. The mission, the
// code window and the answer are guttered blocks under brand chips; the header
// carries the mode, the position, the score, the streak and the countdown that
// used to be four stacked rows. The code is a window because the screen has to
// fit the 24-row terminal the trainer claims: see trainerFrameRows for the
// arithmetic.
//
// Where the installer's own screens have room for two columns (see layoutFor),
// the exercise screen composes the same two columns: the code window on the
// left and the mission, the answer and the feedback on the right, through the
// shared composeColumns. The code window then keeps the rows the stacked right
// column no longer needs, so a wide terminal shows more code instead of a
// narrower window in the same shape. Below the floor the body is the one
// column it has always been.
func (m Model) renderTrainerExercise(mode string) string {
	exercise := m.trainerCurrentExercise()
	if exercise == nil {
		return m.trainerDeadEnd(mode, "No exercise loaded", "Press [Esc] to return to the trainer.")
	}

	l := layoutFor(m)
	inner := l.Inner

	rows := []string{
		headerRow(fmt.Sprintf("%s · %s", mode, string(m.TrainerGameState.CurrentModule)), m.trainerExerciseVital(), inner),
		headerRule(inner),
	}

	if m.trainerTwoColumnLayout(l) {
		_, codeRows := m.trainerWideTextBudget(exercise, l)
		hints := m.trainerExerciseHints(codeRows < len(exercise.Code))
		rows = append(rows, m.trainerExerciseColumns(exercise, l)...)
		footer := m.footerBar(inner, hints)
		rows = append(rows, m.trainerCompanionRows(inner, len(rows), len(footer))...)
		rows = append(rows, footer...)
		return strings.Join(rows, "\n")
	}

	mission, codeRows := m.trainerTextBudget(exercise)
	viewport := m.trainerCodeViewport(exercise, codeRows, inner-blockGutterWidth-trainerGutterWidth)

	// Every element below is one terminal row, which is what makes the code
	// window's height the frame minus the chrome instead of an estimate: the
	// screen used to render 22+N rows and lose its bottom.
	rows = append(rows, chip("Mission"))
	missionLines := make([]string, len(mission))
	for i, row := range mission {
		missionLines[i] = InfoStyle.Render(row)
	}
	rows = append(rows, gutteredBlock(missionLines)...)

	rows = append(rows, viewport.label())
	rows = append(rows, gutteredBlock(viewport.render())...)

	rows = append(rows,
		chip("Answer"),
		RuleStyle.Render("│ ")+KeyStyle.Render(tailToWidth(m.trainerAnswer(), inner-blockGutterWidth)),
	)
	rows = append(rows, m.trainerFeedbackRows(InfoStyle)...)

	// Help, packed by the shared footer component. It stays two rows at the
	// 80-column floor, so the screen's height is still countable: see
	// trainerFrameRows.
	hints := m.trainerExerciseHints(viewport.rows < len(exercise.Code))
	footer := m.footerBar(inner, hints)
	rows = append(rows, m.trainerCompanionRows(inner, len(rows), len(footer))...)
	rows = append(rows, footer...)

	return strings.Join(rows, "\n")
}

// trainerExerciseColumns composes the exercise screen's two-column body: the
// code window in the left column and the mission, the answer and the feedback
// in the right one. It is the one place the two halves are built, and both are
// measured against the layout's own columns, so the width the renderer draws
// with is the width layoutFor granted.
func (m Model) trainerExerciseColumns(exercise *trainer.Exercise, l layout) []string {
	mission, codeRows := m.trainerWideTextBudget(exercise, l)
	viewport := m.trainerCodeViewport(exercise, codeRows, l.Left-blockGutterWidth-trainerGutterWidth)

	left := append([]string{viewport.label()}, gutteredBlock(viewport.render())...)

	missionLines := make([]string, len(mission))
	for i, row := range mission {
		missionLines[i] = InfoStyle.Render(row)
	}
	right := []string{chip("Mission")}
	right = append(right, gutteredBlock(missionLines)...)
	right = append(right,
		chip("Answer"),
		RuleStyle.Render("│ ")+KeyStyle.Render(tailToWidth(m.trainerAnswer(), l.Right-blockGutterWidth)),
	)
	right = append(right, m.trainerFeedbackRowsWidth(InfoStyle, l.Right)...)

	return composeColumns(left, right, l)
}

// trainerTwoColumnLayout reports whether the current trainer screen composes
// two columns. It is the single predicate the renderer and the scroll/render
// budget both read, so neither can decide "wide" on its own. It requires the
// shared layout's own two-column room (layoutFor, the same threshold and the
// same columns as every installer screen), an exercise screen -- the boss and
// the menu keep their one-column bodies -- and a frame tall enough to hold the
// right column's fixed chrome plus one mission row. When the frame is too short
// the screen falls back to the one-column body rather than overflow it.
func (m Model) trainerTwoColumnLayout(l layout) bool {
	if !l.TwoColumn || (m.Screen != ScreenTrainerLesson && m.Screen != ScreenTrainerPractice) {
		return false
	}
	// The budget reserves the full hint set, including the scroll key, so the
	// body arithmetic cannot change when the drawn footer drops it on an exercise
	// whose code fits. The screen may spend one row less than the budget in that
	// case; it never spends more than the terminal.
	bodyRows := installerBodyRows(m.Height, m.footerBarRows(l.Inner, m.trainerExerciseHints(true)))
	return bodyRows >= trainerRightColumnChrome+1
}

// renderLineWithTwoCursors renders a line with both start and current cursor
func renderLineWithTwoCursors(line string, startCol, currentCol int) string {
	var result strings.Builder

	// Helper to get cursor character (use space for empty/end of line)
	getCursorChar := func(col int) string {
		if col < len(line) {
			return string(line[col])
		}
		return " "
	}

	// Determine order of cursors
	firstCol, secondCol := startCol, currentCol
	firstStyle, secondStyle := StartCursorStyle, CurrentCursorStyle
	if currentCol < startCol {
		firstCol, secondCol = currentCol, startCol
		firstStyle, secondStyle = CurrentCursorStyle, StartCursorStyle
	}

	// Handle empty line case
	if len(line) == 0 {
		// Both cursors on empty line at col 0
		if firstCol == secondCol {
			result.WriteString(firstStyle.Render(" "))
		} else {
			// This shouldn't happen on empty line, but handle it
			result.WriteString(firstStyle.Render(" "))
			result.WriteString(secondStyle.Render(" "))
		}
		return result.String()
	}

	// Build the line piece by piece
	// Part before first cursor
	if firstCol > 0 && firstCol <= len(line) {
		result.WriteString(CodeStyle.Render(line[:firstCol]))
	}

	// First cursor
	result.WriteString(firstStyle.Render(getCursorChar(firstCol)))

	// Part between cursors (if any)
	if firstCol+1 < secondCol && firstCol+1 < len(line) {
		endIdx := secondCol
		if endIdx > len(line) {
			endIdx = len(line)
		}
		if firstCol+1 < endIdx {
			result.WriteString(CodeStyle.Render(line[firstCol+1 : endIdx]))
		}
	}

	// Second cursor (only if different position from first)
	if secondCol > firstCol {
		result.WriteString(secondStyle.Render(getCursorChar(secondCol)))
	}

	// Part after second cursor
	if secondCol+1 < len(line) {
		result.WriteString(CodeStyle.Render(line[secondCol+1:]))
	}

	return result.String()
}

// renderLineWithSelection renders a line with visual selection highlighted
func renderLineWithSelection(line string, startPos trainer.Position, sel trainer.Selection) string {
	var result strings.Builder

	if len(line) == 0 {
		// Empty line with selection
		result.WriteString(SelectionStyle.Render(" "))
		return result.String()
	}

	// Clamp selection bounds to line length
	selStart := sel.StartCol
	selEnd := sel.EndCol
	if selStart < 0 {
		selStart = 0
	}
	if selEnd >= len(line) {
		selEnd = len(line) - 1
	}
	if selEnd < selStart {
		// Invalid selection, just render the line normally with start cursor
		if startPos.Col < len(line) {
			result.WriteString(CodeStyle.Render(line[:startPos.Col]))
			result.WriteString(StartCursorStyle.Render(string(line[startPos.Col])))
			if startPos.Col+1 < len(line) {
				result.WriteString(CodeStyle.Render(line[startPos.Col+1:]))
			}
		} else {
			result.WriteString(CodeStyle.Render(line))
		}
		return result.String()
	}

	// Render: [before selection] [SELECTION] [after selection]
	if selStart > 0 {
		result.WriteString(CodeStyle.Render(line[:selStart]))
	}

	// The selection itself
	selectedText := line[selStart : selEnd+1]
	result.WriteString(SelectionStyle.Render(selectedText))

	// After selection
	if selEnd+1 < len(line) {
		result.WriteString(CodeStyle.Render(line[selEnd+1:]))
	}

	return result.String()
}

// trainerComboText is the multiplier the score is paid by, named only once there is a
// run to speak of: "Combo ×1" on every screen would be noise rather than a fact. It is
// one function so the lesson, practice and boss headers cannot disagree about the
// wording or about when the fact is worth showing.
func trainerComboText(multiplier int) string {
	if multiplier <= 1 {
		return ""
	}
	return fmt.Sprintf("Combo ×%d", multiplier)
}

// trainerBossVital is the boss screen's vital sign, carried by its header instead
// of two stacked rows: the lives as a count and a glyph run, the step the fight
// is on, and the live countdown, each as a chip. The countdown is derived from
// the step's own TimeLimit plus any bonus through the game state's injected
// clock, so it keeps counting while the animation tick re-renders the screen;
// once the deadline passes the countdown leaves the header and the tick charges
// the life.
func (m Model) trainerBossVital() string {
	state := m.TrainerGameState
	boss := state.CurrentBoss
	if boss == nil {
		return ""
	}

	parts := []string{
		chip(fmt.Sprintf("Lives: %d/%d %s", state.BossLives, boss.Lives, trainerLivesGlyphs(state.BossLives, boss.Lives))),
		chip(fmt.Sprintf("Step %d/%d", state.BossStep+1, len(boss.Steps))),
	}
	// The combo sits before the countdown rather than after it: the header drops what
	// does not fit from its tail, and a run of wins is worth less than the seconds
	// left on the step.
	if combo := trainerComboText(state.ComboMultiplier); combo != "" {
		parts = append(parts, chip(combo))
	}
	if remaining := state.BossStepSecondsLeft(); remaining > 0 {
		parts = append(parts, chip(fmt.Sprintf("Time left: %ds", int(math.Ceil(remaining)))))
	}
	return strings.Join(parts, "  ")
}

func (m Model) renderTrainerBoss() string {
	state := m.TrainerGameState
	if state == nil || state.CurrentBoss == nil {
		return m.trainerDeadEnd("Boss", "No boss loaded", "Press [Esc] to return to the trainer.")
	}

	boss := state.CurrentBoss
	inner := trainerInnerWidth(m)
	exercise := state.CurrentExercise

	// Every element below is one terminal row. The boss screen spends the same
	// chrome as the exercise screen (trainerFrameRows), so it gets the same code
	// window: a 14-line boss step scrolls instead of running off the bottom, which
	// is where its whole second half used to be. The lives and the countdown are
	// chips in the header rather than the two label rows they used to stack.
	rows := []string{
		headerRow("⚔️ Boss · "+boss.Name, m.trainerBossVital(), inner),
		headerRule(inner),
		chip("Challenge"),
	}

	if state.BossStep >= len(boss.Steps) || exercise == nil {
		// No step is on screen, so there is no code window to size around. The
		// legend packs through the shared footer component like every other.
		rows = append(rows, m.trainerFeedbackRows(WarningStyle)...)
		hints := []installerHint{
			trainerHintTypeCommand, trainerHintSubmit, trainerHintEscToken,
			trainerHintScroll, trainerHintForfeit, trainerHintBlockVisual,
		}
		footer := m.footerBar(inner, hints)
		rows = append(rows, m.trainerCompanionRows(inner, len(rows), len(footer))...)
		rows = append(rows, footer...)
		return strings.Join(rows, "\n")
	}

	mission, codeRows := m.trainerTextBudget(exercise)
	missionLines := make([]string, len(mission))
	for i, row := range mission {
		missionLines[i] = InfoStyle.Render(row)
	}
	rows = append(rows, gutteredBlock(missionLines)...)

	viewport := m.trainerCodeViewport(exercise, codeRows, inner-blockGutterWidth-trainerGutterWidth)
	rows = append(rows, viewport.label())
	rows = append(rows, gutteredBlock(viewport.render())...)
	rows = append(rows,
		chip("Answer"),
		RuleStyle.Render("│ ")+KeyStyle.Render(tailToWidth(m.trainerAnswer(), inner-blockGutterWidth)),
	)
	rows = append(rows, m.trainerFeedbackRows(WarningStyle)...)
	hints := []installerHint{
		trainerHintTypeCommand, trainerHintSubmit, trainerHintHint,
		trainerHintScroll, trainerHintForfeit, trainerHintEscToken, trainerHintBlockVisual,
	}
	footer := m.footerBar(inner, hints)
	rows = append(rows, m.trainerCompanionRows(inner, len(rows), len(footer))...)
	rows = append(rows, footer...)

	return strings.Join(rows, "\n")
}

// renderBufferLines renders a buffer as a guttered block of numbered lines,
// matching the numbering the exercise screen uses so the result lines up with the
// code the user saw. A line wider than the frame is cut and marked: the frame
// edge used to clip the buffer silently, which is the defect the exercise screen
// had. It returns rows rather than a string so the caller can put them under a
// chip and so the frame counts every row of the block.
func renderBufferLines(buffer []string, inner int) []string {
	width := inner - blockGutterWidth - trainerGutterWidth
	rows := make([]string, len(buffer))
	for i, line := range buffer {
		rows[i] = MutedStyle.Render(fmt.Sprintf("%2d │ ", i+1)) + CodeStyle.Render(truncate(line, width))
	}
	return gutteredBlock(rows)
}

func (m Model) renderTrainerResult() string {
	inner := trainerInnerWidth(m)

	// The outcome is the header's name and the score is its vital sign. The
	// outcome is a glyph and a word, so it reads on a terminal with no colour and
	// on one with no emoji font; the stacked result title and the session-score
	// row it replaces are gone, which is one of the rows the blocks below gained.
	name := "✗ Incorrect"
	if m.TrainerLastCorrect {
		name = "✓ Correct"
	}
	vital := ""
	if m.TrainerGameState != nil {
		vital = MeterStyle.Render(fmt.Sprintf("Score: %d · Streak: %d",
			m.TrainerGameState.SessionScore, m.TrainerGameState.CurrentStreak))
	}

	rows := []string{
		headerRow(name, vital, inner),
		headerRule(inner),
		chip("Summary"),
	}

	// The message is wrapped instead of being left to the frame edge, which cut
	// the solutions list mid-word.
	message := wrapText(m.TrainerMessage, inner-blockGutterWidth, 0)
	if len(message) == 0 {
		message = []string{""}
	}
	messageLines := make([]string, len(message))
	for i, row := range message {
		messageLines[i] = InfoStyle.Render(row)
	}
	rows = append(rows, gutteredBlock(messageLines)...)

	if m.TrainerGameState != nil && m.TrainerGameState.CurrentExercise != nil {
		exercise := m.TrainerGameState.CurrentExercise
		if exercise.Explanation != "" {
			rows = append(rows, chip("Explanation"))
			// The explanation is wrapped rather than clipped: the result screen's
			// snapshot used to end mid-word at the frame edge.
			explanation := wrapText(exercise.Explanation, inner-blockGutterWidth, 0)
			explanationLines := make([]string, len(explanation))
			for i, row := range explanation {
				explanationLines[i] = MutedStyle.Render(row)
			}
			rows = append(rows, gutteredBlock(explanationLines)...)
		}

		// A buffer-verified answer is taught by the buffer it produced, which is
		// the only place its effect is visible. The preview is gated on the
		// judge that ran, so a shipped exercise renders exactly as before. A
		// rejected answer shows the expected buffer next to the produced one,
		// because the difference is what the lesson is about.
		if exercise.BufferVerified && m.TrainerValidation != nil && m.TrainerValidation.BufferVerified {
			if !m.TrainerValidation.IsCorrect {
				rows = append(rows, chip("Expected buffer"))
				rows = append(rows, renderBufferLines(m.TrainerValidation.TargetBuffer, inner)...)
			}
			rows = append(rows, chip("Resulting buffer"))
			rows = append(rows, renderBufferLines(m.TrainerValidation.ActualBuffer, inner)...)
		}
	}

	// The last two blank rows are what the old help line's top margin drew; they
	// are kept so the result screen keeps the breathing room it had above the
	// footer, which now packs through the shared component. The second of them is
	// the row nearest the legend, and it is the one the companion takes: a
	// creature drawn here reacts to the verdict in the header above it without
	// costing the screen a row, so the frame it was measured in is unchanged.
	rows = append(rows, "")
	footer := m.footerBar(inner, []installerHint{trainerHintContinue, hintBack})
	rows = append(rows, m.trainerCompanionRows(inner, len(rows), len(footer))...)
	rows = append(rows, footer...)
	return strings.Join(rows, "\n")
}

// trainerFightDuration renders a fight's length the way the countdowns render theirs:
// whole seconds, so a fast fight does not read as "39.999s" because a frame landed a
// hair early.
func trainerFightDuration(d time.Duration) string {
	seconds := int(d.Round(time.Second).Seconds())
	if seconds < 0 {
		seconds = 0
	}
	return fmt.Sprintf("%ds", seconds)
}

// trainerBossBestTime is the module's recorded best fight, or 0 when it has none. It
// reads the profile without creating a record, like every other question the trainer
// asks of it.
func (m Model) trainerBossBestTime(module trainer.ModuleID) time.Duration {
	if m.TrainerStats == nil {
		return 0
	}
	if progress := m.TrainerStats.ModuleProgressOrNil(module); progress != nil {
		return progress.BossBestTime
	}
	return 0
}

func (m Model) renderTrainerBossResult() string {
	inner := trainerInnerWidth(m)

	// Victory or defeat is the header's name, a glyph and a word, so it reads
	// without colour; the total score and the boss count are the header's vital
	// sign instead of a stacked stats line.
	name := "✗ Defeat"
	if m.TrainerLastCorrect {
		name = "✓ Victory"
	}
	vital := ""
	if m.TrainerStats != nil {
		vital = MeterStyle.Render(fmt.Sprintf("Total Score: %d · Bosses: %d/%d",
			m.TrainerStats.TotalScore, len(m.TrainerStats.BossesDefeated), len(trainer.GetAllModules())))
	}

	rows := []string{
		headerRow(name, vital, inner),
		headerRule(inner),
		chip("Summary"),
	}

	var summary []string
	state := m.TrainerGameState
	if m.TrainerLastCorrect {
		if state != nil && state.CurrentBoss != nil {
			summary = append(summary, InfoStyle.Render("You defeated "+state.CurrentBoss.Name+"!"))
			summary = append(summary, InfoStyle.Render(fmt.Sprintf("Lives remaining: %d/%d %s",
				state.BossLives, state.CurrentBoss.Lives,
				trainerLivesGlyphs(state.BossLives, state.CurrentBoss.Lives))))
		}
		summary = append(summary, SuccessStyle.Render("🎉 +500 bonus points!"))
		nextModuleExists := false
		if state != nil {
			_, nextModuleExists = trainer.NextModule(state.CurrentModule)
		}
		if nextModuleExists {
			summary = append(summary, SuccessStyle.Render("🔓 Next module unlocked!"))
		} else {
			summary = append(summary, SuccessStyle.Render("👑 No modules left — you have cleared them all!"))
		}
	} else {
		if state != nil && state.CurrentBoss != nil {
			summary = append(summary, MutedStyle.Render(state.CurrentBoss.Name+" wins this time..."))
		}
		summary = append(summary, InfoStyle.Render("Keep practicing and try again!"))
	}

	// The fight's own clock, which the state now measures: how long this fight took and
	// the module's best. Both are omitted for a fight that never started, so the row
	// appears only when there is a measurement behind it.
	if state != nil && state.CurrentBoss != nil {
		if elapsed := state.BossFightElapsed(); elapsed > 0 {
			line := fmt.Sprintf("Fight time: %s", trainerFightDuration(elapsed))
			if best := m.trainerBossBestTime(state.CurrentModule); best > 0 {
				line += fmt.Sprintf(" · Best: %s", trainerFightDuration(best))
			}
			summary = append(summary, MeterStyle.Render(line))
		}
	}
	rows = append(rows, gutteredBlock(summary)...)

	if m.TrainerMessage != "" {
		rows = append(rows, chip("Message"))
		rows = append(rows, gutteredBlock([]string{MutedStyle.Render(m.TrainerMessage)})...)
	}

	// The two blank rows are the breathing room above the legend, the same as the
	// result screen's; the one nearest the legend carries the companion, which
	// reacts to the victory or the defeat the header names.
	rows = append(rows, "")
	footer := m.footerBar(inner, []installerHint{trainerHintReturn})
	rows = append(rows, m.trainerCompanionRows(inner, len(rows), len(footer))...)
	rows = append(rows, footer...)
	return strings.Join(rows, "\n")
}
