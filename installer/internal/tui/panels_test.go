package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/albersg/dotfiles/installer/internal/system"
	"github.com/albersg/dotfiles/installer/internal/tui/trainer"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// narrowPanelLayout is the narrowest column a panel can be given: the first
// content width the layout composes at. A panel test measures against this rather
// than against the 160-column composition, because the narrow column is where a
// long value, a long label and a crowded panel all show up first.
func narrowPanelLayout() layout {
	return layoutFor(Model{Width: layoutTwoColumnWidth + 4})
}

// panelText strips a panel's styling and joins its rows, so a test reads the
// words a reader sees instead of the escape sequences they are wrapped in. The
// assertions are on the words on purpose: the panel's meaning has to survive a
// terminal with no colour at all.
func panelText(rows []string) string {
	plain := make([]string, len(rows))
	for i, row := range rows {
		plain[i] = ansiEscape.ReplaceAllString(row, "")
	}
	return strings.Join(plain, "\n")
}

// panelFlat is panelText with the wrapping undone: its rows joined by single
// spaces. A value long enough to wrap is one sentence across several rows, so an
// assertion about the sentence reads it flat, while an assertion about the layout
// reads panelText.
func panelFlat(rows []string) string {
	return strings.Join(strings.Fields(panelText(rows)), " ")
}

// assertPanelFits checks the two bounds every panel has to respect: no row wider
// than its column, and no more rows than the frame left for it.
func assertPanelFits(t *testing.T, rows []string, width, budget int) {
	t.Helper()
	if len(rows) > budget {
		t.Errorf("the panel is %d rows in a %d-row budget:\n%s", len(rows), budget, panelText(rows))
	}
	for _, row := range rows {
		if got := lipgloss.Width(row); got > width {
			t.Errorf("a panel row is %d columns in a %d-column panel: %q", got, width, row)
		}
	}
}

// TestWelcomePanelNamesTheMachineItWasGiven pins the machine panel against a
// pinned host: every row comes from a field system.Detect fills, and the package
// manager is the one the dependency step itself would dispatch through.
func TestWelcomePanelNamesTheMachineItWasGiven(t *testing.T) {
	l := narrowPanelLayout()
	m := Model{SystemInfo: &system.SystemInfo{
		OS:         system.OSDebian,
		OSName:     "Debian/Ubuntu",
		Arch:       "arm64",
		IsWSL:      true,
		WSLVersion: 2,
		IsARM:      true,
		UserShell:  "zsh",
		HomeDir:    "/home/testuser",
	}}

	rows := m.welcomePanel(l, 40)
	assertPanelFits(t, rows, l.Right, 40)
	text := panelText(rows)

	for _, want := range []string{
		welcomePanelLabel,
		"OS", "Debian/Ubuntu",
		"Host", "WSL 2",
		"Arch", "arm64",
		"Shell", "zsh",
		"Packages", "apt-get",
		"Home", "/home/testuser",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the machine panel does not name %q:\n%s", want, text)
		}
	}

	if strings.Contains(text, "Xcode") {
		t.Errorf("a Linux host got an Xcode row, though detection never probed the tools there:\n%s", text)
	}
}

// TestWelcomePanelOmitsFactsTheModelDoesNotHave pins the panel's honesty rule: a
// value the model does not hold is left out, not filled in with a guess or with
// the word "unknown". The pinned Linux host is not ARM, not WSL and not macOS, so
// neither of those rows may appear, and no row may claim a fact nobody measured.
func TestWelcomePanelOmitsFactsTheModelDoesNotHave(t *testing.T) {
	l := narrowPanelLayout()
	m := Model{SystemInfo: goldenSystemInfo()}

	rows := m.welcomePanel(l, 40)
	assertPanelFits(t, rows, l.Right, 40)
	text := panelText(rows)

	for _, absent := range []string{"WSL", "Termux", "Xcode", "ARM", "unknown", "Unknown", "none", "n/a"} {
		if strings.Contains(text, absent) {
			t.Errorf("the machine panel claims %q, which the model does not hold:\n%s", absent, text)
		}
	}
	for _, want := range []string{"OS", "Linux", "Shell", "zsh", "Packages", "apt-get", "Home", "/home/testuser"} {
		if !strings.Contains(text, want) {
			t.Errorf("the machine panel is missing %q:\n%s", want, text)
		}
	}
}

// TestWelcomePanelWrapsLongValuesRatherThanClipping pins the row contract: a
// value wider than the panel wraps onto the lines under it, indented under the
// value column, so nothing is dropped at the gutter.
func TestWelcomePanelWrapsLongValuesRatherThanClipping(t *testing.T) {
	l := narrowPanelLayout()
	const home = "/home/a/user/with/a/really/long/path/that/cannot/fit/one/panel/row/at/all"
	info := goldenSystemInfo()
	info.HomeDir = home
	m := Model{SystemInfo: info}

	rows := m.welcomePanel(l, 40)
	assertPanelFits(t, rows, l.Right, 40)

	// The value is wrapped, not cut: every segment is still on screen in order.
	joined := strings.Join(strings.Fields(panelText(rows)), "")
	if !strings.Contains(joined, home) {
		t.Errorf("the home directory was dropped instead of wrapped:\n%s", panelText(rows))
	}
}

// TestWillHappenPanelMarksWhereTheRunStarts pins the plan panel when the model
// holds a plan: the count, the ▸ the menus already use for "here" on the step
// the run starts at, and that step's name and description. The panel is a glance,
// so the steps after the next one are counted rather than listed -- their names
// and descriptions live on the installing screen, where each step is read as it
// runs -- but the count says how many there are and the marked step says which
// one is next. While nothing has run, that is the first step; mid-run it is the
// step that is running, because that is the next action.
func TestWillHappenPanelMarksWhereTheRunStarts(t *testing.T) {
	l := narrowPanelLayout()
	steps := []InstallStep{
		{Name: "Backup Existing Configs", Description: "Saves a copy of your current configuration first."},
		{Name: "Install Dependencies", Description: "Installs base packages with your distribution's package manager."},
		{Name: "Clone Repository", Description: "Downloads your dotfiles repository."},
	}

	// assertOnlyThisStep is the rule the panel exists for: the count says how many
	// steps there are, the marked step is named, and exactly that step carries its
	// paragraph. The descriptions wrap inside the narrow column, so a present
	// sentence is asserted on the flat text.
	assertOnlyThisStep := func(t *testing.T, text, flat string, current int) {
		t.Helper()
		if !strings.Contains(flat, "Steps 3") {
			t.Errorf("the plan panel does not count the steps:\n%s", text)
		}
		if !strings.Contains(flat, steps[current].Name) {
			t.Errorf("the plan panel does not name the step the run is on (%q):\n%s", steps[current].Name, text)
		}
		for i, step := range steps {
			has := strings.Contains(flat, step.Description)
			if want := i == current; has != want {
				t.Errorf("the plan panel shows the description of step %d = %v, want %v:\n%s", i+1, has, want, text)
			}
		}
	}

	t.Run("nothing has started, so the first step is where it starts", func(t *testing.T) {
		m := Model{Steps: steps}
		rows := m.mainMenuPanel(l, 40)
		assertPanelFits(t, rows, l.Right, 40)
		text := panelText(rows)
		flat := panelFlat(rows)

		assertOnlyThisStep(t, text, flat, 0)
		if got := strings.Count(text, "▸"); got != 1 {
			t.Errorf("the plan panel marks %d steps as where the run starts, want exactly one:\n%s", got, text)
		}
		if !strings.Contains(text, "▸ 1") {
			t.Errorf("the plan panel marks the wrong step as where the run starts:\n%s", text)
		}
	})

	t.Run("a run in progress marks the step it is on", func(t *testing.T) {
		running := append([]InstallStep{}, steps...)
		running[0].Status = StatusDone
		running[1].Status = StatusRunning
		m := Model{Steps: running, CurrentStep: 1}

		rows := m.mainMenuPanel(l, 40)
		assertPanelFits(t, rows, l.Right, 40)
		text := panelText(rows)

		assertOnlyThisStep(t, text, panelFlat(rows), 1)
		if !strings.Contains(text, "▸ 2") {
			t.Errorf("the plan panel does not mark the running step:\n%s", text)
		}
		if strings.Contains(text, "▸ 1") {
			t.Errorf("the plan panel still marks the finished first step as where the run starts:\n%s", text)
		}
	})
}

// TestStepPanelRowsNumberTheWholePlan pins the list shape directly: one row per
// name underneath the number column, the marker in that same column so marking a
// step does not shift it, and every row inside the column it was handed. A step
// count that grows past nine widens the column rather than letting the numbers
// run into the names.
func TestStepPanelRowsNumberTheWholePlan(t *testing.T) {
	steps := make([]InstallStep, 10)
	for i := range steps {
		steps[i] = InstallStep{Name: fmt.Sprintf("Step name %d", i+1)}
	}

	const width = 40
	rows := stepPanelRows(steps, 0, width)
	if len(rows) != len(steps) {
		t.Fatalf("stepPanelRows returned %d rows for %d steps with no descriptions:\n%s", len(rows), len(steps), panelText(rows))
	}

	// Every name starts on the same column and every number ends on the same
	// column: the marker has a column of its own, so marking a step shifts
	// neither, and a two-digit plan right-aligns its digits in a column of its own
	// instead of running them into the names.
	nameColumn, numberEdge := -1, -1
	for _, row := range rows {
		if got := lipgloss.Width(row); got > width {
			t.Errorf("a step row is %d columns in a %d-column panel: %q", got, width, row)
		}
		plain := ansiEscape.ReplaceAllString(row, "")
		at := strings.Index(plain, "Step name")
		if at < 0 {
			t.Fatalf("a step row does not name its step: %q", plain)
		}
		// Compare the COLUMN, not the byte offset: the marker glyph is one column
		// and three bytes, so a byte offset would call the marked row misaligned.
		column := lipgloss.Width(plain[:at])
		if nameColumn < 0 {
			nameColumn = column
		} else if column != nameColumn {
			t.Errorf("a step name starts at column %d, want %d: %q", column, nameColumn, plain)
		}

		// The first digit run in the row is the step's number; the name's own
		// digits come later. Its trailing edge is the same column on every row, so
		// a ten-step plan keeps a straight edge under the marker instead of a
		// ragged one.
		start := strings.IndexFunc(plain, func(r rune) bool { return r >= '0' && r <= '9' })
		if start < 0 {
			t.Fatalf("a step row does not number its step: %q", plain)
		}
		end := start
		for end < len(plain) && plain[end] >= '0' && plain[end] <= '9' {
			end++
		}
		edge := lipgloss.Width(plain[:end])
		if numberEdge < 0 {
			numberEdge = edge
		} else if edge != numberEdge {
			t.Errorf("a step number ends at column %d, want %d: %q", edge, numberEdge, plain)
		}
	}

	text := panelText(rows)
	if got := strings.Count(text, "▸"); got != 1 {
		t.Errorf("the step list marks %d steps, want exactly one:\n%s", got, text)
	}
	if !strings.Contains(panelFlat(rows), "▸ 1 Step name 1") {
		t.Errorf("the step list does not mark the first step:\n%s", text)
	}
	if !strings.Contains(text, "10") {
		t.Errorf("the step list does not number the last step:\n%s", text)
	}
}

