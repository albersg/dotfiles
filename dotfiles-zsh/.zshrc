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

# ─── Palette ─────────────────────────────────────────────────────────────────
# One palette, defined once. The terminal emulators set the same values
# (dotfiles-ghostty, dotfiles-kitty, alacritty.toml), so everything painted
# inside them resolves to the same colours instead of each tool falling back to
# its own defaults.
#
#   base     #06080f   background
#   surface  #263356   selection, de-emphasised punctuation
#   text     #f3f6f9   foreground
#   muted    #8a8fa3   comments, hints, autosuggestions
#   red      #cb7c94   errors, archives, orphan links
#   green    #b7cc85   success, commands, executables
#   yellow   #ffe066   warnings, strings, documents
#   blue     #7fb4ca   accent: directories, headers
#   magenta  #ff8dd7   constants, images, devices
#   cyan     #7aa89f   operators, symlinks, media
#
# Every entry is declared twice because the consumers disagree on the format:
# the prompt and the line editor take hex, while LS_COLORS and EZA_COLORS take
# an SGR sequence. Zsh expands both at file-read time, so the indirection costs
# nothing at startup, and having one list is what stops the two forms drifting.
#
# The 24-bit form is not a style preference. The index form this file used
# before (38;5;67, 38;5;132, 38;5;144) addresses entries 16-255 of the
# terminal's colour cube, which a custom theme never redefines, so those values
# rendered as unrelated hues on any machine using this configuration.
typeset -g PALETTE_BASE="#06080f"       PALETTE_BASE_SGR="38;2;6;8;15"
typeset -g PALETTE_SURFACE="#263356"    PALETTE_SURFACE_SGR="38;2;38;51;86"
typeset -g PALETTE_TEXT="#f3f6f9"       PALETTE_TEXT_SGR="38;2;243;246;249"
typeset -g PALETTE_MUTED="#8a8fa3"      PALETTE_MUTED_SGR="38;2;138;143;163"
typeset -g PALETTE_RED="#cb7c94"        PALETTE_RED_SGR="38;2;203;124;148"
typeset -g PALETTE_GREEN="#b7cc85"      PALETTE_GREEN_SGR="38;2;183;204;133"
typeset -g PALETTE_YELLOW="#ffe066"     PALETTE_YELLOW_SGR="38;2;255;224;102"
typeset -g PALETTE_BLUE="#7fb4ca"       PALETTE_BLUE_SGR="38;2;127;180;202"
typeset -g PALETTE_MAGENTA="#ff8dd7"    PALETTE_MAGENTA_SGR="38;2;255;141;215"
typeset -g PALETTE_CYAN="#7aa89f"       PALETTE_CYAN_SGR="38;2;122;168;159"
# The only value that needs the background form rather than the foreground one.
typeset -g PALETTE_SURFACE_BG_SGR="48;2;38;51;86"
# A bare escape, so the strings that need a literal sequence can be built from
# the palette instead of repeating its digits.
typeset -g PALETTE_ESC=$'\e'

