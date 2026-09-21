package tui

import (
	"errors"
	"strings"
	"testing"
)

// TestRunInstallStepsContinuesPastAFailure pins issue #21: a failed step is
// reported but does not abort the run. The steps after it still get their turn,
// cleanup runs, and the failure is returned so the exit status is non-zero.
func TestRunInstallStepsContinuesPastAFailure(t *testing.T) {
	steps := []InstallStep{
		{ID: "deps", Name: "Install deps"},
		{ID: "wslconfig", Name: "Configure WSL"},
		{ID: "setshell", Name: "Set shell as default"},
		{ID: "cleanup", Name: "Cleanup"},
	}

	var order []string
	workDir := t.TempDir()
	model := &Model{WorkDir: workDir, RepoDir: workDir + "/dotfiles"}

	execute := func(stepID string, m *Model) error {
		order = append(order, stepID)
		if stepID == "wslconfig" {
			return wrapStepError("wslconfig", "Configure WSL",
				"Failed to install /etc/wsl.conf",
				errors.New("permission denied"))
		}
		if stepID == "cleanup" {
			// Mirror the real cleanup: it clears the state it removed.
			m.WorkDir = ""
			m.RepoDir = ""
		}
		return nil
	}

	failures := runInstallSteps(steps, model, execute)

	wantOrder := []string{"deps", "wslconfig", "setshell", "cleanup"}
	if strings.Join(order, ",") != strings.Join(wantOrder, ",") {
		t.Fatalf("step order = %v, want %v", order, wantOrder)
	}
	t.Logf("steps run: %v", order)

	if model.WorkDir != "" {
		t.Errorf("cleanup did not run: WorkDir is still %q", model.WorkDir)
	}

	if len(failures) != 1 {
		t.Fatalf("failures = %v, want exactly one", failures)
	}
	if !strings.Contains(failures[0].Error(), "Configure WSL") {
		t.Errorf("the failure does not name the failed step: %v", failures[0])
	}
	if !strings.Contains(failures[0].Error(), "/etc/wsl.conf") {
		t.Errorf("the failure does not carry the step's cause: %v", failures[0])
	}
	t.Logf("reported failure: %v", failures[0])
}

// TestRunInstallStepsReportsNothingWhenAllStepsSucceed is the control: a clean
// run stays silent so the summary is the only place a failure appears.
func TestRunInstallStepsReportsNothingWhenAllStepsSucceed(t *testing.T) {
	steps := []InstallStep{
		{ID: "deps", Name: "Install deps"},
		{ID: "cleanup", Name: "Cleanup"},
	}

	var order []string
	failures := runInstallSteps(steps, &Model{}, func(stepID string, _ *Model) error {
		order = append(order, stepID)
		return nil
	})

	if len(failures) != 0 {
		t.Fatalf("a clean run must report no failures, got %v", failures)
	}
	if strings.Join(order, ",") != "deps,cleanup" {
		t.Fatalf("step order = %v, want every step", order)
	}
}
