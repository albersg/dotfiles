package tui

import (
	"errors"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// withDepsMocks replaces every command runner the dependency step can reach and
// records the calls. When sudoResult is non-nil it is returned by the sudo
// runner, so a failing sudo can be exercised without touching the host.
func withDepsMocks(t *testing.T, sudoResult *system.ExecResult) *[]packageCommandCall {
	t.Helper()

	originalPkg := runPkgInstallWithLogs
	originalSudo := runSudoWithLogs
	originalBrew := runBrewWithLogs

	calls := []packageCommandCall{}

	runPkgInstallWithLogs = func(packages string, opts *system.ExecOptions, onLog func(string)) *system.ExecResult {
		calls = append(calls, packageCommandCall{runner: "pkg", command: packages})
		return &system.ExecResult{Command: packages}
	}
	runSudoWithLogs = func(command string, opts *system.ExecOptions, onLog system.LogCallback) *system.ExecResult {
		calls = append(calls, packageCommandCall{runner: "sudo", command: command})
		if sudoResult != nil {
			result := *sudoResult
			result.Command = command
			return &result
		}
		return &system.ExecResult{Command: command}
	}
	runBrewWithLogs = func(args string, opts *system.ExecOptions, onLog system.LogCallback) *system.ExecResult {
		calls = append(calls, packageCommandCall{runner: "brew", command: args})
		return &system.ExecResult{Command: args}
	}

	t.Cleanup(func() {
		runPkgInstallWithLogs = originalPkg
		runSudoWithLogs = originalSudo
		runBrewWithLogs = originalBrew
	})

	return &calls
}

// TestStepInstallDepsWSLDebianUsesBrewWhenPresent pins the package-manager
// choice issue #12 reported: a Debian-like WSL host with Homebrew must install
// its dependencies through Homebrew and never reach for sudo apt-get.
func TestStepInstallDepsWSLDebianUsesBrewWhenPresent(t *testing.T) {
	calls := withDepsMocks(t, nil)

	m := &Model{SystemInfo: &system.SystemInfo{OS: system.OSDebian, IsWSL: true, HasBrew: true}}
	if err := stepInstallDeps(m); err != nil {
		t.Fatalf("stepInstallDeps failed: %v", err)
	}

	expected := []packageCommandCall{
		{runner: "brew", command: "install curl file git wget unzip fontconfig"},
	}
	if !reflect.DeepEqual(*calls, expected) {
		t.Fatalf("calls = %#v, want %#v", *calls, expected)
	}
}

// TestStepInstallDepsWSLDebianWithoutBrewUsesApt is the other half of the same
// choice: without Homebrew the Debian-like distribution still installs through
// apt. This test used to assert the wslu name in the apt command; that was the
// behaviour issue #24 corrects. Debian 12 does not carry wslu, apt aborts the
// whole transaction on the unknown name, and the names now come from
// depsForInstall rather than being written out here, so the same selection the
// TUI script renders is the one that runs.
func TestStepInstallDepsWSLDebianWithoutBrewUsesApt(t *testing.T) {
	calls := withDepsMocks(t, nil)

	m := &Model{SystemInfo: &system.SystemInfo{OS: system.OSDebian, IsWSL: true, HasBrew: false}}
	if err := stepInstallDeps(m); err != nil {
		t.Fatalf("stepInstallDeps failed: %v", err)
	}

	expected := []packageCommandCall{
		{runner: "sudo", command: "apt-get update"},
		{runner: "sudo", command: "apt-get install -y build-essential curl file git unzip fontconfig procps"},
	}
	if !reflect.DeepEqual(*calls, expected) {
		t.Fatalf("calls = %#v, want %#v", *calls, expected)
	}
	if strings.Contains(callsString(*calls), "wslu") {
		t.Errorf("apt must never be asked for the wslu name Debian does not carry: %#v", *calls)
	}
}

// TestStepInstallDepsResolvesWSLDistribution proves the step dispatches on the
// distribution that detection now records on WSL, so Fedora-on-WSL runs dnf and
// Arch-on-WSL runs pacman instead of both assuming apt.
func TestStepInstallDepsResolvesWSLDistribution(t *testing.T) {
	cases := []struct {
		name string
		os   system.OSType
		want packageCommandCall
	}{
		{
			name: "fedora",
			os:   system.OSFedora,
			want: packageCommandCall{runner: "sudo", command: "dnf install -y @development-tools curl file git wget unzip fontconfig"},
		},
		{
			name: "arch",
			os:   system.OSArch,
			want: packageCommandCall{runner: "sudo", command: "pacman -S --needed --noconfirm base-devel curl file git wget unzip fontconfig"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := withDepsMocks(t, nil)

			m := &Model{SystemInfo: &system.SystemInfo{OS: tc.os, IsWSL: true, HasBrew: false}}
			if err := stepInstallDeps(m); err != nil {
				t.Fatalf("stepInstallDeps failed: %v", err)
			}

			if !reflect.DeepEqual(*calls, []packageCommandCall{tc.want}) {
				t.Fatalf("calls = %#v, want %#v", *calls, []packageCommandCall{tc.want})
			}
		})
	}
}

// TestStepInstallDepsNamesCommandsWhenSudoCannotPrompt pins the other half of
// issue #12: when sudo cannot ask for a password the step fails with the exact
// commands to run and how to skip them, not with the raw sudo error, and it
// never reports a step that did not happen as a success.
func TestStepInstallDepsNamesCommandsWhenSudoCannotPrompt(t *testing.T) {
	sudoFailure := &system.ExecResult{
		Error:  errors.New("exit status 1"),
		Stderr: "sudo: no tty present and no askpass program specified",
	}
	calls := withDepsMocks(t, sudoFailure)

	m := &Model{SystemInfo: &system.SystemInfo{OS: system.OSDebian, IsWSL: true, HasBrew: false}}
	err := stepInstallDeps(m)
	if err == nil {
		t.Fatal("a failed dependency install must be reported as a failure")
	}

	message := err.Error()
	for _, want := range []string{
		"apt-get update",
		"apt-get install -y build-essential curl file git unzip fontconfig procps",
		"DOTFILES_SKIP_DEPS=1",
	} {
		if !strings.Contains(message, want) {
			t.Errorf("error message %q does not mention %q", message, want)
		}
	}
	if strings.Contains(message, "no tty present") {
		t.Errorf("error message should not surface the raw sudo error: %q", message)
	}

	// The first sudo command is the only one that ran, and the step stopped.
	expected := []packageCommandCall{{runner: "sudo", command: "apt-get update"}}
	if !reflect.DeepEqual(*calls, expected) {
		t.Fatalf("calls = %#v, want %#v", *calls, expected)
	}
}

// TestStepInstallDepsHonoursSkipEnv proves the skip path the failure message
// names actually works and runs nothing.
func TestStepInstallDepsHonoursSkipEnv(t *testing.T) {
	calls := withDepsMocks(t, nil)
	t.Setenv(envSkipDeps, "1")

	m := &Model{SystemInfo: &system.SystemInfo{OS: system.OSDebian, IsWSL: true, HasBrew: false}}
	if err := stepInstallDeps(m); err != nil {
		t.Fatalf("skip must not fail: %v", err)
	}
	if len(*calls) != 0 {
		t.Fatalf("skip must run no commands, got %#v", *calls)
	}
}

// callsString flattens the recorded calls for a substring assertion.
func callsString(calls []packageCommandCall) string {
	var b strings.Builder
	for _, call := range calls {
		b.WriteString(call.runner)
		b.WriteString(" ")
		b.WriteString(call.command)
		b.WriteString("\n")
	}
	return b.String()
}

// scriptInstallCommands returns the script lines that actually hand packages to
// a manager, ignoring the echoed notices and the cosmetic lines.
func scriptInstallCommands(script string) []string {
	var commands []string
	for _, line := range strings.Split(script, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "sudo ") || strings.HasPrefix(trimmed, "pkg ") || strings.HasPrefix(trimmed, "\"") {
			commands = append(commands, trimmed)
		}
	}
	return commands
}

