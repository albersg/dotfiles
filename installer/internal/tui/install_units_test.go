package tui

import "testing"

// cataloguedIDs returns the catalogue's IDs as a set, so the guards below can ask
// the one question they all ask: is this ID named here or not.
func cataloguedIDs() map[string]bool {
	ids := map[string]bool{}
	for _, unit := range installUnits {
		ids[unit.ID] = true
	}
	return ids
}

// TestEveryInstallUnitHasAnExecutor closes one direction of the agreement between
// the catalogue in install_units.go and the executor table in installer.go. A unit
// the catalogue names and the executor cannot run is a unit a selection screen
// would offer, a plan would schedule, and a run would report as done having done
// nothing -- the defect this path has produced once per release.
func TestEveryInstallUnitHasAnExecutor(t *testing.T) {
	for _, unit := range installUnits {
		if _, ok := stepExecutors[unit.ID]; !ok {
			t.Errorf("the catalogue names %q, but the executor table has no entry for it, so a run that scheduled it would report it done having done nothing", unit.ID)
		}
	}
}

// TestEveryExecutorEntryIsCatalogued closes the other direction. An executor entry
// the catalogue does not name is a unit no screen can show and no switch can turn
// off: it runs on every machine the planner schedules it for, and the user has no
// way to see it, let alone choose it. That is the shape this front exists to
// remove, so it fails here rather than being inherited silently.
func TestEveryExecutorEntryIsCatalogued(t *testing.T) {
	catalogued := cataloguedIDs()
	for id := range stepExecutors {
		if !catalogued[id] {
			t.Errorf("the executor table runs %q, but the catalogue does not name it, so no screen can show it and no switch can turn it off", id)
		}
	}
}

// TestEveryScheduledUnitIsCatalogued holds the catalogue against both schedulers,
// derived the same way the scheduled-steps-versus-executor guards derive their
// sets: across every host and every set of choices the installer supports. A unit
// either planner can put on a run and the catalogue does not name is a unit the
// selection screen would have to guess at.
//
// The opposite direction -- a catalogued unit no configuration can reach -- is
// already closed transitively: TestEveryExecutorEntryIsCatalogued makes the
// catalogue a superset of the executor table, and
// TestEveryExecutorEntryIsAScheduledStep makes that table a subset of what the
// schedulers produce. It is stated here rather than left implicit because the
// guard that would say it is the one whose message a reader needs.
func TestEveryScheduledUnitIsCatalogued(t *testing.T) {
	catalogued := cataloguedIDs()
	for id := range scheduledStepIDs(t) {
		if !catalogued[id] {
			t.Errorf("buildStepsForChoices or SetupInstallSteps schedules %q, but the catalogue does not name it", id)
		}
	}
}

// TestOnlyTheThreeUnitsTheRunNeedsAreFixed pins S3: the run cannot do without
// clone, cleanup and setshell, and every other unit is a switch the user owns.
// Both directions are checked, because each one is a different mistake -- marking
// a fourth unit fixed takes a choice away from the user, and unmarking one of
// these three offers a choice the run cannot honour.
func TestOnlyTheThreeUnitsTheRunNeedsAreFixed(t *testing.T) {
	needed := map[string]bool{"clone": true, "cleanup": true, "setshell": true}

	for _, unit := range installUnits {
		if unit.Fixed && !needed[unit.ID] {
			t.Errorf("the catalogue marks %q as one the run cannot do without, but the user may turn it off: clone, cleanup and setshell are the three the run itself needs", unit.ID)
		}
		if needed[unit.ID] && !unit.Fixed {
			t.Errorf("the catalogue lets %q be turned off, but the run needs it: it is one of the three the run cannot do without", unit.ID)
		}
	}
}

// TestTheCatalogueNamesNoUnitTwice keeps the catalogue's order meaningful. The
// table is the order a run puts the units in, so naming one twice would mean the
// unit has two positions and the builder reading the table would have to decide
// which of them wins -- a second rule in a file whose whole purpose is to be the
// only one.
func TestTheCatalogueNamesNoUnitTwice(t *testing.T) {
	seen := map[string]bool{}
	for _, unit := range installUnits {
		if seen[unit.ID] {
			t.Errorf("the catalogue names %q twice, so its position in the run is two positions", unit.ID)
		}
		seen[unit.ID] = true
	}
}
