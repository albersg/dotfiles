package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// TestInstallingCounterAndBarAgree pins the installing screen's two readings of
// the run: the step sentence and the percentage come from the same progress, so a
// run on its last step cannot say "Step 16 of 16" beside 94% with no explanation
// of the gap, and a finished run says the last step beside 100%.
func TestInstallingCounterAndBarAgree(t *testing.T) {
	cases := []struct {
		name  string
		steps []InstallStep
	}{
		{"half of five", []InstallStep{
			{Name: "one", Status: StatusDone},
			{Name: "two", Status: StatusDone},
			{Name: "three", Status: StatusRunning, Progress: 0.5},
			{Name: "four", Status: StatusPending},
			{Name: "five", Status: StatusPending},
		}},
		{"last step running", []InstallStep{
			{Name: "one", Status: StatusDone},
			{Name: "two", Status: StatusDone},
			{Name: "three", Status: StatusRunning, Progress: 0.25},
		}},
		{"finished", []InstallStep{
			{Name: "one", Status: StatusDone},
			{Name: "two", Status: StatusSkipped},
			{Name: "three", Status: StatusDone},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := Model{Steps: tc.steps}
			current, total, percent := m.installingPosition()
			if total != len(tc.steps) {
				t.Fatalf("total = %d, want %d", total, len(tc.steps))
			}
			if percent < float64(current-1)/float64(total)-0.001 || percent > float64(current)/float64(total)+0.001 {
				t.Errorf("step %d of %d sits outside its own progress %.3f", current, total, percent)
			}
			if text := m.installingStepText(); !strings.Contains(text, "Step "+itoa(current)+" of "+itoa(total)) {
				t.Errorf("the sentence %q does not name the step the progress puts the run on (%d of %d)", text, current, total)
			}
		})
	}
}

// TestInstallingChecklistNamesTheRowsItHides pins the floor's note: a checklist
// window that cannot show every step says how many it left above or below instead
// of appearing to hold a plan that ended early.
func TestInstallingChecklistNamesTheRowsItHides(t *testing.T) {
	steps := make([]InstallStep, 16)
	for i := range steps {
		steps[i] = InstallStep{Name: "step", Status: StatusPending}
	}
	steps[10].Status = StatusRunning
	m := Model{Steps: steps, CurrentStep: 10}

	rows := m.installingRailRows(40, 5)
	joined := strings.Join(rows, "\n")
	if len(rows) > 5 {
		t.Fatalf("the checklist drew %d rows for a 5-row band", len(rows))
	}
	if !strings.Contains(joined, "above") && !strings.Contains(joined, "below") {
		t.Errorf("the checklist hid rows without saying so:\n%s", joined)
	}
	if !strings.Contains(joined, "step") {
		t.Errorf("the checklist dropped the step it is on:\n%s", joined)
	}
}

// TestEscapeOnTheEndScreensQuits pins the two dead ends: Escape is accepted as
// the quit key everywhere else's footer calls "back", and the completion and
// failure screens used to answer only q.
func TestEscapeOnTheEndScreensQuits(t *testing.T) {
	for _, screen := range []Screen{ScreenComplete, ScreenError} {
		m := Model{Screen: screen}
		updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
		got := updated.(Model)
		if !got.Quitting {
			t.Errorf("Escape on screen %v did not quit", screen)
		}
		if cmd == nil {
			t.Errorf("Escape on screen %v returned no quit command", screen)
		}
	}
}

// TestUtilitiesPanelSpeaksForTheSelectedRow pins the fix: the overview panel names
// the job the cursor is on rather than stating the same machine for every row.
func TestUtilitiesPanelSpeaksForTheSelectedRow(t *testing.T) {
	m := utilitiesFrameModel(t)
	m.SystemInfo = goldenSystemInfo()
	m.Width, m.Height = 160, 50
	m.Animating = false
	m.Cursor = 0
	first := m.GetCurrentOptions()[0]
	view := ansiEscape.ReplaceAllString(m.View(), "")
	if !strings.Contains(view, "Selected") {
		t.Fatalf("the panel does not say which row it is speaking for:\n%s", view)
	}
	title := strings.Split(strings.TrimPrefix(first, utilitiesScreenMark), " ")[0]
	if !strings.Contains(view, title) {
		t.Errorf("the panel does not name the selected row %q", title)
	}
}

// TestThemePaletteSaysWhetherItShowsTheAppliedTheme pins the board's provenance:
// a palette that is only a preview has to say so, and the applied one says it is
// applied, so a reader never mistakes the colours under the cursor for the ones
// the interface is wearing.
func TestThemePaletteSaysWhetherItShowsTheAppliedTheme(t *testing.T) {
	m := utilitiesFrameModel(t)
	m.DotfilesThemeRecord = &dotfilesThemeRecord{Theme: "catppuccin-mocha"}
	defs, err := loadThemeDefinitions(repoRoot(t))
	if err != nil {
		t.Fatalf("load the theme definitions: %v", err)
	}
	m.DotfilesThemes = defs
	facts := strings.Join(m.themePaletteFacts(layout{Right: 80}), "\n")
	if !strings.Contains(facts, "the applied theme") {
		t.Errorf("the palette does not say it is showing the applied theme:\n%s", facts)
	}
}

