package system

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// zshrcFixture mirrors the multiplexer block shipped in dotfiles-zsh/.zshrc,
// so the patcher is exercised against the real shape it must rewrite.
const zshrcFixture = `# Enable Powerlevel10k instant prompt.
if [[ -r "${XDG_CACHE_HOME:-$HOME/.cache}/p10k-instant-prompt-${(%):-%n}.zsh" ]]; then
  source "${XDG_CACHE_HOME:-$HOME/.cache}/p10k-instant-prompt-${(%):-%n}.zsh"
fi

export ZSH="$HOME/.oh-my-zsh"

WM_VAR="$HERDR_ENV"
# WM_CMD array: zsh does not word-split an unquoted parameter, so a multi-word command must be launched as "${WM_CMD[@]}".
typeset -a WM_CMD
WM_CMD=(herdr)

function start_if_needed() {
    if [[ $- == *i* ]] && command -v "${WM_CMD[1]}" >/dev/null 2>&1 && [[ -z "${WM_VAR#/}" ]] && [[ -t 1 ]]; then
        exec "${WM_CMD[@]}"
    fi
}

alias fzfbat='fzf --preview="bat --theme=gruvbox-dark --color=always {}"'

eval "$(fzf --zsh)"
eval "$(zoxide init zsh)"

start_if_needed
`

// zshWMCommandLineOf returns the single WM_CMD assignment line of a patched
// .zshrc, so tests can assert the exact line instead of a loose substring.
func zshWMCommandLineOf(content string) string {
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "WM_CMD=") {
			return line
		}
	}
	return ""
}

func TestPatchZshForWM(t *testing.T) {
	tests := []struct {
		name           string
		wm             string
		installNvim    bool
		wantWMCommand  string
		wantContain    []string
		wantNotContain []string
	}{
		{
			name:          "WM none should remove all WM-related lines",
			wm:            "none",
			installNvim:   true,
			wantWMCommand: "",
			wantContain: []string{
				"eval \"$(zoxide init zsh)\"",
				"alias fzfbat",
			},
			wantNotContain: []string{
				"WM_VAR",
				"WM_CMD",
				"typeset -a WM_CMD",
				"# WM_CMD array:",
				"start_if_needed",
				"function start_if_needed",
				`exec "${WM_CMD[@]}"`,
			},
		},
		{
			name:        "WM none without nvim should wrap fzf",
			wm:          "none",
			installNvim: false,
			wantContain: []string{
				"if command -v fzf",
				"eval \"$(fzf --zsh)\"",
				"fi",
			},
			wantNotContain: []string{
				"WM_VAR",
				"start_if_needed",
			},
		},
		{
			name:          "WM zellij should attach to the main session",
			wm:            "zellij",
			installNvim:   true,
			wantWMCommand: `WM_CMD=(zellij attach -c main)`,
			wantContain: []string{
				`WM_VAR="$ZELLIJ"`,
				`command -v "${WM_CMD[1]}"`,
				"start_if_needed",
				"typeset -a WM_CMD",
			},
			wantNotContain: []string{
				`WM_VAR="/$TMUX"`,
				`WM_CMD=(tmux new-session -A -s main)`,
				`WM_CMD=(herdr)`,
				"# change with ZELLIJ",
			},
		},
		{
			name:          "WM herdr should keep the bare binary name",
			wm:            "herdr",
			installNvim:   true,
			wantWMCommand: `WM_CMD=(herdr)`,
			wantContain: []string{
				`WM_VAR="$HERDR_ENV"`,
				`command -v "${WM_CMD[1]}"`,
				"start_if_needed",
			},
			wantNotContain: []string{
				`WM_VAR="/$TMUX"`,
				`WM_CMD=(tmux new-session -A -s main)`,
				`WM_CMD=(zellij attach -c main)`,
				"# change with ZELLIJ",
			},
		},
		{
			name:          "WM tmux should attach to the main session",
			wm:            "tmux",
			installNvim:   true,
			wantWMCommand: `WM_CMD=(tmux new-session -A -s main)`,
			wantContain: []string{
				`WM_VAR="/$TMUX"`,
				`command -v "${WM_CMD[1]}"`,
				"start_if_needed",
			},
			wantNotContain: []string{
				`WM_CMD=(herdr)`,
				`WM_CMD=(zellij attach -c main)`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpDir := t.TempDir()
			zshrcPath := filepath.Join(tmpDir, ".zshrc")

			if err := os.WriteFile(zshrcPath, []byte(zshrcFixture), 0644); err != nil {
				t.Fatalf("Failed to write temp file: %v", err)
			}

			// Run the patch
			if err := PatchZshForWM(zshrcPath, tt.wm, tt.installNvim); err != nil {
				t.Fatalf("PatchZshForWM failed: %v", err)
			}

			// Read result
			result, err := os.ReadFile(zshrcPath)
			if err != nil {
				t.Fatalf("Failed to read patched file: %v", err)
			}
			content := string(result)

			if got := zshWMCommandLineOf(content); got != tt.wantWMCommand {
				t.Errorf("WM_CMD line = %q, want %q.\nContent:\n%s", got, tt.wantWMCommand, content)
			}

			// Check expected content
			for _, want := range tt.wantContain {
				if !strings.Contains(content, want) {
					t.Errorf("Expected content to contain %q, but it didn't.\nContent:\n%s", want, content)
				}
			}

			// Check content that should not be present
			for _, notWant := range tt.wantNotContain {
				if strings.Contains(content, notWant) {
					t.Errorf("Expected content NOT to contain %q, but it did.\nContent:\n%s", notWant, content)
				}
			}
		})
	}
}

