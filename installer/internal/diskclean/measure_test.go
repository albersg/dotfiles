package diskclean

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// testProbe is the real walker with its clock fixed, so a test measures a
// temporary tree and nothing about the machine it runs on.
func testProbe(t *testing.T) Probe {
	t.Helper()
	probe := DefaultProbe()
	probe.HomeDir = t.TempDir()
	probe.Now = func() time.Time { return time.Unix(1_700_000_000, 0) }
	return probe
}

// writeTestFile creates a file of the given size and returns its path.
func writeTestFile(t *testing.T, path string, size int) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create the directory for %s: %v", path, err)
	}
	content := make([]byte, size)
	for i := range content {
		content[i] = 'x'
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// TestWalkMeasuresATreeAndCountsTheLinksItRefusesToFollow pins the two halves
// of one walk: a file's own bytes are summed, and a symbolic link inside the
// tree is counted as a link rather than followed into whatever it points at.
func TestWalkMeasuresATreeAndCountsTheLinksItRefusesToFollow(t *testing.T) {
	probe := testProbe(t)
	root := t.TempDir()
	outside := t.TempDir()

	writeTestFile(t, filepath.Join(root, "one.bin"), 4096)
	writeTestFile(t, filepath.Join(root, "nested", "two.bin"), 2048)
	writeTestFile(t, filepath.Join(outside, "not-this.bin"), 1<<20)
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatalf("create the symbolic link: %v", err)
	}

	result := Walk(context.Background(), probe, root, "item", NewDedupe(), DefaultLimits())
	if result.Err != "" {
		t.Fatalf("the walk failed: %s", result.Err)
	}
	if want := int64(4096 + 2048); result.Logical != want {
		t.Errorf("logical size is %d, want %d (the linked tree must not be counted)", result.Logical, want)
	}
	if result.Files != 2 {
		t.Errorf("the walk counted %d files, want 2", result.Files)
	}
	if result.Symlinks != 1 {
		t.Errorf("the walk counted %d symbolic links, want 1", result.Symlinks)
	}
	if runtime.GOOS != "windows" && result.Physical < result.Logical {
		t.Errorf("physical size %d is below the logical size %d on a filesystem that allocates whole blocks",
			result.Physical, result.Logical)
	}
}

// TestWalkRefusesALinkAsItsOwnRoot pins the case that matters for deletion: a
// store path that is itself a symbolic link is not measured through the link,
// because the bytes are not under the approved path at all.
func TestWalkRefusesALinkAsItsOwnRoot(t *testing.T) {
	probe := testProbe(t)
	root := t.TempDir()
	target := t.TempDir()
	writeTestFile(t, filepath.Join(target, "data.bin"), 1024)
	link := filepath.Join(root, "store")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("create the symbolic link: %v", err)
	}

	result := Walk(context.Background(), probe, link, "item", NewDedupe(), DefaultLimits())
	if result.Err == "" {
		t.Fatal("a symbolic link was accepted as a store root")
	}
	if !strings.Contains(result.Err, "symbolic link") {
		t.Errorf("the refusal does not say it was a link: %s", result.Err)
	}
	if result.Logical != 0 || result.Files != 0 {
		t.Errorf("the walk counted through the link: %d bytes in %d files", result.Logical, result.Files)
	}
}

// TestMeasureCountsAHardlinkOnceAndNamesTheOwner is the no-double-counting
// rule at its smallest: two items hold hardlinks to one file, and the second
// reports the file as shared rather than adding its bytes a second time.
func TestMeasureCountsAHardlinkOnceAndNamesTheOwner(t *testing.T) {
	probe := testProbe(t)
	root := t.TempDir()
	first := writeTestFile(t, filepath.Join(root, "first", "data.bin"), 8192)
	second := filepath.Join(root, "second", "data.bin")
	if err := os.MkdirAll(filepath.Dir(second), 0o755); err != nil {
		t.Fatalf("create the second directory: %v", err)
	}
	if err := os.Link(first, second); err != nil {
		t.Fatalf("create the hardlink: %v", err)
	}

	items := []Item{
		{ID: "first", Paths: []string{filepath.Join(root, "first")}, Mechanism: Mechanism{Kind: MechanismRemove}},
		{ID: "second", Paths: []string{filepath.Join(root, "second")}, Mechanism: Mechanism{Kind: MechanismRemove}},
	}
	measured, _ := Measure(context.Background(), probe, items, MeasureOptions{Limits: noCeilings()})

	if measured[0].Size.Logical != 8192 || measured[0].Size.Files != 1 {
		t.Errorf("the first item holds %d bytes in %d files, want 8192 in 1",
			measured[0].Size.Logical, measured[0].Size.Files)
	}
	if measured[1].Size.Logical != 0 {
		t.Errorf("the second item added the shared file again: %d logical bytes", measured[1].Size.Logical)
	}
	if measured[1].Size.Shared != 1 || measured[1].Size.SharedBytes != 8192 {
		t.Errorf("the second item reports %d shared files of %d bytes, want 1 of 8192",
			measured[1].Size.Shared, measured[1].Size.SharedBytes)
	}
	if len(measured[1].Size.SharedWith) != 1 || measured[1].Size.SharedWith[0] != "first" {
		t.Errorf("the second item names %v as the owner, want [first]", measured[1].Size.SharedWith)
	}
	if len(measured[0].Overlaps) != 1 || measured[0].Overlaps[0] != "second" {
		t.Errorf("the first item names %v as overlapping, want [second]", measured[0].Overlaps)
	}
}

