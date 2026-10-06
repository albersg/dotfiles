# The theme library: complete palettes for the partials, and more themes (T2 + T4)

The user's ask, in their words: *"more themes: one purple-pink, one darker, one
lighter, etc."* and *"themes that do not exist in some places"*. Before this
change the repository offered two themes (`dotfiles`, `catppuccin-mocha`), held
three more as names with no palette (`everforest`, `kanagawa`, `kagawa`) and
would not offer them, and `[syntax]` was outside the completion rule, so a theme
could be offered without being previewable.

## The rule that governs every value

**No colour is invented.** A value is either what the repository already ships or
a transcription of a published palette, cited in `themes/README.md` with the
project, the URL and the roles it holds. A role the published palette does not
define stays empty. There is no network fetch; the transcriptions are committed
as definition files.

## What changed

| # | Task | State |
|---|---|---|
| T1 | Give the partial themes a palette | `everforest` and `kanagawa` transcribed from their published palettes and now complete; `kagawa` retired (it was never a theme) |
| T2 | The new themes the user asked for | `rose-pine` (purple-pink, and the darkest new base), `catppuccin-latte` (the light flavour); Rosé Pine Moon considered and left out |
| T3 | The completion rule | `[syntax]` is now part of `Complete()`: a theme is offered only when it can be applied **and** shown |
| T4 | Provenance, written | `themes/README.md` gains a per-theme origin section and a "what is transcription and what is the repository's" note; each definition carries `[theme] provenance` |
| T5 | Guards with teeth | `TestOnlyThemesThatCanBePreviewedAreOffered`, `TestNoInventedThemeRoleSlipsIn`; the stale Mocha-no-light guard and the partial-preview guard were updated rather than deleted |

## The themes now

| Theme | State | Source | Covers | Leaves out |
|---|---|---|---|---|
| dotfiles | complete | the repository's six hand-written blocks | terminals, Starship, zsh/p10k, Herdr, bat | fish, Neovim, tmux |
| Catppuccin Mocha | complete | ghostty conf + starship table | the nine above + Neovim | fish, tmux |
| Catppuccin Latte | complete | catppuccin/catppuccin (published Latte) | terminals, Starship, zsh/p10k, Herdr | fish, bat, Neovim, tmux |
| Kanagawa | complete | rebelot/kanagawa.nvim (wave) | terminals, zsh/p10k, Herdr, Neovim | Starship, fish, bat, tmux |
| Everforest | complete | sainnhe/everforest (dark medium) | terminals, zsh/p10k, Herdr | Starship, fish, bat, Neovim, tmux |
| Rosé Pine | complete | rose-pine/rose-pine | terminals, zsh/p10k, Herdr | Starship, fish, bat, Neovim, tmux |

## Kagawa, with the evidence

`themes/kagawa.toml` is removed. The repository's only Kagawa file,
`dotfiles-fish/fish/themes/Kagawa.theme`, was introduced upstream as a
byte-for-byte copy of `Kanagawa.theme`, with Kanagawa's own header. No published
"Kagawa" palette exists. A name without a theme is not offered, so the definition
is gone.

**Open item outside this change's edit surfaces:** the generated fish file
`dotfiles-fish/fish/themes/Kagawa.theme` still exists in the tree and should be
deleted in the same commit, or the repository keeps shipping a Kagawa file for a
theme that no longer exists. See the `risks` note in the handoff.

## The `[syntax]` rule

`Complete()` now requires the twenty-two canonical roles **and** `keyword_dark`
plus `string_dark`, the two `[syntax]` members the preview reads. The light
member stays optional (the preview falls back to the dark one), so a published
palette with no light flavour is not forced to invent one. Catppuccin is the
exception that proves the pairing: Mocha's light member is now Latte's mauve and
green, so the readability limit recorded in the old README is gone.

Teeth:

- `TestOnlyThemesThatCanBePreviewedAreOffered` builds a definition with every
  terminal role and no `[syntax]` and asserts it is not complete, is not offered,
  and that every offered theme previews. Dropping `[syntax]` from `Complete()`
  fails it.
- `TestNoInventedThemeRoleSlipsIn` asserts the parser refuses a palette role
  outside the canonical twenty-two and a `[syntax]` role outside the four the
  preview reads, and that every shipped definition cites a provenance.

## The frame constraint (why Rosé Pine Moon is not here)

The Utilities section fits **six** complete theme rows at the 60×20 floor. A
seventh row overflows the frame guard
(`TestEveryScreenFitsEveryTerminalSize/utilities/60x20`) by one row. That guard lives
in `installer/internal/tui/screen_coverage_test.go`, which is not an edit surface
of this change. Rosé Pine Moon was therefore left out, on top of the honest
reason: its base (#232136) is *lighter* than Rosé Pine's (#191724), so it does not
answer the "más oscuro" the user asked for in the first place.

## Validation

- `cd installer && gofmt -l .` — clean
- `cd installer && go vet ./...` — clean
- `cd installer && go test ./...` — green (no golden moved)
- `make preflight` — green
