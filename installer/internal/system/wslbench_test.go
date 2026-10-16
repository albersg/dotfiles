package system

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// benchTestOptions is a deliberately tiny set so every test is fast. The default (64 files of
// 64 KiB) exists for a real run; a test that used it would write 4 MiB per target, which is neither
// fast nor needed, because the code path is identical.
func benchTestOptions() BenchOptions {
	return BenchOptions{Files: 4, FileSize: 128, Timeout: 5 * time.Second}
}

// snapshotDir records a directory tree's exact shape: every path, its content for a file, and a
// marker for a directory. It is the comparison the "nothing was left behind" tests make.
func snapshotDir(t *testing.T, dir string) map[string]string {
	t.Helper()
	snapshot := map[string]string{}
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return relErr
		}
		if rel == "." {
			return nil
		}
		if info.IsDir() {
			snapshot[rel] = "<dir>"
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		snapshot[rel] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", dir, err)
	}
	return snapshot
}

// TestRunBenchmarkLeavesTheTargetExactlyAsItWas is the non-destructive guarantee: a directory that
// already holds files comes out of the run byte-for-byte unchanged, scratch directory included.
func TestRunBenchmarkLeavesTheTargetExactlyAsItWas(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "keep.txt")
	if err := os.WriteFile(existing, []byte("a project's own file"), 0o644); err != nil {
		t.Fatalf("write pre-existing file: %v", err)
	}
	subdir := filepath.Join(dir, "src")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatalf("mkdir pre-existing subdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subdir, "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatalf("write pre-existing nested file: %v", err)
	}

	before := snapshotDir(t, dir)

	report := RunBenchmark(context.Background(), []BenchTarget{{Name: "this project", Path: dir}}, benchTestOptions())

	after := snapshotDir(t, dir)
	if !reflect.DeepEqual(before, after) {
		t.Errorf("the target changed:\nbefore = %v\nafter  = %v", before, after)
	}
	if !report.Cleaned {
		t.Error("Cleaned = false; a scratch directory was left behind")
	}
	if len(report.Results) != 1 {
		t.Fatalf("got %d results, want 1", len(report.Results))
	}
	if got := report.Results[0].Err; got != "" {
		t.Errorf("Err = %q, want empty for a writable target", got)
	}
	if len(report.Results[0].Measurements) != len(benchOperations) {
		t.Errorf("got %d measurements, want %d", len(report.Results[0].Measurements), len(benchOperations))
	}
}

// TestRunBenchmarkMakesOneScratchDirectoryPerTarget proves the scratch directory is created exactly
// once per target, under the target's own path and with the documented prefix, and that it is gone
// afterwards.
func TestRunBenchmarkMakesOneScratchDirectoryPerTarget(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	var created []string
	opts := benchTestOptions()
	opts.Scratch = func(dir, pattern string) (string, error) {
		if pattern != benchScratchPattern {
			t.Errorf("scratch pattern = %q, want %q", pattern, benchScratchPattern)
		}
		path, err := os.MkdirTemp(dir, pattern)
		if err == nil {
			created = append(created, path)
		}
		return path, err
	}

	report := RunBenchmark(context.Background(), []BenchTarget{
		{Name: "a", Path: dirA},
		{Name: "b", Path: dirB},
	}, opts)

	if len(created) != 2 {
		t.Fatalf("got %d scratch directories, want one per target (2)", len(created))
	}
	if !strings.HasPrefix(created[0], dirA+string(os.PathSeparator)) {
		t.Errorf("scratch %q is not inside the first target %q", created[0], dirA)
	}
	if !strings.HasPrefix(created[1], dirB+string(os.PathSeparator)) {
		t.Errorf("scratch %q is not inside the second target %q", created[1], dirB)
	}
	if !strings.Contains(filepath.Base(created[0]), benchScratchPattern) {
		t.Errorf("scratch %q does not carry the documented prefix %q", created[0], benchScratchPattern)
	}
	for _, path := range created {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("scratch directory %q still exists after the run (stat err = %v)", path, err)
		}
	}
	if !report.Cleaned {
		t.Error("Cleaned = false even though every scratch directory was removed")
	}
}

