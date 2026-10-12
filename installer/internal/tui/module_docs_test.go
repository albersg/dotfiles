package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	"github.com/albersg/dotfiles/installer/internal/tui/trainer"
)

func TestReadmeDocumentsEveryModule(t *testing.T) {
	readme, err := os.ReadFile(filepath.Join(repoRoot(t), "README.md"))
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	modules := trainer.GetAllModules()
	if len(modules) == 0 {
		t.Fatal("trainer.GetAllModules returned no modules")
	}

	readmeText := string(readme)
	knownNames := make(map[string]bool, len(modules))
	for _, module := range modules {
		knownNames[module.Name] = true
		if !strings.Contains(readmeText, module.Name) {
			t.Errorf("README.md does not mention trainer module %q", module.Name)
		}
	}

	sectionStart := strings.Index(readmeText, "## 🎮 Vim Mastery Trainer")
	if sectionStart == -1 {
		t.Fatal("README.md is missing the Vim Mastery Trainer section")
	}
	section := readmeText[sectionStart:]
	if sectionEnd := strings.Index(section, "\n## "); sectionEnd >= 0 {
		section = section[:sectionEnd]
	}

	rowNames := make(map[string]bool)
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := strings.Split(line, "|")
		if len(cells) < 3 {
			continue
		}
		firstCell := strings.TrimSpace(cells[1])
		fields := strings.Fields(firstCell)
		if len(fields) < 2 || fields[0] == "Module" || strings.Trim(fields[0], "-:") == "" {
			continue
		}
		nameStart := strings.IndexFunc(firstCell, unicode.IsSpace)
		if nameStart < 0 {
			continue
		}
		rowNames[strings.TrimSpace(firstCell[nameStart:])] = true
	}

	var missing, stale []string
	for name := range knownNames {
		if !rowNames[name] {
			missing = append(missing, name)
		}
	}
	for name := range rowNames {
		if !knownNames[name] {
			stale = append(stale, name)
		}
	}
	if len(missing) > 0 || len(stale) > 0 {
		t.Errorf("README.md trainer module rows differ from code: missing %v; stale %v", missing, stale)
	}
}

