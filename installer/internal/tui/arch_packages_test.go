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

// archRemovedNames are the package names this installer used to request from
// pacman but that archlinux:latest, checked with `pacman -Si` against every name
// the Arch columns request, does not carry. pacman aborts the whole transaction
// on a single unknown name, so these were removed from every Arch list. The
// list is pinned here so a later edit cannot quietly put one back into a pacman
// package set.
var archRemovedNames = []string{
	"carapace",
	"zsh-theme-powerlevel10k",
}

// assertNoArchRemovedNames fails when a pacman package set still names a package
// the Arch repositories do not have.
func assertNoArchRemovedNames(t *testing.T, label, field string) {
	t.Helper()

	names := strings.Fields(field)
	for _, removed := range archRemovedNames {
		for _, name := range names {
			if name == removed {
				t.Errorf("%s list still contains %q, which the Arch repositories do not carry", label, removed)
			}
		}
	}
}

// TestArchShellListsOmitNamesTheDistributionDoesNotCarry pins the exact pacman
// package set for each shell: only verified names remain, and the removed names
// are reported separately so the caller can tell the user where to get them.
func TestArchShellListsOmitNamesTheDistributionDoesNotCarry(t *testing.T) {
	cases := []struct {
		name     string
		shell    string
		wantArch string
		wantGone []string
	}{
		{
			name:     "fish",
			shell:    "fish",
			wantArch: "fish zoxide atuin starship",
			wantGone: []string{"carapace"},
		},
		{
			name:     "zsh",
			shell:    "zsh",
			wantArch: "zsh zoxide atuin zsh-autosuggestions zsh-syntax-highlighting zsh-autocomplete kubectx eza bat fd ripgrep fzf direnv jq github-cli git-delta",
			wantGone: []string{"carapace", "zsh-theme-powerlevel10k"},
		},
		{
			name:     "nushell",
			shell:    "nushell",
			wantArch: "nushell zoxide atuin jq bash starship",
			wantGone: []string{"carapace"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			packages, _ := shellPlatformPackages(tc.shell)

			if packages.Arch != tc.wantArch {
				t.Errorf("Arch list = %q, want %q", packages.Arch, tc.wantArch)
			}
			assertNoArchRemovedNames(t, tc.name+" Arch", packages.Arch)
			if !reflect.DeepEqual(packages.archUnavailable, tc.wantGone) {
				t.Errorf("archUnavailable = %v, want %v", packages.archUnavailable, tc.wantGone)
			}
		})
	}
}

// TestArchRemovedNamesAreAbsentFromEveryArchList is the reintroduction guard
// across every Arch package set the installer builds. It holds even for the
// lists that never carried a missing name, so a later edit cannot reintroduce
// one anywhere.
func TestArchRemovedNamesAreAbsentFromEveryArchList(t *testing.T) {
	zellij, _ := zellijPlatformPackages()
	assertNoArchRemovedNames(t, "zellij Arch", zellij.Arch)

	nvim, _ := nvimPlatformPackages()
	assertNoArchRemovedNames(t, "nvim Arch", nvim.Arch)

	assertNoArchRemovedNames(t, "deps Arch", baseDependencies().Arch)
	assertNoArchRemovedNames(t, "clipboard Arch", clipboardPlatformPackages("xclip wl-clipboard").Arch)
}

// TestArchWindowManagerAndNvimListsAreFiltered records that the window-manager
// and Neovim Arch lists carry no removed name and, just as important, that the
// filter did not drop a verified name by accident.
func TestArchWindowManagerAndNvimListsAreFiltered(t *testing.T) {
	zellij, _ := zellijPlatformPackages()
	if zellij.Arch != "zellij" {
		t.Errorf("zellij Arch list = %q, want %q", zellij.Arch, "zellij")
	}
	if len(zellij.archUnavailable) != 0 {
		t.Errorf("zellij archUnavailable = %v, want none", zellij.archUnavailable)
	}

	nvim, _ := nvimPlatformPackages()
	wantNvim := "neovim git gcc fzf fd ripgrep coreutils bat curl lazygit tree-sitter"
	if nvim.Arch != wantNvim {
		t.Errorf("nvim Arch list = %q, want %q", nvim.Arch, wantNvim)
	}
	if len(nvim.archUnavailable) != 0 {
		t.Errorf("nvim archUnavailable = %v, want none", nvim.archUnavailable)
	}
}

