package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/diskclean"
	"github.com/albersg/dotfiles/installer/internal/system"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ============================================================================
// THE DISK CLEANER SCREEN, DRIVEN THE WAY A USER DRIVES IT
// ============================================================================
//
// These tests press keys and run the commands those presses start, with a
// probe injected into the model: nothing here scans this machine, removes a
// real store, or runs a real package manager.

// diskCleanerModel is a model sitting on the disk cleaner, with its machine
// interface replaced by a temporary home that holds one npm cache.
func diskCleanerModel(t *testing.T) (Model, string) {
	t.Helper()
	m := NewModel()
	m.Screen = ScreenDiskCleaner

	home := t.TempDir()
	probe := diskclean.DefaultProbe()
	probe.HomeDir = home
	probe.Getenv = func(name string) string {
		switch name {
		case "HOME":
			return home
		case "XDG_CACHE_HOME":
			return filepath.Join(home, ".cache")
		case "XDG_DATA_HOME":
			return filepath.Join(home, ".local", "share")
		}
		return ""
	}
	probe.LookPath = func(name string) (string, error) {
		if name == "npm" {
			return "/usr/bin/npm", nil
		}
		return "", errors.New("not found")
	}
	probe.Run = func(context.Context, string, ...string) ([]byte, error) {
		return nil, errors.New("this test does not run commands")
	}
	m.DiskClean.Probe = &probe
	m.DiskClean.Cwd = ""
	return m, home
}

// diskCleanerScanned is the model after one scan has been started and its
// answer folded back in, which is the state every selection test starts from.
func diskCleanerScanned(t *testing.T) (Model, string) {
	t.Helper()
	m, home := diskCleanerModel(t)
	m, cmd := diskCleanerPress(t, m, diskCleanScanKey)
	if cmd == nil {
		t.Fatal("the scan row started no command")
	}
	m = diskCleanerFeed(t, m, diskCleanerRun(t, cmd))
	if len(m.DiskClean.Items) == 0 {
		t.Fatal("the scan found nothing in a home with an npm cache")
	}
	return m, home
}

// diskCleanerPress sends one key and returns the model the update produced.
func diskCleanerPress(t *testing.T, m Model, key string) (Model, tea.Cmd) {
	t.Helper()
	var msg tea.KeyMsg
	switch key {
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEsc}
	case "up":
		msg = tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		msg = tea.KeyMsg{Type: tea.KeyDown}
	case " ":
		msg = tea.KeyMsg{Type: tea.KeySpace}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
	updated, cmd := m.Update(msg)
	next, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want the model", updated)
	}
	return next, cmd
}

// diskCleanerFeed folds one message back into the model.
func diskCleanerFeed(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	updated, _ := m.Update(msg)
	next, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want the model", updated)
	}
	return next
}

// diskCleanerRun runs a command the way the runtime does.
func diskCleanerRun(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("no command was started")
	}
	return cmd()
}

// TestDiskCleanerGroupsTheFindingsByToolAndProject pins the grouping the request
// asks for: the findings are listed under the tool that owns them, the artifacts
// of an added project directory are listed under that project instead of under
// the word "project", the headings are ordered by name, every finding sits under
// its own heading, the findings within a heading are largest first, two projects
// whose last directory has the same name stay in two groups, and a heading is
// inert -- the cursor can rest on it and no key changes anything through it.
func TestDiskCleanerGroupsTheFindingsByToolAndProject(t *testing.T) {
	m, _ := diskCleanerModel(t)
	m.Width, m.Height = 200, 80

	m.DiskClean.Items = []diskclean.Item{
		diskCleanToolItem("pip-cache", "The pip cache", "pip", 5<<20),
		diskCleanToolItem("npm-cache", "The npm cache", "npm", 1<<20),
		diskCleanToolItem("go-modules", "The module cache", "go", 3<<20),
		diskCleanToolItem("npm-other", "Another npm store", "npm", 7<<20),
		diskCleanProjectItem("app-residue", "/home/probe/apps/app", 2<<20),
		diskCleanProjectItem("app-residue-two", "/home/probe/other/app", 4<<20),
	}
	m.DiskClean.Scanned = true

	var headings []string
	under := map[string]string{}
	headingRow := map[string]int{}
	order := map[string][]string{}
	current := ""
	for index, row := range m.diskCleanRows() {
		switch row.action {
		case diskCleanActionGroup:
			current = row.label
			headings = append(headings, row.label)
		case diskCleanActionItem:
			item := m.DiskClean.Items[row.item]
			under[item.ID] = current
			headingRow[item.ID] = index
			order[current] = append(order[current], item.ID)
		}
	}

	want := []string{"app — 1 finding", "app — 1 finding", "go — 1 finding", "npm — 2 findings", "pip — 1 finding"}
	if strings.Join(headings, " | ") != strings.Join(want, " | ") {
		t.Errorf("the headings are %v, want %v", headings, want)
	}
	expected := map[string]string{
		"pip-cache":       "pip — 1 finding",
		"npm-cache":       "npm — 2 findings",
		"npm-other":       "npm — 2 findings",
		"go-modules":      "go — 1 finding",
		"app-residue":     "app — 1 finding",
		"app-residue-two": "app — 1 finding",
	}
	for id, heading := range expected {
		if under[id] != heading {
			t.Errorf("%s is listed under %q, want %q", id, under[id], heading)
		}
	}
	if headingRow["app-residue"] == 0 || headingRow["app-residue"] == headingRow["app-residue-two"] {
		t.Errorf("the two projects whose last directory is named the same share one heading (rows %d and %d)",
			headingRow["app-residue"], headingRow["app-residue-two"])
	}
	if got := strings.Join(order["npm — 2 findings"], ","); got != "npm-other,npm-cache" {
		t.Errorf("the npm findings are ordered %q, want the largest one first", got)
	}

	// A heading is a heading: nothing is selected or run through it.
	cursor := -1
	for index, row := range m.diskCleanRows() {
		if row.action == diskCleanActionGroup {
			cursor = index
			break
		}
	}
	if cursor < 0 {
		t.Fatal("no heading row to rest the cursor on")
	}
	m.Cursor = cursor
	before := m.DiskClean.Selection.Count(m.DiskClean.Items)
	for _, key := range []string{" ", "enter", "a"} {
		m, cmd := diskCleanerPress(t, m, key)
		if cmd != nil {
			t.Errorf("%q on a heading ran something", key)
		}
		if m.DiskClean.Selection.Count(m.DiskClean.Items) != before {
			t.Errorf("%q on a heading selected something", key)
		}
		if m.DiskClean.Phase != diskCleanList {
			t.Errorf("%q on a heading left the list", key)
		}
	}
}

