package tui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The installer's Learn screens are a second keymap document, and the repository
// has already paid once for one that drifted: this repository's Neovim keymap
// table kept listing the Copilot and OpenCode bindings after
// lua/plugins/disabled.lua turned those plugins off, and docs/ai-configuration.md
// had to say so in words -- "Any documentation or keymap table that describes
// Copilot completions, an OpenCode keymap group, or a Claude Code default
// describes a configuration that does not exist".
//
// These guards derive the truth from the files the configuration ships, so the
// installer's copy of that content cannot drift again without a test naming it.

// learnRepoFile reads a file from the repository checkout the installer ships
// from, so a guard compares the screens against the same bytes the run uses.
func learnRepoFile(t *testing.T, parts ...string) string {
	t.Helper()

	path := filepath.Join(append([]string{repoRoot(t)}, parts...)...)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// markdownTableRowsAfter returns the rows of the first table under a heading, as
// cells with the markdown decoration stripped. The header row and the rule under
// it are left out, so the result is only what a reader would read as an entry.
func markdownTableRowsAfter(t *testing.T, doc, heading string) [][]string {
	t.Helper()

	start := strings.Index(doc, heading)
	if start < 0 {
		t.Fatalf("the document does not carry the heading %q, so this guard proves nothing", heading)
	}
	section := doc[start:]
	if end := strings.Index(section[len(heading):], "\n## "); end >= 0 {
		section = section[:len(heading)+end]
	}

	var rows [][]string
	headerSeen := false
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		for i, cell := range cells {
			cells[i] = strings.Trim(strings.TrimSpace(cell), "*`")
		}
		// The rule under the header is not a row.
		rule := true
		for _, cell := range cells {
			if strings.Trim(cell, "-: ") != "" {
				rule = false
			}
		}
		if rule {
			continue
		}
		// The first row of a section's table is its header, whatever it names, so
		// the caller compares data rows and not a column title.
		if !headerSeen {
			headerSeen = true
			continue
		}
		rows = append(rows, cells)
	}
	return rows
}

// TestLearnAIKeymapsMatchTheActiveAssistant is the parity guard for the group
// that drifted. docs/ai-configuration.md is the repository's statement of which
// assistant is live and with which binding, so the installer's AI group is read
// as a copy of that table: every row it lists must be one the document lists,
// and every row the document lists must be there.
func TestLearnAIKeymapsMatchTheActiveAssistant(t *testing.T) {
	doc := learnRepoFile(t, "docs", "ai-configuration.md")
	want := markdownTableRowsAfter(t, doc, "## The Active Assistant")
	if len(want) == 0 {
		t.Fatal("docs/ai-configuration.md lists no active assistant binding, so this guard proves nothing")
	}

	var got []Keymap
	categories := 0
	for _, category := range GetNvimKeymaps() {
		if strings.Contains(category.Name, "AI") {
			categories++
			got = append(got, category.Keymaps...)
		}
	}
	if categories != 1 {
		t.Fatalf("the keymap reference carries %d AI categories, want exactly one", categories)
	}

	strip := func(s string) string { return strings.TrimSpace(strings.ReplaceAll(s, "`", "")) }

	if len(got) != len(want) {
		t.Errorf("the AI keymap group lists %d bindings and docs/ai-configuration.md lists %d:\nthe reference must describe the assistant that is live, not the one the configuration disabled",
			len(got), len(want))
	}
	for i := 0; i < len(got) && i < len(want); i++ {
		if strip(got[i].Keys) != strip(want[i][0]) {
			t.Errorf("AI binding %d: reference says %q, the document says %q", i, got[i].Keys, want[i][0])
		}
		if strip(got[i].Description) != strip(want[i][1]) {
			t.Errorf("AI binding %d (%s): reference says %q, the document says %q",
				i, got[i].Keys, got[i].Description, want[i][1])
		}
		if strip(got[i].Mode) != strip(want[i][2]) {
			t.Errorf("AI binding %d (%s): reference mode %q, the document mode %q",
				i, got[i].Keys, got[i].Mode, want[i][2])
		}
	}
}