// TestRunBenchmarkOnAReadOnlyTargetReportsAndDoesNotTouchIt proves the unwritable case degrades to an
// Err line: no measurement, no zero row and no panic.
func TestRunBenchmarkOnAReadOnlyTargetReportsAndDoesNotTouchIt(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions, so a read-only directory is not read-only here")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	// Restore write permission so t.TempDir can remove it after the test.
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	report := RunBenchmark(context.Background(), []BenchTarget{{Name: "read-only", Path: dir}}, benchTestOptions())

	if !report.Cleaned {
		t.Error("Cleaned = false for a target that never had a scratch directory")
	}
	if len(report.Results) != 1 {
		t.Fatalf("got %d results, want 1", len(report.Results))
	}
	result := report.Results[0]
	if result.Err == "" {
		t.Error("Err is empty for a read-only target")
	}
	if len(result.Measurements) != 0 {
		t.Errorf("got %d measurements for a read-only target, want none (a zero-throughput row is not a measurement)",
			len(result.Measurements))
	}
}

// TestRunBenchmarkScratchFailureIsReportedNotFatal proves an injected scratch failure produces an Err
// line and leaves Cleaned true, without a panic and without writing anything.
func TestRunBenchmarkScratchFailureIsReportedNotFatal(t *testing.T) {
	dir := t.TempDir()
	before := snapshotDir(t, dir)

	opts := benchTestOptions()
	opts.Scratch = func(string, string) (string, error) {
		return "", os.ErrPermission
	}

	report := RunBenchmark(context.Background(), []BenchTarget{{Name: "project", Path: dir}}, opts)

	if !report.Cleaned {
		t.Error("Cleaned = false when no scratch directory was ever created")
	}
	if len(report.Results) != 1 {
		t.Fatalf("got %d results, want 1", len(report.Results))
	}
	if report.Results[0].Err == "" {
		t.Error("Err is empty when the scratch directory could not be created")
	}
	if len(report.Results[0].Measurements) != 0 {
		t.Errorf("got %d measurements after a scratch failure, want none", len(report.Results[0].Measurements))
	}
	if after := snapshotDir(t, dir); !reflect.DeepEqual(before, after) {
		t.Errorf("the target changed after a scratch failure:\nbefore = %v\nafter = %v", before, after)
	}
}

// TestRunBenchmarkMissingPathReportsAndStillMeasuresTheRest covers the graceful-degradation rule: a
// path that does not exist becomes an Err on that target and the following target is still measured.
func TestRunBenchmarkMissingPathReportsAndStillMeasuresTheRest(t *testing.T) {
	good := t.TempDir()
	missing := filepath.Join(t.TempDir(), "not", "here")

	report := RunBenchmark(context.Background(), []BenchTarget{
		{Name: "missing", Path: missing},
		{Name: "good", Path: good},
	}, benchTestOptions())

	if len(report.Results) != 2 {
		t.Fatalf("got %d results, want 2", len(report.Results))
	}
	if report.Results[0].Err == "" {
		t.Error("Err is empty for a path that does not exist")
	}
	if len(report.Results[0].Measurements) != 0 {
		t.Error("a missing path produced measurements")
	}
	if report.Results[1].Err != "" {
		t.Errorf("the second target failed too: %q", report.Results[1].Err)
	}
	if len(report.Results[1].Measurements) != len(benchOperations) {
		t.Errorf("got %d measurements for the good target, want %d",
			len(report.Results[1].Measurements), len(benchOperations))
	}
	if !report.Cleaned {
		t.Error("Cleaned = false when the only scratch directory was removed")
	}
}

