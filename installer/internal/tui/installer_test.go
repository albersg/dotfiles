package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
