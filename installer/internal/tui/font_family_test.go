package tui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestEveryTerminalConfigNamesTheFamilyTheArchiveProvides is the guard for a
// defect that has now appeared twice.
//
// The installer downloads IosevkaTerm.zip from Nerd Fonts v3, whose patcher
// appends " Nerd Font" to the family name. The v2 releases used " NF". A terminal
// configured with the wrong name does not fail: it falls back to a default face
// silently, which is indistinguishable from the icons simply not working. That is
// the same silent fallback issue #261 is about, and it was found in the installer
// first and then in three of the four terminal configurations.
//
// The family is read from the constant the installer already uses when it writes
// the face on the Windows side, so this guard has one source of truth rather than
// a second copy of the name.
func TestEveryTerminalConfigNamesTheFamilyTheArchiveProvides(t *testing.T) {
	// The shipped configurations that name a font family. They are the files
	// themselves, not a copy of the list: each is read and every Iosevka family it
	// names must be the one the archive provides.
	configs := []string{
		"../../../alacritty.toml",
		"../../../.wezterm.lua",
		"../../../dotfiles-ghostty/config",
		"../../../dotfiles-kitty/kitty.conf",
	}

	// Any family that starts with Iosevka, however it is quoted or assigned.
	familyRE := regexp.MustCompile(`Iosevka[A-Za-z ]*`)

	checked := 0
	for _, rel := range configs {
		path, err := filepath.Abs(rel)
		if err != nil {
			t.Fatalf("resolving %s: %v", rel, err)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("reading %s: %v", rel, err)
			continue
		}
		for _, match := range familyRE.FindAllString(string(body), -1) {
			checked++
			if match != windowsTerminalFontFace {
				t.Errorf("%s names the family %q, and the archive the installer downloads provides %q. A terminal naming a family that does not resolve falls back silently, so the icons simply never appear.",
					rel, match, windowsTerminalFontFace)
			}
		}
	}

	// A guard that read no family at all would pass while checking nothing.
	if checked < len(configs) {
		t.Fatalf("read %d font families from %d configurations: this guard is not looking at the files it names", checked, len(configs))
	}
}

// TestNoShippedConfigUsesTheNerdFontsV2Name is the other half, and it is the half
// that would have caught the defect on its own: the v2 spelling must not appear
// anywhere in a configuration the installer ships. It is checked separately from
// the family comparison because "NF" is what a future edit is most likely to
// reintroduce, copied from an older release's documentation.
func TestNoShippedConfigUsesTheNerdFontsV2Name(t *testing.T) {
	roots := []string{"../../../alacritty.toml", "../../../.wezterm.lua", "../../../dotfiles-ghostty", "../../../dotfiles-kitty"}
	v2 := regexp.MustCompile(`Iosevka[A-Za-z]* (NF|Nerd Font Mono)\b`)

	found := 0
	for _, root := range roots {
		path, err := filepath.Abs(root)
		if err != nil {
			t.Fatalf("resolving %s: %v", root, err)
		}
		err = filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || strings.Contains(p, ".git") {
				return nil
			}
			body, err := os.ReadFile(p)
			if err != nil {
				return nil
			}
			if m := v2.FindString(string(body)); m != "" {
				found++
				t.Errorf("%s names %q: that is the Nerd Fonts v2 spelling, and the installer ships v3, whose family is %q", p, m, windowsTerminalFontFace)
			}
			return nil
		})
		if err != nil {
			t.Errorf("walking %s: %v", root, err)
		}
	}
	if found > 0 {
		t.Logf("%d file(s) still carry the v2 spelling", found)
	}
}
