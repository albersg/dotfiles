package tui

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
	"github.com/albersg/dotfiles/installer/internal/tui/trainer"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

// measuredTerminalSizes is the grid a measurement of every screen ran against:
// twelve terminals from the 60x20 the trainer documents as too small through the
// 80x24 floor and the widths the responsive layout was designed on. It is the
// grid the frame guards never covered as a class -- each sized its own case and
// checked one axis -- which is how four screens shipped overflowing or clipping
// at sizes nobody rendered.
var measuredTerminalSizes = []struct {
	name          string
	width, height int
}{
	{"80x24", 80, 24},
	{"90x28", 90, 28},
	{"100x25", 100, 25},
	{"100x30", 100, 30},
	{"120x24", 120, 24},
	{"120x34", 120, 34},
	{"140x44", 140, 44},
	{"160x50", 160, 50},
	{"200x60", 200, 60},
	{"227x62", 227, 62},
	{"80x30", 80, 30},
	{"60x20", 60, 20},
}

// TestEveryScreenFitsEveryTerminalSize is the class guard for the defect the
// per-screen guards could not see: a screen whose height and width fit the one
// terminal it was measured at but not another. The frame guards sized their own
// case each (the 80x24 floor, then 160x50 and 227x62), so nothing rendered the
// sizes between them, and a screen could render 50 rows in a 44-row terminal or
// 76 columns in a 60-column one without a single test complaining.
//
// It reads the shared matrix pass (terminalFrames) and asserts the two things the
// terminal itself enforces without saying so: no screen draws more rows than the
// terminal has, and no visible line is wider than the terminal. A line is measured
// after its escape sequences are stripped, with each wide rune counted as the two
// cells it occupies.
//
// The pass renders with the companion animating, which is the strictest frame: the
// creature is drawn in rows the body did not need, so a screen that fits with it on
// fits without it, and one that overflowed only with it on is a defect this guard
// now catches rather than one it could miss.
//
// The case count is pinned rather than derived: the point of this guard is that
// every screen is in it, so a screen silently dropping out of the enumeration has
// to fail here instead of shrinking the measurement.
func TestEveryScreenFitsEveryTerminalSize(t *testing.T) {
	frames := terminalFrames(t)

	if screens := len(frames) / len(measuredTerminalSizes); screens != measuredScreens {
		t.Fatalf("the guard enumerates %d screens, want the measured %d: a screen that is not rendered here can overflow its terminal unmeasured",
			screens, measuredScreens)
	}

	checked := 0
	for _, f := range frames {
		f := f
		t.Run(f.caseName+"/"+f.sizeName, func(t *testing.T) {
			assertScreenFitsTerminal(t, f.caseName, f.width, f.height, f.view)
		})
		checked++
	}

	if want := measuredScreens * len(measuredTerminalSizes); checked != want {
		t.Fatalf("the guard rendered %d screen x size cases, want %d", checked, want)
	}
	t.Logf("rendered %d screens at %d sizes: %d screen x size cases, read from the shared matrix pass", measuredScreens, len(measuredTerminalSizes), checked)
}

// measuredScreens is the number of screen cases every matrix guard renders: the
// 47 installer states, the utilities section, the theme picker, the refresh
// review that picker opens, and the trainer's 6. It is pinned rather than
// derived so a screen silently dropping out of the enumeration fails more than
// one guard.
// measuredScreens is the number of screen cases every matrix guard renders: the
// installer's states, the utilities section with the screens it opens one level
// in, and the trainer's own. It is pinned rather than derived on purpose -- the
// point of the guard is that every screen is in it -- so a screen added to the
// package has to be added here and fails loudly until it is.
const measuredScreens = 57 // 47 installer states + utilities + theme-picker + theme-refresh + shell-audit + the trainer's 6

// terminalMatrixFrame is one frame of the single render pass the matrix guards
// share: one screen case rendered at one measured terminal with the companion
// animating. Before the pass existed, the fit guard and the companion-coverage
// guard each built the same 55 models 660 times and rendered the same frames
// twice. The build is the expensive half -- each one points HOME and XDG_STATE_HOME
// at fresh temporary directories -- so the repetition, not the drawing, was what
// took this package from tens of seconds to minutes.
type terminalMatrixFrame struct {
	caseName string
	sizeName string
	width    int
	height   int
	view     string
}

// sharedTerminalFrames holds the one pass every matrix guard reads. It is built
// lazily by the first guard that asks for it. The guards run one after another, so
// there is no concurrency to guard, and building it inside a test keeps the fixture
// builders -- which call t.Setenv -- on the test goroutine. It is never reset, so
// the second guard reuses the bytes the first one rendered.
var sharedTerminalFrames []terminalMatrixFrame

// terminalFrames renders every case at every measured terminal once, for every
// matrix guard to read. Each case's model is built once and resized per terminal
// instead of rebuilt per (case, terminal): View has a value receiver, so a resize
// cannot leave state behind, and the build is the part worth removing. The
// animation flags and the dark ink are set here rather than per guard so the frames
// are the strictest, fully-animated ones.
func terminalFrames(t *testing.T) []terminalMatrixFrame {
	t.Helper()

	if sharedTerminalFrames != nil {
		return sharedTerminalFrames
	}

	cases := terminalFitCases()
	want := len(cases) * len(measuredTerminalSizes)
	frames := make([]terminalMatrixFrame, 0, want)
	for _, c := range cases {
		m := c.build(t)
		m.Animating, m.PixelSprite = true, true
		m.ink = companionInkFor(true)
		for _, size := range measuredTerminalSizes {
			m.Width, m.Height = size.width, size.height
			frames = append(frames, terminalMatrixFrame{
				caseName: c.name,
				sizeName: size.name,
				width:    size.width,
				height:   size.height,
				view:     m.View(),
			})
		}
	}
	if len(frames) != want {
		t.Fatalf("the shared matrix rendered %d frames, want %d", len(frames), want)
	}
	sharedTerminalFrames = frames
	return sharedTerminalFrames
}

