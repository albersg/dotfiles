package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// stageShellRepo stages a throwaway repository checkout holding exactly the
// assets stepInstallShell copies for the zsh branch. The bat theme is included
// only when asked for, so the guard can be exercised against a checkout that
// predates it without touching the real checkout.
func stageShellRepo(t *testing.T, withBatTheme bool) string {
	t.Helper()

	src := repoRoot(t)
	dst := t.TempDir()

	assets := []string{
		repoAssetGitconfig,
		repoAssetGitconfigPersonal,
		repoAssetZshEnv,
		repoAssetZshrc,
		repoAssetP10k,
	}
	if withBatTheme {
		assets = append(assets, repoAssetBatTheme)
	}

	for _, asset := range assets {
		if err := system.CopyFile(filepath.Join(src, asset), filepath.Join(dst, asset)); err != nil {
			t.Fatalf("could not stage %s into the throwaway checkout: %v", asset, err)
		}
	}
	return dst
}

func zshShellModel(repoDir string) Model {
	m := NewModel()
	m.SystemInfo = &system.SystemInfo{OS: system.OSMac, HasBrew: true}
	m.Choices = UserChoices{OS: "mac", Shell: "zsh", WindowMgr: "herdr"}
	m.RepoDir = repoDir
	return m
}

// TestStepInstallShellInstallsBatThemeOnlyWhenTheCheckoutShipsIt pins both sides
// of the bat theme guard. The theme is the newest asset the installer copies, so
// the binary can be newer than the checkout it clones. A checkout that ships the
// theme must still install it; a checkout that predates it must skip the whole
// install rather than abort the shell step, and the skip must leave no
// ~/.config/bat behind on a machine that had none.
func TestStepInstallShellInstallsBatThemeOnlyWhenTheCheckoutShipsIt(t *testing.T) {
	t.Run("the checkout ships the theme", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		withZshMocks(t, home)

		repoDir := stageShellRepo(t, true)
		m := zshShellModel(repoDir)

		if err := stepInstallShell(&m); err != nil {
			t.Fatalf("a checkout that ships the theme must install it cleanly: %v", err)
		}

		installed := filepath.Join(home, ".config", "bat", "themes", "dotfiles.tmTheme")
		got, err := os.ReadFile(installed)
		if err != nil {
			t.Fatalf("the theme was not installed from a checkout that ships it: %v", err)
		}
		want, err := os.ReadFile(filepath.Join(repoDir, repoAssetBatTheme))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Error("the installed theme does not match the checkout copy")
		}
	})

	t.Run("the checkout predates the theme", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		withZshMocks(t, home)

		repoDir := stageShellRepo(t, false)
		m := zshShellModel(repoDir)

		if err := stepInstallShell(&m); err != nil {
			t.Fatalf("a checkout without the theme must not abort the shell step: %v", err)
		}

		if _, err := os.Stat(filepath.Join(home, ".config", "bat")); !os.IsNotExist(err) {
			t.Errorf("a skipped theme install must leave no ~/.config/bat behind: %v", err)
		}
	})
}
