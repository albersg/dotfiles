package tui

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// The live theme preview.
//
// While the cursor is on a theme row the installer repaints its whole interface
// in that theme's colours, and it does so by rebuilding the package styles from
// the definition rather than by holding a second palette of its own. The mapping
// from the installer's semantic colours to the theme's roles is written down once
// here, so a reader can follow which role paints what.
//
// Nothing is written to disk: applyPreviewTheme only reassigns the in-memory
// styles, and the caller restores them before it returns. A screen with no
// preview active is therefore byte-for-byte the default chrome.

// themePreviewColor is the colour a preview paints with. Both adaptive fields
// carry the same hex: the preview shows the theme's real colour rather than an
// adapted one, because the point of the preview is to see the colours the apply
// would write, and the apply writes the definition's value.
func themePreviewColor(hex string) lipgloss.AdaptiveColor {
	return lipgloss.AdaptiveColor{Light: hex, Dark: hex}
}

// themePreviewAdaptive is the colour a preview paints for a role that has a
// light and a dark member. When the definition has no light member (a theme whose
// light flavour this repository does not ship) the dark value is used for both,
// which is a known readability limit rather than an invented colour.
func themePreviewAdaptive(light, dark string) lipgloss.AdaptiveColor {
	if light == "" {
		light = dark
	}
	return lipgloss.AdaptiveColor{Light: light, Dark: dark}
}

// uiThemeRepaintAllowed reports whether this terminal may paint the installer in
// a theme's colours. It is the criterion the companion's volumetric sprite
// already uses: a palette of 24-bit values is only shown on a terminal that
// reports 24-bit colour. A terminal reporting sixteen colours, eight or none
// keeps the installer's own adaptive chrome, which degrades to that terminal,
// instead of approximating a palette the user did not choose.
func uiThemeRepaintAllowed(profile termenv.Profile) bool {
	return profile == termenv.TrueColor
}

// uiThemeOnFillFloor is the contrast a label drawn on a filled block is held to.
// The blocks carry one short word or a meter, drawn bold, so the 3:1 a bold run
// of text needs is the floor rather than the 4.5:1 body text is held to.
const uiThemeOnFillFloor = 3.0

// themeRelativeLuminance is the WCAG relative luminance of a #rrggbb colour. A
// value that is not a six-digit hex colour reads as -1, so a caller can tell a
// dark colour from an unreadable one.
func themeRelativeLuminance(hex string) float64 {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return -1
	}
	var channels [3]float64
	for i := 0; i < 3; i++ {
		value, err := strconv.ParseUint(hex[i*2:i*2+2], 16, 8)
		if err != nil {
			return -1
		}
		c := float64(value) / 255
		if c <= 0.03928 {
			channels[i] = c / 12.92
		} else {
			channels[i] = math.Pow((c+0.055)/1.055, 2.4)
		}
	}
	return 0.2126*channels[0] + 0.7152*channels[1] + 0.0722*channels[2]
}

// themeContrastRatio is the WCAG contrast ratio between two #rrggbb colours. A
// value that is not a colour reads as 0, so it can never pass a floor.
func themeContrastRatio(a, b string) float64 {
	la, lb := themeRelativeLuminance(a), themeRelativeLuminance(b)
	if la < 0 || lb < 0 {
		return 0
	}
	hi, lo := math.Max(la, lb), math.Min(la, lb)
	return (hi + 0.05) / (lo + 0.05)
}

// themeOnFillInk is the ink an applied theme draws the labels of its filled
// blocks in. The theme's own base and text are tried first, and the one with the
// highest worst-case contrast against the fills wins: for a dark theme the base
// is the ink that reads on the lighter fills, and for a light theme the text is
// the darker one. When neither reaches the floor on every fill -- a light theme's
// mid-tone status fills are the real case -- the installer's own adaptive text is
// used instead, forced to the polarity the theme's base implies. That is a
// colour the chrome already owns and was chosen for contrast, so a theme whose
// own palette cannot label a fill stays legible without inventing a colour.
func themeOnFillInk(def themeDefinition, fills ...string) lipgloss.AdaptiveColor {
	candidates := []string{def.Palette["base"]}
	if text := def.Palette["text"]; text != "" && text != def.Palette["base"] {
		candidates = append(candidates, text)
	}

	best := candidates[0]
	bestScore := math.Inf(-1)
	for _, candidate := range candidates {
		score := math.Inf(1)
		for _, fill := range fills {
			score = math.Min(score, themeContrastRatio(candidate, fill))
		}
		if score > bestScore {
			best, bestScore = candidate, score
		}
	}
	if bestScore >= uiThemeOnFillFloor {
		return themePreviewColor(best)
	}

	fallback := defaultUIColors().Text
	if themeBaseIsLight(def) {
		// The theme's fills sit on a light base, so the dark member of the
		// installer's ink is the one that contrasts.
		return themePreviewColor(fallback.Light)
	}
	return themePreviewColor(fallback.Dark)
}

