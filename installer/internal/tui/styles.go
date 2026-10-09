package tui

import "github.com/charmbracelet/lipgloss"

// The installer's own palette.
//
// Every style is built from a uiColors value, and the package-level vars below
// are that build's output. There is one builder, applyUIColors, so the default
// chrome and the live preview cannot drift: the preview calls the same builder
// with another theme's colours, and the default calls it with the dotfiles
// palette.
//
// Only the colours a screen actually renders are here. A constant nothing
// references is dead weight in a palette: it reads as an available choice while
// no screen can be checked against it.
//
// The defaults are adaptive, and that is the fix for a real defect rather than a
// nicety: the palette used to be near-white body text on whatever the terminal's
// background happened to be, so on a light terminal the text was effectively
// invisible. lipgloss.AdaptiveColor asks the terminal whether its background is
// dark (through the same termenv detection the rest of the stack uses) and picks
// the matching field, so no screen carries its own detection. The Dark field
// keeps the shipped theme exactly; the Light field is a deliberately darker
// member of the same hue family, chosen so each colour keeps at least a 4.5:1
// contrast ratio on a white background. The hex values degrade to the terminal's
// profile the way any lipgloss colour does, and the glyphs and words -- not the
// colours -- carry the state, so a 16-colour or no-colour terminal loses no
// information.

// uiColors is the installer's palette as data: the input every style is built
// from. The fields are TerminalColor rather than AdaptiveColor so the live
// preview can put a plain colour in the same slot while the defaults stay
// adaptive.
type uiColors struct {
	Background    lipgloss.AdaptiveColor
	Text          lipgloss.AdaptiveColor
	TextMuted     lipgloss.AdaptiveColor
	Primary       lipgloss.AdaptiveColor
	Secondary     lipgloss.AdaptiveColor
	Accent        lipgloss.AdaptiveColor
	Error         lipgloss.AdaptiveColor
	Warning       lipgloss.AdaptiveColor
	Success       lipgloss.AdaptiveColor
	Info          lipgloss.AdaptiveColor
	BorderActive  lipgloss.AdaptiveColor
	SyntaxKeyword lipgloss.AdaptiveColor
	SyntaxString  lipgloss.AdaptiveColor
	// OnFill is the ink a label drawn on a filled block uses -- the selected row's
	// bar, the cursor and the status blocks. It is a field of its own rather than
	// an alias of Background so an applied theme can choose an ink that stays
	// readable on its own fills; the default is the base colour, exactly as
	// CursorText and Paper named it before.
	OnFill lipgloss.AdaptiveColor
}

// defaultUIColors is the shipped palette. The Dark entries are the dotfiles
// theme's own colours; TestTheDefaultStylesMatchTheDotfilesDefinition pins them
// against themes/dotfiles.toml, so this block is a consumer and not an eighth
// hand-written copy of the palette.
func defaultUIColors() uiColors {
	c := uiColors{
		// Base colour. It is also the colour text takes on top of a filled cursor
		// or selection block, which is why CursorText names it instead of repeating
		// the hex: three cursor styles used to spell "#06080f" out. On a light
		// terminal the block backgrounds become the darker Light palette entries,
		// so the text on them flips to the light end there.
		Background: lipgloss.AdaptiveColor{Light: "#F7F9FC", Dark: "#06080f"},

		// Text is the dark ink the light terminal needed: #1F2430 on white is about
		// 15:1, where the old near-white was unreadable. TextMuted is the same slate
		// darkened until it clears 4.5:1.
		Text:      lipgloss.AdaptiveColor{Light: "#1F2430", Dark: "#F3F6F9"},
		TextMuted: lipgloss.AdaptiveColor{Light: "#5A6275", Dark: "#5C6170"},

		// Accents. The dark pastels all fail on white, so each Light entry is the
		// same hue taken dark enough to read: blue, blue-grey and amber.
		Primary:   lipgloss.AdaptiveColor{Light: "#2E6E8E", Dark: "#7FB4CA"}, // Blue-ish
		Secondary: lipgloss.AdaptiveColor{Light: "#4A5D80", Dark: "#A3B5D6"}, // Light blue
		Accent:    lipgloss.AdaptiveColor{Light: "#8A6A00", Dark: "#E0C15A"}, // Gold/Yellow

		// Status colours. Rose, amber, green and blue, each darkened for light.
		Error:   lipgloss.AdaptiveColor{Light: "#B0325A", Dark: "#CB7C94"}, // Pink-red
		Warning: lipgloss.AdaptiveColor{Light: "#8A5A00", Dark: "#DEBA87"}, // Orange-tan
		Success: lipgloss.AdaptiveColor{Light: "#3F7A1E", Dark: "#B7CC85"}, // Green
		Info:    lipgloss.AdaptiveColor{Light: "#2E6E8E", Dark: "#7FB4CA"}, // Blue

		BorderActive: lipgloss.AdaptiveColor{Light: "#2E6E8E", Dark: "#7FB4CA"},

		// Syntax colours (for code display). The two pairs are declared in
		// themes/dotfiles.toml's [syntax] table and pinned there by
		// TestTheDefaultStylesMatchTheDotfilesDefinition, so they are theme roles now
		// and not two values this file owns.
		SyntaxKeyword: lipgloss.AdaptiveColor{Light: "#7A3E9E", Dark: "#C99AD6"},
		SyntaxString:  lipgloss.AdaptiveColor{Light: "#8A6A00", Dark: "#DFBD76"},
	}
	// The default chrome's filled blocks draw their label in the base colour: the
	// dark base is the ink that reads on the pastel fills. An applied theme may
	// choose otherwise when its own base cannot label a fill.
	c.OnFill = c.Background
	return c
}

