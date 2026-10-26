package updates

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// The test doubles
// ---------------------------------------------------------------------------
//
// Nothing in this package's guards runs a package manager. A guard either calls a
// pure parser with a fixed answer, or drives a Center whose Runner is the fake
// below: the machine a test runs on has a real brew and a real apt, and a check
// that asked them would measure the runner's laptop instead of the code.

// fakeRunner answers a command from a table the guard writes, and records every
// command it was asked to run so a guard can assert on the argv itself.
type fakeRunner struct {
	// answers maps a command's display form to what it produces. A command with
	// no entry is a test mistake, and the fake reports it as a failed command
	// rather than panicking, so the failure names the command.
	answers map[string]Output
	// lines are handed to onLine for every command, in order.
	lines []string
	// calls are the commands the fake was asked to run, in order.
	calls []Command
	// parallel reports whether the fake ever saw two runs at once. It is written
	// under the runner's own lock, so a guard can assert that the center never
	// starts two managers together.
	parallel bool
	running  int
}

func (f *fakeRunner) Run(_ context.Context, cmd Command, onLine func(string)) Output {
	if f.running > 0 {
		f.parallel = true
	}
	f.running++
	defer func() { f.running-- }()

	f.calls = append(f.calls, cmd)
	if onLine != nil {
		for _, line := range f.lines {
			onLine(line)
		}
	}
	if answer, ok := f.answers[QuoteArgv(cmd.Argv)]; ok {
		return answer
	}
	return Output{
		ExitCode: 127,
		Err:      errors.New("the fake runner has no answer for " + QuoteArgv(cmd.Argv)),
		Stderr:   "no answer is recorded for " + QuoteArgv(cmd.Argv),
	}
}

// ok is a command that exited zero with the given output.
func ok(output string) Output { return Output{ExitCode: 0, Stdout: output} }

// failed is a command that refused with the given output on stderr.
func failed(output string) Output {
	return Output{ExitCode: 1, Err: errors.New("exit status 1"), Stderr: output}
}

// testEnv is an environment with exactly the named programs installed and nothing
// else: no PATH lookup reaches the machine the guard runs on.
//
// BrewPrefix is empty on purpose. A prefix would make Resolve stat a path on the
// machine running the tests, and the developer's own Homebrew is installed on the
// machine this package is developed on -- a guard would then measure their laptop.
// The prefix fallback has a guard of its own, against a temporary directory.
func testEnv(programs ...string) Env {
	available := map[string]bool{}
	for _, program := range programs {
		available[program] = true
	}
	return Env{
		GOOS: "linux",
		LookPath: func(name string) (string, error) {
			if available[name] {
				return "/usr/bin/" + name, nil
			}
			return "", errors.New("not found")
		},
		Getenv:        func(string) string { return "" },
		SudoAvailable: available["sudo"],
		Now:           func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) },
	}
}