// TestWillHappenPanelOmitsTheSectionsItHasNotScanned pins what the panel may say
// for a model with no detected host, where there is no OS to describe and so no
// plan to build. It shows nothing about the plan, the configs or the backups:
// "none" would be a claim the model cannot support, because nothing was
// detected. A model that does carry a host gets the plan instead, which
// TestWillHappenPanelPreviewsThePlan pins.
func TestWillHappenPanelOmitsTheSectionsItHasNotScanned(t *testing.T) {
	l := narrowPanelLayout()
	m := Model{}

	rows := m.mainMenuPanel(l, 40)
	assertPanelFits(t, rows, l.Right, 40)
	text := panelText(rows)

	if !strings.Contains(text, mainMenuPanelLabel) {
		t.Errorf("the panel does not name itself:\n%s", text)
	}
	for _, absent := range []string{"Steps", "Overwrites", "Newest backup", "none", "None", "unknown", "0 configs", "0 files", "(detected)"} {
		if strings.Contains(text, absent) {
			t.Errorf("the plan panel claims %q for state nothing has filled in:\n%s", absent, text)
		}
	}
}

// TestWillHappenPanelPreviewsThePlan pins the fix for the empty panel: on the
// main menu, before the wizard has built the plan, the panel still shows the
// plan the run would execute, built by the wizard's own builder from the
// detected host. It labels which host it is describing, because the OS question
// has not been asked yet.
// TestWillHappenPanelPreviewsThePlan pins the fix for the empty panel: on the
// main menu, before the wizard has built the plan, the panel still shows the
// plan the run would execute, built by the wizard's own builder from the
// detected host. It labels which host it is describing, because the OS question
// has not been asked yet. The plan is a glance: the count and the step the run
// would start at, not a row per step.
func TestWillHappenPanelPreviewsThePlan(t *testing.T) {
	l := narrowPanelLayout()
	m := Model{SystemInfo: goldenSystemInfo()}

	rows := m.mainMenuPanel(l, 40)
	assertPanelFits(t, rows, l.Right, 40)
	text := panelText(rows)
	flat := panelFlat(rows)

	for _, want := range []string{
		"on Linux (detected)",
		"Steps 8",
		"Install Dependencies",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("the previewed plan does not show %q:\n%s", want, text)
		}
	}
	// The steps after the next one are counted rather than listed: the panel is a
	// glance and the installing screen is where each step's name is read.
	for _, later := range []string{"Clone Repository", "Install Homebrew", "Set Default Shell", "Cleanup"} {
		if strings.Contains(flat, later) {
			t.Errorf("the previewed plan lists %q though the panel shows the next step only:\n%s", later, text)
		}
	}
	if got := strings.Count(text, "▸"); got != 1 {
		t.Errorf("the previewed plan marks %d steps as where the run starts, want exactly one:\n%s", got, text)
	}
	if !strings.Contains(text, "▸ 1") {
		t.Errorf("the previewed plan does not mark the first step as where the run starts:\n%s", text)
	}
	// A choice that has not been made is not in the plan: the shell step has no
	// name until the player picks one.
	if strings.Contains(flat, "Install  ") {
		t.Errorf("the previewed plan shows a step whose choice is still open:\n%s", text)
	}
}

// TestWillHappenPanelDropsTheHostLineWhenTheChoiceMatchesDetection pins the
// other half of the labelling rule: once the player has chosen the OS detection
// already found, the plan is for the chosen host and the note is dropped.
func TestWillHappenPanelDropsTheHostLineWhenTheChoiceMatchesDetection(t *testing.T) {
	l := narrowPanelLayout()
	m := Model{SystemInfo: goldenSystemInfo(), Choices: UserChoices{OS: "linux"}}

	text := panelFlat(m.mainMenuPanel(l, 40))
	if strings.Contains(text, "(detected)") {
		t.Errorf("the panel still labels a host the player has chosen:\n%s", text)
	}
	if !strings.Contains(text, "Install Dependencies") {
		t.Errorf("the panel does not show the chosen Linux host's plan:\n%s", text)
	}
}

// TestWillHappenPanelNamesTheChosenHostWhenItDiffersFromDetection pins that the
// panel never describes a plan for a host the run will not use: when the choice
// differs from detection, the plan follows the choice and the note names it.
func TestWillHappenPanelNamesTheChosenHostWhenItDiffersFromDetection(t *testing.T) {
	l := narrowPanelLayout()
	m := Model{SystemInfo: goldenSystemInfo(), Choices: UserChoices{OS: "mac"}}

	text := panelFlat(m.mainMenuPanel(l, 40))
	if !strings.Contains(text, "on macOS") {
		t.Errorf("the panel does not name the chosen host:\n%s", text)
	}
	if strings.Contains(text, "Install Dependencies") {
		t.Errorf("the panel shows the Linux plan for a macOS choice:\n%s", text)
	}
	if !strings.Contains(text, "Install Xcode CLI") {
		t.Errorf("the panel does not show the chosen macOS host's plan:\n%s", text)
	}
}

// TestWillHappenPanelOmitsTheBackupRowWithoutBackups pins the omitting rule for
// the last row: a model with no backups shows no backup row, rather than a row
// claiming there is none. The presence half is pinned by
// TestWillHappenPanelShowsTheNewestBackupAndItsAge.
func TestWillHappenPanelOmitsTheBackupRowWithoutBackups(t *testing.T) {
	l := narrowPanelLayout()
	m := Model{SystemInfo: goldenSystemInfo()}

	rows := m.mainMenuPanel(l, 40)
	assertPanelFits(t, rows, l.Right, 40)
	text := panelText(rows)

	if strings.Contains(text, "Newest backup") {
		t.Errorf("the plan panel invents a backup row for a model with none:\n%s", text)
	}
	if strings.Contains(text, "Overwrites") {
		t.Errorf("the plan panel invents an overwrite row for a model with none:\n%s", text)
	}
}

// TestPlanForLeavesOutAStepWhoseChoiceIsOpen pins the one place the shared
// builder is not a pure move of SetupInstallSteps: the shell has no "skip"
// option, so the wizard always has one and its plan is unchanged, while the main
// menu's preview runs before the question and leaves the nameless step out
// instead of showing "Install ".
func TestPlanForLeavesOutAStepWhoseChoiceIsOpen(t *testing.T) {
	open := planFor(planOptions{OS: "linux"})
	for _, step := range open {
		if step.ID == "shell" {
			t.Errorf("planFor planned a shell step with no shell chosen: %q", step.Name)
		}
	}

	chosen := planFor(planOptions{OS: "linux", Shell: "fish"})
	found := false
	for _, step := range chosen {
		if step.ID == "shell" {
			found = true
			if step.Name != "Install fish" {
				t.Errorf("shell step name = %q, want %q", step.Name, "Install fish")
			}
		}
	}
	if !found {
		t.Error("planFor dropped the shell step once a shell was chosen")
	}
}

// TestWizardPanelShowsNoSelectionInsteadOfAPlan pins the answer to an open
// question: when the cursor is noSelection -- PR #143 opens a step with nothing
// highlighted rather than inventing macOS -- the panel must not preview a plan
// for a choice nobody made. The old path ran the out-of-range cursor through
// wizardHighlightedChoice's default, which meant "Linux", so the panel showed a
// Linux plan with no Selected row: no detection claim, but an invented answer.
func TestWizardPanelShowsNoSelectionInsteadOfAPlan(t *testing.T) {
	l := narrowPanelLayout()
	cases := []struct {
		name   string
		screen Screen
		info   *system.SystemInfo
		opts   UserChoices
	}{
		{"an undetected OS step", ScreenOSSelect, &system.SystemInfo{OS: system.OSUnknown}, UserChoices{}},
		{"an undetected shell step", ScreenShellSelect, &system.SystemInfo{OS: system.OSLinux}, UserChoices{OS: "linux"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := Model{Screen: c.screen, Cursor: noSelection, SystemInfo: c.info, Choices: c.opts}
			rows := m.contextPanelFacts(l)
			assertPanelFits(t, rows, l.Right, 40)
			flat := panelFlat(rows)
			for _, invented := range []string{"Steps", "Step ", "Install Dependencies", "on Linux"} {
				if strings.Contains(flat, invented) {
					t.Errorf("the panel of an unanswered question previews a plan (%q):\n%s", invented, panelText(rows))
				}
			}
			if !strings.Contains(flat, noChoiceNote) {
				t.Errorf("the panel does not say the question is unanswered, want %q:\n%s", noChoiceNote, panelText(rows))
			}
			if headline := m.contextHeadline(); headline != "" {
				t.Errorf("the narrow summary answers an unanswered question with %q, want no line", headline)
			}
			// The choice reader must not invent a platform for a cursor with no row
			// under it either: the panel guard is the user-facing half, this is the
			// data half. The shell case has a recorded Linux choice, so only the OS
			// case can tell an invented answer from a recorded one.
			if c.screen == ScreenOSSelect {
				if opts, _ := m.wizardHighlightedChoice(); opts.OS != c.opts.OS {
					t.Errorf("wizardHighlightedChoice changed the recorded OS %q to %q for a cursor with no row under it", c.opts.OS, opts.OS)
				}
			}
		})
	}
}

// TestMainMenuShowsTheTrainerSaveWarningItCarried pins the landing half of the
// ten trainer save sites: escape from the trainer menu and q on it both save and
// then go to the main menu, and a failed save leaves the warning in
// TrainerMessage. The trainer's own screens render that field; the main menu did
// not, so the player who lost progress was told on a screen they had already
// left. The warning is shown as one line there, and it fits.
func TestMainMenuShowsTheTrainerSaveWarningItCarried(t *testing.T) {
	for _, site := range []struct {
		name string
		key  tea.KeyMsg
	}{
		{"escape from the trainer menu", tea.KeyMsg{Type: tea.KeyEsc}},
		{"q on the trainer menu", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}},
	} {
		t.Run(site.name, func(t *testing.T) {
			m := failingTrainerSaveModel(t)
			m.Screen = ScreenTrainerMenu

			res, _ := m.Update(site.key)
			got := res.(Model)
			if got.Screen != ScreenMainMenu {
				t.Fatalf("the key did not land on the main menu: screen = %v", got.Screen)
			}
			if !strings.Contains(got.TrainerMessage, trainerSaveWarning) {
				t.Fatalf("the save warning was lost before it reached the main menu: %q", got.TrainerMessage)
			}

			view := ansiEscape.ReplaceAllString(got.View(), "")
			if !strings.Contains(view, trainerSaveWarning) {
				t.Errorf("the main menu does not show the trainer save warning the player was carried to:\n%s", view)
			}
			if rows := renderedRowCount(view); rows > got.Height {
				t.Errorf("the warning grew the main menu to %d rows in a %d-row terminal:\n%s", rows, got.Height, view)
			}
		})
	}
}