// diskCleanProjectItem is a finding that belongs to a project directory the user
// added, which is what the project headings are built from: the detector names
// the tool "project" and records the project root.
func diskCleanProjectItem(id, root string, size int64) diskclean.Item {
	return diskclean.Item{
		ID: id, Tool: "project", Title: "node_modules in " + root,
		Category: diskclean.CategoryDependencies, Risk: diskclean.RiskCareful,
		Roots: []string{root}, Paths: []string{root + "/node_modules"},
		Reason:    "the project directory was added to the scope on purpose",
		Detail:    "the dependencies of that project, which an install downloads again",
		Size:      diskclean.Size{Logical: size, Physical: size},
		Mechanism: diskclean.Mechanism{Kind: diskclean.MechanismRemove},
	}
}

// diskCleanToolItem is a finding owned by one tool, which is what the grouping
// headings are built from.
func diskCleanToolItem(id, title, tool string, size int64) diskclean.Item {
	return diskclean.Item{
		ID: id, Tool: tool, Title: title, Category: diskclean.CategoryCache, Risk: diskclean.RiskSafe,
		Reason: "the scan found " + title + " for " + tool,
		Size:   diskclean.Size{Logical: size, Physical: size},
		Mechanism: diskclean.Mechanism{Kind: diskclean.MechanismReportOnly,
			Reason: tool + " cannot be cleaned from here"},
	}
}

// diskCleanerCursorOnItem puts the cursor on the row of one finding.
func diskCleanerCursorOnItem(t *testing.T, m Model, itemID string) Model {
	t.Helper()
	for index, row := range m.diskCleanRows() {
		if row.action == diskCleanActionItem && m.DiskClean.Items[row.item].ID == itemID {
			m.Cursor = index
			return m
		}
	}
	t.Fatalf("no row stands for %s", itemID)
	return m
}

// diskCleanerCursorOn puts the cursor on the first row with an action.
func diskCleanerCursorOn(t *testing.T, m Model, action diskCleanAction) Model {
	t.Helper()
	for index, row := range m.diskCleanRows() {
		if row.action == action {
			m.Cursor = index
			return m
		}
	}
	t.Fatalf("no row stands for action %v", action)
	return m
}

// diskCleanerGrammarState is everything a press can change on this screen, in one
// string, so a test can say a key changed nothing without comparing two models that
// carry command closures.
func diskCleanerGrammarState(m Model) string {
	return fmt.Sprintf("screen=%v phase=%v scanning=%v scanned=%v selected=%d roots=%v notice=%q",
		m.Screen, m.DiskClean.Phase, m.DiskClean.Scanning, m.DiskClean.Scanned,
		m.DiskClean.Selection.Count(m.DiskClean.Items), m.DiskClean.ProjectRoots, m.DiskClean.Notice)
}

// diskCleanerWithFinding is a scanned screen holding one finding, which is the
// state that offers the switch rows and, once the finding is marked, the dry run.
func diskCleanerWithFinding(t *testing.T) Model {
	t.Helper()
	m, _ := diskCleanerModel(t)
	m.Width, m.Height = 200, 80
	m.DiskClean.Items = []diskclean.Item{
		diskCleanRemovalItem("store", "The store", filepath.Join(t.TempDir(), "store"), 4096),
	}
	m.DiskClean.Scanned = true
	m.DiskClean.Selection = diskclean.Selection{}
	return m
}

// diskCleanerWithSelection is the same screen with the finding marked, which is the
// state that offers the dry run's own row.
func diskCleanerWithSelection(t *testing.T) Model {
	t.Helper()
	m := diskCleanerWithFinding(t)
	m.DiskClean.Selection = diskclean.Selection{"store": true}
	return m
}

// TestDiskCleanerRowsDeclareTheKeyThatRunsThem pins the key grammar on this screen
// (odd/tasks/key-grammar.md): every row that acts where it stands carries the key
// that runs it, the footer names the same key while the row is offered, Enter runs
// none of them, and Space is only ever a switch's key.
func TestDiskCleanerRowsDeclareTheKeyThatRunsThem(t *testing.T) {
	cases := []struct {
		name   string
		action diskCleanAction
		build  func(t *testing.T) Model
		// footer is the hint the legend has to carry while this row is on screen.
		footer string
	}{
		{"the scan row", diskCleanActionScan, func(t *testing.T) Model {
			m, _ := diskCleanerModel(t)
			m.Width, m.Height = 200, 80
			return m
		}, "[s] scan"},
		{"a finding's switch", diskCleanActionItem, diskCleanerWithFinding, "[Space] select or drop"},
		{"the dry run's row", diskCleanActionPreview, diskCleanerWithSelection, "[p] preview"},
		{"the confirmation's row", diskCleanActionConfirm, func(t *testing.T) Model {
			return diskCleanerWithSelection(t).startDiskCleanPreview()
		}, "[c] clean"},
		{"the project directory's row", diskCleanActionAddRoot, func(t *testing.T) Model {
			m := diskCleanerWithSelection(t)
			m.DiskClean.Cwd = filepath.Join(t.TempDir(), "project")
			return m
		}, "[d] add the project directory"},
		{"the forgetting row", diskCleanActionForgetRoot, func(t *testing.T) Model {
			m := diskCleanerWithSelection(t)
			m.DiskClean.ProjectRoots = []string{"/home/probe/apps/app"}
			return m
		}, "[f] forget the project directory"},
		{"the way out of the preview", diskCleanActionCancelPreview, func(t *testing.T) Model {
			return diskCleanerWithSelection(t).startDiskCleanPreview()
		}, "[Esc] back"},
		{"the way to stop the run", diskCleanActionCancelRun, func(t *testing.T) Model {
			m := diskCleanerWithSelection(t)
			m.DiskClean.Phase = diskCleanRunning
			m.DiskClean.Cancel = func() {}
			return m
		}, "[Esc] stop the run"},
		{"the way back", diskCleanActionBack, func(t *testing.T) Model {
			m, _ := diskCleanerModel(t)
			m.Width, m.Height = 200, 80
			return m
		}, "[Esc] back"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.build(t)
			m = diskCleanerCursorOn(t, m, tc.action)
			row := m.diskCleanRows()[m.Cursor]
			key := diskCleanRowKey(row.label)
			if key == "" {
				t.Fatalf("the row %q carries no mark, so nothing on it says what presses it", row.label)
			}

			switch key {
			case " ":
				// A switch shows the state Space turns over.
				if !strings.HasPrefix(row.label, diskCleanSwitchOffMark) && !strings.HasPrefix(row.label, diskCleanSwitchOnMark) {
					t.Errorf("the row %q is pressed with Space but does not show the state it holds", row.label)
				}
			case "esc":
				if !strings.HasPrefix(row.label, diskCleanBackMark) {
					t.Errorf("the row %q is pressed with Escape but carries no arrow saying so", row.label)
				}
			case "j", "k", "h", "l":
				t.Errorf("the row %q runs on %q, which is a movement key", row.label, key)
			default:
				if !strings.HasPrefix(row.label, "["+key+"] ") {
					t.Errorf("the row %q declares %q, want the key written on the row", row.label, key)
				}
			}

			before := diskCleanerGrammarState(m)
			// Enter opens a screen, a row's detail or a value, and this screen has none
			// of the three: on any row here it runs nothing.
			entered, cmd := diskCleanerPress(t, m, "enter")
			if cmd != nil || diskCleanerGrammarState(entered) != before {
				t.Errorf("Enter ran the row %q: %s became %s", row.label, before, diskCleanerGrammarState(entered))
			}
			// Space is a switch's key and nothing else's, so a row that is not a switch
			// ignores it. Each press starts from the same model.
			if key != " " {
				if spaced, cmd := diskCleanerPress(t, m, " "); cmd != nil || diskCleanerGrammarState(spaced) != before {
					t.Errorf("Space acted on the row %q, which is not a switch: %s became %s",
						row.label, before, diskCleanerGrammarState(spaced))
				}
			}

			pressed, cmd := diskCleanerPress(t, m, key)
			if cmd == nil && diskCleanerGrammarState(pressed) == before {
				t.Errorf("the row %q carries %q and pressing it changed nothing", row.label, key)
			}

			// The legend names the same key while the row is offered, so the row and the
			// footer cannot disagree about what presses it.
			if view := diskCleanWholeView(m); !strings.Contains(view, tc.footer) {
				t.Errorf("the footer does not name the key of %q (%q):\n%s", row.label, tc.footer, view)
			}
		})
	}
}

