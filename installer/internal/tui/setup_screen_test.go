package tui

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
	tea "github.com/charmbracelet/bubbletea"
)

// setupGuardQuestionScreens is the six screens the setup screen replaced as the
// flow's steps. They stay, as detours of the setup screen, and every guard below
// is written over this list so a screen that loses its place is named.
var setupGuardQuestionScreens = []Screen{
	ScreenOSSelect,
	ScreenTerminalSelect,
	ScreenFontSelect,
	ScreenShellSelect,
	ScreenWMSelect,
	ScreenNvimSelect,
}

// setupGuardRowIndex finds a row in a setup screen's own list, so a guard walks
// the rows the renderer and the keyboard read rather than an index written here.
func setupGuardRowIndex(t *testing.T, m Model, label string) int {
	t.Helper()
	for i, row := range m.setupRows() {
		if row.Label == label {
			return i
		}
	}
	t.Fatalf("the setup screen has no %q row: %v", label, m.setupRows())
	return -1
}

// TestTheSetupScreenShowsEveryDecisionRowWithItsValue is the guard for S2's
// interface half: every decision the flow takes is a row of the setup screen, and
// the row carries the value the choices hold. A decision that loses its row, or
// whose row stops following the choice it shows, fails here by name instead of
// leaving the plan to invent an answer nobody gave.
func TestTheSetupScreenShowsEveryDecisionRowWithItsValue(t *testing.T) {
	cases := []struct {
		row   string
		value string
		m     Model
	}{
		{"Platform", "Termux", Model{Choices: UserChoices{OS: "termux"}, SystemInfo: &system.SystemInfo{OS: system.OSTermux}}},
		{"Shell", "Zsh", Model{Choices: UserChoices{Shell: "zsh"}}},
		{"Terminal", "Ghostty", Model{Choices: UserChoices{OS: "linux", Terminal: "ghostty"}}},
		{"Font", "[x] install", Model{Choices: UserChoices{InstallFont: true}}},
		{"Multiplexer", "Zellij", Model{Choices: UserChoices{WindowMgr: "zellij"}}},
		{"Editor (Neovim)", "[x] install", Model{Choices: UserChoices{InstallNvim: true}}},
		{"Existing configurations", "[x] back up", Model{Choices: UserChoices{CreateBackup: true}, ExistingConfigs: []string{"nvim: ~/.config/nvim"}}},
	}

	for _, c := range cases {
		t.Run(c.row, func(t *testing.T) {
			m := c.m
			m.Screen = ScreenSetup
			m.Width, m.Height = 80, 24

			rows := m.setupRows()
			found := false
			var value string
			for _, row := range rows {
				if row.Label == c.row {
					found = true
					value = m.setupRowValue(row)
				}
			}
			if !found {
				t.Fatalf("the setup screen has no %q row: the decisions are %v", c.row, setupGuardRowLabels(rows))
			}
			if value != c.value {
				t.Errorf("the %s row shows %q, want the value the choices hold, %q", c.row, value, c.value)
			}

			view := ansiEscape.ReplaceAllString(m.View(), "")
			if !strings.Contains(view, c.row) {
				t.Errorf("the rendered setup screen does not draw the %q row:\n%s", c.row, view)
			}
			if !strings.Contains(view, c.value) {
				t.Errorf("the rendered setup screen does not carry %q on the %s row:\n%s", c.value, c.row, view)
			}
		})
	}

	// The action row exists and carries no value: pressing it decides nothing to
	// show.
	m := Model{Screen: ScreenSetup, Width: 80, Height: 24}
	rows := m.setupRows()
	action := rows[len(rows)-1]
	if action.Kind != setupRowAction {
		t.Fatalf("the setup screen's last row is %q, want the action row", action.Label)
	}
	if value := m.setupRowValue(action); value != "" {
		t.Errorf("the action row %q carries the value %q, want none", action.Label, value)
	}
}

// setupGuardRowLabels names the rows in a failure message, because a guard that
// fails has to say what it saw.
func setupGuardRowLabels(rows []setupRow) []string {
	labels := make([]string, len(rows))
	for i, row := range rows {
		labels[i] = row.Label
	}
	return labels
}