# --- File listings: GNU ls reads LS_COLORS, eza reads it as its base layer ----
export LS_COLORS="rs=0:\
di=${PALETTE_BLUE_SGR}:\
ln=${PALETTE_CYAN_SGR}:\
mh=${PALETTE_MUTED_SGR}:\
pi=${PALETTE_YELLOW_SGR}:so=${PALETTE_YELLOW_SGR}:do=${PALETTE_YELLOW_SGR}:\
bd=${PALETTE_MAGENTA_SGR}:cd=${PALETTE_MAGENTA_SGR}:\
or=${PALETTE_RED_SGR};1:mi=${PALETTE_RED_SGR};1:ca=${PALETTE_RED_SGR}:\
su=${PALETTE_RED_SGR};${PALETTE_SURFACE_BG_SGR}:sg=${PALETTE_RED_SGR};${PALETTE_SURFACE_BG_SGR}:\
tw=${PALETTE_GREEN_SGR};${PALETTE_SURFACE_BG_SGR}:ow=${PALETTE_GREEN_SGR};${PALETTE_SURFACE_BG_SGR}:\
st=${PALETTE_BLUE_SGR};${PALETTE_SURFACE_BG_SGR}:\
ex=${PALETTE_GREEN_SGR}:\
*.tar=${PALETTE_RED_SGR}:*.tgz=${PALETTE_RED_SGR}:*.tbz2=${PALETTE_RED_SGR}:*.txz=${PALETTE_RED_SGR}:*.zst=${PALETTE_RED_SGR}:\
*.zip=${PALETTE_RED_SGR}:*.7z=${PALETTE_RED_SGR}:*.rar=${PALETTE_RED_SGR}:\
*.gz=${PALETTE_RED_SGR}:*.bz2=${PALETTE_RED_SGR}:*.xz=${PALETTE_RED_SGR}:\
*.png=${PALETTE_MAGENTA_SGR}:*.jpg=${PALETTE_MAGENTA_SGR}:*.jpeg=${PALETTE_MAGENTA_SGR}:*.gif=${PALETTE_MAGENTA_SGR}:*.webp=${PALETTE_MAGENTA_SGR}:*.svg=${PALETTE_MAGENTA_SGR}:*.ico=${PALETTE_MAGENTA_SGR}:\
*.mp4=${PALETTE_MAGENTA_SGR}:*.mkv=${PALETTE_MAGENTA_SGR}:*.mov=${PALETTE_MAGENTA_SGR}:*.webm=${PALETTE_MAGENTA_SGR}:\
*.mp3=${PALETTE_CYAN_SGR}:*.flac=${PALETTE_CYAN_SGR}:*.wav=${PALETTE_CYAN_SGR}:*.ogg=${PALETTE_CYAN_SGR}:*.m4a=${PALETTE_CYAN_SGR}:\
*.pdf=${PALETTE_YELLOW_SGR}:*.md=${PALETTE_YELLOW_SGR}:*.txt=${PALETTE_YELLOW_SGR}:*.rst=${PALETTE_YELLOW_SGR}:\
*.sh=${PALETTE_GREEN_SGR}:*.bash=${PALETTE_GREEN_SGR}:*.zsh=${PALETTE_GREEN_SGR}:*.fish=${PALETTE_GREEN_SGR}:\
*.py=${PALETTE_GREEN_SGR}:*.go=${PALETTE_GREEN_SGR}:*.rs=${PALETTE_GREEN_SGR}:*.js=${PALETTE_GREEN_SGR}:*.ts=${PALETTE_GREEN_SGR}:\
*.json=${PALETTE_GREEN_SGR}:*.yaml=${PALETTE_GREEN_SGR}:*.yml=${PALETTE_GREEN_SGR}:*.toml=${PALETTE_GREEN_SGR}:\
*.db=${PALETTE_BLUE_SGR}:*.sqlite=${PALETTE_BLUE_SGR}:*.sql=${PALETTE_BLUE_SGR}:\
*.log=${PALETTE_MUTED_SGR}:*.lock=${PALETTE_MUTED_SGR}"