// TestDiskCleanerFooterSaysEscapeStopsTheScan pins the other half of Escape's one
// meaning: while a scan is in flight the key stops it rather than leaving the
// screen, and the footer says which of the two it is doing.
func TestDiskCleanerFooterSaysEscapeStopsTheScan(t *testing.T) {
	m, _ := diskCleanerModel(t)
	m.Width, m.Height = 200, 60
	m, cmd := diskCleanerPress(t, m, diskCleanScanKey)
	if cmd == nil {
		t.Fatal("the scan row started no command")
	}
	if !m.DiskClean.Scanning {
		t.Fatal("the scan is not in flight, so there is nothing to stop")
	}
	if view := diskCleanWholeView(m); !strings.Contains(view, "[Esc] stop the scan") {
		t.Errorf("the footer does not say that Escape stops the scan:\n%s", view)
	}
}

// TestDiskCleanerScanIsACommandAndItsAnswerFillsTheScreen pins the shape of a
// scan: a press starts a command, the screen says a scan is in flight, and the
// answer replaces the findings and leaves nothing selected.
func TestDiskCleanerScanIsACommandAndItsAnswerFillsTheScreen(t *testing.T) {
	m, home := diskCleanerModel(t)
	writeTestFile(t, filepath.Join(home, ".npm", "_cacache", "blob.bin"), 8192)

	m, cmd := diskCleanerPress(t, m, diskCleanScanKey)
	if !m.DiskClean.Scanning {
		t.Error("the screen does not say a scan is in flight")
	}
	if !strings.Contains(m.DiskClean.Notice, "Scanning") {
		t.Errorf("the notice line does not name the scan: %q", m.DiskClean.Notice)
	}

	msg, ok := diskCleanerRun(t, cmd).(diskCleanScanMsg)
	if !ok {
		t.Fatalf("the scan command returned %T", diskCleanerRun(t, cmd))
	}
	m = diskCleanerFeed(t, m, msg)

	if m.DiskClean.Scanning {
		t.Error("the screen still says a scan is in flight after its answer arrived")
	}
	if !m.DiskClean.Scanned {
		t.Error("the screen does not remember that it has scanned")
	}
	if m.DiskClean.Cancel != nil {
		t.Error("the finished scan left a way to cancel it")
	}
	if !diskCleanContains(itemIDs(m.DiskClean.Items), "npm-cache") {
		t.Fatalf("the scan found %v, want the npm cache", itemIDs(m.DiskClean.Items))
	}
	if got := m.DiskClean.Selection.Count(m.DiskClean.Items); got != 0 {
		t.Errorf("%d findings were selected by the scan itself", got)
	}
	if !strings.Contains(m.DiskClean.Notice, "Found") {
		t.Errorf("the notice does not report what was found: %q", m.DiskClean.Notice)
	}
}

// TestDiskCleanerStartsNoSecondScanWhileOneRuns pins one scan at a time: a
// second press would measure the same machine twice and leave whichever answer
// arrived last on screen.
func TestDiskCleanerStartsNoSecondScanWhileOneRuns(t *testing.T) {
	m, _ := diskCleanerModel(t)
	m, cmd := diskCleanerPress(t, m, diskCleanScanKey)
	if cmd == nil {
		t.Fatal("the first press started no scan")
	}
	m, second := diskCleanerPress(t, m, diskCleanScanKey)
	if second != nil {
		t.Error("a second scan was started while the first was running")
	}
}

// TestDiskCleanerSelectsNothingForTheUser pins the rule the whole utility rests
// on: a scan leaves every checkbox empty, the screen offers no preview of a
// selection nobody made, and a delicate finding is not selected by a bulk key.
func TestDiskCleanerSelectsNothingForTheUser(t *testing.T) {
	m, home := diskCleanerModel(t)
	writeTestFile(t, filepath.Join(home, ".npm", "_cacache", "blob.bin"), 4096)
	m.DiskClean.Items = []diskclean.Item{
		{
			ID: "npm-cache", Tool: "npm", Title: "The npm cache", Risk: diskclean.RiskSafe,
			Size: diskclean.Size{Logical: 4096, Physical: 4096},
		},
		{
			ID: "docker-volumes", Tool: "docker", Title: "The container volumes", Risk: diskclean.RiskDelicate,
			Size:      diskclean.Size{Logical: 1 << 30, Physical: 1 << 30, Source: diskclean.SourceTool},
			Mechanism: diskclean.Mechanism{Kind: diskclean.MechanismTool, Program: "docker", Args: []string{"volume", "prune"}},
		},
	}
	m.DiskClean.Scanned = true
	m.DiskClean.Selection = diskclean.Selection{}

	for _, row := range m.diskCleanRows() {
		if strings.HasPrefix(row.label, "[x]") {
			t.Errorf("this row is marked before the user marked anything: %s", row.label)
		}
		if row.action == diskCleanActionPreview {
			t.Errorf("the screen offers to preview a selection nobody made: %s", row.label)
		}
	}

	m, _ = diskCleanerPress(t, m, "a")
	if !m.DiskClean.Selection.Has("npm-cache") {
		t.Error("the bulk key did not select the cache that is safe to rebuild")
	}
	if m.DiskClean.Selection.Has("docker-volumes") {
		t.Error("the bulk key selected a delicate finding")
	}

	m = diskCleanerCursorOnItem(t, m, "docker-volumes")
	m, _ = diskCleanerPress(t, m, " ")
	if !m.DiskClean.Selection.Has("docker-volumes") {
		t.Error("marking a delicate finding on purpose did not select it")
	}
}

