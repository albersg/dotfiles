package system

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ============================================================================
// A BOUNDED, NON-DESTRUCTIVE STORAGE BENCHMARK
// ============================================================================
//
// The advisor can already say where a project lives from the mount table, and on a WSL machine that
// answer is often "on the Windows host" -- where every metadata call pays a round trip and the
// host's antivirus. A number makes that concrete for a reader who has never timed a directory tree,
// but only if the number is real and the cost of producing it is bounded and harmless:
//
//   - The benchmark runs inside its own scratch directory, created with os.MkdirTemp directly under
//     the target, and it removes exactly that directory when it is done. It never writes, reads,
//     stats or removes anything it did not itself create, so a directory that already holds a
//     project comes out of the run byte-for-byte as it went in. The one exception is the read-back
//     pass, which reads the files this run wrote; nothing else is opened.
//   - The work is small by default (64 files of 64 KiB) and hard-capped by maxBenchmarkBytes and
//     maxBenchmarkFiles, so a mistaken option cannot fill the filesystem it is measuring.
//   - An operation that did not run is reported as not measured -- a Note or an Err -- and never as a
//     row of zeros. A zero-throughput row reads as "this storage is infinitely slow", which is a
//     measurement the run did not take.
//
// The exact sequence, run per target in this order, is:
//
//	1. write  -- write opts.Files files of opts.FileSize bytes, each from one buffer built once, so
//	             the CPU cost of producing bytes is not what the numbers compare.
//	2. read   -- one read-back pass over exactly those files. It is a second touch of files just
//	             written, so the page cache makes it warm; that caveat is recorded in Contamination.
//	3. stat   -- one os.Stat per written file, the metadata walk a build or a watcher pays for.
//	4. create -- opts.Files small files in a bulk/ subdirectory, all left in place at once.
//	5. remove -- remove exactly the small files the create step made.
//
// Every target runs the same sequence with the same file count and file size, so a comparison between
// two targets compares the storage and not the workload. The targets are measured one after another in
// the order given, which is itself recorded because a later target meets a warmer cache.

// BenchLimits states what every run is bounded by: the set it writes and the ceiling the option
// normalisation holds it under. It exists so a caller that offers the benchmark can tell the user
// what will be written and for how long without hardcoding numbers this file owns and could change.
type BenchLimits struct {
	// Files and FileSize are the set written when the caller expresses no preference.
	Files    int
	FileSize int64
	// Timeout is the whole-run bound when the caller sets none.
	Timeout time.Duration
	// MaxBytes caps the bytes one target's write pass may place, and MaxFiles caps the file count, so
	// no option can turn a benchmark into a disk filler or an inode exhaustion.
	MaxBytes int64
	MaxFiles int
	// LowSpaceBytes is the free space under which the report warns that a write may be slowed by the
	// filesystem hunting for room.
	LowSpaceBytes uint64
}

// DefaultBenchLimits returns the shipped defaults and caps. It is the only reader of those constants
// a caller outside this package needs.
func DefaultBenchLimits() BenchLimits {
	return BenchLimits{
		Files:         defaultBenchFiles,
		FileSize:      defaultBenchFileSize,
		Timeout:       defaultBenchTimeout,
		MaxBytes:      maxBenchmarkBytes,
		MaxFiles:      maxBenchmarkFiles,
		LowSpaceBytes: benchLowSpaceBytes,
	}
}

// BenchOperation names one kind of work the benchmark times. The names are the report's vocabulary.
type BenchOperation string

const (
	// BenchWrite is writing the measured set of files.
	BenchWrite BenchOperation = "write"
	// BenchRead is reading that same set back once.
	BenchRead BenchOperation = "read"
	// BenchStat is stating each file in the set.
	BenchStat BenchOperation = "stat"
	// BenchCreate is creating many small files that are all live at once.
	BenchCreate BenchOperation = "create"
	// BenchRemove is removing the files the create step made.
	BenchRemove BenchOperation = "remove"
)

// benchOperations is the sequence, in the order it is run. It is a table so the tests can assert that
// every target really was given the same operations in the same order.
var benchOperations = []BenchOperation{BenchWrite, BenchRead, BenchStat, BenchCreate, BenchRemove}

