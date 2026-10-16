package diskclean

import (
	"context"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// Category says what kind of thing a finding is. The distinction is the one the
// screen has to make visible: a regenerable cache costs a download to rebuild,
// while project dependencies and persistent volumes are data the user put
// there on purpose and may not be reproducible at all.
type Category string

const (
	CategoryCache        Category = "cache"
	CategoryDownloads    Category = "downloads"
	CategoryBuild        Category = "build"
	CategoryDependencies Category = "dependencies"
	CategoryImages       Category = "images"
	CategoryVolumes      Category = "volumes"
)

// Label is the category as a row says it: a short noun phrase, because the
// rows already carry the tool, the path and the numbers.
func (c Category) Label() string {
	switch c {
	case CategoryCache:
		return "regenerable cache"
	case CategoryDownloads:
		return "downloads"
	case CategoryBuild:
		return "build artifacts"
	case CategoryDependencies:
		return "project dependencies"
	case CategoryImages:
		return "images and build cache"
	case CategoryVolumes:
		return "persistent volumes"
	default:
		return string(c)
	}
}

// Risk grades what removing a finding costs. It is not a colour: it decides
// which items a bulk selection may touch and which ones the preview has to
// spell out.
type Risk int

const (
	// RiskSafe is a store that exists only to be rebuilt: its owner's own
	// command finds it again on the next use, and no user data lives in it.
	RiskSafe Risk = iota
	// RiskCareful is a store whose loss costs real time or a re-download, or
	// whose owner may be running while it is removed.
	RiskCareful
	// RiskDelicate is data that may not be reproducible at all: persistent
	// volumes, and the container's own stores.
	RiskDelicate
)

// Label is the risk as the screen prints it.
func (r Risk) Label() string {
	switch r {
	case RiskSafe:
		return "safe to rebuild"
	case RiskCareful:
		return "costs a re-download or a rebuild"
	case RiskDelicate:
		return "delicate"
	default:
		return "unknown"
	}
}

// Delicate reports whether the item must never come along with a bulk
// selection. A delicate item is still selectable -- the user may know better
// than the label -- but only one at a time, by hand, and the preview says so.
func (r Risk) Delicate() bool { return r == RiskDelicate }

// MechanismKind says how an item may be cleaned, if it may be cleaned here.
type MechanismKind int

const (
	// MechanismTool is the tool's own maintenance command: the only way this
	// utility ever touches a store its owner manages. Container and package
	// stores are never removed from the filesystem.
	MechanismTool MechanismKind = iota
	// MechanismRemove is a guarded removal of the item's own paths. It is used
	// only where the owning tool offers no command that would reclaim the
	// store, and every removal passes the guard in guard.go.
	MechanismRemove
	// MechanismReportOnly is a finding this utility measures and refuses to
	// clean, with Reason saying why. A volume whose owner is unknown, or a
	// store behind a tool that is not installed, is reported rather than
	// silently left out.
	MechanismReportOnly
)

// Mechanism is how one item may be cleaned. Program and Args are kept apart
// from any shell on purpose: the package builds an argument slice and hands it
// to exec.CommandContext. There is no string interpolation anywhere.
type Mechanism struct {
	Kind MechanismKind
	// Program is the executable of the owner's own command.
	Program string
	// Args are the arguments, already split.
	Args []string
	// RequiresRoot marks a command whose owner needs administrator rights
	// (the system package managers). It is attempted with sudo -n and, when
	// that is refused, reported with the exact command instead of prompting.
	RequiresRoot bool
	// Reason explains a report-only mechanism.
	Reason string
}

// Command is the mechanism as one line for the preview and the rows. It is a
// rendering of Program and Args, never a string this package would run.
func (m Mechanism) Command() string {
	if m.Program == "" {
		return ""
	}
	parts := make([]string, 0, len(m.Args)+1)
	parts = append(parts, m.Program)
	for _, arg := range m.Args {
		if strings.ContainsAny(arg, " \t\"'") {
			parts = append(parts, "'"+strings.ReplaceAll(arg, "'", "'\\''")+"'")
			continue
		}
		parts = append(parts, arg)
	}
	line := strings.Join(parts, " ")
	if m.RequiresRoot {
		return "sudo " + line
	}
	return line
}

// Size is what a store holds, measured one way and recorded with its source so
// no screen can present an estimate as a measurement.
type Size struct {
	// Logical is the sum of the files' own byte counts, the number a person
	// sees in a file listing.
	Logical int64
	// Physical is the space the filesystem actually allocated (blocks times
	// the block size). This is what freeing the store returns to the disk, and
	// it is the number the screen reports as reclaimable.
	Physical int64
	// Files is how many files the walk counted.
	Files int
	// Shared is the number of files -- and SharedBytes the space -- already
	// counted under another item of the same scan. They are kept apart rather
	// than added, which is what keeps an overlapping store from being reported
	// twice.
	Shared      int
	SharedBytes int64
	// SharedWith names the items that already own those bytes.
	SharedWith []string
	// Truncated marks a walk that hit its file ceiling, so the number is a
	// lower bound and says so.
	Truncated bool
	// Missing marks a path that is not there: the detector found evidence of
	// the tool but its store is empty or was never created.
	Missing bool
	// Err is why the size could not be measured. It is a reason, never a
	// substituted zero.
	Err string
	// Source is where the number came from.
	Source Source
}

// Reclaimable is the space the screen offers to free for this item: the
// physical allocation, because that is what the filesystem returns, and the
// owner's own reported reclaimable figure for a tool-sourced size.
func (s Size) Reclaimable() int64 { return s.Physical }

// Source names where a Size came from, so "measured" and "reported by the tool"
// are never presented as the same kind of fact.
type Source int

const (
	// SourceWalk is a filesystem walk by this package.
	SourceWalk Source = iota
	// SourceTool is the owner's own accounting, parsed from its command.
	SourceTool
)

// Label is the source as the screen prints it.
func (s Source) Label() string {
	switch s {
	case SourceTool:
		return "reported by the tool"
	default:
		return "measured"
	}
}

// Item is one finding: one store, one row, one decision.
type Item struct {
	// ID is stable for the life of a scan and is what a selection refers to.
	ID string
	// Tool names the ecosystem or program that owns the store.
	Tool string
	// Title is the row's name: what the store is, in the tool's own words.
	Title string
	// Detail explains what the store holds and what regenerates it, so the
	// preview can say what a removal would cost.
	Detail string
	// Category and Risk classify the consequence of removing it.
	Category Category
	Risk     Risk
	// Reason is the evidence that found it: the binary on PATH, the variable
	// that named the path, or the directory that exists.
	Reason string
	// Paths are the filesystem paths the item covers. They are also what is
	// measured and, for a removal mechanism, what is removed.
	Paths []string
	// Roots are the approved scopes a removal of Paths may happen under. They
	// are what the guard is built from, and for most items that is the path
	// itself; a project artifact declares its project directory instead, so the
	// scope is the directory the user added rather than a path a detector
	// computed.
	Roots []string
	// Mechanism is how it may be cleaned.
	Mechanism Mechanism
	// Size is what it holds, filled in by Measure.
	Size Size
	// Overlaps names other items of the same scan whose paths share bytes with
	// this one. The screen shows it rather than adding the numbers.
	Overlaps []string
	// VolatileObjects are the container objects the item stands for (images,
	// containers, volumes, build cache), named so the preview can say what the
	// tool's command will remove. It is empty for a filesystem store.
	VolatileObjects []string
}

// Cleanable reports whether this item has a mechanism that can run here. A
// report-only finding is measured and explained, never selected.
func (i Item) Cleanable() bool { return i.Mechanism.Kind != MechanismReportOnly }

// Reclaimable is the space the item would free if it is cleaned now.
func (i Item) Reclaimable() int64 { return i.Size.Reclaimable() }

// Report is one scan: every finding, in the order the detectors produced, with
// the facts a screen needs to say how the numbers were taken.
type Report struct {
	Items     []Item
	ScannedAt time.Time
	Duration  time.Duration
	// Cancelled reports that the user stopped the scan; the items are the ones
	// that finished before the stop, and each measured size is complete.
	Cancelled bool
	// Err is a scan-level failure, when there is one.
	Err string
}

// Reclaimable is the total space the cleanable items of this report hold. Items
// that share bytes with another item contribute only what they own, so the sum
// is a union rather than a double count.
func (r Report) Reclaimable() int64 {
	var total int64
	for _, item := range r.Items {
		if !item.Cleanable() {
			continue
		}
		total += item.Reclaimable()
	}
	return total
}

// Selection is the set of items the user marked. It is a set of IDs rather than
// a list of indices so that a second scan -- whose items are new values in a new
// order -- cannot silently select something the user never saw.
type Selection map[string]bool

// Has reports whether the item is selected.
func (s Selection) Has(id string) bool { return s[id] }

// Count is how many of the given items are selected.
func (s Selection) Count(items []Item) int {
	n := 0
	for _, item := range items {
		if s[item.ID] {
			n++
		}
	}
	return n
}

// Selected returns the chosen items in report order.
func (s Selection) Selected(items []Item) []Item {
	var chosen []Item
	for _, item := range items {
		if s[item.ID] {
			chosen = append(chosen, item)
		}
	}
	return chosen
}

// SortBySize orders items largest first, with the tool's name breaking a tie so
// the order is stable across scans.
func SortBySize(items []Item) {
	sort.SliceStable(items, func(i, j int) bool {
		left, right := items[i].Reclaimable(), items[j].Reclaimable()
		if left != right {
			return left > right
		}
		return items[i].ID < items[j].ID
	})
}

// Probe carries every effect a scan has, so a test can replace each one: no
// guard may read the runner's own home directory, and no detector may start a
// real package manager. The zero value is not usable; DefaultProbe returns the
// real one.
type Probe struct {
	// HomeDir is the home directory the store paths are resolved from.
	HomeDir string
	// Getenv reads an environment variable. It is a function rather than
	// os.Getenv so a test can pin a machine's overrides without exporting
	// them.
	Getenv func(string) string
	// LookPath reports whether a program is on PATH.
	LookPath func(string) (string, error)
	// Lstat and Stat are os.Lstat and os.Stat; Lstat is what a walk uses so a
	// symlink is never followed.
	Lstat func(string) (os.FileInfo, error)
	Stat  func(string) (os.FileInfo, error)
	// ReadDir reads one directory level.
	ReadDir func(string) ([]os.DirEntry, error)
	// Run starts a program and returns its output. It is how a tool that
	// reports its own accounting is asked (docker system df), and how a
	// command of the plan is run.
	Run func(ctx context.Context, name string, args ...string) ([]byte, error)
	// Now is the clock, so a report's timestamp is testable.
	Now func() time.Time
}

// DefaultProbe returns the probe every real scan uses.
func DefaultProbe() Probe {
	return Probe{
		HomeDir:  os.Getenv("HOME"),
		Getenv:   os.Getenv,
		LookPath: exec.LookPath,
		Lstat:    os.Lstat,
		Stat:     os.Stat,
		ReadDir:  os.ReadDir,
		Run:      runCommand,
		Now:      time.Now,
	}
}

// runCommand starts one program with an argument slice. There is no shell and
// no interpolation: the slice handed to exec is the command that runs.
func runCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.Output()
}