// itoa keeps the guard's expected sentence readable without pulling strconv into
// every branch.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// TestHomeWideDrawsTheReferenceCards pins the wide home's parts: the lockup's
// tagline, the four machine cards, the greeting, the filled call to action, the
// machine card, the numbered plan and the newest backup.
func TestHomeWideDrawsTheReferenceCards(t *testing.T) {
	m := NewModel()
	isolateGoldenTest(t, &m)
	m.Width, m.Height = 160, 50
	m.Screen = ScreenWelcome
	m.SystemInfo = goldenSystemInfo()
	m.AvailableBackups = []system.BackupInfo{{
		Path:      "/home/testuser/.dotfiles-backup/2026-10-20_111941",
		Timestamp: time.Date(2026, 10, 20, 11, 19, 41, 0, time.UTC),
		Files:     []string{"a", "b", "c"},
	}}
	m.Animating = false

	plain := ansiEscape.ReplaceAllString(m.View(), "")
	for _, want := range []string{
		"Your personal development environment.", "Fast. Consistent. Yours.",
		"OS", "Shell", "Arch", "Packages", "Good morning",
		"Press [s] to start installation", "or explore the menu with [Tab]",
		"Your machine", "Next steps", "Recent backup",
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("the wide home does not draw %q", want)
		}
	}
}

// TestShellAuditWideDrawsTheReferenceCards pins the wide shell startup's parts:
// the explanation's numbered questions, the filled measure button, the plan
// options, the results, the recent runs and the methodology.
func TestShellAuditWideDrawsTheReferenceCards(t *testing.T) {
	m := shellAuditFrameCase(t)
	m.Width, m.Height = 160, 50
	m.Animating = false

	plain := ansiEscape.ReplaceAllString(m.View(), "")
	for _, want := range []string{
		"The shell's startup", "What is measured?", "How many runs?", "Controls?",
		shellAuditMeasureRow, "Interactive", "Runs:", "Warm-up:", "Clean comparison",
		"Results", "Run durations (ms)", "Recent runs", "Methodology", "← Back",
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("the wide shell startup does not draw %q", want)
		}
	}
	if rows := renderedRowCount(m.View()); rows > m.Height {
		t.Errorf("the wide shell startup draws %d rows at 160x50, want <= %d", rows, m.Height)
	}
}

// TestThemePaletteNamesThePreviewedTheme is the other half of the board's
// provenance: while the cursor is on a theme row, the palette says the colours
// are the preview and that Enter applies them, not that they are already on.
func TestThemePaletteNamesThePreviewedTheme(t *testing.T) {
	m := themePickerFrameCase(t)
	options := m.dotfilesThemeOptions()
	if len(options) == 0 {
		t.Skip("no theme definitions are available to preview")
	}
	for i, option := range m.GetCurrentOptions() {
		if option == options[0] {
			m.Cursor = i
			break
		}
	}
	if _, ok := m.previewThemeDef(); !ok {
		t.Fatalf("the cursor is not on a theme row, so the preview path is not exercised")
	}
	facts := strings.Join(m.themePaletteFacts(layout{Right: 80}), "\n")
	if !strings.Contains(facts, "previewed theme") {
		t.Errorf("the palette does not say it is showing a preview:\n%s", facts)
	}
}

// TestDoctorHeaderCountMatchesTheRowsDrawn pins the doctor's header range to the
// finding rows that actually land in the list band. The range used to be computed
// before the band was trimmed, so a short frame could say "Showing 1-5 of 9" over
// three rows.
func TestDoctorHeaderCountMatchesTheRowsDrawn(t *testing.T) {
	m := doctorFrameCase(t)
	m.Width, m.Height = 80, 18
	plain := ansiEscape.ReplaceAllString(m.View(), "")

	at := strings.Index(plain, "Showing ")
	if at < 0 {
		t.Skip("this frame draws the whole list, so there is no window to measure")
	}
	var first, last, total int
	if _, err := fmt.Sscanf(plain[at:], "Showing %d-%d of %d", &first, &last, &total); err != nil {
		t.Fatalf("could not read the header range: %v", err)
	}
	drawn := 0
	for _, line := range strings.Split(plain, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "▸ "))
		for _, mark := range []string{"✓", "⊘", "!", "✗", "?"} {
			for _, label := range []string{"OK", "Not applicable", "Warning", "Error", "Unknown"} {
				if strings.HasPrefix(line, mark+" "+label+": ") {
					drawn++
				}
			}
		}
	}
	if drawn != last-first+1 {
		t.Errorf("the header says %d rows (%d-%d) but %d finding rows are drawn:\n%s", last-first+1, first, last, drawn, plain)
	}
}

// TestUpdateCenterFloorDetailAddsAFactTheRowLacks pins the 80x24 detail band: the
// row already carries the name, the owner and both versions, so the detail starts
// with the facts the row cannot hold (the full versions, a hold, the note and the
// command the answer was read with) rather than repeating the row.
func TestUpdateCenterFloorDetailAddsAFactTheRowLacks(t *testing.T) {
	m := updateCenterFrameCase(t)
	m.Width, m.Height = 80, 24
	item, ok := m.updateCenterFocusedItem()
	if !ok {
		t.Fatal("the fixture's cursor is not on an entry")
	}
	plain := ansiEscape.ReplaceAllString(m.View(), "")
	if !strings.Contains(plain, "Read with") {
		t.Errorf("the floor's detail does not state a fact the row lacks:\n%s", plain)
	}
	if strings.Contains(plain, "Name        ") || strings.Contains(plain, "Manager     ") {
		t.Errorf("the floor's detail repeats the row's own facts:\n%s", plain)
	}
	if got := strings.Count(plain, item.Name); got != 1 {
		t.Errorf("the focused entry %q is drawn %d times, want once in the list and not again in the detail", item.Name, got)
	}
}