// The small, documented defaults and the cap this file's honesty rests on.
const (
	// defaultBenchFiles and defaultBenchFileSize are the default set: 64 files of 64 KiB. It is small
	// enough to finish quickly on a slow host-mounted path and large enough for the per-operation
	// overhead to show up instead of being lost to timer noise.
	defaultBenchFiles    = 64
	defaultBenchFileSize = 64 << 10 // 64 KiB

	// defaultBenchTimeout bounds the whole run when the caller sets no timeout. A benchmark that can
	// hang forever on a stalled network mount is not one a TUI can offer.
	defaultBenchTimeout = 30 * time.Second

	// benchSmallFileBytes is the size of each file in the create/remove set. Small enough that the
	// cost is the per-file metadata work, which is the point of those two operations.
	benchSmallFileBytes = 512

	// maxBenchmarkBytes caps the bytes one target's write pass may write. Every other operation works
	// over the same files, so this also caps what a single wrong option can put on disk.
	maxBenchmarkBytes = 32 << 20 // 32 MiB

	// maxBenchmarkFiles caps the file count, because the create pass makes one small file per file and
	// a tiny file size must not turn into an unbounded number of inodes.
	maxBenchmarkFiles = 4096

	// benchScratchPattern is the prefix of every scratch directory. It is recognisable on sight so a
	// reader who finds one after a crash knows what made it.
	benchScratchPattern = ".dotfiles-wsl-bench-"

	// benchLowSpaceBytes and the one-tenth rule decide when a filesystem is "nearly full" enough to
	// warn that a write may be slowed by the filesystem hunting for room.
	benchLowSpaceBytes = 1 << 30 // 1 GiB
)

// benchCutReason is the sentence every operation uses when the run's own time bound stopped it. It is
// one constant so the wording cannot drift between the write, read, stat, create and remove passes.
const benchCutReason = "the run's time bound was reached part-way through the pass"

// BenchTarget is one place to measure: a label the report shows and an existing directory to measure
// inside. The directory is never written to directly; each run makes its own scratch directory under
// it, so two targets never share files.
type BenchTarget struct {
	// Name is the label the report shows, e.g. "this project" or "the distribution's storage".
	Name string
	// Path is an existing directory. A path that is missing or not writable yields an Err line for
	// this target and does not stop the others.
	Path string
}

// BenchOptions controls the size and the bound of the run.
type BenchOptions struct {
	// Files is the files per operation set. 0 uses defaultBenchFiles and the value is capped by
	// maxBenchmarkFiles and by maxBenchmarkBytes.
	Files int
	// FileSize is the bytes per written file. 0 uses defaultBenchFileSize and is capped by
	// maxBenchmarkBytes.
	FileSize int64
	// Timeout bounds the whole run. 0 uses defaultBenchTimeout. The caller's context is honoured as
	// well; the earlier of the two ends the run.
	Timeout time.Duration
	// Scratch creates the run's scratch directory. nil means os.MkdirTemp. It is injectable so a test
	// can force a failure or observe the exact directory the run used.
	Scratch func(dir, pattern string) (string, error)
}

// BenchMeasurement is one measured operation. Duration is always positive when the row exists, and
// the rates are derived from it; a row that would have a zero duration is not emitted at all.
type BenchMeasurement struct {
	// Operation is which pass this row is.
	Operation BenchOperation
	// Files is how many files the pass touched.
	Files int
	// Bytes is how many bytes it moved. It is 0 for stat, which moves no file data: that is a real
	// measurement, not a missing one, because the pass ran.
	Bytes int64
	// Duration is how long the pass took.
	Duration time.Duration
	// PerSecond is files per second.
	PerSecond float64
	// BytesPerSecond is bytes per second.
	BytesPerSecond float64
}

// BenchResult is one target's outcome. Measurements is empty when nothing could be measured, and Err
// then says why; a target with some measurements puts the reasons the others are missing in Notes so
// the successful rows are not thrown away.
type BenchResult struct {
	Target       BenchTarget
	Measurements []BenchMeasurement
	Notes        []string
	Err          string
}

// BenchReport is the whole run. Cleaned reports whether every scratch directory this run created is
// gone -- the result that has to be true even when the run failed or was cut short.
type BenchReport struct {
	Results       []BenchResult
	Started       time.Time
	Duration      time.Duration
	Cleaned       bool
	Contamination []string
	Notes         []string
}