// TestTheSetupScreenOffersTheBackupRowOnlyWhenThereIsSomethingToBackUp pins the
// one conditional row: the backup decision appears exactly when the machine holds
// a configuration the run would replace, and it disappears with the reason when
// there is nothing to decide.
func TestTheSetupScreenOffersTheBackupRowOnlyWhenThereIsSomethingToBackUp(t *testing.T) {
	withConfigs := Model{Screen: ScreenSetup, ExistingConfigs: []string{"nvim: ~/.config/nvim"}}
	if idx := setupGuardRowIndex(t, withConfigs, "Existing configurations"); idx < 0 {
		t.Error("the setup screen does not offer the backup row though the machine holds a configuration")
	}

	withoutConfigs := Model{Screen: ScreenSetup}
	for _, row := range withoutConfigs.setupRows() {
		if row.Label == "Existing configurations" {
			t.Error("the setup screen offers the backup row though there is no configuration to back up")
		}
	}
}

// pluginSetLabel is a registered set's row label, read from the registry the row list
// itself derives its rows from, so a renamed set moves the guard with it.
func pluginSetLabel(t *testing.T, id string) string {
	t.Helper()
	set, ok := pluginSetByID(id)
	if !ok {
		t.Fatalf("the registry no longer holds %q", id)
	}
	return set.Label()
}

// TestTheSetupScreenNamesThePluginSetOfTheToolItInstalls is the S3 guard for the row
// list: the set a tool's step installs is named under that tool, a tool nobody chose
// contributes no row, and a tool with no registered set gets none.
func TestTheSetupScreenNamesThePluginSetOfTheToolItInstalls(t *testing.T) {
	cases := []struct {
		name    string
		choices UserChoices
		want    []string
		absent  []string
	}{
		{
			name:    "zsh, tmux and Neovim carry three sets",
			choices: UserChoices{OS: "linux", Shell: "zsh", Terminal: "kitty", WindowMgr: "tmux", InstallNvim: true},
			want:    []string{pluginSetIDShell, pluginSetIDTmux, pluginSetIDNvim},
		},
		{
			name:    "fish and herdr carry theirs",
			choices: UserChoices{OS: "linux", Shell: "fish", Terminal: "kitty", WindowMgr: "herdr"},
			want:    []string{pluginSetIDFish, pluginSetIDHerdr},
			absent:  []string{pluginSetIDShell, pluginSetIDTmux, pluginSetIDNvim},
		},
		{
			name:    "a tool this installer provisions nothing for gets no set row",
			choices: UserChoices{OS: "linux", Shell: "nushell", Terminal: "kitty", WindowMgr: "zellij"},
			absent:  []string{pluginSetIDShell, pluginSetIDFish, pluginSetIDTmux, pluginSetIDHerdr, pluginSetIDNvim},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := Model{Screen: ScreenSetup, SystemInfo: &system.SystemInfo{OS: system.OSLinux}, Choices: c.choices}
			rows := m.setupRows()

			named := map[string]bool{}
			for _, row := range rows {
				if row.Kind == setupRowPluginSet {
					named[row.SetID] = true
				}
			}
			for _, id := range c.want {
				if !named[id] {
					t.Errorf("the setup screen does not name the plugin set %q, which this tool's step installs", id)
				}
			}
			for _, id := range c.absent {
				if named[id] {
					t.Errorf("the setup screen names the plugin set %q, which this run does not install", id)
				}
			}

			// A set sits directly under the tool whose step installs it, not in a
			// section of its own: the row above a set row is its tool's row.
			for i, row := range rows {
				if row.Kind != setupRowPluginSet {
					continue
				}
				if i == 0 {
					t.Errorf("the set %q is the list's first row: it belongs under its tool", row.SetID)
					continue
				}
				if parent, ok := m.pluginSetForRow(rows[i-1]); !ok || parent.ID != row.SetID {
					t.Errorf("the row above the set %q is not the row of the tool that installs it", row.SetID)
				}
			}
		})
	}
}