# --- eza metadata ------------------------------------------------------------
# eza paints permissions, owner, size and date from its own defaults (bold
# yellow, red, green, blue) which fight with the file names and with every other
# tool in the terminal. Metadata is de-emphasised here and colour is left to
# carry meaning: the names, the git state, and the security bits in the
# permission column.
export EZA_COLORS="\
oc=${PALETTE_MUTED_SGR}:\
ur=${PALETTE_MUTED_SGR}:uw=${PALETTE_MUTED_SGR}:ux=${PALETTE_MUTED_SGR}:ue=${PALETTE_MUTED_SGR}:\
gr=${PALETTE_MUTED_SGR}:gw=${PALETTE_MUTED_SGR}:gx=${PALETTE_MUTED_SGR}:\
tr=${PALETTE_MUTED_SGR}:tw=${PALETTE_MUTED_SGR}:tx=${PALETTE_MUTED_SGR}:\
su=${PALETTE_YELLOW_SGR};1:sf=${PALETTE_YELLOW_SGR};1:xa=${PALETTE_MAGENTA_SGR}:\
sn=${PALETTE_CYAN_SGR}:nb=${PALETTE_CYAN_SGR}:nk=${PALETTE_CYAN_SGR}:nm=${PALETTE_CYAN_SGR}:ng=${PALETTE_CYAN_SGR}:nt=${PALETTE_CYAN_SGR}:\
sb=${PALETTE_MUTED_SGR}:ub=${PALETTE_MUTED_SGR}:uk=${PALETTE_MUTED_SGR}:um=${PALETTE_MUTED_SGR}:ug=${PALETTE_MUTED_SGR}:ut=${PALETTE_MUTED_SGR}:\
df=${PALETTE_MUTED_SGR}:ds=${PALETTE_MUTED_SGR}:lc=${PALETTE_MUTED_SGR}:lm=${PALETTE_MUTED_SGR}:\
uu=${PALETTE_BLUE_SGR}:un=${PALETTE_MUTED_SGR}:uR=${PALETTE_RED_SGR}:\
gu=${PALETTE_BLUE_SGR}:gn=${PALETTE_MUTED_SGR}:gR=${PALETTE_RED_SGR}:\
xx=${PALETTE_SURFACE_SGR}:\
da=${PALETTE_MUTED_SGR}:in=${PALETTE_MUTED_SGR}:bl=${PALETTE_MUTED_SGR}:\
hd=${PALETTE_BLUE_SGR};1:lp=${PALETTE_CYAN_SGR}:cc=${PALETTE_RED_SGR}:bO=${PALETTE_RED_SGR};4:\
sp=${PALETTE_MAGENTA_SGR}:mp=${PALETTE_MAGENTA_SGR}:\
im=${PALETTE_MAGENTA_SGR}:vi=${PALETTE_MAGENTA_SGR}:mu=${PALETTE_CYAN_SGR}:lo=${PALETTE_CYAN_SGR}:\
cr=${PALETTE_YELLOW_SGR}:do=${PALETTE_YELLOW_SGR}:co=${PALETTE_RED_SGR}:tm=${PALETTE_MUTED_SGR}:cm=${PALETTE_MUTED_SGR}:\
ga=${PALETTE_GREEN_SGR}:gm=${PALETTE_YELLOW_SGR}:gd=${PALETTE_RED_SGR}:gv=${PALETTE_MAGENTA_SGR}:\
gt=${PALETTE_YELLOW_SGR}:gi=${PALETTE_MUTED_SGR}:gc=${PALETTE_RED_SGR};1:\
Gm=${PALETTE_BLUE_SGR};1:Go=${PALETTE_CYAN_SGR}:Gc=${PALETTE_GREEN_SGR}:Gd=${PALETTE_YELLOW_SGR}"

# Icons need a space to breathe next to the name.
export EZA_ICON_SPACING=2

# --- bat --------------------------------------------------------------------
# The theme is generated from the palette above and shipped with these dotfiles,
# because none of the themes bat ships can match a custom palette: each was drawn
# for its own background and its own accent colours, so any of them puts colours
# on screen that the terminal never defines.
#
# A theme has to be built into bat's cache before it can be selected, and it is the
# installer that copies the file and runs `bat cache --build`. On a machine where
# that has not happened yet, naming it would make every bat invocation print
# "Unknown theme" and fall back anyway, so the closest shipped theme is used until
# the file is present.
if [[ -f "${XDG_CONFIG_HOME:-$HOME/.config}/bat/themes/dotfiles.tmTheme" ]]; then
    export BAT_THEME="dotfiles"
else
    export BAT_THEME="Catppuccin Mocha"
fi

# --- zsh-autosuggestions ------------------------------------------------------
# The default highlight is `fg=8`, the terminal's bright black, which on this
# palette is very close to invisible. Muted is the palette's own secondary text.
# The buffer limit is a latency guard rather than a look: past a couple of dozen
# characters the suggestion cannot keep up with typing, so it is not computed.
ZSH_AUTOSUGGEST_HIGHLIGHT_STYLE="fg=${PALETTE_MUTED}"
ZSH_AUTOSUGGEST_STRATEGY=(history completion)
ZSH_AUTOSUGGEST_BUFFER_MAX_SIZE=20