// TestArchUnavailableNamesOnlyCompanions pins the classification the
// companion-versus-selected rule depends on: the two names Arch does not carry
// are companions the shell configuration reads at start, not components the
// user selects. Every selectable shell and window manager is in the official
// repositories, which is why the selected-component failure branch stays inert
// on a real Arch host.
func TestArchUnavailableNamesOnlyCompanions(t *testing.T) {
	want := map[string]bool{
		"carapace":                true,
		"zsh-theme-powerlevel10k": true,
	}
	if !reflect.DeepEqual(archUnavailable, want) {
		t.Errorf("archUnavailable = %v, want %v", archUnavailable, want)
	}
	for _, component := range []string{"fish", "zsh", "nushell", "zellij", "tmux"} {
		if archUnavailable[component] {
			t.Errorf("%s is a component the user can select, so it must not be in archUnavailable", component)
		}
	}
}

// pacmanInstallCommand returns the pacman install command the step ran, if any.
func pacmanInstallCommand(calls *[]packageCommandCall) string {
	for _, call := range *calls {
		if call.runner == "sudo" && strings.HasPrefix(call.command, "pacman -S") {
			return call.command
		}
	}
	return ""
}

// TestStepInstallShellReportsArchUnavailableCompanions pins the honesty
// requirement on Arch: a requested shell Arch carries still installs, but the
// companion packages the repositories do not have are named in the log and
// never handed to pacman.
func TestStepInstallShellReportsArchUnavailableCompanions(t *testing.T) {
	t.Setenv("DOTFILES_VERBOSE", "1")
	SetNonInteractiveMode(true)
	t.Cleanup(func() { SetNonInteractiveMode(false) })

	home := t.TempDir()
	t.Setenv("HOME", home)
	// The notice is now reported after the attempt and only for components that
	// are still absent, so the presence check has to see a host without carapace
	// or zsh-theme-powerlevel10k. A private PATH and Homebrew prefix keep a
	// developer machine's own installations from hiding the gap this test
	// describes.
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HOMEBREW_PREFIX", t.TempDir())
	readLog := captureStepStdout(t)
	calls := withZshMocks(t, home)

	m := NewModel()
	m.SystemInfo = &system.SystemInfo{OS: system.OSArch, HasBrew: false}
	m.Choices = UserChoices{OS: "linux", Shell: "zsh", WindowMgr: "none"}
	m.RepoDir = repoRoot(t)

	if err := stepInstallShell(&m); err != nil {
		t.Fatalf("a shell the distribution carries must still install cleanly: %v", err)
	}

	log := readLog()
	for _, want := range []string{
		"Not available in the Arch repositories",
		"carapace",
		"zsh-theme-powerlevel10k",
		"Homebrew",
		"upstream installer",
	} {
		if !strings.Contains(log, want) {
			t.Errorf("the install log does not mention %q:\n%s", want, log)
		}
	}

	pacman := pacmanInstallCommand(calls)
	if pacman == "" {
		t.Fatal("the zsh step did not run a pacman install")
	}
	for _, removed := range archRemovedNames {
		if strings.Contains(" "+pacman+" ", " "+removed+" ") {
			t.Errorf("pacman was still asked for the unavailable %s: %q", removed, pacman)
		}
	}
}

