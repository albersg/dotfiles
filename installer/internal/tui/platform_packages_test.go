package tui

import (
	"errors"
	"reflect"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
)

type packageCommandCall struct {
	runner  string
	command string
}

func withPackageCommandMocks(t *testing.T, sudoErr error) *[]packageCommandCall {
	t.Helper()

	originalPkg := runPkgInstallWithLogs
	originalSudo := runSudoWithLogs
	originalBrew := runBrewWithLogs
	originalOhMyZsh := runOhMyZshInstaller

	calls := []packageCommandCall{}

	runPkgInstallWithLogs = func(packages string, opts *system.ExecOptions, onLog func(string)) *system.ExecResult {
		calls = append(calls, packageCommandCall{runner: "pkg", command: packages})
		return &system.ExecResult{Command: packages}
	}
	runSudoWithLogs = func(command string, opts *system.ExecOptions, onLog system.LogCallback) *system.ExecResult {
		calls = append(calls, packageCommandCall{runner: "sudo", command: command})
		return &system.ExecResult{Command: command, Error: sudoErr}
	}
	runBrewWithLogs = func(args string, opts *system.ExecOptions, onLog system.LogCallback) *system.ExecResult {
		calls = append(calls, packageCommandCall{runner: "brew", command: args})
		return &system.ExecResult{Command: args}
	}
	// Oh My Zsh is installed by shelling out to its official installer; the
	// tests must never reach the network.
	runOhMyZshInstaller = func(command string, opts *system.ExecOptions, onLog system.LogCallback) *system.ExecResult {
		calls = append(calls, packageCommandCall{runner: "oh-my-zsh", command: command})
		return &system.ExecResult{Command: command}
	}

	t.Cleanup(func() {
		runPkgInstallWithLogs = originalPkg
		runSudoWithLogs = originalSudo
		runBrewWithLogs = originalBrew
		runOhMyZshInstaller = originalOhMyZsh
	})

	return &calls
}

// TestInstallPlatformPackagesFedoraUsesBrewWhenPresent replaces the previous
// TestInstallPlatformPackagesFedoraFallsBackToBrewWhenNativeFails. That test
// asserted dnf ran first even on a Fedora host with Homebrew, with brew only
// reached when dnf failed. That is the behaviour the maintainer calls wrong:
// with Homebrew present installPlatformPackages must take the default branch
// and install the unfiltered Brew list, so a component the Fedora repositories
// do not carry is installed instead of being left to a step that would fail.
// The injected dnf failure is kept so a stray dnf call is still visible. The
// native-failure fallback itself is covered directly by
// TestRunNativeWithBrewFallbackUsesBrewWhenNativeFails.
func TestInstallPlatformPackagesFedoraUsesBrewWhenPresent(t *testing.T) {
	calls := withPackageCommandMocks(t, errors.New("dnf failed"))

	m := &Model{SystemInfo: &system.SystemInfo{OS: system.OSFedora, HasBrew: true}}
	result := installPlatformPackages(m, "shell", platformPackages{
		Brew:   "fish carapace zoxide atuin starship",
		Fedora: "fish carapace zoxide atuin starship",
	}, nil)

	if result.Error != nil {
		t.Fatalf("expected brew install to succeed, got error: %v", result.Error)
	}

	expected := []packageCommandCall{
		{runner: "brew", command: "install fish carapace zoxide atuin starship"},
	}
	if !reflect.DeepEqual(*calls, expected) {
		t.Fatalf("calls = %#v, want %#v", *calls, expected)
	}
}

func TestInstallPlatformPackagesDebianWithBrewUsesBrewDirectly(t *testing.T) {
	calls := withPackageCommandMocks(t, nil)

	m := &Model{SystemInfo: &system.SystemInfo{OS: system.OSDebian, HasBrew: true}}
	result := installPlatformPackages(m, "shell", platformPackages{
		Brew:   "fish carapace zoxide atuin starship",
		Debian: "fish zoxide starship",
	}, nil)

	if result.Error != nil {
		t.Fatalf("expected brew install to succeed, got error: %v", result.Error)
	}

	expected := []packageCommandCall{
		{runner: "brew", command: "install fish carapace zoxide atuin starship"},
	}
	if !reflect.DeepEqual(*calls, expected) {
		t.Fatalf("calls = %#v, want %#v", *calls, expected)
	}
}