// TestTheSetupScreenCanTurnAProvisionedSetOff holds S3 where it reaches the run: the
// key on the row writes the same list the flag writes, the plan carries the state the
// executor's guard reads, and turning it back on clears it.
func TestTheSetupScreenCanTurnAProvisionedSetOff(t *testing.T) {
	m := Model{
		Screen:     ScreenSetup,
		SystemInfo: &system.SystemInfo{OS: system.OSLinux},
		Choices:    UserChoices{OS: "linux", Shell: "zsh", Terminal: "kitty", WindowMgr: "tmux"},
	}
	m.SetupCursor = setupGuardRowIndex(t, m, pluginSetLabel(t, pluginSetIDTmux))

	m.toggleSetupRow()
	if !pluginSetIsOff(m.Choices.DisabledPluginSets, pluginSetIDTmux) {
		t.Fatalf("turning the multiplexer's set off recorded %v, want %q", m.Choices.DisabledPluginSets, pluginSetIDTmux)
	}
	step := pluginSetOnStep(t, planFor(m.planOptions()), "wm")
	if step.PluginSet == nil || !step.PluginSet.Off {
		t.Errorf("the plan does not carry the set the screen turned off: %+v", step.PluginSet)
	}
	if value := m.setupRowValue(m.setupRows()[m.SetupCursor]); !strings.Contains(value, "leave it out") {
		t.Errorf("the row says %q after the set was turned off, want it to say the run leaves it out", value)
	}

	m.toggleSetupRow()
	if pluginSetIsOff(m.Choices.DisabledPluginSets, pluginSetIDTmux) {
		t.Errorf("turning the set back on left %v", m.Choices.DisabledPluginSets)
	}
}

// TestTheSetupScreenCanTurnTheShellsSetOff holds the user's decision at L13 where the
// run reads it: the shell's set follows the registry, so the row that said `always
// installed` is a switch whose key writes the same list `--skip=plugins-shell` writes,
// and the plan the executor reads carries it as left out.
func TestTheSetupScreenCanTurnTheShellsSetOff(t *testing.T) {
	set, ok := pluginSetByID(pluginSetIDShell)
	if !ok {
		t.Fatal("the registry does not name the shell's plugin set")
	}
	if !set.Switchable {
		t.Fatalf("the shell's set is carried as Switchable=%v, so the row offers no switch to turn it off", set.Switchable)
	}

	m := Model{
		Screen:     ScreenSetup,
		SystemInfo: &system.SystemInfo{OS: system.OSLinux},
		Choices:    UserChoices{OS: "linux", Shell: "zsh", Terminal: "kitty", WindowMgr: "none"},
	}
	m.SetupCursor = setupGuardRowIndex(t, m, pluginSetLabel(t, pluginSetIDShell))

	if !m.toggleSetupRow() {
		t.Fatal("the shell's set is a switch now, but the key on its row changed nothing")
	}
	if !pluginSetIsOff(m.Choices.DisabledPluginSets, pluginSetIDShell) {
		t.Fatalf("turning the shell's set off recorded %v, want %q", m.Choices.DisabledPluginSets, pluginSetIDShell)
	}
	step := pluginSetOnStep(t, planFor(m.planOptions()), "shell")
	if step.PluginSet == nil || !step.PluginSet.Off {
		t.Errorf("the plan does not carry the shell's set as left out: %+v", step.PluginSet)
	}
	if value := m.setupRowValue(m.setupRows()[m.SetupCursor]); !strings.Contains(value, "leave it out") {
		t.Errorf("the row says %q after the set was turned off, want it to say the run leaves it out", value)
	}

	m.toggleSetupRow()
	if pluginSetIsOff(m.Choices.DisabledPluginSets, pluginSetIDShell) {
		t.Errorf("turning the shell's set back on left %v", m.Choices.DisabledPluginSets)
	}
}

// TestTheSetupScreenGivesNoKeyToASetTheRunCannotLeaveOut is the other half of S3: a
// set a run cannot leave out is named and explained, and no key offers to turn it
// off, because that key would install nothing of what it promised. It is held over
// fish's set -- the shell's set is a switch now (T7), so it is covered like the
// others instead.
func TestTheSetupScreenGivesNoKeyToASetTheRunCannotLeaveOut(t *testing.T) {
	m := Model{
		Screen:     ScreenSetup,
		SystemInfo: &system.SystemInfo{OS: system.OSLinux},
		Choices:    UserChoices{OS: "linux", Shell: "fish", Terminal: "kitty", WindowMgr: "none"},
	}
	m.SetupCursor = setupGuardRowIndex(t, m, pluginSetLabel(t, pluginSetIDFish))

	if m.toggleSetupRow() {
		t.Error("fish's set has a key that turns it off, though a run cannot leave it out")
	}
	if len(m.Choices.DisabledPluginSets) != 0 {
		t.Errorf("a set a run cannot leave out recorded %v", m.Choices.DisabledPluginSets)
	}
	if value := m.setupRowValue(m.setupRows()[m.SetupCursor]); !strings.Contains(value, "always installed") {
		t.Errorf("the row says %q about a set a run cannot leave out, want it to say so", value)
	}
}

