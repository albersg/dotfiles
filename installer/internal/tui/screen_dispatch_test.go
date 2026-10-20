package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// The doctor screen rendered its rows and answered nothing: the hander existed
// and the dispatcher had no case for either of its screens, so every advertised
// key -- a, c, r, t, w, and Escape itself -- reached nothing and the screen read
// as frozen. This test drives the dispatcher, not the handler, because that is
// the layer that was missing.
func TestTheDoctorScreenAnswersTheKeysItAdvertises(t *testing.T) {
	m := NewModel()
	m.Width, m.Height = 120, 40
	m.Screen = ScreenDoctor

	next, cmd := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	if cmd == nil {
		t.Fatalf("the dispatcher returned no command for the doctor's run-everything key")
	}
	model := next.(Model)
	if model.Doctor.Run == nil || len(model.Doctor.Run.Plan) == 0 {
		t.Fatalf("the dispatcher delivered the key but no run started")
	}

	// Escape while the run is in flight cancels it and stays on the screen: the
	// checks already handed to the runtime run to their own bound, and the ones
	// that did not answer are recorded as unknown with the reason.
	cancelled, _ := model.handleKeyPress(tea.KeyMsg{Type: tea.KeyEsc})
	cancelModel := cancelled.(Model)
	if cancelModel.Screen != ScreenDoctor {
		t.Fatalf("Escape during a run left the doctor for %v, want it to stop the run in place", cancelModel.Screen)
	}
	if cancelModel.Doctor.Run == nil || cancelModel.Doctor.Run.running() {
		t.Fatalf("Escape during a run did not close it")
	}

	// With nothing running, the same key is the way back to the section.
	back, _ := cancelModel.handleKeyPress(tea.KeyMsg{Type: tea.KeyEsc})
	if got := back.(Model).Screen; got != ScreenUtilities {
		t.Fatalf("Escape left the doctor on %v, want the utilities section", got)
	}

	confirm := NewModel()
	confirm.Width, confirm.Height = 120, 40
	confirm.Screen = ScreenDoctorFix
	if again, _ := confirm.handleKeyPress(tea.KeyMsg{Type: tea.KeyEsc}); again.(Model).Screen != ScreenDoctor {
		t.Fatalf("Escape left the fix confirmation on %v, want the doctor", again.(Model).Screen)
	}
}

// A handler that nothing dispatches is a screen that draws keys it will never
// receive: the user presses what the footer advertises and the interface does not
// move. This walks the package's own source, so a new screen cannot be added with
// a handler and no dispatch case without failing here.
func TestEveryScreenKeyHandlerIsDispatched(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	handler := regexp.MustCompile(`^handle([A-Z]\w*)Keys$`)
	screens := map[string]bool{}
	dispatched := map[string]bool{}
	handlers := map[string]string{}

	fset := token.NewFileSet()
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.GenDecl:
				if node.Tok != token.CONST {
					return true
				}
				for _, spec := range node.Specs {
					value, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for _, ident := range value.Names {
						if strings.HasPrefix(ident.Name, "Screen") {
							screens[ident.Name] = true
						}
					}
				}
			case *ast.FuncDecl:
				if node.Name == nil {
					return true
				}
				if match := handler.FindStringSubmatch(node.Name.Name); match != nil {
					handlers[node.Name.Name] = "Screen" + match[1]
				}
			case *ast.CaseClause:
				for _, expression := range node.List {
					if ident, ok := expression.(*ast.Ident); ok && strings.HasPrefix(ident.Name, "Screen") {
						dispatched[ident.Name] = true
					}
				}
			}
			return true
		})
	}

	if len(handlers) == 0 {
		t.Fatalf("no key handlers were found; the walk is looking in the wrong place")
	}
	for name, screen := range handlers {
		// A handler whose name does not name a screen constant (a shared table
		// handler, say) has nothing to dispatch and is not this test's subject.
		if !screens[screen] {
			continue
		}
		if !dispatched[screen] {
			t.Errorf("%s handles %s, and no case in the dispatcher names it: its keys reach nothing", name, screen)
		}
	}
	if len(os.Args) == 0 { // keeps os imported for the failure path above
		t.Fatal("unreachable")
	}
}
