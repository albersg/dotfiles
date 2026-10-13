package tui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
	tea "github.com/charmbracelet/bubbletea"
)

// interactiveStepScripts builds one script for every entry in the interactive
// dispatch table, against a model rich enough for the builder to succeed. It is
// the shared setup for the guarantees every step script keeps while the run
// stays on screen.
func interactiveStepScripts(t *testing.T) map[string]string {
	t.Helper()

	// No host brew or terminal binary may change which branch the builders take.
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	repoDir, _, _ := newWSLLayout(t)

	sudoOff := Model{SystemInfo: &system.SystemInfo{OS: system.OSLinux}}
	depsOnArch := Model{SystemInfo: &system.SystemInfo{OS: system.OSArch}}
	terminal := Model{
		SystemInfo: &system.SystemInfo{OS: system.OSLinux},
		Choices:    UserChoices{Terminal: "ghostty"},
		RepoDir:    repoDir,
	}
	setshell := Model{
		SystemInfo: &system.SystemInfo{OS: system.OSLinux},
		Choices:    UserChoices{Shell: "fish"},
	}
	wsl := wslModel(repoDir, true)

	models := map[string]*Model{
		"homebrew":  &sudoOff,
		"deps":      &depsOnArch,
		"terminal":  &terminal,
		"setshell":  &setshell,
		"wslconfig": &wsl,
	}

	scripts := map[string]string{}
	for stepID, m := range models {
		script, err := getInteractiveScript(stepID, m)
		if err != nil {
			t.Fatalf("getInteractiveScript(%q) failed: %v", stepID, err)
		}
		if script == "" {
			t.Fatalf("getInteractiveScript(%q) produced no script", stepID)
		}
		scripts[stepID] = script
	}
	return scripts
}

// TestInteractiveScriptsDoNotPromptForEnter is the T3 guard: the run keeps the
// screen, so a script that waits for a keypress reads from a shell with no
// terminal and blocks where nobody can answer. The test refuses to let the
// prompt come back into a step that no longer needs it.
func TestInteractiveScriptsDoNotPromptForEnter(t *testing.T) {
	markers := []string{"Press Enter to continue", "read -r _", "read dummy"}

	for stepID, script := range interactiveStepScripts(t) {
		for _, marker := range markers {
			if strings.Contains(script, marker) {
				t.Errorf("step %q script still waits for a keypress (%q):\n%s", stepID, marker, script)
			}
		}
	}
}

// TestInteractiveScriptsUseNonInteractiveSudo pins T1 for the script path: with
// the credential cached before the run, every sudo invocation must use -n so a
// script that has no credential fails immediately instead of prompting on a
// terminal the captured executor does not own.
func TestInteractiveScriptsUseNonInteractiveSudo(t *testing.T) {
	forbidden := []string{
		"sudo pacman", "sudo dnf", "sudo apt-get", "sudo cp ", "sudo install ",
		"sudo tee", "sudo usermod", "sudo chsh",
	}

	for stepID, script := range interactiveStepScripts(t) {
		for _, bad := range forbidden {
			if strings.Contains(script, bad) {
				t.Errorf("step %q script runs %q without -n; the captured executor has no terminal, so sudo must fail instead of prompting:\n%s", stepID, bad, script)
			}
		}
	}
}

// TestInteractiveDispatchNeverHandsOverTheTerminal is the T2 boundary: the step
// loop must not produce the message whose only purpose is to give the terminal
// to a child process. The step's script runs through the captured executor
// instead, so the installer stays on screen for the whole run.
func TestInteractiveDispatchNeverHandsOverTheTerminal(t *testing.T) {
	t.Setenv("DOTFILES_DRY_RUN", "")
	t.Setenv("PATH", t.TempDir())

	m := Model{
		SystemInfo:  &system.SystemInfo{OS: system.OSLinux, HasBrew: false},
		Screen:      ScreenInstalling,
		CurrentStep: 0,
		Steps:       []InstallStep{{ID: "homebrew", Name: "Install Homebrew", Interactive: true}},
	}

	msg := m.runNextStep()()

	if _, ok := msg.(needsExecProcessMsg); ok {
		t.Fatal("the step loop handed the terminal to the script; the installer must stay on screen")
	}
}

