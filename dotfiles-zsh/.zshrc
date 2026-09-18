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
    export PATH="$HOME/.local/bin:$HOME/.opencode/bin:$HOME/.cargo/bin:$HOME/.volta/bin:$HOME/.bun/bin:$HOME/.nix-profile/bin:/nix/var/nix/profiles/default/bin:/usr/local/bin:$PATH"
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

# fnm creates its per-shell multishell directory under XDG_RUNTIME_DIR, so that
# variable has to point at a directory that exists before fnm runs below. A shell
# started by a long-lived parent inherits the parent's value: a multiplexer
# server keeps whichever runtime directory it was started with, and /run/user/N
# disappears whenever the systemd user session restarts. fnm then fails on every
# new pane with "Can't create the multishell directory", so repair it here rather
# than two hundred lines later.
if [[ -n "${XDG_RUNTIME_DIR:-}" && ! -d "$XDG_RUNTIME_DIR" ]]; then
    unset XDG_RUNTIME_DIR
fi
if [[ -z "${XDG_RUNTIME_DIR:-}" ]]; then
    if [[ -d "/mnt/wslg/runtime-dir" ]]; then
        export XDG_RUNTIME_DIR="/mnt/wslg/runtime-dir"
    elif [[ -d "/run/user/$(id -u)" ]]; then
        export XDG_RUNTIME_DIR="/run/user/$(id -u)"
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

# WM_CMD array: zsh does not word-split an unquoted parameter, so a multi-word command must be launched as "${WM_CMD[@]}".
typeset -a WM_CMD
WM_CMD=(herdr)

function start_if_needed() {
    if [[ $- == *i* ]] && command -v "${WM_CMD[1]}" >/dev/null 2>&1 && [[ -z "${WM_VAR#/}" ]] && [[ -z "$TMUX" ]] && [[ -z "$ZELLIJ" ]] && [[ -z "$HERDR_ENV" ]] && [[ -t 1 ]]; then
        exec "${WM_CMD[@]}"
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

# carapace regenerates roughly 24 KB of completion registrations on every shell
# start, and that generation alone measures about 250 ms here, more than
# everything else in this file combined outside the completion system. The output
# depends only on the carapace binary and is byte-identical across runs, so
# generate it once and reuse it until the binary itself changes. Without this,
# every new pane pays the cost again.
if command -v carapace >/dev/null 2>&1; then
    CARAPACE_INIT="${XDG_CACHE_HOME:-$HOME/.cache}/carapace/init.zsh"
    if [[ ! -s "$CARAPACE_INIT" || "$commands[carapace]" -nt "$CARAPACE_INIT" ]]; then
        mkdir -p "${CARAPACE_INIT:h}"
        # Write beside the cache and move only a successful, non-empty result
        # into place, so an interrupted generation cannot poison the cache.
        if carapace _carapace >|"${CARAPACE_INIT}.tmp" 2>/dev/null && [[ -s "${CARAPACE_INIT}.tmp" ]]; then
            mv "${CARAPACE_INIT}.tmp" "$CARAPACE_INIT"
        else
            rm -f "${CARAPACE_INIT}.tmp"
        fi
    fi
    # Fall back to generating in place if the cache could not be produced.
    if [[ -s "$CARAPACE_INIT" ]]; then
        source "$CARAPACE_INIT"
    else
        source <(carapace _carapace)
    fi
fi

eval "$(fzf --zsh)"
eval "$(zoxide init zsh)"
eval "$(atuin init zsh)"

# --- direnv: per-directory env vars (works with mise) ---
eval "$(direnv hook zsh)"

# ─── Shell automation ────────────────────────────────────────────────────────
# Functions that remove repeated manual work. Each checks for the tools it needs
# and names the one that is missing, rather than failing with a bare error.

# extract <archive>: unpack almost any archive format.
extract() {
    if [[ $# -eq 0 ]]; then
        print -u2 'usage: extract <archive>'
        return 1
    fi
    local file=$1
    if [[ ! -f "$file" ]]; then
        print -u2 "extract: no such file: $file"
        return 1
    fi

    # Each format is paired with the tool it needs, so a missing dependency is
    # reported by name instead of surfacing as a bare "command not found".
    local need
    case $file in
        *.tar.gz|*.tgz)   need=tar;     set -- tar -xzf "$file" ;;
        *.tar.bz2|*.tbz2) need=tar;     set -- tar -xjf "$file" ;;
        *.tar.xz|*.txz)   need=tar;     set -- tar -xJf "$file" ;;
        *.tar.zst)        need=tar;     set -- tar --zstd -xf "$file" ;;
        *.tar)            need=tar;     set -- tar -xf "$file" ;;
        *.zip)            need=unzip;   set -- unzip -q "$file" ;;
        *.7z)             need=7z;      set -- 7z x "$file" ;;
        *.rar)            need=unrar;   set -- unrar x "$file" ;;
        *.gz)             need=gunzip;  set -- gunzip -kf "$file" ;;
        *.bz2)            need=bunzip2; set -- bunzip2 -kf "$file" ;;
        *.xz)             need=unxz;    set -- unxz -kf "$file" ;;
        *.zst)            need=zstd;    set -- zstd -df "$file" ;;
        *) print -u2 "extract: unsupported format: $file"; return 1 ;;
    esac

    if ! command -v "${need}" >/dev/null 2>&1; then
        print -u2 "extract: needs ${need}, which is not installed"
        return 1
    fi
    "$@"
}

