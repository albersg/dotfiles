package tui

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
	"github.com/albersg/dotfiles/installer/internal/tui/trainer"
)

// The three guards below all read prose from the repository and compare it with
// the code that is supposed to make it true. They exist because each sentence
// they cover was once false in exactly the way a copy of a number or a list goes
// false: it was written down once and the code moved on.

// TestReadmeLessonCountMatchesTheCorpus covers the lesson count in the Vim
// Mastery Trainer section. The README stated a range by hand and the corpus was
// counted by hand, so the two could only agree on the day they were written. The
// counts are read from trainer.GetLessons - the same accessor the installer uses
// to fill a module's lesson total - and the README's range has to be exactly the
// smallest and largest of them.
//
// The number of lessons is not the number of exercise IDs in the corpus: each
// module also has a boss fight with its own steps, and the boss is not a lesson.
// A reader who counts every ID gets a range about six higher than the trainer
// ever shows, which is how this sentence came to be doubted.
func TestReadmeLessonCountMatchesTheCorpus(t *testing.T) {
	readme := readRepoFile(t, "README.md")

	modules := trainer.GetAllModules()
	if len(modules) == 0 {
		t.Fatal("trainer.GetAllModules returned no modules, so this guard proves nothing")
	}

	counts := make(map[string]int, len(modules))
	low, high := 0, 0
	for i, module := range modules {
		count := len(trainer.GetLessons(module.ID))
		if count == 0 {
			t.Errorf("trainer.GetLessons(%q) returned no lessons", module.ID)
		}
		counts[module.Name] = count
		if i == 0 || count < low {
			low = count
		}
		if i == 0 || count > high {
			high = count
		}
	}
	if low == 0 {
		t.Fatal("no module holds a lesson, so this guard proves nothing")
	}
	t.Logf("lessons per module, printed so the README's range comes from here: %v", counts)

	statedLow, statedHigh, ok := statedLessonRange(readme)
	if !ok {
		t.Fatalf("README.md no longer states a lesson range (\"Each module has N-M progressive lessons\"); "+
			"the corpus holds %d-%d, so either restore the sentence or move this guard with it", low, high)
	}
	if statedLow != low || statedHigh != high {
		t.Errorf("README.md says each module has %d-%d progressive lessons, the corpus holds %d-%d per module: %v\n"+
			"Change the README to the numbers this test printed, not the other way around.",
			statedLow, statedHigh, low, high, counts)
	}
}

var statedLessonRangeRE = regexp.MustCompile(`Each module has (\d+)[–-](\d+) progressive lessons`)

// statedLessonRange reads the endpoints out of the README's lesson sentence.
func statedLessonRange(readme string) (int, int, bool) {
	match := statedLessonRangeRE.FindStringSubmatch(readme)
	if match == nil {
		return 0, 0, false
	}
	low, errLow := strconv.Atoi(match[1])
	high, errHigh := strconv.Atoi(match[2])
	if errLow != nil || errHigh != nil {
		return 0, 0, false
	}
	return low, high, true
}

// TestReadmeDoesNotPromiseHomebrewWhereThePlanSkipsIt covers the Homebrew
// requirement row and the Supported Platforms table. The plan deliberately
// excludes Arch and Fedora from the Homebrew step - each keeps its native
// package manager - and Termux has neither, so the README has to name all three
// in the row that says where Homebrew is installed automatically, and the
// platform table has to name the native manager on the rows for the
// distributions that keep one.
//
// The skipped set and the manager names are derived here. The set comes from the
// same pure planFor the wizard runs, and the manager name comes from the same
// planPlatformInstall that installs the packages, so a change to either moves
// this test instead of leaving the README to lie about it.
func TestReadmeDoesNotPromiseHomebrewWhereThePlanSkipsIt(t *testing.T) {
	readme := readRepoFile(t, "README.md")

	// Every host kind the plan can detect, paired with the name the README uses
	// for it. A host not listed here is not detected by the installer.
	hostNames := map[system.OSType]string{
		system.OSArch:   "Arch",
		system.OSFedora: "Fedora",
		system.OSTermux: "Termux",
	}

	var skipped []string
	for osType, name := range hostNames {
		opts := planOptions{
			OS:             "linux",
			DetectedOS:     osType,
			DetectedTermux: osType == system.OSTermux,
			HasBrew:        false,
		}
		if planHasStep(planFor(opts), "homebrew") {
			t.Errorf("the plan now installs Homebrew on %s, so the README's exception for it is stale; "+
				"either the plan or the README has to move", name)
			continue
		}
		skipped = append(skipped, name)
	}
	if len(skipped) == 0 {
		t.Fatal("the plan skips Homebrew nowhere, so this guard proves nothing")
	}

	requirementRow, ok := markdownRowStartingWith(readme, "| **Homebrew** |")
	if !ok {
		t.Fatal("README.md no longer has the Homebrew requirement row, so this guard proves nothing")
	}
	for _, name := range skipped {
		if !strings.Contains(requirementRow, name) {
			t.Errorf("the plan does not install Homebrew on %s, but the README's requirement row does not say so: %q\n"+
				"A user on that host waits for a step that will never come.", name, requirementRow)
		}
	}

	platforms := markdownSection(readme, "## Supported Platforms")
	if platforms == "" {
		t.Fatal("README.md no longer has the Supported Platforms section, so this guard proves nothing")
	}

	// Arch and Fedora keep a native manager, so their rows have to name the
	// manager the install would really run. The name is read from the plan that
	// installs, never typed here.
	nativeRows := []struct {
		osType system.OSType
		name   string
		prefix string
	}{
		{system.OSArch, "Arch", "| Linux (Arch)"},
		{system.OSFedora, "Fedora", "| Linux (Fedora/RHEL)"},
	}
	for _, row := range nativeRows {
		manager := planPlatformInstall(
			&Model{SystemInfo: &system.SystemInfo{OS: row.osType, HasBrew: false}},
			platformPackages{Brew: "fish", Arch: "fish", Fedora: "fish", Debian: "fish"},
		).Manager
		if manager == "" || manager == "brew" {
			t.Errorf("no native package manager is planned for %s, so the README cannot name one", row.name)
			continue
		}
		line, ok := markdownRowStartingWith(platforms, row.prefix)
		if !ok {
			t.Errorf("the Supported Platforms table has no row for %s", row.name)
			continue
		}
		if !strings.Contains(line, manager) {
			t.Errorf("the plan installs through %s on %s, but that row names no such manager: %q",
				manager, row.name, line)
		}
		if strings.Contains(line, "Homebrew") {
			t.Errorf("the plan never installs Homebrew on %s, but the Supported Platforms row still offers it: %q",
				row.name, line)
		}
	}
}

