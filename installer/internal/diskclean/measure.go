package diskclean

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"
)

// Limits bound a scan so that a store larger than anyone expected cannot run
// without end. Every ceiling is declared to the user when it is hit: a walk
// that stopped early reports a lower bound rather than a quiet wrong number.
//
// The zero value of each field means "the default"; a negative value disables
// that particular ceiling.
type Limits struct {
	// MaxFiles is the ceiling on files counted across the whole scan.
	MaxFiles int
	// MaxDuration bounds the whole scan.
	MaxDuration time.Duration
	// MaxReasons is how many per-path failure reasons one item keeps. The count
	// of all of them is kept regardless; only the list is cut.
	MaxReasons int
}

// DefaultLimits are the ceilings a real scan runs under: large enough for a
// machine that has lived through years of builds, small enough that a scan
// someone cancelled stops promptly.
func DefaultLimits() Limits {
	return Limits{
		MaxFiles:    1_000_000,
		MaxDuration: 2 * time.Minute,
		MaxReasons:  24,
	}
}

// normalizeLimits fills every unset ceiling with its default. A negative field
// is left alone: it is how a caller asks for that one ceiling to be off.
func normalizeLimits(limits Limits) Limits {
	defaults := DefaultLimits()
	if limits.MaxFiles == 0 {
		limits.MaxFiles = defaults.MaxFiles
	}
	if limits.MaxDuration == 0 {
		limits.MaxDuration = defaults.MaxDuration
	}
	if limits.MaxReasons == 0 {
		limits.MaxReasons = defaults.MaxReasons
	}
	return limits
}

// Progress is one tick of a running scan, so a screen can say what is being
// walked instead of showing a still frame.
type Progress struct {
	// ItemID and Path name what is being measured right now.
	ItemID string
	Path   string
	// Files and Bytes are the totals so far.
	Files int
	Bytes int64
}

// MeasureOptions shape one measurement pass.
type MeasureOptions struct {
	Limits Limits
	// Progress, when set, is called from the measuring goroutine. It must not
	// touch the model: a screen adapts it into a message.
	Progress func(Progress)
}

// Dedupe is the bookkeeping of one scan. It remembers which item counted each
// file, by device and inode, so a file reachable twice -- a hardlink shared by
// two stores, a store nested inside another item's path, the same directory
// named by two detectors -- is counted once and attributed to the item that
// reached it first. Without it, overlapping stores would each report the whole
// shared tree and the total would be a fiction.
//
// The zero value is not usable; NewDedupe returns an empty one.
type Dedupe struct {
	seen map[fileID]string
}

// NewDedupe returns an empty dedupe for one scan.
func NewDedupe() *Dedupe {
	return &Dedupe{seen: make(map[fileID]string)}
}

// claim records a file under an owner. It returns the owner that already held
// it, if any, so the caller can attribute the bytes elsewhere.
func (d *Dedupe) claim(id fileID, owner string) (string, bool) {
	if d == nil || !id.valid() {
		return "", false
	}
	if holder, ok := d.seen[id]; ok {
		return holder, true
	}
	d.seen[id] = owner
	return "", false
}

// WalkResult is what one path of one item contributed.
type WalkResult struct {
	Logical  int64
	Physical int64
	Files    int
	// Shared is how many files were already counted by another item, and
	// SharedWith holds those items' IDs.
	Shared      int
	SharedBytes int64
	SharedWith  map[string]bool
	// Mounts names subdirectories that live on a different filesystem than the
	// root being walked. The walk stops at them rather than counting another
	// device's bytes as this store's.
	Mounts []string
	// Symlinks counts links met inside the tree. They are counted and not
	// followed, because following one would let a link out of the approved
	// store decide the number.
	Symlinks int
	// Reasons are the paths that could not be read, with why. ReasonsTotal is
	// how many there were before the list was cut to the ceiling.
	Reasons      []string
	ReasonsTotal int
	// Truncated reports that a ceiling stopped the walk, so every number here
	// is a lower bound.
	Truncated bool
	// Cancelled reports that the context ended the walk.
	Cancelled bool
	// Err is why the path could not be measured at all, when it could not.
	Err string
}

