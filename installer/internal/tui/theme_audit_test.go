package tui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// The theme audit.
//
// The redesign added a grammar -- the header bar and the status bar, the two-line
// row, the setup table, the utilities card and its panel, the theme grid, the
// installing checklist with its log card and its bar, and the cards the remaining
// pages draw -- and every one of those surfaces takes its colours from the theme
// applied through the installer. That is a property of the drawing, not a promise
// in a comment, so the guards here measure the bytes each surface emits:
//
//	TestEveryRedesignedSurfacePaintsOnlyTheThemesRoles
//	    no literal colour: every colour a redesigned surface paints is one the
//	    palette of the chrome it is wearing holds, under the default chrome and
//	    under an applied theme.
//	TestTheThemeSwitchRecoloursEveryRedesignedSurface
//	    the switch reaches every surface, and the repaint leaves nothing behind:
//	    the same model drawn again with no theme is the same bytes as before.
//	TestApplyUIColorsRebuildsEveryStyleTheSurfacesDrawWith
//	    no style is left out of the one builder: with a palette of marker colours
//	    applied, every style var renders a marker and nothing else. A style built
//	    once at package init (the emblem's ramp was) keeps the default palette in
//	    every theme and fails here.
//	TestTheSplashLockupWearsTheThemesRamp
//	    the dashboard's lockup paints the three stops of its ramp from the palette
//	    the chrome is wearing, and paints nothing else.
//	TestTheFilledStylesClearTheOnFillFloorInEveryTheme
//	TestTheRedesignKeepsTheBodyFloorOnEveryRoleItDrawsIn
//	    the contrast of the roles and the pairs the new surfaces draw: every style
//	    that fills a block is measured against the ink it draws on it, and the body
//	    ink, the key tone and the muted role are held to their floors -- for every
//	    theme the installer offers and for the default chrome it ships.
//
// The theme selector is the one screen that paints colours which are not its own
// chrome: its cards, its palette board and the preview swatches are pictures of
// the other themes, painted from the definitions the switch would write. The
// guard allows the definitions' palette on that surface and nowhere else, so the
// exemption is written down rather than assumed.

// themeAuditRGB is one colour a rendered frame paints, as the terminal receives it.
type themeAuditRGB struct {
	R, G, B int
}

// themeAuditSGR matches the escape sequences that carry a colour. Only the SGR
// form is read: the surfaces are rendered on a true-colour terminal, which is the
// only profile the applied theme is painted on at all.
var themeAuditSGR = regexp.MustCompile("\x1b\\[([0-9;]*)m")

// themeAuditColorsIn reads every colour a rendered view paints, foreground and
// background alike.
func themeAuditColorsIn(t *testing.T, view string) []themeAuditRGB {
	t.Helper()

	var out []themeAuditRGB
	for _, match := range themeAuditSGR.FindAllStringSubmatch(view, -1) {
		fields := strings.Split(match[1], ";")
		for i := 0; i < len(fields); i++ {
			if fields[i] != "38" && fields[i] != "48" {
				continue
			}
			if i+1 >= len(fields) {
				break
			}
			switch fields[i+1] {
			case "2":
				if i+4 >= len(fields) {
					break
				}
				out = append(out, themeAuditRGB{
					R: themeAuditChannel(t, fields[i+2]),
					G: themeAuditChannel(t, fields[i+3]),
					B: themeAuditChannel(t, fields[i+4]),
				})
				i += 4
			case "5":
				// A palette index means the renderer is not on the true-colour path,
				// so the measurement below would compare two different colour spaces.
				t.Fatalf("a surface was rendered with a palette index (%s): the guard measures 24-bit colours only", match[0])
			}
		}
	}
	return out
}

// themeAuditChannel reads one 8-bit channel of a rendered sequence.
func themeAuditChannel(t *testing.T, field string) int {
	t.Helper()
	value, err := strconv.Atoi(field)
	if err != nil {
		t.Fatalf("read the rendered channel %q: %v", field, err)
	}
	return value
}

