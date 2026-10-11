-- Options are automatically loaded before lazy.nvim startup
-- Default options that are always set: https://github.com/LazyVim/LazyVim/blob/main/lua/lazyvim/config/options.lua
-- Add any additional options here

-- Use the system clipboard for all operations (this also covers WSL).
vim.opt.clipboard = "unnamedplus"

-- Over SSH the system clipboard is out of reach: Neovim would look for X11,
-- Wayland or win32yank tools on the remote host and never touch the terminal we
-- are actually typing in. Install the OSC 52 clipboard instead so copies made
-- inside Neovim travel back through the terminal to the local machine.
if vim.env.SSH_CONNECTION then
  local osc52 = require("vim.ui.clipboard.osc52")
  vim.g.clipboard = {
    name = "OSC 52",
    copy = {
      ["+"] = osc52.copy("+"),
      ["*"] = osc52.copy("*"),
    },
    paste = {
      ["+"] = osc52.paste("+"),
      ["*"] = osc52.paste("*"),
    },
  }
end