func TestInstallPlatformPackagesDebianWithoutBrewUsesApt(t *testing.T) {
	calls := withPackageCommandMocks(t, nil)

	m := &Model{SystemInfo: &system.SystemInfo{OS: system.OSDebian, HasBrew: false}}
	result := installPlatformPackages(m, "shell", platformPackages{
		Brew:   "fish carapace zoxide atuin starship",
		Debian: "fish zoxide starship",
	}, nil)

	if result.Error != nil {
		t.Fatalf("expected apt install to succeed, got error: %v", result.Error)
	}

	expected := []packageCommandCall{
		{runner: "sudo", command: "apt-get install -y fish zoxide starship"},
	}
	if !reflect.DeepEqual(*calls, expected) {
		t.Fatalf("calls = %#v, want %#v", *calls, expected)
	}
}

// TestInstallPlatformPackagesArchUsesBrewWhenPresent replaces the previous
// TestInstallPlatformPackagesArchFallsBackToBrewWhenNativeFails. That test
// asserted pacman ran first even on an Arch host with Homebrew, with brew only
// reached when pacman failed. That is the same shape the Fedora correction
// identified as wrong: with Homebrew present installPlatformPackages must take
// the default branch and install the unfiltered Brew list, so a component the
// Arch repositories do not carry is installed instead of being left to a step
// that would fail. The injected pacman failure is kept so a stray pacman call
// is still visible. The native-failure fallback itself is covered directly by
// TestRunNativeWithBrewFallbackUsesBrewWhenNativeFails.
func TestInstallPlatformPackagesArchUsesBrewWhenPresent(t *testing.T) {
	calls := withPackageCommandMocks(t, errors.New("pacman failed"))

	m := &Model{SystemInfo: &system.SystemInfo{OS: system.OSArch, HasBrew: true}}
	result := installPlatformPackages(m, "shell", platformPackages{
		Brew: "fish carapace zoxide atuin starship",
		Arch: "fish carapace zoxide atuin starship",
	}, nil)

	if result.Error != nil {
		t.Fatalf("expected brew install to succeed, got error: %v", result.Error)
	}

	expected := []packageCommandCall{
		{runner: "brew", command: "install fish carapace zoxide atuin starship"},
	}
	if !reflect.DeepEqual(*calls, expected) {
		t.Fatalf("calls = %#v, want %#v", *calls, expected)
	}
}

// TestRunNativeWithBrewFallbackUsesBrewWhenNativeFails keeps the native-failure
// fallback under test now that every installPlatformPackages branch that reaches
// it is Homebrew-aware: the branch is only entered without Homebrew, so the
// fallback can no longer fire through installPlatformPackages, but it remains
// the one place a failed native command is retried through Homebrew. Both
// directions are pinned here: a failed native command falls back, and the next
// test proves a successful one does not.
func TestRunNativeWithBrewFallbackUsesBrewWhenNativeFails(t *testing.T) {
	calls := withPackageCommandMocks(t, errors.New("pacman failed"))

	result := runNativeWithBrewFallback("pacman -S --needed --noconfirm fish", "fish carapace", true, nil)
	if result.Error != nil {
		t.Fatalf("expected the brew fallback to succeed, got error: %v", result.Error)
	}

	expected := []packageCommandCall{
		{runner: "sudo", command: "pacman -S --needed --noconfirm fish"},
		{runner: "brew", command: "install fish carapace"},
	}
	if !reflect.DeepEqual(*calls, expected) {
		t.Fatalf("calls = %#v, want %#v", *calls, expected)
	}
}

// TestRunNativeWithBrewFallbackSkipsBrewWhenNativeSucceeds pins the other half:
// with nothing to retry, Homebrew is never reached.
func TestRunNativeWithBrewFallbackSkipsBrewWhenNativeSucceeds(t *testing.T) {
	calls := withPackageCommandMocks(t, nil)

	result := runNativeWithBrewFallback("pacman -S --needed --noconfirm fish", "fish carapace", true, nil)
	if result.Error != nil {
		t.Fatalf("expected the native command to succeed, got error: %v", result.Error)
	}

	expected := []packageCommandCall{
		{runner: "sudo", command: "pacman -S --needed --noconfirm fish"},
	}
	if !reflect.DeepEqual(*calls, expected) {
		t.Fatalf("calls = %#v, want %#v", *calls, expected)
	}
}
