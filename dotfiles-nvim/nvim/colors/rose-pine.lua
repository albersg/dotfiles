-- dotfiles-managed-config: nvim
-- name: rose-pine
-- generated from themes/rose-pine.toml; edit the definition, not this file
--
-- The Neovim colorscheme for the Rosé Pine palette. Every colour below is a
-- role themes/rose-pine.toml holds: a highlight group's slot is mapped to a palette
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
vim.g.colors_name = "rose-pine"

local set = function(group, opts)
  vim.api.nvim_set_hl(0, group, opts)
end

set("Normal", { fg = "#e0def4", bg = "#191724" })
set("NormalNC", { fg = "#e0def4", bg = "#191724" })
set("NormalFloat", { fg = "#e0def4", bg = "#191724" })
set("FloatBorder", { fg = "#6e6a86", bg = "#191724" })
set("FloatTitle", { fg = "#9ccfd8", bg = "#191724", bold = true })
set("MsgArea", { fg = "#e0def4", bg = "#191724" })
set("Cursor", { fg = "#191724", bg = "#e0def4" })
set("lCursor", { fg = "#191724", bg = "#e0def4" })
set("TermCursor", { fg = "#191724", bg = "#e0def4" })
set("CursorLine", { bg = "#403d52" })
set("CursorColumn", { bg = "#403d52" })
set("ColorColumn", { bg = "#403d52" })
set("CursorLineNr", { fg = "#f6c177", bold = true })
set("LineNr", { fg = "#6e6a86" })
set("SignColumn", { fg = "#6e6a86" })
set("FoldColumn", { fg = "#6e6a86" })
set("Folded", { fg = "#6e6a86", bg = "#403d52" })
set("NonText", { fg = "#6e6a86" })
set("SpecialKey", { fg = "#6e6a86" })
set("Whitespace", { fg = "#6e6a86" })
set("EndOfBuffer", { fg = "#191724" })
set("WinSeparator", { fg = "#6e6a86", bg = "#191724" })
set("Visual", { bg = "#403d52" })
set("VisualNOS", { bg = "#403d52" })
set("Search", { fg = "#191724", bg = "#f6c177" })
set("IncSearch", { fg = "#191724", bg = "#31748f" })
set("CurSearch", { fg = "#191724", bg = "#31748f" })
set("MatchParen", { fg = "#ebbcba", bg = "#403d52", bold = true })
set("Pmenu", { fg = "#e0def4", bg = "#403d52" })
set("PmenuSel", { fg = "#191724", bg = "#9ccfd8", bold = true })
set("PmenuSbar", { bg = "#403d52" })
set("PmenuThumb", { bg = "#6e6a86" })
set("StatusLine", { fg = "#e0def4", bg = "#403d52" })
set("StatusLineNC", { fg = "#6e6a86", bg = "#191724" })
set("TabLine", { fg = "#6e6a86", bg = "#191724" })
set("TabLineSel", { fg = "#e0def4", bg = "#403d52", bold = true })
set("TabLineFill", { bg = "#191724" })
set("Title", { fg = "#9ccfd8", bold = true })
set("Directory", { fg = "#9ccfd8" })
set("ErrorMsg", { fg = "#eb6f92", bg = "#191724" })
set("WarningMsg", { fg = "#f6c177" })
set("MoreMsg", { fg = "#31748f" })
set("ModeMsg", { fg = "#e0def4", bold = true })
set("Question", { fg = "#31748f" })
set("WildMenu", { fg = "#191724", bg = "#9ccfd8", bold = true })
set("QuickFixLine", { bg = "#403d52" })
set("Comment", { fg = "#6e6a86", italic = true })
set("SpecialComment", { fg = "#6e6a86", italic = true })
set("Constant", { fg = "#c4a7e7" })
set("String", { fg = "#f6c177" })
set("Character", { fg = "#f6c177" })
set("Number", { fg = "#c4a7e7" })
set("Boolean", { fg = "#c4a7e7" })
set("Float", { fg = "#c4a7e7" })
set("Identifier", { fg = "#e0def4" })
set("Function", { fg = "#31748f" })
set("Statement", { fg = "#9ccfd8" })
set("Conditional", { fg = "#9ccfd8" })
set("Repeat", { fg = "#9ccfd8" })
set("Label", { fg = "#9ccfd8" })
set("Operator", { fg = "#ebbcba" })
set("Keyword", { fg = "#9ccfd8" })
set("Exception", { fg = "#eb6f92" })
set("PreProc", { fg = "#c4a7e7" })
set("Include", { fg = "#c4a7e7" })
set("Define", { fg = "#c4a7e7" })
set("Macro", { fg = "#c4a7e7" })
set("PreCondit", { fg = "#c4a7e7" })
set("Type", { fg = "#9ccfd8" })
set("StorageClass", { fg = "#9ccfd8" })
set("Structure", { fg = "#9ccfd8" })
set("Typedef", { fg = "#9ccfd8" })
set("Special", { fg = "#ebbcba" })
set("SpecialChar", { fg = "#ebbcba" })
set("Tag", { fg = "#31748f" })
set("Delimiter", { fg = "#ebbcba" })
set("Debug", { fg = "#eb6f92" })
set("Underlined", { fg = "#9ccfd8", underline = true })
set("Ignore", { fg = "#6e6a86" })
set("Error", { fg = "#eb6f92", bg = "#191724" })
set("Todo", { fg = "#191724", bg = "#f6c177", bold = true })
set("DiffAdd", { fg = "#31748f", bg = "#191724" })
set("DiffChange", { fg = "#f6c177", bg = "#191724" })
set("DiffDelete", { fg = "#eb6f92", bg = "#191724" })
set("DiffText", { fg = "#9ccfd8", bg = "#191724", bold = true })
set("Added", { fg = "#31748f" })
set("Changed", { fg = "#f6c177" })
set("Removed", { fg = "#eb6f92" })
set("DiagnosticError", { fg = "#eb6f92" })
set("DiagnosticWarn", { fg = "#f6c177" })
set("DiagnosticInfo", { fg = "#ebbcba" })
set("DiagnosticHint", { fg = "#6e6a86" })
set("DiagnosticOk", { fg = "#31748f" })
set("LspReferenceText", { bg = "#403d52" })
set("LspReferenceRead", { bg = "#403d52" })
set("LspReferenceWrite", { bg = "#403d52" })

-- The terminal's own sixteen colours, so a :terminal inside Neovim paints the
-- same palette the terminal around it does.
vim.g.terminal_color_0 = "#26233a"
vim.g.terminal_color_1 = "#eb6f92"
vim.g.terminal_color_2 = "#31748f"
vim.g.terminal_color_3 = "#f6c177"
vim.g.terminal_color_4 = "#9ccfd8"
vim.g.terminal_color_5 = "#c4a7e7"
vim.g.terminal_color_6 = "#ebbcba"
vim.g.terminal_color_7 = "#e0def4"
vim.g.terminal_color_8 = "#6e6a86"
vim.g.terminal_color_9 = "#eb6f92"
vim.g.terminal_color_10 = "#31748f"
vim.g.terminal_color_11 = "#f6c177"
vim.g.terminal_color_12 = "#9ccfd8"
vim.g.terminal_color_13 = "#c4a7e7"
vim.g.terminal_color_14 = "#ebbcba"
vim.g.terminal_color_15 = "#e0def4"