# --- zsh-syntax-highlighting --------------------------------------------------
# The plugin fills in a default only for keys that are still unset, so setting
# them before it loads further down this file is what makes them stick. The
# associative array has to be declared first, or the first assignment would turn
# it into an ordinary indexed array and the keys would stop matching.
typeset -gA ZSH_HIGHLIGHT_STYLES
ZSH_HIGHLIGHT_STYLES[default]="fg=${PALETTE_TEXT}"
ZSH_HIGHLIGHT_STYLES[unknown-token]="fg=${PALETTE_RED},bold"
ZSH_HIGHLIGHT_STYLES[reserved-word]="fg=${PALETTE_MAGENTA}"
ZSH_HIGHLIGHT_STYLES[builtin]="fg=${PALETTE_BLUE}"
ZSH_HIGHLIGHT_STYLES[function]="fg=${PALETTE_BLUE}"
ZSH_HIGHLIGHT_STYLES[alias]="fg=${PALETTE_BLUE}"
ZSH_HIGHLIGHT_STYLES[suffix-alias]="fg=${PALETTE_BLUE},underline"
ZSH_HIGHLIGHT_STYLES[global-alias]="fg=${PALETTE_BLUE}"
ZSH_HIGHLIGHT_STYLES[command]="fg=${PALETTE_GREEN}"
ZSH_HIGHLIGHT_STYLES[precommand]="fg=${PALETTE_GREEN},underline"
ZSH_HIGHLIGHT_STYLES[autodirectory]="fg=${PALETTE_BLUE},underline"
ZSH_HIGHLIGHT_STYLES[hashed-command]="fg=${PALETTE_CYAN}"
ZSH_HIGHLIGHT_STYLES[arg0]="fg=${PALETTE_TEXT}"
ZSH_HIGHLIGHT_STYLES[path]="fg=${PALETTE_TEXT}"
ZSH_HIGHLIGHT_STYLES[path_pathseparator]="fg=${PALETTE_CYAN}"
ZSH_HIGHLIGHT_STYLES[path_prefix]="fg=${PALETTE_TEXT}"
ZSH_HIGHLIGHT_STYLES[path_prefix_pathseparator]="fg=${PALETTE_CYAN}"
ZSH_HIGHLIGHT_STYLES[globbing]="fg=${PALETTE_MAGENTA}"
ZSH_HIGHLIGHT_STYLES[history-expansion]="fg=${PALETTE_MAGENTA}"
ZSH_HIGHLIGHT_STYLES[comment]="fg=${PALETTE_MUTED}"
ZSH_HIGHLIGHT_STYLES[assign]="fg=${PALETTE_TEXT}"
ZSH_HIGHLIGHT_STYLES[redirection]="fg=${PALETTE_CYAN}"
ZSH_HIGHLIGHT_STYLES[named-fd]="fg=${PALETTE_CYAN}"
ZSH_HIGHLIGHT_STYLES[numeric-fd]="fg=${PALETTE_CYAN}"
ZSH_HIGHLIGHT_STYLES[single-quoted-argument]="fg=${PALETTE_YELLOW}"
ZSH_HIGHLIGHT_STYLES[single-quoted-argument-unclosed]="fg=${PALETTE_RED},bold"
ZSH_HIGHLIGHT_STYLES[double-quoted-argument]="fg=${PALETTE_YELLOW}"
ZSH_HIGHLIGHT_STYLES[double-quoted-argument-unclosed]="fg=${PALETTE_RED},bold"
ZSH_HIGHLIGHT_STYLES[dollar-quoted-argument]="fg=${PALETTE_YELLOW}"
ZSH_HIGHLIGHT_STYLES[dollar-quoted-argument-unclosed]="fg=${PALETTE_RED},bold"
ZSH_HIGHLIGHT_STYLES[rc-quote]="fg=${PALETTE_MAGENTA}"
ZSH_HIGHLIGHT_STYLES[dollar-double-quoted-argument]="fg=${PALETTE_CYAN}"
ZSH_HIGHLIGHT_STYLES[back-double-quoted-argument]="fg=${PALETTE_CYAN}"
ZSH_HIGHLIGHT_STYLES[back-dollar-quoted-argument]="fg=${PALETTE_CYAN}"
ZSH_HIGHLIGHT_STYLES[back-quoted-argument]="fg=${PALETTE_YELLOW}"
ZSH_HIGHLIGHT_STYLES[back-quoted-argument-unclosed]="fg=${PALETTE_RED},bold"
ZSH_HIGHLIGHT_STYLES[command-substitution]="fg=${PALETTE_YELLOW}"
ZSH_HIGHLIGHT_STYLES[command-substitution-unquoted]="fg=${PALETTE_YELLOW}"
ZSH_HIGHLIGHT_STYLES[process-substitution]="fg=${PALETTE_YELLOW}"
ZSH_HIGHLIGHT_STYLES[arithmetic-expansion]="fg=${PALETTE_YELLOW}"
ZSH_HIGHLIGHT_STYLES[single-square-bracket]="fg=${PALETTE_MAGENTA}"
ZSH_HIGHLIGHT_STYLES[bracket-level-1]="fg=${PALETTE_BLUE}"
ZSH_HIGHLIGHT_STYLES[bracket-level-2]="fg=${PALETTE_GREEN}"
ZSH_HIGHLIGHT_STYLES[bracket-level-3]="fg=${PALETTE_YELLOW}"
ZSH_HIGHLIGHT_STYLES[bracket-error]="fg=${PALETTE_RED},bold"
ZSH_HIGHLIGHT_STYLES[cursor-matchingbracket]=standout
ZSH_HIGHLIGHT_STYLES[numeric-constant]="fg=${PALETTE_MAGENTA}"
ZSH_HIGHLIGHT_STYLES[else]="fg=${PALETTE_RED}"

