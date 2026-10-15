package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// testWindowsTerminalThemeDefinition builds a complete palette so the scheme
// builder has every canonical role to map. Each role gets a distinct value, so a
// field wired to the wrong role is caught rather than passing by luck.
func testWindowsTerminalThemeDefinition() themeDefinition {
	palette := make(map[string]string, len(themePaletteRoles))
	for i, role := range themePaletteRoles {
		palette[role] = fmt.Sprintf("#%06x", i+1)
	}
	return themeDefinition{ID: "dotfiles", Name: "dotfiles", Palette: palette}
}

// testWindowsTerminalSettings is the shape Windows Terminal writes: a defaults
// object, a profile list, a schemes array, comments and keys this installer does
// not own. The comments and the unrelated keys are what the editor must keep.
const testWindowsTerminalSettings = `{
    // Windows Terminal settings
    "$schema": "https://example.invalid/schema.json",
    "profiles": {
        // the default profile settings
        "defaults": {
            "padding": "8"
        },
        "list": [
            { "name": "Debian", "guid": "{00000000-0000-0000-0000-000000000000}" }
        ]
    },
    "schemes": [
        { "name": "Campbell", "background": "#0C0C0C", "foreground": "#CCCCCC" }
    ],
    "theme": "dark"
}
`

// newWindowsTerminalProfile builds a fake Windows profile whose only Windows
// Terminal package is the Store build, so discovery is exercised without ever
// reading a real /mnt/c. An empty settings string leaves the file absent.
func newWindowsTerminalProfile(t *testing.T, settings string) (profileDir, settingsPath string) {
	t.Helper()

	profileDir = t.TempDir()
	settingsPath = filepath.Join(profileDir, "AppData", "Local", "Packages",
		"Microsoft.WindowsTerminal_8wekyb3d8bbwe", "LocalState", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if settings != "" {
		if err := os.WriteFile(settingsPath, []byte(settings), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return profileDir, settingsPath
}

// TestWSLStepSuccessNamesWindowsTerminal is the guard for the reported defect:
// on WSL the installer reports success while the program that actually draws the
// screen is Windows Terminal, on the other side of the boundary, which reads
// neither the Nerd Font nor the palette the install wrote. The guard fails the
// moment the step's own output stops naming it and the two settings it needs.
func TestWSLStepSuccessNamesWindowsTerminal(t *testing.T) {
	t.Setenv(envBinfmtDir, t.TempDir())
	repoDir, _, _ := newWSLLayout(t)
	m := wslModel(repoDir, true)

	var lines []string
	restore := wslLog
	wslLog = func(_, line string) { lines = append(lines, line) }
	t.Cleanup(func() { wslLog = restore })

	if err := stepInstallWSLConfig(&m); err != nil {
		t.Fatalf("step failed: %v", err)
	}

	report := strings.Join(lines, "\n")
	for _, want := range []string{
		"Windows Terminal",
		windowsTerminalFontFace,
		windowsTerminalSchemeName,
		".local/share/fonts",
	} {
		if !strings.Contains(report, want) {
			t.Errorf("the WSL step reports success without %q, which is the defect:\n%s", want, report)
		}
	}
}

// TestWindowsTerminalWriteAlwaysBacksUp is the backup guard: a write over the
// user's settings.json must leave the previous bytes beside it first. A
// regression that writes without a backup fails here.
func TestWindowsTerminalWriteAlwaysBacksUp(t *testing.T) {
	profileDir, settingsPath := newWindowsTerminalProfile(t, testWindowsTerminalSettings)
	offer := readWindowsTerminalOffer(profileDir, testWindowsTerminalThemeDefinition(), windowsTerminalFontFace)
	if !offer.Available {
		t.Fatalf("the offer must be available, got: %s", offer.Reason)
	}

	if _, wrote, err := applyWindowsTerminalOffer(offer, true, wslStepID); err != nil || !wrote {
		t.Fatalf("applyWindowsTerminalOffer wrote=%v err=%v, want a write", wrote, err)
	}

	backups, err := filepath.Glob(settingsPath + ".bak-dotfiles-*")
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 1 {
		t.Fatalf("a write must leave exactly one backup beside the file, got %v", backups)
	}
	saved, err := os.ReadFile(backups[0])
	if err != nil {
		t.Fatal(err)
	}
	if string(saved) != testWindowsTerminalSettings {
		t.Errorf("the backup does not hold the settings as they were before the write")
	}
}

// TestWindowsTerminalWriteAbortsWhenTheBackupFails pins the other half of the
// backup guard: a backup that cannot be written aborts the settings write rather
// than leaving the user without one. The containing directory is made read-only,
// so both the backup and any write into it are refused.
func TestWindowsTerminalWriteAbortsWhenTheBackupFails(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: directory permissions are not enforced")
	}

	profileDir, settingsPath := newWindowsTerminalProfile(t, testWindowsTerminalSettings)
	offer := readWindowsTerminalOffer(profileDir, testWindowsTerminalThemeDefinition(), windowsTerminalFontFace)
	if !offer.Available {
		t.Fatalf("the offer must be available: %s", offer.Reason)
	}

	dir := filepath.Dir(settingsPath)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	if _, wrote, err := applyWindowsTerminalOffer(offer, true, wslStepID); err == nil || wrote {
		t.Fatalf("a failed backup must abort the write, wrote=%v err=%v", wrote, err)
	}
	if after, err := os.ReadFile(settingsPath); err != nil {
		t.Fatal(err)
	} else if string(after) != testWindowsTerminalSettings {
		t.Error("the settings file must be unchanged when the backup fails")
	}
}

// TestWindowsTerminalWriteRequiresTheChoice pins the never-write-without-a-choice
// rule: the read that builds the offer changes nothing, and applying it without
// the explicit yes changes nothing and leaves no backup behind.
func TestWindowsTerminalWriteRequiresTheChoice(t *testing.T) {
	profileDir, settingsPath := newWindowsTerminalProfile(t, testWindowsTerminalSettings)
	offer := readWindowsTerminalOffer(profileDir, testWindowsTerminalThemeDefinition(), windowsTerminalFontFace)
	if !offer.Available {
		t.Fatalf("the offer must be available, got: %s", offer.Reason)
	}

	if before, err := os.ReadFile(settingsPath); err != nil {
		t.Fatal(err)
	} else if string(before) != testWindowsTerminalSettings {
		t.Fatal("the read must not change the settings file")
	}

	notice, wrote, err := applyWindowsTerminalOffer(offer, false, wslStepID)
	if err != nil {
		t.Fatalf("declining must not be an error: %v", err)
	}
	if wrote {
		t.Error("declining must not write")
	}
	if !strings.Contains(notice, "unchanged") {
		t.Errorf("the decline notice should say the file was left unchanged, got %q", notice)
	}
	if after, err := os.ReadFile(settingsPath); err != nil {
		t.Fatal(err)
	} else if string(after) != testWindowsTerminalSettings {
		t.Error("declining must leave the settings file exactly as it was")
	}
	if backups, _ := filepath.Glob(settingsPath + ".bak-dotfiles-*"); len(backups) != 0 {
		t.Errorf("declining must leave no backup, got %v", backups)
	}
}

// TestWindowsTerminalWriteReportsWhatTheFileNowHolds is the T4 guard: the report
// after a write must come from the file Windows Terminal reads, not from the
// bytes the write sent. It names the face and the colour scheme the file now
// holds, which is the only evidence that the two settings landed.
func TestWindowsTerminalWriteReportsWhatTheFileNowHolds(t *testing.T) {
	t.Setenv("DOTFILES_DRY_RUN", "")
	profileDir, settingsPath := newWindowsTerminalProfile(t, testWindowsTerminalSettings)
	offer := readWindowsTerminalOffer(profileDir, testWindowsTerminalThemeDefinition(), windowsTerminalFontFace)
	if !offer.Available {
		t.Fatalf("the offer must be available, got: %s", offer.Reason)
	}

	notice, wrote, err := applyWindowsTerminalOffer(offer, true, wslStepID)
	if err != nil || !wrote {
		t.Fatalf("applyWindowsTerminalOffer wrote=%v err=%v, want a write", wrote, err)
	}

	// The file, not the report, is the authority: read it back the same way the
	// offer does and require the report to agree with it.
	after, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	face, faceOK := windowsTerminalCurrentFontFace(after)
	scheme, schemeOK := windowsTerminalCurrentColorScheme(after)
	if !faceOK || face != offer.Face {
		t.Fatalf("the write did not leave the face in the file: face=%q ok=%v, want %q", face, faceOK, offer.Face)
	}
	if !schemeOK || scheme != offer.Scheme {
		t.Fatalf("the write did not leave the scheme in the file: scheme=%q ok=%v, want %q", scheme, schemeOK, offer.Scheme)
	}

	if !strings.Contains(notice, "now reads") {
		t.Errorf("the report must describe what the file now holds, not the bytes that were sent: %q", notice)
	}
	for _, want := range []string{offer.Face, offer.Scheme, settingsPath} {
		if !strings.Contains(notice, want) {
			t.Errorf("the report should name %q, got %q", want, notice)
		}
	}
}

// TestWindowsTerminalWriteReportsTheValueThatDidNotLand is the teeth of the
// read-back: when the file does not hold what the write was supposed to send,
// the report has to say which value did not land instead of claiming a success.
// The offer's updated bytes are replaced with the file's original bytes, so the
// write runs, changes nothing, and the report can only be right by reading the
// file back.
func TestWindowsTerminalWriteReportsTheValueThatDidNotLand(t *testing.T) {
	t.Setenv("DOTFILES_DRY_RUN", "")
	profileDir, settingsPath := newWindowsTerminalProfile(t, testWindowsTerminalSettings)
	offer := readWindowsTerminalOffer(profileDir, testWindowsTerminalThemeDefinition(), windowsTerminalFontFace)
	if !offer.Available {
		t.Fatalf("the offer must be available, got: %s", offer.Reason)
	}

	// The bytes that are sent are the bytes already on disk: nothing lands.
	offer.updated = offer.Original

	notice, wrote, err := applyWindowsTerminalOffer(offer, true, wslStepID)
	if !wrote {
		t.Fatalf("the write must still run and report, wrote=%v", wrote)
	}
	// The contract changed: a landing that did not land whole is a shortfall, so no caller
	// can print a checkmark over a settings file Windows Terminal will not read as sent.
	// The report stays, and it is what names the value that did not land.
	if err == nil {
		t.Errorf("a landing that did not land whole must be a shortfall, not a success")
	}

	if !strings.Contains(notice, "did not land") {
		t.Errorf("a value the file does not hold must be reported as not landing: %q", notice)
	}
	for _, want := range []string{offer.Face, offer.Scheme} {
		if !strings.Contains(notice, want) {
			t.Errorf("the report should name the value that did not land, %q, got %q", want, notice)
		}
	}

	after, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if face, ok := windowsTerminalCurrentFontFace(after); ok {
		t.Fatalf("this test only means something while the file names no face, got %q", face)
	}
}

// TestWindowsTerminalWriteReportsAValueThatDidNotLandBesideOneThatDid
// triangulates the read-back: each value is checked on its own, so a report that
// collapses both into one verdict fails. The scheme lands, the face does not,
// because the face the offer claims differs from the face the bytes write.
func TestWindowsTerminalWriteReportsAValueThatDidNotLandBesideOneThatDid(t *testing.T) {
	t.Setenv("DOTFILES_DRY_RUN", "")
	profileDir, _ := newWindowsTerminalProfile(t, testWindowsTerminalSettings)
	offer := readWindowsTerminalOffer(profileDir, testWindowsTerminalThemeDefinition(), windowsTerminalFontFace)
	if !offer.Available {
		t.Fatalf("the offer must be available, got: %s", offer.Reason)
	}

	// updated still writes windowsTerminalFontFace; change only the face the
	// report is asked to look for, so the scheme matches and the face does not.
	offer.Face = "Some Other Face"

	notice, wrote, err := applyWindowsTerminalOffer(offer, true, wslStepID)
	if !wrote {
		t.Fatalf("the write must still run and report, wrote=%v", wrote)
	}
	// The contract changed: a landing that did not land whole is a shortfall, so no caller
	// can print a checkmark over a settings file Windows Terminal will not read as sent.
	// The report stays, and it is what names the value that did not land.
	if err == nil {
		t.Errorf("a landing that did not land whole must be a shortfall, not a success")
	}

	if !strings.Contains(notice, "did not land") {
		t.Errorf("the face that did not land must be reported: %q", notice)
	}
	if !strings.Contains(notice, offer.Face) {
		t.Errorf("the report should name the face that did not land %q: %q", offer.Face, notice)
	}
	if !strings.Contains(notice, offer.Scheme) {
		t.Errorf("the report should name the scheme that did land %q: %q", offer.Scheme, notice)
	}
}

// TestWindowsTerminalSettingsContentPreservesCommentsAndOtherKeys is the JSONC
// preservation guard: only the two owned keys and the owned scheme change, and
// every comment, unrelated key and the other schemes survive.
func TestWindowsTerminalSettingsContentPreservesCommentsAndOtherKeys(t *testing.T) {
	scheme, err := windowsTerminalSchemeFor(testWindowsTerminalThemeDefinition())
	if err != nil {
		t.Fatalf("windowsTerminalSchemeFor failed: %v", err)
	}

	updated, err := windowsTerminalSettingsContent([]byte(testWindowsTerminalSettings), scheme, windowsTerminalFontFace)
	if err != nil {
		t.Fatalf("windowsTerminalSettingsContent failed: %v", err)
	}
	got := string(updated)

	for _, kept := range []string{
		"// Windows Terminal settings",
		"// the default profile settings",
		`"$schema"`,
		`"padding": "8"`,
		`"name": "Debian"`,
		`"name": "Campbell"`,
		"#0C0C0C",
		`"theme": "dark"`,
	} {
		if !strings.Contains(got, kept) {
			t.Errorf("the edit dropped something it does not own: %q is gone:\n%s", kept, got)
		}
	}

	root, err := parseJSONC(updated)
	if err != nil {
		t.Fatalf("the edited settings are not valid JSONC: %v\n%s", err, got)
	}
	profiles := root.findMember("profiles")
	if profiles == nil {
		t.Fatal("profiles disappeared")
	}
	defaults := profiles.value.findMember("defaults")
	if defaults == nil {
		t.Fatal("profiles.defaults disappeared")
	}
	if face := defaults.value.findMember("font"); face == nil {
		t.Error("profiles.defaults.font was not created")
	} else if member := face.value.findMember("face"); member == nil || member.value.text != windowsTerminalFontFace {
		t.Errorf("profiles.defaults.font.face = %v, want %q", member, windowsTerminalFontFace)
	}
	if cs := defaults.value.findMember("colorScheme"); cs == nil || cs.value.text != windowsTerminalSchemeName {
		t.Errorf("profiles.defaults.colorScheme = %v, want %q", cs, windowsTerminalSchemeName)
	}

	schemes := root.findMember("schemes")
	if schemes == nil || schemes.value.kind != jsoncArray {
		t.Fatal("schemes is not an array after the edit")
	}
	names := map[string]bool{}
	for _, elem := range schemes.value.elements {
		if name := elem.findMember("name"); name != nil {
			names[name.value.text] = true
		}
	}
	if !names["Campbell"] || !names[windowsTerminalSchemeName] {
		t.Errorf("schemes should hold both Campbell and %q, got %v", windowsTerminalSchemeName, names)
	}

	// The owned scheme must carry the palette: its red must be the theme's red.
	owned := findSchemeNamed(t, schemes.value, windowsTerminalSchemeName)
	red := owned.findMember("red")
	wantRed := testWindowsTerminalThemeDefinition().Palette["red"]
	if red == nil || red.value.text != wantRed {
		t.Errorf("the owned scheme's red = %v, want %q", red, wantRed)
	}
}

// findSchemeNamed returns the scheme object a schemes array holds under a name.
func findSchemeNamed(t *testing.T, schemes *jsoncValue, name string) *jsoncValue {
	t.Helper()

	for _, elem := range schemes.elements {
		if member := elem.findMember("name"); member != nil && member.value.text == name {
			return elem
		}
	}
	t.Fatalf("no scheme named %q in %v", name, schemes)
	return nil
}

// TestWindowsTerminalSettingsContentIsIdempotent pins that a second pass over an
// already-written file is the identity: the editor must not keep adding keys or
// duplicate the scheme.
func TestWindowsTerminalSettingsContentIsIdempotent(t *testing.T) {
	scheme, err := windowsTerminalSchemeFor(testWindowsTerminalThemeDefinition())
	if err != nil {
		t.Fatal(err)
	}
	once, err := windowsTerminalSettingsContent([]byte(testWindowsTerminalSettings), scheme, windowsTerminalFontFace)
	if err != nil {
		t.Fatal(err)
	}
	twice, err := windowsTerminalSettingsContent(once, scheme, windowsTerminalFontFace)
	if err != nil {
		t.Fatal(err)
	}
	if string(once) != string(twice) {
		t.Errorf("a second pass changed the file again:\n--- once ---\n%s\n--- twice ---\n%s", once, twice)
	}
}

// TestWindowsTerminalReadOfferRefusesWhatItCannotEditSafely pins the refusal
// paths: unparseable JSONC, a shape without profiles.defaults, and a settings
// file that is not there are all refused with a reason, and nothing is written.
func TestWindowsTerminalReadOfferRefusesWhatItCannotEditSafely(t *testing.T) {
	tests := []struct {
		name     string
		settings string
		want     string
	}{
		{
			name:     "unparseable JSONC",
			settings: `{ "profiles": { "defaults": { } `,
			want:     "JSONC",
		},
		{
			name:     "profiles is not an object",
			settings: `{ "profiles": [], "schemes": [] }`,
			want:     "profiles",
		},
		{
			name:     "defaults is missing",
			settings: `{ "profiles": { "list": [] }, "schemes": [] }`,
			want:     "defaults",
		},
		{
			name:     "schemes is not an array",
			settings: `{ "profiles": { "defaults": {} }, "schemes": {} }`,
			want:     "schemes",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			profileDir, settingsPath := newWindowsTerminalProfile(t, tt.settings)
			offer := readWindowsTerminalOffer(profileDir, testWindowsTerminalThemeDefinition(), windowsTerminalFontFace)
			if offer.Available {
				t.Fatalf("the offer must be refused for %q", tt.name)
			}
			if !strings.Contains(offer.Reason, tt.want) {
				t.Errorf("the refusal should name %q, got: %s", tt.want, offer.Reason)
			}

			if _, wrote, err := applyWindowsTerminalOffer(offer, true, wslStepID); err != nil || wrote {
				t.Errorf("a refused offer must write nothing, wrote=%v err=%v", wrote, err)
			}
			if after, err := os.ReadFile(settingsPath); err != nil {
				t.Fatal(err)
			} else if string(after) != tt.settings {
				t.Error("a refused offer must leave the file untouched")
			}
		})
	}
}

// TestWindowsTerminalReadOfferRefusesWhenTheFileIsAbsent pins that a package
// directory with no settings.json is a named refusal, not an empty file.
func TestWindowsTerminalReadOfferRefusesWhenTheFileIsAbsent(t *testing.T) {
	profileDir, _ := newWindowsTerminalProfile(t, "")
	offer := readWindowsTerminalOffer(profileDir, testWindowsTerminalThemeDefinition(), windowsTerminalFontFace)
	if offer.Available {
		t.Fatal("an absent settings.json must not be offered")
	}
	if !strings.Contains(offer.Reason, "no Windows Terminal settings.json") {
		t.Errorf("the refusal should say the file is missing, got: %s", offer.Reason)
	}
}

// TestWindowsTerminalSettingsPathInjectsTheMount pins that discovery reads the
// mount it is handed: the Store and Preview package names are both found, an
// ambiguous pair is refused with both named, and no real /mnt/c is ever read.
func TestWindowsTerminalSettingsPathInjectsTheMount(t *testing.T) {
	write := func(t *testing.T, profileDir string, packages ...string) []string {
		t.Helper()
		var paths []string
		for _, pkg := range packages {
			path := filepath.Join(profileDir, "AppData", "Local", "Packages", pkg, "LocalState", "settings.json")
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
				t.Fatal(err)
			}
			paths = append(paths, path)
		}
		return paths
	}

	t.Run("the Store build is found", func(t *testing.T) {
		profileDir := t.TempDir()
		want := write(t, profileDir, "Microsoft.WindowsTerminal_8wekyb3d8bbwe")[0]
		got, err := windowsTerminalSettingsPath(profileDir)
		if err != nil || got != want {
			t.Fatalf("windowsTerminalSettingsPath = %q, %v; want %q", got, err, want)
		}
	})

	t.Run("the Preview build is found", func(t *testing.T) {
		profileDir := t.TempDir()
		want := write(t, profileDir, "Microsoft.WindowsTerminalPreview_8wekyb3d8bbwe")[0]
		got, err := windowsTerminalSettingsPath(profileDir)
		if err != nil || got != want {
			t.Fatalf("windowsTerminalSettingsPath = %q, %v; want %q", got, err, want)
		}
	})

	t.Run("an ambiguous pair is refused with both named", func(t *testing.T) {
		profileDir := t.TempDir()
		write(t, profileDir, "Microsoft.WindowsTerminal_8wekyb3d8bbwe", "Microsoft.WindowsTerminalPreview_8wekyb3d8bbwe")
		_, err := windowsTerminalSettingsPath(profileDir)
		if err == nil {
			t.Fatal("two packages must be refused rather than guessed")
		}
		for _, want := range []string{"Microsoft.WindowsTerminal_", "Preview"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("the refusal should name %q: %v", want, err)
			}
		}
	})

	t.Run("an empty mount is refused", func(t *testing.T) {
		if _, err := windowsTerminalSettingsPath(t.TempDir()); err == nil {
			t.Fatal("an empty mount must not resolve a settings file")
		}
	})
}

