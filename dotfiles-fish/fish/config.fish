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
set -g PALETTE_BASE_SGR "38;2;6;8;15"
set -g PALETTE_SURFACE_SGR "38;2;38;51;86"
set -g PALETTE_TEXT_SGR "38;2;243;246;249"
set -g PALETTE_MUTED_SGR "38;2;138;143;163"
set -g PALETTE_RED_SGR "38;2;203;124;148"
set -g PALETTE_GREEN_SGR "38;2;183;204;133"
set -g PALETTE_YELLOW_SGR "38;2;255;224;102"
set -g PALETTE_BLUE_SGR "38;2;127;180;202"
set -g PALETTE_MAGENTA_SGR "38;2;255;141;215"
set -g PALETTE_CYAN_SGR "38;2;122;168;159"
set -g PALETTE_SURFACE_BG_SGR "48;2;38;51;86"
# The two listing tables, from the same mapping the zsh block reads: fish's
# spelling of a reference and of an export is the only difference, so a file
# keeps the colour the other shell gives it and a switch moves both listings.

# --- File listings: GNU ls reads LS_COLORS, eza reads it as its base layer ----
set -gx LS_COLORS "rs=0:\
di=$PALETTE_BLUE_SGR:\
ln=$PALETTE_CYAN_SGR:\
mh=$PALETTE_MUTED_SGR:\
pi=$PALETTE_YELLOW_SGR:so=$PALETTE_YELLOW_SGR:do=$PALETTE_YELLOW_SGR:\
bd=$PALETTE_MAGENTA_SGR:cd=$PALETTE_MAGENTA_SGR:\
or=$PALETTE_RED_SGR;1:mi=$PALETTE_RED_SGR;1:ca=$PALETTE_RED_SGR:\
su=$PALETTE_RED_SGR;$PALETTE_SURFACE_BG_SGR:sg=$PALETTE_RED_SGR;$PALETTE_SURFACE_BG_SGR:\
tw=$PALETTE_GREEN_SGR;$PALETTE_SURFACE_BG_SGR:ow=$PALETTE_GREEN_SGR;$PALETTE_SURFACE_BG_SGR:\
st=$PALETTE_BLUE_SGR;$PALETTE_SURFACE_BG_SGR:\
ex=$PALETTE_GREEN_SGR:\
*.tar=$PALETTE_RED_SGR:*.tgz=$PALETTE_RED_SGR:*.tbz2=$PALETTE_RED_SGR:*.txz=$PALETTE_RED_SGR:*.zst=$PALETTE_RED_SGR:\
*.zip=$PALETTE_RED_SGR:*.7z=$PALETTE_RED_SGR:*.rar=$PALETTE_RED_SGR:\
*.gz=$PALETTE_RED_SGR:*.bz2=$PALETTE_RED_SGR:*.xz=$PALETTE_RED_SGR:\
*.png=$PALETTE_MAGENTA_SGR:*.jpg=$PALETTE_MAGENTA_SGR:*.jpeg=$PALETTE_MAGENTA_SGR:*.gif=$PALETTE_MAGENTA_SGR:*.webp=$PALETTE_MAGENTA_SGR:*.svg=$PALETTE_MAGENTA_SGR:*.ico=$PALETTE_MAGENTA_SGR:\
*.mp4=$PALETTE_MAGENTA_SGR:*.mkv=$PALETTE_MAGENTA_SGR:*.mov=$PALETTE_MAGENTA_SGR:*.webm=$PALETTE_MAGENTA_SGR:\
*.mp3=$PALETTE_CYAN_SGR:*.flac=$PALETTE_CYAN_SGR:*.wav=$PALETTE_CYAN_SGR:*.ogg=$PALETTE_CYAN_SGR:*.m4a=$PALETTE_CYAN_SGR:\
*.pdf=$PALETTE_YELLOW_SGR:*.md=$PALETTE_YELLOW_SGR:*.txt=$PALETTE_YELLOW_SGR:*.rst=$PALETTE_YELLOW_SGR:\
*.sh=$PALETTE_GREEN_SGR:*.bash=$PALETTE_GREEN_SGR:*.zsh=$PALETTE_GREEN_SGR:*.fish=$PALETTE_GREEN_SGR:\
*.py=$PALETTE_GREEN_SGR:*.go=$PALETTE_GREEN_SGR:*.rs=$PALETTE_GREEN_SGR:*.js=$PALETTE_GREEN_SGR:*.ts=$PALETTE_GREEN_SGR:\
*.json=$PALETTE_GREEN_SGR:*.yaml=$PALETTE_GREEN_SGR:*.yml=$PALETTE_GREEN_SGR:*.toml=$PALETTE_GREEN_SGR:\
*.db=$PALETTE_BLUE_SGR:*.sqlite=$PALETTE_BLUE_SGR:*.sql=$PALETTE_BLUE_SGR:\
*.log=$PALETTE_MUTED_SGR:*.lock=$PALETTE_MUTED_SGR"

