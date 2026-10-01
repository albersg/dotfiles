package tui

import (
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