// TestWindowsTerminalSchemeCoversEveryField pins the mapping from the canonical
// palette roles to Windows Terminal's own scheme keys, and that the marshalled
// scheme carries every key the single list names.
func TestWindowsTerminalSchemeCoversEveryField(t *testing.T) {
	def := testWindowsTerminalThemeDefinition()
	scheme, err := windowsTerminalSchemeFor(def)
	if err != nil {
		t.Fatalf("windowsTerminalSchemeFor failed: %v", err)
	}
	if scheme.Name != windowsTerminalSchemeName {
		t.Errorf("scheme name = %q, want %q", scheme.Name, windowsTerminalSchemeName)
	}

	values := map[string]string{}
	for _, c := range scheme.Colors {
		values[c.Key] = c.Value
	}
	for _, tc := range []struct{ key, role string }{
		{"background", "base"},
		{"foreground", "text"},
		{"cursorColor", "cursor"},
		{"selectionBackground", "selection"},
		{"red", "red"},
		{"purple", "magenta"},
		{"brightPurple", "bright_magenta"},
		{"brightWhite", "bright_white"},
	} {
		if got := values[tc.key]; got != def.Palette[tc.role] {
			t.Errorf("scheme %s = %q, want the %q role %q", tc.key, got, tc.role, def.Palette[tc.role])
		}
	}

	// The marshalled object must hold every key the field list names, so a key
	// added to the list without a struct field is caught here.
	data, err := json.Marshal(windowsTerminalSchemeObject(scheme))
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]string
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, c := range scheme.Colors {
		if decoded[c.Key] != c.Value {
			t.Errorf("the marshalled scheme is missing %s = %q", c.Key, c.Value)
		}
	}
}

