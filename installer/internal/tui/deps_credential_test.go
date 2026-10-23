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

// depsCredentialRecorder records, in one ordered list, everything the dependency
// step did about the administrator credential and every command it ran. The order
// is the point: the credential has to be cached before the first command that
// needs root, and a list is the only evidence that can show it.
type depsCredentialRecorder struct {
	events []string
}

func (r *depsCredentialRecorder) record(event string) {
	r.events = append(r.events, event)
}

// sudoEvents returns the events that are root commands, in the order they ran.
func (r *depsCredentialRecorder) sudoEvents() []string {
	var commands []string
	for _, event := range r.events {
		if strings.HasPrefix(event, "sudo:") {
			commands = append(commands, event)
		}
	}
	return commands
}

// count returns how many times the named event was recorded.
func (r *depsCredentialRecorder) count(event string) int {
	total := 0
	for _, recorded := range r.events {
		if recorded == event {
			total++
		}
	}
	return total
}

// withDepsCredentialMocks replaces the credential seams and the command runners
// the dependency step can reach, and returns the recorder they append to.
//
// probeErr is what the "is a credential already cached" ask answers: nil means
// sudo can already run without a prompt, which is the case the step must leave
// alone. askErr is what the terminal ask answers, so a refused password can be
// exercised without a terminal.
func withDepsCredentialMocks(t *testing.T, probeErr, askErr error) *depsCredentialRecorder {
	t.Helper()

	originalProbe := sudoCredentialCached
	originalAsk := sudoTerminalAsk
	originalSudo := runSudoWithLogs
	originalBrew := runBrewWithLogs
	originalPkg := runPkgInstallWithLogs

	recorder := &depsCredentialRecorder{}

	sudoCredentialCached = func() error {
		recorder.record("probe")
		return probeErr
	}
	sudoTerminalAsk = func() error {
		recorder.record("ask")
		return askErr
	}
	runSudoWithLogs = func(command string, opts *system.ExecOptions, onLog system.LogCallback) *system.ExecResult {
		recorder.record("sudo:" + command)
		return &system.ExecResult{Command: command}
	}
	runBrewWithLogs = func(args string, opts *system.ExecOptions, onLog system.LogCallback) *system.ExecResult {
		recorder.record("brew:" + args)
		return &system.ExecResult{Command: args}
	}
	runPkgInstallWithLogs = func(packages string, opts *system.ExecOptions, onLog func(string)) *system.ExecResult {
		recorder.record("pkg:" + packages)
		return &system.ExecResult{Command: packages}
	}

	t.Cleanup(func() {
		sudoCredentialCached = originalProbe
		sudoTerminalAsk = originalAsk
		runSudoWithLogs = originalSudo
		runBrewWithLogs = originalBrew
		runPkgInstallWithLogs = originalPkg
	})

	return recorder
}

// TestTheDependencyStepAsksForTheCredentialBeforeItsFirstRootCommand is the
// reported failure, pinned. The scripted route (`--non-interactive`) executes
// stepInstallDeps directly, so nothing had cached a credential when its first
// `sudo -n apt-get update` ran, and sudo answered "a password is required" on a
// machine that would have asked for one.
//
// The step now asks, once, before that command -- and through the terminal, which
// is where sudo can read a password.
func TestTheDependencyStepAsksForTheCredentialBeforeItsFirstRootCommand(t *testing.T) {
	t.Setenv("DOTFILES_DRY_RUN", "")

	recorder := withDepsCredentialMocks(t, errors.New("sudo: a password is required"), nil)

	m := &Model{SystemInfo: &system.SystemInfo{OS: system.OSDebian, IsWSL: true, HasBrew: false}}
	if err := stepInstallDeps(m); err != nil {
		t.Fatalf("stepInstallDeps failed: %v", err)
	}

	if recorder.count("ask") != 1 {
		t.Fatalf("the credential was asked for %d times, want exactly 1; events = %v", recorder.count("ask"), recorder.events)
	}

	commands := recorder.sudoEvents()
	if len(commands) == 0 {
		t.Fatalf("the dependency step ran no root command; events = %v", recorder.events)
	}

	askAt, firstCommandAt := -1, -1
	for i, event := range recorder.events {
		if event == "ask" && askAt < 0 {
			askAt = i
		}
		if strings.HasPrefix(event, "sudo:") && firstCommandAt < 0 {
			firstCommandAt = i
		}
	}
	if askAt > firstCommandAt {
		t.Fatalf("the credential was asked for after the first root command ran: %v", recorder.events)
	}

	// The ask is the run's, not the step's own arrangement: the model records that
	// this run has already made its request, so a later step does not ask again.
	if !m.SudoPrimed {
		t.Error("the step cached a credential but the model does not record it as primed")
	}
	if !m.SudoPrimeAttempted {
		t.Error("the request is not recorded as made, so a later step would ask again")
	}
}