// TestMeasureReportsANestedItemAsSharedAndTheUnionAsTheTotal is the same rule
// for a path that contains another: the outer item owns the bytes, the inner
// one reports zero of its own, and the report's total is the union rather than
// the sum of two overlapping numbers.
func TestMeasureReportsANestedItemAsSharedAndTheUnionAsTheTotal(t *testing.T) {
	probe := testProbe(t)
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "cache", "one.bin"), 4096)
	writeTestFile(t, filepath.Join(root, "cache", "nested", "two.bin"), 4096)
	writeTestFile(t, filepath.Join(root, "outside.bin"), 1024)

	items := []Item{
		{ID: "outer", Paths: []string{root}, Mechanism: Mechanism{Kind: MechanismRemove}},
		{ID: "inner", Paths: []string{filepath.Join(root, "cache")}, Mechanism: Mechanism{Kind: MechanismRemove}},
	}
	measured, _ := Measure(context.Background(), probe, items, MeasureOptions{Limits: noCeilings()})

	if measured[1].Size.Logical != 0 || measured[1].Size.Shared != 2 {
		t.Errorf("the nested item holds %d bytes with %d shared files, want 0 bytes and 2 shared files",
			measured[1].Size.Logical, measured[1].Size.Shared)
	}
	if measured[1].Reclaimable() != 0 {
		t.Errorf("the nested item offers to free %d bytes that the outer item already counts", measured[1].Reclaimable())
	}
	report := Report{Items: measured}
	// The union is what the outer item alone holds: the nested store's bytes
	// are already inside it, and the report may not add them a second time.
	if report.Reclaimable() != measured[0].Reclaimable() {
		t.Errorf("the report's total is %d and the outer item alone is %d: an overlapping store was counted twice",
			report.Reclaimable(), measured[0].Reclaimable())
	}
	if measured[0].Reclaimable() <= 0 {
		t.Errorf("the outer item holds %d reclaimable bytes, want a measured tree", measured[0].Reclaimable())
	}
	for _, item := range measured {
		if len(item.Overlaps) != 1 {
			t.Errorf("%s names %v as overlapping, want exactly the other item", item.ID, item.Overlaps)
		}
	}
}

// TestWalkStopsAtItsCeilingsAndSaysSo pins that a bounded walk reports a lower
// bound instead of a quiet wrong number.
func TestWalkStopsAtItsCeilingsAndSaysSo(t *testing.T) {
	probe := testProbe(t)
	root := t.TempDir()
	for i := 0; i < 5; i++ {
		writeTestFile(t, filepath.Join(root, "file"+string(rune('a'+i))+".bin"), 1024)
	}

	result := Walk(context.Background(), probe, root, "item", NewDedupe(), Limits{MaxFiles: 2, MaxDuration: -1, MaxReasons: -1})
	if !result.Truncated {
		t.Fatal("a walk cut short by its file ceiling did not say so")
	}
	if result.Files >= 5 {
		t.Errorf("the walk counted %d files past its ceiling of 2", result.Files)
	}
}

// TestWalkStopsWhenTheContextEnds pins cancellation: the user's stop ends the
// walk, and what was counted before it is kept rather than thrown away.
func TestWalkStopsWhenTheContextEnds(t *testing.T) {
	probe := testProbe(t)
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "file.bin"), 1024)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := Walk(ctx, probe, root, "item", NewDedupe(), DefaultLimits())
	if !result.Cancelled {
		t.Error("a walk whose context was already cancelled did not report the cancellation")
	}
	if result.Files != 0 {
		t.Errorf("the cancelled walk counted %d files, want none", result.Files)
	}
}

