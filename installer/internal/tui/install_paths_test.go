package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// repoRoot resolves the repository checkout from the package directory, so the
// tests exercise the very files the installer ships.
func repoRoot(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if !system.DirExists(filepath.Join(root, "installer")) {
		t.Fatalf("repository root not found at %s", root)
	}
	return root
}

// TestRepoAssetsExist guards the whole class of defect where a path moves in the
// repository and the installer keeps copying from the old location. Herdr and
// the Tmux plugin seed both broke exactly this way during the rebrand.
func TestRepoAssetsExist(t *testing.T) {
	root := repoRoot(t)

	optional := map[string]bool{}
	for _, asset := range optionalRepoAssets {
		optional[asset] = true
	}

	for _, asset := range repoAssets {
		if _, err := os.Stat(filepath.Join(root, asset)); err != nil {
			if optional[asset] {
				continue
			}
			t.Errorf("installer copies %s, which does not exist in the repository: %v", asset, err)
		}
	}

	if len(repoAssets) == 0 {
		t.Fatal("no repository assets declared")
	}
}

func TestStepInstallWMHerdrCopiesConfigFromRepository(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	withPackageCommandMocks(t, nil)

	m := NewModel()
	// OS is macOS so the Herdr binary step takes the (mocked) Homebrew route
	// instead of downloading a release.
	m.SystemInfo = &system.SystemInfo{OS: system.OSMac, HasBrew: true}
	m.Choices = UserChoices{OS: "mac", Shell: "zsh", WindowMgr: "herdr"}
	m.RepoDir = repoRoot(t)

	if err := stepInstallWM(&m); err != nil {
		t.Fatalf("herdr step failed: %v", err)
	}

	installed := filepath.Join(home, ".config", "herdr", "config.toml")
	got, err := os.ReadFile(installed)
	if err != nil {
		t.Fatalf("herdr configuration was not installed: %v", err)
	}
	want, err := os.ReadFile(filepath.Join(repoRoot(t), repoAssetHerdrConfig))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Error("installed herdr configuration does not match the repository copy")
	}
}

func TestStepInstallWMTmuxToleratesMissingPluginSeed(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	withPackageCommandMocks(t, nil)

	m := NewModel()
	m.SystemInfo = &system.SystemInfo{OS: system.OSMac, HasBrew: true}
	m.Choices = UserChoices{OS: "mac", Shell: "zsh", WindowMgr: "tmux"}
	m.RepoDir = repoRoot(t)

	// The repository ships no plugin seed: tmux.conf declares them through TPM.
	if system.DirExists(filepath.Join(m.RepoDir, repoAssetTmuxPlugins)) {
		t.Skip("a plugin seed is present again; this guard no longer applies")
	}

	if err := stepInstallWM(&m); err != nil {
		t.Fatalf("tmux step must not fail without a plugin seed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".tmux.conf")); err != nil {
		t.Errorf("tmux.conf was not installed: %v", err)
	}
}

func TestStepInstallShellZshInstallsZshenvAndZshrc(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	calls := withPackageCommandMocks(t, nil)

	m := NewModel()
	m.SystemInfo = &system.SystemInfo{OS: system.OSMac, HasBrew: true}
	m.Choices = UserChoices{OS: "mac", Shell: "zsh", WindowMgr: "herdr"}
	m.RepoDir = repoRoot(t)

	if err := stepInstallShell(&m); err != nil {
		t.Fatalf("zsh step failed: %v", err)
	}

	for _, tc := range []struct {
		asset string
		dst   string
	}{
		{repoAssetZshEnv, ".zshenv"},
		{repoAssetZshrc, ".zshrc"},
		{repoAssetP10k, ".p10k.zsh"},
	} {
		got, err := os.ReadFile(filepath.Join(home, tc.dst))
		if err != nil {
			t.Fatalf("%s was not installed: %v", tc.dst, err)
		}
		want, err := os.ReadFile(filepath.Join(m.RepoDir, tc.asset))
		if err != nil {
			t.Fatal(err)
		}
		// .zshrc is patched for the chosen window manager, so only check that the
		// content is derived from the repository copy.
		if !strings.HasPrefix(string(got), strings.SplitN(string(want), "\n", 2)[0]) {
			t.Errorf("%s does not derive from %s", tc.dst, tc.asset)
		}
	}

	// Oh My Zsh is delegated to its own installer, never vendored: a fresh HOME
	// has nothing there, so the official installer must have been invoked.
	if !callsContain(calls, "oh-my-zsh") {
		t.Error("the official Oh My Zsh installer was not invoked on a fresh HOME")
	}
}

func callsContain(calls *[]packageCommandCall, runner string) bool {
	for _, call := range *calls {
		if call.runner == runner {
			return true
		}
	}
	return false
}

// TestShouldInstallOhMyZsh covers the guard: only a missing installation is
// installed. A real directory and a symlinked one both count as present, which
// is what stops the installer from overwriting a clone that manages itself.
func TestShouldInstallOhMyZsh(t *testing.T) {
	t.Run("missing directory wants an install", func(t *testing.T) {
		if !shouldInstallOhMyZsh(filepath.Join(t.TempDir(), ".oh-my-zsh")) {
			t.Error("a missing ~/.oh-my-zsh must be installed")
		}
	})

	t.Run("real directory is left alone", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), ".oh-my-zsh")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if shouldInstallOhMyZsh(dir) {
			t.Error("an existing ~/.oh-my-zsh must not be reinstalled")
		}
	})

	t.Run("symlinked directory counts as installed", func(t *testing.T) {
		home := t.TempDir()
		real := filepath.Join(home, "ohmyzsh-real")
		if err := os.MkdirAll(real, 0o755); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(home, ".oh-my-zsh")
		if err := os.Symlink(real, link); err != nil {
			t.Skipf("symlinks are not available here: %v", err)
		}
		if shouldInstallOhMyZsh(link) {
			t.Error("a symlinked ~/.oh-my-zsh must not be reinstalled")
		}
	})

	t.Run("broken symlink is not an install", func(t *testing.T) {
		home := t.TempDir()
		link := filepath.Join(home, ".oh-my-zsh")
		if err := os.Symlink(filepath.Join(home, "missing"), link); err != nil {
			t.Skipf("symlinks are not available here: %v", err)
		}
		if !shouldInstallOhMyZsh(link) {
			t.Error("a broken symlink is not a working installation")
		}
	})
}

