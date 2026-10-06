-- dotfiles-managed-config: nvim
-- name: dotfiles
-- generated from themes/dotfiles.toml; edit the definition, not this file
--
-- The Neovim colorscheme for the dotfiles palette. Every colour below is a
-- role themes/dotfiles.toml holds: a highlight group's slot is mapped to a palette
-- role by a table written down once, in installer/internal/tui/installer.go
-- (themeNvimGroups) and in themes/README.md, so no value here was chosen by eye.
-- The switch selects this file through the colorscheme line it generates in
-- lua/plugins/colorscheme.lua, and the file is found because ~/.config/nvim is on
-- Neovim's runtimepath ahead of any plugin.
--
-- The background is the theme's own: its base role, the colour the terminals in
-- this repository paint behind everything, and "dark" because that base
-- is darker than mid grey.

vim.cmd("highlight clear")
if vim.fn.exists("syntax_on") == 1 then
  vim.cmd("syntax reset")
end

vim.o.termguicolors = true
vim.o.background = "dark"
vim.g.colors_name = "dotfiles"

local set = function(group, opts)
  vim.api.nvim_set_hl(0, group, opts)
end

set("Normal", { fg = "#f3f6f9", bg = "#06080f" })
set("NormalNC", { fg = "#f3f6f9", bg = "#06080f" })
set("NormalFloat", { fg = "#f3f6f9", bg = "#06080f" })
set("FloatBorder", { fg = "#8a8fa3", bg = "#06080f" })
set("FloatTitle", { fg = "#7fb4ca", bg = "#06080f", bold = true })
set("MsgArea", { fg = "#f3f6f9", bg = "#06080f" })
set("Cursor", { fg = "#06080f", bg = "#e0c15a" })
set("lCursor", { fg = "#06080f", bg = "#e0c15a" })
set("TermCursor", { fg = "#06080f", bg = "#e0c15a" })
set("CursorLine", { bg = "#263356" })
set("CursorColumn", { bg = "#263356" })
set("ColorColumn", { bg = "#263356" })
set("CursorLineNr", { fg = "#ffe066", bold = true })
set("LineNr", { fg = "#8a8fa3" })
set("SignColumn", { fg = "#8a8fa3" })
set("FoldColumn", { fg = "#8a8fa3" })
set("Folded", { fg = "#8a8fa3", bg = "#263356" })
set("NonText", { fg = "#8a8fa3" })
set("SpecialKey", { fg = "#8a8fa3" })
set("Whitespace", { fg = "#8a8fa3" })
set("EndOfBuffer", { fg = "#06080f" })
set("WinSeparator", { fg = "#8a8fa3", bg = "#06080f" })
set("Visual", { bg = "#263356" })
set("VisualNOS", { bg = "#263356" })
set("Search", { fg = "#06080f", bg = "#ffe066" })
set("IncSearch", { fg = "#06080f", bg = "#b7cc85" })
set("CurSearch", { fg = "#06080f", bg = "#b7cc85" })
set("MatchParen", { fg = "#7aa89f", bg = "#263356", bold = true })
set("Pmenu", { fg = "#f3f6f9", bg = "#263356" })
set("PmenuSel", { fg = "#06080f", bg = "#7fb4ca", bold = true })
set("PmenuSbar", { bg = "#263356" })
set("PmenuThumb", { bg = "#8a8fa3" })
set("StatusLine", { fg = "#f3f6f9", bg = "#263356" })
set("StatusLineNC", { fg = "#8a8fa3", bg = "#06080f" })
set("TabLine", { fg = "#8a8fa3", bg = "#06080f" })
set("TabLineSel", { fg = "#f3f6f9", bg = "#263356", bold = true })
set("TabLineFill", { bg = "#06080f" })
set("Title", { fg = "#7fb4ca", bold = true })
set("Directory", { fg = "#7fb4ca" })
set("ErrorMsg", { fg = "#cb7c94", bg = "#06080f" })
set("WarningMsg", { fg = "#ffe066" })
set("MoreMsg", { fg = "#b7cc85" })
set("ModeMsg", { fg = "#f3f6f9", bold = true })
set("Question", { fg = "#b7cc85" })
set("WildMenu", { fg = "#06080f", bg = "#7fb4ca", bold = true })
set("QuickFixLine", { bg = "#263356" })
set("Comment", { fg = "#8a8fa3", italic = true })
set("SpecialComment", { fg = "#8a8fa3", italic = true })
set("Constant", { fg = "#ff8dd7" })
set("String", { fg = "#ffe066" })
set("Character", { fg = "#ffe066" })
set("Number", { fg = "#ff8dd7" })
set("Boolean", { fg = "#ff8dd7" })
set("Float", { fg = "#ff8dd7" })
set("Identifier", { fg = "#f3f6f9" })
set("Function", { fg = "#b7cc85" })
set("Statement", { fg = "#7fb4ca" })
set("Conditional", { fg = "#7fb4ca" })
set("Repeat", { fg = "#7fb4ca" })
set("Label", { fg = "#7fb4ca" })
set("Operator", { fg = "#7aa89f" })
set("Keyword", { fg = "#7fb4ca" })
set("Exception", { fg = "#cb7c94" })
set("PreProc", { fg = "#ff8dd7" })
set("Include", { fg = "#ff8dd7" })
set("Define", { fg = "#ff8dd7" })
set("Macro", { fg = "#ff8dd7" })
set("PreCondit", { fg = "#ff8dd7" })
set("Type", { fg = "#7fb4ca" })
set("StorageClass", { fg = "#7fb4ca" })
set("Structure", { fg = "#7fb4ca" })
set("Typedef", { fg = "#7fb4ca" })
set("Special", { fg = "#7aa89f" })
set("SpecialChar", { fg = "#7aa89f" })
set("Tag", { fg = "#b7cc85" })
set("Delimiter", { fg = "#7aa89f" })
set("Debug", { fg = "#cb7c94" })
set("Underlined", { fg = "#7fb4ca", underline = true })
set("Ignore", { fg = "#8a8fa3" })
set("Error", { fg = "#cb7c94", bg = "#06080f" })
set("Todo", { fg = "#06080f", bg = "#ffe066", bold = true })
set("DiffAdd", { fg = "#b7cc85", bg = "#06080f" })
set("DiffChange", { fg = "#ffe066", bg = "#06080f" })
set("DiffDelete", { fg = "#cb7c94", bg = "#06080f" })
set("DiffText", { fg = "#7fb4ca", bg = "#06080f", bold = true })
set("Added", { fg = "#b7cc85" })
set("Changed", { fg = "#ffe066" })
set("Removed", { fg = "#cb7c94" })
set("DiagnosticError", { fg = "#cb7c94" })
set("DiagnosticWarn", { fg = "#ffe066" })
set("DiagnosticInfo", { fg = "#7aa89f" })
set("DiagnosticHint", { fg = "#8a8fa3" })
set("DiagnosticOk", { fg = "#b7cc85" })
set("LspReferenceText", { bg = "#263356" })
set("LspReferenceRead", { bg = "#263356" })
set("LspReferenceWrite", { bg = "#263356" })

-- The terminal's own sixteen colours, so a :terminal inside Neovim paints the
-- same palette the terminal around it does.
vim.g.terminal_color_0 = "#06080f"
vim.g.terminal_color_1 = "#cb7c94"
vim.g.terminal_color_2 = "#b7cc85"
vim.g.terminal_color_3 = "#ffe066"
vim.g.terminal_color_4 = "#7fb4ca"
vim.g.terminal_color_5 = "#ff8dd7"
vim.g.terminal_color_6 = "#7aa89f"
vim.g.terminal_color_7 = "#f3f6f9"
vim.g.terminal_color_8 = "#8a8fa3"
vim.g.terminal_color_9 = "#de8fa8"
vim.g.terminal_color_10 = "#d1e8a9"
vim.g.terminal_color_11 = "#fff7b1"
vim.g.terminal_color_12 = "#a3d4d5"
vim.g.terminal_color_13 = "#ffaeea"
vim.g.terminal_color_14 = "#7fb4ca"
vim.g.terminal_color_15 = "#f3f6f9"
