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
// is the point: the credential has to be probed before the first command that
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

// withDepsCredentialMocks replaces the credential probe and the command runners
// the dependency step can reach, and returns the recorder they append to.
//
// probeErr is what the "is a credential already cached" ask answers: nil means
// sudo can already run without a prompt, which is the case the step must leave
// alone. There is no second seam, because this route has no screen to ask on: a
// probe that fails is reported rather than prompted for.
func withDepsCredentialMocks(t *testing.T, probeErr error) *depsCredentialRecorder {
	t.Helper()

	originalProbe := sudoCredentialCached
	originalSudo := runSudoWithLogs
	originalBrew := runBrewWithLogs
	originalPkg := runPkgInstallWithLogs

	recorder := &depsCredentialRecorder{}

	sudoCredentialCached = func() error {
		recorder.record("probe")
		return probeErr
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
		runSudoWithLogs = originalSudo
		runBrewWithLogs = originalBrew
		runPkgInstallWithLogs = originalPkg
	})

	return recorder
}

// TestTheScriptedStepProbesTheCredentialBeforeItsFirstRootCommand pins the half of
// L37 that is still true. The scripted route (`--non-interactive`) executes
// stepInstallDeps directly, so nothing had cached a credential when its first
// `sudo -n apt-get update` ran, and sudo answered "a password is required" on a
// machine that would have asked for one.
//
// The step now asks the machine first, once, before that command -- and the run
// records that its one request was made, so a later step does not repeat it.
func TestTheScriptedStepProbesTheCredentialBeforeItsFirstRootCommand(t *testing.T) {
	t.Setenv("DOTFILES_DRY_RUN", "")

	recorder := withDepsCredentialMocks(t, errors.New("sudo: a password is required"))

	m := &Model{SystemInfo: &system.SystemInfo{OS: system.OSDebian, IsWSL: true, HasBrew: false}}
	if err := stepInstallDeps(m); err != nil {
		t.Fatalf("stepInstallDeps failed: %v", err)
	}

	if recorder.count("probe") != 1 {
		t.Fatalf("the credential was probed %d times, want exactly 1; events = %v", recorder.count("probe"), recorder.events)
	}

	commands := recorder.sudoEvents()
	if len(commands) == 0 {
		t.Fatalf("the dependency step ran no root command; events = %v", recorder.events)
	}

	probeAt, firstCommandAt := -1, -1
	for i, event := range recorder.events {
		if event == "probe" && probeAt < 0 {
			probeAt = i
		}
		if strings.HasPrefix(event, "sudo:") && firstCommandAt < 0 {
			firstCommandAt = i
		}
	}
	if probeAt > firstCommandAt {
		t.Fatalf("the credential was probed after the first root command ran: %v", recorder.events)
	}

	if m.SudoPrimed {
		t.Error("a machine whose sudo wants a password is recorded as primed")
	}
	if !m.SudoPrimeAttempted {
		t.Error("the request is not recorded as made, so a later step would ask again")
	}
}

// TestTheScriptedStepNeverAsksInTheOuterShell pins the shape of the fix on the one
// route that has no interface. `sudo -v` with the terminal attached used to run
// here, which is what put sudo's prompt in the shell and then hung. It is gone: the
// step reports the missing credential and lets the command fail with the manager's
// own reason, which is the failure the user can act on.
func TestTheScriptedStepNeverAsksInTheOuterShell(t *testing.T) {
	t.Setenv("DOTFILES_DRY_RUN", "")

	originalProbe := sudoCredentialCached
	t.Cleanup(func() { sudoCredentialCached = originalProbe })
	sudoCredentialCached = func() error { return errors.New("sudo: a password is required") }

	m := &Model{SystemInfo: &system.SystemInfo{OS: system.OSDebian, HasBrew: false}}
	err := m.primeSudoForStep()
	if err == nil {
		t.Fatal("a machine whose sudo wants a password was primed without one")
	}
	if !strings.Contains(err.Error(), "sudo -v") {
		t.Fatalf("the report does not say how to cache the credential: %v", err)
	}
	if !strings.Contains(err.Error(), "interactively") {
		t.Fatalf("the report does not say which run can ask on screen: %v", err)
	}
	if m.SudoPrimed {
		t.Error("a credential that was never cached is recorded as primed")
	}
	if !m.SudoPrimeAttempted {
		t.Error("the one attempt is not recorded, so it would be repeated")
	}

	// The source of that path may not carry the handover either: anything that
	// attaches this process's terminal is the mechanism the report is about.
	source, readErr := os.ReadFile("interactive.go")
	if readErr != nil {
		t.Fatalf("reading interactive.go: %v", readErr)
	}
	if strings.Contains(string(source), "RunTerminal") {
		t.Fatal("the credential path still attaches this process's terminal, which is the prompt in the shell that hung")
	}
}