// TestDiskCleanerPreviewComesBeforeAnythingRuns pins the two-step rule: the
// preview builds the plan and runs nothing, the store is still there
// afterwards, and only the confirmation starts the run.
func TestDiskCleanerPreviewComesBeforeAnythingRuns(t *testing.T) {
	m, _ := diskCleanerModel(t)
	store := filepath.Join(t.TempDir(), "npm-cache")
	writeTestFile(t, filepath.Join(store, "blob.bin"), 4096)

	m.DiskClean.Items = []diskclean.Item{diskCleanRemovalItem("store", "The npm cache", store, 4096)}
	m.DiskClean.Scanned = true
	m.DiskClean.Selection = diskclean.Selection{"store": true}

	m = diskCleanerCursorOn(t, m, diskCleanActionPreview)
	m, cmd := diskCleanerPress(t, m, diskCleanPreviewKey)
	if cmd != nil {
		t.Error("the preview ran something")
	}
	if m.DiskClean.Phase != diskCleanPreview {
		t.Fatalf("the phase after the preview is %v, want the preview", m.DiskClean.Phase)
	}
	if !m.DiskClean.Planned || len(m.DiskClean.Plan.Items) != 1 {
		t.Fatalf("the preview built no plan: %+v", m.DiskClean.Plan)
	}
	if _, err := os.Lstat(store); err != nil {
		t.Errorf("the preview removed the store: %v", err)
	}

	rows := m.diskCleanRows()
	confirm, cancel := -1, -1
	for index, row := range rows {
		switch row.action {
		case diskCleanActionConfirm:
			confirm = index
		case diskCleanActionCancelPreview:
			cancel = index
		}
	}
	if confirm < 0 || cancel < 0 {
		t.Fatalf("the preview offers %v, want a confirmation and a way back", diskCleanRowLabels(rows))
	}
	if confirm > cancel {
		t.Errorf("the confirmation is below the way back in %v", diskCleanRowLabels(rows))
	}
}

// TestDiskCleanerEscapeLeavesThePreviewWithoutRunningIt pins the way out of a
// preview: the plan is dropped rather than left to be confirmed by a later
// press, and nothing ran.
func TestDiskCleanerEscapeLeavesThePreviewWithoutRunningIt(t *testing.T) {
	m, _ := diskCleanerModel(t)
	store := filepath.Join(t.TempDir(), "npm-cache")
	writeTestFile(t, filepath.Join(store, "blob.bin"), 2048)
	m.DiskClean.Items = []diskclean.Item{diskCleanRemovalItem("store", "The store", store, 2048)}
	m.DiskClean.Scanned = true
	m.DiskClean.Selection = diskclean.Selection{"store": true}

	m = diskCleanerCursorOn(t, m, diskCleanActionPreview)
	m, _ = diskCleanerPress(t, m, diskCleanPreviewKey)
	m, _ = diskCleanerPress(t, m, "esc")

	if m.DiskClean.Phase != diskCleanList {
		t.Errorf("escape left the screen in phase %v", m.DiskClean.Phase)
	}
	if m.DiskClean.Planned || len(m.DiskClean.Plan.Items) != 0 {
		t.Error("escape left the plan in place")
	}
	if _, err := os.Lstat(store); err != nil {
		t.Errorf("escape removed something: %v", err)
	}
}

// TestDiskCleanerConfirmationRunsThePlanAndShowsTheResult pins the run: the
// confirmation starts a command, the screen says a run is in flight, and the
// answer becomes the result phase with the measured release.
func TestDiskCleanerConfirmationRunsThePlanAndShowsTheResult(t *testing.T) {
	m, _ := diskCleanerModel(t)
	store := filepath.Join(t.TempDir(), "npm-cache")
	writeTestFile(t, filepath.Join(store, "blob.bin"), 4096)

	remover := &diskCleanRecordingRemover{}
	probe := *m.DiskClean.Probe
	probe.Remove = remover.remove
	m.DiskClean.Probe = &probe
	m.DiskClean.Items = []diskclean.Item{diskCleanRemovalItem("store", "The store", store, 4096)}
	m.DiskClean.Scanned = true
	m.DiskClean.Selection = diskclean.Selection{"store": true}

	m = diskCleanerCursorOn(t, m, diskCleanActionPreview)
	m, _ = diskCleanerPress(t, m, diskCleanPreviewKey)
	m = diskCleanerCursorOn(t, m, diskCleanActionConfirm)
	m, cmd := diskCleanerPress(t, m, diskCleanCleanKey)

	if cmd == nil {
		t.Fatal("the confirmation started no run")
	}
	if m.DiskClean.Phase != diskCleanRunning {
		t.Errorf("the phase during the run is %v", m.DiskClean.Phase)
	}

	m = diskCleanerFeed(t, m, diskCleanerRun(t, cmd))
	if m.DiskClean.Phase != diskCleanResults {
		t.Fatalf("the phase after the run is %v, want the results", m.DiskClean.Phase)
	}
	report := m.DiskClean.CleanReport
	if len(report.Results) != 1 || !report.Results[0].Succeeded() {
		t.Fatalf("the run reported %+v", report.Results)
	}
	if report.Results[0].Freed <= 0 {
		t.Errorf("the run freed nothing from a store that was emptied: %+v", report.Results[0])
	}
	if report.FreedMeasured && report.FreedBytes <= 0 {
		t.Errorf("the report claims a measurement and reports %d bytes", report.FreedBytes)
	}
	if !report.FreedMeasured && report.FreedNote == "" {
		t.Error("the report has no release and no reason for one")
	}
	if report.FreedMeasured && !strings.Contains(m.DiskClean.Notice, "released") {
		t.Errorf("the notice does not report the release: %q", m.DiskClean.Notice)
	}
	if len(remover.order) == 0 {
		t.Error("nothing was unlinked through the recording double")
	}
	if _, err := os.Lstat(store); !os.IsNotExist(err) {
		t.Errorf("the store is still there: %v", err)
	}
}

