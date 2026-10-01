package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/tui/trainer"
)

// TestEveryScreenIsCoveredByAFrameGuard guards the class of defect where a screen
// ships without ever being rendered at the floor the installer claims to support.
//
// The frame guards iterate a hand-written list of states, so a screen added to
// model.go is measured by nothing at all until somebody remembers to add a state
// for it, and the omission is invisible: the guards stay green. The same shape
// produced this repository's colour-leak defect, where a guard could only see the
// instances it had been told about.
//
// This test derives the screens from the source instead. Every Screen constant
// declared in model.go must be measured by a frame guard: either one of the
// installer's states, or - for the screens those states never reach - a state
// built here, which is measured by this test itself. A screen nobody measures
// fails the test by name, which is the whole point: the failure has to tell the
// next person where to look.
func TestEveryScreenIsCoveredByAFrameGuard(t *testing.T) {
	declared := screenConstantsInDeclarationOrder(t)
	if len(declared) == 0 {
		t.Fatal("model.go declares no Screen constants, so this guard proves nothing")
	}

	// Which guard measures which screen, so a failure can name the gap and a
	// duplicate claim is visible rather than silent.
	measured := map[Screen]string{}
	for _, name := range installerFrameScreenNames {
		m := installerFrameCase(t, name)
		if _, claimed := measured[m.Screen]; !claimed {
			measured[m.Screen] = "TestInstallerScreensFitTheFrame, state " + name
		}
	}

	for _, c := range screensTheInstallerStatesNeverReach(t) {
		assertInstallerScreenFits(t, c.name, c.model)
		if owner, claimed := measured[c.model.Screen]; claimed {
			t.Errorf("%s is measured by %s and again by this test as %s: one of the two is now redundant, "+
				"which hides the screen that is actually missing a state", c.name, owner, c.name)
			continue
		}
		measured[c.model.Screen] = "this guard, state " + c.name
	}

	if len(measured) == 0 {
		t.Fatal("no screen was measured, so this guard proves nothing")
	}

	var missing []string
	for value, name := range declared {
		if _, ok := measured[Screen(value)]; !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		t.Errorf("no frame guard measures %d of %d screens: %v\n"+
			"A screen that nobody renders is a screen that can overflow the frame, clip a column or lose its "+
			"bottom rows without a single test complaining. Add a state for it to installerFrameScreenNames, or "+
			"build one here next to the others.",
			len(missing), len(declared), missing)
	}

	t.Logf("%d screens declared, %d of them measured by a frame guard at %dx%d",
		len(declared), len(measured), trainerFrameWidth, trainerFrameHeight)
}

// screenConstantsInDeclarationOrder reads model.go and returns the names of the
// Screen constants in the order they are declared, which is also their order by
// value while the block stays a contiguous iota run. That assumption is checked
// rather than hoped for: the mapping from a name to the value the guards report
// only holds under it, and a screen declared with an explicit value would
// otherwise be compared against the wrong name.
func screenConstantsInDeclarationOrder(t *testing.T) []string {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "model.go", nil, 0)
	if err != nil {
		t.Fatalf("parse model.go: %v", err)
	}

	var names []string
	blocks := 0
	ast.Inspect(file, func(n ast.Node) bool {
		decl, ok := n.(*ast.GenDecl)
		if !ok || decl.Tok != token.CONST || len(decl.Specs) == 0 {
			return true
		}
		first, ok := decl.Specs[0].(*ast.ValueSpec)
		if !ok {
			return true
		}
		if id, ok := first.Type.(*ast.Ident); !ok || id.Name != "Screen" {
			return true
		}
		blocks++
		for i, spec := range decl.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			if i == 0 {
				if len(value.Values) != 1 {
					t.Fatalf("the Screen block's first constant has %d values, want the iota run", len(value.Values))
				}
				if id, ok := value.Values[0].(*ast.Ident); !ok || id.Name != "iota" {
					t.Fatalf("the Screen block does not start at iota, so a name cannot be mapped to its value")
				}
			} else if len(value.Values) != 0 {
				t.Fatalf("%s has an explicit value inside the Screen block, so the block is no longer a "+
					"contiguous run and the name-to-value mapping this guard relies on is wrong",
					value.Names[0].Name)
			}
			for _, name := range value.Names {
				names = append(names, name.Name)
			}
		}
		return true
	})

	if blocks != 1 {
		t.Fatalf("found %d const blocks declaring the Screen type, want exactly 1", blocks)
	}
	return names
}

// screenCase is a screen rendered by this guard because the installer's states
// never reach it.
type screenCase struct {
	name  string
	model Model
}

// screensTheInstallerStatesNeverReach builds the trainer's screens at the frame
// the repository claims to support. The trainer composes its own rows and is
// entered from its own menu, so none of these appear among the installer's
// states; the fit assertion is the same one the installer's states use, which is
// what makes this a frame guard rather than a second, weaker check.
func screensTheInstallerStatesNeverReach(t *testing.T) []screenCase {
	t.Helper()
	t.Setenv("HOME", t.TempDir())

	built := func(screen Screen) Model {
		m := NewModel()
		m.Width, m.Height = trainerFrameWidth, trainerFrameHeight
		m.Screen = screen
		m.TrainerStats = trainer.NewUserStats()
		m.TrainerGameState = trainer.NewGameStateWithStats(m.TrainerStats)
		return m
	}

	menu := built(ScreenTrainerMenu)
	menu.TrainerModules = trainer.GetAllModules()

	lesson := built(ScreenTrainerLesson)
	lesson.TrainerGameState.StartLesson(trainer.ModuleHorizontal)

	exercises := trainer.GetLessons(trainer.ModuleHorizontal)
	if len(exercises) == 0 {
		t.Fatal("the horizontal module has no lessons to render, so the practice screen cannot be checked")
	}
	practice := built(ScreenTrainerPractice)
	practice.TrainerGameState.SetPracticeExercise(&exercises[0])

	boss := built(ScreenTrainerBoss)
	boss.TrainerGameState.StartBoss(trainer.ModuleHorizontal)

	result := built(ScreenTrainerResult)
	result.TrainerGameState.StartLesson(trainer.ModuleHorizontal)
	result.TrainerLastCorrect = true
	result.TrainerMessage = ""

	bossResult := built(ScreenTrainerBossResult)
	bossResult.TrainerGameState.StartBoss(trainer.ModuleHorizontal)

	return []screenCase{
		{"trainer-menu", menu},
		{"trainer-lesson", lesson},
		{"trainer-practice", practice},
		{"trainer-boss", boss},
		{"trainer-result", result},
		{"trainer-boss-result", bossResult},
	}
}
