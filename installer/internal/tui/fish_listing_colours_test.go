package tui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// These guards cover stage 2 of fish-parity.md: the listing's colours, not only its
// shape. The mapping -- which extension or key takes which palette role -- is
// structure and is written once, in the two tables zsh is rendered from; what
// differs between the shells is the spelling of a variable reference and of an
// export. fish reads what zsh writes once it is spelled that way: measured on this
// machine with fish's own parser, `$VAR` expands inside a double-quoted value, a
// backslash at the end of a line continues that value without keeping the newline,
// and `set -gx` exports the result so eza reads it out of the environment.

// shippedThemeIDs names every definition in themes/, so the guards below run over
// what the switch can actually be asked to apply rather than over one example.
func shippedThemeIDs(t *testing.T) []string {
	t.Helper()
	paths, err := filepath.Glob("../../../themes/*.toml")
	if err != nil {
		t.Fatalf("listing the theme definitions: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no theme definitions found under themes/, so these guards would pass over nothing")
	}
	ids := make([]string, 0, len(paths))
	for _, path := range paths {
		ids = append(ids, strings.TrimSuffix(filepath.Base(path), ".toml"))
	}
	return ids
}

// renderThemePair renders both blocks for one definition: the zsh theme block and
// the fish config block, which are the two artifacts a switch rewrites.
func renderThemePair(t *testing.T, id string) (string, string, themeDefinition) {
	t.Helper()
	data, err := os.ReadFile("../../../themes/" + id + ".toml")
	if err != nil {
		t.Fatalf("reading the %s definition: %v", id, err)
	}
	def, err := parseThemeDefinition(data)
	if err != nil {
		t.Fatalf("parsing the %s definition: %v", id, err)
	}
	zsh, err := renderZshTheme(def)
	if err != nil {
		t.Fatalf("rendering the %s zsh block: %v", id, err)
	}
	fish, err := renderFishConfig(def)
	if err != nil {
		t.Fatalf("rendering the %s fish block: %v", id, err)
	}
	return zsh, fish, def
}

// declaredValues reads the palette variables a generated block declares, in that
// shell's own form: `typeset -g NAME="value"` for zsh, `set -g NAME "value"` for
// fish. The zsh block aligns two declarations on one line to keep the hex and the SGR
// form of each role together, so this reads every match in the block rather than one
// per line; both names are `PALETTE_*`, which is what keeps the two listing tables
// themselves out of the result.
func declaredValues(block, shell string) map[string]string {
	pattern := `(PALETTE_[A-Z0-9_]+)="([^"]*)"`
	if shell == "fish" {
		pattern = `(?m)^set -gx? (PALETTE_[A-Z0-9_]+) "([^"]*)"$`
	}
	values := map[string]string{}
	for _, match := range regexp.MustCompile(pattern).FindAllStringSubmatch(block, -1) {
		values[match[1]] = match[2]
	}
	return values
}

// listingEntries reads one listing table out of a generated block and returns its
// entries in order, with every variable reference replaced by the value the same
// block declares for it. Nothing else than the reference spelling is normalised:
// the point of the comparison is that the mapping itself is one.
func listingEntries(t *testing.T, block, table, shell string) []string {
	t.Helper()
	var declaration, reference string
	switch shell {
	case "zsh":
		declaration, reference = `export `+table+`="`, `\$\{([A-Z0-9_]+)\}`
	case "fish":
		declaration, reference = `set -gx `+table+` "`, `\$([A-Z0-9_]+)`
	default:
		t.Fatalf("unknown shell %q", shell)
	}

	start := strings.Index(block, declaration)
	if start < 0 {
		t.Fatalf("the %s block declares no %s at all, so %s reaches the tool unset and the listing falls back to that tool's own colours: the fish block holds %d bytes and no listing table",
			shell, table, table, len(block))
	}
	rest := block[start+len(declaration):]
	end := strings.Index(rest, `"`)
	if end < 0 {
		t.Fatalf("the %s block's %s value is never closed", shell, table)
	}
	// A backslash at the end of a line continues the quoted value in both shells,
	// and the newline it escapes is not part of it.
	value := strings.ReplaceAll(rest[:end], "\\\n", "")

	values := declaredValues(block, shell)
	pattern := regexp.MustCompile(reference)
	var undeclared []string
	expanded := pattern.ReplaceAllStringFunc(value, func(match string) string {
		name := pattern.FindStringSubmatch(match)[1]
		value, ok := values[name]
		if !ok {
			undeclared = append(undeclared, name)
			return match
		}
		return value
	})
	if len(undeclared) > 0 {
		t.Fatalf("the %s block's %s names %s, which that block never declares, so the entry expands to nothing and the file it paints gets no colour",
			shell, table, strings.Join(undeclared, ", "))
	}
	if strings.Contains(expanded, "$") {
		t.Fatalf("the %s block's %s still carries an unexpanded reference after %s were substituted, so the guard cannot read the mapping",
			shell, table, strings.Join(undeclared, ", "))
	}
	return strings.Split(expanded, ":")
}

