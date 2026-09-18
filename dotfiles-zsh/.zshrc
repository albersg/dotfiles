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

# Use fnm as the single Node version manager.  This must run after
# Homebrew's shell environment so fnm resolves from the managed brew path.
# The npm global prefix is deliberately independent from the Node install:
# all global CLIs live in ~/.npm-global, while fnm owns Node itself.
if [[ $IS_TERMUX -eq 0 ]] && command -v fnm >/dev/null 2>&1; then
    unset NVM_DIR
    export FNM_DIR="$HOME/.local/share/fnm"

    # Corepack is no longer bundled with Node 25+, and enabling it through fnm
    # makes `fnm install` fail on those versions with "Can't enable corepack".
    # Enable it per installation (`corepack enable`) when a project needs it;
    # the shims already present in the current install keep working.
    export FNM_COREPACK_ENABLED=false

    # Remove inherited nvm entries before fnm creates its multishell path.
    # `path` is zsh's array-backed form of PATH.
    typeset -U path
    path=("${(@)path:#$HOME/.nvm/versions/node/*/bin}")
    # `n` ships an unmanaged Node build; it must never shadow fnm.
    path=("${(@)path:#$HOME/.n/bin}")

    eval "$(fnm env --use-on-cd --shell zsh)"

    # Start on the `default` alias, unless the current directory declares its
    # own version (same condition the use-on-cd hook evaluates).
    if [[ ! -f .node-version && ! -f .nvmrc && ! -f package.json ]]; then
        fnm use default --silent-if-unchanged >/dev/null
    fi

    export NPM_CONFIG_PREFIX="$HOME/.npm-global"
    path=("$HOME/.npm-global/bin" $path)
    typeset -U path
fi

# Use the system CA bundle for Node-based tooling. Linux distributions ship it
# at this path; macOS and Termux do not, so the export is guarded.
if [[ -f "/etc/ssl/certs/ca-certificates.crt" ]]; then
    export NODE_OPTIONS="${NODE_OPTIONS:+$NODE_OPTIONS }--use-openssl-ca"
    export SSL_CERT_FILE="/etc/ssl/certs/ca-certificates.crt"
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

export PROJECT_PATHS="$HOME/work"
export FZF_DEFAULT_COMMAND="fd --hidden --strip-cwd-prefix --exclude .git"
export FZF_CTRL_T_COMMAND="$FZF_DEFAULT_COMMAND"
export FZF_ALT_COMMAND="fd --type=d --hidden --strip-cwd-prefix --exclude .git"

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

# --- Modern Unix replacements (transparent: cat→bat, ls→eza, grep→rg) ---
alias cat='bat --paging=never'
alias ls='eza --icons --group-directories-first'
alias ll='eza -l --icons --git --group-directories-first'
alias la='eza -la --icons --git --group-directories-first'
alias tree='eza --tree --icons --level=3'
alias grep='rg --no-heading'

# --- Network tools ---
# `trip` (trippy) is only aliased when it is actually resolvable, so the alias
# never breaks shells on hosts where it is not installed.
if command -v trip >/dev/null 2>&1; then
    alias traceroute='sudo trip'
    alias tracert='sudo trip'
fi
alias http='xh'              # xh > curl for APIs

# bat theme (use the one that matches your terminal palette)
export BAT_THEME="gruvbox-dark"

export CARAPACE_BRIDGES='zsh,fish,bash,inshellisense'
zstyle ':completion:*' format $'\e[2;37mCompleting %d\e[m'
source <(carapace _carapace)

eval "$(fzf --zsh)"
eval "$(zoxide init zsh)"
eval "$(atuin init zsh)"

# --- direnv: per-directory env vars (works with mise) ---
eval "$(direnv hook zsh)"

# To customize prompt, run `p10k configure` or edit ~/.p10k.zsh.
[[ ! -f ~/.p10k.zsh ]] || source ~/.p10k.zsh

start_if_needed

