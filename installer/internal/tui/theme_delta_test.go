package tui

import (
	"regexp"
	"testing"
)

// The generated [delta] block used to put the palette's own green and red in the
// background slot of delta's style string ("normal #rrggbb"), which painted every
// changed line a saturated field and left the syntax highlighting of the line on
// top of it. A diff that cannot be read is worse than no diff colour at all.
//
// The rule this holds: a changed line is marked by the colour of its text, never by
// a filled line, and the colour is whichever of the palette's own normal and bright
// variant reads better on the theme's base. Both are values the definition already
// carries, so the choice cannot invent a colour; it can only refuse the weaker one.
func TestTheDeltaDiffMarksTheTextAndNeverFillsTheLine(t *testing.T) {
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the definitions: %v", err)
	}
	if len(defs) < 7 {
		t.Fatalf("expected the seven offered definitions, got %d", len(defs))
	}

	pattern := regexp.MustCompile(`(?m)^\t(plus|minus)(-emph)?-style = "([^"]*)"$`)
	for _, def := range defs {
		block, err := renderDeltaTheme(def)
		if err != nil {
			t.Fatalf("%s: %v", def.ID, err)
		}
		base := def.Palette["base"]
		if base == "" {
			t.Fatalf("%s: no base, so no contrast can be measured", def.ID)
		}
		want := map[string]string{
			"plus":  strongerOn(def, base, "green", "bright_green"),
			"minus": strongerOn(def, base, "red", "bright_red"),
		}

		seen := map[string]bool{}
		for _, match := range pattern.FindAllStringSubmatch(block, -1) {
			role, emph, value := match[1], match[2] == "-emph", match[3]
			expected := want[role]
			if emph {
				expected = "bold " + expected
			}
			if value != expected {
				t.Errorf("%s: %s-style is %q, want %q", def.ID, role+match[2], value, expected)
			}
			seen[role] = true
		}
		if !seen["plus"] || !seen["minus"] {
			t.Errorf("%s: the block does not style both marks: plus=%v minus=%v", def.ID, seen["plus"], seen["minus"])
		}
	}
}

// strongerOn returns the candidate whose colour reads better on base.
func strongerOn(def themeDefinition, base string, roles ...string) string {
	best, bestRatio := "", -1.0
	for _, role := range roles {
		value := def.Palette[role]
		if ratio := themeContrastRatio(value, base); ratio > bestRatio {
			best, bestRatio = value, ratio
		}
	}
	return best
}
