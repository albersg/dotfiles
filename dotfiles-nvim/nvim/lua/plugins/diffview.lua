-- diffview.nvim — rich side-by-side diffs and file history.
--
-- LazyVim has no diffview extra (checked this install's
-- lua/lazyvim/plugins/extras tree), so this is an explicit spec.
--
-- Key allocation: <leader>gd / <leader>gD previously belonged to git.nvim's
-- simple unified-diff buffer. That is disabled in plugins/git-diff.lua and
-- diffview now owns the diff keys; the snacks git_status/git_stash pickers keep
-- <leader>gs / <leader>gS. All three keys are free after the git.nvim defaults
-- are turned off. <leader>gq closes the current diffview (verified free; LazyVim
-- only owns <leader>qq = Quit All).
return {
  {
    "sindrets/diffview.nvim",
    cmd = {
      "DiffviewOpen",
      "DiffviewClose",
      "DiffviewToggleFiles",
      "DiffviewFocusFiles",
      "DiffviewFileHistory",
    },
    dependencies = { "nvim-tree/nvim-web-devicons" },
    keys = {
      { "<leader>gd", "<cmd>DiffviewOpen<cr>", desc = "Diffview (working tree)" },
      { "<leader>gD", "<cmd>DiffviewFileHistory<cr>", desc = "Diffview (repo history)" },
      { "<leader>gv", "<cmd>DiffviewFileHistory %<cr>", desc = "Diffview (file history)" },
      { "<leader>gq", "<cmd>DiffviewClose<cr>", desc = "Diffview (close)" },
    },
    opts = {},
  },
}
