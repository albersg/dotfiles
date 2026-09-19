package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// maxSplashArtWidth is the widest art row the welcome splash can carry without
// being clipped. View() pads the splash two columns on each side, so an
// 80-column terminal leaves 76 columns of usable width.
const maxSplashArtWidth = 76

// artRows splits a banner constant into its rows, dropping the single leading
// and trailing newline that the literal carries for readability.
func artRows(art string) []string {
	art = strings.TrimPrefix(art, "\n")
	art = strings.TrimSuffix(art, "\n")
	return strings.Split(art, "\n")
}

// TestArtConstantsRenderWithinTerminalWidth guards the geometry the golden
// screenshot cannot: the splash art must be rectangular and must fit the
// terminal. Rows of unequal width shift the centring of the whole lockup, and a
// row wider than the frame is silently clipped. Width is measured with
// lipgloss.Width, not len, so multi-byte glyphs are counted as rendered cells.
func TestArtConstantsRenderWithinTerminalWidth(t *testing.T) {
	art := []struct {
		name string
		body string
	}{
		{"logo", logo},
		{"compactLogo", compactLogo},
		{"dotfilesText", dotfilesText},
	}

	for _, tc := range art {
		t.Run(tc.name, func(t *testing.T) {
			rows := artRows(tc.body)
			if len(rows) == 0 {
				t.Fatalf("%s has no rows", tc.name)
			}

			width := lipgloss.Width(rows[0])
			for i, row := range rows {
				if got := lipgloss.Width(row); got != width {
					t.Errorf("%s row %d has display width %d, but row 0 has width %d: unequal rows shift the centring of the whole lockup",
						tc.name, i, got, width)
				}
			}

			if width > maxSplashArtWidth {
				t.Errorf("%s is %d columns wide, which exceeds the %d columns available in an 80-column terminal after the splash's two-column side padding: the excess is clipped",
					tc.name, width, maxSplashArtWidth)
			}
		})
	}
}

// TestEmblemArtIsBilaterallySymmetric guards the design invariant of the two
// emblems. Each row must equal its own reverse, otherwise a mistyped glyph
// breaks the symmetry in a way that no screenshot comparison would notice.
func TestEmblemArtIsBilaterallySymmetric(t *testing.T) {
	emblems := []struct {
		name string
		body string
	}{
		{"logo", logo},
		{"compactLogo", compactLogo},
	}

	for _, tc := range emblems {
		t.Run(tc.name, func(t *testing.T) {
			rows := artRows(tc.body)
			if len(rows) == 0 {
				t.Fatalf("%s has no rows", tc.name)
			}

			for i, row := range rows {
				runes := []rune(row)
				reversed := make([]rune, len(runes))
				for j := range runes {
					reversed[len(runes)-1-j] = runes[j]
				}
				if row != string(reversed) {
					t.Errorf("%s row %d is not bilaterally symmetric: %q reversed is %q; symmetry is a design invariant of the emblem",
						tc.name, i, row, string(reversed))
				}
			}
		})
	}
}

// TestEmblemConstantsAreDistinct guards the compact and full emblems against
// collapsing into a single shared constant, which would silently remove the
// height-aware choice in renderWelcome.
func TestEmblemConstantsAreDistinct(t *testing.T) {
	if logo == compactLogo {
		t.Error("logo and compactLogo are the same constant: the compact splash would render the full emblem and the height-aware branch would be meaningless")
	}
}
