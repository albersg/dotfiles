package tui

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// fedoraRemovedNames are the package names this installer used to request from
// dnf but that fedora:latest, the image the E2E target builds from, does not
// carry. Every name in the Fedora columns was checked with
// `dnf install --assumeno`, which resolves virtual provides the way the real
// install does; these four were the only ones that did not resolve. dnf aborts
// the whole transaction on a single unknown name, so they were removed from
// every Fedora list. The list is pinned here so a later edit cannot quietly put
// one back into a dnf package set.
var fedoraRemovedNames = []string{
	"carapace",
	"starship",
	"zellij",
	"lazygit",
}

// assertNoFedoraRemovedNames fails when a dnf package set still names a package
// the Fedora repositories do not have.
func assertNoFedoraRemovedNames(t *testing.T, label, field string) {
	t.Helper()

	names := strings.Fields(field)
	for _, removed := range fedoraRemovedNames {
		for _, name := range names {
			if name == removed {
				t.Errorf("%s list still contains %q, which the Fedora repositories do not carry", label, removed)
			}
		}
	}
}

// TestFedoraShellListsOmitNamesTheDistributionDoesNotCarry pins the exact dnf
// package set for each shell: only verified names remain, and the removed names
// are reported separately so the caller can tell the user where to get them.
func TestFedoraShellListsOmitNamesTheDistributionDoesNotCarry(t *testing.T) {
	cases := []struct {
		name       string
		shell      string
		wantFedora string
		wantGone   []string
	}{
		{
			name:       "fish",
			shell:      "fish",
			wantFedora: "fish zoxide atuin",
			wantGone:   []string{"carapace", "starship"},
		},
		{
			name:       "zsh",
			shell:      "zsh",
			wantFedora: "zsh zoxide atuin zsh-autosuggestions zsh-syntax-highlighting eza bat fd-find ripgrep fzf direnv jq gh git-delta",
			wantGone:   []string{"carapace", "starship"},
		},
		{
			name:       "nushell",
			shell:      "nushell",
			wantFedora: "nushell zoxide atuin jq bash",
			wantGone:   []string{"carapace", "starship"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			packages, _ := shellPlatformPackages(tc.shell)

			if packages.Fedora != tc.wantFedora {
				t.Errorf("Fedora list = %q, want %q", packages.Fedora, tc.wantFedora)
			}
			assertNoFedoraRemovedNames(t, tc.name+" Fedora", packages.Fedora)
			if !reflect.DeepEqual(packages.fedoraUnavailable, tc.wantGone) {
				t.Errorf("fedoraUnavailable = %v, want %v", packages.fedoraUnavailable, tc.wantGone)
			}
		})
	}
}

// TestFedoraRemovedNamesAreAbsentFromEveryFedoraList is the reintroduction guard
// across every Fedora package set the installer builds. It holds even for the
// lists that never carried a missing name, so a later edit cannot reintroduce
// one anywhere.
func TestFedoraRemovedNamesAreAbsentFromEveryFedoraList(t *testing.T) {
	zellij, _ := zellijPlatformPackages()
	assertNoFedoraRemovedNames(t, "zellij Fedora", zellij.Fedora)

	nvim, _ := nvimPlatformPackages()
	assertNoFedoraRemovedNames(t, "nvim Fedora", nvim.Fedora)

	assertNoFedoraRemovedNames(t, "deps Fedora", baseDependencies().Fedora)
	assertNoFedoraRemovedNames(t, "clipboard Fedora", clipboardPlatformPackages("xclip wl-clipboard").Fedora)
}