// TestTheCachedCredentialIsNeverAskedFor pins the other side of the same decision:
// a machine where sudo already holds a credential it can use -- a container
// running as root, a NOPASSWD rule -- is not prompted at all, so the scripted route
// stays as unattended as it was.
func TestTheCachedCredentialIsNeverAskedFor(t *testing.T) {
	t.Setenv("DOTFILES_DRY_RUN", "")

	recorder := withDepsCredentialMocks(t, nil)

	m := &Model{SystemInfo: &system.SystemInfo{OS: system.OSDebian, HasBrew: false}}
	if err := stepInstallDeps(m); err != nil {
		t.Fatalf("stepInstallDeps failed: %v", err)
	}

	if len(recorder.sudoEvents()) == 0 {
		t.Fatalf("the dependency step ran no root command; events = %v", recorder.events)
	}
	if !m.SudoPrimed {
		t.Error("a usable credential was found but the model does not record the run as primed")
	}
}

// TestTheScriptedStepDoesNotAskTwice pins the at-most-once promise across the two
// routes. The step loop asks before the first step that needs root; a run that
// already made that request -- whether it succeeded or failed -- is not probed
// again by the step itself, because a repeated failed request is the loop the run
// avoids. A step without the credential reports it through `sudo -n`.
func TestTheScriptedStepDoesNotAskTwice(t *testing.T) {
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
			recorder := withDepsCredentialMocks(t, errors.New("sudo: a password is required"))

			if err := stepInstallDeps(tc.model); err != nil {
				t.Fatalf("stepInstallDeps failed: %v", err)
			}

			if recorder.count("probe") != 0 {
				t.Fatalf("the run probed sudo again after its one request: %v", recorder.events)
			}
		})
	}
}

// TestTheScriptedStepAsksNothingUnderDryRun keeps the documented no-op run
// hermetic: a dry run installs nothing, so it also has no credential to ask for.
func TestTheScriptedStepAsksNothingUnderDryRun(t *testing.T) {
	t.Setenv("DOTFILES_DRY_RUN", "1")

	recorder := withDepsCredentialMocks(t, errors.New("sudo: a password is required"))

	m := &Model{SystemInfo: &system.SystemInfo{OS: system.OSDebian, HasBrew: false}}
	if err := stepInstallDeps(m); err != nil {
		t.Fatalf("stepInstallDeps failed: %v", err)
	}

	if recorder.count("probe") != 0 {
		t.Fatalf("a dry run probed for the administrator credential: %v", recorder.events)
	}
}

// TestAUserScopedPlanAsksForNothing pins the scope of the request to the plan
// that actually runs through sudo. Homebrew and pkg are user-scoped, so a step
// that installs with them asks for no credential at all -- the ask follows the
// command instead of the step's name.
func TestAUserScopedPlanAsksForNothing(t *testing.T) {
	t.Setenv("DOTFILES_DRY_RUN", "")

	recorder := withDepsCredentialMocks(t, errors.New("sudo: a password is required"))

	m := &Model{SystemInfo: &system.SystemInfo{OS: system.OSDebian, IsWSL: true, HasBrew: true}}
	if err := stepInstallDeps(m); err != nil {
		t.Fatalf("stepInstallDeps failed: %v", err)
	}

	if recorder.count("probe") != 0 {
		t.Fatalf("a user-scoped install asked for the administrator credential: %v", recorder.events)
	}
	if len(recorder.sudoEvents()) != 0 {
		t.Fatalf("a Homebrew plan ran a root command: %v", recorder.events)
	}
}

