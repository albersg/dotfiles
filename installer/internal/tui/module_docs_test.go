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