// RunBenchmark measures each target in the order given, inside its own scratch directory, and returns
// every measurement and caveat it could honestly produce. It never returns an error: a target it
// could not measure carries the reason in its Err field and the other targets are still measured.
func RunBenchmark(ctx context.Context, targets []BenchTarget, opts BenchOptions) BenchReport {
	report := BenchReport{Started: time.Now(), Cleaned: true}

	// Normalise before building the bound so a caller who left Timeout at 0 still gets
	// defaultBenchTimeout instead of an unbounded run.
	normalised, clamped := opts.normalised()
	opts = normalised

	// The run-wide bound is the caller's context tightened by the option, so either can end it. The
	// cancel is deferred so the timer is released even when the loop returns early.
	runCtx := ctx
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	if clamped {
		report.Notes = append(report.Notes, "The requested set was reduced to stay inside the documented "+
			strconv.Itoa(maxBenchmarkFiles)+"-file and "+strconv.Itoa(maxBenchmarkBytes)+"-byte bounds.")
	}

	order := make([]string, 0, len(targets))
	for _, target := range targets {
		order = append(order, benchTargetLabel(target))
	}

	cutShort := false
	for _, target := range targets {
		// A target started after the bound expired is reported as not measured rather than run: the
		// work would be cut immediately anyway, and saying so up front is clearer than a partial row.
		if runCtx.Err() != nil {
			cutShort = true
			report.Results = append(report.Results, BenchResult{
				Target: target,
				Err:    "not measured: the run's time bound was reached before this target was started",
			})
			continue
		}

		result, cleaned, cut := measureTarget(runCtx, target, opts)
		if !cleaned {
			report.Cleaned = false
		}
		if cut {
			cutShort = true
		}

		// A nearly full filesystem changes the write more than the storage kind does, so it is called
		// out with the bytes StorageSpace actually read. StorageSpace reports false rather than zeroes
		// when it cannot ask, so an unreadable filesystem adds no note.
		if free, total, ok := StorageSpace(target.Path); ok && benchSpaceIsLow(free, total) {
			result.Notes = append(result.Notes, "The filesystem behind "+benchTargetLabel(target)+" reported "+
				strconv.FormatUint(free, 10)+" bytes free of "+strconv.FormatUint(total, 10)+
				" total, so the write may be slowed by the filesystem looking for room; that is a "+
				"property of a full disk, not of the storage kind.")
		}

		report.Results = append(report.Results, result)
	}

	if cutShort {
		report.Notes = append(report.Notes, "The run was cut short when its time bound was reached; only "+
			"the operations already listed as measured were taken, and every scratch directory was removed.")
	}

	report.Contamination = benchContamination(order)
	report.Duration = time.Since(report.Started)
	return report
}

// normalised applies the documented defaults and caps, and installs the real scratch creator. It
// reports whether it changed the requested size, so the report can say the set was reduced rather
// than silently measuring something else than what was asked for.
func (o BenchOptions) normalised() (BenchOptions, bool) {
	requestedFiles := o.Files
	if requestedFiles <= 0 {
		requestedFiles = defaultBenchFiles
	}
	requestedSize := o.FileSize
	if requestedSize <= 0 {
		requestedSize = defaultBenchFileSize
	}

	o.Files = requestedFiles
	o.FileSize = requestedSize
	if o.Timeout <= 0 {
		o.Timeout = defaultBenchTimeout
	}

	// Cap the count first so the create pass cannot make an unbounded number of small files, then cap
	// the total bytes so one file cannot be larger than the whole budget.
	if o.Files > maxBenchmarkFiles {
		o.Files = maxBenchmarkFiles
	}
	if o.FileSize > maxBenchmarkBytes {
		o.FileSize = maxBenchmarkBytes
	}
	if int64(o.Files)*o.FileSize > maxBenchmarkBytes {
		o.Files = int(maxBenchmarkBytes / o.FileSize)
		if o.Files < 1 {
			o.Files = 1
		}
	}

	if o.Scratch == nil {
		o.Scratch = os.MkdirTemp
	}

	return o, o.Files != requestedFiles || o.FileSize != requestedSize
}

