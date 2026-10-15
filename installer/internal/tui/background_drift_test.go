package tui

// The background-drift guard, written against view.go, model.go and anim.go as
// they stand before the feature exists (S1-S7 of
// odd/tasks/installer-background-drifts.md, branch feat/installer-drift-background).
// Every assertion names the thing the implementation still owes: "no screen
// drifts", "the model cannot turn the drift off", not "got false".
//
// Two contracts a later edit must keep, stated here because S6 holds them:
//
//  1. The model carries the drift's own answer as a bool field named
//
//	Drifting
//
//     read once when the model is built, from the same gate that produces
//     Animating (--no-anim / DOTFILES_ANIM=0 and the terminal gates
//     animationGate applies), exactly as PixelSprite is a sub-answer of the same
//     animation with its own field. Animating stays what it is: the frame tick
//     and the creature.
//
//  2. The package defines exactly one paint point, a function named
//
//	paintDriftBackground
//
//     and calls it exactly once, from Model.View, so every screen -- the forty
//     cases of View's switch, the utilities section and the trainer among them --
//     passes through that one tail and no screen can opt out.
//
// This file must compile before the feature exists, so it never names the field
// directly: it reaches Drifting through reflection on an addressable copy of the
// model (driftAnswer / requireDrift), and when the field is absent it says so
// rather than failing the build for the whole package. S6 reads model.go and the
// non-test Go files and asserts both declarations structurally.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	// driftField is the model field the design adds for the drift's own answer.
	driftField = "Drifting"
	// driftPainter is the one paint point's name, the S6 contract.
	driftPainter = "paintDriftBackground"
)

// driftAnswer returns a copy of m with Model.<driftField> forced to want. It
// reaches the field by name through reflection so this file compiles against a
// tree where the field does not exist yet; the bool reports whether the model
// carried a forceable answer.
func driftAnswer(m Model, want bool) (Model, bool) {
	v := reflect.ValueOf(&m).Elem()
	field := v.FieldByName(driftField)
	if !field.IsValid() || field.Kind() != reflect.Bool || !field.CanSet() {
		return m, false
	}
	field.SetBool(want)
	return m, true
}

// requireDrift is driftAnswer for a test that must see the drift move. A model
// without the field is a failure that names the contract, not a panic.
func requireDrift(t *testing.T, m Model, want bool) Model {
	t.Helper()
	out, ok := driftAnswer(m, want)
	if !ok {
		t.Fatalf("Model has no forceable %s bool: the model cannot turn the drift off, so no run can stop the background (the design gives the background its own answer, read once at build beside Animating and PixelSprite)", driftField)
	}
	return out
}

// driftSubject is a screen whose render depends on no clock but the model: the
// welcome screen at the floor draws no companion, so once the drift exists it is
// the only thing a frame tick can move. The tests that assert the drift advances
// use it so the companion cannot answer for the background.
func driftSubject(t *testing.T) Model {
	t.Helper()
	m := installerFrameCase(t, "welcome")
	m.Width, m.Height = 80, 24
	m.Animating, m.PixelSprite = true, false
	m.ink = companionInkFor(true)
	m.AnimTick = 0
	return m
}

// driftFrame is one screen of the shared matrix rendered twice: with the drift
// on and with it off, every other field -- Animating, PixelSprite, AnimTick --
// identical. The models are built from the same terminalFitCases enumeration the
// shared pass (terminalFrames) renders, at the same measured sizes.
type driftFrame struct {
	caseName string
	sizeName string
	driftOn  string
	driftOff string
}

var (
	driftMatrix        []driftFrame
	driftMatrixPresent bool
)