// TestFedoraWindowManagerAndNvimListsAreFiltered records the filtered lists
// and, just as important, that the filter did not drop a verified name by
// accident.
func TestFedoraWindowManagerAndNvimListsAreFiltered(t *testing.T) {
	zellij, _ := zellijPlatformPackages()
	if zellij.Fedora != "" {
		t.Errorf("zellij Fedora list = %q, want empty so dnf is never asked for it", zellij.Fedora)
	}
	if !reflect.DeepEqual(zellij.fedoraUnavailable, []string{"zellij"}) {
		t.Errorf("zellij fedoraUnavailable = %v, want [zellij]", zellij.fedoraUnavailable)
	}

	nvim, _ := nvimPlatformPackages()
	wantNvim := "neovim git gcc fzf fd-find ripgrep coreutils bat curl tree-sitter-cli"
	if nvim.Fedora != wantNvim {
		t.Errorf("nvim Fedora list = %q, want %q", nvim.Fedora, wantNvim)
	}
	assertNoFedoraRemovedNames(t, "nvim Fedora", nvim.Fedora)
	if !reflect.DeepEqual(nvim.fedoraUnavailable, []string{"lazygit"}) {
		t.Errorf("nvim fedoraUnavailable = %v, want [lazygit]", nvim.fedoraUnavailable)
	}
}

// TestFedoraUnavailableNamesOnlyCompanionsExceptZellij pins the classification
// the companion-versus-selected rule depends on. zellij is the one selectable
// component Fedora does not carry, so only zellij can fail a step; every shell
// the installer can select is present. nushell is deliberately absent from the
// map: it is not in Fedora 40 but fedora:latest carries it, and the check holds
// what the target image actually provides.
func TestFedoraUnavailableNamesOnlyCompanionsExceptZellij(t *testing.T) {
	want := map[string]bool{
		"carapace": true,
		"starship": true,
		"zellij":   true,
		"lazygit":  true,
	}
	if !reflect.DeepEqual(fedoraUnavailable, want) {
		t.Errorf("fedoraUnavailable = %v, want %v", fedoraUnavailable, want)
	}
	for _, component := range []string{"fish", "zsh", "nushell", "tmux"} {
		if fedoraUnavailable[component] {
			t.Errorf("%s is a component the user can select, so it must not be in fedoraUnavailable", component)
		}
	}
	if !fedoraUnavailable["zellij"] {
		t.Error("zellij is a selectable window manager Fedora does not carry, so it must be in fedoraUnavailable")
	}
}

// dnfInstallCommand returns the dnf install command the step ran, if any.
func dnfInstallCommand(calls *[]packageCommandCall) string {
	for _, call := range *calls {
		if call.runner == "sudo" && strings.HasPrefix(call.command, "dnf install") {
			return call.command
		}
	}
	return ""
}

// TestStepInstallShellReportsFedoraUnavailableCompanions pins the honesty
// requirement on Fedora: a requested shell Fedora carries still installs, but
// the companion packages the repositories do not have are named in the log and
// never handed to dnf.
func TestStepInstallShellReportsFedoraUnavailableCompanions(t *testing.T) {
	t.Setenv("DOTFILES_VERBOSE", "1")
	SetNonInteractiveMode(true)
	t.Cleanup(func() { SetNonInteractiveMode(false) })

	home := t.TempDir()
	t.Setenv("HOME", home)
	readLog := captureStepStdout(t)
	calls := withZshMocks(t, home)

	m := NewModel()
	m.SystemInfo = &system.SystemInfo{OS: system.OSFedora, HasBrew: false}
	m.Choices = UserChoices{OS: "linux", Shell: "zsh", WindowMgr: "none"}
	m.RepoDir = repoRoot(t)

	if err := stepInstallShell(&m); err != nil {
		t.Fatalf("a shell the distribution carries must still install cleanly: %v", err)
	}

	log := readLog()
	for _, want := range []string{
		"Not available in the Fedora repositories",
		"carapace",
		"starship",
		"Homebrew",
		"upstream installer",
	} {
		if !strings.Contains(log, want) {
			t.Errorf("the install log does not mention %q:\n%s", want, log)
		}
	}

	dnf := dnfInstallCommand(calls)
	if dnf == "" {
		t.Fatal("the zsh step did not run a dnf install")
	}
	for _, removed := range fedoraRemovedNames {
		if strings.Contains(" "+dnf+" ", " "+removed+" ") {
			t.Errorf("dnf was still asked for the unavailable %s: %q", removed, dnf)
		}
	}
}