# --- eza metadata ------------------------------------------------------------
# eza paints permissions, owner, size and date from its own defaults (bold
# yellow, red, green, blue) which fight with the file names and with every other
# tool in the terminal. Metadata is de-emphasised here and colour is left to
# carry meaning: the names, the git state, and the security bits in the
# permission column.
set -gx EZA_COLORS "\
oc=$PALETTE_MUTED_SGR:\
ur=$PALETTE_MUTED_SGR:uw=$PALETTE_MUTED_SGR:ux=$PALETTE_MUTED_SGR:ue=$PALETTE_MUTED_SGR:\
gr=$PALETTE_MUTED_SGR:gw=$PALETTE_MUTED_SGR:gx=$PALETTE_MUTED_SGR:\
tr=$PALETTE_MUTED_SGR:tw=$PALETTE_MUTED_SGR:tx=$PALETTE_MUTED_SGR:\
su=$PALETTE_YELLOW_SGR;1:sf=$PALETTE_YELLOW_SGR;1:xa=$PALETTE_MAGENTA_SGR:\
sn=$PALETTE_CYAN_SGR:nb=$PALETTE_CYAN_SGR:nk=$PALETTE_CYAN_SGR:nm=$PALETTE_CYAN_SGR:ng=$PALETTE_CYAN_SGR:nt=$PALETTE_CYAN_SGR:\
sb=$PALETTE_MUTED_SGR:ub=$PALETTE_MUTED_SGR:uk=$PALETTE_MUTED_SGR:um=$PALETTE_MUTED_SGR:ug=$PALETTE_MUTED_SGR:ut=$PALETTE_MUTED_SGR:\
df=$PALETTE_MUTED_SGR:ds=$PALETTE_MUTED_SGR:lc=$PALETTE_MUTED_SGR:lm=$PALETTE_MUTED_SGR:\
uu=$PALETTE_BLUE_SGR:un=$PALETTE_MUTED_SGR:uR=$PALETTE_RED_SGR:\
gu=$PALETTE_BLUE_SGR:gn=$PALETTE_MUTED_SGR:gR=$PALETTE_RED_SGR:\
xx=$PALETTE_SURFACE_SGR:\
da=$PALETTE_MUTED_SGR:in=$PALETTE_MUTED_SGR:bl=$PALETTE_MUTED_SGR:\
hd=$PALETTE_BLUE_SGR;1:lp=$PALETTE_CYAN_SGR:cc=$PALETTE_RED_SGR:bO=$PALETTE_RED_SGR;4:\
sp=$PALETTE_MAGENTA_SGR:mp=$PALETTE_MAGENTA_SGR:\
im=$PALETTE_MAGENTA_SGR:vi=$PALETTE_MAGENTA_SGR:mu=$PALETTE_CYAN_SGR:lo=$PALETTE_CYAN_SGR:\
cr=$PALETTE_YELLOW_SGR:do=$PALETTE_YELLOW_SGR:co=$PALETTE_RED_SGR:tm=$PALETTE_MUTED_SGR:cm=$PALETTE_MUTED_SGR:\
ga=$PALETTE_GREEN_SGR:gm=$PALETTE_YELLOW_SGR:gd=$PALETTE_RED_SGR:gv=$PALETTE_MAGENTA_SGR:\
gt=$PALETTE_YELLOW_SGR:gi=$PALETTE_MUTED_SGR:gc=$PALETTE_RED_SGR;1:\
Gm=$PALETTE_BLUE_SGR;1:Go=$PALETTE_CYAN_SGR:Gc=$PALETTE_GREEN_SGR:Gd=$PALETTE_YELLOW_SGR"

# The shell's own roles: the prompt, strings, errors, and the completion pager.
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

