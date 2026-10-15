package tui

import (
	"fmt"
	"strings"
)

// pluginSet is one plugin set this installer provisions itself, named once so
// that the registry, the interface and the command line cannot disagree about
// which sets exist, what they are called, or which tool they belong to.
//
// A set is deliberately not a unit of install_units.go. A unit is a step the plan
// schedules and the executor dispatches; a set is work the step of a tool already
// does. That is why a set is carried by that step instead of being scheduled on
// its own: a second scheduling path for work the tool's executor already performs
// is the defect class the catalogue guard exists to prevent. Adding a set adds a
// switch beside its tool and moves no step, so the plan's identifiers and the
// plan's names stay what they were.
type pluginSet struct {
	// ID is the set's name in the interface, in --skip, and in the flag help. It is
	// prefixed so that a set ID can never be mistaken for a step ID.
	ID string

	// Step is the step of the tool the set belongs to, and Tool is the choice that
	// selects it. A set takes part in a run only when the run installs its tool,
	// which is why both are here rather than in the code that reads them.
	Step string
	Tool string

	// Named is what the interface calls the set beside its tool. A set whose
	// contents are a table this installer already owns leaves this empty and is
	// named by what it declares instead, so the label cannot drift from the table.
	Named string

	// Declared is the plugin names the set installs, when the set is such a table.
	Declared []string

	// Provisioned says this installer installs the set itself. It is false for a set
	// the tool bootstraps from the configuration this installer ships -- fish's
	// fisher, Neovim's lazy.nvim -- because a switch that installs nothing when it
	// is turned off would be a lie; those sets are named on screen with where they
	// come from instead.
	Provisioned bool
}

// pluginSetIDTmux and pluginSetIDHerdr name the sets the executor honours, so the
// registry and the steps that install them cannot drift apart over the string.
const (
	pluginSetIDTmux  = "plugins-tmux"
	pluginSetIDHerdr = "plugins-herdr"
)

// Label is the set's name as the interface shows it: what it is called, and what
// it installs when the installer declares that itself.
func (s pluginSet) Label() string {
	name := s.Named
	if name == "" {
		name = "its plugin set"
	}
	switch len(s.Declared) {
	case 0:
		return name
	case 1:
		return name + ": " + s.Declared[0]
	}
	return name + ": " + strings.Join(s.Declared[:len(s.Declared)-1], ", ") + " and " + s.Declared[len(s.Declared)-1]
}

// pluginSets is every plugin set the interface can name, in the order it names
// them, and it grows one set per commit: a set this installer installs arrives
// with the switch that turns it off, and a set a tool bootstraps for itself is
// registered with Provisioned false when the screen that explains it arrives.
var pluginSets = []pluginSet{
	{
		ID:          pluginSetIDTmux,
		Step:        "wm",
		Tool:        "tmux",
		Named:       "the plugins its configuration declares",
		Provisioned: true,
	},
	{
		ID:          pluginSetIDHerdr,
		Step:        "wm",
		Tool:        "herdr",
		Named:       "its declared plugins",
		Declared:    herdrPluginNames(),
		Provisioned: true,
	},
}

// herdrPluginNames is the names of the set the Herdr step installs, read from the
// table that step installs from, so the label the interface shows cannot drift
// from the set the machine gets.
func herdrPluginNames() []string {
	names := make([]string, 0, len(herdrPluginSet))
	for _, plugin := range herdrPluginSet {
		names = append(names, plugin.Name)
	}
	return names
}

// pluginSetByID returns the registered set with this ID.
func pluginSetByID(id string) (pluginSet, bool) {
	for _, set := range pluginSets {
		if set.ID == id {
			return set, true
		}
	}
	return pluginSet{}, false
}

// SwitchablePluginSetIDs names the plugin sets a run can turn off, in registry
// order. It is what the command line offers and validates against, so a set added
// to the registry is offered without anyone widening a written-out copy.
func SwitchablePluginSetIDs() []string {
	ids := make([]string, 0, len(pluginSets))
	for _, set := range pluginSets {
		if set.Provisioned {
			ids = append(ids, set.ID)
		}
	}
	return ids
}

