package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// TestUpdateCenterWideDrawsTheReferenceTableAndDetail pins the wide shape the
// reference draws: the grouped table with its header row and its own counts, the
// selected entry's card and the actions card. The detail is checked for the fact
// the table row cannot hold (the command the answer was read with) and for the
// absence of the two facts the row already states.
func TestUpdateCenterWideDrawsTheReferenceTableAndDetail(t *testing.T) {
	m := updateCenterFrameCase(t)
	m.Width, m.Height = 160, 50
	m.Animating = false

	view := m.View()
	plain := ansiEscape.ReplaceAllString(view, "")

	for _, want := range []string{"Package", "Current → New version", "Manager", "Scope", "Actions", "Read with"} {
		if !strings.Contains(plain, want) {
			t.Errorf("the wide update center does not draw %q:\n%s", want, plain)
		}
	}
	if !strings.Contains(plain, "updates)") {
		t.Errorf("the grouped table does not carry a group count:\n%s", plain)
	}
	if strings.Contains(plain, "Name        ") || strings.Contains(plain, "Manager     ") {
		t.Errorf("the detail repeats a fact the row already states:\n%s", plain)
	}
	if rows := renderedRowCount(view); rows > m.Height {
		t.Errorf("the wide update center draws %d rows at 160x50, want <= %d", rows, m.Height)
	}
	for _, line := range strings.Split(plain, "\n") {
		if w := lipgloss.Width(line); w > m.Width {
			t.Errorf("the wide update center draws a %d-column line, want <= %d: %q", w, m.Width, line)
		}
	}
}

// TestUpdateCenterFooterNamesTheRunOrTheCount pins the footer's middle line: a
// read in flight says which manager is being read, and a settled list says how
// many entries the managers reported. It is empty before the first read.
func TestUpdateCenterFooterNamesTheRunOrTheCount(t *testing.T) {
	m := updateCenterFrameCase(t)
	if got := m.updateCenterFooterStatus(); got == "" {
		t.Errorf("a checked update center has no footer status")
	}

	m.UpdateCenter.CheckedAt = time.Time{}
	if got := m.updateCenterFooterStatus(); got != "" {
		t.Errorf("an unread update center states %q, want no status", got)
	}
}