// TestStepInstallShellFailsWhenSelectedShellIsUnavailableOnFedora proves the
// selected-component guard is wired. fedora:latest carries every shell this
// installer can select, so the guard is inert on a real Fedora host; making
// nushell temporarily unavailable is what exercises it. The guard now fires
// after the install has been attempted: dnf runs for the packages the filter
// kept, and the step must still fail because the selected shell itself is
// absent, instead of reporting an install it never performed.
func TestStepInstallShellFailsWhenSelectedShellIsUnavailableOnFedora(t *testing.T) {
	fedoraUnavailable["nushell"] = true
	t.Cleanup(func() { delete(fedoraUnavailable, "nushell") })

	t.Setenv("DOTFILES_VERBOSE", "1")
	SetNonInteractiveMode(true)
	t.Cleanup(func() { SetNonInteractiveMode(false) })

	home := t.TempDir()
	t.Setenv("HOME", home)
	// An empty PATH and a private Homebrew prefix keep a host-installed nushell
	// from satisfying the presence check: the shell has to be genuinely absent
	// for the failure this test describes.
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HOMEBREW_PREFIX", t.TempDir())
	readLog := captureStepStdout(t)
	calls := withPackageCommandMocks(t, nil)

	m := NewModel()
	m.SystemInfo = &system.SystemInfo{OS: system.OSFedora, HasBrew: false}
	m.Choices = UserChoices{OS: "linux", Shell: "nushell", WindowMgr: "none"}
	m.RepoDir = repoRoot(t)

	err := stepInstallShell(&m)
	if err == nil {
		t.Fatal("a shell the Fedora repositories cannot install must not be reported as a success")
	}
	var stepErr *StepError
	if !errors.As(err, &stepErr) || stepErr.StepID != "shell" {
		t.Fatalf("the failure is not reported as a shell step failure: %v", err)
	}

	log := readLog()
	for _, want := range []string{
		"nushell",
		"Not available in the Fedora repositories",
		"Homebrew",
		"upstream installer",
	} {
		if !strings.Contains(log, want) {
			t.Errorf("the failure log does not mention %q:\n%s", want, log)
		}
	}

	// The guard is checked after the install attempt, so the packages the filter
	// kept are still handed to dnf before the missing shell is reported.
	dnf := dnfInstallCommand(calls)
	if dnf == "" {
		t.Fatal("the nushell step must attempt the filtered dnf install before failing")
	}
	if strings.Contains(" "+dnf+" ", " nushell ") {
		t.Errorf("dnf was asked for the unavailable nushell: %q", dnf)
	}
	for _, call := range *calls {
		if call.runner == "brew" {
			t.Errorf("no Homebrew route is available on this host, got %#v", call)
		}
	}
}