// TestMainMenuTrainerSaveWarningClearsOnTheNextKey pins the transient half: the
// warning is a status line the main menu shows once, not a log it accumulates.
// The next key clears it, the way a key clears TrainerMessage on the trainer's own
// screens.
func TestMainMenuTrainerSaveWarningClearsOnTheNextKey(t *testing.T) {
	m := failingTrainerSaveModel(t)
	m.Screen = ScreenTrainerMenu

	res, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	landed := res.(Model)
	if !strings.Contains(landed.TrainerMessage, trainerSaveWarning) {
		t.Fatalf("setup: the warning did not reach the main menu: %q", landed.TrainerMessage)
	}

	next, _ := landed.Update(tea.KeyMsg{Type: tea.KeyDown})
	if got := next.(Model).TrainerMessage; got != "" {
		t.Errorf("the next key left the trainer save warning on the main menu: %q", got)
	}
}

// TestTrainerMenuQuitDoesNotCarryAStaleMessageToTheMainMenu pins the clear q does
// before its save: the main menu shows a save warning, not whatever the trainer
// was last saying. Without the clear, a successful save would carry the trainer's
// ordinary feedback onto the main menu as if it were about the save.
func TestTrainerMenuQuitDoesNotCarryAStaleMessageToTheMainMenu(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // a writable home, so the save succeeds
	m := NewModel()
	m.Screen = ScreenTrainerMenu
	m.TrainerStats = trainer.NewUserStats()
	m.TrainerModules = trainer.GetAllModules()
	m.TrainerMessage = "🔒 Module locked! Complete previous boss first."

	res, _ := m.handleTrainerMenuKeys("q")
	got := res.(Model)
	if got.TrainerMessage != "" {
		t.Errorf("q carried the trainer's stale message onto the main menu: %q", got.TrainerMessage)
	}
}

// TestInitDetectsExistingConfigsForTheMainMenu pins the startup scan end to end:
// Init's command reads the config paths, and the message it produces puts them
// on the model, so the main menu's panel can say what the run will overwrite
// before the wizard asks anything.
func TestInitDetectsExistingConfigsForTheMainMenu(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".config", "nvim"), 0o755); err != nil {
		t.Fatalf("creating an existing config failed: %v", err)
	}

	m := NewModel()
	result, _ := m.Update(detectConfigsCmd()())
	got := result.(Model)

	found := false
	for _, config := range got.ExistingConfigs {
		if strings.HasPrefix(config, "nvim: ") {
			found = true
		}
	}
	if !found {
		t.Errorf("the startup scan did not reach the model: ExistingConfigs = %v", got.ExistingConfigs)
	}
}

// TestStartupScanDoesNotOverwriteKnownConfigs pins the bootstrap rule: the scan
// fills the model's overwrite facts when it has none, and leaves them alone when
// it already knows them. The wizard's own scan is the authority that replaces
// them; a startup scan that clobbered a model handed its state would make every
// end-to-end test that seeds the field depend on the HOME the suite runs under.
func TestStartupScanDoesNotOverwriteKnownConfigs(t *testing.T) {
	m := NewModel()
	m.ExistingConfigs = []string{"nvim: /known/nvim"}

	result, _ := m.Update(configsDetectedMsg{configs: []string{"fish: /scanned/fish"}})
	got := result.(Model)

	if len(got.ExistingConfigs) != 1 || got.ExistingConfigs[0] != "nvim: /known/nvim" {
		t.Errorf("the startup scan overwrote known configs: %v", got.ExistingConfigs)
	}
}

// TestPlanPanelRendersTheStartupConfigsAsOverwrites pins the join between the two
// halves: once the startup scan has filled the model, the panel's overwrite rows
// are real facts rather than the absent section TestWillHappenPanelOmitsTheBackupRow
// proposes for an unscanned model.
func TestPlanPanelRendersTheStartupConfigsAsOverwrites(t *testing.T) {
	l := narrowPanelLayout()
	m := Model{SystemInfo: goldenSystemInfo(), ExistingConfigs: []string{"nvim: /home/testuser/.config/nvim"}}

	text := panelFlat(m.mainMenuPanel(l, 40))
	if !strings.Contains(text, "1 config") {
		t.Errorf("the panel does not count the scanned config as an overwrite:\n%s", text)
	}
	if !strings.Contains(text, "nvim: /home/testuser/.config/nvim") {
		t.Errorf("the panel does not name the config the run will overwrite:\n%s", text)
	}
}

// TestWillHappenPanelCountsTheConfigsTheRunWillOverwrite pins the overwrite rows
// for a model that has scanned them: the count, in the singular when it is one,
// and which configs they are.
func TestWillHappenPanelCountsTheConfigsTheRunWillOverwrite(t *testing.T) {
	l := narrowPanelLayout()
	configs := []string{
		"nvim: /home/testuser/.config/nvim",
		"fish: /home/testuser/.config/fish",
	}

	t.Run("several", func(t *testing.T) {
		m := Model{ExistingConfigs: configs}
		rows := m.mainMenuPanel(l, 40)
		assertPanelFits(t, rows, l.Right, 40)
		text := panelText(rows)

		if !strings.Contains(text, "2 configs") {
			t.Errorf("the plan panel does not count the configs the run will overwrite:\n%s", text)
		}
		for _, config := range configs {
			if !strings.Contains(text, config) {
				t.Errorf("the plan panel does not name %q:\n%s", config, text)
			}
		}
	})

	t.Run("one reads as one config", func(t *testing.T) {
		m := Model{ExistingConfigs: configs[:1]}
		text := panelText(m.mainMenuPanel(l, 40))
		if !strings.Contains(text, "1 config") || strings.Contains(text, "1 configs") {
			t.Errorf("the plan panel does not write a single config in the singular:\n%s", text)
		}
	})
}

// TestWillHappenPanelShowsTheNewestBackup pins the last row: the newest backup
// of whatever the model holds, with when it was taken and how many files it
// carries. The list system.ListBackups returns is in directory order, so the
// newest is the greatest timestamp and not the last row. The age is measured
// against the model's own clock, so a model with a zero Now leaves it out and a
// model with a real one adds it, and neither reads the clock while rendering.
func TestWillHappenPanelShowsTheNewestBackup(t *testing.T) {
	l := narrowPanelLayout()
	taken := time.Date(2024, 1, 3, 12, 0, 0, 0, time.UTC)
	newest := system.BackupInfo{
		Path:      "/home/testuser/.dotfiles-backup-20240103-120000",
		Timestamp: taken,
		Files:     []string{"nvim", "fish", "zsh"},
	}
	older := system.BackupInfo{
		Path:      "/home/testuser/.dotfiles-backup-20240101-120000",
		Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		Files:     []string{"nvim"},
	}

	t.Run("a model with no clock shows the date and the count", func(t *testing.T) {
		m := Model{AvailableBackups: []system.BackupInfo{newest, older}}

		rows := m.mainMenuPanel(l, 40)
		assertPanelFits(t, rows, l.Right, 40)
		text := panelText(rows)

		for _, want := range []string{"Newest backup", newest.Timestamp.Format(panelBackupStamp), "3 files"} {
			if !strings.Contains(text, want) {
				t.Errorf("the plan panel does not show %q:\n%s", want, text)
			}
		}
		if strings.Contains(text, "ago") {
			t.Errorf("the plan panel measured an age against no clock:\n%s", text)
		}
		if strings.Contains(text, older.Timestamp.Format(panelBackupStamp)) {
			t.Errorf("the plan panel shows the older backup as the newest:\n%s", text)
		}

		if again := m.mainMenuPanel(l, 40); !equalRows(rows, again) {
			t.Errorf("the plan panel changed between two renders of one model:\nfirst:\n%s\nsecond:\n%s",
				panelText(rows), panelText(again))
		}
	})

	t.Run("a model with a clock adds the age", func(t *testing.T) {
		m := Model{AvailableBackups: []system.BackupInfo{newest, older}, Now: taken.Add(9 * 24 * time.Hour)}

		text := panelFlat(m.mainMenuPanel(l, 40))
		if !strings.Contains(text, "3 files, 9 days ago") {
			t.Errorf("the plan panel does not show the newest backup's age:\n%s", text)
		}
	})
}

// TestBackupAgeNamesTheCoarsestUnitThatSaysSomething pins the age arithmetic to
// fixed clock readings: the panel hands the two instants here, so the function
// reads no clock of its own and cannot flake.
func TestBackupAgeNamesTheCoarsestUnitThatSaysSomething(t *testing.T) {
	now := time.Date(2024, 1, 31, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		then time.Time
		want string
	}{
		{"just taken", now, "just now"},
		{"under a minute", now.Add(-30 * time.Second), "just now"},
		{"one minute", now.Add(-time.Minute), "1 minute ago"},
		{"minutes", now.Add(-5 * time.Minute), "5 minutes ago"},
		{"one hour", now.Add(-time.Hour), "1 hour ago"},
		{"hours", now.Add(-5 * time.Hour), "5 hours ago"},
		{"one day", now.Add(-25 * time.Hour), "1 day ago"},
		{"days", now.Add(-9 * 24 * time.Hour), "9 days ago"},
		{"one month", now.Add(-40 * 24 * time.Hour), "1 month ago"},
		{"months", now.Add(-100 * 24 * time.Hour), "3 months ago"},
		{"a clock that is ahead makes it just now, not a negative age", now.Add(time.Hour), "just now"},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			if got := backupAge(c.then, now); got != c.want {
				t.Errorf("backupAge = %q, want %q", got, c.want)
			}
		})
	}
}

// TestBackupLineOmitsTheAgeWithoutAReferenceTime pins the zero-Now rule: the
// model carries no clock until the startup path or the tick fills one, and a
// golden model stays zero forever, so the row keeps a deterministic date and
// file count and simply leaves the age out instead of reading a clock or
// printing a moment nobody measured.
func TestBackupLineOmitsTheAgeWithoutAReferenceTime(t *testing.T) {
	taken := time.Date(2026, 9, 19, 19, 30, 41, 0, time.UTC)
	backup := system.BackupInfo{Timestamp: taken, Files: make([]string, 12)}

	if got, want := backupLine(backup, time.Time{}), "2026-09-19 19:30:41, 12 files"; got != want {
		t.Errorf("backupLine with a zero reference time = %q, want %q", got, want)
	}
	if got, want := backupLine(backup, taken.Add(9*24*time.Hour)), "2026-09-19 19:30:41, 12 files, 9 days ago"; got != want {
		t.Errorf("backupLine with a real reference time = %q, want %q", got, want)
	}
}