// themeAuditRenderOf is the colour lipgloss paints for one hex value, read from a
// probe rather than computed: lipgloss writes an eight-bit channel through
// sixteen bits, so the value it emits is its own and not the hex restated.
func themeAuditRenderOf(t *testing.T, hex string) themeAuditRGB {
	t.Helper()
	if !themeHexRE.MatchString(hex) {
		t.Fatalf("%q is not a #rrggbb colour", hex)
	}
	probe := lipgloss.NewStyle().Foreground(lipgloss.Color(hex)).Render("x")
	colors := themeAuditColorsIn(t, probe)
	if len(colors) != 1 {
		t.Fatalf("the renderer painted %d colour(s) for %s, want one", len(colors), hex)
	}
	return colors[0]
}

// themeAuditPaletteColors is every colour a uiColors value can paint: every role
// and both members of each adaptive pair, because which member is used depends on
// the terminal's background and the guard does not care which one it got.
func themeAuditPaletteColors(t *testing.T, c uiColors) map[themeAuditRGB]bool {
	t.Helper()

	allowed := map[themeAuditRGB]bool{}
	for _, pair := range []lipgloss.AdaptiveColor{
		c.Background, c.Text, c.TextMuted, c.Primary, c.Secondary, c.Accent,
		c.Key, c.Error, c.Warning, c.Success, c.Info, c.BorderActive,
		c.SyntaxKeyword, c.SyntaxString, c.OnFill,
	} {
		for _, hex := range []string{pair.Light, pair.Dark} {
			if hex == "" {
				continue
			}
			allowed[themeAuditRenderOf(t, hex)] = true
		}
	}
	return allowed
}

// themeAuditDefinitionColors adds the palette of every shipped definition to an
// allowed set. It is for the theme selector, whose cards are pictures of other
// themes: the only surface allowed to paint a colour that is not its own chrome.
func themeAuditDefinitionColors(t *testing.T, allowed map[themeAuditRGB]bool, defs []themeDefinition) {
	t.Helper()

	tables := func(def themeDefinition) []map[string]string {
		return []map[string]string{def.Palette, def.Prompt, def.Syntax}
	}
	for _, def := range defs {
		for _, table := range tables(def) {
			for _, hex := range table {
				if themeHexRE.MatchString(hex) {
					allowed[themeAuditRenderOf(t, hex)] = true
				}
			}
		}
	}
}

// themeAuditSurface is one surface the redesign added, and how to build it.
type themeAuditSurface struct {
	name string
	// build returns the model at its default state: the cursor is not on a theme
	// row, so a render is the chrome the run wears and not a live preview.
	build func(*testing.T) Model
	// definitions says the surface paints the shipped definitions' own colours by
	// design. Only the theme selector's pictures do.
	definitions bool
}

// themeAuditSizes are the two frames a surface is measured at: the documented
// floor and the wide frame the mockup's layout is drawn at.
var themeAuditSizes = [][2]int{{80, 24}, {160, 50}}