// TestWindowsTerminalSchemeRefusesAMissingRole pins that an incomplete theme is
// refused rather than written with an invented colour.
func TestWindowsTerminalSchemeRefusesAMissingRole(t *testing.T) {
	def := testWindowsTerminalThemeDefinition()
	delete(def.Palette, "magenta")
	if _, err := windowsTerminalSchemeFor(def); err == nil {
		t.Fatal("a theme missing a role must be refused")
	} else if !strings.Contains(err.Error(), "magenta") {
		t.Errorf("the refusal should name the missing role: %v", err)
	}
}

// mountThemeDefinition copies one of the repository's own theme definitions into
// the fixture repository, so a test can apply a theme that is not the default
// with the real palette rather than a hand-written stand-in.
func mountThemeDefinition(t *testing.T, repoDir, id string) {
	t.Helper()

	body, err := os.ReadFile(filepath.Join(repoRoot(t), themesDirName, id+".toml"))
	if err != nil {
		t.Fatalf("reading the %s theme: %v", id, err)
	}
	path := filepath.Join(repoDir, themesDirName, id+".toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestWindowsTerminalSchemeFollowsTheAppliedTheme is the regression guard for the
// scheme's palette. The scheme carries the colours of the theme this machine has
// applied, read from the installer's own record, and never the default theme's:
// a machine whose theme switch applied another theme would otherwise have the
// Windows side painted from a palette no tool on the Linux side uses. The scheme
// keeps its one name across themes, because Windows Terminal resolves it by the
// name profiles.defaults.colorScheme holds.
func TestWindowsTerminalSchemeFollowsTheAppliedTheme(t *testing.T) {
	const appliedID = "nocturne"

	repoDir, _, settingsPath := newWindowsTerminalWSLLayout(t)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	mountThemeDefinition(t, repoDir, appliedID)

	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("loading the repository's themes: %v", err)
	}
	applied, ok := themeByID(defs, appliedID)
	if !ok {
		t.Fatalf("the %s theme is not in the repository", appliedID)
	}
	fallback, ok := themeByID(defs, defaultThemeID)
	if !ok {
		t.Fatalf("the %s theme is not in the repository", defaultThemeID)
	}
	// Without two palettes that actually differ, the guard would pass on a
	// function that reads the wrong theme.
	if applied.Palette["base"] == fallback.Palette["base"] {
		t.Fatalf("%s and %s share a background, so this guard cannot tell them apart", appliedID, defaultThemeID)
	}

	if err := writeDotfilesThemeRecord(&dotfilesThemeRecord{Theme: appliedID}); err != nil {
		t.Fatalf("recording the applied theme: %v", err)
	}

	resolved, err := windowsTerminalThemeDefinition(repoDir)
	if err != nil {
		t.Fatalf("resolving the scheme's palette: %v", err)
	}
	if resolved.ID != appliedID {
		t.Errorf("the scheme's palette came from %q, want the applied %q", resolved.ID, appliedID)
	}

	stubWindowsFontQueryStateful(t, map[string]bool{})
	setNonInteractiveForTest(t, true)
	m := wslModel(repoDir, true)
	if err := executeStep("wslconfig", &m); err != nil {
		t.Fatalf("the --non-interactive step failed: %v", err)
	}

	after, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	root, err := parseJSONC(after)
	if err != nil {
		t.Fatalf("the written settings are not valid JSONC: %v\n%s", err, after)
	}
	schemes := root.findMember("schemes")
	if schemes == nil || schemes.value.kind != jsoncArray {
		t.Fatal("schemes is not an array after the write")
	}
	owned := findSchemeNamed(t, schemes.value, windowsTerminalSchemeName)
	if owned == nil {
		t.Fatalf("the write left no %q scheme", windowsTerminalSchemeName)
	}

	// Every role the palette defines, not one of them: a scheme built from the
	// right theme for one key and the wrong one for the rest is the bug.
	for _, field := range windowsTerminalSchemeFields {
		want := applied.Palette[field.role]
		member := owned.findMember(field.key)
		if member == nil || member.value.text != want {
			t.Errorf("the scheme's %s = %v, want %s's %q (the default theme's is %q)",
				field.key, member, appliedID, want, fallback.Palette[field.role])
		}
	}
}

// TestWindowsTerminalSchemeKeepsTheDefaultThemeWithoutARecord pins the fallback
// the guard above is measured against. A machine that has applied nothing, and
// one whose record names a theme this repository no longer defines, both get the
// default theme: the first because it is what a fresh install applies, the
// second because failing the Windows side over a stale record would cost the
// font as well and the default is the only palette the repository can still
// prove.
func TestWindowsTerminalSchemeKeepsTheDefaultThemeWithoutARecord(t *testing.T) {
	t.Run("no record", func(t *testing.T) {
		repoDir, _, _ := newWindowsTerminalWSLLayout(t)
		t.Setenv("XDG_STATE_HOME", t.TempDir())

		resolved, err := windowsTerminalThemeDefinition(repoDir)
		if err != nil {
			t.Fatalf("resolving the scheme's palette: %v", err)
		}
		if resolved.ID != defaultThemeID {
			t.Errorf("a machine with no record resolved %q, want %q", resolved.ID, defaultThemeID)
		}
	})

	t.Run("a record naming a theme the repository does not hold", func(t *testing.T) {
		repoDir, _, _ := newWindowsTerminalWSLLayout(t)
		t.Setenv("XDG_STATE_HOME", t.TempDir())

		if err := writeDotfilesThemeRecord(&dotfilesThemeRecord{Theme: "a-theme-that-is-gone"}); err != nil {
			t.Fatalf("recording the applied theme: %v", err)
		}

		resolved, err := windowsTerminalThemeDefinition(repoDir)
		if err != nil {
			t.Fatalf("a stale record must not fail the Windows side: %v", err)
		}
		if resolved.ID != defaultThemeID {
			t.Errorf("a stale record resolved %q, want the fallback %q", resolved.ID, defaultThemeID)
		}
	})
}

// TestWindowsTerminalSettingsListNamesWhatWrites ties the single list the panel
// reads to the values the writer uses, so the offer cannot name one thing and
// the writer set another.
func TestWindowsTerminalSettingsListNamesWhatWrites(t *testing.T) {
	if len(windowsTerminalSettings) == 0 {
		t.Fatal("the owned-settings list is empty")
	}
	var labels []string
	for _, s := range windowsTerminalSettings {
		labels = append(labels, s.Label)
	}
	joined := strings.Join(labels, " and ")
	for _, want := range []string{windowsTerminalFontFace, windowsTerminalSchemeName} {
		if !strings.Contains(joined, want) {
			t.Errorf("the owned-settings list does not name %q: %v", want, labels)
		}
	}
}

// TestWindowsTerminalOfferBodyIsAChangeTable pins the offer's body as a table of
// what changes. It used to be a paragraph: three sentences built by
// describeWindowsTerminalOffer, the values already in the file and the font-store
// notice appended by wslTerminalOfferLines, and a closing line restating the
// screen's own hint -- seven rows of prose for one yes/no. Every fact survives;
// what the table replaces is the sentence form, and this guard is what holds the
// shape, so the prose cannot come back one clause at a time.
func TestWindowsTerminalOfferBodyIsAChangeTable(t *testing.T) {
	profileDir, settingsPath := newWindowsTerminalProfile(t, testWindowsTerminalSettings)
	installWSLTerminalSettings(t, settingsPath, testWindowsTerminalSettingsWithOtherFace)
	offer := readWindowsTerminalOffer(profileDir, testWindowsTerminalThemeDefinition(), windowsTerminalFontFace)
	if !offer.Available {
		t.Fatalf("the offer must be available: %s", offer.Reason)
	}

	lines := wslTerminalOfferLines(offer)
	body := strings.Join(lines, "\n")

	// Every fact the sentences carried is still on the screen: the file, both
	// settings, the value each holds today, the backup, the rest of the file, and
	// the one effect that does not live in settings.json.
	for _, want := range []string{
		settingsPath,
		"settings.json",
		"font.face",
		"colorScheme",
		"CascadiaMonoNF",
		windowsTerminalFontFace,
		"OneHalfDark",
		windowsTerminalSchemeName,
		">",
		".bak-dotfiles-",
		"every other key and comment",
		"HKCU",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the offer's body does not carry %q:\n%s", want, body)
		}
	}

	// The value the file holds is opposed to the one this installer would write, on
	// the row of the setting it belongs to. That opposition is what replaced the
	// sentences "It already names the font face ... there".
	if !strings.Contains(body, "CascadiaMonoNF > "+windowsTerminalFontFace) {
		t.Errorf("the font row does not oppose the value in the file to the value this installer would write:\n%s", body)
	}
	if !strings.Contains(body, "OneHalfDark > "+windowsTerminalSchemeName) {
		t.Errorf("the colour row does not oppose the value in the file to the value this installer would write:\n%s", body)
	}

	// The closing sentence spent a row repeating the installing screen's own hint.
	if strings.Contains(body, "Press y") {
		t.Errorf("the offer still spends a row on the hint the screen already shows:\n%s", body)
	}

	// No row is a sentence: not one of them ends in the full stop every one of the
	// seven sentences it replaces ended in.
	for _, line := range lines {
		if strings.HasSuffix(strings.TrimSpace(line), ".") {
			t.Errorf("the offer's row %q is still a sentence, and the table is the form that replaces it:\n%s", line, body)
		}
	}

	// The shared description is the table too, so the notice --non-interactive
	// prints stops saying what the screen stopped saying.
	for _, line := range describeWindowsTerminalOffer(offer) {
		if strings.HasSuffix(strings.TrimSpace(line), ".") || strings.Contains(line, "is reachable at") {
			t.Errorf("the shared description is still the sentence the table replaces: %q", line)
		}
	}
}

