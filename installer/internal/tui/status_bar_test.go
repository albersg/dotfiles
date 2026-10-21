package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// TestStatusBarNamesEveryFrameState pins the five words the frame's status bar
// may say and the screen each one belongs to. The mockup's last row leads with a
// dot and the state the frame is in; a word that no screen can reach, or a screen
// that says a word it is not in, is the defect this names.
func TestStatusBarNamesEveryFrameState(t *testing.T) {
	cases := []struct {
		name   string
		screen Screen
		want   string
	}{
		{"the welcome screen", ScreenWelcome, statusReady},
		{"the main menu", ScreenMainMenu, statusReady},
		{"the setup", ScreenSetup, statusReady},
		{"the utilities", ScreenUtilities, statusReady},
		{"the run being watched", ScreenInstalling, statusInstalling},
		{"the theme selector", ScreenThemePicker, statusPreview},
		{"the completed run", ScreenComplete, statusDone},
		{"the failed run", ScreenError, statusFailed},
	}
	words := map[string]bool{}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := Model{Screen: c.screen}
			word, dot := m.statusBar()
			if word != c.want {
				t.Errorf("%s says %q, want %q", c.name, word, c.want)
			}
			// The dot is a glyph and the word is text, so the state survives a
			// terminal with no colour and no Nerd Font: the glyph is decoration.
			block := ansiEscape.ReplaceAllString(statusBarBlock(word, dot), "")
			if block != statusDotGlyph+" "+c.want {
				t.Errorf("%s's status block is %q, want %q", c.name, block, statusDotGlyph+" "+c.want)
			}
			words[word] = true
		})
	}

	// The vocabulary is the mockup's five, so this is what "every state" means.
	if len(words) != 5 {
		t.Errorf("the frame names %d states, want the mockup's 5: %v", len(words), words)
	}
}

// TestEveryRenderedFrameCarriesTheStatusBar is the class guard for S2: every
// screen the terminal matrix renders carries the status bar, whatever the screen
// is. It reads the shared matrix rather than a list of its own, so a screen added
// without the chrome is caught by the same pass that measures its frame.
func TestEveryRenderedFrameCarriesTheStatusBar(t *testing.T) {
	frames := terminalFrames(t)
	if len(frames) == 0 {
		t.Fatal("no frames to read, so this guard proves nothing")
	}

	for _, f := range frames {
		f := f
		t.Run(f.caseName+"/"+f.sizeName, func(t *testing.T) {
			word, _ := Model{Screen: f.screen}.statusBar()
			block := statusDotGlyph + " " + word
			line, ok := rowStartingWith(f.view, block)
			if !ok {
				t.Errorf("%s at %s draws no row that leads with the status bar %q:\n%s",
					f.caseName, f.sizeName, block, ansiEscape.ReplaceAllString(f.view, ""))
				return
			}
			if w := lipgloss.Width(strings.TrimRight(line, " ")); w > f.width-viewPaddingCols*2 {
				t.Errorf("%s at %s draws a %d-column status row on a %d-column terminal",
					f.caseName, f.sizeName, w, f.width-viewPaddingCols*2)
			}
		})
	}
}

// TestStatusBarRidesTheFooterRule pins where the installer's frame puts the
// status bar: the state block leads the rule that closes the body, and the rule
// carries on to the frame's edge. The rule is the one row with nothing else to
// say, so the state costs the body no row and the legend no columns -- which is
// what keeps the review screen's way back and the WSL footers on one row at 80
// columns.
func TestStatusBarRidesTheFooterRule(t *testing.T) {
	cases := []struct {
		name  string
		build func(t *testing.T) Model
	}{
		{"the main menu", func(t *testing.T) Model { return installerFrameCase(t, "main-menu") }},
		{"the setup", func(t *testing.T) Model { return installerFrameCase(t, "setup") }},
		{"the run being watched", func(t *testing.T) Model { return installerFrameCase(t, "installing") }},
		{"the completed run", func(t *testing.T) Model { return installerFrameCase(t, "complete") }},
		{"the failed run", func(t *testing.T) Model { return installerFrameCase(t, "error") }},
		{"the utilities", utilitiesFrameCase},
		{"the theme selector", themePickerFrameCase},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			m := c.build(t)
			m.Width, m.Height = 80, 24
			m.Animating = false

			word, _ := m.statusBar()
			block := statusDotGlyph + " " + word
			lines := strings.Split(ansiEscape.ReplaceAllString(m.View(), ""), "\n")
			status := ""
			at := -1
			for i, line := range lines {
				if strings.HasPrefix(strings.TrimPrefix(line, strings.Repeat(" ", viewPaddingCols)), block) {
					status, at = line, i
					break
				}
			}
			if at < 0 {
				t.Fatalf("%s draws no row that leads with the status bar %q:\n%s", c.name, block, strings.Join(lines, "\n"))
			}
			// Everything after the block on that row is the rule and the padding:
			// the state replaces the rule's first cells, it does not sit beside
			// another thing.
			tail := strings.TrimPrefix(status[strings.Index(status, block)+len(block):], " ")
			if rest := strings.TrimRight(tail, " "); strings.Trim(rest, "─") != "" {
				t.Errorf("%s's status row carries something besides the rule after the block: %q", c.name, status)
			}
			// The legend is below it and ends at the frame's right edge, so the keys
			// keep the whole row the rule gave them.
			if at >= len(lines)-1 {
				return
			}
			last := strings.TrimRight(lines[len(lines)-1], " ")
			if pad := lipgloss.Width(last) - lipgloss.Width(strings.TrimLeft(last, " ")); pad < 1 {
				t.Errorf("%s does not right-align its key hints under the status bar: %q", c.name, last)
			}
		})
	}
}

// rowStartingWith returns the first rendered row that leads with prefix once the
// frame's padding is removed.
func rowStartingWith(view, prefix string) (string, bool) {
	for _, line := range strings.Split(ansiEscape.ReplaceAllString(view, ""), "\n") {
		row := strings.TrimRight(strings.TrimPrefix(line, strings.Repeat(" ", viewPaddingCols)), " ")
		if strings.HasPrefix(row, prefix) {
			return row, true
		}
	}
	return "", false
}
