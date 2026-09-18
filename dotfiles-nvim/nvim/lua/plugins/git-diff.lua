-- Git diff/blame keymap allocation.
--
-- Problem this file solves: dinhhuy258/git.nvim sets GLOBAL keymaps during
-- `BufReadPre`. With its default mappings it also claims <leader>gd, <leader>gD,
-- <leader>go, <leader>gp, <leader>gn, <leader>gr, <leader>gR, racing LazyVim's
-- own keymaps (loaded on VeryLazy) and the snacks pickers. The effective owner
-- therefore depended on event order. We disable git.nvim's defaults and bind
-- only the two actions the user cares about, on keys nothing else owns.
--
-- Final allocation (verified at runtime, see the keymap table):
--   <leader>gb  LazyVim / Snacks git_log_line   (untouched)
--   <leader>gB  snacks inline blame line        (new)
--   <leader>gw  git.nvim blame window           (moved off gb, which LazyVim owns)
--   <leader>go  mini.diff overlay               (untouched)
--   <leader>gO  git.nvim browse                 (moved off go)
--   <leader>gd  diffview working tree           (see plugins/diffview.lua)
--   <leader>gD  diffview repo history           (see plugins/diffview.lua)
--   <leader>gv  diffview file history           (see plugins/diffview.lua)
return {
  {
    -- Stop git.nvim from claiming global maps it does not own.
    "dinhhuy258/git.nvim",
    event = "BufReadPre", -- Load the plugin before reading a buffer (moved from plugins/editor.lua)
    opts = {
      default_mappings = false,
      keymaps = {
        blame = "<leader>gw", -- git.nvim floating blame window (gb is LazyVim's picker)
        browse = "<leader>gO", -- moved off <leader>go so mini.diff keeps the overlay
        -- Explicitly disabled: reachable through :GitDiff, :GitRevert,
        -- :GitRevertFile, :GitCreatePullRequest when needed.
        diff = "",
        diff_close = "",
        revert = "",
        revert_file = "",
        open_pull_request = "",
        create_pull_request = "",
      },
    },
  },

  {
    -- mini.diff already ships hunk navigation (]h / [h / ]H / [H) and the
    -- stage/reset operators (`gh` / `gH`); we only add discoverable <leader>gh*
    -- wrappers so no duplicate bindings are introduced for the existing keys.
    "nvim-mini/mini.diff",
    keys = {
      { "<leader>ghs", "ghgh", remap = true, mode = "n", desc = "Stage Hunk" },
      { "<leader>ghs", "gh", remap = true, mode = "x", desc = "Stage Selection" },
      { "<leader>ghr", "gHgh", remap = true, mode = "n", desc = "Reset Hunk" },
      { "<leader>ghr", "gH", remap = true, mode = "x", desc = "Reset Selection" },
      { "<leader>ghp", function() require("mini.diff").toggle_overlay(0) end, desc = "Preview Hunk (overlay)" },
    },
  },

  {
    -- Floating blame for the current line, and relocation of the snacks
    -- git_diff pickers. The snacks_picker extra and diffview.nvim both claim
    -- <leader>gd / <leader>gD; cross-plugin lazy key collisions are resolved by
    -- registration order, which is not stable. We disable the snacks duplicates
    -- and move them to <leader>gc / <leader>gC so each lhs has one owner.
    "folke/snacks.nvim",
    keys = {
      { "<leader>gd", false },
      { "<leader>gD", false },
      { "<leader>gc", function() Snacks.picker.git_diff() end, desc = "Git Diff (hunks)" },
      {
        "<leader>gC",
        function() Snacks.picker.git_diff({ base = "origin", group = true }) end,
        desc = "Git Diff (origin)",
      },
      { "<leader>gB", function() Snacks.git.blame_line() end, desc = "Blame Line (inline)" },
    },
  },
}