// TestRunBenchmarkStopsAtTheDeadlineAndCleansUp proves the run's own bound is honoured. The context is
// cancelled from inside the scratch hook, which guarantees the deadline is reached after the scratch
// directory exists and before the first file is written.
func TestRunBenchmarkStopsAtTheDeadlineAndCleansUp(t *testing.T) {
	dir := t.TempDir()
	before := snapshotDir(t, dir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	opts := benchTestOptions()
	opts.Scratch = func(targetDir, pattern string) (string, error) {
		path, err := os.MkdirTemp(targetDir, pattern)
		if err == nil {
			cancel()
		}
		return path, err
	}

	report := RunBenchmark(ctx, []BenchTarget{{Name: "project", Path: dir}}, opts)

	if !report.Cleaned {
		t.Error("Cleaned = false after the run was cut short; a scratch directory was left behind")
	}
	if after := snapshotDir(t, dir); !reflect.DeepEqual(before, after) {
		t.Errorf("the target changed after a cut-short run:\nbefore = %v\nafter = %v", before, after)
	}
	if !strings.Contains(strings.Join(report.Notes, " "), "cut short") {
		t.Errorf("the report does not say the run was cut short: %v", report.Notes)
	}
	if report.Results[0].Err == "" {
		t.Error("the cut-short target has no Err line")
	}
	for _, measurement := range report.Results[0].Measurements {
		if measurement.Duration <= 0 || measurement.PerSecond <= 0 {
			t.Errorf("%s row has a non-positive rate after a cut-short run: %+v", measurement.Operation, measurement)
		}
	}
}

// TestRunBenchmarkOperationsAreEquivalentAcrossTargets proves the comparison is meaningful: both
// targets get the same operations, in the same order, over the same file count and byte counts.
func TestRunBenchmarkOperationsAreEquivalentAcrossTargets(t *testing.T) {
	report := RunBenchmark(context.Background(), []BenchTarget{
		{Name: "a", Path: t.TempDir()},
		{Name: "b", Path: t.TempDir()},
	}, benchTestOptions())

	if len(report.Results) != 2 {
		t.Fatalf("got %d results, want 2", len(report.Results))
	}
	a, b := report.Results[0], report.Results[1]
	if len(a.Measurements) != len(b.Measurements) {
		t.Fatalf("measurement counts differ: %d and %d", len(a.Measurements), len(b.Measurements))
	}
	for i := range benchOperations {
		want := benchOperations[i]
		if a.Measurements[i].Operation != want || b.Measurements[i].Operation != want {
			t.Fatalf("operation %d = %q / %q, want %q for both",
				i, a.Measurements[i].Operation, b.Measurements[i].Operation, want)
		}
		if a.Measurements[i].Files != b.Measurements[i].Files {
			t.Errorf("%s file counts differ: %d and %d", want, a.Measurements[i].Files, b.Measurements[i].Files)
		}
		if a.Measurements[i].Bytes != b.Measurements[i].Bytes {
			t.Errorf("%s byte counts differ: %d and %d", want, a.Measurements[i].Bytes, b.Measurements[i].Bytes)
		}
	}
}

// TestRunBenchmarkNeverEmitsAZeroThroughputRow checks rule 4 at the report level: every row that
// exists has a positive duration and a positive rate, so a missing measurement is never disguised as
// a zero.
func TestRunBenchmarkNeverEmitsAZeroThroughputRow(t *testing.T) {
	report := RunBenchmark(context.Background(), []BenchTarget{{Name: "project", Path: t.TempDir()}}, benchTestOptions())

	for _, result := range report.Results {
		for _, measurement := range result.Measurements {
			if measurement.Duration <= 0 {
				t.Errorf("%s has duration %v, want positive", measurement.Operation, measurement.Duration)
			}
			if measurement.PerSecond <= 0 {
				t.Errorf("%s has PerSecond %v, want positive", measurement.Operation, measurement.PerSecond)
			}
			if measurement.Files <= 0 {
				t.Errorf("%s has Files %d, want positive", measurement.Operation, measurement.Files)
			}
		}
	}
}

// TestBenchOptionsNormalisedCapsTheSet checks the documented bound without writing 32 MiB: whatever a
// caller asks for, the normalised set stays inside the file and byte caps.
func TestBenchOptionsNormalisedCapsTheSet(t *testing.T) {
	cases := []struct {
		name        string
		options     BenchOptions
		wantFile    int64
		wantTimeout time.Duration
	}{
		{
			name:        "defaults",
			options:     BenchOptions{},
			wantFile:    defaultBenchFileSize,
			wantTimeout: defaultBenchTimeout,
		},
		{
			name:        "huge file size is capped",
			options:     BenchOptions{Files: 4, FileSize: maxBenchmarkBytes * 4},
			wantFile:    maxBenchmarkBytes,
			wantTimeout: defaultBenchTimeout,
		},
		{
			name:        "many files are capped",
			options:     BenchOptions{Files: maxBenchmarkFiles * 4, FileSize: 1},
			wantFile:    1,
			wantTimeout: defaultBenchTimeout,
		},
		{
			name:        "tiny file size does not explode the count",
			options:     BenchOptions{Files: 1 << 30, FileSize: 1},
			wantFile:    1,
			wantTimeout: defaultBenchTimeout,
		},
		{
			name:        "an explicit timeout is kept",
			options:     BenchOptions{Timeout: time.Minute},
			wantFile:    defaultBenchFileSize,
			wantTimeout: time.Minute,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			normalised, _ := tc.options.normalised()
			if normalised.Files <= 0 {
				t.Fatalf("Files = %d, want positive", normalised.Files)
			}
			if normalised.Files > maxBenchmarkFiles {
				t.Errorf("Files = %d, want at most %d", normalised.Files, maxBenchmarkFiles)
			}
			if normalised.FileSize <= 0 {
				t.Fatalf("FileSize = %d, want positive", normalised.FileSize)
			}
			if normalised.FileSize > maxBenchmarkBytes {
				t.Errorf("FileSize = %d, want at most %d", normalised.FileSize, maxBenchmarkBytes)
			}
			if total := int64(normalised.Files) * normalised.FileSize; total > maxBenchmarkBytes {
				t.Errorf("write pass would write %d bytes, want at most %d", total, maxBenchmarkBytes)
			}
			if normalised.FileSize != tc.wantFile {
				t.Errorf("FileSize = %d, want %d", normalised.FileSize, tc.wantFile)
			}
			if normalised.Timeout != tc.wantTimeout {
				t.Errorf("Timeout = %v, want %v", normalised.Timeout, tc.wantTimeout)
			}
			if normalised.Scratch == nil {
				t.Error("Scratch = nil after normalisation")
			}
		})
	}
}