## functions
# The four commands zsh has and fish had not. They sit after the generated theme block
# and before the user's drop-ins, so a drop-in can still override them.

# `cat` shows a file the way zsh's does: bat's plain output, without paging. An
# argument that begins with a dash goes to the real cat instead, which is what keeps
# the flags working, and the command is a function rather than an alias so a script
# does not inherit it.
function cat
    for arg in $argv
        if string match -q -- '-*' $arg
            command cat $argv
            return
        end
    end
    bat --paging=never --style=plain -- $argv
end

# extract <archive>: unpack almost any archive format.
function extract
    if test (count $argv) -eq 0
        echo 'usage: extract <archive>' >&2
        return 1
    end
    set -l file $argv[1]
    if not test -f "$file"
        echo "extract: no such file: $file" >&2
        return 1
    end

    # Each format is paired with the tool it needs, so a missing dependency is
    # reported by name instead of surfacing as a bare "command not found". The order
    # matters: the first pattern that matches wins, so *.tar.gz is asked before *.gz.
    set -l tool
    set -l cmd
    switch $file
        case '*.tar.gz' '*.tgz'
            set tool tar
            set cmd tar -xzf $file
        case '*.tar.bz2' '*.tbz2'
            set tool tar
            set cmd tar -xjf $file
        case '*.tar.xz' '*.txz'
            set tool tar
            set cmd tar -xJf $file
        case '*.tar.zst'
            set tool tar
            set cmd tar --zstd -xf $file
        case '*.tar'
            set tool tar
            set cmd tar -xf $file
        case '*.zip'
            set tool unzip
            set cmd unzip -q $file
        case '*.7z'
            set tool 7z
            set cmd 7z x $file
        case '*.rar'
            set tool unrar
            set cmd unrar x $file
        case '*.gz'
            set tool gunzip
            set cmd gunzip -kf $file
        case '*.bz2'
            set tool bunzip2
            set cmd bunzip2 -kf $file
        case '*.xz'
            set tool unxz
            set cmd unxz -kf $file
        case '*.zst'
            set tool zstd
            set cmd zstd -df $file
        case '*'
            echo "extract: unsupported format: $file" >&2
            return 1
    end

    if not command -q $tool
        echo "extract: needs $tool, which is not installed" >&2
        return 1
    end
    $cmd
end

# compress <archive> <path>...: build an archive, format taken from the target
# extension. The single-file formats refuse a directory rather than producing an
# archive that cannot be unpacked back.
function compress
    if test (count $argv) -lt 2
        echo 'usage: compress <archive> <path> [path...]' >&2
        echo '       targets: .tar.gz .tar.bz2 .tar.xz .tar.zst .zip .7z .gz .bz2 .xz .zst' >&2
        return 1
    end
    set -l archive $argv[1]
    set -l paths $argv[2..-1]

    # tar is named before the table for the same reason zsh does it: it is the tool
    # most of the formats need, and a missing one should say which tool it is.
    switch $archive
        case '*.tar.*' '*.tgz'
            if not command -q tar
                echo 'compress: needs tar, which is not installed' >&2
                return 1
            end
    end

    set -l tool
    set -l cmd
    switch $archive
        case '*.tar.gz' '*.tgz'
            set tool tar
            set cmd tar -czf $archive $paths
        case '*.tar.bz2'
            set tool tar
            set cmd tar -cjf $archive $paths
        case '*.tar.xz'
            set tool tar
            set cmd tar -cJf $archive $paths
        case '*.tar.zst'
            set tool tar
            set cmd tar --zstd -cf $archive $paths
        case '*.zip'
            set tool zip
            set cmd zip -qr $archive $paths
        case '*.7z'
            set tool 7z
            set cmd 7z a -bso0 -bsp0 $archive $paths
        case '*.gz' '*.bz2' '*.xz' '*.zst'
            if test (count $paths) -ne 1
                echo "compress: $archive holds one file; use a .tar.* or .zip target for several" >&2
                return 1
            end
            set -l src $paths[1]
            if test -d "$src"
                echo "compress: $archive holds one file; use a .tar.* or .zip target for a directory" >&2
                return 1
            end
            if not test -f "$src"
                echo "compress: no such file: $src" >&2
                return 1
            end
            switch $archive
                case '*.gz'
                    set tool gzip
                    set cmd gzip -c $src
                case '*.bz2'
                    set tool bzip2
                    set cmd bzip2 -c $src
                case '*.xz'
                    set tool xz
                    set cmd xz -c $src
                case '*.zst'
                    set tool zstd
                    set cmd zstd -qc $src
            end
            if not command -q $tool
                echo "compress: needs $tool, which is not installed" >&2
                return 1
            end
            # The redirection creates the target before the compressor runs, so a
            # failure would otherwise leave an empty archive behind and still
            # report success.
            if not $cmd >$archive
                rm -f $archive
                echo "compress: failed to create $archive" >&2
                return 1
            end
            echo "created $archive"
            return 0
        case '*'
            echo "compress: unsupported format: $archive" >&2
            return 1
    end

    if not command -q $tool
        echo "compress: needs $tool, which is not installed" >&2
        return 1
    end
    if $cmd
        echo "created $archive"
    else
        rm -f $archive
        echo "compress: failed to create $archive" >&2
        return 1
    end
