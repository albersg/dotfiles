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
		model func(t *testing.T) *Model
	}{
		{
			name:  "tmux",
			setID: "plugins-tmux",
			model: func(t *testing.T) *Model { return planFixtures()[0].fixtureModel() },
		},
		{
			name:  "herdr",
			setID: "plugins-herdr",
			model: func(t *testing.T) *Model {
				m := herdrPluginModel(t, true)
				return &m
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.model(t)

			step := pluginSetOnStep(t, planFor(m.planOptions()), "wm")
			if step.PluginSet == nil {
				t.Fatalf("the wm step carries no plugin set, so the interface cannot name what the step provisions")
			}
			if step.PluginSet.ID != tc.setID {
				t.Errorf("the wm step carries set %q, want %q", step.PluginSet.ID, tc.setID)
			}
			if step.PluginSet.Off {
				t.Error("a set is on unless it was turned off; this one arrived off")
			}

			// Turning it off leaves the step planned: the tool is still installed, and
			// only the set it would have provisioned is not.
			m.Choices.DisabledPluginSets = []string{tc.setID}
			step = pluginSetOnStep(t, planFor(m.planOptions()), "wm")
			if step.PluginSet == nil {
				t.Fatal("turning the set off removed it from the step instead of turning it off")
			}
			if !step.PluginSet.Off {
				t.Error("the plan still says the set is on after it was turned off")
			}
		})
	}

	// The registry is what the plan reads and the executor is what honours the
	// switch, so every set the registry calls provisioned has to be one a run can
	// turn off: this is the seam between the two.
	for _, set := range pluginSets {
		if !set.Provisioned {
			continue
		}
		if !pluginSetIsSwitchable(set.ID) {
			t.Errorf("the registry calls %q provisioned, but a run cannot turn it off", set.ID)
		}
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
