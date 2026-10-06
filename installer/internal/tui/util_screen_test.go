package tui

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// These tests cover the third ownership proof: a file with no ownership marker,
// no generated block and no byte-identity to what the repository ships, but
// whose hand-written region carries the anchors the generator knows. The switch
// must adopt only that region and leave every other byte exactly as it was.

// userForeignPrefix and userForeignSuffix are the bytes a user's own file holds
// around the managed region - comments, keys and order dotfiles never wrote. The
// guards below assert they survive the apply byte-for-byte.
const (
	userForeignPrefix = "# my own configuration, written by hand\nset -x MY_OWN_KEY 1\n\n"
	userForeignSuffix = "# --- my own footer ---\nmy_own_function() { echo mine; }\n"
)

// userLegacyStarshipPalettes is the hand-written Starship palette table an
// older checkout installed, transcribed from
// git show b16e4ac^:starship.toml. Its [palettes.catppuccin_mocha] header is the
// region anchor; the generator no longer writes that table (nor these uppercase
// values), so byte-identity cannot prove the file and only the anchors can.
const userLegacyStarshipPalettes = `[palettes.catppuccin_mocha]
rosewater = "#f5e0dc"
flamingo = "#f2cdcd"
pink = "#f5c2e7"
mauve = "#cba6f7"
red = "#f38ba8"
maroon = "#eba0ac"
peach = "#fab387"
yellow = "#f9e2af"
green = "#a6e3a1"
teal = "#94e2d5"
sky = "#89dceb"
sapphire = "#74c7ec"
blue = "#89b4fa"
lavender = "#b4befe"
text = "#cdd6f4"
subtext1 = "#bac2de"
subtext0 = "#a6adc8"
overlay2 = "#9399b2"
overlay1 = "#7f849c"
overlay0 = "#6c7086"
surface2 = "#585b70"
surface1 = "#45475a"
surface0 = "#313244"
base = "#1e1e2e"
mantle = "#181825"
crust = "#11111b"

[palettes.dotfiles]
text = "#F3F6F9"
red = "#CB7C94"
green = "#B7CC85"
yellow = "#FFE066"
blue = "#7FB4CA"
mauve = "#A3B5D6"
pink = "#FF8DD7"
teal = "#7AA89F"
peach = "#DEBA87"
subtext0 = "#5C6170"
overlay0 = "#232A40"
rosewater = "#E0C15A"
flamingo = "#FF8DD7"
maroon = "#C4746E"
lavender = "#B99BF2"
subtext1 = "#8A8FA3"
overlay2 = "#313342"
overlay1 = "#191E28"
surface2 = "#27345C"
surface1 = "#232A40"
surface0 = "#191E28"
base = "none"
mantle = "#06080f"
crust = "#06080f"`

// userLegacyFishRegion is the hand-written fish palette an older checkout
// installed, transcribed from
// git show 758790b^:dotfiles-fish/fish/config.fish. The generator writes
// `set -g fish_color_normal f3f6f9` today, so none of this file matches what the
// repository ships; only the region anchors identify it.
const userLegacyFishRegion = `set -l foreground F3F6F9 normal
set -l selection 263356 normal
set -l comment 8394A3 brblack
set -l red CB7C94 red
set -l orange DEBA87 orange
set -l yellow FFE066 yellow
set -l green B7CC85 green
set -l purple A3B5D6 purple
set -l cyan 7AA89F cyan
set -l pink FF8DD7 magenta

# Syntax Highlighting Colors
set -g fish_color_normal $foreground
set -g fish_color_command $cyan
set -g fish_color_keyword $pink
set -g fish_color_quote $yellow
set -g fish_color_redirection $foreground
set -g fish_color_end $orange
set -g fish_color_error $red
set -g fish_color_param $purple
set -g fish_color_comment $comment
set -g fish_color_selection --background=$selection
set -g fish_color_search_match --background=$selection
set -g fish_color_operator $green
set -g fish_color_escape $pink
set -g fish_color_autosuggestion $comment

# Completion Pager Colors
set -g fish_pager_color_progress $comment
set -g fish_pager_color_prefix $cyan
set -g fish_pager_color_completion $foreground
set -g fish_pager_color_description $comment`

// userHerdrFixture is the user's Herdr config: the hand-written region the
// repository shipped before the generator, with the file's own keys and order
// around it. The region is the generated block with the marker and tag lines
// stripped, which is exactly the hand-written shape the anchors bracket.
func userHerdrFixture(t *testing.T) string {
	t.Helper()
	art := artifactByName(t, "herdr")
	def, ok := themeByID(mustLoadDefinitions(t), defaultThemeID)
	if !ok {
		t.Fatalf("%s is not defined", defaultThemeID)
	}
	return userForeignPrefix + legacyThemeFile(t, art, def) + userForeignSuffix
}