end

# serve [port]: static file server for the current directory, defaulting to 8000.
function serve
    set -l port $argv[1]
    if test -z "$port"
        set port 8000
    end
    if not string match -qr '^[0-9]+$' -- $port
        echo "serve: port must be a whole number, got '$port'" >&2
        return 1
    end
    if test $port -lt 1; or test $port -gt 65535
        echo "serve: port must be between 1 and 65535, got $port" >&2
        return 1
    end
    if not command -q python3
        echo 'serve: needs python3, which is not installed' >&2
        return 1
    end
    if command -q ss; and ss -ltn 2>/dev/null | grep -q ":$port "
        echo "serve: port $port is already in use" >&2
        return 1
    end
    echo "serving $PWD on http://localhost:$port  (Ctrl-C to stop)"
    python3 -m http.server $port --bind 127.0.0.1
end

## hooks
# The two things zsh does on every directory change, and the three commands that keep
# secrets encrypted. Declared here for the same reason as the commands above: outside the
# generated block, and before the user's drop-ins.

# Activate a project's virtual environment on cd, and leave it on the way out. Only an
# environment this hook activated is ever deactivated, so one the user activated by hand
# is never touched, and moving around inside a project does not re-activate or
# re-announce anything.
set -g _DOTFILES_VENV ""
function _dotfiles_activate_venv --on-variable PWD
    # Search the current directory and, when inside a repository, its ancestors up to the
    # repository root only. Walking all the way to / would activate a stray venv sitting
    # in /tmp or in $HOME for every directory underneath it, which is how a scratch
    # environment ends up capturing unrelated projects. The .git test costs one stat and
    # no subprocess, which matters on every cd.
    set -l search $PWD
    set -l dir $PWD
    while test "$dir" != /; and test "$dir" != "$HOME"
        set dir (path dirname $dir)
        set -a search $dir
        if test -e "$dir/.git"
            break
        end
    end
    if not test -e "$search[-1]/.git"
        set search $PWD
    end

    set -l found ""
    for dir in $search
        for candidate in .venv venv .env
            # activate.fish rather than the POSIX activate beside it: fish cannot source
            # that file at all, and an environment that has only got one is left alone
            # instead of half-activated.
            if test -f "$dir/$candidate/bin/activate.fish"
                set found "$dir/$candidate"
                break
            end
        end
        if test -n "$found"
            break
        end
    end

    if test "$found" = "$_DOTFILES_VENV"
        return 0
    end

    if test -n "$_DOTFILES_VENV"
        if test -n "$VIRTUAL_ENV"; and test "$VIRTUAL_ENV" = "$_DOTFILES_VENV"
            deactivate 2>/dev/null
        end
        set -g _DOTFILES_VENV ""
    end

    if test -n "$found"
        source "$found/bin/activate.fish"
        set -g _DOTFILES_VENV "$found"
        echo "venv: "(path basename $found)
    end
end

