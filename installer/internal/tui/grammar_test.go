package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// TestTheCardGrammarIsExactlyTheWidthItIsGiven pins the card, the row and the
// stat boxes to the width their callers measure them against. A card one column
// wider than its column is what composes into the gutter and cuts a word off the
// neighbouring card; the failure is invisible until two columns are drawn, which
// is why it is measured here instead of only in a golden.
func TestTheCardGrammarIsExactlyTheWidthItIsGiven(t *testing.T) {
	for _, width := range []int{40, 56, 70, 84} {
		body := []string{InkStyle.Render("body")}
		for _, caption := range []string{"", "This machine, now", "A caption that is long enough to be dropped"} {
			lines := card("A title", caption, width, body)
			for i, line := range lines {
				if got := lipgloss.Width(line); got != width {
					t.Errorf("card(%d, %q) line %d is %d columns, want %d: %q", width, caption, i, got, width, line)
				}
			}
		}
	}
}

// TestTheCardRowIsExactlyTheContentWidth pins the two-line row: the selected bar
// has to reach both edges of the card's content and the subtitle has to end on the
// same column as the title, or the selection is a slab with a loose end.
func TestTheCardRowIsExactlyTheContentWidth(t *testing.T) {
	row := cardRow{Icon: dashboardOSIcon, Title: "Change the dotfiles theme", Subtitle: "Recolour the installer and the system", Chevron: true}
	for _, width := range []int{40, 56, 66} {
		for _, selected := range []bool{false, true} {
			for i, line := range cardRowLines(row, width, selected) {
				if got := lipgloss.Width(line); got != width {
					t.Errorf("cardRowLines(%d, selected=%v) line %d is %d columns, want %d: %q", width, selected, i, got, width, line)
				}
			}
		}
	}
}

// TestTheUtilitiesCardIsExactlyItsColumnWidth is the composed case the two-column
// screen actually draws: a titled card holding a filled row.
func TestTheUtilitiesCardIsExactlyItsColumnWidth(t *testing.T) {
	row := cardRow{Icon: utilitiesShellIcon, Title: "Change the dotfiles theme", Subtitle: "Recolour the installer and the system", Chevron: true}
	lines := card("System Utilities", "Maintenance, diagnostics and configuration tools for your environment.", 70,
		cardRowLines(row, cardContentWidth(70), true))
	for i, line := range lines {
		if got := lipgloss.Width(line); got != 70 {
			t.Errorf("the composed utilities card line %d is %d columns, want 70: %q", i, got, line)
		}
	}
}

// TestTheStatBoxesFillTheWidthTheyAreGiven pins the box band the same way.
func TestTheStatBoxesFillTheWidthTheyAreGiven(t *testing.T) {
	boxes := []statBox{
		{Label: "Current median", Value: "412 ms", Tone: BrandStyle},
		{Label: "Best (session)", Value: "398 ms", Tone: InkStyle},
		{Label: "Previous median", Value: "436 ms", Tone: InkStyle},
		{Label: "Last run", Value: "✓ 408 ms", Tone: SuccessStyle},
	}
	for _, width := range []int{60, 96, 120} {
		for i, line := range statBoxLines(boxes, width) {
			if got := lipgloss.Width(line); got != width {
				t.Errorf("statBoxLines(%d) row %d is %d columns, want %d: %q", width, i, got, width, line)
			}
		}
	}
}