// TestTheSetupWindowShowsTheRowTheCursorIsOnAndCountsWhatItHides is the plan's window
// rule applied to the screen whose row list grew with the plugin sets: on the
// twenty-row terminal the frame matrix measures, the list is a window over the rows,
// the cursor's row is drawn, and the rows outside the window are counted rather than
// dropped. The frame guard is what found this screen drawing one row more than the
// terminal had, so the window is the fix this guard holds.
func TestTheSetupWindowShowsTheRowTheCursorIsOnAndCountsWhatItHides(t *testing.T) {
	m := Model{
		Screen:          ScreenSetup,
		SystemInfo:      &system.SystemInfo{OS: system.OSLinux},
		Choices:         UserChoices{OS: "linux", Shell: "zsh", Terminal: "kitty", WindowMgr: "tmux", InstallNvim: true},
		ExistingConfigs: []string{"nvim: /home/user/.config/nvim"},
		Width:           60,
		Height:          20,
	}

	rows := m.setupRows()
	if len(rows) < 9 {
		t.Fatalf("the setup screen offers %d rows, too few for this guard to test a window", len(rows))
	}

	m.SetupCursor = 0
	first := m.renderSetup()
	if !strings.Contains(first, "Platform") {
		t.Error("with the cursor on the first row the screen does not draw it")
	}
	if strings.Contains(first, "more above") {
		t.Error("with the cursor on the first row the screen claims rows above the window")
	}
	if !strings.Contains(first, "more below") {
		t.Error("with the cursor on the first row the screen does not say how many rows the window leaves out")
	}

	m.SetupCursor = len(rows) - 1
	last := m.renderSetup()
	if !strings.Contains(last, "Continue to the plan") {
		t.Error("with the cursor on the last row the screen does not draw it")
	}
	if strings.Contains(last, "more below") {
		t.Error("with the cursor on the last row the screen claims rows below the window")
	}
	if !strings.Contains(last, "more above") {
		t.Error("with the cursor on the last row the screen does not say how many rows the window leaves out")
	}
}

// TestEveryDetourRowReturnsToTheSetupScreen is the detour contract (T2): Enter on
// a row opens that row's rich screen and records where it was opened from, a
// choice there records the answer and returns to the setup screen instead of
// advancing along the old chain, and Escape returns without changing anything.
func TestEveryDetourRowReturnsToTheSetupScreen(t *testing.T) {
	base := Model{
		Screen:     ScreenSetup,
		SystemInfo: &system.SystemInfo{OS: system.OSLinux, UserShell: "fish"},
		Choices:    UserChoices{OS: "linux", Shell: "fish"},
		// The backup row is offered only when there is something to back up, so the
		// guard gives the model a configuration: a row that is hidden here would be a
		// row the detour contract never checks.
		ExistingConfigs: []string{"nvim: ~/.config/nvim"},
	}

	for _, row := range base.setupRows() {
		if row.Screen == 0 {
			continue
		}
		t.Run(row.Label, func(t *testing.T) {
			m := base
			m.SetupCursor = setupGuardRowIndex(t, m, row.Label)

			m.openSetupRow()
			if m.Screen != row.Screen {
				t.Fatalf("Enter on the %s row opened %s, want %s", row.Label, setupGuardScreenName(m.Screen), setupGuardScreenName(row.Screen))
			}
			if !m.SetupDetour || m.PrevScreen != ScreenSetup {
				t.Fatalf("the %s detour did not record the setup screen as where it came from (detour=%v prev=%v)",
					row.Label, m.SetupDetour, m.PrevScreen)
			}

			// A choice records the answer and returns to the setup screen. The key goes
			// through Update, which is the seam the application uses: the six question
			// screens are dispatched from Update to handleSelection, and the backup
			// screen to its own handler, so a guard that called one handler directly
			// would test a seam half the rows do not travel.
			choice := m
			if choice.Cursor < 0 {
				choice.Cursor = 0
			}
			answered, _ := choice.Update(tea.KeyMsg{Type: tea.KeyEnter})
			got := answered.(Model)
			if got.Screen != ScreenSetup {
				t.Errorf("a choice on the %s detour landed on %v, want the setup screen", row.Label, got.Screen)
			}
			if got.SetupDetour {
				t.Errorf("the %s detour is still marked open after the choice", row.Label)
			}

			// Escape returns to the setup screen without changing the answer.
			escaped := m
			before := escaped.Choices
			back, _ := escaped.Update(tea.KeyMsg{Type: tea.KeyEscape})
			left := back.(Model)
			if left.Screen != ScreenSetup {
				t.Errorf("Escape on the %s detour landed on %v, want the setup screen", row.Label, left.Screen)
			}
			if !reflect.DeepEqual(left.Choices, before) {
				t.Errorf("Escape on the %s detour changed the choices from %+v to %+v", row.Label, before, left.Choices)
			}
		})
	}
}

