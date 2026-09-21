package tui

import (
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// interactiveFlaggedStepIDs returns every step ID that SetupInstallSteps can
// mark Interactive, across the configuration space the installer supports.
//
// It is derived from the step catalog rather than restated by hand, so it cannot
// drift from the flags: if a configuration adds an interactive step, the matrix
// below produces it and the invariant tests see it. The matrix crosses every
// field SetupInstallSteps reads, so a future step gated on any one of them is
// still exercised.
func interactiveFlaggedStepIDs(t *testing.T) map[string]bool {
	t.Helper()

	flagged := map[string]bool{}

	for _, sysInfo := range allSystemInfos() {
		for _, choices := range allUserChoices() {
			m := Model{
				SystemInfo:      sysInfo,
				Choices:         choices,
				ExistingConfigs: []string{".config/nvim"},
			}
			m.SetupInstallSteps()
			for _, step := range m.Steps {
				if step.Interactive {
					flagged[step.ID] = true
				}
			}
		}
	}

	return flagged
}

// allSystemInfos returns the cross product of the SystemInfo fields
// SetupInstallSteps reads.
func allSystemInfos() []*system.SystemInfo {
	var out []*system.SystemInfo

	for _, osName := range []system.OSType{
		system.OSMac, system.OSLinux, system.OSDebian,
		system.OSArch, system.OSFedora, system.OSWSL, system.OSTermux,
	} {
		for _, isWSL := range []bool{false, true} {
			for _, isTermux := range []bool{false, true} {
				for _, hasBrew := range []bool{false, true} {
					for _, hasXcode := range []bool{false, true} {
						out = append(out, &system.SystemInfo{
							OS:       osName,
							IsWSL:    isWSL,
							IsTermux: isTermux,
							HasBrew:  hasBrew,
							HasXcode: hasXcode,
						})
					}
				}
			}
		}
	}

	return out
}

// allUserChoices returns the cross product of the UserChoices fields
// SetupInstallSteps reads.
func allUserChoices() []UserChoices {
	var out []UserChoices

	for _, osName := range []string{"", "mac", "linux", "termux"} {
		for _, terminal := range []string{"", "none", "ghostty"} {
			for _, installFont := range []bool{false, true} {
				for _, shell := range []string{"fish", "zsh", "nushell"} {
					for _, windowMgr := range []string{"", "none", "tmux"} {
						for _, installNvim := range []bool{false, true} {
							for _, createBackup := range []bool{false, true} {
								out = append(out, UserChoices{
									OS:           osName,
									Terminal:     terminal,
									InstallFont:  installFont,
									Shell:        shell,
									WindowMgr:    windowMgr,
									InstallNvim:  installNvim,
									CreateBackup: createBackup,
								})
							}
						}
					}
				}
			}
		}
	}

	return out
}

// TestEveryInteractiveStepIsDispatchable is the invariant behind issue #28:
// SetupInstallSteps marks a step Interactive, the step loop routes it to
// getInteractiveScript, and a step flagged Interactive that the dispatch table
// does not know is a contradiction the code can detect. Before the fix for #28
// this failed on wslconfig, which was flagged Interactive with no case, so the
// TUI died at the WSL step before writing either file.
func TestEveryInteractiveStepIsDispatchable(t *testing.T) {
	// Keep the check hermetic: no real Windows profile, no cmd.exe interop and
	// no network. The invariant only asks whether a dispatch case exists; the
	// script's behaviour is covered separately.
	t.Setenv(envWSLWindowsHome, t.TempDir())
	t.Setenv(envWSLConfPath, t.TempDir()+"/wsl.conf")
	t.Setenv(envBinfmtDir, t.TempDir())

	repoDir := t.TempDir()
	m := &Model{
		// A model rich enough that generating any interactive script succeeds.
		RepoDir:    repoDir,
		SystemInfo: &system.SystemInfo{OS: system.OSLinux, IsWSL: true},
		Choices:    UserChoices{Shell: "zsh", Terminal: "none", WindowMgr: "none"},
	}

	flagged := interactiveFlaggedStepIDs(t)
	for stepID := range flagged {
		if _, err := getInteractiveScript(stepID, m); err != nil &&
			strings.Contains(err.Error(), "unknown interactive step") {
			t.Errorf("step %q is flagged Interactive in SetupInstallSteps but getInteractiveScript has no case for it: %v",
				stepID, err)
		}
	}
}

// TestEveryInteractiveDispatchCaseIsAFlaggedStep is the other half of the
// invariant: a case the dispatch table can run, but that no configuration ever
// flags Interactive, is either dead code or a step whose flag was lost. Keeping
// both directions honest is what turns "the next defect surfaces in the field"
// into "the next defect fails in CI".
func TestEveryInteractiveDispatchCaseIsAFlaggedStep(t *testing.T) {
	flagged := interactiveFlaggedStepIDs(t)
	for stepID := range interactiveScriptBuilders {
		if !flagged[stepID] {
			t.Errorf("interactiveScriptBuilders has a case for %q, but no SetupInstallSteps configuration marks that step Interactive",
				stepID)
		}
	}
}
