package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// --- the set is carried by the step of the tool it belongs to --------------

// TestThePlanCarriesThePluginSetOfItsTool pins S3 of
// odd/tasks/start-installation-flow.md: the set this installer provisions for a
// tool is visible beside that tool, and turning it off is not the same as
// turning the tool off. It runs over every set the registry calls provisioned, so
// a set added to the registry without a plan row fails here instead of arriving in
// silence.
func TestThePlanCarriesThePluginSetOfItsTool(t *testing.T) {
	cases := []struct {
		name  string
		setID string
		step  string
		model func(t *testing.T) *Model
	}{
		{
			name:  "tmux",
			setID: "plugins-tmux",
			step:  "wm",
			model: func(t *testing.T) *Model { return planFixtures()[0].fixtureModel() },
		},
		{
			name:  "herdr",
			setID: "plugins-herdr",
			step:  "wm",
			model: func(t *testing.T) *Model {
				m := herdrPluginModel(t, true)
				return &m
			},
		},
		{
			name:  "shell",
			setID: pluginSetIDShell,
			step:  "shell",
			model: func(t *testing.T) *Model {
				m := Model{SystemInfo: &system.SystemInfo{OS: system.OSLinux}, Choices: UserChoices{OS: "linux", Shell: "zsh"}}
				return &m
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.model(t)

			step := pluginSetOnStep(t, planFor(m.planOptions()), tc.step)
			if step.PluginSet == nil {
				t.Fatalf("the %s step carries no plugin set, so the interface cannot name what the step provisions", tc.step)
			}
			if step.PluginSet.ID != tc.setID {
				t.Errorf("the %s step carries set %q, want %q", tc.step, step.PluginSet.ID, tc.setID)
			}
			if step.PluginSet.Off {
				t.Error("a set is on unless it was turned off; this one arrived off")
			}

			// Turning it off leaves the step planned: the tool is still installed, and
			// only the set it would have provisioned is not.
			m.Choices.DisabledPluginSets = []string{tc.setID}
			step = pluginSetOnStep(t, planFor(m.planOptions()), tc.step)
			if step.PluginSet == nil {
				t.Fatal("turning the set off removed it from the step instead of turning it off")
			}
			if !step.PluginSet.Off {
				t.Error("the plan still says the set is on after it was turned off")
			}
		})
	}

	// A set the run can leave out is a set this installer installs itself, and a set
	// it cannot leave out says why: the first is a promise the executor has to keep,
	// the second is what stops a row from leaving the user to wonder.
	for _, set := range pluginSets {
		if set.Switchable && !set.Provisioned {
			t.Errorf("%q is offered as a switch, but this installer does not install it", set.ID)
		}
		if !set.Switchable && set.Because == "" {
			t.Errorf("%q cannot be left out and the registry says nothing about why, so no screen can explain it", set.ID)
		}
	}
}

// TestEveryRegisteredSetIsNamedOnThePlanOfARunThatInstallsItsTool is the S3 guard
// for the whole registry: a set nobody can see is the defect this front exists to
// remove, so every set has to appear on the step of the tool that installs it,
// switchable or not.
func TestEveryRegisteredSetIsNamedOnThePlanOfARunThatInstallsItsTool(t *testing.T) {
	for _, set := range pluginSets {
		choices := UserChoices{OS: "linux", Terminal: "kitty", Shell: "zsh", WindowMgr: "tmux", InstallNvim: true}
		switch set.Tool {
		case "zsh", "fish", "nushell":
			choices.Shell = set.Tool
		case "tmux", "herdr", "zellij":
			choices.WindowMgr = set.Tool
		case "nvim":
			choices.InstallNvim = true
		default:
			t.Fatalf("%q belongs to a tool this guard cannot choose: %s", set.ID, set.Tool)
		}

		m := &Model{SystemInfo: &system.SystemInfo{OS: system.OSLinux}, Choices: choices}
		step := pluginSetOnStep(t, planFor(m.planOptions()), set.Step)
		if step.PluginSet == nil {
			t.Errorf("a run that installs %s does not name its plugin set (%s) on the %s step", set.Tool, set.ID, set.Step)
			continue
		}
		if step.PluginSet.ID != set.ID {
			t.Errorf("the %s step carries %q, want %q", set.Step, step.PluginSet.ID, set.ID)
		}
		if step.PluginSet.Switchable != set.Switchable {
			t.Errorf("%q is carried as Switchable=%v, want %v", set.ID, step.PluginSet.Switchable, set.Switchable)
		}
		if !set.Switchable && step.PluginSet.Because == "" {
			t.Errorf("%q cannot be left out and the plan does not say why", set.ID)
		}
	}
}

