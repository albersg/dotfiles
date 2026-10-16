package tui

import (
	"errors"
	"fmt"
	"runtime"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// RunNonInteractive executes the installation without TUI
// RunNonInteractive installs the plan the choices produce, without the interface.
//
// disabledUnits names the units the caller was told to leave out, in catalogue order.
// It is the same set the review screen leaves in Model.DisabledUnits on the other
// route, and it is handed straight to the one builder, so one selection produces one
// plan whichever route asked for it -- which is what S8 asks for.
func RunNonInteractive(choices UserChoices, disabledUnits ...string) error {
	// Enable non-interactive mode for logging
	SetNonInteractiveMode(true)

	// Detect system info
	sysInfo := system.Detect()

	// Determine OS choice based on system
	osChoice := "linux"
	if runtime.GOOS == "darwin" {
		osChoice = "mac"
	}
	choices.OS = osChoice

	// Create a minimal model for the installation functions
	model := &Model{
		SystemInfo:    sysInfo,
		Choices:       choices,
		LogLines:      []string{},
		DisabledUnits: disabledUnits,
	}

	// Detect existing configs for backup functionality
	if choices.CreateBackup {
		model.ExistingConfigs = system.DetectExistingConfigs()
	}

	// Define steps to run based on choices
	steps := buildStepsForChoices(model)

	fmt.Printf("📋 Running %d installation steps...\n\n", len(steps))

	failures := runInstallSteps(steps, model, executeStep)

	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	switch {
	case len(failures) > 0:
		// The run finished what it could and cleaned up after itself, but it is
		// not a success: a step that failed is reported and the exit status is
		// non-zero so a script or CI run cannot mistake it for one.
		fmt.Printf("⚠️  Installation finished with %d failed step(s):\n", len(failures))
		for _, failure := range failures {
			fmt.Printf("   - %v\n", failure)
		}
	case dryRun():
		fmt.Println("🧪 Dry run complete: nothing was installed.")
	default:
		fmt.Println("✅ Installation complete!")
	}
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	return finishNonInteractiveRun(model, failures)
}

// finishNonInteractiveRun is the epilogue of a scripted run: it records the run
// the same way the interface does, so a machine installed from a script has the
// record that the revert and the content check read. It used to be printing
// alone, which is how a scripted install came to leave no record of itself.
func finishNonInteractiveRun(model *Model, failures []error) error {
	status := runStatusComplete
	failedSteps := []string{}
	if len(failures) > 0 {
		status = runStatusFailed
		for _, failure := range failures {
			failedSteps = append(failedSteps, failure.Error())
		}
	}
	model.recordRun(status, failedSteps)

	if len(failures) > 0 {
		return errors.Join(failures...)
	}
	return nil
}

// runInstallSteps executes the planned steps in order. A failed step is recorded
// and reported, but it does not stop the run: every later step that can still do
// its work is attempted, the default-shell step is never skipped because an
// earlier root-only step failed, and cleanup always gets its turn so a temporary
// checkout is not left behind. One error is returned per failed step, in order,
// and the caller reports them together when the run ends.
func runInstallSteps(steps []InstallStep, model *Model, execute func(stepID string, m *Model) error) []error {
	var failures []error

	for i, step := range steps {
		fmt.Printf("[%d/%d] %s...\n", i+1, len(steps), step.Name)

		// A unit the user turned off is planned as skipped rather than dropped, so it is
		// walked past here instead of executed -- and it is not a failure, because
		// nothing went wrong. The line is this route's report, distinct from the dry
		// run's, which says why it skipped something else.
		if stepIsSkipped(step) {
			fmt.Printf("    • skipped: you turned it off\n")
			continue
		}

		err := execute(step.ID, model)
		if err != nil {
			fmt.Printf("    ❌ FAILED: %v\n", err)
			failures = append(failures, fmt.Errorf("step '%s' failed: %w", step.Name, err))
			continue
		}
		if dryRun() {
			fmt.Printf("    • skipped (dry run)\n")
			continue
		}
		fmt.Printf("    ✓ Done\n")
	}

	return failures
}

// buildStepsForChoices returns the plan the non-interactive run executes.
//
// It is a name for planFor, not a second planner. Two planners for one run had
// already drifted apart, and it was measured on this tree rather than suspected: a
// macOS host was planned with the dependency step -- a Linux step -- by this path
// and with the Apple developer tools by the interface's, a Termux host was planned
// with Homebrew here and without it there, and a machine with nothing to back up
// was planned with a backup step here and without one there.
//
// The rules live in planFor and the units live in install_units.go. This function
// exists so the runner keeps a name its call site and its tests read well with, and
// TestTheNonInteractiveRunnerPlansWhatTheInterfacePlans is what keeps the two from
// separating again.
func buildStepsForChoices(m *Model) []InstallStep {
	return planFor(m.planOptions())
}
