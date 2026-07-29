# Third Party Notices

This project includes and depends on third-party software. Below is an incomplete list of major dependencies and their licenses.

## Go Module Dependencies

The installer (`installer/`) depends on Go modules. See `installer/go.mod` for the complete dependency graph. Key dependencies include:

| Dependency | License | Usage |
|------------|---------|-------|
| [Bubbletea](https://github.com/charmbracelet/bubbletea) | MIT | TUI framework |
| [Bubbles](https://github.com/charmbracelet/bubbles) | MIT | TUI components |
| [Lip Gloss](https://github.com/charmbracelet/lipgloss) | MIT | Terminal styling |
| [Cobra](https://github.com/spf13/cobra) | Apache 2.0 | CLI framework |
| [Viper](https://github.com/spf13/viper) | MIT | Configuration management |
| [go-yaml](https://github.com/go-yaml/yaml) | MIT | YAML parsing |

## Shell Configuration

The shell configuration packages include configurations adapted from:

| Component | Source | License |
|-----------|--------|---------|
| Oh My Zsh | [ohmyzsh/ohmyzsh](https://github.com/ohmyzsh/ohmyzsh) | MIT |
| Powerlevel10k | [romkatv/powerlevel10k](https://github.com/romkatv/powerlevel10k) | MIT |
| TPM (Tmux Plugin Manager) | [tmux-plugins/tpm](https://github.com/tmux-plugins/tpm) | MIT |

## Neovim Configuration

The Neovim configuration is based on [LazyVim](https://github.com/LazyVim/LazyVim) (Apache 2.0) and includes community plugins with their respective licenses.

For a complete audit of all licenses, see [docs/audits/LICENSE-AUDIT.md](docs/audits/LICENSE-AUDIT.md).