# compress <archive> <path>...: build an archive, format taken from the target
# extension. The single-file formats refuse a directory rather than producing an
# archive that cannot be unpacked back.
compress() {
    if [[ $# -lt 2 ]]; then
        print -u2 'usage: compress <archive> <path> [path...]'
        print -u2 '       targets: .tar.gz .tar.bz2 .tar.xz .tar.zst .zip .7z .gz .bz2 .xz .zst'
        return 1
    fi
    local archive=$1
    shift

    if ! command -v tar >/dev/null 2>&1 && [[ $archive == *.tar.* || $archive == *.tgz ]]; then
        print -u2 'compress: needs tar, which is not installed'
        return 1
    fi

    local need
    case $archive in
        *.tar.gz|*.tgz) need=tar;  set -- tar -czf "$archive" "$@" ;;
        *.tar.bz2)      need=tar;  set -- tar -cjf "$archive" "$@" ;;
        *.tar.xz)       need=tar;  set -- tar -cJf "$archive" "$@" ;;
        *.tar.zst)      need=tar;  set -- tar --zstd -cf "$archive" "$@" ;;
        *.zip)          need=zip;  set -- zip -qr "$archive" "$@" ;;
        *.7z)           need=7z;   set -- 7z a -bso0 -bsp0 "$archive" "$@" ;;
        *.gz|*.bz2|*.xz|*.zst)
            if [[ $# -ne 1 ]]; then
                print -u2 "compress: $archive holds one file; use a .tar.* or .zip target for several"
                return 1
            fi
            local src=$1
            if [[ -d $src ]]; then
                print -u2 "compress: $archive holds one file; use a .tar.* or .zip target for a directory"
                return 1
            fi
            if [[ ! -f $src ]]; then
                print -u2 "compress: no such file: $src"
                return 1
            fi
            case $archive in
                *.gz)  need=gzip  ;;
                *.bz2) need=bzip2 ;;
                *.xz)  need=xz    ;;
                *.zst) need=zstd  ;;
            esac
            if ! command -v "${need}" >/dev/null 2>&1; then
                print -u2 "compress: needs ${need}, which is not installed"
                return 1
            fi
            # The redirection creates the target before the compressor runs, so a
            # failure would otherwise leave an empty archive behind and still
            # report success.
            case $archive in
                *.gz)  gzip  -c  "$src" >|"$archive" ;;
                *.bz2) bzip2 -c  "$src" >|"$archive" ;;
                *.xz)  xz    -c  "$src" >|"$archive" ;;
                *.zst) zstd  -qc "$src" >|"$archive" ;;
            esac || { rm -f "$archive"; print -u2 "compress: failed to create $archive"; return 1 }
            print "created $archive"
            return 0
            ;;
        *) print -u2 "compress: unsupported format: $archive"; return 1 ;;
    esac

    if ! command -v "${need}" >/dev/null 2>&1; then
        print -u2 "compress: needs ${need}, which is not installed"
        return 1
    fi
    if "$@"; then
        print "created $archive"
    else
        rm -f "$archive"
        print -u2 "compress: failed to create $archive"
        return 1
    fi
}

