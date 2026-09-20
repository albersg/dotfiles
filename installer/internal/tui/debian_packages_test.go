package tui

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// debianRemovedNames are the package names this installer used to request from
// apt but that ubuntu:22.04, the image the E2E target runs, does not carry. apt
// aborts the whole transaction on a single unknown name, so these were removed
// from every Debian list. The list is pinned here so a later edit cannot quietly
// put one back into an apt package set.
var debianRemovedNames = []string{
	"starship",
	"kubectx",
	"nushell",
	"zellij",
	"lazygit",
	"tree-sitter-cli",
}

// assertNoRemovedNames fails when an apt package set still names a package the
// distribution does not have.
func assertNoRemovedNames(t *testing.T, label, field string) {
	t.Helper()

	names := strings.Fields(field)
	for _, removed := range debianRemovedNames {
		for _, name := range names {
			if name == removed {
				t.Errorf("%s list still contains %q, which Debian/Ubuntu 22.04 does not carry", label, removed)
			}
		}
	}
}

// TestDebianShellListsOmitNamesTheDistributionDoesNotCarry pins the exact apt
// package set for each shell: only verified names remain, and the removed names
// are reported separately so the caller can tell the user where to get them.
func TestDebianShellListsOmitNamesTheDistributionDoesNotCarry(t *testing.T) {
	cases := []struct {
		name       string
		shell      string
		wantDebian string
		wantGone   []string
	}{
		{
			name:       "fish",
			shell:      "fish",
			wantDebian: "fish zoxide",
			wantGone:   []string{"starship"},
		},
		{
			name:       "zsh",
			shell:      "zsh",
			wantDebian: "zsh zoxide zsh-autosuggestions zsh-syntax-highlighting direnv jq gh bat fd-find ripgrep fzf",
			wantGone:   []string{"kubectx"},
		},
		{
			name:       "nushell",
			shell:      "nushell",
			wantDebian: "zoxide jq bash",
			wantGone:   []string{"nushell", "starship"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			packages, unavailable := shellPlatformPackages(tc.shell)

			if packages.Debian != tc.wantDebian {
				t.Errorf("Debian list = %q, want %q", packages.Debian, tc.wantDebian)
			}
			assertNoRemovedNames(t, tc.name+" Debian", packages.Debian)
			if !reflect.DeepEqual(unavailable, tc.wantGone) {
				t.Errorf("unavailable = %v, want %v", unavailable, tc.wantGone)
			}
		})
	}
}

// TestDebianWindowManagerAndNvimListsOmitNamesTheDistributionDoesNotCarry
// extends the same guard to the zellij and nvim package sets, which carried
// three more names Debian/Ubuntu 22.04 does not have.
func TestDebianWindowManagerAndNvimListsOmitNamesTheDistributionDoesNotCarry(t *testing.T) {
	zellij, zellijUnavailable := zellijPlatformPackages()
	if zellij.Debian != "" {
		t.Errorf("zellij Debian list = %q, want empty so apt is never asked for it", zellij.Debian)
	}
	if !reflect.DeepEqual(zellijUnavailable, []string{"zellij"}) {
		t.Errorf("zellij unavailable = %v, want [zellij]", zellijUnavailable)
	}

	nvim, nvimUnavailable := nvimPlatformPackages()
	wantNvim := "neovim git gcc fzf fd-find ripgrep coreutils bat curl"
	if nvim.Debian != wantNvim {
		t.Errorf("nvim Debian list = %q, want %q", nvim.Debian, wantNvim)
	}
	assertNoRemovedNames(t, "nvim Debian", nvim.Debian)
	if !reflect.DeepEqual(nvimUnavailable, []string{"lazygit", "tree-sitter-cli"}) {
		t.Errorf("nvim unavailable = %v, want [lazygit tree-sitter-cli]", nvimUnavailable)
	}
}

// aptInstallCommand returns the apt install command the step ran, if any.
func aptInstallCommand(calls *[]packageCommandCall) string {
	for _, call := range *calls {
		if call.runner == "sudo" && strings.HasPrefix(call.command, "apt-get install") {
			return call.command
		}
	}
	return ""
}

// TestStepInstallShellReportsDebianUnavailableCompanions pins the honesty
// requirement: a requested shell the distribution carries still installs, but
// the companion packages Debian/Ubuntu does not have are named in the log and
// never handed to apt.
func TestStepInstallShellReportsDebianUnavailableCompanions(t *testing.T) {
	t.Setenv("DOTFILES_VERBOSE", "1")
	SetNonInteractiveMode(true)
	t.Cleanup(func() { SetNonInteractiveMode(false) })

	home := t.TempDir()
	t.Setenv("HOME", home)
	readLog := captureStepStdout(t)
	calls := withZshMocks(t, home)

	m := NewModel()
	m.SystemInfo = &system.SystemInfo{OS: system.OSDebian, HasBrew: false}
	m.Choices = UserChoices{OS: "linux", Shell: "zsh", WindowMgr: "none"}
	m.RepoDir = repoRoot(t)

	if err := stepInstallShell(&m); err != nil {
		t.Fatalf("a shell the distribution carries must still install cleanly: %v", err)
	}

	log := readLog()
	for _, want := range []string{
		"Not available in the Debian/Ubuntu repositories",
		"kubectx",
		"Homebrew",
		"upstream installer",
	} {
		if !strings.Contains(log, want) {
			t.Errorf("the install log does not mention %q:\n%s", want, log)
		}
	}

	apt := aptInstallCommand(calls)
	if apt == "" {
		t.Fatal("the zsh step did not run an apt install")
	}
	if strings.Contains(" "+apt+" ", " kubectx ") {
		t.Errorf("apt was still asked for the unavailable kubectx: %q", apt)
	}
}