// TestStepInstallShellFailsWhenSelectedShellIsUnavailableOnArch proves the
// selected-component guard is wired. The Arch repositories carry every shell
// this installer can select, so the guard is inert on a real Arch host; making
// fish temporarily unavailable is what exercises it. The guard fires after the
// install has been attempted: pacman runs for the packages the filter kept, and
// the step must still fail because the selected shell itself is absent, instead
// of reporting an install it never performed.
func TestStepInstallShellFailsWhenSelectedShellIsUnavailableOnArch(t *testing.T) {
	archUnavailable["fish"] = true
	t.Cleanup(func() { delete(archUnavailable, "fish") })

	t.Setenv("DOTFILES_VERBOSE", "1")
	SetNonInteractiveMode(true)
	t.Cleanup(func() { SetNonInteractiveMode(false) })

	home := t.TempDir()
	t.Setenv("HOME", home)
	// An empty PATH and a private Homebrew prefix keep a host-installed fish from
	// satisfying the presence check: the shell has to be genuinely absent for the
	// failure this test describes.
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HOMEBREW_PREFIX", t.TempDir())
	readLog := captureStepStdout(t)
	calls := withPackageCommandMocks(t, nil)

	m := NewModel()
	m.SystemInfo = &system.SystemInfo{OS: system.OSArch, HasBrew: false}
	m.Choices = UserChoices{OS: "linux", Shell: "fish", WindowMgr: "none"}
	m.RepoDir = repoRoot(t)

	err := stepInstallShell(&m)
	if err == nil {
		t.Fatal("a shell the Arch repositories cannot install must not be reported as a success")
	}
	var stepErr *StepError
	if !errors.As(err, &stepErr) || stepErr.StepID != "shell" {
		t.Fatalf("the failure is not reported as a shell step failure: %v", err)
	}

	log := readLog()
	for _, want := range []string{
		"fish",
		"Not available in the Arch repositories",
		"Homebrew",
		"upstream installer",
	} {
		if !strings.Contains(log, want) {
			t.Errorf("the failure log does not mention %q:\n%s", want, log)
		}
	}

	// The guard is checked after the install attempt, so the packages the filter
	// kept are still handed to pacman before the missing shell is reported.
	pacman := pacmanInstallCommand(calls)
	if pacman == "" {
		t.Fatal("the fish step must attempt the filtered pacman install before failing")
	}
	if strings.Contains(" "+pacman+" ", " fish ") {
		t.Errorf("pacman was asked for the unavailable fish: %q", pacman)
	}
	for _, call := range *calls {
		if call.runner == "brew" {
			t.Errorf("no Homebrew route is available on this host, got %#v", call)
		}
	}
}

