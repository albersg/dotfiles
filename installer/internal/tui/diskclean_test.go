package tui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/diskclean"
	"github.com/albersg/dotfiles/installer/internal/system"
	tea "github.com/charmbracelet/bubbletea"
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
	m, cmd := diskCleanerPress(t, m, "enter")
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

// TestDiskCleanerScanIsACommandAndItsAnswerFillsTheScreen pins the shape of a
// scan: a press starts a command, the screen says a scan is in flight, and the
// answer replaces the findings and leaves nothing selected.
func TestDiskCleanerScanIsACommandAndItsAnswerFillsTheScreen(t *testing.T) {
	m, home := diskCleanerModel(t)
	writeTestFile(t, filepath.Join(home, ".npm", "_cacache", "blob.bin"), 8192)

	m, cmd := diskCleanerPress(t, m, "enter")
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
	m, cmd := diskCleanerPress(t, m, "enter")
	if cmd == nil {
		t.Fatal("the first press started no scan")
	}
	m, second := diskCleanerPress(t, m, "enter")
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
	m, cmd := diskCleanerPress(t, m, "enter")
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
	if len(rows) != 2 || rows[0].action != diskCleanActionConfirm || rows[1].action != diskCleanActionCancelPreview {
		t.Fatalf("the preview offers %v, want a confirmation and a way back", diskCleanRowLabels(rows))
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
	m, _ = diskCleanerPress(t, m, "enter")
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
	m, _ = diskCleanerPress(t, m, "enter")
	m = diskCleanerCursorOn(t, m, diskCleanActionConfirm)
	m, cmd := diskCleanerPress(t, m, "enter")

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
	m, _ = diskCleanerPress(t, m, "enter")
	m = diskCleanerCursorOn(t, m, diskCleanActionConfirm)
	m, cmd := diskCleanerPress(t, m, "enter")
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
	m, cmd := diskCleanerPress(t, m, "enter")
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
	m, cmd := diskCleanerPress(t, m, "enter")
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
	m, cmd = diskCleanerPress(t, m, "enter")
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
	m, _ = diskCleanerPress(t, m, "enter")

	rows := m.diskCleanRows()
	if len(rows) == 0 || rows[0].action != diskCleanActionConfirm {
		t.Fatalf("the preview rows are %v", diskCleanRowLabels(rows))
	}
	if !strings.Contains(rows[0].label, "1 item") {
		t.Errorf("the confirmation row promises %q, want one item", rows[0].label)
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
// the frame happens to wrap across two lines.
func diskCleanFlatView(m Model) string {
	plain := ansiEscape.ReplaceAllString(m.View(), "")
	return strings.Join(strings.Fields(plain), " ")
}

// TestUtilitiesSectionOffersTheDevelopmentCacheCleaner pins the way in: the
// section lists the job, the panel names it, the rendered section shows the row
// and says what the job is, and choosing the row opens the screen without
// scanning anything on the way.
func TestUtilitiesSectionOffersTheDevelopmentCacheCleaner(t *testing.T) {
	m := installerFrameModel(t, ScreenUtilities)
	m.Width, m.Height = 120, 50

	rows := m.GetCurrentOptions()
	found := false
	for _, row := range rows {
		if row == utilitiesDiskCleanerRow {
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
	if !strings.Contains(view, "cleans only the findings you mark") {
		t.Errorf("the section does not say what the row does:\n%s", view)
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
		if row == utilitiesDiskCleanerRow {
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
// puts on screen: the exact command for a tool, the exact path for a removal, and
// the reason for a finding the plan refuses -- with the promise that nothing has
// run yet.
func TestDiskCleanerPreviewRendersEachCommandAndEachRefusal(t *testing.T) {
	m, _ := diskCleanerModel(t)
	m.Width, m.Height = 160, 70
	store := filepath.Join(t.TempDir(), "store")
	writeTestFile(t, filepath.Join(store, "data.bin"), 1024)

	m.DiskClean.Items = []diskclean.Item{
		{
			ID: "npm-cache", Tool: "npm", Title: "The npm cache", Risk: diskclean.RiskSafe,
			Size:      diskclean.Size{Logical: 4096, Physical: 4096},
			Mechanism: diskclean.Mechanism{Kind: diskclean.MechanismTool, Program: "npm", Args: []string{"cache", "clean", "--force"}},
		},
		diskCleanRemovalItem("store", "The store", store, 1024),
		{
			ID: "apt-archives", Tool: "apt", Title: "The apt cache", Risk: diskclean.RiskSafe,
			Size:      diskclean.Size{Logical: 1 << 20, Physical: 1 << 20},
			Mechanism: diskclean.Mechanism{Kind: diskclean.MechanismReportOnly, Reason: "apt-get is not on PATH here"},
		},
	}
	m.DiskClean.Scanned = true
	m.DiskClean.Selection = diskclean.Selection{"npm-cache": true, "store": true, "apt-archives": true}

	m = diskCleanerCursorOn(t, m, diskCleanActionPreview)
	m, _ = diskCleanerPress(t, m, "enter")
	if m.DiskClean.Phase != diskCleanPreview {
		t.Fatalf("the preview did not open: %v", m.DiskClean.Phase)
	}

	view := diskCleanFlatView(m)
	for _, want := range []string{
		"would run `npm cache clean --force`",
		"would remove " + store + ", irreversibly",
		"nothing would run. This finding is reported, not cleaned: apt-get is not on PATH here",
		"nothing has run yet",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("the preview does not say %q:\n%s", want, view)
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
func diskCleanRemovalItem(id, title, path string, size int64) diskclean.Item {
	return diskclean.Item{
		ID: id, Title: title, Risk: diskclean.RiskSafe, Paths: []string{path},
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