// TestTheCachedCredentialIsNeverAskedFor pins the other side of the same
// decision: a machine where sudo already holds a credential it can use -- a
// container running as root, a NOPASSWD rule -- is not prompted at all, so the
// scripted route stays as unattended as it was.
func TestTheCachedCredentialIsNeverAskedFor(t *testing.T) {
	t.Setenv("DOTFILES_DRY_RUN", "")

	recorder := withDepsCredentialMocks(t, nil, nil)

	m := &Model{SystemInfo: &system.SystemInfo{OS: system.OSDebian, HasBrew: false}}
	if err := stepInstallDeps(m); err != nil {
		t.Fatalf("stepInstallDeps failed: %v", err)
	}

	if recorder.count("ask") != 0 {
		t.Fatalf("a machine with a usable credential was asked for one: %v", recorder.events)
	}
	if len(recorder.sudoEvents()) == 0 {
		t.Fatalf("the dependency step ran no root command; events = %v", recorder.events)
	}
	if !m.SudoPrimed {
		t.Error("a usable credential was found but the model does not record the run as primed")
	}
}

// TestTheDependencyStepDoesNotAskTwice pins the at-most-once promise across the
// two routes. The step loop asks before the first step that needs root; a run
// that already made that request -- whether it succeeded or failed -- is not asked
// again by the step itself, because a repeated failed ask is the loop the run
// avoids, and a step without the credential reports it through `sudo -n`.
func TestTheDependencyStepDoesNotAskTwice(t *testing.T) {
	t.Setenv("DOTFILES_DRY_RUN", "")

	cases := []struct {
		name  string
		model *Model
	}{
		{
			name:  "the step loop cached the credential",
			model: &Model{SystemInfo: &system.SystemInfo{OS: system.OSDebian}, SudoPrimed: true, SudoPrimeAttempted: true},
		},
		{
			name:  "the step loop asked and the request failed",
			model: &Model{SystemInfo: &system.SystemInfo{OS: system.OSDebian}, SudoPrimed: false, SudoPrimeAttempted: true},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := withDepsCredentialMocks(t, errors.New("sudo: a password is required"), nil)

			if err := stepInstallDeps(tc.model); err != nil {
				t.Fatalf("stepInstallDeps failed: %v", err)
			}

			if recorder.count("ask") != 0 {
				t.Fatalf("the run asked for the credential a second time: %v", recorder.events)
			}
			if recorder.count("probe") != 0 {
				t.Fatalf("the run probed sudo again after its one request: %v", recorder.events)
			}
		})
	}
}

// TestTheDependencyStepAsksNothingUnderDryRun keeps the documented no-op run
// hermetic: a dry run installs nothing, so it also has no credential to ask for.
func TestTheDependencyStepAsksNothingUnderDryRun(t *testing.T) {
	t.Setenv("DOTFILES_DRY_RUN", "1")

	recorder := withDepsCredentialMocks(t, errors.New("sudo: a password is required"), nil)

	m := &Model{SystemInfo: &system.SystemInfo{OS: system.OSDebian, HasBrew: false}}
	if err := stepInstallDeps(m); err != nil {
		t.Fatalf("stepInstallDeps failed: %v", err)
	}

	if recorder.count("probe") != 0 || recorder.count("ask") != 0 {
		t.Fatalf("a dry run asked for the administrator credential: %v", recorder.events)
	}
}

// TestAUserScopedPlanAsksForNothing pins the scope of the request to the plan
// that actually runs through sudo. Homebrew and pkg are user-scoped, so a step
// that installs with them asks for no credential at all -- the ask follows the
// command instead of the step's name.
func TestAUserScopedPlanAsksForNothing(t *testing.T) {
	t.Setenv("DOTFILES_DRY_RUN", "")

	recorder := withDepsCredentialMocks(t, errors.New("sudo: a password is required"), nil)

	m := &Model{SystemInfo: &system.SystemInfo{OS: system.OSDebian, IsWSL: true, HasBrew: true}}
	if err := stepInstallDeps(m); err != nil {
		t.Fatalf("stepInstallDeps failed: %v", err)
	}

	if recorder.count("probe") != 0 || recorder.count("ask") != 0 {
		t.Fatalf("a user-scoped install asked for the administrator credential: %v", recorder.events)
	}
	if len(recorder.sudoEvents()) != 0 {
		t.Fatalf("a Homebrew plan ran a root command: %v", recorder.events)
	}
}

