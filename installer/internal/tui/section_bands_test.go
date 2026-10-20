package tui

import (
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/updates"
)

// The band contract, in two guards.
//
// TestSectionBandsReadNeitherCursorNorSelectionNorNotice pins the arithmetic:
// for every screen that renders through renderThemeScreen, the bands are the
// same while the cursor walks every row, while the selection changes, and while
// a notice comes and goes. A band that read any of those would move the list
// under the user, which is the reported defect.
//
// TestSectionScreenAnchorsDoNotMove pins the rendering for the two screens whose
// action rows appear with a selection: the way back and the result line keep
// their rows across every cursor position and both selection states.

// sectionFixture is one screen of the section frame with the state its guard
// renders: the fixture the frame guards already maintain.
type sectionFixture struct {
	name string
	m    Model
}

// sectionFixtures builds the measured states of the screens the band contract
// governs. The list is derived from terminalFitCases -- the same enumeration the
// frame guards use -- so a screen added to the section is measured here by its
// own fixture and not by a second list somebody has to remember.
func sectionFixtures(t *testing.T) []sectionFixture {
	t.Helper()
	var out []sectionFixture
	for _, c := range terminalFitCases() {
		m := c.build(t)
		if !m.usesSectionBands() {
			continue
		}
		out = append(out, sectionFixture{name: c.name, m: m})
	}
	if len(out) == 0 {
		t.Fatal("no section screen was built, so the guard measured nothing")
	}
	return out
}

// sectionBandsInput is the input the band arithmetic takes from a render, with
// the parts a render does not derive from the cursor fixed.
func sectionBandsInput(m Model) []installerHint {
	return []installerHint{hintUp, hintDown, hintOpen, hintBack}
}

func TestSectionBandsReadNeitherCursorNorSelectionNorNotice(t *testing.T) {
	sizes := [][2]int{{80, 24}, {120, 36}, {200, 55}}
	for _, f := range sectionFixtures(t) {
		for _, size := range sizes {
			m := f.m
			m.Width, m.Height = size[0], size[1]
			hints := sectionBandsInput(m)
			width := contentWidth(m)
			options := m.GetCurrentOptions()
			base := m.sectionBandsFor(width, hints, 5, len(options))

			if options := m.GetCurrentOptions(); len(options) > 0 {
				for cursor := 0; cursor < len(options); cursor++ {
					moved := m
					moved.Cursor = cursor
					if got := moved.sectionBandsFor(width, hints, 5, len(options)); got != base {
						t.Fatalf("%s at %dx%d moves its bands when the cursor is on row %d: %+v then %+v",
							f.name, size[0], size[1], cursor, base, got)
					}
				}
			}

			// A notice is a result line, not a reservation: it may change the text
			// the band holds, never the band.
			for _, notice := range []string{"", "Saved.", strings.Repeat("a long result line ", 12)} {
				noticed := m
				noticed.ThemeNotice, noticed.UpdateCheck.Notice = notice, notice
				noticed.UpdateCenter.Notice, noticed.DiskClean.Notice = notice, notice
				if got := noticed.sectionBandsFor(width, hints, 5, len(options)); got != base {
					t.Fatalf("%s at %dx%d moves its bands when the notice is %q: %+v then %+v",
						f.name, size[0], size[1], notice, base, got)
				}
			}
		}
	}
}

// TestSectionScreenAnchorsDoNotMove is the rendering half: at one cursor
// position, turning the selection on -- which is what adds action rows -- must
// not move the entries, the way back or the closing rule. Cursor movement itself
// may scroll the window, which is the list doing its job; what it may not do is
// reflow the frame, so the way back and the closing rule are also compared
// across every cursor position.
func TestSectionScreenAnchorsDoNotMove(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {120, 36}, {200, 55}} {
		for _, f := range []struct {
			name   string
			build  func(*testing.T) Model
			toggle func(*Model)
		}{
			{
				name:  "update-center",
				build: updateCenterFrameCase,
				toggle: func(m *Model) {
					// Every reported entry selected draws the tallest action block:
					// select-the-shown, apply and clear are all offered.
					m.UpdateCenter.Selected = updates.NewSelection()
					for _, result := range m.UpdateCenter.Results {
						for _, item := range result.Items {
							m.UpdateCenter.Selected.Toggle(item)
						}
					}
				},
			},
			{
				name:  "disk-cleaner",
				build: diskCleanerFrameCase,
				toggle: func(m *Model) {
					for _, item := range m.DiskClean.Items {
						m.DiskClean.Selection[item.ID] = true
					}
				},
			},
		} {
			m := f.build(t)
			m.Width, m.Height = size[0], size[1]

			// The anchors are positions, never text: the card under the list changes
			// its words as the cursor moves, which is the point of the card. What may
			// not change is where the rows are.
			anchor := func(model Model) (back, closing int, marks []int) {
				rows := strings.Split(ansiEscape.ReplaceAllString(model.View(), ""), "\n")
				back, closing = -1, -1
				for i, row := range rows {
					trimmed := strings.TrimSpace(row)
					if len([]rune(trimmed)) > 10 && strings.Trim(trimmed, "─") == "" {
						closing = i
					}
					if strings.Contains(row, "← Back") {
						back = i
					}
					if strings.Contains(row, "[ ]") || strings.Contains(row, "[x]") || strings.Contains(row, "[held]") {
						marks = append(marks, i)
					}
				}
				return back, closing, marks
			}

			options := m.GetCurrentOptions()
			selected := m
			f.toggle(&selected)

			for cursor := 0; cursor < len(options); cursor++ {
				base := m
				base.Cursor = cursor
				withSelection := selected
				withSelection.Cursor = cursor

				wantBack, wantClosing, wantMarks := anchor(base)
				if wantBack < 0 || wantClosing < 0 {
					t.Fatalf("%s at %dx%d draws no way back or no closing rule: back %d closing %d",
						f.name, size[0], size[1], wantBack, wantClosing)
				}
				back, closing, marks := anchor(withSelection)
				if closing != wantClosing {
					t.Errorf("%s at %dx%d moves the closing rule from row %d to %d with the selection on at cursor %d",
						f.name, size[0], size[1], wantClosing, closing, cursor)
				}
				if back != wantBack {
					t.Errorf("%s at %dx%d moves the way back from row %d to %d with the selection on at cursor %d",
						f.name, size[0], size[1], wantBack, back, cursor)
				}
				if len(marks) != len(wantMarks) {
					t.Errorf("%s at %dx%d shows %d entry rows with the selection on at cursor %d, want %d: %v then %v",
						f.name, size[0], size[1], len(marks), cursor, len(wantMarks), wantMarks, marks)
					continue
				}
				for i := range marks {
					if marks[i] != wantMarks[i] {
						t.Errorf("%s at %dx%d moves entry row %d from row %d to %d with the selection on at cursor %d",
							f.name, size[0], size[1], i, wantMarks[i], marks[i], cursor)
					}
				}
			}
		}
	}
}