// The companion coverage main measured before the mini volume rung existed. The
// mini is the same five rows as the glyph cat's full rung and takes over exactly that
// band, so with the encoder on the floor must draw at least as many creatures as it
// did on main: a taller rung that pushes the compact glyph cat out of a screen it
// used to fit is the regression this pins. mainCompanionCoverage80x24 came from the
// 53 measured screens at 80x24; mainCompanionCoverageTotal is the 636-case total.
const (
	mainCompanionCoverage80x24 = 35
	mainCompanionCoverageTotal = 461
)

// TestCompanionCoverageAcrossTerminalSizes measures how many screen x size cases
// draw a creature at all, and pins the floor against the pre-mini baseline. The rung
// is chosen by the terminal, but a frame whose body fills it refuses to overwrite
// content (placeCompanion), so a too-tall rung leaves more screens showing no
// creature than it needs to. It reads the same shared pass the fit guard reads, so
// the two never build or render the same frame twice. The total is logged, and the
// two baselines are asserted: coverage must never drop below what main drew.
func TestCompanionCoverageAcrossTerminalSizes(t *testing.T) {
	frames := terminalFrames(t)
	screens := len(frames) / len(measuredTerminalSizes)

	total, drawn, at80x24 := 0, 0, 0
	for _, f := range frames {
		if _, rows := trainerViewCompanionArt(f.view); rows > 0 {
			drawn++
			if f.sizeName == "80x24" {
				at80x24++
			}
		}
		total++
	}
	t.Logf("companion coverage: %d of %d screen x size cases (%.0f%%) draw a creature; %d draw none; %d of %d draw at 80x24",
		drawn, total, 100*float64(drawn)/float64(total), total-drawn, at80x24, screens)
	if at80x24 < mainCompanionCoverage80x24 {
		t.Errorf("at 80x24 only %d of %d screens draw a creature, want at least the %d main drew: the mini rung must replace the five-row glyph cat, not displace the compact head",
			at80x24, screens, mainCompanionCoverage80x24)
	}
	if drawn < mainCompanionCoverageTotal {
		t.Errorf("companion coverage is %d of %d, below the %d main drew: a rung grew past the rows the body leaves", drawn, total, mainCompanionCoverageTotal)
	}
}

// terminalFitCase is one screen this guard renders at every measured size. The
// builder takes the subtest's *testing.T so the per-case isolation (HOME, the
// pinned greeting time) is scoped to the case and not to the whole guard.
type terminalFitCase struct {
	name  string
	build func(t *testing.T) Model
}

// terminalFitCases is the enumeration the guard renders: the installer's states
// and the trainer's, taken from the same lists the frame guards already iterate.
// Nothing new is invented here -- a screen the guards do not enumerate is caught
// by TestEveryScreenIsCoveredByAFrameGuard, not silently left out of this one.
func terminalFitCases() []terminalFitCase {
	cases := make([]terminalFitCase, 0, len(installerFrameScreenNames)+len(trainerLeakScreenNames))

	for _, name := range installerFrameScreenNames {
		name := name
		cases = append(cases, terminalFitCase{name, func(t *testing.T) Model {
			return installerFrameCase(t, name)
		}})
	}
	for _, name := range trainerLeakScreenNames {
		name := name
		cases = append(cases, terminalFitCase{name, func(t *testing.T) Model {
			return trainerLeakFrameCase(t, name)
		}})
	}
	// The utilities section is entered from the main menu by a key rather than by
	// one of the installer's states, so it is measured here the way the trainer's
	// screens are. The theme picker is one level in from it, for the same reason,
	// and its refresh review is a second state of that picker.
	cases = append(cases, terminalFitCase{utilitiesCaseName, utilitiesFrameCase})
	cases = append(cases, terminalFitCase{themePickerCaseName, themePickerFrameCase})
	cases = append(cases, terminalFitCase{themeRefreshCaseName, themeRefreshReviewFrameCase})
	cases = append(cases, terminalFitCase{shellAuditCaseName, shellAuditFrameCase})

	return cases
}

// assertScreenFitsTerminal measures one rendered screen the way a terminal does.
// The escapes are stripped before the width is taken, so the cell count is the
// visible ink rather than the bytes: lipgloss.Width counts a wide rune as the two
// columns it occupies, and a colour change adds nothing. A screen shorter than
// its terminal is fine; one that is taller or wider is not, because the terminal
// takes the excess away without a marker.
func assertScreenFitsTerminal(t *testing.T, name string, width, height int, view string) {
	t.Helper()

	if rows := renderedRowCount(view); rows > height {
		t.Errorf("%s at %dx%d renders %d rows, want <= %d: the bottom of the screen falls off the terminal (OVERFLOW+%d)",
			name, width, height, rows, height, rows-height)
	}

	widest, widestLine := 0, ""
	for _, line := range strings.Split(ansiEscape.ReplaceAllString(view, ""), "\n") {
		if w := lipgloss.Width(line); w > widest {
			widest, widestLine = w, line
		}
	}
	if widest > width {
		t.Errorf("%s at %dx%d draws a %d-column line, want <= %d: it is clipped silently at the terminal edge (WIDTH+%d): %q",
			name, width, height, widest, width, widest-width, widestLine)
	}

	if widest == 0 && renderedRowCount(view) == 0 {
		// A screen that renders nothing at all would pass both assertions while
		// measuring nothing, so it is a failure rather than a silent pass.
		t.Errorf("%s at %dx%d rendered an empty screen, so the guard measured nothing", name, width, height)
	}
}

