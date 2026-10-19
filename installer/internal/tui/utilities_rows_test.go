package tui

import (
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// TestEveryUtilitiesRowDoesSomething walks every row the Utilities section can
// offer and presses the key the row itself advertises. A row that is drawn but
// has no branch for the key it names is a dead row: the user presses the key the
// row offers, nothing happens, and nothing distinguishes that from a key that was
// never delivered.
//
// #372 was exactly that. utilitiesWSLAdvisorRow was appended by the section and
// rendered with its panel line, but the selection switch had no case for it, so
// Enter on it did nothing.
//
// The row now says what it is (issue #375): a row that opens a screen carries the
// arrow, a row that acts where it stands carries its key in brackets, and a switch
// carries its state. This guard reads that mark and presses exactly the key it
// names, so the claim on the row and the press behind it cannot drift. It also
// pins the grammar's first rule from the other side: Enter must not run a row that
// carries its own letter.
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
				utilitiesDarkRow,
				utilitiesLightRow,
				utilitiesUndoRow,
				utilitiesScreenRow(utilitiesThemeRow),
			},
		},
		{
			name: "a WSL instance",
			prepare: func(m *Model) {
				m.SystemInfo = &system.SystemInfo{OS: system.OSLinux, IsWSL: true}
				m.WSLRuntime = system.WSLRuntime{Inside: true, Version: 2}
				m.WSLState.Available = true
			},
			want: []string{utilitiesScreenRow(utilitiesWSLRow), utilitiesScreenRow(utilitiesWSLAdvisorRow)},
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

				key, kind := utilitiesRowKey(row)
				if kind == utilitiesRowUnknown {
					t.Fatalf("the row %q does not say what it is: every row says it opens a screen, runs a command, or is a switch", row)
				}

				// The command a row hands back is deliberately never run: the
				// switch rows would change the desktop this test runs on.
				m.Cursor = index
				next, cmd := m.handleUtilitiesKeys(key)
				landed := next.(Model)
				if landed.Screen == ScreenUtilities && cmd == nil {
					t.Errorf("the row %q names the key %q and pressing it left the section on its own screen and handed back no command", row, key)
				}

				if kind != utilitiesRowScreen {
					// S1 of odd/tasks/key-grammar.md: Enter opens. A row that acts
					// where it stands must not also run on Enter.
					m.Cursor = index
					next, cmd := m.handleUtilitiesKeys("enter")
					landed := next.(Model)
					if landed.Screen != ScreenUtilities || cmd != nil {
						t.Errorf("the row %q acts where it stands and still answers Enter", row)
					}
				}
			}
		})
	}
}

// The kinds a Utilities row can be, as the mark at the front of the row says.
const (
	utilitiesRowUnknown = iota
	utilitiesRowScreen
	utilitiesRowCommand
	utilitiesRowSwitch
)

// utilitiesRowKey reads the key a drawn Utilities row advertises and the kind that
// key belongs to: the arrow opens with Enter, a command carries its own letter in
// brackets, a switch carries its state and answers Space, and the way back is a
// screen opened with Enter too. A row that carries none of the marks is unknown,
// and the guard fails on it rather than guessing.
func utilitiesRowKey(row string) (string, int) {
	if strings.HasPrefix(row, utilitiesScreenMark) || strings.HasPrefix(row, "← Back") {
		return "enter", utilitiesRowScreen
	}
	if len(row) >= 4 && row[0] == '[' && row[2] == ']' && row[3] == ' ' {
		if row[1] == ' ' {
			return " ", utilitiesRowSwitch
		}
		if row[1] >= 'a' && row[1] <= 'z' {
			return row[1:2], utilitiesRowCommand
		}
	}
	return "", utilitiesRowUnknown
}