// TestPanelNeverExceedsItsColumnOrItsBudget pins the bounds at three budgets: a
// panel with more rows than the frame leaves keeps the rows that fit and says how
// many it dropped, so a wide-but-short terminal loses rows out loud instead of
// pushing the footer off the screen.
func TestPanelNeverExceedsItsColumnOrItsBudget(t *testing.T) {
	l := narrowPanelLayout()
	steps := make([]InstallStep, 0, 8)
	for i := 0; i < 8; i++ {
		steps = append(steps, InstallStep{
			Name:        "A step with a name long enough to wrap inside the column",
			Description: "And a description long enough to take more than one row of the panel beside it.",
		})
	}
	m := Model{
		Steps:            steps,
		ExistingConfigs:  []string{"nvim: /home/testuser/.config/nvim", "fish: /home/testuser/.config/fish"},
		AvailableBackups: []system.BackupInfo{{Timestamp: time.Now().Add(-time.Hour), Files: []string{"nvim"}}},
	}

	for _, budget := range []int{5, 12, 60} {
		rows := m.mainMenuPanel(l, budget)
		assertPanelFits(t, rows, l.Right, budget)

		text := panelText(rows)
		clipped := strings.Contains(text, cutMarker+" and ")
		if clipped != (budget < len(m.mainMenuPanel(l, 200))) {
			t.Errorf("at a %d-row budget the panel's cut note is %v:\n%s", budget, clipped, text)
		}
	}
}

// TestPanelsStayOffTheFloorFrame pins the collapse: below the two-column floor
// neither panel has room, so the screen renders exactly as it did before the
// panels existed and no panel's full title appears. The wide-terminal guard
// covers the other half of the same rule. Since the first slice's collapse, the
// narrow screens show a one-line summary of their active panel instead -- but
// that summary names the panel short ("Machine", "Plan") because it has one row
// to spend, so the full titles stay the tab row's and nothing else's.
func TestPanelsStayOffTheFloorFrame(t *testing.T) {
	for _, screen := range []struct {
		name   string
		screen Screen
		panel  string
	}{
		{"welcome", ScreenWelcome, welcomePanelLabel},
		{"main menu", ScreenMainMenu, mainMenuPanelLabel},
	} {
		screen := screen
		t.Run(screen.name, func(t *testing.T) {
			m := installerFrameModel(t, screen.screen)
			view := ansiEscape.ReplaceAllString(m.View(), "")
			if strings.Contains(view, screen.panel) {
				t.Errorf("the %s screen shows the %q panel at 80x24, where there is no room for a second column:\n%s",
					screen.name, screen.panel, view)
			}
		})
	}
}

// TestWelcomeScreenCarriesTheMachinePanelWhenItHasRoom pins the wiring end to
// end: the welcome screen is the one that asks where am I, so at a two-column
// size it renders the panel the machine's own fields produce, and the frame
// drops that panel below the floor.
func TestWelcomeScreenCarriesTheMachinePanelWhenItHasRoom(t *testing.T) {
	m := Model{Width: 160, Height: 50, Screen: ScreenWelcome, SystemInfo: goldenSystemInfo()}
	view := ansiEscape.ReplaceAllString(m.View(), "")

	if !strings.Contains(view, welcomePanelLabel) {
		t.Errorf("the welcome screen at 160x50 has no room for its panel:\n%s", view)
	}
	for _, want := range []string{"Linux", "zsh", "apt-get", "/home/testuser"} {
		if !strings.Contains(view, want) {
			t.Errorf("the welcome screen's panel does not name %q:\n%s", want, view)
		}
	}
}

// --- The panel registry, Tab and the narrow-terminal summary ----------------

// testPanel is a panel with fixed facts, for the registry tests. The machinery
// they exercise -- the tab row, the [Tab] hint, the narrow summary -- reads a
// panel's title and headline, not where its facts come from, so a synthetic
// panel is enough to reach a two-panel screen while the shipped registry still
// lists one panel per screen.
func testPanel(id panelID, title, headline string, facts ...string) panel {
	return panel{
		ID:       id,
		Title:    title,
		Short:    title,
		Headline: func(Model) string { return headline },
		Facts:    func(Model, layout) []string { return facts },
	}
}

// twoPanelFacts is a stand-in body for a framed two-panel render: short enough
// that the frame keeps spare rows for the narrow summary.
func twoPanelFacts() []string {
	return []string{
		BrandStyle.Render("dotfiles"),
		MutedStyle.Render("What would you like to do?"),
		"",
		"Start Installation",
		"Learn",
		"Quit",
	}
}

// TestPanelsForListsWhatEachScreenOffers pins the registry: the welcome screen
// offers the machine panel, the main menu and the wizard's own questions offer
// the plan, and every other screen offers nothing, exactly as before the panels
// existed. A screen may not advertise a panel it does not list, and the default
// is the one listed first.
func TestPanelsForListsWhatEachScreenOffers(t *testing.T) {
	cases := []struct {
		screen Screen
		want   []panelID
	}{
		{ScreenWelcome, []panelID{panelMachine, panelLive, panelTip}},
		{ScreenMainMenu, []panelID{panelPlan, panelTrainer, panelTip}},
		{ScreenOSSelect, []panelID{panelPlan}},
		{ScreenTerminalSelect, []panelID{panelPlan}},
		{ScreenFontSelect, []panelID{panelPlan}},
		{ScreenShellSelect, []panelID{panelPlan}},
		{ScreenWMSelect, []panelID{panelPlan}},
		{ScreenNvimSelect, []panelID{panelPlan}},
		{ScreenGhosttyWarning, []panelID{panelPlan}},
		{ScreenLearnTerminals, nil},
		{ScreenKeymaps, nil},
		{ScreenInstalling, nil},
		{ScreenComplete, nil},
		{ScreenError, nil},
		{ScreenBackupConfirm, nil},
		{ScreenRestoreBackup, nil},
		{ScreenTrainerMenu, nil},
		{ScreenTrainerLesson, nil},
	}

	for _, c := range cases {
		m := Model{Screen: c.screen}
		got := m.panelsFor()
		if len(got) != len(c.want) {
			t.Errorf("screen %v offers %d panels, want %d", c.screen, len(got), len(c.want))
			continue
		}
		for i, want := range c.want {
			if got[i].ID != want {
				t.Errorf("screen %v panel %d = %q, want %q", c.screen, i, got[i].ID, want)
			}
		}
		if len(got) > 0 && m.activePanelIndex(got) != 0 {
			t.Errorf("screen %v opens on panel %d, want the default (the first)", c.screen, m.activePanelIndex(got))
		}
	}
}

// TestSinglePanelColumnIsTheChipItAlwaysShowed pins the no-change case: a screen
// that offers one panel renders the same chip, rule and rows through the
// registry as it did through the panel's own function, so the machinery cannot
// have moved a single byte of the two panels that already shipped.
func TestSinglePanelColumnIsTheChipItAlwaysShowed(t *testing.T) {
	l := narrowPanelLayout()

	machine := Model{SystemInfo: goldenSystemInfo()}
	if got, want := machine.panelColumn([]panel{machinePanel()}, l, 40), machine.welcomePanel(l, 40); !equalRows(got, want) {
		t.Errorf("the machine column through the registry differs from its own panel:\nregistry:\n%s\npanel:\n%s",
			panelText(got), panelText(want))
	}

	steps := []InstallStep{{Name: "Clone Repository"}, {Name: "Install Dependencies"}}
	plan := Model{Steps: steps, ExistingConfigs: []string{"nvim: /home/testuser/.config/nvim"}}
	if got, want := plan.panelColumn([]panel{planPanel()}, l, 40), plan.mainMenuPanel(l, 40); !equalRows(got, want) {
		t.Errorf("the plan column through the registry differs from its own panel:\nregistry:\n%s\npanel:\n%s",
			panelText(got), panelText(want))
	}
}

// equalRows compares two rendered blocks by their visible words, so a stylistic
// difference is not reported as a content difference.
func equalRows(a, b []string) bool {
	return panelText(a) == panelText(b)
}

// TestTabRowNamesThePanelsAndMarksTheActiveOne pins the tab row: every panel the
// screen offers is named on the one row, the active one in brand chrome and the
// rest in the dim tone, separated by a dim separator. Switching the active index
// moves the chrome and the facts with it.
func TestTabRowNamesThePanelsAndMarksTheActiveOne(t *testing.T) {
	l := narrowPanelLayout()
	panels := []panel{
		testPanel(panelMachine, "Machine", "here", "OS  Linux"),
		testPanel(panelPlan, "Plan", "next", "Steps  8"),
	}

	first := Model{}.panelColumn(panels, l, 40)
	rows := strings.Split(panelText(first), "\n")
	if len(rows) < 2 {
		t.Fatalf("the tab row is missing:\n%s", panelText(first))
	}
	if !strings.Contains(rows[0], "Machine") || !strings.Contains(rows[0], "Plan") {
		t.Errorf("the tab row does not name both panels: %q", rows[0])
	}
	if !strings.Contains(first[0], BrandStyle.Render("Machine")) {
		t.Errorf("the active panel is not in brand chrome: %q", first[0])
	}
	if !strings.Contains(first[0], MutedStyle.Render("Plan")) {
		t.Errorf("the inactive panel is not dim: %q", first[0])
	}
	if !strings.Contains(first[0], MutedStyle.Render(panelTabSeparator)) {
		t.Errorf("the tab row has no dim separator: %q", first[0])
	}
	if !strings.Contains(panelText(first), "OS  Linux") {
		t.Errorf("the first panel's facts are not shown:\n%s", panelText(first))
	}

	second := Model{PanelIndex: 1}.panelColumn(panels, l, 40)
	if !strings.Contains(second[0], BrandStyle.Render("Plan")) {
		t.Errorf("after a Tab the plan is not the active panel: %q", second[0])
	}
	if !strings.Contains(second[0], MutedStyle.Render("Machine")) {
		t.Errorf("after a Tab the machine panel is not dim: %q", second[0])
	}
	if !strings.Contains(panelText(second), "Steps  8") || strings.Contains(panelText(second), "OS  Linux") {
		t.Errorf("after a Tab the column still shows the first panel's facts:\n%s", panelText(second))
	}
}

// TestNextPanelIndexWrapsAndRefusesToMoveWithOnePanel pins the key itself: the
// index advances and wraps to the first panel at the end, and a screen with
// fewer than two panels reports that it did not consume the key, which is what
// leaves Tab free for the trainer screens.
func TestNextPanelIndexWrapsAndRefusesToMoveWithOnePanel(t *testing.T) {
	none := []panel{}
	one := []panel{testPanel(panelPlan, "Plan", "x", "a")}
	three := []panel{
		testPanel(panelMachine, "Machine", "x", "a"),
		testPanel(panelPlan, "Plan", "x", "b"),
		testPanel("third", "Third", "x", "c"),
	}

	if index, moved := nextPanelIndex(none, 0); moved || index != 0 {
		t.Errorf("Tab on a screen with no panels = (%d, %v), want (0, false)", index, moved)
	}
	if index, moved := nextPanelIndex(one, 0); moved || index != 0 {
		t.Errorf("Tab on a one-panel screen = (%d, %v), want (0, false)", index, moved)
	}
	if index, moved := nextPanelIndex(three, 0); !moved || index != 1 {
		t.Errorf("Tab from the first panel = (%d, %v), want (1, true)", index, moved)
	}
	if index, moved := nextPanelIndex(three, 2); !moved || index != 0 {
		t.Errorf("Tab from the last panel = (%d, %v), want (0, true)", index, moved)
	}
}

