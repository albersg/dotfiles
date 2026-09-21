package tui

import "testing"

// scheduledStepIDs returns every step ID that either scheduler can put on a run,
// across the configuration space the installer supports.
//
// It enumerates both schedulers on purpose. The interactive TUI builds its list
// with SetupInstallSteps and the non-interactive runner builds its list with
// buildStepsForChoices; a step either one schedules and the executor cannot run
// is a step that reports itself done having done nothing, which is the defect
// class this path has produced once per release. Deriving the set from the
// schedulers rather than restating it is what lets one test close the whole
// agreement instead of the single step that last broke.
func scheduledStepIDs(t *testing.T) map[string]bool {
	t.Helper()

	ids := map[string]bool{}
	for _, sysInfo := range allSystemInfos() {
		for _, choices := range allUserChoices() {
			nonInteractive := &Model{
				SystemInfo:      sysInfo,
				Choices:         choices,
				ExistingConfigs: []string{".config/nvim"},
			}
			for _, step := range buildStepsForChoices(nonInteractive) {
				ids[step.ID] = true
			}

			interactive := Model{
				SystemInfo:      sysInfo,
				Choices:         choices,
				ExistingConfigs: []string{".config/nvim"},
			}
			interactive.SetupInstallSteps()
			for _, step := range interactive.Steps {
				ids[step.ID] = true
			}
		}
	}

	return ids
}

// TestEveryScheduledStepHasAnExecutor is the scheduled-steps-versus-executor
// agreement: a step the non-interactive runner schedules, or the TUI schedules,
// must have an entry in the executor table. Before the executor table existed
// this agreement had no enforcement at all, so a scheduler could add a step the
// executor had never heard of and the only signal was a field report.
func TestEveryScheduledStepHasAnExecutor(t *testing.T) {
	for stepID := range scheduledStepIDs(t) {
		if _, ok := stepExecutors[stepID]; !ok {
			t.Errorf("step %q is scheduled by buildStepsForChoices or SetupInstallSteps, but the executor table has no entry for it",
				stepID)
		}
	}
}

// TestEveryExecutorEntryIsAScheduledStep is the other half of the agreement: an
// executor entry no scheduler can ever reach is dead code, or a step whose
// scheduling was lost. Keeping both directions honest is what turns "the next
// defect surfaces in the field" into "the next defect fails in CI".
func TestEveryExecutorEntryIsAScheduledStep(t *testing.T) {
	scheduled := scheduledStepIDs(t)
	for stepID := range stepExecutors {
		if !scheduled[stepID] {
			t.Errorf("the executor table has an entry for %q, but no configuration schedules that step",
				stepID)
		}
	}
}
