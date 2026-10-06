-- dotfiles-managed-config: nvim
-- name: everforest
-- generated from themes/everforest.toml; edit the definition, not this file
--
-- The Neovim colorscheme for the Everforest palette. Every colour below is a
-- role themes/everforest.toml holds: a highlight group's slot is mapped to a palette
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
vim.g.colors_name = "everforest"

local set = function(group, opts)
  vim.api.nvim_set_hl(0, group, opts)
end

set("Normal", { fg = "#d3c6aa", bg = "#2d353b" })
set("NormalNC", { fg = "#d3c6aa", bg = "#2d353b" })
set("NormalFloat", { fg = "#d3c6aa", bg = "#2d353b" })
set("FloatBorder", { fg = "#7a8478", bg = "#2d353b" })
set("FloatTitle", { fg = "#7fbbb3", bg = "#2d353b", bold = true })
set("MsgArea", { fg = "#d3c6aa", bg = "#2d353b" })
set("Cursor", { fg = "#2d353b", bg = "#d3c6aa" })
set("lCursor", { fg = "#2d353b", bg = "#d3c6aa" })
set("TermCursor", { fg = "#2d353b", bg = "#d3c6aa" })
set("CursorLine", { bg = "#475258" })
set("CursorColumn", { bg = "#475258" })
set("ColorColumn", { bg = "#475258" })
set("CursorLineNr", { fg = "#dbbc7f", bold = true })
set("LineNr", { fg = "#7a8478" })
set("SignColumn", { fg = "#7a8478" })
set("FoldColumn", { fg = "#7a8478" })
set("Folded", { fg = "#7a8478", bg = "#475258" })
set("NonText", { fg = "#7a8478" })
set("SpecialKey", { fg = "#7a8478" })
set("Whitespace", { fg = "#7a8478" })
set("EndOfBuffer", { fg = "#2d353b" })
set("WinSeparator", { fg = "#7a8478", bg = "#2d353b" })
set("Visual", { bg = "#475258" })
set("VisualNOS", { bg = "#475258" })
set("Search", { fg = "#2d353b", bg = "#dbbc7f" })
set("IncSearch", { fg = "#2d353b", bg = "#a7c080" })
set("CurSearch", { fg = "#2d353b", bg = "#a7c080" })
set("MatchParen", { fg = "#83c092", bg = "#475258", bold = true })
set("Pmenu", { fg = "#d3c6aa", bg = "#475258" })
set("PmenuSel", { fg = "#2d353b", bg = "#7fbbb3", bold = true })
set("PmenuSbar", { bg = "#475258" })
set("PmenuThumb", { bg = "#7a8478" })
set("StatusLine", { fg = "#d3c6aa", bg = "#475258" })
set("StatusLineNC", { fg = "#7a8478", bg = "#2d353b" })
set("TabLine", { fg = "#7a8478", bg = "#2d353b" })
set("TabLineSel", { fg = "#d3c6aa", bg = "#475258", bold = true })
set("TabLineFill", { bg = "#2d353b" })
set("Title", { fg = "#7fbbb3", bold = true })
set("Directory", { fg = "#7fbbb3" })
set("ErrorMsg", { fg = "#e67e80", bg = "#2d353b" })
set("WarningMsg", { fg = "#dbbc7f" })
set("MoreMsg", { fg = "#a7c080" })
set("ModeMsg", { fg = "#d3c6aa", bold = true })
set("Question", { fg = "#a7c080" })
set("WildMenu", { fg = "#2d353b", bg = "#7fbbb3", bold = true })
set("QuickFixLine", { bg = "#475258" })
set("Comment", { fg = "#7a8478", italic = true })
set("SpecialComment", { fg = "#7a8478", italic = true })
set("Constant", { fg = "#d699b6" })
set("String", { fg = "#dbbc7f" })
set("Character", { fg = "#dbbc7f" })
set("Number", { fg = "#d699b6" })
set("Boolean", { fg = "#d699b6" })
set("Float", { fg = "#d699b6" })
set("Identifier", { fg = "#d3c6aa" })
set("Function", { fg = "#a7c080" })
set("Statement", { fg = "#7fbbb3" })
set("Conditional", { fg = "#7fbbb3" })
set("Repeat", { fg = "#7fbbb3" })
set("Label", { fg = "#7fbbb3" })
set("Operator", { fg = "#83c092" })
set("Keyword", { fg = "#7fbbb3" })
set("Exception", { fg = "#e67e80" })
set("PreProc", { fg = "#d699b6" })
set("Include", { fg = "#d699b6" })
set("Define", { fg = "#d699b6" })
set("Macro", { fg = "#d699b6" })
set("PreCondit", { fg = "#d699b6" })
set("Type", { fg = "#7fbbb3" })
set("StorageClass", { fg = "#7fbbb3" })
set("Structure", { fg = "#7fbbb3" })
set("Typedef", { fg = "#7fbbb3" })
set("Special", { fg = "#83c092" })
set("SpecialChar", { fg = "#83c092" })
set("Tag", { fg = "#a7c080" })
set("Delimiter", { fg = "#83c092" })
set("Debug", { fg = "#e67e80" })
set("Underlined", { fg = "#7fbbb3", underline = true })
set("Ignore", { fg = "#7a8478" })
set("Error", { fg = "#e67e80", bg = "#2d353b" })
set("Todo", { fg = "#2d353b", bg = "#dbbc7f", bold = true })
set("DiffAdd", { fg = "#a7c080", bg = "#2d353b" })
set("DiffChange", { fg = "#dbbc7f", bg = "#2d353b" })
set("DiffDelete", { fg = "#e67e80", bg = "#2d353b" })
set("DiffText", { fg = "#7fbbb3", bg = "#2d353b", bold = true })
set("Added", { fg = "#a7c080" })
set("Changed", { fg = "#dbbc7f" })
set("Removed", { fg = "#e67e80" })
set("DiagnosticError", { fg = "#e67e80" })
set("DiagnosticWarn", { fg = "#dbbc7f" })
set("DiagnosticInfo", { fg = "#83c092" })
set("DiagnosticHint", { fg = "#7a8478" })
set("DiagnosticOk", { fg = "#a7c080" })
set("LspReferenceText", { bg = "#475258" })
set("LspReferenceRead", { bg = "#475258" })
set("LspReferenceWrite", { bg = "#475258" })

-- The terminal's own sixteen colours, so a :terminal inside Neovim paints the
-- same palette the terminal around it does.
vim.g.terminal_color_0 = "#475258"
vim.g.terminal_color_1 = "#e67e80"
vim.g.terminal_color_2 = "#a7c080"
vim.g.terminal_color_3 = "#dbbc7f"
vim.g.terminal_color_4 = "#7fbbb3"
vim.g.terminal_color_5 = "#d699b6"
vim.g.terminal_color_6 = "#83c092"
vim.g.terminal_color_7 = "#d3c6aa"
vim.g.terminal_color_8 = "#7a8478"
vim.g.terminal_color_9 = "#e67e80"
vim.g.terminal_color_10 = "#a7c080"
vim.g.terminal_color_11 = "#dbbc7f"
vim.g.terminal_color_12 = "#7fbbb3"
vim.g.terminal_color_13 = "#d699b6"
vim.g.terminal_color_14 = "#83c092"
vim.g.terminal_color_15 = "#d3c6aa"