// TestTrainerCompanionFloorIsADocumentedException pins the one place the
// companion ladder steps off its documented order. The ladder says the tallest
// sprite the spare rows can hold -- full, else compact, else mini -- but the
// trainer forces the one-row mini at or below trainerFloorHeight, where the
// tracker's ladder-only reading calls the compact three-row rung. That reading
// is now answered in the package comment where the ladder is stated: the floor
// body leaves exactly one spare slot and the 80x24 goldens are pinned to the
// mini. This guard holds the two halves of that answer -- one row at the floor,
// a taller rung above it -- so a future change cannot quietly widen the
// exception or drop the documented one-row floor face.
func TestTrainerCompanionFloorIsADocumentedException(t *testing.T) {
	// The ladder itself still documents the compact rung above the floor; if this
	// stops holding, the exception's justification has changed and this guard
	// should be re-read rather than silently adjusted.
	if rung := companionHeight(companionCompactHeight); rung != companionCompactHeight {
		t.Fatalf("the shared ladder returns %d rows for %d spare, want the compact rung %d: the trainer floor exception is written against that ladder",
			rung, companionCompactHeight, companionCompactHeight)
	}

	floorSizes := []struct {
		name          string
		width, height int
	}{
		{"80x24", trainerFrameWidth, trainerFrameHeight},
		{"120x24", 120, trainerFrameHeight},
		{"60x20", 60, 20},
	}

	for _, name := range trainerLeakScreenNames {
		name := name
		for _, size := range floorSizes {
			size := size
			t.Run(name+"/"+size.name, func(t *testing.T) {
				m := trainerLeakFrameCase(t, name)
				m.Width, m.Height = size.width, size.height
				m.Animating, m.PixelSprite = true, false
				m.ink = companionInkFor(true)

				_, rows := trainerViewCompanionArt(m.View())
				if rows == 0 {
					// A trainer screen that leaves the floor no spare row shows no
					// creature, which is the ladder's own "else nothing" step.
					return
				}
				if rows != companionMiniHeight {
					t.Errorf("%s at %s draws %d companion rows, want the documented floor face of %d: the trainer floor exception is one row, not a taller rung",
						name, size.name, rows, companionMiniHeight)
				}
			})
		}
	}

	// Above the floor the exception does not apply: the trainer hands the height
	// back to the shared ladder, so its screen draws more than the floor's one row.
	m := trainerLeakFrameCase(t, "trainer-lesson")
	m.Width, m.Height = 227, 62
	m.Animating, m.PixelSprite = true, false
	m.ink = companionInkFor(true)
	if _, rows := trainerViewCompanionArt(m.View()); rows <= companionMiniHeight {
		t.Errorf("the trainer above the floor draws %d companion rows, want more than the floor's %d: the exception has leaked above trainerFloorHeight",
			rows, companionMiniHeight)
	}

	// The lesson is the trainer screen the floor must draw the creature on, so
	// the exception is not vacuous: if this ever stops drawing, the floor face the
	// package comment states no longer exists.
	floor := trainerLeakFrameCase(t, "trainer-lesson")
	floor.Width, floor.Height = trainerFrameWidth, trainerFrameHeight
	floor.Animating, floor.PixelSprite = true, false
	floor.ink = companionInkFor(true)
	if _, rows := trainerViewCompanionArt(floor.View()); rows != companionMiniHeight {
		t.Errorf("trainer-lesson at %dx%d draws %d companion rows, want the documented floor face of %d",
			trainerFrameWidth, trainerFrameHeight, rows, companionMiniHeight)
	}
}

// screensTheInstallerStatesNeverReach builds the screens no installer state
// reaches at the frame the repository claims to support: the trainer's screens,
// which compose their own rows and are entered from their own menu, and the
// utilities section, which is entered from the main menu by a key. None of these
// appear among the installer's states; the fit assertion is the same one the
// installer's states use, which is what makes this a frame guard rather than a
// second, weaker check.
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
		{utilitiesCaseName, utilitiesFrameCase(t)},
		{themePickerCaseName, themePickerFrameCase(t)},
		{wslResourcesCaseName, wslResourcesFrameCase(t)},
		{shellAuditCaseName, shellAuditFrameCase(t)},
	}
}

// utilitiesCaseName is the name the frame guards know the utilities section by.
const utilitiesCaseName = "utilities"

// utilitiesFrameCase builds the utilities section with a desktop detected and a
// record to undo, so the guards measure the screen carrying its utility rows
// rather than the empty state a runner with no desktop would produce. The
// detection is forced rather than read from the host: a guard that measured
// whatever desktop the runner happened to have would change with the machine.
func utilitiesFrameCase(t *testing.T) Model {
	t.Helper()

	m := installerFrameModel(t, ScreenUtilities)
	target, ok := themeSwitchByID("gnome")
	if !ok {
		t.Fatal("the theme switch table no longer holds the gnome entry")
	}
	m.ThemeSwitch, m.ThemeSwitchFound = target, true
	m.ThemeRecord = &themeRecord{Target: target.ID, Value: "default", WasDark: false, ToDark: true}

	// The dotfiles theme is one row in the section now: the themes themselves
	// live on the picker, which is measured by its own case below.
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	m.DotfilesThemes = defs
	for i, option := range m.GetCurrentOptions() {
		if option == utilitiesThemeRow {
			m.Cursor = i
			break
		}
	}
	return m
}

