package diskclean

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Clean runs a plan. It is the only entry point in this package that changes
// anything on the machine, and it does so one item at a time, with the guard
// re-checked immediately before every removal and with the result of each item
// measured rather than assumed.
//
// Two numbers come out of a run, and they answer two different questions. Each
// result's Freed says what that item's own tree or store gave back. The
// report's FreedBytes is the filesystem's own free-space delta around the whole
// run, which is the only number that cannot be fooled by a hardlink, a shared
// store or a sparse file -- and it is reported as unmeasured, with a reason,
// when the run's storage is not on a filesystem this process can ask.

// CleanOptions shape one run.
type CleanOptions struct {
	// Timeout bounds each item's command. It does not bound a removal, which is
	// bounded by the context and by Limits.
	Timeout time.Duration
	// Limits bound the measurements the run takes around each item.
	Limits Limits
	// Progress is called before each item with its position, from the cleaning
	// goroutine. It must not touch a model directly.
	Progress func(done, total int, item Item)
}

// Result is what happened to one item.
type Result struct {
	Item Item
	// Ran reports whether the item's mechanism was attempted.
	Ran bool
	// Freed is what the item gave back, measured: the difference between the
	// walk before and the walk after for a filesystem store, or the difference
	// between the engine's own accounting before and after for a container
	// store.
	Freed int64
	// FreedSource says how Freed was obtained.
	FreedSource Source
	// Before and After are the two measurements Freed is the difference of.
	Before Size
	After  Size
	// Output is what the command printed, for the cases where a reason lives
	// only there. It is trimmed and bounded.
	Output string
	// Error is why the item did not do what it promised, or why it could not
	// be attempted at all. An empty Error with a non-empty Freed is a clean
	// success.
	Error string
	// Refused reports that the guard refused the item at the last moment,
	// which is a different failure from a command that failed.
	Refused bool
	// Skipped names entries that could not be removed, with why; SkippedTotal
	// counts all of them.
	Skipped      []string
	SkippedTotal int
	// Cancelled reports that the user stopped the run before or during this
	// item.
	Cancelled bool
}

// Succeeded reports whether the item was attempted and nothing went wrong.
func (r Result) Succeeded() bool { return r.Ran && r.Error == "" && !r.Cancelled }

// CleanReport is the run as a whole.
type CleanReport struct {
	Results []Result
	// FreedBytes is the filesystem's own free-space delta around the run, when
	// it could be measured. It is the real release, and it is what the screen
	// leads with.
	FreedBytes int64
	// FreedMeasured says whether FreedBytes is a measurement. When it is
	// false, FreedNote says why not, and no zero is presented as an answer.
	FreedMeasured bool
	FreedNote     string
	// Cancelled reports that the user stopped the run.
	Cancelled bool
	StartedAt time.Time
	Duration  time.Duration
}

// FreedTotal is the sum of the per-item freed figures. It is a second view of
// the run, useful when the filesystem delta could not be taken; it is never
// presented as the filesystem's answer.
func (r CleanReport) FreedTotal() int64 {
	var total int64
	for _, result := range r.Results {
		total += result.Freed
	}
	return total
}

// Failed returns the results that did not succeed, so a screen can lead with
// them instead of hiding them under a total.
func (r CleanReport) Failed() []Result {
	var failed []Result
	for _, result := range r.Results {
		if !result.Succeeded() {
			failed = append(failed, result)
		}
	}
	return failed
}