// TestActivePanelIndexClampsAModelBuiltByHand pins the safety net: an index that
// no longer addresses a panel of the screen -- a model built by hand rather than
// by a screen change -- falls back to the default instead of panicking.
func TestActivePanelIndexClampsAModelBuiltByHand(t *testing.T) {
	panels := []panel{
		testPanel(panelMachine, "Machine", "x", "a"),
		testPanel(panelPlan, "Plan", "x", "b"),
	}
	for _, index := range []int{-1, 2, 99} {
		m := Model{PanelIndex: index}
		if got := m.activePanelIndex(panels); got != 0 {
			t.Errorf("activePanelIndex with index %d = %d, want the default 0", index, got)
		}
	}
}

// TestPanelIndexResetsWhenTheScreenChanges pins the reset through the one path
// every screen change takes: Update. A screen always opens on its default panel,
// so a Tab on one screen cannot leave the next screen showing a panel it does
// not have.
func TestPanelIndexResetsWhenTheScreenChanges(t *testing.T) {
	m := Model{Screen: ScreenWelcome, PanelIndex: 1}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(Model)
	if got.Screen != ScreenMainMenu {
		t.Fatalf("screen after enter = %v, want %v", got.Screen, ScreenMainMenu)
	}
	if got.PanelIndex != 0 {
		t.Errorf("PanelIndex after a screen change = %d, want the default 0", got.PanelIndex)
	}
}

// TestTabIsNotStolenFromTheTrainer pins the other half of the key: the exercise
// screens give Tab their own meaning -- it reveals the hint -- and a screen that
// offers no panel must not consume it. The trainer shipped this binding first,
// so the panel machinery yields to it.
func TestTabIsNotStolenFromTheTrainer(t *testing.T) {
	m := newTrainerLessonModel(t)
	exercise := m.TrainerGameState.CurrentExercise
	if exercise == nil || exercise.Hint == "" {
		t.Fatalf("test setup: the lesson exercise has no hint for Tab to reveal")
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	got := next.(Model)
	if want := "💡 Hint: " + exercise.Hint; got.TrainerMessage != want {
		t.Errorf("TrainerMessage after Tab = %q, want %q: the panel key stole the trainer's hint", got.TrainerMessage, want)
	}
}

// TestFooterAdvertisesTabOnlyWhereThereIsAnotherPanel pins the discoverability
// rule: Tab appears in a screen's footer only when the screen offers a second
// panel, because a key that does nothing must not be advertised.
func TestFooterAdvertisesTabOnlyWhereThereIsAnotherPanel(t *testing.T) {
	one := []panel{testPanel(panelPlan, "Plan", "x", "a")}
	two := []panel{
		testPanel(panelMachine, "Machine", "x", "a"),
		testPanel(panelPlan, "Plan", "x", "b"),
	}
	hints := []installerHint{hintUp, hintDown, hintSelect, hintQuit}

	m := Model{}
	if got := m.panelHints(one, hints); len(got) != len(hints) {
		t.Errorf("a one-panel screen's hints grew to %d, want %d unchanged", len(got), len(hints))
	}
	got := m.panelHints(two, hints)
	if len(got) != len(hints)+1 || got[len(got)-1] != hintTab {
		t.Errorf("a two-panel screen's hints = %v, want the same list with the [Tab] hint appended", got)
	}
	if len(hints) != 4 {
		t.Errorf("panelHints modified the caller's slice: %v", hints)
	}
}

// TestNarrowSummaryShowsTheActivePanelWholeOrNotAtAll pins the summary row: it
// names the active panel in the dim tone followed by its headline, follows the
// active index, wraps rather than cutting, and is left out entirely when the
// text would not fit in the two rows the frame allows -- a headline that stops
// mid-sentence says less than none.
func TestNarrowSummaryShowsTheActivePanelWholeOrNotAtAll(t *testing.T) {
	panels := []panel{
		testPanel(panelMachine, "Machine", "Linux · x86_64", "a"),
		testPanel(panelPlan, "Plan", "8 steps on Linux (detected)", "b"),
	}

	lines := (Model{}).rotatorLines(panels, 96)
	if len(lines) != 1 {
		t.Fatalf("the summary is %d rows at 96 columns, want 1:\n%v", len(lines), lines)
	}
	text := panelText(lines)
	for _, want := range []string{"Machine", "Linux · x86_64"} {
		if !strings.Contains(text, want) {
			t.Errorf("the summary does not name %q: %q", want, text)
		}
	}
	if !strings.Contains(lines[0], MutedStyle.Render("Machine")) {
		t.Errorf("the panel's name is not in the dim tone: %q", lines[0])
	}
	if strings.Contains(text, cutMarker) {
		t.Errorf("the summary was cut instead of wrapped: %q", text)
	}

	second := Model{PanelIndex: 1}.rotatorLines(panels, 96)
	if text := panelText(second); !strings.Contains(text, "8 steps on Linux (detected)") || strings.Contains(text, "x86_64") {
		t.Errorf("the summary does not follow the active panel: %q", text)
	}

	// A headline that needs more than the allowed rows is not shown, and it is
	// not shown rather than cut.
	long := []panel{testPanel(panelPlan, "Plan", strings.Repeat("word ", 30), "a")}
	if lines := (Model{}).rotatorLines(long, 24); len(lines) != 0 {
		t.Errorf("a summary needing more than %d rows was shown as %v", rotatorMaxRows, panelText(lines))
	}
}

// TestRotatorFillsOnlyTheRowsTheBodyDidNotNeed pins the placement rule: the
// summary may take a trailing blank row and nothing else. A body row is never
// displaced, and a frame with fewer spare rows than the summary needs shows no
// summary at all.
func TestRotatorFillsOnlyTheRowsTheBodyDidNotNeed(t *testing.T) {
	cases := []struct {
		name    string
		placed  []string
		rotator []string
		want    []string
	}{
		{"one blank row takes the summary", []string{"a", "", ""}, []string{"r"}, []string{"a", "", "r"}},
		{"a full body is left alone", []string{"a", "b", "c"}, []string{"r"}, []string{"a", "b", "c"}},
		{"a two-row summary takes two blank rows", []string{"a", "", ""}, []string{"r1", "r2"}, []string{"a", "r1", "r2"}},
		{"too few spare rows: no summary", []string{"a", "b", ""}, []string{"r1", "r2"}, []string{"a", "b", ""}},
		{"no rows at all: no panic", nil, []string{"r"}, nil},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			got := placeRotator(c.placed, c.rotator)
			if strings.Join(got, "|") != strings.Join(c.want, "|") {
				t.Errorf("placeRotator = %v, want %v", got, c.want)
			}
		})
	}
}

// TestTwoPanelScreensRenderTheTabRowAndTheSummary pins the machinery end to end
// through the frame, at the two sizes the design was settled on. Above the
// two-column floor the panel column starts with the tab row and the footer
// advertises Tab; below it the active panel becomes the summary row above the
// footer rule, and Tab is advertised there too so the hint stays truthful. A
// screen whose body fills its frame gets no summary, because nothing may be
// dropped from a body to make room for a summary of it.
func TestTwoPanelScreensRenderTheTabRowAndTheSummary(t *testing.T) {
	panels := []panel{
		testPanel(panelMachine, "Machine", "Linux · x86_64", "OS  Linux"),
		testPanel(panelPlan, "Plan", "8 steps", "Steps  8"),
	}
	hints := []installerHint{hintUp, hintDown, hintSelect, hintQuit}

	t.Run("two columns at 160x50", func(t *testing.T) {
		m := Model{Width: 160, Height: 50}
		view := ansiEscape.ReplaceAllString(m.frameWithPanels("Main Menu", "", twoPanelFacts(), hints, panels), "")

		// View() adds the one global padding row; the frame itself spends
		// exactly the rows it reserved.
		if rows := renderedRowCount(view); rows != 50-viewPaddingRows {
			t.Fatalf("the two-panel screen renders %d rows, want %d", rows, 50-viewPaddingRows)
		}
		if !strings.Contains(view, "Machine") || !strings.Contains(view, "Plan") {
			t.Errorf("the panel tab row does not name both panels:\n%s", view)
		}
		if !strings.Contains(view, "[Tab]") {
			t.Errorf("a two-panel screen does not advertise Tab:\n%s", view)
		}
		if !strings.Contains(view, "OS  Linux") {
			t.Errorf("the active panel's facts are not composed beside the body:\n%s", view)
		}
	})

	t.Run("narrow at 100x24", func(t *testing.T) {
		m := Model{Width: 100, Height: 24}
		view := ansiEscape.ReplaceAllString(m.frameWithPanels("Main Menu", "", twoPanelFacts(), hints, panels), "")

		if rows := renderedRowCount(view); rows != 24-viewPaddingRows {
			t.Fatalf("the narrow two-panel screen renders %d rows, want exactly %d", rows, 24-viewPaddingRows)
		}
		lines := strings.Split(view, "\n")
		var summary string
		for _, line := range lines {
			if strings.Contains(line, "Machine") {
				summary = line
			}
		}
		if summary == "" {
			t.Fatalf("the narrow screen has no summary row naming the active panel:\n%s", view)
		}
		if !strings.Contains(summary, "Linux · x86_64") {
			t.Errorf("the summary row does not carry the headline fact: %q", summary)
		}
		if !strings.Contains(view, "[Tab]") {
			t.Errorf("the narrow screen does not advertise Tab:\n%s", view)
		}
	})

	t.Run("a body that fills the frame gets no summary", func(t *testing.T) {
		m := Model{Width: 100, Height: 24}
		rows := installerBodyRows(m.Height, footerRowCount(layoutFor(m).Inner, m.panelHints(panels, hints)))
		body := make([]string, rows)
		for i := range body {
			body[i] = fmt.Sprintf("body row %d", i+1)
		}

		view := ansiEscape.ReplaceAllString(m.frameWithPanels("Main Menu", "", body, hints, panels), "")
		if strings.Contains(view, "Machine") {
			t.Errorf("a full body was summarised and may have lost a row:\n%s", view)
		}
		if rows := renderedRowCount(view); rows != 24-viewPaddingRows {
			t.Errorf("the full-body screen renders %d rows, want exactly %d", rows, 24-viewPaddingRows)
		}
	})
}

// --- The trainer, the tip and the state panels ------------------------------

// TestMainMenuOffersLastInstallOnlyWhenARecordExists pins the conditional slot:
// the state panel is offered when the model holds a record and left out when it
// does not, so the tab row never names a panel with nothing to say and the screen
// never shows a section that reads "never".
func TestMainMenuOffersLastInstallOnlyWhenARecordExists(t *testing.T) {
	offered := func(m Model) bool {
		for _, p := range m.panelsFor() {
			if p.ID == panelLastInstall {
				return true
			}
		}
		return false
	}

	if offered(Model{Screen: ScreenMainMenu}) {
		t.Errorf("the main menu offers a Last install panel with no record")
	}
	withRecord := Model{
		Screen:      ScreenMainMenu,
		LastInstall: &lastInstall{Timestamp: time.Now(), Version: "v0.4.0"},
	}
	if !offered(withRecord) {
		t.Errorf("the main menu does not offer the Last install panel it holds a record for")
	}

	// The welcome screen never carries the state panel; it answers where you are.
	if offered(Model{Screen: ScreenWelcome, LastInstall: withRecord.LastInstall}) {
		t.Errorf("the welcome screen offers the state panel")
	}
}

