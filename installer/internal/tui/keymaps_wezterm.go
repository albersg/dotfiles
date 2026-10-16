package tui

// weztermConfigTable is the WezTerm answer to the question the keymap reference
// asks, and the answer is that this configuration binds nothing: .wezterm.lua
// assigns no config.keys, so the bindings a reader gets are WezTerm's own
// defaults. What the file does set is listed instead, under the section banners
// it already carries.
//
// The guard beside this file fails if the file ever declares a binding
// (TestWezTermKeymapPageDeclaresWhatTheConfigurationDoes), because then this page
// would be answering the question wrongly.
func weztermConfigTable() referenceTable {
	return referenceTable{
		Name:        "WezTerm",
		Description: "This configuration declares no key bindings: you get WezTerm's defaults. It sets these instead",
		Headers:     [3]string{"Setting", "Section", "What this configuration sets"},
		Rows: []referenceRow{
			{"font", "Font", "MesloLGS Nerd Font"},
			{"font_size", "Font", "14"},
			{"window_background_opacity", "Window", "0.95, with 20px of blur on macOS and an Acrylic backdrop on Windows"},
			{"window_padding", "Window", "none, so the text starts at the edge"},
			{"enable_scroll_bar", "Window", "off; the tab bar hides itself while a single tab is open"},
			{"default_cursor_style", "Cursor", "SteadyBlock, blinking every 500ms with a constant ramp"},
			{"term", "Neovim", "wezterm, or xterm-256color under WSL, which has no wezterm terminfo"},
			{"enable_csi_u_key_encoding", "Neovim", "on, so Neovim can tell Ctrl+I from Tab and Ctrl+M from Enter"},
			{"underline_thickness", "Neovim", "2px at -2 offset, for LSP diagnostics and spelling undercurl"},
			{"scrollback_lines", "Neovim", "10000 lines of history"},
			{"max_fps", "Neovim", "240"},
			{"enable_kitty_graphics", "Neovim", "on, for inline images"},
			{"use_dead_keys", "Input", "off; a dead key is sent through instead of being composed"},
		},
	}
}
