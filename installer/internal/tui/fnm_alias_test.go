package tui

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// withFnmMocks replaces the two fnm seams so the alias setup can be exercised
// without a real fnm. It returns the recorded commands and restores the seams
// when the test ends.
func withFnmMocks(t *testing.T, path func() string) *[]string {
	t.Helper()

	originalPath := fnmPath
	originalRun := runFnmWithLogs

	calls := []string{}
	fnmPath = path
	runFnmWithLogs = func(command string, _ *system.ExecOptions, _ system.LogCallback) *system.ExecResult {
		calls = append(calls, command)
		return &system.ExecResult{Command: command}
	}

	t.Cleanup(func() {
		fnmPath = originalPath
		runFnmWithLogs = originalRun
	})

	return &calls
}

// TestEnsureFnmDefaultAliasCreatesTheAlias pins the installer half of issue #23:
// when this run installs fnm, it also creates the `default` alias the shipped
// .zshrc starts on, so the configuration is not left in a state it cannot
// satisfy.
func TestEnsureFnmDefaultAliasCreatesTheAlias(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	calls := withFnmMocks(t, func() string { return "/fake/bin/fnm" })

	ensureFnmDefaultAlias("shell")

	want := []string{`"/fake/bin/fnm" install --lts`, `"/fake/bin/fnm" default lts-latest`}
	if strings.Join(*calls, "\n") != strings.Join(want, "\n") {
		t.Fatalf("fnm commands = %v, want %v", *calls, want)
	}
	t.Logf("fnm commands: %v", *calls)
}

// TestEnsureFnmDefaultAliasLeavesAnExistingAliasAlone proves the alias setup is
// idempotent: a machine that already has a `default` alias is not re-installed.
func TestEnsureFnmDefaultAliasLeavesAnExistingAliasAlone(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	versionDir := filepath.Join(home, ".local", "share", "fnm", "node-versions", "v22.0.0", "installation")
	if err := os.MkdirAll(versionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	aliases := fnmAliasesDir(home)
	if err := os.MkdirAll(aliases, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(versionDir, filepath.Join(aliases, "default")); err != nil {
		t.Skipf("symlinks are not available here: %v", err)
	}
	calls := withFnmMocks(t, func() string { return "/fake/bin/fnm" })

	ensureFnmDefaultAlias("shell")

	if len(*calls) != 0 {
		t.Fatalf("an existing alias must be left alone, got %v", *calls)
	}
}

// TestEnsureFnmDefaultAliasWithoutFnm is the control for the guard: no fnm means
// nothing to do and no command runs.
func TestEnsureFnmDefaultAliasWithoutFnm(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	calls := withFnmMocks(t, func() string { return "" })

	ensureFnmDefaultAlias("shell")

	if len(*calls) != 0 {
		t.Fatalf("no fnm must run no commands, got %v", *calls)
	}
}

// TestEnsureFnmDefaultAliasSurvivesAFailedInstall keeps a failed download from
// failing the shell step: the .zshrc is guarded, so a best-effort alias is the
// honest outcome.
func TestEnsureFnmDefaultAliasSurvivesAFailedInstall(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	originalRun := runFnmWithLogs
	originalPath := fnmPath
	t.Cleanup(func() {
		runFnmWithLogs = originalRun
		fnmPath = originalPath
	})
	fnmPath = func() string { return "/fake/bin/fnm" }

	calls := []string{}
	runFnmWithLogs = func(command string, _ *system.ExecOptions, _ system.LogCallback) *system.ExecResult {
		calls = append(calls, command)
		return &system.ExecResult{Command: command, Error: os.ErrDeadlineExceeded}
	}

	ensureFnmDefaultAlias("shell")

	if len(calls) != 1 {
		t.Fatalf("a failed install must not reach the alias command, got %v", calls)
	}
}

// TestStepInstallShellCreatesTheAliasOnlyForANewFnm is the integration guard for
// issue #23: the alias is created when this run installs fnm, and a machine that
// already had fnm (the user's own setup) is never touched.
func TestStepInstallShellCreatesTheAliasOnlyForANewFnm(t *testing.T) {
	t.Run("fnm installed by this run", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		withZshMocks(t, home)

		seen := 0
		calls := withFnmMocks(t, func() string {
			seen++
			if seen == 1 {
				// Not on the machine when the shell step began.
				return ""
			}
			// Installed by the package step.
			return "/fake/bin/fnm"
		})

		m := NewModel()
		m.SystemInfo = &system.SystemInfo{OS: system.OSDebian, HasBrew: true}
		m.Choices = UserChoices{OS: "linux", Shell: "zsh", WindowMgr: "none"}
		m.RepoDir = repoRoot(t)

		if err := stepInstallShell(&m); err != nil {
			t.Fatalf("zsh step failed: %v", err)
		}
		if len(*calls) != 2 {
			t.Fatalf("a newly installed fnm must get its default alias, got %v", *calls)
		}
	})

	t.Run("user's own fnm is untouched", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		withZshMocks(t, home)
		calls := withFnmMocks(t, func() string { return "/fake/bin/fnm" })

		m := NewModel()
		m.SystemInfo = &system.SystemInfo{OS: system.OSDebian, HasBrew: true}
		m.Choices = UserChoices{OS: "linux", Shell: "zsh", WindowMgr: "none"}
		m.RepoDir = repoRoot(t)

		if err := stepInstallShell(&m); err != nil {
			t.Fatalf("zsh step failed: %v", err)
		}
		if len(*calls) != 0 {
			t.Fatalf("a pre-existing fnm must not be changed, got %v", *calls)
		}
	})
}

