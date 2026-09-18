package tui

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const brewfileName = "Brewfile"

// brewfileEntryKinds lists the directives brew bundle understands and that this
// repository actually uses. An unknown kind is a typo that brew bundle would
// either ignore or reject on a fresh machine.
var brewfileEntryKinds = map[string]bool{
	"brew":   true,
	"tap":    true,
	"cask":   true,
	"mas":    true,
	"go":     true,
	"npm":    true,
	"uv":     true,
	"vscode": true,
	"winget": true,
}

// brewfileEntry matches a directive line, optionally guarded by a platform
// condition, for example: brew "spotify_player" if OS.linux?
var brewfileEntry = regexp.MustCompile(`^([a-z]+)\s+"([^"]+)"(\s+if\s+OS\.(?:linux|mac)\?)?$`)

// TestBrewfileIsWellFormed parses the machine toolset inventory. The file is
// hand-edited whenever a tool is added to the machine, and a duplicated or
// mistyped entry only surfaces when someone runs brew bundle on a fresh
// machine, far from the edit. This test moves that failure into CI.
func TestBrewfileIsWellFormed(t *testing.T) {
	path := filepath.Join(repoRoot(t), brewfileName)

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("cannot read %s: %v", brewfileName, err)
	}
	defer file.Close()

	// Keyed by kind and name so a formula repeated under two kinds is reported,
	// which is how a copy-paste between sections shows up.
	seen := map[string]int{}
	entries := 0

	scanner := bufio.NewScanner(file)
	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimSpace(scanner.Text())

		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		// Platform guards stay balanced: they wrap entries, they do not carry them.
		if text == "end" || strings.HasPrefix(text, "if OS.") {
			continue
		}

		match := brewfileEntry.FindStringSubmatch(text)
		if match == nil {
			t.Errorf("%s:%d: not a directive brew bundle understands: %q", brewfileName, line, text)
			continue
		}

		kind, name := match[1], match[2]
		if !brewfileEntryKinds[kind] {
			t.Errorf("%s:%d: unknown directive %q", brewfileName, line, kind)
			continue
		}

		key := kind + " " + name
		if first, ok := seen[key]; ok {
			t.Errorf("%s:%d: %s appears twice, already declared on line %d", brewfileName, line, key, first)
			continue
		}
		seen[key] = line
		entries++
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("cannot read %s: %v", brewfileName, err)
	}

	// A truncated or emptied file still parses as valid, so pin the scale. The
	// inventory describes a full workstation; a handful of entries means the
	// file lost content rather than being genuinely small.
	if entries < 50 {
		t.Errorf("%s declares only %d entries, which is too few for a full machine inventory", brewfileName, entries)
	}
}