// TestWindowsTerminalOfferRowSaysUnsetRatherThanNothing pins the row of a setting
// the file does not carry. The prose named a value only when there was one, so a
// person reading the screen could not tell an absent value from a screen that had
// run out of rows. The table always answers, and the answer is a fact rather than a
// guess: an available offer proves the file was read and parsed.
func TestWindowsTerminalOfferRowSaysUnsetRatherThanNothing(t *testing.T) {
	profileDir, settingsPath := newWindowsTerminalProfile(t, testWindowsTerminalSettings)
	offer := readWindowsTerminalOffer(profileDir, testWindowsTerminalThemeDefinition(), windowsTerminalFontFace)
	if !offer.Available {
		t.Fatalf("the offer must be available: %s", offer.Reason)
	}
	if _, ok := windowsTerminalCurrentFontFace(offer.Original); ok {
		t.Fatalf("this fixture names a font face, so it cannot pin the absent case: %s", settingsPath)
	}

	body := strings.Join(wslTerminalOfferLines(offer), "\n")
	if !strings.Contains(body, "(unset) > "+windowsTerminalFontFace) {
		t.Errorf("a file that names no font face does not say so, so the screen is silent about the value the choice would replace:\n%s", body)
	}
}

// TestWindowsTerminalOfferFitsTheFrameItIsDrawnOn is the width guard the offer
// never had. The panel is drawn from the rows the installing body has left after
// the live meters, so at the tightest measured terminal it has room for eight
// content rows -- and the paragraph needed more: at 60x20 the last sentences,
// including the value the choice replaces, were cut off with a "…". Nothing
// rendered this panel at any size, so no test could see it, and the panel is the
// screen that stands between the run and a write to settings.json.
func TestWindowsTerminalOfferFitsTheFrameItIsDrawnOn(t *testing.T) {
	repoDir, _, settingsPath := newWindowsTerminalWSLLayout(t)
	stubWindowsFontQueryStateful(t, map[string]bool{})
	installWSLTerminalSettings(t, settingsPath, testWindowsTerminalSettingsWithOtherFace)

	for _, size := range measuredTerminalSizes {
		size := size
		t.Run(size.name, func(t *testing.T) {
			m := driveWSLTerminalOfferToTheScreen(t, mountWSLTerminalStep(t, repoDir))
			m.Width, m.Height = size.width, size.height
			view := m.View()
			assertScreenFitsTerminal(t, "installing screen with the Windows Terminal offer pending", size.width, size.height, view)

			// Fitting inside the frame is not the whole rule: the panel is where the
			// choice is made, so every fact the choice turns on has to be readable in it,
			// not merely narrower than the terminal. Two views are read, because a long
			// settings path is wrapped rather than clipped: the file name can sit either
			// side of a wrap, and the newline, border and padding the panel inserts are
			// not the user's text. Facts that carry a space are read from the first view,
			// where the spaces still stand.
			noNewlines := strings.ReplaceAll(view, "\n", "")
			wrapped := strings.NewReplacer("\n", "", "│", "", " ", "").Replace(view)
			for _, want := range []string{
				"settings.json",
				"font.face",
				"colorScheme",
				"CascadiaMonoNF",
				"OneHalfDark",
				windowsTerminalSchemeName,
				".bak-dotfiles-",
				"HKCU",
			} {
				if !strings.Contains(wrapped, want) {
					t.Errorf("at %s the offer does not show %q, so the user answers a question whose terms fell out of the panel:\n%s", size.name, want, view)
				}
			}
			for _, want := range []string{
				windowsTerminalFontFace,
				"configure Windows Terminal or leave it unchanged",
			} {
				if !strings.Contains(noNewlines, want) {
					t.Errorf("at %s the offer does not show %q, so the user answers a question whose terms fell out of the panel:\n%s", size.name, want, view)
				}
			}
		})
	}
}

