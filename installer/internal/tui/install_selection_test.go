package tui

import (
	"reflect"
	"strings"
	"testing"
)

// catalogueOrderOf is the units of a selection in the catalogue's order, taken from
// the catalogue itself: a guard that wrote the order out by hand would go stale the
// first time the table moved.
func catalogueOrderOf(ids ...string) []string {
	wanted := map[string]bool{}
	for _, id := range ids {
		wanted[id] = true
	}
	ordered := []string{}
	for _, unit := range installUnits {
		if wanted[unit.ID] {
			ordered = append(ordered, unit.ID)
		}
	}
	return ordered
}

// TestTheFlagSelectionIsReadInCatalogueOrder pins that the selection is the catalogue's
// set whatever order the string was written in, so two routes naming the same units
// cannot hand the plan two different lists.
func TestTheFlagSelectionIsReadInCatalogueOrder(t *testing.T) {
	cases := []struct {
		list string
		want []string
	}{
		{"", nil},
		{"   ", nil},
		{",,", nil},
		{"toolset", []string{"toolset"}},
		{"nvim,toolset", catalogueOrderOf("toolset", "nvim")},
		{"toolset, nvim", catalogueOrderOf("toolset", "nvim")},
		{" toolset ", []string{"toolset"}},
		{"toolset,toolset", []string{"toolset"}},
	}

	for _, tc := range cases {
		got, err := ParseUnitSelection(tc.list)
		if err != nil {
			t.Errorf("%q was refused: %v", tc.list, err)
			continue
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%q reads as %v, want %v", tc.list, got, tc.want)
		}
	}
}

// TestTheFlagRefusesWhatTheRunCannotDo is the error half. A unit the run needs, or a
// name nothing knows, is refused with the name in the message: an accepted token that
// does nothing is the failure this guard exists to prevent, because the user believes
// they asked for it.
func TestTheFlagRefusesWhatTheRunCannotDo(t *testing.T) {
	cases := []struct {
		name string
		list string
		want string
	}{
		{"a unit the run needs", "clone", "cannot leave out"},
		{"another unit the run needs", "cleanup", "cannot leave out"},
		{"the third one", "setshell", "cannot leave out"},
		{"a name nothing installs", "printer", "not a unit this run installs"},
		{"a name with a typo", "toolsett", "not a unit this run installs"},
	}

	for _, tc := range cases {
		units, err := ParseUnitSelection(tc.list)
		if err == nil {
			t.Fatalf("%q was accepted as %v, and the run cannot do that", tc.list, units)
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("refusing %q says %q, which does not say %q", tc.list, err, tc.want)
		}
		if !strings.Contains(err.Error(), tc.list) {
			t.Errorf("refusing %q does not name it: %q", tc.list, err)
		}
	}
}

// TestTheFlagOffersExactlyWhatTheScreenOffers keeps the command line and the review
// screen on one list. The screen's rule is the catalogue's Fixed flag as well, so the
// two can only disagree if one of them stops reading the table.
func TestTheFlagOffersExactlyWhatTheScreenOffers(t *testing.T) {
	offered := SwitchableUnitIDs()

	if len(offered) == 0 {
		t.Fatal("no unit is offered, so this guard proves nothing")
	}
	seen := map[string]bool{}
	for _, id := range offered {
		if seen[id] {
			t.Errorf("%q is offered twice", id)
		}
		seen[id] = true
		if installUnitIsFixed(id) {
			t.Errorf("%q is offered, but the run needs it", id)
		}
		if _, err := ParseUnitSelection(id); err != nil {
			t.Errorf("%q is offered by SwitchableUnitIDs and refused by ParseUnitSelection: %v", id, err)
		}
	}

	for _, unit := range installUnits {
		if unit.Fixed {
			continue
		}
		if !seen[unit.ID] {
			t.Errorf("the catalogue can skip %q and the command line does not offer it", unit.ID)
		}
	}
}

// TestTheSameSelectionPlansTheSameOnBothRoutes is S8's parity, and it is the reason
// the flag is worth having: the command line's selection and the review screen's reach
// the plan through the same builder, so the same units produce the same steps, and a
// unit named on either route is planned as skipped rather than left out.
func TestTheSameSelectionPlansTheSameOnBothRoutes(t *testing.T) {
	choices := UserChoices{OS: "linux", Shell: "zsh", InstallNvim: true}

	for _, list := range []string{"", "toolset", "nvim,toolset", "agentskills,officecli,theme"} {
		units, err := ParseUnitSelection(list)
		if err != nil {
			t.Fatalf("%q was refused: %v", list, err)
		}

		// The interface's route: the model the review screen leaves behind.
		fromScreen := planFor((&Model{Choices: choices, DisabledUnits: units}).planOptions())

		// The command line's route: the model RunNonInteractive builds before it runs.
		fromFlag := buildStepsForChoices(&Model{Choices: choices, DisabledUnits: units})

		if !reflect.DeepEqual(fromScreen, fromFlag) {
			t.Errorf("the selection %q plans differently on the two routes:\n screen: %v\n   flag: %v",
				list, planIDList(fromScreen), planIDList(fromFlag))
		}

		for _, id := range units {
			if got := plannedStep(t, fromFlag, id).Status; got != StatusSkipped {
				t.Errorf("%q was asked to be skipped and is planned as %v on the command line's route", id, got)
			}
		}
		if len(units) > 0 && list == "" {
			t.Errorf("an empty list planned %d skipped units", len(units))
		}
	}
}

// TestTheEnvironmentSkipsKeepTheirEffect pins S7: the two environment gates still
// answer exactly what they answered before this work, and neither of them moves the
// plan. They belong to the steps they skip, which is why a selection and an
// environment variable can name the same unit without interfering.
func TestTheEnvironmentSkipsKeepTheirEffect(t *testing.T) {
	for _, value := range []string{"1", "true", "yes", "anything"} {
		t.Setenv(envSkipDeps, value)
		if !skipDeps() {
			t.Errorf("DOTFILES_SKIP_DEPS=%q no longer skips the dependency step", value)
		}
		t.Setenv(envSkipToolset, value)
		if !skipToolset() {
			t.Errorf("DOTFILES_SKIP_TOOLSET=%q no longer skips the toolset step", value)
		}
	}
	for _, value := range []string{"", "0", "false"} {
		t.Setenv(envSkipDeps, value)
		if skipDeps() {
			t.Errorf("DOTFILES_SKIP_DEPS=%q skips the dependency step", value)
		}
		t.Setenv(envSkipToolset, value)
		if skipToolset() {
			t.Errorf("DOTFILES_SKIP_TOOLSET=%q skips the toolset step", value)
		}
	}

	m := &Model{Choices: UserChoices{OS: "linux", Shell: "zsh", InstallNvim: true}}
	t.Setenv(envSkipDeps, "1")
	t.Setenv(envSkipToolset, "1")
	withEnvironment := planFor(m.planOptions())

	t.Setenv(envSkipDeps, "")
	t.Setenv(envSkipToolset, "")
	withoutEnvironment := planFor(m.planOptions())

	if !reflect.DeepEqual(withEnvironment, withoutEnvironment) {
		t.Errorf("the environment moved the plan, and S7 says the two gates keep their effect without changing what is planned: %v against %v",
			planIDList(withEnvironment), planIDList(withoutEnvironment))
	}
}