func TestDescribeInteractiveStepError(t *testing.T) {
	t.Run("homebrew install failure stays specific when brew absent", func(t *testing.T) {
		originalPath := os.Getenv("PATH")
		t.Cleanup(func() {
			_ = os.Setenv("PATH", originalPath)
		})

		if err := os.Setenv("PATH", ""); err != nil {
			t.Fatalf("failed to clear PATH: %v", err)
		}

		err := describeInteractiveStepError("homebrew", errors.New("exit status 1"))
		if !strings.Contains(err.Error(), "Homebrew installation command failed") {
			t.Fatalf("expected install failure message, got %q", err.Error())
		}
	})

	t.Run("post-install failure does not blame homebrew when brew exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		brewPath := filepath.Join(tmpDir, "brew")
		if err := os.WriteFile(brewPath, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
			t.Fatalf("failed to create fake brew: %v", err)
		}

		originalPath := os.Getenv("PATH")
		t.Cleanup(func() {
			_ = os.Setenv("PATH", originalPath)
		})

		if err := os.Setenv("PATH", tmpDir); err != nil {
			t.Fatalf("failed to set PATH: %v", err)
		}

		err := describeInteractiveStepError("homebrew", errors.New("exit status 1"))
		if !strings.Contains(err.Error(), "Homebrew installed, but post-install shell setup did not complete cleanly") {
			t.Fatalf("expected post-install failure message, got %q", err.Error())
		}
	})

	t.Run("non-homebrew steps pass through unchanged", func(t *testing.T) {
		original := errors.New("boom")
		if got := describeInteractiveStepError("deps", original); !errors.Is(got, original) {
			t.Fatalf("expected original error to pass through")
		}
	})
}

