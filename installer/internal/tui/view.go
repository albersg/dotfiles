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

	// Leader mode indicator
	if m.LeaderMode {
		s.WriteString("\n")
		s.WriteString(WarningStyle.Render("▶ LEADER MODE - Press: q=quit, d=details"))
	}

	// Apply global padding (top: 1, right: 2, bottom: 0, left: 2)
	paddedStyle := lipgloss.NewStyle().Padding(1, 2, 0, 2)
	return paddedStyle.Render(s.String())
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

	// System info
	info := fmt.Sprintf("Detected: %s", m.SystemInfo.OSName)
	if m.SystemInfo.IsWSL && m.SystemInfo.OSName != "WSL" {
		info += " (WSL)"
	}
	if m.SystemInfo.HasBrew {
		info += " | Homebrew ✓"
	}
	// The splash carries the build version because it is the first thing a bug
	// report needs, and this is where a user sees it without knowing that a
	// --version flag exists.
	info += " | " + VersionLabel()
	s.WriteString(InfoStyle.Render(info))
	s.WriteString("\n\n")

	// Instructions
	s.WriteString(SubtitleStyle.Render("Your terminal environment, configured in minutes."))
	s.WriteString("\n\n")
	s.WriteString(HelpStyle.Render("Press [Enter] to start • [Space q] to quit"))

	// Center both horizontally and vertically
	return CenterBoth(s.String(), m.Width, m.Height)
}