# `brackets` marks the pair around the cursor and `cursor` highlights the
# character under it; both are cheap and neither repaints the whole line.
typeset -ga ZSH_HIGHLIGHT_HIGHLIGHTERS
ZSH_HIGHLIGHT_HIGHLIGHTERS=(main brackets cursor)
# A guard for very long lines: past this length the highlighter gives up, which
# is what keeps a large paste from stalling the line editor.
ZSH_HIGHLIGHT_MAXLENGTH=500

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

    # fnm prepends a fresh multishell directory for every shell it runs in, and
    # one directory per level of nesting survives in PATH, because `typeset -U
    # path` collapses exact duplicates and these are not duplicates: each carries
    # its own pid. Two had accumulated by the time this was written, and a
    # long-lived multiplexer server that spawns shells would keep adding them.
    # Drop every other shell's directory and put only this one back, which is the
    # position fnm just gave it.
    if [[ -n $FNM_MULTISHELL_PATH ]]; then
        path=("${(@)path:#*/fnm_multishells/*}")
        path=("$FNM_MULTISHELL_PATH/bin" $path)
    fi

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
# Extra completion definitions on fpath, before the completion system is
# initialised below. compinit only sees the directories that are on fpath at the
# moment it runs, so this cannot move further down the file.
if [[ -n "$BREW_BIN" && -d "$(dirname "$BREW_BIN")/share/zsh-completions" ]]; then
    typeset -U fpath
    fpath=("$(dirname "$BREW_BIN")/share/zsh-completions" $fpath)
fi

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
# fzf draws its own UI and defaults to sixteen fixed colours that follow no
# theme, which made the picker look like a different application sitting inside
# the terminal. These are the palette values above.
export FZF_DEFAULT_OPTS="--color=fg:${PALETTE_TEXT},bg:${PALETTE_BASE},hl:${PALETTE_YELLOW} \
--color=fg+:${PALETTE_TEXT},bg+:${PALETTE_SURFACE},hl+:${PALETTE_YELLOW} \
--color=info:${PALETTE_BLUE},prompt:${PALETTE_CYAN},pointer:${PALETTE_MAGENTA} \
--color=marker:${PALETTE_GREEN},spinner:${PALETTE_YELLOW},header:${PALETTE_MUTED} \
--color=border:${PALETTE_MUTED},label:${PALETTE_BLUE},query:${PALETTE_TEXT} \
--border=rounded --layout=reverse --info=inline-right"
# Ctrl+T and Alt+C otherwise open an empty list. The preview is the reason to
# reach for a fuzzy finder instead of typing the path.
export FZF_CTRL_T_OPTS="--preview 'bat --style=numbers --color=always --line-range=:300 {}' --preview-window 'right:55%:wrap'"
# The variable name matters: fzf reads FZF_ALT_C_COMMAND. The previous
# FZF_ALT_COMMAND was never read, so Alt+C fell back to FZF_DEFAULT_COMMAND and
# listed files where only directories were expected.
export FZF_ALT_C_COMMAND="fd --type=d --hidden --strip-cwd-prefix --exclude .git"
export FZF_ALT_C_OPTS="--preview 'eza --tree --icons --group-directories-first --level=2 --color=always {}' --preview-window 'right:55%'"
# fzf also drives the `**<Tab>` completion trigger, which renders with the same
# hard-coded defaults otherwise.
export FZF_COMPLETION_OPTS="--border=rounded --layout=reverse"

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
alias fzfbat='fzf --preview="bat --color=always {}"'
alias fzfnvim='nvim $(fzf --preview="bat --color=always {}")'