// scriptUsesPlan reports whether the rendered script carries the plan's command.
func scriptUsesPlan(script string, plan platformInstallPlan) bool {
	want := plan.nativeCommand()
	switch plan.Manager {
	case "brew":
		want = "install " + plan.Packages
	case "pkg":
		want = "pkg install -y " + plan.Packages
	}
	for _, command := range scriptInstallCommands(script) {
		if want != "" && strings.Contains(command, want) {
			return true
		}
	}
	return false
}

// callsUsePlan reports whether the executed step ran the plan's command.
func callsUsePlan(calls []packageCommandCall, plan platformInstallPlan) bool {
	for _, call := range calls {
		switch plan.Manager {
		case "brew":
			if call.runner == "brew" && call.command == "install "+plan.Packages {
				return true
			}
		case "pkg":
			if call.runner == "pkg" && call.command == plan.Packages {
				return true
			}
		default:
			if call.runner == "sudo" && call.command == plan.nativeCommand() {
				return true
			}
		}
	}
	return false
}

// TestDepsScriptIsValidShell runs the generated dependency scripts through
// `sh -n`, so a rendering change that produces a syntactically broken script is
// caught here rather than in a suspended TUI. The non-interactive path never
// renders a script; this is the per-path check that only the interactive route
// needs.
func TestDepsScriptIsValidShell(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh is not available")
	}

	for _, info := range []*system.SystemInfo{
		{OS: system.OSDebian, IsWSL: true, HasBrew: false},
		{OS: system.OSDebian, IsWSL: true, HasBrew: true},
		{OS: system.OSFedora, HasBrew: false},
		{OS: system.OSArch, HasBrew: false},
	} {
		m := &Model{SystemInfo: info}
		script, err := getDepsScript(m)
		if err != nil {
			t.Fatalf("getDepsScript(%+v) failed: %v", info, err)
		}

		cmd := exec.Command("sh", "-n")
		cmd.Stdin = strings.NewReader(script)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("sh -n rejected the generated script for %+v: %v\n%s\n%s", info, err, out, script)
		}
	}
}

