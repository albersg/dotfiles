# dotfiles-managed-config: fish

if status is-interactive
    # Commands to run in interactive sessions can go here
    # Install Fisher if not installed (git.io was shut down by GitHub in 2023)
    if not functions -q fisher
        curl -sL https://raw.githubusercontent.com/jorgebucaran/fisher/main/functions/fisher.fish | source
        and fisher install jorgebucaran/fisher
    end

end

# Detect Termux
set -l IS_TERMUX 0
if test -n "$TERMUX_VERSION"; or test -d /data/data/com.termux
    set IS_TERMUX 1
end

if test $IS_TERMUX -eq 1
    # Termux - use PREFIX for binaries
    set -x PATH $PREFIX/bin $HOME/.local/bin $HOME/.cargo/bin $PATH
else if test (uname) = Darwin
    # macOS - check for Apple Silicon vs Intel
    if test -f /opt/homebrew/bin/brew
        # Apple Silicon (M1/M2/M3)
        set BREW_BIN /opt/homebrew/bin/brew
    else if test -f /usr/local/bin/brew
        # Intel Mac
        set BREW_BIN /usr/local/bin/brew
    end
    set -x PATH $HOME/.local/bin $HOME/.opencode/bin $HOME/.volta/bin $HOME/.bun/bin $HOME/.nix-profile/bin /nix/var/nix/profiles/default/bin /usr/local/bin $HOME/.cargo/bin $PATH
else
    # Linux
    set BREW_BIN /home/linuxbrew/.linuxbrew/bin/brew
    set -x PATH $HOME/.local/bin $HOME/.opencode/bin $HOME/.volta/bin $HOME/.bun/bin $HOME/.nix-profile/bin /nix/var/nix/profiles/default/bin /usr/local/bin $HOME/.cargo/bin $PATH
end

# Only eval brew shellenv if brew is installed (not on Termux)
if test $IS_TERMUX -eq 0; and set -q BREW_BIN; and test -f $BREW_BIN
    eval ($BREW_BIN shellenv)
end

# Start selected terminal multiplexer
# A unique session per terminal window: -A would attach to an existing session
# instead, mirroring the same panes across every terminal emulator.
if status is-interactive; and command -q tmux; and not set -q TMUX; and not set -q ZELLIJ; and not set -q HERDR_ENV
    tmux new-session -s "term-$(date +%s)-$(random)"
end

# Initialize tools
starship init fish | source
zoxide init fish | source
atuin init fish | source
fzf --fish | source

# Carapace completions
set -Ux CARAPACE_BRIDGES 'zsh,fish,bash,inshellisense'

if not test -d ~/.config/fish/completions
    mkdir -p ~/.config/fish/completions
end

if not test -f ~/.config/fish/completions/.initialized
    if not test -d ~/.config/fish/completions
        mkdir -p ~/.config/fish/completions
    end
    carapace --list | awk '{print $1}' | xargs -I{} touch ~/.config/fish/completions/{}.fish
    touch ~/.config/fish/completions/.initialized
end

carapace _carapace | source

set -g fish_greeting ""

# Enable vi mode
fish_vi_key_bindings

# Set nvim as default editor for opencode and other tools
set -gx EDITOR nvim
set -gx VISUAL nvim

# Use bat as the man page pager
set -gx MANPAGER "sh -c 'col -bx | bat -l man -p'"

## alias
# --- File listings: eza, the tool and the flags zsh's aliases already use ---
# The two shells are meant to list files the same way, and zsh has aliased `ls` to
# `eza --icons --group-directories-first` (with ll/la/lt/tree beside it) for
# longer than this block has existed. fish was left on `gls`, so its listing
# printed no icons at all — which cannot be told apart from the Nerd Font being
# installed wrong, and was reported as that.
#
# The aliases are declared only when eza resolves, because an alias whose tool the
# host has not got fails on every call. eza arrives from the Brewfile, from the
# Arch and Fedora package lists, or from the Homebrew this installer puts in place
# for Debian hosts, so a run whose Homebrew step did not complete has none of it.
if command -q eza
    alias ls='eza --icons --group-directories-first'
    alias ll='eza -l --icons --git --group-directories-first --time-style=long-iso --header'
    alias la='eza -la --icons --git --group-directories-first --time-style=long-iso --header'
    alias lt='eza --tree --icons --group-directories-first --level=2'
    alias tree='eza --tree --icons --group-directories-first --level=3'
else if test (uname) = Darwin
    # No eza: keep the colour `ls` this configuration already gave macOS, rather
    # than trading one broken `ls` for another.
    alias ls='ls --color=auto'
else if command -q gls
    # `gls` is Homebrew's name for GNU coreutils' ls, so it is asked for rather
    # than assumed: a Debian host without Homebrew has only the system `ls`, and
    # an alias to a binary that does not exist fails on every call.
    alias ls='gls --color=auto'
end

# Icons need a space to breathe next to the name, exactly as they have in .zshrc.
set -gx EZA_ICON_SPACING 2

alias fzfbat='fzf --preview="bat --theme=gruvbox-dark --color=always {}"'
alias fzfnvim='nvim (fzf --preview="bat --theme=gruvbox-dark --color=always {}")'

# dotfiles-managed-config: fish
# >>> dotfiles-theme: dotfiles (generated from themes/dotfiles.toml; edit the definition, not this block) >>>
# ┌──────────────────────────────────────────────────────────────────────────────┐
# │                                DOTFILES THEME                                │
# └──────────────────────────────────────────────────────────────────────────────┘
# The fish palette. Generated from themes/dotfiles.toml; edit the definition, not this
# block. These are global variables, so they also win over a universal colour the
# user chose once through fish_config; the switch never writes the user's own
# state in fish_variables.
set -g fish_color_normal f3f6f9
set -g fish_color_command b7cc85
set -g fish_color_keyword ff8dd7
set -g fish_color_quote ffe066
set -g fish_color_redirection f3f6f9
set -g fish_color_end 7aa89f
set -g fish_color_error cb7c94
set -g fish_color_param 7fb4ca
set -g fish_color_comment 8a8fa3
set -g fish_color_selection --background=263356
set -g fish_color_search_match --background=263356
set -g fish_color_operator b7cc85
set -g fish_color_escape ff8dd7
set -g fish_color_autosuggestion 8a8fa3

# Completion pager colours.
set -g fish_pager_color_progress 8a8fa3
set -g fish_pager_color_prefix b7cc85
set -g fish_pager_color_completion f3f6f9
set -g fish_pager_color_description 8a8fa3
# <<< dotfiles-theme <<<

# Files in this directory belong to the user, not to this managed config: the
# installer replaces config.fish on updates, so local edits written here are
# lost. The drop-ins survive because installation leaves this directory alone.
# Source them last so personal settings can override managed defaults, matching
# the user extension point in .zshrc.
if test -d "$HOME/.config/fish/dotfiles.d"
    for _dotfiles_user_config in "$HOME"/.config/fish/dotfiles.d/dotfiles-user-config-*.fish
        if test -f "$_dotfiles_user_config"
            source "$_dotfiles_user_config"
        end
    end
end