// Clean runs a plan against the machine.
func Clean(ctx context.Context, probe Probe, guard Guard, plan Plan, opts CleanOptions) CleanReport {
	if opts.Timeout <= 0 {
		opts.Timeout = DefaultToolTimeout
	}
	limits := normalizeLimits(opts.Limits)

	report := CleanReport{StartedAt: probe.Now()}

	// The free-space reading is taken for every filesystem the run will touch,
	// before the first command runs. A path that is not there yet is looked up
	// through its parent, because a store that does not exist has no device of
	// its own to ask.
	before := make(map[uint64]int64, len(plan.Items))
	devicePaths := make(map[uint64]string, len(plan.Items))
	for _, planned := range plan.Items {
		if !planned.Runs() {
			continue
		}
		for _, path := range planned.Item.Paths {
			device, ok := deviceOfExisting(probe, path)
			if !ok {
				continue
			}
			if _, seen := before[device]; seen {
				continue
			}
			probePath := existingAncestor(probe, path)
			free, err := probe.FreeSpace(probePath)
			if err != nil {
				continue
			}
			before[device] = free
			devicePaths[device] = probePath
		}
	}

	for index, planned := range plan.Items {
		if err := ctx.Err(); err != nil {
			report.Cancelled = true
			report.Results = append(report.Results, Result{
				Item:      planned.Item,
				Cancelled: true,
				Error:     "the run was cancelled before this item was reached",
			})
			continue
		}
		if opts.Progress != nil {
			opts.Progress(index+1, len(plan.Items), planned.Item)
		}
		report.Results = append(report.Results, cleanOne(ctx, probe, guard, planned, limits, opts.Timeout))
	}

	if len(before) > 0 {
		measured := true
		var freed int64
		for device, beforeFree := range before {
			afterFree, err := probe.FreeSpace(existingAncestor(probe, devicePaths[device]))
			if err != nil {
				measured = false
				report.FreedNote = "the free space of one of the filesystems could not be read again after the run"
				continue
			}
			if delta := afterFree - beforeFree; delta > 0 {
				freed += delta
			}
		}
		report.FreedBytes = freed
		report.FreedMeasured = measured
	} else {
		report.FreedNote = "the run's storage is not on a filesystem this utility measured, so the release is " +
			"taken from the tool's own accounting for each item"
	}
	report.Duration = probe.Now().Sub(report.StartedAt)
	return report
}

// cleanOne runs one planned item.
func cleanOne(ctx context.Context, probe Probe, guard Guard, planned PlannedItem, limits Limits, timeout time.Duration) Result {
	result := Result{Item: planned.Item}
	if planned.Blocked != "" {
		result.Error = planned.Blocked
		return result
	}

	switch planned.Item.Mechanism.Kind {
	case MechanismTool:
		return cleanWithTool(ctx, probe, planned, limits, timeout)
	case MechanismRemove:
		return cleanWithRemoval(ctx, probe, guard, planned, limits)
	default:
		result.Error = "this finding is reported, not cleaned"
		return result
	}
}

