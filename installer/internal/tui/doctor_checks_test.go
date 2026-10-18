package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// ============================================================================
// A machine with nothing wrong with it
// ============================================================================

// doctorHealthyHost is a machine the doctor has no complaint about: every
// directory it expects is there with a mode a person would use, every tool it
// manages answers its own version command, the shells' own syntax check passes,
// and the environment variables hold values that mean something. It is the
// other half of every alarm: a check that fires here is a check that cries wolf.
func doctorHealthyHost() *doctorFakeHost {
	const home = "/home/tester"
	h := newDoctorFakeHost()
	h.setenv("PATH", "/usr/bin:/bin")
	h.setenv("SHELL", "/usr/bin/zsh")
	h.setenv("LANG", "en_US.UTF-8")
	h.setenv("XDG_CONFIG_HOME", home+"/.config")
	h.setenv("XDG_DATA_HOME", home+"/.local/share")
	h.setenv("XDG_STATE_HOME", home+"/.local/state")
	h.setenv("XDG_CACHE_HOME", home+"/.cache")
	h.setenv("TMPDIR", home+"/.cache/tmp")

	// The directories PATH and the checks name, and the executables they ask for.
	for _, dir := range []string{"/usr/bin", "/bin", home, home + "/.config", home + "/.local", home + "/.local/share",
		home + "/.local/state", home + "/.cache", home + "/.cache/tmp", home + "/.config/nvim"} {
		mode := os.FileMode(0o755)
		if dir == home+"/.cache/tmp" {
			mode = 0o700
		}
		h.addFile(dir, mode, true, "")
	}
	for _, command := range []string{"/bin/sh", "/bin/bash", "/usr/bin/sudo", "/usr/bin/df", "/usr/bin/infocmp", "/usr/bin/ssh-add"} {
		h.addFile(command, 0o755, false, "")
	}
	for _, tool := range doctorManagedTools() {
		h.addFile("/usr/bin/"+tool.Name, 0o755, false, "")
		h.addCommand(strings.Join(append([]string{"/usr/bin/" + tool.Name}, tool.VersionArgs...), " "), tool.Name+" 1.0.0\n")
	}

	// The ssh material, with the modes ssh itself requires.
	h.addFile(home+"/.ssh", 0o700, true, "")
	h.addText(home+"/.ssh/id_ed25519", 0o600, "-----BEGIN OPENSSH PRIVATE KEY-----\nkey\n-----END OPENSSH PRIVATE KEY-----\n")
	h.addText(home+"/.ssh/id_ed25519.pub", 0o644, "ssh-ed25519 AAAAC3 tester@example\n")
	h.addText(home+"/.ssh/config", 0o600, "Host *\n  AddKeysToAgent yes\n")

	// The zsh configuration this installer writes, with an include that resolves.
	h.addText(home+"/.zshenv", 0o644, "export PATH=\"$HOME/.local/bin:$PATH\"\n")
	h.addText(home+"/.zshrc", 0o644, "source ~/.p10k.zsh\nalias ll='eza -l'\n")
	h.addText(home+"/.p10k.zsh", 0o644, "# the prompt\n")
	h.addFile(home+"/.oh-my-zsh", 0o755, true, "")
	h.addText(home+"/.oh-my-zsh/oh-my-zsh.sh", 0o644, "# the framework\n")
	h.addText(home+"/.bashrc", 0o644, "alias ll='eza -l'\n")
	h.addText(home+"/.config/nvim/init.lua", 0o644, "vim.opt.number = true\n")

	// Git, its identity and the agent.
	h.addCommand("/usr/bin/git --version", "git version 2.44.0")
	h.addCommand("/usr/bin/git config --global --get user.name", "Tester\n")
	h.addCommand("/usr/bin/git config --global --get user.email", "tester@example.com\n")
	h.addCommand("/usr/bin/git config --global --get credential.helper", "osxkeychain\n")
	h.addCommandResult("/usr/bin/git rev-parse --show-toplevel", doctorCommandResult{Code: 128, Err: "exited with status 128"})

	// The shells' own syntax check, and the four commands the environment needs.
	h.addCommand("/usr/bin/zsh -n "+home+"/.zshenv", "")
	h.addCommand("/usr/bin/zsh -n "+home+"/.zshrc", "")
	h.addCommand("/usr/bin/zsh -n "+home+"/.p10k.zsh", "")
	h.addCommand("/usr/bin/zsh -l -c printf %s \"$PATH\"", "/usr/bin:/bin")
	h.addCommand("/usr/bin/df -Pk "+home, "Filesystem 1024-blocks Used Available Capacity Mounted on\n/dev/sda1 100000000 40000000 60000000 40% /\n")
	h.addCommand("/bin/sh -c ulimit -n", "4096\n")
	h.addCommand("/usr/bin/infocmp xterm-256color", "xterm-256color|xterm with 256 colors,\n")
	return h
}

// TestAHealthyMachineRaisesNoAlarm is the strongest statement this suite can
// make about the doctor's usefulness: on a machine where everything is in place,
// the whole catalogue reports no warning and no error. Every guard about a
// finding is worthless if the doctor also shouts about a machine that is fine.
func TestAHealthyMachineRaisesNoAlarm(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	host := doctorHealthyHost()
	host.setenv("MY_SECRET", "not-a-secret-the-doctor-needs")
	scan := doctorTestScan(host, doctorTestLinuxInfo())
	scan.procRootDir = "/proc"
	doctorParentPID = func() int { return 4242 }
	t.Cleanup(func() { doctorParentPID = os.Getppid })
	host.addText("/proc/4242/comm", 0o444, "zsh\n")

	var complaints []string
	for _, check := range doctorChecks() {
		result := runDoctorCheck(scan, check)
		for _, finding := range result.Findings {
			if finding.Status == doctorWarning || finding.Status == doctorError {
				complaints = append(complaints, "    "+finding.Status.String()+" · "+finding.Subject+" — "+finding.Evidence)
			}
		}
		if result.Status == doctorWarning || result.Status == doctorError {
			t.Logf("%s reports %s", result.ID, result.Status)
		}
	}
	if len(complaints) > 0 {
		t.Errorf("a machine with nothing wrong with it produced %d complaint(s):\n%s", len(complaints), strings.Join(complaints, "\n"))
	}
}

// ============================================================================
// PATH
// ============================================================================

// TestPathEntriesClassifiesEveryWayAnEntryCanBeWrong is S2: an unset PATH is an
// error, and each entry that cannot resolve, is written in a way the shell does
// not expand, or is named twice is a warning that says which entry it is.
func TestPathEntriesClassifiesEveryWayAnEntryCanBeWrong(t *testing.T) {
	cases := []struct {
		name     string
		path     string
		status   doctorStatus
		mentions string
	}{
		{"an unset PATH is an error, not a warning", "", doctorError, "PATH is unset"},
		{"an entry that is not there", "/usr/bin:/nowhere/bin", doctorWarning, "PATH entry 2 of 2: /nowhere/bin"},
		{"a tilde the shell does not expand", "~/bin:/usr/bin", doctorWarning, "does not expand inside PATH"},
		{"a variable reference left in the entry", "$HOME/.local/bin:/usr/bin", doctorWarning, "variable reference"},
		{"the same directory twice", "/usr/bin:/bin:/usr/bin", doctorWarning, "already named at position 1"},
		{"an empty entry", "/usr/bin::/bin", doctorWarning, "the entry is empty"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			host := newDoctorFakeHost()
			host.addDir("/usr/bin")
			host.addDir("/bin")
			host.addDir("/home/tester")
			host.setenv("PATH", tc.path)

			findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "path.entries")
			if got := doctorWorst(findings); got != tc.status {
				t.Errorf("the check reports %s, want %s:\n%s", got, tc.status, doctorFindingSummary(findings))
			}
			finding := doctorFindingWith(t, findings, tc.mentions)
			if finding.Evidence == "" || finding.Impact == "" || finding.Advice == "" {
				t.Errorf("the finding for %q does not state its evidence, impact and advice: %+v", tc.mentions, finding)
			}
		})
	}
}