// TestDiskCleanerEscapeStopsTheRunInFlight pins cancellation from the screen:
// escape asks the command in flight to stop, the run's own answer says it was
// cancelled, and the screen shows the items it never reached.
func TestDiskCleanerEscapeStopsTheRunInFlight(t *testing.T) {
	m, _ := diskCleanerModel(t)
	store := filepath.Join(t.TempDir(), "store")
	writeTestFile(t, filepath.Join(store, "data.bin"), 4096)
	m.DiskClean.Items = []diskclean.Item{diskCleanRemovalItem("store", "The store", store, 4096)}
	m.DiskClean.Scanned = true
	m.DiskClean.Selection = diskclean.Selection{"store": true}

	m = diskCleanerCursorOn(t, m, diskCleanActionPreview)
	m, _ = diskCleanerPress(t, m, diskCleanPreviewKey)
	m = diskCleanerCursorOn(t, m, diskCleanActionConfirm)
	m, cmd := diskCleanerPress(t, m, diskCleanCleanKey)
	m, _ = diskCleanerPress(t, m, "esc")

	if m.DiskClean.Notice != "Stopping…" {
		t.Errorf("escape did not report that it is stopping the run: %q", m.DiskClean.Notice)
	}
	m = diskCleanerFeed(t, m, diskCleanerRun(t, cmd))
	if !m.DiskClean.CleanReport.Cancelled {
		t.Error("the run did not report the cancellation")
	}
	if m.DiskClean.CleanReport.FreedBytes != 0 {
		t.Errorf("a cancelled run reports %d bytes freed", m.DiskClean.CleanReport.FreedBytes)
	}
	if !strings.Contains(m.DiskClean.Notice, "stopped") {
		t.Errorf("the notice does not say the run was stopped: %q", m.DiskClean.Notice)
	}
}

// TestDiskCleanerEscapeStopsAScanInFlight pins the same for a scan, including
// that its answer is marked as the partial thing it is.
func TestDiskCleanerEscapeStopsAScanInFlight(t *testing.T) {
	m, _ := diskCleanerModel(t)
	m, cmd := diskCleanerPress(t, m, diskCleanScanKey)
	m, _ = diskCleanerPress(t, m, "esc")

	msg, ok := diskCleanerRun(t, cmd).(diskCleanScanMsg)
	if !ok {
		t.Fatal("the scan command returned something else")
	}
	if !msg.cancelled {
		t.Error("the scan did not notice it was cancelled")
	}
	m = diskCleanerFeed(t, m, msg)
	if m.DiskClean.Scanning {
		t.Error("the screen still says a scan is in flight")
	}
	if !strings.Contains(m.DiskClean.Notice, "cancelled") {
		t.Errorf("the notice does not say the scan was cancelled: %q", m.DiskClean.Notice)
	}
}