// TestDepsInteractiveAndExecutedPathsShareThePlan pins the shape of the issue #24
// fix: the TUI script and the executed step are not two transcriptions of the
// same rules, they are the same depsForInstall result and the same
// planPlatformInstall decision. The test reads that plan and asserts both paths
// carry exactly it, so a future edit that re-implements either path diverges
// visibly. It also pins the reported failure: the WSL Debian plan never names
// wslu, on either path.
func TestDepsInteractiveAndExecutedPathsShareThePlan(t *testing.T) {
	cases := []struct {
		name     string
		info     *system.SystemInfo
		manager  string
		packages string
	}{
		{
			name:     "wsl debian without homebrew",
			info:     &system.SystemInfo{OS: system.OSDebian, IsWSL: true, HasBrew: false},
			manager:  "apt-get",
			packages: "build-essential curl file git unzip fontconfig procps",
		},
		{
			name:     "wsl debian with homebrew",
			info:     &system.SystemInfo{OS: system.OSDebian, IsWSL: true, HasBrew: true},
			manager:  "brew",
			packages: "curl file git wget unzip fontconfig",
		},
		{
			name:     "fedora",
			info:     &system.SystemInfo{OS: system.OSFedora, HasBrew: false},
			manager:  "dnf",
			packages: "@development-tools curl file git wget unzip fontconfig",
		},
		{
			name:     "arch",
			info:     &system.SystemInfo{OS: system.OSArch, HasBrew: false},
			manager:  "pacman",
			packages: "base-devel curl file git wget unzip fontconfig",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := &Model{SystemInfo: tc.info}

			deps, debianGaps := depsForInstall(m)
			plan := planPlatformInstall(m, deps)
			if plan.Manager != tc.manager || plan.Packages != tc.packages {
				t.Fatalf("plan = %+v, want manager %q packages %q", plan, tc.manager, tc.packages)
			}
			if strings.Contains(plan.Packages, "wslu") {
				t.Errorf("the plan still hands the manager the unavailable wslu: %q", plan.Packages)
			}

			script, err := getDepsScript(m)
			if err != nil {
				t.Fatalf("getDepsScript failed: %v", err)
			}
			if !scriptUsesPlan(script, plan) {
				t.Fatalf("the TUI script does not carry plan %+v:\n%s", plan, script)
			}
			for _, command := range scriptInstallCommands(script) {
				if strings.Contains(command, "wslu") {
					t.Errorf("the TUI script hands the manager wslu: %q", command)
				}
			}
			if plan.Manager == "brew" && strings.Contains(script, "wslu") {
				t.Errorf("a Homebrew install must not name the apt-only wslu at all:\n%s", script)
			}
			t.Logf("TUI script for %s:\n%s", tc.name, script)

			calls := withDepsMocks(t, nil)
			if err := stepInstallDeps(m); err != nil {
				t.Fatalf("stepInstallDeps failed: %v", err)
			}
			if !callsUsePlan(*calls, plan) {
				t.Fatalf("the executed step does not carry plan %+v: %#v", plan, *calls)
			}
			if strings.Contains(callsString(*calls), "wslu") {
				t.Errorf("the executed step hands apt wslu: %#v", *calls)
			}
			if tc.info.IsWSL && (len(debianGaps) != 1 || debianGaps[0] != "wslu") {
				t.Errorf("the WSL Debian filter should report wslu as dropped, got %v", debianGaps)
			}
		})
	}
}
