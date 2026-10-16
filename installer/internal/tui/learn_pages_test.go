package tui

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestFontLearnPageNamesTheFamilyTheStepInstalls holds the font page to the
// step's own decision. fontInstallFamily is what the run puts on the machine and
// fontInstallLabel is the phrase the wizard already uses, so a page that named a
// different face would be offering a font no step installs.
func TestFontLearnPageNamesTheFamilyTheStepInstalls(t *testing.T) {
	info := GetFontInfo()
	text := info.Name + "\n" + info.Description + "\n" + strings.Join(info.Pros, "\n") + "\n" + strings.Join(info.Cons, "\n")

	for _, termux := range []bool{false, true} {
		label := fontInstallLabel(fontInstallFamily(termux))
		if !strings.Contains(text, label) {
			t.Errorf("the font page never names %q, the face the step installs for termux=%v", label, termux)
		}
	}
	if !strings.Contains(text, "Nerd Font") {
		t.Error("the font page does not say that the family is a Nerd Font, which is why the icons render")
	}
}

// TestPromptLearnPagesCoverThePromptsTheShellsStart derives the prompts from the
// shipped shell configurations and requires a page for each, naming the shell it
// belongs to. A prompt the run starts and the wiki never explains is the gap this
// guard exists to keep closed.
func TestPromptLearnPagesCoverThePromptsTheShellsStart(t *testing.T) {
	cases := []struct {
		shell, file, marker, key string
	}{
		{"fish", filepath.Join("dotfiles-fish", "fish", "config.fish"), "starship init fish", "starship"},
		{"nushell", filepath.Join("dotfiles-nushell", "env.nu"), "starship init nu", "starship"},
		{"zsh", filepath.Join("dotfiles-zsh", ".zshrc"), "powerlevel10k.zsh-theme", "powerlevel10k"},
	}

	prompts := GetPromptInfo()
	checked := 0
	for _, c := range cases {
		if !strings.Contains(learnRepoFile(t, strings.Split(c.file, string(filepath.Separator))...), c.marker) {
			continue
		}
		checked++
		info, ok := prompts[c.key]
		if !ok {
			t.Errorf("the %s configuration starts %s (%q) and the learn menu has no page for it", c.shell, c.key, c.marker)
			continue
		}
		text := info.Name + "\n" + info.Description + "\n" + strings.Join(info.Pros, "\n") + "\n" + strings.Join(info.Cons, "\n")
		if !strings.Contains(strings.ToLower(text), c.shell) {
			t.Errorf("the %s page never names %s, the shell whose configuration starts it", c.key, c.shell)
		}
	}
	if checked == 0 {
		t.Fatal("no shipped shell configuration starts a prompt, so this guard proves nothing")
	}
}

// referenceNames is the set of names a reference page carries, for comparing a
// page with its source.
func referenceNames(rows []referenceRow) map[string]bool {
	names := map[string]bool{}
	for _, row := range rows {
		names[row.Name] = true
	}
	return names
}

// TestShellToolingPageMatchesTheDocument holds the shell tooling page to the
// table docs/tools.md keeps of the same tools. A tool the document names and the
// page does not is the gap this guard exists to keep closed, and a row the page
// carries alone is the drift the other direction.
func TestShellToolingPageMatchesTheDocument(t *testing.T) {
	doc := learnRepoFile(t, "docs", "tools.md")
	documented := map[string]bool{}
	for _, row := range markdownTableRowsAfter(t, doc, "## Shell Tooling") {
		documented[row[0]] = true
	}
	if len(documented) == 0 {
		t.Fatal("docs/tools.md lists no shell tooling, so this guard proves nothing")
	}

	page := referenceNames(shellToolingRows())
	for tool := range documented {
		if !page[tool] {
			t.Errorf("docs/tools.md documents %s and the shell tooling page has no row for it", tool)
		}
	}
	for tool := range page {
		if !documented[tool] {
			t.Errorf("the shell tooling page lists %s and docs/tools.md does not, so one of the two is out of date", tool)
		}
	}
}