// TestDiskCleanerAddsAndForgetsTheDirectoryItWasStartedFrom pins the project
// scope: the directory this run was started from is offered, adding it rescans
// with it, and forgetting it takes it back out.
func TestDiskCleanerAddsAndForgetsTheDirectoryItWasStartedFrom(t *testing.T) {
	m, _ := diskCleanerModel(t)
	project := t.TempDir()
	writeTestFile(t, filepath.Join(project, "package.json"), 32)
	writeTestFile(t, filepath.Join(project, "node_modules", "left-pad", "index.js"), 512)
	m.DiskClean.Cwd = project

	m = diskCleanerCursorOn(t, m, diskCleanActionAddRoot)
	m, cmd := diskCleanerPress(t, m, diskCleanAddRootKey)
	if !diskCleanHasRoot(m.DiskClean.ProjectRoots, project) {
		t.Fatalf("the project scope is %v, want it to hold %s", m.DiskClean.ProjectRoots, project)
	}
	if cmd == nil {
		t.Fatal("adding a project directory did not start a scan of the new scope")
	}
	m = diskCleanerFeed(t, m, diskCleanerRun(t, cmd))

	found := false
	for _, item := range m.DiskClean.Items {
		for _, root := range item.Roots {
			if root == project {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("the scan of the new scope found nothing in %s: %v", project, itemIDs(m.DiskClean.Items))
	}

	m = diskCleanerCursorOn(t, m, diskCleanActionForgetRoot)
	m, cmd = diskCleanerPress(t, m, diskCleanForgetRootKey)
	if diskCleanHasRoot(m.DiskClean.ProjectRoots, project) {
		t.Errorf("the project scope still holds %s", project)
	}
	if cmd == nil {
		t.Error("forgetting a project directory did not scan the smaller scope")
	}
}

// TestDiskCleanerRowCountsTheWorkNotTheSelection pins the confirmation row: it
// counts the items that would actually run, so a finding the plan refuses does
// not inflate the promise the row makes.
func TestDiskCleanerRowCountsTheWorkNotTheSelection(t *testing.T) {
	m, _ := diskCleanerModel(t)
	store := filepath.Join(t.TempDir(), "store")
	writeTestFile(t, filepath.Join(store, "data.bin"), 1024)
	m.DiskClean.Items = []diskclean.Item{
		diskCleanRemovalItem("store", "The store", store, 1024),
		{
			ID: "apt-archives", Tool: "apt", Title: "The apt cache", Risk: diskclean.RiskSafe,
			Size:      diskclean.Size{Logical: 1 << 20, Physical: 1 << 20},
			Mechanism: diskclean.Mechanism{Kind: diskclean.MechanismReportOnly, Reason: "apt is not installed on this machine"},
		},
	}
	m.DiskClean.Scanned = true
	m.DiskClean.Selection = diskclean.Selection{"store": true, "apt-archives": true}

	m = diskCleanerCursorOn(t, m, diskCleanActionPreview)
	m, _ = diskCleanerPress(t, m, diskCleanPreviewKey)

	rows := m.diskCleanRows()
	confirm := -1
	for index, row := range rows {
		if row.action == diskCleanActionConfirm {
			confirm = index
		}
	}
	if confirm < 0 {
		t.Fatalf("the preview rows are %v", diskCleanRowLabels(rows))
	}
	if !strings.Contains(rows[confirm].label, "1 item") {
		t.Errorf("the confirmation row promises %q, want one item", rows[confirm].label)
	}
	if len(m.DiskClean.Plan.Items) != 2 {
		t.Fatalf("the plan holds %d items, want both selected findings", len(m.DiskClean.Plan.Items))
	}
	blocked := 0
	for _, planned := range m.DiskClean.Plan.Items {
		if !planned.Runs() {
			blocked++
			if !strings.Contains(planned.Blocked, "apt is not installed") {
				t.Errorf("the blocked item does not carry the tool's own reason: %q", planned.Blocked)
			}
		}
	}
	if blocked != 1 {
		t.Errorf("%d items are blocked in the plan, want the report-only one", blocked)
	}
}

// TestDiskCleanerCannotSelectWhatNothingCanBeDoneAbout pins the inert checkbox:
// a finding with no mechanism is not selectable, and the screen says why rather
// than letting the mark do nothing.
func TestDiskCleanerCannotSelectWhatNothingCanBeDoneAbout(t *testing.T) {
	m, _ := diskCleanerModel(t)
	m.DiskClean.Items = []diskclean.Item{{
		ID: "apt-archives", Tool: "apt", Title: "The apt cache", Risk: diskclean.RiskSafe,
		Size:      diskclean.Size{Logical: 1 << 20, Physical: 1 << 20},
		Mechanism: diskclean.Mechanism{Kind: diskclean.MechanismReportOnly, Reason: "apt is not installed on this machine"},
	}}
	m.DiskClean.Scanned = true

	m = diskCleanerCursorOnItem(t, m, "apt-archives")
	m, _ = diskCleanerPress(t, m, " ")
	if m.DiskClean.Selection.Count(m.DiskClean.Items) != 0 {
		t.Error("a finding nothing can be done about was selected")
	}
	if !strings.Contains(m.DiskClean.Notice, "apt is not installed") {
		t.Errorf("the screen does not say why the finding cannot be cleaned: %q", m.DiskClean.Notice)
	}
}

// diskCleanFlatView is the screen's text with the escapes removed and every run
// of whitespace collapsed to one space, so an assertion can name a phrase that
// the frame happens to wrap across two lines. On a frame wide enough for two
// columns the card is drawn in the right column beside the list, so the helper
// reads that column alone: a phrase the card wraps would otherwise be split by a
// row of the list on the same line.
func diskCleanFlatView(m Model) string {
	plain := ansiEscape.ReplaceAllString(m.View(), "")
	l := layoutFor(m)
	text := plain
	if l.TwoColumn && m.sectionDetailPane() {
		start := viewPaddingCols + l.Leading + l.Left + layoutGutter
		var detail []string
		for _, line := range strings.Split(plain, "\n") {
			detail = append(detail, string(cellsFrom(line, start)))
		}
		text = strings.Join(detail, "\n")
	}
	return strings.Join(strings.Fields(text), " ")
}

// diskCleanWholeView is the whole frame flattened the old way: the header, the
// footer and both columns. It is what a footer or header assertion reads, because
// those rows are outside the detail column.
func diskCleanWholeView(m Model) string {
	return strings.Join(strings.Fields(ansiEscape.ReplaceAllString(m.View(), "")), " ")
}

// cellsFrom is the part of a rendered line that starts at the given cell column,
// measured the way the terminal measures it: a wide rune is two columns, an SGR
// escape is none. The helper reads a column of a composed frame, and a rune
// count would drift on the arrows and dashes the card carries.
func cellsFrom(line string, start int) []rune {
	runes := []rune(line)
	column := 0
	for i, r := range runes {
		if column >= start {
			return runes[i:]
		}
		column += lipgloss.Width(string(r))
	}
	return nil
}

// TestUtilitiesSectionOffersTheDevelopmentCacheCleaner pins the way in: the
// section lists the job, the panel names it, the rendered section shows the row,
// and choosing the row opens the screen without scanning anything on the way.
//
// It used to assert that the section's own body carried a sentence about this
// job. It does not, and that is the same trade main's update center made when the
// row before this one arrived (utilitiesDescription): at the frame the guards read
// the body holds the rows its jobs and its two absences need, so the newest job's
// line is the one that yields, and TestUtilitiesBodyCarriesEveryJobAtTheFloor is
// what keeps the rest of them there. What this job is and what it will never do is
// read on the screen the row opens, which is where the marks are made.
func TestUtilitiesSectionOffersTheDevelopmentCacheCleaner(t *testing.T) {
	m := installerFrameModel(t, ScreenUtilities)
	m.Width, m.Height = 120, 50

	rows := m.GetCurrentOptions()
	found := false
	for _, row := range rows {
		if row == utilitiesScreenRow(utilitiesDiskCleanerRow) {
			found = true
		}
	}
	if !found {
		t.Fatalf("the utilities section does not offer %q: %v", utilitiesDiskCleanerRow, rows)
	}

	view := ansiEscape.ReplaceAllString(m.View(), "")
	if !strings.Contains(view, utilitiesDiskCleanerRow) {
		t.Errorf("the rendered section does not show %q:\n%s", utilitiesDiskCleanerRow, view)
	}
	// The job's own sentence, on the screen the row opens rather than in the
	// section's body: the section names every job in its list, and the screen says
	// what the job does.
	screen := installerFrameModel(t, ScreenDiskCleaner)
	screen.Width, screen.Height = 120, 50
	if !strings.Contains(diskCleanFlatView(screen), "measures each store without removing anything") {
		t.Errorf("the screen does not say what the job does:\n%s", diskCleanFlatView(screen))
	}
	offered, flat := false, utilitiesPanelFlat(t, m)
	for _, entry := range m.utilitiesPanelEntries() {
		if entry.offered && entry.label == "Caches" {
			offered = true
		}
	}
	if !offered || !strings.Contains(flat, "Caches") {
		t.Errorf("the Utilities panel does not carry the Caches row (offered=%v):\n%s", offered, flat)
	}

	for index, row := range rows {
		if row == utilitiesScreenRow(utilitiesDiskCleanerRow) {
			m.Cursor = index
		}
	}
	next, cmd := m.handleUtilitiesKeys("enter")
	opened, ok := next.(Model)
	if !ok {
		t.Fatalf("the section returned %T", next)
	}
	if opened.Screen != ScreenDiskCleaner {
		t.Fatalf("choosing the row opened %v, want the disk cleaner", opened.Screen)
	}
	if cmd != nil {
		t.Error("opening the disk cleaner started work on the way in")
	}
}

// TestDiskCleanerScreenStatesWhatItWillNotDo pins the honest half of the screen:
// the tool's own command instead of its internal store, no quarantine, nothing
// preselected, no telemetry, and -- inside WSL -- the plain fact that freeing
// Linux space does not shrink the distribution's disk.
func TestDiskCleanerScreenStatesWhatItWillNotDo(t *testing.T) {
	m := installerFrameModel(t, ScreenDiskCleaner)
	m.Width, m.Height = 160, 70
	if m.SystemInfo == nil {
		m.SystemInfo = &system.SystemInfo{OS: system.OSLinux, OSName: "Linux"}
	}
	m.SystemInfo.IsWSL = true

	view := diskCleanFlatView(m)
	for _, want := range []string{
		"never deleted file by file",
		"engine's own prune command",
		"Nothing is quarantined",
		"never marked for you",
		"does not shrink on its own",
		"no telemetry",
		"Escape stops either one",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("the screen does not say %q:\n%s", want, view)
		}
	}

	// The same screen on a host outside WSL must not carry the WSL paragraph:
	// the note is about a machine the user is on, not about the utility.
	m.SystemInfo.IsWSL = false
	plain := diskCleanFlatView(m)
	if strings.Contains(plain, "does not shrink on its own") {
		t.Errorf("the screen tells a host outside WSL about the distribution's disk:\n%s", plain)
	}
}

// TestDiskCleanerPreviewRendersEachCommandAndEachRefusal pins what the dry run
// puts on screen: every item it is about is a row, and the card under the cursor
// says that item's exact command, its exact path for a removal, its reason and the
// path it covers -- or the reason nothing would run -- with the promise that
// nothing has run yet.
func TestDiskCleanerPreviewRendersEachCommandAndEachRefusal(t *testing.T) {
	m, _ := diskCleanerModel(t)
	m.Width, m.Height = 200, 80
	store := filepath.Join(t.TempDir(), "store")
	writeTestFile(t, filepath.Join(store, "data.bin"), 1024)

	npm := diskclean.Item{
		ID: "npm-cache", Tool: "npm", Title: "The npm cache", Category: diskclean.CategoryCache,
		Risk: diskclean.RiskSafe, Reason: "npm is on PATH and its cache directory exists",
		Detail: "the packages npm downloaded and unpacked, addressed by content",
		Paths:  []string{filepath.Join(m.DiskClean.Probe.HomeDir, ".npm", "_cacache")},
		Size:   diskclean.Size{Logical: 4096, Physical: 4096},
		Mechanism: diskclean.Mechanism{Kind: diskclean.MechanismTool, Program: "npm",
			Args: []string{"cache", "clean", "--force"}},
	}

	m.DiskClean.Items = []diskclean.Item{
		npm,
		diskCleanRemovalItem("store", "The store", store, 1024),
		{
			ID: "apt-archives", Tool: "apt", Title: "The apt cache", Risk: diskclean.RiskSafe,
			Reason: "the apt archive directory exists",
			Size:   diskclean.Size{Logical: 1 << 20, Physical: 1 << 20},
			Mechanism: diskclean.Mechanism{Kind: diskclean.MechanismReportOnly,
				Reason: "apt-get is not on PATH here"},
		},
	}
	m.DiskClean.Scanned = true
	m.DiskClean.Selection = diskclean.Selection{"npm-cache": true, "store": true, "apt-archives": true}

	m = diskCleanerCursorOn(t, m, diskCleanActionPreview)
	m, _ = diskCleanerPress(t, m, diskCleanPreviewKey)
	if m.DiskClean.Phase != diskCleanPreview {
		t.Fatalf("the preview did not open: %v", m.DiskClean.Phase)
	}

	checked := map[string]bool{}
	for index, row := range m.diskCleanRows() {
		if row.action != diskCleanActionPreviewItem {
			continue
		}
		item := m.DiskClean.Items[row.item]
		m.Cursor = index
		view := diskCleanFlatView(m)

		want := []string{"Found because"}
		switch item.ID {
		case "npm-cache":
			want = append(want, "would run `npm cache clean --force`",
				"npm is on PATH and its cache directory exists", "regenerable cache",
				"the packages npm downloaded and unpacked")
		case "store":
			want = append(want, "would remove "+store+" — irreversibly")
		case "apt-archives":
			want = append(want, "Nothing would run for this one.",
				"This finding is reported, not cleaned: apt-get is not on PATH here")
		default:
			t.Fatalf("the preview carries an item the test does not know: %s", item.ID)
		}
		for _, phrase := range want {
			if strings.Contains(view, phrase) {
				continue
			}
			// A path is one token and can be longer than the detail column: macOS
			// temp directories are, and the card wraps the token where it must. The
			// words are what the reader needs; where the wrap lands is the column's
			// business, so the assertion compares the text without its spaces.
			if strings.Contains(strings.ReplaceAll(view, " ", ""), strings.ReplaceAll(phrase, " ", "")) {
				continue
			}
			t.Errorf("the card for %s does not say %q:\n%s", item.ID, phrase, view)
		}
		checked[item.ID] = true
	}
	if len(checked) != 3 {
		t.Fatalf("the preview listed %d items, want all three: %v", len(checked), checked)
	}

	// The preview promises that nothing has run yet, and the row for every planned
	// item says what the confirmation would do to it.
	rows := diskCleanRowLabels(m.diskCleanRows())
	joined := strings.Join(rows, " | ")
	if !strings.Contains(joined, "would run npm cache clean --force") ||
		!strings.Contains(joined, "would remove "+store) ||
		!strings.Contains(joined, "nothing would run") {
		t.Errorf("the preview rows do not carry each item's action: %s", joined)
	}
	view := diskCleanFlatView(m)
	for _, want := range []string{"Preview: nothing has run yet. 2 items would run and 1 item would not"} {
		if !strings.Contains(view, want) {
			t.Errorf("the preview does not say %q:\n%s", want, view)
		}
	}
}

// TestDiskCleanerShowsWhyEachFindingWasFoundAndWhereItIs pins S3 on the findings
// list: the card of the finding under the cursor names its category, its risk,
// what it holds and where that number came from, the evidence that found it, the
// paths or engine objects it covers, and what removing it would cost to get back
// -- and moving the cursor moves the card.
func TestDiskCleanerShowsWhyEachFindingWasFoundAndWhereItIs(t *testing.T) {
	m, home := diskCleanerModel(t)
	m.Width, m.Height = 200, 80

	m.DiskClean.Items = []diskclean.Item{
		{
			ID: "npm-cache", Tool: "npm", Title: "The npm cache", Category: diskclean.CategoryCache,
			Risk:   diskclean.RiskSafe,
			Reason: "npm is on PATH and its cache directory exists",
			Detail: "the packages npm downloaded and unpacked, addressed by content",
			Paths:  []string{filepath.Join(home, ".npm", "_cacache")},
			Size:   diskclean.Size{Logical: 6 << 20, Physical: 4 << 20, Files: 12},
			Mechanism: diskclean.Mechanism{Kind: diskclean.MechanismTool, Program: "npm",
				Args: []string{"cache", "clean", "--force"}},
		},
		{
			ID: "docker-volumes", Tool: "docker", Title: "The container volumes",
			Category: diskclean.CategoryVolumes, Risk: diskclean.RiskDelicate,
			Reason:          "docker is on PATH and answered `system df` for the volumes",
			Detail:          "the volumes a container wrote: data the engine owns and may not be reproducible",
			VolatileObjects: []string{"volumes"},
			Size: diskclean.Size{Logical: 1 << 30, Physical: 1 << 30,
				Source: diskclean.SourceTool, Truncated: false},
			Mechanism: diskclean.Mechanism{Kind: diskclean.MechanismTool, Program: "docker",
				Args: []string{"volume", "prune", "-f"}},
		},
	}
	m.DiskClean.Scanned = true

	m = diskCleanerCursorOnItem(t, m, "npm-cache")
	card := diskCleanFlatView(m)
	for _, want := range []string{
		"The npm cache — regenerable cache — safe to rebuild",
		"Holds 4.0 MiB on disk, 6.0 MiB in contents, 12 files (walked).",
		"Found because npm is on PATH and its cache directory exists.",
		"the packages npm downloaded and unpacked",
	} {
		if !strings.Contains(card, want) {
			t.Errorf("the card of the npm cache does not say %q:\n%s", want, card)
		}
	}
	// A path is one token and this one is longer than the column, so the card
	// wraps it. The words are what the reader needs; where the wrap lands is the
	// column's business. Comparing the text without its spaces checks the path
	// itself rather than the width it happened to be drawn in.
	path := filepath.Join(home, ".npm", "_cacache")
	if !strings.Contains(strings.ReplaceAll(card, " ", ""), strings.ReplaceAll("On disk: "+path, " ", "")) {
		t.Errorf("the card of the npm cache does not say where it is (%q):\n%s", path, card)
	}
	if strings.Contains(card, "Inside the engine: volumes") {
		t.Errorf("the npm cache card carries the container objects of another finding:\n%s", card)
	}

	m = diskCleanerCursorOnItem(t, m, "docker-volumes")
	volumes := diskCleanFlatView(m)
	for _, want := range []string{
		"The container volumes — persistent volumes — delicate",
		"Holds 1.0 GiB on disk (tool-reported): volumes.",
		"docker is on PATH and answered `system df` for the volumes.",
		"data the engine owns and may not be reproducible",
	} {
		if !strings.Contains(volumes, want) {
			t.Errorf("the card of the volumes does not say %q:\n%s", want, volumes)
		}
	}
	if strings.Contains(volumes, filepath.Join(home, ".npm", "_cacache")) {
		t.Errorf("the volumes card carries the npm cache's path:\n%s", volumes)
	}
}

// TestDiskCleanerCardSaysWhyAnUnmeasuredFindingIsNotAZero pins the other half of
// the card: a store that is gone, unreadable or cut short by the walk's own
// ceiling says so, in words, instead of being printed as an empty store.
func TestDiskCleanerCardSaysWhyAnUnmeasuredFindingIsNotAZero(t *testing.T) {
	m, _ := diskCleanerModel(t)
	m.Width, m.Height = 200, 80

	m.DiskClean.Items = []diskclean.Item{
		{
			ID: "gone", Title: "The pip cache", Category: diskclean.CategoryCache, Risk: diskclean.RiskSafe,
			Reason: "pip is on PATH", Size: diskclean.Size{Missing: true},
			Mechanism: diskclean.Mechanism{Kind: diskclean.MechanismRemove},
			Paths:     []string{"/tmp/not-there"},
		},
		{
			ID: "unreadable", Title: "The Go build cache", Category: diskclean.CategoryBuild, Risk: diskclean.RiskSafe,
			Reason:    "go is on PATH and its build cache directory exists",
			Size:      diskclean.Size{Err: "permission denied", Physical: 0},
			Mechanism: diskclean.Mechanism{Kind: diskclean.MechanismRemove},
			Paths:     []string{"/tmp/unreadable"},
		},
		{
			ID: "cut-short", Title: "The Cargo cache", Category: diskclean.CategoryDownloads, Risk: diskclean.RiskSafe,
			Reason:    "cargo is on PATH and its registry cache exists",
			Size:      diskclean.Size{Logical: 1 << 40, Physical: 1 << 40, Truncated: true, Shared: 2},
			Mechanism: diskclean.Mechanism{Kind: diskclean.MechanismRemove},
			Paths:     []string{"/tmp/cargo"},
		},
	}
	m.DiskClean.Scanned = true

	m = diskCleanerCursorOnItem(t, m, "gone")
	if view := diskCleanFlatView(m); !strings.Contains(view, "Holds nothing: the store is not there any more") {
		t.Errorf("a store that is gone is not described as gone:\n%s", view)
	}

	m = diskCleanerCursorOnItem(t, m, "unreadable")
	if view := diskCleanFlatView(m); !strings.Contains(view, "Its size could not be measured: permission denied.") {
		t.Errorf("an unreadable store does not carry its reason:\n%s", view)
	}

	m = diskCleanerCursorOnItem(t, m, "cut-short")
	view := diskCleanFlatView(m)
	for _, want := range []string{"lower bound rather than the whole store", "counted once and listed here as shared"} {
		if !strings.Contains(view, want) {
			t.Errorf("a walk cut short does not say %q:\n%s", want, view)
		}
	}
}

// TestDiskCleanerResultsRenderEachOutcomeAndTheFilesystemNumber pins the result
// half: one line per item with what came back and from where, and the
// filesystem's own delta named as the number to trust.
func TestDiskCleanerResultsRenderEachOutcomeAndTheFilesystemNumber(t *testing.T) {
	m, _ := diskCleanerModel(t)
	m.Width, m.Height = 160, 70

	cleaned := diskCleanRemovalItem("store", "The store", "/tmp/store", 4096)
	m.DiskClean.CleanReport = diskclean.CleanReport{
		FreedBytes:    12 << 20,
		FreedMeasured: true,
		Results: []diskclean.Result{
			{Item: cleaned, Ran: true, Freed: 4 << 20, FreedSource: diskclean.SourceWalk},
			{
				Item:        diskclean.Item{ID: "docker-images", Title: "The container images", Tool: "docker"},
				Ran:         true,
				Freed:       8 << 20,
				FreedSource: diskclean.SourceTool,
			},
			{
				Item:    diskclean.Item{ID: "pip-cache", Title: "The pip cache", Tool: "pip"},
				Ran:     true,
				Error:   "refused: the path is not in the approved scope",
				Refused: true,
			},
		},
	}
	m.DiskClean.Phase = diskCleanResults
	m.DiskClean.Notice = diskCleanResultNotice(m.DiskClean.CleanReport)

	view := diskCleanFlatView(m)
	for _, want := range []string{
		"The store: 4.0 MiB came back",
		"The container images: the tool reported 8.0 MiB reclaimed",
		"The pip cache: refused before anything was removed",
		"gained 12.0 MiB around the run",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("the result screen does not say %q:\n%s", want, view)
		}
	}
}

