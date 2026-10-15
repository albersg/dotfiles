package tui

import (
	"strings"
	"testing"
)

// --- the set is carried by the step of the tool it belongs to --------------

// TestThePlanCarriesThePluginSetOfItsTool pins S3 of
// odd/tasks/start-installation-flow.md: the set this installer provisions for a
// tool is visible beside that tool, and turning it off is not the same as
// turning the tool off.
func TestThePlanCarriesThePluginSetOfItsTool(t *testing.T) {
	m := herdrPluginModel(t, true)

	step := pluginSetOnStep(t, planFor(m.planOptions()), "wm")
	if step.PluginSet == nil {
		t.Fatal("the wm step carries no plugin set, so the interface cannot name what the step provisions")
	}
	if step.PluginSet.ID != "plugins-herdr" {
		t.Errorf("the wm step carries set %q, want %q", step.PluginSet.ID, "plugins-herdr")
	}
	if step.PluginSet.Off {
		t.Error("a set is on unless it was turned off; this one arrived off")
	}

	// Turning it off leaves the step planned: the tool is still installed, and
	// only the set it would have provisioned is not.
	m.Choices.DisabledPluginSets = []string{"plugins-herdr"}
	step = pluginSetOnStep(t, planFor(m.planOptions()), "wm")
	if step.PluginSet == nil {
		t.Fatal("turning the set off removed it from the step instead of turning it off")
	}
	if !step.PluginSet.Off {
		t.Error("the plan still says the set is on after it was turned off")
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