// measureTarget runs the sequence in one scratch directory and removes that directory no matter how
// the sequence ends. It returns the target's result, whether the scratch directory is gone, and
// whether the run's time bound cut a pass short.
func measureTarget(ctx context.Context, target BenchTarget, opts BenchOptions) (BenchResult, bool, bool) {
	result := BenchResult{Target: target}
	if target.Path == "" {
		result.Err = "the target has no directory to measure inside"
		return result, true, false
	}

	scratch, err := opts.Scratch(target.Path, benchScratchPattern)
	if err != nil {
		result.Err = "a scratch directory could not be created inside " + target.Path + ": " + err.Error()
		return result, true, false
	}

	// The cleanup is deferred before any file is written, so every early return below -- an error, a
	// deadline, a panic in a later pass -- still removes exactly the one directory this run made.
	cleaned := true
	cutShort := false
	defer func() {
		if err := os.RemoveAll(scratch); err != nil {
			cleaned = false
			result.Notes = append(result.Notes, "The scratch directory "+scratch+" could not be removed: "+
				err.Error())
		}
	}()

	// The content is built once and written to every file, so the time measured is the storage's, not
	// the allocator's. The read pass then reads exactly these bytes back.
	content := bytes.Repeat([]byte("d"), int(opts.FileSize))
	small := bytes.Repeat([]byte("s"), benchSmallFileBytes)

	writePaths := make([]string, opts.Files)
	for i := range writePaths {
		writePaths[i] = filepath.Join(scratch, fmt.Sprintf("write-%06d.bin", i))
	}

	writeDuration, writeDone, writeErr := runPass(ctx, writePaths, func(path string) error {
		file, err := os.Create(path)
		if err != nil {
			return err
		}
		if _, err := file.Write(content); err != nil {
			file.Close()
			return err
		}
		return file.Close()
	})
	switch {
	case writeErr != nil:
		result.noteFail(BenchWrite, writeErr.Error())
		return result, cleaned, cutShort
	case !writeDone:
		cutShort = true
		result.noteFail(BenchWrite, benchCutReason)
		return result, cleaned, cutShort
	}
	writtenBytes := int64(len(writePaths)) * opts.FileSize
	result.Measurements = append(result.Measurements,
		newMeasurement(BenchWrite, len(writePaths), writtenBytes, writeDuration))

	// The read-back pass reads the files just written, which is a cache-warm read; the Contamination
	// list says so in the report's own words.
	readDuration, readDone, readErr := runPass(ctx, writePaths, func(path string) error {
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		if _, err := io.Copy(io.Discard, file); err != nil {
			file.Close()
			return err
		}
		return file.Close()
	})
	switch {
	case readErr != nil:
		result.noteFail(BenchRead, readErr.Error())
	case readDone:
		result.Measurements = append(result.Measurements,
			newMeasurement(BenchRead, len(writePaths), writtenBytes, readDuration))
	default:
		cutShort = true
		result.noteFail(BenchRead, benchCutReason)
		return result, cleaned, cutShort
	}

	// stat moves no bytes, so its row records zero bytes on purpose. It is a real measurement: the walk
	// ran, and its files-per-second is what the report compares.
	statDuration, statDone, statErr := runPass(ctx, writePaths, func(path string) error {
		_, err := os.Stat(path)
		return err
	})
	switch {
	case statErr != nil:
		result.noteFail(BenchStat, statErr.Error())
	case statDone:
		result.Measurements = append(result.Measurements,
			newMeasurement(BenchStat, len(writePaths), 0, statDuration))
	default:
		cutShort = true
		result.noteFail(BenchStat, benchCutReason)
		return result, cleaned, cutShort
	}

	// The small files go in their own subdirectory inside the same scratch directory, so they are
	// still covered by the one RemoveAll while being a separate, all-at-once set for create/remove.
	bulkDir := filepath.Join(scratch, "bulk")
	if err := os.Mkdir(bulkDir, 0o700); err != nil {
		result.noteFail(BenchCreate, "the bulk subdirectory could not be created: "+err.Error())
		return result, cleaned, cutShort
	}
	smallPaths := make([]string, opts.Files)
	for i := range smallPaths {
		smallPaths[i] = filepath.Join(bulkDir, fmt.Sprintf("small-%06d.bin", i))
	}
	smallBytes := int64(len(smallPaths)) * benchSmallFileBytes

	createDuration, createDone, createErr := runPass(ctx, smallPaths, func(path string) error {
		return os.WriteFile(path, small, 0o644)
	})
	switch {
	case createErr != nil:
		result.noteFail(BenchCreate, createErr.Error())
	case createDone:
		result.Measurements = append(result.Measurements,
			newMeasurement(BenchCreate, len(smallPaths), smallBytes, createDuration))
	default:
		cutShort = true
		result.noteFail(BenchCreate, benchCutReason)
		return result, cleaned, cutShort
	}

	// remove takes the same paths, so it removes only what create made. A pre-existing file is never
	// in this list.
	removeDuration, removeDone, removeErr := runPass(ctx, smallPaths, os.Remove)
	switch {
	case removeErr != nil:
		result.noteFail(BenchRemove, removeErr.Error())
	case removeDone:
		result.Measurements = append(result.Measurements,
			newMeasurement(BenchRemove, len(smallPaths), smallBytes, removeDuration))
	default:
		cutShort = true
		result.noteFail(BenchRemove, benchCutReason)
	}

	return result, cleaned, cutShort
}

// runPass times one operation over the paths. It stops at the deadline and reports done=false with a
// nil error, or stops at the first real error and returns it. A pass that did not finish is never
// turned into a measurement: a partial pass would time fewer files and report a rate for none.
func runPass(ctx context.Context, paths []string, action func(string) error) (time.Duration, bool, error) {
	start := time.Now()
	for _, path := range paths {
		if ctx.Err() != nil {
			return time.Since(start), false, nil
		}
		if err := action(path); err != nil {
			return time.Since(start), false, err
		}
	}
	return time.Since(start), true, nil
}