// TestWalkRecordsAnUnreadableDirectoryAndCarriesOn pins the partial-failure
// rule: one directory nobody can list is a reason beside the size, not the end
// of the measurement.
func TestWalkRecordsAnUnreadableDirectoryAndCarriesOn(t *testing.T) {
	probe := testProbe(t)
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "readable.bin"), 2048)
	broken := filepath.Join(root, "locked")
	if err := os.MkdirAll(broken, 0o755); err != nil {
		t.Fatalf("create the locked directory: %v", err)
	}
	probe.ReadDir = func(dir string) ([]os.DirEntry, error) {
		if dir == broken {
			return nil, os.ErrPermission
		}
		return os.ReadDir(dir)
	}

	result := Walk(context.Background(), probe, root, "item", NewDedupe(), DefaultLimits())
	if result.Files != 1 || result.Logical != 2048 {
		t.Errorf("the walk counted %d bytes in %d files, want 2048 in 1", result.Logical, result.Files)
	}
	if len(result.Reasons) != 1 || !strings.Contains(result.Reasons[0], broken) {
		t.Errorf("the unreadable directory is not named in %v", result.Reasons)
	}
	if result.ReasonsTotal != 1 {
		t.Errorf("the walk counted %d reasons, want 1", result.ReasonsTotal)
	}
}

// TestMeasureSaysMissingRatherThanZero pins the difference the screen depends
// on: a store that is not there is missing, not a zero-byte store.
func TestMeasureSaysMissingRatherThanZero(t *testing.T) {
	probe := testProbe(t)
	root := t.TempDir()
	items := []Item{{
		ID:        "gone",
		Paths:     []string{filepath.Join(root, "never-created")},
		Mechanism: Mechanism{Kind: MechanismRemove},
	}}

	measured, _ := Measure(context.Background(), probe, items, MeasureOptions{Limits: noCeilings()})
	if !measured[0].Size.Missing {
		t.Error("a path that does not exist is not reported as missing")
	}
	if measured[0].Size.Err != "" {
		t.Errorf("a missing store is reported as an error: %s", measured[0].Size.Err)
	}
}

// TestMeasureReportsProgressAndSkipsItemsWithNothingToWalk pins the two
// behaviors a screen wires up: a progress tick per path, and no walk for an
// item whose owner reports its own accounting.
func TestMeasureReportsProgressAndSkipsItemsWithNothingToWalk(t *testing.T) {
	probe := testProbe(t)
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "cache.bin"), 512)

	items := []Item{
		{ID: "walked", Paths: []string{root}, Mechanism: Mechanism{Kind: MechanismRemove}},
		{ID: "reported", Mechanism: Mechanism{Kind: MechanismTool, Program: "docker"}},
		{ID: "refused", Mechanism: Mechanism{Kind: MechanismReportOnly, Reason: "no command exists"}},
	}
	var ticks []Progress
	measured, _ := Measure(context.Background(), probe, items, MeasureOptions{
		Limits:   noCeilings(),
		Progress: func(p Progress) { ticks = append(ticks, p) },
	})

	if len(ticks) != 1 || ticks[0].ItemID != "walked" || ticks[0].Files != 1 {
		t.Errorf("progress ticks are %+v, want one tick for the walked item", ticks)
	}
	if measured[1].Size.Files != 0 || measured[2].Size.Files != 0 {
		t.Errorf("an item with no paths was walked: %+v", measured[1].Size)
	}
}

// TestSortBySizeIsStable pins the row order: largest reclaimable first, with
// the ID breaking a tie so two scans of the same machine read the same way.
func TestSortBySizeIsStable(t *testing.T) {
	items := []Item{
		{ID: "b", Size: Size{Physical: 10}},
		{ID: "a", Size: Size{Physical: 10}},
		{ID: "c", Size: Size{Physical: 99}},
	}
	SortBySize(items)
	if items[0].ID != "c" || items[1].ID != "a" || items[2].ID != "b" {
		t.Errorf("the order is %s %s %s, want c a b", items[0].ID, items[1].ID, items[2].ID)
	}
}

// noCeilings is the limit set the small fixtures run under: every ceiling off,
// so a test's assertion is about the fixture and not about the default budget.
func noCeilings() Limits {
	return Limits{MaxFiles: -1, MaxDuration: -1, MaxReasons: -1}
}