func (m Model) renderMainMenu() string {
	var s strings.Builder

	// Title
	s.WriteString(TitleStyle.Render("🧰 dotfiles"))
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
	s.WriteString(HelpStyle.Render("↑/k up • ↓/j down • [Enter] select • [Space q] quit"))

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
	s.WriteString(MutedStyle.Render(m.GetScreenDescription()))
	s.WriteString("\n\n")

	// Options
	options := m.GetCurrentOptions()
	for i, opt := range options {
		// Separator line
		if strings.HasPrefix(opt, "───") {
			s.WriteString(MutedStyle.Render(opt))
			s.WriteString("\n")
			continue
		}

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
	s.WriteString(HelpStyle.Render("↑/k up • ↓/j down • [Enter] select • [Esc] back"))

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
	var s strings.Builder

	s.WriteString(TitleStyle.Render(m.GetScreenTitle()))
	s.WriteString("\n")
	s.WriteString(MutedStyle.Render("Select a terminal to learn more about it"))
	s.WriteString("\n\n")

	// If viewing a specific tool, show its info
	if m.ViewingTool != "" {
		return m.renderToolInfo(GetTerminalInfo(), m.ViewingTool, "terminal")
	}

	// Menu
	options := m.GetCurrentOptions()
	for i, opt := range options {
		if strings.HasPrefix(opt, "───") {
			s.WriteString(MutedStyle.Render(opt))
			s.WriteString("\n")
			continue
		}

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
	s.WriteString(HelpStyle.Render("↑/k up • ↓/j down • [Enter] select • [Esc] back"))

	return s.String()
}

func (m Model) renderLearnShells() string {
	var s strings.Builder

	s.WriteString(TitleStyle.Render(m.GetScreenTitle()))
	s.WriteString("\n")
	s.WriteString(MutedStyle.Render("Select a shell to learn more about it"))
	s.WriteString("\n\n")

	// If viewing a specific tool, show its info
	if m.ViewingTool != "" {
		return m.renderToolInfo(GetShellInfo(), m.ViewingTool, "shell")
	}

	// Menu
	options := m.GetCurrentOptions()
	for i, opt := range options {
		if strings.HasPrefix(opt, "───") {
			s.WriteString(MutedStyle.Render(opt))
			s.WriteString("\n")
			continue
		}

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
	s.WriteString(HelpStyle.Render("↑/k up • ↓/j down • [Enter] select • [Esc] back"))

	return s.String()
}

func (m Model) renderLearnWM() string {
	var s strings.Builder

	s.WriteString(TitleStyle.Render(m.GetScreenTitle()))
	s.WriteString("\n")
	s.WriteString(MutedStyle.Render("Select a window manager to learn more about it"))
	s.WriteString("\n\n")

	// If viewing a specific tool, show its info
	if m.ViewingTool != "" {
		return m.renderToolInfo(GetWMInfo(), m.ViewingTool, "wm")
	}

	// Menu
	options := m.GetCurrentOptions()
	for i, opt := range options {
		if strings.HasPrefix(opt, "───") {
			s.WriteString(MutedStyle.Render(opt))
			s.WriteString("\n")
			continue
		}

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
	s.WriteString(HelpStyle.Render("↑/k up • ↓/j down • [Enter] select • [Esc] back"))

	return s.String()
}

func (m Model) renderLearnNvim() string {
	var s strings.Builder

	s.WriteString(TitleStyle.Render(m.GetScreenTitle()))
	s.WriteString("\n")
	s.WriteString(MutedStyle.Render("Explore Neovim features and keybindings"))
	s.WriteString("\n\n")

	// If viewing features, show Nvim info
	if m.ViewingTool == "features" {
		info := GetNvimInfo()
		return m.renderSingleToolInfo(info)
	}

	// Menu
	options := m.GetCurrentOptions()
	for i, opt := range options {
		if strings.HasPrefix(opt, "───") {
			s.WriteString(MutedStyle.Render(opt))
			s.WriteString("\n")
			continue
		}

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
	s.WriteString(HelpStyle.Render("↑/k up • ↓/j down • [Enter] select • [Esc] back"))

	return s.String()
}

func (m Model) renderToolInfo(tools map[string]ToolInfo, toolKey string, category string) string {
	var s strings.Builder

	info, exists := tools[toolKey]
	if !exists {
		s.WriteString(ErrorStyle.Render("Tool not found"))
		return s.String()
	}

	return m.renderSingleToolInfo(info)
}

func (m Model) renderSingleToolInfo(info ToolInfo) string {
	var s strings.Builder

	// Tool name and description
	s.WriteString(TitleStyle.Render(info.Name))
	s.WriteString("\n")
	s.WriteString(MutedStyle.Render(info.Description))
	s.WriteString("\n")
	s.WriteString(MutedStyle.Render(info.Website))
	s.WriteString("\n\n")

	// Pros
	s.WriteString(SuccessStyle.Render("✓ Pros"))
	s.WriteString("\n")
	for _, pro := range info.Pros {
		s.WriteString(InfoStyle.Render("  • " + pro))
		s.WriteString("\n")
	}

	s.WriteString("\n")

	// Cons
	s.WriteString(WarningStyle.Render("✗ Cons"))
	s.WriteString("\n")
	for _, con := range info.Cons {
		s.WriteString(MutedStyle.Render("  • " + con))
		s.WriteString("\n")
	}

	s.WriteString("\n")
	s.WriteString(HelpStyle.Render("↑/k up • ↓/j down • [Enter] select • [Esc] back • [Space q] quit"))

	return s.String()
}

func (m Model) renderKeymapsMenu() string {
	var s strings.Builder

	s.WriteString(TitleStyle.Render(m.GetScreenTitle()))
	s.WriteString("\n")
	s.WriteString(MutedStyle.Render("Select a category to view keybindings"))
	s.WriteString("\n\n")

	// Menu
	options := m.GetCurrentOptions()
	for i, opt := range options {
		if strings.HasPrefix(opt, "───") {
			s.WriteString(MutedStyle.Render(opt))
			s.WriteString("\n")
			continue
		}

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
	s.WriteString(HelpStyle.Render("↑/k up • ↓/j down • [Enter] select • [Esc/q] back"))

	return s.String()
}

func (m Model) renderKeymapCategory() string {
	var s strings.Builder

	if m.SelectedCategory >= len(m.KeymapCategories) {
		return ErrorStyle.Render("Category not found")
	}

	category := m.KeymapCategories[m.SelectedCategory]

	s.WriteString(TitleStyle.Render(category.Name))
	s.WriteString("\n")
	s.WriteString(MutedStyle.Render(category.Description))
	s.WriteString("\n\n")

	// Table header
	header := fmt.Sprintf("%-15s %-6s %s", "Keys", "Mode", "Description")
	s.WriteString(SubtitleStyle.Render(header))
	s.WriteString("\n")
	s.WriteString(MutedStyle.Render(strings.Repeat("─", 60)))
	s.WriteString("\n")

	// Calculate visible items based on terminal height
	// Reserve space for: title(1) + description(1) + blank(1) + header(1) + separator(1) + scroll info(2) + help(2) = 9 lines
	visibleItems := m.Height - 9
	if visibleItems < 5 {
		visibleItems = 5 // Minimum 5 items
	}
	if visibleItems > len(category.Keymaps) {
		visibleItems = len(category.Keymaps)
	}

	// Keymaps with scrolling
	start := m.KeymapScroll
	end := start + visibleItems
	if end > len(category.Keymaps) {
		end = len(category.Keymaps)
		start = end - visibleItems
		if start < 0 {
			start = 0
		}
	}

	for i := start; i < end; i++ {
		km := category.Keymaps[i]
		s.WriteString(KeyStyle.Render(km.Keys))
		s.WriteString(MutedStyle.Render(fmt.Sprintf(" %-6s ", km.Mode)))
		s.WriteString(InfoStyle.Render(km.Description))
		s.WriteString("\n")
	}

	// Scroll indicator
	if len(category.Keymaps) > visibleItems {
		s.WriteString("\n")
		scrollInfo := fmt.Sprintf("Showing %d-%d of %d", start+1, end, len(category.Keymaps))
		s.WriteString(MutedStyle.Render(scrollInfo))
	}

	s.WriteString("\n\n")
	s.WriteString(HelpStyle.Render("↑/k up • ↓/j down • [Enter/Esc/q] back"))

	return s.String()
}

// renderToolKeymapsMenu renders the tool selection menu (Neovim, Tmux, Zellij, Ghostty)
func (m Model) renderToolKeymapsMenu() string {
	var s strings.Builder

	s.WriteString(TitleStyle.Render(m.GetScreenTitle()))
	s.WriteString("\n")
	s.WriteString(MutedStyle.Render("Select a tool to view its keybindings"))
	s.WriteString("\n\n")

	// Menu
	options := m.GetCurrentOptions()
	for i, opt := range options {
		if strings.HasPrefix(opt, "───") {
			s.WriteString(MutedStyle.Render(opt))
			s.WriteString("\n")
			continue
		}

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
	s.WriteString(HelpStyle.Render("↑/k up • ↓/j down • [Enter] select • [Esc/q] back"))

	return s.String()
}

// renderTmuxKeymapsMenu renders the Tmux keymap categories menu
func (m Model) renderTmuxKeymapsMenu() string {
	var s strings.Builder

	s.WriteString(TitleStyle.Render(m.GetScreenTitle()))
	s.WriteString("\n")
	s.WriteString(MutedStyle.Render("Select a category to view Tmux keybindings"))
	s.WriteString("\n\n")

	// Menu
	options := m.GetCurrentOptions()
	for i, opt := range options {
		if strings.HasPrefix(opt, "───") {
			s.WriteString(MutedStyle.Render(opt))
			s.WriteString("\n")
			continue
		}

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
	s.WriteString(HelpStyle.Render("↑/k up • ↓/j down • [Enter] select • [Esc/q] back"))

	return s.String()
}

// renderTmuxKeymapCategory renders a specific Tmux keymap category
func (m Model) renderTmuxKeymapCategory() string {
	var s strings.Builder

	if m.TmuxSelectedCategory >= len(m.TmuxKeymapCategories) {
		return ErrorStyle.Render("Category not found")
	}

	category := m.TmuxKeymapCategories[m.TmuxSelectedCategory]

	s.WriteString(TitleStyle.Render(category.Name))
	s.WriteString("\n")
	s.WriteString(MutedStyle.Render(category.Description))
	s.WriteString("\n\n")

	// Table header
	header := fmt.Sprintf("%-20s %-6s %s", "Keys", "Mode", "Description")
	s.WriteString(SubtitleStyle.Render(header))
	s.WriteString("\n")
	s.WriteString(MutedStyle.Render(strings.Repeat("─", 60)))
	s.WriteString("\n")

	// Calculate visible items
	visibleItems := m.Height - 9
	if visibleItems < 5 {
		visibleItems = 5
	}
	if visibleItems > len(category.Keymaps) {
		visibleItems = len(category.Keymaps)
	}

	// Keymaps with scrolling
	start := m.TmuxKeymapScroll
	end := start + visibleItems
	if end > len(category.Keymaps) {
		end = len(category.Keymaps)
		start = end - visibleItems
		if start < 0 {
			start = 0
		}
	}

	for i := start; i < end; i++ {
		km := category.Keymaps[i]
		s.WriteString(KeyStyle.Render(km.Keys))
		s.WriteString(MutedStyle.Render(fmt.Sprintf(" %-6s ", km.Mode)))
		s.WriteString(InfoStyle.Render(km.Description))
		s.WriteString("\n")
	}

	// Scroll indicator
	if len(category.Keymaps) > visibleItems {
		s.WriteString("\n")
		scrollInfo := fmt.Sprintf("Showing %d-%d of %d", start+1, end, len(category.Keymaps))
		s.WriteString(MutedStyle.Render(scrollInfo))
	}

	s.WriteString("\n\n")
	s.WriteString(HelpStyle.Render("↑/k up • ↓/j down • [Enter/Esc/q] back"))

	return s.String()
}

// renderZellijKeymapsMenu renders the Zellij keymap categories menu
func (m Model) renderZellijKeymapsMenu() string {
	var s strings.Builder

	s.WriteString(TitleStyle.Render(m.GetScreenTitle()))
	s.WriteString("\n")
	s.WriteString(MutedStyle.Render("Select a category to view Zellij keybindings"))
	s.WriteString("\n\n")

	// Menu
	options := m.GetCurrentOptions()
	for i, opt := range options {
		if strings.HasPrefix(opt, "───") {
			s.WriteString(MutedStyle.Render(opt))
			s.WriteString("\n")
			continue
		}

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
	s.WriteString(HelpStyle.Render("↑/k up • ↓/j down • [Enter] select • [Esc/q] back"))

	return s.String()
}

// renderZellijKeymapCategory renders a specific Zellij keymap category
func (m Model) renderZellijKeymapCategory() string {
	var s strings.Builder

	if m.ZellijSelectedCategory >= len(m.ZellijKeymapCategories) {
		return ErrorStyle.Render("Category not found")
	}

	category := m.ZellijKeymapCategories[m.ZellijSelectedCategory]

	s.WriteString(TitleStyle.Render(category.Name))
	s.WriteString("\n")
	s.WriteString(MutedStyle.Render(category.Description))
	s.WriteString("\n\n")

	// Table header
	header := fmt.Sprintf("%-15s %-8s %s", "Keys", "Mode", "Description")
	s.WriteString(SubtitleStyle.Render(header))
	s.WriteString("\n")
	s.WriteString(MutedStyle.Render(strings.Repeat("─", 60)))
	s.WriteString("\n")

	// Calculate visible items
	visibleItems := m.Height - 9
	if visibleItems < 5 {
		visibleItems = 5
	}
	if visibleItems > len(category.Keymaps) {
		visibleItems = len(category.Keymaps)
	}

	// Keymaps with scrolling
	start := m.ZellijKeymapScroll
	end := start + visibleItems
	if end > len(category.Keymaps) {
		end = len(category.Keymaps)
		start = end - visibleItems
		if start < 0 {
			start = 0
		}
	}

	for i := start; i < end; i++ {
		km := category.Keymaps[i]
		s.WriteString(KeyStyle.Render(km.Keys))
		s.WriteString(MutedStyle.Render(fmt.Sprintf(" %-8s ", km.Mode)))
		s.WriteString(InfoStyle.Render(km.Description))
		s.WriteString("\n")
	}

	// Scroll indicator
	if len(category.Keymaps) > visibleItems {
		s.WriteString("\n")
		scrollInfo := fmt.Sprintf("Showing %d-%d of %d", start+1, end, len(category.Keymaps))
		s.WriteString(MutedStyle.Render(scrollInfo))
	}

	s.WriteString("\n\n")
	s.WriteString(HelpStyle.Render("↑/k up • ↓/j down • [Enter/Esc/q] back"))

	return s.String()
}

// renderGhosttyKeymapsMenu renders the Ghostty keymap categories menu
func (m Model) renderGhosttyKeymapsMenu() string {
	var s strings.Builder

	s.WriteString(TitleStyle.Render(m.GetScreenTitle()))
	s.WriteString("\n")
	s.WriteString(MutedStyle.Render("Select a category to view Ghostty keybindings"))
	s.WriteString("\n\n")

	// Menu
	options := m.GetCurrentOptions()
	for i, opt := range options {
		if strings.HasPrefix(opt, "───") {
			s.WriteString(MutedStyle.Render(opt))
			s.WriteString("\n")
			continue
		}

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
	s.WriteString(HelpStyle.Render("↑/k up • ↓/j down • [Enter] select • [Esc/q] back"))

	return s.String()
}

// renderGhosttyKeymapCategory renders a specific Ghostty keymap category
func (m Model) renderGhosttyKeymapCategory() string {
	var s strings.Builder

	if m.GhosttySelectedCategory >= len(m.GhosttyKeymapCategories) {
		return ErrorStyle.Render("Category not found")
	}

	category := m.GhosttyKeymapCategories[m.GhosttySelectedCategory]

	s.WriteString(TitleStyle.Render(category.Name))
	s.WriteString("\n")
	s.WriteString(MutedStyle.Render(category.Description))
	s.WriteString("\n\n")

	// Table header
	header := fmt.Sprintf("%-18s %-6s %s", "Keys", "Mode", "Description")
	s.WriteString(SubtitleStyle.Render(header))
	s.WriteString("\n")
	s.WriteString(MutedStyle.Render(strings.Repeat("─", 60)))
	s.WriteString("\n")

	// Calculate visible items
	visibleItems := m.Height - 9
	if visibleItems < 5 {
		visibleItems = 5
	}
	if visibleItems > len(category.Keymaps) {
		visibleItems = len(category.Keymaps)
	}

	// Keymaps with scrolling
	start := m.GhosttyKeymapScroll
	end := start + visibleItems
	if end > len(category.Keymaps) {
		end = len(category.Keymaps)
		start = end - visibleItems
		if start < 0 {
			start = 0
		}
	}

	for i := start; i < end; i++ {
		km := category.Keymaps[i]
		s.WriteString(KeyStyle.Render(km.Keys))
		s.WriteString(MutedStyle.Render(fmt.Sprintf(" %-6s ", km.Mode)))
		s.WriteString(InfoStyle.Render(km.Description))
		s.WriteString("\n")
	}

	// Scroll indicator
	if len(category.Keymaps) > visibleItems {
		s.WriteString("\n")
		scrollInfo := fmt.Sprintf("Showing %d-%d of %d", start+1, end, len(category.Keymaps))
		s.WriteString(MutedStyle.Render(scrollInfo))
	}

	s.WriteString("\n\n")
	s.WriteString(HelpStyle.Render("↑/k up • ↓/j down • [Enter/Esc/q] back"))

	return s.String()
}

// renderHerdrKeymapsMenu renders the Herdr keymap categories menu
func (m Model) renderHerdrKeymapsMenu() string {
	var s strings.Builder

	s.WriteString(TitleStyle.Render(m.GetScreenTitle()))
	s.WriteString("\n")
	s.WriteString(MutedStyle.Render("Herdr is mouse-first; keyboard is optional. Select a category"))
	s.WriteString("\n\n")

	// Menu
	options := m.GetCurrentOptions()
	for i, opt := range options {
		if strings.HasPrefix(opt, "───") {
			s.WriteString(MutedStyle.Render(opt))
			s.WriteString("\n")
			continue
		}

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
	s.WriteString(HelpStyle.Render("↑/k up • ↓/j down • [Enter] select • [Esc/q] back"))

	return s.String()
}

// renderHerdrKeymapCategory renders a specific Herdr keymap category
func (m Model) renderHerdrKeymapCategory() string {
	var s strings.Builder

	if m.HerdrSelectedCategory >= len(m.HerdrKeymapCategories) {
		return ErrorStyle.Render("Category not found")
	}

	category := m.HerdrKeymapCategories[m.HerdrSelectedCategory]

	s.WriteString(TitleStyle.Render(category.Name))
	s.WriteString("\n")
	s.WriteString(MutedStyle.Render(category.Description))
	s.WriteString("\n\n")

	// Table header
	header := fmt.Sprintf("%-20s %-6s %s", "Keys", "Mode", "Description")
	s.WriteString(SubtitleStyle.Render(header))
	s.WriteString("\n")
	s.WriteString(MutedStyle.Render(strings.Repeat("─", 60)))
	s.WriteString("\n")

	// Calculate visible items
	visibleItems := m.Height - 9
	if visibleItems < 5 {
		visibleItems = 5
	}
	if visibleItems > len(category.Keymaps) {
		visibleItems = len(category.Keymaps)
	}

	// Keymaps with scrolling
	start := m.HerdrKeymapScroll
	end := start + visibleItems
	if end > len(category.Keymaps) {
		end = len(category.Keymaps)
		start = end - visibleItems
		if start < 0 {
			start = 0
		}
	}

	for i := start; i < end; i++ {
		km := category.Keymaps[i]
		s.WriteString(KeyStyle.Render(km.Keys))
		s.WriteString(MutedStyle.Render(fmt.Sprintf(" %-6s ", km.Mode)))
		s.WriteString(InfoStyle.Render(km.Description))
		s.WriteString("\n")
	}

	// Scroll indicator
	if len(category.Keymaps) > visibleItems {
		s.WriteString("\n")
		scrollInfo := fmt.Sprintf("Showing %d-%d of %d", start+1, end, len(category.Keymaps))
		s.WriteString(MutedStyle.Render(scrollInfo))
	}

	s.WriteString("\n\n")
	s.WriteString(HelpStyle.Render("↑/k up • ↓/j down • [Enter/Esc/q] back"))

	return s.String()
}

func (m Model) renderLazyVimMenu() string {
	var s strings.Builder

	s.WriteString(TitleStyle.Render(m.GetScreenTitle()))
	s.WriteString("\n")
	s.WriteString(MutedStyle.Render("Learn how to use and customize LazyVim"))
	s.WriteString("\n\n")

	// Menu
	options := m.GetCurrentOptions()
	for i, opt := range options {
		if strings.HasPrefix(opt, "───") {
			s.WriteString(MutedStyle.Render(opt))
			s.WriteString("\n")
			continue
		}

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
	s.WriteString(HelpStyle.Render("↑/k up • ↓/j down • [Enter] select • [Esc/q] back"))

	return s.String()
}

func (m Model) renderLazyVimTopic() string {
	var s strings.Builder

	if m.SelectedLazyVimTopic >= len(m.LazyVimTopics) {
		return ErrorStyle.Render("Topic not found")
	}

	topic := m.LazyVimTopics[m.SelectedLazyVimTopic]

	s.WriteString(TitleStyle.Render(topic.Title))
	s.WriteString("\n")
	s.WriteString(SubtitleStyle.Render(topic.Description))
	s.WriteString("\n\n")

	// Build all content
	var allLines []string

	// Content
	allLines = append(allLines, topic.Content...)
	allLines = append(allLines, "") // Empty line

	// Code example
	if topic.CodeExample != "" {
		allLines = append(allLines, "📝 Example:")
		allLines = append(allLines, "")
		codeLines := strings.Split(topic.CodeExample, "\n")
		allLines = append(allLines, codeLines...)
		allLines = append(allLines, "") // Empty line
	}

	// Tips
	if len(topic.Tips) > 0 {
		allLines = append(allLines, "💡 Tips:")
		for _, tip := range topic.Tips {
			allLines = append(allLines, "  • "+tip)
		}
	}

	// Calculate view height based on terminal size
	// Reserve space for: title(1) + description(1) + blank(2) + scroll info(2) + help(2) = 8 lines
	viewHeight := m.Height - 8
	if viewHeight < 10 {
		viewHeight = 10 // Minimum
	}

	// Apply scrolling
	start := m.LazyVimScroll
	end := start + viewHeight
	if end > len(allLines) {
		end = len(allLines)
	}
	if start > len(allLines) {
		start = 0
	}

	for i := start; i < end; i++ {
		line := allLines[i]
		// Style code lines differently
		if strings.HasPrefix(line, "--") || strings.HasPrefix(line, "local") ||
			strings.HasPrefix(line, "return") || strings.HasPrefix(line, "{") ||
			strings.HasPrefix(line, "}") || strings.HasPrefix(line, "  ") ||
			strings.HasPrefix(line, "map(") || strings.HasPrefix(line, "vim.") ||
			strings.HasPrefix(line, "require") {
			s.WriteString(CodeStyle.Render(line))
		} else if strings.HasPrefix(line, "📝") || strings.HasPrefix(line, "💡") {
			s.WriteString(SubtitleStyle.Render(line))
		} else if strings.HasPrefix(line, "  •") {
			s.WriteString(InfoStyle.Render(line))
		} else if strings.HasPrefix(line, "•") {
			s.WriteString(MutedStyle.Render(line))
		} else {
			s.WriteString(InfoStyle.Render(line))
		}
		s.WriteString("\n")
	}

	// Scroll indicator
	if len(allLines) > viewHeight {
		s.WriteString("\n")
		scrollInfo := fmt.Sprintf("Lines %d-%d of %d (↑↓ to scroll, PgUp/PgDn for fast scroll)", start+1, end, len(allLines))
		s.WriteString(MutedStyle.Render(scrollInfo))
	}

	s.WriteString("\n\n")
	s.WriteString(HelpStyle.Render("↑/k up • ↓/j down • PgUp/PgDn • [Enter/Esc/q] back"))

	return s.String()
}

// Spinner frames for running steps
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (m Model) renderInstalling() string {
	var s strings.Builder

	s.WriteString(TitleStyle.Render("🚀 Installing dotfiles"))
	s.WriteString("\n\n")

	// Progress steps
	for i, step := range m.Steps {
		var icon string
		var style lipgloss.Style

		switch step.Status {
		case StatusPending:
			icon = "○"
			style = MutedStyle
		case StatusRunning:
			// Animated spinner
			icon = spinnerFrames[m.SpinnerFrame%len(spinnerFrames)]
			style = WarningStyle
		case StatusDone:
			icon = "✓"
			style = SuccessStyle
		case StatusFailed:
			icon = "✗"
			style = ErrorStyle
		case StatusSkipped:
			icon = "⊘"
			style = MutedStyle
		}

		line := fmt.Sprintf("%s %s", icon, step.Name)
		s.WriteString(style.Render(line))
		s.WriteString("\n")

		// Show current step description
		if i == m.CurrentStep && step.Status == StatusRunning {
			s.WriteString(MutedStyle.Render("   " + step.Description))
			s.WriteString("\n")
		}
	}

	// Log output if details enabled
	if m.ShowDetails && len(m.LogLines) > 0 {
		s.WriteString("\n")
		s.WriteString(BoxStyle.Render(strings.Join(m.LogLines[max(0, len(m.LogLines)-10):], "\n")))
	}

	s.WriteString("\n")
	s.WriteString(HelpStyle.Render("[space+d] toggle details"))

	return s.String()
}

func (m Model) renderComplete() string {
	var s strings.Builder

	s.WriteString(SuccessStyle.Render("✨ Installation Complete! ✨"))
	s.WriteString("\n\n")

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
		s.WriteString(InfoStyle.Render("  • " + item))
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
	s.WriteString("\n\n")

	s.WriteString(InfoStyle.Render("To use your new shell now, run:"))
	s.WriteString("\n")
	s.WriteString(HighlightStyle.Render(fmt.Sprintf("   exec %s", shellCmd)))
	s.WriteString("\n\n")

	s.WriteString(HelpStyle.Render("Press [Enter] or [q] to exit"))

	return s.String()
}

func (m Model) renderError() string {
	var s strings.Builder

	s.WriteString(ErrorStyle.Render("❌ Installation Failed"))
	s.WriteString("\n\n")

	s.WriteString(MutedStyle.Render("Error:"))
	s.WriteString("\n")
	s.WriteString(ErrorStyle.Render(m.ErrorMsg))
	s.WriteString("\n\n")

	// Show last few log lines for context
	if len(m.LogLines) > 0 {
		s.WriteString(MutedStyle.Render("Recent logs:"))
		s.WriteString("\n")
		// Show last 5 log lines
		startIdx := len(m.LogLines) - 5
		if startIdx < 0 {
			startIdx = 0
		}
		for _, line := range m.LogLines[startIdx:] {
			s.WriteString(InfoStyle.Render("  " + line))
			s.WriteString("\n")
		}
		s.WriteString("\n")
	}

	s.WriteString(HelpStyle.Render("[r] retry • [space+q] quit"))

	return s.String()
}

func (m Model) renderBackupConfirm() string {
	var s strings.Builder

	s.WriteString(TitleStyle.Render(m.GetScreenTitle()))
	s.WriteString("\n")
	s.WriteString(MutedStyle.Render("The following configs will be overwritten:"))
	s.WriteString("\n\n")

	// List existing configs
	for _, config := range m.ExistingConfigs {
		s.WriteString(WarningStyle.Render("  ⚠️  " + config))
		s.WriteString("\n")
	}

	s.WriteString("\n")
	s.WriteString(InfoStyle.Render("Creating a backup allows you to restore later if needed."))
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
	s.WriteString(HelpStyle.Render("↑/k up • ↓/j down • [Enter] select • [Esc] back"))

	return s.String()
}

func (m Model) renderRestoreBackup() string {
	var s strings.Builder

	s.WriteString(TitleStyle.Render(m.GetScreenTitle()))
	s.WriteString("\n")
	s.WriteString(MutedStyle.Render("Select a backup to restore or delete"))
	s.WriteString("\n\n")

	if len(m.AvailableBackups) == 0 {
		s.WriteString(MutedStyle.Render("No backups found."))
		s.WriteString("\n")
	} else {
		// List backups
		for i, backup := range m.AvailableBackups {
			cursor := "  "
			style := UnselectedStyle
			if i == m.Cursor {
				cursor = "▸ "
				style = SelectedStyle
			}

			// Format: timestamp + item count
			label := fmt.Sprintf("📁 %s (%d items)", backup.Timestamp.Format("2006-01-02 15:04:05"), len(backup.Files))
			s.WriteString(style.Render(cursor + label))
			s.WriteString("\n")
		}
	}

	// Separator and Back
	s.WriteString(MutedStyle.Render("─────────────"))
	s.WriteString("\n")

	backIdx := len(m.AvailableBackups) + 1
	cursor := "  "
	style := UnselectedStyle
	if m.Cursor == backIdx {
		cursor = "▸ "
		style = SelectedStyle
	}
	s.WriteString(style.Render(cursor + "← Back"))
	s.WriteString("\n")

	s.WriteString("\n")
	s.WriteString(HelpStyle.Render("↑/k up • ↓/j down • [Enter] select • [Esc] back"))

	return s.String()
}

func (m Model) renderRestoreConfirm() string {
	var s strings.Builder

	if m.SelectedBackup >= len(m.AvailableBackups) {
		return ErrorStyle.Render("No backup selected")
	}

	backup := m.AvailableBackups[m.SelectedBackup]

	s.WriteString(TitleStyle.Render(m.GetScreenTitle()))
	s.WriteString("\n")
	s.WriteString(MutedStyle.Render("Backup from: " + backup.Timestamp.Format("2006-01-02 15:04:05")))
	s.WriteString("\n\n")

	// List files in backup
	s.WriteString(SubtitleStyle.Render("Contents:"))
	s.WriteString("\n")
	for _, file := range backup.Files {
		s.WriteString(InfoStyle.Render("  • " + file))
		s.WriteString("\n")
	}

	s.WriteString("\n")
	s.WriteString(WarningStyle.Render("⚠️  Restoring will overwrite your current configs!"))
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
	s.WriteString(HelpStyle.Render("↑/k up • ↓/j down • [Enter] select • [Esc] cancel"))

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

// The labels and legends the two exercise screens share. They are one string
// each so the lesson, practice and boss screens cannot drift apart on the keys
// they promise: Ctrl-e types the token an insert answer needs to leave insert
// mode, Esc is the trainer's own exit key, Backspace is an input edit the
// engine never sees, and PgUp/PgDn scroll the code window. The scroll keys are
// key names rather than printable characters, so they cannot collide with
// typing, with the hint key, with submit or with back.
const (
	trainerAnswerLabel     = "⌨️  Your answer: "
	trainerExerciseHelpOne = "Type command • [Enter] submit • [Tab] hint • [PgUp/PgDn] scroll"
	trainerExerciseHelpTwo = "[Backspace] delete • [Ctrl-e] type " + trainer.EscToken + " • [Esc] quit"
	trainerBossHelpOne     = "Type command • [Enter] submit • [Ctrl-e] type " + trainer.EscToken
	trainerBossHelpTwo     = "[PgUp/PgDn] scroll • [Esc] forfeit"
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

// trainerInnerWidth is the columns a trainer screen can render in: the model
// width minus the two columns of left and right padding View() applies to every
// screen.
func trainerInnerWidth(m Model) int {
	inner := m.Width - 4
	if inner < 20 {
		inner = 20
	}
	return inner
}

// trainerCurrentExercise returns the exercise the exercise or boss screen is
// showing, or nil when there is none.
func (m Model) trainerCurrentExercise() *trainer.Exercise {
	if m.TrainerGameState == nil {
		return nil
	}
	return m.TrainerGameState.CurrentExercise
}

// rule renders the separator a trainer code block is wrapped in. It replaces
// the 60-column rule the exercise and boss screens each wrote out by hand, so
// the two cannot drift apart and the rule follows the frame instead of being
// fixed at a width the frame may not have.
func rule(width int) string {
	if width < 1 {
		width = 1
	}
	return MutedStyle.Render(strings.Repeat("─", width))
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

	// Legend, one row per line, so the menu's height is countable.
	s.WriteString(TrainerHelpStyle.Render("↑/k up • ↓/j down • [Enter/l] lesson • [p] practice • [b] boss"))
	s.WriteString("\n")
	s.WriteString(TrainerHelpStyle.Render("[r] reset module • [R] reset all • [q/Esc] back"))

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
		return ErrorStyle.Render("No exercise loaded")
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

	// Help. The two lines go through TrainerHelpStyle, one row each, so the
	// screen's height is countable: see trainerFrameRows.
	rows = append(rows,
		"",
		TrainerHelpStyle.Render(trainerExerciseHelpOne),
		TrainerHelpStyle.Render(trainerExerciseHelpTwo),
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
		return ErrorStyle.Render("No boss loaded")
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
		DangerStyle.Render("⚔️  BOSS FIGHT: " + boss.Name),
		lives,
		countdown,
		"",
		SubtitleStyle.Render("📋 Challenge:"),
	}

	exercise := state.CurrentExercise
	if currentStep >= len(boss.Steps) || exercise == nil {
		// No step is on screen, so there is no code window to size around.
		rows = append(rows, m.trainerFeedbackRows(WarningStyle)...)
		rows = append(rows, "", TrainerHelpStyle.Render(trainerBossHelpOne), TrainerHelpStyle.Render(trainerBossHelpTwo))
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
	rows = append(rows, "", TrainerHelpStyle.Render(trainerBossHelpOne), TrainerHelpStyle.Render(trainerBossHelpTwo))

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
	s.WriteString(HelpStyle.Render("[Enter] continue • [Esc] back"))

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
	s.WriteString(HelpStyle.Render("[Enter/Space/Esc/q] return to menu"))

	return s.String()
}