# --- Modern Unix replacements (cat→bat, ls→eza) ---
# `cat` keeps the syntax highlighting for plain invocations and hands everything
# else to the real binary. bat is not a drop-in replacement: it has no -v or -T,
# and `cat -v` is exactly the kind of invocation that turns up in a copied
# recipe, where it used to fail with "unexpected argument". Testing the
# arguments, rather than the tool, keeps the highlight where it helps and keeps
# the flags working where they belong. A function rather than an alias, so it is
# never inherited by a script.
function cat() {
    local arg
    for arg in "$@"; do
        if [[ $arg == -* ]]; then
            command cat "$@"
            return
        fi
    done
    bat --paging=never --style=plain -- "$@"
}

# `grep` is deliberately NOT aliased to ripgrep. They are not interchangeable: in
# ripgrep -r means replace, so `grep -r pattern .` in the interactive shell became
# a replace command, and -E, --include and -A/-B do not mean the same thing on
# both. An alias is not exported, so no script was ever affected; the damage was
# that a command copied from the terminal into a script ran somewhere else with
# different semantics. `rg` is short enough to type on purpose.
alias ls='eza --icons --group-directories-first'
alias ll='eza -l --icons --git --group-directories-first --time-style=long-iso --header'
alias la='eza -la --icons --git --group-directories-first --time-style=long-iso --header'
alias lt='eza --tree --icons --group-directories-first --level=2'
alias tree='eza --tree --icons --group-directories-first --level=3'

# --- Network tools ---
# `trip` (trippy) is only aliased when it is actually resolvable, so the alias
# never breaks shells on hosts where it is not installed.
if command -v trip >/dev/null 2>&1; then
    alias traceroute='sudo trip'
    alias tracert='sudo trip'
fi
alias http='xh'              # xh > curl for APIs

# bat's theme is set with the palette near the top of this file.

export CARAPACE_BRIDGES='zsh,fish,bash,inshellisense'
# Completion listings. The group heading uses the palette instead of a hard-coded
# ANSI grey, and the entries reuse the same LS_COLORS the listings print with, so
# the menu does not switch to a second colour scheme.
zstyle ':completion:*:descriptions' format "${PALETTE_ESC}[1;${PALETTE_BLUE_SGR}m%d${PALETTE_ESC}[0m"
zstyle ':completion:*' format "${PALETTE_ESC}[${PALETTE_MUTED_SGR}mCompleting %d${PALETTE_ESC}[0m"
zstyle ':completion:*' group-name ''
zstyle ':completion:*:default' list-colors ${(s.:.)LS_COLORS}

# fzf-tab hands the completion list to fzf. The completion menu has to be turned
# off, or zsh draws its own menu in the same keystroke that opens fzf's.
zstyle ':completion:*' menu no
zstyle ':fzf-tab:*' fzf-flags --height=60%
zstyle ':fzf-tab:complete:cd:*' fzf-preview 'eza --tree --icons --group-directories-first --level=2 --color=always $realpath'

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

# fzf-tab replaces the completion menu with fzf, and it is sourced here on
# purpose: fzf's own shell integration, one line above, binds Tab to
# fzf-completion, so loading fzf-tab earlier would leave it overwritten and
# installed but never used. fzf keeps the bindings it owns, Ctrl+T and Alt+C; this
# only takes Tab. Homebrew installs the plugin under `opt` rather than `share`,
# and there is no distribution package to fall back to.
if [[ -n "$BREW_SHARE" && -f "${BREW_SHARE:h}/opt/fzf-tab/share/fzf-tab/fzf-tab.zsh" ]]; then
    source "${BREW_SHARE:h}/opt/fzf-tab/share/fzf-tab/fzf-tab.zsh"