// Walk measures one tree: what it holds, in logical bytes and in what the
// filesystem allocated, without ever following a symlink and without ever
// descending into another filesystem.
//
// It never fails as a whole because of one unreadable entry. A permission
// failure is a reason recorded against the path it happened at, and the walk
// carries on, because a cache with one unreadable corner still has a size.
func Walk(ctx context.Context, probe Probe, root string, owner string, dedupe *Dedupe, limits Limits) WalkResult {
	var result WalkResult

	info, err := probe.Lstat(root)
	if err != nil {
		result.Err = fmt.Sprintf("%s could not be read: %v", root, err)
		return result
	}
	if info.Mode()&os.ModeSymlink != 0 {
		// The path itself is a link. Refusing here is what keeps a store that
		// was symlinked elsewhere from being measured -- or later removed --
		// through the link's name.
		result.Err = fmt.Sprintf("%s is a symbolic link, and a link is never followed: the store it points at is not measured through it", root)
		return result
	}
	if !info.IsDir() {
		result.Err = fmt.Sprintf("%s is not a directory, so it is not a store this walk can measure", root)
		return result
	}

	rootDevice := deviceOf(info)
	deadline := time.Time{}
	if limits.MaxDuration > 0 {
		deadline = probe.Now().Add(limits.MaxDuration)
	}
	// An explicit stack rather than recursion: a store can be deeper than the
	// call stack should be asked to go, and every level is already bounded by
	// the file ceiling.
	stack := []string{root}
	files := 0
	for len(stack) > 0 {
		if err := ctx.Err(); err != nil {
			result.Cancelled = true
			break
		}
		if !deadline.IsZero() && probe.Now().After(deadline) {
			result.Truncated = true
			break
		}
		if limits.MaxFiles > 0 && files >= limits.MaxFiles {
			result.Truncated = true
			break
		}

		dir := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		entries, err := probe.ReadDir(dir)
		if err != nil {
			result.addReason(fmt.Sprintf("%s could not be listed: %v", dir, err), limits)
			continue
		}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				result.Cancelled = true
				break
			}
			childInfo, infoErr := entry.Info()
			if infoErr != nil {
				result.addReason(fmt.Sprintf("%s could not be read: %v", entryPath(dir, entry.Name()), infoErr), limits)
				continue
			}
			switch {
			case childInfo.Mode()&os.ModeSymlink != 0:
				// Counted and not followed. A link is not the store's bytes.
				result.Symlinks++
				continue
			case childInfo.IsDir():
				if device := deviceOf(childInfo); device != 0 && rootDevice != 0 && device != rootDevice {
					// A different filesystem inside the tree is a mount: the
					// bytes behind it belong to that filesystem, not to this
					// store, and a walk that crossed it would both overstate
					// the store and later offer to delete another device's
					// data.
					if len(result.Mounts) < 8 {
						result.Mounts = append(result.Mounts, entryPath(dir, entry.Name()))
					}
					continue
				}
				stack = append(stack, entryPath(dir, entry.Name()))
			case childInfo.Mode().IsRegular():
				if limits.MaxFiles > 0 && files >= limits.MaxFiles {
					result.Truncated = true
					break
				}
				files++
				if holder, shared := dedupe.claim(probe.identity(entryPath(dir, entry.Name()), childInfo), owner); shared {
					result.Shared++
					result.SharedBytes += childInfo.Size()
					if result.SharedWith == nil {
						result.SharedWith = make(map[string]bool)
					}
					result.SharedWith[holder] = true
					continue
				}
				result.Logical += childInfo.Size()
				result.Physical += physicalSize(childInfo)
				result.Files++
			}
		}
		if result.Cancelled {
			break
		}
		if limits.MaxFiles > 0 && files >= limits.MaxFiles {
			result.Truncated = true
			break
		}
	}
	return result
}

// addReason records one path that could not be read, keeping the list bounded
// while still counting every one of them.
func (r *WalkResult) addReason(reason string, limits Limits) {
	r.ReasonsTotal++
	if limits.MaxReasons <= 0 || len(r.Reasons) < limits.MaxReasons {
		r.Reasons = append(r.Reasons, reason)
	}
}

// entryPath joins a directory and an entry name. It is filepath.Join's job, but
// it is spelled here so the intent -- a path built from a listing, never from
// user input -- is visible at the call site.
func entryPath(dir, name string) string {
	if dir == "" {
		return name
	}
	return dir + string(os.PathSeparator) + name
}