// TestAMissingCredentialDoesNotAbortTheStepAndIsReported pins what happens when the
// credential cannot be cached on this route. The step keeps its behaviour: the
// command runs, sudo refuses it where the user can read it, and the step reports the
// failure with the manager's own words and the commands to run by hand -- the same
// report a run that was never primed gave. Nothing waits, and nothing prompts.
func TestAMissingCredentialDoesNotAbortTheStepAndIsReported(t *testing.T) {
	t.Setenv("DOTFILES_DRY_RUN", "")

	originalProbe := sudoCredentialCached
	originalSudo := runSudoWithLogs
	originalPkg := runPkgInstallWithLogs
	t.Cleanup(func() {
		sudoCredentialCached = originalProbe
		runSudoWithLogs = originalSudo
		runPkgInstallWithLogs = originalPkg
	})

	sudoCredentialCached = func() error { return errors.New("sudo: a password is required") }
	ran := 0
	runSudoWithLogs = func(command string, opts *system.ExecOptions, onLog system.LogCallback) *system.ExecResult {
		ran++
		return &system.ExecResult{Command: command, Error: errors.New("sudo: a password is required")}
	}
	runPkgInstallWithLogs = func(packages string, opts *system.ExecOptions, onLog func(string)) *system.ExecResult {
		return &system.ExecResult{Command: packages, Error: errors.New("sudo: a password is required")}
	}

	m := &Model{SystemInfo: &system.SystemInfo{OS: system.OSDebian, HasBrew: false}}
	err := stepInstallDeps(m)

	if ran == 0 {
		t.Fatal("a missing credential stopped the step's commands from running, so nothing reported why")
	}
	if err == nil {
		t.Fatal("a root command refused for want of a password was reported as a success")
	}
	// The report has to say what is missing and where it can be fixed: the run that
	// reaches this line is the one without a screen, so both ways out are named.
	if !strings.Contains(err.Error(), "no screen to ask") {
		t.Fatalf("the step's failure does not say why nothing asked: %v", err)
	}
	if !strings.Contains(err.Error(), "without --non-interactive") {
		t.Fatalf("the step's failure does not name the run that can ask on screen: %v", err)
	}
	if !strings.Contains(err.Error(), "run these commands in a terminal") {
		t.Fatalf("the step's failure does not name the commands to run by hand: %v", err)
	}
	if m.SudoPrimed {
		t.Error("a missing credential is recorded as a cached one")
	}
	if !m.SudoPrimeAttempted {
		t.Error("a failed request is not recorded as asked, so it would be asked again")
	}
}

// TestTheProbeAndTheScreenAskSudoTheSameWay pins the flags against a real child
// process, because the question is which channel the password travels on.
//
// The probe (`sudo -n -v`) must never prompt: it is what makes a machine with a
// usable credential silent. The attempt made from the installer's screen must give
// sudo `-S` -- read it from standard input -- and `-v`, and must never put the
// password in its arguments, where any account on the host could read it.
func TestTheProbeAndTheScreenAskSudoTheSameWay(t *testing.T) {
	argsFile, stdinFile := fakeSudo(t)

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

	password := []byte("typed on the installer's screen")
	if msg, ok := defaultSudoPasswordPrimer(password, 1)().(sudoPrimedMsg); !ok || msg.err != nil {
		t.Fatalf("the attempt against a sudo that succeeds reported %v, want no failure", msg.err)
	}
	attemptArgs, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("the attempt ran no sudo: %v", err)
	}
	flags := strings.Split(strings.TrimSpace(string(attemptArgs)), "\n")
	if len(flags) != 2 || flags[0] != "-S" || flags[1] != "-v" {
		t.Fatalf("the attempt gave sudo %v, want exactly [-S -v]", flags)
	}
	if strings.Contains(string(attemptArgs), "typed on the installer's screen") {
		t.Fatal("the password travelled in sudo's arguments, where any account on the host can read it")
	}
	stdin, err := os.ReadFile(stdinFile)
	if err != nil {
		t.Fatalf("the attempt recorded no standard input: %v", err)
	}
	if strings.TrimRight(string(stdin), "\n") != "typed on the installer's screen" {
		t.Fatalf("sudo read %q from its standard input, want the password", stdin)
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

// TestTheFakeSudoOnPathIsReallyUsed keeps the guards above honest: several of them
// depend on a `sudo` found through PATH rather than on the host's, and a helper
// that silently stops putting it there would turn them into tests of nothing.
func TestTheFakeSudoOnPathIsReallyUsed(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "sudo"), []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatalf("writing the fake sudo: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err := defaultSudoCredentialCached(); err != nil {
		t.Fatalf("the probe did not reach the fake sudo on PATH: %v", err)
	}
}