fi
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

# ─── Secrets ─────────────────────────────────────────────────────────────────
# A file encrypted with SOPS keeps its keys readable and only its values
# encrypted, so it can live in a repository and still show which keys exist and
# which one changed. The key is a single age file rather than a GPG keyring,
# which avoids the agent and pinentry dance that is fragile under WSL.
#
# Create the key once:
#
#   mkdir -p ~/.config/sops/age && age-keygen -o ~/.config/sops/age/keys.txt
#
# Then put the public key it prints into a .sops.yaml in each project:
#
#   creation_rules:
#     - path_regex: .*\.env$
#       age: <public key>
#
# An age key has no recovery path. Lose that file and every encrypted value is
# gone for good, so back it up somewhere that is not this machine.
typeset -g DOTFILES_SOPS_FILE=secrets.env
export SOPS_AGE_KEY_FILE="${SOPS_AGE_KEY_FILE:-$HOME/.config/sops/age/keys.txt}"

# Every secrets helper starts here, so a missing tool or key is reported once,
# with the command that fixes it, instead of failing somewhere further down.
_dotfiles_secrets_ready() {
    if ! command -v sops >/dev/null 2>&1; then
        print -u2 'secrets: sops is not installed'
        return 1
    fi
    if [[ ! -f $SOPS_AGE_KEY_FILE ]]; then
        print -u2 "secrets: no age key at $SOPS_AGE_KEY_FILE"
        print -u2 "  create one with: mkdir -p ${SOPS_AGE_KEY_FILE:h} && age-keygen -o $SOPS_AGE_KEY_FILE"
        return 1
    fi
    return 0
}

# env-edit [file]: edit an encrypted file. SOPS decrypts into the editor and
# re-encrypts on save, so the plaintext never lands on disk.
env-edit() {
    _dotfiles_secrets_ready || return 1
    local file=${1:-$DOTFILES_SOPS_FILE}
    if [[ ! -f $file ]]; then
        print -u2 "secrets: no such file: $file"
        return 1
    fi
    sops "$file"
}

# env-load [file] [.env]: decrypt into a local file for tools that insist on
# reading one. It refuses unless the destination is already ignored by git,
# because writing plaintext secrets into a repository is the mistake this whole
# arrangement exists to prevent.
env-load() {
    _dotfiles_secrets_ready || return 1
    local src=${1:-$DOTFILES_SOPS_FILE} dst=${2:-.env}
    if [[ ! -f $src ]]; then
        print -u2 "secrets: no such file: $src"
        return 1
    fi
    # Refuse unless git already ignores the destination. Writing a plaintext
    # secret into a repository is the mistake this arrangement exists to avoid,
    # and creating a fresh .env that git would happily track is the same mistake
    # with a different name, so the check runs whether or not the file exists.
    # Outside a work tree there is nothing to leak into, so it is allowed.
    if git rev-parse --is-inside-work-tree >/dev/null 2>&1 && ! git check-ignore -q -- "$dst" 2>/dev/null; then
        print -u2 "secrets: git does not ignore $dst, so the plaintext would be tracked"
        print -u2 "  add it to .gitignore, or to a global exclude, and try again"
        return 1
    fi
    if [[ -e $dst ]] && [[ ! -w $dst ]]; then
        print -u2 "secrets: cannot write $dst"
        return 1
    fi
    if ! sops -d "$src" >|"$dst"; then
        rm -f "$dst"
        print -u2 "secrets: could not decrypt $src"
        return 1
    fi
    print "secrets: wrote $dst"
}

# env-export [file]: print export lines for the current shell, for an .envrc:
#
#   eval "$(env-export secrets.env)"
#
# Nothing is written to disk this way, which is the option to prefer.
env-export() {
    _dotfiles_secrets_ready || return 1
    local src=${1:-$DOTFILES_SOPS_FILE}
    if [[ ! -f $src ]]; then
        print -u2 "secrets: no such file: $src"
        return 1
    fi
    sops -d --output-type dotenv "$src" 2>/dev/null | sed 's/^/export /'
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
