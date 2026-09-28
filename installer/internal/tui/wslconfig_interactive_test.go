package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// generateWSLConfigScript builds the interactive WSL step's script against the
// model, the way runInteractiveStep does before it hands it to tea.ExecProcess.
func generateWSLConfigScript(t *testing.T, m *Model) string {
	t.Helper()

	script, err := getInteractiveScript(wslStepID, m)
	if err != nil {
		t.Fatalf("getInteractiveScript(%q) failed: %v", wslStepID, err)
	}
	if script == "" {
		t.Fatalf("the interactive WSL step produced no script")
	}

	// The script copies from a temporary file Go writes and deletes during the
	// run. Keep the test tidy when it inspects the script without running it.
	src := wslConfigSourceFromScript(t, script)
	t.Cleanup(func() { _ = os.Remove(src) })
	return script
}

// wslConfigSourceFromScript returns the rendered .wslconfig path the generated
// script copies from. Those bytes are the whole point of the rendered install:
// they are what the non-interactive step writes too.
func wslConfigSourceFromScript(t *testing.T, script string) string {
	t.Helper()

	for _, line := range strings.Split(script, "\n") {
		value, ok := strings.CutPrefix(line, "WSL_CONFIG_SRC=")
		if !ok {
			continue
		}
		return strings.Trim(value, "'")
	}
	t.Fatal("the generated script does not set WSL_CONFIG_SRC")
	return ""
}

// runShell writes script to a file and runs it through sh. Both the syntax
// check and the execution go through the same temporary file the TUI would hand
// to the shell.
func runShell(t *testing.T, script string) (string, error) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "wslconfig.sh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("writing the generated script: %v", err)
	}

	if out, err := exec.Command("sh", "-n", path).CombinedOutput(); err != nil {
		t.Fatalf("sh -n rejected the generated script: %v\n%s", err, out)
	}

	cmd := exec.Command("sh", path)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// TestWSLConfigInteractiveScriptPassesShellSyntaxCheck is the sh -n half of the
// issue #28 fix: the TUI runs the script through a shell, so a script the shell
// cannot parse is still a broken step even when the Go side compiles.
func TestWSLConfigInteractiveScriptPassesShellSyntaxCheck(t *testing.T) {
	// No WSLInterop entry, so the win32yank branch is skipped and no download is
	// attempted; the branch is still present in the text and still parsed.
	t.Setenv(envBinfmtDir, t.TempDir())
	repoDir, _, _ := newWSLLayout(t)
	m := wslModel(repoDir, true)

	script := generateWSLConfigScript(t, &m)

	path := filepath.Join(t.TempDir(), "wslconfig.sh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("sh", "-n", path).CombinedOutput(); err != nil {
		t.Fatalf("sh -n rejected the generated script: %v\n%s", err, out)
	}
}