// themePickerCaseName is the name the frame guards know the theme picker by.
const themePickerCaseName = "theme-picker"

// themeRefreshCaseName is the name the frame guards know the refresh review by.
const themeRefreshCaseName = "theme-refresh"

// themePickerFrameCase builds the theme picker with the definitions read from
// the repository and a record to undo, so the guards measure the list, its
// exclusion lists, the undo row and the live preview. The cursor is put on a
// theme row so the preview row is measured too: the preview is a row, and a row
// can overflow a frame.
func themePickerFrameCase(t *testing.T) Model {
	t.Helper()

	m := installerFrameModel(t, ScreenThemePicker)
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	m.DotfilesThemes = defs
	m.DotfilesThemeRecord = &dotfilesThemeRecord{Theme: "dotfiles"}
	for i, option := range m.GetCurrentOptions() {
		if strings.HasPrefix(option, "Apply the ") {
			m.Cursor = i
			break
		}
	}
	return m
}

// TestUtilitiesUnavailableFitsEveryTerminalSize measures the other half of the
// utilities section at the same twelve terminals: the state a server, Termux or
// a bare terminal sees, where the section names the situation and offers no
// switch row. The available state is the guard case above; this one exists
// because the copy that explains the absence is itself rows, and rows can
// overflow a frame.
func TestUtilitiesUnavailableFitsEveryTerminalSize(t *testing.T) {
	checked := 0
	for _, size := range measuredTerminalSizes {
		size := size
		t.Run(size.name, func(t *testing.T) {
			m := installerFrameModel(t, ScreenUtilities)
			m.ThemeSwitch, m.ThemeSwitchFound = themeSwitch{}, false
			m.Width, m.Height = size.width, size.height
			assertScreenFitsTerminal(t, "utilities-unavailable", size.width, size.height, m.View())
		})
		checked++
	}
	if checked != len(measuredTerminalSizes) {
		t.Fatalf("the guard rendered %d sizes, want %d", checked, len(measuredTerminalSizes))
	}
}

// TestUtilitiesFrameFitIsMeasuredAtEverySize prints the utilities section's
// measured frame at the twelve terminals the fit guard uses, so the size it is
// drawn at is a number a reader can re-derive rather than a claim. The
// assertions repeat the guard's two rules on purpose: this is the case that
// carries the numbers, and a measurement that is only logged cannot fail.
func TestUtilitiesFrameFitIsMeasuredAtEverySize(t *testing.T) {
	for _, size := range measuredTerminalSizes {
		m := utilitiesFrameCase(t)
		m.Width, m.Height = size.width, size.height
		view := m.View()

		rows := renderedRowCount(view)
		widest := 0
		for _, line := range strings.Split(ansiEscape.ReplaceAllString(view, ""), "\n") {
			widest = max(widest, lipgloss.Width(line))
		}
		t.Logf("utilities at %s: %d of %d rows, %d of %d columns", size.name, rows, size.height, widest, size.width)

		if rows > size.height {
			t.Errorf("utilities at %s renders %d rows, want <= %d", size.name, rows, size.height)
		}
		if widest > size.width {
			t.Errorf("utilities at %s draws %d columns, want <= %d", size.name, widest, size.width)
		}
	}
}

// wslResourcesCaseName is the name the frame guards know the WSL resource screen
// by.
const wslResourcesCaseName = "wsl-resources"

// wslResourcesFrameCase builds the WSL resource screen on a WSL host with a host
// capacity, a recommendation and a file to read, so the guards measure the rows
// and the prose rather than the still-reading state. The host is forced rather
// than detected: a guard that measured the runner's own machine would change
// with the machine.
func wslResourcesFrameCase(t *testing.T) Model {
	t.Helper()

	m := installerFrameModel(t, ScreenWSLResources)
	m.SystemInfo = &system.SystemInfo{OS: system.OSWSL, IsWSL: true, OSName: "WSL"}
	m.WSLState = wslResourceState{
		Resolved:  true,
		Available: true,
		Path:      "/mnt/c/Users/alber/.wslconfig",
		RepoDir:   repoRoot(t),
		Host:      system.HostResources{MemoryBytes: 16 * testGiB, LogicalCPUs: 8},
		Plan:      WSLResources{MemoryMB: 8192, Processors: 8, SwapMB: 2048},
		Current:   WSLResources{MemoryMB: 4096, SwapMB: 1024},
		HasFile:   true,
		Draft:     WSLResources{MemoryMB: 8192, Processors: 8, SwapMB: 2048},
	}
	m.Cursor = wslResourceRowMemory
	return m
}

