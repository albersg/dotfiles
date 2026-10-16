package tui

// alacrittyConfigTable is the Alacritty answer to the question the keymap
// reference asks, and the answer is that this configuration binds nothing:
// alacritty.toml has no [keyboard] section, so the bindings a reader gets are
// Alacritty's own defaults. What the file does set is listed instead, under the
// sections it already carries.
//
// The guard beside this file fails if the file ever declares a binding
// (TestAlacrittyKeymapPageDeclaresWhatTheConfigurationDoes), because then this
// page would be answering the question wrongly.
func alacrittyConfigTable() referenceTable {
	return referenceTable{
		Name:        "Alacritty",
		Description: "This configuration declares no key bindings: you get Alacritty's defaults. It sets these instead",
		Headers:     [3]string{"Setting", "Section", "What this configuration sets"},
		Rows: []referenceRow{
			{"family", "Font", "MesloLGS Nerd Font"},
			{"size", "Font", "14"},
			{"opacity", "Window", "0.95, blurred"},
			{"option_as_alt", "Window", "Both, so the Option key works as Alt"},
			{"style", "Cursor", "Block, and still a Block in vi mode"},
			{"blink_interval", "Cursor", "500ms"},
			{"unfocused_hollow", "Cursor", "on: the cursor hollows out when the window loses focus"},
			{"TERM", "Terminal", "xterm-256color"},
			{"enabled", "Hints", "empty, so no hint keybinding is defined"},
		},
	}
}
