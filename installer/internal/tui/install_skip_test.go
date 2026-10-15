package tui

import (
	"strings"
	"testing"
)

// planWithDisabled is the plan the model's own options produce with a set of units
// turned off -- which is exactly what the review screen leaves in Model.DisabledUnits.
func planWithDisabled(m *Model, disabled ...string) []InstallStep {
	m.DisabledUnits = append([]string(nil), disabled...)
	return planFor(m.planOptions())
}

// plannedStep is the planned step for a unit, or a failure naming what was planned,
// so a unit disappearing from the plan fails here rather than proving something else.
func plannedStep(t *testing.T, steps []InstallStep, id string) InstallStep {
	t.Helper()
	for _, step := range steps {
		if step.ID == id {
			return step
		}
	}
	t.Fatalf("no step is planned for %q; the plan holds %v", id, planIDList(steps))
	return InstallStep{}
}

// chosenModel is the model the wizard leaves behind: a Linux host, a shell chosen,
// and Neovim asked for, so the units these guards name are all in the plan.
func chosenModel() *Model {
	m := NewModel()
	m.Choices = UserChoices{OS: "linux", Shell: "zsh", InstallNvim: true}
	return &m
}

// TestATurnedOffUnitIsPlannedAsSkipped pins S6's first half: the unit stays in the
// plan and is marked, rather than vanishing from a list that then reports nothing
// about it. A user who turns a unit off should be able to see that the run knows.
func TestATurnedOffUnitIsPlannedAsSkipped(t *testing.T) {
	steps := planWithDisabled(chosenModel(), "toolset", "nvim")

	for _, id := range []string{"toolset", "nvim"} {
		step := plannedStep(t, steps, id)
		if step.Status != StatusSkipped {
			t.Errorf("%q is planned as %v, want skipped", id, step.Status)
		}
		if step.Name == "" {
			t.Errorf("%q is skipped and nameless, so the run cannot say what it did not do", id)
		}
	}

	for _, step := range steps {
		if step.ID == "toolset" || step.ID == "nvim" {
			continue
		}
		if step.Status != StatusPending {
			t.Errorf("%q is %v, want pending: turning two units off marked a third", step.ID, step.Status)
		}
	}
}

// TestTheThreeUnitsTheRunNeedsAreNeverSkipped makes S3 a property of the plan rather
// than a promise the screen keeps. The screen cannot offer these as switches, so a
// selection naming them is not one the interface can produce; the plan refuses it all
// the same, because the three are the run's own conditions.
func TestTheThreeUnitsTheRunNeedsAreNeverSkipped(t *testing.T) {
	steps := planWithDisabled(chosenModel(), "clone", "cleanup", "setshell")

	for _, id := range []string{"clone", "cleanup", "setshell"} {
		if got := plannedStep(t, steps, id).Status; got == StatusSkipped {
			t.Errorf("%q is skipped, but the run needs it", id)
		}
	}
}

// TestNothingTurnedOffMarksNothingSkipped is S4 at the status level: the default
// selection is the empty set, and the empty set leaves every planned unit pending.
func TestNothingTurnedOffMarksNothingSkipped(t *testing.T) {
	m := chosenModel()

	for _, steps := range [][]InstallStep{planFor(m.planOptions()), planWithDisabled(m)} {
		for _, step := range steps {
			if step.Status == StatusSkipped {
				t.Errorf("%q is skipped with nothing turned off", step.ID)
			}
		}
	}
}

// TestATurnedOffUnitIsNotExecutedAndTheRunSaysSo pins S6's second half on the
// non-interactive route: the executor never sees the unit, the run reports no
// failure, and its own output says the unit was skipped.
func TestATurnedOffUnitIsNotExecutedAndTheRunSaysSo(t *testing.T) {
	steps := []InstallStep{
		{ID: "deps", Name: "Install the base dependencies"},
		{ID: "toolset", Name: "Install the command-line toolset", Status: StatusSkipped},
		{ID: "cleanup", Name: "Remove the temporary files"},
	}

	read := captureStepStdout(t)
	var ran []string
	model := NewModel()
	failures := runInstallSteps(steps, &model, func(stepID string, _ *Model) error {
		ran = append(ran, stepID)
		return nil
	})
	out := read()

	if len(failures) != 0 {
		t.Errorf("the run reported %v, want no failure: a unit the user turned off is not a failure", failures)
	}
	for _, id := range ran {
		if id == "toolset" {
			t.Fatalf("the executor ran %q, which the user turned off", id)
		}
	}
	if len(ran) != 2 {
		t.Errorf("the executor ran %v, want the two units that are on", ran)
	}
	if !strings.Contains(out, "skipped") {
		t.Errorf("the run's own output does not say the unit was skipped:\n%s", out)
	}
	if strings.Contains(out, "FAILED") {
		t.Errorf("the run's own output reports the skipped unit as a failure:\n%s", out)
	}
}

// TestTheInteractiveRunWalksPastATurnedOffUnit is the same rule on the route that
// drives the screens. The step is not executed, no credential is asked for on its
// behalf, and the run does not stop on it: it advances and hands back the command for
// whatever comes next. Its log names the unit, because that is this route's report.
func TestTheInteractiveRunWalksPastATurnedOffUnit(t *testing.T) {
	original := stepExecutor
	t.Cleanup(func() { stepExecutor = original })
	var ran []string
	stepExecutor = func(stepID string, _ *Model) error {
		ran = append(ran, stepID)
		return nil
	}

	m := NewModel()
	m.Steps = []InstallStep{
		{ID: "deps", Name: "Install the base dependencies"},
		{ID: "toolset", Name: "Install the command-line toolset", Status: StatusSkipped},
		{ID: "cleanup", Name: "Remove the temporary files"},
	}
	m.CurrentStep = 1

	cmd := m.runNextStep()

	if m.CurrentStep != 2 {
		t.Errorf("the run stopped on the unit that was turned off: it is on step %d", m.CurrentStep)
	}
	if len(ran) != 0 {
		t.Errorf("the run executed %v instead of walking past the unit that was turned off", ran)
	}
	if cmd == nil {
		t.Errorf("the run returned no command, so it neither reached the next unit nor finished")
	}

	if logged := strings.Join(m.LogLines, "\n"); !strings.Contains(logged, "Install the command-line toolset") {
		t.Errorf("the run's log does not name the unit it skipped:\n%s", logged)
	}
}

// TestTheRunFinishesWhenTheLastUnitWasTurnedOff is the end of that walk: a skipped
// unit at the end of the plan finishes the run rather than leaving it waiting on a
// step that will never execute.
func TestTheRunFinishesWhenTheLastUnitWasTurnedOff(t *testing.T) {
	m := NewModel()
	m.Steps = []InstallStep{
		{ID: "deps", Name: "Install the base dependencies"},
		{ID: "toolset", Name: "Install the command-line toolset", Status: StatusSkipped},
	}
	m.CurrentStep = 1

	done := m.runNextStep()
	if done == nil {
		t.Fatalf("the run returned no command for the step past the last one")
	}
	if _, ok := done().(installCompleteMsg); !ok {
		t.Errorf("the run past a skipped last unit returned %T, want the completion", done())
	}
}