// TestTheShellsSetIsASwitchAndTheConfigurationPaysForIt holds the decision the user
// took at L13: the shell's set is a switch like the others, and the price he accepted
// -- the `.zshrc` a run writes standing on its own -- is paid by the markers and the
// strip held in zsh_plugins_test.go, not by a second configuration this repository
// would have to maintain. This replaces
// TestTheShellsSetIsNotASwitchBecauseTheConfigurationSourcesIt, whose whole subject
// was the decision that changed; the guards for the switch itself are here and the
// guards for the configuration rule are beside the strip.
func TestTheShellsSetIsASwitchAndTheConfigurationPaysForIt(t *testing.T) {
	set, ok := pluginSetByID(pluginSetIDShell)
	if !ok {
		t.Fatal("the registry does not name the shell's plugin set")
	}
	if !set.Switchable {
		t.Fatal("the shell's set is not a switch, so the decision at L13 is not in the registry")
	}
	if set.Because != "" {
		t.Errorf("the shell's set can be left out, so it no longer carries the reason it cannot: %q", set.Because)
	}

	// The command line reaches the same switch the screen offers: `--skip` accepts the
	// name and the plan the executor reads carries it as left out.
	units, sets, err := ParseSkipSelection(pluginSetIDShell)
	if err != nil {
		t.Fatalf("--skip must accept the switch the screen offers: %v", err)
	}
	if len(units) != 0 || len(sets) != 1 || sets[0] != pluginSetIDShell {
		t.Errorf("--skip %s read as units=%v sets=%v, want just the set", pluginSetIDShell, units, sets)
	}

	m := Model{SystemInfo: &system.SystemInfo{OS: system.OSLinux}, Choices: UserChoices{OS: "linux", Shell: "zsh"}}
	m.Choices.DisabledPluginSets = sets
	step := pluginSetOnStep(t, planFor(m.planOptions()), "shell")
	if step.PluginSet == nil || !step.PluginSet.Off {
		t.Errorf("the plan of a run that skipped the set does not carry it as left out: %+v", step.PluginSet)
	}
}

// TestTheHerdrSetNamesThePluginsItProvisions holds the other half of visibility:
// the name the interface shows is read from the set's own declared table, so
// adding, removing or renaming a declared plugin moves the name with it instead
// of leaving a second copy of the list to drift.
func TestTheHerdrSetNamesThePluginsItProvisions(t *testing.T) {
	set, ok := pluginSetByID("plugins-herdr")
	if !ok {
		t.Fatal("the registry has no set for Herdr")
	}
	if !set.Provisioned {
		t.Error("this installer installs the Herdr set itself; a registry that says otherwise would have the interface explain the set away instead of offering it")
	}
	label := set.Label()
	for _, plugin := range herdrPluginSet {
		if !strings.Contains(label, plugin.Name) {
			t.Errorf("the set's label %q does not name the declared plugin %q", label, plugin.Name)
		}
	}
}

// --- turned off means nothing runs -----------------------------------------

// TestTurningOffHerdrsSetRunsNothing pins the promise the switch makes: with the
// set off, the tool's step runs no plugin command at all, asks the machine
// nothing, and does not fail -- a set is not a choice the user can get wrong.
func TestTurningOffHerdrsSetRunsNothing(t *testing.T) {
	m := herdrPluginModel(t, true)
	m.Choices.DisabledPluginSets = []string{"plugins-herdr"}
	calls := stubHerdrPluginCLI(t, herdrPluginStub{
		version: "herdr 0.9.5",
		list:    herdrListWith(declaredHerdrPluginIDs()...),
	})

	if err := installHerdrPlugins(&m, "wm"); err != nil {
		t.Fatalf("a set the user turned off must not fail the step: %v", err)
	}
	if len(calls.commands) != 0 || calls.listCalls != 0 {
		t.Errorf("a set that was turned off still ran commands: %v listCalls=%d", calls.commands, calls.listCalls)
	}
}

