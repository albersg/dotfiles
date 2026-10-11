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

## Plugins

The installer provisions a plugin set for four tools, and in each case the set
is a decision this repository has already made — the step is copying or installing
a list that lives here:

| Tool | Where the set is declared | What the installer does |
|------|---------------------------|-------------------------|
| **Tmux** | `dotfiles-tmux/tmux.conf` through TPM | Clones TPM, copies the seeded plugin directory and runs `install_plugins` |
| **Fish** | `dotfiles-fish/fish/fish_plugins` | Ships the plugin list and the plugin files with the Fish config; `config.fish` bootstraps Fisher on first start |
| **Zsh** | `.zshrc` | Installs the plugin packages (autosuggestions, syntax highlighting, completions, fzf-tab, Powerlevel10k) that the `.zshrc` sources |
| **Neovim** | `dotfiles-nvim/nvim/lua/plugins/` | Ships the LazyVim plugin set, which lazy.nvim installs on first launch |

**Herdr is the one tool whose plugins are user-managed.** A Herdr plugin is
third-party code installed from GitHub shorthand, not a configuration line, and
this repository declares no plugin set for it. The installer does not choose
which third-party code you run, so it neither installs, links, updates nor
removes Herdr plugins:

```bash
herdr plugin install <owner>/<repo>/<subdir>   # install a plugin (GitHub shorthand only)
herdr plugin link <path>                        # link a plugin from a working tree
herdr plugin list                               # list the plugins installed for this user
herdr plugin uninstall <id>                     # unregister an installed plugin
herdr plugin unlink <id>                        # unregister a linked one
```

Because the installer never looks at plugin state, nothing backs it up,
refreshes it or undoes it: a reinstall copies `config.toml` and leaves the
plugins already on the machine exactly as they are. That is the deliberate
default, not a missing step. Herdr's keymaps screen in the installer names the
same commands.

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
