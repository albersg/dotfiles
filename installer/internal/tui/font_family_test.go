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
// The installer downloads Meslo.zip from Nerd Fonts v3, whose patcher appends
// " Nerd Font" to the family name. The v2 releases used " NF". A terminal
// configured with the wrong name does not fail: it falls back to a default face
// silently, which is indistinguishable from the icons simply not working. That is
// the same silent fallback issue #261 is about, and it was found in the installer
// first and then in three of the four terminal configurations. The Linux step now
// installs Meslo, so an Iosevka family anywhere in a shipped configuration is the
// defect again.
//
// The family is read from the constant the installer already uses when it writes
// the face on the Windows side, so this guard has one source of truth rather than
// a second copy of the name.
func TestEveryTerminalConfigNamesTheFamilyTheArchiveProvides(t *testing.T) {
	// The shipped configurations that name a font family. They are the files
	// themselves, not a copy of the list: each is read and every Nerd Font family
	// it names must be the one the archive provides.
	configs := []string{
		"../../../alacritty.toml",
		"../../../.wezterm.lua",
		"../../../dotfiles-ghostty/config",
		"../../../dotfiles-kitty/kitty.conf",
	}

	// Any font family that starts with Iosevka or Meslo, however it is quoted or
	// assigned. The pattern follows the family into digits, dots, underscores and
	// hyphens so a name like IosevkaTerm2 or MesloLGS-NF is read rather than
	// slipped past, which is what the comment promises. Matching Iosevka as well
	// as Meslo is deliberate: an Iosevka family left in a configuration is
	// exactly the half-migrated state this guard must fail on.
	familyRE := regexp.MustCompile(`(?:Iosevka|Meslo)[A-Za-z0-9_. -]*`)

	filesRead := 0
	familiesRead := 0
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
		filesRead++
		matches := familyRE.FindAllString(string(body), -1)
		if len(matches) == 0 {
			t.Errorf("%s names no Nerd Font family at all, so this guard is not looking at the file it names", rel)
			continue
		}
		for _, match := range matches {
			familiesRead++
			match = strings.TrimSpace(match)
			if match == windowsTerminalFontFace {
				continue
			}
			t.Errorf("%s names the family %q, and the installer ships Meslo, whose family is %q. An Iosevka family in a shipped configuration is the defect: a terminal naming a family that does not resolve falls back silently, so the icons simply never appear.",
				rel, match, windowsTerminalFontFace)
		}
	}

	// The completeness gate counts the files actually read, not the families
	// found: one configuration naming two families must not mask another that
	// read none.
	if filesRead < len(configs) {
		t.Fatalf("read %d of %d configuration files (%d families): this guard is not looking at every file it names", filesRead, len(configs), familiesRead)
	}
}

// TestNoShippedConfigNamesIosevka is the other half, and after the move to Meslo
// it is the half that would have caught the defect on its own: the Iosevka family
// must not appear anywhere in a configuration the installer ships. It is checked
// separately from the family comparison because a stale line copied from an older
// configuration is exactly what a future edit is most likely to reintroduce.
func TestNoShippedConfigNamesIosevka(t *testing.T) {
	roots := []string{"../../../alacritty.toml", "../../../.wezterm.lua", "../../../dotfiles-ghostty", "../../../dotfiles-kitty"}
	iosevka := regexp.MustCompile(`Iosevka`)

	filesRead := 0
	for _, root := range roots {
		path, err := filepath.Abs(root)
		if err != nil {
			t.Fatalf("resolving %s: %v", root, err)
		}
		read := 0
		err = filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || strings.Contains(p, ".git") {
				return nil
			}
			body, err := os.ReadFile(p)
			if err != nil {
				return nil
			}
			read++
			filesRead++
			if m := iosevka.FindString(string(body)); m != "" {
				t.Errorf("%s names %q: the installer ships the Meslo archive, whose family is %q", p, m, windowsTerminalFontFace)
			}
			return nil
		})
		if err != nil {
			t.Errorf("walking %s: %v", root, err)
		}
		if read == 0 {
			t.Errorf("walked %s but read no file: this guard is not looking at the configuration it names", root)
		}
	}

	// A walk that read nothing at all would pass while checking nothing, so the
	// total is gated on the number of roots this guard claims to walk.
	if filesRead < len(roots) {
		t.Fatalf("read %d files across %d configuration roots: this guard is not looking at the configurations it names", filesRead, len(roots))
	}
}

// TestTheInstallerInstallsTheFamilyTheConfigsName ties the archive the font step
// downloads to the family the configurations name and to the family the Windows
// writer resolves against. If the step downloads one family while a config or the
// writer names another, the icons fall back silently and nothing on screen says
// so; this guard is what makes the three names one decision.
func TestTheInstallerInstallsTheFamilyTheConfigsName(t *testing.T) {
	if !strings.Contains(mesloArchiveURL, "nerd-fonts") || !strings.Contains(mesloArchiveURL, "Meslo.zip") {
		t.Errorf("the font step must download the Meslo archive, got %q", mesloArchiveURL)
	}
	if mesloArchiveName != "Meslo.zip" {
		t.Errorf("the font step's archive name = %q, want Meslo.zip", mesloArchiveName)
	}
	if !strings.HasPrefix(windowsTerminalFontFace, "Meslo") {
		t.Errorf("the family the configurations name is %q, but the archive the step downloads is Meslo", windowsTerminalFontFace)
	}
	if !strings.HasPrefix(windowsFontFamilyFilePrefix, "Meslo") {
		t.Errorf("the file name prefix the Windows copy reads is %q, but the archive the step downloads is Meslo", windowsFontFamilyFilePrefix)
	}
}

// TestWindowsTerminalFaceIsResolvedBeforeItIsNamed is the guard for the "name
// what resolves" rule. The face written to Windows Terminal must be the one the
// Windows font store answers for, preferring the machine's own Meslo build and
// falling back to the family the archive provides. A hand-written name is the
// defect, because a family the store does not resolve falls back silently. The
// query boundary is injected, so no test reaches a real Windows side.
func TestWindowsTerminalFaceIsResolvedBeforeItIsNamed(t *testing.T) {
	// The preference order is the rule, not an accident of the candidate list.
	if len(windowsFontFaceCandidates) < 2 {
		t.Fatalf("the candidate list must hold the machine's own build and the fallback: %v", windowsFontFaceCandidates)
	}
	if windowsFontFaceCandidates[0] != windowsTerminalFontFacePreferred {
		t.Errorf("the machine's own family %q must be preferred, got %v", windowsTerminalFontFacePreferred, windowsFontFaceCandidates)
	}
	if windowsFontFaceCandidates[1] != windowsTerminalFontFace {
		t.Errorf("the archive family %q must be the fallback, got %v", windowsTerminalFontFace, windowsFontFaceCandidates)
	}

	for _, tc := range []struct {
		name      string
		installed map[string]bool
		want      string
		wantOK    bool
	}{
		{"the machine's own build resolves", map[string]bool{windowsTerminalFontFacePreferred: true}, windowsTerminalFontFacePreferred, true},
		{"only the archive family resolves", map[string]bool{windowsTerminalFontFace: true}, windowsTerminalFontFace, true},
		{"neither resolves", map[string]bool{}, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stubWindowsFontQueryStateful(t, tc.installed)
			got, ok := installedWindowsFontFace()
			if got != tc.want || ok != tc.wantOK {
				t.Errorf("installedWindowsFontFace() = (%q, %v), want (%q, %v)", got, ok, tc.want, tc.wantOK)
			}
		})
	}
}
