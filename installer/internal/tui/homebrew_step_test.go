package tui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// The Homebrew step shells out to curl for the upstream install script, and the
// reported defect was about curl's exit status disappearing inside a command
// substitution. These tests therefore put a stub curl first on PATH and drive
// the step's real command path -- real shell, real temporary file, real download
// call -- instead of mocking the step away. No test touches the network.
const (
	brewStubLogEnv      = "HOMEBREW_STEP_STUB_LOG"
	brewStubCurlExitEnv = "HOMEBREW_STEP_STUB_CURL_EXIT"
	brewStubScriptEnv   = "HOMEBREW_STEP_STUB_SCRIPT"
)

// brewStubCurlScript records its arguments and, when the download is meant to
// succeed, writes the install script curl would have fetched. The script body is
// built by the test, so it can append a marker to the command log -- proving the
// step really executed the download rather than an empty substitution -- and, in
// the success case, materialize a brew on PATH so the post-install check finds
// it. The stub writes the body verbatim; nothing in it is reinterpreted here.
var brewStubCurlScript = fmt.Sprintf(`#!/bin/sh
echo "curl $*" >> "$%s"
out=""
prev=""
for arg in "$@"; do
  if [ "$prev" = "-o" ]; then out="$arg"; fi
  prev="$arg"
done
if [ "$%s" = "0" ] && [ -n "$out" ]; then
  printf '%%s' "$%s" > "$out"
fi
exit "$%s"
`, brewStubLogEnv, brewStubCurlExitEnv, brewStubScriptEnv, brewStubCurlExitEnv)

type brewStepStubs struct {
	log string
}

// stubBrewCommands installs the curl stub and pins the environment so that
// system.BrewInstalled is deterministic: HOMEBREW_PREFIX points at an empty
// directory and PATH is limited to the stub directory plus the system bin
// directories that hold the shell and the stub curl. That keeps the developer's
// own Homebrew off PATH. When withBrew is set, the downloaded script itself
// creates brew on PATH, so the post-install check can succeed.
func stubBrewCommands(t *testing.T, curlExit int, withBrew bool) *brewStepStubs {
	t.Helper()

	t.Setenv("HOME", t.TempDir())

	logPath := filepath.Join(t.TempDir(), "homebrew-step-commands.log")
	t.Setenv(brewStubLogEnv, logPath)
	t.Setenv(brewStubCurlExitEnv, fmt.Sprintf("%d", curlExit))
	t.Setenv("HOMEBREW_PREFIX", filepath.Join(t.TempDir(), "no-homebrew-here"))

	binDir := t.TempDir()

	// The downloaded script always records that it ran. In the success case it
	// also creates brew on PATH, so BrewInstalled is false before the step and
	// true after it, which is exactly what the post-install check is watching.
	script := "#!/bin/sh\n" + fmt.Sprintf("printf 'install-script-ran\\n' >> %q\n", logPath)
	if withBrew {
		brewPath := filepath.Join(binDir, "brew")
		script += fmt.Sprintf("printf '#!/bin/sh\\nexit 0\\n' > %q\nchmod +x %q\n", brewPath, brewPath)
	}
	t.Setenv(brewStubScriptEnv, script)

	writeStubCommand(t, binDir, "curl", brewStubCurlScript)
	t.Setenv("PATH", strings.Join([]string{binDir, "/usr/bin", "/bin"}, string(os.PathListSeparator)))

	return &brewStepStubs{log: logPath}
}

