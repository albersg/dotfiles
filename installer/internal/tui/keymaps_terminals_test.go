package tui

import (
	"strings"
	"testing"
)

// kittyMapLines reads the `map <keys> <action>` lines of
// dotfiles-kitty/kitty.conf, which is what the Kitty reference claims to copy.
func kittyMapLines(t *testing.T) map[string]string {
	t.Helper()

	lines := map[string]string{}
	for _, line := range strings.Split(learnRepoFile(t, "dotfiles-kitty", "kitty.conf"), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[0] != "map" {
			continue
		}
		lines[fields[1]] = strings.Join(fields[2:], " ")
	}
	return lines
}

// TestKittyKeymapsMatchTheConfiguration holds the Kitty table to the file it
// copies, in both directions: a `map` line with no row is a binding the reference
// hides, and a row with no `map` line is a binding the reference invent.
func TestKittyKeymapsMatchTheConfiguration(t *testing.T) {
	mapped := kittyMapLines(t)
	if len(mapped) == 0 {
		t.Fatal("dotfiles-kitty/kitty.conf declares no map line, so this guard proves nothing")
	}

	rows := map[string]bool{}
	for _, category := range GetKittyKeymaps() {
		for _, km := range category.Keymaps {
			rows[strings.ToLower(strings.TrimSpace(km.Keys))] = true
		}
	}

	for keys, action := range mapped {
		if !rows[keys] {
			t.Errorf("kitty.conf maps %s (%s) and the Kitty reference has no row for it", keys, action)
		}
	}
	for keys := range rows {
		if _, ok := mapped[keys]; !ok {
			t.Errorf("the Kitty reference lists %s and kitty.conf never maps it", keys)
		}
	}
}

// TestWezTermKeymapPageDeclaresWhatTheConfigurationDoes holds the WezTerm page to
// the two things that would make it wrong: a configuration that has grown a key
// binding, and a row naming a setting the file does not carry. The page's whole
// answer is that this configuration binds nothing, so it has to stay that way
// until someone rewrites the page.
func TestWezTermKeymapPageDeclaresWhatTheConfigurationDoes(t *testing.T) {
	config := learnRepoFile(t, ".wezterm.lua")
	table := weztermConfigTable()

	assertConfigurationDeclaresNoBindings(t, "WezTerm", config, []string{"config.keys", "key =", "key_table"})
	assertEveryRowNamesASetting(t, table, config, "config.")
	assertPageSaysTheConfigurationBindsNothing(t, table)
}

// TestAlacrittyKeymapPageDeclaresWhatTheConfigurationDoes is the Alacritty twin
// of the WezTerm guard: alacritty.toml carries no [keyboard] section, and the page
// answers the reference's question with what the file does set instead.
func TestAlacrittyKeymapPageDeclaresWhatTheConfigurationDoes(t *testing.T) {
	config := learnRepoFile(t, "alacritty.toml")
	table := alacrittyConfigTable()

	assertConfigurationDeclaresNoBindings(t, "Alacritty", config, []string{"[keyboard]", "bindings", "key_bindings"})
	assertEveryRowNamesASetting(t, table, config, "")
	assertPageSaysTheConfigurationBindsNothing(t, table)
}

// assertConfigurationDeclaresNoBindings fails on the markers that would mean the
// configuration binds keys after all.
func assertConfigurationDeclaresNoBindings(t *testing.T, tool, config string, markers []string) {
	t.Helper()

	for _, marker := range markers {
		if strings.Contains(config, marker) {
			t.Errorf("%s's configuration carries %q, so the page's \"declares no key bindings\" is no longer true",
				tool, marker)
		}
	}
}

// assertEveryRowNamesASetting reads each row back against the configuration, with
// the prefix the file's own settings carry (config. for the Lua, none for the
// TOML).
func assertEveryRowNamesASetting(t *testing.T, table referenceTable, config, prefix string) {
	t.Helper()

	for _, row := range table.Rows {
		if !strings.Contains(config, prefix+row.Name) {
			t.Errorf("the %s page has a %s row and the configuration never sets %s%s",
				table.Name, row.Name, prefix, row.Name)
		}
		if row.Kind == "" || row.What == "" {
			t.Errorf("the %s page's %s row says nothing about the section it belongs to or what it sets",
				table.Name, row.Name)
		}
	}
}

// assertPageSaysTheConfigurationBindsNothing requires the page to state its
// answer: a reference menu row that leads to a settings table without saying why
// reads as a missing keymap table rather than as the answer.
func assertPageSaysTheConfigurationBindsNothing(t *testing.T, table referenceTable) {
	t.Helper()

	if !strings.Contains(strings.ToLower(table.Description), "no key bindings") {
		t.Errorf("the %s page does not say that the configuration declares no key bindings, which is its whole answer",
			table.Name)
	}
}

// TestTheKeymapReferenceOffersEveryTerminalItDocuments reads the menu's own
// options and requires a row for every terminal the learn pages document: a
// terminal the wiki explains and the reference cannot show is the gap this guard
// exists to keep closed.
func TestTheKeymapReferenceOffersEveryTerminalItDocuments(t *testing.T) {
	m := NewModel()
	m.Screen = ScreenKeymapsMenu

	offered := map[string]bool{}
	for _, option := range m.GetCurrentOptions() {
		offered[strings.ToLower(option)] = true
	}

	terminals := GetTerminalInfo()
	checked := 0
	for name := range terminals {
		checked++
		if !offered[name] {
			t.Errorf("the learn pages document %s and the keymap reference menu does not offer it", name)
		}
	}
	if checked == 0 {
		t.Fatal("the learn pages document no terminal, so this guard proves nothing")
	}
}