// TestWSLResourcesFrameFitIsMeasuredAtEverySize prints the WSL resource screen's
// measured frame at the twelve terminals the fit guard uses, so the size it is
// drawn at is a number a reader can re-derive rather than a claim. The rows are
// the data, so they win the budget: at every size each managed value, the write
// row and the way back are on screen, and the prose is what gives way.
func TestWSLResourcesFrameFitIsMeasuredAtEverySize(t *testing.T) {
	for _, size := range measuredTerminalSizes {
		m := wslResourcesFrameCase(t)
		m.Width, m.Height = size.width, size.height
		view := m.View()

		plain := ansiEscape.ReplaceAllString(view, "")
		rows := renderedRowCount(view)
		widest := 0
		for _, line := range strings.Split(plain, "\n") {
			widest = max(widest, lipgloss.Width(line))
		}
		t.Logf("wsl-resources at %s: %d of %d rows, %d of %d columns", size.name, rows, size.height, widest, size.width)

		if rows > size.height {
			t.Errorf("wsl-resources at %s renders %d rows, want <= %d", size.name, rows, size.height)
		}
		if widest > size.width {
			t.Errorf("wsl-resources at %s draws %d columns, want <= %d", size.name, widest, size.width)
		}

		for _, option := range m.GetCurrentOptions() {
			if strings.HasPrefix(option, menuSeparatorPrefix) {
				continue
			}
			if !strings.Contains(plain, option) {
				t.Errorf("wsl-resources at %s dropped the row %q: the data must survive the short frame",
					size.name, option)
			}
		}
	}
}

// shellAuditCaseName is the name the frame guards know the shell startup screen
// by.
const shellAuditCaseName = "shell-audit"

// shellAuditFrameCase builds the shell startup screen with a finished
// measurement, a run that timed out and zprof's table, so the guards measure the
// rows and the prose rather than the still-unmeasured state. The shell and the
// numbers are forced rather than read: a guard that measured the runner's own
// shell would change with the machine.
func shellAuditFrameCase(t *testing.T) Model {
	t.Helper()

	m := installerFrameModel(t, ScreenShellAudit)
	m.ShellAudit = shellAuditTestState()
	m.Cursor = 0
	return m
}

// TestShellAuditFrameFitIsMeasuredAtEverySize prints the shell startup screen's
// measured frame at the twelve terminals the fit guard uses, so the size it is
// drawn at is a number a reader can re-derive rather than a claim. The rows are
// the data, so the guard requires each of them on screen at every size: the
// median and its range, the runs that did not finish, the function the profiler
// named, the row that measures and the way back.
func TestShellAuditFrameFitIsMeasuredAtEverySize(t *testing.T) {
	for _, size := range measuredTerminalSizes {
		m := shellAuditFrameCase(t)
		m.Width, m.Height = size.width, size.height
		view := m.View()

		plain := ansiEscape.ReplaceAllString(view, "")
		rows := renderedRowCount(view)
		widest := 0
		for _, line := range strings.Split(plain, "\n") {
			widest = max(widest, lipgloss.Width(line))
		}
		t.Logf("shell-audit at %s: %d of %d rows, %d of %d columns", size.name, rows, size.height, widest, size.width)

		if rows > size.height {
			t.Errorf("shell-audit at %s renders %d rows, want <= %d", size.name, rows, size.height)
		}
		if widest > size.width {
			t.Errorf("shell-audit at %s draws %d columns, want <= %d", size.name, widest, size.width)
		}

		for _, option := range m.GetCurrentOptions() {
			if strings.HasPrefix(option, menuSeparatorPrefix) {
				continue
			}
			if !strings.Contains(plain, option) {
				t.Errorf("shell-audit at %s dropped the row %q: the data must survive the short frame",
					size.name, option)
			}
		}
	}
}

// TestShellAuditUnavailableFitsEveryTerminalSize measures the other half at the
// same twelve terminals: the state a server, a container or a run with no
// terminal attached sees, where the screen names why nothing can be measured
// rather than offering a row that would fail. The copy that explains the absence
// is rows too, and rows can overflow a frame.
func TestShellAuditUnavailableFitsEveryTerminalSize(t *testing.T) {
	checked := 0
	for _, size := range measuredTerminalSizes {
		size := size
		t.Run(size.name, func(t *testing.T) {
			m := installerFrameModel(t, ScreenShellAudit)
			m.ShellAudit = shellAuditState{
				Resolved: true,
				Reason:   "the login shell is not named: $SHELL is empty, so there is no shell to open and nothing to measure.",
			}
			m.Width, m.Height = size.width, size.height
			assertScreenFitsTerminal(t, "shell-audit-unavailable", size.width, size.height, m.View())
		})
		checked++
	}
	if checked != len(measuredTerminalSizes) {
		t.Fatalf("the guard rendered %d sizes, want %d", checked, len(measuredTerminalSizes))
	}
}

// TestThemePickerFrameFitIsMeasuredAtEverySize prints the theme picker's measured
// frame at the twelve terminals the fit guard uses, so the size it is drawn at is
// a number a reader can re-derive rather than a claim. The assertions repeat the
// guard's two rules on purpose: this is the case that carries the numbers, and a
// measurement that is only logged cannot fail.
// themeRefreshReviewFrameCase builds the refresh review with a short list of
// files and one that cannot be refreshed, so the guards measure the review's
// own rows: the banner, the exact file list, the preserved destinations and the
// confirmation. The long paths are the actual home paths the section names.
func themeRefreshReviewFrameCase(t *testing.T) Model {
	t.Helper()

	m := installerFrameModel(t, ScreenThemePicker)
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	m.DotfilesThemes = defs
	m.ThemeRefreshCandidates = []themeRefreshCandidate{
		{Tool: "zsh", Path: "/home/testuser/.zshrc", Reason: "no dotfiles ownership marker",
			Preserve: "/home/testuser/.zshrc.d", PreserveLabel: "your current file is preserved first in /home/testuser/.zshrc.d/"},
		{Tool: "herdr", Path: "/home/testuser/.config/herdr/config.toml", Reason: "no dotfiles ownership marker",
			PreserveLabel: "your current file is preserved first as /home/testuser/.config/herdr/config.toml.bak-dotfiles-<stamp>"},
		{Tool: "ghostty", Path: "/home/testuser/.config/ghostty/config", Problem: "its content is not a recognizable dotfiles theme block"},
	}
	m.ThemeRefreshReview = true
	m.Cursor = 0
	return m
}

