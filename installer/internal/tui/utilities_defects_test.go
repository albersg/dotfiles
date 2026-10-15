package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// ---------------------------------------------------------------------------
// The utilities section: the defects this front fixes
// ---------------------------------------------------------------------------
//
// Two defects the audit found in the section that already shipped, both of them
// invisible in a single frame and both measured against the code that caused
// them. Each guard names the defect and pins the behaviour, so a later refactor
// that reintroduces one fails here rather than in a user's session.

// sectionUtilitiesModel is the section as its first frame is drawn: the desktop
// switch is forced away and the WSL read is forced finished-and-absent, so the
// row list depends on the reads under test and on nothing this runner happens to
// have. Without that, a machine with a desktop or a .wslconfig would hand the
// guards a different list and they would measure the host.
func sectionUtilitiesModel(t *testing.T) Model {
	t.Helper()

	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	m := NewModel()
	m.Screen = ScreenUtilities
	m.Width, m.Height = 120, 40
	m.ThemeSwitchFound = false
	m.WSLState = wslResourceState{Resolved: true, Available: false, Reason: "no .wslconfig"}
	return m
}

// TestTheSectionHoldsItsCursorWhenABackgroundReadInsertsARow is the guard for the
// first defect: the section built its rows from three background reads and kept
// the cursor by index while every other menu had already been taught to keep it
// by label. A row that arrives late is inserted ABOVE the rows that were there,
// so the index the user left on names a different job the moment the answer
// lands, and the next Enter opens a screen nobody aimed at.
//
// Both reads that insert a row above the others are measured: the theme
// definitions, which are read on every host, and the WSL resources, which are
// read on the hosts where the row appears at all.
func TestTheSectionHoldsItsCursorWhenABackgroundReadInsertsARow(t *testing.T) {
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}

	tests := []struct {
		name    string
		prepare func(m Model) Model
		answer  func(t *testing.T) tea.Msg
	}{
		{
			"the theme definitions read",
			func(m Model) Model { return m },
			func(t *testing.T) tea.Msg {
				return dotfilesThemesLoadedMsg{themes: defs, repoDir: repoRoot(t)}
			},
		},
		{
			"the WSL resource read",
			// The first read is the one that fills the state; a model that already
			// holds a resolved one would have its answer dropped and the guard would
			// measure nothing.
			func(m Model) Model { m.WSLState = wslResourceState{}; return m },
			func(t *testing.T) tea.Msg {
				return wslResourceLoadedMsg{state: wslResourceTestState(t)}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			m := tt.prepare(sectionUtilitiesModel(t))
			// The theme definitions and the WSL row are what each read adds, so
			// the section must not already carry them for this to measure anything.
			if m.WSLState.Available {
				t.Fatal("the WSL row is already offered, so this case measures nothing")
			}

			before := m.GetCurrentOptions()
			aimed := utilitiesVerifyRow
			index := -1
			for i, option := range before {
				if option == aimed {
					index = i
				}
			}
			if index < 0 {
				t.Fatalf("the first frame offers no %q row: %v", aimed, before)
			}
			m.Cursor = index
			if got := m.selectedOption(); got != aimed {
				t.Fatalf("the cursor names %q before the answer, want %q", got, aimed)
			}

			next, _ := m.Update(tt.answer(t))
			m = next.(Model)

			after := m.GetCurrentOptions()
			if len(after) <= len(before) {
				t.Fatalf("the answer added no row (%d rows before, %d after), so this guard proves nothing: %v",
					len(before), len(after), after)
			}
			if got := m.selectedOption(); got != aimed {
				t.Errorf("the answer moved the cursor from %q to %q: the row arrived above it and the index was kept",
					aimed, got)
			}
		})
	}
}

