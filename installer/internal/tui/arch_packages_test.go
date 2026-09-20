package tui

import (
	"errors"
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
// fish temporarily unavailable is what exercises it. If a later edit introduces
// an unavailable selectable shell, the step must fail instead of reporting an
// install it never performed.
func TestStepInstallShellFailsWhenSelectedShellIsUnavailableOnArch(t *testing.T) {
	archUnavailable["fish"] = true
	t.Cleanup(func() { delete(archUnavailable, "fish") })

	t.Setenv("DOTFILES_VERBOSE", "1")
	SetNonInteractiveMode(true)
	t.Cleanup(func() { SetNonInteractiveMode(false) })

	home := t.TempDir()
	t.Setenv("HOME", home)
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

	if len(*calls) != 0 {
		t.Errorf("no package install should run for a shell the distribution cannot provide, got %#v", *calls)
	}
}

// TestStepInstallWMFailsWhenZellijIsUnavailableOnArch applies the same
// selected-component guard to the window manager. As above, Arch carries
// zellij, so the map is edited for the duration of the test to prove the guard
// is wired rather than to claim a real Arch host is missing it.
func TestStepInstallWMFailsWhenZellijIsUnavailableOnArch(t *testing.T) {
	if system.CommandExists("zellij") {
		t.Skip("zellij is installed on this host, so the step would take the already-installed path")
	}

	archUnavailable["zellij"] = true
	t.Cleanup(func() { delete(archUnavailable, "zellij") })

	t.Setenv("DOTFILES_VERBOSE", "1")
	SetNonInteractiveMode(true)
	t.Cleanup(func() { SetNonInteractiveMode(false) })

	home := t.TempDir()
	t.Setenv("HOME", home)
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
