package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// weztermInteractiveModel builds the model the interactive terminal step uses
// for a host installing WezTerm with the terminal absent.
//
// The presence check reads PATH, and the test points PATH at an empty directory
// so the install route is taken deterministically instead of depending on
// whether the test host happens to carry WezTerm. The shell syntax check below
// uses an absolute shell path because this PATH does not contain one.
func weztermInteractiveModel(t *testing.T, osType system.OSType) *Model {
	t.Helper()

	t.Setenv("PATH", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	return &Model{
		SystemInfo: &system.SystemInfo{OS: osType},
		Choices:    UserChoices{Terminal: "wezterm", OS: "linux"},
	}
}

// TestInteractiveWezTermScriptInstallsOnDebianLikeHosts covers the defect that
// this change removes: SetupInstallSteps flags the terminal step Interactive
// whenever the choice is Linux, but getTerminalScript returned an empty script
// for WezTerm on Debian/Ubuntu and WSL, so the TUI marked the step done without
// installing anything while the non-interactive step installed through Homebrew.
func TestInteractiveWezTermScriptInstallsOnDebianLikeHosts(t *testing.T) {
	for _, platform := range []struct {
		name   string
		osType system.OSType
	}{
		{"Debian/Ubuntu", system.OSDebian},
		{"Linux", system.OSLinux},
		{"WSL", system.OSWSL},
	} {
		t.Run(platform.name, func(t *testing.T) {
			m := weztermInteractiveModel(t, platform.osType)

			script, err := getTerminalScript(m)
			if err != nil {
				t.Fatalf("getTerminalScript(wezterm) failed: %v", err)
			}
			if script == "" {
				t.Fatal("the interactive WezTerm step produced an empty script: it would report itself done without installing")
			}

			// The tap and the formula are what stepInstallTerminal runs through
			// Homebrew on this host, and the interactive path has to run them
			// too.
			for _, want := range []string{"wez/wezterm-linuxbrew", "install wezterm"} {
				if !strings.Contains(script, want) {
					t.Errorf("the interactive WezTerm script does not install: %q is missing", want)
				}
			}

			path := filepath.Join(t.TempDir(), "wezterm.sh")
			if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			if out, err := exec.Command("/bin/sh", "-n", path).CombinedOutput(); err != nil {
				t.Fatalf("sh -n rejected the generated script: %v\n%s", err, out)
			}
		})
	}
}

// TestInteractiveWezTermScriptRunsTheSharedRoute is the keep-the-two-paths-in-
// step check: the interactive script renders the same install command lines the
// non-interactive step executes, so a change to one route cannot leave the other
// behind.
func TestInteractiveWezTermScriptRunsTheSharedRoute(t *testing.T) {
	m := weztermInteractiveModel(t, system.OSDebian)

	script, err := getTerminalScript(m)
	if err != nil {
		t.Fatalf("getTerminalScript(wezterm) failed: %v", err)
	}

	commands := weztermInstallCommands(m.SystemInfo)
	if len(commands) == 0 {
		t.Fatal("the shared WezTerm route is empty, so neither path would install anything")
	}
	for _, command := range commands {
		if !strings.Contains(script, command) {
			t.Errorf("the interactive script does not run %q, which is a command the non-interactive step runs", command)
		}
	}
}

// TestInteractiveWezTermScriptPassesShellcheck runs the linter the project uses
// for shell when it is installed. It skips where shellcheck is absent, so the
// suite still runs in a bare container, and it treats a warning as a failure
// because the generated script runs with the user's privileges.
func TestInteractiveWezTermScriptPassesShellcheck(t *testing.T) {
	shellcheck, err := exec.LookPath("shellcheck")
	if err != nil {
		t.Skip("shellcheck is not installed")
	}

	m := weztermInteractiveModel(t, system.OSDebian)
	script, err := getTerminalScript(m)
	if err != nil {
		t.Fatalf("getTerminalScript(wezterm) failed: %v", err)
	}

	path := filepath.Join(t.TempDir(), "wezterm.sh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(shellcheck, "--severity=warning", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("shellcheck rejected the generated script: %v\n%s", err, out)
	}
}

// TestInteractiveWezTermScriptDoesNotReinstallWhenPresent is the negative case:
// a machine that already carries WezTerm must not run the install route, so the
// non-empty script above is the absence branch and not a script that always
// reinstalls.
func TestInteractiveWezTermScriptDoesNotReinstallWhenPresent(t *testing.T) {
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "wezterm"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)
	t.Setenv("HOME", t.TempDir())

	m := &Model{
		SystemInfo: &system.SystemInfo{OS: system.OSDebian},
		Choices:    UserChoices{Terminal: "wezterm", OS: "linux"},
	}

	script, err := getTerminalScript(m)
	if err != nil {
		t.Fatalf("getTerminalScript(wezterm) failed: %v", err)
	}
	if !strings.Contains(script, "already installed") {
		t.Errorf("the interactive script does not report the present WezTerm: %q", script)
	}
	if strings.Contains(script, "wez/wezterm-linuxbrew") {
		t.Error("the interactive script installs WezTerm although it is already present")
	}
}
