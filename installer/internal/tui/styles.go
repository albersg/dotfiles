package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors - dotfiles Theme (from opencode theme).
	//
	// Only the colours a screen actually renders live here. A constant nothing
	// references is dead weight in a palette: it reads as an available choice
	// while no screen can be checked against it.
	//
	// Every colour is adaptive, and that is the fix for a real defect rather than
	// a nicety: the palette used to be near-white body text on whatever the
	// terminal's background happened to be, so on a light terminal the text was
	// effectively invisible. lipgloss.AdaptiveColor asks the terminal whether its
	// background is dark (through the same termenv detection the rest of the
	// stack uses) and picks the matching field, so no screen carries its own
	// detection. The Dark field keeps the shipped theme exactly; the Light field
	// is a deliberately darker member of the same hue family, chosen so each
	// colour keeps at least a 4.5:1 contrast ratio on a white background. The
	// hex values degrade to the terminal's profile the way any lipgloss colour
	// does, and the glyphs and words -- not the colours -- carry the state, so a
	// 16-colour or no-colour terminal loses no information.

	// Base color. It is also the colour text takes on top of a filled cursor or
	// selection block, which is why CursorText names it instead of repeating the
	// hex: three cursor styles used to spell "#06080f" out. On a light terminal
	// the block backgrounds become the darker Light palette entries, so the text
	// on them flips to the light end here.
	Background = lipgloss.AdaptiveColor{Light: "#F7F9FC", Dark: "#06080f"}

	// Text colors. Text is the dark ink the light terminal needed: #1F2430 on
	// white is about 15:1, where the old near-white was unreadable. TextMuted is
	// the same slate darkened until it clears 4.5:1.
	Text      = lipgloss.AdaptiveColor{Light: "#1F2430", Dark: "#F3F6F9"}
	TextMuted = lipgloss.AdaptiveColor{Light: "#5A6275", Dark: "#5C6170"}

	// Accent colors. The dark pastels all fail on white, so each Light entry is
	// the same hue taken dark enough to read: blue, blue-grey and amber.
	Primary   = lipgloss.AdaptiveColor{Light: "#2E6E8E", Dark: "#7FB4CA"} // Blue-ish
	Secondary = lipgloss.AdaptiveColor{Light: "#4A5D80", Dark: "#A3B5D6"} // Light blue
	Accent    = lipgloss.AdaptiveColor{Light: "#8A6A00", Dark: "#E0C15A"} // Gold/Yellow

	// Status colors. Rose, amber, green and blue, each darkened for light.
	Error   = lipgloss.AdaptiveColor{Light: "#B0325A", Dark: "#CB7C94"} // Pink-red
	Warning = lipgloss.AdaptiveColor{Light: "#8A5A00", Dark: "#DEBA87"} // Orange-tan
	Success = lipgloss.AdaptiveColor{Light: "#3F7A1E", Dark: "#B7CC85"} // Green
	Info    = lipgloss.AdaptiveColor{Light: "#2E6E8E", Dark: "#7FB4CA"} // Blue

	// Border colors
	BorderActive = lipgloss.AdaptiveColor{Light: "#2E6E8E", Dark: "#7FB4CA"}

	// Syntax colors (for code display)
	SyntaxKeyword = lipgloss.AdaptiveColor{Light: "#7A3E9E", Dark: "#C99AD6"} // Purple
	SyntaxString  = lipgloss.AdaptiveColor{Light: "#8A6A00", Dark: "#DFBD76"} // Gold

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
	// block: the palette's base colour, so the block reads as a cut-out. It is a
	// name for Background rather than a fourth copy of the hex, which is what the
	// cursor styles used to carry. It flips with the theme, so the text still
	// contrasts against the darker Light entries those blocks take on a light
	// terminal.
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