// themeAuditSurfaces is every surface the redesign added, one entry per panel of
// the mockup plus the pages the mockup does not draw. The installer's frame is
// shared, so the header bar and the status bar are measured on all of them.
func themeAuditSurfaces() []themeAuditSurface {
	frame := func(name string) func(*testing.T) Model {
		return func(t *testing.T) Model {
			t.Helper()
			m := installerFrameCase(t, name)
			m.Animating = false
			return m
		}
	}
	return []themeAuditSurface{
		// The dashboard, the main menu and the two-line rows.
		{name: "dashboard", build: frame("welcome")},
		{name: "main-menu", build: frame("main-menu")},
		{name: "main-menu-restore", build: frame("main-menu-restore")},
		// The setup table and its panel.
		{name: "setup", build: frame("setup")},
		{name: "install-review", build: frame("install-review")},
		// The utilities card and the System Overview panel.
		{name: "utilities", build: func(t *testing.T) Model {
			t.Helper()
			m := utilitiesFrameModel(t)
			return m
		}},
		// The theme grid, its cards and its floor list.
		{name: "theme-picker", build: themeAuditPickerModel, definitions: true},
		// The installing checklist, the log card and the bar.
		{name: "installing", build: frame("installing-details")},
		{name: "installing-at-end", build: frame("installing-at-end")},
		// The completion, the dead ends and the backup screens.
		{name: "complete", build: frame("complete")},
		{name: "error", build: frame("error")},
		{name: "backup-confirm", build: frame("backup-confirm")},
		{name: "restore-backup", build: frame("restore-backup")},
		{name: "restore-confirm", build: frame("restore-confirm")},
		// The wizard's questions.
		{name: "os-select", build: frame("os-select")},
		{name: "terminal-select", build: frame("terminal-select")},
		{name: "terminal-select-wsl", build: frame("terminal-select-wsl")},
		{name: "font-select", build: frame("font-select")},
		{name: "shell-select", build: frame("shell-select")},
		{name: "wm-select", build: frame("wm-select")},
		{name: "nvim-select", build: frame("nvim-select")},
		{name: "ghostty-warning", build: frame("ghostty-warning")},
		// The remaining pages: the learn tables, the keymaps and the two section
		// screens the slice left on the shared renderer.
		{name: "learn-terminals", build: frame("learn-terminals")},
		{name: "learn-tooling", build: frame("learn-tooling")},
		{name: "keymaps", build: frame("keymaps")},
		{name: "update-center", build: func(t *testing.T) Model {
			t.Helper()
			return updateCenterFrameCase(t)
		}},
		{name: "doctor", build: func(t *testing.T) Model {
			t.Helper()
			return doctorFrameCase(t)
		}},
	}
}

// themeAuditPickerModel is the theme selector with the shipped definitions and the
// cursor off the theme rows, so the render is the chrome the run wears rather than
// the live preview the picker paints while the cursor is on a theme.
func themeAuditPickerModel(t *testing.T) Model {
	t.Helper()

	m := themePickerFixtureModel(t, trainerFrameWidth, trainerFrameHeight)
	m.DotfilesThemeRecord = nil
	m.Animating = false
	m.Cursor = len(m.GetCurrentOptions()) - 1
	return m
}

// TestEveryRedesignedSurfacePaintsOnlyTheThemesRoles is the no-literal-colour rule
// for the redesign: under the default chrome and under an applied theme, every
// colour a redesigned surface paints is one the palette it is wearing holds. A hex
// spelled into a screen, a style that keeps the previous palette, or a colour
// derived outside the roles fails here with the surface and the colour named.
func TestEveryRedesignedSurfacePaintsOnlyTheThemesRoles(t *testing.T) {
	forceTrueColor(t)
	defs, applied := themeTestDefinitions(t, appliedThemeID)
	appliedColors, err := themePreviewColors(applied)
	if err != nil {
		t.Fatalf("the applied theme cannot be painted: %v", err)
	}

	for _, surface := range themeAuditSurfaces() {
		for _, size := range themeAuditSizes {
			profiles := []struct {
				name   string
				record *dotfilesThemeRecord
				colors uiColors
			}{
				{name: "the default chrome", colors: defaultUIColors()},
				{name: "the applied " + applied.Name, record: &dotfilesThemeRecord{Theme: applied.ID}, colors: appliedColors},
			}
			for _, profile := range profiles {
				m := surface.build(t)
				m.DotfilesThemes = defs
				m.DotfilesThemeRecord = profile.record
				m.Width, m.Height = size[0], size[1]

				view := m.View()
				colors := themeAuditColorsIn(t, view)
				if len(colors) == 0 {
					t.Fatalf("%s at %dx%d paints no colour under %s: the guard would pass on an empty frame",
						surface.name, size[0], size[1], profile.name)
				}

				allowed := themeAuditPaletteColors(t, profile.colors)
				if surface.definitions {
					themeAuditDefinitionColors(t, allowed, defs)
				}
				for _, got := range colors {
					if allowed[got] {
						continue
					}
					t.Errorf("%s at %dx%d paints #%02x%02x%02x under %s, which is not one of that palette's roles",
						surface.name, size[0], size[1], got.R, got.G, got.B, profile.name)
				}
			}
		}
	}
}