// TestWSLConfigInteractiveScriptInstallsBothArtifacts runs the generated script
// against the same fixture the non-interactive step's tests use and checks the
// two files land, so the interactive path stops being the one that silently
// writes nothing.
func TestWSLConfigInteractiveScriptInstallsBothArtifacts(t *testing.T) {
	t.Setenv(envBinfmtDir, t.TempDir())
	repoDir, winHome, wslConf := newWSLLayout(t)
	m := wslModel(repoDir, true)

	script := generateWSLConfigScript(t, &m)
	if _, err := runShell(t, script); err != nil {
		t.Fatalf("the generated script failed: %v", err)
	}

	for _, tc := range []struct {
		name string
		path string
		want string
	}{
		{".wslconfig", filepath.Join(winHome, ".wslconfig"), renderedTestWSLConfig},
		{"wsl.conf", wslConf, "[boot]\nsystemd=true\n"},
	} {
		got, err := os.ReadFile(tc.path)
		if err != nil {
			t.Fatalf("%s was not installed at %s: %v", tc.name, tc.path, err)
		}
		if string(got) != tc.want {
			t.Errorf("%s content = %q, want %q", tc.name, got, tc.want)
		}
		info, err := os.Stat(tc.path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o644 {
			t.Errorf("%s mode = %o, want 644", tc.name, info.Mode().Perm())
		}
	}
}

// TestWSLConfigInteractiveScriptMatchesTheStep is the keep-the-two-in-step
// check: the step and the script are run against identical fixtures and their
// outputs must agree. If either side changes what it installs or where, this
// fails here instead of in the field, which is exactly how the second
// implementation behind #24 was caught.
func TestWSLConfigInteractiveScriptMatchesTheStep(t *testing.T) {
	t.Setenv(envBinfmtDir, t.TempDir())

	// Run the non-interactive step against the first fixture.
	stepRepo, stepWinHome, stepConf := newWSLLayout(t)
	stepModel := wslModel(stepRepo, true)
	if err := stepInstallWSLConfig(&stepModel); err != nil {
		t.Fatalf("stepInstallWSLConfig failed: %v", err)
	}

	// Run the interactive script against a second, identical fixture.
	scriptRepo, scriptWinHome, scriptConf := newWSLLayout(t)
	scriptModel := wslModel(scriptRepo, true)
	script := generateWSLConfigScript(t, &scriptModel)
	if _, err := runShell(t, script); err != nil {
		t.Fatalf("the generated script failed: %v", err)
	}

	assertSameFile(t, filepath.Join(stepWinHome, ".wslconfig"), filepath.Join(scriptWinHome, ".wslconfig"))
	assertSameFile(t, stepConf, scriptConf)
}

// TestWSLConfigInteractiveScriptBacksUpExistingFiles mirrors the step's backup
// contract, so the interactive path does not quietly overwrite a hand-written
// configuration.
func TestWSLConfigInteractiveScriptBacksUpExistingFiles(t *testing.T) {
	t.Setenv(envBinfmtDir, t.TempDir())
	repoDir, winHome, wslConf := newWSLLayout(t)

	existing := filepath.Join(winHome, ".wslconfig")
	if err := os.WriteFile(existing, []byte("[wsl2]\nmemory=32GB\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(wslConf, []byte("[boot]\nsystemd=false\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := wslModel(repoDir, true)
	script := generateWSLConfigScript(t, &m)
	if _, err := runShell(t, script); err != nil {
		t.Fatalf("the generated script failed: %v", err)
	}

	for _, dst := range []string{existing, wslConf} {
		backups, err := filepath.Glob(dst + ".bak-dotfiles-*")
		if err != nil {
			t.Fatal(err)
		}
		if len(backups) != 1 {
			t.Fatalf("expected one backup of %s, got %v", dst, backups)
		}
		if got, err := os.ReadFile(backups[0]); err != nil {
			t.Fatal(err)
		} else if string(got) == "" {
			t.Errorf("backup of %s is empty", dst)
		}
	}
}

// TestWSLConfigInteractiveScriptPassesShellcheck runs the linter the project
// uses for shell when it is installed. It skips where shellcheck is absent, so
// the suite still runs in a bare container, and it treats a warning as a
// failure because the generated script is run with the user's privileges.
func TestWSLConfigInteractiveScriptPassesShellcheck(t *testing.T) {
	shellcheck, err := exec.LookPath("shellcheck")
	if err != nil {
		t.Skip("shellcheck is not installed")
	}

	t.Setenv(envBinfmtDir, t.TempDir())
	repoDir, _, _ := newWSLLayout(t)
	m := wslModel(repoDir, true)
	script := generateWSLConfigScript(t, &m)

	path := filepath.Join(t.TempDir(), "wslconfig.sh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(shellcheck, "--severity=warning", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("shellcheck rejected the generated script: %v\n%s", err, out)
	}
}

// TestWSLConfigInteractiveScriptSkipsWindowsSideWhenProfileUnavailable mirrors
// the step's contract for a host where interop is off and no mounted profile can
// be found: the Windows half is skipped and the in-distribution half still runs.
func TestWSLConfigInteractiveScriptSkipsWindowsSideWhenProfileUnavailable(t *testing.T) {
	t.Setenv(envBinfmtDir, t.TempDir())
	repoDir, _, wslConf := newWSLLayout(t)
	// A non-existent override makes windowsUserProfile refuse without running
	// cmd.exe, so the test never depends on the host's real Windows profile.
	t.Setenv(envWSLWindowsHome, filepath.Join(t.TempDir(), "does-not-exist"))

	m := wslModel(repoDir, true)
	script := generateWSLConfigScript(t, &m)
	if _, err := runShell(t, script); err != nil {
		t.Fatalf("a missing Windows profile must not fail the script: %v", err)
	}
	if _, err := os.Stat(wslConf); err != nil {
		t.Errorf("wsl.conf was not installed: %v", err)
	}
}

// TestWSLConfigInteractiveScriptFailsWhenArtifactIsMissing mirrors the step's
// failure contract: a missing or unrenderable template is reported, not turned
// into a script that would install nothing.
func TestWSLConfigInteractiveScriptFailsWhenArtifactIsMissing(t *testing.T) {
	t.Setenv(envBinfmtDir, t.TempDir())
	repoDir, _, _ := newWSLLayout(t)
	if err := os.Remove(filepath.Join(repoDir, repoAssetWSLConfig)); err != nil {
		t.Fatal(err)
	}

	m := wslModel(repoDir, true)
	if _, err := getInteractiveScript(wslStepID, &m); err == nil {
		t.Fatal("a missing template must be reported instead of installing nothing")
	}
}

// TestWSLConfigRoutesRenderIdenticalContent pins the two routes against one
// overridden host: the bytes the step installs and the bytes the generated
// script is pointed at must come from the same render. It inspects the script's
// source file rather than its installed output, so the content is checked even
// before the shell runs.
func TestWSLConfigRoutesRenderIdenticalContent(t *testing.T) {
	t.Setenv(envBinfmtDir, t.TempDir())

	stepRepo, stepWinHome, _ := newWSLLayout(t)
	// Pin one deterministic host for both routes, after the fixture so the plan
	// does not depend on the machine running the test: 16 GiB and 8 CPUs become
	// 8192 MB, 8 processors and 2048 MB of swap.
	t.Setenv(envWSLHostMemoryMB, "16384")
	t.Setenv(envWSLHostCPUs, "8")

	stepModel := wslModel(stepRepo, true)
	if err := stepInstallWSLConfig(&stepModel); err != nil {
		t.Fatalf("stepInstallWSLConfig failed: %v", err)
	}
	installed, err := os.ReadFile(filepath.Join(stepWinHome, ".wslconfig"))
	if err != nil {
		t.Fatalf("the step did not install .wslconfig: %v", err)
	}

	scriptRepo, _, _ := newWSLLayout(t)
	t.Setenv(envWSLHostMemoryMB, "16384")
	t.Setenv(envWSLHostCPUs, "8")

	scriptModel := wslModel(scriptRepo, true)
	script := generateWSLConfigScript(t, &scriptModel)
	rendered, err := os.ReadFile(wslConfigSourceFromScript(t, script))
	if err != nil {
		t.Fatalf("reading the rendered config the script copies from: %v", err)
	}

	if string(installed) != string(rendered) {
		t.Errorf("the step and the interactive script do not install the same bytes:\nstep:   %q\nscript: %q",
			installed, rendered)
	}
	for _, want := range []string{"memory=8192MB", "processors=8", "swap=2048MB"} {
		if !strings.Contains(string(installed), want) {
			t.Errorf("both routes should render %q for the overridden host, got:\n%s", want, installed)
		}
	}
}

// assertSameFile fails the test unless both paths hold the same bytes.
func assertSameFile(t *testing.T, want, got string) {
	t.Helper()

	wantData, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("reading %s: %v", want, err)
	}
	gotData, err := os.ReadFile(got)
	if err != nil {
		t.Fatalf("reading %s: %v", got, err)
	}
	if string(wantData) != string(gotData) {
		t.Errorf("the step and the interactive script disagree about %s:\nstep:   %q\nscript: %q",
			filepath.Base(got), wantData, gotData)
	}
}

// TestWSLConfigInteractiveScriptKeepsWin32YankPinned ties the clipboard bridge
// in the script to the constants the Go step downloads, so the two cannot drift
// apart on the URL or the checksum the installer trusts.
func TestWSLConfigInteractiveScriptKeepsWin32YankPinned(t *testing.T) {
	t.Setenv(envBinfmtDir, t.TempDir())
	repoDir, _, _ := newWSLLayout(t)
	m := wslModel(repoDir, true)

	script := generateWSLConfigScript(t, &m)

	for _, want := range []string{win32yankArchive, win32yankSHA256} {
		if !strings.Contains(script, want) {
			t.Errorf("the interactive script does not carry the pinned win32yank value %q", want)
		}
	}
}