// TestAFailedThemeSwitchIsStillReadableWhenTheUserComesBack is the guard for the
// second defect. The failure of a desktop theme switch is written into the
// section's notice, and its comment says a failure stays there -- but stepping
// into the section cleared the notice first, and a real failure is the slow case,
// so the one outcome a user has to see was the one they could never read.
//
// The sequence is the one that produced it: press a switch row, step away before
// the bounded command answers, let the answer land while the user is elsewhere,
// and come back.
func TestAFailedThemeSwitchIsStillReadableWhenTheUserComesBack(t *testing.T) {
	m := sectionUtilitiesModel(t)

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if m.Screen != ScreenMainMenu {
		t.Fatalf("esc left the section on %v, want ScreenMainMenu", m.Screen)
	}
	if cmd != nil {
		t.Log("leaving the section still returned a command; the guard only needs the notice")
	}

	// The answer lands while the user is on the main menu. It is a failure, which
	// is the slow outcome and the one nothing else reports.
	next, _ = m.Update(themeChangedMsg{err: errors.New("gsettings: command not found")})
	m = next.(Model)
	if m.ThemeNotice == "" {
		t.Fatal("the failure was not recorded on the model at all")
	}

	// Coming back is the only route to the outcome: the switch lives in this
	// section and nowhere else reports it.
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	m = next.(Model)
	if m.Screen != ScreenUtilities {
		t.Fatalf("the section shortcut landed on %v, want ScreenUtilities", m.Screen)
	}
	if m.ThemeNotice == "" {
		t.Fatal("the failure was cleared on the way in, so it can never be read")
	}

	view := ansiEscape.ReplaceAllString(m.View(), "")
	if !strings.Contains(strings.Join(strings.Fields(view), " "), "gsettings: command not found") {
		t.Errorf("the section does not show why the switch failed:\n%s", view)
	}
}

// TestTheUtilitiesSummaryNamesTheSectionWithoutADesktopSwitch is the guard for the
// fifth defect: the one-line summary the narrow layout gives every row was blank
// for the Utilities row on a host with no desktop theme switch, so the row that
// opens five utilities described itself as nothing while the rows beside it said
// what they do. The switch-detected half is asserted beside it, because that is
// the behaviour the fix touches.
func TestTheUtilitiesSummaryNamesTheSectionWithoutADesktopSwitch(t *testing.T) {
	m := contextualMainMenuModel()
	m.ThemeSwitch, m.ThemeSwitchFound = themeSwitch{}, false

	index := -1
	for i, option := range m.GetCurrentOptions() {
		if strings.Contains(option, "Utilities") {
			index = i
		}
	}
	if index < 0 {
		t.Fatalf("the main menu offers no Utilities row: %v", m.GetCurrentOptions())
	}
	m.Cursor = index
	if choice, _ := m.mainMenuSelection(); choice != choiceUtilities {
		t.Fatalf("the cursor names %v, want the utilities choice", choice)
	}

	if got := m.contextHeadline(); got == "" {
		t.Error("the utilities row has no summary on a host with no desktop theme switch, so the row describes itself as nothing")
	}

	// A detected switch is what the summary named before, and still does.
	target, ok := themeSwitchByID("gnome")
	if !ok {
		t.Fatal("the theme switch table no longer holds the gnome entry")
	}
	m.ThemeSwitch, m.ThemeSwitchFound = target, true
	headline := m.contextHeadline()
	if !strings.Contains(headline, "GNOME") || !strings.Contains(headline, "theme switch") {
		t.Errorf("the summary no longer names the detected switch: %q", headline)
	}
}

// defect: with no definitions readable the body said the dotfiles theme was "not
// switchable here" in the one state where the row is still offered -- for the
// undo of a change the record names. The panel already read that state correctly,
// so the body was the odd one out.
func TestTheSectionDoesNotDenyTheThemeItStillOffers(t *testing.T) {
	m := sectionUtilitiesModel(t)
	m.DotfilesThemes = nil
	m.DotfilesThemesErr = themeDefinitionsNotFoundMessage()
	m.DotfilesThemeRecord = &dotfilesThemeRecord{Theme: "catppuccin-mocha"}

	if !anyOptionContains(m.GetCurrentOptions(), utilitiesThemeRow) {
		t.Fatalf("the row is not offered, so the body cannot contradict it: %v", m.GetCurrentOptions())
	}

	body := strings.Join(m.utilitiesDescription(), " ")
	if strings.Contains(body, "not switchable") {
		t.Errorf("the body denies the utility its own row offers: %q", body)
	}
	if !strings.Contains(body, "undone") {
		t.Errorf("the body does not say what the row can still do: %q", body)
	}
}
