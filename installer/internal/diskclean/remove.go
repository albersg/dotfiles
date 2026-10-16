package diskclean

import (
	"context"
	"fmt"
	"os"
	"sort"
)

// removeTree is the only code in this package that unlinks anything, and it is
// written the way the guard demands: the tree is surveyed first and removed
// afterwards, so a tree that turns out to contain a mount point is refused
// whole rather than half-emptied; every unlink is a single probe.Remove; and a
// failure on one entry is a recorded reason, not the end of the run.
//
// It removes bottom-up: children before the directory that holds them, which
// is the only order that leaves no directory behind.

// RemovalResult is what one removeTree did.
type RemovalResult struct {
	// RemovedBytes is the allocation of the files that were really unlinked,
	// counted once per file even when two names pointed at it.
	RemovedBytes int64
	// RemovedFiles is how many files were unlinked.
	RemovedFiles int
	// Skipped names the entries that could not be removed, with why; the count
	// of all of them is SkippedTotal.
	Skipped      []string
	SkippedTotal int
	// Mounts names the subdirectories that turned out to live on another
	// filesystem. A tree containing one is refused whole.
	Mounts []string
	// Cancelled reports that the context ended the removal after it had already
	// begun.
	Cancelled bool
	// Err is why the tree as a whole could not be removed.
	Err string
}

// entry is one unlink to make, collected before anything is removed.
type entry struct {
	path     string
	dir      bool
	size     int64
	identity fileID
}

// removeTree unlinks root and everything below it.
func removeTree(ctx context.Context, probe Probe, root string, limits Limits) RemovalResult {
	var result RemovalResult
	limits = normalizeLimits(limits)

	info, err := probe.Lstat(root)
	if err != nil {
		result.Err = fmt.Sprintf("%s could not be inspected: %v", root, err)
		return result
	}
	if info.Mode()&os.ModeSymlink != 0 {
		result.Err = fmt.Sprintf("%s is a symbolic link, and a link is not removed as a tree", root)
		return result
	}
	if !info.IsDir() {
		// A single file: the survey is the file itself.
		if err := probe.Remove(root); err != nil {
			result.Err = fmt.Sprintf("%s could not be removed: %v", root, err)
			return result
		}
		result.RemovedBytes += physicalSize(info)
		result.RemovedFiles = 1
		return result
	}

	rootDevice := deviceOf(info)
	entries, mounts, surveyErr := surveyTree(ctx, probe, root, rootDevice, limits)
	if surveyErr != "" {
		result.Err = surveyErr
		return result
	}
	if len(mounts) > 0 {
		// Nothing has been removed yet, which is the point of surveying first:
		// a tree that contains another filesystem is refused whole, because
		// deleting around a mount would leave the store half-empty and the
		// other filesystem's bytes untouched.
		result.Mounts = mounts
		result.Err = fmt.Sprintf("%s contains %d mount point(s), the first at %s: a tree that contains another filesystem is not removed",
			root, len(mounts), mounts[0])
		return result
	}
	if ctx.Err() != nil {
		// The survey was stopped part way through, so what it collected is only
		// part of the tree. Removing that part would leave a tree the user never
		// approved, so nothing is removed at all.
		result.Cancelled = true
		result.Err = "the run was cancelled before the tree was surveyed, so nothing of it was removed"
		return result
	}

	// Children before their parent, which is the order the survey recorded
	// them in, reversed.
	seen := make(map[fileID]bool, len(entries))
	for i := len(entries) - 1; i >= 0; i-- {
		if ctx.Err() != nil {
			result.Cancelled = true
			break
		}
		current := entries[i]
		if err := probe.Remove(current.path); err != nil {
			result.SkippedTotal++
			if limits.MaxReasons <= 0 || len(result.Skipped) < limits.MaxReasons {
				result.Skipped = append(result.Skipped, fmt.Sprintf("%s could not be removed: %v", current.path, err))
			}
			continue
		}
		if !current.dir {
			result.RemovedFiles++
			// A file reachable twice inside the same tree counts once: the
			// second name frees nothing.
			if id := current.identity; id.valid() {
				if seen[id] {
					continue
				}
				seen[id] = true
			}
			result.RemovedBytes += current.size
		}
	}

	if result.Cancelled {
		return result
	}
	if len(result.Skipped) > 0 {
		result.Err = fmt.Sprintf("%d of the %d entries under %s could not be removed", result.SkippedTotal, len(entries), root)
	}
	if err := probe.Remove(root); err != nil {
		result.SkippedTotal++
		if result.Err == "" {
			result.Err = fmt.Sprintf("%s could not be removed: %v", root, err)
		}
	}
	return result
}

// surveyTree collects every entry under root, parent before child, without
// following a link and without descending into another filesystem. The
// directories of another filesystem are returned as mounts rather than
// entered.
func surveyTree(ctx context.Context, probe Probe, root string, rootDevice uint64, limits Limits) ([]entry, []string, string) {
	var entries []entry
	var mounts []string
	stack := []string{root}
	counted := 0

	for len(stack) > 0 {
		if err := ctx.Err(); err != nil {
			// A cancelled survey walks no further; the removal that follows
			// still removes what was already surveyed.
			break
		}
		if limits.MaxFiles > 0 && counted >= limits.MaxFiles {
			break
		}
		dir := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		children, err := probe.ReadDir(dir)
		if err != nil {
			// An unreadable directory cannot be emptied, so it is not removed:
			// the run says so instead of leaving a half-tree behind.
			return nil, nil, fmt.Sprintf("%s could not be listed, so its tree is not removed: %v", dir, err)
		}
		for _, child := range children {
			childInfo, infoErr := child.Info()
			if infoErr != nil {
				return nil, nil, fmt.Sprintf("%s could not be inspected, so its tree is not removed: %v", entryPath(dir, child.Name()), infoErr)
			}
			path := entryPath(dir, child.Name())
			switch {
			case childInfo.Mode()&os.ModeSymlink != 0:
				// A link is unlinked itself; its target is never touched.
				entries = append(entries, entry{path: path})
			case childInfo.IsDir():
				if device := deviceOf(childInfo); device != 0 && rootDevice != 0 && device != rootDevice {
					mounts = append(mounts, path)
					continue
				}
				entries = append(entries, entry{path: path, dir: true})
				stack = append(stack, path)
			case childInfo.Mode().IsRegular():
				counted++
				entries = append(entries, entry{
					path:     path,
					size:     physicalSize(childInfo),
					identity: probe.identity(path, childInfo),
				})
			}
		}
	}
	return entries, mounts, ""
}

// sortedMounts is a deterministic order for a diagnosis line.
func sortedMounts(mounts []string) []string {
	out := append([]string(nil), mounts...)
	sort.Strings(out)
	return out
}

// guardNote is unused today; sortedMounts stays so a mount diagnosis keeps one
// order wherever it is printed.
var _ = sortedMounts