// themePreviewColors derives the installer's palette from a theme definition.
// Every value comes from the definition; where a semantic role has no terminal
// role of its own the prompt table supplies it, and where the definition has
// nothing the fallback role does. An empty result means the definition cannot
// paint the interface at all.
func themePreviewColors(def themeDefinition) (uiColors, error) {
	role := func(name string) string { return def.Palette[name] }
	prompt := func(name, fallback string) string {
		if value := def.Prompt[name]; value != "" && value != "none" {
			return value
		}
		return def.Palette[fallback]
	}

	// The mapping, in the order the styles read it:
	//
	//   Background    base          the terminal's own background
	//   Text          text          body text
	//   TextMuted     prompt subtext0, else bright_black   metadata
	//   Primary       blue          chrome and accent
	//   Secondary     prompt mauve, else bright_blue       subtitles
	//   Accent        cursor        key tokens and markers
	//   Error         red
	//   Warning       prompt peach, else yellow
	//   Success       green
	//   Info          blue
	//   BorderActive  blue
	//   SyntaxKeyword [syntax] keyword   the theme's own code-display tints
	//   SyntaxString  [syntax] string
	semantic := map[string]string{
		"background":    role("base"),
		"text":          role("text"),
		"text_muted":    prompt("subtext0", "bright_black"),
		"primary":       role("blue"),
		"secondary":     prompt("mauve", "bright_blue"),
		"accent":        role("cursor"),
		"error":         role("red"),
		"warning":       prompt("peach", "yellow"),
		"success":       role("green"),
		"info":          role("blue"),
		"border_active": role("blue"),
	}

	keywordDark := def.Syntax["keyword_dark"]
	stringDark := def.Syntax["string_dark"]

	colors := uiColors{
		Background:    themePreviewColor(semantic["background"]),
		Text:          themePreviewColor(semantic["text"]),
		TextMuted:     themePreviewColor(semantic["text_muted"]),
		Primary:       themePreviewColor(semantic["primary"]),
		Secondary:     themePreviewColor(semantic["secondary"]),
		Accent:        themePreviewColor(semantic["accent"]),
		Error:         themePreviewColor(semantic["error"]),
		Warning:       themePreviewColor(semantic["warning"]),
		Success:       themePreviewColor(semantic["success"]),
		Info:          themePreviewColor(semantic["info"]),
		BorderActive:  themePreviewColor(semantic["border_active"]),
		SyntaxKeyword: themePreviewAdaptive(def.Syntax["keyword_light"], keywordDark),
		SyntaxString:  themePreviewAdaptive(def.Syntax["string_light"], stringDark),
		OnFill:        themeOnFillInk(def, semantic["warning"], semantic["success"], semantic["primary"]),
	}

	for name, value := range semantic {
		if !themeHexRE.MatchString(value) {
			return uiColors{}, fmt.Errorf("theme %q cannot be previewed: %s has no colour (%q)", def.ID, name, value)
		}
	}
	for name, value := range map[string]string{"keyword": keywordDark, "string": stringDark} {
		if !themeHexRE.MatchString(value) {
			return uiColors{}, fmt.Errorf("theme %q cannot be previewed: its [syntax] %s has no colour (%q)", def.ID, name, value)
		}
	}
	return colors, nil
}

// applyPreviewTheme repaints the interface in def's colours and returns the
// function that puts the default chrome back. It writes nothing to disk and is
// deterministic: the restore re-applies the palette that was current, so the
// bytes after a restore are the bytes before the preview.
func applyPreviewTheme(def themeDefinition) (func(), error) {
	colors, err := themePreviewColors(def)
	if err != nil {
		return func() {}, err
	}
	previous := currentUIColors
	applyUIColors(colors)
	return func() { applyUIColors(previous) }, nil
}
