package tui

import (
	"reflect"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// planFixture is one configuration the plan is pinned against, with the plan it
// produced before the two planners became one.
//
// The identifiers are a single comma-joined string rather than a slice so a
// failure shows the whole sequence on one line and the difference is readable as
// what it is: a unit that moved, appeared or went missing.
type planFixture struct {
	name    string
	choices UserChoices
	info    *system.SystemInfo
	configs []string
	wantIDs string
}

// planFixtures are the configurations the plan's conditions can differ on, and
// the plans measured from the tree as it stood before T2 changed the builder. The
// four rows that matter most are macOS with the tools present (where the two
// planners disagreed about the dependency step), macOS without them, Termux (where
// they disagreed about Homebrew) and a machine with nothing to back up (where they
// disagreed about the backup step).
func planFixtures() []planFixture {
	all := func(os string) UserChoices {
		return UserChoices{
			OS:           os,
			CreateBackup: true,
			Terminal:     "kitty",
			InstallFont:  true,
			Shell:        "zsh",
			WindowMgr:    "tmux",
			InstallNvim:  true,
		}
	}
	twoConfigs := []string{".zshrc", ".config/nvim"}

	return []planFixture{
		{
			name:    "linux-debian, no brew, no xcode, every choice taken",
			choices: all("linux"),
			info:    &system.SystemInfo{OS: system.OSDebian},
			configs: twoConfigs,
			wantIDs: "backup,deps,clone,homebrew,terminal,font,shell,wm,nvim,toolset,agentskills,officecli,theme,setshell,cleanup",
		},
		{
			name:    "macos with xcode, no brew",
			choices: all("mac"),
			info:    &system.SystemInfo{OS: system.OSMac, HasXcode: true},
			configs: twoConfigs,
			wantIDs: "backup,clone,homebrew,terminal,font,shell,wm,nvim,toolset,agentskills,officecli,theme,setshell,cleanup",
		},
		{
			name:    "macos without xcode, no brew",
			choices: all("mac"),
			info:    &system.SystemInfo{OS: system.OSMac},
			configs: twoConfigs,
			wantIDs: "backup,xcode,clone,homebrew,terminal,font,shell,wm,nvim,toolset,agentskills,officecli,theme,setshell,cleanup",
		},
		{
			name:    "termux host, as the non-interactive runner sees it",
			choices: all("linux"),
			info:    &system.SystemInfo{OS: system.OSLinux, IsTermux: true},
			configs: twoConfigs,
			wantIDs: "backup,deps,clone,terminal,font,shell,wm,nvim,toolset,agentskills,officecli,theme,setshell,cleanup",
		},
		{
			name:    "fedora with brew already installed",
			choices: all("linux"),
			info:    &system.SystemInfo{OS: system.OSFedora, HasBrew: true},
			configs: twoConfigs,
			wantIDs: "backup,deps,clone,terminal,font,shell,wm,nvim,toolset,agentskills,officecli,theme,setshell,cleanup",
		},
		{
			name:    "linux with nothing to back up",
			choices: all("linux"),
			info:    &system.SystemInfo{OS: system.OSDebian},
			configs: nil,
			wantIDs: "deps,clone,homebrew,terminal,font,shell,wm,nvim,toolset,agentskills,officecli,theme,setshell,cleanup",
		},
	}
}

// fixtureModel is the model a fixture describes, in the shape both callers build:
// the non-interactive runner and the wizard fill the same fields.
func (f planFixture) fixtureModel() *Model {
	return &Model{SystemInfo: f.info, Choices: f.choices, ExistingConfigs: f.configs}
}

// planIDList is a plan as the identifiers a reader can compare, in order.
func planIDList(steps []InstallStep) string {
	ids := make([]string, 0, len(steps))
	for _, step := range steps {
		ids = append(ids, step.ID)
	}
	return strings.Join(ids, ",")
}

// planNameList is a plan as the names a reader sees, in order. The identifiers pin
// which units the plan holds; the names pin what it calls them, which is what the
// user reads in the plan and in the run's step list.
func planNameList(steps []InstallStep) []string {
	names := make([]string, 0, len(steps))
	for _, step := range steps {
		names = append(names, step.ID+":"+step.Name)
	}
	return names
}

// TestThePlanNamesWhatItInstalls pins the other half of the plan a reader meets.
// The identifiers of TestThePlanIsStillThePlanItWas say which units run; these say
// what each one is called when it does.
//
// It exists because a name is not decoration. The E2E suite asserts two of them
// (installer/e2e/e2e_test.sh, in the terminal axis and the font option) and it was
// the only place that did: when the two planners became one, the non-interactive
// route began using the plan's names, and the E2E -- run on four distributions,
// minutes into CI -- was the first thing to notice. These pairs were measured from
// the fixtures, so a name that changes now fails here, in seconds.
func TestThePlanNamesWhatItInstalls(t *testing.T) {
	fixture := planFixtures()[0]
	got := planNameList(planFor(fixture.fixtureModel().planOptions()))
	want := []string{
		"backup:Backup Existing Configs",
		"deps:Install Dependencies",
		"clone:Clone Repository",
		"homebrew:Install Homebrew",
		"terminal:Install kitty",
		"font:Install Meslo Nerd Font",
		"shell:Install zsh",
		"wm:Install tmux",
		"nvim:Install Neovim",
		"toolset:Install Toolset",
		"agentskills:Install Pi Agent Skills",
		"officecli:Install OfficeCLI",
		"theme:Restore the Applied Theme",
		"setshell:Set Default Shell",
		"cleanup:Cleanup",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("the plan's names changed:\n got:\n      %s\nwant:\n      %s",
			strings.Join(got, "\n      "), strings.Join(want, "\n      "))
	}

	// The two names the E2E reads by shape rather than by value, held to the same
	// rule here: a terminal step carries the terminal the choices named, and a font
	// step carries the part every family shares. The family itself is the
	// platform's, which is why neither the E2E nor this guard writes a label down.
	for _, f := range planFixtures() {
		for _, step := range planFor(f.fixtureModel().planOptions()) {
			switch step.ID {
			case "terminal":
				if want := "Install " + f.choices.Terminal; step.Name != want {
					t.Errorf("%s: the terminal step is named %q, want %q", f.name, step.Name, want)
				}
			case "font":
				if !strings.Contains(step.Name, "Nerd Font") {
					t.Errorf("%s: the font step is named %q, which does not carry \"Nerd Font\"", f.name, step.Name)
				}
			}
		}
	}
}

// TestThePlanIsStillThePlanItWas pins S4: with the selection untouched, the plan is
// today's plan, from the same facts. Each fixture's identifiers were measured from
// the tree before the builder changed, so this fails if a unit appears, disappears
// or moves -- which is the whole surface the two planners had drifted across.
func TestThePlanIsStillThePlanItWas(t *testing.T) {
	for _, fixture := range planFixtures() {
		got := planIDList(planFor(fixture.fixtureModel().planOptions()))
		if got != fixture.wantIDs {
			t.Errorf("%s:\n  planned %s\n  want    %s", fixture.name, got, fixture.wantIDs)
		}
	}
}

// TestTheNonInteractiveRunnerPlansWhatTheInterfacePlans pins S8, and it is an
// anchor rather than a proof: today one function serves both callers, so it cannot
// fail while that is true. It is here because the shape it forbids -- a second set
// of rules inside the non-interactive path -- is what the tree carried before, and
// the way that arrived was not by anyone deciding to duplicate the plan. It
// compares whole steps, not identifiers: a name that only one path gives a unit is
// the same defect one layer down.
func TestTheNonInteractiveRunnerPlansWhatTheInterfacePlans(t *testing.T) {
	for _, fixture := range planFixtures() {
		m := fixture.fixtureModel()
		wanted := planFor(m.planOptions())
		got := buildStepsForChoices(m)
		if !reflect.DeepEqual(got, wanted) {
			t.Errorf("%s: the non-interactive plan is not the interface's:\n  non-interactive %s\n  interface       %s",
				fixture.name, planIDList(got), planIDList(wanted))
			for i := range wanted {
				if i < len(got) && got[i] != wanted[i] {
					t.Errorf("  step %d: %+v, want %+v", i, got[i], wanted[i])
				}
			}
		}
	}
}

// TestTheWholePlanIsWhatItWas pins the plan field by field for one configuration:
// the identifiers say which units run, and these say what each one is called and
// says about itself, which is what the run prints and what the main menu's panel
// shows. Measured from the tree as it stood before the builder changed.
//
// The multiplexer's step carries the plugin set it provisions
// (install_plugin_sets.go): the plan names what the step would install and whether
// the run turned it off, so the guard sees it here instead of the step being blind
// to its own work. This is the one field the plan gained for it.
func TestTheWholePlanIsWhatItWas(t *testing.T) {
	fixture := planFixtures()[0]
	m := fixture.fixtureModel()

	got := planFor(m.planOptions())
	want := []InstallStep{
		{ID: "backup", Name: "Backup Existing Configs", Description: "Saves a copy of your current configuration first.", Status: StatusPending},
		{ID: "deps", Name: "Install Dependencies", Description: "Installs base packages with your distribution's package manager.", Status: StatusPending, Interactive: true},
		{ID: "clone", Name: "Clone Repository", Description: "Downloads your dotfiles repository.", Status: StatusPending},
		{ID: "homebrew", Name: "Install Homebrew", Description: "Installs Homebrew, the package manager.", Status: StatusPending, Interactive: true},
		{ID: "terminal", Name: "Install kitty", Description: "Installs your terminal emulator.", Status: StatusPending, Interactive: true},
		{ID: "font", Name: "Install Meslo Nerd Font", Description: "Installs the Meslo Nerd Font for icons.", Status: StatusPending},
		{ID: "shell", Name: "Install zsh", Description: "Installs your shell and its plugins.", Status: StatusPending},
		{ID: "wm", Name: "Install tmux", Description: "Installs your terminal multiplexer.", Status: StatusPending, PluginSet: &PluginSetState{ID: "plugins-tmux", Name: "the plugins its configuration declares"}},
		{ID: "nvim", Name: "Install Neovim", Description: "Installs Neovim with your configuration.", Status: StatusPending},
		{ID: "toolset", Name: "Install Toolset", Description: "Installs the command-line tools listed in your Brewfile.", Status: StatusPending},
		{ID: "agentskills", Name: "Install Pi Agent Skills", Description: "Installs the pinned security-audit, archify and officecli skills.", Status: StatusPending},
		{ID: "officecli", Name: "Install OfficeCLI", Description: "Installs the pinned, checksum-verified OfficeCLI binary.", Status: StatusPending},
		{ID: "theme", Name: "Restore the Applied Theme", Description: "Writes the theme this machine has applied back into the files the install replaced.", Status: StatusPending},
		{ID: "setshell", Name: "Set Default Shell", Description: "Sets your shell as the default.", Status: StatusPending, Interactive: true},
		{ID: "cleanup", Name: "Cleanup", Description: "Removes the temporary files it created.", Status: StatusPending},
	}

	if len(got) != len(want) {
		t.Fatalf("the plan has %d steps, want %d:\n  %s", len(got), len(want), planIDList(got))
	}
	for i := range want {
		// Compared by value rather than with ==: the plan carries a pointer to the set
		// its step provisions, and == would compare the pointer, so an expectation that
		// says the same thing would still fail for having been built separately.
		if !reflect.DeepEqual(got[i], want[i]) {
			t.Errorf("step %d (%s):\n  got  %+v\n  want %+v", i, want[i].ID, got[i], want[i])
		}
	}
}

// TestEveryPlanFollowsTheCatalogueOrder is what makes install_units.go the
// authority on the order a run schedules units in rather than a description of it.
// Every plan has to be a subsequence of the catalogue: deps and xcode share a
// position there because a run plans one or neither, and everything else is in the
// same order, so a unit that moved in either place fails here. Taking the plans of
// every configuration together also closes the other direction -- a catalogued unit
// no configuration ever plans would leave the union short.
func TestEveryPlanFollowsTheCatalogueOrder(t *testing.T) {
	catalogue := make([]string, 0, len(installUnits))
	for _, unit := range installUnits {
		catalogue = append(catalogue, unit.ID)
	}

	planned := map[string]bool{}

	for _, sysInfo := range allSystemInfos() {
		for _, choices := range allUserChoices() {
			m := &Model{SystemInfo: sysInfo, Choices: choices, ExistingConfigs: []string{".zshrc"}}
			ids := planIDList(planFor(m.planOptions()))
			for _, id := range strings.Split(ids, ",") {
				planned[id] = true
			}
			if !isSubsequence(strings.Split(ids, ","), catalogue) {
				t.Errorf("the plan for %s is not in the catalogue's order:\n  plan      %s\n  catalogue %s",
					ids, ids, strings.Join(catalogue, ","))
			}
		}
	}

	for _, id := range catalogue {
		if !planned[id] {
			t.Errorf("the catalogue names %q, but no configuration plans it", id)
		}
	}
}

// isSubsequence reports whether every element of want appears in got, in order.
func isSubsequence(want, got []string) bool {
	i := 0
	for _, g := range got {
		if i < len(want) && want[i] == g {
			i++
		}
	}
	return i == len(want)
}
