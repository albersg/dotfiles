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
// saw the offer at all.
func TestInteractiveWSLStepShowsTheOfferOnTheInstallingScreen(t *testing.T) {
	repoDir, _, settingsPath := newWindowsTerminalWSLLayout(t)
	stubWindowsFontQueryStateful(t, map[string]bool{})
	installWSLTerminalSettings(t, settingsPath, testWindowsTerminalSettingsWithOtherFace)

	m := driveWSLTerminalOfferToTheScreen(t, mountWSLTerminalStep(t, repoDir))

	// The screen shows the offer, and the screen is what a person reads: the
	// assertion is on the rendered frame, not only on the model field.
	view := strings.ReplaceAll(m.View(), "\n", "")
	for _, want := range []string{
		"Windows Terminal's settings.json is reachable at",
		"CascadiaMonoNF",
		"OneHalfDark",
		"configure Windows Terminal or leave it unchanged",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("the installing screen does not show %q, so a write could happen without the user reading what it replaces:\n%s", want, m.View())
		}
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

// partialWindowsTerminalSettings is the bytes a write that lost the colour
// scheme leaves behind: profiles.defaults.font.face is set and
// profiles.defaults.colorScheme is not. It is the half-landed file a write can
// leave, and the report has to name it rather than the settings that were sent.
func partialWindowsTerminalSettings(face string) string {
	return `{"profiles":{"defaults":{"font":{"face":"` + face + `"}}},"schemes":[]}`
}

// windowsTerminalTestLabel names one owned setting the way the report does, by
// reading the same list the report reads. The test builds the expectation itself
// so a report that names a setting the list does not own fails here.
func windowsTerminalTestLabel(t *testing.T, id, face string) string {
	t.Helper()
	for _, setting := range windowsTerminalSettings {
		if setting.ID != id {
			continue
		}
		if setting.ID == "font-face" {
			return "the font face " + face
		}
		return setting.Label
	}
	t.Fatalf("no owned setting with id %q", id)
	return ""
}

// TestApplyWindowsTerminalOfferReportsWhatTheFileHolds is the reporting contract
// for one write: the notice names the settings the file now holds, not the ones
// the writer sent, and a landing that is not whole is a shortfall rather than a
// success. The offer's bytes are the fixture because a partial landing is only
// reachable through a write that leaves a setting behind; the backup, the write,
// the read-back and the report are the production ones.
func TestApplyWindowsTerminalOfferReportsWhatTheFileHolds(t *testing.T) {
	for _, tt := range []struct {
		name        string
		updated     func(face string) string
		wantLanded  []string
		wantMissing []string
	}{
		{
			// A nil updated keeps the bytes the production writer computed, so the
			// whole-landing case exercises the real content as well as the report.
			name:       "both settings land",
			wantLanded: []string{"font-face", "colorScheme"},
		},
		{
			name:        "only the face lands",
			updated:     partialWindowsTerminalSettings,
			wantLanded:  []string{"font-face"},
			wantMissing: []string{"colorScheme"},
		},
		{
			name: "only the scheme lands",
			updated: func(string) string {
				return `{"profiles":{"defaults":{"colorScheme":"` + windowsTerminalSchemeName + `"}},"schemes":[]}`
			},
			wantLanded:  []string{"colorScheme"},
			wantMissing: []string{"font-face"},
		},
		{
			name: "neither lands",
			updated: func(string) string {
				return `{"profiles":{"defaults":{}},"schemes":[]}`
			},
			wantMissing: []string{"font-face", "colorScheme"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			profileDir, settingsPath := newWindowsTerminalProfile(t, testWindowsTerminalSettings)
			offer := readWindowsTerminalOffer(profileDir, testWindowsTerminalThemeDefinition(), windowsTerminalFontFace)
			if !offer.Available {
				t.Fatalf("the offer must be available: %s", offer.Reason)
			}
			if tt.updated != nil {
				offer.updated = []byte(tt.updated(offer.Face))
			}

			notice, wrote, err := applyWindowsTerminalOffer(offer, true, wslStepID)
			if !wrote {
				t.Fatalf("the write must happen, wrote=%v err=%v", wrote, err)
			}
			if len(tt.wantMissing) == 0 && err != nil {
				t.Errorf("a whole landing must not be a shortfall: %v", err)
			}
			if len(tt.wantMissing) > 0 && err == nil {
				t.Error("a landing that did not land whole must be a shortfall, not a success")
			}

			for _, id := range tt.wantLanded {
				label := windowsTerminalTestLabel(t, id, offer.Face)
				if !strings.Contains(notice, label) {
					t.Errorf("the report must name %q as landed:\n%s", label, notice)
				}
			}
			for _, id := range tt.wantMissing {
				label := windowsTerminalTestLabel(t, id, offer.Face)
				if !strings.Contains(notice, label) {
					t.Errorf("the report must name %q as not landed:\n%s", label, notice)
				}
			}
			if len(tt.wantMissing) > 0 && !strings.Contains(notice, "did not land") {
				t.Errorf("the report must say what did not land:\n%s", notice)
			}
			if len(tt.wantMissing) == 0 && !strings.Contains(notice, "read back") {
				t.Errorf("a whole landing must say the settings were read back from the file:\n%s", notice)
			}

			after, readErr := os.ReadFile(settingsPath)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if string(after) != string(offer.updated) {
				t.Errorf("the file must hold the bytes the write reported:\n%s", after)
			}
		})
	}
}

// TestInteractiveWSLStepReportsAPartialLanding is the reachability guard for the
// read-back: it drives the offer onto the installing screen through the TUI's own
// step loop, answers yes, and makes the write land only the font face. The report
// must name the half that landed and the half that did not, on the screen a person
// reads, and it must not read as a success. Before the read-back the step reported
// both settings written whatever the file held, which is the defect: a partial
// landing told as a success.
func TestInteractiveWSLStepReportsAPartialLanding(t *testing.T) {
	repoDir, _, settingsPath := newWindowsTerminalWSLLayout(t)
	stubWindowsFontQueryStateful(t, map[string]bool{})
	installWSLTerminalSettings(t, settingsPath, testWindowsTerminalSettingsWithOtherFace)

	m := driveWSLTerminalOfferToTheScreen(t, mountWSLTerminalStep(t, repoDir))
	// The details panel is opened and the frame is tall enough for its log box,
	// so the report the step logs is the report a person reads off the screen.
	m.ShowDetails = true
	m.Height = 40
	face := m.WSLTerminalOffer.Face

	// A real run reaches the offer with earlier lines already in the log, and the
	// screen's log box shows them; the guard lands the offer's own report as the
	// newest line of a log that already has one, so the screen renders it.
	m = m.appendInstallLog("Installing dotfiles-wsl artifacts…")

	// The offer is the production one; only the bytes a write would leave behind
	// are the half-landed ones, so the write, the backup, the read-back and the
	// report all run for real.
	m.WSLTerminalOffer.updated = []byte(partialWindowsTerminalSettings(face))

	result, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if cmd == nil {
		t.Fatal("accepting the offer returned no command, so nothing could be written")
	}
	m = result.(Model)

	applied := cmd()
	result, _ = m.Update(applied)
	m = result.(Model)

	report := strings.Join(m.LogLines, "\n")
	if !strings.Contains(report, face) {
		t.Errorf("the report must name the face %q that landed:\n%s", face, report)
	}
	if !strings.Contains(report, windowsTerminalSchemeName) {
		t.Errorf("the report must name the colour scheme %q that did not land:\n%s", windowsTerminalSchemeName, report)
	}
	if !strings.Contains(report, "did not land") {
		t.Errorf("the report must say the colour scheme did not land:\n%s", report)
	}
	if strings.Contains(report, "✓") {
		t.Errorf("a partial landing must not be reported as a success:\n%s", report)
	}

	// The screen is what a person reads, so the partial landing has to be on it.
	// The log box clips a long line, so the assertion is on the part of the report
	// the screen can show: the run is told the write was only partly configured,
	// not that it succeeded.
	view := strings.ReplaceAll(m.View(), "\n", "")
	if !strings.Contains(view, "only partly configured") {
		t.Errorf("the installing screen must show the partial landing:\n%s", m.View())
	}
	if strings.Contains(view, "✓") {
		t.Errorf("the installing screen must not show a success mark for a partial landing:\n%s", m.View())
	}

	// The report is a fact about the file, so the file holds the half that landed.
	after, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := windowsTerminalCurrentFontFace(after); !ok || got != face {
		t.Errorf("the write should have landed the face %q, file holds %q", face, got)
	}
	if scheme, ok := windowsTerminalCurrentColorScheme(after); ok {
		t.Errorf("the fixture's write landed no colour scheme, so the file must not hold one, got %q", scheme)
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
