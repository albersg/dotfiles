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
	return result
}

// The key hints the installer's screens share. Twelve screens used to spell the
// same four concepts seven different ways ([Space q], [space+q], [q/Esc],
// [Esc/q], [Enter/Esc/q], space+d and q=quit), which made the help line the most
// visible sloppiness in the TUI. One notation and one order is the fix: every
// key is bracketed, and a screen lists its hints navigation first, then the
// action, then the way back or out.
//
// Only the hints a screen actually honours live here. The installing screen,
// for instance, does not offer [Space q] because the leader's q is deliberately
// disabled mid-install, and the tool-info screen does not offer the navigation
// pair because its cursor moves nothing a reader can see.
const (
	helpNavigate = "↑/k up • ↓/j down"
	helpSelect   = "[Enter] select"
	helpBack     = "[Esc] back"
	helpCancel   = "[Esc] cancel"
	helpQuit     = "[Space q] quit"
	helpDetails  = "[Space d] details"

	// helpReturnToMenu is the trainer's boss-result legend. It is one fragment
	// because the four keys all do the same thing there, and it goes through the
	// shared notation like every other legend.
	helpReturnToMenu = "[Enter/Space/Esc/q] return to menu"
)

// helpNotation joins a screen's key hints with the one separator and the one
// order the program shares, so a legend is a list of named fragments instead of
// a hand-typed sentence that drifts from its neighbours. It is the notation
// itself; helpLine and trainerHelpLine differ only in the style around it.
func helpNotation(hints ...string) string {
	return strings.Join(hints, " • ")
}

// helpLine renders a screen's key hints with the one separator and the one
// order they share.
func helpLine(hints ...string) string {
	return HelpStyle.Render(helpNotation(hints...))
}

// trainerHelpLine is helpLine without the margin: the trainer budgets one row
// per element, so it draws the same notation through TrainerHelpStyle, which
// carries no margin of its own.
func trainerHelpLine(hints ...string) string {
	return TrainerHelpStyle.Render(helpNotation(hints...))
}

