package tui

// GetKittyKeymaps returns the bindings dotfiles-kitty/kitty.conf declares. Kitty
// ships working defaults for everything else, so the file maps only what the
// dotfiles configuration wants on top of them: the nine tab jumps, and the two
// clipboard bindings that keep the meaning Ctrl+Shift+C and Ctrl+Shift+V have in
// a terminal rather than the one macOS gives them.
//
// The guard beside this file reads the same `map` lines and fails by name when
// the two disagree (TestKittyKeymapsMatchTheConfiguration).
func GetKittyKeymaps() []KeymapCategory {
	return []KeymapCategory{
		{
			Name:        "Tabs",
			Description: "Jump straight to a tab instead of cycling through them",
			Keymaps: []Keymap{
				{Keys: "cmd+1", Description: "Go to tab 1", Mode: ""},
				{Keys: "cmd+2", Description: "Go to tab 2", Mode: ""},
				{Keys: "cmd+3", Description: "Go to tab 3", Mode: ""},
				{Keys: "cmd+4", Description: "Go to tab 4", Mode: ""},
				{Keys: "cmd+5", Description: "Go to tab 5", Mode: ""},
				{Keys: "cmd+6", Description: "Go to tab 6", Mode: ""},
				{Keys: "cmd+7", Description: "Go to tab 7", Mode: ""},
				{Keys: "cmd+8", Description: "Go to tab 8", Mode: ""},
				{Keys: "cmd+9", Description: "Go to tab 9", Mode: ""},
			},
		},
		{
			Name:        "Clipboard",
			Description: "Copy and paste without leaving the keyboard",
			Keymaps: []Keymap{
				{Keys: "ctrl+shift+c", Description: "Copy the selection to the clipboard", Mode: ""},
				{Keys: "ctrl+shift+v", Description: "Paste the clipboard", Mode: ""},
			},
		},
	}
}
