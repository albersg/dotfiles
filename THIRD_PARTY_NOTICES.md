# Third Party Notices

This project includes and depends on third-party software. The list below names the services the
installer links and the configuration it adapts; the complete Go module graph lives in
`installer/go.mod`, and the full licence audit in [docs/audits/LICENSE-AUDIT.md](docs/audits/LICENSE-AUDIT.md).

## Go Module Dependencies

The installer (`installer/`) imports four modules directly. `installer/go.mod` and `installer/go.sum`
hold the complete graph, transitive modules included; this table names what the code imports and the
licence each one carries.

| Dependency | Version | License | Usage |
|------------|---------|---------|-------|
| [Bubble Tea](https://github.com/charmbracelet/bubbletea) | v1.3.10 | MIT | TUI framework and runtime |
| [Lip Gloss](https://github.com/charmbracelet/lipgloss) | v1.1.0 | MIT | Terminal styling and colour profiles |
| [teatest](https://github.com/charmbracelet/x/tree/main/exp/teatest) | v0.0.0-20251215102626 | MIT | End-to-end TUI testing, test-only and not linked into the binary |
| [termenv](https://github.com/muesli/termenv) | v0.16.0 | MIT | Terminal capability and colour detection, including the companion's palette |

All four are MIT, and so is every transitive module, with `golang.org/x/*` under BSD-3-Clause. No
copyleft (GPL, AGPL, LGPL) module reaches the binary in any form.

Corrected on 2026-09-29: this section used to name Bubbles, Cobra, Viper and go-yaml, none of which the
installer depends on, directly or transitively. An attribution file that credits software nobody uses
cannot be believed when it does matter.

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
