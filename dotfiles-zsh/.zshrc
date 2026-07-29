export ZSH="$HOME/.oh-my-zsh"

# Enable Powerlevel10k instant prompt. Should stay close to the top of ~/.zshrc.
if [[ -r "${XDG_CACHE_HOME:-$HOME/.cache}/p10k-instant-prompt-${(%):-%n}.zsh" ]]; then
  source "${XDG_CACHE_HOME:-$HOME/.cache}/p10k-instant-prompt-${(%):-%n}.zsh"
fi

# Detect Termux
IS_TERMUX=0
if [[ -n "$TERMUX_VERSION" ]] || [[ -d "/data/data/com.termux" ]]; then
    IS_TERMUX=1
fi

# Set PATH based on platform
if [[ $IS_TERMUX -eq 1 ]]; then
    # Termux - use PREFIX for binaries
    export PATH="$PREFIX/bin:$HOME/.local/bin:$HOME/.cargo/bin:$PATH"
else
    export PATH="$HOME/.local/bin:$HOME/.opencode/bin:$HOME/.cargo/bin:$HOME/.volta/bin:$HOME/.bun/bin:$HOME/.nix-profile/bin:/nix/var/nix/profiles/default/bin:/usr/local/bin:$HOME/.config:$HOME/.cargo/bin:/usr/local/lib/*:$PATH"
fi

# Set nvim as default editor for opencode and other tools
export EDITOR="nvim"
export VISUAL="nvim"

if [[ $- == *i* ]]; then
    # Commands to run in interactive sessions can go here
fi

export LS_COLORS="di=38;5;67:ow=48;5;60:ex=38;5;132:ln=38;5;144:*.tar=38;5;180:*.zip=38;5;180:*.jpg=38;5;175:*.png=38;5;175:*.mp3=38;5;175:*.wav=38;5;175:*.txt=38;5;223:*.sh=38;5;132"
if [[ "$(uname)" == "Darwin" ]]; then
  alias ls='ls --color=auto'
else
  alias ls='gls --color=auto'
fi

# Homebrew setup (skip on Termux)
if [[ $IS_TERMUX -eq 0 ]]; then
    if [[ "$(uname)" == "Darwin" ]]; then
        # macOS - check for Apple Silicon vs Intel
        if [[ -f "/opt/homebrew/bin/brew" ]]; then
            # Apple Silicon (M1/M2/M3)
            BREW_BIN="/opt/homebrew/bin"
        elif [[ -f "/usr/local/bin/brew" ]]; then
            # Intel Mac
            BREW_BIN="/usr/local/bin"
        fi
    else
        # Linux
        BREW_BIN="/home/linuxbrew/.linuxbrew/bin"
    fi

    # Only eval brew shellenv if brew is installed
    if [[ -n "$BREW_BIN" && -f "$BREW_BIN/brew" ]]; then
        eval "$($BREW_BIN/brew shellenv)"

        # Keep standard Zsh autoload functions aligned with the installed brew Zsh.
        BREW_ZSH_FUNCTIONS="$($BREW_BIN/brew --prefix zsh)/share/zsh/functions"
        if [[ -d "$BREW_ZSH_FUNCTIONS" ]]; then
            typeset -U fpath
            fpath=("$BREW_ZSH_FUNCTIONS" $fpath)
        fi
    fi
fi

# Use the user-managed Node runtime consistently.  This must run after
# Homebrew's shell environment so `node` and `npm` do not fall back to the
# system packages.  Codex is installed through npm's user-global prefix.
if [[ $IS_TERMUX -eq 0 ]]; then
    export PATH="$HOME/.npm-global/bin:$PATH"
    export NVM_DIR="$HOME/.nvm"
    if [[ -s "$NVM_DIR/nvm.sh" ]]; then
        source "$NVM_DIR/nvm.sh"
        nvm use --silent 22.23.1
    fi
fi

# Zsh built-ins required by Oh My Zsh and completion plugins.
zmodload zsh/zutil
zmodload zsh/complist
autoload -Uz add-zsh-hook add-zle-hook-widget bashcompinit colors compinit is-at-least zmathfunc zrecompile

# Oh My Zsh must initialize before third-party plugins.
plugins=(
  command-not-found
)
source "$ZSH/oh-my-zsh.sh"

