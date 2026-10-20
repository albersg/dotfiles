package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/albersg/dotfiles/installer/internal/updates"
)

// The actions of the update center used to be the tail of the same list the
// packages are in, so the first screenful of a real machine pushed them past the
// bottom of the frame while the footer still advertised them: the only way to
// apply, clear or filter the selection was off screen. This drives the real
// renderer with a list longer than the frame and reads the frame back.
func TestTheUpdateCenterKeepsItsActionsOnScreenAtTheFloor(t *testing.T) {
	m := installerFrameModel(t, ScreenUpdateCenter)

	items := make([]updates.Item, 0, 40)
	for i := 0; i < 40; i++ {
		items = append(items, updates.Item{
			Manager:   updates.ManagerApt,
			Scope:     updates.ScopeSystem,
			Name:      fmt.Sprintf("package-%02d", i),
			Current:   "1.0",
			Available: "1.1",
		})
	}
	m.UpdateCenter = updateCenterState{
		Engine:    &updates.Center{Env: updates.Env{GOOS: "linux", SudoAvailable: true}},
		Resolved:  true,
		CheckedAt: time.Date(2026, time.October, 4, 12, 0, 0, 0, time.UTC),
		Channels:  []updates.Channel{{Manager: updates.ManagerApt, Label: "apt", Scope: updates.ScopeSystem, Available: true}},
		Order:     []updates.ManagerID{updates.ManagerApt},
		Results: []updates.Result{{
			Manager: updates.ManagerApt,
			Status:  updates.StatusUpdates,
			Command: "apt-get --simulate upgrade",
			Items:   items,
		}},
		Notice: "40 updates are available.",
	}
	m.Width, m.Height = 80, 24

	view := ansiEscape.ReplaceAllString(m.View(), "")
	for _, want := range []string{updateCenterSelectVisibleRowPrefix, updateCenterCheckRow, "← Back"} {
		if !strings.Contains(view, want) {
			t.Errorf("the 80x24 frame with 40 packages does not show %q:\n%s", want, view)
		}
	}
}