// diskCleanRemovalItem is a finding that removes a directory, with the path the
// guard is asked to approve.
// diskCleanRemovalItem is a finding of the plainest kind: a directory the scan
// found, sized by walking it, removable directly. It carries a reason and a detail
// because every real finding does, and the card renders them.
func diskCleanRemovalItem(id, title, path string, size int64) diskclean.Item {
	return diskclean.Item{
		ID: id, Title: title, Risk: diskclean.RiskSafe, Paths: []string{path},
		Category:  diskclean.CategoryCache,
		Reason:    "the scan found this directory, and the test named it",
		Detail:    "data the owning tool can build again",
		Size:      diskclean.Size{Logical: size, Physical: size},
		Mechanism: diskclean.Mechanism{Kind: diskclean.MechanismRemove},
	}
}

// diskCleanRecordingRemover is a removal double that records every unlink, so a
// test can see the order without watching a real store disappear.
type diskCleanRecordingRemover struct {
	order []string
}

func (r *diskCleanRecordingRemover) remove(path string) error {
	r.order = append(r.order, path)
	return os.Remove(path)
}

// writeTestFile writes a file of a size, creating its directories.
func writeTestFile(t *testing.T, path string, size int) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// itemIDs is the list of findings by identifier.
func itemIDs(items []diskclean.Item) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

// diskCleanContains reports whether a list of identifiers holds one.
func diskCleanContains(list []string, want string) bool {
	for _, value := range list {
		if value == want {
			return true
		}
	}
	return false
}
