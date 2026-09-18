# nvim scripts

## `safe-update.sh` — explicit, reversible plugin updates

`Lazy` can update every plugin in one shot, but a bad update only shows up on the
next interactive start — with no obvious rollback point. This script turns the
update into a transaction with a guaranteed way back.

```bash
scripts/safe-update.sh                  # real update (snapshot + sync + health gate)
scripts/safe-update.sh --dry-run        # report only: Lazy check + health, no writes
scripts/safe-update.sh --strict-health  # also fail if :checkhealth ERRORs grow
```

What it does, in order:

1. Commits any uncommitted change in `~/.config/nvim` with a timestamped message
   (the config rollback point).
2. Backs up `lazy-lock.json` to `~/.cache/nvim/lazy-lock.pre-update.json`.
3. Runs `nvim --headless "+Lazy! sync" +qa`, logging to
   `~/.cache/nvim/lazy-update.log`.
4. Health-gates: a headless start must exit `0` and write `0` bytes to stderr.
   `:checkhealth` ERROR counts are recorded and compared to a saved baseline.
5. On failure it restores the lockfile + `:Lazy restore`, prints rollback
   commands and the snapshot hash, and exits non-zero.
6. On success it prints the lockfile delta, the log path and the snapshot hash.

Nothing runs automatically — no timer, no background checker, no `checker`
integration. Updates only happen when you invoke it.

### From inside Neovim

```vim
:NzUpdatePlugins          " run the real update in a floating Snacks terminal
:NzUpdatePlugins check    " dry-run (no changes)
```

The command is registered by `lua/config/update.lua` (wired from
`lua/config/keymaps.lua`). If Snacks is unavailable it prints the command to run
manually instead of failing silently.

### Rollback

```bash
# plugins only
cp ~/.cache/nvim/lazy-lock.pre-update.json ~/.config/nvim/lazy-lock.json
nvim --headless "+Lazy! restore" +qa

# whole config (including today's WIP, which is already committed by the script)
git -C ~/.config/nvim reset --hard <snapshot hash printed by the run>
```

## Today's integration keymaps

| Key            | Action                                                    |
| -------------- | --------------------------------------------------------- |
| `<leader>ia`   | CodeGraph affected tests for the current file             |
| `<leader>ic`   | CodeGraph callers of the symbol under the cursor          |
| `<leader>iC`   | CodeGraph callees of the symbol under the cursor          |
| `<leader>ii`   | CodeGraph impact of the symbol under the cursor           |
| `<leader>iq`   | CodeGraph symbol query (prompt)                           |
| `<leader>is`   | CodeGraph index status                                    |
| `<leader>gd`   | Diffview — working tree diff                              |
| `<leader>gD`   | Diffview — repository history                             |
| `<leader>gv`   | Diffview — current file history                           |
| `<leader>ghs`  | mini.diff — stage hunk / selection                        |
| `<leader>ghr`  | mini.diff — reset hunk / selection                        |
| `<leader>ghp`  | mini.diff — preview hunk overlay                          |
| `<leader>gw`   | git.nvim — blame window (moved off `<leader>gb`)          |
| `<leader>gO`   | git.nvim — browse (moved off `<leader>go`)                |
| `<leader>gc`   | Snacks — git diff hunks (moved off `<leader>gd`)          |
| `<leader>gC`   | Snacks — git diff vs origin (moved off `<leader>gD`)      |
| `<leader>gB`   | Snacks — inline blame for the current line                |

Owner details live in the header comments of `lua/plugins/git-diff.lua`,
`lua/plugins/diffview.lua` and `lua/config/codegraph.lua`.
