-- dotfiles-managed-config: nvim
-- name: nocturne
-- generated from themes/nocturne.toml; edit the definition, not this file
--
-- The Neovim colorscheme for the Nocturne palette. Every colour below is a
-- role themes/nocturne.toml holds: a highlight group's slot is mapped to a palette
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
vim.g.colors_name = "nocturne"

local set = function(group, opts)
  vim.api.nvim_set_hl(0, group, opts)
end

set("Normal", { fg = "#c0c0c0", bg = "#151316" })
set("NormalNC", { fg = "#c0c0c0", bg = "#151316" })
set("NormalFloat", { fg = "#c0c0c0", bg = "#151316" })
set("FloatBorder", { fg = "#a0a0a0", bg = "#151316" })
set("FloatTitle", { fg = "#8686cb", bg = "#151316", bold = true })
set("MsgArea", { fg = "#c0c0c0", bg = "#151316" })
set("Cursor", { fg = "#151316", bg = "#e0c070" })
set("lCursor", { fg = "#151316", bg = "#e0c070" })
set("TermCursor", { fg = "#151316", bg = "#e0c070" })
set("CursorLine", { bg = "#202030" })
set("CursorColumn", { bg = "#202030" })
set("ColorColumn", { bg = "#202030" })
set("CursorLineNr", { fg = "#e0c070", bold = true })
set("LineNr", { fg = "#a0a0a0" })
set("SignColumn", { fg = "#a0a0a0" })
set("FoldColumn", { fg = "#a0a0a0" })
set("Folded", { fg = "#a0a0a0", bg = "#202030" })
set("NonText", { fg = "#a0a0a0" })
set("SpecialKey", { fg = "#a0a0a0" })
set("Whitespace", { fg = "#a0a0a0" })
set("EndOfBuffer", { fg = "#151316" })
set("WinSeparator", { fg = "#a0a0a0", bg = "#151316" })
set("Visual", { bg = "#202030" })
set("VisualNOS", { bg = "#202030" })
set("Search", { fg = "#151316", bg = "#e0c070" })
set("IncSearch", { fg = "#151316", bg = "#86cb86" })
set("CurSearch", { fg = "#151316", bg = "#86cb86" })
set("MatchParen", { fg = "#86cbcb", bg = "#202030", bold = true })
set("Pmenu", { fg = "#c0c0c0", bg = "#202030" })
set("PmenuSel", { fg = "#151316", bg = "#8686cb", bold = true })
set("PmenuSbar", { bg = "#202030" })
set("PmenuThumb", { bg = "#a0a0a0" })
set("StatusLine", { fg = "#c0c0c0", bg = "#202030" })
set("StatusLineNC", { fg = "#a0a0a0", bg = "#151316" })
set("TabLine", { fg = "#a0a0a0", bg = "#151316" })
set("TabLineSel", { fg = "#c0c0c0", bg = "#202030", bold = true })
set("TabLineFill", { bg = "#151316" })
set("Title", { fg = "#8686cb", bold = true })
set("Directory", { fg = "#8686cb" })
set("ErrorMsg", { fg = "#cb8686", bg = "#151316" })
set("WarningMsg", { fg = "#e0c070" })
set("MoreMsg", { fg = "#86cb86" })
set("ModeMsg", { fg = "#c0c0c0", bold = true })
set("Question", { fg = "#86cb86" })
set("WildMenu", { fg = "#151316", bg = "#8686cb", bold = true })
set("QuickFixLine", { bg = "#202030" })
set("Comment", { fg = "#a0a0a0", italic = true })
set("SpecialComment", { fg = "#a0a0a0", italic = true })
set("Constant", { fg = "#a08090" })
set("String", { fg = "#e0c070" })
set("Character", { fg = "#e0c070" })
set("Number", { fg = "#a08090" })
set("Boolean", { fg = "#a08090" })
set("Float", { fg = "#a08090" })
set("Identifier", { fg = "#c0c0c0" })
set("Function", { fg = "#86cb86" })
set("Statement", { fg = "#8686cb" })
set("Conditional", { fg = "#8686cb" })
set("Repeat", { fg = "#8686cb" })
set("Label", { fg = "#8686cb" })
set("Operator", { fg = "#86cbcb" })
set("Keyword", { fg = "#8686cb" })
set("Exception", { fg = "#cb8686" })
set("PreProc", { fg = "#a08090" })
set("Include", { fg = "#a08090" })
set("Define", { fg = "#a08090" })
set("Macro", { fg = "#a08090" })
set("PreCondit", { fg = "#a08090" })
set("Type", { fg = "#8686cb" })
set("StorageClass", { fg = "#8686cb" })
set("Structure", { fg = "#8686cb" })
set("Typedef", { fg = "#8686cb" })
set("Special", { fg = "#86cbcb" })
set("SpecialChar", { fg = "#86cbcb" })
set("Tag", { fg = "#86cb86" })
set("Delimiter", { fg = "#86cbcb" })
set("Debug", { fg = "#cb8686" })
set("Underlined", { fg = "#8686cb", underline = true })
set("Ignore", { fg = "#a0a0a0" })
set("Error", { fg = "#cb8686", bg = "#151316" })
set("Todo", { fg = "#151316", bg = "#e0c070", bold = true })
set("DiffAdd", { fg = "#86cb86", bg = "#151316" })
set("DiffChange", { fg = "#e0c070", bg = "#151316" })
set("DiffDelete", { fg = "#cb8686", bg = "#151316" })
set("DiffText", { fg = "#8686cb", bg = "#151316", bold = true })
set("Added", { fg = "#86cb86" })
set("Changed", { fg = "#e0c070" })
set("Removed", { fg = "#cb8686" })
set("DiagnosticError", { fg = "#cb8686" })
set("DiagnosticWarn", { fg = "#e0c070" })
set("DiagnosticInfo", { fg = "#86cbcb" })
set("DiagnosticHint", { fg = "#a0a0a0" })
set("DiagnosticOk", { fg = "#86cb86" })
set("LspReferenceText", { bg = "#202030" })
set("LspReferenceRead", { bg = "#202030" })
set("LspReferenceWrite", { bg = "#202030" })

-- The terminal's own sixteen colours, so a :terminal inside Neovim paints the
-- same palette the terminal around it does.
vim.g.terminal_color_0 = "#100a0f"
vim.g.terminal_color_1 = "#cb8686"
vim.g.terminal_color_2 = "#86cb86"
vim.g.terminal_color_3 = "#e0c070"
vim.g.terminal_color_4 = "#8686cb"
vim.g.terminal_color_5 = "#a08090"
vim.g.terminal_color_6 = "#86cbcb"
vim.g.terminal_color_7 = "#c0c0c0"
vim.g.terminal_color_8 = "#a0a0a0"
vim.g.terminal_color_9 = "#daa9a9"
vim.g.terminal_color_10 = "#a9daa9"
vim.g.terminal_color_11 = "#e9d29a"
vim.g.terminal_color_12 = "#a9a9da"
vim.g.terminal_color_13 = "#b69daa"
vim.g.terminal_color_14 = "#a9dada"
vim.g.terminal_color_15 = "#f0f0f0"