// sharedDriftFrames renders the shared matrix once with the drift on and off.
// The bool says whether the model carried the drift field at all; S1 fails on a
// false, S5 needs no answer either way because an absent field makes the two
// renders the same bytes.
func sharedDriftFrames(t *testing.T) ([]driftFrame, bool) {
	t.Helper()
	if driftMatrix != nil {
		return driftMatrix, driftMatrixPresent
	}
	cases := terminalFitCases()
	frames := make([]driftFrame, 0, len(cases)*len(measuredTerminalSizes))
	present := false
	for _, c := range cases {
		m := c.build(t)
		m.Animating, m.PixelSprite = true, true
		m.ink = companionInkFor(true)
		for _, size := range measuredTerminalSizes {
			m.Width, m.Height = size.width, size.height
			on, okOn := driftAnswer(m, true)
			off, okOff := driftAnswer(m, false)
			if okOn && okOff {
				present = true
			}
			frames = append(frames, driftFrame{
				caseName: c.name,
				sizeName: size.name,
				driftOn:  on.View(),
				driftOff: off.View(),
			})
		}
	}
	if want := len(cases) * len(measuredTerminalSizes); len(frames) != want {
		t.Fatalf("the matrix rendered %d frames, want %d: the guard is not reading the same screens the shared pass does", len(frames), want)
	}
	driftMatrix, driftMatrixPresent = frames, present
	return driftMatrix, driftMatrixPresent
}

// TestBackgroundDriftS1EveryScreenDriftsWhenTheDriftIsOn is the guard for S1:
// with the drift on, every screen in the shared matrix differs from the same
// model with the drift off, every other field -- Animating included -- identical.
// No screen is left out, and no SGR signature is read: the difference itself is
// the evidence.
func TestBackgroundDriftS1EveryScreenDriftsWhenTheDriftIsOn(t *testing.T) {
	frames, present := sharedDriftFrames(t)
	if !present {
		t.Fatalf("Model has no forceable %s bool: no screen can be turned on or off for its background, and every screen is expected to differ when the drift is on (the design gives the background its own answer, read once at build beside Animating and PixelSprite)", driftField)
	}

	var same []string
	for _, f := range frames {
		if f.driftOn == f.driftOff {
			same = append(same, f.caseName+"/"+f.sizeName)
		}
	}
	if len(same) > 0 {
		shown := same
		if len(shown) > 8 {
			shown = shown[:8]
		}
		t.Errorf("%d of %d rendered screens are byte-identical with the drift on and with it off (%s): no screen may opt out of the background",
			len(same), len(frames), strings.Join(shown, ", "))
	}
}

// TestBackgroundDriftS2TheExistingFrameTickCarriesTheDrift is the guard for S2:
// the drift advances when animTickMsg -- the frame tick that already exists -- is
// delivered through Update, and the same message re-arms that same cadence. No
// second timer is delivered here, and none is needed to see the render move.
func TestBackgroundDriftS2TheExistingFrameTickCarriesTheDrift(t *testing.T) {
	forceTrueColor(t)
	m := requireDrift(t, driftSubject(t), true)
	before := m.View()

	cur := m
	var lastCmd tea.Cmd
	for i := 0; i < 2*animTicksPerSecond; i++ {
		next, cmd := cur.Update(animTickMsg{})
		model, ok := next.(Model)
		if !ok {
			t.Fatalf("Update returned %T, want Model", next)
		}
		cur, lastCmd = model, cmd
	}

	if cur.View() == before {
		t.Errorf("delivering the existing animTickMsg %d times changed nothing: the drift does not advance on the frame tick that already exists, so a second timer would be needed to see it move", 2*animTicksPerSecond)
	}
	if lastCmd == nil {
		t.Errorf("the frame tick did not re-arm through Update, so the cadence the drift rides stopped")
	} else if reflect.ValueOf(lastCmd).Pointer() != reflect.ValueOf(animTickCmd()).Pointer() {
		t.Errorf("Update re-armed a command that is not this package's existing frame tick: the drift is riding a cadence of its own")
	}
}

// TestBackgroundDriftS3TheDriftIsAPureFunctionOfTheFrameCounter is the guard for
// S3: with the drift on, two renders of identical state, the frame counter
// included, are byte for byte the same -- the render reads no clock and no
// environment -- and some advance of the counter moves the bytes, so a static
// gradient fails this guard.
func TestBackgroundDriftS3TheDriftIsAPureFunctionOfTheFrameCounter(t *testing.T) {
	forceTrueColor(t)
	m := requireDrift(t, driftSubject(t), true)

	if m.View() != m.View() {
		t.Errorf("two renders of the same model differ, so a render reads something outside the model state")
	}

	a := m
	a.AnimTick = 0
	advanced := false
	for k := 1; k <= 2*animTicksPerSecond; k++ {
		b := m
		b.AnimTick = a.AnimTick + k
		if a.View() != b.View() {
			advanced = true
			break
		}
	}
	if !advanced {
		t.Errorf("the render at frame %d is identical to every render up to frame %d: the background is static, so it is not a function of the frame counter",
			a.AnimTick, a.AnimTick+2*animTicksPerSecond)
	}
}

