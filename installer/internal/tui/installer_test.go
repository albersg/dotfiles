package tui

import (
	"os"
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