// TestDryRunSkipsInteractiveSteps is the regression test for issue #127: the
// TUI routes interactive steps around executeStep (runNextStep calls
// runInteractiveStep), so DOTFILES_DRY_RUN=1 did not stop a documented no-op run
// from performing a real Homebrew install, sudo package installs and chsh.
//
// The assertion is made at the TUI's own dispatch boundary. runNextStep returns
// a tea.Cmd; under the flag an interactive step yields execFinishedMsg without
// running anything, and it must never ask for the administrator credential. The
// test runs the returned command, so it asserts what the TUI decided it would
// execute rather than what a mock was told. See
// TestInteractiveStepRunsThroughTheCapturedExecutor for the other side of the
// same boundary.
func TestDryRunSkipsInteractiveSteps(t *testing.T) {
	t.Setenv("DOTFILES_DRY_RUN", "1")
	t.Setenv("DOTFILES_VERBOSE", "1")
	SetNonInteractiveMode(true)
	t.Cleanup(func() { SetNonInteractiveMode(false) })

	cases := []struct {
		name   string
		stepID string
		model  Model
	}{
		{
			// Absent brew is what makes the Homebrew script non-empty; without
			// the dry-run gate this is the step that would install Homebrew.
			name:   "homebrew",
			stepID: "homebrew",
			model: Model{
				SystemInfo: &system.SystemInfo{OS: system.OSLinux, HasBrew: false},
			},
		},
		{
			// The shell step is the chsh path named in the issue.
			name:   "setshell",
			stepID: "setshell",
			model: Model{
				SystemInfo: &system.SystemInfo{OS: system.OSLinux},
				Choices:    UserChoices{Shell: "fish"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// A private PATH keeps the developer's own brew (or a real host)
			// out of the presence check, so the step has a command it would
			// really run without the gate.
			t.Setenv("PATH", t.TempDir())

			readLog := captureStepStdout(t)

			m := tc.model
			m.Screen = ScreenInstalling
			m.CurrentStep = 0
			m.Steps = []InstallStep{{ID: tc.stepID, Name: tc.stepID, Interactive: true}}

			msg := m.runNextStep()()

			if _, ok := msg.(needsExecProcessMsg); ok {
				t.Fatalf("dry run handed step %q to tea.ExecProcess, so it would run for real", tc.stepID)
			}

			finished, ok := msg.(execFinishedMsg)
			if !ok {
				t.Fatalf("dry run produced %T for step %q, want execFinishedMsg", msg, tc.stepID)
			}
			if finished.stepID != tc.stepID {
				t.Fatalf("message carries step %q, want %q", finished.stepID, tc.stepID)
			}
			if finished.err != nil {
				t.Fatalf("dry run failed interactive step %q: %v", tc.stepID, finished.err)
			}

			// The run must continue exactly as executeStep's dry run does: the
			// skipped step is marked done, the cursor advances and the loop moves
			// on. nextCmd is inspected but never run, so this stays hermetic even
			// if a regression hands back a command that would execute something.
			result, nextCmd := m.Update(finished)
			m = result.(Model)
			if m.Steps[0].Status != StatusDone {
				t.Fatalf("skipped step status = %v, want StatusDone", m.Steps[0].Status)
			}
			if m.CurrentStep != 1 {
				t.Fatalf("CurrentStep = %d, want 1 after the skipped step", m.CurrentStep)
			}
			_ = nextCmd

			wantLog := fmt.Sprintf("DRY RUN: skipping step %q", tc.stepID)
			if log := readLog(); !strings.Contains(log, wantLog) {
				t.Errorf("dry run did not report the skip; log = %q, want it to contain %q", log, wantLog)
			}
		})
	}
}

// TestSudoCredentialIsRequestedOnceBeforeAStepThatNeedsIt is the T1 boundary:
// the first step that needs root asks for the credential once and shows the ask
// on the installing screen; with the credential cached, no later step asks
// again.
func TestSudoCredentialIsRequestedOnceBeforeAStepThatNeedsIt(t *testing.T) {
	t.Setenv("DOTFILES_DRY_RUN", "")

	originalPrimer := sudoPrimer
	t.Cleanup(func() { sudoPrimer = originalPrimer })
	asked := 0
	sudoPrimer = func() tea.Cmd {
		asked++
		return func() tea.Msg { return sudoPrimedMsg{} }
	}

	m := Model{
		SystemInfo:  &system.SystemInfo{OS: system.OSLinux, HasBrew: false},
		Screen:      ScreenInstalling,
		CurrentStep: 0,
		Steps:       []InstallStep{{ID: "deps", Name: "Install Dependencies"}},
	}

	msg := m.runNextStep()()
	if _, ok := msg.(sudoPrimedMsg); !ok {
		t.Fatalf("the first root-needing step produced %T, want sudoPrimedMsg", msg)
	}
	if asked != 1 {
		t.Fatalf("the credential was requested %d times, want exactly 1", asked)
	}
	if len(m.LogLines) == 0 {
		t.Fatal("the credential request is not visible on the installing screen")
	}

	// The ask happens once: with the credential cached, the same step is primed
	// and does not ask again.
	m.SudoPrimed = true
	if m.shouldPrimeSudo("deps") {
		t.Fatal("a primed run would ask for the credential a second time")
	}
}

// TestFailedCredentialRequestIsNotRepeated pins the at-most-once promise even
// when the one request fails: a cancelled or failed ask must not become a loop,
// and the run continues so the steps that need root report the missing
// credential visibly.
func TestFailedCredentialRequestIsNotRepeated(t *testing.T) {
	t.Setenv("DOTFILES_DRY_RUN", "")

	originalPrimer := sudoPrimer
	t.Cleanup(func() { sudoPrimer = originalPrimer })
	asked := 0
	sudoPrimer = func() tea.Cmd {
		asked++
		return func() tea.Msg { return sudoPrimedMsg{err: errors.New("cancelled")} }
	}

	m := Model{
		SystemInfo:  &system.SystemInfo{OS: system.OSLinux, HasBrew: false},
		Screen:      ScreenInstalling,
		CurrentStep: 0,
		Steps:       []InstallStep{{ID: "deps", Name: "Install Dependencies"}},
	}

	_ = m.runNextStep()()
	result, _ := m.Update(sudoPrimedMsg{err: errors.New("cancelled")})
	m = result.(Model)

	if asked != 1 {
		t.Fatalf("the credential was requested %d times, want exactly 1", asked)
	}
	if m.shouldPrimeSudo("deps") {
		t.Fatal("a failed credential request would be asked again in a loop")
	}
}

// TestSudoIsNotRequestedUnderDryRun pins the other side: a documented no-op run
// asks for nothing, so --dry-run stays hermetic.
func TestSudoIsNotRequestedUnderDryRun(t *testing.T) {
	t.Setenv("DOTFILES_DRY_RUN", "1")

	originalPrimer := sudoPrimer
	t.Cleanup(func() { sudoPrimer = originalPrimer })
	sudoPrimer = func() tea.Cmd {
		t.Fatal("a dry run asked for the administrator credential")
		return nil
	}

	m := Model{
		SystemInfo:  &system.SystemInfo{OS: system.OSLinux, HasBrew: false},
		Screen:      ScreenInstalling,
		CurrentStep: 0,
		Steps:       []InstallStep{{ID: "deps", Name: "Install Dependencies"}},
	}

	msg := m.runNextStep()()
	if _, ok := msg.(sudoPrimedMsg); ok {
		t.Fatalf("a dry-run dispatch asked for the credential: %T", msg)
	}
}

// TestInteractiveStepRunsThroughTheCapturedExecutor is the T2 boundary: without
// --dry-run the step still runs, but through the captured-output executor that
// puts its output in the activity log, never through a terminal handback.
func TestInteractiveStepRunsThroughTheCapturedExecutor(t *testing.T) {
	t.Setenv("DOTFILES_DRY_RUN", "")
	t.Setenv("PATH", t.TempDir())

	originalRunner := runStepScript
	t.Cleanup(func() { runStepScript = originalRunner })
	var ranStep, ranScript string
	runStepScript = func(stepID, script string) error {
		ranStep, ranScript = stepID, script
		return nil
	}

	m := Model{
		SystemInfo:  &system.SystemInfo{OS: system.OSLinux, HasBrew: false},
		Screen:      ScreenInstalling,
		CurrentStep: 0,
		SudoPrimed:  true,
		Steps:       []InstallStep{{ID: "homebrew", Name: "Install Homebrew", Interactive: true}},
	}

	msg := m.runNextStep()()
	if _, ok := msg.(needsExecProcessMsg); ok {
		t.Fatal("the step handed the terminal over instead of running captured")
	}
	finished, ok := msg.(execFinishedMsg)
	if !ok {
		t.Fatalf("the interactive step produced %T, want execFinishedMsg", msg)
	}
	if finished.err != nil {
		t.Fatalf("the captured run failed: %v", finished.err)
	}
	if ranStep != "homebrew" || ranScript == "" {
		t.Fatalf("the captured executor ran step %q with script %q, want the homebrew script", ranStep, ranScript)
	}
}

// TestInteractiveStepRefusesAScriptThatPromptsForEnter is the teeth of the T3
// guard: reintroducing the prompt into a step's script makes the step fail
// visibly instead of blocking a run with no terminal to read from.
func TestInteractiveStepRefusesAScriptThatPromptsForEnter(t *testing.T) {
	t.Setenv("DOTFILES_DRY_RUN", "")

	originalBuilder := interactiveScriptBuilders["homebrew"]
	t.Cleanup(func() { interactiveScriptBuilders["homebrew"] = originalBuilder })
	interactiveScriptBuilders["homebrew"] = func(*Model) (string, error) {
		return "#!/bin/sh\necho 'Press Enter to continue...'\nread -r _\n", nil
	}

	m := Model{
		SystemInfo:  &system.SystemInfo{OS: system.OSLinux, HasBrew: false},
		Screen:      ScreenInstalling,
		CurrentStep: 0,
		SudoPrimed:  true,
		Steps:       []InstallStep{{ID: "homebrew", Name: "Install Homebrew", Interactive: true}},
	}

	finished, ok := m.runNextStep()().(execFinishedMsg)
	if !ok {
		t.Fatal("the step with a returning prompt did not report a failure")
	}
	if finished.err == nil || !strings.Contains(finished.err.Error(), "enter-to-continue") {
		t.Fatalf("the guard did not refuse the prompt: err = %v", finished.err)
	}
}

// TestStepScriptPromptsForEnterRecognisesThePrompt pins the guard's own
// recognition, so a rewritten marker cannot make it silently accept a prompt.
func TestStepScriptPromptsForEnterRecognisesThePrompt(t *testing.T) {
	for _, script := range []string{
		"echo 'Press Enter to continue...'\nread -r _\n",
		"echo 'continuing'\nread dummy\n",
	} {
		if !stepScriptPromptsForEnter(script) {
			t.Errorf("the guard did not recognise a prompt in %q", script)
		}
	}
	if stepScriptPromptsForEnter("#!/bin/sh\necho installing\n") {
		t.Error("the guard refused a script with no prompt")
	}
}

// TestSudoKeepAliveRefreshesWithoutATerminal pins the keep-alive: while the
// credential is cached, a refresh runs through sudo -n and re-arms itself, and
// an unprimed run never refreshes.
func TestSudoKeepAliveRefreshesWithoutATerminal(t *testing.T) {
	original := sudoRefresher
	t.Cleanup(func() { sudoRefresher = original })
	calls := 0
	sudoRefresher = func() error { calls++; return nil }

	result, cmd := (&Model{SudoPrimed: true}).Update(sudoKeepAliveMsg{})
	if cmd == nil {
		t.Fatal("a primed run armed no refresh")
	}
	if calls != 0 {
		t.Fatal("the refresh must run in the command, not in Update")
	}
	got := cmd()
	refreshed, ok := got.(sudoRefreshedMsg)
	if !ok || refreshed.err != nil {
		t.Fatalf("the refresh produced %#v, want a clean sudoRefreshedMsg", got)
	}
	if calls != 1 {
		t.Fatalf("the refresh ran %d times, want 1", calls)
	}
	if _, rearm := result.(Model).Update(refreshed); rearm == nil {
		t.Fatal("a successful refresh did not re-arm the keep-alive")
	}

	if _, cmd := (&Model{}).Update(sudoKeepAliveMsg{}); cmd != nil {
		t.Fatal("an unprimed run armed a credential refresh")
	}
}

// TestNonInteractiveSudoUsesDashN pins the single place the -n flag is written,
// so every executor path that runs root through the shared helper inherits it.
func TestNonInteractiveSudoUsesDashN(t *testing.T) {
	if got := nonInteractiveSudo("apt-get install -y curl"); got != "sudo -n apt-get install -y curl" {
		t.Fatalf("nonInteractiveSudo = %q, want the -n flag", got)
	}
	plan := platformInstallPlan{Manager: "pacman", Packages: "git"}
	if got := plan.sudoCommand(); !strings.HasPrefix(got, "sudo -n ") {
		t.Fatalf("plan sudoCommand = %q, want it to start with sudo -n", got)
	}
}
