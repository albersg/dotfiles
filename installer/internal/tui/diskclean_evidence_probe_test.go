package tui

// This file is the independent verification's own probe, kept as a durable
// regression test. It was written by a read-only `gentle-ai-verify` run that
// judged S1-S14 against the spec text in odd/tasks/developer-disk-cleaner.md,
// and it is the probe that failed on the S3/S5 defect the feature's own tests
// had missed: the screen showed a size and a risk and none of the evidence
// beside them (fixed in `29985dd`).
//
// It is kept exactly as the verification wrote it, markers and all, on purpose:
// the markers are unique, so a passing assertion cannot come from prose of our
// own that happens to read well. The S3 half of it is red at `52be18a`, the
// commit before the fix, and green after it; the S5 half pins the dry run, and
// the third test pins both to the terminal size the request calls the floor.
//
// It is a probe, not a specification: the specs stay in the task document.

import (
	"strings"
	"testing"

	"github.com/albersg/dotfiles/installer/internal/diskclean"
)

// probeItems is the verifier's own fixture, derived from the verbatim S3 text:
// readable units, origin paths/engine objects, detection reason, and the degree
// of certainty (category, risk, measurement source, logical vs physical,
// truncated/shared/not-measured). The markers are unique on purpose, so a
// passing assertion cannot come from the writer's own prose.
func probeItems() []diskclean.Item {
	return []diskclean.Item{
		{
			ID: "cache-item", Tool: "npm", Title: "PROBE cache title",
			Category: diskclean.CategoryCache, Risk: diskclean.RiskSafe,
			Reason: "REASON-CACHE-MARKER", Detail: "DETAIL-CACHE-MARKER",
			Paths: []string{"/tmp/PATH-CACHE-MARKER"},
			Size:  diskclean.Size{Logical: 1 << 30, Physical: 900 << 20, Files: 4213},
		},
		{
			ID: "unmeasured", Tool: "pip", Title: "PROBE unmeasured title",
			Category: diskclean.CategoryDownloads, Risk: diskclean.RiskCareful,
			Reason: "REASON-UNMEASURED-MARKER", Detail: "DETAIL-UNMEASURED-MARKER",
			Paths: []string{"/tmp/PATH-UNMEASURED-MARKER"},
			Size:  diskclean.Size{Err: "SIZE-ERR-MARKER"},
		},
		{
			ID: "truncated", Tool: "go", Title: "PROBE truncated title",
			Category: diskclean.CategoryBuild, Risk: diskclean.RiskSafe,
			Reason: "REASON-TRUNCATED-MARKER", Detail: "DETAIL-TRUNCATED-MARKER",
			Paths: []string{"/tmp/PATH-TRUNCATED-MARKER"},
			Size:  diskclean.Size{Logical: 700 << 20, Physical: 640 << 20, Truncated: true},
		},
		{
			ID: "shared", Tool: "gradle", Title: "PROBE shared title",
			Category: diskclean.CategoryCache, Risk: diskclean.RiskSafe,
			Reason: "REASON-SHARED-MARKER", Detail: "DETAIL-SHARED-MARKER",
			Paths:    []string{"/tmp/PATH-SHARED-MARKER"},
			Overlaps: []string{"truncated"},
			Size:     diskclean.Size{Logical: 50 << 20, Physical: 50 << 20, Files: 3, Shared: 2, SharedBytes: 10 << 20, SharedWith: []string{"truncated"}},
		},
		{
			ID: "toolitem", Tool: "docker", Title: "PROBE tool title",
			Category: diskclean.CategoryVolumes, Risk: diskclean.RiskDelicate,
			Reason: "REASON-TOOL-MARKER", Detail: "DETAIL-TOOL-MARKER",
			VolatileObjects: []string{"VOLOBJ-MARKER"},
			Size:            diskclean.Size{Logical: 500 << 20, Physical: 500 << 20, Source: diskclean.SourceTool},
			Mechanism:       diskclean.Mechanism{Kind: diskclean.MechanismTool, Program: "docker", Args: []string{"volume", "prune", "-f"}},
		},
	}
}

func probeModel(t *testing.T, items []diskclean.Item) Model {
	t.Helper()
	m := installerFrameModel(t, ScreenDiskCleaner)
	m.Width, m.Height = 80, 24
	m.Animating = false
	m.DiskClean = diskCleanState{
		Resolved:  true,
		Scanned:   true,
		Cwd:       "",
		Items:     items,
		Selection: diskclean.Selection{},
	}
	return m
}

func probeFocusItem(t *testing.T, m Model, itemIndex int) Model {
	t.Helper()
	for i, row := range m.diskCleanRows() {
		if row.action == diskCleanActionItem && row.item == itemIndex {
			m.Cursor = i
			return m
		}
	}
	t.Fatalf("no list row for item %d", itemIndex)
	return m
}

