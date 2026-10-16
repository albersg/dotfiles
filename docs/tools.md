# Tools Reference

Detailed overview of all tools configured by dotfiles.

Most of these tools also have a page in the installer's **Learn About Tools** hub, which is the same
content in the interface. The shell tooling table below is copied there row for row, and a test
fails by name when this document and that page disagree.

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

Herdr's sound alerts are enabled by the managed configuration, but Herdr ships no audio
decoder and shells out to an external mp3 player. The installer provisions `mpg123` when none
of the players Herdr tries (`paplay`, `pw-play`, `ffplay`, `mpg123`, `mpv`) resolves, and it
reports the gap rather than claiming success when even that cannot be provided.

## Editor

| Tool | Description |
|------|-------------|
| **Neovim** | LazyVim config with LSP, completions, AI integration |

## Plugins

The installer provisions a plugin set for five tools. For four of them the set
is a list that lives in this repository and the step copies or installs it; for
Herdr the set is declared in the installer's own source, because Herdr has no
plugin list file to copy:

Three of those sets are work this installer does and can be left out of a run:
Tmux's, with `--skip=plugins-tmux`; Herdr's, with `--skip=plugins-herdr`; and the
shell's, with `--skip=plugins-shell`. A run that leaves Tmux's out installs no
TPM, downloads no plugin, and strips the plugin declarations and the line that
runs TPM out of the `tmux.conf` it writes, so tmux is never asked to run a
checkout that is not there. A run that leaves the shell's out installs no Oh My
Zsh and no package that exists only to provide a plugin of the set, and the
`.zshrc` it writes replaces the block the `dotfiles-plugins` markers bound -- the
plugin declaration, the `source "$ZSH/oh-my-zsh.sh"` line and the third-party
loaders -- with a line saying why, so a machine without the set still starts.
Fish's and Neovim's sets are named on the same row and are not switches: they are
not the installer's to leave out at all, because `config.fish` bootstraps Fisher
on first start and lazy.nvim installs the Neovim set on first launch.

| Tool | Where the set is declared | What the installer does |
|------|---------------------------|-------------------------|
| **Tmux** | `dotfiles-tmux/tmux.conf` through TPM | Clones TPM, copies the seeded plugin directory and runs `install_plugins` |
| **Fish** | `dotfiles-fish/fish/fish_plugins` | Ships the plugin list and the plugin files with the Fish config; `config.fish` bootstraps Fisher on first start |
| **Zsh** | `.zshrc` | Installs the plugin packages (autosuggestions, syntax highlighting, completions, fzf-tab, carapace, Powerlevel10k) that the `.zshrc` sources; with the set left out it installs none of them and strips the marked block from the `.zshrc` it writes |
| **Neovim** | `dotfiles-nvim/nvim/lua/plugins/` | Ships the LazyVim plugin set, which lazy.nvim installs on first launch |
| **Herdr** | `herdrPluginSet` in `installer/internal/tui/installer.go` | Runs `herdr plugin install <owner>/<repo> --yes` for each declared plugin and verifies each with `herdr plugin list` |
herdr plugin update --yes                              # update every GitHub-installed plugin

### Herdr

The declared Herdr set is two plugins, installed from GitHub shorthand:

| Plugin | Source | What it is |
|--------|--------|------------|
| `aigora.lantern` | `aigorahub/herdr-lantern` | Lantern, by Elves |
| `herdr.auto-title` | `kryptamine/herdr-auto-title` | Auto Title, built from Go at install time (Go is required) |

The step runs only on the Herdr multiplexer choice and only where the `herdr`
binary resolves. A run can leave its set out: `--skip=plugins-herdr` turns it
off, and the step then runs no `herdr` command, installs nothing, updates
nothing, and reports the set as left out. It installs each plugin with `--yes`, the noninteractive form
of `herdr plugin install`, then asks the machine what it has with `herdr plugin
list` and reports the revision Herdr recorded for each plugin
(`source.resolved_commit`). A plugin that failed to install, or whose build
failed, is reported as a failure and not as an installed plugin.

**The set is installed every run, and updated where the CLI can.** `herdr plugin
install` re-resolves the repository's default branch and replaces the managed
checkout it owns, and on Herdr 0.9.4 or newer `herdr plugin update --yes` updates
every GitHub-installed plugin -- including any you added yourself. On an older CLI the
step names the version it found and reports that the plugins were installed but not
updated, because `plugin update` did not exist before 0.9.4. The installer never
uninstalls, unlinks or disables a plugin. The same commands:

```bash
herdr plugin install <owner>/<repo>/<subdir> --yes   # install or update (GitHub shorthand only)
herdr plugin link <path>                             # link a plugin from a working tree
herdr plugin list                                    # list the plugins installed for this user
herdr plugin uninstall <id>                          # unregister an installed plugin
herdr plugin unlink <id>                             # unregister a linked one
```

Herdr's keymaps screen in the installer names the same commands.

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
| **eza** | `ls`, `ll`, `la`, `lt` and `tree` aliases, in zsh and in fish |
| **bat** | `cat` function (plain output, no paging), in zsh and in fish; fzf previews; `BAT_THEME` |
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
