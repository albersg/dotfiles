package tui

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// The theme selector's own guards. The behaviour guards for the picker live
// beside the screens they walk (update_test.go, util_screen_test.go) and the frame
// guards measure it at every terminal size (screen_coverage_test.go); these
// measure the drawing the mockup's sixth panel asked for: the grid, the miniature
// and the honest state where there is nothing to choose.

// themePickerFixtureModel is the picker with the repository's definitions, a
// recorded change (so the undo row is offered) and the cursor on the first theme,
// at the size given.
func themePickerFixtureModel(t *testing.T, width, height int) Model {
	t.Helper()

	m := installerFrameModel(t, ScreenThemePicker)
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	m.DotfilesThemes = defs
	m.DotfilesThemeRecord = &dotfilesThemeRecord{Theme: defaultThemeID}
	m.Cursor = 0
	m.Width, m.Height = width, height
	return m
}

// TestThemePickerDrawsTheMockupsGridWhereTheFrameHoldsIt pins the shape the
// mockup draws and the shape the floor keeps. At the wide frame the selector is
// two columns of cards with their miniatures, so a grid row opens two boxes. At
// the 80x24 floor the same themes are one row each -- the mark, the name and the
// palette strip -- with no box at all, because a boxed picture there would cost
// the rows every theme needs. Both frames carry the mockup's heading, its subtitle
// and the tip, and every theme's name is drawn whole at both.
func TestThemePickerDrawsTheMockupsGridWhereTheFrameHoldsIt(t *testing.T) {
	m := themePickerFixtureModel(t, 160, 50)
	plain := ansiEscape.ReplaceAllString(m.View(), "")

	if strings.Count(plain, "╭") == 0 {
		t.Fatalf("the wide selector draws no card at all:\n%s", plain)
	}
	// A grid row's boxes share one line, so the first box line carries two of them.
	boxes := 0
	for _, line := range strings.Split(plain, "\n") {
		if strings.Contains(line, "╭") {
			boxes = strings.Count(line, "╭")
			break
		}
	}
	if boxes != 2 {
		t.Errorf("the wide selector opens %d card(s) on its first grid row, want the mockup's 2", boxes)
	}
	for _, want := range []string{themePickerHeading, themePickerSubtitle, themePickerTipText} {
		if !strings.Contains(plain, want) {
			t.Errorf("the wide selector does not draw %q:\n%s", want, plain)
		}
	}
	themePickerAssertEveryThemeNamed(t, plain, m, "the wide frame")

	m.Width, m.Height = 80, 24
	plain = ansiEscape.ReplaceAllString(m.View(), "")
	if strings.Contains(plain, "╭") {
		t.Errorf("the 80x24 selector draws a card box, which costs the rows the themes need:\n%s", plain)
	}
	for _, want := range []string{themePickerHeading, themePickerSubtitle, themePickerTipText} {
		if !strings.Contains(plain, want) {
			t.Errorf("the 80x24 selector does not draw %q:\n%s", want, plain)
		}
	}
	themePickerAssertEveryThemeNamed(t, plain, m, "the floor")
}

// themePickerAssertEveryThemeNamed fails when a complete theme is not named whole
// on the frame it was drawn for. A theme the user cannot read is a theme they
// cannot choose, and a name cut mid-word is the case this holds.
func themePickerAssertEveryThemeNamed(t *testing.T, plain string, m Model, where string) {
	t.Helper()
	for _, def := range m.DotfilesThemes {
		if !def.Complete() {
			continue
		}
		if !strings.Contains(plain, def.Name) {
			t.Errorf("the selector at %s does not name %q whole:\n%s", where, def.Name, plain)
		}
	}
}

// TestThemePickerSaysTheReasonWhenThereIsNothingToChoose covers the state the
// mockup has no panel for: the definitions could not be read, so the selector has
// no card to draw and must state why rather than an empty grid. The heading stays
// and the screen's own reason takes the subtitle's place, at every measured
// terminal, inside the frame.
func TestThemePickerSaysTheReasonWhenThereIsNothingToChoose(t *testing.T) {
	for _, size := range measuredTerminalSizes {
		m := installerFrameModel(t, ScreenThemePicker)
		m.Width, m.Height = size.width, size.height
		m.DotfilesThemes = nil
		m.DotfilesThemesErr = themeDefinitionsNotFoundMessage()

		view := m.View()
		assertScreenFitsTerminal(t, "theme-picker-unavailable", size.width, size.height, view)

		plain := ansiEscape.ReplaceAllString(view, "")
		if !strings.Contains(plain, themePickerHeading) {
			t.Errorf("the selector at %s draws no heading over an empty list", size.name)
		}
		if !strings.Contains(plain, "not switchable") {
			t.Errorf("the selector at %s does not say why there is nothing to choose:\n%s", size.name, plain)
		}
		if strings.Contains(plain, themeCardMarked) || strings.Contains(plain, themeCardUnmarked) {
			// The status bar's dot is the marked glyph and is not a theme row: the rows
			// above it are checked, and the state word and its dot are not.
			for _, line := range strings.Split(plain, "\n") {
				if strings.Contains(line, statusPreview) {
					break
				}
				if strings.Contains(line, themeCardMarked) || strings.Contains(line, themeCardUnmarked) {
					t.Errorf("the selector at %s draws a theme mark with no theme to mark:\n%s", size.name, plain)
					break
				}
			}
		}
	}
}