// Measure walks every path of every item once, sharing one dedupe, and returns
// the items with their sizes and their overlaps filled in.
//
// The order of the items decides who owns a shared file: the first item that
// reaches it counts it, and every later one reports it as shared rather than
// adding it again. That is the only rule that makes the totals honest without
// either merging two stores into one row or presenting the same bytes twice.
func Measure(ctx context.Context, probe Probe, items []Item, opts MeasureOptions) ([]Item, WalkResult) {
	limits := normalizeLimits(opts.Limits)

	dedupe := NewDedupe()
	// The scan-wide total is what the caller reports as "the scan was cut
	// short" and what the progress line counts.
	var scan WalkResult
	measured := make([]Item, 0, len(items))
	reasons := 0

	for _, item := range items {
		if !item.Cleanable() || len(item.Paths) == 0 {
			// A container item is measured by its owner's own accounting, and a
			// report-only finding by nothing: neither has paths to walk.
			measured = append(measured, item)
			continue
		}

		size := Size{Source: SourceWalk}
		missing := true
		for _, path := range item.Paths {
			if err := ctx.Err(); err != nil {
				scan.Cancelled = true
				break
			}
			walk := Walk(ctx, probe, path, item.ID, dedupe, limits)
			reasons += walk.ReasonsTotal
			scan.Logical += walk.Logical
			scan.Physical += walk.Physical
			scan.Files += walk.Files
			if walk.Cancelled {
				scan.Cancelled = true
			}
			if walk.Truncated {
				scan.Truncated = true
				size.Truncated = true
			}
			for _, owner := range sortedKeys(walk.SharedWith) {
				if owner != item.ID {
					size.SharedWith = appendUnique(size.SharedWith, owner)
				}
			}
			size.Shared += walk.Shared
			size.SharedBytes += walk.SharedBytes
			size.Logical += walk.Logical
			size.Physical += walk.Physical
			size.Files += walk.Files

			// A path that is simply not there is not an error: the tool is
			// installed and its store is empty or has never been made. Any other
			// failure is kept as the item's reason, because a size nobody could
			// measure must not be presented as a zero someone measured.
			if _, statErr := probe.Lstat(path); statErr == nil {
				missing = false
			} else if !os.IsNotExist(statErr) && size.Err == "" && walk.Err != "" {
				size.Err = walk.Err
			}
			if opts.Progress != nil {
				opts.Progress(Progress{ItemID: item.ID, Path: path, Files: size.Files, Bytes: size.Logical})
			}
		}

		if missing && size.Files == 0 && size.Logical == 0 {
			size.Missing = true
		}
		item.Size = size
		item.Overlaps = appendUnique(item.Overlaps, size.SharedWith...)
		measured = append(measured, item)
	}

	// Overlap is symmetric: an item whose bytes another item already counted
	// names that item, and the item that owns the bytes names the one that
	// reported them as shared.
	index := make(map[string]int, len(measured))
	for i, item := range measured {
		index[item.ID] = i
	}
	for i := range measured {
		for _, other := range measured[i].Overlaps {
			if j, ok := index[other]; ok && j != i {
				measured[j].Overlaps = appendUnique(measured[j].Overlaps, measured[i].ID)
			}
		}
		for j := range measured {
			if j == i {
				continue
			}
			if pathsOverlap(measured[i].Paths, measured[j].Paths) {
				measured[i].Overlaps = appendUnique(measured[i].Overlaps, measured[j].ID)
			}
		}
	}
	for i := range measured {
		sort.Strings(measured[i].Overlaps)
	}

	if reasons > 0 {
		scan.ReasonsTotal = reasons
	}
	return measured, scan
}

// pathsOverlap reports whether one path is an ancestor of the other. It is what
// catches a container path that fully encloses a smaller item, which the walk's
// shared-file accounting also sees but a preview should know before it runs.
func pathsOverlap(left, right []string) bool {
	for _, a := range left {
		for _, b := range right {
			if a == b || isUnder(a, b) || isUnder(b, a) {
				return true
			}
		}
	}
	return false
}

// isUnder reports whether path is strictly inside root.
func isUnder(root, path string) bool {
	if root == "" || path == "" || root == path {
		return false
	}
	trimmed := root
	for len(trimmed) > 1 && trimmed[len(trimmed)-1] == os.PathSeparator {
		trimmed = trimmed[:len(trimmed)-1]
	}
	return len(path) > len(trimmed) &&
		path[:len(trimmed)] == trimmed &&
		path[len(trimmed)] == os.PathSeparator
}

// appendUnique adds value to list when it is not already there.
func appendUnique(list []string, values ...string) []string {
	for _, value := range values {
		if value == "" {
			continue
		}
		found := false
		for _, existing := range list {
			if existing == value {
				found = true
				break
			}
		}
		if !found {
			list = append(list, value)
		}
	}
	return list
}

// sortedKeys returns a set's keys in order, so a value built from a map is
// deterministic.
func sortedKeys(set map[string]bool) []string {
	if len(set) == 0 {
		return nil
	}
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
