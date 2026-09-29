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

	// Box styles
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BorderActive).
			Padding(1, 2)

	// Progress bar styles. The installing screen is the one place the whole run's
	// progress is visible at a glance. Its filled part is brand chrome, not a
	// status colour: progress is not a state, and Success is reserved for the
	// steps that are actually done. The bar's cells are glyphs (█ and ░) rather
	// than filled backgrounds, so these styles carry the colour only and the bar
	// still reads with no colour at all.
	ProgressBarFilled = lipgloss.NewStyle().
				Foreground(Brand)

	ProgressBarEmpty = lipgloss.NewStyle().
				Foreground(TextMuted)

	// Help style. The installer's footer draws through AccentKeyStyle and
	// HelpVerbStyle now; HelpStyle remains for deadEnd's one-line way out.
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

	// TrainerHelpStyle is HelpStyle without its margin. The trainer screens budget
	// one terminal row per element, so the blank line above a legend is an element
	// of its own there instead of a margin the row count cannot see. The menu rows
	// and the trainer title used to have styles of their own here; they go through
	// the shared rowBar, headerRow and chip now, so those names were removed rather
	// than left as unused choices no screen can be checked against.
	TrainerHelpStyle = lipgloss.NewStyle().
				Foreground(TextMuted).
				Italic(true)

	// --- Semantic roles ---------------------------------------------------
	//
	// These names are the design's vocabulary, and they are the only names the
	// installer's own screens reach for a colour through. The palette above does
	// not change: each role resolves to the one palette entry it may use, so
	// "one meaning per colour" is enforced by the name a screen asks for rather
	// than by a comment beside a hex value. The trainer screens now reach for these
	// roles too: their menu rows are the shared rowBar, their titles are headerRow
	// and their blocks are chip plus gutteredBlock, so the frozen style names the
	// previous slice left them no longer exist.
	//
	//   Brand     chrome: the header title and a section chip
	//   BrandSoft the bar behind a selected row
	//   Paper     the cut-out text on that bar
	//   Ink       body text
	//   InkDim    metadata, help verbs, line numbers and trailing meters
	//   Rule      every separator and gutter
	//
	// Rule and InkDim are the same quiet slate on purpose: a separator and a line
	// of metadata are both "not content", and the shared rule() every screen draws
	// through is already that slate, so naming a second dim tone here would have
	// invented a distinction no reader could see.
	Brand     = Primary
	BrandSoft = Primary
	Paper     = Background
	Ink       = Text
	InkDim    = TextMuted
	Rule      = TextMuted

	// BrandStyle is chrome text: the frame's header title and a section chip.
	BrandStyle = lipgloss.NewStyle().
			Foreground(Brand).
			Bold(true)

	// RuleStyle draws a separator or a block's gutter in the rule tone.
	RuleStyle = lipgloss.NewStyle().
			Foreground(Rule)

	// InkStyle is plain body text at the row's own indent.
	InkStyle = lipgloss.NewStyle().
			Foreground(Ink)

	// AccentKeyStyle is a key token: the one thing to act on now.
	AccentKeyStyle = lipgloss.NewStyle().
			Foreground(Accent).
			Bold(true)

	// HelpVerbStyle is the verb a footer key token does.
	HelpVerbStyle = lipgloss.NewStyle().
			Foreground(InkDim)

	// MeterFilledStyle is the filled part of a progress meter: brand chrome, the
	// same role the filled part of the step counter takes.
	MeterFilledStyle = lipgloss.NewStyle().
				Foreground(Brand)

	// MeterStyle is a meter's empty cells and its trailing count: metadata.
	MeterStyle = lipgloss.NewStyle().
			Foreground(InkDim)

	// MeterOnBarStyle is a trailing meter drawn on a selected row's bar, so the
	// bar's background runs under it.
	MeterOnBarStyle = lipgloss.NewStyle().
			Foreground(InkDim).
			Background(BrandSoft)

	// RowBarStyle is the body of a selected row: Paper on the BrandSoft bar.
	RowBarStyle = lipgloss.NewStyle().
			Foreground(Paper).
			Background(BrandSoft).
			Bold(true)

	// RowBarMarkerStyle is the ▸ that marks the selected row. It shares the bar's
	// background so the marker sits inside the bar rather than beside it.
	RowBarMarkerStyle = lipgloss.NewStyle().
				Foreground(Accent).
				Background(BrandSoft).
				Bold(true)
)

// CenterHorizontally centers text horizontally within a given width
func CenterHorizontally(text string, width int) string {
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(text)
}