// TestBackgroundDriftS4TheDriftOffRenderDoesNotMoveWithTheCounter is the in-file
// half of S4: with the drift off, the render does not move with the frame
// counter -- counters n and n+k are byte-identical.
//
// The fleet-level half of S4 is not assertable from here and is stated here
// instead: a drift-off run must be today's bytes, and the package already proves
// it by keeping every existing frame comparison green unchanged -- 227 calls to
// View() across 20 test files. If the drift-off render stopped being today's
// bytes, those comparisons would move, and none of them lives in this file.
//
// The subject is the welcome screen at the floor, which draws no companion: it is
// the screen where nothing but the drift reads the frame counter, so a render
// that moves here is the background moving and nothing else.
func TestBackgroundDriftS4TheDriftOffRenderDoesNotMoveWithTheCounter(t *testing.T) {
	forceTrueColor(t)
	m := driftSubject(t)
	m, _ = driftAnswer(m, false) // no field means no drift, which is off already

	a := m
	a.AnimTick = 0
	for k := 1; k <= 2*animTicksPerSecond; k++ {
		b := m
		b.AnimTick = a.AnimTick + k
		if a.View() != b.View() {
			t.Errorf("with the drift off, the render at frame %d differs from the render at frame %d: the background must not move while it is off",
				b.AnimTick, a.AnimTick)
			break
		}
	}
}

// TestBackgroundDriftS5TheDriftChangesOnlyColour is the guard for S5: with
// everything identical except the drift -- the same model, the same animation
// gate, the same sprite, the same counter -- the ANSI-stripped renders are
// identical: same rows, same order, same count, the same width per row. The
// background can never reflow, clip or shorten a screen.
//
// The reference is the same model with Drifting false, not the animation gate's
// off state: Animating also draws the companion, moves the panel summary and
// changes the live panel's text, so comparing it would measure those and not the
// background.
func TestBackgroundDriftS5TheDriftChangesOnlyColour(t *testing.T) {
	frames, _ := sharedDriftFrames(t)
	for _, f := range frames {
		onRows := strings.Split(ansiEscape.ReplaceAllString(f.driftOn, ""), "\n")
		offRows := strings.Split(ansiEscape.ReplaceAllString(f.driftOff, ""), "\n")
		if len(onRows) != len(offRows) {
			t.Errorf("%s at %s renders %d rows with the drift on and %d with it off: the drift changed the geometry",
				f.caseName, f.sizeName, len(onRows), len(offRows))
			continue
		}
		for r := range onRows {
			if onRows[r] != offRows[r] {
				t.Errorf("%s at %s changes row %d when the drift is on: the drift must change colour only, never text\n on: %q\n off: %q",
					f.caseName, f.sizeName, r, onRows[r], offRows[r])
				break
			}
			if got, want := lipgloss.Width(onRows[r]), lipgloss.Width(offRows[r]); got != want {
				t.Errorf("%s at %s is %d columns wide on row %d with the drift on and %d with it off: the drift changed the geometry",
					f.caseName, f.sizeName, got, r, want)
				break
			}
		}
	}
}