func (s *brewStepStubs) commands(t *testing.T) []string {
	t.Helper()

	data, err := os.ReadFile(s.log)
	if err != nil {
		return nil
	}

	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func (s *brewStepStubs) ranInstallScript(t *testing.T) bool {
	t.Helper()

	for _, line := range s.commands(t) {
		if line == "install-script-ran" {
			return true
		}
	}
	return false
}

// downloadedInstallerPath is the -o path the step handed curl, read back from
// the command the step really ran. The path is quoted in the command, so the
// surrounding quotes are stripped before it is returned.
func (s *brewStepStubs) downloadedInstallerPath(t *testing.T) string {
	t.Helper()

	for _, line := range s.commands(t) {
		if !strings.HasPrefix(line, "curl ") {
			continue
		}
		args := strings.Fields(line)
		for i, arg := range args {
			if arg == "-o" && i+1 < len(args) {
				return strings.Trim(args[i+1], `"`)
			}
		}
	}
	return ""
}

func linuxBrewModel() Model {
	m := NewModel()
	m.SystemInfo = &system.SystemInfo{OS: system.OSLinux}
	m.Choices = UserChoices{OS: "linux"}
	return m
}

// verboseHomebrewStep makes the step's log observable on stdout, the way the
// installer prints it in non-interactive verbose runs. Without it the reported
// symptom -- a "✓ Homebrew installed successfully" line after a failed download
// -- would not reach stdout.
func verboseHomebrewStep(t *testing.T) func() string {
	t.Helper()

	t.Setenv("DOTFILES_VERBOSE", "1")
	SetNonInteractiveMode(true)
	t.Cleanup(func() { SetNonInteractiveMode(false) })
	return captureStepStdout(t)
}

// TestStepInstallHomebrewReportsAFailedDownload is the reported defect: the
// download failed (HTTP 403 in the E2E container), curl's status disappeared
// inside the command substitution, and the step still claimed success. A failed
// download has to be an error, no install script may run, and no success line
// may be printed.
func TestStepInstallHomebrewReportsAFailedDownload(t *testing.T) {
	readLog := verboseHomebrewStep(t)
	stubs := stubBrewCommands(t, 22, false)
	m := linuxBrewModel()

	err := stepInstallHomebrew(&m)
	if err == nil {
		t.Fatal("a failed download must not be reported as a successful install")
	}

	var stepErr *StepError
	if !errors.As(err, &stepErr) || stepErr.StepID != "homebrew" {
		t.Fatalf("the failure is not reported as a homebrew step failure: %v", err)
	}
	if !strings.Contains(err.Error(), "Failed to download the Homebrew install script") {
		t.Errorf("the failure does not name the download: %v", err)
	}
	t.Logf("the failed-download step returned: %v", err)
	if m.SystemInfo.HasBrew {
		t.Error("a failed download must not mark Homebrew as installed")
	}
	if stubs.ranInstallScript(t) {
		t.Error("the step ran the install script after the download failed")
	}

	// The temporary file belongs to the step and must not survive any path out.
	if installer := stubs.downloadedInstallerPath(t); installer != "" {
		if _, statErr := os.Stat(installer); !os.IsNotExist(statErr) {
			t.Errorf("the temporary installer %s survived the failed step", installer)
		}
	}

	if log := readLog(); strings.Contains(log, "✓ Homebrew installed successfully") {
		t.Errorf("a failed download still reported success: %q", log)
	}
}

// TestStepInstallHomebrewFailsWhenBrewIsMissingAfterInstall covers the other
// half of the defect: the install script exited 0 but left no brew behind, and
// the step logged a warning under a success report. That is a failure, and the
// message has to name the likely cause.
func TestStepInstallHomebrewFailsWhenBrewIsMissingAfterInstall(t *testing.T) {
	readLog := verboseHomebrewStep(t)
	stubs := stubBrewCommands(t, 0, false)
	m := linuxBrewModel()

	err := stepInstallHomebrew(&m)
	if err == nil {
		t.Fatal("an install that left no brew behind must not be reported as successful")
	}

	var stepErr *StepError
	if !errors.As(err, &stepErr) || stepErr.StepID != "homebrew" {
		t.Fatalf("the failure is not reported as a homebrew step failure: %v", err)
	}
	if !strings.Contains(err.Error(), "still missing after the install script ran") {
		t.Errorf("the failure does not name the missing Homebrew afterwards: %v", err)
	}
	if m.SystemInfo.HasBrew {
		t.Error("the step marked Homebrew as installed although brew is missing")
	}
	if !stubs.ranInstallScript(t) {
		t.Error("the download succeeded, so the install script should have run")
	}
	if log := readLog(); strings.Contains(log, "✓ Homebrew installed successfully") {
		t.Errorf("a missing Homebrew still reported success: %q", log)
	}
}

// TestStepInstallHomebrewSucceedsWhenBrewAppears is the positive control: the
// new post-install check and the two-command download must still leave a real
// successful install reporting success and refreshing SystemInfo.HasBrew.
func TestStepInstallHomebrewSucceedsWhenBrewAppears(t *testing.T) {
	readLog := verboseHomebrewStep(t)
	stubs := stubBrewCommands(t, 0, true)
	m := linuxBrewModel()

	if err := stepInstallHomebrew(&m); err != nil {
		t.Fatalf("a successful Homebrew install must still report success: %v", err)
	}
	if !m.SystemInfo.HasBrew {
		t.Error("a successful install must refresh SystemInfo.HasBrew")
	}
	if !stubs.ranInstallScript(t) {
		t.Error("the step did not run the script it downloaded")
	}
	if log := readLog(); !strings.Contains(log, "✓ Homebrew installed successfully") {
		t.Errorf("a successful install no longer reports success: %q", log)
	}
}
