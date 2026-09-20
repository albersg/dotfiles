package tui

// GetHerdrKeymaps returns all Herdr keymaps organized by category.
//
// Herdr is mouse-native: its documentation states that keyboard control is
// optional and that no keybinding is required to use it. To avoid presenting
// the tool as keyboard-only, the mouse/pointer surface is listed first, before
// the optional prefix bindings.
//
// Bindings come from https://herdr.dev/docs/keyboard/ and
// https://herdr.dev/agent-guide.md for version 0.9.1, cross-checked against the
// commented defaults printed by `herdr --default-config` (herdr 0.9.1).
func GetHerdrKeymaps() []KeymapCategory {
	return []KeymapCategory{
		{
			Name:        "Mouse & Pointer (Primary)",
			Description: "Herdr is mouse-native: click, drag and right-click cover the whole UI",
			Keymaps: []Keymap{
				{Keys: "Click", Description: "Focus a pane, tab, workspace or agent", Mode: "mouse"},
				{Keys: "Drag border", Description: "Resize a split pane", Mode: "mouse"},
				{Keys: "Right-click", Description: "Open context menus (pane, tab, workspace)", Mode: "mouse"},
				{Keys: "Drag-select", Description: "Copy text without entering copy mode", Mode: "mouse"},
				{Keys: "Double-click", Description: "Select a word", Mode: "mouse"},
				{Keys: "Scroll wheel", Description: "Scroll the pane scrollback", Mode: "mouse"},
			},
		},
		{
			Name:        "Prefix & Essentials",
			Description: "The prefix is Ctrl+b; the keyboard is optional",
			Keymaps: []Keymap{
				{Keys: "Ctrl+b", Description: "Prefix key (default)", Mode: ""},
				{Keys: "Ctrl+b ?", Description: "Show every active binding (keybind help)", Mode: ""},
				{Keys: "Ctrl+b q", Description: "Detach, leave everything running", Mode: ""},
				{Keys: "Ctrl+b s", Description: "Open settings", Mode: ""},
				{Keys: "Ctrl+b Shift+r", Description: "Reload config", Mode: ""},
				{Keys: "Ctrl+b o", Description: "Open notification target", Mode: ""},
			},
		},
		{
			Name:        "Panes",
			Description: "Split, navigate, and manage panes",
			Keymaps: []Keymap{
				{Keys: "Ctrl+b v", Description: "Split pane right (vertical)", Mode: ""},
				{Keys: "Ctrl+b -", Description: "Split pane down (horizontal)", Mode: ""},
				{Keys: "Ctrl+b h/j/k/l", Description: "Move focus between panes", Mode: ""},
				{Keys: "Ctrl+b Tab", Description: "Cycle to next pane", Mode: ""},
				{Keys: "Ctrl+b Shift+Tab", Description: "Cycle to previous pane", Mode: ""},
				{Keys: "Ctrl+b z", Description: "Zoom (fullscreen) the focused pane", Mode: ""},
				{Keys: "Ctrl+b x", Description: "Close pane", Mode: ""},
				{Keys: "Ctrl+b Shift+h/j/k/l", Description: "Swap panes", Mode: ""},
				{Keys: "Ctrl+b r", Description: "Enter resize mode", Mode: ""},
				{Keys: "Ctrl+b [", Description: "Enter copy mode", Mode: ""},
				{Keys: "Ctrl+b Shift+p", Description: "Rename pane", Mode: ""},
				{Keys: "Ctrl+b e", Description: "Edit scrollback in $EDITOR", Mode: ""},
			},
		},
		{
			Name:        "Tabs",
			Description: "Create and manage tabs",
			Keymaps: []Keymap{
				{Keys: "Ctrl+b c", Description: "New tab", Mode: ""},
				{Keys: "Ctrl+b n", Description: "Next tab", Mode: ""},
				{Keys: "Ctrl+b p", Description: "Previous tab", Mode: ""},
				{Keys: "Ctrl+b 1..9", Description: "Jump to tab number", Mode: ""},
				{Keys: "Ctrl+b Shift+t", Description: "Rename tab", Mode: ""},
				{Keys: "Ctrl+b Shift+x", Description: "Close tab", Mode: ""},
			},
		},
		{
			Name:        "Workspaces & Session",
			Description: "Move around workspaces and manage the session",
			Keymaps: []Keymap{
				{Keys: "Ctrl+b w", Description: "Workspace navigation / picker", Mode: ""},
				{Keys: "Ctrl+b g", Description: "Goto picker", Mode: ""},
				{Keys: "Ctrl+b Shift+n", Description: "New workspace", Mode: ""},
				{Keys: "Ctrl+b Shift+g", Description: "New worktree", Mode: ""},
				{Keys: "Ctrl+b Shift+w", Description: "Rename workspace", Mode: ""},
				{Keys: "Ctrl+b Shift+d", Description: "Close workspace", Mode: ""},
				{Keys: "Ctrl+b b", Description: "Toggle sidebar", Mode: ""},
			},
		},
		{
			Name:        "Copy Mode (Ctrl+b [)",
			Description: "Navigate, search and copy from the pane scrollback",
			Keymaps: []Keymap{
				{Keys: "h/j/k/l", Description: "Move the cursor", Mode: "copy"},
				{Keys: "w/b/e, W/B/E", Description: "Move by word / big word", Mode: "copy"},
				{Keys: "{ / }", Description: "Jump by paragraph", Mode: "copy"},
				{Keys: "PageUp/PageDown", Description: "Page up / page down", Mode: "copy"},
				{Keys: "Ctrl+u/Ctrl+d", Description: "Half page up / down", Mode: "copy"},
				{Keys: "Ctrl+b/Ctrl+f", Description: "Page up / down (needs a non-default prefix)", Mode: "copy"},
				{Keys: "/ or ?", Description: "Search forward / backward (literal)", Mode: "copy"},
				{Keys: "n/N", Description: "Repeat search same / opposite direction", Mode: "copy"},
				{Keys: "v or Space", Description: "Start a selection", Mode: "copy"},
				{Keys: "y or Enter", Description: "Copy the selection", Mode: "copy"},
				{Keys: "q or Esc", Description: "Leave without copying (Esc clears selection/search)", Mode: "copy"},
			},
		},
		{
			Name:        "Editing Text Fields",
			Description: "Cursor editing in Herdr dialogs and filters, not the shell",
			Keymaps: []Keymap{
				{Keys: "←/→, Ctrl+B/Ctrl+F", Description: "Move one character", Mode: "field"},
				{Keys: "Home/End, Ctrl+A/Ctrl+E", Description: "Move to the beginning / end", Mode: "field"},
				{Keys: "Alt+B/Alt+F", Description: "Move one word", Mode: "field"},
				{Keys: "Backspace, Ctrl+H", Description: "Delete the previous character", Mode: "field"},
				{Keys: "Delete, Ctrl+D", Description: "Delete the next character", Mode: "field"},
				{Keys: "Ctrl+U/Ctrl+K", Description: "Cut to the beginning / end", Mode: "field"},
				{Keys: "Ctrl+W, Alt+Backspace, Ctrl+Backspace", Description: "Cut the previous word", Mode: "field"},
				{Keys: "Alt+D", Description: "Cut the next word", Mode: "field"},
				{Keys: "Ctrl+Y", Description: "Insert the last cut text", Mode: "field"},
			},
		},
	}
}