// The palette's names, and the semantic roles every screen reaches a colour
// through.
//
//	Brand     chrome: the header title and a section chip
//	BrandSoft the bar behind a selected row
//	Paper     the cut-out text on that bar
//	Ink       body text
//	InkDim    metadata, help verbs, line numbers and trailing meters
//	Rule      every separator and gutter
//
// Rule and InkDim are the same quiet slate on purpose: a separator and a line
// of metadata are both "not content", and the shared rule() every screen draws
// through is already that slate, so naming a second dim tone here would have
// invented a distinction no reader could see.
var (
	Background    lipgloss.AdaptiveColor
	Text          lipgloss.AdaptiveColor
	TextMuted     lipgloss.AdaptiveColor
	Primary       lipgloss.AdaptiveColor
	Secondary     lipgloss.AdaptiveColor
	Accent        lipgloss.AdaptiveColor
	Error         lipgloss.AdaptiveColor
	Warning       lipgloss.AdaptiveColor
	Success       lipgloss.AdaptiveColor
	Info          lipgloss.AdaptiveColor
	BorderActive  lipgloss.AdaptiveColor
	SyntaxKeyword lipgloss.AdaptiveColor
	SyntaxString  lipgloss.AdaptiveColor

	CursorText lipgloss.AdaptiveColor
	Brand      lipgloss.AdaptiveColor
	BrandSoft  lipgloss.AdaptiveColor
	Paper      lipgloss.AdaptiveColor
	Ink        lipgloss.AdaptiveColor
	InkDim     lipgloss.AdaptiveColor
	Rule       lipgloss.AdaptiveColor
)

// The styles, rebuilt by applyUIColors.
var (
	TitleStyle    lipgloss.Style
	SubtitleStyle lipgloss.Style
	SuccessStyle  lipgloss.Style
	ErrorStyle    lipgloss.Style
	WarningStyle  lipgloss.Style
	InfoStyle     lipgloss.Style
	MutedStyle    lipgloss.Style
	BoxStyle      lipgloss.Style

	ProgressBarFilled lipgloss.Style
	ProgressBarEmpty  lipgloss.Style

	HelpStyle lipgloss.Style
	KeyStyle  lipgloss.Style
	CodeStyle lipgloss.Style

	DangerStyle    lipgloss.Style
	HighlightStyle lipgloss.Style

	StartCursorStyle   lipgloss.Style
	CurrentCursorStyle lipgloss.Style
	SelectionStyle     lipgloss.Style

	BrandStyle        lipgloss.Style
	RuleStyle         lipgloss.Style
	InkStyle          lipgloss.Style
	AccentKeyStyle    lipgloss.Style
	HelpVerbStyle     lipgloss.Style
	MeterFilledStyle  lipgloss.Style
	MeterStyle        lipgloss.Style
	MeterOnBarStyle   lipgloss.Style
	CompanionStyle    lipgloss.Style
	RowBarStyle       lipgloss.Style
	RowBarMarkerStyle lipgloss.Style
)