// TestBothShellsListFilesFromTheSameMapping is stage 2's rule: the file fish paints
// green is the file zsh paints green. The mapping is written once and both renderers
// read it, so a key added for one shell reaches the other or fails here.
func TestBothShellsListFilesFromTheSameMapping(t *testing.T) {
	for _, id := range shippedThemeIDs(t) {
		zsh, fish, _ := renderThemePair(t, id)
		for _, table := range []string{"LS_COLORS", "EZA_COLORS"} {
			fromZsh := listingEntries(t, zsh, table, "zsh")
			fromFish := listingEntries(t, fish, table, "fish")

			if len(fromFish) != len(fromZsh) {
				t.Errorf("theme %s: zsh's %s carries %d entries and fish's carries %d, so the two shells disagree about which names get a colour",
					id, table, len(fromZsh), len(fromFish))
			}
			for i := 0; i < len(fromZsh) && i < len(fromFish); i++ {
				if fromFish[i] != fromZsh[i] {
					t.Errorf("theme %s: entry %d of %s is %q under zsh and %q under fish, so the same file is painted two different colours depending on the shell",
						id, i, table, fromZsh[i], fromFish[i])
				}
			}
		}
	}
}

// TestFishListingTablesPaintTheThemesOwnValues: every colour fish's tables name comes
// from the theme's own palette, so switching theme moves fish's listing with zsh's
// instead of leaving it on the sixteen a tool ships with.
func TestFishListingTablesPaintTheThemesOwnValues(t *testing.T) {
	sequence := regexp.MustCompile(`(?:38|48);2;\d+;\d+;\d+`)
	for _, id := range shippedThemeIDs(t) {
		_, fish, def := renderThemePair(t, id)

		allowed := map[string]bool{}
		for _, role := range themeShellRoles {
			hex := def.Palette[role.Role]
			if hex == "" {
				continue
			}
			sgr, err := themeSGR(hex, false)
			if err != nil {
				t.Fatalf("theme %s: rendering %s as an SGR sequence: %v", id, role.Role, err)
			}
			allowed[sgr] = true
			background, err := themeSGR(hex, true)
			if err != nil {
				t.Fatalf("theme %s: rendering %s as a background sequence: %v", id, role.Role, err)
			}
			allowed[background] = true
		}

		painted := map[string]bool{}
		for _, table := range []string{"LS_COLORS", "EZA_COLORS"} {
			for _, entry := range listingEntries(t, fish, table, "fish") {
				key, value, ok := strings.Cut(entry, "=")
				if !ok {
					t.Errorf("theme %s: the fish %s carries %q, which is not a name and a colour", id, table, entry)
					continue
				}
				found := sequence.FindAllString(value, -1)
				// rs=0 is the one entry that names no colour on purpose: 0 is SGR's reset,
				// so that entry says "leave this as the terminal has it" instead of naming
				// a palette role. Any other entry without a palette colour is a hole.
				if len(found) == 0 && value != "0" {
					t.Errorf("theme %s: %s paints %q in no colour of the palette, so that entry keeps whatever the tool defaults to",
						id, key, value)
				}
				for _, one := range found {
					painted[one] = true
					if !allowed[one] {
						t.Errorf("theme %s: %s is painted %q, which is not a value in themes/%s.toml, so fish paints a colour the theme never chose",
							id, key, one, id)
					}
				}
			}
		}
		if len(painted) < 4 {
			t.Errorf("theme %s: the fish tables paint %d distinct palette colours, so this guard is passing over a listing that is nearly uncoloured",
				id, len(painted))
		}
	}
}

// withoutComments drops a block's comment lines, so the block may name the other
// shell's spelling while explaining itself without this guard reading that prose as
// code fish would run.
func withoutComments(block string) string {
	var b strings.Builder
	for _, line := range strings.Split(block, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

// TestTheFishBlockSpeaksFish: the block is fish. zsh's spelling is not fish's, and a
// block that carries it fails on the line that holds it, which takes the rest of the
// block with it. The variables have to be declared before the value that reads them,
// because fish expands the reference when it sets the variable.
func TestTheFishBlockSpeaksFish(t *testing.T) {
	for _, id := range shippedThemeIDs(t) {
		_, fish, _ := renderThemePair(t, id)
		code := withoutComments(fish)

		if count := strings.Count(code, "${"); count > 0 {
			t.Errorf("theme %s: the fish block carries %d braced variable references, which zsh's parser writes and fish reads as a literal",
				id, count)
		}
		if count := strings.Count(code, "export "); count > 0 {
			t.Errorf("theme %s: the fish block carries %d zsh exports, which fish has no builtin for, so every line holding one fails",
				id, count)
		}

		for _, table := range []string{"LS_COLORS", "EZA_COLORS"} {
			declaration := `set -gx ` + table + ` "`
			at := strings.Index(code, declaration)
			if at < 0 {
				t.Errorf("theme %s: the fish block declares no %s, so fish is given no mapping to read", id, table)
				continue
			}
			value := code[at:]
			if end := strings.Index(value, `"`); end >= 0 {
				value = value[:end]
			}
			for _, reference := range regexp.MustCompile(`\$([A-Z0-9_]+)`).FindAllStringSubmatch(value, -1) {
				name := reference[1]
				declared := strings.Index(code, "set -g "+name+" ")
				if declared < 0 {
					t.Errorf("theme %s: %s reads $%s, which the block never declares, so the entry expands to nothing",
						id, table, name)
					continue
				}
				if declared > at {
					t.Errorf("theme %s: %s reads $%s before the block declares it, so fish expands an empty value and that entry loses its colour",
						id, table, name)
				}
			}
		}
	}
}