// TestStepInstallShellFailsWhenRequestedShellIsNotInTheDistribution pins the
// rule applied to an explicitly requested shell: fish and zsh lose only a
// companion package, but a shell the user selected with --shell= and that the
// distribution cannot install must not end in a success report.
func TestStepInstallShellFailsWhenRequestedShellIsNotInTheDistribution(t *testing.T) {
	t.Setenv("DOTFILES_VERBOSE", "1")
	SetNonInteractiveMode(true)
	t.Cleanup(func() { SetNonInteractiveMode(false) })

	home := t.TempDir()
	t.Setenv("HOME", home)
	readLog := captureStepStdout(t)
	calls := withPackageCommandMocks(t, nil)

	m := NewModel()
	m.SystemInfo = &system.SystemInfo{OS: system.OSDebian, HasBrew: false}
	m.Choices = UserChoices{OS: "linux", Shell: "nushell", WindowMgr: "none"}
	m.RepoDir = repoRoot(t)

	err := stepInstallShell(&m)
	if err == nil {
		t.Fatal("a shell the distribution cannot install must not be reported as a success")
	}
	var stepErr *StepError
	if !errors.As(err, &stepErr) || stepErr.StepID != "shell" {
		t.Fatalf("the failure is not reported as a shell step failure: %v", err)
	}

	log := readLog()
	for _, want := range []string{
		"nushell",
		"Not available in the Debian/Ubuntu repositories",
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

// TestStepInstallShellInstallsNushellThroughHomebrew is the positive control for
// the rule above: the same shell installs normally when Homebrew is the package
// manager, so the failure is specific to apt and not to nushell itself.
func TestStepInstallShellInstallsNushellThroughHomebrew(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	calls := withPackageCommandMocks(t, nil)

	m := NewModel()
	m.SystemInfo = &system.SystemInfo{OS: system.OSDebian, HasBrew: true}
	m.Choices = UserChoices{OS: "linux", Shell: "nushell", WindowMgr: "none"}
	m.RepoDir = repoRoot(t)

	if err := stepInstallShell(&m); err != nil {
		t.Fatalf("a Debian host with Homebrew must still install nushell: %v", err)
	}

	found := false
	for _, call := range *calls {
		if call.runner == "sudo" {
			t.Errorf("nushell must not go through sudo/apt when Homebrew is present: %#v", call)
		}
		if call.runner == "brew" && strings.Contains(call.command, "nushell") {
			found = true
		}
	}
	if !found {
		t.Errorf("nushell was not installed through Homebrew: %#v", *calls)
	}
}

// TestStepInstallWMFailsWhenZellijIsNotInTheDistribution applies the same rule
// to the window manager the user explicitly selected, and names the route that
// does work instead of implying the install happened.
func TestStepInstallWMFailsWhenZellijIsNotInTheDistribution(t *testing.T) {
	if system.CommandExists("zellij") {
		t.Skip("zellij is installed on this host, so the step would take the already-installed path")
	}

	t.Setenv("DOTFILES_VERBOSE", "1")
	SetNonInteractiveMode(true)
	t.Cleanup(func() { SetNonInteractiveMode(false) })

	home := t.TempDir()
	t.Setenv("HOME", home)
	readLog := captureStepStdout(t)
	calls := withPackageCommandMocks(t, nil)

	m := NewModel()
	m.SystemInfo = &system.SystemInfo{OS: system.OSDebian, HasBrew: false}
	m.Choices = UserChoices{OS: "linux", Shell: "zsh", WindowMgr: "zellij"}
	m.RepoDir = repoRoot(t)

	err := stepInstallWM(&m)
	if err == nil {
		t.Fatal("zellij the distribution cannot install must not be reported as a success")
	}
	var stepErr *StepError
	if !errors.As(err, &stepErr) || stepErr.StepID != "wm" {
		t.Fatalf("the failure is not reported as a wm step failure: %v", err)
	}

	log := readLog()
	for _, want := range []string{
		"zellij",
		"Not available in the Debian/Ubuntu repositories",
		"Homebrew",
		"upstream",
	} {
		if !strings.Contains(log, want) {
			t.Errorf("the failure log does not mention %q:\n%s", want, log)
		}
	}

	if len(*calls) != 0 {
		t.Errorf("no package install should run for zellij, got %#v", *calls)
	}
}