// TestPathEntriesNamesADirectoryWithoutSearchPermission covers the third way an
// entry fails: it is there and cannot be searched, which is a warning with the
// mode as its evidence rather than "not found".
func TestPathEntriesNamesADirectoryWithoutSearchPermission(t *testing.T) {
	host := newDoctorFakeHost()
	host.addDir("/usr/bin")
	host.addFile("/locked", 0o600, true, "")
	host.setenv("PATH", "/usr/bin:/locked")

	findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "path.entries")
	finding := doctorFindingWith(t, findings, "/locked")
	if finding.Status != doctorWarning {
		t.Errorf("a directory with no search permission is reported as %s", finding.Status)
	}
	if !strings.Contains(finding.Evidence, "0600") {
		t.Errorf("the finding does not say what the mode is: %q", finding.Evidence)
	}
}

// TestPathShadowedComparesTheCopiesThatRun is S2's precedence rule: two copies of
// a managed tool are a warning that names the difference, one copy is stated as
// ok, and copies whose version cannot be read are unknown rather than assumed
// equal.
func TestPathShadowedComparesTheCopiesThatRun(t *testing.T) {
	base := func() *doctorFakeHost {
		host := newDoctorFakeHost()
		host.setenv("PATH", "/home/tester/.local/bin:/usr/bin:/bin")
		host.addFile("/home/tester/.local/bin", 0o755, true, "")
		host.addDir("/usr/bin")
		host.addDir("/bin")
		return host
	}

	t.Run("two copies with two versions", func(t *testing.T) {
		host := base()
		host.addFile("/home/tester/.local/bin/git", 0o755, false, "")
		host.addFile("/usr/bin/git", 0o755, false, "")
		host.addCommand("/home/tester/.local/bin/git --version", "git version 2.46.0\n")
		host.addCommand("/usr/bin/git --version", "git version 2.39.0\n")

		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "path.shadowed")
		finding := doctorFindingWith(t, findings, "resolves to")
		if finding.Status != doctorWarning {
			t.Fatalf("two copies with two versions is reported as %s", finding.Status)
		}
		for _, want := range []string{"2.46.0", "2.39.0", "~/.local/bin/git", "/usr/bin/git"} {
			if !strings.Contains(finding.Evidence, want) {
				t.Errorf("the finding does not name %q: %q", want, finding.Evidence)
			}
		}
		if !strings.Contains(finding.Advice, "first") {
			t.Errorf("the advice does not say what to do about the precedence: %q", finding.Advice)
		}
	})

	t.Run("two copies of the same version", func(t *testing.T) {
		host := base()
		host.addFile("/home/tester/.local/bin/git", 0o755, false, "")
		host.addFile("/usr/bin/git", 0o755, false, "")
		host.addCommand("/home/tester/.local/bin/git --version", "git version 2.44.0\n")
		host.addCommand("/usr/bin/git --version", "git version 2.44.0\n")

		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "path.shadowed")
		finding := doctorFindingWith(t, findings, "resolves to")
		if finding.Status != doctorOK {
			t.Errorf("two copies of the same version is reported as %s, want ok:\n%s", finding.Status, doctorFindingSummary(findings))
		}
	})

	t.Run("neither copy answers", func(t *testing.T) {
		host := base()
		host.addFile("/home/tester/.local/bin/git", 0o755, false, "")
		host.addFile("/usr/bin/git", 0o755, false, "")

		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "path.shadowed")
		finding := doctorFindingWith(t, findings, "resolves to")
		if finding.Status != doctorUnknown {
			t.Errorf("two copies that do not answer are reported as %s, want unknown:\n%s", finding.Status, doctorFindingSummary(findings))
		}
	})
}

// TestPathEffectiveComparesTheLoginShellWithThisProcess is S2's second half. A
// login shell that resolves a different PATH is a warning naming both sets, a
// shell that never answers is unknown, and no shell at all is not applicable.
func TestPathEffectiveComparesTheLoginShellWithThisProcess(t *testing.T) {
	base := func() *doctorFakeHost {
		host := newDoctorFakeHost()
		host.setenv("PATH", "/usr/bin:/bin")
		host.setenv("SHELL", "/bin/zsh")
		host.addDir("/usr/bin")
		host.addDir("/bin")
		host.addFile("/bin/zsh", 0o755, false, "")
		return host
	}
	const line = "/bin/zsh -l -c printf %s \"$PATH\""

	t.Run("the same PATH", func(t *testing.T) {
		host := base()
		host.addCommand(line, "/usr/bin:/bin")
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "path.effective")
		if got := doctorWorst(findings); got != doctorOK {
			t.Errorf("an equal PATH is reported as %s:\n%s", got, doctorFindingSummary(findings))
		}
	})

	t.Run("a PATH the login shell adds to", func(t *testing.T) {
		host := base()
		host.addCommand(line, "/home/tester/.local/bin:/usr/bin:/bin:/opt/homebrew/bin")
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "path.effective")
		finding := doctorFindingWith(t, findings, "")
		if finding.Status != doctorWarning {
			t.Fatalf("a different PATH is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
		for _, want := range []string{"/home/tester/.local/bin", "/opt/homebrew/bin"} {
			if !strings.Contains(finding.Evidence, want) {
				t.Errorf("the finding does not name %q: %q", want, finding.Evidence)
			}
		}
	})

	t.Run("a login shell that never answers", func(t *testing.T) {
		host := base()
		host.addCommandResult(line, doctorCommandResult{TimedOut: true, Code: -1, Err: "the command did not finish inside its bound"})
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "path.effective")
		finding := doctorFindingWith(t, findings, "")
		if finding.Status != doctorUnknown {
			t.Errorf("a hanging login shell is reported as %s, want unknown:\n%s", finding.Status, doctorFindingSummary(findings))
		}
	})

	t.Run("no shell named", func(t *testing.T) {
		host := base()
		host.unsetenv("SHELL")
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "path.effective")
		if got := doctorWorst(findings); got != doctorNotApplicable {
			t.Errorf("no login shell is reported as %s, want n/a:\n%s", got, doctorFindingSummary(findings))
		}
	})
}

// ============================================================================
// The shells and their configuration
// ============================================================================