// TestBackgroundDriftS6OnePaintPointAndTheDriftFieldOnTheModel is the guard for
// S6: the model declares the drift's own bool (Drifting) and the package has
// exactly one paint point, the function named by the contract at the top of this
// file, called once, from Model.View.
func TestBackgroundDriftS6OnePaintPointAndTheDriftFieldOnTheModel(t *testing.T) {
	fset := token.NewFileSet()

	if !modelDeclaresDriftField(t, fset) {
		t.Errorf("model.go declares no %s bool on Model: the drift has no answer on the model, so no run can turn the background off", driftField)
	}

	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parsing the package's non-test Go files: %v", err)
	}

	definitions, files := 0, 0
	var calls []*ast.CallExpr
	var view *ast.FuncDecl
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			files++
			ast.Inspect(file, func(n ast.Node) bool {
				switch node := n.(type) {
				case *ast.FuncDecl:
					if node.Name.Name == driftPainter {
						definitions++
					}
					if node.Name.Name == "View" && receiverIsModel(node) {
						view = node
					}
				case *ast.CallExpr:
					if callName(node) == driftPainter {
						calls = append(calls, node)
					}
				}
				return true
			})
		}
	}
	if files == 0 {
		t.Fatalf("parsed no non-test Go file: this guard is not reading the package it names")
	}
	if definitions == 0 {
		t.Errorf("the package defines no %s: no screen's render passes through a single paint point, so every screen would have to paint its own background", driftPainter)
	} else if definitions > 1 {
		t.Errorf("the package defines %s %d times: every screen goes through one paint point, not several", driftPainter, definitions)
	}
	if definitions > 0 && len(calls) != 1 {
		t.Errorf("%s is called %d times in the package's non-test code, want exactly once from View: a second call site is a second paint point", driftPainter, len(calls))
		return
	}
	if definitions == 0 {
		return
	}
	if view == nil {
		t.Fatalf("the package has no Model.View, so the painter's single call is not the tail every screen passes through")
	}
	if calls[0].Pos() < view.Pos() || calls[0].End() > view.End() {
		t.Errorf("%s is called outside Model.View at %s: the paint point must be the one tail every screen already passes through", driftPainter, fset.Position(calls[0].Pos()))
	}
}

// TestBackgroundDriftS7TheSpriteSwitchLeavesTheDriftAlone is the guard for S7:
// the sprite and the drift are separate answers. With PixelSprite off and
// Animating on, the drift still makes the render differ.
func TestBackgroundDriftS7TheSpriteSwitchLeavesTheDriftAlone(t *testing.T) {
	forceTrueColor(t)
	m := driftSubject(t)
	if m.PixelSprite {
		t.Fatalf("the subject has the pixel sprite on, so this guard cannot tell the two switches apart")
	}
	if !m.Animating {
		t.Fatalf("the subject has the animation gate off, so this guard cannot show the drift still moves")
	}

	on := requireDrift(t, m, true)
	off := requireDrift(t, m, false)
	if on.PixelSprite || !on.Animating {
		t.Fatalf("forcing the drift changed the sprite or the animation switch: PixelSprite=%t Animating=%t", on.PixelSprite, on.Animating)
	}
	if on.View() == off.View() {
		t.Errorf("with the pixel sprite off and the animation on, the drift on and the drift off render the same bytes: the sprite's answer switched the background")
	}
}

// modelDeclaresDriftField reports whether model.go declares Model.<driftField>
// as a bool. It parses the file directly, the way screenConstantsInDeclarationOrder
// reads the Screen block, so the guard asserts the declaration itself rather than
// trusting the reflection helper.
func modelDeclaresDriftField(t *testing.T, fset *token.FileSet) bool {
	t.Helper()
	file, err := parser.ParseFile(fset, "model.go", nil, 0)
	if err != nil {
		t.Fatalf("parse model.go: %v", err)
	}
	found := false
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.TypeSpec)
		if !ok || spec.Name.Name != "Model" {
			return true
		}
		strukt, ok := spec.Type.(*ast.StructType)
		if !ok {
			return true
		}
		for _, field := range strukt.Fields.List {
			for _, name := range field.Names {
				if name.Name != driftField {
					continue
				}
				if id, ok := field.Type.(*ast.Ident); ok && id.Name == "bool" {
					found = true
				}
			}
		}
		return true
	})
	return found
}

// receiverIsModel reports whether a method declaration has a Model receiver.
func receiverIsModel(decl *ast.FuncDecl) bool {
	if decl.Recv == nil || len(decl.Recv.List) != 1 {
		return false
	}
	switch typ := decl.Recv.List[0].Type.(type) {
	case *ast.Ident:
		return typ.Name == "Model"
	case *ast.StarExpr:
		if id, ok := typ.X.(*ast.Ident); ok {
			return id.Name == "Model"
		}
	}
	return false
}

// callName is the name a call expression invokes, for the plain and the method
// spelling.
func callName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		return fun.Sel.Name
	}
	return ""
}
