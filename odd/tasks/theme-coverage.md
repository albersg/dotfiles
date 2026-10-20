# Theme coverage: every installed tool paints from the one definition

The theme library in `themes/` is the single source of truth for the palettes the
repository paints. Thirteen tools read it; five installed tools that carry colour do
not. This pass closes those five, one unit per tool, each with the guard that keeps
it closed.

## Specs

- **S1 (owner, verbatim):** "pues revisa todo eso. Pero todo todo. Revis todos los posibels huecos que hay en todos lados en nvim, lazygit, todas y cada una de las herrameintas que se instalan en el installer, en zellij, fzf..., ghostyy, bat, todo todoo todo."
- **S2 (owner, verbatim):** "Primero investiga todas las partes donde faltan cambiar colores, y luego me arreglando todo esto que te he dicho,"
- **S3:** A tool that ships colour in this repository takes it from `themes/*.toml`. A colour a generated block carries is a value the definition holds (`TestGeneratedThemeBlocksInventNoColour`), and a tool a theme cannot paint is reported by `themeCoverage`, never silent.
- **S4:** The five gaps are the ones the investigation measured: **lazygit** (installed, no configuration at all), **zellij** (a fixed `kanagawa_dragon` block and `theme "kanagawa_wave"`), **btop** (installed, no configuration), **fzf in fish** (only `--height 40%`; zsh already paints it from `PALETTE_*`), and **delta** (`.gitconfig` runs it as the pager with no `[delta]` colours).
- **S5:** Every unit lands with a guard that fails on the old state: the tool is painted for every offered theme, the generated block carries only definition values, and the shipped file the definition claims to come from keeps matching it.

## Tasks

| # | Unit | Files | Specs | Commit |
|---|---|---|---|---|
| U1 | lazygit: a generated `gui.theme` block | `dotfiles-lazygit/…/config.yml` (new), `installer.go`, `install_paths_test.go` | S1, S3–S5 | |
| U2 | zellij: the `themes` block generated, not fixed | `dotfiles-zellij/zellij/config.kdl`, `installer.go` | S1, S3–S5 | |
| U3 | btop: a generated theme file and `color_theme` | `installer.go`, `dotfiles-btop/…` (new) | S1, S3–S5 | |
| U4 | fzf in fish: the colours zsh already sets | `dotfiles-fish/fish/config.fish`, `installer.go` | S1, S3–S5 | |
| U5 | delta: `[delta]` colours from the palette | `.gitconfig`, `installer.go` | S1, S3–S5 | |
| U6 | the docs and the contrast note | `themes/README.md`, `docs/tui-installer.md` | S3, S5 | |

## Log

- L1 (owner, verbatim): "pues revisa todo eso. Pero todo todo. Revis todos los posibels huecos que hay en todos lados en nvim, lazygit, todas y cada una de las herrameintas que se instalan en el installer, en zellij, fzf..., ghostyy, bat, todo todoo todo."
- L2 (owner, verbatim): "Primero investiga todas las partes donde faltan cambiar colores, y luego me arreglando todo esto que te he dicho,"
- L3 (evidence, the investigation): `themeTools` in `installer.go:1077` names thirteen tools (alacritty, kitty, wezterm, ghostty, starship, zsh, p10k, herdr, fish, bat, nvim, tmux, nushell) and `TestEveryOfferedThemePaintsEveryTool` holds them green. The shipped configurations with no theme block are `dotfiles-zellij/zellij/config.kdl` (47 unique hex, a fixed `kanagawa_dragon` theme and `theme "kanagawa_wave"`), `.gitconfig` (runs `delta` as pager, no `[delta]` colours), and the fish `FZF_DEFAULT_OPTS` universal variable (`--height 40%` only, while `.zshrc:414` paints fzf from `PALETTE_*`). lazygit and btop are installed on every platform and ship **no** configuration at all. Checked and already painted: eza/LS_COLORS (both shells read `PALETTE_*_SGR`), zsh plugins, herdr (two colour settings, both themed), nvim's lualine (`auto`), and nvim's active colorscheme (generated).
- L4 (decision): the units follow the existing generator shape rather than a second table: one `renderXTheme(def)` per tool, one `themeTools` entry with `Available`, one source block for drift, and a reload entry.
- L5 (evidence, contrast): body text on `base` clears AAA for all seven themes (dotfiles 18.45, rose-pine 13.39, mocha 11.34, kanagawa 11.26, nocturne 10.15, everforest 7.38, latte 7.06). The accent roles below AA 4.5 on their own base are the published palettes' own values, not invented here: latte green 2.96 / yellow 2.31 / blue 4.34 / magenta 2.34 / cyan 3.31, kanagawa red 3.22, rose-pine green 3.38, everforest bright_black 3.21. The documentation names them; no colour is changed to hide them.