// TestPatchZshForWMIsIdempotent pins the property the installer relies on:
// re-patching an already-patched .zshrc must not change it again.
func TestPatchZshForWMIsIdempotent(t *testing.T) {
	for _, wm := range []string{"tmux", "zellij", "herdr", "none"} {
		t.Run(wm, func(t *testing.T) {
			zshrcPath := filepath.Join(t.TempDir(), ".zshrc")
			if err := os.WriteFile(zshrcPath, []byte(zshrcFixture), 0644); err != nil {
				t.Fatalf("Failed to write temp file: %v", err)
			}

			if err := PatchZshForWM(zshrcPath, wm, true); err != nil {
				t.Fatalf("first PatchZshForWM failed: %v", err)
			}
			first, err := os.ReadFile(zshrcPath)
			if err != nil {
				t.Fatalf("Failed to read patched file: %v", err)
			}

			if err := PatchZshForWM(zshrcPath, wm, true); err != nil {
				t.Fatalf("second PatchZshForWM failed: %v", err)
			}
			second, err := os.ReadFile(zshrcPath)
			if err != nil {
				t.Fatalf("Failed to read re-patched file: %v", err)
			}

			if string(first) != string(second) {
				t.Errorf("second patch changed the file.\nfirst:\n%s\nsecond:\n%s", first, second)
			}
		})
	}
}

// TestPatchZshForWMUpgradesLegacyCommandForms covers existing installations,
// whose .zshrc still holds the scalar WM_CMD="..." assignment.
func TestPatchZshForWMUpgradesLegacyCommandForms(t *testing.T) {
	legacy := `WM_VAR="/$TMUX"
WM_CMD="tmux"

function start_if_needed() {
    if [[ $- == *i* ]] && command -v "$WM_CMD" >/dev/null 2>&1 && [[ -z "${WM_VAR#/}" ]] && [[ -t 1 ]]; then
        exec $WM_CMD
    fi
}

start_if_needed
`

	expects := map[string]string{
		"tmux":   `WM_CMD=(tmux new-session -A -s main)`,
		"zellij": `WM_CMD=(zellij attach -c main)`,
		"herdr":  `WM_CMD=(herdr)`,
	}

	for wm, expect := range expects {
		t.Run(wm, func(t *testing.T) {
			zshrcPath := filepath.Join(t.TempDir(), ".zshrc")
			if err := os.WriteFile(zshrcPath, []byte(legacy), 0644); err != nil {
				t.Fatalf("Failed to write temp file: %v", err)
			}

			if err := PatchZshForWM(zshrcPath, wm, true); err != nil {
				t.Fatalf("PatchZshForWM failed: %v", err)
			}
			result, err := os.ReadFile(zshrcPath)
			if err != nil {
				t.Fatalf("Failed to read patched file: %v", err)
			}
			content := string(result)

			if got := zshWMCommandLineOf(content); got != expect {
				t.Errorf("WM_CMD line = %q, want %q.\nContent:\n%s", got, expect, content)
			}
			if strings.Contains(content, `WM_CMD="`) {
				t.Errorf("legacy scalar WM_CMD survived:\n%s", content)
			}
			if !strings.Contains(content, `command -v "${WM_CMD[1]}"`) {
				t.Errorf("guard still checks the whole array instead of the binary:\n%s", content)
			}
		})
	}
}