func TestThemePickerFrameFitIsMeasuredAtEverySize(t *testing.T) {
	for _, size := range measuredTerminalSizes {
		m := themePickerFrameCase(t)
		m.Width, m.Height = size.width, size.height
		view := m.View()

		rows := renderedRowCount(view)
		widest := 0
		for _, line := range strings.Split(ansiEscape.ReplaceAllString(view, ""), "\n") {
			widest = max(widest, lipgloss.Width(line))
		}
		t.Logf("theme picker at %s: %d of %d rows, %d of %d columns", size.name, rows, size.height, widest, size.width)

		if rows != size.height {
			t.Errorf("the theme picker at %s renders %d rows, want exactly %d", size.name, rows, size.height)
		}
		if widest != size.width {
			t.Errorf("the theme picker at %s draws %d columns, want exactly %d", size.name, widest, size.width)
		}

		// The property this guard holds changed with the list. Before, every row had
		// to fit the frame at once; now the picker windows a longer list around the
		// cursor (listWindow), so the property is reachability: every theme row, the
		// undo row and the way back are reached by moving the cursor, the row under
		// the cursor is the one drawn, and the frame still fits the terminal exactly.
		// A row that cannot be reached, or a frame that grows to hold one, fails.
		options := m.GetCurrentOptions()
		visited := make([]bool, len(options))
		for i, option := range options {
			if strings.HasPrefix(option, menuSeparatorPrefix) {
				continue
			}
			m.Cursor = i
			plain := ansiEscape.ReplaceAllString(m.View(), "")
			label := truncate(option, layoutFor(m).RowMeasure-2)
			selected := ""
			for _, line := range strings.Split(plain, "\n") {
				if strings.Contains(line, "▸") {
					selected = line
					break
				}
			}
			if selected == "" {
				t.Fatalf("the theme picker at %s: no row carries the cursor marker (cursor %d of %d)", size.name, i, len(options)-1)
			}
			if !strings.Contains(selected, label) {
				t.Errorf("the theme picker at %s: the row under the cursor is not %q (cursor %d): %q", size.name, label, i, selected)
			}
			visited[i] = true
		}
		for i, option := range options {
			if strings.HasPrefix(option, menuSeparatorPrefix) {
				continue
			}
			if !visited[i] {
				t.Errorf("the theme picker at %s: the row %q (index %d) was never reached by the cursor", size.name, option, i)
			}
		}
	}
}

// themePickerScrollModel builds the picker with count complete themes and a
// recorded change, so its list holds the themes, the undo row and the way back,
// and the cursor starts on the first theme row with its preview live.
func themePickerScrollModel(t *testing.T, count int) Model {
	t.Helper()
	m := installerFrameModel(t, ScreenThemePicker)
	m.DotfilesThemes = scrollThemeDefinitions(count)
	m.DotfilesThemeRecord = &dotfilesThemeRecord{Theme: "scroll-00"}
	m.Cursor = 0
	return m
}

// scrollThemeDefinitions builds count complete definitions with distinct names so
// the picker derives count theme rows. The guard needs more themes than either
// floor can hold, and the repository ships six, so the extra ones are built here
// rather than added to themes/. They carry the canonical palette and the
// [syntax] members the preview reads, so they are complete and offered; with no
// artifact table they name every tool they cannot paint, which is exactly what a
// definition the repository does not ship would say.
func scrollThemeDefinitions(count int) []themeDefinition {
	palette := make(map[string]string, len(themePaletteRoles))
	for _, role := range themePaletteRoles {
		palette[role] = "#336699"
	}
	syntax := make(map[string]string, len(themeSyntaxRequired))
	for _, role := range themeSyntaxRequired {
		syntax[role] = "#336699"
	}
	defs := make([]themeDefinition, 0, count)
	for i := 0; i < count; i++ {
		defs = append(defs, themeDefinition{
			ID:      fmt.Sprintf("scroll-%02d", i),
			Name:    fmt.Sprintf("Scroll Theme %02d", i),
			Palette: palette,
			Syntax:  syntax,
		})
	}
	return defs
}