# Announce a project's available tasks when entering it. Silent when the project
# declares none, and skippable with DOTFILES_NO_TASKS=1.
function _dotfiles_show_tasks --on-variable PWD
    if test -n "$DOTFILES_NO_TASKS"
        return 0
    end
    if test "$PWD" = "$HOME"
        return 0
    end

    if test -f justfile; or test -f Justfile
        command -q just; and just --list 2>/dev/null | head -20
    else if test -f Makefile; or test -f makefile
        # List declared targets without executing anything: the awk pass only reads lines
        # shaped like a target rule, so a Makefile that runs work at parse time stays
        # untouched.
        awk -F: '/^[a-zA-Z0-9_.-]+:([^=]|$)/ { print "  make " $1 }' Makefile 2>/dev/null | sort -u | head -20
    end

    if test -f package.json; and command -q jq
        jq -r '.scripts // {} | keys[]' package.json 2>/dev/null | head -20 | sed 's/^/  npm run /'
    end

    if test -f Taskfile.yml; or test -f Taskfile.yaml
        command -q task; and task --list 2>/dev/null | head -20
    end
end

## secrets
# A file encrypted with SOPS keeps its keys readable and only its values encrypted, so it
# can live in a repository and still show which keys exist and which one changed. The key
# is a single age file rather than a GPG keyring, which avoids the agent and pinentry
# dance that is fragile under WSL.
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
# An age key has no recovery path. Lose that file and every encrypted value is gone for
# good, so back it up somewhere that is not this machine.
set -g DOTFILES_SOPS_FILE secrets.env
if not set -q SOPS_AGE_KEY_FILE
    set -gx SOPS_AGE_KEY_FILE $HOME/.config/sops/age/keys.txt
end

# Every secrets helper starts here, so a missing tool or key is reported once, with the
# command that fixes it, instead of failing somewhere further down.
function _dotfiles_secrets_ready
    if not command -q sops
        echo 'secrets: sops is not installed' >&2
        return 1
    end
    if not test -f "$SOPS_AGE_KEY_FILE"
        set -l keydir (path dirname $SOPS_AGE_KEY_FILE)
        echo "secrets: no age key at $SOPS_AGE_KEY_FILE" >&2
        echo "  create one with: mkdir -p $keydir && age-keygen -o $SOPS_AGE_KEY_FILE" >&2
        return 1
    end
    return 0
end

# env-edit [file]: edit an encrypted file. SOPS decrypts into the editor and re-encrypts on
# save, so the plaintext never lands on disk.
function env-edit
    _dotfiles_secrets_ready; or return 1
    set -l file $argv[1]
    if test -z "$file"
        set file $DOTFILES_SOPS_FILE
    end
    if not test -f "$file"
        echo "secrets: no such file: $file" >&2
        return 1
    end
    sops $file
end

# env-load [file] [.env]: decrypt into a local file for tools that insist on reading one.
# It refuses unless the destination is already ignored by git, because writing plaintext
# secrets into a repository is the mistake this whole arrangement exists to prevent.
function env-load
    _dotfiles_secrets_ready; or return 1
    set -l src $argv[1]
    if test -z "$src"
        set src $DOTFILES_SOPS_FILE
    end
    set -l dst $argv[2]
    if test -z "$dst"
        set dst .env
    end
    if not test -f "$src"
        echo "secrets: no such file: $src" >&2
        return 1
    end
    # Refuse unless git already ignores the destination. Writing a plaintext secret into a
    # repository is the mistake this arrangement exists to avoid, and creating a fresh .env
    # that git would happily track is the same mistake with a different name, so the check
    # runs whether or not the file exists. Outside a work tree there is nothing to leak
    # into, so it is allowed.
    if git rev-parse --is-inside-work-tree >/dev/null 2>&1; and not git check-ignore -q -- "$dst" 2>/dev/null
        echo "secrets: git does not ignore $dst, so the plaintext would be tracked" >&2
        echo "  add it to .gitignore, or to a global exclude, and try again" >&2
        return 1
    end
    if test -e "$dst"; and not test -w "$dst"
        echo "secrets: cannot write $dst" >&2
        return 1
    end
    if not sops -d "$src" >"$dst"
        rm -f "$dst"
        echo "secrets: could not decrypt $src" >&2
        return 1
    end
    echo "secrets: wrote $dst"
end

# env-export [file]: print export lines for the current shell, for an .envrc. Nothing is
# written to disk this way, which is the option to prefer.
function env-export
    _dotfiles_secrets_ready; or return 1
    set -l src $argv[1]
    if test -z "$src"
        set src $DOTFILES_SOPS_FILE
    end
    if not test -f "$src"
        echo "secrets: no such file: $src" >&2
        return 1
    end
    sops -d --output-type dotenv "$src" 2>/dev/null | sed 's/^/export /'
end

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
