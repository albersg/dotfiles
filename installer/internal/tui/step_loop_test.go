package tui

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// driveInstallStepLoop drives the model's installation step loop the way the
// bubbletea runtime does: Update returns a command, the command returns a
// message, and that message goes back into Update. It stops when a command
// produces no message or when the loop stops returning one, and it is bounded
// so a regression cannot hang the suite.
func driveInstallStepLoop(t *testing.T, m Model, start tea.Msg) Model {
	t.Helper()

	result, cmd := m.Update(start)
	for i := 0; i < 64 && cmd != nil; i++ {
		m = result.(Model)
		msg := cmd()
		if msg == nil {
			break
		}
		result, cmd = m.Update(msg)
	}
	return result.(Model)
}

// TestTUIStepLoopCarriesWhatAStepRecordsToTheNextStep closes the coverage gap
// behind issue #26: no test had ever driven the TUI's own step loop, so a loop
// that ran every step against a throwaway copy of the model went unnoticed. The
// clone step's recorded checkout is the state every later step reads and the
// one cleanup removes, so this drives a clone, a step that needs the checkout,
// and the real cleanup through Update.
func TestTUIStepLoopCarriesWhatAStepRecordsToTheNextStep(t *testing.T) {
	workDir := t.TempDir()
	checkout := filepath.Join(workDir, "dotfiles")
	if err := os.MkdirAll(filepath.Join(checkout, ".git"), 0o755); err != nil {
		t.Fatalf("preparing the fake checkout: %v", err)
	}

	original := stepExecutor
	t.Cleanup(func() { stepExecutor = original })

	var seenByTheShellStep string
	stepExecutor = func(stepID string, m *Model) error {
		switch stepID {
		case "clone":
			// Mirror stepCloneRepo: record the checkout for later steps.
			m.WorkDir = workDir
			m.RepoDir = checkout
			return nil
		case "shell":
			// Mirror every step that needs the checkout: read it back off the
			// model the loop carried forward.
			recorded, err := m.repoDir()
			if err != nil {
				return err
			}
			seenByTheShellStep = recorded
			return nil
		default:
			return executeStep(stepID, m)
		}
	}

	m := NewModel()
	m.Screen = ScreenInstalling
	m.CurrentStep = 0
	m.Steps = []InstallStep{
		{ID: "clone", Name: "Clone Repository"},
		{ID: "shell", Name: "Install zsh"},
		{ID: "cleanup", Name: "Cleanup"},
	}

	m = driveInstallStepLoop(t, m, installStartMsg{})

	if m.Screen == ScreenError {
		t.Fatalf("the step after clone failed: %s", m.ErrorMsg)
	}
	if seenByTheShellStep != checkout {
		t.Fatalf("the shell step saw RepoDir %q, want the checkout the clone step recorded (%q)",
			seenByTheShellStep, checkout)
	}
	if m.WorkDir != "" || m.RepoDir != "" {
		t.Errorf("cleanup did not clear the recorded checkout: WorkDir=%q RepoDir=%q", m.WorkDir, m.RepoDir)
	}
	if _, err := os.Stat(workDir); !os.IsNotExist(err) {
		t.Errorf("cleanup left the recorded checkout behind: %v", err)
	}
}

// TestRunInstallStepsStillCleansUpAfterAFailure ties the failure contract of the
// non-interactive runner to the state the clone step records: a failed step does
// not cost cleanup its turn, and cleanup still finds the checkout because the
// runner shares one model with every step.
func TestRunInstallStepsStillCleansUpAfterAFailure(t *testing.T) {
	workDir := t.TempDir()
	checkout := filepath.Join(workDir, "dotfiles")
	if err := os.MkdirAll(checkout, 0o755); err != nil {
		t.Fatalf("preparing the fake checkout: %v", err)
	}

	model := &Model{}
	steps := []InstallStep{
		{ID: "clone", Name: "Clone dotfiles repository"},
		{ID: "shell", Name: "Install zsh shell"},
		{ID: "cleanup", Name: "Cleanup"},
	}

	execute := func(stepID string, m *Model) error {
		switch stepID {
		case "clone":
			m.WorkDir = workDir
			m.RepoDir = checkout
			return nil
		case "shell":
			if _, err := m.repoDir(); err != nil {
				return err
			}
			return errors.New("zsh installation failed")
		default:
			return executeStep(stepID, m)
		}
	}

	failures := runInstallSteps(steps, model, execute)

	if len(failures) != 1 {
		t.Fatalf("failures = %v, want exactly the one failed step", failures)
	}
	if model.WorkDir != "" || model.RepoDir != "" {
		t.Errorf("cleanup did not run after the failure: WorkDir=%q RepoDir=%q", model.WorkDir, model.RepoDir)
	}
	if _, err := os.Stat(workDir); !os.IsNotExist(err) {
		t.Errorf("the failed run left the recorded checkout behind: %v", err)
	}
}
