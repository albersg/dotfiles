package tui

// installUnit is one unit the installer can schedule, named once so that the
// catalogue of units exists in a single place.
//
// The two tables answer different questions and are held to each other by
// install_units_test.go rather than by review. The executor table
// (stepExecutors, installer.go) says how each unit runs; this one says what the
// units are, the order a run puts them in, and which of them the run cannot do
// without. A unit added to one and not the other fails the suite instead of
// surfacing as a step that reports itself done having done nothing, which is the
// defect class this path has produced once per release.
type installUnit struct {
	// ID is the step ID: the plan schedules it, the executor dispatches on it,
	// and this catalogue names it. It is the same string in all three places on
	// purpose, and the guards in install_units_test.go are what keep it so.
	ID string

	// Fixed marks the units the run itself needs, and it is what the selection
	// screen refuses to turn off (S3 of odd/tasks/installer-customization.md):
	// clone is the repository the run works from, cleanup is the state it
	// created, and setshell is the machine's default shell left as chosen. Every
	// other unit is a switch the user owns.
	//
	// Widening this set is a product decision, not a refactor: each one added
	// takes a choice away from the user, and a guard fails until this comment and
	// that decision move together.
	Fixed bool
}

// installUnits is every unit the run can schedule, in the order a run schedules
// them.
//
// deps and xcode share a position because they are alternatives rather than a
// sequence: the dependency step asks the distribution whether it wants the
// distribution's own packages (Linux, Termux) or the Apple developer tools
// (macOS without them), and a run plans one or neither, as planFor has it. Their
// relative order here is the one the interactive planner gives them.
//
// The order a run actually puts them in is this table's, and the builder that
// reads it is what makes that true (T2 of the front). Nothing here is a
// presentation string: the run's step names and descriptions are the planner's,
// because several of them are built from the choices ("Install " + the terminal
// the user picked) and a second set of names here would be a second answer that
// could drift.
var installUnits = []installUnit{
	{ID: "backup"},
	{ID: "deps"},
	{ID: "xcode"},
	{ID: "clone", Fixed: true},
	{ID: "homebrew"},
	{ID: "terminal"},
	{ID: "font"},
	{ID: "shell"},
	{ID: "wm"},
	{ID: "nvim"},
	{ID: "toolset"},
	{ID: "agentskills"},
	{ID: "officecli"},
	{ID: "wslconfig"},
	{ID: "theme"},
	{ID: "setshell", Fixed: true},
	{ID: "cleanup", Fixed: true},
}