// TestTheCommandLineReachesTheSameSwitch pins the parity S6 asks for: a run
// without a screen turns the set off with the flag it already has, and widening
// that flag did not make it accept a name nothing recognises, nor a unit the run
// needs.
func TestTheCommandLineReachesTheSameSwitch(t *testing.T) {
	units, sets, err := ParseSkipSelection("officecli,plugins-herdr")
	if err != nil {
		t.Fatalf("a unit and a set together must be read without error: %v", err)
	}
	if len(units) != 1 || units[0] != "officecli" {
		t.Errorf("units = %v, want just officecli", units)
	}
	if len(sets) != 1 || sets[0] != "plugins-herdr" {
		t.Errorf("sets = %v, want just plugins-herdr", sets)
	}

	if _, _, err := ParseSkipSelection("plugins-clone"); err == nil {
		t.Error("a name nothing recognises was accepted")
	}
	if _, _, err := ParseSkipSelection("clone"); err == nil {
		t.Error("a unit the run needs was accepted as skippable")
	}
	if units, sets, err := ParseSkipSelection(""); err != nil || units != nil || sets != nil {
		t.Errorf("an empty flag must mean no selection at all, got units=%v sets=%v err=%v", units, sets, err)
	}
}

// --- a set a configuration names is stripped from it -------------------------

// TestThePluginMarkersBoundTheTextTheInstallerTreatsAsTheSet is the guard the Tmux
// patch cannot do without. A run that leaves the set out rewrites the installed
// tmux.conf between markers, so the markers decide what the user loses: a block
// that is too wide takes a key binding with it, and a plugin declaration left
// outside the block survives the strip and names something that is not there.
func TestThePluginMarkersBoundTheTextTheInstallerTreatsAsTheSet(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "dotfiles-tmux", "tmux.conf"))
	if err != nil {
		t.Fatalf("the Tmux configuration this installer ships could not be read: %v", err)
	}
	content := string(data)

	for _, marker := range []string{tmuxPluginBlockStart, tmuxPluginBlockEnd, tmuxTPMBlockStart, tmuxTPMBlockEnd} {
		if got := strings.Count(content, marker); got != 1 {
			t.Errorf("the Tmux configuration carries %q %d times, want exactly once: the installer strips what is between the markers, and a duplicate or a missing one would leave the machine with text nobody owns", marker, got)
		}
	}

	pluginStart := strings.Index(content, tmuxPluginBlockStart)
	pluginEnd := strings.Index(content, tmuxPluginBlockEnd)
	tpmStart := strings.Index(content, tmuxTPMBlockStart)
	tpmEnd := strings.Index(content, tmuxTPMBlockEnd)
	if pluginStart < 0 || pluginEnd < pluginStart || tpmStart < pluginEnd || tpmEnd < tpmStart {
		t.Fatalf("the markers are not in the order the installer strips them: plugins %d..%d, tpm %d..%d", pluginStart, pluginEnd, tpmStart, tpmEnd)
	}

	block := content[pluginStart+len(tmuxPluginBlockStart) : pluginEnd]
	tpmBlock := content[tpmStart+len(tmuxTPMBlockStart) : tpmEnd]
	outside := content[:pluginStart] + content[pluginEnd:]

	if !strings.Contains(block, "set -g @plugin ") {
		t.Error("the marked block declares no plugin, so the markers bound the wrong text")
	}
	for _, line := range strings.Split(block, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "bind-key ") {
			t.Errorf("the marked block swallows a key binding, so turning the plugin set off would silently remove a keymap: %q", trimmed)
		}
	}
	for _, line := range strings.Split(outside, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "set -g @plugin ") {
			t.Errorf("a plugin is declared outside the marked block, so a run that leaves the set out would leave the declaration behind: %q", trimmed)
		}
	}

	if !strings.Contains(tpmBlock, "run '~/.tmux/plugins/tpm/tpm'") {
		t.Error("the TPM block does not hold the line that runs TPM, so stripping it would leave the machine running a checkout that is not there")
	}
	for _, line := range strings.Split(content[:tpmStart]+content[tpmEnd:], "\n") {
		if strings.Contains(strings.TrimSpace(line), "~/.tmux/plugins/tpm/tpm") {
			t.Errorf("a line outside the TPM block runs TPM, so a run that leaves the set out would leave it behind: %q", strings.TrimSpace(line))
		}
	}
}