// shippedFnmBlock returns the fnm initialization from the shipped .zshrc, so the
// test runs the configuration that is actually installed instead of a paraphrase
// of it.
func shippedFnmBlock(t *testing.T) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(repoRoot(t), repoAssetZshrc))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	const start = "# Use fnm as the single Node version manager."
	const end = "# Use the system CA bundle for Node-based tooling."
	from := strings.Index(text, start)
	to := strings.Index(text, end)
	if from < 0 || to <= from {
		t.Fatal("could not locate the fnm block in the shipped .zshrc")
	}
	return text[from:to]
}

// TestShippedZshrcDoesNotShoutAboutAMissingFnmAlias pins the configuration half
// of issue #23 by running the shipped block in zsh. The fake fnm writes an error
// to stderr for `use`, exactly as the real one does when the alias is missing:
// with the alias present that error is discarded, and without it the command is
// never run, so an interactive shell stays silent in both cases.
func TestShippedZshrcDoesNotShoutAboutAMissingFnmAlias(t *testing.T) {
	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("zsh is not available")
	}

	block := shippedFnmBlock(t)

	run := func(t *testing.T, withAlias bool) (log, stderr string) {
		home := t.TempDir()
		fnmDir := filepath.Join(home, ".local", "share", "fnm")
		aliases := filepath.Join(fnmDir, "aliases")
		if err := os.MkdirAll(aliases, 0o755); err != nil {
			t.Fatal(err)
		}
		if withAlias {
			versionDir := filepath.Join(fnmDir, "node-versions", "v22.0.0", "installation")
			if err := os.MkdirAll(versionDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(versionDir, filepath.Join(aliases, "default")); err != nil {
				t.Skipf("symlinks are not available here: %v", err)
			}
		}

		bin := t.TempDir()
		callLog := filepath.Join(bin, "calls")
		// `env` is what the block evaluates; anything else is recorded. `use`
		// also writes the real error text to stderr so an unguarded call would be
		// visible in the assertion below.
		fake := "#!/bin/sh\n" +
			"case \"$1\" in\n" +
			"  env) exit 0 ;;\n" +
			"  *) echo \"$@\" >> \"" + callLog + "\"\n" +
			"     echo 'error: Requested version default is not currently installed' >&2 ;;\n" +
			"esac\n"
		if err := os.WriteFile(filepath.Join(bin, "fnm"), []byte(fake), 0o755); err != nil {
			t.Fatal(err)
		}

		cmd := exec.Command("zsh", "-f", "-c", block)
		cmd.Env = append(os.Environ(),
			"HOME="+home,
			"IS_TERMUX=0",
			"PATH="+bin,
		)
		var stderrBuf bytes.Buffer
		cmd.Stderr = &stderrBuf
		if err := cmd.Run(); err != nil {
			t.Fatalf("sourcing the shipped fnm block failed: %v\n%s", err, stderrBuf.String())
		}

		contents, _ := os.ReadFile(callLog)
		return string(contents), stderrBuf.String()
	}

	t.Run("alias present", func(t *testing.T) {
		log, stderr := run(t, true)
		if !strings.Contains(log, "use default") {
			t.Errorf("with the alias present the shell should start on it, calls: %q", log)
		}
		if stderr != "" {
			t.Errorf("an interactive shell printed an error: %q", stderr)
		}
	})

	t.Run("alias absent for the user's own reasons", func(t *testing.T) {
		log, stderr := run(t, false)
		if strings.Contains(log, "use") {
			t.Errorf("without the alias fnm must not be asked to use it, calls: %q", log)
		}
		if stderr != "" {
			t.Errorf("a shell without the alias printed an error: %q", stderr)
		}
	})
}