// TestTheThemeSwitchRecoloursEveryRedesignedSurface is the user's own report as a
// guard, measured on every surface the redesign added: applying a theme repaints
// the frame, and the repaint is the theme's and not a layer left on top of the
// default chrome. Clearing the record draws the bytes the model drew before, so
// nothing of the applied palette survives the render that used it.
func TestTheThemeSwitchRecoloursEveryRedesignedSurface(t *testing.T) {
	forceTrueColor(t)
	defs, applied := themeTestDefinitions(t, appliedThemeID)
	blue := themeBlueSequence(t, applied)
	before := currentUIColors
	t.Cleanup(func() { applyUIColors(before) })

	for _, surface := range themeAuditSurfaces() {
		m := surface.build(t)
		m.DotfilesThemes = defs
		m.DotfilesThemeRecord = nil
		m.Width, m.Height = trainerFrameWidth, trainerFrameHeight

		plain := m.View()
		if strings.Contains(plain, blue) {
			t.Errorf("%s paints %s before any theme is applied, so the guard cannot tell what the switch did",
				surface.name, applied.Palette["blue"])
		}

		m.DotfilesThemeRecord = &dotfilesThemeRecord{Theme: applied.ID}
		themed := m.View()
		if !strings.Contains(themed, blue) {
			t.Errorf("%s draws none of %s with %s applied: the switch does not reach this surface",
				surface.name, applied.Palette["blue"], applied.Name)
		}
		if themed == plain {
			t.Errorf("%s is byte-for-byte the same frame with %s applied", surface.name, applied.Name)
		}

		// The paint is a render, not state: with no record the surface draws what
		// it drew before, and the package palette is the one it started with.
		m.DotfilesThemeRecord = nil
		if after := m.View(); after != plain {
			t.Errorf("%s draws a different frame after the applied theme is cleared: the repaint leaked into the model",
				surface.name)
		}
		if currentUIColors != before {
			t.Fatalf("%s left the package palette painted in %s after View returned", surface.name, applied.Name)
		}
	}
}

// TestApplyUIColorsRebuildsEveryStyleTheSurfacesDrawWith is the other half of the
// one-builder rule: applyUIColors is the only place the chrome is built, so every
// style a surface draws with has to be built there. The palette used here gives
// every field a colour of its own; a style the builder rebuilt therefore paints a
// marker colour, and a style it missed keeps the default palette and cannot. The
// emblem's ramp was built once at package init and painted no theme colour at all
// -- it is the case this guard was written for.
func TestApplyUIColorsRebuildsEveryStyleTheSurfacesDrawWith(t *testing.T) {
	forceTrueColor(t)
	previous := currentUIColors
	t.Cleanup(func() { applyUIColors(previous) })

	markers := themeAuditMarkerPalette()
	applyUIColors(markers)
	if currentUIColors != markers {
		t.Fatalf("applyUIColors did not take the palette it was given: %+v", currentUIColors)
	}
	allowed := themeAuditPaletteColors(t, markers)

	styles := themeAuditStyleVars(t)
	if len(styles) == 0 {
		t.Fatal("no style to check, so the guard proves nothing")
	}
	for _, entry := range styles {
		colors := themeAuditColorsIn(t, entry.style.Render("x"))
		if len(colors) == 0 {
			t.Errorf("%s paints no colour after applyUIColors: it was built once and never rebuilt", entry.name)
			continue
		}
		for _, got := range colors {
			if allowed[got] {
				continue
			}
			t.Errorf("%s paints #%02x%02x%02x after applyUIColors, which is not in the palette it was given: the style was not rebuilt",
				entry.name, got.R, got.G, got.B)
		}
	}
}

