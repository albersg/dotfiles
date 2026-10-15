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

// herdrPluginStubVersion is the version the fully-stubbed boundary reports. It
// is deliberately not a released version: a test that reads it knows the seam is
// stubbed, and no test can mistake it for a fact about the machine.
const herdrPluginStubVersion = "herdr 0.0.0-test"

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
	herdrPluginVersion = func() (string, error) {
		return herdrPluginStubVersion, nil
	}
}

// TestHerdrPluginBoundaryIsStubbedInATestBinary is the guard for that init: if
// it is removed, a test in this package could install plugins into the real
// ~/.config/herdr and reach GitHub.
func TestHerdrPluginBoundaryIsStubbedInATestBinary(t *testing.T) {
	if !herdrPluginBoundaryStubbed {
		t.Fatal("the Herdr plugin CLI boundary is not stubbed in this test binary; a test could reach a real herdr and the network")
	}
	// A test binary that lost the init would read the real `herdr --version`,
	// which is exactly the reach into the machine this guard exists to stop.
	version, err := herdrPluginVersion()
	if err != nil || version != herdrPluginStubVersion {
		t.Fatalf("the Herdr version boundary is not stubbed in this test binary: got (%q, %v); a test could run a real `herdr --version`", version, err)
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

// herdrPluginStub describes the machine a Herdr plugin test pretends to run on.
// Nothing here reaches a real herdr or the network: the version output, the
// registry answer and the install error are all supplied by the test.
type herdrPluginStub struct {
	// version is the raw `herdr --version` output, e.g. "herdr 0.9.3".
	version string
	// versionErr makes the version read fail, for a machine whose herdr cannot
	// report one.
	versionErr error
	// list is the raw `herdr plugin list --json` answer.
	list string
	// listErr makes the registry read fail.
	listErr error
	// installErr makes an install command fail, the way a failed build does.
	installErr error
	// updateErr makes an update command fail on a CLI that has it.
	updateErr error
}

type herdrPluginCalls struct {
	// commands is every command the step ran through herdrPluginRun, in order.
	commands []string
	// installs is the subset that are `herdr plugin install`, which the tests
	// that predate the update command assert on.
	installs []string
	// updates is the subset that are `herdr plugin update`.
	updates   []string
	listCalls int
}

// stubHerdrPluginCLI replaces every half of the CLI boundary for one test: the
// version answer, installs and the list answer are served from memory. An update
// command fails when the stubbed version predates it, the way the real 0.9.3 CLI
// does, so a step that runs update unconditionally fails its guard instead of
// passing on a machine that cannot update.
func stubHerdrPluginCLI(t *testing.T, stub herdrPluginStub) *herdrPluginCalls {
	t.Helper()

	calls := &herdrPluginCalls{}
	originalRun := herdrPluginRun
	originalList := herdrPluginList
	originalVersion := herdrPluginVersion

	herdrPluginRun = func(command string, opts *system.ExecOptions, onLog system.LogCallback) *system.ExecResult {
		calls.commands = append(calls.commands, command)
		switch {
		case strings.HasPrefix(command, "herdr plugin install "):
			calls.installs = append(calls.installs, command)
			return &system.ExecResult{Command: command, Error: stub.installErr}
		case strings.HasPrefix(command, "herdr plugin update"):
			calls.updates = append(calls.updates, command)
			if stub.versionErr == nil && !herdrVersionAtLeast(parseHerdrVersion(stub.version), herdrPluginUpdateMinVersion) {
				return &system.ExecResult{Command: command, Error: fmt.Errorf("error: unrecognized subcommand 'update'")}
			}
			return &system.ExecResult{Command: command, Error: stub.updateErr}
		}
		return &system.ExecResult{Command: command}
	}
	herdrPluginList = func() (string, error) {
		calls.listCalls++
		return stub.list, stub.listErr
	}
	herdrPluginVersion = func() (string, error) {
		if stub.versionErr != nil {
			return "", stub.versionErr
		}
		return stub.version, nil
	}

	t.Cleanup(func() {
		herdrPluginRun = originalRun
		herdrPluginList = originalList
		herdrPluginVersion = originalVersion
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
	calls := stubHerdrPluginCLI(t, herdrPluginStub{
		version: "herdr 0.9.5",
		list:    herdrListWith(declaredHerdrPluginIDs()...),
	})

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

// TestStepInstallWMHerdrRerunReinstallsAndUpdates holds the owner's request:
// the plugins are installed on every run, and updated as well when the CLI can.
// The install is still what provisions a plugin first time and what replaces
// Herdr's managed checkout, so it must not be skipped in favour of the update.
func TestStepInstallWMHerdrRerunReinstallsAndUpdates(t *testing.T) {
	m := herdrPluginModel(t, true)
	calls := stubHerdrPluginCLI(t, herdrPluginStub{
		version: "herdr 0.9.5",
		list:    herdrListWith(declaredHerdrPluginIDs()...),
	})

	for run := 1; run <= 2; run++ {
		if err := stepInstallWM(&m); err != nil {
			t.Fatalf("run %d failed: %v", run, err)
		}
	}

	if got, want := len(calls.installs), 2*len(herdrPluginSet); got != want {
		t.Errorf("installs across two runs = %d (%v), want %d: a re-run must reinstall the declared set on every run", got, calls.installs, want)
	}
	if got, want := len(calls.updates), 2; got != want {
		t.Errorf("updates across two runs = %d (%v), want %d: a capable CLI is updated on every run", got, calls.updates, want)
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
	calls := stubHerdrPluginCLI(t, herdrPluginStub{
		version: "herdr 0.9.5",
		list:    herdrListWith(machine...),
	})

	if err := stepInstallWM(&m); err != nil {
		t.Fatalf("a machine with an extra plugin must still succeed: %v", err)
	}

	for _, command := range calls.commands {
		if strings.Contains(command, "someone.else") {
			t.Errorf("the installer named a plugin it does not declare: %q", command)
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
	stubHerdrPluginCLI(t, herdrPluginStub{
		version: "herdr 0.9.5",
		list:    herdrListWith("aigora.lantern"),
	})

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
	stubHerdrPluginCLI(t, herdrPluginStub{
		version:    "herdr 0.9.5",
		list:       herdrListWith("aigora.lantern"),
		installErr: fmt.Errorf("error: plugin build failed"),
	})

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
	stubHerdrPluginCLI(t, herdrPluginStub{
		version: "herdr 0.9.5",
		listErr: fmt.Errorf("no Herdr server is running"),
	})

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
	calls := stubHerdrPluginCLI(t, herdrPluginStub{
		version: "herdr 0.9.5",
		list:    herdrListWith(declaredHerdrPluginIDs()...),
	})

	if err := installHerdrPlugins(&m, "wm"); err != nil {
		t.Fatalf("a machine without herdr must not fail the step: %v", err)
	}
	if len(calls.commands) != 0 || calls.listCalls != 0 {
		t.Errorf("a machine without herdr must get nothing: commands=%v listCalls=%d", calls.commands, calls.listCalls)
	}
}

// TestHerdrPluginStepDoesNothingForAnotherMultiplexer holds the other half of
// constraint 4: only the Herdr choice provisions Herdr plugins.
func TestHerdrPluginStepDoesNothingForAnotherMultiplexer(t *testing.T) {
	m := herdrPluginModel(t, true)
	calls := stubHerdrPluginCLI(t, herdrPluginStub{
		version: "herdr 0.9.5",
		list:    herdrListWith(declaredHerdrPluginIDs()...),
	})

	for _, choice := range []string{"tmux", "zellij", "none"} {
		m.Choices.WindowMgr = choice
		if err := installHerdrPlugins(&m, "wm"); err != nil {
			t.Fatalf("choice %q must not run the plugin step: %v", choice, err)
		}
	}
	if len(calls.commands) != 0 || calls.listCalls != 0 {
		t.Errorf("a non-Herdr multiplexer must get no Herdr plugins: commands=%v listCalls=%d", calls.commands, calls.listCalls)
	}
}

// --- the version guard ------------------------------------------------------

// TestStepInstallWMHerdrUpdatesWhenTheCLICan is the owner's request on today's
// Herdr: the declared set is installed, and then updated with the CLI's own
// `plugin update`. 0.9.4's `--help` reads
// `herdr plugin update [OPTIONS] [PLUGIN]...` and the noninteractive form is
// `--yes`, so the step runs `herdr plugin update --yes`.
func TestStepInstallWMHerdrUpdatesWhenTheCLICan(t *testing.T) {
	m := herdrPluginModel(t, true)
	calls := stubHerdrPluginCLI(t, herdrPluginStub{
		version: "herdr 0.9.5",
		list:    herdrListWith(declaredHerdrPluginIDs()...),
	})
	lines := captureHerdrLog(t)

	if err := stepInstallWM(&m); err != nil {
		t.Fatalf("the Herdr step must succeed when the CLI can update: %v", err)
	}
	if want := []string{"herdr plugin update --yes"}; !slices.Equal(calls.updates, want) {
		t.Errorf("update commands = %v, want exactly %v: a capable CLI must be updated after the install", calls.updates, want)
	}
	if got, want := len(calls.installs), len(herdrPluginSet); got != want {
		t.Errorf("installs = %d (%v), want %d: the set is still installed on every run", got, calls.installs, want)
	}
	if !herdrLogContains(*lines, "Herdr 0.9.5") {
		t.Errorf("the step does not name the Herdr version it found; logs = %v", *lines)
	}
}

// TestStepInstallWMHerdrWithoutPluginUpdateSucceedsAndSaysSo is the trap guard.
// `herdr plugin update` exits non-zero on a CLI that has no such subcommand, so
// a step that runs it unconditionally fails on exactly the machines that have
// been running this installer longest. On 0.9.3 the step must succeed, name the
// version it found, install the set, and say plainly that they were not updated
// and what to do about it.
func TestStepInstallWMHerdrWithoutPluginUpdateSucceedsAndSaysSo(t *testing.T) {
	m := herdrPluginModel(t, true)
	calls := stubHerdrPluginCLI(t, herdrPluginStub{
		version: "herdr 0.9.3",
		list:    herdrListWith(declaredHerdrPluginIDs()...),
	})
	lines := captureHerdrLog(t)

	if err := stepInstallWM(&m); err != nil {
		t.Fatalf("a Herdr older than 0.9.4 has no `plugin update`; the step must succeed anyway: %v", err)
	}
	if len(calls.updates) != 0 {
		t.Errorf("the step ran `plugin update` on a CLI that does not have it: %v", calls.updates)
	}
	if got, want := len(calls.installs), len(herdrPluginSet); got != want {
		t.Errorf("installs = %d (%v), want %d: the plugins are installed on every run, update or not", got, calls.installs, want)
	}
	report := strings.Join(*lines, "\n")
	if !strings.Contains(report, "Herdr 0.9.3") {
		t.Errorf("the step does not name the version of Herdr it found:\n%s", report)
	}
	if !strings.Contains(report, "not updated") {
		t.Errorf("the step does not say the plugins were installed but not updated:\n%s", report)
	}
	if !strings.Contains(report, herdrPluginUpdateMinVersion) {
		t.Errorf("the step does not name the Herdr version that introduced `plugin update`:\n%s", report)
	}
}

// TestStepInstallWMHerdrUnreadableVersionStillInstalls covers a version that
// cannot be read at all: the step cannot prove the CLI can update, so it must
// not run the update, and it must still install and succeed.
func TestStepInstallWMHerdrUnreadableVersionStillInstalls(t *testing.T) {
	m := herdrPluginModel(t, true)
	calls := stubHerdrPluginCLI(t, herdrPluginStub{
		versionErr: fmt.Errorf("herdr: command not found"),
		list:       herdrListWith(declaredHerdrPluginIDs()...),
	})
	lines := captureHerdrLog(t)

	if err := stepInstallWM(&m); err != nil {
		t.Fatalf("a Herdr whose version cannot be read must not fail the step: %v", err)
	}
	if len(calls.updates) != 0 {
		t.Errorf("the step could not prove the CLI has `plugin update` but ran it: %v", calls.updates)
	}
	if got, want := len(calls.installs), len(herdrPluginSet); got != want {
		t.Errorf("installs = %d (%v), want %d: the set is installed whatever the version read does", got, calls.installs, want)
	}
	if !strings.Contains(strings.Join(*lines, "\n"), "could not be read") {
		t.Errorf("the step does not say why it did not update; logs = %v", *lines)
	}
}

// TestStepInstallWMHerdrUpdateFailureIsNotSuccess is the other side of the same
// guard: on a CLI that can update, the update has to succeed. This is what gives
// the version guard its teeth -- if the capability check is removed, the stub
// makes `herdr plugin update` fail the way 0.9.3 does, and this step fails
// instead of quietly skipping the failure.
func TestStepInstallWMHerdrUpdateFailureIsNotSuccess(t *testing.T) {
	m := herdrPluginModel(t, true)
	calls := stubHerdrPluginCLI(t, herdrPluginStub{
		version:   "herdr 0.9.5",
		updateErr: fmt.Errorf("error: aigora.lantern: fetch failed"),
		list:      herdrListWith(declaredHerdrPluginIDs()...),
	})

	err := stepInstallWM(&m)
	if err == nil {
		t.Fatal("an update that failed was reported as a successful step")
	}
	if len(calls.updates) == 0 {
		t.Error("the step failed without ever attempting the update")
	}
	if !strings.Contains(err.Error(), "update") {
		t.Errorf("the failure does not name the update: %v", err)
	}
}

// TestHerdrFallbackVersionHasPluginUpdate is the source guard for the pinned
// fallback: the version this installer downloads has to be one that can update
// its plugins, or a machine it provisions is stuck on the install-only path.
func TestHerdrFallbackVersionHasPluginUpdate(t *testing.T) {
	found := strings.TrimPrefix(herdrVersion, "v")
	if !herdrVersionAtLeast(found, herdrPluginUpdateMinVersion) {
		t.Errorf("the fallback download %s predates `herdr plugin update` (%s), so a machine this installer provisions cannot update its plugins", herdrVersion, herdrPluginUpdateMinVersion)
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

// --- the version parser is what makes the capability guard possible ---------

// TestParseHerdrVersion pins the shapes `herdr --version` answers with. The
// version is what decides whether the step may run `plugin update`, so a parser
// that answers "" for a real version would silently stop updating every machine.
func TestParseHerdrVersion(t *testing.T) {
	for _, tc := range []struct {
		name   string
		output string
		want   string
	}{
		{"the real one-line form", "herdr 0.9.3", "0.9.3"},
		{"a v prefix", "herdr v0.9.4", "0.9.4"},
		{"a build suffix", "herdr 0.9.4-dev", "0.9.4"},
		{"surrounding whitespace", "\nherdr 0.9.5\n", "0.9.5"},
		{"no version at all", "herdr", ""},
		{"empty", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseHerdrVersion(tc.output); got != tc.want {
				t.Errorf("parseHerdrVersion(%q) = %q, want %q", tc.output, got, tc.want)
			}
		})
	}
}

// TestHerdrVersionAtLeast pins the comparison against the release that first
// shipped `herdr plugin update` (0.9.4), including the 0.9 -> 0.10 boundary that
// a string comparison would get wrong.
func TestHerdrVersionAtLeast(t *testing.T) {
	for _, tc := range []struct {
		found string
		want  bool
	}{
		{"0.9.3", false},
		{"0.9.4", true},
		{"0.9.5", true},
		{"0.10.0", true},
		{"1.0.0", true},
		{"0.9", false},
		{"", false},
		{"not a version", false},
	} {
		t.Run(tc.found, func(t *testing.T) {
			if got := herdrVersionAtLeast(tc.found, herdrPluginUpdateMinVersion); got != tc.want {
				t.Errorf("herdrVersionAtLeast(%q, %q) = %v, want %v", tc.found, herdrPluginUpdateMinVersion, got, tc.want)
			}
		})
	}
}