// currentUIColors is the palette the vars currently hold, so a caller can save
// and restore it without copying every style.
var currentUIColors uiColors

func init() { applyUIColors(defaultUIColors()) }

// applyUIColors rebuilds every colour name and every style from c. It is the one
// place the chrome is built, so the live preview and the default cannot drift.
func applyUIColors(c uiColors) {
	currentUIColors = c

	Background = c.Background
	Text = c.Text
	TextMuted = c.TextMuted
	Primary = c.Primary
	Secondary = c.Secondary
	Accent = c.Accent
	Error = c.Error
	Warning = c.Warning
	Success = c.Success
	Info = c.Info
	BorderActive = c.BorderActive
	SyntaxKeyword = c.SyntaxKeyword
	SyntaxString = c.SyntaxString

	// The semantic roles resolve to the palette entries they may use, so "one
	// meaning per colour" is enforced by the name a screen asks for rather than by
	// a comment beside a hex value.
	CursorText = c.OnFill
	Brand = Primary
	BrandSoft = Primary
	Paper = c.OnFill
	Ink = Text
	InkDim = TextMuted
	Rule = TextMuted

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

	BoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(BorderActive).
		Padding(1, 2)

	// The progress bar's filled part is brand chrome, not a status colour:
	// progress is not a state, and Success is reserved for the steps that are
	// actually done. The bar's cells are glyphs (█ and ░), so these styles carry
	// the colour only and the bar still reads with no colour at all.
	ProgressBarFilled = lipgloss.NewStyle().
		Foreground(Brand)

	ProgressBarEmpty = lipgloss.NewStyle().
		Foreground(TextMuted)

	// HelpStyle remains for deadEnd's one-line way out; the footer draws through
	// AccentKeyStyle and HelpVerbStyle now.
	HelpStyle = lipgloss.NewStyle().
		Foreground(TextMuted).
		Italic(true).
		MarginTop(1)

	KeyStyle = lipgloss.NewStyle().
		Foreground(SyntaxKeyword).
		Bold(true)

	CodeStyle = lipgloss.NewStyle().
		Foreground(SyntaxString)

	DangerStyle = lipgloss.NewStyle().
		Foreground(Error).
		Bold(true)

	HighlightStyle = lipgloss.NewStyle().
		Foreground(Accent).
		Bold(true)

	StartCursorStyle = lipgloss.NewStyle().
		Foreground(CursorText).
		Background(Warning).
		Bold(true)

	CurrentCursorStyle = lipgloss.NewStyle().
		Foreground(CursorText).
		Background(Success).
		Bold(true)

	// Visual selection, like Vim's visual mode. Its background is the palette's
	// own blue: "#7aa2f7" was the one colour outside this theme, and it read as a
	// stranger next to the accents it sits between.
	SelectionStyle = lipgloss.NewStyle().
		Foreground(CursorText).
		Background(Primary).
		Bold(false)

	BrandStyle = lipgloss.NewStyle().
		Foreground(Brand).
		Bold(true)

	RuleStyle = lipgloss.NewStyle().
		Foreground(Rule)

	InkStyle = lipgloss.NewStyle().
		Foreground(Ink)

	AccentKeyStyle = lipgloss.NewStyle().
		Foreground(Accent).
		Bold(true)

	HelpVerbStyle = lipgloss.NewStyle().
		Foreground(InkDim)

	MeterFilledStyle = lipgloss.NewStyle().
		Foreground(Brand)

	MeterStyle = lipgloss.NewStyle().
		Foreground(InkDim)

	MeterOnBarStyle = lipgloss.NewStyle().
		Foreground(InkDim).
		Background(BrandSoft)

	// The companion's one tone: the brand role, because the creature is frame
	// furniture rather than content, and the tone carries nothing on its own (the
	// eyes, the z and the ! say which state it is in).
	CompanionStyle = lipgloss.NewStyle().
		Foreground(Brand)

	RowBarStyle = lipgloss.NewStyle().
		Foreground(Paper).
		Background(BrandSoft).
		Bold(true)

	RowBarMarkerStyle = lipgloss.NewStyle().
		Foreground(Accent).
		Background(BrandSoft).
		Bold(true)
}

// CenterHorizontally centers text horizontally within a given width
func CenterHorizontally(text string, width int) string {
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(text)
}