// TestEscapeFromTheSetupScreenLeavesForTheMainMenu pins the way out: Escape on
// the setup screen returns to the main menu and keeps the answers and the row the
// cursor was on. Leaving the screen is not discarding the work: the old handler
// zeroed both, so a reader who stepped out to check something came back to a
// screen that had forgotten every choice and moved its cursor.
func TestEscapeFromTheSetupScreenLeavesForTheMainMenu(t *testing.T) {
	choices := UserChoices{OS: "linux", Shell: "fish", Terminal: "ghostty", InstallFont: true, InstallNvim: true}
	m := Model{
		Screen:      ScreenSetup,
		Choices:     choices,
		SetupCursor: 3,
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	got := updated.(Model)
	if got.Screen != ScreenMainMenu {
		t.Errorf("Escape on the setup screen landed on %v, want the main menu", got.Screen)
	}
	if !reflect.DeepEqual(got.Choices, choices) {
		t.Errorf("Escape on the setup screen changed the choices to %+v, want them kept", got.Choices)
	}
	if got.SetupCursor != 3 {
		t.Errorf("Escape on the setup screen moved its cursor to %d, want it kept on 3", got.SetupCursor)
	}
}

// TestTheBackupRowOpensTheScreenThatListsTheConfigurations holds the decision taken
// when the backup question stopped being the flow's last screen: the decision is a
// row, and Enter on that row still shows what would be backed up. The row is a
// switch, so a guard that only walked the rows that open a screen would skip it --
// losing the screen would leave the list unreachable and nothing would fail.
func TestTheBackupRowOpensTheScreenThatListsTheConfigurations(t *testing.T) {
	const config = "nvim: /home/user/.config/nvim"
	m := Model{
		Screen:          ScreenSetup,
		SystemInfo:      &system.SystemInfo{OS: system.OSLinux, UserShell: "fish"},
		Choices:         UserChoices{OS: "linux", Shell: "fish", Terminal: "kitty"},
		ExistingConfigs: []string{config},
	}
	m.SetupCursor = setupGuardRowIndex(t, m, "Existing configurations")

	m.openSetupRow()
	if m.Screen != ScreenBackupConfirm {
		t.Fatalf("Enter on the backup row opened %s, want the screen that lists the configurations found",
			setupGuardScreenName(m.Screen))
	}
	// The list is the reason this screen is still reachable: it names the
	// configuration the run would replace. It is measured at a width where the path
	// is not truncated, so the assertion is about the data and not about the frame.
	m.Width, m.Height = 120, 40
	plain := ansiEscape.ReplaceAllString(m.View(), "")
	if !strings.Contains(plain, "Existing configurations") {
		t.Errorf("the screen the backup row opens is not the one that lists the configurations: %q", plain)
	}
	if !strings.Contains(plain, config) {
		t.Errorf("the screen the backup row opens does not name the configuration it would replace: %q", plain)
	}

	// Space still records the decision itself, so the row is both the answer and the
	// way to the list, and the plan reads the same value either way.
	for _, on := range []bool{true, false} {
		row := Model{
			Screen:          ScreenSetup,
			SystemInfo:      &system.SystemInfo{OS: system.OSLinux, UserShell: "fish"},
			Choices:         UserChoices{OS: "linux", Shell: "fish", Terminal: "kitty"},
			ExistingConfigs: []string{config},
		}
		row.SetupCursor = setupGuardRowIndex(t, row, "Existing configurations")
		row.Choices.CreateBackup = !on
		row.toggleSetupRow()
		if row.Choices.CreateBackup != on {
			t.Errorf("Space on the backup row recorded CreateBackup=%v, want %v", row.Choices.CreateBackup, on)
		}
	}
}

// TestTheFlowHasNoQuestionToQuestionTransition is guard for the chain this front
// removed (T2/S1): the setup screen is the only entry to the question screens, so
// neither Enter nor Escape on one may land on another. Before the setup screen,
// every screen's Enter advanced to the next question.
func TestTheFlowHasNoQuestionToQuestionTransition(t *testing.T) {
	for _, from := range setupGuardQuestionScreens {
		for _, key := range []string{"enter", "esc"} {
			t.Run(fmt.Sprintf("%d/%s", from, key), func(t *testing.T) {
				m := Model{
					Screen:     from,
					SystemInfo: &system.SystemInfo{OS: system.OSLinux, UserShell: "fish"},
					Choices:    UserChoices{OS: "linux", Shell: "fish"},
					Cursor:     0,
				}

				updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
				if key == "esc" {
					updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
				}
				got := updated.(Model)
				if got.Screen != ScreenSetup {
					t.Errorf("pressing %s on the %v screen landed on %v, want the setup screen: "+
						"no question screen may lead to another", key, from, got.Screen)
				}
			})
		}
	}
}

// setupGuardScreenName names a screen in a failure message. fmt prints a Screen
// value as "Unknown" because the type has no String method, and a guard that
// fails has to say which screen it saw.
func setupGuardScreenName(s Screen) string {
	return fmt.Sprintf("%d (%s)", s, Model{Screen: s}.GetScreenTitle())
}

// TestTheFlowHasExactlyTwoMandatorySteps is the meter's guard (T2): the wizard
// counts the setup screen and the plan, a detour inherits the meter of the screen
// it returns to, and a screen the flow does not walk has no meter at all. The
// count is checked against the flow's own screens rather than a number written
// here, so a step added to the meter without a screen (or a screen without a
// step) fails.
func TestTheFlowHasExactlyTwoMandatorySteps(t *testing.T) {
	mandatory := []struct {
		screen        Screen
		step, ofTotal int
	}{
		{ScreenSetup, 1, 2},
		{ScreenInstallReview, 2, 2},
	}

	if len(mandatory) != 2 {
		t.Fatalf("the mandatory list names %d screens, want the two the meter counts", len(mandatory))
	}
	for _, c := range mandatory {
		if current, total := (Model{Screen: c.screen}).wizardProgress(); current != c.step || total != c.ofTotal {
			t.Errorf("%s reports %d/%d, want %d/%d", setupGuardScreenName(c.screen), current, total, c.step, c.ofTotal)
		}
	}

	// A detour inherits the meter of the screen it returns to, which is the setup
	// screen.
	for _, screen := range append([]Screen{ScreenGhosttyWarning}, setupGuardQuestionScreens...) {
		if current, total := (Model{Screen: screen}).wizardProgress(); current != 1 || total != 2 {
			t.Errorf("the %s detour reports %d/%d, want the setup screen's 1/2", setupGuardScreenName(screen), current, total)
		}
	}

	// A screen the flow does not walk gets no filler meter.
	for _, screen := range []Screen{ScreenWelcome, ScreenMainMenu, ScreenInstalling, ScreenComplete} {
		if _, total := (Model{Screen: screen}).wizardProgress(); total != 0 {
			t.Errorf("%s reports a %d-step meter, want none: the flow does not count it", setupGuardScreenName(screen), total)
		}
	}

	// The meter is painted, not only modelled: the two mandatory screens draw it in
	// the header, so a screen that answers a step but shows the user none fails
	// here. The fixtures are the frame guards' own, at a size where the header has
	// room for the chip.
	for _, c := range []struct {
		name          string
		step, ofTotal int
	}{
		{"setup", 1, 2},
		{"install-review", 2, 2},
	} {
		m := installerFrameCase(t, c.name)
		m.Width, m.Height = 100, 30
		want := fmt.Sprintf("%d/%d", c.step, c.ofTotal)
		if plain := ansiEscape.ReplaceAllString(m.View(), ""); !strings.Contains(plain, want) {
			t.Errorf("the %s screen does not draw its %s meter in the header", c.name, want)
		}
	}
}

// TestTheForcedPlatformAnswersComeFromOnePlace checks requirement 7 where it
// matters: the answers Termux and WSL force are applied when the setup screen is
// entered and when the Platform row changes, and they show as the rows' values.
func TestTheForcedPlatformAnswersComeFromOnePlace(t *testing.T) {
	t.Run("termux", func(t *testing.T) {
		m := Model{SystemInfo: &system.SystemInfo{OS: system.OSTermux, IsTermux: true}}
		m.enterSetup()
		if m.Choices.OS != "termux" {
			t.Fatalf("the setup screen recorded OS %q, want termux", m.Choices.OS)
		}
		if m.Choices.Terminal != "none" {
			t.Errorf("Termux forces no terminal emulator; the choices hold %q", m.Choices.Terminal)
		}
		if !m.Choices.InstallFont {
			t.Error("Termux takes the Nerd Font; the choices say it does not")
		}
		if value := m.setupRowValue(m.setupRows()[setupGuardRowIndex(t, m, "Terminal")]); value != "None" {
			t.Errorf("the Terminal row shows %q, want the forced answer's own row", value)
		}
		if value := m.setupRowValue(m.setupRows()[setupGuardRowIndex(t, m, "Font")]); value != "[x] install" {
			t.Errorf("the Font row shows %q, want the forced answer's own row", value)
		}
	})

	t.Run("wsl", func(t *testing.T) {
		m := Model{SystemInfo: &system.SystemInfo{OS: system.OSDebian, IsWSL: true}}
		m.enterSetup()
		if m.Choices.OS != "linux" {
			t.Fatalf("the setup screen recorded OS %q, want linux for a WSL host", m.Choices.OS)
		}
		if m.Choices.Terminal != "none" {
			t.Errorf("WSL forces no terminal emulator; the choices hold %q", m.Choices.Terminal)
		}
		if m.Choices.InstallFont {
			t.Error("WSL installs the font on the Windows host; the choices say it installs it here")
		}
	})

	t.Run("changing the Platform row applies them again", func(t *testing.T) {
		m := Model{SystemInfo: &system.SystemInfo{OS: system.OSLinux}}
		m.enterSetup()
		m.SetupCursor = setupGuardRowIndex(t, m, "Platform")
		// Walk to Termux: the forced answers follow the row.
		m.applyQuestionAnswer(ScreenOSSelect, "Termux")
		if m.Choices.Terminal != "none" || !m.Choices.InstallFont {
			t.Errorf("choosing Termux on the setup screen did not apply its forced answers: %+v", m.Choices)
		}
		// Back to Linux, which forces nothing: the answers stay where they are
		// rather than being reset behind the user, and the row is the user's to
		// cycle -- which is what keeps a platform change from being a trap.
		before := m.Choices
		m.applyQuestionAnswer(ScreenOSSelect, "Linux")
		if m.Choices.Terminal != before.Terminal || m.Choices.InstallFont != before.InstallFont {
			t.Errorf("a platform that forces nothing changed the answers from %+v to %+v", before, m.Choices)
		}
		m.SetupCursor = setupGuardRowIndex(t, m, "Terminal")
		m.cycleSetupRow(1)
		if m.Choices.Terminal == "none" {
			t.Error("the Terminal row cannot leave the forced answer once the platform stopped forcing it")
		}
	})
}

// TestTheSetupFooterNamesSpaceOnlyOnASwitchRow pins the footer to the handler: Space
// turns the focused switch off or on, so a switch row offers it and a cycle row does
// not, and the footer never names a key the row under the cursor would not answer.
func TestTheSetupFooterNamesSpaceOnlyOnASwitchRow(t *testing.T) {
	namesSpace := func(m Model) bool {
		for _, hint := range m.setupHints() {
			if hint == hintToggle {
				return true
			}
		}
		return false
	}

	m := Model{Screen: ScreenSetup, Width: 80, Height: 24}

	m.SetupCursor = setupRowIndexFor(t, m, "Font")
	if !namesSpace(m) {
		t.Errorf("the Font switch row does not offer Space: %v", m.setupHints())
	}

	m.SetupCursor = setupRowIndexFor(t, m, "Platform")
	if namesSpace(m) {
		t.Errorf("the Platform cycle row offers Space, and it has no switch: %v", m.setupHints())
	}

	// The handler answers Space on the row it offers it for.
	m.SetupCursor = setupRowIndexFor(t, m, "Font")
	row := m.setupRows()[m.SetupCursor]
	before := m.setupRowIsOn(row)
	next, _ := m.handleSetupKeys(" ")
	if next.(Model).setupRowIsOn(row) == before {
		t.Error("Space on the Font switch row did not flip it")
	}
}
