package tui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
)

func TestFishInstallMergingCopyKeepsPreservedUserConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	withPackageCommandMocks(t, nil)

	userConfig := "# user's exact fish settings\nalias ll='ls -lah'\n"
	configPath := filepath.Join(home, ".config", "fish", "config.fish")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(userConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	m := NewModel()
	m.SystemInfo = &system.SystemInfo{OS: system.OSMac, HasBrew: true}
	m.Choices = UserChoices{OS: "mac", Shell: "fish", WindowMgr: "none"}
	m.RepoDir = repoRoot(t)

	if err := stepInstallShell(&m); err != nil {
		t.Fatalf("Fish install step failed: %v", err)
	}

	preservedPath := filepath.Join(home, ".config", "fish", "dotfiles.d")
	matches, err := filepath.Glob(filepath.Join(preservedPath, "dotfiles-user-config-*.fish"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("preserved drop-ins = %v, want exactly one", matches)
	}
	got, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read preserved Fish config: %v", err)
	}
	if string(got) != userConfig {
		t.Errorf("preserved Fish config = %q, want exact user bytes %q", got, userConfig)
	}

	// The Fish tree is installed with CopyDir's merging semantics. A pruning
	// copy would delete dotfiles.d because it is intentionally absent from the
	// shipped tree, taking the user's preserved settings with it.
	shippedConfig, err := os.ReadFile(filepath.Join(m.RepoDir, repoAssetFish, "config.fish"))
	if err != nil {
		t.Fatal(err)
	}
	installedConfig, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	loader := `for _dotfiles_user_config in "$HOME"/.config/fish/dotfiles.d/dotfiles-user-config-*.fish`
	if !strings.Contains(string(shippedConfig), loader) {
		t.Fatalf("shipped config.fish does not source Fish user drop-ins: missing %q", loader)
	}
	if !strings.Contains(string(installedConfig), loader) {
		t.Errorf("installed config.fish does not source the drop-in directory")
	}
	if strings.Index(string(shippedConfig), loader) < strings.LastIndex(string(shippedConfig), "set -g fish_pager_color_description") {
		t.Errorf("Fish user drop-in loader is not at the end of the shipped config")
	}
}