// themeAuditMarkerPalette is a palette whose every field is a colour of its own, so
// a style that paints something outside it is painting something the builder did
// not give it. The values are far apart so lipgloss's eight-bit rounding cannot
// turn one field into another.
func themeAuditMarkerPalette() uiColors {
	marker := func(index int) lipgloss.AdaptiveColor {
		hex := fmt.Sprintf("#%02x%02x%02x", index*16, index*16, index*16)
		return lipgloss.AdaptiveColor{Light: hex, Dark: hex}
	}
	return uiColors{
		Background:    marker(1),
		Text:          marker(2),
		TextMuted:     marker(3),
		Primary:       marker(4),
		Secondary:     marker(5),
		Accent:        marker(6),
		Key:           marker(7),
		Error:         marker(8),
		Warning:       marker(9),
		Success:       marker(10),
		Info:          marker(11),
		BorderActive:  marker(12),
		SyntaxKeyword: marker(13),
		SyntaxString:  marker(14),
		OnFill:        marker(15),
	}
}

// themeAuditStyleVar is one style the surfaces draw with, named for the failure
// message.
type themeAuditStyleVar struct {
	name  string
	style lipgloss.Style
}

// themeAuditStyleVars is every style applyUIColors builds, read after it has run.
// The list is written out rather than reflected because Go cannot enumerate a
// package's variables; a style added to styles.go without being added here is
// caught by the colour sweep above, which fails on any colour the palette does not
// hold.
func themeAuditStyleVars(t *testing.T) []themeAuditStyleVar {
	t.Helper()

	styles := []themeAuditStyleVar{{"TitleStyle", TitleStyle},
		{"SubtitleStyle", SubtitleStyle},
		{"SuccessStyle", SuccessStyle},
		{"ErrorStyle", ErrorStyle},
		{"WarningStyle", WarningStyle},
		{"InfoStyle", InfoStyle},
		{"MutedStyle", MutedStyle},
		{"ProgressBarFilled", ProgressBarFilled},
		{"ProgressBarEmpty", ProgressBarEmpty},
		{"KeyStyle", KeyStyle},
		{"CodeStyle", CodeStyle},
		{"HighlightStyle", HighlightStyle},
		{"StartCursorStyle", StartCursorStyle},
		{"CurrentCursorStyle", CurrentCursorStyle},
		{"SelectionStyle", SelectionStyle},
		{"BrandStyle", BrandStyle},
		{"RuleStyle", RuleStyle},
		{"PanelBorderStyle", PanelBorderStyle},
		{"HeaderChipStyle", HeaderChipStyle},
		{"HeaderRuleStyle", HeaderRuleStyle},
		{"HeaderChipSuccessStyle", HeaderChipSuccessStyle},
		{"HeaderRuleSuccessStyle", HeaderRuleSuccessStyle},
		{"InkStyle", InkStyle},
		{"AccentKeyStyle", AccentKeyStyle},
		{"HelpVerbStyle", HelpVerbStyle},
		{"MeterFilledStyle", MeterFilledStyle},
		{"MeterStyle", MeterStyle},
		{"MeterOnBarStyle", MeterOnBarStyle},
		{"CompanionStyle", CompanionStyle},
		{"RowBarStyle", RowBarStyle},
		{"RowBarMarkerStyle", RowBarMarkerStyle},
		{"ButtonStyle", ButtonStyle},
	}
	if len(EmblemRampStyles) != 3 {
		// The ramp is three stops -- brand, secondary and accent -- and it is
		// rebuilt with the rest: the splash emblem is the dashboard's lockup, which
		// is the first thing a themed run shows.
		t.Fatalf("the emblem ramp has %d stop(s), want the three the ramp is drawn in: it is not rebuilt with the chrome",
			len(EmblemRampStyles))
	}
	for i, style := range EmblemRampStyles {
		styles = append(styles, themeAuditStyleVar{fmt.Sprintf("EmblemRampStyles[%d]", i), style})
	}
	return styles
}