// TestTrainerSkillListsEveryModuleConstant covers the other place the module list
// is written down in prose: the skill a future exercise author reads before
// touching the trainer. It listed seven of the nine constants, and the two
// modules added tonight were missing from the unlock order beside it too, which is
// how a reader comes to believe a module that exists does not - or adds a second
// one next to it.
//
// The names come from the trainer's types.go rather than from this test, so a
// constant added without the skill being updated fails here.
func TestTrainerSkillListsEveryModuleConstant(t *testing.T) {
	names := moduleIDConstantsInDeclarationOrder(t)
	if len(names) == 0 {
		t.Fatal("trainer/types.go declares no ModuleID constants, so this guard proves nothing")
	}

	skill, err := os.ReadFile(filepath.Join(repoRoot(t), "skills", "dotfiles-trainer", "SKILL.md"))
	if err != nil {
		t.Fatalf("read the trainer skill: %v", err)
	}

	var missing []string
	for _, name := range names {
		if !strings.Contains(string(skill), name) {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		t.Errorf("the trainer skill does not mention %d of %d module constants: %v\n"+
			"A reader who trusts it will not know the module exists, and may add a second one beside it.",
			len(missing), len(names), missing)
	}
}

// moduleIDConstantsInDeclarationOrder reads the trainer's types.go and returns the
// names of the ModuleID constants in declaration order.
func moduleIDConstantsInDeclarationOrder(t *testing.T) []string {
	t.Helper()

	path := filepath.Join(repoRoot(t), "installer", "internal", "tui", "trainer", "types.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	var names []string
	ast.Inspect(file, func(n ast.Node) bool {
		decl, ok := n.(*ast.GenDecl)
		if !ok || decl.Tok != token.CONST {
			return true
		}
		for _, spec := range decl.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			id, ok := value.Type.(*ast.Ident)
			if !ok || id.Name != "ModuleID" {
				continue
			}
			for _, name := range value.Names {
				names = append(names, name.Name)
			}
		}
		return true
	})
	return names
}

// TestHerdrPluginStanceMatchesTheInstaller keeps the statement the documentation
// makes about Herdr plugins in agreement with what the installer actually does.
//
// The installer now provisions a plugin set for Herdr too, declared in
// installer.go and installed from GitHub shorthand, so the docs must say the
// installer installs them and no doc may still call them user-managed. That
// claim is only honest while an installer step does invoke the Herdr plugin CLI,
// and this guard derives that fact from the installer's own sources rather than
// repeating it here: a provisioning step removed without the docs being updated,
// or added without them, makes the two disagree and fails this.
//
// The keymap reference (keymaps_*.go) is left out: it lists `herdr plugin` as UI
// text for the user to read, which is the screen naming the command, not the
// installer running it. Comments are invisible to the parser, so a comment that
// merely mentions plugins does not count either.
func TestHerdrPluginStanceMatchesTheInstaller(t *testing.T) {
	provisions := installerInvokesHerdrPlugin(t)

	for _, doc := range []string{"README.md",
		"README.es.md", filepath.Join("docs", "tools.md"), filepath.Join("docs", "tui-installer.md")} {
		data, err := os.ReadFile(filepath.Join(repoRoot(t), doc))
		if err != nil {
			t.Fatalf("read %s: %v", doc, err)
		}
		text := string(data)

		if !strings.Contains(text, "herdr plugin") {
			t.Errorf("%s does not name the `herdr plugin` command, so a user who wants a plugin is not told how", doc)
		}

		userManaged := strings.Contains(text, "user-managed")
		if provisions && userManaged {
			t.Errorf("%s states that Herdr plugins are user-managed, but the installer invokes the Herdr plugin CLI: "+
				"the statement and the step code disagree", doc)
		}
		if !provisions && !userManaged {
			t.Errorf("%s does not state that Herdr plugins are user-managed, and the installer does not invoke the Herdr plugin CLI: "+
				"a reader cannot tell the deliberate omission from a missing step", doc)
		}
	}
}

// installerInvokesHerdrPlugin reports whether any installer step runs a
// `herdr plugin` command. It reads the string literals of the installer's own
// sources through the parser, so the fact comes from the code and not from a
// copy in the test.
//
// TestHerdrPluginSetAndDocumentationAgree is the other half of the agreement:
// the declared set in installer.go and the prose in docs/tools.md name the same
// plugins. Adding a plugin to herdrPluginSet without documenting it fails there,
// and emptying the set fails the non-empty guard, so a guard that checks nothing
// cannot pass.
func installerInvokesHerdrPlugin(t *testing.T) bool {
	t.Helper()

	dir := filepath.Join(repoRoot(t), "installer", "internal", "tui")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}

	found := false
	scanned := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		// The keymap reference is UI text, not step code: it names the command
		// for the user, which is exactly the opposite of the installer running it.
		if strings.HasPrefix(name, "keymaps_") {
			continue
		}

		path := filepath.Join(dir, name)
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		scanned++
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			if strings.Contains(strings.ToLower(lit.Value), "herdr plugin") {
				found = true
			}
			return true
		})
	}
	if scanned == 0 {
		t.Fatal("no installer sources were scanned, so this guard proves nothing")
	}
	return found
}

// TestHerdrPluginSetAndDocumentationAgree keeps the declared set and the prose
// that names it in step. The set is declared once, in installer.go, and the
// installer's documentation must list the same plugins and the same GitHub
// sources. A plugin added to the set without a doc line fails here; clearing the
// set fails the non-empty guard, so a guard that checks nothing cannot pass.
func TestHerdrPluginSetAndDocumentationAgree(t *testing.T) {
	if len(herdrPluginSet) == 0 {
		t.Fatal("the declared Herdr plugin set is empty, so this guard proves nothing")
	}

	tools, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "tools.md"))
	if err != nil {
		t.Fatalf("read docs/tools.md: %v", err)
	}
	text := string(tools)

	for _, plugin := range herdrPluginSet {
		if !strings.Contains(text, plugin.ID) {
			t.Errorf("docs/tools.md does not name the declared plugin id %q", plugin.ID)
		}
		if !strings.Contains(text, plugin.shorthand()) {
			t.Errorf("docs/tools.md does not name the declared source %q, so a reader cannot tell where %q comes from",
				plugin.shorthand(), plugin.ID)
		}
	}
}