func TestPatchFishForWM(t *testing.T) {
	fishContent := `if status is-interactive
    # Commands to run in interactive sessions can go here
end

if test (uname) = Darwin
    if test -f /opt/homebrew/bin/brew
        set BREW_BIN /opt/homebrew/bin/brew
    end
end

if not set -q TMUX
    tmux
end

#if not set -q ZELLIJ 
#  zellij
#end

starship init fish | source
zoxide init fish | source
fzf --fish | source

alias fzfbat='fzf --preview="bat --theme=gruvbox-dark --color=always {}"'
`

	tests := []struct {
		name           string
		wm             string
		installNvim    bool
		wantContain    []string
		wantNotContain []string
	}{
		{
			name:        "WM none should remove tmux block",
			wm:          "none",
			installNvim: true,
			wantContain: []string{
				"starship init fish | source",
				"zoxide init fish | source",
			},
			wantNotContain: []string{
				"if not set -q TMUX",
				"tmux",
				"#if not set -q ZELLIJ",
			},
		},
		{
			name:        "WM none without nvim should wrap fzf",
			wm:          "none",
			installNvim: false,
			wantContain: []string{
				"if command -v fzf",
				"fzf --fish | source",
				"end",
			},
			wantNotContain: []string{
				"if not set -q TMUX",
			},
		},
		{
			name:        "WM zellij should configure zellij and remove tmux",
			wm:          "zellij",
			installNvim: true,
			wantContain: []string{
				"command -q zellij",
				"not set -q ZELLIJ",
				"zellij attach -c main",
			},
			wantNotContain: []string{
				"if not set -q TMUX",
				"#if not set -q ZELLIJ",
			},
		},
		{
			name:        "WM herdr should use herdr guard",
			wm:          "herdr",
			installNvim: true,
			wantContain: []string{
				"command -q herdr",
				"not set -q HERDR_ENV",
				"herdr; or echo",
			},
			wantNotContain: []string{
				"if not set -q TMUX",
				"#if not set -q ZELLIJ",
			},
		},
		{
			name:        "WM tmux should keep original",
			wm:          "tmux",
			installNvim: true,
			wantContain: []string{
				"command -q tmux",
				"tmux new-session -A -s main",
			},
			wantNotContain: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			fishPath := filepath.Join(tmpDir, "config.fish")

			if err := os.WriteFile(fishPath, []byte(fishContent), 0644); err != nil {
				t.Fatalf("Failed to write temp file: %v", err)
			}

			if err := PatchFishForWM(fishPath, tt.wm, tt.installNvim); err != nil {
				t.Fatalf("PatchFishForWM failed: %v", err)
			}

			result, err := os.ReadFile(fishPath)
			if err != nil {
				t.Fatalf("Failed to read patched file: %v", err)
			}
			content := string(result)

			for _, want := range tt.wantContain {
				if !strings.Contains(content, want) {
					t.Errorf("Expected content to contain %q, but it didn't.\nContent:\n%s", want, content)
				}
			}

			for _, notWant := range tt.wantNotContain {
				if strings.Contains(content, notWant) {
					t.Errorf("Expected content NOT to contain %q, but it did.\nContent:\n%s", notWant, content)
				}
			}
		})
	}
}