// TestTheBrewPrefixFallbackFindsABrewThatIsNotOnPath pins the one resolution that
// is not a PATH lookup: a run whose own PATH predates the Homebrew install still
// has to see it, which is exactly the state the installer leaves behind.
func TestTheBrewPrefixFallbackFindsABrewThatIsNotOnPath(t *testing.T) {
	prefix := t.TempDir()
	if err := os.MkdirAll(filepath.Join(prefix, "bin"), 0o755); err != nil {
		t.Fatalf("could not build the prefix: %v", err)
	}
	if err := os.WriteFile(filepath.Join(prefix, "bin", "brew"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("could not write the fake brew: %v", err)
	}

	env := testEnv()
	env.BrewPrefix = prefix
	path, ok := env.Resolve("brew")
	if !ok {
		t.Fatal("a brew in the prefix's bin directory must be found")
	}
	if path != filepath.Join(prefix, "bin", "brew") {
		t.Fatalf("Resolve = %q, want the prefix's own brew", path)
	}

	env.BrewPrefix = t.TempDir()
	if _, ok := env.Resolve("brew"); ok {
		t.Fatal("an empty prefix directory holds no brew and must resolve to nothing")
	}
}

// ---------------------------------------------------------------------------
// Detection and inventory
// ---------------------------------------------------------------------------

// TestChannelsNameWhatIsMissingAndWhatIsUnsupported pins the two different
// absences: a manager whose program is not installed is unavailable, and one that
// does not serve this host is unsupported. Neither is an error, and neither is
// silent.
func TestChannelsNameWhatIsMissingAndWhatIsUnsupported(t *testing.T) {
	center := &Center{Adapters: DefaultAdapters(), Runner: &fakeRunner{}, Env: testEnv("apt-get", "apt-mark")}
	available := map[ManagerID]bool{}
	reasons := map[ManagerID]string{}
	for _, channel := range center.Channels() {
		available[channel.Manager] = channel.Available
		reasons[channel.Manager] = channel.Reason
	}

	if !available[ManagerApt] {
		t.Fatalf("apt-get is installed and must be available, reason %q", reasons[ManagerApt])
	}
	for _, manager := range []ManagerID{ManagerBrew, ManagerPacman, ManagerDnf, ManagerPkg, ManagerSnap, ManagerFlatpak, ManagerRustup} {
		if available[manager] {
			t.Fatalf("%s is not installed here and must not be offered as available", manager)
		}
		if reasons[manager] == "" {
			t.Fatalf("%s is not available, so a channel must carry the reason", manager)
		}
	}
}

// TestATermuxHostOffersPkgAndNotTheDistributionManagers pins the platform rule:
// on Termux the channel is pkg's, and the distribution managers are unsupported
// rather than missing.
func TestATermuxHostOffersPkgAndNotTheDistributionManagers(t *testing.T) {
	env := testEnv("apt-get", "apt-mark", "pkg", "dnf", "pacman")
	env.Termux = true
	center := &Center{Adapters: DefaultAdapters(), Runner: &fakeRunner{}, Env: env}

	byManager := map[ManagerID]Channel{}
	for _, channel := range center.Channels() {
		byManager[channel.Manager] = channel
	}
	if !byManager[ManagerPkg].Available {
		t.Fatalf("pkg is the Termux channel and must be available, reason %q", byManager[ManagerPkg].Reason)
	}
	if byManager[ManagerApt].Available || byManager[ManagerApt].Status != StatusUnsupported {
		t.Fatalf("apt on Termux must be unsupported, got available=%v status=%s", byManager[ManagerApt].Available, byManager[ManagerApt].Status)
	}
	for _, manager := range []ManagerID{ManagerPacman, ManagerDnf, ManagerSnap, ManagerFlatpak} {
		if byManager[manager].Available {
			t.Fatalf("%s must not be offered on Termux", manager)
		}
	}
}

// TestAnUnsupportedHostStillReportsTheOthers pins graceful degradation: a macOS
// host has Homebrew and no apt, and the apt channel answers with its reason
// instead of failing the whole check.
func TestAnUnsupportedHostStillReportsTheOthers(t *testing.T) {
	env := testEnv("brew")
	env.GOOS = "darwin"
	center := &Center{Adapters: DefaultAdapters(), Runner: &fakeRunner{}, Env: env}

	results := map[ManagerID]Result{}
	for _, channel := range center.Channels() {
		if !channel.Available {
			continue
		}
		results[channel.Manager] = center.Read(context.Background(), channel.Manager)
	}
	if _, ok := results[ManagerBrew]; !ok {
		t.Fatal("a macOS host must offer Homebrew")
	}
	if len(results) != 1 {
		t.Fatalf("a macOS host must offer exactly Homebrew, got %d channels", len(results))
	}
}

// ---------------------------------------------------------------------------
// Reading, through a fake manager
// ---------------------------------------------------------------------------

// TestReadKeepsTheManagersOwnVersionsAndHolds pins what one read produces: the
// entries Homebrew names, with the installed and available versions as Homebrew
// printed them, and a pinned entry present but marked.
func TestReadKeepsTheManagersOwnVersionsAndHolds(t *testing.T) {
	runner := &fakeRunner{answers: map[string]Output{
		QuoteArgv([]string{"/usr/bin/brew", "outdated", "--json=v2"}): ok(`{
			"formulae": [
				{"name": "git", "installed_versions": ["2.43.0"], "current_version": "2.44.0", "pinned": false},
				{"name": "pinned-one", "installed_versions": ["1.0"], "current_version": "1.1", "pinned": true}
			],
			"casks": [
				{"name": "alacritty", "installed_versions": ["0.13.0"], "current_version": "0.13.1", "pinned": false}
			]
		}`),
	}}
	center := &Center{Adapters: DefaultAdapters(), Runner: runner, Env: testEnv("brew")}

	result := center.Read(context.Background(), ManagerBrew)
	if result.Status != StatusUpdates {
		t.Fatalf("a read with entries must be %s, got %s (%s)", StatusUpdates, result.Status, result.Reason)
	}
	if len(result.Items) != 3 {
		t.Fatalf("three entries were reported, got %d: %+v", len(result.Items), result.Items)
	}
	items := map[string]Item{}
	for _, item := range result.Items {
		items[item.Name] = item
	}
	if items["git"].Current != "2.43.0" || items["git"].Available != "2.44.0" {
		t.Fatalf("the versions must be Homebrew's own, got %+v", items["git"])
	}
	if !items["pinned-one"].Pinned || items["pinned-one"].PinReason == "" {
		t.Fatalf("a pinned entry must be marked and carry its hold, got %+v", items["pinned-one"])
	}
	if items["alacritty"].Kind != KindCask {
		t.Fatalf("a cask must be an entry of its own kind, got %+v", items["alacritty"])
	}
	if items["git"].Scope != ScopeUser {
		t.Fatalf("a Homebrew entry is the user's, got %+v", items["git"])
	}
	// The read command is recorded for the screen's provenance line, and the
	// manager's own JSON is not:
	if !strings.Contains(result.Command, "outdated --json=v2") {
		t.Fatalf("the result must name the read command, got %q", result.Command)
	}
}

// TestReadReportsAnOfflineSourceRatherThanAnEmptyAnswer pins the distinction the
// mission asks for: a source that cannot be reached is offline, and it is not the
// same answer as "nothing to update".
func TestReadReportsAnOfflineSourceRatherThanAnEmptyAnswer(t *testing.T) {
	runner := &fakeRunner{answers: map[string]Output{
		QuoteArgv([]string{"/usr/bin/brew", "outdated", "--json=v2"}): failed("Error: Failure while executing; `git fetch` exited with 128. fatal: unable to access 'https://github.com/Homebrew/brew/': Could not resolve host: github.com\n"),
	}}
	center := &Center{Adapters: DefaultAdapters(), Runner: runner, Env: testEnv("brew")}

	result := center.Read(context.Background(), ManagerBrew)
	if result.Status != StatusOffline {
		t.Fatalf("an unreachable source must be %s, got %s (%s)", StatusOffline, result.Status, result.Reason)
	}
	if len(result.Items) != 0 {
		t.Fatalf("an offline read must offer nothing, got %+v", result.Items)
	}
	if !strings.Contains(result.Reason, "Could not resolve host") {
		t.Fatalf("the reason must carry the manager's own words, got %q", result.Reason)
	}
}

// TestReadOfAManagerThatIsNotHereIsAnAnswer pins that reading an unavailable
// channel is not an error and not a command: nothing is run.
func TestReadOfAManagerThatIsNotHereIsAnAnswer(t *testing.T) {
	runner := &fakeRunner{}
	center := &Center{Adapters: DefaultAdapters(), Runner: runner, Env: testEnv()}

	result := center.Read(context.Background(), ManagerPacman)
	if result.Status != StatusUnavailable {
		t.Fatalf("an unavailable manager must answer %s, got %s", StatusUnavailable, result.Status)
	}
	if result.Reason == "" {
		t.Fatal("the answer must carry the reason")
	}
	if len(runner.calls) != 0 {
		t.Fatalf("an unavailable manager must not be asked anything, ran %d commands", len(runner.calls))
	}
}

// TestADeepSeamNeverRunsTwoManagersAtOnce pins the lock rule: the center starts
// one command at a time, whatever a check does.
func TestADeepSeamNeverRunsTwoManagersAtOnce(t *testing.T) {
	runner := &fakeRunner{answers: map[string]Output{
		QuoteArgv([]string{"/usr/bin/brew", "outdated", "--json=v2"}):                       ok(`{"formulae":[],"casks":[]}`),
		QuoteArgv([]string{"apt-get", "--simulate", "-o", "Debug::NoLocking=1", "upgrade"}): ok("Reading package lists...\n0 upgraded, 0 newly installed, 0 to remove and 0 not upgraded.\n"),
		QuoteArgv([]string{"apt-mark", "showhold"}):                                         ok(""),
	}}
	center := &Center{Adapters: DefaultAdapters(), Runner: runner, Env: testEnv("brew", "apt-get", "apt-mark")}

	for _, channel := range center.Channels() {
		if !channel.Available {
			continue
		}
		center.Read(context.Background(), channel.Manager)
	}
	if runner.parallel {
		t.Fatal("the center started two managers at once; apt, pacman, dnf and pkg share locks")
	}
}

// ---------------------------------------------------------------------------
// Planning a selection
// ---------------------------------------------------------------------------

// TestPlansForBuildsOneCommandPerManagerAndLeavesThePinnedAlone pins the two
// halves of planning: a selection becomes the managers' own commands, and a pinned
// entry is never in one of them.
func TestPlansForBuildsOneCommandPerManagerAndLeavesThePinnedAlone(t *testing.T) {
	center := &Center{Adapters: DefaultAdapters(), Runner: &fakeRunner{}, Env: testEnv("brew")}
	items := []Item{
		{Manager: ManagerBrew, Name: "git", Scope: ScopeUser, Kind: KindFormula, Available: "2.44.0"},
		{Manager: ManagerBrew, Name: "curl", Scope: ScopeUser, Kind: KindFormula, Available: "8.7.1"},
		{Manager: ManagerBrew, Name: "alacritty", Scope: ScopeUser, Kind: KindCask, Available: "0.13.1"},
		{
			Manager: ManagerBrew, Name: "pinned-one", Scope: ScopeUser, Kind: KindFormula,
			Available: "1.1", Pinned: true, PinReason: "pinned with `brew pin`; moving it needs `brew unpin pinned-one`",
		},
	}

	plans, leftOut := center.PlansFor(items)
	if len(plans) != 2 {
		t.Fatalf("a formula selection and a cask selection need two commands, got %d", len(plans))
	}
	if len(leftOut) != 1 || leftOut[0].Item.Name != "pinned-one" {
		t.Fatalf("the pinned entry must be left out with its reason, got %+v", leftOut)
	}
	if !strings.Contains(leftOut[0].Reason, "brew pin") {
		t.Fatalf("the left-out reason must name the hold, got %q", leftOut[0].Reason)
	}

	formulaPlan := plans[0]
	if formulaPlan.Kind != KindFormula || len(formulaPlan.Items) != 2 {
		t.Fatalf("the formula plan must carry both formulae, got %+v", formulaPlan)
	}
	if got := formulaPlan.Display(); got != "/usr/bin/brew upgrade git curl" {
		t.Fatalf("the formula command = %q", got)
	}
	if formulaPlan.Elevation() {
		t.Fatal("Homebrew never needs an administrator")
	}
	caskPlan := plans[1]
	if caskPlan.Kind != KindCask {
		t.Fatalf("the cask selection must build a cask plan, got kind %s", caskPlan.Kind)
	}
	if got := caskPlan.Display(); got != "/usr/bin/brew upgrade --cask alacritty" {
		t.Fatalf("the cask command = %q", got)
	}
	for _, plan := range plans {
		if plan.Restart == "" || plan.Rollback == "" {
			t.Fatalf("every plan must state what stays pending and what going back costs: %+v", plan)
		}
	}
}

// TestASystemManagerPlanIsElevatedAndNonInteractive pins what a system update
// declares before it runs: elevation, the manager's own non-interactive flags, and
// the note that the manager resolves dependencies itself.
func TestASystemManagerPlanIsElevatedAndNonInteractive(t *testing.T) {
	center := &Center{Adapters: DefaultAdapters(), Runner: &fakeRunner{}, Env: testEnv("apt-get")}
	items := []Item{
		{Manager: ManagerApt, Name: "curl", Scope: ScopeSystem, Kind: KindPackage, Available: "8.7.1"},
		{Manager: ManagerApt, Name: "git", Scope: ScopeSystem, Kind: KindPackage, Available: "2.44.0"},
	}

	plans, leftOut := center.PlansFor(items)
	if len(leftOut) != 0 {
		t.Fatalf("nothing in this selection is pinned, got %+v", leftOut)
	}
	if len(plans) != 1 {
		t.Fatalf("one manager needs one command, got %d", len(plans))
	}
	plan := plans[0]
	if !plan.Elevation() {
		t.Fatal("a system package needs an administrator and the plan must say so")
	}
	display := plan.Display()
	for _, want := range []string{"apt-get install", "--only-upgrade", "sudo", "-n", "curl", "git"} {
		if !strings.Contains(display, want) {
			t.Fatalf("the plan's command must carry %q, got %q", want, display)
		}
	}
	if !hasEnv(plan.Command.Env, "DEBIAN_FRONTEND=noninteractive") {
		t.Fatalf("apt must be told not to prompt, env is %q", plan.Command.Env)
	}
	dpkgNote := false
	for _, note := range plan.Notes {
		if strings.Contains(note, "configuration files") {
			dpkgNote = true
			break
		}
	}
	if !dpkgNote {
		t.Fatalf("the plan must state what dpkg does with configuration files, got %q", plan.Notes)
	}
}

// TestAPlanForAManagerThatIsNotHereRefuses pins the refusal: no adapter builds a
// command for a manager it cannot see.
func TestAPlanForAManagerThatIsNotHereRefuses(t *testing.T) {
	center := &Center{Adapters: DefaultAdapters(), Runner: &fakeRunner{}, Env: testEnv()}
	plans, leftOut := center.PlansFor([]Item{{Manager: ManagerDnf, Name: "git", Scope: ScopeSystem, Kind: KindPackage}})
	if len(plans) != 0 {
		t.Fatalf("dnf is not here and must not produce a plan, got %+v", plans)
	}
	if len(leftOut) != 1 || !strings.Contains(leftOut[0].Reason, "dnf") {
		t.Fatalf("the refusal must name the manager, got %+v", leftOut)
	}
}

// TestAnEmptySelectionPlansNothing pins the rule the interface depends on: no
// selection means no command, so there is no "update everything" on this path.
func TestAnEmptySelectionPlansNothing(t *testing.T) {
	center := &Center{Adapters: DefaultAdapters(), Runner: &fakeRunner{}, Env: testEnv("brew", "apt-get")}
	plans, leftOut := center.PlansFor(nil)
	if len(plans) != 0 || len(leftOut) != 0 {
		t.Fatalf("an empty selection builds nothing, got %d plans and %d left out", len(plans), len(leftOut))
	}
}

// ---------------------------------------------------------------------------
// Applying, and what the manager says afterwards
// ---------------------------------------------------------------------------

// brewPlanFor is one Homebrew plan over the named entries, built by the adapter so
// a guard cannot drift from the adapter's own argv.
func brewPlanFor(t *testing.T, env Env, items ...Item) Plan {
	t.Helper()
	plans, leftOut := (&Center{Adapters: DefaultAdapters(), Env: env}).PlansFor(items)
	if len(leftOut) != 0 || len(plans) != 1 {
		t.Fatalf("the guard's selection must build exactly one plan, got %d plans and %+v", len(plans), leftOut)
	}
	return plans[0]
}

// TestApplyConfirmsWhatTheManagerNoLongerReports pins the distinction the mission
// names: a command that exited zero is executed, and only the manager's fresh
// answer confirms it.
func TestApplyConfirmsWhatTheManagerNoLongerReports(t *testing.T) {
	item := Item{Manager: ManagerBrew, Name: "git", Scope: ScopeUser, Kind: KindFormula, Current: "2.43.0", Available: "2.44.0"}
	env := testEnv("brew")
	plan := brewPlanFor(t, env, item)

	runner := &fakeRunner{answers: map[string]Output{
		QuoteArgv(plan.Command.Argv): ok("==> Upgrading git\n"),
		// The re-read no longer lists git: the manager says it moved.
		QuoteArgv([]string{"/usr/bin/brew", "outdated", "--json=v2"}): ok(`{"formulae":[],"casks":[]}`),
	}}
	center := &Center{Adapters: DefaultAdapters(), Runner: runner, Env: env}

	result := center.Apply(context.Background(), plan, nil)
	if len(result.Outcomes) != 1 {
		t.Fatalf("one entry was planned and must have one outcome, got %+v", result.Outcomes)
	}
	if result.Outcomes[0].Step != StepConfirmed {
		t.Fatalf("the manager no longer reports the entry, so it is confirmed, got %s (%s)",
			result.Outcomes[0].Step, result.Outcomes[0].Detail)
	}
	if result.Confirmed() != 1 || result.Failed() != 0 {
		t.Fatalf("the counts must agree with the outcomes: confirmed=%d failed=%d", result.Confirmed(), result.Failed())
	}
	if result.Command == "" {
		t.Fatal("the result must name the command it ran")
	}
}

// TestApplyReportsAnEntryTheManagerStillReportsAsOutOfDate pins the honest middle
// state: the command succeeded and the manager still lists the entry, so it was
// executed and it is not confirmed.
func TestApplyReportsAnEntryTheManagerStillReportsAsOutOfDate(t *testing.T) {
	item := Item{Manager: ManagerBrew, Name: "git", Scope: ScopeUser, Kind: KindFormula, Current: "2.43.0", Available: "2.44.0"}
	env := testEnv("brew")
	plan := brewPlanFor(t, env, item)

	runner := &fakeRunner{answers: map[string]Output{
		QuoteArgv(plan.Command.Argv):                                  ok("==> Upgrading git\n"),
		QuoteArgv([]string{"/usr/bin/brew", "outdated", "--json=v2"}): ok(`{"formulae":[{"name":"git","installed_versions":["2.43.0"],"current_version":"2.44.0"}],"casks":[]}`),
	}}
	center := &Center{Adapters: DefaultAdapters(), Runner: runner, Env: env}

	result := center.Apply(context.Background(), plan, nil)
	if result.Outcomes[0].Step != StepExecuted {
		t.Fatalf("an entry the manager still lists is executed and not confirmed, got %s (%s)",
			result.Outcomes[0].Step, result.Outcomes[0].Detail)
	}
	if result.Confirmed() != 0 {
		t.Fatalf("nothing may be confirmed when the manager still lists it, got %d", result.Confirmed())
	}
}

// TestApplyReportsAPartialRunPerEntry pins partial failure: one entry of a plan is
// confirmed and the other is still out of date, so the run reports both, one at a
// time.
func TestApplyReportsAPartialRunPerEntry(t *testing.T) {
	first := Item{Manager: ManagerBrew, Name: "curl", Scope: ScopeUser, Kind: KindFormula, Current: "8.7.0", Available: "8.7.1"}
	second := Item{Manager: ManagerBrew, Name: "git", Scope: ScopeUser, Kind: KindFormula, Current: "2.43.0", Available: "2.44.0"}
	env := testEnv("brew")
	plan := brewPlanFor(t, env, first, second)

	runner := &fakeRunner{answers: map[string]Output{
		// The command refused, and it refused after moving curl: apt, brew and dnf
		// all do that when a single package of the selection fails.
		QuoteArgv(plan.Command.Argv):                                  failed("Error: git: the bottle could not be poured\n"),
		QuoteArgv([]string{"/usr/bin/brew", "outdated", "--json=v2"}): ok(`{"formulae":[{"name":"git","installed_versions":["2.43.0"],"current_version":"2.44.0"}],"casks":[]}`),
	}}
	center := &Center{Adapters: DefaultAdapters(), Runner: runner, Env: env}

	result := center.Apply(context.Background(), plan, nil)
	if result.Status != StatusFailed {
		t.Fatalf("a refused command is a failure, got %s", result.Status)
	}
	steps := map[string]OutcomeStep{}
	for _, outcome := range result.Outcomes {
		steps[outcome.Item.Name] = outcome.Step
	}
	if steps["curl"] != StepConfirmed {
		t.Fatalf("an entry the manager no longer reports is confirmed even when the command failed, got %s", steps["curl"])
	}
	if steps["git"] != StepFailed {
		t.Fatalf("an entry the manager still reports after a failure failed, got %s", steps["git"])
	}
	if result.Moved() != 1 || result.Failed() != 1 {
		t.Fatalf("the counts must report the partial run: moved=%d failed=%d", result.Moved(), result.Failed())
	}
	if !strings.Contains(result.Output, "the bottle could not be poured") {
		t.Fatalf("the log must keep the manager's output, got %q", result.Output)
	}
}

// TestApplyReportsACancelledRunAsUnknown pins cancellation: an entry that is still
// out of date after a stopped run is unknown, not failed, because the command was
// not given the chance to move it.
func TestApplyReportsACancelledRunAsUnknown(t *testing.T) {
	item := Item{Manager: ManagerApt, Name: "curl", Scope: ScopeSystem, Kind: KindPackage, Current: "8.7.0", Available: "8.7.1"}
	env := testEnv("apt-get", "apt-mark")
	center := &Center{Adapters: DefaultAdapters(), Env: env}
	plans, _ := center.PlansFor([]Item{item})
	plan := plans[0]

	runner := &fakeRunner{answers: map[string]Output{
		QuoteArgv(plan.Command.Argv):             {ExitCode: -1, Err: errors.New("signal: interrupt"), Cancelled: true},
		QuoteArgv([]string{"apt-get", "update"}): ok("Hit:1 http://deb.debian.org/debian stable InRelease\n"),
		QuoteArgv([]string{"apt-get", "--simulate", "-o", "Debug::NoLocking=1", "upgrade"}): ok("Inst curl [8.7.0] (8.7.1 Debian:13/stable [amd64])\n"),
		QuoteArgv([]string{"apt-mark", "showhold"}):                                         ok(""),
	}}
	center.Runner = runner

	result := center.Apply(context.Background(), plan, nil)
	if result.Status != StatusCancelled {
		t.Fatalf("the run was stopped, so its status is %s, got %s", StatusCancelled, result.Status)
	}
	if result.Outcomes[0].Step != StepUnknown {
		t.Fatalf("an interrupted entry is unknown, got %s (%s)", result.Outcomes[0].Step, result.Outcomes[0].Detail)
	}
	if len(runner.calls) != 4 {
		t.Fatalf("the re-read must happen even after a cancelled run, ran %d commands", len(runner.calls))
	}
}

// TestApplySaysSoWhenTheManagerCannotBeReadAgain pins the limit of confirmation:
// a re-read that failed leaves the entry executed and the reason named, never
// confirmed.
func TestApplySaysSoWhenTheManagerCannotBeReadAgain(t *testing.T) {
	item := Item{Manager: ManagerBrew, Name: "git", Scope: ScopeUser, Kind: KindFormula, Current: "2.43.0", Available: "2.44.0"}
	env := testEnv("brew")
	plan := brewPlanFor(t, env, item)

	runner := &fakeRunner{answers: map[string]Output{
		QuoteArgv(plan.Command.Argv):                                  ok("==> Upgrading git\n"),
		QuoteArgv([]string{"/usr/bin/brew", "outdated", "--json=v2"}): failed("Error: Could not resolve host: github.com\n"),
	}}
	center := &Center{Adapters: DefaultAdapters(), Runner: runner, Env: env}

	result := center.Apply(context.Background(), plan, nil)
	if result.Outcomes[0].Step != StepExecuted {
		t.Fatalf("without a fresh answer nothing may be confirmed, got %s (%s)",
			result.Outcomes[0].Step, result.Outcomes[0].Detail)
	}
	if !strings.Contains(result.Outcomes[0].Detail, "could not be read again") {
		t.Fatalf("the outcome must say why it could not be confirmed, got %q", result.Outcomes[0].Detail)
	}
}

// TestApplyHandsEveryLineToTheScreenAndRedactsThem pins the log path: the
// interface gets the command's lines as they arrive, and a credential in one of
// them is already gone when it gets there.
func TestApplyHandsEveryLineToTheScreenAndRedactsThem(t *testing.T) {
	item := Item{Manager: ManagerBrew, Name: "git", Scope: ScopeUser, Kind: KindFormula, Current: "2.43.0", Available: "2.44.0"}
	env := testEnv("brew")
	plan := brewPlanFor(t, env, item)

	runner := &fakeRunner{
		lines: []string{"==> Upgrading git", "Fetching https://user:s3cr3t@example.com/git.tgz"},
		answers: map[string]Output{
			QuoteArgv(plan.Command.Argv):                                  ok("==> Upgrading git\n"),
			QuoteArgv([]string{"/usr/bin/brew", "outdated", "--json=v2"}): ok(`{"formulae":[],"casks":[]}`),
		},
	}
	center := &Center{Adapters: DefaultAdapters(), Runner: runner, Env: env}

	var seen []string
	result := center.Apply(context.Background(), plan, func(line string) { seen = append(seen, line) })
	if len(seen) != 2 {
		t.Fatalf("every line must reach the screen, got %q", seen)
	}
	if strings.Contains(strings.Join(seen, "\n"), "s3cr3t") {
		t.Fatalf("a credential must not reach the screen: %q", seen)
	}
	if result.Output == "" {
		t.Fatal("the result must keep the command's output")
	}
}

// TestAPlanCannotMoveTwoScopesOfOneManager pins the Flatpak rule inside the
// planner: a manager that names the installation per command gets one plan per
// installation.
func TestAPlanCannotMoveTwoScopesOfOneManager(t *testing.T) {
	env := testEnv("flatpak")
	center := &Center{Adapters: DefaultAdapters(), Runner: &fakeRunner{}, Env: env}
	plans, leftOut := center.PlansFor([]Item{
		{Manager: ManagerFlatpak, Name: "org.example.User", Scope: ScopeUser, Kind: KindFlatpak, Available: "2"},
		{Manager: ManagerFlatpak, Name: "org.example.System", Scope: ScopeSystem, Kind: KindFlatpak, Available: "2"},
	})
	if len(leftOut) != 0 {
		t.Fatalf("nothing here is pinned, got %+v", leftOut)
	}
	if len(plans) != 2 {
		t.Fatalf("one installation per command means two plans, got %d", len(plans))
	}
	user, system := plans[0], plans[1]
	if user.Elevation() || !strings.Contains(user.Display(), "--user") {
		t.Fatalf("the user installation needs no administrator, got %q (elevated=%v)", user.Display(), user.Elevation())
	}
	if !system.Elevation() || !strings.Contains(system.Display(), "--system") {
		t.Fatalf("the system installation needs an administrator, got %q (elevated=%v)", system.Display(), system.Elevation())
	}
}

// hasEnv reports whether an environment carries an exact entry.
func hasEnv(env []string, entry string) bool {
	for _, candidate := range env {
		if candidate == entry {
			return true
		}
	}
	return false
}

// describeChannels is a debugging aid for a guard that fails on a channel list.
func describeChannels(channels []Channel) string {
	var lines []string
	for _, channel := range channels {
		lines = append(lines, fmt.Sprintf("%s available=%v status=%s reason=%s", channel.Manager, channel.Available, channel.Status, channel.Reason))
	}
	return strings.Join(lines, "\n")
}