// pluginSetIsSwitchable reports whether a run can turn a set off. A set is
// switchable exactly when this installer installs it itself: for a set a tool
// bootstraps from the configuration this repository ships, there is nothing a run
// could leave out, and offering it would promise a switch that does nothing.
func pluginSetIsSwitchable(id string) bool {
	set, ok := pluginSetByID(id)
	return ok && set.Provisioned
}

// SwitchableSkipIDs names everything --skip accepts: the units a run can leave
// out, in catalogue order, then the plugin sets it can turn off, in registry
// order.
func SwitchableSkipIDs() []string {
	ids := SwitchableUnitIDs()
	return append(ids, SwitchablePluginSetIDs()...)
}

// PluginSetState is the plugin set a step provisions, as the interface reads it:
// which set it is, what it is called, and whether this run turned it off.
//
// A set that was turned off is still carried, with Off set: the row has to say
// what the run is leaving out rather than disappear, which is the whole reason
// the set is on the step instead of in the step.
type PluginSetState struct {
	ID   string
	Name string
	Off  bool
}

// planPluginSet returns the set this step provisions for these choices, or nil
// when it provisions none. It is the single place the plan learns which tool
// carries which set, so the screen that offers the switch and the executor that
// honours it cannot disagree.
func planPluginSet(stepID string, opts planOptions) *PluginSetState {
	for _, set := range pluginSets {
		if set.Step != stepID || !set.Provisioned {
			continue
		}
		if !pluginSetToolChosen(set, opts) {
			continue
		}
		return &PluginSetState{
			ID:   set.ID,
			Name: set.Label(),
			Off:  pluginSetIsOff(opts.DisabledPluginSets, set.ID),
		}
	}
	return nil
}

// pluginSetToolChosen reports whether this run installs the tool a set belongs to.
func pluginSetToolChosen(set pluginSet, opts planOptions) bool {
	switch set.Step {
	case "shell":
		return opts.Shell == set.Tool
	case "wm":
		return opts.WindowMgr == set.Tool
	}
	return false
}

// pluginSetIsOff reports whether the user turned this set off. The empty list is
// the default and means every set is installed -- the same direction the unit list
// reads in, so a set the registry later gains is on until someone turns it off
// rather than off by accident.
func pluginSetIsOff(disabled []string, id string) bool {
	for _, name := range disabled {
		if name == id {
			return true
		}
	}
	return false
}

// ParseSkipSelection reads the comma-separated list --skip carries and returns
// what it names: the units to leave out, in catalogue order, and the plugin sets
// to turn off, in registry order.
//
// One flag names both because to a user they are one question -- what should this
// run not do -- and a second flag would be a second vocabulary for the same
// answer. Everything the flag refuses is refused with a message saying what it
// could have asked for instead; a name that was quietly dropped would install
// something the user believes they left out.
func ParseSkipSelection(list string) (units, sets []string, err error) {
	wanted := map[string]bool{}
	unitTokens := make([]string, 0, 4)
	for _, token := range strings.Split(list, ",") {
		name := strings.TrimSpace(token)
		if name == "" {
			continue
		}
		if set, ok := pluginSetByID(name); ok {
			if !set.Provisioned {
				return nil, nil, fmt.Errorf("%q is a plugin set this installer does not install: %s installs it from the configuration this repository ships, so there is nothing to leave out; this run can skip: %s",
					name, set.Tool, strings.Join(SwitchableSkipIDs(), ", "))
			}
			wanted[name] = true
			continue
		}
		// A name in the sets' own namespace is reported as a set rather than as an
		// unknown unit: "plugins-fish is not a unit this run installs" would send the
		// user looking in the wrong list.
		if strings.HasPrefix(name, "plugins-") {
			return nil, nil, fmt.Errorf("%q is not a plugin set this installer installs; this run can skip: %s",
				name, strings.Join(SwitchableSkipIDs(), ", "))
		}
		unitTokens = append(unitTokens, name)
	}

	if len(wanted) > 0 {
		sets = make([]string, 0, len(wanted))
		for _, set := range pluginSets {
			if wanted[set.ID] {
				sets = append(sets, set.ID)
			}
		}
	}

	units, err = ParseUnitSelection(strings.Join(unitTokens, ","))
	if err != nil {
		return nil, nil, err
	}
	return units, sets, nil
}