// TestThemePickerScrollsToEveryRowAtTheSmallTerminals is the guard for the
// defect a growing theme list exposed: the picker drew every row it had, so the
// frame -- not the palette -- was the limit on how many themes could be offered,
// and one row too many ran off the bottom of a 60x20 terminal.
//
// It builds more complete themes than either floor can hold, then walks the
// cursor through the whole list with the real key handler and asserts, at every
// step and both sizes, that the frame still fits the terminal exactly, that the
// row under the cursor is the one drawn with the marker, that the preview follows
// the cursor onto every theme row, and that a windowed list names the slice it is
// showing. Between them those assertions prove every theme, the undo row and the
// way back are reachable and visible however long the list grows.
func TestThemePickerScrollsToEveryRowAtTheSmallTerminals(t *testing.T) {
	const themeCount = 24 // more than either floor can hold, preview included
	sizes := []struct {
		name          string
		width, height int
	}{
		{"60x20", 60, 20},
		{"80x24", 80, 24},
	}

	for _, size := range sizes {
		size := size
		t.Run(size.name, func(t *testing.T) {
			m := themePickerScrollModel(t, themeCount)
			m.Width, m.Height = size.width, size.height
			options := m.GetCurrentOptions()

			visited := map[int]bool{}
			for step := 0; ; step++ {
				view := m.View()
				plain := ansiEscape.ReplaceAllString(view, "")

				// The frame still fits the terminal exactly, even while the list is
				// windowed: the list scrolls, the frame does not grow.
				if rows := renderedRowCount(view); rows != size.height {
					t.Errorf("%s at step %d renders %d rows, want exactly %d: the frame must not grow with the list",
						size.name, step, rows, size.height)
				}
				widest := 0
				for _, line := range strings.Split(plain, "\n") {
					widest = max(widest, lipgloss.Width(line))
				}
				if widest != size.width {
					t.Errorf("%s at step %d draws %d columns, want exactly %d", size.name, step, widest, size.width)
				}

				// The row under the cursor is the one carrying the marker, so a
				// scrolled window can never hide the selection.
				label := truncate(options[m.Cursor], layoutFor(m).RowMeasure-2)
				selected := ""
				for _, line := range strings.Split(plain, "\n") {
					if strings.Contains(line, "▸") {
						selected = line
						break
					}
				}
				if selected == "" {
					t.Fatalf("%s at step %d: no row carries the cursor marker, so the highlighted row is off screen (cursor %d of %d)",
						size.name, step, m.Cursor, len(options)-1)
				}
				if !strings.Contains(selected, label) {
					t.Errorf("%s at step %d: the highlighted row is not %q (cursor %d): %q",
						size.name, step, label, m.Cursor, selected)
				}
				visited[m.Cursor] = true

				// The preview follows the cursor onto every theme row, including the
				// ones the first screen cannot show.
				if strings.HasPrefix(options[m.Cursor], "Apply the ") {
					if !strings.Contains(plain, "Preview (nothing applied)") {
						t.Errorf("%s at step %d: the cursor is on the theme row %q but no preview row is drawn",
							size.name, step, options[m.Cursor])
					}
				}

				if step == 0 {
					// The guard is not vacuous: the list really is longer than the
					// first screen, and it says so with a visible-range count.
					visible := strings.Count(plain, "Apply the ")
					t.Logf("%s: windowed first screen shows %d theme rows; %d menu entries total", size.name, visible, len(options))
					if visible >= themeCount {
						t.Fatalf("%s: the first screen shows %d theme rows, want fewer than the %d built: the guard is not exercising a windowed list",
							size.name, visible, themeCount)
					}
					if !strings.Contains(plain, fmt.Sprintf("of %d", len(options))) {
						t.Errorf("%s: the windowed list draws no visible-range count in the header: %q", size.name, plain)
					}
				}

				before := m.Cursor
				next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
				m = next.(Model)
				if m.Cursor == before {
					break
				}
			}

			// Every datum was reached by the cursor, not only the first screen's.
			for i, option := range options {
				if strings.HasPrefix(option, menuSeparatorPrefix) {
					continue
				}
				if !visited[i] {
					t.Errorf("%s: the row %q (index %d) was never under the cursor while walking the list", size.name, option, i)
				}
			}
		})
	}
}

