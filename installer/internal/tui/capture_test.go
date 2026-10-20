package tui

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestCaptureEveryScreen writes one text frame per screen the constants define, so
// a layout can be inspected instead of assumed. It is opt-in: with
// DOTFILES_CAPTURE_DIR unset it does nothing, because a test that writes files into
// the machine is not a check. With it set, the frames land in that directory
// together with a summary of the rows each one spent, and a screen that panics
// while drawing is named there rather than taking the run down.
func TestCaptureEveryScreen(t *testing.T) {
	out := os.Getenv("DOTFILES_CAPTURE_DIR")
	if out == "" {
		t.Skip("set DOTFILES_CAPTURE_DIR to write the frames")
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}

	file, err := parser.ParseFile(token.NewFileSet(), "model.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	ast.Inspect(file, func(node ast.Node) bool {
		spec, ok := node.(*ast.ValueSpec)
		if !ok {
			return true
		}
		for _, ident := range spec.Names {
			if strings.HasPrefix(ident.Name, "Screen") {
				names = append(names, ident.Name)
			}
		}
		return true
	})

	ansi := regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]`)
	var summary strings.Builder
	for i, name := range names {
		func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Fprintf(&summary, "%03d\t%s\tPANIC\t%v\n", i, name, r)
				}
			}()
			m := NewModel()
			m.SystemInfo = goldenSystemInfo()
			m.CreatedAt = goldenGreetingTime
			m.Width, m.Height = 120, 36
			m.Screen = Screen(i)
			m.Animating = false
			view := ansi.ReplaceAllString(m.View(), "")
			path := filepath.Join(out, fmt.Sprintf("%03d-%s.txt", i, name))
			os.WriteFile(path, []byte(view), 0o644)
			lines := strings.Count(view, "\n") + 1
			footer := strings.Contains(view, "•")
			fmt.Fprintf(&summary, "%03d\t%s\tlines=%d\tover=%v\tfooter=%v\n", i, name, lines, lines > 36, footer)
		}()
	}
	os.WriteFile(filepath.Join(out, "summary.tsv"), []byte(summary.String()), 0o644)
	fmt.Printf("screens=%d\n%s\n", len(names), summary.String())
}
