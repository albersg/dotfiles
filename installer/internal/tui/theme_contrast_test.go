package tui

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPreviewMutedRoleStaysLegibleOnTheDrift guards the contrast fix the VHS
// captures found: the muted role is metadata, and the interface may draw it dim
// -- several themes do, deliberately -- but the drifting background must not
// make it dimmer than the plain base does. A light theme is held to the 4.5:1
// body floor as well, because Catppuccin Latte's subtext0 read at 3.7:1 on the
// drift end and the preview was nearly unreadable.
func TestPreviewMutedRoleStaysLegibleOnTheDrift(t *testing.T) {
	paths, err := filepath.Glob("../../../themes/*.toml")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no theme definitions found: %v", err)
	}

	const bodyFloor = 4.5
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		def, err := parseThemeDefinition(data)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if def.Partial {
			continue
		}
		colors, err := themePreviewColors(def)
		if err != nil {
			t.Fatalf("%s cannot be previewed: %v", path, err)
		}
		// Both adaptive members carry the definition's own value, so either one
		// is the colour the preview paints.
		muted := colors.TextMuted.Dark
		base := def.Palette["base"]
		baseRatio := themeContrastRatio(muted, base)

		if themeBaseIsLight(def) && baseRatio < bodyFloor {
			t.Errorf("%s: light theme's muted %s on base %s is %.2f:1, want at least %.1f:1",
				path, muted, base, baseRatio, bodyFloor)
		}

		// The footer's key tokens take the Key role; they are body-sized text on
		// the base, so they get the same floor.
		key := colors.Key.Dark
		if ratio := themeContrastRatio(key, base); ratio < bodyFloor {
			t.Errorf("%s: the key tone %s on base %s is %.2f:1, want at least %.1f:1",
				path, key, base, ratio, bodyFloor)
		}

		accent := def.Palette["cursor"]
		if !themeHexRE.MatchString(accent) {
			continue
		}
		// The drift's floor: 4.5:1 when the plain base already clears it, 3.0:1
		// for the themes whose muted role is deliberately dimmer.
		floor := bodyFloor
		if baseRatio < bodyFloor {
			floor = 3.0
		}
		if floor > baseRatio {
			floor = baseRatio
		}
		cap := driftTintCapFor(parseHexColour(base), parseHexColour(accent), muted)
		if cap <= 0 {
			t.Errorf("%s: the drift tint is zero, so the background no longer moves", path)
		}
		drift := hexColour(blendColour(parseHexColour(base), parseHexColour(accent), cap))
		driftRatio := themeContrastRatio(muted, drift)
		if driftRatio < floor-0.02 {
			t.Errorf("%s: the drift takes muted %s to %.2f:1 on %s, under its %.1f:1 floor",
				path, muted, driftRatio, drift, floor)
		}
		if themeBaseIsLight(def) && driftRatio < bodyFloor {
			t.Errorf("%s: light theme's muted %s on the drift end %s is %.2f:1, want at least %.1f:1",
				path, muted, drift, driftRatio, bodyFloor)
		}
	}
}
