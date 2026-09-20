package tui

import (
	"errors"
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
// choice: without Homebrew the Debian-like distribution and its wslu package
// still install through apt.
func TestStepInstallDepsWSLDebianWithoutBrewUsesApt(t *testing.T) {
	calls := withDepsMocks(t, nil)

	m := &Model{SystemInfo: &system.SystemInfo{OS: system.OSDebian, IsWSL: true, HasBrew: false}}
	if err := stepInstallDeps(m); err != nil {
		t.Fatalf("stepInstallDeps failed: %v", err)
	}

	expected := []packageCommandCall{
		{runner: "sudo", command: "apt-get update"},
		{runner: "sudo", command: "apt-get install -y build-essential curl file git unzip fontconfig procps wslu"},
	}
	if !reflect.DeepEqual(*calls, expected) {
		t.Fatalf("calls = %#v, want %#v", *calls, expected)
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
			want: packageCommandCall{runner: "sudo", command: "dnf install -y --skip-unavailable @development-tools curl file git wget unzip fontconfig"},
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
		"apt-get install -y build-essential curl file git unzip fontconfig procps wslu",
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
