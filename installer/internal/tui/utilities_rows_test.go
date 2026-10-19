package tui

import (
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// TestEveryUtilitiesRowDoesSomething walks every row the Utilities section can
// offer and presses Enter on it. A row that is drawn but has no branch in the
// section's selection handler is a dead row: the user presses Enter, nothing
// happens, and nothing distinguishes that from a key that was never delivered.
//
// #372 was exactly that. utilitiesWSLAdvisorRow was appended by the section and
// rendered with its panel line, but the selection switch had no case for it, so
// Enter on it did nothing.
//
// An earlier version of this guard asked every row for a new screen, and passed
// on the machine it was written on. CI showed what that had missed: a host with a
// desktop theme switch is offered "Switch to the dark theme" and "Switch to the
// light theme", and those rows act where they stand -- they set a notice and
// return the command that changes the desktop -- so they never leave the
// section. The question a row has to answer is therefore "does it do something":
// open a screen, or hand back a command. A row that does neither is dead.
//
// The host decides which rows exist, so the walk is run once per shape that adds
// rows, and each shape names the rows it must offer. Without that the guard
// silently covered only the rows of the runner it ran on: ThemeSwitchFound is
// false on Linux, so the two switch rows are absent there and were offered, and
// failed, on macOS. A new condition in the section means a new shape here.
func TestEveryUtilitiesRowDoesSomething(t *testing.T) {
	target, ok := themeSwitchByID("gnome")
	if !ok {
		t.Fatal("the theme switch table no longer holds the gnome entry")
	}

	shapes := []struct {
		name    string
		prepare func(m *Model)
		// want are the rows this shape exists to cover. A shape that stops
		// offering one of them fails here rather than quietly walking less than
		// the shape claims to cover.
		want []string
	}{
		{
			name: "a host with no desktop theme switch and no WSL instance",
			prepare: func(m *Model) {
				m.ThemeSwitch, m.ThemeSwitchFound = themeSwitch{}, false
				m.ThemeRecord = nil
				m.DotfilesThemes = nil
				m.DotfilesThemeRecord = nil
				m.SystemInfo = &system.SystemInfo{OS: system.OSLinux}
				m.WSLRuntime = system.WSLRuntime{}
				m.WSLState.Available = false
			},
		},
		{
			name: "a host whose desktop theme switch was detected, with a record this installer wrote",
			prepare: func(m *Model) {
				m.ThemeSwitch, m.ThemeSwitchFound = target, true
				m.ThemeRecord = &themeRecord{Target: target.ID}
				m.DotfilesThemeRecord = &dotfilesThemeRecord{Theme: "dotfiles"}
			},
			want: []string{
				"Switch to the dark theme",
				"Switch to the light theme",
				"Undo the last theme change",
				utilitiesThemeRow,
			},
		},
		{
			name: "a WSL instance",
			prepare: func(m *Model) {
				m.SystemInfo = &system.SystemInfo{OS: system.OSLinux, IsWSL: true}
				m.WSLRuntime = system.WSLRuntime{Inside: true, Version: 2}
				m.WSLState.Available = true
			},
			want: []string{utilitiesWSLRow, utilitiesWSLAdvisorRow},
		},
	}

	for _, shape := range shapes {
		t.Run(shape.name, func(t *testing.T) {
			m := NewModel()
			m.Screen = ScreenUtilities
			m.Cursor = 0
			shape.prepare(&m)

			rows := m.GetCurrentOptions()
			if len(rows) == 0 {
				t.Fatal("the Utilities section offers no rows, so this guard would pass by looking at nothing")
			}
			for _, row := range shape.want {
				if !containsRow(rows, row) {
					t.Fatalf("this shape no longer offers %q, so the guard would cover less than it claims: %v", row, rows)
				}
			}

			for index, row := range rows {
				// The divider a section uses to group its choices is offered in
				// this list and is not a choice: it has no branch, and it must not
				// have one. The views detect it by its dashes, which is the same
				// test the section's own handler uses to ignore it.
				if strings.HasPrefix(row, menuSeparatorPrefix) {
					continue
				}

				// The command a row hands back is deliberately never run: the
				// switch rows would change the desktop this test runs on.
				m.Cursor = index
				next, cmd := m.handleUtilitiesKeys("enter")
				landed := next.(Model)
				if landed.Screen == ScreenUtilities && cmd == nil {
					t.Errorf("the row %q has no branch in the section's handler: pressing Enter left the section on its own screen and handed back no command", row)
				}
			}
		})
	}
}
