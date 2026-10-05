package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
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
	//   SyntaxKeyword prompt mauve, else magenta   the installer's two syntax
	//   SyntaxString  prompt peach, else yellow    tints, mapped to the theme
	semantic := map[string]string{
		"background":     role("base"),
		"text":           role("text"),
		"text_muted":     prompt("subtext0", "bright_black"),
		"primary":        role("blue"),
		"secondary":      prompt("mauve", "bright_blue"),
		"accent":         role("cursor"),
		"error":          role("red"),
		"warning":        prompt("peach", "yellow"),
		"success":        role("green"),
		"info":           role("blue"),
		"border_active":  role("blue"),
		"syntax_keyword": prompt("mauve", "magenta"),
		"syntax_string":  prompt("peach", "yellow"),
	}

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
		SyntaxKeyword: themePreviewColor(semantic["syntax_keyword"]),
		SyntaxString:  themePreviewColor(semantic["syntax_string"]),
	}

	for name, value := range semantic {
		if !themeHexRE.MatchString(value) {
			return uiColors{}, fmt.Errorf("theme %q cannot be previewed: %s has no colour (%q)", def.ID, name, value)
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
