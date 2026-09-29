package tui

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

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

// deadEnd renders a screen with nothing to show: what is missing, and the way
// out of it. The screens used to name the fact and stop ("Category not found",
// "No backup selected"), which left the reader with no next step.
func deadEnd(message, action string) string {
	return ErrorStyle.Render(message) + "\n\n" + HelpStyle.Render(action)
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
// The installer screens go through this frame, which pads their body so the
// rule above the footer lands on the same row on every one of them. The trainer
// screens cannot: they size their code window from the exact number of rows the
// chrome leaves, so they compose the same parts -- headerRow, rule, chip and
// gutteredBlock -- directly, one row per element, without placeBody's padding.
// The trainer's legends go through footerHints too, one row per packed line, so
// their keys and verbs read the same there as on every installer screen.

// installerHint is one footer entry: the keys that trigger an action and the
// verb that names it. The frame draws the keys in Accent and the verb in InkDim,
// so the eye finds the keys without reading the sentence. It is how every
// footer, the trainer's included, spells one hint.
type installerHint struct {
	keys string
	verb string
}

// The installer's hints, one per action, in the canonical order the previous
// slice fixed: navigation first, then the action, then the way back or out. A
// screen lists the actions it honours by name instead of retyping a sentence.
var (
	hintUp      = installerHint{"↑/k", "up"}
	hintDown    = installerHint{"↓/j", "down"}
	hintSelect  = installerHint{"[Enter]", "select"}
	hintBack    = installerHint{"[Esc]", "back"}
	hintCancel  = installerHint{"[Esc]", "cancel"}
	hintQuit    = installerHint{"[Space q]", "quit"}
	hintDetails = installerHint{"[Space d]", "details"}
	hintPage    = installerHint{"[PgUp/PgDn]", "page"}
	hintReturn  = installerHint{"[Enter/Space/Esc/q]", "return to menu"}
	hintRetry   = installerHint{"[r]", "retry"}
	hintStart   = installerHint{"[Enter]", "start"}
	hintExit    = installerHint{"[Enter]", "exit"}
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

// renderHint draws one hint: keys in Accent, verb in InkDim. A hint with no keys
// renders as its verb alone, with no leading space for the key that is not there.
func renderHint(h installerHint) string {
	if h.keys == "" {
		return HelpVerbStyle.Render(h.verb)
	}
	return AccentKeyStyle.Render(h.keys) + " " + HelpVerbStyle.Render(h.verb)
}

// footerHints packs hints into at most two rows within inner columns, the rows
// the frame's footer may spend. The previous slice's budget is one row when it
// fits and two at 80 columns, and it never reaches three: a screen that would
// need a third has too many hints, not a taller footer.
func footerHints(inner int, hints []installerHint) []string {
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
	if len(rows) > 2 {
		rows = rows[:2]
	}
	return rows
}

// footerRowCount is how many rows the footer spends for a screen. It is the
// same packing footerHints draws, so a screen's body budget and the rows the
// footer actually takes cannot disagree.
func footerRowCount(inner int, hints []installerHint) int {
	return len(footerHints(inner, hints))
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
func headerRow(name, vital string, inner int) string {
	left := BrandStyle.Render(name)
	if vital == "" {
		return left
	}
	gap := inner - lipgloss.Width(name) - lipgloss.Width(vital)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + vital
}

// placeBody fits a screen's body into the rows the frame leaves. The body is
// top-aligned, or centered for the welcome splash, and padded with blank rows
// so the rule above the footer lands on the same row on every screen. It does
// NOT truncate an oversized body: a screen that draws more rows than it reserved
// must fail the frame guard, not be quietly clipped here.
func placeBody(body []string, rows int, center bool) []string {
	out := make([]string, 0, rows)
	if center {
		for i := 0; i < (rows-len(body))/2; i++ {
			out = append(out, "")
		}
	}
	out = append(out, body...)
	for len(out) < rows {
		out = append(out, "")
	}
	return out
}

// frame wraps a screen body in the persistent frame. The footer is pinned to the
// bottom by padding the body region, so a short screen and a full one still put
// their header and their help on the same rows.
func (m Model) frame(name, vital string, body []string, hints []installerHint, centerBody bool) string {
	inner := contentWidth(m)
	footer := footerHints(inner, hints)
	placed := placeBody(body, installerBodyRows(m.Height, len(footer)), centerBody)

	var b strings.Builder
	b.WriteString(headerRow(name, vital, inner))
	b.WriteString("\n")
	b.WriteString(rule(inner))
	b.WriteString("\n")
	for _, line := range placed {
		b.WriteString(line)
		b.WriteString("\n")
	}
	b.WriteString(rule(inner))
	b.WriteString("\n")
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
		ScreenShellSelect, ScreenWMSelect, ScreenNvimSelect, ScreenGhosttyWarning:
		return "Setup"
	case ScreenInstalling:
		return "Installing dotfiles"
	case ScreenComplete:
		return "Installation complete"
	case ScreenError:
		return "Installation failed"
	case ScreenLearnTerminals, ScreenLearnShells, ScreenLearnWM, ScreenLearnNvim:
		return "Learn"
	case ScreenKeymaps, ScreenKeymapCategory, ScreenKeymapsMenu,
		ScreenKeymapsTmux, ScreenKeymapsTmuxCat, ScreenKeymapsZellij,
		ScreenKeymapsZellijCat, ScreenKeymapsGhostty, ScreenKeymapsGhosttyCat,
		ScreenKeymapsHerdr, ScreenKeymapsHerdrCat:
		return "Keymaps"
	case ScreenLearnLazyVim, ScreenLazyVimTopic:
		return "LazyVim"
	case ScreenBackupConfirm, ScreenRestoreBackup, ScreenRestoreConfirm:
		return "Backups"
	default:
		return "dotfiles"
	}
}

// wizardProgress is the current step and the number of steps the wizard shows
// for this run. WSL and Termux skip the terminal and font questions, so their
// counter counts the four steps they actually answer rather than the six the
// wizard's fixed chip list used to imply. A screen that is not one of the
// wizard's questions (the Ghostty warning, for instance) reports 0/0 so its
// header gets no filler meter.
func (m Model) wizardProgress() (current, total int) {
	switch m.Screen {
	case ScreenOSSelect, ScreenTerminalSelect, ScreenFontSelect,
		ScreenShellSelect, ScreenWMSelect, ScreenNvimSelect:
	default:
		return 0, 0
	}

	skipped := m.SystemInfo != nil && (m.SystemInfo.IsWSL || m.SystemInfo.IsTermux)
	total = 6
	if skipped {
		total = 4
	}
	switch m.Screen {
	case ScreenOSSelect:
		current = 1
	case ScreenTerminalSelect:
		current = 2
	case ScreenFontSelect:
		current = 3
	case ScreenShellSelect:
		if skipped {
			current = 2
		} else {
			current = 4
		}
	case ScreenWMSelect:
		if skipped {
			current = 3
		} else {
			current = 5
		}
	case ScreenNvimSelect:
		if skipped {
			current = 4
		} else {
			current = 6
		}
	}
	return current, total
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

// rowBar renders one menu row. The selected row is a full-width bar with the ▸
// in Accent and the text in Paper on BrandSoft; the unselected row is the same
// text in Ink at the SAME indent, so state comes from the bar and the weight
// rather than from an indent that moves the text. A row may carry a trailing
// meter for anything with progress.
func (m Model) rowBar(label string, selected bool, meterPlain string) string {
	inner := contentWidth(m)
	const markerWidth = 2
	meterWidth := 0
	if meterPlain != "" {
		meterWidth = 1 + lipgloss.Width(meterPlain)
	}
	textWidth := inner - markerWidth - meterWidth
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
// ▸, and a separator rendered as a frame-width rule. Every menu in the TUI goes
// through it, so the marker, the cursor style and the divider cannot drift from
// screen to screen, and the divider follows the frame instead of being the
// fixed 13-glyph string it used to be. Each option is a rowBar, so the selected
// row is a full-width bar and the unselected rows keep the same indent.
func (m Model) menuRows(options []string, cursor int) []string {
	rows := make([]string, 0, len(options))
	for i, opt := range options {
		if strings.HasPrefix(opt, menuSeparatorPrefix) {
			rows = append(rows, rule(contentWidth(m)))
			continue
		}
		rows = append(rows, m.rowBar(opt, i == cursor, ""))
	}
	return rows
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

	var s strings.Builder

	switch m.Screen {
	case ScreenWelcome:
		s.WriteString(m.renderWelcome())
	case ScreenMainMenu:
		s.WriteString(m.renderMainMenu())
	case ScreenOSSelect, ScreenTerminalSelect, ScreenFontSelect, ScreenShellSelect, ScreenWMSelect, ScreenNvimSelect, ScreenGhosttyWarning:
		s.WriteString(m.renderSelection())
	case ScreenLearnTerminals:
		s.WriteString(m.renderLearnTerminals())
	case ScreenLearnShells:
		s.WriteString(m.renderLearnShells())
	case ScreenLearnWM:
		s.WriteString(m.renderLearnWM())
	case ScreenLearnNvim:
		s.WriteString(m.renderLearnNvim())
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
	case ScreenInstalling:
		s.WriteString(m.renderInstalling())
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
		content = replaceLastLine(content, WarningStyle.Render("▶ Leader mode: [q] quit • [d] details"))
	}

	// Apply global padding (top: 1, right: 2, bottom: 0, left: 2)
	paddedStyle := lipgloss.NewStyle().Padding(1, 2, 0, 2)
	return paddedStyle.Render(content)
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

// emblemRampStyles is the vertical ramp the splash emblem is drawn in: brand
// blue at the top, the softer secondary through the middle and the accent gold
// at the tip. Every stop is an existing palette entry, so the ramp is a use of
// the palette rather than a change to it.
var emblemRampStyles = []lipgloss.Style{
	lipgloss.NewStyle().Foreground(Primary).Bold(true),
	lipgloss.NewStyle().Foreground(Secondary).Bold(true),
	lipgloss.NewStyle().Foreground(Accent).Bold(true),
}

// renderEmblem draws the emblem one row at a time with a vertical ramp through
// the palette, so it is no longer one flat colour, and returns one string per
// row so the frame can count and center it.
func renderEmblem(art string) []string {
	rows := strings.Split(strings.Trim(art, "\n"), "\n")
	out := make([]string, len(rows))
	for i, row := range rows {
		idx := i * len(emblemRampStyles) / len(rows)
		if idx >= len(emblemRampStyles) {
			idx = len(emblemRampStyles) - 1
		}
		out[i] = emblemRampStyles[idx].Render(row)
	}
	return out
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

func (m Model) renderWelcome() string {
	inner := contentWidth(m)

	// Emblem plus wordmark. Only the colouring changed: the glyphs and the
	// full/compact threshold are the measured decisions the geometry tests pin.
	var body []string
	if m.Height >= welcomeFullLockupHeight {
		body = append(body, renderEmblem(logo)...)
		body = append(body, "")
		body = append(body, renderWordmark(dotfilesText)...)
	} else {
		body = append(body, renderEmblem(compactLogo)...)
		body = append(body, "")
		body = append(body, BrandStyle.Render("dotfiles"))
	}

	// The version and the detected environment are one dim line under the
	// wordmark now, instead of a sentence above the help: a machine fact belongs
	// with the lockup, not between the pitch and its keys. The platform leads
	// because it is the fact that changes how the installer behaves.
	env := "Running on " + m.SystemInfo.OSName
	if m.SystemInfo.IsWSL && m.SystemInfo.OSName != "WSL" {
		env += " under WSL"
	}
	if m.SystemInfo.HasBrew {
		env += ", with Homebrew already installed"
	}
	env += " (" + VersionLabel() + ")"
	body = append(body, MeterStyle.Render(truncate(env, inner)))

	body = append(body, "")
	body = append(body, SubtitleStyle.Render("Your terminal environment, configured in minutes."))

	// Center the splash horizontally within the frame and vertically in the rows
	// the frame leaves, so the last row of a full-height screen still lands on
	// the terminal's last row instead of one past it.
	centered := make([]string, len(body))
	for i, line := range body {
		centered[i] = CenterHorizontally(line, inner)
	}
	return m.frame(m.headerName(), "", centered, []installerHint{hintStart, hintQuit}, true)
}

func (m Model) renderMainMenu() string {
	// Title. The toolbox emoji that used to open it was the one thing on the
	// first screen that a terminal without an emoji font drew as a box.
	body := []string{
		BrandStyle.Render("dotfiles"),
		MutedStyle.Render("What would you like to do?"),
		"",
	}
	body = append(body, m.menuRows(m.GetCurrentOptions(), m.Cursor)...)
	return m.frame(m.headerName(), "", body, []installerHint{hintUp, hintDown, hintSelect, hintQuit}, false)
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

func (m Model) renderSelection() string {
	body := []string{BrandStyle.Render(stripStepPrefix(m.GetScreenTitle()))}

	// The description can be several sentences (the WSL terminal note is the
	// longest) and used to be left to the frame edge, which clipped it mid-word.
	for _, line := range strings.Split(m.GetScreenDescription(), "\n") {
		for _, row := range wrapText(line, contentWidth(m), 0) {
			body = append(body, MutedStyle.Render(row))
		}
	}
	body = append(body, "")
	body = append(body, m.menuRows(m.GetCurrentOptions(), m.Cursor)...)

	// The step counter is the wizard's vital sign, and it lives in the header
	// now instead of costing the body a chip row above the title.
	vital := ""
	if current, total := m.wizardProgress(); total > 0 {
		vital = meterRendered(current, total, 10)
	}
	return m.frame(m.headerName(), vital, body, []installerHint{hintUp, hintDown, hintSelect, hintBack}, false)
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
		return deadEnd("Tool not found", "Press [Esc] to go back.")
	}

	return m.renderSingleToolInfo(info)
}

// toolInfoFixedRows is the part of the tool-info body that is not a list: the
// title, the pros chip, the cons chip and the blank rows around them. The
// description and the website add one row each and are counted at render time,
// so a description that wraps to two rows cannot push the legend off screen.
const toolInfoFixedRows = 5

func (m Model) renderSingleToolInfo(info ToolInfo) string {
	width := contentWidth(m)
	hints := []installerHint{hintBack, hintQuit}
	bodyRows := installerBodyRows(m.Height, footerRowCount(width, hints))

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
	// stays brand chrome and neither green nor amber is used as decoration.
	body = append(body, SuccessStyle.Render("✓")+" "+chip("Pros"))
	body = append(body, gutteredBlock(listRows(info.Pros, "• ", prosRows, width-2, InfoStyle))...)
	body = append(body, "")
	body = append(body, WarningStyle.Render("✗")+" "+chip("Cons"))
	body = append(body, gutteredBlock(listRows(info.Cons, "• ", consRows, width-2, MutedStyle))...)

	return m.frame(m.headerName(), "", body, hints, false)
}

// keymapTableBodyFixed is the part of a keymap table's body that is not a
// binding row: the title, the description, the blank under it, the column
// header and the rule. The table's scroll range moved into the header, so the
// body no longer spends a row on it.
const keymapTableBodyFixed = 5

// keymapTableMinRows keeps the table readable when the frame is at its floor.
const keymapTableMinRows = 3

// keymapTableRows is how many keymap rows the frame leaves. The view and the
// scroll keys both ask for it, so the keys scroll exactly the rows the screen
// draws; the five tables used to compute their own window against m.Height-9,
// which is what let a 15-row window overflow a 24-row frame. It reserves a
// one-row footer, which is what the navigate-and-return hints take at the
// documented 80-column floor.
func keymapTableRows(height int) int {
	rows := installerBodyRows(height, 1) - keymapTableBodyFixed
	if rows < keymapTableMinRows {
		rows = keymapTableMinRows
	}
	return rows
}

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

// renderKeymapTable renders one keymap category: its title, description, a
// column header and the rows the frame has room for. Its scroll value IS the
// top-row offset (0 shows the first binding, offsetWindowMax shows the last), so
// it windows through offsetWindow and the category key handlers clamp that same
// value with offsetWindowMax; the last binding is reachable. It replaces five
// copies of the same table that had each picked their own column widths, their
// own window and their own 60-column rule.
func (m Model) renderKeymapTable(category KeymapCategory, scroll int) string {
	width := contentWidth(m)
	longestKey := 0
	for _, km := range category.Keymaps {
		if w := lipgloss.Width(km.Keys); w > longestKey {
			longestKey = w
		}
	}
	keys, mode, description := keymapTableColumns(width, longestKey)

	rows := keymapTableRows(m.Height)
	start, end := offsetWindow(scroll, rows, len(category.Keymaps))

	body := []string{
		BrandStyle.Render(category.Name),
		MutedStyle.Render(truncate(category.Description, width)),
		"",
	}
	header := padRight("Keys", keys) + " " + padRight("Mode", mode) + " Description"
	body = append(body, MutedStyle.Render(truncate(header, width)))
	body = append(body, rule(width))
	for i := start; i < end; i++ {
		km := category.Keymaps[i]
		body = append(body,
			KeyStyle.Render(padRight(truncate(km.Keys, keys), keys))+" "+
				MutedStyle.Render(padRight(truncate(km.Mode, mode), mode))+" "+
				InfoStyle.Render(truncate(km.Description, description)))
	}

	// The scroll range is the table's vital sign, so it lives in the header and
	// costs the body no row: the old body held a scroll row even when the list
	// fit, and reading it back out of the header still satisfies the scroll
	// reachability tests.
	vital := ""
	if len(category.Keymaps) > rows {
		vital = scrollVital("Showing", start+1, end, len(category.Keymaps))
	}
	return m.frame(m.headerName(), vital, body, []installerHint{hintUp, hintDown, hintReturn}, false)
}

// menuBodyFixed is the part of a menu's body that is not an option row: the
// title, the description and the blank under it. The scroll range moved into the
// header, so the body no longer spends a held scroll row on it.
const menuBodyFixed = 3

// renderMenu renders the menus of the keymaps and learn sections. They differ
// only in their description, so the title, the list and the legend are shared:
// six copies of the same menu used to spell the legend three different ways. The
// list is windowed around the cursor, so a section with more categories than the
// frame has rows scrolls instead of overflowing.
func (m Model) renderMenu(description string) string {
	rows := m.menuRows(m.GetCurrentOptions(), m.Cursor)
	hints := []installerHint{hintUp, hintDown, hintSelect, hintBack}

	visible := installerBodyRows(m.Height, footerRowCount(contentWidth(m), hints)) - menuBodyFixed
	if visible < 1 {
		visible = 1
	}
	start, end := listWindow(m.Cursor, visible, len(rows))

	body := []string{
		BrandStyle.Render(m.GetScreenTitle()),
		MutedStyle.Render(description),
		"",
	}
	body = append(body, rows[start:end]...)

	// The scroll range is the menu's vital sign in the header. A menu whose list
	// fits has nothing to report and gets no filler.
	vital := ""
	if len(rows) > visible {
		vital = scrollVital("Showing", start+1, end, len(rows))
	}
	return m.frame(m.headerName(), vital, body, hints, false)
}

func (m Model) renderKeymapsMenu() string {
	return m.renderMenu("Select a category to view keybindings")
}

func (m Model) renderKeymapCategory() string {
	if m.SelectedCategory >= len(m.KeymapCategories) {
		return deadEnd("Category not found", "Press [Esc] to pick another category.")
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
		return deadEnd("Category not found", "Press [Esc] to pick another category.")
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
		return deadEnd("Category not found", "Press [Esc] to pick another category.")
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
		return deadEnd("Category not found", "Press [Esc] to pick another category.")
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
		return deadEnd("Category not found", "Press [Esc] to pick another category.")
	}
	return m.renderKeymapTable(m.HerdrKeymapCategories[m.HerdrSelectedCategory], m.HerdrKeymapScroll)
}

func (m Model) renderLazyVimMenu() string {
	return m.renderMenu("Learn how to use and customize LazyVim")
}

// lazyVimTopicLines is the topic's content exactly as the screen draws it: the
// body, the code example and the tips, each line cut to the frame. The view
// draws these lines and the scroll keys measure them, so the two cannot disagree
// about how far the topic scrolls. It was the scroll keys' own, different count
// that let the closing lines sit behind a bound the renderer never reached.
func (m Model) lazyVimTopicLines(topic LazyVimTopic) []string {
	width := contentWidth(m)

	var allLines []string
	for _, line := range topic.Content {
		allLines = append(allLines, truncate(line, width))
	}
	allLines = append(allLines, "") // Empty line

	if topic.CodeExample != "" {
		allLines = append(allLines, "📝 Example:", "")
		for _, line := range strings.Split(topic.CodeExample, "\n") {
			allLines = append(allLines, truncate(line, width))
		}
		allLines = append(allLines, "") // Empty line
	}

	if len(topic.Tips) > 0 {
		allLines = append(allLines, "💡 Tips:")
		for _, tip := range topic.Tips {
			allLines = append(allLines, truncate("  • "+tip, width))
		}
	}

	return allLines
}

// lazyVimTopicBodyFixed is the part of the topic body that is not content: the
// title, the description and the blank under it. The scroll range moved into the
// header, so the body no longer spends a held scroll row on it.
const lazyVimTopicBodyFixed = 3

// lazyVimTopicRows is the content rows the topic shows: the frame minus the
// screen's own chrome. The view and the scroll keys both ask for it, so the
// window the keys scroll is exactly the window the screen draws. It reserves a
// one-row footer, which is what the navigate-page-return hints take at 80
// columns.
func lazyVimTopicRows(height int) int {
	rows := installerBodyRows(height, 1) - lazyVimTopicBodyFixed
	if rows < 1 {
		rows = 1
	}
	return rows
}

// renderLazyVimTopic renders one topic. Its scroll value IS the top-line offset
// (0 shows the first line, offsetWindowMax shows the last), so it windows through
// offsetWindow and its key handler clamps the same value with offsetWindowMax;
// the topic's closing lines are reachable.
func (m Model) renderLazyVimTopic() string {
	if m.SelectedLazyVimTopic >= len(m.LazyVimTopics) {
		return deadEnd("Topic not found", "Press [Esc] to return to the guide.")
	}

	topic := m.LazyVimTopics[m.SelectedLazyVimTopic]
	width := contentWidth(m)

	allLines := m.lazyVimTopicLines(topic)
	viewHeight := lazyVimTopicRows(m.Height)
	start, end := offsetWindow(m.LazyVimScroll, viewHeight, len(allLines))

	body := []string{
		BrandStyle.Render(topic.Title),
		MutedStyle.Render(truncate(topic.Description, width)),
		"",
	}

	for i := start; i < end; i++ {
		line := allLines[i]
		// Style code lines differently
		switch {
		case strings.HasPrefix(line, "--"), strings.HasPrefix(line, "local"),
			strings.HasPrefix(line, "return"), strings.HasPrefix(line, "{"),
			strings.HasPrefix(line, "}"), strings.HasPrefix(line, "  "),
			strings.HasPrefix(line, "map("), strings.HasPrefix(line, "vim."),
			strings.HasPrefix(line, "require"):
			body = append(body, CodeStyle.Render(line))
		case strings.HasPrefix(line, "📝"), strings.HasPrefix(line, "💡"):
			body = append(body, chip(line))
		case strings.HasPrefix(line, "  •"):
			body = append(body, InfoStyle.Render(line))
		case strings.HasPrefix(line, "•"):
			body = append(body, MutedStyle.Render(line))
		default:
			body = append(body, InfoStyle.Render(line))
		}
	}

	// The scroll range is the topic's vital sign in the header, so the body does
	// not need a held scroll row; the scroll reachability tests read the range
	// back out of the header.
	vital := ""
	if len(allLines) > viewHeight {
		vital = scrollVital("Lines", start+1, end, len(allLines))
	}
	return m.frame(m.headerName(), vital, body, []installerHint{hintUp, hintDown, hintPage, hintReturn}, false)
}

// Installing screen rows. The bar is the one place the whole run's progress is
// visible at a glance, and the step rail is windowed so a run with fifteen steps
// cannot push its bottom off a 24-row terminal.
const (
	// installingBodyFixed is the part of the installing body that is not the step
	// rail: the progress bar and the blank under it. The title moved into the
	// frame's header, and the legend into its footer.
	installingBodyFixed = 2
	// installingDetailsRows is the whole height the log box adds: the blank above
	// it, the box's border and padding around three log rows, and the blank below
	// it.
	installingDetailsRows = 9
	// installingDetailsLogLines is how many log rows the box shows. Three is a
	// deliberate trade, not an oversight: the box is a fixed height so turning
	// details on cannot push the step rail or the legend off a 24-row frame, and a
	// fixed box cannot also grow with the terminal. The newest three lines are the
	// useful end of an installer log, and the final frame guard renders the screen
	// with twelve lines queued to prove the box stays three rows tall. Raising it is
	// a frame-budget change, not a one-line one.
	installingDetailsLogLines = 3
	// installingMinStepRows keeps the rail useful when the log box has taken its
	// room.
	installingMinStepRows = 3
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
// text glyphs (█ and ░) rather than a background colour, so the bar still reads
// on a 16-colour terminal and in a terminal with no colour at all -- the same
// reason the step rail's glyphs carry its state.
func renderProgressBar(width int, progress float64) string {
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
	return ProgressBarFilled.Render(strings.Repeat("█", filled)) +
		ProgressBarEmpty.Render(strings.Repeat("░", width-filled))
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

// installingStepRows is how many rows the step rail may use: the frame minus the
// screen's own fixed rows and, when they are on, the log box.
func (m Model) installingStepRows() int {
	fixed := installingBodyFixed
	if m.ShowDetails && len(m.LogLines) > 0 {
		fixed += installingDetailsRows
	}
	rows := installerBodyRows(m.Height, footerRowCount(contentWidth(m), []installerHint{hintDetails})) - fixed
	if rows < installingMinStepRows {
		rows = installingMinStepRows
	}
	return rows
}

// installingVital is the installing screen's header vital sign: which step of
// the run is in progress, as the same bar-and-count the wizard uses at the top
// of the flow.
func (m Model) installingVital() string {
	total := len(m.Steps)
	if total == 0 {
		return ""
	}
	current := m.CurrentStep + 1
	if current > total {
		current = total
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

// railRow draws one step-rail row with the trailing meter the row's own
// progress earns: the glyph and name on the left, the filled meter on the right.
// It is the row-level use of the same meter the header uses for the run.
func railRow(name string, style lipgloss.Style, fraction float64, cells, inner int) string {
	bar := meterBar(fraction, cells)
	textWidth := inner - cells - 2
	if textWidth < 1 {
		textWidth = 1
	}
	name = truncate(name, textWidth)
	pad := textWidth - lipgloss.Width(name)
	return style.Render(name+strings.Repeat(" ", pad)) + "  " + bar
}

func (m Model) renderInstalling() string {
	width := contentWidth(m)

	// Progress bar. It is sized to the frame and labeled with the percentage, so
	// the longest thing a user watches says how far along it is. The screen's
	// title moved into the frame's header, and the details legend into its footer.
	barWidth := width - 8
	if barWidth < 10 {
		barWidth = 10
	}
	progress := m.installProgress()
	body := []string{
		renderProgressBar(barWidth, progress) + MutedStyle.Render(fmt.Sprintf(" %3.0f%%", progress*100)),
		"",
	}

	// Step rail. Each step is one row and the running step's description is one
	// more, and the whole rail is windowed around the running step so a long run
	// keeps the step in progress on screen. Each row carries its own trailing
	// meter, so a step's progress is visible on the row rather than only in the
	// run's single bar.
	const railMeterCells = 10
	rows := make([]string, 0, len(m.Steps)+1)
	runningIdx := 0
	for i, step := range m.Steps {
		icon, style := stepGlyph(step)
		if i == m.CurrentStep {
			runningIdx = len(rows)
		}
		rows = append(rows, railRow(fmt.Sprintf("%s %s", icon, step.Name), style, stepFraction(step), railMeterCells, width))
		if i == m.CurrentStep && step.Status == StatusRunning && step.Description != "" {
			rows = append(rows, MutedStyle.Render("   "+truncate(step.Description, width-4)))
		}
	}

	start, end := listWindow(runningIdx, m.installingStepRows(), len(rows))
	body = append(body, rows[start:end]...)

	// Log output when details are on. The box is a fixed height and its lines are
	// cut to the frame, so turning details on cannot push the rail or the footer
	// off screen. Its rendered rows are split back out so the frame counts every
	// row of the box, not the box as one row.
	if m.ShowDetails && len(m.LogLines) > 0 {
		body = append(body, "")
		logs := m.LogLines[max(0, len(m.LogLines)-installingDetailsLogLines):]
		lines := make([]string, 0, len(logs))
		for _, line := range logs {
			lines = append(lines, truncate(line, width-4))
		}
		body = append(body, strings.Split(BoxStyle.Render(strings.Join(lines, "\n")), "\n")...)
		body = append(body, "")
	}

	return m.frame(m.headerName(), m.installingVital(), body, []installerHint{hintDetails}, false)
}

func (m Model) renderComplete() string {
	width := contentWidth(m)

	// The screen's name is the frame's header now, so the body opens on the first
	// block instead of repeating it as a second label row.
	body := []string{chip("Summary")}
	items := []string{
		fmt.Sprintf("OS: %s", m.Choices.OS),
		fmt.Sprintf("Terminal: %s", m.Choices.Terminal),
		fmt.Sprintf("Shell: %s", m.Choices.Shell),
		fmt.Sprintf("Window Manager: %s", m.Choices.WindowMgr),
	}
	if m.Choices.InstallFont {
		items = append(items, "Font: Iosevka Term Nerd Font")
	}
	if m.Choices.InstallNvim {
		items = append(items, "Editor: Neovim with dotfiles config")
	}
	rendered := make([]string, len(items))
	for i, item := range items {
		rendered[i] = InfoStyle.Render(truncate(item, width-2))
	}
	body = append(body, gutteredBlock(rendered)...)
	body = append(body, "")

	// Next Step block.
	shell := m.Choices.Shell
	shellCmd := shell
	if shell == "nushell" {
		shellCmd = "nu"
	}
	body = append(body, chip("Next Step"))
	body = append(body, gutteredBlock([]string{
		InfoStyle.Render("To use your new shell now, run:"),
		HighlightStyle.Render(fmt.Sprintf("exec %s", shellCmd)),
	})...)

	return m.frame(m.headerName(), "", body, []installerHint{hintExit}, false)
}

func (m Model) renderError() string {
	width := contentWidth(m)

	// The screen's name is the frame's header now, so the body opens on the failure
	// block instead of repeating it as a second label row.
	body := []string{chip("Error")}

	// The message is wrapped and capped so a long failure cannot run off the
	// bottom of the screen; the last row ends with the cut marker.
	errLines := make([]string, 0, 4)
	for _, row := range wrapText(m.ErrorMsg, width-2, 4) {
		errLines = append(errLines, ErrorStyle.Render(row))
	}
	body = append(body, gutteredBlock(errLines)...)

	// Show last few log lines for context.
	if len(m.LogLines) > 0 {
		body = append(body, "")
		body = append(body, chip("Recent logs"))
		// Show last 5 log lines, cut to the frame so a long line is marked rather
		// than clipped at the edge.
		startIdx := len(m.LogLines) - 5
		if startIdx < 0 {
			startIdx = 0
		}
		logLines := make([]string, 0, len(m.LogLines)-startIdx)
		for _, line := range m.LogLines[startIdx:] {
			logLines = append(logLines, InfoStyle.Render(truncate(line, width-2)))
		}
		body = append(body, gutteredBlock(logLines)...)
	}

	return m.frame(m.headerName(), "", body, []installerHint{hintRetry, hintQuit}, false)
}

// backupConfirmFixed is the rows the backup-confirmation body spends around the
// config list: the title, the description, a blank, a blank, the note, a blank
// and the options. The list gets what is left, so a machine with every config
// path present cannot push the options off the bottom of the screen.
const backupConfirmFixed = 6

func (m Model) renderBackupConfirm() string {
	width := contentWidth(m)
	hints := []installerHint{hintUp, hintDown, hintSelect, hintBack}
	options := m.GetCurrentOptions()
	bodyRows := installerBodyRows(m.Height, footerRowCount(width, hints))

	// The config list is bounded to the frame: a machine with all sixteen config
	// paths present used to push the options off the bottom of the screen.
	configRows := bodyRows - backupConfirmFixed - len(options)
	if configRows < 1 {
		configRows = 1
	}

	body := []string{
		BrandStyle.Render(m.GetScreenTitle()),
		MutedStyle.Render("The following configs will be overwritten:"),
		"",
	}
	body = append(body, listRows(m.ExistingConfigs, "  ⚠️ ", configRows, width, WarningStyle)...)
	body = append(body, "")
	body = append(body, InfoStyle.Render("Creating a backup allows you to restore later if needed."))
	body = append(body, "")
	body = append(body, m.menuRows(options, m.Cursor)...)

	return m.frame(m.headerName(), "", body, hints, false)
}

// restoreBackupBodyFixed is the part of the restore list's body that is not a
// backup row: the title, the description, a blank, the rule above Back and the
// Back row.
const restoreBackupBodyFixed = 5

func (m Model) renderRestoreBackup() string {
	width := contentWidth(m)
	hints := []installerHint{hintUp, hintDown, hintSelect, hintBack}
	bodyRows := installerBodyRows(m.Height, footerRowCount(width, hints))

	// The list is bounded and follows the cursor, so a long history scrolls
	// instead of pushing the Back row off the frame.
	listBudget := bodyRows - restoreBackupBodyFixed
	if listBudget < 1 {
		listBudget = 1
	}

	body := []string{
		BrandStyle.Render(m.GetScreenTitle()),
		MutedStyle.Render("Select a backup to restore or delete"),
		"",
	}

	start, end := 0, 0
	if len(m.AvailableBackups) == 0 {
		body = append(body, MutedStyle.Render("No backups found."))
	} else {
		start, end = listWindow(m.Cursor, listBudget, len(m.AvailableBackups))
		for i := start; i < end; i++ {
			backup := m.AvailableBackups[i]
			label := fmt.Sprintf("📁 %s (%d items)", backup.Timestamp.Format("2006-01-02 15:04:05"), len(backup.Files))
			body = append(body, m.rowBar(label, i == m.Cursor, ""))
		}
	}

	// Separator and Back. The separator is the shared frame-width rule, not the
	// fixed 13-glyph string that disagreed with every other divider.
	body = append(body, rule(width))

	backIdx := len(m.AvailableBackups) + 1
	body = append(body, m.rowBar("← Back", m.Cursor == backIdx, ""))

	// The scroll range is the list's vital sign in the header, so the body does
	// not hold a scroll row that would move the Back row as the history grows.
	vital := ""
	if len(m.AvailableBackups) > listBudget {
		vital = scrollVital("Showing", start+1, end, len(m.AvailableBackups))
	}
	return m.frame(m.headerName(), vital, body, hints, false)
}

// restoreConfirmBodyFixed is the part of the restore confirmation's body that is
// not a file row: the title, the description, a blank, the Contents chip, a
// blank, the warning and a blank.
const restoreConfirmBodyFixed = 7

func (m Model) renderRestoreConfirm() string {
	if m.SelectedBackup >= len(m.AvailableBackups) {
		return deadEnd("No backup selected", "Press [Esc] to go back.")
	}

	backup := m.AvailableBackups[m.SelectedBackup]
	width := contentWidth(m)
	hints := []installerHint{hintUp, hintDown, hintSelect, hintCancel}
	options := m.GetCurrentOptions()
	bodyRows := installerBodyRows(m.Height, footerRowCount(width, hints))

	// List files in backup, bounded to the frame so a backup with a long file
	// list cannot push the options off the bottom of the screen.
	fileRows := bodyRows - restoreConfirmBodyFixed - len(options)
	if fileRows < 1 {
		fileRows = 1
	}

	body := []string{
		BrandStyle.Render(m.GetScreenTitle()),
		MutedStyle.Render("Backup from: " + backup.Timestamp.Format("2006-01-02 15:04:05")),
		"",
		chip("Contents"),
	}
	body = append(body, gutteredBlock(listRows(backup.Files, "• ", fileRows, width-2, InfoStyle))...)
	body = append(body, "")
	body = append(body, WarningStyle.Render("⚠️ Restoring will overwrite your current configs!"))
	body = append(body, "")
	body = append(body, m.menuRows(options, m.Cursor)...)

	return m.frame(m.headerName(), "", body, hints, false)
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

	trainerHintTypeCommand = installerHint{"", "Type command"}
	trainerHintSubmit      = installerHint{"[Enter]", "submit"}
	trainerHintHint        = installerHint{"[Tab]", "hint"}
	trainerHintScroll      = installerHint{"[PgUp/PgDn]", "scroll"}
	trainerHintDelete      = installerHint{"[Backspace]", "delete"}
	trainerHintEscToken    = installerHint{"[Ctrl-e]", "type " + trainer.EscToken}
	trainerHintBlockVisual = installerHint{"[Ctrl-v]", "block"}
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

	return fmt.Sprintf("Lessons %s %d/%d Mastered %s %d/%d",
		meterCellsPlain(lessonsFraction, trainerRowMeterCells), lessonsCompleted, lessonsTotal,
		meterCellsPlain(masteryFraction, trainerRowMeterCells), practice.MasteredCount, practice.TotalExercises)
}

// trainerPracticeAccuracyText renders the selected module's practice accuracy,
// the percentage the boss unlock threshold is measured against. It is omitted
// until the module has a recorded attempt, so a fresh module does not claim 0%.
// It names itself because it is now a line of the selected module's detail
// block rather than a suffix on the row, where "Acc 60%" had to be terse.
func trainerPracticeAccuracyText(progress *trainer.ModuleProgress) string {
	if progress == nil || progress.PracticeAttempts == 0 {
		return ""
	}
	return fmt.Sprintf("Practice accuracy: %.0f%%", progress.PracticeAccuracy*100)
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
		MeterStyle.Render(fmt.Sprintf("  Score: %d · Streak: %d · Bosses: %d/%d",
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
		headerRow("🎮 Vim Mastery Trainer", m.trainerMenuVital(), inner),
		rule(inner),
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
		label := fmt.Sprintf("%s %s %s", status, module.Icon, module.Name)

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

		rows = append(rows, m.rowBar(label, i == m.TrainerCursor, meter))

		// The selected module's detail block: its commands, the accuracy the row
		// no longer carries, and the exercises it misses most. All three are
		// display-only lines attached to the entry and all three sit behind one
		// gutter.
		if i == m.TrainerCursor {
			detail := []string{InkStyle.Render(truncate(module.Description, inner-blockGutterWidth))}
			if accuracy := trainerPracticeAccuracyText(progress); accuracy != "" {
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
	rows = append(rows, footerHints(inner, []installerHint{
		hintUp, hintDown,
		trainerHintLesson, trainerHintPractice, trainerHintBoss,
		trainerHintReset, trainerHintResetAll, trainerHintBack,
	})...)

	return strings.Join(rows, "\n")
}

// trainerTextBudget splits the rows the chrome leaves (trainerFrameRows)
// between the mission text, which cannot be scrolled, and the code window, which
// can. The renderer and the scroll key both ask for it, so the window the keys
// scroll is exactly the window the screen draws.
func (m Model) trainerTextBudget(exercise *trainer.Exercise) (missionRows []string, codeRows int) {
	flex := m.Height - trainerFrameRows
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
	rows := make([]string, trainerMessageRows)
	for i, row := range wrapText(m.TrainerMessage, trainerInnerWidth(m), trainerMessageRows) {
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
	parts = append(parts, MeterStyle.Render(fmt.Sprintf("Score: %d · Streak: %d", state.SessionScore, state.CurrentStreak)))
	if remaining := state.RemainingSeconds(); remaining > 0 {
		parts = append(parts, WarningStyle.Render(fmt.Sprintf("⏳ Hint in %ds", int(math.Ceil(remaining)))))
	}
	return strings.Join(parts, "  ")
}

// renderTrainerExercise renders a lesson or practice exercise. The mission, the
// code window and the answer are guttered blocks under brand chips; the header
// carries the mode, the position, the score, the streak and the countdown that
// used to be four stacked rows. The code is a window because the screen has to
// fit the 24-row terminal the trainer claims: see trainerFrameRows for the
// arithmetic.
func (m Model) renderTrainerExercise(mode string) string {
	exercise := m.trainerCurrentExercise()
	if exercise == nil {
		return deadEnd("No exercise loaded", "Press [Esc] to return to the trainer.")
	}

	inner := trainerInnerWidth(m)
	mission, codeRows := m.trainerTextBudget(exercise)
	viewport := m.trainerCodeViewport(exercise, codeRows, inner-blockGutterWidth-trainerGutterWidth)

	// Every element below is one terminal row, which is what makes the code
	// window's height the frame minus the chrome instead of an estimate: the
	// screen used to render 22+N rows and lose its bottom.
	rows := []string{
		headerRow(fmt.Sprintf("🎮 %s · %s", mode, string(m.TrainerGameState.CurrentModule)), m.trainerExerciseVital(), inner),
		rule(inner),
		chip("Mission"),
	}
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
	rows = append(rows, "")
	rows = append(rows, footerHints(inner, []installerHint{
		trainerHintTypeCommand, trainerHintSubmit, trainerHintHint, trainerHintScroll,
		trainerHintDelete, trainerHintEscToken, trainerHintBlockVisual, trainerHintQuit,
	})...)

	return strings.Join(rows, "\n")
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
	if remaining := state.BossStepSecondsLeft(); remaining > 0 {
		parts = append(parts, chip(fmt.Sprintf("⏳ Time left: %ds", int(math.Ceil(remaining)))))
	}
	return strings.Join(parts, "  ")
}

func (m Model) renderTrainerBoss() string {
	state := m.TrainerGameState
	if state == nil || state.CurrentBoss == nil {
		return deadEnd("No boss loaded", "Press [Esc] to return to the trainer.")
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
		rule(inner),
		chip("Challenge"),
	}

	if state.BossStep >= len(boss.Steps) || exercise == nil {
		// No step is on screen, so there is no code window to size around. The
		// legend packs through the shared footer component like every other.
		rows = append(rows, m.trainerFeedbackRows(WarningStyle)...)
		rows = append(rows, "")
		rows = append(rows, footerHints(inner, []installerHint{
			trainerHintTypeCommand, trainerHintSubmit, trainerHintEscToken,
			trainerHintScroll, trainerHintForfeit, trainerHintBlockVisual,
		})...)
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
	rows = append(rows, "")
	rows = append(rows, footerHints(inner, []installerHint{
		trainerHintTypeCommand, trainerHintSubmit, trainerHintEscToken,
		trainerHintScroll, trainerHintForfeit, trainerHintBlockVisual,
	})...)

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
		rule(inner),
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
	// footer, which now packs through the shared component.
	rows = append(rows, "", "")
	rows = append(rows, footerHints(inner, []installerHint{trainerHintContinue, hintBack})...)
	return strings.Join(rows, "\n")
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
		rule(inner),
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
	rows = append(rows, gutteredBlock(summary)...)

	if m.TrainerMessage != "" {
		rows = append(rows, chip("Message"))
		rows = append(rows, gutteredBlock([]string{MutedStyle.Render(m.TrainerMessage)})...)
	}

	rows = append(rows, "", "")
	rows = append(rows, footerHints(inner, []installerHint{hintReturn})...)
	return strings.Join(rows, "\n")
}