func mustLoadDefinitions(t *testing.T) []themeDefinition {
	t.Helper()
	defs, err := loadThemeDefinitions(tempThemeRepo(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	return defs
}

// applyTarget loads the repository definitions and returns catppuccin-mocha,
// the theme the switch tests apply on top of the dotfiles fixtures.
func applyTarget(t *testing.T) themeDefinition {
	t.Helper()
	def, ok := themeByID(mustLoadDefinitions(t), "catppuccin-mocha")
	if !ok {
		t.Fatal("the catppuccin-mocha definition is missing")
	}
	return def
}

// TestThemeRegionAdoptionRewritesOnlyTheManagedRegion is the user's exact case:
// an unmarked file that has drifted and carries no generated block, but whose
// hand-written region still holds the anchors the generator knows. Today the
// switch refuses it; it must instead rewrite the region between the anchors and
// leave the bytes around it untouched.
func TestThemeRegionAdoptionRewritesOnlyTheManagedRegion(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("DOTFILES_DRY_RUN", "0")

	home := t.TempDir()
	target := applyTarget(t)

	art := artifactByName(t, "herdr")
	dst := themeInstalledPath(art, home)
	original := userHerdrFixture(t)
	writeThemeFileAt(t, dst, original)

	rec, notice, err := applyDotfilesTheme(home, repoRoot(t), target)
	if err != nil {
		t.Fatalf("apply the theme to a file whose region carries the anchors: %v", err)
	}
	if rec == nil {
		t.Fatal("the region adoption returned no record, so the change is not reversible")
	}
	if strings.Contains(notice, "DRY RUN") {
		t.Fatalf("a real run reported a dry run: %q", notice)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), themeBeginTag(art.Block)) {
		t.Errorf("the adopted file carries no generated block:\n%s", got)
	}
	if !strings.Contains(string(got), target.Palette["selection"]) {
		t.Errorf("the region was not rewritten with the %s theme (no %s)", target.Name, target.Palette["selection"])
	}
	// The teeth: the bytes outside the anchors are preserved exactly. A rewrite
	// that reformatted, reordered or dropped the surrounding file fails here.
	if !strings.HasPrefix(string(got), userForeignPrefix) {
		t.Errorf("the bytes before the region changed:\n%s", got)
	}
	if !strings.HasSuffix(string(got), userForeignSuffix) {
		t.Errorf("the bytes after the region changed:\n%s", got)
	}

	if _, err := undoDotfilesTheme(*rec); err != nil {
		t.Fatalf("undo the region adoption: %v", err)
	}
	back, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(back, []byte(original)) {
		t.Errorf("undo did not restore the file byte-for-byte\n got: %q\nwant: %q", back, original)
	}
}

// TestThemeRegionAdoptionLeavesForeignContentIntact is the guard with the
// user's own content around the region: their comments, their keys and their
// order. It proves the region adoption writes nothing outside the anchors by
// comparing the exact prefix and suffix bytes before and after.
func TestThemeRegionAdoptionLeavesForeignContentIntact(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("DOTFILES_DRY_RUN", "0")

	home := t.TempDir()
	target := applyTarget(t)

	art := artifactByName(t, "zsh")
	dst := themeInstalledPath(art, home)
	def, _ := themeByID(mustLoadDefinitions(t), defaultThemeID)

	prefix := "# my zsh aliases\nalias ll='ls -la'\nalias gs='git status'\n\nexport MY_EDITOR=nvim\n\n"
	suffix := "\n# my own functions\nmkcd() { mkdir -p \"$1\" && cd \"$1\"; }\n\nsetopt hist_ignore_all_dups\n"
	region := legacyThemeFile(t, art, def)
	original := prefix + region + suffix
	writeThemeFileAt(t, dst, original)

	if _, _, err := applyDotfilesTheme(home, repoRoot(t), target); err != nil {
		t.Fatalf("apply the theme around the user's own .zshrc content: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), themeBeginTag(art.Block)) {
		t.Errorf("the zsh region was not adopted:\n%s", got)
	}
	if !strings.HasPrefix(string(got), prefix) {
		t.Errorf("the user's aliases and exports, before the region, were changed:\n%s", got)
	}
	if !strings.HasSuffix(string(got), suffix) {
		t.Errorf("the user's functions and options, after the region, were changed:\n%s", got)
	}
	if !strings.Contains(string(got), "alias ll='ls -la'") || !strings.Contains(string(got), "setopt hist_ignore_all_dups") {
		t.Errorf("the user's own lines are missing from the result:\n%s", got)
	}
}

// TestThemeRegionAdoptionCoversTheUsersFiveFiles is the user's machine in one
// test. Each of the five files they reported has no marker and no generated
// block, and each carried the hand-written region whose anchors the generator
// knows, with the file's own bytes around it. Every one must be adopted and
// every foreign byte preserved.
func TestThemeRegionAdoptionCoversTheUsersFiveFiles(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("DOTFILES_DRY_RUN", "0")

	home := t.TempDir()
	target := applyTarget(t)
	def, _ := themeByID(mustLoadDefinitions(t), defaultThemeID)

	herdrArt := artifactByName(t, "herdr")
	zshArt := artifactByName(t, "zsh")
	p10kArt := artifactByName(t, "p10k")
	paletteArt := artifactByName(t, "starship")

	files := map[string]string{
		themeInstalledPath(herdrArt, home): userForeignPrefix + legacyThemeFile(t, herdrArt, def) + userForeignSuffix,
		themeInstalledPath(zshArt, home):   userForeignPrefix + legacyThemeFile(t, zshArt, def) + userForeignSuffix,
		themeInstalledPath(p10kArt, home):  userForeignPrefix + legacyThemeFile(t, p10kArt, def) + userForeignSuffix,
		themeInstalledPath(paletteArt, home): userForeignPrefix +
			`palette = "dotfiles"` + "\n\n[fill]\nsymbol = ' '\n\n" +
			userLegacyStarshipPalettes + "\n" + userForeignSuffix,
	}
	// fish has no artifact helper that matches it by tool, so it is addressed by
	// its own path and the legacy region the generator no longer reproduces.
	fishPath := themeInstalledPath(themeArtifact{Tool: "fish"}, home)
	files[fishPath] = userForeignPrefix + userLegacyFishRegion + "\n" + userForeignSuffix

	for path, content := range files {
		writeThemeFileAt(t, path, content)
	}

	rec, _, err := applyDotfilesTheme(home, repoRoot(t), target)
	if err != nil {
		t.Fatalf("apply the theme to the user's five files: %v", err)
	}
	if rec == nil {
		t.Fatal("the five-file adoption returned no record, so it is not reversible")
	}
	if len(rec.Files) != len(files) {
		t.Errorf("the record holds %d file(s), the switch touched %d", len(rec.Files), len(files))
	}

	for path := range files {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(got), ">>> dotfiles-theme") {
			t.Errorf("%s was not adopted into the generated form:\n%s", path, got)
		}
		if !strings.HasPrefix(string(got), userForeignPrefix) {
			t.Errorf("%s lost the user's bytes before the region", path)
		}
		if !strings.HasSuffix(string(got), userForeignSuffix) {
			t.Errorf("%s lost the user's bytes after the region", path)
		}
	}

	if _, err := undoDotfilesTheme(*rec); err != nil {
		t.Fatalf("undo the five-file adoption: %v", err)
	}
	for path, want := range files {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Errorf("%s was not restored byte-for-byte after undo\n got: %q\nwant: %q", path, got, want)
		}
	}
}

// TestThemeRefusalNamesTheProbesAndTheWayForward pins the message. A refusal
// that says only "not owned" is a closed door with no sign, so it must name the
// file, every proof the switch attempted - the marker, the generated block, the
// region anchors, the byte-identity - and what the user can do next.
func TestThemeRefusalNamesTheProbesAndTheWayForward(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("DOTFILES_DRY_RUN", "0")

	home := t.TempDir()
	target := applyTarget(t)

	art := artifactByName(t, "herdr")
	dst := themeInstalledPath(art, home)
	user := "# my own herdr config\nsome_setting = true\n"
	writeThemeFileAt(t, dst, user)

	_, _, err := applyDotfilesTheme(home, repoRoot(t), target)
	if err == nil {
		t.Fatal("the switch accepted a file that carries none of the proofs")
	}
	msg := err.Error()
	for _, want := range []string{
		dst,
		"not owned",
		themeOwnershipMarker,
		"generated",
		"anchor",
		"identical",
		"reinstall",
		"leave",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("the refusal does not name %q:\n%s", want, msg)
		}
	}
	got, readErr := os.ReadFile(dst)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != user {
		t.Error("the refused file was changed")
	}
}