// TestTrainerPanelSaysSoWhenThereAreNoRuns pins the honesty rule the panel
// exists for: a nil profile (no file) and a saved profile with nothing played
// both read as "no run", and neither is drawn as a column of zeros that would
// look like progress.
func TestTrainerPanelSaysSoWhenThereAreNoRuns(t *testing.T) {
	for _, stats := range []*trainer.UserStats{nil, trainer.NewUserStats()} {
		m := Model{TrainerStats: stats, TrainerModules: trainer.GetAllModules()}
		rows := m.trainerPanelFacts(narrowPanelLayout())

		if len(rows) != 1 {
			t.Errorf("a profile with no runs rendered %d rows, want the one line that says so:\n%s", len(rows), panelText(rows))
			continue
		}
		text := panelText(rows)
		if !strings.Contains(text, "No runs recorded") {
			t.Errorf("the no-runs panel does not say so: %q", text)
		}
		for _, zero := range []string{"0%", "0/", "streak", "boss"} {
			if strings.Contains(text, zero) {
				t.Errorf("the no-runs panel draws %q, which reads as measured progress: %q", zero, text)
			}
		}
	}
}

// TestTrainerPanelNamesModulesAccuracyStreakAndTheNextBoss pins the loaded
// panel: a started module with its lessons and mastery, the overall accuracy and
// the best streak, and the next boss with the practice gate it needs. A module
// that was never opened stays out rather than appearing as "0/5".
func TestTrainerPanelNamesModulesAccuracyStreakAndTheNextBoss(t *testing.T) {
	stats := trainer.NewUserStats()
	progress := stats.GetModuleProgress(trainer.ModuleHorizontal)
	progress.LessonsCompleted = 2
	progress.LessonsTotal = 5
	progress.PracticeAttempts = 4
	progress.PracticeCorrect = 3
	progress.PracticeAccuracy = 0.75
	stats.BestStreak = 6

	m := Model{TrainerStats: stats, TrainerModules: trainer.GetAllModules()}
	l := narrowPanelLayout()
	rows := m.trainerPanelFacts(l)
	assertPanelFits(t, rows, l.Right, 200)

	flat := panelFlat(rows)
	for _, want := range []string{
		"Horizontal", "2/5 lessons", "mastered",
		"Accuracy 75%",
		"Best streak 6",
		"Next boss The Line Walker",
		"80% practice accuracy and 10 tries",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("the trainer panel does not name %q:\n%s", want, panelText(rows))
		}
	}
	if strings.Contains(flat, "Vertical Motions") {
		t.Errorf("the trainer panel shows a module that was never opened as a row of zeros:\n%s", panelText(rows))
	}

	headline := m.trainerHeadline()
	if !strings.Contains(headline, "accuracy 75%") || !strings.Contains(headline, "best streak 6") {
		t.Errorf("the trainer headline = %q, want the accuracy and the best streak", headline)
	}
}

// TestTipPoolIsDeclaredOrderAndDeterministic pins the pool: it starts with the
// keymap reference data's first binding, walks the sources in their declared
// order, ends with the trainer's lessons, and is rebuilt byte-for-byte the same
// on a second call, so two runs on one machine show one sequence.
func TestTipPoolIsDeclaredOrderAndDeterministic(t *testing.T) {
	a := buildTipPool()
	b := buildTipPool()
	if len(a) < 2 {
		t.Fatalf("the tip pool has %d entries, want at least 2", len(a))
	}
	if len(a) != len(b) {
		t.Fatalf("the pool rebuilt to %d entries, want %d", len(b), len(a))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("the pool rebuilt differently at %d: %+v vs %+v", i, a[i], b[i])
		}
	}

	nvim := GetNvimKeymaps()
	first := nvim[0]
	if a[0].Context != "Neovim"+panelTabSeparator+first.Name {
		t.Errorf("tip 0 comes from %q, want the first Neovim category %q", a[0].Context, first.Name)
	}
	if a[0].Keys != first.Keymaps[0].Keys || a[0].Action != first.Keymaps[0].Description {
		t.Errorf("tip 0 = %+v, want the first Neovim binding %+v", a[0], first.Keymaps[0])
	}
	if a[1].Keys != first.Keymaps[1].Keys || a[1].Action != first.Keymaps[1].Description {
		t.Errorf("tip 1 = %+v, want the second Neovim binding %+v", a[1], first.Keymaps[1])
	}

	// The trainer's lessons follow every keymap source, so the trainer block starts
	// exactly one keymap count along and runs to the end of the pool. The first
	// surviving lesson is one of the first module's; lessons whose mission does not
	// fit the tip's rows were left out of the pool rather than admitted and cut.
	keymapCount := 0
	for _, cats := range [][]KeymapCategory{GetNvimKeymaps(), GetTmuxKeymaps(), GetZellijKeymaps(), GetGhosttyKeymaps(), GetHerdrKeymaps()} {
		for _, cat := range cats {
			keymapCount += len(cat.Keymaps)
		}
	}
	if keymapCount >= len(a) {
		t.Fatalf("the pool holds %d entries, all keymaps", len(a))
	}
	modules := trainer.GetAllModules()
	if a[keymapCount].Context != "Vim Trainer"+panelTabSeparator+modules[0].Name {
		t.Errorf("the first trainer tip comes from %q, want the first module %q", a[keymapCount].Context, modules[0].Name)
	}
	horizontalOptimal := make(map[string]bool)
	for _, exercise := range trainer.GetLessons(modules[0].ID) {
		horizontalOptimal[exercise.Optimal] = true
	}
	if !horizontalOptimal[a[keymapCount].Keys] {
		t.Errorf("the first trainer tip's keys = %q, not an optimal of %q", a[keymapCount].Keys, modules[0].Name)
	}
	for i := keymapCount; i < len(a); i++ {
		if !strings.HasPrefix(a[i].Context, "Vim Trainer"+panelTabSeparator) {
			t.Errorf("a keymap tip %q follows the trainer tips at %d", a[i].Context, i)
			break
		}
	}
}

// TestTipRotationFollowsTheTickAndStaysAtZeroWithoutAnimation pins the rotation
// arithmetic: one tip per ten seconds, wrapping at the end of the pool, and tip 0
// while the counter cannot leave zero.
func TestTipRotationFollowsTheTickAndStaysAtZeroWithoutAnimation(t *testing.T) {
	size := len(tipPool())
	if size < 2 {
		t.Fatalf("the tip pool has %d entries, want at least 2", size)
	}

	// Ten seconds is the rule and the frames follow from the rate. The tip has to
	// hold for every frame of those ten seconds and turn on the first frame of the
	// next ten -- checked frame by frame, so a rotation that turns one frame early
	// or late is caught here rather than by someone counting ticks.
	if want := animTicksPerSecond * 10; ticksPerTip != want {
		t.Fatalf("a tip holds for %d frames, want %d: ten seconds at %d frames a second",
			ticksPerTip, want, animTicksPerSecond)
	}
	for tick := 0; tick < ticksPerTip; tick++ {
		if got := tipIndex(Model{AnimTick: tick}); got != 0 {
			t.Fatalf("the tip at frame %d of %d is %d, want tip 0 for the whole first ten seconds",
				tick, ticksPerTip, got)
		}
	}
	for tick := ticksPerTip; tick < 2*ticksPerTip; tick++ {
		if got := tipIndex(Model{AnimTick: tick}); got != 1 {
			t.Fatalf("the tip at frame %d (the second ten seconds) is %d, want 1", tick, got)
		}
	}

	cases := []struct {
		tick int
		want int
	}{
		{0, 0},
		{ticksPerTip - 1, 0},
		{ticksPerTip, 1},
		{2 * ticksPerTip, 2},
		{size * ticksPerTip, 0},
		{(size + 1) * ticksPerTip, 1},
	}
	for _, c := range cases {
		if got := tipIndex(Model{AnimTick: c.tick}); got != c.want {
			t.Errorf("tipIndex(tick %d) = %d, want %d", c.tick, got, c.want)
		}
	}

	if got := tipIndex(Model{AnimTick: -1}); got != 0 {
		t.Errorf("tipIndex with a negative tick = %d, want 0", got)
	}

	// Animation off is the tick staying at zero, so the screen is on tip 0 with no
	// second source of truth to disagree with the tick.
	if got := tipIndex(Model{Animating: false, AnimTick: 0}); got != 0 {
		t.Errorf("tipIndex with animation off = %d, want 0", got)
	}
}

// TestShippedTipBlocksAreAtMostThreeRowsAndFitTheColumn pins the shape every
// shipped tip has to keep: two or three rows, none of them wider than the column,
// at the narrowest panel and at the widest, so the rotation cannot make a panel
// jump and no tip is cut at the gutter. From tipMinPanelWidth up, which is every
// width a two-column screen can compose, no shipped tip may end in the cut marker
// either: the pool admitted only whole tips.
func TestShippedTipBlocksAreAtMostThreeRowsAndFitTheColumn(t *testing.T) {
	for _, width := range []int{20, tipMinPanelWidth, 84} {
		for i, tip := range tipPool() {
			rows := tipRows(tip, width)
			if len(rows) > 3 {
				t.Errorf("tip %d rendered %d rows at %d columns, want at most 3: %+v", i, len(rows), width, tip)
			}
			for _, row := range rows {
				if got := lipgloss.Width(row); got > width {
					t.Errorf("tip %d rendered a %d-column row in a %d-column column: %q", i, got, width, row)
				}
			}
			if width >= tipMinPanelWidth && strings.Contains(panelText(rows), cutMarker) {
				t.Errorf("tip %d is cut at %d columns, a width the layout composes: %+v", i, width, tip)
			}
		}
	}
}

// TestTipPanelShowsTheTipTheTickSelects pins the panel end to end: tip 0 at tick
// 0, the next tip once another ten ticks have passed, and the same tip again
// after the pool wraps.
func TestTipPanelShowsTheTipTheTickSelects(t *testing.T) {
	l := narrowPanelLayout()
	pool := tipPool()

	first := Model{AnimTick: 0}.tipFacts(l)
	if !strings.Contains(panelFlat(first), pool[0].Action) {
		t.Errorf("the panel at tick 0 does not show tip 0 %+v:\n%s", pool[0], panelText(first))
	}

	second := Model{AnimTick: ticksPerTip}.tipFacts(l)
	if !strings.Contains(panelFlat(second), pool[1].Action) || strings.Contains(panelFlat(second), pool[0].Action) {
		t.Errorf("the panel at tick %d does not show tip 1 %+v:\n%s", ticksPerTip, pool[1], panelText(second))
	}

	wrapped := Model{AnimTick: len(pool) * ticksPerTip}.tipFacts(l)
	if panelFlat(wrapped) != panelFlat(first) {
		t.Errorf("the panel did not wrap back to tip 0:\n%s", panelText(wrapped))
	}
}