// deadEnd renders a screen with nothing to show: what is missing, and the way
// out of it. The screens used to name the fact and stop ("Category not found",
// "No backup selected"), which left the reader with no next step.
func deadEnd(message, action string) string {
	return ErrorStyle.Render(message) + "\n\n" + HelpStyle.Render(action)
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
// selected visible. Lists sized to their frame use it so a long list scrolls
// instead of running off the bottom of the terminal, and so the entry under the
// cursor is always one of the rows on screen.
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

// menuRows renders a menu's options: one row per choice, the cursor marked with
// ▸, and a separator rendered as a frame-width rule. Every menu in the TUI goes
// through it, so the marker, the cursor style and the divider cannot drift from
// screen to screen, and the divider follows the frame instead of being the
// fixed 13-glyph string it used to be.
func (m Model) menuRows(options []string, cursor int) []string {
	rows := make([]string, 0, len(options))
	for i, opt := range options {
		if strings.HasPrefix(opt, menuSeparatorPrefix) {
			rows = append(rows, rule(contentWidth(m)))
			continue
		}
		marker, style := "  ", UnselectedStyle
		if i == cursor {
			marker, style = "▸ ", SelectedStyle
		}
		rows = append(rows, style.Render(marker+opt))
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

// The full welcome lockup is 30 lines: emblem, wordmark and three text lines.
// CenterBoth places content taller than the frame by overflowing it, which
// clips the top of the emblem, so a terminal shorter than the lockup gets the
// version without the wordmark instead. The wordmark is the part that is
// dropped because it repeats what the emblem already says.
// 32 is the smallest frame that holds the 30-line lockup plus the splash's
// one-row top padding: at 30 or 31 the last line falls outside the frame and
// CenterBoth clips the top of the emblem. Measured, not estimated:
// TestWelcomeLockupFitsWhenFullEmblemIsChosen renders both branches and fails
// if this constant stops matching the art.
const welcomeFullLockupHeight = 32

func (m Model) renderWelcome() string {
	var s strings.Builder

	// Logo
	if m.Height >= welcomeFullLockupHeight {
		s.WriteString(LogoStyle.Render(logo))
		s.WriteString("\n")
		s.WriteString(TitleStyle.Render(dotfilesText))
	} else {
		s.WriteString(LogoStyle.Render(compactLogo))
		s.WriteString("\n")
		s.WriteString(TitleStyle.Render("dotfiles"))
	}
	s.WriteString("\n\n")

	// Tagline. The detected environment used to sit between the logo and this
	// line, which put a machine fact in the middle of the pitch; it is a footnote
	// now, dim and below the help.
	s.WriteString(SubtitleStyle.Render("Your terminal environment, configured in minutes."))
	s.WriteString("\n\n")
	s.WriteString(helpLine("[Enter] start", helpQuit))

	// The detected environment is a footnote because it is what a bug report
	// needs and what a first-time reader does not: it names the platform, whether
	// Homebrew is already present, and the build version. It used to bolt the
	// three together with pipe separators, which read as three unrelated facts;
	// it is one sentence now, and the platform leads because it is the fact that
	// changes how the installer behaves.
	env := "Running on " + m.SystemInfo.OSName
	if m.SystemInfo.IsWSL && m.SystemInfo.OSName != "WSL" {
		env += " under WSL"
	}
	if m.SystemInfo.HasBrew {
		env += ", with Homebrew already installed"
	}
	env += " (" + VersionLabel() + ")"
	s.WriteString("\n\n")
	s.WriteString(MutedStyle.Render(truncate(env, contentWidth(m))))

	// Center both horizontally and vertically. The frame is the terminal minus
	// the one blank row the global padding adds on top, so the last line of a
	// full-height screen lands on the terminal's last row instead of one past it.
	return CenterBoth(s.String(), contentWidth(m), m.Height-viewPaddingRows)
}

func (m Model) renderMainMenu() string {
	var s strings.Builder

	// Title. The toolbox emoji that used to open it was the one thing on the
	// first screen that a terminal without an emoji font drew as a box.
	s.WriteString(TitleStyle.Render("dotfiles"))
	s.WriteString("\n")
	s.WriteString(MutedStyle.Render("What would you like to do?"))
	s.WriteString("\n\n")

	// Options
	options := m.GetCurrentOptions()
	for i, opt := range options {
		cursor := "  "
		style := UnselectedStyle
		if i == m.Cursor {
			cursor = "▸ "
			style = SelectedStyle
		}
		s.WriteString(style.Render(cursor + opt))
		s.WriteString("\n")
	}

	s.WriteString("\n")
	s.WriteString(helpLine(helpNavigate, helpSelect, helpQuit))

	return s.String()
}

func (m Model) renderSelection() string {
	var s strings.Builder

	// Progress indicator
	s.WriteString(m.renderStepProgress())
	s.WriteString("\n\n")

	// Title
	s.WriteString(TitleStyle.Render(m.GetScreenTitle()))
	s.WriteString("\n")

	// The description can be several sentences (the WSL terminal note is the
	// longest) and used to be left to the frame edge, which clipped it mid-word.
	for _, line := range strings.Split(m.GetScreenDescription(), "\n") {
		for _, row := range wrapText(line, contentWidth(m), 0) {
			s.WriteString(MutedStyle.Render(row))
			s.WriteString("\n")
		}
	}
	s.WriteString("\n")

	// Options
	for _, row := range m.menuRows(m.GetCurrentOptions(), m.Cursor) {
		s.WriteString(row)
		s.WriteString("\n")
	}

	s.WriteString("\n")
	s.WriteString(helpLine(helpNavigate, helpSelect, helpBack))

	return s.String()
}

func (m Model) renderStepProgress() string {
	steps := []string{"OS", "Terminal", "Font", "Shell", "WM", "Nvim"}
	currentIdx := 0

	switch m.Screen {
	case ScreenOSSelect:
		currentIdx = 0
	case ScreenTerminalSelect:
		currentIdx = 1
	case ScreenFontSelect:
		currentIdx = 2
	case ScreenShellSelect:
		// WSL and Termux skip terminal + font: Shell is step 2
		if m.SystemInfo.IsWSL || m.SystemInfo.IsTermux {
			currentIdx = 2
		} else {
			currentIdx = 3
		}
	case ScreenWMSelect:
		// WSL and Termux skip terminal + font: WM is step 3
		if m.SystemInfo.IsWSL || m.SystemInfo.IsTermux {
			currentIdx = 3
		} else {
			currentIdx = 4
		}
	case ScreenNvimSelect:
		// WSL and Termux skip terminal + font: Nvim is step 4
		if m.SystemInfo.IsWSL || m.SystemInfo.IsTermux {
			currentIdx = 4
		} else {
			currentIdx = 5
		}
	}

	var parts []string
	for i, step := range steps {
		var style lipgloss.Style
		if i < currentIdx {
			style = StepDoneStyle
			parts = append(parts, style.Render("✓ "+step))
		} else if i == currentIdx {
			style = StepActiveStyle
			parts = append(parts, style.Render("● "+step))
		} else {
			style = StepPendingStyle
			parts = append(parts, style.Render("○ "+step))
		}
	}

	return strings.Join(parts, MutedStyle.Render(" → "))
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

// toolInfoChrome is the fixed rows the tool-info page spends around its two
// lists: the global top padding, the title, a description of up to two rows, the
// website, the pros header, the cons header, three blank rows and the legend. The
// two lists split what the frame leaves, so a tool with an unusual number of
// entries cannot push the legend off the bottom of the screen.
const toolInfoChrome = 9

func (m Model) renderSingleToolInfo(info ToolInfo) string {
	var s strings.Builder

	width := contentWidth(m)

	// The header and the legend are fixed rows; the two lists share what is left.
	bodyRows := m.Height - viewPaddingRows - toolInfoChrome
	if bodyRows < 4 {
		bodyRows = 4
	}
	prosRows := bodyRows / 2
	consRows := bodyRows - prosRows

	// Tool name and description
	s.WriteString(TitleStyle.Render(info.Name))
	s.WriteString("\n")
	for _, row := range wrapText(info.Description, width, 2) {
		s.WriteString(MutedStyle.Render(row))
		s.WriteString("\n")
	}
	s.WriteString(MutedStyle.Render(truncate(info.Website, width)))
	s.WriteString("\n\n")

	// Pros
	s.WriteString(SuccessStyle.Render("✓ Pros"))
	s.WriteString("\n")
	for _, row := range listRows(info.Pros, "  • ", prosRows, width, InfoStyle) {
		s.WriteString(row)
		s.WriteString("\n")
	}

	s.WriteString("\n")

	// Cons
	s.WriteString(WarningStyle.Render("✗ Cons"))
	s.WriteString("\n")
	for _, row := range listRows(info.Cons, "  • ", consRows, width, MutedStyle) {
		s.WriteString(row)
		s.WriteString("\n")
	}

	s.WriteString("\n")
	s.WriteString(helpLine(helpBack, helpQuit))

	return s.String()
}

// keymapTableChrome is the rows a keymap table spends on everything that is not
// a keymap row: the global top padding, the title, the description, the blank
// under it, the column header, the rule, the blank-and-scroll row under the
// rows, the blank above the legend and the legend itself.
const keymapTableChrome = 10

// keymapTableMinRows keeps the table readable when the frame is at its floor.
const keymapTableMinRows = 3

// keymapTableRows is how many keymap rows the frame leaves. The view and the
// scroll keys both ask for it, so the keys scroll exactly the rows the screen
// draws; the five tables used to compute their own window against m.Height-9,
// which is what let a 15-row window overflow a 24-row frame.
func keymapTableRows(height int) int {
	rows := height - keymapTableChrome
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
// column header and the rows the frame has room for, scrolled to keep the
// visible window inside the list. It replaces five copies of the same table that
// had each picked their own column widths, their own window and their own
// 60-column rule.
func (m Model) renderKeymapTable(category KeymapCategory, scroll int) string {
	var s strings.Builder

	width := contentWidth(m)
	longestKey := 0
	for _, km := range category.Keymaps {
		if w := lipgloss.Width(km.Keys); w > longestKey {
			longestKey = w
		}
	}
	keys, mode, description := keymapTableColumns(width, longestKey)

	s.WriteString(TitleStyle.Render(category.Name))
	s.WriteString("\n")
	s.WriteString(MutedStyle.Render(truncate(category.Description, width)))
	s.WriteString("\n\n")

	header := padRight("Keys", keys) + " " + padRight("Mode", mode) + " Description"
	s.WriteString(SubtitleStyle.Render(truncate(header, width)))
	s.WriteString("\n")
	s.WriteString(rule(width))
	s.WriteString("\n")

	rows := keymapTableRows(m.Height)
	start, end := listWindow(scroll, rows, len(category.Keymaps))
	for i := start; i < end; i++ {
		km := category.Keymaps[i]
		s.WriteString(KeyStyle.Render(padRight(truncate(km.Keys, keys), keys)))
		s.WriteString(" ")
		s.WriteString(MutedStyle.Render(padRight(truncate(km.Mode, mode), mode)))
		s.WriteString(" ")
		s.WriteString(InfoStyle.Render(truncate(km.Description, description)))
		s.WriteString("\n")
	}

	// The scroll row is held even when the list fits, so a short category and a
	// long one lay their legend out on the same row and the height is countable.
	scrollInfo := ""
	if len(category.Keymaps) > rows {
		scrollInfo = fmt.Sprintf("Showing %d-%d of %d", start+1, end, len(category.Keymaps))
	}
	s.WriteString("\n")
	s.WriteString(MutedStyle.Render(scrollInfo))
	s.WriteString("\n")
	s.WriteString(helpLine(helpNavigate, helpBack))

	return s.String()
}

// menuChrome is the rows a menu spends on everything that is not an option row:
// the global top padding, the title, the description, two blank rows, the scroll
// row and the legend. The option list gets what is left, so a long list of
// keymap categories cannot push the legend off the bottom of the screen.
const menuChrome = 7

// renderMenu renders the menus of the keymaps and learn sections. They differ
// only in their description, so the title, the list and the legend are shared:
// six copies of the same menu used to spell the legend three different ways. The
// list is windowed around the cursor, so a section with more categories than the
// frame has rows scrolls instead of overflowing.
func (m Model) renderMenu(description string) string {
	var s strings.Builder

	rows := m.menuRows(m.GetCurrentOptions(), m.Cursor)

	visible := m.Height - viewPaddingRows - menuChrome
	if visible < 1 {
		visible = 1
	}
	start, end := listWindow(m.Cursor, visible, len(rows))

	s.WriteString(TitleStyle.Render(m.GetScreenTitle()))
	s.WriteString("\n")
	s.WriteString(MutedStyle.Render(description))
	s.WriteString("\n\n")

	for _, row := range rows[start:end] {
		s.WriteString(row)
		s.WriteString("\n")
	}

	// The scroll row is held even when the list fits, so a short menu and a long
	// one lay their legend out on the same row and the height is countable.
	scrollInfo := ""
	if len(rows) > visible {
		scrollInfo = fmt.Sprintf("Showing %d-%d of %d", start+1, end, len(rows))
	}
	s.WriteString(MutedStyle.Render(scrollInfo))
	s.WriteString("\n")
	s.WriteString(helpLine(helpNavigate, helpSelect, helpBack))

	return s.String()
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

func (m Model) renderLazyVimTopic() string {
	if m.SelectedLazyVimTopic >= len(m.LazyVimTopics) {
		return deadEnd("Topic not found", "Press [Esc] to return to the guide.")
	}

	topic := m.LazyVimTopics[m.SelectedLazyVimTopic]
	width := contentWidth(m)

	var s strings.Builder

	s.WriteString(TitleStyle.Render(topic.Title))
	s.WriteString("\n")
	s.WriteString(SubtitleStyle.Render(truncate(topic.Description, width)))
	s.WriteString("\n\n")

	// Build the content, cut to the frame so a long line is marked instead of
	// clipped silently at the edge.
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

	// The content window is the rows the frame leaves, and the scroll row is held
	// whether or not the topic is longer than the window, so a short topic and a
	// long one lay their legend out on the same row.
	viewHeight := m.Height - viewPaddingRows - lazyVimTopicChrome
	if viewHeight < 1 {
		viewHeight = 1
	}
	start, end := listWindow(m.LazyVimScroll, viewHeight, len(allLines))

	for i := start; i < end; i++ {
		line := allLines[i]
		// Style code lines differently
		switch {
		case strings.HasPrefix(line, "--"), strings.HasPrefix(line, "local"),
			strings.HasPrefix(line, "return"), strings.HasPrefix(line, "{"),
			strings.HasPrefix(line, "}"), strings.HasPrefix(line, "  "),
			strings.HasPrefix(line, "map("), strings.HasPrefix(line, "vim."),
			strings.HasPrefix(line, "require"):
			s.WriteString(CodeStyle.Render(line))
		case strings.HasPrefix(line, "📝"), strings.HasPrefix(line, "💡"):
			s.WriteString(SubtitleStyle.Render(line))
		case strings.HasPrefix(line, "  •"):
			s.WriteString(InfoStyle.Render(line))
		case strings.HasPrefix(line, "•"):
			s.WriteString(MutedStyle.Render(line))
		default:
			s.WriteString(InfoStyle.Render(line))
		}
		s.WriteString("\n")
	}

	scrollInfo := ""
	if len(allLines) > viewHeight {
		scrollInfo = fmt.Sprintf("Lines %d-%d of %d", start+1, end, len(allLines))
	}
	s.WriteString("\n")
	s.WriteString(MutedStyle.Render(scrollInfo))
	s.WriteString("\n")
	s.WriteString(helpLine(helpNavigate, "[PgUp/PgDn] page", helpBack))

	return s.String()
}

// lazyVimTopicChrome is the rows a topic spends on everything that is not its
// content: the global top padding, the title, the description, two blank rows,
// the scroll row and the legend.
const lazyVimTopicChrome = 8

// Installing screen rows. The bar is the one place the whole run's progress is
// visible at a glance, and the step rail is windowed so a run with fifteen steps
// cannot push its bottom off a 24-row terminal.
const (
	// installingFrameRows is what the screen spends on rows that are not the step
	// rail: the global top padding, the title, the blank, the progress bar, the
	// blank and the legend.
	installingFrameRows = 6
	// installingDetailsRows is the whole height of the log box when details are on:
	// the blank above it, the box's border and padding around three log rows, and
	// the blank below it.
	installingDetailsRows = 9
	// installingDetailsLogLines is how many log rows the box shows. It is the
	// last few, because the newest output is the useful end of a log.
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
// ✗ failed, ⊘ skipped.
func stepGlyph(step InstallStep) (string, lipgloss.Style) {
	switch step.Status {
	case StatusRunning:
		return "●", WarningStyle
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
// screen's own chrome and, when they are on, the log box.
func (m Model) installingStepRows() int {
	chrome := installingFrameRows
	if m.ShowDetails && len(m.LogLines) > 0 {
		chrome += installingDetailsRows
	}
	rows := m.Height - viewPaddingRows - chrome
	if rows < installingMinStepRows {
		rows = installingMinStepRows
	}
	return rows
}

func (m Model) renderInstalling() string {
	var s strings.Builder

	width := contentWidth(m)

	s.WriteString(TitleStyle.Render(m.GetScreenTitle()))
	s.WriteString("\n")

	// Progress bar. It is sized to the frame and labeled with the percentage, so
	// the longest thing a user watches says how far along it is.
	barWidth := width - 8
	if barWidth < 10 {
		barWidth = 10
	}
	progress := m.installProgress()
	s.WriteString(renderProgressBar(barWidth, progress))
	s.WriteString(MutedStyle.Render(fmt.Sprintf(" %3.0f%%", progress*100)))
	s.WriteString("\n\n")

	// Step rail. Each step is one row and the running step's description is one
	// more, and the whole rail is windowed around the running step so a long run
	// keeps the step in progress on screen.
	rows := make([]string, 0, len(m.Steps)+1)
	runningIdx := 0
	for i, step := range m.Steps {
		icon, style := stepGlyph(step)
		if i == m.CurrentStep {
			runningIdx = len(rows)
		}
		rows = append(rows, style.Render(fmt.Sprintf("%s %s", icon, step.Name)))
		if i == m.CurrentStep && step.Status == StatusRunning && step.Description != "" {
			rows = append(rows, MutedStyle.Render("   "+truncate(step.Description, width-4)))
		}
	}

	start, end := listWindow(runningIdx, m.installingStepRows(), len(rows))
	for _, row := range rows[start:end] {
		s.WriteString(row)
		s.WriteString("\n")
	}

	// Log output when details are on. The box is a fixed height and its lines are
	// cut to the frame, so turning details on cannot push the legend off screen.
	if m.ShowDetails && len(m.LogLines) > 0 {
		s.WriteString("\n")
		logs := m.LogLines[max(0, len(m.LogLines)-installingDetailsLogLines):]
		lines := make([]string, 0, len(logs))
		for _, line := range logs {
			lines = append(lines, truncate(line, width-4))
		}
		s.WriteString(BoxStyle.Render(strings.Join(lines, "\n")))
		s.WriteString("\n")
	}

	s.WriteString("\n")
	s.WriteString(helpLine(helpDetails))

	return s.String()
}

func (m Model) renderComplete() string {
	var s strings.Builder

	// The title comes from GetScreenTitle like every other screen; it used to be a
	// second hard-coded string here, which left the model's own title dead.
	s.WriteString(TitleStyle.Render(m.GetScreenTitle()))
	s.WriteString("\n")

	// Summary
	s.WriteString(TitleStyle.Render("Summary"))
	s.WriteString("\n")

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

	for _, item := range items {
		s.WriteString(InfoStyle.Render("  • " + truncate(item, contentWidth(m)-4)))
		s.WriteString("\n")
	}

	// Shell change instructions
	shell := m.Choices.Shell
	shellCmd := shell
	if shell == "nushell" {
		shellCmd = "nu"
	}

	s.WriteString("\n")
	s.WriteString(TitleStyle.Render("Next Step"))
	s.WriteString("\n")

	s.WriteString(InfoStyle.Render("To use your new shell now, run:"))
	s.WriteString("\n")
	s.WriteString(HighlightStyle.Render(fmt.Sprintf("   exec %s", shellCmd)))
	s.WriteString("\n\n")

	s.WriteString(helpLine("[Enter] exit"))

	return s.String()
}

func (m Model) renderError() string {
	var s strings.Builder

	width := contentWidth(m)

	// The title comes from GetScreenTitle like every other screen; it used to be a
	// second hard-coded string here, which left the model's own title dead.
	s.WriteString(TitleStyle.Render(m.GetScreenTitle()))
	s.WriteString("\n")

	s.WriteString(MutedStyle.Render("Error:"))
	s.WriteString("\n")
	// The message is wrapped and capped so a long failure cannot run off the
	// bottom of the screen; the last row ends with the cut marker.
	for _, row := range wrapText(m.ErrorMsg, width, 4) {
		s.WriteString(ErrorStyle.Render(row))
		s.WriteString("\n")
	}
	s.WriteString("\n")

	// Show last few log lines for context
	if len(m.LogLines) > 0 {
		s.WriteString(MutedStyle.Render("Recent logs:"))
		s.WriteString("\n")
		// Show last 5 log lines, cut to the frame so a long line is marked rather
		// than clipped at the edge.
		startIdx := len(m.LogLines) - 5
		if startIdx < 0 {
			startIdx = 0
		}
		for _, line := range m.LogLines[startIdx:] {
			s.WriteString(InfoStyle.Render("  " + truncate(line, width-2)))
			s.WriteString("\n")
		}
		s.WriteString("\n")
	}

	s.WriteString(helpLine("[r] retry", helpQuit))

	return s.String()
}

// backupConfirmChrome is the rows the backup-confirmation screen spends on
// everything that is not the config list: the global top padding, the title, the
// description, three blank rows, the note, the three options and the legend. The
// list gets what is left, so a machine with every config path present cannot push
// the options off the bottom of the screen.
const backupConfirmChrome = 13

func (m Model) renderBackupConfirm() string {
	var s strings.Builder

	width := contentWidth(m)

	s.WriteString(TitleStyle.Render(m.GetScreenTitle()))
	s.WriteString("\n")
	s.WriteString(MutedStyle.Render("The following configs will be overwritten:"))
	s.WriteString("\n\n")

	// The config list is bounded to the frame: a machine with all sixteen config
	// paths present used to push the options off the bottom of the screen.
	configRows := m.Height - viewPaddingRows - backupConfirmChrome
	if configRows < 1 {
		configRows = 1
	}
	for _, row := range listRows(m.ExistingConfigs, "  ⚠️ ", configRows, width, WarningStyle) {
		s.WriteString(row)
		s.WriteString("\n")
	}

	s.WriteString("\n")
	s.WriteString(InfoStyle.Render("Creating a backup allows you to restore later if needed."))
	s.WriteString("\n\n")

	for _, row := range m.menuRows(m.GetCurrentOptions(), m.Cursor) {
		s.WriteString(row)
		s.WriteString("\n")
	}

	s.WriteString("\n")
	s.WriteString(helpLine(helpNavigate, helpSelect, helpBack))

	return s.String()
}

// restoreBackupChrome is the rows the restore list spends on everything that is
// not a backup: the global top padding, the title, the description, three blank
// rows, the scroll row, the rule, the Back row and the legend.
const restoreBackupChrome = 10

func (m Model) renderRestoreBackup() string {
	var s strings.Builder

	width := contentWidth(m)

	s.WriteString(TitleStyle.Render(m.GetScreenTitle()))
	s.WriteString("\n")
	s.WriteString(MutedStyle.Render("Select a backup to restore or delete"))
	s.WriteString("\n\n")

	// The list is bounded and follows the cursor, so a long history scrolls
	// instead of pushing the Back row off the frame.
	listBudget := m.Height - viewPaddingRows - restoreBackupChrome
	if listBudget < 1 {
		listBudget = 1
	}

	start, end := 0, 0
	if len(m.AvailableBackups) == 0 {
		s.WriteString(MutedStyle.Render("No backups found."))
		s.WriteString("\n")
	} else {
		start, end = listWindow(m.Cursor, listBudget, len(m.AvailableBackups))
		for i := start; i < end; i++ {
			backup := m.AvailableBackups[i]
			cursor, style := "  ", UnselectedStyle
			if i == m.Cursor {
				cursor, style = "▸ ", SelectedStyle
			}
			label := fmt.Sprintf("📁 %s (%d items)", backup.Timestamp.Format("2006-01-02 15:04:05"), len(backup.Files))
			s.WriteString(style.Render(cursor + truncate(label, width-2)))
			s.WriteString("\n")
		}
	}

	// The scroll row is held even when the list fits, so the Back row and the
	// legend do not move as the history grows.
	scrollInfo := ""
	if len(m.AvailableBackups) > listBudget {
		scrollInfo = fmt.Sprintf("Showing %d-%d of %d", start+1, end, len(m.AvailableBackups))
	}
	s.WriteString(MutedStyle.Render(scrollInfo))
	s.WriteString("\n")

	// Separator and Back. The separator is the shared frame-width rule, not the
	// fixed 13-glyph string that disagreed with every other divider.
	s.WriteString(rule(width))
	s.WriteString("\n")

	backIdx := len(m.AvailableBackups) + 1
	cursor, style := "  ", UnselectedStyle
	if m.Cursor == backIdx {
		cursor, style = "▸ ", SelectedStyle
	}
	s.WriteString(style.Render(cursor + "← Back"))
	s.WriteString("\n")

	s.WriteString("\n")
	s.WriteString(helpLine(helpNavigate, helpSelect, helpBack))

	return s.String()
}

// restoreConfirmChrome is the rows the restore confirmation spends on everything
// that is not the file list: the global top padding, the title, the description,
// three blank rows, the Contents header, the warning, the three options and the
// legend. The list gets what is left.
const restoreConfirmChrome = 14

func (m Model) renderRestoreConfirm() string {
	if m.SelectedBackup >= len(m.AvailableBackups) {
		return deadEnd("No backup selected", "Press [Esc] to go back.")
	}

	backup := m.AvailableBackups[m.SelectedBackup]

	var s strings.Builder

	width := contentWidth(m)

	s.WriteString(TitleStyle.Render(m.GetScreenTitle()))
	s.WriteString("\n")
	s.WriteString(MutedStyle.Render("Backup from: " + backup.Timestamp.Format("2006-01-02 15:04:05")))
	s.WriteString("\n\n")

	// List files in backup, bounded to the frame so a backup with a long file
	// list cannot push the options off the bottom of the screen.
	s.WriteString(SubtitleStyle.Render("Contents:"))
	s.WriteString("\n")
	fileRows := m.Height - viewPaddingRows - restoreConfirmChrome
	if fileRows < 1 {
		fileRows = 1
	}
	for _, row := range listRows(backup.Files, "  • ", fileRows, width, InfoStyle) {
		s.WriteString(row)
		s.WriteString("\n")
	}

	s.WriteString("\n")
	s.WriteString(WarningStyle.Render("⚠️ Restoring will overwrite your current configs!"))
	s.WriteString("\n\n")

	for _, row := range m.menuRows(m.GetCurrentOptions(), m.Cursor) {
		s.WriteString(row)
		s.WriteString("\n")
	}

	s.WriteString("\n")
	s.WriteString(helpLine(helpNavigate, helpSelect, helpCancel))

	return s.String()
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
// Two rules keep the arithmetic true: a legend is rendered one line per element
// with no margin of its own (that is what TrainerHelpStyle is for), and a
// wrapped block is capped to a fixed number of elements so it cannot consume
// rows the code window was promised.

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
	// trainerFrameRows is what the exercise and boss screens spend on rows that
	// are neither the mission text nor the code window: the global top padding
	// View() adds, the title, the progress or lives line, the reserved countdown
	// row, the blank above the mission label, that label, the blank, the code
	// label, both rules, the answer row, the reserved feedback area and the
	// two-line legend. The mission and the code window share what is left, which
	// is 24-16 = 8 rows at 80x24. It is checked rather than trusted:
	// TestTrainerScreensFitTheFrame renders every lesson and every boss step at
	// 80x24, so a screen that spends a row this constant does not count fails that
	// test instead of quietly losing the bottom of the screen.
	trainerFrameRows = 16
)

// The labels and hints the trainer screens share. Each hint is one named
// fragment, joined through the same helpNotation as every other screen, so the
// menu, the lesson, the practice and the boss legends cannot drift apart on the
// keys they promise: Ctrl-e types the token an insert answer needs to leave
// insert mode, Esc is the trainer's own exit key, Backspace is an input edit the
// engine never sees, and PgUp/PgDn scroll the code window. The scroll keys are
// key names rather than printable characters, so they cannot collide with
// typing, with the hint key, with submit or with back.
const (
	trainerAnswerLabel = "⌨️ Your answer: "

	trainerHelpLesson   = "[Enter/l] lesson"
	trainerHelpPractice = "[p] practice"
	trainerHelpBoss     = "[b] boss"
	trainerHelpReset    = "[r] reset module"
	trainerHelpResetAll = "[R] reset all"
	trainerHelpBack     = "[q/Esc] back"

	trainerHelpTypeCommand = "Type command"
	trainerHelpSubmit      = "[Enter] submit"
	trainerHelpHint        = "[Tab] hint"
	trainerHelpScroll      = "[PgUp/PgDn] scroll"
	trainerHelpDelete      = "[Backspace] delete"
	trainerHelpEscToken    = "[Ctrl-e] type " + trainer.EscToken
	trainerHelpQuit        = "[Esc] quit"
	trainerHelpForfeit     = "[Esc] forfeit"
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

// trainerModuleProgressText renders the per-module progress the trainer menu
// shows: lessons completed against total, and mastered exercises. Mastery comes
// from GetPracticeStatsForModule, which reads the same ExerciseStats.IsMastered
// predicate weighted practice selection uses, so the count on screen cannot
// drift from the practice pool. The boss is not repeated here: the row's status
// field already says whether the boss is cleared, and saying it twice is what
// cost the row its room inside 80 columns.
func trainerModuleProgressText(progress *trainer.ModuleProgress, practice trainer.PracticeStats) string {
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

	return fmt.Sprintf("Lessons %d/%d · Mastered %d/%d",
		lessonsCompleted, lessonsTotal, practice.MasteredCount, practice.TotalExercises)
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

	const prefix = "     ⚠️ Weakest: "
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

// renderTrainerMenu renders the module list. Every row state starts on the same
// column: the marker column is fixed width, the status field is padded to
// trainerModuleStatusWidth, and the row styles carry no padding of their own
// (the shared SelectedStyle's PaddingLeft is what used to push the selected row
// two columns right of the others while a locked row lost its padding
// entirely). What used to sit on the row -- the accuracy of the selected module
// -- moved into the detail block under it, so the row survives 80 columns.
func (m Model) renderTrainerMenu() string {
	var s strings.Builder

	inner := trainerInnerWidth(m)

	// Header. The trainer title style has no margin of its own: the blank line
	// under the title is one of the menu's row elements, counted with the rest.
	s.WriteString(TrainerTitleStyle.Render("🎮 Vim Mastery Trainer"))
	s.WriteString("\n")
	s.WriteString(MutedStyle.Render("Master Vim motions through progressive challenges"))
	s.WriteString("\n")

	// Stats bar
	if m.TrainerStats != nil {
		score := fmt.Sprintf("Score: %d", m.TrainerStats.TotalScore)
		streak := fmt.Sprintf("Streak: %d", m.TrainerStats.CurrentStreak)
		bosses := fmt.Sprintf("Bosses: %d/%d", len(m.TrainerStats.BossesDefeated), len(trainer.GetAllModules()))
		s.WriteString(InfoStyle.Render(fmt.Sprintf("📊 %s  |  🔥 %s  |  👑 %s", score, streak, bosses)))
		s.WriteString("\n")
	}

	s.WriteString("\n")
	s.WriteString(SubtitleStyle.Render("Select a Module:"))
	s.WriteString("\n")

	for i, module := range m.TrainerModules {
		isUnlocked := m.TrainerStats != nil && m.TrainerStats.IsModuleUnlocked(module.ID)
		isBossDefeated := m.TrainerStats != nil && m.TrainerStats.IsBossDefeated(module.ID)
		isPracticeReady := m.TrainerStats != nil && m.TrainerStats.IsPracticeReady(module.ID)
		isBossReady := m.TrainerStats != nil && m.TrainerStats.IsBossReady(module.ID)

		marker := "  "
		style := TrainerRowStyle
		if i == m.TrainerCursor {
			marker = "▸ "
			style = TrainerRowSelectedStyle
		}
		if !isUnlocked {
			// A locked row keeps its marker column and loses the emphasis: the
			// column is what makes the list scannable, the colour is what says the
			// row cannot be entered yet.
			style = TrainerRowLockedStyle
		}

		status := padRight(trainerModuleStatus(isUnlocked, isBossDefeated, isBossReady, isPracticeReady), trainerModuleStatusWidth)
		line := fmt.Sprintf("%s%s %s %s", marker, status, module.Icon, module.Name)

		// Progress is display-only text attached to the existing entry. The
		// unlock/ready predicates above and this direct map read both look the
		// recorded progress up without creating records, so rendering the menu
		// neither manufactures module records nor exercise records in the
		// persisted stats.
		var progress *trainer.ModuleProgress
		var practice trainer.PracticeStats
		if m.TrainerStats != nil {
			progress = m.TrainerStats.ModuleProgress[module.ID]
			practice = trainer.GetPracticeStatsForModule(module.ID, progress)
			line += "  " + trainerModuleProgressText(progress, practice)
		}

		s.WriteString(style.Render(line))
		s.WriteString("\n")

		// The selected module's detail block: its commands, the accuracy the row
		// no longer carries, and the exercises it misses most. All three are
		// display-only lines attached to the entry.
		if i == m.TrainerCursor {
			s.WriteString(MutedStyle.Render("     " + module.Description))
			s.WriteString("\n")
			if accuracy := trainerPracticeAccuracyText(progress); accuracy != "" {
				s.WriteString(InfoStyle.Render("     " + accuracy))
				s.WriteString("\n")
			}
			if weak := trainerWeakExerciseText(progress, practice, inner); weak != "" {
				s.WriteString(WarningStyle.Render(weak))
				s.WriteString("\n")
			}
		}
	}

	// Feedback. The rows are held even when there is nothing to say, so the
	// module rows above never move when a message arrives, and they double as the
	// gap above the legend.
	message := wrapText(m.TrainerMessage, inner, trainerMessageRows)
	for i := 0; i < trainerMessageRows; i++ {
		if i < len(message) {
			s.WriteString(WarningStyle.Render(message[i]))
		}
		s.WriteString("\n")
	}

	// Legend, one row per line through the shared notation, so the menu's height
	// is countable and its keys read the way every other screen's do.
	s.WriteString(trainerHelpLine(helpNavigate, trainerHelpLesson, trainerHelpPractice, trainerHelpBoss))
	s.WriteString("\n")
	s.WriteString(trainerHelpLine(trainerHelpReset, trainerHelpResetAll, trainerHelpBack))

	return s.String()
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
	missionRows = wrapText(exercise.Mission, trainerInnerWidth(m)-3, maxMission)

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
// appear under the mission, so the window has to say where it is.
func (v codeViewport) label() string {
	if v.rows >= len(v.exercise.Code) {
		return "📝 Code:"
	}
	last := v.offset + v.rows
	if last > len(v.exercise.Code) {
		last = len(v.exercise.Code)
	}
	return fmt.Sprintf("📝 Code:  rows %d-%d of %d", v.offset+1, last, len(v.exercise.Code))
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

// renderTrainerExercise renders a lesson or practice exercise. The answer area is
// one row and the code is a window, because the screen has to fit the 24-row
// terminal the trainer claims: see trainerFrameRows for the arithmetic.
func (m Model) renderTrainerExercise(mode string) string {
	exercise := m.trainerCurrentExercise()
	if exercise == nil {
		return deadEnd("No exercise loaded", "Press [Esc] to return to the trainer.")
	}

	inner := trainerInnerWidth(m)
	mission, codeRows := m.trainerTextBudget(exercise)

	// Progress bar
	var progressText string
	if m.TrainerGameState.IsLessonMode {
		current := m.TrainerGameState.ExerciseIndex + 1
		total := len(m.TrainerGameState.Exercises)
		progressText = fmt.Sprintf("Exercise %d of %d", current, total)
	} else {
		progressText = fmt.Sprintf("Score: %d | Streak: %d", m.TrainerGameState.SessionScore, m.TrainerGameState.CurrentStreak)
	}

	// Live countdown to the automatic hint. It is derived from the exercise's own
	// TimeoutSecs through the game state's injected clock, so it keeps counting
	// while the animation tick re-renders the screen. Once the deadline passes
	// the countdown disappears and the hint itself appears in the feedback area;
	// the row stays, so the code below does not move when that happens.
	countdown := ""
	if remaining := m.TrainerGameState.RemainingSeconds(); remaining > 0 {
		countdown = WarningStyle.Render(fmt.Sprintf("⏳ Hint in %ds", int(math.Ceil(remaining))))
	}

	viewport := m.trainerCodeViewport(exercise, codeRows, inner-trainerGutterWidth)

	// Every element below is one terminal row, which is what makes the code
	// window's height the frame minus the chrome instead of an estimate: the
	// screen used to render 22+N rows and lose its bottom.
	rows := []string{
		TrainerTitleStyle.Render(fmt.Sprintf("🎮 %s Mode: %s", mode, string(m.TrainerGameState.CurrentModule))),
		MutedStyle.Render(progressText),
		countdown,
		"",
		SubtitleStyle.Render("📋 Mission:"),
	}
	for _, row := range mission {
		rows = append(rows, InfoStyle.Render("   "+row))
	}
	rows = append(rows,
		"",
		SubtitleStyle.Render(viewport.label()),
		rule(inner),
	)
	rows = append(rows, viewport.render()...)
	rows = append(rows,
		rule(inner),
		SubtitleStyle.Render(trainerAnswerLabel)+KeyStyle.Render(tailToWidth(m.trainerAnswer(), inner-lipgloss.Width(trainerAnswerLabel))),
	)
	rows = append(rows, m.trainerFeedbackRows(InfoStyle)...)

	// Help. The two lines go through trainerHelpLine, one row each, so the
	// screen's height is countable: see trainerFrameRows.
	rows = append(rows,
		"",
		trainerHelpLine(trainerHelpTypeCommand, trainerHelpSubmit, trainerHelpHint, trainerHelpScroll),
		trainerHelpLine(trainerHelpDelete, trainerHelpEscToken, trainerHelpQuit),
	)

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

func (m Model) renderTrainerBoss() string {
	state := m.TrainerGameState
	if state == nil || state.CurrentBoss == nil {
		return deadEnd("No boss loaded", "Press [Esc] to return to the trainer.")
	}

	boss := state.CurrentBoss
	currentStep := state.BossStep
	inner := trainerInnerWidth(m)

	// Live countdown for the current step, in the same visual language as the
	// exercise screen's hint countdown. It is derived from the step's own
	// TimeLimit plus any bonus through the game state's injected clock, so it
	// keeps counting while the animation tick re-renders the screen. Once the
	// deadline passes the countdown disappears and the tick charges the life.
	countdown := ""
	if remaining := state.BossStepSecondsLeft(); remaining > 0 {
		countdown = WarningStyle.Render(fmt.Sprintf("⏳ Time left: %ds", int(math.Ceil(remaining))))
	}

	// The lives are a count first and a glyph run second. The red and black heart
	// emoji this replaces were the only thing saying how many lives were left, and
	// a terminal without an emoji font drew them as boxes.
	lives := fmt.Sprintf("Lives: %d/%d %s  |  Step: %d/%d",
		state.BossLives, boss.Lives, trainerLivesGlyphs(state.BossLives, boss.Lives), currentStep+1, len(boss.Steps))

	// Every element below is one terminal row. The boss screen spends the same
	// chrome as the exercise screen (trainerFrameRows), so it gets the same code
	// window: a 14-line boss step scrolls instead of running off the bottom, which
	// is where its whole second half used to be.
	rows := []string{
		DangerStyle.Render("⚔️ BOSS FIGHT: " + boss.Name),
		lives,
		countdown,
		"",
		SubtitleStyle.Render("📋 Challenge:"),
	}

	exercise := state.CurrentExercise
	if currentStep >= len(boss.Steps) || exercise == nil {
		// No step is on screen, so there is no code window to size around.
		rows = append(rows, m.trainerFeedbackRows(WarningStyle)...)
		rows = append(rows, "",
			trainerHelpLine(trainerHelpTypeCommand, trainerHelpSubmit, trainerHelpEscToken),
			trainerHelpLine(trainerHelpScroll, trainerHelpForfeit))
		return strings.Join(rows, "\n")
	}

	mission, codeRows := m.trainerTextBudget(exercise)
	for _, row := range mission {
		rows = append(rows, InfoStyle.Render("   "+row))
	}

	viewport := m.trainerCodeViewport(exercise, codeRows, inner-trainerGutterWidth)
	rows = append(rows,
		"",
		SubtitleStyle.Render(viewport.label()),
		rule(inner),
	)
	rows = append(rows, viewport.render()...)
	rows = append(rows,
		rule(inner),
		SubtitleStyle.Render(trainerAnswerLabel)+KeyStyle.Render(tailToWidth(m.trainerAnswer(), inner-lipgloss.Width(trainerAnswerLabel))),
	)
	rows = append(rows, m.trainerFeedbackRows(WarningStyle)...)
	rows = append(rows, "",
		trainerHelpLine(trainerHelpTypeCommand, trainerHelpSubmit, trainerHelpEscToken),
		trainerHelpLine(trainerHelpScroll, trainerHelpForfeit))

	return strings.Join(rows, "\n")
}

// renderBufferLines renders a buffer as numbered lines, matching the numbering
// the exercise screen uses so the result lines up with the code the user saw. A
// line wider than the frame is cut and marked: the frame edge used to clip the
// buffer silently, which is the defect the exercise screen had.
func renderBufferLines(buffer []string, width int) string {
	rows := []string{rule(width)}
	for i, line := range buffer {
		rows = append(rows, MutedStyle.Render(fmt.Sprintf("%2d │ ", i+1))+CodeStyle.Render(truncate(line, width-trainerGutterWidth)))
	}
	rows = append(rows, "")
	return strings.Join(rows, "\n")
}

func (m Model) renderTrainerResult() string {
	var s strings.Builder

	inner := trainerInnerWidth(m)

	// Result header
	if m.TrainerLastCorrect {
		s.WriteString(SuccessStyle.Render("✨ CORRECT! ✨"))
	} else {
		s.WriteString(ErrorStyle.Render("❌ INCORRECT"))
	}
	s.WriteString("\n\n")

	// Show message/explanation. The message is wrapped instead of being left to
	// the frame edge, which cut the solutions list mid-word.
	message := wrapText(m.TrainerMessage, inner, 0)
	if len(message) == 0 {
		message = []string{""}
	}
	for _, row := range message {
		s.WriteString(InfoStyle.Render(row))
		s.WriteString("\n")
	}

	if m.TrainerGameState != nil && m.TrainerGameState.CurrentExercise != nil {
		exercise := m.TrainerGameState.CurrentExercise
		if exercise.Explanation != "" {
			s.WriteString("\n")
			s.WriteString(SubtitleStyle.Render("📖 Explanation:"))
			s.WriteString("\n")
			// The explanation is wrapped rather than clipped: the result screen's
			// snapshot used to end mid-word at the frame edge.
			for _, row := range wrapText(exercise.Explanation, inner-3, 0) {
				s.WriteString(MutedStyle.Render("   " + row))
				s.WriteString("\n")
			}
		}

		// A buffer-verified answer is taught by the buffer it produced, which is
		// the only place its effect is visible. The preview is gated on the
		// judge that ran, so a shipped exercise renders exactly as before. A
		// rejected answer shows the expected buffer next to the produced one,
		// because the difference is what the lesson is about.
		if exercise.BufferVerified && m.TrainerValidation != nil && m.TrainerValidation.BufferVerified {
			if !m.TrainerValidation.IsCorrect {
				s.WriteString("\n")
				s.WriteString(SubtitleStyle.Render("🎯 Expected buffer:"))
				s.WriteString("\n")
				s.WriteString(renderBufferLines(m.TrainerValidation.TargetBuffer, inner))
			}
			s.WriteString("\n")
			s.WriteString(SubtitleStyle.Render("📝 Resulting buffer:"))
			s.WriteString("\n")
			s.WriteString(renderBufferLines(m.TrainerValidation.ActualBuffer, inner))
		}
	}

	// Score info
	if m.TrainerGameState != nil {
		s.WriteString("\n")
		s.WriteString(MutedStyle.Render(fmt.Sprintf("Session Score: %d  |  Streak: %d", m.TrainerGameState.SessionScore, m.TrainerGameState.CurrentStreak)))
		s.WriteString("\n")
	}

	// Help
	s.WriteString("\n")
	s.WriteString(helpLine("[Enter] continue", helpBack))

	return s.String()
}

func (m Model) renderTrainerBossResult() string {
	var s strings.Builder

	// Victory or defeat
	if m.TrainerLastCorrect {
		s.WriteString(SuccessStyle.Render("🏆 VICTORY! 🏆"))
		s.WriteString("\n\n")
		if m.TrainerGameState != nil && m.TrainerGameState.CurrentBoss != nil {
			s.WriteString(TitleStyle.Render("You defeated " + m.TrainerGameState.CurrentBoss.Name + "!"))
			s.WriteString("\n\n")
			s.WriteString(InfoStyle.Render(fmt.Sprintf("Lives remaining: %d/%d %s",
				m.TrainerGameState.BossLives, m.TrainerGameState.CurrentBoss.Lives,
				trainerLivesGlyphs(m.TrainerGameState.BossLives, m.TrainerGameState.CurrentBoss.Lives))))
			s.WriteString("\n")
		}
		s.WriteString("\n")
		s.WriteString(SuccessStyle.Render("🎉 +500 bonus points!"))
		s.WriteString("\n")
		nextModuleExists := false
		if m.TrainerGameState != nil {
			_, nextModuleExists = trainer.NextModule(m.TrainerGameState.CurrentModule)
		}
		if nextModuleExists {
			s.WriteString(SuccessStyle.Render("🔓 Next module unlocked!"))
		} else {
			s.WriteString(SuccessStyle.Render("👑 No modules left — you have cleared them all!"))
		}
	} else {
		s.WriteString(DangerStyle.Render("💀 DEFEATED 💀"))
		s.WriteString("\n\n")
		if m.TrainerGameState != nil && m.TrainerGameState.CurrentBoss != nil {
			s.WriteString(MutedStyle.Render(m.TrainerGameState.CurrentBoss.Name + " wins this time..."))
			s.WriteString("\n\n")
		}
		s.WriteString(InfoStyle.Render("Keep practicing and try again!"))
	}

	// Show message
	if m.TrainerMessage != "" {
		s.WriteString("\n\n")
		s.WriteString(MutedStyle.Render(m.TrainerMessage))
	}

	// Stats
	if m.TrainerStats != nil {
		s.WriteString("\n\n")
		s.WriteString(MutedStyle.Render(fmt.Sprintf("Total Score: %d  |  Bosses Defeated: %d/%d", m.TrainerStats.TotalScore, len(m.TrainerStats.BossesDefeated), len(trainer.GetAllModules()))))
	}

	// Help
	s.WriteString("\n\n")
	s.WriteString(helpLine(helpReturnToMenu))

	return s.String()
}
