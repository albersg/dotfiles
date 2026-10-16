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

	// Switchable says a run can leave the set out. It is a separate question from
	// Provisioned, and the shell's set is why: this installer does install it, and a
	// run cannot leave it out, because the shell configuration this repository ships
	// is built on it -- its .zshrc declares the plugins, sources $ZSH/oh-my-zsh.sh
	// and sources ~/.p10k.zsh for the prompt -- so leaving it out would mean shipping
	// a second, unverified shell configuration rather than patching this one.
	Switchable bool

	// Because is why a set a run cannot leave out is not offered as a switch, and it
	// is what the row says instead of a switch. It is empty for a switchable set, and
	// a guard fails on a set that is neither switchable nor explained.
	Because string
}

// pluginSetIDTmux and pluginSetIDHerdr name the sets the executor honours, so the
// registry and the steps that install them cannot drift apart over the string. The
// other three are named here too, because the interface names them on screen.
const (
	pluginSetIDShell = "plugins-shell"
	pluginSetIDFish  = "plugins-fish"
	pluginSetIDTmux  = "plugins-tmux"
	pluginSetIDHerdr = "plugins-herdr"
	pluginSetIDNvim  = "plugins-nvim"
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
		ID:          pluginSetIDShell,
		Step:        "shell",
		Tool:        "zsh",
		Named:       "Oh My Zsh with Powerlevel10k and the plugin packages",
		Provisioned: true,
		Because:     "the zsh configuration this repository ships is built on it: its .zshrc declares the plugins, sources $ZSH/oh-my-zsh.sh and sources ~/.p10k.zsh for the prompt",
	},
	{
		ID:    pluginSetIDFish,
		Step:  "shell",
		Tool:  "fish",
		Named: "fisher and the fish functions this repository vendors",
		Because: "the fish configuration this repository ships bootstraps it on first start, " +
			"so there is nothing a run could leave out",
	},
	{
		ID:          pluginSetIDTmux,
		Step:        "wm",
		Tool:        "tmux",
		Named:       "the plugins its configuration declares",
		Provisioned: true,
		Switchable:  true,
	},
	{
		ID:          pluginSetIDHerdr,
		Step:        "wm",
		Tool:        "herdr",
		Named:       "its declared plugins",
		Declared:    herdrPluginNames(),
		Provisioned: true,
		Switchable:  true,
	},
	{
		ID:    pluginSetIDNvim,
		Step:  "nvim",
		Tool:  "nvim",
		Named: "the LazyVim plugin set",
		Because: "lazy.nvim installs it on first launch from the configuration this " +
			"repository ships",
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
		if set.Switchable {
			ids = append(ids, set.ID)
		}
	}
	return ids
}

// pluginSetIsSwitchable reports whether a run can turn a set off. A set is
// switchable exactly when this installer can leave it out and the configuration it
// installs survives the absence: for the tmux and herdr sets the configuration is
// patched to name nothing, and for the shell's, fish's and Neovim's the
// configuration the repository ships is built on the set, which is why they are
// named with the reason instead of offered as a switch that would install nothing
// or ship a second, unverified configuration.
func pluginSetIsSwitchable(id string) bool {
	set, ok := pluginSetByID(id)
	return ok && set.Switchable
}

// SwitchableSkipIDs names everything --skip accepts: the units a run can leave
// out, in catalogue order, then the plugin sets it can turn off, in registry
// order.
func SwitchableSkipIDs() []string {
	ids := SwitchableUnitIDs()
	return append(ids, SwitchablePluginSetIDs()...)
}

// PluginSetState is the plugin set a step provisions, as the interface reads it:
// which set it is, what it is called, whether this run turned it off, and -- for a
// set a run cannot leave out -- why it is not offered as a switch.
//
// A set that was turned off is still carried, with Off set: the row has to say
// what the run is leaving out rather than disappear, which is the whole reason
// the set is on the step instead of in the step. A set that cannot be turned off
// is carried for the same reason: the user reads what the run installs and why it
// is not theirs to change, instead of finding it later.
type PluginSetState struct {
	ID         string
	Name       string
	Off        bool
	Switchable bool
	Because    string
}

// planPluginSet returns the set this step provisions for these choices, or nil
// when it provisions none. It is the single place the plan learns which tool
// carries which set, so the screen that offers the switch and the executor that
// honours it cannot disagree.
func planPluginSet(stepID string, opts planOptions) *PluginSetState {
	for _, set := range pluginSets {
		if set.Step != stepID || !pluginSetToolChosen(set, opts) {
			continue
		}
		state := &PluginSetState{
			ID:         set.ID,
			Name:       set.Label(),
			Switchable: set.Switchable,
			Because:    set.Because,
		}
		// A set a run cannot leave out has no off state to read: nothing a user can
		// type turns it off, so the plan never reports one.
		if set.Switchable {
			state.Off = pluginSetIsOff(opts.DisabledPluginSets, set.ID)
		}
		return state
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
	case "nvim":
		return opts.InstallNvim
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
			if !set.Switchable {
				return nil, nil, fmt.Errorf("%q is a plugin set this run cannot leave out: %s; this run can skip: %s",
					name, set.Because, strings.Join(SwitchableSkipIDs(), ", "))
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