// TestLearnNvimInfoNeverOffersADisabledAssistant holds the prose to the same
// rule as the tables. A plugin the configuration disables may still be named --
// as a thing you could turn on -- but never as something the configuration
// gives you, so any line in Neovim's learn entry that names one must say so.
func TestLearnNvimInfoNeverOffersADisabledAssistant(t *testing.T) {
	doc := learnRepoFile(t, "docs", "ai-configuration.md")
	disabled := markdownTableRowsAfter(t, doc, "## Disabled AI Plugins")
	if len(disabled) == 0 {
		t.Fatal("docs/ai-configuration.md lists no disabled plugin, so this guard proves nothing")
	}

	names := make([]string, 0, len(disabled))
	for _, row := range disabled {
		name := strings.TrimSpace(row[0])
		name = strings.TrimSuffix(name, ".nvim")
		name = strings.TrimSuffix(name, ".lua")
		if name != "" {
			names = append(names, name)
		}
	}

	info := GetNvimInfo()
	lines := append([]string{info.Name, info.Description}, info.Pros...)
	lines = append(lines, info.Cons...)

	for _, line := range lines {
		for _, name := range names {
			if strings.Contains(strings.ToLower(line), strings.ToLower(name)) &&
				!strings.Contains(strings.ToLower(line), "disabled") {
				t.Errorf("the Neovim learn entry offers %q as available: %q\ndocs/ai-configuration.md disables %s, so the line must say so",
					name, line, name)
			}
		}
	}
}

// TestLearnPickerMentionsMatchTheShippedPicker keeps the content's picker
// vocabulary honest. The shipped configuration selects the snacks picker
// (dotfiles-nvim/nvim/lua/plugins/obsidian.lua), so a tip or an example that
// sends a reader to a different picker sends them to a plugin the run does not
// load.
func TestLearnPickerMentionsMatchTheShippedPicker(t *testing.T) {
	selection := pickerTheConfigurationSelects(t)
	if selection == "" {
		t.Fatal("the shipped configuration no longer names a picker, so this guard proves nothing")
	}
	selected := strings.SplitN(selection, ".", 2)[0]

	content := learnRepoFile(t, "installer", "internal", "tui", "tools_info.go")
	for _, picker := range []string{"telescope", "fzf-lua", "mini.pick", "snacks"} {
		if strings.Contains(strings.ToLower(content), picker) && !strings.EqualFold(picker, selected) {
			t.Errorf("the learn content names the %s picker while the configuration selects %q; "+
				"a reader sent to a plugin this run does not load", picker, selection)
		}
	}
}

// pickerTheConfigurationSelects reads the picker the shipped Neovim configuration
// chooses. It is the `name = "<picker>"` line that names a picker among the ones
// the file itself lists, so an unrelated workspace name cannot be mistaken for it.
func pickerTheConfigurationSelects(t *testing.T) string {
	t.Helper()

	for _, match := range regexp.MustCompile(`name = "([^"]+)"`).
		FindAllStringSubmatch(learnRepoFile(t, "dotfiles-nvim", "nvim", "lua", "plugins", "obsidian.lua"), -1) {
		if strings.Contains(match[1], "picker") {
			return match[1]
		}
	}
	return ""
}

// TestTmuxLearnEntryMatchesTheShippedMouseSetting is the same rule for a plain
// configuration fact: dotfiles-tmux/tmux.conf turns mouse support on, so the
// entry may not tell the reader that Tmux has no mouse support or that it needs
// configuration this repository already ships.
func TestTmuxLearnEntryMatchesTheShippedMouseSetting(t *testing.T) {
	conf := learnRepoFile(t, "dotfiles-tmux", "tmux.conf")
	if !regexp.MustCompile(`(?m)^\s*set\s+-g\s+mouse\s+on\s*$`).MatchString(conf) {
		t.Fatal("dotfiles-tmux/tmux.conf no longer sets mouse on, so this guard proves nothing")
	}

	info, ok := GetWMInfo()["tmux"]
	if !ok {
		t.Fatal("the multiplexer table carries no tmux entry, so this guard proves nothing")
	}

	stated := false
	rows := append(append([]string{}, info.Pros...), info.Cons...)
	for _, row := range rows {
		lower := strings.ToLower(row)
		if !strings.Contains(lower, "mouse") {
			continue
		}
		if strings.Contains(lower, "no native") || strings.Contains(lower, "needs config") {
			t.Errorf("the Tmux entry says %q, but dotfiles-tmux/tmux.conf sets `set -g mouse on`", row)
		}
		if strings.Contains(lower, "mouse on") {
			stated = true
		}
	}
	if !stated {
		t.Error("the Tmux entry never names the `set -g mouse on` the shipped configuration already sets")
	}
}