// TestTurningOffTmuxsSetLeavesNoTPMAndNoPluginInTheServer pins both halves of the
// switch where the user feels it: with the set off nothing provisions the plugins
// and the installed configuration declares none, and with the set on the
// configuration is the one this repository ships.
func TestTurningOffTmuxsSetLeavesNoTPMAndNoPluginInTheServer(t *testing.T) {
	for _, tc := range []struct {
		name        string
		disabled    []string
		wantPlugins bool
	}{
		{name: "set on", wantPlugins: true},
		{name: "set off", disabled: []string{"plugins-tmux"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			readLog := verboseTerminalStep(t)

			home := t.TempDir()
			t.Setenv("HOME", home)

			// tmux counts as installed, so the step's job under test is the plugin set
			// rather than a package install.
			binDir := t.TempDir()
			writeStubCommand(t, binDir, "tmux", "#!/bin/sh\nexit 0\n")
			t.Setenv("PATH", strings.Join([]string{binDir, "/usr/bin", "/bin"}, string(os.PathListSeparator)))

			if tc.wantPlugins {
				// A machine that already has TPM: the clone would otherwise reach the
				// network, and the install run is stubbed so nothing is downloaded.
				tpmBin := filepath.Join(home, ".tmux", "plugins", "tpm", "bin")
				if err := os.MkdirAll(tpmBin, 0o755); err != nil {
					t.Fatal(err)
				}
				writeStubCommand(t, tpmBin, "install_plugins", "#!/bin/sh\nexit 0\n")
			}

			m := NewModel()
			m.SystemInfo = &system.SystemInfo{OS: system.OSMac, HasBrew: true}
			m.Choices = UserChoices{OS: "mac", Shell: "zsh", WindowMgr: "tmux", DisabledPluginSets: tc.disabled}
			m.RepoDir = repoRoot(t)

			if err := stepInstallWM(&m); err != nil {
				t.Fatalf("the Tmux step must succeed either way: %v", err)
			}

			_, statErr := os.Stat(filepath.Join(home, ".tmux", "plugins", "tpm"))
			if tc.wantPlugins != (statErr == nil) {
				t.Errorf("the TPM checkout exists = %v, want %v: a set that is turned off must not provision one, and a set that is on must", statErr == nil, tc.wantPlugins)
			}

			data, err := os.ReadFile(filepath.Join(home, ".tmux.conf"))
			if err != nil {
				t.Fatalf("the Tmux configuration was not installed: %v", err)
			}
			config := string(data)
			if got := strings.Contains(config, "set -g @plugin "); got != tc.wantPlugins {
				t.Errorf("the installed configuration declares plugins = %v, want %v:\n%s", got, tc.wantPlugins, config)
			}
			if got := strings.Contains(config, "~/.tmux/plugins/tpm/tpm"); got != tc.wantPlugins {
				t.Errorf("the installed configuration runs TPM = %v, want %v:\n%s", got, tc.wantPlugins, config)
			}

			if !tc.wantPlugins {
				out := readLog()
				if !strings.Contains(out, "plugin set is turned off") {
					t.Errorf("a set the user turned off was not reported: %q", out)
				}
				if !strings.Contains(out, "✓ Tmux configured") {
					t.Errorf("a run whose configuration is complete withheld its success line: %q", out)
				}
			}
		})
	}
}

// pluginSetOnStep returns the step with this ID, failing the test when the plan
// has no such step.
func pluginSetOnStep(t *testing.T, steps []InstallStep, id string) InstallStep {
	t.Helper()
	for _, step := range steps {
		if step.ID == id {
			return step
		}
	}
	t.Fatalf("the plan has no %q step: %s", id, planIDList(steps))
	return InstallStep{}
}