// TestThemeCardTextNamesTheToolsTheThemeLeavesOut keeps the fact the option row
// has always carried on the card the screen now draws: a theme that cannot paint
// every tool says so where the choice is made, in the same words the row uses. The
// label is one function both shapes read, so the wide grid and the floor's list
// cannot disagree about it.
func TestThemeCardTextNamesTheToolsTheThemeLeavesOut(t *testing.T) {
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	def, ok := themeByID(defs, defaultThemeID)
	if !ok {
		t.Fatalf("%s is not defined", defaultThemeID)
	}
	if got := themeCardText(def); got != def.Name {
		t.Errorf("a theme that paints every tool is labelled %q, want its name %q", got, def.Name)
	}

	// One role removed from the palette is one tool per file that reads it: the
	// definition the switch would refuse to paint is named on its card.
	broken := def
	broken.Palette = make(map[string]string, len(def.Palette))
	for role, hex := range def.Palette {
		if role == "cursor" {
			continue
		}
		broken.Palette[role] = hex
	}
	_, uncovered := themeCoverage(broken)
	if len(uncovered) == 0 {
		t.Fatal("the fixture left no tool out, so the guard proves nothing")
	}
	got := themeCardText(broken)
	if !strings.Contains(got, broken.Name) {
		t.Errorf("the label %q does not name the theme", got)
	}
	for _, tool := range uncovered {
		if !strings.Contains(got, tool) {
			t.Errorf("the label %q does not name %q, which the theme leaves out", got, tool)
		}
	}
	if !strings.Contains(got, "not ") {
		t.Errorf("the label %q does not read as a list of what is left out", got)
	}
}

// TestThemeCardMiniaturePaintsTheDefinitionsOwnPalette is the no-second-palette
// rule for the picture inside a card: every colour the miniature fills a cell with
// is one the definition holds, and the roles the mockup's picture is built from --
// the base, the selected row and the accent chip -- are painted.
//
// lipgloss writes an eight-bit channel through sixteen bits, which can move one by
// one, so a colour is matched to the palette within one step and no further: a
// miniature holding a different palette would be off by far more.
func TestThemeCardMiniaturePaintsTheDefinitionsOwnPalette(t *testing.T) {
	previousProfile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previousProfile) })

	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	fillRE := regexp.MustCompile(`48;2;(\d+);(\d+);(\d+)`)

	for _, def := range defs {
		if !def.Complete() {
			continue
		}
		owned := map[string]miniColor{}
		for role, hex := range def.Palette {
			if color, ok := parseMiniColor(hex); ok {
				owned[role] = color
			}
		}

		lines := themeMiniatureLines(def, 40, themeMiniMaxRows)
		if len(lines) != themeMiniMaxRows {
			t.Fatalf("theme %q drew %d miniature row(s), want %d", def.ID, len(lines), themeMiniMaxRows)
		}
		painted := map[string]bool{}
		for _, line := range lines {
			for _, match := range fillRE.FindAllStringSubmatch(line, -1) {
				got := miniColor{channelOrFail(t, match[1]), channelOrFail(t, match[2]), channelOrFail(t, match[3])}
				roles := miniColorRoles(owned, got)
				if len(roles) == 0 {
					t.Errorf("theme %q paints the miniature with %+v, which is not in its palette", def.ID, got)
					continue
				}
				for _, role := range roles {
					painted[role] = true
				}
			}
		}
		for _, role := range []string{"base", "blue", "selection"} {
			if _, ok := owned[role]; ok && !painted[role] {
				t.Errorf("theme %q does not paint %s (%s) in its miniature", def.ID, role, def.Palette[role])
			}
		}
	}
}

// miniColor is one rendered colour, channel by channel.
type miniColor struct {
	r, g, b int
}

// parseMiniColor reads a #rrggbb colour into its channels.
func parseMiniColor(hex string) (miniColor, bool) {
	if !themeHexRE.MatchString(hex) {
		return miniColor{}, false
	}
	var out miniColor
	for i, dst := range []*int{&out.r, &out.g, &out.b} {
		value, err := strconv.ParseUint(hex[1+i*2:3+i*2], 16, 8)
		if err != nil {
			return miniColor{}, false
		}
		*dst = int(value)
	}
	return out, true
}

// miniColorRoles is every palette role a rendered colour is, within the one step
// lipgloss's rounding can move a channel. It returns all of them because a palette
// may carry the same colour under two roles, and a run of that colour paints both.
func miniColorRoles(owned map[string]miniColor, got miniColor) []string {
	closeEnough := func(a, b int) bool { return a-b <= 1 && b-a <= 1 }
	var roles []string
	for role, want := range owned {
		if closeEnough(want.r, got.r) && closeEnough(want.g, got.g) && closeEnough(want.b, got.b) {
			roles = append(roles, role)
		}
	}
	return roles
}

// channelOrFail reads one channel of a rendered sequence.
func channelOrFail(t *testing.T, channel string) int {
	t.Helper()
	value, err := strconv.Atoi(channel)
	if err != nil {
		t.Fatalf("read the rendered channel %q: %v", channel, err)
	}
	return value
}