// TestVerifyS3ListRendersEvidence is the S3 probe: at the 80x24 floor, a finding
// under the cursor must render every evidence field the request names.
func TestVerifyS3ListRendersEvidence(t *testing.T) {
	items := probeItems()
	m := probeModel(t, items)
	cases := []struct {
		id     string
		fields []string
	}{
		{"cache-item", []string{"regenerable cache", "safe to rebuild", "REASON-CACHE-MARKER", "/tmp/PATH-CACHE-MARKER", "900.0 MiB", "1.0 GiB", "walked", "DETAIL-CACHE-MARKER"}},
		{"unmeasured", []string{"downloads", "costs a re-download or a rebuild", "REASON-UNMEASURED-MARKER", "/tmp/PATH-UNMEASURED-MARKER", "could not be measured", "SIZE-ERR-MARKER"}},
		{"truncated", []string{"build artifacts", "REASON-TRUNCATED-MARKER", "/tmp/PATH-TRUNCATED-MARKER", "Lower bound"}},
		{"shared", []string{"REASON-SHARED-MARKER", "/tmp/PATH-SHARED-MARKER", "shared"}},
		{"toolitem", []string{"persistent volumes", "delicate", "REASON-TOOL-MARKER", "tool-reported", "VOLOBJ-MARKER"}},
	}
	for _, tc := range cases {
		idx := -1
		for i := range items {
			if items[i].ID == tc.id {
				idx = i
			}
		}
		if idx < 0 {
			t.Fatalf("fixture lost item %q", tc.id)
		}
		mm := probeFocusItem(t, m, idx)
		plain := ansiEscape.ReplaceAllString(mm.View(), "")
		for _, field := range tc.fields {
			if !strings.Contains(plain, field) {
				t.Errorf("S3: 80x24 list view for %q does not render %q\n---\n%s", tc.id, field, plain)
			}
		}
	}
}

// TestVerifyS5PreviewRendersExactActions is the S5 probe: the dry run must name
// the exact command, the exact path, or the exact reason an item would not run,
// plus the reconstruction cost, and nothing may have run yet.
func TestVerifyS5PreviewRendersExactActions(t *testing.T) {
	tool := diskclean.Item{
		ID: "tool", Tool: "npm", Title: "PROBE preview tool",
		Category: diskclean.CategoryCache, Risk: diskclean.RiskSafe,
		Reason: "REASON-TOOL-PREVIEW", Detail: "DETAIL-TOOL-PREVIEW",
		Size:      diskclean.Size{Physical: 42 << 20},
		Mechanism: diskclean.Mechanism{Kind: diskclean.MechanismTool, Program: "npm", Args: []string{"cache", "clean", "--force"}},
	}
	remove := diskclean.Item{
		ID: "remove", Tool: "custom", Title: "PROBE preview remove",
		Category: diskclean.CategoryBuild, Risk: diskclean.RiskCareful,
		Reason: "REASON-REMOVE-PREVIEW", Detail: "DETAIL-REMOVE-PREVIEW",
		Paths:     []string{"/home/probe/EXACT-PATH-MARKER"},
		Size:      diskclean.Size{Physical: 10 << 20},
		Mechanism: diskclean.Mechanism{Kind: diskclean.MechanismRemove},
	}
	report := diskclean.Item{
		ID: "report", Tool: "apt", Title: "PROBE preview report",
		Category: diskclean.CategoryDownloads, Risk: diskclean.RiskSafe,
		Reason: "REASON-REPORT-PREVIEW", Detail: "DETAIL-REPORT-PREVIEW",
		Size:      diskclean.Size{Physical: 5 << 20},
		Mechanism: diskclean.Mechanism{Kind: diskclean.MechanismReportOnly, Reason: "REFUSAL-MARKER"},
	}

	m := probeModel(t, []diskclean.Item{tool, remove, report})
	m.DiskClean.Phase = diskCleanPreview
	m.DiskClean.Planned = true
	m.DiskClean.Plan = diskclean.Plan{Items: []diskclean.PlannedItem{
		{Item: tool, Action: "npm cache clean --force"},
		{Item: remove},
		{Item: report, Blocked: "This finding is reported, not cleaned: REFUSAL-MARKER"},
	}}
	m.Cursor = m.diskCleanFirstItemCursor()

	if m.DiskClean.Phase != diskCleanPreview {
		t.Fatalf("the probe is not in the preview phase")
	}
	if len(m.DiskClean.CleanReport.Results) != 0 {
		t.Fatalf("something already ran before the confirmation")
	}

	cases := []struct {
		idx    int
		fields []string
	}{
		{0, []string{"npm cache clean --force", "DETAIL-TOOL-PREVIEW", "42.0 MiB"}},
		{1, []string{"/home/probe/EXACT-PATH-MARKER", "irreversibly", "DETAIL-REMOVE-PREVIEW"}},
		{2, []string{"REFUSAL-MARKER", "DETAIL-REPORT-PREVIEW"}},
	}
	for _, tc := range cases {
		mm := m
		for i, row := range mm.diskCleanRows() {
			if row.action == diskCleanActionPreviewItem && row.item == tc.idx {
				mm.Cursor = i
			}
		}
		plain := ansiEscape.ReplaceAllString(mm.View(), "")
		for _, field := range tc.fields {
			if !strings.Contains(plain, field) {
				t.Errorf("S5: 80x24 preview view for item %d does not render %q\n---\n%s", tc.idx, field, plain)
			}
		}
	}
}

// TestVerifyS3S5ViewFitsTheFloor pins the rendering probes to the terminal the
// request treats as the floor.
func TestVerifyS3S5ViewFitsTheFloor(t *testing.T) {
	m := probeFocusItem(t, probeModel(t, probeItems()), 0)
	for _, line := range strings.Split(ansiEscape.ReplaceAllString(m.View(), ""), "\n") {
		if len([]rune(line)) > 80 {
			t.Errorf("a rendered line is wider than 80 columns: %q", line)
		}
	}
	if got := strings.Count(ansiEscape.ReplaceAllString(m.View(), ""), "\n"); got > 24 {
		t.Errorf("the view renders %d rows, more than 24", got)
	}
}