// cleanWithTool runs the owner's own command and measures the difference it
// made.
//
// A command that needs administrator rights is attempted with `sudo -n`: the
// non-interactive flag means a missing credential is a refusal, not a prompt
// inside a full-screen interface. A refusal is reported with the exact command
// the user can run in a terminal.
func cleanWithTool(ctx context.Context, probe Probe, planned PlannedItem, limits Limits, timeout time.Duration) Result {
	item := planned.Item
	result := Result{Item: item, FreedSource: SourceWalk}

	var before Size
	if len(item.Paths) > 0 {
		before = measurePaths(ctx, probe, item.Paths, limits)
		result.Before = before
	}

	var beforeStore int64
	var beforeUsage []toolStore
	if item.StoreKind != "" {
		beforeUsage = containerUsage(ctx, probe, item.Mechanism.Program, item.Tool, timeout)
		beforeStore = storeReclaimable(beforeUsage, item.StoreKind)
	}

	program := item.Mechanism.Program
	args := append([]string(nil), item.Mechanism.Args...)
	if item.Mechanism.RequiresRoot {
		args = append([]string{"-n", program}, args...)
		program = "sudo"
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	output, err := probe.Run(runCtx, program, args...)
	cancel()

	result.Ran = true
	result.Output = boundedOutput(output)

	if err != nil {
		if item.Mechanism.RequiresRoot {
			result.Error = fmt.Sprintf("the command needs administrator rights and sudo refused without a prompt (%v); "+
				"run `%s` in a terminal to clean it", err, item.Mechanism.Command())
		} else {
			result.Error = fmt.Sprintf("%s failed: %v", item.Mechanism.Command(), err)
		}
	}

	switch {
	case len(item.Paths) > 0:
		after := measurePaths(ctx, probe, item.Paths, limits)
		result.After = after
		result.Freed = positiveDelta(before.Physical, after.Physical)
		result.FreedSource = SourceWalk
	case item.StoreKind != "":
		afterUsage := containerUsage(ctx, probe, item.Mechanism.Program, item.Tool, timeout)
		result.Freed = positiveDelta(beforeStore, storeReclaimable(afterUsage, item.StoreKind))
		result.FreedSource = SourceTool
		result.Before = Size{Physical: beforeStore, Logical: beforeStore, Source: SourceTool}
		result.After = Size{Physical: beforeStore - result.Freed, Logical: beforeStore - result.Freed, Source: SourceTool}
	}
	return result
}

// cleanWithRemoval unlinks the item's paths, one path at a time, with the guard
// re-checked immediately before each one. A path the guard refuses is reported
// and the others still run: a partial cleanup that says which half happened is
// better than a run that stops at the first refusal.
func cleanWithRemoval(ctx context.Context, probe Probe, guard Guard, planned PlannedItem, limits Limits) Result {
	result := Result{Item: planned.Item, FreedSource: SourceWalk}

	for _, approval := range planned.Approvals {
		if ctx.Err() != nil {
			result.Cancelled = true
			if result.Error == "" {
				result.Error = "the run was cancelled before this path was removed"
			}
			break
		}
		if err := guard.Verify(approval); err != nil {
			result.Refused = true
			result.Error = joinReasons(result.Error, err.Error())
			continue
		}
		removal := removeTree(ctx, probe, approval.Path, limits)
		result.Ran = true
		result.Freed += removal.RemovedBytes
		result.SkippedTotal += removal.SkippedTotal
		result.Skipped = append(result.Skipped, removal.Skipped...)
		if removal.Cancelled {
			result.Cancelled = true
		}
		if removal.Err != "" {
			result.Error = joinReasons(result.Error, removal.Err)
		}
	}
	if !result.Ran && result.Error == "" {
		result.Error = "there was nothing to remove: no path of this item was approved"
	}
	return result
}

// measurePaths measures the item's paths with a dedupe of its own, which is
// what makes a before/after pair comparable: both walks attribute shared files
// the same way.
func measurePaths(ctx context.Context, probe Probe, paths []string, limits Limits) Size {
	dedupe := NewDedupe()
	size := Size{Source: SourceWalk}
	for _, path := range paths {
		walk := Walk(ctx, probe, path, "clean", dedupe, limits)
		size.Logical += walk.Logical
		size.Physical += walk.Physical
		size.Files += walk.Files
		size.Shared += walk.Shared
		size.SharedBytes += walk.SharedBytes
		if walk.Err != "" && size.Err == "" {
			size.Err = walk.Err
		}
	}
	if size.Files == 0 && size.Logical == 0 {
		size.Missing = true
	}
	return size
}

// storeReclaimable reads one store's reclaimable bytes out of an engine's
// accounting.
func storeReclaimable(stores []toolStore, kind string) int64 {
	for _, store := range stores {
		if store.Kind == kind {
			return store.Reclaimable
		}
	}
	return 0
}

// positiveDelta is the difference when it is a release, and zero when the
// measurement moved the other way -- a store that grew while the command ran
// did not free anything, and a negative release is not a number to report.
func positiveDelta(before, after int64) int64 {
	if after >= before {
		return 0
	}
	return before - after
}

// deviceOfExisting returns the device a path lives on, looking through its
// parent when the path itself is not there.
func deviceOfExisting(probe Probe, path string) (uint64, bool) {
	if info, err := probe.Lstat(path); err == nil {
		if device := deviceOf(info); device != 0 {
			return device, true
		}
		return 0, false
	}
	info, err := probe.Lstat(filepathDir(path))
	if err != nil {
		return 0, false
	}
	device := deviceOf(info)
	return device, device != 0
}

// spaceProbePath is the path the free-space reading is taken from: the store
// itself when it exists, and its parent when it does not.
func spaceProbePath(probe Probe, path string) string {
	if _, err := probe.Lstat(path); err == nil {
		return path
	}
	return filepathDir(path)
}

// existingAncestor returns path when it is there and the nearest ancestor that
// is when it is not. It is what lets the free-space reading be taken again
// after the store has been removed: the store's own path is gone by then, and
// the filesystem holding it is asked through the closest directory that still
// exists.
func existingAncestor(probe Probe, path string) string {
	current := path
	for {
		if _, err := probe.Lstat(current); err == nil {
			return current
		}
		parent := filepathDir(current)
		if parent == current {
			return current
		}
		current = parent
	}
}

// filepathDir is the parent of a path, without importing filepath into a file
// whose other paths are already built by hand.
func filepathDir(path string) string {
	trimmed := strings.TrimRight(path, "/")
	if index := strings.LastIndex(trimmed, "/"); index > 0 {
		return trimmed[:index]
	}
	return "/"
}

// boundedOutput trims a command's output to something a screen can show,
// keeping the head, which is where a failure explains itself.
func boundedOutput(output []byte) string {
	const limit = 2000
	text := strings.TrimSpace(string(output))
	if len(text) > limit {
		return text[:limit] + "…"
	}
	return text
}

// joinReasons appends a reason to a list of them without repeating a separator
// the caller already put in.
func joinReasons(existing, reason string) string {
	if existing == "" {
		return reason
	}
	if reason == "" {
		return existing
	}
	return existing + "; " + reason
}