// TestDocsMarkMacOnlyTerminals covers the --terminal value list in both places
// it is written down. kitty is the one value supportedTerminals accepts on
// macOS and refuses everywhere else, and both documents listed it as a general
// choice, so a Linux user reading either one is offered a terminal the binary
// then refuses.
//
// The restricted set is the difference between the two platform lists the
// binary itself uses, so a second restricted terminal fails this test rather
// than being missed.
func TestDocsMarkMacOnlyTerminals(t *testing.T) {
	var restricted []string
	for _, terminal := range SupportedTerminals("darwin") {
		if terminal == "none" {
			continue
		}
		if !terminalListContains(SupportedTerminals("linux"), terminal) {
			restricted = append(restricted, terminal)
		}
	}
	if len(restricted) == 0 {
		t.Fatal("no terminal is restricted to macOS, so this guard proves nothing")
	}
	t.Logf("terminals the binary only installs on macOS: %v", restricted)

	readme := readRepoFile(t, "README.md")
	readmeParagraph, ok := paragraphContaining(readme, "`--terminal`")
	if !ok {
		t.Fatal("README.md no longer documents --terminal, so this guard proves nothing")
	}
	tuiDoc := readRepoFile(t, filepath.Join("docs", "tui-installer.md"))
	tuiRow, ok := markdownRowStartingWith(tuiDoc, "| `--terminal`")
	if !ok {
		t.Fatal("docs/tui-installer.md no longer documents --terminal, so this guard proves nothing")
	}

	for _, terminal := range restricted {
		if !strings.Contains(readmeParagraph, terminal) || !strings.Contains(readmeParagraph, "macOS") {
			t.Errorf("README.md's --terminal list names %s without saying it is macOS only:\n%s", terminal, readmeParagraph)
		}
		if !strings.Contains(tuiRow, terminal) || !strings.Contains(tuiRow, "macOS") {
			t.Errorf("docs/tui-installer.md's --terminal row names %s without saying it is macOS only: %q", terminal, tuiRow)
		}
	}
}

// readRepoFile reads a file from the repository checkout the package lives in.
func readRepoFile(t *testing.T, name string) string {
	t.Helper()

	path := filepath.Join(repoRoot(t), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// markdownRowStartingWith returns the first line that begins with prefix, which
// is how one row is picked out of a markdown table.
func markdownRowStartingWith(text, prefix string) (string, bool) {
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, prefix) {
			return line, true
		}
	}
	return "", false
}

// markdownSection returns the text of the section whose heading is exactly
// heading, up to the next heading of the same level.
func markdownSection(text, heading string) string {
	start := strings.Index(text, heading)
	if start == -1 {
		return ""
	}
	section := text[start:]
	if end := strings.Index(section[len(heading):], "\n## "); end >= 0 {
		section = section[:len(heading)+end]
	}
	return section
}

// paragraphContaining returns the blank-line-delimited paragraph holding want.
func paragraphContaining(text, want string) (string, bool) {
	for _, paragraph := range strings.Split(text, "\n\n") {
		if strings.Contains(paragraph, want) {
			return paragraph, true
		}
	}
	return "", false
}

// planHasStep reports whether a plan holds a step with the given ID.
func planHasStep(steps []InstallStep, id string) bool {
	for _, step := range steps {
		if step.ID == id {
			return true
		}
	}
	return false
}
