# Tools Reference

Detailed overview of all tools configured by dotfiles.

## Terminal Emulators

| Tool | Description |
|------|-------------|
| **Ghostty** | GPU-accelerated, native, blazing fast |
| **Kitty** | Feature-rich, GPU-based rendering |
| **WezTerm** | Lua-configurable, cross-platform |
| **Alacritty** | Minimal, Rust-based, lightweight |

## Shells

| Tool | Description |
|------|-------------|
| **Nushell** | Structured data, modern syntax, pipelines |
| **Fish** | User-friendly, great defaults, no config needed |
| **Zsh** | Highly customizable, POSIX-compatible, Powerlevel10k |

## Multiplexers

| Tool | Description |
|------|-------------|
| **Tmux** | Battle-tested, widely used, lots of plugins |
| **Zellij** | Modern, WebAssembly plugins, floating panes |
| **Herdr** | Agent-focused terminal multiplexer for AI coding workflows |

## Editor

| Tool | Description |
|------|-------------|
| **Neovim** | LazyVim config with LSP, completions, AI integration |

## Prompts

| Tool | Description |
|------|-------------|
| **Starship** | Cross-shell prompt with Git integration |
| **Powerlevel10k** | Zsh prompt theme used by the shipped `.zshrc` |

## Shell Tooling

The zsh configuration starts these tools directly, so the shell step installs
them next to the shell itself.

| Tool | Used for |
|------|----------|
| **eza** | `ls`, `ll`, `la` and `tree` aliases |
| **bat** | `cat` alias, fzf previews, `BAT_THEME` |
| **ripgrep** | `grep` alias and fzf's default search |
| **fd** | `FZF_DEFAULT_COMMAND` and the fzf directory jump |
| **fzf** | `Ctrl+R` history, `Alt+C`, `eval "$(fzf --zsh)"` |
| **fnm** | Owns the Node runtime; npm globals live in `~/.npm-global` |
| **direnv** | Per-directory environments (`direnv hook zsh`) |
| **jq** | JSON in shell helpers and scripts |
| **gh** | GitHub CLI, and the credential helper configured in `.gitconfig` |
| **git-delta** | `core.pager` and `interactive.diffFilter` in `.gitconfig` |
| **xh** | `http` alias |
| **trippy** | `traceroute` and `tracert` aliases (only aliased when present) |
| **btop** | `top` alias (only aliased when present) |
| **zoxide** | `z` directory jumping |
| **atuin** | Shell history |
| **carapace** | Completion bridge for zsh, fish and bash |

Homebrew is the complete source for this list. Debian stable does not package
starship, fnm, eza, git-delta or xh, so a Debian host without Homebrew gets the
subset available in `apt` and the `.zshrc` guards degrade gracefully.

## Machine Toolset

The toolset step provisions the machine toolset the repository declares in its
`Brewfile`, on top of what the shell step installs for the configuration itself.
The step reads the `Brewfile` from the checkout the clone step created, filters
it, and hands it to `brew bundle`, so the `Brewfile` stays the single source of
truth: a tool added to it is provisioned without a change to the installer.

| Directive | Installs |
|-----------|----------|
| **brew** | Homebrew formulae (`btop`, `sops`, `age`, `k9s`, and the rest) |
| **tap** | Third-party taps a later formula comes from |
| **go** | Go module binaries |
| **npm** | Global npm packages |
| **uv** | Python tools |

The exclusion is a two-directive denylist, not an allowlist. Every other line
of the `Brewfile` is installed, so a `cask` or `mas` entry added to the file
later is provisioned without a change here; the table above names only the
directives the file carries today.

The `vscode` and `winget` sections are deliberately excluded. Installing Windows
desktop applications (Chrome, Office, Teams, a JDK) as a side effect of a
dotfiles install is out of proportion, and the VSCode extensions need the `code`
CLI, which is absent on servers. The step logs how many lines it dropped from
each section instead of hiding the omission. Both sections stay in the `Brewfile`
for anyone who applies it by hand on a workstation.

The step is best-effort: an entry that cannot be installed, and a `Brewfile`
that is missing or unreadable, are logged and the run continues. There is one
exception, and it is deliberate: the `Brewfile` is read from the checkout the
clone step created, and when that checkout is absent the step reports a failure
rather than silently skipping. A run with no checkout has already failed at the
clone step, and a silent skip there would hide that. Termux and any host without
a usable Homebrew skip the step, and `DOTFILES_SKIP_TOOLSET=1` skips it
explicitly.