// TestPluginPagesCoverTheSetsTheInstallerProvisions derives the plugin sets from
// the files and values that declare them, the way the run reads them, and
// requires a row for each. The page is a statement about what a run provisions;
// this is the statement's evidence.
func TestPluginPagesCoverTheSetsTheInstallerProvisions(t *testing.T) {
	page := referenceNames(pluginSetRows())

	checked := 0
	cases := []struct{ tool, file, marker string }{
		{"Tmux", filepath.Join("dotfiles-tmux", "tmux.conf"), "tpm"},
		{"Fish", filepath.Join("dotfiles-fish", "fish", "fish_plugins"), "jorgebucaran/fisher"},
		{"Zsh", filepath.Join("dotfiles-zsh", ".zshrc"), "zsh-autosuggestions"},
		{"Neovim", filepath.Join("dotfiles-nvim", "nvim", "lua", "config", "lazy.lua"), "lazy.nvim"},
	}
	for _, c := range cases {
		content := learnRepoFile(t, strings.Split(c.file, string(filepath.Separator))...)
		if !strings.Contains(strings.ToLower(content), strings.ToLower(c.marker)) {
			continue
		}
		checked++
		if !page[c.tool] {
			t.Errorf("%s declares its plugin set (%q in %s) and the plugins page has no row for it", c.tool, c.marker, c.file)
		}
	}

	// Herdr's set is the one declared in Go rather than in a file.
	if len(herdrPluginSet) == 0 {
		t.Error("the installer declares no Herdr plugin set, so the plugins page's Herdr row describes a run that no longer happens")
	} else {
		checked++
		if !page["Herdr"] {
			t.Errorf("the installer declares %d Herdr plugins and the plugins page has no row for them", len(herdrPluginSet))
		}
	}

	if checked == 0 {
		t.Fatal("no shipped configuration declares a plugin set, so this guard proves nothing")
	}
}

// TestMachineToolsetPageMatchesTheBrewfileAndTheFilter reads the page's rows
// against the two things that decide what a run installs: the directives the
// Brewfile actually declares, and filterBrewfile, which is run over each row's
// directive so the page cannot claim a line is installed while the step drops it.
func TestMachineToolsetPageMatchesTheBrewfileAndTheFilter(t *testing.T) {
	rows := toolsetDirectives()
	byDirective := map[string]toolsetDirective{}
	for _, d := range rows {
		byDirective[d.Directive] = d
	}

	declared := map[string]int{}
	for _, line := range strings.Split(learnRepoFile(t, "Brewfile"), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || !brewfileEntryLine.MatchString(line) {
			continue
		}
		declared[strings.Fields(line)[0]]++
	}
	if len(declared) == 0 {
		t.Fatal("the Brewfile declares no entry, so this guard proves nothing")
	}
	for directive, lines := range declared {
		d, ok := byDirective[directive]
		if !ok {
			t.Errorf("the Brewfile declares %d %s lines and the toolset page has no row for them", lines, directive)
			continue
		}
		if d.Dropped {
			// A dropped directive that the file declares is the case the row
			// documents, not a mismatch: vscode and winget lines are in the
			// Brewfile and the step leaves them behind on purpose. What the run
			// does with them is checked against filterBrewfile below.
			continue
		}
	}

	for _, d := range rows {
		_, dropped := filterBrewfile(d.Directive + " \"entry\"\n")
		if dropped[d.Directive] > 0 != d.Dropped {
			t.Errorf("the toolset page says %s lines are dropped=%v, and filterBrewfile says dropped=%v",
				d.Directive, d.Dropped, dropped[d.Directive] > 0)
		}
		if d.Installs == "" {
			t.Errorf("the toolset page's %s row never says what the directive installs", d.Directive)
		}
	}
}