func TestPatchNushellForWM(t *testing.T) {
	nushellContent := `$env.config = {
    show_banner: false
}

def fzfbat [] {
  fzf --preview "bat --theme=gruvbox-dark --color=always {}" 
}

source ~/.zoxide.nu
source ~/.cache/carapace/init.nu

let MULTIPLEXER = "tmux" 
let MULTIPLEXER_ENV_PREFIX = "TMUX"

def start_multiplexer [] {
  if $MULTIPLEXER_ENV_PREFIX not-in ($env | columns) {
    run-external $MULTIPLEXER
  }
}

start_multiplexer
`

	tests := []struct {
		name           string
		wm             string
		wantContain    []string
		wantNotContain []string
	}{
		{
			name: "WM none should remove multiplexer code",
			wm:   "none",
			wantContain: []string{
				"$env.config",
				"def fzfbat",
				"source ~/.zoxide.nu",
			},
			wantNotContain: []string{
				"let MULTIPLEXER",
				"def start_multiplexer",
				"start_multiplexer",
				"run-external",
			},
		},
		{
			name: "WM zellij should replace tmux with zellij",
			wm:   "zellij",
			wantContain: []string{
				"let MULTIPLEXER = \"zellij\"",
				"let MULTIPLEXER_ENV_PREFIX = \"ZELLIJ\"",
				"start_multiplexer",
			},
			wantNotContain: []string{
				"let MULTIPLEXER = \"tmux\"",
				"let MULTIPLEXER_ENV_PREFIX = \"TMUX\"",
			},
		},
		{
			name: "WM herdr should replace tmux with herdr",
			wm:   "herdr",
			wantContain: []string{
				"let MULTIPLEXER = \"herdr\"",
				"let MULTIPLEXER_ENV_PREFIX = \"HERDR_ENV\"",
				"which $MULTIPLEXER",
				"start_multiplexer",
			},
			wantNotContain: []string{
				"let MULTIPLEXER = \"tmux\"",
				"let MULTIPLEXER_ENV_PREFIX = \"TMUX\"",
			},
		},
		{
			name: "WM tmux should keep original",
			wm:   "tmux",
			wantContain: []string{
				"let MULTIPLEXER = \"tmux\"",
				"let MULTIPLEXER_ENV_PREFIX = \"TMUX\"",
				"start_multiplexer",
			},
			wantNotContain: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			nuPath := filepath.Join(tmpDir, "config.nu")

			if err := os.WriteFile(nuPath, []byte(nushellContent), 0644); err != nil {
				t.Fatalf("Failed to write temp file: %v", err)
			}

			if err := PatchNushellForWM(nuPath, tt.wm); err != nil {
				t.Fatalf("PatchNushellForWM failed: %v", err)
			}

			result, err := os.ReadFile(nuPath)
			if err != nil {
				t.Fatalf("Failed to read patched file: %v", err)
			}
			content := string(result)

			for _, want := range tt.wantContain {
				if !strings.Contains(content, want) {
					t.Errorf("Expected content to contain %q, but it didn't.\nContent:\n%s", want, content)
				}
			}

			for _, notWant := range tt.wantNotContain {
				if strings.Contains(content, notWant) {
					t.Errorf("Expected content NOT to contain %q, but it did.\nContent:\n%s", notWant, content)
				}
			}
		})
	}
}

func TestPatchZshForWM_FileNotFound(t *testing.T) {
	err := PatchZshForWM("/nonexistent/path/.zshrc", "none", true)
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}

func TestPatchFishForWM_FileNotFound(t *testing.T) {
	err := PatchFishForWM("/nonexistent/path/config.fish", "none", true)
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}

func TestPatchNushellForWM_FileNotFound(t *testing.T) {
	err := PatchNushellForWM("/nonexistent/path/config.nu", "none")
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}
