return {
  -- External plugin (third-party, not related to this project)
  "Gentleman-Programming/veil.nvim",
  config = function()
    require("veil").setup()
  end,
}