// TestLastInstallPanelNamesTheRunItWasGiven pins the state panel's rows: when the
// run finished, the build it ran, and the files it touched -- and no files row
// when it touched none, so absence is not drawn as a zero.
func TestLastInstallPanelNamesTheRunItWasGiven(t *testing.T) {
	l := narrowPanelLayout()
	when := time.Date(2026, 9, 29, 14, 30, 0, 0, time.UTC)

	m := Model{LastInstall: &lastInstall{
		Timestamp: when,
		Version:   "v0.4.0",
		Files:     []string{"nvim: /home/testuser/.config/nvim"},
	}}
	rows := m.lastInstallPanelFacts(l)
	assertPanelFits(t, rows, l.Right, 200)
	flat := panelFlat(rows)
	for _, want := range []string{
		"Ran 2026-09-29 14:30:00",
		"Version v0.4.0",
		"Touched 1 file",
		"nvim: /home/testuser/.config/nvim",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("the state panel does not name %q:\n%s", want, panelText(rows))
		}
	}

	bare := Model{LastInstall: &lastInstall{Timestamp: when, Version: "v0.4.0"}}
	text := panelText(bare.lastInstallPanelFacts(l))
	if strings.Contains(text, "Touched") || strings.Contains(text, "never") {
		t.Errorf("a run that touched nothing rendered a zero or a \"never\":\n%s", text)
	}
	if !strings.Contains(panelFlat(bare.lastInstallPanelFacts(l)), "Ran 2026-09-29 14:30:00") {
		t.Errorf("a run with no files still has to name when it ran:\n%s", text)
	}

	if rows := (Model{}).lastInstallPanelFacts(l); rows != nil {
		t.Errorf("a model with no record rendered %v, want no rows", panelText(rows))
	}
}

// TestTheFullMainMenuTabRowFitsTheNarrowestTwoColumnPanel pins the widest tab
// row the shipped registry can build against the narrowest panel it can be drawn
// in. The four titles together are one character from the column's edge at 124
// terminal columns, so a title that grows -- or a fifth panel -- must fail here
// with its own name rather than silently fall back to a truncated row.
func TestTheFullMainMenuTabRowFitsTheNarrowestTwoColumnPanel(t *testing.T) {
	m := Model{
		Screen:      ScreenMainMenu,
		LastInstall: &lastInstall{Timestamp: time.Now(), Version: "v0.4.0"},
	}
	panels := m.panelsFor()
	if len(panels) != 4 {
		t.Fatalf("the main menu offers %d panels, want 4 with a record", len(panels))
	}

	// 124 terminal columns is the two-column floor; its panel is the narrowest one.
	l := layoutFor(Model{Width: layoutTwoColumnWidth + 4})
	if !l.TwoColumn {
		t.Fatalf("the two-column floor did not lay out as two columns")
	}

	row := tabRow(panels, 0, l.Right)
	if got := lipgloss.Width(row); got > l.Right {
		t.Errorf("the tab row is %d columns in a %d-column panel: %q", got, l.Right, panelText([]string{row}))
	}
	if strings.Contains(panelText([]string{row}), cutMarker) {
		t.Errorf("the tab row fell back to a truncated row in a %d-column panel: %q", l.Right, panelText([]string{row}))
	}
	for _, p := range panels {
		if !strings.Contains(panelText([]string{row}), p.Title) {
			t.Errorf("the tab row does not name %q: %q", p.Title, panelText([]string{row}))
		}
	}
}

// --- the greeting by time of day -------------------------------------------

