package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// captureHerdrLog records what the Herdr step reports. The step's log sink is a
// variable for exactly this: Herdr's silence is the defect, so the guard has to
// read the installer's own account of a missing player.
func captureHerdrLog(t *testing.T) *[]string {
	t.Helper()

	lines := []string{}
	restore := herdrLog
	herdrLog = func(_, line string) { lines = append(lines, line) }
	t.Cleanup(func() { herdrLog = restore })
	return &lines
}

// herdrLookup returns a lookup over the given names, for tests that describe a
// machine instead of touching one.
func herdrLookup(present ...string) func(string) bool {
	set := map[string]bool{}
	for _, name := range present {
		set[name] = true
	}
	return func(command string) bool { return set[command] }
}

func herdrLogContains(lines []string, want string) bool {
	for _, line := range lines {
		if strings.Contains(line, want) {
			return true
		}
	}
	return false
}

// TestHerdrSoundPlayerFollowsHerdrOrder is the resolution rule. Herdr tries its
// players in a fixed order and the installer must answer with the one that would
// actually fire, not an arbitrary member of the set.
func TestHerdrSoundPlayerFollowsHerdrOrder(t *testing.T) {
	for _, tc := range []struct {
		name    string
		present []string
		want    string
	}{
		{name: "no player resolves", present: nil, want: ""},
		{name: "paplay is Herdr's first choice", present: []string{"mpg123", "paplay", "mpv"}, want: "paplay"},
		{name: "ffplay beats later players", present: []string{"mpv", "mpg123", "ffplay"}, want: "ffplay"},
		{name: "mpg123 alone", present: []string{"mpg123"}, want: "mpg123"},
		{name: "a decoder Herdr does not try is not a player", present: []string{"vlc", "afplay", "sox"}, want: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := herdrSoundPlayer(herdrLookup(tc.present...)); got != tc.want {
				t.Errorf("herdrSoundPlayer() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestParseHerdrSoundEnabled reads the boolean out of the small TOML subset the
// managed file uses. A value outside [ui.sound] must not turn sound on.
func TestParseHerdrSoundEnabled(t *testing.T) {
	for _, tc := range []struct {
		name string
		toml string
		want bool
	}{
		{name: "enabled", toml: "[ui.sound]\nenabled = true\n", want: true},
		{name: "disabled", toml: "[ui.sound]\nenabled = false\n", want: false},
		{name: "trailing comment", toml: "[ui.sound]\nenabled = true # play alerts\n", want: true},
		{name: "no sound table", toml: "[ui]\naccent = \"#7fb4ca\"\n", want: false},
		{name: "enabled outside the table", toml: "enabled = true\n[ui.sound]\nother = 1\n", want: false},
		{name: "comments only", toml: "# [ui.sound]\n# enabled = true\n", want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseHerdrSoundEnabled([]byte(tc.toml)); got != tc.want {
				t.Errorf("parseHerdrSoundEnabled(%q) = %v, want %v", tc.toml, got, tc.want)
			}
		})
	}
}

// TestHerdrSoundShippedConfigAndProvisionAgree is the repo-level guard for the
// defect: the configuration the installer ships may only turn sound on if the
// installer also declares the player that makes it able to fire. The moment a
// change enables sound without a provisioned player, or removes the player while
// the configuration still enables it, this fails.
func TestHerdrSoundShippedConfigAndProvisionAgree(t *testing.T) {
	config, err := os.ReadFile(filepath.Join(repoRoot(t), repoAssetHerdrConfig))
	if err != nil {
		t.Fatal(err)
	}
	if !parseHerdrSoundEnabled(config) {
		t.Skip("the managed Herdr configuration does not enable sound, so no player is promised")
	}

	if herdrSoundProvision == "" {
		t.Fatal("the managed configuration enables sound but the installer names no player")
	}

	brewfile, err := os.ReadFile(filepath.Join(repoRoot(t), "Brewfile"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(brewfile), `brew "`+herdrSoundProvision+`"`) {
		t.Errorf("the managed configuration enables sound, but the Brewfile does not declare %q", herdrSoundProvision)
	}

	pkgs := herdrSoundPlatformPackages()
	for _, column := range []struct {
		name     string
		packages string
	}{
		{"Brew", pkgs.Brew},
		{"Debian", pkgs.Debian},
		{"Arch", pkgs.Arch},
		{"Fedora", pkgs.Fedora},
	} {
		if !strings.Contains(" "+column.packages+" ", " "+herdrSoundProvision+" ") {
			t.Errorf("the %s package list %q does not carry the player %q the managed configuration needs",
				column.name, column.packages, herdrSoundProvision)
		}
	}
}

// TestEnsureHerdrSoundProvisionsPlayerWhenNoneResolves is the guard that the
// installer closes the gap it opened. With sound enabled and no player on the
// machine, the step must ask the package manager for the declared player, and it
// must confirm the player resolves afterwards before it reports ready.
func TestEnsureHerdrSoundProvisionsPlayerWhenNoneResolves(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	calls := withPackageCommandMocks(t, nil)

	configPath := filepath.Join(home, "config.toml")
	if err := os.WriteFile(configPath, []byte("[ui.sound]\nenabled = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// The machine resolves mpg123 only once the package manager has been asked
	// for it, so the reply describes the machine after the install.
	restore := herdrSoundPlayerPresent
	herdrSoundPlayerPresent = func(command string) bool {
		if command != herdrSoundProvision {
			return false
		}
		for _, call := range *calls {
			if strings.Contains(call.command, herdrSoundProvision) {
				return true
			}
		}
		return false
	}
	t.Cleanup(func() { herdrSoundPlayerPresent = restore })

	lines := captureHerdrLog(t)
	m := NewModel()
	m.SystemInfo = &system.SystemInfo{OS: system.OSMac, HasBrew: true}

	if ok := ensureHerdrSound(&m, "wm", configPath); !ok {
		t.Fatalf("sound must be ready once the player is provisioned; logs = %v", *lines)
	}

	found := false
	for _, call := range *calls {
		if call.runner == "brew" && strings.Contains(call.command, herdrSoundProvision) {
			found = true
		}
	}
	if !found {
		t.Errorf("sound is enabled and no player resolves, but the step never asked for %q: calls = %#v", herdrSoundProvision, *calls)
	}
	if !herdrLogContains(*lines, herdrSoundProvision) {
		t.Errorf("the provisioned player is not named in the report: %v", *lines)
	}
}

// TestEnsureHerdrSoundReportsMissingPlayer is the guard against Herdr's silence.
// When sound is enabled and even the install cannot make a player resolve, the
// step must say so and must report itself not ready, which is what withholds the
// success line from the step.
func TestEnsureHerdrSoundReportsMissingPlayer(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	withPackageCommandMocks(t, nil)

	configPath := filepath.Join(home, "config.toml")
	if err := os.WriteFile(configPath, []byte("[ui.sound]\nenabled = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	restore := herdrSoundPlayerPresent
	herdrSoundPlayerPresent = func(string) bool { return false }
	t.Cleanup(func() { herdrSoundPlayerPresent = restore })

	lines := captureHerdrLog(t)
	m := NewModel()
	m.SystemInfo = &system.SystemInfo{OS: system.OSMac, HasBrew: true}

	if ok := ensureHerdrSound(&m, "wm", configPath); ok {
		t.Fatalf("sound must not be reported ready while no player resolves; logs = %v", *lines)
	}

	report := strings.Join(*lines, "\n")
	if !strings.Contains(report, "cannot play") {
		t.Errorf("the step hides the missing player instead of naming it:\n%s", report)
	}
	for _, player := range herdrSoundPlayers {
		if !strings.Contains(report, player) {
			t.Errorf("the report does not name %q among the players Herdr tries:\n%s", player, report)
		}
	}
}

// TestEnsureHerdrSoundSkipsPlayerWhenDisabled is the guard against installing a
// player on a machine that did not ask for sound. A configuration that does not
// enable sound must leave the package manager untouched and still report ready.
func TestEnsureHerdrSoundSkipsPlayerWhenDisabled(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	calls := withPackageCommandMocks(t, nil)

	configPath := filepath.Join(home, "config.toml")
	if err := os.WriteFile(configPath, []byte("[ui.sound]\nenabled = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	restore := herdrSoundPlayerPresent
	herdrSoundPlayerPresent = func(string) bool { return false }
	t.Cleanup(func() { herdrSoundPlayerPresent = restore })

	lines := captureHerdrLog(t)
	m := NewModel()
	m.SystemInfo = &system.SystemInfo{OS: system.OSMac, HasBrew: true}

	if ok := ensureHerdrSound(&m, "wm", configPath); !ok {
		t.Fatalf("a configuration that disables sound is ready by definition; logs = %v", *lines)
	}
	if len(*calls) != 0 {
		t.Errorf("sound is disabled but the step installed a player: calls = %#v", *calls)
	}
}

// TestStepInstallWMHerdrSoundFailureWithholdsSuccess is the step-level guard:
// when the sound player cannot be provided, the step must not close with the
// unqualified "✓ Herdr configured" success line. That is the silent success the
// issue describes, since Herdr itself never says the alert could not fire.
func TestStepInstallWMHerdrSoundFailureWithholdsSuccess(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	withPackageCommandMocks(t, nil)

	restoreBinary := herdrBinaryPresent
	herdrBinaryPresent = func(string) bool { return true }
	t.Cleanup(func() { herdrBinaryPresent = restoreBinary })

	restorePlayer := herdrSoundPlayerPresent
	herdrSoundPlayerPresent = func(string) bool { return false }
	t.Cleanup(func() { herdrSoundPlayerPresent = restorePlayer })

	lines := captureHerdrLog(t)
	m := NewModel()
	m.SystemInfo = &system.SystemInfo{OS: system.OSMac, HasBrew: true}
	m.Choices = UserChoices{OS: "mac", Shell: "zsh", WindowMgr: "herdr"}
	m.RepoDir = repoRoot(t)

	if err := stepInstallWM(&m); err != nil {
		t.Fatalf("herdr step failed: %v", err)
	}

	report := strings.Join(*lines, "\n")
	if strings.Contains(report, "✓ Herdr configured") {
		t.Errorf("the step reports success for a configuration whose dependency is missing:\n%s", report)
	}
	if !strings.Contains(report, "will stay silent") {
		t.Errorf("the step does not explain the silence it leaves behind:\n%s", report)
	}
}

// TestStepInstallWMHerdrSoundReadyReportsSuccess is the other half of the same
// guard: a machine where a player resolves still gets the plain success line,
// and the step names the player Herdr will use.
func TestStepInstallWMHerdrSoundReadyReportsSuccess(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	calls := withPackageCommandMocks(t, nil)

	restoreBinary := herdrBinaryPresent
	herdrBinaryPresent = func(string) bool { return true }
	t.Cleanup(func() { herdrBinaryPresent = restoreBinary })

	restorePlayer := herdrSoundPlayerPresent
	herdrSoundPlayerPresent = herdrLookup("paplay")
	t.Cleanup(func() { herdrSoundPlayerPresent = restorePlayer })

	lines := captureHerdrLog(t)
	m := NewModel()
	m.SystemInfo = &system.SystemInfo{OS: system.OSMac, HasBrew: true}
	m.Choices = UserChoices{OS: "mac", Shell: "zsh", WindowMgr: "herdr"}
	m.RepoDir = repoRoot(t)

	if err := stepInstallWM(&m); err != nil {
		t.Fatalf("herdr step failed: %v", err)
	}

	if !herdrLogContains(*lines, "✓ Herdr configured") {
		t.Errorf("a ready machine must get the success line; logs = %v", *lines)
	}
	if !herdrLogContains(*lines, "Herdr will use paplay") {
		t.Errorf("the step does not name the player that will fire; logs = %v", *lines)
	}
	for _, call := range *calls {
		if strings.Contains(call.command, herdrSoundProvision) {
			t.Errorf("a player already resolves, but the step installed another one: %#v", call)
		}
	}
}