# ─── WSL2 cross-platform support ──────────────────────────────────────────────
if grep -qi microsoft /proc/version 2>/dev/null; then
  # --- WSL detection ----------------------------------------------------------
  IS_WSL=1
  # Keep WSL_INTEROP untouched: WSL sets it to the current interop socket.
  # Use a separate variable for the runtime directory instead.
  WSL_RUNTIME_DIR="/run/WSL"
  # Recover shells started from an older session that incorrectly exported the
  # runtime directory instead of a socket; WSL will select the interop socket.
  if [[ -n "${WSL_INTEROP:-}" && -d "$WSL_INTEROP" ]]; then
    unset WSL_INTEROP
  fi
  WSL_DISTRO="${WSL_DISTRO_NAME:-unknown}"

  # --- WSLg DISPLAY (auto-set by WSLg, but guard for headless scenarios) -----
   if [[ -z "$DISPLAY" ]] && [[ -f "$WSL_RUNTIME_DIR/interop" ]]; then
     export DISPLAY=":0"
   fi
   if [[ -z "$WAYLAND_DISPLAY" ]] && [[ -f "$WSL_RUNTIME_DIR/interop" ]]; then
     export WAYLAND_DISPLAY="wayland-0"
   fi

  # --- BROWSER: prefer wslview (from wslu package), fallback chain -----------
  if command -v wslview &>/dev/null; then
    export BROWSER="wslview"
  elif command -v powershell.exe &>/dev/null; then
    export BROWSER="powershell.exe -Command Start-Process"
  elif [[ -f "/mnt/c/Program Files/Google/Chrome/Application/chrome.exe" ]]; then
    export BROWSER='"/mnt/c/Program Files/Google/Chrome/Application/chrome.exe"'
  elif [[ -f "/mnt/c/Program Files (x86)/Microsoft/Edge/Application/msedge.exe" ]]; then
    export BROWSER='"/mnt/c/Program Files (x86)/Microsoft/Edge/Application/msedge.exe"'
  fi

  # --- Docker Desktop integration (common WSL setup) -------------------------
  if [[ -S "/var/run/docker.sock" ]] && command -v docker &>/dev/null; then
    export DOCKER_HOST="unix:///var/run/docker.sock"
  fi

  # --- Clipboard integration via clip.exe (available by default in WSL) ------
  if command -v clip.exe &>/dev/null; then
    alias clip="clip.exe"
  fi
  if command -v powershell.exe &>/dev/null; then
    alias pbcopy="powershell.exe -Command 'Set-Clipboard -Value ([System.Console]::In.ReadToEnd())'"
    alias pbpaste="powershell.exe -Command 'Get-Clipboard'"
  fi

  # --- Convenience aliases ---------------------------------------------------
  alias explorer="explorer.exe . 2>/dev/null &!"
  alias xdg-open="wslview"
  alias open="wslview"

  # --- Windows path shortcuts -------------------------------------------------
  WIN_HOME="$(wslpath "$(cmd.exe /c 'echo %USERPROFILE%' 2>/dev/null | tr -d '\r\n')" 2>/dev/null)"
  export WIN_HOME="${WIN_HOME:-/mnt/c/Users/$(whoami)}"
  alias winhome="cd \"$WIN_HOME\""
  alias downloads="cd \"$WIN_HOME/Downloads\""

  # --- VS Code integration from Windows host ----------------------------------
  CODE_BIN="/mnt/c/Program Files/Microsoft VS Code/bin"
  if [[ -d "$CODE_BIN" ]]; then
    # Prefer the official launcher and remove only duplicate Code-bin entries.
    path=("$CODE_BIN" "${(@)path:#$CODE_BIN}")
  fi

  # --- WSLg (Wayland) runtime directory ---------------------------------------
  # Only meaningful inside WSL with WSLg available, never on a bare Linux host.
  if [[ -d "/mnt/wslg/runtime-dir" ]]; then
    export XDG_RUNTIME_DIR="/mnt/wslg/runtime-dir"
    export WAYLAND_DISPLAY="wayland-0"
  fi

  # Some launchers start the shell without WSL_DISTRO_NAME; the kernel release
  # string is the reliable fallback.
  if [[ -z "${WSL_DISTRO_NAME:-}" && "$(uname -r)" == *microsoft* ]]; then
    export WSL_DISTRO_NAME="${WSL_DISTRO:-Debian}"
  fi
fi

export PATH="$HOME/go/bin:$PATH"