// TestRunBenchmarkReportsTheReductionWhenItCaps proves a clamped request is disclosed rather than
// silently measured at a different size.
func TestRunBenchmarkReportsTheReductionWhenItCaps(t *testing.T) {
	report := RunBenchmark(context.Background(), []BenchTarget{{Name: "project", Path: t.TempDir()}},
		BenchOptions{Files: 4, FileSize: maxBenchmarkBytes * 4})

	if !strings.Contains(strings.Join(report.Notes, " "), "reduced") {
		t.Errorf("the report did not disclose that the requested set was reduced: %v", report.Notes)
	}
}

// TestCompare only returns a ratio when both sides really measured the operation.
func TestCompare(t *testing.T) {
	measured := func(op BenchOperation, rate float64) BenchResult {
		return BenchResult{
			Target: BenchTarget{Name: "t"},
			Measurements: []BenchMeasurement{
				{Operation: op, Files: 10, Duration: time.Second, PerSecond: rate, BytesPerSecond: rate * 8},
			},
		}
	}

	cases := []struct {
		name    string
		a       BenchResult
		b       BenchResult
		op      BenchOperation
		wantOK  bool
		wantVal float64
	}{
		{
			name:    "both measured",
			a:       measured(BenchWrite, 200),
			b:       measured(BenchWrite, 100),
			op:      BenchWrite,
			wantOK:  true,
			wantVal: 2,
		},
		{
			name:   "operation missing on one side",
			a:      measured(BenchWrite, 200),
			b:      measured(BenchRead, 100),
			op:     BenchWrite,
			wantOK: false,
		},
		{
			name:   "target with an Err yields nothing",
			a:      BenchResult{Err: "not measured", Measurements: []BenchMeasurement{{Operation: BenchWrite, Files: 1, Duration: time.Second, PerSecond: 10}}},
			b:      measured(BenchWrite, 100),
			op:     BenchWrite,
			wantOK: false,
		},
		{
			name:   "empty measurement set yields nothing",
			a:      BenchResult{},
			b:      measured(BenchWrite, 100),
			op:     BenchWrite,
			wantOK: false,
		},
		{
			name: "zero duration is not a measurement",
			a: BenchResult{Measurements: []BenchMeasurement{
				{Operation: BenchWrite, Files: 1, Duration: 0, PerSecond: 10},
			}},
			b:      measured(BenchWrite, 100),
			op:     BenchWrite,
			wantOK: false,
		},
		{
			name: "zero rate cannot be a denominator",
			a: BenchResult{Measurements: []BenchMeasurement{
				{Operation: BenchWrite, Files: 1, Duration: time.Second, PerSecond: 10},
			}},
			b: BenchResult{Measurements: []BenchMeasurement{
				{Operation: BenchWrite, Files: 0, Duration: time.Second, PerSecond: 0},
			}},
			op:     BenchWrite,
			wantOK: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := Compare(tc.a, tc.b, tc.op)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if ok && got != tc.wantVal {
				t.Errorf("ratio = %v, want %v", got, tc.wantVal)
			}
		})
	}
}

