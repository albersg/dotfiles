package diskclean

import (
	"strings"
	"time"
)

// The plan is the dry run. It is built from the items the user selected, it is
// what the confirmation screen shows, and building it runs nothing: every
// removal path is put through the guard here, so a path that cannot be removed
// is refused before the user is asked to confirm anything.
//
// The plan also carries, per item, the exact command that would run and the
// exact paths that would be unlinked, which is what makes the preview a
// statement about the run rather than a summary of it.

// Plan is the preview of one cleanup.
type Plan struct {
	// Items are the selected findings, in the order they were selected.
	Items []PlannedItem
	// Expected is the union of the reclaimable bytes of the items that the
	// plan would actually run. A blocked item contributes nothing, because the
	// run will not free it.
	Expected int64
	// ApprovedRoots are the scopes the guard checked the removals against, in
	// the order they were derived. They are shown so the user can see the scope
	// the plan was actually held to.
	ApprovedRoots []string
	// BuiltAt is when the plan was checked against the filesystem.
	BuiltAt time.Time
}

// PlannedItem is one item of a plan.
type PlannedItem struct {
	Item Item
	// Action is the exact command a tool mechanism would run. It is empty for
	// a removal, whose action is the paths the item covers.
	Action string
	// RequiresRoot marks a command that will be attempted with sudo -n.
	RequiresRoot bool
	// Approvals are the guard's verdicts for a removal, one per path, in the
	// order of Item.Paths.
	Approvals []Approval
	// Blocked is why the plan will not run this item at all. An item the user
	// selected that cannot run is shown with its reason rather than silently
	// dropped.
	Blocked string
}

// Removes reports whether this item is cleaned by removing its paths.
func (p PlannedItem) Removes() bool { return p.Item.Mechanism.Kind == MechanismRemove }

// Runs reports whether the plan would attempt this item.
func (p PlannedItem) Runs() bool { return p.Blocked == "" }

// BuildPlan turns a selection into the previewed plan. guard is the scope the
// removals are checked against; GuardFor builds the one a selection implies.
func BuildPlan(items []Item, guard Guard) Plan {
	plan := Plan{BuiltAt: time.Now(), ApprovedRoots: guard.ApprovedRoots()}
	for _, item := range items {
		planned := PlannedItem{Item: item}
		switch {
		case !item.Cleanable():
			planned.Blocked = "This finding is reported, not cleaned: " + item.Mechanism.Reason
		case item.Mechanism.Kind == MechanismTool:
			planned.Action = item.Mechanism.Command()
			planned.RequiresRoot = item.Mechanism.RequiresRoot
		case item.Mechanism.Kind == MechanismRemove:
			planned.Approvals, planned.Blocked = approvePaths(guard, item.Paths)
		}
		if planned.Runs() {
			plan.Expected += item.Reclaimable()
		}
		plan.Items = append(plan.Items, planned)
	}
	return plan
}

// approvePaths puts every path of an item through the guard. The first refusal
// blocks the whole item: a removal that can only do half of what it promised
// is not a removal the user approved.
func approvePaths(guard Guard, paths []string) ([]Approval, string) {
	approvals := make([]Approval, 0, len(paths))
	for _, path := range paths {
		approval, err := guard.Check(path)
		if err != nil {
			return nil, err.Error()
		}
		approvals = append(approvals, approval)
	}
	return approvals, ""
}

// GuardFor builds the guard a selection implies: one approved root per item --
// its declared Roots when it has them, its own paths otherwise. Nothing else
// is approved, so a path a detector never offered is not removable even if the
// user selected an item whose row mentioned it.
func GuardFor(probe Probe, items []Item) Guard {
	var roots []string
	for _, item := range items {
		if len(item.Roots) > 0 {
			roots = append(roots, item.Roots...)
			continue
		}
		roots = append(roots, item.Paths...)
	}
	return NewGuard(probe, roots)
}

// Summary renders the plan as the short lines a preview screen shows: what
// would run and what it would free, per item, with a blocked item naming its
// reason instead of its action.
func (p Plan) Summary() []string {
	var lines []string
	for _, planned := range p.Items {
		switch {
		case !planned.Runs():
			lines = append(lines, planned.Item.Title+": not run -- "+planned.Blocked)
		case planned.Removes():
			lines = append(lines, planned.Item.Title+": remove "+strings.Join(planned.Item.Paths, ", "))
		default:
			lines = append(lines, planned.Item.Title+": "+planned.Action)
		}
	}
	return lines
}