// TestMainMenuPlanPanelLeavesRoomForTheCompanion is the guard for the reported
// defect: on the main menu's Start Installation option the plan panel is tall
// enough that the frame's spare rows cannot hold both it and the creature, so
// the pet disappears at the sizes where the panel crowds it out.
//
// It renders the main menu and its restore variant at the same twelve sizes the
// frame guards use, and asserts the pair the report is about:
//
//   - the creature draws wherever the terminal selects a rung (the ladder is a
//     pure function of the terminal, so a selected rung the frame refuses is the
//     defect, not a smaller creature);
//   - the panel still answers what will happen: the plan's step count is on
//     screen, and where the panel has a second column the destructive facts it
//     holds (the overwrites and the newest backup) are on screen too.
//
// The two are not always possible together: at a short terminal the summary's
// own row plus the rung can exceed the rows the body did not need. Those sizes
// are recorded with their numbers rather than silently dropped, so a size that
// cannot hold both is a measured finding and a size that can is held to the
// assertion.
func TestMainMenuPlanPanelLeavesRoomForTheCompanion(t *testing.T) {
	// The sizes where the body, the selected rung and the panel summary genuinely
	// cannot fit together, with the numbers that say so. A size enters this list
	// only with its arithmetic checked below; the guard still checks that its panel
	// answers, so a listed size cannot hide a lost plan.
	type impossibleCase struct {
		body, rung, summary, budget int
	}
	impossible := map[string]impossibleCase{
		// Both variants now spend four chrome rows (the title, the greeting, the
		// question and the blank under it) plus their options, and the Utilities
		// row is one of them. In a 14-row frame the summary keeps its row (a fact
		// beats a decoration) and there is no room left for the creature: 11 + 3 +
		// 1 = 15 for the plain menu, 12 + 3 + 1 = 16 with the restore row.
		"main-menu/60x20":         {body: 11, rung: 3, summary: 1, budget: 14},
		"main-menu-restore/60x20": {body: 12, rung: 3, summary: 1, budget: 14},
	}

	for _, name := range []string{"main-menu", "main-menu-restore"} {
		for _, size := range measuredTerminalSizes {
			size := size
			t.Run(name+"/"+size.name, func(t *testing.T) {
				m := installerFrameCase(t, name)
				m.Width, m.Height = size.width, size.height
				m.Animating, m.PixelSprite = true, true
				m.ink = companionInkFor(true)

				view := m.View()
				if rows := renderedRowCount(view); rows != m.Height {
					t.Errorf("the frame renders %d rows at %s, want the terminal's %d: reserving the creature's block must not grow the frame",
						rows, size.name, m.Height)
				}
				l := layoutFor(m)
				budget := installerBodyRows(m.Height, footerRowCount(l.Inner, m.panelHints(m.panelsFor(), mainMenuHints())))
				panelRows := len(m.panelColumn(m.panelsFor(), l, budget))
				if !l.TwoColumn {
					panelRows = len(m.rotatorLines(m.panelsFor(), l.Inner))
				}
				rung := m.companionHeightNow()
				_, drawn := trainerViewCompanionArt(view)
				t.Logf("%-18s %-7s two=%-5v budget=%2d panel=%2d rung=%2d drawn=%2d",
					name, size.name, l.TwoColumn, budget, panelRows, rung, drawn)

				if !planPanelAnswers(m, view) {
					t.Errorf("the panel no longer answers what will happen at %s: the plan's step count is not on screen", size.name)
				}
				if l.TwoColumn && !mainMenuDestructiveFactsVisible(m, view) {
					t.Errorf("the two-column panel dropped a destructive fact at %s:\n%s", size.name, ansiEscape.ReplaceAllString(view, ""))
				}

				if c, known := impossible[name+"/"+size.name]; known {
					if c.body+c.rung+c.summary <= c.budget {
						t.Errorf("%s is recorded as unable to hold both, but %d body + %d rung + %d summary = %d fits the %d-row budget",
							size.name, c.body, c.rung, c.summary, c.body+c.rung+c.summary, c.budget)
					}
					if drawn != 0 {
						t.Errorf("%s is recorded as unable to hold both but drew %d creature rows", size.name, drawn)
					}
					return
				}
				if rung < 1 {
					t.Skipf("no rung is selected at %s, so the guard proves nothing", size.name)
				}
				if drawn == 0 {
					t.Errorf("no creature is drawn at %s though the rung is %d: budget=%d panel=%d\n%s",
						size.name, rung, budget, panelRows, ansiEscape.ReplaceAllString(view, ""))
				} else if drawn != rung {
					t.Errorf("the creature draws %d rows at %s, want the selected rung %d", drawn, size.name, rung)
				}
			})
		}
	}
}

// mainMenuHints is the footer the main menu hands the frame, so the guard's row
// budget is the frame's own.
func mainMenuHints() []installerHint {
	return []installerHint{hintUp, hintDown, hintSelect, hintQuit}
}

// TestMainMenuShowsStartInstallationAtEveryTerminalSize pins, as a pure layout
// fact, the property the macOS smoke test asserts: at every terminal the frame
// guards measure, the main menu draws its first option on screen. The menu grew
// recently -- the Utilities row, then the rows the panel summary and the
// creature reserve -- so a size where "Start Installation" fell off the bottom
// would be a menu defect, not a test defect. The failed macOS run of
// TestMainMenuWithRestoreOption looked like that defect but was the output-wait
// race; this guard keeps the two distinguishable.
func TestMainMenuShowsStartInstallationAtEveryTerminalSize(t *testing.T) {
	for _, name := range []string{"main-menu", "main-menu-restore"} {
		for _, size := range measuredTerminalSizes {
			size := size
			t.Run(name+"/"+size.name, func(t *testing.T) {
				m := installerFrameCase(t, name)
				m.Width, m.Height = size.width, size.height
				// The companion is the strictest frame: it draws in the rows the body
				// did not need, and the option must survive it.
				m.Animating, m.PixelSprite = true, true
				m.ink = companionInkFor(true)

				view := ansiEscape.ReplaceAllString(m.View(), "")
				if !strings.Contains(view, "Start Installation") {
					t.Errorf("the main menu at %s (%s) does not draw its first option: the body, the panel summary and the creature took the row:\n%s",
						size.name, name, view)
				}
			})
		}
	}
}

// planPanelAnswers reports whether the rendered screen still states the plan's
// step count. It reads the visible words, so it holds on a terminal with no
// colour and on both the two-column panel and the narrow summary.
func planPanelAnswers(m Model, view string) bool {
	steps, _ := m.previewPlan()
	if len(steps) == 0 {
		return false
	}
	flat := strings.Join(strings.Fields(ansiEscape.ReplaceAllString(view, "")), " ")
	return strings.Contains(flat, stepCount(len(steps))) ||
		strings.Contains(flat, fmt.Sprintf("Steps %d", len(steps)))
}

// mainMenuDestructiveFactsVisible reports whether the panel still names the
// facts a reader must not be surprised by: the configs the run will overwrite and
// the newest backup it holds. It asserts only on the two-column panel, where
// those rows live; the narrow summary has one line and says the count.
func mainMenuDestructiveFactsVisible(m Model, view string) bool {
	flat := strings.Join(strings.Fields(ansiEscape.ReplaceAllString(view, "")), " ")
	if len(m.ExistingConfigs) > 0 && !strings.Contains(flat, "Overwrites") {
		return false
	}
	if _, ok := newestBackup(m.AvailableBackups); ok && !strings.Contains(flat, "Newest backup") {
		return false
	}
	return true
}