// TestWindowsTerminalOfferDescribesWhatWouldChange pins the offer's body: it
// names the file, the font face and the scheme before the user chooses.
func TestWindowsTerminalOfferDescribesWhatWouldChange(t *testing.T) {
	profileDir, settingsPath := newWindowsTerminalProfile(t, testWindowsTerminalSettings)
	offer := readWindowsTerminalOffer(profileDir, testWindowsTerminalThemeDefinition(), windowsTerminalFontFace)
	if !offer.Available {
		t.Fatalf("the offer must be available: %s", offer.Reason)
	}

	description := strings.Join(describeWindowsTerminalOffer(offer), "\n")
	for _, want := range []string{settingsPath, windowsTerminalFontFace, windowsTerminalSchemeName} {
		if !strings.Contains(description, want) {
			t.Errorf("the offer should name %q before a choice is made:\n%s", want, description)
		}
	}
}

// TestStepInstallWSLConfigNeverTouchesWindowsTerminalWithoutWSL pins that the
// Windows file is only a WSL concern: on a non-WSL host the step is a no-op and
// a reachable-looking settings.json is left alone.
func TestStepInstallWSLConfigNeverTouchesWindowsTerminalWithoutWSL(t *testing.T) {
	profileDir, settingsPath := newWindowsTerminalProfile(t, testWindowsTerminalSettings)
	t.Setenv(envWSLWindowsHome, profileDir)

	m := wslModel("", false)
	if err := stepInstallWSLConfig(&m); err != nil {
		t.Fatalf("non-WSL host must be a no-op: %v", err)
	}
	if after, err := os.ReadFile(settingsPath); err != nil {
		t.Fatal(err)
	} else if string(after) != testWindowsTerminalSettings {
		t.Error("a non-WSL host must not touch Windows Terminal's settings")
	}
}

// TestWindowsTerminalRendererLinesNameTheLandingSites pins the truth the WSL
// screen states: where the font and the palette landed and that Windows Terminal
// is the renderer.
func TestWindowsTerminalRendererLinesNameTheLandingSites(t *testing.T) {
	joined := strings.Join(windowsTerminalRendererLines(), "\n")
	for _, want := range []string{
		".local/share/fonts",
		"palette",
		"Windows Terminal",
		windowsTerminalFontFace,
		windowsTerminalSchemeName,
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("the renderer notice should name %q:\n%s", want, joined)
		}
	}
	// Windows Terminal caches its font list at start, so the step has to tell the
	// user to reopen it or the icons stay boxes until they happen to restart it.
	if !strings.Contains(joined, "reopen") {
		t.Errorf("the renderer notice should say Windows Terminal has to be reopened:\n%s", joined)
	}
}

// ============================================================================
// THE WINDOWS-SIDE WRITE: REACHABILITY, THE VERIFICATION GATE, AND THE REFUSAL
// ============================================================================

// setNonInteractiveForTest drives the step through the same switch the
// --non-interactive flag flips, restoring the previous value when the test ends.
func setNonInteractiveForTest(t *testing.T, enabled bool) {
	t.Helper()
	previous := nonInteractiveMode
	SetNonInteractiveMode(enabled)
	t.Cleanup(func() { SetNonInteractiveMode(previous) })
}

// stubWindowsFontQuery replaces the Windows command boundary for one test. It
// answers the verification script with answer and returns nothing for the
// registration script, recording every script so a test can assert both ran and
// in which order. No test ever spawns powershell.exe.
func stubWindowsFontQuery(t *testing.T, answer string) *[]string {
	t.Helper()

	scripts := &[]string{}
	previous := windowsFontQuery
	windowsFontQuery = func(_ context.Context, script string) ([]byte, error) {
		*scripts = append(*scripts, script)
		if strings.Contains(script, "InstalledFontCollection") {
			return []byte(answer + "\n"), nil
		}
		return nil, nil
	}
	t.Cleanup(func() { windowsFontQuery = previous })
	return scripts
}

// stubWindowsFontQueryStateful replaces the Windows command boundary with a
// fake font store. installed names the families the store resolves before any
// registration; a family that is not installed starts resolving once a
// registration query has run, because registration writes the per-user font key
// the store reads. scripts records every query so a test can assert what ran. No
// test ever spawns powershell.exe.
func stubWindowsFontQueryStateful(t *testing.T, installed map[string]bool) *[]string {
	t.Helper()

	scripts := &[]string{}
	registered := false
	previous := windowsFontQuery
	windowsFontQuery = func(_ context.Context, script string) ([]byte, error) {
		*scripts = append(*scripts, script)
		if strings.Contains(script, "New-ItemProperty") {
			registered = true
			return nil, nil
		}
		for face, present := range installed {
			if strings.Contains(script, psSingleQuote(face)) {
				if present {
					return []byte(windowsFontPresent + "\n"), nil
				}
				return []byte(windowsFontAbsent + "\n"), nil
			}
		}
		// A family the map does not hold resolves only after the installer has
		// registered the archive family into the store.
		if registered && strings.Contains(script, psSingleQuote(windowsTerminalFontFace)) {
			return []byte(windowsFontPresent + "\n"), nil
		}
		// A query that names a family the map does not hold is absent.
		return []byte(windowsFontAbsent + "\n"), nil
	}
	t.Cleanup(func() { windowsFontQuery = previous })
	return scripts
}