// TestStepInstallShellNeverTouchesAnExistingOhMyZsh pins the reason the vendored
// tree was removed: writing into a real clone dirties its tracked files and
// breaks `omz update`.
func TestStepInstallShellNeverTouchesAnExistingOhMyZsh(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	calls := withPackageCommandMocks(t, nil)

	omz := filepath.Join(home, ".oh-my-zsh")
	if err := os.MkdirAll(filepath.Join(omz, "themes"), 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(omz, "oh-my-zsh.sh")
	if err := os.WriteFile(marker, []byte("# user clone\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := NewModel()
	m.SystemInfo = &system.SystemInfo{OS: system.OSMac, HasBrew: true}
	m.Choices = UserChoices{OS: "mac", Shell: "zsh", WindowMgr: "herdr"}
	m.RepoDir = repoRoot(t)

	if err := stepInstallShell(&m); err != nil {
		t.Fatalf("zsh step failed: %v", err)
	}

	if callsContain(calls, "oh-my-zsh") {
		t.Error("an existing ~/.oh-my-zsh must not be reinstalled")
	}
	got, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("the existing installation was removed: %v", err)
	}
	if string(got) != "# user clone\n" {
		t.Errorf("the existing installation was overwritten: %q", got)
	}
}

// TestPatchZshForWMHandlesTheShippedZshrc keeps the window-manager patching
// verified against the real configuration instead of a mock, so the markers it
// depends on cannot silently disappear.
func TestPatchZshForWMHandlesTheShippedZshrc(t *testing.T) {
	source := filepath.Join(repoRoot(t), repoAssetZshrc)
	original, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}

	for _, wm := range []string{"tmux", "zellij", "herdr", "none"} {
		t.Run(wm, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), ".zshrc")
			if err := os.WriteFile(target, original, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := system.PatchZshForWM(target, wm, true); err != nil {
				t.Fatalf("patch failed: %v", err)
			}
			patched, err := os.ReadFile(target)
			if err != nil {
				t.Fatal(err)
			}

			wantWMVar := wm != "none"
			if got := strings.Contains(string(patched), `WM_VAR="`); got != wantWMVar {
				t.Errorf("WM_VAR present = %v, want %v", got, wantWMVar)
			}
			if got := strings.Contains(string(patched), `WM_CMD="`); got != wantWMVar {
				t.Errorf("WM_CMD present = %v, want %v", got, wantWMVar)
			}

			expects := map[string]string{"tmux": `WM_CMD="tmux"`, "zellij": `WM_CMD="zellij"`, "herdr": `WM_CMD="herdr"`}
			if expect, ok := expects[wm]; ok && !strings.Contains(string(patched), expect) {
				t.Errorf("patched .zshrc is missing %s", expect)
			}
		})
	}
}

func TestDryRunSkipsEveryStep(t *testing.T) {
	t.Setenv("DOTFILES_DRY_RUN", "1")
	home := t.TempDir()
	t.Setenv("HOME", home)

	m := NewModel()
	m.SystemInfo = &system.SystemInfo{OS: system.OSDebian, HasBrew: false}
	m.Choices = UserChoices{OS: "linux", Shell: "zsh", WindowMgr: "herdr", InstallNvim: true}

	for _, stepID := range []string{"backup", "deps", "clone", "homebrew", "shell", "wm", "nvim", "wslconfig", "cleanup", "setshell"} {
		if err := executeStep(stepID, &m); err != nil {
			t.Errorf("dry run must not run step %q: %v", stepID, err)
		}
	}

	if m.RepoDir != "" || m.WorkDir != "" {
		t.Errorf("dry run must not clone anything: WorkDir=%q RepoDir=%q", m.WorkDir, m.RepoDir)
	}
	entries, err := os.ReadDir(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("dry run wrote into HOME: %v", entries)
	}
}