// TestStepInstallWMInstallsZellijThroughHomebrewOnFedora is the positive
// direction of the selected-component rule: Fedora does not carry zellij, but a
// Fedora host whose Homebrew can provide it must install it instead of failing.
// The mocked brew install writes the binary into the Homebrew prefix, which is
// where the real one lands and which this process's PATH does not contain, so
// the test also pins that a component installed moments earlier is not reported
// missing.
func TestStepInstallWMInstallsZellijThroughHomebrewOnFedora(t *testing.T) {
	t.Setenv("DOTFILES_VERBOSE", "1")
	SetNonInteractiveMode(true)
	t.Cleanup(func() { SetNonInteractiveMode(false) })

	home := t.TempDir()
	t.Setenv("HOME", home)
	// An empty PATH keeps a host-installed zellij from short-circuiting the step,
	// and the private prefix is where the mocked install puts the binary.
	t.Setenv("PATH", t.TempDir())
	prefix := t.TempDir()
	t.Setenv("HOMEBREW_PREFIX", prefix)

	readLog := captureStepStdout(t)
	calls := withPackageCommandMocks(t, nil)
	// The real brew install creates the binary in its prefix; simulate that so
	// the step's presence check sees a component installed moments earlier.
	runBrewWithLogs = func(args string, opts *system.ExecOptions, onLog system.LogCallback) *system.ExecResult {
		*calls = append(*calls, packageCommandCall{runner: "brew", command: args})
		if err := os.MkdirAll(filepath.Join(prefix, "bin"), 0o755); err != nil {
			t.Fatalf("mock brew could not create the Homebrew prefix: %v", err)
		}
		if err := os.WriteFile(filepath.Join(prefix, "bin", "zellij"), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatalf("mock brew could not create the zellij binary: %v", err)
		}
		return &system.ExecResult{Command: args}
	}

	m := NewModel()
	m.SystemInfo = &system.SystemInfo{OS: system.OSFedora, HasBrew: true}
	m.Choices = UserChoices{OS: "linux", Shell: "zsh", WindowMgr: "zellij"}
	m.RepoDir = repoRoot(t)

	if err := stepInstallWM(&m); err != nil {
		t.Fatalf("a Fedora host with Homebrew must install zellij rather than fail: %v\n%s", err, readLog())
	}

	installed := false
	for _, call := range *calls {
		if call.runner == "sudo" {
			t.Errorf("zellij must not be attempted through dnf when the Fedora list is empty: %#v", call)
		}
		if call.runner == "brew" && strings.Contains(call.command, "zellij") {
			installed = true
		}
	}
	if !installed {
		t.Errorf("zellij was not installed through Homebrew: %#v", *calls)
	}
}

// TestStepInstallWMFailsWhenZellijIsUnavailableOnFedora applies the same
// selected-component rule to the window manager. Unlike Arch, Fedora really
// does not carry zellij, so this is the live case: with no Homebrew there is no
// route that can install it, and the step must fail, after attempting the
// install, rather than copy a configuration for a window manager it never
// installed.
func TestStepInstallWMFailsWhenZellijIsUnavailableOnFedora(t *testing.T) {
	t.Setenv("DOTFILES_VERBOSE", "1")
	SetNonInteractiveMode(true)
	t.Cleanup(func() { SetNonInteractiveMode(false) })

	home := t.TempDir()
	t.Setenv("HOME", home)
	// A controlled PATH and Homebrew prefix make zellij genuinely absent, so the
	// step reaches the failure instead of a host-installed copy.
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HOMEBREW_PREFIX", t.TempDir())
	readLog := captureStepStdout(t)
	calls := withPackageCommandMocks(t, nil)

	m := NewModel()
	m.SystemInfo = &system.SystemInfo{OS: system.OSFedora, HasBrew: false}
	m.Choices = UserChoices{OS: "linux", Shell: "zsh", WindowMgr: "zellij"}
	m.RepoDir = repoRoot(t)

	err := stepInstallWM(&m)
	if err == nil {
		t.Fatal("a window manager the Fedora repositories cannot install must not be reported as a success")
	}
	var stepErr *StepError
	if !errors.As(err, &stepErr) || stepErr.StepID != "wm" {
		t.Fatalf("the failure is not reported as a wm step failure: %v", err)
	}

	log := readLog()
	for _, want := range []string{
		"zellij",
		"Not available in the Fedora repositories",
		"Homebrew",
		"upstream installer",
	} {
		if !strings.Contains(log, want) {
			t.Errorf("the failure log does not mention %q:\n%s", want, log)
		}
	}

	if len(*calls) != 0 {
		t.Errorf("no package install should run for zellij, got %#v", *calls)
	}
}
