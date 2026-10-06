-- dotfiles-managed-config: nvim
-- name: catppuccin-latte
-- generated from themes/catppuccin-latte.toml; edit the definition, not this file
--
-- The Neovim colorscheme for the Catppuccin Latte palette. Every colour below is a
-- role themes/catppuccin-latte.toml holds: a highlight group's slot is mapped to a palette
-- role by a table written down once, in installer/internal/tui/installer.go
-- (themeNvimGroups) and in themes/README.md, so no value here was chosen by eye.
-- The switch selects this file through the colorscheme line it generates in
-- lua/plugins/colorscheme.lua, and the file is found because ~/.config/nvim is on
-- Neovim's runtimepath ahead of any plugin.
--
-- The background is the theme's own: its base role, the colour the terminals in
-- this repository paint behind everything, and "light" because that base
-- is lighter than mid grey.

vim.cmd("highlight clear")
if vim.fn.exists("syntax_on") == 1 then
  vim.cmd("syntax reset")
end

vim.o.termguicolors = true
vim.o.background = "light"
vim.g.colors_name = "catppuccin-latte"

local set = function(group, opts)
  vim.api.nvim_set_hl(0, group, opts)
end

set("Normal", { fg = "#4c4f69", bg = "#eff1f5" })
set("NormalNC", { fg = "#4c4f69", bg = "#eff1f5" })
set("NormalFloat", { fg = "#4c4f69", bg = "#eff1f5" })
set("FloatBorder", { fg = "#6c6f85", bg = "#eff1f5" })
set("FloatTitle", { fg = "#1e66f5", bg = "#eff1f5", bold = true })
set("MsgArea", { fg = "#4c4f69", bg = "#eff1f5" })
set("Cursor", { fg = "#eff1f5", bg = "#dc8a78" })
set("lCursor", { fg = "#eff1f5", bg = "#dc8a78" })
set("TermCursor", { fg = "#eff1f5", bg = "#dc8a78" })
set("CursorLine", { bg = "#ccd0da" })
set("CursorColumn", { bg = "#ccd0da" })
set("ColorColumn", { bg = "#ccd0da" })
set("CursorLineNr", { fg = "#df8e1d", bold = true })
set("LineNr", { fg = "#6c6f85" })
set("SignColumn", { fg = "#6c6f85" })
set("FoldColumn", { fg = "#6c6f85" })
set("Folded", { fg = "#6c6f85", bg = "#ccd0da" })
set("NonText", { fg = "#6c6f85" })
set("SpecialKey", { fg = "#6c6f85" })
set("Whitespace", { fg = "#6c6f85" })
set("EndOfBuffer", { fg = "#eff1f5" })
set("WinSeparator", { fg = "#6c6f85", bg = "#eff1f5" })
set("Visual", { bg = "#ccd0da" })
set("VisualNOS", { bg = "#ccd0da" })
set("Search", { fg = "#eff1f5", bg = "#df8e1d" })
set("IncSearch", { fg = "#eff1f5", bg = "#40a02b" })
set("CurSearch", { fg = "#eff1f5", bg = "#40a02b" })
set("MatchParen", { fg = "#179299", bg = "#ccd0da", bold = true })
set("Pmenu", { fg = "#4c4f69", bg = "#ccd0da" })
set("PmenuSel", { fg = "#eff1f5", bg = "#1e66f5", bold = true })
set("PmenuSbar", { bg = "#ccd0da" })
set("PmenuThumb", { bg = "#6c6f85" })
set("StatusLine", { fg = "#4c4f69", bg = "#ccd0da" })
set("StatusLineNC", { fg = "#6c6f85", bg = "#eff1f5" })
set("TabLine", { fg = "#6c6f85", bg = "#eff1f5" })
set("TabLineSel", { fg = "#4c4f69", bg = "#ccd0da", bold = true })
set("TabLineFill", { bg = "#eff1f5" })
set("Title", { fg = "#1e66f5", bold = true })
set("Directory", { fg = "#1e66f5" })
set("ErrorMsg", { fg = "#d20f39", bg = "#eff1f5" })
set("WarningMsg", { fg = "#df8e1d" })
set("MoreMsg", { fg = "#40a02b" })
set("ModeMsg", { fg = "#4c4f69", bold = true })
set("Question", { fg = "#40a02b" })
set("WildMenu", { fg = "#eff1f5", bg = "#1e66f5", bold = true })
set("QuickFixLine", { bg = "#ccd0da" })
set("Comment", { fg = "#6c6f85", italic = true })
set("SpecialComment", { fg = "#6c6f85", italic = true })
set("Constant", { fg = "#ea76cb" })
set("String", { fg = "#df8e1d" })
set("Character", { fg = "#df8e1d" })
set("Number", { fg = "#ea76cb" })
set("Boolean", { fg = "#ea76cb" })
set("Float", { fg = "#ea76cb" })
set("Identifier", { fg = "#4c4f69" })
set("Function", { fg = "#40a02b" })
set("Statement", { fg = "#1e66f5" })
set("Conditional", { fg = "#1e66f5" })
set("Repeat", { fg = "#1e66f5" })
set("Label", { fg = "#1e66f5" })
set("Operator", { fg = "#179299" })
set("Keyword", { fg = "#1e66f5" })
set("Exception", { fg = "#d20f39" })
set("PreProc", { fg = "#ea76cb" })
set("Include", { fg = "#ea76cb" })
set("Define", { fg = "#ea76cb" })
set("Macro", { fg = "#ea76cb" })
set("PreCondit", { fg = "#ea76cb" })
set("Type", { fg = "#1e66f5" })
set("StorageClass", { fg = "#1e66f5" })
set("Structure", { fg = "#1e66f5" })
set("Typedef", { fg = "#1e66f5" })
set("Special", { fg = "#179299" })
set("SpecialChar", { fg = "#179299" })
set("Tag", { fg = "#40a02b" })
set("Delimiter", { fg = "#179299" })
set("Debug", { fg = "#d20f39" })
set("Underlined", { fg = "#1e66f5", underline = true })
set("Ignore", { fg = "#6c6f85" })
set("Error", { fg = "#d20f39", bg = "#eff1f5" })
set("Todo", { fg = "#eff1f5", bg = "#df8e1d", bold = true })
set("DiffAdd", { fg = "#40a02b", bg = "#eff1f5" })
set("DiffChange", { fg = "#df8e1d", bg = "#eff1f5" })
set("DiffDelete", { fg = "#d20f39", bg = "#eff1f5" })
set("DiffText", { fg = "#1e66f5", bg = "#eff1f5", bold = true })
set("Added", { fg = "#40a02b" })
set("Changed", { fg = "#df8e1d" })
set("Removed", { fg = "#d20f39" })
set("DiagnosticError", { fg = "#d20f39" })
set("DiagnosticWarn", { fg = "#df8e1d" })
set("DiagnosticInfo", { fg = "#179299" })
set("DiagnosticHint", { fg = "#6c6f85" })
set("DiagnosticOk", { fg = "#40a02b" })
set("LspReferenceText", { bg = "#ccd0da" })
set("LspReferenceRead", { bg = "#ccd0da" })
set("LspReferenceWrite", { bg = "#ccd0da" })

-- The terminal's own sixteen colours, so a :terminal inside Neovim paints the
-- same palette the terminal around it does.
vim.g.terminal_color_0 = "#5c5f77"
vim.g.terminal_color_1 = "#d20f39"
vim.g.terminal_color_2 = "#40a02b"
vim.g.terminal_color_3 = "#df8e1d"
vim.g.terminal_color_4 = "#1e66f5"
vim.g.terminal_color_5 = "#ea76cb"
vim.g.terminal_color_6 = "#179299"
vim.g.terminal_color_7 = "#acb0be"
vim.g.terminal_color_8 = "#6c6f85"
vim.g.terminal_color_9 = "#d20f39"
vim.g.terminal_color_10 = "#40a02b"
vim.g.terminal_color_11 = "#df8e1d"
vim.g.terminal_color_12 = "#1e66f5"
vim.g.terminal_color_13 = "#ea76cb"
vim.g.terminal_color_14 = "#179299"
vim.g.terminal_color_15 = "#bcc0cc"
