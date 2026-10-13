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
// The family is read from mesloArchiveFamily, the one family the pinned archive
// declares; every other name (the Windows registration and the shipped
// configurations) derives from it, so this guard has one source of truth rather
// than a second copy of the name.
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
			if match == mesloArchiveFamily {
				continue
			}
			t.Errorf("%s names the family %q, and the installer ships Meslo, whose family is %q. An Iosevka family in a shipped configuration is the defect: a terminal naming a family that does not resolve falls back silently, so the icons simply never appear.",
				rel, match, mesloArchiveFamily)
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
				t.Errorf("%s names %q: the installer ships the Meslo archive, whose family is %q", p, m, mesloArchiveFamily)
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

// TestOneMesloFamilyDecidesEveryName is the one-place guard for T8. The v3
// archive and the macOS cask declare "MesloLGS Nerd Font". The Windows registry
// name must be that same family rather than a second name that can drift from it,
// and the Termux face is the same family. Everything the installer writes reads
// this one constant; the shipped configurations are held to it by
// TestEveryTerminalConfigNamesTheFamilyTheArchiveProvides above.
func TestOneMesloFamilyDecidesEveryName(t *testing.T) {
	if windowsTerminalFontFace != mesloArchiveFamily {
		t.Errorf("windowsTerminalFontFace = %q, but the one family the installer installs is %q; the Windows registration name must derive from it, not be typed again", windowsTerminalFontFace, mesloArchiveFamily)
	}
	if termuxFontFace != mesloArchiveFamily {
		t.Errorf("termuxFontFace = %q, but the one family the installer installs is %q", termuxFontFace, mesloArchiveFamily)
	}
	if windowsTerminalFontFacePreferred == mesloArchiveFamily {
		t.Errorf("windowsTerminalFontFacePreferred = %q must stay the machine's own build, distinct from the archive family", windowsTerminalFontFacePreferred)
	}
}

// TestTheTermuxStepInstallsTheFaceTheDecisionNames is the guard for the Termux
// half of issue #269. The decision recorded in #265 is that the font is Meslo,
// for every terminal, and the archive the installer already pins for Linux
// carries the same family. The step used to fetch JetBrainsMono while the wizard
// said Meslo; this guard makes the label and the asset one decision, so neither
// can be "fixed" to match the other without failing here.
//
// The URL was verified by hand, not by this test: the family below was read from
// the downloaded face's own `name` table (fc-scan on the v3-patched Meslo S
// face, which declares "MesloLGS Nerd Font"). The test touches no network and
// only checks that the URL and the label name the same face.
func TestTheTermuxStepInstallsTheFaceTheDecisionNames(t *testing.T) {
	if !strings.Contains(termuxFontURL, "patched-fonts/Meslo/") {
		t.Errorf("the Termux step must download a Meslo face, got %q", termuxFontURL)
	}
	if !strings.Contains(termuxFontURL, "MesloLGSNerdFont-Regular.ttf") {
		t.Errorf("the Termux step must download the Meslo S Regular face, got %q", termuxFontURL)
	}
	if strings.Contains(termuxFontURL, "JetBrainsMono") {
		t.Errorf("the Termux step still downloads the JetBrainsMono face: %q", termuxFontURL)
	}
	// The family the downloaded face declares, read from its name table: the v3
	// "MesloLGS Nerd Font". The older v2 naming and the JetBrainsMono face this
	// step used to fetch are both different names, and neither is what this face
	// declares.
	if termuxFontFace != "MesloLGS Nerd Font" {
		t.Errorf("termuxFontFace = %q, want the family the downloaded Meslo S face declares: %q", termuxFontFace, "MesloLGS Nerd Font")
	}
	if termuxFontDirName != ".termux" || termuxFontFileName != "font.ttf" {
		t.Errorf("Termux reads ~/.termux/font.ttf, but the step writes ~/%s/%s", termuxFontDirName, termuxFontFileName)
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

// TestTheInterfaceNamesTheFamilyTheStepInstalls is the guard for T7: every
// interface string that names the font must derive it from the family the font
// step installs, so a screen can never claim Meslo over a face the step did not
// put on the machine.
//
// The family is computed rather than typed beside the label: a non-Meslo family
// is shown as itself, which is what makes a label that always says "Meslo" fail
// here. The five strings are then compared to the same helper the install family
// flows through, so the guard asserts that the interface and the step agree
// rather than pinning a literal name.
func TestTheInterfaceNamesTheFamilyTheStepInstalls(t *testing.T) {
	// The shorthand follows the family: a Meslo face keeps the installer's own
	// phrase, and anything else is named rather than mislabelled.
	for _, tc := range []struct{ family, want string }{
		{mesloArchiveFamily, "Meslo Nerd Font"},
		{termuxFontFace, "Meslo Nerd Font"},
		{windowsTerminalFontFace, "Meslo Nerd Font"},
		{"JetBrainsMono Nerd Font", "JetBrainsMono Nerd Font"},
		{"IosevkaTerm Nerd Font", "IosevkaTerm Nerd Font"},
	} {
		if got := fontInstallLabel(tc.family); got != tc.want {
			t.Errorf("fontInstallLabel(%q) = %q, want %q", tc.family, got, tc.want)
		}
	}

	// Every string the interface shows for the font agrees with the family the
	// step installs on the platform, rather than carrying its own copy of it.
	for _, tc := range []struct {
		name   string
		m      Model
		termux bool
	}{
		{"linux", Model{Screen: ScreenFontSelect, Choices: UserChoices{OS: "linux", InstallFont: true}}, false},
		{"termux", Model{Screen: ScreenFontSelect, Choices: UserChoices{OS: "termux", InstallFont: true}}, true},
	} {
		want := fontInstallLabel(fontInstallFamily(tc.termux))

		opts := tc.m.GetCurrentOptions()
		if len(opts) == 0 || !strings.Contains(opts[0], want) {
			t.Errorf("%s: the wizard row is %q, which does not name the family the step installs (%q)", tc.name, opts, want)
		}
		if desc := tc.m.GetScreenDescription(); !strings.Contains(desc, want) {
			t.Errorf("%s: the requirement line %q does not name the family the step installs (%q)", tc.name, desc, want)
		}

		steps := planFor(planOptions{OS: tc.m.Choices.OS, InstallFont: true, DetectedTermux: tc.termux})
		var fontStep *InstallStep
		for i := range steps {
			if steps[i].ID == "font" {
				fontStep = &steps[i]
				break
			}
		}
		if fontStep == nil {
			t.Fatalf("%s: the plan has no font step", tc.name)
		}
		if !strings.Contains(fontStep.Name, want) {
			t.Errorf("%s: the menu row %q does not name the family the step installs (%q)", tc.name, fontStep.Name, want)
		}
		if !strings.Contains(fontStep.Description, want) {
			t.Errorf("%s: the menu description %q does not name the family the step installs (%q)", tc.name, fontStep.Description, want)
		}

		tc.m.Width, tc.m.Height = 80, 24
		if summary := tc.m.renderComplete(); !strings.Contains(summary, want) {
			t.Errorf("%s: the summary does not name the family the step installs (%q)", tc.name, want)
		}
	}
}
