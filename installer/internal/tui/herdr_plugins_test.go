package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// herdrPluginBoundaryStubbed records that this test binary replaced the Herdr
// plugin CLI boundary with a local stub. TestHerdrPluginBoundaryIsStubbedIn-
// ATestBinary fails if the init below is ever removed.
var herdrPluginBoundaryStubbed bool

// init points the Herdr plugin step's CLI boundary at a local stub for the whole
// package, once, before any test runs -- the same shape as the update-check
// disarm in update_check_test.go.
//
// It is the one thing a guard cannot do for itself: stepInstallWM runs in this
// package's tests, and on a developer's machine the real herdr binary is on
// PATH, so a test that installs the multiplexer would otherwise run
// `herdr plugin install` for real -- reaching GitHub and writing into the real
// ~/.config/herdr. Tests that exercise the step replace this stub with a
// recording one; tests that only pass through it (the Herdr config-copy test)
// must still see a verified install and no error.
func init() {
	herdrPluginBoundaryStubbed = true

	herdrPluginRun = func(command string, opts *system.ExecOptions, onLog system.LogCallback) *system.ExecResult {
		return &system.ExecResult{Command: command}
	}
	herdrPluginList = func() (string, error) {
		var doc herdrPluginListDocument
		for i, plugin := range herdrPluginSet {
			found := installedHerdrPlugin{PluginID: plugin.ID, Name: plugin.Name, Enabled: true}
			found.Source.ResolvedCommit = fmt.Sprintf("%040x", i+1)
			doc.Result.Plugins = append(doc.Result.Plugins, found)
		}
		data, err := json.Marshal(doc)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
}

// TestHerdrPluginBoundaryIsStubbedInATestBinary is the guard for that init: if
// it is removed, a test in this package could install plugins into the real
// ~/.config/herdr and reach GitHub.
func TestHerdrPluginBoundaryIsStubbedInATestBinary(t *testing.T) {
	if !herdrPluginBoundaryStubbed {
		t.Fatal("the Herdr plugin CLI boundary is not stubbed in this test binary; a test could reach a real herdr and the network")
	}
}

// --- helpers ---------------------------------------------------------------

// declaredHerdrPluginIDs lists the plugins the step declares, so a test names
// the same set the step installs instead of a copy that can drift.
func declaredHerdrPluginIDs() []string {
	ids := make([]string, 0, len(herdrPluginSet))
	for _, plugin := range herdrPluginSet {
		ids = append(ids, plugin.ID)
	}
	return ids
}

// herdrListWith renders a `herdr plugin list --json` answer in the real
// envelope, so the step parses the answer the machine would actually give.
func herdrListWith(ids ...string) string {
	var doc herdrPluginListDocument
	for i, id := range ids {
		plugin := installedHerdrPlugin{PluginID: id, Enabled: true}
		plugin.Source.ResolvedCommit = fmt.Sprintf("%040x", i+1)
		doc.Result.Plugins = append(doc.Result.Plugins, plugin)
	}
	data, err := json.Marshal(doc)
	if err != nil {
		panic(err)
	}
	return string(data)
}

type herdrPluginCalls struct {
	installs  []string
	listCalls int
}

// stubHerdrPluginCLI replaces both halves of the CLI boundary for one test:
// installs are recorded (returning installErr when asked to fail) and the list
// answer is served from memory. Nothing here reaches a real herdr.
func stubHerdrPluginCLI(t *testing.T, listOutput string, listErr, installErr error) *herdrPluginCalls {
	t.Helper()

	calls := &herdrPluginCalls{}
	originalRun := herdrPluginRun
	originalList := herdrPluginList

	herdrPluginRun = func(command string, opts *system.ExecOptions, onLog system.LogCallback) *system.ExecResult {
		calls.installs = append(calls.installs, command)
		return &system.ExecResult{Command: command, Error: installErr}
	}
	herdrPluginList = func() (string, error) {
		calls.listCalls++
		return listOutput, listErr
	}

	t.Cleanup(func() {
		herdrPluginRun = originalRun
		herdrPluginList = originalList
	})
	return calls
}

// herdrPluginModel is a Herdr multiplexer selection with a checkout that ships
// the config the step copies. withHerdrOnPath controls whether the herdr binary
// resolves: when it does not, the step must touch nothing.
func herdrPluginModel(t *testing.T, withHerdrOnPath bool) Model {
	t.Helper()
	t.Setenv("HOME", t.TempDir())

	if withHerdrOnPath {
		binDir := t.TempDir()
		writeStubCommand(t, binDir, "herdr", "#!/bin/sh\nexit 0\n")
		t.Setenv("PATH", strings.Join([]string{binDir, "/usr/bin", "/bin"}, string(os.PathListSeparator)))
	} else {
		t.Setenv("PATH", t.TempDir())
	}

	m := NewModel()
	m.SystemInfo = &system.SystemInfo{OS: system.OSMac, HasBrew: true}
	m.Choices = UserChoices{OS: "mac", Shell: "zsh", WindowMgr: "herdr"}
	m.RepoDir = repoRoot(t)
	return m
}

// --- the step installs exactly the declared set ----------------------------

// TestStepInstallWMHerdrInstallsTheDeclaredHerdrPlugins is the positive
// contract: on the Herdr choice, with the binary present, the step runs
// `herdr plugin install <owner>/<repo> --yes` for every declared plugin and
// verifies each against the machine. --yes is the noninteractive form, so the
// step never blocks on the CLI's trust preview.
func TestStepInstallWMHerdrInstallsTheDeclaredHerdrPlugins(t *testing.T) {
	m := herdrPluginModel(t, true)
	calls := stubHerdrPluginCLI(t, herdrListWith(declaredHerdrPluginIDs()...), nil, nil)

	if err := stepInstallWM(&m); err != nil {
		t.Fatalf("the Herdr step must succeed when every declared plugin verifies: %v", err)
	}

	want := make([]string, 0, len(herdrPluginSet))
	for _, plugin := range herdrPluginSet {
		want = append(want, "herdr plugin install "+plugin.shorthand()+" --yes")
	}
	if !slices.Equal(calls.installs, want) {
		t.Errorf("install commands = %v, want exactly the declared set %v", calls.installs, want)
	}
	if calls.listCalls == 0 {
		t.Error("the step never asked the machine what it has, so it assumed instead of verifying")
	}
}

// TestStepInstallWMHerdrRerunReinstallsToUpdate documents the only update path
// Herdr 0.9.3 has. Its plugin surface has no update, upgrade or refresh
// command, and `herdr plugin install` replaces the managed checkout it owns, so
// re-running the installer re-resolves the default branch and lands the latest.
// If this ever stops re-running the install, the documented claim that a
// reinstall updates the plugins becomes false.
func TestStepInstallWMHerdrRerunReinstallsToUpdate(t *testing.T) {
	m := herdrPluginModel(t, true)
	calls := stubHerdrPluginCLI(t, herdrListWith(declaredHerdrPluginIDs()...), nil, nil)

	for run := 1; run <= 2; run++ {
		if err := stepInstallWM(&m); err != nil {
			t.Fatalf("run %d failed: %v", run, err)
		}
	}

	if got, want := len(calls.installs), 2*len(herdrPluginSet); got != want {
		t.Errorf("installs across two runs = %d (%v), want %d: a re-run must reinstall the declared set so the managed checkout is replaced with the latest", got, calls.installs, want)
	}
}

// TestStepInstallWMHerdrNeverTouchesUndeclaredPlugins is the boundary that
// protects the user's own plugins: the step only ever installs the declared set
// and never uninstalls, unlinks or disables anything, so a plugin the installer
// does not know about survives untouched. Removing the declared-set loop and
// replacing it with a "clean up everything" pass would fail this.
func TestStepInstallWMHerdrNeverTouchesUndeclaredPlugins(t *testing.T) {
	m := herdrPluginModel(t, true)
	machine := append([]string{"someone.else"}, declaredHerdrPluginIDs()...)
	calls := stubHerdrPluginCLI(t, herdrListWith(machine...), nil, nil)

	if err := stepInstallWM(&m); err != nil {
		t.Fatalf("a machine with an extra plugin must still succeed: %v", err)
	}

	for _, command := range calls.installs {
		if strings.Contains(command, "someone.else") {
			t.Errorf("the installer touched a plugin it does not declare: %q", command)
		}
		for _, verb := range []string{"uninstall", "unlink", "disable"} {
			if strings.Contains(command, verb) {
				t.Errorf("the plugin step must never remove or disable a plugin, but ran %q", command)
			}
		}
	}
}

// --- failures are never reported as installed ------------------------------

// TestStepInstallWMHerdrPluginVerificationFailureIsNotSuccess removes the
// verify-not-assume behaviour: the install command reports success but the
// machine never registers herdr.auto-title. The step must fail, because the
// install's own exit status is not proof. Without the read-back this test sees a
// successful step over a plugin that is not there.
func TestStepInstallWMHerdrPluginVerificationFailureIsNotSuccess(t *testing.T) {
	m := herdrPluginModel(t, true)
	// aigora.lantern is present; herdr.auto-title is missing from the answer.
	stubHerdrPluginCLI(t, herdrListWith("aigora.lantern"), nil, nil)

	err := stepInstallWM(&m)
	if err == nil {
		t.Fatal("a plugin missing from `herdr plugin list` was reported as installed")
	}
	if !strings.Contains(err.Error(), "herdr.auto-title") {
		t.Errorf("the failure does not name the plugin the machine did not report: %v", err)
	}
}

// TestStepInstallWMHerdrPluginBuildFailureIsNotSuccess covers herdr.auto-title's
// install-time build: the CLI exits non-zero when a manifest build command
// fails, and a failed build must not be reported as an installed plugin.
func TestStepInstallWMHerdrPluginBuildFailureIsNotSuccess(t *testing.T) {
	m := herdrPluginModel(t, true)
	stubHerdrPluginCLI(t, herdrListWith("aigora.lantern"), nil, fmt.Errorf("error: plugin build failed"))

	err := stepInstallWM(&m)
	if err == nil {
		t.Fatal("a plugin whose build failed was reported as installed")
	}
	if !strings.Contains(err.Error(), "failed to install") {
		t.Errorf("the failure does not name the install: %v", err)
	}
}

// TestStepInstallWMHerdrPluginListFailureIsNotSuccess covers the read-back
// itself failing: "no answer" must not look like "nothing installed" or, worse,
// like success.
func TestStepInstallWMHerdrPluginListFailureIsNotSuccess(t *testing.T) {
	m := herdrPluginModel(t, true)
	stubHerdrPluginCLI(t, "", fmt.Errorf("no Herdr server is running"), nil)

	err := stepInstallWM(&m)
	if err == nil {
		t.Fatal("a plugin list that could not be read was reported as a successful install")
	}
	if !strings.Contains(err.Error(), "read back") {
		t.Errorf("the failure does not name the verification read-back: %v", err)
	}
}

// --- only the Herdr choice, only where herdr resolves ----------------------

// TestHerdrPluginStepDoesNothingWithoutTheHerdrBinary holds constraint 4: a
// machine where `herdr` does not resolve in PATH gets no plugins and no error.
func TestHerdrPluginStepDoesNothingWithoutTheHerdrBinary(t *testing.T) {
	m := herdrPluginModel(t, false)
	calls := stubHerdrPluginCLI(t, herdrListWith(declaredHerdrPluginIDs()...), nil, nil)

	if err := installHerdrPlugins(&m, "wm"); err != nil {
		t.Fatalf("a machine without herdr must not fail the step: %v", err)
	}
	if len(calls.installs) != 0 || calls.listCalls != 0 {
		t.Errorf("a machine without herdr must get nothing: installs=%v listCalls=%d", calls.installs, calls.listCalls)
	}
}

// TestHerdrPluginStepDoesNothingForAnotherMultiplexer holds the other half of
// constraint 4: only the Herdr choice provisions Herdr plugins.
func TestHerdrPluginStepDoesNothingForAnotherMultiplexer(t *testing.T) {
	m := herdrPluginModel(t, true)
	calls := stubHerdrPluginCLI(t, herdrListWith(declaredHerdrPluginIDs()...), nil, nil)

	for _, choice := range []string{"tmux", "zellij", "none"} {
		m.Choices.WindowMgr = choice
		if err := installHerdrPlugins(&m, "wm"); err != nil {
			t.Fatalf("choice %q must not run the plugin step: %v", choice, err)
		}
	}
	if len(calls.installs) != 0 || calls.listCalls != 0 {
		t.Errorf("a non-Herdr multiplexer must get no Herdr plugins: installs=%v listCalls=%d", calls.installs, calls.listCalls)
	}
}

// --- the parser is what makes verification possible ------------------------

func TestParseInstalledHerdrPlugins(t *testing.T) {
	installed, err := parseInstalledHerdrPlugins(herdrListWith("a.b", "c.d"))
	if err != nil {
		t.Fatalf("a well-formed answer must parse: %v", err)
	}
	if len(installed) != 2 {
		t.Fatalf("parsed %d plugins, want 2", len(installed))
	}
	if got, ok := installed["a.b"]; !ok || got.Source.ResolvedCommit == "" {
		t.Errorf("plugin a.b = %+v, want it present with the commit Herdr recorded", got)
	}

	if _, err := parseInstalledHerdrPlugins("{not json"); err == nil {
		t.Error("an unreadable answer must be an error, never an empty list")
	}
}