// TestARefusedCredentialDoesNotAbortTheDependencyStep pins what happens when the
// person declines the prompt. The step keeps its behaviour: the command runs, sudo
// refuses it where the user can read it, and the step reports the failure with the
// commands to run by hand -- the same report a run that was never primed gave.
func TestARefusedCredentialDoesNotAbortTheDependencyStep(t *testing.T) {
	t.Setenv("DOTFILES_DRY_RUN", "")

	recorder := withDepsCredentialMocks(t, errors.New("sudo: a password is required"), errors.New("the terminal ask was cancelled"))

	m := &Model{SystemInfo: &system.SystemInfo{OS: system.OSDebian, HasBrew: false}}
	if err := stepInstallDeps(m); err != nil {
		t.Fatalf("a refused credential aborted the step instead of reporting through sudo: %v", err)
	}

	if len(recorder.sudoEvents()) == 0 {
		t.Fatalf("a refused credential stopped the step's commands from running: %v", recorder.events)
	}
	if m.SudoPrimed {
		t.Error("a refused credential is recorded as a cached one")
	}
	if !m.SudoPrimeAttempted {
		t.Error("a refused credential is not recorded as asked, so it would be asked again")
	}
}

// TestTheDependencyStepAsksTheSameWayTheRunDoes pins the ask itself against a
// real child process, because the question is which flags sudo is given.
//
// The probe (`sudo -n -v`) must never prompt: it is what makes a machine with a
// usable credential silent. The terminal ask (`sudo -v`) must be the one that can
// prompt, so the password is typed where sudo reads it, and it must carry neither
// `-n` (which would refuse to ask) nor `-S` (which needs the password on standard
// input, where this path has none to give).
func TestTheDependencyStepAsksTheSameWayTheRunDoes(t *testing.T) {
	argsFile, _ := fakeSudo(t)

	if err := defaultSudoCredentialCached(); err != nil {
		t.Fatalf("the probe against a sudo that succeeds reported %v, want no error", err)
	}
	probeArgs, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("the probe ran no sudo: %v", err)
	}
	for _, want := range []string{"-n", "-v"} {
		if !strings.Contains(string(probeArgs), want) {
			t.Fatalf("the probe did not give sudo %q: %q", want, probeArgs)
		}
	}
	if strings.Contains(string(probeArgs), "-S") {
		t.Fatalf("the probe asked sudo to read a password from standard input: %q", probeArgs)
	}

	if err := defaultSudoTerminalAsk(); err != nil {
		t.Fatalf("the terminal ask against a sudo that succeeds reported %v, want no error", err)
	}
	askArgs, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("the terminal ask ran no sudo: %v", err)
	}
	var flags []string
	for _, arg := range strings.Split(strings.TrimSpace(string(askArgs)), "\n") {
		flags = append(flags, arg)
	}
	if len(flags) != 1 || flags[0] != "-v" {
		t.Fatalf("the terminal ask gave sudo %v, want exactly [-v]: %q", flags, askArgs)
	}
}

// TestTheTerminalAskFailsWhereThereIsNoTerminal pins the honest failure: a
// scripted run with nothing to prompt on reports the refusal instead of waiting
// on a question nobody can answer.
func TestTheTerminalAskFailsWhereThereIsNoTerminal(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\necho 'sudo: no tty present and no askpass program specified' >&2\nexit 1\n"
	if err := os.WriteFile(filepath.Join(dir, "sudo"), []byte(script), 0o700); err != nil {
		t.Fatalf("writing the fake sudo: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	err := defaultSudoTerminalAsk()
	if err == nil {
		t.Fatal("a sudo that could not ask for a password reported success")
	}
	if !strings.Contains(err.Error(), "exit status") {
		t.Fatalf("the refusal does not carry sudo's own failure: %v", err)
	}
}

// TestTheRootPlanIsTheOneThatAsks keeps the credential request tied to the plan
// instead of to the step's name: the managers that run through sudo are exactly
// the plan's native ones, so a plan that would run no root command cannot ask for
// a credential and a plan that runs one cannot skip the ask.
func TestTheRootPlanIsTheOneThatAsks(t *testing.T) {
	cases := []struct {
		manager string
		want    bool
	}{
		{manager: "apt-get", want: true},
		{manager: "pacman", want: true},
		{manager: "dnf", want: true},
		{manager: "brew", want: false},
		{manager: "pkg", want: false},
		{manager: "", want: false},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("manager=%q", tc.manager), func(t *testing.T) {
			plan := platformInstallPlan{Manager: tc.manager}
			if got := plan.needsRoot(); got != tc.want {
				t.Fatalf("plan(%q).needsRoot() = %v, want %v", tc.manager, got, tc.want)
			}
			// The plan's own answer and the command it would run must agree: a plan
			// that needs root spells its command with sudo, and one that does not
			// never does.
			if hasSudo := strings.HasPrefix(plan.sudoCommand(), "sudo -n "); hasSudo != tc.want {
				t.Fatalf("plan(%q).sudoCommand() = %q, which does not agree with needsRoot() = %v", tc.manager, plan.sudoCommand(), tc.want)
			}
		})
	}
}