// TestStepInstallWMFailsWhenZellijIsUnavailableOnArch applies the same
// selected-component guard to the window manager. As above, Arch carries
// zellij, so the map is edited for the duration of the test to prove the guard
// is wired rather than to claim a real Arch host is missing it.
func TestStepInstallWMFailsWhenZellijIsUnavailableOnArch(t *testing.T) {
	archUnavailable["zellij"] = true
	t.Cleanup(func() { delete(archUnavailable, "zellij") })

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
	m.SystemInfo = &system.SystemInfo{OS: system.OSArch, HasBrew: false}
	m.Choices = UserChoices{OS: "linux", Shell: "zsh", WindowMgr: "zellij"}
	m.RepoDir = repoRoot(t)

	err := stepInstallWM(&m)
	if err == nil {
		t.Fatal("a window manager the Arch repositories cannot install must not be reported as a success")
	}
	var stepErr *StepError
	if !errors.As(err, &stepErr) || stepErr.StepID != "wm" {
		t.Fatalf("the failure is not reported as a wm step failure: %v", err)
	}

	log := readLog()
	for _, want := range []string{
		"zellij",
		"Not available in the Arch repositories",
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

// TestStepInstallShellArchWithHomebrewInstallsThroughBrew is the positive
// direction of the route correction for the shell. With Homebrew present the
// Arch lists are not sent to pacman at all: the default branch installs the
// unfiltered Brew list, so the companions Arch does not carry are installed
// rather than reported missing.
func TestStepInstallShellArchWithHomebrewInstallsThroughBrew(t *testing.T) {
	t.Setenv("DOTFILES_VERBOSE", "1")
	SetNonInteractiveMode(true)
	t.Cleanup(func() { SetNonInteractiveMode(false) })

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir())
	prefix := t.TempDir()
	t.Setenv("HOMEBREW_PREFIX", prefix)

	readLog := captureStepStdout(t)
	calls := withZshMocks(t, home)
	runBrewWithLogs = func(args string, opts *system.ExecOptions, onLog system.LogCallback) *system.ExecResult {
		*calls = append(*calls, packageCommandCall{runner: "brew", command: args})
		if err := os.MkdirAll(filepath.Join(prefix, "bin"), 0o755); err != nil {
			t.Fatalf("mock brew could not create the Homebrew prefix: %v", err)
		}
		for _, name := range []string{"zsh", "carapace", "zsh-theme-powerlevel10k"} {
			if err := os.WriteFile(filepath.Join(prefix, "bin", name), []byte("#!/bin/sh\n"), 0o755); err != nil {
				t.Fatalf("mock brew could not create %s: %v", name, err)
			}
		}
		return &system.ExecResult{Command: args}
	}

	m := NewModel()
	m.SystemInfo = &system.SystemInfo{OS: system.OSArch, HasBrew: true}
	m.Choices = UserChoices{OS: "linux", Shell: "zsh", WindowMgr: "none"}
	m.RepoDir = repoRoot(t)

	if err := stepInstallShell(&m); err != nil {
		t.Fatalf("an Arch host with Homebrew must install the shell through it: %v\n%s", err, readLog())
	}

	installed := false
	for _, call := range *calls {
		if call.runner == "sudo" {
			t.Errorf("pacman must not run when Homebrew is present: %#v", call)
		}
		if call.runner == "brew" && strings.Contains(call.command, "carapace") {
			installed = true
		}
	}
	if !installed {
		t.Errorf("the unfiltered Brew list was not installed: %#v", *calls)
	}
	if log := readLog(); strings.Contains(log, "Not available in the Arch repositories") {
		t.Errorf("an Arch host with Homebrew must not be told the components are unavailable:\n%s", log)
	}
}

// TestStepInstallWMInstallsZellijThroughHomebrewOnArch is the positive direction
// of the selected-component rule. Arch carries zellij, but the same guard fires
// when a future edit makes it unavailable, and an Arch host whose Homebrew can
// provide it must install it instead of failing. The mocked brew install writes
// the binary into the Homebrew prefix, which is where the real one lands and
// which this process's PATH does not contain, so the test also pins that a
// component installed moments earlier is not reported missing.
func TestStepInstallWMInstallsZellijThroughHomebrewOnArch(t *testing.T) {
	archUnavailable["zellij"] = true
	t.Cleanup(func() { delete(archUnavailable, "zellij") })

	t.Setenv("DOTFILES_VERBOSE", "1")
	SetNonInteractiveMode(true)
	t.Cleanup(func() { SetNonInteractiveMode(false) })

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir())
	prefix := t.TempDir()
	t.Setenv("HOMEBREW_PREFIX", prefix)

	readLog := captureStepStdout(t)
	calls := withPackageCommandMocks(t, nil)
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
	m.SystemInfo = &system.SystemInfo{OS: system.OSArch, HasBrew: true}
	m.Choices = UserChoices{OS: "linux", Shell: "zsh", WindowMgr: "zellij"}
	m.RepoDir = repoRoot(t)

	if err := stepInstallWM(&m); err != nil {
		t.Fatalf("an Arch host with Homebrew must install zellij rather than fail: %v\n%s", err, readLog())
	}

	installed := false
	for _, call := range *calls {
		if call.runner == "sudo" {
			t.Errorf("zellij must not be attempted through pacman when the Arch list is filtered: %#v", call)
		}
		if call.runner == "brew" && strings.Contains(call.command, "zellij") {
			installed = true
		}
	}
	if !installed {
		t.Errorf("zellij was not installed through Homebrew: %#v", *calls)
	}
	if log := readLog(); strings.Contains(log, "Not available in the Arch repositories") {
		t.Errorf("an Arch host with Homebrew must not be told zellij is unavailable:\n%s", log)
	}
}