# Third-party plugins: syntax highlighting must be loaded last.
if [[ $IS_TERMUX -eq 1 ]]; then
    # Termux - plugins installed via pkg
    [[ -f "$PREFIX/share/zsh-autocomplete/zsh-autocomplete.plugin.zsh" ]] && source "$PREFIX/share/zsh-autocomplete/zsh-autocomplete.plugin.zsh"
    [[ -f "$PREFIX/share/zsh-autosuggestions/zsh-autosuggestions.zsh" ]] && source "$PREFIX/share/zsh-autosuggestions/zsh-autosuggestions.zsh"
    # Powerlevel10k on Termux - may need manual install
    [[ -f "$PREFIX/share/powerlevel10k/powerlevel10k.zsh-theme" ]] && source "$PREFIX/share/powerlevel10k/powerlevel10k.zsh-theme"
    [[ -f "$PREFIX/share/zsh-syntax-highlighting/zsh-syntax-highlighting.zsh" ]] && source "$PREFIX/share/zsh-syntax-highlighting/zsh-syntax-highlighting.zsh"
else
    source_first_existing() {
        local plugin_path
        for plugin_path in "$@"; do
            if [[ -f "$plugin_path" ]]; then
                source "$plugin_path"
                return
            fi
        done
    }

    BREW_SHARE="${BREW_BIN:+$(dirname "$BREW_BIN")/share}"

    # Prefer Homebrew, then fall back to native Linux package layouts.
    # zsh-autocomplete is intentionally disabled because it adds input lag.
    source_first_existing "$BREW_SHARE/zsh-autosuggestions/zsh-autosuggestions.zsh" \
        "/usr/share/zsh/plugins/zsh-autosuggestions/zsh-autosuggestions.zsh" \
        "/usr/share/zsh-autosuggestions/zsh-autosuggestions.zsh"
    source_first_existing "$BREW_SHARE/powerlevel10k/powerlevel10k.zsh-theme" \
        "/usr/share/zsh-theme-powerlevel10k/powerlevel10k.zsh-theme" \
        "/usr/share/powerlevel10k/powerlevel10k.zsh-theme"
    source_first_existing "$BREW_SHARE/zsh-syntax-highlighting/zsh-syntax-highlighting.zsh" \
        "/usr/share/zsh/plugins/zsh-syntax-highlighting/zsh-syntax-highlighting.zsh" \
        "/usr/share/zsh-syntax-highlighting/zsh-syntax-highlighting.zsh"
fi

export PROJECT_PATHS="/home/alanbuscaglia/work"
export FZF_DEFAULT_COMMAND="fd --hidden --strip-cwd-prefix --exclude .git"
export FZF_DEFAULT_T_COMMAND="$FZF_DEFAULT_COMMAND"
export FZF_ALT_COMMAND="fd --type=d --hidden --strip-cwd-prefix --exlude .git"

WM_VAR="$HERDR_ENV"
WM_CMD="herdr"

function start_if_needed() {
    if [[ $- == *i* ]] && command -v "$WM_CMD" >/dev/null 2>&1 && [[ -z "${WM_VAR#/}" ]] && [[ -z "$TMUX" ]] && [[ -z "$ZELLIJ" ]] && [[ -z "$HERDR_ENV" ]] && [[ -t 1 ]]; then
        exec $WM_CMD
    fi
}

# alias
alias fzfbat='fzf --preview="bat --theme=gruvbox-dark --color=always {}"'
alias fzfnvim='nvim $(fzf --preview="bat --theme=gruvbox-dark --color=always {}")'

export CARAPACE_BRIDGES='zsh,fish,bash,inshellisense'
zstyle ':completion:*' format $'\e[2;37mCompleting %d\e[m'
source <(carapace _carapace)

eval "$(fzf --zsh)"
eval "$(zoxide init zsh)"
eval "$(atuin init zsh)"

# To customize prompt, run `p10k configure` or edit ~/.p10k.zsh.
[[ ! -f ~/.p10k.zsh ]] || source ~/.p10k.zsh

start_if_needed
# WSL-specific: VS Code integration from Windows host
if grep -qi microsoft /proc/version 2>/dev/null; then
  CODE_BIN="/mnt/c/Program Files/Microsoft VS Code/bin"
  if [[ -d "$CODE_BIN" ]]; then
    export PATH="$PATH:$CODE_BIN"
  fi

  code() {
    local distro="${WSL_DISTRO_NAME}"
    local path="$(pwd)"
    "${CODE_BIN}/code" \
      --folder-uri "vscode-remote://wsl+${distro}${path}"
  }
fi