// TestShellFilesCountsWhatTheInstallerWrites is S3's first question: are the
// files this installer writes for the shells that are installed present. A shell
// that is not installed is not applicable, never a fault.
func TestShellFilesCountsWhatTheInstallerWrites(t *testing.T) {
	t.Run("a shell that is installed and configured", func(t *testing.T) {
		host := doctorHealthyHost()
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "shell.files")
		finding := doctorFindingWith(t, findings, "zsh configuration")
		if finding.Status != doctorOK {
			t.Errorf("a complete zsh configuration is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
	})

	t.Run("a file this installer needs and that is not there", func(t *testing.T) {
		host := doctorHealthyHost()
		delete(host.files, "/home/tester/.zshenv")
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "shell.files")
		finding := doctorFindingWith(t, findings, "~/.zshenv")
		if finding.Status != doctorWarning {
			t.Fatalf("a missing ~/.zshenv is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
		if !strings.Contains(finding.Advice, "installer") {
			t.Errorf("the advice does not say where the file comes from: %q", finding.Advice)
		}
	})

	t.Run("a shell that is not installed", func(t *testing.T) {
		host := newDoctorFakeHost()
		host.setenv("PATH", "/usr/bin:/bin")
		host.addDir("/usr/bin")
		host.addDir("/bin")
		host.addDir("/home/tester")
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "shell.files")
		if got := doctorWorst(findings); got != doctorNotApplicable {
			t.Errorf("no installed shell is reported as %s, want n/a:\n%s", got, doctorFindingSummary(findings))
		}
	})
}

// TestShellSyntaxUsesTheShellsOwnCheckerAndNoScript is the rule that keeps the
// doctor from pretending to be a shell: a non-zero check is an error that quotes
// the checker, a checker that cannot be run is unknown, and the check never
// interprets the file itself.
func TestShellSyntaxUsesTheShellsOwnCheckerAndNoScript(t *testing.T) {
	t.Run("the shell's own check passes", func(t *testing.T) {
		host := doctorHealthyHost()
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "shell.syntax")
		finding := doctorFindingWith(t, findings, "zsh configuration syntax")
		if finding.Status != doctorOK {
			t.Errorf("a file the shell accepts is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
	})

	t.Run("the shell's own check fails", func(t *testing.T) {
		host := doctorHealthyHost()
		host.addCommandResult("/usr/bin/zsh -n /home/tester/.zshrc",
			doctorCommandResult{Code: 1, Err: "exited with status 1", Stderr: "/home/tester/.zshrc:3: parse error near `\\n'\n"})
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "shell.syntax")
		finding := doctorFindingWith(t, findings, "zsh configuration syntax")
		if finding.Status != doctorError {
			t.Fatalf("a file the shell refuses is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
		if !strings.Contains(finding.Evidence, "parse error") {
			t.Errorf("the finding does not quote what the shell said: %q", finding.Evidence)
		}
	})

	t.Run("the checker cannot be run", func(t *testing.T) {
		host := doctorHealthyHost()
		host.addCommandResult("/usr/bin/zsh -n /home/tester/.zshrc",
			doctorCommandResult{TimedOut: true, Code: -1, Err: "the command did not finish inside its bound"})
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "shell.syntax")
		finding := doctorFindingWith(t, findings, "zsh configuration syntax")
		if finding.Status != doctorUnknown {
			t.Errorf("a checker that never answers is reported as %s, want unknown:\n%s", finding.Status, doctorFindingSummary(findings))
		}
	})
}

// TestShellReferencesReadsLiteralLinesOnly pins the include check: a literal
// include that is not there is a warning, one that is there is ok, and a line
// that only a shell could resolve is skipped rather than guessed at.
func TestShellReferencesReadsLiteralLinesOnly(t *testing.T) {
	t.Run("an include that is not there", func(t *testing.T) {
		host := doctorHealthyHost()
		host.addText("/home/tester/.zshrc", 0o644, "source ~/.gone/missing.zsh\nalias ll='eza -l'\n")
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "shell.references")
		finding := doctorFindingWith(t, findings, "~/.gone/missing.zsh")
		if finding.Status != doctorWarning {
			t.Fatalf("a broken include is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
	})

	t.Run("a line that needs a shell", func(t *testing.T) {
		host := doctorHealthyHost()
		host.addText("/home/tester/.zshrc", 0o644, "source \"$(brew --prefix)/share/zsh/zsh-syntax-highlighting.zsh\"\nalias ll='eza -l'\n")
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "shell.references")
		for _, finding := range findings {
			if finding.Status == doctorWarning || finding.Status == doctorError {
				t.Errorf("a line only a shell can resolve is judged as %s: %s", finding.Status, finding.Evidence)
			}
		}
	})
}

// TestShellConflictsReportsTwoDifferentLiteralValues is S3's conflict rule: the
// same alias or variable defined twice with two different literal values is a
// warning, and the same value twice is not a conflict at all.
func TestShellConflictsReportsTwoDifferentLiteralValues(t *testing.T) {
	t.Run("two different values", func(t *testing.T) {
		host := doctorHealthyHost()
		host.addText("/home/tester/.zshrc", 0o644, "alias ll='eza -l --icons'\n")
		host.addText("/home/tester/.bashrc", 0o644, "alias ll='ls -l'\n")
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "shell.conflicts")
		finding := doctorFindingWith(t, findings, "ll")
		if finding.Status != doctorWarning {
			t.Fatalf("two different values are reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
		for _, want := range []string{"eza -l --icons", "ls -l"} {
			if !strings.Contains(finding.Evidence, want) {
				t.Errorf("the finding does not say which two values: %q", finding.Evidence)
			}
		}
	})

	t.Run("the same value twice", func(t *testing.T) {
		host := doctorHealthyHost()
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "shell.conflicts")
		for _, finding := range findings {
			if finding.Status == doctorWarning || finding.Status == doctorError {
				t.Errorf("the same alias in two files is reported as %s: %s", finding.Status, finding.Evidence)
			}
		}
	})
}

// TestShellIdentityComparesTheThreeAnswers is S3's identity rule: the shell the
// terminal starts and the shell this process runs under are compared rather than
// taken for granted, and an answer that cannot be read is unknown.
func TestShellIdentityComparesTheThreeAnswers(t *testing.T) {
	withProcess := func(comm string) *doctorFakeHost {
		host := doctorHealthyHost()
		host.setenv("SHELL", "/usr/bin/zsh")
		host.addText("/proc/4242/comm", 0o444, comm+"\n")
		return host
	}
	scan := func(t *testing.T, host *doctorFakeHost) *doctorScan {
		t.Helper()
		doctorParentPID = func() int { return 4242 }
		t.Cleanup(func() { doctorParentPID = os.Getppid })
		s := doctorTestScan(host, doctorTestLinuxInfo())
		s.procRootDir = "/proc"
		return s
	}

	t.Run("the process runs under the shell it names", func(t *testing.T) {
		findings := runDoctorCheck(scan(t, withProcess("zsh")), doctorCheckForTest(t, "shell.identity")).Findings
		finding := doctorFindingWith(t, findings, "this process runs under")
		if finding.Status != doctorOK {
			t.Errorf("a matching login shell is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
	})

	t.Run("the process runs under another shell", func(t *testing.T) {
		// A shell inside a shell is how a person works, not a fault: the finding
		// states both answers and lets the reader decide whether it was meant.
		findings := runDoctorCheck(scan(t, withProcess("bash")), doctorCheckForTest(t, "shell.identity")).Findings
		finding := doctorFindingWith(t, findings, "this process runs under")
		if finding.Status != doctorOK {
			t.Fatalf("a shell inside a shell is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
		for _, want := range []string{"bash", "zsh"} {
			if !strings.Contains(finding.Evidence, want) {
				t.Errorf("the finding does not name %q: %q", want, finding.Evidence)
			}
		}
	})

	t.Run("the process table cannot be read", func(t *testing.T) {
		host := withProcess("zsh")
		s := doctorTestScan(host, doctorTestLinuxInfo())
		s.procRootDir = "/nowhere"
		doctorParentPID = func() int { return 4242 }
		t.Cleanup(func() { doctorParentPID = os.Getppid })
		findings := runDoctorCheck(s, doctorCheckForTest(t, "shell.identity")).Findings
		finding := doctorFindingWith(t, findings, "this process runs under")
		if finding.Status != doctorUnknown {
			t.Errorf("an unreadable process table is reported as %s, want unknown:\n%s", finding.Status, doctorFindingSummary(findings))
		}
	})
}

// ============================================================================
// The tools the installer manages, and the ones it runs its steps with
// ============================================================================

// TestToolsManagedReportsMissingToolsByWhatTheyBreak is S4: a required tool that
// is missing is an error, an optional one is a warning that names the unit it
// belongs to, and a tool whose version command fails is not counted as working.
func TestToolsManagedReportsMissingToolsByWhatTheyBreak(t *testing.T) {
	t.Run("everything in place", func(t *testing.T) {
		findings := doctorCheckFindings(t, doctorHealthyHost(), doctorTestLinuxInfo(), "tools.managed")
		if got := doctorWorst(findings); got != doctorOK {
			t.Errorf("a machine with every managed tool is reported as %s:\n%s", got, doctorFindingSummary(findings))
		}
	})

	t.Run("a required tool is missing", func(t *testing.T) {
		host := doctorHealthyHost()
		delete(host.files, "/usr/bin/git")
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "tools.managed")
		finding := doctorFindingWith(t, findings, "git")
		if finding.Status != doctorError {
			t.Fatalf("a missing required tool is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
		if !strings.Contains(finding.Evidence, "PATH") {
			t.Errorf("the finding does not say where it looked: %q", finding.Evidence)
		}
	})

	t.Run("an optional tool is missing", func(t *testing.T) {
		host := doctorHealthyHost()
		delete(host.files, "/usr/bin/tmux")
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "tools.managed")
		finding := doctorFindingWith(t, findings, "tmux")
		if finding.Status != doctorWarning {
			t.Fatalf("a missing optional tool is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
		if !strings.Contains(finding.Advice, "wm") {
			t.Errorf("the advice does not name the unit that would install it: %q", finding.Advice)
		}
	})

	t.Run("a required tool that is there but does not answer", func(t *testing.T) {
		host := doctorHealthyHost()
		host.addCommandResult("/usr/bin/git --version", doctorCommandResult{Code: 1, Err: "exited with status 1"})
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "tools.managed")
		finding := doctorFindingWith(t, findings, "does not answer")
		if finding.Status != doctorError {
			t.Errorf("a required tool whose version command fails is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
	})

	t.Run("an optional tool that is there but does not answer", func(t *testing.T) {
		host := doctorHealthyHost()
		host.addCommandResult("/usr/bin/nvim --version", doctorCommandResult{Code: 1, Err: "exited with status 1"})
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "tools.managed")
		finding := doctorFindingWith(t, findings, "does not answer")
		if finding.Status != doctorWarning {
			t.Errorf("an optional tool whose version command fails is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
	})
}

// TestToolsRunReportsTheCommandsTheStepsNeed is the second half of S4: the shell
// the run's own scripts execute in and the package manager, both of which a run
// cannot do without.
func TestToolsRunReportsTheCommandsTheStepsNeed(t *testing.T) {
	t.Run("a shell and a package manager", func(t *testing.T) {
		findings := doctorCheckFindings(t, doctorHealthyHost(), doctorTestLinuxInfo(), "tools.run")
		for _, finding := range findings {
			if finding.Status == doctorError {
				t.Errorf("a machine with sh, a package manager and sudo reports an error: %s — %s", finding.Subject, finding.Evidence)
			}
		}
	})

	t.Run("no shell at all", func(t *testing.T) {
		host := doctorHealthyHost()
		delete(host.files, "/bin/sh")
		delete(host.files, "/bin/bash")
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "tools.run")
		finding := doctorFindingWith(t, findings, "shell")
		if finding.Status != doctorError {
			t.Errorf("a machine with no sh and no bash is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
	})

	t.Run("no package manager", func(t *testing.T) {
		host := doctorHealthyHost()
		for _, name := range []string{"apt-get", "dnf", "pacman", "zypper", "apk", "brew"} {
			delete(host.files, "/usr/bin/"+name)
		}
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "tools.run")
		finding := doctorFindingWith(t, findings, "package manager")
		if finding.Status != doctorError {
			t.Errorf("a machine with no package manager is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
	})

	t.Run("no sudo", func(t *testing.T) {
		host := doctorHealthyHost()
		delete(host.files, "/usr/bin/sudo")
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "tools.run")
		finding := doctorFindingWith(t, findings, "administrator")
		if finding.Status != doctorWarning {
			t.Errorf("a machine with no sudo is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
	})
}

// ============================================================================
// The dotfiles this installer manages
// ============================================================================

// doctorWriteRecord writes a real record into a state directory of the test's
// own and mirrors it into the fake machine, so the check's own read and the path
// it stats agree. The record reader is the installer's own, shared with --verify,
// so this is the one place a test writes a file through the real filesystem --
// into a temporary directory, never into the developer's home.
func doctorWriteRecord(t *testing.T, host *doctorFakeHost, rec lastInstall) {
	t.Helper()
	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("the test could not write its own record: %v", err)
	}
	writeDoctorRecordFile(t, host, string(data))
}

// writeDoctorRecordFile is the same step for a record that is deliberately not
// valid JSON.
func writeDoctorRecordFile(t *testing.T, host *doctorFakeHost, content string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	path := filepath.Join(dir, stateAppDir, lastInstallFileName)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("the test could not make its own state directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("the test could not write its own record: %v", err)
	}
	host.addText(path, 0o600, content)
}

// TestDotfilesRecordTellsAbsentFromUnreadable is S5's first distinction: no
// record at all means there is nothing to check, and a record that cannot be read
// is a warning rather than a clean answer.
func TestDotfilesRecordTellsAbsentFromUnreadable(t *testing.T) {
	t.Run("no record", func(t *testing.T) {
		t.Setenv("XDG_STATE_HOME", t.TempDir())
		findings := doctorCheckFindings(t, doctorHealthyHost(), doctorTestLinuxInfo(), "dotfiles.record")
		if got := doctorWorst(findings); got != doctorNotApplicable {
			t.Errorf("no record is reported as %s, want n/a:\n%s", got, doctorFindingSummary(findings))
		}
	})

	t.Run("a record", func(t *testing.T) {
		host := doctorHealthyHost()
		doctorWriteRecord(t, host, lastInstall{Timestamp: time.Now(), Version: "test", Files: []string{"/home/tester/.zshrc"}})
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "dotfiles.record")
		finding := doctorFindingWith(t, findings, "")
		if finding.Status != doctorOK {
			t.Errorf("a readable record is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
	})

	t.Run("a record that cannot be parsed", func(t *testing.T) {
		host := doctorHealthyHost()
		writeDoctorRecordFile(t, host, "{not json")
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "dotfiles.record")
		if got := doctorWorst(findings); got != doctorWarning {
			t.Errorf("an unreadable record is reported as %s, want a warning:\n%s", got, doctorFindingSummary(findings))
		}
	})
}

// TestDotfilesPathsChecksWhatTheLastRunTouched is the second half of S5: a path
// the record names and that is gone is a warning, and a path that is there is ok.
func TestDotfilesPathsChecksWhatTheLastRunTouched(t *testing.T) {
	t.Run("a path the record names and that is there", func(t *testing.T) {
		host := doctorHealthyHost()
		doctorWriteRecord(t, host, lastInstall{Timestamp: time.Now(), Version: "test", Files: []string{"/home/tester/.zshrc"}})
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "dotfiles.paths")
		if got := doctorWorst(findings); got != doctorOK {
			t.Errorf("a recorded path that is there is reported as %s:\n%s", got, doctorFindingSummary(findings))
		}
	})

	t.Run("a path the record names and that is gone", func(t *testing.T) {
		host := doctorHealthyHost()
		doctorWriteRecord(t, host, lastInstall{Timestamp: time.Now(), Version: "test", Files: []string{"/home/tester/.gone"}})
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "dotfiles.paths")
		finding := doctorFindingWith(t, findings, "~/.gone")
		if finding.Status != doctorWarning {
			t.Fatalf("a recorded path that is gone is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
		if !strings.Contains(finding.Advice, "installer") {
			t.Errorf("the advice does not say how to put it back: %q", finding.Advice)
		}
	})
}

// TestDotfilesLinksFollowsWhatItCannotRead is S5's link rule: a link whose target
// is gone is a warning naming both sides, a link that resolves is ok, and a
// machine with no link at all says so rather than reporting nothing.
func TestDotfilesLinksFollowsWhatItCannotRead(t *testing.T) {
	t.Run("a dangling link", func(t *testing.T) {
		host := doctorHealthyHost()
		host.addLink("/home/tester/.p10k.zsh", "/home/tester/gone/p10k.zsh")
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "dotfiles.links")
		finding := doctorFindingWith(t, findings, "~/.p10k.zsh")
		if finding.Status != doctorWarning {
			t.Fatalf("a dangling link is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
		if !strings.Contains(finding.Evidence, "gone/p10k.zsh") {
			t.Errorf("the finding does not name where the link points: %q", finding.Evidence)
		}
	})

	t.Run("a link that resolves", func(t *testing.T) {
		host := doctorHealthyHost()
		host.addText("/home/tester/dotfiles/p10k.zsh", 0o644, "# the prompt\n")
		host.addLink("/home/tester/.p10k.zsh", "/home/tester/dotfiles/p10k.zsh")
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "dotfiles.links")
		if got := doctorWorst(findings); got != doctorOK {
			t.Errorf("a link that resolves is reported as %s:\n%s", got, doctorFindingSummary(findings))
		}
	})

	t.Run("no link at all", func(t *testing.T) {
		findings := doctorCheckFindings(t, doctorHealthyHost(), doctorTestLinuxInfo(), "dotfiles.links")
		if got := doctorWorst(findings); got != doctorNotApplicable {
			t.Errorf("a machine with no link is reported as %s, want n/a:\n%s", got, doctorFindingSummary(findings))
		}
	})
}

// TestDotfilesPermissionsReportsModesThatLetOthersIn is S5's permission rule: a
// path that anyone may write is a warning, and a path only its owner may write is
// ok.
func TestDotfilesPermissionsReportsModesThatLetOthersIn(t *testing.T) {
	t.Run("a configuration anyone may write", func(t *testing.T) {
		host := doctorHealthyHost()
		host.addText("/home/tester/.zshrc", 0o666, "alias ll='eza -l'\n")
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "dotfiles.permissions")
		finding := doctorFindingWith(t, findings, "~/.zshrc")
		if finding.Status != doctorWarning {
			t.Fatalf("a world-writable configuration is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
		if !strings.Contains(finding.Evidence, "0666") {
			t.Errorf("the finding does not state the mode: %q", finding.Evidence)
		}
	})

	t.Run("every configuration owned by its user", func(t *testing.T) {
		findings := doctorCheckFindings(t, doctorHealthyHost(), doctorTestLinuxInfo(), "dotfiles.permissions")
		if got := doctorWorst(findings); got != doctorOK {
			t.Errorf("configurations only their owner can write are reported as %s:\n%s", got, doctorFindingSummary(findings))
		}
	})
}

// TestDotfilesForeignIsNotAFault is the boundary the brief draws: a configuration
// this installer did not write is stated, never reported as a fault.
func TestDotfilesForeignIsNotAFault(t *testing.T) {
	host := doctorHealthyHost()
	doctorWriteRecord(t, host, lastInstall{Timestamp: time.Now(), Version: "test", Files: []string{"/home/tester/.zshrc"}})
	findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "dotfiles.foreign")
	for _, finding := range findings {
		if finding.Status == doctorWarning || finding.Status == doctorError {
			t.Errorf("a configuration this installer has not written is reported as %s: %s", finding.Status, finding.Evidence)
		}
	}
	if len(findings) == 0 {
		t.Error("the check said nothing at all about the configurations it did not write")
	}
}

// ============================================================================
// Git and ssh
// ============================================================================

// TestGitBinaryNamesTheVersionOrTheFault is S6's first line: git resolves and
// answers, or the doctor says which of the two failed.
func TestGitBinaryNamesTheVersionOrTheFault(t *testing.T) {
	t.Run("git is installed", func(t *testing.T) {
		findings := doctorCheckFindings(t, doctorHealthyHost(), doctorTestLinuxInfo(), "git.binary")
		finding := doctorFindingWith(t, findings, "Git")
		if finding.Status != doctorOK {
			t.Errorf("an installed git is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
		if !strings.Contains(finding.Evidence, "2.44.0") {
			t.Errorf("the finding does not name the version: %q", finding.Evidence)
		}
	})

	t.Run("git is missing", func(t *testing.T) {
		host := doctorHealthyHost()
		delete(host.files, "/usr/bin/git")
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "git.binary")
		finding := doctorFindingWith(t, findings, "Git")
		if finding.Status != doctorError {
			t.Errorf("a machine with no git is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
	})

	t.Run("git is there but does not answer", func(t *testing.T) {
		host := doctorHealthyHost()
		host.addCommandResult("/usr/bin/git --version", doctorCommandResult{Code: 127, Err: "exited with status 127"})
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "git.binary")
		finding := doctorFindingWith(t, findings, "Git")
		if finding.Status != doctorError {
			t.Errorf("a git that does not answer is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
	})
}

// TestGitIdentityDoesNotReportALocalIdentityAsAFault is S10's rule about the
// developer's own habits: an identity set globally and overridden inside one
// repository is a normal configuration and must read as ok.
func TestGitIdentityDoesNotReportALocalIdentityAsAFault(t *testing.T) {
	t.Run("a global identity", func(t *testing.T) {
		findings := doctorCheckFindings(t, doctorHealthyHost(), doctorTestLinuxInfo(), "git.identity")
		if got := doctorWorst(findings); got != doctorOK {
			t.Errorf("a global identity is reported as %s:\n%s", got, doctorFindingSummary(findings))
		}
	})

	t.Run("a repository that overrides it", func(t *testing.T) {
		host := doctorHealthyHost()
		host.addDir("/home/tester/project")
		host.addCommand("/usr/bin/git rev-parse --show-toplevel", "/home/tester/project\n")
		host.addCommand("/usr/bin/git -C /home/tester/project config --local --get user.name", "Someone Else\n")
		host.addCommand("/usr/bin/git -C /home/tester/project config --local --get user.email", "someone@work.example\n")

		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "git.identity")
		if got := doctorWorst(findings); got != doctorOK {
			t.Fatalf("a repository with its own identity is reported as %s:\n%s", got, doctorFindingSummary(findings))
		}
		finding := doctorFindingWith(t, findings, "this repository would use")
		if !strings.Contains(finding.Evidence, "someone@work.example") {
			t.Errorf("the finding does not state the identity the repository uses: %q", finding.Evidence)
		}
		if !strings.Contains(finding.Impact, "normal") {
			t.Errorf("the finding does not say that a repository identity is normal: %q", finding.Impact)
		}
	})

	t.Run("no identity at all", func(t *testing.T) {
		host := doctorHealthyHost()
		host.addCommandResult("/usr/bin/git config --global --get user.name", doctorCommandResult{Code: 1, Err: "exited with status 1"})
		host.addCommandResult("/usr/bin/git config --global --get user.email", doctorCommandResult{Code: 1, Err: "exited with status 1"})
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "git.identity")
		finding := doctorFindingWith(t, findings, "identity")
		if finding.Status != doctorWarning {
			t.Fatalf("no git identity is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
		if !strings.Contains(finding.Advice, "git config") {
			t.Errorf("the advice does not print the command to set one: %q", finding.Advice)
		}
	})
}

// TestGitCredentialsReportsTheKindAndNeverTheSecret is S10's second rule: a
// helper is named by what it is, the custom command's text is never printed, and
// a credentials file that anyone may read is a warning about its mode.
func TestGitCredentialsReportsTheKindAndNeverTheSecret(t *testing.T) {
	t.Run("a known helper", func(t *testing.T) {
		findings := doctorCheckFindings(t, doctorHealthyHost(), doctorTestLinuxInfo(), "git.credentials")
		finding := doctorFindingWith(t, findings, "authenticates")
		if finding.Status != doctorOK {
			t.Errorf("a stored helper is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
	})

	t.Run("a custom command", func(t *testing.T) {
		host := doctorHealthyHost()
		host.addCommand("/usr/bin/git config --global --get credential.helper", "!f() { echo hunter2-from-the-helper; }; f\n")
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "git.credentials")
		finding := doctorFindingWith(t, findings, "custom command")
		if finding.Status != doctorOK {
			t.Errorf("a custom helper is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
		for _, candidate := range findings {
			if strings.Contains(candidate.Evidence, "hunter2-from-the-helper") || strings.Contains(candidate.Advice, "hunter2-from-the-helper") {
				t.Errorf("the finding printed the helper's command, which can hold a token: %+v", candidate)
			}
		}
	})

	t.Run("a credentials file others may read", func(t *testing.T) {
		host := doctorHealthyHost()
		host.addText("/home/tester/.git-credentials", 0o644, "https://user:token@example.com\n")
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "git.credentials")
		finding := doctorFindingWith(t, findings, "git-credentials")
		if finding.Status != doctorWarning {
			t.Fatalf("a world-readable credentials file is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
		for _, candidate := range findings {
			if strings.Contains(candidate.Evidence, "token@example.com") {
				t.Errorf("the finding printed the credentials file's content: %+v", candidate)
			}
		}
	})

	t.Run("no helper and no file", func(t *testing.T) {
		host := doctorHealthyHost()
		host.addCommandResult("/usr/bin/git config --global --get credential.helper", doctorCommandResult{Code: 1, Err: "exited with status 1"})
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "git.credentials")
		for _, finding := range findings {
			if finding.Status == doctorError {
				t.Errorf("a machine with no stored credential helper reports an error: %s", finding.Evidence)
			}
		}
	})
}

// TestGitSSHAuthChecksModesAndCountsIdentities is S6's ssh half: the directory
// and the keys have the modes ssh requires, and the agent is reported by how many
// identities it holds, never by their names.
func TestGitSSHAuthChecksModesAndCountsIdentities(t *testing.T) {
	t.Run("the modes ssh requires", func(t *testing.T) {
		findings := doctorCheckFindings(t, doctorHealthyHost(), doctorTestLinuxInfo(), "git.sshauth")
		if got := doctorWorst(findings); got != doctorOK {
			t.Errorf("a well-kept ~/.ssh is reported as %s:\n%s", got, doctorFindingSummary(findings))
		}
	})

	t.Run("a directory others may read, with a fix", func(t *testing.T) {
		host := doctorHealthyHost()
		host.addFile("/home/tester/.ssh", 0o755, true, "")
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "git.sshauth")
		finding := doctorFindingWith(t, findings, "readable by others")
		if finding.Status != doctorWarning {
			t.Fatalf("a world-readable ~/.ssh is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
		if finding.Fix == nil || finding.Fix.Kind != doctorFixChmodDir || finding.Fix.Mode != 0o700 {
			t.Errorf("the finding does not offer the mode fix: %+v", finding.Fix)
		}
	})

	t.Run("a key others may read, with a fix", func(t *testing.T) {
		host := doctorHealthyHost()
		host.addText("/home/tester/.ssh/id_ed25519", 0o644, "-----BEGIN OPENSSH PRIVATE KEY-----\nkey\n-----END OPENSSH PRIVATE KEY-----\n")
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "git.sshauth")
		finding := doctorFindingWith(t, findings, "private key")
		if finding.Status != doctorWarning {
			t.Fatalf("a world-readable key is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
		if finding.Fix == nil || finding.Fix.Kind != doctorFixChmodFile || finding.Fix.Mode != 0o600 {
			t.Errorf("the finding does not offer the 0600 fix: %+v", finding.Fix)
		}
	})

	t.Run("an agent that holds identities", func(t *testing.T) {
		host := doctorHealthyHost()
		host.setenv("SSH_AUTH_SOCK", "/tmp/agent.sock")
		host.addText("/tmp/agent.sock", 0o600, "")
		host.addCommand("/usr/bin/ssh-add -l", "256 SHA256:aaa tester@example.com (ED25519)\n")
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "git.sshauth")
		finding := doctorFindingWith(t, findings, "SSH agent")
		if finding.Status != doctorOK {
			t.Fatalf("a working agent is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
		if !strings.Contains(finding.Evidence, "1 identity") {
			t.Errorf("the finding does not count the identities: %q", finding.Evidence)
		}
		if strings.Contains(finding.Evidence, "tester@example.com") {
			t.Errorf("the finding printed an identity: %q", finding.Evidence)
		}
	})

	t.Run("an agent with nothing loaded", func(t *testing.T) {
		host := doctorHealthyHost()
		host.setenv("SSH_AUTH_SOCK", "/tmp/agent.sock")
		host.addText("/tmp/agent.sock", 0o600, "")
		host.addCommandResult("/usr/bin/ssh-add -l", doctorCommandResult{Code: 1, Err: "exited with status 1"})
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "git.sshauth")
		finding := doctorFindingWith(t, findings, "SSH agent")
		if finding.Status != doctorWarning {
			t.Errorf("an empty agent is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
	})

	t.Run("a socket that is not there", func(t *testing.T) {
		host := doctorHealthyHost()
		host.setenv("SSH_AUTH_SOCK", "/tmp/gone.sock")
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "git.sshauth")
		finding := doctorFindingWith(t, findings, "SSH agent")
		if finding.Status != doctorWarning {
			t.Errorf("a stale socket is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
	})

	t.Run("no agent in this environment", func(t *testing.T) {
		findings := doctorCheckFindings(t, doctorHealthyHost(), doctorTestLinuxInfo(), "git.sshauth")
		finding := doctorFindingWith(t, findings, "SSH agent")
		if finding.Status != doctorOK {
			t.Errorf("an environment with no agent is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
		if !strings.Contains(finding.Evidence, "not set") {
			t.Errorf("the finding does not say the variable is unset: %q", finding.Evidence)
		}
	})
}

// ============================================================================
// The environment the installer runs in
// ============================================================================

// TestEnvHomeReportsWhatItRead pins the home check: a home that is a directory is
// ok, an unset HOME is an error, and a home that is a file is an error too.
func TestEnvHomeReportsWhatItRead(t *testing.T) {
	t.Run("a home directory", func(t *testing.T) {
		findings := doctorCheckFindings(t, doctorHealthyHost(), doctorTestLinuxInfo(), "env.home")
		if got := doctorWorst(findings); got != doctorOK {
			t.Errorf("a home directory is reported as %s:\n%s", got, doctorFindingSummary(findings))
		}
	})

	t.Run("HOME is unset", func(t *testing.T) {
		host := doctorHealthyHost()
		host.unsetenv("HOME")
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "env.home")
		if got := doctorWorst(findings); got != doctorError {
			t.Errorf("an unset HOME is reported as %s, want an error:\n%s", got, doctorFindingSummary(findings))
		}
	})

	t.Run("HOME names a file", func(t *testing.T) {
		host := newDoctorFakeHost()
		host.addText("/home/tester", 0o644, "not a directory\n")
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "env.home")
		if got := doctorWorst(findings); got != doctorError {
			t.Errorf("a HOME that is a file is reported as %s, want an error:\n%s", got, doctorFindingSummary(findings))
		}
	})
}

// TestEnvXDGReportsTheVariablesThatCannotBeUsed is S7's first rule: a variable
// that is set to a relative path or to a directory that is not there is a
// warning, and the fallback the specification defines is stated with it.
func TestEnvXDGReportsTheVariablesThatCannotBeUsed(t *testing.T) {
	t.Run("all four usable", func(t *testing.T) {
		host := doctorHealthyHost()
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "env.xdg")
		if got := doctorWorst(findings); got != doctorOK {
			t.Errorf("four usable variables are reported as %s:\n%s", got, doctorFindingSummary(findings))
		}
	})

	t.Run("a relative path", func(t *testing.T) {
		host := doctorHealthyHost()
		host.setenv("XDG_CONFIG_HOME", "config")
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "env.xdg")
		finding := doctorFindingWith(t, findings, "XDG_CONFIG_HOME")
		if finding.Status != doctorWarning {
			t.Fatalf("a relative variable is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
		if !strings.Contains(finding.Advice, ".config") {
			t.Errorf("the advice does not name the fallback: %q", finding.Advice)
		}
	})

	t.Run("a directory that is not there, with a fix", func(t *testing.T) {
		host := doctorHealthyHost()
		host.setenv("XDG_STATE_HOME", "/home/tester/.local/missing")
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "env.xdg")
		finding := doctorFindingWith(t, findings, "XDG_STATE_HOME")
		if finding.Status != doctorWarning {
			t.Fatalf("a missing directory is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
		if finding.Fix == nil || finding.Fix.Kind != doctorFixMakeDir {
			t.Errorf("the finding does not offer to create the directory: %+v", finding.Fix)
		}
	})
}

// TestEnvTempReportsTheDirectoryWritesGoTo is S7's second rule: the directory
// temporary files land in must exist and be writable, and the default is named
// when nothing overrides it.
func TestEnvTempReportsTheDirectoryWritesGoTo(t *testing.T) {
	t.Run("a writable directory", func(t *testing.T) {
		findings := doctorCheckFindings(t, doctorHealthyHost(), doctorTestLinuxInfo(), "env.temp")
		if got := doctorWorst(findings); got != doctorOK {
			t.Errorf("a writable TMPDIR is reported as %s:\n%s", got, doctorFindingSummary(findings))
		}
	})

	t.Run("a directory that is not there, with a fix", func(t *testing.T) {
		host := doctorHealthyHost()
		host.setenv("TMPDIR", "/home/tester/.cache/gone")
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "env.temp")
		finding := doctorFindingWith(t, findings, "TMPDIR")
		if finding.Status != doctorError {
			t.Fatalf("a missing TMPDIR is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
		if finding.Fix == nil || finding.Fix.Kind != doctorFixMakeDir {
			t.Errorf("the finding does not offer to create the directory: %+v", finding.Fix)
		}
	})
}

// TestEnvCapacityStatesItsSourceOrSaysUnknown is S7's third rule: a number with
// the command it came from, a warning under the floor a run needs, and unknown --
// never a guess -- when the tools that answer are not installed.
func TestEnvCapacityStatesItsSourceOrSaysUnknown(t *testing.T) {
	t.Run("a number with its source", func(t *testing.T) {
		findings := doctorCheckFindings(t, doctorHealthyHost(), doctorTestLinuxInfo(), "env.capacity")
		finding := doctorFindingWith(t, findings, "Room under the home directory")
		if finding.Status != doctorOK {
			t.Fatalf("60 GB free is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
		if !strings.Contains(finding.Evidence, "60000000 KB") {
			t.Errorf("the finding does not state the number with its source: %q", finding.Evidence)
		}
	})

	t.Run("room for a run to fail", func(t *testing.T) {
		host := doctorHealthyHost()
		host.addCommand("/usr/bin/df -Pk /home/tester", "Filesystem 1024-blocks Used Available Capacity Mounted on\n/dev/sda1 100000000 99500000 500000 99% /\n")
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "env.capacity")
		finding := doctorFindingWith(t, findings, "Room under the home directory")
		if finding.Status != doctorWarning {
			t.Errorf("500 MB free is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
	})

	t.Run("not enough room for a run at all", func(t *testing.T) {
		host := doctorHealthyHost()
		host.addCommand("/usr/bin/df -Pk /home/tester", "Filesystem 1024-blocks Used Available Capacity Mounted on\n/dev/sda1 100000000 99990000 100000 99% /\n")
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "env.capacity")
		finding := doctorFindingWith(t, findings, "Room under the home directory")
		if finding.Status != doctorError {
			t.Errorf("100 MB free is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
	})

	t.Run("no way to read the space", func(t *testing.T) {
		host := doctorHealthyHost()
		delete(host.files, "/usr/bin/df")
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "env.capacity")
		finding := doctorFindingWith(t, findings, "Room under the home directory")
		if finding.Status != doctorUnknown {
			t.Errorf("a machine with no df is reported as %s, want unknown:\n%s", finding.Status, doctorFindingSummary(findings))
		}
	})

	t.Run("a low descriptor limit", func(t *testing.T) {
		host := doctorHealthyHost()
		host.addCommand("/bin/sh -c ulimit -n", "128\n")
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "env.capacity")
		finding := doctorFindingWith(t, findings, "Open files per process")
		if finding.Status != doctorWarning {
			t.Errorf("a limit of 128 is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
	})
}

// TestEnvLocaleReportsWhatTheToolsWillPrint is S7's locale rule: a locale that is
// not set, one that is not UTF-8, and a category variable that overrides LANG are
// each named.
func TestEnvLocaleReportsWhatTheToolsWillPrint(t *testing.T) {
	cases := []struct {
		name     string
		env      map[string]string
		unset    []string
		status   doctorStatus
		mentions string
	}{
		{"a UTF-8 locale", map[string]string{"LANG": "en_US.UTF-8"}, []string{"LC_ALL", "LC_CTYPE"}, doctorOK, "en_US.UTF-8"},
		{"no locale at all", nil, []string{"LANG", "LC_ALL", "LC_CTYPE"}, doctorWarning, "all unset"},
		{"a locale that is not UTF-8", map[string]string{"LANG": "C"}, []string{"LC_ALL", "LC_CTYPE"}, doctorWarning, "C"},
		{"LC_ALL overriding LANG", map[string]string{"LANG": "en_US.UTF-8", "LC_ALL": "C"}, []string{"LC_CTYPE"}, doctorWarning, "LC_ALL"},
		{"LC_CTYPE overriding LANG for character handling", map[string]string{"LANG": "en_US.UTF-8", "LC_CTYPE": "C"}, []string{"LC_ALL"}, doctorWarning, "LC_CTYPE"},
		{"a UTF-8 LC_CTYPE that agrees with LANG", map[string]string{"LANG": "en_US.UTF-8", "LC_CTYPE": "en_US.UTF-8"}, []string{"LC_ALL"}, doctorOK, "en_US.UTF-8"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			host := doctorHealthyHost()
			for name, value := range tc.env {
				host.setenv(name, value)
			}
			for _, name := range tc.unset {
				host.unsetenv(name)
			}
			findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "env.locale")
			if got := doctorWorst(findings); got != tc.status {
				t.Errorf("the check reports %s, want %s:\n%s", got, tc.status, doctorFindingSummary(findings))
			}
			doctorFindingWith(t, findings, tc.mentions)
		})
	}
}

// TestEnvTermStatesWhatTerminfoKnows is S7's terminal rule: the entry is looked
// up rather than assumed, and a machine without infocmp says unknown.
func TestEnvTermStatesWhatTerminfoKnows(t *testing.T) {
	t.Run("an entry terminfo knows", func(t *testing.T) {
		findings := doctorCheckFindings(t, doctorHealthyHost(), doctorTestLinuxInfo(), "env.term")
		if got := doctorWorst(findings); got != doctorOK {
			t.Errorf("a known terminal is reported as %s:\n%s", got, doctorFindingSummary(findings))
		}
	})

	t.Run("an entry terminfo does not know", func(t *testing.T) {
		host := doctorHealthyHost()
		host.setenv("TERM", "xterm-kitty")
		host.addCommandResult("/usr/bin/infocmp xterm-kitty", doctorCommandResult{Code: 1, Err: "exited with status 1"})
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "env.term")
		finding := doctorFindingWith(t, findings, "terminfo")
		if finding.Status != doctorWarning {
			t.Errorf("an unknown terminal is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
	})

	t.Run("no TERM in this environment", func(t *testing.T) {
		host := doctorHealthyHost()
		host.unsetenv("TERM")
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "env.term")
		for _, finding := range findings {
			if finding.Status == doctorError {
				t.Errorf("an unset TERM is reported as an error: %s", finding.Evidence)
			}
		}
	})
}

// TestEnvVarsChecksOnlyTheValuesThatMeanSomething is S7's last rule: a documented
// variable set to a value nothing reads is a warning, and an unset variable is
// not a fault.
func TestEnvVarsChecksOnlyTheValuesThatMeanSomething(t *testing.T) {
	t.Run("nothing set", func(t *testing.T) {
		findings := doctorCheckFindings(t, doctorHealthyHost(), doctorTestLinuxInfo(), "env.vars")
		if got := doctorWorst(findings); got != doctorOK {
			t.Errorf("a machine with no installer variable set is reported as %s:\n%s", got, doctorFindingSummary(findings))
		}
	})

	t.Run("a value nothing reads", func(t *testing.T) {
		host := doctorHealthyHost()
		host.setenv("DOTFILES_ANIM", "maybe")
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "env.vars")
		finding := doctorFindingWith(t, findings, "DOTFILES_ANIM")
		if finding.Status != doctorWarning {
			t.Fatalf("a value nothing reads is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
		if !strings.Contains(finding.Advice, "1 or 0") {
			t.Errorf("the advice does not say what to use instead: %q", finding.Advice)
		}
	})

	t.Run("a directory that is not there", func(t *testing.T) {
		host := doctorHealthyHost()
		host.setenv("DOTFILES_DIR", "/home/tester/gone-dotfiles")
		findings := doctorCheckFindings(t, host, doctorTestLinuxInfo(), "env.vars")
		finding := doctorFindingWith(t, findings, "DOTFILES_DIR")
		if finding.Status != doctorWarning {
			t.Errorf("a DOTFILES_DIR that is not there is reported as %s:\n%s", finding.Status, doctorFindingSummary(findings))
		}
	})
}

// TestEnvPlatformIsGatedAndSaysWhenItCannotTell is S7's platform rule: every
// platform-specific fault is checked on the platform it belongs to, reported as
// not applicable everywhere else, and the whole check answers unknown when the
// platform was never detected.
func TestEnvPlatformIsGatedAndSaysWhenItCannotTell(t *testing.T) {
	t.Run("linux has no platform-specific fault", func(t *testing.T) {
		findings := doctorCheckFindings(t, doctorHealthyHost(), doctorTestLinuxInfo(), "env.platform")
		if got := doctorWorst(findings); got != doctorNotApplicable {
			t.Errorf("plain Linux is reported as %s, want n/a:\n%s", got, doctorFindingSummary(findings))
		}
	})

	t.Run("no platform was detected", func(t *testing.T) {
		findings := doctorCheckFindings(t, doctorHealthyHost(), nil, "env.platform")
		if got := doctorWorst(findings); got != doctorUnknown {
			t.Errorf("an undetected platform is reported as %s, want unknown:\n%s", got, doctorFindingSummary(findings))
		}
	})

	t.Run("wsl", func(t *testing.T) {
		host := doctorHealthyHost()
		host.addFile("/mnt/c", 0o755, true, "")
		findings := doctorCheckFindings(t, host, &system.SystemInfo{OS: system.OSWSL, OSName: "WSL", IsWSL: true, WSLVersion: 2, HomeDir: "/home/tester", UserShell: "/bin/zsh"}, "env.platform")
		if got := doctorWorst(findings); got == doctorNotApplicable {
			t.Errorf("WSL is reported as n/a, so the platform's own checks never run:\n%s", doctorFindingSummary(findings))
		}
		doctorFindingWith(t, findings, "/mnt/c")
	})

	t.Run("termux", func(t *testing.T) {
		host := doctorHealthyHost()
		host.setenv("PREFIX", "/data/data/com.termux/files/usr")
		host.setenv("PATH", "/data/data/com.termux/files/usr/bin")
		host.addDir("/data/data/com.termux/files/usr")
		host.addDir("/data/data/com.termux/files/usr/bin")
		host.addDir("/home/tester/storage")
		info := &system.SystemInfo{OS: system.OSTermux, OSName: "Termux", IsTermux: true, Prefix: "/data/data/com.termux/files/usr", HomeDir: "/home/tester", UserShell: "/bin/zsh"}
		findings := doctorCheckFindings(t, host, info, "env.platform")
		if len(findings) == 0 {
			t.Fatal("the Termux checks said nothing at all")
		}
		for _, finding := range findings {
			if finding.Status == doctorError || finding.Status == doctorWarning {
				t.Errorf("a Termux environment with its own prefix is reported as %s: %s", finding.Status, finding.Evidence)
			}
		}
	})

	t.Run("termux without its own prefix", func(t *testing.T) {
		host := doctorHealthyHost()
		// No $PREFIX and no shared storage: both are things a real Termux user has
		// to do once, and both are named with what to run.
		info := &system.SystemInfo{OS: system.OSTermux, OSName: "Termux", IsTermux: true, HomeDir: "/home/tester", UserShell: "/bin/zsh"}
		findings := doctorCheckFindings(t, host, info, "env.platform")
		prefix := doctorFindingWith(t, findings, "Termux prefix")
		if prefix.Status != doctorWarning {
			t.Errorf("a Termux without $PREFIX is reported as %s: %s", prefix.Status, prefix.Evidence)
		}
		storage := doctorFindingWith(t, findings, "Shared storage")
		if storage.Status != doctorWarning {
			t.Errorf("a Termux without shared storage is reported as %s: %s", storage.Status, storage.Evidence)
		}
		if !strings.Contains(storage.Advice, "termux-setup-storage") {
			t.Errorf("the advice does not name the command that grants it: %q", storage.Advice)
		}
	})

	t.Run("macos", func(t *testing.T) {
		host := doctorHealthyHost()
		host.goos = "darwin"
		host.addDir("/opt/homebrew/bin")
		info := &system.SystemInfo{OS: system.OSMac, OSName: "macOS", HomeDir: "/home/tester", UserShell: "/bin/zsh", Arch: "arm64"}
		findings := doctorCheckFindings(t, host, info, "env.platform")
		if len(findings) == 0 {
			t.Fatal("the macOS checks said nothing at all")
		}
	})
}