// TestTheSplashLockupWearsTheThemesRamp is the dashboard's half of the guard
// above: the emblem is the first thing a themed run shows, so it is drawn in the
// palette the chrome is wearing -- all three stops of its ramp, and nothing else.
// It is the case a ramp built once at package init failed: the stops were empty
// colours and the lockup lost its ramp in every theme.
func TestTheSplashLockupWearsTheThemesRamp(t *testing.T) {
	forceTrueColor(t)
	_, applied := themeTestDefinitions(t, appliedThemeID)
	appliedColors, err := themePreviewColors(applied)
	if err != nil {
		t.Fatalf("the applied theme cannot be painted: %v", err)
	}

	previous := currentUIColors
	t.Cleanup(func() { applyUIColors(previous) })

	for _, palette := range []struct {
		name   string
		colors uiColors
	}{
		{name: "the default chrome", colors: defaultUIColors()},
		{name: "the applied " + applied.Name, colors: appliedColors},
	} {
		applyUIColors(palette.colors)

		painted := map[themeAuditRGB]bool{}
		for _, row := range renderEmblem(compactLogo) {
			for _, color := range themeAuditColorsIn(t, row) {
				painted[color] = true
			}
		}

		// The ramp is the brand at the top, the softer secondary through the middle
		// and the accent at the tip. Either member of an adaptive pair is the stop, so
		// the guard measures what the terminal resolved and not a fixed polarity.
		for _, stop := range []struct {
			name string
			pair lipgloss.AdaptiveColor
		}{
			{"brand", Primary},
			{"secondary", Secondary},
			{"accent", Accent},
		} {
			found := false
			for _, hex := range []string{stop.pair.Light, stop.pair.Dark} {
				if painted[themeAuditRenderOf(t, hex)] {
					found = true
				}
			}
			if !found {
				t.Errorf("the lockup draws none of the %s stop under %s: the ramp is not painted with the chrome's palette",
					stop.name, palette.name)
			}
		}
		if len(painted) != 3 {
			t.Errorf("the lockup paints %d colour(s) under %s, want the ramp's three stops",
				len(painted), palette.name)
		}
	}
}

// TestTheFilledStylesClearTheOnFillFloorInEveryTheme measures the colour pairs the
// new surfaces paint a block with: the header tab, the selected row's bar, the
// state blocks, the button and the meter on the bar all fill a run of cells and
// draw their label on it, so the ink and the fill are held to the floor a bold run
// of text needs -- for every theme the installer offers and for the default chrome
// it ships.
//
// RowBarMarkerStyle is deliberately not in the list: it paints glyphs and never a
// word (the cursor's "▸", the setup table's chevron, the checklist's state glyph
// and the picker's mark), and a glyph on a fill is told apart by its shape and its
// hue. Every word drawn on a bar goes through RowBarStyle, which is measured here.
func TestTheFilledStylesClearTheOnFillFloorInEveryTheme(t *testing.T) {
	forceTrueColor(t)
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}

	palettes := []struct {
		name   string
		colors uiColors
	}{{name: "the default chrome", colors: defaultUIColors()}}
	for _, def := range defs {
		if !def.Complete() {
			continue
		}
		colors, err := themePreviewColors(def)
		if err != nil {
			t.Errorf("theme %q cannot be painted, so it cannot be offered: %v", def.ID, err)
			continue
		}
		palettes = append(palettes, struct {
			name   string
			colors uiColors
		}{name: def.Name, colors: colors})
	}

	previous := currentUIColors
	t.Cleanup(func() { applyUIColors(previous) })

	measured := 0
	for _, palette := range palettes {
		applyUIColors(palette.colors)
		for _, filled := range themeAuditFilledStyles() {
			ink, fill, ok := themeAuditStylePair(t, filled.style)
			if !ok {
				t.Errorf("%s draws no ink on a fill under %s", filled.name, palette.name)
				continue
			}
			measured++
			if ratio := themeContrastRatio(ink, fill); ratio < uiThemeOnFillFloor {
				t.Errorf("%s draws %s on %s under %s at %.2f:1, want at least %.1f:1",
					filled.name, ink, fill, palette.name, ratio, uiThemeOnFillFloor)
			}
		}
	}
	if measured < 2 {
		t.Fatalf("only %d pair(s) measured: the guard would pass on an empty list", measured)
	}
}