// TestRunBenchmarkContaminationStatesTheRequiredCaveats is the spec's honesty requirement: the first
// four caveats plus the recorded order must be present in every report.
func TestRunBenchmarkContaminationStatesTheRequiredCaveats(t *testing.T) {
	report := RunBenchmark(context.Background(), []BenchTarget{
		{Name: "this project", Path: t.TempDir()},
		{Name: "the distribution's storage", Path: t.TempDir()},
	}, benchTestOptions())

	joined := strings.ToLower(strings.Join(report.Contamination, "\n"))
	for _, phrase := range []string{
		"page cache",
		"antivirus",
		"virtualised",
		"single-threaded",
		"measured in this order",
	} {
		if !strings.Contains(joined, phrase) {
			t.Errorf("Contamination does not mention %q:\n%s", phrase, joined)
		}
	}
	if !strings.Contains(joined, "this project") || !strings.Contains(joined, "the distribution's storage") {
		t.Errorf("Contamination does not record the measurement order:\n%s", joined)
	}
}

// TestBenchSpaceIsLow pins the nearly-full threshold, including the total==0 case StorageSpace cannot
// produce but a caller's fixture can.
func TestBenchSpaceIsLow(t *testing.T) {
	cases := []struct {
		name      string
		free      uint64
		total     uint64
		wantIsLow bool
	}{
		{name: "plenty free", free: 10 << 30, total: 100 << 30, wantIsLow: false},
		{name: "under one gibibyte", free: 512 << 20, total: 100 << 30, wantIsLow: true},
		{name: "under one tenth", free: 5 << 30, total: 100 << 30, wantIsLow: true},
		{name: "unknown total and little free", free: 128 << 20, total: 0, wantIsLow: true},
		{name: "unknown total and plenty free", free: 10 << 30, total: 0, wantIsLow: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := benchSpaceIsLow(tc.free, tc.total); got != tc.wantIsLow {
				t.Errorf("benchSpaceIsLow(%d, %d) = %v, want %v", tc.free, tc.total, got, tc.wantIsLow)
			}
		})
	}
}
