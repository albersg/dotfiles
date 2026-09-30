-- Google Antigravity CLI integration (successor to Gemini CLI).
--
-- Replaces jonroosevelt/gemini-cli.nvim, which is deprecated and whose own
-- plugin/gemini.vim calls setup() at load time, running it a second time on top
-- of the config function this file used to declare.
--
-- Shipped disabled, like every other AI plugin here: the configuration enables
-- only one assistant at a time and <leader>a is shared by all of them. Turn it
-- on by removing its entry from lua/plugins/disabled.lua (and leaving the other
-- AI plugins off).
return {
  "mceazy2700/antigravity-cli.nvim",
  dependencies = {
    "nvim-lua/plenary.nvim",
    "folke/snacks.nvim",
  },
  cmd = {
    "Antigravity",
    "AntigravityAsk",
    "AntigravitySelectModel",
    "AntigravityResume",
    "AntigravityDiffAccept",
    "AntigravityDiffDeny",
  },
  keys = {
    { "<leader>a", nil, desc = "AI/Antigravity" },
    { "<leader>ac", "<cmd>Antigravity<cr>", desc = "Toggle Antigravity" },
    { "<leader>ar", "<cmd>AntigravityResume<cr>", desc = "Resume Antigravity" },
    { "<leader>am", "<cmd>AntigravitySelectModel<cr>", desc = "Select Antigravity model" },
    { "<leader>as", "<cmd>AntigravityAsk<cr>", mode = { "n", "v" }, desc = "Ask Antigravity" },
    { "<leader>aa", "<cmd>AntigravityDiffAccept<cr>", desc = "Accept diff" },
    { "<leader>ad", "<cmd>AntigravityDiffDeny<cr>", desc = "Deny diff" },
  },
  opts = {
    command = "agy",
    terminal = {
      provider = "native",
      position = "left", -- mirrors the Claude Code panel placement
      size = 55,         -- columns; plugin default is 80, which is too wide here
    },
  },
}