// themeAuditFilledStyles is every style that fills a run of cells and draws an ink
// on it. RowBarMarkerStyle is excluded on purpose; see the guard above.
func themeAuditFilledStyles() []themeAuditStyleVar {
	return []themeAuditStyleVar{
		{"HeaderChipStyle", HeaderChipStyle},
		{"HeaderChipSuccessStyle", HeaderChipSuccessStyle},
		{"StartCursorStyle", StartCursorStyle},
		{"CurrentCursorStyle", CurrentCursorStyle},
		{"SelectionStyle", SelectionStyle},
		{"RowBarStyle", RowBarStyle},
		{"MeterOnBarStyle", MeterOnBarStyle},
		{"ButtonStyle", ButtonStyle},
	}
}

// themeAuditStylePair reads a style's ink and its fill off a rendered probe. A
// style with no background fills nothing, so it has no pair to measure.
func themeAuditStylePair(t *testing.T, style lipgloss.Style) (ink, fill string, ok bool) {
	t.Helper()

	var fg, bg *themeAuditRGB
	for _, match := range themeAuditSGR.FindAllStringSubmatch(style.Render("x"), -1) {
		fields := strings.Split(match[1], ";")
		for i := 0; i < len(fields); i++ {
			if (fields[i] != "38" && fields[i] != "48") || i+4 >= len(fields) || fields[i+1] != "2" {
				continue
			}
			color := themeAuditRGB{
				R: themeAuditChannel(t, fields[i+2]),
				G: themeAuditChannel(t, fields[i+3]),
				B: themeAuditChannel(t, fields[i+4]),
			}
			seen := color
			if fields[i] == "38" {
				fg = &seen
			} else {
				bg = &seen
			}
			i += 4
		}
	}
	if fg == nil || bg == nil {
		return "", "", false
	}
	hex := func(c themeAuditRGB) string { return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B) }
	return hex(*fg), hex(*bg), true
}

// TestTheRedesignKeepsTheBodyFloorOnEveryRoleItDrawsIn is the body floor as the
// surfaces use it: the ink the tables, the cards and the prose are read through is
// the body ink, the muted role is held to the floor its own definition earns, and
// the key tokens of every footer are the key tone. It measures the palette each
// theme is painted from, so a role that a new surface reaches for cannot be the
// one colour that fails.
func TestTheRedesignKeepsTheBodyFloorOnEveryRoleItDrawsIn(t *testing.T) {
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}

	const bodyFloor = 4.5
	// The muted role is metadata rather than body text, and several definitions
	// draw it deliberately dimmer -- the shipped default among them, at 3.2:1. The
	// ladder is the one the drift guard already holds: a light theme's muted role
	// is held to the body floor, a dark theme's to the 3:1 a large, dim run needs.
	const mutedFloor = 3.0

	measured := 0
	for _, def := range defs {
		if !def.Complete() {
			continue
		}
		colors, err := themePreviewColors(def)
		if err != nil {
			t.Errorf("theme %q cannot be painted, so it cannot be offered: %v", def.ID, err)
			continue
		}
		base := colors.Background.Dark
		measured++

		if got := themeContrastRatio(colors.Text.Dark, base); got < bodyFloor {
			t.Errorf("%s: the body ink %s on the base %s is %.2f:1, want at least %.1f:1",
				def.ID, colors.Text.Dark, base, got, bodyFloor)
		}
		if got := themeContrastRatio(colors.Key.Dark, base); got < bodyFloor {
			t.Errorf("%s: the key tone %s on the base %s is %.2f:1, want at least %.1f:1",
				def.ID, colors.Key.Dark, base, got, bodyFloor)
		}

		muted := colors.TextMuted.Dark
		floor := mutedFloor
		if themeBaseIsLight(def) {
			floor = bodyFloor
		}
		if got := themeContrastRatio(muted, base); got < floor {
			t.Errorf("%s: the muted role %s on the base %s is %.2f:1, want at least %.1f:1",
				def.ID, muted, base, got, floor)
		}
	}
	if measured < 2 {
		t.Fatalf("only %d theme(s) measured: the guard would pass on an empty repository", measured)
	}
}