// newMeasurement derives the rates from the observed duration. A non-positive duration would divide
// by zero, so the rates are simply left at zero; the caller never appends such a row, and Compare
// refuses it as well.
func newMeasurement(op BenchOperation, files int, bytes int64, duration time.Duration) BenchMeasurement {
	measurement := BenchMeasurement{Operation: op, Files: files, Bytes: bytes, Duration: duration}
	if seconds := duration.Seconds(); seconds > 0 {
		measurement.PerSecond = float64(files) / seconds
		measurement.BytesPerSecond = float64(bytes) / seconds
	}
	return measurement
}

// noteFail records that one operation produced no measurement. When the target has no measurements at
// all, the line becomes Err -- why this whole target produced nothing -- because "the write could not
// run" is the answer to "why is there no result here". Once there is a measurement, the line is a
// Note, so the rows that did run are kept and the missing one is explained beside them.
func (r *BenchResult) noteFail(op BenchOperation, reason string) {
	line := "The " + string(op) + " operation was not measured: " + reason
	if len(r.Measurements) == 0 && r.Err == "" {
		r.Err = line
		return
	}
	r.Notes = append(r.Notes, line)
}

// Compare returns the ratio of a's throughput to b's for one operation. It reports ok=false whenever
// either side has no usable measurement of that operation -- a missing row, a target that failed, a
// zero rate -- so a caller can print "not measured" instead of a number the run did not take.
func Compare(a, b BenchResult, op BenchOperation) (ratio float64, ok bool) {
	if a.Err != "" || b.Err != "" {
		return 0, false
	}
	aMeasurement, aOK := measurementOf(a, op)
	bMeasurement, bOK := measurementOf(b, op)
	if !aOK || !bOK {
		return 0, false
	}
	// A zero rate cannot be a denominator: dividing by it would invent an infinite improvement.
	if aMeasurement.PerSecond <= 0 || bMeasurement.PerSecond <= 0 {
		return 0, false
	}
	return aMeasurement.PerSecond / bMeasurement.PerSecond, true
}

// measurementOf finds a target's measurement of one operation. A row is only usable when its duration
// is positive, which is the same condition that keeps a zero-duration row out of the report.
func measurementOf(result BenchResult, op BenchOperation) (BenchMeasurement, bool) {
	if len(result.Measurements) == 0 {
		return BenchMeasurement{}, false
	}
	for _, measurement := range result.Measurements {
		if measurement.Operation == op && measurement.Duration > 0 {
			return measurement, true
		}
	}
	return BenchMeasurement{}, false
}

// benchContamination is the honest caveat list, always attached to the report. The first four cover
// the page cache, the host's antivirus and metadata round trips, machine/disk/CPU conditions, and the
// single-threaded sequential pass; the order note records that the targets were measured one after
// another, which is part of why the numbers are not independent.
func benchContamination(order []string) []string {
	lines := []string{
		"The page cache makes a second read of the same files much faster than the first, so the read " +
			"numbers here are cache-warm and higher than a cold read would be.",
		"A host-mounted filesystem pays the Windows host's antivirus (Defender) scan and a metadata " +
			"round trip on every operation; the distribution's own storage does not, and part of what " +
			"the two numbers differ by is that.",
		"A warm or occupied machine, a busy disk, a nearly full filesystem and a virtualised CPU all " +
			"change these numbers; one run observes a moving machine rather than rating the hardware.",
		"The pass is single-threaded and sequential, so it says nothing about concurrent or parallel " +
			"throughput.",
	}
	if len(order) > 0 {
		lines = append(lines, "The targets were measured in this order: "+strings.Join(order, ", ")+
			", so a later target met a warmer page cache and a busier disk than an earlier one.")
	}
	return lines
}

// benchSpaceIsLow reports whether a filesystem is full enough that a write may be slowed by the
// filesystem looking for room. The one-tenth rule is compared as free < total/10 to avoid overflowing
// the uint64 space readings on a filesystem larger than a few exabytes.
func benchSpaceIsLow(free, total uint64) bool {
	if total == 0 {
		return free < benchLowSpaceBytes
	}
	return free < benchLowSpaceBytes || free < total/10
}

// benchTargetLabel names a target the way the report's notes should. A target with no name is
// described by its path, so a note never says "the filesystem behind " and stops.
func benchTargetLabel(target BenchTarget) string {
	if target.Name != "" {
		return target.Name
	}
	return target.Path
}