// nonBlankLines strips a view's styling and its blank rows, so a comparison
// between two renders reads the content instead of the padding that centring
// shifts around.
func nonBlankLines(view string) []string {
	var out []string
	for _, line := range strings.Split(ansiEscape.ReplaceAllString(view, ""), "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

// TestGreetingForEachPartOfTheDay pins every part of the day the welcome and
// main menu greet by. The function is pure, so the assertion is a table of
// instants rather than something that waits for an hour to pass. A zero time has
// no greeting at all, which is what keeps a model built without a creation time
// from guessing an hour.
func TestGreetingForEachPartOfTheDay(t *testing.T) {
	at := func(h, m int) time.Time {
		return time.Date(2026, time.March, 14, h, m, 0, 0, time.UTC)
	}
	cases := []struct {
		name string
		when time.Time
		want string
	}{
		{"the first minute of the morning", at(5, 0), "Good morning"},
		{"the last minute of the morning", at(11, 59), "Good morning"},
		{"the first minute of the afternoon", at(12, 0), "Good afternoon"},
		{"the last minute of the afternoon", at(17, 59), "Good afternoon"},
		{"the first minute of the evening", at(18, 0), "Good evening"},
		{"the last minute of the evening", at(23, 59), "Good evening"},
		{"just after midnight", at(0, 0), "Good evening"},
		{"no time at all", time.Time{}, ""},
	}
	for _, c := range cases {
		if got := greetingFor(c.when); got != c.want {
			t.Errorf("%s: greetingFor(%s) = %q, want %q", c.name, c.when.Format(time.RFC3339), got, c.want)
		}
	}
}

// TestGreetingComesFromTheModelNotTheClock pins that the greeting is model state
// and not a clock read inside the renderer: two renders of one model produce the
// same bytes, and a model with no creation time shows no greeting line. A
// renderer that called time.Now could not be snapshotted and would change the
// bytes when nothing else did.
func TestGreetingComesFromTheModelNotTheClock(t *testing.T) {
	for _, screen := range []Screen{ScreenWelcome, ScreenMainMenu} {
		screen := screen
		t.Run(fmt.Sprintf("screen %d", screen), func(t *testing.T) {
			m := NewModel()
			isolateGoldenTest(t, &m)
			m.Screen = screen
			m.Width, m.Height = 80, 24

			greeting := greetingFor(m.CreatedAt)
			if greeting == "" {
				t.Fatal("the pinned creation time produced no greeting")
			}
			first := ansiEscape.ReplaceAllString(m.View(), "")
			if !strings.Contains(first, greeting) {
				t.Errorf("the screen does not show the greeting from the model's creation time:\n%s", first)
			}
			if second := m.View(); second != m.View() {
				t.Errorf("two renders of one model differ")
			}

			noClock := m
			noClock.CreatedAt = time.Time{}
			if lines := ansiEscape.ReplaceAllString(noClock.View(), ""); strings.Contains(lines, greeting) {
				t.Errorf("a model with no creation time showed a greeting:\n%s", lines)
			}
		})
	}
}

// TestGreetingIsOnlyOnTheWelcomeAndTheMenu pins that the added line reaches the
// two screens the design places it on and nowhere else, so no other snapshot may
// move for it.
func TestGreetingIsOnlyOnTheWelcomeAndTheMenu(t *testing.T) {
	greeting := greetingFor(goldenGreetingTime)
	if greeting == "" {
		t.Fatal("the golden greeting time produced no greeting")
	}
	for _, name := range installerFrameScreenNames {
		view := ansiEscape.ReplaceAllString(installerFrameCase(t, name).View(), "")
		want := name == "welcome" || name == "main-menu" || name == "main-menu-restore"
		if got := strings.Contains(view, greeting); got != want {
			t.Errorf("screen %q shows the greeting = %v, want %v", name, got, want)
		}
	}
}

// TestGreetingIsAnAddedLineThatMovesNothingElse pins the snapshot move the
// greeting causes. In one column -- the 80-column floor, where the menu has no
// panel -- the rendered lines are the lines without the greeting with exactly one
// line inserted. In two columns the panel is a pure read of the model, so it is
// byte-identical with and without the greeting; only the body gains the row.
func TestGreetingIsAnAddedLineThatMovesNothingElse(t *testing.T) {
	bare := func(screen Screen, width, height int) Model {
		m := NewModel()
		isolateGoldenTest(t, &m)
		m.Screen = screen
		m.Width, m.Height = width, height
		return m
	}

	t.Run("one column gains exactly the one line", func(t *testing.T) {
		with := bare(ScreenMainMenu, 80, 24)
		without := with
		without.CreatedAt = time.Time{}

		greeting := greetingFor(with.CreatedAt)
		got := nonBlankLines(with.View())
		want := nonBlankLines(without.View())

		withoutGreeting := make([]string, 0, len(got))
		for _, line := range got {
			if strings.TrimSpace(line) == greeting {
				continue
			}
			withoutGreeting = append(withoutGreeting, line)
		}
		if !reflect.DeepEqual(withoutGreeting, want) {
			t.Errorf("the greeting moved more than itself:\nwith it, less the line:\n%s\nwithout it:\n%s",
				strings.Join(withoutGreeting, "\n"), strings.Join(want, "\n"))
		}
		if len(got) != len(want)+1 {
			t.Errorf("the greeting added %d lines, want 1", len(got)-len(want))
		}
	})

	t.Run("two columns leave the panel untouched", func(t *testing.T) {
		with := bare(ScreenMainMenu, 160, 50)
		without := with
		without.CreatedAt = time.Time{}

		l := layoutFor(with)
		hints := with.panelHints(with.panelsFor(), []installerHint{hintUp, hintDown, hintSelect, hintQuit})
		budget := installerBodyRows(with.Height, footerRowCount(l.Inner, hints))

		withPanel := with.panelColumn(with.panelsFor(), l, budget)
		withoutPanel := without.panelColumn(without.panelsFor(), l, budget)
		if !reflect.DeepEqual(withPanel, withoutPanel) {
			t.Errorf("the greeting changed the panel:\nwith it:\n%s\nwithout it:\n%s",
				panelText(withPanel), panelText(withoutPanel))
		}
	})
}

// --- The panel follows the selection -----------------------------------------

// contextualMainMenuModel is a main menu with every kind of state the panel can
// be pointed at: a host to plan for, a config to overwrite and two backups to
// restore, so every option under the cursor has real facts to show.
func contextualMainMenuModel() Model {
	return Model{
		Screen:          ScreenMainMenu,
		SystemInfo:      goldenSystemInfo(),
		ExistingConfigs: []string{"nvim: /home/testuser/.config/nvim"},
		AvailableBackups: []system.BackupInfo{
			{
				Path:      "/home/testuser/.dotfiles-backup-20240103-120000",
				Timestamp: time.Date(2024, 1, 3, 12, 0, 0, 0, time.UTC),
				Files:     []string{"nvim", "fish"},
			},
			{
				Path:      "/home/testuser/.dotfiles-backup-20240101-120000",
				Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				Files:     []string{"nvim"},
			},
		},
	}
}

// TestMainMenuPanelDescribesTheOptionUnderTheCursor pins the point of the slice:
// with the cursor on a main-menu option, the panel names that option and shows
// what it holds, and it shows nothing the option does not hold.
func TestMainMenuPanelDescribesTheOptionUnderTheCursor(t *testing.T) {
	l := narrowPanelLayout()
	modules, lessons, bosses := trainerCurriculum()

	cases := []struct {
		cursor int
		name   string
		want   []string
		absent []string
	}{
		{
			cursor: 0,
			name:   "Start Installation",
			want: []string{
				"Selected Start Installation",
				"Steps 8",
				"Install Dependencies",
				"Overwrites 1 config",
				"nvim: /home/testuser/.config/nvim",
				"Newest backup",
				"2024-01-03 12:00:00, 2 files",
			},
		},
		{
			cursor: 1,
			name:   "Learn About Tools",
			want: []string{
				"Selected Learn About Tools",
				fmt.Sprintf("Terminals %d", len(GetTerminalInfo())),
				fmt.Sprintf("Shells %d", len(GetShellInfo())),
				fmt.Sprintf("Multiplexers %d", len(GetWMInfo())),
			},
			absent: []string{"Steps", "Overwrites", "Newest backup"},
		},
		{
			cursor: 2,
			name:   "Keymaps Reference",
			want: []string{
				"Selected Keymaps Reference",
				fmt.Sprintf("Neovim %s", keymapBindingCount(keymapCount(GetNvimKeymaps()))),
				"Ghostty",
			},
			absent: []string{"Steps", "Terminals"},
		},
		{
			cursor: 3,
			name:   "LazyVim Guide",
			want: []string{
				"Selected LazyVim Guide",
				fmt.Sprintf("Topics %d", len(GetLazyVimTopics())),
			},
			absent: []string{"Steps", "Neovim"},
		},
		{
			cursor: 4,
			name:   "Vim Trainer",
			want: []string{
				"Selected Vim Trainer",
				fmt.Sprintf("Modules %d", modules),
				fmt.Sprintf("Lessons %d", lessons),
				fmt.Sprintf("Bosses %d", bosses),
			},
			absent: []string{"Steps", "Overwrites"},
		},
		{
			cursor: 5,
			name:   "Restore from Backup",
			want: []string{
				"Selected Restore from Backup",
				"Backups 2",
				"2024-01-03 12:00:00, 2 files",
				"2024-01-01 12:00:00, 1 file",
			},
			absent: []string{"Steps", "Overwrites", "Modules"},
		},
		{
			cursor: 6,
			name:   "Exit",
			want: []string{
				"Selected Exit",
				"Nothing on this machine changes.",
			},
			absent: []string{"Steps", "Overwrites", "Newest backup", "Backups", "Modules", "Topics", "Terminals"},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			m := contextualMainMenuModel()
			m.Cursor = c.cursor
			rows := m.mainMenuPanel(l, 200)
			assertPanelFits(t, rows, l.Right, 200)
			flat := panelFlat(rows)
			for _, want := range c.want {
				if !strings.Contains(flat, want) {
					t.Errorf("the panel for %q does not show %q:\n%s", c.name, want, panelText(rows))
				}
			}
			for _, absent := range c.absent {
				if strings.Contains(flat, absent) {
					t.Errorf("the panel for %q shows %q, which that option does not hold:\n%s", c.name, absent, panelText(rows))
				}
			}
		})
	}
}

// TestMainMenuPanelNameStaysStableAcrossTheCursor pins the second half of the
// design: the panel's name and the tab row are the same for every selection, so
// Tab is not a moving target, and the option's name lives inside the panel.
func TestMainMenuPanelNameStaysStableAcrossTheCursor(t *testing.T) {
	l := narrowPanelLayout()
	m := contextualMainMenuModel()
	if panels := m.panelsFor(); len(panels) == 0 || panels[0].ID != panelPlan {
		t.Fatalf("the main menu's first panel is not the plan panel")
	}
	wantTab := tabRow(m.panelsFor(), 0, l.Right)

	for cursor := range m.GetCurrentOptions() {
		m.Cursor = cursor
		got := m.panelsFor()
		if got[0].Title != mainMenuPanelLabel || got[0].Short != "Plan" {
			t.Errorf("with the cursor at %d the panel's name is (%q, %q), want (%q, Plan)",
				cursor, got[0].Title, got[0].Short, mainMenuPanelLabel)
		}
		if row := tabRow(got, 0, l.Right); row != wantTab {
			t.Errorf("with the cursor at %d the tab row moved:\n%q\n%q",
				cursor, panelText([]string{wantTab}), panelText([]string{row}))
		}
	}
}

// visibleSlice returns the columns [start, end) of a plain (escape-free) line,
// counting columns the way the terminal does so a wide glyph is one slice.
func visibleSlice(line string, start, end int) string {
	var b strings.Builder
	col := 0
	for _, r := range line {
		w := lipgloss.Width(string(r))
		if col >= start && col+w <= end {
			b.WriteRune(r)
		}
		col += w
	}
	return b.String()
}

// TestMovingTheCursorChangesOnlyThePanel pins the rule the design depends on:
// the cursor move changes the panel column and nothing else on the screen. The
// frame is split at the gutter the layout computed, the selection marker is
// normalised away first because that is the cursor's own highlight, and a row
// that moved has to have moved in the panel's column.
func TestMovingTheCursorChangesOnlyThePanel(t *testing.T) {
	m := contextualMainMenuModel()
	m.Width, m.Height = 160, 50

	first := m
	first.Cursor = 0 // Start Installation
	second := m
	second.Cursor = 5 // Restore from Backup, whose panel holds other facts

	l := layoutFor(m)
	if !l.TwoColumn {
		t.Fatalf("the main menu at 160x50 lays out as one column")
	}
	// View() pads two columns on the left and adds one blank row above the frame.
	const viewPad = 2
	bodyStart := viewPad + l.Leading
	bodyEnd := bodyStart + l.Left
	panelStart := bodyEnd + layoutGutter
	panelEnd := panelStart + l.Right

	strip := func(view string) []string {
		plain := ansiEscape.ReplaceAllString(view, "")
		plain = strings.ReplaceAll(plain, "▸ ", "  ")
		return strings.Split(plain, "\n")
	}
	rowsFirst := strip(first.View())
	rowsSecond := strip(second.View())
	if len(rowsFirst) != len(rowsSecond) {
		t.Fatalf("the cursor changed the row count from %d to %d", len(rowsFirst), len(rowsSecond))
	}

	panelChanged, bodyMoved := false, false
	for i := range rowsFirst {
		if rowsFirst[i] == rowsSecond[i] {
			continue
		}
		if got, want := visibleSlice(rowsFirst[i], bodyStart, bodyEnd), visibleSlice(rowsSecond[i], bodyStart, bodyEnd); got != want {
			bodyMoved = true
			t.Errorf("row %d changed in the body column:\n%q\n%q", i, got, want)
		}
		if visibleSlice(rowsFirst[i], panelStart, panelEnd) != visibleSlice(rowsSecond[i], panelStart, panelEnd) {
			panelChanged = true
		}
	}
	if bodyMoved {
		t.Errorf("the cursor move changed a body row")
	}
	if !panelChanged {
		t.Errorf("the cursor move did not change the panel column")
	}
}

// TestNarrowSummaryFollowsTheSelection pins the same rule for the one-line
// summary a narrow terminal gets: its headline is the selected option's answer,
// not the plan's, while the panel's short name stays put.
func TestNarrowSummaryFollowsTheSelection(t *testing.T) {
	m := contextualMainMenuModel()
	m.Width = 100
	m.Cursor = 5 // Restore from Backup

	lines := m.rotatorLines(m.panelsFor(), 96)
	text := panelText(lines)
	if !strings.Contains(text, "Plan") {
		t.Errorf("the summary does not name the panel: %q", text)
	}
	if !strings.Contains(text, "2 backups to restore") {
		t.Errorf("the summary does not carry the selection's headline: %q", text)
	}
}

// TestWizardChoicePanelDescribesTheChoiceUnderTheCursor pins the wizard half:
// each question names the choice the cursor is on and shows the plan that choice
// would lead to, with the step the plan is on still marked.
func TestWizardChoicePanelDescribesTheChoiceUnderTheCursor(t *testing.T) {
	l := narrowPanelLayout()

	t.Run("the operating system question previews the highlighted OS", func(t *testing.T) {
		m := Model{Screen: ScreenOSSelect, SystemInfo: goldenSystemInfo()}

		m.Cursor = 0 // macOS
		mac := panelFlat(m.mainMenuPanel(l, 200))
		for _, want := range []string{"Selected macOS", "on macOS", "Install Xcode CLI"} {
			if !strings.Contains(mac, want) {
				t.Errorf("the macOS choice panel does not show %q:\n%s", want, mac)
			}
		}
		if !strings.Contains(mac, "▸") {
			t.Errorf("the macOS choice panel does not mark the step the plan is on:\n%s", mac)
		}

		m.Cursor = 1 // Linux, what detection found
		linux := panelFlat(m.mainMenuPanel(l, 200))
		for _, want := range []string{"Selected Linux", "Install Dependencies"} {
			if !strings.Contains(linux, want) {
				t.Errorf("the Linux choice panel does not show %q:\n%s", want, linux)
			}
		}
		if strings.Contains(linux, "Xcode") {
			t.Errorf("the Linux choice panel shows the macOS plan:\n%s", linux)
		}
		if strings.Contains(linux, "(detected)") {
			t.Errorf("the Linux choice panel labels a host the player is choosing as detected:\n%s", linux)
		}
	})

	t.Run("the shell question previews the highlighted shell", func(t *testing.T) {
		for _, c := range []struct {
			cursor int
			want   string
		}{
			{0, "Selected Fish"},
			{1, "Selected Zsh"},
			{2, "Selected Nushell"},
		} {
			m := Model{Screen: ScreenShellSelect, SystemInfo: goldenSystemInfo(), Choices: UserChoices{OS: "linux"}}
			m.Cursor = c.cursor
			flat := panelFlat(m.mainMenuPanel(l, 200))
			if !strings.Contains(flat, c.want) {
				t.Errorf("the shell panel at cursor %d does not name the choice %q:\n%s", c.cursor, c.want, flat)
			}
			// The panel still answers what the choice would do: the plan's count is on
			// screen and the step the run would start at is named.
			if !strings.Contains(flat, "Steps") || !strings.Contains(flat, "Install Dependencies") {
				t.Errorf("the shell panel at cursor %d lost the plan:\n%s", c.cursor, flat)
			}
		}
	})
}

// TestMainMenuPanelFitsTheFrameForEverySelection pins the frame at every cursor
// position and every two-column size: one selection's panel is longer than
// another's, and the longest may not push the footer off the frame. The shipped
// guards render the menu at cursor zero only, so this covers the selections they
// never reach.
func TestMainMenuPanelFitsTheFrameForEverySelection(t *testing.T) {
	sizes := []struct{ width, height int }{
		{124, 24}, // the two-column floor
		{160, 50},
		{227, 62},
	}
	for _, size := range sizes {
		for cursor := range contextualMainMenuModel().GetCurrentOptions() {
			m := contextualMainMenuModel()
			m.Width, m.Height = size.width, size.height
			m.Cursor = cursor
			view := m.View()
			if rows := renderedRowCount(view); rows != size.height {
				t.Errorf("the main menu with cursor %d at %dx%d renders %d rows, want %d",
					cursor, size.width, size.height, rows, size.height)
			}
			for _, line := range strings.Split(view, "\n") {
				if w := lipgloss.Width(line); w > size.width {
					t.Errorf("the main menu with cursor %d at %dx%d renders a %d-column line: %q",
						cursor, size.width, size.height, w, line)
				}
			}
		}
	}
}
