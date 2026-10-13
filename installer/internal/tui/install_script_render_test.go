package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// e2eRenderStepOrder is the order an Arch Linux install reaches the interactive
// steps whose scripts carry a root command. Arch is the fixture's platform
// because it is the one Linux target where the dependency, terminal and shell
// scripts each own their `sudo` call: there is no Homebrew step and the
// terminal is installed through pacman rather than by a downloaded installer.
// The order matches planFor, which is what the run actually executes.
var e2eRenderStepOrder = []string{"deps", "terminal", "setshell"}

// TestRenderInstallScriptsForE2E renders the real generated step scripts to
// DOTFILES_E2E_RENDER_DIR so the container harness in installer/e2e can run
// them against a real sudo. The scripts come from getInteractiveScript, the
// same dispatch table the installer calls, so the harness measures the bytes a
// run would execute instead of a copy that can drift from the generator.
//
// The test is skipped unless DOTFILES_E2E_RENDER_DIR is set, so it costs
// nothing in an ordinary `go test` run. It reads no host state beyond the
// builders' own decisions: PATH and HOME are pinned to empty directories so a
// brew or terminal binary on the build machine cannot move the render onto a
// different branch.
func TestRenderInstallScriptsForE2E(t *testing.T) {
	dir := os.Getenv("DOTFILES_E2E_RENDER_DIR")
	if dir == "" {
		t.Skip("DOTFILES_E2E_RENDER_DIR not set; not an e2e render run")
	}

	// HOME and RepoDir are the container's paths, not the host's, so the rendered
	// bytes are the same on every machine and the harness can pre-create the
	// checkout the terminal script copies from.
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HOME", "/home/tester")

	m := &Model{
		SystemInfo: &system.SystemInfo{OS: system.OSArch},
		Choices:    UserChoices{Terminal: "ghostty", Shell: "fish"},
		RepoDir:    "/home/tester/dotfiles",
	}

	var manifest strings.Builder
	for _, stepID := range e2eRenderStepOrder {
		script, err := getInteractiveScript(stepID, m)
		if err != nil {
			t.Fatalf("getInteractiveScript(%q): %v", stepID, err)
		}
		if script == "" {
			t.Fatalf("getInteractiveScript(%q) returned an empty script; the harness would silently measure one step less", stepID)
		}
		if err := os.WriteFile(filepath.Join(dir, stepID+".sh"), []byte(script), 0o755); err != nil {
			t.Fatalf("writing %s.sh: %v", stepID, err)
		}
		manifest.WriteString(stepID + "\n")
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.txt"), []byte(manifest.String()), 0o644); err != nil {
		t.Fatalf("writing manifest.txt: %v", err)
	}
}