// newWindowsTerminalWSLLayout builds the whole WSL fixture: a checkout that
// ships the .wslconfig template, wsl.conf and the dotfiles theme; a Windows
// profile whose settings.json is reachable; a Linux font directory holding the
// faces the font step would have extracted; and the pins that keep every lookup
// and query in the temporary tree.
func newWindowsTerminalWSLLayout(t *testing.T) (repoDir, profileDir, settingsPath string) {
	t.Helper()

	repoDir, _, _ = newWSLLayout(t)
	t.Setenv(envBinfmtDir, t.TempDir())
	t.Setenv("DOTFILES_DRY_RUN", "")

	// The renderer's scheme is built from the real dotfiles theme, so the fixture
	// exercises the same palette the installer applies to every other tool.
	theme, err := os.ReadFile(filepath.Join(repoRoot(t), themesDirName, defaultThemeID+".toml"))
	if err != nil {
		t.Fatalf("reading the %s theme: %v", defaultThemeID, err)
	}
	themePath := filepath.Join(repoDir, themesDirName, defaultThemeID+".toml")
	if err := os.MkdirAll(filepath.Dir(themePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(themePath, theme, 0o644); err != nil {
		t.Fatal(err)
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	fontDir := filepath.Join(home, ".local", "share", "fonts")
	if err := os.MkdirAll(fontDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, face := range windowsFontFaces {
		if err := os.WriteFile(filepath.Join(fontDir, face.File), []byte("font-bytes:"+face.File), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	profileDir, settingsPath = newWindowsTerminalProfile(t, testWindowsTerminalSettings)
	t.Setenv(envWSLWindowsHome, profileDir)
	return repoDir, profileDir, settingsPath
}

// TestWSLConfigStepConfiguresWindowsTerminalUnderNonInteractive is the
// reachability guard for the write path. It drives the very executor
// RunNonInteractive uses (executeStep, through stepExecutors), with the mode
// --non-interactive sets, and fails unless the font lands in the Windows font
// store, the family is verified, and settings.json is written with a backup. It
// is the guard the dead offer functions never had: a test that called them
// directly hid the fact that no flag and no screen could reach them.
func TestWSLConfigStepConfiguresWindowsTerminalUnderNonInteractive(t *testing.T) {
	repoDir, profileDir, settingsPath := newWindowsTerminalWSLLayout(t)
	scripts := stubWindowsFontQueryStateful(t, map[string]bool{})
	setNonInteractiveForTest(t, true)

	m := wslModel(repoDir, true)
	if err := executeStep("wslconfig", &m); err != nil {
		t.Fatalf("the --non-interactive step failed: %v", err)
	}

	store := filepath.Join(profileDir, filepath.FromSlash(windowsFontStoreRelPath))
	for _, face := range windowsFontFaces {
		if _, err := os.Stat(filepath.Join(store, face.File)); err != nil {
			t.Errorf("the face %s was not copied into the Windows font store: %v", face.File, err)
		}
	}

	registered, verified := false, false
	for _, script := range *scripts {
		if strings.Contains(script, "New-ItemProperty") {
			registered = true
		}
		if strings.Contains(script, "InstalledFontCollection") && strings.Contains(script, windowsTerminalFontFace) {
			verified = true
		}
	}
	if !registered {
		t.Errorf("the family must be registered under HKCU before the face is written; queries were %v", *scripts)
	}
	if !verified {
		t.Errorf("the family %q must be verified in the font store before the face is written; queries were %v", windowsTerminalFontFace, *scripts)
	}

	after, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	root, err := parseJSONC(after)
	if err != nil {
		t.Fatalf("the written settings are not valid JSONC: %v\n%s", err, after)
	}
	defaults := root.findMember("profiles").value.findMember("defaults").value
	if face := defaults.findMember("font"); face == nil || face.value.findMember("face") == nil || face.value.findMember("face").value.text != windowsTerminalFontFace {
		t.Errorf("profiles.defaults.font.face was not written with %q", windowsTerminalFontFace)
	}
	if cs := defaults.findMember("colorScheme"); cs == nil || cs.value.text != windowsTerminalSchemeName {
		t.Errorf("profiles.defaults.colorScheme was not written with %q", windowsTerminalSchemeName)
	}
	if backups, _ := filepath.Glob(settingsPath + ".bak-dotfiles-*"); len(backups) != 1 {
		t.Errorf("the write must leave exactly one backup, got %v", backups)
	}
}

// TestWSLConfigStepPrefersTheMachinesOwnFace is the "name what resolves" guard
// for the preference rule. When the Windows font store already resolves the
// machine's own Meslo build, the writer names it and copies nothing; a family
// hand-written here rather than resolved would be the silent fallback the rule
// exists to prevent.
func TestWSLConfigStepPrefersTheMachinesOwnFace(t *testing.T) {
	repoDir, profileDir, settingsPath := newWindowsTerminalWSLLayout(t)
	stubWindowsFontQueryStateful(t, map[string]bool{windowsTerminalFontFacePreferred: true})
	setNonInteractiveForTest(t, true)

	var lines []string
	restore := wslLog
	wslLog = func(_, line string) { lines = append(lines, line) }
	t.Cleanup(func() { wslLog = restore })

	m := wslModel(repoDir, true)
	if err := executeStep("wslconfig", &m); err != nil {
		t.Fatalf("the --non-interactive step failed: %v", err)
	}

	store := filepath.Join(profileDir, filepath.FromSlash(windowsFontStoreRelPath))
	for _, face := range windowsFontFaces {
		if _, err := os.Stat(filepath.Join(store, face.File)); !os.IsNotExist(err) {
			t.Errorf("the machine's own face resolved, so %s must not be copied into the Windows font store: %v", face.File, err)
		}
	}

	after, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	root, err := parseJSONC(after)
	if err != nil {
		t.Fatalf("the written settings are not valid JSONC: %v\n%s", err, after)
	}
	defaults := root.findMember("profiles").value.findMember("defaults").value
	if face := defaults.findMember("font").value.findMember("face"); face == nil || face.value.text != windowsTerminalFontFacePreferred {
		t.Errorf("profiles.defaults.font.face was not written with the machine's own family %q", windowsTerminalFontFacePreferred)
	}

	// The notice names the face that was written, not the fallback constant: a
	// description naming a different family than the settings file is the same
	// silent mismatch in prose that the resolution rule closes in the file.
	notice := strings.Join(lines, "\n")
	if !strings.Contains(notice, windowsTerminalFontFacePreferred) {
		t.Errorf("the success notice should name the resolved family %q, got:\n%s", windowsTerminalFontFacePreferred, notice)
	}
}

// TestWSLConfigStepRefusesTheFaceWhenTheFamilyDoesNotVerify is the constraint-1
// guard: when the Windows font store answers that the family is absent, or gives
// no answer at all, the face must not be written. Without the gate the face
// landed in settings.json even though Windows could not resolve it, which is the
// silent fallback this defect is about.
func TestWSLConfigStepRefusesTheFaceWhenTheFamilyDoesNotVerify(t *testing.T) {
	for _, tc := range []struct {
		name   string
		answer string
		want   string
	}{
		{"the family is absent", windowsFontAbsent, "not present"},
		{"the store gives no answer", "something else", "no answer"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repoDir, _, settingsPath := newWindowsTerminalWSLLayout(t)
			stubWindowsFontQuery(t, tc.answer)
			setNonInteractiveForTest(t, true)

			var lines []string
			restore := wslLog
			wslLog = func(_, line string) { lines = append(lines, line) }
			t.Cleanup(func() { wslLog = restore })

			m := wslModel(repoDir, true)
			if err := stepInstallWSLConfig(&m); err != nil {
				t.Fatalf("step failed: %v", err)
			}

			if after, err := os.ReadFile(settingsPath); err != nil {
				t.Fatal(err)
			} else if string(after) != testWindowsTerminalSettings {
				t.Errorf("the face must not be written when the family does not verify:\n%s", after)
			}
			if report := strings.Join(lines, "\n"); !strings.Contains(report, tc.want) {
				t.Errorf("the refusal should say %q, got:\n%s", tc.want, report)
			}
		})
	}
}

// TestWSLConfigStepRefusesWindowsTerminalWithoutTheExplicitYes is the interactive
// half of the maintainer's decision: outside --non-interactive the step names the
// offer and writes nothing to the Windows side, because docs/tui-installer.md
// promises a write only after an explicit yes.
func TestWSLConfigStepRefusesWindowsTerminalWithoutTheExplicitYes(t *testing.T) {
	repoDir, profileDir, settingsPath := newWindowsTerminalWSLLayout(t)
	stubWindowsFontQuery(t, windowsFontPresent)
	setNonInteractiveForTest(t, false)

	var lines []string
	restore := wslLog
	wslLog = func(_, line string) { lines = append(lines, line) }
	t.Cleanup(func() { wslLog = restore })

	m := wslModel(repoDir, true)
	if err := stepInstallWSLConfig(&m); err != nil {
		t.Fatalf("step failed: %v", err)
	}

	if after, err := os.ReadFile(settingsPath); err != nil {
		t.Fatal(err)
	} else if string(after) != testWindowsTerminalSettings {
		t.Error("without an explicit yes the settings file must be unchanged")
	}
	if _, err := os.Stat(filepath.Join(profileDir, filepath.FromSlash(windowsFontStoreRelPath))); !os.IsNotExist(err) {
		t.Error("without an explicit yes no font may be copied into the Windows store")
	}

	report := strings.Join(lines, "\n")
	if !strings.Contains(report, settingsPath) {
		t.Errorf("the offer should name the settings file before a choice:\n%s", report)
	}
	if !strings.Contains(report, "explicit yes") {
		t.Errorf("the refusal should say the write needs an explicit yes:\n%s", report)
	}
}

// testWindowsTerminalSettingsWithOtherFace is the settings.json of a machine
// another tool already configured: a font face of its own and a colour scheme of
// its own sit beside the keys this installer owns. The offer has to name them,
// because an offer that only says what it would set is how a user loses a value
// they chose.
const testWindowsTerminalSettingsWithOtherFace = `{
    // Windows Terminal settings
    "$schema": "https://example.invalid/schema.json",
    "profiles": {
        "defaults": {
            "font": { "face": "CascadiaMonoNF" },
            "colorScheme": "OneHalfDark",
            "padding": "8"
        },
        "list": [
            { "name": "Debian", "guid": "{00000000-0000-0000-0000-000000000000}" }
        ]
    },
    "schemes": [
        { "name": "Campbell", "background": "#0C0C0C", "foreground": "#CCCCCC" }
    ],
    "theme": "dark"
}
`

// installWSLTerminalSettings overwrites the fixture's settings.json with a
// document of the caller's choosing, so a test can pin what the offer reads
// outside the shape the shared fixture ships.
func installWSLTerminalSettings(t *testing.T, settingsPath, content string) {
	t.Helper()
	if err := os.WriteFile(settingsPath, []byte(content), 0o644); err != nil {
		t.Fatalf("writing the fixture settings: %v", err)
	}
}

// mountWSLTerminalStep parks a model on the installing screen with the
// interactive WSL step as the one the run is on. It is the state the TUI is in
// when the step loop arrives at the offer.
func mountWSLTerminalStep(t *testing.T, repoDir string) Model {
	t.Helper()
	m := wslModel(repoDir, true)
	m.Screen = ScreenInstalling
	m.Width, m.Height = trainerFrameWidth, trainerFrameHeight
	m.CurrentStep = 0
	m.Steps = []InstallStep{{ID: wslStepID, Name: "Configure WSL", Interactive: true}}
	return m
}

// driveWSLTerminalOfferToTheScreen drives the step loop the way the bubbletea
// runtime does: installStartMsg makes Update return runNextStep's command, that
// command returns the offer message, and the message goes back into Update. It is
// the reachability assertion the dead offer functions never had -- the offer has
// to arrive through the model and its Update loop, so a screen nobody wired would
// fail here instead of passing a test that called the writer directly. Running
// the returned command is what proves the offer is produced by the TUI's own
// dispatch rather than by a test helper.
func driveWSLTerminalOfferToTheScreen(t *testing.T, m Model) Model {
	t.Helper()

	result, cmd := m.Update(installStartMsg{})
	if cmd == nil {
		t.Fatal("the step loop returned no command, so the WSL step would never start")
	}
	m = result.(Model)

	msg := cmd()
	result, _ = m.Update(msg)
	m = result.(Model)

	if m.WSLTerminalOffer == nil {
		t.Fatalf("the interactive WSL step reached %T and produced no on-screen offer; with no offer a TUI user can never say yes, which is exactly the gap this guard exists to close", msg)
	}
	return m
}

// TestInteractiveWSLStepShowsTheOfferOnTheInstallingScreen is the reachability
// guard for the interactive half of the maintainer's decision on #261: the TUI's
// own step loop must produce the Windows Terminal offer and the installing screen
// must draw it, naming the file, the settings and the value already there. Before
// this wiring the offer functions were reached only by --non-interactive (or, the
// release before, only by a test), so a user who installed through the TUI never
// saw the offer at all. The screen states those three things as rows now -- the
// file, then one row per setting with the value the file holds beside the value a
// write would set -- so this reads the rows the screen draws rather than the
// sentences that used to carry them.
func TestInteractiveWSLStepShowsTheOfferOnTheInstallingScreen(t *testing.T) {
	repoDir, _, settingsPath := newWindowsTerminalWSLLayout(t)
	stubWindowsFontQueryStateful(t, map[string]bool{})
	installWSLTerminalSettings(t, settingsPath, testWindowsTerminalSettingsWithOtherFace)

	m := driveWSLTerminalOfferToTheScreen(t, mountWSLTerminalStep(t, repoDir))

	// The screen shows the offer, and the screen is what a person reads: the
	// assertion is on the rendered frame, not only on the model field. The rows are
	// read as a person reads them, with the panel's own newline, border and padding
	// taken out: a settings path too long for the panel is wrapped rather than
	// clipped, so the file name can sit either side of a wrap and still be on the
	// screen. Facts carrying a space are read from the view where the spaces stand.
	view := strings.ReplaceAll(m.View(), "\n", "")
	wrapped := strings.NewReplacer("\n", "", "│", "", " ", "").Replace(m.View())
	for _, want := range []string{
		"settings.json",
		"font.face",
		"colorScheme",
		"CascadiaMonoNF",
		"OneHalfDark",
	} {
		if !strings.Contains(wrapped, want) {
			t.Errorf("the installing screen does not show %q, so a write could happen without the user reading what it replaces:\n%s", want, m.View())
		}
	}
	if hint := "configure Windows Terminal or leave it unchanged"; !strings.Contains(view, hint) {
		t.Errorf("the installing screen does not show the hint that answers the offer (%q):\n%s", hint, m.View())
	}
	if !strings.Contains(strings.Join(m.WSLTerminalOfferLines, "\n"), settingsPath) {
		t.Errorf("the offer must name the settings.json it would change, got:\n%s", strings.Join(m.WSLTerminalOfferLines, "\n"))
	}

	// Reading the offer changed nothing on disk: the offer is read-only and the
	// write waits for the answer.
	if got, err := os.ReadFile(settingsPath); err != nil {
		t.Fatal(err)
	} else if string(got) != testWindowsTerminalSettingsWithOtherFace {
		t.Error("building the offer must not change the settings file")
	}
}

// TestInteractiveWSLStepOfferFitsTheInstallingFrame measures the offer state the
// frame guards do not carry: it reuses ScreenInstalling, so the derived coverage
// guard sees the screen, but nothing measures the frame while the offer is on it.
// A question that overflows the frame would be clipped, so this renders it at the
// 80x24 floor with the same fit assertion the installer's screens use.
func TestInteractiveWSLStepOfferFitsTheInstallingFrame(t *testing.T) {
	repoDir, _, settingsPath := newWindowsTerminalWSLLayout(t)
	stubWindowsFontQueryStateful(t, map[string]bool{})
	installWSLTerminalSettings(t, settingsPath, testWindowsTerminalSettingsWithOtherFace)

	m := driveWSLTerminalOfferToTheScreen(t, mountWSLTerminalStep(t, repoDir))
	assertInstallerScreenFits(t, "installing-wsl-terminal-offer", m)
}

// TestInteractiveWSLStepOfferIsSkippedOnADryRun pins the gate runNextStep puts
// around the offer. Without it a documented no-op run would still build the
// offer, ask for a yes, and then copy the Nerd Font into the Windows font store
// on that answer, because installAndVerifyWindowsFont runs before the writer's
// own dry-run check. Under the flag the step takes its ordinary skipped path,
// exactly as runInteractiveStep already decides for every interactive step.
func TestInteractiveWSLStepOfferIsSkippedOnADryRun(t *testing.T) {
	repoDir, profileDir, _ := newWindowsTerminalWSLLayout(t)
	t.Setenv("DOTFILES_DRY_RUN", "1")
	stubWindowsFontQueryStateful(t, map[string]bool{})

	m := mountWSLTerminalStep(t, repoDir)
	msg := m.runNextStep()()

	finished, ok := msg.(execFinishedMsg)
	if !ok {
		t.Fatalf("a dry run produced %T for the WSL step, want execFinishedMsg", msg)
	}
	if finished.stepID != wslStepID {
		t.Fatalf("dry-run message carries step %q, want %q", finished.stepID, wslStepID)
	}
	if _, err := os.Stat(filepath.Join(profileDir, filepath.FromSlash(windowsFontStoreRelPath))); !os.IsNotExist(err) {
		t.Error("a dry run must not copy the font into the Windows store")
	}
}

// TestInteractiveWSLStepRefusalLeavesTheFileUntouched is the refusal half of the
// interactive guard: pressing n answers the offer through Update, and the file,
// the font store and the absence of a backup all have to be exactly what they
// were. The script command the step continues to is inspected but never run, so
// the guard stays hermetic.
func TestInteractiveWSLStepRefusalLeavesTheFileUntouched(t *testing.T) {
	repoDir, profileDir, settingsPath := newWindowsTerminalWSLLayout(t)
	stubWindowsFontQueryStateful(t, map[string]bool{})
	installWSLTerminalSettings(t, settingsPath, testWindowsTerminalSettingsWithOtherFace)

	m := driveWSLTerminalOfferToTheScreen(t, mountWSLTerminalStep(t, repoDir))

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if cmd == nil {
		t.Fatal("refusing the offer returned no command, so the step would never continue to its script")
	}
	m = result.(Model)

	// The refusal runs through the same apply path the write does; run it and feed
	// the answer back, then stop before the script command runs a shell.
	applied := cmd()
	if _, ok := applied.(wslTerminalOfferAppliedMsg); !ok {
		t.Fatalf("refusing produced %T, want wslTerminalOfferAppliedMsg", applied)
	}
	result, scriptCmd := m.Update(applied)
	m = result.(Model)
	if scriptCmd == nil {
		t.Fatal("the refused step produced no script command, so the rest of the WSL step would not run")
	}

	if got, err := os.ReadFile(settingsPath); err != nil {
		t.Fatal(err)
	} else if string(got) != testWindowsTerminalSettingsWithOtherFace {
		t.Error("refusing must leave the settings file exactly as it was")
	}
	if backups, _ := filepath.Glob(settingsPath + ".bak-dotfiles-*"); len(backups) != 0 {
		t.Errorf("refusing must leave no backup, got %v", backups)
	}
	if _, err := os.Stat(filepath.Join(profileDir, filepath.FromSlash(windowsFontStoreRelPath))); !os.IsNotExist(err) {
		t.Error("refusing must not copy the font into the Windows store")
	}
	if m.WSLTerminalOffer != nil {
		t.Error("the offer must be cleared once it is answered")
	}
}

// TestInteractiveWSLStepYesWritesTheFile is the yes half: pressing y answers the
// offer through Update, and the font is installed and verified before the face is
// written through the same writer the non-interactive step uses, with the previous
// file kept beside it. It is the proof that the interactive screen, and not only
// the flag, can reach the write.
func TestInteractiveWSLStepYesWritesTheFile(t *testing.T) {
	repoDir, profileDir, settingsPath := newWindowsTerminalWSLLayout(t)
	scripts := stubWindowsFontQueryStateful(t, map[string]bool{})
	installWSLTerminalSettings(t, settingsPath, testWindowsTerminalSettingsWithOtherFace)

	m := driveWSLTerminalOfferToTheScreen(t, mountWSLTerminalStep(t, repoDir))

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if cmd == nil {
		t.Fatal("accepting the offer returned no command, so nothing could be written")
	}
	m = result.(Model)

	applied := cmd()
	result, scriptCmd := m.Update(applied)
	m = result.(Model)
	if scriptCmd == nil {
		t.Fatal("the accepted step produced no script command, so the rest of the WSL step would not run")
	}

	store := filepath.Join(profileDir, filepath.FromSlash(windowsFontStoreRelPath))
	for _, face := range windowsFontFaces {
		if _, err := os.Stat(filepath.Join(store, face.File)); err != nil {
			t.Errorf("the face %s was not copied into the Windows font store: %v", face.File, err)
		}
	}
	registered, verified := false, false
	for _, script := range *scripts {
		if strings.Contains(script, "New-ItemProperty") {
			registered = true
		}
		if strings.Contains(script, "InstalledFontCollection") && strings.Contains(script, windowsTerminalFontFace) {
			verified = true
		}
	}
	if !registered {
		t.Errorf("the family must be registered under HKCU before the face is written; queries were %v", *scripts)
	}
	if !verified {
		t.Errorf("the family %q must be verified in the font store before the face is written; queries were %v", windowsTerminalFontFace, *scripts)
	}

	after, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	root, err := parseJSONC(after)
	if err != nil {
		t.Fatalf("the written settings are not valid JSONC: %v\n%s", err, after)
	}
	defaults := root.findMember("profiles").value.findMember("defaults").value
	if face := defaults.findMember("font").value.findMember("face"); face == nil || face.value.text != windowsTerminalFontFace {
		t.Errorf("profiles.defaults.font.face was not written with %q", windowsTerminalFontFace)
	}
	if cs := defaults.findMember("colorScheme"); cs == nil || cs.value.text != windowsTerminalSchemeName {
		t.Errorf("profiles.defaults.colorScheme was not written with %q", windowsTerminalSchemeName)
	}
	backups, _ := filepath.Glob(settingsPath + ".bak-dotfiles-*")
	if len(backups) != 1 {
		t.Fatalf("the write must leave exactly one backup, got %v", backups)
	}
	if saved, err := os.ReadFile(backups[0]); err != nil {
		t.Fatal(err)
	} else if string(saved) != testWindowsTerminalSettingsWithOtherFace {
		t.Error("the backup must hold the other tool's settings as they were")
	}
	if m.WSLTerminalOffer != nil {
		t.Error("the offer must be cleared once it is answered")
	}
}

// TestParseWindowsFontPresenceHandlesTheConsoleBOM pins constraint 4's second
// half: the console can prepend a BOM to redirected output, and the parser must
// still read the answer. Anything that is not one of the two answers is "no
// answer" rather than an absent font, because the step refuses the face either
// way but reports a different reason.
func TestParseWindowsFontPresenceHandlesTheConsoleBOM(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		present bool
		known   bool
	}{
		{"present", "font=present\r\n", true, true},
		{"absent with a BOM", "\ufefffont=absent\r\n", false, true},
		{"present with a BOM and noise", "\ufeffsome noise\r\nfont=present\r\n", true, true},
		{"no answer", "some noise\r\n", false, false},
		{"empty", "", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			present, known := parseWindowsFontPresence([]byte(tt.output))
			if present != tt.present || known != tt.known {
				t.Errorf("parseWindowsFontPresence(%q) = (%v, %v), want (%v, %v)", tt.output, present, known, tt.present, tt.known)
			}
		})
	}
}

// TestWindowsFontScriptsNameTheFamilyAndOnlyTheCopiedFaces pins the two PowerShell
// programs the boundary runs: registration writes one HKCU value per copied face
// and none of the uncopied ones, and the verification asks the font store for the
// family the settings will name. A script that registered the whole family or
// verified a different family would break the constraint-2 and constraint-1
// promises respectively.
func TestWindowsFontScriptsNameTheFamilyAndOnlyTheCopiedFaces(t *testing.T) {
	copied := windowsFontFaces[:2]
	register := windowsFontRegisterScript(copied)
	if !strings.Contains(register, "HKCU:\\Software\\Microsoft\\Windows NT\\CurrentVersion\\Fonts") {
		t.Errorf("the registration must write the per-user font key, got:\n%s", register)
	}
	for _, face := range copied {
		if !strings.Contains(register, face.File) {
			t.Errorf("the registration is missing the copied face %s:\n%s", face.File, register)
		}
	}
	for _, face := range windowsFontFaces[2:] {
		if strings.Contains(register, face.File) {
			t.Errorf("the registration includes %s, which was not copied:\n%s", face.File, register)
		}
	}

	verify := windowsFontVerifyScript()
	for _, want := range []string{"InstalledFontCollection", windowsTerminalFontFace} {
		if !strings.Contains(verify, want) {
			t.Errorf("the verification should ask the font store for %q, got:\n%s", want, verify)
		}
	}
}

// TestWindowsFontRegistrationCannotUninstallTheFontsItDoesNotOwn pins the two
// promises the registration program has to keep on the machine it runs on, both
// of which were broken at once and both of which were measured.
//
// The key it writes into is not the installer's: it holds every per-user font on
// the machine. PowerShell's New-Item -Force recreates a key that already exists
// and discards the values it held, so an unguarded New-Item over this key
// uninstalls every per-user font the installer did not put there. That is not
// hypothetical: the cost measured on the machine this was found on was 36
// registrations, the whole CaskaydiaCove Nerd Font family, whose .ttf files were
// left behind in the store with nothing to resolve them by. The guard below is
// the whole fix for that half.
//
// The second half is why the family never resolved. Windows follows a per-user
// face through the absolute path in the value; a bare file name is resolved
// against %WINDIR%\Fonts, where these faces are not, so a registration naming one
// is a registration Windows cannot follow -- and the verification that gates the
// face write then refuses, correctly, forever.
func TestWindowsFontRegistrationCannotUninstallTheFontsItDoesNotOwn(t *testing.T) {
	register := windowsFontRegisterScript(windowsFontFaces)

	// Every New-Item must sit behind the Test-Path guard. An unguarded one is how
	// the key's existing values are lost, so this is the failure the test exists
	// to catch rather than a preference about how the program reads.
	for i, line := range strings.Split(register, "\n") {
		// "New-Item -Path" and not "New-ItemProperty": only the key creation
		// recreates the key, and the value writes are what must survive it.
		if !strings.Contains(line, "New-Item -Path") {
			continue
		}
		if !strings.Contains(line, "Test-Path") {
			t.Errorf("line %d creates the key without checking whether it exists, which discards every registration already in it:\n%s", i+1, line)
		}
	}

	// The store directory is the one fact that cannot be hard-coded from the
	// Linux side: it is %LOCALAPPDATA% on Windows, which only the Windows side
	// knows, so the program resolves it there and joins it, rather than being
	// handed a path some other layer guessed.
	if !strings.Contains(register, "Join-Path $env:LOCALAPPDATA") {
		t.Errorf("the registration must resolve the per-user font store from %%LOCALAPPDATA%%, got:\n%s", register)
	}

	for _, face := range windowsFontFaces {
		if !strings.Contains(register, "Join-Path $store "+psSingleQuote(face.File)) {
			t.Errorf("the registration must write %s joined onto the store, so the value is an absolute path Windows can follow:\n%s", face.File, register)
		}
		if strings.Contains(register, "-Value "+psSingleQuote(face.File)) {
			t.Errorf("the registration writes the bare file name %s as the value, which is resolved against %%WINDIR%%\\Fonts and cannot be followed:\n%s", face.File, register)
		}
	}
}

// TestWindowsFontQueryIsBounded pins constraint 5: a Windows-side query that
// hangs times out into a reported refusal instead of blocking the installer. The
// fake honours its context, as the real system.WindowsQuery does through
// exec.CommandContext and WaitDelay, and the step must return once the deadline
// fires.
func TestWindowsFontQueryIsBounded(t *testing.T) {
	repoDir, _, settingsPath := newWindowsTerminalWSLLayout(t)
	setNonInteractiveForTest(t, true)

	previousTimeout := windowsFontQueryTimeout
	windowsFontQueryTimeout = 20 * time.Millisecond
	t.Cleanup(func() { windowsFontQueryTimeout = previousTimeout })

	previousQuery := windowsFontQuery
	windowsFontQuery = func(ctx context.Context, _ string) ([]byte, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	t.Cleanup(func() { windowsFontQuery = previousQuery })

	m := wslModel(repoDir, true)
	start := time.Now()
	if err := stepInstallWSLConfig(&m); err != nil {
		t.Fatalf("a hanging Windows query must not fail the step: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("the hanging query blocked the step for %s; it must be bounded", elapsed)
	}
	if after, err := os.ReadFile(settingsPath); err != nil {
		t.Fatal(err)
	} else if string(after) != testWindowsTerminalSettings {
		t.Error("a query that cannot be answered must leave the face unwritten")
	}
}

// TestAPartialLandingIsAShortfallAndStillReports is the half of #288 that main did not have.
// Its report was already per-value, but it returned no error, so every caller could print a
// checkmark over a write that landed one of the two settings. A partial landing must be a
// shortfall the caller cannot mistake for success, and the notice must still carry the whole
// report, so the log says which value did not land and what the file holds instead.
func TestAPartialLandingIsAShortfallAndStillReports(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	// A settings file the writer would edit: an empty profiles.defaults, no schemes.
	original := []byte("{\n  \"profiles\": {\n    \"defaults\": {}\n  }\n}\n")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatalf("writing the fixture: %v", err)
	}

	scheme := windowsTerminalScheme{Name: "dotfiles"}
	face := "MesloLGS Nerd Font"
	whole, err := windowsTerminalSettingsContent(original, scheme, face)
	if err != nil {
		t.Fatalf("the writer refused to build a whole landing: %v", err)
	}
	// The face lands and the scheme does not: the file holds the face and no colorScheme.
	half := []byte("{\n  \"profiles\": {\n    \"defaults\": {\n      \"font\": {\n        \"face\": \"MesloLGS Nerd Font\"\n      }\n    }\n  }\n}\n")

	for _, tc := range []struct {
		name  string
		bytes []byte
		want  bool // whether both settings landed
	}{
		{"both settings land", whole, true},
		{"only the face lands", half, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			offer := windowsTerminalOffer{
				Available: true,
				Path:      path,
				Original:  original,
				updated:   tc.bytes,
				Face:      face,
				Scheme:    scheme.Name,
			}
			notice, wrote, err := applyWindowsTerminalOffer(offer, true, wslStepID)
			if !wrote {
				t.Fatalf("the file was written, so wrote must be true")
			}
			if tc.want {
				if err != nil {
					t.Errorf("a whole landing must not be a shortfall: %v", err)
				}
				return
			}
			if err == nil {
				t.Errorf("a landing that did not land whole must be a shortfall, not a success, so no caller can print a checkmark over it; notice was %q", notice)
			}
			if !strings.Contains(notice, "did not land") {
				t.Errorf("the notice must still say what did not land, got %q", notice)
			}
			if err != nil && !strings.Contains(err.Error(), "did not land") {
				t.Errorf("the shortfall must name the value that did not land, got %q", err.Error())
			}
		})
	}
}