# Activate a project's virtual environment on cd, and leave it on the way out.
# Only an environment this hook activated is ever deactivated, so one the user
# activated by hand is never touched, and moving around inside a project does not
# re-activate or re-announce anything.
typeset -g _DOTFILES_VENV=
_dotfiles_activate_venv() {
    # Search the current directory and, when inside a repository, its ancestors up
    # to the repository root only. Walking all the way to / would activate a stray
    # venv sitting in /tmp or in $HOME for every directory underneath it, which is
    # how a scratch environment ends up capturing unrelated projects. The .git
    # test costs one stat and no subprocess, which matters on every cd.
    local -a search=("$PWD")
    local dir=$PWD candidate found=
    while [[ $dir != / && $dir != $HOME ]]; do
        dir=${dir:h}
        search+=("$dir")
        [[ -e "$dir/.git" ]] && break
    done
    [[ -e "${search[-1]}/.git" ]] || search=("$PWD")

    for dir in "$search[@]"; do
        for candidate in .venv venv .env; do
            if [[ -f "$dir/$candidate/bin/activate" ]]; then
                found="$dir/$candidate"
                break 2
            fi
        done
    done

    [[ $found == $_DOTFILES_VENV ]] && return 0

    if [[ -n $_DOTFILES_VENV ]]; then
        if [[ -n ${VIRTUAL_ENV:-} && $VIRTUAL_ENV == $_DOTFILES_VENV ]]; then
            deactivate 2>/dev/null
        fi
        _DOTFILES_VENV=
    fi

    if [[ -n $found ]]; then
        source "$found/bin/activate"
        _DOTFILES_VENV=$found
        print "venv: ${found:t}"
    fi
}
add-zsh-hook chpwd _dotfiles_activate_venv

# Announce a project's available tasks when entering it. Silent when the project
# declares none, and skippable with DOTFILES_NO_TASKS=1.
_dotfiles_show_tasks() {
    [[ -n ${DOTFILES_NO_TASKS:-} ]] && return 0
    [[ $PWD == $HOME ]] && return 0

    if [[ -f justfile || -f Justfile ]]; then
        command -v just >/dev/null 2>&1 && just --list 2>/dev/null | head -20
    elif [[ -f Makefile || -f makefile ]]; then
        # List declared targets without executing anything: the awk pass only
        # reads lines shaped like a target rule, so a Makefile that runs work at
        # parse time stays untouched.
        awk -F: '/^[a-zA-Z0-9_.-]+:([^=]|$)/ { print "  make " $1 }' Makefile 2>/dev/null | sort -u | head -20
    fi

    if [[ -f package.json ]] && command -v jq >/dev/null 2>&1; then
        jq -r '.scripts // {} | keys[]' package.json 2>/dev/null | head -20 | sed 's/^/  npm run /'
    fi

    if [[ -f Taskfile.yml || -f Taskfile.yaml ]] && command -v task >/dev/null 2>&1; then
        task --list 2>/dev/null | head -20
    fi
}
add-zsh-hook chpwd _dotfiles_show_tasks

# serve [port]: static file server for the current directory, defaulting to 8000.
serve() {
    local port=${1:-8000}
    if [[ ! $port == <-> ]]; then
        print -u2 "serve: port must be a whole number, got '$port'"
        return 1
    fi
    if (( port < 1 || port > 65535 )); then
        print -u2 "serve: port must be between 1 and 65535, got $port"
        return 1
    fi
    if ! command -v python3 >/dev/null 2>&1; then
        print -u2 'serve: needs python3, which is not installed'
        return 1
    fi
    if command -v ss >/dev/null 2>&1 && ss -ltn 2>/dev/null | grep -q ":$port "; then
        print -u2 "serve: port $port is already in use"
        return 1
    fi
    print "serving $PWD on http://localhost:$port  (Ctrl-C to stop)"
    python3 -m http.server "$port" --bind 127.0.0.1
}

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

  # --- WSLg (Wayland) display -------------------------------------------------
  # Only meaningful inside WSL with WSLg available, never on a bare Linux host.
  # XDG_RUNTIME_DIR is settled near the top of this file, before fnm needs it;
  # here only the display is set.
  if [[ -d "/mnt/wslg/runtime-dir" ]]; then
    export WAYLAND_DISPLAY="wayland-0"
  fi

  # Some launchers start the shell without WSL_DISTRO_NAME; the kernel release
  # string is the reliable fallback.
  if [[ -z "${WSL_DISTRO_NAME:-}" && "$(uname -r)" == *microsoft* ]]; then
    export WSL_DISTRO_NAME="${WSL_DISTRO:-Debian}"
  fi
fi

export PATH="$HOME/go/bin:$PATH"
