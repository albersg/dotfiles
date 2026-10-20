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
| U3 | btop: a generated theme file and `color_theme` | `installer.go`, `dotfiles-btop/…` (new) | S1, S3–S5 | (delegated) |
| U4 | fzf in fish: the colours zsh already sets | `dotfiles-fish/fish/config.fish`, `installer.go` | S1, S3–S5 | `219be27` |
| U5 | delta: `[delta]` colours from the palette | `.gitconfig`, `installer.go` | S1, S3–S5 | |
| U6 | the docs and the contrast note | `themes/README.md`, `docs/tui-installer.md` | S3, S5 | `08c2e75` |
| U7 | zellij's layouts, not only its config | `dotfiles-zellij/zellij/layouts/*.kdl`, `installer.go` | S1, S3–S5 | |
| U8 | tmux: the plugin's own theme name | `dotfiles-tmux/tmux.conf`, `installer.go` | S1, S3–S5 | |
| U9 | Herdr's built-in theme name | `dotfiles-herdr/config.toml`, `installer.go` | S1, S3–S5 | |
| U10 | nushell's bat previews and fish's `fish_color_match` | `dotfiles-nushell/config.nu`, `dotfiles-fish/fish/config.fish` | S1, S3–S5 | |
| U11 | the stale ghostty theme asset that also ships | `dotfiles-ghostty`, `installer.go` | S3, S5 | |

## Log

- L1 (owner, verbatim): "pues revisa todo eso. Pero todo todo. Revis todos los posibels huecos que hay en todos lados en nvim, lazygit, todas y cada una de las herrameintas que se instalan en el installer, en zellij, fzf..., ghostyy, bat, todo todoo todo."
- L2 (owner, verbatim): "Primero investiga todas las partes donde faltan cambiar colores, y luego me arreglando todo esto que te he dicho,"
- L3 (evidence, the investigation): `themeTools` in `installer.go:1077` names thirteen tools (alacritty, kitty, wezterm, ghostty, starship, zsh, p10k, herdr, fish, bat, nvim, tmux, nushell) and `TestEveryOfferedThemePaintsEveryTool` holds them green. The shipped configurations with no theme block are `dotfiles-zellij/zellij/config.kdl` (47 unique hex, a fixed `kanagawa_dragon` theme and `theme "kanagawa_wave"`), `.gitconfig` (runs `delta` as pager, no `[delta]` colours), and the fish `FZF_DEFAULT_OPTS` universal variable (`--height 40%` only, while `.zshrc:414` paints fzf from `PALETTE_*`). lazygit and btop are installed on every platform and ship **no** configuration at all. Checked and already painted: eza/LS_COLORS (both shells read `PALETTE_*_SGR`), zsh plugins, herdr (two colour settings, both themed), nvim's lualine (`auto`), and nvim's active colorscheme (generated).
- L4 (decision): the units follow the existing generator shape rather than a second table: one `renderXTheme(def)` per tool, one `themeTools` entry with `Available`, one source block for drift, and a reload entry.
- L5 (evidence, contrast): body text on `base` clears AAA for all seven themes (dotfiles 18.45, rose-pine 13.39, mocha 11.34, kanagawa 11.26, nocturne 10.15, everforest 7.38, latte 7.06). The accent roles below AA 4.5 on their own base are the published palettes' own values, not invented here: latte green 2.96 / yellow 2.31 / blue 4.34 / magenta 2.34 / cyan 3.31, kanagawa red 3.22, rose-pine green 3.38, everforest bright_black 3.21. The documentation names them; no colour is changed to hide them.
- L6 (evidence, the exhaustive sweep): a read-only pass over every shipped configuration found what the first investigation missed. **zellij's five layouts** (`dotfiles-zellij/zellij/layouts/*.kdl`) each carry 6-7 hex values in the zjstatus bar and nothing rewrites them — and `#C65C5B` appears byte-identical in all five, labelled "Kanagawa red" in `work_sakura.kdl:23`, so the Everforest, Sakura and Rosé Pine layouts paint Kanagawa's red for an active tab; `config.kdl:480 default_layout "work_kanagawa"` is hardcoded outside the generated block. **tmux** sets `@kanagawa-theme 'dragon'` at `dotfiles-tmux/tmux.conf:34`, outside the generated block, so the plugin owns the git/cpu/ram segments in dragon colours under every theme. **Herdr** keeps `name = "vesper"` at `dotfiles-herdr/config.toml:13`, outside the block, so only three tokens follow the definition. **nushell** hardcodes `bat --theme=gruvbox-dark` at `config.nu:919,923`, a theme the repository does not even ship. **fish** keeps `fish_color_match:7cafc2` in `fish_variables:25`, a zenburn blue no definition holds. **ghostty** ships `themes/catppuccin-mocha.conf` (an unreferenced palette that is also the provenance input for `themes/catppuccin-mocha.toml`) into the user's config. Also found: `lazy.lua:78` named `tokyonight` as LazyVim's install fallback and `colorscheme.lua:87` painted the dashboard emblem with `#7fb4ca`; both fixed in `27eb776`. **atuin** is installed with no configuration, so it keeps its own theme.
