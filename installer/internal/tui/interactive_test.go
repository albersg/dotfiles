package tui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
)

func TestGetHomebrewScriptUsesTTYSafePrompt(t *testing.T) {
	originalPath := os.Getenv("PATH")
	t.Cleanup(func() {
		_ = os.Setenv("PATH", originalPath)
	})

	if err := os.Setenv("PATH", ""); err != nil {
		t.Fatalf("failed to clear PATH: %v", err)
	}

	script, err := getHomebrewScript(&Model{})
	if err != nil {
		t.Fatalf("getHomebrewScript returned error: %v", err)
	}

	if !strings.Contains(script, `if [ -t 0 ] && [ -r /dev/tty ]; then`) {
		t.Fatalf("expected Homebrew script to guard prompt with tty check")
	}

	if !strings.Contains(script, `IFS= read -r dummy < /dev/tty`) {
		t.Fatalf("expected Homebrew script to read from /dev/tty")
	}

	if strings.Contains(script, "echo \"Press Enter to continue...\"\nread dummy") {
		t.Fatalf("expected Homebrew script to avoid unconditional read dummy")
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
// a tea.Cmd, and running it yields either needsExecProcessMsg -- the message
// whose only purpose is to hand a shell command to tea.ExecProcess, i.e. to run
// it -- or execFinishedMsg, which skips the step. The test never calls
// tea.ExecProcess, so nothing runs: it asserts what the TUI decided it would
// execute, which is the decision under test. Building a real command would also
// be observable only through that message, so this is the seam the code offers
// rather than a mock. See TestInteractiveStepStillDispatchesWithoutDryRun for
// the other side of the same boundary.
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

// TestInteractiveStepStillDispatchesWithoutDryRun is the other side of the same
// boundary: without --dry-run the interactive step must still produce the
// message that runs it. Without this, deleting the interactive path entirely
// would satisfy the dry-run test, and the test would prove nothing about the
// dispatch it claims to cover.
func TestInteractiveStepStillDispatchesWithoutDryRun(t *testing.T) {
	t.Setenv("DOTFILES_DRY_RUN", "")
	t.Setenv("PATH", t.TempDir())

	m := Model{
		SystemInfo:  &system.SystemInfo{OS: system.OSLinux, HasBrew: false},
		Screen:      ScreenInstalling,
		CurrentStep: 0,
		Steps:       []InstallStep{{ID: "homebrew", Name: "Install Homebrew", Interactive: true}},
	}

	msg := m.runNextStep()()

	needs, ok := msg.(needsExecProcessMsg)
	if !ok {
		t.Fatalf("without --dry-run the homebrew step produced %T, want needsExecProcessMsg", msg)
	}
	if needs.stepID != "homebrew" {
		t.Fatalf("message carries step %q, want %q", needs.stepID, "homebrew")
	}
	if needs.cmd == nil {
		t.Fatal("needsExecProcessMsg carried no command")
	}
	// The command is never handed to tea.ExecProcess, so it never runs.
}
