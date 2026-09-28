package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors - dotfiles Theme (from opencode theme).
	//
	// Only the colours a screen actually renders live here. A constant nothing
	// references is dead weight in a palette: it reads as an available choice
	// while no screen can be checked against it.

	// Base color. It is also the colour text takes on top of a filled cursor or
	// selection block, which is why CursorText names it instead of repeating the
	// hex: three cursor styles used to spell "#06080f" out.
	Background = lipgloss.Color("#06080f")

	// Text colors
	Text      = lipgloss.Color("#F3F6F9")
	TextMuted = lipgloss.Color("#5C6170")

	// Accent colors
	Primary   = lipgloss.Color("#7FB4CA") // Blue-ish
	Secondary = lipgloss.Color("#A3B5D6") // Light blue
	Accent    = lipgloss.Color("#E0C15A") // Gold/Yellow

	// Status colors
	Error   = lipgloss.Color("#CB7C94") // Pink-red
	Warning = lipgloss.Color("#DEBA87") // Orange-tan
	Success = lipgloss.Color("#B7CC85") // Green
	Info    = lipgloss.Color("#7FB4CA") // Blue

	// Border colors
	BorderActive = lipgloss.Color("#7FB4CA")

	// Syntax colors (for code display)
	SyntaxKeyword = lipgloss.Color("#C99AD6") // Purple
	SyntaxString  = lipgloss.Color("#DFBD76") // Gold

	// Text styles
	TitleStyle = lipgloss.NewStyle().
			Foreground(Primary).
			Bold(true).
			MarginBottom(1)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(Secondary).
			Italic(true)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(Success).
			Bold(true)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(Error).
			Bold(true)

	WarningStyle = lipgloss.NewStyle().
			Foreground(Warning)

	InfoStyle = lipgloss.NewStyle().
			Foreground(Info)

	MutedStyle = lipgloss.NewStyle().
			Foreground(TextMuted)

	// Selection styles
	SelectedStyle = lipgloss.NewStyle().
			Foreground(Accent).
			Bold(true).
			PaddingLeft(2)

	UnselectedStyle = lipgloss.NewStyle().
			Foreground(Text).
			PaddingLeft(4)

	// Box styles
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BorderActive).
			Padding(1, 2)

	// Progress bar styles. They were deleted as dead surface in the trainer slice
	// because nothing rendered a bar; the installing screen renders one now, and
	// it is the one place the whole run's progress is visible. The bar's cells are
	// glyphs (█ and ░) rather than filled backgrounds, so these styles carry the
	// colour only and the bar still reads with no colour at all.
	ProgressBarFilled = lipgloss.NewStyle().
				Foreground(Success)

	ProgressBarEmpty = lipgloss.NewStyle().
				Foreground(TextMuted)

	// Logo style
	LogoStyle = lipgloss.NewStyle().
			Foreground(Primary).
			Bold(true)

	// Step indicator
	StepActiveStyle = lipgloss.NewStyle().
			Foreground(Accent).
			Bold(true)

	StepDoneStyle = lipgloss.NewStyle().
			Foreground(Success)

	StepPendingStyle = lipgloss.NewStyle().
				Foreground(TextMuted)

	// Help style
	HelpStyle = lipgloss.NewStyle().
			Foreground(TextMuted).
			Italic(true).
			MarginTop(1)

	// Keymaps style
	KeyStyle = lipgloss.NewStyle().
			Foreground(SyntaxKeyword).
			Bold(true)

	// Code style
	CodeStyle = lipgloss.NewStyle().
			Foreground(SyntaxString)

	DangerStyle = lipgloss.NewStyle().
			Foreground(Error).
			Bold(true)

	HighlightStyle = lipgloss.NewStyle().
			Foreground(Accent).
			Bold(true)

	// CursorText is the text colour used on top of a filled cursor or selection
	// block: the terminal background, so the block reads as a cut-out. It is a
	// name for the palette's own base colour rather than a fourth copy of the
	// hex, which is what the cursor styles used to carry.
	CursorText = Background

	// Vim Trainer cursor styles
	StartCursorStyle = lipgloss.NewStyle().
				Foreground(CursorText).
				Background(Warning).
				Bold(true)

	CurrentCursorStyle = lipgloss.NewStyle().
				Foreground(CursorText).
				Background(Success).
				Bold(true)

	// Visual selection style (like Vim's visual mode). Its background is the
	// palette's own blue: "#7aa2f7" was the one colour outside this theme, and it
	// read as a stranger next to the accents it sits between.
	SelectionStyle = lipgloss.NewStyle().
			Foreground(CursorText).
			Background(Primary).
			Bold(false)

	// Vim Trainer menu rows. Every row state -- selected, unselected and locked
	// -- has to start on the same column, so these styles carry no padding: the
	// row's own marker column is what separates the rows. The shared menu styles
	// keep their padding for the installer's own menus.
	TrainerRowStyle = lipgloss.NewStyle().
			Foreground(Text)

	TrainerRowSelectedStyle = lipgloss.NewStyle().
				Foreground(Accent).
				Bold(true)

	TrainerRowLockedStyle = lipgloss.NewStyle().
				Foreground(TextMuted)

	// TrainerTitleStyle and TrainerHelpStyle are TitleStyle and HelpStyle without
	// their margins. The trainer screens budget one terminal row per element, so
	// the blank line above a legend and below a title is an element of its own
	// there instead of a margin the row count cannot see.
	TrainerTitleStyle = lipgloss.NewStyle().
				Foreground(Primary).
				Bold(true)

	TrainerHelpStyle = lipgloss.NewStyle().
				Foreground(TextMuted).
				Italic(true)
)

// CenterHorizontally centers text horizontally within a given width
func CenterHorizontally(text string, width int) string {
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(text)
}

// CenterVertically centers text vertically within a given height
func CenterVertically(text string, height int) string {
	lines := lipgloss.Height(text)
	if lines >= height {
		return text
	}

	topPadding := (height - lines) / 2
	return lipgloss.NewStyle().PaddingTop(topPadding).Render(text)
}

// CenterBoth centers text both horizontally and vertically using lipgloss.Place
func CenterBoth(text string, width, height int) string {
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, text)
}