func TestShippedShellConfigOwnershipAndDropInLoading(t *testing.T) {
	root := filepath.Join("..", "..", "..")

	zshrc, err := os.ReadFile(filepath.Join(root, "dotfiles-zsh", ".zshrc"))
	if err != nil {
		t.Fatal(err)
	}
	zsh := string(zshrc)
	for _, line := range []string{
		"# dotfiles-managed-config: zsh",
		`for _dotfiles_user_config in "$HOME"/.zshrc.d/*.zsh(N.); do`,
		`source "$_dotfiles_user_config"`,
	} {
		if !strings.Contains(zsh, line) {
			t.Errorf("shipped .zshrc is missing %q", line)
		}
	}
	wmCall := strings.Index(zsh, "start_if_needed\n")
	instantPrompt := strings.Index(zsh, "# Enable Powerlevel10k instant prompt")
	userLoader := strings.Index(zsh, `for _dotfiles_user_config in "$HOME"/.zshrc.d/*.zsh(N.); do`)
	if wmCall < 0 || instantPrompt < 0 || wmCall > instantPrompt {
		t.Errorf("window-manager launcher must remain before the instant prompt: launcher=%d prompt=%d", wmCall, instantPrompt)
	}
	if userLoader < strings.Index(zsh, "# To customize prompt") {
		t.Errorf("user drop-ins must load near the end of .zshrc after managed configuration")
	}

	fishConfig, err := os.ReadFile(filepath.Join(root, "dotfiles-fish", "fish", "config.fish"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(fishConfig), "# dotfiles-managed-config: fish") {
		t.Error("shipped config.fish is missing its ownership marker")
	}
}

// The Ghostty step downloads an install script with curl, and the reported
// defect was the same command-substitution idiom the Homebrew step already
// fixed: a failed curl expanded to an empty command, the shell ran nothing and
// exited 0, and the step copied configuration under a success line. These tests
// stub curl on PATH and drive the step's real command path, so no test touches
// the network.
const (
	ghosttyStubLogEnv  = "GHOSTTY_STEP_STUB_LOG"
	ghosttyStubExitEnv = "GHOSTTY_STEP_STUB_CURL_EXIT"
)

// ghosttyStubCurlScript records its arguments and, when the download is meant
// to succeed, writes the install script curl would have fetched. The written
// script records that it ran, which proves the step executed the download rather
// than an empty substitution.
var ghosttyStubCurlScript = fmt.Sprintf(`#!/bin/sh
echo "curl $*" >> "$%s"
out=""
prev=""
for arg in "$@"; do
  if [ "$prev" = "-o" ]; then out="$arg"; fi
  prev="$arg"
done
if [ "$%s" = "0" ] && [ -n "$out" ]; then
  printf '#!/bin/sh\nprintf "ghostty-install-script-ran\n" >> "$%s"\n' > "$out"
fi
exit "$%s"
`, ghosttyStubLogEnv, ghosttyStubExitEnv, ghosttyStubLogEnv, ghosttyStubExitEnv)

// stubGhosttyCommands installs the curl stub and points PATH at it, so the
// step's own download call is the thing under test and the host's Ghostty (and
// curl) cannot leak in.
func stubGhosttyCommands(t *testing.T, curlExit int) string {
	t.Helper()

	logPath := filepath.Join(t.TempDir(), "ghostty-step-commands.log")
	t.Setenv(ghosttyStubLogEnv, logPath)
	t.Setenv(ghosttyStubExitEnv, fmt.Sprintf("%d", curlExit))

	binDir := t.TempDir()
	writeStubCommand(t, binDir, "curl", ghosttyStubCurlScript)
	t.Setenv("PATH", strings.Join([]string{binDir, "/usr/bin", "/bin"}, string(os.PathListSeparator)))

	return logPath
}

// stageGhosttyCheckout stages the one asset the step copies after the install,
// so a run that should fail has to fail at the download and not at the config
// copy.
func stageGhosttyCheckout(t *testing.T) string {
	t.Helper()

	checkout := t.TempDir()
	assetDir := filepath.Join(checkout, repoAssetGhostty)
	if err := os.MkdirAll(assetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assetDir, "config"), []byte("ghostty config\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return checkout
}

func linuxGhosttyModel(repoDir string) Model {
	m := NewModel()
	m.SystemInfo = &system.SystemInfo{OS: system.OSLinux}
	m.Choices = UserChoices{OS: "linux", Terminal: "ghostty"}
	m.RepoDir = repoDir
	return m
}

// verboseTerminalStep makes the step's log observable on stdout, the way the
// installer prints it in non-interactive verbose runs, so a success line after
// a failed download cannot hide.
func verboseTerminalStep(t *testing.T) func() string {
	t.Helper()

	t.Setenv("DOTFILES_VERBOSE", "1")
	SetNonInteractiveMode(true)
	t.Cleanup(func() { SetNonInteractiveMode(false) })
	return captureStepStdout(t)
}

// TestStepInstallTerminalGhosttyReportsAFailedDownload is the reported defect:
// curl failed (an HTTP error in the field), its status disappeared inside the
// command substitution, and the step still reported a configured Ghostty. A
// failed download has to be an error, no install script may run, and no success
// line may be printed.
func TestStepInstallTerminalGhosttyReportsAFailedDownload(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	readLog := verboseTerminalStep(t)
	log := stubGhosttyCommands(t, 22)
	m := linuxGhosttyModel(stageGhosttyCheckout(t))

	err := stepInstallTerminal(&m)
	if err == nil {
		t.Fatal("a failed download must not be reported as a successful install")
	}

	var stepErr *StepError
	if !errors.As(err, &stepErr) || stepErr.StepID != "terminal" {
		t.Fatalf("the failure is not reported as a terminal step failure: %v", err)
	}
	if !strings.Contains(err.Error(), "Failed to download") {
		t.Errorf("the failure does not name the download: %v", err)
	}
	t.Logf("the failed-download step returned: %v", err)

	if logRanInstallerScript(t, log) {
		t.Error("the step ran the install script after the download failed")
	}
	if out := readLog(); strings.Contains(out, "✓ Ghostty configured") {
		t.Errorf("a failed download still reported Ghostty as configured: %q", out)
	}
	if _, statErr := os.Stat(filepath.Join(os.Getenv("HOME"), ".config", "ghostty", "config")); !os.IsNotExist(statErr) {
		t.Errorf("a failed install still copied the Ghostty configuration: %v", statErr)
	}
}

// TestStepInstallTerminalGhosttyRunsTheDownloadItFetches is the positive
// control: the two-command download must still install on a successful fetch,
// run the script it downloaded, and report success.
func TestStepInstallTerminalGhosttyRunsTheDownloadItFetches(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	readLog := verboseTerminalStep(t)
	log := stubGhosttyCommands(t, 0)
	m := linuxGhosttyModel(stageGhosttyCheckout(t))

	if err := stepInstallTerminal(&m); err != nil {
		t.Fatalf("a successful Ghostty install must still report success: %v", err)
	}
	if !logRanInstallerScript(t, log) {
		t.Error("the step did not run the script it downloaded")
	}
	if out := readLog(); !strings.Contains(out, "✓ Ghostty configured") {
		t.Errorf("a successful install no longer reports success: %q", out)
	}
	if _, statErr := os.Stat(filepath.Join(os.Getenv("HOME"), ".config", "ghostty", "config")); statErr != nil {
		t.Errorf("a successful install did not copy the configuration: %v", statErr)
	}
}

func logRanInstallerScript(t *testing.T, logPath string) bool {
	t.Helper()

	data, err := os.ReadFile(logPath)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "ghostty-install-script-ran")
}

// TestInteractiveGhosttyScriptFailsWhenTheDownloadFails covers the interactive
// path, which used `curl ... | bash`: a failed curl left the consumer shell with
// an empty script, `set -e` had nothing to abort on, and the step reported
// success. The generated script is executed against a stub curl so the failure
// is observed, not asserted from the text.
func TestInteractiveGhosttyScriptFailsWhenTheDownloadFails(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	stubGhosttyCommands(t, 22)

	m := &Model{
		SystemInfo: &system.SystemInfo{OS: system.OSLinux},
		Choices:    UserChoices{OS: "linux", Terminal: "ghostty"},
		RepoDir:    stageGhosttyCheckout(t),
	}
	script, err := getTerminalScript(m)
	if err != nil {
		t.Fatalf("getTerminalScript(ghostty) failed: %v", err)
	}

	out, err := runInteractiveScript(t, script)
	if err == nil {
		t.Fatalf("a failed download still reported success; output:\n%s", out)
	}
	if strings.Contains(out, "ghostty configured!") {
		t.Errorf("a failed download still reported Ghostty as configured:\n%s", out)
	}
}

// TestInteractiveGhosttyScriptDownloadsThenRuns pins the shape that keeps
// curl's exit status: the script downloads to a file and then runs that file.
// The command-substitution and pipe forms are what this test exists to reject,
// because both discard curl's status.
func TestInteractiveGhosttyScriptDownloadsThenRuns(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	stubGhosttyCommands(t, 0)

	m := &Model{
		SystemInfo: &system.SystemInfo{OS: system.OSLinux},
		Choices:    UserChoices{OS: "linux", Terminal: "ghostty"},
		RepoDir:    stageGhosttyCheckout(t),
	}
	script, err := getTerminalScript(m)
	if err != nil {
		t.Fatalf("getTerminalScript(ghostty) failed: %v", err)
	}
	if strings.Contains(script, "| bash") {
		t.Errorf("the interactive Ghostty install still pipes curl into bash, discarding its exit status:\n%s", script)
	}
	if !strings.Contains(script, "curl -fsSL") || !strings.Contains(script, "-o ") {
		t.Errorf("the interactive Ghostty script does not download to a file:\n%s", script)
	}

	out, err := runInteractiveScript(t, script)
	if err != nil {
		t.Fatalf("a successful download must run cleanly: %v\n%s", err, out)
	}
	if !strings.Contains(out, "ghostty configured!") {
		t.Errorf("a successful download no longer reports success:\n%s", out)
	}
}

// runInteractiveScript executes a generated interactive script the way the step
// runner does -- through a shell with stdin attached -- and returns its combined
// output. The newline answers the script's closing prompt so the run's exit
// status reflects the install, not the prompt's EOF.
func runInteractiveScript(t *testing.T, script string) (string, error) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "interactive.sh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("/bin/sh", path)
	cmd.Stdin = strings.NewReader("\n")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// TestStepInstallWMTmuxReportsFailedPluginInstall covers the discarded result at
// installer.go:2255-2257: TPM's plugin installer failed and the step still
// logged "✓ Tmux configured". TPM is staged locally so the check is reached
// without a network clone, and its plugin installer is the thing that fails.
// The step's other work is done and tmux retries the plugins at startup, so the
// failure is reported as a warning and the step still completes; what must not
// happen is the success line.
func TestStepInstallWMTmuxReportsFailedPluginInstall(t *testing.T) {
	readLog := verboseTerminalStep(t)

	home := t.TempDir()
	t.Setenv("HOME", home)

	// tmux counts as installed, so the step's job under test is the plugin run
	// rather than a package install.
	binDir := t.TempDir()
	writeStubCommand(t, binDir, "tmux", "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", strings.Join([]string{binDir, "/usr/bin", "/bin"}, string(os.PathListSeparator)))

	tpmBin := filepath.Join(home, ".tmux", "plugins", "tpm", "bin")
	if err := os.MkdirAll(tpmBin, 0o755); err != nil {
		t.Fatal(err)
	}
	installPlugins := filepath.Join(tpmBin, "install_plugins")
	if err := os.WriteFile(installPlugins, []byte("#!/bin/sh\necho 'tpm: could not reach the plugin repositories' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	m := NewModel()
	m.SystemInfo = &system.SystemInfo{OS: system.OSMac, HasBrew: true}
	m.Choices = UserChoices{OS: "mac", Shell: "zsh", WindowMgr: "tmux"}
	m.RepoDir = repoRoot(t)

	if err := stepInstallWM(&m); err != nil {
		t.Fatalf("a failed Tmux plugin install must not abort the step: %v", err)
	}

	out := readLog()
	if !strings.Contains(out, "could not install the Tmux plugins") {
		t.Errorf("a failed plugin install was not reported: %q", out)
	}
	if strings.Contains(out, "✓ Tmux configured") {
		t.Errorf("a failed plugin install still reported Tmux as configured: %q", out)
	}
}
