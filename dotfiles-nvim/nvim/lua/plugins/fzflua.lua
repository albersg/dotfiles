return {
  "ibhagwan/fzf-lua",
  -- Loaded on demand instead of at startup. It is only a fallback behind
  -- snacks.picker, and config/keymaps.lua reaches it through
  -- require("fzf-lua"), which still loads it lazily.
  cmd = "FzfLua",
  dependencies = { "nvim-tree/nvim-web-devicons" },
  opts = {},
}
